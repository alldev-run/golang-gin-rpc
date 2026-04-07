package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	appconfig "github.com/alldev-run/golang-gin-rpc/pkg/config"
	"github.com/alldev-run/golang-gin-rpc/pkg/db"
	"github.com/alldev-run/golang-gin-rpc/pkg/db/mysql"
	"github.com/alldev-run/golang-gin-rpc/pkg/db/pool"
)

func main() {
	factory := db.NewFactory()

	poolCfg := pool.DefaultConfig()
	poolCfg.MaxSize = 20
	poolCfg.InitialSize = 1
	poolCfg.AcquireTimeout = 3 * time.Second

	p := pool.New(poolCfg, factory)
	defer p.Close()

	if len(os.Args) > 1 {
		configPath := os.Args[1]
		if err := registerFromGlobalConfig(p, configPath); err != nil {
			log.Fatalf("register from config failed: %v", err)
		}

		ctx := context.Background()
		for _, name := range []string{"primary", "replica"} {
			client, err := p.Acquire(ctx, name)
			if err != nil {
				continue
			}
			if err := client.Ping(ctx); err != nil {
				log.Printf("%s ping failed: %v", name, err)
				continue
			}
			fmt.Printf("%s mysql pool is ready\n", name)
		}
		fmt.Printf("pool stats: %+v\n", p.GetStats())
		return
	}

	mysqlCfg := db.Config{
		Type: db.TypeMySQL,
		MySQL: mysql.Config{
			Host:     "127.0.0.1",
			Port:     3306,
			Database: "app",
			Username: "root",
			Password: "root",
			// 不填池参数也可：mysql.New 会自动补默认池配置
			// MaxOpenConns: 25
			// MaxIdleConns: 10
		},
	}

	if err := p.Register("primary", mysqlCfg); err != nil {
		log.Fatalf("register primary failed: %v", err)
	}

	ctx := context.Background()
	client, err := p.Acquire(ctx, "primary")
	if err != nil {
		log.Fatalf("acquire primary failed: %v", err)
	}

	if err := client.Ping(ctx); err != nil {
		log.Fatalf("ping failed: %v", err)
	}

	fmt.Println("mysql pool is ready")
	fmt.Printf("pool stats: %+v\n", p.GetStats()["primary"])
}

func registerFromGlobalConfig(p *pool.Pool, configPath string) error {
	loader := appconfig.NewLoader()
	if err := loader.Load(configPath); err != nil {
		return err
	}

	cfg := loader.Get()

	if cfg.Database.Primary.Enabled && cfg.Database.Primary.Driver == "mysql" {
		if err := p.Register("primary", db.Config{
			Type: db.TypeMySQL,
			MySQL: mysql.Config{
				Host:            cfg.Database.Primary.Host,
				Port:            cfg.Database.Primary.Port,
				Database:        cfg.Database.Primary.Database,
				Username:        cfg.Database.Primary.Username,
				Password:        cfg.Database.Primary.Password,
				MaxOpenConns:    cfg.Database.Pool.MaxOpenConns,
				MaxIdleConns:    cfg.Database.Pool.MaxIdleConns,
				ConnMaxLifetime: cfg.Database.Pool.ConnMaxLifetime,
				ConnMaxIdleTime: cfg.Database.Pool.ConnMaxIdleTime,
			},
		}); err != nil {
			return err
		}
	}

	if cfg.Database.Replica.Enabled && cfg.Database.Replica.Driver == "mysql" {
		if err := p.Register("replica", db.Config{
			Type: db.TypeMySQL,
			MySQL: mysql.Config{
				Host:            cfg.Database.Replica.Host,
				Port:            cfg.Database.Replica.Port,
				Database:        cfg.Database.Replica.Database,
				Username:        cfg.Database.Replica.Username,
				Password:        cfg.Database.Replica.Password,
				MaxOpenConns:    cfg.Database.Pool.MaxOpenConns,
				MaxIdleConns:    cfg.Database.Pool.MaxIdleConns,
				ConnMaxLifetime: cfg.Database.Pool.ConnMaxLifetime,
				ConnMaxIdleTime: cfg.Database.Pool.ConnMaxIdleTime,
			},
		}); err != nil {
			return err
		}
	}

	return nil
}
