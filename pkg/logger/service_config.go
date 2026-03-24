package logger

// ServiceLoggerConfig holds service-specific logger configuration
type ServiceLoggerConfig struct {
	// ServiceName is the name of the service (e.g., "api-gateway", "rpc", "websocket")
	ServiceName string `yaml:"service_name" json:"service_name"`
	
	// BaseDir is the base directory for all logs (default: "./logs")
	BaseDir string `yaml:"base_dir" json:"base_dir"`
	
	// EnableDateFolder enables date-based subdirectories (default: true)
	EnableDateFolder bool `yaml:"enable_date_folder" json:"enable_date_folder"`
	
	// SeparateByLevel enables separate files for different log levels (default: false)
	SeparateByLevel bool `yaml:"separate_by_level" json:"separate_by_level"`
	
	// InheritGlobalConfig determines if service logger should inherit global config
	InheritGlobalConfig bool `yaml:"inherit_global_config" json:"inherit_global_config"`
	
	// OverrideConfig allows overriding specific global settings
	OverrideConfig Config `yaml:"override_config" json:"override_config"`
}

// DefaultServiceLoggerConfig returns default service logger configuration
func DefaultServiceLoggerConfig(serviceName string) ServiceLoggerConfig {
	return ServiceLoggerConfig{
		ServiceName:       serviceName,
		BaseDir:           "./logs",
		EnableDateFolder:  true,
		SeparateByLevel:   false,
		InheritGlobalConfig: true,
		OverrideConfig:    DefaultConfig(),
	}
}
