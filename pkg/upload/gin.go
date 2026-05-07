package upload

import (
	"fmt"
	"net/http"

	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
	"github.com/gin-gonic/gin"
)

// GinMiddleware provides Gin middleware for file upload
type GinMiddleware struct {
	handler *Handler
}

// NewGinMiddleware creates a new Gin middleware
func NewGinMiddleware(config *Config) *GinMiddleware {
	return &GinMiddleware{
		handler: NewHandler(config),
	}
}

// UploadHandler handles file upload in Gin
func (m *GinMiddleware) UploadHandler(c *gin.Context) {
	// Set CORS headers
	m.setCORSHeaders(c)

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
	results := m.handler.uploader.UploadMultiple(files)

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
func (m *GinMiddleware) SingleUploadHandler(c *gin.Context) {
	// Set CORS headers
	m.setCORSHeaders(c)

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
	result := m.handler.uploader.Upload(file)

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
func (m *GinMiddleware) DeleteHandler(c *gin.Context) {
	// Set CORS headers
	m.setCORSHeaders(c)

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
	if err := m.handler.uploader.Delete(filename); err != nil {
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

// setCORSHeaders sets CORS headers for Gin
func (m *GinMiddleware) setCORSHeaders(c *gin.Context) {
	config := m.handler.uploader.GetConfig()
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
func (m *GinMiddleware) CORSMiddleware() gin.HandlerFunc {
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
func (m *GinMiddleware) RegisterRoutes(router *gin.RouterGroup) {
	router.Use(m.CORSMiddleware())
	router.POST("/upload", m.UploadHandler)
	router.POST("/upload/single", m.SingleUploadHandler)
	router.DELETE("/delete", m.DeleteHandler)
}
