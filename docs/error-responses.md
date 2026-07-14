# ARMOR Error Response Documentation

## Overview

ARMOR provides S3-compatible error responses for all authentication, authorization, and request processing failures. This document describes the error response format, all error codes, rejection scenarios, performance characteristics, and examples.

## Quick Reference: All Error Codes

| Error Code | HTTP Status | Category | Description |
|------------|-------------|----------|-------------|
| `MissingAuthenticationToken` | 403 | Authentication | Authorization header or X-Amz-Credential query parameter is missing |
| `InvalidAccessKeyId` | 403 | Authentication | The provided access key does not exist in credentials store |
| `SignatureDoesNotMatch` | 403 | Authentication | Calculated signature does not match the provided signature |
| `InvalidAlgorithm` | 403 | Authentication | Only AWS4-HMAC-SHA256 is supported |
| `InvalidCredential` | 403 | Authentication | Credential format is invalid (insufficient parts) |
| `IncompleteSignature` | 403 | Authentication | Authorization header is missing required fields |
| `RequestExpired` | 403 | Authentication | Request timestamp is outside allowed window (±15 minutes) |
| `MissingDateHeader` | 403 | Authentication | X-Amz-Date header is missing |
| `InvalidDateFormat` | 403 | Authentication | X-Amz-Date header format is invalid (not ISO 8601) |
| `AccessDenied` | 403 | Authorization | ACL restrictions prevent access to requested bucket/key |
| `InvalidRequest` | 400 | Request | Invalid request parameters or unsupported operation |
| `NoSuchKey` | 404 | Request | Requested object does not exist in the bucket |
| `MethodNotAllowed` | 405 | Request | HTTP method is not supported for the requested endpoint |
| `PreconditionFailed` | 412 | Request | Conditional request precondition failed |
| `InternalError` | 500 | Server | Server encountered an error during request processing |

## Error Response Format

## Error Response Format

All error responses follow the S3 XML error format with consistent headers:

### HTTP Headers

```http
Content-Type: application/xml
Status: 403 Forbidden (for authentication errors)
```

### Response Body

```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>ErrorCode</Code>
  <Message>Human-readable error description</Message>
</Error>
```

## Authentication Error Codes

### Invalid Credentials

| Error Code | HTTP Status | Message | When Returned |
|------------|-------------|---------|---------------|
| `InvalidAccessKeyId` | 403 | The AWS Access Key Id you provided does not exist in our records. | Access key not found in credentials store |
| `SignatureDoesNotMatch` | 403 | The request signature we calculated does not match the signature you provided. | Secret key is incorrect or signature is invalid |
| `MissingAuthenticationToken` | 403 | Missing authentication token. | Authorization header is completely missing |
| `InvalidCredential` | 403 | Invalid credential format. | Credential string has insufficient parts (expected 5 parts) |

### Malformed Authorization Headers

| Error Code | HTTP Status | Message | When Returned |
|------------|-------------|---------|---------------|
| `IncompleteSignature` | 403 | The authorization header is malformed. | Required components missing (Credential, SignedHeaders, or Signature) |
| `InvalidAlgorithm` | 403 | The authorization header algorithm is not supported. | Algorithm is not AWS4-HMAC-SHA256 |
| `AccessDenied` | 403 | Access Denied. | Generic authentication failure |

### Timing Issues

| Error Code | HTTP Status | Message | When Returned |
|------------|-------------|---------|---------------|
| `RequestExpired` | 403 | Request has expired. | Request timestamp is outside the allowed window (±15 minutes) |
| `MissingDateHeader` | 403 | Missing required header: x-amz-date. | X-Amz-Date header is missing |
| `InvalidDateFormat` | 403 | Invalid date format in X-Amz-Date header. | X-Amz-Date header is not in ISO 8601 format (YYYYMMDDTHHMMSSZ) |

## Authorization and Access Control Errors

### ACL-Based Access Denial

| Error Code | HTTP Status | Message | When Returned |
|------------|-------------|---------|---------------|
| `AccessDenied` | 403 | Access Denied. | Credential exists but ACL restrictions prevent access to the requested bucket/key |

**ACL Access Control Scenarios:**

ARMOR supports bucket and prefix-based access control lists (ACLs). When a credential has ACL restrictions configured, the following scenarios will result in `AccessDenied` errors:

1. **Bucket Mismatch** - Credential's ACL bucket restriction doesn't match the requested bucket
2. **Prefix Mismatch** - Credential's ACL prefix restriction doesn't match the requested key
3. **No Matching ACL** - No ACL entry allows access to the requested bucket/key combination

**Example:**

```yaml
# Credential configuration
credentials:
  RESTRICTEDKEY:
    access_key: RESTRICTEDKEY
    secret_key: SECRET123...
    acls:
      - bucket: "data-bucket"
        prefix: "allowed/"
```

With this configuration:
- ✓ `GET /data-bucket/allowed/file.txt` - Success
- ✗ `GET /data-bucket/forbidden/file.txt` - AccessDenied
- ✗ `GET /other-bucket/file.txt` - AccessDenied

## Malformed Signature Scenarios

ARMOR validates signature format and provides specific error codes:

| Scenario | Error Code | Performance |
|----------|------------|-------------|
| Non-hex signature characters | `SignatureDoesNotMatch` | < 50ms |
| Too short signature (< 32 bytes) | `SignatureDoesNotMatch` | < 50ms |
| Empty signature | `IncompleteSignature` | < 50ms |
| Random characters in signature | `SignatureDoesNotMatch` | < 50ms |

## Query Parameter Authentication Errors

ARMOR supports authentication via presigned URLs using query parameters. These scenarios can result in authentication errors:

| Error Code | HTTP Status | Message | When Returned |
|------------|-------------|---------|---------------|
| `MissingAuthenticationToken` | 403 | Missing Authentication Token | X-Amz-Credential query parameter is missing |
| `InvalidCredential` | 403 | Invalid credential format | X-Amz-Credential has insufficient parts (expected 5 parts) |
| `InvalidAccessKeyId` | 403 | The AWS Access Key Id you provided does not exist | Access key from credential parameter not found |
| `IncompleteSignature` | 403 | Authorization header is missing required fields | X-Amz-SignedHeaders or X-Amz-Signature query parameter is missing |
| `RequestExpired` | 403 | Request has expired | Presigned URL has exceeded its expiration time (X-Amz-Expires) |

**Presigned URL Format:**
```
https://bucket.s3.amazonaws.com/key?
  X-Amz-Algorithm=AWS4-HMAC-SHA256&
  X-Amz-Credential=ACCESSKEY/DATE/REGION/s3/aws4_request&
  X-Amz-Date=YYYYMMDDTHHMMSSZ&
  X-Amz-Expires=SECONDS&
  X-Amz-SignedHeaders=host&
  X-Amz-Signature=CALCULATED_SIGNATURE
```

## Request Method Errors

ARMOR validates that the HTTP method is appropriate for the requested operation:

| Error Code | HTTP Status | Message | When Returned |
|------------|-------------|---------|---------------|
| `MethodNotAllowed` | 405 | Method {METHOD} not allowed | HTTP method is not supported for the requested endpoint |

**Examples:**
- Unsupported POST operation on endpoints that only support GET
- Using DELETE on read-only operations

## Internal Server Errors

These errors indicate server-side problems during request processing:

| Error Code | HTTP Status | Message | When Returned |
|------------|-------------|---------|---------------|
| `InternalError` | 500 | Failed to {operation}: {error} | Server encountered an error during request processing |

**Common InternalError Scenarios:**
- Encryption key derivation failures
- DEK generation failures
- IV generation failures
- Header encoding failures
- Upload failures
- Temporary file creation failures

## Performance Guarantees

All authentication rejection scenarios respond within strict time limits:

| Test Type | Target | Actual | Environment |
|-----------|--------|--------|-------------|
| Unit test rejections | < 100ms | < 1ms | Local httptest |
| Integration test rejections | < 500ms | < 50ms | Real server |
| Malformed signature rejections | < 50ms | < 1ms | Local httptest |

### Performance Test Coverage

The test suite includes performance verification for all rejection scenarios:

1. **TestInvalidCredentialRejection/Rejection_happens_quickly** - Verifies < 100ms response time
2. **TestMalformedSignatureRejection/Rejection_happens_quickly** - Verifies < 50ms response time for malformed signatures

## Error Message Quality

All error responses include:

1. **Specific Error Code** - Identifies the exact problem (e.g., `InvalidAccessKeyId`, `SignatureDoesNotMatch`)
2. **Meaningful Message** - Human-readable description of the problem
3. **XML Format** - S3-compatible XML structure with proper escaping

### Message Validation

Test suite verifies that:
- Error messages are never empty
- Messages are at least 10 characters long
- Messages contain relevant keywords (authentication, signature, credential, algorithm, header, aws4)
- XML is properly escaped to prevent injection

## Response Consistency

All error responses maintain consistency:

### Headers
- Always return `Content-Type: application/xml`
- Always return appropriate HTTP status code (403 for auth errors)
- Response body is never empty

### Structure
- XML declaration with encoding: `<?xml version="1.0" encoding="UTF-8"?>`
- Root element: `<Error>`
- Two child elements: `<Code>` and `<Message>`
- Proper XML escaping for special characters

## Test Coverage Summary

The ARMOR test suite includes comprehensive coverage for rejection scenarios:

### Unit Tests (`invalid_credential_test.go`)
- 12 test scenarios covering:
  - Invalid AWS credentials
  - Malformed signatures  
  - Missing authentication headers
  - Malformed authorization headers
  - Insufficient credential parts
  - Missing required components (SignedHeaders, Signature, date)
  - Expired requests
  - Performance validation

### Unit Tests (`malformed_signature_test.go`)
- 20+ test scenarios covering:
  - Garbage signature strings (non-hex, too short, empty, random chars)
  - Invalid signature formats (missing algorithm, wrong algorithm, missing components)
  - Partial signatures (missing components)
  - Error message quality
  - Performance validation

### Unit Tests (`error_response_verification_test.go`)
- Comprehensive verification of all acceptance criteria:
  - Meaningful error messages for all rejection scenarios
  - Error messages specify the rejection reason
  - Response time under 100ms for all rejections
  - Consistent response headers across rejection types
  - Performance statistics and thresholds
  - Documentation generation for error response format

### Integration Tests (`invalid_credential_integration_test.go`)
- Real server tests with actual HTTP client
- Performance validation under realistic conditions
- End-to-end verification of error responses

### Headers Consistency (`error_response_test.go`)
- Verifies consistent headers across all rejection types
- Validates Content-Type header
- Ensures proper XML structure

### Authorization Tests (`auth_headers_doc_test.go`)
- Documents authentication rejection response headers
- Generates comprehensive header documentation
- Verifies error codes and messages for all auth scenarios

## Examples

### Example 1: Invalid Access Key

**Request:**
```http
GET /test-bucket/test-key HTTP/1.1
Host: test-bucket.s3.us-east-005.backblazeb2.com
Authorization: AWS4-HMAC-SHA256 Credential=INVALIDKEY/20250714/us-east-005/s3/aws4_request, SignedHeaders=host;x-amz-date, Signature=abc123...
X-Amz-Date: 20250714T044805Z
```

**Response:**
```http
HTTP/1.1 403 Forbidden
Content-Type: application/xml

<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>InvalidAccessKeyId</Code>
  <Message>The AWS Access Key Id you provided does not exist in our records.</Message>
</Error>
```

### Example 2: Missing Authentication

**Request:**
```http
GET /test-bucket/test-key HTTP/1.1
Host: test-bucket.s3.us-east-005.backblazeb2.com
```

**Response:**
```http
HTTP/1.1 403 Forbidden
Content-Type: application/xml

<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>MissingAuthenticationToken</Code>
  <Message>Missing authentication token.</Message>
</Error>
```

### Example 3: Expired Request

**Request:**
```http
GET /test-bucket/test-key HTTP/1.1
Host: test-bucket.s3.us-east-005.backblazeb2.com
Authorization: AWS4-HMAC-SHA256 Credential=TESTACCESSKEY/20250714/us-east-005/s3/aws4_request, SignedHeaders=host, Signature=!@#$%^&*()
X-Amz-Date: 20250714T044805Z
```

**Response:**
```http
HTTP/1.1 403 Forbidden
Content-Type: application/xml

<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>SignatureDoesNotMatch</Code>
  <Message>The request signature we calculated does not match the signature you provided.</Message>
</Error>
```

### Example 4: ACL Access Denied

**Request:**
```http
GET /protected-bucket/admin/config.yaml HTTP/1.1
Host: protected-bucket.s3.us-east-005.backblazeb2.com
Authorization: AWS4-HMAC-SHA256 Credential=RESTRICTEDKEY/20250714/us-east-005/s3/aws4_request, SignedHeaders=host;x-amz-date, Signature=valid123...
X-Amz-Date: 20250714T044805Z
```

**Configuration:**
```yaml
credentials:
  RESTRICTEDKEY:
    access_key: RESTRICTEDKEY
    secret_key: SECRET123...
    acls:
      - bucket: "protected-bucket"
        prefix: "public/"
```

**Response:**
```http
HTTP/1.1 403 Forbidden
Content-Type: application/xml

<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>AccessDenied</Code>
  <Message>Access Denied</Message>
</Error>
```

### Example 5: Expired Presigned URL

**Request:**
```http
GET /data-bucket/file.csv?
  X-Amz-Algorithm=AWS4-HMAC-SHA256&
  X-Amz-Credential=TESTACCESSKEY/20250701/us-east-005/s3/aws4_request&
  X-Amz-Date=20250701T120000Z&
  X-Amz-Expires=3600&
  X-Amz-SignedHeaders=host&
  X-Amz-Signature=abc123...
HTTP/1.1
Host: data-bucket.s3.us-east-005.backblazeb2.com
```

**Response:**
```http
HTTP/1.1 403 Forbidden
Content-Type: application/xml

<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>RequestExpired</Code>
  <Message>Request has expired</Message>
</Error>
```

### Example 6: Invalid Date Format

**Request:**
```http
GET /test-bucket/test-key HTTP/1.1
Host: test-bucket.s3.us-east-005.backblazeb2.com
Authorization: AWS4-HMAC-SHA256 Credential=TESTACCESSKEY/20250714/us-east-005/s3/aws4_request, SignedHeaders=host;x-amz-date, Signature=valid123...
X-Amz-Date: July-14-2025
```

**Response:**
```http
HTTP/1.1 403 Forbidden
Content-Type: application/xml

<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>InvalidDateFormat</Code>
  <Message>Invalid date format in X-Amz-Date header</Message>
</Error>
```

## Implementation

Error responses are generated by two implementations:

1. **Server Handler** (`internal/server/server.go:796-805`)
   ```go
   func (s *Server) writeError(w http.ResponseWriter, code, message string, statusCode int) {
       w.Header().Set("Content-Type", "application/xml")
       w.WriteHeader(statusCode)
       // XML generation with proper escaping
   }
   ```

2. **Handlers Package** (`internal/server/handlers/handlers.go:2695-2704`)
   ```go
   func (h *Handlers) writeError(w http.ResponseWriter, code, message string, statusCode int) {
       w.Header().Set("Content-Type", "application/xml")
       w.WriteHeader(statusCode)
       // XML generation with proper escaping
   }
   ```

Both implementations ensure:
- Consistent Content-Type header
- Proper XML escaping to prevent injection
- S3-compatible format

## Testing

To run the error response test suite:

```bash
# Run all rejection tests
go test -v -run "TestInvalidCredentialRejection|TestMalformedSignatureRejection" ./internal/server/

# Run headers consistency test
go test -v -run TestErrorResponseHeadersConsistency ./internal/server/

# Run integration tests (requires INTEGRATION_TEST=1)
INTEGRATION_TEST=1 go test -v -run TestInvalidCredentialsIntegration ./internal/server/
```

## Maintenance

When adding new error scenarios:

1. **Add test coverage** - Create tests in appropriate test file
2. **Verify error code** - Use existing S3 error code when possible
3. **Check performance** - Ensure response time < 100ms
4. **Validate headers** - Confirm Content-Type and XML structure
5. **Update this doc** - Document new error code and scenario

## References

- [S3 Error Responses](https://docs.aws.amazon.com/AmazonS3/latest/API/ErrorResponses.html)
- Test files:
  - `internal/server/invalid_credential_test.go`
  - `internal/server/malformed_signature_test.go`
  - `internal/server/invalid_credential_integration_test.go`
  - `internal/server/error_response_test.go`
