
package main

import (
	"alldev-gin-rpc/pkg/logger"
	"alldev-gin-rpc/pkg/rpc"
	"alldev-gin-rpc/pkg/rpc/examples"
	"alldev-gin-rpc/pkg/rpc/jsonrpc"
	"context"
	"log"

	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logger.Init(logger.DefaultConfig())

	// Create RPC manager with default JSON-RPC server
	config := rpc.DefaultManagerConfig()
	manager := rpc.NewManager(config)

	// Create and add authentication middleware
	authConfig := rpc.DefaultAuthConfig()
	authConfig.Enabled = true
	authConfig.APIKeys = map[string]string{
		"sk-1234567890abcdef": "admin-user",
		"sk-abcdef1234567890": "service-account",
		"sk-9876543210fedcba": "readonly-user",
		"demo-key-2024":       "demo-user",
	}

	// Create auth middleware that wraps RPCAuth
	rpcAuth := rpc.NewRPCAuth(authConfig)
	authMiddleware := rpc.NewMiddleware("auth", func(ctx context.Context, req interface{}) (interface{}, error) {
		return rpcAuth.Execute(ctx, req)
	})
	manager.AddMiddleware(authMiddleware)

	// Register services
	userService := examples.NewUserService()
	if err := manager.RegisterService(userService); err != nil {
		log.Fatalf("Failed to register user service: %v", err)
	}

	// Start RPC servers
	go func() {
		if err := manager.Start(); err != nil {
			logger.Errorf("Failed to start RPC manager", zap.Error(err))
		}
	}()

	logger.Info("RPC servers started successfully")
	logger.Info("JSON-RPC server: localhost:8080")
	logger.Info("Authentication enabled - valid API keys:")
	for key, user := range authConfig.APIKeys {
		logger.Info("  %s -> %s", zap.String("key", key), zap.String("user", user))
	}

	// Demo API key usage
	demoAPIKeyUsage()

	// Wait for interrupt signal
	select {}
}

func demoAPIKeyUsage() {
	logger.Info("=== API Key Usage Demo ===")

	// Demo 1: Valid API key
	logger.Info("Demo 1: Using valid API key")
	if err := callWithAPIKey("demo-key-2024", "user.list", nil); err != nil {
		logger.Errorf("Call failed", zap.Error(err))
	}

	// Demo 2: Invalid API key
	logger.Info("Demo 2: Using invalid API key")
	if err := callWithAPIKey("invalid-key", "user.list", nil); err != nil {
		logger.Errorf("Call failed (expected)", zap.Error(err))
	}

	// Demo 3: No API key
	logger.Info("Demo 3: Using no API key")
	if err := callWithAPIKey("", "user.list", nil); err != nil {
		logger.Errorf("Call failed (expected)", zap.Error(err))
	}

	// Demo 4: Skip auth method (system.ping)
	logger.Info("Demo 4: Calling skip auth method")
	if err := callWithAPIKey("", "system.ping", nil); err != nil {
		logger.Errorf("Call failed", zap.Error(err))
	}
}

func callWithAPIKey(apiKey, method string, params interface{}) error {
	config := jsonrpc.DefaultClientConfig()

	// Add API key to headers
	if config.Headers == nil {
		config.Headers = make(map[string]string)
	}
	if apiKey != "" {
		config.Headers["X-API-Key"] = apiKey
	}

	client := jsonrpc.NewTracedClient(config)

	var result interface{}
	err := client.Call(context.Background(), method, params, &result)

	if err != nil {
		logger.Errorf("RPC call failed",
			zap.String("method", method),
			zap.String("api_key", apiKey),
			zap.Error(err))
		return err
	}

	logger.Info("RPC call successful",
		zap.String("method", method),
		zap.String("api_key", apiKey),
		zap.Any("result", result))

	return nil
}

// Example of programmatic API key management
func manageAPIKeys(server *rpc.JSONRPCServer) {
	logger.Info("=== API Key Management Demo ===")

	// Add new API key
	server.AddAPIKey("new-key-123", "new-user")
	logger.Info("Added new API key")

	// Check if key exists
	exists := server.GetAuthConfig().HasAPIKey("new-key-123")
	logger.Info("Key exists", zap.Bool("exists", exists))

	// Remove API key
	server.RemoveAPIKey("new-key-123")
	logger.Info("Removed API key")

	exists = server.GetAuthConfig().HasAPIKey("new-key-123")
	logger.Info("Key exists after removal", zap.Bool("exists", exists))

	// Enable/disable authentication
	logger.Info("Disabling authentication")
	server.DisableAuth()

	logger.Info("Re-enabling authentication")
	server.EnableAuth()
}
