// Package actor 提供一个轻量级、基于 goroutine + channel 的 Actor 模型实现。
//
// 每个 Actor 拥有独立的邮箱（mailbox），消息按到达顺序串行处理，
// 天然具备并发安全和容错能力。支持生命周期钩子、行为切换、优雅关闭。
package actor

import (
	stdctx "context"
)

// Message 是 Actor 之间传递的消息，使用空接口以保持最大灵活性。
type Message interface{}

// PID 唯一标识一个 Actor 进程。
type PID string

// Envelope 是邮箱中的消息信封，携带发送方信息。
type Envelope struct {
	Sender  PID
	Message Message
}

// Context 是 Actor 处理消息时可使用的上下文。
//
// 所有方法均可在 Actor 的 Receive 生命周期中安全调用。
type Context interface {
	// Self 返回当前 Actor 的 PID。
	Self() PID

	// Sender 返回当前消息的发送方 PID，若为空字符串则表示未知发送方。
	Sender() PID

	// System 返回所属的 Actor 系统。
	System() *System

	// Send 向目标 Actor 发送消息，发送方自动设置为当前 Actor。
	Send(to PID, msg Message) error

	// SendWithContext 支持带超时的发送。
	SendWithContext(ctx stdctx.Context, to PID, msg Message) error

	// StopSelf 停止当前 Actor。
	StopSelf()

	// Become 切换当前 Actor 的行为函数。
	// 传入 nil 会重置为默认 Actor.Receive 行为。
	Become(behavior func(Context, Message))
}

// Actor 是用户自定义 Actor 必须实现的接口。
type Actor interface {
	// Receive 处理一条消息。
	Receive(ctx Context, msg Message)
}

// PreStarter 是可选的生命周期接口。
// Actor 被创建并启动自己的 goroutine 后，会调用 PreStart。
type PreStarter interface {
	PreStart(ctx Context)
}

// PostStopper 是可选的生命周期接口。
// Actor 被停止后会调用 PostStop。
type PostStopper interface {
	PostStop(ctx Context)
}
