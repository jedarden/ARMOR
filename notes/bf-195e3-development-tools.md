# ARMOR (Pluck) Development Tools Categorization

## Overview
This document categorizes all development tools used across the ARMOR project based on analysis of configuration files, build scripts, and dependency management files.

---

## Build Tools

| Tool | Category | Source Files |
|------|----------|--------------|
| **Go (golang)** | Language/Build | `go.mod:3` |
| **Go Modules** | Dependency Management | `go.mod`, `go.sum` |
| **go build** | Build Tool | `Dockerfile:22` |
| **CGO** | Build System | `Dockerfile:16`, `Dockerfile:19`, `Dockerfile:22` |
| **ldflags** | Build Optimization | `Dockerfile:22` |

### Build-Related Dependencies (go.mod)
- `github.com/aws/aws-sdk-go-v2` - AWS SDK v2
- `github.com/kurin/blazer` - Google Cloud Storage client
- `golang.org/x/crypto` - Cryptography extensions
- `golang.org/x/sync` - Concurrency primitives

---

## Test Frameworks & Tools

| Tool | Category | Language | Source Files |
|------|----------|----------|--------------|
| **Go testing** | Unit Test Framework | Go | `Dockerfile:19`, `*_test.go` files |
| **go test** | Test Runner | Go | `Dockerfile:19` |
| **go vet** | Static Analysis/Testing | Go | `Dockerfile:16`, `Dockerfile:19` |
| **pytest** | Testing Framework | Python | `tools/parse_module/requirements.txt:8` |
| **Custom test runner** | Test Framework | Python | `tools/parse_module/test_runner.py` |

### Test Files Found
- `internal/crypto/crypto_test.go`
- `internal/provenance/provenance_test.go`
- `internal/manifest/loader_test.go`
- `internal/manifest/manifest_test.go`
- `internal/manifest/compaction_test.go`
- `internal/manifest/roundtrip_test.go`
- `internal/manifest/writer_test.go`
- `internal/yamlutil/validator_test.go`
- `internal/yamlutil/parser_test.go`
- `internal/yamlutil/file_test.go`
- `tools/parse_module/test_*.py`

---

## Linters & Static Analysis

| Tool | Category | Language | Source Files |
|------|----------|----------|--------------|
| **golangci-lint** | Linting Engine | Go | `.golangci.yml` |
| **govet** | Static Analyzer | Go | `.golangci.yml:9` |
| **ineffassign** | Linter (ineffective assignments) | Go | `.golangci.yml:10` |
| **staticcheck** | Static Analyzer | Go | `.golangci.yml:11` |
| **unused** | Dead Code Detector | Go | `.golangci.yml:12` |

### Linter Configuration
`.golangci.yml` configures:
- Go version: 1.25
- Enabled linters: govet, ineffassign, staticcheck, unused
- Default linters disabled (custom configuration)

---

## Formatters

| Tool | Category | Language | Source Files |
|------|----------|----------|--------------|
| **Note:** No explicit formatter configuration files found (e.g., `.gofmt`, `.prettierrc`). Go's standard `gofmt` is typically used implicitly. |

---

## Container & Deployment Tools

| Tool | Category | Source Files |
|------|----------|--------------|
| **Docker** | Container Platform | `Dockerfile` |
| **Alpine Linux** | Base Image | `Dockerfile:2` |
| **Kubernetes** | Container Orchestration | `deploy/kubernetes/` |
| **kubectl** | Cluster Management | `deploy/kubernetes/kustomization.yaml:5` |
| **Kustomize** | Kubernetes Configuration | `deploy/kubernetes/kustomization.yaml` |
| **Ingress** | Kubernetes Routing | `deploy/kubernetes/ingress-dashboard.yaml` |

### Kubernetes Resources
- `kustomization.yaml` - Kustomize configuration
- `deployment.yaml` - Deployment manifest
- `service.yaml` - Service manifest
- `secret.yaml` - Secret management
- `ingress-dashboard.yaml` - Ingress rules

---

## Version Control

| Tool | Category | Source Files |
|------|----------|--------------|
| **Git** | Version Control | `.gitignore`, `go.sum` |
| **GitHub** | Remote Repository | Implied from project structure |

---

## Python Tools (parse_module utility)

| Tool | Category | Source Files |
|------|----------|--------------|
| **pip** | Package Manager | `tools/parse_module/requirements.txt:2` |
| **PyYAML** | YAML Parser | `tools/parse_module/requirements.txt:5` |
| **pytest** | Testing Framework | `tools/parse_module/requirements.txt:8` |

---

## Project-Specific Tools

| Tool | Category | Purpose |
|------|----------|---------|
| **NEEDLE** | Strand Management | `.needle.yaml` (Pluck bead selection) |
| **Pluck** | Work Distribution | `pluck-config.yaml` (debug/logging config) |

---

## Summary by Category

### Count of Tools per Category
- **Build Tools**: 6
- **Test Frameworks**: 5
- **Linters/Static Analysis**: 5
- **Container/Deployment**: 6
- **Version Control**: 2
- **Python Tools**: 3
- **Project-Specific**: 2

### Total Unique Tools: **29+**

---

## Tool Sources Summary

| Source File | Tools Identified |
|-------------|-----------------|
| `go.mod` | Go modules, AWS SDK, crypto libraries |
| `go.sum` | Dependency checksums |
| `.golangci.yml` | golangci-lint, govet, ineffassign, staticcheck, unused |
| `Dockerfile` | Docker, Go build, go vet, go test, Alpine |
| `*_test.go` files | Go testing framework |
| `tools/parse_module/requirements.txt` | pip, PyYAML, pytest |
| `deploy/kubernetes/*` | Kustomize, kubectl, Ingress |
| `.needle.yaml` | NEEDLE configuration |
| `pluck-config.yaml` | Pluck debugging |

---

## Generated
Generated for bead bf-195e3: "Identify and categorize Pluck development tools"
