package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"alldev-gin-rpc/internal/bootstrap"
	"alldev-gin-rpc/pkg/logger"
)

func main() {
	// Initialize logger
	logger.Init(logger.Config{
		Level: "info",
		Env:   "dev",
	})

	// Load configuration
	configPath := "configs/config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	// Create bootstrap
	bs, err := bootstrap.NewBootstrap(configPath)
	if err != nil {
		log.Fatalf("Failed to create bootstrap: %v", err)
	}

	// Initialize all components
	if err := bs.InitializeAll(); err != nil {
		log.Fatalf("Failed to initialize components: %v", err)
	}

	// Get gateway instance
	gw := bs.GetGateway()
	if gw == nil {
		log.Fatal("Gateway not initialized")
	}

	// Create Gin engine
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	// Setup gateway routes
	gw.SetupRoutes(r)

	// Add custom middleware
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// Add custom routes
	r.GET("/gateway/info", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"service": "http-gateway",
			"version": "1.0.0",
			"status":  "running",
		})
	})

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start server
	gatewayConfig := gw.GetConfig()
	addr := fmt.Sprintf("%s:%d", gatewayConfig.Host, gatewayConfig.Port)

	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  gatewayConfig.ReadTimeout,
		WriteTimeout: gatewayConfig.WriteTimeout,
		IdleTimeout:  gatewayConfig.IdleTimeout,
	}

	go func() {
		logger.Info("HTTP Gateway starting",
			logger.String("address", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorf("Failed to start server: %v", logger.Errorf(err))
		}
	}()

	// Wait for signal
	<-sigCh
	logger.Info("Shutting down HTTP Gateway...")

	// Shutdown server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Errorf("Error during server shutdown: %v", logger.Errorf(err))
	}

	// Close bootstrap
	if err := bs.Close(); err != nil {
		logger.Errorf("Error during bootstrap shutdown: %v", logger.Errorf(err))
	}

	logger.Info("HTTP Gateway shutdown complete")
}
