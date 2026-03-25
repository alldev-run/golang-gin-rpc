package main

import (
	"fmt"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/db/mysql"
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
	fmt.Println("=== ORM 演示示例 ===")
	fmt.Println("注意：此示例演示 ORM API 用法，不需要实际的数据库连接")

	// 演示 ORM 构建器的用法
	demonstrateORMUsage()

	fmt.Println("=== ORM 演示示例完成 ===")
}

// demonstrateORMUsage 演示 ORM 用法（不连接数据库）
func demonstrateORMUsage() {
	fmt.Println("\n--- 1. ORM 构建器演示 ---")

	// 演示 SELECT 构建器
	fmt.Println("1.1 SELECT 查询构建器:")

	// 注意：这里只是演示构建器的链式调用，不实际执行
	fmt.Println("   构建查询示例:")
	fmt.Println("   - ormInstance.Select(\"users\")")
	fmt.Println("   -     Columns(\"id\", \"name\", \"email\", \"age\")")
	fmt.Println("   -     Eq(\"status\", \"active\")")
	fmt.Println("   -     Where(\"age > ?\", 18)")
	fmt.Println("   -     OrderBy(\"name ASC\")")
	fmt.Println("   -     Limit(10)")
	fmt.Println("   -     Query(ctx)")

	fmt.Println("\n1.2 INSERT 构建器:")
	fmt.Println("   构建插入示例:")
	fmt.Println("   - ormInstance.Insert(\"users\")")
	fmt.Println("   -     Set(\"name\", \"张三\")")
	fmt.Println("   -     Set(\"email\", \"zhangsan@example.com\")")
	fmt.Println("   -     Set(\"age\", 25)")
	fmt.Println("   -     Exec(ctx)")

	fmt.Println("\n1.3 UPDATE 构建器:")
	fmt.Println("   构建更新示例:")
	fmt.Println("   - ormInstance.Update(\"users\")")
	fmt.Println("   -     Set(\"age\", 26)")
	fmt.Println("   -     Eq(\"email\", \"zhangsan@example.com\")")
	fmt.Println("   -     Exec(ctx)")

	fmt.Println("\n1.4 DELETE 构建器:")
	fmt.Println("   构建删除示例:")
	fmt.Println("   - ormInstance.Delete(\"users\")")
	fmt.Println("   -     Eq(\"id\", 1)")
	fmt.Println("   -     Exec(ctx)")

	fmt.Println("\n--- 2. 事务演示 ---")
	fmt.Println("   事务使用示例:")
	fmt.Println("   - ormInstance.Transaction(ctx, func(txORM *orm.ORM) error {")
	fmt.Println("   -     // 在事务中执行操作")
	fmt.Println("   -     _, err := txORM.Insert(\"users\").Set(\"name\", \"Alice\").Exec(ctx)")
	fmt.Println("   -     if err != nil {")
	fmt.Println("   -         return err // 自动回滚")
	fmt.Println("   -     }")
	fmt.Println("   -     return nil // 自动提交")
	fmt.Println("   - })")

	fmt.Println("\n--- 3. 数据扫描演示 ---")
	fmt.Println("   单行扫描示例:")
	fmt.Println("   - row := ormInstance.Select(\"users\").Eq(\"id\", 1).QueryRow(ctx)")
	fmt.Println("   - var user User")
	fmt.Println("   - err := row.Scan(&user.ID, &user.Name, &user.Email, &user.Age)")

	fmt.Println("\n   多行扫描示例:")
	fmt.Println("   - rows, err := ormInstance.Select(\"users\").Query(ctx)")
	fmt.Println("   - defer rows.Close()")
	fmt.Println("   - var users []User")
	fmt.Println("   - err := orm.StructScanAll(rows, &users)")

	fmt.Println("\n--- 4. MySQL 连接配置演示 ---")
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

	fmt.Printf("   MySQL 连接配置:\n")
	fmt.Printf("   - Host: %s\n", config.Host)
	fmt.Printf("   - Port: %d\n", config.Port)
	fmt.Printf("   - Database: %s\n", config.Database)
	fmt.Printf("   - Username: %s\n", config.Username)
	fmt.Printf("   - MaxOpenConns: %d\n", config.MaxOpenConns)
	fmt.Printf("   - MaxIdleConns: %d\n", config.MaxIdleConns)

	fmt.Println("\n--- 5. 实际使用时的完整流程 ---")
	fmt.Println("   1. 创建 MySQL 客户端:")
	fmt.Println("      client, err := mysql.New(config)")
	fmt.Println("   2. 创建 ORM 实例:")
	fmt.Println("      ormInstance := orm.NewORMWithDB(client.DB(), orm.NewMySQLDialect())")
	fmt.Println("   3. 执行数据库操作:")
	fmt.Println("      result, err := ormInstance.Insert(\"users\").Set(...).Exec(ctx)")
	fmt.Println("   4. 处理结果:")
	fmt.Println("      id, err := result.LastInsertId()")
	fmt.Println("   5. 清理资源:")
	fmt.Println("      defer client.Close()")
	fmt.Println("      defer ormInstance.Close()")

	fmt.Println("\n✓ ORM API 演示完成")
	fmt.Println("\n💡 提示：")
	fmt.Println("   - 要运行实际的数据库操作，请确保 MySQL 服务正在运行")
	fmt.Println("   - 创建测试数据库: CREATE DATABASE test_db;")
	fmt.Println("   - 运行完整示例: go run main_fixed.go")
}
