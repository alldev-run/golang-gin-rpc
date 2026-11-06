package upload

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
)

// ValidationError represents a file validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Validator validates uploaded files
type Validator struct {
	config *Config
}

func NewValidator(config *Config) *Validator {
	return &Validator{config: config}
}

// Validate validates a file against the configuration
func (v *Validator) Validate(fileHeader *multipart.FileHeader) error {
	// Check file size
	if fileHeader.Size > v.config.MaxFileSize {
		return &ValidationError{
			Field:   "file_size",
			Message: fmt.Sprintf("file size %d exceeds maximum allowed size %d", fileHeader.Size, v.config.MaxFileSize),
		}
	}

	// Check file extension
	if len(v.config.AllowedExtensions) > 0 {
		ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
		allowed := false
		for _, allowedExt := range v.config.AllowedExtensions {
			if strings.ToLower(allowedExt) == ext {
				allowed = true
				break
			}
		}
		if !allowed {
			return &ValidationError{
				Field:   "file_extension",
				Message: fmt.Sprintf("file extension %s is not allowed", ext),
			}
		}
	}

	// Check MIME type
	if len(v.config.AllowedMimeTypes) > 0 {
		file, err := fileHeader.Open()
		if err != nil {
			return &ValidationError{
				Field:   "file_open",
				Message: fmt.Sprintf("failed to open file: %v", err),
			}
		}
		defer file.Close()

		// Read first 512 bytes to detect MIME type
		buffer := make([]byte, 512)
		_, err = file.Read(buffer)
		if err != nil && err != io.EOF {
			return &ValidationError{
				Field:   "file_read",
				Message: fmt.Sprintf("failed to read file: %v", err),
			}
		}

		mimeType := http.DetectContentType(buffer)
		allowed := false
		for _, allowedMime := range v.config.AllowedMimeTypes {
			if strings.HasPrefix(mimeType, strings.Split(allowedMime, "/")[0]+"/") {
				allowed = true
				break
			}
			if mimeType == allowedMime {
				allowed = true
				break
			}
		}
		if !allowed {
			return &ValidationError{
				Field:   "file_mime_type",
				Message: fmt.Sprintf("file MIME type %s is not allowed", mimeType),
			}
		}
	}

	return nil
}

// ValidateMultiple validates multiple files
func (v *Validator) ValidateMultiple(fileHeaders []*multipart.FileHeader) []error {
	var errs []error
	for _, fileHeader := range fileHeaders {
		if err := v.Validate(fileHeader); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// IsAllowedExtension checks if an extension is allowed
func (v *Validator) IsAllowedExtension(filename string) bool {
	if len(v.config.AllowedExtensions) == 0 {
		return true
	}
	ext := strings.ToLower(filepath.Ext(filename))
	for _, allowedExt := range v.config.AllowedExtensions {
		if strings.ToLower(allowedExt) == ext {
			return true
		}
	}
	return false
}

// IsAllowedMimeType checks if a MIME type is allowed
func (v *Validator) IsAllowedMimeType(mimeType string) bool {
	if len(v.config.AllowedMimeTypes) == 0 {
		return true
	}
	for _, allowedMime := range v.config.AllowedMimeTypes {
		if strings.HasPrefix(mimeType, strings.Split(allowedMime, "/")[0]+"/") {
			return true
		}
		if mimeType == allowedMime {
			return true
		}
	}
	return false
}

// GetMaxFileSize returns the maximum allowed file size
func (v *Validator) GetMaxFileSize() int64 {
	return v.config.MaxFileSize
}
