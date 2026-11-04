package memcache

import (
	"testing"
	"time"

	"github.com/alldev-run/golang-gin-rpc/plugins/db"
)

func TestDriverRegistered(t *testing.T) {
	// init() of this package must have registered the memcache db driver.
	if !db.DriverLinked(db.TypeMemcache) {
		t.Fatal("memcache driver should be linked after import")
	}
}

func TestDecodeConfig(t *testing.T) {
	t.Run("direct type", func(t *testing.T) {
		want := Config{Hosts: []string{"127.0.0.1:11211"}}
		got, ok := decodeConfig[Config](want)
		if !ok || len(got.Hosts) != 1 || got.Hosts[0] != want.Hosts[0] {
			t.Fatalf("decodeConfig = (%+v, %v), want (%+v, true)", got, ok, want)
		}
	})

	t.Run("map round-trip", func(t *testing.T) {
		m := map[string]any{"hosts": []any{"127.0.0.1:11211"}}
		got, ok := decodeConfig[Config](m)
		if !ok || len(got.Hosts) != 1 || got.Hosts[0] != "127.0.0.1:11211" {
			t.Fatalf("decodeConfig = (%+v, %v), want hosts [127.0.0.1:11211]", got, ok)
		}
	})

	t.Run("nil map", func(t *testing.T) {
		if _, ok := decodeConfig[Config]((map[string]any)(nil)); ok {
			t.Error("expected ok=false for nil map")
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		if _, ok := decodeConfig[Config](42); ok {
			t.Error("expected ok=false for wrong type")
		}
	})
}

func TestDbClientAdapter_Offline(t *testing.T) {
	// memcache.New is lazy, so the adapter can be exercised without a server:
	// Ping must fail against an unreachable endpoint, Close must succeed.
	client, err := New(Config{Hosts: []string{"127.0.0.1:1"}, Timeout: 100 * time.Millisecond})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	adapter := &dbClient{client: client}

	if err := adapter.Ping(t.Context()); err == nil {
		t.Error("Ping against unreachable memcache should fail")
	}
	if err := adapter.Close(); err != nil {
		t.Errorf("Close error = %v, want nil", err)
	}
}
