package bootstrap

import (
	"context"
	"fmt"

	"github.com/alldev-run/golang-gin-rpc/pkg/kernel"
	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
)

// FrameworkOptions defines what bootstrap should initialize and manage.
// 组件级开关由 plugins.<name>.enabled 配置段驱动（kernel.Loader 构建期裁剪）；
// v0.2 起不再支持旧的 framework.init_* 兼容键。
type FrameworkOptions struct {
	ValidateDependencyCoverage bool

	WebSocket *WebSocketServiceOptions
	Services  []string
}

// DefaultFrameworkOptions returns a production-friendly default.
func DefaultFrameworkOptions() FrameworkOptions {
	return FrameworkOptions{
		ValidateDependencyCoverage: true,
		Services:                   []string{ServiceRPC, ServiceAPIGateway},
	}
}

// FrameworkOptionsFromConfig builds framework options from loaded global config.
func (b *Bootstrap) FrameworkOptionsFromConfig() FrameworkOptions {
	if b == nil || b.config == nil {
		return DefaultFrameworkOptions()
	}
	return FrameworkOptions{
		ValidateDependencyCoverage: b.config.Framework.ValidateDependencyCoverage,
		Services:                   append([]string(nil), b.config.Framework.Services...),
	}
}

// StartFramework 通过 kernel 插件机制初始化核心依赖，并启动所选托管服务。
//
// 顺序：内置插件（db/cache/discovery/tracing/auth/metrics/health/errors/
// upload/rpc，按依赖拓扑排序、可被配置禁用）→ 托管服务（api-gateway/rpc/
// websocket 等按注册顺序）→ 依赖覆盖校验。
func (b *Bootstrap) StartFramework(ctx context.Context, options FrameworkOptions) error {
	// 注册内置组件适配器（幂等），构建并启动插件应用。
	registerBuiltinPlugins(b)

	app, err := kernel.NewLoader(b.loader, kernel.WithRegistry(b.registry)).Build()
	if err != nil {
		return fmt.Errorf("failed to build plugin app: %w", err)
	}
	if err := app.Start(ctx); err != nil {
		return err
	}
	b.pluginMu.Lock()
	b.pluginApp = app
	b.pluginMu.Unlock()

	if options.WebSocket != nil {
		if err := b.RegisterWebSocketServiceFactory(*options.WebSocket); err != nil {
			return err
		}
	}
	if len(options.Services) > 0 {
		if err := b.StartServices(ctx, options.Services...); err != nil {
			return err
		}
	}
	if options.ValidateDependencyCoverage {
		if err := b.ValidateDependencyCoverage(options); err != nil {
			return err
		}
	}
	return nil
}

// StopFramework stops selected managed services and the plugin app.
func (b *Bootstrap) StopFramework(ctx context.Context, services ...string) error {
	if len(services) > 0 {
		if err := b.StopServices(ctx, services...); err != nil {
			return err
		}
	}
	return b.stopPluginApp(ctx)
}

// stopPluginApp 逆序停止所有已启动插件，并记录已停止的组件供 Close 判重。
func (b *Bootstrap) stopPluginApp(ctx context.Context) error {
	b.pluginMu.RLock()
	app := b.pluginApp
	b.pluginMu.RUnlock()
	if app == nil {
		return nil
	}

	stopped := make(map[string]bool)
	for _, p := range app.Plugins() {
		stopped[p.Name()] = true
	}

	if err := app.Stop(ctx); err != nil {
		logger.Warn("plugin app stop reported errors", logger.Error(err))
	}
	b.pluginMu.Lock()
	b.pluginApp = nil
	if b.stoppedSet == nil {
		b.stoppedSet = make(map[string]bool)
	}
	for name := range stopped {
		b.stoppedSet[name] = true
	}
	b.pluginMu.Unlock()
	return nil
}
