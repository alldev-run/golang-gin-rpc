package gin

import (
	"crypto/subtle"
	"fmt"
	"net/http"

	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
	"github.com/alldev-run/golang-gin-rpc/pkg/upload"
	"github.com/gin-gonic/gin"
)

// Middleware provides Gin middleware for file upload
type Middleware struct {
	handler *upload.Handler
}

// NewMiddleware creates a new Gin middleware
func NewMiddleware(config *upload.Config) *Middleware {
	return &Middleware{
		handler: upload.NewHandler(config),
	}
}

func secureEquals(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func (m *Middleware) checkEnabled() bool {
	return m.handler.GetUploader().GetConfig().Enabled
}

// checkAuth checks if the request is authenticated
func (m *Middleware) checkAuth(c *gin.Context) bool {
	config := m.handler.GetUploader().GetConfig()
	if !config.EnableAuth {
		return true
	}

	if config.AuthUsername == "" || config.AuthPassword == "" {
		return false
	}

	username, password, ok := c.Request.BasicAuth()
	if !ok {
		return false
	}

	return secureEquals(username, config.AuthUsername) && secureEquals(password, config.AuthPassword)
}

// sendAuthRequired sends an authentication required response
func (m *Middleware) sendAuthRequired(c *gin.Context) {
	c.Header("WWW-Authenticate", `Basic realm="Upload Server"`)
	c.JSON(http.StatusUnauthorized, gin.H{
		"success": false,
		"message": "Authentication required",
	})
	c.Abort()
}

// UploadHandler handles file upload in Gin
func (m *Middleware) UploadHandler(c *gin.Context) {
	// Set CORS headers
	m.setCORSHeaders(c)

	if !m.checkEnabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": "Upload service is disabled"})
		return
	}

	// Check authentication
	if !m.checkAuth(c) {
		m.sendAuthRequired(c)
		return
	}

	// Get files from form
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Failed to parse multipart form",
		})
		return
	}

	files := form.File["files"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "No files uploaded",
		})
		return
	}

	// Upload files
	results := m.handler.GetUploader().UploadMultiple(files)

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
		c.JSON(http.StatusMultiStatus, gin.H{
			"success": false,
			"message": fmt.Sprintf("%d files uploaded successfully, %d files failed", successCount, len(errors)),
			"data":    results,
			"errors":  errors,
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": fmt.Sprintf("%d files uploaded successfully", successCount),
			"data":    results,
		})
	}

	logger.Info("File upload request processed via Gin",
		logger.Int("total_files", len(files)),
		logger.Int("success_count", successCount),
		logger.Int("error_count", len(errors)))
}

// SingleUploadHandler handles single file upload in Gin
func (m *Middleware) SingleUploadHandler(c *gin.Context) {
	// Set CORS headers
	m.setCORSHeaders(c)

	if !m.checkEnabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": "Upload service is disabled"})
		return
	}

	// Check authentication
	if !m.checkAuth(c) {
		m.sendAuthRequired(c)
		return
	}

	// Get file from form
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "No file uploaded",
		})
		return
	}

	// Upload file
	result := m.handler.GetUploader().Upload(file)

	// Send response
	if !result.Success {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": result.Error.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "File uploaded successfully",
		"data":    result,
	})

	logger.Info("Single file upload processed via Gin",
		logger.String("filename", result.OriginalFilename),
		logger.String("saved_as", result.SavedFilename))
}

// DeleteHandler handles file deletion in Gin
func (m *Middleware) DeleteHandler(c *gin.Context) {
	// Set CORS headers
	m.setCORSHeaders(c)

	if !m.checkEnabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": "Upload service is disabled"})
		return
	}

	// Check authentication
	if !m.checkAuth(c) {
		m.sendAuthRequired(c)
		return
	}

	// Get filename from query parameter
	filename := c.Query("filename")
	if filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Filename is required",
		})
		return
	}

	// Delete file
	if err := m.handler.GetUploader().Delete(filename); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Failed to delete file: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "File deleted successfully",
	})

	logger.Info("File deleted via Gin", logger.String("filename", filename))
}

// ListHandler handles file listing in Gin
func (m *Middleware) ListHandler(c *gin.Context) {
	// Set CORS headers
	m.setCORSHeaders(c)

	if !m.checkEnabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": "Upload service is disabled"})
		return
	}

	// Check authentication
	if !m.checkAuth(c) {
		m.sendAuthRequired(c)
		return
	}

	// List files
	files, err := m.handler.GetUploader().ListFiles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Failed to list files: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Files listed successfully",
		"data":    files,
	})

	logger.Info("Files listed via Gin", logger.Int("count", len(files)))
}

// DownloadHandler handles file download in Gin
func (m *Middleware) DownloadHandler(c *gin.Context) {
	m.handler.DownloadHandler(c.Writer, c.Request)
	c.Abort()
}

// UploadStreamHandler handles raw binary stream upload in Gin
func (m *Middleware) UploadStreamHandler(c *gin.Context) {
	m.handler.UploadStreamHandler(c.Writer, c.Request)
	c.Abort()
}

// IssueTokenHandler issues short-lived token in Gin
func (m *Middleware) IssueTokenHandler(c *gin.Context) {
	m.handler.IssueTokenHandler(c.Writer, c.Request)
	c.Abort()
}

// setCORSHeaders sets CORS headers for Gin
func (m *Middleware) setCORSHeaders(c *gin.Context) {
	config := m.handler.GetUploader().GetConfig()
	if !config.EnableCORS {
		return
	}

	corsConfig := config.CORS

	// Set allowed origins
	if len(corsConfig.AllowedOrigins) > 0 {
		origin := c.Request.Header.Get("Origin")
		for _, allowedOrigin := range corsConfig.AllowedOrigins {
			if allowedOrigin == "*" || allowedOrigin == origin {
				c.Header("Access-Control-Allow-Origin", allowedOrigin)
				break
			}
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
		c.Header("Access-Control-Allow-Methods", methods)
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
		c.Header("Access-Control-Allow-Headers", headers)
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
		c.Header("Access-Control-Expose-Headers", headers)
	}

	// Set allow credentials
	if corsConfig.AllowCredentials {
		c.Header("Access-Control-Allow-Credentials", "true")
	}

	// Set max age
	if corsConfig.MaxAge > 0 {
		c.Header("Access-Control-Max-Age", fmt.Sprintf("%d", corsConfig.MaxAge))
	}
}

// CORSMiddleware returns a CORS middleware for Gin
func (m *Middleware) CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		m.setCORSHeaders(c)

		// Handle preflight requests
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	}
}

// RegisterRoutes registers upload routes in Gin
func (m *Middleware) RegisterRoutes(router *gin.RouterGroup) {
	router.Use(m.CORSMiddleware())
	router.POST("/upload", m.UploadHandler)
	router.POST("/upload/single", m.SingleUploadHandler)
	router.POST("/upload/stream", m.UploadStreamHandler)
	router.GET("/list", m.ListHandler)
	router.GET("/token", m.IssueTokenHandler)
	router.GET("/download", m.DownloadHandler)
	router.DELETE("/delete", m.DeleteHandler)
}
