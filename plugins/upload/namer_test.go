package upload

import (
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestUUIDNamer(t *testing.T) {
	withExt := NewUUIDNamer(true).Generate("report.pdf")
	if !strings.HasSuffix(withExt, ".pdf") {
		t.Errorf("Generate = %q, want preserved .pdf extension", withExt)
	}
	if _, err := uuid.Parse(strings.TrimSuffix(withExt, ".pdf")); err != nil {
		t.Errorf("base name %q is not a UUID: %v", withExt, err)
	}

	withoutExt := NewUUIDNamer(false).Generate("report.pdf")
	if strings.Contains(withoutExt, ".") {
		t.Errorf("Generate = %q, want no extension when preserveExtension is false", withoutExt)
	}
	if _, err := uuid.Parse(withoutExt); err != nil {
		t.Errorf("Generate = %q is not a UUID: %v", withoutExt, err)
	}
}

func TestTimestampNamer(t *testing.T) {
	defaultFmt := NewTimestampNamer(true, "").Generate("log.txt")
	if !regexp.MustCompile(`^\d{14}\.txt$`).MatchString(defaultFmt) {
		t.Errorf("Generate = %q, want default 14-digit timestamp plus extension", defaultFmt)
	}

	custom := NewTimestampNamer(true, "2006-01-02").Generate("log.txt")
	if !regexp.MustCompile(`^\d{4}-\d{2}-\d{2}\.txt$`).MatchString(custom) {
		t.Errorf("Generate = %q, want custom date format plus extension", custom)
	}

	noExt := NewTimestampNamer(false, "").Generate("log.txt")
	if !regexp.MustCompile(`^\d{14}$`).MatchString(noExt) {
		t.Errorf("Generate = %q, want bare timestamp without extension", noExt)
	}
}

func TestOriginalNamer(t *testing.T) {
	got := NewOriginalNamer().Generate("keep me.txt")
	if got != "keep me.txt" {
		t.Errorf("Generate = %q, want original name unchanged", got)
	}
}

func digitsOnly(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func TestCustomNamerAllPlaceholders(t *testing.T) {
	n := NewCustomNamer("{uuid}_{timestamp}_{date}_{original}_{random}", true)
	got := n.Generate("photo.jpg")

	if !strings.HasSuffix(got, ".jpg") {
		t.Errorf("Generate = %q, want preserved .jpg extension", got)
	}
	parts := strings.Split(strings.TrimSuffix(got, ".jpg"), "_")
	if len(parts) != 5 {
		t.Fatalf("Generate = %q, want 5 placeholder segments separated by '_'", got)
	}
	if _, err := uuid.Parse(parts[0]); err != nil {
		t.Errorf("{uuid} segment %q is not a UUID: %v", parts[0], err)
	}
	if len(parts[1]) != 14 || !digitsOnly(parts[1]) {
		t.Errorf("{timestamp} segment %q is not a 14-digit timestamp", parts[1])
	}
	if len(parts[2]) != 8 || !digitsOnly(parts[2]) {
		t.Errorf("{date} segment %q is not an 8-digit date", parts[2])
	}
	if parts[3] != "photo" {
		t.Errorf("{original} segment = %q, want base name without extension %q", parts[3], "photo")
	}
	if !digitsOnly(parts[4]) {
		t.Errorf("{random} segment %q is not numeric", parts[4])
	}
}

func TestCustomNamerVariants(t *testing.T) {
	originalOnly := NewCustomNamer("{original}", false).Generate("photo.jpg")
	if originalOnly != "photo" {
		t.Errorf("Generate = %q, want %q (no extension preserved)", originalOnly, "photo")
	}

	uuidOnly := NewCustomNamer("{uuid}", false).Generate("x.bin")
	if _, err := uuid.Parse(uuidOnly); err != nil {
		t.Errorf("Generate = %q is not a bare UUID: %v", uuidOnly, err)
	}

	static := NewCustomNamer("static-name", true).Generate("photo.jpg")
	if static != "static-name.jpg" {
		t.Errorf("Generate = %q, want template literal plus extension", static)
	}
}

func TestGetNamer(t *testing.T) {
	cases := []struct {
		strategy string
		want     Namer
	}{
		{"uuid", NewUUIDNamer(true)},
		{"timestamp", NewTimestampNamer(true, "")},
		{"original", NewOriginalNamer()},
		{"custom", NewCustomNamer("{original}", true)},
		// Unknown strategies fall back to the UUID namer.
		{"mystery-strategy", NewUUIDNamer(true)},
		{"", NewUUIDNamer(true)},
	}
	for _, tc := range cases {
		t.Run("strategy="+tc.strategy, func(t *testing.T) {
			got := GetNamer(tc.strategy, "{original}", true)
			if got == nil {
				t.Fatal("GetNamer returned nil")
			}
			// Concrete namer types carry no comparable state beyond the
			// constructor inputs; compare via generated output shape instead
			// of relying on struct equality of interface values.
			gotType, wantType := typeName(got), typeName(tc.want)
			if gotType != wantType {
				t.Errorf("GetNamer(%q) returned %s, want %s", tc.strategy, gotType, wantType)
			}
		})
	}
}

func typeName(n Namer) string {
	switch n.(type) {
	case *UUIDNamer:
		return "*UUIDNamer"
	case *TimestampNamer:
		return "*TimestampNamer"
	case *OriginalNamer:
		return "*OriginalNamer"
	case *CustomNamer:
		return "*CustomNamer"
	default:
		return "unknown"
	}
}
