package jwtx

import (
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
)

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func GenerateTokenPair(userID, username, deviceID string) (*TokenPair, error) {

	now := time.Now()

	version := 1

	if config.Store != nil {

		v, err := config.Store.Get("user:version:" + userID)

		if err == nil {

			i, _ := strconv.Atoi(v)

			version = i
		}
	}

	access := Claims{
		UserID:   userID,
		Username: username,
		DeviceID: deviceID,
		TokenID:  uuid.NewString(),

		Version: version,
		Type:    TokenTypeAccess,

		IssuedAt: now,
		ExpireAt: now.Add(config.AccessTokenTTL),
	}

	refresh := Claims{
		UserID:   userID,
		Username: username,
		DeviceID: deviceID,
		TokenID:  uuid.NewString(),

		Version: version,
		Type:    TokenTypeRefresh,

		IssuedAt: now,
		ExpireAt: now.Add(config.RefreshTokenTTL),
	}

	accessToken, err := encodeClaims(access)
	if err != nil {
		return nil, err
	}

	refreshToken, err := encodeClaims(refresh)
	if err != nil {
		return nil, err
	}

	if config.Store != nil {

		config.Store.Set(
			"refresh:"+refresh.TokenID,
			userID,
			config.RefreshTokenTTL,
		)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func ValidateAccessToken(token string) (*Claims, error) {

	claims, err := decodeClaims(token)
	if err != nil {
		return nil, err
	}

	if claims.Type != TokenTypeAccess {
		return nil, errors.New("invalid token type")
	}

	if time.Now().After(claims.ExpireAt) {
		return nil, errors.New("token expired")
	}

	if config.Store != nil {

		_, err = config.Store.Get("blacklist:" + claims.TokenID)

		if err == nil {
			return nil, errors.New("token revoked")
		}

		v, err := config.Store.Get("user:version:" + claims.UserID)

		if err == nil {

			ver, _ := strconv.Atoi(v)

			if ver != claims.Version {
				return nil, errors.New("token invalid")
			}
		}
	}

	return claims, nil
}
