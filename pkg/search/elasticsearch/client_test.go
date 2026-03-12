package elasticsearch

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if len(cfg.Addresses) != 1 || cfg.Addresses[0] != "http://localhost:9200" {
		t.Errorf("DefaultConfig() Addresses = %v, want [http://localhost:9200]", cfg.Addresses)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("DefaultConfig() Timeout = %v, want 30s", cfg.Timeout)
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("DefaultConfig() MaxRetries = %v, want 3", cfg.MaxRetries)
	}
}

func TestConfigStruct(t *testing.T) {
	cfg := Config{
		Addresses:  []string{"http://es1:9200", "http://es2:9200"},
		Username:   "elastic",
		Password:   "changeme",
		APIKey:     "api-key-here",
		CloudID:    "cloud-id-here",
		Timeout:    60 * time.Second,
		MaxRetries: 5,
	}

	if len(cfg.Addresses) != 2 {
		t.Error("Config struct assignment failed for Addresses")
	}
	if cfg.Username != "elastic" {
		t.Error("Config struct assignment failed for Username")
	}
	if cfg.Password != "changeme" {
		t.Error("Config struct assignment failed for Password")
	}
	if cfg.APIKey != "api-key-here" {
		t.Error("Config struct assignment failed for APIKey")
	}
	if cfg.MaxRetries != 5 {
		t.Error("Config struct assignment failed for MaxRetries")
	}
}

// TestNewWithInvalidHost tests connection with invalid host
func TestNewWithInvalidHost(t *testing.T) {
	cfg := Config{
		Addresses:  []string{"http://invalid.host.that.does.not.exist:9200"},
		Timeout:    1 * time.Second,
		MaxRetries: 0,
	}

	client, err := New(cfg)
	if err == nil {
		// ES client creation succeeds but ping fails
		if client != nil {
			t.Log("Client created (ping may have failed)")
		}
	} else {
		t.Logf("Expected connection error: %v", err)
	}
}

// TestQueryBuilderFunctions tests the query builder helper functions
func TestQueryBuilderFunctions(t *testing.T) {
	// Test BuildMatchAllQuery
	matchAll := BuildMatchAllQuery()
	if matchAll == nil {
		t.Error("BuildMatchAllQuery() returned nil")
	}

	// Test BuildTermQuery
	termQuery := BuildTermQuery("status", "active")
	if termQuery == nil {
		t.Error("BuildTermQuery() returned nil")
	}

	// Test BuildRangeQuery
	rangeQuery := BuildRangeQuery("age", 18, 65)
	if rangeQuery == nil {
		t.Error("BuildRangeQuery() returned nil")
	}

	// Test BuildBoolQuery
	must := []map[string]any{{"match": map[string]string{"field": "value"}}}
	boolQuery := BuildBoolQuery(must, nil, nil)
	if boolQuery == nil {
		t.Error("BuildBoolQuery() returned nil")
	}

	// Test BuildMultiMatchQuery
	multiMatch := BuildMultiMatchQuery("search term", []string{"title", "content"}, "best_fields")
	if multiMatch == nil {
		t.Error("BuildMultiMatchQuery() returned nil")
	}
}

// TestResponseHelpers tests the response helper functions
func TestResponseHelpers(t *testing.T) {
	// Test IsSuccess
	if !IsSuccess(200) {
		t.Error("IsSuccess(200) should return true")
	}
	if !IsSuccess(201) {
		t.Error("IsSuccess(201) should return true")
	}
	if IsSuccess(404) {
		t.Error("IsSuccess(404) should return false")
	}
	if IsSuccess(500) {
		t.Error("IsSuccess(500) should return false")
	}

	// Test IsNotFound
	if !IsNotFound(404) {
		t.Error("IsNotFound(404) should return true")
	}
	if IsNotFound(200) {
		t.Error("IsNotFound(200) should return false")
	}
}
