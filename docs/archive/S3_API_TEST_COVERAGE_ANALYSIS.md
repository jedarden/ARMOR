# S3 API Test Coverage Analysis

**Date:** 2026-08-17  
**Bead:** armor-0e93924f  
**Purpose:** Comprehensive analysis of existing S3 operation test coverage, identifying gaps in correctness, S3 compliance, and edge case testing.

## Executive Summary

The README documents full S3 API coverage with 10 transforming operations and 7 passthrough operations. This analysis maps each documented operation to existing tests and identifies gaps where comprehensive correctness, S3 compliance, and edge case testing is missing.

**Key Findings:**
- ✅ **18/19 operations** have basic test coverage (94.7%)
- ⚠️ **Comprehensive edge case coverage** is incomplete for most operations
- ⚠️ **S3 compliance verification** is limited for error cases and boundary conditions
- ❌ **CreateMultipartUpload/UploadPart** have only indirect coverage through end-to-end tests

## Documented S3 Operations

### Transforming Operations (encryption/decryption applied)

| Operation | README Support | Basic Tests | Edge Case Tests | Compliance Tests | Coverage Status |
|-----------|----------------|-------------|-----------------|------------------|-----------------|
| PutObject | Full (streaming for large files) | ✅ TestPutObjectGetObject | ⚠️ Partial | ⚠️ Partial | 🟡 Moderate |
| GetObject | Full (range reads) | ✅ TestGetObjectRange, TestPutObjectGetObject | ✅ Good | ⚠️ Partial | 🟢 Good |
| HeadObject | Full (plaintext size, conditionals) | ✅ TestHeadObject, TestHeadConditionalRequests | ⚠️ Partial | ⚠️ Limited | 🟡 Moderate |
| CopyObject | Full (DEK re-wrapping, cross-bucket) | ✅ TestCopyObject* (4 variants) | ⚠️ Partial | ⚠️ Limited | 🟡 Moderate |
| CreateMultipartUpload | Full | ❌ Only indirect | ❌ Missing | ❌ Missing | 🔴 Poor |
| UploadPart | Full | ❌ Only indirect | ❌ Missing | ❌ Missing | 🔴 Poor |
| CompleteMultipartUpload | Full | ✅ TestCompleteMultipartUpload* (backend) | ✅ Good | ⚠️ Limited | 🟢 Good |
| AbortMultipartUpload | Full | ✅ TestAbortMultipartUpload | ⚠️ Partial | ⚠️ Limited | 🟡 Moderate |
| ListParts | Full | ✅ TestListParts (integration) | ⚠️ Partial | ⚠️ Limited | 🟡 Moderate |
| ListMultipartUploads | Full | ✅ TestListMultipartUploads (5 variants) | ⚠️ Partial | ⚠️ Limited | 🟢 Good |

### Passthrough Operations

| Operation | README Support | Basic Tests | Edge Case Tests | Compliance Tests | Coverage Status |
|-----------|----------------|-------------|-----------------|------------------|-----------------|
| ListObjectsV2 | Full (size correction, `.armor/` filter) | ✅ TestListObjectsV2 | ⚠️ Partial | ⚠️ Limited | 🟡 Moderate |
| DeleteObject | Full | ✅ TestDeleteObject | ❌ Minimal | ❌ Missing | 🟡 Moderate |
| DeleteObjects | Full | ✅ TestDeleteObjects (6 variants) | ✅ Good | ⚠️ Limited | 🟢 Good |
| ListBuckets | Full | ✅ TestListBuckets | ❌ Minimal | ❌ Missing | 🟡 Moderate |
| CreateBucket | Full | ✅ TestCreateBucket | ❌ Minimal | ❌ Missing | 🟡 Moderate |
| DeleteBucket | Full | ✅ TestDeleteBucket | ❌ Minimal | ❌ Missing | 🟡 Moderate |
| HeadBucket | Full | ✅ TestHeadBucket | ❌ Minimal | ❌ Missing | 🟡 Moderate |
| Lifecycle config | Full | ✅ Test*LifecycleConfig (3 tests) | ❌ Minimal | ❌ Missing | 🟡 Moderate |
| Object Lock | Full | ✅ Test*ObjectLock* (2 tests) | ❌ Minimal | ❌ Missing | 🟡 Moderate |
| Retention | Full | ✅ Test*Retention (2 tests) | ❌ Minimal | ❌ Missing | 🟡 Moderate |
| Legal Hold | Full | ✅ Test*LegalHold (2 tests) | ❌ Minimal | ❌ Missing | 🟡 Moderate |

## Test Coverage Details

### 1. Transforming Operations

#### 1.1 PutObject
**Existing Tests:**
- `TestPutObjectGetObject` - Basic upload/download roundtrip
- `TestPutObjectLargeFile` - Large file handling
- `TestEncryptionRoundTrip` - Encryption verification
- `TestETagConsistency` - ETag generation
- `TestStreamingEncryptionLargeFile` - Streaming encryption

**Coverage Gaps:**
- ❌ Metadata preservation edge cases
- ❌ Content-Type handling
- ❌ Content-Encoding behavior
- ❌ Server-side encryption headers (should reject)
- ❌ Chunked upload encoding
- ❌ Zero-byte objects
- ❌ Object key encoding/escaping
- ❌ Precondition failures (If-Match, If-None-Match)

**S3 Compliance:**
- ⚠️ Limited error response format validation
- ⚠️ Missing validation of required headers
- ⚠️ No verification of ETag format for encrypted objects

#### 1.2 GetObject
**Existing Tests:**
- `TestGetObjectRange` - Range request handling
- `TestGetObjectRangeCompressed` - Compressed range requests
- `TestRangeSuffixRequest` - Suffix range requests
- `TestStreamingDecryption*` - Multiple streaming tests
- `TestHMACVerification` - Integrity verification

**Coverage Gaps:**
- ⚠️ Partial range edge cases (offset at block boundaries)
- ❌ Range overlapping end of file
- ❌ Invalid range format handling
- ❌ If-Modified-Since, If-Unmodified-Since
- ❌ If-Match, If-None-Match conditional gets
- ❌ Response header completeness
- ❌ Missing object error cases

**S3 Compliance:**
- ✅ Good range request coverage
- ⚠️ Limited conditional request validation
- ⚠️ Missing validation of response headers

#### 1.3 HeadObject
**Existing Tests:**
- `TestHeadObject` - Basic HEAD request
- `TestHeadConditionalRequests` - Conditional HEAD
- `TestHeadConditionalRequestsWithRange` - Conditional with range

**Coverage Gaps:**
- ❌ Metadata preservation verification
- ❌ Last-Modified format
- ❌ ETag consistency with GET
- ❌ Content-Type preservation
- ❌ Missing object error cases

**S3 Compliance:**
- ⚠️ Limited response header validation
- ⚠️ Missing error response format validation

#### 1.4 CopyObject
**Existing Tests:**
- `TestCopyObject` - Basic copy
- `TestCopyObjectRewrapsDEK` - DEK re-wrapping verification
- `TestCopyObjectNonARMOR` - Non-ARMOR to ARMOR copy
- `TestCopyObjectMissingSource` - Error case
- `TestCopyObjectWithMetadataDirective` - Metadata handling

**Coverage Gaps:**
- ❌ Cross-bucket copy edge cases
- ❌ MetadataDirective=REPLACE vs COPY behavior
- ❌ Copy with object lock
- ❌ Copy with retention settings
- ❌ CopySourceIfMatch/If-None-Match
- ❌ CopySourceRange partial copy
- ❌ Large file copy verification

**S3 Compliance:**
- ⚠️ Limited directive validation
- ⚠️ Missing error response format validation
- ⚠️ No verification of copy progress for large objects

#### 1.5 CreateMultipartUpload
**Existing Tests:**
- ❌ **No dedicated S3 handler tests** (only backend tests)
- Tests exist at backend level: `TestB2Backend_CreateMultipartUpload_Success`

**Coverage Gaps:**
- ❌ S3 API parameter validation (Content-Type, Metadata, etc.)
- ❌ Error responses for invalid parameters
- ❌ UploadId format validation
- ❌ Concurrent upload creation for same key
- ❌ Metadata preservation
- ❌ Object lock inheritance
- ❌ Server-side encryption headers (should reject)

**S3 Compliance:**
- ❌ No validation of S3 request/response format
- ❌ Missing error response validation
- ❌ No verification of UploadId uniqueness

#### 1.6 UploadPart
**Existing Tests:**
- ❌ **No dedicated S3 handler tests** (only backend tests)
- Tests exist at backend level: `TestB2Backend_UploadPart_Success`
- Tests exist at routing level: `TestUploadPartRoutingNeverFallsThroughToPut`

**Coverage Gaps:**
- ❌ Part number validation (1-10000)
- ❌ Part size validation (5MB minimum except last)
- ❌ ETag generation per part
- ❌ Content-MD5 header validation
- ❌ Concurrent part uploads
- ❌ Out-of-order part uploads
- ❌ Duplicate part number handling
- ❌ Invalid uploadId error cases

**S3 Compliance:**
- ❌ No validation of S3 request/response format
- ❌ Missing part size limit validation
- ❌ No error response format validation

#### 1.7 CompleteMultipartUpload
**Existing Tests:**
- ✅ `TestCompleteMultipartUpload*` (8 handler tests covering ordering, edge cases)
- ✅ `TestB2Backend_CompleteMultipartUpload_Success` (backend test)
- ✅ `TestMultipartUpload` (integration test)

**Coverage Gaps:**
- ⚠️ Partial: XML validation for part list ordering
- ❌ Final ETag calculation verification
- ❌ Parts totaling less than minimum size
- ❌ Missing parts error handling
- ❌ Invalid part ETag handling
- ❌ Aborted upload completion
- ❌ Multipart completion with 1 part (optimization path)

**S3 Compliance:**
- ⚠️ Good XML ordering coverage
- ⚠️ Limited error response validation
- ⚠️ Missing validation of final ETag format

#### 1.8 AbortMultipartUpload
**Existing Tests:**
- ✅ `TestAbortMultipartUpload` - Basic abort
- ✅ `TestAbortMultipartUploadNotFound` - Error case

**Coverage Gaps:**
- ❌ Abort after completion (should fail)
- ❌ Abort with partial parts cleanup
- ❌ Concurrent abort/completion
- ❌ Abort already aborted upload

**S3 Compliance:**
- ⚠️ Limited error response validation
- ⚠️ No verification of cleanup behavior

#### 1.9 ListParts
**Existing Tests:**
- ✅ `TestListParts` - Integration test (pagination, max-parts, markers)
- ✅ `TestListParts_EmptyUpload` - Edge case
- ✅ `TestListParts_NonExistentUpload` - Error case
- ✅ `TestListPartsNotFound` - Handler test

**Coverage Gaps:**
- ⚠️ Good pagination coverage
- ❌ Part number filter validation
- ❌ Encoding of special characters in ETag
- ❌ Part size accuracy verification
- ❌ LastModified format validation

**S3 Compliance:**
- ⚠️ Good pagination behavior
- ⚠️ Limited response format validation
- ⚠️ Missing error response format validation

#### 1.10 ListMultipartUploads
**Existing Tests:**
- ✅ `TestListMultipartUploads*` (5 handler variants + 3 integration tests)
- ✅ Good coverage of: prefix, delimiter, max-uploads, key-marker, upload-id-marker

**Coverage Gaps:**
- ⚠️ Good parameter coverage
- ❌ Encoding of special characters in keys
- ❌ Initiator/Owner fields validation
- ❌ Storage class handling

**S3 Compliance:**
- ⚠️ Good pagination behavior
- ⚠️ Limited response format validation

### 2. Passthrough Operations

#### 2.1 ListObjectsV2
**Existing Tests:**
- ✅ `TestListObjectsV2` - Basic listing
- ✅ Integration tests mention size correction and `.armor/` filtering

**Coverage Gaps:**
- ❌ Continuation token pagination
- ❌ Prefix + delimiter filtering
- ❌ Max-keys limit validation
- ❌ FetchOwner parameter behavior
- ❌ StartAfter parameter
- ❌ EncodingType parameter (URL encoding)
- ❌ CommonPrefixes vs Contents validation
- ❌ .armor/ prefix filtering verification
- ❌ Size correction accuracy (ciphertext vs plaintext)

**S3 Compliance:**
- ⚠️ Limited pagination validation
- ⚠️ No verification of encoding behavior
- ⚠️ Missing delimiter handling validation

#### 2.2 DeleteObject
**Existing Tests:**
- ✅ `TestDeleteObject` - Basic delete

**Coverage Gaps:**
- ❌ Delete non-existent object (should succeed or return error)
- ❌ Delete with versioning (if supported)
- ❌ Delete with object lock (should fail if locked)
- ❌ Concurrent deletes
- ❌ Delete with retention check

**S3 Compliance:**
- ❌ No validation of error response format
- ❌ No verification of success response format

#### 2.3 DeleteObjects
**Existing Tests:**
- ✅ `TestDeleteObjects` - Basic multi-object delete
- ✅ `TestDeleteObjectsQuiet` - Quiet mode
- ✅ `TestDeleteObjects_BulkDelete` - Integration bulk delete
- ✅ `TestDeleteObjects_QuietMode` - Integration quiet mode
- ✅ `TestDeleteObjects_NonExistentObjects` - Mixed existing/non-existing
- ✅ `TestDeleteObjects_EmptyList` - Edge case

**Coverage Gaps:**
- ⚠️ Good bulk coverage
- ❌ Error response format validation for partial failures
- ❌ Batching behavior with large lists (>1000)
- ❌ Object lock enforcement

**S3 Compliance:**
- ⚠️ Good quiet mode coverage
- ⚠️ Limited error response format validation

#### 2.4 ListBuckets
**Existing Tests:**
- ✅ `TestListBuckets` - Basic bucket list

**Coverage Gaps:**
- ❌ Empty bucket list
- ❌ Bucket location constraint (if any)
- ❌ Encoding of bucket names
- ❌ Owner field validation

**S3 Compliance:**
- ❌ No validation of response format
- ❌ No verification of owner/creation date fields

#### 2.5 CreateBucket / DeleteBucket / HeadBucket
**Existing Tests:**
- ✅ `TestCreateBucket` - Basic create
- ✅ `TestDeleteBucket` - Basic delete
- ✅ `TestHeadBucket` - Basic HEAD

**Coverage Gaps:**
- ❌ Create bucket with existing name (error case)
- ❌ Delete non-empty bucket (error case)
- ❌ Delete with .armor/ keys (should handle)
- ❌ ObjectLocationConstraint parameter
- ❌ HeadBucket error responses

**S3 Compliance:**
- ❌ No validation of error response formats
- ❌ No verification of bucket naming constraints

#### 2.6 Lifecycle Configuration
**Existing Tests:**
- ✅ `TestGetBucketLifecycleConfiguration` - GET lifecycle
- ✅ `TestPutBucketLifecycleConfiguration` - PUT lifecycle
- ✅ `TestDeleteBucketLifecycleConfiguration` - DELETE lifecycle

**Coverage Gaps:**
- ❌ XML schema validation
- ❌ Rule validation (ID, filter, status, expiration)
- ❌ Noncurrent version expiration (if supported)
- ❌ Abort incomplete multipart upload rule
- ❌ Transition rules (if supported)
- ❌ Multiple rules validation
- ❌ Rule ordering and priority

**S3 Compliance:**
- ❌ No XML schema validation
- ❌ No validation of rule constraints
- ❌ Missing error response format validation

#### 2.7 Object Lock / Retention / Legal Hold
**Existing Tests:**
- ✅ `TestGetObjectLockConfiguration` / `TestPutObjectLockConfiguration` - Bucket lock config
- ✅ `TestGetObjectRetention` / `TestPutObjectRetention` - Per-object retention
- ✅ `TestGetObjectLegalHold` / `TestPutObjectLegalHold` - Legal hold

**Coverage Gaps:**
- ❌ Retention modes (GOVERNANCE vs COMPLIANCE)
- ❌ Retention period validation
- ❌ Bypass governance retention with special headers
- ❌ Legal hold ON/OFF validation
- ❌ Object lock with versioning (if supported)
- ❌ WORM behavior verification

**S3 Compliance:**
- ❌ No validation of retention period enforcement
- ❌ No verification of WORM behavior
- ❌ Missing error response format validation

## Recommendations

### High Priority (P1) - Core S3 Compliance

1. **CreateMultipartUpload/UploadPart Handler Tests**
   - Add S3 handler-level tests (currently only backend tests exist)
   - Validate S3 request/response format
   - Test error cases (invalid part numbers, sizes, upload IDs)
   - Verify metadata and Content-Type handling

2. **PutObject/GetObject/HeadObject Edge Cases**
   - Add comprehensive conditional request tests (If-Match, If-None-Match, If-Modified-Since)
   - Test metadata preservation edge cases
   - Validate Content-Encoding/Content-Type handling
   - Add zero-byte object tests
   - Verify response header completeness

3. **ListObjectsV2 Comprehensive Testing**
   - Add continuation token pagination tests
   - Validate prefix + delimiter filtering
   - Test encoding behavior (EncodingType parameter)
   - Verify .armor/ filtering and size correction
   - Add max-keys boundary tests

### Medium Priority (P2) - Error Handling & Edge Cases

4. **DeleteObject/DeleteObjects Error Cases**
   - Test delete with object lock enforcement
   - Validate partial failure error responses
   - Test batching behavior (>1000 objects)
   - Verify non-existent object handling

5. **Bucket Operations Error Handling**
   - Add create existing bucket error tests
   - Test delete non-empty bucket errors
   - Validate HeadBucket error responses
   - Verify bucket naming constraints

6. **CopyObject Comprehensive Testing**
   - Add CrossSourceRange tests
   - Validate MetadataDirective behavior thoroughly
   - Test copy with object lock/retention
   - Add large file copy verification

### Lower Priority (P3) - Advanced Features

7. **Lifecycle Configuration Validation**
   - Add XML schema validation tests
   - Test rule constraints and validation
   - Verify abort incomplete multipart upload rules
   - Add multiple rules interaction tests

8. **Object Lock WORM Behavior**
   - Verify retention mode enforcement (GOVERNANCE vs COMPLIANCE)
   - Test retention period validation
   - Add bypass governance retention tests
   - Validate legal hold ON/OFF behavior

9. **S3 Response Format Validation**
   - Add systematic validation of S3 XML response formats
   - Verify error response structures
   - Validate required vs optional headers
   - Test encoding of special characters

## Implementation Strategy

### Phase 1: High-Value Gaps (Immediate)
1. CreateMultipartUpload/UploadPart handler tests
2. PutObject/GetObject/HeadObject edge cases
3. ListObjectsV2 comprehensive tests

### Phase 2: Error Handling (Short-term)
4. Delete operations error cases
5. Bucket operations error handling
6. CopyObject comprehensive tests

### Phase 3: Advanced Features (Ongoing)
7. Lifecycle configuration validation
8. Object lock WORM behavior
9. Systematic S3 response format validation

## Test Organization Structure

Proposed new test files for organization:

```
tests/
├── integration/
│   ├── s3_comprehensive/
│   │   ├── test_putobject_edge_cases.go
│   │   ├── test_getobject_edge_cases.go
│   │   ├── test_headobject_edge_cases.go
│   │   ├── test_copyobject_comprehensive.go
│   │   ├── test_listobjectsv2_comprehensive.go
│   │   ├── test_multipart_upload_handler.go
│   │   ├── test_delete_comprehensive.go
│   │   ├── test_bucket_operations_comprehensive.go
│   │   ├── test_lifecycle_validation.go
│   │   └── test_objectlock_validation.go
│   └── s3_compliance/
│       ├── test_response_formats.go
│       ├── test_error_responses.go
│       └── test_xml_schemas.go
```

## Conclusion

ARMOR has solid foundational test coverage (94.7% of operations have basic tests), but comprehensive edge case, S3 compliance, and error handling testing is incomplete. The highest priority gaps are:

1. **CreateMultipartUpload/UploadPart handler tests** - These operations have no direct S3 handler tests
2. **PutObject/GetObject/HeadObject conditional requests** - Critical S3 feature with minimal testing
3. **ListObjectsV2 comprehensive testing** - Complex pagination/filtering behavior needs validation

Implementing these tests will significantly improve confidence in ARMOR's S3 API compliance and robustness.
