# Pluck System Package Dependencies

**Bead:** bf-12cyf  
**Created:** 2026-07-09  
**Scope:** Locate and document Pluck system-level package dependencies

## Summary

Pluck is a strand (component) within the NEEDLE project, written in Rust. The system package dependencies are primarily build-time dependencies required to compile the NEEDLE binary, which includes the Pluck strand.

## Dependency List Location

The primary system package dependency definitions are located in:

- **Build Container:** `/home/coding/NEEDLE/ci/Dockerfile.ci`
- **Release Workflow:** `/home/coding/NEEDLE/.github/workflows/release.yml`
- **Installation Script:** `/home/coding/NEEDLE/install.sh`

## System Package Dependencies

### Package Manager: APT (Debian/Ubuntu)

The following system packages are required for building Pluck/NEEDLE on Debian-based systems:

#### Build Dependencies

| Package | Purpose | Required For |
|---------|---------|--------------|
| `git` | Version control | Cloning source repository |
| `curl` | HTTP client | Downloading installers, Rust toolchain, GitHub releases |
| `jq` | JSON processor | Parsing GitHub API responses |
| `build-essential` | Meta-package for compilation | GCC, g++, make, and other build tools |
| `pkg-config` | Package configuration tool | Finding library compile/link flags |
| `libssl-dev` | OpenSSL development headers | Cryptographic library support in Rust dependencies |
| `musl-tools` | musl libc toolchain | Static binary compilation (release builds only) |

#### GitHub CLI (Optional but Recommended)

| Package | Purpose | Required For |
|---------|---------|--------------|
| `gh` | GitHub CLI | Release automation, integration with GitHub workflows |

**Installation command:**
```bash
sudo apt-get update && sudo apt-get install -y \
    git curl jq build-essential pkg-config libssl-dev \
    && sudo rm -rf /var/lib/apt/lists/*
```

## Runtime Requirements

### Operating System Support

- **Linux:** Any distribution with glibc (tested on Ubuntu, Debian)
- **macOS:** Both x86_64 and ARM64 (Apple Silicon)
- **Architecture:** x86_64 (Intel/AMD), aarch64 (ARM64)

### Minimum System Versions

| Component | Minimum Version | Recommended Version |
|-----------|----------------|-------------------|
| Rust | 1.75+ (MSRV) | Latest stable |
| SQLite | 3.38+ | Latest system version |

### Runtime Binaries

Pluck/NEEDLE is distributed as precompiled static binaries and does not require system packages at runtime beyond standard C library support (glibc on Linux).

## Dependency Context Files

### 1. Build Container (ci/Dockerfile.ci)

```dockerfile
FROM debian:bookworm

# System deps
RUN apt-get update && apt-get install -y \
    git curl jq build-essential pkg-config libssl-dev \
    && rm -rf /var/lib/apt/lists/*
```

### 2. Release Build (.github/workflows/release.yml)

Static binary compilation requires:
```bash
sudo apt-get install -y musl-tools
```

### 3. Installation Script (install.sh)

The installer checks for and uses:
- `curl` or `wget` (for downloading releases)
- `sha256sum` or `shasum` (for checksum verification)
- `gpg` (optional, for GPG signature verification)

## No System Package Manager Dependencies for Pluck Strand

**Important finding:** Pluck is a Rust component within NEEDLE (`/home/coding/NEEDLE/src/strand/pluck.rs`) and does **not** have its own separate system package dependencies. All system dependencies are shared at the NEEDLE project level and are primarily build-time dependencies for compiling the Rust binary.

## Summary Table

| Package Manager | Packages | Use Case |
|----------------|----------|----------|
| **APT (Debian/Ubuntu)** | `git`, `curl`, `jq`, `build-essential`, `pkg-config`, `libssl-dev`, `musl-tools` | Build-time dependencies for compiling NEEDLE/Pluck |
| **N/A (runtime)** | None (static binary) | Runtime execution (glibc only) |

## Package Manager Identification

- **APT:** Primary package manager for Debian-based distributions (Ubuntu, Debian)
- **DNF/YUM:** Not directly used, but equivalent packages would be: `git`, `curl`, `jq`, `gcc-c++`, `make`, `pkg-config`, `openssl-devel`, `musl-tools`
- **Pacman (Arch):** Equivalent packages: `git`, `curl`, `jq`, `base-devel`, `pkg-config`, `openssl`, `musl`
- **APK (Alpine):** Equivalent packages: `git`, `curl`, `jq`, `build-base`, `pkgconfig`, `openssl-dev`, `musl-dev`

## Verification

All dependencies were verified by examining:
1. CI build container definition (`ci/Dockerfile.ci`)
2. GitHub Actions release workflow (`.github/workflows/release.yml`)
3. Installation script (`install.sh`)
4. NEEDLE project documentation and source code structure
