package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"
)

// ===== 测试辅助 =====

func startTestServer(t *testing.T, cfg ServerConfig, h Handler) (*Server, string) {
	t.Helper()
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = 5 * time.Second
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 5 * time.Second
	}
	server := NewServer(cfg)
	server.Handle("/ws", h)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	if err := server.StartListener(ln); err != nil {
		t.Fatalf("StartListener() error = %v", err)
	}
	t.Cleanup(func() { _ = server.Stop(context.Background()) })
	return server, "ws://" + ln.Addr().String() + "/ws"
}

func connectClient(t *testing.T, url string) *Client {
	t.Helper()
	client := NewClient(Config{URL: url, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

func mustJSON(t *testing.T, v interface{}) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

// ===== 协议层 =====

// TestRouterEchoEnvelope 验证 action 路由分发与 req_id 请求-响应关联。
func TestRouterEchoEnvelope(t *testing.T) {
	router := NewRouter()
	router.Handle("echo", func(ctx context.Context, conn *Conn, env *Envelope) {
		_ = conn.Reply(ctx, env, map[string]string{"got": string(env.Data)}, nil)
	})
	_, url := startTestServer(t, ServerConfig{}, router.Handler())
	client := connectClient(t, url)

	ctx := context.Background()
	if err := client.SendText(ctx, `{"action":"echo","req_id":"r-42","data":{"msg":"hi"}}`); err != nil {
		t.Fatalf("SendText: %v", err)
	}
	_, payload, err := client.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	var env Envelope
	if err := json.Unmarshal(payload, &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Action != "echo" || env.ReqID != "r-42" || env.Code != CodeOK {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	var got map[string]string
	if err := json.Unmarshal(env.Data, &got); err != nil || got["got"] != `{"msg":"hi"}` {
		t.Fatalf("unexpected data: %s", env.Data)
	}
	if env.Seq != 1 {
		t.Fatalf("expected seq 1, got %d", env.Seq)
	}
}

// TestRouterUnknownAction 验证未知 action 得到错误回包。
func TestRouterUnknownAction(t *testing.T) {
	router := NewRouter()
	_, url := startTestServer(t, ServerConfig{}, router.Handler())
	client := connectClient(t, url)

	ctx := context.Background()
	_ = client.SendText(ctx, `{"action":"nope","req_id":"r-1"}`)
	_, payload, err := client.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	var env Envelope
	_ = json.Unmarshal(payload, &env)
	if env.Code == CodeOK || env.Err == "" {
		t.Fatalf("expected error envelope, got %+v", env)
	}
}

// TestEnvelopeSeqMonotonicUnderConcurrency 验证并发发送下 Seq 与线上顺序一致。
func TestEnvelopeSeqMonotonicUnderConcurrency(t *testing.T) {
	router := NewRouter()
	router.Handle("burst", func(ctx context.Context, conn *Conn, env *Envelope) {
		var n int
		_ = json.Unmarshal(env.Data, &n)
		done := make(chan struct{})
		for i := 0; i < n; i++ {
			go func(i int) {
				defer func() { done <- struct{}{} }()
				_ = conn.SendEnvelope(ctx, &Envelope{Action: "b", Data: mustJSON(t, i)})
			}(i)
		}
		for i := 0; i < n; i++ {
			<-done
		}
		_ = conn.Reply(ctx, env, nil, nil)
	})
	_, url := startTestServer(t, ServerConfig{SendQueueSize: 128}, router.Handler())
	client := connectClient(t, url)

	ctx := context.Background()
	_ = client.SendText(ctx, `{"action":"burst","req_id":"r","data":32}`)
	var last uint64
	count := 0
	for count < 33 {
		_, payload, err := client.Receive(ctx)
		if err != nil {
			t.Fatalf("Receive: %v", err)
		}
		var env Envelope
		_ = json.Unmarshal(payload, &env)
		if env.Seq <= last {
			t.Fatalf("seq regression: %d after %d", env.Seq, last)
		}
		last = env.Seq
		count++
	}
}

// ===== 会话恢复 =====

func TestSessionResumeReplay(t *testing.T) {
	router := NewRouter()
	router.Handle("push", func(ctx context.Context, conn *Conn, env *Envelope) {
		for i := 0; i < 3; i++ {
			_ = conn.SendEnvelope(ctx, &Envelope{Action: "push.data", Data: mustJSON(t, i)})
		}
		_ = conn.Reply(ctx, env, map[string]int{"sent": 3}, nil)
	})
	cfg := ServerConfig{SessionResume: &SessionResumeConfig{BufferCapacity: 16, TTL: 5 * time.Second}}
	_, url := startTestServer(t, cfg, router.Handler())

	// 第一次连接:产生 seq 1..4 并正常收完。
	c1 := connectClient(t, url+"?session_id=s-resume")
	ctx := context.Background()
	_ = c1.SendText(ctx, `{"action":"push","req_id":"r1"}`)
	for i := 0; i < 4; i++ {
		if _, _, err := c1.Receive(ctx); err != nil {
			t.Fatalf("first session receive #%d: %v", i, err)
		}
	}
	c1.Close()
	time.Sleep(50 * time.Millisecond) // 让服务端处理断连

	// 断线重连:带 last_seq=1,应补发 seq 2..4。
	c2 := connectClient(t, url+"?session_id=s-resume&last_seq=1")
	for i := 0; i < 3; i++ { // 3 条重放(seq 2,3,4)
		_, payload, err := c2.Receive(ctx)
		if err != nil {
			t.Fatalf("replay receive #%d: %v", i, err)
		}
		var env Envelope
		_ = json.Unmarshal(payload, &env)
		if want := uint64(i + 2); env.Seq != want {
			t.Fatalf("replay #%d: expected seq %d, got %d (%s)", i, want, env.Seq, env.Action)
		}
	}
}

// ===== 房间系统 =====

func TestRoomJoinBroadcastLeave(t *testing.T) {
	rooms := NewRoomManager(RoomOptions{})
	defer rooms.Close()
	router := NewRouter()
	router.Handle("join", func(ctx context.Context, conn *Conn, env *Envelope) {
		var roomID string
		_ = json.Unmarshal(env.Data, &roomID)
		n, err := rooms.Join(roomID, conn)
		_ = conn.Reply(ctx, env, map[string]int{"members": n}, err)
	})
	router.Handle("say", func(ctx context.Context, conn *Conn, env *Envelope) {
		var roomID string
		_ = json.Unmarshal(env.Data, &roomID)
		room, ok := rooms.Get(roomID)
		if !ok {
			_ = conn.Reply(ctx, env, nil, fmt.Errorf("no room"))
			return
		}
		_ = room.Broadcast(ctx, &Envelope{Action: "room.msg", Data: mustJSON(t, "hello-room")})
	})
	_, url := startTestServer(t, ServerConfig{}, router.Handler())

	c1 := connectClient(t, url)
	c2 := connectClient(t, url)
	ctx := context.Background()

	_ = c1.SendText(ctx, `{"action":"join","req_id":"j1","data":"room-1"}`)
	_, p1, err := c1.Receive(ctx)
	if err != nil {
		t.Fatalf("join reply: %v", err)
	}
	var joinEnv Envelope
	_ = json.Unmarshal(p1, &joinEnv)
	if joinEnv.ReqID != "j1" {
		t.Fatalf("unexpected join reply: %+v", joinEnv)
	}

	_ = c2.SendText(ctx, `{"action":"join","req_id":"j2","data":"room-1"}`)
	if _, _, err := c2.Receive(ctx); err != nil { // join reply
		t.Fatalf("c2 join: %v", err)
	}

	_ = c1.SendText(ctx, `{"action":"say","data":"room-1"}`)
	// 两个成员都应收到 room.msg。
	for _, c := range []*Client{c1, c2} {
		_, payload, err := c.Receive(ctx)
		if err != nil {
			t.Fatalf("room broadcast: %v", err)
		}
		var env Envelope
		_ = json.Unmarshal(payload, &env)
		if env.Action != "room.msg" {
			t.Fatalf("expected room.msg, got %s", env.Action)
		}
	}
}

func TestRoomEmptyEviction(t *testing.T) {
	fired := make(chan string, 1)
	rooms := NewRoomManager(RoomOptions{
		EmptyTTL:     40 * time.Millisecond,
		JanitorEvery: 20 * time.Millisecond,
		OnEmpty:      func(roomID string) { fired <- roomID },
	})
	defer rooms.Close()

	conn := &Conn{identity: Identity{ConnectionID: "c-1"}}
	if _, err := rooms.Join("room-x", conn); err != nil {
		t.Fatalf("Join: %v", err)
	}
	room, ok := rooms.Get("room-x")
	if !ok || !room.Has("c-1") {
		t.Fatal("expected member")
	}
	rooms.Leave("room-x", conn)
	select {
	case id := <-fired:
		if id != "room-x" {
			t.Fatalf("evicted %s", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("empty room was not evicted")
	}
}

// ===== 限流 =====

func TestConnRateLimit(t *testing.T) {
	router := NewRouter()
	router.Handle("ping", func(ctx context.Context, conn *Conn, env *Envelope) {
		_ = conn.Reply(ctx, env, nil, nil)
	})
	cfg := ServerConfig{RateLimit: &ConnRateLimitConfig{Rate: 20, Burst: 2}}
	_, url := startTestServer(t, cfg, router.Handler())
	client := connectClient(t, url)

	ctx := context.Background()
	replies := 0
	var closed bool
	for i := 0; i < 10 && !closed; i++ {
		if err := client.SendText(ctx, fmt.Sprintf(`{"action":"ping","req_id":"r%d"}`, i)); err != nil {
			break
		}
		_, _, err := client.Receive(ctx)
		if err != nil {
			closed = true // 超限后连接被服务端关闭
			break
		}
		replies++
	}
	if replies == 0 {
		t.Fatal("expected some replies before rate limiting")
	}
	if replies >= 10 {
		t.Fatal("expected rate limiting to kick in for 10 rapid messages")
	}
}

// ===== 连接上限 =====

func TestMaxConnections(t *testing.T) {
	block := make(chan struct{})
	handler := func(ctx context.Context, conn *Conn) {
		<-block
	}
	_, url := startTestServer(t, ServerConfig{MaxConnections: 1}, handler)

	c1 := connectClient(t, url) // 占用唯一名额
	blocked := NewClient(Config{URL: url, ReadTimeout: time.Second, WriteTimeout: time.Second})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := blocked.Connect(ctx); err == nil {
		blocked.Close()
		t.Fatal("expected second connection to be rejected")
	}
	close(block)
	_ = c1
}

// ===== 重放缓冲单元测试 =====

func TestReplayBufferRotation(t *testing.T) {
	buf := newReplayBuffer(3)
	for i := uint64(1); i <= 5; i++ {
		buf.store(i, []byte(fmt.Sprintf("m%d", i)))
	}
	if got := buf.replayAfter(3); len(got) != 2 || string(got[0]) != "m4" || string(got[1]) != "m5" {
		t.Fatalf("unexpected replay: %v", got)
	}
	// 环形轮换后,早于容量窗口的消息不再可重放。
	if got := buf.replayAfter(0); len(got) != 3 {
		t.Fatalf("rotation should keep only the last %d messages, got %d", 3, len(got))
	}
}
