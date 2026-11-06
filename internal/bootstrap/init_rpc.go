package bootstrap

import (
	"fmt"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
	"github.com/alldev-run/golang-gin-rpc/plugins/discovery"
	"github.com/alldev-run/golang-gin-rpc/plugins/rpc"
)

// InitializeRPC initializes RPC services
func (b *Bootstrap) InitializeRPC() error {
	if !b.pluginConfig.RPC.Enabled {
		logger.Info("No RPC servers configured, skipping RPC initialization")
		return nil
	}

	managerConfig := rpc.DefaultManagerConfig()
	if servers := b.pluginConfig.RPC.Servers; len(servers) > 0 {
		// rpc.servers 段存在时完全按配置构建（端口/超时/消息上限/reflection 均可配）。
		managerConfig.Servers = make(map[string]rpc.Config, len(servers))
		for name, s := range servers {
			serverType := rpc.ServerTypeGRPC
			if s.Type == "jsonrpc" {
				serverType = rpc.ServerTypeJSONRPC
			}
			managerConfig.Servers[name] = rpc.Config{
				Type:       serverType,
				Host:       s.Host,
				Port:       s.Port,
				Network:    firstNonEmpty(s.Network, "tcp"),
				Timeout:    rpcTimeoutOrDefault(s.Timeout),
				MaxMsgSize: rpcMaxMsgSizeOrDefault(s.MaxMsgSize),
				Reflection: s.Reflection,
			}
		}
	} else {
		switch b.pluginConfig.RPC.Protocol {
		case "jsonrpc":
			managerConfig.Servers = map[string]rpc.Config{
				"jsonrpc": {
					Type:    rpc.ServerTypeJSONRPC,
					Host:    b.config.Server.HTTP.Host,
					Port:    b.config.Server.HTTP.Port,
					Network: "tcp",
					Timeout: int((30 * time.Second).Seconds()),
				},
			}
		case "grpc":
			fallthrough
		default:
			managerConfig.Servers = map[string]rpc.Config{
				"grpc": {
					Type:       rpc.ServerTypeGRPC,
					Host:       b.config.Server.GRPC.Host,
					Port:       b.config.Server.GRPC.Port,
					Network:    "tcp",
					Timeout:    int((30 * time.Second).Seconds()),
					MaxMsgSize: 4 * 1024 * 1024,
					Reflection: true,
				},
			}
		}
	}

	b.rpcManager = rpc.NewManager(managerConfig)
	b.setDependency(DepRPCManager, b.rpcManager)

	if b.pluginConfig.RPC.Degradation.Enabled {
		dmConfig := rpc.DefaultDegradationConfig()
		dmConfig.Enabled = b.pluginConfig.RPC.Degradation.Enabled
		dmConfig.AutoDetect = b.pluginConfig.RPC.Degradation.AutoDetect
		dmConfig.CPUThreshold = b.pluginConfig.RPC.Degradation.CPUThreshold
		dmConfig.MemoryThreshold = b.pluginConfig.RPC.Degradation.MemoryThreshold
		dmConfig.ErrorRateThreshold = b.pluginConfig.RPC.Degradation.ErrorRateThreshold
		dmConfig.LatencyThreshold = b.pluginConfig.RPC.Degradation.LatencyThreshold
		dmConfig.EnableCircuitBreaker = b.pluginConfig.RPC.Degradation.EnableCircuitBreaker
		dmConfig.Level = parseDegradationLevel(b.pluginConfig.RPC.Degradation.Level)

		dm, err := rpc.NewDegradationManager(dmConfig)
		if err != nil {
			return fmt.Errorf("failed to create degradation manager: %w", err)
		}
		b.rpcManager.SetDegradationManager(dm)
	}

	logger.Info("Initializing RPC services",
		logger.String("protocol", b.pluginConfig.RPC.Protocol))

	if err := b.rpcManager.Start(); err != nil {
		return fmt.Errorf("failed to start RPC manager: %w", err)
	}

	if b.discoveryManager != nil {
		if err := b.rpcManager.SetDiscoveryIntegration(b.discoveryManager, rpc.DiscoveryRegistrationConfig{
			Enabled:        b.pluginConfig.Discovery.Enabled,
			ServiceName:    b.config.App.Name,
			ServiceAddress: firstNonEmpty(b.pluginConfig.Discovery.Config["service_address"], firstNonEmpty(b.config.Server.GRPC.Host, b.config.Server.HTTP.Host)),
			ServiceTags:    append([]string{"rpc", b.pluginConfig.RPC.Protocol}, splitAndTrim(b.pluginConfig.Discovery.Config["rpc_tags"])...),
			Metadata:       map[string]string(b.pluginConfig.Discovery.Config),
		}); err != nil {
			return fmt.Errorf("failed to connect rpc manager with discovery manager: %w", err)
		}
	}

	logger.Info("RPC services initialized successfully")
	return nil
}

// InitializeDiscovery initializes service discovery
func (b *Bootstrap) InitializeDiscovery() error {
	if !b.pluginConfig.Discovery.Enabled {
		logger.Info("Service discovery disabled, skipping discovery initialization")
		return nil
	}

	if b.pluginConfig.Discovery.Address == "" {
		logger.Warn("Service discovery enabled but no registry address configured, skipping discovery initialization")
		return nil
	}

	managerConfig := discovery.DefaultManagerConfig()
	managerConfig.Enabled = b.pluginConfig.Discovery.Enabled
	managerConfig.RegistryType = b.pluginConfig.Discovery.Type
	managerConfig.RegistryAddress = b.pluginConfig.Discovery.Address
	managerConfig.Timeout = b.pluginConfig.Discovery.Timeout
	managerConfig.ServiceName = b.config.App.Name
	managerConfig.ServiceAddress = b.config.Server.HTTP.Host
	managerConfig.ServicePort = b.config.Server.HTTP.Port

	manager, err := discovery.NewServiceDiscoveryManager(managerConfig)
	if err != nil {
		return fmt.Errorf("failed to create discovery manager: %w", err)
	}

	// Start discovery manager
	if err := manager.Start(); err != nil {
		return fmt.Errorf("failed to start discovery manager: %w", err)
	}

	b.discoveryManager = manager
	b.setDependency(DepDiscoveryManager, manager)

	if b.rpcManager != nil {
		if err := b.rpcManager.SetDiscoveryIntegration(manager, rpc.DiscoveryRegistrationConfig{
			Enabled:        b.pluginConfig.Discovery.Enabled,
			ServiceName:    b.config.App.Name,
			ServiceAddress: firstNonEmpty(b.pluginConfig.Discovery.Config["service_address"], firstNonEmpty(b.config.Server.GRPC.Host, b.config.Server.HTTP.Host)),
			ServiceTags:    append([]string{"rpc", b.pluginConfig.RPC.Protocol}, splitAndTrim(b.pluginConfig.Discovery.Config["rpc_tags"])...),
			Metadata:       map[string]string(b.pluginConfig.Discovery.Config),
		}); err != nil {
			return fmt.Errorf("failed to connect discovery manager with rpc manager: %w", err)
		}
	}

	logger.Info("Service discovery initialized successfully")
	return nil
}
