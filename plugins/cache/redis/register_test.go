package redis

import (
	"testing"

	"github.com/alldev-run/golang-gin-rpc/plugins/db"
)

func TestDriverRegistered(t *testing.T) {
	// init() of this package must have registered the redis db driver.
	if !db.DriverLinked(db.TypeRedis) {
		t.Fatal("redis driver should be linked after import")
	}
}

func TestDecodeConfig(t *testing.T) {
	want := Config{Host: "127.0.0.1", Port: 6390}

	t.Run("direct type", func(t *testing.T) {
		got, ok := decodeConfig[Config](want)
		if !ok || got.Host != want.Host || got.Port != want.Port {
			t.Fatalf("decodeConfig = (%+v, %v), want (%+v, true)", got, ok, want)
		}
	})

	t.Run("map round-trip", func(t *testing.T) {
		m := map[string]any{"host": "127.0.0.1", "port": 6390}
		got, ok := decodeConfig[Config](m)
		if !ok || got.Host != "127.0.0.1" || got.Port != 6390 {
			t.Fatalf("decodeConfig = (%+v, %v), want host 127.0.0.1 port 6390", got, ok)
		}
	})

	t.Run("nil map", func(t *testing.T) {
		if _, ok := decodeConfig[Config]((map[string]any)(nil)); ok {
			t.Error("expected ok=false for nil map")
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		if _, ok := decodeConfig[Config]("not a config"); ok {
			t.Error("expected ok=false for wrong type")
		}
	})
}
