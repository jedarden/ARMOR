# Pluck Dependencies Documentation

**Bead:** bf-29m3g  
**Last Updated:** 2026-07-09  
**Status:** ✅ Complete

## Overview

**Pluck** is a core "strand" within the NEEDLE system that handles primary bead selection from assigned workspaces. It processes >90% of all beads by querying the bead store for unassigned, ready beads, filtering by excluded labels, and sorting them in deterministic priority order.

**Source Location:** `/home/coding/NEEDLE/src/strand/pluck.rs`  
**Project:** NEEDLE (Rust-based task/work item management system)

---

## System Requirements

### Platform
- **OS:** Linux (tested on Debian Bookworm)
- **Architecture:** x86_64

### Minimum Versions
- **Rust:** 1.75+ (specified in `rust-version` field)
- **Edition:** Rust 2021

---

## Core Rust Dependencies

### Async Runtime
- **tokio** (v1+) - Full feature set
  - Async runtime and utilities
  - Required for async operations

### Serialization
- **serde** (v1+) - With derive feature
  - Serialization framework
- **serde_json** (v1+)
  - JSON serialization support
- **serde_yaml** (v0.9+)
  - YAML serialization support

### CLI Framework
- **clap** (v4+) - With derive feature
  - Command-line argument parsing

### Error Handling
- **anyhow** (v1+)
  - Convenient error handling
- **thiserror** (v1+)
  - Derive error enums

### Logging & Telemetry
- **tracing** (v0.1+)
  - Structured logging framework
- **tracing-subscriber** (v0.3+) - With env-filter, json features
  - Log routing and formatting
  - Required for RUST_LOG environment variable support

### Time Handling
- **chrono** (v0.4+) - With serde feature
  - Date and time manipulation
  - Used for bead timestamps

### Process Management
- **which** (v4+)
  - Locate executables in PATH

### Async Traits
- **async-trait** (v0.1+)
  - Async trait support

### File Operations
- **fs2** (v0.4+)
  - Cross-platform file locking (flock)

### Hashing
- **sha2** (v0.10+)
  - SHA-2 hashing for content fingerprints
- **hex** (v0.4+)
  - Hex encoding/decoding

### Pattern Matching
- **regex** (v1+)
  - Regular expression support (agent token extraction)
- **glob** (v0.3+)
  - Glob pattern matching (doc file discovery)
- **aho-corasick** (v1+)
  - Multi-pattern string search

### HTTP Client
- **ureq** (v2+)
  - Simple HTTP client for self-update functionality

### Configuration
- **cfg-if** (v1+)
  - Conditional compilation
- **toml** (v0.8+)
  - TOML parsing (gitleaks config)

### Terminal Detection
- **atty** (v0.2+)
  - Terminal detection (ANSI color support)

### System Integration
- **libc** (v0.2+)
  - Unix process handling (PID liveness checks)
- **rand** (v0.8+)
  - Random jitter for backoff desynchronization
- **gethostname** (v0.4+)
  - Hostname detection

### Async Utilities
- **futures** (v0.3+)
  - Async utilities

---

## OpenTelemetry Dependencies (Optional)

The following dependencies are **optional** and gated behind the `otlp` feature:

- **opentelemetry** (v0.31+) - Optional
- **opentelemetry_sdk** (v0.31+) - With rt-tokio feature - Optional
- **opentelemetry-otlp** (v0.31+) - With grpc-tonic, http-proto features - Optional
- **opentelemetry-semantic-conventions** (v0.31+) - Optional
- **tonic** (v0.14+) - Optional
- **tracing-opentelemetry** (v0.32+) - Optional

These are only needed if you want OTLP telemetry export functionality.

---

## Development/Testing Dependencies

### Test Utilities
- **tokio-test** (v0.4+)
  - Tokio testing utilities
- **tempfile** (v3+)
  - Temporary file handling in tests
- **proptest** (v1+)
  - Property-based testing
- **filetime** (v0.2+)
  - File time manipulation in tests
- **criterion** (v0.5+)
  - Benchmarking framework

### Integration Testing
- **testcontainers** (v0.23+) - Optional (requires `integration` feature)
  - Docker container management for integration tests

---

## System-Level Dependencies

### Build Tools
- **build-essential** - GCC, make, etc.
- **pkg-config** - Package configuration tool
- **libssl-dev** - OpenSSL development libraries

### Utilities
- **git** - Version control
- **curl** - HTTP client for downloads
- **jq** - JSON processor

### Optional: GitHub CLI
- **gh** - GitHub CLI (for CI workflows)

---

## Environment Configuration

### Logging Configuration
Pluck uses the RUST_LOG environment variable for debug output:

```bash
# Standard debug level
export RUST_LOG=needle::strand::pluck=debug

# Comprehensive debug
export RUST_LOG=needle::strand::pluck=trace

# Full system context
export RUST_LOG=needle::strand::pluck=trace,needle=debug
```

### Default Excluded Labels
When not configured, Pluck excludes beads with these labels by default:
- `deferred`
- `human`
- `blocked`

---

## Build & Installation

### From Source
```bash
# Clone NEEDLE repository
git clone <repository-url> NEEDLE
cd NEEDLE

# Build release binary
cargo build --release

# Run NEEDLE with Pluck strand
./target/release/needle run -w /path/to/workspace -c 1
```

### Feature Flags
- `--features otpl` - Enable OpenTelemetry support
- `--features integration` - Enable integration tests

---

## Dependency Checklist

### For Development
- [x] Rust 1.75+
- [x] Cargo (comes with Rust)
- [x] build-essential
- [x] pkg-config
- [x] libssl-dev

### For Runtime
- [x] git (for workspace operations)
- [x] curl (for self-update)
- [x] jq (for JSON processing in scripts)

### Optional
- [ ] gh CLI (for GitHub integration)
- [ ] Docker (for integration tests)

---

## Verification

To verify all dependencies are installed:

```bash
# Check Rust installation
rustc --version
cargo --version

# Check system dependencies
gcc --version
pkg-config --version

# Check build
cd /home/coding/NEEDLE
cargo check
cargo test --lib strand::pluck
```

---

## Related Files

- **Cargo.toml:** `/home/coding/NEEDLE/Cargo.toml`
- **Pluck Source:** `/home/coding/NEEDLE/src/strand/pluck.rs`
- **CI Dockerfile:** `/home/coding/NEEDLE/ci/Dockerfile.ci`

---

## Next Steps

Based on this documentation, you can:

1. ✅ **Set up a development environment** - All dependencies documented
2. ✅ **Build Pluck from source** - Build commands provided
3. ✅ **Configure logging** - RUST_LOG patterns documented
4. ✅ **Run tests** - Testing dependencies identified
5. ✅ **Enable optional features** - OpenTelemetry dependencies listed

---

**Documentation Complete** - All required dependencies for Pluck have been identified and documented with minimum version requirements where applicable.