package redis

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// 分布式锁:SET NX PX 获取 + token 校验释放/续期(Lua 保证原子) + 看门狗自动续期。
//
// 与旧 Lock/Unlock(SetNX 后无条件 Del)的区别:释放前校验持有者 token,
// 不会误删他人的锁;可选看门狗在业务执行期间自动续期,避免业务超时锁先过期。

var (
	// ErrLockNotHeld 在未持有锁时执行 Release/Renew 返回。
	ErrLockNotHeld = errors.New("redislock: lock not held")

	// luaRelease 仅当 token 匹配时删除锁键。
	luaRelease = `
if redis.call("get", KEYS[1]) == ARGV[1] then
	return redis.call("del", KEYS[1])
else
	return 0
end`

	// luaRenew 仅当 token 匹配时重设过期时间。
	luaRenew = `
if redis.call("get", KEYS[1]) == ARGV[1] then
	return redis.call("pexpire", KEYS[1], ARGV[2])
else
	return 0
end`
)

// DistributedLock 是一把基于 Redis 的互斥锁实例。
// 同一把锁应通过多次 NewDistributedLock 在不同持有者间竞争。
type DistributedLock struct {
	client *Client
	key    string
	token  string
	ttl    time.Duration

	mu              sync.Mutex
	acquired        bool
	watchdogEnabled bool
	watchdogStop    chan struct{}
	watchdogDone    chan struct{}
}

// NewDistributedLock 创建一把锁。ttl 是锁的过期时间,必须显著大于持锁方
// 单次业务的预期耗时,或启用看门狗续期。
func NewDistributedLock(client *Client, key string, ttl time.Duration) *DistributedLock {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	return &DistributedLock{
		client: client,
		key:    "lock:" + key,
		token:  uuid.NewString(),
		ttl:    ttl,
	}
}

// Key 返回实际的 Redis 键名(带 lock: 前缀)。
func (l *DistributedLock) Key() string { return l.key }

// TryAcquire 尝试一次加锁,不等待。成功后启动看门狗(若已开启)。
func (l *DistributedLock) TryAcquire(ctx context.Context) (bool, error) {
	ok, err := l.client.SetNX(ctx, l.key, l.token, l.ttl)
	if err != nil {
		return false, fmt.Errorf("redislock: acquire: %w", err)
	}
	if ok {
		l.mu.Lock()
		l.acquired = true
		l.mu.Unlock()
		l.maybeStartWatchdog()
	}
	return ok, nil
}

// Acquire 以固定间隔重试加锁,直到成功、ctx 结束或出错。
func (l *DistributedLock) Acquire(ctx context.Context, retryInterval time.Duration) error {
	if retryInterval <= 0 {
		retryInterval = 100 * time.Millisecond
	}
	ticker := time.NewTicker(retryInterval)
	defer ticker.Stop()
	for {
		ok, err := l.TryAcquire(ctx)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// Release 释放锁(仅当仍为本持有者)。释放后看门狗停止。
func (l *DistributedLock) Release(ctx context.Context) error {
	l.mu.Lock()
	watchdogStop := l.watchdogStop
	l.mu.Unlock()
	if watchdogStop != nil {
		l.stopWatchdog()
	}
	res, err := l.client.Eval(ctx, luaRelease, []string{l.key}, l.token)
	if err != nil {
		return fmt.Errorf("redislock: release: %w", err)
	}
	if n, _ := res.(int64); n != 1 {
		l.mu.Lock()
		l.acquired = false
		l.mu.Unlock()
		return ErrLockNotHeld
	}
	l.mu.Lock()
	l.acquired = false
	l.mu.Unlock()
	return nil
}

// Renew 手动续期(重设 TTL,仅当仍为本持有者)。
func (l *DistributedLock) Renew(ctx context.Context) (bool, error) {
	res, err := l.client.Eval(ctx, luaRenew, []string{l.key}, l.token, l.ttl.Milliseconds())
	if err != nil {
		return false, fmt.Errorf("redislock: renew: %w", err)
	}
	if n, _ := res.(int64); n != 1 {
		return false, ErrLockNotHeld
	}
	return true, nil
}

// EnableWatchdog 开启看门狗:Acquire 成功后每 ttl/3 自动续期,直到 Release
// 或 CloseWatchdog。可在 Acquire 前任意时机调用。
func (l *DistributedLock) EnableWatchdog() {
	l.mu.Lock()
	l.watchdogEnabled = true
	l.mu.Unlock()
}

func (l *DistributedLock) maybeStartWatchdog() {
	l.mu.Lock()
	enabled := l.watchdogEnabled
	started := l.watchdogStop != nil
	l.mu.Unlock()
	if !enabled || started {
		return
	}
	l.mu.Lock()
	// 双重检查(并发 Acquire)。
	if l.watchdogStop != nil {
		l.mu.Unlock()
		return
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	l.watchdogStop = stop
	l.watchdogDone = done
	l.mu.Unlock()
	go func() {
		defer close(done)
		interval := l.ttl / 3
		if interval <= 0 {
			interval = time.Second
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), interval)
				_, _ = l.Renew(ctx)
				cancel()
			}
		}
	}()
}

func (l *DistributedLock) stopWatchdog() {
	l.mu.Lock()
	stop, done := l.watchdogStop, l.watchdogDone
	l.watchdogStop, l.watchdogDone = nil, nil
	l.mu.Unlock()
	if stop != nil {
		close(stop)
	}
	if done != nil {
		<-done
	}
}

// CloseWatchdog 停止看门狗(不释放锁)。
func (l *DistributedLock) CloseWatchdog() {
	l.stopWatchdog()
}
