package rpc

import (
	"testing"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/metrics"
)

func TestNewMetricsObserver(t *testing.T) {
	collector := metrics.NewMetricsCollector()
	observer := NewMetricsObserver("orders", collector)
	if observer == nil {
		t.Fatal("expected observer to be created")
	}
	if observer.collector == nil {
		t.Fatal("expected collector to be assigned")
	}
	if observer.service != "orders" {
		t.Fatalf("unexpected service: %s", observer.service)
	}
}

func TestMetricsObserver_RecordRequest_WithExplicitService(t *testing.T) {
	collector := metrics.NewMetricsCollector()
	observer := NewMetricsObserver("orders", collector)

	observer.RecordRequest(ClientTypeGRPC, "/svc.Method", "localhost:9001", "ok", time.Millisecond)
	observer.RecordRequest(ClientTypeJSONRPC, "user.get", "http://localhost:8080", "rpc_error", time.Millisecond)
}

func TestMetricsObserver_RecordRequest_UsesClientTypeWhenServiceEmpty(t *testing.T) {
	collector := metrics.NewMetricsCollector()
	observer := NewMetricsObserver("", collector)

	observer.RecordRequest(ClientTypeGRPC, "/svc.Method", "localhost:9001", "ok", time.Millisecond)
	observer.RecordRequest(ClientTypeJSONRPC, "user.get", "http://localhost:8080", "error", time.Millisecond)
}

func TestMetricsObserver_RecordRetry_WithExplicitService(t *testing.T) {
	collector := metrics.NewMetricsCollector()
	observer := NewMetricsObserver("orders", collector)

	observer.RecordRetry(ClientTypeGRPC, "/svc.Method", 1)
	observer.RecordRetry(ClientTypeJSONRPC, "user.get", 2)
}

func TestMetricsObserver_RecordRetry_UsesClientTypeWhenServiceEmpty(t *testing.T) {
	collector := metrics.NewMetricsCollector()
	observer := NewMetricsObserver("", collector)

	observer.RecordRetry(ClientTypeGRPC, "/svc.Method", 1)
	observer.RecordRetry(ClientTypeJSONRPC, "user.get", 2)
}

func TestMetricsObserver_RecordRequest_NilReceiverDoesNotPanic(t *testing.T) {
	var observer *MetricsObserver
	observer.RecordRequest(ClientTypeGRPC, "/svc.Method", "localhost:9001", "ok", time.Millisecond)
}

func TestMetricsObserver_RecordRetry_NilReceiverDoesNotPanic(t *testing.T) {
	var observer *MetricsObserver
	observer.RecordRetry(ClientTypeGRPC, "/svc.Method", 1)
}

func TestMetricsObserver_RecordRequest_NilCollectorDoesNotPanic(t *testing.T) {
	observer := NewMetricsObserver("orders", nil)
	observer.RecordRequest(ClientTypeGRPC, "/svc.Method", "localhost:9001", "ok", time.Millisecond)
}

func TestMetricsObserver_RecordRetry_NilCollectorDoesNotPanic(t *testing.T) {
	observer := NewMetricsObserver("orders", nil)
	observer.RecordRetry(ClientTypeGRPC, "/svc.Method", 1)
}
