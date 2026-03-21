package nethttp

import (
	"net/http"

	"alldev-gin-rpc/pkg/logger"
	"alldev-gin-rpc/pkg/panicx"
)

func Recovery() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					rid, _ := GetRequestID(r.Context())
					logger.Errorf("Panic recovered",
						logger.String("error", panicx.ErrorString(err)),
						logger.String("method", r.Method),
						logger.String("path", r.URL.Path),
						logger.String("request_id", rid),
						logger.String("stack", panicx.Stack()),
					)
					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte("{\"error\":\"Internal server error\"}"))
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
