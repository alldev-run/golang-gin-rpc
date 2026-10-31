// Package migration provides database migration support.
// It supports both up and down migrations with version tracking.
package migration

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"
)

// Migration represents a single database migration.
type Migration struct {
	Version   int
	Name      string
	UpSQL     string
	DownSQL   string
	Timestamp time.Time
}

// Migrator handles database migrations.
type Migrator struct {
	DB         *sql.DB
	TableName  string
	migrations []Migration
}

// New creates a new migrator instance.
func New(db *sql.DB) *Migrator {
	return &Migrator{
		DB:         db,
		TableName:  "schema_migrations",
		migrations: make([]Migration, 0),
	}
}

// Add registers a new migration.
func (m *Migrator) Add(version int, name, upSQL, downSQL string) {
	m.migrations = append(m.migrations, Migration{
		Version:   version,
		Name:      name,
		UpSQL:     upSQL,
		DownSQL:   downSQL,
		Timestamp: time.Now(),
	})
}

// Init creates the migration tracking table.
func (m *Migrator) Init(ctx context.Context) error {
	sql := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			version BIGINT PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`, m.TableName)

	_, err := m.DB.ExecContext(ctx, sql)
	return err
}

// GetCurrentVersion returns the current schema version.
func (m *Migrator) GetCurrentVersion(ctx context.Context) (int, error) {
	var version int
	err := m.DB.QueryRowContext(ctx,
		fmt.Sprintf("SELECT COALESCE(MAX(version), 0) FROM %s", m.TableName),
	).Scan(&version)
	return version, err
}

// Up runs all pending migrations.
func (m *Migrator) Up(ctx context.Context) error {
	if err := m.Init(ctx); err != nil {
		return fmt.Errorf("failed to init migrations: %w", err)
	}

	current, err := m.GetCurrentVersion(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	// Sort migrations by version
	sort.Slice(m.migrations, func(i, j int) bool {
		return m.migrations[i].Version < m.migrations[j].Version
	})

	for _, migration := range m.migrations {
		if migration.Version > current {
			if err := m.runUp(ctx, migration); err != nil {
				return fmt.Errorf("migration %d failed: %w", migration.Version, err)
			}
		}
	}

	return nil
}

// Down rolls back the last migration.
func (m *Migrator) Down(ctx context.Context) error {
	current, err := m.GetCurrentVersion(ctx)
	if err != nil {
		return err
	}

	if current == 0 {
		return fmt.Errorf("no migrations to rollback")
	}

	// Find the migration to rollback
	for _, migration := range m.migrations {
		if migration.Version == current {
			return m.runDown(ctx, migration)
		}
	}

	return fmt.Errorf("migration %d not found", current)
}

// runUp executes an up migration.
func (m *Migrator) runUp(ctx context.Context, migration Migration) error {
	tx, err := m.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, migration.UpSQL); err != nil {
		return fmt.Errorf("up sql failed: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		fmt.Sprintf("INSERT INTO %s (version, name) VALUES (?, ?)", m.TableName),
		migration.Version, migration.Name,
	)
	if err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	return tx.Commit()
}

// runDown executes a down migration.
func (m *Migrator) runDown(ctx context.Context, migration Migration) error {
	tx, err := m.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, migration.DownSQL); err != nil {
		return fmt.Errorf("down sql failed: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		fmt.Sprintf("DELETE FROM %s WHERE version = ?", m.TableName),
		migration.Version,
	)
	if err != nil {
		return fmt.Errorf("failed to remove migration record: %w", err)
	}

	return tx.Commit()
}

// Status returns migration status information.
func (m *Migrator) Status(ctx context.Context) ([]MigrationStatus, error) {
	if err := m.Init(ctx); err != nil {
		return nil, err
	}

	current, _ := m.GetCurrentVersion(ctx)

	statuses := make([]MigrationStatus, 0, len(m.migrations))
	for _, migration := range m.migrations {
		statuses = append(statuses, MigrationStatus{
			Version: migration.Version,
			Name:    migration.Name,
			Applied: migration.Version <= current,
		})
	}

	return statuses, nil
}

// MigrationStatus represents the status of a migration.
type MigrationStatus struct {
	Version int
	Name    string
	Applied bool
}
