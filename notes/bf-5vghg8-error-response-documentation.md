# ARMOR Error Response Documentation

## Overview

ARMOR implements S3-compatible error responses following the AWS S3 error response format. All error responses are returned as XML with consistent structure and headers.

## Error Response Format

All ARMOR error responses follow this standard S3 XML format:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>ErrorCode</Code>
  <Message>Human-readable error message</Message>
</Error>
```

### HTTP Status Codes

Error responses use HTTP status codes that align with S3 conventions:
- **400 Bad Request**: Invalid request parameters or malformed input
- **403 Forbidden**: Authentication failures, authorization denials, ACL violations
- **404 Not Found**: Objects, buckets, or resources not found
- **405 Method Not Allowed**: Unsupported HTTP methods
- **500 Internal Server Error**: Server-side errors, encryption/decryption failures
- **503 Service Unavailable**: Service unavailability (e.g., canary check failed)

### Response Headers

All error responses include these consistent headers:
- **Content-Type**: `application/xml` (always present)
- **Content-Length**: Size of the XML error response

## Authentication and Authorization Errors

These errors occur during request authentication and authorization checks.

### Missing Authentication Token
- **Error Code**: `MissingAuthenticationToken`
- **HTTP Status**: 403
- **Message**: "Missing Authentication Token"
- **Trigger**: Authorization header is missing from the request
- **Example**:
  ```bash
  curl -X GET https://armor.example.com/bucket/key
  ```

### Invalid Algorithm
- **Error Code**: `InvalidAlgorithm`
- **HTTP Status**: 403
- **Message**: "Only AWS4-HMAC-SHA256 is supported"
- **Trigger**: Authorization header uses unsupported algorithm
- **Example**: Authorization header specifies AWS2-HMAC-SHA256

### Invalid Credential
- **Error Code**: `InvalidCredential`
- **HTTP Status**: 403
- **Message**: "Invalid credential format"
- **Trigger**: Malformed credential string in Authorization header

### Incomplete Signature
- **Error Code**: `IncompleteSignature`
- **HTTP Status**: 403
- **Message**: "Authorization header is missing required fields"
- **Trigger**: Authorization header is missing required fields (Credential, SignedHeaders, or Signature)

### Invalid Access Key ID
- **Error Code**: `InvalidAccessKeyId`
- **HTTP Status**: 403
- **Message**: "The AWS Access Key Id you provided does not exist"
- **Trigger**: Provided access key does not exist in credentials database

### Missing Date Header
- **Error Code**: `MissingDateHeader`
- **HTTP Status**: 403
- **Message**: "Missing X-Amz-Date header"
- **Trigger**: X-Amz-Date header is missing from the request

### Invalid Date Format
- **Error Code**: `InvalidDateFormat`
- **HTTP Status**: 403
- **Message**: "Invalid date format in X-Amz-Date header"
- **Trigger**: X-Amz-Date header is not in ISO 8601 format (YYYYMMDDTHHMMSSZ)

### Request Expired
- **Error Code**: `RequestExpired`
- **HTTP Status**: 403
- **Message**: "Request has expired"
- **Trigger**: Request timestamp is outside the 15-minute allowed window

### Signature Does Not Match
- **Error Code**: `SignatureDoesNotMatch`
- **HTTP Status**: 403
- **Message**: "The request signature we calculated does not match the signature you provided"
- **Trigger**: Calculated signature does not match provided signature (wrong secret key, tampered request)

### Access Denied
- **Error Code**: `AccessDenied`
- **HTTP Status**: 403
- **Message**: "Access Denied"
- **Trigger**: 
  - ACL-based authorization check failed
  - Credential does not have permission to access the requested bucket/key
  - Attempted access to `.armor/` reserved namespace

## Data Operation Errors

These errors occur during S3 data operations.

### Invalid Request
- **Error Code**: `InvalidRequest`
- **HTTP Status**: 400
- **Message**: Various context-specific messages
- **Trigger**:
  - Unsupported POST operation
  - Missing partNumber in multipart upload
  - Invalid partNumber (must be 1-10000)
  - No parts specified in CompleteMultipartUpload

### NoSuchKey
- **Error Code**: `NoSuchKey`
- **HTTP Status**: 404
- **Message**: "Object not found" or similar
- **Trigger**: Requested object does not exist in the bucket

### NoSuchBucket
- **Error Code**: `NoSuchBucket`
- **HTTP Status**: 404
- **Message**: "Bucket not found"
- **Trigger**: Requested bucket does not exist

### NoSuchUpload
- **Error Code**: `NoSuchUpload`
- **HTTP Status**: 404
- **Message**: "Multipart upload not found" or "Multipart upload does not match bucket/key"
- **Trigger**: 
  - Multipart upload ID does not exist
  - Upload ID exists but does not match the requested bucket/key

### Precondition Failed
- **Error Code**: `PreconditionFailed`
- **HTTP Status**: 412
- **Message**: "Precondition failed"
- **Trigger**: Conditional request header (If-Match, If-Unmodified-Since) failed

### Method Not Allowed
- **Error Code**: `MethodNotAllowed`
- **HTTP Status**: 405
- **Message**: "Method {METHOD} not allowed"
- **Trigger**: HTTP method is not supported for the requested resource

### Malformed XML
- **Error Code**: `MalformedXML`
- **HTTP Status**: 400
- **Message**: "Failed to parse XML: {error details}" or "No objects specified for deletion"
- **Trigger**:
  - Invalid XML in request body
  - DeleteObjects request with no objects specified

### Invalid Range
- **Error Code**: `InvalidRange`
- **HTTP Status**: 400
- **Message**: "Invalid range: {error details}"
- **Trigger**: Range header is malformed or out of bounds

## Internal Server Errors

These errors indicate server-side processing failures.

### Internal Error
- **Error Code**: `InternalError`
- **HTTP Status**: 500
- **Message**: Various context-specific messages
- **Triggers**:
  - Failed to read request body
  - Failed to get encryption/decryption key
  - Failed to generate DEK or IV
  - Failed to wrap/unwrap DEK
  - Failed to create/encode header
  - Failed to create encryptor/decryptor
  - Failed to encrypt/decrypt
  - Failed to upload to backend
  - Failed to create temp file (streaming uploads)
  - Failed to parse ARMOR metadata
  - Failed to compute HMACs
  - Failed to save/update multipart state
  - Failed to complete multipart upload
  - Failed to save HMAC table
  - Failed to update metadata
  - Failed to marshal XML response
  - Failed to get object lock/lifecycle configuration
  - Failed to put object lock/lifecycle configuration
  - Failed to delete lifecycle configuration
  - Encryption/decryption errors
  - Backend operation failures

## Performance Characteristics

Based on test results from comprehensive error verification:

- **Average response time**: 16.228µs (0.016ms)
- **Min response time**: 6.398µs (0.006ms)
- **Max response time**: 29.097µs (0.029ms)
- **All rejections under 100ms**: ✓ Yes

Performance test results for authentication failures:
```
Total scenarios: 8
Average response time: 16.228µs
Min response time: 6.398µs
Max response time: 29.097µs
All responses under 100ms: true
```

All error responses complete well within the 100ms performance threshold.

## Reserved Namespace Protection

ARMOR protects the `.armor/` namespace for internal use:

### Access Denied (.armor/ namespace)
- **Error Code**: `AccessDenied`
- **HTTP Status**: 403
- **Message**: "Access to .armor/ reserved namespace is denied"
- **Trigger**: Client attempts to access any object key with the `.armor/` prefix

This protection ensures that ARMOR's internal metadata and state files cannot be accessed directly by clients.

## Error Response Consistency

### Header Consistency
All error responses across all rejection types include:
- **Content-Type**: `application/xml` (100% consistent)
- **Content-Length**: Automatically set by the HTTP server
- **XML Declaration**: `<?xml version="1.0" encoding="UTF-8"?>`

### Format Consistency
All error responses follow the same XML structure:
```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>{ErrorCode}</Code>
  <Message>{ErrorMessage}</Message>
</Error>
```

Error codes and messages are XML-escaped to prevent injection attacks.

## Testing Coverage

The ARMOR test suite includes comprehensive error response verification covering:

1. **Authentication failures** (8 scenarios):
   - Missing authentication header
   - Invalid access key
   - Invalid signature (wrong secret key)
   - Malformed authorization header
   - Missing date header
   - Expired request
   - Empty signature
   - Invalid signature characters

2. **Performance verification**:
   - All response times under 100ms threshold
   - Average, min, and max response time tracking
   - Consistent Content-Type headers across all errors

3. **Format verification**:
   - XML structure validation
   - Error code matching
   - Error message quality (meaningful, non-empty, specifies rejection reason)

## Error Response Examples

### Example 1: Missing Authentication Token

**Request:**
```bash
curl -X GET https://armor.example.com/bucket/key
```

**Response:**
```http
HTTP/1.1 403 Forbidden
Content-Type: application/xml
Content-Length: 245

<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>MissingAuthenticationToken</Code>
  <Message>Missing Authentication Token</Message>
</Error>
```

### Example 2: Invalid Access Key

**Request:**
```bash
curl -X GET https://armor.example.com/bucket/key \
  -H "Authorization: AWS4-HMAC-SHA256 Credential=INVALIDKEY/20250714/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=..."
```

**Response:**
```http
HTTP/1.1 403 Forbidden
Content-Type: application/xml
Content-Length: 276

<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>InvalidAccessKeyId</Code>
  <Message>The AWS Access Key Id you provided does not exist</Message>
</Error>
```

### Example 3: Access Denied (ACL violation)

**Request:**
```bash
curl -X GET https://armor.example.com/restricted-bucket/sensitive-key \
  -H "Authorization: AWS4-HMAC-SHA256 Credential=RESTRICTEDKEY/20250714/us-east-1/s3/aws4_request, ..."
```

**Response:**
```http
HTTP/1.1 403 Forbidden
Content-Type: application/xml
Content-Length: 229

<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>AccessDenied</Code>
  <Message>Access Denied</Message>
</Error>
```

### Example 4: Object Not Found

**Request:**
```bash
curl -X GET https://armor.example.com/bucket/nonexistent-key \
  -H "Authorization: AWS4-HMAC-SHA256 Credential=VALIDKEY/20250714/us-east-1/s3/aws4_request, ..."
```

**Response:**
```http
HTTP/1.1 404 Not Found
Content-Type: application/xml
Content-Length: 245

<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>NoSuchKey</Code>
  <Message>Object not found</Message>
</Error>
```

### Example 5: Reserved Namespace Access

**Request:**
```bash
curl -X GET https://armor.example.com/bucket/.armor/internal-state \
  -H "Authorization: AWS4-HMAC-SHA256 Credential=VALIDKEY/20250714/us-east-1/s3/aws4_request, ..."
```

**Response:**
```http
HTTP/1.1 403 Forbidden
Content-Type: application/xml
Content-Length: 301

<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>AccessDenied</Code>
  <Message>Access to .armor/ reserved namespace is denied</Message>
</Error>
```

## Summary

ARMOR provides comprehensive, S3-compatible error responses with:

- ✓ All error responses include meaningful error messages
- ✓ Error messages specify the rejection reason
- ✓ Response time for all rejections under 100ms (typically < 30µs)
- ✓ Response headers are consistent across rejection types (Content-Type: application/xml)
- ✓ Well-documented error response format
- ✓ Comprehensive test coverage for authentication failures
- ✓ XML-escaped error codes and messages for security
