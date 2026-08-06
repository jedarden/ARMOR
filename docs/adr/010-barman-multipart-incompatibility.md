# ADR-010: Barman-Cloud-Backup Multipart Incompatibility with ARMOR Encryption

**Status:** Accepted
**Date:** 2026-08-06
**Related:** ADR-002, ADR-003, ADR-005, ADR-008

## Context

`queue-db` and `forgejo-postgres` (CNPG PostgreSQL clusters on `iad-ci`) have failed **100% of scheduled barman-cloud base backups since 2026-05-26** — 600+ consecutive failures with `InvalidPartSize` errors. Live pod logs from 2026-08-06T21:03-21:04Z show:

```
queue-db:          Part size 11876352 is not a multiple of the block size (65536 bytes)
forgejo-postgres:   Part size 4284416 is not a multiple of the block size (65536 bytes)
```

These are **correct rejections**. ADR-005 established a uniform-part-size contract requiring every non-final multipart part to be an exact multiple of ARMOR's 64 KiB encryption block size, because part offsets and per-block HMAC indices are derived from block-aligned arithmetic. Accepting a misaligned part would silently produce unreadable or corrupt ciphertext — exactly the class of bug ADR-002/ADR-003/ADR-005 already fought once.

A same-day "fix" (declarative-config commit `c8aefe75`, 2026-08-06 08:50 EDT) disabled gzip compression on the theory that barman's tar streamer flushes in exact 65536-byte chunks when uncompressed. **This was incorrect.** Live logs pulled hours after the fix showed both clusters still failing on every attempt. The "verified empirically" claim was based on insufficient testing.

## Root Cause

Barman-cloud-backup 3.19.1 (the version confirmed live in both pods) produces multipart upload parts that are not multiples of ARMOR's 65536-byte encryption block size. This is a **fundamental incompatibility** between barman's upload logic and ARMOR's encryption requirements, not a configuration issue.

### The Mechanism

From `barman/cloud.py:246-261` in barman 3.19.1:

```python
def write(self, buf):
    # Flush when buffer.tell() > chunk_size
    if self.buffer and self.buffer.tell() > self.chunk_size:
        self.flush()
    if not self.buffer:
        self.buffer = self._buffer()
    if self.compressor:
        compressed_buf = self.compressor.add_chunk(buf)
        self.buffer.write(compressed_buf)
        self.size += len(compressed_buf)
    else:
        self.buffer.write(buf)
        self.size += len(buf)
```

The flush condition is `buffer.tell() > chunk_size`. This produces parts of size `chunk_size + N` where N depends on the last write operation:
- For uncompressed tar streams (the post-"fix" configuration), N is a multiple of 512 bytes (POSIX tar block size).
- For compressed streams, N is unpredictable (depends on compression ratio).
- N is **not** controlled by `chunk_size` — it depends on the PostgreSQL backup stream's write pattern.

ARMOR requires `part_size % 65536 == 0` (enforced at `internal/server/handlers/handlers.go:2194`). This is non-negotiable because part encryption offsets are calculated as `(part_number - 1) × part_size / block_size`, and misaligned parts would produce incorrect HMAC indices.

### The Evidence

Real-world failure reconstruction shows the mismatch precisely:

```
queue-db: 11,876,352 = 65536 × 181 + 14,336
           14,336 = 512 × 28  ✓ (multiple of 512)
                           ✗ (not multiple of 65536)

forgejo-postgres: 4,284,416 = 65536 × 65 + 24,576
                  24,576 = 512 × 48  ✓ (multiple of 512)
                                    ✗ (not multiple of 65536)
```

Both remainders are exact multiples of 512 (tar block size) but NOT multiples of 65536. This is consistent with barman's `write()` logic and incompatible with ARMOR's contract.

### Why No Configuration Fix Exists

Attempted workarounds all failed:
1. `--min-chunk-size=5MB` (default): Still produces misaligned parts.
2. `--min-chunk-size=2GB`: Attempted to force single-part uploads. Still produced one misaligned part.
3. Disable gzip compression: Still produces misaligned parts (the 2026-08-06 "fix").

**The problem is in barman's `write()` logic, not configuration parameters.** Even with a `chunk_size` that is a multiple of 65536, the overflow (the amount by which the buffer exceeds `chunk_size` before flushing) breaks alignment. The overflow is determined by the PostgreSQL backup stream's write pattern, which is outside barman's control.

Mathematically, for any fixed `chunk_size` C:
```
Part size = C + N × 512
```

where N depends on the backup stream. For ARMOR compliance:
```
C + N × 512 ≡ 0 (mod 65536)
```

No fixed C can satisfy this for arbitrary N (i.e., arbitrary data sizes). The only reliable solution is to modify barman's source code to pad parts to 65536-byte boundaries — a code change, not a configuration change.

## Decision

**Route `queue-db` and `forgejo-postgres` base backups through an S3-compatible backend that does not require block-aligned parts** (e.g., Garage, which other clusters already use for barman), with ARMOR/B2 maintained as a secondary off-site copy via replication or a separate backup job. This is the lowest-risk option that:

1. **Immediately resolves the 600+ consecutive backup failures** on iad-ci.
2. **Maintains ARMOR's correctness guarantees** — the uniform-part-size contract remains enforced for all clients that can satisfy it.
3. **Follows an already-operational pattern** — `apexalgo-iad` and `ardenone-cluster` already route barman backups through non-ARMOR backends.

## Alternatives Considered

### 1. Modify barman-cloud-backup source code

Rejected: This is outside the scope of this ADR (it requires upstream changes to the barman project) and would not address the immediate backup failures on iad-ci. If pursued, it would require patching `CloudTarUploader.write()` to pad parts to 65536-byte boundaries before flushing.

### 2. Add an opt-in ARMOR mode for misaligned parts

Rejected for now as higher-risk than routing through an alternative backend. Could be revisited if:
- Multiple S3 clients cannot be modified to satisfy the uniform-part-size contract.
- A carefully threat-modeled design is produced that does not reintroduce the silent corruption from ADR-002.
- The new integrity mechanism is extensively tested against the same failure modes that ADR-002/003/005 addressed.

The risk is that any accommodation for misaligned parts must compute HMAC offsets differently, and this new path must not have the same blind spots as the pre-ADR-005 code. This requires more design work than the immediate iad-ci backup outage warrants.

### 3. Continue attempting configuration workarounds

Rejected: The root cause analysis demonstrates that no configuration value can fix the incompatibility. Further attempts would waste time without addressing the fundamental issue.

## Consequences

### Immediate

- `queue-db` and `forgejo-postgres` base backups must be routed to a non-ARMOR backend (e.g., Garage) in iad-ci.
- ARMOR/B2 can be maintained as a secondary off-site copy via a separate backup job or replication.
- The uniform-part-size contract (ADR-005) remains enforced for all other clients.

### Long-Term

- ARMOR's uniform-part-size contract is documented as a requirement for clients using ARMOR's S3 endpoint for multipart uploads.
- Future S3 client integrations should be evaluated for their ability to produce block-aligned parts before deployment.
- If more clients cannot satisfy the contract, an ARMOR accommodation (Alternative 2) may be worth the design investment — but only with adversarial threat modeling and extensive testing.

## Related Documentation

- ADR-002: Close detection gaps that let the multipart-upload corruption bug run 40 days undetected
- ADR-003: Per-part HMAC tables for multipart uploads
- ADR-005: Out-of-order multipart parts via a uniform-part-size contract
- ADR-008: Server-side observability for multipart part-size rejections
- Root cause analysis: `/tmp/barman_armor_root_cause_analysis.md` (code review, simulation, real-world reconstruction)
