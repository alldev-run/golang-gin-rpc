package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRouteRegistryBuildAndMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	registry := NewRouteRegistry()
	registry.Use(func(c *gin.Context) {
		c.Header("X-Global-Middleware", "applied")
		c.Next()
	})
	registry.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "healthy")
	}, "health check")

	group := registry.Group("v1", "/v1")
	if registry.Group("v1", "/ignored") != group {
		t.Fatal("Group() did not return the existing group")
	}
	group.Use(func(c *gin.Context) {
		c.Header("X-Group-Middleware", "applied")
		c.Next()
	})
	group.GET("/items", func(c *gin.Context) {
		c.String(http.StatusOK, "items")
	}, "list items").Middleware(func(c *gin.Context) {
		c.Header("X-Route-Middleware", "applied")
		c.Next()
	})

	handler := registry.Build()
	for _, test := range []struct {
		path        string
		wantBody    string
		wantHeaders []string
	}{
		{path: "/health", wantBody: "healthy", wantHeaders: []string{"X-Global-Middleware"}},
		{path: "/v1/items", wantBody: "items", wantHeaders: []string{
			"X-Global-Middleware", "X-Group-Middleware", "X-Route-Middleware",
		}},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
		if response.Code != http.StatusOK || response.Body.String() != test.wantBody {
			t.Errorf("%s: response = %d %q", test.path, response.Code, response.Body.String())
		}
		for _, header := range test.wantHeaders {
			if response.Header().Get(header) != "applied" {
				t.Errorf("%s: middleware header %s was not applied", test.path, header)
			}
		}
	}
}

func TestRouteRegistryWildcardAndChainMiddlewares(t *testing.T) {
	gin.SetMode(gin.TestMode)
	registry := NewRouteRegistry()
	registry.Register("*", "/wild", func(c *gin.Context) {
		c.String(http.StatusOK, "wild")
	})
	handler := registry.Build()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPatch, "/wild", nil))
	if response.Code != http.StatusOK || response.Body.String() != "wild" {
		t.Fatalf("wildcard route response = %d %q", response.Code, response.Body.String())
	}

	engine := gin.New()
	engine.GET("/chain", ChainMiddlewares(func(c *gin.Context) {
		c.String(http.StatusOK, "chain")
	}, func(c *gin.Context) {
		c.Set("middleware", true)
		c.Next()
	}))
	response = httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/chain", nil))
	if response.Code != http.StatusOK || response.Body.String() != "chain" {
		t.Fatalf("chained middleware response = %d %q", response.Code, response.Body.String())
	}
}
