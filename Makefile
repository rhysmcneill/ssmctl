VERSION  ?= $(shell git describe --tags --always --dirty)
COMMIT   ?= $(shell git rev-parse --short HEAD)
DATE     ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS  = -ldflags "\
	-X github.com/rhysmcneill/ssmctl/internal/version.Version=$(VERSION) \
	-X github.com/rhysmcneill/ssmctl/internal/version.Commit=$(COMMIT) \
	-X github.com/rhysmcneill/ssmctl/internal/version.BuildDate=$(DATE)"

.PHONY: build build-all test lint install

build:
	go build $(LDFLAGS) -o bin/ssmctl ./cmd/ssmctl

build-all:
	GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o bin/ssmctl-linux-amd64       ./cmd/ssmctl
	GOOS=linux   GOARCH=arm64 go build $(LDFLAGS) -o bin/ssmctl-linux-arm64       ./cmd/ssmctl
	GOOS=darwin  GOARCH=amd64 go build $(LDFLAGS) -o bin/ssmctl-darwin-amd64      ./cmd/ssmctl
	GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -o bin/ssmctl-darwin-arm64      ./cmd/ssmctl
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/ssmctl-windows-amd64.exe ./cmd/ssmctl
	GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o bin/ssmctl-windows-arm64.exe ./cmd/ssmctl

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install $(LDFLAGS) ./cmd/ssmctl
