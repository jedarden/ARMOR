# Pluck Dependencies Verification Report

**Bead:** bf-5b04s  
**Date:** 2026-07-09  
**Status:** ✅ Complete

## Overview

This report verifies that all documented dependencies for Pluck (from `bf-29m3g`) are actually installed on the system.

## System Information

- **OS:** NixOS 25.05 (Warbler)
- **Architecture:** x86_64
- **Kernel:** Linux 6.12.63
- **Platform:** NixOS (not Debian-based - uses Nix package manager)

## Required Dependencies - Status

### Build Tools ✅

| Dependency | Required Version | Installed Version | Status | Location |
|------------|-----------------|-------------------|--------|----------|
| gcc | build-essential | GCC 13.3.0 | ✅ Installed | `/nix/store/...` |
| make | build-essential | GNU Make 4.4.1 | ✅ Installed | `/nix/store/...` |
| pkg-config | pkg-config | 0.29.2 | ✅ Installed | `/run/current-system/sw/bin/pkg-config` |
| libssl-dev | libssl-dev | OpenSSL 3.6.2 | ✅ Installed | `/nix/store/...-openssl-3.6.2/` |

**Note:** On NixOS, `libssl-dev` equivalent is provided by the `openssl` package in the Nix store.

### Version Control ✅

| Dependency | Required Version | Installed Version | Status | Location |
|------------|-----------------|-------------------|--------|----------|
| git | git | git version 2.50.1 | ✅ Installed | `/run/current-system/sw/bin/git` |

### Network Utilities ✅

| Dependency | Required Version | Installed Version | Status | Location |
|------------|-----------------|-------------------|--------|----------|
| curl | curl | curl 8.14.1 | ✅ Installed | `/run/current-system/sw/bin/curl` |
| jq | jq | jq-1.7.1 | ✅ Installed | `/run/current-system/sw/bin/jq` |

### Rust Toolchain ✅

| Dependency | Required Version | Installed Version | Status | Location |
|------------|-----------------|-------------------|--------|----------|
| rustc | 1.75+ | rustc 1.96.1 | ✅ Installed | `/home/coding/.rustup/toolchains/stable-x86_64-unknown-linux-gnu/` |
| cargo | (with Rust) | cargo 1.96.1 | ✅ Installed | `/home/coding/.rustup/toolchains/stable-x86_64-unknown-linux-gnu/` |

**Note:** Rust 1.96.1 exceeds the minimum requirement of 1.75+ specified in NEEDLE's `rust-toolchain.toml`.

### Optional Dependencies ⚠️

| Dependency | Required Version | Installed Version | Status | Location |
|------------|-----------------|-------------------|--------|----------|
| gh CLI | optional | NOT FOUND | ❌ Missing | N/A (optional for CI) |
| docker | optional (tests) | Docker 27.5.1 | ✅ Installed | `/run/current-system/sw/bin/docker` |

**Note:** GitHub CLI (`gh`) is optional for CI workflows. Its absence does not block Pluck development or runtime.

## Rust Crate Dependencies (Cargo.toml)

All required Rust crates are properly specified in `/home/coding/NEEDLE/Cargo.toml`:

### Core Dependencies ✅

- ✅ tokio (v1, features: full) - Async runtime
- ✅ serde (v1, features: derive) - Serialization
- ✅ serde_json (v1) - JSON support
- ✅ serde_yaml (v0.9) - YAML support
- ✅ clap (v4, features: derive) - CLI parsing
- ✅ anyhow (v1) - Error handling
- ✅ thiserror (v1) - Error derives
- ✅ tracing (v0.1) - Logging framework
- ✅ tracing-subscriber (v0.3, features: env-filter, json) - Log routing
- ✅ chrono (v0.4, features: serde) - Time handling
- ✅ which (v4) - Executable location
- ✅ async-trait (v0.1) - Async traits
- ✅ fs2 (v0.4) - File locking
- ✅ sha2 (v0.10) - Hashing
- ✅ hex (v0.4) - Hex encoding
- ✅ regex (v1) - Pattern matching
- ✅ glob (v0.3) - Glob patterns
- ✅ ureq (v2) - HTTP client
- ✅ aho-corasick (v1) - Multi-pattern search
- ✅ cfg-if (v1) - Conditional compilation
- ✅ atty (v0.2) - Terminal detection
- ✅ toml (v0.8) - TOML parsing
- ✅ libc (v0.2) - Unix process handling
- ✅ rand (v0.8) - Random jitter
- ✅ futures (v0.3) - Async utilities
- ✅ gethostname (v0.4) - Hostname detection

### OpenTelemetry Dependencies (Optional, feature-gated) ✅

All OTLP dependencies are properly specified as optional and gated behind the `otlp` feature:

- ✅ opentelemetry (v0.31, optional)
- ✅ opentelemetry_sdk (v0.31, features: rt-tokio, optional)
- ✅ opentelemetry-otlp (v0.31, features: grpc-tonic, http-proto, optional)
- ✅ opentelemetry-semantic-conventions (v0.31, optional)
- ✅ tonic (v0.14, optional)
- ✅ tracing-opentelemetry (v0.32, optional)

### Development Dependencies ✅

- ✅ tokio-test (v0.4) - Tokio testing
- ✅ tempfile (v3) - Temporary files
- ✅ proptest (v1) - Property-based testing
- ✅ filetime (v0.2) - File time manipulation
- ✅ criterion (v0.5) - Benchmarking
- ✅ testcontainers (v0.23, optional, feature: integration) - Integration tests

## Installation Paths Summary

### System Tools (NixOS)
- Location: `/run/current-system/sw/bin/` and `/nix/store/`
- Package manager: Nix (not apt/dpkg)
- All build tools and utilities managed by NixOS configuration

### Rust Toolchain
- Location: `/home/coding/.rustup/toolchains/stable-x86_64-unknown-linux-gnu/`
- Managed by: rustup
- Components: rustc, cargo, rustfmt, clippy

### OpenSSL Libraries
- Location: `/nix/store/...-openssl-3.6.2/`
- Libraries: `libssl.so`, `libcrypto.so`
- Headers: Available in system include paths

### NEEDLE Project
- Location: `/home/coding/NEEDLE/`
- Cargo.toml: Present and valid
- rust-toolchain.toml: Configured for stable Rust with required components

## Missing Dependencies

### 1. GitHub CLI (gh) - OPTIONAL
- **Purpose:** CI workflows, GitHub API interaction
- **Impact:** LOW - Only needed for GitHub-based CI automation
- **Installation:** Not required for Pluck development or runtime
- **Recommendation:** Install if GitHub automation workflows are needed

## Compliance with Documentation

### vs. notes/pluck-dependencies.md (bf-29m3g)

All dependencies documented in `notes/pluck-dependencies.md` have been verified:

| Category | Documented | Verified | Status |
|----------|------------|----------|--------|
| Build Tools | ✅ | ✅ | All present |
| Version Control | ✅ | ✅ | All present |
| Network Utils | ✅ | ✅ | All present |
| Rust Toolchain | ✅ | ✅ | All present (exceeds minimum) |
| SSL Libraries | ✅ | ✅ | Present (NixOS equivalent) |
| Optional (gh) | ✅ | ❌ | Missing but optional |
| Optional (docker) | ✅ | ✅ | Present |

## Verification Commands

To verify dependencies in the future:

```bash
# Check system tools
gcc --version
make --version
pkg-config --version

# Check version control
git --version

# Check network utils
curl --version
jq --version

# Check Rust toolchain
rustc --version
cargo --version

# Check SSL libraries
find /nix/store -name "libssl*" | head -5

# Check optional dependencies
docker --version
gh --version  # Will fail if not installed
```

## Conclusions

### ✅ All Required Dependencies Present
- All build tools, version control, network utilities, and Rust toolchain dependencies are installed
- System is fully capable of building and running Pluck
- Rust version (1.96.1) exceeds minimum requirement (1.75+)

### ⚠️ One Optional Dependency Missing
- GitHub CLI (`gh`) is not installed but is optional
- Does not block Pluck development, testing, or runtime
- Can be installed later if GitHub automation workflows are needed

### ✅ Cargo Dependencies Properly Configured
- All required Rust crates specified in Cargo.toml
- Optional dependencies properly gated behind features
- Development dependencies present for testing

### ✅ NixOS Compatibility
- All Debian package equivalents available in Nix store
- OpenSSL development libraries present
- Build toolchain complete

## Recommendations

1. **No action required** - All required dependencies are present
2. **Optional:** Install GitHub CLI if GitHub automation is needed: `nix-env -iA nixos.gh`
3. **Verification complete** - System is ready for Pluck development and runtime

---

**Verification Method:** Manual check of each documented dependency using command-line tools and system inspection.  
**Documentation Source:** `notes/pluck-dependencies.md` (bead bf-29m3g)  
**Verification Date:** 2026-07-09
