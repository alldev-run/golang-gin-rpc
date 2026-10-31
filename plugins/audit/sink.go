package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/logger"
)

// Sink is the output target for audit events.
type Sink interface {
	Write(ctx context.Context, event Event) error
}

// MultiSink fan-outs a single event to multiple sinks.
type MultiSink struct {
	sinks []Sink
}

// NewMultiSink creates a combined sink.
func NewMultiSink(sinks ...Sink) *MultiSink {
	return &MultiSink{sinks: sinks}
}

// Write writes event to all sinks and returns the first error.
func (m *MultiSink) Write(ctx context.Context, event Event) error {
	for _, sink := range m.sinks {
		if err := sink.Write(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

// LogSink writes audit events via framework logger.
type LogSink struct{}

// Write writes an event to structured log.
func (LogSink) Write(ctx context.Context, event Event) error {
	start := time.Now()
	raw, err := json.Marshal(event)
	if err != nil {
		recordAuditWriteMetric("log", "error", time.Since(start))
		return err
	}
	logger.Info("audit_event", logger.String("payload", string(raw)))
	recordAuditWriteMetric("log", "success", time.Since(start))
	return nil
}
