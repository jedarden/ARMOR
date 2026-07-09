# ARMOR Project Dependencies Inventory

**Generated:** 2026-07-09  
**Project:** github.com/jedarden/armor  
**Go Version:** 1.25.0  
**Task:** bf-42cj1 - List installed Pluck dependencies with versions

## Overview

This document provides a comprehensive inventory of all currently installed Go dependencies in the ARMOR project, including exact versions, checksums, and specification locations.

## Direct Dependencies

The following dependencies are directly required by the ARMOR project:

| Dependency | Version | Purpose | Specification Location |
|------------|---------|---------|------------------------|
| github.com/aws/aws-sdk-go-v2 | v1.41.4 | AWS SDK for Go v2 | go.mod:6 |
| github.com/aws/aws-sdk-go-v2/config | v1.32.12 | AWS SDK configuration | go.mod:7 |
| github.com/aws/aws-sdk-go-v2/credentials | v1.19.12 | AWS SDK credentials | go.mod:8 |
| github.com/aws/aws-sdk-go-v2/service/s3 | v1.97.2 | AWS S3 service client | go.mod:9 |
| github.com/kurin/blazer | v0.5.3 | Google Cloud Storage client | go.mod:10 |
| golang.org/x/crypto | v0.49.0 | Cryptography utilities | go.mod:11 |
| golang.org/x/sync | v0.12.0 | Synchronization primitives | go.mod:12 |

## Indirect Dependencies

The following dependencies are transitive dependencies required by the direct dependencies:

| Dependency | Version | Parent Dependency | Purpose |
|------------|---------|-------------------|---------|
| github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream | v1.7.8 | aws-sdk-go-v2 | Event stream protocol |
| github.com/aws/aws-sdk-go-v2/feature/ec2/imds | v1.18.20 | aws-sdk-go-v2/config | EC2 Instance Metadata Service |
| github.com/aws/aws-sdk-go-v2/internal/configsources | v1.4.20 | aws-sdk-go-v2/config | Internal configuration sources |
| github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 | v2.7.20 | aws-sdk-go-v2/config | Internal endpoint resolution |
| github.com/aws/aws-sdk-go-v2/internal/ini | v1.8.6 | aws-sdk-go-v2/config | INI parsing utilities |
| github.com/aws/aws-sdk-go-v2/internal/v4a | v1.4.21 | aws-sdk-go-v2/config | Internal v4 signing |
| github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding | v1.13.7 | aws-sdk-go-v2/service/s3 | Accept-encoding handling |
| github.com/aws/aws-sdk-go-v2/service/internal/checksum | v1.9.12 | aws-sdk-go-v2/service/s3 | Checksum utilities |
| github.com/aws/aws-sdk-go-v2/service/internal/presigned-url | v1.13.20 | aws-sdk-go-v2/service/s3 | Presigned URL handling |
| github.com/aws/aws-sdk-go-v2/service/internal/s3shared | v1.19.20 | aws-sdk-go-v2/service/s3 | Shared S3 utilities |
| github.com/aws/aws-sdk-go-v2/service/signin | v1.0.8 | aws-sdk-go-v2/config | AWS sign-in service |
| github.com/aws/aws-sdk-go-v2/service/sso | v1.30.13 | aws-sdk-go-v2/config | AWS SSO service |
| github.com/aws/aws-sdk-go-v2/service/ssooidc | v1.35.17 | aws-sdk-go-v2/config | AWS SSO OIDC service |
| github.com/aws/aws-sdk-go-v2/service/sts | v1.41.9 | aws-sdk-go-v2/config | AWS STS service |
| github.com/aws/smithy-go | v1.24.2 | aws-sdk-go-v2 | Smithy protocol utilities |

## Dependency File Locations

- **go.mod** - `/home/coding/ARMOR/go.mod` (main dependency specification)
- **go.sum** - `/home/coding/ARMOR/go.sum` (dependency checksums for verification)

## Dependency Analysis

### Cloud Storage Dependencies
- **AWS S3**: Primary storage backend via aws-sdk-go-v2/service/s3 (v1.97.2)
- **Google Cloud Storage**: Alternative backend via kurin/blazer (v0.5.3)

### AWS SDK Version 2 Components
The project uses AWS SDK v2 (not v1), with the following modular components:
- Core SDK: v1.41.4
- Configuration: v1.32.12  
- Credentials: v1.19.12
- S3 Service: v1.97.2

### Cryptography Support
- `golang.org/x/crypto` v0.49.0 - Provides additional cryptographic primitives beyond Go standard library

### Synchronization
- `golang.org/x/sync` v0.12.0 - Extended synchronization primitives

## Checksums (go.sum entries)

All dependencies include cryptographic checksums in go.sum for supply chain security:

```
github.com/aws/aws-sdk-go-v2 v1.41.4 h1:10f50G7WyU02T56ox1wWXq+zTX9I1zxG46HYuG1hH/k=
github.com/aws/aws-sdk-go-v2/config v1.32.12 h1:O3csC7HUGn2895eNrLytOJQdoL2xyJy0iYXhoZ1OmP0=
github.com/aws/aws-sdk-go-v2/credentials v1.19.12 h1:oqtA6v+y5fZg//tcTWahyN9PEn5eDU/Wpvc2+kJ4aY8=
github.com/aws/aws-sdk-go-v2/service/s3 v1.97.2 h1:MRNiP6nqa20aEl8fQ6PJpEq11b2d40b16sm4WD7QgMU=
github.com/kurin/blazer v0.5.3 h1:SAgYv0TKU0kN/ETfO5ExjNAPyMt2FocO2s/UlCHfjAk=
golang.org/x/crypto v0.49.0 h1:+Ng2ULVvLHnJ/ZFEq4KdcDd/cfjrrjjNSXNzxg0Y4U4=
golang.org/x/sync v0.12.0 h1:MHc5BpPuC30uJk597Ri8TV3CNZcTLu6B6z4lJy+g6Jw=
```

## Summary

- **Total Direct Dependencies:** 7
- **Total Indirect Dependencies:** 15  
- **Total Unique Dependencies:** 22
- **Specification Files:** 2 (go.mod, go.sum)
- **Go Version Requirement:** 1.25.0

All dependencies are properly specified with exact versions in go.mod and include cryptographic checksums in go.sum for reproducible builds and security verification.