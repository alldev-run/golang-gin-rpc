package discovery

import (
	"context"
	"fmt"
	"sync"
)

// staticRegistry is an in-memory Discovery implementation used when the
// registry type is RegistryTypeStatic. Instances are kept in process memory
// only, so a registration takes effect immediately, which makes the static
// registry suitable for single-process deployments and for testing.
type staticRegistry struct {
	mu       sync.RWMutex
	services map[string]map[string]*ServiceInstance // service name -> instance ID -> instance
}

// newStaticRegistry creates an empty in-memory registry.
func newStaticRegistry() *staticRegistry {
	return &staticRegistry{
		services: make(map[string]map[string]*ServiceInstance),
	}
}

// Register stores an instance in memory; the change is visible to GetService
// as soon as Register returns.
func (r *staticRegistry) Register(_ context.Context, instance *ServiceInstance) error {
	if r == nil {
		return fmt.Errorf("static registry is not initialized")
	}
	if instance == nil {
		return fmt.Errorf("service instance is nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.services == nil {
		r.services = make(map[string]map[string]*ServiceInstance)
	}
	bucket := r.services[instance.Name]
	if bucket == nil {
		bucket = make(map[string]*ServiceInstance)
		r.services[instance.Name] = bucket
	}
	bucket[instance.ID] = cloneServiceInstance(instance)
	return nil
}

// Deregister removes an instance from memory; unknown instances are ignored.
func (r *staticRegistry) Deregister(_ context.Context, instance *ServiceInstance) error {
	if r == nil || instance == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	bucket := r.services[instance.Name]
	if bucket == nil {
		return nil
	}
	delete(bucket, instance.ID)
	if len(bucket) == 0 {
		delete(r.services, instance.Name)
	}
	return nil
}

// GetService returns a deep copy of the instances registered under
// serviceName; an unknown service yields an empty, non-nil slice.
func (r *staticRegistry) GetService(_ context.Context, serviceName string) ([]*ServiceInstance, error) {
	if r == nil {
		return nil, fmt.Errorf("static registry is not initialized")
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	bucket := r.services[serviceName]
	instances := make([]*ServiceInstance, 0, len(bucket))
	for _, instance := range bucket {
		instances = append(instances, cloneServiceInstance(instance))
	}
	return instances, nil
}

// cloneServiceInstance returns a deep copy so callers cannot mutate registry
// state through returned instances.
func cloneServiceInstance(instance *ServiceInstance) *ServiceInstance {
	cloned := *instance
	if instance.Payload != nil {
		cloned.Payload = make(map[string]string, len(instance.Payload))
		for key, value := range instance.Payload {
			cloned.Payload[key] = value
		}
	}
	return &cloned
}
