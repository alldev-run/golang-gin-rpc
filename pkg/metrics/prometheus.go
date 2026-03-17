// Package metrics provides Prometheus metrics collection with enterprise features
package metrics

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"

	"alldev-gin-rpc/pkg/logger"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// MetricsCollector collects and exposes application metrics with enterprise features
type MetricsCollector struct {
	// HTTP metrics
	httpRequestsTotal   *prometheus.CounterVec
	httpRequestDuration *prometheus.HistogramVec
	httpRequestSize     *prometheus.HistogramVec
	httpResponseSize    *prometheus.HistogramVec

	// RPC metrics
	rpcRequestsTotal   *prometheus.CounterVec
	rpcRequestDuration *prometheus.HistogramVec
	rpcErrorsTotal     *prometheus.CounterVec
	rpcRequestsDetailedTotal   *prometheus.CounterVec
	rpcRequestDetailedDuration *prometheus.HistogramVec
	rpcGovernanceEventsTotal   *prometheus.CounterVec

	// Database metrics
	dbConnectionsActive *prometheus.GaugeVec
	dbConnectionsIdle   *prometheus.GaugeVec
	dbQueryDuration     *prometheus.HistogramVec
	dbErrorsTotal       *prometheus.CounterVec
	dbTransactionCount  *prometheus.CounterVec

	// Cache metrics
	cacheOperationsTotal *prometheus.CounterVec
	cacheHitRatio        *prometheus.GaugeVec
	cacheSize            *prometheus.GaugeVec
	cacheEvictions       *prometheus.CounterVec

	// Service discovery metrics
	discoveryServicesTotal   *prometheus.GaugeVec
	discoveryOperationsTotal *prometheus.CounterVec

	// Application metrics
	activeConnections *prometheus.GaugeVec
	memoryUsage       *prometheus.GaugeVec
	goroutineCount    *prometheus.GaugeVec

	// Business metrics
	businessRevenue     *prometheus.CounterVec
	businessOrdersTotal  *prometheus.CounterVec
	businessUsersActive  *prometheus.GaugeVec

	// Error metrics
	errorTotal          *prometheus.CounterVec
	errorRate           *prometheus.GaugeVec
	panicTotal          *prometheus.CounterVec

	// Security metrics
	authAttemptsTotal   *prometheus.CounterVec
	authFailuresTotal   *prometheus.CounterVec
	rateLimitHitsTotal  *prometheus.CounterVec

	// Custom metrics registry
	customMetrics map[string]prometheus.Metric
	customMetricsMu sync.RWMutex
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	collector := &MetricsCollector{
		customMetrics: make(map[string]prometheus.Metric),
		// HTTP metrics
		httpRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "endpoint", "status_code"},
		),
		httpRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "endpoint"},
		),

		// RPC metrics
		rpcRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rpc_requests_total",
				Help: "Total number of RPC requests",
			},
			[]string{"service", "method", "status"},
		),
		rpcRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "rpc_request_duration_seconds",
				Help:    "RPC request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"service", "method"},
		),
		rpcErrorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rpc_errors_total",
				Help: "Total number of RPC errors",
			},
			[]string{"service", "method", "error_type"},
		),
		rpcRequestsDetailedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rpc_requests_detailed_total",
				Help: "Total number of RPC requests with protocol, target and status labels",
			},
			[]string{"protocol", "service", "method", "target", "status"},
		),
		rpcRequestDetailedDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "rpc_request_detailed_duration_seconds",
				Help:    "RPC request duration in seconds with protocol and target labels",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"protocol", "service", "method", "target", "status"},
		),
		rpcGovernanceEventsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rpc_governance_events_total",
				Help: "Total number of RPC governance events",
			},
			[]string{"protocol", "service", "method", "event"},
		),

		authAttemptsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "auth_attempts_total",
				Help: "Total number of auth attempts",
			},
			[]string{"protocol", "service", "method"},
		),
		authFailuresTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "auth_failures_total",
				Help: "Total number of auth failures",
			},
			[]string{"protocol", "service", "method"},
		),
		rateLimitHitsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rate_limit_hits_total",
				Help: "Total number of rate limit rejections",
			},
			[]string{"protocol", "service", "method"},
		),

		// Database metrics
		dbConnectionsActive: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "db_connections_active",
				Help: "Number of active database connections",
			},
			[]string{"database", "type"},
		),
		dbQueryDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "db_query_duration_seconds",
				Help:    "Database query duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"database", "operation"},
		),
		dbErrorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "db_errors_total",
				Help: "Total number of database errors",
			},
			[]string{"database", "operation", "error_type"},
		),

		// Cache metrics
		cacheOperationsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cache_operations_total",
				Help: "Total number of cache operations",
			},
			[]string{"cache", "operation", "result"},
		),
		cacheHitRatio: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "cache_hit_ratio",
				Help: "Cache hit ratio",
			},
			[]string{"cache"},
		),

		// Service discovery metrics
		discoveryServicesTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "discovery_services_total",
				Help: "Total number of discovered services",
			},
			[]string{"registry", "service"},
		),
		discoveryOperationsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "discovery_operations_total",
				Help: "Total number of discovery operations",
			},
			[]string{"registry", "operation", "result"},
		),

		// Application metrics
		activeConnections: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "active_connections",
				Help: "Number of active connections",
			},
			[]string{"type"},
		),
		memoryUsage: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "memory_usage_bytes",
				Help: "Memory usage in bytes",
			},
			[]string{"type"},
		),
		goroutineCount: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "goroutine_count",
				Help: "Number of goroutines",
			},
			[]string{"type"},
		),
	}

	// Register metrics with Prometheus and reuse already-registered collectors
	collector.httpRequestsTotal = registerCounterVec(collector.httpRequestsTotal)
	collector.httpRequestDuration = registerHistogramVec(collector.httpRequestDuration)
	collector.rpcRequestsTotal = registerCounterVec(collector.rpcRequestsTotal)
	collector.rpcRequestDuration = registerHistogramVec(collector.rpcRequestDuration)
	collector.rpcErrorsTotal = registerCounterVec(collector.rpcErrorsTotal)
	collector.rpcRequestsDetailedTotal = registerCounterVec(collector.rpcRequestsDetailedTotal)
	collector.rpcRequestDetailedDuration = registerHistogramVec(collector.rpcRequestDetailedDuration)
	collector.rpcGovernanceEventsTotal = registerCounterVec(collector.rpcGovernanceEventsTotal)
	collector.dbConnectionsActive = registerGaugeVec(collector.dbConnectionsActive)
	collector.dbQueryDuration = registerHistogramVec(collector.dbQueryDuration)
	collector.dbErrorsTotal = registerCounterVec(collector.dbErrorsTotal)
	collector.cacheOperationsTotal = registerCounterVec(collector.cacheOperationsTotal)
	collector.cacheHitRatio = registerGaugeVec(collector.cacheHitRatio)
	collector.discoveryServicesTotal = registerGaugeVec(collector.discoveryServicesTotal)
	collector.discoveryOperationsTotal = registerCounterVec(collector.discoveryOperationsTotal)
	collector.activeConnections = registerGaugeVec(collector.activeConnections)
	collector.memoryUsage = registerGaugeVec(collector.memoryUsage)
	collector.goroutineCount = registerGaugeVec(collector.goroutineCount)
	collector.authAttemptsTotal = registerCounterVec(collector.authAttemptsTotal)
	collector.authFailuresTotal = registerCounterVec(collector.authFailuresTotal)
	collector.rateLimitHitsTotal = registerCounterVec(collector.rateLimitHitsTotal)

	return collector
}

func registerCollector(collector prometheus.Collector) {
	if err := prometheus.Register(collector); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); ok {
			return
		}
		panic(err)
	}
}

func registerCounterVec(collector *prometheus.CounterVec) *prometheus.CounterVec {
	if err := prometheus.Register(collector); err != nil {
		if existing, ok := err.(prometheus.AlreadyRegisteredError); ok {
			if registered, ok := existing.ExistingCollector.(*prometheus.CounterVec); ok {
				return registered
			}
			panic(err)
		}
		panic(err)
	}
	return collector
}

func registerHistogramVec(collector *prometheus.HistogramVec) *prometheus.HistogramVec {
	if err := prometheus.Register(collector); err != nil {
		if existing, ok := err.(prometheus.AlreadyRegisteredError); ok {
			if registered, ok := existing.ExistingCollector.(*prometheus.HistogramVec); ok {
				return registered
			}
			panic(err)
		}
		panic(err)
	}
	return collector
}

func registerGaugeVec(collector *prometheus.GaugeVec) *prometheus.GaugeVec {
	if err := prometheus.Register(collector); err != nil {
		if existing, ok := err.(prometheus.AlreadyRegisteredError); ok {
			if registered, ok := existing.ExistingCollector.(*prometheus.GaugeVec); ok {
				return registered
			}
			panic(err)
		}
		panic(err)
	}
	return collector
}

// RecordHTTPRequest records an HTTP request
func (m *MetricsCollector) RecordHTTPRequest(method, endpoint, statusCode string, duration time.Duration) {
	m.httpRequestsTotal.WithLabelValues(method, endpoint, statusCode).Inc()
	m.httpRequestDuration.WithLabelValues(method, endpoint).Observe(duration.Seconds())
}

// RecordRPCRequest records an RPC request
func (m *MetricsCollector) RecordRPCRequest(service, method, status string, duration time.Duration) {
	m.rpcRequestsTotal.WithLabelValues(service, method, status).Inc()
	m.rpcRequestDuration.WithLabelValues(service, method).Observe(duration.Seconds())
}

// RecordRPCError records an RPC error
func (m *MetricsCollector) RecordRPCError(service, method, errorType string) {
	m.rpcErrorsTotal.WithLabelValues(service, method, errorType).Inc()
}

func (m *MetricsCollector) RecordRPCRequestDetailed(protocol, service, method, target, status string, duration time.Duration) {
	m.rpcRequestsDetailedTotal.WithLabelValues(protocol, service, method, target, status).Inc()
	m.rpcRequestDetailedDuration.WithLabelValues(protocol, service, method, target, status).Observe(duration.Seconds())
}

func (m *MetricsCollector) RecordRPCGovernanceEvent(protocol, service, method, event string) {
	m.rpcGovernanceEventsTotal.WithLabelValues(protocol, service, method, event).Inc()
}

func (m *MetricsCollector) RecordAuthAttempt(protocol, service, method string) {
	m.authAttemptsTotal.WithLabelValues(protocol, service, method).Inc()
}

func (m *MetricsCollector) RecordAuthFailure(protocol, service, method string) {
	m.authFailuresTotal.WithLabelValues(protocol, service, method).Inc()
}

func (m *MetricsCollector) RecordRateLimitHit(protocol, service, method string) {
	m.rateLimitHitsTotal.WithLabelValues(protocol, service, method).Inc()
}

// RecordDBConnection records database connection
func (m *MetricsCollector) RecordDBConnection(database, dbType string, count float64) {
	m.dbConnectionsActive.WithLabelValues(database, dbType).Set(count)
}

// RecordDBQuery records database query
func (m *MetricsCollector) RecordDBQuery(database, operation string, duration time.Duration) {
	m.dbQueryDuration.WithLabelValues(database, operation).Observe(duration.Seconds())
}

// RecordDBError records database error
func (m *MetricsCollector) RecordDBError(database, operation, errorType string) {
	m.dbErrorsTotal.WithLabelValues(database, operation, errorType).Inc()
}

// RecordCacheOperation records cache operation
func (m *MetricsCollector) RecordCacheOperation(cache, operation, result string) {
	m.cacheOperationsTotal.WithLabelValues(cache, operation, result).Inc()
}

// UpdateCacheHitRatio updates cache hit ratio
func (m *MetricsCollector) UpdateCacheHitRatio(cache string, ratio float64) {
	m.cacheHitRatio.WithLabelValues(cache).Set(ratio)
}

// UpdateDiscoveryServices updates discovered services count
func (m *MetricsCollector) UpdateDiscoveryServices(registry, service string, count float64) {
	m.discoveryServicesTotal.WithLabelValues(registry, service).Set(count)
}

// RecordDiscoveryOperation records discovery operation
func (m *MetricsCollector) RecordDiscoveryOperation(registry, operation, result string) {
	m.discoveryOperationsTotal.WithLabelValues(registry, operation, result).Inc()
}

// UpdateActiveConnections updates active connections
func (m *MetricsCollector) UpdateActiveConnections(connType string, count float64) {
	m.activeConnections.WithLabelValues(connType).Set(count)
}

// UpdateMemoryUsage updates memory usage
func (m *MetricsCollector) UpdateMemoryUsage(memType string, bytes float64) {
	m.memoryUsage.WithLabelValues(memType).Set(bytes)
}

// UpdateGoroutineCount updates goroutine count
func (m *MetricsCollector) UpdateGoroutineCount(goroutineType string, count float64) {
	m.goroutineCount.WithLabelValues(goroutineType).Set(count)
}

// StartMetricsServer starts the Prometheus metrics server
func (m *MetricsCollector) StartMetricsServer(addr string) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	logger.Info("Starting metrics server", zap.String("address", addr))
	return http.ListenAndServe(addr, mux)
}

// StartMetricsServerWithContext starts the metrics server with context
func (m *MetricsCollector) StartMetricsServerWithContext(ctx context.Context, addr string) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	logger.Info("Starting metrics server", zap.String("address", addr))

	go func() {
		<-ctx.Done()
		logger.Info("Shutting down metrics server")
		server.Shutdown(ctx)
	}()

	return server.ListenAndServe()
}

// MetricsMiddleware provides HTTP metrics middleware
func (m *MetricsCollector) MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a response writer wrapper to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: 200}

		next.ServeHTTP(wrapped, r)

		method := ""
		path := ""
		if r != nil {
			method = r.Method
			if r.URL != nil {
				path = r.URL.Path
			}
		}

		duration := time.Since(start)
		m.RecordHTTPRequest(
			method,
			path,
			fmt.Sprintf("%d", wrapped.statusCode),
			duration,
		)
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// SystemMetricsCollector collects system-level metrics
type SystemMetricsCollector struct {
	collector *MetricsCollector
	interval  time.Duration
	stopCh    chan struct{}
}

// NewSystemMetricsCollector creates a new system metrics collector
func NewSystemMetricsCollector(collector *MetricsCollector, interval time.Duration) *SystemMetricsCollector {
	return &SystemMetricsCollector{
		collector: collector,
		interval:  interval,
		stopCh:    make(chan struct{}),
	}
}

// Start starts collecting system metrics
func (s *SystemMetricsCollector) Start() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.collectSystemMetrics()
		}
	}
}

// Stop stops collecting system metrics
func (s *SystemMetricsCollector) Stop() {
	close(s.stopCh)
}

// collectSystemMetrics collects system metrics
func (s *SystemMetricsCollector) collectSystemMetrics() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Update memory metrics
	s.collector.UpdateMemoryUsage("heap", float64(m.HeapAlloc))
	s.collector.UpdateMemoryUsage("stack", float64(m.StackInuse))
	s.collector.UpdateMemoryUsage("gc", float64(m.GCSys))

	// Update goroutine count
	s.collector.UpdateGoroutineCount("total", float64(runtime.NumGoroutine()))
}

// DefaultMetricsExporter provides default metrics export functionality
type DefaultMetricsExporter struct {
	collector       *MetricsCollector
	systemCollector *SystemMetricsCollector
}

// NewDefaultMetricsExporter creates a new default metrics exporter
func NewDefaultMetricsExporter() *DefaultMetricsExporter {
	collector := NewMetricsCollector()
	systemCollector := NewSystemMetricsCollector(collector, 30*time.Second)

	return &DefaultMetricsExporter{
		collector:       collector,
		systemCollector: systemCollector,
	}
}

// Start starts the metrics exporter
func (e *DefaultMetricsExporter) Start(addr string) error {
	// Start system metrics collection
	go e.systemCollector.Start()

	// Start metrics server
	return e.collector.StartMetricsServer(addr)
}

// StartWithContext starts the metrics exporter with context
func (e *DefaultMetricsExporter) StartWithContext(ctx context.Context, addr string) error {
	// Start system metrics collection
	go e.systemCollector.Start()

	// Start metrics server
	return e.collector.StartMetricsServerWithContext(ctx, addr)
}

// Stop stops the metrics exporter
func (e *DefaultMetricsExporter) Stop() {
	e.systemCollector.Stop()
}

// GetCollector returns the metrics collector
func (e *DefaultMetricsExporter) GetCollector() *MetricsCollector {
	return e.collector
}
