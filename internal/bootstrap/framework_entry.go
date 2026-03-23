package bootstrap

import "context"

// FrameworkOptions defines what bootstrap should initialize and manage.
type FrameworkOptions struct {
	InitDatabases bool
	InitCache     bool
	InitDiscovery bool
	InitTracing   bool
	InitAuth      bool
	InitMetrics   bool
	InitHealth    bool
	InitErrors    bool

	ValidateDependencyCoverage bool

	WebSocket *WebSocketServiceOptions
	Services  []string
}

// DefaultFrameworkOptions returns a production-friendly default.
func DefaultFrameworkOptions() FrameworkOptions {
	return FrameworkOptions{
		InitDatabases: true,
		InitCache:     true,
		InitDiscovery: true,
		InitTracing:   true,
		InitAuth:      true,
		InitMetrics:   true,
		InitHealth:    true,
		InitErrors:    true,
		ValidateDependencyCoverage: true,
		Services:      []string{ServiceRPC, ServiceAPIGateway},
	}
}

// FrameworkOptionsFromConfig builds framework options from loaded global config.
func (b *Bootstrap) FrameworkOptionsFromConfig() FrameworkOptions {
	if b == nil || b.config == nil {
		return DefaultFrameworkOptions()
	}
	return FrameworkOptions{
		InitDatabases:             b.config.Framework.InitDatabases,
		InitCache:                 b.config.Framework.InitCache,
		InitDiscovery:             b.config.Framework.InitDiscovery,
		InitTracing:               b.config.Framework.InitTracing,
		InitAuth:                  b.config.Framework.InitAuth,
		InitMetrics:               b.config.Framework.InitMetrics,
		InitHealth:                b.config.Framework.InitHealth,
		InitErrors:                b.config.Framework.InitErrors,
		ValidateDependencyCoverage: b.config.Framework.ValidateDependencyCoverage,
		Services:                  append([]string(nil), b.config.Framework.Services...),
	}
}

// StartFramework initializes core dependencies and starts selected managed services.
func (b *Bootstrap) StartFramework(ctx context.Context, options FrameworkOptions) error {
	if options.InitDatabases {
		if err := b.InitializeDatabases(); err != nil {
			return err
		}
	}
	if options.InitCache {
		if err := b.InitializeCache(); err != nil {
			return err
		}
	}
	if options.InitDiscovery {
		if err := b.InitializeDiscovery(); err != nil {
			return err
		}
	}
	if options.InitTracing {
		if err := b.InitializeTracing(); err != nil {
			return err
		}
	}
	if options.InitAuth {
		if err := b.InitializeAuth(); err != nil {
			return err
		}
	}
	if options.InitMetrics {
		if err := b.InitializeMetrics(); err != nil {
			return err
		}
	}
	if options.InitHealth {
		if err := b.InitializeHealth(); err != nil {
			return err
		}
	}
	if options.InitErrors {
		if err := b.InitializeErrors(); err != nil {
			return err
		}
	}
	if options.WebSocket != nil {
		if err := b.RegisterWebSocketServiceFactory(*options.WebSocket); err != nil {
			return err
		}
	}
	if len(options.Services) > 0 {
		if err := b.StartServices(ctx, options.Services...); err != nil {
			return err
		}
	}
	if options.ValidateDependencyCoverage {
		if err := b.ValidateDependencyCoverage(options); err != nil {
			return err
		}
	}
	return nil
}

// StopFramework stops selected managed services.
func (b *Bootstrap) StopFramework(ctx context.Context, services ...string) error {
	return b.StopServices(ctx, services...)
}
