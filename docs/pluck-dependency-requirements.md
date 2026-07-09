# Pluck (NEEDLE) Dependency Requirements

## Overview

**Pluck** is the primary strand within the NEEDLE system. It processes beads from the assigned workspace and is the first step in NEEDLE's strand escalation sequence. Pluck is not a standalone tool, but a core component of the NEEDLE worker framework.

**Repository:** https://github.com/jedarden/NEEDLE  
**Current Version:** 0.2.11  
**Primary Language:** Rust (Edition 2021)  
**Minimum Rust Version:** 1.75+

## What is Pluck?

Pluck is a "strand" - a processing mode within NEEDLE that:
- Processes beads from the assigned workspace
- Dispatches work to configured headless CLI agents (Claude Code, OpenCode, Codex, Aider)
- Handles every outcome through explicit, predefined paths
- Coordinates with other strands (Mend, Explore, Weave, Unravel, Pulse, Reflect, Splice, Knot)

### Strand Escalation Sequence

1. 🪡 **Pluck** - Process beads from assigned workspace
2. 🔧 **Mend** - Cleanup: orphaned claims, stale locks, health checks  
3. 🔭 **Explore** - Search other workspaces for claimable beads
4. 🕸️ **Weave** - Create beads from documentation gaps *(opt-in)*
5. 🪢 **Unravel** - Propose alternatives for HUMAN-blocked beads *(opt-in)*
6. 💓 **Pulse** - Codebase health scans, auto-generate beads *(opt-in)*
7. 🪞 **Reflect** - Consolidate learnings from recent beads *(opt-in)*
8. 🪡 **Splice** - Document worker failures, create alert beads
9. 🪢 **Knot** - All strands exhausted — alert human, wait

## Core Dependencies

### Rust Toolchain Requirements

| Component | Minimum Version | Notes |
|-----------|----------------|-------|
| **Rust** | 1.75+ | Required for compilation |
| **Cargo** | (comes with Rust) | Build system and package manager |
| **rustfmt** | (any) | Code formatting (dev requirement) |
| **clippy** | (any) | Rust linter (dev requirement) |

### Compilation Targets

| Target | Platform | Usage |
|--------|----------|-------|
| `x86_64-unknown-linux-gnu` | Linux x86_64 | Standard Linux builds |
| `x86_64-unknown-linux-musl` | Linux x86_64 | Static linking (for release binaries) |
| `aarch64-apple-darwin` | macOS ARM64 | Apple Silicon Macs |

## Cargo.toml Dependencies

### Runtime Dependencies

| Dependency | Version | Purpose |
|------------|---------|---------|
| **tokio** | 1 | Async runtime (full features) |
| **serde** | 1 | Serialization framework (with derive) |
| **serde_json** | 1 | JSON serialization |
| **serde_yaml** | 0.9 | YAML serialization |
| **clap** | 4 | CLI argument parsing (with derive) |
| **anyhow** | 1 | Error handling |
| **thiserror** | 1 | Error derive macros |
| **tracing** | 0.1 | Structured logging |
| **tracing-subscriber** | 0.3 | Log subscribers (env-filter, json) |
| **chrono** | 0.4 | Time handling (with serde) |
| **which** | 4 | Executable detection |
| **async-trait** | 0.1 | Async trait support |
| **fs2** | 0.4 | Cross-platform file locking |
| **sha2** | 0.10 | Hashing algorithms |
| **hex** | 0.4 | Hex encoding/decoding |
| **regex** | 1 | Regular expressions |
| **glob** | 0.3 | Glob pattern matching |
| **ureq** | 2 | HTTP client (for self-update) |
| **aho-corasick** | 1 | Multi-pattern string search |
| **cfg-if** | 1 | Conditional compilation |
| **atty** | 0.2 | Terminal detection |
| **toml** | 0.8 | TOML parsing |
| **libc** | 0.2 | Unix process handling |
| **rand** | 0.8 | Random jitter generation |
| **futures** | 0.3 | Async utilities |
| **gethostname** | 0.4 | Hostname detection |

### OpenTelemetry Dependencies (Optional - behind `otlp` feature)

| Dependency | Version | Purpose |
|------------|---------|---------|
| **opentelemetry** | 0.31 | OpenTelemetry API |
| **opentelemetry_sdk** | 0.31 | OpenTelemetry SDK (rt-tokio) |
| **opentelemetry-otlp** | 0.31 | OTLP exporter (grpc-tonic, http-proto) |
| **opentelemetry-semantic-conventions** | 0.31 | Semantic conventions |
| **tonic** | 0.14 | gRPC library for OTLP |
| **tracing-opentelemetry** | 0.32 | Tracing bridge |

### Development Dependencies

| Dependency | Version | Purpose |
|------------|---------|---------|
| **tokio-test** | 0.4 | Tokio testing utilities |
| **tempfile** | 3 | Temporary file handling |
| **proptest** | 1 | Property testing |
| **filetime** | 0.2 | File time manipulation |
| **criterion** | 0.5 | Benchmarking |

### Integration Test Dependencies (Optional - behind `integration` feature)

| Dependency | Version | Purpose |
|------------|---------|---------|
| **testcontainers** | 0.23 | Docker containers for testing |

## Development Tools Requirements

### Build Tools

| Tool | Purpose | Installation |
|------|---------|--------------|
| **cargo** | Build and package management | Included with Rust |
| **rustc** | Rust compiler | Included with Rust |
| **strip** | Binary stripping (release builds) | System package manager |

### Optional Development Tools

| Tool | Purpose | Used For |
|------|---------|----------|
| **rustfmt** | Code formatting | Development workflow |
| **clippy** | Linting | Code quality checks |
| **cargo-expand** | Macro expansion | Debugging derive macros |
| **cargo-watch** | Watch mode | Development workflow |

## System Requirements

### Linux (Development)

| Component | Requirements |
|-----------|--------------|
| **OS** | Any modern Linux distribution |
| **Arch** | x86_64 (amd64) or aarch64 (ARM64) |
| **Memory** | 2GB+ recommended for compilation |
| **Disk** | 500MB+ for debug builds, 100MB+ for release |

### Linux (Static Release Builds)

| Component | Requirements |
|-----------|--------------|
| ** musl-tools** | `sudo apt-get install musl-tools` (Ubuntu/Debian) |
| **Target** | `x86_64-unknown-linux-musl` |

### macOS (Development)

| Component | Requirements |
|-----------|--------------|
| **OS** | macOS 10.15+ (Catalina or later) |
| **Arch** | aarch64 (Apple Silicon) or x86_64 (Intel) |
| **Xcode** | Command Line Tools for Xcode |
| **Memory** | 2GB+ recommended for compilation |

### Runtime Requirements

| Component | Requirements |
|-----------|--------------|
| **OS** | Linux, macOS, or any Unix-like system |
| **Arch** | x86_64 or aarch64 |
| **Memory** | Minimal (typically <50MB per worker) |
| **Disk** | ~10MB for binary |

## Installation Methods

### Method 1: Pre-built Binary (Recommended)

```bash
curl -fsSL https://github.com/jedarden/NEEDLE/releases/latest/download/install.sh | bash
```

**Requirements:** 
- `curl` or `wget`
- `sha256sum` or `shasum` (for verification, optional)
- `gpg` (for signature verification, optional)

### Method 2: Build from Source

```bash
cargo install --git https://github.com/jedarden/NEEDLE
```

**Requirements:**
- Rust 1.75+
- Cargo
- Git

### Method 3: Development Clone

```bash
git clone https://github.com/jedarden/NEEDLE
cd NEEDLE
cargo build --release
```

**Requirements:**
- Rust 1.75+
- Cargo
- Git

## Feature Flags

| Feature | Dependencies | Description |
|---------|--------------|-------------|
| **default** | otlp | Enables OpenTelemetry support by default |
| **otlp** | opentelemetry-* | Enables OTLP telemetry export |
| **integration** | otlp + testcontainers | Enables integration tests |

## Agent Adapter Requirements

Pluck requires agent adapters to dispatch work. Each adapter may have its own requirements:

| Agent | CLI Requirement | Notes |
|-------|----------------|-------|
| **Claude Code (interactive)** | `claude` CLI + `claude-interactive` plugin | Subscription billing |
| **Claude Code (API)** | `claude --print` | Programmatic/API billing |
| **OpenCode** | `opencode` binary | User-provided |
| **Codex CLI** | `codex` binary | User-provided |
| **Aider** | `aider --message` | User-provided |
| **Custom** | Any CLI | Configured via YAML adapter |

## Configuration Requirements

### Workspace Requirements

Pluck requires a workspace with a bead store:

| Component | Location | Description |
|-----------|----------|-------------|
| **Bead Store** | `.beads/beads.db` | SQLite database |
| **Checkpoint** | `.beads/issues.jsonl` | JSONL checkpoint file |
| **Config** | `.needle.yaml` | Optional worker configuration |

### Environment Variables

| Variable | Purpose | Required |
|----------|---------|----------|
| `RUST_LOG` | Logging control | No |
| `NEEDLE_INSTALL_PATH` | Installation path | No |

## Verification

### Verify Installation

```bash
needle --version
needle --help
```

### Verify Dependencies

```bash
# Check Rust installation
rustc --version
cargo --version

# Check compilation
cargo build --release

# Run tests
cargo test

# Run linter
cargo clippy --all-targets -- -D warnings

# Check formatting
cargo fmt --check
```

## Version Compatibility

### Rust Edition

- **Rust Edition:** 2021
- **Minimum rust-version:** 1.75
- **Recommended:** Stable or latest stable

### Dependency Pinning

The project uses `Cargo.lock` for reproducible builds. All dependency versions are locked at commit time.

## Troubleshooting

### Common Issues

| Issue | Solution |
|-------|----------|
| **Rust too old** | Update via `rustup update stable` |
| **Missing musl-tools** | `sudo apt-get install musl-tools` (Ubuntu) |
| **Permission denied** | Add `~/.local/bin` to PATH |
| **Binary not executable** | `chmod +x needle` |

## Related Documentation

- **NEEDLE Repository:** https://github.com/jedarden/NEEDLE
- **Strand System:** See NEEDLE README section "🧶 Strand Escalation"
- **Agent Configuration:** See NEEDLE `docs/examples/`
- **OpenTelemetry:** See NEEDLE `docs/plan/plan.md`

## Summary

Pluck is a core strand within the NEEDLE system and requires:

- **Rust 1.75+** for compilation
- **Standard Cargo dependencies** for async runtime, serialization, CLI parsing, and telemetry
- **Optional OpenTelemetry support** for observability
- **Agent CLI** for dispatching work (Claude Code, OpenCode, etc.)
- **Bead store** (SQLite + JSONL) in workspace
- **Minimal runtime resources** (<50MB RAM per worker)

For most users, the recommended installation is via the pre-built binary installer, which requires only `curl` or `wget`.