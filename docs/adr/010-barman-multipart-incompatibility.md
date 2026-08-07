# ADR-010: Barman-Cloud-Backup Multipart Incompatibility with ARMOR Encryption

**Status:** Superseded by [ADR-011](011-barman-stays-on-armor-non-uniform-multipart.md) (2026-08-07)

> **Superseded.** The barman root-cause analysis below remains valid and is the
> reference for why no `chunk_size` can fix part alignment. Its *decision* —
> reroute base backups to Garage — is **not** adopted: barman backups stay on
> ARMOR (ADR-011). This ADR also missed that ARMOR's alignment rule was
> over-broad: alignment is only required of parts that another part is placed
> after, so part 1 and a short final part never needed it. Correcting that
> makes single-part uploads of any size work without weakening the contract.
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

**REVISED 2026-08-07 — superseded below.** The original decision here (routing these two backup targets through Garage instead of ARMOR) was rejected: this backup topology stays on ARMOR/B2. Instead:

**Add an opt-in "variable-part-size" mode to ARMOR** that lets an explicitly-scoped client accommodate non-uniform, non-block-aligned multipart parts — targeting barman's actual behavior directly instead of asking the backup topology or barman itself to change. This mode must not weaken ADR-005's guarantees for any client that doesn't opt in.

The mechanism, replacing the uniform-part-size assumption for opted-in uploads only:

1. **Explicit, narrow opt-in.** The client signals this mode on `CreateMultipartUpload` (e.g. a request header or query param specific to this mode); it must not be inferable or defaulted, and should be further scoped to the specific credential/bucket-prefix these two backup targets use so no other client can trigger it accidentally.
2. **Sequential-only delivery is required.** A part arriving out of expected order (`partNumber` ≠ next-expected) is rejected outright — the same poison-detection posture ADR-005 already applies to its own contract, just enforcing order instead of size uniformity. This removes any need to guess at an unseen part's offset.
3. **True cumulative byte offset, tracked server-side.** ARMOR persists a running offset in the upload's existing state (alongside the ADR-003 per-part HMAC table), computed from each part's *actual* received size — not `(partNumber-1) × partSize`.
4. **Blocks that straddle a part boundary must be handled explicitly.** Since part sizes are no longer guaranteed multiples of the 64 KiB encryption block, a block can now span the tail of one part and the head of the next — the current code assumes every part starts and ends on a block boundary and does not handle this. The implementation must buffer an incomplete trailing block's plaintext across the `UploadPart` boundary until enough bytes arrive to complete it. This buffered state must either survive a process restart within the upload's lifetime, or the upload must fail loudly on restart — silently dropping buffered bytes would reintroduce exactly the class of silent corruption ADR-002/003/005 already fought.

This is real crypto-engineering work, not a config flag — the cost estimate in "Alternatives Considered" below ("more design work than the immediate iad-ci outage warrants") was accurate when written; the direction is chosen anyway because keeping a single backend (ARMOR/B2) for this backup topology was judged more valuable than that added cost.

## Alternatives Considered

### 1. Route through Garage

**Originally accepted, now rejected.** Would have immediately resolved the failures with lower engineering risk by reusing the pattern `apexalgo-iad`/`ardenone-cluster` already use, at the cost of a second backend/topology for these two backup targets. Rejected in favor of keeping everything on the existing ARMOR/B2 path.

### 2. Modify barman-cloud-backup source code

Rejected: requires upstream changes to the barman project, would need to be maintained across barman version upgrades, and only fixes barman specifically — the ARMOR accommodation (the chosen decision) is reusable for any future client with the same limitation. If pursued instead, it would require patching `CloudTarUploader.write()` to pad parts to 65536-byte boundaries before flushing.

### 3. Continue attempting configuration workarounds

Rejected: the root cause analysis demonstrates that no configuration value can fix the incompatibility. Further attempts would waste time without addressing the fundamental issue.

## Consequences

### Immediate

- ARMOR needs a new opt-in variable-part-size multipart mode (see Decision) before `queue-db`/`forgejo-postgres` backups can succeed. Until it ships, both remain broken exactly as described in Root Cause — no interim topology change is planned.
- The uniform-part-size contract (ADR-005) remains fully enforced, unchanged, for every client that doesn't explicitly opt in.

### Long-Term

- ARMOR's uniform-part-size contract remains the default and documented requirement for clients using ARMOR's S3 endpoint for multipart uploads; the variable-part-size mode is an explicit, narrowly-scoped exception, not a relaxation of the default.
- Future S3 clients that can't produce block-aligned parts can reuse the variable-part-size mode instead of requiring a bespoke accommodation each time — but each new user of it should be deliberately reviewed, not silently allowed.
- Given this project's repeated history of silent multipart corruption (ADR-002/003/005), any change to the variable-part-size mode after initial ship must be held to the same adversarial-threat-modeling and real-crypto-path-testing bar as the initial implementation.

## Related Documentation

- ADR-002: Close detection gaps that let the multipart-upload corruption bug run 40 days undetected
- ADR-003: Per-part HMAC tables for multipart uploads
- ADR-005: Out-of-order multipart parts via a uniform-part-size contract
- ADR-008: Server-side observability for multipart part-size rejections
- Root cause analysis: `docs/research/barman_armor_root_cause_analysis.md` (code review, simulation, real-world reconstruction)
- Simulation script: `docs/research/barman_part_size_simulation.py`
