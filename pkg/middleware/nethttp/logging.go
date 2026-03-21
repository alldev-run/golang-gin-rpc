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
	httplog.Log(httplog.Fields{
		Method:    r.Method,
		Path:      r.URL.Path,
		ClientIP:  r.RemoteAddr,
		UserAgent: r.UserAgent(),
		Status:    sw.status,
		Latency:   latency,
		RequestID: requestID,
	})
}
