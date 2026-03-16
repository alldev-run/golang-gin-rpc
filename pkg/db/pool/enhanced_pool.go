package pool

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"golang-gin-rpc/pkg/logger"
	"golang-gin-rpc/pkg/metrics"

	"go.uber.org/zap"
)

// PoolConfig holds database connection pool configuration
type PoolConfig struct {
	// Basic connection settings
	MaxOpenConns    int           `yaml:"max_open_conns" json:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns" json:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" json:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `yaml:"conn_max_idle_time" json:"conn_max_idle_time"`
	
	// Advanced settings
	MinOpenConns     int           `yaml:"min_open_conns" json:"min_open_conns"`
	ConnectTimeout   time.Duration `yaml:"connect_timeout" json:"connect_timeout"`
	QueryTimeout     time.Duration `yaml:"query_timeout" json:"query_timeout"`
	
	// Health check settings
	HealthCheckPeriod    time.Duration `yaml:"health_check_period" json:"health_check_period"`
	HealthCheckTimeout    time.Duration `yaml:"health_check_timeout" json:"health_check_timeout"`
	HealthCheckQuery      string        `yaml:"health_check_query" json:"health_check_query"`
	
	// Retry settings
	MaxRetries      int           `yaml:"max_retries" json:"max_retries"`
	RetryDelay      time.Duration `yaml:"retry_delay" json:"retry_delay"`
	
	// Metrics settings
	EnableMetrics    bool `yaml:"enable_metrics" json:"enable_metrics"`
	MetricsInterval  time.Duration `yaml:"metrics_interval" json:"metrics_interval"`
}

// DefaultPoolConfig returns default pool configuration
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxOpenConns:         25,
		MaxIdleConns:         10,
		ConnMaxLifetime:      30 * time.Minute,
		ConnMaxIdleTime:      5 * time.Minute,
		MinOpenConns:         5,
		ConnectTimeout:       10 * time.Second,
		QueryTimeout:         30 * time.Second,
		HealthCheckPeriod:    1 * time.Minute,
		HealthCheckTimeout:   5 * time.Second,
		HealthCheckQuery:     "SELECT 1",
		MaxRetries:           3,
		RetryDelay:           1 * time.Second,
		EnableMetrics:        true,
		MetricsInterval:      30 * time.Second,
	}
}

// ProductionPoolConfig returns production-optimized pool configuration
func ProductionPoolConfig() PoolConfig {
	return PoolConfig{
		MaxOpenConns:         50,
		MaxIdleConns:         25,
		ConnMaxLifetime:      1 * time.Hour,
		ConnMaxIdleTime:      10 * time.Minute,
		MinOpenConns:         10,
		ConnectTimeout:       5 * time.Second,
		QueryTimeout:         10 * time.Second,
		HealthCheckPeriod:    30 * time.Second,
		HealthCheckTimeout:   3 * time.Second,
		HealthCheckQuery:     "SELECT 1",
		MaxRetries:           5,
		RetryDelay:           500 * time.Millisecond,
		EnableMetrics:        true,
		MetricsInterval:      15 * time.Second,
	}
}

// DevelopmentPoolConfig returns development-optimized pool configuration
func DevelopmentPoolConfig() PoolConfig {
	return PoolConfig{
		MaxOpenConns:         10,
		MaxIdleConns:         5,
		ConnMaxLifetime:      10 * time.Minute,
		ConnMaxIdleTime:      2 * time.Minute,
		MinOpenConns:         2,
		ConnectTimeout:       30 * time.Second,
		QueryTimeout:         60 * time.Second,
		HealthCheckPeriod:    2 * time.Minute,
		HealthCheckTimeout:   10 * time.Second,
		HealthCheckQuery:     "SELECT 1",
		MaxRetries:           2,
		RetryDelay:           2 * time.Second,
		EnableMetrics:        true,
		MetricsInterval:      1 * time.Minute,
	}
}

// EnhancedPool wraps sql.DB with enterprise features
type EnhancedPool struct {
	db     *sql.DB
	config PoolConfig
	mu     sync.RWMutex
	
	// Health monitoring
	lastHealthCheck time.Time
	isHealthy       bool
	
	// Metrics
	metricsCollector *metrics.MetricsCollector
	metricsTicker    *time.Ticker
	stopCh           chan struct{}
	
	// Connection tracking
	activeConnections int64
	idleConnections   int64
}

// NewEnhancedPool creates a new enhanced database pool
func NewEnhancedPool(db *sql.DB, config PoolConfig) (*EnhancedPool, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}
	
	pool := &EnhancedPool{
		db:     db,
		config: config,
		stopCh: make(chan struct{}),
	}
	
	// Apply pool configuration
	if err := pool.applyConfig(); err != nil {
		return nil, fmt.Errorf("failed to apply pool config: %w", err)
	}
	
	// Start background tasks
	if config.EnableMetrics {
		pool.metricsCollector = metrics.NewMetricsCollector()
		pool.startMetricsCollection()
	}
	
	pool.startHealthCheck()
	
	logger.Info("Database pool initialized",
		zap.Int("max_open_conns", config.MaxOpenConns),
		zap.Int("max_idle_conns", config.MaxIdleConns),
		zap.Duration("conn_max_lifetime", config.ConnMaxLifetime),
	)
	
	return pool, nil
}

// applyConfig applies the configuration to the database pool
func (p *EnhancedPool) applyConfig() error {
	// Set connection pool limits
	p.db.SetMaxOpenConns(p.config.MaxOpenConns)
	p.db.SetMaxIdleConns(p.config.MaxIdleConns)
	p.db.SetConnMaxLifetime(p.config.ConnMaxLifetime)
	p.db.SetConnMaxIdleTime(p.config.ConnMaxIdleTime)
	
	return nil
}

// GetDB returns the underlying sql.DB instance
func (p *EnhancedPool) GetDB() *sql.DB {
	return p.db
}

// Query executes a query with timeout and retry logic
func (p *EnhancedPool) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	ctx, cancel := context.WithTimeout(ctx, p.config.QueryTimeout)
	defer cancel()
	
	var rows *sql.Rows
	var err error
	
	for attempt := 0; attempt <= p.config.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(p.config.RetryDelay):
				logger.Warn("Retrying database query",
					zap.Int("attempt", attempt),
					zap.String("query", query))
			}
		}
		
		rows, err = p.db.QueryContext(ctx, query, args...)
		if err == nil {
			p.recordQuery("query", "success")
			return rows, nil
		}
		
		// Don't retry on certain errors
		if !isRetryableError(err) {
			p.recordQuery("query", "error")
			return nil, err
		}
		
		p.recordQuery("query", "retry")
	}
	
	p.recordQuery("query", "failed")
	return nil, fmt.Errorf("query failed after %d attempts: %w", p.config.MaxRetries+1, err)
}

// QueryRow executes a query that returns a single row
func (p *EnhancedPool) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	ctx, cancel := context.WithTimeout(ctx, p.config.QueryTimeout)
	defer cancel()
	
	return p.db.QueryRowContext(ctx, query, args...)
}

// Exec executes a query that doesn't return rows
func (p *EnhancedPool) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	ctx, cancel := context.WithTimeout(ctx, p.config.QueryTimeout)
	defer cancel()
	
	var result sql.Result
	var err error
	
	for attempt := 0; attempt <= p.config.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(p.config.RetryDelay):
				logger.Warn("Retrying database exec",
					zap.Int("attempt", attempt),
					zap.String("query", query))
			}
		}
		
		result, err = p.db.ExecContext(ctx, query, args...)
		if err == nil {
			p.recordQuery("exec", "success")
			return result, nil
		}
		
		// Don't retry on certain errors
		if !isRetryableError(err) {
			p.recordQuery("exec", "error")
			return nil, err
		}
		
		p.recordQuery("exec", "retry")
	}
	
	p.recordQuery("exec", "failed")
	return nil, fmt.Errorf("exec failed after %d attempts: %w", p.config.MaxRetries+1, err)
}

// Begin begins a transaction
func (p *EnhancedPool) Begin(ctx context.Context) (*sql.Tx, error) {
	ctx, cancel := context.WithTimeout(ctx, p.config.QueryTimeout)
	defer cancel()
	
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		p.recordQuery("begin", "error")
		return nil, err
	}
	
	p.recordQuery("begin", "success")
	return tx, nil
}

// Transaction executes a function within a transaction
func (p *EnhancedPool) Transaction(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := p.Begin(ctx)
	if err != nil {
		return err
	}
	
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p) // re-throw panic after rollback
		}
	}()
	
	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			logger.Error("Failed to rollback transaction",
				zap.Error(err),
				zap.Error(rbErr))
		}
		return err
	}
	
	return tx.Commit()
}

// Ping checks if the database is reachable
func (p *EnhancedPool) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, p.config.HealthCheckTimeout)
	defer cancel()
	
	return p.db.PingContext(ctx)
}

// Stats returns database statistics
func (p *EnhancedPool) Stats() sql.DBStats {
	return p.db.Stats()
}

// Close closes the database connection pool
func (p *EnhancedPool) Close() error {
	close(p.stopCh)
	
	if p.metricsTicker != nil {
		p.metricsTicker.Stop()
	}
	
	return p.db.Close()
}

// IsHealthy returns the current health status
func (p *EnhancedPool) IsHealthy() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.isHealthy
}

// startHealthCheck starts the health check routine
func (p *EnhancedPool) startHealthCheck() {
	go func() {
		ticker := time.NewTicker(p.config.HealthCheckPeriod)
		defer ticker.Stop()
		
		for {
			select {
			case <-p.stopCh:
				return
			case <-ticker.C:
				p.performHealthCheck()
			}
		}
	}()
}

// performHealthCheck performs a health check
func (p *EnhancedPool) performHealthCheck() {
	ctx, cancel := context.WithTimeout(context.Background(), p.config.HealthCheckTimeout)
	defer cancel()
	
	err := p.db.PingContext(ctx)
	
	p.mu.Lock()
	p.lastHealthCheck = time.Now()
	p.isHealthy = err == nil
	p.mu.Unlock()
	
	if err != nil {
		logger.Error("Database health check failed",
			zap.Error(err),
			zap.Time("last_check", p.lastHealthCheck))
	} else {
		logger.Debug("Database health check passed",
			zap.Time("last_check", p.lastHealthCheck))
	}
}

// startMetricsCollection starts collecting metrics
func (p *EnhancedPool) startMetricsCollection() {
	p.metricsTicker = time.NewTicker(p.config.MetricsInterval)
	
	go func() {
		for {
			select {
			case <-p.stopCh:
				return
			case <-p.metricsTicker.C:
				p.collectMetrics()
			}
		}
	}()
}

// collectMetrics collects pool metrics
func (p *EnhancedPool) collectMetrics() {
	stats := p.db.Stats()
	
	if p.metricsCollector != nil {
		p.metricsCollector.RecordDBConnection("primary", "sql", float64(stats.OpenConnections))
		p.metricsCollector.UpdateActiveConnections("database", float64(stats.OpenConnections))
	}
}

// recordQuery records query metrics
func (p *EnhancedPool) recordQuery(operation, status string) {
	if p.metricsCollector != nil {
		p.metricsCollector.RecordDBQuery("primary", operation, 0)
		if status == "error" || status == "failed" {
			p.metricsCollector.RecordDBError("primary", operation, "query_error")
		}
	}
}

// isRetryableError checks if an error is retryable
func isRetryableError(err error) bool {
	// Add logic to determine if an error is retryable
	// For now, return true for most errors
	return true
}

// PoolManager manages multiple database pools
type PoolManager struct {
	pools map[string]*EnhancedPool
	mu    sync.RWMutex
}

// NewPoolManager creates a new pool manager
func NewPoolManager() *PoolManager {
	return &PoolManager{
		pools: make(map[string]*EnhancedPool),
	}
}

// AddPool adds a database pool
func (pm *PoolManager) AddPool(name string, db *sql.DB, config PoolConfig) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	pool, err := NewEnhancedPool(db, config)
	if err != nil {
		return fmt.Errorf("failed to create pool %s: %w", name, err)
	}
	
	pm.pools[name] = pool
	return nil
}

// GetPool gets a database pool by name
func (pm *PoolManager) GetPool(name string) (*EnhancedPool, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	pool, exists := pm.pools[name]
	return pool, exists
}

// Close closes all pools
func (pm *PoolManager) Close() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	var errors []error
	for name, pool := range pm.pools {
		if err := pool.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close pool %s: %w", name, err))
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("errors closing pools: %v", errors)
	}
	
	return nil
}

// GetStats returns statistics for all pools
func (pm *PoolManager) GetStats() map[string]sql.DBStats {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	stats := make(map[string]sql.DBStats)
	for name, pool := range pm.pools {
		stats[name] = pool.Stats()
	}
	
	return stats
}
