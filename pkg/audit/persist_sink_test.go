package audit

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"
)

func TestFileSinkWrite(t *testing.T) {
	buf := &bytes.Buffer{}
	sink := NewWriterFileSink(buf)

	err := sink.Write(context.Background(), Event{Action: ActionCreate, Path: "/users"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"action":"create"`) {
		t.Fatalf("expected action json, got %s", out)
	}
	if !strings.HasSuffix(out, "\n") {
		t.Fatalf("expected newline terminated json line")
	}
}

func TestFileSinkClose(t *testing.T) {
	f, err := os.CreateTemp("", "audit-sink-*.log")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	path := f.Name()
	_ = f.Close()
	defer os.Remove(path)

	sink, err := NewFileSink(path)
	if err != nil {
		t.Fatalf("new file sink: %v", err)
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
}

type fakeSQLExecer struct {
	query string
	args  []any
	err   error
}

func (f *fakeSQLExecer) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	f.query = query
	f.args = args
	return nil, f.err
}

func TestNewSQLSinkInvalidTable(t *testing.T) {
	_, err := NewSQLSink(&fakeSQLExecer{}, SQLSinkConfig{Table: "audit-events;drop"})
	if err != ErrInvalidAuditTable {
		t.Fatalf("expected ErrInvalidAuditTable, got %v", err)
	}
}

func TestSQLSinkWrite(t *testing.T) {
	execer := &fakeSQLExecer{}
	sink, err := NewSQLSink(execer, DefaultSQLSinkConfig())
	if err != nil {
		t.Fatalf("new sql sink: %v", err)
	}
	event := Event{
		Timestamp:  time.Now(),
		RequestID:  "r1",
		Action:     ActionUpdate,
		Path:       "/users/1",
		StatusCode: 200,
		Metadata: map[string]interface{}{
			"k": "v",
		},
	}

	if err := sink.Write(context.Background(), event); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if !strings.Contains(execer.query, "INSERT INTO audit_events") {
		t.Fatalf("unexpected query: %s", execer.query)
	}
	if len(execer.args) != 17 {
		t.Fatalf("unexpected args len: %d", len(execer.args))
	}
	metadata, ok := execer.args[14].(string)
	if !ok || !strings.Contains(metadata, `"k":"v"`) {
		t.Fatalf("unexpected metadata arg: %#v", execer.args[14])
	}
}
