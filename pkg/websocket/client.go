package websocket

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"sync"
	"time"

	pkgtracing "alldev-gin-rpc/pkg/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	nws "nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

type Config struct {
	URL                  string            `yaml:"url" json:"url"`
	Origin               string            `yaml:"origin" json:"origin"`
	Headers              map[string]string `yaml:"headers" json:"headers"`
	HandshakeTimeout     time.Duration     `yaml:"handshake_timeout" json:"handshake_timeout"`
	ReadTimeout          time.Duration     `yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout         time.Duration     `yaml:"write_timeout" json:"write_timeout"`
	AutoReconnect        bool              `yaml:"auto_reconnect" json:"auto_reconnect"`
	ReconnectInterval    time.Duration     `yaml:"reconnect_interval" json:"reconnect_interval"`
	MaxReconnectAttempts int               `yaml:"max_reconnect_attempts" json:"max_reconnect_attempts"`
	MaxReconnectInterval time.Duration     `yaml:"max_reconnect_interval" json:"max_reconnect_interval"`
	ReconnectMultiplier  float64           `yaml:"reconnect_multiplier" json:"reconnect_multiplier"`
	ReconnectJitter      float64           `yaml:"reconnect_jitter" json:"reconnect_jitter"`
	HeartbeatInterval    time.Duration     `yaml:"heartbeat_interval" json:"heartbeat_interval"`
	HeartbeatMessage     string            `yaml:"heartbeat_message" json:"heartbeat_message"`
	Observer             Observer          `yaml:"-" json:"-"`
	Tracer               *pkgtracing.TracerProvider `yaml:"-" json:"-"`
}

func DefaultConfig() Config {
	return Config{
		URL:                  "ws://localhost:8080/ws",
		Origin:               "http://localhost/",
		Headers:              make(map[string]string),
		HandshakeTimeout:     10 * time.Second,
		ReadTimeout:          30 * time.Second,
		WriteTimeout:         30 * time.Second,
		ReconnectInterval:    3 * time.Second,
		MaxReconnectInterval: 30 * time.Second,
		ReconnectMultiplier:  2.0,
		ReconnectJitter:      0.2,
		HeartbeatInterval:    30 * time.Second,
		HeartbeatMessage:     "ping",
	}
}

type Client struct {
	config           Config
	conn             *nws.Conn
	mu               sync.RWMutex
	wmu              sync.Mutex
	bgMu             sync.Mutex
	bgCtx            context.Context
	bgCancel         context.CancelFunc
	heartbeatStarted bool
	reconnectStarted bool
	observer         Observer
	rng              *rand.Rand
	tracer           *pkgtracing.TracerProvider
}

func NewClient(config Config) *Client {
	if config.Origin == "" {
		config.Origin = "http://localhost/"
	}
	if config.Headers == nil {
		config.Headers = make(map[string]string)
	}
	if config.HandshakeTimeout <= 0 {
		config.HandshakeTimeout = 10 * time.Second
	}
	if config.ReadTimeout <= 0 {
		config.ReadTimeout = 30 * time.Second
	}
	if config.WriteTimeout <= 0 {
		config.WriteTimeout = 30 * time.Second
	}
	if config.ReconnectInterval <= 0 {
		config.ReconnectInterval = 3 * time.Second
	}
	if config.MaxReconnectInterval <= 0 {
		config.MaxReconnectInterval = 30 * time.Second
	}
	if config.ReconnectMultiplier <= 1 {
		config.ReconnectMultiplier = 2.0
	}
	if config.ReconnectJitter < 0 {
		config.ReconnectJitter = 0
	}
	if config.HeartbeatInterval <= 0 {
		config.HeartbeatInterval = 30 * time.Second
	}
	if config.HeartbeatMessage == "" {
		config.HeartbeatMessage = "ping"
	}
	return &Client{
		config:   config,
		observer: observerOrNoop(config.Observer),
		rng:      rand.New(rand.NewSource(time.Now().UnixNano())),
		tracer:   tracerOrGlobal(config.Tracer),
	}
}

func (c *Client) Connect(ctx context.Context) error {
	headers := http.Header{}
	for k, v := range c.config.Headers {
		headers.Set(k, v)
	}
	injectTraceToHTTPHeaders(ctx, headers)
	traceCtx, span := startWebsocketSpan(ctx, c.tracer, "websocket.client.connect",
		attribute.String("websocket.url", c.config.URL),
		attribute.String("websocket.origin", c.config.Origin),
	)
	dialCtx, cancel := context.WithTimeout(ctx, c.config.HandshakeTimeout)
	defer cancel()
	conn, _, err := nws.Dial(dialCtx, c.config.URL, &nws.DialOptions{
		HTTPHeader: headers,
	})
	if err != nil {
		endSpan(span, err)
		return fmt.Errorf("failed to connect websocket: %w", err)
	}
	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()
	identity := Identity{
		ConnectionID: trace.SpanFromContext(traceCtx).SpanContext().SpanID().String(),
		ClientID:     c.config.Headers["X-Client-ID"],
		Path:         c.config.URL,
	}
	c.observer.OnConnect(identity)
	endSpan(span, nil)
	c.startBackgroundLoops()
	return nil
}

func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn != nil
}

func (c *Client) Close() error {
	c.mu.Lock()
	conn := c.conn
	c.conn = nil
	c.mu.Unlock()
	c.bgMu.Lock()
	if c.bgCancel != nil {
		c.bgCancel()
		c.bgCancel = nil
	}
	c.heartbeatStarted = false
	c.reconnectStarted = false
	c.bgCtx = nil
	c.bgMu.Unlock()
	if conn == nil {
		return nil
	}
	err := conn.Close(nws.StatusNormalClosure, "")
	c.observer.OnDisconnect(Identity{Path: c.config.URL}, CloseCodeNormal, err)
	return err
}

func (c *Client) SendText(ctx context.Context, message string) error {
	return c.send(ctx, []byte(message), nws.MessageText)
}

func (c *Client) SendBinary(ctx context.Context, payload []byte) error {
	return c.send(ctx, payload, nws.MessageBinary)
}

func (c *Client) SendJSON(ctx context.Context, payload interface{}) error {
	conn, err := c.getConn()
	if err != nil {
		return err
	}
	traceCtx, span := startWebsocketSpan(ctx, c.tracer, traceMessageName("send", true),
		attribute.String("websocket.url", c.config.URL),
	)
	writeCtx, cancel := context.WithTimeout(ctx, c.config.WriteTimeout)
	defer cancel()
	if err := wsjson.Write(writeCtx, conn, payload); err != nil {
		c.handleConnectionError(err)
		endSpan(span, err)
		return err
	}
	c.observer.OnMessage("out", Identity{Path: c.config.URL}, 1)
	_ = traceCtx
	endSpan(span, nil)
	return nil
}

func (c *Client) Receive(ctx context.Context) (int, []byte, error) {
	conn, err := c.getConn()
	if err != nil {
		return 0, nil, err
	}
	_, span := startWebsocketSpan(ctx, c.tracer, traceMessageName("receive", false),
		attribute.String("websocket.url", c.config.URL),
	)
	readCtx, cancel := context.WithTimeout(ctx, c.config.ReadTimeout)
	defer cancel()
	msgType, data, err := conn.Read(readCtx)
	if err != nil {
		c.handleConnectionError(err)
		endSpan(span, err)
		return 0, nil, err
	}
	c.observer.OnMessage("in", Identity{Path: c.config.URL}, len(data))
	span.SetAttributes(attribute.Int("websocket.message_size", len(data)))
	endSpan(span, nil)
	return int(msgType), data, nil
}

func (c *Client) ReceiveJSON(ctx context.Context, dest interface{}) error {
	conn, err := c.getConn()
	if err != nil {
		return err
	}
	_, span := startWebsocketSpan(ctx, c.tracer, traceMessageName("receive", true),
		attribute.String("websocket.url", c.config.URL),
	)
	readCtx, cancel := context.WithTimeout(ctx, c.config.ReadTimeout)
	defer cancel()
	if err := wsjson.Read(readCtx, conn, dest); err != nil {
		c.handleConnectionError(err)
		endSpan(span, err)
		return err
	}
	c.observer.OnMessage("in", Identity{Path: c.config.URL}, 1)
	endSpan(span, nil)
	return nil
}

func (c *Client) Ping(ctx context.Context) error {
	conn, err := c.getConn()
	if err != nil {
		return err
	}
	_, span := startWebsocketSpan(ctx, c.tracer, "websocket.client.ping",
		attribute.String("websocket.url", c.config.URL),
	)
	pingCtx, cancel := context.WithTimeout(ctx, c.config.WriteTimeout)
	defer cancel()
	if err := conn.Ping(pingCtx); err != nil {
		c.handleConnectionError(err)
		endSpan(span, err)
		return err
	}
	endSpan(span, nil)
	return nil
}

func (c *Client) send(ctx context.Context, payload []byte, msgType nws.MessageType) error {
	conn, err := c.getConn()
	if err != nil {
		return err
	}
	_, span := startWebsocketSpan(ctx, c.tracer, traceMessageName("send", false),
		attribute.String("websocket.url", c.config.URL),
		attribute.Int("websocket.message_size", len(payload)),
	)
	c.wmu.Lock()
	defer c.wmu.Unlock()
	writeCtx, cancel := context.WithTimeout(ctx, c.config.WriteTimeout)
	defer cancel()
	if err := conn.Write(writeCtx, msgType, payload); err != nil {
		c.handleConnectionError(err)
		endSpan(span, err)
		return err
	}
	c.observer.OnMessage("out", Identity{Path: c.config.URL}, len(payload))
	endSpan(span, nil)
	return nil
}

func (c *Client) getConn() (*nws.Conn, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.conn == nil {
		return nil, fmt.Errorf("websocket client is not connected")
	}
	return c.conn, nil
}

func (c *Client) startBackgroundLoops() {
	c.bgMu.Lock()
	defer c.bgMu.Unlock()
	if c.bgCtx == nil {
		c.bgCtx, c.bgCancel = context.WithCancel(context.Background())
	}
	if c.config.HeartbeatInterval > 0 && !c.heartbeatStarted {
		c.heartbeatStarted = true
		go c.heartbeatLoop(c.bgCtx)
	}
	if c.config.AutoReconnect && !c.reconnectStarted {
		c.reconnectStarted = true
		go c.reconnectLoop(c.bgCtx)
	}
}

func (c *Client) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(c.config.HeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !c.IsConnected() {
				continue
			}
			if c.config.HeartbeatMessage != "" {
				_, span := startWebsocketSpan(ctx, c.tracer, "websocket.client.heartbeat",
					attribute.String("websocket.url", c.config.URL),
				)
				sendCtx, cancel := context.WithTimeout(ctx, c.config.WriteTimeout)
				err := c.SendText(sendCtx, c.config.HeartbeatMessage)
				cancel()
				endSpan(span, err)
				continue
			}
			pingCtx, cancel := context.WithTimeout(ctx, c.config.WriteTimeout)
			err := c.Ping(pingCtx)
			cancel()
			if _, span := startWebsocketSpan(ctx, c.tracer, "websocket.client.heartbeat.ping",
				attribute.String("websocket.url", c.config.URL),
			); span != nil {
				endSpan(span, err)
			}
		}
	}
}

func (c *Client) reconnectLoop(ctx context.Context) {
	attempts := 0
	for {
		delay := c.nextReconnectDelay(attempts)
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
			if c.IsConnected() {
				attempts = 0
				continue
			}
			if c.config.MaxReconnectAttempts > 0 && attempts >= c.config.MaxReconnectAttempts {
				c.observer.OnDisconnect(Identity{Path: c.config.URL}, CloseCodeReconnectExhausted, nil)
				return
			}
			attempts++
			traceCtx, span := startWebsocketSpan(ctx, c.tracer, "websocket.client.reconnect",
				attribute.String("websocket.url", c.config.URL),
				attribute.Int("websocket.reconnect_attempt", attempts),
				attribute.Int64("websocket.reconnect_delay_ms", delay.Milliseconds()),
			)
			c.observer.OnReconnectAttempt(c.config.URL, attempts, delay)
			reconnectCtx, cancel := context.WithTimeout(ctx, c.config.HandshakeTimeout)
			err := c.Connect(reconnectCtx)
			cancel()
			_ = traceCtx
			endSpan(span, err)
		}
	}
}

func (c *Client) nextReconnectDelay(attempt int) time.Duration {
	base := float64(c.config.ReconnectInterval)
	maxDelay := float64(c.config.MaxReconnectInterval)
	if attempt <= 0 {
		return c.config.ReconnectInterval
	}
	delay := base * math.Pow(c.config.ReconnectMultiplier, float64(attempt))
	if delay > maxDelay {
		delay = maxDelay
	}
	if c.config.ReconnectJitter > 0 {
		jitter := 1 + ((c.rng.Float64()*2 - 1) * c.config.ReconnectJitter)
		delay *= jitter
	}
	if delay < float64(time.Millisecond) {
		delay = float64(time.Millisecond)
	}
	return time.Duration(delay)
}

func (c *Client) handleConnectionError(err error) {
	if err == nil {
		return
	}
	c.mu.Lock()
	conn := c.conn
	c.conn = nil
	c.mu.Unlock()
	if conn != nil {
		_ = conn.Close(nws.StatusInternalError, err.Error())
	}
}
