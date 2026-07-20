// Package server provides key rotation functionality for ARMOR.
package server

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
	"github.com/jedarden/armor/internal/manifest"
)

// B2CopyObjectSizeCeiling is the maximum object size B2's S3-compatible
// CopyObject API will copy in a single request: 5 GiB, the same ceiling as AWS
// S3 CopyObject. Rotation re-wraps DEKs via CopyObject(MetadataDirective=
// REPLACE); objects larger than this cannot be re-wrapped that way and must be
// rewritten with a multipart copy instead. Rotation enumerates such objects as
// exceptions (see ErrCopyObjectTooLarge) rather than silently skipping them.
const B2CopyObjectSizeCeiling int64 = 5 * 1024 * 1024 * 1024 // 5 GiB

// ErrCopyObjectTooLarge is returned by rotateObject when an object exceeds
// B2CopyObjectSizeCeiling. The rotation loop surfaces these as exceptions
// (RotationResult.Exceptions / ExceptionKeys) instead of attempting a copy
// that the B2 API would reject, and instead of silently skipping the object.
var ErrCopyObjectTooLarge = errors.New("object exceeds B2 CopyObject size ceiling")

// armor metadata header keys used by rotation. Defined here as constants so the
// merge-and-overwrite logic in rotateObject can never drift from the keys the
// encrypt/decrypt paths read and write.
const (
	armorMetaVersion    = "x-amz-meta-armor-version"
	armorMetaWrappedDEK = "x-amz-meta-armor-wrapped-dek"
	armorMetaMultipart  = "x-amz-meta-armor-multipart"
	armorMetaPartSize   = "x-amz-meta-armor-part-size"
)

// RotationState tracks the progress of a key rotation operation.
type RotationState struct {
	// ID is a unique identifier for this rotation (hash of old MEK + new MEK + timestamp)
	ID string `json:"id"`
	// OldMEKHash is the SHA-256 hash of the old MEK (first 16 hex chars for verification)
	OldMEKHash string `json:"old_mek_hash"`
	// NewMEKHash is the SHA-256 hash of the new MEK (first 16 hex chars for verification)
	NewMEKHash string `json:"new_mek_hash"`
	// StartTime is when the rotation began
	StartTime time.Time `json:"start_time"`
	// LastUpdated is when the state was last updated
	LastUpdated time.Time `json:"last_updated"`
	// Status is the current status: "in_progress", "completed", "failed"
	Status string `json:"status"`
	// TotalObjects is the total number of objects to rotate
	TotalObjects int `json:"total_objects"`
	// ProcessedObjects is the number of objects processed so far
	ProcessedObjects int `json:"processed_objects"`
	// LastKey is the last object key processed (for resumption)
	LastKey string `json:"last_key"`
	// ErrorMessage contains any error that occurred
	ErrorMessage string `json:"error_message,omitempty"`
}

// RotationResult contains the result of a key rotation operation.
type RotationResult struct {
	TotalObjects     int           `json:"total_objects"`
	ProcessedObjects int           `json:"processed_objects"`
	SkippedObjects   int           `json:"skipped_objects"`
	// Exceptions is the number of objects that could not be re-wrapped via
	// CopyObject — currently objects larger than B2CopyObjectSizeCeiling.
	// These are NOT counted in ProcessedObjects or SkippedObjects and are NOT
	// silently skipped: ExceptionKeys lists them so an operator can re-wrap them
	// with a multipart copy.
	Exceptions     int           `json:"exceptions"`
	ExceptionKeys  []string      `json:"exception_keys,omitempty"`
	Duration       time.Duration `json:"duration"`
	Status         string        `json:"status"`
	ErrorMessage   string        `json:"error_message,omitempty"`
}

// KeyRotator handles MEK rotation operations.
type KeyRotator struct {
	backend backend.Backend
	bucket  string
	oldMEK  []byte
	newMEK  []byte
	// idx is the manifest index used to skip HeadObject calls during rotation.
	// May be nil when the manifest is disabled or unavailable.
	idx *manifest.Index

	// state tracks rotation progress
	state     *RotationState
	stateMu   sync.Mutex
	statePath string // .armor/rotation-state.json
}

// NewKeyRotator creates a new key rotator. idx may be nil if the manifest
// index is not available; rotation falls back to per-object HeadObject calls.
func NewKeyRotator(b backend.Backend, bucket string, oldMEK, newMEK []byte, idx *manifest.Index) *KeyRotator {
	return &KeyRotator{
		backend:   b,
		bucket:    bucket,
		oldMEK:    oldMEK,
		newMEK:    newMEK,
		idx:       idx,
		statePath: ".armor/rotation-state.json",
	}
}

// Rotate performs the key rotation, re-wrapping all DEKs with the new MEK.
func (kr *KeyRotator) Rotate(ctx context.Context) (*RotationResult, error) {
	startTime := time.Now()

	// Initialize or load state
	if err := kr.initOrLoadState(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize rotation state: %w", err)
	}

	kr.stateMu.Lock()
	kr.state.Status = "in_progress"
	kr.state.StartTime = startTime
	kr.state.LastUpdated = startTime
	kr.stateMu.Unlock()

	// Save initial state
	if err := kr.saveState(ctx); err != nil {
		return nil, fmt.Errorf("failed to save initial state: %w", err)
	}

	result := &RotationResult{
		Status: "in_progress",
	}

	// Count total objects first
	if err := kr.countObjects(ctx); err != nil {
		return nil, fmt.Errorf("failed to count objects: %w", err)
	}

	// Process all objects
	var continuationToken string
	for {
		select {
		case <-ctx.Done():
			result.Status = "interrupted"
			result.ErrorMessage = ctx.Err().Error()
			kr.stateMu.Lock()
			kr.state.Status = "interrupted"
			kr.state.ErrorMessage = ctx.Err().Error()
			kr.stateMu.Unlock()
			kr.saveState(context.Background()) // Best effort save
			return result, ctx.Err()
		default:
		}

		listResult, err := kr.backend.List(ctx, kr.bucket, "", "", continuationToken, 1000)
		if err != nil {
			result.Status = "failed"
			result.ErrorMessage = err.Error()
			kr.stateMu.Lock()
			kr.state.Status = "failed"
			kr.state.ErrorMessage = err.Error()
			kr.stateMu.Unlock()
			kr.saveState(context.Background())
			return result, fmt.Errorf("failed to list objects: %w", err)
		}

		for _, obj := range listResult.Objects {
			// Skip internal ARMOR objects
			if len(obj.Key) >= 7 && obj.Key[:7] == ".armor/" {
				result.SkippedObjects++
				continue
			}

			// Skip non-ARMOR encrypted objects (pass-through)
			if !obj.IsARMOREncrypted {
				result.SkippedObjects++
				continue
			}

			// Check if we should skip this object (already processed in a previous run)
			kr.stateMu.Lock()
			if kr.state.LastKey != "" && obj.Key <= kr.state.LastKey {
				kr.stateMu.Unlock()
				continue
			}
			kr.stateMu.Unlock()

			// Re-wrap the DEK for this object
			if err := kr.rotateObject(ctx, obj); err != nil {
				if errors.Is(err, ErrCopyObjectTooLarge) {
					// Oversized objects cannot be re-wrapped via CopyObject.
					// Enumerate them as exceptions (not silently skipped) and
					// advance LastKey past them so resume doesn't re-report them.
					result.Exceptions++
					result.ExceptionKeys = append(result.ExceptionKeys, obj.Key)
					log.Printf("rotation exception: %s cannot be re-wrapped via CopyObject: %v", obj.Key, err)
					kr.stateMu.Lock()
					kr.state.LastKey = obj.Key
					kr.state.LastUpdated = time.Now()
					kr.stateMu.Unlock()
					continue
				}
				log.Printf("Warning: failed to rotate key for %s: %v", obj.Key, err)
				// Continue with other objects - rotation is best-effort
			}

			result.ProcessedObjects++

			// Update state
			kr.stateMu.Lock()
			kr.state.ProcessedObjects++
			kr.state.LastKey = obj.Key
			kr.state.LastUpdated = time.Now()
			kr.stateMu.Unlock()

			// Save state periodically (every 100 objects)
			if result.ProcessedObjects%100 == 0 {
				if err := kr.saveState(ctx); err != nil {
					log.Printf("Warning: failed to save rotation state: %v", err)
				}
			}
		}

		if !listResult.IsTruncated {
			break
		}
		continuationToken = listResult.NextToken
	}

	// Mark rotation as complete
	kr.stateMu.Lock()
	kr.state.Status = "completed"
	kr.state.LastUpdated = time.Now()
	kr.stateMu.Unlock()

	if err := kr.saveState(ctx); err != nil {
		log.Printf("Warning: failed to save final rotation state: %v", err)
	}

	result.TotalObjects = kr.state.TotalObjects
	result.Duration = time.Since(startTime)
	result.Status = "completed"

	return result, nil
}

// rotateObject re-wraps the DEK for a single object in place.
//
// Rotation re-wraps the DEK via CopyObject with MetadataDirective=REPLACE.
// REPLACE overwrites the ENTIRE object metadata set with whatever map we send,
// so we MUST start from the object's current full metadata and overwrite only
// the wrapped-DEK. Rebuilding the map from ARMORMetadata.ToMetadata() would
// silently drop x-amz-meta-armor-multipart and x-amz-meta-armor-part-size —
// which makes every rotated multipart object unreadable, because the read path
// keys off the multipart marker to find the HMAC sidecar instead of an embedded
// envelope header. That is exactly the bf-24sxh7 failure mode reintroduced by
// rotation; preserving the raw metadata here is what prevents it.
func (kr *KeyRotator) rotateObject(ctx context.Context, obj backend.ObjectInfo) error {
	// Enforce the B2 CopyObject size ceiling before attempting the copy. B2/S3
	// CopyObject rejects objects above 5 GiB; surfacing it here yields a clear,
	// typed error the loop reports as an exception instead of an opaque
	// CopyObject failure or — worse — a silent skip.
	if obj.Size > B2CopyObjectSizeCeiling {
		return fmt.Errorf("%w: %s is %d bytes (ceiling %d); re-wrap requires a multipart copy, not CopyObject",
			ErrCopyObjectTooLarge, obj.Key, obj.Size, B2CopyObjectSizeCeiling)
	}

	// Resolve the object's full raw metadata. ListObjectsV2 on B2/S3 does not
	// return custom metadata, so when the List result lacks armor metadata we
	// fall back to a HeadObject call.
	rawMeta, err := kr.objectMetadata(ctx, obj)
	if err != nil {
		return err
	}

	// Resolve the current wrapped DEK. Prefer the manifest fast-path (avoids
	// re-parsing headers); fall back to parsing the raw metadata.
	oldWrappedDEK := kr.wrappedDEKFromManifest(obj.Key)
	if oldWrappedDEK == nil {
		armorMeta, ok := backend.ParseARMORMetadata(rawMeta)
		if !ok {
			return fmt.Errorf("object %s is not ARMOR-encrypted", obj.Key)
		}
		oldWrappedDEK = armorMeta.WrappedDEK
	}

	// Unwrap DEK with old MEK
	dek, err := crypto.UnwrapDEK(kr.oldMEK, oldWrappedDEK)
	if err != nil {
		return fmt.Errorf("failed to unwrap DEK with old MEK: %w", err)
	}

	// Re-wrap DEK with new MEK
	newWrappedDEK, err := crypto.WrapDEK(kr.newMEK, dek)
	if err != nil {
		return fmt.Errorf("failed to wrap DEK with new MEK: %w", err)
	}

	// Clone the full raw metadata and overwrite ONLY the wrapped-DEK. This
	// preserves x-amz-meta-armor-multipart, x-amz-meta-armor-part-size,
	// x-amz-meta-armor-key-id, plaintext-sha256, etag, and any non-ARMOR user
	// metadata across the REPLACE copy.
	newMeta := make(map[string]string, len(rawMeta))
	for k, v := range rawMeta {
		newMeta[k] = v
	}
	newMeta[armorMetaWrappedDEK] = base64.StdEncoding.EncodeToString(newWrappedDEK)

	// Copy object in place with updated metadata (B2 server-side copy).
	// For in-place copy, src and dst bucket/key are the same. The object body
	// (ciphertext) and ETag are untouched — only metadata changes.
	if err := kr.backend.Copy(ctx, kr.bucket, obj.Key, kr.bucket, obj.Key, newMeta, true); err != nil {
		return fmt.Errorf("failed to update object metadata: %w", err)
	}

	return nil
}

// objectMetadata returns the object's full raw metadata map. B2/S3
// ListObjectsV2 omits custom metadata, so when the List result does not carry
// armor metadata we fall back to a HeadObject call.
func (kr *KeyRotator) objectMetadata(ctx context.Context, obj backend.ObjectInfo) (map[string]string, error) {
	if obj.Metadata != nil && obj.Metadata[armorMetaVersion] != "" {
		return obj.Metadata, nil
	}
	info, err := kr.backend.Head(ctx, kr.bucket, obj.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to get object metadata: %w", err)
	}
	return info.Metadata, nil
}

// wrappedDEKFromManifest returns the wrapped DEK for the object from the
// in-memory manifest index, or nil if the manifest is disabled or has no entry.
func (kr *KeyRotator) wrappedDEKFromManifest(key string) []byte {
	if kr.idx == nil {
		return nil
	}
	if entry, ok := kr.idx.Get(kr.bucket, key); ok {
		return entry.WrappedDEK
	}
	return nil
}

// initOrLoadState initializes a new rotation state or loads an existing one.
func (kr *KeyRotator) initOrLoadState(ctx context.Context) error {
	// Compute rotation ID
	oldMEKHash := sha256.Sum256(kr.oldMEK)
	newMEKHash := sha256.Sum256(kr.newMEK)
	rotationID := fmt.Sprintf("%s-%s-%d",
		hex.EncodeToString(oldMEKHash[:8]),
		hex.EncodeToString(newMEKHash[:8]),
		time.Now().Unix())

	kr.state = &RotationState{
		ID:          rotationID,
		OldMEKHash:  hex.EncodeToString(oldMEKHash[:8]),
		NewMEKHash:  hex.EncodeToString(newMEKHash[:8]),
		StartTime:   time.Now(),
		LastUpdated: time.Now(),
		Status:      "initialized",
	}

	// Try to load existing state
	existingState, err := kr.loadState(ctx)
	if err == nil && existingState != nil {
		// Check if this is a continuation of the same rotation
		if existingState.OldMEKHash == kr.state.OldMEKHash &&
			existingState.NewMEKHash == kr.state.NewMEKHash &&
			existingState.Status == "in_progress" {
			kr.state = existingState
			log.Printf("Resuming rotation from key: %s", existingState.LastKey)
		}
	}

	return nil
}

// loadState loads the rotation state from B2.
func (kr *KeyRotator) loadState(ctx context.Context) (*RotationState, error) {
	reader, _, err := kr.backend.GetDirect(ctx, kr.bucket, kr.statePath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read state: %w", err)
	}

	var state RotationState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state: %w", err)
	}

	return &state, nil
}

// saveState saves the rotation state to B2.
func (kr *KeyRotator) saveState(ctx context.Context) error {
	kr.stateMu.Lock()
	state := *kr.state
	kr.stateMu.Unlock()

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Use a pipe to convert the byte slice to an io.Reader
	reader, writer := io.Pipe()
	go func() {
		defer writer.Close()
		writer.Write(data)
	}()

	meta := map[string]string{
		"Content-Type": "application/json",
	}

	if err := kr.backend.Put(ctx, kr.bucket, kr.statePath, reader, int64(len(data)), meta); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	return nil
}

// countObjects counts the total number of ARMOR-encrypted objects.
func (kr *KeyRotator) countObjects(ctx context.Context) error {
	var count int
	var continuationToken string

	for {
		listResult, err := kr.backend.List(ctx, kr.bucket, "", "", continuationToken, 1000)
		if err != nil {
			return err
		}

		for _, obj := range listResult.Objects {
			// Skip internal ARMOR objects
			if len(obj.Key) >= 7 && obj.Key[:7] == ".armor/" {
				continue
			}
			// Only count ARMOR-encrypted objects
			if obj.IsARMOREncrypted {
				count++
			}
		}

		if !listResult.IsTruncated {
			break
		}
		continuationToken = listResult.NextToken
	}

	kr.stateMu.Lock()
	kr.state.TotalObjects = count
	kr.stateMu.Unlock()

	return nil
}

// GetState returns the current rotation state.
func (kr *KeyRotator) GetState() *RotationState {
	kr.stateMu.Lock()
	defer kr.stateMu.Unlock()
	if kr.state == nil {
		return nil
	}
	state := *kr.state
	return &state
}
