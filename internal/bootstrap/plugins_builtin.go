package bootstrap

import (
	stdctx "context"

	"github.com/alldev-run/golang-gin-rpc/pkg/kernel"
)

// 内置组件以 kernel.Plugin 形式接入框架加载机制：
//   - 链接期：registerBuiltinPlugins 注册了哪些适配器，哪些组件参与构建；
//   - 运行期：plugins.<name>.enabled 或旧的 framework.init_* 开关决定是否启用；
//   - 启动顺序：由 depends 声明 + kernel 拓扑排序，替代原来的硬编码 if 链。
//
// 插件模块（plugins/*）保持纯库；谁装配、按什么顺序装配由框架（组合根）负责。

// builtinPlugin 把 Bootstrap 上的 Initialize/Stop 逻辑包装成 kernel.Plugin，
// 实例在注册时绑定。
type builtinPlugin struct {
	boot *Bootstrap

	name      string
	depends   []string
	enabledFn func(b *Bootstrap) bool
	startFn   func(b *Bootstrap) error
	stopFn    func(b *Bootstrap, ctx stdctx.Context) error
}

func (p *builtinPlugin) Name() string      { return p.name }
func (p *builtinPlugin) Depends() []string { return p.depends }

func (p *builtinPlugin) Configure(_ *kernel.Context) error {
	if p.enabledFn != nil && !p.enabledFn(p.boot) {
		return kernel.ErrDisabled
	}
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

// registerBuiltinPlugins 把框架内置组件注册到 kernel 注册表（幂等）。
func registerBuiltinPlugins(boot *Bootstrap) {
	builtin := []*builtinPlugin{
		{
			name:      "db",
			enabledFn: func(b *Bootstrap) bool { return b.config.Framework.InitDatabases },
			startFn:   func(b *Bootstrap) error { return b.InitializeDatabases() },
		},
		{
			name:      "cache",
			enabledFn: func(b *Bootstrap) bool { return b.config.Framework.InitCache },
			startFn:   func(b *Bootstrap) error { return b.InitializeCache() },
			stopFn: func(b *Bootstrap, _ stdctx.Context) error {
				if b.cache != nil {
					return b.cache.Close()
				}
				return nil
			},
		},
		{
			name:      "discovery",
			enabledFn: func(b *Bootstrap) bool { return b.config.Framework.InitDiscovery },
			startFn:   func(b *Bootstrap) error { return b.InitializeDiscovery() },
			stopFn: func(b *Bootstrap, _ stdctx.Context) error {
				if b.discoveryManager != nil {
					return b.discoveryManager.Stop()
				}
				return nil
			},
		},
		{
			name:      "tracing",
			enabledFn: func(b *Bootstrap) bool { return b.config.Framework.InitTracing },
			startFn:   func(b *Bootstrap) error { return b.InitializeTracing() },
			stopFn: func(b *Bootstrap, ctx stdctx.Context) error {
				if b.tracer != nil {
					return b.tracer.Shutdown(ctx)
				}
				return nil
			},
		},
		{
			name:      "auth",
			enabledFn: func(b *Bootstrap) bool { return b.config.Framework.InitAuth },
			startFn:   func(b *Bootstrap) error { return b.InitializeAuth() },
		},
		{
			name:      "metrics",
			enabledFn: func(b *Bootstrap) bool { return b.config.Framework.InitMetrics },
			startFn:   func(b *Bootstrap) error { return b.InitializeMetrics() },
		},
		{
			name:      "health",
			enabledFn: func(b *Bootstrap) bool { return b.config.Framework.InitHealth },
			depends:   []string{"db", "cache"},
			startFn:   func(b *Bootstrap) error { return b.InitializeHealth() },
		},
		{
			name:      "errors",
			enabledFn: func(b *Bootstrap) bool { return b.config.Framework.InitErrors },
			startFn:   func(b *Bootstrap) error { return b.InitializeErrors() },
		},
		{
			name:    "upload",
			startFn: func(b *Bootstrap) error { return b.InitializeUpload() }, // 由 upload 配置段自身决定
		},
	}

	for _, def := range builtin {
		if _, exists := kernel.GetRegistered(def.name); exists {
			continue
		}
		p := *def
		p.boot = boot
		kernel.MustRegister(&p)
	}
}
