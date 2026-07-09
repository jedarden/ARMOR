# System Dependencies Verification Report

**Bead:** bf-34avz  
**Created:** 2026-07-09  
**Scope:** Verify installation status for each Pluck/NEEDLE system dependency  
**System:** NixOS (identified via `/run/current-system/sw/bin` paths)

## Executive Summary

**Package Manager Detected:** NixOS (not APT/Debian-based)  
**Core Build Dependencies:** ✓ INSTALLED (8/8)  
**Optional Tools:** ✗ MISSING (2/2)  
**Runtime Dependencies:** ✗ PARTIAL (1 missing, but non-blocking)

## Detailed Installation Status

### Core Build Dependencies (REQUIRED)

| Dependency | Status | Version | Location | Notes |
|------------|--------|---------|----------|-------|
| `git` | ✓ INSTALLED | 2.50.1 | `/run/current-system/sw/bin/git` | Version control for cloning source |
| `curl` | ✓ INSTALLED | 8.14.1 | `/run/current-system/sw/bin/curl` | HTTP client for downloads |
| `jq` | ✓ INSTALLED | 1.7.1 | `/run/current-system/sw/bin/jq` | JSON processor for GitHub API |
| `gcc` | ✓ INSTALLED | 13.3.0 | System-managed | C compiler (part of build-essential equivalent) |
| `make` | ✓ INSTALLED | 4.4.1 | System-managed | Build automation tool |
| `pkg-config` | ✓ INSTALLED | 0.29.2 | `/home/coding/.nix-profile/bin/pkg-config` | Package configuration tool |
| `libssl-dev` (headers) | ✓ INSTALLED | Available | `/run/current-system/sw/include/openssl/` | OpenSSL development headers |
| `rustc` | ✓ INSTALLED | 1.96.1 | System-managed | Rust compiler (exceeds MSRV 1.75+) |

### Optional Tools (RECOMMENDED but not REQUIRED)

| Dependency | Status | Version | Location | Notes |
|------------|--------|---------|----------|-------|
| `gh` (GitHub CLI) | ✗ NOT FOUND | N/A | N/A | Optional: for GitHub release automation |
| `musl-gcc` (musl tools) | ✗ NOT FOUND | N/A | N/A | Optional: for static binary compilation |

### Runtime Dependencies (NON-BLOCKING)

| Dependency | Status | Version | Location | Notes |
|------------|--------|---------|----------|-------|
| `sqlite3` | ✗ NOT FOUND | N/A | N/A | Runtime only: br CLI brings its own SQLite |

## Package Manager Detection Results

**Detected Package Manager:** NixOS

**Evidence:**
- System paths use `/run/current-system/sw/bin/` structure
- `nix-store` binary available at `/run/current-system/sw/bin/nix-store`
- No `dpkg` or `apt` commands available
- User packages in `/home/coding/.nix-profile/bin/`

**Implications:**
- Documentation referencing `apt-get install` does not apply to this system
- Packages are managed declaratively via Nix configurations
- Build tools are provided via NixOS system closure

## Dependency vs. Documentation Mapping

The original documentation (bf-12cyf) listed Debian/Ubuntu APT packages:

| APT Package | NixOS Equivalent | Status |
|-------------|------------------|--------|
| `git` | `git` | ✓ Available |
| `curl` | `curl` | ✓ Available |
| `jq` | `jq` | ✓ Available |
| `build-essential` | `gcc`, `make` | ✓ Available |
| `pkg-config` | `pkg-config` | ✓ Available |
| `libssl-dev` | `openssl.dev` | ✓ Available (headers present) |
| `musl-tools` | `musl` | ✗ Not available (optional) |

## Missing Packages Impact Assessment

### Non-Blocking Missing Dependencies

1. **sqlite3** - Not a blocker because:
   - The `br` CLI distributes its own embedded SQLite
   - Runtime dependency only, not build-time
   - Does not prevent compilation or execution of Pluck/NEEDLE

2. **gh (GitHub CLI)** - Not a blocker because:
   - Marked as "optional but recommended" in documentation
   - Only needed for release automation workflows
   - Does not affect building or running NEEDLE/Pluck

3. **musl-tools** - Not a blocker because:
   - Only required for static binary compilation (release builds)
   - Dynamic linking works fine for development builds
   - Does not affect standard compilation

## Conclusion

✅ **All required build dependencies are installed and functional.**

The NixOS system has all core tools needed to compile NEEDLE/Pluck from source:
- Version control (git)
- Build toolchain (gcc, make)
- Package discovery (pkg-config)
- Cryptographic support (OpenSSL headers)
- Rust toolchain (rustc 1.96.1, exceeds MSRV 1.75)
- Download and JSON parsing tools (curl, jq)

The missing packages (sqlite3, gh, musl-tools) are either optional or have alternatives:
- SQLite embedded in br CLI
- GitHub CLI only for release automation
- musl-tools only for static linking (optional)

**Recommendation:** No action required. The system is ready to build and run Pluck/NEEDLE.

## Verification Method

Packages were verified using:
1. `command -v <binary>` to check availability in PATH
2. `--version` flags to get version numbers
3. Directory checks for header files (`/run/current-system/sw/include/openssl/`)
4. Compiler checks (`gcc --version`, `rustc --version`)

## Next Steps

Since all required dependencies are present, the following tasks can proceed:
1. ✓ Clone NEEDLE repository (git available)
2. ✓ Build from source (gcc, make, pkg-config, OpenSSL headers, Rust toolchain)
3. ✓ Run tests (all dependencies present)
4. ✓ Install br CLI via cargo (Rust available)

---

**Related Beads:**
- bf-12cyf: System package dependency documentation
- bf-29m3g: Complete Pluck dependency documentation
- bf-34avz: This verification report
