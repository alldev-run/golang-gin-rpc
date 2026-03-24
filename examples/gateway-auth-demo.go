//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"log"

	"github.com/alldev-run/golang-gin-rpc/pkg/gateway"
)

func main() {
	// 加载配置
	gwCfg, err := gateway.LoadGatewayConfig("./api/http-gateway/config/config.yaml")
	if err != nil {
		log.Fatalf("failed to load gateway config: %v", err)
	}

	// 直接创建 Gateway 实例（使用公开 API）
	gw := gateway.NewGateway(gwCfg)

	// 演示 API Key 管理
	fmt.Println("=== Gateway API Key Management Demo ===")

	// 添加新的 API Key
	gw.AddAPIKey("demo-key-123", "demo-application")
	fmt.Println("Added API key: demo-key-123")

	// 检查 API Key 是否存在
	if gw.HasAPIKey("demo-key-123") {
		fmt.Println("API key demo-key-123 exists")
	}

	// 获取认证中间件
	auth := gw.GetAuth()
	if auth != nil {
		fmt.Printf("Authentication enabled: %v\n", gwCfg.Security.Auth.Enabled)
		fmt.Printf("API Keys count: %d\n", len(gwCfg.Security.Auth.APIKeys))
		
		// 检查路径是否需要认证
		fmt.Printf("Path /api/users requires auth: %v\n", !auth.ShouldSkipAuth("/api/users"))
		fmt.Printf("Path /health requires auth: %v\n", !auth.ShouldSkipAuth("/health"))
	}

	// 移除 API Key
	gw.RemoveAPIKey("demo-key-123")
	fmt.Println("Removed API key: demo-key-123")

	fmt.Println("=== Demo completed ===")
}
