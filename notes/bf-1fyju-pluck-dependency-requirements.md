# Pluck Dependency Requirements

**Source:** `/home/coding/NEEDLE/` (NEEDLE project - Pluck is a strand within NEEDLE)  
**Documentation Date:** 2026-07-09  
**NEEDLE Version:** 0.2.11

## Overview

Pluck is a strand within the NEEDLE system. As such, its dependency requirements are the same as the NEEDLE project itself. NEEDLE is a Rust-based universal wrapper for headless coding CLI agents.

## Core Requirements

### Rust Toolchain

**Minimum Rust Version:** 1.75 or later  
**Edition:** 2021  
**Toolchain Channel:** Stable

**Rust Components Required:**
- `rustfmt` - Code formatting
- `clippy` - Linting

**Build Targets:**
- `x86_64-unknown-linux-gnu` (primary Linux target)
- `aarch64-apple-darwin` (macOS ARM64 target)

**Source:** `rust-toolchain.toml` and `Cargo.toml` (`rust-version = "1.75"`)

## Runtime Dependencies

### Core Runtime (All Required)

| Dependency | Minimum Version | Purpose | Features |
|------------|----------------|---------|----------|
| `tokio` | 1.x | Async runtime | full |
| `serde` | 1.x | Serialization | derive |
| `serde_json` | 1.x | JSON serialization | - |
| `serde_yaml` | 0.9.x | YAML serialization | - |
| `clap` | 4.x | CLI parsing | derive |
| `anyhow` | 1.x | Error handling | - |
| `thiserror` | 1.x | Error deriving | - |
| `tracing` | 0.1.x | Logging framework | - |
| `tracing-subscriber` | 0.3.x | Logging implementation | env-filter, json |
| `chrono` | 0.4.x | Time handling | serde |
| `which` | 4.x | Process management | - |
| `async-trait` | 0.1.x | Async traits | - |
| `fs2` | 0.4.x | File locking | - |
| `sha2` | 0.10.x | Hashing | - |
| `hex` | 0.4.x | Hex encoding | - |
| `regex` | 1.x | Regular expressions | - |
| `glob` | 0.3.x | Pattern matching | - |
| `ureq` | 2.x | HTTP client | - |
| `aho-corasick` | 1.x | Multi-pattern search | - |
| `cfg-if` | 1.x | Conditional compilation | - |
| `atty` | 0.2.x | Terminal detection | - |
| `toml` | 0.8.x | TOML parsing | - |
| `libc` | 0.2.x | Unix process handling | - |
| `rand` | 0.8.x | Random generation | - |
| `futures` | 0.3.x | Async utilities | - |
| `gethostname` | 0.4.x | Hostname detection | - |

### Optional Dependencies (OTLP Feature)

| Dependency | Minimum Version | Purpose | Required |
|------------|----------------|---------|----------|
| `opentelemetry` | 0.31.x | OpenTelemetry API | No |
| `opentelemetry_sdk` | 0.31.x | OpenTelemetry SDK | No |
| `opentelemetry-otlp` | 0.31.x | OTLP exporter | No |
| `opentelemetry-semantic-conventions` | 0.31.x | Semantic conventions | No |
| `tonic` | 0.14.x | gRPC for OTLP | No |
| `tracing-opentelemetry` | 0.32.x | Tracing bridge | No |

**Note:** OTLP dependencies are optional and gated behind the `otlp` feature flag.

### Development Dependencies

| Dependency | Minimum Version | Purpose |
|------------|----------------|---------|
| `tokio-test` | 0.4.x | Async testing utilities |
| `tempfile` | 3.x | Temporary file handling |
| `proptest` | 1.x | Property-based testing |
| `filetime` | 0.2.x | File time testing |
| `criterion` | 0.5.x | Benchmarking |

## Development Tools Required

### Essential Tools

1. **Cargo** - Rust package manager (included with Rust installation)
2. **rustfmt** - Code formatter (Rust component)
3. **clippy** - Linter (Rust component)

### Build System

- **Cargo build**: `cargo build --release`
- **Cargo test**: `cargo test`
- **Cargo clippy**: `cargo clippy --all-targets -- -D warnings`
- **Cargo fmt**: `cargo fmt --check`

### Optional: Claude-Interactive Plugin

If using the `claude-interactive` plugin for subscription billing:

**Requirements:**
- Python 3.10 or later
- `pyte` Python package (`pip install pyte`)
- `claude` CLI on PATH

## Installation Methods

### Method 1: Pre-built Binary (Recommended)

```bash
curl -fsSL https://github.com/jedarden/NEEDLE/releases/latest/download/install.sh | bash
```

### Method 2: Build from Source

```bash
cargo install --git https://github.com/jedarden/NEEDLE
```

**Requirements for Source Build:**
- Rust 1.75+ stable toolchain
- Cargo
- Git

## System Requirements

### Linux
- x86_64 architecture (primary target)
- Standard C library (glibc)
- POSIX-compliant system (for file locking, process handling)

### macOS
- ARM64 architecture (aarch64-apple-darwin target)
- Xcode command line tools (for system libraries)

## Feature Flags

- **default**: `otlp` - OTLP telemetry enabled by default
- **otlp**: Includes OpenTelemetry dependencies for telemetry export
- **integration**: `otlp` + `testcontainers` for integration testing

## Verification Commands

To verify installed versions meet requirements:

```bash
# Check Rust version
rustc --version  # Should be 1.75 or later

# Check Cargo version
cargo --version

# Verify components
rustfmt --version
clippy --version

# Test build
cargo build --release

# Run tests
cargo test
```

## References

- **Source Repository:** `/home/coding/NEEDLE/`
- **Main Config:** `Cargo.toml`
- **Toolchain Config:** `rust-toolchain.toml`
- **Documentation:** `README.md`
- **CI Configuration:** `.github/workflows/ci.yml`

## Notes

1. **Pluck is a strand:** Pluck is one of nine strands in NEEDLE (Pluck, Mend, Explore, Weave, Unravel, Pulse, Reflect, Splice, Knot)
2. **No separate installation:** There is no standalone "Pluck" package - it's part of NEEDLE
3. **Shared dependencies:** All strands use the same dependency tree defined in NEEDLE's `Cargo.toml`
4. **Feature-gated telemetry:** OTLP/telemetry features can be disabled to reduce dependency footprint
