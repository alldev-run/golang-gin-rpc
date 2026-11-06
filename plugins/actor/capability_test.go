package actor

import (
	stdctx "context"
	"sync/atomic"
	"testing"
	"time"
)

// ===== Ask =====

type askEchoActor struct{}

func (e *askEchoActor) Receive(ctx Context, msg Message) {
	_ = ctx.Send(ctx.Sender(), "echo:"+msg.(string))
}

type askSilentActor struct{}

func (s *askSilentActor) Receive(ctx Context, msg Message) {}

func TestAskReply(t *testing.T) {
	sys := New()
	defer sys.Shutdown()

	pid, err := sys.SpawnNamed("echo", &askEchoActor{})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	reply, err := sys.Ask(stdctx.Background(), pid, "hello", time.Second)
	if err != nil {
		t.Fatalf("ask: %v", err)
	}
	select {
	case msg, ok := <-reply:
		if !ok {
			t.Fatal("reply channel closed without value")
		}
		if msg != "echo:hello" {
			t.Fatalf("got %v", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for reply")
	}
}

func TestAskTimeout(t *testing.T) {
	sys := New()
	defer sys.Shutdown()

	pid, err := sys.SpawnNamed("silent", &askSilentActor{})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	start := time.Now()
	reply, err := sys.Ask(stdctx.Background(), pid, "hi", 100*time.Millisecond)
	if err != nil {
		t.Fatalf("ask: %v", err)
	}
	if _, ok := <-reply; ok {
		t.Fatal("expected closed channel on timeout")
	}
	if time.Since(start) < 90*time.Millisecond {
		t.Fatal("timeout returned too early")
	}
}

func TestAskTargetMissing(t *testing.T) {
	sys := New()
	defer sys.Shutdown()
	if _, err := sys.Ask(stdctx.Background(), "ghost", "hi", time.Second); err == nil {
		t.Fatal("expected error for missing target")
	}
}

// ===== 定时消息 =====

func TestSendAfter(t *testing.T) {
	sys := New()
	defer sys.Shutdown()

	var hits atomic.Int32
	pid, _ := sys.SpawnNamed("counter", FromActorFunc(func(ctx Context, msg Message) {
		hits.Add(1)
	}))
	cancel, err := sys.SendAfter(pid, "tick", 50*time.Millisecond)
	if err != nil {
		t.Fatalf("sendafter: %v", err)
	}
	defer cancel()
	time.Sleep(150 * time.Millisecond)
	if hits.Load() != 1 {
		t.Fatalf("expected 1 hit, got %d", hits.Load())
	}
}

func TestSendEveryAndCancel(t *testing.T) {
	sys := New()
	defer sys.Shutdown()

	var hits atomic.Int32
	pid, _ := sys.SpawnNamed("ticker-target", FromActorFunc(func(ctx Context, msg Message) {
		hits.Add(1)
	}))
	cancel, err := sys.SendEvery(pid, "tick", 30*time.Millisecond)
	if err != nil {
		t.Fatalf("sendevery: %v", err)
	}
	time.Sleep(120 * time.Millisecond)
	cancel()
	// 允许 1 条 cancel 时已在途的 tick 被处理。
	afterCancel := hits.Load() + 1
	time.Sleep(200 * time.Millisecond)
	if hits.Load() > afterCancel {
		t.Fatalf("cancel did not stop ticker: %d -> %d", afterCancel, hits.Load())
	}
	if afterCancel == 1 {
		t.Fatal("expected at least one tick before cancel")
	}
}

// FromActorFunc 便于测试中用闭包构造 Actor。
func FromActorFunc(f func(Context, Message)) Actor {
	return actorFunc(f)
}

type actorFunc func(Context, Message)

func (f actorFunc) Receive(ctx Context, msg Message) { f(ctx, msg) }

// ===== Supervision =====

type panicOnceActor struct {
	panics  atomic.Int32
	handled atomic.Int32
}

func (p *panicOnceActor) Receive(ctx Context, msg Message) {
	if msg == "boom" {
		p.panics.Add(1)
		panic("boom!")
	}
	p.handled.Add(1)
}

func TestSupervisionRestartNeverKeepsOldBehavior(t *testing.T) {
	sys := New() // 无监督配置 = RestartNever
	defer sys.Shutdown()

	a := &panicOnceActor{}
	pid, _ := sys.SpawnNamed("pn", a)
	_ = sys.Send(pid, "boom")
	_ = sys.Send(pid, "ok")
	waitFor(t, func() bool { return a.handled.Load() == 1 })
	// RestartNever:panic 后 Actor 仍存活并继续处理。
	if sys.Count() == 0 {
		t.Fatal("actor should still be registered")
	}
}

func TestSupervisionRestartRebuildsInstance(t *testing.T) {
	var created atomic.Int32
	sys := NewWithOptions(Options{DefaultSupervision: &Supervision{Strategy: RestartAlways}})
	defer sys.Shutdown()

	pid, _ := sys.Spawn(FromProducer(func() Actor {
		created.Add(1)
		return &panicOnceActor{}
	}).WithName("sup"))

	_ = sys.Send(pid, "boom")
	waitFor(t, func() bool { return created.Load() == 2 }) // Producer 重建
	if _, ok := sys.get(pid); !ok {
		t.Fatal("restarted actor should be registered under the same PID")
	}
}

func TestSupervisionGiveUpAfterMaxRestarts(t *testing.T) {
	giveUps := make(chan string, 1)
	sys := NewWithOptions(Options{DefaultSupervision: &Supervision{
		Strategy:               RestartAlways,
		MaxConsecutiveRestarts: 2,
		OnGiveUp:               func(pid PID, err error) { giveUps <- string(pid) },
	}})
	defer sys.Shutdown()

	pid, _ := sys.Spawn(FromProducer(func() Actor { return &panicOnceActor{} }).WithName("doomed"))
	for i := 0; i < 5; i++ {
		_ = sys.Send(pid, "boom")
	}
	select {
	case got := <-giveUps:
		if got != string(pid) {
			t.Fatalf("giveup pid = %s", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected OnGiveUp")
	}
	waitFor(t, func() bool {
		_, ok := sys.get(pid)
		return !ok
	})
}

func TestSupervisionBackoffRestart(t *testing.T) {
	var created atomic.Int32
	sys := NewWithOptions(Options{DefaultSupervision: &Supervision{
		Strategy:       RestartBackoff,
		BackoffInitial: 50 * time.Millisecond,
		BackoffMax:     200 * time.Millisecond,
	}})
	defer sys.Shutdown()

	pid, _ := sys.Spawn(FromProducer(func() Actor {
		created.Add(1)
		return &panicOnceActor{}
	}).WithName("backoff"))
	_ = sys.Send(pid, "boom")
	waitFor(t, func() bool { return created.Load() == 2 })
}

func TestSupervisionConsecutiveFailuresReset(t *testing.T) {
	var created atomic.Int32
	sys := NewWithOptions(Options{DefaultSupervision: &Supervision{
		Strategy:               RestartAlways,
		MaxConsecutiveRestarts: 1,
	}})
	defer sys.Shutdown()

	a := &panicOnceActor{}
	pid, _ := sys.Spawn(FromProducer(func() Actor { created.Add(1); return a }).WithName("reset"))

	// boom -> restart; ok -> 计数清零;boom -> 再 restart,不应 give up。
	sequence := []string{"boom", "ok", "boom", "ok"}
	for _, m := range sequence {
		_ = sys.Send(pid, m)
	}
	waitFor(t, func() bool { return created.Load() >= 3 })
	// Actor 未被移除 = 未触发 give up。
	if _, ok := sys.get(pid); !ok {
		t.Fatal("actor should not be removed; consecutive counter should reset on success")
	}
}

// ===== 测试辅助 =====

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met within 2s")
}
