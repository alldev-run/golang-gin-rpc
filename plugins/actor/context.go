package actor

import (
	stdctx "context"
)

// actorContext 是 Context 接口的具体实现。
type actorContext struct {
	self   PID
	sender PID
	system *System
	cell   *actorCell
}

func (c *actorContext) Self() PID   { return c.self }
func (c *actorContext) Sender() PID { return c.sender }
func (c *actorContext) System() *System {
	return c.system
}

func (c *actorContext) Send(to PID, msg Message) error {
	return c.system.send(stdctx.Background(), to, msg, c.self)
}

func (c *actorContext) SendWithContext(ctx stdctx.Context, to PID, msg Message) error {
	return c.system.send(ctx, to, msg, c.self)
}

func (c *actorContext) StopSelf() {
	c.cell.stop()
}

func (c *actorContext) Become(behavior func(Context, Message)) {
	c.cell.behavior = behavior
}
