package rpc

import (
	"strings"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/metrics"
)

type MetricsObserver struct {
	collector *metrics.MetricsCollector
	service   string
}

type governanceObserver interface {
	RecordGovernance(clientType ClientType, method, event string)
}

func NewMetricsObserver(service string, collector *metrics.MetricsCollector) *MetricsObserver {
	return &MetricsObserver{collector: collector, service: service}
}

func (m *MetricsObserver) RecordRequest(clientType ClientType, method, target, status string, duration time.Duration) {
	if m == nil || m.collector == nil {
		return
	}
	protocol, service, methodName, targetName, statusName := m.normalize(clientType, method, target, status)
	m.collector.RecordRPCRequest(service, method, status, duration)
	m.collector.RecordRPCRequestDetailed(protocol, service, methodName, targetName, statusName, duration)
	if statusName != "ok" {
		m.collector.RecordRPCError(service, methodName, statusName)
	}
}

func (m *MetricsObserver) RecordRetry(clientType ClientType, method string, attempt int) {
	if m == nil || m.collector == nil {
		return
	}
	protocol, service, methodName, _, _ := m.normalize(clientType, method, "", "retry")
	m.collector.RecordRPCError(service, methodName, "retry")
	m.collector.RecordRPCGovernanceEvent(protocol, service, methodName, "retry")
}

func (m *MetricsObserver) RecordGovernance(clientType ClientType, method, event string) {
	if m == nil || m.collector == nil {
		return
	}
	protocol, service, methodName, _, _ := m.normalize(clientType, method, "", event)
	m.collector.RecordRPCGovernanceEvent(protocol, service, methodName, event)
	switch event {
	case "auth_reject":
		m.collector.RecordAuthFailure(protocol, service, methodName)
	case "auth_attempt":
		m.collector.RecordAuthAttempt(protocol, service, methodName)
	case "rate_limit_reject":
		m.collector.RecordRateLimitHit(protocol, service, methodName)
	}
}

func (m *MetricsObserver) normalize(clientType ClientType, method, target, status string) (protocol, service, methodName, targetName, statusName string) {
	protocol = string(clientType)
	if protocol == "" {
		protocol = "unknown"
	}
	methodName = normalizeMetricValue(method)
	targetName = normalizeMetricValue(target)
	statusName = normalizeMetricValue(status)
	service = normalizeMetricValue(m.service)
	if service == "unknown" {
		service = deriveServiceFromMethod(clientType, method)
	}
	return protocol, service, methodName, targetName, statusName
}

func deriveServiceFromMethod(clientType ClientType, method string) string {
	switch clientType {
	case ClientTypeJSONRPC:
		parts := strings.Split(method, ".")
		if len(parts) > 1 {
			return normalizeMetricValue(parts[0])
		}
	case ClientTypeGRPC:
		trimmed := strings.TrimPrefix(method, "/")
		parts := strings.Split(trimmed, "/")
		if len(parts) > 1 {
			return normalizeMetricValue(parts[0])
		}
	}
	return "unknown"
}

func normalizeMetricValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	return value
}
