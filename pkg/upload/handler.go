package upload

import (
	"encoding/json"
	"fmt"
	"net/http"

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

// UploadHandler handles file upload requests
func (h *Handler) UploadHandler(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	h.setCORSHeaders(w)

	// Handle preflight requests
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
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

	// Handle preflight requests
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
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

// DeleteHandler handles file deletion requests
func (h *Handler) DeleteHandler(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	h.setCORSHeaders(w)

	// Handle preflight requests
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
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
		for _, origin := range corsConfig.AllowedOrigins {
			w.Header().Add("Access-Control-Allow-Origin", origin)
		}
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
	http.HandleFunc("/delete", h.DeleteHandler)

	logger.Info("Starting upload server", logger.String("addr", addr))
	return http.ListenAndServe(addr, nil)
}
