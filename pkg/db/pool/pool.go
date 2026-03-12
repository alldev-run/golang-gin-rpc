// Package pool provides a thread-safe connection pool for database clients
// with health checking, automatic reconnection, and lifecycle management.
package pool

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"golang-gin-rpc/pkg/db"
)

// State represents the connection state.
type State int32

const (
	StateDisconnected State = iota
	StateConnecting
	StateConnected
	StateFailed
)

// PooledClient wraps a db.Client with pool metadata.
type PooledClient struct {
	client    db.Client
	config    db.Config
	state     int32 // atomic State
	lastUsed  int64 // atomic unix timestamp
	createdAt int64 // atomic unix timestamp
	useCount  int64 // atomic usage counter
	failCount int32 // atomic consecutive failure counter
	id        string
}

// IsConnected returns true if the client is connected.
func (p *PooledClient) IsConnected() bool {
	return State(atomic.LoadInt32(&p.state)) == StateConnected
}

// markUsed updates last used time and increments use count (thread-safe).
func (p *PooledClient) markUsed() {
	atomic.StoreInt64(&p.lastUsed, time.Now().Unix())
	atomic.AddInt64(&p.useCount, 1)
	atomic.StoreInt32(&p.failCount, 0)
}

// markFailed increments failure count (thread-safe).
func (p *PooledClient) markFailed() {
	atomic.AddInt32(&p.failCount, 1)
}

// GetStats returns usage statistics (thread-safe).
func (p *PooledClient) GetStats() ClientStats {
	return ClientStats{
		ID:        p.id,
		State:     State(atomic.LoadInt32(&p.state)),
		LastUsed:  time.Unix(atomic.LoadInt64(&p.lastUsed), 0),
		CreatedAt: time.Unix(atomic.LoadInt64(&p.createdAt), 0),
		UseCount:  atomic.LoadInt64(&p.useCount),
		FailCount: atomic.LoadInt32(&p.failCount),
	}
}

// ClientStats holds statistics for a pooled client.
type ClientStats struct {
	ID        string    `json:"id"`
	State     State     `json:"state"`
	LastUsed  time.Time `json:"last_used"`
	CreatedAt time.Time `json:"created_at"`
	UseCount  int64     `json:"use_count"`
	FailCount int32     `json:"fail_count"`
}

// Config holds connection pool configuration.
type Config struct {
	MaxSize           int           // Maximum number of connections
	InitialSize       int           // Initial connections to create
	MaxIdleTime       time.Duration // Max time a connection can be idle
	HealthCheckPeriod time.Duration // How often to check health
	MaxFailures       int32         // Max consecutive failures before marking bad
	AcquireTimeout    time.Duration // Max time to wait for a connection
	RetryDelay        time.Duration // Delay between reconnection attempts
}

// DefaultConfig returns default pool configuration.
func DefaultConfig() Config {
	return Config{
		MaxSize:           10,
		InitialSize:       2,
		MaxIdleTime:       30 * time.Minute,
		HealthCheckPeriod: 30 * time.Second,
		MaxFailures:       3,
		AcquireTimeout:    5 * time.Second,
		RetryDelay:        1 * time.Second,
	}
}

// Pool manages a pool of database connections.
type Pool struct {
	config     Config
	factory    *db.Factory
	clients    map[string]*PooledClient // key: connection name
	mutex      sync.RWMutex
	sem        chan struct{} // Semaphore for limiting concurrent creates
	healthStop chan struct{}
	wg         sync.WaitGroup
}

// New creates a new connection pool.
func New(config Config, factory *db.Factory) *Pool {
	if factory == nil {
		factory = db.NewFactory()
	}

	p := &Pool{
		config:     config,
		factory:    factory,
		clients:    make(map[string]*PooledClient),
		sem:        make(chan struct{}, config.MaxSize),
		healthStop: make(chan struct{}),
	}

	// Start health checker
	p.wg.Add(1)
	go p.healthChecker()

	return p
}

// Close shuts down the pool and closes all connections.
func (p *Pool) Close() error {
	// Stop health checker
	close(p.healthStop)
	p.wg.Wait()

	// Close all clients
	p.mutex.Lock()
	defer p.mutex.Unlock()

	var errs []error
	for name, pc := range p.clients {
		if pc != nil && pc.client != nil {
			if err := pc.client.Close(); err != nil {
				errs = append(errs, fmt.Errorf("failed to close %s: %w", name, err))
			}
		}
	}

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// Acquire gets a client from the pool (thread-safe).
func (p *Pool) Acquire(ctx context.Context, name string) (db.Client, error) {
	// Try to get existing client first
	p.mutex.RLock()
	pc, exists := p.clients[name]
	p.mutex.RUnlock()

	if exists && pc.IsConnected() {
		pc.markUsed()
		return pc.client, nil
	}

	// Need to create or reconnect
	return p.acquireOrCreate(ctx, name)
}

// acquireOrCreate creates a new connection or waits for one (thread-safe).
func (p *Pool) acquireOrCreate(ctx context.Context, name string) (db.Client, error) {
	// Try to acquire semaphore with timeout
	acquireCtx, cancel := context.WithTimeout(ctx, p.config.AcquireTimeout)
	defer cancel()

	select {
	case p.sem <- struct{}{}:
		// Acquired slot, can create connection
		defer func() { <-p.sem }()
	case <-acquireCtx.Done():
		return nil, fmt.Errorf("timeout waiting for connection slot: %w", acquireCtx.Err())
	}

	// Double-check after acquiring semaphore
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if pc, exists := p.clients[name]; exists && pc.IsConnected() {
		pc.markUsed()
		return pc.client, nil
	}

	// Create new connection
	return p.createClientLocked(name)
}

// createClientLocked creates a new client (must hold write lock).
func (p *Pool) createClientLocked(name string) (db.Client, error) {
	// Get config for this connection
	// For simplicity, using default config - in real usage,
	// you would pass config when registering connections
	cfg := db.Config{
		Type: db.TypeMySQL, // Default
	}

	client, err := p.factory.Create(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	now := time.Now().Unix()
	pc := &PooledClient{
		client:    client,
		config:    cfg,
		id:        name,
		state:     int32(StateConnected),
		lastUsed:  now,
		createdAt: now,
	}

	p.clients[name] = pc
	return client, nil
}

// Register adds a new connection configuration to the pool.
func (p *Pool) Register(name string, cfg db.Config) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if _, exists := p.clients[name]; exists {
		return fmt.Errorf("connection %s already registered", name)
	}

	// Create initial connection
	client, err := p.factory.Create(cfg)
	if err != nil {
		return fmt.Errorf("failed to create initial connection: %w", err)
	}

	now := time.Now().Unix()
	p.clients[name] = &PooledClient{
		client:    client,
		config:    cfg,
		id:        name,
		state:     int32(StateConnected),
		lastUsed:  now,
		createdAt: now,
	}

	return nil
}

// Unregister removes a connection from the pool.
func (p *Pool) Unregister(name string) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	pc, exists := p.clients[name]
	if !exists {
		return fmt.Errorf("connection %s not found", name)
	}

	if err := pc.client.Close(); err != nil {
		return fmt.Errorf("failed to close client: %w", err)
	}

	delete(p.clients, name)
	return nil
}

// GetStats returns statistics for all pooled connections (thread-safe).
func (p *Pool) GetStats() map[string]ClientStats {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	stats := make(map[string]ClientStats, len(p.clients))
	for name, pc := range p.clients {
		stats[name] = pc.GetStats()
	}
	return stats
}

// healthChecker periodically checks connection health.
func (p *Pool) healthChecker() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.config.HealthCheckPeriod)
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

// checkHealth checks all connections and reconnects if needed (thread-safe).
func (p *Pool) checkHealth() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	now := time.Now().Unix()

	for name, pc := range p.clients {
		// Check if connection is idle too long
		lastUsed := atomic.LoadInt64(&pc.lastUsed)
		if time.Duration(now-lastUsed)*time.Second > p.config.MaxIdleTime {
			// Connection too idle, close it
			_ = pc.client.Close()
			atomic.StoreInt32(&pc.state, int32(StateDisconnected))
			continue
		}

		// Ping connection
		if err := pc.client.Ping(ctx); err != nil {
			pc.markFailed()

			// Check if too many failures
			if atomic.LoadInt32(&pc.failCount) >= p.config.MaxFailures {
				_ = pc.client.Close()
				atomic.StoreInt32(&pc.state, int32(StateFailed))

				// Try to reconnect
				go p.reconnect(name, pc.config)
			}
		} else {
			// Connection healthy
			atomic.StoreInt32(&pc.state, int32(StateConnected))
			atomic.StoreInt32(&pc.failCount, 0)
		}
	}
}

// reconnect attempts to reconnect a failed connection (runs in background).
func (p *Pool) reconnect(name string, cfg db.Config) {
	time.Sleep(p.config.RetryDelay)

	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Check if still failed
	pc, exists := p.clients[name]
	if !exists || State(atomic.LoadInt32(&pc.state)) != StateFailed {
		return // Already reconnected or removed
	}

	atomic.StoreInt32(&pc.state, int32(StateConnecting))

	client, err := p.factory.Create(cfg)
	if err != nil {
		atomic.StoreInt32(&pc.state, int32(StateFailed))
		return
	}

	// Replace client
	_ = pc.client.Close()
	pc.client = client
	atomic.StoreInt64(&pc.lastUsed, time.Now().Unix())
	atomic.StoreInt32(&pc.state, int32(StateConnected))
	atomic.StoreInt32(&pc.failCount, 0)
}

// GetConnectionNames returns all registered connection names (thread-safe).
func (p *Pool) GetConnectionNames() []string {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	names := make([]string, 0, len(p.clients))
	for name := range p.clients {
		names = append(names, name)
	}
	return names
}

// Size returns current pool size (thread-safe).
func (p *Pool) Size() int {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return len(p.clients)
}
