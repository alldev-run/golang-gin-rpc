package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"alldev-gin-rpc/pkg/gateway"
	"alldev-gin-rpc/pkg/logger"
	"alldev-gin-rpc/pkg/tracing"

	"github.com/gin-gonic/gin"
)

func main() {
	// Initialize logger
	logger.Init(logger.DefaultConfig())

	// Initialize tracing
	if err := tracing.InitForDevelopment("http-gateway"); err != nil {
		log.Printf("Failed to initialize tracing: %v", err)
	}
	
	// Ensure tracing is shutdown gracefully
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tracing.ShutdownGlobalTracer(ctx); err != nil {
			log.Printf("Failed to shutdown tracer: %v", err)
		}
	}()

	// Create gateway configuration
	config := gateway.DefaultConfig()
	config.Host = "0.0.0.0"
	config.Port = 8080

	// Add example routes
	config.Routes = []gateway.RouteConfig{
		{
			Path:        "/api/users",
			Method:      "GET",
			Service:     "user-service",
			StripPrefix: false,
			Timeout:     30 * time.Second,
			Retries:     3,
		},
		{
			Path:        "/api/users",
			Method:      "POST",
			Service:     "user-service",
			StripPrefix: false,
			Timeout:     30 * time.Second,
			Retries:     3,
		},
	}

	// Create gateway instance
	gw := gateway.NewGateway(config)

	// Initialize gateway
	if err := gw.Initialize(); err != nil {
		logger.Errorf("Failed to initialize gateway", logger.Error(err))
		os.Exit(1)
	}

	// Setup Gin engine
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()

	// Setup gateway routes and middleware
	gw.SetupRoutes(engine)

	// Create HTTP server
	srv := &http.Server{
		Addr:    ":8080",
		Handler: engine,
	}

	// Start gateway
	if err := gw.Start(); err != nil {
		logger.Errorf("Failed to start gateway", logger.Error(err))
		os.Exit(1)
	}

	logger.Info("HTTP Gateway starting on :8080")

	// Handle graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Start server in goroutine
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorf("Failed to start server", logger.Error(err))
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	<-quit
	logger.Info("Shutting down HTTP Gateway...")

	// Shutdown gateway
	if err := gw.Stop(); err != nil {
		logger.Errorf("Error stopping gateway", logger.Error(err))
	}

	// Shutdown HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Errorf("Error during server shutdown", logger.Error(err))
	}

	logger.Info("HTTP Gateway shutdown complete")
}