# ADR-001: Shared Bucket via ARMOR_PREFIX

**Status:** Accepted  
**Date:** 2026-07-05

## Context

Each ARMOR deployment was originally paired with its own dedicated B2 bucket. As the number of deployments grew across clusters (iad-ci, iad-kalshi, iad-native-ads, ord-devimprint, rs-manager), this created several problems:

- One B2 application key per bucket — key sprawl and per-bucket rotation overhead
- Private buckets required to protect content — no Cloudflare CDN caching, so all reads incur B2 egress at $0.01/GB
- Public buckets with per-deployment encryption mean the encryption benefit exists but egress costs remain per-bucket

The Backblaze/Cloudflare Bandwidth Alliance provides **free egress** when a B2 bucket is public and traffic routes through Cloudflare. ARMOR already encrypts all content (AES-256-CTR), making a public bucket safe. The blocker was that a single shared public bucket had no mechanism to namespace objects per deployment — any two ARMOR instances writing to the same bucket could collide on key names.

## Decision

Add an optional `ARMOR_PREFIX` environment variable. When set, ARMOR prepends this prefix to every S3 key before forwarding to B2, and strips it from keys in all responses. This makes the prefix transparent to consumers while enforcing namespace isolation at the proxy layer.

**Normalization rule:** the prefix is stored internally with exactly one trailing slash and no leading slash. `kalshi-tape`, `kalshi-tape/`, and `/kalshi-tape/` all normalize to `kalshi-tape/`. This ensures consistent key construction without requiring consumers to manage trailing slashes.

**Empty prefix behavior:** when `ARMOR_PREFIX` is unset or empty, ARMOR applies no prefix. Keys pass through unchanged. There is no default prefix. Existing deployments that do not set this variable are entirely unaffected.

## Consequences

**Positive:**
- All ARMOR deployments can share a single public bucket with one B2 application key
- Cloudflare CDN caches reads — egress is free regardless of query frequency
- Prefix enforcement is at the proxy layer — consumers do not need to coordinate naming conventions
- One B2 key rotation covers all workloads
- Per-workload MEKs remain in place — a compromised MEK for one prefix does not expose others

**Negative:**
- A misconfigured prefix (or missing prefix on a new deployment) could result in objects written to the bucket root, complicating cleanup
- ListBuckets from a prefixed ARMOR still lists the full bucket name — consumers see the shared bucket, not a virtual per-prefix bucket

## Alternatives Considered

**Client-side prefix convention** — require each consumer to write keys under its own prefix. Rejected: unenforceable, depends on every consumer being correctly configured, breaks if a consumer is updated without updating its key convention.

**One bucket per deployment (current state)** — keep dedicated buckets, accept egress costs. Rejected: egress cost scales with query frequency; queryable analytics workloads (DuckDB over Parquet) would be expensive.

**Per-workload Cloudflare Workers** — add a Worker per bucket that enforces namespace. Rejected: operational complexity, cost, and latency overhead for what is fundamentally a proxy-layer concern.

## Addendum: Bucket Naming Policy (2026-09-05)

This ADR named the first shared bucket in a public repository, which is how the
first bucket's name became public on the day it was created. A replacement
bucket was created on 2026-09-03 with a random name that is deliberately kept
out of git, and all consolidation lands in that one. The naming rule is now
part of this ADR rather than an operator habit:

- The unified bucket's name is **not written to git** — not to `docs/`, `tests/`,
  source comments, commit messages, or beads (`.beads/checkpoint/` is
  git-tracked, so a name in a bead note is a name in git).
- Documentation, including this ADR, refers to **"the unified bucket."**
- The name is stored in OpenBao only: once at the canonical record, and once in
  each tenant's path. `docs/runbooks/unified-bucket-tenant-onboarding.md` is the
  authoritative layout.
- Kubernetes Secrets and pod environment are acceptable carriers — they are
  cluster state, not the repository — but pod logs are not, so the startup log
  records a fingerprint of the bucket name rather than the name itself.

Onboarding a tenant onto the unified bucket, and moving an existing tenant's
objects into it, follow the runbook rather than this document.

