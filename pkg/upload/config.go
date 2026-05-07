package upload

// Config represents the upload configuration
type Config struct {
	// Enable upload functionality
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Upload directory
	UploadDir string `yaml:"upload_dir" json:"upload_dir"`

	// Max file size in bytes (default: 10MB)
	MaxFileSize int64 `yaml:"max_file_size" json:"max_file_size"`

	// Allowed file extensions (e.g., [".jpg", ".png", ".pdf"])
	AllowedExtensions []string `yaml:"allowed_extensions" json:"allowed_extensions"`

	// Allowed MIME types (e.g., ["image/jpeg", "image/png", "application/pdf"])
	AllowedMimeTypes []string `yaml:"allowed_mime_types" json:"allowed_mime_types"`

	// File naming strategy: "uuid", "timestamp", "original", "custom"
	NamingStrategy string `yaml:"naming_strategy" json:"naming_strategy"`

	// Custom naming template (used when naming_strategy is "custom")
	// Supported placeholders: {uuid}, {timestamp}, {date}, {original}, {random}
	CustomNameTemplate string `yaml:"custom_name_template" json:"custom_name_template"`

	// Preserve original extension
	PreserveExtension bool `yaml:"preserve_extension" json:"preserve_extension"`

	// Enable CORS
	EnableCORS bool `yaml:"enable_cors" json:"enable_cors"`

	// CORS configuration
	CORS CORSConfig `yaml:"cors" json:"cors"`

	// Server port (for standalone upload server)
	Port int `yaml:"port" json:"port"`

	// Enable standalone upload server
	EnableServer bool `yaml:"enable_server" json:"enable_server"`

	// Enable auto-creation of upload directory
	AutoCreateDir bool `yaml:"auto_create_dir" json:"auto_create_dir"`

	// Enable file overwrite
	EnableOverwrite bool `yaml:"enable_overwrite" json:"enable_overwrite"`

	// Enable authentication
	EnableAuth bool `yaml:"enable_auth" json:"enable_auth"`

	// Authentication username
	AuthUsername string `yaml:"auth_username" json:"auth_username"`

	// Authentication password
	AuthPassword string `yaml:"auth_password" json:"auth_password"`

	// Token signing secret for short-lived access signatures
	TokenSecret string `yaml:"token_secret" json:"token_secret"`

	// Token expiration in seconds
	TokenTTLSeconds int64 `yaml:"token_ttl_seconds" json:"token_ttl_seconds"`
}

// CORSConfig represents CORS configuration
type CORSConfig struct {
	// Allowed origins (e.g., ["*"] or ["https://example.com"])
	AllowedOrigins []string `yaml:"allowed_origins" json:"allowed_origins"`

	// Allowed methods
	AllowedMethods []string `yaml:"allowed_methods" json:"allowed_methods"`

	// Allowed headers
	AllowedHeaders []string `yaml:"allowed_headers" json:"allowed_headers"`

	// Exposed headers
	ExposedHeaders []string `yaml:"exposed_headers" json:"exposed_headers"`

	// Allow credentials
	AllowCredentials bool `yaml:"allow_credentials" json:"allow_credentials"`

	// Max age for preflight requests (in seconds)
	MaxAge int `yaml:"max_age" json:"max_age"`
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
		Enabled:           true,
		UploadDir:         "./uploads",
		MaxFileSize:       10 * 1024 * 1024, // 10MB
		AllowedExtensions: []string{".jpg", ".jpeg", ".png", ".gif", ".pdf", ".doc", ".docx", ".xls", ".xlsx"},
		AllowedMimeTypes: []string{
			"image/jpeg",
			"image/png",
			"image/gif",
			"application/pdf",
			"application/msword",
			"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			"application/vnd.ms-excel",
			"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		},
		NamingStrategy:    "uuid",
		PreserveExtension: true,
		EnableCORS:        true,
		CORS: CORSConfig{
			AllowedOrigins:   []string{"*"},
			AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
			AllowedHeaders:   []string{"Origin", "Content-Type", "Accept", "Authorization"},
			AllowCredentials: false,
			MaxAge:           86400, // 24 hours
		},
		Port:            8081,
		EnableServer:    false,
		AutoCreateDir:   true,
		EnableOverwrite: false,
		EnableAuth:      false,
		AuthUsername:    "",
		AuthPassword:    "",
		TokenSecret:     "",
		TokenTTLSeconds: 300,
	}
}
