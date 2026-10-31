package mongodb

import (
	"context"

	"github.com/alldev-run/golang-gin-rpc/plugins/db"
)

// init 将 MongoDB 驱动注册到 db 工厂。
// 只有 import 本包的二进制才会链接 mongo-driver —— 不用不加载。
func init() {
	db.RegisterDriver(db.TypeMongoDB, func(cfg db.Config) (db.Client, error) {
		mc, ok := cfg.MongoDB.(Config)
		if !ok {
			mc = DefaultConfig()
		}
		applyConfigDefaults(&mc)
		client, err := New(mc)
		if err != nil {
			return nil, err
		}
		return &dbClient{client: client}, nil
	})
}

// applyConfigDefaults 与移除前的 db.applyDefaults 保持一致。
func applyConfigDefaults(cfg *Config) {
	if cfg.URI == "" {
		cfg.URI = "mongodb://localhost:27017"
	}
	if cfg.Database == "" {
		cfg.Database = "default"
	}
}

// dbClient 将 mongodb.Client 适配为 db.Client。
type dbClient struct {
	client *Client
}

func (a *dbClient) Ping(ctx context.Context) error { return a.client.Ping(ctx) }
func (a *dbClient) Close() error                   { return a.client.Close(context.Background()) }
