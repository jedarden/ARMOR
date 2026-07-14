# Admin Endpoint Error Response Headers Specification

## Overview

This document catalogs all error response headers returned by ARMOR's administrative and management endpoints, including response formats, status codes, and inconsistencies between endpoints.

## Scope

This document covers the following admin endpoints:
- `/admin/b2/keys/*` - B2 key management (list, create, delete)
- `/admin/key/*` - Key operations (verify, rotate, export)
- `/admin/presign` - Pre-signed URL generation
- `/share/*` - Pre-signed URL access

## Response Format Summary

| Endpoint | Success Format | Error Format | Headers on Error |
|----------|----------------|---------------|-------------------|
| `/admin/b2/keys` | JSON | JSON (plain text) | Varies |
| `/admin/key/*` | JSON | JSON (plain text) | None |
| `/admin/presign` | JSON | JSON (plain text) | None |
| `/share/*` | Binary | Plain text | None |

**Key Finding:** Admin endpoints use **plain text error responses** with `http.Error()`, unlike S3 endpoints which use XML format. No admin endpoint sets explicit error response headers.

---

## `/admin/b2/keys` Endpoint

### `GET /admin/b2/keys` - List B2 Keys

#### Error Responses

| HTTP Status | Error Scenario | Response Body | Response Headers |
|-------------|-----------------|---------------|------------------|
| 503 Service Unavailable | B2 key management not available | `B2 key management not available - check B2 credentials` | None |
| 500 Internal Server Error | Failed to list keys (backend error) | `{"error":"Failed to list keys: {error}"}` | None |

**Headers on success:**
- `Content-Type: application/json`

**Implementation:**
```go
// Line 1250-1252
if s.b2keys == nil {
    http.Error(w, `{"error":"B2 key management not available - check B2 credentials"}`, http.StatusServiceUnavailable)
    return
}

// Line 1277-1281
if err != nil {
    s.logger.WithFields(map[string]interface{}{
        "error": err.Error(),
    }).Error("Failed to list B2 keys")
    http.Error(w, fmt.Sprintf(`{"error":"Failed to list keys: %v"}`, err), http.StatusInternalServerError)
    return
}
```

### `POST /admin/b2/keys` - Create B2 Key

#### Error Responses

| HTTP Status | Error Scenario | Response Body | Response Headers |
|-------------|-----------------|---------------|------------------|
| 503 Service Unavailable | B2 key management not available | `B2 key management not available - check B2 credentials` | None |
| 400 Bad Request | Invalid request body (malformed JSON) | `{"error":"Invalid request body: {error}"}` | None |
| 400 Bad Request | Missing name field | `{"error":"name is required"}` | None |
| 400 Bad Request | Missing capabilities field | `{"error":"capabilities is required"}` | None |
| 500 Internal Server Error | Failed to create key (backend error) | `{"error":"Failed to create key: {error}"}` | None |

**Headers on success (201):**
- `Content-Type: application/json`

**Implementation:**
```go
// Line 1291-1294
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
    http.Error(w, fmt.Sprintf(`{"error":"Invalid request body: %v"}`, err), http.StatusBadRequest)
    return
}

// Line 1296-1299
if req.Name == "" {
    http.Error(w, `{"error":"name is required"}`, http.StatusBadRequest)
    return
}

// Line 1301-1304
if len(req.Capabilities) == 0 {
    http.Error(w, `{"error":"capabilities is required"}`, http.StatusBadRequest)
    return
}

// Line 1307-1314
if err != nil {
    s.logger.WithFields(map[string]interface{}{
        "error": err.Error(),
        "name":  req.Name,
    }).Error("Failed to create B2 key")
    http.Error(w, fmt.Sprintf(`{"error":"Failed to create key: %v"}`, err), http.StatusInternalServerError)
    return
}
```

### `DELETE /admin/b2/keys/{id}` - Delete B2 Key

#### Error Responses

| HTTP Status | Error Scenario | Response Body | Response Headers |
|-------------|-----------------|---------------|------------------|
| 503 Service Unavailable | B2 key management not available | `B2 key management not available - check B2 credentials` | None |
| 405 Method Not Allowed | Non-DELETE method | `Method not allowed` | None |
| 400 Bad Request | Missing key ID in path | `{"error":"key ID is required"}` | None |
| 404 Not Found | Key not found | `{"error":"key not found"}` | None |
| 500 Internal Server Error | Failed to delete key (backend error) | `{"error":"Failed to delete key: {error}"}` | None |

**Headers on success (204):**
- None

**Implementation:**
```go
// Line 1328-1331
if s.b2keys == nil {
    http.Error(w, `{"error":"B2 key management not available - check B2 credentials"}`, http.StatusServiceUnavailable)
    return
}

// Line 1333-1336
if r.Method != http.MethodDelete {
    http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    return
}

// Line 1339-1343
keyID := strings.TrimPrefix(r.URL.Path, "/admin/b2/keys/")
if keyID == "" || keyID == r.URL.Path {
    http.Error(w, `{"error":"key ID is required"}`, http.StatusBadRequest)
    return
}

// Line 1346-1350
err := s.b2keys.DeleteKey(r.Context(), keyID)
if err != nil {
    if errors.Is(err, b2keys.ErrKeyNotFound) {
        http.Error(w, `{"error":"key not found"}`, http.StatusNotFound)
        return
    }
    // ...
}

// Line 1351-1357
s.logger.WithFields(map[string]interface{}{
    "error":  err.Error(),
    "key_id": keyID,
}).Error("Failed to delete B2 key")
http.Error(w, fmt.Sprintf(`{"error":"Failed to delete key: %v"}`, err), http.StatusInternalServerError)
return
```

---

## `/admin/key/*` Endpoints

### `GET /admin/key/verify` - Verify Master Encryption Key

#### Error Responses

| HTTP Status | Error Scenario | Response Body | Response Headers |
|-------------|-----------------|---------------|------------------|
| 405 Method Not Allowed | Non-GET method | `Method not allowed` | None |

**Headers on success (200 or 503):**
- None

**Success Responses:**

| HTTP Status | Response Body |
|-------------|---------------|
| 200 OK | `{"status":"verified","message":"MEK is correct"}` |
| 200 OK | `{"status":"unknown","error":"canary monitor not configured"}` |
| 503 Service Unavailable | `{"status":"unverified","error":"canary check failed - MEK may be incorrect"}` |

**Implementation:**
```go
// Line 487-490
if r.Method != http.MethodGet {
    http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    return
}

// Line 492-496
if s.canary == nil {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte(`{"status":"unknown","error":"canary monitor not configured"}`))
    return
}

// Line 498-503
status := s.canary.GetStatus()
if status.DecryptVerified && status.HMACVerified {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte(`{"status":"verified","message":"MEK is correct"}`))
    return
}

// Line 505-506
w.WriteHeader(http.StatusServiceUnavailable)
w.Write([]byte(`{"status":"unverified","error":"canary check failed - MEK may be incorrect"}`))
```

### `POST /admin/key/rotate` - Rotate Master Encryption Key

#### Error Responses

| HTTP Status | Error Scenario | Response Body | Response Headers |
|-------------|-----------------|---------------|------------------|
| 405 Method Not Allowed | Non-POST method | `Method not allowed` | None |
| 400 Bad Request | Failed to read request body | `Failed to read request body: {error}` | None |
| 400 Bad Request | Invalid hex-encoded MEK | `Invalid hex-encoded MEK` | None |
| 400 Bad Request | Invalid MEK length | `Invalid MEK length: expected 32 bytes or 64 hex chars, got {n}` | None |
| 500 Internal Server Error | Rotation failed | JSON body with status/error/result fields | None |

**Headers on success (200):**
- `Content-Type: application/json`

**Implementation:**
```go
// Line 511-514
if r.Method != http.MethodPost {
    http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    return
}

// Line 517-521
body, err := io.ReadAll(r.Body)
if err != nil {
    http.Error(w, fmt.Sprintf("Failed to read request body: %v", err), http.StatusBadRequest)
    return
}

// Line 529-533
newMEK, err = hex.DecodeString(string(body))
if err != nil {
    http.Error(w, "Invalid hex-encoded MEK", http.StatusBadRequest)
    return
}

// Line 537-540
} else {
    http.Error(w, fmt.Sprintf("Invalid MEK length: expected 32 bytes or 64 hex chars, got %d", len(body)), http.StatusBadRequest)
    return
}

// Line 548-557
result, err := rotator.Rotate(r.Context())
if err != nil {
    w.WriteHeader(http.StatusInternalServerError)
    json.NewEncoder(w).Encode(map[string]interface{}{
        "status":  "failed",
        "error":   err.Error(),
        "result":  result,
    })
    return
}
```

### `GET /admin/key/export` - Export Master Encryption Key

#### Error Responses

| HTTP Status | Error Scenario | Response Body | Response Headers |
|-------------|-----------------|---------------|------------------|
| 405 Method Not Allowed | Non-GET method | `Method not allowed` | None |
| 400 Bad Request | Missing ?confirm=yes query parameter | `Must include ?confirm=yes to export key` | None |

**Headers on success (200):**
- `Content-Type: application/json`

**Implementation:**
```go
// Line 577-580
if r.Method != http.MethodGet {
    http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    return
}

// Line 582-585
if r.URL.Query().Get("confirm") != "yes" {
    http.Error(w, "Must include ?confirm=yes to export key", http.StatusBadRequest)
    return
}
```

---

## `/admin/presign` Endpoint

### `POST /admin/presign` - Generate Pre-signed URL

#### Error Responses

| HTTP Status | Error Scenario | Response Body | Response Headers |
|-------------|-----------------|---------------|------------------|
| 405 Method Not Allowed | Non-POST method | `Method not allowed` | None |
| 403 Forbidden | Authentication error (AuthError) | XML Error format | `Content-Type: application/xml` |
| 403 Forbidden | Invalid credentials (generic) | XML Error format | `Content-Type: application/xml` |
| 403 Forbidden | ACL check failed | XML Error format | `Content-Type: application/xml` |
| 400 Bad Request | Invalid request body (malformed JSON) | `Invalid request body: {error}` | None |
| 400 Bad Request | Key field is required | `key is required` | None |
| 400 Bad Request | Invalid expires_in format | `Invalid expires_in: {error}` | None |
| 500 Internal Server Error | Failed to generate URL | `Failed to generate URL: {error}` | None |

**Headers on success (200):**
- `Content-Type: application/json`

**Implementation:**
```go
// Line 831-834
if r.Method != http.MethodPost {
    http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    return
}

// Line 837-846
cred, err := s.verifyAuthAndGetCredential(r)
if err != nil {
    // Return specific authentication error code and message
    if authErr, ok := err.(*AuthError); ok {
        s.writeError(w, authErr.Code, authErr.Message, 403)
    } else {
        s.writeError(w, "AccessDenied", "Invalid credentials", 403)
    }
    return
}

// Line 857-860
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
    http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
    return
}

// Line 869-872
if req.Key == "" {
    http.Error(w, "key is required", http.StatusBadRequest)
    return
}

// Line 875-878
if err := CheckACL(cred, bucket, req.Key); err != nil {
    s.writeError(w, "AccessDenied", "Access Denied", 403)
    return
}

// Line 883-887
expiration, err = presign.ParseExpiration(req.ExpiresIn)
if err != nil {
    http.Error(w, fmt.Sprintf("Invalid expires_in: %v", err), http.StatusBadRequest)
    return
}

// Line 901-904
shareURL, err := s.presigner.GenerateURL(bucket, req.Key, expiration, opts...)
if err != nil {
    http.Error(w, fmt.Sprintf("Failed to generate URL: %v", err), http.StatusInternalServerError)
    return
}
```

---

## `/share/*` Endpoint

### `GET /share/<token>` - Access Pre-signed URL

#### Error Responses

| HTTP Status | Error Scenario | Response Body | Response Headers |
|-------------|-----------------|---------------|------------------|
| 405 Method Not Allowed | Non-GET method | `Method not allowed` | None |
| 400 Bad Request | Missing token in path | `Missing token` | None |
| 400 Bad Request | Invalid token (general) | `Invalid token` | None |
| 410 Gone | Link expired | `Link expired` | None |
| 403 Forbidden | Invalid link signature | `Invalid link` | None |
| 404 Not Found | Object not found | `Object not found: {error}` | None |
| 500 Internal Server Error | Failed to get object | `Failed to get object: {error}` | None |
| 500 Internal Server Error | Failed to parse ARMOR metadata | `Failed to parse object metadata` | None |
| 500 Internal Server Error | Failed to get decryption key | `Failed to get decryption key` | None |
| 500 Internal Server Error | Failed to unwrap DEK | `Failed to unwrap DEK` | None |
| 500 Internal Server Error | Failed to create decryptor | `Failed to create decryptor` | None |

**Headers on success (200 or 206):**
- `Content-Length`
- `Content-Type`
- `Accept-Ranges: bytes`
- `Content-Disposition` (if specified in token)
- `Content-Range` (for partial content 206)

**Implementation:**
```go
// Line 918-921
if r.Method != http.MethodGet {
    http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    return
}

// Line 924-928
tokenStr := presign.ParseTokenFromPath(r.URL.Path)
if tokenStr == "" {
    http.Error(w, "Missing token", http.StatusBadRequest)
    return
}

// Line 931-943
token, err := s.presigner.VerifyToken(tokenStr)
if err != nil {
    if errors.Is(err, presign.ErrExpiredToken) {
        http.Error(w, "Link expired", http.StatusGone)
        return
    }
    if errors.Is(err, presign.ErrInvalidSignature) {
        http.Error(w, "Invalid link", http.StatusForbidden)
        return
    }
    http.Error(w, "Invalid token", http.StatusBadRequest)
    return
}

// Line 948-951
info, err := s.backend.Head(ctx, token.Bucket, token.Key)
if err != nil {
    http.Error(w, fmt.Sprintf("Object not found: %v", err), http.StatusNotFound)
    return
}

// Line 974-978
armorMeta, ok := backend.ParseARMORMetadata(info.Metadata)
if !ok {
    http.Error(w, "Failed to parse object metadata", http.StatusInternalServerError)
    return
}

// Line 980-985
mek, err := s.keyManager.GetMEKByID(armorMeta.KeyID)
if err != nil {
    http.Error(w, "Failed to get decryption key", http.StatusInternalServerError)
    return
}

// Line 987-992
dek, err := crypto.UnwrapDEK(mek, armorMeta.WrappedDEK)
if err != nil {
    http.Error(w, "Failed to unwrap DEK", http.StatusInternalServerError)
    return
}

// Line 994-999
decryptor, err := crypto.NewDecryptor(dek, armorMeta.IV, armorMeta.BlockSize)
if err != nil {
    http.Error(w, "Failed to create decryptor", http.StatusInternalServerError)
    return
}
```

---

## Inconsistencies Summary

### Response Format Inconsistencies

| Endpoint | Success Format | Error Format | Inconsistency |
|----------|----------------|---------------|---------------|
| `/admin/b2/keys/*` | JSON | JSON (503) or plain text (other errors) | ⚠️ Mixed: JSON and plain text |
| `/admin/key/verify` | JSON | JSON | ✅ Consistent |
| `/admin/key/rotate` | JSON | JSON or plain text | ⚠️ Mixed: JSON (500) and plain text (400, 405) |
| `/admin/key/export` | JSON | Plain text | ⚠️ Plain text errors |
| `/admin/presign` | JSON | XML (403 auth) or plain text (other errors) | ⚠️ Mixed: XML, plain text, JSON |
| `/share/*` | Binary | Plain text | ✅ Consistent |

### Error Response Header Inconsistencies

| Endpoint | Error Headers | Inconsistency |
|----------|---------------|---------------|
| `/admin/b2/keys/*` | None | ✅ Consistent |
| `/admin/key/*` | None | ✅ Consistent |
| `/admin/presign` | `Content-Type: application/xml` (auth errors only) | ⚠️ Auth uses XML headers, other errors have no headers |
| `/share/*` | None | ✅ Consistent |

### HTTP Status Code Inconsistencies

**Service Unavailable (503) Usage:**
- `/admin/b2/keys/*`: Used when B2 client not initialized
- `/admin/key/verify`: Used when canary check fails (indicates MEK may be incorrect)
- **Inconsistency:** Same status code for different conditions (unavailable service vs. service available but unverified)

**Gone (410) Usage:**
- `/share/*`: Used for expired pre-signed links
- **Unique:** Only endpoint using 410 status code

---

## Compliance Notes

### S3 Compliance

Admin endpoints are **not S3-facing** and therefore do not need to comply with AWS S3 specifications. However:

⚠️ **Partial Issues:**
- `/admin/presign` mixes S3 XML error format (for auth errors) with plain text errors (for validation errors)
- This inconsistency could confuse clients that expect a single error format

### Recommendations

1. **Standardize Error Format:** Consider using a single error format (JSON or plain text) across all admin endpoints
   - JSON is preferred for machine readability
   - Plain text is acceptable for human-facing admin endpoints

2. **Add Error Headers:** Consider adding `Content-Type` headers to all error responses for consistency

3. **Document Error Conditions:** Ensure all error scenarios are documented with their corresponding status codes and response bodies

4. **Consider Structured Errors:** For `/admin/presign`, choose either S3 XML format or plain text, not both

---

## Testing

To verify admin endpoint error responses:

```bash
# 503 B2 key management not available
curl -i http://localhost:8080/admin/b2/keys

# 400 Invalid request body
curl -i -X POST -H "Content-Type: application/json" \
  -d '{"invalid json"' \
  http://localhost:8080/admin/b2/keys

# 400 Missing name
curl -i -X POST -H "Content-Type: application/json" \
  -d '{"capabilities":["readFiles"]}' \
  http://localhost:8080/admin/b2/keys

# 404 Key not found
curl -i -X DELETE http://localhost:8080/admin/b2/keys/nonexistent

# 405 Method not allowed
curl -i -X PUT http://localhost:8080/admin/key/verify

# 400 Invalid expires_in
curl -i -X POST -H "Content-Type: application/json" \
  -d '{"key":"test","expires_in":"invalid"}' \
  http://localhost:8080/admin/presign

# 400 Missing confirm
curl -i http://localhost:8080/admin/key/export

# 400 Missing token
curl -i http://localhost:8080/share/

# 410 Link expired
# (requires creating an expired token)

# 403 Invalid link
# (requires creating a token with invalid signature)
```

---

## References

- ARMOR Source Code: `/home/coding/ARMOR/internal/server/server.go`
- S3 Endpoint Error Headers: `/home/coding/ARMOR/docs/error-response-headers-specification.md`
- Error Response Consistency: `/home/coding/ARMOR/docs/error-response-header-consistency.md`
