package bootstrap

import (
	"fmt"
	"sort"
	"strings"
)

// componentDependencyKeys 内置组件插件 → 依赖容器键。
var componentDependencyKeys = map[string]string{
	"db":        "db.factory",
	"cache":     "cache",
	"discovery": "discovery.manager",
	"tracing":   "tracer",
	"auth":      "auth.manager",
	"metrics":   "metrics.collector",
	"health":    "health.manager",
	"errors":    "errors.initialized",
}

// ValidateDependencyCoverage checks whether required framework dependencies
// have been injected into bootstrap dependency container.
// 组件级要求由实际启动的内置插件决定；服务级要求仍由 options.Services 决定。
func (b *Bootstrap) ValidateDependencyCoverage(options FrameworkOptions) error {
	required := make(map[string]struct{})

	for plugin, key := range componentDependencyKeys {
		if b.pluginStarted(plugin) {
			required[key] = struct{}{}
		}
	}

	for _, service := range options.Services {
		svc := strings.TrimSpace(service)
		switch svc {
		case ServiceRPC:
			required["rpc.manager"] = struct{}{}
		case ServiceAPIGateway:
			required["gateway"] = struct{}{}
			required["gateway.http_service"] = struct{}{}
		case ServiceWebSocket:
			required["websocket.server"] = struct{}{}
		default:
			if strings.HasPrefix(svc, "api-gateway") {
				required["gateway"] = struct{}{}
				required["gateway.http_service"] = struct{}{}
			}
			if strings.HasPrefix(svc, "websocket") {
				required["websocket.server"] = struct{}{}
			}
		}
	}

	missing := make([]string, 0)
	for key := range required {
		if _, ok := b.GetDependency(key); !ok {
			missing = append(missing, key)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	return fmt.Errorf("bootstrap dependency coverage validation failed, missing: %s", strings.Join(missing, ", "))
}
