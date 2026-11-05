# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to semantic versioning.

## [Unreleased]

### Added

- HTTP webhook delivery for alerts, with configurable headers, context support, and a bounded default client timeout.
- HTTP distributed-transaction coordinator endpoints and OpenTelemetry transaction tracing through the application's global provider.
- Transaction-coordinator and PostgreSQL bulk-insert protocol documentation.
- Redis and Memcached fallback configuration for failover caches.
- TCP health checks and metadata-weighted service selection for discovery.
- File-based APNs and FCM credentials.
- `bootstrap.Dep*` constants (for example `bootstrap.DepDBFactory`, `bootstrap.DepTracer`) for the dependency-container keys; the string values are unchanged.
- `kernel.NewRegistry`, `kernel.DefaultRegistry`, and the `kernel.WithRegistry` loader option for per-instance plugin registries; the default registry still provides blank-import self-registration.
- `tracing.SetGlobalTracer` for installing (or, with `nil`, resetting) the process-wide tracer provider.

### Changed

- **Breaking:** the Redis, RPC, Discovery, Messaging and Security sections moved out of `pkg/config.GlobalConfig` into `github.com/alldev-run/golang-gin-rpc/bootstrap/pluginconfig.Config`. YAML keys are unchanged. Read them with `boot.GetPluginConfig()` instead of `boot.GetConfig()`, for example `boot.GetPluginConfig().Security.JWT`. `Observability` stays in `GlobalConfig` because the core logger reads its `logging` block.
- **Breaking:** `pkg/` is now a separate Go module, `github.com/alldev-run/golang-gin-rpc/pkg`. Plugin modules require it instead of the root module. Import paths are unchanged; modules that build against a local checkout need `require github.com/alldev-run/golang-gin-rpc/pkg v0.0.0` plus a `replace` pointing at `pkg/`.
- **Breaking:** `github.com/alldev-run/golang-gin-rpc/pkg/bootstrap` moved to `github.com/alldev-run/golang-gin-rpc/bootstrap`. Its API is unchanged; update the import path. It links every built-in plugin, so it cannot live in the plugin-free core module.
- Documentation is now organized into category directories under `docs/`; see [the documentation index](docs/README.md).
- **Breaking:** WebSocket servers now use the WebSocket library's same-origin policy when `origin_patterns` is empty. Set `allow_any_origin: true` only when unrestricted cross-origin access is intentional.
- **Breaking:** `bootstrap.StopFramework` now returns an error; callers should handle shutdown errors.
- **Breaking:** `jwtx.(*Manager).RevokeUser` and `jwtx.RevokeUser` now return an error.
- **Breaking:** configuration `Validate` methods now use pointer receivers for `logger.Config`, `health.HealthConfig`, `health.HealthCheckConfig`, `messaging.Config`, `middleware.Config` and its nested config types, `tracing.Config`, `cache/redis.Config`, and `cache/memcache.Config`.
- PostgreSQL `CopyFrom` now performs batched, parameterized multi-row inserts in one transaction because the `database/sql` client uses `lib/pq` and does not expose a pgx connection.
- Query-cache writes continue to invalidate the entire cache because cache keys are hashes and do not retain table information.
- The tracing plugin uses Zipkin v2 HTTP export and Jaeger's OTLP gRPC receiver instead of deprecated OpenTelemetry exporter packages.
- Plugin maturity grades retain GA only for previously-GA plugins meeting the measured coverage, README, and placeholder criteria; no Beta plugin was promoted without a verifiable production case.
- Configured Go lint findings were fixed, module tidy drift was resolved, and CI now defines root/plugin checks through `.github/workflows/ci.yml` and `.golangci.yml`.
- The root Dockerfile now preserves plugin module paths during dependency download; `.dockerignore` excludes generated and repository metadata.
- Plugin and repository documentation now describe supported APIs, configuration, operations, contribution, and security-reporting policies.
- Generated `bin/alldev-gin-rpc` and `scaffold` binaries are no longer tracked.

### Fixed

- Bootstrap tracing is now installed as the global tracer, so RPC, gateway, and WebSocket spans use the configured provider instead of a disabled default. The gateway reuses an active global provider rather than replacing it with its own `tracing` config.
- Each `Bootstrap` now registers its built-in plugins in its own kernel registry, so a second `Bootstrap` in the same process no longer reuses plugins bound to the first one.
- Data races on the global tracer provider and the gateway global router factory.
- Cache singleflight wait-group publication ordering and its concurrent result behavior.
- Declaration-order scanning and nullable pointer handling for `StructScanRow`.
- Nondeterministic timing assumptions in the hash-performance and rate-limiter concurrency tests.
- Cache backend fallback validation and service health checks.

### Deprecated

- `orm.NewJaegerTracer` is retained as a compatibility alias for `orm.NewOTelTracer`; configure exporters through the global OpenTelemetry `TracerProvider`.

### Removed

- The stale `tracing.NewJaegerExporter` API reference was removed from the tracing documentation; configure Jaeger through the OTLP-based provider.
- The exported ORM test helpers `orm.MockSpan` and `orm.MockSpanContext` were removed from the production API.
