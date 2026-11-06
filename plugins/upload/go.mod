module github.com/alldev-run/golang-gin-rpc/plugins/upload

go 1.26.0

require github.com/google/uuid v1.6.0

require (
	github.com/alldev-run/golang-gin-rpc/pkg v0.0.0
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.1
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
)

replace github.com/alldev-run/golang-gin-rpc/pkg => ../../pkg
