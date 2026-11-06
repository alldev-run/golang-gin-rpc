package router

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/alldev-run/golang-gin-rpc/internal/app"
	"github.com/alldev-run/golang-gin-rpc/pkg/response"
	"github.com/alldev-run/golang-gin-rpc/plugins/cache"
	"github.com/alldev-run/golang-gin-rpc/plugins/db"
)

// Options wires framework-managed dependencies into the application routes.
type Options struct {
	// DBFactory enables /api/v1/db/status (nil: endpoint reports not initialized).
	DBFactory *db.Factory
	// Cache enables /api/v1/cache/* (nil: endpoints report not initialized).
	Cache cache.Cache
}

// Router handles route registration
type Router struct {
	app       *app.Application
	dbFactory *db.Factory
	cache     cache.Cache
}

// NewRouter creates a new router instance
func NewRouter(application *app.Application, opts Options) *Router {
	return &Router{
		app:       application,
		dbFactory: opts.DBFactory,
		cache:     opts.Cache,
	}
}

// RegisterRoutes registers all application routes
func (r *Router) RegisterRoutes() {
	router := r.app.Router()

	// Health check
	router.GET("/health", func(c *gin.Context) {
		response.Success(c, gin.H{
			"status":  "healthy",
			"service": "github.com/alldev-run/golang-gin-rpc",
		})
	})

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// User routes
		users := v1.Group("/users")
		{
			users.GET("", r.getUsers)
			users.POST("", r.createUser)
			users.GET("/:id", r.getUser)
			users.PUT("/:id", r.updateUser)
			users.DELETE("/:id", r.deleteUser)
		}

		// Database routes
		db := v1.Group("/db")
		{
			db.GET("/status", r.getDatabaseStatus)
			db.POST("/query", r.executeQuery)
		}

		// Cache routes
		cache := v1.Group("/cache")
		{
			cache.GET("/:key", r.getCache)
			cache.POST("/:key", r.setCache)
			cache.DELETE("/:key", r.deleteCache)
		}
	}
}

// Route handlers
func (r *Router) getUsers(c *gin.Context) {
	// TODO: Implement get users logic
	response.Success(c, gin.H{
		"users": []interface{}{},
		"total": 0,
	})
}

func (r *Router) createUser(c *gin.Context) {
	// TODO: Implement create user logic
	response.Success(c, gin.H{
		"message": "User created successfully",
	})
}

func (r *Router) getUser(c *gin.Context) {
	id := c.Param("id")
	// TODO: Implement get user logic
	response.Success(c, gin.H{
		"id":   id,
		"user": gin.H{},
	})
}

func (r *Router) updateUser(c *gin.Context) {
	id := c.Param("id")
	// TODO: Implement update user logic
	response.Success(c, gin.H{
		"id":      id,
		"message": "User updated successfully",
	})
}

func (r *Router) deleteUser(c *gin.Context) {
	id := c.Param("id")
	// TODO: Implement delete user logic
	response.Success(c, gin.H{
		"id":      id,
		"message": "User deleted successfully",
	})
}

func (r *Router) getDatabaseStatus(c *gin.Context) {
	if r.dbFactory == nil {
		response.Error(c, "database not initialized", nil)
		return
	}

	ctx := c.Request.Context()
	databases := gin.H{}
	healthy := true
	checked := 0
	for _, dbType := range []db.Type{db.TypeMySQL, db.TypePostgres} {
		client, err := r.dbFactory.GetClient(dbType)
		if err != nil {
			continue // 该驱动未启用
		}
		checked++
		pingErr := client.Ping(ctx)
		databases[string(dbType)] = gin.H{
			"healthy": pingErr == nil,
			"error":   errString(pingErr),
		}
		if pingErr != nil {
			healthy = false
		}
	}
	if checked == 0 {
		response.Error(c, "no database client configured", nil)
		return
	}

	if !healthy {
		response.Error(c, "database unhealthy", gin.H{"healthy": false, "databases": databases})
		return
	}
	response.Success(c, gin.H{"healthy": true, "databases": databases})
}

// errString maps a nil error to an empty string for JSON payloads.
func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (r *Router) executeQuery(c *gin.Context) {
	// TODO: Implement query execution
	response.Success(c, gin.H{
		"message": "Query executed successfully",
		"results": []interface{}{},
	})
}

func (r *Router) getCache(c *gin.Context) {
	if r.cache == nil {
		response.Error(c, "cache not initialized", nil)
		return
	}
	key := c.Param("key")

	value, err := r.cache.Get(c.Request.Context(), key)
	if err != nil {
		response.Error(c, "cache get failed: "+err.Error(), nil)
		return
	}
	response.Success(c, gin.H{
		"key":    key,
		"value":  value,
		"exists": value != nil,
	})
}

// setCacheRequest is the JSON body of POST /api/v1/cache/:key.
type setCacheRequest struct {
	Value      interface{} `json:"value"`
	TTLSeconds int         `json:"ttl_seconds"` // 0 或缺省时使用默认 TTL
}

// defaultCacheTTL is applied when the request does not specify one.
const defaultCacheTTL = 5 * time.Minute

func (r *Router) setCache(c *gin.Context) {
	if r.cache == nil {
		response.Error(c, "cache not initialized", nil)
		return
	}
	key := c.Param("key")

	var req setCacheRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, "invalid request body: "+err.Error(), nil)
		return
	}
	ttl := time.Duration(req.TTLSeconds) * time.Second
	if ttl <= 0 {
		ttl = defaultCacheTTL
	}

	if err := r.cache.Set(context.Background(), key, req.Value, ttl); err != nil {
		response.Error(c, "cache set failed: "+err.Error(), nil)
		return
	}
	response.Success(c, gin.H{
		"key":         key,
		"ttl_seconds": int(ttl.Seconds()),
		"message":     "Cache set successfully",
	})
}

func (r *Router) deleteCache(c *gin.Context) {
	if r.cache == nil {
		response.Error(c, "cache not initialized", nil)
		return
	}
	key := c.Param("key")

	if err := r.cache.Delete(c.Request.Context(), key); err != nil {
		response.Error(c, "cache delete failed: "+err.Error(), nil)
		return
	}
	response.Success(c, gin.H{
		"key":     key,
		"message": "Cache deleted successfully",
	})
}
