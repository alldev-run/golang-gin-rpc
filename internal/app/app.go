package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
)

// Application represents the main application
type Application struct {
	server   *http.Server
	shutdown chan struct{}
}

// Config holds application configuration
type Config struct {
	Host         string
	Port         string
	Mode         string // debug, release, test
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// NewApplication creates a new application instance
func NewApplication(config Config) *Application {
	// Set Gin mode
	gin.SetMode(config.Mode)

	// Create Gin engine
	router := gin.New()

	// Add middleware
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	return &Application{
		server: &http.Server{
			Addr:         fmt.Sprintf("%s:%s", config.Host, config.Port),
			Handler:      router,
			ReadTimeout:  config.ReadTimeout,
			WriteTimeout: config.WriteTimeout,
			IdleTimeout:  config.IdleTimeout,
		},
		shutdown: make(chan struct{}),
	}
}

// Router returns the Gin router for adding routes
func (app *Application) Router() *gin.Engine {
	return app.server.Handler.(*gin.Engine)
}

// Start starts the application
func (app *Application) Start() error {
	logger.Info("Starting application on %s", zap.String("addr", app.server.Addr))

	// Start server in goroutine
	go func() {
		if err := app.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorf("Server failed to start", logger.Error(err))
			close(app.shutdown)
		}
	}()

	return nil
}

// Shutdown gracefully shuts down the application
func (app *Application) Shutdown(ctx context.Context) error {
	logger.Info("Shutting down server...")
	
	if err := app.server.Shutdown(ctx); err != nil {
		logger.Errorf("Server forced to shutdown", logger.Error(err))
		return err
	} else {
		logger.Info("Server shutdown complete")
		return nil
	}
}

// WaitForShutdown waits for shutdown signals
func (app *Application) WaitForShutdown() {
	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		logger.Info("Shutting down server...")
	case <-app.shutdown:
		logger.Info("Server shutdown initiated...")
	}

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := app.server.Shutdown(ctx); err != nil {
		logger.Errorf("Server forced to shutdown", logger.Error(err))
	} else {
		logger.Info("Server shutdown complete")
	}
}

// Logger returns the global logger
func (app *Application) Logger() *zap.Logger {
	return logger.L()
}
