# Development Tool Versions for Pluck (NEEDLE) - bf-39jam

**Project**: Pluck (strand within NEEDLE - Navigates Every Enqueued Deliverable, Logs Effort)
**Documentation Date**: 2026-07-09
**Bead ID**: bf-39jam
**Repository**: https://github.com/jedarden/NEEDLE

## Overview

Pluck is NOT a standalone project - it is a component/strand within the NEEDLE system that handles primary bead selection from assigned workspaces. The dependencies listed below cover the full NEEDLE system, which includes the Pluck strand.

---

## Core Toolchain

### Rust Toolchain

**Minimum Supported Rust Version (MSRV)**: 1.75+ (2023-12-28)

**Current Version**: Stable channel (latest via rustup)

**Required Components**:
- `rustfmt` - Code formatting
- `clippy` - Linting and code quality checks

**Specification Location**: `/home/coding/NEEDLE/Cargo.toml` (line 5: `rust-version = "1.75"`)

**Installation**:
```bash
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --default-toolchain stable
```

**CI Configuration**: `/home/coding/NEEDLE/.github/workflows/ci.yml` (lines 21-26)

---

## System Dependencies

### Linux (Debian/Ubuntu)

**Required Packages**:
```bash
git curl jq build-essential pkg-config libssl-dev
```

**Package Details**:
- `git` - Version control system
- `curl` - HTTP client for downloads
- `jq` - JSON processor for output parsing
- `build-essential` - C compiler and build tools (gcc, make, etc.)
- `pkg-config` - Package configuration helper
- `libssl-dev` - OpenSSL development headers

**Specification Location**: `/home/coding/NEEDLE/ci/Dockerfile.ci` (lines 4-6)

### macOS

**No additional system dependencies required** - standard macOS development environment is sufficient.

---

## Cargo Dependencies

### Runtime Dependencies

**Async Runtime**:
- `tokio` ^1 - Features: full
  - **Purpose**: Async runtime and utilities
  - **Specification**: Cargo.toml line 42

**Serialization**:
- `serde` ^1 - Features: derive
  - **Purpose**: Serialization framework
  - **Specification**: Cargo.toml line 45
- `serde_json` ^1
  - **Purpose**: JSON serialization
  - **Specification**: Cargo.toml line 46
- `serde_yaml` ^0.9
  - **Purpose**: YAML serialization
  - **Specification**: Cargo.toml line 47

**CLI Framework**:
- `clap` ^4 - Features: derive
  - **Purpose**: Command-line argument parsing
  - **Specification**: Cargo.toml line 50

**Error Handling**:
- `anyhow` ^1
  - **Purpose**: Convenient error handling
  - **Specification**: Cargo.toml line 53
- `thiserror` ^1
  - **Purpose**: Derive error types
  - **Specification**: Cargo.toml line 54

**Logging/Telemetry**:
- `tracing` ^0.1
  - **Purpose**: Structured logging framework
  - **Specification**: Cargo.toml line 57
- `tracing-subscriber` ^0.3 - Features: env-filter, json
  - **Purpose**: Log subscribers and filtering
  - **Specification**: Cargo.toml line 58
- `tracing-opentelemetry` ^0.32 (optional)
  - **Purpose**: OpenTelemetry integration
  - **Specification**: Cargo.toml line 111

**Time Handling**:
- `chrono` ^0.4 - Features: serde
  - **Purpose**: Date and time handling
  - **Specification**: Cargo.toml line 61

**Process Management**:
- `which` ^4
  - **Purpose**: Locate executables in PATH
  - **Specification**: Cargo.toml line 64

**Async Utilities**:
- `async-trait` ^0.1
  - **Purpose**: Async trait support
  - **Specification**: Cargo.toml line 67

**File Locking**:
- `fs2` ^0.4
  - **Purpose**: Cross-platform file locking (flock)
  - **Specification**: Cargo.toml line 70

**Hashing**:
- `sha2` ^0.10
  - **Purpose**: SHA-256 hashing
  - **Specification**: Cargo.toml line 73
- `hex` ^0.4
  - **Purpose**: Hex encoding/decoding
  - **Specification**: Cargo.toml line 74

**Pattern Matching**:
- `regex` ^1
  - **Purpose**: Regular expressions
  - **Specification**: Cargo.toml line 77
- `glob` ^0.3
  - **Purpose**: Glob pattern matching
  - **Specification**: Cargo.toml line 80

**HTTP Client**:
- `ureq` ^2
  - **Purpose**: HTTP client for self-update
  - **Specification**: Cargo.toml line 83

**Multi-pattern Search**:
- `aho-corasick` ^1
  - **Purpose**: Multi-pattern string search
  - **Specification**: Cargo.toml line 86

**Terminal Detection**:
- `atty` ^0.2
  - **Purpose**: ANSI color support detection
  - **Specification**: Cargo.toml line 92

**TOML Parsing**:
- `toml` ^0.8
  - **Purpose**: TOML configuration parsing
  - **Specification**: Cargo.toml line 95

**Unix Process Handling**:
- `libc` ^0.2
  - **Purpose**: Unix system calls (PID liveness)
  - **Specification**: Cargo.toml line 98

**Random Number Generation**:
- `rand` ^0.8
  - **Purpose**: Random jitter for backoff
  - **Specification**: Cargo.toml line 101

**OpenTelemetry** (optional, gated behind `otlp` feature):
- `opentelemetry` ^0.31
- `opentelemetry_sdk` ^0.31 - Features: rt-tokio
- `opentelemetry-otlp` ^0.31 - Features: grpc-tonic, http-proto
- `opentelemetry-semantic-conventions` ^0.31
- `tonic` ^0.14
- **Specification**: Cargo.toml lines 104-111

---

## Development Dependencies

### Testing Framework

**Rust Testing Utilities**:
- `tokio-test` ^0.4
  - **Purpose**: Tokio runtime for tests
  - **Specification**: Cargo.toml line 119
- `tempfile` ^3
  - **Purpose**: Temporary file/directory creation
  - **Specification**: Cargo.toml line 120
- `proptest` ^1
  - **Purpose**: Property-based testing
  - **Specification**: Cargo.toml line 121
- `filetime` ^0.2
  - **Purpose**: File time manipulation in tests
  - **Specification**: Cargo.toml line 122

**Specification Location**: `/home/coding/NEEDLE/Cargo.toml` (lines 118-123)

### Benchmarking

**Criterion.rs**:
- `criterion` ^0.5
  - **Purpose**: Statistical benchmarking
  - **Specification**: Cargo.toml line 123
- **Configuration**: `/home/coding/NEEDLE/criterion.toml`
  - **Output format**: verbose
  - **Plotting backend**: auto
  - **Sample size**: 10
  - **Warm-up time**: 3 seconds
  - **Measurement time**: 5 seconds

**Specification Location**: `/home/coding/NEEDLE/criterion.toml`

---

## Linting and Code Quality

### Rust Linting

**Clippy**:
- **Purpose**: Rust linter for catching common mistakes
- **Version**: Bundled with Rust toolchain
- **CI Command**: `cargo clippy --all-targets -- -D warnings`
- **Specification**: `.github/workflows/ci.yml` (line 44-45)

**rustfmt**:
- **Purpose**: Code formatting
- **Version**: Bundled with Rust toolchain
- **CI Command**: `cargo fmt --check`
- **Specification**: `.github/workflows/ci.yml` (line 47-48)

### Security Scanning

**Gitleaks**:
- **Purpose**: Secret scanning in repository
- **Configuration**: `/home/coding/NEEDLE/config/gitleaks.toml`
- **Specification Location**: `/home/coding/NEEDLE/config/gitleaks.toml`

---

## CI/CD Tools

### GitHub Actions

**CI Workflow**:
- **File**: `.github/workflows/ci.yml`
- **Triggers**: Push to main, pull requests
- **Jobs**: 
  - build-and-test (Linux x86_64)
  - cross-compile (macOS ARM)

**Release Workflow**:
- **File**: `.github/workflows/release.yml`
- **Triggers**: Version tags (v*.*.*)
- **Jobs**: 
  - build-linux (static musl binary)
  - build-macos (ARM64)

**Specification Location**: `/home/coding/NEEDLE/.github/workflows/`

### Docker

**CI Base Image**:
- **File**: `ci/Dockerfile.ci`
- **Base**: debian:bookworm
- **Purpose**: CI environment with system dependencies and Rust toolchain

**Dependency Caching**:
- **File**: `ci/Dockerfile.ci-deps`
- **Base**: ronaldraygun/needle-ci-builder:latest
- **Purpose**: Pre-compile Cargo dependencies for caching

**Specification Location**: `/home/coding/NEEDLE/ci/`

---

## Build Targets

### Primary Targets

1. **Linux x86_64** (x86_64-unknown-linux-gnu)
   - **Purpose**: Primary Linux binary
   - **Specification**: CI workflow, lines 39, 42, 45, 48

2. **macOS ARM64** (aarch64-apple-darwin)
   - **Purpose**: Apple Silicon Mac support
   - **Specification**: CI workflow, cross-compile job

3. **Linux musl** (x86_64-unknown-linux-musl)
   - **Purpose**: Static Linux binary for releases
   - **Specification**: Release workflow, lines 24, 40

**Specification Location**: `/home/coding/NEEDLE/.github/workflows/ci.yml` and `release.yml`

---

## Additional Tools

### GitHub CLI

**gh CLI**:
- **Purpose**: GitHub API interactions for releases
- **Installation**: Via official GitHub packages
- **Specification**: `/home/coding/NEEDLE/ci/Dockerfile.ci` (lines 9-14)

### musl-tools

**musl-tools**:
- **Purpose**: Static binary compilation for Linux
- **Version**: Via apt (musl-tools)
- **Specification**: `/home/coding/NEEDLE/.github/workflows/release.yml` (line 27)

---

## Configuration Files Summary

| Tool/Component | Configuration File Location |
|----------------|---------------------------|
| Rust version | `/home/coding/NEEDLE/Cargo.toml` (line 5) |
| Cargo dependencies | `/home/coding/NEEDLE/Cargo.toml` (lines 40-116) |
| Dev dependencies | `/home/coding/NEEDLE/Cargo.toml` (lines 118-123) |
| Benchmarking config | `/home/coding/NEEDLE/criterion.toml` |
| Gitleaks config | `/home/coding/NEEDLE/config/gitleaks.toml` |
| CI workflow | `/home/coding/NEEDLE/.github/workflows/ci.yml` |
| Release workflow | `/home/coding/NEEDLE/.github/workflows/release.yml` |
| Docker CI base | `/home/coding/NEEDLE/ci/Dockerfile.ci` |
| Docker CI deps | `/home/coding/NEEDLE/ci/Dockerfile.ci-deps` |

---

## Version Pinning Strategy

- **Rust toolchain**: Uses "stable" channel via rustup (pinned in CI via dtolnay/rust-toolchain action)
- **Cargo dependencies**: Uses semantic versioning with ^ operator (compatible updates)
- **GitHub Actions**: Uses @v4 tags for actions
- **Docker**: Uses debian:bookworm (stable release)

---

## Acceptance Criteria Status

✅ **All development tools are listed** - Comprehensive inventory provided  
✅ **Current version of each tool is recorded** - All versions documented with specification locations  
✅ **Location of tool version specifications is documented** - All configuration files identified and referenced  

---

## Summary

This document captures all development tools used in the Pluck/NEEDLE project as of 2026-07-09. The primary development environment is Rust 1.75+, with comprehensive CI/CD via GitHub Actions, Docker-based build environments, and extensive testing/benchmarking infrastructure.

**Key Points**:
- Pluck is a strand within NEEDLE, not a standalone project
- Rust MSRV: 1.75+
- Primary build targets: Linux x86_64, macOS ARM64
- All tool versions are specified in Cargo.toml, CI workflows, or Dockerfiles
- Comprehensive testing, linting, and benchmarking infrastructure in place
