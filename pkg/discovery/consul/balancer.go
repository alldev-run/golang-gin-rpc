package consul

import (
	"fmt"
	"sync/atomic"
)

// 接口抽象：只要实现了 List()，就能用这个负载均衡器
type ListableWatcher interface {
	List() []*ServiceInstance
}

type RoundRobinBalancer struct {
	watcher ListableWatcher
	index   uint64
}

func NewRoundRobinBalancer(watcher ListableWatcher) *RoundRobinBalancer {
	return &RoundRobinBalancer{watcher: watcher}
}

func (rr *RoundRobinBalancer) Pick() (*ServiceInstance, error) {
	list := rr.watcher.List()
	if len(list) == 0 {
		return nil, fmt.Errorf("no instances available")
	}

	// 这里的 atomic 操作对 PHP 开发者很重要：
	// 它保证了即便 1000 个请求同时 Pick，索引也不会错乱（竞态安全）
	idx := atomic.AddUint64(&rr.index, 1)
	return list[(idx-1)%uint64(len(list))], nil
}
