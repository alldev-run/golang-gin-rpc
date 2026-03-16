package main

import (
	"log"
	"time"

	"golang-gin-rpc/internal/app"
	"golang-gin-rpc/internal/bootstrap"
	"golang-gin-rpc/internal/router"
)

func main() {
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

	// Start application
	if err := application.Start(); err != nil {
		log.Fatalf("Failed to start application: %v", err)
	}

	// Wait for shutdown
	application.WaitForShutdown()
}