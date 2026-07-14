# ARMOR Error Response Catalog

## Overview

This document catalogs all API rejection scenarios and their current error responses in the ARMOR S3-compatible HTTP server.

**Current Response Format:** All S3 API errors return XML in the following format:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>ERROR_CODE</Code>
  <Message>Human-readable error message</Message>
</Error>
```

**Content-Type:** `application/xml`

## Authentication & Authorization Errors (403 Forbidden)

### Missing Authentication Token

- **HTTP Status:** 403 Forbidden
- **Error Code:** `MissingAuthenticationToken`
- **Error Message:** "Missing Authentication Token"
- **Trigger:** Authorization header is missing from request
- **Test Endpoint:** `GET /test-bucket/test-key` (without auth headers)
- **Code Path:** `server.go:669-676`, `auth.go:339-340`
- **Test File:** `error_response_verification_test.go:64-70`

### Invalid Algorithm

- **HTTP Status:** 403 Forbidden
- **Error Code:** `InvalidAlgorithm`
- **Error Message:** "Only AWS4-HMAC-SHA256 is supported"
- **Trigger:** Authorization header uses unsupported algorithm (e.g., AWS3-HMAC-SHA256, AWS4-HMAC-SHA1)
- **Test Endpoint:** Authorization header with `AWS3-HMAC-SHA256` or `AWS4-HMAC-SHA1`
- **Code Path:** `server.go:669-676`, `auth.go:63-65`, `auth.go:341`
- **Test File:** `malformed_signature_test.go:105-113`

### Invalid Credential Format

- **HTTP Status:** 403 Forbidden
- **Error Code:** `InvalidCredential`
- **Error Message:** "Invalid credential format"
- **Trigger:** Credential string in Authorization header has incorrect format
- **Test Endpoint:** Malformed credential (e.g., insufficient parts)
- **Code Path:** `server.go:669-676`, `auth.go:89-92`, `auth.go:342`
- **Test File:** `malformed_signature_test.go:130-138`

### Incomplete Signature

- **HTTP Status:** 403 Forbidden
- **Error Code:** `IncompleteSignature`
- **Error Message:** "Authorization header is missing required fields"
- **Trigger:** Authorization header is missing required components (Credential, SignedHeaders, or Signature)
- **Test Endpoint:** Authorization header missing signature component
- **Code Path:** `server.go:669-676`, `auth.go:107-109`, `auth.go:343`
- **Test File:** `malformed_signature_test.go:115-122`

### Invalid Access Key ID

- **HTTP Status:** 403 Forbidden
- **Error Code:** `InvalidAccessKeyId`
- **Error Message:** "The AWS Access Key Id you provided does not exist"
- **Trigger:** Access key not found in credentials configuration
- **Test Endpoint:** `GET /test-bucket/test-key` with non-existent access key
- **Code Path:** `server.go:669-676`, `auth.go:128-131`, `auth.go:344`
- **Test File:** `error_response_verification_test.go:73-80`

### Missing X-Amz-Date Header

- **HTTP Status:** 403 Forbidden
- **Error Code:** `MissingDateHeader`
- **Error Message:** "Missing X-Amz-Date header"
- **Trigger:** X-Amz-Date header not present in request
- **Test Endpoint:** Valid Authorization header without X-Amz-Date
- **Code Path:** `server.go:669-676`, `auth.go:134-137`, `auth.go:345`
- **Test File:** `error_response_verification_test.go:102-111`

### Invalid Date Format

- **HTTP Status:** 403 Forbidden
- **Error Code:** `InvalidDateFormat`
- **Error Message:** "Invalid date format in X-Amz-Date header"
- **Trigger:** X-Amz-Date header is not in correct format (YYYYMMDDTHHmmssZ)
- **Test Endpoint:** X-Amz-Date with invalid format
- **Code Path:** `server.go:669-676`, `auth.go:140-143`, `auth.go:346`
- **Code File:** `auth.go`

### Request Expired

- **HTTP Status:** 403 Forbidden
- **Error Code:** `RequestExpired`
- **Error Message:** "Request has expired"
- **Trigger:** Request timestamp is outside 15-minute window (±15 minutes)
- **Test Endpoint:** Request with timestamp >15 minutes in past or future
- **Code Path:** `server.go:669-676`, `auth.go:145-147`, `auth.go:347`
- **Test File:** `error_response_verification_test.go:113-121`

### Signature Does Not Match

- **HTTP Status:** 403 Forbidden
- **Error Code:** `SignatureDoesNotMatch`
- **Error Message:** "The request signature we calculated does not match the signature you provided"
- **Trigger:** Calculated signature doesn't match provided signature (wrong secret key or tampered request)
- **Test Endpoint:** Valid access key with wrong secret key
- **Code Path:** `server.go:669-676`, `auth.go:160-162`, `auth.go:348`
- **Test File:** `error_response_verification_test.go:82-89`

### Access Denied (ACL)

- **HTTP Status:** 403 Forbidden
- **Error Code:** `AccessDenied`
- **Error Message:** "Access Denied"
- **Trigger:** Credential's ACL does not allow access to requested bucket/key
- **Test Endpoint:** Request with credential restricted to different prefix
- **Code Path:** `server.go:686-690`, `auth.go:293-318`, `auth.go:349`
- **Test File:** `error_response_verification_test.go:22-37`

## Bad Request Errors (400 Bad Request)

### Invalid Request Body

- **HTTP Status:** 400 Bad Request
- **Error Code:** Various (context-dependent)
- **Error Message:** "Invalid request body: {error details}"
- **Trigger:** Invalid JSON in POST request body
- **Test Endpoint:** `POST /admin/presign` with invalid JSON
- **Code Path:** `server.go:858`, `server.go:1292`
- **Response Format:** Plain text (not XML for admin endpoints)

### Key Is Required

- **HTTP Status:** 400 Bad Request
- **Error Code:** N/A
- **Error Message:** "key is required"
- **Trigger:** Presign request missing 'key' field
- **Test Endpoint:** `POST /admin/presign` without key
- **Code Path:** `server.go:870`
- **Response Format:** Plain text (not XML)

### Invalid Expires In

- **HTTP Status:** 400 Bad Request
- **Error Code:** N/A
- **Error Message:** "Invalid expires_in: {error details}"
- **Trigger:** Invalid expiration duration format in presign request
- **Test Endpoint:** `POST /admin/presign` with invalid expires_in
- **Code Path:** `server.go:885`
- **Response Format:** Plain text (not XML)

### Missing Token

- **HTTP Status:** 400 Bad Request
- **Error Code:** N/A
- **Error Message:** "Missing token"
- **Trigger:** Share endpoint called without token in path
- **Test Endpoint:** `GET /share/` (empty token)
- **Code Path:** `server.go:926`
- **Response Format:** Plain text (not XML)

### Invalid Token

- **HTTP Status:** 400 Bad Request
- **Error Code:** N/A
- **Error Message:** "Invalid token"
- **Trigger:** Share token cannot be verified
- **Test Endpoint:** `GET /share/{invalid-token}`
- **Code Path:** `server.go:941`
- **Response Format:** Plain text (not XML)

### Invalid Range

- **HTTP Status:** 400 Bad Request
- **Error Code:** `InvalidRange`
- **Error Message:** "Invalid range: {error details}"
- **Trigger:** Range header has invalid format or out of bounds
- **Test Endpoint:** `GET /test-bucket/test-key` with `Range: invalid`
- **Code Path:** `handlers.go:863`
- **Test File:** `handlers_test.go`

### Method Not Allowed

- **HTTP Status:** 405 Method Not Allowed
- **Error Code:** `MethodNotAllowed`
- **Error Message:** "Method {method} not allowed"
- **Trigger:** Unsupported HTTP method for endpoint
- **Test Endpoint:** `POST /bucket/key` (unsupported method)
- **Code Path:** `handlers.go:259`, `server.go:488,512,578,601,620,832,919`
- **Response Format:** Plain text (not XML)

### Invalid Copy Source

- **HTTP Status:** 400 Bad Request
- **Error Code:** `InvalidCopySource`
- **Error Message:** "Invalid copy source format"
- **Trigger:** x-amz-copy-source header has invalid format
- **Test Endpoint:** `PUT /bucket/key` with invalid `x-amz-copy-source`
- **Code Path:** `handlers.go:1252`
- **Response Format:** XML

### Missing Copy Source

- **HTTP Status:** 400 Bad Request
- **Error Code:** `InvalidRequest`
- **Error Message:** "Missing x-amz-copy-source header"
- **Trigger:** Copy operation without x-amz-copy-source header
- **Test Endpoint:** `PUT /bucket/key?copy` without source header
- **Code Path:** `handlers.go:1244`
- **Response Format:** XML

### Unsupported POST Operation

- **HTTP Status:** 400 Bad Request
- **Error Code:** `InvalidRequest`
- **Error Message:** "Unsupported POST operation"
- **Trigger:** POST request to unsupported endpoint
- **Test Endpoint:** `POST /bucket/key` without query params
- **Code Path:** `handlers.go:256`
- **Response Format:** XML

## Not Found Errors (404 Not Found)

### Object Not Found (HeadObject)

- **HTTP Status:** 404 Not Found
- **Error Code:** `NoSuchKey`
- **Error Message:** "Object not found: {error details}"
- **Trigger:** Requested object does not exist in backend
- **Test Endpoint:** `HEAD /test-bucket/nonexistent-key`
- **Code Path:** `handlers.go:611`
- **Response Format:** XML

### Object Not Found (GetObject)

- **HTTP Status:** 404 Not Found
- **Error Code:** `NoSuchKey`
- **Error Message:** "Object not found"
- **Trigger:** GetObject request for non-existent object
- **Test Endpoint:** `GET /test-bucket/nonexistent-key`
- **Code Path:** `handlers.go:1164`
- **Response Format:** XML

### Source Object Not Found (Copy)

- **HTTP Status:** 404 Not Found
- **Error Code:** `NoSuchKey`
- **Error Message:** "Source object not found: {error details}"
- **Trigger:** Copy operation with non-existent source
- **Test Endpoint:** `PUT /dest/key?copy` with non-existent source
- **Code Path:** `handlers.go:1263`
- **Response Format:** XML

### Key Not Found (B2 Keys)

- **HTTP Status:** 404 Not Found
- **Error Code:** N/A
- **Error Message:** "key not found"
- **Trigger:** Attempt to delete non-existent B2 application key
- **Test Endpoint:** `DELETE /admin/b2/keys/{non-existent-id}`
- **Code Path:** `server.go:1348`
- **Response Format:** JSON

## Gone Errors (410 Gone)

### Link Expired

- **HTTP Status:** 410 Gone
- **Error Code:** N/A
- **Error Message:** "Link expired"
- **Trigger:** Pre-signed URL has exceeded its expiration time
- **Test Endpoint:** `GET /share/{expired-token}`
- **Code Path:** `server.go:934`
- **Response Format:** Plain text (not XML)

## Forbidden Errors (403 Forbidden - Non-Auth)

### Invalid Link (Share)

- **HTTP Status:** 403 Forbidden
- **Error Code:** N/A
- **Error Message:** "Invalid link"
- **Trigger:** Share token signature verification fails
- **Test Endpoint:** `GET /share/{tampered-token}`
- **Code Path:** `server.go:938`
- **Response Format:** Plain text (not XML)

## Precondition Failed Errors (412 Precondition Failed)

### Precondition Failed

- **HTTP Status:** 412 Precondition Failed
- **Error Code:** `PreconditionFailed`
- **Error Message:** "Precondition failed"
- **Trigger:** If-Match or If-None-Match header condition not met
- **Test Endpoint:** `GET /test-bucket/test-key` with `If-Match: invalid-etag`
- **Code Path:** `handlers.go:623,684,1033,1042,1146,1195`
- **Response Format:** XML

## Internal Server Errors (500 Internal Server Error)

### Encryption/Decryption Errors

- **HTTP Status:** 500 Internal Server Error
- **Error Code:** `InternalError`
- **Error Message:** "Failed to {operation}: {error details}"
- **Trigger:** Various crypto operation failures
- **Test Endpoint:** Various operations when crypto fails
- **Code Path:** `handlers.go:285-575` (multiple crypto failure points)
- **Examples:**
  - "Failed to read body"
  - "Failed to get encryption key"
  - "Failed to generate DEK"
  - "Failed to generate IV"
  - "Failed to wrap DEK"
  - "Failed to create header"
  - "Failed to encode header"
  - "Failed to create encryptor"
  - "Failed to encrypt"
  - "Failed to upload"
  - "Failed to get decryption key"
  - "Failed to unwrap DEK"
  - "Failed to create decryptor"
  - "Failed to decrypt range"
  - "Failed to prefetch HMAC table"
  - "Failed to read HMAC table"
  - "Failed to get object stream"
  - "Failed to read header"
  - "Failed to decode header"
- **Response Format:** XML

### Backend Operation Failures

- **HTTP Status:** 500 Internal Server Error
- **Error Code:** `InternalError`
- **Error Message:** "Failed to {operation}: {error details}"
- **Trigger:** Backend storage operation failures
- **Test Endpoint:** Any operation when backend is unavailable
- **Code Path:** `handlers.go:378,566,631,720,726,734,742,749,898,937,944`
- **Examples:**
  - "Failed to upload"
  - "Failed to get object"
  - "Failed to delete"
  - "Failed to translate range"
- **Response Format:** XML

### Metadata Parse Errors

- **HTTP Status:** 500 Internal Server Error
- **Error Code:** `InternalError`
- **Error Message:** "Failed to parse ARMOR metadata"
- **Trigger:** Corrupted ARMOR metadata on object
- **Test Endpoint:** Get object with corrupted metadata
- **Code Path:** `handlers.go:648,1283`
- **Response Format:** XML

### MEK Export/Rotation Errors

- **HTTP Status:** 500 Internal Server Error
- **Error Code:** N/A
- **Error Message:** JSON error response
- **Trigger:** Key rotation or export failure
- **Test Endpoint:** `POST /admin/key/rotate`, `GET /admin/key/export`
- **Code Path:** `server.go:550-556`
- **Response Format:** JSON

### URL Generation Failure

- **HTTP Status:** 500 Internal Server Error
- **Error Code:** N/A
- **Error Message:** "Failed to generate URL: {error details}"
- **Trigger:** Presign URL generation failure
- **Test Endpoint:** `POST /admin/presign`
- **Code Path:** `server.go:902`
- **Response Format:** Plain text (not XML)

### Audit Failure

- **HTTP Status:** 500 Internal Server Error
- **Error Code:** N/A
- **Error Message:** JSON error response
- **Trigger:** Audit operation fails
- **Test Endpoint:** `GET /armor/audit`
- **Code Path:** `server.go:628-634`
- **Response Format:** JSON

### B2 Key Management Failures

- **HTTP Status:** 500 Internal Server Error
- **Error Code:** N/A
- **Error Message:** "Failed to list keys: {error}" or "Failed to create key: {error}"
- **Trigger:** B2 key management API failures
- **Test Endpoint:** `GET /admin/b2/keys`, `POST /admin/b2/keys`
- **Code Path:** `server.go:1280,1312`
- **Response Format:** JSON

## Service Unavailable Errors (503 Service Unavailable)

### Canary Check Failed (ReadyZ)

- **HTTP Status:** 503 Service Unavailable
- **Error Code:** N/A
- **Error Message:** "Not ready - canary check failed"
- **Trigger:** Canary monitor reports unhealthy
- **Test Endpoint:** `GET /readyz` when canary unhealthy
- **Code Path:** `server.go:450-453`
- **Response Format:** Plain text (not XML)

### Manifest Writer Never Flushed

- **HTTP Status:** 503 Service Unavailable
- **Error Code:** N/A
- **Error Message:** "Not ready - manifest writer has never flushed"
- **Trigger:** Manifest writer hasn't flushed within threshold
- **Test Endpoint:** `GET /readyz` when manifest not flushing
- **Code Path:** `server.go:472-473`
- **Response Format:** Plain text (not XML)

### Manifest Writer Last Flush Expired

- **HTTP Status:** 503 Service Unavailable
- **Error Code:** N/A
- **Error Message:** "Not ready - manifest writer last flush {duration} ago (threshold 1m0s)"
- **Trigger:** Last manifest flush was too long ago
- **Test Endpoint:** `GET /readyz` when last flush old
- **Code Path:** `server.go:475`
- **Response Format:** Plain text (not XML)

### No Health Signal Available

- **HTTP Status:** 503 Service Unavailable
- **Error Code:** N/A
- **Error Message:** "Not ready - no health signal available"
- **Trigger:** Neither canary nor manifest writer available
- **Test Endpoint:** `GET /readyz` when no health signals
- **Code Path:** `server.go:481-482`
- **Response Format:** Plain text (not XML)

### B2 Key Management Not Available

- **HTTP Status:** 503 Service Unavailable
- **Error Code:** N/A
- **Error Message:** "B2 key management not available - check B2 credentials"
- **Trigger:** B2 keys client not initialized
- **Test Endpoint:** `GET/POST/DELETE /admin/b2/keys` without B2 client
- **Code Path:** `server.go:1251,1329`
- **Response Format:** JSON

### Key Verification Unavailable

- **HTTP Status:** 503 Service Unavailable (but 200 OK in code)
- **Error Code:** N/A
- **Error Message:** "Canary monitor not configured"
- **Trigger:** Canary monitor not configured
- **Test Endpoint:** `GET /admin/key/verify` without canary
- **Code Path:** `server.go:493-496`
- **Response Format:** JSON

## Method Not Allowed Errors (405 Method Not Allowed)

### Invalid HTTP Method

- **HTTP Status:** 405 Method Not Allowed
- **Error Code:** N/A
- **Error Message:** "Method not allowed"
- **Trigger:** Wrong HTTP method for endpoint
- **Test Endpoint:** `POST /admin/key/verify` (should be GET)
- **Code Path:** `server.go:488,512,578,601,620,832,919`
- **Response Format:** Plain text (not XML)

## Admin API Request Errors

### Must Include Confirm

- **HTTP Status:** 400 Bad Request
- **Error Code:** N/A
- **Error Message:** "Must include ?confirm=yes to export key"
- **Trigger:** Key export without confirmation parameter
- **Test Endpoint:** `GET /admin/key/export` without confirm
- **Code Path:** `server.go:583`
- **Response Format:** Plain text (not XML)

### Failed to Read Request Body

- **HTTP Status:** 400 Bad Request
- **Error Code:** N/A
- **Error Message:** "Failed to read request body: {error}"
- **Trigger:** Invalid request body in key rotation
- **Test Endpoint:** `POST /admin/key/rotate` with invalid body
- **Code Path:** `server.go:519`
- **Response Format:** Plain text (not XML)

### Invalid Hex-Encoded MEK

- **HTTP Status:** 400 Bad Request
- **Error Code:** N/A
- **Error Message:** "Invalid hex-encoded MEK"
- **Trigger:** Non-hex characters in 64-char MEK string
- **Test Endpoint:** `POST /admin/key/rotate` with invalid hex
- **Code Path:** `server.go:531`
- **Response Format:** Plain text (not XML)

### Invalid MEK Length

- **HTTP Status:** 400 Bad Request
- **Error Code:** N/A
- **Error Message:** "Invalid MEK length: expected 32 bytes or 64 hex chars, got {n}"
- **Trigger:** MEK with incorrect length
- **Test Endpoint:** `POST /admin/key/rotate` with wrong length
- **Code Path:** `server.go:538`
- **Response Format:** Plain text (not XML)

### B2 Key Creation Missing Fields

- **HTTP Status:** 400 Bad Request
- **Error Code:** N/A
- **Error Message:** "name is required" or "capabilities is required"
- **Trigger:** Missing required fields in B2 key creation
- **Test Endpoint:** `POST /admin/b2/keys` without name or capabilities
- **Code Path:** `server.go:1297,1302`
- **Response Format:** JSON

## Response Format Inconsistencies

### S3 API Errors (XML Format)

**Locations:** Most handlers in `handlers.go`, authentication errors in `server.go:writeError()`

**Format:**
- Content-Type: `application/xml`
- HTTP status: 403/400/404/412/500
- Body: XML with `<Error>` containing `<Code>` and `<Message>`

### Admin API Errors (Plain Text Format)

**Locations:** `server.go` admin endpoints

**Format:**
- Content-Type: `text/plain` (usually)
- HTTP status: Various (400/403/404/405/410/500/503)
- Body: Plain text error message

### Admin API Errors (JSON Format)

**Locations:** Some admin endpoints

**Format:**
- Content-Type: `application/json`
- HTTP status: 500/503
- Body: JSON with `{"error": "message"}` or structured error object

### JSON Format Admin Errors

Examples:
- Key rotation failures: `{"status":"failed","error":"...","result":...}`
- B2 key errors: `{"error":"message"}`
- Audit failures: `{"status":"error","error":"..."}`

## Performance Characteristics

All error responses are designed to complete quickly:

- **Target:** <100ms for all rejections
- **Actual (local testing):** <1ms average
- **Authentication rejections:** Typically <1ms (no external calls)
- **Backend errors:** May take longer depending on B2 latency

## Test Coverage

### Authentication/Authorization Tests

- `error_response_verification_test.go` - Comprehensive auth error scenarios
- `malformed_signature_test.go` - Signature format validation
- `invalid_credential_test.go` - Invalid credential scenarios
- `error_response_test.go` - Header consistency

### Handler Error Tests

- `handlers_test.go` - Precondition failed scenarios
- `handlers_prefix_test.go` - ACL enforcement

### Admin API Tests

- `b2keys_test.go` - B2 key management errors
- `readyz_test.go` - Health check responses

## Summary Statistics

**Total Error Scenarios:** 60+

**By HTTP Status Code:**
- 400 Bad Request: 12 scenarios
- 403 Forbidden: 11 scenarios (auth + ACL)
- 404 Not Found: 3 scenarios
- 405 Method Not Allowed: 6 scenarios
- 410 Gone: 1 scenario
- 412 Precondition Failed: 1 scenario
- 500 Internal Server Error: 20+ scenarios
- 503 Service Unavailable: 6 scenarios

**By Response Format:**
- XML (S3-compatible): ~35 scenarios
- Plain text: ~15 scenarios
- JSON: ~10 scenarios

**Error Codes:**
- S3 error codes: 15+ (MissingAuthenticationToken, InvalidAccessKeyId, SignatureDoesNotMatch, etc.)
- Internal error codes: 1 (InternalError with context-specific messages)
- No code (plain text/JSON): ~25 scenarios
