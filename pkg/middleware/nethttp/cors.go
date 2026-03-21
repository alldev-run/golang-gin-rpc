package nethttp

import (
	"net/http"

	"alldev-gin-rpc/pkg/cors"
	"alldev-gin-rpc/pkg/gateway"
)

func CORSFromGatewayConfig(cfg *gateway.Config) Middleware {
	if cfg == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	gc := cfg.CORS
	cc := cors.Config{
		AllowedOrigins:   gc.AllowedOrigins,
		AllowedMethods:   gc.AllowedMethods,
		AllowedHeaders:   gc.AllowedHeaders,
		ExposedHeaders:   gc.ExposedHeaders,
		AllowCredentials: gc.AllowCredentials,
		MaxAge:           gc.MaxAge,
		OptionsPassthrough: false,
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if handled := cors.Apply(w, r, cc); handled {
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
