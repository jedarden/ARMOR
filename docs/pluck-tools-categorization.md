# Pluck Development Tools Categorization

**Bead:** bf-195e3  
**Workspace:** /home/coding/ARMOR  
**Last Updated:** 2026-07-09  
**Status:** ✅ Complete

## Overview

This document provides a categorized inventory of all development tools used across the Pluck project in the ARMOR workspace, organized by functional category.

## Tool Categories Summary

| Category | Tool Count | Primary Tools |
|----------|------------|---------------|
| **Build Tools** | 4 | Go, Python, Docker, Kaniko |
| **Testing Tools** | 3 | go test, unittest, pytest |
| **Linting Tools** | 5 | go vet, golangci-lint, staticcheck, ineffassign, unused |
| **Formatting Tools** | 1 | go fmt |
| **Configuration Tools** | 2 | YAML parsers, Config parsers |
| **Development Tools** | 3 | needle, br/bf CLI, Git |
| **Deployment Tools** | 3 | kubectl, kustomize, Docker Hub |
| **Version Control** | 1 | Git |

**Total Tools Identified:** 21+ across 7 categories

---

## Category 1: Build Tools

### Go Toolchain
| Tool | Version | Source File | Purpose |
|------|---------|-------------|---------|
| **go** | 1.25.0 | `go.mod` | Go compiler and toolchain |
| **go mod** | 1.25.0 | `go.mod` | Dependency management |
| **go build** | 1.25.0 | `Dockerfile` | Build Go binaries |

### Python Toolchain
| Tool | Version | Source File | Purpose |
|------|---------|-------------|---------|
| **python3** | 3.12.12 | System | Python interpreter |
| **pip** | 3.12.12 | System | Package management |

### Container Tools
| Tool | Version | Source File | Purpose |
|------|---------|-------------|---------|
| **docker** | 27.5.1 | `Dockerfile` | Container building |
| **docker build** | 27.5.1 | `Dockerfile` | Build container images |
| **Kaniko** | v1.23.2 | CI/CD Pipeline | Container image builds in CI |

---

## Category 2: Testing Tools

### Go Testing
| Tool | Version | Source File | Purpose |
|------|---------|-------------|---------|
| **go test** | 1.25.0 (built-in) | `*_test.go` files | Go testing framework |
| **testing** | 1.25.0 (stdlib) | `*_test.go` files | Test assertions |

### Python Testing
| Tool | Version | Source File | Purpose |
|------|---------|-------------|---------|
| **unittest** | 3.12.12 (stdlib) | `tests/*.py` | Python test framework |
| **pytest** | 7.0.0+ (optional) | Not installed | Advanced test runner |

### Test Execution Examples
```bash
# Go unit tests (integration tests auto-skipped)
go test ./... -short

# Python tests
python3 -m unittest discover tests/
```

---

## Category 3: Linting Tools

### Go Linting
| Tool | Version | Source File | Purpose |
|------|---------|-------------|---------|
| **go vet** | 1.25.0 (built-in) | `Dockerfile` | Static analysis |
| **golangci-lint** | Format v2 | `.golangci.yml` | Comprehensive linting |

### golangci-lint Linters (`.golangci.yml`)
| Linter | Purpose | Category |
|--------|---------|----------|
| **govet** | Go static analysis | Bug detection |
| **staticcheck** | Advanced static analysis | Bug detection |
| **ineffassign** | Detect ineffectual assignments | Code quality |
| **unused** | Detect unused code | Code cleanup |

### Linting Execution
```bash
# CI/CD pipeline
golangci-lint run

# Local equivalent
go vet ./...
```

---

## Category 4: Formatting Tools

### Code Formatting
| Tool | Version | Source File | Purpose |
|------|---------|-------------|---------|
| **go fmt** | 1.25.0 (built-in) | Standard Go tool | Code formatting |

### Formatting Configuration
- **Go fmt:** Standard Go formatting rules
- **No explicit formatters found for Python** (uses standard conventions)

---

## Category 5: Configuration Tools

### YAML and Configuration Parsing
| Tool | Language | Source File | Purpose |
|------|----------|-------------|---------|
| **YAML Parser** | Python | `tools/parse_module/yaml_parser.py` | YAML parsing and validation |
| **Config Parser** | Python | `tools/config_parser/parse_configs.py` | Multi-format config parsing |
| **Inventory Reader** | Python | `scripts/debug-config-parser/inventory.py` | Debug file inventory |

### Configuration Files
| File | Purpose |
|------|---------|
| `pluck-config.yaml` | Pluck debug configuration |
| `.needle.yaml` | NEEDLE strand configuration |
| `.beads/config.yaml` | Bead store configuration |
| `.golangci.yml` | Linting configuration |

---

## Category 6: Development Tools

### NEEDLE/Pluck CLI Tools
| Tool | Version | Source Location | Purpose |
|------|---------|-----------------|---------|
| **needle** | 0.2.11 | `~/.local/bin/needle` | NEEDLE CLI and Pluck strand |
| **br/bf CLI** | v0.2.0 | `~/.local/bin/br` | Bead management CLI |

### needle Key Commands
- `needle run` - Launch workers to process beads
- `needle stop` - Stop running workers
- `needle cleanup` - Remove orphaned tmux sessions
- `needle list` - List active workers
- `needle logs` - View telemetry logs
- `needle doctor` - Check system health
- `needle version` - Show version information

### br/bf CLI Key Commands
- `br create` - Create new beads
- `br update` - Update existing beads
- `br close` - Close completed beads
- `br sync` - Synchronize bead store
- `br doctor` - Check and repair database

### Development Utilities
| Tool | Language | Purpose |
|------|----------|---------|
| **Git** | - | Version control |
| **Bash scripts** | Bash | Various automation tasks |

---

## Category 7: Deployment Tools

### Kubernetes Tools
| Tool | Version | Source File | Purpose |
|------|---------|-------------|---------|
| **kubectl** | v1.33.3 | System | Kubernetes cluster management |
| **kustomize** | v5.6.0 | `deploy/kubernetes/kustomization.yaml` | Kubernetes customization |

### Container Deployment
| Tool | Purpose | Registry |
|------|---------|----------|
| **Docker Hub** | Container registry | ronaldraygun/armor:<VERSION> |
| **Argo Workflows** | CI/CD pipeline | iad-ci cluster |
| **ArgoCD** | GitOps deployments | ardenone-manager cluster |

### Kubernetes Resources
| Resource | Source File |
|----------|-------------|
| Deployment | `deploy/kubernetes/deployment.yaml` |
| Service | `deploy/kubernetes/service.yaml` |
| Ingress | `deploy/kubernetes/ingress-dashboard.yaml` |
| Kustomization | `deploy/kubernetes/kustomization.yaml` |

---

## Category 8: Version Control

### Git Tools
| Tool | Version | Source File | Purpose |
|------|---------|-------------|---------|
| **git** | 2.50.1 | System | Version control |

---

## Summary Table: All Tools by Category

| Tool | Category | Version | Source File | Purpose |
|------|----------|---------|-------------|---------|
| **go** | Build | 1.25.0 | go.mod | Go compiler |
| **python3** | Build | 3.12.12 | System | Python interpreter |
| **docker** | Build | 27.5.1 | Dockerfile | Container builds |
| **Kaniko** | Build | v1.23.2 | CI/CD | CI container builds |
| **go test** | Test | 1.25.0 | Built-in | Go testing |
| **unittest** | Test | 3.12.12 | Built-in | Python testing |
| **pytest** | Test | 7.0.0+ | Optional | Python test runner |
| **go vet** | Lint | 1.25.0 | Built-in | Static analysis |
| **golangci-lint** | Lint | v2 | .golangci.yml | Comprehensive linting |
| **staticcheck** | Lint | - | golangci-lint | Advanced analysis |
| **ineffassign** | Lint | - | golangci-lint | Detect ineffective assignments |
| **unused** | Lint | - | golangci-lint | Detect unused code |
| **go fmt** | Format | 1.25.0 | Built-in | Code formatting |
| **YAML Parser** | Config | Python | tools/parse_module/ | YAML parsing |
| **Config Parser** | Config | Python | tools/config_parser/ | Config parsing |
| **needle** | Dev | 0.2.11 | ~/.local/bin/ | NEEDLE CLI |
| **br/bf CLI** | Dev | v0.2.0 | ~/.local/bin/ | Bead management |
| **Git** | Dev | 2.50.1 | System | Version control |
| **kubectl** | Deploy | v1.33.3 | System | K8s management |
| **kustomize** | Deploy | v5.6.0 | Built-in to kubectl | K8s customization |
| **Docker Hub** | Deploy | - | Registry | Container registry |

---

## Tool Configuration Files Summary

| File | Category | Tools Configured |
|------|----------|------------------|
| `go.mod` | Build | Go 1.25.0, dependencies |
| `Dockerfile` | Build | Docker, Go build steps |
| `.golangci.yml` | Lint | golangci-lint, enabled linters |
| `pluck-config.yaml` | Config | Pluck debug configuration |
| `.needle.yaml` | Dev | NEEDLE strand configuration |
| `.beads/config.yaml` | Dev | Bead store configuration |
| `deploy/kubernetes/kustomization.yaml` | Deploy | Kubernetes resources |

---

## Acceptance Criteria Verification

✅ **All tool categories identified:** 7 categories defined
✅ **5+ tools found across categories:** 21+ tools identified
✅ **Tools grouped by function:** Build, Test, Lint, Format, Config, Dev, Deploy, VCS

---

## Installation Requirements Summary

### Required Tools
```bash
# Build Tools
go version go1.25.0
python3 --version 3.12.12
docker --version 27.5.1

# Testing
go test  # Built-in to Go
python3 -m unittest  # Built-in to Python

# Linting
go vet  # Built-in to Go
golangci-lint run  # Optional

# Development
needle --version 0.2.11
br --version v0.2.0

# Deployment
kubectl version --client v1.33.3
```

---

## References

### Source Files Analyzed
- `/home/coding/ARMOR/go.mod` - Go module configuration
- `/home/coding/ARMOR/Dockerfile` - Container build configuration
- `/home/coding/ARMOR/.golangci.yml` - Linting configuration
- `/home/coding/ARMOR/pluck-config.yaml` - Pluck configuration
- `/home/coding/ARMOR/.gitignore` - Build artifacts

### Related Documentation
- [Pluck Development Tools Complete](./pluck-development-tools-complete.md) - Detailed version inventory
- [Pluck Development Tools Version Inventory](./pluck-development-tools-version-inventory.md) - Version constraints

---

**Document Status:** ✅ Complete  
**Tools Identified:** 21+ across 7 categories  
**Last Updated:** 2026-07-09  
**Maintained By:** ARMOR Development Team
