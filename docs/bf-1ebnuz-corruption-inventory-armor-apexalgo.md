# bf-1ebnuz Corruption Audit: armor-apexalgo (iad-acb / ai-code-battle)

**Generated**: 2026-08-06
**Bead**: bf-1ebnuz ("Multipart-era corruption audit of unaudited ARMOR buckets")
**Scope of this document**: `armor-apexalgo` only, one of the four buckets bf-1ebnuz covers
(`armor-apexalgo`, `iad-ci`, `iad-kalshi`, `rs-manager`). The other three are **not** audited
here.
**Trigger**: `restore-verifier-acb` (rs-manager, namespace `armor`) reported 0/11 sampled
objects verified on its 2026-08-06 12:00 UTC run — 10 real objects (`replays/*.json.gz`,
`thumbnails/*.png`) failed with a SHA256 mismatch between "ARMOR path" and "Direct path"
plaintext digests; `.armor-b2-test` (a canary/test object, not a real finding) correctly
failed as "not ARMOR-encrypted".

## Executive Summary

**No corruption found.** All 10 real objects flagged by `restore-verifier-acb` are intact,
correctly decrypt with the sole MEK on file (OpenBao `secret/rs-manager/iad-acb/armor`), and
match their own upload-time-recorded plaintext SHA-256. The "0/11 verified" result is a false
positive caused by a bug in `restore-verifier`'s dual-path comparison logic
(`internal/restoreverifier/verifier.go`, `verifyObjectDual`/`restoreViaARMOR`), not by data
corruption of any kind — legacy multipart or otherwise. See Root Cause below.

## Method

1. Pulled B2 credentials + MEK for `armor-apexalgo` from OpenBao (`secret/rs-manager/iad-acb/armor`).
2. Rebuilt `cmd/armor-decrypt` from this repo's HEAD (the shipped binary in the repo root is
   linked against a Nix store path unavailable in this environment).
3. HEAD'd all 10 failing objects directly against B2's S3 API (bypassing both ARMOR and
   restore-verifier) for ground-truth metadata.
4. Decrypted all 10 objects with `armor-decrypt`, independently of both restore-verifier code
   paths.
5. Reproduced `restore-verifier`'s "ARMOR path" byte-for-byte using a standalone program that
   calls `internal/backend.B2Backend.Get()` exactly as `restoreViaARMOR()` does, to confirm
   the mechanism behind the false SHA mismatch.

## Findings: object-level ground truth

| Key | Size (B2) | Uploaded (UTC) | Multipart? | armor-decrypt result | File type | plaintext-SHA verified |
|---|---|---|---|---|---|---|
| replays/m_ec7d9753303469be.json.gz | 2526 | 2026-05-02 14:48:21 | no | OK | valid gzip | yes |
| replays/m_48ad1987377186bf.json.gz | 4156 | 2026-04-30 20:21:59 | no | OK | valid gzip | yes |
| thumbnails/m_54e68b02d31d6d3f.png | 5418 | 2026-05-07 04:49:27 | no | OK | valid PNG, 640x360 RGB | yes |
| thumbnails/m_f7be98b9039b41fd.png | 4941 | 2026-04-30 16:13:33 | no | OK | valid PNG, 640x360 RGB | yes |
| replays/m_654ed99a4d0dc73b.json.gz | 2235 | 2026-05-07 13:53:51 | no | OK | valid gzip | yes |
| thumbnails/m_6f764984e3921e86.png | 4052 | 2026-05-06 03:33:16 | no | OK | valid PNG, 640x360 RGB | yes |
| replays/m_d264ca1120832235.json.gz | 1834 | 2026-05-03 14:54:56 | no | OK | valid gzip | yes |
| thumbnails/m_d72c972940ec8508.png | 5800 | 2026-05-02 19:03:33 | no | OK | valid PNG, 640x360 RGB | yes |
| thumbnails/m_b516a504944e7697.png | 5881 | 2026-05-06 02:33:57 | no | OK | valid PNG, 640x360 RGB | yes |
| thumbnails/m_5a41802e98b9a28c.png | 5516 | 2026-05-05 04:03:02 | no | OK | valid PNG, 640x360 RGB | yes |

**All 10 objects**: 1.7–5.9 KB, single-PUT (no `x-amz-meta-armor-multipart`), uploaded
2026-04-30 through 2026-05-07 — this rules out the ADR-002 multipart write-time corruption bug
(versions 0.1.35–0.1.41, silent corruption of multipart uploads ≥5 MiB, fixed 2026-06-10/11)
outright for this sample: these objects are ~1000x smaller than the multipart threshold and
never went through multipart upload machinery at all. "plaintext-SHA verified" = `armor-decrypt`
independently recomputed SHA-256 of the decrypted plaintext and confirmed it matches the
object's own envelope-header-embedded digest (`EnvelopeHeader.VerifyPlaintextSHA`, written by
ARMOR at upload time) — i.e. the content has not changed since it was written.

## Root cause of the false positive

`restore-verifier`'s dual-path check (`ModeDual`, the *default* continuous 6h verification
loop — `runVerification()` → `verifyBucket(..., ModeDual)`) compares two things per object:

- **"Direct" path** (`restoreViaDirectDecrypt`): fetches raw ciphertext from B2, unwraps the
  DEK with the escrowed MEK, decrypts with per-block HMAC verification. **Correctly
  implemented** — confirmed byte-identical to the independently-rebuilt `armor-decrypt` CLI
  for all 10 objects.
- **"ARMOR" path** (`restoreViaARMOR`, verifier.go:1052): calls `v.backend.Get(ctx, bucket,
  key)`. The doc comment claims this is "the normal backend GetObject which will decrypt
  through ARMOR" — **this is false for the concrete backend restore-verifier is wired to.**
  `cmd/restore-verifier/main.go` constructs exactly one `backend.Backend` — a raw
  `backend.NewB2Backend(...)` pointed directly at B2/Cloudflare — and passes it as the sole
  backend for *both* paths. There is no HTTP client anywhere in restore-verifier pointed at a
  live ARMOR server's decrypt-on-GET S3 endpoint (no `-armor-url`/`ARMOR_ENDPOINT` flag exists
  in `cmd/restore-verifier/main.go` at all). `B2Backend.Get()` (`internal/backend/b2.go:112`)
  never touches `internal/crypto` — it is a plain storage read.

Made worse by `B2Backend.Head()` (`internal/backend/b2.go:188-213`): for ARMOR-encrypted
objects it deliberately overrides `ObjectInfo.Size` from the true raw Content-Length to the
`x-amz-meta-armor-plaintext-size` value (by design, so other callers see the logical/decrypted
size). `Get()` then does `GetRange(0, info.Size)` — i.e. fetches `plaintextSize` raw bytes
starting at raw offset 0. That range is neither the true raw object (header + ciphertext +
HMAC trailer, which is longer) nor the true plaintext (which doesn't start at raw offset 0 —
the first 64 bytes are the envelope header). The result is a meaningless truncated blob: the
64-byte envelope header followed by the first `(plaintextSize − 64)` bytes of ciphertext,
missing the tail of the ciphertext and the entire HMAC trailer.

This deterministic-but-meaningless blob is what gets SHA-256'd and reported as `ARMORSHA256`
(verifier.go:991), then compared against the correctly-decrypted `DirectSHA256` at
verifier.go:1011 — **guaranteed to mismatch for every ARMOR-encrypted object, unconditionally,
independent of whether the underlying data is actually corrupted.**

**Confirmed empirically**, not just by code reading: a standalone Go program calling
`backend.B2Backend.Get()` directly (same production credentials, same code) reproduced the
exact `ARMOR=` SHA-256 values from the `restore-verifier-acb` pod's log, byte for byte, for
both objects tested:

```
replays/m_ec7d9753303469be.json.gz:  16359599e5695a2446b063e9440ffdef35b9c8f5d6794957c0f6cc549235fbb0  (matches pod log exactly)
thumbnails/m_54e68b02d31d6d3f.png:    54e16e0cc471d00770dd68faa252df5ef5ac5a85b92b5246c56dca7dc6713c74  (matches pod log exactly)
```

Neither value is the SHA-256 of the true raw B2 object either (checked directly) — it's
specifically the header+truncated-ciphertext blob described above.

### Scope of the bug

Not specific to `armor-apexalgo`, not specific to this session, not specific to multipart
objects. `verifyObjectDual` is the *only* code path `runVerification()` (the default periodic
loop) ever calls. Every `restore-verifier` instance across the fleet, verifying any bucket, on
every scheduled run, will report every real ARMOR-encrypted object as a checksum conflict. This
has almost certainly been producing "0% verified" (or near-0%) results since whichever
restore-verifier deployments started running `ModeDual` checks against real data — worth
checking Prometheus history / other clusters' restore-verifier logs for the same symptom.

**What is *not* affected**: `ModeDRDrill` (`verifyObjectDirectOnly`) — the actual
disaster-recovery drill path referenced by `docs/disaster-recovery.md` — never calls
`restoreViaARMOR` at all; it only exercises the (correctly implemented) direct-decrypt path.
The real "can we recover data with just the escrowed MEK and B2 credentials if ARMOR itself is
gone" guarantee is intact and not implicated by this bug.

**Operational note**: `restore-verifier-acb` currently runs with `VERIFIER_ESCALATION=false`,
so this has not been auto-filing beads — but it is feeding `recordBucketRun`'s Prometheus gauge
a permanent 0/N `runRatio` for `armor-apexalgo`. Any alerting built on that gauge should be
treated as uninformative until the comparison bug is fixed, not as evidence of an active
incident.

## Hypothesis disposition (per bf-1ebnuz investigation ask)

- **(a) Legacy multipart-era write corruption (ADR-002, versions 0.1.35–0.1.41)**: **Ruled
  out** for all 10 sampled objects. None are multipart (no `x-amz-meta-armor-multipart`
  metadata), all are far below the 5 MiB multipart threshold, and all predate the June 2026 bug
  window's fixes only because they predate needing multipart at all — irrelevant to this
  bug class.
- **(b) restore-verifier comparison bug**: **Confirmed** as the sole explanation. Root cause
  identified and reproduced independently (see above): `restoreViaARMOR`/`B2Backend.Get()`
  never decrypts; `verifyObjectDual` compares a meaningless raw-bytes blob against a correctly
  decrypted plaintext and reports every object as corrupt.
- **(c) something else**: Not needed — (b) fully and exclusively explains the observed
  100% failure rate, with byte-for-byte reproduction of the false-positive hash.

## armor-apexalgo status against bf-1ebnuz's original mandate

bf-1ebnuz asks to "enumerate objects over 5MiB written ... while an affected ARMOR version was
deployed" and verify via armor-decrypt. The 10 objects surfaced by `restore-verifier-acb` are
not part of that >5MiB population (they're all under 6KB) — they came from restore-verifier's
own reservoir sample, not a targeted >5MiB multipart-window enumeration. **A true >5MiB /
multipart-window enumeration of `armor-apexalgo` has not been done by this investigation** and
remains open. What this investigation does establish: the live `iad-acb` ARMOR deployment
(`ronaldraygun/armor:fcbf6d3`) is pinned to commit `fcbf6d3c` (2026-04-30), which **predates**
every relevant fix — the write-side multipart routing fix (`b96d7eb`, 2026-06-10), the
multipart GET/Range read-path fix (`5bc58e0d`, bf-24sxh7, 2026-07-15), and the real
per-part-digest fix (`9f6d5694`, bf-1v2ehf, 2026-07-19). If ai-code-battle's client (or any
writer of this bucket) ever produced a ≥5MiB multipart upload against this ARMOR instance —
unknown, not checked by this investigation — that object would be a genuine ADR-002 casualty,
permanent, unfixable. **Recommended next step for anyone continuing bf-1ebnuz on this
bucket**: enumerate `armor-apexalgo` objects ≥5MiB by size (independent of what
restore-verifier's reservoir sample happens to pick) and check each for the multipart marker
and upload timestamp against the fix-commit dates above. Separately, `iad-acb` running an
8-months-stale, pre-numbered-release commit is itself worth flagging to whoever owns that
deployment, independent of this bug — it means `armor-apexalgo` has never received *any* of
the correctness fixes documented in ADR-002/003/005.

## References

- Pod logs: `kubectl --kubeconfig=~/.kube/rs-manager.kubeconfig logs -n armor -l app=restore-verifier-acb`
- `internal/restoreverifier/verifier.go`: `verifyObjectDual` (:971), `restoreViaARMOR` (:1052),
  `restoreViaDirectDecrypt` (:1087), `plaintextDigestForMetadata` (:109)
- `internal/backend/b2.go`: `B2Backend.Get` (:112), `B2Backend.Head` (:188)
- `cmd/restore-verifier/main.go`: backend construction (:162-173), sole `Backend` passed to
  `restoreverifier.New` (:237-243)
- `armor_deployments.json`: `iad-acb` → `ronaldraygun/armor:fcbf6d3`
- ADR-002, ADR-003, ADR-005; bf-24sxh7 (closed), bf-1v2ehf (closed)
