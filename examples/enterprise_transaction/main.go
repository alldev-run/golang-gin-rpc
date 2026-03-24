
package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/db/orm"
)

func main() {
	fmt.Println("=== Enterprise Transaction Manager Examples ===")

	// Example 1: Basic enterprise transaction manager setup
	fmt.Println("1. Basic Enterprise Transaction Manager:")
	
	config := orm.TransactionConfig{
		// Retry configuration
		MaxRetries:         5,
		RetryDelay:         50 * time.Millisecond,
		RetryBackoffFactor: 2.0,
		MaxRetryDelay:      10 * time.Second,
		
		// Transaction configuration
		IsolationLevel: sql.LevelReadCommitted,
		ReadOnly:       false,
		Timeout:        30 * time.Second,
		
		// Monitoring and logging
		EnableMetrics:      true,
		EnableTracing:      true,
		LogSlowQueries:     true,
		SlowQueryThreshold: 500 * time.Millisecond,
		
		// Connection pool configuration
		MaxOpenConns:    50,
		MaxIdleConns:    10,
		ConnMaxLifetime: 1 * time.Hour,
		ConnMaxIdleTime: 30 * time.Minute,
	}

	tm := orm.NewTransactionManager(config)
	defer tm.Close()

	// Mock database connection (in real usage, you'd use a actual DB connection)
	db := &mockDB{}

	// Example transaction
	result, err := tm.WithTransaction(context.Background(), db, func(txORM *orm.ORM) error {
		// Simulate database operations
		fmt.Println("  Executing transaction logic...")
		time.Sleep(100 * time.Millisecond) // Simulate work
		
		// Simulate an error to test retry logic
		// return fmt.Errorf("simulated connection error")
		
		return nil
	})

	if err != nil {
		fmt.Printf("  Transaction failed: %v\n", err)
	} else {
		fmt.Printf("  Transaction succeeded: %+v\n", result)
	}

	// Example 2: Metrics and monitoring
	fmt.Println("\n2. Metrics and Monitoring:")
	
	metrics := tm.GetMetrics()
	fmt.Printf("  Total Transactions: %d\n", metrics.TotalTransactions)
	fmt.Printf("  Successful Transactions: %d\n", metrics.SuccessfulTransactions)
	fmt.Printf("  Failed Transactions: %d\n", metrics.FailedTransactions)
	fmt.Printf("  Retried Transactions: %d\n", metrics.RetriedTransactions)
	fmt.Printf("  Average Duration: %v\n", metrics.AvgDuration)
	fmt.Printf("  Max Duration: %v\n", metrics.MaxDuration)
	fmt.Printf("  Min Duration: %v\n", metrics.MinDuration)
	fmt.Printf("  Active Transactions: %d\n", tm.GetActiveTransactionCount())

	// Example 3: Nested transactions with savepoints
	fmt.Println("\n3. Nested Transactions with Savepoints:")
	
	// Create a mock ORM instance
	mockORM := &orm.ORM{} // In real usage, this would be properly initialized
	
	err = tm.WithNestedTransaction(context.Background(), mockORM, "user_update_sp", func(nestedORM *orm.ORM) error {
		fmt.Println("  Executing nested transaction...")
		
		// Nested savepoint
		err := tm.WithNestedTransaction(context.Background(), nestedORM, "audit_sp", func(auditORM *orm.ORM) error {
			fmt.Println("    Executing audit savepoint...")
			time.Sleep(50 * time.Millisecond)
			return nil
		})
		
		if err != nil {
			fmt.Printf("    Audit savepoint failed: %v\n", err)
		}
		
		return nil
	})
	
	if err != nil {
		fmt.Printf("  Nested transaction failed: %v\n", err)
	} else {
		fmt.Println("  Nested transaction succeeded")
	}

	// Example 4: Retry logic with different error types
	fmt.Println("\n4. Retry Logic with Error Classification:")
	
	// Test different error scenarios
	errorScenarios := []struct {
		name  string
		error error
	}{
		{"Connection Error", fmt.Errorf("connection refused")},
		{"Deadlock Error", fmt.Errorf("deadlock detected")},
		{"Constraint Error", fmt.Errorf("unique constraint violation")},
		{"Timeout Error", fmt.Errorf("query timeout exceeded")},
		{"Logic Error", fmt.Errorf("invalid input data")},
	}

	for _, scenario := range errorScenarios {
		fmt.Printf("  Testing %s:\n", scenario.name)
		
		result, err := tm.WithTransaction(context.Background(), db, func(txORM *orm.ORM) error {
			return scenario.error
		})
		
		if err != nil {
			fmt.Printf("    Failed after %d retries: %v\n", result.Retries, err)
		} else {
			fmt.Printf("    Succeeded after %d retries\n", result.Retries)
		}
	}

	// Example 5: Configuration updates
	fmt.Println("\n5. Dynamic Configuration Updates:")
	
	// Update configuration for different workloads
	highConcurrencyConfig := orm.TransactionConfig{
		MaxRetries:         2,
		RetryDelay:         25 * time.Millisecond,
		RetryBackoffFactor: 1.5,
		MaxRetryDelay:      2 * time.Second,
		IsolationLevel:     sql.LevelReadCommitted,
		Timeout:            10 * time.Second,
		EnableMetrics:      true,
		LogSlowQueries:     true,
		SlowQueryThreshold: 200 * time.Millisecond,
		MaxOpenConns:       100,
		MaxIdleConns:       20,
		ConnMaxLifetime:    30 * time.Minute,
		ConnMaxIdleTime:    5 * time.Minute,
	}

	tm.SetConfig(highConcurrencyConfig)
	fmt.Printf("  Updated configuration for high concurrency\n")
	
	// Test with new configuration
	result, err = tm.WithTransaction(context.Background(), db, func(txORM *orm.ORM) error {
		fmt.Println("  Executing with high concurrency config...")
		time.Sleep(50 * time.Millisecond)
		return nil
	})
	
	if err != nil {
		fmt.Printf("  Failed: %v\n", err)
	} else {
		fmt.Printf("  Succeeded in %v with %d retries\n", result.Duration, result.Retries)
	}

	// Example 6: Enterprise retry logic
	fmt.Println("\n6. Enterprise Retry Logic:")
	
	err = tm.WithRetry(context.Background(), func() error {
		fmt.Println("  Attempting operation...")
		return fmt.Errorf("temporary network error")
	})
	
	if err != nil {
		fmt.Printf("  Operation failed after retries: %v\n", err)
	} else {
		fmt.Println("  Operation succeeded")
	}

	// Example 7: Performance monitoring
	fmt.Println("\n7. Performance Monitoring:")
	
	// Simulate multiple transactions to gather metrics
	for i := 0; i < 10; i++ {
		_, err := tm.WithTransaction(context.Background(), db, func(txORM *orm.ORM) error {
			time.Sleep(time.Duration(i*10) * time.Millisecond) // Variable duration
			if i == 5 {
				return fmt.Errorf("simulated deadlock") // Test retry
			}
			return nil
		})
		
		if err != nil && i != 5 {
			fmt.Printf("  Transaction %d failed: %v\n", i+1, err)
		}
	}
	
	// Display updated metrics
	updatedMetrics := tm.GetMetrics()
	fmt.Printf("  Updated Metrics:\n")
	fmt.Printf("    Total: %d, Success: %d, Failed: %d, Retried: %d\n",
		updatedMetrics.TotalTransactions,
		updatedMetrics.SuccessfulTransactions,
		updatedMetrics.FailedTransactions,
		updatedMetrics.RetriedTransactions)
	fmt.Printf("    Avg Duration: %v, Max: %v, Min: %v\n",
		updatedMetrics.AvgDuration,
		updatedMetrics.MaxDuration,
		updatedMetrics.MinDuration)

	// Example 8: Configuration validation and defaults
	fmt.Println("\n8. Configuration Validation and Defaults:")
	
	// Test with empty configuration (should use defaults)
	emptyConfig := orm.TransactionConfig{}
	defaultTM := orm.NewTransactionManager(emptyConfig)
	defer defaultTM.Close()
	
	defaultConfig := defaultTM.GetConfig()
	fmt.Printf("  Default MaxRetries: %d\n", defaultConfig.MaxRetries)
	fmt.Printf("  Default RetryDelay: %v\n", defaultConfig.RetryDelay)
	fmt.Printf("  Default IsolationLevel: %s\n", defaultConfig.IsolationLevel.String())
	fmt.Printf("  Default Timeout: %v\n", defaultConfig.Timeout)

	fmt.Println("\n=== Enterprise Transaction Manager Examples Completed ===")
}

// mockDB provides a mock database implementation for testing
type mockDB struct{}

// Begin implements the DB interface for mock
func (m *mockDB) Begin(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	// Mock implementation
	fmt.Printf("    Begin transaction\n")
	return nil, fmt.Errorf("mock database - transaction not implemented")
}

// Exec implements the DB interface for mock
func (m *mockDB) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	// Mock implementation
	fmt.Printf("    Exec: %s\n", query)
	return nil, nil
}

// Query implements the DB interface for mock
func (m *mockDB) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	// Mock implementation
	fmt.Printf("    Query: %s\n", query)
	return nil, nil
}

// QueryRow implements the DB interface for mock
func (m *mockDB) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	// Mock implementation
	fmt.Printf("    QueryRow: %s\n", query)
	return nil
}

// Ping implements the DB interface for mock
func (m *mockDB) Ping(ctx context.Context) error {
	// Mock implementation
	return nil
}

// Close implements the DB interface for mock
func (m *mockDB) Close() error {
	// Mock implementation
	return nil
}

// Stats implements the DB interface for mock
func (m *mockDB) Stats() sql.DBStats {
	// Mock implementation
	return sql.DBStats{}
}
