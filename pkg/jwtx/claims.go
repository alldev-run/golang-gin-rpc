// Package jwtx provides JWT token management with access/refresh token pairs,
// automatic token refresh, token revocation, and Gin middleware integration.
package jwtx

import "time"

// TokenType defines the type of JWT token.
type TokenType string

const (
	// TokenTypeAccess represents an access token for short-lived authentication.
	TokenTypeAccess TokenType = "access"
	// TokenTypeRefresh represents a refresh token for obtaining new access tokens.
	TokenTypeRefresh TokenType = "refresh"
)

// Claims represents the custom JWT claims containing user and token metadata.
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`

	DeviceID string `json:"device_id"`
	TokenID  string `json:"token_id"`

	Version int `json:"version"`

	Type TokenType `json:"type"`

	IssuedAt time.Time `json:"issued_at"`
	ExpireAt time.Time `json:"expire_at"`
}
