package websocket

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	nws "nhooyr.io/websocket"
)

func waitForCondition(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}

func httpHandlerForHeartbeat(t *testing.T, heartbeats chan<- string) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := nws.Accept(w, r, &nws.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer conn.Close(nws.StatusNormalClosure, "")
		for {
			_, payload, err := conn.Read(r.Context())
			if err != nil {
				return
			}
			heartbeats <- string(payload)
		}
	})
}

func TestManager_BroadcastText(t *testing.T) {
	manager := NewManager()
	server := NewServer(ServerConfig{Path: "/ws", ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second})
	server.HandleManaged("/ws", manager, func(ctx context.Context, conn *Conn) {
		<-ctx.Done()
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer ln.Close()
	if err := server.StartListener(ln); err != nil {
		t.Fatalf("StartListener() error = %v", err)
	}
	defer server.Stop(context.Background())

	client1 := NewClient(Config{URL: "ws://" + ln.Addr().String() + "/ws", Origin: "http://" + ln.Addr().String(), ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second})
	client2 := NewClient(Config{URL: "ws://" + ln.Addr().String() + "/ws", Origin: "http://" + ln.Addr().String(), ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client1.Connect(ctx); err != nil {
		t.Fatalf("client1.Connect() error = %v", err)
	}
	defer client1.Close()
	if err := client2.Connect(ctx); err != nil {
		t.Fatalf("client2.Connect() error = %v", err)
	}
	defer client2.Close()

	waitForCondition(t, time.Second, func() bool { return manager.Count() == 2 })
	if err := manager.BroadcastText(ctx, "broadcast"); err != nil {
		t.Fatalf("BroadcastText() error = %v", err)
	}

	_, payload1, err := client1.Receive(ctx)
	if err != nil {
		t.Fatalf("client1.Receive() error = %v", err)
	}
	_, payload2, err := client2.Receive(ctx)
	if err != nil {
		t.Fatalf("client2.Receive() error = %v", err)
	}
	if string(payload1) != "broadcast" || string(payload2) != "broadcast" {
		t.Fatalf("unexpected broadcast payloads: %s / %s", string(payload1), string(payload2))
	}
}

func TestClient_AutoReconnect(t *testing.T) {
	server := newTestWebsocketServer(t)
	defer server.Close()

	client := NewClient(Config{
		URL:                  wsURLFromHTTP(server.URL),
		Origin:               server.URL,
		ReadTimeout:          time.Second,
		WriteTimeout:         time.Second,
		AutoReconnect:        true,
		ReconnectInterval:    50 * time.Millisecond,
		MaxReconnectAttempts: 20,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer client.Close()

	client.handleConnectionError(errors.New("disconnect"))
	waitForCondition(t, 2*time.Second, func() bool { return client.IsConnected() })
}

func TestClient_Heartbeat(t *testing.T) {
	heartbeats := make(chan string, 2)
	server := httptest.NewServer(httpHandlerForHeartbeat(t, heartbeats))
	defer server.Close()

	client := NewClient(Config{
		URL:               wsURLFromHTTP(server.URL),
		Origin:            server.URL,
		ReadTimeout:       time.Second,
		WriteTimeout:      time.Second,
		HeartbeatInterval: 50 * time.Millisecond,
		HeartbeatMessage:  "hb",
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer client.Close()

	select {
	case msg := <-heartbeats:
		if msg != "hb" {
			t.Fatalf("expected hb, got %s", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("did not receive heartbeat message")
	}
}

func TestManagedGinHandler_Echo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	manager := NewManager()
	config := ServerConfig{ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second}
	engine.GET("/ws", ManagedGinHandler(config, manager, func(ctx context.Context, conn *Conn) {
		defer conn.Close()
		_, payload, err := conn.Receive(ctx)
		if err != nil {
			return
		}
		_ = conn.SendText(ctx, string(payload))
	}))
	server := httptest.NewServer(engine)
	defer server.Close()

	client := NewClient(Config{URL: wsURLFromHTTP(server.URL) + "/ws", Origin: server.URL, ReadTimeout: time.Second, WriteTimeout: time.Second})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer client.Close()

	waitForCondition(t, time.Second, func() bool { return manager.Count() == 1 })
	if err := client.SendText(ctx, "gin-echo"); err != nil {
		t.Fatalf("SendText() error = %v", err)
	}
	_, payload, err := client.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}
	if string(payload) != "gin-echo" {
		t.Fatalf("expected gin-echo, got %s", string(payload))
	}
}

func TestManager_ClusterBroadcastAndTargetedDelivery(t *testing.T) {
	bus := NewInMemoryClusterBus()
	managerA := NewManager()
	managerB := NewManager()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := managerA.EnableCluster(ctx, ClusterConfig{
		NodeID:    "node-a",
		Topic:     "ws.test.cluster",
		Transport: bus.Clone(),
	}); err != nil {
		t.Fatalf("managerA.EnableCluster() error = %v", err)
	}
	defer managerA.DisableCluster()

	if err := managerB.EnableCluster(ctx, ClusterConfig{
		NodeID:    "node-b",
		Topic:     "ws.test.cluster",
		Transport: bus.Clone(),
	}); err != nil {
		t.Fatalf("managerB.EnableCluster() error = %v", err)
	}
	defer managerB.DisableCluster()

	serverA := NewServer(ServerConfig{Path: "/ws", ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second})
	serverA.HandleManaged("/ws", managerA, func(ctx context.Context, conn *Conn) { <-ctx.Done() })
	lnA, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen node A error = %v", err)
	}
	defer lnA.Close()
	if err := serverA.StartListener(lnA); err != nil {
		t.Fatalf("serverA.StartListener() error = %v", err)
	}
	defer serverA.Stop(context.Background())

	serverB := NewServer(ServerConfig{Path: "/ws", ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second})
	serverB.HandleManaged("/ws", managerB, func(ctx context.Context, conn *Conn) { <-ctx.Done() })
	lnB, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen node B error = %v", err)
	}
	defer lnB.Close()
	if err := serverB.StartListener(lnB); err != nil {
		t.Fatalf("serverB.StartListener() error = %v", err)
	}
	defer serverB.Stop(context.Background())

	clientA := NewClient(Config{
		URL:         "ws://" + lnA.Addr().String() + "/ws",
		Origin:      "http://" + lnA.Addr().String(),
		ReadTimeout: 5 * time.Second,
		WriteTimeout: 5 * time.Second,
		Headers: map[string]string{
			"X-Client-ID": "client-a",
		},
	})
	if err := clientA.Connect(ctx); err != nil {
		t.Fatalf("clientA.Connect() error = %v", err)
	}
	defer clientA.Close()

	clientB := NewClient(Config{
		URL:         "ws://" + lnB.Addr().String() + "/ws",
		Origin:      "http://" + lnB.Addr().String(),
		ReadTimeout: 5 * time.Second,
		WriteTimeout: 5 * time.Second,
		Headers: map[string]string{
			"X-Client-ID": "client-b",
		},
	})
	if err := clientB.Connect(ctx); err != nil {
		t.Fatalf("clientB.Connect() error = %v", err)
	}
	defer clientB.Close()

	waitForCondition(t, time.Second, func() bool { return managerA.Count() == 1 && managerB.Count() == 1 })

	for _, conn := range managerA.snapshotConnections() {
		conn.mu.Lock()
		conn.identity.UserID = "user-a"
		conn.identity.ClientID = "client-a"
		conn.identity.Groups = []string{"group-a"}
		conn.mu.Unlock()
	}
	for _, conn := range managerB.snapshotConnections() {
		conn.mu.Lock()
		conn.identity.UserID = "user-b"
		conn.identity.ClientID = "client-b"
		conn.identity.Groups = []string{"group-b"}
		conn.mu.Unlock()
	}

	if err := managerA.BroadcastText(ctx, "cluster-all"); err != nil {
		t.Fatalf("managerA.BroadcastText() error = %v", err)
	}
	_, payloadA, err := clientA.Receive(ctx)
	if err != nil {
		t.Fatalf("clientA.Receive() error = %v", err)
	}
	_, payloadB, err := clientB.Receive(ctx)
	if err != nil {
		t.Fatalf("clientB.Receive() error = %v", err)
	}
	if string(payloadA) != "cluster-all" || string(payloadB) != "cluster-all" {
		t.Fatalf("unexpected cluster broadcast payloads: %s / %s", string(payloadA), string(payloadB))
	}

	if err := managerA.BroadcastToUser(ctx, "user-b", "hello-user-b"); err != nil {
		t.Fatalf("managerA.BroadcastToUser() error = %v", err)
	}
	_, targetedPayload, err := clientB.Receive(ctx)
	if err != nil {
		t.Fatalf("clientB.Receive() targeted error = %v", err)
	}
	if string(targetedPayload) != "hello-user-b" {
		t.Fatalf("expected targeted payload hello-user-b, got %s", string(targetedPayload))
	}
}
