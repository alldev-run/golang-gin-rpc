package actor

import "errors"

var (
	// ErrSystemStopped 表示 Actor 系统已经停止。
	ErrSystemStopped = errors.New("actor system is stopped")

	// ErrActorNotFound 表示目标 Actor 不存在。
	ErrActorNotFound = errors.New("actor not found")

	// ErrActorAlreadyExists 表示同名 Actor 已存在。
	ErrActorAlreadyExists = errors.New("actor already exists")

	// ErrMailboxClosed 表示 Actor 邮箱已关闭。
	ErrMailboxClosed = errors.New("actor mailbox is closed")
)
