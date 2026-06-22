.PHONY: build test test-unit test-integration lint fmt tidy release-snapshot release

BINARY := bin/cfl
PKG := ./...

# Version metadata injected at build time (see .goreleaser.yaml for release builds).
VERSION ?= dev
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
	-X github.com/addozhang/cfl/internal/cli.version=$(VERSION) \
	-X github.com/addozhang/cfl/internal/cli.commit=$(COMMIT) \
	-X github.com/addozhang/cfl/internal/cli.date=$(DATE)

build:
	go build -ldflags '$(LDFLAGS)' -o $(BINARY) ./cmd/cfl

test:
	go test $(PKG) -race -cover

test-unit:
	go test ./internal/... -race -cover

test-integration:
	go test ./test/integration/... -race

lint:
	golangci-lint run $(PKG)

fmt:
	gofmt -s -w .
	goimports -w .

tidy:
	go mod tidy

release-snapshot:
	goreleaser release --snapshot --clean

# Local release is forbidden; this target exists only to be run in CI.
release:
	@echo "release runs in CI only; do not run locally" && exit 1
