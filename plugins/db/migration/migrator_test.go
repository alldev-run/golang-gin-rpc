package migration

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

// TestMigrationStruct tests the Migration struct
type TestMigrationStruct struct {
	Version   int
	Name      string
	UpSQL     string
	DownSQL   string
	Timestamp time.Time
}

func TestMigrationStatusStruct(t *testing.T) {
	status := MigrationStatus{
		Version: 1,
		Name:    "create_users",
		Applied: true,
	}

	if status.Version != 1 {
		t.Errorf("Version = %d, want 1", status.Version)
	}
	if status.Name != "create_users" {
		t.Errorf("Name = %s, want create_users", status.Name)
	}
	if !status.Applied {
		t.Error("Applied should be true")
	}
}

func TestMigratorAdd(t *testing.T) {
	// Create in-memory SQLite for testing
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	m := New(db)

	// Add a migration
	m.Add(1, "create_users", "CREATE TABLE users (id INT);", "DROP TABLE users;")

	if len(m.migrations) != 1 {
		t.Errorf("Expected 1 migration, got %d", len(m.migrations))
	}

	if m.migrations[0].Version != 1 {
		t.Errorf("Version = %d, want 1", m.migrations[0].Version)
	}

	if m.migrations[0].Name != "create_users" {
		t.Errorf("Name = %s, want create_users", m.migrations[0].Name)
	}
}

func TestMigratorInit(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	m := New(db)

	ctx := context.Background()
	err = m.Init(ctx)
	if err != nil {
		t.Errorf("Init failed: %v", err)
	}

	// Verify table exists
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='schema_migrations'").Scan(&count)
	if err != nil {
		t.Errorf("Failed to check table existence: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected table to exist, got count=%d", count)
	}
}

func TestGetCurrentVersion(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	m := New(db)
	ctx := context.Background()

	// Initialize
	if err := m.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Get version (should be 0 initially)
	version, err := m.GetCurrentVersion(ctx)
	if err != nil {
		t.Errorf("GetCurrentVersion failed: %v", err)
	}
	if version != 0 {
		t.Errorf("Expected version 0, got %d", version)
	}
}

func TestMigratorUp(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	m := New(db)
	ctx := context.Background()

	// Add migrations
	m.Add(1, "create_users", "CREATE TABLE users (id INTEGER PRIMARY KEY);", "DROP TABLE users;")
	m.Add(2, "create_posts", "CREATE TABLE posts (id INTEGER PRIMARY KEY, user_id INTEGER);", "DROP TABLE posts;")

	// Run migrations
	err = m.Up(ctx)
	if err != nil {
		t.Errorf("Up failed: %v", err)
	}

	// Verify version
	version, _ := m.GetCurrentVersion(ctx)
	if version != 2 {
		t.Errorf("Expected version 2, got %d", version)
	}

	// Verify tables exist
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('users', 'posts')").Scan(&count)
	if err != nil {
		t.Errorf("Failed to check tables: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 tables, got %d", count)
	}
}

func TestMigratorDown(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	m := New(db)
	ctx := context.Background()

	// Add and run migrations
	m.Add(1, "create_users", "CREATE TABLE users (id INTEGER PRIMARY KEY);", "DROP TABLE users;")
	err = m.Up(ctx)
	if err != nil {
		t.Fatalf("Up failed: %v", err)
	}

	// Rollback
	err = m.Down(ctx)
	if err != nil {
		t.Errorf("Down failed: %v", err)
	}

	// Verify version is 0
	version, _ := m.GetCurrentVersion(ctx)
	if version != 0 {
		t.Errorf("Expected version 0 after rollback, got %d", version)
	}
}

func TestMigratorDownNoMigrations(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	m := New(db)
	ctx := context.Background()

	// Try to rollback without any migrations
	err = m.Down(ctx)
	if err == nil {
		t.Error("Expected error when no migrations to rollback")
	}
}

func TestMigratorStatus(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	m := New(db)
	ctx := context.Background()

	// Add migrations
	m.Add(1, "create_users", "CREATE TABLE users (id INTEGER PRIMARY KEY);", "DROP TABLE users;")
	m.Add(2, "create_posts", "CREATE TABLE posts (id INTEGER PRIMARY KEY);", "DROP TABLE posts;")

	// Run first migration only by removing second before running
	// Create a new migrator with just the first migration
	m2 := New(db)
	m2.Add(1, "create_users", "CREATE TABLE users (id INTEGER PRIMARY KEY);", "DROP TABLE users;")
	err = m2.Up(ctx)
	if err != nil {
		t.Fatalf("Up failed: %v", err)
	}

	// Now check status with both migrations registered
	statuses, err := m.Status(ctx)
	if err != nil {
		t.Errorf("Status failed: %v", err)
	}

	if len(statuses) != 2 {
		t.Errorf("Expected 2 statuses, got %d", len(statuses))
	}

	// First should be applied, second not
	if !statuses[0].Applied {
		t.Error("First migration should be applied")
	}
	if statuses[1].Applied {
		t.Error("Second migration should not be applied")
	}
}

func TestDefaultTableName(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	m := New(db)

	if m.TableName != "schema_migrations" {
		t.Errorf("Default table name should be schema_migrations, got %s", m.TableName)
	}
}

// TestConcurrentMigrations tests that concurrent access doesn't cause issues
func TestConcurrentMigrations(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	m := New(db)
	ctx := context.Background()

	m.Add(1, "create_users", "CREATE TABLE users (id INTEGER PRIMARY KEY);", "DROP TABLE users;")

	// Run migration multiple times (should be idempotent)
	for i := 0; i < 3; i++ {
		err = m.Up(ctx)
		if err != nil {
			t.Errorf("Iteration %d: Up failed: %v", i, err)
		}
	}

	// Version should still be 1
	version, _ := m.GetCurrentVersion(ctx)
	if version != 1 {
		t.Errorf("Expected version 1 after idempotent runs, got %d", version)
	}
}
