package etcd

import (
	"errors"
	"sync/atomic"
)

var ErrNoInstance = errors.New("no available service instances")

// Balancer 负载均衡接口
type Balancer interface {
	Pick() (*ServiceInstance, error)
}

// RoundRobinBalancer 轮询实现
type RoundRobinBalancer struct {
	watcher *ServiceWatcher
	index   uint64
}

func NewRoundRobinBalancer(watcher *ServiceWatcher) *RoundRobinBalancer {
	return &RoundRobinBalancer{
		watcher: watcher,
	}
}

func (rr *RoundRobinBalancer) Pick() (*ServiceInstance, error) {
	instances := rr.watcher.List()
	n := len(instances)
	if n == 0 {
		return nil, ErrNoInstance
	}

	// 原子递增，返回的是新值
	next := atomic.AddUint64(&rr.index, 1)

	// 直接用 uint64 取模，避免负数或溢出担忧
	idx := next % uint64(n) // next 从1开始也没关系，%n 后均匀
	return instances[idx], nil
}
