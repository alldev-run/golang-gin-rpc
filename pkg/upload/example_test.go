package upload

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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

func ExampleGinMiddleware() {
	// This example shows how to use the Gin middleware
	// Note: This is a conceptual example, not runnable in tests
	/*
		config := DefaultConfig()
		middleware := NewGinMiddleware(config)

		r := gin.Default()
		api := r.Group("/api")
		middleware.RegisterRoutes(api)

		r.Run(":8080")
	*/
	fmt.Println("Gin middleware registered at /api/upload, /api/upload/single, /api/delete")
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
	config := DefaultConfig()
	config.UploadDir = "./test_uploads"
	config.AutoCreateDir = true
	defer os.RemoveAll(config.UploadDir)

	handler := NewHandler(config)

	// Create a multipart form
	body := &strings.Builder{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("files", "test.txt")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}

	io.WriteString(part, "test content")
	writer.Close()

	req := httptest.NewRequest("POST", "/upload", strings.NewReader(body.String()))
	req.Header.Set("Content-Type", writer.FormDataContentType())

	w := httptest.NewRecorder()
	handler.UploadHandler(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusMultiStatus {
		t.Fatalf("Expected status 200 or 207, got %d", w.Code)
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
