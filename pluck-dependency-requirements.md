# Pluck Dependency Requirements

## Context

**Pluck** is a strand within the NEEDLE system (Navigates Every Enqueued Deliverable, Logs Effort). It handles the primary bead selection from assigned workspaces, processing >90% of all bead operations.

**Note:** Pluck is NOT a standalone project - it is a component of NEEDLE. The dependencies listed below are for the full NEEDLE system, which includes the Pluck strand.

---

## Core Requirements

### Rust Toolchain

**Minimum Supported Rust Version (MSRV):** 1.75+ (2023-12-28)

**Required Components:**
- `rustfmt` - Code formatting
- `clippy` - Linting and code quality checks

**Toolchain Configuration:**
```toml
[toolchain]
channel = "stable"
components = ["rustfmt", "clippy"]
targets = ["x86_64-unknown-linux-gnu", "aarch64-apple-darwin"]
```

**Installation:**
```bash
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --default-toolchain stable
```

---

## System Dependencies

### Linux (Debian/Ubuntu)

**Required System Packages:**
```bash
git curl jq build-essential pkg-config libssl-dev
```

**Package Details:**
- `git` - Version control system
- `curl` - HTTP client for downloads
- `jq` - JSON processor for output parsing
- `build-essential` - C compiler and build tools (gcc, make, etc.)
- `pkg-config` - Package configuration helper
- `libssl-dev` - OpenSSL development headers

### macOS

**No additional system dependencies required** - standard macOS development environment is sufficient.

---

## Cargo Dependencies

### Runtime Dependencies

**Async Runtime:**
- `tokio` ^1 - Features: full

**Serialization:**
- `serde` ^1 - Features: derive
- `serde_json` ^1
- `serde_yaml` ^0.9

**CLI Framework:**
- `clap` ^4 - Features: derive

**Error Handling:**
- `anyhow` ^1
- `thiserror` ^1

**Logging/Telemetry:**
- `tracing` ^0.1
- `tracing-subscriber` ^0.3 - Features: env-filter, json

**Time Handling:**
- `chrono` ^0.4 - Features: serde

**Process Management:**
- `which` ^4

**Async Traits:**
- `async-trait` ^0.1

**File Locking:**
- `fs2` ^0.4

**Hashing:**
- `sha2` ^0.10
- `hex` ^0.4

**Pattern Matching:**
- `regex` ^1
- `glob` ^0.3
- `aho-corasick` ^1

**HTTP Client:**
- `ureq` ^2

**Configuration:**
- `cfg-if` ^1
- `atty` ^0.2
- `toml` ^0.8

**System Integration:**
- `libc` ^0.2
- `rand` ^0.8
- `futures` ^0.3
- `gethostname` ^0.4

### Optional OpenTelemetry Dependencies (feature-gated)

**OTLP Telemetry:**
- `opentelemetry` ^0.31
- `opentelemetry_sdk` ^0.31 - Features: rt-tokio
- `opentelemetry-otlp` ^0.31 - Features: grpc-tonic, http-proto
- `opentelemetry-semantic-conventions` ^0.31
- `tonic` ^0.14
- `tracing-opentelemetry` ^0.32

### Development Dependencies

**Testing:**
- `tokio-test` ^0.4
- `tempfile` ^3
- `proptest` ^1
- `filetime` ^0.2
- `criterion` ^0.5

**Integration Testing (optional):**
- `testcontainers` ^0.23

---

## Development Tools

### Command Line Tools

**GitHub CLI:**
```bash
# Installation (Linux)
curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | \
  dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg
echo "deb [arch=$(dpkg --print-architecture) \
  signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] \
  https://cli.github.com/packages stable main" | \
  tee /etc/apt/sources.list.d/github-cli.list
apt-get update && apt-get install -y gh
```

### BEAD CLI (br)

**Required for NEEDLE operation:**
- The `br` CLI (beads_rust) manages bead stores
- Available as a separate Rust project
- Used for bead creation, claiming, and lifecycle management

---

## Build Requirements

### Compilation

**Standard Build:**
```bash
cargo build --release
```

**Cross-Compilation Targets:**
- `x86_64-unknown-linux-gnu` (Linux x86_64)
- `aarch64-apple-darwin` (macOS ARM)

**Build Profile:**
```toml
[profile.release]
strip = true
lto = true
codegen-units = 1
```

### Testing

**Run Tests:**
```bash
cargo test
```

**Linting:**
```bash
cargo clippy --all-targets -- -D warnings
cargo fmt --check
```

---

## Installation Methods

### Pre-built Binary (Recommended)

```bash
curl -fsSL https://github.com/jedarden/NEEDLE/releases/latest/download/install.sh | bash
```

**Requirements for install script:**
- `curl` or `wget` for downloading
- `sha256sum` or `shasum` for checksum verification
- `gpg` (optional) for signature verification

### Build from Source

```bash
git clone https://github.com/jedarden/NEEDLE
cd NEEDLE
cargo install --path .
```

---

## Version Information

**Current NEEDLE Version:** 0.2.11

**Source Locations:**
- Primary Repository: `https://github.com/jedarden/NEEDLE`
- Documentation: `/home/coding/NEEDLE/README.md`
- Project Conventions: `/home/coding/NEEDLE/CLAUDE.md`
- Dependencies: `/home/coding/NEEDLE/Cargo.toml`
- Rust Toolchain: `/home/coding/NEEDLE/rust-toolchain.toml`

---

## Pluck-Specific Requirements

### Pluck Strand Configuration

**Default Exclude Labels:**
- `deferred` - Beads deferred for later processing
- `human` - Beads requiring human intervention
- `blocked` - Beads blocked by dependencies

**Split Threshold:**
- Default: 3 consecutive failures trigger bead splitting
- Configurable via `split_after_failures` parameter

### Runtime Dependencies for Pluck

**Bead Store Backend:**
- SQLite database (`beads.db`)
- JSONL checkpoint (`issues.jsonl`)
- Managed via `.beads/` directory

**Required CLI:**
- `br` CLI for bead store operations

---

## Quick Verification Commands

**Check Rust Version:**
```bash
rustc --version
# Should output: rustc 1.75.0 or later
```

**Check Cargo Installation:**
```bash
cargo --version
```

**Verify System Dependencies (Linux):**
```bash
git --version
curl --version
jq --version
```

**Test Build:**
```bash
cargo build --release
```

---

## Additional Notes

**Platform Support:**
- Linux: x86_64 (primary target)
- macOS: ARM64 (aarch64-apple-darwin)

**Feature Flags:**
- `otlp` (default) - OpenTelemetry/OTLP telemetry support
- `integration` - Integration testing with testcontainers

**Code Quality Requirements:**
- No `unwrap()` or `expect()` in non-test code
- All public functions must return `Result<T>`
- Exhaustive pattern matching (no catch-all `_`)
- Telemetry at every state transition

---

## Document Sources

- `/home/coding/NEEDLE/Cargo.toml` - Dependency specifications
- `/home/coding/NEEDLE/README.md` - Project documentation
- `/home/coding/NEEDLE/CLAUDE.md` - Development conventions
- `/home/coding/NEEDLE/rust-toolchain.toml` - Toolchain configuration
- `/home/coding/NEEDLE/install.sh` - Installation requirements
- `/home/coding/NEEDLE/ci/Dockerfile.ci` - CI environment setup
- `/home/coding/NEEDLE/src/strand/pluck.rs` - Pluck implementation

Generated: 2026-07-09
Bead ID: bf-1fyju
