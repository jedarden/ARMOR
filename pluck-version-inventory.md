# Pluck Dependency Version Inventory

**Document Created:** 2026-07-09  
**Bead:** bf-fq15h  
**Workspace:** /home/coding/ARMOR  
**Status:** ✅ Complete

## Overview

This document provides a comprehensive inventory of Pluck and its dependencies. Pluck is a strand within the NEEDLE system that processes beads from the assigned workspace. This inventory covers:

1. **Core NEEDLE/Pluck dependencies** (Rust-based)
2. **br CLI dependencies** (bead management system)
3. **ARMOR workspace dependencies** (Go-based)
4. **Development tool versions**

---

## Core NEEDLE/Pluck Dependencies

### Project Information

| Attribute | Value |
|-----------|-------|
| **Project Name** | needle |
| **Current Version** | 0.2.11 |
| **Rust Edition** | 2021 |
| **MSRV (Minimum Supported Rust Version)** | 1.75 (2023-12-28) |
| **License** | MIT |

### Installed Dependencies

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

#### File Operations
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

#### OpenTelemetry (Optional - otlp feature)
| Dependency | Version | Purpose |
|------------|---------|---------|
| opentelemetry | 0.31 | OpenTelemetry API |
| opentelemetry_sdk | 0.31 (with rt-tokio) | OpenTelemetry SDK |
| opentelemetry-otlp | 0.31 (with grpc-tonic, http-proto) | OTLP exporter |
| opentelemetry-semantic-conventions | 0.31 | Semantic conventions |
| tonic | 0.14 | gRPC for OTLP |

#### Development Dependencies
| Dependency | Version | Purpose |
|------------|---------|---------|
| tokio-test | 0.4 | Tokio testing utilities |
| tempfile | 3 | Temporary file handling |
| proptest | 1 | Property-based testing |
| filetime | 0.2 | File time manipulation |
| criterion | 0.5 | Benchmarking |

#### Integration Testing (Optional - integration feature)
| Dependency | Version | Purpose |
|------------|---------|---------|
| testcontainers | 0.23 | Docker container integration testing |

---

## br CLI (Beads Rust) Information

| Attribute | Value |
|-----------|-------|
| **Binary Name** | br (beads_rust) |
| **Current Version** | 0.2.0 |
| **Installation Path** | ~/.local/bin/br |
| **Purpose** | Bead store management and CLI for NEEDLE |

### Core Functionality
- Bead creation, listing, and management
- Atomic bead claiming via SQLite transactions
- Workspace coordination for multi-worker setups
- Integration with NEEDLE strand system

---

## ARMOR Workspace Dependencies (Go)

### Project Information

| Attribute | Value |
|-----------|-------|
| **Module Path** | github.com/jedarden/armor |
| **Go Version** | 1.25.0 |

### Installed Dependencies

#### AWS SDK v2
| Dependency | Version | Purpose |
|------------|---------|---------|
| github.com/aws/aws-sdk-go-v2 | v1.41.4 | AWS SDK core |
| github.com/aws/aws-sdk-go-v2/config | v1.32.12 | AWS configuration |
| github.com/aws/aws-sdk-go-v2/credentials | v1.19.12 | AWS credentials |
| github.com/aws/aws-sdk-go-v2/service/s3 | v1.97.2 | S3 service |
| github.com/aws/aws-sdk-go-v2/feature/ec2/imds | v1.18.20 (indirect) | EC2 instance metadata |
| github.com/aws/aws-sdk-go-v2/service/sso | v1.30.13 (indirect) | SSO service |
| github.com/aws/aws-sdk-go-v2/service/ssooidc | v1.35.17 (indirect) | SSO OIDC |
| github.com/aws/aws-sdk-go-v2/service/sts | v1.41.9 (indirect) | STS service |

#### Google Cloud Storage
| Dependency | Version | Purpose |
|------------|---------|---------|
| github.com/kurin/blazer | v0.5.3 | Google Cloud Storage client |

#### Google Extended Libraries
| Dependency | Version | Purpose |
|------------|---------|---------|
| golang.org/x/crypto | v0.49.0 | Cryptography extensions |
| golang.org/x/sync | v0.12.0 | Concurrency extensions |

#### AWS Smithy Framework
| Dependency | Version | Purpose |
|------------|---------|---------|
| github.com/aws/smithy-go | v1.24.2 (indirect) | Smithy protocol framework |

---

## Development Tools

### Rust Toolchain

| Tool | Version | Purpose |
|------|---------|---------|
| rustc | 1.96.1 (31fca3adb 2026-06-26) | Rust compiler |
| cargo | 1.96.1 (356927216 2026-06-26) | Package manager |
| rustfmt | 1.9.0-stable (31fca3adb2 2026-06-26) | Code formatter |
| clippy | 0.1.96 (31fca3adb2 2026-06-26) | Linter |

### Rust Toolchain Configuration

**File:** `rust-toolchain.toml`

```toml
[toolchain]
channel = "stable"
components = ["rustfmt", "clippy"]
targets = ["x86_64-unknown-linux-gnu", "aarch64-apple-darwin"]
```

### Go Toolchain

| Tool | Minimum Version | Current Version |
|------|-----------------|-----------------|
| go | 1.25.0 | 1.25.0 |

---

## Minimum Version Requirements

### NEEDLE/Pluck

| Component | Minimum Version | Rationale |
|-----------|-----------------|-----------|
| Rust | 1.75 | MSRV (Minimum Supported Rust Version) |
| Rust Edition | 2021 | Language edition requirement |
| SQLite | 3.0 | Bead store backend (via br CLI) |

### ARMOR Workspace

| Component | Minimum Version | Rationale |
|-----------|-----------------|-----------|
| Go | 1.25.0 | Go module requirement |

---

## System Requirements

### Build Requirements
- **Disk Space:** ~100GB for Rust builds (target/ directory)
- **Memory:** 8GB+ RAM recommended for cargo builds
- **CPU:** Multi-core recommended for parallel builds

### Runtime Requirements
- **Linux:** x86_64-unknown-linux-gnu
- **macOS:** aarch64-apple-darwin (Apple Silicon)
- **SQLite:** For bead store (.beads/beads.db)

---

## Dependency Management

### Rust Dependencies (NEEDLE)

**Location:** `/home/coding/NEEDLE/Cargo.toml`

**Update Process:**
```bash
cd /home/coding/NEEDLE
cargo update
cargo build --release
```

**Add New Dependency:**
```bash
cargo add dependency_name
```

### Go Dependencies (ARMOR)

**Location:** `/home/coding/ARMOR/go.mod`

**Update Process:**
```bash
cd /home/coding/ARMOR
go get -u ./...
go mod tidy
```

**Add New Dependency:**
```bash
go get github.com/package/name
```

---

## Feature Flags

### NEEDLE Features

| Feature | Description | Dependencies |
|---------|-------------|--------------|
| `otlp` | OpenTelemetry OTLP export (default) | opentelemetry, tonic, etc. |
| `integration` | Integration test support | testcontainers |

### Usage

```bash
# Build with default features (includes otlp)
cargo build --release

# Build without otlp
cargo build --release --no-default-features

# Build with integration tests
cargo build --release --features integration
```

---

## Security Considerations

### Dependency Auditing

```bash
# Check for security advisories in Rust dependencies
cd /home/coding/NEEDLE
cargo audit

# Check for vulnerabilities in Go dependencies
cd /home/coding/ARMOR
go list -json -m all | nancy sleuth
```

### Known Security Considerations

- **HTTP Client:** Uses ureq for self-update (simple HTTP, no TLS verification by default)
- **Process Management:** Executes external agent CLIs via bash -c
- **File Operations:** Uses SQLite with flock for coordination
- **Credential Storage:** AWS/GCP credentials managed via standard SDK chains

---

## Troubleshooting

### Common Issues

#### Build Failures
```bash
# Clean build if dependency cache is corrupted
cd /home/coding/NEEDLE
cargo clean
cargo build --release
```

#### Dependency Conflicts
```bash
# Update Cargo.lock and resolve conflicts
cd /home/coding/NEEDLE
cargo update
```

#### Disk Space Issues
```bash
# Check disk space before large builds
df -BG --output=avail /
du -sh ~/NEEDLE/target
```

---

## Version Upgrade Notes

### NEEDLE 0.2.x → Next Major

**Breaking Changes Expected:**
- MSRV may increase to 1.80+
- OpenTelemetry dependencies may upgrade to 0.32+
- tokio dependency may require newer async features

**Migration Path:**
1. Update rust-toolchain.toml if MSRV increases
2. Run `cargo update`
3. Test with `--features integration` before production deployment

### ARMOR Go Dependencies

**Version Policies:**
- AWS SDK v2: Follow AWS recommended versions
- golang.org/x packages: Track stable releases
- Always run `go mod tidy` after version updates

---

## Documentation References

### Internal Documentation

- **NEEDLE README:** `/home/coding/NEEDLE/README.md`
- **NEEDLE CLAUDE.md:** `/home/coding/NEEDLE/CLAUDE.md`
- **Pluck Configuration:** `/home/coding/ARMOR/pluck-config.yaml`
- **Pluck Debug Summary:** `/home/coding/ARMOR/pluck-debug-summary.md`

### External Resources

- **Rust MSRV Policy:** https://rust-lang.github.io/rfcs/2495-min-rust-version.html
- **AWS SDK v2 Documentation:** https://aws.github.io/aws-sdk-go-v2/docs/
- **OpenTelemetry Rust:** https://opentelemetry.io/docs/instrumentation/rust/

---

## Change History

| Date | Version | Changes |
|------|---------|---------|
| 2026-07-09 | 1.0 | Initial version inventory created |

---

## Maintenance Notes

### Regular Maintenance Tasks

1. **Monthly:** Run `cargo update` in NEEDLE and check for security advisories
2. **Monthly:** Run `go get -u ./...` in ARMOR and update dependencies
3. **Quarterly:** Review and update this inventory document
4. **As Needed:** Update after major version bumps or breaking changes

### Contact Information

- **Repository:** https://github.com/jedarden/NEEDLE
- **Issues:** https://github.com/jedarden/NEEDLE/issues
- **ARMOR Repository:** https://github.com/jedarden/ARMOR

---

**Document Status:** ✅ Complete  
**Last Updated:** 2026-07-09  
**Next Review:** 2026-10-09 (Quarterly)