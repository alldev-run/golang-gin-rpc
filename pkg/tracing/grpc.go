package tracing

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// GRPCInterceptor provides gRPC tracing interceptor
type GRPCInterceptor struct {
	tracer *TracerProvider
}

// NewGRPCInterceptor creates a new gRPC tracing interceptor
func NewGRPCInterceptor(tracer *TracerProvider) *GRPCInterceptor {
	return &GRPCInterceptor{tracer: tracer}
}

// UnaryServerInterceptor returns a unary server interceptor for tracing
func (i *GRPCInterceptor) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if !i.tracer.IsEnabled() {
			return handler(ctx, req)
		}

		// Extract tracing context from metadata
		md, ok := metadata.FromIncomingContext(ctx)
		if ok {
			ctx = otel.GetTextMapPropagator().Extract(ctx, &metadataCarrier{md})
		}

		// Start span
		spanName := fmt.Sprintf("grpc.%s", info.FullMethod)
		ctx, span := i.tracer.StartSpan(ctx, spanName, trace.WithAttributes(
			attribute.String("grpc.method", info.FullMethod),
			attribute.String("grpc.service", extractServiceName(info.FullMethod)),
			attribute.String("grpc.type", "unary"),
		))
		defer span.End()

		// Call handler
		resp, err := handler(ctx, req)

		// Set span attributes based on response
		if err != nil {
			st, ok := status.FromError(err)
			if ok {
				span.SetAttributes(
					attribute.String("grpc.status_code", st.Code().String()),
					attribute.String("grpc.status_message", st.Message()),
				)
			} else {
				span.SetAttributes(
					attribute.String("grpc.status_code", "Unknown"),
					attribute.String("error", err.Error()),
				)
			}
			SetSpanError(span, err)
		} else {
			span.SetAttributes(
				attribute.String("grpc.status_code", "OK"),
			)
			SetSpanOK(span)
		}

		return resp, err
	}
}

// StreamServerInterceptor returns a stream server interceptor for tracing
func (i *GRPCInterceptor) StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if !i.tracer.IsEnabled() {
			return handler(srv, ss)
		}

		// Extract tracing context from metadata
		ctx := ss.Context()
		md, ok := metadata.FromIncomingContext(ctx)
		if ok {
			ctx = otel.GetTextMapPropagator().Extract(ctx, &metadataCarrier{md})
			ctx = context.WithValue(ctx, "original_stream", ss)
		}

		// Start span
		spanName := fmt.Sprintf("grpc.%s", info.FullMethod)
		ctx, span := i.tracer.StartSpan(ctx, spanName, trace.WithAttributes(
			attribute.String("grpc.method", info.FullMethod),
			attribute.String("grpc.service", extractServiceName(info.FullMethod)),
			attribute.String("grpc.type", "stream"),
		))
		defer span.End()

		// Wrap stream with new context
		wrappedStream := &tracedServerStream{
			ServerStream: ss,
			ctx:          ctx,
		}

		// Call handler
		err := handler(srv, wrappedStream)

		// Set span attributes based on response
		if err != nil {
			st, ok := status.FromError(err)
			if ok {
				span.SetAttributes(
					attribute.String("grpc.status_code", st.Code().String()),
					attribute.String("grpc.status_message", st.Message()),
				)
			} else {
				span.SetAttributes(
					attribute.String("grpc.status_code", "Unknown"),
					attribute.String("error", err.Error()),
				)
			}
			SetSpanError(span, err)
		} else {
			span.SetAttributes(
				attribute.String("grpc.status_code", "OK"),
			)
			SetSpanOK(span)
		}

		return err
	}
}

// UnaryClientInterceptor returns a unary client interceptor for tracing
func (i *GRPCInterceptor) UnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if !i.tracer.IsEnabled() {
			return invoker(ctx, method, req, reply, cc, opts...)
		}

		// Start span
		spanName := fmt.Sprintf("grpc.%s", method)
		ctx, span := i.tracer.StartSpan(ctx, spanName, trace.WithAttributes(
			attribute.String("grpc.method", method),
			attribute.String("grpc.service", extractServiceName(method)),
			attribute.String("grpc.type", "unary_client"),
			attribute.String("grpc.target", cc.Target()),
		))
		defer span.End()

		// Inject tracing context into metadata
		md := metadata.New(nil)
		otel.GetTextMapPropagator().Inject(ctx, &metadataCarrier{md})
		ctx = metadata.NewOutgoingContext(ctx, md)

		// Call invoker
		err := invoker(ctx, method, req, reply, cc, opts...)

		// Set span attributes based on response
		if err != nil {
			st, ok := status.FromError(err)
			if ok {
				span.SetAttributes(
					attribute.String("grpc.status_code", st.Code().String()),
					attribute.String("grpc.status_message", st.Message()),
				)
			} else {
				span.SetAttributes(
					attribute.String("grpc.status_code", "Unknown"),
					attribute.String("error", err.Error()),
				)
			}
			SetSpanError(span, err)
		} else {
			span.SetAttributes(
				attribute.String("grpc.status_code", "OK"),
			)
			SetSpanOK(span)
		}

		return err
	}
}

// StreamClientInterceptor returns a stream client interceptor for tracing
func (i *GRPCInterceptor) StreamClientInterceptor() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		if !i.tracer.IsEnabled() {
			return streamer(ctx, desc, cc, method, opts...)
		}

		// Start span
		spanName := fmt.Sprintf("grpc.%s", method)
		ctx, span := i.tracer.StartSpan(ctx, spanName, trace.WithAttributes(
			attribute.String("grpc.method", method),
			attribute.String("grpc.service", extractServiceName(method)),
			attribute.String("grpc.type", "stream_client"),
			attribute.String("grpc.target", cc.Target()),
		))
		defer span.End()

		// Inject tracing context into metadata
		md := metadata.New(nil)
		otel.GetTextMapPropagator().Inject(ctx, &metadataCarrier{md})
		ctx = metadata.NewOutgoingContext(ctx, md)

		// Call streamer
		clientStream, err := streamer(ctx, desc, cc, method, opts...)

		// Set span attributes based on response
		if err != nil {
			st, ok := status.FromError(err)
			if ok {
				span.SetAttributes(
					attribute.String("grpc.status_code", st.Code().String()),
					attribute.String("grpc.status_message", st.Message()),
				)
			} else {
				span.SetAttributes(
					attribute.String("grpc.status_code", "Unknown"),
					attribute.String("error", err.Error()),
				)
			}
			SetSpanError(span, err)
		} else {
			span.SetAttributes(
				attribute.String("grpc.status_code", "OK"),
			)
			SetSpanOK(span)
		}

		return clientStream, err
	}
}

// metadataCarrier implements propagation.TextMapCarrier for gRPC metadata
type metadataCarrier struct {
	metadata.MD
}

func (c *metadataCarrier) Get(key string) string {
	values := c.MD.Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (c *metadataCarrier) Set(key, value string) {
	c.MD.Set(key, value)
}

func (c *metadataCarrier) Keys() []string {
	keys := make([]string, 0, len(c.MD))
	for k := range c.MD {
		keys = append(keys, k)
	}
	return keys
}

// tracedServerStream wraps grpc.ServerStream to provide tracing context
type tracedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *tracedServerStream) Context() context.Context {
	return s.ctx
}

// extractServiceName extracts service name from full method
func extractServiceName(fullMethod string) string {
	// fullMethod format: /package.service/method
	if len(fullMethod) < 3 || fullMethod[0] != '/' {
		return fullMethod
	}
	
	// Find the second slash
	for i := 1; i < len(fullMethod); i++ {
		if fullMethod[i] == '/' {
			return fullMethod[1:i] // Return package.service
		}
	}
	
	return fullMethod[1:] // Return everything after first slash if no second slash
}
