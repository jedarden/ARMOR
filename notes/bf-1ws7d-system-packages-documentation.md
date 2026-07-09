# System Packages and Installation Paths Documentation

**Bead:** bf-1ws7d  
**Date:** 2026-07-09  
**Project:** ARMOR (Authenticated Range-readable Managed Object Repository)  
**Workspace:** /home/coding/ARMOR

## System Information

### Operating System
- **OS:** NixOS 25.05 (Warbler)
- **Version ID:** 25.05
- **Build ID:** 25.05.20260102.ac62194
- **Kernel:** Linux 6.12.63
- **Architecture:** linux/amd64

### Package Managers

#### Nix Package Manager
- **Version:** 2.28.5
- **Binary Path:** /run/current-system/sw/bin/nix-channel
- **Profile Path:** /home/coding/.nix-profile/
- **Store Path:** /nix/store/
- **Total Profile Binaries:** 106

#### Go Modules
- **Go Version:** go1.25.0 (but using go1.23.8 binary from Nix)
- **Binary Path:** /home/coding/.nix-profile/bin/go
- **Actual Binary:** /nix/store/41y1amjfxmrrz3knk09ndrh3iacfz2sv-go-1.23.8/share/go/bin/go
- **GOROOT:** /home/coding/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.0.linux-amd64
- **GOPATH:** /home/coding/go

## Development Tools

### Version Control
- **Git:**
  - Version: 2.50.1
  - Path: /run/current-system/sw/bin/git
  - Type: System binary

### Build Tools
- **Go:**
  - Version: go1.25.0
  - Path: /home/coding/.nix-profile/bin/go
  - Type: Nix profile package

- **GNU Make:**
  - Version: 4.4.1
  - Path: /home/coding/.nix-profile/bin/make
  - Type: Nix profile package
  - License: GPLv3+

- **Cargo (Rust):**
  - Version: 1.96.1 (356927216 2026-06-26)
  - Path: /home/coding/.local/bin/cargo
  - Type: Cargo installed in home directory

- **Rustc (Rust Compiler):**
  - Version: 1.96.1 (31fca3adb 2026-06-26)
  - Path: /home/coding/.cargo/bin/rustc
  - Type: Cargo installed in home directory

### Container Tools
- **Docker:**
  - Version: 27.5.1 (build v27.5.1)
  - Path: /home/coding/.nix-profile/bin/docker
  - Type: Nix profile package

- **kubectl:**
  - Version: (installed but version check failed)
  - Path: /home/coding/.nix-profile/bin/kubectl
  - Type: Nix profile package

### Web/Node.js Tools
- **Node.js:**
  - Version: v22.16.0
  - Path: /home/coding/.nix-profile/bin/node
  - Type: Nix profile package

- **npm:**
  - Version: 10.9.2
  - Path: /home/coding/.nix-profile/bin/npm
  - Type: Nix profile package

### Scripting Languages
- **Python 3:**
  - Version: 3.12.12
  - Path: /run/current-system/sw/bin/python3
  - Type: System binary
  - Additional Tools:
    - 2to3: /run/current-system/sw/bin/2to3
    - 2to3-3.12: /run/current-system/sw/bin/2to3-3.12

- **Bash:**
  - Version: 5.2.37(1)-release
  - Path: /run/current-system/sw/bin/bash
  - Type: System binary
  - Architecture: x86_64-pc-linux-gnu

### Network Tools
- **curl:**
  - Version: 8.14.1
  - Path: /run/current-system/sw/bin/curl
  - Type: System binary
  - Release Date: 2025-06-04
  - Supported Protocols: dict, file, ftp, ftps, gopher, gophers, http, https, imap, imaps, ipfs, ipns, mqtt, pop3, pop3s, rtsp, scp, sftp, smb, smbs, smtp, smtps, telnet, tftp
  - Features: alt-svc, AsynchDNS, brotli, GSS-API, HSTS, HTTP2, HTTPS-proxy, IDN, IPv6, Kerberos, Largefile, libz, NTLM, PSL, SPNEGO, SSL, threadsafe, TLS-SRP, UnixSockets, zstd

### Mobile/Android Tools
- **Android Debug Bridge (adb):**
  - Version: 35.0.1
  - Path: /home/coding/.nix-profile/bin/adb
  - Type: Nix profile package
  - Package: android-tools-35.0.1
  - Additional Tools:
    - append2simg: /home/coding/.nix-profile/bin/append2simg
    - avbtool: /home/coding/.nix-profile/bin/avbtool

### Tools Not Found/Not Installed
- **openssl:** Not in PATH
- **sqlite3:** Not in PATH  
- **AWS CLI:** Not installed in standard locations

## Go Module Dependencies (ARMOR Project)

### Direct Dependencies
- **github.com/aws/aws-sdk-go-v2** v1.41.4
- **github.com/aws/aws-sdk-go-v2/config** v1.32.12
- **github.com/aws/aws-sdk-go-v2/credentials** v1.19.12
- **github.com/aws/aws-sdk-go-v2/service/s3** v1.97.2
- **github.com/kurin/blazer** v0.5.3 (Backblaze B2 client)
- **golang.org/x/crypto** v0.49.0
- **golang.org/x/sync** v0.12.0

### AWS SDK Dependencies
- **github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream** v1.7.8
- **github.com/aws/aws-sdk-go-v2/feature/ec2/imds** v1.18.20
- **github.com/aws/aws-sdk-go-v2/internal/configsources** v1.4.20
- **github.com/aws/aws-sdk-go-v2/internal/endpoints/v2** v2.7.20
- **github.com/aws/aws-sdk-go-v2/internal/ini** v1.8.6
- **github.com/aws/aws-sdk-go-v2/internal/v4a** v1.4.21
- **github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding** v1.13.7
- **github.com/aws/aws-sdk-go-v2/service/internal/checksum** v1.9.12
- **github.com/aws/aws-sdk-go-v2/service/internal/presigned-url** v1.13.20
- **github.com/aws/aws-sdk-go-v2/service/internal/s3shared** v1.19.20
- **github.com/aws/aws-sdk-go-v2/service/signin** v1.0.8
- **github.com/aws/aws-sdk-go-v2/service/sso** v1.30.13
- **github.com/aws/aws-sdk-go-v2/service/ssooidc** v1.35.17
- **github.com/aws/aws-sdk-go-v2/service/sts** v1.41.9
- **github.com/aws/smithy-go** v1.24.2

### Golang.org Dependencies
- **golang.org/x/net** v0.51.0
- **golang.org/x/sys** v0.42.0
- **golang.org/x/term** v0.41.0
- **golang.org/x/text** v0.35.0

## Configuration Locations

### Go Configuration
- **GOROOT:** /home/coding/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.0.linux-amd64
- **GOPATH:** /home/coding/go
- **Go Module Cache:** /home/coding/go/pkg/mod/
- **ARMOR Project:** /home/coding/ARMOR

### Cargo/Rust Configuration
- **Cargo Home:** /home/coding/.cargo/
- **Rust Toolchain:** /home/coding/.cargo/bin/

### Git Configuration
- **Git User:** jedarden
- **Default Branch:** main

### Docker Configuration
- **Docker Hub Repository:** ronaldraygun/armor
- **Current Version Tag:** (from VERSION file)

## Package Management Notes

### NixOS-Specific Considerations
- System uses NixOS package management with rollback capabilities
- User-specific packages installed in `/home/coding/.nix-profile/`
- System packages in `/run/current-system/sw/bin/`
- All packages are symlinked to `/nix/store/` with content-addressable paths

### Go Module Management
- Uses Go 1.25.0 toolchain via go.mod requirement
- Project uses standard Go module structure
- Dependencies are vendored in go.sum for reproducibility
- Module: github.com/jedarden/armor

### Development Environment
- Primary development language: Go (1.25.0)
- Secondary languages: Rust (1.96.1), Python (3.12.12), Node.js (v22.16.0)
- Container support: Docker (27.5.1)
- Version control: Git (2.50.1)
- Build automation: GNU Make (4.4.1)

## System Package Inventory Summary

### Total Identified Packages: 30+
- **Development Languages:** Go, Rust, Python, Node.js, Bash
- **Build Tools:** Make, Cargo, Go modules
- **Version Control:** Git
- **Container Tools:** Docker, kubectl
- **Network Tools:** curl
- **Mobile Development:** Android SDK tools
- **Package Managers:** Nix, Go modules, Cargo, npm

### Package Installation Types
1. **System Packages:** 1 (managed by NixOS configuration)
2. **Nix Profile Packages:** 106 total binaries in user profile
3. **Local Installations:** Cargo (Rust toolchain)
4. **Go Modules:** 24 direct and indirect dependencies

## Verification Status
✅ All major development tools identified and versioned  
✅ Installation paths documented  
✅ Package managers identified  
✅ Go module dependencies catalogued  
✅ Configuration locations noted  

---
**Documentation Generated:** 2026-07-09  
**Bead Completion Status:** In Progress  
**Next Steps:** Commit documentation and close bead bf-1ws7d