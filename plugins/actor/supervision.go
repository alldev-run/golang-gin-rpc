package actor

import (
	"fmt"
	"time"
)

// RestartStrategy 定义消息处理 panic 后的重启策略。
type RestartStrategy int

const (
	// RestartNever 保持兼容行为:仅记录 panic,Actor 继续处理后续消息。
	RestartNever RestartStrategy = iota
	// RestartAlways panic 后立即重建实例(Producer)并重跑 PreStart。
	RestartAlways
	// RestartBackoff 按指数退避重建,避免崩溃循环打满 CPU。
	RestartBackoff
)

// Supervision 是 Actor 的监督(容错)配置。
//
// 语义:
//   - 只有消息处理 panic 触发重启;PreStart/PostStop panic 仍只记录。
//   - 重启 = 用 Producer 重建实例(未设置 Producer 则复用原实例)+ 重置
//     Become 行为 + 重跑 PreStart。
//   - 连续失败计数在任一消息处理成功后清零。
type Supervision struct {
	// Strategy 重启策略,默认 RestartNever(兼容旧行为)。
	Strategy RestartStrategy

	// MaxConsecutiveRestarts 允许的最大连续重启次数;超过后停止 Actor
	// 并调用 OnGiveUp。0 表示不设上限。
	MaxConsecutiveRestarts int

	// BackoffInitial / BackoffMax 供 RestartBackoff 使用;
	// 退避序列为 initial * 2^(n-1),封顶 max。两者 <=0 时取 100ms / 5s。
	BackoffInitial time.Duration
	BackoffMax     time.Duration

	// OnPanic 每次 panic 触发时回调(观测用)。
	OnPanic func(pid PID, msg Message, r interface{})

	// OnGiveUp 重启超限、Actor 被停止时回调。
	OnGiveUp func(pid PID, err error)
}

func (sv *Supervision) backoffFor(attempt int) time.Duration {
	initial := sv.BackoffInitial
	if initial <= 0 {
		initial = 100 * time.Millisecond
	}
	max := sv.BackoffMax
	if max <= 0 {
		max = 5 * time.Second
	}
	d := initial
	for i := 1; i < attempt; i++ {
		d *= 2
		if d >= max {
			return max
		}
	}
	return d
}

func (sv *Supervision) giveUpErr(pid PID) error {
	return fmt.Errorf("actor: %s restart limit exceeded", pid)
}
