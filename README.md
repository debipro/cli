# debi CLI

`debi` is a command-line interface for the [Debi API](https://debi.pro/docs/api)
(formerly TuCuota). It helps you build, test, and manage your Debi integration
right from the terminal — inspired by the Stripe CLI.

With the CLI you can:

- Authenticate once and store your secret key securely in your OS keychain.
- Call any Debi endpoint with generic `get`/`post`/`put`/`delete` commands.
- Use auto-generated resource commands (`debi customers list`,
  `debi payments create`, ...) built from Debi's OpenAPI specification.
- Poll, replay, and forward webhook events while developing locally.

## Quick start

```bash
# Install (see Installation below), then:
debi login                              # store your sk_test_... key
debi --test customers list --limit 5    # first API call
debi spec info                          # check embedded spec version
debi spec update                        # refresh commands when the API adds endpoints
```

If a resource command is missing, run `debi spec update` to fetch the latest
OpenAPI document. Release binaries embed a snapshot; `make spec-update`
refreshes that snapshot before tagging a release.

## Installation

### Prebuilt binaries

Download the archive for your platform from the
[Releases page](https://github.com/debipro/cli/releases), extract it, and
place the `debi` binary on your `PATH`.

### Homebrew

After the first release, install from the tap (update checksums in
[`formula/debi.rb`](formula/debi.rb) when publishing):

```bash
brew install ./formula/debi.rb
```

### Docker

```bash
docker run --rm -e DEBI_API_KEY=sk_test_... ghcr.io/debipro/cli:latest version
```

Inside a container there is no OS keychain, so pass your key via the
`DEBI_API_KEY` environment variable.

### From source

```bash
go install github.com/debipro/cli/cmd/debi@latest
```

## Authentication

Create a secret key in the [Debi developers dashboard](https://debi.pro/dashboard/developers),
then store it:

```bash
debi login                 # prompts for the key (hidden input)
debi login --api-key sk_test_...
echo "sk_test_..." | debi login   # non-interactive
debi logout                # remove key from keychain (keeps profile settings)
```

The key is stored in your operating system's secure credential store (macOS
Keychain, Windows Credential Manager, or the Linux Secret Service). The
environment (`live`/`test`) is inferred from the key prefix.

Key resolution order for every command:

1. `--api-key` flag
2. `DEBI_API_KEY` environment variable
3. OS keychain entry for the active profile

## Usage

```bash
debi [command]
debi [command] --help
debi completion bash > /etc/bash_completion.d/debi
```

Use `--verbose` or set `DEBI_DEBUG=1` to log API requests to stderr (never
includes your API key).

### Generic requests

```bash
debi get /v1/customers -d limit=5
debi get /v1/customers --auto-paginate
debi post /v1/customers -d name="Jane Doe" -d email=jane@example.com
debi put /v1/customers/CSxxxx -d email=new@example.com
debi delete /v1/links/LKxxxx
```

`-d key=value` sends a string; `-d key:=json` sends a raw JSON value (numbers,
booleans, arrays, objects). Dotted keys create nested objects:

```bash
debi post /v1/payments -d amount:=1600 -d currency=ARS -d metadata.order_id=123
```

### Resource commands

Generated from the embedded OpenAPI spec:

```bash
debi customers list --limit 10
debi customers create --name "Jane Doe" --email jane@example.com
debi customers retrieve CSxxxx
debi customers archive CSxxxx
debi payments create --amount 1600 --currency ARS
debi subscriptions cancel SBxxxx
debi billing_portal configurations list
```

Run `debi <resource> --help` to discover available operations and flags.

### Environment selection

```bash
debi --test customers list   # sandbox (api.debi-test.pro)
debi --live customers list   # production (api.debi.pro)
```

### Profiles

```bash
debi config list
debi config get mode
debi --profile prod login
debi config use prod
debi config set api_version 2025-10-02
debi config set device_name "fede-macbook"   # sent as X-Debi-CLI-Device header
debi config unset prod
```

### Webhooks (local development)

Debi does not push events to the CLI, so webhook tooling is built on the Events
API and local replay:

```bash
# Tail recent events (polls the Events API).
debi events tail
debi events tail --type 'customer.*' --since 1700000000
debi events tail --forward-to http://127.0.0.1:4242/ --webhook-secret whsec_...

# Run a local receiver (verify signatures with --webhook-secret).
debi listen 4242 --webhook-secret whsec_...

# Re-deliver a stored event's payload to a local endpoint (localhost only).
debi events resend EVxxxx --forward-to http://127.0.0.1:3000/webhooks --webhook-secret whsec_...

# Run sandbox scenarios that emit events.
debi events trigger customer.created

# Verify a payload signature offline.
echo '{"id":"EV..."}' | debi events verify --webhook-secret whsec_... --signature 't=...,v1=...'
```

**Security:** `events resend` and `events tail --forward-to` only allow
`http://127.0.0.1`, `http://localhost`, and `http://[::1]` targets to reduce
accidental SSRF. Use `debi listen` plus a tunnel (ngrok, cloudflared) when you
need a public URL for the Debi dashboard.

### Output

Output is pretty-printed, colorized JSON by default. Use `--json` for raw output
(ideal for piping into `jq`) and `--no-color` to disable colors.

## Configuration

- Config file: `$XDG_CONFIG_HOME/debi/config.toml` (override with `--config`).
- Override config dir with `DEBI_CONFIG_DIR`.
- Secrets are never written to the config file.

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md) for architecture notes and golden-test
workflow.

```bash
make build         # build ./bin/debi
make test          # run tests
make vet           # go vet
make spec-update   # refresh the embedded OpenAPI spec
make snapshot      # local GoReleaser snapshot build
```

Resource commands are generated at runtime from Debi's OpenAPI specification,
whose canonical source is
[`debipro/openapi`](https://github.com/debipro/openapi). A copy is embedded in
the binary as an offline fallback (`pkg/spec/openapi.yaml`).

```bash
debi spec info      # show the active source and version
debi spec update    # fetch the latest spec from GitHub and cache it locally
make spec-update    # refresh the embedded fallback copy (for a new release)
```

At runtime the CLI prefers a locally cached copy (written by `debi spec
update`, stored in the config dir) and falls back to the embedded copy, so no
command list is hand-maintained and the CLI works offline.

## Releasing

Releases are produced by [GoReleaser](https://goreleaser.com) on tag push
(`vX.Y.Z`) via GitHub Actions: cross-platform binaries are attached to the
GitHub Release and a multi-arch Docker image is published to GHCR. GoReleaser
refreshes the embedded OpenAPI spec before each build.
