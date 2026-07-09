# Pluck Development Tools Version Inventory

**Document Created:** 2026-07-09  
**Bead:** bf-4qcfn  
**Workspace:** /home/coding/ARMOR  
**Status:** ✅ Complete

## Overview

This document provides a comprehensive inventory of all development tools used in the Pluck project, including current versions, version constraints, and tool categorization. Pluck is a strand within the NEEDLE system that processes beads from assigned workspaces.

---

## Development Tools Categories

### 1. Build Tools

| Tool | Version | Minimum Required | Purpose | Location |
|------|---------|-----------------|---------|----------|
| **Go** | 1.25.0 | 1.25.0 | Primary build toolchain | go.mod, Dockerfile |
| **Docker** | 27.5.1 | - | Container builds | Dockerfile |
| **Cargo** | 1.96.1 | 1.75+ | Rust package manager (for NEEDLE) | ~/.cargo/bin/ |

**Build Tool Details:**

- **Go 1.25.0**: Exact version requirement specified in go.mod
  - Multi-stage Docker builds use `golang:1.25-alpine` base image
  - Required for ARMOR workspace compilation
  
- **Docker 27.5.1**: Latest stable version
  - Multi-stage builds for optimized images
  - Build stage uses golang:1.25-alpine
  - Runtime stage uses scratch for minimal footprint

- **Cargo 1.96.1**: Rust package manager
  - Required for NEEDLE/Pluck system compilation
  - MSRV (Minimum Supported Rust Version): 1.75

### 2. Test Frameworks

| Tool | Version | Minimum Required | Purpose | Location |
|------|---------|-----------------|---------|----------|
| **Go test** | (built-in to Go 1.25.0) | - | Go testing framework | tests/ directory |
| **pytest** | >= 7.0.0 | 7.0.0 | Python testing framework | tools/parse_module/requirements.txt |
| **Python unittest** | (built-in to Python 3.12.12) | - | Python unit testing | tests/test_inventory_reader.py |

**Test Framework Details:**

- **Go test**: Built into Go 1.25.0
  - Integration tests in `tests/integration/`
  - Unit tests in `internal/*/` packages
  - Test gate in Dockerfile: `CGO_ENABLED=0 go test ./... -short`

- **pytest >= 7.0.0**: Python testing framework
  - Used for YAML validation utilities
  - Test files in `tests/yamlutil/`
  - Inventory reader tests

- **Python unittest**: Built into Python 3.12.12
  - Used for test_inventory_reader.py
  - Standard library testing framework

### 3. Linters and Code Quality Tools

| Tool | Version | Minimum Required | Purpose | Configuration |
|------|---------|-----------------|---------|---------------|
| **golangci-lint** | 2 (latest) | - | Go linting aggregator | .golangci.yml |
| **go vet** | (built-in to Go 1.25.0) | - | Go static analysis | Dockerfile test gate |
| **govet** | (via golangci-lint) | - | Go vet wrapper | .golangci.yml |
| **ineffassign** | (via golangci-lint) | - | Ineffective assignment detection | .golangci.yml |
| **staticcheck** | (via golangci-lint) | - | Go static checking | .golangci.yml |
| **unused** | (via golangci-lint) | - | Unused code detection | .golangci.yml |
| **rustfmt** | 1.96.1-stable | - | Rust code formatter | NEEDLE toolchain |
| **clippy** | 0.1.96 | - | Rust linter | NEEDLE toolchain |

**Linter Configuration:**

**golangci-lint (.golangci.yml):**
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

**Dockerfile Test Gate:**
```dockerfile
# Test gate: run go vet and unit tests before building
RUN CGO_ENABLED=0 go vet ./... && CGO_ENABLED=0 go test ./... -short
```

### 4. Package Managers

| Tool | Version | Purpose | Configuration Files |
|------|---------|---------|---------------------|
| **go mod** | (Go 1.25.0) | Go dependency management | go.mod, go.sum |
| **cargo** | 1.96.1 | Rust dependency management | Cargo.toml, Cargo.lock |
| **pip** | (Python 3.12.12) | Python package management | tools/parse_module/requirements.txt |

### 5. Development Languages

| Language | Version | Minimum Required | Purpose |
|----------|---------|-----------------|---------|
| **Go** | 1.25.0 | 1.25.0 | Primary development language |
| **Python** | 3.12.12 | 3.10+ (recommended) | Utilities and testing |
| **Rust** | 1.96.1 | 1.75 (MSRV) | NEEDLE/Pluck system |

### 6. Version Control Tools

| Tool | Version | Purpose |
|------|---------|---------|
| **Git** | 2.50.1 | Version control |

### 7. Container and Deployment Tools

| Tool | Version | Purpose | Configuration |
|------|---------|---------|--------------|
| **Docker** | 27.5.1 | Container builds | Dockerfile |
| **Kubernetes** | - | Container orchestration | deploy/kubernetes/ |
| **Kustomize** | - | Kubernetes configuration management | deploy/kubernetes/kustomization.yaml |

### 8. Utility Libraries

| Library | Version | Minimum Required | Purpose | Location |
|---------|---------|-----------------|---------|----------|
| **pyyaml** | >= 6.0 | 6.0 | YAML parsing for Python utilities | tools/parse_module/requirements.txt |

### 9. NEEDLE/Pluck Integration Tools

| Tool | Version | Purpose | Binary Location |
|------|---------|---------|------------------|
| **NEEDLE CLI** | 0.2.11 | Bead management system | ~/.local/bin/needle |
| **br CLI (bead-forge)** | 0.2.0 | Bead store operations | ~/.local/bin/br |

---

## Version Constraints and Requirements

### Critical Version Constraints

1. **Go 1.25.0**: Exact version requirement
   - Specified in go.mod: `go 1.25.0`
   - Required for ARMOR workspace
   - Docker builds use golang:1.25-alpine
   - No compatibility testing with other versions

2. **Python 3.10+**: Minimum recommended
   - Current: 3.12.12
   - Required for utility scripts and testing
   - pytest requires Python 3.8+

3. **Rust 1.75+**: Minimum Supported Rust Version (MSRV)
   - Current: 1.96.1
   - Required for NEEDLE/Pluck system
   - Substantial version buffer available

4. **pytest >= 7.0.0**: Python testing
   - Minimum version for test utilities
   - Current version meets requirement

### Version Compatibility Matrix

| Component | Current Version | Minimum Required | Status | Buffer |
|-----------|----------------|-------------------|--------|--------|
| Go | 1.25.0 | 1.25.0 | ✅ Exact match | 0 |
| Python | 3.12.12 | 3.10+ | ✅ Compliant | +2.12 |
| Rust | 1.96.1 | 1.75 | ✅ Compliant | +21.1 |
| pytest | >= 7.0.0 | 7.0.0 | ✅ Compliant | Current |
| pyyaml | >= 6.0 | 6.0 | ✅ Compliant | Current |

---

## Installation and Setup

### Build Tools Installation

**Go 1.25.0:**
```bash
# Verify installation
go version
# Expected output: go version go1.25.0 linux/amd64
```

**Docker:**
```bash
# Verify installation
docker --version
# Expected output: Docker version 27.5.1 (or later)
```

**Rust/Cargo (for NEEDLE):**
```bash
# Install via rustup
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --default-toolchain stable

# Verify installation
rustc --version
cargo --version
```

### Test Framework Installation

**pytest (Python):**
```bash
# Install from requirements
pip install -r tools/parse_module/requirements.txt

# Verify installation
pytest --version
# Expected: pytest 7.0.0 or later
```

### Linters Installation

**golangci-lint:**
```bash
# Install (if desired - configured but optional)
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin

# Verify installation
golangci-lint --version
```

**Rust linters (installed via rustup):**
```bash
# Add components via rustup
rustup component add rustfmt clippy

# Verify installation
rustfmt --version
cargo clippy --version
```

---

## Usage Examples

### Build Commands

**Go Application:**
```bash
# Build ARMOR
go build ./...

# Build with specific output
go build -o armor ./cmd/armor

# Run tests
go test ./... -short

# Run with race detection
go test ./... -race -short
```

**Docker Build:**
```bash
# Build container image
docker build -t armor:latest .

# Build with specific tag
docker build -t armor:0.1.373 .
```

**Rust (NEEDLE/Pluck):**
```bash
# Build NEEDLE
cd /path/to/NEEDLE
cargo build --release

# Run tests
cargo test

# Format code
cargo fmt

# Run linter
cargo clippy --all-targets -- -D warnings
```

### Test Commands

**Go Tests:**
```bash
# Run all tests (short mode)
go test ./... -short

# Run specific test package
go test ./internal/config -short

# Run with coverage
go test ./... -short -cover

# Run integration tests (requires credentials)
go test ./tests/integration
```

**Python Tests:**
```bash
# Run pytest
pytest tests/

# Run specific test file
pytest tests/test_inventory_reader.py

# Run YAML validation tests
pytest tests/yamlutil/

# Run with verbose output
pytest tests/ -v
```

### Lint Commands

**Go Linting:**
```bash
# Run go vet
go vet ./...

# Run golangci-lint (if installed)
golangci-lint run

# Run staticcheck (if installed)
staticcheck ./...
```

**Rust Linting:**
```bash
# Run clippy
cargo clippy --all-targets -- -D warnings

# Format code
cargo fmt

# Check formatting
cargo fmt --check
```

---

## Development Workflow

### Pre-commit Checklist

1. **Build Check:**
   ```bash
   go build ./...
   ```

2. **Test Gate:**
   ```bash
   CGO_ENABLED=0 go test ./... -short
   ```

3. **Lint Check:**
   ```bash
   go vet ./...
   ```

4. **Python Tests:**
   ```bash
   pytest tests/
   ```

### Continuous Integration

**Dockerfile Test Gate:**
The Dockerfile includes a test gate that must pass before building:
```dockerfile
RUN CGO_ENABLED=0 go vet ./... && CGO_ENABLED=0 go test ./... -short
```

**Local CI Emulation:**
```bash
# Run the same checks as Docker build
CGO_ENABLED=0 go vet ./... && CGO_ENABLED=0 go test ./... -short
```

---

## Tool Upgrade Policy

### Version Pinning

**Pinned Versions:**
- Go: Exactly 1.25.0 (go.mod requirement)
- Python: 3.10+ minimum (3.12.12 current)
- Rust: 1.75+ MSRV (1.96.1 current)

**Minimum Version Requirements:**
- pytest: >= 7.0.0
- pyyaml: >= 6.0

### Upgrade Procedure

**Go Dependencies:**
```bash
# Check for updates
go list -u -m all

# Update dependencies
go get -u ./...
go mod tidy

# Test thoroughly
go test ./... -short
```

**Python Dependencies:**
```bash
# Update requirements
pip install --upgrade -r tools/parse_module/requirements.txt

# Test changes
pytest tests/
```

**Rust Dependencies:**
```bash
cd /path/to/NEEDLE
cargo update
cargo test
```

---

## Troubleshooting

### Common Issues

**Go version mismatch:**
```bash
# Check current version
go version

# Expected: go version go1.25.0
# If different, install Go 1.25.0
```

**pytest not found:**
```bash
# Install from requirements
pip install -r tools/parse_module/requirements.txt
```

**Docker build failures:**
```bash
# Check Docker version
docker --version

# Verify build context
docker build -t test-build .
```

### Verification Commands

**Full tool verification:**
```bash
#!/bin/bash
echo "=== Development Tools Verification ==="

echo -n "Go version: "
go version

echo -n "Python version: "
python3 --version

echo -n "pytest version: "
pytest --version 2>/dev/null || echo "pytest not installed"

echo -n "Docker version: "
docker --version

echo -n "Git version: "
git --version

echo "=== Test Build ==="
go build ./... && echo "✅ Build successful" || echo "❌ Build failed"

echo "=== Run Tests ==="
go test ./... -short && echo "✅ Tests passed" || echo "❌ Tests failed"

echo "=== Python Tests ==="
pytest tests/ -q && echo "✅ Python tests passed" || echo "❌ Python tests failed"
```

---

## Related Documentation

- **Pluck Version Inventory:** `/home/coding/ARMOR/pluck-version-inventory.md`
- **Pluck Dependency Requirements:** `/home/coding/ARMOR/pluck-dependency-requirements.md`
- **Version Compatibility Findings:** `/home/coding/ARMOR/version-compatibility-findings.md`
- **NEEDLE Documentation:** `/home/coding/NEEDLE/README.md`
- **ARMOR Documentation:** `/home/coding/ARMOR/README.md`

---

## Maintenance and Updates

**Document Maintenance:**
- **Created:** 2026-07-09
- **Bead:** bf-4qcfn
- **Next Review:** 2026-10-09 (Quarterly)

**Update Procedure:**
1. Run version verification commands
2. Update version tables with actual versions
3. Document any new tools or version changes
4. Verify all minimum requirements are still met
5. Test build and test workflows

---

## Summary

### Total Tools Count

- **Build Tools:** 3 (Go, Docker, Cargo)
- **Test Frameworks:** 3 (Go test, pytest, Python unittest)
- **Linters:** 8 (golangci-lint, go vet, govet, ineffassign, staticcheck, unused, rustfmt, clippy)
- **Package Managers:** 3 (go mod, cargo, pip)
- **Languages:** 3 (Go, Python, Rust)
- **Version Control:** 1 (Git)
- **Container/Deployment:** 3 (Docker, Kubernetes, Kustomize)
- **Utility Libraries:** 1 (pyyaml)
- **Integration Tools:** 2 (NEEDLE CLI, br CLI)

**Total:** 27 development tools across 9 categories

### Compliance Status

✅ **100% Compliant** - All tools meet or exceed minimum version requirements

**Production Ready:** Yes
**Critical Issues:** 0
**Required Upgrades:** 0

---

**End of Pluck Development Tools Version Inventory**
