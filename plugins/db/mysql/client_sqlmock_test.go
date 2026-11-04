package mysql

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"go.uber.org/zap"
)

// newSQLMockClient builds a Client around a sqlmock DB, mirroring the
// postgres client_test.go "direct struct injection" pattern.
func newSQLMockClient(t *testing.T) (*Client, sqlmock.Sqlmock, func()) {
	t.Helper()

	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	client := &Client{db: db, defaultVersionField: "version"}
	return client, mock, func() { _ = db.Close() }
}

func TestQuoteIdentifier(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		want       string
		wantErr    bool
		errMessage string
	}{
		{name: "simple table", input: "users", want: "`users`"},
		{name: "trims whitespace", input: "  users  ", want: "`users`"},
		{name: "qualified name", input: "db.users", want: "`db`.`users`"},
		{name: "with underscores and digits", input: "user_accounts_2", want: "`user_accounts_2`"},
		{name: "empty", input: "", wantErr: true, errMessage: "invalid identifier: empty"},
		{name: "whitespace only", input: "   ", wantErr: true, errMessage: "invalid identifier: empty"},
		{name: "starts with digit", input: "1users", wantErr: true},
		{name: "contains hyphen", input: "my-table", wantErr: true},
		{name: "contains backtick injection", input: "us`ers", wantErr: true},
		{name: "bad qualified part", input: "db.1users", wantErr: true},
		{name: "injection attempt", input: "users; DROP TABLE x", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := quoteIdentifier(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("quoteIdentifier(%q) expected error, got %q", tt.input, got)
				}
				if tt.errMessage != "" && err.Error() != tt.errMessage {
					t.Errorf("quoteIdentifier(%q) error = %q, want %q", tt.input, err.Error(), tt.errMessage)
				}
				return
			}
			if err != nil {
				t.Fatalf("quoteIdentifier(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("quoteIdentifier(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeConfig(t *testing.T) {
	tests := []struct {
		name  string
		input Config
		check func(t *testing.T, got Config)
	}{
		{
			name:  "empty config gets defaults",
			input: Config{},
			check: func(t *testing.T, got Config) {
				if got.Charset != "utf8mb4" {
					t.Errorf("Charset = %q, want utf8mb4", got.Charset)
				}
				if got.MaxOpenConns != 25 {
					t.Errorf("MaxOpenConns = %d, want 25", got.MaxOpenConns)
				}
				if got.MaxIdleConns != 10 {
					t.Errorf("MaxIdleConns = %d, want 10", got.MaxIdleConns)
				}
				if got.ConnMaxLifetime != time.Hour {
					t.Errorf("ConnMaxLifetime = %v, want 1h", got.ConnMaxLifetime)
				}
				if got.ConnMaxIdleTime != 30*time.Minute {
					t.Errorf("ConnMaxIdleTime = %v, want 30m", got.ConnMaxIdleTime)
				}
			},
		},
		{
			name: "explicit values preserved",
			input: Config{
				Charset:         "latin1",
				MaxOpenConns:    40,
				MaxIdleConns:    20,
				ConnMaxLifetime: 2 * time.Hour,
				ConnMaxIdleTime: 10 * time.Minute,
			},
			check: func(t *testing.T, got Config) {
				if got.Charset != "latin1" {
					t.Errorf("Charset = %q, want latin1", got.Charset)
				}
				if got.MaxOpenConns != 40 {
					t.Errorf("MaxOpenConns = %d, want 40", got.MaxOpenConns)
				}
				if got.MaxIdleConns != 20 {
					t.Errorf("MaxIdleConns = %d, want 20", got.MaxIdleConns)
				}
				if got.ConnMaxLifetime != 2*time.Hour {
					t.Errorf("ConnMaxLifetime = %v, want 2h", got.ConnMaxLifetime)
				}
				if got.ConnMaxIdleTime != 10*time.Minute {
					t.Errorf("ConnMaxIdleTime = %v, want 10m", got.ConnMaxIdleTime)
				}
			},
		},
		{
			name: "negative values get defaults",
			input: Config{
				MaxOpenConns:    -5,
				MaxIdleConns:    -3,
				ConnMaxLifetime: -time.Second,
				ConnMaxIdleTime: -time.Second,
			},
			check: func(t *testing.T, got Config) {
				if got.MaxOpenConns != 25 {
					t.Errorf("MaxOpenConns = %d, want 25", got.MaxOpenConns)
				}
				if got.MaxIdleConns != 10 {
					t.Errorf("MaxIdleConns = %d, want 10", got.MaxIdleConns)
				}
			},
		},
		{
			name: "max idle clamped to max open",
			input: Config{
				MaxOpenConns: 5,
				MaxIdleConns: 50,
			},
			check: func(t *testing.T, got Config) {
				if got.MaxIdleConns != 5 {
					t.Errorf("MaxIdleConns = %d, want clamped to MaxOpenConns 5", got.MaxIdleConns)
				}
				if got.MaxOpenConns != 5 {
					t.Errorf("MaxOpenConns = %d, want 5", got.MaxOpenConns)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, normalizeConfig(tt.input))
		})
	}
}

func TestIsZero(t *testing.T) {
	// NOTE: the multi-type cases in isZero compare the boxed interface value
	// against the untyped constant 0 (whose default type is int), so only
	// plain int zero values compare equal. The expectations below pin down
	// the current behavior; do not "fix" them without changing production code.
	tests := []struct {
		name  string
		value interface{}
		want  bool
	}{
		{name: "nil", value: nil, want: true},
		{name: "zero int", value: 0, want: true},
		{name: "non-zero int", value: 42, want: false},
		{name: "zero int64 (dynamic type mismatch)", value: int64(0), want: false},
		{name: "non-zero int8", value: int8(3), want: false},
		{name: "zero uint (dynamic type mismatch)", value: uint(0), want: false},
		{name: "non-zero uint64", value: uint64(9), want: false},
		{name: "zero float (dynamic type mismatch)", value: 0.0, want: false},
		{name: "non-zero float", value: 0.5, want: false},
		{name: "empty string", value: "", want: true},
		{name: "non-empty string", value: "x", want: false},
		{name: "false bool", value: false, want: true},
		{name: "true bool", value: true, want: false},
		{name: "struct", value: struct{ A int }{A: 1}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isZero(tt.value); got != tt.want {
				t.Errorf("isZero(%v) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestBuildInsertQuery(t *testing.T) {
	client := &Client{defaultVersionField: "version"}

	query, args, err := client.buildInsertQuery("users", map[string]interface{}{
		"name":  "alice",
		"email": "a@example.com",
	})
	if err != nil {
		t.Fatalf("buildInsertQuery error = %v", err)
	}

	wantQuery := "INSERT INTO `users` (`email`, `name`, `version`) VALUES (?, ?, ?)"
	if query != wantQuery {
		t.Errorf("query = %q, want %q", query, wantQuery)
	}
	// Columns are sorted; version defaults to 1 when absent.
	wantArgs := []interface{}{"a@example.com", "alice", 1}
	if len(args) != len(wantArgs) {
		t.Fatalf("args = %v, want %v", args, wantArgs)
	}
	for i, want := range wantArgs {
		if args[i] != want {
			t.Errorf("args[%d] = %v, want %v", i, args[i], want)
		}
	}
}

func TestBuildInsertQueryKeepsExplicitVersion(t *testing.T) {
	client := &Client{defaultVersionField: "version"}

	query, args, err := client.buildInsertQuery("users", map[string]interface{}{
		"name":    "bob",
		"version": 7,
	})
	if err != nil {
		t.Fatalf("buildInsertQuery error = %v", err)
	}

	if query != "INSERT INTO `users` (`name`, `version`) VALUES (?, ?)" {
		t.Errorf("query = %q", query)
	}
	if len(args) != 2 || args[1] != 7 {
		t.Errorf("args = %v, want explicit version 7 preserved", args)
	}
}

func TestBuildInsertQueryDefaultsVersionOnEmptyData(t *testing.T) {
	client := &Client{defaultVersionField: "version"}

	// The version field is injected before the emptiness check, so an empty
	// data map still yields a version-only insert.
	query, args, err := client.buildInsertQuery("users", map[string]interface{}{})
	if err != nil {
		t.Fatalf("buildInsertQuery error = %v", err)
	}
	if query != "INSERT INTO `users` (`version`) VALUES (?)" {
		t.Errorf("query = %q", query)
	}
	if len(args) != 1 || args[0] != 1 {
		t.Errorf("args = %v, want [1]", args)
	}
}

func TestBuildInsertQueryInvalidTable(t *testing.T) {
	client := &Client{defaultVersionField: "version"}

	_, _, err := client.buildInsertQuery("bad-table", map[string]interface{}{"name": "x"})
	if err == nil {
		t.Fatal("expected error for invalid table name")
	}
}

func TestBuildInsertQueryInvalidColumn(t *testing.T) {
	client := &Client{defaultVersionField: "version"}

	_, _, err := client.buildInsertQuery("users", map[string]interface{}{"bad col": "x"})
	if err == nil {
		t.Fatal("expected error for invalid column name")
	}
}

func TestBuildUpdateQuery(t *testing.T) {
	client := &Client{defaultVersionField: "version"}

	query, args, err := client.buildUpdateQuery("users", "id", 5, map[string]interface{}{
		"name":    "carol",
		"version": 2,
	})
	if err != nil {
		t.Fatalf("buildUpdateQuery error = %v", err)
	}

	// The supplied version is moved out of the SET clause into an optimistic
	// locking WHERE condition.
	wantQuery := "UPDATE `users` SET `name` = ? WHERE `id` = ? AND `version` = ?"
	if query != wantQuery {
		t.Errorf("query = %q, want %q", query, wantQuery)
	}
	wantArgs := []interface{}{"carol", 5, 2}
	if len(args) != len(wantArgs) {
		t.Fatalf("args = %v, want %v", args, wantArgs)
	}
	for i, want := range wantArgs {
		if args[i] != want {
			t.Errorf("args[%d] = %v, want %v", i, args[i], want)
		}
	}
}

func TestBuildUpdateQueryWithoutVersion(t *testing.T) {
	client := &Client{defaultVersionField: "version"}

	query, args, err := client.buildUpdateQuery("users", "id", 9, map[string]interface{}{
		"name": "dave",
		"age":  30,
	})
	if err != nil {
		t.Fatalf("buildUpdateQuery error = %v", err)
	}

	wantQuery := "UPDATE `users` SET `age` = ?, `name` = ? WHERE `id` = ?"
	if query != wantQuery {
		t.Errorf("query = %q, want %q", query, wantQuery)
	}
	if len(args) != 3 || args[0] != 30 || args[1] != "dave" || args[2] != 9 {
		t.Errorf("args = %v, want [30 dave 9]", args)
	}
}

func TestBuildUpdateQueryEmptyData(t *testing.T) {
	client := &Client{defaultVersionField: "version"}

	query, args, err := client.buildUpdateQuery("users", "id", 1, map[string]interface{}{})
	if err != nil {
		t.Fatalf("buildUpdateQuery error = %v", err)
	}
	if query != "" || args != nil {
		t.Errorf("empty data should return empty query, got %q / %v", query, args)
	}
}

func TestBuildUpdateQueryInvalidIdentifier(t *testing.T) {
	client := &Client{defaultVersionField: "version"}

	if _, _, err := client.buildUpdateQuery("bad-table", "id", 1, map[string]interface{}{"name": "x"}); err == nil {
		t.Error("expected error for invalid table name")
	}
	if _, _, err := client.buildUpdateQuery("users", "bad id", 1, map[string]interface{}{"name": "x"}); err == nil {
		t.Error("expected error for invalid id column")
	}
}

func TestClientUninitialized(t *testing.T) {
	var client *Client
	ctx := context.Background()

	if err := client.Close(); !errors.Is(err, ErrClientNotInitialized) {
		t.Errorf("nil client Close() = %v, want ErrClientNotInitialized", err)
	}
	if err := client.Ping(ctx); !errors.Is(err, ErrClientNotInitialized) {
		t.Errorf("nil client Ping() = %v, want ErrClientNotInitialized", err)
	}
	if got := client.Stats(); got.MaxOpenConnections != 0 {
		t.Errorf("nil client Stats() = %+v, want zero value", got)
	}
	if _, err := client.Query(ctx, "SELECT 1"); !errors.Is(err, ErrClientNotInitialized) {
		t.Errorf("nil client Query() = %v, want ErrClientNotInitialized", err)
	}
	if row := client.QueryRow(ctx, "SELECT 1"); row == nil {
		t.Error("nil client QueryRow() should return a non-nil row (carrying the error)")
	}
	if _, err := client.Exec(ctx, "SELECT 1"); !errors.Is(err, ErrClientNotInitialized) {
		t.Errorf("nil client Exec() = %v, want ErrClientNotInitialized", err)
	}
	if _, err := client.Begin(ctx, nil); !errors.Is(err, ErrClientNotInitialized) {
		t.Errorf("nil client Begin() = %v, want ErrClientNotInitialized", err)
	}
	if _, err := client.Prepare(ctx, "SELECT 1"); !errors.Is(err, ErrClientNotInitialized) {
		t.Errorf("nil client Prepare() = %v, want ErrClientNotInitialized", err)
	}

	empty := &Client{}
	if err := empty.Close(); !errors.Is(err, ErrClientNotInitialized) {
		t.Errorf("empty client Close() = %v, want ErrClientNotInitialized", err)
	}
	if err := empty.ensureDB(); !errors.Is(err, ErrClientNotInitialized) {
		t.Errorf("empty client ensureDB() = %v, want ErrClientNotInitialized", err)
	}
}

func TestClientQueryExecWithSQLMock(t *testing.T) {
	client, mock, cleanup := newSQLMockClient(t)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name FROM users WHERE id = ?")).
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "alice"))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE users SET name = ? WHERE id = ?")).
		WithArgs("bob", 1).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO users (name) VALUES (?)")).
		WithArgs("carol").
		WillReturnResult(sqlmock.NewResult(42, 1))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM users WHERE id = ?")).
		WithArgs(2).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectPing()

	ctx := context.Background()

	rows, err := client.Query(ctx, "SELECT id, name FROM users WHERE id = ?", 1)
	if err != nil {
		t.Fatalf("Query error = %v", err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatal("expected one row")
	}
	var id int
	var name string
	if err := rows.Scan(&id, &name); err != nil {
		t.Fatalf("Scan error = %v", err)
	}
	if id != 1 || name != "alice" {
		t.Errorf("row = (%d, %s), want (1, alice)", id, name)
	}

	// Update returns rows affected.
	affected, err := client.Update(ctx, "UPDATE users SET name = ? WHERE id = ?", "bob", 1)
	if err != nil {
		t.Fatalf("Update error = %v", err)
	}
	if affected != 1 {
		t.Errorf("Update affected = %d, want 1", affected)
	}

	// InsertGetID returns last insert id.
	newID, err := client.InsertGetID(ctx, "INSERT INTO users (name) VALUES (?)", "carol")
	if err != nil {
		t.Fatalf("InsertGetID error = %v", err)
	}
	if newID != 42 {
		t.Errorf("InsertGetID = %d, want 42", newID)
	}

	// Exec passes through.
	if _, err := client.Exec(ctx, "DELETE FROM users WHERE id = ?", 2); err != nil {
		t.Fatalf("Exec error = %v", err)
	}

	if err := client.Ping(ctx); err != nil {
		t.Errorf("Ping error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestClientQueryRowAndStats(t *testing.T) {
	client, mock, cleanup := newSQLMockClient(t)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT name FROM users WHERE id = ?")).
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("alice"))

	var name string
	if err := client.QueryRow(context.Background(), "SELECT name FROM users WHERE id = ?", 1).Scan(&name); err != nil {
		t.Fatalf("QueryRow Scan error = %v", err)
	}
	if name != "alice" {
		t.Errorf("name = %q, want alice", name)
	}

	_ = client.Stats()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestSetFieldByID(t *testing.T) {
	client, mock, cleanup := newSQLMockClient(t)
	defer cleanup()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE `users` SET `status` = ? WHERE `id` = ?")).
		WithArgs("active", 7).
		WillReturnResult(sqlmock.NewResult(0, 1))

	affected, err := client.SetFieldByID(context.Background(), "users", "id", 7, "status", "active")
	if err != nil {
		t.Fatalf("SetFieldByID error = %v", err)
	}
	if affected != 1 {
		t.Errorf("affected = %d, want 1", affected)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestSetFieldByIDInvalidIdentifiers(t *testing.T) {
	client, _, cleanup := newSQLMockClient(t)
	defer cleanup()

	if _, err := client.SetFieldByID(context.Background(), "bad-table", "id", 1, "status", "x"); err == nil {
		t.Error("expected error for invalid table")
	}
	if _, err := client.SetFieldByID(context.Background(), "users", "id", 1, "bad field", "x"); err == nil {
		t.Error("expected error for invalid field")
	}
	if _, err := client.SetFieldByID(context.Background(), "users", "bad id", 1, "status", "x"); err == nil {
		t.Error("expected error for invalid id column")
	}
}

func TestSaveInsertAndUpdate(t *testing.T) {
	client, mock, cleanup := newSQLMockClient(t)
	defer cleanup()

	// id == 0 → INSERT path (version defaulted to 1).
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `users` (`name`, `version`) VALUES (?, ?)")).
		WithArgs("erin", 1).
		WillReturnResult(sqlmock.NewResult(11, 1))

	newID, err := client.Save(context.Background(), "users", "id", 0, map[string]interface{}{"name": "erin"})
	if err != nil {
		t.Fatalf("Save(insert) error = %v", err)
	}
	if newID != 11 {
		t.Errorf("Save(insert) id = %d, want 11", newID)
	}

	// id != 0 → UPDATE path with optimistic version check.
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `users` SET `name` = ? WHERE `id` = ? AND `version` = ?")).
		WithArgs("erin-v2", 11, 3).
		WillReturnResult(sqlmock.NewResult(0, 1))

	affected, err := client.Save(context.Background(), "users", "id", 11, map[string]interface{}{"name": "erin-v2", "version": 3})
	if err != nil {
		t.Fatalf("Save(update) error = %v", err)
	}
	if affected != 1 {
		t.Errorf("Save(update) affected = %d, want 1", affected)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestSaveEmptyDataInsertsVersionOnly(t *testing.T) {
	client, mock, cleanup := newSQLMockClient(t)
	defer cleanup()

	// An empty data map still produces the defaulted version column.
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `users` (`version`) VALUES (?)")).
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(3, 1))

	newID, err := client.Save(context.Background(), "users", "id", 0, map[string]interface{}{})
	if err != nil {
		t.Fatalf("Save(insert with empty data) error = %v", err)
	}
	if newID != 3 {
		t.Errorf("Save(insert with empty data) id = %d, want 3", newID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestTransactionCommit(t *testing.T) {
	client, mock, cleanup := newSQLMockClient(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO users (name) VALUES (?)")).
		WithArgs("frank").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := client.Transaction(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO users (name) VALUES (?)", "frank")
		return err
	})
	if err != nil {
		t.Fatalf("Transaction error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestTransactionRollbackOnError(t *testing.T) {
	client, mock, cleanup := newSQLMockClient(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectRollback()

	wantErr := errors.New("business rule violated")
	err := client.Transaction(context.Background(), func(tx *sql.Tx) error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Errorf("Transaction error = %v, want %v", err, wantErr)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestTransactionRollbackOnPanic(t *testing.T) {
	client, mock, cleanup := newSQLMockClient(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectRollback()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic to propagate after rollback")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unmet expectations: %v", err)
		}
	}()

	_ = client.Transaction(context.Background(), func(tx *sql.Tx) error {
		panic("boom")
	})
}

func TestTransactionBeginError(t *testing.T) {
	client, mock, cleanup := newSQLMockClient(t)
	defer cleanup()

	mock.ExpectBegin().WillReturnError(errors.New("db closed"))

	err := client.Transaction(context.Background(), func(tx *sql.Tx) error { return nil })
	if err == nil {
		t.Fatal("Transaction should fail when Begin fails")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestClientBeginAndPrepare(t *testing.T) {
	client, mock, cleanup := newSQLMockClient(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectPrepare(regexp.QuoteMeta("SELECT name FROM users WHERE id = ?")).
		ExpectQuery().
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("alice"))

	tx, err := client.Begin(context.Background(), nil)
	if err != nil {
		t.Fatalf("Begin error = %v", err)
	}

	stmt, err := client.Prepare(context.Background(), "SELECT name FROM users WHERE id = ?")
	if err != nil {
		t.Fatalf("Prepare error = %v", err)
	}

	var name string
	if err := stmt.QueryRow(1).Scan(&name); err != nil {
		t.Fatalf("stmt.QueryRow error = %v", err)
	}
	if name != "alice" {
		t.Errorf("name = %q, want alice", name)
	}

	mock.ExpectRollback()
	if err := tx.Rollback(); err != nil {
		t.Errorf("tx.Rollback error = %v", err)
	}
	if err := stmt.Close(); err != nil {
		t.Errorf("stmt.Close error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestNewWithBadDSNHost(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Host = "256.256.256.256" // unreachable/invalid address
	cfg.Username = "u"
	cfg.Password = "p"
	cfg.Database = "d"

	if _, err := New(cfg); err == nil {
		t.Fatal("New() should fail when ping to an invalid host fails")
	}
}

func TestNewSQLLoggerWithLogger(t *testing.T) {
	nop := zap.NewNop()

	// Ensure NewSQLLoggerWithLogger wires the custom logger and all LogQuery
	// branches run without panicking.
	logger := NewSQLLoggerWithLogger("debug", 10*time.Millisecond, nop)
	if logger == nil {
		t.Fatal("NewSQLLoggerWithLogger returned nil")
	}
	if logger.logger != nop {
		t.Error("custom logger should be stored on the SQLLogger")
	}

	logger.SetLogger(zap.NewNop())

	logger.LogQuery("SELECT 1", []interface{}{1}, 5*time.Millisecond, nil)
	logger.LogQuery("SELECT 1", nil, 50*time.Millisecond, nil) // slow
	logger.LogQuery("SELECT 1", nil, 5*time.Millisecond, errors.New("boom"))

	for _, level := range []string{"error", "warn", "debug", "trace", "unknown"} {
		l := NewSQLLoggerWithLogger(level, time.Second, zap.NewNop())
		l.LogQuery("SELECT 1", nil, time.Millisecond, nil)
	}
}

func TestClientSetSQLLogger(t *testing.T) {
	client, _, cleanup := newSQLMockClient(t)
	defer cleanup()

	// No-op when no logger is configured.
	client.SetSQLLogger(nil)

	client.sqlLogger = NewSQLLogger("info", time.Second)
	client.SetSQLLogger(zap.NewNop())
	if client.sqlLogger.logger == nil {
		t.Error("SetSQLLogger should replace the underlying zap logger")
	}
}
