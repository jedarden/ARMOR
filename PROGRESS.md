# ARMOR Implementation Progress

## Phase 1: Core (MVP)

### Completed
- [x] Project structure and Go module initialization
- [x] Configuration module (environment variable loading)
- [x] Crypto module
  - [x] AES-256-CTR encryption with per-block HMAC
  - [x] Envelope encryption (MEK wraps DEK per file)
  - [x] Encrypted object format (header + data blocks + HMAC table)
  - [x] Key wrap/unwrap (AES-KWP RFC 5649)
  - [x] HMAC key derivation via HKDF
  - [x] Range read translation (plaintext offset → encrypted block offset)
- [x] Backend interface and B2 S3 implementation
  - [x] Pluggable Backend interface
  - [x] B2 S3 client for uploads
  - [x] Cloudflare download path for free egress
  - [x] Metadata LRU cache
- [x] S3 server and handlers
  - [x] PutObject (with encryption)
  - [x] GetObject (full + range, with decryption)
  - [x] HeadObject (metadata translation)
  - [x] DeleteObject
  - [x] ListObjectsV2 (with size correction)
  - [x] Full AWS SigV4 authentication (signature verification)
- [x] Health check endpoints (`/healthz`, `/readyz`, `/armor/canary`)
- [x] Self-healing canary integrity monitor
  - [x] CF-Cache-Status header detection for Cloudflare cache hit tracking
- [x] Parquet footer pinning (in-memory, keyed by ETag)
- [x] Parallel data + HMAC range fetch (errgroup)
- [x] Pipelined stream decryption (io.Pipe) - decrypts blocks as they stream
- [x] Unit tests for crypto, canary, and auth modules (all passing)
- [x] Multi-stage Dockerfile
- [x] CI build + GHCR publish
  - [x] GitHub Actions CI workflow (test, build, lint)
  - [x] GitHub Actions release workflow (tag-triggered Docker build + push to GHCR)
  - [x] Multi-platform support (linux/amd64, linux/arm64)

## Phase 2: Production Hardening

### Completed
- [x] CopyObject (for rename and key rotation)
  - [x] DEK re-wrapping on copy (enables key rotation)
  - [x] Cross-bucket copy support
  - [x] Metadata directive (COPY/REPLACE)
  - [x] Unit tests
- [x] DeleteObjects (bulk delete)
  - [x] XML parsing for delete request
  - [x] Quiet mode support
  - [x] Unit tests
- [x] Bucket operations
  - [x] ListBuckets
  - [x] CreateBucket
  - [x] DeleteBucket
  - [x] HeadBucket
  - [x] Unit tests
- [x] Multipart upload support
  - [x] CreateMultipartUpload (generates DEK+IV, stores state in B2)
  - [x] UploadPart (encrypts with CTR counter offset, stores per-part HMACs)
  - [x] CompleteMultipartUpload (assembles parts, stores HMAC sidecar)
  - [x] AbortMultipartUpload (cleans up state)
  - [x] ListParts (with plaintext sizes)
  - [x] ListMultipartUploads (lists active multipart uploads)
  - [x] Multipart state persistence in B2 (.armor/multipart/<upload-id>.state)
  - [x] HMAC sidecar for multipart objects (.armor/hmac/<key-hash>)
  - [x] Unit tests for all multipart operations
- [x] Kubernetes manifests
  - [x] Deployment with health/readiness probes
  - [x] Service (ClusterIP + headless)
  - [x] Secret template
  - [x] Kustomization
- [x] Key rotation via API endpoint
  - [x] POST /admin/key/rotate endpoint
  - [x] Re-wraps all DEKs with new MEK via CopyObject
  - [x] Progress tracking in B2 (.armor/rotation-state.json)
  - [x] Resumable rotation (can continue interrupted rotations)
  - [x] Skips internal .armor/ objects and non-ARMOR objects
  - [x] GET /admin/key/export endpoint (with ?confirm=yes safety)
  - [x] GET /admin/key/verify endpoint (via canary status)
  - [x] Unit tests
- [x] Cryptographic provenance chain
  - [x] Provenance manager for recording uploads
  - [x] Per-writer chain branches in B2
  - [x] Chain hash linking (SHA-256 of prev + object metadata)
  - [x] Skip internal .armor/ objects
  - [x] Unit tests
- [x] Audit endpoint
  - [x] GET /admin/audit endpoint
  - [x] Walks all writer chains
  - [x] Detects untracked ARMOR-encrypted objects
  - [x] Returns JSON audit result
- [x] Provenance integration with handlers
  - [x] Record provenance on PutObject
  - [x] Record provenance on CopyObject
  - [x] Record provenance on CompleteMultipartUpload
- [x] Graceful shutdown + in-flight request draining
  - [x] RequestTracker with sync.WaitGroup
  - [x] Multi-phase shutdown (stop accepting → drain requests → stop background)
  - [x] Proper canary monitor shutdown
- [x] Structured logging (JSON)
  - [x] New logging package with JSON output
  - [x] Log levels (Debug, Info, Warn, Error)
  - [x] Field chaining for structured context
  - [x] Integration with server handlers
- [x] Prometheus metrics
  - [x] New metrics package with expvar
  - [x] Request/transfer/cache/encryption/canary metrics
  - [x] /metrics endpoint in Prometheus format
  - [x] Unit tests for logging and metrics packages
- [x] Conditional request handling (RFC 7232)
  - [x] If-Match header (412 Precondition Failed on mismatch)
  - [x] If-None-Match header (304 Not Modified on match)
  - [x] If-Modified-Since header (304 Not Modified if not modified)
  - [x] If-Unmodified-Since header (412 Precondition Failed if modified)
  - [x] Support for multiple ETags in If-Match/If-None-Match
  - [x] Support for wildcard (*) in If-Match/If-None-Match
  - [x] Applied to GetObject and HeadObject handlers
  - [x] Unit tests for all conditional request scenarios

### Completed
- [x] Integration tests against real B2 + Cloudflare
  - [x] Integration test framework (tests/integration/)
  - [x] PutObject/GetObject roundtrip test
  - [x] Range read tests
  - [x] HeadObject plaintext size test
  - [x] ListObjectsV2 size correction test
  - [x] DeleteObject test
  - [x] CopyObject test
  - [x] Multipart upload test
  - [x] Large file streaming test
  - [x] Conditional request tests
  - [x] Pre-signed URL test
  - [x] Health endpoint tests
  - [x] Canary endpoint test
  - [x] Direct B2 download test (verifies encryption)
  - [x] README with setup instructions

## Phase 3: Advanced Features

### Completed
- [x] Multi-key routing (different MEKs for different prefixes)
  - [x] New keymanager package for key routing
  - [x] Support for ARMOR_MEK_<NAME> environment variables
  - [x] Support for ARMOR_KEY_ROUTES prefix-to-key mapping
  - [x] Key ID stored in x-amz-meta-armor-key-id metadata
  - [x] Automatic key selection on encrypt/decrypt
  - [x] Key-aware CopyObject (re-wraps with destination key)
  - [x] Key-aware multipart uploads
  - [x] Unit tests

### Completed
- [x] Multiple auth credentials with per-key ACLs
  - [x] Credential struct with AccessKey, SecretKey, and ACLs
  - [x] Named credentials via ARMOR_AUTH_<NAME>_ACCESS_KEY/SECRET_KEY/ACL env vars
  - [x] ACL format: bucket:prefix (wildcard bucket "*", empty prefix for full access)
  - [x] SigV4Auth updated to support credential lookup
  - [x] CheckACL function for bucket/prefix validation
  - [x] Unit tests for multi-credential auth and ACLs

### Completed
- [x] Pre-signed URL proxy
  - [x] New presign package for URL generation and verification
  - [x] HMAC-SHA256 signature for token authentication
  - [x] Configurable expiration (1 minute to 7 days)
  - [x] POST /admin/presign endpoint to generate share URLs
  - [x] GET /share/<token> endpoint to serve decrypted content
  - [x] Range request support for partial content
  - [x] Content-Disposition override option
  - [x] Unit tests for token generation and verification

### Completed
- [x] Streaming encryption for very large uploads
  - [x] Automatic threshold-based switching (10MB threshold)
  - [x] Temp file buffering for SHA-256 computation
  - [x] io.Pipe streaming for memory-efficient encryption
  - [x] X-Armor-Streaming header for visibility
  - [x] Full range read support for streaming-encrypted files
  - [x] Bug fix: DecryptRange now uses relative block indices
  - [x] Unit tests for streaming encryption scenarios

### Completed
- [x] Lifecycle rule passthrough
  - [x] GetBucketLifecycleConfiguration (GET ?lifecycle)
  - [x] PutBucketLifecycleConfiguration (PUT ?lifecycle)
  - [x] DeleteBucketLifecycleConfiguration (DELETE ?lifecycle)
  - [x] Backend interface methods for lifecycle operations
  - [x] B2 S3 implementation of lifecycle operations
  - [x] Unit tests for lifecycle handlers

### Completed
- [x] Object Lock / retention passthrough
  - [x] GetObjectLockConfiguration (GET ?object-lock on bucket)
  - [x] PutObjectLockConfiguration (PUT ?object-lock on bucket)
  - [x] GetObjectRetention (GET ?retention on object)
  - [x] PutObjectRetention (PUT ?retention on object)
  - [x] GetObjectLegalHold (GET ?legal-hold on object)
  - [x] PutObjectLegalHold (PUT ?legal-hold on object)
  - [x] Backend interface methods for object lock operations
  - [x] B2 S3 implementation of object lock operations
  - [x] Unit tests for object lock handlers (6 new tests)

### Completed
- [x] ListObjectVersions with per-version decryption
  - [x] Backend interface method (ListObjectVersions)
  - [x] B2 S3 implementation
  - [x] ObjectVersionInfo and ListObjectVersionsResult types
  - [x] Unit tests for types
  - [x] S3 handler for GET ?versions
  - [x] Per-version metadata retrieval (HeadVersion method)
  - [x] Unit tests for handler

### Completed
- [x] Admin API: B2 application key management via native API
  - [x] kurin/blazer dependency for B2 native API
  - [x] b2keys package with Client wrapper
  - [x] GET /admin/b2/keys - List B2 application keys
  - [x] POST /admin/b2/keys - Create new B2 application key
  - [x] DELETE /admin/b2/keys/{id} - Delete B2 application key
  - [x] Key capabilities, prefix, and duration support
  - [x] Unit tests for b2keys package and handlers

---

## Implementation Status

**All three phases are complete.** The ARMOR implementation is feature-complete per the plan.

**Last verified:** 2026-03-24 — CI passing, all tests green, no lint errors. Marathon verification at 2026-03-24T22:45Z confirmed project is feature-complete with no pending work.

### Remaining Optional Items
- [x] Web dashboard (optional): bucket browser, encryption status, cache stats
  - GET /dashboard — HTML dashboard with bucket browser
  - GET /dashboard/object?key=... — Object detail JSON (ARMOR metadata)
  - GET /dashboard/metrics — JSON metrics summary
  - Cache hit rate, bytes transferred, canary status display
  - Breadcrumb navigation for prefix browsing
  - ARMOR encryption badges with key ID display
  - Unit tests for all handlers

---

## Documentation Updates

### Completed
- [x] README.md updated to reflect actual S3 proxy implementation
  - Removed outdated CLI interface documentation
  - Added Docker quick start instructions
  - Added client configuration examples (AWS CLI, boto3, DuckDB)
  - Added full configuration reference
  - Added multi-key and multi-credential examples
  - Added Admin API documentation
  - Added S3 API coverage table

---

## Post-Implementation Fixes

### Completed
- [x] Dashboard build fix: Corrected PlaintextSHA field name and removed unused import
  - Changed PlaintextSHA256 to PlaintextSHA to match ARMORMetadata struct
  - Removed unused 'bytes' import from dashboard_test.go
- [x] Dashboard test fix: Fixed nil pointer dereference in TestObjectDetailHandlerNotFound
  - Mock Head method now returns error when object not found
- [x] Go version fix: Upgraded from 1.24 to 1.25.0 (required by golang.org/x/crypto@v0.49.0)
  - Updated go.mod to Go 1.25.0
  - Updated CI workflow to use Go 1.25 with GOTOOLCHAIN=local
- [x] CI lint job fix: Updated golangci-lint from v1.64.8 to v2.11.4
  - v1.64.8 was built with Go 1.24, incompatible with Go 1.25
  - v2.11.4 supports Go 1.25
- [x] Data race fix in TestRequestTrackerWait
  - Fixed race condition where Wait() could be called before Start() completed
  - Added synchronization channel to ensure proper ordering
- [x] golangci-lint v2 config format fix
  - Removed `gosimple` linter (merged into `staticcheck` in v2)
  - Moved `linters.settings` to top-level `linters-settings` section
  - Added `default: none` to explicitly control enabled linters
- [x] Staticcheck lint fixes (20+ issues resolved)
  - ST1005: Lowercased error strings
  - QF1003: Converted if-chains to tagged switches
  - SA9003: Removed empty branches
  - QF1001: Fixed unnecessary calls to reflect.Value.Interface
  - Disabled errcheck for intentional defer Close() patterns
