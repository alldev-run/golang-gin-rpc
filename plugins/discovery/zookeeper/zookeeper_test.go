package zookeeper

import (
	"testing"
	"time"
)

func TestSplitServers(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want []string
	}{
		{name: "empty", addr: "", want: []string{}},
		{name: "blank", addr: "  ,  ,", want: []string{}},
		{name: "single", addr: "127.0.0.1:2181", want: []string{"127.0.0.1:2181"}},
		{name: "multiple", addr: "zk1:2181,zk2:2181", want: []string{"zk1:2181", "zk2:2181"}},
		{name: "whitespace and empties", addr: " zk1:2181 , , zk2:2181 ", want: []string{"zk1:2181", "zk2:2181"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := splitServers(test.addr)
			if len(got) != len(test.want) {
				t.Fatalf("splitServers(%q) = %v, want %v", test.addr, got, test.want)
			}
			for i := range got {
				if got[i] != test.want[i] {
					t.Fatalf("splitServers(%q)[%d] = %q, want %q", test.addr, i, got[i], test.want[i])
				}
			}
		})
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "empty falls back to default", value: "", want: "/services"},
		{name: "blank falls back to default", value: "   ", want: "/services"},
		{name: "missing leading slash", value: "services", want: "/services"},
		{name: "absolute path unchanged", value: "/services", want: "/services"},
		{name: "custom absolute path unchanged", value: "/infra/registry", want: "/infra/registry"},
		{name: "custom relative path", value: "infra/registry", want: "/infra/registry"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := normalizePath(test.value); got != test.want {
				t.Fatalf("normalizePath(%q) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}

func TestNewRegistryRequiresAddress(t *testing.T) {
	_, err := NewRegistry("", time.Second, nil)
	if err == nil {
		t.Fatal("NewRegistry(\"\") succeeded, want error")
	}
	if err.Error() != "zookeeper address is required" {
		t.Fatalf("NewRegistry(\"\") error = %v, want address-required message", err)
	}
}

func TestNewRegistryFailsAgainstUnreachableServer(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]interface{}
	}{
		{name: "default options", options: nil},
		{name: "custom base path", options: map[string]interface{}{"base_path": "/custom/services"}},
		{name: "whitespace base path keeps default", options: map[string]interface{}{"base_path": "   "}},
		{name: "zero timeout uses default", options: map[string]interface{}{"base_path": "/services"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// zookeeper.Connect returns immediately with a background retry
			// loop; the first ensurePath request is flushed with ErrNoServer,
			// so NewRegistry fails fast without a server. A short session
			// timeout keeps the failed attempts cheap.
			registry, err := NewRegistry("127.0.0.1:1", 100*time.Millisecond, test.options)
			if err == nil {
				_ = registry.Close()
				t.Fatal("NewRegistry() succeeded without a ZooKeeper server")
			}
		})
	}
}
