# GeoIP Package

提供 GeoIP2 数据库集成的 IP 地理位置查询功能。

## 功能特性

- 国家/城市信息查询
- 欧盟检测
- 私有 IP 检测
- 线程安全

## 使用方法

### 基本使用

```go
import "github.com/alldev-run/golang-gin-rpc/pkg/geoip"

// 创建管理器
manager, err := geoip.NewManager("/path/to/GeoLite2-Country.mmdb")
if err != nil {
    log.Fatal(err)
}
defer manager.Close()

// 查询国家代码
country, err := manager.GetCountry("8.8.8.8")
if err != nil {
    log.Fatal(err)
}
fmt.Println(country) // "US"

// 查询详细信息
info, err := manager.GetCountryInfo("8.8.8.8")
if err != nil {
    log.Fatal(err)
}
fmt.Println(info.Name)      // "United States"
fmt.Println(info.Continent) // "North America"
```

### 单例模式

```go
// 初始化默认管理器
err := geoip.InitDefaultManager("/path/to/GeoLite2-Country.mmdb")
if err != nil {
    log.Fatal(err)
}

// 在其他地方使用
country, err := geoip.DefaultGetCountry("8.8.8.8")
```

### 城市数据库

```go
// 需要 GeoLite2-City.mmdb
city, err := manager.GetCity("8.8.8.8")
if err != nil {
    log.Fatal(err)
}
fmt.Println(city.City)     // "Mountain View"
fmt.Println(city.Latitude)  // 37.386
```

## 获取 GeoIP2 数据库

1. 注册 [MaxMind 账号](https://www.maxmind.com/en/geolite2/signup)
2. 下载 GeoLite2 免费数据库
3. 或使用付费版 GeoIP2 获得更精确数据

## 数据库文件

- **GeoLite2-Country.mmdb** - 国家信息（必需）
- **GeoLite2-City.mmdb** - 城市信息（可选）

## 性能

- 数据库加载到内存，查询 O(1)
- 线程安全，支持高并发
- 自动处理 IPv4/IPv6
