# Code Policy

This document summarizes the engineering conventions currently in force in this
repository and where each one is enforced. Versioning and release rules live in
the [release policy](release-policy.md); vulnerability reporting in
[SECURITY.md](../../SECURITY.md).

> The English version is authoritative. Chinese translation:
> [code-policy.zh-CN.md](code-policy.zh-CN.md).

## Branching and change flow

- Long-lived branches: `main` (release line) and `develop` (integration branch).
- Pull requests always target `develop`; releases go through a `develop` →
  `main` merge (see the [release policy](release-policy.md)).
- Keep changes focused: one concern per PR; behavior changes come with
  regression tests.
- Do not add third-party dependencies without a concrete need and review.

## Commit convention

Conventional Commits with `type(scope): subject`. Types in use:
`feat`, `fix`, `docs`, `style`, `refactor`, `test`, `build`, `chore`; the scope
is a module or package name (e.g. `fix(cache): close singleflight race`).
Breaking changes add `!` after the type, e.g.
`refactor(config)!: move built-in plugin sections out of GlobalConfig`.

## Code style and static analysis

- Formatting follows `gofmt` (the `formatters` section of `.golangci.yml`);
- `.golangci.yml` enables the `standard` linter set; `errcheck` is exempt in
  `_test.go` files, `examples/` is excluded from linting, and the
  comments / common-false-positives / std-error-handling presets are excluded;
- The lint version is pinned to `golangci-lint v2.15.0` (CI and local match);
- Run the same commands locally as CI does (see the
  [contributing guide](../../CONTRIBUTING.md)).

## Quality gates (CI-enforced)

`.github/workflows/ci.yml` splits checks into jobs; any failure blocks a merge:

| Gate | Scope | Requirement |
|---|---|---|
| Build / vet | all modules | compile and `go vet` pass |
| Test | root, `pkg`, 27 plugins | `go test -count=1 -race -cover` |
| Coverage gate | GA + GA-ready plugins | statement coverage ≥50% (`scripts/coverage_gate.sh`; the `GATED_PLUGINS` list stays in sync with the plugin architecture doc) |
| Dependency boundary | core module (`pkg`) | `go list -deps` must not contain `plugins/*` or `internal/*` |
| Tidy check | all modules | `go mod tidy` leaves no diff (`git diff --exit-code -- go.mod go.sum`) |
| Lint | root, `pkg`, 27 plugins | golangci-lint passes |
| Docker build | repository root | Dockerfile builds |

`examples/` runs build, vet, and the tidy check only.

## Module dependency boundaries

Architectural rules enforced with CI (details in
[plugin architecture](../architecture/plugin-architecture.md)):

- `pkg/` is a standalone core module; it never imports `plugins/*` or
  `internal/*` (CI-enforced);
- Plugin modules require only the core module `.../pkg`, never the root module;
  in-repo they use `require v0.0.0` plus a local `replace`;
- No cyclic dependencies between plugins; known directions: `cache → db`
  (redis/memcache drivers self-register), `health → cache → db`;
- Aggregator plugins depend on small interfaces (`rpc.MetricsRecorder`,
  `middleware.PermissionChecker`, `gateway.PermissionChecker`) instead of
  concrete plugin types;
- `examples/` is a standalone module and stays out of the main module's
  require set.

## Plugin policy

- A new plugin is a standalone `plugins/<name>` module with a complete README
  that satisfies the `kernel.Plugin` contract; heavy SDKs integrate through
  the driver-registry self-registration subpackage pattern;
- Maturity has three grades: **GA / Beta / Experimental** (definitions and the
  current grading table in the plugin architecture doc);
- GA bar: at least one verifiable production case, statement coverage ≥50%,
  complete README, no placeholder implementations; Beta plugins are not
  promoted to GA before a production case is verified;
- GA and GA-ready plugins are enforced by the coverage gate (≥50%); promotions
  must update `GATED_PLUGINS` in `scripts/coverage_gate.sh` and the grading
  table in the plugin architecture doc together.

## Dependencies and toolchain

- The Go toolchain is pinned to `toolchain go1.27.3` (root `go.mod`), CI runs
  `GO_VERSION=1.27.3`, and the Docker builder image matches;
- Security first for upgrades: `govulncheck ./...` must keep reporting zero
  reachable vulnerabilities;
- `go mod tidy` must produce no diff in every module (run with `GOWORK=off`
  where CI does).

## Configuration policy

- The only component-toggle mechanism is `plugins.<name>.enabled`; the
  `framework.init_*` compat keys were removed in v0.2 and no new
  framework-level translation layer may be added;
- Plugins parse their own config section (`UnmarshalSection`) and must not push
  private settings into `pkg/config.GlobalConfig`; the standalone sections of
  built-in plugins live in `bootstrap/pluginconfig.Config`;
- Breaking configuration changes must be marked `**Breaking:**` in the
  CHANGELOG with a migration note (see the [release policy](release-policy.md));
- `configs/config.example.yaml` is the configuration baseline; new config keys
  must update it and the related docs.

## Documentation policy

- Documentation lives in topic directories under `docs/` with
  [docs/README.md](../README.md) as the index;
- Adding a plugin or a discovery backend, or changing user-visible behavior,
  requires updating the corresponding docs and plugin README;
- The CHANGELOG follows Keep a Changelog 1.1.0; record changes under
  `## [Unreleased]` as they land.

## Repository hygiene

Do not commit: build output and generated binaries (`bin/`, `scaffold`, …),
coverage reports, logs, credentials and local config (`.env`), or editor/IDE
directories.

## Security

Report vulnerabilities privately through GitHub Security Advisories per
[SECURITY.md](../../SECURITY.md); never describe an unpatched vulnerability in
a public issue or PR. Audit events and troubleshooting live in
[security audit](../security/audit.md) and the
[audit runbook](audit-runbook.md).
