package orm

import (
	"database/sql"
	"errors"
	"testing"
	"time"
)

// ==================== DefaultErrorClassifier ====================

func TestDefaultErrorClassifier_ClassifyError(t *testing.T) {
	classifier := NewDefaultErrorClassifier()

	tests := []struct {
		name       string
		err        error
		wantType   ErrorType
		wantRetry  bool
		wantTrans  bool
		wantSev    ErrorSeverity
		wantCatego string
	}{
		{
			name:       "nil error",
			err:        nil,
			wantType:   ErrorTypeUnknown,
			wantRetry:  false,
			wantTrans:  false,
			wantSev:    SeverityLow,
			wantCatego: "",
		},
		{
			name:       "deadlock beats timeout pattern",
			err:        errors.New("lock wait timeout exceeded"),
			wantType:   ErrorTypeDeadlock,
			wantRetry:  true,
			wantTrans:  true,
			wantSev:    SeverityHigh,
			wantCatego: "concurrency",
		},
		{
			name:       "serialization failure is deadlock",
			err:        errors.New("could not serialize access due to concurrent update"),
			wantType:   ErrorTypeDeadlock,
			wantRetry:  true,
			wantTrans:  true,
			wantSev:    SeverityHigh,
			wantCatego: "concurrency",
		},
		{
			name:       "deadline exceeded is timeout",
			err:        errors.New("context deadline exceeded"),
			wantType:   ErrorTypeTimeout,
			wantRetry:  true,
			wantTrans:  true,
			wantSev:    SeverityMedium,
			wantCatego: "performance",
		},
		{
			name:       "connection refused is connection",
			err:        errors.New("dial tcp: connection refused"),
			wantType:   ErrorTypeConnection,
			wantRetry:  true,
			wantTrans:  true,
			wantSev:    SeverityHigh,
			wantCatego: "infrastructure",
		},
		{
			name:       "too many connections is connection",
			err:        errors.New("pq: too many connections"),
			wantType:   ErrorTypeConnection,
			wantRetry:  true,
			wantTrans:  true,
			wantSev:    SeverityHigh,
			wantCatego: "infrastructure",
		},
		{
			name:       "duplicate key is constraint",
			err:        errors.New("duplicate key value violates unique constraint"),
			wantType:   ErrorTypeConstraint,
			wantRetry:  false,
			wantTrans:  false,
			wantSev:    SeverityMedium,
			wantCatego: "data_integrity",
		},
		{
			name:       "foreign key constraint",
			err:        errors.New("foreign key constraint failed"),
			wantType:   ErrorTypeConstraint,
			wantRetry:  false,
			wantTrans:  false,
			wantSev:    SeverityMedium,
			wantCatego: "data_integrity",
		},
		{
			name:       "permission denied",
			err:        errors.New("permission denied for table users"),
			wantType:   ErrorTypePermission,
			wantRetry:  false,
			wantTrans:  false,
			wantSev:    SeverityCritical,
			wantCatego: "security",
		},
		{
			name:       "resource exhaustion",
			err:        errors.New("disk full while writing"),
			wantType:   ErrorTypeResource,
			wantRetry:  false,
			wantTrans:  false,
			wantSev:    SeverityCritical,
			wantCatego: "infrastructure",
		},
		{
			name:       "unknown business error is logic",
			err:        errors.New("order not found"),
			wantType:   ErrorTypeLogic,
			wantRetry:  false,
			wantTrans:  false,
			wantSev:    SeverityLow,
			wantCatego: "application",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifier.ClassifyError(tt.err)
			if got.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", got.Type, tt.wantType)
			}
			if got.Retryable != tt.wantRetry {
				t.Errorf("Retryable = %v, want %v", got.Retryable, tt.wantRetry)
			}
			if got.Transient != tt.wantTrans {
				t.Errorf("Transient = %v, want %v", got.Transient, tt.wantTrans)
			}
			if got.Severity != tt.wantSev {
				t.Errorf("Severity = %q, want %q", got.Severity, tt.wantSev)
			}
			if got.Category != tt.wantCatego {
				t.Errorf("Category = %q, want %q", got.Category, tt.wantCatego)
			}
		})
	}
}

func TestDefaultErrorClassifier_IsRetryable_IsTransient(t *testing.T) {
	classifier := NewDefaultErrorClassifier()

	tests := []struct {
		name          string
		err           error
		wantRetry     bool
		wantTransient bool
	}{
		{"deadlock", errors.New("deadlock detected"), true, true},
		{"read timeout", errors.New("read timeout after 5s"), true, true},
		{"connection lost", errors.New("connection lost mid-query"), true, true},
		{"temporary failure", errors.New("temporary failure resolving"), true, true},
		{"network glitch", errors.New("network error occurred"), true, true},
		{"constraint", errors.New("not null constraint violation"), false, false},
		{"access denied", errors.New("access denied for user"), false, false},
		{"nil", nil, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifier.IsRetryable(tt.err); got != tt.wantRetry {
				t.Errorf("IsRetryable = %v, want %v", got, tt.wantRetry)
			}
			if got := classifier.IsTransient(tt.err); got != tt.wantTransient {
				t.Errorf("IsTransient = %v, want %v", got, tt.wantTransient)
			}
		})
	}
}

func TestDefaultErrorClassifier_GetRetryDelay(t *testing.T) {
	classifier := NewDefaultErrorClassifier()

	tests := []struct {
		name      string
		attempt   int
		err       error
		wantDelay time.Duration
	}{
		{"deadlock attempt 2", 2, errors.New("deadlock found"), 400 * time.Millisecond}, // 2^2 * 100ms
		{"timeout attempt 1", 1, errors.New("query timeout"), 400 * time.Millisecond},   // 2^1 * 200ms
		// time.Duration(math.Pow(1.5, 2)) truncates 2.25 -> 2 before scaling.
		{"connection attempt 2", 2, errors.New("connection refused"), 200 * time.Millisecond},
		{"default attempt 3", 3, errors.New("business error"), 400 * time.Millisecond}, // (3+1) * 100ms
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifier.GetRetryDelay(tt.attempt, tt.err)
			if got != tt.wantDelay {
				t.Errorf("GetRetryDelay(%d, %v) = %v, want %v", tt.attempt, tt.err, got, tt.wantDelay)
			}
		})
	}
}

// ==================== ExponentialBackoffStrategy ====================

func TestExponentialBackoffStrategy_ShouldRetry(t *testing.T) {
	strategy := NewExponentialBackoffStrategy(TransactionConfig{MaxRetries: 2})

	tests := []struct {
		name    string
		attempt int
		err     error
		want    bool
	}{
		{"retryable within budget", 0, errors.New("connection refused"), true},
		{"retryable at limit", 2, errors.New("connection refused"), false},
		{"beyond limit", 3, errors.New("connection refused"), false},
		{"non-retryable", 0, errors.New("permission denied"), false},
		{"nil error", 0, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := strategy.ShouldRetry(tt.attempt, tt.err); got != tt.want {
				t.Errorf("ShouldRetry(%d, %v) = %v, want %v", tt.attempt, tt.err, got, tt.want)
			}
		})
	}
}

func TestExponentialBackoffStrategy_GetDelay(t *testing.T) {
	strategy := NewExponentialBackoffStrategy(TransactionConfig{
		MaxRetries:         3,
		RetryBackoffFactor: 2.0,
		MaxRetryDelay:      5 * time.Second,
	})

	// Attempt 0 with a connection error: base delay 100ms * 2^0, ±10% jitter.
	delay := strategy.GetDelay(0, errors.New("connection refused"))
	if delay < 80*time.Millisecond || delay > 120*time.Millisecond {
		t.Errorf("GetDelay(0) = %v, want ~100ms (±jitter)", delay)
	}

	// Attempt 3: base 1.5^3*100ms = 337.5ms, * 2^3 = 2.7s, ±jitter keeps it well under the cap.
	delay = strategy.GetDelay(3, errors.New("connection refused"))
	if delay <= 2*time.Second || delay >= 3*time.Second {
		t.Errorf("GetDelay(3) = %v, want ~2.7s (±jitter)", delay)
	}
}

func TestExponentialBackoffStrategy_GetDelayCapped(t *testing.T) {
	const maxDelay = 500 * time.Millisecond
	strategy := NewExponentialBackoffStrategy(TransactionConfig{
		MaxRetries:         3,
		RetryBackoffFactor: 2.0,
		MaxRetryDelay:      maxDelay,
	})

	// Deadlock at attempt 3: base 2^3*100ms = 800ms, * 2^3 = 6.4s → capped at 500ms.
	delay := strategy.GetDelay(3, errors.New("deadlock detected"))
	if delay != maxDelay {
		t.Errorf("GetDelay(3) = %v, want capped at %v", delay, maxDelay)
	}
}

func TestExponentialBackoffStrategy_GetMaxAttempts(t *testing.T) {
	strategy := NewExponentialBackoffStrategy(TransactionConfig{MaxRetries: 7})
	if got := strategy.GetMaxAttempts(); got != 7 {
		t.Errorf("GetMaxAttempts = %d, want 7", got)
	}
}

// ==================== DefaultTransactionMonitor ====================

func TestDefaultTransactionMonitor_CallbacksDoNotPanic(t *testing.T) {
	monitor := NewDefaultTransactionMonitor(TransactionConfig{
		EnableMetrics:      true,
		LogSlowQueries:     true,
		SlowQueryThreshold: time.Second,
	})

	result := &TransactionResult{
		Success:       false,
		Duration:      10 * time.Millisecond,
		Retries:       1,
		Error:         errors.New("connection refused"),
		TransactionID: "tx_test",
	}

	monitor.OnTransactionStart("tx_test", TransactionConfig{IsolationLevel: sql.LevelReadCommitted})
	monitor.OnTransactionCommit("tx_test", &TransactionResult{Duration: time.Millisecond})
	monitor.OnTransactionRollback("tx_test", result)
	monitor.OnTransactionRetry("tx_test", 2, errors.New("deadlock"))
	monitor.OnSlowQuery("tx_test", "SELECT 1", 2*time.Second)
}

func TestDefaultTransactionMonitor_NilErrorPayload(t *testing.T) {
	// A result with a nil error must not crash the monitor; enterprise code
	// paths (distributed rollback notifications) may pass success-shaped
	// results into rollback notifications.
	monitor := NewDefaultTransactionMonitor(TransactionConfig{EnableMetrics: true})

	monitor.OnTransactionRollback("tx_test", &TransactionResult{Error: nil})
	monitor.OnTransactionRetry("tx_test", 1, nil)
}

func TestDefaultTransactionMonitor_DisabledSilent(t *testing.T) {
	monitor := NewDefaultTransactionMonitor(TransactionConfig{})

	// All callbacks are no-ops when metrics and slow-query logging are off;
	// they must not panic on any payload.
	monitor.OnTransactionStart("tx_test", TransactionConfig{})
	monitor.OnTransactionCommit("tx_test", &TransactionResult{})
	monitor.OnTransactionRollback("tx_test", &TransactionResult{Error: nil})
	monitor.OnTransactionRetry("tx_test", 1, nil)
	monitor.OnSlowQuery("tx_test", "SELECT 1", time.Second)
}

// ==================== ConnectionPool ====================

func TestConnectionPool_InitializeAppliesSettings(t *testing.T) {
	db, _ := sqlmockNewDB(t)
	defer db.Close()

	pool := NewConnectionPool(TransactionConfig{
		MaxOpenConns:    11,
		MaxIdleConns:    3,
		ConnMaxLifetime: time.Minute,
		ConnMaxIdleTime: 30 * time.Second,
	})

	if err := pool.Initialize(db); err != nil {
		t.Fatalf("Initialize error: %v", err)
	}

	stats := pool.GetDB().Stats()
	if stats.MaxOpenConnections != 11 {
		t.Errorf("MaxOpenConnections = %d, want 11", stats.MaxOpenConnections)
	}
	// MaxIdleConns/ConnMaxLifetime/ConnMaxIdleTime have no direct getter on
	// sql.DBStats; calling Initialize covers their Set* invocations.
}

func TestConnectionPool_GetDB_Uninitialized(t *testing.T) {
	pool := NewConnectionPool(TransactionConfig{})
	if got := pool.GetDB(); got != nil {
		t.Errorf("GetDB on uninitialized pool = %v, want nil", got)
	}
}

func TestConnectionPool_Close(t *testing.T) {
	t.Run("uninitialized returns nil", func(t *testing.T) {
		pool := NewConnectionPool(TransactionConfig{})
		if err := pool.Close(); err != nil {
			t.Errorf("Close on uninitialized pool = %v, want nil", err)
		}
	})

	t.Run("closes underlying db", func(t *testing.T) {
		db, _ := sqlmockNewDB(t)
		pool := NewConnectionPool(TransactionConfig{})
		if err := pool.Initialize(db); err != nil {
			t.Fatalf("Initialize error: %v", err)
		}
		if err := pool.Close(); err != nil {
			t.Errorf("Close error = %v, want nil", err)
		}
		if err := db.Ping(); err == nil {
			t.Error("db.Ping after Close should fail")
		}
	})
}
