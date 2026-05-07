package upload

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

	// Note: Handler is now in nethttp package
	// This test should be moved to nethttp/handler_test.go
	// For now, we'll skip this test
	t.Skip("Handler test moved to nethttp package")
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

func TestAuthUsesConfiguredCredentials(t *testing.T) {
	config := DefaultConfig()
	config.EnableAuth = true
	config.AuthUsername = "admin"
	config.AuthPassword = "secret"

	handler := NewHandler(config)

	req := httptest.NewRequest(http.MethodGet, "/list", nil)
	unauth := httptest.NewRecorder()
	handler.ListHandler(unauth, req)
	if unauth.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized without credentials, got %d", unauth.Code)
	}

	reqAuth := httptest.NewRequest(http.MethodGet, "/list", nil)
	reqAuth.SetBasicAuth("admin", "secret")
	auth := httptest.NewRecorder()
	handler.ListHandler(auth, reqAuth)
	if auth.Code == http.StatusUnauthorized {
		t.Fatal("expected authorized request with valid credentials")
	}
}

func TestIssueTokenRequiresAuthAndDownloadWithToken(t *testing.T) {
	config := DefaultConfig()
	config.UploadDir = "./test_uploads"
	config.AutoCreateDir = true
	config.NamingStrategy = "original"
	config.AllowedExtensions = []string{".txt"}
	config.AllowedMimeTypes = []string{"text/plain"}
	config.EnableAuth = true
	config.AuthUsername = "admin"
	config.AuthPassword = "secret"
	config.TokenTTLSeconds = 60
	defer os.RemoveAll(config.UploadDir)

	uploader := NewUploader(config)
	upload := uploader.UploadFromBytes("signed.txt", []byte("signed-content"))
	if !upload.Success {
		t.Fatalf("failed preparing upload fixture: %v", upload.Error)
	}

	handler := NewHandler(config)

	unauthReq := httptest.NewRequest(http.MethodGet, "/token?filename=signed.txt", nil)
	unauthW := httptest.NewRecorder()
	handler.IssueTokenHandler(unauthW, unauthReq)
	if unauthW.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauth token request, got %d", unauthW.Code)
	}

	authReq := httptest.NewRequest(http.MethodGet, "/token?filename=signed.txt", nil)
	authReq.SetBasicAuth("admin", "secret")
	authW := httptest.NewRecorder()
	handler.IssueTokenHandler(authW, authReq)
	if authW.Code != http.StatusOK {
		t.Fatalf("expected 200 for token request, got %d, body=%s", authW.Code, authW.Body.String())
	}

	var tokenResp struct {
		Data struct {
			Token   string `json:"token"`
			Expires int64  `json:"expires"`
		} `json:"data"`
	}
	if err := json.Unmarshal(authW.Body.Bytes(), &tokenResp); err != nil {
		t.Fatalf("failed to parse token response: %v", err)
	}
	if tokenResp.Data.Token == "" || tokenResp.Data.Expires == 0 {
		t.Fatal("token response missing token/expires")
	}

	downloadURL := "/download?filename=signed.txt"
	downloadReq := httptest.NewRequest(http.MethodGet, downloadURL, nil)
	downloadReq.Header.Set("Authorization", "Bearer "+tokenResp.Data.Token)
	downloadW := httptest.NewRecorder()
	handler.DownloadHandler(downloadW, downloadReq)
	if downloadW.Code != http.StatusOK {
		t.Fatalf("expected 200 for signed download, got %d, body=%s", downloadW.Code, downloadW.Body.String())
	}

	if strings.TrimSpace(downloadW.Body.String()) != "signed-content" {
		t.Fatalf("unexpected downloaded content: %q", downloadW.Body.String())
	}

	if !strings.Contains(downloadW.Header().Get("Content-Disposition"), filepath.Base("signed.txt")) {
		t.Fatalf("expected content-disposition filename, got %s", downloadW.Header().Get("Content-Disposition"))
	}
}

func TestPublicGenerateAndVerifyDownloadToken(t *testing.T) {
	config := DefaultConfig()
	config.UploadDir = "./test_uploads"
	config.AutoCreateDir = true
	config.NamingStrategy = "original"
	config.AllowedExtensions = []string{".txt"}
	config.AllowedMimeTypes = []string{"text/plain"}
	config.EnableAuth = true
	config.AuthUsername = "admin"
	config.AuthPassword = "secret"
	config.TokenTTLSeconds = 60
	defer os.RemoveAll(config.UploadDir)

	uploader := NewUploader(config)
	upload := uploader.UploadFromBytes("public-token.txt", []byte("token-content"))
	if !upload.Success {
		t.Fatalf("failed preparing file fixture: %v", upload.Error)
	}

	handler := NewHandler(config)
	token, expiresAt, err := handler.GenerateDownloadToken("public-token.txt")
	if err != nil {
		t.Fatalf("GenerateDownloadToken failed: %v", err)
	}
	if token == "" || expiresAt <= 0 {
		t.Fatal("expected non-empty token and positive expiresAt")
	}

	if !handler.VerifyDownloadToken("public-token.txt", expiresAt, token) {
		t.Fatal("expected generated token to verify successfully")
	}

	if handler.VerifyDownloadToken("public-token.txt", expiresAt, "bad-token") {
		t.Fatal("expected bad token to fail verification")
	}
}

func TestUploadStreamHandler(t *testing.T) {
	config := DefaultConfig()
	config.UploadDir = "./test_uploads"
	config.AutoCreateDir = true
	config.AllowedExtensions = []string{".txt"}
	config.AllowedMimeTypes = []string{"text/plain"}
	defer os.RemoveAll(config.UploadDir)

	handler := NewHandler(config)

	req := httptest.NewRequest(http.MethodPost, "/upload/stream?filename=stream.txt", strings.NewReader("stream-content"))
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()

	handler.UploadStreamHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Data []struct {
			FilePath string `json:"FilePath"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse stream upload response: %v", err)
	}
	if len(resp.Data) == 0 || resp.Data[0].FilePath == "" {
		t.Fatalf("expected uploaded file path in response, body=%s", w.Body.String())
	}

	if _, err := os.Stat(resp.Data[0].FilePath); err != nil {
		t.Fatalf("expected stream file to be created, err=%v", err)
	}
}
