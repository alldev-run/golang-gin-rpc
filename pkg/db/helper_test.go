package db

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/cache/redis"
	"github.com/alldev-run/golang-gin-rpc/pkg/db/mysql"
	"github.com/alldev-run/golang-gin-rpc/pkg/db/postgres"
)

// mockFactory is a test mock that implements the minimal factory interface

type mockFactory struct {
	mysqlClient      *mysql.Client
	redisClient      interface{}
	postgresClient   interface{}
	getMySQLError    error
	getRedisError    error
	getPostgresError error
}

func (m *mockFactory) GetMySQL() (*mysql.Client, error) {
	if m.getMySQLError != nil {
		return nil, m.getMySQLError
	}
	return m.mysqlClient, nil
}

func (m *mockFactory) GetMySQLSQLClient() (SQLClient, error) {
	if m.getMySQLError != nil {
		return nil, m.getMySQLError
	}
	// In real scenario, mysql.Client implements SQLClient
	return m.mysqlClient, nil
}

func (m *mockFactory) GetRedis() (*redis.Client, error) {
	if m.getRedisError != nil {
		return nil, m.getRedisError
	}
	return m.redisClient.(*redis.Client), nil
}

func (m *mockFactory) GetPostgres() (*postgres.Client, error) {
	if m.getPostgresError != nil {
		return nil, m.getPostgresError
	}
	return m.postgresClient.(*postgres.Client), nil
}

// ==================== Test Global Factory ====================

func TestSetAndGetGlobalFactory(t *testing.T) {
	// Clean up after test
	defer func() {
		SetGlobalFactory(nil)
	}()

	// Test setting and getting factory
	mock := &Factory{}
	SetGlobalFactory(mock)

	got := GetGlobalFactory()
	if got != mock {
		t.Errorf("GetGlobalFactory() = %v, want %v", got, mock)
	}

	// Test nil factory
	SetGlobalFactory(nil)
	got = GetGlobalFactory()
	if got != nil {
		t.Errorf("GetGlobalFactory() after nil set = %v, want nil", got)
	}
}

func TestGetGlobalFactory_Concurrent(t *testing.T) {
	defer func() {
		SetGlobalFactory(nil)
	}()

	mock := &Factory{}
	SetGlobalFactory(mock)

	// Concurrent reads should be safe
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_ = GetGlobalFactory()
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("Timeout waiting for concurrent reads")
		}
	}
}

// ==================== Test MySQL Helper ====================

func TestMySQL_FactoryNotInitialized(t *testing.T) {
	defer func() {
		SetGlobalFactory(nil)
	}()

	SetGlobalFactory(nil)

	_, err := MySQL()
	if err != ErrFactoryNotInitialized {
		t.Errorf("MySQL() error = %v, want %v", err, ErrFactoryNotInitialized)
	}
}

func TestMySQLClient_FactoryNotInitialized(t *testing.T) {
	defer func() {
		SetGlobalFactory(nil)
	}()

	SetGlobalFactory(nil)

	_, err := MySQLClient()
	if err != ErrFactoryNotInitialized {
		t.Errorf("MySQLClient() error = %v, want %v", err, ErrFactoryNotInitialized)
	}
}

// ==================== Test ORM Helpers ====================

func TestSelect_FactoryNotInitialized(t *testing.T) {
	defer func() {
		SetGlobalFactory(nil)
	}()

	SetGlobalFactory(nil)

	_, err := Select("users")
	if err != ErrFactoryNotInitialized {
		t.Errorf("Select() error = %v, want %v", err, ErrFactoryNotInitialized)
	}
}

func TestInsert_FactoryNotInitialized(t *testing.T) {
	defer func() {
		SetGlobalFactory(nil)
	}()

	SetGlobalFactory(nil)

	_, err := Insert("users")
	if err != ErrFactoryNotInitialized {
		t.Errorf("Insert() error = %v, want %v", err, ErrFactoryNotInitialized)
	}
}

func TestUpdate_FactoryNotInitialized(t *testing.T) {
	defer func() {
		SetGlobalFactory(nil)
	}()

	SetGlobalFactory(nil)

	_, err := Update("users")
	if err != ErrFactoryNotInitialized {
		t.Errorf("Update() error = %v, want %v", err, ErrFactoryNotInitialized)
	}
}

func TestDelete_FactoryNotInitialized(t *testing.T) {
	defer func() {
		SetGlobalFactory(nil)
	}()

	SetGlobalFactory(nil)

	_, err := Delete("users")
	if err != ErrFactoryNotInitialized {
		t.Errorf("Delete() error = %v, want %v", err, ErrFactoryNotInitialized)
	}
}

// ==================== Test Redis Helper ====================

func TestRedis_FactoryNotInitialized(t *testing.T) {
	defer func() {
		SetGlobalFactory(nil)
	}()

	SetGlobalFactory(nil)

	_, err := Redis()
	if err != ErrFactoryNotInitialized {
		t.Errorf("Redis() error = %v, want %v", err, ErrFactoryNotInitialized)
	}
}

// ==================== Test PostgreSQL Helper ====================

func TestPostgres_FactoryNotInitialized(t *testing.T) {
	defer func() {
		SetGlobalFactory(nil)
	}()

	SetGlobalFactory(nil)

	_, err := Postgres()
	if err != ErrFactoryNotInitialized {
		t.Errorf("Postgres() error = %v, want %v", err, ErrFactoryNotInitialized)
	}
}

// ==================== Test Query Helpers ====================

func TestQuery_FactoryNotInitialized(t *testing.T) {
	defer func() {
		SetGlobalFactory(nil)
	}()

	SetGlobalFactory(nil)

	ctx := context.Background()
	_, err := Query(ctx, "SELECT 1")
	if err != ErrFactoryNotInitialized {
		t.Errorf("Query() error = %v, want %v", err, ErrFactoryNotInitialized)
	}
}

func TestQueryRow_FactoryNotInitialized(t *testing.T) {
	defer func() {
		SetGlobalFactory(nil)
	}()

	SetGlobalFactory(nil)

	ctx := context.Background()
	row := QueryRow(ctx, "SELECT 1")
	// QueryRow returns an empty sql.Row on error, just verify it doesn't panic
	if row == nil {
		t.Error("QueryRow() returned nil")
	}
}

func TestExec_FactoryNotInitialized(t *testing.T) {
	defer func() {
		SetGlobalFactory(nil)
	}()

	SetGlobalFactory(nil)

	ctx := context.Background()
	_, err := Exec(ctx, "INSERT INTO test VALUES (1)")
	if err != ErrFactoryNotInitialized {
		t.Errorf("Exec() error = %v, want %v", err, ErrFactoryNotInitialized)
	}
}

func TestBegin_FactoryNotInitialized(t *testing.T) {
	defer func() {
		SetGlobalFactory(nil)
	}()

	SetGlobalFactory(nil)

	ctx := context.Background()
	_, err := Begin(ctx, nil)
	if err != ErrFactoryNotInitialized {
		t.Errorf("Begin() error = %v, want %v", err, ErrFactoryNotInitialized)
	}
}

func TestTransaction_FactoryNotInitialized(t *testing.T) {
	defer func() {
		SetGlobalFactory(nil)
	}()

	SetGlobalFactory(nil)

	ctx := context.Background()
	err := Transaction(ctx, func(tx *sql.Tx) error {
		return nil
	})
	if err != ErrFactoryNotInitialized {
		t.Errorf("Transaction() error = %v, want %v", err, ErrFactoryNotInitialized)
	}
}

func TestDB_FactoryNotInitialized(t *testing.T) {
	defer func() {
		SetGlobalFactory(nil)
	}()

	SetGlobalFactory(nil)

	_, err := DB()
	if err != ErrFactoryNotInitialized {
		t.Errorf("DB() error = %v, want %v", err, ErrFactoryNotInitialized)
	}
}

// ==================== Test DBError ====================

func TestDBError_Error(t *testing.T) {
	err := &DBError{Message: "test error"}
	if got := err.Error(); got != "test error" {
		t.Errorf("DBError.Error() = %v, want %v", got, "test error")
	}
}

func TestErrFactoryNotInitialized(t *testing.T) {
	expected := "database factory not initialized, ensure bootstrap.InitializeDatabases() was called"
	if ErrFactoryNotInitialized.Error() != expected {
		t.Errorf("ErrFactoryNotInitialized message = %v, want %v",
			ErrFactoryNotInitialized.Error(), expected)
	}
}

// ==================== Test with Real Factory (Integration Style) ====================

func TestHelpers_WithMockFactory(t *testing.T) {
	// Note: This test uses nil mysql client since we don't have a real connection
	// In real integration tests, you'd use a test database

	factory := NewFactory()
	SetGlobalFactory(factory)
	defer func() {
		SetGlobalFactory(nil)
	}()

	// Test that factory methods return "client not found" errors
	// rather than "factory not initialized" since factory is set

	// Test MySQLClient returns error for no MySQL initialized
	_, err := MySQLClient()
	if err == nil {
		t.Error("MySQLClient() expected error when no MySQL initialized, got nil")
	}

	// Test Select returns error for no MySQL initialized
	_, err = Select("users")
	if err == nil {
		t.Error("Select() expected error when no MySQL initialized, got nil")
	}

	// Test Insert returns error for no MySQL initialized
	_, err = Insert("users")
	if err == nil {
		t.Error("Insert() expected error when no MySQL initialized, got nil")
	}

	// Test Update returns error for no MySQL initialized
	_, err = Update("users")
	if err == nil {
		t.Error("Update() expected error when no MySQL initialized, got nil")
	}

	// Test Delete returns error for no MySQL initialized
	_, err = Delete("users")
	if err == nil {
		t.Error("Delete() expected error when no MySQL initialized, got nil")
	}

	// Test Redis returns error for no Redis initialized
	_, err = Redis()
	if err == nil {
		t.Error("Redis() expected error when no Redis initialized, got nil")
	}

	// Test Postgres returns error for no Postgres initialized
	_, err = Postgres()
	if err == nil {
		t.Error("Postgres() expected error when no Postgres initialized, got nil")
	}
}

// TestHelpers_ConcurrentAccess tests concurrent access to global factory
func TestHelpers_ConcurrentAccess(t *testing.T) {
	factory := NewFactory()
	SetGlobalFactory(factory)
	defer func() {
		SetGlobalFactory(nil)
	}()

	// Concurrent calls should not panic
	done := make(chan bool, 20)
	for i := 0; i < 10; i++ {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Concurrent access panicked: %v", r)
				}
				done <- true
			}()
			_, _ = Select("users")
		}()
		go func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Concurrent access panicked: %v", r)
				}
				done <- true
			}()
			_, _ = MySQLClient()
		}()
	}

	for i := 0; i < 20; i++ {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("Timeout waiting for concurrent access")
			return
		}
	}
}

// TestOrmBuilderTypes verifies builder types are returned correctly
func TestOrmBuilderTypes(t *testing.T) {
	// This test verifies the type assertions work correctly
	// We can't test actual builder creation without a real database
	// but we can verify the error handling paths

	tests := []struct {
		name    string
		fn      func() (interface{}, error)
		wantErr bool
	}{
		{
			name: "Select builder",
			fn: func() (interface{}, error) {
				return Select("test_table")
			},
			wantErr: true, // factory not initialized
		},
		{
			name: "Insert builder",
			fn: func() (interface{}, error) {
				return Insert("test_table")
			},
			wantErr: true,
		},
		{
			name: "Update builder",
			fn: func() (interface{}, error) {
				return Update("test_table")
			},
			wantErr: true,
		},
		{
			name: "Delete builder",
			fn: func() (interface{}, error) {
				return Delete("test_table")
			},
			wantErr: true,
		},
	}

	SetGlobalFactory(nil)
	defer func() {
		SetGlobalFactory(nil)
	}()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.fn()
			if (err != nil) != tt.wantErr {
				t.Errorf("%s error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
			if err != ErrFactoryNotInitialized {
				t.Errorf("%s error = %v, want %v", tt.name, err, ErrFactoryNotInitialized)
			}
		})
	}
}

// TestQueryHelpers_ContextCancellation tests context handling
func TestQueryHelpers_ContextCancellation(t *testing.T) {
	factory := NewFactory()
	SetGlobalFactory(factory)
	defer func() {
		SetGlobalFactory(nil)
	}()

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// These should return errors (though not specifically context cancelled
	// since the factory has no clients initialized)
	_, err := Query(ctx, "SELECT 1")
	if err == nil {
		t.Error("Query() with cancelled context expected error")
	}
}

// TestIntegrationPattern demonstrates the typical usage pattern
func TestIntegrationPattern(t *testing.T) {
	// This test demonstrates the pattern for using the helper
	// In real code, bootstrap would call SetGlobalFactory

	// Pattern 1: Check error before using
	t.Run("pattern_check_error", func(t *testing.T) {
		SetGlobalFactory(nil)
		defer SetGlobalFactory(nil)

		builder, err := Select("users")
		if err != nil {
			// Handle initialization error
			t.Logf("Expected error: %v", err)
			return
		}
		_ = builder
	})

	// Pattern 2: MustGet style (panic on error) - for services that require DB
	t.Run("pattern_must_get", func(t *testing.T) {
		factory := NewFactory()
		SetGlobalFactory(factory)
		defer SetGlobalFactory(nil)

		client, err := MySQLClient()
		// In real usage with proper bootstrap, err would be nil after InitializeDatabases
		if err != nil {
			t.Logf("MySQL not initialized (expected in test without DB): %v", err)
			return
		}
		_ = client
	})
}
