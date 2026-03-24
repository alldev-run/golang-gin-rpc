package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	serviceLoggers = make(map[string]*zap.Logger)
	serviceMutex   sync.RWMutex
)

// GetServiceLogger gets or creates a service-specific logger
func GetServiceLoggerInstance(serviceName string, config ServiceLoggerConfig) *zap.Logger {
	serviceMutex.RLock()
	if logger, exists := serviceLoggers[serviceName]; exists {
		serviceMutex.RUnlock()
		return logger
	}
	serviceMutex.RUnlock()

	// Create new logger
	serviceMutex.Lock()
	defer serviceMutex.Unlock()

	// Double-check after acquiring write lock
	if logger, exists := serviceLoggers[serviceName]; exists {
		return logger
	}

	logger := createServiceLogger(serviceName, config)
	serviceLoggers[serviceName] = logger
	return logger
}

// createServiceLogger creates a new service-specific logger
func createServiceLogger(serviceName string, config ServiceLoggerConfig) *zap.Logger {
	// Build final configuration
	var finalConfig Config
	if config.InheritGlobalConfig && defaultL != nil {
		// Use global config as base
		finalConfig = DefaultConfig() // We'll improve this later
	} else {
		finalConfig = config.OverrideConfig
	}

	// Override service-specific settings
	finalConfig = finalConfig.WithField("service", serviceName)

	// Determine log level
	var zapLevel zapcore.Level
	switch string(finalConfig.Level) {
	case "debug":
		zapLevel = zapcore.DebugLevel
	case "info":
		zapLevel = zapcore.InfoLevel
	case "warn":
		zapLevel = zapcore.WarnLevel
	case "error":
		zapLevel = zapcore.ErrorLevel
	case "fatal", "panic":
		zapLevel = zapcore.FatalLevel
	default:
		zapLevel = zapcore.InfoLevel
	}

	// Create encoder config
	encoderConfig := zap.NewProductionEncoderConfig()
	if finalConfig.TimeFormat != "" {
		encoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendString(t.Format(finalConfig.TimeFormat))
		}
	} else {
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	}
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	if finalConfig.EnableCaller {
		encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	}

	// Create cores for different outputs
	var cores []zapcore.Core

	// Add console output if configured
	if finalConfig.Output == LogOutputStdout || finalConfig.Output == LogOutputStderr {
		consoleEncoderConfig := encoderConfig
		if finalConfig.Format != LogFormatJSON && finalConfig.EnableConsoleColors {
			consoleEncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		}

		var encoder zapcore.Encoder
		if finalConfig.Format == LogFormatJSON {
			encoder = zapcore.NewJSONEncoder(consoleEncoderConfig)
		} else {
			encoder = zapcore.NewConsoleEncoder(consoleEncoderConfig)
		}

		var writer zapcore.WriteSyncer
		if finalConfig.Output == LogOutputStderr {
			writer = zapcore.AddSync(os.Stderr)
		} else {
			writer = zapcore.AddSync(os.Stdout)
		}

		cores = append(cores, zapcore.NewCore(encoder, writer, zapLevel))
	}

	// Add file output with service-specific paths
	if finalConfig.Output == LogOutputFile || finalConfig.LogPath != "" {
		fileCores := createServiceFileCores(serviceName, config, finalConfig, encoderConfig, zapLevel)
		cores = append(cores, fileCores...)
	}

	// Combine cores
	core := zapcore.NewTee(cores...)

	// Create logger with options
	options := []zap.Option{
		zap.AddCallerSkip(1),
		zap.AddCaller(),
	}

	if finalConfig.EnableStacktrace {
		options = append(options, zap.AddStacktrace(zapcore.ErrorLevel))
	}

	// Add service field
	logger := zap.New(core, options...).With(zap.String("service", serviceName))

	return logger
}

// createServiceFileCores creates file cores for service logger
func createServiceFileCores(serviceName string, serviceConfig ServiceLoggerConfig, config Config, encoderConfig zapcore.EncoderConfig, level zapcore.Level) []zapcore.Core {
	var cores []zapcore.Core

	// Build log directory path
	baseDir := serviceConfig.BaseDir
	if baseDir == "" {
		baseDir = "./logs"
	}

	var logDir string
	if serviceConfig.EnableDateFolder {
		today := time.Now().Format("2006-01-02")
		logDir = filepath.Join(baseDir, serviceName, today)
	} else {
		logDir = filepath.Join(baseDir, serviceName)
	}

	// Ensure directory exists
	if err := os.MkdirAll(logDir, 0755); err != nil {
		// Fallback to console if directory creation fails
		return cores
	}

	if serviceConfig.SeparateByLevel {
		// Create separate files for different levels
		levelConfigs := []struct {
			level      string
			zapLevel   zapcore.Level
			suffix     string
			shouldLog  bool
		}{
			{"debug", zapcore.DebugLevel, "debug", level <= zapcore.DebugLevel},
			{"info", zapcore.InfoLevel, "info", level <= zapcore.InfoLevel},
			{"warn", zapcore.WarnLevel, "warn", level <= zapcore.WarnLevel},
			{"error", zapcore.ErrorLevel, "error", level <= zapcore.ErrorLevel},
		}

		for _, lc := range levelConfigs {
			if !lc.shouldLog {
				continue
			}

			filename := filepath.Join(logDir, fmt.Sprintf("%s.%s.log", serviceName, lc.suffix))
			fileWriter := &lumberjack.Logger{
				Filename:   filename,
				MaxSize:    config.MaxSize,
				MaxBackups: config.MaxBackups,
				MaxAge:     config.MaxAge,
				Compress:   config.Compress,
				LocalTime:  config.LocalTime,
			}

			// Use JSON format for files
			fileEncoder := zapcore.NewJSONEncoder(encoderConfig)
			cores = append(cores, zapcore.NewCore(fileEncoder, zapcore.AddSync(fileWriter), lc.zapLevel))
		}
	} else {
		// Single file for all levels
		filename := filepath.Join(logDir, fmt.Sprintf("%s.log", serviceName))
		fileWriter := &lumberjack.Logger{
			Filename:   filename,
			MaxSize:    config.MaxSize,
			MaxBackups: config.MaxBackups,
			MaxAge:     config.MaxAge,
			Compress:   config.Compress,
			LocalTime:  config.LocalTime,
		}

		// Use JSON format for files
		fileEncoder := zapcore.NewJSONEncoder(encoderConfig)
		cores = append(cores, zapcore.NewCore(fileEncoder, zapcore.AddSync(fileWriter), level))
	}

	return cores
}

// CleanupOldLogs removes old log directories based on retention policy
func CleanupOldLogs(baseDir string, maxAgeDays int) error {
	if maxAgeDays <= 0 {
		return nil
	}

	cutoff := time.Now().AddDate(0, 0, -maxAgeDays)
	var dirsToDelete []string

	// First pass: collect directories to delete
	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			return nil
		}

		// Check if this is a date directory (format: YYYY-MM-DD)
		dirName := filepath.Base(path)
		if dirDate, parseErr := time.Parse("2006-01-02", dirName); parseErr == nil {
			if dirDate.Before(cutoff) {
				dirsToDelete = append(dirsToDelete, path)
				return filepath.SkipDir
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Second pass: delete collected directories
	for _, dir := range dirsToDelete {
		if removeErr := os.RemoveAll(dir); removeErr != nil {
			return removeErr
		}
	}

	return nil
}

// GetServiceLogPath returns the current log path for a service
func GetServiceLogPath(serviceName string, config ServiceLoggerConfig) string {
	baseDir := config.BaseDir
	if baseDir == "" {
		baseDir = "./logs"
	}

	var logDir string
	if config.EnableDateFolder {
		today := time.Now().Format("2006-01-02")
		logDir = filepath.Join(baseDir, serviceName, today)
	} else {
		logDir = filepath.Join(baseDir, serviceName)
	}

	if config.SeparateByLevel {
		return filepath.Join(logDir, fmt.Sprintf("%s.info.log", serviceName))
	}

	return filepath.Join(logDir, fmt.Sprintf("%s.log", serviceName))
}

// ServiceLogger provides convenient methods for service-specific logging
type ServiceLogger struct {
	serviceName string
	logger      *zap.Logger
}

// Logger returns the underlying zap logger
func (sl *ServiceLogger) Logger() *zap.Logger {
	return sl.logger
}

// With creates a logger with additional fields
func (sl *ServiceLogger) With(fields ...zap.Field) *ServiceLogger {
	return &ServiceLogger{
		serviceName: sl.serviceName,
		logger:      sl.logger.With(fields...),
	}
}

// Convenience methods
func (sl *ServiceLogger) Debug(msg string, fields ...zap.Field) {
	sl.logger.Debug(msg, fields...)
}

func (sl *ServiceLogger) Info(msg string, fields ...zap.Field) {
	sl.logger.Info(msg, fields...)
}

func (sl *ServiceLogger) Warn(msg string, fields ...zap.Field) {
	sl.logger.Warn(msg, fields...)
}

func (sl *ServiceLogger) Error(msg string, fields ...zap.Field) {
	sl.logger.Error(msg, fields...)
}

func (sl *ServiceLogger) Fatal(msg string, fields ...zap.Field) {
	sl.logger.Fatal(msg, fields...)
}

func (sl *ServiceLogger) Panic(msg string, fields ...zap.Field) {
	sl.logger.Panic(msg, fields...)
}
