# ARMOR Error Response Catalog

This document catalogs all API rejection scenarios and their current error responses in ARMOR.

## Error Response Format

All error responses follow the S3 XML error format:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>ERROR_CODE</Code>
  <Message>Human-readable error message</Message>
</Error>
```

**Response Headers:**
- `Content-Type: application/xml` (consistent across all error types)
- HTTP status code varies by error type

**Performance Characteristics:**
- Average response time: <1ms (local testing)
- Maximum response time: <100ms under normal conditions
- Response time includes authentication verification

---

## Authentication & Authorization Errors (403 Forbidden)

### 1. MissingAuthenticationToken
**Scenario:** Authorization header is completely missing from the request.

**Test Endpoint:** Any S3 operation without auth header
```bash
GET /test-bucket/test-key
# No Authorization header
```

**Error Response:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>MissingAuthenticationToken</Code>
  <Message>Missing Authentication Token</Message>
</Error>
```

**HTTP Status:** 403 Forbidden

**Code Path:** `server.go:669-676`, `auth.go:118-120`

---

### 2. InvalidAccessKeyId
**Scenario:** The provided access key does not exist in the configured credentials.

**Test Endpoint:** Any S3 operation with non-existent access key
```bash
GET /test-bucket/test-key
Authorization: AWS4-HMAC-SHA256 Credential=INVALIDKEY/...
```

**Error Response:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>InvalidAccessKeyId</Code>
  <Message>The AWS Access Key Id you provided does not exist</Message>
</Error>
```

**HTTP Status:** 403 Forbidden

**Code Path:** `server.go:669-676`, `auth.go:127-131`

---

### 3. SignatureDoesNotMatch
**Scenario:** The calculated signature does not match the signature provided (typically wrong secret key).

**Test Endpoint:** Any S3 operation with incorrect secret key
```bash
GET /test-bucket/test-key
Authorization: AWS4-HMAC-SHA256 Credential=TESTACCESSKEY/...
Signature=<calculated_with_wrong_secret>
```

**Error Response:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>SignatureDoesNotMatch</Code>
  <Message>The request signature we calculated does not match the signature you provided</Message>
</Error>
```

**HTTP Status:** 403 Forbidden

**Code Path:** `server.go:669-676`, `auth.go:160-162`

---

### 4. InvalidAlgorithm
**Scenario:** Authorization header uses an unsupported algorithm (only AWS4-HMAC-SHA256 is supported).

**Test Endpoint:**
```bash
GET /test-bucket/test-key
Authorization: AWS3-HMAC-SHA256 Credential=...
```

**Error Response:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>InvalidAlgorithm</Code>
  <Message>Only AWS4-HMAC-SHA256 is supported</Message>
</Error>
```

**HTTP Status:** 403 Forbidden

**Code Path:** `server.go:669-676`, `auth.go:63-65`

---

### 5. IncompleteSignature
**Scenario:** Authorization header is missing required fields (Credential, SignedHeaders, or Signature).

**Test Endpoint:**
```bash
GET /test-bucket/test-key
Authorization: AWS4-HMAC-SHA256 Credential=TESTACCESSKEY/...
# Missing Signature or SignedHeaders
```

**Error Response:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>IncompleteSignature</Code>
  <Message>Authorization header is missing required fields</Message>
</Error>
```

**HTTP Status:** 403 Forbidden

**Code Path:** `server.go:669-676`, `auth.go:106-109`

---

### 6. MissingDateHeader
**Scenario:** X-Amz-Date header is missing from the request.

**Test Endpoint:**
```bash
GET /test-bucket/test-key
Authorization: AWS4-HMAC-SHA256 Credential=TESTACCESSKEY/...
# No X-Amz-Date header
```

**Error Response:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>MissingDateHeader</Code>
  <Message>Missing X-Amz-Date header</Message>
</Error>
```

**HTTP Status:** 403 Forbidden

**Code Path:** `server.go:669-676`, `auth.go:134-137`

---

### 7. InvalidDateFormat
**Scenario:** X-Amz-Date header has invalid format (must be YYYYMMDDTHHMMSSZ).

**Test Endpoint:**
```bash
GET /test-bucket/test-key
X-Amz-Date: invalid-date-format
```

**Error Response:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>InvalidDateFormat</Code>
  <Message>Invalid date format in X-Amz-Date header</Message>
</Error>
```

**HTTP Status:** 403 Forbidden

**Code Path:** `server.go:669-676`, `auth.go:140-143`

---

### 8. RequestExpired
**Scenario:** Request timestamp is outside the allowed 15-minute window.

**Test Endpoint:**
```bash
GET /test-bucket/test-key
X-Amz-Date: 20250101T000000Z  # >15 minutes from now
```

**Error Response:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>RequestExpired</Code>
  <Message>Request has expired</Message>
</Error>
```

**HTTP Status:** 403 Forbidden

**Code Path:** `server.go:669-676`, `auth.go:145-147`

---

### 9. InvalidCredential
**Scenario:** Credential format in Authorization header is malformed.

**Test Endpoint:**
```bash
GET /test-bucket/test-key
Authorization: AWS4-HMAC-SHA256 Credential=malformed
```

**Error Response:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>InvalidCredential</Code>
  <Message>Invalid credential format</Message>
</Error>
```

**HTTP Status:** 403 Forbidden

**Code Path:** `server.go:669-676`, `auth.go:89-92`

---

### 10. AccessDenied
**Scenario:** ACL-based access control rejection (credential exists but lacks permission).

**Test Endpoint:** Any S3 operation with credentials restricted by ACL
```bash
GET /test-bucket/restricted-key
Authorization: AWS4-HMAC-SHA256 Credential=RESTRICTEDKEY/...
```

**Error Response:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>AccessDenied</Code>
  <Message>Access Denied</Message>
</Error>
```

**HTTP Status:** 403 Forbidden

**Code Path:** `server.go:686-689`, `auth.go:293-318`

---

## Object Errors

### 11. NoSuchKey
**Scenario:** Object does not exist in the bucket.

**Test Endpoint:**
```bash
GET /test-bucket/nonexistent-key
```

**Error Response:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>NoSuchKey</Code>
  <Message>Object not found: ...</Message>
</Error>
```

**HTTP Status:** 404 Not Found

**Code Path:** `handlers.go:610-612`

---

### 12. InvalidRange
**Scenario:** Invalid Range header format or range out of bounds.

**Test Endpoint:**
```bash
GET /test-bucket/test-key
Range: invalid-range
```

**Error Response:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>InvalidRange</Code>
  <Message>Invalid range: ...</Message>
</Error>
```

**HTTP Status:** 400 Bad Request

**Code Path:** `handlers.go:863-864`, `handlers.go:1121-1124`

---

## Request Errors

### 13. InvalidRequest
**Scenario:** Unsupported POST operation or invalid request parameters.

**Test Endpoint:**
```bash
POST /test-bucket/test-key
# Without ?uploads, ?uploadId, or ?delete
```

**Error Response:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>InvalidRequest</Code>
  <Message>Unsupported POST operation</Message>
</Error>
```

**HTTP Status:** 400 Bad Request

**Code Path:** `handlers.go:256-257`

---

### 14. MalformedXML
**Scenario:** Failed to parse request XML (e.g., DeleteObjects, CompleteMultipartUpload).

**Test Endpoint:**
```bash
POST /test-bucket/test-key?delete
Body: <invalid>xml</>
```

**Error Response:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>MalformedXML</Code>
  <Message>Failed to parse XML: ...</Message>
</Error>
```

**HTTP Status:** 400 Bad Request

**Code Path:** `handlers.go:1706`, `handlers.go:2073`

---

### 15. MethodNotAllowed
**Scenario:** HTTP method not supported for the endpoint.

**Test Endpoint:** Any unsupported HTTP method
```bash
PATCH /test-bucket/test-key
```

**Error Response:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>MethodNotAllowed</Code>
  <Message>Method PATCH not allowed</Message>
</Error>
```

**HTTP Status:** 405 Method Not Allowed

**Code Path:** `handlers.go:259-260`

---

### 16. PreconditionFailed
**Scenario:** Conditional request (If-Match, If-Unmodified-Since) failed.

**Test Endpoint:**
```bash
GET /test-bucket/test-key
If-Match: "wrong-etag"
```

**Error Response:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>PreconditionFailed</Code>
  <Message>Precondition failed</Message>
</Error>
```

**HTTP Status:** 412 Precondition Failed

**Code Path:** `handlers.go:623-624`, `handlers.go:684`, `handlers.go:1146`, `handlers.go:1195`

---

## Bucket Errors

### 17. NoSuchBucket
**Scenario:** Bucket does not exist or is not accessible.

**Test Endpoint:**
```bash
HEAD /nonexistent-bucket
```

**Error Response:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>NoSuchBucket</Code>
  <Message>Bucket not found: ...</Message>
</Error>
```

**HTTP Status:** 404 Not Found

**Code Path:** `handlers.go:1592`, `handlers.go:1606`, `handlers.go:1634`

---

## Multipart Upload Errors

### 18. NoSuchUpload
**Scenario:** Multipart upload ID does not exist or does not match bucket/key.

**Test Endpoint:**
```bash
GET /test-bucket/test-key?uploadId=invalid-upload-id
```

**Error Response:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>NoSuchUpload</Code>
  <Message>Multipart upload not found: ...</Message>
</Error>
```

**HTTP Status:** 404 Not Found

**Code Path:** `handlers.go:1949-1952`, `handlers.go:2044-2047`, `handlers.go:2202-2205`

---

## Internal Server Errors (500)

### 19. InternalError
**Scenario:** Various internal failures (encryption, decryption, backend errors).

**Test Endpoints:**
- Failed to generate DEK
- Failed to encrypt/decrypt
- Backend operation failure
- Key management errors

**Error Response:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>InternalError</Code>
  <Message>Failed to [operation]: ...</Message>
</Error>
```

**HTTP Status:** 500 Internal Server Error

**Code Path:** Multiple locations throughout `handlers.go` and `server.go`

---

## Error Code Reference Table

| Error Code | HTTP Status | Category | Message |
|-----------|-------------|----------|---------|
| MissingAuthenticationToken | 403 | Auth | Missing Authentication Token |
| InvalidAccessKeyId | 403 | Auth | The AWS Access Key Id you provided does not exist |
| SignatureDoesNotMatch | 403 | Auth | The request signature we calculated does not match the signature you provided |
| InvalidAlgorithm | 403 | Auth | Only AWS4-HMAC-SHA256 is supported |
| IncompleteSignature | 403 | Auth | Authorization header is missing required fields |
| MissingDateHeader | 403 | Auth | Missing X-Amz-Date header |
| InvalidDateFormat | 403 | Auth | Invalid date format in X-Amz-Date header |
| RequestExpired | 403 | Auth | Request has expired |
| InvalidCredential | 403 | Auth | Invalid credential format |
| AccessDenied | 403 | Auth | Access Denied |
| NoSuchKey | 404 | Object | Object not found |
| InvalidRange | 400 | Object | Invalid range |
| InvalidRequest | 400 | Request | Unsupported POST operation |
| MalformedXML | 400 | Request | Failed to parse XML |
| MethodNotAllowed | 405 | Request | Method X not allowed |
| PreconditionFailed | 412 | Request | Precondition failed |
| NoSuchBucket | 404 | Bucket | Bucket not found |
| NoSuchUpload | 404 | Multipart | Multipart upload not found |
| InternalError | 500 | Internal | Failed to [operation] |

---

## Test Coverage

All authentication error scenarios have test coverage in:
- `internal/server/invalid_credential_test.go` - Basic invalid credential tests
- `internal/server/malformed_signature_test.go` - Malformed signature tests
- `internal/server/error_response_verification_test.go` - Comprehensive error verification
- `internal/server/error_response_test.go` - Header consistency tests

All tests verify:
1. Correct HTTP status code (403 for auth errors)
2. Correct error code in XML response
3. Meaningful error messages (non-empty, descriptive)
4. Response time under 100ms
5. Consistent Content-Type header (application/xml)

---

## Acceptance Criteria Status

✅ **List of all rejection scenarios** - Documented above (19+ scenarios)
✅ **Current error response format for each scenario** - Documented with XML examples
✅ **Test endpoints or code paths for each scenario** - Included in each scenario

All acceptance criteria met.
