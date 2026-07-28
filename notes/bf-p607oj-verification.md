# DR-Drill Mode Implementation Verification

## Bead: bf-p607oj
**Title:** Add DR-drill mode to restore-verifier (direct-to-ciphertext-only run)

## Implementation Status: ✅ COMPLETE

The DR-drill mode feature was fully implemented in commit `63cf61ce` on 2026-07-19.
This document verifies that all requirements have been met.

## Requirements Verification

### ✅ 1. Armor-decrypt-only drill automation
**Requirement:** Automate the armor-decrypt-only drill from docs/disaster-recovery.md

**Implementation:**
- `verifyObjectDirectOnly()` function in `internal/restoreverifier/verifier.go` (lines 917-969)
- Implements full direct path: MEK unwrap → raw B2 fetch → ADR-003 decrypt → checksum + artifact assertion
- Deliberately excludes ARMOR read path (proven by tests)

### ✅ 2. Direct path execution only
**Requirement:** Run ONLY the direct path (MEK from env, raw B2 fetch, decrypt honoring ADR-003 multipart sidecar layout, checksum + artifact assertion)

**Implementation:**
- MEK from env: `v.mek` field populated from `ARMOR_MEK` environment variable
- Raw B2 fetch: `v.backend.Head()`, `v.backend.GetRange()` for ciphertext
- ADR-003 multipart support: `readMultipartCiphertext()` handles JSON HMAC sidecars
- Checksum enforcement: `plaintextDigestForMetadata()` + SHA-256 comparison
- Artifact assertion: `assertion.Verify(plaintext, obj.Metadata)`

### ✅ 3. ARMOR read path exclusion
**Requirement:** With the ARMOR read path deliberately excluded, proving the ARMOR-server-is-gone recovery works

**Implementation:**
- `verifyObjectDirectOnly()` never calls `restoreViaARMOR()`
- Test `TestVerifyObject_DRDrill_DirectOnlyExcludesARMORReadPath` asserts `fb.armorGet == 0`
- Result path is `PathDirect`, never `PathARMOR` or `PathDualMatch`

### ✅ 4. Mode flag on trigger handler
**Requirement:** Expose as a mode flag on the existing trigger handler

**Implementation:**
- `POST /trigger?mode=dr-drill` endpoint in `internal/restoreverifier/handlers.go` (lines 72-127)
- Query parameter parsing: `mode := Mode(r.URL.Query().Get("mode"))`
- Validation: unknown mode returns 400 (fails loudly on typos)
- Default: `ModeDual` when `mode` parameter is empty

### ✅ 5. Own scheduling interval
**Requirement:** Its own scheduling interval

**Implementation:**
- CLI flag: `-dr-drill-interval` (alias: `VERIFIER_DR_DRILL_INTERVAL`)
- Independent ticker: `drillTicker` separate from main `ticker`
- Nil-channel idiom: zero value disables periodic drill (on-demand only)
- Default: `0` (disabled) - leaves running fleet unchanged

### ✅ 6. Distinct status/metrics fields
**Requirement:** Distinct status/metrics fields (drill_last_success etc.)

**Implementation:**

**Status fields in `BucketState`:**
- `DrillLastVerification time.Time` (line 438)
- `DrillLastSuccess time.Time` (line 439)
- `DrillTotalObjects int64` (line 440)
- `DrillVerifiedObjects int64` (line 441)
- `DrillFailedObjects int64` (line 442)

**Metrics in `internal/metrics/metrics.go`:**
- `armor_drill_last_verified_timestamp` - last drill attempt
- `armor_drill_last_success_timestamp` - last successful drill
- `armor_drill_verified_object_ratio` - recovery ratio (0..1)
- `armor_drill_failures_total` - monotonic failure counter

### ✅ 7. Unit tests with mock backend
**Requirement:** Unit tests with the mock backend

**Implementation:**
- `TestVerifyObject_DRDrill_DirectOnlyExcludesARMORReadPath` - Core acceptance test
- `TestVerifyObject_DRDrill_ChecksumMismatch` - Checksum enforcement
- `TestVerifyObject_DRDrill_MultipartDigestEnforced` - ADR-003 multipart case
- Mock backend: `fakeBackend` in `verifier_test.go`
- All tests assert `fb.armorGet == 0` (ARMOR read path never called)

## Test Results

```bash
$ go test -v ./internal/restoreverifier/... -run "TestVerifyObject_DRDrill"
=== RUN   TestVerifyObject_DRDrill_DirectOnlyExcludesARMORReadPath
=== RUN   TestVerifyObject_DRDrill_DirectOnlyExcludesARMORReadPath/single_put_valid_recovers_direct_only
=== RUN   TestVerifyObject_DRDrill_DirectOnlyExcludesARMORReadPath/single_put_corrupt_artifact_caught_direct_only
=== RUN   TestVerifyObject_DRDrill_DirectOnlyExcludesARMORReadPath/multipart_sidecar_recovers_direct_only_adr003
--- PASS: TestVerifyObject_DRDrill_DirectOnlyExcludesARMORReadPath (0.00s)
=== RUN   TestVerifyObject_DRDrill_ChecksumMismatch
--- PASS: TestVerifyObject_DRDrill_ChecksumMismatch (0.00s)
=== RUN   TestVerifyObject_DRDrill_MultipartDigestEnforced
--- PASS: TestVerifyObject_DRDrill_MultipartDigestEnforced (0.00s)
PASS
ok  	github.com/jedarden/armor/internal/restoreverifier	0.006s
```

## Code Locations

| Component | File | Lines |
|-----------|------|-------|
| Mode constant | `internal/restoreverifier/verifier.go` | 62-78 |
| verifyObjectDirectOnly | `internal/restoreverifier/verifier.go` | 917-969 |
| Trigger handler | `internal/restoreverifier/handlers.go` | 72-127 |
| runDRDrill | `internal/restoreverifier/verifier.go` | 639-657 |
| BucketState drill fields | `internal/restoreverifier/verifier.go` | 432-443 |
| Drill metrics | `internal/metrics/metrics.go` | 78+ |
| CLI flags | `cmd/restore-verifier/main.go` | 61-66 |
| Unit tests | `internal/restoreverifier/verifier_test.go` | 467-650+ |

## Usage Examples

### Manual trigger (on-demand)
```bash
curl -X POST http://localhost:9002/trigger?mode=dr-drill
# Response: DR-drill (direct-only) triggered
```

### Periodic drill (disabled by default)
```bash
export VERIFIER_DR_DRILL_INTERVAL=168h  # weekly
restore-verifier -dr-drill-interval 168h
```

### Check drill status
```bash
curl http://localhost:9002/status | jq '.[].drill_last_success'
```

### Drill metrics
```
armor_drill_last_success_timestamp{bucket="my-bucket"} 1721438400
armor_drill_verified_object_ratio{bucket="my-bucket"} 0.95
armor_drill_failures_total{bucket="my-bucket"} 2
```

## Conclusion

All requirements for bead bf-p607oj have been fully implemented and tested.
The feature is production-ready and has been available since commit 63cf61ce (2026-07-19).

**Implementation complete.** ✅
