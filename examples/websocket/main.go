package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"alldev-gin-rpc/pkg/websocket"
	"github.com/gin-gonic/gin"
)

func main() {
	gin.SetMode(gin.ReleaseMode)

	metricsObserver := websocket.NewMetricsObserver()
	observer := websocket.NewCompositeObserver(metricsObserver, websocket.NewLoggingObserver())
	manager := websocket.NewManager()

	config := websocket.DefaultServerConfig()
	config.Observer = observer
	config.RequireAuth = true
	config.Authenticator = func(r *http.Request) (*websocket.AuthResult, error) {
		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			return nil, fmt.Errorf("missing user_id")
		}
		room := r.URL.Query().Get("room")
		if room == "" {
			room = "general"
		}
		return &websocket.AuthResult{
			ClientID: r.URL.Query().Get("client_id"),
			UserID:   userID,
			TenantID: "demo-tenant",
			Groups:   []string{room},
			Metadata: map[string]string{"room": room},
		}, nil
	}

	engine := gin.New()
	engine.GET("/ws", websocket.ManagedGinHandler(config, manager, func(ctx context.Context, conn *websocket.Conn) {
		identity := conn.Identity()
		_ = conn.SendJSON(ctx, map[string]interface{}{
			"type":          "welcome",
			"connection_id": identity.ConnectionID,
			"user_id":       identity.UserID,
			"groups":        identity.Groups,
		})
		for {
			var incoming map[string]interface{}
			if err := conn.ReceiveJSON(ctx, &incoming); err != nil {
				return
			}
			action, _ := incoming["action"].(string)
			switch action {
			case "join":
				if group, _ := incoming["group"].(string); group != "" {
					conn.AddGroups(group)
					_ = conn.SendJSON(ctx, map[string]interface{}{"type": "joined", "group": group})
				}
			case "group_broadcast":
				group, _ := incoming["group"].(string)
				message, _ := incoming["message"].(string)
				_ = manager.BroadcastToGroup(ctx, group, fmt.Sprintf("[group:%s][user:%s] %s", group, identity.UserID, message))
			case "user_message":
				targetUserID, _ := incoming["target_user_id"].(string)
				message, _ := incoming["message"].(string)
				_ = manager.BroadcastToUser(ctx, targetUserID, fmt.Sprintf("[private][from:%s] %s", identity.UserID, message))
			default:
				payload, _ := incoming["message"].(string)
				_ = conn.SendJSON(ctx, map[string]interface{}{
					"type":    "echo",
					"user_id": identity.UserID,
					"message": payload,
				})
			}
		}
	}))
	engine.GET("/metrics", func(c *gin.Context) {
		c.JSON(http.StatusOK, metricsObserver.Snapshot())
	})

	httpServer := &http.Server{
		Addr:    ":18080",
		Handler: engine,
	}

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("websocket example server error: %v\n", err)
		}
	}()

	fmt.Println("websocket example server started: http://127.0.0.1:18080")
	fmt.Println("metrics endpoint: http://127.0.0.1:18080/metrics")

	go runDemoClient()
	go runBroadcastDemo(manager)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
	_ = manager.CloseAll()
}

func runDemoClient() {
	time.Sleep(1200 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := websocket.NewClient(websocket.Config{
		URL:               "ws://127.0.0.1:18080/ws?user_id=user-1&client_id=demo-client&room=general",
		Origin:            "http://127.0.0.1:18080",
		AutoReconnect:     true,
		ReconnectInterval: time.Second,
		HeartbeatInterval: 15 * time.Second,
		HeartbeatMessage:  "heartbeat",
	})
	if err := client.Connect(ctx); err != nil {
		fmt.Printf("demo client connect error: %v\n", err)
		return
	}
	defer client.Close()

	var welcome map[string]interface{}
	if err := client.ReceiveJSON(ctx, &welcome); err == nil {
		fmt.Printf("demo client welcome: %+v\n", welcome)
	}

	_ = client.SendJSON(ctx, map[string]interface{}{"action": "group_broadcast", "group": "general", "message": "hello group"})

	var message map[string]interface{}
	if err := client.ReceiveJSON(ctx, &message); err == nil {
		fmt.Printf("demo client received: %+v\n", message)
	}
}

func runBroadcastDemo(manager *websocket.Manager) {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = manager.BroadcastText(ctx, "[system] periodic broadcast")
		cancel()
	}
}
