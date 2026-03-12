// Package metrics provides Prometheus metrics collection for database operations.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// DBQueryDuration tracks query execution time
	DBQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "Database query duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"database", "operation", "table"},
	)

	// DBQueryTotal tracks total queries
	DBQueryTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_query_total",
			Help: "Total number of database queries",
		},
		[]string{"database", "operation", "status"},
	)

	// DBConnectionPoolSize tracks connection pool size
	DBConnectionPoolSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "db_connection_pool_size",
			Help: "Current connection pool size",
		},
		[]string{"database", "type"}, // type: master/replica
	)

	// DBConnectionActive tracks active connections
	DBConnectionActive = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "db_connection_active",
			Help: "Number of active connections",
		},
		[]string{"database", "type"},
	)

	// DBSlowQueryTotal tracks slow queries
	DBSlowQueryTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_slow_query_total",
			Help: "Total number of slow queries",
		},
		[]string{"database", "threshold"},
	)

	// CircuitBreakerState tracks circuit breaker state
	CircuitBreakerState = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "circuit_breaker_state",
			Help: "Circuit breaker state (0=closed, 1=open, 2=half-open)",
		},
		[]string{"name"},
	)

	// CircuitBreakerFailures tracks circuit breaker failures
	CircuitBreakerFailures = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "circuit_breaker_failures_total",
			Help: "Total number of circuit breaker failures",
		},
		[]string{"name"},
	)
)
