package actor

import (
	"errors"
	"sync/atomic"

	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
)

// actorCell 是单个 Actor 的运行时封装，包含实例、邮箱和上下文。
type actorCell struct {
	pid      PID
	actor    Actor
	props    *Props
	system   *System
	mailbox  *mailbox
	ctx      *actorContext
	behavior func(Context, Message)
	running  int32
	stopped  int32
	done     chan struct{}
}

// start 启动 Actor 的 goroutine。必须成功调用一次。
func (c *actorCell) start() error {
	if !atomic.CompareAndSwapInt32(&c.running, 0, 1) {
		return errors.New("actor already started")
	}
	go c.run()
	return nil
}

// run 是 Actor 的主循环，负责处理生命周期和消息。
func (c *actorCell) run() {
	defer close(c.done)
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("actor goroutine panic recovered",
				logger.String("pid", string(c.pid)),
				logger.Any("panic", r))
		}
	}()

	if pre, ok := c.actor.(PreStarter); ok {
		c.invoke(pre.PreStart)
	}

	c.loop()
	c.stop()

	if post, ok := c.actor.(PostStopper); ok {
		c.invoke(post.PostStop)
	}
}

// loop 不断从邮箱取出并处理消息，直到邮箱关闭。
func (c *actorCell) loop() {
	for {
		env, ok := c.mailbox.next()
		if !ok {
			return
		}
		c.process(env)
	}
}

// process 处理单条消息，并捕获 panic 以避免整个 goroutine 崩溃。
func (c *actorCell) process(env Envelope) {
	c.ctx.sender = env.Sender
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("actor message panic recovered",
				logger.String("pid", string(c.pid)),
				logger.Any("panic", r),
				logger.Any("message", env.Message))
		}
	}()

	if c.behavior != nil {
		c.behavior(c.ctx, env.Message)
		return
	}
	c.actor.Receive(c.ctx, env.Message)
}

// invoke 安全地调用生命周期函数。
func (c *actorCell) invoke(f func(Context)) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("actor lifecycle panic recovered",
				logger.String("pid", string(c.pid)),
				logger.Any("panic", r))
		}
	}()
	f(c.ctx)
}

// stop 停止 Actor。多次调用幂等。
func (c *actorCell) stop() bool {
	if !atomic.CompareAndSwapInt32(&c.stopped, 0, 1) {
		return false
	}
	c.system.remove(c.pid)
	c.mailbox.stop()
	return true
}
