package websocket

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestServer_ClientEcho(t *testing.T) {
	server := NewServer(ServerConfig{Path: "/ws", ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second})
	server.Handle("/ws", func(ctx context.Context, conn *Conn) {
		defer conn.Close()
		_, payload, err := conn.Receive(ctx)
		if err != nil {
			return
		}
		_ = conn.SendText(ctx, string(payload))
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

	client := NewClient(Config{
		URL:          "ws://" + ln.Addr().String() + "/ws",
		Origin:       "http://" + ln.Addr().String(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer client.Close()

	if err := client.SendText(ctx, "hello-server"); err != nil {
		t.Fatalf("SendText() error = %v", err)
	}

	_, payload, err := client.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}
	if string(payload) != "hello-server" {
		t.Fatalf("expected hello-server, got %s", string(payload))
	}
}

func TestServer_AddrBeforeAndAfterStart(t *testing.T) {
	server := NewServer(ServerConfig{Addr: ":18080", Path: "/ws"})
	if server.Addr() == "" {
		t.Fatal("expected Addr() to be non-empty before start")
	}
}
