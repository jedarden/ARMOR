# Pluck Dependencies - Installed Versions

**Bead:** bf-2xfb1  
**Date:** 2026-07-09  
**NEEDLE Version:** 0.2.11  
**Rust Version:** 1.96.1 (installed)  
**Minimum Required:** 1.75.0

## Installation Status Summary

✅ **All dependencies are installed and compatible** with the current Rust version (1.96.1).

## Core Dependencies - Installed Versions

### Async Runtime

| Dependency | Version | MSRV | Status |
|------------|---------|------|--------|
| **tokio** | 1.52.3 | 1.64.0+ | ✅ Installed |

### Serialization

| Dependency | Version | MSRV | Status |
|------------|---------|------|--------|
| **serde** | 1.0.228 | 1.56.0 | ✅ Installed |
| **serde_json** | 1.0.150 | 1.56.0 | ✅ Installed |
| **serde_yaml** | 0.9.34+deprecated | 1.56.0 | ✅ Installed |

### CLI

| Dependency | Version | MSRV | Status |
|------------|---------|------|--------|
| **clap** | 4.6.1 | 1.64.0 | ✅ Installed |
| **clap_builder** | 4.6.0 | 1.64.0 | ✅ Installed |
| **clap_derive** | 4.6.1 | 1.64.0 | ✅ Installed |

### Error Handling

| Dependency | Version | MSRV | Status |
|------------|---------|------|--------|
| **anyhow** | 1.0.103 | 1.68.0 | ✅ Installed |
| **thiserror** | 1.0.69 | 1.68.0 | ✅ Installed |

### Logging / Telemetry

| Dependency | Version | MSRV | Status |
|------------|---------|------|--------|
| **tracing** | 0.1.44 | 1.65.0 | ✅ Installed |
| **tracing-subscriber** | 0.3.23 | 1.65.0 | ✅ Installed |
| **tracing-opentelemetry** | 0.32.1 | 1.65.0+ | ✅ Installed |

### Time

| Dependency | Version | MSRV | Status |
|------------|---------|------|--------|
| **chrono** | 0.4.45 | 1.62.0 | ✅ Installed |

### Async Traits

| Dependency | Version | MSRV | Status |
|------------|---------|------|--------|
| **async-trait** | 0.1.89 | 1.56.0 | ✅ Installed |

## OpenTelemetry Dependencies (Optional - otlp feature)

| Dependency | Version | MSRV | Status |
|------------|---------|------|--------|
| **opentelemetry** | 0.31.0 | 1.75.0 | ✅ Installed |
| **opentelemetry_sdk** | 0.31.0 | 1.75.0 | ✅ Installed |
| **opentelemetry-otlp** | 0.31.1 | 1.75.0 | ✅ Installed |
| **opentelemetry-semantic-conventions** | 0.31.0 | 1.75.0 | ✅ Installed |
| **tonic** | 0.14.6 | 1.70.0+ | ✅ Installed |

## Other Dependencies

### Process Management

| Dependency | Version | Status |
|------------|---------|--------|
| **which** | 4.4.2 | ✅ Installed |
| **libc** | 0.2.186 | ✅ Installed |

### File System

| Dependency | Version | Status |
|------------|---------|--------|
| **fs2** | 0.4.3 | ✅ Installed |

### Cryptography

| Dependency | Version | Status |
|------------|---------|--------|
| **sha2** | 0.10.9 | ✅ Installed |
| **hex** | 0.4.3 | ✅ Installed |

### Pattern Matching

| Dependency | Version | Status |
|------------|---------|--------|
| **regex** | 1.12.4 | ✅ Installed |
| **glob** | 0.3.3 | ✅ Installed |
| **aho-corasick** | 1.1.4 | ✅ Installed |

### Networking

| Dependency | Version | Status |
|------------|---------|--------|
| **ureq** | 2.12.1 | ✅ Installed |

### Utilities

| Dependency | Version | Status |
|------------|---------|--------|
| **rand** | 0.8.6 | ✅ Installed |
| **atty** | 0.2.14 | ✅ Installed |
| **cfg-if** | 1.0.4 | ✅ Installed |
| **toml** | 0.8.23 | ✅ Installed |
| **gethostname** | 0.4.3 | ✅ Installed |
| **futures** | 0.3.32 | ✅ Installed |

## Version Compatibility Analysis

### Rust Compiler Version
- **Installed:** 1.96.1 (2026-06-26)
- **Required Minimum:** 1.75.0
- **Status:** ✅ Well above minimum requirement

### Dependency Status
All 35+ dependencies are installed and compatible with the current Rust version. No conflicts or missing dependencies detected.

### Key Observations
1. **All core dependencies** meet or exceed their minimum supported Rust versions (MSRV)
2. **OpenTelemetry stack** is fully installed with the otlp feature enabled
3. **Version ranges** in Cargo.toml allow automatic updates while maintaining compatibility
4. **No transitive dependency conflicts** detected in the dependency tree

## Verification Commands

To verify the installation status:

```bash
# Check Rust version
rustc --version

# Verify NEEDLE builds
cd /home/coding/NEEDLE
cargo check

# List all dependencies
cd /home/coding/NEEDLE
cargo tree
```

## Documentation References

- **Pluck Dependencies Documentation:** `/home/coding/ARMOR/docs/pluck-dependencies.md`
- **NEEDLE Source:** `/home/coding/NEEDLE/src/strand/pluck.rs`
- **Cargo Configuration:** `/home/coding/NEEDLE/Cargo.toml`

---

*This document was created as part of bead bf-2xfb1 to document the current installation status of Pluck dependencies.*
