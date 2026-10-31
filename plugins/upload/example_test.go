package upload

import (
	"fmt"
	"os"
	"testing"
)

func ExampleUploader_Upload() {
	config := DefaultConfig()
	config.UploadDir = "./test_uploads"
	config.AutoCreateDir = true

	uploader := NewUploader(config)

	// Create a test file
	content := []byte("test file content")
	result := uploader.UploadFromBytes("test.txt", content)

	if result.Success {
		fmt.Printf("File uploaded successfully: %s\n", result.SavedFilename)
		// Clean up
		os.Remove(result.FilePath)
		os.Remove(config.UploadDir)
	}
}

func ExampleConfig() {
	config := DefaultConfig()
	fmt.Printf("Default upload directory: %s\n", config.UploadDir)
	fmt.Printf("Max file size: %d bytes\n", config.MaxFileSize)
	fmt.Printf("Naming strategy: %s\n", config.NamingStrategy)
}

func TestUploadFromBytes(t *testing.T) {
	config := DefaultConfig()
	config.UploadDir = "./test_uploads"
	config.AutoCreateDir = true
	config.AllowedExtensions = []string{".txt", ".jpg", ".png", ".pdf"}
	config.AllowedMimeTypes = []string{"text/plain", "image/jpeg", "image/png", "application/pdf"}
	defer os.RemoveAll(config.UploadDir)

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
	config.UploadDir = "./test_uploads"
	config.AutoCreateDir = true
	defer os.RemoveAll(config.UploadDir)

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
	config.UploadDir = "./test_uploads"
	config.AutoCreateDir = true
	config.AllowedExtensions = []string{".txt", ".jpg", ".png", ".pdf"}
	config.AllowedMimeTypes = []string{"text/plain", "image/jpeg", "image/png", "application/pdf"}
	defer os.RemoveAll(config.UploadDir)

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

			os.Remove(result.FilePath)
		})
	}
}

func TestHandlerUpload(t *testing.T) {
	// HTTP handlers are now in API Gateway layer, not in core upload package
	t.Skip("HTTP handlers are in API Gateway layer")
}

func TestAuthUsesConfiguredCredentials(t *testing.T) {
	// Authentication is handled in API Gateway layer, not in core upload package
	t.Skip("Authentication is in API Gateway layer")
}

func TestIssueTokenRequiresAuthAndDownloadWithToken(t *testing.T) {
	// Token handling is in API Gateway layer, not in core upload package
	t.Skip("Token handling is in API Gateway layer")
}

func TestPublicGenerateAndVerifyDownloadToken(t *testing.T) {
	// Token handling is in API Gateway layer, not in core upload package
	t.Skip("Token handling is in API Gateway layer")
}

func TestUploadStreamHandler(t *testing.T) {
	// HTTP handlers are in API Gateway layer, not in core upload package
	t.Skip("HTTP handlers are in API Gateway layer")
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
	config.UploadDir = "./test_uploads"
	config.AutoCreateDir = true
	config.NamingStrategy = "original"
	config.AllowedExtensions = []string{".txt"}
	config.AllowedMimeTypes = []string{"text/plain"}
	defer os.RemoveAll(config.UploadDir)

	uploader := NewUploader(config)

	result := uploader.UploadFromBytes("../evil.txt", []byte("blocked"))
	if result.Success {
		t.Fatal("expected upload to fail for path traversal filename")
	}

	if err := uploader.Delete("../evil.txt"); err == nil {
		t.Fatal("expected delete to fail for path traversal filename")
	}
}
