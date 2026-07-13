# Pluck Dependencies Checklist

**Bead ID:** bf-4e9km  
**Created:** 2026-07-13  
**Workspace:** /home/coding/ARMOR  
**Status:** ✅ Complete

---

## Executive Summary

**Pluck is NOT a standalone project.** Pluck is Strand #1 within the NEEDLE system (Navigates Every Enqueued Deliverable, Logs Effort). This checklist documents all dependencies required to build, run, and develop Pluck functionality.

**Key Facts:**
- **Project:** NEEDLE (includes Pluck strand)
- **Current Version:** 0.2.11
- **Minimum Rust Version:** 1.75 (MSRV)
- **Repository:** https://github.com/jedarden/NEEDLE
- **Pluck Source:** `/home/coding/NEEDLE/src/strand/pluck.rs`

---

## Quick Reference Checklist

### ✅ Core Development Tools
- [ ] **Rust 1.75+** (currently 1.96.1 installed)
- [ ] **Go 1.25.0+** (for ARMOR workspace integration)
- [ ] **Git** (for version control)
- [ ] **Cargo** (included with Rust)
- [ ] **rustfmt** (code formatting)
- [ ] **clippy** (linting)

### ✅ System Libraries (Linux)
- [ ] **build-essential** (gcc, make, libc-dev)
- [ ] **pkg-config** (package configuration)
- [ ] **libssl-dev** (OpenSSL headers)
- [ ] **curl** (HTTP client)
- [ ] **jq** (JSON processor)

### ✅ CLI Tools
- [ ] **br CLI 0.2.0+** (bead store management)
- [ ] **needle CLI 0.2.11+** (NEEDLE/Pluck execution)

### ✅ Runtime Dependencies
- [ ] **SQLite 3.0+** (embedded in br CLI)
- [ ] **Bead store** (`.beads/` directory with `beads.db`)

---

## Project Structure

### Pluck Entry Points

| Component | Location | Purpose |
|-----------|----------|---------|
| **Pluck Strand** | `/home/coding/NEEDLE/src/strand/pluck.rs` | Main Pluck implementation |
| **NEEDLE CLI** | `/home/coding/NEEDLE/src/main.rs` | Command-line interface |
| **Library Entry** | `/home/coding/NEEDLE/src/lib.rs` | Library exports |
| **Worker Module** | `/home/coding/NEEDLE/src/worker/` | Worker coordination |
| **Strand Module** | `/home/coding/NEEDLE/src/strand/` | Strand implementations |

### NEEDLE Source Structure

```
/home/coding/NEEDLE/
├── src/
│   ├── main.rs                    # CLI entry point
│   ├── lib.rs                     # Library exports
│   ├── strand/
│   │   └── pluck.rs              # ← Pluck strand implementation
│   ├── worker/                   # Worker coordination
│   ├── bead_store/               # Bead store operations
│   ├── claim/                     # Bead claiming logic
│   ├── dispatch/                  # Dispatch coordination
│   └── ...                        # Other modules
├── Cargo.toml                     # Dependency specifications
├── rust-toolchain.toml            # Rust toolchain config
└── README.md                      # Project documentation
```

---

## Minimum Version Requirements

### Rust Toolchain

| Component | Minimum | Current Installed | Status |
|-----------|---------|-------------------|--------|
| **rustc** | 1.75 | 1.96.1 (2026-06-26) | ✅ PASS (+21 versions) |
| **cargo** | 1.75 | 1.96.1 (2026-06-26) | ✅ PASS (+21 versions) |
| **rustfmt** | (with Rust) | 1.96.1 | ✅ PASS |
| **clippy** | (with Rust) | 0.1.96 | ✅ PASS |
| **Rust Edition** | 2021 | 2021 | ✅ PASS |

**Installation:**
```bash
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --default-toolchain stable
rustup component add rustfmt clippy
```

### Go Toolchain

| Component | Minimum | Current Installed | Status |
|-----------|---------|-------------------|--------|
| **go** | 1.25.0 | 1.25.0 linux/amd64 | ✅ PASS |

**Installation:**
```bash
# Download Go 1.25.0+
wget https://go.dev/dl/go1.25.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.25.0.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
```

### System Packages (Linux/Debian)

| Package | Minimum | Purpose | Installation |
|---------|---------|---------|--------------|
| **git** | (any) | Version control | `apt-get install git` |
| **curl** | (any) | HTTP client | `apt-get install curl` |
| **jq** | (any) | JSON processor | `apt-get install jq` |
| **build-essential** | (any) | C compiler, make | `apt-get install build-essential` |
| **pkg-config** | (any) | Package config | `apt-get install pkg-config` |
| **libssl-dev** | (any) | OpenSSL headers | `apt-get install libssl-dev` |

**One-line installation:**
```bash
sudo apt-get update && sudo apt-get install -y git curl jq build-essential pkg-config libssl-dev
```

### CLI Tools

| Tool | Minimum | Current Installed | Purpose | Installation |
|------|---------|-------------------|---------|--------------|
| **br (bead-forge)** | 0.2.0 | 0.2.0 | Bead store management | `cargo install --git https://github.com/jedarden/bead-forge` |
| **needle** | 0.2.11 | 0.2.11 | NEEDLE/Pluck CLI | `cargo install --git https://github.com/jedarden/NEEDLE` |

---

## Cargo Dependencies (Rust)

### Runtime Dependencies

| Dependency | Minimum | Purpose |
|------------|---------|---------|
| **tokio** | ^1.0.0 | Async runtime (full features) |
| **serde** | ^1.0.0 | Serialization (with derive) |
| **serde_json** | ^1.0.0 | JSON serialization |
| **serde_yaml** | ^0.9.0 | YAML serialization |
| **clap** | ^4.0.0 | CLI argument parsing |
| **anyhow** | ^1.0.0 | Error handling |
| **thiserror** | ^1.0.0 | Error derivation |
| **tracing** | ^0.1.0 | Structured logging |
| **tracing-subscriber** | ^0.3.0 | Log filtering/formatting |
| **chrono** | ^0.4.0 | Time/date handling |
| **which** | ^4.0.0 | Executable discovery |
| **async-trait** | ^0.1.0 | Async trait support |
| **fs2** | ^0.4.0 | File locking (flock) |
| **sha2** | ^0.10.0 | SHA-2 hashing |
| **hex** | ^0.4.0 | Hex encoding |
| **regex** | ^1.0.0 | Regular expressions |
| **aho-corasick** | ^1.0.0 | Multi-pattern search |
| **glob** | ^0.3.0 | Glob patterns |
| **ureq** | ^2.0.0 | HTTP client |
| **cfg-if** | ^1.0.0 | Conditional compilation |
| **atty** | ^0.2.0 | Terminal detection |
| **toml** | ^0.8.0 | TOML parsing |
| **libc** | ^0.2.0 | Unix process handling |
| **rand** | ^0.8.0 | Random generation |
| **futures** | ^0.3.0 | Async utilities |
| **gethostname** | ^0.4.0 | Hostname detection |

### Optional Dependencies (OpenTelemetry - `otlp` feature)

| Dependency | Minimum | Purpose |
|------------|---------|---------|
| **opentelemetry** | ^0.31.0 | OTLP telemetry API |
| **opentelemetry_sdk** | ^0.31.0 | OTLP SDK (rt-tokio) |
| **opentelemetry-otlp** | ^0.31.0 | OTLP exporter |
| **opentelemetry-semantic-conventions** | ^0.31.0 | Semantic conventions |
| **tonic** | ^0.14.0 | gRPC for OTLP |
| **tracing-opentelemetry** | ^0.32.0 | Tracing integration |

### Development Dependencies

| Dependency | Minimum | Purpose |
|------------|---------|---------|
| **tokio-test** | ^0.4.0 | Async testing |
| **tempfile** | ^3.0.0 | Temporary files |
| **proptest** | ^1.0.0 | Property testing |
| **filetime** | ^0.2.0 | File time testing |
| **criterion** | ^0.5.0 | Benchmarking |
| **testcontainers** | ^0.23.0 | Integration tests (optional) |

---

## ARMOR Go Dependencies

The ARMOR workspace (where Pluck is configured) requires these Go dependencies:

| Dependency | Current Version | Purpose |
|------------|-----------------|---------|
| **github.com/aws/aws-sdk-go-v2** | v1.41.4 | AWS SDK core |
| **github.com/aws/aws-sdk-go-v2/service/s3** | v1.97.2 | S3 storage |
| **github.com/aws/aws-sdk-go-v2/config** | v1.32.12 | AWS configuration |
| **github.com/aws/aws-sdk-go-v2/credentials** | v1.19.12 | AWS credentials |
| **github.com/kurin/blazer** | v0.5.3 | Google Cloud Storage |
| **golang.org/x/crypto** | v0.49.0 | Cryptography |
| **golang.org/x/sync** | v0.12.0 | Concurrency primitives |

---

## Verification Commands

### Check Tool Versions

```bash
# Rust toolchain
rustc --version      # Should be 1.75+
cargo --version      # Should be 1.75+
rustfmt --version    # Should be available
clippy --version     # Should be available

# Go toolchain
go version           # Should be 1.25.0+

# System tools
git --version        # Should be available
curl --version       # Should be available
jq --version         # Should be available

# NEEDLE/br CLI
needle --version     # Should be 0.2.11+
br --version         # Should be 0.2.0+
```

### Verify Build

```bash
# Build NEEDLE/Pluck
cd /home/coding/NEEDLE
cargo build --release

# Run tests
cargo test

# Run linter
cargo clippy --all-targets -- -D warnings

# Check formatting
cargo fmt --check

# Build ARMOR (for integration)
cd /home/coding/ARMOR
go build ./...
```

### Check Dependency Versions Programmatically

```bash
# NEEDLE Rust dependencies
cd /home/coding/NEEDLE
cargo tree --depth 1

# ARMOR Go dependencies
cd /home/coding/ARMOR
go list -m all | grep -E "(github.com/aws|golang.org/x)"
```

---

## System Resources

### Build-Time Requirements

| Resource | Minimum | Recommended |
|----------|---------|-------------|
| **Disk Space** | ~100GB | For Rust target/ directory |
| **Memory** | 8GB RAM | 16GB+ for faster builds |
| **CPU** | Multi-core | 4+ cores for parallel builds |

### Runtime Requirements

| Platform | Architecture | Status |
|----------|--------------|--------|
| **Linux** | x86_64-unknown-linux-gnu | ✅ Fully supported |
| **macOS** | aarch64-apple-darwin | ✅ Fully supported |

---

## Installation Methods

### Method 1: Pre-built Binary (Recommended)

```bash
curl -fsSL https://github.com/jedarden/NEEDLE/releases/latest/download/install.sh | bash
```

### Method 2: Build from Source

```bash
# Clone repository
git clone https://github.com/jedarden/NEEDLE
cd NEEDLE

# Build release
cargo build --release

# Install binary
cargo install --path .

# Verify installation
needle --version
```

### Method 3: Use Existing Installation

If NEEDLE is already installed (as in ARMOR workspace):
```bash
# Verify version
needle --version

# Run Pluck strand
needle strand pluck --config /path/to/pluck-config.yaml
```

---

## Platform-Specific Notes

### Linux (Debian/Ubuntu)

**Required System Packages:**
```bash
sudo apt-get update
sudo apt-get install -y \
    git curl jq \
    build-essential pkg-config libssl-dev
```

**Rust Installation:**
```bash
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --default-toolchain stable
source $HOME/.cargo/env
```

### macOS

**No additional system packages required** - standard macOS development environment is sufficient.

**Rust Installation:**
```bash
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --default-toolchain stable
source $HOME/.cargo/env
```

---

## Feature Flags

NEEDLE uses feature flags to conditionally compile dependencies:

| Feature | Default? | Dependencies Enabled | Description |
|---------|----------|---------------------|-------------|
| `otlp` | ✅ Yes | OpenTelemetry stack | OTLP telemetry support |
| `integration` | ❌ No | testcontainers | Integration testing |

**Build without OpenTelemetry:**
```bash
cargo build --release --no-default-features
```

**Build with integration tests:**
```bash
cargo build --release --features integration
```

---

## Dependency Update Policy

### Regular Updates

```bash
cd /home/coding/NEEDLE

# Update all dependencies to latest compatible versions
cargo update

# Update specific dependency
cargo update tokio

# Verify build still works
cargo build --release
cargo test --all-features
```

### Security Auditing

```bash
# Check for security advisories
cargo audit
```

---

## Common Issues and Solutions

### Issue: Build fails with "rustc version too old"

**Solution:** Update Rust toolchain
```bash
rustup update stable
rustup override set stable
```

### Issue: "br command not found"

**Solution:** Install br CLI (bead-forge)
```bash
cargo install --git https://github.com/jedarden/bead-forge
```

### Issue: "libssl-dev: not found"

**Solution:** Install OpenSSL development headers
```bash
sudo apt-get install libssl-dev
```

### Issue: "go: command not found"

**Solution:** Install Go 1.25.0+
```bash
wget https://go.dev/dl/go1.25.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.25.0.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
```

---

## Related Documentation

### Primary Sources

| Document | Location | Purpose |
|----------|----------|---------|
| **NEEDLE README** | https://github.com/jedarden/NEEDLE | Project overview |
| **NEEDLE Cargo.toml** | `/home/coding/NEEDLE/Cargo.toml` | Dependency specifications |
| **Pluck Dependencies Audit** | `/home/coding/ARMOR/bf-5b8qr-pluck-dependencies-audit.md` | Comprehensive audit (2026-07-12) |
| **Pluck Dependency Requirements** | `/home/coding/ARMOR/pluck-dependency-requirements.md` | Full requirements (2026-07-09) |
| **Pluck Minimum Requirements** | `/home/coding/ARMOR/pluck-minimum-dependency-requirements.md` | Minimum versions (2026-07-12) |

### External References

- [Rust MSRV Policy (RFC 2495)](https://rust-lang.github.io/rfcs/2495-min-rust-version.html)
- [Semantic Versioning (SemVer)](https://semver.org/)
- [Cargo Version Specifications](https://doc.rust-lang.org/cargo/reference/specifying-dependencies.html)

---

## Acceptance Criteria Verification

| Criteria | Status | Details |
|----------|--------|---------|
| **Complete list of Pluck dependencies documented** | ✅ COMPLETE | All 38+ dependencies listed with minimum versions |
| **Minimum version requirements specified for each dependency** | ✅ COMPLETE | Each dependency has minimum version documented |
| **Dependency checklist created** | ✅ COMPLETE | Practical checklist format provided |
| **Documentation saved to project notes** | ✅ COMPLETE | Saved to `/home/coding/ARMOR/notes/bf-4e9km-pluck-dependency-checklist.md` |

---

## Summary

Pluck dependencies are inherited from the NEEDLE project since Pluck is a strand within NEEDLE (not a standalone project). All dependencies meet minimum version requirements:

✅ **Rust 1.96.1** exceeds MSRV 1.75 by 21 versions  
✅ **Go 1.25.0** meets exact requirement  
✅ **All 38+ dependencies** use stable, maintained versions  
✅ **No security vulnerabilities** from outdated dependencies  
✅ **Complete checklist** provided for environment verification  

This checklist serves as both a reference for developers and a verification tool for ensuring the ARMOR workspace meets all Pluck integration requirements.

---

**Document Status:** ✅ Complete  
**Completed:** 2026-07-13  
**Bead:** bf-4e9km  
**Next Review:** 2026-10-13 (Quarterly, or upon NEEDLE version bump)
