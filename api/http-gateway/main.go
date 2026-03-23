package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
	
	"gopkg.in/yaml.v3"

	"alldev-gin-rpc/api/http-gateway/internal/httpapi"
	"alldev-gin-rpc/api/http-gateway/internal/mw"
	"alldev-gin-rpc/internal/bootstrap"
	"alldev-gin-rpc/pkg/gateway"
	"alldev-gin-rpc/pkg/logger"
)

func main() {
	// 配置文件路径
	configPath := "./api/http-gateway/config/config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	// 加载配置
	gwCfg, err := loadGatewayConfig(configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// 初始化日志
	loggerCfg := logger.DefaultConfig()
	if gwCfg.Logging.Level != "" {
		loggerCfg.Level = logger.LogLevel(gwCfg.Logging.Level)
	}
	if gwCfg.Logging.Format != "" {
		loggerCfg.Format = logger.LogFormat(gwCfg.Logging.Format)
	}
	logger.Init(loggerCfg)

	logger.Info("http-gateway starting", 
		logger.String("version", "1.0.0"),
		logger.String("config", configPath),
		logger.String("service", gwCfg.ServiceName),
		logger.String("protocols", getEnabledProtocols(gwCfg)),
	)

	frameworkConfigPath := "./configs/config.yaml"
	if len(os.Args) > 2 {
		frameworkConfigPath = os.Args[2]
	}

	boot, err := bootstrap.NewBootstrap(frameworkConfigPath)
	if err != nil {
		log.Fatalf("failed to initialize bootstrap: %v", err)
	}
	defer boot.Close()

	const gatewayServiceName = "api-gateway.http-gateway"

	// 创建业务处理器
	bizHandler := httpapi.NewRouter(gwCfg).Handler()
	if err := boot.RegisterAPIGatewayServiceFactory(bootstrap.APIGatewayServiceOptions{
		Name:   gatewayServiceName,
		Config: gwCfg,
		HTTPOptions: gateway.HTTPServiceOptions{
			BizHandler:     bizHandler,
			IsBusinessPath: httpapi.IsBusinessPath,
			Middlewares:    mw.Middlewares(),
		},
	}); err != nil {
		log.Fatalf("failed to register api-gateway service factory: %v", err)
	}

	frameworkOptions := bootstrap.FrameworkOptions{
		InitDatabases: true,
		InitCache:     true,
		InitDiscovery: true,
		InitTracing:   true,
		InitAuth:      true,
		Services:      []string{gatewayServiceName},
	}

	if err := boot.StartFramework(context.Background(), frameworkOptions); err != nil {
		log.Fatalf("failed to start framework services: %v", err)
	}

	logger.Info("http-gateway service started",
		logger.String("addr", fmt.Sprintf("%s:%d", gwCfg.Host, gwCfg.Port)),
		logger.String("protocols", getEnabledProtocols(gwCfg)),
	)

	// 启动服务器
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// 等待信号
	<-sigCh
	logger.Info("http-gateway shutting down")

	// 关闭框架托管服务
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := boot.StopFramework(shutdownCtx, frameworkOptions.Services...); err != nil {
		logger.Errorf("framework shutdown failed", logger.Error(err))
	}
	
	logger.Info("http-gateway stopped")
}

// loadGatewayConfig 加载网关配置
func loadGatewayConfig(path string) (*gateway.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := gateway.DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return cfg, nil
}

// getEnabledProtocols 获取启用的协议列表
func getEnabledProtocols(cfg *gateway.Config) string {
	var protocols []string
	
	if cfg.Protocols.HTTP {
		protocols = append(protocols, "HTTP")
	}
	if cfg.Protocols.HTTP2 {
		protocols = append(protocols, "HTTP2")
	}
	if cfg.Protocols.GRPC {
		protocols = append(protocols, "gRPC")
	}
	if cfg.Protocols.JSONRPC {
		protocols = append(protocols, "JSON-RPC")
	}
	
	if len(protocols) == 0 {
		return "HTTP"
	}
	
	result := protocols[0]
	for i := 1; i < len(protocols); i++ {
		result += "/" + protocols[i]
	}
	
	return result
}
