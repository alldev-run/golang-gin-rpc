package upload

import (
	"mime/multipart"
	"strings"
	"testing"
)

func TestValidatorValidate(t *testing.T) {
	// PNG magic bytes: content sniffing reports image/png regardless of the filename.
	pngPayload := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}

	t.Run("valid file passes", func(t *testing.T) {
		up := newTestUploader(t, textUploadConfig)
		v := NewValidator(up.GetConfig())
		if err := v.Validate(mustFileHeader(t, "ok.txt", []byte("hello"))); err != nil {
			t.Fatalf("expected valid file to pass, got %v", err)
		}
	})

	t.Run("size exceeds limit", func(t *testing.T) {
		up := newTestUploader(t, func(c *Config) {
			textUploadConfig(c)
			c.MaxFileSize = 10
		})
		v := NewValidator(up.GetConfig())

		err := v.Validate(mustFileHeader(t, "big.txt", []byte("0123456789ABCDEF")))
		if err == nil {
			t.Fatal("expected size validation error")
		}
		verr, ok := err.(*ValidationError)
		if !ok {
			t.Fatalf("expected *ValidationError, got %T", err)
		}
		if verr.Field != "file_size" {
			t.Errorf("field = %q, want file_size", verr.Field)
		}
		if !strings.Contains(verr.Message, "exceeds maximum allowed size") {
			t.Errorf("message = %q, want size-limit wording", verr.Message)
		}
	})

	t.Run("extension rejected", func(t *testing.T) {
		up := newTestUploader(t, textUploadConfig)
		v := NewValidator(up.GetConfig())

		err := v.Validate(mustFileHeader(t, "evil.exe", []byte("nope")))
		if err == nil {
			t.Fatal("expected extension validation error")
		}
		verr := err.(*ValidationError)
		if verr.Field != "file_extension" {
			t.Errorf("field = %q, want file_extension", verr.Field)
		}
		if !strings.Contains(verr.Message, ".exe is not allowed") {
			t.Errorf("message = %q, want it to name the rejected extension", verr.Message)
		}
	})

	t.Run("mime type rejected despite allowed extension", func(t *testing.T) {
		up := newTestUploader(t, textUploadConfig) // only text/plain allowed
		v := NewValidator(up.GetConfig())

		err := v.Validate(mustFileHeader(t, "notes.txt", pngPayload))
		if err == nil {
			t.Fatal("expected MIME validation error")
		}
		verr := err.(*ValidationError)
		if verr.Field != "file_mime_type" {
			t.Errorf("field = %q, want file_mime_type", verr.Field)
		}
		if !strings.Contains(verr.Message, "image/png is not allowed") {
			t.Errorf("message = %q, want detected MIME type", verr.Message)
		}
	})

	t.Run("mime accepted via type prefix", func(t *testing.T) {
		up := newTestUploader(t, func(c *Config) {
			c.AllowedExtensions = []string{".txt"}
			// image/png itself is not listed; image/* prefix match must accept it.
			c.AllowedMimeTypes = []string{"image/jpeg"}
		})
		v := NewValidator(up.GetConfig())

		if err := v.Validate(mustFileHeader(t, "notes.txt", pngPayload)); err != nil {
			t.Fatalf("expected prefix match to accept image/png, got %v", err)
		}
	})

	t.Run("open failure", func(t *testing.T) {
		up := newTestUploader(t, textUploadConfig)
		v := NewValidator(up.GetConfig())

		// Zero-value FileHeader: no in-memory content and no backing tmpfile,
		// so FileHeader.Open fails and the file_open branch fires.
		ghost := &multipart.FileHeader{Filename: "ok.txt", Size: 2}
		err := v.Validate(ghost)
		if err == nil {
			t.Fatal("expected file_open validation error")
		}
		verr := err.(*ValidationError)
		if verr.Field != "file_open" {
			t.Errorf("field = %q, want file_open", verr.Field)
		}
	})

	// Note: the read-failure branch (file_read) requires FileHeader.Open to
	// succeed while Read returns a non-EOF error. Both content and tmpfile are
	// unexported fields of mime/multipart.FileHeader, so this state cannot be
	// constructed from an external package; every reachable header either
	// reads cleanly or fails at Open time.

	t.Run("empty allowlists skip checks", func(t *testing.T) {
		up := newTestUploader(t, func(c *Config) {
			c.AllowedExtensions = nil
			c.AllowedMimeTypes = nil
		})
		v := NewValidator(up.GetConfig())

		if err := v.Validate(mustFileHeader(t, "anything.bin", []byte{0x00, 0x01})); err != nil {
			t.Fatalf("expected empty allowlists to skip checks, got %v", err)
		}
	})
}

func TestValidatorValidateMultiple(t *testing.T) {
	up := newTestUploader(t, func(c *Config) {
		textUploadConfig(c)
		c.MaxFileSize = 100
	})
	v := NewValidator(up.GetConfig())

	good := mustFileHeader(t, "g.txt", []byte("hi"))
	bad := mustFileHeader(t, "b.exe", []byte("hi"))

	if errs := v.ValidateMultiple([]*multipart.FileHeader{good}); errs != nil {
		t.Fatalf("expected nil errors for all-valid batch, got %v", errs)
	}

	errs := v.ValidateMultiple([]*multipart.FileHeader{bad, good})
	if len(errs) != 1 {
		t.Fatalf("got %d errors, want 1", len(errs))
	}
	if !strings.Contains(errs[0].Error(), "file_extension") {
		t.Errorf("first error = %v, want extension failure", errs[0])
	}
}

func TestValidationErrorFormat(t *testing.T) {
	err := &ValidationError{Field: "file_size", Message: "too big"}
	if got := err.Error(); got != "file_size: too big" {
		t.Errorf("Error() = %q, want %q", got, "file_size: too big")
	}
}

func TestValidatorIsAllowedExtension(t *testing.T) {
	v := NewValidator(&Config{AllowedExtensions: []string{".txt", ".PDF"}})

	if !v.IsAllowedExtension("a.txt") {
		t.Error("expected .txt to be allowed")
	}
	if !v.IsAllowedExtension("a.TXT") {
		t.Error("extension matching should be case-insensitive")
	}
	if !v.IsAllowedExtension("a.pdf") {
		t.Error("allowlist entries should be matched case-insensitively")
	}
	if v.IsAllowedExtension("a.exe") {
		t.Error("expected .exe to be rejected")
	}
	if v.IsAllowedExtension("noext") {
		t.Error("expected extension-less name to be rejected")
	}

	empty := NewValidator(&Config{})
	if !empty.IsAllowedExtension("whatever.anything") {
		t.Error("empty allowlist should allow everything")
	}
}

func TestValidatorIsAllowedMimeType(t *testing.T) {
	v := NewValidator(&Config{AllowedMimeTypes: []string{"image/jpeg", "text/plain"}})

	if !v.IsAllowedMimeType("text/plain") {
		t.Error("expected exact match to be allowed")
	}
	if !v.IsAllowedMimeType("image/webp") {
		t.Error("expected same-type prefix match (image/*) to be allowed")
	}
	if v.IsAllowedMimeType("application/zip") {
		t.Error("expected unrelated MIME type to be rejected")
	}

	empty := NewValidator(&Config{})
	if !empty.IsAllowedMimeType("application/zip") {
		t.Error("empty allowlist should allow everything")
	}
}

func TestValidatorGetMaxFileSize(t *testing.T) {
	v := NewValidator(&Config{MaxFileSize: 2048})
	if got := v.GetMaxFileSize(); got != 2048 {
		t.Errorf("GetMaxFileSize = %d, want 2048", got)
	}
}
