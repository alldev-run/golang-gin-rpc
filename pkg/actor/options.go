package actor

// Options 是 Actor 系统的全局配置。
type Options struct {
	// DefaultMailboxSize 是默认邮箱缓冲区大小。
	DefaultMailboxSize int

	// OnDeadLetter 在消息无法送达时被调用（例如目标 Actor 不存在）。
	OnDeadLetter func(pid PID, msg Message, err error)
}

// DefaultOptions 返回默认配置。
func DefaultOptions() Options {
	return Options{
		DefaultMailboxSize: 64,
	}
}
