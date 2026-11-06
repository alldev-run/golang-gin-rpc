package zookeeper

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-zookeeper/zk"
)

// ==================== In-memory zkConn fake ====================

type fakeZKNode struct {
	children map[string]*fakeZKNode
	data     []byte
}

func newFakeZKNode() *fakeZKNode {
	return &fakeZKNode{children: make(map[string]*fakeZKNode)}
}

type fakeZKConn struct {
	mu      sync.Mutex
	root    *fakeZKNode
	errs    map[string]error // op -> injected error ("Exists", "Create", ...)
	creates []string         // recorded created paths
	deletes []string         // recorded deleted paths
	closed  bool
}

func newFakeZKConn() *fakeZKConn {
	return &fakeZKConn{
		root: newFakeZKNode(),
		errs: make(map[string]error),
	}
}

func (f *fakeZKConn) fail(op string) error {
	if err := f.errs[op]; err != nil {
		return err
	}
	return nil
}

// node walks the tree and returns the node at path (or nil).
func (f *fakeZKConn) node(path string) *fakeZKNode {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 1 && parts[0] == "" {
		return f.root
	}
	current := f.root
	for _, part := range parts {
		next, ok := current.children[part]
		if !ok {
			return nil
		}
		current = next
	}
	return current
}

// ensureNode creates all intermediate nodes for path.
func (f *fakeZKConn) ensureNode(path string) (*fakeZKNode, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	current := f.root
	for _, part := range parts {
		next, ok := current.children[part]
		if !ok {
			next = newFakeZKNode()
			current.children[part] = next
		}
		current = next
	}
	return current, nil
}

func (f *fakeZKConn) Exists(path string) (bool, *zk.Stat, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.fail("Exists"); err != nil {
		return false, nil, err
	}
	return f.node(path) != nil, &zk.Stat{}, nil
}

func (f *fakeZKConn) Create(path string, data []byte, flags int32, acl []zk.ACL) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.fail("Create"); err != nil {
		return "", err
	}
	if f.node(path) != nil {
		return "", zk.ErrNodeExists
	}
	// Unlike real ZooKeeper, missing parents are auto-created so test seeds
	// stay terse; the registry always ensures parents explicitly.
	node, err := f.ensureNode(path)
	if err != nil {
		return "", err
	}
	node.data = data
	f.creates = append(f.creates, path)
	return path, nil
}

func (f *fakeZKConn) Delete(path string, version int32) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.fail("Delete"); err != nil {
		return err
	}
	parent := f.node(parentOf(path))
	if parent == nil {
		return zk.ErrNoNode
	}
	child, ok := parent.children[baseOf(path)]
	if !ok {
		return zk.ErrNoNode
	}
	if len(child.children) > 0 {
		return zk.ErrNotEmpty
	}
	delete(parent.children, baseOf(path))
	f.deletes = append(f.deletes, path)
	return nil
}

func (f *fakeZKConn) Children(path string) ([]string, *zk.Stat, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.fail("Children"); err != nil {
		return nil, nil, err
	}
	node := f.node(path)
	if node == nil {
		return nil, nil, zk.ErrNoNode
	}
	names := make([]string, 0, len(node.children))
	for name := range node.children {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, &zk.Stat{}, nil
}

func (f *fakeZKConn) Get(path string) ([]byte, *zk.Stat, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.fail("Get"); err != nil {
		return nil, nil, err
	}
	node := f.node(path)
	if node == nil {
		return nil, nil, zk.ErrNoNode
	}
	return node.data, &zk.Stat{}, nil
}

func (f *fakeZKConn) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
}

func parentOf(path string) string {
	idx := strings.LastIndex(path, "/")
	if idx <= 0 {
		return "/"
	}
	return path[:idx]
}

func baseOf(path string) string {
	return path[strings.LastIndex(path, "/")+1:]
}

// ==================== Register ====================

func TestRegistry_RegisterCreatesEphemeralNode(t *testing.T) {
	fake := newFakeZKConn()
	registry := newRegistryWithConn(fake, "/services")

	inst := &ServiceInstance{ID: "i-1", Name: "orders", Address: "10.0.0.1", Port: 8080}
	if err := registry.Register(t.Context(), inst); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if got := registry.paths["i-1"]; got != "/services/orders/i-1" {
		t.Errorf("paths[i-1] = %q, want /services/orders/i-1", got)
	}
	if len(fake.creates) == 0 {
		t.Fatal("no nodes created")
	}
	if last := fake.creates[len(fake.creates)-1]; last != "/services/orders/i-1" {
		t.Errorf("last created = %q, want /services/orders/i-1", last)
	}

	// The instance JSON payload must be stored on the leaf node.
	data, _, err := fake.Get("/services/orders/i-1")
	if err != nil {
		t.Fatalf("fake Get() error = %v", err)
	}
	var stored ServiceInstance
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatalf("stored payload is not valid JSON: %v", err)
	}
	if stored.Address != "10.0.0.1" || stored.Port != 8080 {
		t.Errorf("stored instance = %+v, want address/port from input", stored)
	}
}

func TestRegistry_RegisterReplacesStalePath(t *testing.T) {
	fake := newFakeZKConn()
	registry := newRegistryWithConn(fake, "/services")

	if err := registry.Register(t.Context(), &ServiceInstance{ID: "i-1", Name: "orders"}); err != nil {
		t.Fatalf("first Register() error = %v", err)
	}
	// Same ID re-registered under a different service: the stale path is removed.
	if err := registry.Register(t.Context(), &ServiceInstance{ID: "i-1", Name: "billing"}); err != nil {
		t.Fatalf("second Register() error = %v", err)
	}

	if got := registry.paths["i-1"]; got != "/services/billing/i-1" {
		t.Errorf("paths[i-1] = %q, want /services/billing/i-1", got)
	}
	foundStale := false
	for _, deleted := range fake.deletes {
		if deleted == "/services/orders/i-1" {
			foundStale = true
		}
	}
	if !foundStale {
		t.Errorf("stale path not deleted; deletes = %v", fake.deletes)
	}
}

func TestRegistry_RegisterIdempotentOnExistingNode(t *testing.T) {
	fake := newFakeZKConn()
	registry := newRegistryWithConn(fake, "/services")

	inst := &ServiceInstance{ID: "i-1", Name: "orders"}
	if err := registry.Register(t.Context(), inst); err != nil {
		t.Fatalf("first Register() error = %v", err)
	}
	// Simulate a crashed session losing the in-memory tracking while the
	// znode survives in ZooKeeper: clear paths and re-register.
	registry.paths = make(map[string]string)

	if err := registry.Register(t.Context(), inst); err != nil {
		t.Fatalf("Register() over an existing node error = %v", err)
	}
	if got := registry.paths["i-1"]; got != "/services/orders/i-1" {
		t.Errorf("paths[i-1] = %q, want tracked after re-register", got)
	}
}

func TestRegistry_RegisterErrors(t *testing.T) {
	newInst := func() *ServiceInstance {
		return &ServiceInstance{ID: "i-1", Name: "orders"}
	}

	t.Run("nil instance", func(t *testing.T) {
		registry := newRegistryWithConn(newFakeZKConn(), "/services")
		if err := registry.Register(t.Context(), nil); err == nil {
			t.Error("Register(nil) should fail")
		}
	})

	t.Run("uninitialized registry", func(t *testing.T) {
		var registry *Registry
		if err := registry.Register(t.Context(), newInst()); err == nil {
			t.Error("Register on nil registry should fail")
		}
	})

	t.Run("nil conn", func(t *testing.T) {
		registry := &Registry{basePath: "/services"}
		if err := registry.Register(t.Context(), newInst()); err == nil {
			t.Error("Register without conn should fail")
		}
	})

	t.Run("ensurePath Exists failure", func(t *testing.T) {
		fake := newFakeZKConn()
		fake.errs["Exists"] = errors.New("zk down")
		registry := newRegistryWithConn(fake, "/services")
		if err := registry.Register(t.Context(), newInst()); err == nil {
			t.Error("Register with Exists failure should fail")
		}
	})

	t.Run("create leaf failure", func(t *testing.T) {
		fake := newFakeZKConn()
		fake.errs["Create"] = errors.New("quota exceeded")
		registry := newRegistryWithConn(fake, "/services")
		if err := registry.Register(t.Context(), newInst()); err == nil {
			t.Error("Register with Create failure should fail")
		}
	})
}

// ==================== Deregister ====================

func TestRegistry_Deregister(t *testing.T) {
	t.Run("registered path is deleted", func(t *testing.T) {
		fake := newFakeZKConn()
		registry := newRegistryWithConn(fake, "/services")
		inst := &ServiceInstance{ID: "i-1", Name: "orders"}
		if err := registry.Register(t.Context(), inst); err != nil {
			t.Fatalf("Register() error = %v", err)
		}
		if err := registry.Deregister(t.Context(), inst); err != nil {
			t.Fatalf("Deregister() error = %v", err)
		}
		if _, exists := registry.paths["i-1"]; exists {
			t.Error("paths entry should be removed after Deregister")
		}
		if exists, _, _ := fake.Exists("/services/orders/i-1"); exists {
			t.Error("znode should be gone after Deregister")
		}
	})

	t.Run("untracked instance falls back to computed path", func(t *testing.T) {
		fake := newFakeZKConn()
		registry := newRegistryWithConn(fake, "/services")
		if _, err := fake.Create("/services/orders/i-2", nil, 0, zk.WorldACL(zk.PermAll)); err != nil {
			t.Fatalf("seeding node failed: %v", err)
		}
		if err := registry.Deregister(t.Context(), &ServiceInstance{ID: "i-2", Name: "orders"}); err != nil {
			t.Fatalf("Deregister() error = %v", err)
		}
		if exists, _, _ := fake.Exists("/services/orders/i-2"); exists {
			t.Error("computed-path znode should be gone after Deregister")
		}
	})

	t.Run("missing node tolerated", func(t *testing.T) {
		fake := newFakeZKConn()
		registry := newRegistryWithConn(fake, "/services")
		if err := registry.Deregister(t.Context(), &ServiceInstance{ID: "ghost", Name: "orders"}); err != nil {
			t.Errorf("Deregister() of absent node error = %v, want nil", err)
		}
	})

	t.Run("delete failure propagates", func(t *testing.T) {
		fake := newFakeZKConn()
		fake.errs["Delete"] = errors.New("session expired")
		registry := newRegistryWithConn(fake, "/services")
		if err := registry.Deregister(t.Context(), &ServiceInstance{ID: "i-1", Name: "orders"}); err == nil {
			t.Error("Deregister() with Delete failure should fail")
		}
	})

	t.Run("nil receiver and instance are no-ops", func(t *testing.T) {
		var registry *Registry
		if err := registry.Deregister(t.Context(), nil); err != nil {
			t.Errorf("nil registry Deregister error = %v, want nil", err)
		}
		plain := &Registry{basePath: "/services"}
		if err := plain.Deregister(t.Context(), nil); err != nil {
			t.Errorf("nil instance Deregister error = %v, want nil", err)
		}
	})
}

// ==================== GetService ====================

func seedService(t *testing.T, fake *fakeZKConn, name string, instances ...*ServiceInstance) {
	t.Helper()
	for _, inst := range instances {
		data, err := json.Marshal(inst)
		if err != nil {
			t.Fatalf("marshal instance: %v", err)
		}
		if _, err := fake.Create(fmt.Sprintf("/services/%s/%s", name, inst.ID), data, 0, zk.WorldACL(zk.PermAll)); err != nil {
			t.Fatalf("seed instance %s: %v", inst.ID, err)
		}
	}
}

func TestRegistry_GetService(t *testing.T) {
	t.Run("returns all instances", func(t *testing.T) {
		fake := newFakeZKConn()
		registry := newRegistryWithConn(fake, "/services")
		seedService(t, fake, "orders",
			&ServiceInstance{ID: "i-1", Name: "orders", Address: "10.0.0.1", Port: 8080},
			&ServiceInstance{ID: "i-2", Name: "orders", Address: "10.0.0.2", Port: 8080},
		)

		instances, err := registry.GetService(t.Context(), "orders")
		if err != nil {
			t.Fatalf("GetService() error = %v", err)
		}
		if len(instances) != 2 {
			t.Fatalf("len(instances) = %d, want 2", len(instances))
		}
		addresses := map[string]bool{}
		for _, inst := range instances {
			addresses[inst.Address] = true
		}
		if !addresses["10.0.0.1"] || !addresses["10.0.0.2"] {
			t.Errorf("instances = %v, want both endpoints", instances)
		}
	})

	t.Run("unknown service returns empty list", func(t *testing.T) {
		fake := newFakeZKConn()
		registry := newRegistryWithConn(fake, "/services")
		instances, err := registry.GetService(t.Context(), "ghost")
		if err != nil {
			t.Fatalf("GetService() error = %v", err)
		}
		if len(instances) != 0 {
			t.Errorf("len(instances) = %d, want 0 for unknown service", len(instances))
		}
	})

	t.Run("corrupt payload is skipped", func(t *testing.T) {
		fake := newFakeZKConn()
		registry := newRegistryWithConn(fake, "/services")
		seedService(t, fake, "orders", &ServiceInstance{ID: "i-1", Name: "orders"})
		if _, _, err := fake.Get("/services/orders/i-1"); err != nil {
			t.Fatalf("seed check failed: %v", err)
		}
		// Overwrite with invalid JSON.
		fake.node("/services/orders/i-1").data = []byte("{not json")

		instances, err := registry.GetService(t.Context(), "orders")
		if err != nil {
			t.Fatalf("GetService() error = %v", err)
		}
		if len(instances) != 0 {
			t.Errorf("len(instances) = %d, want 0 (corrupt payload skipped)", len(instances))
		}
	})

	t.Run("children error propagates", func(t *testing.T) {
		fake := newFakeZKConn()
		fake.errs["Children"] = errors.New("connection lost")
		registry := newRegistryWithConn(fake, "/services")
		if _, err := registry.GetService(t.Context(), "orders"); err == nil {
			t.Error("GetService() with Children failure should fail")
		}
	})

	t.Run("get failure propagates", func(t *testing.T) {
		fake := newFakeZKConn()
		seedService(t, fake, "orders", &ServiceInstance{ID: "i-1", Name: "orders"})
		fake.errs["Get"] = errors.New("too many connections")
		registry := newRegistryWithConn(fake, "/services")
		if _, err := registry.GetService(t.Context(), "orders"); err == nil {
			t.Error("GetService() with Get failure should fail")
		}
	})

	t.Run("uninitialized registry fails", func(t *testing.T) {
		var registry *Registry
		if _, err := registry.GetService(t.Context(), "orders"); err == nil {
			t.Error("GetService on nil registry should fail")
		}
		plain := &Registry{basePath: "/services"}
		if _, err := plain.GetService(t.Context(), "orders"); err == nil {
			t.Error("GetService without conn should fail")
		}
	})
}

// ==================== ensurePath / Close ====================

func TestRegistry_EnsurePathCreatesMissingLevels(t *testing.T) {
	fake := newFakeZKConn()
	registry := newRegistryWithConn(fake, "/deep/nested/root")

	if err := registry.ensurePath("/deep/nested/root/services"); err != nil {
		t.Fatalf("ensurePath() error = %v", err)
	}
	for _, path := range []string{"/deep", "/deep/nested", "/deep/nested/root", "/deep/nested/root/services"} {
		if exists, _, _ := fake.Exists(path); !exists {
			t.Errorf("path %s should exist after ensurePath", path)
		}
	}

	// Existing nodes must not be re-created (no ErrNodeExists failures).
	if err := registry.ensurePath("/deep/nested/root/services"); err != nil {
		t.Fatalf("second ensurePath() error = %v", err)
	}
}

func TestRegistry_EnsurePathCreateError(t *testing.T) {
	fake := newFakeZKConn()
	fake.errs["Create"] = errors.New("read-only server")
	registry := newRegistryWithConn(fake, "/services")
	if err := registry.ensurePath("/services/orders"); err == nil {
		t.Error("ensurePath() with Create failure should fail")
	}
}

func TestRegistry_Close(t *testing.T) {
	fake := newFakeZKConn()
	registry := newRegistryWithConn(fake, "/services")
	if err := registry.Close(); err != nil {
		t.Fatalf("Close() error = %v, want nil", err)
	}
	if !fake.closed {
		t.Error("underlying connection should be closed")
	}

	var nilRegistry *Registry
	if err := nilRegistry.Close(); err != nil {
		t.Errorf("nil registry Close error = %v, want nil", err)
	}
	plain := &Registry{basePath: "/services"}
	if err := plain.Close(); err != nil {
		t.Errorf("conn-less registry Close error = %v, want nil", err)
	}
}

// ==================== Integration (opt-in) ====================

// TestRegistry_Integration exercises the registry against a real ZooKeeper.
// Opt in with ZOOKEEPER_INTEGRATION=1 and optionally ZOOKEEPER_ADDR.
func TestRegistry_Integration(t *testing.T) {
	if os.Getenv("ZOOKEEPER_INTEGRATION") != "1" {
		t.Skip("set ZOOKEEPER_INTEGRATION=1 and ZOOKEEPER_ADDR to run ZooKeeper integration tests")
	}
	addr := os.Getenv("ZOOKEEPER_ADDR")
	if addr == "" {
		addr = "127.0.0.1:2181"
	}

	registry, err := NewRegistry(addr, 5*time.Second, map[string]interface{}{"base_path": "/goframe_it"})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	defer func() { _ = registry.Close() }()

	ctx := t.Context()
	inst := &ServiceInstance{ID: "it-1", Name: "demo", Address: "127.0.0.1", Port: 9090}
	if err := registry.Register(ctx, inst); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	instances, err := registry.GetService(ctx, "demo")
	if err != nil {
		t.Fatalf("GetService() error = %v", err)
	}
	if len(instances) != 1 || instances[0].ID != "it-1" {
		t.Fatalf("instances = %+v, want [it-1]", instances)
	}

	if err := registry.Deregister(ctx, inst); err != nil {
		t.Fatalf("Deregister() error = %v", err)
	}
	instances, err = registry.GetService(ctx, "demo")
	if err != nil {
		t.Fatalf("GetService() after Deregister error = %v", err)
	}
	if len(instances) != 0 {
		t.Errorf("instances after Deregister = %+v, want empty", instances)
	}
}
