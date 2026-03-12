package jwtx

import "time"

type Config struct {
	Secret string

	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration

	Store Store
}

var config Config

func Init(cfg Config) {

	if cfg.AccessTokenTTL == 0 {
		cfg.AccessTokenTTL = time.Minute * 15
	}

	if cfg.RefreshTokenTTL == 0 {
		cfg.RefreshTokenTTL = time.Hour * 24 * 7
	}

	config = cfg
}
