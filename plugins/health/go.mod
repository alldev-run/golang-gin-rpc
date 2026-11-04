module github.com/alldev-run/golang-gin-rpc/plugins/health

go 1.25.7

require (
	github.com/alldev-run/golang-gin-rpc v0.0.0
	github.com/alldev-run/golang-gin-rpc/plugins/cache v0.0.0
)

require github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect

require (
	github.com/bradfitz/gomemcache v0.0.0-20250403215159-8d39553ac7cf // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/redis/go-redis/v9 v9.18.0 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.1 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/alldev-run/golang-gin-rpc => ../..
	github.com/alldev-run/golang-gin-rpc/plugins/cache => ../cache
)
