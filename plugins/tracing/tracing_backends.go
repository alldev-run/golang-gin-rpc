package tracing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type zipkinExporter struct {
	endpoint    string
	serviceName string
	client      *http.Client
}

type zipkinSpan struct {
	TraceID       string            `json:"traceId"`
	Name          string            `json:"name"`
	ID            string            `json:"id"`
	ParentID      string            `json:"parentId,omitempty"`
	Kind          string            `json:"kind,omitempty"`
	Timestamp     int64             `json:"timestamp"`
	Duration      int64             `json:"duration"`
	LocalEndpoint zipkinEndpoint    `json:"localEndpoint"`
	Tags          map[string]string `json:"tags,omitempty"`
}

type zipkinEndpoint struct {
	ServiceName string `json:"serviceName"`
}

func (e *zipkinExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	payload := make([]zipkinSpan, 0, len(spans))
	for _, span := range spans {
		spanContext := span.SpanContext()
		if !spanContext.IsValid() {
			continue
		}

		startedAt := span.StartTime()
		duration := span.EndTime().Sub(startedAt).Microseconds()
		if duration < 0 {
			duration = 0
		}

		zipkinSpan := zipkinSpan{
			TraceID:       spanContext.TraceID().String(),
			Name:          span.Name(),
			ID:            spanContext.SpanID().String(),
			Timestamp:     startedAt.UnixMicro(),
			Duration:      duration,
			LocalEndpoint: zipkinEndpoint{ServiceName: e.serviceName},
		}
		if parent := span.Parent(); parent.IsValid() {
			zipkinSpan.ParentID = parent.SpanID().String()
		}
		switch span.SpanKind() {
		case trace.SpanKindClient:
			zipkinSpan.Kind = "CLIENT"
		case trace.SpanKindServer:
			zipkinSpan.Kind = "SERVER"
		case trace.SpanKindProducer:
			zipkinSpan.Kind = "PRODUCER"
		case trace.SpanKindConsumer:
			zipkinSpan.Kind = "CONSUMER"
		}

		for _, attribute := range span.Attributes() {
			if zipkinSpan.Tags == nil {
				zipkinSpan.Tags = make(map[string]string)
			}
			zipkinSpan.Tags[string(attribute.Key)] = attribute.Value.String()
		}
		if status := span.Status(); status.Code == codes.Error {
			if zipkinSpan.Tags == nil {
				zipkinSpan.Tags = make(map[string]string)
			}
			zipkinSpan.Tags["error"] = status.Description
			if status.Description == "" {
				zipkinSpan.Tags["error"] = "true"
			}
		}

		payload = append(payload, zipkinSpan)
	}
	if len(payload) == 0 {
		return nil
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode Zipkin spans: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create Zipkin export request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("export Zipkin spans: %w", err)
	}
	_, readErr := io.Copy(io.Discard, resp.Body)
	closeErr := resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("zipkin export returned HTTP status %s", resp.Status)
	}
	if readErr != nil {
		return fmt.Errorf("read Zipkin export response: %w", readErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close Zipkin export response: %w", closeErr)
	}
	return nil
}

func (e *zipkinExporter) Shutdown(context.Context) error {
	e.client.CloseIdleConnections()
	return nil
}

func newZipkinTracerProvider(config Config) (*TracerProvider, error) {
	exporter := &zipkinExporter{
		endpoint:    config.GetURL(),
		serviceName: config.ServiceName,
		client:      &http.Client{Timeout: 10 * time.Second},
	}

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

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter,
			sdktrace.WithBatchTimeout(config.BatchTimeout),
			sdktrace.WithMaxExportBatchSize(config.MaxExportBatchSize),
		),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(config.SampleRate)),
	)

	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	pluginLogger().Info("Zipkin tracing initialized",
		zap.String("service", config.ServiceName),
		zap.String("url", config.GetURL()),
		zap.Float64("sample_rate", config.SampleRate),
	)

	return &TracerProvider{provider: provider, config: config}, nil
}

func newJaegerTracerProvider(config Config) (*TracerProvider, error) {
	if config.Port == 0 || config.Port == 14268 {
		config.Port = Jaeger.DefaultPort()
	}
	config.Endpoint = ""

	provider, err := newOTLPTracerProvider(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Jaeger OTLP exporter: %w", err)
	}
	pluginLogger().Info("Jaeger tracing initialized using OTLP",
		zap.String("service", config.ServiceName),
		zap.String("endpoint", fmt.Sprintf("%s:%d", config.Host, config.Port)),
	)
	return provider, nil
}
