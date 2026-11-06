package jwtx

import (
	"sync"
	"time"
)

// Config holds the JWT configuration parameters.
type Config struct {
	// Secret is the signing key for JWT tokens.
	Secret string

	// AccessTokenTTL is the lifetime of access tokens.
	AccessTokenTTL time.Duration
	// RefreshTokenTTL is the lifetime of refresh tokens.
	RefreshTokenTTL time.Duration

	// Store is the optional storage backend for token persistence.
	Store Store
}

type Manager struct {
	config Config
}

var (
	defaultManagerMu sync.RWMutex
	defaultManager   = &Manager{config: applyDefaults(Config{})}
)

func applyDefaults(cfg Config) Config {
	if cfg.AccessTokenTTL == 0 {
		cfg.AccessTokenTTL = time.Minute * 15
	}
	if cfg.RefreshTokenTTL == 0 {
		cfg.RefreshTokenTTL = time.Hour * 24 * 7
	}
	return cfg
}

func NewManager(cfg Config) *Manager {
	return &Manager{config: applyDefaults(cfg)}
}

func DefaultManager() *Manager {
	defaultManagerMu.RLock()
	defer defaultManagerMu.RUnlock()
	return defaultManager
}

func (m *Manager) Config() Config {
	if m == nil {
		return applyDefaults(Config{})
	}
	return m.config
}

// Init initializes the JWT package with the provided configuration.
// Sets default token TTLs if not specified.
func Init(cfg Config) {
	defaultManagerMu.Lock()
	defer defaultManagerMu.Unlock()
	defaultManager = NewManager(cfg)
}
