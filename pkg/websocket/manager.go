package websocket

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
)

type Manager struct {
	mu            sync.RWMutex
	conns         map[*Conn]struct{}
	byConnID      map[string]*Conn
	clusterMu     sync.RWMutex
	clusterConfig ClusterConfig
	clusterCtx    context.Context
	clusterCancel context.CancelFunc
}

func NewManager() *Manager {
	return &Manager{
		conns:    make(map[*Conn]struct{}),
		byConnID: make(map[string]*Conn),
	}
}

func (m *Manager) EnableCluster(ctx context.Context, config ClusterConfig) error {
	if m == nil {
		return nil
	}
	if config.NodeID == "" {
		config.NodeID = uuid.NewString()
	}
	if config.Topic == "" {
		config.Topic = DefaultClusterConfig().Topic
	}
	if config.Transport == nil {
		return nil
	}
	m.clusterMu.Lock()
	if m.clusterCancel != nil {
		m.clusterCancel()
	}
	clusterCtx, cancel := context.WithCancel(ctx)
	m.clusterCtx = clusterCtx
	m.clusterCancel = cancel
	m.clusterConfig = config
	m.clusterMu.Unlock()
	return config.Transport.Subscribe(clusterCtx, config.Topic, func(msgCtx context.Context, payload []byte) error {
		return m.handleClusterMessage(msgCtx, payload)
	})
}

func (m *Manager) DisableCluster() error {
	if m == nil {
		return nil
	}
	m.clusterMu.Lock()
	cancel := m.clusterCancel
	transport := m.clusterConfig.Transport
	m.clusterCancel = nil
	m.clusterCtx = nil
	m.clusterConfig = ClusterConfig{}
	m.clusterMu.Unlock()
	if cancel != nil {
		cancel()
	}
	if transport != nil {
		return transport.Close()
	}
	return nil
}

func (m *Manager) Register(conn *Conn) {
	if m == nil || conn == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.conns[conn] = struct{}{}
	identity := conn.Identity()
	if identity.ConnectionID != "" {
		m.byConnID[identity.ConnectionID] = conn
	}
}

func (m *Manager) Unregister(conn *Conn) {
	if m == nil || conn == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.conns, conn)
	identity := conn.Identity()
	if identity.ConnectionID != "" {
		delete(m.byConnID, identity.ConnectionID)
	}
}

func (m *Manager) Count() int {
	if m == nil {
		return 0
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.conns)
}

func (m *Manager) BroadcastText(ctx context.Context, message string) error {
	traceCtx, span := startWebsocketSpan(ctx, nil, "websocket.manager.broadcast.text",
		attribute.Int("websocket.connection_count", m.Count()),
		attribute.Int("websocket.message_size", len(message)),
	)
	err := m.broadcast(ctx, func(conn *Conn) error {
		return conn.SendText(traceCtx, message)
	})
	if err == nil {
		err = m.publishClusterEvent(ctx, clusterEnvelope{
			ID:        uuid.NewString(),
			Type:      "broadcast_text",
			Target:    "all",
			Payload:   []byte(message),
			CreatedAt: time.Now(),
		})
	}
	m.observeBroadcast("all", m.Count(), err)
	endSpan(span, err)
	return err
}

func (m *Manager) BroadcastBinary(ctx context.Context, payload []byte) error {
	traceCtx, span := startWebsocketSpan(ctx, nil, "websocket.manager.broadcast.binary",
		attribute.Int("websocket.connection_count", m.Count()),
		attribute.Int("websocket.message_size", len(payload)),
	)
	err := m.broadcast(ctx, func(conn *Conn) error {
		return conn.SendBinary(traceCtx, payload)
	})
	if err == nil {
		err = m.publishClusterEvent(ctx, clusterEnvelope{
			ID:        uuid.NewString(),
			Type:      "broadcast_binary",
			Target:    "all",
			Payload:   append([]byte(nil), payload...),
			CreatedAt: time.Now(),
		})
	}
	m.observeBroadcast("all", m.Count(), err)
	endSpan(span, err)
	return err
}

func (m *Manager) BroadcastJSON(ctx context.Context, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return m.BroadcastText(ctx, string(data))
}

func (m *Manager) BroadcastToGroup(ctx context.Context, group string, message string) error {
	conns := m.connectionsByFilter(func(identity Identity) bool {
		for _, item := range identity.Groups {
			if item == group {
				return true
			}
		}
		return false
	})
	traceCtx, span := startWebsocketSpan(ctx, nil, "websocket.manager.broadcast.group",
		attribute.String("websocket.group", group),
		attribute.Int("websocket.connection_count", len(conns)),
	)
	err := m.broadcastSnapshot(ctx, conns, func(conn *Conn) error {
		return conn.SendText(traceCtx, message)
	})
	if err == nil {
		err = m.publishClusterEvent(ctx, clusterEnvelope{
			ID:        uuid.NewString(),
			Type:      "broadcast_group_text",
			Target:    group,
			Payload:   []byte(message),
			CreatedAt: time.Now(),
		})
	}
	m.observeBroadcast("group:"+group, len(conns), err)
	endSpan(span, err)
	return err
}

func (m *Manager) BroadcastToUser(ctx context.Context, userID string, message string) error {
	conns := m.connectionsByFilter(func(identity Identity) bool {
		return identity.UserID == userID
	})
	traceCtx, span := startWebsocketSpan(ctx, nil, "websocket.manager.broadcast.user",
		attribute.String("websocket.user_id", userID),
		attribute.Int("websocket.connection_count", len(conns)),
	)
	err := m.broadcastSnapshot(ctx, conns, func(conn *Conn) error {
		return conn.SendText(traceCtx, message)
	})
	if err == nil {
		err = m.publishClusterEvent(ctx, clusterEnvelope{
			ID:        uuid.NewString(),
			Type:      "broadcast_user_text",
			Target:    userID,
			Payload:   []byte(message),
			CreatedAt: time.Now(),
		})
	}
	m.observeBroadcast("user:"+userID, len(conns), err)
	endSpan(span, err)
	return err
}

func (m *Manager) BroadcastToClient(ctx context.Context, clientID string, message string) error {
	conns := m.connectionsByFilter(func(identity Identity) bool {
		return identity.ClientID == clientID
	})
	traceCtx, span := startWebsocketSpan(ctx, nil, "websocket.manager.broadcast.client",
		attribute.String("websocket.client_id", clientID),
		attribute.Int("websocket.connection_count", len(conns)),
	)
	err := m.broadcastSnapshot(ctx, conns, func(conn *Conn) error {
		return conn.SendText(traceCtx, message)
	})
	if err == nil {
		err = m.publishClusterEvent(ctx, clusterEnvelope{
			ID:        uuid.NewString(),
			Type:      "broadcast_client_text",
			Target:    clientID,
			Payload:   []byte(message),
			CreatedAt: time.Now(),
		})
	}
	m.observeBroadcast("client:"+clientID, len(conns), err)
	endSpan(span, err)
	return err
}

func (m *Manager) SendToConnection(ctx context.Context, connectionID string, message string) error {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	conn := m.byConnID[connectionID]
	m.mu.RUnlock()
	traceCtx, span := startWebsocketSpan(ctx, nil, "websocket.manager.broadcast.connection",
		attribute.String("websocket.connection_id", connectionID),
	)
	if conn != nil {
		if err := conn.SendText(traceCtx, message); err != nil {
			m.observeBroadcast("connection:"+connectionID, 1, err)
			endSpan(span, err)
			return err
		}
	}
	err := m.publishClusterEvent(ctx, clusterEnvelope{
		ID:        uuid.NewString(),
		Type:      "send_connection_text",
		Target:    connectionID,
		Payload:   []byte(message),
		CreatedAt: time.Now(),
	})
	m.observeBroadcast("connection:"+connectionID, 1, err)
	endSpan(span, err)
	return err
}

func (m *Manager) CloseAll() error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	conns := make([]*Conn, 0, len(m.conns))
	for conn := range m.conns {
		conns = append(conns, conn)
	}
	m.conns = make(map[*Conn]struct{})
	m.byConnID = make(map[string]*Conn)
	m.mu.Unlock()
	for _, conn := range conns {
		_ = conn.CloseWithCode(CloseCodeServerShutdown, nil)
	}
	return m.DisableCluster()
}

func (m *Manager) broadcast(ctx context.Context, fn func(*Conn) error) error {
	if m == nil {
		return nil
	}
	return m.broadcastSnapshot(ctx, m.snapshotConnections(), fn)
}

func (m *Manager) broadcastSnapshot(ctx context.Context, conns []*Conn, fn func(*Conn) error) error {
	var firstErr error
	for _, conn := range conns {
		if err := fn(conn); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (m *Manager) connectionsByFilter(fn func(Identity) bool) []*Conn {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	conns := make([]*Conn, 0, len(m.conns))
	for conn := range m.conns {
		if fn(conn.Identity()) {
			conns = append(conns, conn)
		}
	}
	return conns
}

func (m *Manager) snapshotConnections() []*Conn {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	conns := make([]*Conn, 0, len(m.conns))
	for conn := range m.conns {
		conns = append(conns, conn)
	}
	return conns
}

func (m *Manager) observeBroadcast(target string, count int, err error) {
	if m == nil {
		return
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for conn := range m.conns {
		conn.observer.OnBroadcast(target, count, err)
		break
	}
}

func (m *Manager) publishClusterEvent(ctx context.Context, envelope clusterEnvelope) error {
	if m == nil {
		return nil
	}
	m.clusterMu.RLock()
	config := m.clusterConfig
	m.clusterMu.RUnlock()
	if config.Transport == nil {
		return nil
	}
	envelope.SourceNode = config.NodeID
	payload, err := marshalClusterEnvelope(envelope)
	if err != nil {
		return err
	}
	return config.Transport.Publish(ctx, config.Topic, payload)
}

func (m *Manager) handleClusterMessage(ctx context.Context, payload []byte) error {
	if m == nil {
		return nil
	}
	envelope, err := unmarshalClusterEnvelope(payload)
	if err != nil {
		return err
	}
	m.clusterMu.RLock()
	config := m.clusterConfig
	m.clusterMu.RUnlock()
	if config.NodeID != "" && envelope.SourceNode == config.NodeID {
		return nil
	}
	switch envelope.Type {
	case "broadcast_text":
		return m.broadcastSnapshot(ctx, m.snapshotConnections(), func(conn *Conn) error {
			return conn.SendText(ctx, string(envelope.Payload))
		})
	case "broadcast_binary":
		return m.broadcastSnapshot(ctx, m.snapshotConnections(), func(conn *Conn) error {
			return conn.SendBinary(ctx, envelope.Payload)
		})
	case "broadcast_group_text":
		return m.broadcastSnapshot(ctx, m.connectionsByFilter(func(identity Identity) bool {
			for _, group := range identity.Groups {
				if group == envelope.Target {
					return true
				}
			}
			return false
		}), func(conn *Conn) error {
			return conn.SendText(ctx, string(envelope.Payload))
		})
	case "broadcast_user_text":
		return m.broadcastSnapshot(ctx, m.connectionsByFilter(func(identity Identity) bool {
			return identity.UserID == envelope.Target
		}), func(conn *Conn) error {
			return conn.SendText(ctx, string(envelope.Payload))
		})
	case "broadcast_client_text":
		return m.broadcastSnapshot(ctx, m.connectionsByFilter(func(identity Identity) bool {
			return identity.ClientID == envelope.Target
		}), func(conn *Conn) error {
			return conn.SendText(ctx, string(envelope.Payload))
		})
	case "send_connection_text":
		m.mu.RLock()
		conn := m.byConnID[envelope.Target]
		m.mu.RUnlock()
		if conn == nil {
			return nil
		}
		return conn.SendText(ctx, string(envelope.Payload))
	default:
		return nil
	}
}
