package httpapi

import (
	"net/http"
	"strings"

	"github.com/alldev-run/golang-gin-rpc/api/http-gateway/internal/routes"
	"github.com/alldev-run/golang-gin-rpc/pkg/gateway"
	"github.com/alldev-run/golang-gin-rpc/pkg/router"
)

type Router struct {
	cfg    *gateway.Config
	handler http.Handler
}

func NewRouter(cfg *gateway.Config) *Router {
	// 使用 pkg 下的路由工厂创建构建器
	builder := router.CreateRouter(cfg)
	
	// 注册调试路由
	builder.RegisterDebugRoutes()
	
	// 注册业务路由
	builder.RegisterBusinessRoutes(&RouteRegistrarAdapter{})
	
	// 构建最终处理器
	handler := builder.Build()
	
	return &Router{
		cfg:    cfg,
		handler: handler,
	}
}

// RouteRegistrarAdapter 适配器，将 routes.RegisterAll 适配为 RouteRegistrar 接口
type RouteRegistrarAdapter struct{}

// RegisterAll 实现 RouteRegistrar 接口
func (a *RouteRegistrarAdapter) RegisterAll(registry *router.RouteRegistry) {
	routes.RegisterAll(registry)
}

func (r *Router) Handler() http.Handler {
	// 直接返回构建好的处理器
	return r.handler
}

func IsBusinessPath(path string) bool {
	if path == "/" {
		return true
	}
	return strings.HasPrefix(path, "/debug/") || strings.HasPrefix(path, "/api/")
}
