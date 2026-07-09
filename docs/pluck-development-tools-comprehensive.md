# Pluck Development Tools - Comprehensive Documentation

**Document Created:** 2026-07-09  
**Bead:** bf-4q2s0  
**Workspace:** /home/coding/ARMOR  
**Status:** ✅ Complete  
**Compilation Sources:** bf-195e3, bf-4qcfn, bf-5riod, bf-2p935

---

## Overview

This document provides a comprehensive reference for all development tools used in the Pluck project within the ARMOR workspace. It compiles categorization, version information, constraints, and requirements into a single, navigable resource.

**What is Pluck?**
Pluck is a strand within the NEEDLE system (Navigates Every Enqueued Deliverable, Logs Effort). It handles primary bead selection from assigned workspaces and processes >90% of all bead operations.

**Key Context:** Pluck is NOT a standalone project - it is a component of NEEDLE. The dependencies listed include the full NEEDLE system and ARMOR workspace tools.

---

## Quick Reference: Critical Tools

| Tool | Version | Minimum | Purpose | Source |
|------|---------|---------|---------|--------|
| **needle** | 0.2.11 | - | NEEDLE CLI and Pluck strand | Binary |
| **br CLI** | 0.2.0 | - | Bead management CLI | Binary |
| **Go** | 1.25.0 | 1.25.0 | ARMOR build toolchain | go.mod |
| **Rust** | 1.96.1 | 1.75 (MSRV) | NEEDLE build toolchain | Cargo.toml |
| **Docker** | 27.5.1 | 20.10+ | Container builds | System |
| **kubectl** | v1.33.3 | 1.20+ | Kubernetes management | System |
| **Python** | 3.12.12 | 3.8+ | Test utilities | System |

---

## Table of Contents

1. [Tool Categories](#tool-categories)
2. [Build Tools](#build-tools)
3. [Testing Tools](#testing-tools)
4. [Linting Tools](#linting-tools)
5. [Formatting Tools](#formatting-tools)
6. [Configuration Tools](#configuration-tools)
7. [Development Tools](#development-tools)
8. [Deployment Tools](#deployment-tools)
9. [Version Control](#version-control)
10. [NEEDLE/Pluck Rust Dependencies](#needlepluck-rust-dependencies)
11. [ARMOR Go Dependencies](#armor-go-dependencies)
12. [Minimum Version Requirements](#minimum-version-requirements)
13. [Platform Support](#platform-support)
14. [Installation Requirements](#installation-requirements)
15. [Verification Commands](#verification-commands)

---

## Tool Categories

| Category | Tool Count | Primary Tools | Purpose |
|----------|------------|---------------|---------|
| **Build Tools** | 4 | Go, Python, Docker, Kaniko | Compiling and building |
| **Testing Tools** | 3 | go test, unittest, pytest | Quality assurance |
| **Linting Tools** | 5 | go vet, golangci-lint, staticcheck, ineffassign, unused | Code quality |
| **Formatting Tools** | 1 | go fmt | Code standardization |
| **Configuration Tools** | 2 | YAML parsers, Config parsers | Config management |
| **Development Tools** | 3 | needle, br/bf CLI, Git | Workflow management |
| **Deployment Tools** | 3 | kubectl, kustomize, Docker Hub | Deployment operations |

**Total Tools:** 21+ across 7 categories

---

## Build Tools

### Go Toolchain

| Tool | Current Version | Minimum | Purpose | Source |
|------|----------------|---------|---------|--------|
| **go** | go1.25.0 linux/amd64 | 1.25.0 | Go compiler and toolchain | go.mod:3 |
| **go mod** | (bundled with go1.25.0) | 1.25.0 | Dependency management | go.mod |
| **go build** | (bundled with go1.25.0) | 1.25.0 | Build Go binaries | Dockerfile |
| **go test** | (bundled with go1.25.0) | 1.25.0 | Run Go tests | Built-in |
| **go vet** | (bundled with go1.25.0) | 1.25.0 | Static analysis | Built-in |

**Version Constraints:**
- **Minimum:** 1.25.0 (required by go.mod)
- **Current:** 1.25.0
- **Status:** ✅ Compliant
- **Source:** `/home/coding/ARMOR/go.mod` line 3

**Verification:**
```bash
go version
# Expected output: go version go1.25.0 linux/amd64
```

### Rust Toolchain

| Tool | Current Version | Minimum | Purpose | Source |
|------|----------------|---------|---------|--------|
| **rustc** | 1.96.1 (2026-06-26) | 1.75 (MSRV) | Rust compiler | Cargo.toml:5 |
| **cargo** | 1.96.1 (bundled) | 1.75 (implied) | Package manager | rust-toolchain.toml |
| **rustfmt** | 1.96.1 (bundled) | Not specified | Code formatter | rust-toolchain.toml |
| **clippy** | 0.1.96 | Not specified | Rust linter | rust-toolchain.toml |

**Version Constraints:**
- **MSRV:** 1.75 (released 2023-12-28)
- **Current:** 1.96.1
- **Status:** ✅ Compliant
- **Dynamic:** ⚠️ YES - Rust stable updates every 6 weeks
- **Source:** `/home/coding/NEEDLE/Cargo.toml` line 5

**Verification:**
```bash
rustc --version
# Expected output: rustc 1.96.1 (or newer stable)
```

### Python Toolchain

| Tool | Current Version | Minimum | Purpose | Source |
|------|----------------|---------|---------|--------|
| **python3** | Python 3.12.12 | 3.8+ | Python interpreter | System |
| **pip** | (bundled with python3) | 3.8+ | Package management | System |
| **unittest** | (standard library) | - | Testing framework | Built-in |

**Version Constraints:**
- **Minimum:** Python 3.8+ (inferred from modern syntax usage)
- **Current:** Python 3.12.12
- **Status:** ✅ Compliant
- **Purpose:** Test utilities, YAML parsing, configuration validation

**Verification:**
```bash
python3 --version
# Expected output: Python 3.12.12
```

### Container Tools

| Tool | Current Version | Minimum | Purpose | Source |
|------|----------------|---------|---------|--------|
| **docker** | Docker 27.5.1 | 20.10+ | Container building and running | System |
| **docker build** | (bundled with Docker) | 20.10+ | Build container images | System |
| **Kaniko** | v1.23.2 | - | Container image builds in CI | CI/CD Pipeline |

**Version Constraints:**
- **Docker Minimum:** 20.10+ for multi-stage builds and COPY --from syntax
- **Base Image:** golang:1.25-alpine (Docker Hub)
- **Current:** Docker 27.5.1
- **Status:** ✅ Compliant

**Verification:**
```bash
docker --version
# Expected output: Docker version 27.5.1, build v27.5.1
```

---

## Testing Tools

### Go Testing Framework

| Tool | Version | Type | Purpose | Source |
|------|---------|------|---------|--------|
| **go test** | (bundled with go1.25.0) | Standard | Unit tests | Built-in |
| **testing** | (standard library) | Standard | Test assertions | Built-in |
| **-short flag** | (go test feature) | Standard | Skip integration tests | Built-in |

**Test Categories:**
- **Unit Tests:** `*_test.go` files throughout codebase
- **Integration Tests:** `tests/integration/` (requires build tags and credentials)
- **Compatibility Tests:** `tests/aws-cli-compatibility/` (AWS CLI compatibility)

**Test Execution:**
```bash
# Unit tests only (integration tests auto-skipped)
go test ./... -short

# All tests (requires credentials)
go test ./... -v
```

### Python Testing Framework

| Tool | Version | Type | Purpose | Source |
|------|---------|------|---------|--------|
| **unittest** | (standard library) | Standard | Python test framework | Built-in |
| **pytest** | 7.0.0+ (optional) | Third-party | Test runner | Inferred |

**Test Files:**
- `tests/test_inventory_reader.py` - Inventory reader tests
- `tests/yamlutil/test_broken_samples.py` - YAML validation tests
- `tests/yamlutil/test_validator.py` - YAML validator tests
- `tests/yamlutil/verify_implementation.py` - Implementation verification

**Test Execution:**
```bash
python3 -m unittest discover tests/
```

### Specialized Test Tools

| Tool | Purpose | Location |
|------|---------|----------|
| **AWS CLI test script** | AWS CLI compatibility validation | `tests/aws-cli-compatibility/test-aws-cli.sh` |
| **Integration test suite** | S3-compatible behavior verification | `tests/integration/` |
| **YAML validation tests** | Configuration file validation | `tests/yamlutil/` |

---

## Linting Tools

### Go Linting

| Tool | Version | Status | Purpose | Configuration |
|------|---------|--------|---------|---------------|
| **go vet** | (bundled with go1.25.0) | ✅ Active | Static analysis | Standard Go vet |
| **golangci-lint** | Not installed locally | ⚠️ CI-only | Comprehensive linting | `.golangci.yml` |

**golangci-lint Configuration** (`.golangci.yml`):
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

**Enabled Linters:**
- **govet:** Standard Go static analysis (similar to go vet)
- **ineffassign:** Detect ineffectual assignments
- **staticcheck:** Comprehensive static analysis
- **unused:** Detect unused code, constants, variables, functions, and types

**Linting Execution:**
```bash
# In CI/CD pipeline
golangci-lint run

# Local equivalent
go vet ./...
```

### Python Code Quality

| Tool | Version | Status | Purpose |
|------|---------|--------|---------|
| **pylint** | Not detected | - | Python linting (not used) |
| **flake8** | Not detected | - | Style checking (not used) |
| **mypy** | Not detected | - | Type checking (not used) |

**Note:** Python code appears to use standard library conventions without additional linting tools.

---

## Formatting Tools

### Code Formatting

| Tool | Version | Source File | Purpose |
|------|---------|-------------|---------|
| **go fmt** | 1.25.0 (built-in) | Standard Go tool | Code formatting |

**Formatting Configuration:**
- **Go fmt:** Standard Go formatting rules
- **Python:** No explicit formatters (uses standard conventions)

---

## Configuration Tools

### YAML and Configuration Parsing

| Tool | Language | Source File | Purpose |
|------|----------|-------------|---------|
| **YAML Parser** | Python | `tools/parse_module/yaml_parser.py` | YAML parsing and validation |
| **Config Parser** | Python | `tools/config_parser/parse_configs.py` | Multi-format config parsing |
| **Inventory Reader** | Python | `scripts/debug-config-parser/inventory.py` | Debug file inventory |

**Configuration Files:**

| File | Purpose |
|------|---------|
| `pluck-config.yaml` | Pluck debug configuration |
| `.needle.yaml` | NEEDLE strand configuration |
| `.beads/config.yaml` | Bead store configuration |
| `.golangci.yml` | Linting configuration |
| `go.mod` | Go module configuration |
| `Cargo.toml` | Rust package configuration |

---

## Development Tools

### NEEDLE/Pluck CLI Tools

| Tool | Version | Source Location | Purpose |
|------|---------|-----------------|---------|
| **needle** | 0.2.11 | `~/.local/bin/needle` | NEEDLE CLI and Pluck strand |
| **br/bf CLI** | v0.2.0 | `~/.local/bin/br` | Bead management CLI |
| **pluck** | (part of needle) | `needle strand pluck` | Bead selection strand |

**NEEDLE Components:**
- **Pluck Strand:** Primary bead selection from workspace
- **Bead Store:** SQLite-based bead tracking (`.beads/beads.db`)
- **JSONL Checkpoint:** Bead state persistence (`.beads/issues.jsonl`)

**needle Key Commands:**
- `needle run` - Launch workers to process beads
- `needle stop` - Stop running workers
- `needle cleanup` - Remove orphaned tmux sessions
- `needle list` - List active workers
- `needle logs` - View telemetry logs
- `needle doctor` - Check system health
- `needle version` - Show version information

**br/bf CLI Key Commands:**
- `br create` - Create new beads
- `br update` - Update existing beads
- `br close` - Close completed beads
- `br sync` - Synchronize bead store
- `br doctor` - Check and repair database

**Version Verification:**
```bash
needle --version
# Expected: needle 0.2.11

br --version
# Expected: Error: bf 0.2.0 (error format artifact)
```

### Scripting Utilities

| Script | Purpose | Language |
|--------|---------|----------|
| `execute-pluck-*.sh` | Execute Pluck with bead-specific config | Bash |
| `test-pluck-*.sh` | Test Pluck behavior | Bash |
| `validate-pluck-syntax*.sh` | Validate Pluck configuration syntax | Bash |
| `capture-pluck-debug.sh` | Capture Pluck debug logs | Bash |
| `analyze-pluck-debug.sh` | Analyze Pluck debug output | Bash |

---

## Deployment Tools

### Kubernetes Tools

| Tool | Version | Purpose | Status |
|------|---------|---------|--------|
| **kubectl** | v1.33.3 | Kubernetes cluster management | ✅ Installed |
| **kustomize** | v5.6.0 | Kubernetes customization | ✅ Available (bundled) |

**Kubernetes Resources:**
- Deployment: `deploy/kubernetes/deployment.yaml`
- Service: `deploy/kubernetes/service.yaml`
- Ingress: `deploy/kubernetes/ingress-dashboard.yaml`
- Kustomization: `deploy/kubernetes/kustomization.yaml`

**Deployment Commands:**
```bash
# Apply Kubernetes manifests
kubectl apply -k deploy/kubernetes/

# Check deployment status
kubectl get pods -l app=armor

# View logs
kubectl logs -l app=armor -f
```

### Container Registry

| Registry | Image | Purpose |
|----------|-------|---------|
| **Docker Hub** | ronaldraygun/armor:<VERSION> | Public container images |
| **Versioning** | From VERSION file | Auto-bumped by CI pipeline |

**Current Version:** 0.1.373 (from `VERSION` file)

---

## Version Control

### Git Tools

| Tool | Version | Purpose | Status |
|------|---------|---------|--------|
| **git** | 2.50.1 | Version control | ✅ Installed |
| **github** | (via git remote) | Remote repository hosting | ✅ Configured |

**Git Configuration:**
- Repository format version: 0
- File mode: true (Unix permissions)
- Remote: GitHub (jedarden/ARMOR)

---

## NEEDLE/Pluck Rust Dependencies

### Core Runtime Dependencies (24 packages)

| Package | Exact Version | Minimum Constraint | Purpose | Dynamic |
|---------|---------------|-------------------|---------|---------|
| **tokio** | v1.52.3 | ^1.0.0 | Async runtime with full features | ❌ |
| **serde** | v1.0.228 | ^1.0.0 | Serialization framework with derive | ❌ |
| **serde_json** | v1.0.150 | ^1.0.0 | JSON serialization | ❌ |
| **serde_yaml** | v0.9.34+deprecated | ^0.9.0 | YAML serialization | ❌ |
| **clap** | v4.6.1 | ^4.0.0 | CLI framework with derive | ❌ |
| **anyhow** | v1.0.103 | ^1.0.0 | Error handling | ❌ |
| **thiserror** | v1.0.69 | ^1.0.0 | Error derivation | ❌ |
| **tracing** | v0.1.44 | ^0.1.0 | Structured logging | ❌ |
| **tracing-subscriber** | v0.3.23 | ^0.3.0 | Log filtering with env-filter, json | ❌ |
| **chrono** | v0.4.45 | ^0.4.0 | Time handling with serde | ❌ |
| **which** | v4.4.2 | ^4.0.0 | Executable discovery in PATH | ❌ |
| **async-trait** | v0.1.89 | ^0.1.0 | Async trait support | ❌ |
| **fs2** | v0.4.3 | ^0.4.0 | Cross-platform file locking (flock) | ❌ |
| **sha2** | v0.10.9 | ^0.10.0 | SHA-2 hashing for content hash | ❌ |
| **hex** | v0.4.3 | ^0.4.0 | Hex encoding for fingerprinting | ❌ |
| **regex** | v1.12.4 | ^1.0.0 | Regular expressions (token extraction) | ❌ |
| **glob** | v0.3.3 | ^0.3.0 | Glob pattern matching (discovery) | ❌ |
| **ureq** | v2.12.1 | ^2.0.0 | Simple HTTP client (self-update) | ❌ |
| **aho-corasick** | v1.1.4 | ^1.0.0 | Multi-pattern string search | ❌ |
| **cfg-if** | v1.0.4 | ^1.0.0 | Conditional compilation | ❌ |
| **atty** | v0.2.14 | ^0.2.0 | Terminal detection (ANSI support) | ❌ |
| **toml** | v0.8.23 | ^0.8.0 | TOML parsing (gitleaks config) | ❌ |
| **libc** | v0.2.186 | ^0.2.0 | Unix process handling (PID check) | ❌ |
| **rand** | v0.8.6 | ^0.8.0 | Random jitter (desynchronization) | ❌ |

**Source:** `/home/coding/NEEDLE/Cargo.toml` lines 42-101, `/home/coding/NEEDLE/Cargo.lock`

**Version Constraint Pattern:** Caret requirements (^) permit backward-compatible updates

### OpenTelemetry Dependencies (Optional, feature-gated)

**Feature:** `otlp` (default feature)

| Package | Exact Version | Minimum Constraint | Purpose | Dynamic |
|---------|---------------|-------------------|---------|---------|
| **opentelemetry** | v0.31.0 | ^0.31.0 | OpenTelemetry API | ❌ |
| **opentelemetry_sdk** | v0.31.0 | ^0.31.0 | OTLP SDK with rt-tokio | ❌ |
| **opentelemetry-otlp** | v0.31.1 | ^0.31.0 | OTLP exporter with grpc-tonic, http-proto | ❌ |
| **opentelemetry-semantic-conventions** | v0.31.0 | ^0.31.0 | Semantic conventions | ❌ |
| **tonic** | v0.14.6 | ^0.14.0 | gRPC for OTLP | ❌ |
| **tracing-opentelemetry** | v0.32.1 | ^0.32.0 | Tracing integration | ❌ |

**Total OpenTelemetry Dependencies:** 6  
**Feature Gate:** `--features otlp` required for compilation  
**Source:** `/home/coding/NEEDLE/Cargo.toml` lines 104-111

### Development Dependencies (8 packages)

| Package | Exact Version | Minimum Constraint | Purpose |
|---------|---------------|-------------------|---------|
| **tokio-test** | v0.4.5 | ^0.4.0 | Tokio testing utilities |
| **tempfile** | v3.27.0 | ^3.0.0 | Temporary file handling |
| **proptest** | v1.11.0 | ^1.0.0 | Property-based testing |
| **filetime** | v0.2.29 | ^0.2.0 | File time manipulation |
| **criterion** | v0.5.1 | ^0.5.0 | Benchmarking |
| **futures** | v0.3.32 | ^0.3.0 | Async utilities |
| **gethostname** | v0.4.3 | ^0.4.0 | Hostname detection |
| **testcontainers** | v0.23.3 | ^0.23.0 | Docker container integration testing |

**Source:** `/home/coding/NEEDLE/Cargo.toml` lines 112-123

---

## ARMOR Go Dependencies

### Direct Go Dependencies (7 packages)

| Package | Version | Purpose | Source |
|---------|---------|---------|--------|
| **github.com/aws/aws-sdk-go-v2** | v1.41.4 | AWS SDK core | go.mod:6 |
| **github.com/aws/aws-sdk-go-v2/config** | v1.32.12 | AWS configuration | go.mod:7 |
| **github.com/aws/aws-sdk-go-v2/credentials** | v1.19.12 | AWS credentials | go.mod:8 |
| **github.com/aws/aws-sdk-go-v2/service/s3** | v1.97.2 | S3 storage | go.mod:9 |
| **github.com/kurin/blazer** | v0.5.3 | Google Cloud Storage | go.mod:10 |
| **golang.org/x/crypto** | v0.49.0 | Cryptography | go.mod:11 |
| **golang.org/x/sync** | v0.12.0 | Concurrency | go.mod:12 |

**Total Go Direct Dependencies:** 7  
**Source Files:** `/home/coding/ARMOR/go.mod`, `/home/coding/ARMOR/go.sum`

---

## Minimum Version Requirements

### Critical Path Tools

| Tool | Minimum | Current | Buffer | Status |
|------|---------|---------|--------|--------|
| **Go** | 1.25.0 | 1.25.0 | Exact | ✅ Compliant |
| **Rust (MSRV)** | 1.75 | 1.96.1 | 21 versions | ✅ Compliant |
| **Python** | 3.8+ | 3.12.12 | 4 major versions | ✅ Compliant |
| **Docker** | 20.10+ | 27.5.1 | 7 major versions | ✅ Compliant |
| **kubectl** | 1.20+ | 1.33.3 | 13 minor versions | ✅ Compliant |
| **needle** | - | 0.2.11 | - | ✅ Installed |
| **br CLI** | - | 0.2.0 | - | ✅ Installed |

### Optional Tools

| Tool | Minimum | Current | Status |
|------|---------|---------|--------|
| **golangci-lint** | 2.0+ | Not installed locally | ⚠️ CI-only |
| **pytest** | 7.0+ | Not verified | ⚠️ Inferred usage |

### System Dependencies

**Linux (Debian/Ubuntu) Minimum Required System Packages:**

| Package | Purpose | Source |
|---------|---------|--------|
| **git** | Version control system | install.sh |
| **curl** | HTTP client for downloads | install.sh |
| **jq** | JSON processor for output parsing | install.sh |
| **build-essential** | C compiler and build tools | Dependency compilation |
| **pkg-config** | Package configuration helper | Dependency compilation |
| **libssl-dev** | OpenSSL development headers | ureq dependency |

**Source:** `/home/coding/NEEDLE/install.sh` and dependency requirements

---

## Platform Support

### Supported Platforms

| Platform | Architecture | Status | Source |
|----------|-------------|--------|--------|
| **Linux** | x86_64 (amd64) | ✅ Primary Target | rust-toolchain.toml |
| **Linux** | aarch64 (ARM64) | ✅ Supported | rust-toolchain.toml |
| **macOS** | x86_64 | ✅ Supported | Cross-compilation |
| **macOS** | ARM64 (aarch64-apple-darwin) | ✅ Supported | rust-toolchain.toml |
| **Windows** | x86_64 | ⚠️ Limited Support | Not officially supported |

**Source:** `/home/coding/NEEDLE/rust-toolchain.toml` targets

---

## Installation Requirements

### Development Environment Setup

**Required Tools:**
```bash
# Go 1.25.0
go version

# Python 3.8+
python3 --version

# Docker 20.10+
docker --version

# Git 2.0+
git --version

# kubectl 1.20+ (for deployment)
kubectl version --client
```

**Optional Tools:**
```bash
# NEEDLE/Pluck (for bead-based development)
curl -fsSL https://github.com/jedarden/NEEDLE/releases/latest/download/install.sh | bash

# golangci-lint (for local linting)
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh
```

### Pre-built Binary Installation (Recommended)

**Requirements:**
- `curl` or `wget` for downloading
- `sha256sum` or `shasum` for checksum verification
- `gpg` (optional) for signature verification

**Installation Command:**
```bash
curl -fsSL https://github.com/jedarden/NEEDLE/releases/latest/download/install.sh | bash
```

### Build from Source Installation

**Requirements:**
- Rust 1.75+ toolchain
- Cargo package manager
- System build tools (gcc, make, etc.)
- OpenSSL development headers (libssl-dev)

**Installation Command:**
```bash
cargo install --git https://github.com/jedarden/NEEDLE
```

### CI/CD Environment

**Pipeline Requirements:**
- Go 1.25.0
- Docker 20.10+
- golangci-lint 2.0+
- kubectl 1.20+ (for deployment stages)

---

## Verification Commands

### Quick Environment Check

```bash
#!/bin/bash
echo "=== Pluck Development Tools Version Check ==="
echo ""
echo "Build Tools:"
echo "  Go: $(go version 2>&1 | head -1)"
echo "  Rust: $(rustc --version 2>&1)"
echo "  Python: $(python3 --version 2>&1)"
echo "  Docker: $(docker --version 2>&1)"
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
echo "  kubectl: $(kubectl version --client 2>&1 | grep 'Client Version')"
echo "  Kustomize: $(kubectl version --client 2>&1 | grep 'Kustomize Version')"
echo ""
echo "Development Tools:"
echo "  Git: $(git --version 2>&1)"
echo "  NEEDLE: $(needle --version 2>&1 || echo 'Not installed')"
echo "  br CLI: $(br --version 2>&1 || echo 'Not installed')"
```

### Verify Pluck Versions

```bash
# Core tools
needle --version                    # Expected: needle 0.2.11
br --version                        # Expected: Error: bf 0.2.0

# Check Cargo.lock is up to date
cd /home/coding/NEEDLE
cargo tree --depth 1                # Should match Cargo.toml constraints

# Verify no uncommitted dependency changes
git diff Cargo.lock                 # Should be empty
```

### Verify Toolchain Versions

```bash
# Rust toolchain (dynamic)
rustc --version                     # Expected: 1.96.1 (or newer stable)
rustup show                         # Shows active toolchain and update status

# Go toolchain (static)
go version                          # Expected: go version go1.25.0

# Docker (static)
docker --version                    # Expected: Docker version 27.5.1

# Git (static)
git --version                       # Expected: git version 2.50.1
```

### Check for Dynamic Version Updates

```bash
# Check if Rust stable has updates
rustup check                        # Shows available updates

# Check GitHub Actions for new releases
# Visit: https://github.com/actions/checkout/releases
# Visit: https://github.com/dtolnay/rust-toolchain/releases
```

---

## Compliance Status Summary

### Core Requirements Compliance

| Category | Minimum Required | Current Installed | Status | Source |
|----------|-----------------|-------------------|--------|--------|
| **Rust** | 1.75 | 1.96.1 | ✅ Compliant | Cargo.toml |
| **Go** | 1.25.0 | 1.25.0 | ✅ Compliant | go.mod |
| **br CLI** | 0.2.0 | 0.2.0 | ✅ Compliant | Bead store |
| **needle** | - | 0.2.11 | ✅ Installed | Binary |
| **tokio** | ^1 | v1.52.3 | ✅ Compliant | Cargo.toml |
| **serde** | ^1 | v1.0.228 | ✅ Compliant | Cargo.toml |
| **OpenTelemetry** | ^0.31 | v0.31.0 | ✅ Compliant | Cargo.toml |

### Overall Assessment

- **Compliance Rate:** 100%
- **Critical Issues:** 0
- **Missing Dependencies:** 0
- **Below-Minimum Versions:** 0

✅ **ALL REQUIREMENTS MET** - All dependencies meet or exceed minimum version requirements.

---

## Dependency Update Procedures

### Update Rust Dependencies

```bash
cd /home/coding/NEEDLE

# Update all dependencies to latest compatible versions
cargo update

# Update specific dependency
cargo update -p tokio

# Regenerate Cargo.lock after Cargo.toml changes
cargo generate-lockfile

# Commit updated lockfile
git add Cargo.lock
git commit -m "chore: update Rust dependencies"
```

### Update Go Dependencies

```bash
cd /home/coding/ARMOR

# Update all dependencies
go get -u ./...
go mod tidy

# Update specific dependency
go get -u github.com/aws/aws-sdk-go-v2@latest

# Commit updated dependencies
git add go.mod go.sum
git commit -m "chore: update Go dependencies"
```

### Update Python Tools

```bash
# Update PyYAML
pip3 install --upgrade pyyaml

# Update pytest (if used)
pip3 install --upgrade pytest
```

---

## Maintenance Schedule

### Regular Maintenance

| Frequency | Task | Purpose |
|-----------|------|---------|
| **Weekly** | `rustup check` | Check for Rust stable updates |
| **Monthly** | Review GitHub Actions releases | Update pinned @vX references |
| **Quarterly** | `cargo update` | Update Rust dependencies |
| **Quarterly** | `go get -u ./...` | Update Go dependencies |
| **As Needed** | Pin dynamic CI versions | Reproducibility after breaks |
| **Quarterly** | Review Docker base images | Security and optimization |
| **As Needed** | Update kubectl | Cluster compatibility |

---

## Security Considerations

### High-Priority Tools

- **Go 1.25.0** - Monitor for security updates
- **Python 3.12.12** - Security patches via system updates
- **Docker 27.5.1** - Container security patches
- **kubectl 1.33.3** - Cluster access security
- **Rust 1.96.1** - Monitor for security advisories

### Security Scanning

```bash
# Go dependency audit
go list -json -m all | nancy sleuth

# Docker image scan
docker scan ronaldraygun/armor:<version>

# Kubernetes manifest validation
kubectl apply -k deploy/kubernetes/ --dry-run=server

# Cargo.lock verification
cd /home/coding/NEEDLE
cargo fetch                    # Downloads with checksum verification
cargo build                     # Fails if checksums don't match

# Go.sum verification
cd /home/coding/ARMOR
go mod verify                  # Verifies go.sum against downloaded modules
```

---

## Troubleshooting

### Common Issues

**Issue: Go version mismatch**
```bash
# Check current version
go version

# Update if needed
# See: https://go.dev/dl/
```

**Issue: Python module import errors**
```bash
# Install PyYAML
pip3 install pyyaml

# Verify installation
python3 -c "import yaml; print(yaml.__version__)"
```

**Issue: Docker build failures**
```bash
# Check Docker version
docker --version

# Verify base image availability
docker pull golang:1.25-alpine

# Clean build
docker system prune -f
docker build -t armor:test .
```

**Issue: kubectl connection errors**
```bash
# Check kubectl version
kubectl version --client

# Verify cluster connection
kubectl cluster-info

# Check context
kubectl config current-context
```

---

## Configuration File Cross-Reference

### NEEDLE Configuration Files

| File | Purpose | Key Requirements |
|------|---------|------------------|
| `Cargo.toml` | Rust package configuration | rust-version = "1.75" |
| `rust-toolchain.toml` | Rust toolchain specification | channel = "stable" |
| `CLAUDE.md` | Project conventions | MSRV 1.75 documentation |
| `README.md` | Project documentation | Installation requirements |
| `install.sh` | Installation script | System package requirements |

### ARMOR Configuration Files

| File | Purpose | Key Requirements |
|------|---------|------------------|
| `go.mod` | Go module configuration | Go 1.25.0 |
| `go.sum` | Dependency checksums | Integrity verification |
| `.golangci.yml` | Linting configuration | golangci-lint v2 |
| `Dockerfile` | Container build | golang:1.25-alpine |

---

## References

### Internal Documentation

- **Pluck Tools Categorization:** `/home/coding/ARMOR/docs/pluck-tools-categorization.md`
- **Pluck Development Tools Version Inventory:** `/home/coding/ARMOR/docs/pluck-development-tools-version-inventory.md`
- **Pluck Tools Version Sources:** `/home/coding/ARMOR/docs/pluck-tools-version-sources.md`
- **Pluck Minimum Version Requirements:** `/home/coding/ARMOR/docs/pluck-minimum-version-requirements.md`
- **ARMOR Version Inventory:** `/home/coding/ARMOR/docs/comprehensive-version-inventory.md`

### Source Files

- **NEEDLE Cargo.toml:** `/home/coding/NEEDLE/Cargo.toml`
- **NEEDLE Cargo.lock:** `/home/coding/NEEDLE/Cargo.lock`
- **ARMOR go.mod:** `/home/coding/ARMOR/go.mod`
- **ARMOR go.sum:** `/home/coding/ARMOR/go.sum`
- **CI Workflows:** `/home/coding/NEEDLE/.github/workflows/`

### External Resources

- **NEEDLE Repository:** https://github.com/jedarden/NEEDLE
- **ARMOR Repository:** https://github.com/jedarden/ARMOR
- **Go Downloads:** https://go.dev/dl/
- **Rust MSRV Policy:** https://rust-lang.github.io/rfcs/2495-min-rust-version.html
- **Cargo Semantic Versioning:** https://doc.rust-lang.org/cargo/semver.html
- **Kubernetes Documentation:** https://kubernetes.io/docs/
- **Docker Documentation:** https://docs.docker.com/

---

## Acceptance Criteria Verification

✅ **All tool categories identified:** 7 categories defined (Build, Test, Lint, Format, Config, Dev, Deploy)  
✅ **Version information for each tool:** All 21+ tools documented with current versions  
✅ **Constraints and requirements included:** Minimum requirements, version constraints, and dependencies documented  
✅ **Clear, readable documentation:** Organized with table of contents, tables, and code examples  
✅ **Ready for docs/ folder:** Formatted as Markdown, comprehensive reference  
✅ **Review for completeness:** All tools from previous beads compiled and cross-referenced  

---

**Document Status:** ✅ Complete  
**Tools Documented:** 21+ across 7 categories  
**Dependencies Documented:** 24 Rust + 7 Go + system tools  
**Last Updated:** 2026-07-09  
**Next Review:** 2026-10-09 (Quarterly)  
**Maintained By:** ARMOR Development Team
