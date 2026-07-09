# Development Tool Versions - ARMOR

**Bead:** bf-62zau  
**Date:** 2026-07-09  
**Project:** ARMOR (Authenticated Range-readable Managed Object Repository)

## Overview

This document captures the current versions of all development tools used in the ARMOR project. ARMOR is a Go-based S3-compatible proxy server that provides transparent encryption for Backblaze B2 storage with Cloudflare egress optimization.

## Core Development Tools

### Go (Golang)
- **Required Version:** 1.25.0 (specified in `go.mod`)
- **Installed Version:** 1.25.0 linux/amd64
- **Purpose:** Primary programming language for ARMOR
- **Source Citation:** `go.mod:3`

### Docker
- **Installed Version:** 27.5.1
- **Base Image Used:** golang:1.25-alpine
- **Purpose:** Container builds and multi-stage deployment
- **Build Configuration:** `Dockerfile`

### golangci-lint
- **Configured Version:** 1.25 (in `.golangci.yml`)
- **Installed:** Not currently in PATH on this development machine
- **Purpose:** Go linting and static analysis
- **Enabled Linters:**
  - `govet` - Go vet static analysis
  - `ineffassign` - Detect ineffectual assignments
  - `staticcheck` - Go static analysis
  - `unused` - Detect unused code
- **Source Citation:** `.golangci.yml`

### Git
- **Installed Version:** 2.50.1
- **Purpose:** Version control
- **Remote:** github.com/jedarden/armor

## Testing Framework

### Go Testing
- **Framework:** Built-in Go testing (`go test`)
- **Test Locations:**
  - Unit tests: Throughout codebase (standard `*_test.go` files)
  - Integration tests: `tests/integration/` (requires build tags and credentials)
- **Test Command:** `CGO_ENABLED=0 go test ./... -short` (used in Docker build)
- **Source Citation:** `Dockerfile:19`

## Kubernetes Deployment

### Kubernetes
- **Access Method:** kubectl-proxy over Tailscale (read-only)
- **Purpose:** Production deployment orchestration
- **Deployment Files:** `deploy/kubernetes/`
  - `deployment.yaml` - Main deployment manifest
  - `service.yaml` - Service definitions
  - `ingress-dashboard.yaml` - Ingress configuration
  - `secret.yaml` - Secret management
  - `kustomization.yaml` - Kustomize configuration

### ArgoCD
- **Purpose:** GitOps continuous deployment
- **Access:** Via ardenone-manager cluster
- **Status:** ARMOR is managed via declarative-config repository

## Build Tools

### go build
- **Build Command:** `CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /armor ./cmd/armor`
- **Flags:**
  - `CGO_ENABLED=0` - Disable CGO for static binary
  - `GOOS=linux` - Target Linux
  - `-ldflags="-s -w"` - Strip debug information for smaller binary
- **Binary Location:** `/armor` (in container)

## Dependency Management

### Go Modules
- **Module Path:** `github.com/jedarden/armor`
- **Key Dependencies:**
  - `github.com/aws/aws-sdk-go-v2` v1.41.4 - AWS S3 SDK v2
  - `github.com/kurin/blazer` v0.5.3 - Google Cloud Storage client
  - `golang.org/x/crypto` v0.49.0 - Cryptographic primitives
  - `golang.org/x/sync` v0.12.0 - Concurrency primitives
- **Source Citation:** `go.mod`

## Current Application Version

- **ARMOR Version:** 0.1.343
- **Container Image:** ronaldraygun/armor:0.1.343
- **Version File:** `/home/coding/ARMOR/VERSION`
- **CI Pipeline:** Auto-bumps VERSION file on every build and publishes to Docker Hub

## Development Workflow

### Local Development
1. **Build:** `go build ./cmd/armor`
2. **Test:** `go test ./... -short` (excludes integration tests)
3. **Lint:** `golangci-lint run` (when installed)
4. **Vet:** `go vet ./...`

### Docker Build
1. **Stage 1 (Builder):** golang:1.25-alpine
   - Downloads dependencies
   - Runs `go vet` and `go test -short`
   - Builds static binary
2. **Stage 2 (Runtime):** scratch
   - Copies only CA certs, timezone data, and binary
   - Minimal attack surface

### CI/CD
- **Platform:** Argo Workflows (iad-ci cluster)
- **Template:** `armor-build`
- **Registry:** Docker Hub (ronaldraygun/armor)
- **GitOps:** ArgoCD on ardenone-manager cluster

## Tool Requirements Summary

| Tool | Required Version | Installed Version | Purpose |
|------|-----------------|-------------------|---------|
| Go | 1.25.0 | 1.25.0 | Core language |
| Docker | Latest | 27.5.1 | Container builds |
| golangci-lint | Compatible with Go 1.25 | Not in PATH | Linting |
| Git | Any recent | 2.50.1 | Version control |
| Kubernetes | N/A (via proxy) | N/A | Deployment |

## Notes

1. **No kubectl in PATH** - Kubernetes access is via kubectl-proxy over Tailscale, not direct kubeconfig
2. **golangci-lint not installed** - Configured in project but not currently available on this development machine
3. **Integration tests require credentials** - Automatically excluded by `-short` flag in CI builds
4. **Multi-stage Docker build** - Ensures minimal runtime image with only necessary dependencies
5. **Static binary** - CGO_ENABLED=0 produces portable binary without external dependencies

## Verification

To verify current tool versions:

```bash
# Check Go version
go version

# Check Docker version  
docker --version

# Check golangci-lint (if installed)
golangci-lint version

# Check current ARMOR version
cat VERSION
```

---
**Document Status:** Complete  
**Last Updated:** 2026-07-09  
**Bead Status:** Ready for commit
