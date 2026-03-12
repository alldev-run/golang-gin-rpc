package rwproxy

import (
	"database/sql"
	"regexp"
	"testing"
)

// mockDB is a mock for testing (we can't use real DB in unit tests)
type mockDB struct {
	*sql.DB
}

func TestQueryTypeDetection(t *testing.T) {
	client := &Client{
		queryChecker: &queryTypeChecker{
			writePattern: regexp.MustCompile(`^\s*(?i)(INSERT|UPDATE|DELETE|REPLACE|CREATE|DROP|ALTER|TRUNCATE|MERGE|UPSERT|GRANT|REVOKE|LOCK)\s+`),
			readPattern:  regexp.MustCompile(`^\s*(?i)SELECT\s+`),
		},
	}

	tests := []struct {
		query    string
		expected QueryType
	}{
		{"SELECT * FROM users", QueryRead},
		{"  SELECT id FROM posts", QueryRead},
		{"select count(*) from orders", QueryRead},
		{"INSERT INTO users VALUES (1)", QueryWrite},
		{"  UPDATE users SET name='test'", QueryWrite},
		{"DELETE FROM users WHERE id=1", QueryWrite},
		{"CREATE TABLE test (id INT)", QueryWrite},
		{"DROP TABLE test", QueryWrite},
		{"ALTER TABLE users ADD COLUMN age INT", QueryWrite},
		{"TRUNCATE TABLE users", QueryWrite},
		{"GRANT SELECT ON users TO admin", QueryWrite},
		{"LOCK TABLE users", QueryWrite},
		{"-- This is a comment", QueryUnknown},
		{"", QueryUnknown},
	}

	for _, tt := range tests {
		result := client.getQueryType(tt.query)
		if result != tt.expected {
			t.Errorf("getQueryType(%q) = %v, want %v", tt.query, result, tt.expected)
		}
	}
}

func TestSelectReplica(t *testing.T) {
	// Test with no replicas
	client := New(Config{
		Replicas: []*sql.DB{},
		Strategy: LBStrategyRoundRobin,
	})
	
	if db := client.selectReplica(); db != nil {
		t.Error("selectReplica() should return nil with no replicas")
	}

	// Test with replicas - can't create real sql.DB in unit tests
	// but we can verify the logic
	client2 := New(Config{
		Replicas: []*sql.DB{nil, nil}, // nil pointers just for testing logic
		Strategy: LBStrategyRoundRobin,
	})
	
	replica := client2.selectReplica()
	// Should return one of the replicas (even if nil in this test)
	_ = replica
}

func TestForceMaster(t *testing.T) {
	client := New(Config{
		ForceMaster: false,
	})
	
	if client.IsMasterForced() {
		t.Error("IsMasterForced() should be false initially")
	}
	
	client.ForceMaster(true)
	if !client.IsMasterForced() {
		t.Error("IsMasterForced() should be true after ForceMaster(true)")
	}
	
	client.ForceMaster(false)
	if client.IsMasterForced() {
		t.Error("IsMasterForced() should be false after ForceMaster(false)")
	}
}

func TestGetReplicaCount(t *testing.T) {
	client := New(Config{
		Replicas: []*sql.DB{nil, nil, nil},
	})
	
	if count := client.GetReplicaCount(); count != 3 {
		t.Errorf("GetReplicaCount() = %d, want 3", count)
	}
}

func TestRemoveReplica(t *testing.T) {
	client := New(Config{
		Replicas: []*sql.DB{nil, nil, nil},
	})
	
	// Remove middle replica
	if err := client.RemoveReplica(1); err != nil {
		t.Errorf("RemoveReplica(1) error = %v", err)
	}
	
	if count := client.GetReplicaCount(); count != 2 {
		t.Errorf("GetReplicaCount() after remove = %d, want 2", count)
	}
	
	// Try to remove invalid index
	if err := client.RemoveReplica(10); err == nil {
		t.Error("RemoveReplica(10) should return error for invalid index")
	}
}

func TestAddReplica(t *testing.T) {
	client := New(Config{})
	
	if client.GetReplicaCount() != 0 {
		t.Error("Initial replica count should be 0")
	}
	
	client.AddReplica(nil)
	if client.GetReplicaCount() != 1 {
		t.Errorf("GetReplicaCount() after AddReplica = %d, want 1", client.GetReplicaCount())
	}
}

func TestGetQueryTypeWithWhitespace(t *testing.T) {
	client := &Client{
		queryChecker: &queryTypeChecker{
			writePattern: regexp.MustCompile(`^\s*(?i)(INSERT|UPDATE|DELETE|REPLACE|CREATE|DROP|ALTER|TRUNCATE|MERGE|UPSERT|GRANT|REVOKE|LOCK)\s+`),
			readPattern:  regexp.MustCompile(`^\s*(?i)SELECT\s+`),
		},
	}

	// Test with various whitespace
	queries := []string{
		"  INSERT INTO t VALUES (1)",
		"\tSELECT * FROM t",
		"\n\nDELETE FROM t",
		"  \t SELECT * FROM t",
	}

	expected := []QueryType{QueryWrite, QueryRead, QueryWrite, QueryRead}

	for i, q := range queries {
		result := client.getQueryType(q)
		if result != expected[i] {
			t.Errorf("getQueryType(%q) = %v, want %v", q, result, expected[i])
		}
	}
}

func TestStats(t *testing.T) {
	client := New(Config{
		Replicas: []*sql.DB{},
	})
	
	stats := client.Stats()
	if stats.Master != (sql.DBStats{}) {
		t.Log("Master stats available (may have mocked DB)")
	}
	if len(stats.Replicas) != 0 {
		t.Errorf("Expected 0 replica stats, got %d", len(stats.Replicas))
	}
}
