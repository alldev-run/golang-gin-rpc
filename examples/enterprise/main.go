
package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"alldev-gin-rpc/pkg/cache"
	"alldev-gin-rpc/pkg/db/pool"
	"alldev-gin-rpc/pkg/errors"
	"alldev-gin-rpc/pkg/health"
	"alldev-gin-rpc/pkg/metrics"
	"alldev-gin-rpc/pkg/logger"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// EnterpriseApplication demonstrates enterprise-level optimizations
type EnterpriseApplication struct {
	healthManager *health.HealthManager
	metrics      *metrics.MetricsCollector
	cache        cache.Cache
	dbPool       *pool.EnhancedPool
}

type sqlPinger struct {
	db *sql.DB
}

func (p *sqlPinger) Ping(ctx context.Context) error {
	return p.db.PingContext(ctx)
}

// NewEnterpriseApplication creates a new enterprise application
func NewEnterpriseApplication() (*EnterpriseApplication, error) {
	app := &EnterpriseApplication{}
	
	// Initialize components
	if err := app.initializeComponents(); err != nil {
		return nil, fmt.Errorf("failed to initialize components: %w", err)
	}
	
	return app, nil
}

// initializeComponents initializes all enterprise components
func (app *EnterpriseApplication) initializeComponents() error {
	// 1. Initialize metrics
	app.metrics = metrics.NewMetricsCollector()
	
	// 2. Initialize health manager
	app.healthManager = health.GetGlobalHealthManager()
	
	// 3. Initialize cache with breakdown protection
	baseCache, err := cache.NewRedisCache(cache.RedisConfig{
		Host: "localhost",
		Port: 6379,
	})
	if err != nil {
		return fmt.Errorf("failed to create cache: %w", err)
	}
	app.cache = cache.NewBreakdownCache(baseCache)
	
	// 4. Initialize database with enhanced pool
	db, err := sql.Open("mysql", "user:password@tcp(localhost:3306)/dbname")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	
	poolConfig := pool.ProductionPoolConfig()
	app.dbPool, err = pool.NewEnhancedPool(db, poolConfig)
	if err != nil {
		return fmt.Errorf("failed to create enhanced pool: %w", err)
	}
	
	// 5. Register health checkers
	app.registerHealthCheckers()
	
	// 6. Start health monitoring
	app.healthManager.Start()
	
	return nil
}

// registerHealthCheckers registers all health checkers
func (app *EnterpriseApplication) registerHealthCheckers() {
	// Database health checker
	dbChecker := health.NewDatabaseHealthChecker("database", &sqlPinger{db: app.dbPool.GetDB()})
	app.healthManager.RegisterChecker(dbChecker, health.DefaultHealthCheckConfig())
	
	// Cache health checker
	cacheChecker := health.NewCacheHealthChecker("redis", app.cache)
	app.healthManager.RegisterChecker(cacheChecker, health.DefaultHealthCheckConfig())
	
	// Custom application health checker
	appChecker := health.NewCustomHealthChecker("application", func(ctx context.Context) *health.CheckResult {
		// Custom health logic
		return &health.CheckResult{
			Name:      "application",
			Status:    health.StatusHealthy,
			Message:   "Application is healthy",
			Timestamp: time.Now(),
		}
	})
	app.healthManager.RegisterChecker(appChecker, health.DefaultHealthCheckConfig())
}

// SetupRoutes sets up application routes with enterprise middleware
func (app *EnterpriseApplication) SetupRoutes() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	
	// Add enterprise middleware
	r.Use(app.metricsMiddleware())
	r.Use(app.recoveryMiddleware())
	r.Use(app.requestIDMiddleware())
	
	// Health check endpoint
	r.GET("/health", gin.WrapF(app.healthManager.HTTPHandler()))
	r.GET("/health/:check", func(c *gin.Context) {
		checkName := c.Param("check")
		result := app.healthManager.CheckHealthByName(c.Request.Context(), checkName)
		
		if result.Status == health.StatusHealthy {
			c.JSON(http.StatusOK, result)
		} else {
			c.JSON(http.StatusServiceUnavailable, result)
		}
	})
	
	// Metrics endpoint
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	
	// API routes
	api := r.Group("/api/v1")
	{
		api.GET("/users", app.getUsers)
		api.POST("/users", app.createUser)
		api.GET("/cache/:key", app.getCacheValue)
		api.POST("/cache/:key", app.setCacheValue)
	}
	
	return r
}

// metricsMiddleware records HTTP metrics
func (app *EnterpriseApplication) metricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		
		c.Next()
		
		duration := time.Since(start)
		statusCode := fmt.Sprintf("%d", c.Writer.Status())
		
		app.metrics.RecordHTTPRequest(c.Request.Method, c.Request.URL.Path, statusCode, duration)
	}
}

// recoveryMiddleware handles panics with structured logging
func (app *EnterpriseApplication) recoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				appErr := errors.New(errors.ErrCodePanicRecovered, "Panic recovered in HTTP handler").
					WithCause(fmt.Errorf("%v", err)).
					WithRequestID(c.GetString("request_id")).
					WithStackTrace()
				
				logger.Errorf("Panic recovered in HTTP handler",
					zap.String("error", appErr.Error()),
					zap.String("request_id", c.GetString("request_id")),
					zap.String("method", c.Request.Method),
					zap.String("path", c.Request.URL.Path),
				)
				
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": appErr.Code,
					"message": "Internal server error",
					"request_id": c.GetString("request_id"),
				})
				c.Abort()
			}
		}()
		
		c.Next()
	}
}

// requestIDMiddleware adds request ID to context
func (app *EnterpriseApplication) requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = fmt.Sprintf("%d", time.Now().UnixNano())
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

// getUsers demonstrates database operations with error handling
func (app *EnterpriseApplication) getUsers(c *gin.Context) {
	ctx := c.Request.Context()
	
	// Use enhanced database pool with retry logic
	rows, err := app.dbPool.Query(ctx, "SELECT id, name, email FROM users LIMIT 100")
	if err != nil {
		appErr := errors.Wrap(err, errors.ErrCodeDBQuery, "Failed to query users").
			WithRequestID(c.GetString("request_id"))
		
		logger.Errorf("Failed to query users",
			zap.Error(appErr),
			zap.String("request_id", c.GetString("request_id")),
		)
		
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": appErr.Code,
			"message": "Failed to fetch users",
		})
		return
	}
	defer rows.Close()
	
	var users []map[string]interface{}
	for rows.Next() {
		var user struct {
			ID    int    `json:"id"`
			Name  string `json:"name"`
			Email string `json:"email"`
		}
		
		if err := rows.Scan(&user.ID, &user.Name, &user.Email); err != nil {
			appErr := errors.Wrap(err, errors.ErrCodeDBQuery, "Failed to scan user").
				WithRequestID(c.GetString("request_id"))
			
			logger.Errorf("Failed to scan user",
				zap.Error(appErr),
				zap.String("request_id", c.GetString("request_id")),
			)
			
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": appErr.Code,
				"message": "Failed to process user data",
			})
			return
		}
		
		users = append(users, map[string]interface{}{
			"id":    user.ID,
			"name":  user.Name,
			"email": user.Email,
		})
	}
	
	app.metrics.RecordHTTPRequest("GET", "/api/v1/users", "200", 0)
	c.JSON(http.StatusOK, gin.H{
		"users": users,
		"count": len(users),
	})
}

// createUser demonstrates transaction handling
func (app *EnterpriseApplication) createUser(c *gin.Context) {
	ctx := c.Request.Context()
	
	var user struct {
		Name  string `json:"name" binding:"required"`
		Email string `json:"email" binding:"required,email"`
	}
	
	if err := c.ShouldBindJSON(&user); err != nil {
		appErr := errors.Wrap(err, errors.ErrCodeValidationFailed, "Invalid user data").
			WithDetails(map[string]interface{}{
				"validation_errors": err.Error(),
			}).
			WithRequestID(c.GetString("request_id"))
		
		c.JSON(http.StatusBadRequest, gin.H{
			"error": appErr.Code,
			"message": "Invalid user data",
			"details": appErr.Details,
		})
		return
	}
	
	// Use transaction with enhanced pool
	err := app.dbPool.Transaction(ctx, func(tx *sql.Tx) error {
		// Insert user
		result, err := tx.ExecContext(ctx, 
			"INSERT INTO users (name, email, created_at) VALUES (?, ?, ?)",
			user.Name, user.Email, time.Now())
		if err != nil {
			return err
		}
		
		// Get user ID
		userID, err := result.LastInsertId()
		if err != nil {
			return err
		}
		
		// Cache user data
		cacheKey := fmt.Sprintf("user:%d", userID)
		if err := app.cache.SetWithRandomTTL(ctx, cacheKey, user, time.Hour); err != nil {
			// Log cache error but don't fail the request
			logger.Warn("Failed to cache user data",
				zap.Error(err),
				zap.String("cache_key", cacheKey),
			)
		}
		
		return nil
	})
	
	if err != nil {
		appErr := errors.Wrap(err, errors.ErrCodeDBTransaction, "Failed to create user").
			WithRequestID(c.GetString("request_id"))
		
		logger.Errorf("Failed to create user",
			zap.Error(appErr),
			zap.String("request_id", c.GetString("request_id")),
		)
		
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": appErr.Code,
			"message": "Failed to create user",
		})
		return
	}
	
	app.metrics.RecordHTTPRequest("POST", "/api/v1/users", "201", 0)
	c.JSON(http.StatusCreated, gin.H{
		"message": "User created successfully",
		"user": user,
	})
}

// getCacheValue demonstrates cache operations with breakdown protection
func (app *EnterpriseApplication) getCacheValue(c *gin.Context) {
	ctx := c.Request.Context()
	key := c.Param("key")
	
	// Use cache with breakdown protection
	value, err := app.cache.GetWithLock(ctx, key)
	if err != nil {
		appErr := errors.Wrap(err, errors.ErrCodeCacheMiss, "Cache miss").
			WithRequestID(c.GetString("request_id"))
		
		c.JSON(http.StatusNotFound, gin.H{
			"error": appErr.Code,
			"message": "Cache key not found",
		})
		return
	}
	
	// Record cache metrics
	stats := app.cache.GetStats()
	app.metrics.UpdateCacheHitRatio("redis", float64(stats.Hits)/float64(stats.Hits+stats.Misses))
	
	c.JSON(http.StatusOK, gin.H{
		"key": key,
		"value": value,
		"stats": stats,
	})
}

// setCacheValue demonstrates cache operations with avalanche protection
func (app *EnterpriseApplication) setCacheValue(c *gin.Context) {
	ctx := c.Request.Context()
	key := c.Param("key")
	
	var request struct {
		Value interface{} `json:"value"`
		TTL   int         `json:"ttl"` // TTL in seconds
	}
	
	if err := c.ShouldBindJSON(&request); err != nil {
		appErr := errors.Wrap(err, errors.ErrCodeValidationFailed, "Invalid cache data").
			WithRequestID(c.GetString("request_id"))
		
		c.JSON(http.StatusBadRequest, gin.H{
			"error": appErr.Code,
			"message": "Invalid cache data",
		})
		return
	}
	
	ttl := time.Hour // Default TTL
	if request.TTL > 0 {
		ttl = time.Duration(request.TTL) * time.Second
	}
	
	// Use cache with avalanche protection
	err := app.cache.SetWithRandomTTL(ctx, key, request.Value, ttl)
	if err != nil {
		appErr := errors.Wrap(err, errors.ErrCodeCacheConnection, "Failed to set cache").
			WithRequestID(c.GetString("request_id"))
		
		logger.Errorf("Failed to set cache",
			zap.Error(appErr),
			zap.String("key", key),
			zap.String("request_id", c.GetString("request_id")),
		)
		
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": appErr.Code,
			"message": "Failed to set cache value",
		})
		return
	}
	
	app.metrics.RecordHTTPRequest("POST", "/api/v1/cache/:key", "200", 0)
	c.JSON(http.StatusOK, gin.H{
		"message": "Cache value set successfully",
		"key": key,
		"ttl": ttl.Seconds(),
	})
}

// Shutdown gracefully shuts down the application
func (app *EnterpriseApplication) Shutdown(ctx context.Context) error {
	logger.Info("Shutting down enterprise application")
	
	// Stop health manager
	app.healthManager.Stop()
	
	// Close database pool
	if err := app.dbPool.Close(); err != nil {
		logger.Errorf("Failed to close database pool", zap.Error(err))
	}
	
	// Close cache
	if err := app.cache.Close(); err != nil {
		logger.Errorf("Failed to close cache", zap.Error(err))
	}
	
	logger.Info("Enterprise application shutdown complete")
	return nil
}

func main() {
	// Initialize logger
	logger.Init(logger.Config{
		Level: "info",
		Env:   "production",
	})
	
	// Create enterprise application
	app, err := NewEnterpriseApplication()
	if err != nil {
		logger.Fatal("Failed to create enterprise application", zap.Error(err))
	}
	
	// Setup routes
	router := app.SetupRoutes()
	
	// Start server
	server := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}
	
	// Start metrics server in background
	go func() {
		logger.Info("Starting metrics server on :9090")
		if err := http.ListenAndServe(":9090", promhttp.Handler()); err != nil {
			logger.Errorf("Metrics server failed", zap.Error(err))
		}
	}()
	
	logger.Info("Starting enterprise application on :8080")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatal("Server failed", zap.Error(err))
	}
}
