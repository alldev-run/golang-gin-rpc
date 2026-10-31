// Package alert provides alerting and notification functionality
package alert

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Level represents alert severity level
type Level string

const (
	// LevelDebug debug level
	LevelDebug Level = "debug"
	// LevelInfo info level
	LevelInfo Level = "info"
	// LevelWarning warning level
	LevelWarning Level = "warning"
	// LevelError error level
	LevelError Level = "error"
	// LevelCritical critical level
	LevelCritical Level = "critical"
)

// Alert represents an alert notification
type Alert struct {
	ID        string                 `json:"id"`
	Level     Level                  `json:"level"`
	Title     string                 `json:"title"`
	Message   string                 `json:"message"`
	Source    string                 `json:"source"`
	Tags      map[string]string      `json:"tags"`
	Metadata  map[string]interface{} `json:"metadata"`
	Timestamp time.Time              `json:"timestamp"`
}

// Channel represents an alert notification channel
type Channel interface {
	Name() string
	Send(ctx context.Context, alert Alert) error
}

// Config holds alert manager configuration
type Config struct {
	Enabled       bool          `yaml:"enabled" json:"enabled"`
	BufferSize    int           `yaml:"buffer_size" json:"buffer_size"`
	FlushInterval time.Duration `yaml:"flush_interval" json:"flush_interval"`
	Channels      []ChannelConfig `yaml:"channels" json:"channels"`
}

// ChannelConfig represents channel configuration
type ChannelConfig struct {
	Type    string                 `yaml:"type" json:"type"`
	Name    string                 `yaml:"name" json:"name"`
	Enabled bool                   `yaml:"enabled" json:"enabled"`
	Config  map[string]interface{} `yaml:"config" json:"config"`
}

// Manager manages alert notifications
type Manager struct {
	config    Config
	channels  map[string]Channel
	buffer    chan Alert
	wg        sync.WaitGroup
	stopCh    chan struct{}
	handlers  map[string][]AlertHandler
	mu        sync.RWMutex
}

// AlertHandler is a function that handles alerts
type AlertHandler func(Alert)

// DefaultConfig returns default alert configuration
func DefaultConfig() Config {
	return Config{
		Enabled:       true,
		BufferSize:    1000,
		FlushInterval: 5 * time.Second,
		Channels:      []ChannelConfig{},
	}
}

// NewManager creates a new alert manager
func NewManager(config Config) *Manager {
	if config.BufferSize <= 0 {
		config.BufferSize = 1000
	}
	if config.FlushInterval <= 0 {
		config.FlushInterval = 5 * time.Second
	}

	m := &Manager{
		config:   config,
		channels: make(map[string]Channel),
		buffer:   make(chan Alert, config.BufferSize),
		stopCh:   make(chan struct{}),
		handlers: make(map[string][]AlertHandler),
	}

	if config.Enabled {
		m.wg.Add(1)
		go m.processLoop()
	}

	return m
}

// RegisterChannel registers an alert channel
func (m *Manager) RegisterChannel(channel Channel) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.channels[channel.Name()] = channel
}

// UnregisterChannel unregisters an alert channel
func (m *Manager) UnregisterChannel(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.channels, name)
}

// OnAlert registers a handler for specific alert levels
func (m *Manager) OnAlert(level Level, handler AlertHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[string(level)] = append(m.handlers[string(level)], handler)
}

// Send sends an alert immediately
func (m *Manager) Send(ctx context.Context, alert Alert) error {
	if !m.config.Enabled {
		return nil
	}

	if alert.ID == "" {
		alert.ID = generateID()
	}
	if alert.Timestamp.IsZero() {
		alert.Timestamp = time.Now()
	}

	// Execute handlers
	m.mu.RLock()
	handlers := m.handlers[string(alert.Level)]
	allHandlers := m.handlers["*"]
	m.mu.RUnlock()

	for _, h := range handlers {
		go h(alert)
	}
	for _, h := range allHandlers {
		go h(alert)
	}

	// Send to channels
	m.mu.RLock()
	channels := make([]Channel, 0, len(m.channels))
	for _, ch := range m.channels {
		channels = append(channels, ch)
	}
	m.mu.RUnlock()

	var lastErr error
	for _, ch := range channels {
		if err := ch.Send(ctx, alert); err != nil {
			lastErr = fmt.Errorf("failed to send to %s: %w", ch.Name(), err)
		}
	}

	return lastErr
}

// Queue queues an alert for batch processing
func (m *Manager) Queue(alert Alert) {
	if !m.config.Enabled {
		return
	}

	if alert.ID == "" {
		alert.ID = generateID()
	}
	if alert.Timestamp.IsZero() {
		alert.Timestamp = time.Now()
	}

	select {
	case m.buffer <- alert:
	default:
		// Buffer full, drop oldest
		select {
		case <-m.buffer:
			m.buffer <- alert
		default:
		}
	}
}

// Close closes the alert manager
func (m *Manager) Close() error {
	close(m.stopCh)
	m.wg.Wait()
	close(m.buffer)
	return nil
}

func (m *Manager) processLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.FlushInterval)
	defer ticker.Stop()

	batch := make([]Alert, 0, 100)

	for {
		select {
		case <-m.stopCh:
			// Process remaining alerts
			for alert := range m.buffer {
				batch = append(batch, alert)
				if len(batch) >= 100 {
					m.flush(batch)
					batch = batch[:0]
				}
			}
			if len(batch) > 0 {
				m.flush(batch)
			}
			return

		case alert := <-m.buffer:
			batch = append(batch, alert)
			if len(batch) >= 100 {
				m.flush(batch)
				batch = batch[:0]
			}

		case <-ticker.C:
			if len(batch) > 0 {
				m.flush(batch)
				batch = batch[:0]
			}
		}
	}
}

func (m *Manager) flush(alerts []Alert) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, alert := range alerts {
		_ = m.Send(ctx, alert)
	}
}

func generateID() string {
	return fmt.Sprintf("alert_%d", time.Now().UnixNano())
}

// LogChannel is a channel that logs alerts
type LogChannel struct {
	name   string
	logger interface{}
}

// NewLogChannel creates a new log channel
func NewLogChannel(name string, logger interface{}) *LogChannel {
	return &LogChannel{name: name, logger: logger}
}

// Name returns the channel name
func (c *LogChannel) Name() string {
	return c.name
}

// Send sends an alert to the log
func (c *LogChannel) Send(ctx context.Context, alert Alert) error {
	// Implementation depends on your logger
	fmt.Printf("[ALERT][%s] %s: %s\n", alert.Level, alert.Title, alert.Message)
	return nil
}

// WebhookChannel is a channel that sends alerts via webhook
type WebhookChannel struct {
	name   string
	url    string
	headers map[string]string
}

// NewWebhookChannel creates a new webhook channel
func NewWebhookChannel(name, url string, headers map[string]string) *WebhookChannel {
	return &WebhookChannel{name: name, url: url, headers: headers}
}

// Name returns the channel name
func (c *WebhookChannel) Name() string {
	return c.name
}

// Send sends an alert via webhook
func (c *WebhookChannel) Send(ctx context.Context, alert Alert) error {
	// Implementation would use http.Client to POST to webhook URL
	fmt.Printf("[WEBHOOK][%s] Would send to %s: %s\n", c.name, c.url, alert.Title)
	return nil
}

// Helper methods for common alert types

// Debug sends a debug alert
func (m *Manager) Debug(title, message string, tags ...map[string]string) {
	m.Queue(Alert{
		Level:   LevelDebug,
		Title:   title,
		Message: message,
		Tags:    mergeTags(tags...),
	})
}

// Info sends an info alert
func (m *Manager) Info(title, message string, tags ...map[string]string) {
	m.Queue(Alert{
		Level:   LevelInfo,
		Title:   title,
		Message: message,
		Tags:    mergeTags(tags...),
	})
}

// Warning sends a warning alert
func (m *Manager) Warning(title, message string, tags ...map[string]string) {
	m.Queue(Alert{
		Level:   LevelWarning,
		Title:   title,
		Message: message,
		Tags:    mergeTags(tags...),
	})
}

// Error sends an error alert
func (m *Manager) Error(title, message string, tags ...map[string]string) {
	m.Queue(Alert{
		Level:   LevelError,
		Title:   title,
		Message: message,
		Tags:    mergeTags(tags...),
	})
}

// Critical sends a critical alert
func (m *Manager) Critical(title, message string, tags ...map[string]string) {
	m.Queue(Alert{
		Level:   LevelCritical,
		Title:   title,
		Message: message,
		Tags:    mergeTags(tags...),
	})
}

func mergeTags(tags ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, t := range tags {
		for k, v := range t {
			result[k] = v
		}
	}
	return result
}
