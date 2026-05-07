package upload

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/auth/jwtx"
	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
)

// Handler provides HTTP handlers for file upload
type Handler struct {
	uploader *Uploader
}

// NewHandler creates a new upload handler
func NewHandler(config *Config) *Handler {
	return &Handler{
		uploader: NewUploader(config),
	}
}

// UploadResponse represents the HTTP response for file upload
type UploadResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    []*UploadResult `json:"data,omitempty"`
	Errors  []string        `json:"errors,omitempty"`
}

func secureEquals(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// checkEnabled checks if upload service is enabled
func (h *Handler) checkEnabled() bool {
	return h.GetConfig().Enabled
}

// checkAuth checks if the request is authenticated
func (h *Handler) checkAuth(r *http.Request) bool {
	config := h.GetConfig()
	if !config.EnableAuth {
		return true
	}

	if config.AuthUsername == "" || config.AuthPassword == "" {
		return false
	}

	username, password, ok := r.BasicAuth()
	if !ok {
		return false
	}

	return secureEquals(username, config.AuthUsername) && secureEquals(password, config.AuthPassword)
}

// sendAuthRequired sends an authentication required response
func (h *Handler) sendAuthRequired(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="Upload Server"`)
	h.sendError(w, http.StatusUnauthorized, "Authentication required")
}

func (h *Handler) getTokenSecret() string {
	config := h.GetConfig()
	if config.TokenSecret != "" {
		return config.TokenSecret
	}
	return config.AuthPassword
}

func (h *Handler) getJWTManagerForDownloadToken() (*jwtx.Manager, error) {
	secret := h.getTokenSecret()
	if secret == "" {
		return nil, fmt.Errorf("token secret is not configured")
	}

	ttl := h.GetConfig().TokenTTLSeconds
	if ttl <= 0 {
		ttl = 300
	}

	return jwtx.NewManager(jwtx.Config{
		Secret:         secret,
		AccessTokenTTL: time.Duration(ttl) * time.Second,
	}), nil
}

func (h *Handler) verifyDownloadToken(filename, token string) bool {
	if filename == "" || token == "" {
		return false
	}

	manager, err := h.getJWTManagerForDownloadToken()
	if err != nil {
		return false
	}

	claims, err := manager.ParseClaims(token)
	if err != nil {
		return false
	}

	if claims.Type != jwtx.TokenTypeAccess {
		return false
	}

	if time.Now().After(claims.ExpireAt) {
		return false
	}

	if claims.Payload == nil {
		return false
	}

	if claims.Payload["scope"] != "upload_download" {
		return false
	}

	return claims.Payload["filename"] == filename
}

func (h *Handler) extractTokenString(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(strings.ToLower(trimmed), "bearer ") {
		return strings.TrimSpace(trimmed[7:])
	}
	return trimmed
}

// GenerateDownloadToken generates a short-lived signed token for a file.
func (h *Handler) GenerateDownloadToken(filename string) (string, int64, error) {
	trimmed := strings.TrimSpace(filename)
	if trimmed == "" {
		return "", 0, fmt.Errorf("filename is required")
	}

	if !h.uploader.Exists(trimmed) {
		return "", 0, fmt.Errorf("file not found")
	}

	ttl := h.GetConfig().TokenTTLSeconds
	if ttl <= 0 {
		ttl = 300
	}

	manager, err := h.getJWTManagerForDownloadToken()
	if err != nil {
		return "", 0, err
	}

	now := time.Now()
	expiresAt := now.Add(time.Duration(ttl) * time.Second).Unix()
	token, err := manager.SignAccessClaims(jwtx.Claims{
		UserID:   "upload",
		Username: "upload-download",
		DeviceID: "upload",
		Type:     jwtx.TokenTypeAccess,
		IssuedAt: now,
		ExpireAt: time.Unix(expiresAt, 0),
		Payload: map[string]string{
			"scope":    "upload_download",
			"filename": trimmed,
		},
	})
	if err != nil {
		return "", 0, err
	}

	return token, expiresAt, nil
}

// VerifyDownloadToken verifies a signed token for a file.
func (h *Handler) VerifyDownloadToken(filename string, expiresAt int64, token string) bool {
	if strings.TrimSpace(filename) == "" || strings.TrimSpace(token) == "" || expiresAt <= 0 {
		return false
	}
	if time.Now().Unix() > expiresAt {
		return false
	}
	return h.verifyDownloadToken(strings.TrimSpace(filename), strings.TrimSpace(token))
}

// IssueTokenHandler issues a short-lived signed token for downloading a file.
func (h *Handler) IssueTokenHandler(w http.ResponseWriter, r *http.Request) {
	h.setCORSHeaders(w)

	if !h.checkEnabled() {
		h.sendError(w, http.StatusServiceUnavailable, "Upload service is disabled")
		return
	}

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet {
		h.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	if !h.GetConfig().EnableAuth || !h.checkAuth(r) {
		h.sendAuthRequired(w)
		return
	}

	filename := strings.TrimSpace(r.URL.Query().Get("filename"))
	if filename == "" {
		h.sendError(w, http.StatusBadRequest, "Filename is required")
		return
	}

	token, expiresAt, err := h.GenerateDownloadToken(filename)
	if err != nil {
		if err.Error() == "file not found" {
			h.sendError(w, http.StatusNotFound, err.Error())
			return
		}
		h.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to issue token: %v", err))
		return
	}

	ttl := h.GetConfig().TokenTTLSeconds
	if ttl <= 0 {
		ttl = 300
	}

	h.sendResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Token issued successfully",
		"data": map[string]interface{}{
			"filename": filename,
			"token":    token,
			"expires":  expiresAt,
			"ttl":      ttl,
		},
	})
}

// UploadHandler handles file upload requests
func (h *Handler) UploadHandler(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	h.setCORSHeaders(w)

	if !h.checkEnabled() {
		h.sendError(w, http.StatusServiceUnavailable, "Upload service is disabled")
		return
	}

	// Handle preflight requests
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Check authentication
	if !h.checkAuth(r) {
		h.sendAuthRequired(w)
		return
	}

	// Only accept POST requests
	if r.Method != http.MethodPost {
		h.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Parse multipart form
	if err := r.ParseMultipartForm(h.uploader.GetConfig().MaxFileSize); err != nil {
		h.sendError(w, http.StatusBadRequest, "Failed to parse multipart form")
		return
	}

	// Get files from request
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		h.sendError(w, http.StatusBadRequest, "No files uploaded")
		return
	}

	// Upload files
	results := h.uploader.UploadMultiple(files)

	// Check for errors
	var errors []string
	var successCount int
	for _, result := range results {
		if !result.Success {
			errors = append(errors, result.Error.Error())
		} else {
			successCount++
		}
	}

	// Send response
	if len(errors) > 0 {
		h.sendResponse(w, http.StatusMultiStatus, UploadResponse{
			Success: false,
			Message: fmt.Sprintf("%d files uploaded successfully, %d files failed", successCount, len(errors)),
			Data:    results,
			Errors:  errors,
		})
	} else {
		h.sendResponse(w, http.StatusOK, UploadResponse{
			Success: true,
			Message: fmt.Sprintf("%d files uploaded successfully", successCount),
			Data:    results,
		})
	}

	logger.Info("File upload request processed",
		logger.Int("total_files", len(files)),
		logger.Int("success_count", successCount),
		logger.Int("error_count", len(errors)))
}

// SingleUploadHandler handles single file upload
func (h *Handler) SingleUploadHandler(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	h.setCORSHeaders(w)

	if !h.checkEnabled() {
		h.sendError(w, http.StatusServiceUnavailable, "Upload service is disabled")
		return
	}

	// Handle preflight requests
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Check authentication
	if !h.checkAuth(r) {
		h.sendAuthRequired(w)
		return
	}

	// Only accept POST requests
	if r.Method != http.MethodPost {
		h.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Parse multipart form
	if err := r.ParseMultipartForm(h.uploader.GetConfig().MaxFileSize); err != nil {
		h.sendError(w, http.StatusBadRequest, "Failed to parse multipart form")
		return
	}

	// Get file from request
	files := r.MultipartForm.File["file"]
	if len(files) == 0 {
		h.sendError(w, http.StatusBadRequest, "No file uploaded")
		return
	}

	if len(files) > 1 {
		h.sendError(w, http.StatusBadRequest, "Only one file allowed")
		return
	}

	// Upload file
	result := h.uploader.Upload(files[0])

	// Send response
	if !result.Success {
		h.sendError(w, http.StatusBadRequest, result.Error.Error())
		return
	}

	h.sendResponse(w, http.StatusOK, UploadResponse{
		Success: true,
		Message: "File uploaded successfully",
		Data:    []*UploadResult{result},
	})

	logger.Info("Single file upload processed",
		logger.String("filename", result.OriginalFilename),
		logger.String("saved_as", result.SavedFilename))
}

// UploadStreamHandler handles raw binary stream upload.
// Filename should be provided via query `filename` or header `X-Filename`.
func (h *Handler) UploadStreamHandler(w http.ResponseWriter, r *http.Request) {
	h.setCORSHeaders(w)

	if !h.checkEnabled() {
		h.sendError(w, http.StatusServiceUnavailable, "Upload service is disabled")
		return
	}

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if !h.checkAuth(r) {
		h.sendAuthRequired(w)
		return
	}

	if r.Method != http.MethodPost {
		h.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	filename := strings.TrimSpace(r.URL.Query().Get("filename"))
	if filename == "" {
		filename = strings.TrimSpace(r.Header.Get("X-Filename"))
	}
	if filename == "" {
		h.sendError(w, http.StatusBadRequest, "Filename is required")
		return
	}

	maxSize := h.uploader.GetConfig().MaxFileSize
	r.Body = http.MaxBytesReader(w, r.Body, maxSize)
	defer r.Body.Close()

	data, err := io.ReadAll(r.Body)
	if err != nil {
		h.sendError(w, http.StatusBadRequest, "Failed to read stream body")
		return
	}

	result := h.uploader.UploadFromBytes(filename, data)
	if !result.Success {
		h.sendError(w, http.StatusBadRequest, result.Error.Error())
		return
	}

	h.sendResponse(w, http.StatusOK, UploadResponse{
		Success: true,
		Message: "Stream uploaded successfully",
		Data:    []*UploadResult{result},
	})

	logger.Info("Stream upload processed",
		logger.String("filename", result.OriginalFilename),
		logger.String("saved_as", result.SavedFilename),
		logger.Int64("size", result.FileSize))
}

// DeleteHandler handles file deletion requests
func (h *Handler) DeleteHandler(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	h.setCORSHeaders(w)

	if !h.checkEnabled() {
		h.sendError(w, http.StatusServiceUnavailable, "Upload service is disabled")
		return
	}

	// Handle preflight requests
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Check authentication
	if !h.checkAuth(r) {
		h.sendAuthRequired(w)
		return
	}

	// Only accept DELETE requests
	if r.Method != http.MethodDelete {
		h.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Get filename from query parameter
	filename := r.URL.Query().Get("filename")
	if filename == "" {
		h.sendError(w, http.StatusBadRequest, "Filename is required")
		return
	}

	// Delete file
	if err := h.uploader.Delete(filename); err != nil {
		h.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete file: %v", err))
		return
	}

	h.sendResponse(w, http.StatusOK, UploadResponse{
		Success: true,
		Message: "File deleted successfully",
	})

	logger.Info("File deleted", logger.String("filename", filename))
}

// ListHandler handles file listing requests
func (h *Handler) ListHandler(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	h.setCORSHeaders(w)

	if !h.checkEnabled() {
		h.sendError(w, http.StatusServiceUnavailable, "Upload service is disabled")
		return
	}

	// Handle preflight requests
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Check authentication
	if !h.checkAuth(r) {
		h.sendAuthRequired(w)
		return
	}

	// Only accept GET requests
	if r.Method != http.MethodGet {
		h.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// List files
	files, err := h.uploader.ListFiles()
	if err != nil {
		h.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list files: %v", err))
		return
	}

	h.sendResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Files listed successfully",
		"data":    files,
	})

	logger.Info("Files listed", logger.Int("count", len(files)))
}

// DownloadHandler handles file download requests
func (h *Handler) DownloadHandler(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	h.setCORSHeaders(w)

	if !h.checkEnabled() {
		h.sendError(w, http.StatusServiceUnavailable, "Upload service is disabled")
		return
	}

	// Handle preflight requests
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Only accept GET requests
	if r.Method != http.MethodGet {
		h.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Get filename from query parameter
	filename := strings.TrimSpace(r.URL.Query().Get("filename"))
	if filename == "" {
		h.sendError(w, http.StatusBadRequest, "Filename is required")
		return
	}

	// Allow either valid short-lived bearer token/header token or valid auth.
	token := h.extractTokenString(r.Header.Get("Authorization"))
	if token == "" {
		token = h.extractTokenString(r.URL.Query().Get("token"))
	}
	hasValidToken := h.verifyDownloadToken(filename, token)
	if !hasValidToken && !h.checkAuth(r) {
		h.sendAuthRequired(w)
		return
	}

	// Serve file
	if err := h.uploader.ServeFile(filename, w); err != nil {
		h.sendError(w, http.StatusNotFound, fmt.Sprintf("Failed to serve file: %v", err))
		return
	}
}

// GetConfig returns the current configuration
func (h *Handler) GetConfig() *Config {
	return h.uploader.GetConfig()
}

// GetUploader returns the uploader instance
func (h *Handler) GetUploader() *Uploader {
	return h.uploader
}

// UpdateConfig updates the configuration
func (h *Handler) UpdateConfig(config *Config) {
	h.uploader.UpdateConfig(config)
}

// setCORSHeaders sets CORS headers based on configuration
func (h *Handler) setCORSHeaders(w http.ResponseWriter) {
	config := h.GetConfig()
	if !config.EnableCORS {
		return
	}

	corsConfig := config.CORS

	// Set allowed origins
	if len(corsConfig.AllowedOrigins) > 0 {
		w.Header().Set("Access-Control-Allow-Origin", corsConfig.AllowedOrigins[0])
	}

	// Set allowed methods
	if len(corsConfig.AllowedMethods) > 0 {
		methods := ""
		for i, method := range corsConfig.AllowedMethods {
			if i > 0 {
				methods += ", "
			}
			methods += method
		}
		w.Header().Set("Access-Control-Allow-Methods", methods)
	}

	// Set allowed headers
	if len(corsConfig.AllowedHeaders) > 0 {
		headers := ""
		for i, header := range corsConfig.AllowedHeaders {
			if i > 0 {
				headers += ", "
			}
			headers += header
		}
		w.Header().Set("Access-Control-Allow-Headers", headers)
	}

	// Set exposed headers
	if len(corsConfig.ExposedHeaders) > 0 {
		headers := ""
		for i, header := range corsConfig.ExposedHeaders {
			if i > 0 {
				headers += ", "
			}
			headers += header
		}
		w.Header().Set("Access-Control-Expose-Headers", headers)
	}

	// Set allow credentials
	if corsConfig.AllowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}

	// Set max age
	if corsConfig.MaxAge > 0 {
		w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", corsConfig.MaxAge))
	}
}

// sendResponse sends a JSON response
func (h *Handler) sendResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// sendError sends an error response
func (h *Handler) sendError(w http.ResponseWriter, statusCode int, message string) {
	h.sendResponse(w, statusCode, UploadResponse{
		Success: false,
		Message: message,
	})
}

// StartServer starts the standalone upload server
func (h *Handler) StartServer(addr string) error {
	config := h.uploader.GetConfig()
	if !config.EnableServer {
		return fmt.Errorf("server is not enabled in configuration")
	}

	if addr == "" {
		addr = fmt.Sprintf(":%d", config.Port)
	}

	http.HandleFunc("/upload", h.UploadHandler)
	http.HandleFunc("/upload/single", h.SingleUploadHandler)
	http.HandleFunc("/upload/stream", h.UploadStreamHandler)
	http.HandleFunc("/list", h.ListHandler)
	http.HandleFunc("/token", h.IssueTokenHandler)
	http.HandleFunc("/download", h.DownloadHandler)
	http.HandleFunc("/delete", h.DeleteHandler)

	logger.Info("Starting upload server", logger.String("addr", addr))
	return http.ListenAndServe(addr, nil)
}
