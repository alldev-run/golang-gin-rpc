package db

import (
	"sync"

	"github.com/alldev-run/golang-gin-rpc/plugins/cache/memcache"
	"github.com/alldev-run/golang-gin-rpc/plugins/cache/redis"
	"github.com/alldev-run/golang-gin-rpc/plugins/db/mysql"
	"github.com/alldev-run/golang-gin-rpc/plugins/db/postgres"
)

// 驱动注册表：各驱动子包在 init() 中调用 RegisterDriver 自注册。
// 未被 import 的驱动不参与链接 —— 应用按需引入 plugins/db/<driver>。
//
// 内置轻量驱动（mysql/postgres/redis/memcache）由本包直接注册
// （Config 结构持有其配置类型）；重型驱动（clickhouse/mongodb/elasticsearch）
// 在各自子包注册，Config 以 any 携带其配置。

// DriverCreator 根据工厂配置创建对应的 Client（不做缓存，缓存由 Factory 负责）。
type DriverCreator func(cfg Config) (Client, error)

var (
	driverMu sync.RWMutex
	drivers  = make(map[Type]DriverCreator)
)

// RegisterDriver 注册一个数据库驱动。重复注册同一类型会 panic（注册发生在 init 期）。
func RegisterDriver(t Type, creator DriverCreator) {
	driverMu.Lock()
	defer driverMu.Unlock()
	if _, dup := drivers[t]; dup {
		panic("db: driver already registered: " + string(t))
	}
	drivers[t] = creator
}

// DriverLinked 返回指定驱动是否已链接进当前二进制。
func DriverLinked(t Type) bool {
	driverMu.RLock()
	defer driverMu.RUnlock()
	_, ok := drivers[t]
	return ok
}

// LinkedDrivers 返回当前已注册的驱动类型列表（按注册顺序）。
func LinkedDrivers() []Type {
	driverMu.RLock()
	defer driverMu.RUnlock()
	out := make([]Type, 0, len(drivers))
	for t := range drivers {
		out = append(out, t)
	}
	return out
}

func lookupDriver(t Type) (DriverCreator, bool) {
	driverMu.RLock()
	defer driverMu.RUnlock()
	c, ok := drivers[t]
	return c, ok
}

func init() {
	// 内置轻量驱动。
	RegisterDriver(TypeMySQL, func(cfg Config) (Client, error) {
		client, err := mysql.New(cfg.MySQL)
		if err != nil {
			return nil, err
		}
		return &mysqlAdapter{client: client}, nil
	})
	RegisterDriver(TypeRedis, func(cfg Config) (Client, error) {
		client, err := redis.New(cfg.Redis)
		if err != nil {
			return nil, err
		}
		return &redisAdapter{client: client}, nil
	})
	RegisterDriver(TypePostgres, func(cfg Config) (Client, error) {
		client, err := postgres.New(cfg.PG)
		if err != nil {
			return nil, err
		}
		return &postgresAdapter{client: client}, nil
	})
	RegisterDriver(TypeMemcache, func(cfg Config) (Client, error) {
		client, err := memcache.New(cfg.Memcache)
		if err != nil {
			return nil, err
		}
		return &memcacheAdapter{client: client}, nil
	})
}
