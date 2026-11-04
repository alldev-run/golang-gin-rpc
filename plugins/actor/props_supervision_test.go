package actor

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type supervisionBoomMsg struct{}

// TestPropsWithSupervision_OverridesSystemDefault verifies that
// Props.WithSupervision wins over Options.DefaultSupervision: the system is
// configured with RestartNever, yet the actor restarts because its Props
// select RestartAlways.
func TestPropsWithSupervision_OverridesSystemDefault(t *testing.T) {
	sys := NewWithOptions(Options{DefaultSupervision: &Supervision{Strategy: RestartNever}})
	defer sys.Shutdown()

	var (
		created atomic.Int32
		panics  atomic.Int32
		onPanic atomic.Int32
		handled atomic.Int32
	)

	props := FromProducer(func() Actor {
		created.Add(1)
		return FromActorFunc(func(ctx Context, msg Message) {
			if _, ok := msg.(*supervisionBoomMsg); ok {
				panics.Add(1)
				panic("supervised boom")
			}
			handled.Add(1)
		})
	}).
		WithName("props-supervised").
		WithSupervision(&Supervision{
			Strategy: RestartAlways,
			OnPanic:  func(pid PID, msg Message, r interface{}) { onPanic.Add(1) },
		})

	pid, err := sys.Spawn(props)
	require.NoError(t, err)

	sys.Send(pid, &supervisionBoomMsg{})

	// Wait for the restart to rebuild the instance.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if created.Load() >= 2 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	assert.GreaterOrEqual(t, created.Load(), int32(2), "actor instance should be rebuilt via Props supervision")
	assert.Equal(t, int32(1), panics.Load(), "the boom message should panic exactly once")
	assert.Equal(t, int32(1), onPanic.Load(), "the Props OnPanic callback should fire once")

	// The restarted actor must stay responsive to normal messages.
	sys.Send(pid, "ping")
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if handled.Load() >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	assert.GreaterOrEqual(t, handled.Load(), int32(1), "restarted actor should process follow-up messages")
}

// TestPropsWithSupervision_NilKeepsSystemDefault verifies the fallback: a
// Props without supervision inherits the system default strategy.
func TestPropsWithSupervision_NilKeepsSystemDefault(t *testing.T) {
	var giveUps atomic.Int32
	sys := NewWithOptions(Options{DefaultSupervision: &Supervision{
		Strategy:               RestartAlways,
		MaxConsecutiveRestarts: 1,
		OnGiveUp:               func(pid PID, err error) { giveUps.Add(1) },
	}})
	defer sys.Shutdown()

	props := FromProducer(func() Actor {
		return FromActorFunc(func(ctx Context, msg Message) {
			panic("always")
		})
	}).WithName("default-supervised") // no WithSupervision call

	pid, err := sys.Spawn(props)
	require.NoError(t, err)

	// Two consecutive panics exceed MaxConsecutiveRestarts=1: the first
	// triggers a restart, the second trips OnGiveUp.
	sys.Send(pid, &supervisionBoomMsg{})
	sys.Send(pid, &supervisionBoomMsg{})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if giveUps.Load() >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	assert.GreaterOrEqual(t, giveUps.Load(), int32(1), "system-default supervision should give up after the restart limit")
}
