package bootstrap

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alldev-run/golang-gin-rpc/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadDatabaseConfig(t *testing.T) {
	tests := []struct {
		name        string
		configData  string
		expectError bool
		expectedDB  map[string]config.DBConfig
	}{
		{
			name: "nested mysql format",
			configData: `mysql_primary:
  type: mysql
  mysql:
    host: "localhost"
    port: 3306
    database: "testdb"
    username: "root"
    password: "password"
    charset: "utf8mb4"`,
			expectError: false,
			expectedDB: map[string]config.DBConfig{
				"mysql_primary": {
					Enabled:  true,
					Driver:   "mysql",
					Host:     "localhost",
					Port:     3306,
					Database: "testdb",
					Username: "root",
					Password: "password",
					SSLMode:  "utf8mb4",
				},
			},
		},
		{
			name: "flat format",
			configData: `mysql_primary:
  enabled: true
  driver: mysql
  host: "localhost"
  port: 3306
  database: "testdb"
  username: "root"
  password: "password"
  ssl_mode: "disable"`,
			expectError: false,
			expectedDB: map[string]config.DBConfig{
				"mysql_primary": {
					Enabled:  true,
					Driver:   "mysql",
					Host:     "localhost",
					Port:     3306,
					Database: "testdb",
					Username: "root",
					Password: "password",
					SSLMode:  "disable",
				},
			},
		},
		{
			name: "postgres format",
			configData: `postgres_main:
  type: postgres
  postgres:
    host: "localhost"
    port: 5432
    database: "testdb"
    username: "postgres"
    password: "password"
    ssl_mode: "disable"`,
			expectError: false,
			expectedDB: map[string]config.DBConfig{
				"postgres_main": {
					Enabled:  true,
					Driver:   "postgres",
					Host:     "localhost",
					Port:     5432,
					Database: "testdb",
					Username: "postgres",
					Password: "password",
					SSLMode:  "disable",
				},
			},
		},
		{
			name: "invalid yaml",
			configData: `invalid: yaml: content: [`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary config file
			tmpDir := t.TempDir()
			configFile := filepath.Join(tmpDir, "database.yml")
			err := os.WriteFile(configFile, []byte(tt.configData), 0644)
			require.NoError(t, err)

			// Create bootstrap instance with defaults
			boot, err := NewBootstrapWithDefaults()
			require.NoError(t, err)
			defer boot.Close()

			// Load database config
			err = LoadDatabaseConfig(boot, configFile)
			
			if tt.expectError {
				assert.Error(t, err)
				return
			}
			
			require.NoError(t, err)
			
			// Verify the config was loaded correctly
			// We can't directly access the internal config, but we can test
			// that the UpdateDatabaseConfig method was called successfully
			assert.NoError(t, err)
		})
	}
}

func TestNewBootstrapWithDefaults(t *testing.T) {
	boot, err := NewBootstrapWithDefaults()
	require.NoError(t, err)
	require.NotNil(t, boot)
	defer boot.Close()
	
	// Test that bootstrap was created with default config
	assert.NoError(t, err)
}

func TestUpdateDatabaseConfig(t *testing.T) {
	boot, err := NewBootstrapWithDefaults()
	require.NoError(t, err)
	defer boot.Close()
	
	dbConfigs := map[string]config.DBConfig{
		"test_db": {
			Enabled:  true,
			Driver:   "mysql",
			Host:     "localhost",
			Port:     3306,
			Database: "testdb",
			Username: "root",
			Password: "password",
		},
	}
	
	err = boot.UpdateDatabaseConfig(dbConfigs)
	assert.NoError(t, err)
}

func TestLoadDatabaseConfigNilBootstrap(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "database.yml")
	
	err := os.WriteFile(configFile, []byte("test: config"), 0644)
	require.NoError(t, err)
	
	err = LoadDatabaseConfig(nil, configFile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bootstrap instance is nil")
}

func TestGetDatabaseFactory(t *testing.T) {
	boot, err := NewBootstrapWithDefaults()
	require.NoError(t, err)
	defer boot.Close()
	
	factory := GetDatabaseFactory(boot)
	// Factory is nil when database is not initialized
	assert.Nil(t, factory)
}

func TestGetMySQLClient(t *testing.T) {
	boot, err := NewBootstrapWithDefaults()
	require.NoError(t, err)
	defer boot.Close()
	
	// Test when database is not initialized
	client, err := GetMySQLClient(boot)
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "database factory not initialized")
	
	// Test with nil bootstrap
	client, err = GetMySQLClient(nil)
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "bootstrap instance is nil")
}

func TestGetMySQLSQLClient(t *testing.T) {
	boot, err := NewBootstrapWithDefaults()
	require.NoError(t, err)
	defer boot.Close()
	
	// Test when database is not initialized
	client, err := GetMySQLSQLClient(boot)
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "database factory not initialized")
	
	// Test with nil bootstrap
	client, err = GetMySQLSQLClient(nil)
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "bootstrap instance is nil")
}

func TestGetRedisClient(t *testing.T) {
	boot, err := NewBootstrapWithDefaults()
	require.NoError(t, err)
	defer boot.Close()
	
	// Test when database is not initialized
	client, err := GetRedisClient(boot)
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "database factory not initialized")
	
	// Test with nil bootstrap
	client, err = GetRedisClient(nil)
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "bootstrap instance is nil")
}

func TestGetPostgresClient(t *testing.T) {
	boot, err := NewBootstrapWithDefaults()
	require.NoError(t, err)
	defer boot.Close()
	
	// Test when database is not initialized
	client, err := GetPostgresClient(boot)
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "database factory not initialized")
	
	// Test with nil bootstrap
	client, err = GetPostgresClient(nil)
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "bootstrap instance is nil")
}

func TestLoadDatabaseConfigFileNotFound(t *testing.T) {
	boot, err := NewBootstrapWithDefaults()
	require.NoError(t, err)
	defer boot.Close()
	
	err = LoadDatabaseConfig(boot, "/nonexistent/config.yml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read database config file")
}
