package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/alldev-run/golang-gin-rpc/pkg/db/orm"
)


func main() {
	fmt.Println("=== ORM CRUD 操作示例 ===")

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
	if err := initUserTable(ormInstance, ctx); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}

	// 执行 CRUD 操作示例
	if err := runCRUDExamples(ormInstance, ctx); err != nil {
		log.Fatalf("执行 CRUD 示例失败: %v", err)
	}

	fmt.Println("=== ORM CRUD 操作示例完成 ===")
}



// runCRUDExamples 运行 CRUD 操作示例
func runCRUDExamples(ormInstance *orm.ORM, ctx context.Context) error {
	fmt.Println("\n--- 1. CREATE (插入) 示例 ---")
	if err := createExamples(ormInstance, ctx); err != nil {
		return err
	}

	fmt.Println("\n--- 2. READ (查询) 示例 ---")
	if err := readExamples(ormInstance, ctx); err != nil {
		return err
	}

	fmt.Println("\n--- 3. UPDATE (更新) 示例 ---")
	if err := updateExamples(ormInstance, ctx); err != nil {
		return err
	}

	fmt.Println("\n--- 4. DELETE (删除) 示例 ---")
	if err := deleteExamples(ormInstance, ctx); err != nil {
		return err
	}

	return nil
}

// createExamples 插入操作示例
func createExamples(ormInstance *orm.ORM, ctx context.Context) error {
	// 示例1: 单条插入
	fmt.Println("1.1 单条插入:")
	user1 := &User{
		Name:   "张三",
		Email:  "zhangsan@example.com",
		Age:    25,
		Status: "active",
	}

	result, err := ormInstance.Insert("users").
		Set("name", user1.Name).
		Set("email", user1.Email).
		Set("age", user1.Age).
		Set("status", user1.Status).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("插入用户失败: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("获取插入ID失败: %w", err)
	}
	user1.ID = id

	fmt.Printf("   ✓ 插入用户成功: ID=%d, Name=%s, Email=%s\n", user1.ID, user1.Name, user1.Email)

	// 示例2: 批量插入
	fmt.Println("1.2 批量插入:")
	columns := []string{"name", "email", "age", "status"}
	rows := [][]interface{}{
		{"李四", "lisi@example.com", 30, "active"},
		{"王五", "wangwu@example.com", 28, "active"},
		{"赵六", "zhaoliu@example.com", 32, "inactive"},
	}

	result, err = ormInstance.Insert("users").
		Values(columns, rows...).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("批量插入失败: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取影响行数失败: %w", err)
	}

	fmt.Printf("   ✓ 批量插入成功: 插入了 %d 行\n", affected)

	return nil
}

// readExamples 查询操作示例
func readExamples(ormInstance *orm.ORM, ctx context.Context) error {
	// 示例1: 查询单条记录
	fmt.Println("2.1 查询单条记录:")
	row := ormInstance.Select("users").
		Columns("id", "name", "email", "age", "status").
		Eq("email", "zhangsan@example.com").
		QueryRow(ctx)

	var user User
	err := row.Scan(&user.ID, &user.Name, &user.Email, &user.Age, &user.Status)
	if err != nil {
		if err == sql.ErrNoRows {
			fmt.Println("   ✗ 未找到用户")
		} else {
			return fmt.Errorf("扫描用户失败: %w", err)
		}
	} else {
		fmt.Printf("   ✓ 查询成功: ID=%d, Name=%s, Email=%s, Age=%d, Status=%s\n",
			user.ID, user.Name, user.Email, user.Age, user.Status)
	}

	// 示例2: 查询多条记录
	fmt.Println("2.2 查询多条记录:")
	rows, err := ormInstance.Select("users").
		Columns("id", "name", "email", "age", "status").
		Eq("status", "active").
		OrderBy("age ASC").
		Limit(10).
		Query(ctx)
	if err != nil {
		return fmt.Errorf("查询活跃用户失败: %w", err)
	}
	defer rows.Close()

	var users []User
	if err := orm.StructScanAll(rows, &users); err != nil {
		return fmt.Errorf("扫描用户列表失败: %w", err)
	}

	fmt.Printf("   ✓ 查询到 %d 个活跃用户:\n", len(users))
	for i, user := range users {
		fmt.Printf("     %d. ID=%d, Name=%s, Age=%d\n", i+1, user.ID, user.Name, user.Age)
	}

	// 示例3: 复杂条件查询
	fmt.Println("2.3 复杂条件查询:")
	rows, err = ormInstance.Select("users").
		Columns("id", "name", "email", "age", "status").
		Where("(age BETWEEN ? AND ?) AND (status = ?)", 25, 30, "active").
		OrderByDesc("created_at").
		Query(ctx)
	if err != nil {
		return fmt.Errorf("复杂查询失败: %w", err)
	}
	defer rows.Close()

	var filteredUsers []User
	if err := orm.StructScanAll(rows, &filteredUsers); err != nil {
		return fmt.Errorf("扫描过滤用户失败: %w", err)
	}

	fmt.Printf("   ✓ 复杂查询到 %d 个用户 (年龄25-30且活跃):\n", len(filteredUsers))
	for _, user := range filteredUsers {
		fmt.Printf("     - ID=%d, Name=%s, Age=%d\n", user.ID, user.Name, user.Age)
	}

	// 示例4: 聚合查询
	fmt.Println("2.4 聚合查询:")
	row = ormInstance.Select("users").
		Columns("COUNT(*) as total", "AVG(age) as avg_age").
		Eq("status", "active").
		QueryRow(ctx)

	var total int
	var avgAge float64
	if err := row.Scan(&total, &avgAge); err != nil {
		return fmt.Errorf("聚合查询失败: %w", err)
	}

	fmt.Printf("   ✓ 聚合查询: 活跃用户总数=%d, 平均年龄=%.1f\n", total, avgAge)

	return nil
}

// updateExamples 更新操作示例
func updateExamples(ormInstance *orm.ORM, ctx context.Context) error {
	// 示例1: 更新单个字段
	fmt.Println("3.1 更新单个字段:")
	result, err := ormInstance.Update("users").
		Set("age", 26).
		Eq("email", "zhangsan@example.com").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("更新年龄失败: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取影响行数失败: %w", err)
	}

	fmt.Printf("   ✓ 更新成功: 影响了 %d 行\n", affected)

	// 示例2: 更新多个字段
	fmt.Println("3.2 更新多个字段:")
	result, err = ormInstance.Update("users").
		Set("status", "inactive").
		SetExpr("age", "`age` + ?", 1).
		Eq("email", "lisi@example.com").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("更新多个字段失败: %w", err)
	}

	affected, err = result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取影响行数失败: %w", err)
	}

	fmt.Printf("   ✓ 更新成功: 影响了 %d 行\n", affected)

	// 示例3: 条件更新
	fmt.Println("3.3 条件更新:")
	result, err = ormInstance.Update("users").
		Set("status", "active").
		Where("age < ? AND status = ?", 30, "inactive").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("条件更新失败: %w", err)
	}

	affected, err = result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取影响行数失败: %w", err)
	}

	fmt.Printf("   ✓ 条件更新成功: 将 %d 个30岁以下非活跃用户设为活跃\n", affected)

	return nil
}

// deleteExamples 删除操作示例
func deleteExamples(ormInstance *orm.ORM, ctx context.Context) error {
	// 示例1: 根据ID删除
	fmt.Println("4.1 根据ID删除:")
	// 先查询一个要删除的用户ID
	row := ormInstance.Select("users").
		Columns("id").
		Eq("email", "zhaoliu@example.com").
		QueryRow(ctx)

	var deleteID int64
	if err := row.Scan(&deleteID); err != nil {
		if err == sql.ErrNoRows {
			fmt.Println("   ✗ 未找到要删除的用户")
		} else {
			return fmt.Errorf("查询要删除的用户失败: %w", err)
		}
	} else {
		result, err := ormInstance.Delete("users").
			Eq("id", deleteID).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("删除用户失败: %w", err)
		}

		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("获取影响行数失败: %w", err)
		}

		fmt.Printf("   ✓ 删除成功: 删除了 ID=%d 的用户，影响 %d 行\n", deleteID, affected)
	}

	// 示例2: 条件删除
	fmt.Println("4.2 条件删除:")
	result, err := ormInstance.Delete("users").
		Where("age > ? AND status = ?", 35, "inactive").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("条件删除失败: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取影响行数失败: %w", err)
	}

	fmt.Printf("   ✓ 条件删除成功: 删除了 %d 个35岁以上的非活跃用户\n", affected)

	return nil
}
