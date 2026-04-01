package orm

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Test Builder core functionality
func TestNewBuilder(t *testing.T) {
	ctx := context.Background()
	b := NewBuilder(ctx, "users")

	if b.table != "users" {
		t.Errorf("Expected table='users', got='%s'", b.table)
	}

	if b.ctx != ctx {
		t.Error("Expected builder to use provided context")
	}
}

func TestBuilderNilContext(t *testing.T) {
	b := NewBuilder(nil, "users")

	if b.ctx == nil {
		t.Error("Expected context to be set even when nil provided")
	}
}

func TestBuilderWhere(t *testing.T) {
	b := NewBuilder(context.Background(), "users").
		Where("status = ?", "active").
		Where("age > ?", 18)

	if len(b.where) != 2 {
		t.Errorf("Expected 2 where conditions, got %d", len(b.where))
	}

	if len(b.args) != 2 {
		t.Errorf("Expected 2 args, got %d", len(b.args))
	}
}

func TestBuilderOrderBy(t *testing.T) {
	b := NewBuilder(context.Background(), "users").
		OrderBy("created_at DESC").
		OrderBy("id ASC")

	if len(b.orderBy) != 2 {
		t.Errorf("Expected 2 order conditions, got %d", len(b.orderBy))
	}
}

func TestBuilderLimit(t *testing.T) {
	b := NewBuilder(context.Background(), "users").
		Limit(10)

	if b.limit == nil || *b.limit != 10 {
		t.Error("Expected limit to be 10")
	}
}

func TestBuilderOffset(t *testing.T) {
	b := NewBuilder(context.Background(), "users").
		Offset(20)

	if b.offset == nil || *b.offset != 20 {
		t.Error("Expected offset to be 20")
	}
}

func TestBuilderBuild(t *testing.T) {
	tests := []struct {
		name         string
		builder      *Builder
		expectedSQL  string
		expectedArgs []interface{}
	}{
		{
			name:         "simple select",
			builder:      NewBuilder(context.Background(), "users"),
			expectedSQL:  "SELECT * FROM users",
			expectedArgs: nil,
		},
		{
			name: "with where",
			builder: NewBuilder(context.Background(), "users").
				Where("status = ?", "active"),
			expectedSQL:  "SELECT * FROM users WHERE status = ?",
			expectedArgs: []interface{}{"active"},
		},
		{
			name: "with multiple where",
			builder: NewBuilder(context.Background(), "users").
				Where("status = ?", "active").
				Where("age > ?", 18),
			expectedSQL:  "SELECT * FROM users WHERE status = ? AND age > ?",
			expectedArgs: []interface{}{"active", 18},
		},
		{
			name: "with order by",
			builder: NewBuilder(context.Background(), "users").
				OrderBy("created_at DESC"),
			expectedSQL:  "SELECT * FROM users ORDER BY created_at DESC",
			expectedArgs: nil,
		},
		{
			name: "with limit",
			builder: NewBuilder(context.Background(), "users").
				Limit(10),
			expectedSQL:  "SELECT * FROM users LIMIT 10",
			expectedArgs: nil,
		},
		{
			name: "with offset",
			builder: NewBuilder(context.Background(), "users").
				Offset(20),
			expectedSQL:  "SELECT * FROM users OFFSET 20",
			expectedArgs: nil,
		},
		{
			name: "with limit and offset",
			builder: NewBuilder(context.Background(), "users").
				Limit(10).
				Offset(20),
			expectedSQL:  "SELECT * FROM users LIMIT 10 OFFSET 20",
			expectedArgs: nil,
		},
		{
			name: "full query",
			builder: NewBuilder(context.Background(), "users").
				Where("status = ?", "active").
				Where("age > ?", 18).
				OrderBy("created_at DESC").
				Limit(10).
				Offset(20),
			expectedSQL:  "SELECT * FROM users WHERE status = ? AND age > ? ORDER BY created_at DESC LIMIT 10 OFFSET 20",
			expectedArgs: []interface{}{"active", 18},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args := tt.builder.Build()

			if sql != tt.expectedSQL {
				t.Errorf("Expected SQL='%s', got='%s'", tt.expectedSQL, sql)
			}

			if !reflect.DeepEqual(args, tt.expectedArgs) {
				t.Errorf("Expected args=%v, got=%v", tt.expectedArgs, args)
			}
		})
	}
}

// Test Scope functionality
func TestScope(t *testing.T) {
	ctx := context.Background()

	// Create a custom scope
	customScope := func(c context.Context, b *Builder) *Builder {
		return b.Where("custom = ?", "value")
	}

	builder := NewBuilder(ctx, "users").
		Scope(customScope)

	if len(builder.scopes) != 1 {
		t.Errorf("Expected 1 scope, got %d", len(builder.scopes))
	}

	// Apply and verify
	builder.ApplyScopes()
	sql, args := builder.Build()

	if sql != "SELECT * FROM users WHERE custom = ?" {
		t.Errorf("Unexpected SQL: %s", sql)
	}

	if len(args) != 1 || args[0] != "value" {
		t.Errorf("Unexpected args: %v", args)
	}
}

func TestRouting(t *testing.T) {
	ctx := context.Background()

	// Create a routing scope
	routingScope := func(c context.Context, b *Builder) *Builder {
		return b.Table("users_0")
	}

	builder := NewBuilder(ctx, "users").
		Routing(routingScope)

	if len(builder.scopes) != 1 {
		t.Errorf("Expected 1 scope, got %d", len(builder.scopes))
	}

	if builder.scopes[0].Type != ScopeRouting {
		t.Errorf("Expected scope type to be ScopeRouting, got %v", builder.scopes[0].Type)
	}
}

func TestMeta(t *testing.T) {
	ctx := context.Background()

	// Create a meta scope
	metaScope := func(c context.Context, b *Builder) *Builder {
		return b
	}

	builder := NewBuilder(ctx, "users").
		Meta(metaScope)

	if len(builder.scopes) != 1 {
		t.Errorf("Expected 1 scope, got %d", len(builder.scopes))
	}

	if builder.scopes[0].Type != ScopeMeta {
		t.Errorf("Expected scope type to be ScopeMeta, got %v", builder.scopes[0].Type)
	}
}

func TestAdd(t *testing.T) {
	ctx := context.Background()

	namedScope := Named("testScope", ScopeQuery, func(c context.Context, b *Builder) *Builder {
		return b.Where("test = ?", true)
	})

	builder := NewBuilder(ctx, "users").
		Add(namedScope).
		ApplyScopes()

	sql, args := builder.Build()

	if sql != "SELECT * FROM users WHERE test = ?" {
		t.Errorf("Unexpected SQL: %s", sql)
	}

	if len(args) != 1 || args[0] != true {
		t.Errorf("Unexpected args: %v", args)
	}

	// Check applied scopes tracking
	applied := builder.AppliedScopes()
	if len(applied) != 1 || applied[0] != "testScope" {
		t.Errorf("Expected applied scope 'testScope', got %v", applied)
	}
}

// Test Predefined Scopes
func TestActive(t *testing.T) {
	ctx := context.Background()
	builder := NewBuilder(ctx, "users").
		Scope(Active("status")).
		ApplyScopes()

	sql, args := builder.Build()

	expectedSQL := "SELECT * FROM users WHERE status = ?"
	if sql != expectedSQL {
		t.Errorf("Expected SQL='%s', got='%s'", expectedSQL, sql)
	}

	if len(args) != 1 || args[0] != "active" {
		t.Errorf("Expected args=['active'], got=%v", args)
	}
}

func TestNotDeleted(t *testing.T) {
	ctx := context.Background()
	builder := NewBuilder(ctx, "users").
		Scope(NotDeleted()).
		ApplyScopes()

	sql, args := builder.Build()

	expectedSQL := "SELECT * FROM users WHERE deleted_at IS NULL"
	if sql != expectedSQL {
		t.Errorf("Expected SQL='%s', got='%s'", expectedSQL, sql)
	}

	if len(args) != 0 {
		t.Errorf("Expected no args, got=%v", args)
	}
}

func TestPaginate(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		page        int
		size        int
		expectedSQL string
	}{
		{
			name:        "normal page",
			page:        2,
			size:        10,
			expectedSQL: "SELECT * FROM users LIMIT 10 OFFSET 10",
		},
		{
			name:        "first page",
			page:        1,
			size:        20,
			expectedSQL: "SELECT * FROM users LIMIT 20 OFFSET 0",
		},
		{
			name:        "invalid page defaults to 1",
			page:        0,
			size:        10,
			expectedSQL: "SELECT * FROM users LIMIT 10 OFFSET 0",
		},
		{
			name:        "negative page defaults to 1",
			page:        -5,
			size:        10,
			expectedSQL: "SELECT * FROM users LIMIT 10 OFFSET 0",
		},
		{
			name:        "zero size defaults to 10",
			page:        1,
			size:        0,
			expectedSQL: "SELECT * FROM users LIMIT 10 OFFSET 0",
		},
		{
			name:        "negative size defaults to 10",
			page:        1,
			size:        -5,
			expectedSQL: "SELECT * FROM users LIMIT 10 OFFSET 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewBuilder(ctx, "users").
				Scope(Paginate(tt.page, tt.size)).
				ApplyScopes()

			sql, args := builder.Build()

			if sql != tt.expectedSQL {
				t.Errorf("Expected SQL='%s', got='%s'", tt.expectedSQL, sql)
			}

			if len(args) != 0 {
				t.Errorf("Expected no args, got=%v", args)
			}
		})
	}
}

func TestOrderByDesc(t *testing.T) {
	ctx := context.Background()
	builder := NewBuilder(ctx, "users").
		Scope(OrderByDesc("created_at")).
		ApplyScopes()

	sql, args := builder.Build()

	expectedSQL := "SELECT * FROM users ORDER BY created_at DESC"
	if sql != expectedSQL {
		t.Errorf("Expected SQL='%s', got='%s'", expectedSQL, sql)
	}

	if len(args) != 0 {
		t.Errorf("Expected no args, got=%v", args)
	}
}

func TestCreatedAfter(t *testing.T) {
	ctx := context.Background()
	testTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	builder := NewBuilder(ctx, "users").
		Scope(CreatedAfter(testTime)).
		ApplyScopes()

	sql, args := builder.Build()

	expectedSQL := "SELECT * FROM users WHERE created_at >= ?"
	if sql != expectedSQL {
		t.Errorf("Expected SQL='%s', got='%s'", expectedSQL, sql)
	}

	if len(args) != 1 || !reflect.DeepEqual(args[0], testTime) {
		t.Errorf("Expected args=[time], got=%v", args)
	}
}

// Test Routing Scopes
func TestHashTable(t *testing.T) {
	ctx := context.Background()
	builder := NewBuilder(ctx, "orders").
		Scope(HashTable("orders", 42, 8)).
		ApplyScopes()

	sql, _ := builder.Build()

	// 42 % 8 = 2, so should route to orders_2
	expectedSQL := "SELECT * FROM orders_2"
	if sql != expectedSQL {
		t.Errorf("Expected SQL='%s', got='%s'", expectedSQL, sql)
	}
}

func TestShardByUser(t *testing.T) {
	ctx := context.Background()
	builder := NewBuilder(ctx, "orders").
		Scope(ShardByUser(12345)).
		ApplyScopes()

	sql, args := builder.Build()

	expectedSQL := "SELECT * FROM orders WHERE user_id = ?"
	if sql != expectedSQL {
		t.Errorf("Expected SQL='%s', got='%s'", expectedSQL, sql)
	}

	if len(args) != 1 || args[0] != int64(12345) {
		t.Errorf("Expected args=[12345], got=%v", args)
	}
}

// Test Trace (Meta Scope)
func TestTrace(t *testing.T) {
	ctx := context.Background()

	// Trace should not modify the query
	builder := NewBuilder(ctx, "users").
		Scope(Trace("test-trace")).
		ApplyScopes()

	sql, args := builder.Build()

	expectedSQL := "SELECT * FROM users"
	if sql != expectedSQL {
		t.Errorf("Expected SQL='%s', got='%s'", expectedSQL, sql)
	}

	if len(args) != 0 {
		t.Errorf("Expected no args, got=%v", args)
	}
}

// Test Helpers
func TestCompose(t *testing.T) {
	ctx := context.Background()

	scope1 := func(c context.Context, b *Builder) *Builder {
		return b.Where("condition1 = ?", 1)
	}

	scope2 := func(c context.Context, b *Builder) *Builder {
		return b.Where("condition2 = ?", 2)
	}

	scope3 := func(c context.Context, b *Builder) *Builder {
		return b.OrderBy("id DESC")
	}

	builder := NewBuilder(ctx, "users").
		Scope(Compose(scope1, scope2, scope3)).
		ApplyScopes()

	sql, args := builder.Build()

	// Note: The order of where conditions may vary
	if sql != "SELECT * FROM users WHERE condition1 = ? AND condition2 = ? ORDER BY id DESC" {
		t.Errorf("Unexpected SQL: %s", sql)
	}

	if len(args) != 2 {
		t.Errorf("Expected 2 args, got=%v", args)
	}
}

func TestIf(t *testing.T) {
	ctx := context.Background()

	activeScope := func(c context.Context, b *Builder) *Builder {
		return b.Where("active = ?", true)
	}

	tests := []struct {
		name        string
		condition   bool
		expectedSQL string
		expectWhere bool
	}{
		{
			name:        "condition true - apply scope",
			condition:   true,
			expectedSQL: "SELECT * FROM users WHERE active = ?",
			expectWhere: true,
		},
		{
			name:        "condition false - skip scope",
			condition:   false,
			expectedSQL: "SELECT * FROM users",
			expectWhere: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewBuilder(ctx, "users").
				Scope(If(tt.condition, activeScope)).
				ApplyScopes()

			sql, args := builder.Build()

			if sql != tt.expectedSQL {
				t.Errorf("Expected SQL='%s', got='%s'", tt.expectedSQL, sql)
			}

			if tt.expectWhere && len(args) != 1 {
				t.Errorf("Expected 1 arg, got=%v", args)
			}

			if !tt.expectWhere && len(args) != 0 {
				t.Errorf("Expected 0 args, got=%v", args)
			}
		})
	}
}

func TestIfNotZero(t *testing.T) {
	ctx := context.Background()

	userIDScope := func(id int64) Scope {
		return func(c context.Context, b *Builder) *Builder {
			return b.Where("user_id = ?", id)
		}
	}

	tests := []struct {
		name        string
		value       int64
		expectedSQL string
		expectWhere bool
	}{
		{
			name:        "non-zero value - apply scope",
			value:       123,
			expectedSQL: "SELECT * FROM users WHERE user_id = ?",
			expectWhere: true,
		},
		{
			name:        "zero value - skip scope",
			value:       0,
			expectedSQL: "SELECT * FROM users",
			expectWhere: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewBuilder(ctx, "users").
				Scope(IfNotZero(tt.value, userIDScope)).
				ApplyScopes()

			sql, args := builder.Build()

			if sql != tt.expectedSQL {
				t.Errorf("Expected SQL='%s', got='%s'", tt.expectedSQL, sql)
			}

			if tt.expectWhere && len(args) != 1 {
				t.Errorf("Expected 1 arg, got=%v", args)
			}

			if !tt.expectWhere && len(args) != 0 {
				t.Errorf("Expected 0 args, got=%v", args)
			}
		})
	}
}

// Test Complex Scenarios
func TestScopeChaining(t *testing.T) {
	ctx := context.Background()

	builder := NewBuilder(ctx, "articles").
		Scope(Active("status")).
		Scope(NotDeleted()).
		Scope(OrderByDesc("created_at")).
		Scope(Paginate(1, 10)).
		ApplyScopes()

	sql, args := builder.Build()

	expectedSQL := "SELECT * FROM articles WHERE status = ? AND deleted_at IS NULL ORDER BY created_at DESC LIMIT 10 OFFSET 0"
	if sql != expectedSQL {
		t.Errorf("Unexpected SQL: %s", sql)
	}

	if len(args) != 1 || args[0] != "active" {
		t.Errorf("Expected args=['active'], got=%v", args)
	}
}

func TestScopeExecutionOrder(t *testing.T) {
	ctx := context.Background()

	// Routing scopes should execute before query scopes
	builder := NewBuilder(ctx, "data").
		Routing(func(c context.Context, b *Builder) *Builder {
			return b.Table("data_0")
		}).
		Scope(func(c context.Context, b *Builder) *Builder {
			return b.Where("value = ?", 100)
		}).
		ApplyScopes()

	sql, args := builder.Build()

	// Should route first, then apply where
	expectedSQL := "SELECT * FROM data_0 WHERE value = ?"
	if sql != expectedSQL {
		t.Errorf("Expected SQL='%s', got='%s'", expectedSQL, sql)
	}

	if len(args) != 1 || args[0] != 100 {
		t.Errorf("Unexpected args: %v", args)
	}
}

func TestNilScopeReturn(t *testing.T) {
	ctx := context.Background()

	// Scope that returns nil should not modify builder
	nilScope := func(c context.Context, b *Builder) *Builder {
		return nil
	}

	builder := NewBuilder(ctx, "users").
		Where("status = ?", "active").
		Scope(nilScope).
		ApplyScopes()

	sql, args := builder.Build()

	// Should still have the where condition
	expectedSQL := "SELECT * FROM users WHERE status = ?"
	if sql != expectedSQL {
		t.Errorf("Expected SQL='%s', got='%s'", expectedSQL, sql)
	}

	if len(args) != 1 || args[0] != "active" {
		t.Errorf("Unexpected args: %v", args)
	}
}

func TestBuilderScopeRegistration(t *testing.T) {
	ctx := context.Background()

	// Original builder
	builder := NewBuilder(ctx, "users").
		Where("base = ?", "value")

	// Apply scope registration - note: Scope() only registers, doesn't execute
	builder.Scope(func(c context.Context, b *Builder) *Builder {
		return b.Where("extra = ?", "extra_value")
	})

	// Before ApplyScopes: only 1 where condition (the original)
	if len(builder.where) != 1 {
		t.Errorf("Before ApplyScopes: expected 1 where condition, got %d", len(builder.where))
	}

	// After ApplyScopes: 2 where conditions
	builder.ApplyScopes()
	if len(builder.where) != 2 {
		t.Errorf("After ApplyScopes: expected 2 where conditions, got %d", len(builder.where))
	}
}

// Test Combined Usage
func TestCombinedScopes(t *testing.T) {
	ctx := context.Background()

	// Simulate a real-world query: active, not deleted users, paginated
	page := 2
	pageSize := 20
	status := "active"

	builder := NewBuilder(ctx, "users").
		Scope(Active("status")).
		Scope(NotDeleted()).
		Scope(OrderByDesc("created_at")).
		Scope(Paginate(page, pageSize)).
		ApplyScopes()

	sql, args := builder.Build()

	// Should contain all conditions
	expectedConditions := []string{
		"SELECT * FROM users",
		"status = ?",
		"deleted_at IS NULL",
		"ORDER BY created_at DESC",
		"LIMIT",
		"OFFSET",
	}

	for _, condition := range expectedConditions {
		if !strings.Contains(sql, condition) {
			t.Errorf("SQL should contain '%s', got: %s", condition, sql)
		}
	}

	// Should have exactly one arg (status = "active")
	if len(args) != 1 || args[0] != status {
		t.Errorf("Expected args=['%s'], got=%v", status, args)
	}
}

// Benchmarks
func BenchmarkBuilderBuild(b *testing.B) {
	ctx := context.Background()
	builder := NewBuilder(ctx, "users").
		Where("status = ?", "active").
		Where("age > ?", 18).
		OrderBy("created_at DESC").
		Limit(10).
		Offset(20)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		builder.Build()
	}
}

func BenchmarkScopeApplication(b *testing.B) {
	ctx := context.Background()

	for i := 0; i < b.N; i++ {
		builder := NewBuilder(ctx, "users").
			Scope(Active("status")).
			Scope(NotDeleted()).
			Scope(OrderByDesc("created_at")).
			Scope(Paginate(1, 10)).
			ApplyScopes()

		builder.Build()
	}
}
