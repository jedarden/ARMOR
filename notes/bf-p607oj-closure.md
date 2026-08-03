# Bead bf-p607oj Closure Summary

**Date:** 2026-08-03
**Bead:** Add DR-drill mode to restore-verifier (direct-to-ciphertext-only run)
**Status:** CLOSED - Feature already implemented

## Background

The DR-drill mode feature was fully implemented in commit `63cf61ce` on 2026-07-19 and verified as complete in commit `b3947ffa`. However, the bead bf-p607oj was never formally closed.

## Implementation Summary

The DR-drill mode automates the armor-decrypt-only drill from `docs/disaster-recovery.md`. It runs ONLY the direct path:
- MEK unwrap from environment
- Raw B2 fetch (bypassing ARMOR server)
- ADR-003-compliant decryption (envelope for single-PUT, bare ciphertext + JSON sidecar for multipart)
- Checksum verification
- Artifact assertion

## Implementation Details

### Mode Selection
- `POST /trigger?mode=dr-drill` - On-demand DR drill trigger
- Default mode: `dual` (both ARMOR and direct paths)
- Unknown modes return 400 (fail loudly on typos)

### Scheduling
- Independent DR-drill ticker: `-dr-drill-interval` / `VERIFIER_DR_DRILL_INTERVAL`
- Default: 0 (disabled) - on-demand only
- Separate from dual-path check interval

### Status/Metrics
Separate drill-specific fields in `BucketState`:
- `DrillLastVerification`, `DrillLastSuccess`
- `DrillTotalObjects`, `DrillVerifiedObjects`, `DrillFailedObjects`

Prometheus metrics:
- `armor_drill_last_verified_timestamp`
- `armor_drill_last_success_timestamp`
- `armor_drill_verified_object_ratio`
- `armor_drill_failures_total`

### Testing
Unit tests with mock backend (`fakeBackend`):
- `TestVerifyObject_DRDrill_DirectOnlyExcludesARMORReadPath` - Core acceptance test
- `TestVerifyObject_DRDrill_ChecksumMismatch` - Checksum enforcement
- `TestVerifyObject_DRDrill_MultipartDigestEnforced` - ADR-003 multipart case

All tests assert ARMOR read path is never called (`armorGet == 0`).

## References
- Implementation: commit 63cf61ce
- Verification: commit b3947ffa
- Detailed verification notes: `notes/bf-p607oj-verification.md`
- ADR-003 multipart layout: `docs/adr/003-multipart-object-layout-and-read-path.md`

## Closure Action

Bead closed as feature already implemented and verified. No new code changes required.
