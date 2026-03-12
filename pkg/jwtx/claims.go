package jwtx

import "time"

type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

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
