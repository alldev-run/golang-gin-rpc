package bootstrap

import (
	stdctx "context"

	"github.com/alldev-run/golang-gin-rpc/pkg/kernel"
)

// 内置组件以 kernel.Plugin 形式接入框架加载机制：
//   - 链接期：registerBuiltinPlugins 注册了哪些适配器，哪些组件参与构建；
//   - 运行期：plugins.<name>.enabled 配置段决定是否启用（kernel.Loader 裁剪，
//     v0.2 起不再支持旧的 framework.init_* 兼容键）；
//   - 启动顺序：由 depends 声明 + kernel 拓扑排序，替代原来的硬编码 if 链。
//
// 插件模块（plugins/*）保持纯库；谁装配、按什么顺序装配由框架（组合根）负责。

// builtinPlugin 把 Bootstrap 上的 Initialize/Stop 逻辑包装成 kernel.Plugin，
// 实例在注册时绑定。
type builtinPlugin struct {
	boot *Bootstrap

	name     string
	depends  []string
	softDeps []string
	startFn  func(b *Bootstrap) error
	stopFn   func(b *Bootstrap, ctx stdctx.Context) error
}

func (p *builtinPlugin) Name() string          { return p.name }
func (p *builtinPlugin) Depends() []string     { return p.depends }
func (p *builtinPlugin) SoftDepends() []string { return p.softDeps }

// Configure 无需解析额外配置段；启停完全由 plugins.<name>.enabled 驱动
// （kernel.Loader 在构建期裁剪），各 Initialize* 自身对未配置场景保持空值安全。
func (p *builtinPlugin) Configure(_ *kernel.Context) error {
	return nil
}

func (p *builtinPlugin) Start(_ *kernel.Context) error {
	return p.startFn(p.boot)
}

func (p *builtinPlugin) Stop(ctx *kernel.Context) error {
	if p.stopFn == nil {
		return nil
	}
	return p.stopFn(p.boot, ctx)
}

// registerBuiltinPlugins 把框架内置组件注册到该 Bootstrap 独占的 kernel 注册表（幂等），
// 同一进程内的多个 Bootstrap 互不共享内置插件实例。
func registerBuiltinPlugins(boot *Bootstrap) {
	builtin := []*builtinPlugin{
		{
			name:    "db",
			startFn: func(b *Bootstrap) error { return b.InitializeDatabases() },
		},
		{
			name:    "cache",
			startFn: func(b *Bootstrap) error { return b.InitializeCache() },
			stopFn: func(b *Bootstrap, _ stdctx.Context) error {
				if b.cache != nil {
					return b.cache.Close()
				}
				return nil
			},
		},
		{
			name:    "discovery",
			startFn: func(b *Bootstrap) error { return b.InitializeDiscovery() },
			stopFn: func(b *Bootstrap, _ stdctx.Context) error {
				if b.discoveryManager != nil {
					return b.discoveryManager.Stop()
				}
				return nil
			},
		},
		{
			name:    "tracing",
			startFn: func(b *Bootstrap) error { return b.InitializeTracing() },
			stopFn: func(b *Bootstrap, ctx stdctx.Context) error {
				if b.tracer != nil {
					return b.shutdownTracer(ctx)
				}
				return nil
			},
		},
		{
			name:    "auth",
			startFn: func(b *Bootstrap) error { return b.InitializeAuth() },
		},
		{
			name:    "metrics",
			startFn: func(b *Bootstrap) error { return b.InitializeMetrics() },
		},
		{
			name:    "health",
			startFn: func(b *Bootstrap) error { return b.InitializeHealth() },
			// db/cache 为软依赖：目标禁用时 health 降级（少注册对应 checker）。
			softDeps: []string{"db", "cache"},
		},
		{
			name:    "errors",
			startFn: func(b *Bootstrap) error { return b.InitializeErrors() },
		},
		{
			name:    "upload",
			startFn: func(b *Bootstrap) error { return b.InitializeUpload() }, // 由 upload 配置段自身决定
		},
		{
			name:    "actor",
			startFn: func(b *Bootstrap) error { return b.InitializeActor() }, // 由 actor 配置段自身决定
			stopFn: func(b *Bootstrap, _ stdctx.Context) error {
				if b.actorSystem != nil {
					return b.actorSystem.Shutdown()
				}
				return nil
			},
		},
	}

	if boot.registry == nil {
		boot.registry = kernel.NewRegistry()
	}
	for _, def := range builtin {
		// 默认注册表中的同名插件视为应用的替换实现；已注册的保持幂等。
		if _, exists := kernel.GetRegistered(def.name); exists {
			continue
		}
		if _, exists := boot.registry.Get(def.name); exists {
			continue
		}
		p := *def
		p.boot = boot
		boot.registry.MustRegister(&p)
	}
}
