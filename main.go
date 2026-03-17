package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"alldev-gin-rpc/internal/app"
	"alldev-gin-rpc/internal/bootstrap"
	"alldev-gin-rpc/internal/router"
	"alldev-gin-rpc/pkg/tracing"
)

func main() {
	// Initialize tracing first
	if err := tracing.InitFromFile("./configs/tracing.yaml"); err != nil {
		log.Printf("Failed to initialize tracing: %v", err)
		// Continue without tracing
	}
	
	// Ensure tracing is shutdown gracefully
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tracing.ShutdownGlobalTracer(ctx); err != nil {
			log.Printf("Failed to shutdown tracer: %v", err)
		}
	}()

	// Initialize bootstrap
	boot, err := bootstrap.NewBootstrap("./configs/config.yaml")
	if err != nil {
		log.Fatalf("Failed to initialize bootstrap: %v", err)
	}
	defer boot.Close()

	// Initialize databases
	if err := boot.InitializeDatabases(); err != nil {
		log.Fatalf("Failed to initialize databases: %v", err)
	}

	// Initialize cache
	if err := boot.InitializeCache(); err != nil {
		log.Fatalf("Failed to initialize cache: %v", err)
	}

	// Initialize RPC services
	if err := boot.InitializeRPC(); err != nil {
		log.Fatalf("Failed to initialize RPC services: %v", err)
	}

	// Initialize service discovery
	if err := boot.InitializeDiscovery(); err != nil {
		log.Fatalf("Failed to initialize service discovery: %v", err)
	}

	// Get configuration
	config := boot.GetConfig()

	// Create application
	application := app.NewApplication(app.Config{
		Host:         config.Server.Host,
		Port:         config.Server.Port,
		Mode:         config.Server.Mode,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	})

	// Register routes
	router := router.NewRouter(application)
	router.RegisterRoutes()

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start application in goroutine
	go func() {
		if err := application.Start(); err != nil {
			log.Fatalf("Failed to start application: %v", err)
		}
	}()

	// Wait for signal
	<-sigCh
	log.Println("Shutting down application...")

	// Shutdown application
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	
	if err := application.Shutdown(shutdownCtx); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	log.Println("Application shutdown complete")
}