# ARMOR Makefile
# Targets: build, test, test-integration, lint, docker, compat

VERSION ?=$(shell cat VERSION)

# Nix Go installation (NOS/NixOS environment)
NIX_GO := /nix/store/3yh8g6m3balbhzxsx77jlyyx443bxqii-go-1.26.6/share/go/bin/go
export PATH := $(dir $(NIX_GO)):$(PATH)
GO := go
CGO_ENABLED ?= 0
GOOS ?= $(shell $(NIX_GO) env GOOS)
GOARCH ?= $(shell $(NIX_GO) env GOARCH)

# LDFLAGS for version injection
LDFLAGS := -s -w -X github.com/jedarden/armor/internal/version.Version=$(VERSION)

# Directories
CMDDIR := ./cmd
TESTDIR := ./tests
BUILDDIR := ./bin

# Binaries to build
BINARIES := armor armor-decrypt armor-fleet restore-verifier verify-objects

# Docker build arguments
DOCKER_BUILD := docker build --build-arg VERSION=$(VERSION)

.PHONY: all build test test-integration lint docker compat clean help

all: build test lint

## build: Build all cmd/ binaries with version injection
build:
	@echo "Building ARMOR binaries (version $(VERSION))..."
	@mkdir -p $(BUILDDIR)
	@for bin in $(BINARIES); do \
		echo "  Building $$bin..."; \
		CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILDDIR)/$$bin $(CMDDIR)/$$bin; \
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

## docker: Build both Dockerfiles (armor and test images)
docker: Dockerfile Dockerfile.test
	@echo "Building Docker images..."
	@echo "  Building ronaldraygun/armor:$(VERSION)..."
	$(DOCKER_BUILD) -t ronaldraygun/armor:$(VERSION) -f Dockerfile .
	$(DOCKER_BUILD) -t ronaldraygun/armor:latest -f Dockerfile .
	@echo "  Building ronaldraygun/armor-test:$(VERSION)..."
	$(DOCKER_BUILD) -t ronaldraygun/armor-test:$(VERSION) -f Dockerfile.test .
	$(DOCKER_BUILD) -t ronaldraygun/armor-test:latest -f Dockerfile.test .
	@echo "Docker images built successfully"

## compat: Run AWS CLI / rclone compatibility tests
compat:
	@echo "Running AWS CLI / rclone compatibility tests..."
	CGO_ENABLED=$(CGO_ENABLED) $(GO) test -v $(TESTDIR)/aws-cli-compatibility/...

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
	@echo "  VERSION    - Version string (default: read from VERSION file)"
	@echo "  CGO_ENABLED - CGO mode (default: 0)"
	@echo "  GOFLAGS    - Additional Go build flags"
