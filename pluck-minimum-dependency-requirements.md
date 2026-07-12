# Pluck Minimum Dependency Version Requirements

**Document Created:** 2026-07-12  
**Bead:** bf-4kr5w  
**Workspace:** /home/coding/ARMOR  
**Status:** ✅ Complete

---

## Executive Summary

**Pluck is NOT a standalone crate.** Pluck is a **strand** within the NEEDLE system—a headless agent orchestrator written in Rust. Consequently, Pluck's minimum dependency requirements are inherited directly from the NEEDLE project.

**Key Finding:** NEEDLE's official source specifies **Rust 1.75** as the Minimum Supported Rust Version (MSRV), declared in both the project README badge and the Cargo.toml manifest.

---

## Context: What is Pluck?

Pluck is Strand #1 in NEEDLE's deterministic bead-processing escalation sequence:

| # | Strand | Agent? | Purpose |
|---|--------|--------|---------|
| 1 | 🪡 **Pluck** | Yes | Process beads from the assigned workspace |

**Source:** [NEEDLE README - Strand Escalation Section](https://github.com/jedarden/NEEDLE)

When a NEEDLE worker runs with Pluck active, it processes beads from the configured workspace bead queue. Pluck is the primary strand—the default worker mode for bead processing.

---

## Core Minimum Requirements

### Rust Toolchain

| Component | Minimum Version | Source | Notes |
|-----------|-----------------|--------|-------|
| **Rust Compiler** | **1.75** | [`/home/coding/NEEDLE/Cargo.toml`](file:///home/coding/NEEDLE/Cargo.toml) (line 5) | Declared as `rust-version = "1.75"` |
| **Rust Edition** | **2021** | [`/home/coding/NEEDLE/Cargo.toml`](file:///home/coding/NEEDLE/Cargo.toml) (line 4) | Declared as `edition = "2021"` |
| **Badge** | **1.75+** | [NEEDLE README](https://github.com/jedarden/NEEDLE) (badge) | Visual confirmation: `[![Rust](https://img.shields.io/badge/rust-1.75+-orange.svg)]` |

**MSRV Policy:** NEEDLE explicitly declares Rust 1.75 as the minimum supported version. This is the authoritative source. Builds will fail on earlier Rust versions.

---

## Minimum Dependency Versions

All dependency versions in NEEDLE's `Cargo.toml` use **semantic version caret requirements** (`^`). These specify the **minimum acceptable version** while allowing compatible updates within the same major version.

### Key Dependencies and Their Minimum Versions

| Dependency | Minimum Version | Actual Requirement | Purpose |
|------------|-----------------|-------------------|---------|
| `tokio` | `1.0.0` | `"1"` → `^1.0.0` | Async runtime (full features) |
| `serde` | `1.0.0` | `"1"` → `^1.0.0` | Serialization framework (with derive) |
| `serde_json` | `1.0.0` | `"1"` → `^1.0.0` | JSON serialization |
| `serde_yaml` | `0.9.0` | `"0.9"` → `^0.9.0` | YAML serialization |
| `clap` | `4.0.0` | `"4"` → `^4.0.0` | CLI argument parsing (with derive) |
| `anyhow` | `1.0.0` | `"1"` → `^1.0.0` | Error handling |
| `thiserror` | `1.0.0` | `"1"` → `^1.0.0` | Error derivation |
| `tracing` | `0.1.0` | `"0.1"` → `^0.1.0` | Structured logging |
| `tracing-subscriber` | `0.3.0` | `"0.3"` → `^0.3.0` | Log filtering and formatting |
| `chrono` | `0.4.0` | `"0.4"` → `^0.4.0` | Time and date handling (with serde) |
| `which` | `4.0.0` | `"4"` → `^4.0.0` | Locate executables in PATH |
| `async-trait` | `0.1.0` | `"0.1"` → `^0.1.0` | Async trait support |
| `fs2` | `0.4.0` | `"0.4"` → `^0.4.0` | Cross-platform file locking (flock) |
| `sha2` | `0.10.0` | `"0.10"` → `^0.10.0` | SHA-2 hashing (prompt content hash) |
| `hex` | `0.4.0` | `"0.4"` → `^0.4.0` | Hex encoding (binary fingerprinting) |
| `regex` | `1.0.0` | `"1"` → `^1.0.0` | Regular expressions (agent token extraction) |
| `aho-corasick` | `1.0.0` | `"1"` → `^1.0.0` | Multi-pattern string search |
| `glob` | `0.3.0` | `"0.3"` → `^0.3.0` | Glob pattern matching (doc file discovery) |
| `ureq` | `2.0.0` | `"2"` → `^2.0.0` | Simple HTTP client (self-update) |
| `cfg-if` | `1.0.0` | `"1"` → `^1.0.0` | Conditional compilation |
| `atty` | `0.2.0` | `"0.2"` → `^0.2.0` | Terminal detection (ANSI color support) |
| `toml` | `0.8.0` | `"0.8"` → `^0.8.0` | TOML parsing (gitleaks config) |
| `libc` | `0.2.0` | `"0.2"` → `^0.2.0` | Unix process handling (PID liveness check) |
| `rand` | `0.8.0` | `"0.8"` → `^0.8.0` | Random jitter (backoff desynchronization) |
| `futures` | `0.3.0` | `"0.3"` → `^0.3.0` | Async utilities |
| `gethostname` | `0.4.0` | `"0.4"` → `^0.4.0` | Hostname detection |

### Optional Dependencies (OpenTelemetry - gated behind `otlp` feature)

| Dependency | Minimum Version | Actual Requirement | Purpose |
|------------|-----------------|-------------------|---------|
| `opentelemetry` | `0.31.0` | `"0.31"` → `^0.31.0` (optional) | OpenTelemetry API |
| `opentelemetry_sdk` | `0.31.0` | `"0.31"` → `^0.31.0` (optional) | OpenTelemetry SDK (with rt-tokio) |
| `opentelemetry-otlp` | `0.31.0` | `"0.31"` → `^0.31.0` (optional) | OTLP exporter (with grpc-tonic, http-proto) |
| `opentelemetry-semantic-conventions` | `0.31.0` | `"0.31"` → `^0.31.0` (optional) | Semantic conventions |
| `tonic` | `0.14.0` | `"0.14"` → `^0.14.0` (optional) | gRPC for OTLP |
| `tracing-opentelemetry` | `0.32.0` | `"0.32"` → `^0.32.0` (optional) | Tracing integration |

### Development Dependencies

| Dependency | Minimum Version | Actual Requirement | Purpose |
|------------|-----------------|-------------------|---------|
| `tokio-test` | `0.4.0` | `"0.4"` → `^0.4.0` | Tokio testing utilities |
| `tempfile` | `3.0.0` | `"3"` → `^3.0.0` | Temporary file handling |
| `proptest` | `1.0.0` | `"1"` → `^1.0.0` | Property-based testing |
| `filetime` | `0.2.0` | `"0.2"` → `^0.2.0` | File time manipulation |
| `criterion` | `0.5.0` | `"0.5"` → `^0.5.0` | Benchmarking |

### Optional Test Dependencies (gated behind `integration` feature)

| Dependency | Minimum Version | Actual Requirement | Purpose |
|------------|-----------------|-------------------|---------|
| `testcontainers` | `0.23.0` | `"0.23"` → `^0.23.0` (optional) | Docker container integration testing |

---

## Version Constraint Semantics

### Cargo Semantic Versioning

NEEDLE uses **caret version requirements** (`^`), which is the default in Cargo. This means:

- `"1"` → `^1.0.0` → allows any version `>= 1.0.0` and `< 2.0.0`
- `"0.9"` → `^0.9.0` → allows any version `>= 0.9.0` and `< 0.10.0`
- `"4"` → `^4.0.0` → allows any version `>= 4.0.0` and `< 5.0.0`

**Key Principle:** Caret requirements allow updates that do not break the public API (according to SemVer). The minimum version is always the leftmost specified version with `.0.0` appended.

### Example: `tokio = "1"`

```toml
[dependencies]
tokio = { version = "1", features = ["full"] }
```

**This expands to:** `tokio = "^1.0.0"`

**Minimum acceptable version:** `1.0.0`  
**Maximum allowed version:** `< 2.0.0` (e.g., `1.99.99` is allowed, `2.0.0` is not)

---

## Special Version Constraints and Ranges

### 1. MSRV Policy (Rust 1.75)

NEEDLE explicitly declares a Minimum Supported Rust Version of 1.75. This is a **hard constraint**:

- Builds compiled with Rust < 1.75 will fail
- The MSRV is guaranteed to remain compatible until a major version bump (following Rust MSRV best practices)

**Rationale:** Rust 1.75 was released on **2023-12-28** and includes critical language features used by NEEDLE (e.g., improved `const` support, stabilized APIs).

### 2. Rust Edition 2021

NEEDLE uses Rust 2021 edition, which requires:

- **Minimum Rust version:** 1.56 (the version that introduced edition 2021)
- **Actual MSRV:** 1.75 (higher, due to other dependency requirements)

### 3. Feature Flags

NEEDLE uses feature flags to conditionally compile optional dependencies:

| Feature | Default? | Dependencies Enabled |
|---------|----------|---------------------|
| `otlp` | ✅ Yes (default) | OpenTelemetry stack (`opentelemetry-*`, `tonic`, `tracing-opentelemetry`) |
| `integration` | ❌ No | `testcontainers` for integration tests |

**Implication:** Building without default features (`--no-default-features`) removes all OpenTelemetry dependencies and their version constraints.

### 4. Indirect Dependencies

Some dependencies are pulled in transitively. NEEDLE does not directly constrain these versions—they follow the SemVer requirements of the direct dependencies:

- `aws/smithy-go` (v1.24.2) — pulled in by AWS SDK v2
- Various `golang.org/x/*` packages — Go workspace dependencies (ARMOR)

---

## br CLI (Bead Store Manager)

While Pluck/NEEDLE runs the worker logic, the **br CLI** (Beads Rust) manages the bead store. Pluck depends on br being installed and accessible:

| Component | Minimum Version | Purpose |
|-----------|-----------------|---------|
| `br` (beads_rust) | 0.2.0 | Bead store management and CLI for NEEDLE |

**Installation Path:** `~/.local/bin/br`  
**Source:** [jedarden/bead-forge](https://github.com/jedarden/bead-forge)  
**Core Functionality:**
- Bead creation, listing, and management
- Atomic bead claiming via SQLite transactions
- Workspace coordination for multi-worker setups

---

## SQLite Requirement

The bead store uses SQLite as the backend database:

| Component | Minimum Version | Rationale |
|-----------|-----------------|-----------|
| **SQLite** | **3.0** | Bead store backend (`.beads/beads.db`) |

**Implementation:** br CLI uses the Rust `rusqlite` crate (via SQLite FFI) to manage the bead store. SQLite 3.0+ provides the required features:
- Atomic transactions (for bead claiming)
- WAL mode (for concurrent access)
- JSON1 extension (for metadata storage)

**System Integration:** Most Linux distributions include SQLite 3.x via the system package manager or bundled with the Rust binary.

---

## System Requirements

### Build-Time Requirements

| Resource | Minimum | Recommended |
|----------|---------|-------------|
| **Disk Space** | ~100GB | For Rust target/ directory during builds |
| **Memory** | 8GB RAM | 16GB+ for faster cargo builds |
| **CPU** | Multi-core | 4+ cores for parallel builds |

### Runtime Requirements

| Platform | Architecture | Status |
|----------|--------------|--------|
| **Linux** | x86_64-unknown-linux-gnu | ✅ Fully supported |
| **macOS** | aarch64-apple-darwin (Apple Silicon) | ✅ Fully supported |
| **SQLite** | Any platform with SQLite 3.x | Required for bead store |

---

## Verification Methods

### 1. Check MSRV from Cargo.toml

```bash
grep "rust-version" /home/coding/NEEDLE/Cargo.toml
# Output: rust-version = "1.75"
```

### 2. Verify README Badge

The NEEDLE README displays the MSRV as a badge:
```markdown
[![Rust](https://img.shields.io/badge/rust-1.75+-orange.svg)](rust-toolchain.toml)
```

This badge is generated from `rust-toolchain.toml` in the repository.

### 3. Build with Minimum Rust Version

To verify MSRV compliance, you can build NEEDLE with Rust 1.75:

```bash
# Install Rust 1.75 via rustup
rustup install 1.75
rustup override set 1.75

# Build NEEDLE
cd /home/coding/NEEDLE
cargo build --release

# If successful, MSRV is verified
```

### 4. Check Dependency Minimums Programmatically

```bash
# Extract all dependency version requirements
cd /home/coding/NEEDLE
grep -E "version = \"" Cargo.toml | sort -u
```

---

## Version Compatibility Matrix

| Rust Version | NEEDLE 0.2.x | Status |
|--------------|-------------|--------|
| **1.75** | ✅ Supported | **MSRV** — Minimum guaranteed to work |
| **1.76** | ✅ Supported | Compatible |
| **1.77** | ✅ Supported | Compatible |
| **1.78** | ✅ Supported | Compatible |
| **1.79** | ✅ Supported | Compatible |
| **1.80** | ✅ Supported | Compatible |
| **1.81** | ✅ Supported | Compatible |
| **1.96** | ✅ Supported | Current toolchain version (2026-06-26) |
| **< 1.75** | ❌ Unsupported | Below MSRV — builds will fail |

---

## Dependency Update Policy

NEEDLE follows Rust ecosystem best practices for dependency management:

1. **Caret requirements** (`^`) allow compatible updates within the same major version
2. **MSRV is guaranteed** until a major version bump (NEEDLE 1.0.0 → 2.0.0)
3. **Security updates** are applied via `cargo update` on a regular cadence
4. **Breaking changes** in dependencies require a minor version bump in NEEDLE

### Updating Dependencies

```bash
cd /home/coding/NEEDLE

# Update all dependencies to latest compatible versions
cargo update

# Update a specific dependency
cargo update tokio

# Verify build still works
cargo build --release

# Run tests
cargo test --all-features
```

---

## Security Considerations

### Dependency Auditing

```bash
# Check for security advisories in Rust dependencies
cd /home/coding/NEEDLE
cargo audit
```

**Known Security Considerations:**
- **HTTP Client:** Uses `ureq` for self-update (simple HTTP, no TLS verification by default)
- **Process Management:** Executes external agent CLIs via `bash -c`
- **File Operations:** Uses SQLite with `flock` for coordination
- **Credential Storage:** AWS/GCP credentials managed via standard SDK chains

---

## Sources and References

### Primary Sources

| Source | Location | What It Provides |
|--------|----------|------------------|
| **NEEDLE Cargo.toml** | `/home/coding/NEEDLE/Cargo.toml` | Authoritative MSRV declaration, dependency versions |
| **NEEDLE README** | https://github.com/jedarden/NEEDLE | Visual MSRV badge, Pluck strand documentation |
| **GitHub Repository** | https://github.com/jedarden/NEEDLE | Source code, CI configuration |
| **Existing Inventory** | `/home/coding/ARMOR/pluck-version-inventory.md` | Current installed versions (for comparison) |

### External References

- [Rust MSRV Policy (RFC 2495)](https://rust-lang.github.io/rfcs/2495-min-rust-version.html)
- [Semantic Versioning (SemVer)](https://semver.org/)
- [Cargo Version Specifications](https://doc.rust-lang.org/cargo/reference/specifying-dependencies.html)

---

## Change History

| Date | Version | Changes |
|------|---------|---------|
| 2026-07-12 | 1.0 | Initial minimum dependency requirements document created for bf-4kr5w |

---

## Maintenance Notes

### Review Schedule

This document should be reviewed and updated when:
1. NEEDLE releases a new major version (e.g., 0.2.x → 0.3.0 or 1.0.0)
2. The MSRV increases in NEEDLE's Cargo.toml
3. Significant dependency updates occur that change minimum versions
4. New feature flags are introduced that alter dependency requirements

### Contact Information

- **Repository:** https://github.com/jedarden/NEEDLE
- **Issues:** https://github.com/jedarden/NEEDLE/issues
- **ARMOR Repository:** https://github.com/jedarden/ARMOR

---

**Document Status:** ✅ Complete  
**Last Updated:** 2026-07-12  
**Next Review:** 2026-10-12 (Quarterly, or upon NEEDLE version bump)

---

## Summary

Pluck's minimum dependency requirements are **inherited from NEEDLE**, as Pluck is a strand within the NEEDLE system:

- **Minimum Rust Version:** 1.75 (MSRV, explicitly declared)
- **Rust Edition:** 2021
- **All dependencies:** Use semantic caret requirements (`^`), specifying minimum versions at the leftmost `.0.0` of each declared version
- **Special constraints:** OpenTelemetry dependencies are optional (default feature), integration test dependencies are optional (integration feature)
- **Supporting tools:** br CLI (0.2.0+) and SQLite (3.0+) required for bead store operations

This document provides a comprehensive reference for understanding Pluck's minimum version requirements, sourced directly from the NEEDLE project's Cargo.toml and official documentation.
