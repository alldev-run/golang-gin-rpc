# Audit Security Guide

## Ops Assets

- Grafana dashboard: `configs/grafana/audit_dashboard.json`
- Troubleshooting runbook: `docs/AUDIT_TROUBLESHOOTING_RUNBOOK.md`
- Alert rules: `configs/audit_alerts.yml`

## Integration Environment

Run `docker-compose -f deploy/docker-compose.integration.yml up -d` to spin up Prometheus + Grafana with auto-provisioned audit dashboard and alert rules:

- Grafana: http://localhost:3000 (admin/admin) → Audit folder → Audit Pipeline Observability
- Prometheus: http://localhost:9090 → Alerts → audit-pipeline

Paths auto-mounted:
- `configs/grafana/audit_dashboard.json` → Grafana dashboard
- `configs/grafana/provisioning/` → Grafana datasource and dashboard provisioning
- `configs/audit_alerts.yml` → Prometheus alert rules
- `deploy/prometheus.yml` → Prometheus config with rule loading

## Overview

`pkg/audit` + `pkg/middleware` provides an enterprise-friendly audit trail capability:

- Structured audit event model (`pkg/audit`)
- Sensitive field masking (`pkg/audit.Masker`)
- Pluggable sinks (`pkg/audit.Sink`, `MultiSink`, `LogSink`)
- HTTP audit middleware (`pkg/middleware.Audit`)

## Quick Start

```go
import (
    "github.com/alldev-run/golang-gin-rpc/pkg/middleware"
)

router := gin.New()
router.Use(middleware.RequestID())
router.Use(middleware.Audit())
```

## Custom Sink

```go
type KafkaAuditSink struct{}

func (KafkaAuditSink) Write(ctx context.Context, event audit.Event) error {
    // publish event to Kafka / MQ / SIEM
    return nil
}

cfg := middleware.DefaultAuditConfig()
cfg.Sink = audit.NewMultiSink(audit.LogSink{}, KafkaAuditSink{})
router.Use(middleware.Audit(cfg))
```

## Gateway Integration

`pkg/gateway.Gateway` now wires `middleware.Audit(...)` in `SetupRoutes(...)`.

```go
gw := bootstrap.GetGateway()

auditCfg := middleware.DefaultAuditConfig()
auditCfg.Sink = audit.NewMultiSink(audit.LogSink{}, KafkaAuditSink{})

gw.SetAuditConfig(auditCfg)
```

See runnable example: `examples/gateway/main.go`.

## ConfigCenter Dynamic Governance

You can update audit policy at runtime via `pkg/configcenter` without restarting services.

```go
base := middleware.DefaultAuditConfig()
dynamic := middleware.NewDynamicAuditConfig(base)

cfg := base
cfg.Dynamic = dynamic
router.Use(middleware.Audit(cfg))

sub, err := middleware.BindAuditConfigCenter(
    context.Background(),
    cc,
    "governance",
    "audit/http",
    dynamic,
)
if err != nil {
    panic(err)
}
defer sub.Close()
```

Config value format (JSON):

```json
{
  "enabled": true,
  "skip_paths": ["/health", "/ready", "/metrics"],
  "sensitive_keys": ["password", "token", "authorization", "api_key", "secret"]
}
```

Delete key `governance/audit/http` to reset to middleware boot defaults.

## Audit Metrics

Audit pipeline now emits Prometheus metrics via `pkg/metrics`:

- `audit_writes_total{sink,result}`
- `audit_write_duration_seconds{sink,result}`
- `audit_dropped_total{sink,reason}`

Common labels:

- `sink`: `log`, `file`, `sql`, `async`
- `result`: `success`, `error`
- `reason`: `buffer_full`, `ctx_done`, `closed`

### PromQL Examples

- Audit write throughput (QPS)

```promql
sum(rate(audit_writes_total[5m]))
```

- Audit write error ratio (5m)

```promql
sum(rate(audit_writes_total{result="error"}[5m]))
/
clamp_min(sum(rate(audit_writes_total[5m])), 0.001)
```

- Audit write latency p95 by sink

```promql
histogram_quantile(
  0.95,
  sum(rate(audit_write_duration_seconds_bucket[5m])) by (le, sink)
)
```

- Dropped events in 5m

```promql
sum(increase(audit_dropped_total[5m])) by (sink, reason)
```

### Alert Rules

Ready-to-use rules are provided in `configs/audit_alerts.yml` and loaded by `configs/prometheus.yml`:

- `AuditWriteErrorRateHigh`
- `AuditDroppedEventsDetected`
- `AuditWriteLatencyP95High`

## Async Sink (High Throughput)

For high QPS services, wrap your sink with `audit.AsyncSink` to avoid blocking request path:

```go
base := audit.NewMultiSink(audit.LogSink{}, KafkaAuditSink{})
async := audit.NewAsyncSink(base, audit.AsyncSinkConfig{
    BufferSize: 2048,
    Workers:    2,
    DropOnFull: true,
})
defer async.Close()

cfg := middleware.DefaultAuditConfig()
cfg.Sink = async
router.Use(middleware.Audit(cfg))
```

## Persistent Storage Sinks

### FileSink (JSON Lines)

```go
fileSink, err := audit.NewFileSink("./logs/audit/audit.log")
if err != nil {
    panic(err)
}
defer fileSink.Close()

cfg := middleware.DefaultAuditConfig()
cfg.Sink = fileSink
router.Use(middleware.Audit(cfg))
```

### SQLSink (Database)

```go
sqlSink, err := audit.NewSQLSink(db, audit.SQLSinkConfig{
    Table: "audit_events",
})
if err != nil {
    panic(err)
}

cfg := middleware.DefaultAuditConfig()
cfg.Sink = sqlSink
router.Use(middleware.Audit(cfg))
```

### BatchSQLSink (批量写入优化)

For high QPS scenarios, use `BatchSQLSink` to batch inserts and reduce database round-trips:

```go
batchSink, err := audit.NewBatchSQLSink(db, audit.BatchSQLSinkConfig{
    Table:         "audit_events",
    BatchSize:     100,              // 每批写入数量
    FlushInterval: 5 * time.Second,  // 自动刷新间隔
    MaxRetries:    3,                // 失败重试次数
    RetryDelay:    100 * time.Millisecond,
})
if err != nil {
    panic(err)
}
defer batchSink.Close()

cfg := middleware.DefaultAuditConfig()
cfg.Sink = batchSink
router.Use(middleware.Audit(cfg))
```

Features:
- **Automatic batching**: Accumulates events up to `BatchSize`
- **Timed flush**: Ensures low latency with `FlushInterval`
- **Retry with backoff**: Automatic retry on transient failures
- **Graceful shutdown**: `Close()` flushes remaining events

### Async + BatchSQLSink (最高吞吐量)

Combine `AsyncSink` + `BatchSQLSink` for maximum throughput with backpressure control:

```go
// 1. Create batch SQL sink for efficient database writes
batchSink, err := audit.NewBatchSQLSink(db, audit.BatchSQLSinkConfig{
    Table:         "audit_events",
    BatchSize:     200,
    FlushInterval: 3 * time.Second,
    MaxRetries:    3,
})
if err != nil {
    panic(err)
}

// 2. Wrap with AsyncSink for non-blocking request path
asyncSink := audit.NewAsyncSink(batchSink, audit.AsyncSinkConfig{
    BufferSize: 5000,    // 内存缓冲队列大小
    Workers:    4,       // 并发写入工作者数
    DropOnFull: true,    // 队列满时丢弃（保护应用）
})
defer asyncSink.Close()

// 3. Use in middleware
cfg := middleware.DefaultAuditConfig()
cfg.Sink = asyncSink
router.Use(middleware.Audit(cfg))
```

Performance characteristics:

| Configuration | Latency Impact | Throughput | Durability |
|--------------|----------------|------------|------------|
| `SQLSink` (direct) | High (sync DB write) | Low | Strong |
| `BatchSQLSink` | Medium (batching delay) | Medium-High | Strong |
| `AsyncSink` → `BatchSQLSink` | **None** (async) | **Very High** | Eventual |
| `AsyncSink` → `SQLSink` | None (async) | High | Eventual |

Warning: When using `DropOnFull: true`, audit events may be lost during traffic spikes. Monitor `audit_dropped_total` metric.

Reference table schema:

```sql
CREATE TABLE audit_events (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  timestamp DATETIME NOT NULL,
  request_id VARCHAR(128) NULL,
  trace_id VARCHAR(128) NULL,
  tenant_id VARCHAR(128) NULL,
  user_id VARCHAR(128) NULL,
  username VARCHAR(128) NULL,
  client_ip VARCHAR(64) NULL,
  method VARCHAR(16) NULL,
  path VARCHAR(512) NULL,
  status_code INT NULL,
  action VARCHAR(32) NOT NULL,
  resource VARCHAR(256) NULL,
  result VARCHAR(32) NULL,
  message TEXT NULL,
  metadata JSON NULL,
  sensitive TINYINT(1) NULL,
  duration_ms BIGINT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

## Sensitive Data Masking

By default, these keys are masked:

- `password`
- `token`
- `authorization`
- `api_key`
- `secret`

You can override via `AuditConfig.SensitiveKeys`.

## Context Conventions

Audit middleware reads the following context keys if present:

- `request_id`
- `trace_id`
- `tenant_id`
- `user_id`
- `username`

Recommend enabling in this order:

1. `middleware.RequestID()`
2. Authentication middleware (`JWT`, API key)
3. `middleware.Audit()`

## Best Practices

1. Send audit events to immutable storage (e.g. MQ + cold storage)
2. Keep masking rules centrally governed by `configcenter`
3. Distinguish audit success/failure by HTTP status and business result codes
4. Avoid storing full request bodies unless compliance requires it
