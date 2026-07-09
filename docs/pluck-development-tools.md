# Pluck Development Tools - Version Inventory

**Bead:** bf-62zau  
**Last Updated:** 2026-07-09  
**NEEDLE Version:** 0.2.11

## Overview

This document captures the current versions of development tools used in the Pluck project. Pluck is a strand component within NEEDLE (a Rust project) and shares the same development toolchain as the parent project.

## Required Development Tools

### Rust Toolchain

| Tool | Current Version | Minimum Required | Purpose |
|------|-----------------|------------------|---------|
| **rustc** | 1.96.1 | 1.75.0 | Rust compiler |
| **cargo** | 1.96.1 | 1.75.0 | Package manager and build tool |
| **rustfmt** | 1.9.0-stable | N/A | Code formatting |
| **clippy** | 0.1.96 | N/A | Linting and code quality checks |

**Installation Status:** All tools are currently installed and available in PATH.

**Verification:**
```bash
rustc --version    # rustc 1.96.1 (31fca3adb 2026-06-26)
cargo --version    # cargo 1.96.1 (356927216 2026-06-26)
rustfmt --version  # rustfmt 1.9.0-stable (31fca3adb2 2026-06-26)
clippy-driver --version  # clippy 0.1.96 (31fca3adb2 2026-06-26)
```

### Toolchain Configuration

The project uses a `rust-toolchain.toml` file to ensure consistent toolchain versions across all development environments:

```toml
[toolchain]
channel = "stable"
components = ["rustfmt", "clippy"]
targets = ["x86_64-unknown-linux-gnu", "aarch64-apple-darwin"]
```

**Location:** `/home/coding/NEEDLE/rust-toolchain.toml`

### Build Targets

| Target Platform | Architecture | Purpose |
|-----------------|--------------|---------|
| **x86_64-unknown-linux-gnu** | x86_64 | Primary Linux build target |
| **aarch64-apple-darwin** | ARM64 | macOS ARM (Apple Silicon) cross-compilation |

## Project Minimum Version Requirements

### Rust Edition and MSRV

- **Rust Edition:** 2021
- **Minimum Supported Rust Version (MSRV):** 1.75.0

**Source:** `/home/coding/NEEDLE/Cargo.toml`
```toml
[package]
edition = "2021"
rust-version = "1.75"
```

### Minimum Version Justification

The MSRV of 1.75.0 is determined by:
1. **OpenTelemetry dependencies** require Rust 1.75.0+
2. **tonic** library requires Rust 1.70.0+  
3. **tracing-opentelemetry** requires Rust 1.65.0+

All other core dependencies have lower MSRV requirements but are superseded by NEEDLE's 1.75 baseline.

## Development Workflow Tools

### Local Development Commands

| Command | Purpose |
|---------|---------|
| `cargo build` | Build the project |
| `cargo test` | Run unit and integration tests |
| `cargo clippy --all-targets -- -D warnings` | Lint with Clippy (warnings treated as errors) |
| `cargo fmt --check` | Check code formatting |
| `cargo build --target aarch64-apple-darwin` | Cross-compile for macOS ARM |

### CI/CD Pipeline Tools

The project uses GitHub Actions for continuous integration with the following tools:

| Tool | Version | Purpose |
|------|---------|---------|
| **actions/checkout** | v4 | Checkout repository code |
| **actions/cache** | v4 | Cache Cargo registry and build artifacts |
| **dtolnay/rust-toolchain** | master | Install Rust toolchain with components |

**Workflow Location:** `/home/coding/NEEDLE/.github/workflows/ci.yml`

### Cross-Compilation Requirements

For cross-compilation to macOS ARM (aarch64-apple-darwin):
- **Toolchain:** Rust stable with aarch64-apple-darwin target
- **Process:** Native cross-compilation from Linux (no macOS required)

## Development Environment

### Current System Information

- **OS:** Linux (kernel 6.12.63)
- **Platform:** linux/amd64
- **Shell:** GNU bash 5.2.37(1)-release

### Installation Location

Development tools are installed in:
- **Rust toolchain:** `~/.cargo/bin/` (via rustup)
- **Configuration:** `~/.rustup/` (toolchain management)

## Version Compatibility

### Toolchain Compatibility Matrix

| Tool Version | NEEDLE 0.2.11 | Notes |
|--------------|---------------|-------|
| Rust 1.75.0+ | ✅ Compatible | Minimum required version |
| Rust 1.96.1 | ✅ Compatible | Current production version |

### Known Version Constraints

1. **OpenTelemetry dependencies** require Rust 1.75.0 or higher
2. **All components** in rust-toolchain.toml (rustfmt, clippy) are compatible with the stable channel
3. **Cross-compilation** targets are limited to Linux (x86_64) and macOS ARM (aarch64)

## Related Documentation

- **[Pluck Dependencies - Minimum Version Requirements](pluck-dependencies.md)** - Detailed Rust crate dependency information
- **[NEEDLE Repository](https://github.com/jedarden/needle)** - Source code and issue tracking

## Version Upgrade Guidelines

### When to Upgrade

1. **Security updates:** Upgrade immediately when Rust toolchain security vulnerabilities are announced
2. **Dependency MSRV bumps:** When core dependencies increase their MSRV above current minimum
3. **Feature requirements:** When new Rust features are needed for development

### Upgrade Process

1. Update `rust-toolchain.toml` if pinning to a specific version
2. Update `rust-version` in `Cargo.toml` if raising MSRV
3. Run `cargo update` to refresh dependency locks
4. Run full test suite: `cargo test --all-targets`
5. Verify cross-compilation: `cargo build --target aarch64-apple-darwin`

## Summary

**Key Takeaways:**
- **Current Rust version:** 1.96.1 (stable channel)
- **Minimum required:** 1.75.0 (set by OpenTelemetry dependencies)
- **Required components:** rustc, cargo, rustfmt, clippy
- **Build targets:** x86_64-unknown-linux-gnu (primary), aarch64-apple-darwin (cross-compile)
- **CI toolchain:** Matches local development via rust-toolchain.toml

---

*This document was created as part of bead bf-62zau to capture development tool versions used in the Pluck project.*
