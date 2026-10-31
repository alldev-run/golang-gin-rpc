module github.com/alldev-run/golang-gin-rpc/plugins/search

go 1.25.7

require (
	github.com/alldev-run/golang-gin-rpc/plugins/db v0.0.0
	github.com/elastic/go-elasticsearch/v8 v8.19.7
)

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/alldev-run/golang-gin-rpc v0.0.0 // indirect
	github.com/alldev-run/golang-gin-rpc/plugins/cache v0.0.0 // indirect
	github.com/bradfitz/gomemcache v0.0.0-20250403215159-8d39553ac7cf // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/elastic/elastic-transport-go/v8 v8.9.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-sql-driver/mysql v1.9.3 // indirect
	github.com/lib/pq v1.11.2 // indirect
	github.com/redis/go-redis/v9 v9.18.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel v1.42.0 // indirect
	go.opentelemetry.io/otel/metric v1.42.0 // indirect
	go.opentelemetry.io/otel/trace v1.42.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.1 // indirect
	golang.org/x/sync v0.20.0 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/alldev-run/golang-gin-rpc => ../..
	github.com/alldev-run/golang-gin-rpc/plugins/cache => ../cache
	github.com/alldev-run/golang-gin-rpc/plugins/db => ../db
)
