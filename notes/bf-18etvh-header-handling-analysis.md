# ARMOR Header Handling Architecture

**Date:** 2026-07-14  
**Task:** bf-18etvh - Examine current header handling implementation  
**Purpose:** Understand authentication header flow through the ARMOR request pipeline

## Executive Summary

ARMOR implements AWS S3-compatible authentication using AWS Signature Version 4 (SigV4). Authentication headers enter the system via HTTP requests, flow through a middleware layer, and terminate at the ARMOR server endpoint where they are verified for credential validation and ACL enforcement.

---

## 1. Header Entry Points (Ingress)

### Primary HTTP Servers

ARMOR operates two separate HTTP servers, each with distinct authentication requirements:

#### 1.1 Main S3 API Server
- **Address:** `cfg.Listen` (typically `:443` or `:8080`)
- **Handler:** `srv.Handler()` (server.go:361)
- **Routes:** All S3-compatible operations (Get, Put, Delete, Head, List, etc.)

#### 1.2 Admin API Server  
- **Address:** `cfg.AdminListen` (typically `:8081`)
- **Handler:** `srv.AdminHandler()` (server.go:391)
- **Routes:** Administrative endpoints (key management, metrics, dashboard)

### Route Registration

Both servers use `http.NewServeMux()` to register routes:

```go
// Main S3 API routes (server.go:362-387)
mux.HandleFunc("/healthz", s.healthz)           // Public - no auth
mux.HandleFunc("/readyz", s.readyz)             // Public - no auth  
mux.HandleFunc("/share/", s.handleShare)        // Public - token-based auth
mux.HandleFunc("/", s.wrapHandler(h.HandleRoot)) // Authenticated - requires SigV4

// Admin API routes (server.go:392-424)
mux.HandleFunc("/healthz", s.healthz)
mux.HandleFunc("/admin/key/verify", s.verifyKey)
mux.HandleFunc("/admin/key/rotate", s.rotateKey)
// ... additional admin routes
mux.HandleFunc("/metrics", s.metrics.Handler())
```

---

## 2. Header Flow Through Intermediate Layers

### 2.1 HTTP Request Reception

When a request arrives at the HTTP server, it follows this path:

```
HTTP Request → http.Server → Server.Handler() → ServeMux → Route Handler
```

### 2.2 Public Path Check (server.go:667-691)

The `wrapHandler()` middleware first determines if authentication is required:

```go
func (s *Server) wrapHandler(h http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // ... CORS headers added ...
        
        // Verify auth for non-public endpoints
        if !s.isPublicPath(r.URL.Path) {
            cred, err := s.verifyAuthAndGetCredential(r)
            if err != nil {
                // Return authentication error
                s.writeError(w, authErr.Code, authErr.Message, 403)
                return
            }
            
            // Check ACL for bucket/key access
            if err := CheckACL(cred, bucket, key); err != nil {
                s.writeError(w, "AccessDenied", "Access Denied", 403)
                return
            }
        }
        
        // Execute the actual handler
        h(rw, r)
    }
}
```

**Public paths (no authentication required):**
- `/healthz` - Liveness probe
- `/readyz` - Readiness probe
- `/share/*` - Public file sharing (uses token-based authentication instead)

### 2.3 Authentication Header Extraction (server.go:746-766)

The `verifyAuthAndGetCredential()` method extracts and validates authentication:

```go
func (s *Server) verifyAuthAndGetCredential(r *http.Request) (*config.Credential, error) {
    auth := NewSigV4AuthWithCredentials(s.config.Credentials, s.config.B2Region)
    
    // Check for query-based auth (presigned URLs)
    if r.URL.Query().Get("X-Amz-Credential") != "" {
        return auth.VerifyQueryAuth(r)
    }
    
    // Check for header-based auth
    authHeader := r.Header.Get("Authorization")
    if authHeader == "" {
        return nil, ErrMissingAuthHeader
    }
    
    return auth.VerifyRequest(r, nil)
}
```

### 2.4 Header Parsing (auth.go:60-112)

The `ParseAuthHeader()` function extracts components from the Authorization header:

```
Authorization: AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request, 
               SignedHeaders=host;x-amz-date, 
               Signature=fe5f80f77d5fa3beca038a248ff027d0445342fe2855ddc963176630326f1024
```

**Parsed components:**
- `Algorithm`: "AWS4-HMAC-SHA256"
- `AccessKey`: "AKIAIOSFODNN7EXAMPLE"
- `CredentialDate`: "20130524"
- `Region`: "us-east-1"
- `Service`: "s3"
- `SignedHeaders`: ["host", "x-amz-date"]
- `Signature`: "fe5f80f7..."

---

## 3. Header Transformations and Middleware

### 3.1 CORS Headers (server.go:657-660)

All requests receive CORS headers regardless of authentication status:

```go
w.Header().Set("Access-Control-Allow-Origin", "*")
w.Header().Set("Access-Control-Allow-Methods", "GET, PUT, DELETE, HEAD, POST, OPTIONS")
w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Range, Content-Length")
```

### 3.2 AWS Chunked Streaming Decoding (server.go:693-703)

For MinIO streaming signatures, the request body is wrapped to decode chunked framing:

```go
if r.Header.Get("X-Amz-Content-Sha256") == "STREAMING-AWS4-HMAC-SHA256-PAYLOAD" {
    if decoded := r.Header.Get("X-Amz-Decoded-Content-Length"); decoded != "" {
        if n, err := strconv.ParseInt(decoded, 10, 64); err == nil {
            r.ContentLength = n
        }
    }
    r.Body = newAWSChunkedReader(r.Body)
}
```

### 3.3 Response Headers

Individual handlers set response headers based on operation type:

- **GetObject:** `Content-Length`, `Content-Type`, `ETag`, `Accept-Ranges`, `Last-Modified`
- **HeadObject:** Same as GetObject but no body
- **PutObject:** `ETag` on success
- **ErrorResponse:** `Content-Type: application/xml` with error details

---

## 4. ARMOR Authentication Endpoint

### 4.1 Credential Verification (auth.go:114-165)

The `VerifyRequest()` method performs AWS SigV4 signature verification:

```go
func (a *SigV4Auth) VerifyRequest(r *http.Request, body []byte) (*config.Credential, error) {
    authHeader := r.Header.Get("Authorization")
    if authHeader == "" {
        return nil, ErrMissingAuthHeader
    }
    
    parsed, err := ParseAuthHeader(authHeader)
    if err != nil {
        return nil, err
    }
    
    // Look up credential by access key
    cred, exists := a.credentials[parsed.AccessKey]
    if !exists {
        return nil, ErrInvalidAccessKey
    }
    
    // Verify timestamp (within 15 minutes)
    amzDate := r.Header.Get("X-Amz-Date")
    if amzDate == "" {
        return nil, ErrMissingDateHeader
    }
    
    // Build canonical request and verify signature
    canonicalRequest := a.buildCanonicalRequest(r, parsed.SignedHeaders, body)
    stringToSign := a.buildStringToSign(amzDate, parsed.CredentialDate, parsed.Region, canonicalRequest)
    
    // Calculate and compare signatures
    signingKey := a.getSigningKeyForCredential(cred, parsed.CredentialDate, parsed.Region)
    calculatedSig := hex.EncodeToString(a.hmacSHA256(signingKey, stringToSign))
    
    if calculatedSig != parsed.Signature {
        return nil, ErrSignatureMismatch
    }
    
    return cred, nil
}
```

### 4.2 ACL Enforcement (auth.go:294-318)

After successful authentication, the `CheckACL()` function enforces access controls:

```go
func CheckACL(cred *config.Credential, bucket, key string) error {
    // No ACLs means full access
    if len(cred.ACLs) == 0 {
        return nil
    }
    
    for _, acl := range cred.ACLs {
        // Check bucket match
        if acl.Bucket != "*" && acl.Bucket != bucket {
            continue
        }
        
        // Check prefix match
        if acl.Prefix == "" {
            return nil  // Empty prefix means any key in the bucket
        }
        if strings.HasPrefix(key, acl.Prefix) {
            return nil
        }
    }
    
    return ErrAccessDenied
}
```

---

## 5. Special Authentication Paths

### 5.1 Query-Based Authentication (Presigned URLs)

Presigned URLs embed authentication in query parameters instead of headers:

```go
func (a *SigV4Auth) VerifyQueryAuth(r *http.Request) (*config.Credential, error) {
    query := r.URL.Query()
    
    accessKeyCred := query.Get("X-Amz-Credential")
    signature := query.Get("X-Amz-Signature")
    amzDate := query.Get("X-Amz-Date")
    signedHeadersStr := query.Get("X-Amz-SignedHeaders")
    expires := query.Get("X-Amz-Expires")
    
    // Verify signature with same logic as header-based auth
    // ...
}
```

### 5.2 Token-Based Authentication (Share Endpoint)

The `/share/` endpoint uses a custom token system instead of SigV4:

```go
func (s *Server) handleShare(w http.ResponseWriter, r *http.Request) {
    // Extract token from path
    tokenStr := presign.ParseTokenFromPath(r.URL.Path)
    
    // Verify token signature and expiration
    token, err := s.presigner.VerifyToken(tokenStr)
    if err != nil {
        http.Error(w, "Invalid token", http.StatusForbidden)
        return
    }
    
    // Serve decrypted content
    // ...
}
```

---

## 6. Key Headers in the Request Pipeline

### 6.1 Authentication Headers

| Header | Purpose | Source | Validation |
|--------|---------|--------|------------|
| `Authorization` | SigV4 signature | Client SDK/CLI | Parsed and verified in `ParseAuthHeader()` |
| `X-Amz-Date` | Request timestamp | Client SDK/CLI | Verified to be within 15 minutes |
| `X-Amz-Credential` | Presigned URL auth | Query parameter | Verified in `VerifyQueryAuth()` |
| `X-Amz-SignedHeaders` | List of signed headers | Auth header/query | Used in canonical request |
| `X-Amz-Signature` | Calculated signature | Auth header/query | Verified against calculated value |

### 6.2 Content Headers

| Header | Purpose | Handling |
|--------|---------|----------|
| `Content-Type` | Object content type | Passed through to B2 metadata |
| `Content-Length` | Request body size | Used for streaming vs buffered decision |
| `X-Amz-Content-Sha256` | Payload hash (or streaming marker) | Preferred over body hash for verification |
| `X-Amz-Decoded-Content-Length` | True size for streaming uploads | Extracted and set as `ContentLength` |

### 6.3 Response Headers

| Header | Purpose | Set By |
|--------|---------|--------|
| `ETag` | Object MD5 hash | All object write operations |
| `Content-Length` | Response body size | All object read operations |
| `Accept-Ranges` | Range request support | GetObject, HeadObject |
| `Last-Modified` | Object modification time | All object operations |
| `X-Armor-Stream` | Pipelined streaming indicator | Full object downloads |
| `X-Armor-Footer-Cache` | Parquet footer cache hit/miss | Range requests |

---

## 7. Error Response Headers

All authentication and authorization errors return S3-compatible error responses:

```go
func (s *Server) writeError(w http.ResponseWriter, code, message string, statusCode int) {
    w.Header().Set("Content-Type", "application/xml")
    w.WriteHeader(statusCode)
    fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>%s</Code>
  <Message>%s</Message>
</Error>`, code, message)
}
```

**Common authentication error codes:**
- `MissingAuthenticationToken` - No Authorization header
- `InvalidAccessKeyId` - Access key not found
- `SignatureDoesNotMatch` - Signature verification failed
- `RequestExpired` - Timestamp outside 15-minute window
- `AccessDenied` - ACL check failed

---

## 8. Architectural Observations

### 8.1 Separation of Concerns

ARMOR maintains clear separation between:
- **Authentication**: Credential verification (`SigV4Auth.VerifyRequest`)
- **Authorization**: Access control enforcement (`CheckACL`)
- **Request Processing**: Business logic (`handlers.HandleRoot`)

### 8.2 Header Termination Points

Authentication headers terminate at:
1. **Primary endpoint:** `verifyAuthAndGetCredential()` in `wrapHandler()` middleware
2. **Query auth:** `VerifyQueryAuth()` in `SigV4Auth` 
3. **Share endpoint:** `VerifyToken()` in presign module

### 8.3 No Header Propagation

ARMOR does NOT forward authentication headers to backend systems (B2). All authentication happens locally at the ARMOR layer before any backend operations.

### 8.4 Multiple Authentication Methods

ARMOR supports three authentication mechanisms:
1. **Header-based SigV4**: Standard AWS Authorization header
2. **Query-based SigV4**: Presigned URL parameters
3. **Token-based**: Custom tokens for public file sharing

---

## 9. Testing Implications

Based on this analysis, header handling tests should cover:

1. **Ingress validation**: Verify headers are correctly extracted at entry points
2. **Middleware flow**: Test CORS header addition and public path detection
3. **Authentication**: Verify SigV4 signature validation for both header and query auth
4. **Authorization**: Test ACL enforcement after successful authentication
5. **Header transformations**: Validate AWS chunked decoding and response headers
6. **Error responses**: Confirm proper error codes and messages for auth failures
7. **Special paths**: Verify `/healthz`, `/readyz`, and `/share/` bypass normal auth

---

## Conclusion

ARMOR's header handling follows a clear, layered architecture where authentication headers enter through HTTP servers, flow through middleware that performs validation, and terminate at the authentication endpoint before business logic executes. The system properly separates authentication (who is making the request) from authorization (what they are allowed to do), supporting multiple authentication methods while maintaining S3 compatibility.
