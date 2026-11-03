# Contributing

## Development environment

- Download and install Go **1.25.7** for your operating system and architecture from
  [go.dev/dl](https://go.dev/dl/), put its `bin` directory on `PATH`, and verify
  with `go version`. Use the repository's `go.work` workspace from the repo root.
- Install `golangci-lint` **2.15.0** for lint checks.
- From the repository root, run `make help` to see available targets; common targets
  include `make build`, `make test`, `make vet`, and `make fmt`.
- The 27 directories under `plugins/` are independent Go modules. Run checks from
  each module directory as well as the root module.

## Local CI checks

For the root module:

```sh
go build ./...
go vet ./...
go test -count=1 -race -cover ./...
GOWORK=off go mod tidy
golangci-lint run --config=.golangci.yml ./...
```

For each plugin module, run from its directory:

```sh
go build ./...
go vet ./...
go test -count=1 -race -cover ./...
go mod tidy
git diff --exit-code -- go.mod go.sum
golangci-lint run --config=../../.golangci.yml ./...
```

`go mod tidy` must not leave an unexpected diff. Run it in one module at a time
and review any dependency changes before committing. Do not commit build output,
coverage reports, credentials, or generated local binaries.

## Changes and pull requests

- Keep changes focused and add regression tests for behavior changes.
- Use conventional commit messages, for example `fix(cache): close singleflight race`
  or `docs: clarify plugin readiness`.
- Open pull requests against `develop`; include the checks run and any known
  limitations in the description.
- Do not add dependencies without a concrete need and review.
