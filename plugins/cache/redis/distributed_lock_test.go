package redis

import (
	"testing"
	"time"
)

// ===== 单元测试(不依赖真实 Redis)=====

func TestDistributedLockBasics(t *testing.T) {
	client, _ := New(DefaultConfig())
	l1 := NewDistributedLock(client, "order-1", time.Second)
	l2 := NewDistributedLock(client, "order-1", time.Second)

	if l1.Key() != "lock:order-1" {
		t.Fatalf("key = %s", l1.Key())
	}
	if l1.token == l2.token {
		t.Fatal("tokens must be unique per lock instance")
	}
	if l1.ttl != time.Second {
		t.Fatalf("ttl = %v", l1.ttl)
	}
	// 非法 ttl 使用默认值。
	def := NewDistributedLock(client, "k", 0)
	if def.ttl != 30*time.Second {
		t.Fatalf("default ttl = %v", def.ttl)
	}
}

func TestDistributedLockWatchdogStopsWithoutRedis(t *testing.T) {
	client, _ := New(DefaultConfig())
	l := NewDistributedLock(client, "wd", 200*time.Millisecond)
	l.EnableWatchdog()
	// 未 Acquire 时看门狗 goroutine 已启动但续期失败应静默;停止不阻塞。
	done := make(chan struct{})
	go func() {
		l.CloseWatchdog()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("CloseWatchdog blocked")
	}
}
