package kernel

import (
	stdctx "context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
	"go.uber.org/zap"
)

// pluginsConfig 对应配置文件的顶层 plugins 段：{ <插件名>: { enabled: bool } }。
type pluginToggle struct {
	Enabled *bool `yaml:"enabled" json:"enabled"`
}

// Loader 从注册表构建一次可启动的插件应用。
type Loader struct {
	source ConfigSource
	log    *zap.Logger
}

// LoaderOption 定制 Loader。
type LoaderOption func(*Loader)

// WithLogger 指定框架日志器。
func WithLogger(l *zap.Logger) LoaderOption {
	return func(lo *Loader) { lo.log = l }
}

// NewLoader 创建插件加载器。source 用于读取 plugins.* 配置段。
func NewLoader(source ConfigSource, opts ...LoaderOption) *Loader {
	l := &Loader{source: source, log: logger.L()}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// App 是按依赖序排列、可整体启停的插件集合。
type App struct {
	plugins  []Plugin
	context  *Context
	log      *zap.Logger
	mu       sync.RWMutex
	started  []string
	services *serviceRegistry
}

// Build 执行过滤 → 配置 → 拓扑排序，得到可启动的 App。
//
// 参与构建的插件 = 已注册 && 配置未显式禁用 && Configure 未返回 ErrDisabled。
// 任一已启用插件依赖了被禁用或未注册的插件时报错。
func (l *Loader) Build() (*App, error) {
	registered := Registered()

	// 1. 运行期裁剪：读 plugins.<name>.enabled，默认启用。
	enabled := make(map[string]Plugin, len(registered))
	for _, p := range registered {
		if !l.isEnabled(p.Name()) {
			l.log.Info("kernel: plugin disabled by config", zap.String("plugin", p.Name()))
			continue
		}
		enabled[p.Name()] = p
	}

	// 2. Configure：让插件解析自己的配置段，仍可主动放弃加载。
	services := newServiceRegistry()
	shared := &Context{Context: stdctx.Background(), source: l.source, services: services, log: l.log}
	configured := make(map[string]Plugin, len(enabled))
	for _, p := range registered {
		if _, ok := enabled[p.Name()]; !ok {
			continue
		}
		if err := p.Configure(shared.clone(stdctx.Background(), p.Name())); err != nil {
			if errors.Is(err, ErrDisabled) {
				l.log.Info("kernel: plugin opted out during configure", zap.String("plugin", p.Name()))
				delete(enabled, p.Name())
				continue
			}
			return nil, fmt.Errorf("kernel: configure plugin %q: %w", p.Name(), err)
		}
		configured[p.Name()] = p
	}

	// 3. 依赖校验：所有已启用插件的依赖必须同样已启用（软依赖除外）。
	for name, p := range configured {
		for _, dep := range p.Depends() {
			if _, ok := configured[dep]; !ok {
				return nil, fmt.Errorf("kernel: plugin %q depends on %q which is disabled or not registered (import the plugin package or enable it in config)",
					name, dep)
			}
		}
	}

	// 4. 拓扑排序（Kahn，注册序为稳定次序）。
	order, err := topoSort(configured)
	if err != nil {
		return nil, err
	}

	plugins := make([]Plugin, 0, len(order))
	for _, name := range order {
		plugins = append(plugins, configured[name])
	}
	l.log.Info("kernel: plugin app built",
		zap.Int("plugins", len(plugins)),
		zap.String("order", strings.Join(order, " -> ")))
	return &App{plugins: plugins, context: shared, log: l.log, services: services}, nil
}

// softDepends 返回插件的软依赖列表（未实现 SoftDepender 则为空）。
func softDepends(p Plugin) []string {
	if sd, ok := p.(SoftDepender); ok {
		return sd.SoftDepends()
	}
	return nil
}

// isEnabled 读取 plugins.<name>.enabled，未配置时默认 true。
func (l *Loader) isEnabled(name string) bool {
	if l.source == nil {
		return true
	}
	// 配置形态：plugins: { cache: { enabled: true }, ... }
	var section map[string]pluginToggle
	found, err := l.source.UnmarshalSection("plugins", &section)
	if err != nil || !found {
		return true
	}
	t, ok := section[name]
	if !ok || t.Enabled == nil {
		return true
	}
	return *t.Enabled
}

// topoSort 对 configured 中的插件按依赖排序；结果确定（依赖相同时按注册序）。
func topoSort(configured map[string]Plugin) ([]string, error) {
	// 注册序作为平局决胜。
	registered := Registered()
	seq := make(map[string]int, len(registered))
	for i, p := range registered {
		seq[p.Name()] = i
	}

	indegree := make(map[string]int, len(configured))
	dependents := make(map[string][]string, len(configured))
	for name, p := range configured {
		// 硬依赖已在上面校验过必然存在；软依赖仅在目标参与构建时计入顺序。
		deps := dedupe(p.Depends())
		for _, dep := range dedupe(softDepends(p)) {
			if _, ok := configured[dep]; ok && !contains(deps, dep) {
				deps = append(deps, dep)
			}
		}
		indegree[name] = len(deps)
		for _, dep := range deps {
			dependents[dep] = append(dependents[dep], name)
		}
	}

	queue := make([]string, 0, len(configured))
	for name := range configured {
		if indegree[name] == 0 {
			queue = append(queue, name)
		}
	}
	bySeq := func(a, b string) bool { return seq[a] < seq[b] }
	sortSliceStable(queue, bySeq)

	order := make([]string, 0, len(configured))
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		order = append(order, name)
		for _, next := range dependents[name] {
			indegree[next]--
			if indegree[next] == 0 {
				queue = append(queue, next)
			}
		}
		sortSliceStable(queue, bySeq)
	}

	if len(order) != len(configured) {
		// 收集环上的节点便于定位。
		remaining := make([]string, 0)
		for name := range configured {
			if !contains(order, name) {
				remaining = append(remaining, name)
			}
		}
		sortStrings(remaining)
		return nil, fmt.Errorf("kernel: dependency cycle detected among plugins: %s", strings.Join(remaining, ", "))
	}
	return order, nil
}

// Start 按依赖顺序启动所有插件；失败时回滚已启动的插件（逆序 Stop）。
func (a *App) Start(lifecycle stdctx.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, p := range a.plugins {
		pctx := a.context.clone(lifecycle, p.Name())
		a.log.Info("kernel: starting plugin", zap.String("plugin", p.Name()))
		if err := p.Start(pctx); err != nil {
			a.rollbackLocked(pctx)
			return fmt.Errorf("kernel: start plugin %q: %w", p.Name(), err)
		}
		a.started = append(a.started, p.Name())
	}
	a.log.Info("kernel: all plugins started", zap.Int("count", len(a.started)))
	return nil
}

// Stop 按依赖逆序停止所有插件，聚合全部错误。
func (a *App) Stop(lifecycle stdctx.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.rollbackLocked(a.context.clone(lifecycle, ""))
}

func (a *App) rollbackLocked(ctx *Context) error {
	var errs []error
	for i := len(a.started) - 1; i >= 0; i-- {
		name := a.started[i]
		p, ok := findPlugin(a.plugins, name)
		if !ok {
			continue
		}
		a.log.Info("kernel: stopping plugin", zap.String("plugin", name))
		if err := p.Stop(ctx); err != nil {
			errs = append(errs, fmt.Errorf("stop plugin %q: %w", name, err))
		}
	}
	a.started = nil
	return errors.Join(errs...)
}

// Plugins 返回按启动顺序排列的插件快照。
func (a *App) Plugins() []Plugin {
	out := make([]Plugin, len(a.plugins))
	copy(out, a.plugins)
	return out
}

// Started 返回指定插件当前是否处于已启动状态。
func (a *App) Started(name string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, n := range a.started {
		if n == name {
			return true
		}
	}
	return false
}

// Plugin 按名称查找已启动集合中的插件。
func (a *App) Plugin(name string) (Plugin, bool) {
	p, ok := findPlugin(a.plugins, name)
	return p, ok
}

// Context 返回插件共享的运行上下文（服务定位器）。
func (a *App) Context() *Context { return a.context }

// --- 小工具 ---

func findPlugin(plugins []Plugin, name string) (Plugin, bool) {
	for _, p := range plugins {
		if p.Name() == name {
			return p, true
		}
	}
	return nil, false
}

func dedupe(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := in[:0]
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// sortSliceStable/sortStrings 薄封装，避免 loader 直接依赖 sort 的散乱调用。
func sortSliceStable(list []string, less func(a, b string) bool) {
	// insertion sort：插件数量级小（几十个），足够且稳定。
	for i := 1; i < len(list); i++ {
		for j := i; j > 0 && less(list[j], list[j-1]); j-- {
			list[j], list[j-1] = list[j-1], list[j]
		}
	}
}

func sortStrings(list []string) {
	sortSliceStable(list, func(a, b string) bool { return a < b })
}
