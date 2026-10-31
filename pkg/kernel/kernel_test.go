package kernel

import (
	stdctx "context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// fakeConfig 简单的内存配置源（yaml 文本）。
type fakeConfig struct {
	sections map[string]string
}

func (f *fakeConfig) UnmarshalSection(name string, out any) (bool, error) {
	raw, ok := f.sections[name]
	if !ok {
		return false, nil
	}
	return true, yaml.Unmarshal([]byte(raw), out)
}

// fakePlugin 记录生命周期调用轨迹。
type fakePlugin struct {
	name     string
	depends  []string
	disabled bool // Configure 时主动放弃
	startErr error
	stopErr  error

	mu      sync.Mutex
	actions *[]string
	set     map[string]any // Start 时注册的服务
}

func (p *fakePlugin) Name() string      { return p.name }
func (p *fakePlugin) Depends() []string { return p.depends }

func (p *fakePlugin) Configure(_ *Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.disabled {
		return ErrDisabled
	}
	if p.actions != nil {
		*p.actions = append(*p.actions, "configure:"+p.name)
	}
	return nil
}

func (p *fakePlugin) Start(ctx *Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.actions != nil {
		*p.actions = append(*p.actions, "start:"+p.name)
	}
	for k, v := range p.set {
		ctx.SetService(k, v)
	}
	return p.startErr
}

func (p *fakePlugin) Stop(_ *Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.actions != nil {
		*p.actions = append(*p.actions, "stop:"+p.name)
	}
	return p.stopErr
}

func buildApp(t *testing.T, source ConfigSource) *App {
	t.Helper()
	app, err := NewLoader(source).Build()
	require.NoError(t, err)
	return app
}

func TestTopoOrder(t *testing.T) {
	Reset()
	defer Reset()
	var actions []string

	base := &fakePlugin{name: "base", actions: &actions}
	mid := &fakePlugin{name: "mid", depends: []string{"base"}, actions: &actions}
	top := &fakePlugin{name: "top", depends: []string{"mid", "base"}, actions: &actions}

	// 故意按依赖的逆序注册，验证 Build 后按依赖序启停。
	MustRegister(top)
	MustRegister(mid)
	MustRegister(base)

	app := buildApp(t, &fakeConfig{})
	require.NoError(t, app.Start(stdctx.Background()))
	require.NoError(t, app.Stop(stdctx.Background()))

	assert.Equal(t, []string{
		"configure:top", "configure:mid", "configure:base",
		"start:base", "start:mid", "start:top",
		"stop:top", "stop:mid", "stop:base",
	}, actions)
}

func TestDisabledByConfig(t *testing.T) {
	Reset()
	defer Reset()
	// c 独立（无下游依赖者），禁用后应从启动列表消失。
	MustRegister(&fakePlugin{name: "a"})
	MustRegister(&fakePlugin{name: "b"})
	MustRegister(&fakePlugin{name: "c"})

	// 顶层段名就是 plugins，内容直接是插件名 → 开关的映射。
	src := &fakeConfig{sections: map[string]string{
		"plugins": "c:\n  enabled: false",
	}}

	app := buildApp(t, src)
	var names []string
	for _, p := range app.Plugins() {
		names = append(names, p.Name())
	}
	assert.Equal(t, []string{"a", "b"}, names)
}

func TestDefaultEnabledWithoutConfig(t *testing.T) {
	Reset()
	defer Reset()
	MustRegister(&fakePlugin{name: "solo"})

	// 无任何 plugins 配置段 → 默认启用。
	app := buildApp(t, &fakeConfig{})
	assert.Len(t, app.Plugins(), 1)
}

func TestOptOutDuringConfigure(t *testing.T) {
	Reset()
	defer Reset()
	MustRegister(&fakePlugin{name: "a", disabled: true})

	app := buildApp(t, &fakeConfig{})
	assert.Empty(t, app.Plugins())
}

func TestMissingDependency(t *testing.T) {
	Reset()
	defer Reset()
	// b 依赖 a，但 a 未注册。
	MustRegister(&fakePlugin{name: "b", depends: []string{"a"}})

	_, err := NewLoader(&fakeConfig{}).Build()
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"b" depends on "a"`)
}

func TestDisabledDependency(t *testing.T) {
	Reset()
	defer Reset()
	MustRegister(&fakePlugin{name: "a"})
	MustRegister(&fakePlugin{name: "b", depends: []string{"a"}})

	src := &fakeConfig{sections: map[string]string{
		"plugins": "a:\n  enabled: false",
	}}
	_, err := NewLoader(src).Build()
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"b" depends on "a"`)
}

func TestDependencyCycle(t *testing.T) {
	Reset()
	defer Reset()
	MustRegister(&fakePlugin{name: "x", depends: []string{"y"}})
	MustRegister(&fakePlugin{name: "y", depends: []string{"x"}})

	_, err := NewLoader(&fakeConfig{}).Build()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dependency cycle")
}

func TestStartFailureRollback(t *testing.T) {
	Reset()
	defer Reset()
	var actions []string
	MustRegister(&fakePlugin{name: "a", actions: &actions})
	MustRegister(&fakePlugin{name: "b", depends: []string{"a"}, actions: &actions})
	MustRegister(&fakePlugin{name: "c", depends: []string{"b"}, actions: &actions,
		startErr: fmt.Errorf("boom")})

	app := buildApp(t, &fakeConfig{})
	err := app.Start(stdctx.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"c"`)

	// a、b 已启动，应被回滚停止；c 的 start 动作先记录再返回错误。
	assert.Equal(t, []string{
		"configure:a", "configure:b", "configure:c",
		"start:a", "start:b", "start:c",
		"stop:b", "stop:a", // 逆序回滚
	}, actions)
}

func TestServiceLocator(t *testing.T) {
	Reset()
	defer Reset()

	MustRegister(&fakePlugin{name: "provider", set: map[string]any{"cache": "i-am-cache"}})
	MustRegister(&fakePlugin{name: "consumer", depends: []string{"provider"}})

	app := buildApp(t, &fakeConfig{})
	require.NoError(t, app.Start(stdctx.Background()))
	defer app.Stop(stdctx.Background())

	v, ok := app.Context().GetService("cache")
	assert.True(t, ok)
	assert.Equal(t, "i-am-cache", v)

	_, ok = app.Context().GetService("nope")
	assert.False(t, ok)
}

func TestDuplicateRegister(t *testing.T) {
	Reset()
	defer Reset()
	MustRegister(&fakePlugin{name: "dup"})
	err := Register(&fakePlugin{name: "dup"})
	assert.ErrorContains(t, err, "already registered")
}

func TestEmptyRegistry(t *testing.T) {
	Reset()
	defer Reset()
	app := buildApp(t, &fakeConfig{})
	assert.Empty(t, app.Plugins())
	assert.NoError(t, app.Start(stdctx.Background()))
}

func TestPluginLookup(t *testing.T) {
	Reset()
	defer Reset()
	p := &fakePlugin{name: "lookup"}
	MustRegister(p)

	app := buildApp(t, &fakeConfig{})
	got, ok := app.Plugin("lookup")
	assert.True(t, ok)
	assert.Same(t, Plugin(p), got)

	_, ok = app.Plugin("absent")
	assert.False(t, ok)
}
