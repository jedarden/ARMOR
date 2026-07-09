# Pluck Development Tools Version Inventory

**Document Created:** 2026-07-09  
**Bead:** bf-4qcfn  
**Workspace:** /home/coding/ARMOR  
**Status:** ✅ Complete

## Overview

This document provides a comprehensive inventory of all development tools used in the Pluck project's ARMOR workspace. It includes current versions, minimum requirements, and categorization by tool purpose.

## Document Scope

This inventory captures:
- **Build Tools**: Compilers, package managers, and build systems
- **Testing Tools**: Testing frameworks and test runners
- **Linting Tools**: Code quality and style checkers
- **Development Tools**: IDE support, debugging, and utilities
- **Deployment Tools**: Container, Kubernetes, and deployment utilities
- **Version Control**: Git and related tools

---

## Build Tools

### Go Toolchain

| Tool | Current Version | Minimum Required | Purpose | Status |
|------|----------------|-----------------|---------|--------|
| **go** | go1.25.0 linux/amd64 | 1.25.0 | Go compiler and toolchain | ✅ Compliant |
| **go mod** | (bundled with go1.25.0) | 1.25.0 | Dependency management | ✅ Available |
| **go build** | (bundled with go1.25.0) | 1.25.0 | Build Go binaries | ✅ Available |
| **go test** | (bundled with go1.25.0) | 1.25.0 | Run Go tests | ✅ Available |
| **go vet** | (bundled with go1.25.0) | 1.25.0 | Static analysis | ✅ Available |

**Version Constraints:**
- **Minimum:** 1.25.0 (required by go.mod)
- **Recommended:** 1.25.0 or later
- **Source:** `go.mod` specifies `go 1.25.0`

**Installation Check:**
```bash
go version
# Expected output: go version go1.25.0 linux/amd64
```

### Python Toolchain

| Tool | Current Version | Minimum Required | Purpose | Status |
|------|----------------|-----------------|---------|--------|
| **python3** | Python 3.12.12 | 3.8+ | Python interpreter | ✅ Compliant |
| **pip** | (bundled with python3) | 3.8+ | Package management | ✅ Available |
| **unittest** | (standard library) | - | Testing framework | ✅ Available |

**Version Constraints:**
- **Minimum:** Python 3.8+ (inferred from modern syntax usage)
- **Current:** Python 3.12.12
- **Purpose:** Test utilities, YAML parsing, configuration validation

**Installation Check:**
```bash
python3 --version
# Expected output: Python 3.12.12
```

### Docker / Container Tools

| Tool | Current Version | Minimum Required | Purpose | Status |
|------|----------------|-----------------|---------|--------|
| **docker** | Docker 27.5.1 | 20.10+ | Container building and running | ✅ Compliant |
| **docker build** | (bundled with Docker) | 20.10+ | Build container images | ✅ Available |
| **alpine** | golang:1.25-alpine | - | Build base image | ✅ Available |

**Version Constraints:**
- **Docker:** 20.10+ for multi-stage builds and COPY --from syntax
- **Base Image:** golang:1.25-alpine (Docker Hub)
- **Purpose:** ARMOR container builds for deployment

**Installation Check:**
```bash
docker --version
# Expected output: Docker version 27.5.1, build v27.5.1
```

---

## Testing Tools

### Go Testing Framework

| Tool | Version | Type | Purpose |
|------|---------|------|---------|
| **go test** | (bundled with go1.25.0) | Standard | Unit tests |
| **testing** | (standard library) | Standard | Test assertions |
| **-short flag** | (go test feature) | Standard | Skip integration tests |

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

# Specific test package
go test ./tests/integration/ -v
```

### Python Testing Framework

| Tool | Version | Type | Purpose |
|------|---------|------|---------|
| **unittest** | (standard library) | Standard | Python test framework |
| **pytest** | (inferred from .pytest_cache) | Third-party | Test runner (likely) |

**Test Files:**
- `tests/test_inventory_reader.py` - Inventory reader tests
- `tests/yamlutil/test_broken_samples.py` - YAML validation tests
- `tests/yamlutil/test_validator.py` - YAML validator tests
- `tests/yamlutil/verify_implementation.py` - Implementation verification

**Test Execution:**
```bash
# Run Python tests
python3 -m unittest discover tests/

# Run specific test file
python3 -m unittest tests.test_inventory_reader
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

## Development Tools

### NEEDLE/Pluck CLI Tools

| Tool | Version | Purpose | Location |
|------|---------|---------|----------|
| **needle** | 0.2.11 | NEEDLE CLI and Pluck strand | `~/.local/bin/needle` |
| **br** | 0.2.0 (bead-forge) | Bead management CLI | `~/.local/bin/br` |
| **pluck** | (part of needle) | Bead selection strand | `needle strand pluck` |

**NEEDLE Components:**
- **Pluck Strand:** Primary bead selection from workspace
- **Bead Store:** SQLite-based bead tracking (`.beads/beads.db`)
- **JSONL Checkpoint:** Bead state persistence (`.beads/issues.jsonl`)

**Version Check:**
```bash
needle --version
# Expected: needle 0.2.11

br --version
# Expected: bf 0.2.0 (note: error output format)
```

### Configuration and Parsing Tools

| Tool | Language | Purpose | Location |
|------|----------|---------|----------|
| **YAML Parser** | Python | YAML parsing and validation | `tools/parse_module/yaml_parser.py` |
| **Config Parser** | Python | Multi-format config parsing | `tools/config_parser/parse_configs.py` |
| **Inventory Reader** | Python | Debug file inventory | `scripts/debug-config-parser/inventory.py` |

**Python Dependencies:**
- **PyYAML:** YAML parsing (import yaml)
- **Standard Library:** json, pathlib, dataclasses, enum

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

## System Dependencies

### Required System Packages (Linux)

| Package | Purpose | Status |
|---------|---------|--------|
| **git** | Version control | ✅ Installed |
| **curl** | HTTP client | ✅ Installed |
| **ca-certificates** | SSL/TLS certificates | ✅ In Docker image |
| **tzdata** | Timezone data | ✅ In Docker image |

**Note:** System packages are installed via Alpine APK in the Docker build stage.

---

## Tool Categories Summary

### Category 1: Core Build Tools
- **Go 1.25.0** - Primary language and build system
- **Python 3.12.12** - Test utilities and configuration parsing
- **Docker 27.5.1** - Container builds

### Category 2: Testing Frameworks
- **go test** - Go unit and integration tests
- **unittest** - Python testing framework
- **pytest** - Python test runner (inferred)

### Category 3: Code Quality
- **go vet** - Go static analysis
- **golangci-lint** - Comprehensive Go linting (CI-only)
- **Static analysis** - Part of CI pipeline

### Category 4: Deployment
- **kubectl 1.33.3** - Kubernetes management
- **Kustomize 5.6.0** - Kubernetes customization
- **Docker Hub** - Container registry

### Category 5: Development Utilities
- **NEEDLE 0.2.11** - Bead-based task management
- **br CLI 0.2.0** - Bead store management
- **Git 2.50.1** - Version control

---

## Minimum Version Requirements Summary

### Critical Path Tools
| Tool | Minimum | Current | Buffer |
|------|---------|---------|--------|
| **Go** | 1.25.0 | 1.25.0 | ✅ Exact |
| **Python** | 3.8+ | 3.12.12 | ✅ 4 major versions |
| **Docker** | 20.10+ | 27.5.1 | ✅ 7 major versions |
| **kubectl** | 1.20+ | 1.33.3 | ✅ 13 minor versions |

### Optional Tools
| Tool | Minimum | Current | Status |
|------|---------|---------|--------|
| **golangci-lint** | 2.0+ | Not installed locally | ⚠️ CI-only |
| **pytest** | 7.0+ | Not verified | ⚠️ Inferred usage |

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

---

## Compatibility Notes

### Go Version Compatibility

**Current State:**
- **ARMOR workspace requires Go 1.25.0** (from go.mod)
- **Rust 1.96.1** is installed for NEEDLE/Pluck tools (MSRV 1.75+)
- **No version conflicts detected**

**Known Constraints:**
- golang.org/x/crypto@v0.49.0 requires Go 1.25+
- AWS SDK v2 modules support Go 1.21+

### Python Version Compatibility

**Current State:**
- **Python 3.12.12** is well above minimum 3.8+
- **All standard library features available**
- **PyYAML compatibility confirmed**

**Dependencies:**
- PyYAML (standard import yaml)
- Standard library (json, pathlib, dataclasses, enum)

### Docker Compatibility

**Current State:**
- **Docker 27.5.1** supports all required features
- **Multi-stage builds** ✅
- **COPY --from syntax** ✅
- **Alpine base images** ✅

---

## Maintenance Schedule

### Regular Maintenance

| Frequency | Task | Purpose |
|-----------|------|---------|
| **Monthly** | Check Go updates | Security and feature updates |
| **Monthly** | Check Python updates | Security patches |
| **Quarterly** | Review Docker base images | Security and optimization |
| **As Needed** | Update kubectl | Cluster compatibility |
| **As Needed** | Update NEEDLE/br | Bead management features |

### Update Procedures

**Go Dependencies:**
```bash
cd /home/coding/ARMOR
go get -u ./...
go mod tidy
git add go.mod go.sum
git commit -m "chore: update Go dependencies"
```

**Docker Base Image:**
```bash
# Update FROM line in Dockerfile
# FROM golang:1.25-alpine → FROM golang:1.26-alpine

# Test build
docker build -t armor:test .

# Update go.mod if needed
go mod tidy
```

**Python Tools:**
```bash
# Update PyYAML
pip3 install --upgrade pyyaml

# Update pytest (if used)
pip3 install --upgrade pytest
```

---

## Security Considerations

### Tool Security

**High-Priority Tools:**
- **Go 1.25.0** - Monitor for security updates
- **Python 3.12.12** - Security patches via system updates
- **Docker 27.5.1** - Container security patches
- **kubectl 1.33.3** - Cluster access security

**Security Scanning:**
```bash
# Go dependency audit
go list -json -m all | nancy sleuth

# Docker image scan
docker scan ronaldraygun/armor:<version>

# Kubernetes manifest validation
kubectl apply -k deploy/kubernetes/ --dry-run=server
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

## Document Maintenance

### Change History

| Date | Version | Changes |
|------|---------|---------|
| 2026-07-09 | 1.0 | Initial development tools version inventory |

### Next Review Date

**Scheduled Review:** 2026-10-09 (Quarterly)

**Review Checklist:**
- [ ] Verify all tool versions are current
- [ ] Check for security advisories
- [ ] Update minimum requirements if needed
- [ ] Verify CI/CD pipeline compatibility
- [ ] Document any new tools introduced

---

## References

### Internal Documentation

- **Pluck Configuration:** `/home/coding/ARMOR/pluck-config.yaml`
- **Pluck Dependency Requirements:** `/home/coding/ARMOR/pluck-dependency-requirements.md`
- **ARMOR README:** `/home/coding/ARMOR/README.md`
- **Go Module:** `/home/coding/ARMOR/go.mod`

### External Resources

- **Go Downloads:** https://go.dev/dl/
- **Python Downloads:** https://www.python.org/downloads/
- **Docker Documentation:** https://docs.docker.com/
- **Kubernetes Documentation:** https://kubernetes.io/docs/
- **NEEDLE Repository:** https://github.com/jedarden/NEEDLE
- **ARMOR Repository:** https://github.com/jedarden/ARMOR

---

**Document Status:** ✅ Complete  
**Last Updated:** 2026-07-09  
**Next Review:** 2026-10-09 (Quarterly)  
**Maintained By:** ARMOR Development Team
