package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"alldev-gin-rpc/api/http-gateway/internal/service"
	"alldev-gin-rpc/pkg/gateway"
	"alldev-gin-rpc/pkg/middleware/nethttp"
)

type Router struct {
	helloSvc *service.HelloService
	cfg      *gateway.Config
}

func NewRouter(cfg *gateway.Config) *Router {
	return &Router{helloSvc: service.NewHelloService(), cfg: cfg}
}

func (r *Router) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", r.root)
	mux.HandleFunc("/debug/request-id", r.debugRequestID)
	mux.HandleFunc("/debug/ok", r.debugOK)

	return nethttp.Chain(
		mux,
		nethttp.Recovery(),
		nethttp.RequestID(),
		nethttp.CORSFromGatewayConfig(r.cfg),
		nethttp.RateLimitFromGatewayConfig(r.cfg),
		nethttp.Logging(),
	)
}

func (r *Router) root(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		http.NotFound(w, req)
		return
	}
	writeJSON(w, http.StatusOK, r.helloSvc.Hello())
}

func (r *Router) debugOK(w http.ResponseWriter, req *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (r *Router) debugRequestID(w http.ResponseWriter, req *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"request_id_hdr":  req.Header.Get("X-Request-ID"),
		"request_id_resp": w.Header().Get("X-Request-ID"),
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func IsBusinessPath(path string) bool {
	if path == "/" {
		return true
	}
	return strings.HasPrefix(path, "/debug/")
}
