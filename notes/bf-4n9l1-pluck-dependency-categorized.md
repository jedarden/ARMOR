# Pluck Dependencies - Categorized List for Verification

**Bead:** bf-4n9l1  
**Source Documentation:** bf-29m3g  
**Component:** Pluck Strand (part of NEEDLE)  
**Last Updated:** 2026-07-09

---

## Summary

This document provides a categorized list of all Pluck dependencies extracted from the comprehensive documentation in bead bf-29m3g. Dependencies are organized by installation type to facilitate verification checks.

---

## Category 1: System Packages (Pre-installed Requirements)

### Required System Tools
| Tool | Minimum Version | Installation Method | Purpose |
|------|-----------------|---------------------|---------|
| `rustc` | 1.75+ | rustup | Rust compiler |
| `cargo` | Works with 1.75+ | rustup (comes with rustc) | Build tool |
| `rustfmt` | Latest | rustup component | Code formatting |
| `clippy` | Latest | rustup component | Linting |

### Optional System Tools (Release Builds Only)
| Tool | Version | Installation Method | Purpose |
|------|---------|---------------------|---------|
| `musl-tools` | Any | Package manager (apt/yum) | Static linking for Linux releases |

---

## Category 2: Rust/Cargo Dependencies (Managed via Cargo.toml)

### Core Runtime Dependencies (Required)
| Crate | Version | Purpose |
|-------|---------|---------|
| `tokio` | 1.x | Async runtime (full features) |
| `serde` | 1.x | Serialization (with derive) |
| `serde_json` | 1.x | JSON serialization |
| `serde_yaml` | 0.9.x | YAML serialization |
| `async-trait` | 0.1.x | Async trait support |
| `tracing` | 0.1.x | Logging/telemetry framework |
| `tracing-subscriber` | 0.3.x | Log subscriber (env-filter, json) |

### CLI Dependencies (Required)
| Crate | Version | Purpose |
|-------|---------|---------|
| `clap` | 4.x | CLI argument parsing (derive) |

### Error Handling (Required)
| Crate | Version | Purpose |
|-------|---------|---------|
| `anyhow` | 1.x | Generic error handling |
| `thiserror` | 1.x | Error derive macros |

### Data Processing & Utilities (Required)
| Crate | Version | Purpose |
|-------|---------|---------|
| `chrono` | 0.4.x | Time handling (serde features) |
| `regex` | 1.x | Regular expressions |
| `glob` | 0.3.x | Pattern matching |
| `aho-corasick` | 1.x | Multi-pattern string search |
| `sha2` | 0.10.x | Hashing (content fingerprinting) |
| `hex` | 0.4.x | Hex encoding |

### System Integration (Required)
| Crate | Version | Purpose |
|-------|---------|---------|
| `fs2` | 0.4.x | Cross-platform file locking (flock) |
| `which` | 4.x | Process management |
| `libc` | 0.2.x | Unix process handling (PID checks) |
| `atty` | 0.2.x | Terminal detection (ANSI colors) |
| `toml` | 0.8.x | TOML parsing |
| `cfg-if` | 1.x | Conditional compilation |
| `rand` | 0.8.x | Random jitter (backoff desync) |
| `futures` | 0.3.x | Async utilities |
| `gethostname` | 0.4.x | Hostname retrieval |

### Network & HTTP (Required)
| Crate | Version | Purpose |
|-------|---------|---------|
| `ureq` | 2.x | HTTP client (self-update) |

---

## Category 3: Optional Feature-Gated Dependencies

### OpenTelemetry/OTLP Feature (Optional)
| Crate | Version | Purpose |
|-------|---------|---------|
| `opentelemetry` | 0.31.x | OTel SDK |
| `opentelemetry_sdk` | 0.31.x | OTel SDK (rt-tokio) |
| `opentelemetry-otlp` | 0.31.x | OTLP exporter (grpc-tonic, http-proto) |
| `opentelemetry-semantic-conventions` | 0.31.x | Semantic conventions |
| `tonic` | 0.14.x | gRPC for OTLP |
| `tracing-opentelemetry` | 0.32.x | Tracing bridge |

### Integration Test Feature (Development Only)
| Crate | Version | Purpose |
|-------|---------|---------|
| `testcontainers` | 0.23.x | Containerized integration tests |

---

## Category 4: Development Dependencies (Build-time Only)

| Crate | Version | Purpose |
|-------|---------|---------|
| `tokio-test` | 0.4.x | Async testing utilities |
| `tempfile` | 3.x | Temporary file testing |
| `proptest` | 1.x | Property-based testing |
| `filetime` | 0.2.x | File time testing |
| `criterion` | 0.5.x | Benchmarking |

---

## Installation Type Summary

### Type A: Pre-installed System Packages
- **Tools:** rustc, cargo, rustfmt, clippy, musl-tools (optional)
- **Verification:** Check system PATH and version commands
- **Installation:** rustup for Rust toolchain, package manager for musl-tools

### Type B: Language Package Dependencies (Cargo)
- **Total Required Crates:** 22 core dependencies
- **Installation:** Automatically managed by `cargo build` or `cargo install`
- **Verification:** Check Cargo.lock file or `cargo tree` output

### Type C: Optional Feature Dependencies
- **Total Optional Crates:** 7 (OTEL) + 1 (testcontainers)
- **Installation:** Activated via Cargo features (`--features otlp`, `--features integration`)
- **Verification:** Check build configuration and feature flags

### Type D: Development Dependencies
- **Total Dev Crates:** 5
- **Installation:** Automatically included in dev builds by Cargo
- **Verification:** Check Cargo.toml `[dev-dependencies]` section

---

## Dependency Count Summary

| Category | Count | Required for Production |
|----------|-------|-------------------------|
| **System Packages (Required)** | 4 | Yes (rustc, cargo, rustfmt, clippy) |
| **System Packages (Optional)** | 1 | No (musl-tools for release builds only) |
| **Core Runtime (Cargo)** | 7 | Yes |
| **CLI (Cargo)** | 1 | Yes |
| **Error Handling (Cargo)** | 2 | Yes |
| **Data Processing (Cargo)** | 6 | Yes |
| **System Integration (Cargo)** | 9 | Yes |
| **Network (Cargo)** | 1 | Yes |
| **Total Required Cargo Dependencies** | **26** | **Yes** |
| **Optional OTEL Dependencies** | 6 | No (feature-gated) |
| **Optional Test Dependencies** | 1 | No (dev-only) |
| **Development Dependencies** | 5 | No (dev-only) |
| **Grand Total** | **52** | **26 required, 26 optional/dev** |

---

## Verification Checklist

### System Package Verification
- [ ] Rust 1.75+ installed (`rustc --version`)
- [ ] Cargo working (`cargo --version`)
- [ ] rustfmt available (`rustfmt --version`)
- [ ] clippy available (`clippy --version`)

### Cargo Dependency Verification
- [ ] Cargo.lock exists and is up-to-date
- [ ] All dependencies resolve without errors (`cargo check`)
- [ ] No security vulnerabilities (`cargo audit`)
- [ ] Build completes successfully (`cargo build --release`)

### Optional Dependencies Verification
- [ ] OTEL feature builds (if enabled): `cargo build --features otlp`
- [ ] Integration tests build (if needed): `cargo build --features integration`

### Development Dependencies Verification
- [ ] Development build works: `cargo build`
- [ ] Tests pass: `cargo test`
- [ ] Benchmarks run: `cargo bench` (if criterion available)

---

## Quick Reference Commands

### Check System Dependencies
```bash
# Rust toolchain
rustc --version    # Should be 1.75+
cargo --version    # Should work with 1.75+
rustfmt --version  # Should be available
clippy --version   # Should be available

# Optional musl-tools (for static linking)
musl-gcc --version  # Only needed for release builds
```

### Check Cargo Dependencies
```bash
# From NEEDLE repository
cd /home/coding/NEEDLE

# Verify all dependencies resolve
cargo check

# Show dependency tree
cargo tree

# Check for security issues (requires cargo-audit)
cargo audit

# Verify build succeeds
cargo build --release
```

### Check Optional Dependencies
```bash
# Test OTEL feature
cargo build --features otlp

# Test integration feature
cargo build --features integration

# Test both features together
cargo build --features "otlp,integration"
```

---

## References

- **Full Documentation:** `/home/coding/ARMOR/notes/bf-29m3g-pluck-dependencies.md`
- **Source Documentation Bead:** bf-29m3g
- **NEEDLE Repository:** https://github.com/jedarden/NEEDLE
- **Pluck Source:** `/home/coding/NEEDLE/src/strand/pluck.rs`
- **Cargo.toml:** `/home/coding/NEEDLE/Cargo.toml`

---

**Status:** ✅ **Dependency Categorization Complete**

All Pluck dependencies have been:
- ✅ Located from previous documentation (bf-29m3g)
- ✅ Extracted and parsed
- ✅ Categorized by installation type (system packages, Cargo dependencies, optional, dev-only)
- ✅ Organized for verification checks
- ✅ Summarized with counts and verification commands
