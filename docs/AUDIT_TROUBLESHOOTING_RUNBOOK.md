# Audit Troubleshooting Runbook

## Scope

This runbook is for production troubleshooting of the HTTP audit pipeline:

- Middleware emit path (`pkg/middleware.Audit`)
- Sink write path (`LogSink` / `FileSink` / `SQLSink` / `AsyncSink`)
- Dynamic governance via `configcenter`
- Audit Prometheus metrics and alerts

## Quick Health Checks

1. Check metrics endpoint:

```bash
curl -s http://<service-host>/metrics | grep -E "audit_writes_total|audit_dropped_total|audit_write_duration_seconds"
```

2. Verify write throughput exists:

```promql
sum(rate(audit_writes_total[5m]))
```

3. Verify drop events:

```promql
sum(increase(audit_dropped_total[5m])) by (sink, reason)
```

## Common Symptoms

### 1) `AuditDroppedEventsDetected` fired

Likely causes:

- `AsyncSink` buffer too small (`reason="buffer_full"`)
- upstream request deadlines too aggressive (`reason="ctx_done"`)
- sink lifecycle closed unexpectedly (`reason="closed"`)

Actions:

1. Increase `AsyncSinkConfig.BufferSize` and/or `Workers`
2. Review downstream sink latency (`audit_write_duration_seconds`)
3. Check service shutdown sequence and sink `Close()` timing

### 2) `AuditWriteErrorRateHigh` fired

Likely causes:

- `SQLSink` DB connection failures / schema mismatch
- `FileSink` path permission or disk full
- serialization errors (rare)

Actions:

1. Check application logs for sink write errors
2. For `SQLSink`, validate table and permissions
3. For `FileSink`, validate directory and remaining disk

### 3) `AuditWriteLatencyP95High` fired

Likely causes:

- database write bottleneck
- slow storage I/O
- synchronous sink chain too long

Actions:

1. Break out sink latency by `sink` label
2. Use `AsyncSink` wrapping slow sinks
3. Reduce payload size in `Metadata` if oversized

## SQLSink Verification

Expected insert target:

- table: `audit_events` (or configured table)
- required columns align with `docs/AUDIT_SECURITY_GUIDE.md`

Validation SQL examples:

```sql
SELECT COUNT(*) FROM audit_events;
SELECT action, result, COUNT(*) FROM audit_events GROUP BY action, result;
```

## ConfigCenter Dynamic Governance Checks

1. Verify key exists:

- namespace: `governance`
- key: `audit/http`

2. Verify payload format:

```json
{
  "enabled": true,
  "skip_paths": ["/health", "/ready", "/metrics"],
  "sensitive_keys": ["password", "token", "authorization", "api_key", "secret"]
}
```

3. If policy rollback needed, delete key `governance/audit/http` to reset middleware boot defaults.

## Dashboard & Alerts

- Grafana dashboard JSON: `configs/grafana/audit_dashboard.json`
- Prometheus alert rules: `configs/audit_alerts.yml`
- Prometheus config entry: `configs/prometheus.yml`

## Escalation Checklist

Before escalation, collect:

1. Last 15m values for:
   - `audit_writes_total`
   - `audit_dropped_total`
   - `audit_write_duration_seconds`
2. Sink-specific logs (`log/file/sql/async`)
3. Current dynamic config payload from `configcenter`
4. Recent deploy and config-change timeline
