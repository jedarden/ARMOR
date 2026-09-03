# ADR-016: B2-Safe Multipart Metadata Finalization Protocol

## Status

**Status:** Accepted (2026-08-31)

**Context:** ARMOR multipart upload must securely finalize metadata (plaintext size, SHA-256, ETag, envelope version, wrapped DEK, IV, key ID) for objects at all sizes, including those exceeding B2's 5 GB CopyObject limit.

## Problem Statement

The current multipart contract assembles ciphertext first, then stamps final metadata with a same-source/same-destination CopyObject. This approach has critical limitations:

### Current Unsafe Pattern (envelope-v3.md:307-313)

```markdown
3. `CompleteMultipartUpload`:
   - B2 Complete → concatenate parts
   - `CopyObject` with `REPLACE` to set final metadata (version, wrapped DEK, etc.)
   - Delete all `.armor/multipart/<upload-id>/part-*.json` and `meta.json`
```

### Critical Limitations

1. **B2 5 GB CopyObject Limit**: `b2_copy_file` and S3-compatible CopyObject are limited to 5 GB source objects. For larger files, the operation fails entirely.
2. **Request Deadlines**: Even multi-gigabyte objects below 5 GB can outlive HTTP timeouts during CopyObject.
3. **Race Condition Window**: Between B2 CompleteMultipartUpload success and CopyObject success, the object exists as raw ciphertext without ARMOR metadata. If CopyObject fails or times out, the object is permanently corrupted metadata-wise.
4. **No Atomic Metadata Update**: B2 and S3 objects are immutable. Metadata changes require creating a new object (new file ID/version), leaving windows of inconsistency.
5. **Metadata Unavailable at Initiate**: Critical metadata values (plaintext size, combined plaintext SHA-256, final ETag) are only known after all parts are uploaded and combined.

### ARMOR Metadata Fields (12 total)

- `x-amz-meta-armor-version` - Format version (2 or 3)
- `x-amz-meta-armor-block-size` - Encryption block size  
- `x-amz-meta-armor-plaintext-size` - Original plaintext size
- `x-amz-meta-armor-plaintext-sha256` - SHA-256 of plaintext
- `x-amz-meta-armor-etag` - ARMOR's ETag
- `x-amz-meta-armor-content-type` - Original content type
- `x-amz-meta-armor-iv` - Initialization vector (base64)
- `x-amz-meta-armor-wrapped-dek` - Wrapped data encryption key
- `x-amz-meta-armor-key-id` - Key identifier
- `x-amz-meta-armor-multipart` - "true" for multipart uploads
- `x-amz-meta-armor-compressed` - Compression flag
- `x-amz-meta-armor-compression-type` - Compression algorithm

## Decision

**Selected Approach: Metadata Manifest Object Pattern**

Store final ARMOR metadata in a separate manifest object (`<key>.armor-manifest`) that references the assembled ciphertext object. The manifest is written atomically after CompleteMultipartUpload succeeds and contains all metadata. The ciphertext object itself never carries ARMOR metadata; it's opaque encrypted data.

### Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                      ARMOR Multipart Upload                       │
└─────────────────────────────────────────────────────────────────┘

1. CreateMultipartUpload (ciphertext object)
   ├── Upload ID: <upload-id>
   ├── Key: <prefix><key>
   └── Metadata: NONE (ciphertext object is opaque)

2. Upload Parts
   └── Store per-part state in .armor/multipart/<upload-id>/part-<n>.json

3. CompleteMultipartUpload (ciphertext object)
   └── B2 assembles parts → <prefix><key> (raw ciphertext)

4. Write Manifest Object (NEW: atomic metadata finalization)
   ├── Key: <prefix><key>.armor-manifest
   ├── Metadata:
   │   ├── x-amz-meta-armor-ciphertext-ref: <prefix><key>
   │   ├── x-amz-meta-armor-version: 3
   │   ├── x-amz-meta-armor-block-size: <block-size>
   │   ├── x-amz-meta-armor-plaintext-size: <total-size>
   │   ├── x-amz-meta-armor-plaintext-sha256: <combined-sha256>
   │   ├── x-amz-meta-armor-etag: <b2-etag>
   │   ├── x-amz-meta-armor-content-type: <original-type>
   │   ├── x-amz-meta-armor-iv: <iv-base64>
   │   ├── x-amz-meta-armor-wrapped-dek: <wrapped-dek-base64>
   │   ├── x-amz-meta-armor-key-id: <key-id>
   │   └── x-amz-meta-armor-multipart: "true"
   └── Body: JSON manifest (optional, for debugging)

5. Read Path (LOOKUP)
   ├── GET <prefix><key>.armor-manifest (manifest object)
   │   └── Returns ARMOR metadata
   └── GET <prefix><key> (ciphertext object)
       └── Returns raw ciphertext (no ARMOR metadata)

6. Cleanup
   └── Delete .armor/multipart/<upload-id>/ (unchanged)
```

### Protocol Steps

#### Step 1: CreateMultipartUpload (Unchanged)

```go
// Initialize B2 multipart upload with NO ARMOR metadata
uploadID, err := h.backend.CreateMultipartUpload(ctx, bucket, prefixedKey, nil)
if err != nil {
    return err
}

// Store upload state in .armor/multipart/<upload-id>/meta.json
state := &MultipartState{
    UploadID:        uploadID,
    Key:             key,
    Created:         time.Now().UTC(),
    BlockSize:       config.BlockSize,
    ContentType:     req.ContentType,
    IV:              iv,
    WrappedDEK:      wrappedDEK,
    MEKFingerprint:  mekFingerprint,
    KeyID:           keyID,
    FormatVersion:   3,
    PartSizes:       make(map[int64]int64),
    PartHMACs:       make(map[int64]string),
}
```

#### Step 2: UploadPart (Unchanged)

```go
// Upload encrypted part to B2
etag, err := h.backend.UploadPart(ctx, bucket, prefixedKey, uploadID, partNumber, ciphertextReader)
if err != nil {
    return err
}

// Store per-part state in .armor/multipart/<upload-id>/part-<n>.json
partState := &PartDataV3{
    PartNumber:   partNumber,
    ETag:         etag,
    Size:         int64(len(ciphertext)),
    PlaintextSHA: plaintextPartSHA,
    HMACs:        blockHMACs,
}
```

#### Step 3: CompleteMultipartUpload (Modified)

```go
// Complete B2 multipart upload
etag, err := h.backend.CompleteMultipartUpload(ctx, bucket, prefixedKey, uploadID, parts)
if err != nil {
    // Handle ambiguous completion (existing code)
    if backend.IsNoSuchUpload(err) {
        // Recovery logic: verify object exists and has correct size/timestamp
        info, headErr := h.backend.Head(ctx, bucket, prefixedKey)
        createdAt := state.Created.UTC().Truncate(time.Second)
        if headErr != nil || info == nil || info.Size != totalCiphertextSize ||
            (!state.Created.IsZero() && info.LastModified.UTC().Before(createdAt)) {
            return fmt.Errorf("failed to recover ambiguous completion")
        }
        etag = info.ETag
    } else {
        return err
    }
}

// NEW: Compute final metadata (existing code)
plaintextSHAHex := computeCombinedSHA256(state.PartDigests)

// NEW: Build ARMOR metadata
meta := (&backend.ARMORMetadata{
    Version:        3,
    BlockSize:      state.BlockSize,
    PlaintextSize:  totalPlaintextSize,
    ContentType:    state.ContentType,
    IV:             state.IV,
    WrappedDEK:     state.WrappedDEK,
    MEKFingerprint: state.MEKFingerprint,
    PlaintextSHA:   plaintextSHAHex,
    ETag:           etag,
    KeyID:          state.KeyID,
    Multipart:      true,
}).ToMetadata()

// NEW: Add ciphertext object reference
meta["x-amz-meta-armor-ciphertext-ref"] = h.applyPrefix(key)
```

#### Step 4: Write Manifest Object (NEW)

```go
// NEW: Write manifest object atomically
manifestKey := h.applyPrefix(key) + ".armor-manifest"

// Build manifest body (optional, for debugging)
manifestBody := &ManifestBody{
    CiphertextObject: h.applyPrefix(key),
    UploadID:         uploadID,
    CompletedAt:      time.Now().UTC(),
    Metadata:         meta,
}

manifestJSON, _ := json.Marshal(manifestBody)

// Write manifest object with metadata
manifestMeta := map[string]string{
    "Content-Type": "application/x-armor-manifest+json",
}
for k, v := range meta {
    manifestMeta[k] = v
}

err = h.backend.Put(ctx, bucket, manifestKey, bytes.NewReader(manifestJSON), int64(len(manifestJSON)), manifestMeta)
if err != nil {
    // CRITICAL: Manifest write failed but ciphertext object exists
    // This is a recoverable error - the upload succeeded but metadata is incomplete
    // Return error to client; they can retry manifest creation
    return fmt.Errorf("manifest write failed after completion: %w", err)
}

// Continue with HMAC sidecar and cleanup (existing code)
```

#### Step 5: Read Path (Modified)

```go
// NEW: Read manifest first, then ciphertext
manifestKey := h.applyPrefix(key) + ".armor-manifest"

manifestBody, manifestMeta, err := h.backend.Get(ctx, bucket, manifestKey)
if err != nil {
    // Fallback: check for legacy pattern (manifest missing, metadata on object)
    if errors.Is(err, backend.NoSuchKey) {
        // Try legacy read path
        return h.legacyGetObject(ctx, bucket, key, w, r)
    }
    return err
}

// Parse ARMOR metadata from manifest
armorMeta, err := backend.ParseARMORMetadata(manifestMeta)
if err != nil {
    return fmt.Errorf("invalid ARMOR metadata: %w", err)
}

// Get ciphertext object reference from manifest
ciphertextRef := armorMeta["x-amz-meta-armor-ciphertext-ref"]
if ciphertextRef == "" {
    return fmt.Errorf("missing ciphertext reference in manifest")
}

// Read ciphertext object
ciphertextBody, _, err := h.backend.Get(ctx, bucket, ciphertextRef)
if err != nil {
    return err
}

// Decrypt and stream response (existing code)
```

### Safety Properties

1. **No Race Condition Window**: The manifest write is atomic. Either the manifest exists (complete object) or it doesn't (upload in progress/failed). There is never a window where a ciphertext object is readable as a valid ARMOR object without metadata.

2. **Works at All Sizes**: No CopyObject operation required. Works for 1 MB objects the same as 10 TB objects.

3. **No Paid Egress**: No data movement or re-upload. Only small manifest object writes.

4. **Recoverable Failures**: 
   - If manifest write fails, the ciphertext object exists but is not readable as ARMOR. Client can retry manifest creation.
   - If CompleteMultipartUpload times out ambiguously, existing recovery logic applies.
   - If process crashes after Complete but before manifest, manual recovery can recreate manifest from upload state.

5. **Same-Key Overwrites**: New manifest atomically replaces old manifest. Ciphertext object is overwritten by CompleteMultipartUpload as before.

### Source-Version Pinning

The manifest includes the ciphertext object reference, creating an explicit dependency. This prevents a race condition where:

1. Upload A completes (ciphertext A, manifest A)
2. Upload B completes to same key (ciphertext B overwrites A)
3. Manifest A still references ciphertext A (now gone)

**Solution**: The manifest includes `x-amz-meta-armor-completed-at` timestamp. On read, verify:
- Ciphertext object's `LastModified` <= manifest's `completed-at`
- If not (ciphertext NEWER than the manifest), the ciphertext has been overwritten since this manifest was finalized - return error or re-read the latest manifest

> **Correction (2026-09-03):** This section originally specified
> `LastModified >= completed-at`. That direction is backwards for the flow as
> implemented: `CompleteMultipartUpload` assembles the ciphertext object first
> (B2 multipart complete) and writes the manifest afterwards, so a ciphertext
> *older* than `completed-at` is the normal, healthy ordering. Applying the
> original comparison rejected every multipart object whose completion outlived
> its assembly second — seen in production on ord-devimprint (2026-08-31), where
> a 60s assembly gap permanently 500ed GetObject for a litestream segment. The
> implemented check (`handlers.verifyCiphertextFreshness`) rejects only a
> ciphertext strictly newer than the manifest, which is the ADR's stated intent:
> detecting an overwrite that landed between the two manifest writes.

### Concurrent Writers

Same-key concurrent writes are already serialized by B2's upload ID mechanism. With manifests:

1. Writer A: CreateMultipartUpload → uploadID-A
2. Writer B: CreateMultipartUpload → uploadID-B  
3. Writer A: Complete → ciphertext-A, write manifest-A
4. Writer B: Complete → ciphertext-B (overwrites A), write manifest-B (overwrites A)

**Race**: Writer A's manifest now references gone ciphertext-A.

**Mitigation**: `completed-at` timestamp verification as above. If stale, re-read latest manifest.

### Cancellation and Cleanup

- **User Cancels**: Delete all uploaded parts + upload state. No manifest written.
- **Timeout During Upload**: B2 auto-aborts multipart upload after 7 days. Parts + upload state remain; cleanup job deletes.
- **Timeout During Complete**: Handled by existing ambiguous completion recovery.
- **Manifest Write Failure**: Ciphertext exists but no manifest. Operator can manually recreate manifest from upload state in `.armor/multipart/<upload-id>/`.

### Metadata Authority

**Authoritative locations:**

| Metadata Field | Authoritative Source | Backup/Recovery |
|---|---|---|
| Upload ID | `.armor/multipart/<upload-id>/meta.json` | Lost if state deleted before manifest |
| IV, Wrapped DEK, Key ID | Manifest object | Recreate from upload state |
| Plaintext Size | Manifest object | Recalculate from part sizes |
| Plaintext SHA-256 | Manifest object | Recalculate from part digests |
| ETag | Manifest object | Get from B2 HeadObject |
| Block Size | Manifest object | From upload state |

**Recovery Procedure** (if manifest lost but ciphertext exists):
1. Locate upload state from logs or operator records
2. Load `.armor/multipart/<upload-id>/meta.json` and part-*.json files
3. Recalculate final SHA-256 from part digests
4. Get ETag from B2 HeadObject on ciphertext object
5. Reconstruct and write manifest

## Alternatives Considered

### Alternative 1: Set All Metadata at CreateMultipartUpload

**Approach**: Set all possible metadata at initiation time, using placeholders for unknown values.

**Problems**:
- Plaintext size unknown at initiation (depends on what client uploads)
- Combined SHA-256 unknown until all parts uploaded
- Final ETag unknown until B2 CompleteMultipartUpload
- Would require post-completion CopyObject anyway to update placeholders
- Doesn't solve the fundamental problem

**Rejected**: Cannot set final metadata at initiation.

### Alternative 2: Segmented UploadPartCopy for Large Objects

**Approach**: For objects >5 GB, use multipart upload copy (b2_copy_part) instead of CopyObject.

**Problems**:
- Requires a new multipart upload ID for the copy operation
- Creates a second assembled object (double storage during copy)
- Doesn't eliminate CopyObject, just works around 5 GB limit
- Still has race condition window between complete and copy
- Significantly more complex and error-prone

**Rejected**: Doesn't solve race condition; adds complexity and temporary storage overhead.

### Alternative 3: DynamoDB Metadata Store Pattern

**Approach**: Store ARMOR metadata in DynamoDB instead of object metadata, reference by key hash.

**Problems**:
- Introduces external dependency (DynamoDB) not present in current architecture
- Adds operational complexity (DynamoDB provisioning, capacity planning, cost)
- Creates consistency boundary between S3 and DynamoDB
- Doesn't eliminate race condition (DynamoDB write could still fail after Complete)
- Read path requires both S3 GetObject + DynamoDB GetItem (latency + cost)
- B2 deployments may not have DynamoDB available

**Rejected**: External dependency adds complexity without solving the core atomic write problem. The manifest pattern keeps everything within the storage system.

### Alternative 4: Deferred Metadata via Object Lock

**Approach**: Create object with Object Lock (WORM), update metadata via governance bypass, then release lock.

**Problems**:
- Object Lock is a retention/governance feature, not a metadata update mechanism
- Still requires CopyObject to update metadata (lock doesn't bypass immutability)
- Governance bypass requires special IAM permissions (`s3:BypassGovernanceRetention`)
- Creates security/cost implications
- Not available on all S3-compatible systems (B2 support unclear)
- Doesn't solve the 5 GB CopyObject limit

**Rejected**: Object Lock doesn't enable metadata updates; still requires CopyObject.

## Migration Strategy

### Phase 1: Dual-Write (Backward Compatible)

1. Modify CompleteMultipartUpload to:
   - Write manifest object (new)
   - Attempt CopyObject for metadata (legacy, best-effort)
   
2. Modify read path to:
   - Try manifest object first
   - Fallback to legacy pattern if manifest missing

3. Deploy and monitor for manifest write success rate

### Phase 2: Manifest-Primary

1. Remove best-effort CopyObject
2. Manifest object is now the authoritative metadata source
3. Legacy fallback remains for old objects

### Phase 3: Cleanup

1. Add migration job to add manifests to existing multipart objects
2. Remove legacy fallback after migration completes
3. Update envelope-v3.md documentation to remove CopyObject mandate

## Consequences

### Positive

1. **Eliminates 5 GB Limit**: No CopyObject means no size constraint.
2. **Eliminates Race Condition**: Atomic manifest write, no window of inconsistency.
3. **Simpler Recovery**: Clear authoritative source for metadata.
4. **No Performance Regression**: No additional round-trips or data movement.
5. **B2 and S3 Compatible**: Works on any S3-compatible storage.

### Negative

1. **Breaking Change**: Requires read path modification to look for manifest.
2. **Dual Object Pattern**: Every ARMOR object now requires 2 S3 objects (ciphertext + manifest).
3. **Migration Complexity**: Existing objects need manifests added.
4. **Debugging Complexity**: Manifest and ciphertext can get out of sync (detectable via timestamp).

### Neutral

1. **Storage Cost**: Manifest objects are small (~1 KB each), negligible cost impact.
2. **Request Cost**: One additional PUT request per upload, one additional GET per read.
3. **Operational Complexity**: Need to handle manifest creation failures and recovery.

## Implementation Plan

### Step 1: Backend Interface Changes

```go
// internal/backend/backend.go

type ARMORMetadata struct {
    Version         int
    BlockSize       int
    PlaintextSize   int64
    ContentType     string
    IV              string // base64
    WrappedDEK      string // base64
    MEKFingerprint  string
    PlaintextSHA    string // hex
    ETag            string
    KeyID           string
    Multipart       bool
    Compressed      bool
    CompressionType string
    // NEW:
    CiphertextRef   string // prefixed key to ciphertext object
    CompletedAt     string // ISO 8601 timestamp
}

func (m *ARMORMetadata) ToMetadata() map[string]string {
    // Existing implementation + new fields
}

type ManifestBody struct {
    CiphertextObject string            `json:"ciphertext_object"`
    UploadID        string            `json:"upload_id"`
    CompletedAt     time.Time         `json:"completed_at"`
    Metadata        map[string]string `json:"metadata"`
}
```

### Step 2: Handler Changes

```go
// internal/server/handlers/handlers.go

func (h *Handler) CompleteMultipartUpload(w http.ResponseWriter, r *http.Request) {
    // ... existing completion logic ...
    
    // NEW: Write manifest object
    if err := h.writeManifest(ctx, bucket, key, meta, uploadID, etag); err != nil {
        h.writeError(w, r, "InternalError", 
            fmt.Sprintf("Failed to write manifest after completion: %v", err), 500)
        return
    }
    
    // ... existing HMAC sidecar and cleanup ...
}

func (h *Handler) GetObject(w http.ResponseWriter, r *http.Request) {
    // NEW: Try manifest first
    manifest, meta, err := h.readManifest(ctx, bucket, key)
    if err == nil {
        // Verify ciphertext freshness
        if err := h.verifyCiphertextFreshness(ctx, bucket, manifest, meta); err != nil {
            h.writeError(w, r, "InternalError", 
                fmt.Sprintf("Stale manifest: %v", err), 500)
            return
        }
        return h.serveObjectWithManifest(ctx, bucket, key, manifest, meta, w, r)
    }
    
    // Fallback to legacy pattern
    return h.legacyGetObject(ctx, bucket, key, w, r)
}
```

### Step 3: Testing

```go
// internal/server/handlers/handlers_test.go

func TestMultipartMetadataFinalization(t *testing.T) {
    // Test 1: Small object (< 5 GB) - manifest written successfully
    // Test 2: Large object (> 5 GB) - manifest written successfully
    // Test 3: Manifest write failure - recovery path
    // Test 4: Read path with manifest
    // Test 5: Read path fallback to legacy
    // Test 6: Stale manifest detection
    // Test 7: Concurrent same-key uploads
}
```

### Step 4: Documentation Updates

```markdown
# docs/format/envelope-v3.md (UPDATED)

## Multipart Upload Flow

### 3. CompleteMultipartUpload

- Read all per-part JSON objects from `.armor/multipart/<upload-id>/part-*.json`
- Construct HMAC sidecar (see HMAC Sidecar below)
- **NEW**: Write manifest object to `<prefix><key>.armor-manifest` with all ARMOR metadata
- B2 Complete → concatenate parts into `<prefix><key>` (ciphertext object, NO metadata)
- Delete all `.armor/multipart/<upload-id>/part-*.json` and `meta.json`

### Manifest Object Structure

The manifest object (`<key>.armor-manifest`) is the authoritative source for ARMOR metadata:

```
Key: <prefix><key>.armor-manifest
Metadata:
  x-amz-meta-armor-ciphertext-ref: <prefix><key>
  x-amz-meta-armor-version: 3
  x-amz-meta-armor-block-size: <block-size>
  x-amz-meta-armor-plaintext-size: <total-size>
  x-amz-meta-armor-plaintext-sha256: <combined-sha256>
  x-amz-meta-armor-etag: <b2-etag>
  x-amz-meta-armor-content-type: <original-type>
  x-amz-meta-armor-iv: <iv-base64>
  x-amz-meta-armor-wrapped-dek: <wrapped-dek-base64>
  x-amz-meta-armor-key-id: <key-id>
  x-amz-meta-armor-multipart: "true"
Body (JSON, optional):
  {
    "ciphertext_object": "<prefix><key>",
    "upload_id": "<upload-id>",
    "completed_at": "2026-08-31T12:34:56Z",
    "metadata": { ... }
  }
```

### Read Path

1. GET `<prefix><key>.armor-manifest` → returns ARMOR metadata
2. GET `<prefix><key>` → returns raw ciphertext
3. Decrypt ciphertext using metadata from manifest
```

## References

- B2 API Documentation: https://www.backblaze.com/apidocs/b2-copy-file
- B2 API Documentation: https://www.backblaze.com/apidocs/b2-start-large-file
- AWS S3 CompleteMultipartUpload: https://docs.aws.amazon.com/AmazonS3/latest/API/API_CompleteMultipartUpload.html
- ARMOR envelope-v3.md (current, to be updated)
- ARMOR handlers.go:3726 (current CopyObject implementation)
- ARMOR multipart.go (state management and sidecar storage)
