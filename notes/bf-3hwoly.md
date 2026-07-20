# bf-3hwoly: Key rotation test coverage — verification & landing record

## Task

Prove MEK rotation works end-to-end at the handler layer with real `internal/crypto`,
especially for multipart objects, across six requirements, with `go test -race` green.

## What landed

The bead's deliverables existed as **uncommitted working-tree changes** when the bead
was dispatched (the session-start `git status` was truncated at 2k chars and hid
them). They are now committed in this bead's commit:

- `internal/server/key_rotation.go` (+156/−38) — the `B2CopyObjectSizeCeiling` /
  `ErrCopyObjectTooLarge` machinery, and the raw-metadata-preserving `rotateObject`
  (clones the full metadata map and overwrites only `armor-wrapped-dek`, so the
  multipart markers survive the `MetadataDirective=REPLACE` copy).
- `internal/server/key_rotation_test.go` (+623) — the five new tests below plus
  helpers (`createTestMultipartARMORObject`, `crashBackend`, sidecar assertions).
- `docs/key-rotation-runbook.md` (new) — the operational runbook.
- `docs/disaster-recovery.md` (+2) — cross-link to the runbook from the failure-
  recovery section.

## Requirements traceability

| # | Requirement | Location | Status |
|---|-------------|----------|--------|
| 1 | Mixed prefix (single-PUT + multipart) → new MEK; decrypts NEW / fails OLD; plaintext + ETags unchanged | `TestKeyRotationMixedPrefixPreservesMultipart` | ✅ |
| 2 | Rotation preserves ALL `x-amz-meta-armor-*` on multipart objects (marker + part-size + wrapped-DEK), does NOT touch HMAC sidecar, object round-trips (guards bf-24sxh7) | `TestKeyRotationMixedPrefixPreservesMultipart` + `rotateObject` raw-metadata clone | ✅ |
| 3 | Interrupted rotation resumes: kill mid-flight, re-run, idempotent completion, no object left old-wrapped | `TestKeyRotationInterruptedResume` (crash on 101st Copy after the 100-object periodic save) | ✅ |
| 4 | Non-ARMOR passthrough objects skipped untouched | `TestKeyRotationPassthroughUnchanged` + `TestKeyRotationSkipsNonARMORObjects` | ✅ |
| 5 | B2 CopyObject size ceiling: clear typed error, enumerated as EXCEPTIONS not silently skipped | `TestRotateObjectRejectsOversizedWithTypedError` + `TestKeyRotationB2CopyObjectCeiling` + runbook §"B2 CopyObject size ceiling" | ✅ |
| 6 | Runbook ordering documented: OpenBao → ESO sync → rotate → pod restart; admin API must never diverge from OpenBao (bf-5m9nde auth) | `docs/key-rotation-runbook.md` §"Required ordering" + Step 3 callout | ✅ |
| 7 | `go test -race` green | see below | ✅ |

### Why the multipart guard is a real test, not a tautology

`rotateObject` resolves the object's **full raw metadata** and clones it before
overwriting only `armor-wrapped-dek`. It deliberately does *not* rebuild via
`ARMORMetadata.ToMetadata()` (which omits `armor-multipart`/`armor-part-size`).
The real B2 backend sets `MetadataDirective = types.MetadataDirectiveReplace`
(`b2.go:346`), which overwrites the *entire* metadata set, so preserving the raw
map is load-bearing. The test's mock `Copy(replaceMetadata=true)` reproduces those
exact REPLACE semantics — a dropped marker would fail the assertion, proving the
test exercises the bf-24sxh7 bug class.

### Verification commands

```bash
# 11 rotation tests, race detector on, all PASS (~1.08s)
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
ok  	github.com/jedarden/armor/internal/server	1.080s

$ go vet ./internal/server/        # clean (exit 0)
```

### Notes for a future operator

- Resume `LastKey` cursor uses lexicographic `<=`, matching both mock and real
  B2/S3 `ListObjectsV2` ordering. A pre-existing object with a key lexicographically
  `<= LastKey` created *after* rotation started would be skipped on resume — an
  inherent limitation of cursor-based resume, out of scope here.
- `RotationResult.ProcessedObjects` on a resumed run reflects only that run's work;
  the persisted `.armor/rotation-state.json` `ProcessedObjects` is cumulative and is
  the source of truth for progress.

### Correction

An earlier commit in this bead (`3eb890ba`) recorded a notes-only deliverable based
on the truncated `git status`, incorrectly concluding the work was already committed.
It was not — the comprehensive tests, runbook, and code were uncommitted working-tree
changes. This commit lands them; the record above supersedes the earlier note.
