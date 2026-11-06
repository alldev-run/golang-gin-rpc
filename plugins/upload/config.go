package upload

// Config represents the core upload configuration
type Config struct {
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

	// Enable auto-creation of upload directory
	AutoCreateDir bool `yaml:"auto_create_dir" json:"auto_create_dir"`

	// Enable file overwrite
	EnableOverwrite bool `yaml:"enable_overwrite" json:"enable_overwrite"`
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
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
		AutoCreateDir:     true,
		EnableOverwrite:   false,
	}
}
