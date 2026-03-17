package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	pkgtracing "alldev-gin-rpc/pkg/tracing"
	"go.opentelemetry.io/otel/attribute"
	nws "nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

type ServerConfig struct {
	Addr             string        `yaml:"addr" json:"addr"`
	Path             string        `yaml:"path" json:"path"`
	ReadTimeout      time.Duration `yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout     time.Duration `yaml:"write_timeout" json:"write_timeout"`
	RequireAuth      bool          `yaml:"require_auth" json:"require_auth"`
	HeartbeatTimeout time.Duration `yaml:"heartbeat_timeout" json:"heartbeat_timeout"`
	IdleTimeout      time.Duration `yaml:"idle_timeout" json:"idle_timeout"`
	Authenticator    Authenticator `yaml:"-" json:"-"`
	Observer         Observer      `yaml:"-" json:"-"`
	Tracer           *pkgtracing.TracerProvider `yaml:"-" json:"-"`
}

func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Addr:             ":8080",
		Path:             "/ws",
		ReadTimeout:      30 * time.Second,
		WriteTimeout:     30 * time.Second,
		HeartbeatTimeout: 90 * time.Second,
		IdleTimeout:      2 * time.Minute,
	}
}

type Conn struct {
	conn      *nws.Conn
	config    ServerConfig
	identity  Identity
	observer  Observer
	tracer    *pkgtracing.TracerProvider
	traceCtx  context.Context
	wmu       sync.Mutex
	mu        sync.RWMutex
	lastSeen  time.Time
	closed    bool
	closeCode CloseCode
}

type Handler func(ctx context.Context, conn *Conn)

type Server struct {
	config   ServerConfig
	mux      *http.ServeMux
	handlers map[string]Handler
	server   *http.Server
	listener net.Listener
	mu       sync.RWMutex
}

func NewServer(config ServerConfig) *Server {
	if config.Addr == "" {
		config.Addr = ":8080"
	}
	if config.Path == "" {
		config.Path = "/ws"
	}
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
	mux := http.NewServeMux()
	s := &Server{
		config:   config,
		mux:      mux,
		handlers: make(map[string]Handler),
	}
	s.server = &http.Server{Addr: config.Addr, Handler: mux}
	return s
}

func (s *Server) Handle(path string, handler Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[path] = handler
	s.mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		s.serveHTTPConn(w, r, handler)
	})
}

func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		return nil
	}
	ln, err := net.Listen("tcp", s.config.Addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.config.Addr, err)
	}
	s.listener = ln
	go func() {
		_ = s.server.Serve(ln)
	}()
	return nil
}

func (s *Server) StartListener(listener net.Listener) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		return nil
	}
	s.listener = listener
	go func() {
		_ = s.server.Serve(listener)
	}()
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener == nil {
		return nil
	}
	err := s.server.Shutdown(ctx)
	s.listener = nil
	return err
}

func (s *Server) Addr() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return s.config.Addr
}

func (s *Server) serveHTTPConn(w http.ResponseWriter, req *http.Request, handler Handler) {
	traceCtx := extractTraceFromHTTPHeaders(req.Context(), req.Header)
	traceCtx, handshakeSpan := startWebsocketSpan(traceCtx, s.config.Tracer, "websocket.server.handshake",
		attribute.String("http.method", req.Method),
		attribute.String("websocket.path", req.URL.Path),
		attribute.String("websocket.remote_addr", req.RemoteAddr),
	)
	var (
		authResult *AuthResult
		err        error
	)
	if s.config.Authenticator != nil {
		authResult, err = s.config.Authenticator(req)
		if err != nil {
			s.config.Observer.OnAuthFailed(req.URL.Path, err)
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
	if s.config.RequireAuth && authResult == nil {
		authErr := authError(req.URL.Path, fmt.Errorf("authentication required"))
		s.config.Observer.OnAuthFailed(req.URL.Path, authErr)
		endSpan(handshakeSpan, authErr)
		_ = wsConn.Close(nws.StatusPolicyViolation, authErr.Error())
		return
	}
	conn := &Conn{
		conn:      wsConn,
		config:    s.config,
		identity:  newIdentity(req, authResult),
		observer:  s.config.Observer,
		tracer:    tracerOrGlobal(s.config.Tracer),
		traceCtx:  traceCtx,
		lastSeen:  time.Now(),
		closeCode: CloseCodeNormal,
	}
	conn.observer.OnConnect(conn.identity)
	endSpan(handshakeSpan, nil)
	ctx, cancel := context.WithCancel(traceCtx)
	defer cancel()
	if s.config.HeartbeatTimeout > 0 || s.config.IdleTimeout > 0 {
		go conn.monitorLiveness(ctx)
	}
	handler(ctx, conn)
	_ = conn.CloseWithCode(CloseCodeNormal, nil)
}

func (c *Conn) Receive(ctx context.Context) (int, []byte, error) {
	spanCtx, span := startWebsocketSpan(contextWithFallback(ctx, c.traceCtx), c.tracer, traceMessageName("receive", false),
		websocketTraceAttrs(c.identity)...,
	)
	readCtx, cancel := context.WithTimeout(ctx, c.config.ReadTimeout)
	defer cancel()
	msgType, payload, err := c.conn.Read(readCtx)
	if err != nil {
		endSpan(span, err)
		return 0, nil, err
	}
	c.markSeen()
	c.observer.OnMessage("in", c.identity, len(payload))
	span.SetAttributes(attribute.Int("websocket.message_size", len(payload)))
	endSpan(span, nil)
	c.traceCtx = spanCtx
	return int(msgType), payload, nil
}

func (c *Conn) ReceiveJSON(ctx context.Context, dest interface{}) error {
	spanCtx, span := startWebsocketSpan(contextWithFallback(ctx, c.traceCtx), c.tracer, traceMessageName("receive", true),
		websocketTraceAttrs(c.identity)...,
	)
	readCtx, cancel := context.WithTimeout(ctx, c.config.ReadTimeout)
	defer cancel()
	if err := wsjson.Read(readCtx, c.conn, dest); err != nil {
		endSpan(span, err)
		return err
	}
	c.markSeen()
	c.observer.OnMessage("in", c.identity, 1)
	endSpan(span, nil)
	c.traceCtx = spanCtx
	return nil
}

func (c *Conn) SendText(ctx context.Context, message string) error {
	return c.send(ctx, []byte(message), byte(nws.MessageText))
}

func (c *Conn) SendBinary(ctx context.Context, payload []byte) error {
	return c.send(ctx, payload, byte(nws.MessageBinary))
}

func (c *Conn) SendJSON(ctx context.Context, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return c.SendText(ctx, string(data))
}

func (c *Conn) Close() error {
	return c.CloseWithCode(CloseCodeNormal, nil)
}

func (c *Conn) CloseWithCode(code CloseCode, err error) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.closeCode = code
	c.mu.Unlock()
	c.observer.OnDisconnect(c.identity, classifyCloseCode(err, code), err)
	status := nws.StatusNormalClosure
	switch code {
	case CloseCodeAuthFailed:
		status = nws.StatusPolicyViolation
	case CloseCodeHeartbeatTimeout, CloseCodeIdleTimeout:
		status = nws.StatusGoingAway
	case CloseCodeWriteError, CloseCodeReadError, CloseCodeInternalError:
		status = nws.StatusInternalError
	}
	reason := ""
	if err != nil {
		reason = err.Error()
	}
	return c.conn.Close(status, reason)
}

func (c *Conn) Identity() Identity {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.identity
}

func (c *Conn) send(ctx context.Context, payload []byte, frameType byte) error {
	spanCtx, span := startWebsocketSpan(contextWithFallback(ctx, c.traceCtx), c.tracer, traceMessageName("send", false),
		websocketTraceAttrs(c.identity, attribute.Int("websocket.message_size", len(payload)))...,
	)
	c.wmu.Lock()
	defer c.wmu.Unlock()
	writeCtx, cancel := context.WithTimeout(ctx, c.config.WriteTimeout)
	defer cancel()
	msgType := nws.MessageText
	if frameType != byte(nws.MessageText) {
		msgType = nws.MessageBinary
	}
	if err := c.conn.Write(writeCtx, msgType, payload); err != nil {
		endSpan(span, err)
		return err
	}
	c.markSeen()
	c.observer.OnMessage("out", c.identity, len(payload))
	endSpan(span, nil)
	c.traceCtx = spanCtx
	return nil
}

func (c *Conn) SetGroups(groups ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.identity.Groups = append([]string(nil), groups...)
}

func (c *Conn) AddGroups(groups ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	exists := make(map[string]struct{}, len(c.identity.Groups))
	for _, group := range c.identity.Groups {
		exists[group] = struct{}{}
	}
	for _, group := range groups {
		if _, ok := exists[group]; ok {
			continue
		}
		c.identity.Groups = append(c.identity.Groups, group)
		exists[group] = struct{}{}
	}
}

func (c *Conn) markSeen() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastSeen = time.Now()
}

func (c *Conn) monitorLiveness(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.mu.RLock()
			lastSeen := c.lastSeen
			closed := c.closed
			identity := c.identity
			c.mu.RUnlock()
			if closed {
				return
			}
			now := time.Now()
			if c.config.HeartbeatTimeout > 0 && now.Sub(lastSeen) > c.config.HeartbeatTimeout {
				_, span := startWebsocketSpan(c.traceCtx, c.tracer, "websocket.server.heartbeat_timeout", websocketTraceAttrs(identity)...)
				c.observer.OnHeartbeatTimeout(identity)
				endSpan(span, context.DeadlineExceeded)
				_ = c.CloseWithCode(CloseCodeHeartbeatTimeout, context.DeadlineExceeded)
				return
			}
			if c.config.IdleTimeout > 0 && now.Sub(lastSeen) > c.config.IdleTimeout {
				_, span := startWebsocketSpan(c.traceCtx, c.tracer, "websocket.server.idle_timeout", websocketTraceAttrs(identity)...)
				endSpan(span, context.DeadlineExceeded)
				_ = c.CloseWithCode(CloseCodeIdleTimeout, context.DeadlineExceeded)
				return
			}
		}
	}
}
