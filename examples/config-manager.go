package main

import (
	"fmt"
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

// ManagerConfig 配置管理器结构（扁平版）
type ManagerConfig struct {
	Host        string `yaml:"host"`
	Port        int    `yaml:"port"`
	ServiceName string `yaml:"service_name"`

	Protocols struct {
		HTTP    bool `yaml:"http"`
		HTTP2   bool `yaml:"http2"`
		GRPC    bool `yaml:"grpc"`
		JSONRPC bool `yaml:"jsonrpc"`

		Security struct {
			Auth struct {
				Enabled    bool              `yaml:"enabled"`
				Type       string            `yaml:"type"`
				HeaderName string            `yaml:"header_name"`
				QueryName  string            `yaml:"query_name"`
				APIKeys    map[string]string `yaml:"api_keys"`
			} `yaml:"auth"`
		} `yaml:"security"`
	} `yaml:"protocols"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("用法:")
		fmt.Println("  config-manager add <key> <description>  - 添加 API Key")
		fmt.Println("  config-manager remove <key>              - 删除 API Key")
		fmt.Println("  config-manager list                     - 列出所有 API Keys")
		fmt.Println("  config-manager enable                   - 启用认证")
		fmt.Println("  config-manager disable                  - 禁用认证")
		fmt.Println("  config-manager status                    - 查看状态")
		os.Exit(1)
	}

	command := os.Args[1]
	configFile := "./api/http-gateway/config/config.yaml"

	// 读取配置
	data, err := os.ReadFile(configFile)
	if err != nil {
		log.Fatalf("配置文件读取失败: %v", err)
	}

	var config ManagerConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		log.Fatalf("配置解析失败: %v", err)
	}

	switch command {
	case "add":
		if len(os.Args) < 4 {
			log.Fatal("用法: config-manager add <key> <description>")
		}
		key := os.Args[2]
		description := os.Args[3]

		if config.Protocols.Security.Auth.APIKeys == nil {
			config.Protocols.Security.Auth.APIKeys = make(map[string]string)
		}
		config.Protocols.Security.Auth.APIKeys[key] = description
		config.Protocols.Security.Auth.Enabled = true

		fmt.Printf("✅ 添加 API Key: %s -> %s\n", key, description)

	case "remove":
		if len(os.Args) < 3 {
			log.Fatal("用法: config-manager remove <key>")
		}
		key := os.Args[2]

		if _, exists := config.Protocols.Security.Auth.APIKeys[key]; !exists {
			log.Fatalf("API Key 不存在: %s", key)
		}

		delete(config.Protocols.Security.Auth.APIKeys, key)
		fmt.Printf("🗑️  删除 API Key: %s\n", key)

	case "list":
		fmt.Println("📋 API Keys 列表:")
		if len(config.Protocols.Security.Auth.APIKeys) == 0 {
			fmt.Println("  (无 API Keys)")
		} else {
			for key, desc := range config.Protocols.Security.Auth.APIKeys {
				fmt.Printf("  %s -> %s\n", key, desc)
			}
		}

	case "enable":
		config.Protocols.Security.Auth.Enabled = true
		fmt.Println("✅ 认证已启用")

	case "disable":
		config.Protocols.Security.Auth.Enabled = false
		fmt.Println("❌ 认证已禁用")

	case "status":
		fmt.Printf("🔐 认证状态: %v\n", config.Protocols.Security.Auth.Enabled)
		fmt.Printf("📋 API Keys 数量: %d\n", len(config.Protocols.Security.Auth.APIKeys))
		fmt.Printf("🔑 头部名称: %s\n", config.Protocols.Security.Auth.HeaderName)
		fmt.Printf("🔍 查询参数: %s\n", config.Protocols.Security.Auth.QueryName)

	default:
		log.Fatalf("未知命令: %s", command)
	}

	// 保存配置（如果是修改操作）
	if command == "add" || command == "remove" || command == "enable" || command == "disable" {
		newData, err := yaml.Marshal(config)
		if err != nil {
			log.Fatalf("配置序列化失败: %v", err)
		}

		if err := os.WriteFile(configFile, newData, 0644); err != nil {
			log.Fatalf("配置文件写入失败: %v", err)
		}

		fmt.Println("💾 配置已保存")
	}
}
