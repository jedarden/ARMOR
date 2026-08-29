# S3 API Test Coverage Analysis

This document maps ARMOR's documented S3 API operations against existing test coverage, identifying gaps for comprehensive testing.

## Transforming Operations (with encryption/decryption)

| Operation | Status | Test Location | Coverage Notes | Gaps |
|-----------|--------|---------------|----------------|------|
| PutObject | ✅ Partial | `integration_test.go:TestPutGetRoundtrip`, `awscli_test.go` | Basic upload works, streaming tested | Edge cases: zero-byte files, large files >5GB, metadata preservation |
| GetObject | ✅ Partial | `integration_test.go:TestPutGetRoundtrip`, `TestRangeRead` | Basic download and range reads work | Edge cases: multipart object range reads across part boundaries |
| HeadObject | ✅ Partial | `integration_test.go:TestHeadObject` | Basic metadata and size | Conditional requests, metadata edge cases |
| CopyObject | ✅ Partial | `integration_test.go:TestCopyObject` | Basic copy works | Cross-bucket copy, metadata replacement, replace-tagging |
| CreateMultipartUpload | ✅ Partial | `integration_test.go:TestMultipartUpload` | Basic create works | Metadata on create, custom content-type |
| UploadPart | ✅ Partial | `integration_test.go:TestMultipartUpload` | Basic upload works | Part size edge cases, part ordering, concurrent part uploads |
| CompleteMultipartUpload | ✅ Partial | `integration_test.go:TestMultipartUpload` | Basic complete works | Part verification, missing parts, out-of-order parts |
| AbortMultipartUpload | ✅ Partial | `awscli-compat:TestLifecycle_MultipartAbort` | Basic abort works | Abort after completion, abort non-existent upload |
| **ListParts** | ❌ Missing | - | Not tested | Part listing, part numbering, max-keys, part-number-marker |
| **ListMultipartUploads** | ❌ Missing | - | Not tested | Upload listing, prefix filtering, max-keys, delimiter |

## Passthrough Operations

| Operation | Status | Test Location | Coverage Notes | Gaps |
|-----------|--------|---------------|----------------|------|
| ListObjectsV2 | ✅ Partial | `integration_test.go:TestListObjectsV2` | Basic listing, size correction | Continuation tokens, encoding, delimiter, prefix filtering edge cases |
| DeleteObject | ✅ Partial | `integration_test.go:TestDeleteObject` | Basic delete works | Delete non-existent, versioning |
| **DeleteObjects** | ❌ Missing | - | Not tested | Multi-object delete, quiet vs verbose, partial failures |
| **ListBuckets** | ❌ Missing | - | Not tested | Bucket listing response structure |
| **CreateBucket** | ❌ Missing | - | Not tested | Bucket creation, duplicate bucket, location constraint |
| **DeleteBucket** | ❌ Missing | - | Not tested | Bucket deletion, non-empty bucket, missing bucket |
| **HeadBucket** | ❌ Missing | - | Not tested | Bucket existence check, missing bucket |
| **GetBucketLifecycleConfiguration** | ❌ Missing | - | Not tested | Lifecycle rule parsing, empty configuration |
| **PutBucketLifecycleConfiguration** | ❌ Missing | - | Not tested | Lifecycle rule validation, multiple rules |
| **DeleteBucketLifecycleConfiguration** | ❌ Missing | - | Not tested | Delete lifecycle rules |
| **GetObjectLockConfiguration** | ❌ Missing | - | Not tested | Object lock enablement, retention modes |
| **PutObjectLockConfiguration** | ❌ Missing | - | Not tested | Object lock configuration validation |
| **GetObjectRetention** | ❌ Missing | - | Not tested | Retention period parsing, bypass governance |
| **PutObjectRetention** | ❌ Missing | - | Not tested | Retention mode validation, period limits |
| **GetObjectLegalHold** | ❌ Missing | - | Not tested | Legal hold status |
| **PutObjectLegalHold** | ❌ Missing | - | Not tested | Legal hold ON/OFF validation |

## Test Gap Summary

**High Priority Gaps (Core S3 operations without tests):**
1. ListParts - Critical for multipart upload management
2. ListMultipartUploads - Critical for multipart upload discovery
3. DeleteObjects - Critical for bulk delete operations
4. CreateBucket/DeleteBucket/HeadBucket - Basic bucket operations
5. ListBuckets - Multi-bucket deployments

**Medium Priority Gaps (Configuration and management):**
6. Get/Put/DeleteBucketLifecycleConfiguration - Lifecycle management
7. Get/PutObjectLockConfiguration - Object lock configuration
8. Get/PutObjectRetention - Object-level retention settings
9. Get/PutObjectLegalHold - Legal hold compliance features

## Test Strategy

Each missing operation needs comprehensive integration tests covering:

1. **Happy path** - Basic operation success
2. **Edge cases** - Empty results, boundary conditions
3. **Error cases** - Invalid inputs, non-existent resources
4. **S3 compliance** - Response structure, headers, error codes
5. **ARMOR specifics** - Encryption transparency, metadata handling

Tests should follow the existing integration test pattern using real ARMOR server against B2 backend (requires `ARMOR_INTEGRATION_TEST=1`).
