// Package geoip provides GeoIP2 database integration for IP geolocation
package geoip

import (
	"fmt"
	"net"
	"sync"

	"github.com/oschwald/geoip2-golang"
)

var (
	defaultManager *GeoIPManager
	defaultOnce    sync.Once
)

// GeoIPManager GeoIP 管理器
type GeoIPManager struct {
	db    *geoip2.Reader
	path  string
	mu    sync.RWMutex
}

// CountryInfo 国家信息
type CountryInfo struct {
	Code       string            // ISO 国家代码（如: CN, US）
	Name       string            // 国家名称
	Names      map[string]string // 多语言名称
	IsEU       bool              // 是否在欧盟
	Continent  string            // 大洲
}

// CityInfo 城市信息
type CityInfo struct {
	Country   CountryInfo
	City      string
	Subdivision string // 省/州
	PostalCode  string
	Latitude    float64
	Longitude   float64
	Timezone    string
}

// NewManager 创建 GeoIP 管理器
func NewManager(dbPath string) (*GeoIPManager, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("geoip database path is required")
	}

	db, err := geoip2.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open geoip database: %w", err)
	}

	return &GeoIPManager{
		db:   db,
		path: dbPath,
	}, nil
}

// InitDefaultManager 初始化默认管理器（单例）
func InitDefaultManager(dbPath string) error {
	var err error
	defaultOnce.Do(func() {
		defaultManager, err = NewManager(dbPath)
	})
	return err
}

// GetDefaultManager 获取默认管理器
func GetDefaultManager() *GeoIPManager {
	return defaultManager
}

// Close 关闭数据库
func (m *GeoIPManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.db != nil {
		return m.db.Close()
	}
	return nil
}

// GetCountry 获取国家信息
func (m *GeoIPManager) GetCountry(ip string) (string, error) {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return "", fmt.Errorf("invalid IP address: %s", ip)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.db == nil {
		return "", fmt.Errorf("geoip database not initialized")
	}

	record, err := m.db.Country(parsedIP)
	if err != nil {
		return "", fmt.Errorf("failed to lookup country: %w", err)
	}

	return record.Country.IsoCode, nil
}

// GetCountryInfo 获取详细国家信息
func (m *GeoIPManager) GetCountryInfo(ip string) (*CountryInfo, error) {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return nil, fmt.Errorf("invalid IP address: %s", ip)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.db == nil {
		return nil, fmt.Errorf("geoip database not initialized")
	}

	record, err := m.db.Country(parsedIP)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup country: %w", err)
	}

	return &CountryInfo{
		Code:      record.Country.IsoCode,
		Name:      record.Country.Names["en"],
		Names:     record.Country.Names,
		IsEU:      record.Country.IsInEuropeanUnion,
		Continent: record.Continent.Names["en"],
	}, nil
}

// GetCity 获取城市信息（需要 City 数据库）
func (m *GeoIPManager) GetCity(ip string) (*CityInfo, error) {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return nil, fmt.Errorf("invalid IP address: %s", ip)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.db == nil {
		return nil, fmt.Errorf("geoip database not initialized")
	}

	record, err := m.db.City(parsedIP)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup city: %w", err)
	}

	subdivision := ""
	if len(record.Subdivisions) > 0 {
		subdivision = record.Subdivisions[0].Names["en"]
	}

	return &CityInfo{
		Country: CountryInfo{
			Code:      record.Country.IsoCode,
			Name:      record.Country.Names["en"],
			Names:     record.Country.Names,
			IsEU:      record.Country.IsInEuropeanUnion,
			Continent: record.Continent.Names["en"],
		},
		City:        record.City.Names["en"],
		Subdivision: subdivision,
		PostalCode:  record.Postal.Code,
		Latitude:    record.Location.Latitude,
		Longitude:   record.Location.Longitude,
		Timezone:    record.Location.TimeZone,
	}, nil
}

// IsInCountry 检查 IP 是否属于指定国家
func (m *GeoIPManager) IsInCountry(ip string, countryCode string) bool {
	code, err := m.GetCountry(ip)
	if err != nil {
		return false
	}
	return code == countryCode
}

// IsInEU 检查 IP 是否属于欧盟国家
func (m *GeoIPManager) IsInEU(ip string) bool {
	info, err := m.GetCountryInfo(ip)
	if err != nil {
		return false
	}
	return info.IsEU
}

// IsPrivateIP 检查是否为私有 IP
func IsPrivateIP(ip string) bool {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	privateBlocks := []*net.IPNet{
		{IP: net.ParseIP("10.0.0.0"), Mask: net.CIDRMask(8, 32)},
		{IP: net.ParseIP("172.16.0.0"), Mask: net.CIDRMask(12, 32)},
		{IP: net.ParseIP("192.168.0.0"), Mask: net.CIDRMask(16, 32)},
		{IP: net.ParseIP("127.0.0.0"), Mask: net.CIDRMask(8, 32)},
		{IP: net.ParseIP("169.254.0.0"), Mask: net.CIDRMask(16, 32)},
		{IP: net.ParseIP("::1"), Mask: net.CIDRMask(128, 128)},
		{IP: net.ParseIP("fc00::"), Mask: net.CIDRMask(7, 128)},
	}

	for _, block := range privateBlocks {
		if block.Contains(parsedIP) {
			return true
		}
	}

	return false
}

// DefaultGetCountry 使用默认管理器获取国家代码
func DefaultGetCountry(ip string) (string, error) {
	if defaultManager == nil {
		return "", fmt.Errorf("default geoip manager not initialized")
	}
	return defaultManager.GetCountry(ip)
}
