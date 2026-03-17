package websocket

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"github.com/gin-gonic/gin"
	nws "nhooyr.io/websocket"
)

func HTTPHandler(config ServerConfig, handler Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveConfigConn(config, nil, w, r, handler)
	})
}

func ManagedHTTPHandler(config ServerConfig, manager *Manager, handler Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveConfigConn(config, manager, w, r, handler)
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

func (s *Server) HandleManaged(path string, manager *Manager, handler Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[path] = handler
	s.mux.Handle(path, ManagedHTTPHandler(s.config, manager, handler))
}

func serveConfigConn(config ServerConfig, manager *Manager, w http.ResponseWriter, req *http.Request, handler Handler) {
	if config.ReadTimeout <= 0 {
		config.ReadTimeout = 30 * time.Second
	}
	if config.WriteTimeout <= 0 {
		config.WriteTimeout = 30 * time.Second
	}
	if config.HeartbeatTimeout <= 0 {
		config.HeartbeatTimeout = 90 * time.Second
	}
	if config.IdleTimeout <= 0 {
		config.IdleTimeout = 2 * time.Minute
	}
	config.Observer = observerOrNoop(config.Observer)
	traceCtx := extractTraceFromHTTPHeaders(req.Context(), req.Header)
	traceCtx, handshakeSpan := startWebsocketSpan(traceCtx, config.Tracer, "websocket.server.handshake",
		attribute.String("http.method", req.Method),
		attribute.String("websocket.path", req.URL.Path),
		attribute.String("websocket.remote_addr", req.RemoteAddr),
	)
	var (
		authResult *AuthResult
		err        error
	)
	if config.Authenticator != nil {
		authResult, err = config.Authenticator(req)
		if err != nil {
			config.Observer.OnAuthFailed(req.URL.Path, err)
			endSpan(handshakeSpan, err)
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
	}
	wsConn, err := nws.Accept(w, req, &nws.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		endSpan(handshakeSpan, err)
		return
	}
	if config.RequireAuth && authResult == nil {
		authErr := authError(req.URL.Path, fmt.Errorf("authentication required"))
		config.Observer.OnAuthFailed(req.URL.Path, authErr)
		endSpan(handshakeSpan, authErr)
		_ = wsConn.Close(nws.StatusPolicyViolation, authErr.Error())
		return
	}
	conn := &Conn{
		conn:      wsConn,
		config:    config,
		identity:  newIdentity(req, authResult),
		observer:  config.Observer,
		tracer:    tracerOrGlobal(config.Tracer),
		traceCtx:  traceCtx,
		lastSeen:  time.Now(),
		closeCode: CloseCodeNormal,
	}
	if manager != nil {
		manager.Register(conn)
		defer manager.Unregister(conn)
	}
	conn.observer.OnConnect(conn.identity)
	endSpan(handshakeSpan, nil)
	ctx, cancel := context.WithCancel(traceCtx)
	defer cancel()
	if config.HeartbeatTimeout > 0 || config.IdleTimeout > 0 {
		go conn.monitorLiveness(ctx)
	}
	handler(ctx, conn)
	_ = conn.CloseWithCode(CloseCodeNormal, nil)
}
