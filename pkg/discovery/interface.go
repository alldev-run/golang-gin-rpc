package discovery

// This file is kept for backward compatibility
// All types are now defined in types.go

import "context"

// ServiceInstance represents a service node information
type ServiceInstance struct {
	ID      string            // Unique instance ID
	Name    string            // Service name (e.g., user-service)
	Address string            // IP address
	Port    int               // Port number
	Payload map[string]string // Additional metadata
}

// Discovery unified interface
type Discovery interface {
	// Register registers a service
	Register(ctx context.Context, instance *ServiceInstance) error
	// Deregister unregisters a service
	Deregister(ctx context.Context, instance *ServiceInstance) error
	// GetService retrieves instance list by service name
	GetService(ctx context.Context, serviceName string) ([]*ServiceInstance, error)
}
