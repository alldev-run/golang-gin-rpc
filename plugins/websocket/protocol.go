package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	nws "github.com/coder/websocket"
)

// 协议层:统一的 WebSocket 消息信封与按连接的有序投递。
//
// 客户端 → 服务端:{ "action": "chat.say", "req_id": "r1", "data": {...} }
// 服务端 → 客户端:{ "seq": 42, "action": "chat.say", "req_id": "r1",
//                  "code": 0, "data": {...} }
//
// 约定:
//   - Action 是路由键(服务端 Router 按它分发;客户端按它处理推送)。
//   - ReqID 用于请求-响应关联,服务端处理器通过 Conn.Reply 原样回填。
//   - Seq 是服务端为该连接分配的单调递增序号,配合会话重放缓冲实现
//     重连补发(见 session.go)。广播消息同样按连接逐一盖 Seq。
//   - Code != 0 表示业务失败,Err 携带可读错误信息。

// 应用层结果码约定;业务可扩展自己的正数错误码。
const (
	CodeOK    = 0
	CodeError = 1
)

// Envelope is the standard message wrapper exchanged on the wire.
type Envelope struct {
	// Seq is the per-connection monotonic outbound sequence (server→client only).
	Seq uint64 `json:"seq,omitempty"`
	// Action is the routing key.
	Action string `json:"action"`
	// ReqID correlates a response to its request.
	ReqID string `json:"req_id,omitempty"`
	// Code is the application result code; 0 means success.
	Code int `json:"code,omitempty"`
	// Err carries a human readable message when Code != 0.
	Err string `json:"err,omitempty"`
	// Data is the opaque JSON payload.
	Data json.RawMessage `json:"data,omitempty"`
}

var (
	errEnvelopeNil    = errors.New("websocket: envelope is nil")
	errEnvelopeAction = errors.New("websocket: envelope action is empty")
	errNotTextFrame   = errors.New("websocket: expected text frame for envelope")
	errUnknownAction  = errors.New("websocket: unknown action")
)

// NewEnvelope builds an envelope, marshalling data into the JSON payload.
func NewEnvelope(action string, data interface{}) (*Envelope, error) {
	if action == "" {
		return nil, errEnvelopeAction
	}
	env := &Envelope{Action: action}
	if data != nil {
		raw, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		env.Data = raw
	}
	return env, nil
}

// SendEnvelope stamps env with the per-connection outbound sequence and queues
// it. Sequence assignment and queueing are atomic per connection so observed
// order matches sequence order even under concurrent broadcasts.
func (c *Conn) SendEnvelope(ctx context.Context, env *Envelope) error {
	if env == nil {
		return errEnvelopeNil
	}
	if env.Action == "" {
		return errEnvelopeAction
	}
	c.seqMu.Lock()
	if c.session != nil {
		env.Seq = c.session.nextSeq()
	} else {
		env.Seq = c.outSeq.Add(1)
	}
	data, err := json.Marshal(env)
	if err == nil {
		if c.session != nil {
			c.session.store(env.Seq, data)
		}
		err = c.send(ctx, data, byte(nws.MessageText))
	}
	c.seqMu.Unlock()
	return err
}

// Reply sends a response envelope correlated to the request. A nil appErr
// yields Code=0 with data; a non-nil appErr yields Code=CodeError and Err.
func (c *Conn) Reply(ctx context.Context, req *Envelope, data interface{}, appErr error) error {
	if req == nil {
		return errEnvelopeNil
	}
	resp := &Envelope{Action: req.Action, ReqID: req.ReqID}
	if appErr != nil {
		resp.Code = CodeError
		resp.Err = appErr.Error()
	} else if data != nil {
		raw, err := json.Marshal(data)
		if err != nil {
			return err
		}
		resp.Data = raw
	}
	return c.SendEnvelope(ctx, resp)
}

// ReceiveEnvelope reads the next text frame and parses it as an Envelope.
func (c *Conn) ReceiveEnvelope(ctx context.Context) (*Envelope, error) {
	msgType, payload, err := c.Receive(ctx)
	if err != nil {
		return nil, err
	}
	if msgType != int(nws.MessageText) {
		return nil, errNotTextFrame
	}
	var env Envelope
	if err := json.Unmarshal(payload, &env); err != nil {
		return nil, err
	}
	return &env, nil
}

// Router dispatches inbound Envelopes by Action, sequentially per connection
// (in-order processing). Handlers doing slow work should spawn goroutines.
type Router struct {
	mu       sync.RWMutex
	handlers map[string]ActionHandler
	fallback ActionHandler
}

// ActionHandler processes one inbound envelope on the given connection.
type ActionHandler func(ctx context.Context, conn *Conn, env *Envelope)

// NewRouter creates an empty router.
func NewRouter() *Router {
	return &Router{handlers: make(map[string]ActionHandler)}
}

// Handle registers a handler for an action; re-registering replaces it.
func (r *Router) Handle(action string, h ActionHandler) {
	if action == "" || h == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[action] = h
}

// HandleDefault registers the handler for unknown actions; nil disables it
// (unknown actions with a ReqID then receive an errUnknownAction reply).
func (r *Router) HandleDefault(h ActionHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fallback = h
}

func (r *Router) lookup(action string) ActionHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if h, ok := r.handlers[action]; ok {
		return h
	}
	return r.fallback
}

// Handler returns a websocket.Handler implementing the envelope read loop.
// It returns when the connection closes. Inbound rate limiting (if configured)
// and heartbeat marking happen inside conn.Receive.
func (r *Router) Handler() Handler {
	return func(ctx context.Context, conn *Conn) {
		for {
			env, err := conn.ReceiveEnvelope(ctx)
			if err != nil {
				return // read error / closed / rate limited
			}
			if env.Action == "" {
				_ = conn.Reply(ctx, env, nil, errEnvelopeAction)
				continue
			}
			h := r.lookup(env.Action)
			if h == nil {
				_ = conn.Reply(ctx, env, nil, errUnknownAction)
				continue
			}
			h(ctx, conn, env)
		}
	}
}
