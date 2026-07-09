# Pluck Dependency Requirements

## Project Information

**Pluck** is the primary strand in the NEEDLE system, a universal wrapper for headless coding CLI agents. NEEDLE processes bead queues deterministically and dispatches work to various agent CLIs.

- **Repository**: [jedarden/NEEDLE](https://github.com/jedarden/NEEDLE)
- **Documentation Source**: `/home/coding/NEEDLE/`
- **Reference Date**: 2026-07-09
- **Current Version**: 0.2.11

---

## Minimum Required Versions

### Core Platform Requirements

| Component | Minimum Version | Notes |
|-----------|----------------|-------|
| **Rust** | 1.75+ | Specified in Cargo.toml as `rust-version = "1.75"` |
| **Rust Edition** | 2021 | Modern Rust edition with updated idioms |
| **Channel** | Stable | Production deployments use stable channel |

### Required Toolchain Components

The following Rust toolchain components are required:

- **rustfmt** - Code formatting
- **clippy** - Linting and code quality checks

### Supported Target Platforms

- `x86_64-unknown-linux-gnu` - Primary Linux target
- `aarch64-apple-darwin` - macOS ARM (Apple Silicon)

---

## Runtime Dependencies

### Async Runtime & Concurrency

| Dependency | Min Version | Features | Purpose |
|------------|-------------|----------|---------|
| **tokio** | 1.x | full | Async runtime, task scheduling |

### Serialization & Data Formats

| Dependency | Min Version | Features | Purpose |
|------------|-------------|----------|---------|
| **serde** | 1.x | derive | Serialization framework |
| **serde_json** | 1.x | - | JSON serialization |
| **serde_yaml** | 0.9.x | - | YAML configuration parsing |

### Command-Line Interface

| Dependency | Min Version | Features | Purpose |
|------------|-------------|----------|---------|
| **clap** | 4.x | derive | CLI argument parsing |

### Error Handling

| Dependency | Min Version | Purpose |
|------------|-------------|---------|
| **anyhow** | 1.x | Generic error context |
| **thiserror** | 1.x | Error type derivation |

### Logging & Telemetry

| Dependency | Min Version | Features | Purpose |
|------------|-------------|----------|---------|
| **tracing** | 0.1.x | - | Structured instrumentation |
| **tracing-subscriber** | 0.3.x | env-filter, json | Log formatting & filtering |

**Optional OTLP Telemetry** (gated behind `otlp` feature):

| Dependency | Min Version | Features | Purpose |
|------------|-------------|----------|---------|
| **opentelemetry** | 0.31.x | - | OpenTelemetry SDK |
| **opentelemetry_sdk** | 0.31.x | rt-tokio | OTLP with Tokio runtime |
| **opentelemetry-otlp** | 0.31.x | grpc-tonic, http-proto | OTLP export |
| **opentelemetry-semantic-conventions** | 0.31.x | - | Standard semantic conventions |
| **tonic** | 0.14.x | - | gRPC transport |
| **tracing-opentelemetry** | 0.32.x | - | Tracing bridge |

### Time & Date

| Dependency | Min Version | Features | Purpose |
|------------|-------------|----------|---------|
| **chrono** | 0.4.x | serde | Date/time operations |

### Process & System Operations

| Dependency | Min Version | Purpose |
|------------|-------------|---------|
| **which** | 4.x | Executable discovery |
| **libc** | 0.2.x | Unix system calls (PID checks) |
| **gethostname** | 0.4.x | Hostname detection |

### File Operations

| Dependency | Min Version | Purpose |
|------------|-------------|---------|
| **fs2** | 0.4.x | Cross-platform file locking (flock) |
| **glob** | 0.3.x | Pattern matching (file discovery) |
| **cfg-if** | 1.x | Conditional compilation |

### Cryptographic & Hashing

| Dependency | Min Version | Purpose |
|------------|-------------|---------|
| **sha2** | 0.10.x | Content hashing |
| **hex** | 0.4.x | Hex encoding |

### Text Processing

| Dependency | Min Version | Purpose |
|------------|-------------|---------|
| **regex** | 1.x | Regular expressions (token extraction) |
| **aho-corasick** | 1.x | Multi-pattern string search (sanitization) |

### Network & HTTP

| Dependency | Min Version | Purpose |
|------------|-------------|---------|
| **ureq** | 2.x | HTTP client (self-update) |

### Utilities

| Dependency | Min Version | Purpose |
|------------|-------------|---------|
| **async-trait** | 0.1.x | Async trait support |
| **atty** | 0.2.x | Terminal detection (ANSI colors) |
| **toml** | 0.8.x | TOML parsing (gitleaks config) |
| **rand** | 0.8.x | Random jitter (backoff desynchronization) |
| **futures** | 0.3.x | Async utilities |

---

## Development Dependencies

### Testing Framework

| Dependency | Min Version | Purpose |
|------------|-------------|---------|
| **tokio-test** | 0.4.x | Tokio testing utilities |
| **tempfile** | 3.x | Temporary file handling in tests |
| **proptest** | 1.x | Property-based testing |
| **filetime** | 0.2.x | File timestamp manipulation |

### Benchmarking

| Dependency | Min Version | Purpose |
|------------|-------------|---------|
| **criterion** | 0.5.x | Performance benchmarking |

### Integration Testing (Optional)

| Dependency | Min Version | Purpose |
|------------|-------------|---------|
| **testcontainers** | 0.23.x | Container-based integration tests |

---

## Feature Flags

### Default Features
- `otlp` - OpenTelemetry telemetry export enabled by default

### Optional Features

| Feature | Dependencies | Description |
|---------|--------------|-------------|
| **otlp** | opentelemetry*, tonic*, tracing-opentelemetry* | Enables OTLP telemetry export |
| **integration** | otlp + testcontainers | Integration testing with containers |

---

## Build & Development Requirements

### Minimum Build Tools

- **Rust Stable**: 1.75 or later
- **Cargo**: Bundled with Rust toolchain
- **rustfmt**: Code formatting
- **clippy**: Linting

### Build Commands

```bash
# Standard build
cargo build --release

# Run tests
cargo test

# Lint
cargo clippy --all-targets -- -D warnings

# Format check
cargo fmt --check
```

### Cross-Compilation Support

The project supports cross-compilation for:
- **macOS ARM** (`aarch64-apple-darwin`)

---

## Installation Methods

### From Source

```bash
# Install via Cargo
cargo install --git https://github.com/jedarden/NEEDLE

# Or build locally
git clone https://github.com/jedarden/NEEDLE
cd NEEDLE
cargo build --release
```

### Pre-built Binaries

```bash
curl -fsSL https://github.com/jedarden/NEEDLE/releases/latest/download/install.sh | bash
```

---

## Claude-Interactive Plugin Requirements

The `claude-interactive` plugin (for subscription billing) has additional requirements:

| Requirement | Minimum Version | Purpose |
|-------------|----------------|---------|
| **Python** | 3.10+ | Plugin runtime |
| **pyte** | (via pip) | PTY emulation |
| **claude CLI** | (on PATH) | Claude Code CLI |

**Installation**:
```bash
pip install pyte
gh release download --repo jedarden/NEEDLE --pattern 'claude-interactive*'
```

---

## System Requirements

### Runtime

- **Operating System**: Linux (primary), macOS (supported via cross-compile)
- **Architecture**: x86_64, ARM64 (aarch64)
- **Memory**: No specific minimum (Rust runtime is efficient)
- **Disk**: ~100MB for release build (stripped, LTO-enabled)

### Development

- **Git**: For source management
- **Cargo**: For dependency management and building
- **Rust Stable 1.75+**: Minimum compiler version

---

## CI/CD Requirements

The CI pipeline uses:
- **Ubuntu latest** for builds
- **dtolnay/rust-toolchain** GitHub Action
- **Cargo caching** for faster builds

---

## Dependency Locking

The project uses `Cargo.lock` for reproducible builds. All dependencies are locked to specific versions that have been tested together.

**Lock File**: `/home/coding/NEEDLE/Cargo.lock`

---

## Security Considerations

### Dependencies with Security Implications

1. **ureq** (HTTP client) - Used for self-update functionality
2. **serde_json/serde_yaml** - Data deserialization
3. **libc** - Direct system calls
4. **regex** - Pattern matching (DoS potential with malicious patterns)

### Build Security

- Release builds use `strip = true` to remove debug symbols
- LTO (Link-Time Optimization) enabled for performance
- Single codegen unit for better optimization

---

## Documentation References

- **Source Repository**: [jedarden/NEEDLE](https://github.com/jedarden/NEEDLE)
- **Main Documentation**: `/home/coding/NEEDLE/README.md`
- **Dependency Manifest**: `/home/coding/NEEDLE/Cargo.toml`
- **Lock File**: `/home/coding/NEEDLE/Cargo.lock`
- **Toolchain Config**: `/home/coding/NEEDLE/rust-toolchain.toml`

---

## Version Compatibility Matrix

| NEEDLE Version | Min Rust Version | Status |
|----------------|-------------------|--------|
| 0.2.11 | 1.75 | Current |
| Future | TBD | Maintained |

---

## Notes

- Pluck is strand #1 in the NEEDLE system's strand escalation sequence
- All version requirements are minimums - later versions may work
- The project uses semantic versioning for releases
- OTLP telemetry is enabled by default but can be disabled
- Cross-platform support focuses on Linux x86_64 as primary target

---

*Document generated for bead BF-1FYJU - Pluck dependency requirements gathering*
*Generated: 2026-07-09*
*Source: /home/coding/NEEDLE repository analysis*
