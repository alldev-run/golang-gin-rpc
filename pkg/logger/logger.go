// Package logger 提供统一的结构化日志接口，基于 zap 实现
package logger

import (
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	once     sync.Once
	defaultL *zap.Logger
)

// Init 初始化全局 logger
//   - level: 日志级别 ("debug", "info", "warn", "error", "fatal", "panic")
//   - env:   环境 ("dev" 或其他)
//   - dev 模式：彩色 console 输出，便于开发调试
//   - prod 模式：JSON 格式，适合生产环境收集
func Init(level string, env string) {
	once.Do(func() {
		var cfg zap.Config

		if env == "dev" {
			// 开发环境：彩色、人类友好格式
			cfg = zap.NewDevelopmentConfig()
			cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
			cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
			cfg.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
		} else {
			// 生产环境：JSON 格式
			cfg = zap.NewProductionConfig()
			cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
			cfg.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
			cfg.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
		}

		// 设置日志级别
		var zapLevel zapcore.Level
		switch level {
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
			zapLevel = zapcore.InfoLevel // 默认 info
		}
		cfg.Level = zap.NewAtomicLevelAt(zapLevel)

		// 输出到 stdout
		cfg.OutputPaths = []string{"stdout"}
		cfg.ErrorOutputPaths = []string{"stderr"}

		var err error
		defaultL, err = cfg.Build(
			zap.AddCaller(),                       // 记录调用者文件和行号
			zap.AddStacktrace(zapcore.ErrorLevel), // error 以上级别带栈追踪
		)
		if err != nil {
			// 初始化失败，直接用 fallback
			defaultL = zap.NewExample()
		}
	})
}

// 获取全局 logger（必须先 Init）
func L() *zap.Logger {
	if defaultL == nil {
		// 未初始化时用默认配置兜底
		Init("info", "dev")
	}
	return defaultL
}

// 常用快捷方法（类似 logrus 的风格）

func Debug(msg string, fields ...zap.Field) {
	L().Debug(msg, fields...)
}

func Info(msg string, fields ...zap.Field) {
	L().Info(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
	L().Warn(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	L().Error(msg, fields...)
}

func Fatal(msg string, fields ...zap.Field) {
	L().Fatal(msg, fields...)
}

func Panic(msg string, fields ...zap.Field) {
	L().Panic(msg, fields...)
}

// With 创建带默认字段的子 logger（常用于中间件、请求上下文）
func With(fields ...zap.Field) *zap.Logger {
	return L().With(fields...)
}
