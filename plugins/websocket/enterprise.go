package websocket

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type CloseCode string

const (
	CloseCodeNormal           CloseCode = "normal"
	CloseCodeAuthFailed       CloseCode = "auth_failed"
	CloseCodeHeartbeatTimeout CloseCode = "heartbeat_timeout"
	CloseCodeIdleTimeout      CloseCode = "idle_timeout"
	CloseCodeReadError        CloseCode = "read_error"
	CloseCodeWriteError       CloseCode = "write_error"
	CloseCodeServerShutdown   CloseCode = "server_shutdown"
	CloseCodeReconnectExhausted CloseCode = "reconnect_exhausted"
	CloseCodeInternalError    CloseCode = "internal_error"
)

type Identity struct {
	ConnectionID  string
	ClientID      string
	UserID        string
	TenantID      string
	Groups        []string
	Metadata      map[string]string
	RemoteAddr    string
	Path          string
	Authenticated bool
}

type AuthResult struct {
	ClientID string
	UserID   string
	TenantID string
	Groups   []string
	Metadata map[string]string
}

type Authenticator func(r *http.Request) (*AuthResult, error)

type Observer interface {
	OnConnect(identity Identity)
	OnDisconnect(identity Identity, code CloseCode, err error)
	OnAuthFailed(path string, err error)
	OnBroadcast(target string, count int, err error)
	OnMessage(direction string, identity Identity, size int)
	OnReconnectAttempt(url string, attempt int, delay time.Duration)
	OnHeartbeatTimeout(identity Identity)
}

type noopObserver struct{}

func (noopObserver) OnConnect(Identity)                              {}
func (noopObserver) OnDisconnect(Identity, CloseCode, error)         {}
func (noopObserver) OnAuthFailed(string, error)                      {}
func (noopObserver) OnBroadcast(string, int, error)                  {}
func (noopObserver) OnMessage(string, Identity, int)                 {}
func (noopObserver) OnReconnectAttempt(string, int, time.Duration)   {}
func (noopObserver) OnHeartbeatTimeout(Identity)                     {}

func observerOrNoop(observer Observer) Observer {
	if observer == nil {
		return noopObserver{}
	}
	return observer
}

type CompositeObserver struct {
	observers []Observer
}

func NewCompositeObserver(observers ...Observer) *CompositeObserver {
	filtered := make([]Observer, 0, len(observers))
	for _, observer := range observers {
		if observer != nil {
			filtered = append(filtered, observer)
		}
	}
	return &CompositeObserver{observers: filtered}
}

func (c *CompositeObserver) OnConnect(identity Identity) {
	for _, observer := range c.observers { observer.OnConnect(identity) }
}
func (c *CompositeObserver) OnDisconnect(identity Identity, code CloseCode, err error) {
	for _, observer := range c.observers { observer.OnDisconnect(identity, code, err) }
}
func (c *CompositeObserver) OnAuthFailed(path string, err error) {
	for _, observer := range c.observers { observer.OnAuthFailed(path, err) }
}
func (c *CompositeObserver) OnBroadcast(target string, count int, err error) {
	for _, observer := range c.observers { observer.OnBroadcast(target, count, err) }
}
func (c *CompositeObserver) OnMessage(direction string, identity Identity, size int) {
	for _, observer := range c.observers { observer.OnMessage(direction, identity, size) }
}
func (c *CompositeObserver) OnReconnectAttempt(url string, attempt int, delay time.Duration) {
	for _, observer := range c.observers { observer.OnReconnectAttempt(url, attempt, delay) }
}
func (c *CompositeObserver) OnHeartbeatTimeout(identity Identity) {
	for _, observer := range c.observers { observer.OnHeartbeatTimeout(identity) }
}

type MetricsSnapshot struct {
	ConnectionsOpened    uint64
	ConnectionsClosed    uint64
	AuthFailures         uint64
	Broadcasts           uint64
	MessagesSent         uint64
	MessagesReceived     uint64
	ReconnectAttempts    uint64
	HeartbeatTimeouts    uint64
}

type MetricsObserver struct {
	connectionsOpened uint64
	connectionsClosed uint64
	authFailures      uint64
	broadcasts        uint64
	messagesSent      uint64
	messagesReceived  uint64
	reconnectAttempts uint64
	heartbeatTimeouts uint64
}

func NewMetricsObserver() *MetricsObserver { return &MetricsObserver{} }
func (m *MetricsObserver) Snapshot() MetricsSnapshot {
	return MetricsSnapshot{
		ConnectionsOpened: atomic.LoadUint64(&m.connectionsOpened),
		ConnectionsClosed: atomic.LoadUint64(&m.connectionsClosed),
		AuthFailures: atomic.LoadUint64(&m.authFailures),
		Broadcasts: atomic.LoadUint64(&m.broadcasts),
		MessagesSent: atomic.LoadUint64(&m.messagesSent),
		MessagesReceived: atomic.LoadUint64(&m.messagesReceived),
		ReconnectAttempts: atomic.LoadUint64(&m.reconnectAttempts),
		HeartbeatTimeouts: atomic.LoadUint64(&m.heartbeatTimeouts),
	}
}
func (m *MetricsObserver) OnConnect(Identity) { atomic.AddUint64(&m.connectionsOpened, 1) }
func (m *MetricsObserver) OnDisconnect(Identity, CloseCode, error) { atomic.AddUint64(&m.connectionsClosed, 1) }
func (m *MetricsObserver) OnAuthFailed(string, error) { atomic.AddUint64(&m.authFailures, 1) }
func (m *MetricsObserver) OnBroadcast(string, int, error) { atomic.AddUint64(&m.broadcasts, 1) }
func (m *MetricsObserver) OnMessage(direction string, _ Identity, _ int) {
	if direction == "out" { atomic.AddUint64(&m.messagesSent, 1) } else { atomic.AddUint64(&m.messagesReceived, 1) }
}
func (m *MetricsObserver) OnReconnectAttempt(string, int, time.Duration) { atomic.AddUint64(&m.reconnectAttempts, 1) }
func (m *MetricsObserver) OnHeartbeatTimeout(Identity) { atomic.AddUint64(&m.heartbeatTimeouts, 1) }

type LoggingObserver struct{}

func NewLoggingObserver() *LoggingObserver { return &LoggingObserver{} }
func (o *LoggingObserver) OnConnect(identity Identity) {
	logger.Info("websocket connected", zap.String("connection_id", identity.ConnectionID), zap.String("client_id", identity.ClientID), zap.String("user_id", identity.UserID), zap.String("tenant_id", identity.TenantID), zap.String("path", identity.Path), zap.String("remote_addr", identity.RemoteAddr))
}
func (o *LoggingObserver) OnDisconnect(identity Identity, code CloseCode, err error) {
	fields := []zap.Field{zap.String("connection_id", identity.ConnectionID), zap.String("client_id", identity.ClientID), zap.String("user_id", identity.UserID), zap.String("tenant_id", identity.TenantID), zap.String("code", string(code))}
	if err != nil { fields = append(fields, zap.Error(err)) }
	logger.Info("websocket disconnected", fields...)
}
func (o *LoggingObserver) OnAuthFailed(path string, err error) {
	logger.Warn("websocket auth failed", zap.String("path", path), zap.Error(err))
}
func (o *LoggingObserver) OnBroadcast(target string, count int, err error) {
	fields := []zap.Field{zap.String("target", target), zap.Int("count", count)}
	if err != nil { fields = append(fields, zap.Error(err)) }
	logger.Info("websocket broadcast", fields...)
}
func (o *LoggingObserver) OnMessage(direction string, identity Identity, size int) {
	logger.Debug("websocket message", zap.String("direction", direction), zap.String("connection_id", identity.ConnectionID), zap.Int("size", size))
}
func (o *LoggingObserver) OnReconnectAttempt(url string, attempt int, delay time.Duration) {
	logger.Info("websocket reconnect attempt", zap.String("url", url), zap.Int("attempt", attempt), zap.Duration("delay", delay))
}
func (o *LoggingObserver) OnHeartbeatTimeout(identity Identity) {
	logger.Warn("websocket heartbeat timeout", zap.String("connection_id", identity.ConnectionID), zap.String("user_id", identity.UserID))
}

func newIdentity(r *http.Request, auth *AuthResult) Identity {
	identity := Identity{
		ConnectionID: uuid.NewString(),
		RemoteAddr:   r.RemoteAddr,
		Path:         r.URL.Path,
		Metadata:     map[string]string{},
	}
	if auth == nil {
		return identity
	}
	identity.ClientID = auth.ClientID
	identity.UserID = auth.UserID
	identity.TenantID = auth.TenantID
	identity.Groups = append(identity.Groups, auth.Groups...)
	identity.Authenticated = true
	for k, v := range auth.Metadata {
		identity.Metadata[k] = v
	}
	return identity
}

func classifyCloseCode(err error, fallback CloseCode) CloseCode {
	if err == nil {
		if fallback != "" { return fallback }
		return CloseCodeNormal
	}
	if errors.Is(err, context.DeadlineExceeded) {
		if fallback != "" { return fallback }
		return CloseCodeIdleTimeout
	}
	if fallback != "" {
		return fallback
	}
	return CloseCodeInternalError
}

func authError(path string, err error) error {
	return fmt.Errorf("websocket auth failed for %s: %w", path, err)
}
