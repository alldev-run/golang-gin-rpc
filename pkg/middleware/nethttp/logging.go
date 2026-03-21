package nethttp

import (
	"net/http"
	"time"

	"alldev-gin-rpc/pkg/httplog"
)

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func Logging() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			start := time.Now()
			next.ServeHTTP(sw, r)
			latency := time.Since(start)
			rid, _ := GetRequestID(r.Context())

			httpLog(sw, r, latency, rid)
		})
	}
}

func httpLog(sw *statusWriter, r *http.Request, latency time.Duration, requestID string) {
	// 使用增强的日志函数，根据状态码记录不同级别的日志
	httplog.LogWithLevel(httplog.Fields{
		Method:    r.Method,
		Path:      r.URL.Path,
		ClientIP:  r.RemoteAddr,
		UserAgent: r.UserAgent(),
		Status:    sw.status,
		Latency:   latency,
		RequestID: requestID,
	})
	
	// 如果是慢请求，额外记录慢请求日志
	if latency > 1*time.Second {
		httplog.LogSlowRequest(httplog.Fields{
			Method:    r.Method,
			Path:      r.URL.Path,
			ClientIP:  r.RemoteAddr,
			UserAgent: r.UserAgent(),
			Status:    sw.status,
			Latency:   latency,
			RequestID: requestID,
		}, 1*time.Second)
	}
}
