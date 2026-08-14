# ARMOR Test Failure Catalog

**Generated:** 2026-08-12  
**Test Run:** Full `go test ./internal/... -v`  
**Purpose:** Diagnostic pass before fixing decompression/error-handling test suite

## Summary

- **Total Packages Tested:** 17
- **Passing Packages:** 15 (88.2%)
- **Failing Packages:** 1 (5.9%)
- **Skipped Tests:** 9
- **Timeouts:** 0
- **Panics:** 0

## Package Status Overview

| Package | Status | Cached/Time |
|---------|--------|-------------|
| github.com/jedarden/armor/internal/b2keys | PASS | cached |
| github.com/jedarden/armor/internal/backend | PASS | cached |
| github.com/jedarden/armor/internal/canary | PASS | cached |
| github.com/jedarden/armor/internal/config | PASS | cached |
| github.com/jedarden/armor/internal/crypto | PASS | cached |
| github.com/jedarden/armor/internal/dashboard | PASS | cached |
| github.com/jedarden/armor/internal/keymanager | PASS | cached |
| github.com/jedarden/armor/internal/logging | PASS | cached |
| github.com/jedarden/armor/internal/manifest | PASS | cached |
| github.com/jedarden/armor/internal/metrics | PASS | cached |
| github.com/jedarden/armor/internal/presign | PASS | cached |
| github.com/jedarden/armor/internal/provenance | PASS | cached |
| github.com/jedarden/armor/internal/replication | PASS | cached |
| github.com/jedarden/armor/internal/restoreverifier | PASS | cached |
| **github.com/jedarden/armor/internal/server** | **FAIL** | **31.567s** |
| github.com/jedarden/armor/internal/server/handlers | PASS | cached |
| github.com/jedarden/armor/internal/testutil | PASS | cached |

## Failing Tests

### 1. TestSecondaryBackendInitialization/filesystem_secondary_backend_with_valid_path

**Location:** `internal/server/server_test.go:131`  
**Duration:** 0.35s  
**Error Type:** Assertion Failure - Backend Initialization  
**Error Message:** 
```
expected secondary backend to be initialized, got nil
```

**Context:**
- Test is attempting to verify that a filesystem secondary backend is properly initialized with a valid path
- Server starts but secondary backend remains nil despite valid configuration
- Test logs show server startup completes successfully with warnings about malformed S3 credentials (expected for test environment)

**Categorization:** 
- **Primary:** Secondary Backend Initialization
- **Secondary:** Configuration Validation
- **Impact:** Medium - affects secondary storage replication feature

---

### 2. TestSecondaryBackendFilesystemIntegration

**Location:** `internal/server/server_test.go:271`  
**Duration:** 0.30s  
**Error Type:** Assertion Failure - Backend Initialization  
**Error Message:**
```
expected secondary backend to be initialized
```

**Context:**
- Integration test for filesystem secondary backend functionality
- Test expects secondary backend to be available for filesystem operations
- Server starts successfully but secondary backend is not initialized
- Same pattern as TestSecondaryBackendInitialization failure

**Categorization:**
- **Primary:** Secondary Backend Integration
- **Secondary:** End-to-End Testing
- **Impact:** Medium - blocks verification of secondary storage workflows

**Common Pattern:** Both failures indicate the secondary backend initialization is not working as expected in the test environment, despite what appears to be valid configuration.

---

## Skipped Tests (9 total)

### Integration Tests Requiring External Credentials (7 tests)

| Test Name | Skip Reason | File |
|-----------|-------------|------|
| TestInitB2Backend_BadCredentialsRejection | Requires ARMOR_B2_TEST_{ENDPOINT,REGION,KEY_ID,SECRET,BUCKET} | secondary_init_b2_integration_test.go:55 |
| TestInvalidCredentialsIntegration | Requires INTEGRATION_TEST=1 | invalid_credential_integration_test.go:210 |
| TestMultipleCredentialSets | Requires INTEGRATION_TEST=1 | multi_credential_integration_test.go:17 |
| TestCredentialIsolation | Requires INTEGRATION_TEST=1 | multi_credential_integration_test.go:110 |
| TestCredentialRotation | Requires INTEGRATION_TEST=1 | multi_credential_integration_test.go:207 |
| TestCredentialEdgeCases | Requires INTEGRATION_TEST=1 | multi_credential_integration_test.go:329 |

**Note:** These are legitimate skips for integration tests that require live cloud credentials (B2, S3). This is standard practice.

### Tests Awaiting B2 Secondary Backend Implementation (2 tests)

| Test Name | Skip Reason | File |
|-----------|-------------|------|
| TestSecondaryBackendB2Initialization | B2 secondary backend not implemented; config.Load rejects non-filesystem types per ADR-006 | server_test.go:188 |
| TestSecondaryBackendInvalidB2Config | B2 secondary backend not implemented; incomplete config cannot be distinguished from invalid type | server_test.go:231 |

**Note:** These tests are skipped because the feature they test (B2 secondary backend) does not yet exist. They should be unskipped when B2 secondary support lands.

### Other Skipped Test

| Test Name | Skip Reason | Type |
|-----------|-------------|------|
| TestCompleteMultipartUploadOutOfOrder | Not documented in output | Multipart upload |

---

## Additional Observations

### No Issues Found
- **Timeouts:** 0 tests exceeded timeout limits
- **Panics:** 0 tests caused runtime panics
- **Unexpected Exits:** 0 tests terminated abnormally
- **Build Tag Issues:** No tests were excluded due to missing build tags

### Server Warnings (Expected)
Both failing tests show expected warnings in the logs:
```
manifest startup load failed — continuing with empty manifest index
error: ListObjectsV2 failed: ... InvalidAccessKeyId: Malformed Access Key Id
```
These are **expected** in test environments without valid S3 credentials and do not indicate test failures.

### Test Suite Health
- **Core Functionality:** All 15 passing packages cover core functionality (crypto, config, metrics, handlers, etc.)
- **Decompression Handling:** The new decompression tests (`TestShareGET_CorruptedCompressedData`) are **passing** and properly handling corrupted zstd data with appropriate 500 errors
- **Isolation:** Failures are isolated to secondary backend functionality only

## Recommended Action Order

1. **Fix Secondary Backend Initialization** (2 failures)
   - Investigate why `TestSecondaryBackendInitialization` fails despite valid filesystem path
   - Check configuration loading logic for secondary backend
   - Verify test setup matches expected configuration schema

2. **Verify Integration Test Environment** (optional)
   - Set up credentials for integration tests if needed
   - Or document that these tests should remain skipped in CI

3. **Monitor for Flakiness**
   - Re-run test suite to confirm failures are consistent
   - Check if timing issues contribute to initialization failures

## Files for Investigation

1. `internal/server/server_test.go:131` - First failure location
2. `internal/server/server_test.go:271` - Second failure location
3. `internal/config/config.go` - Configuration loading per ADR-006
4. Secondary backend initialization code path

## Full Test Output

Complete verbose test output saved to: `/tmp/armor-test-full-run.log`

---

**End of Catalog**
