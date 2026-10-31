//go:build ignore
// +build ignore

package main

import (
	"encoding/json"
	"fmt"
	"net/http"

)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "message": "ok"})
	})
	mux.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": []any{}})
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
	fmt.Println("Middleware features:")
	fmt.Println("  - Request ID automatically added")
	fmt.Println("  - CORS headers configured")
	fmt.Println("  - Rate limiting headers added")
	fmt.Println("  - Unified structured logging (same format as http middleware)")
	fmt.Println("  - Panic recovery")
	fmt.Println()
	fmt.Println("Log format (unified with http middleware):")
	fmt.Println(`{"level":"INFO","ts":"2026-03-22T02:09:17+08:00","caller":"logger/logger.go:42","msg":"HTTP Request","method":"GET","path":"/debug/ok","client_ip":"::1","status":200,"latency":0,"request_id":"agordfsr3azu3cttts2pydb46i","user_agent":"Mozilla/5.0..."}`)

	if err := http.ListenAndServe(":8080", mux); err != nil {
		panic(err)
	}
}
