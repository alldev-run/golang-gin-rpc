package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/db/orm"
)

// DatabaseConfig represents database configuration
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
}

// ShardingConfig represents sharding configuration
type ShardingConfig struct {
	Strategy      string // "modulo", "range", "consistent_hash", "date_based"
	DatabaseCount int
	TableCount    int
	TablePrefix   string
	ShardKey      string
}

// DynamicShardingService demonstrates dynamic sharding in a service layer
type DynamicShardingService struct {
	dsORM *orm.DynamicShardingORM
}

// NewDynamicShardingService creates a new dynamic sharding service
func NewDynamicShardingService(config ShardingConfig) (*DynamicShardingService, error) {
	// Create sharding strategy based on configuration
	var strategy orm.ShardStrategy
	
	switch config.Strategy {
	case "modulo":
		strategy = orm.NewModuloShardStrategy(config.DatabaseCount, config.TableCount)
	case "range":
		// Example: user ID ranges for databases and tables
		dbRanges := []orm.Range{
			{Min: 0, Max: 1000000},      // First million users
			{Min: 1000000, Max: 2000000}, // Second million users
			{Min: 2000000, Max: 3000000}, // Third million users
		}
		tableRanges := []orm.Range{
			{Min: 0, Max: 100000},      // First 100k users
			{Min: 100000, Max: 200000},  // Second 100k users
			{Min: 200000, Max: 300000},  // Third 100k users
			{Min: 300000, Max: 400000},  // Fourth 100k users
		}
		strategy = orm.NewRangeShardStrategy(dbRanges, tableRanges)
	case "consistent_hash":
		dbNodes := []string{"shard_db_1", "shard_db_2", "shard_db_3", "shard_db_4"}
		tableNodes := []string{"table_1", "table_2", "table_3", "table_4", "table_5", "table_6"}
		strategy = orm.NewConsistentHashShardStrategy(dbNodes, tableNodes)
	case "date_based":
		strategy = orm.NewDateBasedShardStrategy("monthly", "daily")
	default:
		return nil, fmt.Errorf("unsupported sharding strategy: %s", config.Strategy)
	}
	
	// Create mock database connections (in practice, these would be real DB connections)
	dataSources := createMockDataSources(config.DatabaseCount)
	
	// Create sharding manager
	manager := orm.NewDynamicShardingManager(strategy, dataSources, config.TablePrefix)
	
	// Setup table patterns
	for i := 0; i < config.TableCount; i++ {
		tableName := fmt.Sprintf("%s_%d", config.TablePrefix, i)
		manager.AddTable(i, tableName)
	}
	
	// Create base ORM (mock)
	baseORM := &orm.ORM{}
	
	// Create dynamic sharding ORM
	dsORM := orm.NewDynamicShardingORM(baseORM, manager, config.TablePrefix)
	
	return &DynamicShardingService{
		dsORM: dsORM,
	}, nil
}

// createMockDataSources creates mock database connections for demonstration
func createMockDataSources(count int) []orm.DB {
	dataSources := make([]orm.DB, count)
	for i := 0; i < count; i++ {
		dataSources[i] = &MockDatabaseConnection{
			name: fmt.Sprintf("shard_db_%d", i),
			id:   i,
		}
	}
	return dataSources
}

// MockDatabaseConnection represents a mock database connection
type MockDatabaseConnection struct {
	name string
	id   int
}

func (m *MockDatabaseConnection) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	fmt.Printf("Query on %s: %s with args %v\n", m.name, query, args)
	return nil, nil
}

func (m *MockDatabaseConnection) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	fmt.Printf("QueryRow on %s: %s with args %v\n", m.name, query, args)
	return nil
}

func (m *MockDatabaseConnection) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	fmt.Printf("Exec on %s: %s with args %v\n", m.name, query, args)
	return &MockResult{}, nil
}

func (m *MockDatabaseConnection) Begin(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	fmt.Printf("Begin transaction on %s\n", m.name)
	return nil, nil
}

func (m *MockDatabaseConnection) Ping(ctx context.Context) error {
	fmt.Printf("Ping %s\n", m.name)
	return nil
}

func (m *MockDatabaseConnection) Stats() sql.DBStats {
	return sql.DBStats{}
}

func (m *MockDatabaseConnection) Close() error {
	fmt.Printf("Close connection to %s\n", m.name)
	return nil
}

// MockResult represents a mock SQL result
type MockResult struct{}

func (mr *MockResult) LastInsertId() (int64, error) {
	return 1, nil
}

func (mr *MockResult) RowsAffected() (int64, error) {
	return 1, nil
}

// Business methods using dynamic sharding

// FindUserOrders finds orders for a specific user with dynamic sharding
func (dss *DynamicShardingService) FindUserOrders(ctx context.Context, userID int64, page, pageSize int) ([]Order, error) {
	// Dynamic sharding based on user ID
	builder, err := dss.dsORM.SelectWithDynamicSharding("user_id", userID)
	if err != nil {
		return nil, fmt.Errorf("failed to create sharded query: %w", err)
	}
	
	// Apply shard condition and other filters
	query := builder.
		ShardBy("user_id").
		Where("status = ?", "active").
		Where("deleted_at IS NULL").
		OrderBy("created_at DESC").
		Limit(pageSize).
		Offset((page - 1) * pageSize)
	
	// Execute query
	rows, err := query.Query(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()
	
	// Process results (simplified for demonstration)
	var orders []Order
	fmt.Printf("Found orders for user %d using dynamic sharding\n", userID)
	
	return orders, nil
}

// FindOrdersAcrossShards finds orders across multiple shards
func (dss *DynamicShardingService) FindOrdersAcrossShards(ctx context.Context, userIDs []int64) ([]Order, error) {
	// Create cross-shard query
	shardKeys := make([]interface{}, len(userIDs))
	shardValues := make([]interface{}, len(userIDs))
	
	for i, userID := range userIDs {
		shardKeys[i] = "user_id"
		shardValues[i] = userID
	}
	
	crossBuilder, err := dss.dsORM.CrossShardQuery(shardKeys, shardValues)
	if err != nil {
		return nil, fmt.Errorf("failed to create cross-shard query: %w", err)
	}
	
	// Add conditions for each shard
	for i, userID := range userIDs {
		crossBuilder.AddShard(i, "status = 'active'", "deleted_at IS NULL")
	}
	
	// Execute cross-shard query
	result, err := crossBuilder.Execute(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to execute cross-shard query: %w", err)
	}
	
	fmt.Printf("Found %d orders across %d shards\n", result.TotalCount, len(userIDs))
	
	return []Order{}, nil
}

// InsertOrder inserts an order with dynamic sharding
func (dss *DynamicShardingService) InsertOrder(ctx context.Context, order Order) error {
	// Dynamic sharding based on user ID
	builder, err := dss.dsORM.SelectWithDynamicSharding("user_id", order.UserID)
	if err != nil {
		return fmt.Errorf("failed to create sharded insert: %w", err)
	}
	
	// Use the underlying database connection to insert
	db, tableName, err := dss.dsORM.shardingManager.GetTargetDataSource("user_id", order.UserID)
	if err != nil {
		return fmt.Errorf("failed to get target data source: %w", err)
	}
	
	// Execute insert (simplified)
	query := fmt.Sprintf("INSERT INTO %s (user_id, product_id, amount, status, created_at) VALUES (?, ?, ?, ?, ?)", tableName)
	_, err = db.Exec(ctx, query, order.UserID, order.ProductID, order.Amount, order.Status, order.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert order: %w", err)
	}
	
	fmt.Printf("Inserted order for user %d into table %s\n", order.UserID, tableName)
	return nil
}

// FindUserActivity finds user activity across time-based sharding
func (dss *DynamicShardingService) FindUserActivity(ctx context.Context, userID int64, startDate, endDate time.Time) ([]Activity, error) {
	// For time-based sharding, we might shard by date
	builder, err := dss.dsORM.SelectWithDynamicSharding("date", startDate.Format("2006-01-02"))
	if err != nil {
		return nil, fmt.Errorf("failed to create time-based sharded query: %w", err)
	}
	
	// Apply filters
	query := builder.
		Where("user_id = ?", userID).
		Where("created_at >= ?", startDate).
		Where("created_at <= ?", endDate).
		OrderBy("created_at DESC").
		Limit(100)
	
	// Execute query
	rows, err := query.Query(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to execute activity query: %w", err)
	}
	defer rows.Close()
	
	fmt.Printf("Found activity for user %d from %s to %s\n", userID, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
	
	return []Activity{}, nil
}

// Data structures

type Order struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	ProductID int64     `json:"product_id"`
	Amount    float64   `json:"amount"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Activity struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Action    string    `json:"action"`
	Details   string    `json:"details"`
	CreatedAt time.Time `json:"created_at"`
}

func main() {
	fmt.Println("=== 动态分库分表示例 ===\n")
	
	// Example 1: Modulo sharding
	fmt.Println("1. 模运算分片策略:")
	moduloConfig := ShardingConfig{
		Strategy:      "modulo",
		DatabaseCount: 4,
		TableCount:    8,
		TablePrefix:   "orders",
		ShardKey:      "user_id",
	}
	
	moduloService, err := NewDynamicShardingService(moduloConfig)
	if err != nil {
		log.Fatalf("Failed to create modulo sharding service: %v", err)
	}
	
	// Test different user IDs to show sharding
	userIDs := []int64{123, 456, 789, 1000, 12345}
	for _, userID := range userIDs {
		orders, err := moduloService.FindUserOrders(context.Background(), userID, 1, 20)
		if err != nil {
			log.Printf("Error finding orders for user %d: %v", userID, err)
			continue
		}
		_ = orders
	}
	
	// Insert an order
	order := Order{
		UserID:    12345,
		ProductID: 678,
		Amount:    99.99,
		Status:    "active",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	err = moduloService.InsertOrder(context.Background(), order)
	if err != nil {
		log.Printf("Error inserting order: %v", err)
	}
	
	fmt.Println()
	
	// Example 2: Range sharding
	fmt.Println("2. 范围分片策略:")
	rangeConfig := ShardingConfig{
		Strategy:      "range",
		DatabaseCount: 3,
		TableCount:    4,
		TablePrefix:   "users",
		ShardKey:      "user_id",
	}
	
	rangeService, err := NewDynamicShardingService(rangeConfig)
	if err != nil {
		log.Fatalf("Failed to create range sharding service: %v", err)
	}
	
	// Test different user ID ranges
	rangeUserIDs := []int64{500000, 1500000, 2500000}
	for _, userID := range rangeUserIDs {
		orders, err := rangeService.FindUserOrders(context.Background(), userID, 1, 20)
		if err != nil {
			log.Printf("Error finding orders for user %d: %v", userID, err)
			continue
		}
		_ = orders
	}
	
	fmt.Println()
	
	// Example 3: Consistent hash sharding
	fmt.Println("3. 一致性哈希分片策略:")
	hashConfig := ShardingConfig{
		Strategy:      "consistent_hash",
		DatabaseCount: 4,
		TableCount:    6,
		TablePrefix:   "products",
		ShardKey:      "category_id",
	}
	
	hashService, err := NewDynamicShardingService(hashConfig)
	if err != nil {
		log.Fatalf("Failed to create hash sharding service: %v", err)
	}
	
	// Test different category IDs
	categoryIDs := []int64{100, 200, 300, 400, 500}
	for _, categoryID := range categoryIDs {
		orders, err := hashService.FindUserOrders(context.Background(), categoryID, 1, 20)
		if err != nil {
			log.Printf("Error finding orders for category %d: %v", categoryID, err)
			continue
		}
		_ = orders
	}
	
	fmt.Println()
	
	// Example 4: Cross-shard query
	fmt.Println("4. 跨分片查询:")
	crossUserIDs := []int64{123, 456, 789}
	orders, err := moduloService.FindOrdersAcrossShards(context.Background(), crossUserIDs)
	if err != nil {
		log.Printf("Error in cross-shard query: %v", err)
	} else {
		_ = orders
	}
	
	fmt.Println()
	
	// Example 5: Time-based sharding
	fmt.Println("5. 时间分片策略:")
	dateConfig := ShardingConfig{
		Strategy:      "date_based",
		DatabaseCount: 1,
		TableCount:    365,
		TablePrefix:   "logs",
		ShardKey:      "date",
	}
	
	dateService, err := NewDynamicShardingService(dateConfig)
	if err != nil {
		log.Fatalf("Failed to create date-based sharding service: %v", err)
	}
	
	// Query activity for a date range
	startDate := time.Now().AddDate(0, 0, -7) // 7 days ago
	endDate := time.Now()
	
	activities, err := dateService.FindUserActivity(context.Background(), 12345, startDate, endDate)
	if err != nil {
		log.Printf("Error finding user activity: %v", err)
	} else {
		_ = activities
	}
	
	fmt.Println()
	fmt.Println("=== 动态分库分表特性总结 ===")
	fmt.Println("✅ 支持多种分片策略: 模运算、范围、一致性哈希、时间")
	fmt.Println("✅ 动态数据库和表选择")
	fmt.Println("✅ 跨分片查询支持")
	fmt.Println("✅ 自动分片路由")
	fmt.Println("✅ 业务层透明使用")
	fmt.Println("✅ 高性能和可扩展性")
	fmt.Println("✅ 完整的错误处理")
}
