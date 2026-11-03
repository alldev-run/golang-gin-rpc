package alert

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

type captureAlertChannel struct {
	name   string
	alerts chan Alert
	err    error
}

func (c *captureAlertChannel) Name() string {
	return c.name
}

func (c *captureAlertChannel) Send(_ context.Context, alert Alert) error {
	if c.err != nil {
		return c.err
	}
	c.alerts <- alert
	return nil
}

type captureLogger struct {
	message string
}

func (l *captureLogger) Printf(format string, args ...any) {
	l.message = fmt.Sprintf(format, args...)
}

func TestWebhookChannelSend(t *testing.T) {
	want := Alert{
		ID:      "alert-1",
		Level:   LevelError,
		Title:   "database",
		Message: "connection failed",
	}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", got)
		}
		if got := r.Header.Get("X-Alert-Token"); got != "secret" {
			t.Errorf("X-Alert-Token = %q, want secret", got)
		}
		var got Alert
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("request alert = %+v, want %+v", got, want)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	channel := NewWebhookChannel("webhook", server.URL, map[string]string{"X-Alert-Token": "secret"})
	if channel.client.Timeout != 10*time.Second {
		t.Fatalf("default client timeout = %v, want 10s", channel.client.Timeout)
	}
	if err := channel.Send(context.Background(), want); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if requests != 1 {
		t.Errorf("requests = %d, want 1", requests)
	}
}

func TestWebhookChannelSendNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	channel := NewWebhookChannel("webhook", server.URL, nil)
	err := channel.Send(context.Background(), Alert{Title: "unavailable"})
	if err == nil || !strings.Contains(err.Error(), "503 Service Unavailable") {
		t.Fatalf("Send() error = %v, want HTTP 503 status", err)
	}
}

func TestWebhookChannelSendCanceledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("handler called for canceled request")
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	channel := NewWebhookChannelWithClient("webhook", server.URL, nil, server.Client())
	if err := channel.Send(ctx, Alert{}); err == nil {
		t.Fatal("Send() with canceled context returned nil")
	}
}

func TestLogChannelSend(t *testing.T) {
	logger := &captureLogger{}
	channel := NewLogChannel("log", logger)
	if err := channel.Send(context.Background(), Alert{Level: LevelWarning, Title: "warning"}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if logger.message == "" {
		t.Error("logger did not receive an alert")
	}
}

func TestManagerSend(t *testing.T) {
	manager := NewManager(Config{Enabled: true})
	defer manager.Close()

	channel := &captureAlertChannel{name: "capture", alerts: make(chan Alert, 1)}
	manager.RegisterChannel(channel)
	handled := make(chan Alert, 1)
	manager.OnAlert(LevelError, func(alert Alert) { handled <- alert })

	if err := manager.Send(context.Background(), Alert{Level: LevelError, Title: "database"}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	sent := <-channel.alerts
	if sent.ID == "" || sent.Timestamp.IsZero() {
		t.Errorf("Send() did not populate ID and timestamp: %+v", sent)
	}
	if handledAlert := <-handled; handledAlert.ID != sent.ID {
		t.Errorf("handler alert ID = %q, want %q", handledAlert.ID, sent.ID)
	}
	manager.UnregisterChannel(channel.Name())
}

func TestManagerSendChannelError(t *testing.T) {
	manager := NewManager(Config{Enabled: true})
	defer manager.Close()
	manager.RegisterChannel(&captureAlertChannel{name: "broken", err: errors.New("send failed")})

	err := manager.Send(context.Background(), Alert{Level: LevelError})
	if err == nil || !strings.Contains(err.Error(), "failed to send to broken") {
		t.Fatalf("Send() error = %v, want channel error", err)
	}
}

func TestManagerQueueAndClose(t *testing.T) {
	manager := NewManager(Config{
		Enabled:       true,
		BufferSize:    10,
		FlushInterval: time.Hour,
	})
	channel := &captureAlertChannel{name: "capture", alerts: make(chan Alert, 10)}
	manager.RegisterChannel(channel)

	manager.Debug("debug", "message")
	manager.Info("info", "message", map[string]string{"service": "api"})
	manager.Warning("warning", "message")
	manager.Error("error", "message")
	manager.Critical("critical", "message")

	if err := manager.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	manager.Queue(Alert{Level: LevelInfo, Title: "after close"})

	got := make(map[Level]int)
	for len(channel.alerts) > 0 {
		got[(<-channel.alerts).Level]++
	}
	for _, level := range []Level{LevelDebug, LevelInfo, LevelWarning, LevelError, LevelCritical} {
		if got[level] != 1 {
			t.Errorf("queued %s alerts = %d, want 1", level, got[level])
		}
	}
	if gotCount := len(channel.alerts); gotCount != 0 {
		t.Errorf("alerts after close = %d, want 0", gotCount)
	}
}

func TestManagerDisabled(t *testing.T) {
	config := DefaultConfig()
	if !config.Enabled || config.BufferSize <= 0 || config.FlushInterval <= 0 {
		t.Fatalf("DefaultConfig() = %+v", config)
	}

	manager := NewManager(Config{})
	if err := manager.Send(context.Background(), Alert{}); err != nil {
		t.Fatalf("disabled Send() error = %v", err)
	}
	manager.Queue(Alert{})
	if err := manager.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
