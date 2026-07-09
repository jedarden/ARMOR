# Pluck (NEEDLE) Dependency Requirements

## Overview

**Pluck** is a strand within the NEEDLE system (Navigates Every Enqueued Deliverable, Logs Effort). NEEDLE is a Rust-based universal wrapper for headless coding CLI agents that processes bead queues in deterministic order.

- **Project**: NEEDLE (includes Pluck as strand #1)
- **Repository**: https://github.com/jedarden/NEEDLE
- **Current Version**: 0.2.11
- **Primary Language**: Rust
- **MSRV**: Rust 1.75+ (2023-12-28)

## Core Requirements

### Rust Toolchain

**Minimum Supported Rust Version (MSRV): 1.75**

```toml
# rust-toolchain.toml
[toolchain]
channel = "stable"
components = ["rustfmt", "clippy"]
targets = ["x86_64-unknown-linux-gnu", "aarch64-apple-darwin"]
```

**Required Components:**
- `rustc` 1.75+ (Rust compiler)
- `cargo` (comes with Rust)
- `rustfmt` (code formatter)
- `clippy` (linter)

**Installation Methods:**
1. **Via rustup** (recommended):
   ```bash
   curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | bash
   rustup install stable
   rustup component add rustfmt clippy
   ```

2. **Via system package manager** (may not be latest version)

3. **From source**: https://github.com/rust-lang/rust

### Cargo Dependencies (Runtime)

**Async Runtime:**
- `tokio` 1+ (features: "full")

**Serialization:**
- `serde` 1+ (features: "derive")
- `serde_json` 1+
- `serde_yaml` 0.9+

**CLI:**
- `clap` 4+ (features: "derive")

**Error Handling:**
- `anyhow` 1+
- `thiserror` 1+

**Logging/Telemetry:**
- `tracing` 0.1+
- `tracing-subscriber` 0.3+ (features: "env-filter", "json")

**Time Handling:**
- `chrono` 0.4+ (features: "serde")

**Process Management:**
- `which` 4+

**Async Traits:**
- `async-trait` 0.1+

**File Operations:**
- `fs2` 0.4+ (cross-platform file locking)

**Hashing:**
- `sha2` 0.10+ (prompt content hash, binary fingerprinting)
- `hex` 0.4+

**Pattern Matching:**
- `regex` 1+ (agent token extraction)
- `glob` 0.3+ (doc file discovery)

**HTTP Client:**
- `ureq` 2+ (self-update)

**String Search:**
- `aho-corasick` 1+ (sanitizer keyword pre-filter)

**Compilation:**
- `cfg-if` 1+

**Terminal Detection:**
- `atty` 0.2+ (ANSI color support)

**Configuration Parsing:**
- `toml` 0.8+ (gitleaks config)

**Unix Process Handling:**
- `libc` 0.2+ (PID liveness check)

**Random Number Generation:**
- `rand` 0.8+ (backoff desynchronization)

**OpenTelemetry/OTLP (optional, gated behind `otlp` feature):**
- `opentelemetry` 0.31+
- `opentelemetry_sdk` 0.31+ (features: "rt-tokio")
- `opentelemetry-otlp` 0.31+ (features: "grpc-tonic", "http-proto")
- `opentelemetry-semantic-conventions` 0.31+
- `tonic` 0.14+
- `tracing-opentelemetry` 0.32+
- `futures` 0.3+
- `gethostname` 0.4+

**Testing/Development:**
- `tokio-test` 0.4+
- `tempfile` 3+
- `proptest` 1+
- `filetime` 0.2+
- `criterion` 0.5+ (benchmarks)
- `testcontainers` 0.23+ (optional, integration tests)

## Installation Requirements

### Required Tools for Installation

**Download Tools:**
- `curl` OR `wget` (for downloading releases)
  - Check: `curl --version` or `wget --version`

**Checksum Verification:**
- `sha256sum` OR `shasum` (for integrity verification)
  - Linux: `sha256sum --version`
  - macOS: `shasum --version`

**Optional Verification:**
- `gpg` (for GPG signature verification of releases)
  - Check: `gpg --version`

**Installation Path:**
- Target: `~/.local/bin/needle` (configurable via `$NEEDLE_INSTALL_PATH`)
- Requires `~/.local/bin` to be in `PATH`

## Development Tools Requirements

### Building from Source

**Required:**
- Rust 1.75+ (see above)
- Cargo (comes with Rust)
- Git (for cloning repository)

**Build Commands:**
```bash
# Standard release build
cargo build --release

# Development build
cargo build

# With OTLP features
cargo build --release --features otlp

# Run tests (CI only, not recommended locally)
cargo test
```

### Code Quality Tools

**Required before committing:**
- `cargo fmt` (code formatting)
- `cargo clippy --all-targets -- -D warnings` (linting)

**Development Guidelines:**
- No `unwrap()` or `expect()` in non-test code
- All public functions must return `Result<T>`
- Telemetry emission at every state transition
- Exhaustive match arms (no catch-all `_` on outcome enums)

## Optional Plugin Dependencies

### claude-interactive Plugin

**Purpose:** Run Claude Code in interactive mode using subscription billing instead of programmatic API credits.

**Requirements:**
- **Python 3.10+**
  - Check: `python3 --version`
- **pyte** (Python library for PTY handling)
  - Install: `pip install pyte`
- **claude CLI** (Anthropic's Claude Code CLI)
  - Must be available in `PATH`
  - Check: `claude --version`

**Installation:**
```bash
# Download claude-interactive from NEEDLE releases
gh release download --repo jedarden/NEEDLE --pattern 'claude-interactive*'
chmod +x claude-interactive-install.sh
./claude-interactive-install.sh
```

**Usage:**
```bash
cd /path/to/workspace
needle run --agent claude-interactive --count 4
```

## Platform Support

**Supported Operating Systems:**
- Linux (x86_64, aarch64)
- macOS (x86_64, aarch64)

**Target Triples:**
- `x86_64-unknown-linux-gnu`
- `aarch64-apple-darwin`

**Release Assets:**
- `needle-x86_64-unknown-linux-gnu`
- `needle-aarch64-unknown-linux-gnu`
- `needle-x86_64-apple-darwin`
- `needle-aarch64-apple-darwin`

## Quick Verification Commands

To verify your environment meets the requirements:

```bash
# Rust toolchain
rustc --version      # Should be 1.75+
cargo --version
rustfmt --version
clippy --version    # via: cargo clippy --version

# Build tools
git --version
make --version       # if using makefiles

# Installation tools
curl --version       # OR wget --version
sha256sum --version  # OR shasum --version
gpg --version        # optional

# Python (for claude-interactive plugin)
python3 --version    # Should be 3.10+
pip3 --version

# Claude CLI (for claude-interactive plugin)
claude --version

# Test NEEDLE installation
needle --version
```

## Source References

- **Cargo.toml**: `/home/coding/NEEDLE/Cargo.toml` - Full dependency manifest
- **rust-toolchain.toml**: `/home/coding/NEEDLE/rust-toolchain.toml` - Rust version requirements
- **install.sh**: `/home/coding/NEEDLE/install.sh` - Installation script and requirements
- **CLAUDE.md**: `/home/coding/NEEDLE/CLAUDE.md` - MSRV and development conventions
- **README.md**: `/home/coding/NEEDLE/README.md` - Project overview and quickstart

## Related Projects

- **claude-governor**: API spend management and quota enforcement
- **ccdash**: TUI for monitoring Claude Code sessions
- **CLASP**: Proxy for multiple LLM backends

## Version History

- **0.2.11**: Current version
- **0.2.8**: Previous stable release
- MSRV established at Rust 1.75 (2023-12-28)

---

**Document Generated**: 2026-07-09  
**NEEDLE Version**: 0.2.11  
**Purpose**: Document all minimum version requirements for Pluck/NEEDLE dependencies and development tools