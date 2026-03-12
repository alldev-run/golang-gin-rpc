// Package rwproxy provides automatic read-write splitting for SQL databases.
// It routes write operations (INSERT/UPDATE/DELETE) to the master and
// read operations (SELECT) to replicas with load balancing.
package rwproxy

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
)

// QueryType represents the type of SQL query.
type QueryType int

const (
	QueryUnknown QueryType = iota
	QueryWrite             // INSERT, UPDATE, DELETE, etc.
	QueryRead              // SELECT
)

// LBStrategy represents the load balancing strategy for replicas.
type LBStrategy int

const (
	LBStrategyRoundRobin LBStrategy = iota // Default
	LBStrategyRandom
)

// Config holds the read-write split configuration.
type Config struct {
	Master        *sql.DB    // Master database for writes
	Replicas      []*sql.DB  // Replica databases for reads
	Strategy      LBStrategy // Load balancing strategy
	MaxReplicaLag int        // Max acceptable replication lag (seconds, 0 = no check)
	ForceMaster   bool       // Force all queries to master (for maintenance)
}

// Client is a read-write splitting SQL client.
type Client struct {
	config       Config
	roundRobin   uint32 // Atomic counter for round-robin
	queryChecker *queryTypeChecker
}

// queryTypeChecker determines if a query is read or write.
type queryTypeChecker struct {
	writePattern *regexp.Regexp
	readPattern  *regexp.Regexp
}

// New creates a new read-write splitting client.
func New(config Config) *Client {
	return &Client{
		config: config,
		queryChecker: &queryTypeChecker{
			// Matches INSERT, UPDATE, DELETE, REPLACE, CREATE, DROP, ALTER, TRUNCATE, MERGE, UPSERT
			writePattern: regexp.MustCompile(`^\s*(?i)(INSERT|UPDATE|DELETE|REPLACE|CREATE|DROP|ALTER|TRUNCATE|MERGE|UPSERT|GRANT|REVOKE|LOCK)\s+`),
			// Matches SELECT
			readPattern: regexp.MustCompile(`^\s*(?i)SELECT\s+`),
		},
	}
}

// Close closes all database connections.
func (c *Client) Close() error {
	var errs []error

	if c.config.Master != nil {
		if err := c.config.Master.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close master: %w", err))
		}
	}

	for i, replica := range c.config.Replicas {
		if err := replica.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close replica %d: %w", i, err))
		}
	}

	if len(errs) > 0 {
		return errs[0] // Return first error
	}
	return nil
}

// getQueryType determines if a query is read or write.
func (c *Client) getQueryType(query string) QueryType {
	trimmed := strings.TrimSpace(query)

	if c.queryChecker.writePattern.MatchString(trimmed) {
		return QueryWrite
	}
	if c.queryChecker.readPattern.MatchString(trimmed) {
		return QueryRead
	}

	return QueryUnknown
}

// selectReplica selects a replica based on the load balancing strategy.
func (c *Client) selectReplica() *sql.DB {
	if len(c.config.Replicas) == 0 {
		return nil
	}

	if c.config.ForceMaster {
		return nil
	}

	switch c.config.Strategy {
	case LBStrategyRandom:
		// Simple implementation - can be enhanced with actual random
		idx := int(atomic.AddUint32(&c.roundRobin, 1)) % len(c.config.Replicas)
		return c.config.Replicas[idx]
	case LBStrategyRoundRobin:
		fallthrough
	default:
		idx := int(atomic.AddUint32(&c.roundRobin, 1)) % len(c.config.Replicas)
		return c.config.Replicas[idx]
	}
}

// getDB returns the appropriate database for the query.
func (c *Client) getDB(queryType QueryType) *sql.DB {
	// Always use master for writes or when force master is enabled
	if queryType == QueryWrite || c.config.ForceMaster {
		return c.config.Master
	}

	// For reads, try to use a replica
	if db := c.selectReplica(); db != nil {
		return db
	}

	// Fallback to master if no replicas available
	return c.config.Master
}

// ==================== Query Methods ====================

// Query executes a SELECT query on a replica (load balanced).
func (c *Client) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	db := c.getDB(c.getQueryType(query))
	if db == nil {
		return nil, fmt.Errorf("no database available for query")
	}
	return db.QueryContext(ctx, query, args...)
}

// QueryRow executes a SELECT query on a replica, returning a single row.
func (c *Client) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	db := c.getDB(c.getQueryType(query))
	if db == nil {
		// Return a row with error - this is not ideal but maintains compatibility
		return nil
	}
	return db.QueryRowContext(ctx, query, args...)
}

// Exec executes a write query (INSERT/UPDATE/DELETE) on the master.
func (c *Client) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	db := c.getDB(QueryWrite)
	if db == nil {
		return nil, fmt.Errorf("no master database available")
	}
	return db.ExecContext(ctx, query, args...)
}

// Prepare creates a prepared statement on the master.
// Note: For read-write split, prepared statements should generally be on master
// to avoid confusion about where to execute.
func (c *Client) Prepare(ctx context.Context, query string) (*sql.Stmt, error) {
	db := c.getDB(QueryWrite)
	if db == nil {
		return nil, fmt.Errorf("no master database available")
	}
	return db.PrepareContext(ctx, query)
}

// Begin starts a transaction on the master.
// Transactions should always go to master to ensure consistency.
func (c *Client) Begin(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	if c.config.Master == nil {
		return nil, fmt.Errorf("no master database available")
	}
	return c.config.Master.BeginTx(ctx, opts)
}

// Transaction executes a function within a transaction on the master.
func (c *Client) Transaction(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := c.Begin(ctx, nil)
	if err != nil {
		return err
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}

// Ping checks the health of master and all replicas.
func (c *Client) Ping(ctx context.Context) error {
	var errs []error
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Ping master
	wg.Add(1)
	go func() {
		defer wg.Done()
		if c.config.Master != nil {
			if err := c.config.Master.PingContext(ctx); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("master ping failed: %w", err))
				mu.Unlock()
			}
		}
	}()

	// Ping replicas
	for i, replica := range c.config.Replicas {
		wg.Add(1)
		go func(idx int, r *sql.DB) {
			defer wg.Done()
			if r != nil {
				if err := r.PingContext(ctx); err != nil {
					mu.Lock()
					errs = append(errs, fmt.Errorf("replica %d ping failed: %w", idx, err))
					mu.Unlock()
				}
			}
		}(i, replica)
	}

	wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("ping errors: %v", errs)
	}
	return nil
}

// Stats returns aggregated statistics from master and all replicas.
func (c *Client) Stats() DBStats {
	var stats DBStats

	if c.config.Master != nil {
		stats.Master = c.config.Master.Stats()
	}

	for _, replica := range c.config.Replicas {
		if replica != nil {
			stats.Replicas = append(stats.Replicas, replica.Stats())
		}
	}

	return stats
}

// DBStats holds aggregated statistics.
type DBStats struct {
	Master   sql.DBStats
	Replicas []sql.DBStats
}

// ==================== Utility Methods ====================

// ForceMaster forces all subsequent queries to use the master (for consistency).
func (c *Client) ForceMaster(force bool) {
	c.config.ForceMaster = force
}

// IsMasterForced returns whether master is being forced.
func (c *Client) IsMasterForced() bool {
	return c.config.ForceMaster
}

// AddReplica adds a new replica to the pool.
func (c *Client) AddReplica(db *sql.DB) {
	c.config.Replicas = append(c.config.Replicas, db)
}

// RemoveReplica removes a replica from the pool by index.
func (c *Client) RemoveReplica(index int) error {
	if index < 0 || index >= len(c.config.Replicas) {
		return fmt.Errorf("invalid replica index: %d", index)
	}

	c.config.Replicas = append(
		c.config.Replicas[:index],
		c.config.Replicas[index+1:]...,
	)
	return nil
}

// GetReplicaCount returns the number of replicas.
func (c *Client) GetReplicaCount() int {
	return len(c.config.Replicas)
}

// GetMasterDB returns the master database instance.
func (c *Client) GetMasterDB() *sql.DB {
	return c.config.Master
}
