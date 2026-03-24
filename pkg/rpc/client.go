// Package rpc provides RPC client implementations for gRPC and JSON-RPC
package rpc

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/discovery"
	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
	"github.com/alldev-run/golang-gin-rpc/pkg/ratelimiter"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// ClientType represents the type of RPC client
type ClientType string

const (
	ClientTypeGRPC    ClientType = "grpc"
	ClientTypeJSONRPC ClientType = "jsonrpc"
)

// ClientConfig holds client configuration
type ClientConfig struct {
	Type        ClientType    `yaml:"type" json:"type"`
	Host        string        `yaml:"host" json:"host"`
	Port        int           `yaml:"port" json:"port"`
	Timeout     time.Duration `yaml:"timeout" json:"timeout"`
	EnableTLS   bool          `yaml:"enable_tls" json:"enable_tls"`
	MaxMsgSize  int           `yaml:"max_msg_size" json:"max_msg_size"`
	ServiceName string        `yaml:"service_name" json:"service_name"`
	APIKey      string        `yaml:"api_key" json:"api_key"`
	AuthHeader  string        `yaml:"auth_header" json:"auth_header"`
	LoadBalance string        `yaml:"load_balance" json:"load_balance"`
	RetryCount  int           `yaml:"retry_count" json:"retry_count"`
	RetryBackoff time.Duration `yaml:"retry_backoff" json:"retry_backoff"`
	RetryJitter float64       `yaml:"retry_jitter" json:"retry_jitter"`
	IdempotentMethods []string `yaml:"idempotent_methods" json:"idempotent_methods"`
	MaxRetryElapsedTime time.Duration `yaml:"max_retry_elapsed_time" json:"max_retry_elapsed_time"`
	DiscoveryCacheTTL time.Duration `yaml:"discovery_cache_ttl" json:"discovery_cache_ttl"`
	FailoverThreshold int `yaml:"failover_threshold" json:"failover_threshold"`
	FailoverCooldown time.Duration `yaml:"failover_cooldown" json:"failover_cooldown"`
}

type discoveryServiceResolver interface {
	GetService(ctx context.Context, serviceName string) ([]*discovery.ServiceInstance, error)
}

type clientObserver interface {
	RecordRequest(clientType ClientType, method, target, status string, duration time.Duration)
	RecordRetry(clientType ClientType, method string, attempt int)
}

// DefaultClientConfig returns default client configuration
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		Type:       ClientTypeGRPC,
		Host:       "localhost",
		Port:       50051,
		Timeout:    30 * time.Second,
		MaxMsgSize: 4 * 1024 * 1024,
		AuthHeader: "X-API-Key",
		LoadBalance: "round_robin",
		RetryCount:  1,
		RetryBackoff: 100 * time.Millisecond,
		RetryJitter: 0.2,
		MaxRetryElapsedTime: 2 * time.Second,
		DiscoveryCacheTTL: 5 * time.Second,
		FailoverThreshold: 3,
		FailoverCooldown: 30 * time.Second,
	}
}

// Client represents the RPC client interface
type Client interface {
	Connect() error
	Close() error
	IsConnected() bool
	Type() ClientType
	Call(ctx context.Context, method string, params interface{}, result interface{}) error
}

// GRPCClient wraps gRPC client
type GRPCClient struct {
	config      ClientConfig
	conn        *grpc.ClientConn
	dialOptions []grpc.DialOption
	degradation *DegradationManager
	rateLimiter *ratelimiter.Manager
	discovery   discoveryServiceResolver
	targetSelector *clientTargetSelector
	observer    clientObserver
	cache       *clientEndpointCache
	failover    *clientTargetFailover
}

// JSONRPCClient wraps JSON-RPC client
type JSONRPCClient struct {
	config      ClientConfig
	httpClient  *http.Client
	baseURL     string
	degradation *DegradationManager
	rateLimiter *ratelimiter.Manager
	discovery   discoveryServiceResolver
	targetSelector *clientTargetSelector
	observer    clientObserver
	cache       *clientEndpointCache
	failover    *clientTargetFailover
}

type clientTargetSelector struct {
	strategy string
	counter  uint64
	rng      *rand.Rand
}

type clientEndpointCache struct {
	mu        sync.RWMutex
	targets   []string
	expiresAt time.Time
}

type clientTargetFailover struct {
	mu      sync.RWMutex
	targets map[string]*targetHealthState
}

type targetHealthState struct {
	failures     int
	ejectedUntil time.Time
}

// NewClient creates a new RPC client based on configuration
func NewClient(config ClientConfig) (Client, error) {
	switch config.Type {
	case ClientTypeGRPC:
		return NewGRPCClient(config), nil
	case ClientTypeJSONRPC:
		return NewJSONRPCClient(config), nil
	default:
		return nil, fmt.Errorf("unsupported client type: %s", config.Type)
	}
}

// NewGRPCClient creates a new gRPC client
func NewGRPCClient(config ClientConfig) *GRPCClient {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	return &GRPCClient{
		config:      config,
		rateLimiter: ratelimiter.NewManager(ratelimiter.DefaultConfig()),
		targetSelector: newClientTargetSelector(config.LoadBalance),
		cache:       &clientEndpointCache{},
		failover:    newClientTargetFailover(),
	}
}

// Connect establishes connection to gRPC server
func (c *GRPCClient) Connect() error {
	addr, err := c.resolveGRPCTarget(context.Background())
	if err != nil {
		return err
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	if c.config.EnableTLS {
		opts[0] = grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			ServerName: c.config.Host,
		}))
	}

	if c.config.MaxMsgSize > 0 {
		opts = append(opts,
			grpc.WithDefaultCallOptions(
				grpc.MaxCallRecvMsgSize(c.config.MaxMsgSize),
				grpc.MaxCallSendMsgSize(c.config.MaxMsgSize),
			),
		)
	}
	if c.config.APIKey != "" {
		opts = append(opts, grpc.WithUnaryInterceptor(c.authUnaryClientInterceptor()))
	}
	if len(c.dialOptions) > 0 {
		opts = append(opts, c.dialOptions...)
	}

	conn, err := grpc.Dial(addr, opts...)
	if err != nil {
		c.invalidateTargets()
		return fmt.Errorf("failed to connect to gRPC server: %w", err)
	}

	c.conn = conn
	logger.Info("RPC client connected",
		logger.String("type", string(ClientTypeGRPC)),
		logger.String("target", addr))
	return nil
}

// Close closes the gRPC connection
func (c *GRPCClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// IsConnected returns true if connected
func (c *GRPCClient) IsConnected() bool {
	return c.conn != nil
}

// Type returns the client type
func (c *GRPCClient) Type() ClientType {
	return ClientTypeGRPC
}

// Call makes a gRPC call
func (c *GRPCClient) Call(ctx context.Context, method string, params interface{}, result interface{}) error {
	if c.rateLimiter != nil && !c.rateLimiter.Allow(method) {
		return fmt.Errorf("rate limit exceeded for method: %s", method)
	}

	if c.degradation != nil && !c.degradation.ShouldAllowMethod(method) {
		return fmt.Errorf("method %s blocked by degradation policy", method)
	}

	return c.Invoke(ctx, method, params, result)
}

// Connection returns the underlying gRPC connection for use with generated stubs
func (c *GRPCClient) Connection() *grpc.ClientConn {
	return c.conn
}

func (c *GRPCClient) SetDialOptions(opts ...grpc.DialOption) {
	c.dialOptions = append([]grpc.DialOption(nil), opts...)
}

// SetDegradationManager sets the degradation manager for the client
func (c *GRPCClient) SetDegradationManager(dm *DegradationManager) {
	c.degradation = dm
}

// SetRateLimiterManager sets the rate limiter manager for the client
func (c *GRPCClient) SetRateLimiterManager(rlm *ratelimiter.Manager) {
	c.rateLimiter = rlm
}

func (c *GRPCClient) SetDiscoveryResolver(resolver discoveryServiceResolver) {
	c.discovery = resolver
}

func (c *GRPCClient) SetObserver(observer clientObserver) {
	c.observer = observer
}

func (c *GRPCClient) Invoke(ctx context.Context, method string, req, reply interface{}) error {
	start := time.Now()
	status := "ok"
	target := ""
	defer func() {
		if c.observer != nil {
			c.observer.RecordRequest(ClientTypeGRPC, method, target, status, time.Since(start))
		}
	}()

	attempts := c.config.RetryCount
	if attempts <= 0 {
		attempts = 1
	}
	backoff := c.config.RetryBackoff
	if backoff <= 0 {
		backoff = 100 * time.Millisecond
	}
	maxElapsed := c.config.MaxRetryElapsedTime

	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if maxElapsed > 0 && time.Since(start) >= maxElapsed {
			status = "retry_budget_exhausted"
			break
		}
		lastErr = nil
		if !c.IsConnected() {
			if err := c.Connect(); err != nil {
				lastErr = err
				status = "connect_error"
			} else if c.conn != nil {
				target = c.conn.Target()
			}
		}
		if lastErr == nil && c.conn != nil {
			if err := c.conn.Invoke(ctx, method, req, reply); err == nil {
				c.recordTargetSuccess(target)
				return nil
			} else {
				lastErr = err
				status = "invoke_error"
				c.recordTargetFailure(target)
			}
		}

		if c.observer != nil {
			c.observer.RecordRetry(ClientTypeGRPC, method, attempt+1)
		}
		if !c.shouldRetryGRPC(method, lastErr) {
			break
		}
		logger.Warn("RPC client retrying grpc invoke",
			logger.String("method", method),
			logger.Int("attempt", attempt+1),
			logger.Error(lastErr))
		if c.conn != nil {
			_ = c.conn.Close()
			c.conn = nil
		}
		c.invalidateTargets()
		if attempt < attempts-1 {
			time.Sleep(c.retryBackoff(attempt, backoff))
		}
	}
	return lastErr
}

// NewJSONRPCClient creates a new JSON-RPC client
func NewJSONRPCClient(config ClientConfig) *JSONRPCClient {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	baseURL := fmt.Sprintf("http://%s:%d", config.Host, config.Port)
	if config.EnableTLS {
		baseURL = fmt.Sprintf("https://%s:%d", config.Host, config.Port)
	}

	return &JSONRPCClient{
		config:      config,
		baseURL:     baseURL,
		rateLimiter: ratelimiter.NewManager(ratelimiter.DefaultConfig()),
		targetSelector: newClientTargetSelector(config.LoadBalance),
		cache:       &clientEndpointCache{},
		failover:    newClientTargetFailover(),
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// Connect tests connection to JSON-RPC server (no-op for HTTP)
func (c *JSONRPCClient) Connect() error {
	ctx, cancel := context.WithTimeout(context.Background(), c.config.Timeout)
	defer cancel()

	if err := c.refreshJSONRPCBaseURL(ctx); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/rpc?ping=true", nil)
	if err != nil {
		return fmt.Errorf("failed to create ping request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.invalidateTargets()
		return fmt.Errorf("failed to connect to JSON-RPC server: %w", err)
	}
	defer resp.Body.Close()

	return nil
}

// Close closes the client (no-op for HTTP)
func (c *JSONRPCClient) Close() error {
	return nil
}

// IsConnected always returns true for HTTP client
func (c *JSONRPCClient) IsConnected() bool {
	return true
}

// Type returns the client type
func (c *JSONRPCClient) Type() ClientType {
	return ClientTypeJSONRPC
}

// SetDegradationManager sets the degradation manager for the client
func (c *JSONRPCClient) SetDegradationManager(dm *DegradationManager) {
	c.degradation = dm
}

// SetRateLimiterManager sets the rate limiter manager for the client
func (c *JSONRPCClient) SetRateLimiterManager(rlm *ratelimiter.Manager) {
	c.rateLimiter = rlm
}

func (c *JSONRPCClient) SetDiscoveryResolver(resolver discoveryServiceResolver) {
	c.discovery = resolver
}

func (c *JSONRPCClient) SetObserver(observer clientObserver) {
	c.observer = observer
}

// Call makes a JSON-RPC call
func (c *JSONRPCClient) Call(ctx context.Context, method string, params interface{}, result interface{}) error {
	if c.rateLimiter != nil && !c.rateLimiter.Allow(method) {
		return fmt.Errorf("rate limit exceeded for method: %s", method)
	}

	if c.degradation != nil && !c.degradation.ShouldAllowMethod(method) {
		if fallback, ok := c.degradation.GetFallback(method); ok {
			fallbackResult, err := fallback(ctx, method, params)
			if err != nil {
				return err
			}
			if result != nil && fallbackResult != nil {
				resultBytes, err := json.Marshal(fallbackResult)
				if err != nil {
					return fmt.Errorf("failed to marshal fallback result: %w", err)
				}
				if err := json.Unmarshal(resultBytes, result); err != nil {
					return fmt.Errorf("failed to unmarshal fallback result: %w", err)
				}
			}
			return nil
		}
		return fmt.Errorf("method %s blocked by degradation policy", method)
	}

	start := time.Now()
	status := "ok"
	defer func() {
		if c.degradation != nil {
			c.degradation.RecordMetrics(time.Since(start), nil)
		}
		if c.observer != nil {
			c.observer.RecordRequest(ClientTypeJSONRPC, method, c.baseURL, status, time.Since(start))
		}
	}()

	if err := c.refreshJSONRPCBaseURL(ctx); err != nil {
		return err
	}

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/rpc", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	if c.config.APIKey != "" {
		headerName := c.config.AuthHeader
		if headerName == "" {
			headerName = "X-API-Key"
		}
		httpReq.Header.Set(headerName, c.config.APIKey)
	}

	resp, err := c.doJSONRPCRequestWithRetry(httpReq)
	if err != nil {
		status = "error"
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	var jsonResp JSONRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if jsonResp.Error != nil {
		status = "rpc_error"
		return fmt.Errorf("JSON-RPC error [%d]: %s", jsonResp.Error.Code, jsonResp.Error.Message)
	}

	if result != nil && jsonResp.Result != nil {
		resultBytes, err := json.Marshal(jsonResp.Result)
		if err != nil {
			return fmt.Errorf("failed to marshal result: %w", err)
		}
		if err := json.Unmarshal(resultBytes, result); err != nil {
			return fmt.Errorf("failed to unmarshal result: %w", err)
		}
	}

	return nil
}

// SetAPIKey sets the API key for authentication
func (c *JSONRPCClient) SetAPIKey(apiKey string) {
	c.httpClient.Transport = &apiKeyTransport{
		Base:   c.httpClient.Transport,
		APIKey: apiKey,
	}
}

// apiKeyTransport adds API key to requests
type apiKeyTransport struct {
	Base   http.RoundTripper
	APIKey string
}

func (t *apiKeyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("X-API-Key", t.APIKey)
	if t.Base == nil {
		t.Base = http.DefaultTransport
	}
	return t.Base.RoundTrip(req)
}

func (c *GRPCClient) resolveGRPCTarget(ctx context.Context) (string, error) {
	if target, ok := c.getCachedTarget(); ok {
		return target, nil
	}
	if c.discovery != nil && c.config.ServiceName != "" {
		instances, err := c.discovery.GetService(ctx, c.config.ServiceName)
		if err != nil {
			return "", fmt.Errorf("failed to resolve grpc service %s: %w", c.config.ServiceName, err)
		}
		if len(instances) == 0 {
			return "", fmt.Errorf("no instances found for grpc service: %s", c.config.ServiceName)
		}
		targets := make([]string, 0, len(instances))
		for _, instance := range instances {
			targets = append(targets, fmt.Sprintf("%s:%d", instance.Address, instance.Port))
		}
		c.storeTargets(targets)
		selected, err := c.selectHealthyTarget(targets)
		if err == nil {
			logger.Debug("RPC client resolved discovery target",
				logger.String("type", string(ClientTypeGRPC)),
				logger.String("service", c.config.ServiceName),
				logger.String("target", selected))
		}
		return selected, err
	}
	return fmt.Sprintf("%s:%d", c.config.Host, c.config.Port), nil
}

func (c *JSONRPCClient) refreshJSONRPCBaseURL(ctx context.Context) error {
	if target, ok := c.getCachedTarget(); ok {
		c.baseURL = target
		return nil
	}
	if c.discovery == nil || c.config.ServiceName == "" {
		return nil
	}

	instances, err := c.discovery.GetService(ctx, c.config.ServiceName)
	if err != nil {
		return fmt.Errorf("failed to resolve jsonrpc service %s: %w", c.config.ServiceName, err)
	}
	if len(instances) == 0 {
		return fmt.Errorf("no instances found for jsonrpc service: %s", c.config.ServiceName)
	}

	scheme := "http"
	if c.config.EnableTLS {
		scheme = "https"
	}
	targets := make([]string, 0, len(instances))
	for _, instance := range instances {
		targets = append(targets, fmt.Sprintf("%s://%s:%d", scheme, instance.Address, instance.Port))
	}
	c.storeTargets(targets)
	selected, err := c.selectHealthyTarget(targets)
	if err != nil {
		return err
	}
	c.baseURL = selected
	logger.Debug("RPC client resolved discovery target",
		logger.String("type", string(ClientTypeJSONRPC)),
		logger.String("service", c.config.ServiceName),
		logger.String("target", selected))
	return nil
}

func newClientTargetSelector(strategy string) *clientTargetSelector {
	if strategy == "" {
		strategy = "round_robin"
	}
	return &clientTargetSelector{
		strategy: strategy,
		rng:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (s *clientTargetSelector) Select(targets []string) (string, error) {
	if len(targets) == 0 {
		return "", fmt.Errorf("no targets available")
	}

	switch s.strategy {
	case "random":
		return targets[s.rng.Intn(len(targets))], nil
	case "round_robin":
		fallthrough
	default:
		index := atomic.AddUint64(&s.counter, 1) - 1
		return targets[index%uint64(len(targets))], nil
	}
}

func (c *GRPCClient) authUnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		headerName := c.config.AuthHeader
		if headerName == "" {
			headerName = "x-api-key"
		}
		ctx = metadata.AppendToOutgoingContext(ctx, headerName, c.config.APIKey)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

func (c *JSONRPCClient) doJSONRPCRequestWithRetry(req *http.Request) (*http.Response, error) {
	attempts := c.config.RetryCount
	if attempts <= 0 {
		attempts = 1
	}
	backoff := c.config.RetryBackoff
	if backoff <= 0 {
		backoff = 100 * time.Millisecond
	}

	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		cloned := req.Clone(req.Context())
		if req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, err
			}
			cloned.Body = body
		}
		resp, err := c.httpClient.Do(cloned)
		if err == nil && resp.StatusCode < http.StatusInternalServerError {
			c.recordTargetSuccess(req.URL.Host)
			return resp, nil
		}
		if err == nil {
			lastErr = fmt.Errorf("server returned status %d", resp.StatusCode)
			_ = resp.Body.Close()
		} else {
			lastErr = err
		}
		c.recordTargetFailure(req.URL.Host)
		c.invalidateTargets()
		if c.observer != nil {
			c.observer.RecordRetry(ClientTypeJSONRPC, req.URL.Path, attempt+1)
		}
		logger.Warn("RPC client retrying request",
			logger.String("type", string(ClientTypeJSONRPC)),
			logger.String("url", req.URL.String()),
			logger.Int("attempt", attempt+1),
			logger.Error(lastErr))
		if attempt < attempts-1 {
			time.Sleep(backoff * time.Duration(attempt+1))
		}
	}
	return nil, lastErr
}

func (c *GRPCClient) getCachedTarget() (string, bool) {
	if c.cache == nil {
		return "", false
	}
	return c.cache.get(c.targetSelector)
}

func (c *JSONRPCClient) getCachedTarget() (string, bool) {
	if c.cache == nil {
		return "", false
	}
	return c.cache.get(c.targetSelector)
}

func (c *GRPCClient) storeTargets(targets []string) {
	if c.cache == nil {
		return
	}
	c.cache.store(targets, c.config.DiscoveryCacheTTL)
}

func (c *JSONRPCClient) storeTargets(targets []string) {
	if c.cache == nil {
		return
	}
	c.cache.store(targets, c.config.DiscoveryCacheTTL)
}

func (c *GRPCClient) invalidateTargets() {
	if c.cache != nil {
		c.cache.invalidate()
	}
}

func (c *JSONRPCClient) invalidateTargets() {
	if c.cache != nil {
		c.cache.invalidate()
	}
}

func (c *clientEndpointCache) get(selector *clientTargetSelector) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.targets) == 0 || time.Now().After(c.expiresAt) {
		return "", false
	}
	target, err := selector.Select(c.targets)
	if err != nil {
		return "", false
	}
	return target, true
}

func (c *clientEndpointCache) store(targets []string, ttl time.Duration) {
	if ttl <= 0 {
		ttl = 5 * time.Second
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.targets = append(c.targets[:0], targets...)
	c.expiresAt = time.Now().Add(ttl)
}

func (c *clientEndpointCache) invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.targets = nil
	c.expiresAt = time.Time{}
}

func (c *GRPCClient) shouldRetryGRPC(method string, err error) bool {
	if err == nil {
		return false
	}
	if !c.isIdempotentMethod(method) {
		return false
	}
	st, ok := status.FromError(err)
	if !ok {
		return true
	}
	switch st.Code() {
	case codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted, codes.Aborted:
		return true
	default:
		return false
	}
}

func (c *GRPCClient) retryBackoff(attempt int, base time.Duration) time.Duration {
	if base <= 0 {
		base = 100 * time.Millisecond
	}
	backoff := base << attempt
	maxBackoff := 2 * time.Second
	if backoff > maxBackoff {
		backoff = maxBackoff
	}
	if c.config.RetryJitter > 0 {
		factor := 1 + ((rand.Float64()*2 - 1) * c.config.RetryJitter)
		if factor < 0 {
			factor = 0
		}
		backoff = time.Duration(float64(backoff) * factor)
	}
	return backoff
}

func (c *GRPCClient) isIdempotentMethod(method string) bool {
	if len(c.config.IdempotentMethods) == 0 {
		return true
	}
	for _, candidate := range c.config.IdempotentMethods {
		if candidate == method {
			return true
		}
	}
	return false
}

func (c *GRPCClient) selectHealthyTarget(targets []string) (string, error) {
	if c.failover == nil {
		return c.targetSelector.Select(targets)
	}
	filtered := c.failover.filterAvailable(targets)
	if len(filtered) == 0 {
		filtered = targets
	}
	return c.targetSelector.Select(filtered)
}

func (c *JSONRPCClient) selectHealthyTarget(targets []string) (string, error) {
	if c.failover == nil {
		return c.targetSelector.Select(targets)
	}
	filtered := c.failover.filterAvailable(targets)
	if len(filtered) == 0 {
		filtered = targets
	}
	return c.targetSelector.Select(filtered)
}

func newClientTargetFailover() *clientTargetFailover {
	return &clientTargetFailover{targets: make(map[string]*targetHealthState)}
}

func (f *clientTargetFailover) filterAvailable(targets []string) []string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	now := time.Now()
	available := make([]string, 0, len(targets))
	for _, target := range targets {
		state, ok := f.targets[target]
		if !ok || state.ejectedUntil.IsZero() || !now.Before(state.ejectedUntil) {
			available = append(available, target)
		}
	}
	return available
}

func (f *clientTargetFailover) recordSuccess(target string) {
	if target == "" {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.targets, target)
}

func (f *clientTargetFailover) recordFailure(target string, threshold int, cooldown time.Duration) {
	if target == "" {
		return
	}
	if threshold <= 0 {
		threshold = 3
	}
	if cooldown <= 0 {
		cooldown = 30 * time.Second
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	state, ok := f.targets[target]
	if !ok {
		state = &targetHealthState{}
		f.targets[target] = state
	}
	state.failures++
	if state.failures >= threshold {
		state.ejectedUntil = time.Now().Add(cooldown)
		state.failures = 0
	}
}

func (c *GRPCClient) recordTargetSuccess(target string) {
	if c.failover != nil {
		c.failover.recordSuccess(target)
	}
}

func (c *JSONRPCClient) recordTargetSuccess(target string) {
	if c.failover != nil {
		c.failover.recordSuccess(target)
	}
}

func (c *GRPCClient) recordTargetFailure(target string) {
	if c.failover != nil {
		c.failover.recordFailure(target, c.config.FailoverThreshold, c.config.FailoverCooldown)
	}
}

func (c *JSONRPCClient) recordTargetFailure(target string) {
	if c.failover != nil {
		c.failover.recordFailure(target, c.config.FailoverThreshold, c.config.FailoverCooldown)
	}
}
