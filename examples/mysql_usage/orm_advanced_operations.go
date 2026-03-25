package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/db/mysql"
	"github.com/alldev-run/golang-gin-rpc/pkg/db/orm"
)

// Product 产品模型
type Product struct {
	ID          int64     `db:"id" json:"id"`
	Name        string    `db:"name" json:"name"`
	Description string    `db:"description" json:"description"`
	Price       float64   `db:"price" json:"price"`
	Stock       int       `db:"stock" json:"stock"`
	Category    string    `db:"category" json:"category"`
	Status      string    `db:"status" json:"status"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

// Order 订单模型
type Order struct {
	ID        int64     `db:"id" json:"id"`
	UserID    int64     `db:"user_id" json:"user_id"`
	Total     float64   `db:"total" json:"total"`
	Status    string    `db:"status" json:"status"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// OrderItem 订单项模型
type OrderItem struct {
	ID        int64   `db:"id" json:"id"`
	OrderID   int64   `db:"order_id" json:"order_id"`
	ProductID int64   `db:"product_id" json:"product_id"`
	Quantity  int     `db:"quantity" json:"quantity"`
	Price     float64 `db:"price" json:"price"`
}

func main() {
	fmt.Println("=== ORM 高级操作示例 ===")

	// 创建数据库连接
	mysqlClient, err := createMySQLConnection()
	if err != nil {
		log.Fatalf("创建数据库连接失败: %v", err)
	}
	defer mysqlClient.Close()

	// 创建 ORM 实例
	ormInstance := orm.NewORMWithDB(mysqlClient.DB(), orm.NewMySQLDialect())
	defer ormInstance.Close()

	ctx := context.Background()

	// 初始化数据库表
	if err := initDatabase(ormInstance, ctx); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}

	// 准备测试数据
	if err := prepareTestData(ormInstance, ctx); err != nil {
		log.Fatalf("准备测试数据失败: %v", err)
	}

	// 执行高级操作示例
	if err := runAdvancedExamples(ormInstance, ctx); err != nil {
		log.Fatalf("执行高级示例失败: %v", err)
	}

	fmt.Println("=== ORM 高级操作示例完成 ===")
}

// createMySQLConnection 创建 MySQL 连接
func createMySQLConnection() (*mysql.Client, error) {
	config := mysql.Config{
		Host:            "localhost",
		Port:            3306,
		Database:        "myblog",
		Username:        "root",
		Password:        "q1w2e3r4",
		Charset:         "utf8mb4",
		MaxOpenConns:    25,
		MaxIdleConns:    10,
		ConnMaxLifetime: time.Hour,
		ConnMaxIdleTime: 30 * time.Minute,
	}

	client, err := mysql.New(config)
	if err != nil {
		return nil, fmt.Errorf("创建 MySQL 客户端失败: %w", err)
	}

	// 测试连接
	ctx := context.Background()
	if err := client.DB().PingContext(ctx); err != nil {
		return nil, fmt.Errorf("数据库连接测试失败: %w", err)
	}

	fmt.Println("✓ 数据库连接成功")
	return client, nil
}

// initDatabase 初始化数据库表
func initDatabase(ormInstance *orm.ORM, ctx context.Context) error {
	// 创建产品表
	createProductsSQL := `
	CREATE TABLE IF NOT EXISTS products (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		name VARCHAR(200) NOT NULL,
		description TEXT,
		Price DECIMAL(10,2) DEFAULT 0.00,
		stock INT DEFAULT 0,
		category VARCHAR(50) DEFAULT 'general',
		status VARCHAR(20) DEFAULT 'active',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		INDEX idx_category (category),
		INDEX idx_status (status),
		INDEX idx_price (price)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
	`

	// 创建订单表
	createOrdersSQL := `
	CREATE TABLE IF NOT EXISTS orders (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		user_id BIGINT NOT NULL,
		total DECIMAL(10,2) DEFAULT 0.00,
		status VARCHAR(20) DEFAULT 'pending',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		INDEX idx_user_id (user_id),
		INDEX idx_status (status),
		INDEX idx_created_at (created_at)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
	`

	// 创建订单项表
	createOrderItemsSQL := `
	CREATE TABLE IF NOT EXISTS order_items (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		order_id BIGINT NOT NULL,
		product_id BIGINT NOT NULL,
		quantity INT DEFAULT 1,
		price DECIMAL(10,2) NOT NULL,
		INDEX idx_order_id (order_id),
		INDEX idx_product_id (product_id),
		FOREIGN KEY (order_id) REFERENCES orders(id),
		FOREIGN KEY (product_id) REFERENCES products(id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
	`

	tables := []string{createProductsSQL, createOrdersSQL, createOrderItemsSQL}
	for _, sql := range tables {
		if _, err := ormInstance.DB().Exec(ctx, sql); err != nil {
			return fmt.Errorf("创建表失败: %w", err)
		}
	}

	fmt.Println("✓ 数据库表初始化成功")
	return nil
}

// prepareTestData 准备测试数据
func prepareTestData(ormInstance *orm.ORM, ctx context.Context) error {
	// 清空现有数据
	tables := []string{"order_items", "orders", "products"}
	for _, table := range tables {
		if _, err := ormInstance.DB().Exec(ctx, fmt.Sprintf("DELETE FROM %s", table)); err != nil {
			return fmt.Errorf("清空表 %s 失败: %w", table, err)
		}
	}

	// 插入产品数据
	products := []Product{
		{Name: "iPhone 14", Description: "苹果手机", Price: 799.00, Stock: 100, Category: "electronics"},
		{Name: "MacBook Pro", Description: "苹果笔记本", Price: 1299.00, Stock: 50, Category: "electronics"},
		{Name: "AirPods", Description: "苹果耳机", Price: 199.00, Stock: 200, Category: "electronics"},
		{Name: "Coffee Maker", Description: "咖啡机", Price: 89.00, Stock: 30, Category: "appliances"},
		{Name: "Desk Chair", Description: "办公椅", Price: 299.00, Stock: 20, Category: "furniture"},
	}

	for _, product := range products {
		result, err := ormInstance.Insert("products").
			Set("name", product.Name).
			Set("description", product.Description).
			Set("price", product.Price).
			Set("stock", product.Stock).
			Set("category", product.Category).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("插入产品失败: %w", err)
		}
		id, _ := result.LastInsertId()
		product.ID = id
	}

	fmt.Printf("✓ 准备了 %d 个产品\n", len(products))
	return nil
}

// runAdvancedExamples 运行高级操作示例
func runAdvancedExamples(ormInstance *orm.ORM, ctx context.Context) error {
	fmt.Println("\n--- 1. JOIN 查询示例 ---")
	if err := joinExamples(ormInstance, ctx); err != nil {
		return err
	}

	fmt.Println("\n--- 2. 子查询示例 ---")
	if err := subqueryExamples(ormInstance, ctx); err != nil {
		return err
	}

	fmt.Println("\n--- 3. 分页和排序示例 ---")
	if err := paginationExamples(ormInstance, ctx); err != nil {
		return err
	}

	fmt.Println("\n--- 4. 聚合和分组示例 ---")
	if err := aggregationExamples(ormInstance, ctx); err != nil {
		return err
	}

	fmt.Println("\n--- 5. 事务示例 ---")
	if err := transactionExamples(ormInstance, ctx); err != nil {
		return err
	}

	return nil
}

// joinExamples JOIN 查询示例
func joinExamples(ormInstance *orm.ORM, ctx context.Context) error {
	// 先创建一些订单数据
	orderIDs := []int64{1, 2}
	for i, userID := range []int64{1001, 1002} {
		result, err := ormInstance.Insert("orders").
			Set("user_id", userID).
			Set("total", float64(i+1)*100.00).
			Set("status", "completed").
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("创建订单失败: %w", err)
		}
		orderID, _ := result.LastInsertId()
		orderIDs[i] = orderID

		// 为每个订单创建订单项
		for j, productID := range []int64{1, 2} {
			_, err = ormInstance.Insert("order_items").
				Set("order_id", orderID).
				Set("product_id", productID).
				Set("quantity", j+1).
				Set("price", float64(j+1)*50.00).
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("创建订单项失败: %w", err)
			}
		}
	}

	// 示例1: INNER JOIN
	fmt.Println("1.1 INNER JOIN 查询:")
	rows, err := ormInstance.Select("orders o").
		Columns("o.id", "o.user_id", "o.total", "oi.product_id", "oi.quantity").
		Join("order_items oi", "o.id = oi.order_id").
		Eq("o.status", "completed").
		Query(ctx)
	if err != nil {
		return fmt.Errorf("JOIN 查询失败: %w", err)
	}
	defer rows.Close()

	type OrderItemResult struct {
		OrderID   int64   `db:"id"`
		UserID    int64   `db:"user_id"`
		Total     float64 `db:"total"`
		ProductID int64   `db:"product_id"`
		Quantity  int     `db:"quantity"`
	}

	var results []OrderItemResult
	if err := orm.StructScanAll(rows, &results); err != nil {
		return fmt.Errorf("扫描 JOIN 结果失败: %w", err)
	}

	fmt.Printf("   ✓ INNER JOIN 查询到 %d 条记录:\n", len(results))
	for _, r := range results {
		fmt.Printf("     订单ID=%d, 用户ID=%d, 产品ID=%d, 数量=%d\n",
			r.OrderID, r.UserID, r.ProductID, r.Quantity)
	}

	// 示例2: LEFT JOIN
	fmt.Println("1.2 LEFT JOIN 查询 (包含没有订单项的订单):")
	rows, err = ormInstance.Select("orders o").
		Columns("o.id", "o.user_id", "COUNT(oi.id) as item_count").
		LeftJoin("order_items oi", "o.id = oi.order_id").
		GroupBy("o.id", "o.user_id").
		Query(ctx)
	if err != nil {
		return fmt.Errorf("LEFT JOIN 查询失败: %w", err)
	}
	defer rows.Close()

	type OrderCountResult struct {
		OrderID   int64 `db:"id"`
		UserID    int64 `db:"user_id"`
		ItemCount int   `db:"item_count"`
	}

	var countResults []OrderCountResult
	if err := orm.StructScanAll(rows, &countResults); err != nil {
		return fmt.Errorf("扫描 LEFT JOIN 结果失败: %w", err)
	}

	fmt.Printf("   ✓ LEFT JOIN 查询到 %d 个订单:\n", len(countResults))
	for _, r := range countResults {
		fmt.Printf("     订单ID=%d, 用户ID=%d, 订单项数量=%d\n",
			r.OrderID, r.UserID, r.ItemCount)
	}

	return nil
}

// subqueryExamples 子查询示例
func subqueryExamples(ormInstance *orm.ORM, ctx context.Context) error {
	// 示例1: EXISTS 子查询
	fmt.Println("2.1 EXISTS 子查询:")
	// 查询有订单项的产品
	subQuerySQL, subQueryArgs := ormInstance.Select("order_items").
		Columns("1").
		Eq("product_id", 1).
		Build()

	rows, err := ormInstance.Select("products p").
		Columns("p.id", "p.name", "p.price").
		Where("EXISTS ("+subQuerySQL+")", subQueryArgs...).
		Query(ctx)
	if err != nil {
		return fmt.Errorf("EXISTS 子查询失败: %w", err)
	}
	defer rows.Close()

	var products []Product
	if err := orm.StructScanAll(rows, &products); err != nil {
		return fmt.Errorf("扫描 EXISTS 结果失败: %w", err)
	}

	fmt.Printf("   ✓ EXISTS 子查询找到 %d 个有订单的产品:\n", len(products))
	for _, p := range products {
		fmt.Printf("     - ID=%d, Name=%s, Price=%.2f\n", p.ID, p.Name, p.Price)
	}

	// 示例2: IN 子查询
	fmt.Println("2.2 IN 子查询:")
	// 查询价格高于平均价格的产品
	subQuerySQL2, subQueryArgs2 := ormInstance.Select("products").
		Columns("AVG(price)").
		Eq("category", "electronics").
		Build()

	rows, err = ormInstance.Select("products").
		Columns("id", "name", "price", "category").
		Where("price > ("+subQuerySQL2+")", subQueryArgs2...).
		Where("category = ?", "electronics").
		Query(ctx)
	if err != nil {
		return fmt.Errorf("IN 子查询失败: %w", err)
	}
	defer rows.Close()

	var expensiveProducts []Product
	if err := orm.StructScanAll(rows, &expensiveProducts); err != nil {
		return fmt.Errorf("扫描 IN 结果失败: %w", err)
	}

	fmt.Printf("   ✓ IN 子查询找到 %d 个高于平均价格的电子产品:\n", len(expensiveProducts))
	for _, p := range expensiveProducts {
		fmt.Printf("     - ID=%d, Name=%s, Price=%.2f\n", p.ID, p.Name, p.Price)
	}

	// 示例3: FROM 子查询
	fmt.Println("2.3 FROM 子查询:")
	// 查询每个类别的产品数量和平均价格
	subQuerySQL3, subQueryArgs3 := ormInstance.Select("products").
		Columns("category", "COUNT(*) as count", "AVG(price) as avg_price").
		GroupBy("category").
		Having("COUNT(*) > ?", 0).
		Build()

	rows, err = ormInstance.Select("ignored").
		FromRaw("("+subQuerySQL3+") as cat_stats", subQueryArgs3...).
		Columns("category", "count", "avg_price").
		OrderByDesc("avg_price").
		Query(ctx)
	if err != nil {
		return fmt.Errorf("FROM 子查询失败: %w", err)
	}
	defer rows.Close()

	type CategoryStats struct {
		Category string  `db:"category"`
		Count    int     `db:"count"`
		AvgPrice float64 `db:"avg_price"`
	}

	var stats []CategoryStats
	if err := orm.StructScanAll(rows, &stats); err != nil {
		return fmt.Errorf("扫描 FROM 子查询结果失败: %w", err)
	}

	fmt.Printf("   ✓ FROM 子查询统计到 %d 个类别:\n", len(stats))
	for _, s := range stats {
		fmt.Printf("     - 类别=%s, 数量=%d, 平均价格=%.2f\n", s.Category, s.Count, s.AvgPrice)
	}

	return nil
}

// paginationExamples 分页和排序示例
func paginationExamples(ormInstance *orm.ORM, ctx context.Context) error {
	// 示例1: 基础分页
	fmt.Println("3.1 基础分页:")
	page := 1
	pageSize := 2

	rows, err := ormInstance.Select("products").
		Columns("id", "name", "price", "category").
		OrderBy("price DESC").
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Query(ctx)
	if err != nil {
		return fmt.Errorf("分页查询失败: %w", err)
	}
	defer rows.Close()

	var products []Product
	if err := orm.StructScanAll(rows, &products); err != nil {
		return fmt.Errorf("扫描分页结果失败: %w", err)
	}

	fmt.Printf("   ✓ 第 %d 页 (每页 %d 条) 查询到 %d 个产品:\n", page, pageSize, len(products))
	for _, p := range products {
		fmt.Printf("     - ID=%d, Name=%s, Price=%.2f\n", p.ID, p.Name, p.Price)
	}

	// 示例2: 多字段排序
	fmt.Println("3.2 多字段排序:")
	rows, err = ormInstance.Select("products").
		Columns("id", "name", "price", "category", "stock").
		OrderBy("category ASC").
		OrderByDesc("price").
		OrderBy("name ASC").
		Query(ctx)
	if err != nil {
		return fmt.Errorf("多字段排序查询失败: %w", err)
	}
	defer rows.Close()

	var sortedProducts []Product
	if err := orm.StructScanAll(rows, &sortedProducts); err != nil {
		return fmt.Errorf("扫描排序结果失败: %w", err)
	}

	fmt.Printf("   ✓ 多字段排序查询到 %d 个产品:\n", len(sortedProducts))
	for _, p := range sortedProducts {
		fmt.Printf("     - %s/%s: %s (%.2f)\n", p.Category, p.Name, p.Name, p.Price)
	}

	// 示例3: 条件分页
	fmt.Println("3.3 条件分页:")
	rows, err = ormInstance.Select("products").
		Columns("id", "name", "price", "stock").
		Where("price BETWEEN ? AND ?", 100, 500).
		Where("stock > ?", 10).
		OrderBy("price ASC").
		Limit(3).
		Query(ctx)
	if err != nil {
		return fmt.Errorf("条件分页查询失败: %w", err)
	}
	defer rows.Close()

	var filteredProducts []Product
	if err := orm.StructScanAll(rows, &filteredProducts); err != nil {
		return fmt.Errorf("扫描条件分页结果失败: %w", err)
	}

	fmt.Printf("   ✓ 条件分页查询到 %d 个产品 (价格100-500且库存>10):\n", len(filteredProducts))
	for _, p := range filteredProducts {
		fmt.Printf("     - ID=%d, Name=%s, Price=%.2f, Stock=%d\n", p.ID, p.Name, p.Price, p.Stock)
	}

	return nil
}

// aggregationExamples 聚合和分组示例
func aggregationExamples(ormInstance *orm.ORM, ctx context.Context) error {
	// 示例1: 基础聚合
	fmt.Println("4.1 基础聚合:")
	row := ormInstance.Select("products").
		Columns("COUNT(*) as total", "AVG(price) as avg_price", "MIN(price) as min_price", "MAX(price) as max_price").
		QueryRow(ctx)

	var total int
	var avgPrice, minPrice, maxPrice float64
	if err := row.Scan(&total, &avgPrice, &minPrice, &maxPrice); err != nil {
		return fmt.Errorf("聚合查询失败: %w", err)
	}

	fmt.Printf("   ✓ 聚合统计: 总数=%d, 平均价格=%.2f, 最低价格=%.2f, 最高价格=%.2f\n",
		total, avgPrice, minPrice, maxPrice)

	// 示例2: 分组聚合
	fmt.Println("4.2 分组聚合:")
	rows, err := ormInstance.Select("products").
		Columns("category", "COUNT(*) as count", "AVG(price) as avg_price", "SUM(stock) as total_stock").
		GroupBy("category").
		Having("COUNT(*) > ?", 0).
		OrderByDesc("count").
		Query(ctx)
	if err != nil {
		return fmt.Errorf("分组聚合查询失败: %w", err)
	}
	defer rows.Close()

	type CategoryAggregation struct {
		Category   string  `db:"category"`
		Count      int     `db:"count"`
		AvgPrice   float64 `db:"avg_price"`
		TotalStock int     `db:"total_stock"`
	}

	var aggregations []CategoryAggregation
	if err := orm.StructScanAll(rows, &aggregations); err != nil {
		return fmt.Errorf("扫描分组聚合结果失败: %w", err)
	}

	fmt.Printf("   ✓ 分组聚合统计到 %d 个类别:\n", len(aggregations))
	for _, a := range aggregations {
		fmt.Printf("     - %s: 数量=%d, 平均价格=%.2f, 总库存=%d\n",
			a.Category, a.Count, a.AvgPrice, a.TotalStock)
	}

	// 示例3: 复杂聚合
	fmt.Println("4.3 复杂聚合:")
	rows, err = ormInstance.Select("products").
		Columns("category",
			"COUNT(*) as total_products",
			"COUNT(CASE WHEN price > 100 THEN 1 END) as expensive_products",
			"AVG(CASE WHEN stock > 50 THEN price END) as high_stock_avg_price").
		GroupBy("category").
		Having("COUNT(*) > ?", 1).
		Query(ctx)
	if err != nil {
		return fmt.Errorf("复杂聚合查询失败: %w", err)
	}
	defer rows.Close()

	type ComplexAggregation struct {
		Category          string  `db:"category"`
		TotalProducts     int     `db:"total_products"`
		ExpensiveProducts int     `db:"expensive_products"`
		HighStockAvgPrice float64 `db:"high_stock_avg_price"`
	}

	var complexAggs []ComplexAggregation
	if err := orm.StructScanAll(rows, &complexAggs); err != nil {
		return fmt.Errorf("扫描复杂聚合结果失败: %w", err)
	}

	fmt.Printf("   ✓ 复杂聚合统计到 %d 个类别:\n", len(complexAggs))
	for _, a := range complexAggs {
		fmt.Printf("     - %s: 总产品=%d, 高价产品=%d, 高库存均价=%.2f\n",
			a.Category, a.TotalProducts, a.ExpensiveProducts, a.HighStockAvgPrice)
	}

	return nil
}

// transactionExamples 事务示例
func transactionExamples(ormInstance *orm.ORM, ctx context.Context) error {
	// 示例1: 基础事务
	fmt.Println("5.1 基础事务:")
	err := ormInstance.Transaction(ctx, func(txORM *orm.ORM) error {
		// 在事务中创建订单
		result, err := txORM.Insert("orders").
			Set("user_id", 2001).
			Set("total", 299.00).
			Set("status", "pending").
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("创建订单失败: %w", err)
		}

		orderID, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("获取订单ID失败: %w", err)
		}

		fmt.Printf("   ✓ 在事务中创建订单: ID=%d\n", orderID)

		// 在事务中添加订单项
		_, err = txORM.Insert("order_items").
			Set("order_id", orderID).
			Set("product_id", 1).
			Set("quantity", 1).
			Set("price", 299.00).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("创建订单项失败: %w", err)
		}

		fmt.Printf("   ✓ 在事务中添加订单项: 订单ID=%d, 产品ID=1\n", orderID)

		// 更新产品库存
		_, err = txORM.Update("products").
			Set("stock", "stock - 1").
			Eq("id", 1).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("更新产品库存失败: %w", err)
		}

		fmt.Printf("   ✓ 在事务中更新产品库存: 产品ID=1\n")

		// 更新订单状态为完成
		_, err = txORM.Update("orders").
			Set("status", "completed").
			Eq("id", orderID).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("更新订单状态失败: %w", err)
		}

		fmt.Printf("   ✓ 在事务中更新订单状态: 订单ID=%d\n", orderID)

		return nil
	})

	if err != nil {
		fmt.Printf("   ✗ 事务失败: %v\n", err)
		return fmt.Errorf("事务执行失败: %w", err)
	}

	fmt.Printf("   ✓ 事务成功完成\n")

	// 示例2: 事务回滚示例
	fmt.Println("5.2 事务回滚示例:")
	err = ormInstance.Transaction(ctx, func(txORM *orm.ORM) error {
		// 创建订单
		result, err := txORM.Insert("orders").
			Set("user_id", 2002).
			Set("total", 199.00).
			Set("status", "pending").
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("创建订单失败: %w", err)
		}

		orderID, _ := result.LastInsertId()
		fmt.Printf("   ✓ 在事务中创建订单: ID=%d\n", orderID)

		// 模拟错误 - 尝试插入无效的产品ID
		_, err = txORM.Insert("order_items").
			Set("order_id", orderID).
			Set("product_id", 99999). // 不存在的产品ID
			Set("quantity", 1).
			Set("price", 199.00).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("创建订单项失败（预期错误）: %w", err)
		}

		return nil
	})

	if err != nil {
		fmt.Printf("   ✓ 事务按预期回滚: %v\n", err)
	} else {
		fmt.Printf("   ✗ 事务意外成功\n")
	}

	// 验证回滚效果
	row := ormInstance.Select("orders").
		Columns("COUNT(*)").
		Eq("user_id", 2002).
		QueryRow(ctx)

	var count int
	if err := row.Scan(&count); err != nil {
		return fmt.Errorf("验证回滚失败: %w", err)
	}

	fmt.Printf("   ✓ 验证回滚: 用户2002的订单数量=%d\n", count)

	return nil
}
