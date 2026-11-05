package bootstrap

import (
	"fmt"
	"sort"
	"strings"
)

// componentDependencyKeys 内置组件插件 → 依赖容器键。
var componentDependencyKeys = map[string]string{
	"db":        DepDBFactory,
	"cache":     DepCache,
	"discovery": DepDiscoveryManager,
	"tracing":   DepTracer,
	"auth":      DepAuthManager,
	"metrics":   DepMetricsCollector,
	"health":    DepHealthManager,
	"errors":    DepErrorsInitialized,
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
		if keys, ok := b.serviceDependencies(svc); ok {
			for _, key := range keys {
				required[key] = struct{}{}
			}
			continue
		}
		switch svc {
		case ServiceRPC:
			required[DepRPCManager] = struct{}{}
		case ServiceAPIGateway:
			required[DepGateway] = struct{}{}
			required[DepGatewayHTTPService] = struct{}{}
		case ServiceWebSocket:
			required[DepWebSocketServer] = struct{}{}
		default:
			if strings.HasPrefix(svc, "api-gateway") {
				required[DepGateway] = struct{}{}
				required[DepGatewayHTTPService] = struct{}{}
			}
			if strings.HasPrefix(svc, "websocket") {
				required[DepWebSocketServer] = struct{}{}
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
