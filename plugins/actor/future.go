package actor

import (
	stdctx "context"
	"sync"
	"time"
)

// DefaultAskTimeout 是 Ask 未指定超时时的默认等待时长。
const DefaultAskTimeout = 5 * time.Second

// futureActor 把目标 Actor 的回复桥接到 channel,实现请求-应答。
// 收到第一条回复(或超时)后 channel 关闭。
type futureActor struct {
	ch   chan Message
	once sync.Once
}

func (f *futureActor) Receive(ctx Context, msg Message) {
	f.deliver(msg)
	ctx.StopSelf()
}

// deliver 投递第一条回复并关闭 channel;后续消息与超时路径幂等。
func (f *futureActor) deliver(msg Message) {
	f.once.Do(func() {
		if msg != nil {
			select {
			case f.ch <- msg:
			default:
			}
		}
		close(f.ch)
	})
}

// Ask 向目标 Actor 发送请求并等待其回复。
//
// 目标通过 ctx.Send(ctx.Sender(), reply) 回复,Sender 即 Ask 内部
// 创建的 future;返回的 channel 在收到第一条回复后关闭,携带该回复;
// 超时或目标不存在时 channel 关闭且无值(接收方 ok == false)。
//
// 每次 Ask 会临时创建一个轻量 future Actor,高频路径请改用 Send。
func (s *System) Ask(ctx stdctx.Context, to PID, msg Message, timeout time.Duration) (<-chan Message, error) {
	if timeout <= 0 {
		timeout = DefaultAskTimeout
	}
	f := &futureActor{ch: make(chan Message, 1)}
	pid, err := s.Spawn(FromProducer(func() Actor { return f }))
	if err != nil {
		return nil, err
	}
	if err := s.send(ctx, to, msg, pid); err != nil {
		_ = s.Stop(pid)
		f.deliver(nil)
		return nil, err
	}
	time.AfterFunc(timeout, func() {
		f.deliver(nil)
		_ = s.Stop(pid)
	})
	return f.ch, nil
}
