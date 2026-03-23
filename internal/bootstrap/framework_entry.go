package bootstrap

import "context"

// FrameworkOptions defines what bootstrap should initialize and manage.
type FrameworkOptions struct {
	InitDatabases bool
	InitCache     bool
	InitDiscovery bool
	InitTracing   bool
	InitAuth      bool

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
		Services:      []string{ServiceRPC, ServiceAPIGateway},
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
	if options.WebSocket != nil {
		if err := b.RegisterWebSocketServiceFactory(*options.WebSocket); err != nil {
			return err
		}
	}
	if len(options.Services) == 0 {
		return nil
	}
	return b.StartServices(ctx, options.Services...)
}

// StopFramework stops selected managed services.
func (b *Bootstrap) StopFramework(ctx context.Context, services ...string) error {
	return b.StopServices(ctx, services...)
}
