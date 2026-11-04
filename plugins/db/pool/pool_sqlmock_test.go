package pool

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alldev-run/golang-gin-rpc/plugins/db"
)

// --- sqlmock-backed EnhancedPool tests ---

func newTestEnhancedPool(t *testing.T, mutate func(*PoolConfig)) (*EnhancedPool, sqlmock.Sqlmock, func()) {
	t.Helper()

	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}

	cfg := PoolConfig{
		MaxOpenConns:       5,
		MaxIdleConns:       2,
		ConnMaxLifetime:    time.Hour,
		ConnMaxIdleTime:    time.Hour,
		QueryTimeout:       2 * time.Second,
		HealthCheckPeriod:  time.Hour, // background ticker must not fire during tests
		HealthCheckTimeout: time.Second,
		MetricsInterval:    time.Hour, // ditto for the metrics ticker
		MaxRetries:         1,
		RetryDelay:         time.Millisecond, // keep retry loops fast and deterministic
		EnableMetrics:      false,
	}
	if mutate != nil {
		mutate(&cfg)
	}

	p, err := NewEnhancedPool(db, cfg)
	if err != nil {
		t.Fatalf("NewEnhancedPool error = %v", err)
	}
	return p, mock, func() {
		_ = p.Close()
	}
}

func TestNewEnhancedPoolWithSQLMock(t *testing.T) {
	p, mock, cleanup := newTestEnhancedPool(t, func(c *PoolConfig) { c.EnableMetrics = true })
	defer cleanup()

	if p.GetDB() == nil {
		t.Error("GetDB() should return the injected *sql.DB")
	}
	// isHealthy only flips after the first health check.
	if p.IsHealthy() {
		t.Error("fresh pool should not be healthy before the first health check")
	}

	stats := p.Stats()
	if stats.MaxOpenConnections != 5 {
		t.Errorf("Stats().MaxOpenConnections = %d, want 5 (from applied config)", stats.MaxOpenConnections)
	}

	_ = mock
}

func TestEnhancedPoolQuery(t *testing.T) {
	p, mock, cleanup := newTestEnhancedPool(t, nil)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM users")).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	rows, err := p.Query(context.Background(), "SELECT id FROM users")
	if err != nil {
		t.Fatalf("Query error = %v", err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatal("expected a row")
	}
	var id int
	if err := rows.Scan(&id); err != nil {
		t.Fatalf("Scan error = %v", err)
	}
	if id != 1 {
		t.Errorf("id = %d, want 1", id)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestEnhancedPoolQueryRetriesThenSucceeds(t *testing.T) {
	p, mock, cleanup := newTestEnhancedPool(t, nil)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT 1")).
		WillReturnError(errors.New("transient failure"))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT 1")).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))

	rows, err := p.Query(context.Background(), "SELECT 1")
	if err != nil {
		t.Fatalf("Query should succeed after retry, got error: %v", err)
	}
	_ = rows.Close()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestEnhancedPoolQueryExhaustsRetries(t *testing.T) {
	p, mock, cleanup := newTestEnhancedPool(t, nil)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT 1")).
		WillReturnError(errors.New("failure 1"))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT 1")).
		WillReturnError(errors.New("failure 2"))

	_, err := p.Query(context.Background(), "SELECT 1")
	if err == nil {
		t.Fatal("Query should fail after exhausting retries")
	}
	if want := "query failed after 2 attempts"; !contains(err.Error(), want) {
		t.Errorf("error = %v, want it to contain %q", err, want)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestEnhancedPoolQueryRow(t *testing.T) {
	p, mock, cleanup := newTestEnhancedPool(t, nil)
	defer cleanup()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT name FROM users WHERE id = ?")).
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("alice"))

	var name string
	if err := p.QueryRow(context.Background(), "SELECT name FROM users WHERE id = ?", 1).Scan(&name); err != nil {
		t.Fatalf("QueryRow Scan error = %v", err)
	}
	if name != "alice" {
		t.Errorf("name = %q, want alice", name)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestEnhancedPoolExec(t *testing.T) {
	p, mock, cleanup := newTestEnhancedPool(t, nil)
	defer cleanup()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE users SET name = ? WHERE id = ?")).
		WithArgs("bob", 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	res, err := p.Exec(context.Background(), "UPDATE users SET name = ? WHERE id = ?", "bob", 1)
	if err != nil {
		t.Fatalf("Exec error = %v", err)
	}
	if affected, _ := res.RowsAffected(); affected != 1 {
		t.Errorf("RowsAffected = %d, want 1", affected)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestEnhancedPoolExecRetriesThenFails(t *testing.T) {
	p, mock, cleanup := newTestEnhancedPool(t, nil)
	defer cleanup()

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM users")).
		WillReturnError(errors.New("attempt 1"))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM users")).
		WillReturnError(errors.New("attempt 2"))

	_, err := p.Exec(context.Background(), "DELETE FROM users")
	if err == nil {
		t.Fatal("Exec should fail after exhausting retries")
	}
	if want := "exec failed after 2 attempts"; !contains(err.Error(), want) {
		t.Errorf("error = %v, want it to contain %q", err, want)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestEnhancedPoolBegin(t *testing.T) {
	p, mock, cleanup := newTestEnhancedPool(t, nil)
	defer cleanup()

	mock.ExpectBegin()
	tx, err := p.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin error = %v", err)
	}
	// Begin's deferred context cancel races with our explicit rollback in
	// database/sql; whichever runs first consumes the driver-level rollback.
	mock.ExpectRollback()
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		t.Errorf("Rollback error = %v", err)
	}

	mock.ExpectBegin().WillReturnError(errors.New("too many connections"))
	if _, err := p.Begin(context.Background()); err == nil {
		t.Error("Begin should propagate begin errors")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// NOTE: EnhancedPool.Transaction's happy path cannot be tested deterministically:
// Begin cancels its context on return (defer cancel()), and database/sql then
// rolls the transaction back in the background. The error and panic paths below
// only rely on behavior that is stable under that race.

func TestEnhancedPoolTransactionRollbackOnError(t *testing.T) {
	p, mock, cleanup := newTestEnhancedPool(t, nil)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectRollback()

	wantErr := errors.New("validation failed")
	err := p.Transaction(context.Background(), func(tx *sql.Tx) error { return wantErr })
	if !errors.Is(err, wantErr) {
		t.Errorf("Transaction error = %v, want %v", err, wantErr)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestEnhancedPoolTransactionRollbackOnPanic(t *testing.T) {
	p, mock, cleanup := newTestEnhancedPool(t, nil)
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

	_ = p.Transaction(context.Background(), func(tx *sql.Tx) error {
		panic("boom")
	})
}

func TestEnhancedPoolPing(t *testing.T) {
	p, mock, cleanup := newTestEnhancedPool(t, nil)
	defer cleanup()

	mock.ExpectPing()
	if err := p.Ping(context.Background()); err != nil {
		t.Errorf("Ping error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestEnhancedPoolPerformHealthCheck(t *testing.T) {
	p, mock, cleanup := newTestEnhancedPool(t, nil)
	defer cleanup()

	mock.ExpectPing()
	p.performHealthCheck()
	if !p.IsHealthy() {
		t.Error("pool should be healthy after successful ping")
	}

	mock.ExpectPing().WillReturnError(errors.New("connection refused"))
	p.performHealthCheck()
	if p.IsHealthy() {
		t.Error("pool should be unhealthy after failed ping")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestEnhancedPoolCollectMetrics(t *testing.T) {
	p, _, cleanup := newTestEnhancedPool(t, func(c *PoolConfig) { c.EnableMetrics = true })
	defer cleanup()

	// Must not panic with metrics enabled and must record pool stats.
	p.collectMetrics()
	p.recordQuery("query", "success")
	p.recordQuery("exec", "error")
	p.recordQuery("begin", "failed")
}

func TestPoolManager(t *testing.T) {
	pm := NewPoolManager()

	if err := pm.AddPool("bad", nil, DefaultPoolConfig()); err == nil {
		t.Error("AddPool should reject nil DB")
	}

	db1, mock1, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db1.Close()
	cfg := DefaultPoolConfig()
	cfg.HealthCheckPeriod = time.Hour
	cfg.MetricsInterval = time.Hour
	if err := pm.AddPool("primary", db1, cfg); err != nil {
		t.Fatalf("AddPool error = %v", err)
	}

	if p, ok := pm.GetPool("primary"); !ok || p == nil {
		t.Error("GetPool should return the added pool")
	}
	if _, ok := pm.GetPool("missing"); ok {
		t.Error("GetPool should return false for unknown pool")
	}

	stats := pm.GetStats()
	if len(stats) != 1 {
		t.Errorf("GetStats returned %d entries, want 1", len(stats))
	}
	if _, ok := stats["primary"]; !ok {
		t.Error("GetStats should include the primary pool")
	}

	mock1.ExpectClose()
	if err := pm.Close(); err != nil {
		t.Errorf("Close error = %v", err)
	}
}

// --- PooledClient unit tests ---

func TestPooledClientStatsAndMarkers(t *testing.T) {
	pc := &PooledClient{
		id:        "client-1",
		state:     int32(StateConnected),
		createdAt: time.Now().Unix(),
	}

	if !pc.IsConnected() {
		t.Error("client should start connected")
	}

	pc.markUsed()
	pc.markUsed()
	pc.markFailed()

	stats := pc.GetStats()
	if stats.ID != "client-1" {
		t.Errorf("ID = %q, want client-1", stats.ID)
	}
	if stats.State != StateConnected {
		t.Errorf("State = %v, want StateConnected", stats.State)
	}
	if stats.UseCount != 2 {
		t.Errorf("UseCount = %d, want 2", stats.UseCount)
	}
	if stats.FailCount != 1 {
		t.Errorf("FailCount = %d, want 1", stats.FailCount)
	}
	if stats.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}

	// markUsed resets the failure counter.
	pc.markUsed()
	if got := pc.GetStats().FailCount; got != 0 {
		t.Errorf("FailCount after markUsed = %d, want 0", got)
	}
}

func TestPooledClientConcurrentMarking(t *testing.T) {
	pc := &PooledClient{id: "client-2", state: int32(StateConnected)}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(3)
		go func() { defer wg.Done(); pc.markUsed() }()
		go func() { defer wg.Done(); pc.markFailed() }()
		go func() { defer wg.Done(); _ = pc.GetStats() }()
	}
	wg.Wait()

	stats := pc.GetStats()
	if stats.UseCount != 50 {
		t.Errorf("UseCount = %d, want 50 (atomic increments)", stats.UseCount)
	}
	// markUsed resets the failure counter concurrently with markFailed, so the
	// final FailCount is nondeterministic; just bound it.
	if stats.FailCount < 0 || stats.FailCount > 50 {
		t.Errorf("FailCount = %d, out of expected range [0, 50]", stats.FailCount)
	}
}

// --- Pool (pool.go) tests with a registered fake driver ---

const fakeDriverType = db.Type("testpool-fake-driver")

// fakePoolClient is a controllable db.Client used by the fake driver. Its Ping
// consults the package-level fakePingFailure flag at call time so tests can
// flip health state after the client has been created.
type fakePoolClient struct {
	closed atomic.Int32
}

func (f *fakePoolClient) Ping(ctx context.Context) error {
	if err := fakePingFailure.Load(); err != nil {
		return *err
	}
	return nil
}

func (f *fakePoolClient) Close() error {
	f.closed.Add(1)
	return nil
}

var (
	fakeDriverOnce  sync.Once
	fakePingFailure atomic.Pointer[error]
)

func ensureFakeDriver(t *testing.T) {
	t.Helper()
	fakeDriverOnce.Do(func() {
		db.RegisterDriver(fakeDriverType, func(cfg db.Config) (db.Client, error) {
			return &fakePoolClient{}, nil
		})
	})
}

func setFakePingFailure(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		fakePingFailure.Store(nil)
		return
	}
	fakePingFailure.Store(&err)
}

func newTestPool(t *testing.T, mutate func(*Config)) *Pool {
	t.Helper()
	ensureFakeDriver(t)

	cfg := Config{
		MaxSize:           4,
		InitialSize:       0,
		MaxIdleTime:       time.Hour,
		HealthCheckPeriod: time.Hour, // ticker must not fire during tests
		MaxFailures:       3,
		AcquireTimeout:    time.Second,
		RetryDelay:        50 * time.Millisecond,
	}
	if mutate != nil {
		mutate(&cfg)
	}

	p := New(cfg, db.NewFactory())
	t.Cleanup(func() { _ = p.Close() })
	return p
}

func TestPoolRegisterAcquireAndStats(t *testing.T) {
	p := newTestPool(t, nil)

	fakeCfg := db.Config{Type: fakeDriverType}
	if err := p.Register("primary", fakeCfg); err != nil {
		t.Fatalf("Register error = %v", err)
	}
	if err := p.Register("primary", fakeCfg); err == nil {
		t.Error("Register should reject duplicate names")
	}
	if err := p.Register("bad", db.Config{Type: ""}); err == nil {
		t.Error("Register should reject configs without a type")
	}

	if p.Size() != 1 {
		t.Errorf("Size = %d, want 1", p.Size())
	}
	if names := p.GetConnectionNames(); len(names) != 1 || names[0] != "primary" {
		t.Errorf("GetConnectionNames = %v, want [primary]", names)
	}

	client, err := p.Acquire(context.Background(), "primary")
	if err != nil {
		t.Fatalf("Acquire error = %v", err)
	}
	if client == nil {
		t.Fatal("Acquire returned nil client")
	}
	if err := client.Ping(context.Background()); err != nil {
		t.Errorf("acquired client Ping error = %v", err)
	}

	stats := p.GetStats()
	cs, ok := stats["primary"]
	if !ok {
		t.Fatal("GetStats should include primary")
	}
	if cs.UseCount < 1 {
		t.Errorf("UseCount = %d, want >= 1 after Acquire", cs.UseCount)
	}
	if cs.State != StateConnected {
		t.Errorf("State = %v, want StateConnected", cs.State)
	}

	if err := p.Unregister("missing"); err == nil {
		t.Error("Unregister should fail for unknown name")
	}
	if err := p.Unregister("primary"); err != nil {
		t.Errorf("Unregister error = %v", err)
	}
	if p.Size() != 0 {
		t.Errorf("Size after Unregister = %d, want 0", p.Size())
	}
}

func TestPoolAcquireTimeoutWhenFull(t *testing.T) {
	p := newTestPool(t, func(c *Config) {
		c.MaxSize = 1
		c.AcquireTimeout = 80 * time.Millisecond
	})

	// Saturate the semaphore slot directly so acquireOrCreate hits the
	// timeout branch deterministically.
	p.sem <- struct{}{}

	done := make(chan error, 1)
	go func() {
		_, err := p.Acquire(context.Background(), "any")
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Acquire should time out when all slots are taken")
		}
		if want := "timeout waiting for connection slot"; !contains(err.Error(), want) {
			t.Errorf("error = %v, want it to contain %q", err, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Acquire did not return before the test deadline")
	}
}

func TestPoolCheckHealthHealthyClient(t *testing.T) {
	p := newTestPool(t, nil)

	if err := p.Register("primary", db.Config{Type: fakeDriverType}); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	p.checkHealth()

	stats := p.GetStats()["primary"]
	if stats.State != StateConnected {
		t.Errorf("State = %v, want StateConnected after healthy check", stats.State)
	}
	if stats.FailCount != 0 {
		t.Errorf("FailCount = %d, want 0", stats.FailCount)
	}
}

func TestPoolCheckHealthMarksFailedAndReconnects(t *testing.T) {
	p := newTestPool(t, func(c *Config) {
		c.MaxFailures = 1
		c.RetryDelay = 200 * time.Millisecond
	})

	if err := p.Register("primary", db.Config{Type: fakeDriverType}); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	// Make every client created by the fake driver fail its ping.
	setFakePingFailure(t, errors.New("connection refused"))
	defer setFakePingFailure(t, nil)

	p.checkHealth()

	stats := p.GetStats()["primary"]
	if stats.State != StateFailed {
		t.Errorf("State = %v, want StateFailed after exceeding MaxFailures", stats.State)
	}
	if stats.FailCount < 1 {
		t.Errorf("FailCount = %d, want >= 1", stats.FailCount)
	}

	// The background reconnect goroutine should eventually replace the client
	// (retry delay is 200ms). Poll instead of sleeping to stay deterministic.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if s := p.GetStats()["primary"]; s.State == StateConnected {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Error("reconnect goroutine did not restore StateConnected within the deadline")
}

func TestPoolCloseStopsHealthChecker(t *testing.T) {
	ensureFakeDriver(t)

	cfg := Config{
		MaxSize:           2,
		HealthCheckPeriod: 5 * time.Millisecond, // short so the ticker fires
		AcquireTimeout:    time.Second,
		RetryDelay:        10 * time.Millisecond,
	}
	p := New(cfg, db.NewFactory())

	if err := p.Register("primary", db.Config{Type: fakeDriverType}); err != nil {
		t.Fatalf("Register error = %v", err)
	}

	// Close waits for the health checker goroutine to exit; if it deadlocked
	// this test would hang and fail via the go test timeout.
	if err := p.Close(); err != nil {
		t.Errorf("Close error = %v", err)
	}

	// Double-close panics by design (closed channel); guard against that here.
	// Not invoked — we simply document the contract.
}

// contains reports whether s contains substr.
func contains(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
