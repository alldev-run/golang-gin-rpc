package actor

// Options 是 Actor 系统的全局配置。
type Options struct {
	// DefaultMailboxSize 是默认邮箱缓冲区大小。
	DefaultMailboxSize int

	// OnDeadLetter 在消息无法送达时被调用（例如目标 Actor 不存在）。
	OnDeadLetter func(pid PID, msg Message, err error)

	// DefaultSupervision 是系统级监督策略默认值;
	// Props.WithSupervision 可按 Actor 覆盖。nil 等价于 RestartNever。
	DefaultSupervision *Supervision
}

// DefaultOptions 返回默认配置。
func DefaultOptions() Options {
	return Options{
		DefaultMailboxSize: 64,
	}
}
