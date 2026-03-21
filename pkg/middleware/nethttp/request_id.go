package nethttp

import (
	"context"
	"net/http"

	"alldev-gin-rpc/pkg/requestid"
)

type requestIDCtxKey struct{}

func RequestID() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-ID")
			if id == "" {
				id = requestid.MustNew()
			}
			w.Header().Set("X-Request-ID", id)
			ctx := context.WithValue(r.Context(), requestIDCtxKey{}, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetRequestID(ctx context.Context) (string, bool) {
	v := ctx.Value(requestIDCtxKey{})
	if v == nil {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}
