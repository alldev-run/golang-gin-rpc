package middleware

import (
	"github.com/gin-gonic/gin"
	"alldev-gin-rpc/pkg/tracing"
)

// Tracing creates a new Gin tracing middleware
func Tracing(serviceName string) gin.HandlerFunc {
	return tracing.GinMiddleware(serviceName)
}