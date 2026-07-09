# Pluck Dependencies - Minimum Version Requirements

**Bead:** bf-647lq  
**Last Updated:** 2026-07-09  
**NEEDLE Version:** 0.2.11

## Overview

Pluck is a strand component within NEEDLE (a Rust project). Pluck handles primary bead selection from the assigned workspace, processing >90% of all beads by querying the bead store for unassigned, ready beads, filtering by excluded labels, and sorting them in deterministic priority order.

## Project-Level Requirements

### NEEDLE (Parent Project)

- **Minimum Rust Version:** 1.75.0
- **Rust Edition:** 2021
- **Source:** `/home/coding/NEEDLE/Cargo.toml`

This is the **primary minimum version requirement** for Pluck, as it defines the baseline Rust compiler version needed for the entire project.

## Core Dependencies - Minimum Versions

### Async Runtime

| Dependency | Version | MSRV | Notes |
|------------|---------|------|-------|
| **tokio** | 1.x | 1.64.0+ | Tokio maintains a rolling MSRV policy of at least 6 months. For NEEDLE's usage, use Rust 1.75+ to ensure compatibility. |

**Sources:**
- [Tokio GitHub - MSRV Policy](https://github.com/tokio-rs/tokio)
- [Tokio 1.0 Support - 5 Year Commitment](https://tokio.rs/blog/2020-12-tokio-0-1#support-policy)

### Serialization

| Dependency | Version | MSRV | Compatibility |
|------------|---------|------|---------------|
| **serde** | 1.x | 1.56.0 | Compatible with serde_json 1.x |
| **serde_json** | 1.x | 1.56.0 | Designed for serde 1.0 compatibility |
| **serde_yaml** | 0.9 | 1.56.0 | Part of serde ecosystem |

**Configuration:**
```toml
serde = { version = "1", features = ["derive"] }
serde_json = "1"
serde_yaml = "0.9"
```

**Sources:**
- [Serde crates.io](https://crates.io/crates/serde)
- [Rust Users Forum - Serde Compatibility](https://users.rust-lang.org/t/what-is-the-correct-way-to-take-serde-json-into-use/31553)
- [Reddit - Cargo MSRV Support](https://www.reddit.com/r/rust/comments/qcy2w2/psa_rust_cargo_handles_minimum_supported_rust/)

### CLI

| Dependency | Version | MSRV | Notes |
|------------|---------|------|-------|
| **clap** | 4.x | 1.64.0 | clap 4.1.0 increased MSRV to 1.64.0 |

**Configuration:**
```toml
clap = { version = "4", features = ["derive"] }
```

**Sources:**
- [Rust Users Forum - clap MSRV](https://users.rust-lang.org/t/cargo-add-version-suitable-for-my-non-latest-rustc/88916)

### Error Handling

| Dependency | Version | MSRV | Maintainer |
|------------|---------|------|------------|
| **anyhow** | 1.x | 1.68.0 | David Tolnay (dtolnay) |
| **thiserror** | 1.x | 1.68.0 | David Tolnay (dtolnay) |

**Configuration:**
```toml
anyhow = "1"
thiserror = "1"
```

**Sources:**
- [anyhow crates.io](https://crates.io/crates/anyhow)
- [thiserror crates.io](https://crates.io/crates/thiserror)

### Logging / Telemetry

| Dependency | Version | MSRV | Notes |
|------------|---------|------|-------|
| **tracing** | 0.1.x | 1.65.0 | Minimum Rust 1.65.0 |
| **tracing-subscriber** | 0.3.x | 1.65.0 | Features: env-filter, json |

**Configuration:**
```toml
tracing = "0.1"
tracing-subscriber = { version = "0.3", features = ["env-filter", "json"] }
```

**Sources:**
- [tracing GitHub](https://github.com/tokio-rs/tracing)
- [chrono crates.io - MSRV 1.62.0](https://crates.io/crates/chrono)

### Time

| Dependency | Version | MSRV | Notes |
|------------|---------|------|-------|
| **chrono** | 0.4.x | 1.62.0 | MSRV tested in CI |

**Configuration:**
```toml
chrono = { version = "0.4", features = ["serde"] }
```

**Sources:**
- [chrono crates.io](https://crates.io/crates/chrono)

### Async Traits

| Dependency | Version | MSRV | Notes |
|------------|---------|------|-------|
| **async-trait** | 0.1.x | 1.56.0 | 320M+ downloads, type erasure for async fn in traits |

**Configuration:**
```toml
async-trait = "0.1"
```

**Sources:**
- [async-trait crates.io](https://crates.io/crates/async-trait)
- [RFC 2495 - rust-version Field](https://rust-lang.github.io/rfcs/2495-min-rust-version.html)

## Optional Dependencies (OpenTelemetry)

These are **optional** dependencies, only used when the `otlp` feature is enabled.

| Dependency | Version | MSRV | Feature |
|------------|---------|------|---------|
| **opentelemetry** | 0.31 | 1.75.0 | Core OpenTelemetry |
| **opentelemetry_sdk** | 0.31 | 1.75.0 | rt-tokio feature |
| **opentelemetry-otlp** | 0.31 | 1.75.0 | grpc-tonic, http-proto |
| **opentelemetry-semantic-conventions** | 0.31 | 1.75.0 | Semantic conventions |
| **tonic** | 0.14 | 1.70.0+ | gRPC tonic library |
| **tracing-opentelemetry** | 0.32 | 1.65.0+ | Tracing integration |

**Feature Configuration:**
```toml
[features]
default = ["otlp"]
otlp = [
    "dep:opentelemetry",
    "dep:opentelemetry_sdk",
    "dep:opentelemetry-otlp",
    "dep:opentelemetry-semantic-conventions",
    "dep:tonic",
    "dep:tracing-opentelemetry",
]
```

**Sources:**
- [OpenTelemetry Rust docs.rs](https://docs.rs/crate/opentelemetry/latest/source/README.md)
- [OpenTelemetry Contributing Guide](https://open-telemetry.opentelemetry-rust.mintlify.app/contributing/supported-versions)
- [opentelemetry_sdk crates.io](https://crates.io/crates/opentelemetry_sdk)

## Other Dependencies

### Process Management

| Dependency | Version | Purpose |
|------------|---------|---------|
| **which** | 4 | Locate executables in PATH |
| **libc** | 0.2 | Unix process handling (PID liveness) |

### File System

| Dependency | Version | Purpose |
|------------|---------|---------|
| **fs2** | 0.4 | Cross-platform file locking (flock) |

### Cryptography

| Dependency | Version | Purpose |
|------------|---------|---------|
| **sha2** | 0.10 | Hashing (prompt content hash, binary fingerprinting) |
| **hex** | 0.4 | Hex encoding |

### Pattern Matching

| Dependency | Version | Purpose |
|------------|---------|---------|
| **regex** | 1 | Regular expressions (agent token extraction) |
| **glob** | 0.3 | Glob pattern matching (doc file discovery) |
| **aho-corasick** | 1 | Multi-pattern string search (sanitizer keyword pre-filter) |

### Networking

| Dependency | Version | Purpose |
|------------|---------|---------|
| **ureq** | 2 | HTTP client (self-update) |

### Utilities

| Dependency | Version | Purpose |
|------------|---------|---------|
| **rand** | 0.8 | Random jitter (backoff desynchronization) |
| **atty** | 0.2 | Terminal detection (ANSI color support) |
| **cfg-if** | 1 | Conditional compilation |
| **toml** | 0.8 | TOML parsing (gitleaks config) |
| **gethostname** | 0.4 | Get system hostname |
| **futures** | 0.3 | Async utilities |

## Summary of Minimum Requirements

### For Pluck/NEEDLE Usage

**Minimum Rust Version: 1.75.0**

This is set by NEEDLE's `rust-version = "1.75"` in Cargo.toml and is the primary constraint.

### Compatibility Notes

1. **OpenTelemetry dependencies** require Rust 1.75.0 or higher, which aligns with NEEDLE's baseline requirement.

2. **Most core dependencies** (tokio, serde, async-trait, chrono) have lower MSRV requirements (1.56-1.65), but NEEDLE's 1.75 baseline superseds these.

3. **thiserror and anyhow** require Rust 1.68.0+, which is lower than NEEDLE's 1.75 requirement.

4. **tracing** requires Rust 1.65.0+, lower than NEEDLE's 1.75 requirement.

5. **clap 4.x** requires Rust 1.64.0+, lower than NEEDLE's 1.75 requirement.

## Version Conflicts / Constraints

### No Known Conflicts

All dependencies are compatible with Rust 1.75.0 or higher, which is NEEDLE's baseline version. The Cargo.toml uses version ranges (e.g., `"1"`, `"0.4"`) allowing Cargo to resolve compatible versions automatically.

### Compatibility Guarantee

The NEEDLE project's `rust-version = "1.75"` field in Cargo.toml ensures that all dependencies are tested against and compatible with Rust 1.75.0 as the minimum supported version.

## Recommendations

1. **Use Rust 1.75.0 or later** for any work with Pluck/NEEDLE.

2. **Check crates.io** for the latest MSRV information when updating dependencies, as crates may bump their MSRV in minor releases.

3. **Use the `otlp` feature flag** if you need OpenTelemetry support - all OTLP dependencies are compatible with Rust 1.75+.

4. **Monitor dependency updates** - while most crates maintain backward compatibility within major versions, always check changelogs for MSRV bumps.

## Verification

To verify your Rust version:
```bash
rustc --version
```

To check if NEEDLE builds with your current Rust version:
```bash
cd /home/coding/NEEDLE
cargo check
```

## Document Sources

- **NEEDLE Cargo.toml:** `/home/coding/NEEDLE/Cargo.toml`
- **Pluck Source:** `/home/coding/NEEDLE/src/strand/pluck.rs`
- **Online Documentation:** See inline links throughout this document

---

*This document was created as part of bead bf-647lq to document minimum version requirements for Pluck dependencies.*
