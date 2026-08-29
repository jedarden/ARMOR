# S3 API Test Coverage Analysis

**Generated:** 2026-08-17
**Purpose:** Identify gaps in S3 API test coverage for ARMOR

## Transforming Operations (encryption/decryption applied)

| Operation | Coverage Status | Notes |
|-----------|----------------|-------|
| **PutObject** | ✅ Partial | Basic round-trip tested (integration_test.go). Missing: large files, edge cases, metadata, tags, encryption headers |
| **GetObject** | ✅ Partial | Basic download + range reads tested. Missing: conditional reads, overridden response headers, part_number |
| **HeadObject** | ✅ Partial | Basic existence check tested. Missing: conditional heads, metadata validation, ETag handling |
| **CopyObject** | ❌ MISSING | No tests found. Needs: same-bucket, cross-bucket, metadata replacement, DEK re-wrapping verification |
| **CreateMultipartUpload** | ✅ Partial | Used in multipart tests but no dedicated test. Missing: metadata, tags, server-side encryption params |
| **UploadPart** | ✅ Partial | Used in multipart tests. Missing: part size edge cases, checksums |
| **CompleteMultipartUpload** | ✅ Partial | Used in multipart tests. Missing: verification of combined parts, ETag calculation |
| **AbortMultipartUpload** | ✅ GOOD | Dedicated test file (s3_multipart_abort_test.go) |
| **ListParts** | ✅ GOOD | Comprehensive tests in s3_operations_test.go |
| **ListMultipartUploads** | ✅ GOOD | Comprehensive tests in s3_operations_test.go |

## Passthrough Operations

| Operation | Coverage Status | Notes |
|-----------|----------------|-------|
| **ListObjectsV2** | ⚠️ LIMITED | Used in cleanup but no dedicated test. Missing: prefix, delimiter, continuation token, max-keys, encoding-type, fetch-owner |
| **DeleteObject** | ✅ GOOD | Used in cleanup throughout. Basic delete works |
| **DeleteObjects** | ✅ GOOD | Comprehensive tests in s3_operations_test.go (bulk, quiet mode, mixed) |
| **ListBuckets** | ✅ GOOD | Tested in s3_bucket_operations_test.go |
| **CreateBucket** | ✅ GOOD | Tested with naming constraints in s3_bucket_operations_test.go |
| **DeleteBucket** | ✅ GOOD | Tested with empty/non-empty scenarios |
| **HeadBucket** | ✅ GOOD | Tested with existent/non-existent buckets |
| **Lifecycle configuration** | ⚠️ LIMITED | Basic put/get/delete test exists. Missing: multiple rules, filter types, noncurrent version expiration, abort multipart rules |
| **Object Lock / Retention / Legal Hold** | ✅ GOOD | Comprehensive tests in s3_object_lock_test.go |

## Missing Comprehensive Test Coverage

### High Priority Gaps

1. **CopyObject** - Completely untested
   - Same-bucket copy
   - Cross-bucket copy (if applicable to ARMOR)
   - Metadata replacement (REPLACE vs COPY)
   - Tag replacement
   - ACL directives
   - Directives for metadata and tagging

2. **ListObjectsV2** - Needs dedicated test
   - Pagination (continuation token)
   - Prefix filtering
   - Delimiter and common prefixes
   - Max-keys limit
   - Encoding-type handling
   - Fetch-owner parameter
   - Start-after parameter

3. **PutObject Edge Cases** - Need coverage
   - Large files (> 5MB, > 100MB)
   - Small files (< 1KB)
   - Metadata preservation
   - Custom tags
   - Content-type and content-encoding
   - Server-side encryption headers (should be ignored/passthrough)
   - Storage class (if applicable)

4. **GetObject Edge Cases** - Need coverage
   - Conditional downloads (If-Match, If-None-Match, If-Modified-Since, If-Unmodified-Since)
   - Response header overrides (Cache-Control, Content-Disposition, etc.)
   - Part-number for MPU parts
   - TLS/SSL verification

5. **HeadObject Edge Cases** - Need coverage
   - Conditional heads (same conditionals as GetObject)
   - Metadata validation after put
   - ETag format verification
   - Storage class returned correctly

### Medium Priority Gaps

6. **CreateMultipartUpload** - Need dedicated tests
   - With custom metadata
   - With tags
   - With server-side encryption headers (if supported)
   - ContentType override

7. **CompleteMultipartUpload** - Need verification
   - ETag calculation matches S3 spec
   - Part order independence
   - Combine all parts correctly
   - Final object is decryptable

8. **ListObjectsV2 Metadata** - Comprehensive test
   - Size correction (ARMOR encrypts, so plaintext size vs ciphertext size)
   - `.armor/` prefix filtering (reserved namespace)
   - Owner display name and ID

9. **Lifecycle Configuration** - Extended coverage
   - Multiple rules in one configuration
   - Prefix filters
   - Tag filters
   - And filters (combination)
   - Noncurrent version expiration
   - Abort incomplete multipart upload rules

### Lower Priority Gaps

10. **DeleteObject Edge Cases**
    - Version ID handling (if ARMOR supports versioning)
    - Request Payer headers

11. **Object Tagging**
    - PutObjectTagging
    - GetObjectTagging
    - DeleteObjectTagging

12. **Object ACLs** (if supported)
    - PutObjectAcl
    - GetObjectAcl

13. **Bucket Operations**
    - GetBucketLocation
    - GetBucketPolicy
    - PutBucketPolicy
    - DeleteBucketPolicy
    - GetBucketVersioning
    - PutBucketVersioning

## Test File Organization

Current test files:
- `integration_test.go` - Basic round-trip, range reads
- `s3_operations_test.go` - ListParts, ListMultipartUploads, DeleteObjects
- `s3_bucket_operations_test.go` - Bucket CRUD, lifecycle, versioning
- `s3_object_lock_test.go` - Object lock, retention, legal hold
- `s3_multipart_abort_test.go` - Multipart abort scenarios

Recommended new test files:
- `s3_copyobject_test.go` - All CopyObject scenarios
- `s3_listobjectsv2_test.go` - ListObjectsV2 comprehensive tests
- `s3_putobject_edge_cases_test.go` - PutObject edge cases and metadata
- `s3_getobject_edge_cases_test.go` - GetObject conditional reads, overrides
- `s3_headobject_edge_cases_test.go` - HeadObject metadata and conditionals
- `s3_multipart_lifecycle_test.go` - CreateMultipartUpload, CompleteMultipartUpload edge cases
- `s3_lifecycle_advanced_test.go` - Advanced lifecycle configuration scenarios

## Test Beads Needed

Based on the README documentation and current coverage, the following test beads should be created:

1. **Comprehensive CopyObject Test Suite**
2. **ListObjectsV2 Comprehensive Test Suite**
3. **PutObject Edge Cases and Metadata**
4. **GetObject Conditional Reads and Response Overrides**
5. **HeadObject Metadata and Conditionals**
6. **Multipart Upload Lifecycle Edge Cases**
7. **Advanced Lifecycle Configuration**

Each bead should focus on one operation family and test:
- Happy path basic operation
- Edge cases and error conditions
- S3 compliance and specification adherence
- ARMOR-specific behavior (encryption transparency, .armor/ filtering, size correction)
