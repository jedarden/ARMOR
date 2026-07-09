# Pluck System Package Dependencies

## Overview

**Pluck** is not a standalone package - it is a strand (module) within the NEEDLE system. NEEDLE is a Rust-based universal wrapper for headless coding CLI agents. Pluck specifically handles primary bead selection from assigned workspaces, processing >90% of all bead work.

**Repository:** `/home/coding/NEEDLE`  
**Pluck Source:** `/home/coding/NEEDLE/src/strand/pluck.rs`  
**Documentation:** [README.md](/home/coding/NEEDLE/README.md)

---

## System Package Dependencies

### 1. Build/Development Dependencies (Debian/Ubuntu)

Source: `/home/coding/NEEDLE/ci/Dockerfile.ci`

| Package | Package Manager | Purpose |
|---------|----------------|---------|
| `git` | apt | Version control |
| `curl` | apt | HTTP client for downloads |
| `jq` | apt | JSON processor |
| `build-essential` | apt | C compiler toolchain (gcc, g++, make) |
| `pkg-config` | apt | Compile-time dependency configuration |
| `libssl-dev` | apt | OpenSSL development headers |
| `gh` | apt (GitHub CLI) | GitHub CLI tool for releases |

**Installation Command (Debian/Ubuntu):**
```bash
apt-get update && apt-get install -y \
    git curl jq build-essential pkg-config libssl-dev
```

### 2. Runtime Dependencies (for binary distribution)

NEEDLE is distributed as pre-built static binaries, so **no runtime system packages are required** for end users. The binaries are completely self-contained.

**Installation (via install.sh):**
```bash
curl -fsSL https://github.com/jedarden/NEEDLE/releases/latest/download/install.sh | bash
```

### 3. Optional Dependencies

These are optional tools referenced in `install.sh` for verification:

| Package | Package Manager | Purpose | Required |
|---------|----------------|---------|-----------|
| `curl` | apt/yum/dnf/pacman | Downloading binaries | No (wget works) |
| `wget` | apt/yum/dnf/pacman | Alternative download tool | No (curl works) |
| `sha256sum` | coreutils (included) | Checksum verification | No |
| `shasum` | perl | Checksum verification (alternative) | No |
| `gpg` | apt/yum/dnf | GPG signature verification | No |

### 4. Special Plugin Dependencies

The `claude-interactive` plugin (separate from core Pluck functionality) requires:

| Dependency | Version | Purpose |
|------------|---------|---------|
| Python 3 | 3.10+ | Plugin runtime |
| `pyte` | pip install | Terminal PTY emulation |
| `claude` CLI | - | Claude Code CLI |

---

## Dependency Locations

### Primary Sources

1. **Dockerfile CI Base Image**: `/home/coding/NEEDLE/ci/Dockerfile.ci` (lines 4-6)
   - Defines core build dependencies

2. **Install Script**: `/home/coding/NEEDLE/install.sh` (lines 73-101)
   - Defines runtime download dependencies (curl/wget)
   - Optional verification tools (sha256sum, gpg)

3. **GitHub Actions CI**: `/.github/workflows/ci.yml` and `/.github/workflows/release.yml`
   - Uses ubuntu-latest and macos-latest runners
   - Requires `musl-tools` for static linking (release.yml line 27)

### Package Manager Types

| Platform | Package Manager | Dependency File |
|----------|----------------|-----------------|
| Debian/Ubuntu | `apt` | Dockerfile.ci (lines 4-6) |
| Fedora/RHEL | `yum` / `dnf` | Not explicitly documented (equivalent packages needed) |
| Arch Linux | `pacman` | Not explicitly documented (equivalent packages needed) |
| Alpine Linux | `apk` | Not explicitly documented (equivalent packages needed) |
| macOS | Homebrew | Not documented (uses pre-built binaries) |

---

## Key Findings

1. **Pluck is a Rust module, not a standalone package** - Dependencies are for the entire NEEDLE system, not Pluck specifically.

2. **No runtime system packages required** - NEEDLE/Pluck ships as static binaries with zero runtime dependencies.

3. **Build dependencies are minimal** - Only standard C build toolchain (build-essential, pkg-config, libssl-dev) plus git/curl/jq for development workflows.

4. **Multi-platform support** - Pre-built binaries available for:
   - Linux x86_64 (static musl-linked)
   - macOS ARM64 (aarch64-apple-darwin)

5. **Installation is self-contained** - The install.sh script downloads static binaries that require no system packages to run.

---

## Verification

All dependency information has been verified from the following source files:

- ✅ `/home/coding/NEEDLE/ci/Dockerfile.ci` - Build dependencies
- ✅ `/home/coding/NEEDLE/install.sh` - Installation script
- ✅ `/home/coding/NEEDLE/src/strand/pluck.rs` - Pluck source code
- ✅ `/home/coding/NEEDLE/README.md` - Project documentation
- ✅ `/home/coding/NEEDLE/.github/workflows/*.yml` - CI/CD configurations
