package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"
)

var (
	ErrInvalidAuditTable = errors.New("invalid audit table name")
)

var auditTableNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_\.]+$`)

// SQLExecer is the minimal DB contract needed by SQLSink.
type SQLExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// SQLSinkConfig controls SQL sink behavior.
type SQLSinkConfig struct {
	Table string
}

// DefaultSQLSinkConfig returns default SQL sink config.
func DefaultSQLSinkConfig() SQLSinkConfig {
	return SQLSinkConfig{Table: "audit_events"}
}

// SQLSink persists audit events into relational database.
type SQLSink struct {
	execer SQLExecer
	table  string
}

// NewSQLSink creates a SQL sink using given DB execer.
func NewSQLSink(execer SQLExecer, cfg SQLSinkConfig) (*SQLSink, error) {
	if execer == nil {
		return nil, fmt.Errorf("sql execer is nil")
	}
	if cfg.Table == "" {
		cfg.Table = DefaultSQLSinkConfig().Table
	}
	if !auditTableNamePattern.MatchString(cfg.Table) {
		return nil, ErrInvalidAuditTable
	}
	return &SQLSink{execer: execer, table: cfg.Table}, nil
}

// Write inserts a normalized event row.
func (s *SQLSink) Write(ctx context.Context, event Event) error {
	start := time.Now()
	metadata, err := json.Marshal(event.Metadata)
	if err != nil {
		recordAuditWriteMetric("sql", "error", time.Since(start))
		return err
	}

	query := fmt.Sprintf("INSERT INTO %s (timestamp, request_id, trace_id, tenant_id, user_id, username, client_ip, method, path, status_code, action, resource, result, message, metadata, sensitive, duration_ms) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", s.table)

	_, err = s.execer.ExecContext(ctx, query,
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
	if err != nil {
		recordAuditWriteMetric("sql", "error", time.Since(start))
		return err
	}
	recordAuditWriteMetric("sql", "success", time.Since(start))
	return err
}
