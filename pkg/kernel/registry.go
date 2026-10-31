package kernel

import (
	"fmt"
	"sync"
)

// registry 保存所有通过 Register 注册的插件。
// 注册通常发生在插件包的 init() 中（应用空导入插件包时触发），
// 因此实际参与构建的插件集合由应用的 import 决定 —— 链接期裁剪。
var registry = struct {
	mu      sync.RWMutex
	plugins map[string]Plugin
	order   []string
}{plugins: make(map[string]Plugin)}

// Register 注册一个插件。重复注册同名插件会返回错误。
func Register(p Plugin) error {
	if p == nil {
		return fmt.Errorf("kernel: register nil plugin")
	}
	name := p.Name()
	if name == "" {
		return fmt.Errorf("kernel: plugin name is empty")
	}

	registry.mu.Lock()
	defer registry.mu.Unlock()
	if _, exists := registry.plugins[name]; exists {
		return fmt.Errorf("kernel: plugin %q already registered", name)
	}
	registry.plugins[name] = p
	registry.order = append(registry.order, name)
	return nil
}

// MustRegister 语义同 Register，失败时 panic（适用于包 init 场景）。
func MustRegister(p Plugin) {
	if err := Register(p); err != nil {
		panic(err)
	}
}

// Registered 按注册顺序返回当前所有已注册插件的快照。
func Registered() []Plugin {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	out := make([]Plugin, 0, len(registry.order))
	for _, name := range registry.order {
		out = append(out, registry.plugins[name])
	}
	return out
}

// GetRegistered 按名称查找已注册插件。
func GetRegistered(name string) (Plugin, bool) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	p, ok := registry.plugins[name]
	return p, ok
}

// Reset 清空注册表，仅供测试使用。
func Reset() {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.plugins = make(map[string]Plugin)
	registry.order = nil
}
