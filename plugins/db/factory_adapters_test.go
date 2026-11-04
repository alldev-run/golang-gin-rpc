package db

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/alldev-run/golang-gin-rpc/plugins/cache/memcache"
	"github.com/alldev-run/golang-gin-rpc/plugins/cache/redis"
	"github.com/alldev-run/golang-gin-rpc/plugins/db/mysql"
	"github.com/alldev-run/golang-gin-rpc/plugins/db/orm"
	"github.com/alldev-run/golang-gin-rpc/plugins/db/postgres"
)

// countingFakeClient is a Client whose lifecycle is countable, used to verify
// factory caching and singleflight behaviour with a registered fake driver.
type countingFakeClient struct {
	pings   atomic.Int64
	closes  atomic.Int64
	failPng atomic.Bool
}

func (c *countingFakeClient) Ping(ctx context.Context) error {
	c.pings.Add(1)
	if c.failPng.Load() {
		return errors.New("fake ping failure")
	}
	return nil
}

func (c *countingFakeClient) Close() error {
	c.closes.Add(1)
	return nil
}

const fakeDriverType = Type("unit-test-fake-driver")

// registerFakeDriverOnce registers the fake driver for the whole test binary.
var fakeDriverOnce sync.Once

func registerFakeDriver(t *testing.T) *Factory {
	t.Helper()
	fakeDriverOnce.Do(func() {
		RegisterDriver(fakeDriverType, func(cfg Config) (Client, error) {
			return &countingFakeClient{}, nil
		})
	})
	if !DriverLinked(fakeDriverType) {
		t.Fatal("fake driver should be linked after registration")
	}
	return NewFactory()
}

// ==================== Driver registry ====================

func TestRegisterDriver_DuplicatePanics(t *testing.T) {
	fakeDriverOnce.Do(func() {
		RegisterDriver(fakeDriverType, func(cfg Config) (Client, error) {
			return &countingFakeClient{}, nil
		})
	})

	defer func() {
		if r := recover(); r == nil {
			t.Error("duplicate RegisterDriver should panic")
		}
	}()
	RegisterDriver(fakeDriverType, func(cfg Config) (Client, error) {
		return &countingFakeClient{}, nil
	})
}

func TestDriverLinked(t *testing.T) {
	registerFakeDriver(t)

	if !DriverLinked(TypeMySQL) {
		t.Error("mysql is registered by init and should be linked")
	}
	if !DriverLinked(fakeDriverType) {
		t.Errorf("fake driver %s should be linked", fakeDriverType)
	}
	if DriverLinked(Type("never-registered")) {
		t.Error("unregistered type should not be linked")
	}
}

func TestLinkedDrivers_ContainsBuiltinAndFake(t *testing.T) {
	registerFakeDriver(t)

	linked := LinkedDrivers()
	found := map[Type]bool{}
	for _, typ := range linked {
		found[typ] = true
	}
	for _, typ := range []Type{TypeMySQL, TypeRedis, TypePostgres, TypeMemcache, fakeDriverType} {
		if !found[typ] {
			t.Errorf("LinkedDrivers() missing %s", typ)
		}
	}
}

// ==================== Factory.Create with fake driver ====================

func fakeDriverConfig() Config {
	return Config{Type: fakeDriverType}
}

func TestFactoryCreate_SuccessAndCache(t *testing.T) {
	factory := registerFakeDriver(t)

	client1, err := factory.Create(fakeDriverConfig())
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := client1.Ping(t.Context()); err != nil {
		t.Errorf("Ping() error = %v, want nil", err)
	}

	client2, err := factory.Create(fakeDriverConfig())
	if err != nil {
		t.Fatalf("second Create() error = %v", err)
	}
	if client1 != client2 {
		t.Error("Create() with identical config should return the cached client")
	}

	fake := client1.(*countingFakeClient)
	if got := fake.pings.Load(); got != 1 {
		t.Errorf("pings = %d, want 1", got)
	}

	// GetClient returns the same instance both via the concrete cache key type
	// and the driver type alias stored by Create.
	for _, typ := range []Type{fakeDriverType} {
		cached, err := factory.GetClient(typ)
		if err != nil {
			t.Fatalf("GetClient(%s) error = %v", typ, err)
		}
		if cached != client1 {
			t.Errorf("GetClient(%s) returned a different client", typ)
		}
	}
}

func TestFactoryCreate_UnlinkedDriver(t *testing.T) {
	factory := NewFactory()

	_, err := factory.Create(Config{Type: Type("no-such-driver")})
	if err == nil {
		t.Fatal("Create() with unlinked driver should fail")
	}
	if !strings.Contains(err.Error(), "not linked") {
		t.Errorf("err = %v, want not-linked message", err)
	}
}

func TestFactoryCreate_EmptyType(t *testing.T) {
	factory := NewFactory()

	if _, err := factory.Create(Config{}); err == nil {
		t.Fatal("Create() with empty type should fail")
	}
}

func TestFactoryCreate_CreatorError(t *testing.T) {
	errorType := Type("unit-test-error-driver")
	RegisterDriver(errorType, func(cfg Config) (Client, error) {
		return nil, errors.New("driver init failed")
	})
	factory := NewFactory()

	_, err := factory.Create(Config{Type: errorType})
	if err == nil {
		t.Fatal("Create() with failing creator should fail")
	}
	if !strings.Contains(err.Error(), "driver init failed") {
		t.Errorf("err = %v, want creator error", err)
	}

	// A failed creation must not be cached: retry hits the creator again.
	_, err = factory.Create(Config{Type: errorType})
	if err == nil {
		t.Fatal("retry after creator error should fail again, not return a cached client")
	}
}

func TestFactoryCreate_Singleflight(t *testing.T) {
	var creatorCalls atomic.Int64
	slowType := Type("unit-test-slow-driver")
	RegisterDriver(slowType, func(cfg Config) (Client, error) {
		creatorCalls.Add(1)
		return &countingFakeClient{}, nil
	})
	factory := NewFactory()

	const goroutines = 8
	var wg sync.WaitGroup
	clients := make([]Client, goroutines)
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			client, err := factory.Create(Config{Type: slowType})
			if err != nil {
				t.Errorf("concurrent Create() error = %v", err)
			}
			clients[idx] = client
		}(i)
	}
	wg.Wait()

	if got := creatorCalls.Load(); got != 1 {
		t.Errorf("creator calls = %d, want 1 (singleflight should collapse concurrent creates)", got)
	}
	for _, client := range clients[1:] {
		if client != clients[0] {
			t.Error("all concurrent creates should return the same client")
		}
	}
}

func TestFactoryCreate_PingFailureIsNotFactoryError(t *testing.T) {
	// A fake client that fails Ping still comes back from the factory; the
	// failure surfaces through Ping, not Create.
	factory := registerFakeDriver(t)

	client, err := factory.Create(Config{Type: fakeDriverType, Memcache: memcache.Config{Hosts: []string{"different"}}})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	fake := client.(*countingFakeClient)
	fake.failPng.Store(true)
	if err := client.Ping(t.Context()); err == nil {
		t.Error("Ping() error = nil, want fake failure")
	}
}

// ==================== GetClient ====================

func TestFactoryGetClient_NotFound(t *testing.T) {
	factory := NewFactory()

	_, err := factory.GetClient(Type("absent"))
	if err == nil {
		t.Fatal("GetClient() for unknown type should fail")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("err = %v, want not-found message", err)
	}
}

// ==================== WithConfig variants (offline error paths) ====================

func TestGetMySQLWithConfig_Unreachable(t *testing.T) {
	factory := NewFactory()

	_, err := factory.GetMySQLWithConfig(mysql.Config{
		Host: "127.0.0.1", Port: 1, Database: "d", Username: "u", Password: "p",
	})
	if err == nil {
		t.Fatal("GetMySQLWithConfig() against an unreachable server should fail")
	}
}

func TestGetRedisWithConfig_Unreachable(t *testing.T) {
	factory := NewFactory()

	_, err := factory.GetRedisWithConfig(redis.Config{Host: "127.0.0.1", Port: 1})
	if err == nil {
		t.Fatal("GetRedisWithConfig() against an unreachable server should fail")
	}
}

func TestGetPostgresWithConfig_Unreachable(t *testing.T) {
	factory := NewFactory()

	_, err := factory.GetPostgresWithConfig(postgres.Config{
		Host: "127.0.0.1", Port: 1, Database: "d", Username: "u", Password: "p", SSLMode: "disable",
	})
	if err == nil {
		t.Fatal("GetPostgresWithConfig() against an unreachable server should fail")
	}
}

func TestGetMemcacheWithConfig_OfflineSuccess(t *testing.T) {
	// memcache.New is lazy (no ping), so the WithConfig path succeeds offline
	// and hands back a usable adapter.
	factory := NewFactory()

	client, err := factory.GetMemcacheWithConfig(memcache.Config{Hosts: []string{"127.0.0.1:1"}})
	if err != nil {
		t.Fatalf("GetMemcacheWithConfig() error = %v", err)
	}
	adapter, ok := client.(*memcacheAdapter)
	if !ok {
		t.Fatalf("type = %T, want *memcacheAdapter", client)
	}

	if err := adapter.Ping(t.Context()); err == nil {
		t.Error("memcacheAdapter.Ping() against an unreachable server should fail")
	}
	if err := adapter.Close(); err != nil {
		t.Errorf("memcacheAdapter.Close() error = %v, want nil", err)
	}
}

// ==================== Context database helpers ====================

func TestWithDBContext_RoundTrip(t *testing.T) {
	ctx := WithDBContext(t.Context(), "tenant_a")
	if got := DBFromContext(ctx); got != "tenant_a" {
		t.Errorf("DBFromContext() = %q, want tenant_a", got)
	}
	if got := DBFromContext(t.Context()); got != "" {
		t.Errorf("DBFromContext() on plain ctx = %q, want empty", got)
	}
}

// ==================== Fluent DBSelector ====================

func TestOn_DBSelector_ErrorPaths(t *testing.T) {
	SetGlobalFactory(NewFactory())
	defer SetGlobalFactory(nil)

	selector := On("tenant_db")

	if _, err := selector.Select("users"); err == nil {
		t.Error("On().Select() without a MySQL client should fail")
	}
	if _, err := selector.Insert("users"); err == nil {
		t.Error("On().Insert() without a MySQL client should fail")
	}
	if _, err := selector.Update("users"); err == nil {
		t.Error("On().Update() without a MySQL client should fail")
	}
	if _, err := selector.Delete("users"); err == nil {
		t.Error("On().Delete() without a MySQL client should fail")
	}
}

func TestSelectDB_NamedDatabaseBuilders(t *testing.T) {
	SetGlobalFactory(NewFactory())
	defer SetGlobalFactory(nil)

	// All four database-scoped builder factories fail identically without a
	// client; the branch over the database prefix is exercised for coverage.
	for name, fn := range map[string]func() error{
		"select": func() error { _, err := SelectDB("tenant_db", "users"); return err },
		"insert": func() error { _, err := InsertDB("tenant_db", "users"); return err },
		"update": func() error { _, err := UpdateDB("tenant_db", "users"); return err },
		"delete": func() error { _, err := DeleteDB("tenant_db", "users"); return err },
	} {
		if err := fn(); err == nil {
			t.Errorf("%s without a MySQL client should fail", name)
		}
	}
}

// ==================== Context-aware raw SQL helpers ====================

func TestContextRawSQLHelpers_RequireClient(t *testing.T) {
	SetGlobalFactory(nil)
	if _, err := QueryContext(t.Context(), "SELECT 1"); err == nil {
		t.Error("QueryContext() without factory should fail")
	}
	if QueryRowContext(t.Context(), "SELECT 1") != nil {
		t.Error("QueryRowContext() without factory should return nil")
	}
	if _, err := ExecContext(t.Context(), "SELECT 1"); err == nil {
		t.Error("ExecContext() without factory should fail")
	}

	SetGlobalFactory(NewFactory())
	defer SetGlobalFactory(nil)
	if _, err := QueryContext(t.Context(), "SELECT 1"); err == nil {
		t.Error("QueryContext() without a MySQL client should fail")
	}
	if QueryRowContext(t.Context(), "SELECT 1") != nil {
		t.Error("QueryRowContext() without a MySQL client should return nil")
	}
	if _, err := ExecContext(t.Context(), "SELECT 1"); err == nil {
		t.Error("ExecContext() without a MySQL client should fail")
	}
}

// ==================== InsertOn ====================

func TestInsertOn_ErrorPaths(t *testing.T) {
	SetGlobalFactory(nil)
	if _, err := InsertOn(TypeMySQL, "users"); !errors.Is(err, ErrFactoryNotInitialized) {
		t.Errorf("InsertOn() without factory = %v, want ErrFactoryNotInitialized", err)
	}

	SetGlobalFactory(NewFactory())
	defer SetGlobalFactory(nil)

	if _, err := InsertOn(TypeClickHouse, "users"); err == nil {
		t.Error("InsertOn() with unsupported type should fail")
	}
	if _, err := InsertOn(TypeMySQL, "users"); err == nil {
		t.Error("InsertOn(mysql) without a MySQL client should fail")
	}
	if _, err := InsertOn(TypePostgres, "users"); err == nil {
		t.Error("InsertOn(postgres) without a PostgreSQL client should fail")
	}
}

// ==================== Legacy helpers over orm.DB ====================

func newOrmDBMock(t *testing.T) (orm.DB, sqlmock.Sqlmock, *sql.DB) {
	t.Helper()
	raw, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	return orm.NewDBWrapper(raw), mock, raw
}

func TestLegacyHelpers_InsertGetID(t *testing.T) {
	dbh, mock, _ := newOrmDBMock(t)

	mock.ExpectExec("INSERT INTO logs").WillReturnResult(sqlmock.NewResult(42, 1))

	id, err := InsertGetID(t.Context(), dbh, "INSERT INTO logs (msg) VALUES (?)", "hello")
	if err != nil {
		t.Fatalf("InsertGetID() error = %v", err)
	}
	if id != 42 {
		t.Errorf("id = %d, want 42", id)
	}

	mock.ExpectExec("INSERT INTO logs").WillReturnError(errors.New("table locked"))
	if _, err := InsertGetID(t.Context(), dbh, "INSERT INTO logs (msg) VALUES (?)", "x"); err == nil {
		t.Error("InsertGetID() should propagate exec errors")
	}
}

func TestLegacyHelpers_UpdateDeleteStatement(t *testing.T) {
	dbh, mock, _ := newOrmDBMock(t)

	mock.ExpectExec("UPDATE users").WillReturnResult(sqlmock.NewResult(0, 3))
	n, err := UpdateStatement(t.Context(), dbh, "UPDATE users SET a = ? WHERE id = ?", 1, 2)
	if err != nil {
		t.Fatalf("UpdateStatement() error = %v", err)
	}
	if n != 3 {
		t.Errorf("rows affected = %d, want 3", n)
	}

	mock.ExpectExec("DELETE FROM users").WillReturnResult(sqlmock.NewResult(0, 5))
	n, err = DeleteStatement(t.Context(), dbh, "DELETE FROM users WHERE id = ?", 2)
	if err != nil {
		t.Fatalf("DeleteStatement() error = %v", err)
	}
	if n != 5 {
		t.Errorf("rows affected = %d, want 5", n)
	}

	mock.ExpectExec("UPDATE users").WillReturnError(errors.New("deadlock"))
	if _, err := UpdateStatement(t.Context(), dbh, "UPDATE users SET a = 1"); err == nil {
		t.Error("UpdateStatement() should propagate exec errors")
	}
}

func TestLegacyHelpers_QueryQueryRowExec(t *testing.T) {
	dbh, mock, _ := newOrmDBMock(t)

	mock.ExpectQuery("SELECT id").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	rows, err := Query(t.Context(), dbh, "SELECT id FROM users")
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	rows.Close()

	mock.ExpectQuery("SELECT id").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
	var id int
	if err := QueryRow(t.Context(), dbh, "SELECT id FROM users WHERE id = ?", 7).Scan(&id); err != nil {
		t.Fatalf("QueryRow().Scan() error = %v", err)
	}
	if id != 7 {
		t.Errorf("id = %d, want 7", id)
	}

	mock.ExpectExec("DELETE FROM users").WillReturnResult(sqlmock.NewResult(0, 1))
	if _, err := Exec(t.Context(), dbh, "DELETE FROM users WHERE id = ?", 1); err != nil {
		t.Errorf("Exec() error = %v", err)
	}

	// Ping through the wrapper covers the remaining orm.DB passthrough used
	// by these helpers.
	if err := dbh.Ping(t.Context()); err != nil {
		t.Errorf("Ping() error = %v", err)
	}
}

