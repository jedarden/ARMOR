# Package Manager Detection Report

## System Information
- **OS:** NixOS 25.05 (Warbler)
- **Kernel:** Linux 6.12.63
- **Architecture:** linux/amd64

## Primary Package Manager

### Nix (2.28.5)
NixOS uses the Nix package manager as its primary package management system. All Nix tools are available:

| Command | Path |
|---------|------|
| nix | `/run/current-system/sw/bin/nix` |
| nix-env | `/run/current-system/sw/bin/nix-env` |
| nix-store | `/run/current-system/sw/bin/nix-store` |
| nix-channel | `/run/current-system/sw/bin/nix-channel` |
| nix-build | `/run/current-system/sw/bin/nix-build` |
| nix-shell | `/run/current-system/sw/bin/nix-shell` |
| nix-collect-garbage | `/run/current-system/sw/bin/nix-collect-garbage` |
| nixos-rebuild | `/run/current-system/sw/bin/nixos-rebuild` |

## Language Package Managers

| Language | Package Manager | Version |
|----------|-----------------|---------|
| Rust | cargo | 1.96.1 |
| Node.js | npm | 10.9.2 |
| Go | go modules | 1.25.0 |

## Traditional Package Managers: Not Available

The following traditional package managers are **NOT** available on this NixOS system:
- `apt` / `apt-get` (Debian/Ubuntu)
- `yum` (RHEL/CentOS 6-)
- `dnf` (RHEL/CentOS 8+, Fedora)
- `pacman` (Arch Linux)
- `zypper` (openSUSE)
- `apk` (Alpine Linux)
- `opkg` (OpenWrt)
- `emerge` (Gentoo)

## Universal Package Systems: Not Available

The following universal package systems are **NOT** available:
- Flatpak
- Snap
- Homebrew (brew)

## Notes

NixOS is a purely functional Linux distribution that uses Nix as its package manager. Unlike traditional distributions, NixOS does not use conventional package managers like apt or yum. Instead, it uses declarative configuration through Nix expressions and the Nix package manager.

Language-specific package managers (cargo, npm, go) are available and work as expected within the Nix ecosystem.
