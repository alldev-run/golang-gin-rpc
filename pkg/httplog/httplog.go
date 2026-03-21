package httplog

import (
	"time"

	"alldev-gin-rpc/pkg/logger"
)

type Fields struct {
	Method    string
	Path      string
	ClientIP  string
	UserAgent string
	Status    int
	Latency   time.Duration
	RequestID string
}

func Log(f Fields) {
	logger.Info("HTTP Request",
		logger.String("method", f.Method),
		logger.String("path", f.Path),
		logger.String("client_ip", f.ClientIP),
		logger.Int("status", f.Status),
		logger.Duration("latency", f.Latency),
		logger.String("request_id", f.RequestID),
		logger.String("user_agent", f.UserAgent),
	)
}
