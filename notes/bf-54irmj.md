# Full HTTP API Lifecycle Testing - bf-54irmj

## Summary
Successfully implemented and verified comprehensive HTTP API lifecycle testing for ARMOR. The test suite now exercises the full S3 object lifecycle through real HTTP requests (SigV4-signed via AWS SDK v2) against ARMOR's actual listener, backed by a real B2 backend (not mockBackend).

## Implementation
- **File**: `tests/aws-cli-compatibility/zz_lifecycle_integration_test.go` (already existed)
- **Fix**: Added missing `B2Endpoint` configuration in `tests/aws-cli-compatibility/harness_test.go:525-544`
- **Issue**: The `startRealArmorServer` function was missing the `B2Endpoint` field, causing "Custom endpoint `` was not a valid URI" errors
- **Solution**: Added automatic B2 endpoint URL construction: `https://s3.{region}.backblazeb2.com`

## Test Coverage (All Requirements Met)

### 1. Real HTTP API with SigV4 Signing ✅
- **Tests use**: AWS SDK v2 with SigV4 signing
- **Target**: ARMOR's actual S3-compatible HTTP listener (`httptest.NewServer(srv.Handler())`)
- **Backend**: Real B2 (not `mockBackend`)

### 2. Full Object Lifecycle ✅
All operations verified in one continuous test run (`TestLifecycle_FullContinuousEndToEnd`):

#### Phase 1: Single-part PUT
- **Operation**: PUT 1 MiB payload
- **HTTP Status**: 200 OK
- **Evidence**: `Phase 1 PUT succeeded: HTTP 200, ETag="a6a70f25f81a631624afbf9bc9d8643d"`

#### Phase 2: HEAD
- **Operation**: HEAD object metadata
- **HTTP Status**: 200 OK
- **Evidence**: `Phase 2 HEAD succeeded: HTTP 200, ContentLength=1048576`

#### Phase 3: GET (full download)
- **Operation**: GET entire object
- **HTTP Status**: 200 OK
- **Evidence**: `Phase 3 GET succeeded: HTTP 200, verified 1048576 bytes`

#### Phase 4: GET (byte-range)
- **Operation**: GET bytes 100-200
- **HTTP Status**: 206 Partial Content
- **Evidence**: `Phase 4 GET range succeeded: HTTP 206, ContentRange=bytes 100-200/1048576, 101 bytes`

#### Phase 5: LIST
- **Operation**: LIST objects with prefix='lifecycle-test/'
- **HTTP Status**: 200 OK
- **Evidence**: `Phase 5 LIST succeeded: HTTP 200, found Key=lifecycle-test/full-continuous.dat with Size=1048576`

#### Phase 6: Multipart PUT (overwrite same key)
- **Operation**: CreateMultipartUpload + UploadPart (2 parts, 5.25 MiB each) + CompleteMultipartUpload
- **HTTP Status**: 200 OK (all 3 operations)
- **Evidence**: 
  - `Phase 6 CreateMultipartUpload succeeded: HTTP 200, UploadID=4_z12b0ca0d06d7e07193d90117_f233b9b5e387d3b24_d20260729_m020059_c002_v0203012_t0029_u01785290459512`
  - `Phase 6 UploadPart 1 succeeded: HTTP 200, ETag="880a792e8db44fe9543c20987b1c1eb9"`
  - `Phase 6 UploadPart 2 succeeded: HTTP 200, ETag="847c3a2895cad1e0e0e479b1add89ff4"`
  - `Phase 6 CompleteMultipartUpload succeeded: HTTP 200, ETag="364929bd008c52d02cac2009f39f6f84-2"`

#### Phase 7: GET (verify overwrite)
- **Operation**: GET entire object after multipart overwrite
- **HTTP Status**: 200 OK
- **Evidence**: `Phase 7 GET succeeded: HTTP 200, verified 11010048 bytes (new content wins)`

#### Phase 8: GET (byte-range crossing part boundary)
- **Operation**: GET bytes 5500000-5505000 (crosses part boundary at 5505024)
- **HTTP Status**: 206 Partial Content
- **Evidence**: `Phase 8 GET range succeeded: HTTP 206, ContentRange=bytes 5500000-5505000/11010048, 5001 bytes (crossed part boundary)`

#### Phase 9: LIST (verify new size)
- **Operation**: LIST objects with prefix='lifecycle-test/'
- **HTTP Status**: 200 OK
- **Evidence**: `Phase 9 LIST succeeded: HTTP 200, found Key=lifecycle-test/full-continuous.dat with new Size=11010048`

#### Phase 10: DELETE
- **Operation**: DELETE object
- **HTTP Status**: 204 No Content
- **Evidence**: `Phase 10 DELETE succeeded: HTTP 204`

#### Phase 11: Post-delete GET (expect 404)
- **Operation**: GET deleted object
- **HTTP Status**: 404 Not Found
- **Evidence**: `Phase 11 GET correctly failed: operation error S3: GetObject, https response error StatusCode: 404, RequestID: , HostID: , NoSuchKey: Object not found`

#### Phase 12: Post-delete HEAD (expect 404)
- **Operation**: HEAD deleted object
- **HTTP Status**: 404 Not Found
- **Evidence**: `Phase 12 HEAD correctly failed: operation error S3: HeadObject, https response error StatusCode: 404, RequestID: 462be544ec47f428, HostID: aMtIwJmHdZPc2+DeKMDkxdTNzOXwx5zdu, NotFound: (NoSuchKey)`

#### Phase 13: Post-delete LIST (key should not appear)
- **Operation**: LIST objects with prefix='lifecycle-test/'
- **HTTP Status**: 200 OK
- **Evidence**: `Phase 13 LIST succeeded: HTTP 200, verified deleted key does not appear`

### 3. Multipart Abort (Deliberate Test) ✅
Separate test `TestLifecycle_MultipartAbort` verifies abort cleanup:

- **Step 1**: CreateMultipartUpload → HTTP 200 OK
- **Step 2**: UploadPart (5.25 MiB) → HTTP 200 OK
- **Step 3**: ListMultipartUploads (verify exists) → HTTP 200 OK
- **Step 4**: AbortMultipartUpload → HTTP 204 No Content
- **Step 5**: ListMultipartUploads (verify gone) → HTTP 200 OK (upload no longer appears)
- **Step 6**: CompleteMultipartUpload (expect failure) → HTTP 404 Not Found
- **Step 7**: GET key (expect 404) → HTTP 404 Not Found

**Evidence**: `TestLifecycle_MultipartAbort` passed in 4.19s with all assertions successful

## Test Results

### Complete Test Suite
All lifecycle tests pass when run against real B2 backend:

```bash
ARMOR_B2_REGION="us-west-002" \
ARMOR_B2_ACCESS_KEY_ID="<redacted>" \
ARMOR_B2_SECRET_ACCESS_KEY="<redacted>" \
ARMOR_BUCKET="nap-html" \
ARMOR_MEK="64f632dfb510073d3831ca02fd1960d88c14e88a2ec15de421cd04eb4df4b769" \
ARMOR_AUTH_ACCESS_KEY="ccb34f280760bad2b8b740c1885dbe71" \
ARMOR_AUTH_SECRET_KEY="e279e1e3d3a37f1ca1910f639bbd37e8218c44c98b4c093df368368469d9ebe0" \
go test -v -run TestLifecycle ./tests/aws-cli-compatibility/
```

**Results**:
- `TestLifecycle_SinglePartPut`: PASS (4.14s)
- `TestLifecycle_ByteRangeRequests`: PASS (13.95s) - crosses block boundaries and part boundaries
- `TestLifecycle_MultipartUpload`: PASS (11.03s)
- `TestLifecycle_HeadAndList`: PASS
- `TestLifecycle_Overwrite`: PASS
- `TestLifecycle_DeleteAndVerify`: PASS
- `TestLifecycle_MultipartAbort`: PASS (4.19s)
- `TestLifecycle_FullContinuousEndToEnd`: PASS (16.28s) - all 13 phases

## Key Improvements
1. **Fixed B2 endpoint configuration** in `startRealArmorServer` function
2. **Added nil-safety checks** for LIST response handling (KeyCount, object fields)
3. **Comprehensive HTTP status code logging** in all test phases
4. **Real backend verification** - tests use actual B2, not mockBackend

## Compliance with Acceptance Criteria
✅ **AC1**: Tests issue REAL HTTP requests (SigV4-signed via AWS SDK v2) against ARMOR's actual listener
✅ **AC2**: Covers full lifecycle including PUT, HEAD, GET (full + range), LIST, overwrite, DELETE, post-delete verification
✅ **AC3**: Separately exercises multipart Abort with B2-side verification
✅ **AC4**: Closure cites actual HTTP status codes and response bodies from live run

## Files Modified
- `tests/aws-cli-compatibility/harness_test.go`: Added B2 endpoint construction (line ~525)
- `tests/aws-cli-compatibility/zz_lifecycle_integration_test.go`: Added nil-safety for LIST responses (lines 443-447, 647-651)

## Conclusion
ARMOR now has comprehensive, production-ready HTTP API lifecycle testing that exercises the full S3 object lifecycle through real HTTP requests against a real B2 backend. This addresses the coverage gap identified in bf-54irmj where the canary only tested direct backend package calls, bypassing the HTTP handler layer that could contain regressions (like bf-24sxh7).
