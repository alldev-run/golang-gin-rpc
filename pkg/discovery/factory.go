package discovery

import (
	"errors"
	"fmt"
	"time"

	"golang-gin-rpc/pkg/discovery/consul"
	"golang-gin-rpc/pkg/discovery/etcd"
)

// Config 统一定义注册中心配置
type Config struct {
	Type    string        // "consul" 或 "etcd"
	Addr    string        // 注册中心地址 "127.0.0.1:8500" 或 "127.0.0.1:2379"
	Timeout time.Duration // 连接超时时间
}

// NewDiscovery 根据配置返回具体的实现实例
func NewDiscovery(conf Config) (Discovery, error) {
	switch conf.Type {
	case "consul":
		// 调用 consul 目录下的初始化函数
		return consul.NewRegistry(conf.Addr)
	case "etcd":
		// 调用 etcd 目录下的初始化函数
		return etcd.NewRegistry(conf.Addr, conf.Timeout)
	case "":
		return nil, errors.New("discovery type is required")
	default:
		return nil, fmt.Errorf("unsupported discovery type: %s", conf.Type)
	}
}
