package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	apperrors "github.com/alldev-run/golang-gin-rpc/pkg/errors"
	pkgtracing "github.com/alldev-run/golang-gin-rpc/plugins/tracing"
	nws "github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/protobuf/proto"
)

var errTooManyConnections = errors.New("websocket: connection limit reached")

type ServerConfig struct {
	Addr             string        `yaml:"addr" json:"addr"`
	Path             string        `yaml:"path" json:"path"`
	MessageFormat    string        `yaml:"message_format" json:"message_format"`
	ReadTimeout      time.Duration `yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout     time.Duration `yaml:"write_timeout" json:"write_timeout"`
	RequireAuth      bool          `yaml:"require_auth" json:"require_auth"`
	HeartbeatTimeout time.Duration `yaml:"heartbeat_timeout" json:"heartbeat_timeout"`
	IdleTimeout      time.Duration `yaml:"idle_timeout" json:"idle_timeout"`
	// MaxConnections caps concurrent websocket connections; 0 means unlimited.
	MaxConnections int `yaml:"max_connections" json:"max_connections"`
	// SendQueueSize is the per-connection outbound queue capacity. A client
	// whose queue is full is closed with CloseCodeSendQueueFull (backpressure).
	SendQueueSize int `yaml:"send_queue_size" json:"send_queue_size"`
	// ReadLimit caps a single inbound message in bytes; 0 keeps the library
	// default (32KiB), -1 disables the limit.
	ReadLimit int64 `yaml:"read_limit" json:"read_limit"`
	// OriginPatterns enables cross-origin verification when non-empty (e.g.
	// "*.example.com"); empty keeps the legacy accept-any-origin behavior.
	OriginPatterns []string `yaml:"origin_patterns" json:"origin_patterns"`
	// Compression enables permessage-deflate (no context takeover).
	Compression bool `yaml:"compression" json:"compression"`
	// RateLimit, when set, throttles inbound messages per connection.
	RateLimit *ConnRateLimitConfig `yaml:"rate_limit" json:"rate_limit"`
	// SessionResume, when enabled, stamps outbound envelopes with sequence
	// numbers and replays missed messages after reconnect (see session.go).
	SessionResume *SessionResumeConfig       `yaml:"session_resume" json:"session_resume"`
	Authenticator Authenticator              `yaml:"-" json:"-"`
	Observer      Observer                   `yaml:"-" json:"-"`
	Tracer        *pkgtracing.TracerProvider `yaml:"-" json:"-"`
}

const defaultWSReadLimit int64 = 32768 // mirrors coder/websocket library default

// ConnRateLimitConfig throttles inbound messages per connection.
type ConnRateLimitConfig struct {
	// Rate is the sustained messages per second.
	Rate int `yaml:"rate" json:"rate"`
	// Burst is the bucket capacity above the sustained rate.
	Burst int `yaml:"burst" json:"burst"`
}

func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Addr:             ":8080",
		Path:             "/ws",
		MessageFormat:    MessageFormatJSON,
		ReadTimeout:      30 * time.Second,
		WriteTimeout:     30 * time.Second,
		HeartbeatTimeout: 90 * time.Second,
		IdleTimeout:      2 * time.Minute,
	}
}

type Conn struct {
	conn      *nws.Conn
	config    ServerConfig
	identity  Identity
	observer  Observer
	tracer    *pkgtracing.TracerProvider
	traceCtx  context.Context
	mu        sync.RWMutex
	lastSeen  time.Time
	closed    bool
	closeCode CloseCode

	// outbound queue: a single writer goroutine drains outbox so slow
	// clients never block callers; overflow kicks the connection.
	outInit      sync.Once
	outbox       chan outMsg
	outDrained   chan struct{}
	closeOutOnce sync.Once
	writeErrOnce sync.Once
	outSeq       atomic.Uint64 // outbound sequence when no session is attached
	seqMu        sync.Mutex    // orders (seq assign + session store + enqueue)
	session      *session      // set when session resume is enabled
	limiter      *connLimiter  // inbound rate limiter, nil when disabled
}

// outMsg is one queued outbound frame.
type outMsg struct {
	payload   []byte
	frameType byte
}

type Handler func(ctx context.Context, conn *Conn)

// connAcceptor owns the websocket upgrade path (limits, auth, session
// attach, connection registration). Server and the standalone HTTP/Gin
// handlers all route through it, so every entry point gets the same
// protections.
type connAcceptor struct {
	config      ServerConfig
	sessions    *SessionRegistry
	activeConns atomic.Int64
	mu          sync.RWMutex
	manager     *Manager
}

func newConnAcceptor(config ServerConfig, manager *Manager) *connAcceptor {
	config = normalizeServerConfig(config)
	a := &connAcceptor{config: config, manager: manager}
	if config.SessionResume != nil {
		a.sessions = NewSessionRegistry(*config.SessionResume)
	}
	return a
}

// normalizeServerConfig fills defaults; safe to call repeatedly.
func normalizeServerConfig(config ServerConfig) ServerConfig {
	config.MessageFormat = normalizeMessageFormat(config.MessageFormat)
	if err := validateMessageFormat(config.MessageFormat); err != nil {
		config.MessageFormat = MessageFormatJSON
	}
	if config.ReadTimeout <= 0 {
		config.ReadTimeout = 30 * time.Second
	}
	if config.WriteTimeout <= 0 {
		config.WriteTimeout = 30 * time.Second
	}
	if config.HeartbeatTimeout <= 0 {
		config.HeartbeatTimeout = 90 * time.Second
	}
	if config.IdleTimeout <= 0 {
		config.IdleTimeout = 2 * time.Minute
	}
	config.Observer = observerOrNoop(config.Observer)
	return config
}

func (a *connAcceptor) setManager(m *Manager) {
	a.mu.Lock()
	a.manager = m
	a.mu.Unlock()
}

func (a *connAcceptor) getManager() *Manager {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.manager
}

func (a *connAcceptor) close() {
	if a.sessions != nil {
		a.sessions.Close()
	}
}

type Server struct {
	config   ServerConfig
	acceptor *connAcceptor
	mux      *http.ServeMux
	handlers map[string]Handler
	server   *http.Server
	listener net.Listener
	mu       sync.RWMutex
}

func NewServer(config ServerConfig) *Server {
	if config.Addr == "" {
		config.Addr = ":8080"
	}
	if config.Path == "" {
		config.Path = "/ws"
	}
	config.MessageFormat = normalizeMessageFormat(config.MessageFormat)
	if err := validateMessageFormat(config.MessageFormat); err != nil {
		config.MessageFormat = MessageFormatJSON
	}
	if config.ReadTimeout <= 0 {
		config.ReadTimeout = 30 * time.Second
	}
	if config.WriteTimeout <= 0 {
		config.WriteTimeout = 30 * time.Second
	}
	if config.HeartbeatTimeout <= 0 {
		config.HeartbeatTimeout = 90 * time.Second
	}
	if config.IdleTimeout <= 0 {
		config.IdleTimeout = 2 * time.Minute
	}
	config.Observer = observerOrNoop(config.Observer)
	config = normalizeServerConfig(config)
	mux := http.NewServeMux()
	s := &Server{
		config:   config,
		mux:      mux,
		handlers: make(map[string]Handler),
		acceptor: newConnAcceptor(config, nil),
	}
	s.server = &http.Server{
		Addr:              config.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       75 * time.Second,
	}
	return s
}

func (s *Server) Handle(path string, handler Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[path] = handler
	s.mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		s.serveHTTPConn(w, r, handler)
	})
}

func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		return nil
	}
	ln, err := net.Listen("tcp", s.config.Addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.config.Addr, err)
	}
	s.listener = ln
	go func() {
		_ = s.server.Serve(ln)
	}()
	return nil
}

func (s *Server) StartListener(listener net.Listener) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		return nil
	}
	s.listener = listener
	go func() {
		_ = s.server.Serve(listener)
	}()
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener == nil {
		return nil
	}
	err := s.server.Shutdown(ctx)
	s.listener = nil
	s.acceptor.close()
	return err
}

func (s *Server) Addr() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return s.config.Addr
}

// ActiveConnections returns the number of currently live websocket connections.
func (s *Server) ActiveConnections() int64 {
	return s.acceptor.activeConns.Load()
}

// serve runs the hardened websocket upgrade path shared by Server, the
// standalone HTTP handler and the Gin handler.
func (a *connAcceptor) serve(w http.ResponseWriter, req *http.Request, handler Handler) {
	traceCtx := extractTraceFromHTTPHeaders(req.Context(), req.Header)
	traceCtx, handshakeSpan := startWebsocketSpan(traceCtx, a.config.Tracer, "websocket.server.handshake",
		attribute.String("http.method", req.Method),
		attribute.String("websocket.path", req.URL.Path),
		attribute.String("websocket.remote_addr", req.RemoteAddr),
	)
	if a.config.MaxConnections > 0 && a.activeConns.Load() >= int64(a.config.MaxConnections) {
		endSpan(handshakeSpan, errTooManyConnections)
		http.Error(w, errTooManyConnections.Error(), http.StatusServiceUnavailable)
		return
	}
	var (
		authResult *AuthResult
		err        error
	)
	if a.config.Authenticator != nil {
		authResult, err = a.config.Authenticator(req)
		if err != nil {
			a.config.Observer.OnAuthFailed(req.URL.Path, err)
			endSpan(handshakeSpan, err)
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
	}
	acceptOpts := &nws.AcceptOptions{}
	if len(a.config.OriginPatterns) > 0 {
		acceptOpts.OriginPatterns = a.config.OriginPatterns
	} else {
		acceptOpts.InsecureSkipVerify = true
	}
	if a.config.Compression {
		acceptOpts.CompressionMode = nws.CompressionNoContextTakeover
	}
	wsConn, err := nws.Accept(w, req, acceptOpts)
	if err != nil {
		endSpan(handshakeSpan, err)
		return
	}
	if a.config.ReadLimit >= 0 {
		limit := a.config.ReadLimit
		if limit == 0 {
			limit = defaultWSReadLimit
		}
		wsConn.SetReadLimit(limit)
	}
	a.activeConns.Add(1)
	defer a.activeConns.Add(-1)
	if a.config.RequireAuth && authResult == nil {
		authErr := authError(req.URL.Path, fmt.Errorf("authentication required"))
		a.config.Observer.OnAuthFailed(req.URL.Path, authErr)
		endSpan(handshakeSpan, authErr)
		_ = wsConn.Close(nws.StatusPolicyViolation, authErr.Error())
		return
	}
	conn := &Conn{
		conn:      wsConn,
		config:    a.config,
		identity:  newIdentity(req, authResult),
		observer:  a.config.Observer,
		tracer:    tracerOrGlobal(a.config.Tracer),
		traceCtx:  traceCtx,
		lastSeen:  time.Now(),
		closeCode: CloseCodeNormal,
	}
	if a.config.RateLimit != nil && a.config.RateLimit.Rate > 0 {
		conn.limiter = newConnLimiter(*a.config.RateLimit)
	}
	if mgr := a.getManager(); mgr != nil {
		mgr.Register(conn)
		defer mgr.Unregister(conn)
	}
	if _, err := a.attachSession(conn, req.URL.Query()); err != nil {
		endSpan(handshakeSpan, err)
		_ = conn.CloseWithCode(CloseCodeInternalError, err)
		return
	}
	conn.observer.OnConnect(conn.Identity())
	endSpan(handshakeSpan, nil)
	ctx, cancel := context.WithCancel(traceCtx)
	defer cancel()
	handler(ctx, conn)
	_ = conn.CloseWithCode(CloseCodeNormal, nil)
}

func (s *Server) serveHTTPConn(w http.ResponseWriter, req *http.Request, handler Handler) {
	s.acceptor.serve(w, req, handler)
}

func (c *Conn) Receive(ctx context.Context) (int, []byte, error) {
	if c.limiter != nil && !c.limiter.allow() {
		return 0, nil, errRateLimited
	}
	spanCtx, span := startWebsocketSpan(contextWithFallback(ctx, c.currentTraceCtx()), c.tracer, traceMessageName("receive", false),
		websocketTraceAttrs(c.Identity())...,
	)
	readCtx, cancel := context.WithTimeout(ctx, c.config.ReadTimeout)
	defer cancel()
	msgType, payload, err := c.conn.Read(readCtx)
	if err != nil {
		endSpan(span, err)
		return 0, nil, err
	}
	c.markSeen()
	c.observer.OnMessage("in", c.Identity(), len(payload))
	span.SetAttributes(attribute.Int("websocket.message_size", len(payload)))
	endSpan(span, nil)
	c.setTraceCtx(spanCtx)
	return int(msgType), payload, nil
}

func (c *Conn) ReceiveJSON(ctx context.Context, dest interface{}) error {
	if c.limiter != nil && !c.limiter.allow() {
		return errRateLimited
	}
	spanCtx, span := startWebsocketSpan(contextWithFallback(ctx, c.currentTraceCtx()), c.tracer, traceMessageName("receive", true),
		websocketTraceAttrs(c.Identity())...,
	)
	readCtx, cancel := context.WithTimeout(ctx, c.config.ReadTimeout)
	defer cancel()
	if err := wsjson.Read(readCtx, c.conn, dest); err != nil {
		endSpan(span, err)
		return err
	}
	c.markSeen()
	c.observer.OnMessage("in", c.Identity(), 1)
	endSpan(span, nil)
	c.setTraceCtx(spanCtx)
	return nil
}

func (c *Conn) SendText(ctx context.Context, message string) error {
	return c.send(ctx, []byte(message), byte(nws.MessageText))
}

func (c *Conn) SendBinary(ctx context.Context, payload []byte) error {
	return c.send(ctx, payload, byte(nws.MessageBinary))
}

func (c *Conn) SendJSON(ctx context.Context, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return c.SendText(ctx, string(data))
}

func (c *Conn) SendProto(ctx context.Context, message proto.Message) error {
	if message == nil {
		return apperrors.New(apperrors.ErrCodeWebSocketProtoMessageNil, "protobuf message is nil")
	}
	data, err := proto.Marshal(message)
	if err != nil {
		return err
	}
	return c.SendBinary(ctx, data)
}

func (c *Conn) ReceiveProto(ctx context.Context, dest proto.Message) error {
	if dest == nil {
		return apperrors.New(apperrors.ErrCodeWebSocketProtoDestinationNil, "protobuf destination is nil")
	}
	msgType, payload, err := c.Receive(ctx)
	if err != nil {
		return err
	}
	if msgType != int(nws.MessageBinary) {
		return apperrors.New(apperrors.ErrCodeWebSocketProtoFrameType, "expected binary frame for protobuf").WithDetails(map[string]interface{}{"frame_type": msgType})
	}
	return proto.Unmarshal(payload, dest)
}

// SendMessage sends payload using configured server message format.
// For protobuf mode, payload must implement proto.Message.
func (c *Conn) SendMessage(ctx context.Context, payload interface{}) error {
	switch normalizeMessageFormat(c.config.MessageFormat) {
	case MessageFormatProtobuf:
		msg, ok := payload.(proto.Message)
		if !ok {
			return apperrors.New(apperrors.ErrCodeWebSocketProtoPayloadType, "protobuf mode requires proto.Message payload")
		}
		return c.SendProto(ctx, msg)
	case MessageFormatJSON:
		fallthrough
	default:
		return c.SendJSON(ctx, payload)
	}
}

// ReceiveMessage receives payload using configured server message format.
// For protobuf mode, dest must implement proto.Message.
func (c *Conn) ReceiveMessage(ctx context.Context, dest interface{}) error {
	switch normalizeMessageFormat(c.config.MessageFormat) {
	case MessageFormatProtobuf:
		msg, ok := dest.(proto.Message)
		if !ok {
			return apperrors.New(apperrors.ErrCodeWebSocketProtoDestinationType, "protobuf mode requires proto.Message destination")
		}
		return c.ReceiveProto(ctx, msg)
	case MessageFormatJSON:
		fallthrough
	default:
		return c.ReceiveJSON(ctx, dest)
	}
}

func (c *Conn) Close() error {
	return c.CloseWithCode(CloseCodeNormal, nil)
}

// maxCloseDrainWait bounds how long Close waits for queued outbound frames.
const maxCloseDrainWait = 5 * time.Second

// CloseWithCode marks the connection closed, drains queued outbound frames
// (bounded wait) and then closes the underlying connection, so sends issued
// before Close still reach the wire.
func (c *Conn) CloseWithCode(code CloseCode, err error) error {
	return c.close(code, err, true)
}

func (c *Conn) close(code CloseCode, err error, drain bool) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.closeCode = code
	c.mu.Unlock()
	c.closeOutbox()
	if drain {
		c.waitDrained()
	}
	if c.session != nil {
		c.session.detach(c)
	}
	c.observer.OnDisconnect(c.Identity(), classifyCloseCode(err, code), err)
	status := nws.StatusNormalClosure
	switch code {
	case CloseCodeAuthFailed:
		status = nws.StatusPolicyViolation
	case CloseCodeHeartbeatTimeout, CloseCodeIdleTimeout:
		status = nws.StatusGoingAway
	case CloseCodeWriteError, CloseCodeReadError, CloseCodeInternalError:
		status = nws.StatusInternalError
	}
	reason := ""
	if err != nil {
		reason = err.Error()
	}
	return c.conn.Close(status, reason)
}

func (c *Conn) Identity() Identity {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.identity
}

// send enqueues an outbound frame onto the per-connection queue. A single
// writer goroutine drains the queue, so a slow client never blocks callers:
// when the queue is full the connection is closed with CloseCodeSendQueueFull
// (backpressure by kicking slow consumers). Write errors surface through
// OnDisconnect / CloseCode instead of this function's return value.
func (c *Conn) send(_ context.Context, payload []byte, frameType byte) error {
	if c == nil || c.conn == nil {
		return errors.New("websocket: connection is not initialized")
	}
	c.mu.RLock()
	closed := c.closed
	c.mu.RUnlock()
	if closed {
		return errors.New("websocket: connection closed")
	}
	c.ensureWriter()
	select {
	case c.outbox <- outMsg{payload: payload, frameType: frameType}:
		return nil
	default:
		err := errors.New("websocket: send queue full")
		c.CloseWithCode(CloseCodeSendQueueFull, err)
		return err
	}
}

// ensureWriter lazily creates the outbound queue and starts the writer
// goroutine (the only place that writes to the underlying connection).
func (c *Conn) ensureWriter() {
	c.outInit.Do(func() {
		size := c.config.SendQueueSize
		if size <= 0 {
			size = 256
		}
		c.outbox = make(chan outMsg, size)
		c.outDrained = make(chan struct{})
		go c.writeLoop()
	})
}

func (c *Conn) writeLoop() {
	defer close(c.outDrained)
	for msg := range c.outbox {
		if err := c.writeFrame(msg.payload, msg.frameType); err != nil {
			// The connection is broken; close without draining.
			c.writeErrOnce.Do(func() {
				_ = c.close(CloseCodeWriteError, err, false)
			})
			return
		}
	}
}

func (c *Conn) writeFrame(payload []byte, frameType byte) error {
	_, span := startWebsocketSpan(contextWithFallback(context.Background(), c.currentTraceCtx()), c.tracer, traceMessageName("send", false),
		websocketTraceAttrs(c.Identity(), attribute.Int("websocket.message_size", len(payload)))...,
	)
	writeCtx, cancel := context.WithTimeout(context.Background(), c.config.WriteTimeout)
	defer cancel()
	msgType := nws.MessageText
	if frameType != byte(nws.MessageText) {
		msgType = nws.MessageBinary
	}
	if err := c.conn.Write(writeCtx, msgType, payload); err != nil {
		endSpan(span, err)
		return err
	}
	c.markSeen()
	c.observer.OnMessage("out", c.Identity(), len(payload))
	endSpan(span, nil)
	return nil
}

func (c *Conn) closeOutbox() {
	c.ensureWriter()
	c.closeOutOnce.Do(func() { close(c.outbox) })
}

// waitDrained blocks until the writer goroutine exits (queue drained or a
// write error aborted it) or the bounded drain budget elapses.
func (c *Conn) waitDrained() {
	if c.outDrained == nil {
		return
	}
	budget := c.config.WriteTimeout
	if budget <= 0 || budget > maxCloseDrainWait {
		budget = maxCloseDrainWait
	}
	select {
	case <-c.outDrained:
	case <-time.After(budget):
	}
}

func (c *Conn) currentTraceCtx() context.Context {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.traceCtx
}

func (c *Conn) setTraceCtx(ctx context.Context) {
	c.mu.Lock()
	c.traceCtx = ctx
	c.mu.Unlock()
}

// OutboxDrained returns a channel closed once the writer goroutine has exited
// (useful in tests and graceful-shutdown paths).
func (c *Conn) OutboxDrained() <-chan struct{} {
	c.ensureWriter()
	return c.outDrained
}

func (c *Conn) SetGroups(groups ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.identity.Groups = append([]string(nil), groups...)
}

func (c *Conn) AddGroups(groups ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	exists := make(map[string]struct{}, len(c.identity.Groups))
	for _, group := range c.identity.Groups {
		exists[group] = struct{}{}
	}
	for _, group := range groups {
		if _, ok := exists[group]; ok {
			continue
		}
		c.identity.Groups = append(c.identity.Groups, group)
		exists[group] = struct{}{}
	}
}

// RemoveGroups removes the given groups from the connection's identity.
func (c *Conn) RemoveGroups(groups ...string) {
	if len(groups) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	drop := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		drop[group] = struct{}{}
	}
	kept := c.identity.Groups[:0]
	for _, group := range c.identity.Groups {
		if _, ok := drop[group]; ok {
			continue
		}
		kept = append(kept, group)
	}
	c.identity.Groups = kept
}

func (c *Conn) markSeen() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastSeen = time.Now()
}
