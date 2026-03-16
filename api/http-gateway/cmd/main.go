package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go-micro/api/http-gateway/internal/middleware"
	"go-micro/api/http-gateway/internal/router"
	"golang-gin-rpc/pkg/tracing"

	"github.com/gin-gonic/gin"
)

func main() {
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

	// Set Gin mode
	gin.SetMode(gin.ReleaseMode)
	
	// Create Gin engine
	r := gin.New()
	
	// Add middleware
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.Tracing("http-gateway"))
	r.Use(middleware.CORS())
	r.Use(middleware.RequestID())

	// Register routes
	router.InitRouter(r)

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start server in goroutine
	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	go func() {
		log.Println("HTTP Gateway starting on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for signal
	<-sigCh
	log.Println("Shutting down HTTP Gateway...")

	// Shutdown server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	log.Println("HTTP Gateway shutdown complete")
}