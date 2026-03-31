package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alldev-run/golang-gin-rpc/pkg/auth/jwtx"
	"github.com/alldev-run/golang-gin-rpc/pkg/rbac"
	"github.com/gin-gonic/gin"
)

func TestRequirePermission_DirectPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)

	policy := rbac.NewPolicy(map[string][]string{
		"admin": {"user.write"},
	})

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("claims", &jwtx.Claims{Payload: map[string]string{"permissions": "user.read,user.write"}})
		c.Next()
	})
	r.Use(RequirePermission(policy, "user.write"))
	r.GET("/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestRequirePermission_ByRolePolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)

	policy := rbac.NewPolicy(map[string][]string{
		"auditor": {"audit.read"},
	})

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("claims", &jwtx.Claims{Payload: map[string]string{"roles": "auditor"}})
		c.Next()
	})
	r.Use(RequirePermission(policy, "audit.read"))
	r.GET("/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestRequirePermission_Forbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)

	policy := rbac.NewPolicy(map[string][]string{
		"user": {"profile.read"},
	})

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("claims", &jwtx.Claims{Payload: map[string]string{"roles": "user"}})
		c.Next()
	})
	r.Use(RequirePermission(policy, "profile.write"))
	r.GET("/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestRequireAnyPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)

	policy := rbac.NewPolicy(map[string][]string{
		"ops": {"svc.read", "svc.deploy"},
	})

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("claims", &jwtx.Claims{Payload: map[string]string{"roles": "ops"}})
		c.Next()
	})
	r.Use(RequireAnyPermission(policy, "svc.delete", "svc.deploy"))
	r.GET("/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}
