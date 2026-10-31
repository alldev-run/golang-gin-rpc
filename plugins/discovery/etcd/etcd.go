package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.etcd.io/etcd/client/v3"
)

// ServiceInstance 保持不变
type ServiceInstance struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Address string            `json:"address"`
	Port    int               `json:"port"`
	Payload map[string]string `json:"payload"`
}

type Registry struct {
	client *clientv3.Client
	// 使用 map 存储租约，Key 为 "/services/name/id"
	leases map[string]clientv3.LeaseID
	// 存储取消函数，用于手动注销时停止心跳协程
	cancels map[string]context.CancelFunc
	mu      sync.RWMutex
}

func NewRegistry(addr string, timeout time.Duration) (*Registry, error) {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{addr},
		DialTimeout: timeout,
	})
	if err != nil {
		return nil, err
	}
	return &Registry{
		client:  client,
		leases:  make(map[string]clientv3.LeaseID),
		cancels: make(map[string]context.CancelFunc),
	}, nil
}

func (r *Registry) Register(ctx context.Context, inst *ServiceInstance) error {
	key := fmt.Sprintf("/services/%s/%s", inst.Name, inst.ID)

	r.mu.Lock()
	defer r.mu.Unlock()

	// 1. 如果已存在，先注销旧的（防止重复注册导致租约泄露）
	if _, ok := r.leases[key]; ok {
		r.deregisterUnlocked(ctx, key)
	}

	// 2. 创建租约 (TTL 10s)
	grant, err := r.client.Grant(ctx, 10)
	if err != nil {
		return fmt.Errorf("etcd grant failed: %w", err)
	}

	val, _ := json.Marshal(inst)
	_, err = r.client.Put(ctx, key, string(val), clientv3.WithLease(grant.ID))
	if err != nil {
		return fmt.Errorf("etcd put failed: %w", err)
	}

	// 3. 开启心跳
	kaCtx, kaCancel := context.WithCancel(context.Background())
	keepAlive, err := r.client.KeepAlive(kaCtx, grant.ID)
	if err != nil {
		kaCancel()
		return fmt.Errorf("etcd keepalive failed: %w", err)
	}

	r.leases[key] = grant.ID
	r.cancels[key] = kaCancel

	// 4. 异步处理续约响应
	go func(k string, id clientv3.LeaseID) {
		for {
			select {
			case _, ok := <-keepAlive:
				if !ok {
					return // 通道关闭，说明租约失效或被 Revoke
				}
			case <-kaCtx.Done():
				return // 外部主动注销
			}
		}
	}(key, grant.ID)

	return nil
}

// Deregister 对外暴露的注销方法
func (r *Registry) Deregister(ctx context.Context, inst *ServiceInstance) error {
	key := fmt.Sprintf("/services/%s/%s", inst.Name, inst.ID)
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.deregisterUnlocked(ctx, key)
}

// deregisterUnlocked 内部注销逻辑（无锁版）
func (r *Registry) deregisterUnlocked(ctx context.Context, key string) error {
	if cancel, ok := r.cancels[key]; ok {
		cancel() // 停止心跳协程
		delete(r.cancels, key)
	}

	if leaseID, ok := r.leases[key]; ok {
		// 立即撤销租约，etcd 会自动删除关联的 Key
		_, err := r.client.Revoke(ctx, leaseID)
		delete(r.leases, key)
		return err
	}
	return nil
}

func (r *Registry) GetService(ctx context.Context, serviceName string) ([]*ServiceInstance, error) {
	prefix := fmt.Sprintf("/services/%s/", serviceName)
	resp, err := r.client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	instances := make([]*ServiceInstance, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var inst ServiceInstance
		if err := json.Unmarshal(kv.Value, &inst); err != nil {
			continue // 忽略错误数据
		}
		instances = append(instances, &inst)
	}
	return instances, nil
}

func (r *Registry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for key := range r.leases {
		r.deregisterUnlocked(context.Background(), key)
	}
	return r.client.Close()
}
