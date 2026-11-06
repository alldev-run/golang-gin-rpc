package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// 请求-应答:基于通用 Client 接口(Publish/Subscribe)实现的 RPC 风格消息模式。
//
// 请求方:Request 把 payload 封进 ReplyEnvelope(携带 correlation_id 与
// reply_to 临时主题)发布到目标主题,并订阅 reply_to 等待响应。
// 应答方:HandleRequests 把业务函数包装成 MessageHandler——解出信封、调用
// 业务函数、把结果(或错误)发布回 reply_to。
//
// 适配说明:NATS/RabbitMQ 上开箱即用;Kafka 的 reply 临时主题通常需要预创建
// 或允许自动建题,请按部署环境配置。

var (
	// ErrRequestTimeout 在超时内未收到响应时返回。
	ErrRequestTimeout = errors.New("messaging: request timeout")
	// ErrReplyMalformed 在响应信封无法解析时返回。
	ErrReplyMalformed = errors.New("messaging: malformed reply envelope")
)

// ReplyEnvelope 是请求与响应共用的信封。
type ReplyEnvelope struct {
	CorrelationID string `json:"correlation_id"`
	// ReplyTo 仅在请求中出现,指向响应应发布的临时主题。
	ReplyTo string `json:"reply_to,omitempty"`
	// IsError 表示响应承载的是业务错误信息而非正常负载。
	IsError bool   `json:"is_error,omitempty"`
	Payload []byte `json:"payload"`
}

// ReplyError 把响应中的业务错误还原为 error。
type ReplyError struct {
	Payload []byte
}

func (e *ReplyError) Error() string {
	return fmt.Sprintf("messaging: remote error: %s", string(e.Payload))
}

// Request 发送一次请求并等待响应,返回响应负载。
// 远端业务错误以 *ReplyError 返回;超时以 ErrRequestTimeout 返回。
func Request(ctx context.Context, client Client, topic string, payload []byte, timeout time.Duration) ([]byte, error) {
	if client == nil {
		return nil, errors.New("messaging: client is nil")
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	correlationID := uuid.NewString()
	replyTo := fmt.Sprintf("reply.%s", correlationID)

	got := make(chan *ReplyEnvelope, 1)
	subCtx, cancelSub := context.WithCancel(ctx)
	defer cancelSub()
	if err := client.Subscribe(subCtx, replyTo, func(_ context.Context, msg *Message) error {
		var env ReplyEnvelope
		if err := json.Unmarshal(msg.Payload, &env); err != nil {
			return nil // 非信封消息,忽略
		}
		select {
		case got <- &env:
		default:
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("messaging: subscribe reply topic: %w", err)
	}

	reqEnv, err := json.Marshal(ReplyEnvelope{
		CorrelationID: correlationID,
		ReplyTo:       replyTo,
		Payload:       payload,
	})
	if err != nil {
		return nil, err
	}
	if err := client.Publish(ctx, topic, &Message{
		Topic:     topic,
		Payload:   reqEnv,
		Timestamp: time.Now(),
	}); err != nil {
		return nil, fmt.Errorf("messaging: publish request: %w", err)
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case env := <-got:
		if env.IsError {
			return env.Payload, &ReplyError{Payload: env.Payload}
		}
		return env.Payload, nil
	case <-timer.C:
		return nil, ErrRequestTimeout
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// HandleRequests 把业务请求函数包装成 MessageHandler:解信封 → 执行 → 回信。
// fn 返回的 error 以 IsError 信封回传,不再向上抛(避免应答通道被错误重试放大)。
func HandleRequests(client Client, fn func(ctx context.Context, payload []byte) ([]byte, error)) MessageHandler {
	return func(ctx context.Context, msg *Message) error {
		if msg == nil || len(msg.Payload) == 0 {
			return nil
		}
		var env ReplyEnvelope
		if err := json.Unmarshal(msg.Payload, &env); err != nil || env.ReplyTo == "" {
			return nil // 非请求信封,忽略
		}
		resp := ReplyEnvelope{CorrelationID: env.CorrelationID}
		out, err := fn(ctx, env.Payload)
		if err != nil {
			resp.IsError = true
			resp.Payload = []byte(err.Error())
		} else {
			resp.Payload = out
		}
		raw, mErr := json.Marshal(resp)
		if mErr != nil {
			return mErr
		}
		return client.Publish(ctx, env.ReplyTo, &Message{
			Topic:     env.ReplyTo,
			Payload:   raw,
			Timestamp: time.Now(),
		})
	}
}

// RequestJSON 是 Request 的 JSON 便捷方法:请求与响应均为任意 JSON 值。
func RequestJSON(ctx context.Context, client Client, topic string, req interface{}, timeout time.Duration) ([]byte, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	return Request(ctx, client, topic, payload, timeout)
}
