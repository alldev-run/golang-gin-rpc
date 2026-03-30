package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"

	"github.com/alldev-run/golang-gin-rpc/pkg/configcenter"
)

// RuntimeAuditConfig is the mutable subset of audit configuration.
type RuntimeAuditConfig struct {
	Enabled       bool     `json:"enabled"`
	SkipPaths     []string `json:"skip_paths"`
	SensitiveKeys []string `json:"sensitive_keys"`
}

// DynamicAuditConfig stores runtime-updatable audit config safely.
type DynamicAuditConfig struct {
	cfg atomic.Value // RuntimeAuditConfig
}

// NewDynamicAuditConfig creates dynamic config initialized from static AuditConfig.
func NewDynamicAuditConfig(base AuditConfig) *DynamicAuditConfig {
	d := &DynamicAuditConfig{}
	d.cfg.Store(RuntimeAuditConfig{
		Enabled:       base.Enabled,
		SkipPaths:     append([]string(nil), base.SkipPaths...),
		SensitiveKeys: append([]string(nil), base.SensitiveKeys...),
	})
	return d
}

// Snapshot returns the latest runtime config.
func (d *DynamicAuditConfig) Snapshot() RuntimeAuditConfig {
	v := d.cfg.Load()
	if v == nil {
		return RuntimeAuditConfig{Enabled: true}
	}
	cfg := v.(RuntimeAuditConfig)
	cfg.SkipPaths = append([]string(nil), cfg.SkipPaths...)
	cfg.SensitiveKeys = append([]string(nil), cfg.SensitiveKeys...)
	return cfg
}

// Update replaces current runtime config.
func (d *DynamicAuditConfig) Update(cfg RuntimeAuditConfig) {
	cfg.SkipPaths = append([]string(nil), cfg.SkipPaths...)
	cfg.SensitiveKeys = append([]string(nil), cfg.SensitiveKeys...)
	d.cfg.Store(cfg)
}

// BindAuditConfigCenter subscribes configcenter key changes into DynamicAuditConfig.
// Value format should be JSON object matching RuntimeAuditConfig.
func BindAuditConfigCenter(ctx context.Context, cc *configcenter.ConfigCenter, namespace, key string, target *DynamicAuditConfig) (configcenter.Subscription, error) {
	if cc == nil {
		return nil, errors.New("configcenter is nil")
	}
	if target == nil {
		return nil, errors.New("dynamic audit config target is nil")
	}

	defaultCfg := target.Snapshot()

	if raw, _, err := cc.Get(ctx, namespace, key); err == nil {
		var cfg RuntimeAuditConfig
		if json.Unmarshal(raw, &cfg) == nil {
			target.Update(cfg)
		}
	} else if !errors.Is(err, configcenter.ErrNotFound) {
		return nil, err
	}

	sub, err := cc.Subscribe(ctx, namespace, func(change configcenter.ConfigChange) {
		if change.Key != key {
			return
		}
		if change.Change == configcenter.ChangeTypeDelete {
			target.Update(defaultCfg)
			return
		}
		var cfg RuntimeAuditConfig
		if err := json.Unmarshal(change.Value, &cfg); err != nil {
			return
		}
		target.Update(cfg)
	})
	if err != nil {
		return nil, err
	}
	return sub, nil
}
