module github.com/alldev-run/golang-gin-rpc/plugins/circuitbreaker

go 1.25.7

require (
	github.com/alldev-run/golang-gin-rpc v0.0.0
	go.uber.org/zap v1.27.1
)

require (
	go.uber.org/multierr v1.11.0 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
)

replace github.com/alldev-run/golang-gin-rpc => ../..
