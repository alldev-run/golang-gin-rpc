# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to semantic versioning.

## [Unreleased]

## [v0.2.0-rc.1] - 2026-11-06

### Added
- Gateway listen port resolution: an explicit `gateway.port` wins; otherwise the gateway falls back to the main-app HTTP port + 2 (default 8082), so enabling `api-gateway` no longer steals the main app's :8080 listener (previously the gin server failed to bind and all business routes were served — and rejected — by the gateway proxy).
- Configurable framework knobs replacing hardcoded values: `server.shutdown_timeout` (graceful shutdown, previously a fixed 30s in main/bootstrap/gateway-service stop), `observability.metrics.listen_addr` (standalone metrics listener, previously hardcoded `:9090`), and full `rpc.servers.*` consumption (type/host/port/network/timeout/max_msg_size/reflection; previously only enabled/protocol/degradation were read).
- The main binary now accepts the config path as its first CLI argument (defaults to `./configs/config.yaml`); the Docker image `CMD` uses it.
- Minimal real implementations for the demo routes `GET /api/v1/db/status` (pings every created database client) and `GET/POST/DELETE /api/v1/cache/:key` (real cache reads/writes/deletes via the bootstrap cache dependency); the endpoints report explicit "not initialized" errors instead of fake success when the dependency is absent. `router.NewRouter` now takes `router.Options`.
- `.air.toml` for `make dev` hot reload; `make setup` falls back to `configs/config.example.yaml`, `start.sh` does the same (replacing its stale embedded config), and `make docker-run` mounts `./configs` read-only and only passes `--env-file` when `.env` exists.

- HTTP webhook delivery for alerts, with configurable headers, context support, and a bounded default client timeout.
- HTTP distributed-transaction coordinator endpoints and OpenTelemetry transaction tracing through the application's global provider.
- Transaction-coordinator and PostgreSQL bulk-insert protocol documentation.
- Redis and Memcached fallback configuration for failover caches.
- TCP health checks and metadata-weighted service selection for discovery.
- File-based APNs and FCM credentials.
- `bootstrap.Dep*` constants (for example `bootstrap.DepDBFactory`, `bootstrap.DepTracer`) for the dependency-container keys; the string values are unchanged.
- `kernel.NewRegistry`, `kernel.DefaultRegistry`, and the `kernel.WithRegistry` loader option for per-instance plugin registries; the default registry still provides blank-import self-registration.
- `SetLogger(*zap.Logger)` in the rpc, gateway, middleware, actor, audit, circuitbreaker, discovery, health, metrics, tracing, upload and websocket packages to inject a logger; the default remains `pkg/logger`'s global logger.
- `rpc.MetricsRecorder` and `middleware.PermissionChecker` interfaces. `rpc.NewMetricsObserver`, `middleware.RequirePermission` and `middleware.RequireAnyPermission` accept them, and `*metrics.MetricsCollector` / `*rbac.Policy` still satisfy them, so existing callers compile unchanged.
- `Bootstrap.RegisterServiceDependencies(name, keys...)` declares the dependency keys a custom managed service needs, so `ValidateDependencyCoverage` checks them; the built-in `rpc`, `api-gateway` and `websocket` services keep their existing requirements.
- `tracing.SetGlobalTracer` for installing (or, with `nil`, resetting) the process-wide tracer provider.

### Changed
- **Breaking:** the legacy `framework.init_*` configuration keys and their adapter translation layer were removed (as planned for v0.2). Component toggles are now driven exclusively by the top-level `plugins.<name>.enabled` section; `FrameworkConfig` keeps only `validate_dependency_coverage` and `services`. Configs that relied on `init_databases: false` etc. without a `plugins` section must migrate.
- **Breaking (compat restored):** `GlobalConfig.Server.HTTP` previously only matched a `server.http` YAML section while shipped configs used a flat `server.host/port`; the loader now maps the legacy flat `server:` keys onto `server.http` (an explicit `server.http` section wins). Shipped configs were migrated to the standard `server.http` schema.
- **Breaking:** `api/http-gateway` no longer mutates framework init toggles programmatically; component toggles come from the framework config's `plugins` section (`plugins.db.enabled: true` replaces the old `InitDatabases = true` after loading `database.yml`).
- Vulnerable dependencies upgraded to fixed versions across all modules (root, pkg, examples, plugins): `google.golang.org/grpc` v1.83.2, `golang.org/x/net` v0.60.0, `go.opentelemetry.io/otel/*` v1.45.0, `github.com/klauspost/compress` v1.20.2, `github.com/rabbitmq/amqp091-go` v1.13.0, `github.com/quic-go/quic-go` v0.59.1. The Go toolchain is pinned via `toolchain go1.27.3` (CI and the Docker builder image aligned); `govulncheck ./...` now reports zero reachable vulnerabilities.

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
- Dependency-coverage validation no longer reports false `missing` entries for built-in components that started but skipped initialization because their own feature config is disabled (for example auth with `security.jwt.enabled: false`); skipped components are now exempt.
- `bootstrap` deadlock risk removed: config normalization inside `Load` uses a lock-free section reader instead of re-entering `RWMutex`.

- Bootstrap tracing is now installed as the global tracer, so RPC, gateway, and WebSocket spans use the configured provider instead of a disabled default. The gateway reuses an active global provider rather than replacing it with its own `tracing` config.
- **Breaking:** `gateway.Gateway.GetRBACPolicy()` now returns the new `gateway.PermissionChecker` interface instead of `*rbac.Policy`, and `SetRBACPolicy` accepts it. Migration: `*rbac.Policy` still satisfies the interface, so `SetRBACPolicy(policy)` is unchanged; callers that need `*rbac.Policy`-only methods from the getter type-assert with `gw.GetRBACPolicy().(*rbac.Policy)`.
- `logger.Base()` returns the global logger for direct calls; plugin `SetLogger` defaults use it, so log `caller` fields point at the real call site again (the previous default added an extra caller skip and reported the caller's caller).
- Service loggers returned by `GetServiceLoggerInstance`/`GetServiceLogger` no longer apply a wrapper caller skip, so direct calls stop reporting `runtime/proc.go` as the caller; the `ServiceLogger` wrapper keeps the skip.
- Business routes registered through `gateway/router.RouteRegistry` can now read the request ID via `middleware.GetRequestID(c)`; previously it was always empty because the registry engine is mounted with `gin.WrapH` and lost the outer context keys.
- Removed the unused `plugins/search` replace directive from `plugins/db/go.mod`.
- Each `Bootstrap` now registers its built-in plugins in its own kernel registry, so a second `Bootstrap` in the same process no longer reuses plugins bound to the first one.
- Data races on the global tracer provider and the gateway global router factory.
- Cache singleflight wait-group publication ordering and its concurrent result behavior.
- Declaration-order scanning and nullable pointer handling for `StructScanRow`.
- Nondeterministic timing assumptions in the hash-performance and rate-limiter concurrency tests.
- Cache backend fallback validation and service health checks.

### Security

- `govulncheck ./...` reports zero reachable vulnerabilities after the dependency upgrades and toolchain alignment above (previously 22 reachable findings across gRPC authorization bypass, net/http HTTP/2, OpenTelemetry and compression libraries).

### Deprecated

- `orm.NewJaegerTracer` is retained as a compatibility alias for `orm.NewOTelTracer`; configure exporters through the global OpenTelemetry `TracerProvider`.

### Removed

- The stale `tracing.NewJaegerExporter` API reference was removed from the tracing documentation; configure Jaeger through the OTLP-based provider.
- The exported ORM test helpers `orm.MockSpan` and `orm.MockSpanContext` were removed from the production API.
