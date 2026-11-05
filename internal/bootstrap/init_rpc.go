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
	if !b.config.RPC.Enabled {
		logger.Info("No RPC servers configured, skipping RPC initialization")
		return nil
	}

	managerConfig := rpc.DefaultManagerConfig()
	switch b.config.RPC.Protocol {
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

	b.rpcManager = rpc.NewManager(managerConfig)
	b.setDependency(DepRPCManager, b.rpcManager)

	if b.config.RPC.Degradation.Enabled {
		dmConfig := rpc.DefaultDegradationConfig()
		dmConfig.Enabled = b.config.RPC.Degradation.Enabled
		dmConfig.AutoDetect = b.config.RPC.Degradation.AutoDetect
		dmConfig.CPUThreshold = b.config.RPC.Degradation.CPUThreshold
		dmConfig.MemoryThreshold = b.config.RPC.Degradation.MemoryThreshold
		dmConfig.ErrorRateThreshold = b.config.RPC.Degradation.ErrorRateThreshold
		dmConfig.LatencyThreshold = b.config.RPC.Degradation.LatencyThreshold
		dmConfig.EnableCircuitBreaker = b.config.RPC.Degradation.EnableCircuitBreaker
		dmConfig.Level = parseDegradationLevel(b.config.RPC.Degradation.Level)

		dm, err := rpc.NewDegradationManager(dmConfig)
		if err != nil {
			return fmt.Errorf("failed to create degradation manager: %w", err)
		}
		b.rpcManager.SetDegradationManager(dm)
	}

	logger.Info("Initializing RPC services",
		logger.String("protocol", b.config.RPC.Protocol))

	if err := b.rpcManager.Start(); err != nil {
		return fmt.Errorf("failed to start RPC manager: %w", err)
	}

	if b.discoveryManager != nil {
		if err := b.rpcManager.SetDiscoveryIntegration(b.discoveryManager, rpc.DiscoveryRegistrationConfig{
			Enabled:        b.config.Discovery.Enabled,
			ServiceName:    b.config.App.Name,
			ServiceAddress: firstNonEmpty(b.config.Discovery.Config["service_address"], firstNonEmpty(b.config.Server.GRPC.Host, b.config.Server.HTTP.Host)),
			ServiceTags:    append([]string{"rpc", b.config.RPC.Protocol}, splitAndTrim(b.config.Discovery.Config["rpc_tags"])...),
			Metadata:       map[string]string(b.config.Discovery.Config),
		}); err != nil {
			return fmt.Errorf("failed to connect rpc manager with discovery manager: %w", err)
		}
	}

	logger.Info("RPC services initialized successfully")
	return nil
}

// InitializeDiscovery initializes service discovery
func (b *Bootstrap) InitializeDiscovery() error {
	if !b.config.Discovery.Enabled {
		logger.Info("Service discovery disabled, skipping discovery initialization")
		return nil
	}

	if b.config.Discovery.Address == "" {
		logger.Warn("Service discovery enabled but no registry address configured, skipping discovery initialization")
		return nil
	}

	managerConfig := discovery.DefaultManagerConfig()
	managerConfig.Enabled = b.config.Discovery.Enabled
	managerConfig.RegistryType = b.config.Discovery.Type
	managerConfig.RegistryAddress = b.config.Discovery.Address
	managerConfig.Timeout = b.config.Discovery.Timeout
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
			Enabled:        b.config.Discovery.Enabled,
			ServiceName:    b.config.App.Name,
			ServiceAddress: firstNonEmpty(b.config.Discovery.Config["service_address"], firstNonEmpty(b.config.Server.GRPC.Host, b.config.Server.HTTP.Host)),
			ServiceTags:    append([]string{"rpc", b.config.RPC.Protocol}, splitAndTrim(b.config.Discovery.Config["rpc_tags"])...),
			Metadata:       map[string]string(b.config.Discovery.Config),
		}); err != nil {
			return fmt.Errorf("failed to connect discovery manager with rpc manager: %w", err)
		}
	}

	logger.Info("Service discovery initialized successfully")
	return nil
}
