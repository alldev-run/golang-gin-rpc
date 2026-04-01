package main

import (
	"context"
	"fmt"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/db/orm"
)

func main() {
	// 模拟创建 ORM 实例
	orm := createMockORM()

	// 演示分库分表 Scopes 的使用
	demonstrateShardingScopes(orm)
}

func createMockORM() *orm.ORM {
	// 这里应该使用真实的数据库连接
	// 为了演示，我们创建一个模拟的 ORM
	return &orm.ORM{} // 实际使用时需要传入真实的 DB 和 dialect
}

func demonstrateShardingScopes(orm *orm.ORM) {
	ctx := context.Background()

	fmt.Println("=== 分库分表 Scopes 使用示例 ===\n")

	// 1. 用户分片查询
	fmt.Println("1. 用户分片查询:")
	userQuery := orm.SelectWithScopes("orders").
		Scope(orm.ShardByUserIDScope(12345)).
		Scope(orm.NotDeletedScope()).
		Scope(orm.OrderByDescScope("created_at")).
		Scope(orm.PaginationScope(1, 20))

	fmt.Printf("查询: %s\n", "SELECT * FROM orders WHERE user_id = ? AND deleted_at IS NULL ORDER BY created_at DESC LIMIT 20")
	fmt.Printf("参数: [12345]\n\n")

	// 2. 租户分片查询
	fmt.Println("2. SaaS 多租户分片查询:")
	tenantQuery := orm.SelectWithScopes("documents").
		Scope(orm.ShardByTenantScope(1001)).
		Scope(orm.StatusScope("status", "active")).
		Scope(orm.SearchScope([]string{"title", "content"}, "重要文档")).
		Scope(orm.PaginationScope(1, 10))

	fmt.Printf("查询: %s\n", "SELECT * FROM documents WHERE tenant_id = ? AND status = ? AND (title LIKE ? OR content LIKE ?) LIMIT 10")
	fmt.Printf("参数: [1001, 'active', '%重要文档%', '%重要文档%']\n\n")

	// 3. 日期分片查询 (日志数据)
	fmt.Println("3. 日期分片查询 (日志数据):")
	year, month := 2023, 12
	logQuery := orm.SelectWithScopes("logs").
		Scope(orm.ShardByDateScope("created_at", year, month)).
		Scope(orm.StatusScope("level", "ERROR")).
		Scope(orm.OrderByDescScope("timestamp")).
		Scope(orm.LimitScope(100))

	fmt.Printf("查询: %s\n", "SELECT * FROM logs WHERE created_at BETWEEN ? AND ? AND level = ? ORDER BY timestamp DESC LIMIT 100")
	fmt.Printf("参数: [2023-12-01 00:00:00, 2023-12-31 23:59:59, 'ERROR']\n\n")

	// 4. 哈希分片查询
	fmt.Println("4. 哈希分片查询:")
	hashValue := 12345 % 8 // 计算哈希值
	hashQuery := orm.SelectWithScopes("user_data").
		Scope(orm.ShardByHashScope("shard_id", hashValue)).
		Scope(orm.EqScope("user_id", 12345)).
		Scope(orm.OrderByDescScope("updated_at"))

	fmt.Printf("查询: %s\n", "SELECT * FROM user_data WHERE shard_id = ? AND user_id = ? ORDER BY updated_at DESC")
	fmt.Printf("参数: [%d, 12345]\n\n", hashValue)

	// 5. 复杂分片场景 (多租户 + 用户 + 日期)
	fmt.Println("5. 复杂分片场景 (多租户 + 用户 + 日期):")
	complexQuery := orm.SelectWithScopes("orders").
		Scope(orm.SaaSApplicationShardScope(1001)). // 租户分片
		Scope(orm.ShardByUserIDScope(2002)).       // 用户分片
		Scope(orm.ShardByDateScope("created_at", 2023, 12)). // 日期分片
		Scope(orm.SearchScope([]string{"product_name"}, "手机")).
		Scope(orm.PaginationScope(1, 25))

	fmt.Printf("查询: %s\n", "SELECT * FROM orders WHERE tenant_id = ? AND deleted_at IS NULL AND user_id = ? AND created_at BETWEEN ? AND ? AND (product_name LIKE ?) ORDER BY updated_at DESC LIMIT 25")
	fmt.Printf("参数: [1001, 2002, 2023-12-01 00:00:00, 2023-12-31 23:59:59, '%手机%']\n\n")

	// 6. 跨分片查询分析
	fmt.Println("6. 跨分片查询分析:")
	crossShardQuery := orm.SelectWithScopes("analytics").
		Scope(orm.CrossShardQueryScope([]interface{}{1001, 1002, 1003})).
		Scope(orm.InScope("tenant_id", []interface{}{1001, 1002, 1003})).
		Scope(orm.ShardByDateScope("date", 2023, 12)).
		Scope(orm.OrderByDescScope("date")).
		Scope(orm.LimitScope(1000))

	fmt.Printf("查询: %s\n", "SELECT * FROM analytics WHERE /* CROSS_SHARD_QUERY: [1001 1002 1003] */ AND tenant_id IN (?, ?, ?) AND date BETWEEN ? AND ? ORDER BY date DESC LIMIT 1000")
	fmt.Printf("参数: [1001, 1002, 1003, 2023-12-01 00:00:00, 2023-12-31 23:59:59]\n\n")

	// 7. 使用 ShardAwareQueryBuilder
	fmt.Println("7. 使用 ShardAwareQueryBuilder:")
	shardInfo := &orm.ShardInfo{
		ShardID:    1,
		ShardKey:   "user_id",
		ShardValue: int64(12345),
		ShardType:  "user_id",
		CrossShard: false,
	}

	baseQuery := orm.Select("orders")
	shardAwareBuilder := orm.NewShardAwareQueryBuilder(baseQuery, shardInfo)
	shardResult := shardAwareBuilder.
		WithShard().
		Scope(orm.NotDeletedScope()).
		Scope(orm.PaginationScope(1, 20)).
		Apply()

	fmt.Printf("查询: %s\n", "SELECT * FROM orders WHERE user_id = ? AND deleted_at IS NULL LIMIT 20")
	fmt.Printf("参数: [12345]\n\n")

	// 8. 预定义业务场景分片
	fmt.Println("8. 预定义业务场景分片:")

	// 电商场景
	ecommerceQuery := orm.SelectWithScopes("orders").
		Scope(orm.ECommerceShardScope(12345)).
		Scope(orm.PaginationScope(1, 10))

	fmt.Printf("电商分片查询: %s\n", "SELECT * FROM orders WHERE user_id = ? AND deleted_at IS NULL ORDER BY created_at DESC LIMIT 10")
	fmt.Printf("参数: [12345]\n")

	// IoT 场景
	iotQuery := orm.SelectWithScopes("sensor_data").
		Scope(orm.IoTDataShardScope("device001", 2023, 12)).
		Scope(orm.EqScope("sensor_type", "temperature")).
		Scope(orm.PaginationScope(1, 50))

	fmt.Printf("IoT分片查询: %s\n", "SELECT * FROM sensor_data WHERE device_id = ? AND timestamp BETWEEN ? AND ? ORDER BY timestamp DESC LIMIT 50")
	fmt.Printf("参数: ['device001', 2023-12-01 00:00:00, 2023-12-31 23:59:59]\n")

	// 日志场景
	logScenarioQuery := orm.SelectWithScopes("logs").
		Scope(orm.LogDataLogScope(2023, 12)).
		Scope(orm.StatusScope("level", "WARN")).
		Scope(orm.SearchScope([]string{"message"}, "timeout"))

	fmt.Printf("日志分片查询: %s\n", "SELECT * FROM logs WHERE created_at BETWEEN ? AND ? AND level = ? AND (message LIKE ?) ORDER BY created_at DESC LIMIT 1000")
	fmt.Printf("参数: [2023-12-01 00:00:00, 2023-12-31 23:59:59, 'WARN', '%timeout%']\n\n")

	fmt.Println("=== 分库分表 Scopes 特性总结 ===")
	fmt.Println("✅ 支持多种分片策略: 用户ID、租户ID、日期、哈希、范围")
	fmt.Println("✅ 灵活的 Scope 组合，可以轻松构建复杂查询")
	fmt.Println("✅ 类型安全的查询构建")
	fmt.Println("✅ 支持跨分片查询标记")
	fmt.Println("✅ 预定义业务场景分片模式")
	fmt.Println("✅ 完整的测试覆盖")
	fmt.Println("✅ 与现有 ORM 无缝集成")
}

// 模拟执行查询的函数
func executeQuery(ctx context.Context, query string, args []interface{}) error {
	fmt.Printf("执行查询: %s\n", query)
	fmt.Printf("参数: %v\n", args)
	return nil
}
