package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sync"
	"time"
)

var (
	ErrBatchSQLSinkClosed   = errors.New("batch sql sink closed")
	ErrBatchSizeInvalid     = errors.New("batch size must be greater than 0")
)

var batchAuditTableNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_\.]+$`)

// BatchSQLSinkConfig controls batch SQL sink behavior.
type BatchSQLSinkConfig struct {
	Table         string
	BatchSize     int           // 批量写入大小
	FlushInterval time.Duration // 自动刷新间隔
	MaxRetries    int           // 最大重试次数
	RetryDelay    time.Duration // 重试间隔
}

// DefaultBatchSQLSinkConfig returns default batch SQL sink config.
func DefaultBatchSQLSinkConfig() BatchSQLSinkConfig {
	return BatchSQLSinkConfig{
		Table:         "audit_events",
		BatchSize:     100,
		FlushInterval: 5 * time.Second,
		MaxRetries:    3,
		RetryDelay:    100 * time.Millisecond,
	}
}

// BatchSQLSink persists audit events with batching for high throughput.
type BatchSQLSink struct {
	execer SQLExecer
	cfg    BatchSQLSinkConfig

	mu       sync.Mutex
	buffer   []Event
	ticker   *time.Ticker
	stopCh   chan struct{}
	stopped  bool
	flushSem chan struct{} // 限制并发刷新
}

// NewBatchSQLSink creates a batching SQL sink.
func NewBatchSQLSink(execer SQLExecer, cfg BatchSQLSinkConfig) (*BatchSQLSink, error) {
	if execer == nil {
		return nil, fmt.Errorf("sql execer is nil")
	}
	if cfg.Table == "" {
		cfg.Table = DefaultBatchSQLSinkConfig().Table
	}
	if !batchAuditTableNamePattern.MatchString(cfg.Table) {
		return nil, ErrInvalidAuditTable
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = DefaultBatchSQLSinkConfig().BatchSize
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = DefaultBatchSQLSinkConfig().FlushInterval
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = DefaultBatchSQLSinkConfig().MaxRetries
	}
	if cfg.RetryDelay <= 0 {
		cfg.RetryDelay = DefaultBatchSQLSinkConfig().RetryDelay
	}

	s := &BatchSQLSink{
		execer:   execer,
		cfg:      cfg,
		buffer:   make([]Event, 0, cfg.BatchSize),
		ticker:   time.NewTicker(cfg.FlushInterval),
		stopCh:   make(chan struct{}),
		flushSem: make(chan struct{}, 1), // 单并发刷新
	}

	go s.backgroundFlush()
	return s, nil
}

// Write queues event for batch insert.
func (s *BatchSQLSink) Write(ctx context.Context, event Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopped {
		recordAuditDropMetric("batch_sql", "closed")
		return ErrBatchSQLSinkClosed
	}

	s.buffer = append(s.buffer, event)

	if len(s.buffer) >= s.cfg.BatchSize {
		return s.flushUnlocked(ctx)
	}
	return nil
}

// Flush immediately writes all buffered events.
func (s *BatchSQLSink) Flush(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.flushUnlocked(ctx)
}

// Close stops background ticker and flushes remaining events.
func (s *BatchSQLSink) Close() error {
	s.mu.Lock()
	s.stopped = true
	s.mu.Unlock()

	s.ticker.Stop()
	close(s.stopCh)

	// Final flush
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.Flush(ctx)
}

func (s *BatchSQLSink) backgroundFlush() {
	for {
		select {
		case <-s.ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			_ = s.Flush(ctx)
			cancel()
		case <-s.stopCh:
			return
		}
	}
}

func (s *BatchSQLSink) flushUnlocked(ctx context.Context) error {
	if len(s.buffer) == 0 {
		return nil
	}

	// 获取刷新许可
	select {
	case s.flushSem <- struct{}{}:
	default:
		// 已有刷新在进行，跳过本次
		return nil
	}
	defer func() { <-s.flushSem }()

	// 复制缓冲区并清空
	events := make([]Event, len(s.buffer))
	copy(events, s.buffer)
	s.buffer = s.buffer[:0]

	s.mu.Unlock()
	defer s.mu.Lock()

	return s.batchInsertWithRetry(ctx, events)
}

func (s *BatchSQLSink) batchInsertWithRetry(ctx context.Context, events []Event) error {
	start := time.Now()
	var lastErr error

	for attempt := 0; attempt < s.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(s.cfg.RetryDelay * time.Duration(attempt))
		}

		lastErr = s.batchInsert(ctx, events)
		if lastErr == nil {
			recordAuditWriteMetric("batch_sql", "success", time.Since(start))
			return nil
		}
	}

	recordAuditWriteMetric("batch_sql", "error", time.Since(start))
	return fmt.Errorf("batch insert failed after %d retries: %w", s.cfg.MaxRetries, lastErr)
}

func (s *BatchSQLSink) batchInsert(ctx context.Context, events []Event) error {
	if len(events) == 0 {
		return nil
	}

	// 构建批量插入语句
	query := fmt.Sprintf(
		"INSERT INTO %s (timestamp, request_id, trace_id, tenant_id, user_id, username, client_ip, method, path, status_code, action, resource, result, message, metadata, sensitive, duration_ms) VALUES ",
		s.cfg.Table,
	)

	args := make([]interface{}, 0, len(events)*17)
	placeholders := make([]string, 0, len(events))

	for _, event := range events {
		metadata, err := json.Marshal(event.Metadata)
		if err != nil {
			return err
		}

		placeholders = append(placeholders, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
		args = append(args,
			event.Timestamp,
			event.RequestID,
			event.TraceID,
			event.TenantID,
			event.UserID,
			event.Username,
			event.ClientIP,
			event.Method,
			event.Path,
			event.StatusCode,
			string(event.Action),
			event.Resource,
			event.Result,
			event.Message,
			string(metadata),
			event.Sensitive,
			event.DurationMS,
		)
	}

	query += joinStrings(placeholders, ", ")
	_, err := s.execer.ExecContext(ctx, query, args...)
	return err
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
