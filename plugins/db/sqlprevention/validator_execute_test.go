package sqlprevention

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestParameterizedQuery_Execute(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT name FROM users WHERE id = \\?").
		WithArgs(42).
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("alice"))

	pq := NewParameterizedQuery("SELECT name FROM users WHERE id = ?").AddParam(42)

	rows, err := pq.Execute(db)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		t.Fatal("expected one row")
	}
	var name string
	if err := rows.Scan(&name); err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if name != "alice" {
		t.Errorf("name = %q, want alice", name)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestParameterizedQuery_ExecuteError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT").WillReturnError(errors.New("injected failure"))

	pq := NewParameterizedQuery("SELECT 1")
	if _, err := pq.Execute(db); err == nil {
		t.Error("Execute() should propagate query errors")
	}
}
