# ARMOR Admin Endpoint Error Response Headers Specification

## Overview

This document catalogs all error response headers returned by ARMOR's administrative and management endpoints, including response formats, status codes, and inconsistencies between different endpoint types.

## Endpoint Categories

ARMOR admin endpoints fall into three categories based on response format:

1. **JSON Endpoints** - Most admin endpoints return JSON responses
2. **Plain Text Endpoints** - Health endpoints return plain text
3. **Mixed Format Endpoints** - Some endpoints return different formats based on success/error

## Summary of Admin Endpoints

| Endpoint | Methods | Response Format | Purpose |
|----------|---------|-----------------|---------|
| `/healthz` | GET | Plain Text | Liveness probe |
| `/admin/key/verify` | GET | JSON | Verify MEK correctness |
| `/admin/key/rotate` | POST | JSON | Rotate master encryption key |
| `/admin/key/export` | GET | JSON | Export current MEK |
| `/admin/presign` | POST | JSON | Generate pre-signed URL |
| `/admin/b2/keys` | GET, POST | JSON | List/create B2 keys |
| `/admin/b2/keys/{id}` | DELETE | Plain/JSON | Delete B2 key |
| `/armor/canary` | GET | JSON | Canary status |
| `/armor/audit` | GET | JSON | Audit status |
| `/metrics` | GET | Plain Text | Prometheus metrics |

## Detailed Endpoint Documentation

### 1. Health Check Endpoint

#### `/healthz` - Liveness Probe

**Method:** GET

**Success Response (200 OK):**
```
Status: 200 OK
Content-Type: text/plain
Content-Length: 2

OK
```

**Headers:**
- No special headers
- Returns plain text "OK"

**Error Responses:** None (always returns 200)

---

### 2. Key Management Endpoints

#### `/admin/key/verify` - Verify MEK

**Method:** GET

**Success Response (200 OK):**
```json
Status: 200 OK
Content-Type: application/json

{
  "status": "verified",
  "message": "MEK is correct"
}
```

**Alternative Success (200 OK - when canary not configured):**
```json
Status: 200 OK
Content-Type: application/json

{
  "status": "unknown",
  "error": "canary monitor not configured"
}
```

**Error Response (503 Service Unavailable):**
```json
Status: 503 Service Unavailable
Content-Type: application/json

{
  "status": "unverified",
  "error": "canary check failed - MEK may be incorrect"
}
```

**Error Response (405 Method Not Allowed):**
```
Status: 405 Method Not Allowed
Content-Type: text/plain

Method not allowed
```

**Headers:**
- Success: `Content-Type: application/json`
- Error (405): `Content-Type: text/plain` ⚠️ **INCONSISTENT**

**Code Reference:** `/home/coding/ARMOR/internal/server/server.go:486-507`

---

#### `/admin/key/rotate` - Rotate Master Encryption Key

**Method:** POST

**Success Response (200 OK):**
```json
Status: 200 OK
Content-Type: application/json

{
  "status": "completed",
  "rotated_objects": 123,
  "failed_objects": 0,
  "duration_ms": 45678,
  "started_at": "2024-01-01T00:00:00Z",
  "completed_at": "2024-01-01T00:00:45Z"
}
```

**Error Response (400 Bad Request - Body Read Error):**
```
Status: 400 Bad Request
Content-Type: text/plain

Failed to read request body: <error details>
```

**Error Response (400 Bad Request - Invalid Hex):**
```
Status: 400 Bad Request
Content-Type: text/plain

Invalid hex-encoded MEK
```

**Error Response (400 Bad Request - Invalid Length):**
```
Status: 400 Bad Request
Content-Type: text/plain

Invalid MEK length: expected 32 bytes or 64 hex chars, got <actual length>
```

**Error Response (405 Method Not Allowed):**
```
Status: 405 Method Not Allowed
Content-Type: text/plain

Method not allowed
```

**Error Response (500 Internal Server Error):**
```json
Status: 500 Internal Server Error
Content-Type: application/json

{
  "status": "failed",
  "error": "<error message>",
  "result": {
    "rotated_objects": 10,
    "failed_objects": 1,
    ...
  }
}
```

**Headers:**
- Success: `Content-Type: application/json`
- Errors (400/405): `Content-Type: text/plain` ⚠️ **INCONSISTENT**
- Server Error (500): `Content-Type: application/json`

**Code Reference:** `/home/coding/ARMOR/internal/server/server.go:510-573`

---

#### `/admin/key/export` - Export Current MEK

**Method:** GET

**Error Response (400 Bad Request - Missing Confirmation):**
```
Status: 400 Bad Request
Content-Type: text/plain

Must include ?confirm=yes to export key
```

**Error Response (405 Method Not Allowed):**
```
Status: 405 Method Not Allowed
Content-Type: text/plain

Method not allowed
```

**Success Response (200 OK):**
```json
Status: 200 OK
Content-Type: application/json

{
  "mek": "64-char-hex-encoded-key",
  "format": "hex",
  "warning": "This key provides access to all encrypted data. Store securely."
}
```

**Headers:**
- Success: `Content-Type: application/json`
- Errors: `Content-Type: text/plain` ⚠️ **INCONSISTENT**

**Code Reference:** `/home/coding/ARMOR/internal/server/server.go:576-596`

---

### 3. Pre-signed URL Endpoint

#### `/admin/presign` - Generate Pre-signed URL

**Method:** POST

**Request Body:**
```json
{
  "bucket": "my-bucket",           // Optional, defaults to configured bucket
  "key": "path/to/file.parquet",   // Required
  "expires_in": "1h",              // Optional, defaults to 1h
  "content_disposition": "...",    // Optional
  "range": "bytes=0-1023"          // Optional
}
```

**Success Response (200 OK):**
```json
Status: 200 OK
Content-Type: application/json

{
  "url": "https://...",
  "expires_in": "1h",
  "expires_at": "2024-01-01T01:00:00Z"
}
```

**Error Response (405 Method Not Allowed):**
```
Status: 405 Method Not Allowed
Content-Type: text/plain

Method not allowed
```

**Error Response (403 Forbidden - Auth Error):**
```xml
Status: 403 Forbidden
Content-Type: application/xml

<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>AccessDenied</Code>
  <Message>Invalid credentials</Message>
</Error>
```

**Error Response (403 Forbidden - ACL Denied):**
```xml
Status: 403 Forbidden
Content-Type: application/xml

<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>AccessDenied</Code>
  <Message>Access Denied</Message>
</Error>
```

**Error Response (400 Bad Request - Invalid Body):**
```
Status: 400 Bad Request
Content-Type: text/plain

Invalid request body: <error details>
```

**Error Response (400 Bad Request - Missing Key):**
```
Status: 400 Bad Request
Content-Type: text/plain

key is required
```

**Error Response (400 Bad Request - Invalid Expiration):**
```
Status: 400 Bad Request
Content-Type: text/plain

Invalid expires_in: <error details>
```

**Error Response (500 Internal Server Error):**
```
Status: 500 Internal Server Error
Content-Type: text/plain

Failed to generate URL: <error details>
```

**Headers:**
- Success: `Content-Type: application/json`
- Auth Errors (403): `Content-Type: application/xml` ⚠️ **INCONSISTENT**
- Other Errors: `Content-Type: text/plain` ⚠️ **INCONSISTENT**

**Code Reference:** `/home/coding/ARMOR/internal/server/server.go:828-913`

---

### 4. B2 Key Management Endpoints

#### `/admin/b2/keys` - List/Create B2 Keys

**Methods:** GET (list), POST (create)

**GET - List Keys:**

**Success Response (200 OK):**
```json
Status: 200 OK
Content-Type: application/json

{
  "keys": [
    {
      "id": "keyId1",
      "name": "key-name",
      "capabilities": ["readFiles", "writeFiles"],
      "key_id": "appId_keyId",
      "secret_key": "appKey...",
      "expires_at": "2024-01-01T01:00:00Z"
    }
  ],
  "next_cursor": "cursor-for-next-page"
}
```

**Error Response (503 Service Unavailable):**
```
Status: 503 Service Unavailable
Content-Type: text/plain

{"error":"B2 key management not available - check B2 credentials"}
```

**Error Response (500 Internal Server Error):**
```
Status: 500 Internal Server Error
Content-Type: text/plain

{"error":"Failed to list keys: <error details>"}
```

**POST - Create Key:**

**Request Body:**
```json
{
  "name": "key-name",
  "capabilities": ["readFiles", "writeFiles"],
  "valid_duration_seconds": 3600
}
```

**Success Response (201 Created):**
```json
Status: 201 Created
Content-Type: application/json

{
  "id": "keyId1",
  "name": "key-name",
  "capabilities": ["readFiles", "writeFiles"],
  "key_id": "appId_keyId",
  "secret_key": "appKey...",
  "expires_at": "2024-01-01T01:00:00Z"
}
```

**Error Response (503 Service Unavailable):**
```
Status: 503 Service Unavailable
Content-Type: text/plain

{"error":"B2 key management not available - check B2 credentials"}
```

**Error Response (400 Bad Request - Invalid Body):**
```
Status: 400 Bad Request
Content-Type: text/plain

{"error":"Invalid request body: <error details>"}
```

**Error Response (400 Bad Request - Missing Name):**
```
Status: 400 Bad Request
Content-Type: text/plain

{"error":"name is required"}
```

**Error Response (400 Bad Request - Missing Capabilities):**
```
Status: 400 Bad Request
Content-Type: text/plain

{"error":"capabilities is required"}
```

**Error Response (405 Method Not Allowed):**
```
Status: 405 Method Not Allowed
Content-Type: text/plain

Method not allowed
```

**Error Response (500 Internal Server Error):**
```
Status: 500 Internal Server Error
Content-Type: text/plain

{"error":"Failed to create key: <error details>"}
```

**Headers:**
- Success: `Content-Type: application/json`
- Errors: `Content-Type: text/plain` (but JSON body) ⚠️ **INCONSISTENT**
- Note: Error responses claim to be plain text but contain JSON

**Code Reference:** `/home/coding/ARMOR/internal/server/server.go:1246-1324`

---

#### `/admin/b2/keys/{id}` - Delete B2 Key

**Method:** DELETE

**Success Response (204 No Content):**
```
Status: 204 No Content
```

**Error Response (503 Service Unavailable):**
```
Status: 503 Service Unavailable
Content-Type: text/plain

{"error":"B2 key management not available - check B2 credentials"}
```

**Error Response (405 Method Not Allowed):**
```
Status: 405 Method Not Allowed
Content-Type: text/plain

Method not allowed
```

**Error Response (400 Bad Request - Missing Key ID):**
```
Status: 400 Bad Request
Content-Type: text/plain

{"error":"key ID is required"}
```

**Error Response (404 Not Found):**
```
Status: 404 Not Found
Content-Type: text/plain

{"error":"key not found"}
```

**Error Response (500 Internal Server Error):**
```
Status: 500 Internal Server Error
Content-Type: text/plain

{"error":"Failed to delete key: <error details>"}
```

**Headers:**
- Success: No headers (204)
- Errors: `Content-Type: text/plain` (but JSON body) ⚠️ **INCONSISTENT**
- Note: Error responses claim to be plain text but contain JSON

**Code Reference:** `/home/coding/ARMOR/internal/server/server.go:1326-1364`

---

### 5. ARMOR Status Endpoints

#### `/armor/canary` - Canary Status

**Method:** GET

**Success Response (200 OK - with canary):**
```json
Status: 200 OK
Content-Type: application/json

{
  "decrypt_verified": true,
  "hmac_verified": true,
  "last_check": "2024-01-01T00:00:00Z",
  "error": ""
}
```

**Success Response (200 OK - without canary):**
```json
Status: 200 OK
Content-Type: application/json

{
  "status": "unknown",
  "error": "canary monitor not configured"
}
```

**Error Response (405 Method Not Allowed):**
```
Status: 405 Method Not Allowed
Content-Type: text/plain

Method not allowed
```

**Headers:**
- Success: `Content-Type: application/json`
- Error: `Content-Type: text/plain` ⚠️ **INCONSISTENT**

**Code Reference:** `/home/coding/ARMOR/internal/server/server.go:599-615`

---

#### `/armor/audit` - Audit Status

**Method:** GET

**Success Response (200 OK):**
```json
Status: 200 OK
Content-Type: application/json

{
  "total_objects": 1234,
  "verified_objects": 1230,
  "failed_objects": 4,
  "errors": ["error1", "error2"]
}
```

**Error Response (405 Method Not Allowed):**
```
Status: 405 Method Not Allowed
Content-Type: text/plain

Method not allowed
```

**Error Response (500 Internal Server Error):**
```json
Status: 500 Internal Server Error
Content-Type: application/json

{
  "status": "error",
  "error": "<error details>"
}
```

**Headers:**
- Success: `Content-Type: application/json`
- Error (405): `Content-Type: text/plain` ⚠️ **INCONSISTENT**
- Server Error (500): `Content-Type: application/json`

**Code Reference:** `/home/coding/ARMOR/internal/server/server.go:617-639`

---

### 6. Metrics Endpoint

#### `/metrics` - Prometheus Metrics

**Method:** GET

**Success Response (200 OK):**
```
Status: 200 OK
Content-Type: text/plain

# HELP armor_requests_total Total number of requests
# TYPE armor_requests_total counter
armor_requests_total{method="GET",status="200"} 1234
...
```

**Headers:**
- `Content-Type: text/plain` (Prometheus text format)

---

## Inconsistencies Found

### 1. Content-Type Header Mismatches

**Issue:** Many endpoints return JSON responses but declare `Content-Type: text/plain` in error cases.

| Endpoint | Problem |
|----------|---------|
| `/admin/b2/keys` | Error responses return JSON but declare `text/plain` |
| `/admin/b2/keys/{id}` | Error responses return JSON but declare `text/plain` |
| `/admin/presign` | Auth errors use XML format, other errors use plain text |

### 2. Format Inconsistency Within Endpoints

**Issue:** Some endpoints use different response formats for success vs error, or for different error types.

| Endpoint | Success Format | Error Format | Issue |
|----------|---------------|---------------|-------|
| `/admin/key/verify` | JSON | JSON | Consistent ✅ |
| `/admin/key/rotate` | JSON | Plain text | ⚠️ Inconsistent |
| `/admin/key/export` | JSON | Plain text | ⚠️ Inconsistent |
| `/admin/presign` | JSON | XML/Plain text | ⚠️ Multiple formats |
| `/admin/b2/keys` | JSON | JSON in Plain | ⚠️ Type mismatch |
| `/armor/canary` | JSON | Plain text | ⚠️ Inconsistent |
| `/armor/audit` | JSON | Mixed | ⚠️ Inconsistent |

### 3. HTTP Method Not Allowed Handling

**Issue:** 405 Method Not Allowed responses consistently use `text/plain` format across all endpoints, even when the endpoint normally returns JSON.

**Pattern:**
```
Status: 405 Method Not Allowed
Content-Type: text/plain

Method not allowed
```

This is handled by Go's `http.Error()` function, which doesn't allow specifying content type or structured responses.

### 4. Authentication Error Handling

**Issue:** `/admin/presign` uses S3 XML error format for authentication failures, while other admin endpoints would use plain text or JSON.

**Admin endpoint auth errors:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>AccessDenied</Code>
  <Message>Invalid credentials</Message>
</Error>
```

This is because `/admin/presign` calls `s.writeError()` which uses the S3 error format, while other endpoints call `http.Error()` or return JSON directly.

### 5. Error Response Structure Variations

**JSON Error Responses:**
```json
{"error": "error message"}
```
```json
{"status": "error", "error": "error message"}
```
```json
{"status": "failed", "error": "error message", "result": {...}}
```

**Plain Text Errors:**
```
Error message
```
```
Failed to <operation>: <details>
```

**XML Errors:**
```xml
<Error><Code>AccessDenied</Code><Message>...</Message></Error>
```

## Response Format Summary

### JSON Endpoints (Consistent)

These endpoints consistently return JSON for both success and errors:
- ✅ `/admin/key/verify` - All responses use JSON
- ✅ `/armor/audit` - Server errors use JSON (405 is plain text)

### Mixed Format Endpoints (Inconsistent)

These endpoints mix formats:
- ⚠️ `/admin/key/rotate` - JSON success, plain text errors
- ⚠️ `/admin/key/export` - JSON success, plain text errors  
- ⚠️ `/admin/presign` - JSON success, XML auth errors, plain text other errors
- ⚠️ `/admin/b2/keys` - JSON success, JSON-in-plain-text errors
- ⚠️ `/admin/b2/keys/{id}` - JSON-in-plain-text errors
- ⚠️ `/armor/canary` - JSON success, plain text method not allowed

### Plain Text Endpoints (Consistent)

- ✅ `/healthz` - Always plain text
- ✅ `/readyz` - Always plain text
- ✅ `/metrics` - Always plain text

## Recommendations

### 1. Standardize Error Response Format

**Option A:** All admin endpoints return JSON
```json
{
  "error": "error message",
  "code": "ErrorCode",
  "details": {...}
}
```

**Option B:** Use structured error responses
```json
{
  "status": "error",
  "error": {
    "code": "ErrorCode",
    "message": "error message",
    "details": {...}
  }
}
```

### 2. Fix Content-Type Headers

Ensure `Content-Type` headers match the actual response body format:
- JSON responses should declare `Content-Type: application/json`
- Plain text responses should declare `Content-Type: text/plain`
- XML responses should declare `Content-Type: application/xml`

### 3. Unify Authentication Error Handling

Choose one format for authentication errors across all admin endpoints:
- Use JSON (recommended for consistency)
- Or use XML (for S3 compatibility, but inconsistent with admin API)

### 4. Create Structured Error Types

Define error types that can be consistently serialized:
```go
type AdminError struct {
    Code    string `json:"code"`
    Message string `json:"message"`
    Details map[string]interface{} `json:"details,omitempty"`
}

func (e *AdminError) Error() string {
    return e.Message
}

func writeAdminError(w http.ResponseWriter, err *AdminError, status int) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(err)
}
```

### 5. Update Method Not Allowed Responses

Replace generic 405 responses with structured JSON:
```json
{
  "error": "Method not allowed",
  "code": "MethodNotAllowed",
  "allowed_methods": ["GET", "POST"]
}
```

## Testing

To verify admin endpoint error responses:

```bash
# Test 405 Method Not Allowed
curl -i -X POST /admin/key/verify

# Test 400 Bad Request
curl -i -X POST /admin/presign -d "{}"

# Test 403 Authentication Error
curl -i -X POST /admin/presign -d '{"key":"test"}' \
  -H "Authorization: invalid"

# Test 503 Service Unavailable
# (Requires misconfiguring B2 credentials)

# Test JSON format in text/plain content type
curl -i -X POST /admin/b2/keys -d "{}"
```

## References

- ARMOR Source Code: `/home/coding/ARMOR/internal/server/server.go`
- ARMOR Source Code: `/home/coding/ARMOR/internal/b2keys/b2keys.go`
- Related Documentation: `/home/coding/ARMOR/docs/error-response-headers-specification.md`
- Related Documentation: `/home/coding/ARMOR/docs/auth-rejection-headers.md`

## Changelog

- **2024-01-14** - Initial documentation created for bead bf-a5evuz
- Covers all admin endpoints in `/home/coding/ARMOR/internal/server/server.go`
- Identified 5 major inconsistency categories
- Documented response formats and headers for all endpoints