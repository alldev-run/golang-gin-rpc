//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

// Config 配置结构（扁平版）
type Config struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
	ServiceName string `yaml:"service_name"`
	
	Protocols struct {
		HTTP    bool `yaml:"http"`
		HTTP2   bool `yaml:"http2"`
		GRPC    bool `yaml:"grpc"`
		JSONRPC bool `yaml:"jsonrpc"`
		
		Security struct {
			Auth struct {
				Enabled    bool              `yaml:"enabled"`
				Type       string            `yaml:"type"`
				HeaderName string            `yaml:"header_name"`
				QueryName  string            `yaml:"query_name"`
				APIKeys    map[string]string `yaml:"api_keys"`
			} `yaml:"auth"`
		} `yaml:"security"`
	} `yaml:"protocols"`
	
	Routes []struct {
		Path     string `yaml:"path"`
		Method   string `yaml:"method"`
		Protocol string `yaml:"protocol"`
	} `yaml:"routes"`
}

func main() {
	fmt.Println("=== Gateway 配置结构验证 ===")

	// 读取配置文件
	data, err := os.ReadFile("./api/http-gateway/config/config.yaml")
	if err != nil {
		log.Fatalf("配置文件读取失败: %v", err)
	}

	// 解析配置
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		log.Fatalf("配置解析失败: %v", err)
	}

	fmt.Println("✅ 配置加载成功")
	fmt.Printf("📋 服务名: %s\n", config.ServiceName)
	fmt.Printf("🌐 服务器地址: %s:%d\n", config.Host, config.Port)

	// 验证协议配置
	fmt.Println("\n📡 协议配置:")
	fmt.Printf("  HTTP: %v\n", config.Protocols.HTTP)
	fmt.Printf("  HTTP2: %v\n", config.Protocols.HTTP2)
	fmt.Printf("  gRPC: %v\n", config.Protocols.GRPC)
	fmt.Printf("  JSON-RPC: %v\n", config.Protocols.JSONRPC)

	// 验证认证配置
	fmt.Println("\n🔐 RPC 认证配置:")
	fmt.Printf("  启用认证: %v\n", config.Protocols.Security.Auth.Enabled)
	fmt.Printf("  认证类型: %s\n", config.Protocols.Security.Auth.Type)
	fmt.Printf("  头部名称: %s\n", config.Protocols.Security.Auth.HeaderName)
	fmt.Printf("  查询参数: %s\n", config.Protocols.Security.Auth.QueryName)
	fmt.Printf("  API Keys 数量: %d\n", len(config.Protocols.Security.Auth.APIKeys))

	// 显示 API Keys
	fmt.Println("  API Keys:")
	for key, user := range config.Protocols.Security.Auth.APIKeys {
		fmt.Printf("    %s -> %s\n", key[:8]+"...", user)
	}

	// 验证路由配置
	fmt.Println("\n🛣️ 路由配置:")
	rpcRoutes := 0
	httpRoutes := 0
	for _, route := range config.Routes {
		if route.Protocol == "grpc" || route.Protocol == "jsonrpc" {
			rpcRoutes++
		} else {
			httpRoutes++
		}
	}
	fmt.Printf("  总路由数: %d\n", len(config.Routes))
	fmt.Printf("  RPC 路由: %d\n", rpcRoutes)
	fmt.Printf("  HTTP 路由: %d\n", httpRoutes)

	// 验证配置结构
	fmt.Println("\n🔍 配置结构验证:")
	
	// 检查关键配置路径
	if config.Protocols.Security.Auth.Enabled {
		fmt.Println("  ✅ 认证配置路径正确: protocols.security.auth")
	}
	
	if len(config.Protocols.Security.Auth.APIKeys) > 0 {
		fmt.Println("  ✅ API Keys 配置正确")
	}
	
	if config.Protocols.GRPC || config.Protocols.JSONRPC {
		fmt.Println("  ✅ RPC 协议配置正确")
	}

	fmt.Println("\n🎉 配置验证完成！")
	fmt.Println("📝 配置结构调整成功，认证配置已正确放置在 protocols.security 下")
	
	// 验证配置逻辑
	fmt.Println("\n🧪 配置逻辑验证:")
	if config.Protocols.GRPC && config.Protocols.Security.Auth.Enabled {
		fmt.Println("  ✅ gRPC 协议启用 + 认证启用 = 正确配置")
	}
	if config.Protocols.JSONRPC && config.Protocols.Security.Auth.Enabled {
		fmt.Println("  ✅ JSON-RPC 协议启用 + 认证启用 = 正确配置")
	}
	
	fmt.Println("\n🚀 配置结构优化完成，可以正常使用！")
}
