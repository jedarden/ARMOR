# Package Manager Detection Report

## System Type
**NixOS 25.05 (Warbler)** - Declarative Linux distribution using the Nix package manager

## Available Package Managers

### System Package Manager: Nix
- **nix**: v2.28.5
- **Location**: `/run/current-system/sw/bin/nix`
- **Related tools present**:
  - nix-env
  - nix-shell
  - nix-store
  - nix-channel
  - nix-build

NixOS uses Nix as its native package manager. Unlike traditional distributions (Debian/Ubuntu with apt, RHEL/CentOS with yum/dnf, Arch with pacman), NixOS manages all packages through the Nix expression language and declarative configuration.

### Language-Specific Package Managers

| Tool | Version | Location | Purpose |
|------|---------|----------|---------|
| **cargo** | 1.96.1 | `~/.local/bin/cargo` | Rust package manager and build tool |
| **npm** | 10.9.2 | `~/.nix-profile/bin/npm` | Node.js package manager |
| **go** | 1.25.0 | `~/.nix-profile/bin/go` | Go toolchain (includes module support) |

### Container Package Managers

| Tool | Version | Purpose |
|------|---------|---------|
| **docker** | 27.5.1 | Container image management |
| **podman** | 5.4.1 | Daemonless container engine (Docker-compatible) |

### Traditional Package Managers: NOT PRESENT

The following traditional Linux package managers are **not available** on this NixOS system:
- apt/apt-get (Debian/Ubuntu)
- yum/dnf (RHEL/CentOS/Fedora)
- pacman (Arch Linux)
- zypper (openSUSE)
- apk (Alpine Linux)
- brew (macOS/Linux)
- snap (Canonical)
- flatpak (universal Linux)

This is expected behavior for NixOS, which uses Nix exclusively for system package management.

### Build Tools Present

| Tool | Version | Purpose |
|------|---------|---------|
| **git** | 2.50.1 | Version control |
| **curl** | 8.14.1 | HTTP client |
| **wget** | 1.25.0 | HTTP/FTP client |
| **make** | 4.4.1 | Build automation |

### Language Package Managers: NOT PRESENT

The following language package managers are **not available**:
- pip/pip3 (Python)
- yarn (Node.js alternative)
- composer (PHP)
- gem (Ruby)
- luarocks (Lua)

## Summary

This is a **NixOS 25.05 system** with Nix (v2.28.5) as the primary package manager. Development tools for Rust (cargo), Node.js (npm), and Go are available. Both Docker and Podman are installed for container management. Traditional package managers (apt, yum, pacman, etc.) are not present, which is the expected and correct configuration for NixOS.

## Date
2026-07-09
