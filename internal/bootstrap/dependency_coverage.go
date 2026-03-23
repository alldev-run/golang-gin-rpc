package bootstrap

import (
	"fmt"
	"sort"
	"strings"
)

// ValidateDependencyCoverage checks whether required framework dependencies
// have been injected into bootstrap dependency container.
func (b *Bootstrap) ValidateDependencyCoverage(options FrameworkOptions) error {
	required := make(map[string]struct{})

	if options.InitDatabases {
		required["db.factory"] = struct{}{}
	}
	if options.InitCache {
		required["cache"] = struct{}{}
	}
	if options.InitDiscovery {
		required["discovery.manager"] = struct{}{}
	}
	if options.InitTracing {
		required["tracer"] = struct{}{}
	}
	if options.InitAuth {
		required["auth.manager"] = struct{}{}
	}
	if options.InitMetrics {
		required["metrics.collector"] = struct{}{}
	}
	if options.InitHealth {
		required["health.manager"] = struct{}{}
	}
	if options.InitErrors {
		required["errors.initialized"] = struct{}{}
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
