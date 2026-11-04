package etcd

import (
	"errors"
	"sync"
	"testing"
)

func TestRoundRobinBalancerEmptyList(t *testing.T) {
	watcher := &ServiceWatcher{}
	watcher.instances.Store([]*ServiceInstance{})
	balancer := NewRoundRobinBalancer(watcher)

	if _, err := balancer.Pick(); !errors.Is(err, ErrNoInstance) {
		t.Fatalf("Pick() error = %v, want ErrNoInstance", err)
	}
}

func TestRoundRobinBalancerCyclesThroughInstances(t *testing.T) {
	instances := []*ServiceInstance{
		{ID: "one", Address: "127.0.0.1", Port: 8080},
		{ID: "two", Address: "127.0.0.2", Port: 8081},
	}
	watcher := &ServiceWatcher{}
	watcher.instances.Store(instances)
	balancer := NewRoundRobinBalancer(watcher)

	// The atomic index starts at 1, so picks follow two, one, two, one.
	want := []string{"two", "one", "two", "one"}
	for i, expected := range want {
		got, err := balancer.Pick()
		if err != nil {
			t.Fatalf("Pick() #%d error = %v", i+1, err)
		}
		if got.ID != expected {
			t.Fatalf("Pick() #%d = %s, want %s", i+1, got.ID, expected)
		}
	}
}

func TestRoundRobinBalancerConcurrentPicks(t *testing.T) {
	instances := []*ServiceInstance{
		{ID: "one", Address: "127.0.0.1", Port: 8080},
		{ID: "two", Address: "127.0.0.2", Port: 8081},
	}
	watcher := &ServiceWatcher{}
	watcher.instances.Store(instances)
	balancer := NewRoundRobinBalancer(watcher)

	const workers = 8
	const picksPerWorker = 100

	pickCounts := make(map[string]int)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < picksPerWorker; i++ {
				instance, err := balancer.Pick()
				if err != nil {
					t.Errorf("Pick() error = %v", err)
					return
				}
				mu.Lock()
				pickCounts[instance.ID]++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	total := workers * picksPerWorker
	for _, id := range []string{"one", "two"} {
		if pickCounts[id] != total/2 {
			t.Fatalf("instance %s picked %d times, want %d", id, pickCounts[id], total/2)
		}
	}
}
