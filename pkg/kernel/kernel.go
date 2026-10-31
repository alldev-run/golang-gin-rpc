// Package kernel 提供框架的插件化加载机制。
//
// 插件通过实现 Plugin 接口并调用 Register 自注册；应用侧只 import 需要的
// 插件包（链接期裁剪），再由 Loader 根据配置决定哪些插件参与启动（运行期裁剪）。
// 插件之间通过 Depends 声明启动顺序依赖，Loader 负责拓扑排序与环检测。
//
// 典型用法（组合根）：
//
//	import _ "github.com/alldev-run/golang-gin-rpc/plugins/cache" // 空导入触发注册
//
//	app, err := kernel.NewLoader(configSource).Build()
//	if err != nil { ... }
//	if err := app.Start(ctx); err != nil { ... }
//	defer app.Stop(ctx)
package kernel

import (
	stdctx "context"
	"errors"
	"fmt"
	"sync"

	"go.uber.org/zap"
)

// ErrDisabled 可由 Configure 返回，表示该插件根据配置决定不启用。
var ErrDisabled = errors.New("kernel: plugin disabled")

// Plugin 是框架插件的生命周期契约。
type Plugin interface {
	// Name 返回插件唯一名称，同时是配置段 plugins.<name> 的键。
	Name() string

	// Depends 返回依赖的其他插件名称，Loader 据此做拓扑排序。
	// 未启用的依赖、或未注册的依赖都会导致构建失败。
	Depends() []string

	// Configure 在启动前调用一次，用于解析插件自身配置段并保存。
	// 此阶段不应产生副作用（不开连接、不起 goroutine）。
	// 返回 ErrDisabled 表示放弃加载本插件。
	Configure(ctx *Context) error

	// Start 启动插件（建连接、起后台任务、向 ctx 注册本插件提供的服务等），
	// 按依赖序调用。
	Start(ctx *Context) error

	// Stop 优雅停止插件，按依赖的逆序调用。
	Stop(ctx *Context) error
}

// ConfigSource 由配置层实现，供插件读取自己的原始配置段，
// 使核心配置结构不必感知任何插件的配置结构体。
type ConfigSource interface {
	// UnmarshalSection 将顶层 yaml 段 name 反序列化到 out。
	// 段不存在时返回 (false, nil)。
	UnmarshalSection(name string, out any) (bool, error)
}

// Context 是传递给插件的生命周期上下文，同时充当插件间的服务定位器。
// Start/Stop 阶段内嵌的 stdctx.Context 携带生命周期取消信号；Configure 阶段为 Background。
type Context struct {
	stdctx.Context

	pluginName string
	source     ConfigSource
	services   *serviceRegistry
	log        *zap.Logger
}

// clone 派生一个面向特定插件与生命周期信号的上下文。
func (c *Context) clone(lifecycle stdctx.Context, pluginName string) *Context {
	cp := *c
	cp.Context = lifecycle
	cp.pluginName = pluginName
	return &cp
}

// Name 返回当前正在被配置/启动的插件名。
func (c *Context) Name() string { return c.pluginName }

// Config 返回配置源，插件用它解析自己的配置段。
func (c *Context) Config() ConfigSource { return c.source }

// Logger 返回框架日志器。
func (c *Context) Logger() *zap.Logger { return c.log }

// SetService 将本插件提供的服务注册到定位器，供下游插件获取。
func (c *Context) SetService(name string, svc any) {
	c.services.set(name, svc)
}

// GetService 按名称获取上游插件注册的服务。
func (c *Context) GetService(name string) (any, bool) {
	return c.services.get(name)
}

// MustGetService 语义同 GetService，找不到时 panic（用于启动期强依赖装配）。
func (c *Context) MustGetService(name string) any {
	svc, ok := c.services.get(name)
	if !ok {
		panic(fmt.Sprintf("kernel: service %q not found (missing or disabled plugin?)", name))
	}
	return svc
}

// serviceRegistry 是并发安全的服务定位器。
type serviceRegistry struct {
	mu       sync.RWMutex
	services map[string]any
}

func newServiceRegistry() *serviceRegistry {
	return &serviceRegistry{services: make(map[string]any)}
}

func (r *serviceRegistry) set(name string, svc any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.services[name] = svc
}

func (r *serviceRegistry) get(name string) (any, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	svc, ok := r.services[name]
	return svc, ok
}
