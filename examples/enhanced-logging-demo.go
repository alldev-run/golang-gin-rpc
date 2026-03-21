package main

import (
	"fmt"
	"net/http"
	"time"

	"alldev-gin-rpc/api/http-gateway/internal/httpapi"
	"alldev-gin-rpc/pkg/gateway"
	"alldev-gin-rpc/pkg/logger"
)

func main() {
	// 初始化 logger 配置
	loggerCfg := logger.Config{
		Level:       "info",
		Format:      "json",
		Output:      "console",
		TimeFormat:  "2006-01-02T15:04:05+08:00",
		EnableCaller: true,
	}
	logger.Init(loggerCfg)

	// 创建基础配置
	cfg := &gateway.Config{
		ServiceName: "test-gateway",
		Host:        "localhost",
		Port:        8080,
		CORS: gateway.CORSConfig{
			AllowedOrigins:   []string{"*"},
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"*"},
			ExposedHeaders:   []string{"X-Request-ID"},
			AllowCredentials: false,
			MaxAge:           86400,
		},
		RateLimit: gateway.RateLimitConfig{
			Enabled:  true,
			Requests: 100,
			Window:   "1m",
		},
	}

	// 创建路由器
	router := httpapi.NewRouter(cfg)
	handler := router.Handler()

	// 启动服务器
	fmt.Println("Starting test server on :8080")
	fmt.Println("Available endpoints:")
	fmt.Println("  GET  /                    - Hello endpoint")
	fmt.Println("  GET  /debug/ok           - Debug OK")
	fmt.Println("  GET  /debug/request-id   - Debug Request ID")
	fmt.Println("  GET  /debug/tracing      - Debug Tracing")
	fmt.Println("  GET  /api/users          - User list")
	fmt.Println("  POST /api/user           - Create user (requires X-API-Key header)")
	fmt.Println("  GET  /api/user/:id       - Get user (requires X-API-Key header)")
	fmt.Println("  PUT  /api/user/:id       - Update user (requires X-API-Key header)")
	fmt.Println("  DELETE /api/user/:id     - Delete user (requires X-API-Key header)")
	fmt.Println()
	fmt.Println("Enhanced Logging Features:")
	fmt.Println("  - Level-based logging (INFO/WARN/ERROR based on HTTP status)")
	fmt.Println("  - Request ID tracking with structured logging")
	fmt.Println("  - Slow request detection (>1s)")
	fmt.Println("  - Unified log format using pkg/logger")
	fmt.Println()
	fmt.Println("Log Examples:")
	fmt.Println(`{"level":"INFO","ts":"2026-03-22T02:09:17+08:00","caller":"logger/logger.go:42","msg":"HTTP Request","method":"GET","path":"/debug/ok","client_ip":"::1","status":200,"latency":0,"request_id":"agordfsr3azu3cttts2pydb46i","user_agent":"Mozilla/5.0..."}`)
	fmt.Println(`{"level":"WARN","ts":"2026-03-22T02:09:17+08:00","caller":"logger/logger.go:42","msg":"HTTP Request","method":"GET","path":"/api/notfound","client_ip":"::1","status":404,"latency":0,"request_id":"...","user_agent":"..."}`)
	fmt.Println(`{"level":"ERROR","ts":"2026-03-22T02:09:17+08:00","caller":"logger/logger.go:42","msg":"HTTP Request","method":"GET","path":"/api/error","client_ip":"::1","status":500,"latency":0,"request_id":"...","user_agent":"..."}`)
	fmt.Println(`{"level":"WARN","ts":"2026-03-22T02:09:17+08:00","caller":"logger/logger.go:42","msg":"HTTP Slow Request","threshold":1000000000,"method":"GET","path":"/api/slow","client_ip":"::1","status":200,"latency":1500000000,"request_id":"...","user_agent":"..."}`)

	if err := http.ListenAndServe(":8080", handler); err != nil {
		panic(err)
	}
}
