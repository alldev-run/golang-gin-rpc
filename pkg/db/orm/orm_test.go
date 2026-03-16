// Package orm provides unit tests for the database-agnostic ORM layer.
package orm

import (
	"context"
	"database/sql"
	"reflect"
	"testing"
)

// assertQueryAndArgs checks if the query and args match the expected values.
func assertQueryAndArgs(t *testing.T, query string, args []interface{}, expectedQuery string, expectedArgs []interface{}) {
	t.Helper()
	if query != expectedQuery || !reflect.DeepEqual(args, expectedArgs) {
		t.Errorf("Expected query='%s', args=%v, got query='%s', args=%v", expectedQuery, expectedArgs, query, args)
	}
}

// MockDB implements the DB interface for testing purposes.
type MockDB struct {
	QueryFunc    func(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowFunc func(ctx context.Context, query string, args ...interface{}) *sql.Row
	ExecFunc     func(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	BeginFunc    func(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
	PingFunc     func(ctx context.Context) error
	StatsFunc    func() sql.DBStats
	CloseFunc    func() error
}

// Query calls the mock QueryFunc.
func (m *MockDB) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if m.QueryFunc != nil {
		return m.QueryFunc(ctx, query, args...)
	}
	return nil, nil
}

// QueryRow calls the mock QueryRowFunc.
func (m *MockDB) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if m.QueryRowFunc != nil {
		return m.QueryRowFunc(ctx, query, args...)
	}
	return nil
}

// Exec calls the mock ExecFunc.
func (m *MockDB) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if m.ExecFunc != nil {
		return m.ExecFunc(ctx, query, args...)
	}
	return nil, nil
}

// Close calls the mock CloseFunc.
func (m *MockDB) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

// Ping calls the mock PingFunc.
func (m *MockDB) Ping(ctx context.Context) error {
	if m.PingFunc != nil {
		return m.PingFunc(ctx)
	}
	return nil
}

// Stats calls the mock StatsFunc.
func (m *MockDB) Stats() sql.DBStats {
	if m.StatsFunc != nil {
		return m.StatsFunc()
	}
	return sql.DBStats{}
}

// Begin calls the mock BeginFunc.
func (m *MockDB) Begin(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	if m.BeginFunc != nil {
		return m.BeginFunc(ctx, opts)
	}
	return nil, nil
}

// MockResult implements sql.Result for testing.
type MockResult struct {
	lastInsertID int64
	rowsAffected int64
}

func (m *MockResult) LastInsertId() (int64, error) {
	return m.lastInsertID, nil
}

func (m *MockResult) RowsAffected() (int64, error) {
	return m.rowsAffected, nil
}

func TestDefaultDialect(t *testing.T) {
	d := DefaultDialect{}

	if d.LockForUpdate() != "FOR UPDATE" {
		t.Errorf("Expected 'FOR UPDATE', got '%s'", d.LockForUpdate())
	}

	if d.LockInShareMode() != "LOCK IN SHARE MODE" {
		t.Errorf("Expected 'LOCK IN SHARE MODE', got '%s'", d.LockInShareMode())
	}

	if d.QuoteIdentifier("test") != "`test`" {
		t.Errorf("Expected '`test`', got '%s'", d.QuoteIdentifier("test"))
	}
}

func TestWhereBuilder(t *testing.T) {
	wb := NewWhereBuilder(NewDefaultDialect())

	// Test empty builder
	where, args := wb.Build()
	if where != "" || len(args) != 0 {
		t.Errorf("Empty WhereBuilder should return empty string and no args, got where='%s', args=%v", where, args)
	}

	// Test single WHERE condition
	wb.Where("id = ?", 1)
	where, args = wb.Build()
	expectedWhere := "WHERE id = ?"
	if where != expectedWhere || len(args) != 1 || args[0] != 1 {
		t.Errorf("Expected where='%s', args=%v, got where='%s', args=%v", expectedWhere, []interface{}{1}, where, args)
	}

	// Test AND condition
	wb.And("status = ?", "active")
	where, args = wb.Build()
	expectedWhere = "WHERE id = ? AND status = ?"
	expectedArgs := []interface{}{1, "active"}
	if where != expectedWhere || !reflect.DeepEqual(args, expectedArgs) {
		t.Errorf("Expected where='%s', args=%v, got where='%s', args=%v", expectedWhere, expectedArgs, where, args)
	}

	// Test OR condition
	wb2 := NewWhereBuilder(NewDefaultDialect())
	wb2.Where("id = ?", 1).Or("status = ?", "active")
	where, args = wb2.Build()
	expectedWhere = "WHERE id = ? OR status = ?"
	expectedArgs = []interface{}{1, "active"}
	if where != expectedWhere || !reflect.DeepEqual(args, expectedArgs) {
		t.Errorf("Expected where='%s', args=%v, got where='%s', args=%v", expectedWhere, expectedArgs, where, args)
	}
}

func TestSelectBuilder(t *testing.T) {
	mockDB := &MockDB{}
	sb := NewSelectBuilder(mockDB, "users")
	var expectedArgs []interface{}

	// Test basic SELECT
	sb.Columns("id", "name", "email")
	query, args := sb.Build()
	expectedQuery := "SELECT `id`, `name`, `email` FROM `users`"
	if query != expectedQuery || len(args) != 0 {
		t.Errorf("Expected query='%s', args=%v, got query='%s', args=%v", expectedQuery, []interface{}{}, query, args)
	}

	// Test WHERE condition
	sb.Where("id = ?", 1)
	query, args = sb.Build()
	expectedQuery = "SELECT `id`, `name`, `email` FROM `users` WHERE id = ?"
	if query != expectedQuery || len(args) != 1 || args[0] != 1 {
		t.Errorf("Expected query='%s', args=%v, got query='%s', args=%v", expectedQuery, []interface{}{1}, query, args)
	}

	// Test LIMIT
	sb.Limit(10)
	query, args = sb.Build()
	expectedQuery = "SELECT `id`, `name`, `email` FROM `users` WHERE id = ? LIMIT 10"
	if query != expectedQuery {
		t.Errorf("Expected query='%s', got query='%s'", expectedQuery, query)
	}

	// Test with ORDER BY
	sb.OrderBy("created_at DESC")
	query, args = sb.Build()
	expectedQuery = "SELECT `id`, `name`, `email` FROM `users` WHERE id = ? ORDER BY created_at DESC"
	expectedArgs = []interface{}{1}
	if query != expectedQuery || !reflect.DeepEqual(args, expectedArgs) {
		t.Errorf("Expected query='%s', args=%v, got query='%s', args=%v", expectedQuery, expectedArgs, query, args)
	}

	// Test with LIMIT
	sb.Limit(10)
	query, args = sb.Build()
	expectedQuery = "SELECT `id`, `name`, `email` FROM `users` WHERE id = ? ORDER BY created_at DESC LIMIT 10"
	expectedArgs = []interface{}{1}
	if query != expectedQuery || !reflect.DeepEqual(args, expectedArgs) {
		t.Errorf("Expected query='%s', args=%v, got query='%s', args=%v", expectedQuery, expectedArgs, query, args)
	}

	// Test with OFFSET
	sb.Offset(20)
	query, args = sb.Build()
	expectedQuery = "SELECT `id`, `name`, `email` FROM `users` WHERE id = ? ORDER BY created_at DESC LIMIT 10 OFFSET 20"
	expectedArgs = []interface{}{1}
	if query != expectedQuery || !reflect.DeepEqual(args, expectedArgs) {
		t.Errorf("Expected query='%s', args=%v, got query='%s', args=%v", expectedQuery, expectedArgs, query, args)
	}

	// Test FOR UPDATE
	sb.ForUpdate()
	query, args = sb.Build()
	expectedQuery = "SELECT `id`, `name`, `email` FROM `users` WHERE id = ? ORDER BY created_at DESC LIMIT 10 OFFSET 20 FOR UPDATE"
	expectedArgs = []interface{}{1}
	if query != expectedQuery || !reflect.DeepEqual(args, expectedArgs) {
		t.Errorf("Expected query='%s', args=%v, got query='%s', args=%v", expectedQuery, expectedArgs, query, args)
	}
}

func TestSelectBuilderJoins(t *testing.T) {
	mockDB := &MockDB{}
	sb := NewSelectBuilder(mockDB, "users")

	// Test INNER JOIN
	sb.Join("profiles", "users.id = profiles.user_id")
	query, args := sb.Build()
	expectedQuery := "SELECT * FROM users INNER JOIN profiles ON users.id = profiles.user_id"
	if query != expectedQuery || len(args) != 0 {
		t.Errorf("Expected query='%s', args=%v, got query='%s', args=%v", expectedQuery, []interface{}{}, query, args)
	}

	// Test LEFT JOIN
	sb2 := NewSelectBuilder(mockDB, "users")
	sb2.LeftJoin("profiles", "users.id = profiles.user_id")
	query, args = sb2.Build()
	expectedQuery = "SELECT * FROM users LEFT JOIN profiles ON users.id = profiles.user_id"
	if query != expectedQuery || len(args) != 0 {
		t.Errorf("Expected query='%s', args=%v, got query='%s', args=%v", expectedQuery, []interface{}{}, query, args)
	}

	// Test RIGHT JOIN
	sb3 := NewSelectBuilder(mockDB, "users")
	sb3.RightJoin("profiles", "users.id = profiles.user_id")
	query, args = sb3.Build()
	expectedQuery = "SELECT * FROM users RIGHT JOIN profiles ON users.id = profiles.user_id"
	if query != expectedQuery || len(args) != 0 {
		t.Errorf("Expected query='%s', args=%v, got query='%s', args=%v", expectedQuery, []interface{}{}, query, args)
	}

	// Test FULL OUTER JOIN
	sb4 := NewSelectBuilder(mockDB, "users")
	sb4.FullOuterJoin("profiles", "users.id = profiles.user_id")
	query, args = sb4.Build()
	expectedQuery = "SELECT * FROM users FULL OUTER JOIN profiles ON users.id = profiles.user_id"
	if query != expectedQuery || len(args) != 0 {
		t.Errorf("Expected query='%s', args=%v, got query='%s', args=%v", expectedQuery, []interface{}{}, query, args)
	}
}

func TestSelectBuilderGroupByHaving(t *testing.T) {
	mockDB := &MockDB{}
	sb := NewSelectBuilder(mockDB, "orders")

	sb.Columns("status", "COUNT(*)")
	sb.GroupBy("status")
	sb.Having("COUNT(*) > ?", 5)

	query, args := sb.Build()
	expectedQuery := "SELECT status, COUNT(*) FROM orders GROUP BY status HAVING COUNT(*) > ?"
	expectedArgs := []interface{}{5}
	if query != expectedQuery || !reflect.DeepEqual(args, expectedArgs) {
		t.Errorf("Expected query='%s', args=%v, got query='%s', args=%v", expectedQuery, expectedArgs, query, args)
	}

	// Test HAVING with AND
	sb.HavingAnd("status != ?", "cancelled")
	query, args = sb.Build()
	expectedQuery = "SELECT status, COUNT(*) FROM orders GROUP BY status HAVING COUNT(*) > ? AND status != ?"
	expectedArgs = []interface{}{5, "cancelled"}
	if query != expectedQuery || !reflect.DeepEqual(args, expectedArgs) {
		t.Errorf("Expected query='%s', args=%v, got query='%s', args=%v", expectedQuery, expectedArgs, query, args)
	}
}

func TestDeleteBuilder(t *testing.T) {
	mockDB := &MockDB{}
	db := NewDeleteBuilder(mockDB, "users")
	var expectedArgs []interface{}

	// Test basic DELETE
	query, args := db.Build()
	expectedQuery := "DELETE FROM users"
	if query != expectedQuery || len(args) != 0 {
		t.Errorf("Expected query='%s', args=%v, got query='%s', args=%v", expectedQuery, []interface{}{}, query, args)
	}

	// Test with WHERE
	db.Where("status = ?", "inactive")
	query, args = db.Build()
	expectedQuery = "DELETE FROM users WHERE status = ?"
	expectedArgs = []interface{}{"inactive"}
	if query != expectedQuery || !reflect.DeepEqual(args, expectedArgs) {
		t.Errorf("Expected query='%s', args=%v, got query='%s', args=%v", expectedQuery, expectedArgs, query, args)
	}

	// Test with LIMIT
	db.Limit(100)
	query, args = db.Build()
	expectedQuery = "DELETE FROM users WHERE status = ? LIMIT 100"
	expectedArgs = []interface{}{"inactive"}
	if query != expectedQuery || !reflect.DeepEqual(args, expectedArgs) {
		t.Errorf("Expected query='%s', args=%v, got query='%s', args=%v", expectedQuery, expectedArgs, query, args)
	}
}

func TestBuildInsertQuery(t *testing.T) {
	// Test basic insert
	data := map[string]interface{}{
		"name":  "John Doe",
		"email": "john@example.com",
	}
	query, args, err := BuildInsertQuery("users", data, NewDefaultDialect())
	if err != nil {
		t.Fatalf("Failed to build insert query: %v", err)
	}
	expectedQuery := "INSERT INTO `users` (`email`, `name`) VALUES (?, ?)"
	expectedArgs := []interface{}{"john@example.com", "John Doe"}
	if query != expectedQuery || !reflect.DeepEqual(args, expectedArgs) {
		t.Errorf("Expected query='%s', args=%v, got query='%s', args=%v", expectedQuery, expectedArgs, query, args)
	}

	// Test insert with existing version
	dataWithVersion := map[string]interface{}{
		"name":    "Jane Doe",
		"email":   "jane@example.com",
		"version": 5,
	}
	query, args, err = BuildInsertQuery("users", dataWithVersion, NewDefaultDialect())
	if err != nil {
		t.Fatalf("Failed to build insert query: %v", err)
	}
	expectedQuery = "INSERT INTO `users` (`email`, `name`, `version`) VALUES (?, ?, ?)"
	expectedArgs = []interface{}{"jane@example.com", "Jane Doe", 5}
	if query != expectedQuery || !reflect.DeepEqual(args, expectedArgs) {
		t.Errorf("Expected query='%s', args=%v, got query='%s', args=%v", expectedQuery, expectedArgs, query, args)
	}
}

func TestBuildUpdateQuery(t *testing.T) {
	// Test basic update
	data := map[string]interface{}{
		"name":  "John Smith",
		"email": "johnsmith@example.com",
	}
	query, args, err := BuildUpdateQuery("users", "id", 1, data, NewDefaultDialect())
	if err != nil {
		t.Fatalf("Failed to build update query: %v", err)
	}
	expectedQuery := "UPDATE `users` SET `email` = ?, `name` = ? WHERE `id` = ?"
	expectedArgs := []interface{}{"johnsmith@example.com", "John Smith", 1}
	if query != expectedQuery || !reflect.DeepEqual(args, expectedArgs) {
		t.Errorf("Expected query='%s', args=%v, got query='%s', args=%v", expectedQuery, expectedArgs, query, args)
	}

	// Test update with version
	dataWithVersion := map[string]interface{}{
		"name":    "Jane Smith",
		"email":   "janesmith@example.com",
		"version": 3,
	}
	query, args, err = BuildUpdateQuery("users", "id", 2, dataWithVersion, NewDefaultDialect())
	if err != nil {
		t.Fatalf("Failed to build update query: %v", err)
	}
	expectedQuery = "UPDATE `users` SET `email` = ?, `name` = ?, `version` = ? WHERE `id` = ?"
	expectedArgs = []interface{}{"janesmith@example.com", "Jane Smith", 3, 2}
	if query != expectedQuery || !reflect.DeepEqual(args, expectedArgs) {
		t.Errorf("Expected query='%s', args=%v, got query='%s', args=%v", expectedQuery, expectedArgs, query, args)
	}
}

func TestIsZero(t *testing.T) {
	tests := []struct {
		value    interface{}
		expected bool
	}{
		{nil, true},
		{0, true},
		{int64(0), true},
		{uint(0), true},
		{float64(0), true},
		{"", true},
		{false, true},
		{1, false},
		{int64(42), false},
		{uint(10), false},
		{float64(3.14), false},
		{"hello", false},
		{true, false},
	}

	for _, test := range tests {
		result := IsZero(test.value)
		if result != test.expected {
			t.Errorf("isZero(%v) = %v, expected %v", test.value, result, test.expected)
		}
	}
}

func TestSave(t *testing.T) {
	mockDB := &MockDB{}
	ctx := context.Background()

	// Test INSERT (id is zero)
	mockDB.ExecFunc = func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
		return &MockResult{lastInsertID: 123}, nil
	}

	data := map[string]interface{}{
		"name":  "John Doe",
		"email": "john@example.com",
	}

	id, err := Save(ctx, mockDB, "users", "id", 0, data)
	if err != nil {
		t.Errorf("Save failed: %v", err)
	}
	if id != 123 {
		t.Errorf("Expected ID 123, got %d", id)
	}

	// Test UPDATE (id is not zero)
	mockDB.ExecFunc = func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
		return &MockResult{rowsAffected: 1}, nil
	}

	affected, err := Save(ctx, mockDB, "users", "id", 123, data)
	if err != nil {
		t.Errorf("Save failed: %v", err)
	}
	if affected != 1 {
		t.Errorf("Expected 1 row affected, got %d", affected)
	}
}

func TestInsertGetID(t *testing.T) {
	mockDB := &MockDB{}
	ctx := context.Background()

	mockDB.ExecFunc = func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
		return &MockResult{lastInsertID: 456}, nil
	}

	id, err := InsertGetID(ctx, mockDB, "INSERT INTO users (name) VALUES (?)", "Test User")
	if err != nil {
		t.Errorf("InsertGetID failed: %v", err)
	}
	if id != 456 {
		t.Errorf("Expected ID 456, got %d", id)
	}
}

func TestUpdate(t *testing.T) {
	mockDB := &MockDB{}
	ctx := context.Background()

	mockDB.ExecFunc = func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
		return &MockResult{rowsAffected: 3}, nil
	}

	affected, err := Update(ctx, mockDB, "UPDATE users SET status = ? WHERE id = ?", "active", 1)
	if err != nil {
		t.Errorf("Update failed: %v", err)
	}
	if affected != 3 {
		t.Errorf("Expected 3 rows affected, got %d", affected)
	}
}

func TestSetFieldByID(t *testing.T) {
	mockDB := &MockDB{}
	ctx := context.Background()

	mockDB.ExecFunc = func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
		return &MockResult{rowsAffected: 1}, nil
	}

	affected, err := SetFieldByID(ctx, mockDB, "users", "id", 1, "status", "active")
	if err != nil {
		t.Errorf("SetFieldByID failed: %v", err)
	}
	if affected != 1 {
		t.Errorf("Expected 1 row affected, got %d", affected)
	}
}
