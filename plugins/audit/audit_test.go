package audit

import (
	"context"
	"testing"
)

func TestMaskerMaskMap(t *testing.T) {
	masker := NewMasker([]string{"password", "authorization"})
	in := map[string]interface{}{
		"password": "abc",
		"normal":   "ok",
	}
	out := masker.MaskMap(in)
	if out["password"] != maskedValue {
		t.Fatalf("expected password to be masked")
	}
	if out["normal"] != "ok" {
		t.Fatalf("expected normal field unchanged")
	}
}

type testSink struct {
	count int
}

func (s *testSink) Write(ctx context.Context, event Event) error {
	s.count++
	return nil
}

func TestMultiSink(t *testing.T) {
	s1 := &testSink{}
	s2 := &testSink{}
	multi := NewMultiSink(s1, s2)

	err := multi.Write(context.Background(), Event{Action: ActionCustom})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s1.count != 1 || s2.count != 1 {
		t.Fatalf("unexpected sink counts: %d, %d", s1.count, s2.count)
	}
}
