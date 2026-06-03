BINARY  := debi
PKG     := github.com/debipro/cli
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X $(PKG)/pkg/version.Version=$(VERSION)

SPEC_URL := https://raw.githubusercontent.com/debipro/openapi/refs/heads/main/openapi/spec1.yaml

.PHONY: build install test vet lint run clean spec-update snapshot

build: ## Build the debi binary into ./bin
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/debi

install: ## Install debi into $GOBIN
	go install -ldflags "$(LDFLAGS)" ./cmd/debi

test: ## Run tests
	go test ./...

vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint (must be installed)
	golangci-lint run

spec-update: ## Re-download the embedded OpenAPI spec
	curl -fsSL $(SPEC_URL) -o pkg/spec/openapi.yaml

snapshot: ## Build a local snapshot release with GoReleaser
	goreleaser release --snapshot --clean

clean: ## Remove build artifacts
	rm -rf bin dist
