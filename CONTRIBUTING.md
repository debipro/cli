# Contributing to debi CLI

Thank you for contributing to the debi CLI. This document covers the architecture and common development tasks.

## Architecture

```text
cmd/debi/main.go          Entry point
pkg/cmd/                  Cobra command tree
pkg/debi/                 HTTP client for the Debi API
pkg/spec/                 Embedded OpenAPI spec + local cache
pkg/config/               TOML profiles (non-secret settings)
pkg/keyring/              OS keychain / encrypted file backend
pkg/webhook/              Debi-Signature HMAC helpers
pkg/output/               JSON pretty-printing
```

Resource commands (`debi customers list`, etc.) are **generated at runtime** from the OpenAPI specification. Path heuristics live in [`pkg/cmd/resource.go`](pkg/cmd/resource.go) (`analyzePath`).

The active spec is loaded in this order:

1. Valid cached copy at `$DEBI_CONFIG_DIR/openapi.yaml` (from `debi spec update`)
2. Embedded fallback at `pkg/spec/openapi.yaml` (refreshed via `make spec-update` before releases)

## Development

```bash
make build
make test
make vet
make lint          # requires golangci-lint
make spec-update   # refresh embedded OpenAPI copy
```

Set `DEBI_CONFIG_DIR` to a temp directory in tests to avoid touching your real config.

### Updating the command tree golden file

When resource commands change intentionally:

```bash
UPDATE_GOLDEN=1 go test ./pkg/cmd -run Golden
```

Commit the updated `pkg/cmd/testdata/commands.golden`.

## Pull requests

- Keep changes focused; match existing code style.
- Run `go test ./...` before submitting.
- Update README when adding user-facing commands or flags.

## Releasing

Releases are automated via GoReleaser when a `v*.*.*` tag is pushed. The release workflow:

1. Downloads the latest OpenAPI spec into the embed path
2. Builds cross-platform binaries
3. Publishes Docker images to `ghcr.io/debipro/cli`

See [README.md](README.md#releasing) for details.
