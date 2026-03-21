package main

import (
	"fmt"
	"net/http"

	"alldev-gin-rpc/api/http-gateway/internal/httpapi"
	"alldev-gin-rpc/pkg/gateway"
	"alldev-gin-rpc/pkg/router"
)

// CustomRouterBuilder 自定义路由构建器示例
type CustomRouterBuilder struct {
	cfg *gateway.Config
}

// NewCustomRouterBuilder 创建自定义路由构建器
func NewCustomRouterBuilder(cfg *gateway.Config) *CustomRouterBuilder {
	return &CustomRouterBuilder{cfg: cfg}
}

// RegisterDebugRoutes 注册调试路由（自定义实现）
func (rb *CustomRouterBuilder) RegisterDebugRoutes() {
	fmt.Println("Custom debug routes registered")
}

// RegisterBusinessRoutes 注册业务路由（自定义实现）
func (rb *CustomRouterBuilder) RegisterBusinessRoutes(registrar interface{}) {
	fmt.Println("Custom business routes registered")
}

// Build 构建处理器（自定义实现）
func (rb *CustomRouterBuilder) Build() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Custom Router Implementation - Service: %s", rb.cfg.ServiceName)
	})
	return mux
}

// GetEngine 获取引擎
func (rb *CustomRouterBuilder) GetEngine() interface{} {
	return nil // 自定义实现可能没有引擎
}

// GetRegistry 获取注册器
func (rb *CustomRouterBuilder) GetRegistry() interface{} {
	return nil // 自定义实现可能没有注册器
}

// CustomRouterFactory 自定义路由工厂
type CustomRouterFactory struct{}

// CreateRouterBuilder 创建自定义路由构建器
func (f *CustomRouterFactory) CreateRouterBuilder(cfg *gateway.Config) router.IRouterBuilder {
	return NewCustomRouterBuilder(cfg)
}

func main() {
	// 示例1：使用默认的 Gin 路由
	fmt.Println("=== Using Default Gin Router ===")
	cfg := &gateway.Config{
		ServiceName: "test-gateway",
		Host:        "localhost",
		Port:        8080,
	}

	// 使用默认路由
	defaultRouter := httpapi.NewRouter(cfg)
	fmt.Printf("Default router type: %T\n", defaultRouter.Handler())

	// 示例2：更换为自定义路由实现
	fmt.Println("\n=== Switching to Custom Router ===")
	
	// 设置自定义路由工厂
	router.SetRouterFactory(&CustomRouterFactory{})
	
	// 现在创建的路由将使用自定义实现
	customRouter := httpapi.NewRouter(cfg)
	fmt.Printf("Custom router type: %T\n", customRouter.Handler())

	// 示例3：恢复默认路由
	fmt.Println("\n=== Restoring Default Router ===")
	router.SetRouterFactory(router.NewRouterFactory())
	restoredRouter := httpapi.NewRouter(cfg)
	fmt.Printf("Restored router type: %T\n", restoredRouter.Handler())

	fmt.Println("\n=== Router Switching Demo Complete ===")
	fmt.Println("This demonstrates how easy it is to switch router implementations")
	fmt.Println("by just changing the router factory.")
}
