package rpc

import (
	"sync/atomic"

	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
	"go.uber.org/zap"
)

var injectedLogger atomic.Pointer[zap.Logger]

// SetLogger injects the logger used by this package. Passing nil restores the
// default, pkg/logger's global logger (logger.Base).
func SetLogger(l *zap.Logger) {
	injectedLogger.Store(l)
}

func pluginLogger() *zap.Logger {
	if l := injectedLogger.Load(); l != nil {
		return l
	}
	return logger.Base()
}
