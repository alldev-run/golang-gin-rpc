package bootstrap

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
	"github.com/alldev-run/golang-gin-rpc/plugins/health"
	"github.com/alldev-run/golang-gin-rpc/plugins/metrics"
	"github.com/alldev-run/golang-gin-rpc/plugins/tracing"
)

// InitializeErrors initializes error handling
func (b *Bootstrap) InitializeErrors() error {
	b.setDependency(DepErrorsInitialized, true)
	logger.Info("Error handling initialized successfully")
	return nil
}

// InitializeHealth initializes health check services
func (b *Bootstrap) InitializeHealth() error {
	healthManager := health.NewHealthManager()

	// Register default health checkers
	// Database health checker
	if b.db != nil {
		// Create a simple database health checker
		dbChecker := health.NewCustomHealthChecker("database", func(ctx context.Context) *health.CheckResult {
			// Note: In real implementation, you would check actual database connections
			return &health.CheckResult{
				Name:      "database",
				Status:    health.StatusHealthy,
				Message:   "Database connections healthy",
				Timestamp: time.Now(),
			}
		})
		healthManager.RegisterChecker(dbChecker, health.DefaultHealthCheckConfig())
	}

	// Cache health checker
	if b.cache != nil {
		cacheChecker := health.NewCustomHealthChecker("cache", func(ctx context.Context) *health.CheckResult {
			return &health.CheckResult{
				Name:      "cache",
				Status:    health.StatusHealthy,
				Message:   "Cache service healthy",
				Timestamp: time.Now(),
			}
		})
		healthManager.RegisterChecker(cacheChecker, health.DefaultHealthCheckConfig())
	}

	b.healthManager = healthManager
	b.setDependency(DepHealthManager, healthManager)

	logger.Info("Health check services initialized successfully")
	return nil
}

// InitializeMetrics initializes metrics collection
func (b *Bootstrap) InitializeMetrics() error {
	if !b.config.Observability.Metrics.Enabled {
		logger.Info("Metrics collection disabled")
		return nil
	}

	metricsCollector := metrics.NewMetricsCollector()
	b.metricsCollector = metricsCollector
	b.setDependency(DepMetricsCollector, metricsCollector)

	// Start metrics server in background
	go func() {
		endpoint := b.config.Observability.Metrics.Endpoint
		addr := endpoint
		if addr == "" || strings.HasPrefix(addr, "/") {
			// endpoint 是路径或为空时，独立监听地址取 observability.metrics.listen_addr，
			// 未配置则回退 :9090。
			addr = b.config.Observability.Metrics.ListenAddr
			if addr == "" {
				addr = ":9090"
			}
			if strings.HasPrefix(endpoint, "/") {
				logger.Warn("Metrics endpoint is a path, using standalone listen address",
					logger.String("endpoint", endpoint),
					logger.String("listen_addr", addr))
			}
		}
		if err := metricsCollector.StartMetricsServer(addr); err != nil {
			logger.Errorf("Failed to start metrics server", logger.Error(err))
		}
	}()

	logger.Info("Metrics collection initialized successfully",
		logger.String("endpoint", b.config.Observability.Metrics.Endpoint))

	return nil
}

// InitializeTracing initializes tracing services
func (b *Bootstrap) InitializeTracing() error {
	if !b.config.Observability.Tracing.Enabled {
		logger.Info("Tracing services disabled")
		return nil
	}

	tracingConfig := tracing.DefaultConfig()
	tracingConfig.Enabled = b.config.Observability.Tracing.Enabled
	tracingConfig.Type = b.config.Observability.Tracing.Type
	tracingConfig.Endpoint = b.config.Observability.Tracing.Endpoint
	tracingConfig.SampleRate = b.config.Observability.Tracing.SampleRate
	tracingConfig.ServiceName = b.config.App.Name
	tracingConfig.ServiceVersion = b.config.App.Version
	tracingConfig.Environment = b.config.App.Environment

	tracer, err := tracing.NewTracer(tracingConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize tracing: %w", err)
	}

	b.tracer = tracer
	tracing.SetGlobalTracer(tracer)
	b.setDependency(DepTracer, tracer)

	logger.Info("Tracing services initialized successfully")
	return nil
}

// shutdownTracer stops the bootstrap tracer and clears it from the global slot.
func (b *Bootstrap) shutdownTracer(ctx context.Context) error {
	if tracing.GlobalTracer() == b.tracer {
		tracing.SetGlobalTracer(nil)
	}
	return b.tracer.Shutdown(ctx)
}
