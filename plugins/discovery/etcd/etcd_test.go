package etcd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

// fakeKV implements clientv3.KV with canned responses and call recording.
type fakeKV struct {
	putErr   error
	putCalls int
	putKeys  []string
	putVals  []string

	getResp *clientv3.GetResponse
	getErr  error
	getKeys []string
}

func (f *fakeKV) Put(_ context.Context, key, val string, _ ...clientv3.OpOption) (*clientv3.PutResponse, error) {
	f.putCalls++
	f.putKeys = append(f.putKeys, key)
	f.putVals = append(f.putVals, val)
	if f.putErr != nil {
		return nil, f.putErr
	}
	return &clientv3.PutResponse{}, nil
}

func (f *fakeKV) Get(_ context.Context, key string, _ ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	f.getKeys = append(f.getKeys, key)
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.getResp, nil
}

func (f *fakeKV) Delete(context.Context, string, ...clientv3.OpOption) (*clientv3.DeleteResponse, error) {
	return &clientv3.DeleteResponse{}, nil
}

func (f *fakeKV) Compact(context.Context, int64, ...clientv3.CompactOption) (*clientv3.CompactResponse, error) {
	return &clientv3.CompactResponse{}, nil
}

func (f *fakeKV) Do(context.Context, clientv3.Op) (clientv3.OpResponse, error) {
	return clientv3.OpResponse{}, nil
}

func (f *fakeKV) Txn(context.Context) clientv3.Txn {
	return nil // never exercised by the registry
}

// fakeLease implements clientv3.Lease with canned responses and call recording.
type fakeLease struct {
	nextID       clientv3.LeaseID
	grantErr     error
	revokeErr    error
	keepAliveErr error
	keepAliveCh  chan *clientv3.LeaseKeepAliveResponse

	revokeCalls int
	revokedIDs  []clientv3.LeaseID
}

func (f *fakeLease) Grant(context.Context, int64) (*clientv3.LeaseGrantResponse, error) {
	if f.grantErr != nil {
		return nil, f.grantErr
	}
	f.nextID++
	return &clientv3.LeaseGrantResponse{ID: f.nextID}, nil
}

func (f *fakeLease) Revoke(_ context.Context, id clientv3.LeaseID) (*clientv3.LeaseRevokeResponse, error) {
	f.revokeCalls++
	f.revokedIDs = append(f.revokedIDs, id)
	if f.revokeErr != nil {
		return nil, f.revokeErr
	}
	return &clientv3.LeaseRevokeResponse{}, nil
}

func (f *fakeLease) TimeToLive(context.Context, clientv3.LeaseID, ...clientv3.LeaseOption) (*clientv3.LeaseTimeToLiveResponse, error) {
	return &clientv3.LeaseTimeToLiveResponse{}, nil
}

func (f *fakeLease) Leases(context.Context) (*clientv3.LeaseLeasesResponse, error) {
	return &clientv3.LeaseLeasesResponse{}, nil
}

func (f *fakeLease) KeepAlive(context.Context, clientv3.LeaseID) (<-chan *clientv3.LeaseKeepAliveResponse, error) {
	if f.keepAliveErr != nil {
		return nil, f.keepAliveErr
	}
	if f.keepAliveCh != nil {
		return f.keepAliveCh, nil
	}
	return make(chan *clientv3.LeaseKeepAliveResponse), nil
}

func (f *fakeLease) KeepAliveOnce(context.Context, clientv3.LeaseID) (*clientv3.LeaseKeepAliveResponse, error) {
	return &clientv3.LeaseKeepAliveResponse{}, nil
}

func (f *fakeLease) Close() error { return nil }

// newTestRegistry builds a Registry around a lazily dialed clientv3 client with
// the KV and Lease interfaces replaced by fakes; no etcd server is contacted.
func newTestRegistry(t *testing.T, kv clientv3.KV, lease clientv3.Lease) *Registry {
	t.Helper()
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:1"}, // never dialed: gRPC connects lazily
		DialTimeout: time.Second,
		Logger:      zap.NewNop(),
	})
	if err != nil {
		t.Fatalf("clientv3.New() error = %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if kv != nil {
		client.KV = kv
	}
	if lease != nil {
		client.Lease = lease
	}
	return &Registry{
		client:  client,
		leases:  make(map[string]clientv3.LeaseID),
		cancels: make(map[string]context.CancelFunc),
	}
}

func serviceInstanceJSON(instance *ServiceInstance) []byte {
	data, err := json.Marshal(instance)
	if err != nil {
		panic(fmt.Sprintf("marshal test instance: %v", err))
	}
	return data
}

func TestNewRegistryCreatesClient(t *testing.T) {
	registry, err := NewRegistry("127.0.0.1:1", time.Second)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	t.Cleanup(func() { _ = registry.Close() })
	if registry.client == nil || registry.leases == nil || registry.cancels == nil {
		t.Fatalf("NewRegistry() = %+v, want initialized client and maps", registry)
	}
}

func TestRegisterStoresLeaseAndKey(t *testing.T) {
	lease := &fakeLease{}
	kv := &fakeKV{}
	registry := newTestRegistry(t, kv, lease)
	ctx := context.Background()

	instance := &ServiceInstance{ID: "api-1", Name: "api", Address: "127.0.0.1", Port: 8080}
	if err := registry.Register(ctx, instance); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	key := "/services/api/api-1"
	if kv.putCalls != 1 || kv.putKeys[0] != key {
		t.Fatalf("Put calls/keys = %d %v, want 1 [%s]", kv.putCalls, kv.putKeys, key)
	}
	var stored ServiceInstance
	if err := json.Unmarshal([]byte(kv.putVals[0]), &stored); err != nil {
		t.Fatalf("stored value is not valid JSON: %v", err)
	}
	if stored.ID != instance.ID || stored.Address != instance.Address || stored.Port != instance.Port {
		t.Fatalf("stored value = %+v, want the registered instance", stored)
	}
	if registry.leases[key] == 0 {
		t.Fatal("Register() did not record the lease ID")
	}
	if registry.cancels[key] == nil {
		t.Fatal("Register() did not record the keepalive cancel func")
	}

	// Deregister revokes the lease and clears both maps.
	leaseID := registry.leases[key]
	if err := registry.Deregister(ctx, instance); err != nil {
		t.Fatalf("Deregister() error = %v", err)
	}
	if lease.revokeCalls != 1 || lease.revokedIDs[0] != leaseID {
		t.Fatalf("Deregister() revoked %v, want lease %d", lease.revokedIDs, leaseID)
	}
	if _, ok := registry.leases[key]; ok {
		t.Fatal("Deregister() left the lease entry behind")
	}
	if _, ok := registry.cancels[key]; ok {
		t.Fatal("Deregister() left the cancel entry behind")
	}
}

func TestRegisterReplacesExistingLease(t *testing.T) {
	lease := &fakeLease{}
	kv := &fakeKV{}
	registry := newTestRegistry(t, kv, lease)
	ctx := context.Background()

	instance := &ServiceInstance{ID: "api-1", Name: "api", Address: "127.0.0.1", Port: 8080}
	if err := registry.Register(ctx, instance); err != nil {
		t.Fatalf("first Register() error = %v", err)
	}
	firstLease := registry.leases["/services/api/api-1"]
	if err := registry.Register(ctx, instance); err != nil {
		t.Fatalf("second Register() error = %v", err)
	}

	if lease.revokeCalls != 1 || lease.revokedIDs[0] != firstLease {
		t.Fatalf("revoked = %v, want the stale lease %d", lease.revokedIDs, firstLease)
	}
	if kv.putCalls != 2 {
		t.Fatalf("Put calls = %d, want 2", kv.putCalls)
	}
	if current := registry.leases["/services/api/api-1"]; current == 0 || current == firstLease {
		t.Fatalf("lease after re-register = %d, want a fresh lease", current)
	}
}

func TestRegisterErrorPaths(t *testing.T) {
	instance := &ServiceInstance{ID: "api-1", Name: "api", Address: "127.0.0.1", Port: 8080}

	t.Run("grant fails", func(t *testing.T) {
		kv := &fakeKV{}
		lease := &fakeLease{grantErr: errors.New("no leader")}
		registry := newTestRegistry(t, kv, lease)
		err := registry.Register(context.Background(), instance)
		if err == nil || !contains(err.Error(), "etcd grant failed") {
			t.Fatalf("Register() error = %v, want grant failure", err)
		}
		if kv.putCalls != 0 || len(registry.leases) != 0 {
			t.Fatal("grant failure must not put data or record leases")
		}
	})

	t.Run("put fails", func(t *testing.T) {
		kv := &fakeKV{putErr: errors.New("disk full")}
		lease := &fakeLease{}
		registry := newTestRegistry(t, kv, lease)
		err := registry.Register(context.Background(), instance)
		if err == nil || !contains(err.Error(), "etcd put failed") {
			t.Fatalf("Register() error = %v, want put failure", err)
		}
		if len(registry.leases) != 0 || len(registry.cancels) != 0 {
			t.Fatal("put failure must not record lease state")
		}
	})

	t.Run("keepalive fails", func(t *testing.T) {
		kv := &fakeKV{}
		lease := &fakeLease{keepAliveErr: errors.New("stream closed")}
		registry := newTestRegistry(t, kv, lease)
		err := registry.Register(context.Background(), instance)
		if err == nil || !contains(err.Error(), "etcd keepalive failed") {
			t.Fatalf("Register() error = %v, want keepalive failure", err)
		}
		if len(registry.leases) != 0 || len(registry.cancels) != 0 {
			t.Fatal("keepalive failure must not record lease state")
		}
	})
}

func TestDeregisterUnknownKeyIsNoOp(t *testing.T) {
	lease := &fakeLease{}
	registry := newTestRegistry(t, &fakeKV{}, lease)
	if err := registry.Deregister(context.Background(), &ServiceInstance{ID: "ghost", Name: "ghost"}); err != nil {
		t.Fatalf("Deregister() unknown key error = %v", err)
	}
	if lease.revokeCalls != 0 {
		t.Fatal("Deregister() revoked a lease for an unknown key")
	}
}

func TestDeregisterRevokeError(t *testing.T) {
	lease := &fakeLease{revokeErr: errors.New("lease expired")}
	registry := newTestRegistry(t, &fakeKV{}, lease)
	ctx := context.Background()
	instance := &ServiceInstance{ID: "api-1", Name: "api", Address: "127.0.0.1", Port: 8080}
	if err := registry.Register(ctx, instance); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := registry.Deregister(ctx, instance); err == nil {
		t.Fatal("Deregister() succeeded despite revoke failure")
	}
	// The stale lease entry must be dropped even when revoke fails.
	if _, ok := registry.leases["/services/api/api-1"]; ok {
		t.Fatal("Deregister() kept the lease entry after revoke failure")
	}
}

func TestGetService(t *testing.T) {
	ctx := context.Background()
	kv := &fakeKV{getResp: &clientv3.GetResponse{Kvs: []*mvccpb.KeyValue{
		{Key: []byte("/services/api/api-1"), Value: serviceInstanceJSON(&ServiceInstance{
			ID: "api-1", Name: "api", Address: "127.0.0.1", Port: 8080,
			Payload: map[string]string{"version": "1.0.0"},
		})},
		{Key: []byte("/services/api/broken"), Value: []byte("{not-json")},
	}}}
	registry := newTestRegistry(t, kv, &fakeLease{})

	instances, err := registry.GetService(ctx, "api")
	if err != nil {
		t.Fatalf("GetService() error = %v", err)
	}
	if len(instances) != 1 {
		t.Fatalf("GetService() returned %d instances, want 1 (broken entry skipped)", len(instances))
	}
	if instances[0].ID != "api-1" || instances[0].Payload["version"] != "1.0.0" {
		t.Fatalf("GetService() = %+v", instances[0])
	}
	if len(kv.getKeys) != 1 || kv.getKeys[0] != "/services/api/" {
		t.Fatalf("Get keys = %v, want a prefix query for /services/api/", kv.getKeys)
	}

	kv.getErr = errors.New("etcd unavailable")
	if _, err := registry.GetService(ctx, "api"); err == nil {
		t.Fatal("GetService() succeeded despite KV failure")
	}
}

func TestCloseDeregistersAllLeases(t *testing.T) {
	lease := &fakeLease{}
	registry := newTestRegistry(t, &fakeKV{}, lease)
	ctx := context.Background()

	for _, id := range []string{"api-1", "api-2"} {
		if err := registry.Register(ctx, &ServiceInstance{ID: id, Name: "api", Address: "127.0.0.1", Port: 8080}); err != nil {
			t.Fatalf("Register(%s) error = %v", id, err)
		}
	}
	if len(registry.leases) != 2 {
		t.Fatalf("leases after registrations = %v, want 2 entries", registry.leases)
	}

	if err := registry.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if lease.revokeCalls != 2 {
		t.Fatalf("revoke calls after Close() = %d, want 2", lease.revokeCalls)
	}
	if len(registry.leases) != 0 {
		t.Fatalf("leases after Close() = %v, want empty", registry.leases)
	}
}

func TestCloseReportsDeregisterFailure(t *testing.T) {
	lease := &fakeLease{revokeErr: errors.New("lease expired")}
	registry := newTestRegistry(t, &fakeKV{}, lease)
	ctx := context.Background()
	if err := registry.Register(ctx, &ServiceInstance{ID: "api-1", Name: "api", Address: "127.0.0.1", Port: 8080}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := registry.Close(); err == nil {
		t.Fatal("Close() succeeded despite revoke failure")
	}
}

func contains(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
