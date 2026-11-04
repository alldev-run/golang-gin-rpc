package rwproxy

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"sync"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// newSQLMockDB creates a standalone sqlmock database.
func newSQLMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()

	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	return db, mock
}

// newSplitClient wires one master and two replicas backed by sqlmock.
func newSplitClient(t *testing.T) (*Client, sqlmock.Sqlmock, sqlmock.Sqlmock, sqlmock.Sqlmock, func()) {
	t.Helper()

	masterDB, masterMock := newSQLMockDB(t)
	replica1, mock1 := newSQLMockDB(t)
	replica2, mock2 := newSQLMockDB(t)

	client := New(Config{
		Master:   masterDB,
		Replicas: []*sql.DB{replica1, replica2},
		Strategy: LBStrategyRoundRobin,
	})

	cleanup := func() {
		_ = masterDB.Close()
		_ = replica1.Close()
		_ = replica2.Close()
	}
	return client, masterMock, mock1, mock2, cleanup
}

func TestGetDBRouting(t *testing.T) {
	client, _, mock1, mock2, cleanup := newSplitClient(t)
	defer cleanup()

	master := client.GetMasterDB()
	if master == nil {
		t.Fatal("GetMasterDB returned nil")
	}

	// Writes always go to master.
	if got := client.getDB(QueryWrite); got != master {
		t.Error("getDB(QueryWrite) should return master")
	}

	// Reads round-robin across replicas.
	first := client.getDB(QueryRead)
	second := client.getDB(QueryRead)
	if first == nil || second == nil {
		t.Fatal("getDB(QueryRead) returned nil with replicas configured")
	}
	if first == second {
		t.Error("round-robin should alternate between replicas")
	}
	if first == master || second == master {
		t.Error("reads should not hit master when replicas are available")
	}

	// Distinguish which replica was hit first by executing a query on each.
	ctx := context.Background()
	mock1.ExpectQuery(regexp.QuoteMeta("SELECT 1")).WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock2.ExpectQuery(regexp.QuoteMeta("SELECT 1")).WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))

	rows, err := client.Query(ctx, "SELECT 1")
	if err != nil {
		t.Fatalf("Query error = %v", err)
	}
	_ = rows.Close()
	rows, err = client.Query(ctx, "SELECT 1")
	if err != nil {
		t.Fatalf("Query error = %v", err)
	}
	_ = rows.Close()

	if err := mock1.ExpectationsWereMet(); err != nil {
		t.Errorf("replica 1 expectations: %v", err)
	}
	if err := mock2.ExpectationsWereMet(); err != nil {
		t.Errorf("replica 2 expectations: %v", err)
	}
}

func TestGetDBRoutingWithForceMaster(t *testing.T) {
	client, _, _, _, cleanup := newSplitClient(t)
	defer cleanup()

	client.ForceMaster(true)
	master := client.GetMasterDB()

	if got := client.getDB(QueryRead); got != master {
		t.Error("ForceMaster should route reads to master")
	}
	if got := client.getDB(QueryWrite); got != master {
		t.Error("writes should still go to master")
	}
	if !client.IsMasterForced() {
		t.Error("IsMasterForced should be true")
	}

	client.ForceMaster(false)
	if client.IsMasterForced() {
		t.Error("IsMasterForced should be false after ForceMaster(false)")
	}
}

func TestGetDBFallbackToMasterWithoutReplicas(t *testing.T) {
	masterDB, _ := newSQLMockDB(t)
	defer masterDB.Close()

	client := New(Config{Master: masterDB})
	if got := client.getDB(QueryRead); got != masterDB {
		t.Error("reads should fall back to master when no replicas exist")
	}
}

func TestSelectReplicaNilWithoutReplicas(t *testing.T) {
	client := New(Config{Master: nil})
	if db := client.selectReplica(); db != nil {
		t.Error("selectReplica should return nil without replicas")
	}

	// ForceMaster short-circuits replica selection.
	masterDB, _ := newSQLMockDB(t)
	defer masterDB.Close()
	client2 := New(Config{Master: masterDB, Replicas: []*sql.DB{masterDB}, ForceMaster: true})
	if db := client2.selectReplica(); db != nil {
		t.Error("selectReplica should return nil when master is forced")
	}
}

func TestClientQueryReadOnReplica(t *testing.T) {
	client, masterMock, mock1, mock2, cleanup := newSplitClient(t)
	defer cleanup()

	// The round-robin counter starts at 0; the first Add(1) yields 1%2=1 → replica[1] (mock2).
	mock2.ExpectQuery(regexp.QuoteMeta("SELECT name FROM users WHERE id = ?")).
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("alice"))

	ctx := context.Background()
	var name string
	if err := client.QueryRow(ctx, "SELECT name FROM users WHERE id = ?", 1).Scan(&name); err != nil {
		t.Fatalf("QueryRow error = %v", err)
	}
	if name != "alice" {
		t.Errorf("name = %q, want alice", name)
	}

	// Master and mock1 must not have been touched.
	if err := masterMock.ExpectationsWereMet(); err != nil {
		t.Errorf("master unexpectedly used: %v", err)
	}
	if err := mock1.ExpectationsWereMet(); err != nil {
		t.Errorf("replica 1 unexpectedly used: %v", err)
	}
	_ = mock2
}

func TestClientExecGoesToMaster(t *testing.T) {
	client, masterMock, mock1, mock2, cleanup := newSplitClient(t)
	defer cleanup()

	masterMock.ExpectExec(regexp.QuoteMeta("INSERT INTO users (name) VALUES (?)")).
		WithArgs("bob").
		WillReturnResult(sqlmock.NewResult(5, 1))

	res, err := client.Exec(context.Background(), "INSERT INTO users (name) VALUES (?)", "bob")
	if err != nil {
		t.Fatalf("Exec error = %v", err)
	}
	if id, _ := res.LastInsertId(); id != 5 {
		t.Errorf("LastInsertId = %d, want 5", id)
	}

	if err := masterMock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet master expectations: %v", err)
	}
	if err := mock1.ExpectationsWereMet(); err != nil {
		t.Errorf("replica 1 unexpectedly used: %v", err)
	}
	if err := mock2.ExpectationsWereMet(); err != nil {
		t.Errorf("replica 2 unexpectedly used: %v", err)
	}
}

func TestClientPrepareGoesToMaster(t *testing.T) {
	client, masterMock, _, _, cleanup := newSplitClient(t)
	defer cleanup()

	masterMock.ExpectPrepare(regexp.QuoteMeta("SELECT name FROM users WHERE id = ?")).
		ExpectQuery().
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("carol"))

	stmt, err := client.Prepare(context.Background(), "SELECT name FROM users WHERE id = ?")
	if err != nil {
		t.Fatalf("Prepare error = %v", err)
	}
	defer stmt.Close()

	var name string
	if err := stmt.QueryRow(1).Scan(&name); err != nil {
		t.Fatalf("stmt.QueryRow error = %v", err)
	}
	if name != "carol" {
		t.Errorf("name = %q, want carol", name)
	}
	if err := masterMock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet master expectations: %v", err)
	}
}

func TestClientTransactionCommit(t *testing.T) {
	client, masterMock, _, _, cleanup := newSplitClient(t)
	defer cleanup()

	masterMock.ExpectBegin()
	masterMock.ExpectExec(regexp.QuoteMeta("UPDATE users SET name = ? WHERE id = ?")).
		WithArgs("dave", 1).
		WillReturnResult(sqlmock.NewResult(0, 1))
	masterMock.ExpectCommit()

	err := client.Transaction(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec("UPDATE users SET name = ? WHERE id = ?", "dave", 1)
		return err
	})
	if err != nil {
		t.Fatalf("Transaction error = %v", err)
	}
	if err := masterMock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet master expectations: %v", err)
	}
}

func TestClientTransactionRollbackOnError(t *testing.T) {
	client, masterMock, _, _, cleanup := newSplitClient(t)
	defer cleanup()

	masterMock.ExpectBegin()
	masterMock.ExpectRollback()

	wantErr := errors.New("constraint violation")
	err := client.Transaction(context.Background(), func(tx *sql.Tx) error { return wantErr })
	if !errors.Is(err, wantErr) {
		t.Errorf("Transaction error = %v, want %v", err, wantErr)
	}
	if err := masterMock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet master expectations: %v", err)
	}
}

func TestClientTransactionRollbackOnPanic(t *testing.T) {
	client, masterMock, _, _, cleanup := newSplitClient(t)
	defer cleanup()

	masterMock.ExpectBegin()
	masterMock.ExpectRollback()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic to propagate after rollback")
		}
		if err := masterMock.ExpectationsWereMet(); err != nil {
			t.Errorf("unmet master expectations: %v", err)
		}
	}()

	_ = client.Transaction(context.Background(), func(tx *sql.Tx) error {
		panic("boom")
	})
}

func TestClientBeginWithoutMaster(t *testing.T) {
	client := New(Config{})
	_, err := client.Begin(context.Background(), nil)
	if err == nil {
		t.Fatal("Begin should fail without a master")
	}
	if want := "no master database available"; err.Error() != want {
		t.Errorf("Begin error = %v, want %q", err, want)
	}
}

func TestClientPing(t *testing.T) {
	client, masterMock, mock1, mock2, cleanup := newSplitClient(t)
	defer cleanup()

	masterMock.ExpectPing()
	mock1.ExpectPing()
	mock2.ExpectPing()

	if err := client.Ping(context.Background()); err != nil {
		t.Fatalf("Ping error = %v", err)
	}

	// A failing replica surfaces as an aggregate error.
	mock2.ExpectPing().WillReturnError(errors.New("replica down"))
	if err := client.Ping(context.Background()); err == nil {
		t.Fatal("Ping should fail when a replica is down")
	}

	if err := masterMock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet master expectations: %v", err)
	}
}

func TestClientPingConcurrent(t *testing.T) {
	client, masterMock, mock1, mock2, cleanup := newSplitClient(t)
	defer cleanup()

	masterMock.ExpectPing()
	mock1.ExpectPing()
	mock2.ExpectPing()

	// Ping fans out with goroutines; run it a few times to shake races.
	for i := 0; i < 3; i++ {
		masterMock.ExpectPing()
		mock1.ExpectPing()
		mock2.ExpectPing()
		if err := client.Ping(context.Background()); err != nil {
			t.Fatalf("Ping iteration %d error = %v", i, err)
		}
	}
}

func TestClientStats(t *testing.T) {
	client, _, _, _, cleanup := newSplitClient(t)
	defer cleanup()

	stats := client.Stats()
	if len(stats.Replicas) != 2 {
		t.Errorf("Stats().Replicas = %d entries, want 2", len(stats.Replicas))
	}

	// With a nil master, only replica stats are populated.
	client2 := New(Config{Replicas: []*sql.DB{nil}})
	stats2 := client2.Stats()
	if len(stats2.Replicas) != 0 {
		t.Errorf("nil replicas should be skipped, got %d", len(stats2.Replicas))
	}
}

func TestClientClose(t *testing.T) {
	masterDB, masterMock := newSQLMockDB(t)
	replica1, mock1 := newSQLMockDB(t)

	client := New(Config{
		Master:   masterDB,
		Replicas: []*sql.DB{replica1},
	})

	masterMock.ExpectClose()
	mock1.ExpectClose()
	if err := client.Close(); err != nil {
		t.Errorf("Close error = %v", err)
	}
	if err := masterMock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet master expectations: %v", err)
	}
	if err := mock1.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet replica expectations: %v", err)
	}
}

func TestClientCloseWithoutMaster(t *testing.T) {
	replica, mock := newSQLMockDB(t)
	defer replica.Close()

	client := New(Config{Replicas: []*sql.DB{replica}})
	mock.ExpectClose()
	if err := client.Close(); err != nil {
		t.Errorf("Close error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestClientAddRemoveReplica(t *testing.T) {
	client := New(Config{})

	if got := client.GetReplicaCount(); got != 0 {
		t.Errorf("initial replica count = %d, want 0", got)
	}

	replica, _ := newSQLMockDB(t)
	client.AddReplica(replica)
	if got := client.GetReplicaCount(); got != 1 {
		t.Errorf("replica count after AddReplica = %d, want 1", got)
	}

	if err := client.RemoveReplica(0); err != nil {
		t.Errorf("RemoveReplica(0) error = %v", err)
	}
	if err := client.RemoveReplica(-1); err == nil {
		t.Error("RemoveReplica(-1) should fail")
	}
	if err := client.RemoveReplica(5); err == nil {
		t.Error("RemoveReplica(5) should fail")
	}

	_ = replica.Close()
}

func TestQueryNilDBReturnsError(t *testing.T) {
	client := New(Config{})
	if _, err := client.Query(context.Background(), "SELECT 1"); err == nil {
		t.Error("Query without any database should fail")
	}
	if _, err := client.Exec(context.Background(), "INSERT INTO t VALUES (1)"); err == nil {
		t.Error("Exec without master should fail")
	}
	if _, err := client.Prepare(context.Background(), "SELECT 1"); err == nil {
		t.Error("Prepare without master should fail")
	}
	if row := client.QueryRow(context.Background(), "SELECT 1"); row != nil {
		t.Error("QueryRow without database should return nil")
	}
}

func TestLBStrategyConstants(t *testing.T) {
	if LBStrategyRoundRobin != 0 {
		t.Error("LBStrategyRoundRobin should be 0")
	}
	if LBStrategyRandom != 1 {
		t.Error("LBStrategyRandom should be 1")
	}
}

func TestQueryTypeConstants(t *testing.T) {
	if QueryUnknown != 0 || QueryWrite != 1 || QueryRead != 2 {
		t.Errorf("unexpected query type constants: %d %d %d", QueryUnknown, QueryWrite, QueryRead)
	}
}

// TestQueryTypeDetectionConcurrent guards the round-robin counter against races.
func TestQueryTypeDetectionConcurrent(t *testing.T) {
	client, _, mock1, mock2, cleanup := newSplitClient(t)
	defer cleanup()

	// After 10 goroutine increments (counter=10), next two queries go to
	// counter=11 → 11%2=1 → mock2, counter=12 → 12%2=0 → mock1.
	mock1.ExpectQuery(regexp.QuoteMeta("SELECT 1")).WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock2.ExpectQuery(regexp.QuoteMeta("SELECT 1")).WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = client.getDB(QueryRead)
			_ = client.getQueryType("SELECT 1")
		}()
	}
	wg.Wait()

	ctx := context.Background()
	rows, err := client.Query(ctx, "SELECT 1")
	if err != nil {
		t.Fatalf("Query error = %v", err)
	}
	_ = rows.Close()
	rows, err = client.Query(ctx, "SELECT 1")
	if err != nil {
		t.Fatalf("Query error = %v", err)
	}
	_ = rows.Close()

	// After 10 goroutine increments (counter=10), next two queries go to
	// counter=11 → 11%2=1 → mock2, counter=12 → 12%2=0 → mock1.
	if err := mock1.ExpectationsWereMet(); err != nil {
		t.Errorf("replica 1 expectations: %v", err)
	}
	if err := mock2.ExpectationsWereMet(); err != nil {
		t.Errorf("replica 2 expectations: %v", err)
	}
}
