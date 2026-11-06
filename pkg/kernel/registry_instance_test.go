package kernel

import "testing"

type namedPlugin struct{ name string }

func (p *namedPlugin) Name() string             { return p.name }
func (p *namedPlugin) Depends() []string        { return nil }
func (p *namedPlugin) Configure(*Context) error { return nil }
func (p *namedPlugin) Start(*Context) error     { return nil }
func (p *namedPlugin) Stop(*Context) error      { return nil }

func TestLoaderWithRegistryMergesDefaultRegistry(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	global := &namedPlugin{name: "shared"}
	MustRegister(global)
	MustRegister(&namedPlugin{name: "global-only"})

	local := NewRegistry()
	override := &namedPlugin{name: "shared"}
	local.MustRegister(override)
	local.MustRegister(&namedPlugin{name: "local-only"})

	app, err := NewLoader(nil, WithRegistry(local)).Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	got, ok := app.Plugin("shared")
	if !ok || got != override {
		t.Fatal("instance registry plugin should override the default registry plugin with the same name")
	}
	for _, name := range []string{"local-only", "global-only"} {
		if _, ok := app.Plugin(name); !ok {
			t.Fatalf("plugin %q missing from built app", name)
		}
	}
	if _, ok := GetRegistered("local-only"); ok {
		t.Fatal("instance registry plugins must not leak into the default registry")
	}
}
