package main

import (
	"fmt"
	"log"

	"alldev-gin-rpc/pkg/gateway"
)

func main() {
	// 创建认证配置
	config := gateway.AuthConfig{
		Enabled:    true,
		HeaderName: "X-API-Key",
		QueryName:  "api_key",
		APIKeys: map[string]string{
			"test-key": "test-user",
		},
	}
	
	// 创建认证中间件
	auth := gateway.NewGatewayAuth(config)
	
	// 测试路径匹配
	fmt.Println("=== RPC Route Detection Test ===")
	
	// 测试各种路径
	testPaths := []struct {
		path     string
		method   string
		protocol string
		expected bool
	}{
		{"/rpc/payment", "POST", "jsonrpc", true},
		{"/grpc/users", "GET", "grpc", true},
		{"/api/users", "GET", "http", false},
		{"/health", "GET", "http", false},
		{"/v1/products", "GET", "grpc", true},
		{"/api/orders", "GET", "http", false},
	}
	
	for _, test := range testPaths {
		// 模拟上下文
		c := &mockContext{
			path:     test.path,
			method:   test.method,
			protocol: test.protocol,
		}
		
		// 测试路由检测
		isRPC := auth.IsRPCRoute(c)
		
		fmt.Printf("Path: %-15s Method: %-6s Protocol: %-8s -> RPC: %v (Expected: %v)\n", 
			test.path, test.method, test.protocol, isRPC, test.expected)
	}
	
	fmt.Println("\n=== Test completed ===")
}

// mockContext 模拟 gin.Context
type mockContext struct {
	path     string
	method   string
	protocol string
}

func (m *mockContext) Request() interface{} {
	return &mockRequest{
		path:   m.path,
		method: m.method,
	}
}

func (m *mockContext) Get(key string) (interface{}, bool) {
	if key == "protocol" {
		return m.protocol, true
	}
	return nil, false
}

// mockRequest 模拟 HTTP 请求
type mockRequest struct {
	path   string
	method string
}

func (m *mockRequest) URL() interface{} {
	return &mockURL{path: m.path}
}

func (m *mockRequest) Method() string {
	return m.method
}

// mockURL 模拟 URL
type mockURL struct {
	path string
}

func (m *mockURL) Path() string {
	return m.path
}

// 为 GatewayAuth 添加测试方法
func (a *gateway.GatewayAuth) IsRPCRoute(c interface{}) bool {
	// 这里我们需要访问私有方法，所以我们需要在 gateway 包中添加一个公共方法
	// 为了测试，我们直接调用逻辑
	
	// 从上下文获取协议信息
	var protocol string
	var path string
	var method string
	
	switch ctx := c.(type) {
	case *mockContext:
		protocol = ctx.protocol
		path = ctx.path
		method = ctx.method
	default:
		return false
	}
	
	// 检查协议
	if protocol == "grpc" || protocol == "jsonrpc" {
		return true
	}
	
	// 检查路径模式
	if isGRPCRoute(path, method) || isJSONRPCRoute(path, method) {
		return true
	}
	
	return false
}

func isGRPCRoute(path, method string) bool {
	grpcPatterns := []string{
		"/grpc/",
		"/api/grpc/",
		"/v1/",
		"/v2/",
	}
	
	for _, pattern := range grpcPatterns {
		if len(path) >= len(pattern) && path[:len(pattern)] == pattern {
			return true
		}
	}
	
	grpcKeywords := []string{"grpc", "proto", "service"}
	for _, keyword := range grpcKeywords {
		if len(path) >= len(keyword) {
			// 简单的包含检查
			for i := 0; i <= len(path)-len(keyword); i++ {
				if path[i:i+len(keyword)] == keyword {
					return true
				}
			}
		}
	}
	
	return false
}

func isJSONRPCRoute(path, method string) bool {
	jsonrpcPatterns := []string{
		"/rpc/",
		"/api/rpc/",
		"/jsonrpc/",
		"/api/jsonrpc/",
	}
	
	for _, pattern := range jsonrpcPatterns {
		if len(path) >= len(pattern) && path[:len(pattern)] == pattern {
			return true
		}
	}
	
	if method != "POST" {
		return false
	}
	
	jsonrpcKeywords := []string{"rpc", "jsonrpc", "service"}
	for _, keyword := range jsonrpcKeywords {
		if len(path) >= len(keyword) {
			// 简单的包含检查
			for i := 0; i <= len(path)-len(keyword); i++ {
				if path[i:i+len(keyword)] == keyword {
					return true
				}
			}
		}
	}
	
	return false
}
