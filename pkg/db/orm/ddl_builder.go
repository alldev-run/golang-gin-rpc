package orm

import (
	"context"
	"fmt"
	"strings"
)

// ColumnDef 定义表字段
type ColumnDef struct {
	Name         string
	Type         string
	Length       int      // 用于 varchar/char 等
	Precision    int      // 用于 decimal
	Scale        int      // 用于 decimal
	Nullable     bool
	Default      string
	AutoIncrement bool
	PrimaryKey   bool
	Unique       bool
	Comment      string
}

// IndexDef 定义索引
type IndexDef struct {
	Name    string
	Columns []string
	Unique  bool
}

// TableBuilder 提供建表/删表 DDL 功能
type TableBuilder struct {
	dialect Dialect
	db      DB
	
	// 建表模式
	createMode bool
	tableName  string
	ifNotExists bool
	columns    []ColumnDef
	indexes    []IndexDef
	primaryKeys []string
	
	// 删表模式
	dropMode      bool
	ifExists      bool
	cascade       bool
}

// CreateTable 创建建表构造器
func (o *ORM) CreateTable(table string) *TableBuilder {
	return &TableBuilder{
		dialect:     o.dialect,
		db:          o.db,
		createMode:  true,
		tableName:   table,
		columns:     make([]ColumnDef, 0),
		indexes:     make([]IndexDef, 0),
		primaryKeys: make([]string, 0),
	}
}

// DropTable 创建删表构造器
func (o *ORM) DropTable(table string) *TableBuilder {
	return &TableBuilder{
		dialect:    o.dialect,
		db:         o.db,
		dropMode:   true,
		tableName:  table,
	}
}

// CreateTable 静态函数版本（用于 db helper）
func CreateTable(db DB, dialect Dialect, table string) *TableBuilder {
	if dialect == nil {
		dialect = NewDefaultDialect()
	}
	return &TableBuilder{
		dialect:     dialect,
		db:          db,
		createMode:  true,
		tableName:   table,
		columns:     make([]ColumnDef, 0),
		indexes:     make([]IndexDef, 0),
		primaryKeys: make([]string, 0),
	}
}

// DropTable 静态函数版本
func DropTable(db DB, dialect Dialect, table string) *TableBuilder {
	if dialect == nil {
		dialect = NewDefaultDialect()
	}
	return &TableBuilder{
		dialect:    dialect,
		db:         db,
		dropMode:   true,
		tableName:  table,
	}
}

// IfNotExists 添加 IF NOT EXISTS 条件（建表）
func (tb *TableBuilder) IfNotExists() *TableBuilder {
	tb.ifNotExists = true
	return tb
}

// IfExists 添加 IF EXISTS 条件（删表）
func (tb *TableBuilder) IfExists() *TableBuilder {
	tb.ifExists = true
	return tb
}

// Cascade 添加 CASCADE 选项（删表）
func (tb *TableBuilder) Cascade() *TableBuilder {
	tb.cascade = true
	return tb
}

// Column 添加字段定义
func (tb *TableBuilder) Column(def ColumnDef) *TableBuilder {
	tb.columns = append(tb.columns, def)
	if def.PrimaryKey {
		tb.primaryKeys = append(tb.primaryKeys, def.Name)
	}
	return tb
}

// Int 便捷方法：添加整数字段
func (tb *TableBuilder) Int(name string, nullable ...bool) *TableBuilder {
	col := ColumnDef{Name: name, Type: "INT"}
	if len(nullable) > 0 {
		col.Nullable = nullable[0]
	}
	return tb.Column(col)
}

// BigInt 便捷方法：添加 BIGINT 字段
func (tb *TableBuilder) BigInt(name string, nullable ...bool) *TableBuilder {
	col := ColumnDef{Name: name, Type: "BIGINT"}
	if len(nullable) > 0 {
		col.Nullable = nullable[0]
	}
	return tb.Column(col)
}

// Varchar 便捷方法：添加 VARCHAR 字段
func (tb *TableBuilder) Varchar(name string, length int, nullable ...bool) *TableBuilder {
	col := ColumnDef{Name: name, Type: "VARCHAR", Length: length}
	if len(nullable) > 0 {
		col.Nullable = nullable[0]
	}
	return tb.Column(col)
}

// Text 便捷方法：添加 TEXT 字段
func (tb *TableBuilder) Text(name string, nullable ...bool) *TableBuilder {
	col := ColumnDef{Name: name, Type: "TEXT"}
	if len(nullable) > 0 {
		col.Nullable = nullable[0]
	}
	return tb.Column(col)
}

// Boolean 便捷方法：添加 BOOLEAN 字段
func (tb *TableBuilder) Boolean(name string, nullable ...bool) *TableBuilder {
	col := ColumnDef{Name: name, Type: "BOOLEAN"}
	if len(nullable) > 0 {
		col.Nullable = nullable[0]
	}
	return tb.Column(col)
}

// DateTime 便捷方法：添加 DATETIME 字段
func (tb *TableBuilder) DateTime(name string, nullable ...bool) *TableBuilder {
	col := ColumnDef{Name: name, Type: "DATETIME"}
	if len(nullable) > 0 {
		col.Nullable = nullable[0]
	}
	return tb.Column(col)
}

// Timestamp 便捷方法：添加 TIMESTAMP 字段
func (tb *TableBuilder) Timestamp(name string, nullable ...bool) *TableBuilder {
	col := ColumnDef{Name: name, Type: "TIMESTAMP"}
	if len(nullable) > 0 {
		col.Nullable = nullable[0]
	}
	return tb.Column(col)
}

// Decimal 便捷方法：添加 DECIMAL 字段
func (tb *TableBuilder) Decimal(name string, precision, scale int, nullable ...bool) *TableBuilder {
	col := ColumnDef{Name: name, Type: "DECIMAL", Precision: precision, Scale: scale}
	if len(nullable) > 0 {
		col.Nullable = nullable[0]
	}
	return tb.Column(col)
}

// ID 便捷方法：添加自增主键
func (tb *TableBuilder) ID(name string) *TableBuilder {
	return tb.Column(ColumnDef{
		Name:          name,
		Type:          "BIGINT",
		AutoIncrement: true,
		PrimaryKey:    true,
		Nullable:      false,
	})
}

// PrimaryKey 设置主键（复合主键）
func (tb *TableBuilder) PrimaryKey(columns ...string) *TableBuilder {
	tb.primaryKeys = columns
	return tb
}

// Index 添加索引
func (tb *TableBuilder) Index(name string, columns ...string) *TableBuilder {
	tb.indexes = append(tb.indexes, IndexDef{
		Name:    name,
		Columns: columns,
		Unique:  false,
	})
	return tb
}

// UniqueIndex 添加唯一索引
func (tb *TableBuilder) UniqueIndex(name string, columns ...string) *TableBuilder {
	tb.indexes = append(tb.indexes, IndexDef{
		Name:    name,
		Columns: columns,
		Unique:  true,
	})
	return tb
}

// Build 生成 SQL 语句
func (tb *TableBuilder) Build() (string, error) {
	if tb.createMode {
		return tb.buildCreate()
	}
	if tb.dropMode {
		return tb.buildDrop()
	}
	return "", fmt.Errorf("no operation mode set")
}

func (tb *TableBuilder) buildCreate() (string, error) {
	if len(tb.columns) == 0 {
		return "", fmt.Errorf("no columns defined for table %s", tb.tableName)
	}
	
	var sb strings.Builder
	sb.WriteString("CREATE TABLE ")
	
	if tb.ifNotExists {
		sb.WriteString("IF NOT EXISTS ")
	}
	
	sb.WriteString(tb.dialect.QuoteIdentifier(tb.tableName))
	sb.WriteString(" (")
	
	// 字段定义
	for i, col := range tb.columns {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(tb.buildColumnDef(col))
	}
	
	// 主键约束
	if len(tb.primaryKeys) > 0 {
		sb.WriteString(", PRIMARY KEY (")
		for i, pk := range tb.primaryKeys {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(tb.dialect.QuoteIdentifier(pk))
		}
		sb.WriteString(")")
	}
	
	// 索引（MySQL 支持在建表时定义索引）
	if _, ok := tb.dialect.(*MySQLDialect); ok {
		for _, idx := range tb.indexes {
			sb.WriteString(", ")
			if idx.Unique {
				sb.WriteString("UNIQUE ")
			}
			sb.WriteString("INDEX ")
			sb.WriteString(tb.dialect.QuoteIdentifier(idx.Name))
			sb.WriteString(" (")
			for i, col := range idx.Columns {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(tb.dialect.QuoteIdentifier(col))
			}
			sb.WriteString(")")
		}
	}
	
	sb.WriteString(")")
	
	// MySQL 表注释
	if _, ok := tb.dialect.(*MySQLDialect); ok {
		sb.WriteString(" ENGINE=InnoDB DEFAULT CHARSET=utf8mb4")
	}
	
	return sb.String(), nil
}

func (tb *TableBuilder) buildColumnDef(col ColumnDef) string {
	var sb strings.Builder
	
	sb.WriteString(tb.dialect.QuoteIdentifier(col.Name))
	sb.WriteString(" ")
	
	// 类型处理
	typeStr := tb.buildColumnType(col)
	sb.WriteString(typeStr)
	
	// NULL/NOT NULL
	if !col.Nullable {
		sb.WriteString(" NOT NULL")
	} else {
		sb.WriteString(" NULL")
	}
	
	// 默认值
	if col.Default != "" {
		sb.WriteString(" DEFAULT ")
		sb.WriteString(col.Default)
	}
	
	// 自增
	if col.AutoIncrement {
		if _, ok := tb.dialect.(*MySQLDialect); ok {
			sb.WriteString(" AUTO_INCREMENT")
		} else if _, ok := tb.dialect.(*PostgreSQLDialect); ok {
			sb.WriteString(" GENERATED ALWAYS AS IDENTITY")
		}
	}
	
	// 注释（MySQL）
	if col.Comment != "" {
		if _, ok := tb.dialect.(*MySQLDialect); ok {
			sb.WriteString(fmt.Sprintf(" COMMENT '%s'", col.Comment))
		}
	}
	
	return sb.String()
}

func (tb *TableBuilder) buildColumnType(col ColumnDef) string {
	typeName := strings.ToUpper(col.Type)
	
	switch typeName {
	case "VARCHAR":
		length := col.Length
		if length <= 0 {
			length = 255
		}
		return fmt.Sprintf("VARCHAR(%d)", length)
	case "CHAR":
		length := col.Length
		if length <= 0 {
			length = 1
		}
		return fmt.Sprintf("CHAR(%d)", length)
	case "DECIMAL":
		precision := col.Precision
		if precision <= 0 {
			precision = 10
		}
		scale := col.Scale
		if scale < 0 {
			scale = 0
		}
		return fmt.Sprintf("DECIMAL(%d,%d)", precision, scale)
	case "BOOLEAN":
		if _, ok := tb.dialect.(*MySQLDialect); ok {
			return "TINYINT(1)"
		}
		return "BOOLEAN"
	case "DATETIME", "TIMESTAMP":
		if _, ok := tb.dialect.(*PostgreSQLDialect); ok {
			// PostgreSQL uses TIMESTAMP instead of DATETIME
			return "TIMESTAMP WITH TIME ZONE"
		}
		return typeName
	default:
		return typeName
	}
}

func (tb *TableBuilder) buildDrop() (string, error) {
	var sb strings.Builder
	sb.WriteString("DROP TABLE ")
	
	if tb.ifExists {
		sb.WriteString("IF EXISTS ")
	}
	
	sb.WriteString(tb.dialect.QuoteIdentifier(tb.tableName))
	
	if tb.cascade {
		sb.WriteString(" CASCADE")
	}
	
	return sb.String(), nil
}

// Exec 执行 DDL 语句
func (tb *TableBuilder) Exec(ctx context.Context) error {
	query, err := tb.Build()
	if err != nil {
		return err
	}
	_, err = tb.db.Exec(ctx, query)
	return err
}

// CreateIndex 创建单独的索引（用于 PostgreSQL 或后续添加索引）
func (o *ORM) CreateIndex(table, name string, unique bool, columns ...string) error {
	var sb strings.Builder
	
	if unique {
		sb.WriteString("CREATE UNIQUE INDEX ")
	} else {
		sb.WriteString("CREATE INDEX ")
	}
	
	sb.WriteString(o.dialect.QuoteIdentifier(name))
	sb.WriteString(" ON ")
	sb.WriteString(o.dialect.QuoteIdentifier(table))
	sb.WriteString(" (")
	
	for i, col := range columns {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(o.dialect.QuoteIdentifier(col))
	}
	sb.WriteString(")")
	
	_, err := o.db.Exec(context.Background(), sb.String())
	return err
}

// DropIndex 删除索引
func (o *ORM) DropIndex(table, name string) error {
	var query string
	
	if _, ok := o.dialect.(*MySQLDialect); ok {
		// MySQL: DROP INDEX name ON table
		query = fmt.Sprintf("DROP INDEX %s ON %s",
			o.dialect.QuoteIdentifier(name),
			o.dialect.QuoteIdentifier(table))
	} else {
		// PostgreSQL: DROP INDEX name
		query = fmt.Sprintf("DROP INDEX %s", o.dialect.QuoteIdentifier(name))
	}
	
	_, err := o.db.Exec(context.Background(), query)
	return err
}

// HasTable 检查表是否存在
func (o *ORM) HasTable(ctx context.Context, table string) (bool, error) {
	var query string
	var args []interface{}
	
	if _, ok := o.dialect.(*MySQLDialect); ok {
		query = "SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?"
		args = []interface{}{table}
	} else if _, ok := o.dialect.(*PostgreSQLDialect); ok {
		query = "SELECT 1 FROM pg_tables WHERE tablename = $1"
		args = []interface{}{table}
	} else {
		// 通用方法，可能不适用于所有数据库
		query = fmt.Sprintf("SELECT 1 FROM %s LIMIT 1", o.dialect.QuoteIdentifier(table))
		args = nil
	}
	
	rows, err := o.db.Query(ctx, query, args...)
	if err != nil {
		return false, nil // 表不存在或查询失败
	}
	defer rows.Close()
	
	return rows.Next(), nil
}

// TruncateTable 清空表
func (o *ORM) TruncateTable(ctx context.Context, table string, cascade ...bool) error {
	var sb strings.Builder
	sb.WriteString("TRUNCATE TABLE ")
	sb.WriteString(o.dialect.QuoteIdentifier(table))
	
	if len(cascade) > 0 && cascade[0] {
		sb.WriteString(" CASCADE")
	}
	
	_, err := o.db.Exec(ctx, sb.String())
	return err
}
