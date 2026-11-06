package memcache

import (
	"context"

	"github.com/alldev-run/golang-gin-rpc/plugins/db"
	"gopkg.in/yaml.v3"
)

// init 将 Memcache 客户端注册为 db 工厂驱动。
// 只有 import 本包的二进制才会链接 gomemcache —— 不用不加载。
func init() {
	db.RegisterDriver(db.TypeMemcache, func(cfg db.Config) (db.Client, error) {
		mc, ok := decodeConfig[Config](cfg.Memcache)
		if !ok {
			mc = DefaultConfig()
		}
		client, err := New(mc)
		if err != nil {
			return nil, err
		}
		return &dbClient{client: client}, nil
	})
}

// dbClient 将 memcache.Client 适配为 db.Client。
type dbClient struct {
	client *Client
}

func (a *dbClient) Ping(ctx context.Context) error { return a.client.Ping(ctx) }
func (a *dbClient) Close() error                   { return a.client.Close() }

// decodeConfig 将 db.Config 中以 any 携带的驱动配置还原为具体类型。
// 直接断言失败时（例如 YAML 加载得到 map），经由 yaml 重序列化兜底。
func decodeConfig[T any](raw any) (T, bool) {
	if cfg, ok := raw.(T); ok {
		return cfg, true
	}
	if m, ok := raw.(map[string]any); ok && m != nil {
		data, err := yaml.Marshal(m)
		if err == nil {
			var out T
			if err := yaml.Unmarshal(data, &out); err == nil {
				return out, true
			}
		}
	}
	var zero T
	return zero, false
}
