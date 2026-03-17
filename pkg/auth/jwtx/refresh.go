package jwtx

import (
	"errors"
	"time"
)

// Refresh validates a refresh token and generates a new token pair.
// Deletes the used refresh token from the store after successful validation.
func (m *Manager) Refresh(refreshToken string) (*TokenPair, error) {
	cfg := m.Config()
	claims, err := m.decodeClaims(refreshToken)
	if err != nil {
		return nil, err
	}

	if claims.Type != TokenTypeRefresh {
		return nil, errors.New("invalid refresh token")
	}

	if time.Now().After(claims.ExpireAt) {
		return nil, errors.New("refresh token expired")
	}

	key := "refresh:" + claims.TokenID

	if cfg.Store != nil {

		_, err = cfg.Store.Get(key)

		if err != nil {
			return nil, errors.New("refresh token invalid")
		}

		cfg.Store.Del(key)
	}

	return m.GenerateTokenPair(
		claims.UserID,
		claims.Username,
		claims.DeviceID,
	)
}

func Refresh(refreshToken string) (*TokenPair, error) {
	return DefaultManager().Refresh(refreshToken)
}
