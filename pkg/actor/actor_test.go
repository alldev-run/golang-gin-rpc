package actor

import (
	stdctx "context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type inc struct{ n int }

type getCount struct {
	reply chan int
}

type panicMsg struct{}

type counterActor struct {
	count int
}

func (c *counterActor) Receive(ctx Context, msg Message) {
	switch m := msg.(type) {
	case *inc:
		c.count += m.n
	case *getCount:
		m.reply <- c.count
	case panicMsg:
		panic("expected panic")
	}
}

func TestSystem_New(t *testing.T) {
	s := New()
	assert.NotNil(t, s)
	assert.True(t, s.Running())
	assert.Equal(t, 0, s.Count())
}

func TestSystem_SpawnAndSend(t *testing.T) {
	s := New()
	defer s.Shutdown()

	pid, err := s.SpawnNamed("counter", &counterActor{})
	assert.NoError(t, err)
	assert.NotEmpty(t, pid)
	assert.Equal(t, 1, s.Count())

	assert.NoError(t, s.Send(pid, &inc{n: 1}))
	assert.NoError(t, s.Send(pid, &inc{n: 2}))

	reply := make(chan int, 1)
	assert.NoError(t, s.Send(pid, &getCount{reply: reply}))

	select {
	case v := <-reply:
		assert.Equal(t, 3, v)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for count")
	}
}

func TestSystem_SendWithContextTimeout(t *testing.T) {
	s := New()
	defer s.Shutdown()

	slow := &slowActor{hold: make(chan struct{})}
	pid, err := s.Spawn(FromActor(slow).WithName("slow").WithMailboxSize(1))
	assert.NoError(t, err)

	// 占满邮箱，并让 Actor 一直持有
	assert.NoError(t, s.Send(pid, "block"))

	ctx, cancel := stdctx.WithTimeout(stdctx.Background(), 50*time.Millisecond)
	defer cancel()

	err = s.SendWithContext(ctx, pid, "overflow")
	assert.Error(t, err)
	assert.True(t, errors.Is(err, stdctx.DeadlineExceeded) || err == stdctx.DeadlineExceeded)

	// 释放 Actor
	close(slow.hold)
}

type slowActor struct {
	hold chan struct{}
}

func (s *slowActor) Receive(ctx Context, msg Message) {
	<-s.hold
}

func TestSystem_SendToNonExistent(t *testing.T) {
	s := New()
	defer s.Shutdown()

	err := s.Send("not-exist", "hello")
	assert.ErrorIs(t, err, ErrActorNotFound)
}

func TestSystem_StopActor(t *testing.T) {
	s := New()

	pid, err := s.SpawnNamed("stop-me", &counterActor{})
	assert.NoError(t, err)
	assert.NoError(t, s.Send(pid, &inc{n: 5}))

	assert.NoError(t, s.Stop(pid))
	assert.Equal(t, 0, s.Count())
	assert.ErrorIs(t, s.Send(pid, &inc{n: 1}), ErrActorNotFound)
	assert.NoError(t, s.Shutdown())
}

func TestSystem_Shutdown(t *testing.T) {
	s := New()

	_, err := s.SpawnNamed("a", &counterActor{})
	assert.NoError(t, err)
	_, err = s.SpawnNamed("b", &counterActor{})
	assert.NoError(t, err)

	assert.Equal(t, 2, s.Count())
	assert.NoError(t, s.Shutdown())
	assert.False(t, s.Running())
	assert.Equal(t, 0, s.Count())
}

func TestSystem_PanicRecovery(t *testing.T) {
	s := New()
	defer s.Shutdown()

	pid, err := s.SpawnNamed("panic", &counterActor{})
	assert.NoError(t, err)

	assert.NoError(t, s.Send(pid, &inc{n: 1}))
	assert.NoError(t, s.Send(pid, panicMsg{}))
	assert.NoError(t, s.Send(pid, &inc{n: 2}))

	reply := make(chan int, 1)
	assert.NoError(t, s.Send(pid, &getCount{reply: reply}))

	select {
	case v := <-reply:
		assert.Equal(t, 3, v)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for count after panic")
	}
}

func TestSystem_Become(t *testing.T) {
	s := New()
	defer s.Shutdown()

	pid, err := s.SpawnNamed("become", &becomeActor{})
	assert.NoError(t, err)

	assert.NoError(t, s.Send(pid, "setup"))
	time.Sleep(50 * time.Millisecond)

	assert.NoError(t, s.Send(pid, "ping"))
	assert.NoError(t, s.Send(pid, "ping"))

	reply := make(chan int, 1)
	assert.NoError(t, s.Send(pid, &getCount{reply: reply}))

	select {
	case v := <-reply:
		assert.Equal(t, 2, v)
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

type becomeActor struct {
	pings int
}

func (b *becomeActor) Receive(ctx Context, msg Message) {
	switch msg.(type) {
	case string:
		if msg == "setup" {
			ctx.Become(b.pinged)
		}
	}
}

func (b *becomeActor) pinged(ctx Context, msg Message) {
	if s, ok := msg.(string); ok && s == "ping" {
		b.pings++
		return
	}
	if g, ok := msg.(*getCount); ok {
		g.reply <- b.pings
	}
}

func TestSystem_PreStartPostStop(t *testing.T) {
	s := New()

	lifecycle := &lifecycleActor{
		started: make(chan struct{}),
		stopped: make(chan struct{}),
	}

	_, err := s.SpawnNamed("lifecycle", lifecycle)
	assert.NoError(t, err)

	select {
	case <-lifecycle.started:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for PreStart")
	}

	assert.NoError(t, s.Shutdown())

	select {
	case <-lifecycle.stopped:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for PostStop")
	}
}

type lifecycleActor struct {
	started chan struct{}
	stopped chan struct{}
	once    sync.Once
}

func (l *lifecycleActor) PreStart(ctx Context) {
	l.once.Do(func() { close(l.started) })
}

func (l *lifecycleActor) Receive(ctx Context, msg Message) {}

func (l *lifecycleActor) PostStop(ctx Context) {
	l.once.Do(func() { close(l.stopped) })
}

func TestSystem_DuplicateName(t *testing.T) {
	s := New()
	defer s.Shutdown()

	_, err := s.SpawnNamed("dup", &counterActor{})
	assert.NoError(t, err)

	_, err = s.SpawnNamed("dup", &counterActor{})
	assert.ErrorIs(t, err, ErrActorAlreadyExists)
}

func TestSystem_Producer(t *testing.T) {
	s := New()
	defer s.Shutdown()

	var created int32
	pid, err := s.Spawn(FromProducer(func() Actor {
		atomic.AddInt32(&created, 1)
		return &counterActor{}
	}).WithName("from-producer"))
	assert.NoError(t, err)

	assert.NoError(t, s.Send(pid, &inc{n: 1}))
	assert.Equal(t, int32(1), atomic.LoadInt32(&created))
}

func TestContext_Send(t *testing.T) {
	s := New()
	defer s.Shutdown()

	echo := &echoActor{}
	pid, err := s.SpawnNamed("echo", echo)
	assert.NoError(t, err)

	reply := make(chan PID, 1)
	assert.NoError(t, s.Send(pid, &tell{reply: reply}))

	select {
	case sender := <-reply:
		assert.Equal(t, pid, sender)
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

type tell struct {
	reply chan PID
}

type echoActor struct{}

func (e *echoActor) Receive(ctx Context, msg Message) {
	if m, ok := msg.(*tell); ok {
		m.reply <- ctx.Sender()
	}
}
