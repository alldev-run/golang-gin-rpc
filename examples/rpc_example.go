package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"alldev-gin-rpc/pkg/logger"
	"alldev-gin-rpc/pkg/rpc"
	"alldev-gin-rpc/pkg/rpc/examples"
	"alldev-gin-rpc/pkg/rpc/grpc"
	"alldev-gin-rpc/pkg/rpc/jsonrpc"
)

func main() {
	// Initialize logger
	logger.Init(logger.Config{
		Level:   "info",
		Env:     "dev",
		LogPath: "./logs/rpc_example.log",
	})

	logger.Info("Starting RPC Example Application")

	// Create RPC manager with default configuration
	config := rpc.DefaultManagerConfig()
	manager := rpc.NewManager(config)

	// Add some middleware
	authMiddleware := rpc.NewMiddleware("auth", func(ctx context.Context, req interface{}) (interface{}, error) {
		logger.Info("Auth middleware", zap.Any("request", req))
		return req, nil // Pass through for demo
	})

	loggingMiddleware := rpc.NewMiddleware("logging", func(ctx context.Context, req interface{}) (interface{}, error) {
		logger.Info("Request received", zap.Any("request", req))
		return req, nil // Pass through
	})

	manager.AddMiddleware(authMiddleware)
	manager.AddMiddleware(loggingMiddleware)

	// Create and register services
	userService := examples.NewUserService()
	calculatorService := examples.NewCalculatorService()
	echoService := examples.NewEchoService()
	systemService := rpc.NewSystemService()

	if err := manager.RegisterService(userService); err != nil {
		log.Fatalf("Failed to register user service: %v", err)
	}

	if err := manager.RegisterService(calculatorService); err != nil {
		log.Fatalf("Failed to register calculator service: %v", err)
	}

	if err := manager.RegisterService(echoService); err != nil {
		log.Fatalf("Failed to register echo service: %v", err)
	}

	if err := manager.RegisterService(systemService); err != nil {
		log.Fatalf("Failed to register system service: %v", err)
	}

	// Start the RPC manager
	if err := manager.Start(); err != nil {
		log.Fatalf("Failed to start RPC manager: %v", err)
	}

	logger.Info("RPC servers started successfully")
	logger.Info("gRPC server: localhost:50051")
	logger.Info("JSON-RPC server: localhost:8080")

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Start demo client in background
	go func() {
		time.Sleep(2 * time.Second) // Wait for servers to start
		runDemoClients()
	}()

	<-quit
	logger.Info("Shutting down RPC servers...")

	// Graceful shutdown
	if err := manager.Stop(); err != nil {
		logger.Errorf("Error during shutdown", zap.Error(err))
	} else {
		logger.Info("RPC servers stopped successfully")
	}
}

func runDemoClients() {
	logger.Info("Running demo clients...")

	// Demo gRPC client
	go func() {
		if err := demoGRPCClient(); err != nil {
			logger.Errorf("gRPC client demo failed", zap.Error(err))
		}
	}()

	// Demo JSON-RPC client
	go func() {
		if err := demoJSONRPCClient(); err != nil {
			logger.Errorf("JSON-RPC client demo failed", zap.Error(err))
		}
	}()
}

func demoGRPCClient() {
	logger.Info("Demo gRPC client")

	config := grpc.DefaultClientConfig()
	client, err := grpc.NewClient(config)
	if err != nil {
		logger.Errorf("Failed to create gRPC client", zap.Error(err))
		return
	}
	defer client.Close()

	logger.Info("gRPC client connected", zap.String("address", client.Address()))

	// In a real implementation, you would use the generated gRPC client stubs
	// For this demo, we'll just show the connection
	logger.Info("gRPC client demo completed")
}

func demoJSONRPCClient() {
	logger.Info("Demo JSON-RPC client")

	config := jsonrpc.DefaultClientConfig()
	client := jsonrpc.NewClient(config)

	// Test system.ping
	var pingResult interface{}
	err := client.Call(context.Background(), "system.ping", nil, &pingResult)
	if err != nil {
		logger.Errorf("JSON-RPC ping failed", zap.Error(err))
	} else {
		logger.Info("JSON-RPC ping successful", zap.Any("result", pingResult))
	}

	// Test calculator.add
	addReq := &examples.AddRequest{
		Operand1: 10.5,
		Operand2: 20.3,
	}
	var addResp examples.AddResponse
	err = client.Call(context.Background(), "calculator.add", addReq, &addResp)
	if err != nil {
		logger.Errorf("JSON-RPC add failed", zap.Error(err))
	} else {
		logger.Info("JSON-RPC add successful", zap.Float64("result", addResp.Result))
	}

	// Test calculator.multiply
	mulReq := &examples.MultiplyRequest{
		Operand1: 5.0,
		Operand2: 6.0,
	}
	var mulResp examples.MultiplyResponse
	err = client.Call(context.Background(), "calculator.multiply", mulReq, &mulResp)
	if err != nil {
		logger.Errorf("JSON-RPC multiply failed", zap.Error(err))
	} else {
		logger.Info("JSON-RPC multiply successful", zap.Float64("result", mulResp.Result))
	}

	// Test echo.echo
	echoReq := &examples.EchoRequest{
		Message: "Hello, JSON-RPC!",
		Times:   3,
	}
	var echoResp examples.EchoResponse
	err = client.Call(context.Background(), "echo.echo", echoReq, &echoResp)
	if err != nil {
		logger.Errorf("JSON-RPC echo failed", zap.Error(err))
	} else {
		logger.Info("JSON-RPC echo successful", 
			zap.String("message", echoResp.Message),
			zap.Int("count", echoResp.Count))
	}

	// Test echo.reverse
	reverseReq := &examples.ReverseRequest{
		Text: "Hello, World!",
	}
	var reverseResp examples.ReverseResponse
	err = client.Call(context.Background(), "echo.reverse", reverseReq, &reverseResp)
	if err != nil {
		logger.Errorf("JSON-RPC reverse failed", zap.Error(err))
	} else {
		logger.Info("JSON-RPC reverse successful", zap.String("reversed", reverseResp.Reversed))
	}

	// Test calculator.history
	historyReq := &examples.HistoryRequest{
		Limit: 5,
	}
	var historyResp examples.HistoryResponse
	err = client.Call(context.Background(), "calculator.getHistory", historyReq, &historyResp)
	if err != nil {
		logger.Errorf("JSON-RPC getHistory failed", zap.Error(err))
	} else {
		logger.Info("JSON-RPC getHistory successful", 
			zap.Int("total", historyResp.Total),
			zap.Int("returned", len(historyResp.History)))
	}

	logger.Info("JSON-RPC client demo completed")
}

// Example of creating a custom service
type CustomService struct {
	*rpc.BaseService
}

func NewCustomService() *CustomService {
	return &CustomService{
		BaseService: rpc.NewBaseService("custom"),
	}
}

func (s *CustomService) Register(server interface{}) error {
	s.SetMetadata("custom_service", true)
	return nil
}

func (s *CustomService) HelloWorld(ctx context.Context, req interface{}) (interface{}, error) {
	return map[string]interface{}{
		"message": "Hello, World!",
		"service": s.Name(),
		"time":    time.Now().Unix(),
	}, nil
}

func (s *CustomService) GetInfo(ctx context.Context, req interface{}) (interface{}, error) {
	return map[string]interface{}{
		"name":        s.Name(),
		"uptime":      s.Uptime().String(),
		"started_at":  s.StartTime(),
		"metadata":    s.GetAllMetadata(),
	}, nil
}
