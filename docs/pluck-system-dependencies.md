# Pluck System Dependencies Documentation

**Document Created:** 2026-07-12  
**Bead:** bf-49y71  
**Workspace:** /home/coding/ARMOR  
**Status:** ✅ Complete

## Overview

Pluck is a strand within the NEEDLE system that processes beads from assigned workspaces. This document provides a comprehensive inventory of all system-level dependencies required to build, deploy, and run Pluck.

---

## 1. Core Build System Dependencies

### 1.1 Rust Toolchain Requirements

| Component | Minimum Version | Current Version | Purpose |
|-----------|-----------------|-----------------|---------|
| **rustc** | 1.75 | 1.96.1 | Rust compiler |
| **cargo** | 1.75 | 1.96.1 | Package manager and build tool |
| **rustfmt** | - | 1.9.0-stable | Code formatter |
| **clippy** | - | 0.1.96 | Linter |

**MSRV (Minimum Supported Rust Version):** 1.75 (2023-12-28)  
**Rust Edition:** 2021

### 1.2 Rust Toolchain Configuration

**File:** `rust-toolchain.toml`

```toml
[toolchain]
channel = "stable"
components = ["rustfmt", "clippy"]
targets = ["x86_64-unknown-linux-gnu", "aarch64-apple-darwin"]
```

---

## 2. NEEDLE/Pluck Rust Dependencies

### 2.1 Core Runtime Dependencies

#### Async Runtime
| Dependency | Version | Purpose |
|------------|---------|---------|
| tokio | 1 | Async runtime with full features |

#### Serialization
| Dependency | Version | Purpose |
|------------|---------|---------|
| serde | 1 (with derive) | Serialization framework |
| serde_json | 1 | JSON serialization |
| serde_yaml | 0.9 | YAML serialization |

#### CLI Framework
| Dependency | Version | Purpose |
|------------|---------|---------|
| clap | 4 (with derive) | Command-line argument parsing |

#### Error Handling
| Dependency | Version | Purpose |
|------------|---------|---------|
| anyhow | 1 | Error handling |
| thiserror | 1 | Error derivation |

#### Logging/Telemetry
| Dependency | Version | Purpose |
|------------|---------|---------|
| tracing | 0.1 | Structured logging |
| tracing-subscriber | 0.3 (with env-filter, json) | Log filtering and formatting |
| tracing-opentelemetry | 0.32 (optional) | OpenTelemetry integration |

#### Time Handling
| Dependency | Version | Purpose |
|------------|---------|---------|
| chrono | 0.4 (with serde) | Time and date handling |

#### Process Management
| Dependency | Version | Purpose |
|------------|---------|---------|
| which | 4 | Locate executables in PATH |

#### Async Traits
| Dependency | Version | Purpose |
|------------|---------|---------|
| async-trait | 0.1 | Async trait support |

#### File Operations
| Dependency | Version | Purpose |
|------------|---------|---------|
| fs2 | 0.4 | Cross-platform file locking (flock) |

#### Cryptography
| Dependency | Version | Purpose |
|------------|---------|---------|
| sha2 | 0.10 | SHA-2 hashing (prompt content hash) |
| hex | 0.4 | Hex encoding (binary fingerprinting) |

#### Text Processing
| Dependency | Version | Purpose |
|------------|---------|---------|
| regex | 1 | Regular expressions (agent token extraction) |
| aho-corasick | 1 | Multi-pattern string search |

#### File Pattern Matching
| Dependency | Version | Purpose |
|------------|---------|---------|
| glob | 0.3 | Glob pattern matching (doc file discovery) |

#### HTTP Client
| Dependency | Version | Purpose |
|------------|---------|---------|
| ureq | 2 | Simple HTTP client (self-update) |

#### Utilities
| Dependency | Version | Purpose |
|------------|---------|---------|
| cfg-if | 1 | Conditional compilation |
| atty | 0.2 | Terminal detection (ANSI color support) |
| toml | 0.8 | TOML parsing (gitleaks config) |
| libc | 0.2 | Unix process handling (PID liveness check) |
| rand | 0.8 | Random jitter (backoff desynchronization) |
| futures | 0.3 | Async utilities |
| gethostname | 0.4 | Hostname detection |

### 2.2 OpenTelemetry Dependencies (Optional - otlp feature)

| Dependency | Version | Purpose |
|------------|---------|---------|
| opentelemetry | 0.31 | OpenTelemetry API |
| opentelemetry_sdk | 0.31 (with rt-tokio) | OpenTelemetry SDK |
| opentelemetry-otlp | 0.31 (with grpc-tonic, http-proto) | OTLP exporter |
| opentelemetry-semantic-conventions | 0.31 | Semantic conventions |
| tonic | 0.14 | gRPC for OTLP |

### 2.3 Development Dependencies

| Dependency | Version | Purpose |
|------------|---------|---------|
| tokio-test | 0.4 | Tokio testing utilities |
| tempfile | 3 | Temporary file handling |
| proptest | 1 | Property-based testing |
| filetime | 0.2 | File time manipulation |
| criterion | 0.5 | Benchmarking |

### 2.4 Integration Testing (Optional - integration feature)

| Dependency | Version | Purpose |
|------------|---------|---------|
| testcontainers | 0.23 | Docker container integration testing |

---

## 3. bead-forge (br CLI) Dependencies

### 3.1 Core Dependencies

| Dependency | Version | Purpose |
|------------|---------|---------|
| clap | 4 (with derive) | Command-line argument parsing |
| serde | 1 (with derive) | Serialization framework |
| serde_json | 1 | JSON serialization |
| serde_yaml | 0.9 | YAML serialization |
| chrono | 0.4 (with serde) | Time and date handling |
| rusqlite | 0.31 (with bundled) | SQLite database (bundled) |
| sha2 | 0.10 | SHA-2 hashing |
| rand | 0.8 | Random number generation |
| regex | 1 | Regular expressions |
| tracing | 0.1 | Structured logging |
| tracing-subscriber | 0.3 | Log formatting |
| anyhow | 1 | Error handling |
| thiserror | 1 | Error derivation |
| num-bigint | 0.4 | Big integer support |
| num-traits | 0.2 | Numeric traits |
| shell-words | 1 | Shell command parsing |
| which | 7 | Locate executables in PATH |

**Note:** rusqlite uses the "bundled" feature flag, which means it includes its own SQLite library and does not require system-level SQLite installation.

---

## 4. Platform-Specific Dependencies

### 4.1 Supported Platforms

| Platform | Architecture | Status | Notes |
|----------|--------------|--------|-------|
| **Linux** | x86_64 | ✅ Fully Supported | Primary target platform |
| **Linux** | aarch64 | ✅ Supported | ARM64 Linux systems |
| **macOS** | x86_64 | ✅ Supported | Intel-based Macs |
| **macOS** | aarch64 | ✅ Supported | Apple Silicon Macs |
| **Windows** | - | ⚠️ Partial | Limited support (Unix-specific features) |

### 4.2 Platform-Specific Code

#### Unix-Specific Features (Linux, macOS)

The following features are Unix-specific and have stub implementations on other platforms:

- **Signal Handling:** Unix signal handling for graceful shutdown (SIGTERM, SIGINT)
- **Process Management:** PID liveness checks using libc
- **File Locking:** flock-based file locking for coordination

#### Platform Detection

Pluck uses conditional compilation to detect the platform:

```rust
// Platform suffix for release downloads
fn get_platform_suffix() -> Result<&'static str> {
    match (std::env::consts::OS, std::env::consts::ARCH) {
        ("linux", "x86_64") => "x86_64-unknown-linux-gnu",
        ("linux", "aarch64") => "aarch64-unknown-linux-gnu",
        ("macos", "x86_64") => "x86_64-apple-darwin",
        ("macos", "aarch64") => "aarch64-apple-darwin",
        _ => anyhow::bail!("Unsupported platform"),
    }
}
```

### 4.3 Cross-Platform Considerations

- **File Locking:** Uses `fs2` crate for cross-platform file locking (flock on Unix, Windows alternative)
- **Path Handling:** Uses standard Rust `Path` and `PathBuf` for cross-platform path operations
- **Process Execution:** Uses `std::process::Command` for cross-platform process spawning

---

## 5. System-Level Dependencies

### 5.1 Required System Utilities

| Utility | Purpose | Platform |
|---------|---------|----------|
| **bash** | Shell execution for agent dispatch | Linux/macOS |
| **sh** | Fallback shell | Linux/macOS |
| **git** | Version control operations | All platforms |
| **rm** | File operations | Linux/macOS |
| **mkdir** | Directory creation | Linux/macOS |
| **cat** | File reading | Linux/macOS |
| **ps** | Process listing | Linux/macOS (Unix-specific) |

### 5.2 Optional System Utilities

| Utility | Purpose | Required For |
|---------|---------|--------------|
| **sqlite3** | Direct database inspection | Debugging bead stores |
| **kubectl** | Kubernetes cluster access | CI/CD workflows |
| **docker** | Container operations | Integration tests |

### 5.3 No External System Library Requirements

**Important:** Pluck has NO external system library dependencies because:

1. **rusqlite uses bundled SQLite** - no need for libsqlite3-dev
2. **All dependencies are pure Rust** - no C/C++ library dependencies
3. **Static linking** - release builds are fully self-contained

---

## 6. Build Dependencies and Requirements

### 6.1 Disk Space Requirements

| Build Type | Minimum Space | Recommended Space |
|------------|---------------|-------------------|
| **Debug build** | 10 GB | 20 GB |
| **Release build** | 20 GB | 50 GB |
| **Full dependency build** | 50 GB | 100 GB |

**Note:** Rust target/ directories can grow to 80-100GB+. See `/home/coding/CLAUDE.md` for disk space management.

### 6.2 Memory Requirements

| Build Type | Minimum RAM | Recommended RAM |
|------------|-------------|-----------------|
| **Debug build** | 4 GB | 8 GB |
| **Release build** | 8 GB | 16 GB |
| **Parallel builds** | 16 GB | 32 GB |

### 6.3 CPU Requirements

- **Minimum:** 2 cores (single-threaded build)
- **Recommended:** 4+ cores (parallel builds)
- **Optimal:** 8+ cores for maximum parallelization

---

## 7. Feature Flags

### 7.1 NEEDLE Features

| Feature | Description | Dependencies |
|---------|-------------|--------------|
| `otlp` | OpenTelemetry OTLP export (default) | opentelemetry, tonic, etc. |
| `integration` | Integration test support | testcontainers |

### 7.2 Build Commands

```bash
# Build with default features (includes otlp)
cargo build --release

# Build without otlp
cargo build --release --no-default-features

# Build with integration tests
cargo build --release --features integration
```

---

## 8. Development Environment Setup

### 8.1 Install Rust Toolchain

```bash
# Install Rust via rustup
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh

# Install specific version matching MSRV
rustup toolchain install stable
rustup default stable

# Add components
rustup component add rustfmt clippy
```

### 8.2 Install bead-forge (br CLI)

```bash
# Option 1: Build from source
cd /home/coding/bead-forge
cargo install --path .

# Option 2: Download release binary
curl -fsSL https://github.com/jedarden/bead-forge/releases/latest/download/install.sh | bash
```

### 8.3 Install NEEDLE

```bash
# Option 1: Build from source
cd /home/coding/NEEDLE
cargo install --path .

# Option 2: Download release binary
curl -fsSL https://github.com/jedarden/NEEDLE/releases/latest/download/install.sh | bash
```

---

## 9. Runtime Requirements

### 9.1 Environment Variables

| Variable | Purpose | Default | Example |
|----------|---------|---------|---------|
| `RUST_LOG` | Logging level | info | needle::strand::pluck=debug |
| `RUST_BACKTRACE` | Backtrace on panic | 0 | 1 |
| `NEEDLE_CONFIG` | Config file path | None | /path/to/config.yaml |

### 9.2 File System Requirements

| Location | Purpose | Required Space |
|----------|---------|-----------------|
| `.beads/` | Bead store database | Variable (1-100 MB per workspace) |
| `logs/` | Log files | Variable (1-10 MB per run) |
| `target/` | Build artifacts | 20-100 GB |

### 9.3 Network Requirements

| Purpose | Required | Protocol |
|---------|----------|----------|
| **Agent API calls** | Yes | HTTPS |
| **Self-update** | Optional | HTTPS |
| **OpenTelemetry** | Optional | HTTP/2 (gRPC) |

---

## 10. Security Considerations

### 10.1 Dependency Security

```bash
# Check for security advisories in Rust dependencies
cd /home/coding/NEEDLE
cargo audit

# Install cargo-audit if not present
cargo install cargo-audit
```

### 10.2 Known Security Considerations

- **HTTP Client:** Uses ureq for self-update (simple HTTP, verify TLS certificates)
- **Process Management:** Executes external agent CLIs via bash -c
- **File Operations:** Uses SQLite with flock for coordination
- **Credential Storage:** Follows standard AWS SDK/GCP SDK credential chains

---

## 11. Troubleshooting

### 11.1 Build Failures

```bash
# Clean build if dependency cache is corrupted
cd /home/coding/NEEDLE
cargo clean
cargo build --release
```

### 11.2 Dependency Conflicts

```bash
# Update Cargo.lock and resolve conflicts
cd /home/coding/NEEDLE
cargo update
```

### 11.3 Disk Space Issues

```bash
# Check disk space before large builds
df -BG --output=avail /
du -sh ~/NEEDLE/target
du -sh ~/bead-forge/target
```

---

## 12. Minimum Version Requirements Summary

### 12.1 Core Requirements

| Component | Minimum Version | Rationale |
|-----------|-----------------|-----------|
| **Rust** | 1.75 | MSRV (Minimum Supported Rust Version) |
| **Cargo** | 1.75 | Matches Rust version |
| **SQLite** | bundled | rusqlite includes bundled SQLite |
| **bash** | any | Shell execution for agent dispatch |
| **git** | any | Version control operations |

### 12.2 Platform Requirements

| Platform | Minimum OS Version | Notes |
|----------|-------------------|-------|
| **Linux** | Any modern distribution | Tested on Ubuntu 20.04+, Debian 11+ |
| **macOS** | 10.15+ (Catalina) | Apple Silicon (M1/M2) supported |
| **Windows** | 10+ | Limited support, Unix features not available |

---

## 13. Dependency Management

### 13.1 Update Dependencies

```bash
# Update NEEDLE dependencies
cd /home/coding/NEEDLE
cargo update
cargo build --release

# Update bead-forge dependencies
cd /home/coding/bead-forge
cargo update
cargo build --release
```

### 13.2 Add New Dependency

```bash
# Add to NEEDLE
cd /home/coding/NEEDLE
cargo add dependency_name

# Add to bead-forge
cd /home/coding/bead-forge
cargo add dependency_name
```

---

## 14. Version Upgrade Notes

### 14.1 NEEDLE 0.2.x → Future Versions

**Breaking Changes Expected:**
- MSRV may increase to 1.80+
- OpenTelemetry dependencies may upgrade to 0.32+
- tokio dependency may require newer async features

**Migration Path:**
1. Update rust-toolchain.toml if MSRV increases
2. Run `cargo update`
3. Test with `--features integration` before production deployment

### 14.2 bead-forge 0.2.0 → Future Versions

**Version Policies:**
- Follow Rust MSRV policy
- Maintain bundled SQLite for consistency
- Always run `cargo clippy` before releases

---

## 15. Documentation References

### 15.1 Internal Documentation

- **NEEDLE README:** `/home/coding/NEEDLE/README.md`
- **NEEDLE CLAUDE.md:** `/home/coding/NEEDLE/CLAUDE.md`
- **Pluck Configuration:** `/home/coding/ARMOR/pluck-config.yaml`
- **Pluck Debug Summary:** `/home/coding/ARMOR/pluck-debug-summary.md`
- **Version Inventory:** `/home/coding/ARMOR/pluck-version-inventory.md`

### 15.2 External Resources

- **Rust MSRV Policy:** https://rust-lang.github.io/rfcs/2495-min-rust-version.html
- **Cargo Documentation:** https://doc.rust-lang.org/cargo/
- **OpenTelemetry Rust:** https://opentelemetry.io/docs/instrumentation/rust/
- **rusqlite Documentation:** https://docs.rs/rusqlite/

---

## 16. Change History

| Date | Version | Changes |
|------|---------|---------|
| 2026-07-12 | 1.0 | Initial comprehensive system dependency documentation |

---

## 17. Maintenance Schedule

### 17.1 Regular Maintenance Tasks

| Frequency | Task | Purpose |
|-----------|------|---------|
| **Monthly** | Run `cargo audit` | Check for security advisories |
| **Monthly** | Run `cargo update` | Keep dependencies current |
| **Quarterly** | Review and update this document | Ensure accuracy |
| **As Needed** | Update after major version bumps | Document breaking changes |

### 17.2 Contact Information

- **NEEDLE Repository:** https://github.com/jedarden/NEEDLE
- **bead-forge Repository:** https://github.com/jedarden/bead-forge
- **ARMOR Repository:** https://github.com/jedarden/ARMOR
- **Issues:** https://github.com/jedarden/NEEDLE/issues

---

**Document Status:** ✅ Complete  
**Last Updated:** 2026-07-12  
**Next Review:** 2026-10-12 (Quarterly)  
**Bead:** bf-49y71  
**Workspace:** /home/coding/ARMOR