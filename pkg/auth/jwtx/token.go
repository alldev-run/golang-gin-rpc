package jwtx

import (
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// TokenPair represents a pair of access and refresh tokens.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// GenerateTokenPair generates a new pair of access and refresh tokens for a user.
func (m *Manager) GenerateTokenPair(userID, username, deviceID string) (*TokenPair, error) {
	cfg := m.Config()
	now := time.Now()

	version := 1

	if cfg.Store != nil {

		v, err := cfg.Store.Get("user:version:" + userID)

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
		ExpireAt: now.Add(cfg.AccessTokenTTL),
	}

	refresh := Claims{
		UserID:   userID,
		Username: username,
		DeviceID: deviceID,
		TokenID:  uuid.NewString(),

		Version: version,
		Type:    TokenTypeRefresh,

		IssuedAt: now,
		ExpireAt: now.Add(cfg.RefreshTokenTTL),
	}

	accessToken, err := m.encodeClaims(access)
	if err != nil {
		return nil, err
	}

	refreshToken, err := m.encodeClaims(refresh)
	if err != nil {
		return nil, err
	}

	if cfg.Store != nil {

		cfg.Store.Set(
			"refresh:"+refresh.TokenID,
			userID,
			cfg.RefreshTokenTTL,
		)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// ValidateAccessToken validates an access token and returns its claims.
// Checks token type, expiration, blacklist status, and user version.
func (m *Manager) ValidateAccessToken(token string) (*Claims, error) {
	cfg := m.Config()
	claims, err := m.decodeClaims(token)
	if err != nil {
		return nil, err
	}

	if claims.Type != TokenTypeAccess {
		return nil, errors.New("invalid token type")
	}

	if time.Now().After(claims.ExpireAt) {
		return nil, errors.New("token expired")
	}

	if cfg.Store != nil {

		_, err = cfg.Store.Get("blacklist:" + claims.TokenID)

		if err == nil {
			return nil, errors.New("token revoked")
		}

		v, err := cfg.Store.Get("user:version:" + claims.UserID)

		if err == nil {

			ver, _ := strconv.Atoi(v)

			if ver != claims.Version {
				return nil, errors.New("token invalid")
			}
		}
	}

	return claims, nil
}

func GenerateTokenPair(userID, username, deviceID string) (*TokenPair, error) {
	return DefaultManager().GenerateTokenPair(userID, username, deviceID)
}

func ValidateAccessToken(token string) (*Claims, error) {
	return DefaultManager().ValidateAccessToken(token)
}
