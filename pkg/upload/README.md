# Upload Package

The upload package provides a comprehensive file upload functionality for the golang-gin-rpc framework. It supports configurable file validation, naming strategies, CORS, and integration with both standard HTTP (net/http) and Gin framework.

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

- **File Validation**: Validate file types by extension and MIME type, and file size limits
- **Naming Strategies**: Multiple naming strategies including UUID, timestamp, original name, and custom templates
- **CORS Support**: Configurable CORS settings for cross-origin requests
- **Authentication**: Basic HTTP authentication support with username/password
- **Short-Lived Download Token**: Signed token issuance with framework `jwtx` component
- **Gin Integration**: Built-in middleware for seamless integration with Gin framework
- **net/http Integration**: Native HTTP handlers for standard Go HTTP servers
- **File Browsing**: List and browse uploaded files
- **File Download**: Download files with proper MIME type detection
- **Standalone Server**: Option to run as a standalone upload server
- **Auto Directory Creation**: Automatically creates upload directories
- **File Overwrite Control**: Configurable file overwrite behavior

## Installation

```bash
go get github.com/alldev-run/golang-gin-rpc/pkg/upload
```

## Configuration

### Default Configuration

```go
config := upload.DefaultConfig()
```

### Custom Configuration

```go
config := &upload.Config{
    UploadDir:         "./uploads",
    MaxFileSize:       10 * 1024 * 1024, // 10MB
    AllowedExtensions: []string{".jpg", ".png", ".pdf"},
    AllowedMimeTypes: []string{
        "image/jpeg",
        "image/png",
        "application/pdf",
    },
    NamingStrategy:    "uuid",
    PreserveExtension: true,
    EnableCORS:        true,
    CORS: upload.CORSConfig{
        AllowedOrigins:   []string{"*"},
        AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
        AllowedHeaders:   []string{"Origin", "Content-Type", "Accept"},
        AllowCredentials: false,
        MaxAge:           86400,
    },
    Port:          8081,
    EnableServer:  false,
    AutoCreateDir: true,
    EnableOverwrite: false,
}
```

## Usage

### Basic File Upload

```go
package main

import (
    "fmt"
    "github.com/alldev-run/golang-gin-rpc/pkg/upload"
)

func main() {
    config := upload.DefaultConfig()
    uploader := upload.NewUploader(config)
    
    // Upload from multipart file header
    result := uploader.Upload(fileHeader)
    if result.Success {
        fmt.Printf("File saved as: %s\n", result.SavedFilename)
    } else {
        fmt.Printf("Upload failed: %v\n", result.Error)
    }
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

### Upload from Bytes

```go
data := []byte("file content")
result := uploader.UploadFromBytes("example.txt", data)
if result.Success {
    fmt.Printf("File saved as: %s\n", result.SavedFilename)
}
```

## Naming Strategies

### UUID Naming (Default)

```go
config.NamingStrategy = "uuid"
// Generates: 550e8400-e29b-41d4-a716-446655440000.jpg
```

### Timestamp Naming

```go
config.NamingStrategy = "timestamp"
// Generates: 20240506143025.jpg
```

### Original Name

```go
config.NamingStrategy = "original"
// Preserves: original-filename.jpg
```

### Custom Template

```go
config.NamingStrategy = "custom"
config.CustomNameTemplate = "{date}_{original}_{random}"
// Generates: 20240506_original-filename_1715000000000000000.jpg
```

Supported placeholders:
- `{uuid}` - UUID v4
- `{timestamp}` - Full timestamp (YYYYMMDDHHMMSS)
- `{date}` - Date only (YYYYMMDD)
- `{original}` - Original filename without extension
- `{random}` - Random number based on nanoseconds

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

## Authentication

The upload package supports Basic HTTP authentication for protecting upload endpoints.

### Enable Authentication

```go
config := upload.DefaultConfig()
config.EnableAuth = true
config.AuthUsername = "admin"
config.AuthPassword = "securepassword"

handler := upload.NewHandler(config)
```

### Public Token Methods (for external frameworks)

You can generate and verify short-lived download tokens directly in code (without HTTP `/token` endpoint). The implementation reuses framework `pkg/auth/jwtx`:

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

### Using Authentication with cURL

```bash
# Upload with authentication
curl -X POST -u admin:securepassword -F "files=@test.jpg" http://localhost:8081/upload

# List files with authentication
curl -u admin:securepassword http://localhost:8081/list

# Download file with authentication
curl -u admin:securepassword "http://localhost:8081/download?filename=test.jpg" -o test.jpg

# Issue short-lived token (requires authentication)
curl -u admin:securepassword "http://localhost:8081/token?filename=test.jpg"

# Download file with token (recommended: Authorization Bearer)
curl -H "Authorization: Bearer <TOKEN>" "http://localhost:8081/download?filename=test.jpg" -o test.jpg

# Backward-compatible query token usage
curl "http://localhost:8081/download?filename=test.jpg&token=<TOKEN>" -o test.jpg
```

## Configuration in YAML

Add to your `config.yaml`:

```yaml
upload:
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
  naming_strategy: "uuid"
  custom_name_template: "{date}_{original}"
  preserve_extension: true
  enable_cors: true
  cors:
    allowed_origins:
      - "*"
    allowed_methods:
      - "GET"
      - "POST"
      - "OPTIONS"
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
  enable_auth: true
  auth_username: "admin"
  auth_password: "securepassword"
  token_secret: ""        # Optional. Falls back to auth_password when empty
  token_ttl_seconds: 300   # Token validity in seconds
```

## API Endpoints

### POST /api/upload

Upload multiple files.

**Request:**
- Method: POST
- Content-Type: multipart/form-data
- Form field: `files` (array)

**Response:**
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
  -u admin:securepassword \
  http://localhost:8081/upload/stream
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

## Error Handling

The package provides detailed error information:

```go
result := uploader.Upload(fileHeader)
if !result.Success {
    if validationErr, ok := result.Error.(*upload.ValidationError); ok {
        fmt.Printf("Field: %s, Message: %s\n", validationErr.Field, validationErr.Message)
    } else {
        fmt.Printf("Error: %v\n", result.Error)
    }
}
```

## File Validation

### Check Allowed Extension

```go
validator := upload.NewValidator(config)
if validator.IsAllowedExtension("example.jpg") {
    // File extension is allowed
}
```

### Check Allowed MIME Type

```go
if validator.IsAllowedMimeType("image/jpeg") {
    // MIME type is allowed
}
```

### Get Max File Size

```go
maxSize := validator.GetMaxFileSize()
fmt.Printf("Max file size: %d bytes\n", maxSize)
```

## Security Considerations

1. **File Size Limits**: Always set appropriate file size limits to prevent DoS attacks
2. **File Type Validation**: Validate both file extensions and MIME types
3. **Upload Directory**: Ensure upload directory is not web-accessible or use proper access controls
4. **CORS Configuration**: Configure CORS carefully to prevent unauthorized access
5. **File Naming**: Use UUID or timestamp naming to prevent filename collisions and directory traversal attacks

## License

This package is part of the golang-gin-rpc framework.
