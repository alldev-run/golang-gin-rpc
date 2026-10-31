package websocket

import (
	"context"
	"sync"
	"time"
)

// 房间系统:有成员索引、生命周期与空房回收的房间,替代"线性扫描 Groups
// 标签"的过滤式广播。房间广播对每个成员调用 Conn.SendEnvelope,因此成员
// 各自拿到独立的 Seq 并进入其会话重放缓冲(可靠补发)。
//
// 集群联动:Join/Leave 会同步维护连接上的 "room:<id>" Group 标签,跨节点
// 广播可继续复用 Manager.BroadcastToGroup(ctx, "room:<id>", ...)。

const (
	defaultRoomEmptyTTL     = 30 * time.Second
	defaultRoomJanitorEvery = 10 * time.Second
	roomGroupPrefix         = "room:"
)

// RoomOptions tunes the room manager.
type RoomOptions struct {
	// MaxMembers caps members per room; 0 means unlimited.
	MaxMembers int `yaml:"max_members" json:"max_members"`
	// EmptyTTL evicts rooms that stay empty this long; negative keeps them
	// forever; 0 defaults to 30s.
	EmptyTTL time.Duration `yaml:"empty_ttl" json:"empty_ttl"`
	// JanitorEvery is the eviction sweep interval; 0 defaults to 10s.
	JanitorEvery time.Duration `yaml:"janitor_every" json:"janitor_every"`
	// OnEmpty fires when a room is evicted for being empty.
	OnEmpty func(roomID string)
}

func (o *RoomOptions) normalize() {
	if o.EmptyTTL == 0 {
		o.EmptyTTL = defaultRoomEmptyTTL
	}
	if o.JanitorEvery <= 0 {
		o.JanitorEvery = defaultRoomJanitorEvery
	}
}

// Room is an indexed group of connections with a lifecycle.
type Room struct {
	id   string
	meta map[string]string

	mu         sync.RWMutex
	members    map[string]*Conn // connectionID → conn
	emptySince time.Time
}

// ID returns the room id.
func (r *Room) ID() string { return r.id }

// Count returns the current member count.
func (r *Room) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.members)
}

// MemberIDs lists member connection ids.
func (r *Room) MemberIDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.members))
	for id := range r.members {
		ids = append(ids, id)
	}
	return ids
}

// Has reports whether a connection is a member.
func (r *Room) Has(connectionID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.members[connectionID]
	return ok
}

// Broadcast sends the envelope to every member through their per-connection
// queue (each member gets its own Seq and session replay entry).
func (r *Room) Broadcast(ctx context.Context, env *Envelope) error {
	r.mu.RLock()
	members := make([]*Conn, 0, len(r.members))
	for _, c := range r.members {
		members = append(members, c)
	}
	r.mu.RUnlock()
	errs := &BroadcastErrors{}
	for _, c := range members {
		if err := c.SendEnvelope(ctx, env); err != nil {
			errs.Add(err)
		}
	}
	return errs.First()
}

// RoomManager owns room lifecycle: join/leave, disconnect cleanup and empty
// room eviction.
type RoomManager struct {
	mu    sync.RWMutex
	rooms map[string]*Room
	opts  RoomOptions
	stop  chan struct{}
	once  sync.Once
}

// NewRoomManager creates the manager and starts the empty-room janitor.
func NewRoomManager(opts RoomOptions) *RoomManager {
	opts.normalize()
	rm := &RoomManager{
		rooms: make(map[string]*Room),
		opts:  opts,
		stop:  make(chan struct{}),
	}
	go rm.janitor()
	return rm
}

func (rm *RoomManager) janitor() {
	ticker := time.NewTicker(rm.opts.JanitorEvery)
	defer ticker.Stop()
	for {
		select {
		case <-rm.stop:
			return
		case <-ticker.C:
			rm.evictEmptyRooms()
		}
	}
}

func (rm *RoomManager) evictEmptyRooms() {
	now := time.Now()
	var evicted []string
	rm.mu.Lock()
	for id, room := range rm.rooms {
		room.mu.Lock()
		empty := len(room.members) == 0 &&
			!room.emptySince.IsZero() &&
			rm.opts.EmptyTTL > 0 &&
			now.Sub(room.emptySince) >= rm.opts.EmptyTTL
		if empty {
			delete(rm.rooms, id)
			evicted = append(evicted, id)
		}
		room.mu.Unlock()
	}
	rm.mu.Unlock()
	if rm.opts.OnEmpty != nil {
		for _, id := range evicted {
			rm.opts.OnEmpty(id)
		}
	}
}

// Get returns the room by id if it exists (and is not empty-evicted).
func (rm *RoomManager) Get(roomID string) (*Room, bool) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	room, ok := rm.rooms[roomID]
	return room, ok
}

// Count returns the number of live rooms.
func (rm *RoomManager) Count() int {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return len(rm.rooms)
}

// Join adds the connection to the room, creating the room on demand. It
// returns the member count after joining. Joining also tags the connection
// with the "room:<id>" group for cluster-wide group broadcast.
func (rm *RoomManager) Join(roomID string, conn *Conn) (int, error) {
	if roomID == "" || conn == nil {
		return 0, errEnvelopeNil
	}
	rm.mu.Lock()
	room, ok := rm.rooms[roomID]
	if !ok {
		room = &Room{
			id:      roomID,
			members: make(map[string]*Conn),
		}
		rm.rooms[roomID] = room
	}
	rm.mu.Unlock()

	room.mu.Lock()
	if room.members == nil { // defensive: zero-value Room
		room.members = make(map[string]*Conn)
	}
	if rm.opts.MaxMembers > 0 && len(room.members) >= rm.opts.MaxMembers {
		if _, exists := room.members[conn.Identity().ConnectionID]; !exists {
			room.mu.Unlock()
			return len(room.members), errRoomFull
		}
	}
	room.members[conn.Identity().ConnectionID] = conn
	room.emptySince = time.Time{}
	room.mu.Unlock()

	conn.AddGroups(roomGroupPrefix + roomID)
	return room.Count(), nil
}

// Leave removes the connection from the room and drops the group tag.
func (rm *RoomManager) Leave(roomID string, conn *Conn) bool {
	rm.mu.RLock()
	room, ok := rm.rooms[roomID]
	rm.mu.RUnlock()
	if !ok {
		return false
	}
	room.mu.Lock()
	id := conn.Identity().ConnectionID
	_, existed := room.members[id]
	delete(room.members, id)
	if len(room.members) == 0 {
		room.emptySince = time.Now()
	}
	room.mu.Unlock()
	if existed {
		conn.RemoveGroups(roomGroupPrefix + roomID)
	}
	return existed
}

// RemoveConnection removes one connection from every room it belongs to;
// wire it to OnDisconnect (see Observer()).
func (rm *RoomManager) RemoveConnection(connectionID string) {
	if connectionID == "" {
		return
	}
	rm.mu.RLock()
	rooms := make([]*Room, 0, len(rm.rooms))
	for _, room := range rm.rooms {
		rooms = append(rooms, room)
	}
	rm.mu.RUnlock()
	for _, room := range rooms {
		room.mu.Lock()
		conn, ok := room.members[connectionID]
		if ok {
			delete(room.members, connectionID)
		}
		if len(room.members) == 0 {
			room.emptySince = time.Now()
		}
		room.mu.Unlock()
		if ok {
			conn.RemoveGroups(roomGroupPrefix + room.id)
		}
	}
}

// Observer returns an Observer adapter that keeps rooms in sync with
// connection lifecycles; compose it with the server observer.
func (rm *RoomManager) Observer() Observer {
	return roomJanitorObserver{rm: rm}
}

type roomJanitorObserver struct {
	rm *RoomManager
}

func (o roomJanitorObserver) OnConnect(Identity) {}
func (o roomJanitorObserver) OnDisconnect(identity Identity, _ CloseCode, _ error) {
	o.rm.RemoveConnection(identity.ConnectionID)
}
func (o roomJanitorObserver) OnAuthFailed(string, error)                    {}
func (o roomJanitorObserver) OnBroadcast(string, int, error)                {}
func (o roomJanitorObserver) OnMessage(string, Identity, int)               {}
func (o roomJanitorObserver) OnReconnectAttempt(string, int, time.Duration) {}
func (o roomJanitorObserver) OnHeartbeatTimeout(Identity)                   {}

// Close stops the janitor; rooms and connections are untouched.
func (rm *RoomManager) Close() {
	rm.once.Do(func() { close(rm.stop) })
}

var errRoomFull = &simpleErr{msg: "websocket: room is full"}

type simpleErr struct{ msg string }

func (e *simpleErr) Error() string { return e.msg }
