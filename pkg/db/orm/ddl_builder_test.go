package orm

import (
	"testing"
)

func TestTableBuilder_CreateTable(t *testing.T) {
	tests := []struct {
		name     string
		builder  *TableBuilder
		expected string
	}{
		{
			name: "basic table with id and name",
			builder: CreateTable(nil, NewMySQLDialect(), "users").
				ID("id").
				Varchar("name", 100),
			expected: "CREATE TABLE `users` (`id` BIGINT NOT NULL AUTO_INCREMENT, `name` VARCHAR(100) NOT NULL, PRIMARY KEY (`id`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
		},
		{
			name: "with if not exists",
			builder: CreateTable(nil, NewMySQLDialect(), "products").
				IfNotExists().
				ID("id").
				Varchar("title", 255).
				Decimal("price", 10, 2),
			expected: "CREATE TABLE IF NOT EXISTS `products` (`id` BIGINT NOT NULL AUTO_INCREMENT, `title` VARCHAR(255) NOT NULL, `price` DECIMAL(10,2) NOT NULL, PRIMARY KEY (`id`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
		},
		{
			name: "with index",
			builder: CreateTable(nil, NewMySQLDialect(), "orders").
				ID("id").
				Int("user_id").
				Decimal("total", 12, 2).
				Index("idx_user_id", "user_id"),
			expected: "CREATE TABLE `orders` (`id` BIGINT NOT NULL AUTO_INCREMENT, `user_id` INT NOT NULL, `total` DECIMAL(12,2) NOT NULL, PRIMARY KEY (`id`), INDEX `idx_user_id` (`user_id`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
		},
		{
			name: "postgresql simple table",
			builder: CreateTable(nil, NewPostgreSQLDialect(), "users").
				ID("id").
				Varchar("email", 255),
			expected: `CREATE TABLE "users" ("id" BIGINT NOT NULL GENERATED ALWAYS AS IDENTITY, "email" VARCHAR(255) NOT NULL, PRIMARY KEY ("id"))`,
		},
		{
			name: "multiple columns with constraints",
			builder: CreateTable(nil, NewMySQLDialect(), "posts").
				ID("id").
				Varchar("title", 200).
				Text("content").
				Int("author_id").
				DateTime("created_at").
				UniqueIndex("idx_title", "title"),
			expected: "CREATE TABLE `posts` (`id` BIGINT NOT NULL AUTO_INCREMENT, `title` VARCHAR(200) NOT NULL, `content` TEXT NOT NULL, `author_id` INT NOT NULL, `created_at` DATETIME NOT NULL, PRIMARY KEY (`id`), UNIQUE INDEX `idx_title` (`title`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.builder.Build()
			if err != nil {
				t.Fatalf("Build() error = %v", err)
			}
			if got != tt.expected {
				t.Errorf("Build() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTableBuilder_DropTable(t *testing.T) {
	tests := []struct {
		name     string
		builder  *TableBuilder
		expected string
	}{
		{
			name:     "simple drop",
			builder:  DropTable(nil, NewMySQLDialect(), "users"),
			expected: "DROP TABLE `users`",
		},
		{
			name:     "drop if exists",
			builder:  DropTable(nil, NewMySQLDialect(), "products").IfExists(),
			expected: "DROP TABLE IF EXISTS `products`",
		},
		{
			name:     "drop with cascade",
			builder:  DropTable(nil, NewPostgreSQLDialect(), "orders").IfExists().Cascade(),
			expected: `DROP TABLE IF EXISTS "orders" CASCADE`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.builder.Build()
			if err != nil {
				t.Fatalf("Build() error = %v", err)
			}
			if got != tt.expected {
				t.Errorf("Build() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTableBuilder_ColumnTypes(t *testing.T) {
	dialect := NewMySQLDialect()
	
	tests := []struct {
		name     string
		col      ColumnDef
		expected string
	}{
		{
			name:     "varchar with length",
			col:      ColumnDef{Name: "name", Type: "VARCHAR", Length: 100, Nullable: true},
			expected: "`name` VARCHAR(100) NULL",
		},
		{
			name:     "varchar default length",
			col:      ColumnDef{Name: "code", Type: "VARCHAR", Nullable: true},
			expected: "`code` VARCHAR(255) NULL",
		},
		{
			name:     "int not null",
			col:      ColumnDef{Name: "age", Type: "INT", Nullable: false},
			expected: "`age` INT NOT NULL",
		},
		{
			name:     "decimal with precision",
			col:      ColumnDef{Name: "amount", Type: "DECIMAL", Precision: 10, Scale: 2, Nullable: true},
			expected: "`amount` DECIMAL(10,2) NULL",
		},
		{
			name:     "boolean as tinyint",
			col:      ColumnDef{Name: "active", Type: "BOOLEAN", Nullable: true},
			expected: "`active` TINYINT(1) NULL",
		},
		{
			name:     "with default",
			col:      ColumnDef{Name: "status", Type: "VARCHAR", Length: 20, Default: "'active'", Nullable: true},
			expected: "`status` VARCHAR(20) NULL DEFAULT 'active'",
		},
		{
			name:     "with comment",
			col:      ColumnDef{Name: "title", Type: "VARCHAR", Length: 200, Comment: "文章标题", Nullable: true},
			expected: "`title` VARCHAR(200) NULL COMMENT '文章标题'",
		},
	}

	tb := &TableBuilder{dialect: dialect}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tb.buildColumnDef(tt.col)
			if got != tt.expected {
				t.Errorf("buildColumnDef() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTableBuilder_PostgreSQLTypes(t *testing.T) {
	dialect := NewPostgreSQLDialect()
	
	tests := []struct {
		name     string
		col      ColumnDef
		expected string
	}{
		{
			name:     "boolean in postgres",
			col:      ColumnDef{Name: "active", Type: "BOOLEAN", Nullable: true},
			expected: `"active" BOOLEAN NULL`,
		},
		{
			name:     "timestamp with timezone",
			col:      ColumnDef{Name: "created_at", Type: "TIMESTAMP", Nullable: true},
			expected: `"created_at" TIMESTAMP WITH TIME ZONE NULL`,
		},
		{
			name:     "datetime with timezone",
			col:      ColumnDef{Name: "updated_at", Type: "DATETIME", Nullable: true},
			expected: `"updated_at" TIMESTAMP WITH TIME ZONE NULL`,
		},
	}

	tb := &TableBuilder{dialect: dialect}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tb.buildColumnDef(tt.col)
			if got != tt.expected {
				t.Errorf("buildColumnDef() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTableBuilder_NoColumnsError(t *testing.T) {
	tb := CreateTable(nil, NewMySQLDialect(), "empty_table")
	_, err := tb.Build()
	if err == nil {
		t.Error("expected error for table with no columns, got nil")
	}
}

func TestTableBuilder_CompositePrimaryKey(t *testing.T) {
	tb := CreateTable(nil, NewMySQLDialect(), "order_items").
		Int("order_id", true).
		Int("product_id", true).
		Int("quantity", true).
		PrimaryKey("order_id", "product_id")

	got, err := tb.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	expected := "CREATE TABLE `order_items` (`order_id` INT NULL, `product_id` INT NULL, `quantity` INT NULL, PRIMARY KEY (`order_id`, `product_id`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4"
	if got != expected {
		t.Errorf("Build() = %v, want %v", got, expected)
	}
}
