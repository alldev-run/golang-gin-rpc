package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RouteConfig 路由配置
type RouteConfig struct {
	Path        string
	Method      string
	Handler     gin.HandlerFunc
	Description string
	Middlewares []gin.HandlerFunc
}

// RouteGroup 路由组
type RouteGroup struct {
	name        string
	prefix      string
	routes      []RouteConfig
	middlewares []gin.HandlerFunc
	registry    *RouteRegistry
}

// RouteRegistry 路由注册器
type RouteRegistry struct {
	routes      []RouteConfig
	groups      map[string]*RouteGroup
	middlewares map[string][]gin.HandlerFunc
	engine      *gin.Engine
}

// NewRouteRegistry 创建新的路由注册器
func NewRouteRegistry() *RouteRegistry {
	return &RouteRegistry{
		routes:      make([]RouteConfig, 0),
		groups:      make(map[string]*RouteGroup),
		middlewares: make(map[string][]gin.HandlerFunc),
		engine:      gin.New(),
	}
}

// Group 创建路由组
func (rr *RouteRegistry) Group(name, prefix string) *RouteGroup {
	if existingGroup, exists := rr.groups[name]; exists {
		return existingGroup
	}

	group := &RouteGroup{
		name:        name,
		prefix:      prefix,
		routes:      make([]RouteConfig, 0),
		middlewares: make([]gin.HandlerFunc, 0),
		registry:    rr,
	}
	rr.groups[name] = group
	return group
}

// Use 添加全局中间件
func (rr *RouteRegistry) Use(middlewares ...gin.HandlerFunc) *RouteRegistry {
	for _, middleware := range middlewares {
		rr.middlewares["global"] = append(rr.middlewares["global"], middleware)
	}
	return rr
}

// Register 注册单个路由
func (rr *RouteRegistry) Register(method, path string, handler gin.HandlerFunc, description ...string) *RouteRegistry {
	route := RouteConfig{
		Path:        path,
		Method:      method,
		Handler:     handler,
		Description: "",
		Middlewares: make([]gin.HandlerFunc, 0),
	}
	if len(description) > 0 {
		route.Description = description[0]
	}

	rr.routes = append(rr.routes, route)
	return rr
}

// GET 注册GET路由
func (rr *RouteRegistry) GET(path string, handler gin.HandlerFunc, description ...string) *RouteRegistry {
	return rr.Register("GET", path, handler, description...)
}

// POST 注册POST路由
func (rr *RouteRegistry) POST(path string, handler gin.HandlerFunc, description ...string) *RouteRegistry {
	return rr.Register("POST", path, handler, description...)
}

// PUT 注册PUT路由
func (rr *RouteRegistry) PUT(path string, handler gin.HandlerFunc, description ...string) *RouteRegistry {
	return rr.Register("PUT", path, handler, description...)
}

// DELETE 注册DELETE路由
func (rr *RouteRegistry) DELETE(path string, handler gin.HandlerFunc, description ...string) *RouteRegistry {
	return rr.Register("DELETE", path, handler, description...)
}

// PATCH 注册PATCH路由
func (rr *RouteRegistry) PATCH(path string, handler gin.HandlerFunc, description ...string) *RouteRegistry {
	return rr.Register("PATCH", path, handler, description...)
}

// OPTIONS 注册OPTIONS路由
func (rr *RouteRegistry) OPTIONS(path string, handler gin.HandlerFunc, description ...string) *RouteRegistry {
	return rr.Register("OPTIONS", path, handler, description...)
}

// Build 构建路由处理器
func (rr *RouteRegistry) Build() http.Handler {
	// 注册全局路由
	for _, route := range rr.routes {
		handler := rr.applyMiddlewares(route.Handler, route.Middlewares, rr.middlewares["global"])
		rr.registerRoute(route.Method, route.Path, handler)
	}

	// 注册组路由
	for _, group := range rr.groups {
		// 创建组中间件
		groupMiddlewares := make([]gin.HandlerFunc, len(group.middlewares))
		copy(groupMiddlewares, group.middlewares)

		// 为组创建路由
		for _, route := range group.routes {
			fullPath := group.prefix + route.Path
			allMiddlewares := append(groupMiddlewares, route.Middlewares...)
			handler := rr.applyMiddlewares(route.Handler, allMiddlewares, rr.middlewares["global"])
			rr.registerRoute(route.Method, fullPath, handler)
		}
	}

	return rr.engine
}

// registerRoute 注册路由到 Gin 引擎
func (rr *RouteRegistry) registerRoute(method, path string, handler gin.HandlerFunc) {
	switch method {
	case "GET":
		rr.engine.GET(path, handler)
	case "POST":
		rr.engine.POST(path, handler)
	case "PUT":
		rr.engine.PUT(path, handler)
	case "DELETE":
		rr.engine.DELETE(path, handler)
	case "PATCH":
		rr.engine.PATCH(path, handler)
	case "OPTIONS":
		rr.engine.OPTIONS(path, handler)
	case "*":
		rr.engine.Any(path, handler)
	}
}

// applyMiddlewares 应用中间件
func (rr *RouteRegistry) applyMiddlewares(handler gin.HandlerFunc, middlewares ...[]gin.HandlerFunc) gin.HandlerFunc {
	result := handler

	// 应用所有中间件组（从后往前，符合 Gin 中间件链的执行顺序）
	for i := len(middlewares) - 1; i >= 0; i-- {
		group := middlewares[i]
		for j := len(group) - 1; j >= 0; j-- {
			result = func(next gin.HandlerFunc) gin.HandlerFunc {
				return func(c *gin.Context) {
					c.Set("next_handler", next)
					group[j](c)
					if !c.IsAborted() {
						next(c)
					}
				}
			}(result)
		}
	}

	return result
}

// RouteGroup 方法

// Use 为路由组添加中间件
func (rg *RouteGroup) Use(middlewares ...gin.HandlerFunc) *RouteGroup {
	rg.middlewares = append(rg.middlewares, middlewares...)
	return rg
}

// Register 注册路由到组
func (rg *RouteGroup) Register(method, path string, handler gin.HandlerFunc, description ...string) *RouteGroup {
	route := RouteConfig{
		Path:        path,
		Method:      method,
		Handler:     handler,
		Description: "",
		Middlewares: make([]gin.HandlerFunc, 0),
	}
	if len(description) > 0 {
		route.Description = description[0]
	}

	rg.routes = append(rg.routes, route)
	return rg
}

// GET 注册GET路由
func (rg *RouteGroup) GET(path string, handler gin.HandlerFunc, description ...string) *RouteGroup {
	return rg.Register("GET", path, handler, description...)
}

// POST 注册POST路由
func (rg *RouteGroup) POST(path string, handler gin.HandlerFunc, description ...string) *RouteGroup {
	return rg.Register("POST", path, handler, description...)
}

// PUT 注册PUT路由
func (rg *RouteGroup) PUT(path string, handler gin.HandlerFunc, description ...string) *RouteGroup {
	return rg.Register("PUT", path, handler, description...)
}

// DELETE 注册DELETE路由
func (rg *RouteGroup) DELETE(path string, handler gin.HandlerFunc, description ...string) *RouteGroup {
	return rg.Register("DELETE", path, handler, description...)
}

// PATCH 注册PATCH路由
func (rg *RouteGroup) PATCH(path string, handler gin.HandlerFunc, description ...string) *RouteGroup {
	return rg.Register("PATCH", path, handler, description...)
}

// Middleware 为最后添加的路由添加中间件
func (rg *RouteGroup) Middleware(middlewares ...gin.HandlerFunc) *RouteGroup {
	if len(rg.routes) > 0 {
		lastIdx := len(rg.routes) - 1
		rg.routes[lastIdx].Middlewares = append(rg.routes[lastIdx].Middlewares, middlewares...)
	}
	return rg
}

// ChainMiddlewares 链式组合中间件（工具函数）
func ChainMiddlewares(handler gin.HandlerFunc, middlewares ...gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 按顺序执行中间件
		for _, middleware := range middlewares {
			middleware(c)
			if c.IsAborted() {
				return
			}
		}
		// 最后执行处理器
		handler(c)
	}
}
