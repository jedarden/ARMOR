# Pluck Dependencies Documentation

## Overview

**Pluck** is a strand (command/module) within the NEEDLE project that handles primary bead selection from the assigned workspace. It processes over 90% of all bead operations by querying the bead store for unassigned, ready beads, filtering by excluded labels, and sorting them in deterministic priority order.

**Project:** NEEDLE (Navigates Every Enqueued Deliverable, Logs Effort)  
**Pluck Module:** `/home/coding/NEEDLE/src/strand/pluck.rs`  
**Language:** Rust  
**Current Version:** 0.2.11

---

## Required Dependencies

### 1. Rust Toolchain

**Minimum Rust Version:** 1.75 (specified in `rust-version` field)

**Required Components:**
- `rustc` (Rust compiler)
- `cargo` (Rust package manager)
- `rustfmt` (code formatter - optional but recommended)
- `clippy` (linter - optional but recommended)

**Installation:**
```bash
# Using rustup (recommended)
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh

# Or on Linux with apt
sudo apt install rustc cargo

# Verify installation
rustc --version
cargo --version
```

**Toolchain Configuration:**
- **Channel:** Stable
- **Targets:** 
  - `x86_64-unknown-linux-gnu` (Linux AMD64)
  - `aarch64-apple-darwin` (macOS ARM64)

---

### 2. Core Cargo Dependencies

Pluck relies on these Rust libraries defined in `/home/coding/NEEDLE/Cargo.toml`:

#### Async Runtime
- **tokio** (version 1, features: "full") - Async runtime
- **async-trait** (version 0.1) - Async trait support
- **futures** (version 0.3) - Futures utilities

#### Serialization & Data Handling
- **serde** (version 1, features: "derive") - Serialization framework
- **serde_json** (version 1) - JSON serialization
- **serde_yaml** (version 0.9) - YAML serialization
- **toml** (version 0.8) - TOML parsing

#### CLI & User Interface
- **clap** (version 4, features: "derive") - Command-line argument parsing
- **atty** (version 0.2) - Terminal detection for ANSI color support

#### Error Handling
- **anyhow** (version 1) - Error handling
- **thiserror** (version 1) - Error derive macros

#### Logging & Telemetry
- **tracing** (version 0.1) - Structured logging
- **tracing-subscriber** (version 0.3, features: "env-filter", "json") - Log formatting
- **chrono** (version 0.4, features: "serde") - Time handling

#### OpenTelemetry (Optional Feature)
- **opentelemetry** (version 0.31, optional)
- **opentelemetry_sdk** (version 0.31, features: "rt-tokio", optional)
- **opentelemetry-otlp** (version 0.31, features: "grpc-tonic", "http-proto", optional)
- **opentelemetry-semantic-conventions** (version 0.31, optional)
- **tonic** (version 0.14, optional)
- **tracing-opentelemetry** (version 0.32, optional)

#### File System & Process Management
- **which** (version 4) - Process/executable detection
- **fs2** (version 0.4) - Cross-platform file locking (flock)
- **libc** (version 0.2) - Unix process handling (PID liveness check)

#### Cryptography & Hashing
- **sha2** (version 0.10) - SHA-256 hashing for prompt content
- **hex** (version 0.4) - Hex encoding

#### Pattern Matching & Text Processing
- **regex** (version 1) - Regular expressions (agent token extraction)
- **aho-corasick** (version 1) - Multi-pattern string search
- **glob** (version 0.3) - Glob pattern matching

#### Networking
- **ureq** (version 2) - HTTP client (self-update functionality)

#### Randomization & Utilities
- **rand** (version 0.8) - Random jitter (backoff desynchronization)
- **cfg-if** (version 1) - Conditional compilation
- **gethostname** (version 0.4) - Hostname detection

---

### 3. Development Dependencies

Required for testing and development:
- **tokio-test** (version 0.4) - Tokio testing utilities
- **tempfile** (version 3) - Temporary file handling
- **proptest** (version 1) - Property-based testing
- **filetime** (version 0.2) - File time manipulation
- **criterion** (version 0.5) - Benchmarking framework

---

### 4. Optional System Dependencies

#### For Installation Script (`install.sh`)
The installer optionally uses these system tools (but can work without them):

- **curl** or **wget** - Downloading releases
- **sha256sum** or **shasum** - Checksum verification
- **gpg** - GPG signature verification (optional)

#### For Build Process
- **git** - Version control (for building from git repository)
- **make** - Build automation (if using makefiles)

---

## Minimum Version Requirements

| Component | Minimum Version | Recommended Version |
|-----------|----------------|-------------------|
| Rust | 1.75 | Latest stable |
| Cargo | Bundled with Rust 1.75 | Latest stable |
| tokio | 1.x | Latest 1.x |
| serde | 1.x | Latest 1.x |
| clap | 4.x | Latest 4.x |
| chrono | 0.4.x | Latest 0.4.x |

---

## Build Requirements

### Memory & Disk Space
- **RAM:** 2GB minimum (4GB+ recommended for full builds)
- **Disk Space:** ~500MB for dependencies, ~2GB for release build with LTO

### Build Commands
```bash
# Debug build
cargo build

# Release build (optimized)
cargo build --release

# Run tests
cargo test

# Run linter
cargo clippy --all-targets -- -D warnings

# Format code
cargo fmt
```

### Cross-Compilation Targets
- **Linux AMD64:** `x86_64-unknown-linux-gnu` (native)
- **macOS ARM64:** `aarch64-apple-darwin` (cross-compile)

---

## Runtime Requirements

### Bead Store Backend
Pluck requires a bead store backend (one of):
- **SQLite** (via `br` CLI) - `.beads/beads.db` database
- **br CLI** - Bead store management tool

### Configuration Files
- **`.needle.yaml`** - Needle configuration (optional, has defaults)
- **`.beads/issues.jsonl`** - Bead checkpoint file

### Environment Variables (Optional)
- `RUST_LOG` - Control logging verbosity (e.g., `RUST_LOG=needle::strand::pluck=trace`)
- `NEEDLE_INSTALL_PATH` - Custom installation path for installer

---

## Feature Flags

Pluck supports these Cargo features:

- **default** - Includes `otlp` feature
- **otlp** - Enable OpenTelemetry/OTLP telemetry
- **integration** - Integration testing with testcontainers

### Building with Features
```bash
# Build without OTLP
cargo build --no-default-features

# Build with integration tests
cargo build --features integration

# Build with all features
cargo build --all-features
```

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
install -m 755 target/release/needle ~/.local/bin/needle
```

---

## Verification Checklist

Before running Pluck in production, verify:

- [ ] Rust 1.75+ is installed (`rustc --version`)
- [ ] Cargo is working (`cargo --version`)
- [ ] All dependencies build successfully (`cargo build`)
- [ ] Tests pass (`cargo test`)
- [ ] Linter passes (`cargo clippy`)
- [ ] Code is formatted (`cargo fmt --check`)
- [ ] Binary executes (`needle --version`)
- [ ] Bead store is accessible (`br list`)

---

## Troubleshooting

### Build Issues

**Problem:** "error: linker `aarch64-linux-gnu-gcc` not found"
```bash
# Install cross-compilation toolchain
sudo apt install gcc-aarch64-linux-gnu
```

**Problem:** Out of memory during build
```bash
# Limit parallel jobs
export CARGO_BUILD_JOBS=2
cargo build
```

### Runtime Issues

**Problem:** "Bead store connection failed"
- Verify `.beads/beads.db` exists
- Check file permissions
- Ensure `br` CLI is installed

**Problem:** High memory usage
- Pluck is memory-efficient (<100MB typical)
- If excessive, check for leaks in dependencies

---

## Security Considerations

- All dependencies are from reputable crates.io sources
- No network access required for core Pluck functionality
- Optional `ureq` dependency for self-update (can be disabled)
- File locking (`fs2`) prevents concurrent access issues
- SHA-256 hashing (`sha2`) for content integrity

---

## Next Steps

After verifying dependencies:

1. **Install br CLI** for bead store management
2. **Configure `.needle.yaml`** for your workspace
3. **Initialize bead store** with `br init`
4. **Run Pluck** with `needle run --agent <agent> --identity <identity>`

---

## Additional Resources

- **NEEDLE Repository:** https://github.com/jedarden/NEEDLE
- **Documentation:** `/home/coding/NEEDLE/docs/`
- **Examples:** `/home/coding/NEEDLE/docs/examples/`
- **Pluck Source:** `/home/coding/NEEDLE/src/strand/pluck.rs`
- **Cargo.toml:** `/home/coding/NEEDLE/Cargo.toml`

---

**Document Version:** 1.0  
**Last Updated:** 2026-07-09  
**Bead ID:** bf-29m3g  
**Status:** Complete
