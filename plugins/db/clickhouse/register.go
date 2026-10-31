package clickhouse

import (
	"context"

	"github.com/alldev-run/golang-gin-rpc/plugins/db"
)

// init 将 ClickHouse 驱动注册到 db 工厂。
// 只有 import 本包的二进制才会链接 clickhouse-go —— 不用不加载。
func init() {
	db.RegisterDriver(db.TypeClickHouse, func(cfg db.Config) (db.Client, error) {
		cc, ok := cfg.CH.(Config)
		if !ok {
			cc = DefaultConfig()
		}
		applyConfigDefaults(&cc)
		client, err := New(cc)
		if err != nil {
			return nil, err
		}
		return &dbClient{client: client}, nil
	})
}

// applyConfigDefaults 与移除前的 db.applyDefaults 保持一致。
func applyConfigDefaults(cfg *Config) {
	if len(cfg.Hosts) == 0 {
		cfg.Hosts = []string{"localhost:9000"}
	}
	if cfg.Database == "" {
		cfg.Database = "default"
	}
}

// dbClient 将 clickhouse.Client 适配为 db.Client。
type dbClient struct {
	client *Client
}

func (a *dbClient) Ping(ctx context.Context) error { return a.client.Ping(ctx) }
func (a *dbClient) Close() error                   { return a.client.Close() }
