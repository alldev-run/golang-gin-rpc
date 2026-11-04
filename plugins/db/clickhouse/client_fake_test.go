package clickhouse

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// ==================== fake driver.Conn ====================

type fakeConn struct {
	driver.Conn
	pingErr      error
	closeErr     error
	queryErr     error
	execErr      error
	prepareErr   error
	queryRows    driver.Rows
	queryRow     driver.Row
	batch        driver.Batch
	batchErr     error
}

func (f *fakeConn) Ping(ctx context.Context) error { return f.pingErr }
func (f *fakeConn) Close() error                    { return f.closeErr }

func (f *fakeConn) Query(ctx context.Context, query string, args ...any) (driver.Rows, error) {
	return f.queryRows, f.queryErr
}

func (f *fakeConn) QueryRow(ctx context.Context, query string, args ...any) driver.Row {
	return f.queryRow
}

func (f *fakeConn) Exec(ctx context.Context, query string, args ...any) error {
	return f.execErr
}

func (f *fakeConn) PrepareBatch(ctx context.Context, query string, opts ...driver.PrepareBatchOption) (driver.Batch, error) {
	return f.batch, f.prepareErr
}

// ==================== fake driver.Rows ====================

type fakeRows struct {
	driver.Rows
	closeErr error
}

func (f *fakeRows) Close() error { return f.closeErr }

// ==================== fake driver.Row ====================

type fakeRow struct {
	driver.Row
}

// ==================== fake driver.Batch ====================

type fakeBatch struct {
	driver.Batch
	appendErr error
	sendErr   error
}

func (f *fakeBatch) Append(v ...any) error { return f.appendErr }
func (f *fakeBatch) Send() error           { return f.sendErr }

// ==================== applyConfigDefaults ====================

func TestApplyConfigDefaults(t *testing.T) {
	t.Run("empty_config", func(t *testing.T) {
		cfg := Config{}
		applyConfigDefaults(&cfg)
		if len(cfg.Hosts) != 1 || cfg.Hosts[0] != "localhost:9000" {
			t.Errorf("Hosts = %v, want [localhost:9000]", cfg.Hosts)
		}
		if cfg.Database != "default" {
			t.Errorf("Database = %q, want default", cfg.Database)
		}
	})
	t.Run("already_set", func(t *testing.T) {
		cfg := Config{Hosts: []string{"custom:9000"}, Database: "mydb"}
		applyConfigDefaults(&cfg)
		if cfg.Hosts[0] != "custom:9000" {
			t.Error("Hosts should not be overridden")
		}
		if cfg.Database != "mydb" {
			t.Error("Database should not be overridden")
		}
	})
}

// ==================== decodeConfig ====================

func TestDecodeConfig(t *testing.T) {
	t.Run("direct_assertion", func(t *testing.T) {
		input := Config{Hosts: []string{"h1"}, Database: "db1"}
		got, ok := decodeConfig[Config](input)
		if !ok {
			t.Fatal("expected ok=true for direct assertion")
		}
		if got.Database != "db1" {
			t.Errorf("Database = %q, want db1", got.Database)
		}
	})
	t.Run("map_assertion", func(t *testing.T) {
		input := map[string]any{
			"hosts":    []any{"h1"},
			"database": "db1",
		}
		got, ok := decodeConfig[Config](input)
		if !ok {
			t.Fatal("expected ok=true for map assertion")
		}
		if got.Database != "db1" {
			t.Errorf("Database = %q, want db1", got.Database)
		}
	})
	t.Run("nil_map", func(t *testing.T) {
		var input map[string]any
		_, ok := decodeConfig[Config](input)
		if ok {
			t.Error("expected ok=false for nil map")
		}
	})
	t.Run("wrong_type", func(t *testing.T) {
		_, ok := decodeConfig[Config]("not a config")
		if ok {
			t.Error("expected ok=false for wrong type")
		}
	})
}

// ==================== Client methods via fake conn ====================

func TestClient_Conn(t *testing.T) {
	fc := &fakeConn{}
	c := newClientWithConn(fc, DefaultConfig())
	if c.Conn() != fc {
		t.Error("Conn() should return the injected conn")
	}
}

func TestClient_Close(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fc := &fakeConn{}
		c := newClientWithConn(fc, DefaultConfig())
		if err := c.Close(); err != nil {
			t.Errorf("Close error = %v", err)
		}
	})
	t.Run("error", func(t *testing.T) {
		fc := &fakeConn{closeErr: errors.New("close failed")}
		c := newClientWithConn(fc, DefaultConfig())
		if err := c.Close(); err == nil {
			t.Error("expected error from Close")
		}
	})
}

func TestClient_Ping(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fc := &fakeConn{}
		c := newClientWithConn(fc, DefaultConfig())
		if err := c.Ping(context.Background()); err != nil {
			t.Errorf("Ping error = %v", err)
		}
	})
	t.Run("error", func(t *testing.T) {
		fc := &fakeConn{pingErr: errors.New("ping failed")}
		c := newClientWithConn(fc, DefaultConfig())
		if err := c.Ping(context.Background()); err == nil {
			t.Error("expected error from Ping")
		}
	})
}

func TestClient_Query(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fr := &fakeRows{}
		fc := &fakeConn{queryRows: fr}
		c := newClientWithConn(fc, DefaultConfig())
		rows, err := c.Query(context.Background(), "SELECT 1")
		if err != nil {
			t.Fatalf("Query error = %v", err)
		}
		if rows != fr {
			t.Error("Query should return the fake rows")
		}
	})
	t.Run("error", func(t *testing.T) {
		fc := &fakeConn{queryErr: errors.New("query failed")}
		c := newClientWithConn(fc, DefaultConfig())
		_, err := c.Query(context.Background(), "SELECT 1")
		if err == nil {
			t.Error("expected error from Query")
		}
	})
}

func TestClient_QueryRow(t *testing.T) {
	fr := fakeRow{}
	fc := &fakeConn{queryRow: fr}
	c := newClientWithConn(fc, DefaultConfig())
	row := c.QueryRow(context.Background(), "SELECT 1")
	if row == nil {
		t.Error("QueryRow should not return nil")
	}
}

func TestClient_Exec(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fc := &fakeConn{}
		c := newClientWithConn(fc, DefaultConfig())
		if err := c.Exec(context.Background(), "INSERT INTO t VALUES (1)"); err != nil {
			t.Errorf("Exec error = %v", err)
		}
	})
	t.Run("error", func(t *testing.T) {
		fc := &fakeConn{execErr: errors.New("exec failed")}
		c := newClientWithConn(fc, DefaultConfig())
		if err := c.Exec(context.Background(), "INSERT INTO t VALUES (1)"); err == nil {
			t.Error("expected error from Exec")
		}
	})
}

func TestClient_InsertBatch(t *testing.T) {
	t.Run("empty_rows", func(t *testing.T) {
		fc := &fakeConn{}
		c := newClientWithConn(fc, DefaultConfig())
		err := c.InsertBatch(context.Background(), "t", []string{"a"}, nil)
		if err != nil {
			t.Errorf("InsertBatch with empty rows error = %v", err)
		}
	})
	t.Run("success", func(t *testing.T) {
		fb := &fakeBatch{}
		fc := &fakeConn{batch: fb}
		c := newClientWithConn(fc, DefaultConfig())
		rows := [][]any{{1, "a"}, {2, "b"}}
		err := c.InsertBatch(context.Background(), "t", []string{"id", "name"}, rows)
		if err != nil {
			t.Errorf("InsertBatch error = %v", err)
		}
	})
	t.Run("prepare_error", func(t *testing.T) {
		fc := &fakeConn{prepareErr: errors.New("prepare failed")}
		c := newClientWithConn(fc, DefaultConfig())
		rows := [][]any{{1}}
		err := c.InsertBatch(context.Background(), "t", []string{"id"}, rows)
		if err == nil {
			t.Error("expected error from PrepareBatch")
		}
	})
	t.Run("append_error", func(t *testing.T) {
		fb := &fakeBatch{appendErr: errors.New("append failed")}
		fc := &fakeConn{batch: fb}
		c := newClientWithConn(fc, DefaultConfig())
		rows := [][]any{{1}}
		err := c.InsertBatch(context.Background(), "t", []string{"id"}, rows)
		if err == nil {
			t.Error("expected error from Append")
		}
	})
	t.Run("send_error", func(t *testing.T) {
		fb := &fakeBatch{sendErr: errors.New("send failed")}
		fc := &fakeConn{batch: fb}
		c := newClientWithConn(fc, DefaultConfig())
		rows := [][]any{{1}}
		err := c.InsertBatch(context.Background(), "t", []string{"id"}, rows)
		if err == nil {
			t.Error("expected error from Send")
		}
	})
}

func TestClient_BatchCreate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fb := &fakeBatch{}
		fc := &fakeConn{batch: fb}
		c := newClientWithConn(fc, DefaultConfig())
		batch, err := c.BatchCreate(context.Background(), "INSERT INTO t VALUES")
		if err != nil {
			t.Fatalf("BatchCreate error = %v", err)
		}
		if batch != fb {
			t.Error("BatchCreate should return the fake batch")
		}
	})
	t.Run("error", func(t *testing.T) {
		fc := &fakeConn{prepareErr: errors.New("prepare failed")}
		c := newClientWithConn(fc, DefaultConfig())
		_, err := c.BatchCreate(context.Background(), "INSERT INTO t VALUES")
		if err == nil {
			t.Error("expected error from BatchCreate")
		}
	})
}

// ==================== dbClient adapter ====================

func TestDbClient_Ping(t *testing.T) {
	fc := &fakeConn{}
	chClient := newClientWithConn(fc, DefaultConfig())
	dbc := &dbClient{client: chClient}
	if err := dbc.Ping(context.Background()); err != nil {
		t.Errorf("Ping error = %v", err)
	}
}

func TestDbClient_Close(t *testing.T) {
	fc := &fakeConn{}
	chClient := newClientWithConn(fc, DefaultConfig())
	dbc := &dbClient{client: chClient}
	if err := dbc.Close(); err != nil {
		t.Errorf("Close error = %v", err)
	}
}

// ==================== Ensure sql import is used ====================

var _ = sql.ErrNoRows
