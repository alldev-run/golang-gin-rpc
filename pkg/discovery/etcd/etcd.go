package etcd

import (
	"encoding/json"
	"golang-gin-rpc/pkg/discovery"

	"context"

	"go.etcd.io/etcd/client/v3"
)

type Registry struct {
	client  *clientv3.Client
	leaseID clientv3.LeaseID
}

func (r *Registry) Register(ctx context.Context, inst *discovery.ServiceInstance) error {
	// 1. 创建租约 (例如 10s 过期)
	grant, _ := r.client.Grant(ctx, 10)
	r.leaseID = grant.ID

	// 2. 写入 Key (服务发现通常以 /services/name/id 为格式)
	val, _ := json.Marshal(inst)
	key := "/services/" + inst.Name + "/" + inst.ID
	_, err := r.client.Put(ctx, key, string(val), clientv3.WithLease(grant.ID))

	// 3. 开启自动续约心跳
	keepAlive, _ := r.client.KeepAlive(ctx, grant.ID)
	go func() {
		for range keepAlive { /* 维持心跳循环 */
		}
	}()
	return err
}
