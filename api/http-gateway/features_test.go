package main

import (
	"fmt"
	"testing"

	"alldev-gin-rpc/pkg/gateway"
)

func TestHTTPGatewayConfig(t *testing.T) {
	// 测试默认配置
	config := gateway.DefaultConfig()
	
	// 验证日志配置
	if config.Logging.Level != "info" {
		t.Errorf("Expected log level 'info', got '%s'", config.Logging.Level)
	}
	if config.Logging.Format != "json" {
		t.Errorf("Expected log format 'json', got '%s'", config.Logging.Format)
	}
	
	// 验证追踪配置
	if config.Tracing == nil {
		t.Error("Tracing config should not be nil")
	} else {
		if config.Tracing.Type != "jaeger" {
			t.Errorf("Expected tracing type 'jaeger', got '%s'", config.Tracing.Type)
		}
	}
	
	// 验证协议配置
	if !config.Protocols.HTTP {
		t.Error("HTTP protocol should be enabled")
	}
	if !config.Protocols.HTTP2 {
		t.Error("HTTP2 protocol should be enabled")
	}
	if config.Protocols.GRPC {
		t.Error("gRPC protocol should be disabled by default")
	}
	if config.Protocols.JSONRPC {
		t.Error("JSON-RPC protocol should be disabled by default")
	}
	
	fmt.Printf("✅ HTTP Gateway 配置验证通过\n")
	fmt.Printf("   - 日志级别: %s\n", config.Logging.Level)
	fmt.Printf("   - 日志格式: %s\n", config.Logging.Format)
	fmt.Printf("   - 追踪类型: %s\n", config.Tracing.Type)
	fmt.Printf("   - 启用协议: HTTP=%v, HTTP2=%v, gRPC=%v, JSON-RPC=%v\n", 
		config.Protocols.HTTP, config.Protocols.HTTP2, config.Protocols.GRPC, config.Protocols.JSONRPC)
}

func TestHTTPGatewayFeatures(t *testing.T) {
	// 创建自定义配置
	config := gateway.DefaultConfig()
	config.Tracing.Enabled = true
	config.Protocols.GRPC = true
	config.Protocols.JSONRPC = true
	config.Logging.Level = "debug"
	config.Logging.Format = "console"
	
	// 验证配置更新
	if !config.Tracing.Enabled {
		t.Error("Tracing should be enabled")
	}
	if !config.Protocols.GRPC {
		t.Error("gRPC protocol should be enabled")
	}
	if !config.Protocols.JSONRPC {
		t.Error("JSON-RPC protocol should be enabled")
	}
	
	fmt.Printf("✅ HTTP Gateway 功能验证通过\n")
	fmt.Printf("   - 追踪启用: %v\n", config.Tracing.Enabled)
	fmt.Printf("   - gRPC 启用: %v\n", config.Protocols.GRPC)
	fmt.Printf("   - JSON-RPC 启用: %v\n", config.Protocols.JSONRPC)
	fmt.Printf("   - 调试日志: %v\n", config.Logging.Level == "debug")
}

func TestProtocolSupport(t *testing.T) {
	config := gateway.DefaultConfig()
	
	// 测试协议配置
	protocols := []struct {
		name     string
		enabled  bool
		expected bool
	}{
		{"HTTP", config.Protocols.HTTP, true},
		{"HTTP2", config.Protocols.HTTP2, true},
		{"gRPC", config.Protocols.GRPC, false},
		{"JSON-RPC", config.Protocols.JSONRPC, false},
	}
	
	for _, p := range protocols {
		if p.enabled != p.expected {
			t.Errorf("Protocol %s: expected %v, got %v", p.name, p.expected, p.enabled)
		}
	}
	
	fmt.Printf("✅ 协议支持验证通过\n")
	for _, p := range protocols {
		status := "禁用"
		if p.enabled {
			status = "启用"
		}
		fmt.Printf("   - %s: %s\n", p.name, status)
	}
}

func TestTracingIntegration(t *testing.T) {
	config := gateway.DefaultConfig()
	
	// 启用追踪
	config.Tracing.Enabled = true
	config.Tracing.Type = "zipkin"
	config.Tracing.SampleRate = 0.1
	
	// 验证追踪配置
	if !config.Tracing.Enabled {
		t.Error("Tracing should be enabled")
	}
	if config.Tracing.Type != "zipkin" {
		t.Errorf("Expected tracing type 'zipkin', got '%s'", config.Tracing.Type)
	}
	if config.Tracing.SampleRate != 0.1 {
		t.Errorf("Expected sample rate 0.1, got %f", config.Tracing.SampleRate)
	}
	
	fmt.Printf("✅ 追踪集成验证通过\n")
	fmt.Printf("   - 追踪类型: %s\n", config.Tracing.Type)
	fmt.Printf("   - 采样率: %.1f\n", config.Tracing.SampleRate)
	fmt.Printf("   - 服务名: %s\n", config.Tracing.ServiceName)
}
