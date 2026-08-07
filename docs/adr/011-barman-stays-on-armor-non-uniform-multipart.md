# ADR-011: Barman stays on ARMOR; support non-uniform multipart parts

**Status:** Accepted
**Date:** 2026-08-07
**Supersedes:** [ADR-010](010-barman-multipart-incompatibility.md)
**Related:** ADR-002, ADR-003, ADR-005, ADR-008

## Context

ADR-010 (2026-08-06) concluded that barman-cloud-backup is fundamentally
incompatible with ARMOR and decided to **route `queue-db` and
`forgejo-postgres` base backups to a non-ARMOR backend (Garage)**, keeping
ARMOR/B2 as a secondary copy.

That decision is **reversed by operator direction: barman backups stay on
ARMOR.** This ADR records the reversal and what actually has to be true for
barman to work through ARMOR.

ADR-010's *analysis* of barman is accurate and is retained by reference:
barman 3.19.1 flushes when `buffer.tell() > chunk_size`, producing parts of
`chunk_size + N × 512` — aligned to tar's 512-byte block, never to ARMOR's
65536-byte encryption block. No `chunk_size` fixes this, because the overflow
is a property of the PostgreSQL backup stream, not of configuration.

Where ADR-010 was **incomplete** is its options analysis. It evaluated only
two server-side possibilities — enforce the contract as written, or build a
general "opt-in mode for misaligned parts" — and rejected the latter as too
risky. It did not consider that ARMOR's alignment rule was *over-broad to
begin with*.

## The over-broad rule

Alignment exists for exactly one reason: to keep a part's
`(N-1) × P / BlockSize` CTR offset on a block boundary. It is therefore only
required of parts that another part is placed **after**. Two kinds of part
were being rejected despite never needing alignment:

- **Part 1** always starts at block 0. `(1-1) × P / BlockSize` is 0 for any
  `P`, so its ciphertext and absolute HMAC indices are correct at any size.
  A part 1 that turns out to be the only part needs no alignment at all.
- **A part smaller than an already-pinned `P`** — the presumed-final part.
  Nothing is placed after it, and its partial trailing block is exactly what
  every ordinary non-multipart PUT already produces.

Correcting this (shipped in `fc6c1a86`) makes **single-part uploads of
arbitrary size work**, with no weakening of the contract for parts that do
need alignment. This is a correctness fix to an over-broad check, not an
accommodation.

**This resolves the immediate outage.** `queue-db` is 30 MB and
`forgejo-postgres` 63 MB — both fit comfortably in one part, far below S3's
5 GiB per-part ceiling. Barman must be configured with a `--min-chunk-size`
large enough to force a single flush; the 2026-08-03/04 attempt at exactly
this failed only because the server-side exemption did not yet exist.

## Decision

1. **Barman base backups remain on ARMOR.** ADR-010's reroute to Garage is
   not adopted.
2. **The part-1 / final-part exemptions stand** as the immediate fix, and are
   the correct reading of the contract rather than a concession to one client.
3. **Support non-uniform multipart parts** so barman keeps working as these
   databases grow past a single part. Design below.

## Non-uniform multipart: what it requires

ADR-005 pins one uniform `P` from part 1 and derives every later part's offset
as `(N-1) × P`. That is what makes out-of-order arrival safe. Barman's parts
are both non-aligned *and* non-uniform, so a second part of a different size
contradicts `P` and is rejected today. Supporting it needs three changes:

1. **Per-part offsets from cumulative sizes.** A part's offset becomes the sum
   of the sizes of all lower-numbered parts, not `(N-1) × P`. This requires
   every lower part to be known, so a part arriving before its predecessors is
   deferred with the retryable `503 SlowDown` that ADR-005 already uses for
   part>1-before-part-1. Uniform-size uploads keep the existing fast path
   unchanged.
2. **CTR seek to an arbitrary byte offset.** Encryption is currently
   block-indexed; a part whose cumulative start offset is mid-block must seek
   the keystream to that byte position.
3. **Boundary-block HMAC backfill.** A block spanning two parts cannot have its
   HMAC computed by either part alone. At `CompleteMultipartUpload` — where the
   sidecar HMAC table is already assembled — re-read each boundary block from
   the completed object and compute its HMAC then. At most `numParts - 1`
   blocks of 64 KiB each.

This preserves the on-B2 layout (raw concatenated part ciphertext, sidecar HMAC
table, `x-amz-meta-armor-multipart` marker) and needs no third format.

**Not yet implemented.** Given the silent-corruption history in ADR-002/003/005,
this lands only with adversarial tests covering: mid-block boundaries at every
offset residue, out-of-order arrival under deferral, retried parts at a
different size, and a completed object verified byte-identical end to end.

## Alternatives considered

- **Reroute to Garage (ADR-010's decision).** Rejected by operator direction:
  backups stay on ARMOR.
- **Patch barman to pad parts to 65536 boundaries.** Still viable and arguably
  the cleanest upstream fix, but it requires shipping a patched
  barman-cloud-backup in the CNPG image and does not help other misaligned
  clients. Kept as a fallback if the boundary-block backfill proves harder than
  expected.
- **General "accept misaligned parts" mode.** Rejected, as in ADR-010. The
  design above is narrower: offsets stay exact and every block still gets a
  verified HMAC.

## Consequences

- The immediate 100%-failure outage is resolved by the exemptions plus a
  single-part barman configuration — **not** by rerouting.
- Until non-uniform support lands, barman through ARMOR has a **size ceiling**:
  it works while a backup fits in one part. This must be monitored, because it
  degrades silently into `InvalidPartSize` once exceeded.
- ADR-010 is superseded. Its barman root-cause analysis remains valid and is
  the reference for *why* configuration cannot fix part alignment; its decision
  and its "no server-side fix short of a risky accommodation" framing are not.
- ADR-005's uniform-part-size contract remains the fast path and remains
  enforced for parts that something is placed after.

## Correction to the record

ADR-010 states the 2026-08-06 compression-removal fix was verified
empirically and was wrong. That is right, and the same caution applies here:
as of this ADR the exemption fix is **committed but not yet confirmed in
production** — the deployed image predates it. No claim of resolution should
be made until multiple consecutive real `ScheduledBackup`-triggered runs
complete on both clusters.

Separately, the `ScheduledBackup` schedules on both clusters are wrong
independently of any of this: `0 4 * * *` and `0 3 * * *` are parsed by CNPG
as 6-field cron *with seconds*, so they fire hourly at :04/:03 rather than
daily. Confirmed live 2026-08-07. Fix is `0 0 4 * * *` / `0 0 3 * * *`. This
inflates the failure and retry rate ~24×.
