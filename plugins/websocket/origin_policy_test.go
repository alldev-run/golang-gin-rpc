package websocket

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	nws "github.com/coder/websocket"
)

func TestServerOriginPolicy(t *testing.T) {
	for _, test := range []struct {
		name           string
		allowAnyOrigin bool
		wantAccepted   bool
	}{
		{name: "same-origin default rejects foreign origin"},
		{name: "explicit opt-in accepts foreign origin", allowAnyOrigin: true, wantAccepted: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := NewServer(ServerConfig{Path: "/ws", AllowAnyOrigin: test.allowAnyOrigin})
			server.Handle("/ws", func(ctx context.Context, conn *Conn) {
				defer conn.Close()
				_, _, _ = conn.Receive(ctx)
			})
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatalf("net.Listen() error = %v", err)
			}
			if err := server.StartListener(listener); err != nil {
				_ = listener.Close()
				t.Fatalf("StartListener() error = %v", err)
			}
			defer server.Stop(context.Background())

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			headers := make(http.Header)
			headers.Set("Origin", "http://foreign.example")
			conn, response, err := nws.Dial(ctx, "ws://"+listener.Addr().String()+"/ws", &nws.DialOptions{
				HTTPHeader: headers,
			})
			if test.wantAccepted {
				if err != nil {
					t.Fatalf("Dial() error = %v", err)
				}
				_ = conn.Close(nws.StatusNormalClosure, "test complete")
				return
			}
			if err == nil {
				_ = conn.Close(nws.StatusNormalClosure, "test complete")
				t.Fatal("Dial() accepted a foreign origin by default")
			}
			if response == nil || response.StatusCode != http.StatusForbidden {
				t.Fatalf("handshake response = %v, want HTTP 403", response)
			}
		})
	}
}
