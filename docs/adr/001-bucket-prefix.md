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

## Addendum: Bucket Alias (2026-09-05)

`ARMOR_PREFIX` makes the move itself transparent to a client — the key a client
names is unchanged, only where ARMOR stores it — but it does nothing for the
bucket name in the URL. A tenant that served from its own bucket has every
consumer addressing `<old-bucket>/...`, and after consolidation those objects
live in the unified bucket, so the old name reads as a bucket that no longer
holds anything. Without a bridge, consolidation is coupled to a simultaneous
edit of every consumer: manifests, CI templates, public download routes.

`ARMOR_BUCKET_ALIASES` is that bridge: a comma-separated list of legacy bucket
names that resolve to `ARMOR_BUCKET`. Parsing is deliberately boring — entries
are trimmed of surrounding whitespace, and empty entries, duplicates, and the
configured bucket's own name are dropped, since recording it as an alias of
itself would claim a consolidation that did not happen.

**One seam.** `config.ResolveBucket` is the only place the mapping lives, and
every consumer of a request's bucket name goes through it — `Server
.extractBucketAndKey`, which the authorization middleware calls, and
`Handlers.HandleRoot`, which routes and dispatches. Both resolve before any
other use, so a backend call always names the bucket that holds the objects and
the ACL check always tests the bucket the request is actually served from. A
name matching no configured bucket is returned unchanged, so an unknown bucket
fails in the backend exactly as it did before aliases existed: a typo cannot be
silently remapped into another tenant's namespace.

**Authorization sees the resolved name, which is both the feature and the
guard.** Existing ACL strings are written against the configured bucket, so
resolving first is what lets a pre-consolidation credential keep working for a
client still sending the legacy name. The same rewrite is what keeps aliasing
from being a grant: a credential scoped to the legacy name grants nothing,
because no alias maps onto the bucket it names. Adding an alias therefore
narrows what a legacy-name credential can reach, and never widens anything.

**Response bodies echo the name the client sent.** S3 clients compare the
bucket in a response against the one they addressed and discard a mismatch, so
`ListObjectsV2`'s `Name`, the multipart results, and `ListObjectVersions` all
report the alias while the work happens against the configured bucket.
`CopyObject`'s source arrives in the `x-amz-copy-source` header rather than the
URL, so it is parsed and resolved independently. `ListBuckets` reports only the
configured bucket — an alias is a name for an existing bucket, not a bucket,
and listing it would claim a consolidation that makes the alias unnecessary.

**Virtual-hosted style is still not recognised, deliberately.** A request whose
bucket is in the `Host` header arrives as `GET /key` with no bucket in the path
at all, so its first path segment is read as a bucket name — and that name is
not the alias, it is the first component of the key. Such a request therefore
keeps failing exactly as it did before aliases existed, which is the safe
outcome: an alias in a `Host` header cannot reach the objects, and neither can
it widen access. ARMOR does not inspect the host to fix this, because a
virtual-hosted host cannot be distinguished from an ordinary hostname that
merely begins with a bucket name, and guessing wrong turns a path-style request
into a read of a different object. The alias list changes which names work, not
which addressing styles.

**Cutover ordering.** Arm the alias in the same rollout that completes the
move, and before any consumer is touched. Arming it while the source bucket
still holds the only copy sends old-name requests to a bucket that does not
have their objects yet, and completing the move without the alias leaves a
window where old-name writes land in the source and the two copies diverge —
the alias and the data move are one step, not two. Consumers are then repointed
at their own pace, each independently, and the alias is dropped once no client
sends the legacy name. Keep the source bucket and its B2 key alive for as long
as the alias is set: an un-migrated client's read resolves through it, so
removing either early breaks exactly the client the alias exists to carry. The
operational steps are the runbook's, not this document's.

