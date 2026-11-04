package etcd

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

// fakeWatcher implements clientv3.Watcher, exposing a channel the test controls.
type fakeWatcher struct {
	ch clientv3.WatchChan
}

func (f *fakeWatcher) Watch(context.Context, string, ...clientv3.OpOption) clientv3.WatchChan {
	return f.ch
}

func (f *fakeWatcher) RequestProgress(context.Context) error { return nil }

func (f *fakeWatcher) Close() error { return nil }

// newWatcherTestClient returns a lazily dialed client with fake KV and Watcher
// implementations injected, so no etcd server is ever contacted.
func newWatcherTestClient(t *testing.T, kv clientv3.KV, watcher clientv3.Watcher) *clientv3.Client {
	t.Helper()
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:1"},
		DialTimeout: time.Second,
		Logger:      zap.NewNop(),
	})
	if err != nil {
		t.Fatalf("clientv3.New() error = %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	client.KV = kv
	client.Watcher = watcher
	return client
}

func TestServiceWatcherListReturnsNilBeforeInitialPull(t *testing.T) {
	// A zero-value watcher has never stored anything; List must not panic.
	watcher := &ServiceWatcher{}
	if got := watcher.List(); got != nil {
		t.Fatalf("List() = %v, want nil before the initial full pull", got)
	}
}

func TestInitWithFullPullLoadsAndSkipsBrokenEntries(t *testing.T) {
	kv := &fakeKV{getResp: &clientv3.GetResponse{Kvs: []*mvccpb.KeyValue{
		{Key: []byte("/services/api/api-1"), Value: serviceInstanceJSON(&ServiceInstance{
			ID: "api-1", Name: "api", Address: "127.0.0.1", Port: 8080,
		})},
		{Key: []byte("/services/api/broken"), Value: []byte("{not-json")},
	}}}
	client := newWatcherTestClient(t, kv, &fakeWatcher{})

	watcher := &ServiceWatcher{
		client:      client,
		serviceName: "api",
		ctx:         context.Background(),
	}
	if err := watcher.initWithFullPull(); err != nil {
		t.Fatalf("initWithFullPull() error = %v", err)
	}
	instances := watcher.List()
	if len(instances) != 1 || instances[0].ID != "api-1" {
		t.Fatalf("List() = %+v, want only the valid instance", instances)
	}
	if len(kv.getKeys) != 1 || kv.getKeys[0] != "/services/api/" {
		t.Fatalf("Get keys = %v, want a prefix query for /services/api/", kv.getKeys)
	}

	kv.getErr = errors.New("etcd unavailable")
	if err := watcher.initWithFullPull(); err == nil {
		t.Fatal("initWithFullPull() succeeded despite KV failure")
	}
}

func TestWatchLoopReactsToEventsAndTerminates(t *testing.T) {
	watchCh := make(chan clientv3.WatchResponse, 4)
	kv := &fakeKV{getResp: &clientv3.GetResponse{Kvs: []*mvccpb.KeyValue{
		{Key: []byte("/services/api/api-1"), Value: serviceInstanceJSON(&ServiceInstance{
			ID: "api-1", Name: "api", Address: "127.0.0.1", Port: 8080,
		})},
	}}}
	client := newWatcherTestClient(t, kv, &fakeWatcher{ch: watchCh})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watcher := &ServiceWatcher{
		client:      client,
		serviceName: "api",
		ctx:         ctx,
		cancel:      cancel,
	}

	done := make(chan struct{})
	go func() {
		watcher.watchLoop()
		close(done)
	}()

	// An error response must be skipped without tearing down the loop.
	watchCh <- clientv3.WatchResponse{Canceled: true}

	// A valid event triggers a full pull which updates the local cache.
	watchCh <- clientv3.WatchResponse{}
	deadline := time.Now().Add(2 * time.Second)
	for len(watcher.List()) != 1 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if instances := watcher.List(); len(instances) != 1 || instances[0].ID != "api-1" {
		t.Fatalf("List() after watch event = %+v, want the api-1 instance", instances)
	}

	// Closing the watch channel must terminate the loop.
	close(watchCh)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watchLoop did not exit after the watch channel closed")
	}
}

func TestWatchLoopExitsOnStop(t *testing.T) {
	watchCh := make(chan clientv3.WatchResponse)
	client := newWatcherTestClient(t, &fakeKV{}, &fakeWatcher{ch: watchCh})

	ctx, cancel := context.WithCancel(context.Background())
	watcher := &ServiceWatcher{
		client:      client,
		serviceName: "api",
		ctx:         ctx,
		cancel:      cancel,
	}

	done := make(chan struct{})
	go func() {
		watcher.watchLoop()
		close(done)
	}()

	watcher.Stop()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watchLoop did not exit after Stop()")
	}
}
