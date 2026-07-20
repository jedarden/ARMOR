# bf-3hwoly: Key rotation test coverage — verification record

## Task

Prove MEK rotation works end-to-end at the handler layer with real `internal/crypto`,
especially for multipart objects, across six requirements, with `go test -race` green.

## Finding

The implementation and its test coverage are **already complete, committed, and on
`origin/main`** (tests `0dc7ccfc`; rotation code `636e0b40` / `f356d54d`; runbook
present at `docs/key-rotation-runbook.md`). This bead was re-dispatched after the
work landed. I independently re-verified every requirement against the current
tree rather than trusting the commit history.

### Requirements traceability

| # | Requirement | Location | Status |
|---|-------------|----------|--------|
| 1 | Mixed prefix (single-PUT + multipart) → new MEK; decrypts NEW / fails OLD; plaintext + ETags unchanged | `TestKeyRotationMixedPrefixPreservesMultipart` | ✅ verified |
| 2 | Rotation preserves ALL `x-amz-meta-armor-*` on multipart objects (marker + part-size + wrapped-DEK), does NOT touch HMAC sidecar, object round-trips (guards bf-24sxh7) | `TestKeyRotationMixedPrefixPreservesMultipart` + `rotateObject` raw-metadata clone (`key_rotation.go:264`) | ✅ verified |
| 3 | Interrupted rotation resumes: kill mid-flight, re-run, idempotent completion, no object left old-wrapped | `TestKeyRotationInterruptedResume` (crash on 101st Copy after the 100-object periodic save) | ✅ verified |
| 4 | Non-ARMOR passthrough objects skipped untouched | `TestKeyRotationPassthroughUnchanged` + `TestKeyRotationSkipsNonARMORObjects` | ✅ verified |
| 5 | B2 CopyObject size ceiling: clear typed error, enumerated as EXCEPTIONS not silently skipped | `TestRotateObjectRejectsOversizedWithTypedError` + `TestKeyRotationB2CopyObjectCeiling` + runbook §"B2 CopyObject size ceiling" | ✅ verified |
| 6 | Operational runbook ordering documented: OpenBao → ESO sync → rotate → pod restart; admin API must never diverge from OpenBao (bf-5m9nde auth) | `docs/key-rotation-runbook.md` §"Required ordering" + Step 3 callout | ✅ verified |
| 7 | `go test -race` green | see below | ✅ verified |

### Critical path I scrutinized (item 2 — bf-24sxh7 regression guard)

`rotateObject` resolves the object's **full raw metadata** (`objectMetadata`, from
List or Head), then clones that map and overwrites **only** `x-amz-meta-armor-wrapped-dek`
before `Copy(..., replaceMetadata=true)`. It deliberately does *not* rebuild metadata
from `ARMORMetadata.ToMetadata()` — that path omits `armor-multipart`/`armor-part-size`
and would silently brick every rotated multipart object. The real B2 backend sets
`MetadataDirective = types.MetadataDirectiveReplace` (`b2.go:346`), which overwrites
the *entire* metadata set, so preserving the raw map is load-bearing. The test's
mock `Copy(replaceMetadata=true)` reproduces those exact REPLACE semantics, so the
test genuinely exercises the bug class — not a tautology.

The multipart test additionally writes a sidecar at `.armor/hmac/<sha256(key)>`
(mirroring `backend.MultipartStateManager.SaveHMACTable`) and asserts it is
byte-identical + still parseable after rotation. Rotation skips all `.armor/` keys
(`key_rotation.go:175`), so the sidecar is never copied.

### Verification commands

```bash
# 11 rotation tests, race detector on, all PASS (1.06s)
$ go test -race -count=1 -v -run 'TestKeyRotation|TestRotateObject' ./internal/server/
--- PASS: TestKeyRotationWithManifestIndex
--- PASS: TestKeyRotation
--- PASS: TestKeyRotationResumption
--- PASS: TestKeyRotationStatePersistence
--- PASS: TestKeyRotationSkipsNonARMORObjects
--- PASS: TestKeyRotationSkipsInternalObjects
--- PASS: TestKeyRotationMixedPrefixPreservesMultipart
--- PASS: TestKeyRotationPassthroughUnchanged
--- PASS: TestKeyRotationInterruptedResume
--- PASS: TestRotateObjectRejectsOversizedWithTypedError
--- PASS: TestKeyRotationB2CopyObjectCeiling
PASS
ok  	github.com/jedarden/armor/internal/server	1.060s

$ go vet ./internal/server/        # clean (exit 0)
```

### Notes for a future operator

- Resume `LastKey` cursor uses lexicographic `<=`, which matches both mock and
  real B2/S3 `ListObjectsV2` ordering. A pre-existing object with a key
  lexicographically `<= LastKey` created *after* rotation started would be skipped
  on resume — an inherent limitation of cursor-based resume, out of scope here.
- `RotationResult.ProcessedObjects` on a resumed run reflects only that run's
  work, while the persisted `.armor/rotation-state.json` `ProcessedObjects` is
  cumulative. The persisted state is the source of truth for progress.

No source changes were required; this record is the deliverable.
