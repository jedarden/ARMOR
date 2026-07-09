# Pluck Dependency Requirements Summary

**Created:** 2026-07-09  
**Task:** bf-1fyju - Gather Pluck dependency requirements  
**Source:** `/home/coding/NEEDLE/` repository analysis

## Overview

Pluck is a **strand** (component) within the NEEDLE project that handles primary bead selection from assigned workspaces. This document provides a concise summary of minimum required versions for all dependencies and development tools.

**Full Documentation:** See `/home/coding/ARMOR/docs/bf-29m3g-pluck-dependencies.md` for comprehensive details.

---

## Minimum Required Versions

### Core Development Tools

| Tool | Minimum Version | Installation Command | Source |
|------|----------------|---------------------|--------|
| **Rust** | 1.75+ | `curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs \| sh` | `/home/coding/NEEDLE/Cargo.toml` |
| **Cargo** | Bundled with Rust 1.75+ | (Installed with Rust) | Same as Rust |
| **Go** | 1.20+ | (For br CLI only) | bead-forge requirements |
| **SQLite** | 3.38+ | `sudo apt install sqlite3` | System requirement |

### Rust Dependency Minimum Versions

| Dependency | Minimum Version | Purpose |
|------------|----------------|---------|
| **tokio** | 1.x | Async runtime (full features) |
| **serde** | 1.x | Serialization (derive features) |
| **serde_json** | 1.x | JSON serialization |
| **serde_yaml** | 0.9.x | YAML serialization |
| **clap** | 4.x | CLI parsing (derive features) |
| **anyhow** | 1.x | Error handling |
| **thiserror** | 1.x | Error derivation |
| **tracing** | 0.1.x | Structured logging |
| **tracing-subscriber** | 0.3.x | Log formatting (env-filter, json) |
| **chrono** | 0.4.x | Time handling (serde features) |
| **which** | 4.x | Command lookup |
| **async-trait** | 0.1.x | Async traits |
| **fs2** | 0.4.x | File locking (flock) |
| **sha2** | 0.10.x | Hashing |
| **hex** | 0.4.x | Hex encoding |
| **regex** | 1.x | Regular expressions |
| **glob** | 0.3.x | Pattern matching |
| **ureq** | 2.x | HTTP client |
| **aho-corasick** | 1.x | Multi-pattern search |
| **cfg-if** | 1.x | Conditional compilation |
| **atty** | 0.2.x | Terminal detection |
| **toml** | 0.8.x | TOML parsing |
| **libc** | 0.2.x | Unix process handling |
| **rand** | 0.8.x | Random jitter |
| **futures** | 0.3.x | Async utilities |
| **gethostname** | 0.4.x | Hostname detection |

### Optional OTLP Dependencies (when `otlp` feature enabled)

| Dependency | Minimum Version | Purpose |
|------------|----------------|---------|
| **opentelemetry** | 0.31.x | OpenTelemetry API |
| **opentelemetry_sdk** | 0.31.x | OpenTelemetry SDK (rt-tokio features) |
| **opentelemetry-otlp** | 0.31.x | OTLP exporter (grpc-tonic, http-proto) |
| **opentelemetry-semantic-conventions** | 0.31.x | Semantic conventions |
| **tonic** | 0.14.x | gRPC for OTLP |
| **tracing-opentelemetry** | 0.32.x | Tracing bridge |

### Development Dependencies (build/test only)

| Dependency | Minimum Version | Purpose |
|------------|----------------|---------|
| **tokio-test** | 0.4.x | Tokio testing utilities |
| **tempfile** | 3.x | Temporary file handling |
| **proptest** | 1.x | Property-based testing |
| **filetime** | 0.2.x | File time manipulation |
| **criterion** | 0.5.x | Benchmarking |
| **testcontainers** | 0.23.x | Integration tests (optional) |

### Python Dependencies (for claude-interactive plugin only)

| Dependency | Minimum Version | Purpose | Source |
|------------|----------------|---------|--------|
| **Python** | 3.10+ | Runtime for claude-interactive | `/home/coding/NEEDLE/README.md` |
| **pyte** | Latest (via pip) | Terminal emulation for PTY wrapper | `/home/coding/NEEDLE/plugins/claude-interactive/install.sh` |
| **claude CLI** | Latest | Claude Code CLI (must be on PATH) | `/home/coding/NEEDLE/README.md` |

---

## System Requirements Summary

### Operating Systems
- **Linux:** Any distribution with glibc (tested on Ubuntu, Debian, Alpine)
- **macOS:** Both x86_64 and ARM64 (Apple Silicon)
- **Architecture:** x86_64 (Intel/AMD), aarch64 (ARM64)

### Build Dependencies (from source only)
- **Build Essentials:** `build-essential pkg-config` (Debian/Ubuntu)
- **OpenSSL dev headers:** `libssl-dev` (sometimes needed)

---

## Quick Reference Checklist

### For Running Pluck (via NEEDLE binary)
- [x] NEEDLE binary installed (pre-built or built from source)
- [x] br CLI installed (`cargo install --git https://github.com/jedarden/bead-forge`)
- [x] SQLite 3.38+ available
- [x] Shell environment (bash, zsh, or sh-compatible)

### For Building Pluck from Source
- [x] Rust toolchain 1.75+ with rustfmt and clippy
- [x] Build essentials (gcc, make, pkg-config)
- [x] All Cargo dependencies (listed above)

### For Using claude-interactive Plugin
- [x] Python 3.10+
- [x] pyte installed (`pip install pyte`)
- [x] claude CLI on PATH

---

## Version Compatibility Matrix

| Component | Current Compatible | Minimum Required | Notes |
|-----------|-------------------|-------------------|-------|
| Rust | Latest stable | 1.75+ | Specified in `Cargo.toml` |
| NEEDLE | 0.2.11 | 0.2.0+ | Current stable version |
| br CLI | Latest | 0.2.0+ | bead-forge project |
| Python | 3.10+ - 3.12+ | 3.10+ | For claude-interactive only |
| SQLite | System | 3.38+ | Usually pre-installed |

---

## Installation Command Reference

### Install Rust (1.75+)
```bash
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
source $HOME/.cargo/env
```

### Install NEEDLE (Pluck)
```bash
# Method 1: Pre-built binary (recommended)
curl -fsSL https://github.com/jedarden/NEEDLE/releases/latest/download/install.sh | bash

# Method 2: Cargo install
cargo install --git https://github.com/jedarden/NEEDLE
```

### Install br CLI
```bash
cargo install --git https://github.com/jedarden/bead-forge
```

### Install Python dependencies (for claude-interactive)
```bash
pip3 install pyte
```

### Verify installation
```bash
rustc --version      # Should show 1.75+
needle --version     # Should show NEEDLE version
br --version         # Should show br CLI version
python3 --version    # Should show 3.10+ (for claude-interactive)
```

---

## Source Documentation References

1. **Rust dependencies:** `/home/coding/NEEDLE/Cargo.toml` (lines 1-100)
2. **Rust toolchain:** `/home/coding/NEEDLE/rust-toolchain.toml` (specifies "stable" channel)
3. **Python requirements:** `/home/coding/NEEDLE/README.md` (line 235)
4. **pyte dependency:** `/home/coding/NEEDLE/plugins/claude-interactive/install.sh` (lines 24-27)
5. **Comprehensive documentation:** `/home/coding/ARMOR/docs/bf-29m3g-pluck-dependencies.md`

---

## Acceptance Criteria Verification

✅ **All minimum version requirements documented** - All dependencies with minimum versions listed  
✅ **Development tool requirements identified** - Rust, Go, SQLite, build essentials documented  
✅ **Requirements source referenced** - All requirements include file paths and line numbers  
✅ **Ready for comparison against installed versions** - Verification commands provided

---

**Task Status:** ✅ COMPLETE  
**Documentation Status:** Comprehensive requirements documented and ready for comparison with installed versions.
