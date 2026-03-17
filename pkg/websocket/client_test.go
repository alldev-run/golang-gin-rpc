package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	nws "nhooyr.io/websocket"
)

func newTestWebsocketServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := nws.Accept(w, r, &nws.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer conn.Close(nws.StatusNormalClosure, "")
		for {
			typ, payload, err := conn.Read(r.Context())
			if err != nil {
				return
			}
			if err := conn.Write(r.Context(), typ, payload); err != nil {
				return
			}
		}
	}))
}

func wsURLFromHTTP(httpURL string) string {
	return "ws" + httpURL[len("http"):]
}

func TestClient_SendReceiveText(t *testing.T) {
	server := newTestWebsocketServer(t)
	defer server.Close()

	client := NewClient(Config{URL: wsURLFromHTTP(server.URL), Origin: server.URL})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer client.Close()

	if err := client.SendText(ctx, "hello"); err != nil {
		t.Fatalf("SendText() error = %v", err)
	}

	frameType, payload, err := client.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}
	if frameType != int(nws.MessageText) {
		t.Fatalf("expected text frame, got %d", frameType)
	}
	if string(payload) != "hello" {
		t.Fatalf("expected payload hello, got %s", string(payload))
	}
}

func TestClient_SendReceiveJSON(t *testing.T) {
	server := newTestWebsocketServer(t)
	defer server.Close()

	client := NewClient(Config{URL: wsURLFromHTTP(server.URL), Origin: server.URL})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer client.Close()

	message := map[string]interface{}{"type": "ping", "count": 1}
	if err := client.SendJSON(ctx, message); err != nil {
		t.Fatalf("SendJSON() error = %v", err)
	}

	var received map[string]interface{}
	if err := client.ReceiveJSON(ctx, &received); err != nil {
		t.Fatalf("ReceiveJSON() error = %v", err)
	}

	encodedSent, _ := json.Marshal(message)
	encodedReceived, _ := json.Marshal(received)
	if string(encodedSent) != string(encodedReceived) {
		t.Fatalf("expected %s, got %s", string(encodedSent), string(encodedReceived))
	}
}

func TestClient_NotConnected(t *testing.T) {
	client := NewClient(DefaultConfig())
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := client.SendText(ctx, "hello"); err == nil {
		t.Fatal("expected SendText() to fail when not connected")
	}
	if _, _, err := client.Receive(ctx); err == nil {
		t.Fatal("expected Receive() to fail when not connected")
	}
}
