# IP 过滤与 GeoIP2 使用指南

本文档介绍 golang-gin-rpc 框架的 IP 黑白名单过滤和 GeoIP2 地理位置拦截功能。

## 功能特性

- **IP 黑白名单**：支持单个 IP 和 CIDR 网段过滤
- **国家代码过滤**：基于 GeoIP2 数据库的国家级别拦截
- **代理支持**：自动识别 X-Forwarded-For 和 X-Real-IP 头
- **路径例外**：可配置跳过特定路径（如健康检查）
- **灵活模式**：黑名单模式（默认）或白名单模式

## 快速开始

### 1. 基础 IP 黑名单

```yaml
# configs/config.yaml
gateway:
  ip_filter:
    enabled: true
    mode: "blacklist"
    ip_list:
      - "192.168.1.100"           # 单个 IP
      - "10.0.0.0/8"              # CIDR 网段
      - "172.16.0.0/12"
    block_message: "Your IP has been blocked"
    block_status_code: 403
```

### 2. IP 白名单模式

```yaml
gateway:
  ip_filter:
    enabled: true
    mode: "whitelist"              # 只允许列表内的 IP
    ip_list:
      - "192.168.1.0/24"          # 只允许内网
      - "10.0.0.0/8"
    skip_paths:
      - "/health"                  # 健康检查不拦截
      - "/metrics"
```

### 3. GeoIP 国家拦截

首先下载 [GeoLite2 免费数据库](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data)：

```bash
# 下载并解压到指定目录
wget https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-Country -O GeoLite2-Country.tar.gz
tar -xzf GeoLite2-Country.tar.gz
mv GeoLite2-Country_*/GeoLite2-Country.mmdb /etc/geoip/
```

配置 GeoIP 拦截：

```yaml
gateway:
  ip_filter:
    enabled: true
    enable_geoip: true
    geoip_db_path: "/etc/geoip/GeoLite2-Country.mmdb"
    
    # 黑名单模式：拦截指定国家
    country_blacklist:
      - "CN"    # 中国
      - "RU"    # 俄罗斯
      - "KP"    # 朝鲜
    
    # 或白名单模式：只允许指定国家
    # country_whitelist:
    #   - "US"
    #   - "GB"
    #   - "DE"
```

## 代码中使用

### 直接使用中间件

```go
package main

import (
    "github.com/alldev-run/golang-gin-rpc/pkg/middleware"
    "github.com/gin-gonic/gin"
)

func main() {
    r := gin.Default()

    // 方法1: 简单黑名单
    config := middleware.IPFilterConfig{
        Enabled: true,
        Mode:    middleware.IPFilterModeBlacklist,
        IPList:  []string{"192.168.1.100", "10.0.0.0/8"},
    }
    r.Use(middleware.IPFilter(config))

    // 方法2: 带 GeoIP 的过滤
    geoConfig := middleware.IPFilterConfig{
        Enabled:          true,
        EnableGeoIP:      true,
        GeoIPDBPath:      "/etc/geoip/GeoLite2-Country.mmdb",
        CountryBlacklist: []string{"CN", "RU"},
    }
    _ = geoConfig.InitGeoIP()  // 初始化 GeoIP
    r.Use(middleware.IPFilter(geoConfig))

    r.GET("/api/hello", func(c *gin.Context) {
        c.JSON(200, gin.H{"message": "Hello!"})
    })

    r.Run(":8080")
}
```

### 从配置加载

```go
package main

import (
    "github.com/alldev-run/golang-gin-rpc/pkg/middleware"
)

func setupMiddleware() {
    // 从 YAML 配置加载
    config := middleware.DefaultIPFilterConfig()
    
    // 启用并配置
    config.Enabled = true
    config.Mode = middleware.IPFilterModeBlacklist
    config.IPList = []string{"192.168.1.100"}
    
    // 如果启用 GeoIP，初始化数据库
    if config.EnableGeoIP {
        if err := config.InitGeoIP(); err != nil {
            log.Fatal("Failed to init GeoIP:", err)
        }
    }
}
```

### 获取客户端 IP

```go
func handler(c *gin.Context) {
    // 方法1: 从中间件获取（推荐）
    ip := middleware.GetClientIP(c)
    
    // 方法2: 直接使用 Gin
    ip := c.ClientIP()
    
    // 方法3: 从上下文获取国家（需启用 GeoIP）
    country := middleware.GetCountry(c)
    
    c.JSON(200, gin.H{
        "ip":      ip,
        "country": country,
    })
}
```

## GeoIP2 包使用

直接使用 `pkg/geoip` 进行 IP 地理位置查询：

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/alldev-run/golang-gin-rpc/pkg/geoip"
)

func main() {
    // 创建管理器
    manager, err := geoip.NewManager("/etc/geoip/GeoLite2-Country.mmdb")
    if err != nil {
        log.Fatal(err)
    }
    defer manager.Close()

    // 查询国家代码
    country, err := manager.GetCountry("8.8.8.8")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(country)  // "US"

    // 查询详细信息
    info, err := manager.GetCountryInfo("8.8.8.8")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Country: %s\n", info.Name)      // "United States"
    fmt.Printf("Continent: %s\n", info.Continent) // "North America"
    fmt.Printf("Is EU: %v\n", info.IsEU)        // false

    // 检查私有 IP
    fmt.Println(geoip.IsPrivateIP("192.168.1.1"))  // true
    fmt.Println(geoip.IsPrivateIP("8.8.8.8"))      // false
}
```

### 城市级查询（需要 City 数据库）

```go
// 需要 GeoLite2-City.mmdb
city, err := manager.GetCity("8.8.8.8")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("City: %s\n", city.City)              // "Mountain View"
fmt.Printf("Region: %s\n", city.Subdivision)   // "California"
fmt.Printf("Postal: %s\n", city.PostalCode)     // "94035"
fmt.Printf("Lat: %f, Lng: %f\n", city.Latitude, city.Longitude)
```

### 单例模式

```go
// 全局初始化
func init() {
    err := geoip.InitDefaultManager("/etc/geoip/GeoLite2-Country.mmdb")
    if err != nil {
        log.Fatal(err)
    }
}

// 在其他地方使用
func someFunction() {
    country, err := geoip.DefaultGetCountry("8.8.8.8")
    if err != nil {
        // 处理错误
    }
    // ...
}
```

## 配置选项详解

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `enabled` | bool | false | 是否启用 IP 过滤 |
| `mode` | string | "blacklist" | 过滤模式："blacklist" 或 "whitelist" |
| `ip_list` | []string | [] | IP 列表，支持 CIDR 格式 |
| `country_blacklist` | []string | [] | 国家代码黑名单（ISO 3166-1 alpha-2） |
| `country_whitelist` | []string | [] | 国家代码白名单（优先于黑名单） |
| `enable_geoip` | bool | false | 启用 GeoIP2 查询 |
| `geoip_db_path` | string | "" | GeoIP2 数据库文件路径 |
| `block_message` | string | "Access denied" | 拦截时返回的消息 |
| `block_status_code` | int | 403 | 拦截时的 HTTP 状态码 |
| `skip_paths` | []string | ["/health", "/metrics", "/ping"] | 跳过的路径 |
| `trust_proxy` | bool | true | 信任 X-Forwarded-For 头 |

## 企业级使用建议

### 1. 配置文件分离

```yaml
# configs/ip_filter_production.yaml
gateway:
  ip_filter:
    enabled: true
    mode: "whitelist"
    ip_list:
      - "10.0.0.0/8"        # 内网
      - "172.16.0.0/12"
      - "192.168.0.0/16"
    country_whitelist:
      - "US"
      - "GB"
      - "DE"
      - "FR"
      - "JP"
    enable_geoip: true
    geoip_db_path: "/etc/geoip/GeoLite2-Country.mmdb"
    trust_proxy: true
```

### 2. 动态更新 IP 列表

```go
// 从外部服务加载黑名单
func loadDynamicBlacklist() []string {
    // 从数据库或配置中心加载
    // 如 Redis、Etcd、数据库等
    return fetchFromRedis("ip:blacklist")
}

// 定期更新
config.IPList = loadDynamicBlacklist()
```

### 3. 与限流结合使用

```go
// IP 过滤 + 限流组合
r.Use(middleware.IPFilter(ipConfig))
r.Use(middleware.RateLimiter(rateConfig))
```

### 4. 日志记录

```go
func ipFilterWithLogging(config middleware.IPFilterConfig) gin.HandlerFunc {
    filter := middleware.IPFilter(config)
    
    return func(c *gin.Context) {
        ip := c.ClientIP()
        
        // 记录所有请求
        log.Printf("[IPFilter] Request from %s to %s", ip, c.Request.URL.Path)
        
        filter(c)
        
        // 如果被拦截，记录日志
        if c.IsAborted() {
            log.Printf("[IPFilter] Blocked IP: %s", ip)
        }
    }
}
```

## 获取 GeoIP2 数据库

### 免费版（GeoLite2）

1. 注册 [MaxMind 账号](https://www.maxmind.com/en/geolite2/signup)
2. 生成 License Key
3. 下载数据库：

```bash
# 使用 MaxMind 的 geoipupdate 工具
# 或手动下载
curl "https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-Country&license_key=YOUR_KEY&suffix=tar.gz" -o GeoLite2-Country.tar.gz
```

### 付费版（GeoIP2）

付费版提供更高精度和更多字段：
- 更精确的城市定位
- ISP/组织信息
- 匿名代理检测
- 威胁情报数据

## 故障排查

### GeoIP 数据库加载失败

```
Error: failed to open geoip database
```

**解决**：
1. 检查文件路径是否正确
2. 确认文件存在且有读取权限
3. 验证数据库文件未损坏

### IP 未被正确拦截

```
客户端 IP: 192.168.1.100
配置 IPList: ["192.168.1.100"]
但未被拦截
```

**排查**：
1. 检查 `trust_proxy` 设置
2. 确认 X-Forwarded-For 头值
3. 查看实际获取的 IP：
   ```go
   fmt.Println(c.GetHeader("X-Forwarded-For"))
   fmt.Println(c.ClientIP())
   ```

### 性能问题

GeoIP 查询是内存操作，性能很高：
- 单次查询 < 1μs
- 支持 100K+ QPS

如需更高性能，可启用本地缓存：

```go
// 使用 LRU 缓存国家查询结果
var countryCache = cache.NewLRU(10000)

func getCachedCountry(ip string) string {
    if v, ok := countryCache.Get(ip); ok {
        return v.(string)
    }
    country, _ := manager.GetCountry(ip)
    countryCache.Set(ip, country)
    return country
}
```

## 相关文档

- [Gateway 配置指南](./gateway.md)
- [生产环境部署](./production-deployment.md)
- [API 使用指南](./API_USAGE_GUIDE.md)

## API 参考

### IPFilterConfig

```go
type IPFilterConfig struct {
    Enabled          bool
    Mode             IPFilterMode  // "blacklist" | "whitelist"
    IPList           []string      // 支持 CIDR
    CountryBlacklist []string      // ISO 国家代码
    CountryWhitelist []string
    EnableGeoIP      bool
    GeoIPDBPath      string
    BlockMessage     string
    BlockStatusCode  int
    SkipPaths        []string
    TrustProxy       bool
}
```

### 函数

```go
// 创建中间件
func IPFilter(config IPFilterConfig) gin.HandlerFunc

// 获取客户端 IP
func GetClientIP(c *gin.Context) string

// 获取国家代码（需启用 GeoIP）
func GetCountry(c *gin.Context) string

// GeoIP 管理器
func NewManager(dbPath string) (*GeoIPManager, error)
func (m *GeoIPManager) GetCountry(ip string) (string, error)
func (m *GeoIPManager) GetCountryInfo(ip string) (*CountryInfo, error)
func (m *GeoIPManager) GetCity(ip string) (*CityInfo, error)
func IsPrivateIP(ip string) bool
```
