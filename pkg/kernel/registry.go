package kernel

import (
	"fmt"
	"sync"
)

// Registry 保存一组已注册插件。包级函数 Register/Registered 等操作进程级默认
// 注册表（插件包 init() 中空导入即注册 —— 链接期裁剪）；需要多实例隔离时用
// NewRegistry 创建独立注册表，并通过 Loader 的 WithRegistry 选项使用。
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]Plugin
	order   []string
}

// NewRegistry 创建一个空的独立注册表。
func NewRegistry() *Registry {
	return &Registry{plugins: make(map[string]Plugin)}
}

var defaultRegistry = NewRegistry()

// DefaultRegistry 返回进程级默认注册表。
func DefaultRegistry() *Registry { return defaultRegistry }

// Register 注册一个插件。重复注册同名插件会返回错误。
func (r *Registry) Register(p Plugin) error {
	if p == nil {
		return fmt.Errorf("kernel: register nil plugin")
	}
	name := p.Name()
	if name == "" {
		return fmt.Errorf("kernel: plugin name is empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.plugins[name]; exists {
		return fmt.Errorf("kernel: plugin %q already registered", name)
	}
	r.plugins[name] = p
	r.order = append(r.order, name)
	return nil
}

// MustRegister 语义同 Register，失败时 panic。
func (r *Registry) MustRegister(p Plugin) {
	if err := r.Register(p); err != nil {
		panic(err)
	}
}

// Plugins 按注册顺序返回已注册插件的快照。
func (r *Registry) Plugins() []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Plugin, 0, len(r.order))
	for _, name := range r.order {
		out = append(out, r.plugins[name])
	}
	return out
}

// Get 按名称查找已注册插件。
func (r *Registry) Get(name string) (Plugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.plugins[name]
	return p, ok
}

// Reset 清空注册表。
func (r *Registry) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.plugins = make(map[string]Plugin)
	r.order = nil
}

// Register 向默认注册表注册一个插件。重复注册同名插件会返回错误。
func Register(p Plugin) error { return defaultRegistry.Register(p) }

// MustRegister 语义同 Register，失败时 panic（适用于包 init 场景）。
func MustRegister(p Plugin) { defaultRegistry.MustRegister(p) }

// Registered 按注册顺序返回默认注册表中所有插件的快照。
func Registered() []Plugin { return defaultRegistry.Plugins() }

// GetRegistered 按名称在默认注册表中查找插件。
func GetRegistered(name string) (Plugin, bool) { return defaultRegistry.Get(name) }

// Reset 清空默认注册表，仅供测试使用。
func Reset() { defaultRegistry.Reset() }
