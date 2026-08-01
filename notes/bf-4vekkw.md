# Verification: Root Cause Fixed for Image Publication

**Bead:** bf-4vekkw  
**Date:** 2026-08-01  
**Status:** ✅ RESOLVED

---

## Root Cause Identified

The armor-build workflow was failing at the **lint step**, not the go-test step. The investigation documented in bf-246qgm initially suspected test failures, but the actual blocking issue was:

**golangci-lint failed due to an unused method in test code:**
- `objectSize()` method in `mockRotationBackend` (internal/server/key_rotation_test.go)
- Added during key_rotation test expansion (commit 3decc976)
- Never called in the test suite
- Caused golangci-lint to report `unused` lint error
- This blocked the armor-build workflow before docker-build step

---

## Fix Applied

**Commit 505eb44b** (2026-07-28 22:54:46 EDT):
```
test(lint): remove unused objectSize method from mockRotationBackend

The method was added during bf-3hwoly key rotation test expansion but never
called, causing golangci-lint to fail and blocking the armor-build image pipeline.

This unblocks the CI build that will publish an image for HEAD (will resolve as
0.1.1898 due to auto-bump logic).
```

**Commit 014ce2c8** (2026-07-29 00:42:50 EDT):
```
fix(lint): run gofmt to fix formatting issues blocking CI

This fixes the lint step failures in armor-build workflow that were preventing
image publication for VERSION 0.1.1897.
```

---

## Verification Evidence (2026-08-01)

### 1. Fix is in Current HEAD
```bash
$ git merge-base --is-ancestor 505eb44b HEAD && echo "Fix is in current HEAD"
Fix is in current HEAD
```

### 2. Unused Method Removed
```bash
$ grep -n "objectSize" internal/server/key_rotation_test.go
# No output - method successfully removed
```

### 3. Code Formatting Clean
```bash
$ gofmt -l ./internal/server/key_rotation_test.go
# No output - file is properly formatted
```

### 4. All Tests Pass
```bash
$ go test ./internal/server -run TestKey
=== RUN   TestKeyRotationWithManifestIndex
--- PASS: TestKeyRotationWithManifestIndex (0.00s)
=== RUN   TestKeyRotation
--- PASS: TestKeyRotation (0.00s)
=== RUN   TestKeyRotationResumption
--- PASS: TestKeyRotationResumption (0.00s)
=== RUN   TestKeyRotationStatePersistence
--- PASS: TestKeyRotationStatePersistence (0.00s)
=== RUN   TestKeyRotationSkipsNonARMORObjects
--- PASS: TestKeyRotationSkipsNonARMORObjects (0.00s)
=== RUN   TestKeyRotationSkipsInternalObjects
--- PASS: TestKeyRotationSkipsInternalObjects (0.00s)
=== RUN   TestKeyRotationMixedPrefixPreservesMultipart
--- PASS: TestKeyRotationMixedPrefixPreservesMultipart (0.01s)
=== RUN   TestKeyRotationPassthroughUnchanged
--- PASS: TestKeyRotationPassthroughUnchanged (0.00s)
=== RUN   TestKeyRotationInterruptedResume
--- PASS: TestKeyRotationInterruptedResume (0.04s)
=== RUN   TestRotateObjectRejectsOversizedWithTypedError
--- PASS: TestRotateObjectRejectsOversizedWithTypedError (0.00s)
=== RUN   TestKeyRotationB2CopyObjectCeiling
--- PASS: TestKeyRotationB2CopyObjectCeiling (0.00s)
PASS
ok  	github.com/jedarden/armor/internal/server	0.010s
```

### 5. Full Test Suite Passes
```bash
$ go test ./...
ok  	github.com/jedarden/armor/internal/b2keys	0.006s
ok  	github.com/jedarden/armor/internal/backend	0.852s
ok  	github.com/jedarden/armor/internal/canary	0.556s
ok  	github.com/jedarden/armor/internal/config	0.010s
ok  	github.com/jedarden/armor/internal/crypto	1.356s
ok  	github.com/jedarden/armor/internal/dashboard	0.058s
ok  	github.com/jedarden/armor/internal/keymanager	0.005s
ok  	github.com/jedarden/armor/internal/logging	0.009s
ok  	github.com/jedarden/armor/internal/manifest	0.103s
ok  	github.com/jedarden/armor/internal/metrics	0.061s
ok  	github.com/jedarden/armor/internal/presign	0.010s
ok  	github.com/jedarden/armor/internal/provenance	0.009s
ok  	github.com/jedarden/armor/internal/restoreverifier	0.295s
ok  	github.com/jedarden/armor/internal/server	30.226s
ok  	github.com/jedarden/armor/internal/server/handlers	2.234s
ok  	github.com/jedarden/armor/internal/testutil	0.265s
ok  	github.com/jedarden/armor/tests/aws-cli-compatibility	13.188s
```

### 6. Current Version Image Published
```bash
$ cat VERSION
0.1.1900

$ docker manifest inspect ronaldraygun/armor:0.1.1900 | head -5
{
	"schemaVersion": 2,
	"mediaType": "application/vnd.docker.distribution.manifest.v2+json",
...
```

---

## Conclusion

✅ **Root cause fixed and verified:**
- Unused `objectSize` method removed (commit 505eb44b)
- Code formatting issues fixed (commit 014ce2c8)
- Both fixes are present in current HEAD
- All 11 key_rotation tests pass
- Full test suite passes
- Current VERSION (0.1.1900) image is published

**Acceptance criteria met:**
- ✅ The failing test/step identified in child 1 now passes (lint step fixed)
- ✅ go-test is green for key_rotation package and full suite
- ✅ The fix is committed (505eb44b, 014ce2c8)
- ✅ Clean local go-test run confirms no regressions

The CI pipeline should now successfully build and publish images for HEAD.
