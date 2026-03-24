//go:build ignore
// +build ignore

package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
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

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "message": "ok"})
	})
	mux.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"message": "users",
			"data": map[string]any{
				"users": []map[string]any{{"id": "1", "name": "demo"}},
			},
		})
	})

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

	if err := http.ListenAndServe(":8080", mux); err != nil {
		panic(err)
	}
}
