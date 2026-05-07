# File Upload Guide

The golang-gin-rpc framework provides a core file upload component with configurable validation and naming strategies. HTTP interface layer (handlers, CORS, authentication) is handled by the API Gateway layer.

## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Configuration](#configuration)
- [Quick Start](#quick-start)
- [Bootstrap Integration](#bootstrap-integration)
- [Naming Strategies](#naming-strategies)
- [File Validation](#file-validation)
- [Security Considerations](#security-considerations)
- [Troubleshooting](#troubleshooting)

## Overview

The upload package (`pkg/upload`) provides a flexible and secure file upload core component for the golang-gin-rpc framework. It handles:

- File validation by extension and MIME type
- Configurable file size limits
- Multiple naming strategies (UUID, timestamp, custom)
- Path traversal protection
- Auto directory creation

HTTP handlers, CORS, authentication, and other interface concerns are implemented in the API Gateway layer.

## Package Structure

```
pkg/upload/
├── config.go           # Configuration structures
├── namer.go            # File naming strategies
├── validator.go        # File validation
├── upload.go           # Core upload functionality
└── example_test.go     # Core package tests
```

## Features

### Core Features

- **File Validation**: Validate files by extension and MIME type
- **Size Limits**: Configure maximum file size to prevent DoS attacks
- **Naming Strategies**: Multiple options for auto-naming uploaded files
- **Path Traversal Protection**: Prevents directory traversal attacks
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
  auto_create_dir: true
  enable_overwrite: false
```

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | `false` | Enable/disable upload functionality |
| `upload_dir` | string | `"./uploads"` | Directory to store uploaded files |
| `max_file_size` | int64 | `10485760` | Maximum file size in bytes (10MB) |
| `allowed_extensions` | []string | See default | Allowed file extensions |
| `allowed_mime_types` | []string | See default | Allowed MIME types |
| `naming_strategy` | string | `"uuid"` | File naming strategy |
| `custom_name_template` | string | `"{date}_{original}_{random}"` | Custom naming template |
| `preserve_extension` | bool | `true` | Preserve original file extension |
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

## Bootstrap Integration

The upload package integrates with the framework's bootstrap system for automatic initialization.

### Enable in Configuration

```yaml
upload:
  enabled: true
  upload_dir: "./uploads"
  # ... other config
```

### Get Uploader from Bootstrap

```go
import (
    "github.com/alldev-run/golang-gin-rpc/internal/bootstrap"
    "github.com/alldev-run/golang-gin-rpc/pkg/bootstrap"
)

// Initialize bootstrap
boot := bootstrap.New()
err := boot.InitializeAll()
if err != nil {
    log.Fatal(err)
}

// Get uploader
uploader, err := pkgbootstrap.GetUploader(boot)
if err != nil {
    log.Fatal(err)
}
```

### Create HTTP Handlers in API Gateway Layer

```go
import (
    "github.com/gin-gonic/gin"
    "github.com/alldev-run/golang-gin-rpc/pkg/upload"
)

func uploadHandler(c *gin.Context, uploader *upload.Uploader) {
    file, err := c.FormFile("file")
    if err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    
    result := uploader.Upload(file)
    if !result.Success {
        c.JSON(400, gin.H{"error": result.Error.Error()})
        return
    }
    
    c.JSON(200, gin.H{
        "filename": result.SavedFilename,
        "path": result.FilePath,
        "size": result.FileSize,
    })
}
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
- Use proper file permissions (0755 for directories, 0600 for files)
- Consider storing uploads outside the web root
- Implement access control for file downloads

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

### HTTP Interface Security

HTTP handlers, CORS, authentication, and other interface concerns should be implemented in the API Gateway layer with appropriate security measures.

## Troubleshooting

### Common Issues

#### File Upload Fails with "File extension not allowed"

**Solution:** Add the file extension to `allowed_extensions` in configuration.

#### File Upload Fails with "File size exceeds maximum"

**Solution:** Increase `max_file_size` in configuration or compress files before upload.

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
5. **Monitor upload activity** - Log and track upload attempts
6. **Implement rate limiting in API Gateway** - Prevent abuse of upload endpoints
7. **Regular cleanup** - Remove old or unused uploaded files
8. **Implement authentication in API Gateway** - Protect upload endpoints with proper authentication

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
