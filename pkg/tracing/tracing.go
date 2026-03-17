// Package tracing provides distributed tracing functionality
package tracing

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"alldev-gin-rpc/pkg/logger"
)

// TracerType represents the type of tracing backend
type TracerType string

// Supported tracing backends
const (
	// Zipkin tracing backend
	Zipkin TracerType = "zipkin"
	
	// Jaeger tracing backend
	Jaeger TracerType = "jaeger"
	
	// OpenTelemetry Protocol (OTLP) backend
	OTLP TracerType = "otlp"
	
	// Prometheus tracing backend (future implementation)
	Prometheus TracerType = "prometheus"
	
	// AWS X-Ray tracing backend (future implementation)
	AWSXRay TracerType = "xray"
	
	// Google Cloud Trace backend (future implementation)
	GoogleCloudTrace TracerType = "gcp"
	
	// Azure Application Insights backend (future implementation)
	AzureAppInsights TracerType = "azure"
)

// IsValid checks if the tracer type is supported
func (tt TracerType) IsValid() bool {
	supportedTypes := []TracerType{
		Zipkin,
		Jaeger,
		OTLP,
		Prometheus,
		AWSXRay,
		GoogleCloudTrace,
		AzureAppInsights,
	}
	
	for _, supportedType := range supportedTypes {
		if tt == supportedType {
			return true
		}
	}
	return false
}

// String returns the string representation of TracerType
func (tt TracerType) String() string {
	return string(tt)
}

// DisplayName returns a human-readable display name
func (tt TracerType) DisplayName() string {
	switch tt {
	case Zipkin:
		return "Zipkin"
	case Jaeger:
		return "Jaeger"
	case OTLP:
		return "OpenTelemetry Protocol"
	case Prometheus:
		return "Prometheus"
	case AWSXRay:
		return "AWS X-Ray"
	case GoogleCloudTrace:
		return "Google Cloud Trace"
	case AzureAppInsights:
		return "Azure Application Insights"
	default:
		return "Unknown"
	}
}

// DefaultPort returns the default port for the tracing backend
func (tt TracerType) DefaultPort() int {
	switch tt {
	case Zipkin:
		return 9411
	case Jaeger:
		return 14268 // HTTP collector
	case OTLP:
		return 4317 // gRPC
	case Prometheus:
		return 9090
	case AWSXRay:
		return 2000 // UDP
	case GoogleCloudTrace:
		return 443 // HTTPS
	case AzureAppInsights:
		return 443 // HTTPS
	default:
		return 0
	}
}

// IsCloudBased returns true if the tracing backend is a cloud service
func (tt TracerType) IsCloudBased() bool {
	switch tt {
	case AWSXRay, GoogleCloudTrace, AzureAppInsights:
		return true
	default:
		return false
	}
}

// IsOpenSource returns true if the tracing backend is open source
func (tt TracerType) IsOpenSource() bool {
	switch tt {
	case Zipkin, Jaeger, OTLP, Prometheus:
		return true
	default:
		return false
	}
}

// GetSupportedTypes returns all supported tracer types
func GetSupportedTypes() []TracerType {
	return []TracerType{
		Zipkin,
		Jaeger,
		OTLP,
		Prometheus,
		AWSXRay,
		GoogleCloudTrace,
		AzureAppInsights,
	}
}

// GetImplementedTypes returns tracer types that are currently implemented
func GetImplementedTypes() []TracerType {
	return []TracerType{
		Zipkin,
		// Jaeger and OTLP can be added here when implemented
	}
}

// GetFutureTypes returns tracer types planned for future implementation
func GetFutureTypes() []TracerType {
	return []TracerType{
		Jaeger,
		OTLP,
		Prometheus,
		AWSXRay,
		GoogleCloudTrace,
		AzureAppInsights,
	}
}

// ParseTracerType parses a string into TracerType
func ParseTracerType(s string) (TracerType, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	
	switch s {
	case "zipkin":
		return Zipkin, nil
	case "jaeger":
		return Jaeger, nil
	case "otlp", "opentelemetry":
		return OTLP, nil
	case "prometheus":
		return Prometheus, nil
	case "xray", "aws-xray", "aws":
		return AWSXRay, nil
	case "gcp", "google-cloud", "google":
		return GoogleCloudTrace, nil
	case "azure", "app-insights":
		return AzureAppInsights, nil
	default:
		return "", fmt.Errorf("unsupported tracer type: %s", s)
	}
}

// IsImplemented returns true if the tracer type is currently implemented
func (tt TracerType) IsImplemented() bool {
	implementedTypes := GetImplementedTypes()
	for _, implementedType := range implementedTypes {
		if tt == implementedType {
			return true
		}
	}
	return false
}

// RequiresAuthentication returns true if the tracing backend typically requires authentication
func (tt TracerType) RequiresAuthentication() bool {
	switch tt {
	case Jaeger, OTLP:
		return false // Optional
	case AWSXRay, GoogleCloudTrace, AzureAppInsights:
		return true // Required for cloud services
	default:
		return false
	}
}



// GetURL returns the full URL for the tracing backend
func (c Config) GetURL() string {
	if c.Endpoint != "" {
		if strings.HasPrefix(c.Endpoint, "http") {
			return c.Endpoint
		}
		return fmt.Sprintf("http://%s:%d%s", c.Host, c.Port, c.Endpoint)
	}
	return fmt.Sprintf("http://%s:%d", c.Host, c.Port)
}

// Validate validates the tracing configuration
func (c Config) Validate() error {
	// Parse tracer type
	tracerType, err := ParseTracerType(c.Type)
	if err != nil {
		return fmt.Errorf("invalid tracer type: %w", err)
	}

	// Check if implemented
	if !tracerType.IsImplemented() {
		return fmt.Errorf("tracer type '%s' is not implemented", tracerType.DisplayName())
	}

	// Validate required fields
	if c.ServiceName == "" {
		return fmt.Errorf("service name is required")
	}

	if tracerType.RequiresAuthentication() {
		if c.Username == "" || c.Password == "" {
			return fmt.Errorf("%s requires authentication", tracerType.DisplayName())
		}
	}

	// Set default port if not specified
	if c.Port == 0 {
		c.Port = tracerType.DefaultPort()
	}

	return nil
}

// TracerProvider wraps OpenTelemetry tracer provider
type TracerProvider struct {
	provider *sdktrace.TracerProvider
	config   Config
}

// Tracer is an alias for TracerProvider for backward compatibility
type Tracer = TracerProvider

// NewTracerProvider creates a new tracer provider based on configuration
func NewTracerProvider(config Config) (*TracerProvider, error) {
	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid tracing configuration: %w", err)
	}

	if !config.Enabled {
		logger.Info("Tracing is disabled")
		return &TracerProvider{config: config}, nil
	}

	// Parse tracer type
	tracerType, err := ParseTracerType(config.Type)
	if err != nil {
		return nil, fmt.Errorf("invalid tracer type: %w", err)
	}

	// Check if the tracer type is implemented
	if !tracerType.IsImplemented() {
		return nil, fmt.Errorf("tracer type '%s' is not yet implemented. Supported types: %v", 
			tracerType.DisplayName(), GetImplementedTypes())
	}

	// Create tracer provider based on type
	switch tracerType {
	case Zipkin:
		return newZipkinTracerProvider(config)
	case Jaeger:
		return newJaegerTracerProvider(config)
	case OTLP:
		return newOTLPTracerProvider(config)
	default:
		return nil, fmt.Errorf("unsupported tracer type: %s", tracerType.DisplayName())
	}
}

// NewTracer creates a new tracer for backward compatibility
func NewTracer(config Config) (*Tracer, error) {
	return NewTracerProvider(config)
}

// newZipkinTracerProvider creates a Zipkin tracer provider
func newZipkinTracerProvider(config Config) (*TracerProvider, error) {
	// Create Zipkin exporter
	exporter, err := zipkin.New(
		config.GetURL(),
		zipkin.WithLogger(log.New(os.Stdout, "zipkin: ", log.LstdFlags)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Zipkin exporter: %w", err)
	}

	// Create resource
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			attribute.String("service.name", config.ServiceName),
			attribute.String("service.version", config.ServiceVersion),
			attribute.String("deployment.environment", config.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create tracer provider
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter,
			sdktrace.WithBatchTimeout(config.BatchTimeout),
			sdktrace.WithMaxExportBatchSize(config.MaxExportBatchSize),
		),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(config.SampleRate)),
	)

	// Set global tracer provider
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	logger.Info("Zipkin tracing initialized",
		zap.String("service", config.ServiceName),
		zap.String("url", config.GetURL()),
		zap.Float64("sample_rate", config.SampleRate),
	)

	return &TracerProvider{
		provider: provider,
		config:   config,
	}, nil
}

// newJaegerTracerProvider creates a Jaeger tracer provider (placeholder)
func newJaegerTracerProvider(config Config) (*TracerProvider, error) {
	// TODO: Implement Jaeger exporter
	return nil, fmt.Errorf("Jaeger tracer provider is not yet implemented")
}

// newOTLPTracerProvider creates an OTLP tracer provider (placeholder)
func newOTLPTracerProvider(config Config) (*TracerProvider, error) {
	// TODO: Implement OTLP exporter
	return nil, fmt.Errorf("OTLP tracer provider is not yet implemented")
}

// CreateTracerProviderByType creates a tracer provider using TracerType enum
func CreateTracerProviderByType(tracerType TracerType, config Config) (*TracerProvider, error) {
	// Validate tracer type
	if !tracerType.IsValid() {
		return nil, fmt.Errorf("invalid tracer type: %s", tracerType)
	}

	// Check if implemented
	if !tracerType.IsImplemented() {
		return nil, fmt.Errorf("tracer type '%s' is not yet implemented", tracerType.DisplayName())
	}

	// Update config type
	config.Type = tracerType.String()

	return NewTracerProvider(config)
}

// ValidateConfig validates the tracing configuration
func ValidateConfig(config Config) error {
	return config.Validate()
}

// Tracer returns a tracer instance
func (tp *TracerProvider) Tracer(name string) trace.Tracer {
	if tp.provider == nil {
		return trace.NewNoopTracerProvider().Tracer(name)
	}
	return tp.provider.Tracer(name)
}

// Shutdown shuts down the tracer provider
func (tp *TracerProvider) Shutdown(ctx context.Context) error {
	if tp.provider == nil {
		return nil
	}
	
	logger.Info("Shutting down tracer provider")
	return tp.provider.Shutdown(ctx)
}

// IsEnabled returns whether tracing is enabled
func (tp *TracerProvider) IsEnabled() bool {
	return tp.config.Enabled && tp.provider != nil
}

// StartSpan starts a new span
func (tp *TracerProvider) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if !tp.IsEnabled() {
		return ctx, trace.SpanFromContext(ctx)
	}
	
	tracer := tp.Tracer(name)
	return tracer.Start(ctx, name, opts...)
}

// StartSpanWithParent starts a new span with explicit parent
func (tp *TracerProvider) StartSpanWithParent(ctx context.Context, name string, parentSpan trace.Span, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if !tp.IsEnabled() {
		return ctx, trace.SpanFromContext(ctx)
	}
	
	tracer := tp.Tracer(name)
	opts = append(opts, trace.WithLinks(trace.Link{
		SpanContext: parentSpan.SpanContext(),
	}))
	return tracer.Start(ctx, name, opts...)
}

// Global tracer provider instance
var globalTracerProvider *TracerProvider

// InitGlobalTracer initializes the global tracer provider
func InitGlobalTracer(config Config) error {
	tp, err := NewTracerProvider(config)
	if err != nil {
		return err
	}
	globalTracerProvider = tp
	return nil
}

// GlobalTracer returns the global tracer provider
func GlobalTracer() *TracerProvider {
	if globalTracerProvider == nil {
		// Initialize with default config if not already done
		globalTracerProvider, _ = NewTracerProvider(DefaultConfig())
	}
	return globalTracerProvider
}

// ShutdownGlobalTracer shuts down the global tracer provider
func ShutdownGlobalTracer(ctx context.Context) error {
	if globalTracerProvider != nil {
		return globalTracerProvider.Shutdown(ctx)
	}
	return nil
}

// SpanFromContext extracts a span from context
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// ContextWithSpan adds a span to context
func ContextWithSpan(ctx context.Context, span trace.Span) context.Context {
	return trace.ContextWithSpan(ctx, span)
}

// SetSpanAttributes sets attributes on a span
func SetSpanAttributes(span trace.Span, attributes map[string]interface{}) {
	if span == nil || !span.IsRecording() {
		return
	}
	
	for key, value := range attributes {
		switch v := value.(type) {
		case string:
			span.SetAttributes(attribute.String(key, v))
		case int:
			span.SetAttributes(attribute.Int(key, v))
		case int64:
			span.SetAttributes(attribute.Int64(key, v))
		case float64:
			span.SetAttributes(attribute.Float64(key, v))
		case bool:
			span.SetAttributes(attribute.Bool(key, v))
		default:
			span.SetAttributes(attribute.String(key, fmt.Sprintf("%v", v)))
		}
	}
}

// SetSpanError sets error information on a span
func SetSpanError(span trace.Span, err error) {
	if span == nil || !span.IsRecording() || err == nil {
		return
	}
	
	span.SetAttributes(attribute.String("error", err.Error()))
	span.SetStatus(codes.Error, err.Error())
}

// SetSpanOK sets success status on a span
func SetSpanOK(span trace.Span) {
	if span == nil || !span.IsRecording() {
		return
	}
	
	span.SetStatus(codes.Ok, "OK")
}
