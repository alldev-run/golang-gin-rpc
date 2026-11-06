package configcenter

import "time"

// Option configures ConfigCenter behavior.
type Option func(*ConfigCenter)

// WithCacheTTL sets the default TTL for cached config values.
func WithCacheTTL(ttl time.Duration) Option {
	return func(cc *ConfigCenter) {
		cc.cacheTTL = ttl
	}
}

// WithLogger enables custom logging hook.
type Logger func(args ...interface{})

// WithLogger sets a logger used for internal diagnostic messages.
func WithLogger(logger Logger) Option {
	return func(cc *ConfigCenter) {
		cc.log = logger
	}
}
