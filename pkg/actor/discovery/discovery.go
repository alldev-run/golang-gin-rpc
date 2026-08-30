// Package discovery lets Actors participate in service registration and discovery.
package discovery

import (
	"context"
	"fmt"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/actor"
	sd "github.com/alldev-run/golang-gin-rpc/pkg/discovery"
)

// ServiceInstances is delivered to an Actor containing discovered service nodes.
type ServiceInstances struct {
	Service   string
	Instances []*sd.ServiceInstance
	Error     string
}

// Error is delivered when a discovery operation fails.
type Error struct {
	Op  string
	Err string
}

func (e Error) Error() string {
	return fmt.Sprintf("discovery %s failed: %s", e.Op, e.Err)
}

// Registry bridges an actor system with a service discovery backend.
type Registry struct {
	system    *actor.System
	discovery sd.Discovery
}

// NewRegistry creates an actor-aware service registry.
func NewRegistry(system *actor.System, discovery sd.Discovery) *Registry {
	return &Registry{
		system:    system,
		discovery: discovery,
	}
}

// Register registers a service instance with the discovery backend.
func (r *Registry) Register(ctx context.Context, instance *sd.ServiceInstance) error {
	return r.discovery.Register(ctx, instance)
}

// Deregister removes a service instance from the discovery backend.
func (r *Registry) Deregister(ctx context.Context, instance *sd.ServiceInstance) error {
	return r.discovery.Deregister(ctx, instance)
}

// Lookup returns service instances directly.
func (r *Registry) Lookup(ctx context.Context, serviceName string) ([]*sd.ServiceInstance, error) {
	return r.discovery.GetService(ctx, serviceName)
}

// Discover finds instances for serviceName and sends them as *ServiceInstances to the target Actor.
// 返回值是 discovery 后端查询本身发生的错误；消息投递错误也会一并返回。
// 即使查询失败，也会向 target 投递一条带 Error 字段的 *ServiceInstances，
// 以便 Actor 在消息处理侧感知到失败。
func (r *Registry) Discover(ctx context.Context, serviceName string, target actor.PID) error {
	instances, err := r.discovery.GetService(ctx, serviceName)
	result := &ServiceInstances{Service: serviceName}
	if err != nil {
		result.Error = err.Error()
	} else {
		result.Instances = instances
	}
	if sendErr := r.system.Send(target, result); sendErr != nil {
		// 投递失败时优先返回投递错误，并附带原始查询错误（若有）。
		if err != nil {
			return fmt.Errorf("discover %q failed: %w (and deliver failed: %v)", serviceName, err, sendErr)
		}
		return fmt.Errorf("discover %q deliver failed: %w", serviceName, sendErr)
	}
	return err
}

// RegisterMsg tells a RegistryActor to register an instance.
type RegisterMsg struct {
	Instance *sd.ServiceInstance
}

// DeregisterMsg tells a RegistryActor to deregister an instance.
type DeregisterMsg struct {
	Instance *sd.ServiceInstance
}

// AckMsg is delivered to the sender of RegisterMsg/DeregisterMsg when the
// operation completes (success or failure). On failure, Err is non-empty.
type AckMsg struct {
	Op  string
	Err string
}

// DiscoverMsg tells a RegistryActor to discover a service.
type DiscoverMsg struct {
	Service string
	ReplyTo actor.PID
}

// RegistryActor is an Actor that wraps a Registry and handles discovery messages.
type RegistryActor struct {
	registry *Registry
}

// NewRegistryActor creates a new RegistryActor.
func NewRegistryActor(registry *Registry) *RegistryActor {
	return &RegistryActor{registry: registry}
}

// Receive implements actor.Actor.
func (r *RegistryActor) Receive(ctx actor.Context, msg actor.Message) {
	switch m := msg.(type) {
	case *RegisterMsg:
		r.handle(ctx, "register", func(ctx context.Context) error {
			return r.registry.Register(ctx, m.Instance)
		})
	case *DeregisterMsg:
		r.handle(ctx, "deregister", func(ctx context.Context) error {
			return r.registry.Deregister(ctx, m.Instance)
		})
	case *DiscoverMsg:
		dctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		instances, err := r.registry.Lookup(dctx, m.Service)
		if err != nil {
			_ = ctx.Send(m.ReplyTo, &ServiceInstances{Service: m.Service, Error: err.Error()})
			return
		}
		_ = ctx.Send(m.ReplyTo, &ServiceInstances{Service: m.Service, Instances: instances})
	}
}

func (r *RegistryActor) handle(ctx actor.Context, op string, fn func(context.Context) error) {
	hctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ack := &AckMsg{Op: op}
	if err := fn(hctx); err != nil {
		ack.Err = err.Error()
		// 同时投递结构化 Error，保持向后兼容。
		_ = ctx.Send(ctx.Sender(), &Error{Op: op, Err: err.Error()})
	}
	_ = ctx.Send(ctx.Sender(), ack)
}

// Start spawns a RegistryActor in the provided system using the provided discovery backend.
func Start(system *actor.System, discovery sd.Discovery, name string) (actor.PID, error) {
	registry := NewRegistry(system, discovery)
	return system.SpawnNamed(name, NewRegistryActor(registry))
}
