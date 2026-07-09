# Pluck Dependencies Inventory - Bead bf-42cj1

**Created:** 2026-07-09
**Bead ID:** bf-42cj1
**Task:** List installed Pluck dependencies with versions

---

## Executive Summary

This document provides a complete inventory of all installed Pluck/NEEDLE dependencies in the ARMOR workspace environment, including exact versions and specification locations.

**Key Finding:** All dependencies are current and meet minimum requirements.

---

## Core Development Tools

| Tool | Version | Minimum Required | Specification Location |
|------|---------|------------------|------------------------|
| Go | 1.25.0 linux/amd64 | 1.25.0 | `/home/coding/ARMOR/go.mod` |
| Rust | 1.96.1 (2026-06-26) | 1.75+ | `/home/coding/NEEDLE/rust-toolchain.toml` |
| Cargo | 1.96.1 (2026-06-26) | (with Rust) | `/home/coding/NEEDLE/rust-toolchain.toml` |
| Git | 2.50.1 | (system package) | System package manager |
| curl | 8.14.1 | (system package) | System package manager |
| jq | 1.7.1 | (system package) | System package manager |
| rustfmt | 1.96.1 | (with Rust) | `/home/coding/NEEDLE/rust-toolchain.toml` |
| clippy | 1.96.1 | (with Rust) | `/home/coding/NEEDLE/rust-toolchain.toml` |

---

## Pluck/NEEDLE Components

| Component | Version | Binary Location | Specification Location |
|-----------|---------|-----------------|------------------------|
| NEEDLE CLI | 0.2.11 | `~/.local/bin/needle` | `/home/coding/NEEDLE/Cargo.toml` |
| br CLI (bead-forge) | 0.2.0 | `~/.local/bin/br` | `~/bead-forge/Cargo.toml` |
| Pluck Strand | (part of NEEDLE) | `needle strand pluck` | `/home/coding/NEEDLE/src/strand/pluck.rs` |

---

## ARMOR Go Dependencies

### Direct Dependencies

| Dependency | Version | Purpose | Specification Location |
|------------|---------|---------|------------------------|
| `github.com/aws/aws-sdk-go-v2` | v1.41.4 | AWS SDK core | `/home/coding/ARMOR/go.mod:6` |
| `github.com/aws/aws-sdk-go-v2/config` | v1.32.12 | AWS config | `/home/coding/ARMOR/go.mod:7` |
| `github.com/aws/aws-sdk-go-v2/credentials` | v1.19.12 | AWS credentials | `/home/coding/ARMOR/go.mod:8` |
| `github.com/aws/aws-sdk-go-v2/service/s3` | v1.97.2 | S3 storage | `/home/coding/ARMOR/go.mod:9` |
| `github.com/kurin/blazer` | v0.5.3 | GCS storage | `/home/coding/ARMOR/go.mod:10` |
| `golang.org/x/crypto` | v0.49.0 | Cryptography | `/home/coding/ARMOR/go.mod:11` |
| `golang.org/x/sync` | v0.12.0 | Concurrency | `/home/coding/ARMOR/go.mod:12` |

### Transitive Dependencies (AWS SDK v2)

| Dependency | Version | Purpose | Specification Location |
|------------|---------|---------|------------------------|
| `github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream` | v1.7.8 | Event streaming | `/home/coding/ARMOR/go.sum` |
| `github.com/aws/aws-sdk-go-v2/feature/ec2/imds` | v1.18.20 | EC2 metadata | `/home/coding/ARMOR/go.sum` |
| `github.com/aws/aws-sdk-go-v2/internal/configsources` | v1.4.20 | Config sources | `/home/coding/ARMOR/go.sum` |
| `github.com/aws/aws-sdk-go-v2/internal/endpoints/v2` | v2.7.20 | Endpoint resolution | `/home/coding/ARMOR/go.sum` |
| `github.com/aws/aws-sdk-go-v2/internal/ini` | v1.8.6 | INI parsing | `/home/coding/ARMOR/go.sum` |
| `github.com/aws/aws-sdk-go-v2/internal/v4a` | v1.4.21 | SigV4A signing | `/home/coding/ARMOR/go.sum` |
| `github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding` | v1.13.7 | Accept encoding | `/home/coding/ARMOR/go.sum` |
| `github.com/aws/aws-sdk-go-v2/service/internal/checksum` | v1.9.12 | Checksum handling | `/home/coding/ARMOR/go.sum` |
| `github.com/aws/aws-sdk-go-v2/service/internal/presigned-url` | v1.13.20 | Presigned URLs | `/home/coding/ARMOR/go.sum` |
| `github.com/aws/aws-sdk-go-v2/service/internal/s3shared` | v1.19.20 | S3 utilities | `/home/coding/ARMOR/go.sum` |
| `github.com/aws/aws-sdk-go-v2/service/signin` | v1.0.8 | AWS sign-in | `/home/coding/ARMOR/go.sum` |
| `github.com/aws/aws-sdk-go-v2/service/sso` | v1.30.13 | AWS SSO | `/home/coding/ARMOR/go.sum` |
| `github.com/aws/aws-sdk-go-v2/service/ssooidc` | v1.35.17 | AWS SSO OIDC | `/home/coding/ARMOR/go.sum` |
| `github.com/aws/aws-sdk-go-v2/service/sts` | v1.41.9 | AWS STS | `/home/coding/ARMOR/go.sum` |
| `github.com/aws/smithy-go` | v1.24.2 | Smithy protocol | `/home/coding/ARMOR/go.sum` |
| `golang.org/x/net` | v0.51.0 | Network utilities | `/home/coding/ARMOR/go.sum` |
| `golang.org/x/sys` | v0.42.0 | System interfaces | `/home/coding/ARMOR/go.sum` |
| `golang.org/x/term` | v0.41.0 | Terminal handling | `/home/coding/ARMOR/go.sum` |
| `golang.org/x/text` | v0.35.0 | Text processing | `/home/coding/ARMOR/go.sum` |

---

## Pluck Strand Runtime Dependencies

The Pluck strand is part of NEEDLE and has the following runtime requirements:

| Component | Purpose | Location |
|-----------|---------|----------|
| Bead Store (SQLite) | Bead persistence | `.beads/beads.db` |
| Bead Checkpoint (JSONL) | Bead serialization | `.beads/issues.jsonl` |
| Configuration | Pluck strand config | `/home/coding/ARMOR/.needle.yaml` |
| br CLI | Bead management | `~/.local/bin/br` |

### Pluck Configuration

```yaml
# Location: /home/coding/ARMOR/.needle.yaml
strands:
  pluck:
    exclude_labels: []
    split_after_failures: 0
```

---

## Verification Commands

**Verify all installed versions:**
```bash
# Core tools
echo "=== Core Tools ===" && \
go version && \
rustc --version && \
cargo --version && \
git --version && \
curl --version | head -1 && \
jq --version

# NEEDLE components
echo "" && echo "=== NEEDLE Components ===" && \
needle --version && \
br --version 2>&1

# ARMOR dependencies
echo "" && echo "=== ARMOR Dependencies ===" && \
go list -m all | head -20
```

---

## Compliance Status

✅ **All dependencies meet minimum requirements**
✅ **All specification files documented**
✅ **All versions verified and current**

---

## Dependency Specification File Locations

| Component | Specification File | Purpose |
|-----------|-------------------|---------|
| ARMOR (Go) | `/home/coding/ARMOR/go.mod` | ARMOR direct dependencies |
| ARMOR (Go) | `/home/coding/ARMOR/go.sum` | ARMOR transitive dependencies |
| NEEDLE (Rust) | `/home/coding/NEEDLE/Cargo.toml` | NEEDLE/Pluck dependencies |
| NEEDLE (Rust) | `/home/coding/NEEDLE/Cargo.lock` | NEEDLE locked versions |
| NEEDLE (Rust) | `/home/coding/NEEDLE/rust-toolchain.toml` | Rust toolchain requirements |
| Pluck Config | `/home/coding/ARMOR/.needle.yaml` | Pluck strand configuration |
| Bead Store | `.beads/beads.db` | Bead persistence (SQLite) |
| Bead Store | `.beads/issues.jsonl` | Bead checkpoint (JSONL) |

---

## Completion Notes

**Bead bf-42cj1 (2026-07-09):**
✅ **Task Completed:** All Pluck/NEEDLE/ARMOR dependencies have been inventoried with exact versions and specification locations.

**Acceptance Criteria Met:**
- ✅ All installed dependencies listed with exact versions
- ✅ All dependencies meet minimum requirements
- ✅ Specification file locations documented
- ✅ Runtime dependencies for Pluck strand documented
- ✅ Verification commands provided

**Key Findings:**
- All core development tools are current
- All Go dependencies are at stable versions
- All NEEDLE/Pluck components are operational
- Comprehensive documentation exists in `/home/coding/ARMOR/pluck-dependency-requirements.md`

**Related Documentation:**
- Full dependency requirements: `/home/coding/ARMOR/pluck-dependency-requirements.md`
- NEEDLE project: `/home/coding/NEEDLE/`
- ARMOR project: `/home/coding/ARMOR/`
