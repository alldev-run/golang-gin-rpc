package db

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/db/mysql"
	"github.com/alldev-run/golang-gin-rpc/pkg/cache/redis"
	"github.com/alldev-run/golang-gin-rpc/pkg/db/postgres"
)

// TestFactoryComplete 综合测试 Factory 的所有功能
// 包括配置加载、客户端创建、存储和检索
func TestFactoryComplete(t *testing.T) {
	// 子测试 1: 基本配置测试
	t.Run("BasicConfig", func(t *testing.T) {
		cfg := Config{
			Type: TypeMySQL,
			MySQL: mysql.Config{
				Host: "localhost",
				Port: 3306,
			},
		}

		if cfg.Type != TypeMySQL {
			t.Errorf("Config Type = %v, want mysql", cfg.Type)
		}
		if cfg.MySQL.Host != "localhost" {
			t.Errorf("MySQL Host = %v, want localhost", cfg.MySQL.Host)
		}
		if cfg.MySQL.Port != 3306 {
			t.Errorf("MySQL Port = %v, want 3306", cfg.MySQL.Port)
		}
	})

	// 子测试 2: YAML 配置加载测试
	t.Run("YAMLConfigLoading", func(t *testing.T) {
		yamlContent := `
type: mysql
mysql:
  host: "127.0.0.1"
  port: 3307
  database: "testdb"
  username: "testuser"
  password: "testpass"
`

		tmpDir := t.TempDir()
		tmpFile := filepath.Join(tmpDir, "config.yaml")

		if err := os.WriteFile(tmpFile, []byte(yamlContent), 0644); err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}

		cfg, err := LoadConfigFromYAML(tmpFile)
		if err != nil {
			t.Fatalf("LoadConfigFromYAML() error = %v", err)
		}

		if cfg.Type != TypeMySQL {
			t.Errorf("Config Type = %v, want mysql", cfg.Type)
		}
		if cfg.MySQL.Host != "127.0.0.1" {
			t.Errorf("MySQL Host = %v, want 127.0.0.1", cfg.MySQL.Host)
		}
		if cfg.MySQL.Port != 3307 {
			t.Errorf("MySQL Port = %v, want 3307", cfg.MySQL.Port)
		}
	})

	// 子测试 3: JSON 配置加载测试
	t.Run("JSONConfigLoading", func(t *testing.T) {
		jsonContent := `{
  "type": "redis",
  "redis": {
    "host": "127.0.0.1",
    "port": 6380,
    "database": 1
  }
}`

		tmpDir := t.TempDir()
		tmpFile := filepath.Join(tmpDir, "config.json")

		if err := os.WriteFile(tmpFile, []byte(jsonContent), 0644); err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}

		cfg, err := LoadConfigFromJSON(tmpFile)
		if err != nil {
			t.Fatalf("LoadConfigFromJSON() error = %v", err)
		}

		if cfg.Type != TypeRedis {
			t.Errorf("Config Type = %v, want redis", cfg.Type)
		}
		if cfg.Redis.Host != "127.0.0.1" {
			t.Errorf("Redis Host = %v, want 127.0.0.1", cfg.Redis.Host)
		}
		if cfg.Redis.Port != 6380 {
			t.Errorf("Redis Port = %v, want 6380", cfg.Redis.Port)
		}
	})

	// 子测试 4: Factory 初始化测试
	t.Run("FactoryInitialization", func(t *testing.T) {
		factory := NewFactory()
		if factory == nil {
			t.Error("NewFactory() returned nil")
		}
		if factory.clients == nil {
			t.Error("Factory clients map is nil")
		}
	})

	// 子测试 5: 无效类型创建测试
	t.Run("InvalidTypeCreation", func(t *testing.T) {
		factory := NewFactory()
		cfg := Config{Type: "invalid"}

		_, err := factory.Create(cfg)
		if err == nil {
			t.Error("Create() with invalid type should return error")
		}
	})

	// 子测试 6: 空类型创建测试
	t.Run("EmptyTypeCreation", func(t *testing.T) {
		factory := NewFactory()
		cfg := Config{Type: ""}

		_, err := factory.Create(cfg)
		if err == nil {
			t.Error("Create() with empty type should return error")
		}
	})

	// 子测试 7: 客户端存储和检索测试
	t.Run("ClientStorageAndRetrieval", func(t *testing.T) {
		factory := NewFactory()
		
		// 测试在没有客户端时的错误处理
		testCases := []struct {
			name     string
			testFunc func() error
			expected string
		}{
			{
				name: "MySQL",
				testFunc: func() error {
					_, err := factory.GetMySQL()
					return err
				},
				expected: "MySQL client not found",
			},
			{
				name: "MySQLSQL",
				testFunc: func() error {
					_, err := factory.GetMySQLSQLClient()
					return err
				},
				expected: "MySQL SQL client not found",
			},
			{
				name: "Redis",
				testFunc: func() error {
					_, err := factory.GetRedis()
					return err
				},
				expected: "Redis client not found",
			},
			{
				name: "Postgres",
				testFunc: func() error {
					_, err := factory.GetPostgres()
					return err
				},
				expected: "PostgreSQL client not found",
			},
			{
				name: "GenericClient",
				testFunc: func() error {
					_, err := factory.GetClient(TypeMySQL)
					return err
				},
				expected: "client for type mysql not found",
			},
		}
		
		for _, tc := range testCases {
			t.Run(tc.name+"NotFound", func(t *testing.T) {
				err := tc.testFunc()
				if err == nil {
					t.Errorf("%s: expected error, got nil", tc.name)
				}
				if err.Error() != tc.expected {
					t.Errorf("%s: expected %q, got %q", tc.name, tc.expected, err.Error())
				}
			})
		}
	})

	// 子测试 8: 配置默认值测试
	t.Run("DefaultValues", func(t *testing.T) {
		// 测试 MySQL 默认值
		cfg := Config{Type: TypeMySQL}
		applyDefaults(&cfg)
		if cfg.MySQL.Host != "localhost" {
			t.Errorf("MySQL default Host = %v, want localhost", cfg.MySQL.Host)
		}
		if cfg.MySQL.Port != 3306 {
			t.Errorf("MySQL default Port = %v, want 3306", cfg.MySQL.Port)
		}

		// 测试 Redis 默认值
		cfg = Config{Type: TypeRedis}
		applyDefaults(&cfg)
		if cfg.Redis.Host != "localhost" {
			t.Errorf("Redis default Host = %v, want localhost", cfg.Redis.Host)
		}
		if cfg.Redis.Port != 6379 {
			t.Errorf("Redis default Port = %v, want 6379", cfg.Redis.Port)
		}

		// 测试 PostgreSQL 默认值
		cfg = Config{Type: TypePostgres}
		applyDefaults(&cfg)
		if cfg.PG.Host != "localhost" {
			t.Errorf("PG default Host = %v, want localhost", cfg.PG.Host)
		}
		if cfg.PG.Port != 5432 {
			t.Errorf("PG default Port = %v, want 5432", cfg.PG.Port)
		}
	})

	// 子测试 9: 配置保存和加载测试
	t.Run("ConfigSaveAndLoad", func(t *testing.T) {
		tmpDir := t.TempDir()

		// 测试 YAML 保存和加载
		yamlFile := filepath.Join(tmpDir, "save_test.yaml")
		cfg := Config{
			Type: TypeMySQL,
			MySQL: mysql.Config{
				Host: "testhost",
				Port: 3307,
			},
		}

		if err := SaveConfigToYAML(yamlFile, cfg); err != nil {
			t.Errorf("SaveConfigToYAML() error = %v", err)
		}

		loaded, err := LoadConfigFromYAML(yamlFile)
		if err != nil {
			t.Errorf("LoadConfigFromYAML() error = %v", err)
		}
		if loaded.Type != cfg.Type {
			t.Error("Loaded config type mismatch")
		}

		// 测试 JSON 保存和加载
		jsonFile := filepath.Join(tmpDir, "save_test.json")
		cfg2 := Config{
			Type: TypeRedis,
			Redis: redis.Config{
				Host: "redishost",
				Port: 6380,
			},
		}

		if err := SaveConfigToJSON(jsonFile, cfg2); err != nil {
			t.Errorf("SaveConfigToJSON() error = %v", err)
		}

		loaded2, err := LoadConfigFromJSON(jsonFile)
		if err != nil {
			t.Errorf("LoadConfigFromJSON() error = %v", err)
		}
		if loaded2.Type != cfg2.Type {
			t.Error("Loaded config type mismatch")
		}
	})

	// 子测试 10: 多配置加载测试
	t.Run("MultipleConfigsLoading", func(t *testing.T) {
		yamlContent := `
primary:
  type: mysql
  mysql:
    host: "localhost"
    port: 3306
    database: "mydb"

secondary:
  type: redis
  redis:
    host: "localhost"
    port: 6379
`

		tmpDir := t.TempDir()
		tmpFile := filepath.Join(tmpDir, "configs.yaml")

		if err := os.WriteFile(tmpFile, []byte(yamlContent), 0644); err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}

		cfgs, err := LoadConfigsFromYAML(tmpFile)
		if err != nil {
			t.Fatalf("LoadConfigsFromYAML() error = %v", err)
		}

		if len(cfgs) != 2 {
			t.Errorf("Expected 2 configs, got %d", len(cfgs))
		}

		if cfg, ok := cfgs["primary"]; !ok || cfg.Type != TypeMySQL {
			t.Error("Primary config should be mysql")
		}

		if cfg, ok := cfgs["secondary"]; !ok || cfg.Type != TypeRedis {
			t.Error("Secondary config should be redis")
		}
	})

	// 子测试 11: 模拟客户端创建测试（不依赖真实数据库）
	t.Run("MockClientCreation", func(t *testing.T) {
		factory := NewFactory()
		
		// 创建各种数据库配置
		configs := []Config{
			{
				Type: TypeMySQL,
				MySQL: mysql.Config{
					Host:            "localhost",
					Port:            3306,
					Database:        "test_db",
					Username:        "root",
					Password:        "password",
					Charset:         "utf8mb4",
					MaxOpenConns:    25,
					MaxIdleConns:    10,
					ConnMaxLifetime: time.Hour,
					ConnMaxIdleTime: 30 * time.Minute,
				},
			},
			{
				Type: TypeRedis,
				Redis: redis.Config{
					Host:     "localhost",
					Port:     6379,
					Database: 0,
					PoolSize: 10,
				},
			},
			{
				Type: TypePostgres,
				PG: postgres.Config{
					Host:     "localhost",
					Port:     5432,
					Database: "test_db",
					Username: "postgres",
					Password: "password",
				},
			},
		}

		// 尝试创建客户端（预期失败，因为没有真实数据库）
		for i, cfg := range configs {
			t.Run(fmt.Sprintf("CreateClient_%d_%s", i, cfg.Type), func(t *testing.T) {
				_, err := factory.Create(cfg)
				if err == nil {
					t.Logf("Client created successfully for %s (database server might be running)", cfg.Type)
				} else {
					t.Logf("Client creation failed as expected for %s: %v", cfg.Type, err)
				}
			})
		}
	})
}

// TestFactoryClientStorageDetailed 详细测试客户端存储功能
func TestFactoryClientStorageDetailed(t *testing.T) {
	factory := NewFactory()
	
	// 验证所有数据库类型的 Get 方法在没有客户端时的行为
	databaseTypes := []struct {
		name         string
		dbType       Type
		getFunc      func() error
		expectedErr  string
	}{
		{
			name:    "MySQL",
			dbType:  TypeMySQL,
			getFunc: func() error {
				_, err := factory.GetMySQL()
				return err
			},
			expectedErr: "MySQL client not found",
		},
		{
			name:    "MySQLSQL",
			dbType:  TypeMySQL,
			getFunc: func() error {
				_, err := factory.GetMySQLSQLClient()
				return err
			},
			expectedErr: "MySQL SQL client not found",
		},
		{
			name:    "Redis",
			dbType:  TypeRedis,
			getFunc: func() error {
				_, err := factory.GetRedis()
				return err
			},
			expectedErr: "Redis client not found",
		},
		{
			name:    "Postgres",
			dbType:  TypePostgres,
			getFunc: func() error {
				_, err := factory.GetPostgres()
				return err
			},
			expectedErr: "PostgreSQL client not found",
		},
	}

	for _, dt := range databaseTypes {
		t.Run(dt.name+"ClientNotFound", func(t *testing.T) {
			err := dt.getFunc()
			if err == nil {
				t.Errorf("%s: expected error, got nil", dt.name)
			}
			if err.Error() != dt.expectedErr {
				t.Errorf("%s: expected %q, got %q", dt.name, dt.expectedErr, err.Error())
			}
		})

		t.Run(dt.name+"GenericClientNotFound", func(t *testing.T) {
			_, err := factory.GetClient(dt.dbType)
			if err == nil {
				t.Errorf("%s: expected error for GetClient, got nil", dt.name)
			}
			expected := fmt.Sprintf("client for type %s not found", dt.dbType)
			if err.Error() != expected {
				t.Errorf("%s: expected %q, got %q", dt.name, expected, err.Error())
			}
		})
	}
}

// TestFactoryErrorHandling 测试 Factory 的错误处理
func TestFactoryErrorHandling(t *testing.T) {
	factory := NewFactory()

	t.Run("UnsupportedType", func(t *testing.T) {
		cfg := Config{Type: "unsupported_type"}
		_, err := factory.Create(cfg)
		if err == nil {
			t.Error("Expected error for unsupported type")
		}
		if !contains(err.Error(), "unsupported database type") {
			t.Errorf("Expected 'unsupported database type' error, got %v", err)
		}
	})

	t.Run("EmptyType", func(t *testing.T) {
		cfg := Config{Type: ""}
		_, err := factory.Create(cfg)
		if err == nil {
			t.Error("Expected error for empty type")
		}
		if !contains(err.Error(), "database type is required") {
			t.Errorf("Expected 'database type is required' error, got %v", err)
		}
	})
}

// 辅助函数
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr || 
		   (len(s) > len(substr) && s[len(s)-len(substr):] == substr) ||
		   (len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
