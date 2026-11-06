package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/alldev-run/golang-gin-rpc/internal/app"
	"github.com/alldev-run/golang-gin-rpc/internal/bootstrap"
	"github.com/alldev-run/golang-gin-rpc/internal/router"
)

func main() {
	// Initialize bootstrap (config path may be overridden via the first CLI argument)
	configPath := "./configs/config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}
	boot, err := bootstrap.NewBootstrap(configPath)
	if err != nil {
		log.Fatalf("Failed to initialize bootstrap: %v", err)
	}
	defer boot.Close()

	frameworkOptions := boot.FrameworkOptionsFromConfig()
	if len(frameworkOptions.Services) == 0 {
		frameworkOptions.Services = []string{bootstrap.ServiceRPC}
	}

	if err := boot.StartFramework(context.Background(), frameworkOptions); err != nil {
		log.Fatalf("Failed to start framework services: %v", err)
	}

	// Get configuration
	config := boot.GetConfig()

	// Create application
	application := app.NewApplication(app.Config{
		Host:         config.Server.HTTP.Host,
		Port:         strconv.Itoa(config.Server.HTTP.Port),
		Mode:         appMode(config.App.Environment, config.App.Debug),
		ReadTimeout:  config.Server.HTTP.ReadTimeout,
		WriteTimeout: config.Server.HTTP.WriteTimeout,
		IdleTimeout:  config.Server.HTTP.IdleTimeout,
	})

	// Register routes (wire framework-managed dependencies into demo routes)
	router := router.NewRouter(application, router.Options{
		DBFactory: boot.GetDatabaseFactory(),
		Cache:     boot.GetCache(),
	})
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

	// Shutdown application (graceful period from server.shutdown_timeout, default 30s)
	shutdownTimeout := config.Server.ShutdownTimeout
	if shutdownTimeout <= 0 {
		shutdownTimeout = 30 * time.Second
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	if err := application.Shutdown(shutdownCtx); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	if err := boot.StopFramework(shutdownCtx, frameworkOptions.Services...); err != nil {
		log.Printf("Error stopping framework services: %v", err)
	}

	log.Println("Application shutdown complete")
}
func appMode(environment string, debug bool) string {
	if debug {
		return "debug"
	}
	if environment == "test" {
		return "test"
	}
	return "release"
}
