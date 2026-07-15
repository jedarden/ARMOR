# Upload & Retrieval Pattern Test Matrix

**Date:** 2026-07-15 · **Companion to:** `adr/002-multipart-corruption-detection-gaps.md`

ARMOR fronts several very different S3 clients, and the 40-day multipart corruption incident (ADR-002) proved that "the tests pass" means nothing unless the tests exercise **the same code paths real clients hit, at the same layer the bug can live in**. The original bug was in HTTP *routing* — so backend-level tests and the 1KB canary were structurally blind to it. This matrix enumerates every upload/retrieval pattern a real consumer uses, where it is (or isn't) tested, and at which layer.

## Layers

| Layer | What it can catch | What it cannot |
|---|---|---|
| **L1 unit** (e.g. `aws_chunked_test.go`) | encoding/parsing logic | routing, crypto interplay, B2 semantics |
| **L2 backend** (`internal/backend/*_test.go`, mock store) | part assembly logic, backend API contract | HTTP routing (where the ADR-002 bug lived), encryption |
| **L3 HTTP handler** (`internal/server/handlers/*_test.go`, httptest + mock backend) | routing, encryption/decryption round-trip, S3 XML/headers | real B2 behavior, network |
| **L4 real B2** (canary in prod; `armor-decrypt` offline) | end-to-end truth incl. B2 assembly | only what it's pointed at |

**Rule from ADR-002:** every *write* pattern needs coverage at L3 (routing + crypto) **and** L4 (canary or drill); every *read* pattern needs L3 at minimum.

## Real consumers and the patterns they use

| Consumer | Upload patterns | Retrieval patterns |
|---|---|---|
| **litestream** (queue-api SQLite backups — the ADR-002 victim) | small WAL-segment PUTs; **multipart snapshots >5MiB**; aws-chunked streaming bodies | full GET on restore (the failure that exposed the incident) |
| **barman** (Postgres WAL, iad-native-ads incident) | steady small/medium PUTs; multipart base backups | full GET on recovery |
| **clone-workers / commitgraph** (parquet corpus — the load-bearing dataset) | **large multipart PUTs** via SDK defaults (5–16MiB parts, possibly parallel) | — |
| **duckdb / pyiceberg / website-builder** (corpus readers) | — | **parquet access pattern: suffix-range GET (footer), then many small interior range GETs**; HEAD; List |
| **Canary** (in-cluster, every 5min + hourly) | 1KB single PUT; multipart canary (ADR-002 §1) | GET + byte/HMAC/SHA verify |
| **armor-decrypt** (offline DR tool) | — | direct-from-B2 read + decrypt, bypassing the proxy |
| **Cloudflare zero-egress delivery** | — | GET (+Range) through CF CDN in front of ARMOR |

## Upload patterns

| # | Pattern | Real client | Coverage | Status |
|---|---|---|---|---|
| U1 | Single PUT, small (<64KiB block) | litestream WAL, state files | L3 `TestPutObjectGetObject`, `TestEncryptionRoundTrip`; L4 5-min canary | ✅ |
| U2 | Single PUT, large (streaming encryption path, ≥threshold) | any client below its multipart cutover | L3 `TestStreamingEncryptionLargeFile`/`MultiBlock`/`Threshold`/`SHA256` | ✅ |
| U3 | **aws-chunked streaming PUT** (`STREAMING-AWS4-HMAC-SHA256-PAYLOAD`) | AWS SDKs, litestream (default for streamed bodies) | L1 `aws_chunked_test.go` (**added 2026-07-15**, unrunnable until `bf-15sdaf` fixes the package build); no L3 test sends a chunked-encoded body end-to-end | ⚠️ L1 pending build fix — L3 gap |
| U4 | **Multipart, standard S3 flow** (POST `?uploads` → PUT `?partNumber&uploadId` → POST `?uploadId`) — **the ADR-002 bug path** | litestream snapshots, barman base backups, SDK parquet uploads | L3 routing tripwire `TestUploadPartRoutingNeverFallsThroughToPut` (**passing**) + full-cycle byte verify `TestMultipartFullCycleByteVerification` (**added 2026-07-15 — immediately found `bf-24sxh7` and is skip-gated on it**); L2 byte-verify suite (`multipart_integration_test.go`); L4 hourly multipart canary | 🔴 write path ✅, **read-back broken** (`bf-24sxh7`) |
| U5 | Multipart with POST-style part upload (non-standard; the only path pre-0.1.35 ARMOR handled) | rare | routed (`handlers.go` POST branch); no dedicated test | ⚠️ low-risk gap |
| U6 | **Out-of-order / parallel part upload** | **boto3/SDK default: parts upload concurrently** | none — see `TestMultipartSuspectPatterns` (skipped, documents why) | 🔴 **SUSPECT**: `UploadPart` derives the CTR counter from *arrival-order* cumulative `EncryptedBytes` (`handlers.go:1992`); parts arriving out of order get wrong counter offsets → silent corruption by design, not by regression |
| U7 | **Part retry** (same partNumber re-uploaded after network failure) | all SDKs retry parts | none | 🔴 **SUSPECT**: retried part increments `EncryptedBytes` again → all subsequent counters shift |
| U8 | **Non-block-aligned intermediate parts** (e.g. 10,000,000-byte parts; anything not a multiple of 64KiB) | configurable in every SDK | none | 🔴 **SUSPECT**: `startBlockIndex = EncryptedBytes / BlockSize` truncates on unaligned boundaries |
| U9 | Single-part multipart (multipart flow, 1 part) | SDKs near threshold | L2 `TestMultipartUploadSinglePart` | ✅ L2 (L3 covered implicitly by U4 cycle) |
| U10 | Abort + orphan cleanup | any failed upload | L3 `TestAbortMultipartUpload`(+`NotFound`) | ✅ |
| U11 | Presigned PUT/GET (`internal/presign`) | dashboard/browser flows | unit tests in `internal/presign` | ⚠️ verify scope |
| U12 | Server-side copy (incl. DEK rewrap) | metadata updates (multipart complete uses it) | L3 `TestCopyObject*` | ✅ |

## Retrieval patterns

| # | Pattern | Real client | Coverage | Status |
|---|---|---|---|---|
| R1 | Full GET + decrypt (small & streaming) | restores, readers | L3 `TestPutObjectGetObject`, `TestStreamingDecryption(VariousSizes)` | ✅ |
| R2 | Bounded range GET (`bytes=a-b`) | duckdb/pyiceberg column chunks | L3 `TestGetObjectRange`, `TestStreamingEncryptionRangeRead` | ✅ |
| R3 | Suffix range GET (`bytes=-N`) | **parquet footer read — first op of every corpus query** | L3 `TestRangeSuffixRequest` | ✅ |
| R4 | **Range GET on a multipart-assembled object** | parquet readers on corpus files (all >5MiB → all multipart) | `TestMultipartFullCycleByteVerification` range + suffix-range steps (**added 2026-07-15**, skip-gated on `bf-24sxh7`) | 🔴 **broken** (`bf-24sxh7`) |
| R5 | HEAD (incl. manifest fast path) | readers, sync tools | L3 `TestHeadObject`, `TestHeadObjectManifest*` | ✅ |
| R6 | Conditional GET/HEAD (If-Match/None-Match, ±Range) | CDN revalidation | L3 `TestConditionalRequests*`, `TestConditionalRequestsWithRange` | ✅ |
| R7 | Read-after-complete (immediately GET what multipart just assembled) | litestream restore verification | new U4 full-cycle test (L3, skip-gated on `bf-24sxh7`); hourly multipart canary (L4) | 🔴 **broken** (`bf-24sxh7`) |
| R8 | GET (+Range) **through the Cloudflare zero-egress path** | all public delivery | none automated — prod-only | ⚠️ ops gap: add a scheduled external probe |
| R9 | Offline decrypt from raw B2 (`armor-decrypt`) — proxy-bypass truth check | DR drills | manual only | ⚠️ should be part of any pre-cutover drill |
| R10 | List/ListV2/versions/prefix (Hive partition keys) | corpus discovery, pyiceberg | L3 `TestListObjectsV2`, `TestListObjectVersions`, `TestURLDecodeHivePartitionKeys`, cache tests | ✅ |

## Known integrity gaps (found while building this matrix, 2026-07-15) — beads filed

1. **`bf-24sxh7` (P0): multipart objects are unreadable through ARMOR's GET.** The read path (`handleFullObjectStream`, `handlers.go:720-727`, and the range path) assumes the single-PUT layout — 64-byte header + data + embedded HMAC table — and never checks the `armor-multipart` metadata flag. Multipart objects are raw concatenated part ciphertext with the HMAC table in a *sidecar*, so every GET 500s ("Failed to prefetch HMAC table: offset out of range"). Found by the new full-cycle test on its first run. Likely a second contributor to the ADR-002 "snapshot didn't decode" incident, independent of the routing bug.
2. **`bf-1v2ehf` (P2): multipart objects store a placeholder plaintext-SHA.** `CompleteMultipartUpload` sets `plaintextSHA = sha256("")` (`handlers.go:2132-2134`) — the manifest/provenance SHA for every multipart object is the empty hash; real corruption can't be distinguished from the placeholder.
3. **`bf-59unr3` (P1): U6/U7/U8 are design-level suspects, not regression risks.** The per-part CTR offset scheme assumes strictly sequential, exactly-once, block-aligned parts. Real SDKs violate all three by default. Until fixed or explicitly rejected (400 rather than corrupt), **clients writing through ARMOR must be pinned to sequential, block-aligned, no-retry part behavior — including the commitgraph clone-workers.**
4. **`bf-15sdaf` (P1): `internal/server` does not compile on main** (redeclared helpers from the recent bead-thrash commits), blocking the whole-module build and the new L1 chunked-decoder test.
5. Consumers on pre-fix ARMOR versions: per ADR-002, `iad-ci` (0.1.24) and `iad-kalshi`/`rs-manager` (0.1.13) remain on multipart-corrupting builds. `ord-devimprint` is patched (0.1.42, verified 2026-07-15).

**Consequence for the commitgraph rebuild:** until `bf-24sxh7` and `bf-59unr3` are fixed *and* the multipart canary is deployed to the target cluster, ARMOR multipart cannot be certified for the corpus write path. Interim options: single-PUT streaming writes only (stay under the SDK multipart cutover), or direct-B2 writes with ARMOR reserved for delivery.

## Test-run cheat sheet

```
go test ./internal/server/...          # L1 + L3 (routing tripwire, full-cycle, chunked decode)
go test ./internal/backend/...         # L2 multipart assembly byte-verification
curl $ARMOR/armor/canary               # L4: small + multipart canary status in prod
```
