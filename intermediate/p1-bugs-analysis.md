# Phase 6 P1 Bugs Analysis

**Date:** 2026-08-13
**Related ADRs:** ADR-007, ADR-009
**Impact:** Fleet-wide reliability undermined

## Summary

The Genesis bead bf-42dng8 was created to track Phase 6 implementation. However, the core implementation is actually COMPLETE. The remaining work is fixing two P1 bugs that undermine the entire system fleet-wide, plus operational tasks.

## P1 Bug #1: Discovery Reliability (ADR-007)

**Impact:** 3 of 5 production deployments give false "no objects found"

### Bug A: ARMOR_PREFIX Blindness
**Problem:** `restore-verifier` never applies `ARMOR_PREFIX` when listing objects
**Affected:** `iad-kalshi` (actively writing hourly, but verifier reports empty)
**Root Cause:**
- `BucketConfig.Prefix` is parsed but never read
- Env-driven fallback doesn't read `ARMOR_PREFIX` at all
- Both `getLatestObject` and `getHistoricalSample` hardcode empty prefix

### Bug B: Pagination Swamping
**Problem:** `getLatestObject` makes unpaginated 100-object call that gets swamped by `.armor/*` bookkeeping objects
**Affected:** `iad-ci` (26,219 real objects vs 29,303 `.armor/*` objects), likely `rs-manager`
**Root Cause:**
- Single `List(..., maxKeys=100)` call
- `.armor/*` keys sort first and accumulate monotonically
- Page becomes 100% noise after filtering
- `getHistoricalSample` already has correct pagination; `getLatestObject` doesn't

**Note:** `getHistoricalSample` shares Bug A but not Bug B.

## P1 Bug #2: ARMOR Path False-Positive (ADR-009)

**Impact:** Every ModeDual run reports every real ARMOR-encrypted object as corrupted (false positive)

### The Bug
**Problem:** `restoreViaARMOR` never decrypts - it's just a second raw B2 read
**Affected:** All fleet instances, all buckets, all ARMOR-encrypted objects
**Root Cause:**
- `restoreViaARMOR` calls `v.backend.Get()` which is raw `B2Backend.Get()`
- No crypto client, no decryption
- Fetches meaningless byte range (header + partial ciphertext)
- Guaranteed SHA-256 mismatch vs correctly-decrypted direct path
- Confirmed via byte-for-byte reproduction on `armor-apexalgo` bucket

**What's NOT affected:** `ModeDRDrill` (the actual DR path) never calls `restoreViaARMOR`

### Why This is Worse Than Discovery Bugs
- Discovery bugs = false negatives (no signal produced)
- This bug = false positives (active-looking "corruption" signal)
- Could trigger unnecessary MEK rotation or re-upload responses
- Makes historical metrics uninformative

## Actual Implementation Status (Code Review Verified)

### ✅ COMPLETE:
- Dual-path harness (`cmd/restore-verifier`, `internal/restoreverifier`)
- **Application-level assertions:** SQLite (PRAGMA integrity_check), Parquet (footer parse), tar.gz (extraction validation)
- **Storm-proof failure escalation:** dedupe set, staleness window, no retry loops
- **DR-drill mode:** ModeDRDrill, verifyObjectDirectOnly, on-demand + periodic support
- **Deployment:** All buckets via declarative-config
- **Metrics and alerting:** Gauges + PrometheusRules (but no Prometheus scraping)

### ❌ REMAINING:
- P1 Bug #1: Discovery reliability (both bugs A and B)
- P1 Bug #2: ARMOR path false-positive
- Multipart-era corruption audit (credential-gated)
- Enable DR-drill cadence in deployments (defaults to disabled)
- Add Prometheus scraping (clusters use VictoriaLogs)

## Bead Status Analysis

The Genesis bead progress checklist is **significantly out of date**:
- Marked as "stubs today": ❌ Artifact assertions are **fully implemented**
- Marked as not done: ❌ Escalation is **fully implemented**
- Marked as not done: ❌ DR-drill mode is **fully implemented**
- Marked as ops-gated: ❌ Deployment is **done via declarative-config**

The remaining work is the P1 bugs + operational tasks, not core implementation.

## Recommendation

The Genesis bead bf-42dng8 should be updated to:
1. Mark core implementation items as complete
2. Focus remaining work on the P1 bugs
3. Break out the P1 bugs into separate tracking beads (they appear to have been lost when the beads db was corrupted)
4. Track operational items separately (DR-drill cadence, Prometheus scraping)

The core Phase 6 vision from ADR-004 is implemented. What remains is fixing critical bugs that prevent it from working fleet-wide.
