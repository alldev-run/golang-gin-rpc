package gateway

import (
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"alldev-gin-rpc/pkg/metrics"
)

var (
	metricsOnce sync.Once
	collector   *metrics.MetricsCollector
)

func initMetrics() {
	metricsOnce.Do(func() {
		collector = metrics.NewMetricsCollector()
	})
}

func metricsMiddleware() gin.HandlerFunc {
	initMetrics()
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		status := c.Writer.Status()
		collector.RecordHTTPRequest(c.Request.Method, path, metricsStatusCode(status), time.Since(start))
	}
}

func observeUpstreamError(service string, typ string) {
	initMetrics()
	collector.RecordRPCError(service, "upstream", typ)
}

func metricsStatusCode(code int) string {
	if code <= 0 {
		return "0"
	}
	return strconv.Itoa(code)
}
