package mw

import "net/http"

func init() {
	Register("demo", 100, Demo())
}

func Demo() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Demo-MW", "1")
			if r.Header.Get("X-Demo-Block") == "1" {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte("{\"error\":\"blocked by demo middleware\"}"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
