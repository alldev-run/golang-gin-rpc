package actor

import (
	"errors"
	"sync"
	"time"
)

// SendAfter 在 d 后向 to 投递一次消息,返回取消函数。
// 目标不存在或系统已停止时走 OnDeadLetter 回调。
func (s *System) SendAfter(to PID, msg Message, d time.Duration) (func(), error) {
	if d < 0 {
		return nil, errors.New("actor: negative delay")
	}
	timer := time.AfterFunc(d, func() {
		if err := s.Send(to, msg); err != nil && s.options.OnDeadLetter != nil {
			s.options.OnDeadLetter(to, msg, err)
		}
	})
	return func() { timer.Stop() }, nil
}

// SendEvery 以固定周期向 to 投递消息,直到取消、系统关闭或目标消失
// (ErrActorNotFound 自动停止投递)。返回取消函数。
func (s *System) SendEvery(to PID, msg Message, every time.Duration) (func(), error) {
	if every <= 0 {
		return nil, errors.New("actor: interval must be positive")
	}
	var (
		stop chan struct{}
		once sync.Once
	)
	stop = make(chan struct{})
	cancel := func() { once.Do(func() { close(stop) }) }
	go func() {
		ticker := time.NewTicker(every)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				err := s.Send(to, msg)
				if err == nil {
					continue
				}
				if s.options.OnDeadLetter != nil {
					s.options.OnDeadLetter(to, msg, err)
				}
				if errors.Is(err, ErrActorNotFound) || errors.Is(err, ErrSystemStopped) {
					return
				}
			}
		}
	}()
	return cancel, nil
}
