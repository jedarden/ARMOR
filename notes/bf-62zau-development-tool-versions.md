# Development Tool Versions

## ARMOR Project (Current Workspace)

### Core Development Tools

| Tool | Version | Purpose | Source |
|------|---------|---------|--------|
| **Go** | 1.25.0 | Primary language | go.mod |
| **Docker** | 27.5.1 | Container builds | System |
| **kubectl** | 1.33.3 | Kubernetes deployment | System |
| **Kustomize** | 5.6.0 (bundled) | Kubernetes customization | kubectl |
| **git** | 2.50.1 | Version control | System |

### Container Build Environment

| Component | Version | Configuration |
|-----------|---------|--------------|
| **Base Image** | golang:1.25-alpine | Dockerfile |
| **Build Dependencies** | git, ca-certificates, tzdata | Alpine packages |

### Deployment Tools

| Tool | Version | Usage |
|------|---------|-------|
| **Kubernetes** | 1.33.3 (client) | Deployment target |
| **Kustomize** | v5.6.0 | Configuration management |

---

## NEEDLE Project (Pluck Component)

### Rust Toolchain

| Component | Version | Requirement | Source |
|-----------|---------|-------------|--------|
| **rustc** | 1.96.1 | MSRV: 1.75+ | rust-toolchain.toml |
| **cargo** | 1.96.1 | Package manager | System |
| **rustfmt** | 1.9.0-stable | Code formatting | rust-toolchain.toml |
| **clippy** | 0.1.96 | Linting | rust-toolchain.toml |

### Toolchain Configuration

```toml
[toolchain]
channel = "stable"
components = ["rustfmt", "clippy"]
targets = ["x86_64-unknown-linux-gnu", "aarch64-apple-darwin"]
```

### System Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| **git** | 2.50.1 | Version control |
| **curl** | - | HTTP client |
| **jq** | 1.7.1 | JSON processing |
| **build-essential** | - | C compiler and build tools |
| **pkg-config** | - | Package configuration |
| **libssl-dev** | - | OpenSSL headers |

### CI/CD Tools

| Tool | Version | Platform | Source |
|------|---------|----------|--------|
| **GitHub Actions** | - | CI/CD | .github/workflows/ci.yml |
| **musl-tools** | - | Static linking (Linux) | release.yml |
| **GPG** | - | Release signing | release.yml |

### Cross-Compilation Targets

| Target | Platform | Build Status |
|--------|----------|--------------|
| x86_64-unknown-linux-gnu | Linux x86_64 | ✅ Active |
| x86_64-unknown-linux-musl | Linux x86_64 (static) | ✅ Release |
| aarch64-apple-darwin | macOS ARM64 | ✅ Release |

---

## Version Requirements Summary

### ARMOR Project
- **Go Version Required**: 1.25.0 (minimum)
- **Kubernetes API**: Compatible with 1.33.3 client
- **Docker**: Version 27.5+ recommended

### NEEDLE/Pluck Project
- **Rust Version Required**: 1.75+ (MSRV)
- **Current Stable**: 1.96.1
- **Cargo**: 1.96.1
- **Supports**: Linux (x86_64), macOS (ARM64)

---

## Tool Installation Commands

### Go (for ARMOR)
```bash
# Install Go 1.25
wget https://go.dev/dl/go1.25.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.25.0.linux-amd64.tar.gz
```

### Rust (for NEEDLE/Pluck)
```bash
# Install Rust stable
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --default-toolchain stable
rustc --version
```

### System Dependencies
```bash
# Debian/Ubuntu
sudo apt-get install git curl jq build-essential pkg-config libssl-dev

# macOS
# Standard development environment is sufficient
```

---

## Compatibility Matrix

| Tool | ARMOR | NEEDLE/Pluck | Notes |
|------|-------|--------------|-------|
| Go | ✅ 1.25.0 | ❌ N/A | ARMOR only |
| Rust | ❌ N/A | ✅ 1.75+ | NEEDLE only |
| Docker | ✅ Required | ❌ N/A | ARMOR builds |
| kubectl | ✅ Required | ❌ N/A | ARMOR deployment |
| git | ✅ 2.50.1 | ✅ 2.50.1 | Both projects |
| jq | ✅ Optional | ✅ Optional | Script utilities |

---

## Verification Commands

### ARMOR Environment
```bash
# Verify Go installation
go version
# Expected: go version go1.25.0 linux/amd64

# Verify Docker
docker --version
# Expected: Docker version 27.5.1, build v27.5.1

# Verify kubectl
kubectl version --client
# Expected: Client Version: v1.33.3
```

### NEEDLE/Pluck Environment
```bash
# Verify Rust installation
rustc --version
# Expected: rustc 1.96.1 (31fca3adb 2026-06-26)

# Verify Cargo
cargo --version
# Expected: cargo 1.96.1 (356927216 2026-06-26)

# Verify rustfmt
rustfmt --version
# Expected: rustfmt 1.9.0-stable (31fca3adb2 2026-06-26)

# Verify clippy
cargo clippy --version
# Expected: clippy 0.1.96 (31fca3adb2 2026-06-26)
```

---

*Document generated: 2026-07-09*
*Bead ID: bf-62zau*
