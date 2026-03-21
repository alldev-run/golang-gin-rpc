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
	
	"gopkg.in/yaml.v3"

	"github.com/gin-gonic/gin"

	"alldev-gin-rpc/api/http-gateway/internal/biz/gatewayhttp"
	"alldev-gin-rpc/api/http-gateway/internal/router"
	"alldev-gin-rpc/pkg/gateway"
	"alldev-gin-rpc/pkg/logger"
)

func main() {
	configPath := "./api/http-gateway/config/config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	gwCfg, err := loadGatewayConfig(configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	logger.Init(logger.DefaultConfig())

	gwSvc, err := gatewayhttp.New(gwCfg)
	if err != nil {
		log.Fatalf("failed to init gateway service: %v", err)
	}
	defer func() { _ = gwSvc.Close() }()

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	// Gateway routes + middlewares (CORS/RateLimit/RequestID/Logging)
	gwSvc.Register(r)

	// Business routes
	router.NewRouter().Register(r)

	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", gwCfg.Host, gwCfg.Port),
		Handler:      r,
		ReadTimeout:  gwCfg.ReadTimeout,
		WriteTimeout: gwCfg.WriteTimeout,
		IdleTimeout:  gwCfg.IdleTimeout,
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("http-gateway starting",
			logger.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorf("http server failed", logger.Error(err))
		}
	}()

	<-sigCh
	logger.Info("http-gateway shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

func loadGatewayConfig(path string) (*gateway.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := gateway.DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
