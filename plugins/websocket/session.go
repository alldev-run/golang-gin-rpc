package websocket

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	nws "github.com/coder/websocket"
	"github.com/google/uuid"
)

// 会话恢复:断线重连后补发错过的下行消息。
//
// 工作方式:
//   - 服务端为每个会话维护有界的下行重放缓冲(最近 N 条按 Seq 排序)。
//   - 客户端首次连接可携带 session_id(不传则由服务端生成);断线后在
//     TTL 内以相同 session_id 重连,并携带 last_seq=<已收到的最大 Seq>。
//   - 服务端把缓冲中 Seq > last_seq 的消息按序补发,然后再进入正常处理。
//     同一 session 的旧连接会被关闭(以新连接为准)。
//
// 会话注册表是单节点的;跨节点会话粘性由网关/发现层保证(同一 session_id
// 路由到同一节点)。

const (
	defaultReplayBufferCapacity = 1024
	defaultSessionTTL           = 60 * time.Second
	defaultSessionSweepInterval = 10 * time.Second
)

// SessionResumeConfig enables and tunes connection session resume.
type SessionResumeConfig struct {
	// BufferCapacity is the per-session outbound replay buffer size.
	BufferCapacity int `yaml:"buffer_capacity" json:"buffer_capacity"`
	// TTL is how long a disconnected session stays resumable.
	TTL time.Duration `yaml:"ttl" json:"ttl"`
	// MaxSessions caps tracked sessions; 0 means unlimited.
	MaxSessions int `yaml:"max_sessions" json:"max_sessions"`
}

func (c *SessionResumeConfig) normalize() {
	if c.BufferCapacity <= 0 {
		c.BufferCapacity = defaultReplayBufferCapacity
	}
	if c.TTL <= 0 {
		c.TTL = defaultSessionTTL
	}
}

// storedMessage is one outbound envelope kept for replay.
type storedMessage struct {
	seq     uint64
	payload []byte
}

// replayBuffer is a bounded FIFO of outbound messages keyed by sequence.
type replayBuffer struct {
	mu   sync.Mutex
	cap  int
	ring []storedMessage // oldest first, len <= cap
}

func newReplayBuffer(capacity int) *replayBuffer {
	return &replayBuffer{cap: capacity}
}

func (b *replayBuffer) store(seq uint64, payload []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.ring = append(b.ring, storedMessage{seq: seq, payload: payload})
	if len(b.ring) > b.cap {
		b.ring = b.ring[len(b.ring)-b.cap:]
	}
}

// replayAfter returns payloads with seq > lastSeq, in send order.
func (b *replayBuffer) replayAfter(lastSeq uint64) [][]byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([][]byte, 0, len(b.ring))
	for _, m := range b.ring {
		if m.seq > lastSeq {
			out = append(out, m.payload)
		}
	}
	return out
}

var errSessionDisplaced = errors.New("websocket: session resumed elsewhere")

// session is one resumable client session. At any time at most one Conn is
// attached; a resumed connection displaces the previous one.
type session struct {
	id     string
	userID string
	seq    atomic.Uint64 // authoritative outbound sequence for the session

	mu       sync.Mutex
	conn     *Conn
	buf      *replayBuffer
	lastSeen time.Time
}

func (s *session) nextSeq() uint64 { return s.seq.Add(1) }

// store records an outbound message for replay. Callers hold the conn-level
// seqMu, which keeps buffer order identical to wire order.
func (s *session) store(seq uint64, payload []byte) {
	s.mu.Lock()
	s.buf.store(seq, payload)
	s.mu.Unlock()
}

// attach binds conn to the session, displacing (and closing) any previous
// connection. It returns the displaced connection, if any.
func (s *session) attach(conn *Conn) (displaced *Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	displaced = s.conn
	s.conn = conn
	s.lastSeen = time.Now()
	return displaced
}

// detach records the disconnect if it belongs to the currently attached conn.
func (s *session) detach(conn *Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == conn {
		s.conn = nil
	}
	s.lastSeen = time.Now()
}

// SessionRegistry tracks resumable sessions and evicts expired ones.
type SessionRegistry struct {
	mu       sync.RWMutex
	sessions map[string]*session
	cfg      SessionResumeConfig
	stop     chan struct{}
	stopOnce sync.Once
}

// NewSessionRegistry starts a registry with an expiry janitor.
func NewSessionRegistry(cfg SessionResumeConfig) *SessionRegistry {
	cfg.normalize()
	r := &SessionRegistry{
		sessions: make(map[string]*session),
		cfg:      cfg,
		stop:     make(chan struct{}),
	}
	go r.janitor()
	return r
}

func (r *SessionRegistry) janitor() {
	ticker := time.NewTicker(defaultSessionSweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-r.stop:
			return
		case <-ticker.C:
			r.evictExpired()
		}
	}
}

func (r *SessionRegistry) evictExpired() {
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, s := range r.sessions {
		s.mu.Lock()
		idle := s.conn == nil && now.Sub(s.lastSeen) > r.cfg.TTL
		s.mu.Unlock()
		if idle {
			delete(r.sessions, id)
		}
	}
}

// resolve returns the session for the given id, creating it when absent.
// resumed reports whether an existing (possibly disconnected) session was
// found and should be replayed onto the new connection.
func (r *SessionRegistry) resolve(sessionID, userID string) (*session, bool) {
	r.mu.RLock()
	s, ok := r.sessions[sessionID]
	r.mu.RUnlock()
	if ok {
		s.mu.Lock()
		s.lastSeen = time.Now()
		s.mu.Unlock()
		return s, true
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.sessions[sessionID]; ok { // re-check under write lock
		return s, true
	}
	if r.cfg.MaxSessions > 0 && len(r.sessions) >= r.cfg.MaxSessions {
		return nil, false
	}
	s = &session{
		id:       sessionID,
		userID:   userID,
		buf:      newReplayBuffer(r.cfg.BufferCapacity),
		lastSeen: time.Now(),
	}
	r.sessions[sessionID] = s
	return s, false
}

// Close stops the janitor.
func (r *SessionRegistry) Close() {
	r.stopOnce.Do(func() { close(r.stop) })
}

// resumeInfo carries the parsed resume query parameters.
type resumeInfo struct {
	sessionID string
	lastSeq   uint64
}

func parseResumeQuery(query url.Values) (resumeInfo, bool) {
	sid := query.Get("session_id")
	if sid == "" {
		return resumeInfo{}, false
	}
	info := resumeInfo{sessionID: sid}
	if ls := query.Get("last_seq"); ls != "" {
		if n, err := strconv.ParseUint(ls, 10, 64); err == nil {
			info.lastSeq = n
		}
	}
	return info, true
}

// attachSession wires a fresh conn to its session, replays missed messages
// into the outbound queue before the business handler starts, and returns the
// session id. It is a no-op when session resume is disabled.
func (a *connAcceptor) attachSession(conn *Conn, query url.Values) (string, error) {
	if a.sessions == nil {
		return "", nil
	}
	info, _ := parseResumeQuery(query)
	if info.sessionID == "" {
		info.sessionID = uuid.NewString()
	}
	sess, resumed := a.sessions.resolve(info.sessionID, conn.Identity().UserID)
	if sess == nil {
		return "", errTooManyConnections
	}
	conn.session = sess
	if displaced := sess.attach(conn); displaced != nil && displaced != conn {
		_ = displaced.CloseWithCode(CloseCodeNormal, errSessionDisplaced)
	}
	if resumed {
		ctx, cancel := context.WithTimeout(context.Background(), a.config.WriteTimeout*2)
		defer cancel()
		for _, payload := range sess.buf.replayAfter(info.lastSeq) {
			if err := conn.send(ctx, payload, byte(nws.MessageText)); err != nil {
				return info.sessionID, err
			}
		}
	}
	return info.sessionID, nil
}
