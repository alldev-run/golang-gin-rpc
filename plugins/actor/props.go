package actor

import "errors"

// Props 描述如何创建一个 Actor，包括名称、实例/工厂和邮箱大小。
type Props struct {
	Name string

	// Actor 直接指定一个 Actor 实例。
	// 如果同时设置了 Producer，优先使用 Producer。
	Actor Actor

	// Producer 是创建 Actor 的工厂函数，支持每次重启创建新实例。
	Producer func() Actor

	// MailboxSize 是邮箱缓冲区大小，<=0 时使用 System 默认值。
	MailboxSize int
}

// actor 返回要创建的 Actor 实例。
func (p *Props) actor() (Actor, error) {
	if p.Producer != nil {
		a := p.Producer()
		if a == nil {
			return nil, errors.New("actor: Producer returned nil")
		}
		return a, nil
	}
	if p.Actor != nil {
		return p.Actor, nil
	}
	return nil, errors.New("actor: Props must provide Actor or Producer")
}

// mailboxSize 返回最终邮箱大小。
func (p *Props) mailboxSize(defaultSize int) int {
	if p.MailboxSize > 0 {
		return p.MailboxSize
	}
	return defaultSize
}

// FromActor 使用已有实例构造 Props。
func FromActor(actor Actor) *Props {
	return &Props{Actor: actor}
}

// FromProducer 使用工厂函数构造 Props。
func FromProducer(producer func() Actor) *Props {
	return &Props{Producer: producer}
}

// WithName 设置 Actor 名称，返回 Props 自身以便链式调用。
func (p *Props) WithName(name string) *Props {
	p.Name = name
	return p
}

// WithMailboxSize 设置邮箱大小，返回 Props 自身以便链式调用。
func (p *Props) WithMailboxSize(size int) *Props {
	p.MailboxSize = size
	return p
}
