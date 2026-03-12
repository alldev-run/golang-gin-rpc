package jwtx

import "time"

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

var config Config

// Init initializes the JWT package with the provided configuration.
// Sets default token TTLs if not specified.
func Init(cfg Config) {

	if cfg.AccessTokenTTL == 0 {
		cfg.AccessTokenTTL = time.Minute * 15
	}

	if cfg.RefreshTokenTTL == 0 {
		cfg.RefreshTokenTTL = time.Hour * 24 * 7
	}

	config = cfg
}
