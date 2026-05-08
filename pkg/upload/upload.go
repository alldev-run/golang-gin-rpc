package upload

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
)

// FileInfo represents information about an uploaded file
type FileInfo struct {
	Filename    string    `json:"filename"`
	FilePath    string    `json:"file_path"`
	FileSize    int64     `json:"file_size"`
	ModTime     time.Time `json:"mod_time"`
	IsDirectory bool      `json:"is_directory"`
}

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
	filePath, err := u.resolveSafeFilePath(newFilename)
	if err != nil {
		result.Error = err
		return result
	}

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
	openFlags := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	if !u.config.EnableOverwrite {
		openFlags = os.O_CREATE | os.O_WRONLY | os.O_EXCL
	}

	dst, err := os.OpenFile(filePath, openFlags, 0o600)
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
	// Return path starting from configured UploadDir for frontend use
	result.FilePath = filepath.Join(u.config.UploadDir, newFilename)
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

	// Validate MIME type inferred from content
	mimeType := http.DetectContentType(data)
	if !u.validator.IsAllowedMimeType(mimeType) {
		result.Error = &ValidationError{
			Field:   "file_mime_type",
			Message: fmt.Sprintf("file MIME type %s is not allowed", mimeType),
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
	filePath, err := u.resolveSafeFilePath(newFilename)
	if err != nil {
		result.Error = err
		return result
	}

	// Check if file exists and overwrite is disabled
	if !u.config.EnableOverwrite {
		if _, err := os.Stat(filePath); err == nil {
			result.Error = fmt.Errorf("file already exists: %s", filePath)
			return result
		}
	}

	// Write file
	if err := os.WriteFile(filePath, data, 0o600); err != nil {
		result.Error = fmt.Errorf("failed to write file: %w", err)
		return result
	}

	result.SavedFilename = newFilename
	// Return path starting from configured UploadDir for frontend use
	result.FilePath = filepath.Join(u.config.UploadDir, newFilename)
	result.MimeType = mimeType
	result.Success = true

	logger.Info("File uploaded from bytes successfully",
		logger.String("original", filename),
		logger.String("saved", newFilename),
		logger.Int("size", len(data)))

	return result
}

// Delete deletes a file
func (u *Uploader) Delete(filename string) error {
	filePath, err := u.resolveSafeFilePath(filename)
	if err != nil {
		return err
	}
	return os.Remove(filePath)
}

// Exists checks if a file exists
func (u *Uploader) Exists(filename string) bool {
	filePath, err := u.resolveSafeFilePath(filename)
	if err != nil {
		return false
	}
	_, err = os.Stat(filePath)
	return err == nil
}

// ListFiles lists all files in the upload directory
func (u *Uploader) ListFiles() ([]*FileInfo, error) {
	entries, err := os.ReadDir(u.config.UploadDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read upload directory: %w", err)
	}

	var files []*FileInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		files = append(files, &FileInfo{
			Filename:    entry.Name(),
			FilePath:    entry.Name(),
			FileSize:    info.Size(),
			ModTime:     info.ModTime(),
			IsDirectory: false,
		})
	}

	return files, nil
}

// ServeFile serves a file for download
func (u *Uploader) ServeFile(filename string, w http.ResponseWriter) error {
	filePath, err := u.resolveSafeFilePath(filename)
	if err != nil {
		return err
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", filename)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	buffer := make([]byte, 512)
	n, _ := file.Read(buffer)
	contentType := http.DetectContentType(buffer[:n])
	if contentType == "application/octet-stream" {
		if extType := mime.TypeByExtension(filepath.Ext(filename)); extType != "" {
			contentType = extType
		}
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek file: %w", err)
	}

	// Set headers
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", path.Base(filename)))

	if _, err := io.Copy(w, file); err != nil {
		return fmt.Errorf("failed to stream file: %w", err)
	}

	logger.Info("File served", logger.String("filename", filename))
	return nil
}

func (u *Uploader) resolveSafeFilePath(filename string) (string, error) {
	clean := strings.TrimSpace(filename)
	if clean == "" {
		return "", errors.New("filename is required")
	}

	if strings.Contains(clean, "\\") {
		return "", fmt.Errorf("invalid filename: %s", filename)
	}

	normalized := filepath.Clean(clean)
	if normalized == "." || normalized == ".." {
		return "", fmt.Errorf("invalid filename: %s", filename)
	}

	if strings.HasPrefix(normalized, "../") || strings.Contains(normalized, "/../") {
		return "", fmt.Errorf("invalid filename: %s", filename)
	}

	baseDir, err := filepath.Abs(u.config.UploadDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve upload directory: %w", err)
	}

	fullPath := filepath.Join(baseDir, normalized)
	fullPathAbs, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve file path: %w", err)
	}

	rel, err := filepath.Rel(baseDir, fullPathAbs)
	if err != nil {
		return "", fmt.Errorf("failed to validate file path: %w", err)
	}

	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid filename: %s", filename)
	}

	return fullPathAbs, nil
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
