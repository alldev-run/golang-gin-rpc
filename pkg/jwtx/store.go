package jwtx

import "time"

type Store interface {
	Set(key string, value string, ttl time.Duration) error
	Get(key string) (string, error)
	Del(key string) error
}
