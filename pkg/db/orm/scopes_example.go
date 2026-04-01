// Package orm provides examples of using scopes with the ORM.
package orm

import (
	"context"
	"fmt"
	"time"
)

// ExampleBuilder demonstrates basic Builder usage.
func ExampleBuilder() {
	ctx := context.Background()

	// Create a basic query
	builder := NewBuilder(ctx, "users")
	sql, args := builder.Build()

	fmt.Println("Simple query:", sql)
	fmt.Println("Args:", args)

	// Output:
	// Simple query: SELECT * FROM users
	// Args: []
}

// ExampleBuilder_where demonstrates adding WHERE conditions.
func ExampleBuilder_where() {
	ctx := context.Background()

	builder := NewBuilder(ctx, "users").
		Where("status = ?", "active").
		Where("age > ?", 18)

	sql, args := builder.Build()

	fmt.Println("Query:", sql)
	fmt.Println("Args:", args)

	// Output:
	// Query: SELECT * FROM users WHERE status = ? AND age > ?
	// Args: [active 18]
}

// ExampleBuilder_orderBy demonstrates ORDER BY.
func ExampleBuilder_orderBy() {
	ctx := context.Background()

	builder := NewBuilder(ctx, "users").
		OrderBy("created_at DESC")

	sql, _ := builder.Build()

	fmt.Println("Query:", sql)

	// Output:
	// Query: SELECT * FROM users ORDER BY created_at DESC
}

// ExampleBuilder_pagination demonstrates LIMIT and OFFSET.
func ExampleBuilder_pagination() {
	ctx := context.Background()

	builder := NewBuilder(ctx, "users").
		Limit(10).
		Offset(20)

	sql, _ := builder.Build()

	fmt.Println("Query:", sql)

	// Output:
	// Query: SELECT * FROM users LIMIT 10 OFFSET 20
}

// ExampleScope demonstrates using a simple scope.
func ExampleScope() {
	ctx := context.Background()

	// Define a custom scope
	activeScope := func(c context.Context, b *Builder) *Builder {
		return b.Where("status = ?", "active")
	}

	builder := NewBuilder(ctx, "users").
		Scope(activeScope).
		ApplyScopes()

	sql, args := builder.Build()

	fmt.Println("Query:", sql)
	fmt.Println("Args:", args)

	// Output:
	// Query: SELECT * FROM users WHERE status = ?
	// Args: [active]
}

// ExampleActive demonstrates the Active scope.
func ExampleActive() {
	ctx := context.Background()

	builder := NewBuilder(ctx, "users").
		Scope(Active("status")).
		ApplyScopes()

	sql, args := builder.Build()

	fmt.Println("Query:", sql)
	fmt.Println("Args:", args)

	// Output:
	// Query: SELECT * FROM users WHERE status = ?
	// Args: [active]
}

// ExampleNotDeleted demonstrates the NotDeleted scope.
func ExampleNotDeleted() {
	ctx := context.Background()

	builder := NewBuilder(ctx, "users").
		Scope(NotDeleted()).
		ApplyScopes()

	sql, _ := builder.Build()

	fmt.Println("Query:", sql)

	// Output:
	// Query: SELECT * FROM users WHERE deleted_at IS NULL
}

// ExamplePaginate demonstrates the Paginate scope.
func ExamplePaginate() {
	ctx := context.Background()

	// Page 2 with 10 items per page
	builder := NewBuilder(ctx, "articles").
		Scope(Paginate(2, 10)).
		ApplyScopes()

	sql, _ := builder.Build()

	fmt.Println("Query:", sql)

	// Output:
	// Query: SELECT * FROM articles LIMIT 10 OFFSET 10
}

// ExamplePaginate_firstPage demonstrates first page pagination.
func ExamplePaginate_firstPage() {
	ctx := context.Background()

	// First page (page=1, size=20)
	builder := NewBuilder(ctx, "articles").
		Scope(Paginate(1, 20)).
		ApplyScopes()

	sql, _ := builder.Build()

	fmt.Println("Query:", sql)

	// Output:
	// Query: SELECT * FROM articles LIMIT 20 OFFSET 0
}

// ExampleOrderByDesc demonstrates ordering by descending.
func ExampleOrderByDesc() {
	ctx := context.Background()

	builder := NewBuilder(ctx, "posts").
		Scope(OrderByDesc("created_at")).
		ApplyScopes()

	sql, _ := builder.Build()

	fmt.Println("Query:", sql)

	// Output:
	// Query: SELECT * FROM posts ORDER BY created_at DESC
}

// ExampleCreatedAfter demonstrates filtering by date.
func ExampleCreatedAfter() {
	ctx := context.Background()
	lastWeek := time.Now().AddDate(0, 0, -7)

	builder := NewBuilder(ctx, "users").
		Scope(CreatedAfter(lastWeek)).
		ApplyScopes()

	sql, _ := builder.Build()

	fmt.Println("Query starts with:", sql[:45])

	// Output:
	// Query starts with: SELECT * FROM users WHERE created_at >=
}

// ExampleHashTable demonstrates table sharding with hash.
func ExampleHashTable() {
	ctx := context.Background()
	userID := int64(12345)
	shardCount := 8

	// Route to shard based on user_id % 8
	builder := NewBuilder(ctx, "orders").
		Scope(HashTable("orders", userID, shardCount)).
		ApplyScopes()

	sql, _ := builder.Build()

	fmt.Println("Query:", sql)

	// 12345 % 8 = 1, so routes to orders_1
	// Output:
	// Query: SELECT * FROM orders_1
}

// ExampleShardByUser demonstrates user-based sharding filter.
func ExampleShardByUser() {
	ctx := context.Background()
	userID := int64(42)

	builder := NewBuilder(ctx, "orders").
		Scope(ShardByUser(userID)).
		ApplyScopes()

	sql, args := builder.Build()

	fmt.Println("Query:", sql)
	fmt.Println("Args:", args)

	// Output:
	// Query: SELECT * FROM orders WHERE user_id = ?
	// Args: [42]
}

// ExampleCompose demonstrates composing multiple scopes.
func ExampleCompose() {
	ctx := context.Background()

	// Create reusable scopes
	statusFilter := func(c context.Context, b *Builder) *Builder {
		return b.Where("status = ?", "published")
	}

	dateFilter := func(c context.Context, b *Builder) *Builder {
		return b.Where("created_at > ?", "2024-01-01")
	}

	// Compose them into a single scope
	combinedScope := Compose(statusFilter, dateFilter)

	builder := NewBuilder(ctx, "articles").
		Scope(combinedScope).
		ApplyScopes()

	sql, args := builder.Build()

	fmt.Println("Query:", sql)
	fmt.Println("Args:", args)

	// Output:
	// Query: SELECT * FROM articles WHERE status = ? AND created_at > ?
	// Args: [published 2024-01-01]
}

// ExampleIf demonstrates conditional scopes.
func ExampleIf() {
	ctx := context.Background()

	// Only apply active filter if condition is true
	applyActiveFilter := true

	builder := NewBuilder(ctx, "users").
		Scope(If(applyActiveFilter, func(c context.Context, b *Builder) *Builder {
			return b.Where("active = ?", true)
		})).
		ApplyScopes()

	sql, args := builder.Build()

	fmt.Println("Query:", sql)
	fmt.Println("Args:", args)

	// Output:
	// Query: SELECT * FROM users WHERE active = ?
	// Args: [true]
}

// ExampleIf_skip demonstrates skipping a scope conditionally.
func ExampleIf_skip() {
	ctx := context.Background()

	// Skip filter if condition is false
	applyFilter := false

	builder := NewBuilder(ctx, "users").
		Scope(If(applyFilter, func(c context.Context, b *Builder) *Builder {
			return b.Where("active = ?", true)
		})).
		ApplyScopes()

	sql, args := builder.Build()

	fmt.Println("Query:", sql)
	fmt.Println("Args:", args)

	// Output:
	// Query: SELECT * FROM users
	// Args: []
}

// ExampleIfNotZero demonstrates conditional scope based on value.
func ExampleIfNotZero() {
	ctx := context.Background()

	userID := int64(123) // Non-zero, so scope will be applied

	userScope := func(id int64) Scope {
		return func(c context.Context, b *Builder) *Builder {
			return b.Where("user_id = ?", id)
		}
	}

	builder := NewBuilder(ctx, "orders").
		Scope(IfNotZero(userID, userScope)).
		ApplyScopes()

	sql, args := builder.Build()

	fmt.Println("Query:", sql)
	fmt.Println("Args:", args)

	// Output:
	// Query: SELECT * FROM orders WHERE user_id = ?
	// Args: [123]
}

// ExampleIfNotZero_skip demonstrates skipping when value is zero.
func ExampleIfNotZero_skip() {
	ctx := context.Background()

	var userID int64 = 0 // Zero, so scope will be skipped

	userScope := func(id int64) Scope {
		return func(c context.Context, b *Builder) *Builder {
			return b.Where("user_id = ?", id)
		}
	}

	builder := NewBuilder(ctx, "orders").
		Scope(IfNotZero(userID, userScope)).
		ApplyScopes()

	sql, args := builder.Build()

	fmt.Println("Query:", sql)
	fmt.Println("Args:", args)

	// Output:
	// Query: SELECT * FROM orders
	// Args: []
}

// ExampleScopeChaining demonstrates chaining multiple scopes.
func Example_chainingScopes() {
	ctx := context.Background()

	// Build a complex query with multiple scopes
	builder := NewBuilder(ctx, "articles").
		Scope(Active("status")).                    // Filter active
		Scope(NotDeleted()).                       // Exclude deleted
		Scope(OrderByDesc("published_at")).         // Sort by date
		Scope(Paginate(1, 20)).                     // Paginate
		ApplyScopes()

	sql, args := builder.Build()

	fmt.Println("Query:", sql)
	fmt.Println("Args:", args)

	// Output:
	// Query: SELECT * FROM articles WHERE status = ? AND deleted_at IS NULL ORDER BY published_at DESC LIMIT 20 OFFSET 0
	// Args: [active]
}

// ExampleNamedScopes demonstrates using named scopes.
func ExampleNamed() {
	ctx := context.Background()

	// Create a named scope for tracking
	publishedScope := Named("published", ScopeQuery, func(c context.Context, b *Builder) *Builder {
		return b.Where("published = ?", true)
	})

	builder := NewBuilder(ctx, "articles").
		Add(publishedScope).
		ApplyScopes()

	sql, _ := builder.Build()
	applied := builder.AppliedScopes()

	fmt.Println("Query:", sql)
	fmt.Println("Applied scopes:", applied)

	// Output:
	// Query: SELECT * FROM articles WHERE published = ?
	// Applied scopes: [published]
}

// ExampleRouting demonstrates routing scopes.
func Example_routing() {
	ctx := context.Background()
	userID := int64(42)
	shardCount := 4

	// First apply routing to select shard, then apply filters
	builder := NewBuilder(ctx, "user_data").
		Routing(func(c context.Context, b *Builder) *Builder {
			shardIndex := userID % int64(shardCount)
			return b.Table(fmt.Sprintf("user_data_%d", shardIndex))
		}).
		Scope(func(c context.Context, b *Builder) *Builder {
			return b.Where("user_id = ?", userID)
		}).
		ApplyScopes()

	sql, args := builder.Build()

	fmt.Println("Query:", sql)
	fmt.Println("Args:", args)

	// 42 % 4 = 2, routes to user_data_2
	// Output:
	// Query: SELECT * FROM user_data_2 WHERE user_id = ?
	// Args: [42]
}

// ExampleRealWorldUsage demonstrates a real-world service layer pattern.
func Example_realWorldService() {
	ctx := context.Background()

	// Simulate a service function that builds a query based on parameters
	searchUsers := func(status string, page, pageSize int, searchTerm string) (string, []interface{}) {
		builder := NewBuilder(ctx, "users").
			Scope(NotDeleted())

		// Conditionally apply status filter
		if status != "" {
			builder.Scope(func(c context.Context, b *Builder) *Builder {
				return b.Where("status = ?", status)
			})
		}

		// Conditionally apply search
		if searchTerm != "" {
			builder.Scope(func(c context.Context, b *Builder) *Builder {
				return b.Where("(name LIKE ? OR email LIKE ?)", "%"+searchTerm+"%", "%"+searchTerm+"%")
			})
		}

		// Always apply pagination and ordering
		builder.Scope(OrderByDesc("created_at")).
			Scope(Paginate(page, pageSize)).
			ApplyScopes()

		return builder.Build()
	}

	sql, args := searchUsers("active", 1, 10, "john")

	fmt.Println("Query:", sql)
	fmt.Println("Args:", args)

	// Output:
	// Query: SELECT * FROM users WHERE deleted_at IS NULL AND status = ? AND (name LIKE ? OR email LIKE ?) ORDER BY created_at DESC LIMIT 10 OFFSET 0
	// Args: [active %john% %john%]
}

// ExampleTrace demonstrates trace/meta scope.
func ExampleTrace() {
	ctx := context.Background()

	// Trace scope can be used for logging/tracing without affecting query
	builder := NewBuilder(ctx, "users").
		Scope(Trace("list-users-query")).
		Scope(Active("status")).
		ApplyScopes()

	sql, _ := builder.Build()

	fmt.Println("Query:", sql)

	// Output:
	// Query: SELECT * FROM users WHERE status = ?
}
