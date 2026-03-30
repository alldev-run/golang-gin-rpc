package audit

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestBatchSQLSink_WriteAndFlush(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	cfg := BatchSQLSinkConfig{
		Table:         "audit_events",
		BatchSize:     2,
		FlushInterval: 1 * time.Hour, // 禁用自动刷新以便测试
		MaxRetries:    1,
	}

	sink, err := NewBatchSQLSink(db, cfg)
	assert.NoError(t, err)
	defer sink.Close()

	event1 := Event{
		Timestamp: time.Now(),
		RequestID: "req-1",
		Action:    ActionCreate,
		Result:    "success",
	}
	event2 := Event{
		Timestamp: time.Now(),
		RequestID: "req-2",
		Action:    ActionUpdate,
		Result:    "success",
	}

	// 第一条写入，未满 batch，不触发 flush
	mock.ExpectExec("INSERT INTO audit_events").
		WillReturnResult(sqlmock.NewResult(1, 2))

	err = sink.Write(context.Background(), event1)
	assert.NoError(t, err)

	err = sink.Write(context.Background(), event2)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBatchSQLSink_Flush(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	cfg := BatchSQLSinkConfig{
		Table:         "audit_events",
		BatchSize:     100, // 大 batch size
		FlushInterval: 1 * time.Hour,
		MaxRetries:    1,
	}

	sink, err := NewBatchSQLSink(db, cfg)
	assert.NoError(t, err)
	defer sink.Close()

	event := Event{
		Timestamp: time.Now(),
		RequestID: "req-1",
		Action:    ActionCreate,
		Result:    "success",
	}

	// 手动触发 flush
	mock.ExpectExec("INSERT INTO audit_events").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = sink.Write(context.Background(), event)
	assert.NoError(t, err)

	err = sink.Flush(context.Background())
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBatchSQLSink_AutoFlush(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	cfg := BatchSQLSinkConfig{
		Table:         "audit_events",
		BatchSize:     100,
		FlushInterval: 100 * time.Millisecond,
		MaxRetries:    1,
	}

	sink, err := NewBatchSQLSink(db, cfg)
	assert.NoError(t, err)
	defer sink.Close()

	event := Event{
		Timestamp: time.Now(),
		RequestID: "req-1",
		Action:    ActionCreate,
		Result:    "success",
	}

	// 等待自动刷新
	mock.ExpectExec("INSERT INTO audit_events").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = sink.Write(context.Background(), event)
	assert.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBatchSQLSink_Retry(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	cfg := BatchSQLSinkConfig{
		Table:         "audit_events",
		BatchSize:     1,
		FlushInterval: 1 * time.Hour,
		MaxRetries:    3,
		RetryDelay:    10 * time.Millisecond,
	}

	sink, err := NewBatchSQLSink(db, cfg)
	assert.NoError(t, err)
	defer sink.Close()

	event := Event{
		Timestamp: time.Now(),
		RequestID: "req-1",
		Action:    ActionCreate,
		Result:    "success",
	}

	// 前两次失败，第三次成功
	mock.ExpectExec("INSERT INTO audit_events").
		WillReturnError(sql.ErrConnDone)
	mock.ExpectExec("INSERT INTO audit_events").
		WillReturnError(sql.ErrConnDone)
	mock.ExpectExec("INSERT INTO audit_events").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = sink.Write(context.Background(), event)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBatchSQLSink_Close(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	cfg := BatchSQLSinkConfig{
		Table:         "audit_events",
		BatchSize:     100,
		FlushInterval: 1 * time.Hour,
	}

	sink, err := NewBatchSQLSink(db, cfg)
	assert.NoError(t, err)

	event := Event{
		Timestamp: time.Now(),
		RequestID: "req-1",
		Action:    ActionCreate,
		Result:    "success",
	}

	mock.ExpectExec("INSERT INTO audit_events").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = sink.Write(context.Background(), event)
	assert.NoError(t, err)

	err = sink.Close()
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBatchSQLSink_WriteAfterClose(t *testing.T) {
	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	cfg := DefaultBatchSQLSinkConfig()
	sink, err := NewBatchSQLSink(db, cfg)
	assert.NoError(t, err)

	sink.Close()

	event := Event{
		RequestID: "req-1",
		Action:    ActionCreate,
	}

	err = sink.Write(context.Background(), event)
	assert.Equal(t, ErrBatchSQLSinkClosed, err)
}

func TestDefaultBatchSQLSinkConfig(t *testing.T) {
	cfg := DefaultBatchSQLSinkConfig()
	assert.Equal(t, "audit_events", cfg.Table)
	assert.Equal(t, 100, cfg.BatchSize)
	assert.Equal(t, 5*time.Second, cfg.FlushInterval)
	assert.Equal(t, 3, cfg.MaxRetries)
	assert.Equal(t, 100*time.Millisecond, cfg.RetryDelay)
}
