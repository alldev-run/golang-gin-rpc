package push

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.True(t, config.Enabled)
	assert.Equal(t, 30*time.Second, config.Timeout)
	assert.Equal(t, 3, config.RetryAttempts)
	assert.False(t, config.FCM.UseLegacyAPI)
	assert.True(t, config.APNs.UseTokenAuth)
	assert.False(t, config.APNs.UseSandbox)
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "disabled config is valid",
			config: Config{
				Enabled:  false,
				Provider: "",
			},
			wantErr: false,
		},
		{
			name: "empty provider is invalid",
			config: Config{
				Enabled:  true,
				Provider: "",
			},
			wantErr: true,
			errMsg:  "invalid provider type",
		},
		{
			name: "invalid provider is invalid",
			config: Config{
				Enabled:  true,
				Provider: "invalid",
			},
			wantErr: true,
			errMsg:  "invalid provider type",
		},
		{
			name: "FCM config missing credentials",
			config: Config{
				Enabled:  true,
				Provider: ProviderTypeFCM,
				FCM:      FCMConfig{},
			},
			wantErr: true,
			errMsg:  "FCM config validation failed",
		},
		{
			name: "FCM legacy config missing server key",
			config: Config{
				Enabled:  true,
				Provider: ProviderTypeFCM,
				FCM: FCMConfig{
					UseLegacyAPI: true,
				},
			},
			wantErr: true,
			errMsg:  "legacy server key is required",
		},
		{
			name: "APNs config missing bundle ID",
			config: Config{
				Enabled:  true,
				Provider: ProviderTypeAPNs,
				APNs:     APNsConfig{},
			},
			wantErr: true,
			errMsg:  "bundle ID is required",
		},
		{
			name: "APNs token auth missing team ID",
			config: Config{
				Enabled:  true,
				Provider: ProviderTypeAPNs,
				APNs: APNsConfig{
					BundleID:     "com.test.app",
					UseTokenAuth: true,
					KeyID:        "KEY123",
					AuthKey:      "fake-key",
				},
			},
			wantErr: true,
			errMsg:  "team ID is required",
		},
		{
			name: "APNs cert auth missing cert path",
			config: Config{
				Enabled:  true,
				Provider: ProviderTypeAPNs,
				APNs: APNsConfig{
					BundleID:     "com.test.app",
					UseTokenAuth: false,
				},
			},
			wantErr: true,
			errMsg:  "certificate path is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFCMConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  FCMConfig
		wantErr bool
	}{
		{
			name: "valid modern API with service account key",
			config: FCMConfig{
				ServiceAccountKey: `{"type": "service_account", "project_id": "test"}`,
				ProjectID:         "test",
			},
			wantErr: false,
		},
		{
			name: "valid modern API with path",
			config: FCMConfig{
				ServiceAccountKeyPath: "/path/to/key.json",
				ProjectID:             "test",
			},
			wantErr: false,
		},
		{
			name: "missing service account",
			config: FCMConfig{
				UseLegacyAPI: false,
			},
			wantErr: true,
		},
		{
			name: "valid legacy API",
			config: FCMConfig{
				UseLegacyAPI:    true,
				LegacyServerKey: "key123",
			},
			wantErr: false,
		},
		{
			name: "legacy API missing server key",
			config: FCMConfig{
				UseLegacyAPI: true,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFCMConfig_GetProjectID(t *testing.T) {
	tests := []struct {
		name       string
		projectID  string
		credProjID string
		want       string
	}{
		{
			name:       "use explicit project ID",
			projectID:  "explicit-project",
			credProjID: "cred-project",
			want:       "explicit-project",
		},
		{
			name:       "use credentials project ID",
			projectID:  "",
			credProjID: "cred-project",
			want:       "cred-project",
		},
		{
			name:       "both empty",
			projectID:  "",
			credProjID: "",
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := FCMConfig{
				ProjectID:   tt.projectID,
				Credentials: ServiceAccountCredentials{ProjectID: tt.credProjID},
			}
			got := config.GetProjectID()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAPNsConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  APNsConfig
		wantErr bool
	}{
		{
			name: "valid token auth",
			config: APNsConfig{
				BundleID:     "com.test.app",
				UseTokenAuth: true,
				TeamID:       "TEAM123",
				KeyID:        "KEY123",
				AuthKey:      "-----BEGIN EC PRIVATE KEY-----\nfake-key\n-----END EC PRIVATE KEY-----",
			},
			wantErr: false,
		},
		{
			name: "valid cert auth",
			config: APNsConfig{
				BundleID:     "com.test.app",
				UseTokenAuth: false,
				CertPath:     "/path/to/cert.p12",
			},
			wantErr: false,
		},
		{
			name: "missing bundle ID",
			config: APNsConfig{
				BundleID:     "",
				UseTokenAuth: true,
				TeamID:       "TEAM123",
			},
			wantErr: true,
		},
		{
			name: "token auth missing team ID",
			config: APNsConfig{
				BundleID:     "com.test.app",
				UseTokenAuth: true,
				TeamID:       "",
				KeyID:        "KEY123",
				AuthKey:      "fake-key",
			},
			wantErr: true,
		},
		{
			name: "token auth missing key ID",
			config: APNsConfig{
				BundleID:     "com.test.app",
				UseTokenAuth: true,
				TeamID:       "TEAM123",
				KeyID:        "",
				AuthKey:      "fake-key",
			},
			wantErr: true,
		},
		{
			name: "token auth missing auth key",
			config: APNsConfig{
				BundleID:     "com.test.app",
				UseTokenAuth: true,
				TeamID:       "TEAM123",
				KeyID:        "KEY123",
				AuthKey:      "",
				AuthKeyPath:  "",
			},
			wantErr: true,
		},
		{
			name: "cert auth missing cert path",
			config: APNsConfig{
				BundleID:     "com.test.app",
				UseTokenAuth: false,
				CertPath:     "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestServiceAccountCredentials(t *testing.T) {
	creds := ServiceAccountCredentials{
		Type:                    "service_account",
		ProjectID:               "my-project",
		PrivateKeyID:            "key-id",
		PrivateKey:              "private-key",
		ClientEmail:             "test@my-project.iam.gserviceaccount.com",
		ClientID:                "client-id",
		AuthURI:                 "https://accounts.google.com/o/oauth2/auth",
		TokenURI:                "https://oauth2.googleapis.com/token",
		AuthProviderX509CertURL: "https://www.googleapis.com/oauth2/v1/certs",
		ClientX509CertURL:       "https://www.googleapis.com/robot/v1/metadata/x509/test%40my-project.iam.gserviceaccount.com",
	}

	assert.Equal(t, "service_account", creds.Type)
	assert.Equal(t, "my-project", creds.ProjectID)
	assert.Equal(t, "test@my-project.iam.gserviceaccount.com", creds.ClientEmail)
}
