package upload

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"os"
	"testing"
)

func ExampleUploader_Upload() {
	dir, err := os.MkdirTemp("", "upload-example-*")
	if err != nil {
		return
	}
	defer os.RemoveAll(dir)

	config := DefaultConfig()
	config.UploadDir = dir
	config.AllowedExtensions = []string{".txt"}
	// Short files sniff as application/octet-stream because the uploader
	// detects MIME from a zero-padded 512-byte buffer.
	config.AllowedMimeTypes = []string{"text/plain", "application/octet-stream"}
	config.NamingStrategy = "original"

	uploader := NewUploader(config)

	// Build a multipart form exactly like an inbound HTTP request would carry.
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "notes.txt")
	if err != nil {
		return
	}
	if _, err := part.Write([]byte("hello from multipart")); err != nil {
		return
	}
	if err := writer.Close(); err != nil {
		return
	}
	form, err := multipart.NewReader(&body, writer.Boundary()).ReadForm(1 << 20)
	if err != nil {
		return
	}
	defer func() { _ = form.RemoveAll() }()

	result := uploader.Upload(form.File["file"][0])
	if result.Success {
		fmt.Printf("Saved %d bytes as %s\n", result.FileSize, result.SavedFilename)
	}
	// Output: Saved 20 bytes as notes.txt
}

func ExampleConfig() {
	config := DefaultConfig()
	fmt.Printf("Default upload directory: %s\n", config.UploadDir)
	fmt.Printf("Max file size: %d bytes\n", config.MaxFileSize)
	fmt.Printf("Naming strategy: %s\n", config.NamingStrategy)
	// Output:
	// Default upload directory: ./uploads
	// Max file size: 10485760 bytes
	// Naming strategy: uuid
}

func TestUploadFromBytes(t *testing.T) {
	config := DefaultConfig()
	config.UploadDir = t.TempDir()
	config.AutoCreateDir = true
	config.AllowedExtensions = []string{".txt", ".jpg", ".png", ".pdf"}
	config.AllowedMimeTypes = []string{"text/plain", "image/jpeg", "image/png", "application/pdf"}

	uploader := NewUploader(config)
	content := []byte("test file content")

	result := uploader.UploadFromBytes("test.txt", content)

	if !result.Success {
		t.Fatalf("Upload failed: %v", result.Error)
	}

	if result.SavedFilename == "" {
		t.Fatal("Saved filename is empty")
	}

	if result.FilePath == "" {
		t.Fatal("File path is empty")
	}

	// Verify file exists
	if _, err := os.Stat(result.FilePath); os.IsNotExist(err) {
		t.Fatalf("File was not created: %s", result.FilePath)
	}
}

func TestUploadValidation(t *testing.T) {
	config := DefaultConfig()
	config.MaxFileSize = 100 // 100 bytes
	config.AllowedExtensions = []string{".txt"}
	config.UploadDir = t.TempDir()
	config.AutoCreateDir = true

	uploader := NewUploader(config)

	// Test file size validation
	largeContent := make([]byte, 200)
	result := uploader.UploadFromBytes("large.txt", largeContent)

	if result.Success {
		t.Fatal("Upload should have failed due to file size limit")
	}

	if result.Error == nil {
		t.Fatal("Expected validation error")
	}

	// Test file extension validation
	result = uploader.UploadFromBytes("test.jpg", []byte("test"))

	if result.Success {
		t.Fatal("Upload should have failed due to invalid extension")
	}
}

func TestNamingStrategies(t *testing.T) {
	config := DefaultConfig()
	config.UploadDir = t.TempDir()
	config.AutoCreateDir = true
	config.AllowedExtensions = []string{".txt", ".jpg", ".png", ".pdf"}
	config.AllowedMimeTypes = []string{"text/plain", "image/jpeg", "image/png", "application/pdf"}

	testCases := []struct {
		strategy string
	}{
		{"uuid"},
		{"timestamp"},
		{"original"},
	}

	for _, tc := range testCases {
		t.Run(tc.strategy, func(t *testing.T) {
			config.NamingStrategy = tc.strategy
			uploader := NewUploader(config)
			result := uploader.UploadFromBytes("test.txt", []byte("test"))

			if !result.Success {
				t.Fatalf("Upload failed: %v", result.Error)
			}

			if result.SavedFilename == "" {
				t.Fatal("Saved filename is empty")
			}
		})
	}
}

func TestValidator(t *testing.T) {
	config := DefaultConfig()
	config.MaxFileSize = 1024
	config.AllowedExtensions = []string{".txt", ".jpg"}
	config.AllowedMimeTypes = []string{"text/plain", "image/jpeg"}

	validator := NewValidator(config)

	// Test extension check
	if !validator.IsAllowedExtension("test.txt") {
		t.Error("Expected .txt to be allowed")
	}

	if validator.IsAllowedExtension("test.pdf") {
		t.Error("Expected .pdf to be not allowed")
	}

	// Test MIME type check
	if !validator.IsAllowedMimeType("text/plain") {
		t.Error("Expected text/plain to be allowed")
	}

	// Test max file size
	if validator.GetMaxFileSize() != 1024 {
		t.Errorf("Expected max file size 1024, got %d", validator.GetMaxFileSize())
	}
}

func TestPathTraversalBlocked(t *testing.T) {
	config := DefaultConfig()
	config.UploadDir = t.TempDir()
	config.AutoCreateDir = true
	config.NamingStrategy = "original"
	config.AllowedExtensions = []string{".txt"}
	config.AllowedMimeTypes = []string{"text/plain"}

	uploader := NewUploader(config)

	result := uploader.UploadFromBytes("../evil.txt", []byte("blocked"))
	if result.Success {
		t.Fatal("expected upload to fail for path traversal filename")
	}

	if err := uploader.Delete("../evil.txt"); err == nil {
		t.Fatal("expected delete to fail for path traversal filename")
	}
}
