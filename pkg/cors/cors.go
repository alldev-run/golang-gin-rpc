package cors

import (
	"net/http"
	"strconv"
	"strings"
)

type Config struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           int
	OptionsPassthrough bool
}

// Apply writes CORS headers to w based on cfg and r.
// If the request is a preflight (OPTIONS), it writes 204 and returns true (handled).
func Apply(w http.ResponseWriter, r *http.Request, cfg Config) (handled bool) {
	origin := r.Header.Get("Origin")
	allowedOrigin := resolveAllowedOrigin(origin, cfg.AllowedOrigins)

	if allowedOrigin != "" {
		w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		if cfg.AllowCredentials {
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		if len(cfg.ExposedHeaders) > 0 {
			w.Header().Set("Access-Control-Expose-Headers", strings.Join(cfg.ExposedHeaders, ", "))
		}
	}
	if len(cfg.AllowedMethods) > 0 {
		w.Header().Set("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ", "))
	}
	if len(cfg.AllowedHeaders) > 0 {
		w.Header().Set("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ", "))
	}
	if cfg.MaxAge > 0 {
		w.Header().Set("Access-Control-Max-Age", strconv.Itoa(cfg.MaxAge))
	}

	if r.Method == http.MethodOptions {
		if cfg.OptionsPassthrough {
			return false
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	return false
}

func resolveAllowedOrigin(origin string, allowedOrigins []string) string {
	if origin == "" {
		return ""
	}
	for _, o := range allowedOrigins {
		if o == "*" {
			return "*"
		}
		if o == origin {
			return origin
		}
		if strings.HasPrefix(o, "*.") {
			domain := strings.TrimPrefix(o, "*.")
			if strings.HasSuffix(origin, domain) {
				return origin
			}
		}
	}
	return ""
}
