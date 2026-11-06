package audit

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrAsyncSinkClosed     = errors.New("audit async sink closed")
	ErrAsyncSinkBufferFull = errors.New("audit async sink buffer full")
)

// AsyncSinkConfig controls async sink behavior.
type AsyncSinkConfig struct {
	BufferSize int
	Workers    int
	DropOnFull bool
}

// DefaultAsyncSinkConfig returns sane defaults for production usage.
func DefaultAsyncSinkConfig() AsyncSinkConfig {
	return AsyncSinkConfig{
		BufferSize: 1024,
		Workers:    1,
		DropOnFull: false,
	}
}

// AsyncSink decouples request path from sink I/O latency.
type AsyncSink struct {
	target Sink
	cfg    AsyncSinkConfig

	ch     chan Event
	closed chan struct{}

	once sync.Once
	wg   sync.WaitGroup
}

// NewAsyncSink creates an async wrapper around a target sink.
func NewAsyncSink(target Sink, cfg AsyncSinkConfig) *AsyncSink {
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = DefaultAsyncSinkConfig().BufferSize
	}
	if cfg.Workers <= 0 {
		cfg.Workers = DefaultAsyncSinkConfig().Workers
	}

	s := &AsyncSink{
		target: target,
		cfg:    cfg,
		ch:     make(chan Event, cfg.BufferSize),
		closed: make(chan struct{}),
	}

	for i := 0; i < cfg.Workers; i++ {
		s.wg.Add(1)
		go s.worker()
	}

	return s
}

// Write enqueues an event for async processing.
func (s *AsyncSink) Write(ctx context.Context, event Event) error {
	select {
	case <-s.closed:
		recordAuditDropMetric("async", "closed")
		return ErrAsyncSinkClosed
	default:
	}

	if s.cfg.DropOnFull {
		select {
		case s.ch <- event:
			return nil
		default:
			recordAuditDropMetric("async", "buffer_full")
			return ErrAsyncSinkBufferFull
		}
	}

	select {
	case <-s.closed:
		recordAuditDropMetric("async", "closed")
		return ErrAsyncSinkClosed
	case <-ctx.Done():
		recordAuditDropMetric("async", "ctx_done")
		return ctx.Err()
	case s.ch <- event:
		return nil
	}
}

// Close gracefully stops workers and drains queued events.
func (s *AsyncSink) Close() {
	s.once.Do(func() {
		close(s.closed)
		close(s.ch)
		s.wg.Wait()
	})
}

func (s *AsyncSink) worker() {
	defer s.wg.Done()
	for event := range s.ch {
		start := time.Now()
		if err := s.target.Write(context.Background(), event); err != nil {
			recordAuditWriteMetric("async", "error", time.Since(start))
			continue
		}
		recordAuditWriteMetric("async", "success", time.Since(start))
	}
}
