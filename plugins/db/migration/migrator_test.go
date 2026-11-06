package migration

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// newTestMigrator creates a Migrator backed by sqlmock so tests stay
// deterministic and offline (no CGO / real database required).
func newTestMigrator(t *testing.T) (*Migrator, sqlmock.Sqlmock, func()) {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	m := New(db)
	return m, mock, func() { _ = db.Close() }
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

func TestNewMigratorDefaults(t *testing.T) {
	m, _, cleanup := newTestMigrator(t)
	defer cleanup()

	if m.DB == nil {
		t.Error("New() should set DB")
	}
	if m.TableName != "schema_migrations" {
		t.Errorf("Default table name should be schema_migrations, got %s", m.TableName)
	}
	if len(m.migrations) != 0 {
		t.Errorf("New() should start with 0 migrations, got %d", len(m.migrations))
	}
}

func TestMigratorAdd(t *testing.T) {
	m, _, cleanup := newTestMigrator(t)
	defer cleanup()

	m.Add(1, "create_users", "CREATE TABLE users (id INT);", "DROP TABLE users;")
	m.Add(2, "create_posts", "CREATE TABLE posts (id INT);", "DROP TABLE posts;")

	if len(m.migrations) != 2 {
		t.Fatalf("Expected 2 migrations, got %d", len(m.migrations))
	}

	first := m.migrations[0]
	if first.Version != 1 {
		t.Errorf("Version = %d, want 1", first.Version)
	}
	if first.Name != "create_users" {
		t.Errorf("Name = %s, want create_users", first.Name)
	}
	if first.UpSQL != "CREATE TABLE users (id INT);" {
		t.Errorf("UpSQL = %q, want CREATE TABLE users (id INT);", first.UpSQL)
	}
	if first.DownSQL != "DROP TABLE users;" {
		t.Errorf("DownSQL = %q, want DROP TABLE users;", first.DownSQL)
	}
	if first.Timestamp.IsZero() {
		t.Error("Timestamp should be set by Add")
	}
}

func TestMigratorInit(t *testing.T) {
	m, mock, cleanup := newTestMigrator(t)
	defer cleanup()

	mock.ExpectExec(regexp.QuoteMeta(`CREATE TABLE IF NOT EXISTS schema_migrations`)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	if err := m.Init(context.Background()); err != nil {
		t.Errorf("Init failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestMigratorInitError(t *testing.T) {
	m, mock, cleanup := newTestMigrator(t)
	defer cleanup()

	mock.ExpectExec(regexp.QuoteMeta(`CREATE TABLE IF NOT EXISTS schema_migrations`)).
		WillReturnError(errors.New("disk full"))

	err := m.Init(context.Background())
	if err == nil {
		t.Fatal("Init() should propagate database errors")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestGetCurrentVersion(t *testing.T) {
	tests := []struct {
		name      string
		dbVersion int
	}{
		{name: "empty table returns zero", dbVersion: 0},
		{name: "returns applied version", dbVersion: 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, mock, cleanup := newTestMigrator(t)
			defer cleanup()

			mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(MAX(version), 0) FROM schema_migrations")).
				WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(tt.dbVersion))

			version, err := m.GetCurrentVersion(context.Background())
			if err != nil {
				t.Fatalf("GetCurrentVersion failed: %v", err)
			}
			if version != tt.dbVersion {
				t.Errorf("GetCurrentVersion() = %d, want %d", version, tt.dbVersion)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unmet expectations: %v", err)
			}
		})
	}
}

func TestGetCurrentVersionError(t *testing.T) {
	m, mock, cleanup := newTestMigrator(t)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(MAX(version), 0) FROM schema_migrations")).
		WillReturnError(errors.New("table missing"))

	if _, err := m.GetCurrentVersion(context.Background()); err == nil {
		t.Fatal("GetCurrentVersion() should propagate scan errors")
	}
}

// expectUpMigration stubs the SQL sequence of runUp for one migration.
func expectUpMigration(mock sqlmock.Sqlmock, version int, name, upSQL string) {
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(upSQL)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO schema_migrations (version, name) VALUES (?, ?)")).
		WithArgs(version, name).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
}

// expectDownMigration stubs the SQL sequence of runDown for one migration.
func expectDownMigration(mock sqlmock.Sqlmock, version int, downSQL string) {
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(downSQL)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM schema_migrations WHERE version = ?")).
		WithArgs(version).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
}

func TestMigratorUp(t *testing.T) {
	m, mock, cleanup := newTestMigrator(t)
	defer cleanup()

	// Register out of order to verify sorting by version.
	m.Add(2, "create_posts", "CREATE TABLE posts (id INTEGER PRIMARY KEY);", "DROP TABLE posts;")
	m.Add(1, "create_users", "CREATE TABLE users (id INTEGER PRIMARY KEY);", "DROP TABLE users;")

	mock.ExpectExec(regexp.QuoteMeta(`CREATE TABLE IF NOT EXISTS schema_migrations`)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(MAX(version), 0) FROM schema_migrations")).
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(0))

	// Migrations must run in ascending version order.
	expectUpMigration(mock, 1, "create_users", "CREATE TABLE users (id INTEGER PRIMARY KEY);")
	expectUpMigration(mock, 2, "create_posts", "CREATE TABLE posts (id INTEGER PRIMARY KEY);")

	if err := m.Up(context.Background()); err != nil {
		t.Fatalf("Up failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestMigratorUpIsIdempotent(t *testing.T) {
	m, mock, cleanup := newTestMigrator(t)
	defer cleanup()

	m.Add(1, "create_users", "CREATE TABLE users (id INTEGER PRIMARY KEY);", "DROP TABLE users;")

	// Second run: current version already at 1, no pending migrations.
	mock.ExpectExec(regexp.QuoteMeta(`CREATE TABLE IF NOT EXISTS schema_migrations`)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(MAX(version), 0) FROM schema_migrations")).
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(1))

	if err := m.Up(context.Background()); err != nil {
		t.Fatalf("Up failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestMigratorUpInitError(t *testing.T) {
	m, mock, cleanup := newTestMigrator(t)
	defer cleanup()

	m.Add(1, "create_users", "CREATE TABLE users (id INT);", "DROP TABLE users;")

	mock.ExpectExec(regexp.QuoteMeta(`CREATE TABLE IF NOT EXISTS schema_migrations`)).
		WillReturnError(errors.New("read only database"))

	err := m.Up(context.Background())
	if err == nil {
		t.Fatal("Up() should fail when Init fails")
	}
	if want := "failed to init migrations"; !strings.Contains(err.Error(), want) {
		t.Errorf("Up() error = %v, want it to contain %q", err, want)
	}
}

func TestMigratorUpGetCurrentVersionError(t *testing.T) {
	m, mock, cleanup := newTestMigrator(t)
	defer cleanup()

	m.Add(1, "create_users", "CREATE TABLE users (id INT);", "DROP TABLE users;")

	mock.ExpectExec(regexp.QuoteMeta(`CREATE TABLE IF NOT EXISTS schema_migrations`)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(MAX(version), 0) FROM schema_migrations")).
		WillReturnError(errors.New("query failed"))

	err := m.Up(context.Background())
	if err == nil {
		t.Fatal("Up() should fail when GetCurrentVersion fails")
	}
	if want := "failed to get current version"; !strings.Contains(err.Error(), want) {
		t.Errorf("Up() error = %v, want it to contain %q", err, want)
	}
}

func TestMigratorUpExecError(t *testing.T) {
	m, mock, cleanup := newTestMigrator(t)
	defer cleanup()

	upSQL := "CREATE TABLE users (id INT);"
	m.Add(1, "create_users", upSQL, "DROP TABLE users;")

	mock.ExpectExec(regexp.QuoteMeta(`CREATE TABLE IF NOT EXISTS schema_migrations`)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(MAX(version), 0) FROM schema_migrations")).
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(0))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(upSQL)).
		WillReturnError(errors.New("syntax error"))
	// runUp's deferred rollback must run.
	mock.ExpectRollback()

	err := m.Up(context.Background())
	if err == nil {
		t.Fatal("Up() should fail when the up SQL fails")
	}
	if want := "migration 1 failed"; !strings.Contains(err.Error(), want) {
		t.Errorf("Up() error = %v, want it to contain %q", err, want)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestMigratorUpRecordError(t *testing.T) {
	m, mock, cleanup := newTestMigrator(t)
	defer cleanup()

	upSQL := "CREATE TABLE users (id INT);"
	m.Add(1, "create_users", upSQL, "DROP TABLE users;")

	mock.ExpectExec(regexp.QuoteMeta(`CREATE TABLE IF NOT EXISTS schema_migrations`)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(MAX(version), 0) FROM schema_migrations")).
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(0))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(upSQL)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO schema_migrations (version, name) VALUES (?, ?)")).
		WithArgs(1, "create_users").
		WillReturnError(errors.New("duplicate version"))
	mock.ExpectRollback()

	err := m.Up(context.Background())
	if err == nil {
		t.Fatal("Up() should fail when recording the migration fails")
	}
	if want := "migration 1 failed"; !strings.Contains(err.Error(), want) {
		t.Errorf("Up() error = %v, want it to contain %q", err, want)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestMigratorUpBeginError(t *testing.T) {
	m, mock, cleanup := newTestMigrator(t)
	defer cleanup()

	m.Add(1, "create_users", "CREATE TABLE users (id INT);", "DROP TABLE users;")

	mock.ExpectExec(regexp.QuoteMeta(`CREATE TABLE IF NOT EXISTS schema_migrations`)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(MAX(version), 0) FROM schema_migrations")).
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(0))
	mock.ExpectBegin().
		WillReturnError(errors.New("connection refused"))

	err := m.Up(context.Background())
	if err == nil {
		t.Fatal("Up() should fail when BeginTx fails")
	}
	if want := "migration 1 failed"; !strings.Contains(err.Error(), want) {
		t.Errorf("Up() error = %v, want it to contain %q", err, want)
	}
}

func TestMigratorDown(t *testing.T) {
	m, mock, cleanup := newTestMigrator(t)
	defer cleanup()

	downSQL := "DROP TABLE users;"
	m.Add(1, "create_users", "CREATE TABLE users (id INT);", downSQL)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(MAX(version), 0) FROM schema_migrations")).
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(1))
	expectDownMigration(mock, 1, downSQL)

	if err := m.Down(context.Background()); err != nil {
		t.Fatalf("Down failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestMigratorDownNoMigrations(t *testing.T) {
	m, mock, cleanup := newTestMigrator(t)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(MAX(version), 0) FROM schema_migrations")).
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(0))

	err := m.Down(context.Background())
	if err == nil {
		t.Error("Expected error when no migrations to rollback")
	}
	if want := "no migrations to rollback"; err.Error() != want {
		t.Errorf("Down() error = %v, want %q", err, want)
	}
}

func TestMigratorDownGetCurrentVersionError(t *testing.T) {
	m, mock, cleanup := newTestMigrator(t)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(MAX(version), 0) FROM schema_migrations")).
		WillReturnError(errors.New("connection lost"))

	if err := m.Down(context.Background()); err == nil {
		t.Fatal("Down() should propagate version query errors")
	}
}

func TestMigratorDownUnknownVersion(t *testing.T) {
	m, mock, cleanup := newTestMigrator(t)
	defer cleanup()

	// Version 3 is recorded but never registered on this migrator.
	m.Add(1, "create_users", "CREATE TABLE users (id INT);", "DROP TABLE users;")

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(MAX(version), 0) FROM schema_migrations")).
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(3))

	err := m.Down(context.Background())
	if err == nil {
		t.Fatal("Down() should fail for an unregistered version")
	}
	if want := "migration 3 not found"; err.Error() != want {
		t.Errorf("Down() error = %v, want %q", err, want)
	}
}

func TestMigratorDownExecError(t *testing.T) {
	m, mock, cleanup := newTestMigrator(t)
	defer cleanup()

	downSQL := "DROP TABLE users;"
	m.Add(1, "create_users", "CREATE TABLE users (id INT);", downSQL)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(MAX(version), 0) FROM schema_migrations")).
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(1))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(downSQL)).
		WillReturnError(errors.New("table locked"))
	mock.ExpectRollback()

	err := m.Down(context.Background())
	if err == nil {
		t.Fatal("Down() should fail when the down SQL fails")
	}
	if want := "down sql failed"; !strings.Contains(err.Error(), want) {
		t.Errorf("Down() error = %v, want it to contain %q", err, want)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestMigratorDownBeginError(t *testing.T) {
	m, mock, cleanup := newTestMigrator(t)
	defer cleanup()

	m.Add(1, "create_users", "CREATE TABLE users (id INT);", "DROP TABLE users;")

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(MAX(version), 0) FROM schema_migrations")).
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(1))
	mock.ExpectBegin().
		WillReturnError(errors.New("too many connections"))

	if err := m.Down(context.Background()); err == nil {
		t.Fatal("Down() should fail when BeginTx fails")
	}
}

func TestMigratorStatus(t *testing.T) {
	m, mock, cleanup := newTestMigrator(t)
	defer cleanup()

	m.Add(1, "create_users", "CREATE TABLE users (id INTEGER PRIMARY KEY);", "DROP TABLE users;")
	m.Add(2, "create_posts", "CREATE TABLE posts (id INTEGER PRIMARY KEY);", "DROP TABLE posts;")

	mock.ExpectExec(regexp.QuoteMeta(`CREATE TABLE IF NOT EXISTS schema_migrations`)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(MAX(version), 0) FROM schema_migrations")).
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(1))

	statuses, err := m.Status(context.Background())
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	if len(statuses) != 2 {
		t.Fatalf("Expected 2 statuses, got %d", len(statuses))
	}

	if !statuses[0].Applied {
		t.Error("First migration should be applied")
	}
	if statuses[1].Applied {
		t.Error("Second migration should not be applied")
	}
	if statuses[0].Name != "create_users" || statuses[1].Name != "create_posts" {
		t.Errorf("Status names = [%s, %s], want [create_users, create_posts]", statuses[0].Name, statuses[1].Name)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestMigratorStatusInitError(t *testing.T) {
	m, mock, cleanup := newTestMigrator(t)
	defer cleanup()

	m.Add(1, "create_users", "CREATE TABLE users (id INT);", "DROP TABLE users;")

	mock.ExpectExec(regexp.QuoteMeta(`CREATE TABLE IF NOT EXISTS schema_migrations`)).
		WillReturnError(errors.New("permission denied"))

	if _, err := m.Status(context.Background()); err == nil {
		t.Fatal("Status() should fail when Init fails")
	}
}

func TestMigratorStatusEmpty(t *testing.T) {
	m, mock, cleanup := newTestMigrator(t)
	defer cleanup()

	mock.ExpectExec(regexp.QuoteMeta(`CREATE TABLE IF NOT EXISTS schema_migrations`)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(MAX(version), 0) FROM schema_migrations")).
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(0))

	statuses, err := m.Status(context.Background())
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if len(statuses) != 0 {
		t.Errorf("Expected 0 statuses, got %d", len(statuses))
	}
}

func TestCustomTableName(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	m := New(db)
	m.TableName = "custom_migrations"

	mock.ExpectExec(regexp.QuoteMeta(`CREATE TABLE IF NOT EXISTS custom_migrations`)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	if err := m.Init(context.Background()); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
