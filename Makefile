VERSION  ?= $(shell git describe --tags --always --dirty)
COMMIT   ?= $(shell git rev-parse --short HEAD)
DATE     ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS  = -ldflags "\
	-X github.com/rhysmcneill/ssmctl/internal/version.Version=$(VERSION) \
	-X github.com/rhysmcneill/ssmctl/internal/version.Commit=$(COMMIT) \
	-X github.com/rhysmcneill/ssmctl/internal/version.BuildDate=$(DATE)"

BINARY   = bin/ssmctl

.PHONY: build build-all test test-cover lint fmt vet install setup e2e e2e-aws ci

# ── Build ──────────────────────────────────────────────────────────────────────

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/ssmctl

build-all:
	GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o bin/ssmctl-linux-amd64       ./cmd/ssmctl
	GOOS=linux   GOARCH=arm64 go build $(LDFLAGS) -o bin/ssmctl-linux-arm64       ./cmd/ssmctl
	GOOS=darwin  GOARCH=amd64 go build $(LDFLAGS) -o bin/ssmctl-darwin-amd64      ./cmd/ssmctl
	GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -o bin/ssmctl-darwin-arm64      ./cmd/ssmctl
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/ssmctl-windows-amd64.exe ./cmd/ssmctl
	GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o bin/ssmctl-windows-arm64.exe ./cmd/ssmctl

install:
	go install $(LDFLAGS) ./cmd/ssmctl

# ── Test ───────────────────────────────────────────────────────────────────────

test:
	go test ./...

test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

# Run CLI smoke tests against the compiled binary (no AWS required).
e2e:
	go test ./e2e/ -v -count=1

# Run full AWS integration tests (requires real AWS credentials).
e2e-aws: build
	go test -tags e2e ./e2e/ -v -count=1

# ── Code quality ───────────────────────────────────────────────────────────────

fmt:
	gofmt -w -s .
	goimports -w .

vet:
	go vet ./...

lint:
	golangci-lint run

# ── Developer setup ────────────────────────────────────────────────────────────

# Install development tooling.  Run once after cloning.
setup:
	GOTOOLCHAIN=local go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	@if command -v pre-commit >/dev/null 2>&1; then \
		pre-commit install; \
		pre-commit install --hook-type commit-msg; \
	else \
		echo "pre-commit not found — install via 'pip install pre-commit' or 'brew install pre-commit', then re-run 'make setup'"; \
	fi


# ── CI ─────────────────────────────────────────────────────────────────────────

# Full local CI check — mirrors what the CI workflow runs on PRs.
ci: vet test build e2e
