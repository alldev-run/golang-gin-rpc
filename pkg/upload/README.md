# Upload Package

The upload package provides core file upload functionality for the golang-gin-rpc framework. It handles file validation, storage, and naming strategies as a pure component without HTTP interface logic. HTTP handlers, CORS, authentication, and other interface concerns are handled by the API Gateway layer.

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

- **File Validation**: Validate file types by extension and MIME type, and file size limits
- **Naming Strategies**: Multiple naming strategies including UUID, timestamp, original name, and custom templates
- **Auto Directory Creation**: Automatically creates upload directories
- **File Overwrite Control**: Configurable file overwrite behavior
- **Path Traversal Protection**: Prevents directory traversal attacks

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
    AutoCreateDir:     true,
    EnableOverwrite:   false,
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

### Delete File

```go
err := uploader.Delete("filename.jpg")
if err != nil {
    fmt.Printf("Delete failed: %v\n", err)
}
```

### Check File Existence

```go
if uploader.Exists("filename.jpg") {
    fmt.Println("File exists")
}
```

### Serve File

```go
err := uploader.ServeFile("filename.jpg", responseWriter)
if err != nil {
    fmt.Printf("Serve failed: %v\n", err)
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

## Integration with Bootstrap

The upload package integrates with the framework's bootstrap system:

```go
import (
    "github.com/alldev-run/golang-gin-rpc/pkg/bootstrap"
)

// Get uploader from bootstrap
uploader, err := bootstrap.GetUploader(boot)
if err != nil {
    // handle error
}

// Use uploader in your HTTP handlers
func uploadHandler(c *gin.Context) {
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
    })
}
```

## Configuration in YAML

Add to your `config.yaml`:

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
  naming_strategy: "uuid"
  custom_name_template: "{date}_{original}"
  preserve_extension: true
  auto_create_dir: true
  enable_overwrite: false
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
4. **File Naming**: Use UUID or timestamp naming to prevent filename collisions and directory traversal attacks
5. **HTTP Interface**: HTTP handlers, CORS, authentication should be implemented in the API Gateway layer

## License

This package is part of the golang-gin-rpc framework.
