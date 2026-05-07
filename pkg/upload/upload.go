package upload

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
)

// UploadResult represents the result of a file upload
type UploadResult struct {
	OriginalFilename string
	SavedFilename    string
	FilePath         string
	FileSize         int64
	MimeType         string
	Success          bool
	Error            error
}

// Uploader handles file uploads
type Uploader struct {
	config    *Config
	namer     Namer
	validator *Validator
}

// NewUploader creates a new uploader with the given configuration
func NewUploader(config *Config) *Uploader {
	if config == nil {
		config = DefaultConfig()
	}

	return &Uploader{
		config:    config,
		namer:     GetNamer(config.NamingStrategy, config.CustomNameTemplate, config.PreserveExtension),
		validator: NewValidator(config),
	}
}

// Upload uploads a single file
func (u *Uploader) Upload(fileHeader *multipart.FileHeader) *UploadResult {
	result := &UploadResult{
		OriginalFilename: fileHeader.Filename,
		FileSize:         fileHeader.Size,
		Success:          false,
	}

	// Validate file
	if err := u.validator.Validate(fileHeader); err != nil {
		result.Error = err
		return result
	}

	// Generate new filename
	newFilename := u.namer.Generate(fileHeader.Filename)

	// Create upload directory if needed
	if u.config.AutoCreateDir {
		if err := os.MkdirAll(u.config.UploadDir, 0755); err != nil {
			result.Error = fmt.Errorf("failed to create upload directory: %w", err)
			return result
		}
	}

	// Build full file path
	filePath := filepath.Join(u.config.UploadDir, newFilename)

	// Check if file exists and overwrite is disabled
	if !u.config.EnableOverwrite {
		if _, err := os.Stat(filePath); err == nil {
			result.Error = fmt.Errorf("file already exists: %s", filePath)
			return result
		}
	}

	// Open the uploaded file
	src, err := fileHeader.Open()
	if err != nil {
		result.Error = fmt.Errorf("failed to open uploaded file: %w", err)
		return result
	}
	defer src.Close()

	// Create destination file
	dst, err := os.Create(filePath)
	if err != nil {
		result.Error = fmt.Errorf("failed to create destination file: %w", err)
		return result
	}
	defer dst.Close()

	// Copy file content
	if _, err := io.Copy(dst, src); err != nil {
		result.Error = fmt.Errorf("failed to copy file content: %w", err)
		return result
	}

	// Detect MIME type
	file, _ := os.Open(filePath)
	defer file.Close()
	buffer := make([]byte, 512)
	n, _ := file.Read(buffer)
	if n > 0 {
		result.MimeType = http.DetectContentType(buffer)
	}

	result.SavedFilename = newFilename
	result.FilePath = filePath
	result.Success = true

	logger.Info("File uploaded successfully",
		logger.String("original", fileHeader.Filename),
		logger.String("saved", newFilename),
		logger.Int64("size", fileHeader.Size))

	return result
}

// UploadMultiple uploads multiple files
func (u *Uploader) UploadMultiple(fileHeaders []*multipart.FileHeader) []*UploadResult {
	results := make([]*UploadResult, len(fileHeaders))

	for i, fileHeader := range fileHeaders {
		results[i] = u.Upload(fileHeader)
	}

	return results
}

// UploadFromBytes uploads file from byte data
func (u *Uploader) UploadFromBytes(filename string, data []byte) *UploadResult {
	result := &UploadResult{
		OriginalFilename: filename,
		FileSize:         int64(len(data)),
		Success:          false,
	}

	// Validate file size
	if int64(len(data)) > u.config.MaxFileSize {
		result.Error = &ValidationError{
			Field:   "file_size",
			Message: fmt.Sprintf("file size %d exceeds maximum allowed size %d", len(data), u.config.MaxFileSize),
		}
		return result
	}

	// Validate file extension
	if !u.validator.IsAllowedExtension(filename) {
		result.Error = &ValidationError{
			Field:   "file_extension",
			Message: fmt.Sprintf("file extension %s is not allowed", filepath.Ext(filename)),
		}
		return result
	}

	// Generate new filename
	newFilename := u.namer.Generate(filename)

	// Create upload directory if needed
	if u.config.AutoCreateDir {
		if err := os.MkdirAll(u.config.UploadDir, 0755); err != nil {
			result.Error = fmt.Errorf("failed to create upload directory: %w", err)
			return result
		}
	}

	// Build full file path
	filePath := filepath.Join(u.config.UploadDir, newFilename)

	// Check if file exists and overwrite is disabled
	if !u.config.EnableOverwrite {
		if _, err := os.Stat(filePath); err == nil {
			result.Error = fmt.Errorf("file already exists: %s", filePath)
			return result
		}
	}

	// Write file
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		result.Error = fmt.Errorf("failed to write file: %w", err)
		return result
	}

	result.SavedFilename = newFilename
	result.FilePath = filePath
	result.Success = true

	logger.Info("File uploaded from bytes successfully",
		logger.String("original", filename),
		logger.String("saved", newFilename),
		logger.Int("size", len(data)))

	return result
}

// Delete deletes a file
func (u *Uploader) Delete(filename string) error {
	filePath := filepath.Join(u.config.UploadDir, filename)
	return os.Remove(filePath)
}

// Exists checks if a file exists
func (u *Uploader) Exists(filename string) bool {
	filePath := filepath.Join(u.config.UploadDir, filename)
	_, err := os.Stat(filePath)
	return err == nil
}

// GetConfig returns the current configuration
func (u *Uploader) GetConfig() *Config {
	return u.config
}

// UpdateConfig updates the configuration
func (u *Uploader) UpdateConfig(config *Config) {
	u.config = config
	u.namer = GetNamer(config.NamingStrategy, config.CustomNameTemplate, config.PreserveExtension)
	u.validator = NewValidator(config)
}
