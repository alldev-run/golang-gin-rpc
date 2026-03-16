package auth

import (
	"testing"
	"golang-gin-rpc/pkg/auth/jwtx"
)

func TestNewAuthManager(t *testing.T) {
	tests := []struct {
		name   string
		config AuthConfig
		want   *AuthManager
	}{
		{
			name: "enabled auth",
			config: AuthConfig{
				Enabled: true,
				JWT: jwtx.Config{
					SecretKey: "test-secret",
					Issuer:    "test-issuer",
				},
			},
			want: &AuthManager{
				config: AuthConfig{
					Enabled: true,
					JWT: jwtx.Config{
						SecretKey: "test-secret",
						Issuer:    "test-issuer",
					},
				},
			},
		},
		{
			name: "disabled auth",
			config: AuthConfig{
				Enabled: false,
				JWT: jwtx.Config{
					SecretKey: "test-secret",
					Issuer:    "test-issuer",
				},
			},
			want: &AuthManager{
				config: AuthConfig{
					Enabled: false,
					JWT: jwtx.Config{
						SecretKey: "test-secret",
						Issuer:    "test-issuer",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewAuthManager(tt.config)
			if got == nil {
				t.Fatal("NewAuthManager() returned nil")
			}
			if got.config.Enabled != tt.want.config.Enabled {
				t.Errorf("NewAuthManager().config.Enabled = %v, want %v", got.config.Enabled, tt.want.config.Enabled)
			}
			if got.config.JWT.SecretKey != tt.want.config.JWT.SecretKey {
				t.Errorf("NewAuthManager().config.JWT.SecretKey = %v, want %v", got.config.JWT.SecretKey, tt.want.config.JWT.SecretKey)
			}
			if got.config.JWT.Issuer != tt.want.config.JWT.Issuer {
				t.Errorf("NewAuthManager().config.JWT.Issuer = %v, want %v", got.config.JWT.Issuer, tt.want.config.JWT.Issuer)
			}
		})
	}
}

func TestAuthManager_IsEnabled(t *testing.T) {
	tests := []struct {
		name   string
		config AuthConfig
		want   bool
	}{
		{
			name: "enabled auth",
			config: AuthConfig{
				Enabled: true,
			},
			want: true,
		},
		{
			name: "disabled auth",
			config: AuthConfig{
				Enabled: false,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			am := NewAuthManager(tt.config)
			if got := am.IsEnabled(); got != tt.want {
				t.Errorf("AuthManager.IsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}
