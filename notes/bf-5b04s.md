# Pluck Dependencies Verification Report

**Date:** 2026-07-09  
**Bead ID:** bf-5b04s  
**System:** NixOS  

## Overview

This report verifies the installation status of all documented Pluck/NEEDLE dependencies on the current system. The system is **NixOS**, not Debian/Ubuntu, so package management differs from the documented installation instructions.

## System Information

- **Operating System:** NixOS  
- **Package Manager:** Nix package manager  
- **Architecture:** Linux x86_64

## Dependency Status

### ✅ Installed Core Dependencies

| Dependency | Version | Installation Path | Status |
|------------|---------|-------------------|--------|
| `git` | 2.50.1 | `/run/current-system/sw/bin/git` | ✓ INSTALLED |
| `curl` | 8.14.1 | `/run/current-system/sw/bin/curl` | ✓ INSTALLED |
| `jq` | 1.7.1 | `/run/current-system/sw/bin/jq` | ✓ INSTALLED |
| `gcc` | 13.3.0 | `/home/coding/.nix-profile/bin/gcc` | ✓ INSTALLED |
| `g++` | 13.3.0 | `/home/coding/.nix-profile/bin/g++` | ✓ INSTALLED |
| `make` | 4.4.1 | `/home/coding/.nix-profile/bin/make` | ✓ INSTALLED |
| `pkg-config` | 0.29.2 | `/home/coding/.nix-profile/bin/pkg-config` | ✓ INSTALLED |
| `wget` | 1.25.0 | `/run/current-system/sw/bin/wget` | ✓ INSTALLED |
| `sha256sum` | coreutils 9.7 | `/run/current-system/sw/bin/sha256sum` | ✓ INSTALLED |
| `shasum` | 6.04 | `/run/current-system/sw/bin/shasum` | ✓ INSTALLED |
| `rustc` | 1.96.1 | `/home/coding/.cargo/bin/rustc` | ✓ INSTALLED |
| `cargo` | 1.96.1 | `/home/coding/.local/bin/cargo` | ✓ INSTALLED |

### ❌ Missing Dependencies

| Dependency | Purpose | Required? | Status |
|------------|---------|-----------|--------|
| `gh` | GitHub CLI for releases | Optional | ✗ NOT INSTALLED |
| `gpg` | GPG signature verification | Optional | ✗ NOT INSTALLED |

### 🔶 Special Case: OpenSSL Development Libraries

**Status:** Available in Nix store but not in current environment

- **Library versions in Nix store:**
  - openssl-3.3.3-dev
  - openssl-3.4.3-dev  
  - openssl-3.6.1-dev
  - openssl-3.6.2-dev
  - openssl-3.0.16-dev

- **Headers available:** Yes (include/openssl/*.h)
- **Libraries available:** Yes (lib/*.so*)

**Issue:** `pkg-config` cannot locate OpenSSL because it's not in the current environment's `PKG_CONFIG_PATH`. This is expected NixOS behavior - packages must be explicitly added to the build environment.

**Resolution for Rust builds:** Rust projects typically use the `rust-openssl` crate with `vendored` feature or use Nix shells to provide the development environment.

## Key Findings

1. **All functional dependencies are present** - The core build toolchain (gcc, make, pkg-config) and download tools (curl, wget) are installed and functional.

2. **Optional verification tools are mostly available** - Checksum tools (sha256sum, shasum) are installed; only GPG is missing.

3. **GitHub CLI (gh) is not installed** - This is only needed for release workflows, not for building or running Pluck/NEEDLE.

4. **OpenSSL dev libraries exist but need environment setup** - Multiple OpenSSL versions are available in the Nix store, but must be explicitly added to the build environment via `nix-shell` or similar.

5. **Rust toolchain is properly installed** - Rust 1.96.1 is available via cargo/rustc, which is sufficient for building NEEDLE/Pluck.

## Recommendations

### For Building Pluck/NEEDLE

1. **Use nix-shell for OpenSSL:** When building NEEDLE, use a nix-shell environment that includes OpenSSL dev libraries:
   ```bash
   nix-shell -p openssl pkg-config
   ```

2. **Or use vendored OpenSSL:** Many Rust projects use the `vendored` feature for `openssl-sys` to avoid system dependency issues.

3. **gh CLI is optional:** Only install if creating GitHub releases is needed:
   ```nix
   environment.systemPackages = [ gh ];
   ```

### For Release Workflows

If `gh` CLI is needed for releases:
```bash
nix-env -iA nixos.gh
```

## Conclusion

**Status:** ✅ Core Pluck/NEEDLE dependencies are installed and functional

All required build dependencies are present. The missing `gh` and `gpg` packages are optional and not required for building or running Pluck/NEEDLE. The OpenSSL development libraries are available in the Nix store and can be made available through standard NixOS environment setup methods.

## Verification Methods Used

1. **Command existence check:** `command -v <cmd>`
2. **Version query:** `<cmd> --version`
3. **Nix store search:** `find /nix/store -name "*openssl*"`
4. **Package availability check:** `pkg-config --exists openssl`
5. **File inspection:** Directory listings for library headers
