package upload

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// mustFileHeader builds a real *multipart.FileHeader from a filename and
// content by round-tripping them through multipart.Writer / multipart.Reader,
// exactly the way an inbound HTTP multipart request would be parsed.
func mustFileHeader(tb testing.TB, filename string, content []byte) *multipart.FileHeader {
	tb.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		tb.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		tb.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		tb.Fatalf("close multipart writer: %v", err)
	}

	form, err := multipart.NewReader(&body, writer.Boundary()).ReadForm(1 << 20)
	if err != nil {
		tb.Fatalf("ReadForm: %v", err)
	}
	tb.Cleanup(func() { _ = form.RemoveAll() })

	headers := form.File["file"]
	if len(headers) != 1 {
		tb.Fatalf("expected exactly 1 file part, got %d", len(headers))
	}
	return headers[0]
}

// newTestUploader returns an uploader rooted at a per-test temp directory.
func newTestUploader(tb testing.TB, mutate func(*Config)) *Uploader {
	tb.Helper()
	cfg := DefaultConfig()
	cfg.UploadDir = tb.TempDir()
	if mutate != nil {
		mutate(cfg)
	}
	return NewUploader(cfg)
}

func textUploadConfig(c *Config) {
	c.AllowedExtensions = []string{".txt"}
	// The uploader sniffs MIME from a 512-byte buffer; short files are read
	// into a zero-padded buffer and therefore detect as octet-stream.
	c.AllowedMimeTypes = []string{"text/plain", "application/octet-stream"}
}

func TestNewUploaderDefaults(t *testing.T) {
	up := NewUploader(nil)
	cfg := up.GetConfig()

	if cfg == nil {
		t.Fatal("expected default config, got nil")
	}
	if cfg.UploadDir != "./uploads" {
		t.Errorf("UploadDir = %q, want %q", cfg.UploadDir, "./uploads")
	}
	if cfg.MaxFileSize != 10*1024*1024 {
		t.Errorf("MaxFileSize = %d, want %d", cfg.MaxFileSize, 10*1024*1024)
	}
	if cfg.NamingStrategy != "uuid" {
		t.Errorf("NamingStrategy = %q, want %q", cfg.NamingStrategy, "uuid")
	}
	if !cfg.PreserveExtension {
		t.Error("PreserveExtension = false, want true")
	}
	if !cfg.AutoCreateDir {
		t.Error("AutoCreateDir = false, want true")
	}
	if cfg.EnableOverwrite {
		t.Error("EnableOverwrite = true, want false")
	}
	if len(cfg.AllowedExtensions) == 0 {
		t.Error("expected default AllowedExtensions to be populated")
	}
	if len(cfg.AllowedMimeTypes) == 0 {
		t.Error("expected default AllowedMimeTypes to be populated")
	}
}

func TestUploadMultipartHappyPath(t *testing.T) {
	up := newTestUploader(t, textUploadConfig)

	// Content longer than the 512-byte sniff buffer, so the buffer is fully
	// filled with text and the detected MIME type is a real text/plain value.
	content := bytes.Repeat([]byte("multipart content line\n"), 30)
	result := up.Upload(mustFileHeader(t, "hello.txt", content))

	if !result.Success {
		t.Fatalf("Upload failed: %v", result.Error)
	}
	if result.Error != nil {
		t.Fatalf("unexpected error on success result: %v", result.Error)
	}
	if result.OriginalFilename != "hello.txt" {
		t.Errorf("OriginalFilename = %q, want %q", result.OriginalFilename, "hello.txt")
	}
	if result.FileSize != int64(len(content)) {
		t.Errorf("FileSize = %d, want %d", result.FileSize, len(content))
	}
	if !strings.HasSuffix(result.SavedFilename, ".txt") {
		t.Errorf("SavedFilename = %q, want preserved .txt extension", result.SavedFilename)
	}
	if _, err := uuid.Parse(strings.TrimSuffix(result.SavedFilename, ".txt")); err != nil {
		t.Errorf("SavedFilename base %q is not a UUID: %v", result.SavedFilename, err)
	}
	if result.MimeType != "text/plain; charset=utf-8" {
		t.Errorf("MimeType = %q, want %q", result.MimeType, "text/plain; charset=utf-8")
	}

	wantPath := filepath.Join(up.GetConfig().UploadDir, result.SavedFilename)
	if result.FilePath != wantPath {
		t.Errorf("FilePath = %q, want %q", result.FilePath, wantPath)
	}
	data, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if !bytes.Equal(data, content) {
		t.Errorf("saved content = %q, want %q", data, content)
	}
}

func TestUploadMultipartValidationFailure(t *testing.T) {
	up := newTestUploader(t, func(c *Config) {
		textUploadConfig(c)
		c.MaxFileSize = 10
	})

	result := up.Upload(mustFileHeader(t, "big.txt", bytes.Repeat([]byte("x"), 32)))

	if result.Success {
		t.Fatal("expected upload to fail size validation")
	}
	var verr *ValidationError
	if !errors.As(result.Error, &verr) {
		t.Fatalf("expected *ValidationError, got %T: %v", result.Error, result.Error)
	}
	if verr.Field != "file_size" {
		t.Errorf("error field = %q, want %q", verr.Field, "file_size")
	}
	if !strings.Contains(verr.Message, "exceeds") {
		t.Errorf("error message = %q, want it to mention exceeding the limit", verr.Message)
	}

	entries, err := os.ReadDir(up.GetConfig().UploadDir)
	if err != nil {
		t.Fatalf("read upload dir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected no files written, found %d", len(entries))
	}
}

func TestUploadMultipartExistsWithoutOverwrite(t *testing.T) {
	up := newTestUploader(t, func(c *Config) {
		textUploadConfig(c)
		c.NamingStrategy = "original"
	})

	first := up.Upload(mustFileHeader(t, "dup.txt", []byte("first")))
	if !first.Success {
		t.Fatalf("first upload failed: %v", first.Error)
	}

	second := up.Upload(mustFileHeader(t, "dup.txt", []byte("second")))
	if second.Success {
		t.Fatal("second upload should fail when overwrite is disabled")
	}
	if second.Error == nil || !strings.Contains(second.Error.Error(), "file already exists") {
		t.Fatalf("expected 'file already exists' error, got %v", second.Error)
	}

	data, err := os.ReadFile(first.FilePath)
	if err != nil {
		t.Fatalf("read original file: %v", err)
	}
	if string(data) != "first" {
		t.Errorf("original content = %q, want %q (must not be overwritten)", data, "first")
	}
}

func TestUploadMultipartOverwriteTruncates(t *testing.T) {
	up := newTestUploader(t, func(c *Config) {
		textUploadConfig(c)
		c.NamingStrategy = "original"
		c.EnableOverwrite = true
	})

	if r := up.Upload(mustFileHeader(t, "dup.txt", []byte("first"))); !r.Success {
		t.Fatalf("first upload failed: %v", r.Error)
	}
	second := up.Upload(mustFileHeader(t, "dup.txt", []byte("second-longer-content")))
	if !second.Success {
		t.Fatalf("second upload with EnableOverwrite failed: %v", second.Error)
	}

	data, err := os.ReadFile(second.FilePath)
	if err != nil {
		t.Fatalf("read overwritten file: %v", err)
	}
	if string(data) != "second-longer-content" {
		t.Errorf("content after overwrite = %q, want %q", data, "second-longer-content")
	}
}

func TestUploadMultipartDestinationCreateFails(t *testing.T) {
	up := newTestUploader(t, func(c *Config) {
		textUploadConfig(c)
		c.AutoCreateDir = false
		c.UploadDir = filepath.Join(t.TempDir(), "missing-dir")
	})

	result := up.Upload(mustFileHeader(t, "hello.txt", []byte("data")))
	if result.Success {
		t.Fatal("expected upload to fail when destination dir cannot be used")
	}
	if result.Error == nil || !strings.Contains(result.Error.Error(), "failed to create destination file") {
		t.Fatalf("expected destination create error, got %v", result.Error)
	}
}

func TestUploadMultipartMkdirFailsWhenTargetIsFile(t *testing.T) {
	base := t.TempDir()
	blocker := filepath.Join(base, "blocker")
	if err := os.WriteFile(blocker, []byte("occupied"), 0o600); err != nil {
		t.Fatalf("write blocker file: %v", err)
	}

	up := newTestUploader(t, func(c *Config) {
		textUploadConfig(c)
		c.UploadDir = blocker
	})

	result := up.Upload(mustFileHeader(t, "hello.txt", []byte("data")))
	if result.Success {
		t.Fatal("expected upload to fail when upload dir is an existing file")
	}
	if result.Error == nil || !strings.Contains(result.Error.Error(), "failed to create upload directory") {
		t.Fatalf("expected mkdir error, got %v", result.Error)
	}
}

func TestMultipartFilenameSanitized(t *testing.T) {
	// RFC 7578 section 4.2: the multipart parser (Part.FileName) strips
	// directory information via filepath.Base, so a traversal name never
	// reaches the uploader through a real multipart payload.
	fh := mustFileHeader(t, "../evil.txt", []byte("nope"))
	if fh.Filename != "evil.txt" {
		t.Errorf("sanitized filename = %q, want %q", fh.Filename, "evil.txt")
	}

	// The path-traversal guard itself is covered by resolveSafeFilePath via
	// the string-based APIs (UploadFromBytes, Delete, Exists, ServeFile).
	up := newTestUploader(t, func(c *Config) {
		textUploadConfig(c)
		c.NamingStrategy = "original"
	})
	if r := up.UploadFromBytes("../evil.txt", []byte("nope")); r.Success {
		t.Error("expected UploadFromBytes to reject traversal filename")
	}
}

func TestUploadMultiple(t *testing.T) {
	up := newTestUploader(t, func(c *Config) {
		textUploadConfig(c)
		c.NamingStrategy = "original"
	})

	allGood := up.UploadMultiple([]*multipart.FileHeader{
		mustFileHeader(t, "a.txt", []byte("aaa")),
		mustFileHeader(t, "b.txt", []byte("bb")),
	})
	if len(allGood) != 2 {
		t.Fatalf("got %d results, want 2", len(allGood))
	}
	for i, r := range allGood {
		if r == nil {
			t.Fatalf("result %d is nil", i)
		}
		if !r.Success {
			t.Errorf("result %d failed: %v", i, r.Error)
		}
	}
	if !up.Exists("a.txt") || !up.Exists("b.txt") {
		t.Error("expected both uploaded files to exist")
	}

	mixed := up.UploadMultiple([]*multipart.FileHeader{
		mustFileHeader(t, "c.txt", []byte("ccc")),
		mustFileHeader(t, "evil.exe", []byte("nope")),
	})
	if len(mixed) != 2 {
		t.Fatalf("got %d results, want 2", len(mixed))
	}
	if !mixed[0].Success {
		t.Errorf("first upload should succeed, got error: %v", mixed[0].Error)
	}
	if mixed[1].Success {
		t.Error("second upload should fail extension validation")
	}
}

func TestUploadFromBytesMimeContentMismatch(t *testing.T) {
	up := newTestUploader(t, textUploadConfig)

	// PNG magic bytes smuggled behind a .txt name: extension passes, content sniffing must reject.
	pngPayload := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}
	result := up.UploadFromBytes("payload.txt", pngPayload)

	if result.Success {
		t.Fatal("expected MIME content check to reject binary payload in .txt")
	}
	var verr *ValidationError
	if !errors.As(result.Error, &verr) {
		t.Fatalf("expected *ValidationError, got %T: %v", result.Error, result.Error)
	}
	if verr.Field != "file_mime_type" {
		t.Errorf("error field = %q, want %q", verr.Field, "file_mime_type")
	}

	entries, err := os.ReadDir(up.GetConfig().UploadDir)
	if err != nil {
		t.Fatalf("read upload dir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected no files written, found %d", len(entries))
	}
}

func TestUploadFromBytesExistsWithoutOverwrite(t *testing.T) {
	up := newTestUploader(t, func(c *Config) {
		textUploadConfig(c)
		c.NamingStrategy = "original"
	})

	if r := up.UploadFromBytes("same.txt", []byte("v1")); !r.Success {
		t.Fatalf("first upload failed: %v", r.Error)
	}
	second := up.UploadFromBytes("same.txt", []byte("v2"))
	if second.Success {
		t.Fatal("second upload should fail when overwrite is disabled")
	}
	if second.Error == nil || !strings.Contains(second.Error.Error(), "file already exists") {
		t.Fatalf("expected 'file already exists' error, got %v", second.Error)
	}
}

func TestUploadFromBytesOverwriteAllowed(t *testing.T) {
	up := newTestUploader(t, func(c *Config) {
		textUploadConfig(c)
		c.NamingStrategy = "original"
		c.EnableOverwrite = true
	})

	if r := up.UploadFromBytes("same.txt", []byte("v1")); !r.Success {
		t.Fatalf("first upload failed: %v", r.Error)
	}
	if r := up.UploadFromBytes("same.txt", []byte("v2")); !r.Success {
		t.Fatalf("second upload with EnableOverwrite failed: %v", r.Error)
	}

	data, err := os.ReadFile(filepath.Join(up.GetConfig().UploadDir, "same.txt"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != "v2" {
		t.Errorf("content = %q, want %q", data, "v2")
	}
}

func TestUploadFromBytesMkdirFailsWhenTargetIsFile(t *testing.T) {
	base := t.TempDir()
	blocker := filepath.Join(base, "blocker")
	if err := os.WriteFile(blocker, []byte("occupied"), 0o600); err != nil {
		t.Fatalf("write blocker file: %v", err)
	}

	up := newTestUploader(t, func(c *Config) {
		textUploadConfig(c)
		c.UploadDir = blocker // AutoCreateDir is true by default
	})

	result := up.UploadFromBytes("hello.txt", []byte("data"))
	if result.Success {
		t.Fatal("expected upload to fail when upload dir is an existing file")
	}
	if result.Error == nil || !strings.Contains(result.Error.Error(), "failed to create upload directory") {
		t.Fatalf("expected mkdir error, got %v", result.Error)
	}
}

func TestUploadFromBytesWriteFailsWhenDirIsFile(t *testing.T) {
	base := t.TempDir()
	blocker := filepath.Join(base, "blocker")
	if err := os.WriteFile(blocker, []byte("occupied"), 0o600); err != nil {
		t.Fatalf("write blocker file: %v", err)
	}

	up := newTestUploader(t, func(c *Config) {
		textUploadConfig(c)
		c.AutoCreateDir = false
		c.UploadDir = blocker
	})

	result := up.UploadFromBytes("hello.txt", []byte("data"))
	if result.Success {
		t.Fatal("expected write to fail when parent path is a regular file")
	}
	if result.Error == nil || !strings.Contains(result.Error.Error(), "failed to write file") {
		t.Fatalf("expected write error, got %v", result.Error)
	}
}

func TestServeFilePlainText(t *testing.T) {
	up := newTestUploader(t, func(c *Config) {
		textUploadConfig(c)
		c.NamingStrategy = "original"
	})

	content := "hello download"
	if r := up.UploadFromBytes("notes.txt", []byte(content)); !r.Success {
		t.Fatalf("seed upload failed: %v", r.Error)
	}

	w := httptest.NewRecorder()
	if err := up.ServeFile("notes.txt", w); err != nil {
		t.Fatalf("ServeFile: %v", err)
	}
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if got := w.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q, want %q", got, "text/plain; charset=utf-8")
	}
	if got := w.Header().Get("Content-Disposition"); got != `attachment; filename="notes.txt"` {
		t.Errorf("Content-Disposition = %q, want %q", got, `attachment; filename="notes.txt"`)
	}
	if w.Body.String() != content {
		t.Errorf("body = %q, want %q", w.Body.String(), content)
	}
}

func TestServeFileOctetStreamFallsBackToExtension(t *testing.T) {
	up := newTestUploader(t, nil)

	// Binary garbage: content sniffing yields application/octet-stream, so the
	// .pdf extension must be used to infer the Content-Type instead.
	binary := []byte{0x00, 0x01, 0x02, 0x03, 0xde, 0xad, 0xbe, 0xef}
	target := filepath.Join(up.GetConfig().UploadDir, "doc.pdf")
	if err := os.WriteFile(target, binary, 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	w := httptest.NewRecorder()
	if err := up.ServeFile("doc.pdf", w); err != nil {
		t.Fatalf("ServeFile: %v", err)
	}
	if got := w.Header().Get("Content-Type"); got != "application/pdf" {
		t.Errorf("Content-Type = %q, want extension fallback %q", got, "application/pdf")
	}
	if w.Body.Bytes() == nil || !bytes.Equal(w.Body.Bytes(), binary) {
		t.Errorf("body = %v, want %v", w.Body.Bytes(), binary)
	}
}

func TestServeFileMissing(t *testing.T) {
	up := newTestUploader(t, nil)

	err := up.ServeFile("ghost.txt", httptest.NewRecorder())
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "file not found") {
		t.Errorf("error = %v, want it to mention 'file not found'", err)
	}
}

func TestServeFileRejectsTraversal(t *testing.T) {
	up := newTestUploader(t, nil)

	err := up.ServeFile("../evil.txt", httptest.NewRecorder())
	if err == nil {
		t.Fatal("expected traversal filename to be rejected")
	}
	if !strings.Contains(err.Error(), "invalid filename") {
		t.Errorf("error = %v, want invalid filename error", err)
	}
}

func TestServeFileDirectoryTargetFails(t *testing.T) {
	up := newTestUploader(t, nil)

	if err := os.MkdirAll(filepath.Join(up.GetConfig().UploadDir, "subdir"), 0o755); err != nil {
		t.Fatalf("create subdir: %v", err)
	}

	w := httptest.NewRecorder()
	if err := up.ServeFile("subdir", w); err == nil {
		t.Fatal("expected error when serving a directory")
	}
	if w.Body.Len() != 0 {
		t.Errorf("expected empty body on failure, got %d bytes", w.Body.Len())
	}
}

func TestServeFileNestedPath(t *testing.T) {
	up := newTestUploader(t, nil)

	sub := filepath.Join(up.GetConfig().UploadDir, "nested")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("create nested dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sub, "inner.txt"), []byte("inner"), 0o600); err != nil {
		t.Fatalf("seed nested file: %v", err)
	}

	w := httptest.NewRecorder()
	if err := up.ServeFile("nested/inner.txt", w); err != nil {
		t.Fatalf("ServeFile nested path: %v", err)
	}
	if w.Body.String() != "inner" {
		t.Errorf("body = %q, want %q", w.Body.String(), "inner")
	}
	if got := w.Header().Get("Content-Disposition"); got != `attachment; filename="inner.txt"` {
		t.Errorf("Content-Disposition = %q, want nested base name", got)
	}
}

func TestDelete(t *testing.T) {
	up := newTestUploader(t, func(c *Config) {
		textUploadConfig(c)
		c.NamingStrategy = "original"
	})

	if r := up.UploadFromBytes("gone.txt", []byte("delete me")); !r.Success {
		t.Fatalf("seed upload failed: %v", r.Error)
	}
	if err := up.Delete("gone.txt"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if up.Exists("gone.txt") {
		t.Error("file still exists after Delete")
	}

	if err := up.Delete("ghost.txt"); err == nil {
		t.Error("expected error deleting a missing file")
	}
	if err := up.Delete("../evil.txt"); err == nil {
		t.Error("expected error deleting a traversal filename")
	}
}

func TestExists(t *testing.T) {
	up := newTestUploader(t, func(c *Config) {
		textUploadConfig(c)
		c.NamingStrategy = "original"
	})

	if up.Exists("missing.txt") {
		t.Error("Exists reported true for missing file")
	}
	if r := up.UploadFromBytes("here.txt", []byte("x")); !r.Success {
		t.Fatalf("seed upload failed: %v", r.Error)
	}
	if !up.Exists("here.txt") {
		t.Error("Exists reported false for uploaded file")
	}
	if up.Exists(`..\outside.txt`) {
		t.Error("Exists reported true for invalid traversal filename")
	}
}

func TestListFiles(t *testing.T) {
	up := newTestUploader(t, func(c *Config) {
		textUploadConfig(c)
		c.NamingStrategy = "original"
	})

	if r := up.UploadFromBytes("a.txt", []byte("12345")); !r.Success {
		t.Fatalf("seed upload failed: %v", r.Error)
	}
	if r := up.UploadFromBytes("b.txt", []byte("123")); !r.Success {
		t.Fatalf("seed upload failed: %v", r.Error)
	}
	if err := os.MkdirAll(filepath.Join(up.GetConfig().UploadDir, "subdir"), 0o755); err != nil {
		t.Fatalf("create subdir: %v", err)
	}

	files, err := up.ListFiles()
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2 (directories must be skipped)", len(files))
	}

	byName := make(map[string]*FileInfo, len(files))
	for _, f := range files {
		byName[f.Filename] = f
	}
	a, ok := byName["a.txt"]
	if !ok {
		t.Fatalf("a.txt missing from listing: %v", byName)
	}
	if a.FileSize != 5 {
		t.Errorf("a.txt size = %d, want 5", a.FileSize)
	}
	if a.IsDirectory {
		t.Error("a.txt flagged as directory")
	}
	if a.FilePath != "a.txt" {
		t.Errorf("a.txt FilePath = %q, want %q", a.FilePath, "a.txt")
	}
	if b, ok := byName["b.txt"]; !ok || b.FileSize != 3 {
		t.Errorf("b.txt missing or wrong size: %v", b)
	}

	missing := newTestUploader(t, func(c *Config) {
		c.UploadDir = filepath.Join(t.TempDir(), "does-not-exist")
	})
	if _, err := missing.ListFiles(); err == nil {
		t.Fatal("expected error listing a missing upload directory")
	} else if !strings.Contains(err.Error(), "failed to read upload directory") {
		t.Errorf("error = %v, want read-directory failure", err)
	}
}

func TestGetAndUpdateConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.UploadDir = t.TempDir()
	textUploadConfig(cfg)
	up := NewUploader(cfg)

	if up.GetConfig() != cfg {
		t.Fatal("GetConfig should return the active config instance")
	}

	newCfg := DefaultConfig()
	newCfg.UploadDir = cfg.UploadDir
	newCfg.NamingStrategy = "original"
	newCfg.AllowedExtensions = []string{".md"}
	newCfg.AllowedMimeTypes = []string{"text/plain"}
	up.UpdateConfig(newCfg)

	if up.GetConfig() != newCfg {
		t.Fatal("UpdateConfig did not swap the active config")
	}

	// New namer is applied: original strategy keeps the filename as-is.
	r := up.UploadFromBytes("doc.md", []byte("updated"))
	if !r.Success {
		t.Fatalf("upload after UpdateConfig failed: %v", r.Error)
	}
	if r.SavedFilename != "doc.md" {
		t.Errorf("SavedFilename = %q, want original name %q", r.SavedFilename, "doc.md")
	}

	// New validator is applied: .txt is no longer allowed.
	if r := up.UploadFromBytes("legacy.txt", []byte("x")); r.Success {
		t.Error("upload should fail under the updated extension allowlist")
	}
}

func TestResolveSafeFilePath(t *testing.T) {
	up := newTestUploader(t, nil)
	baseAbs, err := filepath.Abs(up.GetConfig().UploadDir)
	if err != nil {
		t.Fatalf("abs upload dir: %v", err)
	}

	cases := []struct {
		name       string
		in         string
		wantErr    bool
		wantSuffix string // path fragment expected under baseAbs when wantErr is false
	}{
		{"empty filename", "", true, ""},
		{"whitespace only", "   ", true, ""},
		{"single dot", ".", true, ""},
		{"double dot", "..", true, ""},
		{"parent traversal", "../secret.txt", true, ""},
		{"nested traversal", "a/../../secret.txt", true, ""},
		{"backslash separator", `sub\secret.txt`, true, ""},
		{"absolute windows path", `C:\Windows\evil.txt`, true, ""},
		{"plain filename", "notes.txt", false, "notes.txt"},
		{"nested path allowed", "2024/06/notes.txt", false, filepath.Join("2024", "06", "notes.txt")},
		{"surrounding spaces trimmed", "  notes.txt  ", false, "notes.txt"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := up.resolveSafeFilePath(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("resolveSafeFilePath(%q) = %q, want error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveSafeFilePath(%q) unexpected error: %v", tc.in, err)
			}
			want := filepath.Join(baseAbs, tc.wantSuffix)
			if got != want {
				t.Errorf("resolveSafeFilePath(%q) = %q, want %q", tc.in, got, want)
			}
		})
	}
}
