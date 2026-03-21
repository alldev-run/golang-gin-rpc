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

	"alldev-gin-rpc/api/ws-gateway/internal/biz/gatewayhttp"
	"alldev-gin-rpc/pkg/config"
	"alldev-gin-rpc/pkg/logger"
)

func main() {
	configPath := "./configs/config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	cfg, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	initLogger(cfg)

	gwSvc, err := gatewayhttp.NewFromConfig(cfg)
	if err != nil {
		log.Fatalf("failed to init gateway service: %v", err)
	}
	defer func() {
		_ = gwSvc.Close()
	}()

	gin.SetMode(appMode(cfg.App.Environment, cfg.App.Debug))
	r := gin.New()
	r.Use(gin.Recovery())

	gwSvc.Register(r)

	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.HTTP.Host, cfg.Server.HTTP.Port),
		Handler:      r,
		ReadTimeout:  cfg.Server.HTTP.ReadTimeout,
		WriteTimeout: cfg.Server.HTTP.WriteTimeout,
		IdleTimeout:  cfg.Server.HTTP.IdleTimeout,
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("ws-gateway http starting",
			logger.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorf("http server failed", logger.Error(err))
		}
	}()

	<-sigCh
	logger.Info("ws-gateway http shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

func loadConfig(path string) (*config.GlobalConfig, error) {
	loader := config.NewLoader()
	loader.Set(config.DefaultConfig())
	if err := loader.Load(path); err != nil {
		return nil, err
	}
	return loader.Get(), nil
}

func initLogger(cfg *config.GlobalConfig) {
	lc := logger.DefaultConfig()

	if cfg.Observability.Logging.Level != "" {
		lc.Level = logger.LogLevel(cfg.Observability.Logging.Level)
	}
	if cfg.Observability.Logging.Output != "" {
		lc.Output = logger.LogOutput(cfg.Observability.Logging.Output)
	}
	if cfg.Observability.Logging.Format != "" {
		lc.Format = logger.LogFormat(cfg.Observability.Logging.Format)
	}
	if cfg.Observability.Logging.FilePath != "" {
		lc.LogPath = cfg.Observability.Logging.FilePath
	}
	if cfg.Observability.Logging.MaxSize > 0 {
		lc.MaxSize = cfg.Observability.Logging.MaxSize
	}
	if cfg.Observability.Logging.MaxBackups > 0 {
		lc.MaxBackups = cfg.Observability.Logging.MaxBackups
	}
	if cfg.Observability.Logging.MaxAge > 0 {
		lc.MaxAge = cfg.Observability.Logging.MaxAge
	}
	lc.Compress = cfg.Observability.Logging.MaxBackups > 0

	logger.Init(lc)
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
