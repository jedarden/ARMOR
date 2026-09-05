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
	"strings"
	"sync"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
	"github.com/jedarden/armor/internal/keymanager"
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

// ErrAlreadyUsingActiveKey is returned when an object is already wrapped with
// the target key's fingerprint. The rotation loop skips these objects.
var ErrAlreadyUsingActiveKey = errors.New("object already using active key fingerprint")

// armor metadata header keys are defined in format_migration.go as package-level
// constants. This comment documents that rotateObject depends on those constants.

// RotationState tracks the progress of a key rotation operation.
type RotationState struct {
	// ID is a unique identifier for this rotation (hash of old MEK + new MEK + timestamp)
	ID string `json:"id"`
	// OldMEKHash is the SHA-256 hash of the old MEK (first 16 hex chars for verification)
	OldMEKHash string `json:"old_mek_hash"`
	// NewMEKHash is the SHA-256 hash of the new MEK (first 16 hex chars for verification)
	NewMEKHash string `json:"new_mek_hash"`
	// TargetKeyID identifies the routed key being rotated. Empty means the
	// legacy all-object rotation mode used by direct callers of NewKeyRotator.
	TargetKeyID string `json:"target_key_id,omitempty"`
	// StartTime is when the rotation began
	StartTime time.Time `json:"start_time"`
	// LastUpdated is when the state was last updated
	LastUpdated time.Time `json:"last_updated"`
	// Status is the current status: "in_progress", "completed", "failed", or
	// "interrupted" (the walk stopped on a cancelled context). The first two of
	// those are resumable — see isResumableRotationStatus.
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
	TotalObjects     int `json:"total_objects"`
	ProcessedObjects int `json:"processed_objects"`
	SkippedObjects   int `json:"skipped_objects"`
	// Exceptions is the number of objects that could not be re-wrapped via
	// CopyObject — currently objects larger than B2CopyObjectSizeCeiling.
	// These are NOT counted in ProcessedObjects or SkippedObjects and are NOT
	// silently skipped: ExceptionKeys lists them so an operator can re-wrap them
	// with a multipart copy.
	Exceptions    int           `json:"exceptions"`
	ExceptionKeys []string      `json:"exception_keys,omitempty"`
	Duration      time.Duration `json:"duration"`
	Status        string        `json:"status"`
	ErrorMessage  string        `json:"error_message,omitempty"`
}

// KeyRotator handles MEK rotation operations.
type KeyRotator struct {
	backend backend.Backend
	bucket  string
	oldMEK  []byte
	newMEK  []byte
	// oldRing is the retired key ring for the old MEK, used for unwrapping
	// objects encrypted with retired keys during rotation.
	oldRing []keymanager.RingKeyEntry
	// targetFingerprint is the fingerprint of the new MEK. Objects whose
	// wrapped DEK carries this fingerprint are skipped (already using the
	// active key).
	targetFingerprint string
	// targetKeyID is the canonical key name to rotate. An empty value keeps
	// the legacy behavior of rotating every ARMOR object for direct callers of
	// NewKeyRotator.
	targetKeyID string
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
	return newKeyRotator(b, bucket, "", oldMEK, newMEK, nil, idx)
}

// NewKeyRotatorForKey creates a rotator that only re-wraps objects encrypted
// with keyID. The empty key ID selects the default key, including legacy
// objects whose metadata omits x-amz-meta-armor-key-id.
func NewKeyRotatorForKey(b backend.Backend, bucket, keyID string, oldMEK, newMEK []byte, oldRing []keymanager.RingKeyEntry, idx *manifest.Index) *KeyRotator {
	if keyID == "" {
		keyID = "default"
	}
	keyID = strings.ToLower(strings.TrimSpace(keyID))
	return newKeyRotator(b, bucket, keyID, oldMEK, newMEK, oldRing, idx)
}

// NewFingerprintRotator creates a rotator that re-wraps objects to the active
// key's fingerprint. It walks the bucket (manifest first, HeadObject fallback)
// and re-wraps only objects whose fingerprint ≠ the active key's fingerprint.
// This is the new recommended rotation method.
func NewFingerprintRotator(b backend.Backend, bucket, keyID string, activeMEK []byte, oldRing []keymanager.RingKeyEntry, idx *manifest.Index) *KeyRotator {
	if keyID == "" {
		keyID = "default"
	}
	keyID = strings.ToLower(strings.TrimSpace(keyID))
	return &KeyRotator{
		backend:           b,
		bucket:            bucket,
		newMEK:            activeMEK,
		oldRing:           oldRing,
		targetFingerprint: crypto.MEKFingerprint(activeMEK),
		targetKeyID:       keyID,
		idx:               idx,
		statePath:         ".armor/rotation-state.json",
	}
}

func newKeyRotator(b backend.Backend, bucket, targetKeyID string, oldMEK, newMEK []byte, oldRing []keymanager.RingKeyEntry, idx *manifest.Index) *KeyRotator {
	return &KeyRotator{
		backend:     b,
		bucket:      bucket,
		oldMEK:      oldMEK,
		newMEK:      newMEK,
		oldRing:     oldRing,
		targetKeyID: targetKeyID,
		idx:         idx,
		statePath:   ".armor/rotation-state.json",
	}
}

// Rotate performs the key rotation, re-wrapping all DEKs with the new MEK.
func (kr *KeyRotator) Rotate(ctx context.Context) (*RotationResult, error) {
	startTime := time.Now()

	// Initialize or load state
	if err := kr.initOrLoadState(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize rotation state: %w", err)
	}

	// Resume past the run boundary: StartTime belongs to the rotation, not to
	// this particular invocation, so it is left exactly as initOrLoadState set
	// it (fresh for a new rotation, original for a resumed one).
	kr.stateMu.Lock()
	kr.state.Status = "in_progress"
	kr.state.LastUpdated = startTime
	processedSoFar := kr.state.ProcessedObjects
	kr.stateMu.Unlock()

	// Save initial state
	if err := kr.saveState(ctx); err != nil {
		return nil, fmt.Errorf("failed to save initial state: %w", err)
	}

	result := &RotationResult{
		Status: "in_progress",
		// Cumulative across runs: a resumed rotation reports every object
		// re-wrapped so far, matching RotationState.ProcessedObjects.
		ProcessedObjects: processedSoFar,
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
			return kr.abortInterrupted(result, ctx.Err(), startTime)
		default:
		}

		listResult, err := kr.backend.List(ctx, kr.bucket, "", "", continuationToken, 1000)
		if err != nil {
			// A cancellation surfacing through List is an interruption, not a
			// listing failure: recording it as "failed" would leave a state the
			// next invocation refuses to resume from, which is the exact
			// restart-from-scratch this file exists to avoid.
			if ctxErr := ctx.Err(); ctxErr != nil {
				return kr.abortInterrupted(result, ctxErr, startTime)
			}
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

			// Check if we should skip this object (already processed in a previous
			// run) BEFORE any per-object inspection. A prior run already decided
			// the outcome for everything at or below LastKey — re-wrapped,
			// already-active skip, or oversized exception — and advanced LastKey
			// past it, so there is nothing left to learn about it. The ordering
			// matters because the key-ID branch below costs a backend Head per
			// listed object whenever List omits metadata (always on B2): placed
			// after that branch, every resume re-inspected the entire processed
			// prefix, and since skipped objects never reach checkpointState,
			// processed_objects/last_updated froze for the whole replay. Resume
			// only ever adopts state whose TargetKeyID matches (see
			// initOrLoadState), so the key-ID verdicts below LastKey do belong
			// to this rotation.
			kr.stateMu.Lock()
			if kr.state.LastKey != "" && obj.Key <= kr.state.LastKey {
				kr.stateMu.Unlock()
				continue
			}
			kr.stateMu.Unlock()

			var rawMeta map[string]string
			if kr.targetKeyID != "" {
				// ListObjectsV2 does not include user metadata on B2, so inspect
				// the object before filtering. This also makes named-key rotation
				// safe when a bucket contains objects owned by other MEKs.
				rawMeta, err = kr.objectMetadata(ctx, obj)
				if err != nil {
					log.Printf("Warning: failed to inspect object %s: %v", obj.Key, err)
					result.SkippedObjects++
					continue
				}
				armorMeta, ok := backend.ParseARMORMetadata(rawMeta)
				if !ok {
					result.SkippedObjects++
					continue
				}
				effectiveKeyID := strings.ToLower(strings.TrimSpace(armorMeta.KeyID))
				if effectiveKeyID == "" {
					effectiveKeyID = "default"
				}
				if effectiveKeyID != kr.targetKeyID {
					result.SkippedObjects++
					continue
				}
			} else if !obj.IsARMOREncrypted {
				// Legacy callers may provide a backend whose listing includes
				// encryption metadata. Preserve the old fast skip in that case.
				result.SkippedObjects++
				continue
			}

			// Re-wrap the DEK for this object
			if err := kr.rotateObjectWithMetadata(ctx, obj, rawMeta); err != nil {
				if errors.Is(err, ErrAlreadyUsingActiveKey) {
					// Object is already using the active key fingerprint.
					// Nothing to re-wrap, but LastKey still advances past it so
					// a resumed rotation does not re-inspect it.
					result.SkippedObjects++
					kr.checkpointState(obj.Key, false)
					continue
				}
				if errors.Is(err, ErrCopyObjectTooLarge) {
					// Oversized objects cannot be re-wrapped via CopyObject.
					// Enumerate them as exceptions (not silently skipped) and
					// advance LastKey past them so resume doesn't re-report them.
					result.Exceptions++
					result.ExceptionKeys = append(result.ExceptionKeys, obj.Key)
					log.Printf("rotation exception: %s cannot be re-wrapped via CopyObject: %v", obj.Key, err)
					kr.checkpointState(obj.Key, false)
					continue
				}
				// A cancelled or expired context is not an object-level
				// failure: the re-wrap did not happen, so LastKey must not
				// advance past it or a resumed rotation would skip an object
				// that is still old-wrapped. Stop here as interrupted.
				if ctxErr := ctx.Err(); ctxErr != nil {
					return kr.abortInterrupted(result, ctxErr, startTime)
				}
				log.Printf("Warning: failed to rotate key for %s: %v", obj.Key, err)
				// Continue with other objects - rotation is best-effort
			}

			result.ProcessedObjects++

			// Checkpoint after every object: a rotation killed mid-walk loses
			// at most the object that was in flight.
			kr.checkpointState(obj.Key, true)
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
	return kr.rotateObjectWithMetadata(ctx, obj, nil)
}

func (kr *KeyRotator) rotateObjectWithMetadata(ctx context.Context, obj backend.ObjectInfo, rawMeta map[string]string) error {
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
	var err error
	if rawMeta == nil {
		rawMeta, err = kr.objectMetadata(ctx, obj)
	}
	if err != nil {
		return err
	}

	// Parse ARMOR metadata to get the current fingerprint
	armorMeta, ok := backend.ParseARMORMetadata(rawMeta)
	if !ok {
		return fmt.Errorf("object %s is not ARMOR-encrypted", obj.Key)
	}

	// If targetFingerprint is set, skip objects already using that fingerprint
	// (they're already wrapped with the active key)
	if kr.targetFingerprint != "" && armorMeta.MEKFingerprint == kr.targetFingerprint {
		return ErrAlreadyUsingActiveKey
	}

	// Resolve the current wrapped DEK. Prefer the manifest fast-path (avoids
	// re-parsing headers); fall back to parsing the raw metadata.
	oldWrappedDEK := kr.wrappedDEKFromManifest(obj.Key)
	if oldWrappedDEK == nil {
		oldWrappedDEK = armorMeta.WrappedDEK
	}

	// Unwrap DEK with fingerprint-based lookup
	// For fingerprint-based rotation, we use the ring keys to unwrap
	lookupMEK := func(keyID, fingerprint string) ([]byte, bool) {
		// Check old ring keys
		if kr.oldRing != nil {
			for _, ringEntry := range kr.oldRing {
				if ringEntry.Fingerprint == fingerprint {
					return ringEntry.MEK, true
				}
			}
		}
		// For legacy rotation, try oldMEK
		if kr.oldMEK != nil && fingerprint == crypto.MEKFingerprint(kr.oldMEK) {
			return kr.oldMEK, true
		}
		return nil, false
	}

	legacyFallback := func(wrappedDEK []byte) ([]byte, error) {
		// Try old ring keys first
		if kr.oldRing != nil {
			for _, ringEntry := range kr.oldRing {
				dek, err := crypto.UnwrapDEK(ringEntry.MEK, wrappedDEK)
				if err == nil {
					return dek, nil
				}
			}
		}
		// For legacy rotation, try oldMEK
		if kr.oldMEK != nil {
			dek, err := crypto.UnwrapDEK(kr.oldMEK, wrappedDEK)
			if err == nil {
				return dek, nil
			}
		}
		return nil, fmt.Errorf("no old key can unwrap DEK")
	}

	wrappedDEKStr := base64.StdEncoding.EncodeToString(oldWrappedDEK)
	if armorMeta.MEKFingerprint != "" {
		wrappedDEKStr = fmt.Sprintf("v2:%s:%s", armorMeta.MEKFingerprint, wrappedDEKStr)
	}

	dek, _, err := crypto.UnwrapDEKByFingerprint(wrappedDEKStr, lookupMEK, legacyFallback)
	if err != nil {
		return fmt.Errorf("failed to unwrap DEK with old MEK: %w", err)
	}

	// Re-wrap DEK with new MEK
	// Wrap with new MEK and encode fingerprint in v2 format
	newWrappedDEKStr, err := crypto.WrapDEKWithFingerprint(kr.newMEK, dek)
	if err != nil {
		return fmt.Errorf("failed to wrap DEK with new MEK: %w", err)
	}

	// Parse v2 format to extract fingerprint and wrapped DEK bytes
	parts := strings.SplitN(newWrappedDEKStr, ":", 3)
	if len(parts) != 3 || parts[0] != "v2" {
		return fmt.Errorf("invalid wrapped DEK format from WrapDEKWithFingerprint")
	}
	newMekFingerprint := parts[1]
	newWrappedDEK, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		return fmt.Errorf("failed to decode wrapped DEK: %w", err)
	}

	// Clone the full raw metadata and overwrite ONLY the wrapped-DEK. This
	// preserves x-amz-meta-armor-multipart, x-amz-meta-armor-part-size,
	// x-amz-meta-armor-key-id, plaintext-sha256, etag, and any non-ARMOR user
	// metadata across the REPLACE copy.
	newMeta := make(map[string]string, len(rawMeta))
	for k, v := range rawMeta {
		newMeta[k] = v
	}
	// Update wrapped DEK with new MEK fingerprint in v2 format
	base64Wrapped := base64.StdEncoding.EncodeToString(newWrappedDEK)
	newMeta[armorMetaWrappedDEK] = fmt.Sprintf("v2:%s:%s", newMekFingerprint, base64Wrapped)

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

// checkpointState advances LastKey to key and persists the rotation state.
//
// It runs after every object the walk has moved past — a completed re-wrap, an
// already-active-fingerprint skip, or an oversized exception — so the persisted
// LastKey is always a true resume point. Persisting per object replaces the old
// every-100-objects cadence, which threw away up to 99 completed re-wraps on a
// kill. The cost is one small state Put per object, negligible next to the
// CopyObject each re-wrap already performs and to the re-listing a lost
// checkpoint forces on the next attempt.
func (kr *KeyRotator) checkpointState(key string, processed bool) {
	kr.stateMu.Lock()
	kr.state.LastKey = key
	kr.state.LastUpdated = time.Now()
	if processed {
		kr.state.ProcessedObjects++
	}
	kr.stateMu.Unlock()

	// Deliberately not the caller's context: the checkpoint exists to survive
	// that context being cancelled, so it must not die with it.
	if err := kr.saveState(context.Background()); err != nil {
		log.Printf("Warning: failed to save rotation state: %v", err)
	}
}

// abortInterrupted records that the walk stopped on a cancelled context — the
// rotation outlived its request budget — and persists the state so the next
// invocation resumes from the last checkpoint instead of restarting the bucket
// walk from the beginning.
func (kr *KeyRotator) abortInterrupted(result *RotationResult, cause error, startTime time.Time) (*RotationResult, error) {
	result.Status = "interrupted"
	result.ErrorMessage = cause.Error()
	result.Duration = time.Since(startTime)

	kr.stateMu.Lock()
	kr.state.Status = "interrupted"
	kr.state.ErrorMessage = cause.Error()
	result.TotalObjects = kr.state.TotalObjects
	kr.stateMu.Unlock()

	// The caller's context is already done, so this save uses a background one:
	// losing the checkpoint here is exactly what forced a from-scratch restart.
	kr.saveState(context.Background()) // Best effort save

	return result, cause
}

// isResumableRotationStatus reports whether a persisted rotation state may be
// adopted by a later invocation. "in_progress" means the process died before it
// could record an outcome; "interrupted" means the walk stopped on a cancelled
// context. Both leave a LastKey that is a valid resume point. "completed" and
// "failed" are deliberately not resumable: the first is finished, and the second
// is an operator-visible terminal condition rather than one to silently resume
// past.
func isResumableRotationStatus(status string) bool {
	return status == "in_progress" || status == "interrupted"
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
	if kr.targetKeyID != "" {
		rotationID = fmt.Sprintf("%s-%s-%s-%d",
			hex.EncodeToString(oldMEKHash[:8]),
			hex.EncodeToString(newMEKHash[:8]),
			kr.targetKeyID,
			time.Now().Unix())
	}

	kr.state = &RotationState{
		ID:          rotationID,
		OldMEKHash:  hex.EncodeToString(oldMEKHash[:8]),
		NewMEKHash:  hex.EncodeToString(newMEKHash[:8]),
		TargetKeyID: kr.targetKeyID,
		StartTime:   time.Now(),
		LastUpdated: time.Now(),
		Status:      "initialized",
	}

	// Try to load existing state
	existingState, err := kr.loadState(ctx)
	if err == nil && existingState != nil {
		// Check if this is a continuation of the same rotation. "interrupted"
		// is adopted exactly like "in_progress": the cancel path persists its
		// LastKey, and refusing to adopt it would restart the walk from the
		// beginning of the bucket on every retry — a rotation that outlives its
		// request budget could then never reach the end of a real bucket.
		if existingState.OldMEKHash == kr.state.OldMEKHash &&
			existingState.NewMEKHash == kr.state.NewMEKHash &&
			existingState.TargetKeyID == kr.state.TargetKeyID &&
			isResumableRotationStatus(existingState.Status) {
			kr.state = existingState
			log.Printf("Resuming %s rotation from key: %s", existingState.Status, existingState.LastKey)
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
