package bootstrap

import (
	"fmt"

	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
	"github.com/alldev-run/golang-gin-rpc/plugins/actor"
)

// InitializeActor initializes the actor system if enabled.
// actor 配置由本组件自行解析(plugins.actor 段)。
func (b *Bootstrap) InitializeActor() error {
	var actorCfg struct {
		Enabled       bool `yaml:"enabled"`
		MailboxSize   int  `yaml:"mailbox_size"`
		MaxRestarts   int  `yaml:"max_restarts"`
		RestartAlways bool `yaml:"restart_always"`
	}
	found, err := b.loader.UnmarshalSection("actor", &actorCfg)
	if err != nil {
		return fmt.Errorf("failed to parse actor config: %w", err)
	}
	if !found || !actorCfg.Enabled {
		logger.Info("Actor system disabled, skipping initialization")
		return nil
	}

	opts := actor.DefaultOptions()
	if actorCfg.MailboxSize > 0 {
		opts.DefaultMailboxSize = actorCfg.MailboxSize
	}
	if actorCfg.RestartAlways {
		opts.DefaultSupervision = &actor.Supervision{
			Strategy:               actor.RestartAlways,
			MaxConsecutiveRestarts: actorCfg.MaxRestarts,
		}
	}
	b.actorSystem = actor.NewWithOptions(opts)
	b.setDependency(DepActorSystem, b.actorSystem)

	logger.Info("Actor system initialized",
		logger.Int("mailbox_size", opts.DefaultMailboxSize),
		logger.Bool("restart_always", actorCfg.RestartAlways))
	return nil
}
