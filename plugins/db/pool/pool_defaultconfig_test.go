package pool

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxSize != 10 {
		t.Errorf("MaxSize = %d, want 10", cfg.MaxSize)
	}
	if cfg.InitialSize != 2 {
		t.Errorf("InitialSize = %d, want 2", cfg.InitialSize)
	}
	if cfg.MaxIdleTime != 30*time.Minute {
		t.Errorf("MaxIdleTime = %v, want 30m", cfg.MaxIdleTime)
	}
	if cfg.HealthCheckPeriod != 30*time.Second {
		t.Errorf("HealthCheckPeriod = %v, want 30s", cfg.HealthCheckPeriod)
	}
	if cfg.MaxFailures != 3 {
		t.Errorf("MaxFailures = %d, want 3", cfg.MaxFailures)
	}
	if cfg.AcquireTimeout != 5*time.Second {
		t.Errorf("AcquireTimeout = %v, want 5s", cfg.AcquireTimeout)
	}
	if cfg.RetryDelay != time.Second {
		t.Errorf("RetryDelay = %v, want 1s", cfg.RetryDelay)
	}
}
