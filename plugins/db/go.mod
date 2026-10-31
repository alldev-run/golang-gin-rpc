module github.com/alldev-run/golang-gin-rpc/plugins/db

go 1.25.7

require (
	github.com/ClickHouse/clickhouse-go/v2 v2.43.0
	github.com/DATA-DOG/go-sqlmock v1.5.2
	github.com/alldev-run/golang-gin-rpc v0.0.0
	github.com/alldev-run/golang-gin-rpc/plugins/cache v0.0.0
	github.com/alldev-run/golang-gin-rpc/plugins/metrics v0.0.0
	github.com/alldev-run/golang-gin-rpc/plugins/search v0.0.0
	github.com/go-sql-driver/mysql v1.9.3
	github.com/lib/pq v1.11.2
	github.com/mattn/go-sqlite3 v1.14.34
	github.com/prometheus/client_golang v1.23.2
	go.mongodb.org/mongo-driver/v2 v2.5.0
	go.uber.org/zap v1.27.1
	golang.org/x/sync v0.20.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/ClickHouse/ch-go v0.71.0 // indirect
	github.com/andybalholm/brotli v1.2.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bradfitz/gomemcache v0.0.0-20250403215159-8d39553ac7cf // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/elastic/elastic-transport-go/v8 v8.9.0 // indirect
	github.com/elastic/go-elasticsearch/v8 v8.19.7 // indirect
	github.com/go-faster/city v1.0.1 // indirect
	github.com/go-faster/errors v0.7.1 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/klauspost/compress v1.18.5 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/paulmach/orb v0.12.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.25 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.67.5 // indirect
	github.com/prometheus/procfs v0.20.1 // indirect
	github.com/redis/go-redis/v9 v9.18.0 // indirect
	github.com/segmentio/asm v1.2.1 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.2.0 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	github.com/youmark/pkcs8 v0.0.0-20240726163527-a2c0da244d78 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel v1.42.0 // indirect
	go.opentelemetry.io/otel/metric v1.42.0 // indirect
	go.opentelemetry.io/otel/trace v1.42.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v2 v2.4.4 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.49.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/text v0.35.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
)

replace (
	github.com/alldev-run/golang-gin-rpc => ../..
	github.com/alldev-run/golang-gin-rpc/plugins/cache => ../cache
	github.com/alldev-run/golang-gin-rpc/plugins/metrics => ../metrics
	github.com/alldev-run/golang-gin-rpc/plugins/search => ../search
)
