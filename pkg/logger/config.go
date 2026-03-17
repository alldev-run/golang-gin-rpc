package logger

import (
	"os"
	"time"
)

// LogLevel represents the logging level
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
	LogLevelFatal LogLevel = "fatal"
	LogLevelPanic LogLevel = "panic"
)

// LogFormat represents the log format
type LogFormat string

const (
	LogFormatJSON LogFormat = "json"
	LogFormatText LogFormat = "text"
)

// LogOutput represents the log output destination
type LogOutput string

const (
	LogOutputStdout LogOutput = "stdout"
	LogOutputStderr LogOutput = "stderr"
	LogOutputFile   LogOutput = "file"
)

// Config holds logger configuration
type Config struct {
	// Level is the logging level
	Level LogLevel `yaml:"level" json:"level"`
	
	// Env is the deployment environment
	Env string `yaml:"env" json:"env"`
	
	// LogPath is the log file path (when output is "file")
	LogPath string `yaml:"log_path" json:"log_path"`
	
	// Output is the log output destination
	Output LogOutput `yaml:"output" json:"output"`
	
	// Format is the log format
	Format LogFormat `yaml:"format" json:"format"`
	
	// EnableConsoleColors enables console colors
	EnableConsoleColors bool `yaml:"enable_console_colors" json:"enable_console_colors"`
	
	// EnableCaller enables caller information
	EnableCaller bool `yaml:"enable_caller" json:"enable_caller"`
	
	// EnableStacktrace enables stack trace for errors
	EnableStacktrace bool `yaml:"enable_stacktrace" json:"enable_stacktrace"`
	
	// TimeFormat is the time format for logs
	TimeFormat string `yaml:"time_format" json:"time_format"`
	
	// MaxSize is the maximum size of a log file in MB
	MaxSize int `yaml:"max_size" json:"max_size"`
	
	// MaxBackups is the maximum number of old log files to retain
	MaxBackups int `yaml:"max_backups" json:"max_backups"`
	
	// MaxAge is the maximum number of days to retain old log files
	MaxAge int `yaml:"max_age" json:"max_age"`
	
	// Compress indicates if old log files should be compressed
	Compress bool `yaml:"compress" json:"compress"`
	
	// LocalTime indicates if timestamps should use local time
	LocalTime bool `yaml:"local_time" json:"local_time"`
	
	// Fields are additional fields to include in all logs
	Fields map[string]interface{} `yaml:"fields" json:"fields"`
	
	// Sampling configuration
	Sampling SamplingConfig `yaml:"sampling" json:"sampling"`
}

// SamplingConfig holds log sampling configuration
type SamplingConfig struct {
	// Enabled indicates if sampling is enabled
	Enabled bool `yaml:"enabled" json:"enabled"`
	
	// Rate is the sampling rate (0.0 to 1.0)
	Rate float64 `yaml:"rate" json:"rate"`
	
	// Tick is the sampling interval
	Tick time.Duration `yaml:"tick" json:"tick"`
	
	// Initial is the initial burst of logs to allow
	Initial int `yaml:"initial" json:"initial"`
	
	// Thereafter is the rate after initial burst
	Thereafter int `yaml:"thereafter" json:"thereafter"`
}

// DefaultConfig returns default logger configuration
func DefaultConfig() Config {
	return Config{
		Level:              LogLevelInfo,
		Env:                "development",
		LogPath:            "./logs/app.log",
		Output:             LogOutputStdout,
		Format:             LogFormatJSON,
		EnableConsoleColors: true,
		EnableCaller:       true,
		EnableStacktrace:   true,
		TimeFormat:         time.RFC3339,
		MaxSize:            100, // 100MB
		MaxBackups:         10,
		MaxAge:             30, // 30 days
		Compress:           true,
		LocalTime:          false,
		Fields:             make(map[string]interface{}),
		Sampling: SamplingConfig{
			Enabled:    false,
			Rate:       0.1, // 10%
			Tick:       time.Second,
			Initial:    10,
			Thereafter: 100,
		},
	}
}

// DevelopmentConfig returns development-friendly logger configuration
func DevelopmentConfig() Config {
	return Config{
		Level:              LogLevelDebug,
		Env:                "development",
		LogPath:            "./logs/dev.log",
		Output:             LogOutputStdout,
		Format:             LogFormatText,
		EnableConsoleColors: true,
		EnableCaller:       true,
		EnableStacktrace:   true,
		TimeFormat:         "2006-01-02 15:04:05",
		MaxSize:            50, // 50MB
		MaxBackups:         5,
		MaxAge:             7, // 7 days
		Compress:           false,
		LocalTime:          true,
		Fields: map[string]interface{}{
			"env": "development",
		},
		Sampling: SamplingConfig{
			Enabled:    false,
			Rate:       1.0, // 100% in development
			Tick:       time.Second,
			Initial:    100,
			Thereafter: 1000,
		},
	}
}

// ProductionConfig returns production-friendly logger configuration
func ProductionConfig() Config {
	return Config{
		Level:              LogLevelInfo,
		Env:                "production",
		LogPath:            "/var/log/app/app.log",
		Output:             LogOutputFile,
		Format:             LogFormatJSON,
		EnableConsoleColors: false,
		EnableCaller:       false,
		EnableStacktrace:   false,
		TimeFormat:         time.RFC3339,
		MaxSize:            500, // 500MB
		MaxBackups:         30,
		MaxAge:             90, // 90 days
		Compress:           true,
		LocalTime:          false,
		Fields: map[string]interface{}{
			"env": "production",
			"service": "alldev-gin-rpc",
		},
		Sampling: SamplingConfig{
			Enabled:    true,
			Rate:       0.01, // 1% in production
			Tick:       time.Second,
			Initial:    5,
			Thereafter: 10,
		},
	}
}

// TestConfig returns test-friendly logger configuration
func TestConfig() Config {
	return Config{
		Level:              LogLevelDebug,
		Env:                "test",
		LogPath:            "./logs/test.log",
		Output:             LogOutputStdout,
		Format:             LogFormatText,
		EnableConsoleColors: false,
		EnableCaller:       false,
		EnableStacktrace:   false,
		TimeFormat:         time.RFC3339,
		MaxSize:            10, // 10MB
		MaxBackups:         3,
		MaxAge:             1, // 1 day
		Compress:           false,
		LocalTime:          false,
		Fields: map[string]interface{}{
			"env": "test",
		},
		Sampling: SamplingConfig{
			Enabled:    false,
			Rate:       1.0, // 100% in tests
			Tick:       time.Second,
			Initial:    100,
			Thereafter: 1000,
		},
	}
}

// DockerConfig returns Docker-friendly logger configuration
func DockerConfig() Config {
	return Config{
		Level:              LogLevelInfo,
		Env:                "docker",
		LogPath:            "/app/logs/app.log",
		Output:             LogOutputStdout,
		Format:             LogFormatJSON,
		EnableConsoleColors: false,
		EnableCaller:       true,
		EnableStacktrace:   true,
		TimeFormat:         time.RFC3339,
		MaxSize:            100, // 100MB
		MaxBackups:         10,
		MaxAge:             30, // 30 days
		Compress:           true,
		LocalTime:          false,
		Fields: map[string]interface{}{
			"env": "docker",
			"service": "alldev-gin-rpc",
		},
		Sampling: SamplingConfig{
			Enabled:    false,
			Rate:       0.1, // 10%
			Tick:       time.Second,
			Initial:    10,
			Thereafter: 100,
		},
	}
}

// Validate validates the logger configuration
func (c Config) Validate() error {
	if c.Level == "" {
		c.Level = LogLevelInfo
	}
	if c.Env == "" {
		c.Env = "development"
	}
	if c.LogPath == "" {
		c.LogPath = "./logs/app.log"
	}
	if c.Output == "" {
		c.Output = LogOutputStdout
	}
	if c.Format == "" {
		c.Format = LogFormatJSON
	}
	if c.TimeFormat == "" {
		c.TimeFormat = time.RFC3339
	}
	if c.MaxSize == 0 {
		c.MaxSize = 100
	}
	if c.MaxBackups == 0 {
		c.MaxBackups = 10
	}
	if c.MaxAge == 0 {
		c.MaxAge = 30
	}
	if c.Fields == nil {
		c.Fields = make(map[string]interface{})
	}
	if c.Sampling.Tick == 0 {
		c.Sampling.Tick = time.Second
	}
	if c.Sampling.Initial == 0 {
		c.Sampling.Initial = 10
	}
	if c.Sampling.Thereafter == 0 {
		c.Sampling.Thereafter = 100
	}
	
	// Ensure log directory exists
	if c.Output == LogOutputFile && c.LogPath != "" {
		if err := os.MkdirAll(c.LogPath[:len(c.LogPath)-len("/app.log")], 0755); err != nil {
			return err
		}
	}
	
	return nil
}

// IsProduction checks if the environment is production
func (c Config) IsProduction() bool {
	return c.Env == "production" || c.Env == "prod"
}

// IsDevelopment checks if the environment is development
func (c Config) IsDevelopment() bool {
	return c.Env == "development" || c.Env == "dev"
}

// IsTest checks if the environment is test
func (c Config) IsTest() bool {
	return c.Env == "test" || c.Env == "testing"
}

// ShouldEnableConsoleColors checks if console colors should be enabled
func (c Config) ShouldEnableConsoleColors() bool {
	return c.EnableConsoleColors && c.Output != LogOutputFile
}

// WithField adds a field to the configuration
func (c Config) WithField(key string, value interface{}) Config {
	if c.Fields == nil {
		c.Fields = make(map[string]interface{})
	}
	c.Fields[key] = value
	return c
}

// WithFields adds multiple fields to the configuration
func (c Config) WithFields(fields map[string]interface{}) Config {
	if c.Fields == nil {
		c.Fields = make(map[string]interface{})
	}
	for k, v := range fields {
		c.Fields[k] = v
	}
	return c
}
