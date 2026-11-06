package slowquery

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestWrapQueryRow(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT id").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))

	logger := New(Config{Threshold: -time.Millisecond}) // always "slow": exercises the warn branch

	var calls int
	wrapped := logger.WrapQueryRow(func(ctx context.Context, query string, args ...any) *sql.Row {
		calls++
		return db.QueryRowContext(ctx, query, args...)
	})

	var id int
	if err := wrapped(t.Context(), "SELECT id FROM users").Scan(&id); err != nil {
		t.Fatalf("wrapped QueryRow().Scan() error = %v", err)
	}
	if id != 7 {
		t.Errorf("id = %d, want 7", id)
	}
	if calls != 1 {
		t.Errorf("underlying calls = %d, want 1", calls)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWrapQueryRow_PreservesRowSemantics(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	// No rows -> Scan must surface sql.ErrNoRows through the wrapper.
	mock.ExpectQuery("SELECT id").WillReturnRows(sqlmock.NewRows([]string{"id"}))

	logger := New(Config{})
	wrapped := logger.WrapQueryRow(db.QueryRowContext)

	var id int
	if err := wrapped(t.Context(), "SELECT id FROM users").Scan(&id); err == nil {
		t.Error("Scan on empty row set should return ErrNoRows")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
