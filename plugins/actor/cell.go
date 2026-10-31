package actor

import (
	"errors"
	"sync/atomic"
	"time"

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

	consecutiveFailures int
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

	c.preStart()

	c.loop()
	c.stop()

	if post, ok := c.actor.(PostStopper); ok {
		c.invoke(post.PostStop)
	}
}

// preStart 调用 PreStart 生命周期钩子(初始启动与每次重启)。
func (c *actorCell) preStart() {
	if pre, ok := c.actor.(PreStarter); ok {
		c.invoke(pre.PreStart)
	}
}

// supervision 返回生效的监督配置:Props 覆盖系统默认。
func (c *actorCell) supervision() *Supervision {
	if c.props != nil && c.props.supervision != nil {
		return c.props.supervision
	}
	return c.system.options.DefaultSupervision
}

// loop 不断从邮箱取出并处理消息，直到邮箱关闭。
// 消息 panic 按监督策略处理(默认 RestartNever:仅记录)。
func (c *actorCell) loop() {
	for {
		env, ok := c.mailbox.next()
		if !ok {
			return
		}
		if panicked, r := c.process(env); panicked {
			c.handlePanic(env, r)
		} else {
			c.consecutiveFailures = 0
		}
	}
}

// handlePanic 按监督策略处理一次消息 panic。
func (c *actorCell) handlePanic(env Envelope, r interface{}) {
	sup := c.supervision()
	if sup == nil || sup.Strategy == RestartNever {
		return
	}
	func() {
		defer func() {
			if rr := recover(); rr != nil {
				logger.Errorf("actor supervision callback panic recovered",
					logger.String("pid", string(c.pid)), logger.Any("panic", rr))
			}
		}()
		if sup.OnPanic != nil {
			sup.OnPanic(c.pid, env.Message, r)
		}
	}()

	c.consecutiveFailures++
	if sup.MaxConsecutiveRestarts > 0 && c.consecutiveFailures > sup.MaxConsecutiveRestarts {
		if sup.OnGiveUp != nil {
			func() {
				defer func() {
					if rr := recover(); rr != nil {
						logger.Errorf("actor giveup callback panic recovered",
							logger.String("pid", string(c.pid)), logger.Any("panic", rr))
					}
				}()
				sup.OnGiveUp(c.pid, sup.giveUpErr(c.pid))
			}()
		}
		c.stop()
		return
	}

	if sup.Strategy == RestartBackoff {
		time.Sleep(sup.backoffFor(c.consecutiveFailures))
	}
	// 重建实例:Producer 存在时生成新实例,否则复用原实例。
	if c.props != nil && c.props.Producer != nil {
		if na, err := c.props.actor(); err == nil {
			c.actor = na
		}
	}
	c.behavior = nil
	c.preStart()
}

// process 处理单条消息，并捕获 panic 以避免整个 goroutine 崩溃。
// 返回是否发生 panic 及其值,交由监督策略决定后续。
func (c *actorCell) process(env Envelope) (panicked bool, panicVal interface{}) {
	c.ctx.sender = env.Sender
	defer func() {
		if r := recover(); r != nil {
			panicked = true
			panicVal = r
			logger.Errorf("actor message panic recovered",
				logger.String("pid", string(c.pid)),
				logger.Any("panic", r),
				logger.Any("message", env.Message))
		}
	}()

	if c.behavior != nil {
		c.behavior(c.ctx, env.Message)
		return false, nil
	}
	c.actor.Receive(c.ctx, env.Message)
	return false, nil
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
