package bootstrap

import (
	"strings"
	"testing"

	"github.com/alldev-run/golang-gin-rpc/pkg/config"
)

func TestRegisterServiceDependenciesDrivesCoverage(t *testing.T) {
	b := &Bootstrap{config: &config.GlobalConfig{}, dependencies: map[string]interface{}{}}
	b.RegisterServiceDependencies("billing", "billing.client")

	err := b.ValidateDependencyCoverage(FrameworkOptions{Services: []string{"billing"}})
	if err == nil || !strings.Contains(err.Error(), "billing.client") {
		t.Fatalf("expected missing billing.client, got %v", err)
	}

	b.SetDependency("billing.client", struct{}{})
	if err := b.ValidateDependencyCoverage(FrameworkOptions{Services: []string{"billing"}}); err != nil {
		t.Fatalf("ValidateDependencyCoverage() error = %v", err)
	}
}
