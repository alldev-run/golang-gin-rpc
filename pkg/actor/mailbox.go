package actor

import (
	stdctx "context"
	"sync/atomic"
)

// mailbox 是 Actor 的消息队列，使用有缓冲 channel 实现。
type mailbox struct {
	ch     chan Envelope
	stopCh chan struct{}
	closed int32
}

func newMailbox(size int) *mailbox {
	if size <= 0 {
		size = 64
	}
	return &mailbox{
		ch:     make(chan Envelope, size),
		stopCh: make(chan struct{}),
	}
}

// push 将消息放入邮箱。若邮箱已关闭或 context 已取消，则返回错误。
func (m *mailbox) push(ctx stdctx.Context, env Envelope) error {
	if atomic.LoadInt32(&m.closed) == 1 {
		return ErrMailboxClosed
	}
	select {
	case m.ch <- env:
		return nil
	case <-m.stopCh:
		return ErrMailboxClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

// stop 关闭邮箱，之后 push 会返回 ErrMailboxClosed。
func (m *mailbox) stop() {
	if !atomic.CompareAndSwapInt32(&m.closed, 0, 1) {
		return
	}
	close(m.stopCh)
}

// next 从邮箱中取出下一条消息。
// 返回的 bool 表示是否还有消息；false 表示邮箱已停止且已排空。
func (m *mailbox) next() (Envelope, bool) {
	select {
	case env := <-m.ch:
		return env, true
	case <-m.stopCh:
		// 停止前尝试排空剩余消息。
		for {
			select {
			case env := <-m.ch:
				return env, true
			default:
				return Envelope{}, false
			}
		}
	}
}
