# Multipart Client-Concurrency Compatibility Matrix

**Date:** 2026-09-17 · **Companion to:** [ADR-003](adr/003-multipart-object-layout-and-read-path.md), [ADR-015](adr/015-out-of-order-multipart-uniform-part-size.md), [ADR-011](adr/011-barman-stays-on-armor-non-uniform-multipart.md)

## The claim, and what reconciles it

The README promises, scoped: *"Any S3-compatible client — boto3, AWS CLI, DuckDB, rclone, litestream, barman — works without modification: reads, range reads and single-PUT writes are plain S3, and multipart writers run with their default concurrency on the default write format (v3), whose only multipart rule is B2's own ≥ 5 MiB non-final-part minimum. The legacy v2 format keeps a uniform-part-size contract that stock client retry behavior already covers."* This page is what that scoping means, client by client, with the tests that back it.

ADR-003 §4 (2026-07-18) once contradicted that: the interim implementation
enforced sequential part arrival and rejected concurrent or out-of-order
uploads with `InvalidPartOrder`. That enforcement was **superseded two design
cycles ago** and is no longer in the code:

| When | Change | Effect on clients |
|---|---|---|
| 2026-07-19 | [ADR-015](adr/015-out-of-order-multipart-uniform-part-size.md) — uniform-part-size contract; CTR offset derived from part number, not arrival history | Out-of-order and concurrent part uploads accepted; `InvalidPartOrder` removed from the error surface |
| 2026-07-19 (amendment) | P pinned **only** from part 1; earlier arrivals deferred with retryable `503 SlowDown` | AWS CLI / SDK defaults (where the short final part completes first) work unmodified |
| 2026-08-07 | [ADR-011](adr/011-barman-stays-on-armor-non-uniform-multipart.md) — part-1/final-part alignment exemptions + non-uniform part mode | Misaligned, differently-sized parts (barman's `chunk_size + N×512`) accepted; verified in production 2026-08-27 |
| 2026-08-31 | Format v3 multipart layout ([ADR-016](adr/016-multipart-metadata-finalization.md)); `ARMOR_FORMAT_VERSION` now defaults to `3` | Per-part independent counters — no part-order or part-size contract at all |

ADR-003 §4's body text is retained as the historical record of the interim
design; its header and the §4 annotation point here. The pre-ADR-015
[upload/retrieval matrix](archive/upload-retrieval-test-matrix.md) is likewise
historical — its U6/U7/U8 rows ("rejected with 400") describe the superseded
sequential-only behavior, and the U-numbers carry forward to the matrix below.

## The contract per write format

Selected with `ARMOR_FORMAT_VERSION` (default **3**; `armor version` reports it).

**Format v3 (default):** parts are encrypted in independent counter
namespaces. Any part sizes, any part numbers, any arrival order, any
concurrency. No `SlowDown` is ever returned for multipart parts. The only
multipart constraint that remains is B2's own: non-final parts must be
≥ 5 MiB, and ARMOR backstops that at `CompleteMultipartUpload` using part 1's
size. A client's retry/abort semantics are its own; same-number re-uploads
are idempotent.

**Format v2 (legacy):** ADR-015's uniform-part-size contract, as amended:

- Part 1 pins the uniform part size `P`; part 1 may be any size (an
  unaligned part 1 marks the upload single-part-only).
- A part numbered > 1 arriving before part 1 receives **`503 SlowDown`**
  (with `Retry-After`); nothing is stored. Every standard client retries it
  transparently.
- Every part except the highest-numbered one must be exactly `P`; the final
  part may be any size (including zero).
- Parts that are non-aligned or differently-sized switch the upload to
  ADR-011 non-uniform mode (cumulative offsets, boundary-block HMAC backfill).
- Contract contradictions (a part > `P`, two short parts, a size-changing
  retry) **poison the upload**: every further part fails with `InvalidPart`
  and the client must abort and restart. Failures are loud, never silent
  corruption (ADR-002's invariant).

## Compatibility matrix

| Client shape | Format v3 (default) | Format v2 (legacy) | Notes |
|---|---|---|---|
| **AWS CLI, default concurrency** (`aws s3 cp`, ≥ 8 MiB file) | ✅ unmodified | ✅ unmodified | The short final part completes first; on v2 its first attempt is deferred `503 SlowDown` and the CLI retries transparently |
| **AWS CLI, serial** (`max_concurrent_requests = 1`) | ✅ | ✅ | Lowest-common-denominator mode; works on every ARMOR version |
| **SDK transfer manager, concurrent** (boto3 `TransferConfig`, AWS SDK for Go/Java managers, s5cmd) | ✅ unmodified | ✅ unmodified | SDKs retry 5xx by default, which covers the v2 `SlowDown` deferral; out-of-order completion is accepted |
| **SDK, serial / low-level in-order** | ✅ | ✅ | |
| **litestream** (multipart snapshots; no serial knob exposed) | ✅ unmodified | ✅ unmodified | litestream is the client that motivated ADR-015 — it cannot be configured serial |
| **rclone** (`--s3-upload-concurrency`, default 4) | ✅ unmodified | ✅ unmodified | `--s3-chunk-size` tuning optional on v2, not required |
| **barman-cloud-backup** (tar-aligned `chunk_size + N×512` parts) | ✅ unmodified | ✅ | v2: ADR-011 non-uniform mode; single-part backups verified in production 2026-08-27 (ADR-011 §Verification) |
| **DuckDB / pyiceberg / readers** | read-only | read-only | GET / Range / HEAD only; single-PUT writers are unaffected by any of this |

There is **no client configuration required** on the default format. On v2
the only requirement is what the client already does by default: retry on
5xx. Explicit serial configuration (the pre-ADR-015 workaround) is no longer
needed on either format.

### What a client can still get wrong (both formats)

- **Sub-5 MiB non-final parts** — B2's own rule; the upload fails upstream.
- **Retrying a part with a *different* size** (v2) — poisons the upload;
  abort and restart with the same part sizing.
- **Never sending part 1** (v2) — completion fails with `InvalidPart` naming
  the missing part.
- **`InvalidPartOrder`** — no longer exists for well-formed uploads; it was
  ADR-003 §4's rejection and is gone from the error surface.

## Tested configuration examples — AWS CLI, litestream, barman

These are the configurations `armor client-config --for <tool> --endpoint <url>`
emits (run it against your deployment and it appends the multipart contract in
force, so the config carries its own scope), followed by the behaviors the
tests in the next section assert for each: what is supported, and what is
rejected with which error. Placeholders below are literal — real credentials
travel by reference, never into a committed config.

### AWS CLI

`~/.aws/config` (as `armor client-config --for aws-cli` emits it):

```ini
[profile armor]
endpoint_url = https://armor.example.com:9000
s3 =
    addressing_style = path
region = us-east-1
```

Credentials via `aws configure --profile armor` (or the `ARMOR_AUTH_*`
environment variables). `addressing_style = path` is required — B2 does not
serve virtual-hosted style; the region value is a client-side requirement,
unused by ARMOR.

**Supported:**

- `aws s3 cp` at default concurrency — multipart fan-out with the short final
  part usually completing first. Accepted as-is on v3; on v2 the final part's
  first attempt is deferred with a retryable `503 SlowDown` that the CLI's
  default retry clears (`TestMultipartClientCompat_AWSCliDefaultConcurrent/{format_v3,format_v2}`).
- `max_concurrent_requests = 1` serial mode — works on every ARMOR version
  (`TestMultipartClientCompat_Serial`).
- Byte-identical GET and Range reads of the completed object.

**Rejected:**

- A non-final part under 5 MiB — B2's own rule, failed upstream at
  `UploadPart`; on v3 ARMOR additionally backstops at
  `CompleteMultipartUpload` using part 1's size → **`InvalidPartSize`** (400).
- (v2) Retrying a part with a *different* size than its first attempt — the
  contradiction poisons the upload; every further part gets **`InvalidPart`**
  until the client aborts and restarts with stable part sizing.

### litestream

`litestream.yml` replica (as `armor client-config --for litestream` emits it):

```yaml
dbs:
  - path: /path/to/db.sqlite
    replicas:
      - type: s3
        endpoint: https://armor.example.com:9000
        bucket: YOUR_BUCKET
        region: us-east-1
        access-key-id: YOUR_ACCESS_KEY_ID
        secret-access-key: YOUR_SECRET_ACCESS_KEY
```

**Supported:**

- Multipart snapshots at litestream's fixed internal concurrency, out-of-order
  completion included — litestream exposes no serial knob and needs none; this
  is the client that motivated ADR-015, and it works unmodified on both
  formats (`TestMultipartV3ConcurrentOutOfOrder`;
  `TestMultipartClientCompat_SDKTransferManager` models its shape — a
  concurrent pool whose 5xx retry covers the v2 deferral).

**Rejected:**

- None of litestream's own behaviors are rejected: it has no part-size or
  concurrency knobs, so every client-shape violation listed above is
  unreachable through it. Restores are GET/HEAD-only and unaffected. (The
  known litestream failure history — bf-24sxh7, bf-2sq7gf — was server-side
  layout bugs, fixed in 0.1.18xx, not client misconfiguration.)

### barman (barman-cloud-backup)

Environment (as `armor client-config --for barman` emits it):

```bash
export AWS_ENDPOINT_URL=https://armor.example.com:9000
export AWS_REGION=us-east-1  # Required but unused by ARMOR
export AWS_ACCESS_KEY_ID=YOUR_ACCESS_KEY_ID
export AWS_SECRET_ACCESS_KEY=YOUR_SECRET_ACCESS_KEY
```

Then `barman-cloud-backup backup` / `barman-cloud-wal-archive` as usual
(`--endpoint-url "$AWS_ENDPOINT_URL"` where the command does not honor the
environment variable).

**Supported:**

- barman's tar-aligned part sizes (`chunk_size + N×512`, never
  block-aligned) — switched to ADR-011 non-uniform mode on v2, no contract at
  all on v3 (`TestMultipartSuspectPatterns/U8_non_block_aligned_regular_part_accepted_under_adr011`).
- Single-part base backups (a small database fits one flush) — verified in
  production 2026-08-27 (ADR-011 §Verification);
  `TestMultipartLonePartByteVerification` is the in-suite twin.

**Rejected:**

- (v2) A retry that changes a part's size mid-upload — poisons the upload;
  `CompleteMultipartUpload` fails with **`InvalidPart`** and no object is
  stored (`TestMultipartADR015Acceptance/second_short_part_poisons_no_object`).
  barman's part sizing is deterministic, so its own retries are stable.
- (v2) Completion without part 1 ever uploaded — **`InvalidPart`** naming
  part 1 (the uniform part size would be unknown).
- A `--chunk-size` small enough to emit non-final parts under 5 MiB — B2
  rejects those upstream at `UploadPart`.

## Executable rows — where each claim is tested

Every matrix row above is backed by tests that complete the upload and
require a byte-identical GET. Run with:

```bash
go test -short -run TestMultipartClientCompat ./internal/server/handlers/
go test -short ./tests/aws-cli-compatibility/
```

| Row | Tests |
|---|---|
| AWS CLI default concurrent, v3 | `TestMultipartClientCompat_AWSCliDefaultConcurrent/format_v3` (short final part first, concurrent fan-out, round-trip) |
| AWS CLI default concurrent, v2 | same test `/format_v2` (asserts the `503 SlowDown` deferral + transparent retry); `TestMultipartSuspectPatterns/U6_part_2_before_part_1_slowdown_then_succeeds` |
| SDK transfer manager, both formats | `TestMultipartClientCompat_SDKTransferManager/{format_v3,format_v2}` (4-worker pool, out-of-order completion, in-pool SlowDown retry, idempotent same-size re-upload) |
| Serial, both formats | `TestMultipartClientCompat_Serial/{format_v3,format_v2}`; `TestMultipartFullCycleByteVerification` (9-part serial with range checks); `TestMultipartLonePartByteVerification` (single-part barman shape) |
| v3 no-contract semantics | `TestMultipartV3ConcurrentOutOfOrder`, `TestMultipartV3NoSlowDown`, `TestMultipartV3DistinctPartsReachBackendConcurrently`, `TestMultipartV3IndependentPartCounter` |
| Real CLI binaries (not in CI image) | `TestAWSCLI_*`, `TestRclone_*` in `tests/aws-cli-compatibility/` — skip cleanly when `aws`/`rclone` are absent; the `TestVerify_*` SDK twins run always |
| barman non-uniform parts, v2 | `TestMultipartSuspectPatterns/U8_non_block_aligned_regular_part_accepted_under_adr011` (round-trip verified) |
| v2 contract violations poison loudly | `TestMultipartADR015Acceptance/second_short_part_poisons_no_object` in `multipart_routing_test.go` — a genuine contract contradiction (two short parts) is 400'd, the upload id is poisoned so `CompleteMultipartUpload` fails, and no object is stored |

## Related

- [ADR-003](adr/003-multipart-object-layout-and-read-path.md) — multipart layout, sidecar HMAC table, the superseded §4
- [ADR-015](adr/015-out-of-order-multipart-uniform-part-size.md) — the uniform-part-size contract and its amendments
- [ADR-011](adr/011-barman-stays-on-armor-non-uniform-multipart.md) — non-uniform parts; barman stays on ARMOR
- [AWS CLI / rclone Compatibility Tests](../tests/aws-cli-compatibility/README.md) — the harness the rows above run on
- [Archived upload/retrieval matrix](archive/upload-retrieval-test-matrix.md) — the 2026-07-15 audit this doc supersedes for U6/U7/U8
