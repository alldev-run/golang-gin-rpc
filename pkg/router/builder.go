package router

import (
	"net/http"

	"alldev-gin-rpc/pkg/gateway"
	"alldev-gin-rpc/pkg/middleware/gin"

	"github.com/gin-gonic/gin"
)

// RouterBuilder 路由构建器
type RouterBuilder struct {
	cfg      *gateway.Config
	registry *RouteRegistry
	engine   *gin.Engine
}

// NewRouterBuilder 创建新的路由构建器
func NewRouterBuilder(cfg *gateway.Config) *RouterBuilder {
	// 设置 Gin 模式（默认为 Release 模式）
	gin.SetMode(gin.ReleaseMode)
	
	// 创建 Gin 引擎
	engine := gin.New()
	
	// 添加全局中间件
	engine.Use(
		middleware.Recovery(),
		middleware.RequestID(),
		middleware.CORSFromGatewayConfig(cfg),
		middleware.RateLimitFromGatewayConfig(cfg),
		middleware.Logging(),
	)
	
	// 如果启用了追踪，添加追踪中间件
	if cfg.Tracing != nil && cfg.Tracing.Enabled {
		engine.Use(middleware.TracingFromGatewayConfig(cfg))
	}
	
	return &RouterBuilder{
		cfg:      cfg,
		registry: NewRouteRegistry(),
		engine:   engine,
	}
}

// RegisterDebugRoutes 注册调试路由
func (rb *RouterBuilder) RegisterDebugRoutes() {
	debug := rb.engine.Group("/debug")
	{
		debug.GET("/ok", rb.debugOK)
		debug.GET("/request-id", rb.debugRequestID)
		debug.GET("/tracing", rb.debugTracing)
	}
	
	// 根路由
	rb.engine.GET("/", rb.root)
}

// RegisterBusinessRoutes 注册业务路由
func (rb *RouterBuilder) RegisterBusinessRoutes(registrar interface{}) {
	if routeRegistrar, ok := registrar.(interface{ RegisterAll(*RouteRegistry) }); ok {
		routeRegistrar.RegisterAll(rb.registry)
	}
}

// Build 构建最终的路由处理器
func (rb *RouterBuilder) Build() http.Handler {
	// 注册业务路由到 Gin 引擎
	businessHandler := rb.registry.Build()
	rb.engine.Any("/api/*path", gin.WrapH(businessHandler))
	
	return rb.engine
}

// GetEngine 获取 Gin 引擎（用于扩展）
func (rb *RouterBuilder) GetEngine() interface{} {
	return rb.engine
}

// GetRegistry 获取路由注册器（用于扩展）
func (rb *RouterBuilder) GetRegistry() interface{} {
	return rb.registry
}

// GetRouteRegistry 获取类型安全的路由注册器
func (rb *RouterBuilder) GetRouteRegistry() *RouteRegistry {
	return rb.registry
}

// 调试路由处理器

func (rb *RouterBuilder) root(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Hello from Gateway",
		"service": rb.cfg.ServiceName,
	})
}

func (rb *RouterBuilder) debugOK(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (rb *RouterBuilder) debugRequestID(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"request_id_hdr":  c.GetHeader("X-Request-ID"),
		"request_id_resp": c.Writer.Header().Get("X-Request-ID"),
	})
}

func (rb *RouterBuilder) debugTracing(c *gin.Context) {
	traceInfo := gin.H{
		"tracing_enabled": rb.cfg.Tracing != nil && rb.cfg.Tracing.Enabled,
		"trace_id":       c.GetHeader("X-Trace-ID"),
		"span_id":        c.GetHeader("X-Span-ID"),
	}
	
	if rb.cfg.Tracing != nil {
		traceInfo["tracing_type"] = rb.cfg.Tracing.Type
		traceInfo["tracing_service"] = rb.cfg.Tracing.ServiceName
		traceInfo["tracing_sample_rate"] = rb.cfg.Tracing.SampleRate
	}
	
	c.JSON(http.StatusOK, traceInfo)
}
