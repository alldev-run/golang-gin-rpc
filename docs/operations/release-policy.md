# Release Policy

This document defines the version-numbering rules, the release-branch model,
and the release process for `github.com/alldev-run/golang-gin-rpc`. All notable
changes are tracked in the [CHANGELOG](../../CHANGELOG.md) in
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) format, and versions
follow [Semantic Versioning](https://semver.org/).

> The English version is authoritative. Chinese translation:
> [release-policy.zh-CN.md](release-policy.zh-CN.md).

Current engineering conventions are summarized in the
[code policy](code-policy.md).

## Versioning model at a glance

Versions are `vMAJOR.MINOR.PATCH` with pre-release suffixes `-rc.N` (release
candidate) and `-beta.N`. The legacy tags `v0.1.17-bate1`/`-bate2` misspelled
`beta`; do not use that spelling again.

| Stage | Example | Meaning | Compatibility promise |
|---|---|---|---|
| `0.x` — **unstable** (current) | `v0.1.19`, `v0.2.0-rc.1` | APIs, plugin contracts, and the config schema may still change | None between minors (the Go-modules v0 rule) |
| `1.0.0` — **stable** | — | Stability starts: kernel, bootstrap API, and config schema frozen | 1.x compatibility promise, in the spirit of the Go 1 guarantee |
| Minor `1.N.0` | `v1.1.0` | Feature release — the "big update" line inside a stable 1.x | Backward-compatible additions only |
| Patch `1.N.M` | `v1.1.1` | Bug fixes, security fixes, dependency upgrades | No API or config-semantics changes |
| Major `X.0.0` | `v2.0.0` | Breaking changes only | Migration required |

Within a stable 1.x line, the version users perceive as a "big update" is the
MINOR bump (`1.0.x` → `1.1.0`); MAJOR stays reserved for breaking changes.
This is how long-lived Go libraries operate — Gin, for example, has remained
on `v1.x` for years, shipping features as `v1.10.0` and fixes as `v1.9.1`.

## How mature projects do it

| Project | Versioning and release-branch practice |
|---|---|
| [Go](https://go.dev/doc/devel/release) | Go 1 (2012) froze the language and standard library under the Go 1 compatibility promise. Two minor releases per year (Feb/Aug), patch releases as needed, security fixes for the two most recent minors. Every minor keeps a long-lived branch (`release-branch.go1.N`). |
| [Kubernetes](https://kubernetes.io/releases/version-skew-policy/) | Roughly three semver minors per year; a `release-1.N` branch per minor; the three most recent branches receive roughly monthly patch releases. |
| [Gin](https://github.com/gin-gonic/gin) | Unstable before v1.0; stable on `v1.x` for years. Features ride minors, fixes ride patches; releases are cut from the main branch without per-minor branches. |
| [Terraform](https://developer.hashicorp.com/terraform/language/v1-compatibility) | 0.x minors carried breaking changes; v1.0 (2021) froze the API and pushed breaking changes to future majors. |
| [Node.js](https://nodejs.org/en/about/previous-releases) | Even-numbered majors enter LTS and keep a maintained per-line branch (`v20.x`, `v22.x`) for about 30 months. |

This project follows the Go model scaled to its size: stay on 0.x until the
API is proven, then start a 1.x stability promise. Per-minor release branches
carry every final release from `v0.2.0` on.

## Current phase: 0.x (unstable)

The project is currently in the 0.x phase (latest: `v0.2.0-rc.1`). The early
v0.0.x / v0.1.x tags did not strictly separate features from fixes; from
`v0.2.0` onward the rules below apply.

- **Minor `0.N.0`** is a feature release and may contain breaking changes
  (Go modules promise no compatibility for v0), provided that:
  - every breaking item is marked with `**Breaking:**` in the CHANGELOG
    together with a migration note;
  - the corresponding commits use the conventional-commits `!` marker, e.g.
    `refactor(config)!:` and `build!:` in this repository's history.
- **Patch `0.N.M`** contains fixes only: no exported-API signature changes, no
  config-key semantics changes, no minimum-Go-version bump.
- **Release branches.** Every final release is tagged on its own
  `release-0.N` branch (see
  [Release branches and maintenance](#release-branches-and-maintenance)); rc
  tags live on `main` — `v0.2.0-rc.1` was produced this way. When `v0.2.0`
  finalizes, `release-0.2` is cut from the release merge and tagged on the
  branch.

`v1.0.0` is reached when the `pkg` kernel and bootstrap API are stable, the
configuration schema is stable, GA plugins cover the main capabilities, and
verifiable production cases exist — the same 0.x → 1.0 transition Terraform
made in 2021.

## Stable phase: 1.x

- **Minor `1.N.0` — feature releases.** New plugins, new capabilities,
  additive config sections, plugin maturity promotions (Beta → GA),
  deprecation notices, dependency upgrades. Breaking changes are **not**
  allowed; deprecated APIs and old config keys keep working.
- **Patch `1.N.M` — fix releases.** Bug fixes, security fixes, dependency
  vulnerability upgrades; no behavior or API changes beyond the fix. Security
  fixes follow the disclosure process in [SECURITY.md](../../SECURITY.md).
- **Major `X.0.0` — breaking changes.** Removing or changing published
  exported APIs, removing deprecated config keys or changing their semantics,
  raising the minimum Go toolchain version, or module-layout changes. A major
  release requires: an issue describing the breakage surface and migration
  path, a deprecation notice at least one minor ahead (CHANGELOG
  `### Deprecated`, old API kept working during the window), an
  `vX.0.0-rc.1` candidate with a feature freeze, and migration notes
  (`**Breaking:**` entries) in the CHANGELOG.

## Branches and tags

Long-lived branches:

- `develop` — integration branch; pull requests always target it (see the
  [contributing guide](../../CONTRIBUTING.md));
- `main` — release-ready line; release merges land here and rc tags are cut
  here;
- `release-N.M` — one per supported minor (for example `release-0.2`,
  `release-1.1`); final feature/major release tags and patch tags are cut
  here (see below).

## Framework version in code

`pkg/version` is the in-code source of truth for the framework version:

- `version.Version` — the full semantic version without the `v` prefix,
  pre-release suffix included (e.g. `0.2.0-rc.1`);
- `version.Major` — the major version component;
- `version.Minor` — the minor version component.

**Before every release**, a release-prep commit bumps these to match the
upcoming tag exactly — on `develop` for feature releases and majors, on the
`release-N.M` branch for patches. Never tag a release while `pkg/version`
still shows the previous version. `make build` additionally injects
`git describe --tags --always` through LDFLAGS, so release artifacts are
stamped from tagged commits.

## Release branches and maintenance

**Does a released version keep a git branch?** Yes — every final release
gets a branch, and its release tags are cut on that branch:

| Release type | Branch | Tag location |
|---|---|---|
| rc `vX.Y.Z-rc.N` | — (branch not cut yet) | `main` |
| Feature release `v0.N.0` / `v1.N.0` — the "big update" line — and major `vX.0.0` | `release-N.M`, cut from the release merge on `main` | the `release-N.M` branch |
| Patch `vX.N.M` | the existing `release-N.M` | the `release-N.M` branch |

- **Creating:** when a feature release is ready, merge `develop` → `main`
  (`Merge develop for vX.Y.Z`), cut `release-N.M` from that commit, and tag
  `vX.Y.Z` on the release branch — the Go `release-branch.go1.N` /
  Kubernetes `release-1.N` convention. (`v0.2.0-rc.1` on `main` follows the
  rc rule; finalizing `v0.2.0` cuts `release-0.2` and tags it on the
  branch.)
- **Version bump first:** the release-prep commit updating `pkg/version`
  (`Version` / `Major` / `Minor`) lands before the tag, never after.
- **Patching:** a fix lands on `develop` first, then is cherry-picked onto
  the affected `release-N.M` branch; bump `pkg/version` to `X.N.(M+1)` there
  and tag `vX.N.(M+1)` on the branch. Never merge `develop` into a release
  branch — it carries next-minor work.
- **Retention:** a branch lives as long as its line is supported. Baseline:
  the latest minor line (what [SECURITY.md](../../SECURITY.md) promises
  today). Target as the project matures: the latest two minor lines,
  mirroring Go's security-support window. When a line leaves its support
  window, its branch may be deleted — tags persist independently, so any
  released version remains checkable forever.
- **Multi-module tags:** prefixed tags (`pkg/vX.Y.Z`,
  `plugins/<name>/vX.Y.Z`) follow the same branch rules and are tagged from
  the same commits.

rc releases are tagged on `main` after a `Merge develop for vX.Y.Z-rc.N`
merge (`v0.2.0-rc.1` was produced this way). Docker image tags mirror git
tags (CI currently builds images but does not push them).

## Multi-module tagging

This repository is a multi-module monorepo (`go.work`): the root module, the
core module `pkg/`, the 27 `plugins/<name>/` modules, and `examples/` are each
independent Go modules. Module paths and tag rules:

| Module | Module path | Tag format |
|---|---|---|
| Root (framework main module) | `github.com/alldev-run/golang-gin-rpc` | `vX.Y.Z[-rc.N]` |
| Core kernel | `github.com/alldev-run/golang-gin-rpc/pkg` | `pkg/vX.Y.Z[-rc.N]` |
| Plugin | `github.com/alldev-run/golang-gin-rpc/plugins/<name>` | `plugins/<name>/vX.Y.Z[-rc.N]` |

- In-repo development does not depend on remote versions: modules reference
  each other with `require ... v0.0.0` plus a `replace` to a local relative
  path (e.g. `plugins/db/go.mod` has
  `replace github.com/alldev-run/golang-gin-rpc/pkg => ../../pkg`);
- Historical tags (`v0.0.1`…`v0.2.0-rc.1`) cover the root module only; **when
  a core or plugin module is published for external use, it must get a
  prefixed tag per the table above** so consumers can run
  `go get github.com/alldev-run/golang-gin-rpc/pkg@vX.Y.Z`;
- `examples/` is for reference inside this repository only; it is not
  published and not tagged;
- A plugin extracted to its own repository (see
  [plugin architecture](../architecture/plugin-architecture.md)) switches to
  that repository's own tag scheme.

## Release checklist

1. CI green on `develop`: build / vet / `go test -count=1 -race -cover` /
   per-module tidy with no diff / golangci-lint / plugin coverage gate (≥50%) /
   core dependency-boundary check / docker build;
2. `govulncheck ./...` reports no reachable vulnerabilities;
3. Fold the CHANGELOG `## [Unreleased]` into `## [vX.Y.Z] - YYYY-MM-DD` with
   Added / Changed / Fixed / Security / Deprecated / Removed sections and
   `**Breaking:**` markers on breaking entries;
4. Sync README, `docs/`, and the affected plugin READMEs for version-driven
   changes;
5. Bump the framework version in code: a release-prep commit updates
   `pkg/version` (`Version`, `Major`, `Minor`) to match the version about to
   be tagged, pre-release suffix included;
6. Feature release `vX.Y.0` / major `vX.0.0`: merge `develop` → `main`, cut
   `release-N.M` from the merge commit, tag `vX.Y.Z` on the release branch,
   and push branch and tag (push prefixed tags too when publishing
   core/plugin modules);
7. rc release: merge `develop` → `main` and tag `vX.Y.Z-rc.N` on `main`;
8. Patch release `vX.Y.Z`: cherry-pick fixes onto `release-N.M`, bump
   `pkg/version` there, tag `vX.Y.Z` on the branch, and publish matching
   prefixed submodule tags when applicable.

## Supported versions

Consistent with [SECURITY.md](../../SECURITY.md): fixes (including security
fixes) are provided for the latest `main` release line and for `develop` only;
older tags are unmaintained. "Latest release line" means the supported
`release-N.M` branch — baseline the newest one, target the newest two.
