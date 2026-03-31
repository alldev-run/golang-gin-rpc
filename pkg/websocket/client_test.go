package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	userpb "github.com/alldev-run/golang-gin-rpc/proto"
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

func TestClient_SendReceiveProto(t *testing.T) {
	server := newTestWebsocketServer(t)
	defer server.Close()

	client := NewClient(Config{URL: wsURLFromHTTP(server.URL), Origin: server.URL})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer client.Close()

	msg := &userpb.UserRequest{Id: 42}
	if err := client.SendProto(ctx, msg); err != nil {
		t.Fatalf("SendProto() error = %v", err)
	}

	var got userpb.UserRequest
	if err := client.ReceiveProto(ctx, &got); err != nil {
		t.Fatalf("ReceiveProto() error = %v", err)
	}

	if got.Id != 42 {
		t.Fatalf("expected id 42, got %d", got.Id)
	}
}

func TestClient_SendReceiveMessage_JSONByConfig(t *testing.T) {
	server := newTestWebsocketServer(t)
	defer server.Close()

	client := NewClient(Config{URL: wsURLFromHTTP(server.URL), Origin: server.URL, MessageFormat: MessageFormatJSON})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer client.Close()

	sent := map[string]interface{}{"type": "ping", "count": float64(7)}
	if err := client.SendMessage(ctx, sent); err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}

	var got map[string]interface{}
	if err := client.ReceiveMessage(ctx, &got); err != nil {
		t.Fatalf("ReceiveMessage() error = %v", err)
	}

	if got["type"] != "ping" {
		t.Fatalf("expected type ping, got %v", got["type"])
	}
}

func TestClient_SendReceiveMessage_ProtoByConfig(t *testing.T) {
	server := newTestWebsocketServer(t)
	defer server.Close()

	client := NewClient(Config{URL: wsURLFromHTTP(server.URL), Origin: server.URL, MessageFormat: MessageFormatProtobuf})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer client.Close()

	sent := &userpb.UserRequest{Id: 99}
	if err := client.SendMessage(ctx, sent); err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}

	var got userpb.UserRequest
	if err := client.ReceiveMessage(ctx, &got); err != nil {
		t.Fatalf("ReceiveMessage() error = %v", err)
	}

	if got.Id != 99 {
		t.Fatalf("expected id 99, got %d", got.Id)
	}
}

func TestClient_ReceiveMessage_ProtoByConfigFrameTypeMismatch(t *testing.T) {
	server := newTestWebsocketServer(t)
	defer server.Close()

	client := NewClient(Config{URL: wsURLFromHTTP(server.URL), Origin: server.URL, MessageFormat: MessageFormatProtobuf})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer client.Close()

	if err := client.SendText(ctx, "text-frame"); err != nil {
		t.Fatalf("SendText() error = %v", err)
	}

	var got userpb.UserRequest
	if err := client.ReceiveMessage(ctx, &got); !IsProtobufFrameTypeMismatchError(err) {
		t.Fatalf("expected protobuf frame type mismatch error, got err=%v", err)
	}
}

func TestClient_SendReceiveMessage_ProtoByConfigTypeMismatch(t *testing.T) {
	client := NewClient(Config{MessageFormat: MessageFormatProtobuf})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := client.SendMessage(ctx, map[string]interface{}{"type": "not-proto"}); !IsProtobufPayloadTypeMismatchError(err) {
		t.Fatalf("expected protobuf payload type mismatch error, got err=%v", err)
	}

	var nonProto map[string]interface{}
	if err := client.ReceiveMessage(ctx, &nonProto); !IsProtobufDestinationTypeMismatchError(err) {
		t.Fatalf("expected protobuf destination type mismatch error, got err=%v", err)
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
