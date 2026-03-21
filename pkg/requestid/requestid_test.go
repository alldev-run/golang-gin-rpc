package requestid

import (
	"regexp"
	"testing"
)

func TestMustNew(t *testing.T) {
	id := MustNew()
	if id == "" {
		t.Fatalf("expected non-empty id")
	}
	if len(id) != 26 {
		t.Fatalf("expected length 26, got %d (%q)", len(id), id)
	}
	// lower-case base32 (no padding)
	re := regexp.MustCompile(`^[a-z2-7]{26}$`)
	if !re.MatchString(id) {
		t.Fatalf("unexpected format: %q", id)
	}
}
