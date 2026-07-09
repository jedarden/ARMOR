# ARMOR/Pluck Development Tools Inventory

**Bead:** bf-195e3  
**Last Updated:** 2026-07-09  
**Workspace:** /home/coding/ARMOR  
**Project Type:** Go (primary) + Python (utilities)

## Overview

This document categorizes all development tools used across the ARMOR project (which includes Pluck development tools), organized by function with source file references.

---

## Build Tools

| Tool | Version | Purpose | Source Files |
|------|---------|---------|--------------|
| **Go** | 1.25.0 | Core build toolchain | `go.mod:3` |
| **go build** | - | Compile Go binaries | `Dockerfile:22` |
| **go mod** | - | Dependency management | `go.mod:1-13`, `Dockerfile:11` |
| **Docker** | - | Containerized builds | `Dockerfile:1-38` |
| **CGO** | disabled (CGO_ENABLED=0) | Static binary compilation | `Dockerfile:19,22` |

**Build Pattern:** Multi-stage Docker builds with test gates (vet + test before build)

---

## Test Frameworks

| Tool | Version | Purpose | Source Files |
|------|---------|---------|--------------|
| **go test** | - | Go unit testing | `Dockerfile:19` |
| **go test** | - | Integration tests | `tests/integration/` |
| **pytest** | ≥7.0.0 | Python testing framework | `tools/parse_module/requirements.txt:8` |
| **pytest** | - | YAML parser tests | `tools/parse_module/tests/test_yaml_parser.py` |
| **Python unittest** | - | Standalone test runners | `tools/parse_module/test_result_standalone.py` |

**Test Pattern:** `-short` flag for unit tests, build tags for integration tests requiring credentials

---

## Linting Tools

| Tool | Category | Purpose | Source Files |
|------|----------|---------|--------------|
| **golangci-lint** | Meta-linter | Orchestrates multiple Go linters | `.golangci.yml:1-13` |
| **go vet** | Static analysis | Go code analysis | `.golangci.yml:9`, `Dockerfile:19` |
| **staticcheck** | Static analysis | Go bug detection | `.golangci.yml:11` |
| **ineffassign** | Linter | Detect ineffectual assignments | `.golangci.yml:10` |
| **unused** | Linter | Detect unused code | `.golangci.yml:12` |

**Linting Configuration:** Custom linter set (default: none), 4 specific linters enabled

---

## Code Quality & Static Analysis

| Tool | Purpose | Source Files |
|------|---------|--------------|
| **go vet** | Go static analysis | `.golangci.yml:9` |
| **staticcheck** | Find bugs | `.golangci.yml:11` |
| **ineffassign** | Dead code detection | `.golangci.yml:10` |
| **unused** | Unused code detection | `.golangci.yml:12` |

**Quality Gate:** Dockerfile enforces `go vet ./ && go test ./... -short` before build

---

## Validation Tools

| Tool | Purpose | Source Files |
|------|---------|--------------|
| **YAML validators** | Config syntax validation | `scripts/validate-pluck-syntax.sh` |
| **Custom validators** | Pluck command syntax | `test-pluck-syntax.sh` |
| **Config parsers** | YAML parsing/verification | `tools/config_parser/`, `tools/parse_module/` |
| **Syntax checkers** | Pluck command validation | `scripts/validate-pluck-syntax-comprehensive.sh` |

**Validation Pattern:** Pre-execution syntax checks without full runtime execution

---

## Dependency Management

| Tool | Language | Purpose | Source Files |
|------|----------|---------|--------------|
| **go mod** | Go | Module/dependency management | `go.mod`, `go.sum` |
| **pip** | Python | Python package management | `tools/parse_module/requirements.txt` |

**Key Dependencies:**
- AWS SDK v2 (S3, credentials, service endpoints)
- Google Cloud Storage (blazer v0.5.3)
- golang.org/x/crypto (v0.49.0)
- golang.org/x/sync (v0.12.0)
- PyYAML ≥6.0

---

## Development Tools

| Tool | Purpose | Source Files |
|------|---------|--------------|
| **Git** | Version control | `Dockerfile:7` |
| **Bash** | Scripting/automation | All `.sh` files (20+ scripts) |
| **Python 3** | YAML utilities | `tools/parse_module/`, `tests/` |
| **br (bead-forge)** | Issue tracking | `.beads/` directory |

---

## CI/CD Integration

| Tool | Purpose | Source Files |
|------|---------|--------------|
| **Docker build gates** | Pre-commit quality checks | `Dockerfile:16-19` |
| **Shell automation** | Test orchestration | `scripts/validate-pluck-syntax.sh` |
| **Test runners** | Automated test execution | `tools/parse_module/test_runner.py` |

**CI Pattern:** Test gate in Docker build pipeline

---

## Tool Categories Summary

| Category | Tool Count | Examples |
|----------|------------|----------|
| **Build** | 5 | Go, go build, Docker, CGO |
| **Test** | 4 | go test, pytest, unittest |
| **Lint** | 5 | golangci-lint, go vet, staticcheck |
| **Code Quality** | 4 | staticcheck, ineffassign, unused |
| **Validation** | 4 | YAML validators, config parsers |
| **Dependency Management** | 2 | go mod, pip |
| **Development** | 4 | Git, Bash, Python, br |
| **CI/CD** | 3 | Docker gates, shell automation |

**Total: 31 distinct tools across 8 categories**

---

## Source Configuration Files

| File | Tool Category |
|------|---------------|
| `go.mod` | Build, Dependency Management |
| `go.sum` | Dependency Management |
| `Dockerfile` | Build, Test, CI/CD |
| `.golangci.yml` | Lint, Code Quality |
| `tools/parse_module/requirements.txt` | Test, Dependency Management |
| `scripts/validate-pluck-syntax.sh` | Validation, CI/CD |
| `scripts/validate-pluck-syntax-comprehensive.sh` | Validation |
| `test-pluck-syntax.sh` | Validation |
| `test-pluck-redirection.sh` | Validation |

---

## Version Information

| Tool | Version | Source |
|------|---------|--------|
| **Go** | 1.25.0 | `go.mod:3` |
| **Python pytest** | ≥7.0.0 | `tools/parse_module/requirements.txt:8` |
| **PyYAML** | ≥6.0 | `tools/parse_module/requirements.txt:5` |
| **golangci-lint** | Latest (via golangci.yml) | `.golangci.yml:4` |

---

## Key Tool Locations

### Build & Dependency Files
- `/home/coding/ARMOR/go.mod` - Go module definition
- `/home/coding/ARMOR/go.sum` - Go dependency checksums
- `/home/coding/ARMOR/Dockerfile` - Multi-stage build with test gate

### Linting Configuration
- `/home/coding/ARMOR/.golangci.yml` - Custom linter configuration

### Python Testing Tools
- `/home/coding/ARMOR/tools/parse_module/requirements.txt` - Python test dependencies
- `/home/coding/ARMOR/tools/parse_module/tests/test_yaml_parser.py` - Pytest tests
- `/home/coding/ARMOR/tools/parse_module/test_runner.py` - Standalone test runner

### Validation Scripts
- `/home/coding/ARMOR/scripts/validate-pluck-syntax.sh` - Pluck syntax validation
- `/home/coding/ARMOR/scripts/validate-pluck-syntax-comprehensive.sh` - Comprehensive validation
- `/home/coding/ARMOR/test-pluck-syntax.sh` - Syntax testing
- `/home/coding/ARMOR/test-pluck-redirection.sh` - Log redirection testing

### Test Directories
- `/home/coding/ARMOR/tests/` - Go integration tests
- `/home/coding/ARMOR/tests/yamlutil/` - YAML utility tests (pytest)
- `/home/coding/ARMOR/tools/parse_module/tests/` - Python parser tests

---

## Notes

- **Test gates enforced:** Docker build runs `go vet ./ && go test ./... -short` before compilation
- **Integration tests require:** Build tags and credentials, automatically skipped in `-short` mode  
- **Custom linter config:** golangci-lint uses `default: none` with 4 specific linters enabled
- **Multi-language tooling:** Go (primary project) + Python (utilities, testing, YAML parsing)
- **No explicit formatters:** No gofmt or similar formatter configuration found
- **Shell-based automation:** 20+ shell scripts for validation, monitoring, and testing

---

*This inventory was created as part of bead bf-195e3 to categorize development tools used in the ARMOR/Pluck project.*
