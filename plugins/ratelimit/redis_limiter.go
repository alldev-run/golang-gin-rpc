package ratelimit

import (
	"context"
	"fmt"
	"time"
)

// RedisLimiter 是跨实例共享配额的集中式限流器(固定窗口,Lua 原子计数)。
//
// 与进程内 TokenBucket/FixedWindow 的区别:计数保存在 Redis,多实例部署时
// 全局限额共享。通过 RedisScriptRunner 接口注入执行器,避免对本插件之外的
// 具体客户端产生编译依赖——plugins/cache/redis 的 *redis.Client 天然满足。
type RedisLimiter struct {
	redis RedisScriptRunner
	cfg   RedisLimiterConfig
}

// RedisScriptRunner 是限流器所需的最小 Redis 能力。
type RedisScriptRunner interface {
	Eval(ctx context.Context, script string, keys []string, args ...any) (any, error)
}

// RedisLimiterConfig 是集中式限流配置。
type RedisLimiterConfig struct {
	// Requests 是每个窗口内允许的请求数。
	Requests int
	// Window 是窗口大小。
	Window time.Duration
	// Prefix 是计数键前缀,默认 "ratelimit:"。
	Prefix string
}

// DefaultRedisLimiterConfig 返回默认配置(60 次/分钟)。
func DefaultRedisLimiterConfig() RedisLimiterConfig {
	return RedisLimiterConfig{
		Requests: 60,
		Window:   time.Minute,
		Prefix:   "ratelimit:",
	}
}

// NewRedisLimiter 创建集中式限流器。
func NewRedisLimiter(runner RedisScriptRunner, cfg RedisLimiterConfig) (*RedisLimiter, error) {
	if runner == nil {
		return nil, fmt.Errorf("ratelimit: redis runner is nil")
	}
	if cfg.Requests <= 0 {
		return nil, fmt.Errorf("ratelimit: requests must be positive")
	}
	if cfg.Window <= 0 {
		return nil, fmt.Errorf("ratelimit: window must be positive")
	}
	if cfg.Prefix == "" {
		cfg.Prefix = "ratelimit:"
	}
	return &RedisLimiter{redis: runner, cfg: cfg}, nil
}

// luaRedisFixedWindow:INCR 计数,首次写入设置窗口过期;超出配额返回 0。
const luaRedisFixedWindow = `
local current = redis.call("incr", KEYS[1])
if current == 1 then
	redis.call("pexpire", KEYS[1], ARGV[1])
end
if current > tonumber(ARGV[2]) then
	return 0
end
return 1`

// Allow 判断 key(如 userID / clientIP)在当前窗口内是否允许通过。
// Redis 故障时返回 error,由调用方决定放行或拒绝策略(fail-open/fail-closed)。
func (l *RedisLimiter) Allow(ctx context.Context, key string) (bool, error) {
	if key == "" {
		return false, fmt.Errorf("ratelimit: empty key")
	}
	res, err := l.redis.Eval(ctx, luaRedisFixedWindow, []string{l.cfg.Prefix + key},
		l.cfg.Window.Milliseconds(), l.cfg.Requests)
	if err != nil {
		return false, fmt.Errorf("ratelimit: redis eval: %w", err)
	}
	n, ok := res.(int64)
	if !ok {
		return false, fmt.Errorf("ratelimit: unexpected redis reply %T", res)
	}
	return n == 1, nil
}
