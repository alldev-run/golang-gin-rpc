# Redis Client

高性能 Redis 客户端，支持多种部署模式和业务分片。

## 功能特性

- **多模式支持**: 单实例、集群、哨兵、主从、多实例分片
- **自动路由**: 多实例模式下按 key 前缀或哈希自动路由
- **完整操作**: 字符串、Hash、List、Set、分布式锁、Pub/Sub
- **连接池**: 内置连接池管理，支持配置化
- **类型安全**: 基于 go-redis/v9 封装

## 安装

```go
import "github.com/alldev-run/golang-gin-rpc/pkg/cache/redis"
```

## 快速开始

### 单实例模式

```go
cfg := redis.DefaultConfig()
cfg.Host = "localhost"
cfg.Port = 6379

client, err := redis.New(cfg)
if err != nil {
    log.Fatal(err)
}
defer client.Close()

// 基本操作
ctx := context.Background()
client.Set(ctx, "key", "value", 5*time.Minute)
val, _ := client.Get(ctx, "key")
```

## 部署模式

### 1. 单实例模式 (ModeSingle)

```go
cfg := redis.DefaultConfig()
cfg.Host = "redis.local"
cfg.Port = 6379
cfg.Password = "secret"
cfg.Database = 0

client, err := redis.New(cfg)
```

### 2. 集群模式 (ModeCluster)

```go
cfg := redis.ClusterConfig()
cfg.Nodes = []redis.NodeConfig{
    {Host: "redis1.local", Port: 6379},
    {Host: "redis2.local", Port: 6379},
    {Host: "redis3.local", Port: 6379},
}
cfg.Cluster.MaxRedirects = 3

client, err := redis.New(cfg)
```

### 3. 哨兵模式 (ModeSentinel)

```go
cfg := redis.SentinelConfig()
cfg.Sentinel.MasterName = "mymaster"
cfg.Sentinel.SentinelAddrs = []string{
    "sentinel1:26379",
    "sentinel2:26379",
    "sentinel3:26379",
}

client, err := redis.New(cfg)
```

### 4. 主从模式 (ModeMasterSlave)

```go
cfg := redis.MasterSlaveConfig()
cfg.Nodes = []redis.NodeConfig{
    {Host: "master.local", Port: 6379, IsMaster: true},
    {Host: "slave1.local", Port: 6379, IsMaster: false},
    {Host: "slave2.local", Port: 6379, IsMaster: false},
}

client, err := redis.New(cfg)
```

### 5. 多实例分片模式 (ModeMulti)

适用于不同业务使用不同 Redis 实例的场景：

```go
cfg := redis.MultiConfig()
cfg.Nodes = []redis.NodeConfig{
    {Host: "redis-users.local", Port: 6379},   // 实例0: 用户数据
    {Host: "redis-orders.local", Port: 6379},  // 实例1: 订单数据
    {Host: "redis-cache.local", Port: 6379},   // 实例2: 缓存数据
}

// 配置前缀路由
cfg.Multi.ShardingStrategy = "prefix"
cfg.Multi.KeyPrefixRoutes = map[string]int{
    "user:":    0,  // user:* -> 实例0
    "session:": 0,  // session:* -> 实例0
    "order:":   1,  // order:* -> 实例1
    "payment:": 1,  // payment:* -> 实例1
    "cache:":   2,  // cache:* -> 实例2
}
cfg.Multi.DefaultNode = 0

client, err := redis.New(cfg)
```

## 数据操作

### 字符串操作

```go
// 设置
c.Set(ctx, "key", "value", 5*time.Minute)
c.Set(ctx, "counter", 100, redis.KeepTTL)

// 获取
val, err := c.Get(ctx, "key")

// 不存在时设置
ok, err := c.SetNX(ctx, "lock", "1", 30*time.Second)

// 删除
c.Del(ctx, "key1", "key2")

// 检查存在
cnt, _ := c.Exists(ctx, "key1")

// 设置过期时间
c.Expire(ctx, "key", 10*time.Minute)

// 查看剩余时间
ttl, _ := c.TTL(ctx, "key")
```

### Hash 操作

```go
// 设置字段
c.HSet(ctx, "user:1", "name", "张三", "age", 25)

// 获取字段
name, _ := c.HGet(ctx, "user:1", "name")

// 获取所有字段
fields, _ := c.HGetAll(ctx, "user:1")
// fields["name"] == "张三"

// 删除字段
c.HDel(ctx, "user:1", "age")
```

### List 操作

```go
// 左/右推入
c.LPush(ctx, "queue", "item1", "item2")
c.RPush(ctx, "queue", "item3")

// 左/右弹出
item, _ := c.LPop(ctx, "queue")
item, _ := c.RPop(ctx, "queue")

// 获取范围
items, _ := c.LRange(ctx, "queue", 0, 9)

// 获取长度
len, _ := c.LLen(ctx, "queue")
```

### Set 操作

```go
// 添加成员
c.SAdd(ctx, "tags", "go", "redis", "cache")

// 移除成员
c.SRem(ctx, "tags", "cache")

// 检查成员
isMember, _ := c.SIsMember(ctx, "tags", "go")

// 获取所有成员
members, _ := c.SMembers(ctx, "tags")
```

## 高级功能

### 分布式锁

```go
// 获取锁
locked, err := c.Lock(ctx, "resource:lock", 30*time.Second)
if locked {
    defer c.Unlock(ctx, "resource:lock")
    // 执行业务逻辑
}
```

### Pub/Sub

```go
// 发布
c.Publish(ctx, "channel", "message")

// 订阅
pubsub := c.Subscribe(ctx, "channel")
defer pubsub.Close()

msg, err := pubsub.ReceiveMessage(ctx)
```

### Pipeline 批量操作

```go
pipe := c.Pipeline()
pipe.Set(ctx, "k1", "v1", 0)
pipe.Set(ctx, "k2", "v2", 0)
pipe.Get(ctx, "k1")
cmders, err := pipe.Exec(ctx)
```

### Lua 脚本

```go
// 执行 Lua 脚本（原子性操作）
script := `
    local current = redis.call('GET', KEYS[1])
    if tonumber(current) >= tonumber(ARGV[1]) then
        redis.call('DECRBY', KEYS[1], ARGV[1])
        return 1
    end
    return 0
`
result, err := c.Eval(ctx, script, []string{"stock:1001"}, "1")

// 先加载脚本获取 SHA1，后续用 SHA1 执行（更高效）
sha1, _ := c.ScriptLoad(ctx, script, "stock:1001")
result, err = c.EvalSha(ctx, sha1, []string{"stock:1001"}, "1")

// 检查脚本是否存在
exists, _ := c.ScriptExists(ctx, "stock:1001", sha1)
```

### Watch 乐观锁事务

```go
// 使用 Watch 实现乐观锁，防止并发冲突
err := c.Watch(ctx, "balance:1001", func(tx *redis.Tx) error {
    // 在事务中获取当前值
    balance, err := tx.Get(ctx, "balance:1001").Int()
    if err != nil {
        return err
    }
    
    if balance < 100 {
        return errors.New("insufficient balance")
    }
    
    // 执行事务：扣减余额
    _, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
        pipe.DecrBy(ctx, "balance:1001", 100)
        pipe.IncrBy(ctx, "balance:1002", 100)
        return nil
    })
    return err
})
```

## 配置参考

| 参数 | 说明 | 默认值 |
|------|------|--------|
| Host | 主机地址 | localhost |
| Port | 端口 | 6379 |
| Password | 密码 | - |
| Database | 数据库 | 0 |
| PoolSize | 连接池大小 | 10 |
| MinIdleConns | 最小空闲连接 | 2 |
| MaxRetries | 最大重试次数 | 3 |
| DialTimeout | 连接超时 | 5s |
| ReadTimeout | 读取超时 | 3s |
| WriteTimeout | 写入超时 | 3s |

## YAML 配置示例

```yaml
# configs/database.yaml
redis_single:
  redis:
    mode: single
    host: localhost
    port: 6379

redis_cluster:
  redis:
    mode: cluster
    nodes:
      - host: redis1
        port: 6379
      - host: redis2
        port: 6379
    cluster:
      max_redirects: 3

redis_multi:
  redis:
    mode: multi
    nodes:
      - host: redis-users
        port: 6379
      - host: redis-orders
        port: 6379
    multi:
      sharding_strategy: prefix
      key_prefix_routes:
        "user:": 0
        "order:": 1
```
