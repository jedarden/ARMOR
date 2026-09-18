# ARMOR Makefile
# Targets: build, toolchain-check, test, test-integration, lint, docker, compat, test-docker-demo, dod, release, clean, help

VERSION ?= $(shell cat VERSION)

# Go toolchain: go.mod's `toolchain` directive is the single source of the
# version (the Dockerfiles pin golang:<that version>-alpine to match). Builds
# here use whatever `go` is on PATH — a newer go is fine, and `make build`
# runs toolchain-check to warn when it differs. There are deliberately no
# machine-specific toolchain paths here.
GO ?= go
TOOLCHAIN := $(shell awk '$$1 == "toolchain" { print $$2; exit }' go.mod)
CGO_ENABLED ?= 0
GOOS ?= $(shell $(GO) env GOOS)
GOARCH ?= $(shell $(GO) env GOARCH)

# LDFLAGS for version injection
LDFLAGS := -s -w -X github.com/jedarden/armor/internal/version.Version=$(VERSION)

# Directories
CMDDIR := ./cmd
TESTDIR := ./tests
BUILDDIR := ./bin

# One binary per cmd/ directory. The list is derived from the tree so it can
# never name a command that no longer exists (armor-decrypt was folded into
# `armor decrypt` on 2026-08-30 and the old hard-coded list kept building it).
BINARIES := $(notdir $(wildcard $(CMDDIR)/*))

# Docker build arguments
DOCKER_BUILD := docker build --build-arg VERSION=$(VERSION)

.PHONY: all build toolchain-check test test-integration lint docker compat test-docker-demo dod release clean help

all: build test lint

## toolchain-check: Warn when the local go version differs from go.mod's toolchain directive
toolchain-check:
	@if [ -z "$(TOOLCHAIN)" ]; then \
		echo "Toolchain check: go.mod declares no toolchain directive"; \
	elif [ "$$($(GO) env GOVERSION)" = "$(TOOLCHAIN)" ]; then \
		echo "Toolchain check: $$($(GO) env GOVERSION) (matches go.mod)"; \
	else \
		echo "WARNING: local $$($(GO) env GOVERSION) differs from go.mod toolchain $(TOOLCHAIN); building with the local toolchain (the directive only raises the floor)"; \
	fi

## build: Build every cmd/ binary into bin/ with the version injected
build: toolchain-check
	@echo "Building ARMOR binaries (version $(VERSION))..."
	@mkdir -p $(BUILDDIR)
	@for bin in $(BINARIES); do \
		echo "  Building $$bin..."; \
		CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILDDIR)/$$bin $(CMDDIR)/$$bin || exit 1; \
	done
	@echo "Build complete: $(BUILDDIR)/{$(BINARIES)}"

## test: Run go vet and unit tests (-short)
test:
	@echo "Running go vet..."
	CGO_ENABLED=$(CGO_ENABLED) $(GO) vet ./...
	@echo "Running unit tests (-short)..."
	CGO_ENABLED=$(CGO_ENABLED) $(GO) test ./... -short

## test-integration: Run integration tests (requires env/credentials)
test-integration:
	@echo "Running integration tests (requires credentials)..."
	CGO_ENABLED=$(CGO_ENABLED) $(GO) test ./... -tags integration -v

## lint: Run golangci-lint with .golangci.yml config
lint:
	@echo "Running golangci-lint..."
	golangci-lint run --config .golangci.yml

## docker: Build the server and test images, tagged with VERSION only (no floating tags)
docker: Dockerfile Dockerfile.test
	@echo "Building Docker images..."
	@echo "  Building ronaldraygun/armor:$(VERSION)..."
	$(DOCKER_BUILD) -t ronaldraygun/armor:$(VERSION) -f Dockerfile .
	@echo "  Building ronaldraygun/armor-test:$(VERSION)..."
	$(DOCKER_BUILD) -t ronaldraygun/armor-test:$(VERSION) -f Dockerfile.test .
	@echo "Docker images built successfully"

## compat: Run AWS CLI / rclone compatibility tests
compat:
	@echo "Running AWS CLI / rclone compatibility tests..."
	CGO_ENABLED=$(CGO_ENABLED) $(GO) test -v $(TESTDIR)/aws-cli-compatibility/...

## test-docker-demo: Run the README Docker demo smoke test (requires Docker)
test-docker-demo:
	@echo "Running Docker demo smoke test (requires Docker)..."
	CGO_ENABLED=$(CGO_ENABLED) $(GO) test -v $(TESTDIR)/docker-demo-smoke/...

## dod: Run the repository definition of done (fast lane: build, vet, script tests)
dod:
	scripts/definition-of-done.sh --fast

## release: Cut a release commit (VERSION bump + CHANGELOG entry). Usage: make release V=0.1.1970
release:
	@test -n "$(V)" || { echo "usage: make release V=<MAJOR.MINOR.PATCH>"; exit 2; }
	scripts/cut-release.sh $(V)

## clean: Remove build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILDDIR)
	@rm -f *.test
	@echo "Clean complete"

## help: Show this help message
help:
	@echo "ARMOR Makefile - Available targets:"
	@echo ""
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /' | while read -r line; do \
		echo "$$line"; \
	done
	@echo ""
	@echo "Variables:"
	@echo "  VERSION     - Version string (default: read from VERSION file)"
	@echo "  GO          - Go binary (default: go from PATH)"
	@echo "  CGO_ENABLED - CGO mode (default: 0)"
	@echo "  GOFLAGS     - Additional Go build flags"
