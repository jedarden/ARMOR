# Pluck Installed Dependency Versions

**Generated:** 2026-07-09  
**Environment:** ARMOR Workspace  
**Bead ID:** bf-64wgz  
**Host System:** NixOS 2.28.5

## Overview

This document captures the actual installed versions of all Pluck/NEEDLE dependencies and development tools in the current ARMOR environment. This serves as a baseline for comparison against requirements and for troubleshooting.

---

## Core Development Tools

| Tool | Installed Version | Minimum Required | Status | Installation Path |
|------|------------------|------------------|--------|-------------------|
| **Go** | go1.25.0 linux/amd64 | 1.25.0 | ✅ Compliant | System (NixOS) |
| **Rust** | rustc 1.96.1 (31fca3adb 2026-06-26) | 1.75+ | ✅ Compliant | ~/.cargo/bin/rustc |
| **Cargo** | 1.96.1 (356927216 2026-06-26) | (with rustc) | ✅ Compliant | ~/.cargo/bin/cargo |
| **rustup** | 1.29.0 (28d1352db 2026-03-05) | (any) | ✅ Installed | ~/.cargo/bin/rustup |
| **rustfmt** | 1.9.0-stable (31fca3adb2 2026-06-26) | (any) | ✅ Installed | via cargo-fmt |
| **clippy** | 0.1.96 (31fca3adb2 2026-06-26) | (any) | ✅ Installed | via cargo-clippy |
| **Git** | 2.50.1 | (system package) | ✅ Installed | /run/current-system/sw/bin/git |
| **curl** | 8.14.1 (x86_64-pc-linux-gnu) | (system package) | ✅ Installed | /run/current-system/sw/bin/curl |
| **jq** | 1.7.1 | (system package) | ✅ Installed | /run/current-system/sw/bin/jq |
| **nix-shell** | 2.28.5 | (NixOS) | ✅ Installed | System |

### Development Tool Details

**Rust Toolchain Components:**
- **Active channel:** stable (1.96.1)
- **Installed components:** rustfmt, clippy
- **Target platforms:** x86_64-unknown-linux-gnu (default)

**Additional Cargo Tools Available:**
- cargo-audit (18.3 MB) - Security vulnerability scanning
- cargo-bloat (1.4 MB) - Binary size analysis
- cargo-clippy (via rustup)
- cargo-cyclosedx (5.3 MB) - Software Bill of Materials (SBOM)
- cargo-deny (8.8 MB) - License/dependency auditing
- cargo-fmt (via rustup)
- cargo-fuzz (2.2 MB) - Fuzzing support
- cargo-llvm-cov (4.4 MB) - LLVM code coverage
- cargo-miri (via rustup) - Miri interpreter for Rust

---

## NEEDLE/Pluck Components

| Component | Version | Binary Location | Install Date | Status |
|-----------|---------|-----------------|--------------|--------|
| **NEEDLE CLI** | 0.2.11 | ~/.local/bin/needle | 2026-07-06 08:36:20 | ✅ Current |
| **br CLI (bf)** | 0.2.0 | ~/.local/bin/bf | 2026-06-24 14:06:20 | ✅ Current |
| **Pluck Strand** | (part of NEEDLE 0.2.11) | `needle strand pluck` | - | ✅ Available |

### Component Notes

**NEEDLE Installation:**
- Binary: `/home/coding/.local/bin/needle`
- Size: ~3-5 MB (typical stripped release build)
- Last updated: 2026-07-06
- Source: GitHub release (jedarden/NEEDLE)

**br CLI (bead-forge):**
- Binary: `/home/coding/.local/bin/bf` (br-compatible superset)
- Size: 50.4 MB (larger than NEEDLE due to embedded SQLite)
- Last updated: 2026-06-24
- Behavior: Outputs version as "Error: bf 0.2.0" (error handling convention)

---

## ARMOR Go Dependencies

**Total Dependencies:** 27 packages  
**Go Module:** github.com/jedarden/armor  
**Go Version:** 1.25.0

### Direct Dependencies

| Dependency | Version | Purpose |
|------------|---------|---------|
| github.com/aws/aws-sdk-go-v2 | v1.41.4 | AWS SDK for Go v2 |
| github.com/aws/aws-sdk-go-v2/config | v1.32.12 | AWS configuration loading |
| github.com/aws/aws-sdk-go-v2/credentials | v1.19.12 | AWS credential providers |
| github.com/aws/aws-sdk-go-v2/service/s3 | v1.97.2 | Amazon S3 client |
| github.com/kurin/blazer | v0.5.3 | Google Cloud Storage client |
| golang.org/x/crypto | v0.49.0 | Cryptographic primitives |
| golang.org/x/sync | v0.12.0 | Concurrency primitives |

### AWS SDK v2 Transitive Dependencies

| Dependency | Version | Purpose |
|------------|---------|---------|
| github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream | v1.7.8 | Event streaming protocol |
| github.com/aws/aws-sdk-go-v2/feature/ec2/imds | v1.18.20 | EC2 Instance Metadata Service |
| github.com/aws/aws-sdk-go-v2/internal/configsources | v1.4.20 | Internal config sources |
| github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 | v2.7.20 | Endpoint resolution |
| github.com/aws/aws-sdk-go-v2/internal/ini | v1.8.6 | INI file parsing |
| github.com/aws/aws-sdk-go-v2/internal/v4a | v1.4.21 | Signature v4a support |
| github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding | v1.13.7 | Accept-encoding handling |
| github.com/aws/aws-sdk-go-v2/service/internal/checksum | v1.9.12 | Checksum algorithms |
| github.com/aws/aws-sdk-go-v2/service/internal/presigned-url | v1.13.20 | Presigned URL support |
| github.com/aws/aws-sdk-go-v2/service/internal/s3shared | v1.19.20 | S3 shared utilities |
| github.com/aws/aws-sdk-go-v2/service/signin | v1.0.8 | AWS sign-in |
| github.com/aws/aws-sdk-go-v2/service/sso | v1.30.13 | AWS SSO integration |
| github.com/aws/aws-sdk-go-v2/service/ssooidc | v1.35.17 | AWS SSO OIDC |
| github.com/aws/aws-sdk-go-v2/service/sts | v1.41.9 | AWS Security Token Service |
| github.com/aws/smithy-go | v1.24.2 | Smithy code generation runtime |

### Golang.org/x Transitive Dependencies

| Dependency | Version | Purpose |
|------------|---------|---------|
| golang.org/x/net | v0.51.0 | Network utilities |
| golang.org/x/sys | v0.42.0 | System interfaces |
| golang.org/x/term | v0.41.0 | Terminal handling |
| golang.org/x/text | v0.35.0 | Text processing |

---

## NEEDLE Rust Dependencies

**Rust Version:** 1.75 (MSRV) - Currently using 1.96.1  
**NEEDLE Version:** 0.2.11  
**Source:** `/home/coding/NEEDLE/Cargo.toml`

### Runtime Dependencies (Required)

| Dependency | Version Spec | Features | Purpose |
|------------|--------------|----------|---------|
| **tokio** | ^1 | full | Async runtime |
| **serde** | ^1 | derive | Serialization framework |
| **serde_json** | ^1 | - | JSON serialization |
| **serde_yaml** | ^0.9 | - | YAML serialization |
| **clap** | ^4 | derive | CLI argument parsing |
| **anyhow** | ^1 | - | Error handling |
| **thiserror** | ^1 | - | Error derivation |
| **tracing** | ^0.1 | - | Structured logging |
| **tracing-subscriber** | ^0.3 | env-filter, json | Log formatting |
| **chrono** | ^0.4 | serde | Time handling |
| **which** | ^4 | - | Executable location |
| **async-trait** | ^0.1 | - | Async trait support |
| **fs2** | ^0.4 | - | File locking |
| **sha2** | ^0.10 | - | SHA-2 hashing |
| **hex** | ^0.4 | - | Hex encoding |
| **regex** | ^1 | - | Regular expressions |
| **glob** | ^0.3 | - | Glob patterns |
| **aho-corasick** | ^1 | - | Multi-pattern search |
| **ureq** | ^2 | - | HTTP client |
| **cfg-if** | ^1 | - | Conditional compilation |
| **atty** | ^0.2 | - | Terminal detection |
| **toml** | ^0.8 | - | TOML parsing |
| **libc** | ^0.2 | - | Unix FFI |
| **rand** | ^0.8 | - | Random number generation |
| **futures** | ^0.3 | - | Future utilities |
| **gethostname** | ^0.4 | - | Hostname retrieval |

### OpenTelemetry Dependencies (Optional - `otlp` feature)

| Dependency | Version Spec | Features | Purpose |
|------------|--------------|----------|---------|
| **opentelemetry** | ^0.31 | - | OpenTelemetry API |
| **opentelemetry_sdk** | ^0.31 | rt-tokio | OpenTelemetry SDK |
| **opentelemetry-otlp** | ^0.31 | grpc-tonic, http-proto | OTLP exporter |
| **opentelemetry-semantic-conventions** | ^0.31 | - | Semantic conventions |
| **tonic** | ^0.14 | - | gRPC for OTLP |
| **tracing-opentelemetry** | ^0.32 | - | Tracing bridge |

### Development Dependencies

| Dependency | Version Spec | Purpose |
|------------|--------------|---------|
| **tokio-test** | ^0.4 | Tokio testing utilities |
| **tempfile** | ^3 | Temporary file management |
| **proptest** | ^1 | Property-based testing |
| **filetime** | ^0.2 | File time manipulation |
| **criterion** | ^0.5 | Benchmarking |

### Integration Test Dependencies (Optional - `integration` feature)

| Dependency | Version Spec | Purpose |
|------------|--------------|---------|
| **testcontainers** | ^0.23 | Docker container testing |

---

## System Environment

**Operating System:** NixOS  
**Nix Version:** 2.28.5  
**Package Manager:** Nix (declarative)  
**Shell:** bash (via /run/current-system/sw/bin/bash)

### Available System Tools

**Version Control:**
- git 2.50.1

**HTTP/Networking:**
- curl 8.14.1 (with OpenSSL 3.4.3, HTTP/2 support)

**Data Processing:**
- jq 1.7.1

**Build Tools:**
- GCC toolchain (via NixOS build-essential equivalent)
- pkg-config (via NixOS)
- OpenSSL development headers (libssl-dev equivalent)

**Not Available:**
- GitHub CLI (gh) - not installed
- dpkg - not applicable (NixOS)
- apt - not applicable (NixOS)

---

## Feature Flags

### NEEDLE/Pluck Features

**Default Features:**
- `otlp` - OpenTelemetry/OTLP telemetry support

**Optional Features:**
- `integration` - Integration testing with testcontainers (requires Docker)

### Disabled Features

- None - all default features are enabled

---

## Installation Verification Commands

**Verify Core Tools:**
```bash
go version          # Expected: go version go1.25.0
rustc --version     # Expected: rustc 1.96.1 (2026-06-26)
cargo --version     # Expected: cargo 1.96.1 (2026-06-26)
git --version       # Expected: git version 2.50.1
curl --version      # Expected: curl 8.14.1
jq --version        # Expected: jq-1.7.1
```

**Verify NEEDLE/Pluck:**
```bash
needle --version    # Expected: needle 0.2.11
bf --version        # Expected: Error: bf 0.2.0
```

**Verify Rust Components:**
```bash
rustfmt --version   # Expected: rustfmt 1.9.0-stable
cargo clippy --version  # Expected: clippy 0.1.96 (31fca3adb2 2026-06-26)
```

**Verify Go Dependencies:**
```bash
cd /home/coding/ARMOR
go list -m all | grep github.com/aws  # Expected: v1.41.4
go list -m all | grep golang.org/x    # Expected: various v0.35-0.51 versions
```

---

## Compliance Status

### Summary

| Category | Compliant | Notes |
|----------|-----------|-------|
| **Rust Toolchain** | ✅ Yes | 1.96.1 > MSRV 1.75 |
| **Go Toolchain** | ✅ Yes | 1.25.0 = Required 1.25.0 |
| **System Dependencies** | ✅ Yes | All required via NixOS |
| **NEEDLE Installation** | ✅ Yes | 0.2.11 is current |
| **br CLI** | ✅ Yes | 0.2.0 is current |
| **ARMOR Dependencies** | ✅ Yes | All 27 packages resolved |
| **NEEDLE Dependencies** | ✅ Yes | All runtime deps available |

### Issues Found

**None** - All dependencies are installed and meet or exceed requirements.

---

## Dependency Refresh

**Last Verified:** 2026-07-09  
**Verification Method:** Direct version checks and package listings

### Update Recommendations

**Current Status:** No updates needed

**Future Updates to Monitor:**
- AWS SDK v2 updates (security patches)
- Rust stable releases (performance improvements)
- golang.org/x dependencies (security patches)

---

## Documentation Sources

**Verification Data From:**
- Direct binary version checks
- `/home/coding/NEEDLE/Cargo.toml` - NEEDLE dependency specifications
- `/home/coding/ARMOR/go.mod` - ARMOR Go module requirements
- `go list -m all` - Actual resolved Go dependencies
- `ls -la ~/.local/bin/` - Binary installation timestamps
- `rustup show` - Rust toolchain details

---

## Appendix: Quick Reference

### Critical Paths

| Component | Location |
|------------|----------|
| NEEDLE binary | ~/.local/bin/needle |
| br CLI binary | ~/.local/bin/bf |
| NEEDLE source | ~/NEEDLE/ |
| ARMOR workspace | ~/ARMOR/ |
| Rust toolchain | ~/.cargo/bin/ |
| Go module | ~/ARMOR/go.mod |

### Version Matrix

| Tool | Minimum | Installed | Status |
|------|---------|-----------|--------|
| Go | 1.25.0 | 1.25.0 | ✅ Exact match |
| Rust | 1.75+ | 1.96.1 | ✅ Exceeds minimum |
| NEEDLE | - | 0.2.11 | ✅ Current |
| br CLI | - | 0.2.0 | ✅ Current |

---

**End of Document**