package audit

import (
	"sync"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/metrics"
)

type auditMetricsObserver interface {
	RecordAuditWrite(sink, result string, duration time.Duration)
	RecordAuditDrop(sink, reason string)
}

var (
	auditMetricsOnce sync.Once
	auditMetrics     auditMetricsObserver
)

func getAuditMetrics() auditMetricsObserver {
	auditMetricsOnce.Do(func() {
		auditMetrics = metrics.NewMetricsCollector()
	})
	return auditMetrics
}

func recordAuditWriteMetric(sink, result string, duration time.Duration) {
	getAuditMetrics().RecordAuditWrite(sink, result, duration)
}

func recordAuditDropMetric(sink, reason string) {
	getAuditMetrics().RecordAuditDrop(sink, reason)
}
