package elasticsearch

import (
	"context"

	"github.com/alldev-run/golang-gin-rpc/plugins/db"
)

// init 将 Elasticsearch 驱动注册到 db 工厂。
// 只有 import 本包的二进制才会链接 go-elasticsearch —— 不用不加载。
func init() {
	db.RegisterDriver(db.TypeES, func(cfg db.Config) (db.Client, error) {
		ec, ok := cfg.ES.(Config)
		if !ok {
			ec = DefaultConfig()
		}
		applyConfigDefaults(&ec)
		client, err := New(ec)
		if err != nil {
			return nil, err
		}
		return &dbClient{client: client}, nil
	})
}

// applyConfigDefaults 与移除前的 db.applyDefaults 保持一致。
func applyConfigDefaults(cfg *Config) {
	if len(cfg.Addresses) == 0 {
		cfg.Addresses = []string{"http://localhost:9200"}
	}
}

// dbClient 将 elasticsearch.Client 适配为 db.Client。
type dbClient struct {
	client *Client
}

func (a *dbClient) Ping(ctx context.Context) error {
	_, err := a.client.Info(ctx)
	return err
}

func (a *dbClient) Close() error {
	// ES 客户端无需显式关闭
	return nil
}
