package jwtx

import (
	"errors"
	"time"
)

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
