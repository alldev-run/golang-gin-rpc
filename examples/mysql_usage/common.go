package main

import (
	"context"
	"fmt"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/db/mysql"
	"github.com/alldev-run/golang-gin-rpc/pkg/db/orm"
)

// User 用户模型 - 统一使用 int64 ID
type User struct {
	ID        int64     `db:"id" json:"id"`
	Name      string    `db:"name" json:"name"`
	Email     string    `db:"email" json:"email"`
	Age       int       `db:"age" json:"age"`
	Status    string    `db:"status" json:"status"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

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

// createMySQLConnection 创建 MySQL 连接 - 公共方法
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

// createMySQLConnectionWithConfig 使用自定义配置创建 MySQL 连接
func createMySQLConnectionWithConfig(config mysql.Config) (*mysql.Client, error) {
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

// initUserTable 初始化用户表
func initUserTable(ormInstance *orm.ORM, ctx context.Context) error {
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS users (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		name VARCHAR(100) NOT NULL,
		email VARCHAR(100) UNIQUE NOT NULL,
		age INT DEFAULT 0,
		status VARCHAR(20) DEFAULT 'active',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		INDEX idx_email (email),
		INDEX idx_status (status)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
	`

	if _, err := ormInstance.DB().Exec(ctx, createTableSQL); err != nil {
		return fmt.Errorf("创建用户表失败: %w", err)
	}

	fmt.Println("✓ 用户表初始化成功")
	return nil
}

// initProductTables 初始化产品相关表
func initProductTables(ormInstance *orm.ORM, ctx context.Context) error {
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

	fmt.Println("✓ 产品相关表初始化成功")
	return nil
}
