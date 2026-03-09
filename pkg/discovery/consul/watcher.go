package consul

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/api/watch"
)

// ServiceWatcher 服务监听器
type ServiceWatcher struct {
	instances atomic.Value // 存储 []*ServiceInstance
	plan      *watch.Plan
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewServiceWatcher 创建服务监听器
func NewServiceWatcher(ctx context.Context, addr string, serviceName string) (*ServiceWatcher, error) {
	wCtx, cancel := context.WithCancel(ctx)
	sw := &ServiceWatcher{
		ctx:    wCtx,
		cancel: cancel,
	}

	// 1. 创建 Watch 配置（监听健康检查通过的节点）
	params := map[string]interface{}{
		"type":        "service",
		"service":     serviceName,
		"passingonly": true,
	}
	plan, err := watch.Parse(params)
	if err != nil {
		cancel()
		return nil, err
	}

	// 2. 定义回调函数：当 Consul 发现节点变化时，会自动调用此函数
	plan.Handler = func(idx uint64, raw interface{}) {
		if raw == nil {
			return
		}
		// 类型断言：Consul 返回的是 []*api.ServiceEntry
		entries, ok := raw.([]*api.ServiceEntry)
		if !ok {
			return
		}

		var items []*ServiceInstance
		for _, entry := range entries {
			items = append(items, &ServiceInstance{
				ID:      entry.Service.ID,
				Name:    entry.Service.Service,
				Address: entry.Service.Address,
				Port:    entry.Service.Port,
				Payload: entry.Service.Meta,
			})
		}
		// 更新原子本地缓存
		sw.instances.Store(items)
	}

	// 3. 在独立协程中运行（类似 Swoole 的异步协程）
	go func() {
		if err := plan.Run(addr); err != nil {
			fmt.Printf("Consul watch error: %v\n", err)
		}
	}()

	sw.plan = plan
	return sw, nil
}

// List 获取当前服务列表
func (sw *ServiceWatcher) List() []*ServiceInstance {
	val := sw.instances.Load()
	if val == nil {
		return nil
	}
	return val.([]*ServiceInstance)
}

// Stop 停止监听
func (sw *ServiceWatcher) Stop() {
	sw.plan.Stop()
	sw.cancel()
}
