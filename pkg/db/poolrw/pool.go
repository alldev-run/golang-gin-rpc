// Package poolrw integrates connection pooling with read-write splitting.
// It provides a high-level database client that combines connection pooling,
// automatic read-write routing, and load balancing.
package poolrw

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"golang-gin-rpc/pkg/db"
	"golang-gin-rpc/pkg/db/rwproxy"
)

// RWPoolConfig holds configuration for the integrated pool.
type RWPoolConfig struct {
	MasterConfig   db.Config         // Master database config
	ReplicaConfigs []db.Config       // Replica database configs
	Strategy       rwproxy.LBStrategy // Load balancing strategy
	PoolConfig     PoolSettings      // Pool settings
}

// PoolSettings holds pool-specific settings.
type PoolSettings struct {
	MaxIdleTime       time.Duration // Max time a connection can be idle
	HealthCheckPeriod time.Duration // How often to check health
	MaxFailures       int32         // Max consecutive failures before marking bad
	AcquireTimeout    time.Duration // Max time to wait for a connection
}

// DefaultRWPoolConfig returns default configuration.
func DefaultRWPoolConfig() RWPoolConfig {
	return RWPoolConfig{
		Strategy: rwproxy.LBStrategyRoundRobin,
		PoolConfig: PoolSettings{
			MaxIdleTime:       30 * time.Minute,
			HealthCheckPeriod: 30 * time.Second,
			MaxFailures:       3,
			AcquireTimeout:    5 * time.Second,
		},
	}
}

// Pool manages a pool of read-write splitting clients.
type Pool struct {
	config        RWPoolConfig
	factory       *db.Factory
	master        db.Client         // Master connection
	replicas      []db.Client       // Replica connections
	rwClient      *rwproxy.Client   // Read-write split client wrapper
	mutex         sync.RWMutex
	healthStop    chan struct{}
	wg            sync.WaitGroup
	state         int32 // Pool state
}

// Pool state constants.
const (
	StateRunning int32 = iota
	StateClosing
	StateClosed
)

// New creates a new integrated pool with read-write splitting.
func New(config RWPoolConfig, factory *db.Factory) (*Pool, error) {
	if factory == nil {
		factory = db.NewFactory()
	}

	p := &Pool{
		config:     config,
		factory:    factory,
		replicas:   make([]db.Client, 0, len(config.ReplicaConfigs)),
		healthStop: make(chan struct{}),
		state:      StateRunning,
	}

	// Initialize master connection
	if err := p.initMaster(); err != nil {
		return nil, fmt.Errorf("failed to initialize master: %w", err)
	}

	// Initialize replica connections
	if err := p.initReplicas(); err != nil {
		_ = p.Close()
		return nil, fmt.Errorf("failed to initialize replicas: %w", err)
	}

	// Create read-write split client
	if err := p.initRWClient(); err != nil {
		_ = p.Close()
		return nil, fmt.Errorf("failed to initialize rw client: %w", err)
	}

	// Start health checker
	p.wg.Add(1)
	go p.healthChecker()

	return p, nil
}

// initMaster creates the master connection.
func (p *Pool) initMaster() error {
	client, err := p.factory.Create(p.config.MasterConfig)
	if err != nil {
		return err
	}
	p.master = client
	return nil
}

// initReplicas creates replica connections.
func (p *Pool) initReplicas() error {
	for i, cfg := range p.config.ReplicaConfigs {
		client, err := p.factory.Create(cfg)
		if err != nil {
			return fmt.Errorf("failed to create replica %d: %w", i, err)
		}
		p.replicas = append(p.replicas, client)
	}
	return nil
}

// initRWClient creates the read-write split client.
func (p *Pool) initRWClient() error {
	// Get SQL DB interfaces
	masterDB, err := p.getSQLDB(p.master)
	if err != nil {
		return fmt.Errorf("master is not SQL client: %w", err)
	}

	replicaDBs := make([]*sql.DB, 0, len(p.replicas))
	for i, replica := range p.replicas {
		db, err := p.getSQLDB(replica)
		if err != nil {
			return fmt.Errorf("replica %d is not SQL client: %w", i, err)
		}
		replicaDBs = append(replicaDBs, db)
	}

	// Create rwproxy client
	rwConfig := rwproxy.Config{
		Master:   masterDB,
		Replicas: replicaDBs,
		Strategy: p.config.Strategy,
	}
	p.rwClient = rwproxy.New(rwConfig)

	return nil
}

// getSQLDB extracts *sql.DB from db.Client if it's a SQLClient.
func (p *Pool) getSQLDB(client db.Client) (*sql.DB, error) {
	sqlClient, ok := client.(db.SQLClient)
	if !ok {
		return nil, fmt.Errorf("client does not implement SQLClient")
	}
	return sqlClient.DB(), nil
}

// Close shuts down the pool.
func (p *Pool) Close() error {
	// Mark as closing
	// (In a real implementation, use atomic)

	// Stop health checker
	close(p.healthStop)
	p.wg.Wait()

	p.mutex.Lock()
	defer p.mutex.Unlock()

	var errs []error

	// Close rwproxy client
	if p.rwClient != nil {
		if err := p.rwClient.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close rw client: %w", err))
		}
	}

	// Close master
	if p.master != nil {
		if err := p.master.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close master: %w", err))
		}
	}

	// Close replicas
	for i, replica := range p.replicas {
		if err := replica.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close replica %d: %w", i, err))
		}
	}

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// Query executes a read query on a replica.
func (p *Pool) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if p.rwClient == nil {
		return nil, fmt.Errorf("pool not initialized")
	}

	return p.rwClient.Query(ctx, query, args...)
}

// QueryRow executes a read query on a replica, returning a single row.
func (p *Pool) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if p.rwClient == nil {
		return nil
	}

	return p.rwClient.QueryRow(ctx, query, args...)
}

// Exec executes a write query on the master.
func (p *Pool) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if p.rwClient == nil {
		return nil, fmt.Errorf("pool not initialized")
	}

	return p.rwClient.Exec(ctx, query, args...)
}

// Begin starts a transaction on the master.
func (p *Pool) Begin(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if p.rwClient == nil {
		return nil, fmt.Errorf("pool not initialized")
	}

	return p.rwClient.Begin(ctx, opts)
}

// Transaction executes a function within a transaction on the master.
func (p *Pool) Transaction(ctx context.Context, fn func(*sql.Tx) error) error {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if p.rwClient == nil {
		return fmt.Errorf("pool not initialized")
	}

	return p.rwClient.Transaction(ctx, fn)
}

// Prepare creates a prepared statement on the master.
func (p *Pool) Prepare(ctx context.Context, query string) (*sql.Stmt, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if p.rwClient == nil {
		return nil, fmt.Errorf("pool not initialized")
	}

	return p.rwClient.Prepare(ctx, query)
}

// Ping checks the health of all connections.
func (p *Pool) Ping(ctx context.Context) error {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	// Ping master
	if p.master != nil {
		if err := p.master.Ping(ctx); err != nil {
			return fmt.Errorf("master ping failed: %w", err)
		}
	}

	// Ping replicas
	for i, replica := range p.replicas {
		if err := replica.Ping(ctx); err != nil {
			return fmt.Errorf("replica %d ping failed: %w", i, err)
		}
	}

	return nil
}

// ForceMaster forces all queries to use the master.
func (p *Pool) ForceMaster(force bool) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if p.rwClient != nil {
		p.rwClient.ForceMaster(force)
	}
}

// IsMasterForced returns whether master is being forced.
func (p *Pool) IsMasterForced() bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if p.rwClient != nil {
		return p.rwClient.IsMasterForced()
	}
	return false
}

// GetStats returns pool statistics.
func (p *Pool) GetStats() PoolStats {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	stats := PoolStats{
		MasterConnected:  p.master != nil,
		ReplicaCount:     len(p.replicas),
		ReplicasHealthy:  0,
		ForceMaster:      p.IsMasterForced(),
	}

	if p.rwClient != nil {
		stats.ReplicaCount = p.rwClient.GetReplicaCount()
	}

	return stats
}

// PoolStats holds pool statistics.
type PoolStats struct {
	MasterConnected bool `json:"master_connected"`
	ReplicaCount    int  `json:"replica_count"`
	ReplicasHealthy int  `json:"replicas_healthy"`
	ForceMaster     bool `json:"force_master"`
}

// healthChecker periodically checks connection health.
func (p *Pool) healthChecker() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.config.PoolConfig.HealthCheckPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-p.healthStop:
			return
		case <-ticker.C:
			p.checkHealth()
		}
	}
}

// checkHealth checks all connections.
func (p *Pool) checkHealth() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Ping master
	if p.master != nil {
		if err := p.master.Ping(ctx); err != nil {
			// Master failed - could trigger alert or failover logic
			_ = err
		}
	}

	// Ping replicas
	for _, replica := range p.replicas {
		if replica != nil {
			_ = replica.Ping(ctx)
		}
	}
}
