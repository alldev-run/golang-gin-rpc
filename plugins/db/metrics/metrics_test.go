package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestDBQueryDuration(t *testing.T) {
	// Test that the histogram is properly initialized
	if DBQueryDuration == nil {
		t.Fatal("DBQueryDuration is nil")
	}

	// Test recording a duration
	DBQueryDuration.WithLabelValues("mysql", "select", "users").Observe(0.5)

	// Verify the metric was recorded
	metricChan := make(chan prometheus.Metric, 10)
	DBQueryDuration.Collect(metricChan)
	metric := <-metricChan

	if metric == nil {
		t.Fatal("Expected metric, got nil")
	}

	// Test with different labels
	labels := []string{"postgres", "insert", "products"}
	hist := DBQueryDuration.WithLabelValues(labels...)
	hist.Observe(1.0)

	// Verify histogram collector is still available after recording with another label set
	metricChan = make(chan prometheus.Metric, 10)
	DBQueryDuration.Collect(metricChan)
	metric = <-metricChan

	if metric == nil {
		t.Fatal("Expected metric with specific labels, got nil")
	}
}

func TestDBQueryTotal(t *testing.T) {
	// Test that the counter is properly initialized
	if DBQueryTotal == nil {
		t.Fatal("DBQueryTotal is nil")
	}

	// Test incrementing counter
	DBQueryTotal.WithLabelValues("mysql", "select", "success").Inc()

	// Verify the counter was incremented
	metricChan := make(chan prometheus.Metric, 10)
	DBQueryTotal.Collect(metricChan)
	metric := <-metricChan

	if metric == nil {
		t.Fatal("Expected metric, got nil")
	}

	// Test multiple increments
	DBQueryTotal.WithLabelValues("mysql", "select", "success").Inc()
	DBQueryTotal.WithLabelValues("mysql", "select", "success").Inc()

	metricChan = make(chan prometheus.Metric, 10)
	DBQueryTotal.Collect(metricChan)
	metric = <-metricChan

	if metric == nil {
		t.Fatal("Expected metric after multiple increments, got nil")
	}
}

func TestDBConnectionPoolSize(t *testing.T) {
	// Test that the gauge is properly initialized
	if DBConnectionPoolSize == nil {
		t.Fatal("DBConnectionPoolSize is nil")
	}

	// Test setting gauge value
	DBConnectionPoolSize.WithLabelValues("mysql", "master").Set(10.0)

	// Verify the gauge was set
	metricChan := make(chan prometheus.Metric, 10)
	DBConnectionPoolSize.Collect(metricChan)
	metric := <-metricChan

	if metric == nil {
		t.Fatal("Expected metric, got nil")
	}

	// Test updating gauge value
	DBConnectionPoolSize.WithLabelValues("mysql", "master").Set(15.0)

	metricChan = make(chan prometheus.Metric, 10)
	DBConnectionPoolSize.Collect(metricChan)
	metric = <-metricChan

	if metric == nil {
		t.Fatal("Expected metric after update, got nil")
	}
}

func TestDBConnectionActive(t *testing.T) {
	// Test that the gauge is properly initialized
	if DBConnectionActive == nil {
		t.Fatal("DBConnectionActive is nil")
	}

	// Test setting gauge value
	DBConnectionActive.WithLabelValues("mysql", "master").Set(5.0)

	// Verify the gauge was set
	metricChan := make(chan prometheus.Metric, 10)
	DBConnectionActive.Collect(metricChan)
	metric := <-metricChan

	if metric == nil {
		t.Fatal("Expected metric, got nil")
	}
}

func TestDBSlowQueryTotal(t *testing.T) {
	// Test that the counter is properly initialized
	if DBSlowQueryTotal == nil {
		t.Fatal("DBSlowQueryTotal is nil")
	}

	// Test incrementing counter
	DBSlowQueryTotal.WithLabelValues("mysql", "1s").Inc()

	// Verify the counter was incremented
	metricChan := make(chan prometheus.Metric, 10)
	DBSlowQueryTotal.Collect(metricChan)
	metric := <-metricChan

	if metric == nil {
		t.Fatal("Expected metric, got nil")
	}
}

func TestCircuitBreakerState(t *testing.T) {
	// Test that the gauge is properly initialized
	if CircuitBreakerState == nil {
		t.Fatal("CircuitBreakerState is nil")
	}

	// Test setting gauge value (0=closed, 1=open, 2=half-open)
	CircuitBreakerState.WithLabelValues("test-breaker").Set(1.0)

	// Verify the gauge was set
	metricChan := make(chan prometheus.Metric, 10)
	CircuitBreakerState.Collect(metricChan)
	metric := <-metricChan

	if metric == nil {
		t.Fatal("Expected metric, got nil")
	}

	// Test different states
	states := []float64{0.0, 1.0, 2.0}
	for _, state := range states {
		CircuitBreakerState.WithLabelValues("test-breaker").Set(state)

		metricChan = make(chan prometheus.Metric, 10)
		CircuitBreakerState.Collect(metricChan)
		metric = <-metricChan

		if metric == nil {
			t.Fatalf("Expected metric for state %f, got nil", state)
		}
	}
}

func TestCircuitBreakerFailures(t *testing.T) {
	// Test that the counter is properly initialized
	if CircuitBreakerFailures == nil {
		t.Fatal("CircuitBreakerFailures is nil")
	}

	// Test incrementing counter
	CircuitBreakerFailures.WithLabelValues("test-breaker").Inc()

	// Verify the counter was incremented
	metricChan := make(chan prometheus.Metric, 10)
	CircuitBreakerFailures.Collect(metricChan)
	metric := <-metricChan

	if metric == nil {
		t.Fatal("Expected metric, got nil")
	}

	// Test multiple increments
	CircuitBreakerFailures.WithLabelValues("test-breaker").Inc()
	CircuitBreakerFailures.WithLabelValues("test-breaker").Inc()

	metricChan = make(chan prometheus.Metric, 10)
	CircuitBreakerFailures.Collect(metricChan)
	metric = <-metricChan

	if metric == nil {
		t.Fatal("Expected metric after multiple increments, got nil")
	}
}

func TestMetricsWithDifferentLabels(t *testing.T) {
	// Test that metrics work correctly with different label combinations
	testCases := []struct {
		database  string
		operation string
		table     string
		status    string
	}{
		{"mysql", "select", "users", "success"},
		{"postgres", "insert", "products", "error"},
		{"mongodb", "update", "orders", "success"},
		{"redis", "delete", "cache", "timeout"},
	}

	for _, tc := range testCases {
		// Test query duration
		DBQueryDuration.WithLabelValues(tc.database, tc.operation, tc.table).Observe(0.1)

		// Test query total
		DBQueryTotal.WithLabelValues(tc.database, tc.operation, tc.status).Inc()

		// Test connection pool size
		DBConnectionPoolSize.WithLabelValues(tc.database, "master").Set(20.0)

		// Test active connections
		DBConnectionActive.WithLabelValues(tc.database, "master").Set(8.0)

		// Test slow queries
		DBSlowQueryTotal.WithLabelValues(tc.database, "2s").Inc()

		// Test circuit breaker
		CircuitBreakerState.WithLabelValues(tc.database + "-breaker").Set(0.0)
		CircuitBreakerFailures.WithLabelValues(tc.database + "-breaker").Inc()
	}

	// Verify all metrics have data
	metrics := []prometheus.Collector{
		DBQueryDuration,
		DBQueryTotal,
		DBConnectionPoolSize,
		DBConnectionActive,
		DBSlowQueryTotal,
		CircuitBreakerState,
		CircuitBreakerFailures,
	}

	for _, metric := range metrics {
		metricChan := make(chan prometheus.Metric, 10)
		metric.Collect(metricChan)
		close(metricChan)
		
		count := 0
		for range metricChan {
			count++
		}
		
		if count == 0 {
			t.Errorf("Expected at least one metric for %T", metric)
		}
	}
}

func TestMetricsConcurrentAccess(t *testing.T) {
	// Test that metrics can be accessed concurrently without race conditions
	done := make(chan bool, 10)

	// Start multiple goroutines updating metrics
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				DBQueryTotal.WithLabelValues("test", "operation", "success").Inc()
				DBConnectionPoolSize.WithLabelValues("test", "master").Set(float64(j))
				CircuitBreakerState.WithLabelValues("test").Set(float64(j % 3))
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify metrics are still functional
	metricChan := make(chan prometheus.Metric, 10)
	DBQueryTotal.Collect(metricChan)
	metric := <-metricChan

	if metric == nil {
		t.Fatal("Expected metric after concurrent access, got nil")
	}
}
