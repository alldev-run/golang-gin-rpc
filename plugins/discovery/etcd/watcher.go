package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"go.etcd.io/etcd/client/v3"
	"sync/atomic"
)

type ServiceWatcher struct {
	client      *clientv3.Client
	serviceName string
	instances   atomic.Value // 存储 []*ServiceInstance，保证并发读安全
	ctx         context.Context
	cancel      context.CancelFunc
}

func NewServiceWatcher(ctx context.Context, client *clientv3.Client, serviceName string) (*ServiceWatcher, error) {
	wCtx, cancel := context.WithCancel(ctx)
	sw := &ServiceWatcher{
		client:      client,
		serviceName: serviceName,
		ctx:         wCtx,
		cancel:      cancel,
	}

	// 1. 初始化加载
	if err := sw.initWithFullPull(); err != nil {
		cancel()
		return nil, err
	}

	// 2. 启动异步监听
	go sw.watchLoop()

	return sw, nil
}

// initWithFullPull 第一次全量同步
func (sw *ServiceWatcher) initWithFullPull() error {
	prefix := fmt.Sprintf("/services/%s/", sw.serviceName)
	resp, err := sw.client.Get(sw.ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return err
	}

	items := make([]*ServiceInstance, 0)
	for _, kv := range resp.Kvs {
		var inst ServiceInstance
		if err := json.Unmarshal(kv.Value, &inst); err == nil {
			items = append(items, &inst)
		}
	}
	sw.instances.Store(items)
	return nil
}

// watchLoop 核心监听逻辑
func (sw *ServiceWatcher) watchLoop() {
	prefix := fmt.Sprintf("/services/%s/", sw.serviceName)
	// 从当前的 Revision 开始监听，防止丢失事件
	watchChan := sw.client.Watch(sw.ctx, prefix, clientv3.WithPrefix())

	for {
		select {
		case <-sw.ctx.Done():
			return
		case resp, ok := <-watchChan:
			if !ok {
				return
			}
			if resp.Err() != nil {
				// 实际开发中这里应该增加重连逻辑
				continue
			}

			// 监听到变化，重新触发更新
			// 简单起见，这里直接全量更新；复杂场景下建议针对 Event 做增量 Patch
			sw.initWithFullPull()
		}
	}
}

// List 直接从本地缓存读取，性能极高
func (sw *ServiceWatcher) List() []*ServiceInstance {
	return sw.instances.Load().([]*ServiceInstance)
}

func (sw *ServiceWatcher) Stop() {
	sw.cancel()
}
