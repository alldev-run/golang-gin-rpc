package actor

import (
	stdctx "context"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
)

// System 是 Actor 的容器和管理者，负责创建、查找、发送和停止 Actor。
type System struct {
	options Options
	actors  map[string]*actorCell
	mu      sync.RWMutex
	stopped int32
}

// New 使用默认配置创建 Actor 系统。
func New() *System {
	return NewWithOptions(DefaultOptions())
}

// NewWithOptions 使用指定配置创建 Actor 系统。
func NewWithOptions(opts Options) *System {
	if opts.DefaultMailboxSize <= 0 {
		opts.DefaultMailboxSize = 64
	}
	return &System{
		options: opts,
		actors:  make(map[string]*actorCell),
	}
}

// Running 返回系统是否仍在运行。
func (s *System) Running() bool {
	return atomic.LoadInt32(&s.stopped) == 0
}

// Count 返回当前存活的 Actor 数量。
func (s *System) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.actors)
}

// Spawn 使用 Props 创建并启动一个 Actor。
// 若 Props.Name 为空，则自动生成一个 UUID 作为 PID。
func (s *System) Spawn(props *Props) (PID, error) {
	if props == nil {
		return "", errors.New("actor: props is nil")
	}
	if !s.Running() {
		return "", ErrSystemStopped
	}

	actorImpl, err := props.actor()
	if err != nil {
		return "", err
	}

	pid := PID(props.Name)
	if pid == "" {
		pid = PID(uuid.New().String())
	}

	cell, err := s.newActor(pid, actorImpl, props)
	if err != nil {
		return "", err
	}

	// 先注册到系统，再启动 goroutine。
	// 这样 PreStart 中调用 ctx.Send(ctx.Self(), ...) 或其他 Actor
	// 向该 PID 发消息时，能正常找到目标，避免 ErrActorNotFound。
	if err := s.add(pid, cell); err != nil {
		return "", err
	}
	if err := cell.start(); err != nil {
		s.remove(pid)
		return "", err
	}

	return pid, nil
}

// SpawnNamed 是 Spawn 的便捷方法，使用指定的名称和 Actor 实例。
func (s *System) SpawnNamed(name string, actor Actor) (PID, error) {
	return s.Spawn(FromActor(actor).WithName(name))
}

// Send 向目标 Actor 发送消息。
func (s *System) Send(to PID, msg Message) error {
	return s.SendWithContext(stdctx.Background(), to, msg)
}

// SendWithContext 支持带超时的消息发送。
func (s *System) SendWithContext(ctx stdctx.Context, to PID, msg Message) error {
	return s.send(ctx, to, msg, "")
}

// Stop 停止指定 PID 的 Actor，并等待其完成。
func (s *System) Stop(pid PID) error {
	cell, ok := s.get(pid)
	if !ok {
		return ErrActorNotFound
	}
	cell.stop()
	<-cell.done
	return nil
}

// Shutdown 停止整个 Actor 系统，等待所有 Actor 退出后返回。
func (s *System) Shutdown() error {
	if !atomic.CompareAndSwapInt32(&s.stopped, 0, 1) {
		return nil
	}

	cells := s.all()
	var wg sync.WaitGroup
	for _, cell := range cells {
		wg.Add(1)
		go func(c *actorCell) {
			defer wg.Done()
			c.stop()
			<-c.done
		}(cell)
	}
	wg.Wait()
	return nil
}

// send 是内部发送实现，from 表示发送方 PID。
func (s *System) send(ctx stdctx.Context, to PID, msg Message, from PID) error {
	if !s.Running() {
		return ErrSystemStopped
	}
	cell, ok := s.get(to)
	if !ok {
		if s.options.OnDeadLetter != nil {
			s.options.OnDeadLetter(to, msg, ErrActorNotFound)
		}
		return ErrActorNotFound
	}
	return cell.mailbox.push(ctx, Envelope{Sender: from, Message: msg})
}

// newActor 创建 actorCell，但不启动它。
func (s *System) newActor(pid PID, actor Actor, props *Props) (*actorCell, error) {
	size := props.mailboxSize(s.options.DefaultMailboxSize)
	cell := &actorCell{
		pid:     pid,
		actor:   actor,
		props:   props,
		system:  s,
		mailbox: newMailbox(size),
		done:    make(chan struct{}),
	}
	cell.ctx = &actorContext{
		self:   pid,
		system: s,
		cell:   cell,
	}
	return cell, nil
}

// add 将 Actor 注册到系统中。
func (s *System) add(pid PID, cell *actorCell) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped == 1 {
		return ErrSystemStopped
	}
	if _, exists := s.actors[string(pid)]; exists {
		return ErrActorAlreadyExists
	}
	s.actors[string(pid)] = cell
	return nil
}

// get 按 PID 查找 Actor。
func (s *System) get(pid PID) (*actorCell, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cell, ok := s.actors[string(pid)]
	return cell, ok
}

// remove 从系统中移除 Actor。
func (s *System) remove(pid PID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.actors, string(pid))
}

// all 返回当前所有 Actor 的快照。
func (s *System) all() []*actorCell {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cells := make([]*actorCell, 0, len(s.actors))
	for _, c := range s.actors {
		cells = append(cells, c)
	}
	return cells
}
