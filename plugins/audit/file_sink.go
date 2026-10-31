package audit

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"
)

// FileSink persists audit events as JSON lines.
type FileSink struct {
	mu sync.Mutex
	w  io.Writer
	c  io.Closer
}

// NewFileSink creates a file-backed sink in append mode.
func NewFileSink(path string) (*FileSink, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	return &FileSink{w: f, c: f}, nil
}

// NewWriterFileSink creates a sink from an existing writer.
func NewWriterFileSink(w io.Writer) *FileSink {
	return &FileSink{w: w}
}

// Write writes one JSON line for each event.
func (s *FileSink) Write(ctx context.Context, event Event) error {
	start := time.Now()
	raw, err := json.Marshal(event)
	if err != nil {
		recordAuditWriteMetric("file", "error", time.Since(start))
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	_, err = s.w.Write(append(raw, '\n'))
	if err != nil {
		recordAuditWriteMetric("file", "error", time.Since(start))
		return err
	}
	recordAuditWriteMetric("file", "success", time.Since(start))
	return err
}

// Close closes underlying file when available.
func (s *FileSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.c != nil {
		return s.c.Close()
	}
	return nil
}
