package jwtx

import (
	"errors"
	"time"
)

// Refresh validates a refresh token and generates a new token pair.
// Deletes the used refresh token from the store after successful validation.
func Refresh(refreshToken string) (*TokenPair, error) {

	claims, err := decodeClaims(refreshToken)
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

	if config.Store != nil {

		_, err = config.Store.Get(key)

		if err != nil {
			return nil, errors.New("refresh token invalid")
		}

		config.Store.Del(key)
	}

	return GenerateTokenPair(
		claims.UserID,
		claims.Username,
		claims.DeviceID,
	)
}
