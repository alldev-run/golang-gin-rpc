// Package logger 提供统一的结构化日志接口，基于 zap + lumberjack 实现
package logger

import (
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	once     sync.Once
	defaultL *zap.Logger
)

// Config 日志配置参数
type Config struct {
	Level      string // "debug", "info", "warn", "error", "fatal", "panic"
	Env        string // "dev" 或 "prod"（或其他）
	LogPath    string // 日志文件路径，例如 "./logs/app.log" 或 "/var/log/myapp/app.log"
	MaxSize    int    // 单个文件最大 MB（默认 100）
	MaxBackups int    // 最多保留旧文件数（默认 30）
	MaxAge     int    // 最多保留天数（默认 28）
	Compress   bool   // 是否压缩旧文件（默认 true）
}

// Init 初始化全局 logger（建议在 main 最开始调用一次）
func Init(cfg Config) {
	once.Do(func() {
		// 默认值补全
		if cfg.Level == "" {
			cfg.Level = "info"
		}
		if cfg.Env == "" {
			cfg.Env = "prod"
		}
		if cfg.MaxSize == 0 {
			cfg.MaxSize = 100
		}
		if cfg.MaxBackups == 0 {
			cfg.MaxBackups = 30
		}
		if cfg.MaxAge == 0 {
			cfg.MaxAge = 28
		}
		if cfg.Compress == false { // 默认开启压缩
			cfg.Compress = true
		}

		// 确定日志级别
		var zapLevel zapcore.Level
		switch cfg.Level {
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

		// 编码器配置
		encoderConfig := zap.NewProductionEncoderConfig()
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
		encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

		// 输出目标
		var cores []zapcore.Core

		// 控制台输出（始终开启，便于本地调试）
		consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)
		if cfg.Env == "dev" {
			// 开发模式：彩色 + 更人性化
			encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
			consoleEncoder = zapcore.NewConsoleEncoder(encoderConfig)
		}
		cores = append(cores, zapcore.NewCore(
			consoleEncoder,
			zapcore.AddSync(os.Stdout),
			zapLevel,
		))

		// 文件输出（如果指定了路径）
		if cfg.LogPath != "" {
			fileWriter := &lumberjack.Logger{
				Filename:   cfg.LogPath,
				MaxSize:    cfg.MaxSize,
				MaxBackups: cfg.MaxBackups,
				MaxAge:     cfg.MaxAge,
				Compress:   cfg.Compress,
				LocalTime:  true,
			}

			// 生产环境用 JSON 格式写文件，便于日志收集系统解析
			fileEncoder := zapcore.NewJSONEncoder(encoderConfig)
			cores = append(cores, zapcore.NewCore(
				fileEncoder,
				zapcore.AddSync(fileWriter),
				zapLevel,
			))
		}

		// 合并多个输出（Tee）
		core := zapcore.NewTee(cores...)

		// 构建 logger
		defaultL = zap.New(core,
			zap.AddCaller(),                       // 记录调用文件+行号
			zap.AddStacktrace(zapcore.ErrorLevel), // error 以上带栈追踪
			zap.AddCallerSkip(1),                  // 跳过 wrapper 层
		)
	})
}

// L 获取全局 logger（如果未初始化，会用默认配置兜底）
func L() *zap.Logger {
	if defaultL == nil {
		// 兜底：未调用 Init 时自动初始化为 info + dev 模式 + 无文件
		Init(Config{Level: "info", Env: "dev"})
	}
	return defaultL
}

// 快捷方法（类似 logrus 风格）

func Debug(msg string, fields ...zap.Field) { L().Debug(msg, fields...) }
func Info(msg string, fields ...zap.Field)  { L().Info(msg, fields...) }
func Warn(msg string, fields ...zap.Field)  { L().Warn(msg, fields...) }
func Errorf(msg string, fields ...zap.Field) { L().Error(msg, fields...) }
func Fatal(msg string, fields ...zap.Field) { L().Fatal(msg, fields...) }
func Panic(msg string, fields ...zap.Field) { L().Panic(msg, fields...) }

// With 创建带默认字段的子 logger（常用于请求上下文、trace 等）
func With(fields ...zap.Field) *zap.Logger {
	return L().With(fields...)
}

// Zap field helpers - 提供常用的字段构造函数
func String(key, val string) zap.Field               { return zap.String(key, val) }
func Int(key string, val int) zap.Field               { return zap.Int(key, val) }
func Int64(key string, val int64) zap.Field           { return zap.Int64(key, val) }
func Float64(key string, val float64) zap.Field       { return zap.Float64(key, val) }
func Bool(key string, val bool) zap.Field             { return zap.Bool(key, val) }
func Duration(key string, val time.Duration) zap.Field { return zap.Duration(key, val) }
func Time(key string, val time.Time) zap.Field       { return zap.Time(key, val) }
func Error(err error) zap.Field                       { return zap.Error(err) }
func Any(key string, val interface{}) zap.Field       { return zap.Any(key, val) }
