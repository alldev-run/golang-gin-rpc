package websocket

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HTTPHandler returns a standalone hardened websocket http.Handler. It goes
// through the same accept path as Server (limits, auth, rate limiting,
// session resume, outbound queues).
func HTTPHandler(config ServerConfig, handler Handler) http.Handler {
	acceptor := newConnAcceptor(config, nil)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		acceptor.serve(w, r, handler)
	})
}

// ManagedHTTPHandler is HTTPHandler with connection registration on the given
// manager (Register on connect, Unregister on disconnect).
func ManagedHTTPHandler(config ServerConfig, manager *Manager, handler Handler) http.Handler {
	acceptor := newConnAcceptor(config, manager)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		acceptor.serve(w, r, handler)
	})
}

func GinHandler(config ServerConfig, handler Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		HTTPHandler(config, handler).ServeHTTP(c.Writer, c.Request)
	}
}

func ManagedGinHandler(config ServerConfig, manager *Manager, handler Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		ManagedHTTPHandler(config, manager, handler).ServeHTTP(c.Writer, c.Request)
	}
}

// HandleManaged registers the handler and routes every accepted connection
// through the manager (tracking + cluster broadcast). The manager applies to
// the whole server; call it once before Start.
func (s *Server) HandleManaged(path string, manager *Manager, handler Handler) {
	s.acceptor.setManager(manager)
	s.Handle(path, handler)
}
