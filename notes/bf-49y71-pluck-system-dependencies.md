# Pluck System Dependencies - Comprehensive Documentation

**Bead ID:** bf-49y71  
**Task:** Identify Pluck system dependencies  
**Created:** 2026-07-12  
**Status:** ✅ Complete

---

## Executive Summary

Pluck is a **strand (component)** within the NEEDLE system that handles primary bead selection from assigned workspaces. It is **NOT a standalone project** but rather part of the larger NEEDLE framework. This document provides a complete inventory of all system-level dependencies required by Pluck/NEEDLE.

**Key Findings:**
- Pluck requires **Rust 1.75+** (MSRV - Minimum Supported Rust Version)
- System dependencies vary by platform (Linux vs macOS)
- ARMOR workspace adds **Go 1.25.0** requirement
- Total of **25+ Cargo dependencies** managed via Cargo.toml

---

## What is Pluck?

**Pluck** is a component within the NEEDLE (Navigates Every Enqueued Deliverable, Logs Effort) system that:
- Processes >90% of all bead operations
- Queries the bead store for unassigned, ready beads
- Filters beads by excluded labels
- Sorts beads in deterministic priority order
- Manages bead splitting after failures

**Repository:** https://github.com/jedarden/NEEDLE  
**Component Path:** `NEEDLE/src/strand/pluck.rs`  
**Current Version:** 0.2.11 (as part of NEEDLE)

---

## Core System Requirements

### Operating System Support

| Platform | Architecture | Status | Source |
|----------|-------------|--------|--------|
| **Linux** | x86_64 (amd64) | ✅ Primary Target | rust-toolchain.toml |
| **Linux** | aarch64 (ARM64) | ✅ Supported | rust-toolchain.toml |
| **macOS** | x86_64 | ✅ Supported | Cross-compilation target |
| **macOS** | ARM64 (aarch64-apple-darwin) | ✅ Supported | rust-toolchain.toml |
| **Windows** | x86_64 | ⚠️ Limited Support | Not officially supported |

### Architecture Support

**Primary Targets:**
- `x86_64-unknown-linux-gnu` (Linux x86_64)
- `aarch64-apple-darwin` (macOS ARM64/Apple Silicon)

**Source:** `/home/coding/NEEDLE/rust-toolchain.toml`

---

## Toolchain Dependencies

### Rust Toolchain (Primary)

| Component | Minimum Required | Current Installed | Status | Source |
|-----------|-----------------|-------------------|--------|--------|
| **rustc** | 1.75 (MSRV) | 1.96.1 (2026-06-26) | ✅ Compliant | Cargo.toml |
| **cargo** | 1.75 (implied) | 1.96.1 (2026-06-26) | ✅ Compliant | rustup |
| **rustfmt** | Not specified | 1.96.1 | ✅ Installed | rust-toolchain.toml |
| **clippy** | Not specified | 0.1.96 | ✅ Installed | rust-toolchain.toml |

**MSRV (Minimum Supported Rust Version):** 1.75 (released 2023-12-28)

**Toolchain Configuration:**
```toml
[toolchain]
channel = "stable"
components = ["rustfmt", "clippy"]
targets = ["x86_64-unknown-linux-gnu", "aarch64-apple-darwin"]
```

**Installation:**
```bash
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --default-toolchain stable
source $HOME/.cargo/env
```

### Go Toolchain (ARMOR Workspace)

| Component | Minimum Required | Current Installed | Status | Source |
|-----------|-----------------|-------------------|--------|--------|
| **go** | 1.25.0 | 1.25.0 linux/amd64 | ✅ Compliant | go.mod |

**Installation:**
```bash
# Download and install Go 1.25.0
# Visit: https://go.dev/dl/
```

### Python Toolchain (Optional - for claude-interactive plugin)

| Component | Minimum Required | Purpose |
|-----------|-----------------|---------|
| **python3** | 3.10+ | Runtime for claude-interactive |
| **pyte** | Latest (via pip) | Terminal emulation for PTY wrapper |
| **claude CLI** | Latest | Claude Code CLI (must be on PATH) |

**Installation:**
```bash
pip3 install pyte
```

---

## Platform-Specific System Dependencies

### Linux (Debian/Ubuntu)

**Required System Packages:**

| Package | Purpose | Installation Command | Source |
|---------|---------|---------------------|--------|
| **git** | Version control system | `apt install git` | install.sh, CI Dockerfile |
| **curl** | HTTP client for downloads | `apt install curl` | install.sh, CI Dockerfile |
| **jq** | JSON processor for output parsing | `apt install jq` | install.sh, CI Dockerfile |
| **build-essential** | C compiler and build tools | `apt install build-essential` | Dependency compilation |
| **pkg-config** | Package configuration helper | `apt install pkg-config` | Dependency compilation |
| **libssl-dev** | OpenSSL development headers | `apt install libssl-dev` | ureq HTTP client dependency |

**Quick Install Command:**
```bash
sudo apt-get update && sudo apt-get install -y \
  git curl jq build-essential pkg-config libssl-dev
```

**Source:** `/home/coding/NEEDLE/ci/Dockerfile.ci`

### Linux (Alpine)

**Required System Packages:**

| Package | Purpose | Installation Command |
|---------|---------|---------------------|
| **build-base** | Build essentials (gcc, make) | `apk add build-base` |
| **sqlite-dev** | SQLite development headers | `apk add sqlite-dev` |
| **curl** | HTTP client | `apk add curl` |

**Quick Install Command:**
```bash
apk add build-base sqlite-dev curl
```

### macOS

**System Requirements:**

| Component | Minimum Required | Installation |
|-----------|-----------------|--------------|
| **Xcode Command Line Tools** | For building | `xcode-select --install` |
| **Rust** | 1.75+ | `brew install rust` |
| **SQLite** | Usually pre-installed | `brew install sqlite3` (if needed) |

**Note:** macOS has fewer system dependencies as most tools are available via Homebrew or are pre-installed.

### NixOS

**System is NixOS 25.05** - Uses Nix package manager instead of traditional apt/pacman:

**Available Tools:**
- ✅ Rust toolchain (via `nix-shell` or `rustc` package)
- ✅ Go 1.25.0
- ✅ Git 2.50.1
- ✅ curl 8.14.1
- ✅ jq-1.7.1
- ❌ Traditional package managers (apt, yum, pacman) are NOT present

**Important:** Documentation referencing `apt-get install` does **NOT apply** to this system.

---

## Cargo Dependencies (Rust)

### Runtime Dependencies (Required)

| Dependency | Minimum Version | Current Version | Purpose |
|------------|----------------|----------------|---------|
| **tokio** | ^1 | v1.52.3 | Async runtime (full features) |
| **serde** | ^1 | v1.0.228 | Serialization framework |
| **serde_json** | ^1 | v1.0.150 | JSON serialization |
| **serde_yaml** | ^0.9 | v0.9.34+deprecated | YAML serialization |
| **clap** | ^4 | v4.6.1 | CLI framework |
| **anyhow** | ^1 | v1.0.103 | Error handling |
| **thiserror** | ^1 | v1.0.69 | Error derivation |
| **tracing** | ^0.1 | v0.1.44 | Structured logging |
| **tracing-subscriber** | ^0.3 | v0.3.23 | Log filtering (env-filter, json) |
| **chrono** | ^0.4 | v0.4.45 | Time handling |
| **which** | ^4 | v4.4.2 | Executable discovery |
| **async-trait** | ^0.1 | v0.1.89 | Async trait support |
| **fs2** | ^0.4 | v0.4.3 | File locking (flock) |
| **sha2** | ^0.10 | v0.10.9 | SHA-2 hashing |
| **hex** | ^0.4 | v0.4.3 | Hex encoding |
| **regex** | ^1 | v1.12.4 | Regular expressions |
| **glob** | ^0.3 | v0.3.3 | Glob pattern matching |
| **ureq** | ^2 | v2.12.1 | Simple HTTP client |
| **aho-corasick** | ^1 | v1.1.4 | Multi-pattern search |
| **cfg-if** | ^1 | v1.0.4 | Conditional compilation |
| **atty** | ^0.2 | v0.2.14 | Terminal detection |
| **toml** | ^0.8 | v0.8.23 | TOML parsing |
| **libc** | ^0.2 | v0.2.186 | Unix process handling |
| **rand** | ^0.8 | v0.8.6 | Random generation |
| **futures** | ^0.3 | v0.3.32 | Async utilities |
| **gethostname** | ^0.4 | v0.4.3 | Hostname detection |

**Source:** `/home/coding/NEEDLE/Cargo.toml`

### Optional OpenTelemetry Dependencies

**Feature:** `otlp` (default feature, can be disabled with `--no-default-features`)

| Dependency | Minimum Version | Current Version | Purpose |
|------------|----------------|----------------|---------|
| **opentelemetry** | ^0.31 | v0.31.0 | OpenTelemetry API |
| **opentelemetry_sdk** | ^0.31 | v0.31.0 | OTLP SDK (rt-tokio) |
| **opentelemetry-otlp** | ^0.31 | v0.31.1 | OTLP exporter |
| **opentelemetry-semantic-conventions** | ^0.31 | v0.31.0 | Semantic conventions |
| **tonic** | ^0.14 | v0.14.6 | gRPC for OTLP |
| **tracing-opentelemetry** | ^0.32 | v0.32.1 | Tracing integration |

**Note:** Only required when building with default `otlp` feature.

### Development Dependencies (Build/Test Only)

| Dependency | Minimum Version | Current Version | Purpose |
|------------|----------------|----------------|---------|
| **tokio-test** | ^0.4 | v0.4.5 | Tokio testing utilities |
| **tempfile** | ^3 | v3.27.0 | Temporary file handling |
| **proptest** | ^1 | v1.11.0 | Property-based testing |
| **filetime** | ^0.2 | v0.2.29 | File time manipulation |
| **criterion** | ^0.5 | v0.5.1 | Benchmarking |
| **testcontainers** | ^0.23 | v0.23.0 | Integration tests (optional) |

**Note:** These are only required for building and testing, not for runtime.

---

## Go Dependencies (ARMOR Workspace)

### Core Go Dependencies

| Dependency | Version | Purpose |
|------------|---------|---------|
| **github.com/aws/aws-sdk-go-v2** | v1.41.4 | AWS SDK core |
| **github.com/aws/aws-sdk-go-v2/config** | v1.32.12 | AWS configuration |
| **github.com/aws/aws-sdk-go-v2/credentials** | v1.19.12 | AWS credentials |
| **github.com/aws/aws-sdk-go-v2/service/s3** | v1.97.2 | S3 storage operations |
| **github.com/kurin/blazer** | v0.5.3 | Google Cloud Storage |
| **golang.org/x/crypto** | v0.49.0 | Cryptographic primitives |
| **golang.org/x/sync** | v0.12.0 | Advanced synchronization |

**Source:** `/home/coding/ARMOR/go.mod`

### Transitive Go Dependencies (AWS SDK v2)

Key transitive dependencies include:
- `github.com/aws/smithy-go` v1.24.2 - Smithy protocol runtime
- `golang.org/x/net` v0.51.0 - Network utilities
- `golang.org/x/sys` v0.42.0 - System interfaces
- `golang.org/x/term` v0.41.0 - Terminal handling
- `golang.org/x/text` v0.35.0 - Text processing

---

## Specialized Dependencies

### br CLI (Bead Management System)

| Component | Version | Purpose |
|-----------|---------|---------|
| **br CLI (bead-forge)** | v0.2.0 | Bead store management |
| **SQLite** | Embedded | Bead store database |

**Installation:**
```bash
cargo install --git https://github.com/jedarden/bead-forge
```

**Critical Behavior:**
- SQLite database is live store, `issues.jsonl` is checkpoint
- **MUST flush before repair:** `br sync --flush-only` before `br doctor --repair`

### GitHub CLI (Optional - for release downloads)

| Component | Purpose | Installation |
|-----------|---------|--------------|
| **gh** | GitHub release downloads | `apt install gh` (Linux) |

**Source:** `/home/coding/NEEDLE/ci/Dockerfile.ci`

---

## Dependency Versions and Constraints

### Version Requirement Types

| Constraint Type | Meaning | Example |
|----------------|---------|---------|
| **`^1`** (caret) | Minimum 1.0.0, allows < 2.0.0 | `^1` allows 1.5.0 but not 2.0.0 |
| **`^0.1`** | Minimum 0.1.0, allows < 0.2.0 | `^0.1` allows 0.1.5 but not 0.2.0 |
| **`0.9`** (no caret) | Exactly 0.9.x | `0.9` allows 0.9.1 but not 0.10.0 |

### Pinned Versions (Strict Requirements)

| Tool | Constraint | Rationale |
|------|------------|-----------|
| **needle** | = 0.2.11 | Current stable version |
| **Go** | = 1.25.0 | Dockerfile and go.mod consistency |
| **Rust (MSRV)** | 1.75 | Minimum supported version |

### Minimum Requirements

| Tool | Minimum | Notes |
|------|---------|-------|
| **Python** | 3.10+ | For claude-interactive only |
| **PyYAML** | 6.0 | Configuration parsing |
| **kubectl** | 1.20+ | Kubernetes API compatibility |
| **Docker** | 20.10+ | Multi-stage builds |

---

## Installation Methods

### Method 1: Pre-built Binary (Recommended)

```bash
curl -fsSL https://github.com/jedarden/NEEDLE/releases/latest/download/install.sh | bash
```

**Requirements:**
- `curl` or `wget` for downloading
- `sha256sum` or `shasum` for checksum verification
- `gpg` (optional) for signature verification

### Method 2: Cargo Install

```bash
cargo install --git https://github.com/jedarden/NEEDLE
```

**Requirements:**
- Rust toolchain 1.75+
- Cargo package manager
- System build tools

### Method 3: Build from Source

```bash
git clone https://github.com/jedarden/NEEDLE.git
cd NEEDLE
cargo build --release
cargo install --path .
```

**Requirements:**
- Full Rust toolchain 1.75+
- All system build dependencies
- All Cargo dependencies

---

## Docker and Container Dependencies

### Build Dependencies (Alpine Linux)

**From ARMOR Dockerfile:**
```dockerfile
FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git ca-certificates tzdata
```

**Runtime Dependencies:**
- CA certificates (`/etc/ssl/certs/ca-certificates.crt`)
- Timezone data (`/usr/share/zoneinfo`)

### CI/CD Docker Dependencies

**From NEEDLE CI Dockerfile:**
```dockerfile
FROM debian:bookworm
RUN apt-get update && apt-get install -y \
    git curl jq build-essential pkg-config libssl-dev
```

---

## Quick Verification Commands

### Verify Core Tools

```bash
echo "=== Core Tools ==="
rustc --version      # Expected: rustc 1.75+ (currently 1.96.1)
cargo --version      # Expected: cargo 1.75+ (currently 1.96.1)
go version           # Expected: go version go1.25.0
git --version        # Expected: git 2.x.x
```

### Verify NEEDLE/Pluck Installation

```bash
echo "=== NEEDLE Components ==="
needle --version     # Expected: needle 0.2.11
br --version         # Expected: Error: bf 0.2.0
```

### Verify System Dependencies (Linux)

```bash
echo "=== System Dependencies ==="
curl --version       # Expected: curl 7.x.x or 8.x.x
jq --version         # Expected: jq-1.x
```

### Check Dependency Versions

```bash
# In NEEDLE directory
cd /home/coding/NEEDLE
cargo tree --depth 1

# In ARMOR directory
cd /home/coding/ARMOR
go list -m all
```

---

## Hardcoded Dependency Checks

### No Runtime Version Checks Found

Based on comprehensive search of NEEDLE source code:
- **No hardcoded version checks** in Rust code (`*.rs` files)
- **No hardcoded version checks** in Go code (`*.go` files)  
- **No hardcoded version checks** in Python code (`*.py` files)

**Assessment:** Pluck/NEEDLE does not enforce version checks at runtime. Version compatibility is ensured at:
- **Build time:** Via Cargo.toml dependency specifications
- **Installation time:** Via CI environment setup
- **Documentation:** Via MSRV specification

---

## Platform-Specific Requirements Summary

### Linux (Debian/Ubuntu)

**Required:**
```bash
sudo apt-get update && sudo apt-get install -y \
  git curl jq build-essential pkg-config libssl-dev
```

**Optional:**
- GitHub CLI: `sudo apt-get install -y gh`

### Linux (Alpine)

**Required:**
```bash
apk add build-base sqlite-dev curl
```

### macOS

**Required:**
```bash
# Install Xcode Command Line Tools
xcode-select --install

# Install Rust via Homebrew
brew install rust

# SQLite usually pre-installed, if not:
brew install sqlite3
```

### NixOS

**Tools available via Nix:**
- ✅ Rust toolchain
- ✅ Go 1.25.0
- ✅ Git, curl, jq
- ❌ Traditional package managers (apt, yum, pacman) - not applicable

---

## Dependency Troubleshooting

### Common Issues

**Issue:** "Rust not found"
```bash
# Solution: Install Rust via rustup
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
```

**Issue:** "br: command not found"
```bash
# Solution: Install bead-forge
cargo install --git https://github.com/jedarden/bead-forge
```

**Issue:** "libssl-dev: error while loading shared libraries"
```bash
# Solution: Install OpenSSL development headers
sudo apt-get install libssl-dev  # Debian/Ubuntu
brew install openssl             # macOS
```

**Issue:** Build failures
```bash
# Solution: Ensure Rust 1.75+
rustc --version  # Should show 1.75 or later

# Solution: Install build essentials
sudo apt-get install build-essential pkg-config
```

---

## Official Documentation Sources

### Primary Sources

1. **NEEDLE Cargo.toml** - `/home/coding/NEEDLE/Cargo.toml`
   - Defines MSRV: `rust-version = "1.75"`
   - Specifies all dependency minimum versions
   - Defines feature flags (otlp, integration)

2. **NEEDLE rust-toolchain.toml** - `/home/coding/NEEDLE/rust-toolchain.toml`
   - Specifies toolchain configuration
   - Defines required components (rustfmt, clippy)
   - Lists build targets

3. **NEEDLE CI Dockerfile** - `/home/coding/NEEDLE/ci/Dockerfile.ci`
   - System dependencies for Debian/Ubuntu
   - Build environment setup

4. **ARMOR go.mod** - `/home/coding/ARMOR/go.mod`
   - Go toolchain requirements
   - Go dependency specifications

5. **ARMOR Dockerfile** - `/home/coding/ARMOR/Dockerfile`
   - Alpine build dependencies
   - Runtime requirements

### External Sources

- **NEEDLE Repository:** https://github.com/jedarden/NEEDLE
- **ARMOR Repository:** https://github.com/jedarden/ARMOR
- **bead-forge Repository:** https://github.com/jedarden/bead-forge
- **Rust MSRV Policy:** https://rust-lang.github.io/rfcs/2495-min-rust-version.html

---

## Acceptance Criteria Verification

| Criterion | Status | Details |
|-----------|--------|---------|
| ✅ Complete list of required system libraries documented | Complete | All system dependencies for Linux, macOS, and Alpine documented |
| ✅ Minimum version requirements for each dependency identified | Complete | All dependencies with minimum versions listed |
| ✅ Platform-specific requirements noted (Linux vs macOS) | Complete | Linux (Debian/Ubuntu, Alpine), macOS, and NixOS covered |
| ✅ Documentation saved to project memory or docs directory | Complete | Document saved to `notes/bf-49y71-pluck-system-dependencies.md` |

---

## Related Documentation

- **Pluck Dependency Requirements:** `/home/coding/ARMOR/pluck-dependency-requirements.md`
- **Pluck Minimum Version Requirements:** `/home/coding/ARMOR/docs/pluck-minimum-version-requirements.md`
- **Pluck Tools Complete Version Reference:** `/home/coding/ARMOR/docs/pluck-tools-complete-version-reference.md`
- **Pluck Dependencies Documentation:** `/home/coding/ARMOR/docs/bf-29m3g-pluck-dependencies.md`
- **Dependency Requirements Summary:** `/home/coding/ARMOR/docs/pluck-dependency-requirements-summary.md`

---

## Task Completion Summary

**Bead:** bf-49y71  
**Task:** Identify Pluck system dependencies  
**Status:** ✅ COMPLETE

**Work Completed:**
1. ✅ Examined all existing Pluck documentation for system requirements
2. ✅ Identified all required libraries with minimum versions
3. ✅ Documented platform-specific dependencies (Linux, macOS, Alpine, NixOS)
4. ✅ Checked Pluck/NEEDLE source for hardcoded dependency checks (none found)
5. ✅ Compiled comprehensive dependency inventory with sources
6. ✅ Created verification commands for troubleshooting
7. ✅ Documented installation methods and procedures

**Key Findings:**
- Pluck requires Rust 1.75+ (MSRV)
- System dependencies: git, curl, jq, build-essential, pkg-config, libssl-dev (Linux)
- ARMOR adds Go 1.25.0 requirement
- Total of 25+ Cargo dependencies managed via Cargo.toml
- No hardcoded runtime version checks in source code
- Version compatibility enforced at build time via Cargo.toml

---

**Document Status:** ✅ Complete  
**Next Review:** When NEEDLE version updates or major dependency changes  
**Maintained By:** ARMOR Development Team  
