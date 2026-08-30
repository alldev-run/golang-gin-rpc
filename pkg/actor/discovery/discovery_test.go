package discovery

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/actor"
	sd "github.com/alldev-run/golang-gin-rpc/pkg/discovery"
	"github.com/stretchr/testify/assert"
)

type mockDiscovery struct {
	mu         sync.Mutex
	registered map[string]*sd.ServiceInstance
}

func newMockDiscovery() *mockDiscovery {
	return &mockDiscovery{
		registered: make(map[string]*sd.ServiceInstance),
	}
}

func (m *mockDiscovery) instanceKey(inst *sd.ServiceInstance) string {
	return fmt.Sprintf("%s/%s", inst.Name, inst.ID)
}

func (m *mockDiscovery) Register(ctx context.Context, instance *sd.ServiceInstance) error {
	m.mu.Lock()
	m.registered[m.instanceKey(instance)] = instance
	m.mu.Unlock()
	return nil
}

func (m *mockDiscovery) Deregister(ctx context.Context, instance *sd.ServiceInstance) error {
	m.mu.Lock()
	delete(m.registered, m.instanceKey(instance))
	m.mu.Unlock()
	return nil
}

func (m *mockDiscovery) GetService(ctx context.Context, serviceName string) ([]*sd.ServiceInstance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var out []*sd.ServiceInstance
	for _, inst := range m.registered {
		if inst.Name == serviceName {
			out = append(out, inst)
		}
	}
	return out, nil
}

type resultActor struct {
	results chan actor.Message
}

func (a *resultActor) Receive(ctx actor.Context, msg actor.Message) {
	a.results <- msg
}

func TestRegistry_RegisterAndDeregister(t *testing.T) {
	sys := actor.New()
	defer sys.Shutdown()

	mock := newMockDiscovery()
	reg := NewRegistry(sys, mock)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	inst := &sd.ServiceInstance{ID: "a", Name: "svc", Address: "127.0.0.1", Port: 8080}
	err := reg.Register(ctx, inst)
	assert.NoError(t, err)

	mock.mu.Lock()
	assert.Len(t, mock.registered, 1)
	mock.mu.Unlock()

	err = reg.Deregister(ctx, inst)
	assert.NoError(t, err)

	mock.mu.Lock()
	assert.Len(t, mock.registered, 0)
	mock.mu.Unlock()
}

func TestRegistry_Lookup(t *testing.T) {
	sys := actor.New()
	defer sys.Shutdown()

	mock := newMockDiscovery()
	reg := NewRegistry(sys, mock)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_ = reg.Register(ctx, &sd.ServiceInstance{ID: "a", Name: "svc", Address: "127.0.0.1", Port: 8080})
	_ = reg.Register(ctx, &sd.ServiceInstance{ID: "b", Name: "svc", Address: "127.0.0.1", Port: 8081})

	instances, err := reg.Lookup(ctx, "svc")
	assert.NoError(t, err)
	assert.Len(t, instances, 2)
}

func TestRegistry_Discover(t *testing.T) {
	sys := actor.New()
	defer sys.Shutdown()

	res := &resultActor{results: make(chan actor.Message, 1)}
	pid, err := sys.SpawnNamed("receiver", res)
	assert.NoError(t, err)

	mock := newMockDiscovery()
	reg := NewRegistry(sys, mock)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_ = reg.Register(ctx, &sd.ServiceInstance{ID: "a", Name: "svc", Address: "127.0.0.1", Port: 8080})

	err = reg.Discover(ctx, "svc", pid)
	assert.NoError(t, err)

	select {
	case msg := <-res.results:
		svcs, ok := msg.(*ServiceInstances)
		assert.True(t, ok)
		assert.Equal(t, "svc", svcs.Service)
		assert.Len(t, svcs.Instances, 1)
		assert.Empty(t, svcs.Error)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for discovery result")
	}
}

func TestRegistryActor(t *testing.T) {
	sys := actor.New()
	defer sys.Shutdown()

	mock := newMockDiscovery()
	reg := NewRegistry(sys, mock)
	pid, err := sys.SpawnNamed("registry", NewRegistryActor(reg))
	assert.NoError(t, err)

	res := &resultActor{results: make(chan actor.Message, 1)}
	reply, err := sys.SpawnNamed("receiver", res)
	assert.NoError(t, err)

	err = sys.Send(pid, &RegisterMsg{Instance: &sd.ServiceInstance{ID: "a", Name: "svc", Address: "127.0.0.1", Port: 8080}})
	assert.NoError(t, err)

	err = sys.Send(pid, &DiscoverMsg{Service: "svc", ReplyTo: reply})
	assert.NoError(t, err)

	select {
	case msg := <-res.results:
		svcs, ok := msg.(*ServiceInstances)
		assert.True(t, ok)
		assert.Equal(t, "svc", svcs.Service)
		assert.Len(t, svcs.Instances, 1)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for registry actor result")
	}
}
