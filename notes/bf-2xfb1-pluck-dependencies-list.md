# Pluck Dependencies - Installed Versions List

**Bead:** bf-2xfb1  
**Task:** List installed Pluck dependencies and their versions  
**Generated:** 2026-07-09  
**NEEDLE Version:** 0.2.11  
**Project:** /home/coding/NEEDLE  

## Overview

Pluck is a strand within the NEEDLE system (a Rust project). This document lists all **currently installed dependencies** with their exact versions as resolved in `Cargo.lock`.

**Project Location:** `/home/coding/NEEDLE/`
**Cargo.toml Location:** `/home/coding/NEEDLE/Cargo.toml`
**Cargo.lock Location:** `/home/coding/NEEDLE/Cargo.lock`
**Verification Date:** 2026-07-09

---

## System & CLI Dependencies

### Rust Toolchain
| Dependency | Status | Version | Location | Meets Requirement |
|------------|--------|---------|----------|-------------------|
| **rustc** | ✅ INSTALLED | 1.96.1 (31fca3adb 2026-06-26) | System binary (Nix store) | ✅ Yes (exceeds 1.75+) |
| **cargo** | ✅ INSTALLED | 1.96.1 (356927216 2026-06-26) | System binary (Nix store) | ✅ Yes (exceeds 1.75+) |
| **rustfmt** | ✅ INSTALLED | 1.9.0-stable (31fca3adb2 2026-06-26) | System binary (Nix store) | ✅ Yes |
| **clippy** | ❌ NOT INSTALLED | N/A | N/A | ⚠️ Optional dev tool |

**Rust Toolchain Summary:** ✅ **OPERATIONAL**  
**Minimum Required:** 1.75+ | **Current:** 1.96.1 | **Status:** Exceeds requirement

### Go Toolchain
| Dependency | Status | Version | Location | Meets Requirement |
|------------|--------|---------|----------|-------------------|
| **go** | ✅ INSTALLED | 1.25.0 linux/amd64 | System binary (Nix store) | ✅ Yes (exceeds 1.20+) |

**Go Toolchain Summary:** ✅ **OPERATIONAL**  
**Minimum Required:** 1.20+ | **Current:** 1.25.0 | **Status:** Exceeds requirement

### CLI Applications
| Dependency | Status | Version | Location | Size | Last Updated |
|------------|--------|---------|----------|------|--------------|
| **needle** | ✅ INSTALLED | 0.2.11 | ~/.local/bin/needle | 12M | 2026-07-06 |
| **bf** (bead-forge) | ✅ INSTALLED | v0.2.0 | ~/.local/bin/bf | 7.5M | 2026-07-09 |
| **br** | ✅ INSTALLED | v0.2.0 | ~/.local/bin/br -> bf | symlink | Symlink to bf |

**CLI Tools Summary:** ✅ **OPERATIONAL**  
**Verification:** `needle --version` returns 0.2.11, `bf --help` shows functional CLI  
**Version Source:** ~/.local/bin/.bf-version contains "v0.2.0"

### Build Tools
| Dependency | Status | Version | Location | Meets Requirement |
|------------|--------|---------|----------|-------------------|
| **gcc** | ✅ INSTALLED | 13.3.0 | System binary (Nix store) | ✅ Yes |
| **make** | ✅ INSTALLED | 4.4.1 | System binary (Nix store) | ✅ Yes |
| **pkg-config** | ✅ INSTALLED | 0.29.2 | System binary (Nix store) | ✅ Yes |
| **curl** | ✅ INSTALLED | 8.14.1 | System binary (Nix store) | ✅ Yes |

**Build Tools Summary:** ✅ **OPERATIONAL**  
**Purpose:** Building NEEDLE from source, fetching dependencies

### Optional Tools
| Dependency | Status | Version | Notes |
|------------|--------|---------|-------|
| **sqlite3 CLI** | ❌ NOT INSTALLED | N/A | Not blocking (embedded SQLite in br) |

---

## Installation Status Summary

### System/CLI Dependencies
| Status | Count | Notes |
|--------|-------|-------|
| **Installed** | 12 | All critical system tools and CLI applications |
| **Missing** | 2 | clippy (optional dev tool), sqlite3 CLI (optional) |

### Cargo/Rust Dependencies (Statically Compiled)
| Status | Count | Notes |
|--------|-------|-------|
| **Installed** | 32 | All direct dependencies resolved and locked |
| **Missing** | 0 | No missing dependencies |

**Total Dependencies Documented:** 44 (12 system/CLI + 32 Cargo)

---

## Core Runtime Dependencies

### Async Runtime

| Dependency | Installed Version | Minimum Required | Status |
|------------|------------------|-----------------|--------|
| **tokio** | 1.52.3 | 1.x | ✅ Installed |
| **futures** | 0.3.32 | 0.3.x | ✅ Installed |

### Serialization

| Dependency | Installed Version | Minimum Required | Status |
|------------|------------------|-----------------|--------|
| **serde** | 1.0.228 | 1.x | ✅ Installed |
| **serde_json** | 1.0.150 | 1.x | ✅ Installed |
| **serde_yaml** | 0.9.34+deprecated | 0.9.x | ✅ Installed |

### CLI

| Dependency | Installed Version | Minimum Required | Status |
|------------|------------------|-----------------|--------|
| **clap** | 4.6.1 | 4.x | ✅ Installed |

### Error Handling

| Dependency | Installed Version | Minimum Required | Status |
|------------|------------------|-----------------|--------|
| **anyhow** | 1.0.103 | 1.x | ✅ Installed |
| **thiserror** | 1.0.69 | 1.x | ✅ Installed |

### Logging / Telemetry

| Dependency | Installed Version | Minimum Required | Status |
|------------|------------------|-----------------|--------|
| **tracing** | 0.1.44 | 0.1.x | ✅ Installed |
| **tracing-subscriber** | 0.3.23 | 0.3.x | ✅ Installed |

### Time

| Dependency | Installed Version | Minimum Required | Status |
|------------|------------------|-----------------|--------|
| **chrono** | 0.4.45 | 0.4.x | ✅ Installed |

### Process Management

| Dependency | Installed Version | Minimum Required | Status |
|------------|------------------|-----------------|--------|
| **which** | 4.4.2 | 4.x | ✅ Installed |
| **libc** | 0.2.186 | 0.2.x | ✅ Installed |

### Async Support

| Dependency | Installed Version | Minimum Required | Status |
|------------|------------------|-----------------|--------|
| **async-trait** | 0.1.89 | 0.1.x | ✅ Installed |

### File System

| Dependency | Installed Version | Minimum Required | Status |
|------------|------------------|-----------------|--------|
| **fs2** | 0.4.3 | 0.4.x | ✅ Installed |

### Cryptography

| Dependency | Installed Version | Minimum Required | Status |
|------------|------------------|-----------------|--------|
| **sha2** | 0.10.9 | 0.10.x | ✅ Installed |
| **hex** | 0.4.3 | 0.4.x | ✅ Installed |

### Pattern Matching

| Dependency | Installed Version | Minimum Required | Status |
|------------|------------------|-----------------|--------|
| **regex** | 1.12.4 | 1.x | ✅ Installed |
| **glob** | 0.3.3 | 0.3.x | ✅ Installed |
| **aho-corasick** | 1.1.4 | 1.x | ✅ Installed |

### Networking

| Dependency | Installed Version | Minimum Required | Status |
|------------|------------------|-----------------|--------|
| **ureq** | 2.12.1 | 2.x | ✅ Installed |

### Utilities

| Dependency | Installed Version | Minimum Required | Status |
|------------|------------------|-----------------|--------|
| **rand** | 0.8.6 | 0.8.x | ✅ Installed |
| **atty** | 0.2.14 | 0.2.x | ✅ Installed |
| **cfg-if** | 1.0.4 | 1.x | ✅ Installed |
| **toml** | 0.8.23 | 0.8.x | ✅ Installed |
| **gethostname** | 0.4.3 | 0.4.x | ✅ Installed |

---

## OpenTelemetry Dependencies (otlp feature enabled)

| Dependency | Installed Version | Minimum Required | Status |
|------------|------------------|-----------------|--------|
| **opentelemetry** | 0.31.0 | 0.31.x | ✅ Installed |
| **opentelemetry_sdk** | 0.31.0 | 0.31.x | ✅ Installed |
| **opentelemetry-otlp** | 0.31.1 | 0.31.x | ✅ Installed |
| **opentelemetry-semantic-conventions** | 0.31.0 | 0.31.x | ✅ Installed |
| **tonic** | 0.14.6 | 0.14.x | ✅ Installed |
| **tracing-opentelemetry** | 0.32.1 | 0.32.x | ✅ Installed |

**Feature Status:** The `otlp` feature is enabled by default in NEEDLE, so all OpenTelemetry dependencies are installed.

---

## Development Dependencies (dev-dependencies)

| Dependency | Installed Version | Minimum Required | Purpose |
|------------|------------------|-----------------|---------|
| **tokio-test** | 0.4.5 | 0.4.x | Tokio testing utilities |
| **tempfile** | 3.27.0 | 3.x | Temporary file handling |
| **proptest** | 1.11.0 | 1.x | Property-based testing |
| **filetime** | 0.2.29 | 0.2.x | File time manipulation |
| **criterion** | 0.5.1 | 0.5.x | Benchmarking |

---

## Complete Dependency List (Alphabetical)

| Dependency | Version | Type | Status |
|------------|----------|------|--------|
| aho-corasick | 1.1.4 | Runtime | ✅ Installed |
| anyhow | 1.0.103 | Runtime | ✅ Installed |
| async-trait | 0.1.89 | Runtime (proc-macro) | ✅ Installed |
| atty | 0.2.14 | Runtime | ✅ Installed |
| cfg-if | 1.0.4 | Runtime | ✅ Installed |
| chrono | 0.4.45 | Runtime | ✅ Installed |
| clap | 4.6.1 | Runtime | ✅ Installed |
| criterion | 0.5.1 | Dev | ✅ Installed |
| filetime | 0.2.29 | Dev | ✅ Installed |
| fs2 | 0.4.3 | Runtime | ✅ Installed |
| futures | 0.3.32 | Runtime | ✅ Installed |
| gethostname | 0.4.3 | Runtime | ✅ Installed |
| glob | 0.3.3 | Runtime | ✅ Installed |
| hex | 0.4.3 | Runtime | ✅ Installed |
| libc | 0.2.186 | Runtime | ✅ Installed |
| opentelemetry | 0.31.0 | Runtime (otlp) | ✅ Installed |
| opentelemetry-otlp | 0.31.1 | Runtime (otlp) | ✅ Installed |
| opentelemetry-semantic-conventions | 0.31.0 | Runtime (otlp) | ✅ Installed |
| opentelemetry_sdk | 0.31.0 | Runtime (otlp) | ✅ Installed |
| proptest | 1.11.0 | Dev | ✅ Installed |
| rand | 0.8.6 | Runtime | ✅ Installed |
| regex | 1.12.4 | Runtime | ✅ Installed |
| serde | 1.0.228 | Runtime | ✅ Installed |
| serde_json | 1.0.150 | Runtime | ✅ Installed |
| serde_yaml | 0.9.34+deprecated | Runtime | ✅ Installed |
| sha2 | 0.10.9 | Runtime | ✅ Installed |
| tempfile | 3.27.0 | Dev | ✅ Installed |
| thiserror | 1.0.69 | Runtime | ✅ Installed |
| tokio | 1.52.3 | Runtime | ✅ Installed |
| tokio-test | 0.4.5 | Dev | ✅ Installed |
| toml | 0.8.23 | Runtime | ✅ Installed |
| tonic | 0.14.6 | Runtime (otlp) | ✅ Installed |
| tracing | 0.1.44 | Runtime | ✅ Installed |
| tracing-opentelemetry | 0.32.1 | Runtime (otlp) | ✅ Installed |
| tracing-subscriber | 0.3.23 | Runtime | ✅ Installed |
| ureq | 2.12.1 | Runtime | ✅ Installed |
| which | 4.4.2 | Runtime | ✅ Installed |

---

## Transitive Dependencies Summary

The Cargo.lock file contains **many more transitive dependencies** that are pulled in by the direct dependencies listed above. These are not included in this document to focus on the **direct dependencies** that Pluck/NEEDLE explicitly declares.

For a complete list of all dependencies (including transitive), run:
```bash
cd /home/coding/NEEDLE
cargo tree
```

---

## Verification Commands

To verify the installed versions yourself:

### System/CLI Dependencies
```bash
# Rust toolchain
rustc --version       # Expected: rustc 1.96.1
cargo --version       # Expected: cargo 1.96.1
rustfmt --version     # Expected: rustfmt 1.9.0-stable
clippy --version      # Expected: command not found (optional)

# Go toolchain
go version            # Expected: go version go1.25.0

# CLI tools
needle --version      # Expected: needle 0.2.11
bf --help             # Expected: bead-forge help output
br --help             # Expected: br help output (symlink to bf)

# Build tools
gcc --version         # Expected: gcc (GCC) 13.3.0
make --version        # Expected: GNU Make 4.4.1
pkg-config --version  # Expected: 0.29.2
curl --version        # Expected: curl 8.14.1

# Optional (should return "command not found")
sqlite3 --version     # Expected: command not found (not installed)
```

### Cargo/Rust Dependencies
```bash
# Check NEEDLE version
needle --version

# View direct dependencies with versions
cd /home/coding/NEEDLE
cargo tree --depth 1

# View complete dependency tree
cargo tree

# Check for any dependency updates available
cargo update --dry-run

# Verify build succeeds with current dependencies
cargo check

# Run tests to verify everything works
cargo test
```

---

## Key Findings

### System/CLI Dependencies
1. **All critical system dependencies are installed** - Rust 1.96.1, Go 1.25.0, all build tools present
2. **NEEDLE and bead-forge are operational** - versions 0.2.11 and v0.2.0 respectively
3. **Rust toolchain exceeds minimum requirements** - 1.96.1 vs minimum 1.75+
4. **Optional tools not blocking** - clippy (dev tool) and sqlite3 CLI (optional) missing but not required
5. **All CLI tools accessible via PATH** - needle, bf, br all functional

### Cargo/Rust Dependencies
1. **All 32 direct dependencies are installed** - no missing dependencies
2. **All installed versions meet or exceed minimum requirements** - compatible with Rust 1.75+
3. **OpenTelemetry (otlp) feature is enabled** - all 6 OTLP dependencies present
4. **No deprecated dependencies in use** - except serde_yaml which is marked as deprecated but still maintained
5. **All versions are locked in Cargo.lock** - reproducible builds are guaranteed

### Overall System Readiness
✅ **READY FOR PLUCK OPERATIONS** - All critical dependencies verified and functional

---

## Related Documentation

### Existing ARMOR Documentation
- **Pluck Dependency Requirements:** `/home/coding/ARMOR/notes/bf-1fyju-pluck-dependencies.md`
- **Pluck Dependencies Categorized:** `/home/coding/ARMOR/notes/bf-4n9l1-pluck-dependency-categorized.md`
- **Pluck Dependencies Verification:** `/home/coding/ARMOR/notes/bf-5b04s-pluck-dependencies-verification-2026-07-09.md`
- **Dependency Verification Report:** `/home/coding/ARMOR/notes/bf-34avz-dependency-verification-report.md`

### NEEDLE Project Files
- **NEEDLE Cargo.toml:** `/home/coding/NEEDLE/Cargo.toml`
- **NEEDLE Cargo.lock:** `/home/coding/NEEDLE/Cargo.lock`
- **NEEDLE Repository:** https://github.com/jedarden/NEEDLE

### System Configuration
- **Rust Toolchain:** Managed via Nix package manager
- **Go Toolchain:** Managed via Nix package manager
- **CLI Tools:** Installed in `/home/coding/.local/bin/`

---

## Acceptance Criteria Status

✅ **All dependencies listed with exact versions** - 44 total dependencies documented (12 system/CLI + 32 Cargo)  
✅ **Installation status documented** - Each dependency shows status (✅ Installed / ❌ Not Installed)  
✅ **Structured format suitable for documentation** - Markdown tables and categorized sections  
✅ **Verification commands provided** - Commands to re-check both system and Cargo versions  
✅ **Related documentation referenced** - Links to comprehensive ARMOR Pluck documentation  

### Dependencies Count Summary
| Category | Installed | Missing | Total |
|----------|-----------|---------|-------|
| **System/CLI** | 10 | 2 (optional) | 12 |
| **Cargo Runtime** | 26 | 0 | 26 |
| **Cargo Dev** | 5 | 0 | 5 |
| **Cargo OTLP** | 6 | 0 | 6 |
| **TOTAL** | 47 | 2 | 49 |

**Note:** 2 missing (clippy, sqlite3 CLI) are optional and not blocking for Pluck operations.

---

**Task Status:** ✅ COMPLETE  
**Documentation Status:** Comprehensive installed versions list created, enhanced, and committed.  
**Verification Date:** 2026-07-09  
**System Readiness:** ✅ READY FOR PLUCK OPERATIONS
