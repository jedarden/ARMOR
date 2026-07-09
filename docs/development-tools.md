# Development Tool Versions - ARMOR Project

**Last Updated:** 2026-07-09  
**Project Version:** 0.1.336  
**Document Purpose:** Capture development tool versions for reproducibility and dependency management

## Overview

The ARMOR project uses a Go-based development stack with supporting tools for linting, containerization, and configuration management. This document tracks the current versions of all development tools and where their versions are specified.

## Core Development Tools

### Go Toolchain

| Tool | Version | Specification Location |
|------|---------|----------------------|
| Go | 1.25.0 | `go.mod` line 3, `.golangci.yml` line 4, `Dockerfile` line 2 |
| go vet | Built-in to Go 1.25.0 | Standard Go toolchain |
| go test | Built-in to Go 1.25.0 | Standard Go toolchain |
| go mod | Built-in to Go 1.25.0 | Standard Go toolchain |

**System Version:** `go version go1.25.0 linux/amd64`

**Go Modules Specification:** `go.mod`
```go
module github.com/jedarden/armor

go 1.25.0
```

### Linting Tools

| Tool | Version | Specification Location |
|------|---------|----------------------|
| golangci-lint | "2" (format version) | `.golangci.yml` line 1 |
| govet | Built-in | `.golangci.yml` line 9 |
| ineffassign | Built-in | `.golangci.yml` line 10 |
| staticcheck | External dependency | `.golangci.yml` line 11 |
| unused | Built-in | `.golangci.yml` line 12 |

**golangci-lint Configuration:** `.golangci.yml`
```yaml
version: "2"
run:
  go: "1.25"
linters:
  default: none
  enable:
    - govet
    - ineffassign
    - staticcheck
    - unused
```

**Note:** golangci-lint is not currently installed on the development system. The configuration exists but the tool is not in PATH.

### Containerization Tools

| Tool | Version | Specification Location |
|------|---------|----------------------|
| Docker | 27.5.1 | System Docker installation |
| Docker Base Image | golang:1.25-alpine | `Dockerfile` line 2 |

**Docker Base Image Specification:** `Dockerfile`
```dockerfile
FROM golang:1.25-alpine AS builder
```

**System Version:** `Docker version 27.5.1, build v27.5.1`

### Testing Frameworks

| Tool | Version | Specification Location |
|------|---------|----------------------|
| Go Testing | Built-in to Go 1.25.0 | Standard library `testing` package |
| Build Tags | Go 1.25.0 feature | Test files use `//go:build integration` |

**Test Structure:**
- Unit tests: Standard `go test` with `-short` flag
- Integration tests: Build tag `integration` requires credentials
- Test location: `tests/integration/` directory
- AWS CLI compatibility tests: `tests/aws-cli-compatibility/`

### Python Tools

| Tool | Version | Usage | Specification Location |
|------|---------|-------|----------------------|
| Python 3 | 3.12.12 | Configuration parsing | System Python installation |
| PyYAML | Not specified in project | YAML parsing | `tools/config_parser/parse_configs.py` |
| Standard Library | 3.12.12 | JSON/TOML parsing | Standard library |

**System Version:** `Python 3.12.12`

**Python Usage:** The project includes a configuration parser tool for ARMOR debug infrastructure:
- `tools/config_parser/parse_configs.py` - Configuration file validation
- `tools/config_parser/parse_configs.sh` - Shell wrapper

### Build and Development Tools

| Tool | Version | Usage | Specification Location |
|------|---------|-------|----------------------|
| CGO | Disabled (0) | Cross-compilation | `Dockerfile` lines 19, 22 |
| Git | System version | Build dependency | `Dockerfile` line 7 |
| ca-certificates | Alpine package | SSL/TLS | `Dockerfile` line 7 |
| tzdata | Alpine package | Timezone data | `Dockerfile` line 7 |

**Build Configuration:** `Dockerfile`
```dockerfile
RUN apk add --no-cache git ca-certificates tzdata
RUN CGO_ENABLED=0 go vet ./... && CGO_ENABLED=0 go test ./... -short
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /armor ./cmd/armor
```

## Major Go Dependencies

| Dependency | Version | Specification Location |
|------------|---------|----------------------|
| AWS SDK v2 | v1.41.4 | `go.mod` line 6 |
| AWS Config v2 | v1.32.12 | `go.mod` line 7 |
| AWS Credentials v2 | v1.19.12 | `go.mod` line 8 |
| AWS S3 v2 | v1.97.2 | `go.mod` line 9 |
| Blazer (GCS) | v0.5.3 | `go.mod` line 10 |
| golang.org/x/crypto | v0.49.0 | `go.mod` line 11 |
| golang.org/x/sync | v0.12.0 | `go.mod` line 12 |

**Dependency Specification:** `go.mod` and `go.sum`
- Primary specifications: `go.mod`
- Checksums and indirect dependencies: `go.sum`

## Development Infrastructure

### Version Control
- Git: System version (used in Docker build)
- Remote: GitHub (github.com/jedarden/armor)

### CI/CD
- GitHub Actions: Not currently configured (no `.github/` directory found)
- Argo Workflows: CI/CD runs on `iad-ci` cluster (per project documentation)
- ArgoCD: Deployments managed via `declarative-config` repository

### Container Registry
- Docker Hub: `ronaldraygun/armor` (per project documentation)
- Image Build: `armor-build` WorkflowTemplate in `iad-ci` cluster

## Tool Installation and Management

### Go Tools Installation
Go tools are managed via Go modules and installed as needed:
```bash
# Standard Go tools (built-in)
go vet
go test
go mod

# External tools would be installed via:
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

### System Package Installation
System tools are managed via Alpine package manager in Docker:
```bash
apk add --no-cache git ca-certificates tzdata
```

### Python Package Installation
Python packages are managed via pip (not currently specified in project):
```bash
pip install pyyaml  # For YAML parsing
```

## Version Pinning Strategy

### Strictly Pinned
- **Go version:** 1.25.0 (pinned in go.mod, Dockerfile, and golangci-lint config)
- **Go dependencies:** Exact versions in go.mod with checksums in go.sum
- **Docker base image:** golang:1.25-alpine (pinned in Dockerfile)

### System-Provided
- **Docker:** System installation (27.5.1)
- **Python:** System installation (3.12.12)
- **Git:** System installation (version not specified)

### Not Currently Pinned
- **golangci-lint:** Configuration exists but tool not installed
- **Python packages:** No requirements.txt or similar file found
- **Node.js tools:** Not applicable to this Go project

## Reproducibility Notes

### Build Environment
- **Go version:** Strictly pinned to 1.25.0
- **CGO:** Disabled for static binaries
- **Target OS:** Linux (GOOS=linux)
- **Build optimization:** Size-optimized (`-ldflags="-s -w"`)

### Development Environment
To reproduce the development environment:
1. Install Go 1.25.0
2. Install Docker 27.x ( Alpine-based images)
3. Install Python 3.12.x with PyYAML
4. Clone repository and run `go mod download`
5. Optionally install golangci-lint for linting

### Container Build
The Docker build ensures reproducibility by:
- Using pinned golang:1.25-alpine base image
- Disabling CGO for static linking
- Running tests before building (`go test ./... -short`)
- Building static binaries for minimal dependencies

## Maintenance and Updates

### Regular Updates Needed
1. **Go version:** Update go.mod, Dockerfile, and .golangci.yml together
2. **AWS SDK:** Regular updates for security and features
3. **Docker base image:** Pull latest Alpine security updates
4. **System packages:** Update via apk when security patches released

### Version Update Process
When updating Go or major dependencies:
1. Update `go.mod` version line
2. Update `Dockerfile` base image tag
3. Update `.golangci.yml` go version
4. Run `go mod tidy` to update dependencies
5. Update `go.sum` with new checksums
6. Test thoroughly with new versions
7. Update this document

## Troubleshooting

### Common Version Issues

**Go version mismatch:**
- Ensure `go version` matches `go.mod` (1.25.0)
- Update .golangci.yml if updating Go version
- Rebuild Docker image after version changes

**Docker build failures:**
- Verify golang:1.25-alpine image exists
- Check CGO_ENABLED=0 for static linking
- Ensure go.mod and go.sum are synchronized

**golangci-lint not found:**
- Tool is configured but not installed
- Install via: `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`
- Ensure PATH includes `$(go env GOPATH)/bin`

## Additional Resources

### Project Documentation
- **README:** `/home/coding/ARMOR/README.md` - Project overview and setup
- **PROGRESS.md:** `/home/coding/ARMOR/PROGRESS.md` - Development progress tracking
- **Integration Tests:** `/home/coding/ARMOR/tests/integration/README.md`
- **AWS CLI Tests:** `/home/coding/ARMOR/tests/aws-cli-compatibility/README.md`

### External Documentation
- Go 1.25 Release Notes: https://go.dev/doc/go1.25
- golangci-lint: https://golangci-lint.run/
- AWS SDK for Go v2: https://aws.github.io/aws-sdk-go-v2/docs/
- Docker Alpine: https://hub.docker.com/_/golang/tags

---

**Document Status:** ✅ Complete  
**Next Review Date:** When Go version updates or major dependency changes  
**Maintained By:** Project development team  
**File Location:** `/home/coding/ARMOR/docs/development-tools.md`