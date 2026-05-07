package upload

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Namer defines the interface for file naming strategies
type Namer interface {
	Generate(originalName string) string
}

// UUIDNamer generates UUID-based filenames
type UUIDNamer struct {
	preserveExtension bool
}

func NewUUIDNamer(preserveExtension bool) *UUIDNamer {
	return &UUIDNamer{preserveExtension: preserveExtension}
}

func (n *UUIDNamer) Generate(originalName string) string {
	ext := ""
	if n.preserveExtension {
		ext = filepath.Ext(originalName)
	}
	return uuid.New().String() + ext
}

// TimestampNamer generates timestamp-based filenames
type TimestampNamer struct {
	preserveExtension bool
	format           string
}

func NewTimestampNamer(preserveExtension bool, format string) *TimestampNamer {
	if format == "" {
		format = "20060102150405"
	}
	return &TimestampNamer{preserveExtension: preserveExtension, format: format}
}

func (n *TimestampNamer) Generate(originalName string) string {
	ext := ""
	if n.preserveExtension {
		ext = filepath.Ext(originalName)
	}
	timestamp := time.Now().Format(n.format)
	return timestamp + ext
}

// OriginalNamer preserves the original filename
type OriginalNamer struct{}

func NewOriginalNamer() *OriginalNamer {
	return &OriginalNamer{}
}

func (n *OriginalNamer) Generate(originalName string) string {
	return originalName
}

// CustomNamer generates filenames based on a custom template
type CustomNamer struct {
	template         string
	preserveExtension bool
}

func NewCustomNamer(template string, preserveExtension bool) *CustomNamer {
	return &CustomNamer{template: template, preserveExtension: preserveExtension}
}

func (n *CustomNamer) Generate(originalName string) string {
	ext := ""
	if n.preserveExtension {
		ext = filepath.Ext(originalName)
	}

	nameWithoutExt := strings.TrimSuffix(originalName, filepath.Ext(originalName))
	result := n.template

	// Replace placeholders
	result = strings.ReplaceAll(result, "{uuid}", uuid.New().String())
	result = strings.ReplaceAll(result, "{timestamp}", time.Now().Format("20060102150405"))
	result = strings.ReplaceAll(result, "{date}", time.Now().Format("20060102"))
	result = strings.ReplaceAll(result, "{original}", nameWithoutExt)
	result = strings.ReplaceAll(result, "{random}", fmt.Sprintf("%d", time.Now().UnixNano()))

	return result + ext
}

// GetNamer returns the appropriate namer based on strategy
func GetNamer(strategy string, customTemplate string, preserveExtension bool) Namer {
	switch strategy {
	case "uuid":
		return NewUUIDNamer(preserveExtension)
	case "timestamp":
		return NewTimestampNamer(preserveExtension, "")
	case "original":
		return NewOriginalNamer()
	case "custom":
		return NewCustomNamer(customTemplate, preserveExtension)
	default:
		return NewUUIDNamer(preserveExtension)
	}
}
