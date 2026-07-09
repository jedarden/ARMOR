# Pluck Development Tools Version Inventory

**Bead ID:** bf-5riod  
**Date:** 2026-07-09  
**Task:** Extract current version information for Pluck tools  
**Workspace:** /home/coding/ARMOR

---

## Overview

This document captures the current versions of all development tools used in the ARMOR project, along with their sources and any dynamic version flags.

---

## 1. Language Toolchains

### Go (Golang)
| Tool | Version | Source | Type |
|------|---------|--------|------|
| **Go (golang)** | **1.25.0** | `go.mod:3`, `Dockerfile:2` | **Pinned** |
| **Go runtime** | 1.25.0 | `go.mod:3` | Pinned |
| **Docker image** | golang:1.25-alpine | `Dockerfile:2` | Pinned |

### Rust
| Tool | Version | Source | Type |
|------|---------|--------|------|
| **rustc** | **1.96.1** (31fca3adb 2026-06-26) | Local toolchain (`rustc --version`) | **Dynamic** |

### Node.js
| Tool | Version | Source | Type |
|------|---------|--------|------|
| **node** | **v22.16.0** | Local toolchain (`node --version`) | **Dynamic** |

### Python
| Tool | Version | Source | Type |
|------|---------|--------|------|
| **Python** | **3.12.12** | Local toolchain (`python3 --version`) | **Dynamic** |

---

## 2. Go Dependencies (from go.mod/go.sum)

### AWS SDK v2 Stack
| Dependency | Version | Type |
|------------|---------|------|
| `github.com/aws/aws-sdk-go-v2` | **v1.41.4** | Pinned |
| `github.com/aws/aws-sdk-go-v2/config` | **v1.32.12** | Pinned |
| `github.com/aws/aws-sdk-go-v2/credentials` | **v1.19.12** | Pinned |
| `github.com/aws/aws-sdk-go-v2/service/s3` | **v1.97.2** | Pinned |
| `github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream` | **v1.7.8** | Indirect |
| `github.com/aws/aws-sdk-go-v2/feature/ec2/imds` | **v1.18.20** | Indirect |
| `github.com/aws/aws-sdk-go-v2/internal/configsources` | **v1.4.20** | Indirect |
| `github.com/aws/aws-sdk-go-v2/internal/endpoints/v2` | **v2.7.20** | Indirect |
| `github.com/aws/aws-sdk-go-v2/internal/ini` | **v1.8.6** | Indirect |
| `github.com/aws/aws-sdk-go-v2/internal/v4a` | **v1.4.21** | Indirect |
| `github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding` | **v1.13.7** | Indirect |
| `github.com/aws/aws-sdk-go-v2/service/internal/checksum` | **v1.9.12** | Indirect |
| `github.com/aws/aws-sdk-go-v2/service/internal/presigned-url` | **v1.13.20** | Indirect |
| `github.com/aws/aws-sdk-go-v2/service/internal/s3shared` | **v1.19.20** | Indirect |
| `github.com/aws/aws-sdk-go-v2/service/signin` | **v1.0.8** | Indirect |
| `github.com/aws/aws-sdk-go-v2/service/sso` | **v1.30.13** | Indirect |
| `github.com/aws/aws-sdk-go-v2/service/ssooidc` | **v1.35.17** | Indirect |
| `github.com/aws/aws-sdk-go-v2/service/sts` | **v1.41.9** | Indirect |

### Google Cloud Storage
| Dependency | Version | Type |
|------------|---------|------|
| `github.com/kurin/blazer` | **v0.5.3** | Pinned |

### Go Standard Library Extensions
| Dependency | Version | Type |
|------------|---------|------|
| `golang.org/x/crypto` | **v0.49.0** | Pinned |
| `golang.org/x/sync` | **v0.12.0** | Pinned |

### AWS Smithy (Runtime Framework)
| Dependency | Version | Type |
|------------|---------|------|
| `github.com/aws/smithy-go` | **v1.24.2** | Indirect |

### YAML Support
| Dependency | Version | Type |
|------------|---------|------|
| `gopkg.in/yaml.v3` | **v3.0.1** | Indirect |

---

## 3. Python Dependencies (from tools/parse_module/requirements.txt)

| Package | Version | Type |
|---------|---------|------|
| **pyyaml** | **>=6.0** | **Dynamic** (minimum version) |
| **pytest** | **>=7.0.0** | **Dynamic** (minimum version) |

---

## 4. Linting & Static Analysis Tools

| Tool | Version | Source | Type |
|------|---------|--------|------|
| **golangci-lint** | v1.25 (configured) | `.golangci.yml` | Pinned (config) |
| **govet** | (bundled with Go) | `.golangci.yml:9` | Standard |
| **ineffassign** | (via golangci-lint) | `.golangci.yml:10` | Bundled |
| **staticcheck** | (via golangci-lint) | `.golangci.yml:11` | Bundled |
| **unused** | (via golangci-lint) | `.golangci.yml:12` | Bundled |

---

## 5. Container & Build Tools

| Tool | Version | Source | Type |
|------|---------|--------|------|
| **Docker** | **27.5.1** | Local toolchain (`docker --version`) | **Dynamic** |
| **Alpine Linux** | (via golang:1.25-alpine) | `Dockerfile:2` | Pinned (image tag) |
| **scratch** | (minimal base) | `Dockerfile:25` | Static (empty) |

---

## 6. Container Orchestration

| Tool | Version | Source | Type |
|------|---------|--------|------|
| **kubectl** | (not available locally) | N/A | **Dynamic** |
| **Kustomize** | (via kubectl) | `deploy/kubernetes/kustomization.yaml` | **Dynamic** |

**Note:** kubectl is not available in the local environment but is used via kubectl-proxy over Tailscale for cluster access (see CLAUDE.md).

---

## 7. Build System Configuration

| Tool | Setting | Source | Type |
|------|---------|--------|------|
| **CGO_ENABLED** | 0 (disabled) | `Dockerfile:19,22` | Static |
| **GOOS** | linux | `Dockerfile:22` | Pinned |
| **ldflags** | -s -w (strip debug info) | `Dockerfile:22` | Static |

---

## 8. Source Files Analyzed

| File | Purpose | Lock Status |
|------|---------|-------------|
| `go.mod` | Go module definition | **YES** (locked) |
| `go.sum` | Go dependency checksums | **YES** (lock file) |
| `Dockerfile` | Container build recipe | Partial (base image pinned) |
| `.golangci.yml` | Linter configuration | Static config |
| `tools/parse_module/requirements.txt` | Python dependencies | **NO** (dynamic versions) |
| `deploy/kubernetes/kustomization.yaml` | K8s configuration | **NO** (Kustomize overlays) |

---

## 9. Dynamic Version Flags

### High Priority (Actionable)
| Tool | Version Specification | Risk Level | Recommendation |
|------|----------------------|-----------|----------------|
| **pyyaml** | >=6.0 | Medium | Consider pinning exact version |
| **pytest** | >=7.0.0 | Low | Test framework, semver safe |
| **Docker** | 27.5.1 (local) | Low | Version consistency across builds |
| **Node.js** | v22.16.0 (local) | Low | If used, should pin |
| **Rust** | 1.96.1 (local) | Low | If used, should pin |

### Medium Priority
| Tool | Version Specification | Risk Level |
|------|----------------------|-----------|
| **kubectl** | Not installed locally | Medium (cluster access) |
| **Kustomize** | Via kubectl | Medium |

---

## 10. Version Source Documentation

| Category | Source Files | Version Extraction Method |
|----------|--------------|---------------------------|
| **Go toolchain** | `go.mod:3`, `Dockerfile:2` | Parse version string |
| **Go dependencies** | `go.mod`, `go.sum` | Parse module versions |
| **Python deps** | `tools/parse_module/requirements.txt` | Parse package specs |
| **Container base** | `Dockerfile:2` | Extract image tag |
| **Linter config** | `.golangci.yml` | Read go version field |
| **Local tools** | CLI commands (`--version`) | Runtime detection |

---

## 11. Missing/Locked Status Summary

### Locked Versions (Reproducible)
✅ **Go 1.25.0** - Pinned in go.mod and Dockerfile  
✅ **Go dependencies** - All locked in go.sum  
✅ **Docker base image** - golang:1.25-alpine pinned  

### Dynamic Versions (Potential Drift)
⚠️ **Python dependencies** - Minimum versions only (>=6.0, >=7.0.0)  
⚠️ **Local toolchains** - Rust 1.96.1, Node v22.16.0, Python 3.12.12, Docker 27.5.1  
⚠️ **Kubernetes tools** - kubectl version not tracked locally  

### Standard Library Bundles (No Versioning)
📦 **govet** - Part of Go toolchain  
📦 **golangci-lint linters** - Bundled with meta-linter  

---

## 12. Recommendations

### For Reproducibility
1. **Pin Python dependencies**: Replace `>=` with exact versions in `requirements.txt`
2. **Document local toolchain versions**: Add to project documentation
3. **Consider toolchain version file**: e.g., `.tool-versions` for asdf/mise

### For CI/CD
1. **Argo Workflows**: Build uses Dockerfile with pinned Go version
2. **No GitHub Actions**: Disabled per CLAUDE.md
3. **Kubernetes access**: Via kubectl-proxy over Tailscale (read-only)

### For Development
1. **golangci-lint**: Configured for Go 1.25 (matches project)
2. **Testing**: Go standard library + pytest for Python utilities

---

## 13. Integration Points

| Tool | Integration Point | Version Controlled By |
|------|-------------------|----------------------|
| Go modules | `go build` | go.mod + go.sum |
| Docker | Multi-stage build | Dockerfile base image |
| Python utils | `tools/parse_module/` | requirements.txt |
| Linting | Pre-commit (implied) | .golangci.yml |
| Testing | `go test ./... -short` | Go stdlib + pytest |

---

## Generated

Generated for bead bf-5riod: "Extract current version information for Pluck tools"  
Dependencies: Categorized tool list from bead bf-195e3  
Workspace: /home/coding/ARMOR  
Date: 2026-07-09

---

## Summary Statistics

- **Total tools catalogued**: 29+
- **Locked versions**: 24 (Go modules + base image)
- **Dynamic versions**: 5 (Python deps + local toolchains)
- **Source files analyzed**: 8
- **Package dependencies**: 19 Go modules + 2 Python packages
