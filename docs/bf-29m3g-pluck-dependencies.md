# Pluck Dependencies Documentation

**Created:** 2026-07-09  
**Bead:** bf-29m3g  
**Purpose:** Document all required system libraries and dependencies needed for Pluck

## What is Pluck?

Pluck is a **strand** (component) within the NEEDLE project that handles primary bead selection from the assigned workspace. It processes >90% of all bead operations by querying the bead store for unassigned, ready beads, filtering by excluded labels, and sorting them in deterministic priority order.

**Repository:** https://github.com/jedarden/NEEDLE  
**Component Path:** `NEEDLE/src/strand/pluck.rs`  
**License:** MIT

## Project Structure

```
NEEDLE/
├── src/
│   ├── strand/
│   │   ├── pluck.rs          # Pluck strand implementation
│   │   ├── mend.rs           # Secondary strand
│   │   ├── explore.rs        # Exploration strand
│   │   └── ...
│   ├── lib.rs                # NEEDLE library
│   └── main.rs               # NEEDLE binary
├── Cargo.toml                # Rust dependencies
├── rust-toolchain.toml       # Rust version requirements
└── install.sh                # Installation script
```

## Minimum System Requirements

### Operating System
- **Linux:** Any distribution with glibc (tested on Ubuntu, Debian, Alpine)
- **macOS:** Both x86_64 and ARM64 (Apple Silicon)
- **Architecture:** x86_64 (Intel/AMD), aarch64 (ARM64)

### Minimum Versions
| Component | Minimum Version | Recommended Version |
|-----------|----------------|-------------------|
| Rust | 1.75+ | Latest stable |
| Go (for br CLI) | 1.20+ | Latest stable |
| SQLite | 3.38+ | Latest system version |

## Rust Dependencies

### Runtime Dependencies

These are required for Pluck to function:

| Dependency | Version | Purpose | Features |
|------------|---------|---------|----------|
| `tokio` | 1.x | Async runtime | full |
| `serde` | 1.x | Serialization | derive |
| `serde_json` | 1.x | JSON serialization | - |
| `serde_yaml` | 0.9.x | YAML serialization | - |
| `clap` | 4.x | CLI parsing | derive |
| `anyhow` | 1.x | Error handling | - |
| `thiserror` | 1.x | Error derivation | - |
| `tracing` | 0.1.x | Structured logging | - |
| `tracing-subscriber` | 0.3.x | Log formatting | env-filter, json |
| `chrono` | 0.4.x | Time handling | serde |
| `which` | 4.x | Command lookup | - |
| `async-trait` | 0.1.x | Async traits | - |
| `fs2` | 0.4.x | File locking (flock) | - |
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
| `rand` | 0.8.x | Random jitter | - |
| `futures` | 0.3.x | Async utilities | - |
| `gethostname` | 0.4.x | Hostname detection | - |

### Optional OTLP Dependencies

Required only when OpenTelemetry tracing is enabled (`otlp` feature):

| Dependency | Version | Purpose |
|------------|---------|---------|
| `opentelemetry` | 0.31.x | OpenTelemetry API |
| `opentelemetry_sdk` | 0.31.x | OpenTelemetry SDK | rt-tokio |
| `opentelemetry-otlp` | 0.31.x | OTLP exporter | grpc-tonic, http-proto |
| `opentelemetry-semantic-conventions` | 0.31.x | Semantic conventions | - |
| `tonic` | 0.14.x | gRPC for OTLP | - |
| `tracing-opentelemetry` | 0.32.x | Tracing bridge | - |

### Development Dependencies

Required only for building/testing from source:

| Dependency | Version | Purpose |
|------------|---------|---------|
| `tokio-test` | 0.4.x | Tokio testing utilities |
| `tempfile` | 3.x | Temporary file handling |
| `proptest` | 1.x | Property-based testing |
| `filetime` | 0.2.x | File time manipulation |
| `criterion` | 0.5.x | Benchmarking |
| `testcontainers` | 0.23.x | Integration tests (optional) |

## System-Level Dependencies

### Required

1. **Rust Toolchain** (1.75+)
   ```bash
   # Install via rustup
   curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
   
   # Or via system package manager
   apt install rustc cargo  # Debian/Ubuntu
   brew install rust        # macOS
   ```

2. **SQLite 3** (comes with br CLI)
   - Usually pre-installed on most systems
   - `sudo apt install sqlite3`  # Debian/Ubuntu
   - `brew install sqlite3`      # macOS

3. **br CLI** (bead store management)
   ```bash
   # Install from source
   cargo install --git https://github.com/jedarden/bead-forge
   
   # Or use pre-built binary
   curl -fsSL https://github.com/jedarden/bead-forge/releases/latest/download/install.sh | bash
   ```

### Build Dependencies (from source)

1. **Build Essentials**
   ```bash
   # Debian/Ubuntu
   sudo apt install build-essential pkg-config
   
   # macOS (via Xcode Command Line Tools)
   xcode-select --install
   ```

2. **OpenSSL development headers** (sometimes needed)
   ```bash
   sudo apt install libssl-dev  # Debian/Ubuntu
   brew install openssl         # macOS
   ```

## Installation Methods

### Method 1: Pre-built Binary (Recommended)

```bash
curl -fsSL https://github.com/jedarden/NEEDLE/releases/latest/download/install.sh | bash
```

**Dependencies:** None (binary is statically linked where possible)

### Method 2: Cargo Install

```bash
cargo install --git https://github.com/jedarden/NEEDLE
```

**Dependencies:** Rust toolchain (1.75+), build essentials

### Method 3: Build from Source

```bash
git clone https://github.com/jedarden/NEEDLE.git
cd NEEDLE
cargo build --release
cargo install --path .
```

**Dependencies:** Full Rust toolchain, build essentials, all Cargo dependencies

## Runtime Dependencies Summary

For **running** Pluck (via NEEDLE), you need:

1. ✅ **NEEDLE binary** (installed via any method above)
2. ✅ **br CLI** (bead store management)
3. ✅ **SQLite** (usually pre-installed)
4. ✅ **Shell environment** (bash, zsh, or sh-compatible)
5. ✅ **Agent CLI** (Claude Code, OpenCode, Codex, Aider, etc.)

For **building** Pluck from source, you additionally need:

6. ✅ **Rust toolchain 1.75+** with rustfmt and clippy
7. ✅ **Build essentials** (gcc, make, pkg-config)
8. ✅ **All Cargo dependencies** (listed above)

## Dependency Checklist

Use this checklist when setting up a new environment:

### Runtime Setup
- [ ] Install NEEDLE (pre-built binary or built from source)
- [ ] Install br CLI from bead-forge
- [ ] Verify SQLite is available: `sqlite3 --version`
- [ ] Verify NEEDLE works: `needle --version`
- [ ] Verify br works: `br --version`
- [ ] Configure agent CLI (Claude Code, etc.)

### Development Setup (if building from source)
- [ ] Install Rust 1.75+ via rustup
- [ ] Install build essentials
- [ ] Clone NEEDLE repository
- [ ] Run `cargo build --release`
- [ ] Run `cargo test` to verify
- [ ] Run `cargo clippy` for linting
- [ ] Run `cargo fmt` for formatting

## Version Compatibility

| NEEDLE Version | Rust Min | br CLI Version | Notes |
|----------------|---------|----------------|-------|
| 0.2.11 | 1.75 | 0.2.0+ | Current stable |
| 0.2.0 - 0.2.10 | 1.70 | 0.1.x | Earlier versions |
| < 0.2.0 | 1.65 | 0.1.x | Deprecated |

## Platform-Specific Notes

### Linux (Debian/Ubuntu)
```bash
# Quick install script
sudo apt update
sudo apt install -y build-essential pkg-config sqlite3 curl
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
curl -fsSL https://github.com/jedarden/NEEDLE/releases/latest/download/install.sh | bash
cargo install --git https://github.com/jedarden/bead-forge
```

### macOS (Apple Silicon)
```bash
# Quick install script
brew install rust sqlite3
curl -fsSL https://github.com/jedarden/NEEDLE/releases/latest/download/install.sh | bash
cargo install --git https://github.com/jedarden/bead-forge
```

### Alpine Linux
```bash
# Alpine requires musl-target
apk add build-base sqlite-dev curl
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
rustup target add x86_64-unknown-linux-musl
```

## Troubleshooting

### Common Issues

1. **"Rust not found"**
   - Install Rust via rustup: `curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh`

2. **"br: command not found"**
   - Install bead-forge: `cargo install --git https://github.com/jedarden/bead-forge`

3. **"sqlite3: error while loading shared libraries"**
   - Install SQLite: `sudo apt install sqlite3` or `brew install sqlite3`

4. **Build failures**
   - Ensure Rust 1.75+: `rustc --version`
   - Install build essentials: `sudo apt install build-essential pkg-config`

## Related Documentation

- **NEEDLE Repository:** https://github.com/jedarden/NEEDLE
- **bead-forge Repository:** https://github.com/jedarden/bead-forge
- **Pluck Source Code:** `src/strand/pluck.rs` in NEEDLE repo
- **ARMOR Project:** https://github.com/jedarden/ARMOR

## Maintenance Notes

- **Last Updated:** 2026-07-09
- **Rust Version Policy:** Follows Rust stable (minimum 1.75)
- **Dependency Updates:** Tracked via `Cargo.lock` - update via `cargo update`
- **Security Updates:** Monitor RustSec advisories for dependencies

---

**Next Steps:**
1. ✅ All dependencies documented
2. ✅ Minimum version requirements recorded
3. ✅ Installation methods covered
4. ✅ Platform-specific notes included
5. ✅ Troubleshooting guide provided
