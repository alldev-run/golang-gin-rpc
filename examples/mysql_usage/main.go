
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/db"
	"github.com/alldev-run/golang-gin-rpc/pkg/db/mysql"
	"github.com/alldev-run/golang-gin-rpc/pkg/db/orm"
	"github.com/alldev-run/golang-gin-rpc/pkg/db/pool"
	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
)

// User 示例用户模型
type User struct {
	ID        int       `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Email     string    `json:"email" db:"email"`
	Age       int       `json:"age" db:"age"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

func main() {
	// 初始化日志
	logger.Init(logger.Config{
		Level:   "info",
		Env:     "dev",
		LogPath: "./logs/mysql_example.log",
	})

	fmt.Println("=== MySQL 使用示例 ===")

	// 示例1: 直接使用 MySQL 客户端
	fmt.Println("1. 直接使用 MySQL 客户端:")
	directMySQLExample()

	fmt.Println("\n2. 使用工厂模式创建 MySQL 客户端:")
	factoryExample()

	fmt.Println("\n3. 使用 ORM 进行数据库操作:")
	ormExample()

	fmt.Println("\n4. 使用事务管理器:")
	transactionExample()

	fmt.Println("\n5. 使用连接池:")
	poolExample()

	fmt.Println("\n=== MySQL 使用示例完成 ===")
}

// directMySQLExample 直接使用 MySQL 客户端
func directMySQLExample() {
	// 配置 MySQL 连接
	config := mysql.Config{
		Host:            "localhost",
		Port:            3306,
		Database:        "test_db",
		Username:        "root",
		Password:        "password",
		Charset:         "utf8mb4",
		MaxOpenConns:    25,
		MaxIdleConns:    10,
		ConnMaxLifetime: time.Hour,
		ConnMaxIdleTime: 30 * time.Minute,
	}

	// 创建 MySQL 客户端
	client, err := mysql.New(config)
	if err != nil {
		log.Printf("创建 MySQL 客户端失败: %v", err)
		return
	}
	defer client.Close()

	// 测试连接
	ctx := context.Background()
	if err := client.DB().PingContext(ctx); err != nil {
		log.Printf("MySQL 连接测试失败: %v", err)
		return
	}

	fmt.Println("   ✓ MySQL 连接成功")

	// 创建表
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS users (
		id INT AUTO_INCREMENT PRIMARY KEY,
		name VARCHAR(100) NOT NULL,
		email VARCHAR(100) UNIQUE NOT NULL,
		age INT DEFAULT 0,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
	)`

	if _, err := client.DB().ExecContext(ctx, createTableSQL); err != nil {
		log.Printf("创建表失败: %v", err)
		return
	}

	fmt.Println("   ✓ 表创建成功")

	// 插入数据
	insertSQL := "INSERT INTO users (name, email, age) VALUES (?, ?, ?)"
	result, err := client.DB().ExecContext(ctx, insertSQL, "张三", "zhangsan@example.com", 25)
	if err != nil {
		log.Printf("插入数据失败: %v", err)
		return
	}

	id, _ := result.LastInsertId()
	fmt.Printf("   ✓ 插入数据成功，ID: %d\n", id)

	// 查询数据
	var user User
	querySQL := "SELECT id, name, email, age, created_at, updated_at FROM users WHERE id = ?"
	err = client.DB().QueryRowContext(ctx, querySQL, id).Scan(
		&user.ID, &user.Name, &user.Email, &user.Age, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		log.Printf("查询数据失败: %v", err)
		return
	}

	fmt.Printf("   ✓ 查询数据成功: %+v\n", user)

	// 更新数据
	updateSQL := "UPDATE users SET age = ? WHERE id = ?"
	_, err = client.DB().ExecContext(ctx, updateSQL, 26, id)
	if err != nil {
		log.Printf("更新数据失败: %v", err)
		return
	}

	fmt.Println("   ✓ 更新数据成功")

	// 删除数据
	deleteSQL := "DELETE FROM users WHERE id = ?"
	_, err = client.DB().ExecContext(ctx, deleteSQL, id)
	if err != nil {
		log.Printf("删除数据失败: %v", err)
		return
	}

	fmt.Println("   ✓ 删除数据成功")
}

// factoryExample 使用工厂模式创建 MySQL 客户端
func factoryExample() {
	// 配置数据库连接
	config := db.Config{
		Type: db.TypeMySQL,
		MySQL: mysql.Config{
			Host:            "localhost",
			Port:            3306,
			Database:        "test_db",
			Username:        "root",
			Password:        "password",
			Charset:         "utf8mb4",
			MaxOpenConns:    25,
			MaxIdleConns:    10,
			ConnMaxLifetime: time.Hour,
			ConnMaxIdleTime: 30 * time.Minute,
		},
	}

	// 使用工厂创建客户端
	factory := db.NewFactory()
	client, err := factory.Create(config)
	if err != nil {
		log.Printf("工厂创建客户端失败: %v", err)
		return
	}
	defer client.Close()

	// 测试连接
	ctx := context.Background()
	if err := client.Ping(ctx); err != nil {
		log.Printf("连接测试失败: %v", err)
		return
	}

	fmt.Println("   ✓ 工厂模式连接成功")

	// 使用 SQL 接口（如果实现了的话）
	if sqlClient, ok := client.(db.SQLClient); ok {
		// 查询数据
		rows, err := sqlClient.Query(ctx, "SELECT COUNT(*) as count FROM users")
		if err != nil {
			log.Printf("查询失败: %v", err)
			return
		}
		defer rows.Close()

		if rows.Next() {
			var count int
			if err := rows.Scan(&count); err != nil {
				log.Printf("扫描数据失败: %v", err)
				return
			}
			fmt.Printf("   ✓ 用户总数: %d\n", count)
		}
	}
}

// ormExample 使用 ORM 进行数据库操作
func ormExample() {
	// 配置 MySQL 连接
	config := mysql.Config{
		Host:            "localhost",
		Port:            3306,
		Database:        "test_db",
		Username:        "root",
		Password:        "password",
		Charset:         "utf8mb4",
		MaxOpenConns:    25,
		MaxIdleConns:    10,
		ConnMaxLifetime: time.Hour,
		ConnMaxIdleTime: 30 * time.Minute,
	}

	// 创建 MySQL 客户端
	mysqlClient, err := mysql.New(config)
	if err != nil {
		log.Printf("创建 MySQL 客户端失败: %v", err)
		return
	}
	defer mysqlClient.Close()

	// 创建 ORM 实例
	ormInstance := orm.NewORMWithDB(mysqlClient.DB(), nil)
	defer ormInstance.Close()
	ctx := context.Background()

	fmt.Println("   ✓ ORM 实例创建成功")

	// 创建用户
	user := &User{
		Name:  "李四",
		Email: "lisi@example.com",
		Age:   30,
	}

	// 插入用户
	userID, err := ormInstance.Insert("users").
		Set("name", user.Name).
		Set("email", user.Email).
		Set("age", user.Age).
		InsertGetID(ctx)
	if err != nil {
		log.Printf("ORM 创建用户失败: %v", err)
		return
	}
	user.ID = int(userID)

	fmt.Printf("   ✓ ORM 创建用户成功，ID: %d\n", user.ID)

	// 查询用户
	var foundUser User
	err = ormInstance.Select("users").
		Columns("id", "name", "email", "age", "created_at", "updated_at").
		Where("id = ?", user.ID).
		QueryRow(ctx).
		Scan(&foundUser.ID, &foundUser.Name, &foundUser.Email, &foundUser.Age, &foundUser.CreatedAt, &foundUser.UpdatedAt)
	if err != nil {
		log.Printf("ORM 查询用户失败: %v", err)
		return
	}

	fmt.Printf("   ✓ ORM 查询用户成功: %+v\n", foundUser)

	// 更新用户
	_, err = ormInstance.Update("users").
		Set("age", foundUser.Age).
		Where("id = ?", foundUser.ID).
		Exec(ctx)
	if err != nil {
		log.Printf("ORM 更新用户失败: %v", err)
		return
	}
	foundUser.Age = 31

	fmt.Println("   ✓ ORM 更新用户成功")

	// 查询所有用户
	var users []User
	rows, err := ormInstance.Select("users").
		Columns("id", "name", "email", "age", "created_at", "updated_at").
		Query(ctx)
	if err != nil {
		log.Printf("ORM 查询所有用户失败: %v", err)
		return
	}
	defer rows.Close()
	
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Name, &user.Email, &user.Age, &user.CreatedAt, &user.UpdatedAt); err != nil {
			log.Printf("扫描用户数据失败: %v", err)
			continue
		}
		users = append(users, user)
	}

	fmt.Printf("   ✓ ORM 查询到 %d 个用户\n", len(users))

	// 删除用户
	_, err = ormInstance.Delete("users").
		Where("id = ?", foundUser.ID).
		Exec(ctx)
	if err != nil {
		log.Printf("ORM 删除用户失败: %v", err)
		return
	}

	fmt.Println("   ✓ ORM 删除用户成功")
}

// transactionExample 使用事务管理器
func transactionExample() {
	// 配置 MySQL 连接
	config := mysql.Config{
		Host:            "localhost",
		Port:            3306,
		Database:        "test_db",
		Username:        "root",
		Password:        "password",
		Charset:         "utf8mb4",
		MaxOpenConns:    25,
		MaxIdleConns:    10,
		ConnMaxLifetime: time.Hour,
		ConnMaxIdleTime: 30 * time.Minute,
	}

	// 创建 MySQL 客户端
	mysqlClient, err := mysql.New(config)
	if err != nil {
		log.Printf("创建 MySQL 客户端失败: %v", err)
		return
	}
	defer mysqlClient.Close()

	// 创建事务管理器
	tm := orm.NewDefaultTransactionManager()

	ctx := context.Background()

	fmt.Println("   ✓ 事务管理器创建成功")

	// 使用事务
	result, err := tm.WithTransaction(ctx, mysqlClient, func(txORM *orm.ORM) error {
		// 在事务中创建用户
		user1 := &User{
			Name:  "王五",
			Email: "wangwu@example.com",
			Age:   28,
		}

		userID1, err := txORM.Insert("users").
			Set("name", user1.Name).
			Set("email", user1.Email).
			Set("age", user1.Age).
			InsertGetID(ctx)
		if err != nil {
			return fmt.Errorf("创建用户1失败: %w", err)
		}
		user1.ID = int(userID1)

		fmt.Printf("   ✓ 在事务中创建用户1，ID: %d\n", user1.ID)

		// 创建第二个用户
		user2 := &User{
			Name:  "赵六",
			Email: "zhaoliu@example.com",
			Age:   32,
		}

		userID2, err := txORM.Insert("users").
			Set("name", user2.Name).
			Set("email", user2.Email).
			Set("age", user2.Age).
			InsertGetID(ctx)
		if err != nil {
			return fmt.Errorf("创建用户2失败: %w", err)
		}
		user2.ID = int(userID2)

		fmt.Printf("   ✓ 在事务中创建用户2，ID: %d\n", user2.ID)

		// 模拟可能的错误（注释掉以测试成功情况）
		// return fmt.Errorf("模拟错误，事务将回滚")

		return nil
	})

	if err != nil {
		log.Printf("事务执行失败: %v", err)
		fmt.Printf("   ✗ 事务失败: %v (重试次数: %d)\n", err, result.Retries)
	} else {
		fmt.Printf("   ✓ 事务成功: 耗时 %v, 重试次数 %d\n", result.Duration, result.Retries)
	}

	// 检查事务结果
	var userCount int64
	mysqlClient.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE email IN ('wangwu@example.com', 'zhaoliu@example.com')").Scan(&userCount)
	
	if result.Success {
		fmt.Printf("   ✓ 事务提交成功，创建了 %d 个用户\n", userCount)
	} else {
		fmt.Printf("   ✓ 事务回滚成功，用户数量: %d\n", userCount)
	}
}

// poolExample 使用连接池
func poolExample() {
	// 配置多个 MySQL 连接
	configs := []mysql.Config{
		{
			Host:            "localhost",
			Port:            3306,
			Database:        "test_db",
			Username:        "root",
			Password:        "password",
			Charset:         "utf8mb4",
			MaxOpenConns:    10,
			MaxIdleConns:    5,
			ConnMaxLifetime: 30 * time.Minute,
			ConnMaxIdleTime: 5 * time.Minute,
		},
		{
			Host:            "localhost",
			Port:            3306,
			Database:        "test_db",
			Username:        "root",
			Password:        "password",
			Charset:         "utf8mb4",
			MaxOpenConns:    10,
			MaxIdleConns:    5,
			ConnMaxLifetime: 30 * time.Minute,
			ConnMaxIdleTime: 5 * time.Minute,
		},
	}

	// 创建连接池
	poolConfig := pool.Config{
		MaxSize:           5,
		InitialSize:       2,
		MaxIdleTime:       5 * time.Minute,
		HealthCheckPeriod: 30 * time.Second,
		MaxFailures:       3,
		AcquireTimeout:    5 * time.Second,
		RetryDelay:        1 * time.Second,
	}

	factory := db.NewFactory()
	poolInstance := pool.New(poolConfig, factory)
	defer poolInstance.Close()

	ctx := context.Background()

	fmt.Println("   ✓ 连接池创建成功")

	// 注册连接配置到池中
	for i, config := range configs {
		dbConfig := db.Config{
			Type: db.TypeMySQL,
			MySQL: config,
		}

		if err := poolInstance.Register(fmt.Sprintf("mysql-%d", i), dbConfig); err != nil {
			log.Printf("注册连接配置 %d 失败: %v", i, err)
			continue
		}

		fmt.Printf("   ✓ 注册连接配置 %d 到池中\n", i+1)
	}

	// 从池中获取连接并使用
	for i := 0; i < 3; i++ {
		client, err := poolInstance.Acquire(ctx, fmt.Sprintf("mysql-%d", i%2))
		if err != nil {
			log.Printf("从池中获取连接失败: %v", err)
			continue
		}

		// 使用连接
		if err := client.Ping(ctx); err != nil {
			log.Printf("连接健康检查失败: %v", err)
			continue
		}

		fmt.Printf("   ✓ 从池中获取连接成功，第 %d 次\n", i+1)
	}

	// 获取池状态
	stats := poolInstance.GetStats()
	fmt.Printf("   ✓ 连接池状态: 注册连接数=%d\n", len(stats))
}
