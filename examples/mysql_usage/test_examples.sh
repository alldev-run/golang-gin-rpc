#!/bin/bash

echo "=== 测试 ORM 示例文件编译 ==="

echo "1. 测试基础 CRUD 操作示例..."
go build -o /dev/null ./orm_crud_operations.go
if [ $? -eq 0 ]; then
    echo "   ✓ orm_crud_operations.go 编译成功"
else
    echo "   ✗ orm_crud_operations.go 编译失败"
fi

echo "2. 测试高级操作示例..."
go build -o /dev/null ./orm_advanced_operations.go
if [ $? -eq 0 ]; then
    echo "   ✓ orm_advanced_operations.go 编译成功"
else
    echo "   ✗ orm_advanced_operations.go 编译失败"
fi

echo "3. 测试 MySQL 集成示例..."
go build -o /dev/null ./main_fixed.go
if [ $? -eq 0 ]; then
    echo "   ✓ main_fixed.go 编译成功"
else
    echo "   ✗ main_fixed.go 编译失败"
fi

echo "4. 检查示例文件列表..."
ls -la *.go 2>/dev/null | while read line; do
    echo "   $line"
done

echo "=== 编译测试完成 ==="

echo ""
echo "=== 运行说明 ==="
echo "要运行示例，请确保："
echo "1. MySQL 服务正在运行"
echo "2. 创建测试数据库: CREATE DATABASE test_db;"
echo "3. 修改连接配置（如需要）"
echo ""
echo "运行命令："
echo "go run orm_crud_operations.go      # 基础 CRUD"
echo "go run orm_advanced_operations.go  # 高级操作"
echo "go run main_fixed.go               # MySQL 集成"
echo ""
echo "查看完整文档："
echo "docs/orm-usage-guide.md"
