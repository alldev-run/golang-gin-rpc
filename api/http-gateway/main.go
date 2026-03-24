package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
	
	"gopkg.in/yaml.v3"

	"github.com/alldev-run/golang-gin-rpc/api/http-gateway/internal/httpapi"
	"github.com/alldev-run/golang-gin-rpc/api/http-gateway/internal/mw"
	"github.com/alldev-run/golang-gin-rpc/pkg/bootstrap"
	"github.com/alldev-run/golang-gin-rpc/pkg/config"
	"github.com/alldev-run/golang-gin-rpc/pkg/gateway"
	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
	"github.com/alldev-run/golang-gin-rpc/pkg/tracing"
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

	// 初始化全局日志
	loggerCfg := logger.DefaultConfig()
	if gwCfg.Logging.Level != "" {
		loggerCfg.Level = logger.LogLevel(gwCfg.Logging.Level)
	}
	if gwCfg.Logging.Format != "" {
		loggerCfg.Format = logger.LogFormat(gwCfg.Logging.Format)
	}
	logger.Init(loggerCfg)

	// 初始化 API Gateway 服务日志器
	gatewayLoggerConfig := logger.DefaultServiceLoggerConfig("api-gateway")
	gatewayLoggerConfig.EnableDateFolder = true
	gatewayLoggerConfig.SeparateByLevel = true
	gatewayLoggerConfig.InheritGlobalConfig = false
	gatewayLoggerConfig.OverrideConfig = loggerCfg
	gatewayLoggerConfig.OverrideConfig.Level = logger.LogLevelDebug // API Gateway 需要详细日志
	gatewayLoggerConfig.OverrideConfig.MaxSize = 200 // 更大的文件大小
	gatewayLoggerConfig.OverrideConfig.MaxBackups = 15
	gatewayLoggerConfig.OverrideConfig.MaxAge = 45
	
	gatewayLogger := logger.GetServiceLoggerInstance("api-gateway", gatewayLoggerConfig)

	gatewayLogger.Info("http-gateway starting", 
		logger.String("version", "1.0.0"),
		logger.String("config", configPath),
		logger.String("service", gwCfg.ServiceName),
		logger.String("protocols", getEnabledProtocols(gwCfg)),
	)

	frameworkConfigPath := resolveFrameworkConfigPath(configPath)
	if len(os.Args) > 2 {
		frameworkConfigPath = os.Args[2]
	}
	if frameworkConfigPath != "" {
		if _, err := os.Stat(frameworkConfigPath); err != nil {
			gatewayLogger.Warn("framework config path not found, fallback to defaults",
				logger.String("path", frameworkConfigPath),
				logger.Error(err),
			)
			frameworkConfigPath = ""
		}
	}

	var boot *bootstrap.Bootstrap
	if frameworkConfigPath != "" {
		boot, err = bootstrap.NewBootstrap(frameworkConfigPath)
	} else {
		boot, err = bootstrap.NewBootstrapWithDefaults()
	}
	if err != nil {
		log.Fatalf("failed to initialize bootstrap: %v", err)
	}
	defer boot.Close()

	frameworkGatewayBase := buildGatewayConfigFromFramework(boot.GetConfig())
	frameworkGatewayCfg := frameworkGatewayBase
	if frameworkConfigPath != "" {
		frameworkGatewayCfg, err = loadGatewayConfigWithBase(frameworkConfigPath, frameworkGatewayBase)
		if err != nil {
			log.Fatalf("failed to load gateway config from framework file: %v", err)
		}
	}
	mergedGwCfg, err := loadGatewayConfigWithBase(configPath, frameworkGatewayCfg)
	if err != nil {
		log.Fatalf("failed to merge gateway config with framework defaults: %v", err)
	}

	const gatewayServiceName = "api-gateway.http-gateway"

	// 创建业务处理器 - 框架已内置请求日志中间件
	bizHandler := httpapi.NewRouter(mergedGwCfg).Handler()
	
	if err := boot.RegisterAPIGatewayServiceFactory(bootstrap.APIGatewayServiceOptions{
		Name:   gatewayServiceName,
		Config: mergedGwCfg,
		HTTPOptions: gateway.HTTPServiceOptions{
			BizHandler:     bizHandler,
			IsBusinessPath: httpapi.IsBusinessPath,
			Middlewares:    mw.Middlewares(),
		},
	}); err != nil {
		log.Fatalf("failed to register api-gateway service factory: %v", err)
	}

	frameworkOptions := boot.FrameworkOptionsFromConfig()
	frameworkOptions.Services = []string{gatewayServiceName}

	serviceDBConfigPath := filepath.Join(filepath.Dir(configPath), "database.yml")
	if _, err := os.Stat(serviceDBConfigPath); err == nil {
		if err := bootstrap.LoadDatabaseConfig(boot, serviceDBConfigPath); err != nil {
			log.Fatalf("failed to load service database config: %v", err)
		}
		frameworkOptions.InitDatabases = true
		gatewayLogger.Info("service database config loaded and overrides framework database config",
			logger.String("path", serviceDBConfigPath),
		)
	} else if frameworkConfigPath == "" {
		frameworkOptions.InitDatabases = false
	}

	if frameworkConfigPath == "" {
		frameworkOptions.InitCache = false
		frameworkOptions.InitDiscovery = false
		frameworkOptions.InitTracing = false
		frameworkOptions.InitAuth = false
		frameworkOptions.InitMetrics = false
		frameworkOptions.InitHealth = true
		frameworkOptions.InitErrors = true
		frameworkOptions.ValidateDependencyCoverage = false
	}

	if err := boot.StartFramework(context.Background(), frameworkOptions); err != nil {
		log.Fatalf("failed to start framework services: %v", err)
	}

	gatewayLogger.Info("http-gateway service started",
		logger.String("addr", fmt.Sprintf("%s:%d", mergedGwCfg.Host, mergedGwCfg.Port)),
		logger.String("protocols", getEnabledProtocols(mergedGwCfg)),
	)

	// 启动服务器
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// 等待信号
	<-sigCh
	gatewayLogger.Info("http-gateway shutting down")

	// 关闭框架托管服务
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := boot.StopFramework(shutdownCtx, frameworkOptions.Services...); err != nil {
		gatewayLogger.Error("framework shutdown failed", logger.Error(err))
	}
	
	gatewayLogger.Info("http-gateway stopped")
}

// loadGatewayConfig 加载网关配置
func loadGatewayConfig(path string) (*gateway.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := gateway.DefaultConfig()
	if err := unmarshalGatewayConfig(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return cfg, nil
}

func loadGatewayConfigWithBase(path string, base *gateway.Config) (*gateway.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := cloneGatewayConfig(base)
	if err := unmarshalGatewayConfig(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return cfg, nil
}

func unmarshalGatewayConfig(data []byte, cfg *gateway.Config) error {
	if cfg == nil {
		return fmt.Errorf("gateway config target is nil")
	}

	raw := make(map[string]interface{})
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return err
	}

	if gatewayValue, ok := raw["gateway"]; ok {
		nested, err := yaml.Marshal(gatewayValue)
		if err != nil {
			return err
		}
		return yaml.Unmarshal(nested, cfg)
	}

	return yaml.Unmarshal(data, cfg)
}

func buildGatewayConfigFromFramework(cfg *config.GlobalConfig) *gateway.Config {
	base := gateway.DefaultConfig()
	if cfg == nil {
		return base
	}

	base.Host = cfg.Server.HTTP.Host
	base.Port = cfg.Server.HTTP.Port
	base.ReadTimeout = cfg.Server.HTTP.ReadTimeout
	base.WriteTimeout = cfg.Server.HTTP.WriteTimeout
	base.IdleTimeout = cfg.Server.HTTP.IdleTimeout
	base.ServiceName = cfg.App.Name

	base.CORS.AllowedOrigins = append([]string(nil), cfg.Security.CORS.AllowOrigins...)
	base.CORS.AllowedMethods = append([]string(nil), cfg.Security.CORS.AllowMethods...)
	base.CORS.AllowedHeaders = append([]string(nil), cfg.Security.CORS.AllowHeaders...)
	base.CORS.AllowCredentials = cfg.Security.CORS.AllowCredentials

	base.RateLimit.Enabled = cfg.Security.RateLimit.Enabled
	base.RateLimit.Requests = cfg.Security.RateLimit.Limit
	base.RateLimit.Window = cfg.Security.RateLimit.Window.String()

	base.Discovery.Enabled = cfg.Discovery.Enabled
	base.Discovery.Type = cfg.Discovery.Type
	base.Discovery.Timeout = cfg.Discovery.Timeout
	base.Discovery.Namespace = cfg.Discovery.Config["namespace"]
	if base.Discovery.Namespace == "" {
		base.Discovery.Namespace = "default"
	}
	if cfg.Discovery.Address != "" {
		base.Discovery.Endpoints = []string{cfg.Discovery.Address}
	}
	if base.Discovery.Options == nil {
		base.Discovery.Options = make(map[string]string)
	}
	for k, v := range cfg.Discovery.Config {
		base.Discovery.Options[k] = v
	}

	base.Tracing = &tracing.Config{
		Enabled:       cfg.Observability.Tracing.Enabled,
		Type:          cfg.Observability.Tracing.Type,
		Endpoint:      cfg.Observability.Tracing.Endpoint,
		SampleRate:    cfg.Observability.Tracing.SampleRate,
		ServiceName:   cfg.App.Name,
		ServiceVersion: cfg.App.Version,
		Environment:   cfg.App.Environment,
	}

	return base
}

func cloneGatewayConfig(src *gateway.Config) *gateway.Config {
	if src == nil {
		return gateway.DefaultConfig()
	}
	clone := *src
	clone.CORS.AllowedOrigins = append([]string(nil), src.CORS.AllowedOrigins...)
	clone.CORS.AllowedMethods = append([]string(nil), src.CORS.AllowedMethods...)
	clone.CORS.AllowedHeaders = append([]string(nil), src.CORS.AllowedHeaders...)
	clone.CORS.ExposedHeaders = append([]string(nil), src.CORS.ExposedHeaders...)
	clone.Discovery.Endpoints = append([]string(nil), src.Discovery.Endpoints...)
	if src.Discovery.Options != nil {
		clone.Discovery.Options = make(map[string]string, len(src.Discovery.Options))
		for k, v := range src.Discovery.Options {
			clone.Discovery.Options[k] = v
		}
	}
	clone.Routes = append([]gateway.RouteConfig(nil), src.Routes...)
	if src.Tracing != nil {
		tracingClone := *src.Tracing
		clone.Tracing = &tracingClone
	}
	return &clone
}

func resolveFrameworkConfigPath(gatewayConfigPath string) string {
	candidates := []string{
		"./configs/config.yaml",
		"./config/config.yaml",
		gatewayConfigPath,
	}
	for _, p := range candidates {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
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
