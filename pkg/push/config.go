package push

import (
	"fmt"
	"time"
)

// Config represents the configuration for a push notification client
type Config struct {
	// Provider is the type of push notification provider
	Provider ProviderType `json:"provider" yaml:"provider"`

	// Enabled determines if push notifications are enabled
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Timeout for API requests (default: 30s)
	Timeout time.Duration `json:"timeout" yaml:"timeout"`

	// RetryAttempts is the number of retry attempts for failed requests (default: 3)
	RetryAttempts int `json:"retry_attempts" yaml:"retry_attempts"`

	// FCM specific configuration
	FCM FCMConfig `json:"fcm" yaml:"fcm"`

	// APNs specific configuration
	APNs APNsConfig `json:"apns" yaml:"apns"`
}

// FCMConfig represents Google Firebase Cloud Messaging configuration
type FCMConfig struct {
	// ServiceAccountKeyPath is the path to the service account JSON key file
	ServiceAccountKeyPath string `json:"service_account_key_path" yaml:"service_account_key_path"`

	// ServiceAccountKey is the raw service account JSON key (alternative to file path)
	ServiceAccountKey string `json:"service_account_key" yaml:"service_account_key"`

	// ProjectID is the Firebase project ID
	ProjectID string `json:"project_id" yaml:"project_id"`

	// Credentials is the parsed service account credentials
	Credentials ServiceAccountCredentials `json:"-" yaml:"-"`

	// UseLegacyAPI uses the legacy FCM HTTP API instead of HTTP v1 (default: false)
	UseLegacyAPI bool `json:"use_legacy_api" yaml:"use_legacy_api"`

	// LegacyServerKey is the legacy server key (only for legacy API)
	LegacyServerKey string `json:"legacy_server_key" yaml:"legacy_server_key"`
}

// ServiceAccountCredentials represents parsed service account JSON
type ServiceAccountCredentials struct {
	Type                    string `json:"type"`
	ProjectID               string `json:"project_id"`
	PrivateKeyID            string `json:"private_key_id"`
	PrivateKey              string `json:"private_key"`
	ClientEmail             string `json:"client_email"`
	ClientID                string `json:"client_id"`
	AuthURI                 string `json:"auth_uri"`
	TokenURI                string `json:"token_uri"`
	AuthProviderX509CertURL string `json:"auth_provider_x509_cert_url"`
	ClientX509CertURL       string `json:"client_x509_cert_url"`
}

// APNsConfig represents Apple Push Notification service configuration
type APNsConfig struct {
	// CertPath is the path to the APNs certificate file (.p12 or .pem)
	CertPath string `json:"cert_path" yaml:"cert_path"`

	// KeyPath is the path to the private key file (for .pem format)
	KeyPath string `json:"key_path" yaml:"key_path"`

	// CertPassword is the password for the certificate file
	CertPassword string `json:"cert_password" yaml:"cert_password"`

	// TeamID is the Apple Developer Team ID (for token-based auth)
	TeamID string `json:"team_id" yaml:"team_id"`

	// KeyID is the APNs Auth Key ID (for token-based auth)
	KeyID string `json:"key_id" yaml:"key_id"`

	// BundleID is the app bundle identifier (e.g., com.example.app)
	BundleID string `json:"bundle_id" yaml:"bundle_id"`

	// AuthKey is the raw p8 auth key content (for token-based auth)
	AuthKey string `json:"auth_key" yaml:"auth_key"`

	// AuthKeyPath is the path to the p8 auth key file (alternative to AuthKey)
	AuthKeyPath string `json:"auth_key_path" yaml:"auth_key_path"`

	// UseTokenAuth uses JWT token-based authentication instead of certificate-based (default: true)
	UseTokenAuth bool `json:"use_token_auth" yaml:"use_token_auth"`

	// UseSandbox uses the APNs sandbox/development environment (default: false)
	UseSandbox bool `json:"use_sandbox" yaml:"use_sandbox"`
}

// DefaultConfig returns a default configuration
func DefaultConfig() Config {
	return Config{
		Enabled:       true,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
		FCM: FCMConfig{
			UseLegacyAPI: false,
		},
		APNs: APNsConfig{
			UseTokenAuth: true,
			UseSandbox:   false,
		},
	}
}

// Validate validates the configuration
func (c Config) Validate() error {
	if !c.Enabled {
		return nil
	}

	if !c.Provider.IsValid() {
		return fmt.Errorf("invalid provider type: %s", c.Provider)
	}

	switch c.Provider {
	case ProviderTypeFCM:
		if err := c.FCM.Validate(); err != nil {
			return fmt.Errorf("FCM config validation failed: %w", err)
		}
	case ProviderTypeAPNs:
		if err := c.APNs.Validate(); err != nil {
			return fmt.Errorf("APNs config validation failed: %w", err)
		}
	}

	return nil
}

// Validate validates FCM configuration
func (c FCMConfig) Validate() error {
	if c.UseLegacyAPI {
		if c.LegacyServerKey == "" {
			return fmt.Errorf("legacy server key is required when using legacy API")
		}
	} else {
		if c.ServiceAccountKeyPath == "" && c.ServiceAccountKey == "" {
			return fmt.Errorf("service account key path or key content is required")
		}
		if c.ProjectID == "" && c.Credentials.ProjectID == "" {
			return fmt.Errorf("project ID is required")
		}
	}
	return nil
}

// Validate validates APNs configuration
func (c APNsConfig) Validate() error {
	if c.BundleID == "" {
		return fmt.Errorf("bundle ID is required")
	}

	if c.UseTokenAuth {
		if c.TeamID == "" {
			return fmt.Errorf("team ID is required for token-based authentication")
		}
		if c.KeyID == "" {
			return fmt.Errorf("key ID is required for token-based authentication")
		}
		if c.AuthKey == "" && c.AuthKeyPath == "" {
			return fmt.Errorf("auth key content or path is required for token-based authentication")
		}
	} else {
		if c.CertPath == "" {
			return fmt.Errorf("certificate path is required for certificate-based authentication")
		}
	}
	return nil
}

// GetProjectID returns the effective project ID
func (c FCMConfig) GetProjectID() string {
	if c.ProjectID != "" {
		return c.ProjectID
	}
	return c.Credentials.ProjectID
}
