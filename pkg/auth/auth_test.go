package auth

import (
	"testing"
	"alldev-gin-rpc/pkg/auth/jwtx"
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
					Secret: "test-secret",
				},
			},
			want: &AuthManager{
				config: AuthConfig{
					Enabled: true,
					JWT: jwtx.Config{
						Secret: "test-secret",
					},
				},
			},
		},
		{
			name: "disabled auth",
			config: AuthConfig{
				Enabled: false,
				JWT: jwtx.Config{
					Secret: "test-secret",
				},
			},
			want: &AuthManager{
				config: AuthConfig{
					Enabled: false,
					JWT: jwtx.Config{
						Secret: "test-secret",
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
			if got.config.JWT.Secret != tt.want.config.JWT.Secret {
				t.Errorf("NewAuthManager().config.JWT.Secret = %v, want %v", got.config.JWT.Secret, tt.want.config.JWT.Secret)
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

func TestAuthConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  AuthConfig
		wantErr bool
	}{
		{
			name: "disabled auth allows empty secret",
			config: AuthConfig{
				Enabled: false,
			},
		},
		{
			name: "enabled auth requires secret",
			config: AuthConfig{
				Enabled: true,
			},
			wantErr: true,
		},
		{
			name: "enabled auth with secret is valid",
			config: AuthConfig{
				Enabled: true,
				JWT: jwtx.Config{
					Secret: "test-secret",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected validation error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestNewAuthManager_InvalidConfigIsNotReady(t *testing.T) {
	manager := NewAuthManager(AuthConfig{
		Enabled: true,
	})

	if manager.IsReady() {
		t.Fatal("expected auth manager to be not ready")
	}
	if manager.ValidationError() == nil {
		t.Fatal("expected validation error to be exposed")
	}
}

func TestNewAuthManager_ValidConfigIsReady(t *testing.T) {
	manager := NewAuthManager(AuthConfig{
		Enabled: true,
		JWT: jwtx.Config{
			Secret: "test-secret",
		},
	})

	if !manager.IsReady() {
		t.Fatal("expected auth manager to be ready")
	}
	if manager.ValidationError() != nil {
		t.Fatalf("unexpected validation error: %v", manager.ValidationError())
	}
}
