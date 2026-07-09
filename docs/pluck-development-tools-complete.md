# Pluck Development Tools - Complete Version Inventory

**Bead:** bf-4qcfn  
**Last Updated:** 2026-07-09  
**Document Purpose:** Capture all development tool versions used in the Pluck project for reproducibility and dependency management  
**Status:** ✅ Complete

## Overview

Pluck is the strand configuration and bead processing system used within the ARMOR workspace. This document captures all development tools, their current versions, version constraints, and categorization for the complete Pluck development workflow.

## Tool Categories Summary

### Core Pluck Tools
| Tool | Current Version | Minimum Required | Category | Purpose |
|------|----------------|-----------------|----------|---------|
| **needle** | 0.2.11 (rust, linux x86_64) | 0.2.11 | Core CLI | Main Pluck/NEEDLE command interface |
| **br/bf** | v0.2.0 | v0.2.0 | Bead Management | Bead tracking and management |
| **needle-transform-claude** | Built to needle 0.2.11 | 0.2.11 | Transformation | Claude-specific transforms |
| **needle-transform-codex** | Built to needle 0.2.11 | 0.2.11 | Transformation | Codex-specific transforms |

### Configuration Tools
| Tool | Current Version | Minimum Required | Category | Purpose |
|------|----------------|-----------------|----------|---------|
| **YAML parsers** | Python 3.12.12 + PyYAML 6.0+ | Python 3.8+, PyYAML 6.0 | Config | Pluck configuration parsing |
| **pytest** | 7.0.0+ (not installed) | 7.0.0 | Test | Configuration testing |
| **Python unittest** | Built-in to 3.12.12 | Built-in | Test | Built-in Python testing |

### Build Tools
| Tool | Current Version | Minimum Required | Category | Purpose |
|------|----------------|-----------------|----------|---------|
| **Go** | 1.25.0 | 1.25.0 (strict) | Build | ARMOR project builds |
| **Docker** | 27.5.1 | Not specified | Build | Container builds |
| **Kaniko** | v1.23.2 | v1.23.2 | CI/CD | Container image builds |
| **Git** | 2.50.1 | Not specified | VCS | Version control |

### Development Tools
| Tool | Current Version | Minimum Required | Category | Purpose |
|------|----------------|-----------------|----------|---------|
| **golangci-lint** | Not installed | Go 1.25 | Lint | Go code linting |
| **go vet** | Built-in to 1.25.0 | Built-in | Lint | Go static analysis |
| **go test** | Built-in to 1.25.0 | Built-in | Test | Go testing framework |
| **staticcheck** | Via golangci-lint | Go 1.25 | Lint | Advanced Go analysis |

### Deployment Tools
| Tool | Current Version | Minimum Required | Category | Purpose |
|------|----------------|-----------------|----------|---------|
| **kubectl** | Not installed | 1.20+ (implied) | Deploy | Kubernetes deployment |
| **kustomize** | Built into kubectl | 3.0+ (implied) | Deploy | K8s resource customization |
| **Argo Workflows** | iad-ci cluster | Not specified | CI/CD | Container builds |
| **ArgoCD** | ardenone-manager | Not specified | CD | GitOps deployments |

## Core Pluck Tool Details

### needle CLI (Primary Pluck Interface)

**Version Information:**
```bash
needle version
# Output: needle 0.2.11 (rust, linux x86_64)
```

**Installation Location:** `/home/coding/.local/bin/needle`

**Key Commands:**
- `needle run` - Launch workers to process beads
- `needle stop` - Stop running workers  
- `needle cleanup` - Remove orphaned tmux sessions
- `needle list` - List active workers
- `needle attach` - Attach to worker tmux sessions
- `needle status` - Show fleet status and bead counts
- `needle logs` - View and query telemetry logs
- `needle config` - View or inspect configuration
- `needle doctor` - Check system health and repair
- `needle init` - Initialize v2 config with optional v1 migration
- `needle version` - Show version information
- `needle test-agent` - Validate agent adapters
- `needle canary` - Run canary tests
- `needle upgrade` - Check for and install updates
- `needle rollback` - Rollback to previous stable binary

**Configuration Files:**
- `.needle.yaml` - Main strand configuration
- `pluck-config.yaml` - Debug configuration
- `.beads/config.yaml` - Bead store configuration

**Version Constraints:** Strictly pinned to 0.2.11 (current stable)

### br/bf CLI (Bead Management)

**Version Information:**
```bash
# File: /home/coding/.local/bin/.bf-version
v0.2.0
```

**Installation Location:** `/home/coding/.local/bin/bf` (symlink: `br -> bf`)

**Key Commands:**
- `br create` - Create new beads
- `br update` - Update existing beads
- `br close` - Close completed beads
- `br sync` - Synchronize bead store
- `br doctor` - Check and repair bead database
- `br list` - List beads

**Critical Behavior Notes:**
- SQLite database is live store, `issues.jsonl` is checkpoint
- `br sync --flush-only` must run before `br doctor --repair`
- Unflushed beads exist only in database and are lost by repair

**Version Constraints:** v0.2.0 minimum required

### Transformation Tools

**needle-transform-claude**
- **Version:** Built to needle 0.2.11
- **Location:** `/home/coding/.local/bin/needle-transform-claude`
- **Purpose:** Claude-specific data transformations
- **Size:** 408,872 bytes

**needle-transform-codex**
- **Version:** Built to needle 0.2.11  
- **Location:** `/home/coding/.local/bin/needle-transform-codex`
- **Purpose:** Codex-specific data transformations
- **Size:** 415,312 bytes

## Configuration Tools Details

### Python Environment

**System Version:** Python 3.12.12

**Python Packages Required:**
```txt
# From: tools/parse_module/requirements.txt

# YAML parsing library
pyyaml>=6.0

# Testing framework  
pytest>=7.0.0
```

**Current Installation Status:**
- ✅ Python 3.12.12 installed
- ✅ PyYAML available (minimum 6.0)
- ❌ pytest not installed (minimum 7.0.0 required)
- ✅ unittest built-in to Python stdlib

**Usage in Pluck:**
- Configuration parsing (`.needle.yaml`, `pluck-config.yaml`)
- Debug file inventory management
- Log rotation and redirection testing
- YAML syntax validation

## Build Tools Details

### Go Toolchain

**System Version:** `go version go1.25.0 linux/amd64`

**Version Specification Locations:**
- `go.mod` line 3: `go 1.25.0`
- `.golangci.yml` line 4: `go: "1.25"`
- `Dockerfile` line 2: `FROM golang:1.25-alpine`

**Go Components:**
- **go vet:** Built-in static analysis
- **go test:** Built-in testing framework
- **go mod:** Built-in dependency management
- **go fmt:** Built-in code formatting

### Containerization Tools

**Docker:**
- **Version:** 27.5.1 (build v27.5.1)
- **Base Image:** golang:1.25-alpine (pinned in Dockerfile)
- **Registry:** Docker Hub `ronaldraygun/armor`

**Kaniko (CI/CD):**
- **Version:** v1.23.2
- **Image:** `gcr.io/kaniko-project/executor:v1.23.2`
- **Purpose:** Container image builds in iad-ci cluster
- **Workflow:** `armor-build` WorkflowTemplate

### Version Control

**Git:**
- **Version:** 2.50.1
- **Usage:** Source control, build dependency
- **CI Image:** `alpine/git` (in iad-ci workflows)

## Development Tools Details

### Linting Tools

**golangci-lint:**
- **Status:** Not installed locally
- **Version:** Format version "2"
- **Go version:** 1.25
- **Configuration:** `.golangci.yml`
- **Linters enabled:** govet, ineffassign, staticcheck, unused

**Built-in Go Analysis:**
- **go vet:** Static analysis (built-in)
- **staticcheck:** Advanced analysis (via golangci-lint)
- **ineffassign:** Ineffective assignment detection
- **unused:** Unused code detection

### Testing Frameworks

**Go Testing:**
- **Framework:** Built-in `testing` package
- **Unit Tests:** Standard `go test` with `-short` flag
- **Integration Tests:** Build tag `integration` requires credentials
- **Test Location:** `tests/integration/`

**Python Testing:**
- **Framework:** unittest (built-in to Python 3.12.12)
- **Optional Framework:** pytest 7.0.0+ (not currently installed)
- **Test Files:** `tests/test_inventory_reader.py`, integration tests

## Deployment Tools Details

### Kubernetes Tools

**kubectl:**
- **Status:** Not installed locally
- **Minimum Required:** 1.20+ (implied by K8s API version)
- **Usage:** Kubernetes cluster access
- **Clusters:** iad-ci, ardenone-manager, rs-manager

**kustomize:**
- **Version:** Built into kubectl
- **Minimum Required:** 3.0+ (implied)
- **Usage:** Kubernetes resource customization
- **Location:** `deploy/kubernetes/kustomization.yaml`

### CI/CD Pipeline

**Argo Workflows:**
- **Cluster:** iad-ci
- **Namespace:** argo-workflows
- **WorkflowTemplate:** armor-build
- **Access:** `kubectl --server=http://traefik-iad-ci:8001`

**ArgoCD:**
- **Cluster:** ardenone-manager
- **Purpose:** GitOps deployment management
- **Repository:** jedarden/declarative-config
- **Access:** Read-only proxy at `https://argocd-ro-ardenone-manager-ts.ardenone.com:8444`

## Version Constraints Summary

### Strict Requirements (Pinned Versions)

| Tool | Constraint | Rationale | Source |
|------|------------|-----------|--------|
| **needle** | = 0.2.11 | Current stable version | Binary version |
| **br/bf** | >= v0.2.0 | Bead tracking compatibility | Version file |
| **Go** | = 1.25.0 | Dockerfile and go.mod consistency | Multiple locations |
| **Docker base** | golang:1.25-alpine | Go version matching | Dockerfile |
| **Kaniko** | = v1.23.2 | CI/CD container builds | Workflow template |

### Minimum Requirements

| Tool | Minimum | Source |
|------|---------|--------|
| **Python** | 3.8+ | PyYAML and pytest compatibility |
| **PyYAML** | 6.0 | Configuration parsing |
| **pytest** | 7.0.0 | Python testing (optional) |
| **kubectl** | 1.20+ | Kubernetes API compatibility |
| **kustomize** | 3.0+ | Resource customization |

### Flexible Requirements

| Tool | Status | Notes |
|------|--------|-------|
| **Git** | No minimum | Any recent version |
| **Docker** | 27.5.1 installed | No strict minimum |
| **unittest** | Built-in | Part of Python stdlib |

## Installation and Management

### Core Pluck Tools Installation

**needle CLI:**
```bash
# Check installation
needle version

# Installation location
ls -la /home/coding/.local/bin/needle
```

**br/bf CLI:**
```bash
# Check version
cat /home/coding/.local/bin/.bf-version

# Symlink status
ls -la /home/coding/.local/bin/br
```

**Transformation Tools:**
```bash
# Check presence
ls -la /home/coding/.local/bin/needle-transform-*
```

### Configuration Tools Installation

**Python Dependencies:**
```bash
# Install required packages
pip install pyyaml>=6.0
pip install pytest>=7.0.0  # Optional

# Verification
python3 -c "import yaml; print(yaml.__version__)"
```

### Go Tools Installation

**Standard Go Tools:**
```bash
# Built-in tools (no installation needed)
go vet
go test
go mod
go fmt
```

**External Tools:**
```bash
# golangci-lint (not currently installed)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

## Development Environment Setup

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

## Version Compatibility Matrix

### Tool Version Compatibility

| Tool Version | Pluck 0.2.11 | ARMOR 0.1.373 | Notes |
|--------------|--------------|---------------|-------|
| needle 0.2.11 | ✅ Compatible | ✅ Compatible | Current stable |
| br/bf v0.2.0+ | ✅ Compatible | ✅ Compatible | Minimum required |
| Go 1.25.0 | N/A | ✅ Required | Strict requirement |
| Python 3.12.x | ✅ Compatible | ✅ Compatible | Config parsing |
| Docker 27.x | ✅ Compatible | ✅ Compatible | Container builds |

### Known Version Conflicts

1. **Go version:** Must match 1.25.0 across go.mod, Dockerfile, and golangci-lint config
2. **br/bf database:** Version changes may require database migration
3. **needle configuration:** v1 to v2 migration required for current version

## Reproducibility Notes

### Build Environment Reproducibility

- **Go version:** Strictly pinned to 1.25.0
- **CGO:** Disabled for static binaries  
- **Target OS:** Linux (GOOS=linux)
- **Build optimization:** Size-optimized (`-ldflags="-s -w"`)

### Configuration Environment Reproducibility

- **YAML parsing:** PyYAML >= 6.0 for consistent parsing
- **Python version:** 3.8+ minimum for compatibility
- **Configuration files:** Standard YAML syntax validation

### Development Workflow Reproducibility

- **needle version:** Pinned to 0.2.11 for consistent behavior
- **br/bf version:** v0.2.0+ for bead tracking compatibility
- **Transform tools:** Built to specific needle version

## Maintenance and Update Guidelines

### Regular Updates Needed

1. **needle/Pluck:** Check for updates with `needle upgrade`
2. **Go dependencies:** Run `go mod tidy` regularly  
3. **Python packages:** Update pip packages periodically
4. **Docker images:** Pull latest Alpine security updates

### Version Update Process

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

## Troubleshooting

### Common Pluck Tool Issues

**needle version mismatch:**
```bash
# Check version
needle version

# Update if available
needle upgrade
```

**br/bf database corruption:**
```bash
# Check database integrity
sqlite3 .beads/beads.db "PRAGMA integrity_check;"

# Flush before repair
br sync --flush-only
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
```

## Related Documentation

### Project Documentation
- **[development-tools.md](development-tools.md)** - ARMOR project development tools
- **[pluck-development-tools.md](pluck-development-tools.md)** - NEEDLE/Pluck Rust tools
- **[pluck-config.yaml](../pluck-config.yaml)** - Debug configuration
- **[.needle.yaml](../.needle.yaml)** - Strand configuration

### External Documentation
- **NEEDLE Repository:** https://github.com/jedarden/needle
- **Pluck Documentation:** See NEEDLE repository
- **Go 1.25 Release Notes:** https://go.dev/doc/go1.25
- **PyYAML Documentation:** https://pyyaml.org/wiki/PyYAMLDocumentation

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

**Development Workflow:**
- **Configuration:** YAML-based with Python validation
- **Build:** Go 1.25.0 with static linking
- **Testing:** Go built-in + Python unittest
- **Deployment:** Argo Workflows + ArgoCD

**Version Strategy:**
- **Strict:** needle 0.2.11, Go 1.25.0, Kaniko v1.23.2
- **Minimum:** br/bf v0.2.0, PyYAML 6.0, Python 3.8+
- **Flexible:** Git, Docker engine, unittest

---

**Document Status:** ✅ Complete  
**Next Review Date:** When needle version updates or major tool changes  
**Maintained By:** Project development team  
**File Location:** `/home/coding/ARMOR/docs/pluck-development-tools-complete.md`