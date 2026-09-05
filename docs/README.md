# ARMOR Documentation

This index organizes all durable documentation by audience. Every file under `docs/` (excluding `archive/`) is linked exactly once.

## Operate

**For operators running and maintaining ARMOR in production.**

- **[Release Process](release-process.md)** — Version bumping, container publishing, and deployment checklist
- **[Key Rotation Runbook](key-rotation-runbook.md)** — Step-by-step MEK rotation procedure with rollback guidance
- **[Unified Bucket Tenant Onboarding](runbooks/unified-bucket-tenant-onboarding.md)** — Adding a tenant to the shared ADR-001 bucket, prefix-scoped keys, per-tenant MEK, and the data move
- **[Disaster Recovery](disaster-recovery.md)** — MEK backup/escrow, restore drills, and recovery from rotation failures
- **[Drift Check](drift-check.md)** — Version drift detection between ARMOR instances and rollback coordination
- **[Connection Guide](connection-guide.md)** — Network topology, Cloudflare PNI setup, and connectivity troubleshooting
- **[Cloudflare Setup](cloudflare-setup.md)** — DNS and CDN configuration for zero-egress downloads
- **[Dashboard](dashboard.md)** — Web UI for bucket browsing, encryption status, and metrics
- **[Metrics](metrics.md)** — Prometheus metrics reference and alerting guidance
- **[Litestream Restore Procedure](litestream-restore-procedure-and-verification.md)** — Restoring PostgreSQL databases via Litestream through ARMOR
- **[Restore Verifier Deployment Guide](restore-verifier-deployment-guide.md)** — Deploying the continuous restore verification harness
- **[MEK Rotation Future Considerations](mek-rotation-future-considerations.md)** — Multi-key routing and per-prefix rotation design notes
- **[Provenance Audit Walker](provenance-audit-walker.md)** — Walking the cryptographic audit chain for integrity verification

## Design

**For architects and developers understanding ARMOR's design decisions.**

### Implementation Plan

- **[Plan](plan/plan.md)** — Complete implementation roadmap with phases and status

### Architecture Decision Records

| ADR | Title | Status |
|-----|-------|--------|
| [ADR-001](adr/001-bucket-prefix.md) | Shared Bucket via ARMOR_PREFIX | Accepted |
| [ADR-002](adr/002-multipart-corruption-detection-gaps.md) | Close multipart corruption detection gaps | Accepted |
| [ADR-003](adr/003-multipart-object-layout-and-read-path.md) | Multipart object layout and read-path dispatch | Accepted (superseded by ADR-005) |
| [ADR-004](adr/004-continuous-restore-verification.md) | Continuous dual-path restore verification | Accepted |
| [ADR-005](adr/005-ctr-counter-stride-fix.md) | AES-CTR Counter Stride Fix (Version 2) | Accepted |
| [ADR-006](adr/006-dual-backend-replication.md) | Dual-backend async replication for provider-outage resilience | Accepted |
| [ADR-007](adr/007-zstd-compression.md) | zstd Compression for Single-PUT Objects | Accepted |
| [ADR-007](adr/007-restore-verifier-discovery-reliability.md) | Restore-Verifier Discovery Reliability | Accepted |
| [ADR-008](adr/008-multipart-part-size-error-clarity.md) | Server-side observability for part-size rejections | Proposed |
| [ADR-009](adr/009-restore-verifier-armor-path-never-decrypts.md) | restore-verifier's ARMOR path never decrypts | Accepted |
| [ADR-010](adr/010-barman-multipart-incompatibility.md) | Barman multipart incompatibility | Superseded by ADR-011 |
| [ADR-011](adr/011-barman-stays-on-armor-non-uniform-multipart.md) | Barman stays on ARMOR; support non-uniform parts | Accepted |
| [ADR-012](adr/012-authorization-action-verbs-and-consumer-separation.md) | Authorization action verbs and consumer separation | Proposed |
| [ADR-013](adr/013-read-throughput-unpipelined-block-fetches.md) | Read throughput bounded by unpipelined block fetches | Proposed |
| [ADR-014](adr/014-restore-verifier-discovery-reliability.md) | Restore-verifier discovery reliability | Accepted |
| [ADR-015](adr/015-out-of-order-multipart-uniform-part-size.md) | Out-of-order multipart via uniform-part-size contract | Implemented |

### Format Specifications

- **[Envelope V3 Format](format/envelope-v3.md)** — Version 3 envelope specification (future format)
- **[Named Credential Provisioning Spec](named-credential-provisioning-spec.md)** — External credential provisioning interface design
- **[Decompression Verification API Design](decompression-verification-api-design.md)** — Zstd compression verification API

### Error Design

- **[Error Format](error-format.md)** — Standardized error response structure and severity levels
- **[Error Responses](error-responses.md)** — Consolidated error response documentation (formerly 7 separate docs)
- **[Error Response Inventory](error-response-inventory.md)** — Complete catalog of all ARMOR error responses
- **[ARMOR HTTP Status Codes](armor-http-status-codes.md)** — HTTP status code usage and semantics

### S3 Compliance

- **[S3 Compliance Comparison](s3-compliance-comparison.md)** — ARMOR's S3 API coverage vs. AWS S3

### Header Remediation

- **[Header Remediation Matrix](header-remediation-matrix.md)** — Metadata header migration matrix
- **[Header Remediation Plan](header-remediation-plan.md)** — Header cleanup and standardization plan

### Research

Background research and third-party analysis:

- **[Application Requirements](research/application-requirements.md)** — Application storage requirements analysis
- **[B2 Pricing and Features](research/b2-pricing-and-features.md)** — Backblaze B2 cost structure and feature comparison
- **[Bandwidth Alliance](research/bandwidth-alliance.md)** — Cloudflare Bandwidth Alliance research
- **[Barman ARMOR Root Cause Analysis](research/barman_armor_root_cause_analysis.md)** — Barman multipart failure investigation
- **[Cloudflare Architecture](research/cloudflare-architecture.md)** — Cloudflare CDN and PNI architecture
- **[DuckDB Encrypted Parquet](research/duckdb-encrypted-parquet.md)** — DuckDB query patterns over encrypted Parquet
- **[S3 Operation Surface](research/s3-operation-surface.md)** — S3 API operation inventory
- **[SDKs and Encryption](research/sdks-and-encryption.md)** — Client-side encryption vs. proxy-side encryption

### Notes

Contextual notes and temporary documentation:

- **[Corruption Inventory 2026-08](notes/corruption-inventory-2026-08.md)** — Multipart-era corruption audit findings
- **[Litestream Verified Generation ID](notes/litestream-verified-generation-id.md)** — Litestream restore verification notes

## Test

**For QA engineers validating ARMOR compatibility and correctness.**

- **[Integration Tests](../tests/integration/README.md)** — Real B2 + Cloudflare integration test suite
- **[AWS CLI Compatibility Tests](../tests/aws-cli-compatibility/README.md)** — AWS CLI and rclone compatibility verification

## Archive

**Historical documentation retained for reference but not current.**

- **[Archive](archive/)** — Deprecated and superseded documentation
