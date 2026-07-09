# Pluck Development Tools - Complete Version Reference

**Document Created:** 2026-07-09  
**Bead:** bf-4q2s0  
**Workspace:** /home/coding/ARMOR  
**Project Type:** Go (primary) + Python (utilities) + Rust (NEEDLE/Pluck tools)  
**Status:** ✅ Complete  
**Purpose:** Comprehensive reference for all Pluck development tools, versions, and constraints

## Table of Contents

1. [Overview](#overview)
2. [Quick Reference](#quick-reference)
3. [Core Pluck/NEEDLE Tools](#core-pluckneedle-tools)
4. [Build Tools](#build-tools)
5. [Testing Tools](#testing-tools)
6. [Code Quality & Linting](#code-quality--linting)
7. [Configuration Tools](#configuration-tools)
8. [Development Tools](#development-tools)
9. [Deployment Tools](#deployment-tools)
10. [Version Constraints](#version-constraints)
11. [Installation Requirements](#installation-requirements)
12. [Troubleshooting](#troubleshooting)
13. [Maintenance Schedule](#maintenance-schedule)

---

## Overview

The Pluck development environment uses a multi-language toolchain spanning Rust (NEEDLE/Pluck CLI tools), Go (ARMOR project), and Python (utilities and configuration). This document provides a complete inventory of all tools, their versions, constraints, and relationships.

### Tool Statistics

- **Total Tools:** 28 distinct tools
- **Languages:** Rust, Go, Python, Bash
- **Categories:** 9 functional categories
- **Strict Requirements:** 5 pinned versions
- **Minimum Requirements:** 8 version floors

---

## Quick Reference

### Critical Path Tools

| Tool | Version | Constraint | Purpose | Source |
|------|---------|------------|---------|--------|
| **needle** | 0.2.11 | = 0.2.11 | Pluck/NEEDLE CLI | Cargo.toml:3 |
| **br/bf** | v0.2.0 | >= v0.2.0 | Bead management | .bf-version |
| **Go** | 1.25.0 | = 1.25.0 | ARMOR builds | go.mod:3 |
| **Python** | 3.12.12 | >= 3.8+ | Config parsing | System |
| **Docker** | 27.5.1 | >= 20.10+ | Container builds | System |

### Verification Commands

```bash
# Core Pluck tools
needle --version              # Expected: needle 0.2.11
cat ~/.local/bin/.bf-version  # Expected: v0.2.0

# Build tools
go version                    # Expected: go version go1.25.0 linux/amd64
python3 --version            # Expected: Python 3.12.12
docker --version             # Expected: Docker version 27.5.1

# Testing
go test ./... -short          # Go unit tests
python3 -m unittest          # Python tests

# Linting
go vet ./...                  # Go static analysis
```

---

## Core Pluck/NEEDLE Tools

### needle CLI (Primary Pluck Interface)

| Attribute | Value |
|-----------|-------|
| **Version** | 0.2.11 (rust, linux x86_64) |
| **Minimum Required** | 0.2.11 |
| **Constraint Type** | Strict (= 0.2.11) |
| **Binary Location** | `~/.local/bin/needle` |
| **Source Repository** | https://github.com/jedarden/NEEDLE |
| **Version Source** | Cargo.toml:3, Cargo.lock |
| **Rust MSRV** | 1.75 (Minimum Supported Rust Version) |

**Key Commands:**
```bash
needle run          # Launch workers to process beads
needle stop         # Stop running workers
needle cleanup      # Remove orphaned tmux sessions
needle list         # List active workers
needle attach       # Attach to worker tmux sessions
needle status       # Show fleet status and bead counts
needle logs         # View and query telemetry logs
needle config       # View or inspect configuration
needle doctor       # Check system health and repair
needle init         # Initialize v2 config with optional v1 migration
needle version      # Show version information
needle test-agent   # Validate agent adapters
needle canary       # Run canary tests
needle upgrade      # Check for and install updates
needle rollback     # Rollback to previous stable binary
```

**Configuration Files:**
- `.needle.yaml` - Main strand configuration
- `pluck-config.yaml` - Debug configuration
- `.beads/config.yaml` - Bead store configuration

### br/bf CLI (Bead Management)

| Attribute | Value |
|-----------|-------|
| **Version** | v0.2.0 (bead-forge) |
| **Minimum Required** | v0.2.0 |
| **Constraint Type** | Minimum (>= v0.2.0) |
| **Binary Location** | `~/.local/bin/br` (symlink: `br -> bf`) |
| **Version Source** | `~/.local/bin/.bf-version` |
| **Build Tool** | bead-forge (br-compatible superset) |

**Key Commands:**
```bash
br create           # Create new beads
br update           # Update existing beads
br close            # Close completed beads
br sync             # Synchronize bead store
br sync --flush-only # Flush database to JSONL checkpoint
br doctor           # Check and repair bead database
br doctor --repair  # Repair bead database (FLUSH FIRST!)
br list             # List beads
```

**Critical Behavior Notes:**
- **SQLite database is live store**, `issues.jsonl` is checkpoint
- **MUST flush before repair:** `br sync --flush-only` before `br doctor --repair`
- Unflushed beads exist only in database and are **lost by repair**
- This is the most common cause of bead data loss

**Version Verification:**
```bash
cat ~/.local/bin/.bf-version  # Expected: v0.2.0
ls -la ~/.local/bin/br        # Should be symlink to bf
```

### Transformation Tools

#### needle-transform-claude

| Attribute | Value |
|-----------|-------|
| **Version** | Built to needle 0.2.11 |
| **Binary Location** | `~/.local/bin/needle-transform-claude` |
| **Size** | 408,872 bytes |
| **Purpose** | Claude-specific data transformations |
| **Version Source** | Cargo.toml:18, Cargo.lock |

#### needle-transform-codex

| Attribute | Value |
|-----------|-------|
| **Version** | Built to needle 0.2.11 |
| **Binary Location** | `~/.local/bin/needle-transform-codex` |
| **Size** | 415,312 bytes |
| **Purpose** | Codex-specific data transformations |
| **Version Source** | Cargo.toml:22, Cargo.lock |

---

## Build Tools

### Go Toolchain

| Tool | Version | Minimum Required | Constraint Type | Source |
|------|---------|-----------------|-----------------|--------|
| **go** | 1.25.0 linux/amd64 | 1.25.0 | Strict (= 1.25.0) | go.mod:3, Dockerfile:2, .golangci.yml:4 |
| **go build** | Built-in to 1.25.0 | 1.25.0 | Strict (= 1.25.0) | Dockerfile:22 |
| **go mod** | Built-in to 1.25.0 | 1.25.0 | Strict (= 1.25.0) | go.mod:1-13 |
| **go vet** | Built-in to 1.25.0 | 1.25.0 | Strict (= 1.25.0) | .golangci.yml:9 |
| **go test** | Built-in to 1.25.0 | 1.25.0 | Strict (= 1.25.0) | Dockerfile:19 |
| **go fmt** | Built-in to 1.25.0 | 1.25.0 | Strict (= 1.25.0) | Standard tool |

**Go Dependencies (ARMOR Project):**
```go
// From: go.mod
module github.com/jedarden/armor

go 1.25.0

require (
    github.com/aws/aws-sdk-go-v2 v1.41.4
    github.com/aws/aws-sdk-go-v2/config v1.32.12
    github.com/aws/aws-sdk-go-v2/credentials v1.19.12
    github.com/aws/aws-sdk-go-v2/service/s3 v1.97.2
    github.com/kurin/blazer v0.5.3
    golang.org/x/crypto v0.49.0
    golang.org/x/sync v0.12.0
)
```

**Rust Dependencies (NEEDLE Project):**
```toml
# From: /home/coding/NEEDLE/Cargo.toml
[package]
name = "needle"
version = "0.2.11"
rust-version = "1.75"  # MSRV

[dependencies]
tokio = "1"              # v1.52.3 in Cargo.lock
serde = "1"              # v1.0.228 in Cargo.lock
serde_json = "1"         # v1.0.150 in Cargo.lock
serde_yaml = "0.9"       # v0.9.34 in Cargo.lock
clap = "4"                # v4.6.1 in Cargo.lock
anyhow = "1"              # v1.0.103 in Cargo.lock
thiserror = "1"           # v1.0.69 in Cargo.lock
tracing = "0.1"           # v0.1.44 in Cargo.lock
chrono = "0.4"            # v0.4.45 in Cargo.lock
```

### Python Toolchain

| Tool | Version | Minimum Required | Constraint Type | Purpose |
|------|---------|-----------------|-----------------|---------|
| **python3** | 3.12.12 | 3.8+ | Minimum (>= 3.8+) | Python interpreter |
| **pip** | Bundled with 3.12.12 | 3.8+ | Minimum (>= 3.8+) | Package management |
| **unittest** | Built-in to 3.12.12 | Built-in | Standard library | Testing framework |

**Python Packages:**
```txt
# From: tools/parse_module/requirements.txt
pyyaml>=6.0      # YAML parsing library
pytest>=7.0.0    # Testing framework (optional, not installed)
```

**Installation Status:**
- ✅ Python 3.12.12 installed
- ✅ PyYAML available (minimum 6.0)
- ❌ pytest not installed (minimum 7.0.0 required)
- ✅ unittest built-in to Python stdlib

### Rust Toolchain (for NEEDLE/Pluck tools)

| Tool | Version | Minimum Required | Constraint Type | Source |
|------|---------|-----------------|-----------------|--------|
| **rustc** | 1.96.1 (2026-06-26) | 1.75 (MSRV) | Dynamic (stable channel) | rustup |
| **cargo** | 1.96.1 (bundled) | 1.75 | Dynamic (stable channel) | rustup |

**Version Verification:**
```bash
rustc --version    # Expected: rustc 1.96.1 (or newer stable)
cargo --version    # Expected: cargo 1.96.1 (or newer stable)
rustup show        # Shows active toolchain and update status
```

**Note:** Rust toolchain is **dynamic** (auto-updates via rustup). The MSRV (1.75) ensures compatibility with older compilers.

### Containerization Tools

| Tool | Version | Minimum Required | Constraint Type | Purpose | Source |
|------|---------|-----------------|-----------------|---------|--------|
| **Docker** | 27.5.1 | 20.10+ | Minimum (>= 20.10+) | Container builds | System |
| **Docker Base Image** | golang:1.25-alpine | - | Pinned | ARMOR builds | Dockerfile:2 |
| **Kaniko** | v1.23.2 | v1.23.2 | Strict (= v1.23.2) | CI container builds | iad-ci workflow |

**Docker Build Configuration:**
```dockerfile
# From: Dockerfile
FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git ca-certificates tzdata
WORKDIR /armor
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go vet ./... && CGO_ENABLED=0 go test ./... -short
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /armor ./cmd/armor

FROM alpine:latest
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /armor /armor
ENTRYPOINT ["/armor"]
```

---

## Testing Tools

### Go Testing Framework

| Tool | Version | Type | Purpose | Source |
|------|---------|------|---------|--------|
| **go test** | Built-in to 1.25.0 | Standard | Unit/integration tests | Dockerfile:19 |
| **testing** | Standard library | Standard | Test assertions | Go stdlib |
| **-short flag** | Go feature | Standard | Skip integration tests | Test execution |

**Test Categories:**
- **Unit Tests:** `*_test.go` files throughout codebase
- **Integration Tests:** `tests/integration/` (requires build tags and credentials)
- **AWS CLI Compatibility:** `tests/aws-cli-compatibility/`

**Test Execution:**
```bash
# Unit tests only (integration tests auto-skipped)
go test ./... -short

# All tests (requires credentials)
go test ./... -v

# Specific test package
go test ./tests/integration/ -v
```

**Test Pattern:** `-short` flag for unit tests, build tags for integration tests requiring credentials

### Python Testing Framework

| Tool | Version | Type | Purpose | Status |
|------|---------|------|---------|--------|
| **unittest** | Built-in to 3.12.12 | Standard | Python test framework | ✅ Available |
| **pytest** | >= 7.0.0 (not installed) | Third-party | Advanced test runner | ❌ Not installed |

**Test Files:**
- `tests/test_inventory_reader.py` - Inventory reader tests
- `tests/yamlutil/test_broken_samples.py` - YAML validation tests
- `tests/yamlutil/test_validator.py` - YAML validator tests
- `tools/parse_module/tests/test_yaml_parser.py` - YAML parser tests

**Test Execution:**
```bash
# Run Python tests with unittest
python3 -m unittest discover tests/

# Run specific test file
python3 -m unittest tests.test_inventory_reader

# Run YAML parser tests
python3 -m unittest tools.parse_module.tests.test_yaml_parser
```

---

## Code Quality & Linting

### Go Linting Tools

| Tool | Version | Status | Purpose | Configuration |
|------|---------|--------|---------|---------------|
| **go vet** | Built-in to 1.25.0 | ✅ Active | Static analysis | Standard Go vet |
| **golangci-lint** | Format v2 | ⚠️ CI-only | Comprehensive linting | .golangci.yml |
| **staticcheck** | Via golangci-lint | CI-only | Advanced analysis | .golangci.yml:11 |
| **ineffassign** | Via golangci-lint | CI-only | Detect ineffective assignments | .golangci.yml:10 |
| **unused** | Via golangci-lint | CI-only | Detect unused code | .golangci.yml:12 |

**golangci-lint Configuration** (.golangci.yml):
```yaml
version: "2"
run:
  go: "1.25"
linters:
  default: none
  enable:
    - govet          # Go static analysis
    - ineffassign    # Detect ineffectual assignments
    - staticcheck    # Go static checks
    - unused         # Detect unused code
```

**Linting Execution:**
```bash
# CI/CD pipeline
golangci-lint run

# Local equivalent (what CI runs)
go vet ./...
```

**Quality Gate:** Dockerfile enforces `go vet ./ && go test ./... -short` before build

### Python Code Quality

| Tool | Version | Status | Purpose |
|------|---------|--------|---------|
| **pylint** | Not detected | - | Python linting (not used) |
| **flake8** | Not detected | - | Style checking (not used) |
| **mypy** | Not detected | - | Type checking (not used) |

**Note:** Python code uses standard library conventions without additional linting tools.

---

## Configuration Tools

### YAML and Configuration Parsing

| Tool | Language | Purpose | Location |
|------|----------|---------|----------|
| **YAML Parser** | Python | YAML parsing and validation | tools/parse_module/yaml_parser.py |
| **Config Parser** | Python | Multi-format config parsing | tools/config_parser/parse_configs.py |
| **Inventory Reader** | Python | Debug file inventory | scripts/debug-config-parser/inventory.py |

**Configuration Files:**
| File | Purpose |
|------|---------|
| `pluck-config.yaml` | Pluck debug configuration |
| `.needle.yaml` | NEEDLE strand configuration |
| `.beads/config.yaml` | Bead store configuration |
| `.golangci.yml` | Linting configuration |

**Python Dependencies:**
```txt
pyyaml>=6.0       # YAML parsing
pytest>=7.0.0     # Testing (optional, not installed)
```

**Usage in Pluck:**
- Configuration parsing (`.needle.yaml`, `pluck-config.yaml`)
- Debug file inventory management
- Log rotation and redirection testing
- YAML syntax validation

---

## Development Tools

### Version Control

| Tool | Version | Purpose | Status |
|------|---------|---------|--------|
| **Git** | 2.50.1 | Version control | ✅ Installed |

**Git Usage:**
- Source control for ARMOR project
- Build dependency in Dockerfile
- Remote: GitHub (jedarden/ARMOR)

### Scripting Utilities

| Script | Purpose | Language |
|--------|---------|----------|
| `execute-pluck-*.sh` | Execute Pluck with bead-specific config | Bash |
| `test-pluck-*.sh` | Test Pluck behavior | Bash |
| `validate-pluck-syntax*.sh` | Validate Pluck configuration syntax | Bash |
| `capture-pluck-debug.sh` | Capture Pluck debug logs | Bash |
| `analyze-pluck-debug.sh` | Analyze Pluck debug output | Bash |

**Total Shell Scripts:** 20+ scripts for validation, monitoring, and testing

---

## Deployment Tools

### Kubernetes Tools

| Tool | Version | Minimum Required | Constraint Type | Purpose | Status |
|------|---------|-----------------|-----------------|---------|--------|
| **kubectl** | v1.33.3 | 1.20+ | Minimum (>= 1.20+) | Kubernetes cluster management | ✅ Installed |
| **kustomize** | v5.6.0 (built-in to kubectl) | 3.0+ | Minimum (>= 3.0+) | Kubernetes customization | ✅ Available |

**Kubernetes Resources:**
| Resource | Source File |
|----------|-------------|
| Deployment | deploy/kubernetes/deployment.yaml |
| Service | deploy/kubernetes/service.yaml |
| Ingress | deploy/kubernetes/ingress-dashboard.yaml |
| Kustomization | deploy/kubernetes/kustomization.yaml |

**Deployment Commands:**
```bash
# Apply Kubernetes manifests
kubectl apply -k deploy/kubernetes/

# Check deployment status
kubectl get pods -l app=armor

# View logs
kubectl logs -l app=armor -f
```

### CI/CD Pipeline

**Argo Workflows (iad-ci cluster):**
| Attribute | Value |
|-----------|-------|
| **Cluster** | iad-ci |
| **Namespace** | argo-workflows |
| **WorkflowTemplate** | armor-build |
| **Access** | `kubectl --server=http://traefik-iad-ci:8001` |

**ArgoCD (ardenone-manager cluster):**
| Attribute | Value |
|-----------|-------|
| **Cluster** | ardenone-manager |
| **Purpose** | GitOps deployment management |
| **Repository** | jedarden/declarative-config |
| **Access** | Read-only proxy at `https://argocd-ro-ardenone-manager-ts.ardenone.com:8444` |

### Container Registry

| Registry | Image | Purpose |
|----------|-------|---------|
| **Docker Hub** | ronaldraygun/armor:<VERSION> | Public container images |
| **Versioning** | From VERSION file | Auto-bumped by CI pipeline |

**Current Version:** 0.1.373 (from `VERSION` file)

---

## Version Constraints

### Strict Requirements (Pinned Versions)

| Tool | Constraint | Rationale | Source |
|------|------------|-----------|--------|
| **needle** | = 0.2.11 | Current stable version | Cargo.toml:3 |
| **Go** | = 1.25.0 | Dockerfile and go.mod consistency | go.mod:3, Dockerfile:2 |
| **Docker base** | golang:1.25-alpine | Go version matching | Dockerfile:2 |
| **Kaniko** | = v1.23.2 | CI/CD container builds | iad-ci workflow |
| **br/bf** | >= v0.2.0 | Bead tracking compatibility | .bf-version |

### Minimum Requirements

| Tool | Minimum | Source |
|------|---------|--------|
| **Python** | 3.8+ | PyYAML and pytest compatibility |
| **PyYAML** | 6.0 | Configuration parsing |
| **pytest** | 7.0.0 | Python testing (optional) |
| **kubectl** | 1.20+ | Kubernetes API compatibility |
| **kustomize** | 3.0+ | Resource customization |
| **Docker** | 20.10+ | Multi-stage builds, COPY --from |
| **Rust (MSRV)** | 1.75 | NEEDLE compilation |

### Flexible Requirements

| Tool | Status | Notes |
|------|--------|-------|
| **Git** | No minimum | Any recent version |
| **Docker engine** | 27.5.1 installed | No strict minimum |
| **unittest** | Built-in | Part of Python stdlib |

### Dynamic Version Flags

| Component | Update Frequency | Risk Level | Mitigation |
|-----------|-----------------|------------|------------|
| **Rust toolchain (stable)** | Every 6 weeks | Medium | MSRV 1.75 ensures compatibility |
| **GitHub runners (ubuntu-latest)** | Weekly updates | Medium | Tests catch environment breaks |
| **GitHub Actions (@vX)** | Bug fixes, security patches | Low | Major version pinned |

---

## Installation Requirements

### Complete Setup Requirements

To reproduce the complete Pluck development environment:

1. **Install Core Pluck Tools:**
   - needle 0.2.11
   - br/bf v0.2.0
   - Transformation tools

2. **Install Go Environment:**
   - Go 1.25.0
   - golangci-lint (optional)

3. **Install Python Environment:**
   - Python 3.12.12
   - PyYAML >= 6.0
   - pytest >= 7.0.0 (optional)

4. **Install Container Tools:**
   - Docker 27.5.1
   - kubectl (for deployment)

5. **Configure Workspace:**
   - `.needle.yaml` for strand configuration
   - `pluck-config.yaml` for debug configuration
   - `.beads/config.yaml` for bead store

### Quick Environment Check Script

```bash
#!/bin/bash
echo "=== Pluck Development Tools Version Check ==="
echo ""
echo "Core Pluck Tools:"
echo "  needle: $(needle --version 2>&1 || echo 'Not installed')"
echo "  br/bf: $(cat ~/.local/bin/.bf-version 2>/dev/null || echo 'Not installed')"
echo ""
echo "Build Tools:"
echo "  Go: $(go version 2>&1 | head -1)"
echo "  Python: $(python3 --version 2>&1)"
echo "  Docker: $(docker --version 2>&1)"
echo "  Rust: $(rustc --version 2>&1 || echo 'Not installed')"
echo ""
echo "Testing Tools:"
echo "  go test: $(go version 2>&1 | head -1)"
echo "  unittest: $(python3 -c 'import unittest; print(unittest.__version__)' 2>&1)"
echo ""
echo "Linting Tools:"
echo "  go vet: $(go version 2>&1 | head -1)"
echo "  golangci-lint: $(golangci-lint --version 2>&1 || echo 'Not installed locally')"
echo ""
echo "Deployment Tools:"
echo "  kubectl: $(kubectl version --client 2>&1 | grep 'Client Version' || echo 'Not installed')"
echo ""
echo "Version Control:"
echo "  Git: $(git --version 2>&1)"
```

---

## Troubleshooting

### Common Pluck Tool Issues

**needle version mismatch:**
```bash
# Check version
needle --version

# Update if available
needle upgrade
```

**br/bf database corruption:**
```bash
# CRITICAL: Flush before repair
br sync --flush-only

# Check database integrity
sqlite3 .beads/beads.db "PRAGMA integrity_check;"

# Repair database
br doctor --repair
```

**Configuration parsing errors:**
```bash
# Verify Python YAML installation
python3 -c "import yaml; print(yaml.__version__)"

# Test configuration syntax
needle config validate
```

### Build/Tool Issues

**Go version mismatch:**
```bash
# Verify version matches go.mod
go version
# Should output: go version go1.25.0 linux/amd64
```

**Docker build failures:**
```bash
# Verify base image exists
docker pull golang:1.25-alpine

# Check Go modules
go mod download

# Clean build
docker system prune -f
docker build -t armor:test .
```

**Rust compilation errors:**
```bash
# Check Rust toolchain version
rustc --version

# Update if needed
rustup update stable

# Clean rebuild
cd /home/coding/NEEDLE
cargo clean
cargo build --release
```

---

## Maintenance Schedule

### Regular Updates Needed

| Frequency | Task | Purpose |
|-----------|------|---------|
| **Weekly** | `rustup check` | Check for Rust stable updates |
| **Monthly** | Review GitHub Actions releases | Update pinned @vX references |
| **Monthly** | Check Go updates | Security and feature updates |
| **Monthly** | Check Python updates | Security patches |
| **Quarterly** | `cargo update` | Update Rust dependencies |
| **Quarterly** | `go get -u ./...` | Update Go dependencies |
| **Quarterly** | Review Docker base images | Security and optimization |
| **As Needed** | `needle upgrade` | Check for Pluck updates |
| **As Needed** | Update kubectl | Cluster compatibility |

### Dependency Update Procedures

**Core Pluck Tools:**
1. Check `needle upgrade` for new versions
2. Review release notes for breaking changes
3. Test compatibility with current configuration
4. Update transformation tools if needed
5. Verify br/bf compatibility

**Go Environment:**
1. Update `go.mod` version line
2. Update `Dockerfile` base image tag
3. Update `.golangci.yml` go version
4. Run `go mod tidy` for dependencies
5. Test thoroughly before committing

**Python Environment:**
1. Update `requirements.txt` minimum versions
2. Test configuration parsing with new versions
3. Verify YAML compatibility
4. Run Python test suite

---

## Related Documentation

### Project Documentation
- **[development-tools.md](development-tools.md)** - ARMOR project development tools
- **[pluck-development-tools.md](pluck-development-tools.md)** - NEEDLE/Pluck Rust tools
- **[pluck-development-tools-complete.md](pluck-development-tools-complete.md)** - Complete version inventory
- **[pluck-tools-categorization.md](pluck-tools-categorization.md)** - Tool categories
- **[pluck-tools-version-sources.md](pluck-tools-version-sources.md)** - Version sources

### Configuration Files
- **[pluck-config.yaml](../pluck-config.yaml)** - Debug configuration
- **[.needle.yaml](../.needle.yaml)** - Strand configuration
- **[go.mod](../go.mod)** - Go module definition
- **[.golangci.yml](../.golangci.yml)** - Linting configuration

### External Documentation
- **NEEDLE Repository:** https://github.com/jedarden/NEEDLE
- **ARMOR Repository:** https://github.com/jedarden/ARMOR
- **Go 1.25 Release Notes:** https://go.dev/doc/go1.25
- **PyYAML Documentation:** https://pyyaml.org/wiki/PyYAMLDocumentation
- **Kubernetes Documentation:** https://kubernetes.io/docs/

---

## Summary

### Key Takeaways

**Core Pluck Stack:**
- **needle CLI:** 0.2.11 (rust, linux x86_64)
- **br/bf CLI:** v0.2.0 (bead management)
- **Transform tools:** Built to needle 0.2.11

**Supporting Tools:**
- **Go:** 1.25.0 (strict requirement)
- **Python:** 3.12.12 with PyYAML >= 6.0
- **Docker:** 27.5.1 with golang:1.25-alpine base
- **Kaniko:** v1.23.2 for CI/CD builds
- **Rust:** 1.96.1 (dynamic, MSRV 1.75)

**Development Workflow:**
- **Configuration:** YAML-based with Python validation
- **Build:** Go 1.25.0 with static linking
- **Testing:** Go built-in + Python unittest
- **Deployment:** Argo Workflows + ArgoCD

**Version Strategy:**
- **Strict:** needle 0.2.11, Go 1.25.0, Kaniko v1.23.2
- **Minimum:** br/bf v0.2.0, PyYAML 6.0, Python 3.8+
- **Flexible:** Git, Docker engine, unittest

### Version Compatibility Matrix

| Tool Version | Pluck 0.2.11 | ARMOR 0.1.373 | Notes |
|--------------|--------------|---------------|-------|
| needle 0.2.11 | ✅ Compatible | ✅ Compatible | Current stable |
| br/bf v0.2.0+ | ✅ Compatible | ✅ Compatible | Minimum required |
| Go 1.25.0 | N/A | ✅ Required | Strict requirement |
| Python 3.12.x | ✅ Compatible | ✅ Compatible | Config parsing |
| Docker 27.x | ✅ Compatible | ✅ Compatible | Container builds |
| Rust 1.96.1 | ✅ Compatible | N/A | NEEDLE build tool |

---

**Document Status:** ✅ Complete  
**Next Review Date:** When needle version updates or major tool changes  
**Maintained By:** ARMOR Development Team  
**File Location:** `/home/coding/ARMOR/docs/pluck-tools-complete-version-reference.md`
