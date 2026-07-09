# Pluck Dependencies Documentation

**Bead:** bf-29m3g  
**Component:** Pluck Strand (part of NEEDLE)  
**Last Updated:** 2026-07-09

## Overview

Pluck is a strand component within the NEEDLE project. Pluck handles >90% of all bead processing by querying the bead store for unassigned, ready beads, filtering by excluded labels, and sorting them in deterministic priority order.

**Source Location:** `/home/coding/NEEDLE/src/strand/pluck.rs`  
**Project Repository:** https://github.com/jedarden/NEEDLE  
**Documentation Path:** `docs/bf-29m3g-pluck-dependencies.md`

---

## Core System Requirements

### Rust Toolchain
- **Minimum Version:** Rust 1.75+ (specified in Cargo.toml)
- **Edition:** Rust 2021
- **Stable Channel:** Recommended (uses stable toolchain)
- **Components Required:**
  - `rustfmt` - Code formatting
  - `clippy` - Linting
  - `cargo` - Build tool

**Toolchain Configuration:** `/home/coding/NEEDLE/rust-toolchain.toml`
```toml
[toolchain]
channel = "stable"
components = ["rustfmt", "clippy"]
targets = ["x86_64-unknown-linux-gnu", "aarch64-apple-darwin"]
```

### Supported Target Platforms
- **Linux x86_64:** `x86_64-unknown-linux-gnu` (primary)
- **Linux x86_64 static:** `x86_64-unknown-linux-musl` (for release builds)
- **macOS ARM64:** `aarch64-apple-darwin`

---

## Rust Dependencies (from Cargo.toml)

### Core Runtime Dependencies

| Dependency | Version | Purpose | Required |
|------------|---------|---------|----------|
| `tokio` | 1.x | Async runtime (full features) | ✅ Yes |
| `serde` | 1.x | Serialization (with derive) | ✅ Yes |
| `serde_json` | 1.x | JSON serialization | ✅ Yes |
| `serde_yaml` | 0.9.x | YAML serialization | ✅ Yes |
| `async-trait` | 0.1.x | Async trait support | ✅ Yes |
| `tracing` | 0.1.x | Logging/telemetry framework | ✅ Yes |
| `tracing-subscriber` | 0.3.x | Log subscriber (env-filter, json) | ✅ Yes |

### CLI Dependencies

| Dependency | Version | Purpose | Required |
|------------|---------|---------|----------|
| `clap` | 4.x | CLI argument parsing (derive) | ✅ Yes |

### Error Handling

| Dependency | Version | Purpose | Required |
|------------|---------|---------|----------|
| `anyhow` | 1.x | Generic error handling | ✅ Yes |
| `thiserror` | 1.x | Error derive macros | ✅ Yes |

### Data Processing & Utilities

| Dependency | Version | Purpose | Required |
|------------|---------|---------|----------|
| `chrono` | 0.4.x | Time handling (serde features) | ✅ Yes |
| `regex` | 1.x | Regular expressions | ✅ Yes |
| `glob` | 0.3.x | Pattern matching | ✅ Yes |
| `aho-corasick` | 1.x | Multi-pattern string search | ✅ Yes |
| `sha2` | 0.10.x | Hashing (content fingerprinting) | ✅ Yes |
| `hex` | 0.4.x | Hex encoding | ✅ Yes |

### System Integration

| Dependency | Version | Purpose | Required |
|------------|---------|---------|----------|
| `fs2` | 0.4.x | Cross-platform file locking (flock) | ✅ Yes |
| `which` | 4.x | Process management | ✅ Yes |
| `libc` | 0.2.x | Unix process handling (PID checks) | ✅ Yes |
| `atty` | 0.2.x | Terminal detection (ANSI colors) | ✅ Yes |
| `toml` | 0.8.x | TOML parsing | ✅ Yes |
| `cfg-if` | 1.x | Conditional compilation | ✅ Yes |
| `rand` | 0.8.x | Random jitter (backoff desync) | ✅ Yes |
| `futures` | 0.3.x | Async utilities | ✅ Yes |
| `gethostname` | 0.4.x | Hostname retrieval | ✅ Yes |

### Network & HTTP

| Dependency | Version | Purpose | Required |
|------------|---------|---------|----------|
| `ureq` | 2.x | HTTP client (self-update) | ✅ Yes |

### Optional Dependencies (Feature-Gated)

#### OpenTelemetry/OTLP Feature
| Dependency | Version | Purpose | Required |
|------------|---------|---------|----------|
| `opentelemetry` | 0.31.x | OTel SDK | ⚠️ Optional |
| `opentelemetry_sdk` | 0.31.x | OTel SDK (rt-tokio) | ⚠️ Optional |
| `opentelemetry-otlp` | 0.31.x | OTLP exporter (grpc-tonic, http-proto) | ⚠️ Optional |
| `opentelemetry-semantic-conventions` | 0.31.x | Semantic conventions | ⚠️ Optional |
| `tonic` | 0.14.x | gRPC for OTLP | ⚠️ Optional |
| `tracing-opentelemetry` | 0.32.x | Tracing bridge | ⚠️ Optional |

#### Integration Test Feature
| Dependency | Version | Purpose | Required |
|------------|---------|---------|----------|
| `testcontainers` | 0.23.x | Containerized integration tests | ⚠️ Optional |

### Development Dependencies (Build-time Only)

| Dependency | Version | Purpose | Required |
|------------|---------|---------|----------|
| `tokio-test` | 0.4.x | Async testing utilities | 🔧 Dev only |
| `tempfile` | 3.x | Temporary file testing | 🔧 Dev only |
| `proptest` | 1.x | Property-based testing | 🔧 Dev only |
| `filetime` | 0.2.x | File time testing | 🔧 Dev only |
| `criterion` | 0.5.x | Benchmarking | 🔧 Dev only |

---

## System-Level Dependencies

### Build Dependencies

| Tool | Version | Purpose | Required |
|------|---------|---------|----------|
| `cargo` | Latest | Rust build tool | ✅ Yes |
| `rustc` | 1.75+ | Rust compiler | ✅ Yes |
| `musl-tools` | Any | Static linking for Linux (release builds only) | ⚠️ Release only |

### Runtime Dependencies (No external system deps required)
Pluck/NEEDLE is **statically linked** for release builds and has **no runtime system dependencies**.

---

## Installation Methods

### Method 1: Pre-built Binary (Recommended)
```bash
curl -fsSL https://github.com/jedarden/NEEDLE/releases/latest/download/install.sh | bash
```

### Method 2: Cargo Install
```bash
cargo install --git https://github.com/jedarden/NEEDLE
```

### Method 3: Build from Source
```bash
git clone https://github.com/jedarden/NEEDLE.git
cd NEEDLE
cargo build --release
```

---

## Configuration Requirements

### Environment Variables (Optional)
- `RUST_LOG` - For debug logging (e.g., `RUST_LOG=needle::strand::pluck=debug`)
- `CARGO_TERM_COLOR` - Terminal color control
- `RUST_BACKTRACE` - Enable backtraces for debugging

### Build Features
- **Default:** `otlp` (OpenTelemetry enabled)
- **Minimal:** `--no-default-features` (OpenTelemetry disabled)
- **Integration:** `--features integration` (includes testcontainers)

---

## Minimum Version Requirements Summary

| Component | Minimum Version | Notes |
|-----------|-----------------|-------|
| **Rust** | 1.75 | Specified in Cargo.toml |
| **Cargo** | Works with 1.75+ | Comes with Rust |
| **tokio** | 1.x | Async runtime |
| **serde** | 1.x | Serialization |
| **chrono** | 0.4.x | Time handling |
| **tracing** | 0.1.x | Logging |

---

## Dependency Tree (Key Relationships)

```
Pluck (src/strand/pluck.rs)
├── needle::bead_store (BeadStore trait)
├── needle::types (Bead, StrandError, StrandResult)
└── External Dependencies
    ├── async-trait (for BeadStore async trait)
    ├── tracing (for debug logging)
    ├── chrono (for bead created_at timestamps)
    └── anyhow/thiserror (for error handling)
```

---

## Verification Checklist

- [x] Identify Pluck's source code and build configuration
- [x] Extract dependency requirements from Cargo.toml
- [x] Document required system libraries and development tools
- [x] Note minimum version requirements (Rust 1.75+)
- [x] Create comprehensive dependency documentation
- [x] Document supported target platforms
- [x] Document optional (feature-gated) dependencies
- [x] Document installation methods

---

## Next Steps

1. **Install Rust toolchain** (if not already installed):
   ```bash
   curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
   ```

2. **Clone NEEDLE repository** (if building from source):
   ```bash
   git clone https://github.com/jedarden/NEEDLE.git
   cd NEEDLE
   ```

3. **Build Pluck/NEEDLE**:
   ```bash
   cargo build --release
   ```

4. **Verify installation**:
   ```bash
   needle --version
   ```

---

## References

- **NEEDLE Repository:** https://github.com/jedarden/NEEDLE
- **Pluck Source:** `/home/coding/NEEDLE/src/strand/pluck.rs`
- **Cargo.toml:** `/home/coding/NEEDLE/Cargo.toml`
- **Toolchain Config:** `/home/coding/NEEDLE/rust-toolchain.toml`
- **CI Configuration:** `/home/coding/NEEDLE/.github/workflows/ci.yml`
- **Release Configuration:** `/home/coding/NEEDLE/.github/workflows/release.yml`

---

**Status:** ✅ **Documentation Complete**

All required dependencies for Pluck strand have been documented, including:
- Rust toolchain requirements (1.75+)
- All Cargo dependencies with versions
- System-level build dependencies
- Optional feature-gated dependencies
- Installation methods and verification
- Target platforms supported
