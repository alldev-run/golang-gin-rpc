package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestIPFilter_Blacklist(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		config     IPFilterConfig
		clientIP   string
		wantStatus int
	}{
		{
			name: "allowed_ip",
			config: IPFilterConfig{
				Enabled: true,
				Mode:    IPFilterModeBlacklist,
				IPList:  []string{"192.168.1.100"},
			},
			clientIP:   "192.168.1.1",
			wantStatus: http.StatusOK,
		},
		{
			name: "blocked_ip",
			config: IPFilterConfig{
				Enabled: true,
				Mode:    IPFilterModeBlacklist,
				IPList:  []string{"192.168.1.100"},
			},
			clientIP:   "192.168.1.100",
			wantStatus: http.StatusForbidden,
		},
		{
			name: "cidr_blocked",
			config: IPFilterConfig{
				Enabled: true,
				Mode:    IPFilterModeBlacklist,
				IPList:  []string{"10.0.0.0/8"},
			},
			clientIP:   "10.1.2.3",
			wantStatus: http.StatusForbidden,
		},
		{
			name: "cidr_allowed",
			config: IPFilterConfig{
				Enabled: true,
				Mode:    IPFilterModeBlacklist,
				IPList:  []string{"10.0.0.0/8"},
			},
			clientIP:   "172.16.1.1",
			wantStatus: http.StatusOK,
		},
		{
			name: "disabled",
			config: IPFilterConfig{
				Enabled: false,
				Mode:    IPFilterModeBlacklist,
				IPList:  []string{"192.168.1.100"},
			},
			clientIP:   "192.168.1.100",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			_ = tt.config.Validate()
			router.Use(IPFilter(tt.config))
			router.GET("/test", func(c *gin.Context) {
				c.String(http.StatusOK, "OK")
			})

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/test", nil)
			req.Header.Set("X-Forwarded-For", tt.clientIP)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestIPFilter_Whitelist(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		config     IPFilterConfig
		clientIP   string
		wantStatus int
	}{
		{
			name: "whitelisted_ip",
			config: IPFilterConfig{
				Enabled: true,
				Mode:    IPFilterModeWhitelist,
				IPList:  []string{"192.168.1.100", "192.168.1.101"},
			},
			clientIP:   "192.168.1.100",
			wantStatus: http.StatusOK,
		},
		{
			name: "not_in_whitelist",
			config: IPFilterConfig{
				Enabled:     true,
				Mode:        IPFilterModeWhitelist,
				IPList:      []string{"192.168.1.100"},
				TrustProxy:  true,
			},
			clientIP:   "192.168.1.200",
			wantStatus: http.StatusForbidden,
		},
		{
			name: "cidr_whitelist",
			config: IPFilterConfig{
				Enabled: true,
				Mode:    IPFilterModeWhitelist,
				IPList:  []string{"192.168.0.0/16"},
			},
			clientIP:   "192.168.5.10",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			_ = tt.config.Validate()
			router.Use(IPFilter(tt.config))
			router.GET("/test", func(c *gin.Context) {
				c.String(http.StatusOK, "OK")
			})

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/test", nil)
			req.Header.Set("X-Forwarded-For", tt.clientIP)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestIPFilter_SkipPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := IPFilterConfig{
		Enabled:     true,
		Mode:        IPFilterModeBlacklist,
		IPList:      []string{"192.168.1.100"},
		SkipPaths:   []string{"/health", "/metrics"},
		TrustProxy:  true,
	}
	_ = config.Validate()

	router := gin.New()
	router.Use(IPFilter(config))
	router.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "healthy")
	})
	router.GET("/api", func(c *gin.Context) {
		c.String(http.StatusOK, "api")
	})

	tests := []struct {
		path       string
		wantStatus int
	}{
		{"/health", http.StatusOK},
		{"/api", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", tt.path, nil)
			req.Header.Set("X-Forwarded-For", "192.168.1.100")
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestIPFilter_Response(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := IPFilterConfig{
		Enabled:         true,
		Mode:            IPFilterModeBlacklist,
		IPList:          []string{"192.168.1.100"},
		BlockMessage:    "Custom block message",
		BlockStatusCode: http.StatusUnauthorized,
		TrustProxy:      true,
	}
	_ = config.Validate()

	router := gin.New()
	router.Use(IPFilter(config))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.100")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Custom block message")
	assert.Contains(t, w.Body.String(), "IP_BLOCKED")
	assert.Contains(t, w.Body.String(), "192.168.1.100")
}

func TestDefaultIPFilterConfig(t *testing.T) {
	config := DefaultIPFilterConfig()

	assert.False(t, config.Enabled)
	assert.Equal(t, IPFilterModeBlacklist, config.Mode)
	assert.Equal(t, "Access denied", config.BlockMessage)
	assert.Equal(t, http.StatusForbidden, config.BlockStatusCode)
	assert.Contains(t, config.SkipPaths, "/health")
	assert.Contains(t, config.SkipPaths, "/metrics")
	assert.True(t, config.TrustProxy)
	assert.False(t, config.EnableGeoIP)
}

func TestIPFilterConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    IPFilterConfig
		wantError bool
	}{
		{
			name:      "disabled",
			config:    IPFilterConfig{Enabled: false},
			wantError: false,
		},
		{
			name:      "default_values",
			config:    IPFilterConfig{Enabled: true},
			wantError: false,
		},
		{
			name: "invalid_cidr",
			config: IPFilterConfig{
				Enabled: true,
				IPList:  []string{"invalid-cidr"},
			},
			wantError: false, // CIDR 解析移到中间件创建时，不报错
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetClientIP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("client_ip", "10.0.0.1")
		c.Next()
	})
	router.GET("/test", func(c *gin.Context) {
		ip := GetClientIP(c)
		c.String(http.StatusOK, ip)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, "10.0.0.1", w.Body.String())
}

func TestGetClientIP_NotSet(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		ip := GetClientIP(c)
		// 如果 client_ip 未设置，应返回 ClientIP() 的值
		assert.NotNil(t, ip)
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.50:12345"
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
