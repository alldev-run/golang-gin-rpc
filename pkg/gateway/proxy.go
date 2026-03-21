package gateway

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"alldev-gin-rpc/pkg/logger"
	"alldev-gin-rpc/pkg/metrics"
)

// Proxy handles HTTP proxying
type Proxy struct {
	gateway *Gateway
	client  *http.Client
}

// NewProxy creates a new proxy instance
func NewProxy(gateway *Gateway) *Proxy {
	return &Proxy{
		gateway: gateway,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Close closes the HTTP proxy client
func (p *Proxy) Close() error {
	if p.client != nil {
		p.client.CloseIdleConnections()
		logger.Info("HTTP proxy client closed")
	}
	return nil
}

// SetupRoutes sets up the HTTP routes
func (g *Gateway) SetupRoutes(engine *gin.Engine) {
	g.proxy = NewProxy(g)

	// Add tracing middleware first (to trace all requests)
	engine.Use(g.TracingMiddleware())

	// Add CORS middleware
	if g.config.CORS.AllowedOrigins != nil {
		engine.Use(corsMiddleware(g.config.CORS))
	}

	// Add rate limiting middleware
	if g.config.RateLimit.Enabled {
		engine.Use(rateLimitMiddleware(g.config.RateLimit))
	}

	// Add request ID middleware
	engine.Use(requestIDMiddleware())

	// Add logging middleware
	engine.Use(loggingMiddleware())

	engine.Use(metricsMiddleware())

	// Setup proxy routes
	for _, route := range g.config.Routes {
		route.Path = normalizeGinRoutePath(route.Path)
		
		// Create handler based on protocol
		var handler gin.HandlerFunc
		switch route.Protocol {
		case "grpc":
			handler = func(c *gin.Context) {
				c.Set("route", route.Service)
				routeObj := &Route{
					config:   route,
					targets:  route.Targets,
					timeout:  route.Timeout,
					retries:  route.Retries,
				}
				if g.grpcProxy != nil {
					if err := g.grpcProxy.ProxyGRPC(c, routeObj); err != nil {
						logger.Errorf("gRPC proxy error", logger.Error(err))
					}
				} else {
					c.JSON(http.StatusNotImplemented, gin.H{"error": "gRPC proxy not initialized"})
				}
			}
		case "jsonrpc":
			handler = func(c *gin.Context) {
				c.Set("route", route.Service)
				routeObj := &Route{
					config:   route,
					targets:  route.Targets,
					timeout:  route.Timeout,
					retries:  route.Retries,
				}
				if g.jsonProxy != nil {
					if err := g.jsonProxy.ProxyJSONRPC(c, routeObj); err != nil {
						logger.Errorf("JSON-RPC proxy error", logger.Error(err))
					}
				} else {
					c.JSON(http.StatusNotImplemented, gin.H{"error": "JSON-RPC proxy not initialized"})
				}
			}
		default: // HTTP
			handler = g.proxy.handleRoute(route)
		}

		// Register route based on method
		if route.Method == "*" || route.Method == "ANY" {
			engine.Any(route.Path, handler)
		} else {
			engine.Handle(route.Method, route.Path, handler)
		}
	}

	// Health check endpoint
	engine.GET("/health", g.healthCheck)
	engine.GET("/ready", g.readinessCheck)

	// Gateway info endpoint
	engine.GET("/info", g.gatewayInfo)
	engine.GET("/metrics", gin.WrapH(metrics.Handler()))
}

// handleRoute handles a specific route
func (p *Proxy) handleRoute(routeConfig RouteConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		
		// Find route targets
		routePath := normalizeGinRoutePath(routeConfig.Path)
		routeKey := p.gateway.routeKey(routePath, c.Request.Method)
		p.gateway.router.mu.RLock()
		route, exists := p.gateway.router.routes[routeKey]
		if !exists {
			wildcardKey := p.gateway.routeKey(routePath, "*")
			route, exists = p.gateway.router.routes[wildcardKey]
		}
		p.gateway.router.mu.RUnlock()

		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "Route not found"})
			return
		}

		// Select target using load balancer
		selectTargets := route.targets
		if len(route.healthyTargets) > 0 {
			selectTargets = route.healthyTargets
		}
		target, err := p.gateway.balancer.Select(selectTargets)
		if err != nil {
			observeUpstreamError(route.config.Service, "select")
			logger.Errorf("Failed to select target", logger.Error(err))
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service unavailable"})
			return
		}

		// Proxy request
		if err := p.proxyRequest(ctx, c, target, route); err != nil {
			observeUpstreamError(route.config.Service, "proxy")
			logger.Errorf("Proxy request failed", logger.Error(err))
			p.handleProxyError(c, err)
			return
		}
	}
}

// proxyRequest proxies the request to the target service
func (p *Proxy) proxyRequest(ctx context.Context, c *gin.Context, target string, route *Route) error {
	// Create target URL
	targetURL, err := url.Parse(target)
	if err != nil {
		return fmt.Errorf("invalid target URL: %w", err)
	}

	// Build proxy URL
	proxyPath := c.Request.URL.Path
	if route.config.StripPrefix {
		proxyPath = strings.TrimPrefix(proxyPath, route.config.Path)
	}
	
	targetURL.Path = path.Join(targetURL.Path, proxyPath)
	targetURL.RawQuery = c.Request.URL.RawQuery

	// Create proxy request
	reqBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return fmt.Errorf("failed to read request body: %w", err)
	}

	proxyReq, err := http.NewRequestWithContext(ctx, c.Request.Method, targetURL.String(), bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create proxy request: %w", err)
	}

	// Copy headers
	p.copyHeaders(c.Request.Header, proxyReq.Header)
	
	// Add route-specific headers
	for key, value := range route.config.Headers {
		proxyReq.Header.Set(key, value)
	}

	// Add route-specific query parameters
	if len(route.config.Query) > 0 {
		query := proxyReq.URL.Query()
		for key, value := range route.config.Query {
			query.Set(key, value)
		}
		proxyReq.URL.RawQuery = query.Encode()
	}

	// Add gateway headers
	proxyReq.Header.Set("X-Gateway-Request-ID", c.GetHeader("X-Request-ID"))
	proxyReq.Header.Set("X-Forwarded-For", c.ClientIP())
	proxyReq.Header.Set("X-Forwarded-Proto", "http")
	if c.Request.TLS != nil {
		proxyReq.Header.Set("X-Forwarded-Proto", "https")
	}

	// Inject tracing context into proxy request
	p.gateway.InjectTracingHeaders(proxyReq, c.Request.Context())

	// Execute request with retries
	var resp *http.Response
	var lastErr error
	
	maxRetries := route.retries
	if maxRetries <= 0 {
		maxRetries = 1
	}

	for i := 0; i < maxRetries; i++ {
		resp, err = p.client.Do(proxyReq)
		if err == nil {
			break
		}
		lastErr = err
		if i < maxRetries-1 {
			time.Sleep(time.Duration(i+1) * 100 * time.Millisecond) // Exponential backoff
		}
	}

	if err != nil {
		return fmt.Errorf("failed to execute proxy request after %d retries: %w", maxRetries, lastErr)
	}
	defer resp.Body.Close()

	// Copy response headers
	p.copyHeaders(resp.Header, c.Writer.Header())
	
	// Set status code
	c.Status(resp.StatusCode)

	// Copy response body
	_, err = io.Copy(c.Writer, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to copy response body: %w", err)
	}

	return nil
}

// copyHeaders copies headers from source to destination
func (p *Proxy) copyHeaders(src, dst http.Header) {
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

// handleProxyError handles proxy errors
func (p *Proxy) handleProxyError(c *gin.Context, err error) {
	switch {
	case IsTimeout(err):
		c.JSON(http.StatusGatewayTimeout, gin.H{"error": "Gateway timeout"})
	case IsTooManyRetries(err):
		c.JSON(http.StatusBadGateway, gin.H{"error": "Service temporarily unavailable"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
	}
}

// healthCheck handles health check requests
func (g *Gateway) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
	})
}

// readinessCheck handles readiness check requests
func (g *Gateway) readinessCheck(c *gin.Context) {
	// Check if gateway is started
	if !g.started {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not ready",
			"reason": "gateway not started",
		})
		return
	}

	g.router.mu.RLock()
	routeCount := len(g.router.routes)
	healthyRouteCount := 0
	for _, r := range g.router.routes {
		if len(r.healthyTargets) > 0 {
			healthyRouteCount++
		}
	}
	g.router.mu.RUnlock()

	if routeCount == 0 {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not ready",
			"reason": "no routes configured",
		})
		return
	}

	if healthyRouteCount == 0 {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not ready",
			"reason": "no healthy upstream",
			"routes": routeCount,
			"timestamp": time.Now().Unix(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "ready",
		"routes":     routeCount,
		"healthy_routes": healthyRouteCount,
		"timestamp":  time.Now().Unix(),
	})
}

// gatewayInfo returns gateway information
func (g *Gateway) gatewayInfo(c *gin.Context) {
	g.router.mu.RLock()
	routes := make([]map[string]interface{}, 0, len(g.router.routes))
	for _, route := range g.router.routes {
		routes = append(routes, map[string]interface{}{
			"path":    route.config.Path,
			"method":  route.config.Method,
			"service": route.config.Service,
			"targets": len(route.targets),
		})
	}
	g.router.mu.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"version":   "1.0.0",
		"host":      g.server.host,
		"port":      g.server.port,
		"started":   g.started,
		"routes":    routes,
		"timestamp": time.Now().Unix(),
	})
}
