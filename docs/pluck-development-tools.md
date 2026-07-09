# Pluck Development Tools - Comprehensive Reference

**Document Created:** 2026-07-09  
**Bead:** bf-4q2s0  
**Workspace:** /home/coding/ARMOR  
**Status:** ✅ Complete

## Overview

This document provides a comprehensive reference for all development tools used across the Pluck ecosystem. Pluck is a strand within the NEEDLE system that processes beads from assigned workspaces. This reference covers:

- **Core runtime tools** (Rust, Go, br CLI)
- **Build and development tools** (compilers, package managers)
- **Testing frameworks** (unit, integration, benchmarking)
- **Code quality tools** (linters, formatters)
- **Dependency management tools** (Cargo, Go modules)
- **Development utilities** (debugging, monitoring, CLI tools)

---

## Table of Contents

1. [Core Runtime Tools](#core-runtime-tools)
2. [Build Tools and Compilers](#build-tools-and-compilers)
3. [Package Managers](#package-managers)
4. [Testing Frameworks](#testing-frameworks)
5. [Code Quality Tools](#code-quality-tools)
6. [Development Utilities](#development-utilities)
7. [Version Constraints Summary](#version-constraints-summary)
8. [Installation Quick Reference](#installation-quick-reference)
9. [Maintenance Schedule](#maintenance-schedule)

---

## Core Runtime Tools

### Rust Toolchain (NEEDLE/Pluck)

| Tool | Version | Purpose | Minimum Required |
|------|---------|---------|-------------------|
| **rustc** | 1.96.1 (2026-06-26) | Rust compiler | 1.75 (MSRV) |
| **cargo** | 1.96.1 (2026-06-26) | Package manager & build tool | 1.75 (implied) |
| **rustfmt** | 1.9.0-stable (2026-06-26) | Code formatter | Not specified |
| **clippy** | 0.1.96 (2026-06-26) | Linter | Not specified |

**Configuration File:** `rust-toolchain.toml`

```toml
[toolchain]
channel = "stable"
components = ["rustfmt", "clippy"]
targets = ["x86_64-unknown-linux-gnu", "aarch64-apple-darwin"]
```

**Constraints:**
- **MSRV:** 1.75 (Minimum Supported Rust Version)
- **Rust Edition:** 2021
- **Platform Support:** Linux (x86_64), macOS (aarch64-apple-darwin)

**Version Status:** ✅ **PASS** - Installed 1.96.1 exceeds MSRV 1.75 by +21 minor versions

---

### Go Toolchain (ARMOR Workspace)

| Tool | Version | Purpose | Minimum Required |
|------|---------|---------|-------------------|
| **go** | 1.25.0 linux/amd64 | Go compiler and toolchain | 1.25.0 |

**Constraints:**
- **Module Path:** github.com/jedarden/armor
- **Required Version:** 1.25.0 (exact)
- **Platform:** linux/amd64

**Version Status:** ✅ **PASS** - Exact match with requirement

---

### br CLI (Beads Rust)

| Tool | Version | Purpose | Minimum Required |
|------|---------|---------|-------------------|
| **br** | 0.2.0 (via bead-forge) | Bead store management CLI | 0.2.0 |

**Installation Path:** `~/.local/bin/br` (symlink to `bf` binary)

**Constraints:**
- **SQLite Support:** Embedded (static linking)
- **Database Format:** `.beads/beads.db`
- **Compatibility:** bead-forge 0.2.0 (br-compatible superset)

**Version Status:** ✅ **PASS** - Exact match with requirement

**Core Functionality:**
- Bead creation, listing, and management
- Atomic bead claiming via SQLite transactions
- Workspace coordination for multi-worker setups
- Integration with NEEDLE strand system

---

## Build Tools and Compilers

### Rust Build Tools

| Tool | Type | Purpose | Version |
|------|------|---------|---------|
| **cargo** | Build system | Package manager and build tool | 1.96.1 |
| **rustc** | Compiler | Rust compiler | 1.96.1 |
| **cargo build** | Command | Debug build | Via cargo |
| **cargo build --release** | Command | Release build (optimized) | Via cargo |
| **cargo clean** | Command | Clean build artifacts | Via cargo |

**Build Configuration:**
- **Target Directory:** `target/`
- **Debug Build:** `target/debug/`
- **Release Build:** `target/release/`
- **Build Features:** `otlp` (default), `integration` (optional)

**Build Commands:**
```bash
# Standard release build
cargo build --release

# Build without default features (no OpenTelemetry)
cargo build --release --no-default-features

# Build with integration test support
cargo build --release --features integration
```

---

### Go Build Tools

| Tool | Type | Purpose | Version |
|------|------|---------|---------|
| **go build** | Command | Compile Go packages | 1.25.0 |
| **go run** | Command | Build and run Go programs | 1.25.0 |
| **go install** | Command | Build and install binaries | 1.25.0 |

**Build Configuration:**
- **Module Path:** github.com/jedarden/armor
- **Output Directory:** `./` or `$GOBIN`
- **Build Tags:** None specified

---

## Package Managers

### Cargo (Rust)

| Tool | Version | Purpose |
|------|---------|---------|
| **cargo** | 1.96.1 | Rust package manager and build tool |

**Core Commands:**
```bash
cargo build              # Build project
cargo test               # Run tests
cargo add <dep>          # Add dependency
cargo update             # Update dependencies
cargo clean              # Clean build artifacts
cargo doc                # Generate documentation
```

**Configuration Files:**
- `Cargo.toml` - Project manifest and dependencies
- `Cargo.lock` - Locked dependency versions
- `.cargo/config.toml` - Cargo configuration (optional)

---

### Go Modules

| Tool | Version | Purpose |
|------|---------|---------|
| **go mod** | 1.25.0 | Go module management |

**Core Commands:**
```bash
go mod init      # Initialize module
go mod tidy      # Clean up dependencies
go get -u ./...  # Update all dependencies
go get <package> # Add specific dependency
```

**Configuration Files:**
- `go.mod` - Module definition and dependencies
- `go.sum` - Dependency checksums

---

## Testing Frameworks

### Rust Testing Tools

| Tool | Version | Purpose | Type |
|------|---------|---------|------|
| **tokio-test** | 0.4 | Tokio testing utilities | Dev dependency |
| **tempfile** | 3 | Temporary file handling | Dev dependency |
| **proptest** | 1 | Property-based testing | Dev dependency |
| **filetime** | 0.2 | File time manipulation (testing) | Dev dependency |
| **criterion** | 0.5 | Benchmarking framework | Dev dependency |
| **testcontainers** | 0.23 | Docker container integration testing | Optional (integration feature) |

**Testing Commands:**
```bash
# Run all tests
cargo test

# Run tests with output
cargo test -- --nocapture

# Run specific test
cargo test test_name

# Run benchmarks
cargo bench

# Run integration tests (requires feature)
cargo test --features integration
```

**Testing Configuration:**
- **Unit Tests:** `tests/` directory and inline `#[cfg(test)]` modules
- **Integration Tests:** `tests/` directory with `integration` feature
- **Benchmark Tests:** `benches/` directory (via criterion)

---

### Go Testing Tools

| Tool | Version | Purpose | Type |
|------|---------|---------|------|
| **go test** | 1.25.0 | Built-in testing command | Standard library |
| **testing** | 1.25.0 | Testing package | Standard library |
| **net/http/httptest** | 1.25.0 | HTTP testing utilities | Standard library |

**Testing Commands:**
```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests with verbose output
go test -v ./...

# Run specific package tests
go test ./path/to/package
```

---

## Code Quality Tools

### Linters

| Tool | Language | Version | Purpose |
|------|----------|---------|---------|
| **clippy** | Rust | 0.1.96 | Rust linter (via rustc component) |
| **cargo clippy** | Rust | Via cargo | Run clippy on project |
| **gofmt** | Go | 1.25.0 | Go code formatter (built-in) |
| **go vet** | Go | 1.25.0 | Go static analysis (built-in) |

**Linter Commands:**
```bash
# Rust linting
cargo clippy                    # Basic linting
cargo clippy -- -D warnings    # Treat warnings as errors

# Go linting
gofmt -l .                     # List files needing formatting
gofmt -w .                     # Format files in-place
go vet ./...                    # Static analysis
```

---

### Formatters

| Tool | Language | Version | Purpose |
|------|----------|---------|---------|
| **rustfmt** | Rust | 1.9.0-stable | Rust code formatter |
| **cargo fmt** | Rust | Via cargo | Run rustfmt on project |
| **gofmt** | Go | 1.25.0 | Go code formatter |

**Formatter Commands:**
```bash
# Rust formatting
cargo fmt                      # Format all code
cargo fmt --check              # Check formatting without modifying

# Go formatting
gofmt -w .                     # Format all files
gofmt -l .                     # List unformatted files
```

---

## Development Utilities

### CLI Tools

| Tool | Version | Purpose |
|------|---------|---------|
| **br** | 0.2.0 | Bead management CLI (via bead-forge) |
| **bf** | 0.2.0 | bead-forge binary (br-compatible superset) |
| **kubectl** | System | Kubernetes cluster management |
| **adb** | Platform-tools | Android Debug Bridge (Pixel 6 remote control) |

**br CLI Commands:**
```bash
br list                         # List all beads
br show <bead-id>             # Show bead details
br create <type>              # Create new bead
br claim <bead-id>            # Claim bead for work
br release <bead-id>          # Release bead
br close <bead-id>            # Mark bead as complete
br sync --flush-only          # Flush database to JSONL checkpoint
```

---

### Debugging Tools

| Tool | Version | Purpose |
|------|---------|---------|
| **lldb** | System | Rust debugger (on macOS) |
| **gdb** | System | Rust debugger (on Linux) |
| **dlv** | Latest | Go debugger (optional) |

**Debugging Commands:**
```bash
# Rust debugging
rust-lldb ./target/debug/binary_name
rust-gdb ./target/debug/binary_name

# Go debugging
dlv debug ./cmd/mycommand
```

---

### Monitoring and Observability

| Tool | Version | Purpose |
|------|---------|---------|
| **tracing** | 0.1 | Structured logging framework |
| **tracing-subscriber** | 0.3 | Log filtering and formatting |
| **tracing-opentelemetry** | 0.32 | OpenTelemetry integration |
| **opentelemetry** | 0.31 | OpenTelemetry API |
| **opentelemetry-otlp** | 0.31 | OTLP exporter |

**Observability Features:**
- **Default Feature:** `otlp` - OpenTelemetry OTLP export enabled by default
- **Optional Feature:** `integration` - Integration test support with testcontainers

---

## Version Constraints Summary

### Core Runtime Constraints

| Component | Minimum | Installed | Status | Gap |
|-----------|---------|-----------|--------|-----|
| **rustc** | 1.75 | 1.96.1 | ✅ PASS | +0.21.1 |
| **cargo** | 1.75 | 1.96.1 | ✅ PASS | +0.21.1 |
| **go** | 1.25.0 | 1.25.0 | ✅ PASS | 0.0 |
| **br CLI** | 0.2.0 | 0.2.0 | ✅ PASS | 0.0 |

**Overall Compliance:** ✅ **100%** - All core runtime tools meet or exceed minimum requirements

---

### Dependency Version Constraints

#### NEEDLE Core Rust Dependencies

| Dependency | Version | Constraints | Notes |
|------------|---------|-------------|-------|
| **tokio** | 1 | Full features | Async runtime |
| **serde** | 1 | With derive | Serialization |
| **serde_json** | 1 | Stable | JSON support |
| **serde_yaml** | 0.9 | Stable | YAML support |
| **clap** | 4 | With derive | CLI framework |
| **anyhow** | 1 | Stable | Error handling |
| **thiserror** | 1 | Stable | Error derivation |
| **tracing** | 0.1 | Stable | Logging |
| **tracing-subscriber** | 0.3 | With env-filter, json | Log formatting |
| **chrono** | 0.4 | With serde | Time handling |
| **regex** | 1 | Stable | Pattern matching |
| **glob** | 0.3 | Stable | File patterns |

**Constraint Type:** Semantic versioning (^1.0.0) - Allows minor and patch updates

---

#### ARMOR Go Dependencies

| Dependency | Version | Constraints | Notes |
|------------|---------|-------------|-------|
| **aws-sdk-go-v2** | v1.41.4 | AWS SDK core | AWS integration |
| **aws-sdk-go-v2/service/s3** | v1.97.2 | S3 service | S3 storage |
| **aws-sdk-go-v2/config** | v1.32.12 | AWS config | Configuration |
| **blazer** | v0.5.3 | GCS client | Google Cloud Storage |
| **golang.org/x/crypto** | v0.49.0 | Crypto extensions | Cryptography |
| **golang.org/x/sync** | v0.12.0 | Concurrency extensions | Concurrency |

**Constraint Type:** Semantic versioning with explicit pinning - Requires manual updates

---

### Platform Constraints

| Platform | Architecture | Support Status |
|----------|--------------|----------------|
| **Linux** | x86_64-unknown-linux-gnu | ✅ Primary target |
| **macOS** | aarch64-apple-darwin (Apple Silicon) | ✅ Supported target |
| **Windows** | Not configured | ❌ Not supported |

**Build Targets:**
- **Primary:** x86_64-unknown-linux-gnu
- **Secondary:** aarch64-apple-darwin

---

### Feature Flag Constraints

| Feature | Default | Dependencies | Purpose |
|---------|---------|--------------|---------|
| **otlp** | Yes | opentelemetry, tonic, etc. | OpenTelemetry export |
| **integration** | No | testcontainers | Integration testing |

**Build Constraints:**
- Default build includes OpenTelemetry (`otlp` feature)
- Integration tests require explicit `--features integration` flag
- Can disable default features with `--no-default-features`

---

## Installation Quick Reference

### Rust Toolchain Installation

```bash
# Install Rust using rustup (recommended)
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh

# Install specific version (if needed)
rustup install stable
rustup default stable

# Install additional components
rustup component add rustfmt clippy

# Verify installation
rustc --version
cargo --version
rustfmt --version
```

**Alternative Installation:**
```bash
# System package manager (Debian/Ubuntu)
sudo apt-get install rustc cargo rustfmt clippy

# Homebrew (macOS)
brew install rust
```

---

### Go Toolchain Installation

```bash
# Install Go 1.25.0
wget https://go.dev/dl/go1.25.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.25.0.linux-amd64.tar.gz

# Add to PATH (add to ~/.bashrc or ~/.zshrc)
export PATH=$PATH:/usr/local/go/bin

# Verify installation
go version
```

**Alternative Installation:**
```bash
# System package manager (Debian/Ubuntu)
sudo apt-get install golang-go

# Homebrew (macOS)
brew install go
```

---

### br CLI Installation

```bash
# Install bead-forge (br-compatible)
cd ~/bead-forge
cargo install --path .

# Verify installation
br --version
br list
```

**Configuration:**
```bash
# Initialize bead store (first time only)
br init

# Check bead store status
br doctor

# Repair bead store (if corrupted)
br sync --flush-only
br doctor --repair
```

---

### Development Tools Installation

```bash
# Rust development tools (via rustup)
rustup component add rustfmt clippy

# Go tools (included with Go installation)
# gofmt and go vet are built into go

# Additional Go tools (optional)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

---

## Maintenance Schedule

### Daily Tasks

- [ ] **Backup bead store:** `br sync --flush-only`
- [ ] **Check disk space:** `df -BG --output=avail /`

### Weekly Tasks

- [ ] **Update Rust dependencies:** `cd ~/NEEDLE && cargo update`
- [ ] **Update Go dependencies:** `cd ~/ARMOR && go get -u ./... && go mod tidy`
- [ ] **Run security audits:**
  ```bash
  cd ~/NEEDLE && cargo audit
  cd ~/ARMOR && go list -json -m all | nancy sleuth
  ```

### Monthly Tasks

- [ ] **Review version compliance:** Check for new MSRV announcements
- [ ] **Update rust-toolchain.toml:** If MSRV increases
- [ ] **Clean build artifacts:** `cargo clean` (if disk space < 20G)
- [ ] **Run full test suite:**
  ```bash
  cd ~/NEEDLE && cargo test --features integration
  cd ~/ARMOR && go test ./...
  ```

### Quarterly Tasks

- [ ] **Update this documentation:** Review and revise tool versions
- [ ] **Review feature flags:** Evaluate new feature additions
- [ ] **Platform testing:** Test on all supported platforms
- [ ] **Dependency health check:** Review deprecated or end-of-life dependencies

### As-Needed Tasks

- [ ] **Post-major-version upgrade:** Full regression testing
- [ ] **Pre-deployment:** Verify all constraint compliance
- [ ] **Disk space crisis:** Clear idle `target/` directories
- [ ] **Dependency issues:** Run `cargo doctor` or `go mod verify`

---

## Troubleshooting

### Build Failures

**Symptom:** Build fails with dependency errors

**Solutions:**
```bash
# Clean build
cd ~/NEEDLE && cargo clean && cargo build --release

# Update dependencies
cd ~/NEEDLE && cargo update

# Check for known issues
cargo doctor
```

---

### Version Conflicts

**Symptom:** Version mismatch errors

**Solutions:**
```bash
# Verify rust-toolchain.toml
cat rust-toolchain.toml

# Update Rust toolchain
rustup update stable

# Check Go version
go version
```

---

### Disk Space Issues

**Symptom:** Build fails with "No space left on device"

**Solutions:**
```bash
# Check disk space
df -BG --output=avail /

# Find largest target directories
du -sh ~/*/target 2>/dev/null | sort -rh

# Remove idle target directory
rm -rf ~/<idle-repo>/target
```

---

### br CLI Issues

**Symptom:** Bead store corruption

**Solutions:**
```bash
# Flush database to checkpoint
br sync --flush-only

# Check SQLite integrity
sqlite3 .beads/beads.db "PRAGMA integrity_check;"

# Repair bead store
br doctor --repair

# Last resort: full rebuild
rm .beads/beads.db
br sync --import
```

---

## Documentation References

### Internal Documentation

- **Pluck Version Inventory:** `/home/coding/ARMOR/pluck-version-inventory.md`
- **Pluck Version Gap Analysis:** `/home/coding/ARMOR/pluck-version-gap-analysis.md`
- **Pluck Configuration:** `/home/coding/ARMOR/pluck-config.yaml`
- **NEEDLE README:** `/home/coding/NEEDLE/README.md`
- **NEEDLE CLAUDE.md:** `/home/coding/NEEDLE/CLAUDE.md`
- **ARMOR go.mod:** `/home/coding/ARMOR/go.mod`
- **NEEDLE Cargo.toml:** `/home/coding/NEEDLE/Cargo.toml`

### External Resources

- **Rust MSRV Policy:** https://rust-lang.github.io/rfcs/2495-min-rust-version.html
- **AWS SDK v2 Documentation:** https://aws.github.io/aws-sdk-go-v2/docs/
- **OpenTelemetry Rust:** https://opentelemetry.io/docs/instrumentation/rust/
- **Rustup Book:** https://rust-lang.github.io/rustup/
- **Go Module Documentation:** https://golang.org/ref/mod

---

## Change History

| Date | Version | Changes |
|------|---------|---------|
| 2026-07-09 | 1.0 | Initial comprehensive tool documentation created |

---

## Maintenance Notes

### Document Status

**Status:** ✅ Complete  
**Last Updated:** 2026-07-09  
**Next Review:** 2026-10-09 (Quarterly)

### Document Ownership

- **Maintained By:** ARMOR workspace
- **Related Beads:**
  - bf-195e3: Identify and categorize Pluck development tools
  - bf-17el1: Document version constraints and requirements
  - bf-4q2s0: Compile comprehensive tool version documentation (this document)

### Contact Information

- **Repository:** https://github.com/jedarden/ARMOR
- **NEEDLE Repository:** https://github.com/jedarden/NEEDLE
- **Issues:** https://github.com/jedarden/ARMOR/issues

---

**Document Type:** Comprehensive Tool Reference  
**Format:** Markdown  
**Intended Audience:** Developers, DevOps, Maintainers  
**Classification:** Development Documentation
