package middleware

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/alldev-run/golang-gin-rpc/pkg/geoip"
)

// IPFilterMode 定义过滤模式
type IPFilterMode string

const (
	// IPFilterModeBlacklist 黑名单模式（默认）
	IPFilterModeBlacklist IPFilterMode = "blacklist"
	// IPFilterModeWhitelist 白名单模式
	IPFilterModeWhitelist IPFilterMode = "whitelist"
)

// IPFilterConfig IP 过滤器配置
type IPFilterConfig struct {
	// Enabled 是否启用
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Mode 过滤模式: blacklist | whitelist
	Mode IPFilterMode `yaml:"mode" json:"mode"`

	// IPList IP 列表（支持 CIDR 如 192.168.1.0/24）
	IPList []string `yaml:"ip_list" json:"ip_list"`

	// CountryBlacklist 国家代码黑名单（如: CN, US, RU）
	CountryBlacklist []string `yaml:"country_blacklist" json:"country_blacklist"`

	// CountryWhitelist 国家代码白名单
	CountryWhitelist []string `yaml:"country_whitelist" json:"country_whitelist"`

	// EnableGeoIP 是否启用 GeoIP 检测
	EnableGeoIP bool `yaml:"enable_geoip" json:"enable_geoip"`

	// GeoIPDBPath GeoIP2 数据库路径
	GeoIPDBPath string `yaml:"geoip_db_path" json:"geoip_db_path"`

	// BlockMessage 拦截时返回的消息
	BlockMessage string `yaml:"block_message" json:"block_message"`

	// BlockStatusCode 拦截时的 HTTP 状态码
	BlockStatusCode int `yaml:"block_status_code" json:"block_status_code"`

	// SkipPaths 跳过的路径
	SkipPaths []string `yaml:"skip_paths" json:"skip_paths"`

	// TrustProxy 是否信任代理头（X-Forwarded-For）
	TrustProxy bool `yaml:"trust_proxy" json:"trust_proxy"`

	// ipNets 解析后的 IP 网段（运行时填充）
	ipNets []*net.IPNet

	// geoip geoip 实例（运行时填充）
	geoip *geoip.GeoIPManager
}

// DefaultIPFilterConfig 返回默认配置
func DefaultIPFilterConfig() IPFilterConfig {
	return IPFilterConfig{
		Enabled:         false,
		Mode:            IPFilterModeBlacklist,
		BlockMessage:    "Access denied",
		BlockStatusCode: http.StatusForbidden,
		SkipPaths:       []string{"/health", "/metrics", "/ping"},
		TrustProxy:      true,
		EnableGeoIP:     false,
	}
}

// InitGeoIP 初始化 GeoIP
func (c *IPFilterConfig) InitGeoIP() error {
	if !c.EnableGeoIP {
		return nil
	}

	var err error
	c.geoip, err = geoip.NewManager(c.GeoIPDBPath)
	return err
}

// ipFilterMiddleware IP 过滤器中间件
type ipFilterMiddleware struct {
	config IPFilterConfig
}

// newIPFilterMiddleware 创建中间件
func newIPFilterMiddleware(config IPFilterConfig) *ipFilterMiddleware {
	// 解析 CIDR 并保留精确 IP
	for _, ip := range config.IPList {
		if strings.Contains(ip, "/") {
			_, ipNet, err := net.ParseCIDR(ip)
			if err == nil {
				config.ipNets = append(config.ipNets, ipNet)
			}
		}
	}
	return &ipFilterMiddleware{config: config}
}

// shouldSkip 检查是否应该跳过
func (m *ipFilterMiddleware) shouldSkip(path string) bool {
	for _, skip := range m.config.SkipPaths {
		if strings.HasPrefix(path, skip) {
			return true
		}
	}
	return false
}

// getClientIP 获取客户端 IP
func (m *ipFilterMiddleware) getClientIP(c *gin.Context) string {
	if m.config.TrustProxy {
		// 尝试从 X-Forwarded-For 获取
		if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
			ips := strings.Split(xff, ",")
			if len(ips) > 0 {
				return strings.TrimSpace(ips[0])
			}
		}

		// 尝试 X-Real-IP
		if xri := c.GetHeader("X-Real-Ip"); xri != "" {
			return xri
		}
	}

	// 使用 Gin 的 ClientIP
	return c.ClientIP()
}

// isInIPList 检查 IP 是否在列表中（支持 CIDR）
func (m *ipFilterMiddleware) isInIPList(ip string) bool {
	clientIP := net.ParseIP(ip)
	if clientIP == nil {
		return false
	}

	// 检查精确匹配
	for _, allowedIP := range m.config.IPList {
		if allowedIP == ip {
			return true
		}
	}

	// 检查 CIDR 匹配
	for _, ipNet := range m.config.ipNets {
		if ipNet.Contains(clientIP) {
			return true
		}
	}

	return false
}

// isCountryBlocked 检查国家是否被拦截
func (m *ipFilterMiddleware) isCountryBlocked(ip string) (bool, string) {
	if !m.config.EnableGeoIP || m.config.geoip == nil {
		return false, ""
	}

	country, err := m.config.geoip.GetCountry(ip)
	if err != nil {
		return false, ""
	}

	// 白名单模式
	if len(m.config.CountryWhitelist) > 0 {
		for _, c := range m.config.CountryWhitelist {
			if strings.EqualFold(c, country) {
				return false, country
			}
		}
		return true, country
	}

	// 黑名单模式
	for _, c := range m.config.CountryBlacklist {
		if strings.EqualFold(c, country) {
			return true, country
		}
	}

	return false, country
}

// isBlocked 检查是否应该拦截
func (m *ipFilterMiddleware) isBlocked(ip string) (bool, string) {
	// 先检查 GeoIP
	if m.config.EnableGeoIP {
		blocked, country := m.isCountryBlocked(ip)
		if blocked {
			return true, fmt.Sprintf("country %s blocked", country)
		}
	}

	// 检查 IP 列表
	inList := m.isInIPList(ip)

	switch m.config.Mode {
	case IPFilterModeWhitelist:
		// 白名单模式：不在列表中的被拦截
		if !inList {
			return true, "not in whitelist"
		}
	case IPFilterModeBlacklist:
		// 黑名单模式：在列表中的被拦截
		if inList {
			return true, "in blacklist"
		}
	}

	return false, ""
}

// Handle Gin 中间件处理函数
func (m *ipFilterMiddleware) Handle(c *gin.Context) {
	if !m.config.Enabled {
		c.Next()
		return
	}

	// 跳过指定路径
	if m.shouldSkip(c.Request.URL.Path) {
		c.Next()
		return
	}

	// 获取客户端 IP
	clientIP := m.getClientIP(c)

	// 检查是否应该拦截
	blocked, reason := m.isBlocked(clientIP)
	if blocked {
		c.Writer.Header().Set("Content-Type", "application/json")
		c.Writer.WriteHeader(m.config.BlockStatusCode)
		c.Abort()
		resp := map[string]interface{}{
			"error":   m.config.BlockMessage,
			"code":    "IP_BLOCKED",
			"ip":      clientIP,
			"reason":  reason,
		}
		json.NewEncoder(c.Writer).Encode(resp)
		return
	}

	// 将 IP 信息存入上下文
	c.Set("client_ip", clientIP)

	c.Next()
}

// IPFilter 创建 IP 过滤器中间件
func IPFilter(config IPFilterConfig) gin.HandlerFunc {
	middleware := newIPFilterMiddleware(config)
	return middleware.Handle
}

// IPFilterWithGeoIP 创建带 GeoIP 的过滤器（需要先调用 config.InitGeoIP()）
func IPFilterWithGeoIP(config IPFilterConfig) gin.HandlerFunc {
	return IPFilter(config)
}

// GetClientIP 从上下文获取客户端 IP
func GetClientIP(c *gin.Context) string {
	if ip, exists := c.Get("client_ip"); exists {
		if s, ok := ip.(string); ok {
			return s
		}
	}
	return c.ClientIP()
}

// GetCountry 从上下文获取国家代码（需启用 GeoIP）
func GetCountry(c *gin.Context) string {
	if country, exists := c.Get("client_country"); exists {
		if s, ok := country.(string); ok {
			return s
		}
	}
	return ""
}
