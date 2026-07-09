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

## Current Version Inventory (ARMOR Environment)

**As of:** 2026-07-09
**Bead ID:** bf-fq15h

### Core Development Tools

| Tool | Minimum Required | Currently Installed | Status |
|------|-----------------|-------------------|--------|
| Go | 1.25.0 | go1.25.0 linux/amd64 | ✅ Compliant |
| Rust | 1.75+ | rustc 1.96.1 (2026-06-26) | ✅ Compliant |
| Cargo | (with Rust) | cargo 1.96.1 (2026-06-26) | ✅ Installed |
| Git | (system package) | git 2.50.1 | ✅ Installed |
| curl | (system package) | curl 8.14.1 | ✅ Installed |
| jq | (system package) | jq-1.7.1 | ✅ Installed |
| rustfmt | (with Rust) | rustfmt 1.96.1 | ✅ Installed |
| clippy | (with Rust) | clippy 1.96.1 | ✅ Installed |

### NEEDLE/Pluck Components

| Component | Version | Binary Location |
|-----------|---------|----------------|
| NEEDLE CLI | 0.2.11 | `~/.local/bin/needle` |
| br CLI (bead-forge) | 0.2.0 | `~/.local/bin/br` (bead-forge) |
| Pluck Strand | (part of NEEDLE) | `needle strand pluck` |

### NEEDLE Rust Dependencies - Current Installed Versions

| Dependency | Minimum Required | Currently Installed | Purpose |
|------------|-----------------|-------------------|---------|
| tokio | ^1 | v1.52.3 | Async runtime |
| serde | ^1 | v1.0.228 | Serialization |
| serde_json | ^1 | v1.0.150 | JSON serialization |
| serde_yaml | ^0.9 | v0.9.34+deprecated | YAML serialization |
| clap | ^4 | v4.6.1 | CLI framework |
| anyhow | ^1 | v1.0.103 | Error handling |
| thiserror | ^1 | v1.0.69 | Error handling |
| tracing | ^0.1 | v0.1.44 | Logging/telemetry |
| tracing-subscriber | ^0.3 | v0.3.23 | Logging subscriber |
| chrono | ^0.4 | v0.4.45 | Time handling |
| which | ^4 | v4.4.2 | Process management |
| async-trait | ^0.1 | v0.1.89 | Async traits |
| fs2 | ^0.4 | v0.4.3 | File locking |
| sha2 | ^0.10 | v0.10.9 | Hashing |
| hex | ^0.4 | v0.4.3 | Hex encoding |
| regex | ^1 | v1.12.4 | Pattern matching |
| glob | ^0.3 | v0.3.3 | Glob patterns |
| aho-corasick | ^1 | v1.1.4 | Multi-pattern search |
| ureq | ^2 | v2.12.1 | HTTP client |
| cfg-if | ^1 | v1.0.4 | Conditional compilation |
| atty | ^0.2 | v0.2.14 | Terminal detection |
| toml | ^0.8 | v0.8.23 | TOML parsing |
| libc | ^0.2 | v0.2.186 | Unix process handling |
| rand | ^0.8 | v0.8.6 | Random generation |
| futures | ^0.3 | v0.3.32 | Async utilities |
| gethostname | ^0.4 | v0.4.3 | Hostname detection |

### OpenTelemetry Dependencies (Optional Features)

| Dependency | Minimum Required | Currently Installed | Purpose |
|------------|-----------------|-------------------|---------|
| opentelemetry | ^0.31 | v0.31.0 | OTLP telemetry |
| opentelemetry_sdk | ^0.31 | v0.31.0 | OTLP SDK |
| opentelemetry-otlp | ^0.31 | v0.31.1 | OTLP exporter |
| opentelemetry-semantic-conventions | ^0.31 | v0.31.0 | Semantic conventions |
| tonic | ^0.14 | v0.14.6 | gRPC |
| tracing-opentelemetry | ^0.32 | v0.32.1 | Tracing integration |

### Development Dependencies

| Dependency | Minimum Required | Currently Installed | Purpose |
|------------|-----------------|-------------------|---------|
| tokio-test | ^0.4 | v0.4.5 | Async testing |
| tempfile | ^3 | v3.27.0 | Temporary files |
| proptest | ^1 | v1.11.0 | Property testing |
| filetime | ^0.2 | v0.2.29 | File time testing |
| criterion | ^0.5 | v0.5.1 | Benchmarking |
| testcontainers | ^0.23 | v0.23.0 (optional) | Integration testing |

### ARMOR Go Dependencies

| Dependency | Minimum Required | Currently Installed | Status |
|------------|-----------------|-------------------|--------|
| github.com/aws/aws-sdk-go-v2 | - | v1.41.4 | ✅ Current |
| github.com/aws/aws-sdk-go-v2/config | - | v1.32.12 | ✅ Current |
| github.com/aws/aws-sdk-go-v2/credentials | - | v1.19.12 | ✅ Current |
| github.com/aws/aws-sdk-go-v2/service/s3 | - | v1.97.2 | ✅ Current |
| github.com/kurin/blazer | - | v0.5.3 | ✅ Current |
| golang.org/x/crypto | - | v0.49.0 | ✅ Current |
| golang.org/x/sync | - | v0.12.0 | ✅ Current |

### Transitive Go Dependencies (AWS SDK v2)

| Dependency | Currently Installed | Purpose |
|------------|-------------------|---------|
| github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream | v1.7.8 | Event streaming protocol |
| github.com/aws/aws-sdk-go-v2/feature/ec2/imds | v1.18.20 | EC2 Instance Metadata Service |
| github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 | v2.7.20 | Endpoint resolution |
| github.com/aws/aws-sdk-go-v2/service/internal/s3shared | v1.19.20 | S3 shared utilities |
| github.com/aws/aws-sdk-go-v2/service/sso | v1.30.13 | AWS SSO integration |
| github.com/aws/aws-sdk-go-v2/service/ssooidc | v1.35.17 | AWS SSO OIDC |
| github.com/aws/aws-sdk-go-v2/service/sts | v1.41.9 | AWS Security Token Service |
| github.com/aws/smithy-go | v1.24.2 | Smithy protocol runtime |
| golang.org/x/net | v0.51.0 | Network utilities |
| golang.org/x/sys | v0.42.0 | System interfaces |
| golang.org/x/term | v0.41.0 | Terminal handling |
| golang.org/x/text | v0.35.0 | Text processing |

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

**Check Go Version:**
```bash
go version
# Should output: go version go1.25.0 or later
```

**Check Rust Version:**
```bash
rustc --version
# Should output: rustc 1.75.0 or later
```

**Check Cargo Installation:**
```bash
cargo --version
```

**Verify NEEDLE Installation:**
```bash
needle --version
# Should output: needle 0.2.11 or later
```

**Verify br CLI Installation:**
```bash
br --version
# Should output: Error: bf 0.2.0 or later
# (Note: br outputs version as "Error: bf X.Y.Z" due to error handling)
```

**Verify System Dependencies (Linux):**
```bash
git --version
curl --version
jq --version
```

**Check Go Dependency Versions:**
```bash
go list -m all | grep -E "(github.com/aws|golang.org/x)"
```

**Test Build:**
```bash
cargo build --release  # For NEEDLE/Pluck
go build ./...         # For ARMOR
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

## ARMOR Workspace Pluck Integration

### ARMOR Project Overview

ARMOR (Automatic Recovery and Monitoring Operations Resilience) is a Go-based project that integrates with the NEEDLE system's Pluck strand for bead-based task management.

### ARMOR-Specific Dependencies

In addition to the core Pluck/NEEDLE requirements, ARMOR requires:

**AWS SDK for Go v2:**
- `github.com/aws/aws-sdk-go-v2` - AWS service integration
- `github.com/aws/aws-sdk-go-v2/service/s3` - S3 storage operations
- `github.com/aws/aws-sdk-go-v2/config` - AWS configuration
- `github.com/aws/aws-sdk-go-v2/credentials` - AWS credential management

**Google Cloud Storage:**
- `github.com/kurin/blazer` - GCS operations (alternative to S3)

**Cryptography:**
- `golang.org/x/crypto` - Cryptographic primitives

**Concurrency:**
- `golang.org/x/sync` - Advanced synchronization primitives

### ARMOR-Pluck Integration Points

ARMOR interacts with Pluck through:

1. **Bead Store Access:** ARMOR uses the br CLI for bead operations
2. **Configuration Files:** `pluck-config.yaml` controls Pluck behavior in ARMOR workspace
3. **Debug Logging:** Pluck debug logs stored in `logs/pluck-debug.log`
4. **Workspace Management:** ARMOR workspace tracked in NEEDLE bead system

### ARMOR Development Environment

**Build Requirements:**
```bash
# Build ARMOR
go build ./...

# Run ARMOR with Pluck integration
./armor --workspace $(pwd)

# Test Pluck configuration
needle strand pluck --config pluck-config.yaml --dry-run
```

**Testing Requirements:**
```bash
# Run ARMOR tests
go test ./...

# Test Pluck strand integration
needle test pluck --workspace /home/coding/ARMOR
```

### Installation and Setup for ARMOR

**Prerequisites:**

1. **Install Go 1.25.0+:**
   ```bash
   # Verify installation
   go version
   ```

2. **Install NEEDLE/br CLI:**
   ```bash
   # From pre-built binary
   curl -fsSL https://github.com/jedarden/NEEDLE/releases/latest/download/install.sh | bash
   
   # Or build from source
   git clone https://github.com/jedarden/NEEDLE
   cd NEEDLE
   cargo install --path .
   ```

3. **Install ARMOR Dependencies:**
   ```bash
   cd /home/coding/ARMOR
   go mod download
   ```

**Verification:**
```bash
# Check all versions
echo "=== Core Tools ===" && \
go version && \
rustc --version && \
git --version && \
echo "" && \
echo "=== NEEDLE Components ===" && \
needle --version && \
br --version && \
echo "" && \
echo "=== ARMOR Build ===" && \
go build ./... && echo "ARMOR build successful"
```

---

## Document Sources

- `/home/coding/NEEDLE/Cargo.toml` - Dependency specifications
- `/home/coding/NEEDLE/README.md` - Project documentation
- `/home/coding/NEEDLE/CLAUDE.md` - Development conventions
- `/home/coding/NEEDLE/rust-toolchain.toml` - Toolchain configuration
- `/home/coding/NEEDLE/install.sh` - Installation requirements
- `/home/coding/NEEDLE/ci/Dockerfile.ci` - CI environment setup
- `/home/coding/NEEDLE/src/strand/pluck.rs` - Pluck implementation
- `/home/coding/ARMOR/go.mod` - ARMOR Go dependencies
- `/home/coding/ARMOR/pluck-config.yaml` - ARMOR Pluck configuration
- `/home/coding/ARMOR/README.md` - ARMOR project documentation

## Document Maintenance

**Last Updated:** 2026-07-09  
**Updated for Bead:** bf-fq15h  
**Originally Created:** 2026-07-09 for bead bf-1fyju

**Update Procedure:**
1. Run `cargo tree --depth 1` in `/home/coding/NEEDLE` to get current versions
2. Check tool versions: `rustc --version`, `cargo --version`, `go version`, `git --version`
3. Update version inventory tables with actual installed versions
4. Verify all minimum requirements are still met
5. Commit and push changes
