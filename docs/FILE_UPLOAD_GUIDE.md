# File Upload Guide

The golang-gin-rpc framework provides a comprehensive file upload functionality with configurable validation, naming strategies, CORS support, and seamless integration with both standard HTTP and Gin framework.

## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Configuration](#configuration)
- [Quick Start](#quick-start)
- [Integration with Gin](#integration-with-gin)
- [Naming Strategies](#naming-strategies)
- [File Validation](#file-validation)
- [CORS Configuration](#cors-configuration)
- [API Endpoints](#api-endpoints)
- [Security Considerations](#security-considerations)
- [Troubleshooting](#troubleshooting)

## Overview

The upload package (`pkg/upload`) provides a flexible and secure file upload solution for the golang-gin-rpc framework. It supports:

- Multiple file upload methods (multipart form, bytes)
- Configurable file type and size validation
- Multiple naming strategies (UUID, timestamp, custom)
- CORS support for cross-origin requests
- Gin framework middleware integration
- Standalone server mode
- Comprehensive error handling

## Features

### Core Features

- **File Validation**: Validate files by extension and MIME type
- **Size Limits**: Configure maximum file size to prevent DoS attacks
- **Naming Strategies**: Multiple options for auto-naming uploaded files
- **CORS Support**: Full CORS configuration for cross-origin requests
- **Gin Integration**: Built-in middleware for seamless Gin integration
- **Standalone Server**: Option to run as a standalone upload server
- **Auto Directory Creation**: Automatically creates upload directories
- **File Overwrite Control**: Configurable file overwrite behavior

## Configuration

### Basic Configuration

Add the upload configuration to your `config.yaml`:

```yaml
upload:
  enabled: false  # Set to true to enable file upload functionality
  upload_dir: "./uploads"
  max_file_size: 10485760  # 10MB in bytes
  allowed_extensions:
    - ".jpg"
    - ".jpeg"
    - ".png"
    - ".gif"
    - ".pdf"
  allowed_mime_types:
    - "image/jpeg"
    - "image/png"
    - "image/gif"
    - "application/pdf"
  naming_strategy: "uuid"  # uuid, timestamp, original, custom
  custom_name_template: "{date}_{original}_{random}"
  preserve_extension: true
  enable_cors: true
  cors:
    allowed_origins:
      - "*"
    allowed_methods:
      - "GET"
      - "POST"
      - "OPTIONS"
      - "DELETE"
    allowed_headers:
      - "Origin"
      - "Content-Type"
      - "Accept"
      - "Authorization"
    allow_credentials: false
    max_age: 86400
  port: 8081
  enable_server: false
  auto_create_dir: true
  enable_overwrite: false
```

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | `false` | Enable/disable file upload functionality |
| `upload_dir` | string | `"./uploads"` | Directory to store uploaded files |
| `max_file_size` | int64 | `10485760` | Maximum file size in bytes (10MB default) |
| `allowed_extensions` | []string | See default | Allowed file extensions |
| `allowed_mime_types` | []string | See default | Allowed MIME types |
| `naming_strategy` | string | `"uuid"` | File naming strategy |
| `custom_name_template` | string | `""` | Custom naming template |
| `preserve_extension` | bool | `true` | Preserve original file extension |
| `enable_cors` | bool | `true` | Enable CORS support |
| `port` | int | `8081` | Port for standalone server |
| `enable_server` | bool | `false` | Enable standalone server mode |
| `auto_create_dir` | bool | `true` | Auto-create upload directory |
| `enable_overwrite` | bool | `false` | Allow file overwrites |

## Quick Start

### Basic File Upload

```go
package main

import (
    "fmt"
    "github.com/alldev-run/golang-gin-rpc/pkg/upload"
)

func main() {
    // Create configuration
    config := upload.DefaultConfig()
    config.UploadDir = "./uploads"
    config.MaxFileSize = 10 * 1024 * 1024 // 10MB
    
    // Create uploader
    uploader := upload.NewUploader(config)
    
    // Upload file
    result := uploader.Upload(fileHeader)
    if result.Success {
        fmt.Printf("File saved as: %s\n", result.SavedFilename)
    } else {
        fmt.Printf("Upload failed: %v\n", result.Error)
    }
}
```

### Upload from Bytes

```go
data := []byte("file content")
result := uploader.UploadFromBytes("example.txt", data)
if result.Success {
    fmt.Printf("File saved as: %s\n", result.SavedFilename)
}
```

### Multiple File Upload

```go
results := uploader.UploadMultiple(fileHeaders)
for _, result := range results {
    if result.Success {
        fmt.Printf("File saved as: %s\n", result.SavedFilename)
    }
}
```

## Integration with Gin

### Basic Setup

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/alldev-run/golang-gin-rpc/pkg/upload"
)

func main() {
    r := gin.Default()
    
    // Create upload middleware
    config := upload.DefaultConfig()
    middleware := upload.NewGinMiddleware(config)
    
    // Register upload routes
    api := r.Group("/api")
    middleware.RegisterRoutes(api)
    
    r.Run(":8080")
}
```

### Manual Route Registration

```go
middleware := upload.NewGinMiddleware(config)

api := r.Group("/api")
api.Use(middleware.CORSMiddleware())
api.POST("/upload", middleware.UploadHandler)
api.POST("/upload/single", middleware.SingleUploadHandler)
api.DELETE("/delete", middleware.DeleteHandler)
```

### Custom Route Prefix

```go
middleware := upload.NewGinMiddleware(config)

files := r.Group("/files")
middleware.RegisterRoutes(files)
// Routes: /files/upload, /files/upload/single, /files/delete
```

## Naming Strategies

### UUID Naming (Default)

```go
config.NamingStrategy = "uuid"
```

Generates filenames like: `550e8400-e29b-41d4-a716-446655440000.jpg`

### Timestamp Naming

```go
config.NamingStrategy = "timestamp"
```

Generates filenames like: `20240506143025.jpg`

### Original Name

```go
config.NamingStrategy = "original"
```

Preserves the original filename: `my-photo.jpg`

### Custom Template

```go
config.NamingStrategy = "custom"
config.CustomNameTemplate = "{date}_{original}_{random}"
```

Supported placeholders:
- `{uuid}` - UUID v4
- `{timestamp}` - Full timestamp (YYYYMMDDHHMMSS)
- `{date}` - Date only (YYYYMMDD)
- `{original}` - Original filename without extension
- `{random}` - Random number based on nanoseconds

Example output: `20240506_my-photo_1715000000000000000.jpg`

## File Validation

### Extension Validation

```go
validator := upload.NewValidator(config)
if validator.IsAllowedExtension("test.jpg") {
    // File extension is allowed
}
```

### MIME Type Validation

```go
if validator.IsAllowedMimeType("image/jpeg") {
    // MIME type is allowed
}
```

### File Size Validation

The validator automatically checks file size against `max_file_size` configuration.

### Custom Validation

```go
result := uploader.Upload(fileHeader)
if !result.Success {
    if validationErr, ok := result.Error.(*upload.ValidationError); ok {
        fmt.Printf("Field: %s, Message: %s\n", validationErr.Field, validationErr.Message)
    }
}
```

## CORS Configuration

### Basic CORS Setup

```go
config := upload.DefaultConfig()
config.EnableCORS = true
config.CORS = upload.CORSConfig{
    AllowedOrigins:   []string{"https://example.com"},
    AllowedMethods:   []string{"POST", "OPTIONS"},
    AllowedHeaders:   []string{"Content-Type", "Authorization"},
    AllowCredentials: true,
    MaxAge:           86400,
}
```

### CORS for Local Development

```go
config.CORS = upload.CORSConfig{
    AllowedOrigins:   []string{"*"},
    AllowedMethods:   []string{"GET", "POST", "OPTIONS", "DELETE"},
    AllowedHeaders:   []string{"*"},
    AllowCredentials: false,
    MaxAge:           86400,
}
```

### CORS for Production

```go
config.CORS = upload.CORSConfig{
    AllowedOrigins:   []string{"https://yourdomain.com", "https://api.yourdomain.com"},
    AllowedMethods:   []string{"POST", "OPTIONS", "DELETE"},
    AllowedHeaders:   []string{"Content-Type", "Authorization"},
    AllowCredentials: true,
    MaxAge:           3600,
}
```

## API Endpoints

### POST /api/upload

Upload multiple files.

**Request:**
- Method: POST
- Content-Type: multipart/form-data
- Form field: `files` (array)

**Response (Success):**
```json
{
  "success": true,
  "message": "2 files uploaded successfully",
  "data": [
    {
      "original_filename": "example.jpg",
      "saved_filename": "550e8400-e29b-41d4-a716-446655440000.jpg",
      "file_path": "./uploads/550e8400-e29b-41d4-a716-446655440000.jpg",
      "file_size": 1024000,
      "mime_type": "image/jpeg",
      "success": true
    }
  ]
}
```

**Response (Partial Success):**
```json
{
  "success": false,
  "message": "1 files uploaded successfully, 1 files failed",
  "data": [...],
  "errors": ["file_extension: file extension .exe is not allowed"]
}
```

### POST /api/upload/single

Upload a single file.

**Request:**
- Method: POST
- Content-Type: multipart/form-data
- Form field: `file`

**Response:**
```json
{
  "success": true,
  "message": "File uploaded successfully",
  "data": {
    "original_filename": "example.jpg",
    "saved_filename": "550e8400-e29b-41d4-a716-446655440000.jpg",
    "file_path": "./uploads/550e8400-e29b-41d4-a716-446655440000.jpg",
    "file_size": 1024000,
    "mime_type": "image/jpeg",
    "success": true
  }
}
```

### DELETE /api/delete?filename={filename}

Delete a file.

**Request:**
- Method: DELETE
- Query parameter: `filename`

**Response:**
```json
{
  "success": true,
  "message": "File deleted successfully"
}
```

## Security Considerations

### File Size Limits

Always set appropriate file size limits to prevent DoS attacks:

```yaml
upload:
  max_file_size: 10485760  # 10MB
```

### File Type Validation

Validate both file extensions and MIME types:

```yaml
upload:
  allowed_extensions:
    - ".jpg"
    - ".png"
    - ".pdf"
  allowed_mime_types:
    - "image/jpeg"
    - "image/png"
    - "application/pdf"
```

### Upload Directory Security

- Ensure upload directory is not directly web-accessible
- Use proper file permissions (0755 for directories, 0644 for files)
- Consider storing uploads outside the web root
- Implement access control for file downloads

### CORS Configuration

Configure CORS carefully for production:

```yaml
upload:
  enable_cors: true
  cors:
    allowed_origins:
      - "https://yourdomain.com"  # Specific origins only
    allow_credentials: true      # Only if needed
```

### File Naming

Use UUID or timestamp naming to prevent:
- Filename collisions
- Directory traversal attacks
- Information disclosure

```yaml
upload:
  naming_strategy: "uuid"  # Recommended
  preserve_extension: true
```

### File Content Validation

For sensitive applications, implement additional validation:
- Scan uploaded files for malware
- Validate file content matches declared MIME type
- Process files in a sandboxed environment

## Troubleshooting

### Common Issues

#### File Upload Fails with "File extension not allowed"

**Solution:** Add the file extension to `allowed_extensions` in configuration.

#### File Upload Fails with "File size exceeds maximum"

**Solution:** Increase `max_file_size` in configuration or compress files before upload.

#### CORS Errors

**Solution:** Check CORS configuration and ensure the origin is in `allowed_origins`.

#### Directory Not Created

**Solution:** Ensure `auto_create_dir` is set to `true` or create the directory manually with proper permissions.

#### File Overwrite Issues

**Solution:** Set `enable_overwrite: true` if you want to allow overwriting existing files.

### Debug Mode

Enable debug logging to troubleshoot issues:

```yaml
logger:
  level: "debug"
```

### Testing Upload Functionality

Use the test files provided in `pkg/upload/example_test.go`:

```bash
go test ./pkg/upload/... -v
```

## Best Practices

1. **Always validate file types** - Use both extension and MIME type validation
2. **Set reasonable file size limits** - Prevent DoS attacks
3. **Use UUID naming** - Prevent collisions and security issues
4. **Secure upload directories** - Store outside web root when possible
5. **Configure CORS properly** - Use specific origins in production
6. **Monitor upload activity** - Log and track upload attempts
7. **Implement rate limiting** - Prevent abuse of upload endpoints
8. **Regular cleanup** - Remove old or unused uploaded files

## Advanced Usage

### Custom Validation

```go
func customValidator(fileHeader *multipart.FileHeader) error {
    // Add custom validation logic
    return nil
}
```

### Progress Tracking

For large file uploads, implement progress tracking:

```go
type ProgressWriter struct {
    Total   int64
    Written int64
}

func (pw *ProgressWriter) Write(p []byte) (int, error) {
    n := len(p)
    pw.Written += int64(n)
    // Emit progress event
    return n, nil
}
```

### Storage Backends

Extend the uploader to support different storage backends (S3, Azure Blob, etc.):

```go
type StorageBackend interface {
    Store(filename string, data []byte) (string, error)
    Delete(filename string) error
    Exists(filename string) bool
}
```

## Support

For issues, questions, or contributions, please refer to the main project documentation.
