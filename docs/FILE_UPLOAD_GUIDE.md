# File Upload Guide

The golang-gin-rpc framework provides a comprehensive file upload functionality with configurable validation, naming strategies, CORS support, and seamless integration with both standard HTTP (net/http) and Gin framework.

## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Configuration](#configuration)
- [Quick Start](#quick-start)
- [Gin Framework Integration](#gin-framework-integration)
- [net/http Integration](#nethttp-integration)
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
- Native net/http handler support
- Comprehensive error handling

## Package Structure

```
pkg/upload/
├── config.go           # Configuration structures
├── namer.go            # File naming strategies
├── validator.go        # File validation
├── upload.go           # Core upload functionality
├── handler.go          # HTTP handlers (net/http compatible)
├── example_test.go     # Core package tests
└── gin/                # Gin framework integration
    └── gin.go         # Gin middleware and handlers
```

## Features

### Core Features

- **File Validation**: Validate files by extension and MIME type
- **Size Limits**: Configure maximum file size to prevent DoS attacks
- **Naming Strategies**: Multiple options for auto-naming uploaded files
- **CORS Support**: Full CORS configuration for cross-origin requests
- **Authentication**: Basic HTTP authentication support with username/password
- **Gin Integration**: Built-in middleware for seamless Gin integration
- **net/http Integration**: Native HTTP handlers for standard Go HTTP servers
- **File Browsing**: List and browse uploaded files
- **File Download**: Download files with proper MIME type detection
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
  enable_auth: false
  auth_username: "admin"
  auth_password: "password"
  token_secret: ""
  token_ttl_seconds: 300
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
| `enable_server` | bool | `false` | Enable standalone upload server |
| `auto_create_dir` | bool | `true` | Automatically create upload directory |
| `enable_overwrite` | bool | `false` | Allow overwriting existing files |
| `enable_auth` | bool | `false` | Enable authentication |
| `auth_username` | string | `""` | Authentication username |
| `auth_password` | string | `""` | Authentication password |
| `token_secret` | string | `""` | Token signing secret (fallback to `auth_password`) |
| `token_ttl_seconds` | int64 | `300` | Short-lived token validity in seconds |

## Authentication

The upload package supports Basic HTTP authentication to protect upload endpoints.
Download token generation and verification reuse framework `pkg/auth/jwtx`.

### Enabling Authentication

```yaml
upload:
  enable_auth: true
  auth_username: "admin"
  auth_password: "secure_password"
```

### Using Authentication

When authentication is enabled, upload/list/delete endpoints require Basic HTTP authentication. Download can use either Basic Auth or a valid short-lived Bearer token.

#### Using cURL

```bash
# Upload with authentication
curl -X POST -u admin:secure_password -F "files=@test.jpg" http://localhost:8081/upload

# List files with authentication
curl -u admin:secure_password http://localhost:8081/list

# Download file with authentication
curl -u admin:secure_password "http://localhost:8081/download?filename=test.jpg" -o test.jpg

# Issue short-lived token (requires authentication)
curl -u admin:secure_password "http://localhost:8081/token?filename=test.jpg"

# Download with bearer token (recommended)
curl -H "Authorization: Bearer <TOKEN>" "http://localhost:8081/download?filename=test.jpg" -o test.jpg

# Delete file with authentication
curl -X DELETE -u admin:secure_password "http://localhost:8081/delete?filename=test.jpg"
```

#### Using in Code

```go
config := upload.DefaultConfig()
config.EnableAuth = true
config.AuthUsername = "admin"
config.AuthPassword = "secure_password"

handler := upload.NewHandler(config)
// All handlers will now require authentication
```

### Public Token Methods (for external frameworks)

```go
handler := upload.NewHandler(config)

token, expiresAt, err := handler.GenerateDownloadToken("test.jpg")
if err != nil {
    // handle error
}

ok := handler.VerifyDownloadToken("test.jpg", expiresAt, token)
if !ok {
    // handle invalid token
}
```

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

## Gin Framework Integration

### Basic Setup

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/alldev-run/golang-gin-rpc/pkg/upload"
    "github.com/alldev-run/golang-gin-rpc/pkg/upload/gin"
)

func main() {
    r := gin.Default()
    
    config := upload.DefaultConfig()
    middleware := gin.NewMiddleware(config)
    
    // Register upload routes
    api := r.Group("/api")
    middleware.RegisterRoutes(api)
    
    r.Run(":8080")
}
```

### Manual Route Registration

```go
middleware := gin.NewMiddleware(config)

api := r.Group("/api")
api.Use(middleware.CORSMiddleware())
api.POST("/upload", middleware.UploadHandler)
api.POST("/upload/single", middleware.SingleUploadHandler)
api.POST("/upload/stream", middleware.UploadStreamHandler)
api.GET("/list", middleware.ListHandler)
api.GET("/token", middleware.IssueTokenHandler)
api.GET("/download", middleware.DownloadHandler)
api.DELETE("/delete", middleware.DeleteHandler)
```

## net/http Integration

### Basic Setup

```go
package main

import (
    "net/http"
    "github.com/alldev-run/golang-gin-rpc/pkg/upload"
)

func main() {
    config := upload.DefaultConfig()
    handler := upload.NewHandler(config)
    
    http.HandleFunc("/upload", handler.UploadHandler)
    http.HandleFunc("/upload/single", handler.SingleUploadHandler)
    http.HandleFunc("/upload/stream", handler.UploadStreamHandler)
    http.HandleFunc("/list", handler.ListHandler)
    http.HandleFunc("/token", handler.IssueTokenHandler)
    http.HandleFunc("/download", handler.DownloadHandler)
    http.HandleFunc("/delete", handler.DeleteHandler)
    
    http.ListenAndServe(":8080", nil)
}
```

### Standalone Server

```go
config := upload.DefaultConfig()
config.EnableServer = true
config.Port = 8081

handler := upload.NewHandler(config)
go handler.StartServer(":8081")
```

### Custom Route Prefix

```go
middleware := gin.NewMiddleware(config)

files := r.Group("/files")
middleware.RegisterRoutes(files)
// Routes: /files/upload, /files/upload/single, /files/upload/stream, /files/list, /files/token, /files/download, /files/delete
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

### POST /api/upload/stream?filename={filename}

Upload a single file using raw binary stream.

**Request:**
- Method: POST
- Content-Type: `application/octet-stream`
- Query parameter: `filename` (or header `X-Filename`)
- Body: raw binary bytes

**cURL example:**
```bash
curl -X POST \
  -H "Content-Type: application/octet-stream" \
  -H "X-Filename: demo.txt" \
  --data-binary @demo.txt \
  -u admin:secure_password \
  http://localhost:8081/upload/stream
```

### GET /api/list

List all uploaded files.

**Request:**
- Method: GET

**Response:**
```json
{
  "success": true,
  "message": "Files listed successfully",
  "data": [
    {
      "filename": "550e8400-e29b-41d4-a716-446655440000.jpg",
      "file_path": "550e8400-e29b-41d4-a716-446655440000.jpg",
      "file_size": 1024000,
      "mod_time": "2024-05-06T14:30:25Z",
      "is_directory": false
    }
  ]
}
```

### GET /api/download?filename={filename}

Download a file.

**Request:**
- Method: GET
- Query parameter: `filename`
- Auth options:
  - `Authorization: Bearer <token>`
  - or Basic Auth (`-u username:password`)
  - or query token `token=<TOKEN>` (compatibility)

**Response:**
- Content-Type: Detected MIME type (e.g., image/jpeg)
- Content-Disposition: attachment; filename={filename}
- File content in response body

### GET /api/token?filename={filename}

Issue short-lived download token.

**Request:**
- Method: GET
- Requires Basic Auth
- Query parameter: `filename`

**Response:**
```json
{
  "success": true,
  "message": "Token issued successfully",
  "data": {
    "filename": "test.jpg",
    "token": "<SIGNED_TOKEN>",
    "expires": 1715000000,
    "ttl": 300
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
- Use proper file permissions (0755 for directories, 0600 for files)
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
