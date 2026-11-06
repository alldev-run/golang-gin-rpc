package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alldev-run/golang-gin-rpc/plugins/gateway"
	"github.com/gin-gonic/gin"
)

func TestRequestIDMiddlewareConfigAndLogging(t *testing.T) {
	gin.SetMode(gin.TestMode)
	config := &gateway.Config{}
	engine := gin.New()
	engine.Use(RequestIDFromConfig(config))
	engine.GET("/request", func(c *gin.Context) {
		value, ok := c.Get("request_id")
		if !ok || value == "" {
			t.Error("request ID was not stored in the context")
		}
		c.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodGet, "/request", nil)
	request.Header.Set("X-Request-ID", "provided-id")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Header().Get("X-Request-ID") != "provided-id" {
		t.Fatalf("request ID response header = %q", response.Header().Get("X-Request-ID"))
	}

	loggingEngine := gin.New()
	loggingEngine.Use(LoggingWithConfig(LoggingConfig{
		LogRequestBody: true, LogResponseBody: true, LogHeaders: true,
		MaxBodySize: 8, SkipPaths: []string{"/skip"},
		EnableRequestID: true, SensitiveHeaders: []string{"Authorization"},
	}))
	loggingEngine.POST("/log", func(c *gin.Context) {
		body, err := c.GetRawData()
		if err != nil {
			t.Errorf("GetRawData() error = %v", err)
		}
		c.Data(http.StatusInternalServerError, "text/plain", append(body, []byte("-response-too-large")...))
	})
	loggingEngine.GET("/skip", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	request = httptest.NewRequest(http.MethodPost, "/log?verbose=true", strings.NewReader(`{"payload":"request-body-too-large"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "secret")
	response = httptest.NewRecorder()
	loggingEngine.ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError || response.Header().Get("X-Request-ID") == "" {
		t.Fatalf("logged response = %d, request ID %q", response.Code, response.Header().Get("X-Request-ID"))
	}

	response = httptest.NewRecorder()
	loggingEngine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/skip", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("skip-path response = %d", response.Code)
	}
}
