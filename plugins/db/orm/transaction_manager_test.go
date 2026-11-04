package orm

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

// fastRetryConfig returns a transaction config with tiny retry delays so retry
// loops finish quickly in tests. IsolationLevel is left at the zero value so
// database/sql issues a plain Begin, which sqlmock matches via ExpectBegin.
func fastRetryConfig() TransactionConfig {
	return TransactionConfig{
		MaxRetries:         2,
		RetryDelay:         time.Millisecond,
		RetryBackoffFactor: 2.0,
		MaxRetryDelay:      5 * time.Millisecond,
		EnableMetrics:      true,
	}
}

// newTestTransactionManager builds a manager with fast retry settings.
func newTestTransactionManager(t *testing.T) *TransactionManager {
	t.Helper()
	return NewTransactionManager(fastRetryConfig())
}

// ==================== Construction & configuration ====================

func TestNewTransactionManager_Defaults(t *testing.T) {
	tm := NewTransactionManager(TransactionConfig{})
	cfg := tm.GetConfig()

	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", cfg.MaxRetries)
	}
	if cfg.RetryDelay != 100*time.Millisecond {
		t.Errorf("RetryDelay = %v, want 100ms", cfg.RetryDelay)
	}
	if cfg.RetryBackoffFactor != 2.0 {
		t.Errorf("RetryBackoffFactor = %v, want 2.0", cfg.RetryBackoffFactor)
	}
	if cfg.MaxRetryDelay != 5*time.Second {
		t.Errorf("MaxRetryDelay = %v, want 5s", cfg.MaxRetryDelay)
	}
	if cfg.SlowQueryThreshold != time.Second {
		t.Errorf("SlowQueryThreshold = %v, want 1s", cfg.SlowQueryThreshold)
	}
	if cfg.MaxOpenConns != 25 || cfg.MaxIdleConns != 5 {
		t.Errorf("pool defaults: MaxOpenConns = %d, MaxIdleConns = %d, want 25/5", cfg.MaxOpenConns, cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime != 30*time.Minute || cfg.ConnMaxIdleTime != 5*time.Minute {
		t.Errorf("conn lifetime defaults: %v / %v, want 30m / 5m", cfg.ConnMaxLifetime, cfg.ConnMaxIdleTime)
	}
	if tm.connectionPool == nil {
		t.Error("connectionPool should be initialized")
	}
	if tm.errorClassifier == nil || tm.retryStrategy == nil || tm.monitor == nil {
		t.Error("errorClassifier, retryStrategy and monitor should be initialized")
	}
	if tm.coordinator != nil {
		t.Error("coordinator should be nil when distributed is disabled")
	}
	if tm.tracer != nil {
		t.Error("tracer should be nil when tracing is disabled")
	}
}

func TestNewDefaultTransactionManager(t *testing.T) {
	tm := NewDefaultTransactionManager()
	cfg := tm.GetConfig()

	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", cfg.MaxRetries)
	}
	if cfg.IsolationLevel != sql.LevelReadCommitted {
		t.Errorf("IsolationLevel = %v, want ReadCommitted", cfg.IsolationLevel)
	}
	if !cfg.EnableMetrics || !cfg.EnableTracing || !cfg.LogSlowQueries {
		t.Errorf("EnableMetrics/EnableTracing/LogSlowQueries = %v/%v/%v, want all true",
			cfg.EnableMetrics, cfg.EnableTracing, cfg.LogSlowQueries)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", cfg.Timeout)
	}
}

func TestNewTransactionManager_DistributedAndTracing(t *testing.T) {
	tm := NewTransactionManager(TransactionConfig{
		EnableDistributed: true,
		CoordinatorURL:    "http://127.0.0.1:1",
		EnableTracing:     true,
	})
	if tm.coordinator == nil {
		t.Error("coordinator should be created when EnableDistributed is set")
	}
	if tm.tracer == nil {
		t.Error("tracer should be created when EnableTracing is set")
	}
	if _, ok := tm.coordinator.(*HTTPCoordinator); !ok {
		t.Errorf("coordinator type = %T, want *HTTPCoordinator", tm.coordinator)
	}
}

func TestTransactionManager_SetConfig_GetConfig(t *testing.T) {
	tm := newTestTransactionManager(t)
	cfg := fastRetryConfig()
	cfg.MaxRetries = 7
	tm.SetConfig(cfg)

	if got := tm.GetConfig().MaxRetries; got != 7 {
		t.Errorf("MaxRetries after SetConfig = %d, want 7", got)
	}
}

func TestTransactionManager_GetActiveTransactionCount(t *testing.T) {
	tm := newTestTransactionManager(t)
	if got := tm.GetActiveTransactionCount(); got != 0 {
		t.Fatalf("initial active count = %d, want 0", got)
	}

	db, mock := sqlmockNewDB(t)
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectCommit()

	_, err := tm.WithTransaction(context.Background(), NewDBWrapper(db), func(tx *ORM) error {
		if got := tm.GetActiveTransactionCount(); got < 1 {
			t.Errorf("active count inside transaction = %d, want >= 1", got)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WithTransaction error: %v", err)
	}
	if got := tm.GetActiveTransactionCount(); got != 0 {
		t.Errorf("active count after transaction = %d, want 0", got)
	}
}

func TestTransactionManager_GenerateTransactionID(t *testing.T) {
	tm := newTestTransactionManager(t)
	id1 := tm.generateTransactionID()
	id2 := tm.generateTransactionID()

	if !strings.HasPrefix(id1, "tx_") {
		t.Errorf("id1 = %q, want tx_ prefix", id1)
	}
	if id1 == id2 {
		t.Errorf("transaction IDs should be unique, got %q twice", id1)
	}
}

// ==================== WithTransaction ====================

func TestWithTransaction_Commit(t *testing.T) {
	tm := newTestTransactionManager(t)
	db, mock := sqlmockNewDB(t)
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectCommit()

	result, err := tm.WithTransaction(context.Background(), NewDBWrapper(db), func(tx *ORM) error {
		return nil
	})
	if err != nil {
		t.Fatalf("WithTransaction error: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if !result.Success {
		t.Error("result.Success = false, want true")
	}
	if result.Retries != 0 {
		t.Errorf("result.Retries = %d, want 0", result.Retries)
	}
	if !strings.HasPrefix(result.TransactionID, "tx_") {
		t.Errorf("TransactionID = %q, want tx_ prefix", result.TransactionID)
	}
	if len(result.RetryAttempts) != 1 {
		t.Errorf("len(RetryAttempts) = %d, want 1", len(result.RetryAttempts))
	}
	// Duration may legitimately measure as 0 on coarse Windows clocks for
	// instant sqlmock transactions; only sanity-check the sign.
	if result.Duration < 0 {
		t.Errorf("result.Duration = %v, want >= 0", result.Duration)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWithTransaction_NonRetryableError(t *testing.T) {
	tm := newTestTransactionManager(t)
	db, mock := sqlmockNewDB(t)
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectRollback()

	origErr := errors.New("unique constraint violated")
	result, err := tm.WithTransaction(context.Background(), NewDBWrapper(db), func(tx *ORM) error {
		return origErr
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, origErr) {
		t.Errorf("err = %v, want wrapped original error", err)
	}
	if result.Success {
		t.Error("result.Success = true, want false")
	}
	if len(result.RetryAttempts) != 1 {
		t.Errorf("len(RetryAttempts) = %d, want 1", len(result.RetryAttempts))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWithTransaction_RetryThenSuccess(t *testing.T) {
	tm := newTestTransactionManager(t)
	db, mock := sqlmockNewDB(t)
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectRollback()
	mock.ExpectBegin()
	mock.ExpectCommit()

	attempts := 0
	result, err := tm.WithTransaction(context.Background(), NewDBWrapper(db), func(tx *ORM) error {
		attempts++
		if attempts == 1 {
			return errors.New("connection refused")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WithTransaction error: %v", err)
	}
	if attempts != 2 {
		t.Errorf("fn attempts = %d, want 2", attempts)
	}
	if result.Retries != 1 {
		t.Errorf("result.Retries = %d, want 1", result.Retries)
	}
	if len(result.RetryAttempts) != 2 {
		t.Errorf("len(RetryAttempts) = %d, want 2", len(result.RetryAttempts))
	}
	if result.RetryAttempts[0].Error == nil || result.RetryAttempts[1].Error != nil {
		t.Errorf("retry attempts errors = [%v, %v], want [err, nil]",
			result.RetryAttempts[0].Error, result.RetryAttempts[1].Error)
	}

	metrics := tm.GetMetrics()
	if metrics.RetriedTransactions != 1 {
		t.Errorf("metrics.RetriedTransactions = %d, want 1", metrics.RetriedTransactions)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWithTransaction_RetryExhausted(t *testing.T) {
	tm := newTestTransactionManager(t)
	db, mock := sqlmockNewDB(t)
	defer db.Close()
	// MaxRetries=2 -> three attempts, each Begin + (deferred) Rollback.
	mock.ExpectBegin()
	mock.ExpectRollback()
	mock.ExpectBegin()
	mock.ExpectRollback()
	mock.ExpectBegin()
	mock.ExpectRollback()

	result, err := tm.WithTransaction(context.Background(), NewDBWrapper(db), func(tx *ORM) error {
		return errors.New("connection reset by peer")
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// On the final attempt ShouldRetry returns false, so the raw fn error is
	// returned in-loop (the "failed after N attempts" wrap is reserved for
	// custom retry strategies that allow a retry beyond the limit).
	if !errors.Is(err, result.Error) || !strings.Contains(err.Error(), "connection reset by peer") {
		t.Errorf("err = %v, want raw retryable error", err)
	}
	if result.Success {
		t.Error("result.Success = true, want false")
	}
	if result.Retries != 2 {
		t.Errorf("result.Retries = %d, want 2", result.Retries)
	}
	if len(result.RetryAttempts) != 3 {
		t.Errorf("len(RetryAttempts) = %d, want 3", len(result.RetryAttempts))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWithTransaction_ContextTimeoutDuringRetry(t *testing.T) {
	// Built directly (not via SetConfig) so the retry strategy sees the same
	// config: the first retry delay (~100ms, uncapped) exceeds the 30ms
	// transaction timeout, so the retry wait aborts on ctx.Done().
	cfg := fastRetryConfig()
	cfg.MaxRetryDelay = time.Second
	cfg.Timeout = 30 * time.Millisecond
	tm := NewTransactionManager(cfg)

	db, mock := sqlmockNewDB(t)
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectRollback()

	result, err := tm.WithTransaction(context.Background(), NewDBWrapper(db), func(tx *ORM) error {
		return errors.New("connection refused")
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want context.DeadlineExceeded", err)
	}
	if result.Success {
		t.Error("result.Success = true, want false")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWithTransaction_WithTracing(t *testing.T) {
	cfg := fastRetryConfig()
	cfg.EnableTracing = true
	tm := NewTransactionManager(cfg)

	db, mock := sqlmockNewDB(t)
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectCommit()

	result, err := tm.WithTransaction(context.Background(), NewDBWrapper(db), func(tx *ORM) error {
		return nil
	})
	if err != nil {
		t.Fatalf("WithTransaction error: %v", err)
	}
	if !result.Success {
		t.Error("result.Success = false, want true")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWithTransaction_BeginTxError(t *testing.T) {
	tm := newTestTransactionManager(t)
	db, mock := sqlmockNewDB(t)
	defer db.Close()
	mock.ExpectBegin().WillReturnError(errors.New("boom"))

	result, err := tm.WithTransaction(context.Background(), NewDBWrapper(db), func(tx *ORM) error {
		t.Error("fn should not run when Begin fails")
		return nil
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("err = %v, want wrapped begin error", err)
	}
	if result.Success {
		t.Error("result.Success = true, want false")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWithTransaction_MetricsDisabled(t *testing.T) {
	cfg := fastRetryConfig()
	cfg.EnableMetrics = false
	tm := NewTransactionManager(cfg)

	db, mock := sqlmockNewDB(t)
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectCommit()

	if _, err := tm.WithTransaction(context.Background(), NewDBWrapper(db), func(tx *ORM) error {
		return nil
	}); err != nil {
		t.Fatalf("WithTransaction error: %v", err)
	}

	metrics := tm.GetMetrics()
	if metrics.TotalTransactions != 0 {
		t.Errorf("metrics.TotalTransactions = %d, want 0 when metrics disabled", metrics.TotalTransactions)
	}
}

// ==================== Distributed coordinator paths ====================

// fakeCoordinatorServer is an httptest server speaking the HTTPCoordinator
// protocol with per-endpoint failure switches and call counters.
type fakeCoordinatorServer struct {
	server        *httptest.Server
	beginOK       atomic.Bool
	commitOK      atomic.Bool
	rollbackOK    atomic.Bool
	beginCalls    atomic.Int64
	commitCalls   atomic.Int64
	rollbackCalls atomic.Int64
}

func newFakeCoordinatorServer(t *testing.T) *fakeCoordinatorServer {
	t.Helper()
	fake := &fakeCoordinatorServer{}
	fake.beginOK.Store(true)
	fake.commitOK.Store(true)
	fake.rollbackOK.Store(true)

	mux := http.NewServeMux()
	mux.HandleFunc("/transactions", func(w http.ResponseWriter, r *http.Request) {
		fake.beginCalls.Add(1)
		if fake.beginOK.Load() {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	})
	mux.HandleFunc("/transactions/commit", func(w http.ResponseWriter, r *http.Request) {
		fake.commitCalls.Add(1)
		if fake.commitOK.Load() {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	})
	mux.HandleFunc("/transactions/rollback", func(w http.ResponseWriter, r *http.Request) {
		fake.rollbackCalls.Add(1)
		if fake.rollbackOK.Load() {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	})
	// HTTPCoordinator posts to /transactions/<txID>/commit|rollback; the
	// generic trailing-segment routes land here.
	mux.HandleFunc("/transactions/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/commit"):
			fake.commitCalls.Add(1)
			if fake.commitOK.Load() {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
		case strings.HasSuffix(r.URL.Path, "/rollback"):
			fake.rollbackCalls.Add(1)
			if fake.rollbackOK.Load() {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	fake.server = httptest.NewServer(mux)
	t.Cleanup(fake.server.Close)
	return fake
}

func distributedTMConfig(url string) TransactionConfig {
	cfg := fastRetryConfig()
	cfg.EnableDistributed = true
	cfg.CoordinatorURL = url
	return cfg
}

func TestWithTransaction_DistributedBeginFailure(t *testing.T) {
	fake := newFakeCoordinatorServer(t)
	fake.beginOK.Store(false)
	tm := NewTransactionManager(distributedTMConfig(fake.server.URL))

	db, _ := sqlmockNewDB(t)
	defer db.Close()

	result, err := tm.WithTransaction(context.Background(), NewDBWrapper(db), func(tx *ORM) error {
		t.Error("fn should not run when the distributed begin fails")
		return nil
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// The returned error is the raw coordinator failure; the wrapped variant
	// ("failed to begin distributed transaction") is carried on result.Error.
	if !strings.Contains(result.Error.Error(), "failed to begin distributed transaction") {
		t.Errorf("result.Error = %v, want distributed begin failure", result.Error)
	}
	if result.Success {
		t.Error("result.Success = true, want false")
	}
	if got := fake.beginCalls.Load(); got != 1 {
		t.Errorf("coordinator begin calls = %d, want 1", got)
	}
}

func TestWithTransaction_DistributedCommitFailure(t *testing.T) {
	fake := newFakeCoordinatorServer(t)
	fake.commitOK.Store(false)
	tm := NewTransactionManager(distributedTMConfig(fake.server.URL))

	db, mock := sqlmockNewDB(t)
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectCommit()

	result, err := tm.WithTransaction(context.Background(), NewDBWrapper(db), func(tx *ORM) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Raw coordinator failure is returned; wrapped variant lives on result.Error.
	if !strings.Contains(result.Error.Error(), "failed to commit distributed transaction") {
		t.Errorf("result.Error = %v, want distributed commit failure", result.Error)
	}
	if result.Success {
		t.Error("result.Success = true, want false after commit failure")
	}
	// The commit-failure path notifies the monitor, not the coordinator
	// rollback endpoint.
	if got := fake.rollbackCalls.Load(); got != 0 {
		t.Errorf("coordinator rollback calls = %d, want 0", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWithTransaction_DistributedRollbackOnError(t *testing.T) {
	fake := newFakeCoordinatorServer(t)
	tm := NewTransactionManager(distributedTMConfig(fake.server.URL))

	db, mock := sqlmockNewDB(t)
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectRollback()

	origErr := errors.New("duplicate key")
	result, err := tm.WithTransaction(context.Background(), NewDBWrapper(db), func(tx *ORM) error {
		return origErr
	})
	if !errors.Is(err, origErr) {
		t.Errorf("err = %v, want wrapped original error", err)
	}
	if got := fake.beginCalls.Load(); got != 1 {
		t.Errorf("coordinator begin calls = %d, want 1", got)
	}
	if got := fake.rollbackCalls.Load(); got != 1 {
		t.Errorf("coordinator rollback calls = %d, want 1", got)
	}
	if got := fake.commitCalls.Load(); got != 0 {
		t.Errorf("coordinator commit calls = %d, want 0", got)
	}
	if result.Success {
		t.Error("result.Success = true, want false")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWithTransaction_DistributedSuccess(t *testing.T) {
	fake := newFakeCoordinatorServer(t)
	tm := NewTransactionManager(distributedTMConfig(fake.server.URL))

	db, mock := sqlmockNewDB(t)
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectCommit()

	result, err := tm.WithTransaction(context.Background(), NewDBWrapper(db), func(tx *ORM) error {
		return nil
	})
	if err != nil {
		t.Fatalf("WithTransaction error: %v", err)
	}
	if !result.Success {
		t.Error("result.Success = false, want true")
	}
	if got := fake.commitCalls.Load(); got != 1 {
		t.Errorf("coordinator commit calls = %d, want 1", got)
	}
	if got := fake.rollbackCalls.Load(); got != 0 {
		t.Errorf("coordinator rollback calls = %d, want 0", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// ==================== WithRetry ====================

func TestWithRetry_Success(t *testing.T) {
	tm := newTestTransactionManager(t)

	calls := 0
	err := tm.WithRetry(context.Background(), func() error {
		calls++
		if calls < 3 {
			return errors.New("connection refused")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WithRetry error: %v", err)
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}

func TestWithRetry_NonRetryableError(t *testing.T) {
	tm := newTestTransactionManager(t)

	calls := 0
	origErr := errors.New("permission denied")
	err := tm.WithRetry(context.Background(), func() error {
		calls++
		return origErr
	})
	if !errors.Is(err, origErr) {
		t.Errorf("err = %v, want original error", err)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (non-retryable errors return immediately)", calls)
	}
}

func TestWithRetry_Exhausted(t *testing.T) {
	tm := newTestTransactionManager(t)

	calls := 0
	err := tm.WithRetry(context.Background(), func() error {
		calls++
		return errors.New("connection reset")
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// With the default strategy the last attempt returns the raw fn error
	// once ShouldRetry reports the attempt budget is spent.
	if !strings.Contains(err.Error(), "connection reset") {
		t.Errorf("err = %v, want raw retryable error", err)
	}
	if calls != 3 {
		t.Errorf("calls = %d, want MaxRetries+1 = 3", calls)
	}
}

func TestWithRetry_ContextCancelled(t *testing.T) {
	tm := newTestTransactionManager(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := tm.WithRetry(ctx, func() error {
		return errors.New("connection refused")
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

// ==================== WithNestedTransaction ====================

func newNestedTxORM(t *testing.T) (*ORM, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock := sqlmockNewDB(t)
	orm := NewORM(NewDBWrapper(db), nil)
	return orm, mock, func() { _ = db.Close() }
}

func TestWithNestedTransaction_Success(t *testing.T) {
	tm := newTestTransactionManager(t)
	orm, mock, closeDB := newNestedTxORM(t)
	defer closeDB()

	mock.ExpectExec("SAVEPOINT sp_order").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("RELEASE SAVEPOINT sp_order").WillReturnResult(sqlmock.NewResult(0, 0))

	err := tm.WithNestedTransaction(context.Background(), orm, "sp_order", func(tx *ORM) error {
		return nil
	})
	if err != nil {
		t.Fatalf("WithNestedTransaction error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWithNestedTransaction_DefaultSavepointName(t *testing.T) {
	tm := newTestTransactionManager(t)
	orm, mock, closeDB := newNestedTxORM(t)
	defer closeDB()

	mock.ExpectExec("SAVEPOINT sp_").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("RELEASE SAVEPOINT sp_").WillReturnResult(sqlmock.NewResult(0, 0))

	if err := tm.WithNestedTransaction(context.Background(), orm, "", func(tx *ORM) error {
		return nil
	}); err != nil {
		t.Fatalf("WithNestedTransaction error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWithNestedTransaction_RollbackToSavepoint(t *testing.T) {
	tm := newTestTransactionManager(t)
	orm, mock, closeDB := newNestedTxORM(t)
	defer closeDB()

	mock.ExpectExec("SAVEPOINT sp_order").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ROLLBACK TO SAVEPOINT sp_order").WillReturnResult(sqlmock.NewResult(0, 0))

	origErr := errors.New("inner failure")
	err := tm.WithNestedTransaction(context.Background(), orm, "sp_order", func(tx *ORM) error {
		return origErr
	})
	if !errors.Is(err, origErr) {
		t.Errorf("err = %v, want original inner error", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWithNestedTransaction_CreateSavepointFailure(t *testing.T) {
	tm := newTestTransactionManager(t)
	orm, mock, closeDB := newNestedTxORM(t)
	defer closeDB()

	mock.ExpectExec("SAVEPOINT sp_order").WillReturnError(errors.New("savepoint denied"))

	err := tm.WithNestedTransaction(context.Background(), orm, "sp_order", func(tx *ORM) error {
		t.Error("fn should not run when savepoint creation fails")
		return nil
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to create savepoint sp_order") {
		t.Errorf("err = %v, want create-savepoint failure", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWithNestedTransaction_RollbackToSavepointFailure(t *testing.T) {
	tm := newTestTransactionManager(t)
	orm, mock, closeDB := newNestedTxORM(t)
	defer closeDB()

	mock.ExpectExec("SAVEPOINT sp_order").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ROLLBACK TO SAVEPOINT sp_order").WillReturnError(errors.New("rollback failed"))

	err := tm.WithNestedTransaction(context.Background(), orm, "sp_order", func(tx *ORM) error {
		return errors.New("inner failure")
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to rollback to savepoint sp_order") ||
		!strings.Contains(err.Error(), "inner failure") {
		t.Errorf("err = %v, want rollback failure joined with original error", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWithNestedTransaction_ReleaseFailure(t *testing.T) {
	tm := newTestTransactionManager(t)
	orm, mock, closeDB := newNestedTxORM(t)
	defer closeDB()

	mock.ExpectExec("SAVEPOINT sp_order").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("RELEASE SAVEPOINT sp_order").WillReturnError(errors.New("release failed"))

	err := tm.WithNestedTransaction(context.Background(), orm, "sp_order", func(tx *ORM) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to release savepoint sp_order") {
		t.Errorf("err = %v, want release-savepoint failure", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// ==================== Metrics ====================

func TestTransactionManager_MetricsLifecycle(t *testing.T) {
	tm := newTestTransactionManager(t)

	db, mock := sqlmockNewDB(t)
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectCommit()

	if _, err := tm.WithTransaction(context.Background(), NewDBWrapper(db), func(tx *ORM) error {
		return nil
	}); err != nil {
		t.Fatalf("WithTransaction error: %v", err)
	}

	metrics := tm.GetMetrics()
	if metrics.TotalTransactions != 1 {
		t.Errorf("TotalTransactions = %d, want 1", metrics.TotalTransactions)
	}
	if metrics.SuccessfulTransactions != 1 {
		t.Errorf("SuccessfulTransactions = %d, want 1", metrics.SuccessfulTransactions)
	}
	if metrics.FailedTransactions != 0 {
		t.Errorf("FailedTransactions = %d, want 0", metrics.FailedTransactions)
	}
	// sqlmock transactions are near-instant and Windows coarse clocks can
	// measure 0ns, so only assert the duration invariants.
	if metrics.MaxDuration < metrics.MinDuration {
		t.Errorf("MaxDuration %v < MinDuration %v", metrics.MaxDuration, metrics.MinDuration)
	}
	if metrics.AvgDuration > metrics.MaxDuration {
		t.Errorf("AvgDuration %v > MaxDuration %v", metrics.AvgDuration, metrics.MaxDuration)
	}

	resetAt := time.Now()
	tm.ResetMetrics()
	metrics = tm.GetMetrics()
	if metrics.TotalTransactions != 0 || metrics.SuccessfulTransactions != 0 || metrics.FailedTransactions != 0 {
		t.Error("metrics should be zero after ResetMetrics")
	}
	if metrics.LastReset.Before(resetAt.Add(-time.Second)) {
		t.Errorf("LastReset = %v, want ~now", metrics.LastReset)
	}
}

func TestTransactionManager_MetricsAcrossRuns(t *testing.T) {
	tm := newTestTransactionManager(t)

	db, mock := sqlmockNewDB(t)
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectRollback()

	if _, err := tm.WithTransaction(context.Background(), NewDBWrapper(db), func(tx *ORM) error {
		return nil
	}); err != nil {
		t.Fatalf("first WithTransaction error: %v", err)
	}
	if _, err := tm.WithTransaction(context.Background(), NewDBWrapper(db), func(tx *ORM) error {
		return errors.New("check constraint violated")
	}); err == nil {
		t.Fatal("expected error from second WithTransaction")
	}

	metrics := tm.GetMetrics()
	if metrics.TotalTransactions != 2 {
		t.Errorf("TotalTransactions = %d, want 2", metrics.TotalTransactions)
	}
	if metrics.SuccessfulTransactions != 1 || metrics.FailedTransactions != 1 {
		t.Errorf("Successful/Failed = %d/%d, want 1/1", metrics.SuccessfulTransactions, metrics.FailedTransactions)
	}
}

// ==================== Lifecycle ====================

func TestTransactionManager_Close(t *testing.T) {
	tm := newTestTransactionManager(t)
	if err := tm.Close(); err != nil {
		t.Errorf("Close error = %v, want nil", err)
	}
}

func TestTransactionManager_CloseWithInitializedPool(t *testing.T) {
	db, _ := sqlmockNewDB(t)

	tm := NewTransactionManager(fastRetryConfig())
	if err := tm.connectionPool.Initialize(db); err != nil {
		t.Fatalf("Initialize error: %v", err)
	}
	if err := tm.Close(); err != nil {
		t.Errorf("Close error = %v, want nil", err)
	}
}
