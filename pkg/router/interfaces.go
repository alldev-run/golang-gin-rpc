package router

import (
	"net/http"

	"alldev-gin-rpc/pkg/gateway"
)

// RouteRegistrar 路由注册器接口
type RouteRegistrar interface {
	RegisterAll(registry *RouteRegistry)
}

// RouteHandler 路由处理器接口
type RouteHandler interface {
	Handler() http.Handler
}

// IRouterBuilder 路由构建器接口
type IRouterBuilder interface {
	// 注册调试路由
	RegisterDebugRoutes()
	
	// 注册业务路由
	RegisterBusinessRoutes(registrar interface{})
	
	// 构建最终处理器
	Build() http.Handler
	
	// 获取底层引擎（用于扩展）
	GetEngine() interface{}
	
	// 获取路由注册器（用于扩展）
	GetRegistry() interface{}
}

// RouterFactory 路由工厂接口
type RouterFactory interface {
	CreateRouterBuilder(cfg *gateway.Config) IRouterBuilder
}

// DefaultRouterFactory 默认路由工厂
type DefaultRouterFactory struct{}

// CreateRouterBuilder 创建路由构建器
func (f *DefaultRouterFactory) CreateRouterBuilder(cfg *gateway.Config) IRouterBuilder {
	return NewRouterBuilder(cfg)
}

// NewRouterFactory 创建路由工厂
func NewRouterFactory() RouterFactory {
	return &DefaultRouterFactory{}
}

// 全局路由工厂实例
var globalRouterFactory RouterFactory = NewRouterFactory()

// SetRouterFactory 设置全局路由工厂（用于替换路由实现）
func SetRouterFactory(factory RouterFactory) {
	globalRouterFactory = factory
}

// GetRouterFactory 获取全局路由工厂
func GetRouterFactory() RouterFactory {
	return globalRouterFactory
}

// CreateRouter 使用全局工厂创建路由
func CreateRouter(cfg *gateway.Config) IRouterBuilder {
	return globalRouterFactory.CreateRouterBuilder(cfg)
}
