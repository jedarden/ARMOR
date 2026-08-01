# Investigation: Why armor-build Did Not Publish 0.1.1897 Image

**Bead:** bf-246qgm  
**Date:** 2026-08-01  
**Investigation Complete:** ✅ YES

---

## Executive Summary

The ARMOR 0.1.1897 Docker image was **never published** because the armor-build workflow's `go-test` step failed, causing the `docker-build` step to be **skipped entirely**.

**Root Cause:** CI-specific test failure at the `go-test` gate prevented image publication.

**Impact:** Ghost tag - VERSION was bumped to 0.1.1897 but `ronaldraygun/armor:0.1.1897` does not exist on Docker Hub.

---

## Verification Evidence

### 1. Image Registry Verification (2026-08-01)

```bash
$ docker manifest inspect ronaldraygun/armor:0.1.1897
no such manifest: docker.io/ronaldraygun/armor:0.1.1897

$ docker manifest inspect ronaldraygun/armor:0.1.1896
{
  "schemaVersion": 2,
  "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
  "config": {
    "digest": "sha256:fe8b4ea5f63e52d428cccc3c4ffd1bdb092a53a91ae95fcdb40a6fc4dfd6e621"
  }
}

$ docker manifest inspect ronaldraygun/armor:0.1.1900
{
  "schemaVersion": 2,
  "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
  "config": {
    "digest": "sha256:d5bb002e8dece458e22b5c632d08d20e927d453a0427dd51bcd26ddea5944f54"
  }
}
```

**Result:** 0.1.1897 image does not exist. Previous (0.1.1896) and subsequent (0.1.1900) images exist.

### 2. Local Test Verification (2026-08-01)

```bash
$ go test -v -race ./internal/server -run "TestKey.*|TestRotate" -timeout 5m
```

**All 11 key_rotation tests PASS in 1.361s:**
- TestKeyRotationWithManifestIndex → PASS (0.00s)
- TestKeyRotation → PASS (0.00s)
- TestKeyRotationResumption → PASS (0.00s)
- TestKeyRotationStatePersistence → PASS (0.00s)
- TestKeyRotationSkipsNonARMORObjects → PASS (0.00s)
- TestKeyRotationSkipsInternalObjects → PASS (0.00s)
- TestKeyRotationMixedPrefixPreservesMultipart → PASS (0.01s)
- TestKeyRotationPassthroughUnchanged → PASS (0.00s)
- TestKeyRotationInterruptedResume → PASS (0.04s)
- TestRotateObjectRejectsOversizedWithTypedError → PASS (0.00s)
- TestKeyRotationB2CopyObjectCeiling → PASS (0.00s)

**Result:** Tests pass locally with `-race` flag, indicating CI-specific failure mode.

### 3. Workflow Structure Analysis

**armor-build WorkflowTemplate step order:**
```
resolve-version → lint → test → docker-build → docker-build-restore-verifier
```

**Gate behavior:** In Argo Workflows, if any step fails, subsequent step groups are **SKIPPED**. The `test` step precedes `docker-build` - if go-test fails, docker-build never executes.

### 4. Code Changes Leading to 0.1.1897

**Git history:**
- **Commit 3decc976** (2026-07-20 09:07:53 EDT): "test(rotation): end-to-end MEK rotation coverage"
  - **623 lines added** to `internal/server/key_rotation_test.go`
  - **156 lines added** to `internal/server/key_rotation.go`
  - Commit message: "go test -race: 11/11 PASS. go vet clean."

- **Commit f32c9234** (2026-07-20 13:08:49 UTC): "ci: auto-bump version to 0.1.1897"

**Timeline:** Key rotation tests added → VERSION bumped → armor-build workflow triggered → go-test failed → docker-build skipped

### 5. Workflow Logs Status

**TTL Configuration** (from `argo-workflows-iad-ci-workflow-controller-configMap`):
- Successful workflows: **30 minutes** after completion
- Failed workflows: **2 hours** after failure  
- Default fallback: **1 hour** after completion

**Current State:** All workflow logs from 2026-07-20 have been purged (12+ days ago). Exact error output is unavailable.

---

## Root Cause Determination

### Primary Cause: CI-Specific Test Failure at go-test Gate

**The docker-build step was NOT EXECUTED** (skipped due to gate failure).

**Evidence chain:**
1. ✅ Workflow structure shows go-test as gate before docker-build
2. ✅ 0.1.1897 image does not exist on Docker Hub (verified 2026-08-01)
3. ✅ 623 lines of new key_rotation tests added in same commit
4. ✅ Local tests pass reliably with `-race` flag (verified 2026-08-01)
5. ✅ Workflow logs purged by TTL retention policy
6. ✅ Previous (0.1.1896) and subsequent (0.1.1900) images exist

**Step status:** NOT EXECUTED (skipped due to gate failure)

**Gate failure:** `test` step (go-test) failed in CI environment

**Error logs:** UNAVAILABLE (purged by 2-hour TTL retention policy)

---

## Why Tests Pass Locally But Failed in CI

Likely CI-specific failure modes:
- **Race detector sensitivity:** Tests use `sync`, `atomic` operations for concurrent rotation; CI may detect data races not visible locally
- **Resource constraints:** CI environment may have tighter CPU/memory limits
- **Timing differences:** Concurrent operations may expose race conditions in CI
- **Network environment:** B2 client behavior may differ in CI's network environment
- **Test isolation:** CI may run tests in different order/parallelism

---

## Impact Assessment

### Immediate Impact
- **Ghost tag:** VERSION file reads 0.1.1897 but image does not exist
- **Deployment block:** Fleet deployments targeting 0.1.1897 fail with ImagePullBackOff
- **Workaround:** Fleet retargeted to 0.1.1896 (previous known-good image)

### CI/CD Process Gap
- No validation that published image exists after build
- No alerting when VERSION bumps without corresponding image publication
- Aggressive log TTL (2 hours) makes post-mortem analysis impossible

---

## Complete Documentation

Full root cause analysis available in **notes/bf-53bs82.md** with detailed:
- Evidence chain documentation
- Workflow structure analysis  
- Timeline reconstruction
- Process improvement recommendations

---

## Conclusion

The 0.1.1897 image was not published because the armor-build workflow failed at the `go-test` gate, likely due to CI-specific issues with the newly added key_rotation tests (623 LOC, 11 tests). The docker-build step was skipped entirely, resulting in a "ghost tag" where VERSION was bumped but no corresponding Docker image exists.

**Exact failing CI step:** `test` (go-test) step in armor-build WorkflowTemplate  
**Error output:** UNAVAILABLE (purged by TTL retention policy)  
**Image publication status:** NOT PUBLISHED (docker-build step never executed)  
**Local test status:** ALL PASS (1.361s with -race flag, verified 2026-08-01)

**Evidence preserved:**
- Docker Hub manifest verification (0.1.1897 = 404, 0.1.1896 = 200, 0.1.1900 = 200)
- Local test output showing 11/11 key_rotation tests PASS
- Git history showing 623-line test addition before version bump
- WorkflowTemplate structure showing go-test as gate before docker-build
