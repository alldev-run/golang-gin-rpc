package jwtx

import "time"

// Store defines the interface for token storage backends,
// typically implemented using Redis or similar systems.
type Store interface {
	Set(key string, value string, ttl time.Duration) error
	Get(key string) (string, error)
	Del(key string) error
}
