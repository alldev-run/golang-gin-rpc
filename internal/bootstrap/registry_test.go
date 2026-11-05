package bootstrap

import (
	"testing"

	"github.com/alldev-run/golang-gin-rpc/pkg/config"
)

func TestBuiltinPluginsAreScopedPerBootstrap(t *testing.T) {
	first := &Bootstrap{config: &config.GlobalConfig{}, dependencies: map[string]interface{}{}}
	second := &Bootstrap{config: &config.GlobalConfig{}, dependencies: map[string]interface{}{}}
	registerBuiltinPlugins(first)
	registerBuiltinPlugins(second)
	registerBuiltinPlugins(first)

	for _, name := range []string{"db", "cache", "tracing", "actor"} {
		p1, ok1 := first.registry.Get(name)
		p2, ok2 := second.registry.Get(name)
		if !ok1 || !ok2 {
			t.Fatalf("builtin plugin %q not registered on both bootstraps", name)
		}
		if p1.(*builtinPlugin).boot != first || p2.(*builtinPlugin).boot != second {
			t.Fatalf("builtin plugin %q is bound to the wrong bootstrap", name)
		}
	}
}
