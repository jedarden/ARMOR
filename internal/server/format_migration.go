// Package server provides format migration functionality for ARMOR.
package server

import (
	"bytes"
	"context"
	cryptoRand "crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
	"github.com/jedarden/armor/internal/manifest"
)

// armor metadata header keys used by migration. These must match the keys
// used by the encrypt/decrypt paths to ensure metadata consistency.
const (
	armorMetaVersion       = "x-amz-meta-armor-version"
	armorMetaWrappedDEK    = "x-amz-meta-armor-wrapped-dek"
	armorMetaIV            = "x-amz-meta-armor-iv"
	armorMetaBlockSize     = "x-amz-meta-armor-block-size"
	armorMetaMultipart     = "x-amz-meta-armor-multipart"
	armorMetaPartSize      = "x-amz-meta-armor-part-size"
	armorMetaPlaintextSize = "x-amz-meta-armor-plaintext-size"
	armorMetaPlaintextSHA  = "x-amz-meta-armor-sha256"
	armorMetaContentType   = "x-amz-meta-armor-content-type"
	armorMetaETag          = "x-amz-meta-armor-etag"
	armorMetaKeyID         = "x-amz-meta-armor-key-id"
	armorMetaCompressed    = "x-amz-meta-armor-compressed"
	armorMetaCompression   = "x-amz-meta-armor-compression-type"
)

// MigrationState tracks the progress of a format migration operation.
type MigrationState struct {
	// ID is a unique identifier for this migration
	ID string `json:"id"`
	// StartTime is when the migration began
	StartTime time.Time `json:"start_time"`
	// LastUpdated is when the state was last updated
	LastUpdated time.Time `json:"last_updated"`
	// Status is the current status: "in_progress", "completed", "failed", "interrupted"
	Status string `json:"status"`
	// TotalObjects is the total number of objects to migrate
	TotalObjects int `json:"total_objects"`
	// ProcessedObjects is the number of objects processed so far
	ProcessedObjects int `json:"processed_objects"`
	// SkippedObjects is the number of objects skipped (wrong version, not ARMOR, etc.)
	SkippedObjects int `json:"skipped_objects"`
	// FailedObjects is the number of objects that failed migration
	FailedObjects int `json:"failed_objects"`
	// LastKey is the last object key processed (for resumption)
	LastKey string `json:"last_key"`
	// IncludeVersions are the versions to migrate (e.g., ["1", "2"])
	IncludeVersions []string `json:"include_versions"`
	// CurrentWriteVersion is the target version for migration
	CurrentWriteVersion uint8 `json:"current_write_version"`
	// DryRun indicates if this is a dry run (no actual migration)
	DryRun bool `json:"dry_run"`
	// Concurrency is the number of concurrent workers
	Concurrency int `json:"concurrency"`
	// Failures records failed objects with reasons
	Failures []MigrationFailure `json:"failures,omitempty"`
	// ErrorMessage contains any error that occurred
	ErrorMessage string `json:"error_message,omitempty"`
	// Detailed classification counts
	Classification ObjectClassification `json:"classification,omitempty"`
}

// MigrationFailure records a failed migration attempt.
type MigrationFailure struct {
	Key     string    `json:"key"`
	Reason  string    `json:"reason"`
	Time    time.Time `json:"time"`
	Details string    `json:"details,omitempty"`
}

// ObjectClassification provides detailed counts by source format, layout, and outcome.
//
// Every walked object lands in exactly one bucket per dimension, so the
// source, size and outcome buckets each sum to the number of classified
// objects (see Total and Balanced). The fingerprint dimension only counts
// objects that carry extractable key material, so it may total less.
type ObjectClassification struct {
	// By source version and layout. Layout collapses for v3: objects at or
	// beyond the target version are never re-encrypted, so their layout is
	// not separately reported.
	V1SinglePut   int `json:"v1_single_put"`
	V1Multipart   int `json:"v1_multipart"`
	V2SinglePut   int `json:"v2_single_put"`
	V2Multipart   int `json:"v2_multipart"`
	V3            int `json:"v3"` // v3-or-newer objects (already at/beyond target)
	NonARMOR      int `json:"non_armor"`
	Malformed     int `json:"malformed"`     // armor-version header present but unparseable, or metadata unreadable
	Contradictory int `json:"contradictory"` // claims an ARMOR version but carries no wrapped DEK; migration fails

	// By size class (bytes)
	SizeLessThan1MB int `json:"size_lt_1mb"`
	Size1MBTo10MB   int `json:"size_1mb_to_10mb"`
	Size10MBTo100MB int `json:"size_10mb_to_100mb"`
	Size100MBTo1GB  int `json:"size_100mb_to_1gb"`
	Size1GBTo10GB   int `json:"size_1gb_to_10gb"`
	SizeGreater10GB int `json:"size_gt_10gb"`

	// By MEK fingerprint (for rotation visibility)
	ByKeyFingerprint map[string]int `json:"by_key_fingerprint,omitempty"`

	// By outcome
	OutcomeProcessed       int `json:"outcome_processed"`
	OutcomeSkipped         int `json:"outcome_skipped"`
	OutcomeFailed          int `json:"outcome_failed"`
	OutcomeIntegrityFailed int `json:"outcome_integrity_failed"`
}

// sourceKind enumerates the source-format buckets a walked object falls into.
// The buckets are a property of the object alone, not of the run's include
// list, so the same inventory is reported whatever subset is migrated.
type sourceKind int

const (
	srcV1SinglePut   sourceKind = iota // version 1, single-PUT layout
	srcV1Multipart                     // version 1, multipart layout
	srcV2SinglePut                     // version 2, single-PUT layout
	srcV2Multipart                     // version 2, multipart layout
	srcV3Plus                          // version 3 or newer (at/beyond target)
	srcNonARMOR                        // no ARMOR version header
	srcMalformed                       // version header present but unparseable, or metadata unreadable
	srcContradictory                   // claims an ARMOR version but carries no wrapped DEK
)

// outcomeKind enumerates what the migration walk did to one object.
type outcomeKind int

const (
	outcomeProcessed       outcomeKind = iota // migration candidate attempted successfully (dry runs included)
	outcomeSkipped                            // not a candidate: wrong version, at target, non-ARMOR, malformed
	outcomeFailed                             // candidate whose migration attempt errored
	outcomeIntegrityFailed                    // candidate that failed HMAC or SHA-256 verification
)

// ErrIntegrityVerification marks migration failures caused by content
// verification (HMAC or plaintext SHA-256 mismatch) rather than transport,
// key or metadata errors. Objects failing with this error are counted under
// OutcomeIntegrityFailed instead of OutcomeFailed; both still count as
// FailedObjects.
var ErrIntegrityVerification = errors.New("integrity verification failed")

// sizeBucket constants for the classification size dimension (bytes).
const (
	size1MB   = 1 << 20
	size10MB  = 10 << 20
	size100MB = 100 << 20
	size1GB   = 1 << 30
	size10GB  = 10 << 30
)

// legacyFingerprintLabel is the by_key_fingerprint bucket for objects whose
// wrapped DEK predates the fingerprinted v2 wrapping, which records no MEK
// identity at all.
const legacyFingerprintLabel = "legacy"

// record folds one walked object into the classification counters: exactly
// one source bucket, one size bucket and one outcome bucket are incremented.
// fingerprint is a 16-hex MEK fingerprint extracted from a v2-style wrapped
// DEK, legacyFingerprintLabel for v1-style wrapping, or empty when the object
// carries no key material.
func (c *ObjectClassification) record(src sourceKind, sizeBytes int64, fingerprint string, outcome outcomeKind) {
	switch src {
	case srcV1SinglePut:
		c.V1SinglePut++
	case srcV1Multipart:
		c.V1Multipart++
	case srcV2SinglePut:
		c.V2SinglePut++
	case srcV2Multipart:
		c.V2Multipart++
	case srcV3Plus:
		c.V3++
	case srcNonARMOR:
		c.NonARMOR++
	case srcMalformed:
		c.Malformed++
	case srcContradictory:
		c.Contradictory++
	}

	switch {
	case sizeBytes < size1MB:
		c.SizeLessThan1MB++
	case sizeBytes < size10MB:
		c.Size1MBTo10MB++
	case sizeBytes < size100MB:
		c.Size10MBTo100MB++
	case sizeBytes < size1GB:
		c.Size100MBTo1GB++
	case sizeBytes < size10GB:
		c.Size1GBTo10GB++
	default:
		c.SizeGreater10GB++
	}

	if fingerprint != "" {
		if c.ByKeyFingerprint == nil {
			c.ByKeyFingerprint = make(map[string]int)
		}
		c.ByKeyFingerprint[fingerprint]++
	}

	switch outcome {
	case outcomeProcessed:
		c.OutcomeProcessed++
	case outcomeSkipped:
		c.OutcomeSkipped++
	case outcomeFailed:
		c.OutcomeFailed++
	case outcomeIntegrityFailed:
		c.OutcomeIntegrityFailed++
	}
}

// Total returns the number of classified objects: the sum of the source
// buckets, the dimension every walked object lands in exactly once.
func (c *ObjectClassification) Total() int {
	return c.V1SinglePut + c.V1Multipart + c.V2SinglePut + c.V2Multipart +
		c.V3 + c.NonARMOR + c.Malformed + c.Contradictory
}

// Balanced reports whether the per-object dimensions agree: the source, size
// and outcome buckets must each sum to the same count. The fingerprint
// dimension is excluded because it only covers objects carrying key
// material. A classification is balanced exactly when every recorded object
// incremented every dimension once (see record).
func (c *ObjectClassification) Balanced() bool {
	sizeTotal := c.SizeLessThan1MB + c.Size1MBTo10MB + c.Size10MBTo100MB +
		c.Size100MBTo1GB + c.Size1GBTo10GB + c.SizeGreater10GB
	outcomeTotal := c.OutcomeProcessed + c.OutcomeSkipped + c.OutcomeFailed + c.OutcomeIntegrityFailed
	return sizeTotal == c.Total() && outcomeTotal == c.Total()
}

// Summary renders the classification as a human-readable multi-line report,
// one line per dimension with a trailing per-dimension total. Fingerprint
// keys are sorted so repeated runs of the same inventory render identically.
func (c *ObjectClassification) Summary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "source:    v1-single=%d v1-multipart=%d v2-single=%d v2-multipart=%d v3=%d non-armor=%d malformed=%d contradictory=%d (total %d)\n",
		c.V1SinglePut, c.V1Multipart, c.V2SinglePut, c.V2Multipart, c.V3, c.NonARMOR, c.Malformed, c.Contradictory, c.Total())
	fmt.Fprintf(&b, "size:      <1MB=%d 1MB-10MB=%d 10MB-100MB=%d 100MB-1GB=%d 1GB-10GB=%d >10GB=%d (total %d)\n",
		c.SizeLessThan1MB, c.Size1MBTo10MB, c.Size10MBTo100MB, c.Size100MBTo1GB, c.Size1GBTo10GB, c.SizeGreater10GB,
		c.SizeLessThan1MB+c.Size1MBTo10MB+c.Size10MBTo100MB+c.Size100MBTo1GB+c.Size1GBTo10GB+c.SizeGreater10GB)

	fpTotal := 0
	fingerprints := make([]string, 0, len(c.ByKeyFingerprint))
	for fp := range c.ByKeyFingerprint {
		fingerprints = append(fingerprints, fp)
	}
	sort.Strings(fingerprints)
	b.WriteString("keys:      ")
	if len(fingerprints) == 0 {
		b.WriteString("(none)\n")
	} else {
		for i, fp := range fingerprints {
			if i > 0 {
				b.WriteByte(' ')
			}
			fmt.Fprintf(&b, "%s=%d", fp, c.ByKeyFingerprint[fp])
			fpTotal += c.ByKeyFingerprint[fp]
		}
		fmt.Fprintf(&b, " (total %d)\n", fpTotal)
	}

	fmt.Fprintf(&b, "outcome:   processed=%d skipped=%d failed=%d integrity-failed=%d (total %d)\n",
		c.OutcomeProcessed, c.OutcomeSkipped, c.OutcomeFailed, c.OutcomeIntegrityFailed,
		c.OutcomeProcessed+c.OutcomeSkipped+c.OutcomeFailed+c.OutcomeIntegrityFailed)
	return b.String()
}

// copy returns a deep copy so a snapshot handed out (e.g. on MigrationResult)
// never shares the fingerprint map with the live state.
func (c *ObjectClassification) copy() ObjectClassification {
	dup := *c
	if c.ByKeyFingerprint != nil {
		dup.ByKeyFingerprint = make(map[string]int, len(c.ByKeyFingerprint))
		for fp, n := range c.ByKeyFingerprint {
			dup.ByKeyFingerprint[fp] = n
		}
	}
	return dup
}

// MigrationResult contains the result of a format migration operation.
type MigrationResult struct {
	TotalObjects     int                `json:"total_objects"`
	ProcessedObjects int                `json:"processed_objects"`
	SkippedObjects   int                `json:"skipped_objects"`
	FailedObjects    int                `json:"failed_objects"`
	Failures         []MigrationFailure `json:"failures,omitempty"`
	Duration         time.Duration      `json:"duration"`
	Status           string             `json:"status"`
	ErrorMessage     string             `json:"error_message,omitempty"`
	DryRun           bool               `json:"dry_run"`
	// Classification is the cumulative per-dimension object count report
	// (source/layout, size bucket, key fingerprint, outcome). Machine-
	// readable via this JSON encoding; Summary renders it for humans.
	Classification ObjectClassification `json:"classification"`
}

// FormatMigrator handles format migration operations.
type FormatMigrator struct {
	backend backend.Backend
	bucket  string
	mek     []byte // Master encryption key
	keyID   string
	// currentWriteVersion is the target version for migration
	currentWriteVersion uint8
	// includeVersions are the source versions to migrate
	includeVersions []string
	// idx is the manifest index used to skip HeadObject calls
	idx *manifest.Index

	// keyPrefix is the ADR-001 shared-bucket prefix (normalized, trailing
	// slash). The backend does not apply it — callers pass prefixed keys — so
	// the migrator composes it into its own internal keys: migration state and
	// sidecar loads resolve beneath <keyPrefix>.armor/ per ADR-001's "Internal
	// Namespaces" and the ADR-003 sidecar addendum. Empty means the bucket
	// root, which is byte-for-byte the pre-2026-09-20 behavior.
	keyPrefix string

	// state tracks migration progress
	state     *MigrationState
	stateMu   sync.Mutex
	statePath string // <keyPrefix>.armor/migration-state.json
}

// NewFormatMigrator creates a new format migrator. The migrator resolves
// internal state at the bucket root; prefixed deployments must chain
// WithKeyPrefix so state lands inside the tenant namespace (a B2 key scoped
// to namePrefix <tenant>/ is denied bucket-root writes).
func NewFormatMigrator(b backend.Backend, bucket string, mek []byte, keyID string, currentWriteVersion uint8, includeVersions []string, idx *manifest.Index) *FormatMigrator {
	return &FormatMigrator{
		backend:             b,
		bucket:              bucket,
		mek:                 mek,
		keyID:               keyID,
		currentWriteVersion: currentWriteVersion,
		includeVersions:     includeVersions,
		idx:                 idx,
		statePath:           ".armor/migration-state.json",
	}
}

// WithKeyPrefix sets the ADR-001 shared-bucket prefix the migrator resolves
// its internal .armor/ namespace beneath. prefix must be normalized exactly
// as config.normalizePrefix produces — empty, or ending in exactly one slash.
//
// Migration state is progress bookkeeping, not read state for live objects,
// so only the READ side falls back to the bucket root (a migration started
// before the composition resumes instead of restarting); saves always target
// the composed location, which moves the state into the tenant namespace
// from the next save on (ADR-003 addendum).
func (fm *FormatMigrator) WithKeyPrefix(prefix string) *FormatMigrator {
	fm.keyPrefix = prefix
	fm.statePath = prefix + ".armor/migration-state.json"
	return fm
}

// isInternalKey reports whether key is an internal ARMOR object key — at the
// bucket root, or composed beneath the ADR-001 prefix (both branches stay
// live for buckets that gained their prefix after ARMOR had been writing to
// the root). Backend List already filters both; the walk keeps its own guard
// so a backend that leaks internal keys into listings cannot feed
// bookkeeping objects into the migration pipeline.
func (fm *FormatMigrator) isInternalKey(key string) bool {
	if strings.HasPrefix(key, ".armor/") {
		return true
	}
	return fm.keyPrefix != "" && strings.HasPrefix(key, fm.keyPrefix+".armor/")
}

// Migrate performs the format migration, re-encrypting all objects with the current write format.
func (fm *FormatMigrator) Migrate(ctx context.Context, dryRun bool, concurrency int) (*MigrationResult, error) {
	startTime := time.Now()

	// Initialize or load state
	if err := fm.initOrLoadState(ctx, dryRun, concurrency); err != nil {
		return nil, fmt.Errorf("failed to initialize migration state: %w", err)
	}

	fm.stateMu.Lock()
	fm.state.Status = "in_progress"
	fm.state.StartTime = startTime
	fm.state.LastUpdated = startTime
	fm.stateMu.Unlock()

	// Save initial state
	if err := fm.saveState(ctx); err != nil {
		return nil, fmt.Errorf("failed to save initial state: %w", err)
	}

	result := &MigrationResult{
		Status: "in_progress",
		DryRun: dryRun,
	}

	// Count total objects first
	if err := fm.countObjects(ctx); err != nil {
		return nil, fmt.Errorf("failed to count objects: %w", err)
	}

	// Process all objects
	var continuationToken string
	for {
		select {
		case <-ctx.Done():
			result.Status = "interrupted"
			result.ErrorMessage = ctx.Err().Error()
			fm.stateMu.Lock()
			fm.state.Status = "interrupted"
			fm.state.ErrorMessage = ctx.Err().Error()
			fm.stateMu.Unlock()
			fm.saveState(context.Background()) // Best effort save
			return result, ctx.Err()
		default:
		}

		listResult, err := fm.backend.List(ctx, fm.bucket, "", "", continuationToken, 1000)
		if err != nil {
			result.Status = "failed"
			result.ErrorMessage = err.Error()
			fm.stateMu.Lock()
			fm.state.Status = "failed"
			fm.state.ErrorMessage = err.Error()
			fm.stateMu.Unlock()
			fm.saveState(context.Background())
			return result, fmt.Errorf("failed to list objects: %w", err)
		}

		for _, obj := range listResult.Objects {
			// Skip internal ARMOR objects (these are also excluded from TotalObjects count)
			if fm.isInternalKey(obj.Key) {
				// Don't increment SkippedObjects - these were never counted in TotalObjects
				continue
			}

			// Check if we should skip this object (already processed in a previous run)
			fm.stateMu.Lock()
			if fm.state.LastKey != "" && obj.Key <= fm.state.LastKey {
				fm.stateMu.Unlock()
				continue
			}
			fm.stateMu.Unlock()

			// Get object metadata to check version
			rawMeta, err := fm.objectMetadata(ctx, obj)
			if err != nil {
				log.Printf("Warning: failed to get metadata for %s: %v", obj.Key, err)
				failure := fm.recordFailure(obj.Key, fmt.Sprintf("failed to get metadata: %v", err))
				result.FailedObjects++
				result.Failures = append(result.Failures, failure)
				// Record in state immediately so periodic saves, GetState()
				// and progress polling reflect the failure even if this run
				// is interrupted before completion.
				fm.stateMu.Lock()
				fm.state.FailedObjects++
				fm.state.Failures = append(fm.state.Failures, failure)
				fm.stateMu.Unlock()
				// ARMOR-ness could not be established: the stored size from
				// the listing is the only classification input available.
				fm.classifyObject(srcMalformed, obj.Size, "", outcomeFailed)
				fm.advanceCursor(obj.Key)
				continue
			}

			armorMeta, ok := backend.ParseARMORMetadata(rawMeta)

			// First, extract and check the version to determine if we should even attempt migration
			// Use the version from metadata (if parsing failed, try to parse version directly)
			var version int
			if armorMeta != nil {
				version = armorMeta.Version
			} else {
				// Try to parse version directly from metadata
				armorVersion := rawMeta[armorMetaVersion]
				if armorVersion == "" {
					// Not an ARMOR-encrypted object
					result.SkippedObjects++
					fm.classifyObject(srcNonARMOR, obj.Size, "", outcomeSkipped)
					fm.advanceCursor(obj.Key)
					continue
				}
				if _, err := fmt.Sscanf(armorVersion, "%d", &version); err != nil {
					log.Printf("Warning: object %s has invalid version '%s', skipping", obj.Key, armorVersion)
					result.SkippedObjects++
					fm.classifyObject(srcMalformed, obj.Size, "", outcomeSkipped)
					fm.advanceCursor(obj.Key)
					continue
				}
			}

			// Derive the per-dimension classification once, from the same
			// metadata the skip decisions below use, and record it with the
			// outcome each terminal branch settles on.
			src, sizeBytes, fingerprint := classifyListedObject(rawMeta, armorMeta, ok, version, obj.Size)

			// Check if this object should be skipped:
			// First check if version is in the include list (not a source version we want to migrate from)
			// Then check if version is already at target version (for versions in the include list)
			// This order ensures each skipped object is counted exactly once with a clear reason
			if !fm.shouldMigrateVersion(uint8(version)) {
				// Version is not in the include list - skip it
				result.SkippedObjects++
				fm.classifyObject(src, sizeBytes, fingerprint, outcomeSkipped)
				fm.advanceCursor(obj.Key)
				continue
			}
			// Version is in the include list - now check if already at target version
			if uint8(version) == fm.currentWriteVersion {
				// Object is already at target version - skip it
				result.SkippedObjects++
				fm.classifyObject(src, sizeBytes, fingerprint, outcomeSkipped)
				fm.advanceCursor(obj.Key)
				continue
			}

			// If we get here, the object is a migration candidate
			// If metadata parsing failed but version is in include list, attempt migration (will fail and be recorded)
			if !ok {
				// Has ARMOR version but invalid metadata - attempt migration, which will fail
				log.Printf("Warning: object %s has ARMOR version but invalid metadata, attempting migration (will fail)", obj.Key)
			}

			// Migrate the object
			if err := fm.migrateObject(ctx, obj, rawMeta, dryRun); err != nil {
				log.Printf("Warning: failed to migrate %s: %v", obj.Key, err)
				failure := fm.recordFailure(obj.Key, fmt.Sprintf("migration failed: %v", err))
				result.FailedObjects++
				result.Failures = append(result.Failures, failure)
				// Record in state immediately so periodic saves, GetState()
				// and progress polling reflect the failure even if this run
				// is interrupted before completion.
				fm.stateMu.Lock()
				fm.state.FailedObjects++
				fm.state.Failures = append(fm.state.Failures, failure)
				fm.stateMu.Unlock()
				if errors.Is(err, ErrIntegrityVerification) {
					fm.classifyObject(src, sizeBytes, fingerprint, outcomeIntegrityFailed)
				} else {
					fm.classifyObject(src, sizeBytes, fingerprint, outcomeFailed)
				}
				// Continue with other objects - migration is best-effort
			} else {
				fm.classifyObject(src, sizeBytes, fingerprint, outcomeProcessed)
			}

			// Increment processed counter regardless of success/failure
			result.ProcessedObjects++

			// Update cursor
			fm.advanceCursor(obj.Key)

			// Save state periodically (every 100 objects)
			if result.ProcessedObjects%100 == 0 {
				if err := fm.saveState(ctx); err != nil {
					log.Printf("Warning: failed to save migration state: %v", err)
				}
			}
		}

		if !listResult.IsTruncated {
			break
		}
		continuationToken = listResult.NextToken
	}

	// Mark migration as complete
	fm.stateMu.Lock()
	fm.state.Status = "completed"
	fm.state.LastUpdated = time.Now()
	// Preserve cumulative counts from previous runs.
	// ProcessedObjects and SkippedObjects are run-scoped (accumulated on
	// result only), so they are merged into the cumulative state here.
	fm.state.ProcessedObjects += result.ProcessedObjects
	fm.state.SkippedObjects += result.SkippedObjects
	// FailedObjects and Failures are NOT merged again: each failure is
	// applied to state immediately when it occurs, so re-merging result
	// here would double-count this run's failures.
	fm.stateMu.Unlock()

	if err := fm.saveState(ctx); err != nil {
		log.Printf("Warning: failed to save final migration state: %v", err)
	}

	result.TotalObjects = fm.state.TotalObjects
	// Report cumulative totals: the run counters above were merged into
	// state, so the caller sees totals across all runs, not just this one.
	result.ProcessedObjects = fm.state.ProcessedObjects
	result.SkippedObjects = fm.state.SkippedObjects
	result.FailedObjects = fm.state.FailedObjects
	result.Failures = fm.state.Failures
	result.Classification = fm.state.Classification.copy()
	result.Duration = time.Since(startTime)
	result.Status = "completed"

	// Human-readable count report; the same counters travel machine-readable
	// on the result JSON and the persisted/polled migration state.
	log.Printf("Migration %s classification:\n%s", result.Status, result.Classification.Summary())

	return result, nil
}

// migrateObject migrates a single object to the current write format.
func (fm *FormatMigrator) migrateObject(ctx context.Context, obj backend.ObjectInfo, rawMeta map[string]string, dryRun bool) error {
	// Validate base64 fields before attempting to parse
	// This ensures that corrupted metadata produces clear error messages
	if wrappedDEK := rawMeta[armorMetaWrappedDEK]; wrappedDEK != "" {
		// Check if it's v2 format or legacy base64
		var base64DEK string
		if len(wrappedDEK) > 4 && wrappedDEK[:3] == "v2:" {
			parts := strings.SplitN(wrappedDEK, ":", 3)
			if len(parts) == 3 && parts[0] == "v2" {
				base64DEK = parts[2]
			} else {
				return fmt.Errorf("object %s has invalid v2 wrapped DEK format: %s", obj.Key, wrappedDEK)
			}
		} else {
			base64DEK = wrappedDEK
		}
		if _, err := base64.StdEncoding.DecodeString(base64DEK); err != nil {
			return fmt.Errorf("object %s has invalid base64 in wrapped DEK: %w", obj.Key, err)
		}
	}

	if iv := rawMeta[armorMetaIV]; iv != "" {
		if _, err := base64.StdEncoding.DecodeString(iv); err != nil {
			return fmt.Errorf("object %s has invalid base64 in IV: %w", obj.Key, err)
		}
	}

	// Parse ARMOR metadata
	armorMeta, ok := backend.ParseARMORMetadata(rawMeta)
	if !ok {
		return fmt.Errorf("object %s is not ARMOR-encrypted", obj.Key)
	}

	// Get the object content
	reader, _, err := fm.backend.Get(ctx, fm.bucket, obj.Key)
	if err != nil {
		return fmt.Errorf("failed to get object: %w", err)
	}
	defer reader.Close()

	// Decrypt the object using the appropriate read path
	var plaintext []byte
	isMultipart := rawMeta[armorMetaMultipart] == "true"
	if isMultipart {
		// Multipart objects: load HMAC table from sidecar and decrypt
		plaintext, err = fm.decryptMultipartObject(armorMeta, obj.Key, reader)
	} else {
		// Single-PUT objects: envelope header embedded in object
		plaintext, err = fm.decryptSingleObject(armorMeta, reader)
	}
	if err != nil {
		return fmt.Errorf("failed to decrypt object: %w", err)
	}

	// Calculate plaintext SHA-256 for verification
	plaintextSHA := sha256.Sum256(plaintext)

	if dryRun {
		// In dry run mode, just verify we can decrypt and count
		return nil
	}

	// Check if we should use multipart upload for the re-encrypted object
	plaintextSize := len(plaintext)
	if plaintextSize > fm.multipartThreshold() {
		// Use multipart upload for large objects
		err = fm.uploadAsMultipart(ctx, obj.Key, plaintext, plaintextSHA[:], rawMeta)
		if err != nil {
			return fmt.Errorf("failed to upload as multipart: %w", err)
		}
	} else {
		// Re-encrypt as single-PUT with current write format
		ciphertext, newIV, newWrappedDEK, blockSize, mekFingerprint, err := fm.encryptAsSingle(plaintext)
		if err != nil {
			return fmt.Errorf("failed to encrypt as single: %w", err)
		}

		// Build new metadata
		newMeta := fm.buildNewMetadata(rawMeta, newIV, newWrappedDEK, blockSize, plaintextSize, plaintextSHA[:], mekFingerprint)

		// Put the re-encrypted object back
		size := int64(len(ciphertext))
		if err := fm.backend.Put(ctx, fm.bucket, obj.Key, bytesReader(ciphertext), size, newMeta); err != nil {
			return fmt.Errorf("failed to put migrated object: %w", err)
		}
	}

	// Read back and verify
	verifyReader, verifyInfo, err := fm.backend.Get(ctx, fm.bucket, obj.Key)
	if err != nil {
		return fmt.Errorf("failed to read back migrated object: %w", err)
	}
	defer verifyReader.Close()

	// Get migrated object metadata
	verifyMeta, err := fm.objectMetadata(ctx, *verifyInfo)
	if err != nil {
		return fmt.Errorf("failed to get migrated object metadata: %w", err)
	}

	// Parse migrated metadata
	verifyArmorMeta, ok := backend.ParseARMORMetadata(verifyMeta)
	if !ok {
		return fmt.Errorf("migrated object metadata is invalid")
	}

	// Verify the version was updated
	if uint8(verifyArmorMeta.Version) != fm.currentWriteVersion {
		return fmt.Errorf("version not updated: expected %d, got %d",
			fm.currentWriteVersion, verifyArmorMeta.Version)
	}

	// Verify SHA-256 by decrypting the migrated object
	_, err = fm.decryptSingleObject(verifyArmorMeta, verifyReader)
	if err != nil {
		return fmt.Errorf("failed to decrypt migrated object for verification: %w", err)
	}

	// Verify SHA-256 matches
	verifyPlaintextSHA := verifyMeta[armorMetaPlaintextSHA]
	expectedSHA := hex.EncodeToString(plaintextSHA[:])
	if verifyPlaintextSHA != expectedSHA {
		return fmt.Errorf("%w: SHA-256 mismatch after migration: expected %s, got %s",
			ErrIntegrityVerification, expectedSHA, verifyPlaintextSHA)
	}

	return nil
}

// decryptSingleObject decrypts a single-PUT object.
func (fm *FormatMigrator) decryptSingleObject(armorMeta *backend.ARMORMetadata, reader io.Reader) ([]byte, error) {
	// Unwrap DEK
	dek, err := crypto.UnwrapDEK(fm.mek, armorMeta.WrappedDEK)
	if err != nil {
		return nil, fmt.Errorf("failed to unwrap DEK: %w", err)
	}

	// Read envelope header for single-PUT objects
	headerBuf := make([]byte, crypto.HeaderSize)
	if _, err := io.ReadFull(reader, headerBuf); err != nil {
		return nil, fmt.Errorf("failed to read envelope header: %w", err)
	}

	header, err := crypto.DecodeHeader(headerBuf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode envelope header: %w", err)
	}

	// Create decryptor with appropriate version
	_, err = crypto.NewDecryptorWithVersion(dek, header.IV[:], header.BlockSize(), header.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to create decryptor: %w", err)
	}

	// Read the rest of the ciphertext
	ciphertext, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read ciphertext: %w", err)
	}

	// For single-PUT objects, the HMAC table is embedded at the end of the ciphertext
	// Split the ciphertext into data and HMAC table
	// V1 format: [encrypted blocks] + [HMAC table (SHA256 * num_blocks)]
	// V2 format: [encrypted blocks] + [HMAC table (SHA256 * num_blocks)]
	blockSize := header.BlockSize()
	if blockSize <= 0 {
		return nil, fmt.Errorf("invalid block size: %d", blockSize)
	}

	// Calculate number of blocks based on actual plaintext size from header
	plaintextSize := header.PlaintextSize
	numBlocks := (int(plaintextSize) + blockSize - 1) / blockSize
	hmacTableSize := numBlocks * 32 // SHA256 = 32 bytes per block

	if len(ciphertext) < hmacTableSize {
		return nil, fmt.Errorf("ciphertext too short to contain HMAC table: got %d, need %d", len(ciphertext), hmacTableSize)
	}

	// Split ciphertext and HMAC table
	dataSize := len(ciphertext) - hmacTableSize
	encryptedData := ciphertext[:dataSize]
	hmacTable := ciphertext[dataSize:]

	// Decrypt with HMAC verification
	decryptor, err := crypto.NewDecryptorWithVersion(dek, header.IV[:], blockSize, header.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to create decryptor: %w", err)
	}

	plaintext, err := decryptor.Decrypt(encryptedData, hmacTable)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrIntegrityVerification, err)
	}

	// Plaintext integrity: the per-block HMACs cover ciphertext bytes only and
	// are blind to counter-derivation errors, so a wrong-version decrypt can
	// verify every HMAC yet yield garbage plaintext. Enforce the header's
	// recorded plaintext SHA-256 before the plaintext is trusted. Dry-run,
	// migrate and the post-migration read-back verify all pass through here.
	if err := header.VerifyPlaintextSHA(plaintext); err != nil {
		return nil, fmt.Errorf("plaintext integrity check failed: %w", err)
	}

	return plaintext, nil
}

// decryptMultipartObject decrypts a multipart object.
// Multipart objects have no embedded envelope header; the HMAC table is stored
// in a sidecar at the location GetSidecarKey computes (ADR-003 sidecar
// addendum).
func (fm *FormatMigrator) decryptMultipartObject(armorMeta *backend.ARMORMetadata, key string, reader io.Reader) ([]byte, error) {
	// Unwrap DEK
	dek, err := crypto.UnwrapDEK(fm.mek, armorMeta.WrappedDEK)
	if err != nil {
		return nil, fmt.Errorf("failed to unwrap DEK: %w", err)
	}

	// Load HMAC table from sidecar
	hmacTable, err := fm.loadHMCTableFromSidecar(key)
	if err != nil {
		return nil, fmt.Errorf("failed to load HMAC table from sidecar: %w", err)
	}

	// Create decryptor with appropriate version
	// For multipart objects, IV is from metadata (not envelope header)
	decryptor, err := crypto.NewDecryptorWithVersion(dek, armorMeta.IV, armorMeta.BlockSize, uint8(armorMeta.Version))
	if err != nil {
		return nil, fmt.Errorf("failed to create decryptor: %w", err)
	}

	// Read the entire assembled ciphertext (all parts concatenated by B2)
	ciphertext, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read ciphertext: %w", err)
	}

	// Decrypt with HMAC verification
	plaintext, err := decryptor.Decrypt(ciphertext, hmacTable)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrIntegrityVerification, err)
	}

	return plaintext, nil
}

// loadHMCTableFromSidecar loads the HMAC table for a multipart object from its
// sidecar. key is the STORED object key the walk addressed the ciphertext by;
// the sidecar is named by the CLIENT key beneath <keyPrefix>.armor/hmac/
// (ADR-003 sidecar addendum), so the prefix comes off before hashing — with
// no prefix set the two coincide and this is a no-op. The pre-2026-09-20
// bucket-root sidecar is probed as a fallback, so sidecars written before the
// composition keep migrating.
func (fm *FormatMigrator) loadHMCTableFromSidecar(key string) ([]byte, error) {
	clientKey := strings.TrimPrefix(key, fm.keyPrefix)

	var lastErr error
	for _, sidecarPath := range backend.SidecarLocations(fm.keyPrefix, clientKey) {
		reader, _, err := fm.backend.GetDirect(context.Background(), fm.bucket, sidecarPath)
		if err != nil {
			lastErr = err
			continue
		}

		hmacTable, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read HMAC table: %w", err)
		}

		return hmacTable, nil
	}
	return nil, fmt.Errorf("failed to read HMAC sidecar: %w", lastErr)
}

// encryptAsSingle encrypts plaintext as a single-PUT object with the current write format.
func (fm *FormatMigrator) encryptAsSingle(plaintext []byte) (ciphertext, iv, wrappedDEK []byte, blockSize int, mekFingerprint string, err error) {
	// Generate new DEK
	dek := make([]byte, 32) // 256-bit DEK
	if _, err := io.ReadFull(cryptoRand.Reader, dek); err != nil {
		return nil, nil, nil, 0, "", fmt.Errorf("failed to generate DEK: %w", err)
	}

	// Wrap DEK with MEK and fingerprint
	mekFingerprint = crypto.MEKFingerprint(fm.mek)
	wrappedDEK, err = crypto.WrapDEK(fm.mek, dek)
	if err != nil {
		return nil, nil, nil, 0, "", fmt.Errorf("failed to wrap DEK: %w", err)
	}

	// Create encryptor with current write version
	blockSize = 4096 // Default block size
	iv = make([]byte, 16)
	if _, err := io.ReadFull(cryptoRand.Reader, iv); err != nil {
		return nil, nil, nil, 0, "", fmt.Errorf("failed to generate IV: %w", err)
	}

	encryptor, err := crypto.NewEncryptorWithVersion(dek, iv, blockSize, fm.currentWriteVersion)
	if err != nil {
		return nil, nil, nil, 0, "", fmt.Errorf("failed to create encryptor: %w", err)
	}

	// Encrypt
	ciphertext, hmacTable, err := encryptor.Encrypt(plaintext)
	if err != nil {
		return nil, nil, nil, 0, "", fmt.Errorf("failed to encrypt: %w", err)
	}

	// Append HMAC table to ciphertext (as single-PUT format requires)
	ciphertext = append(ciphertext, hmacTable...)

	// Create envelope header and prepend to ciphertext
	plaintextSHA := crypto.ComputePlaintextSHA256(plaintext)
	header, err := crypto.NewEnvelopeHeaderWithVersion(iv, int64(len(plaintext)), blockSize, plaintextSHA, fm.currentWriteVersion)
	if err != nil {
		return nil, nil, nil, 0, "", fmt.Errorf("failed to create envelope header: %w", err)
	}

	headerBuf, err := header.Encode()
	if err != nil {
		return nil, nil, nil, 0, "", fmt.Errorf("failed to encode envelope header: %w", err)
	}

	// Prepend header to ciphertext for storage format
	fullData := append(headerBuf, ciphertext...)

	return fullData, iv, wrappedDEK, blockSize, mekFingerprint, nil
}

// uploadAsMultipart uploads the plaintext as a multipart object with encryption.
// This is used for large objects that exceed the multipart threshold.
func (fm *FormatMigrator) uploadAsMultipart(ctx context.Context, key string, plaintext []byte, plaintextSHA []byte, oldMeta map[string]string) error {
	// Generate new DEK for this upload
	dek := make([]byte, 32)
	if _, err := io.ReadFull(cryptoRand.Reader, dek); err != nil {
		return fmt.Errorf("failed to generate DEK: %w", err)
	}

	// Wrap DEK with MEK and fingerprint
	mekFingerprint := crypto.MEKFingerprint(fm.mek)
	wrappedDEK, err := crypto.WrapDEK(fm.mek, dek)
	if err != nil {
		return fmt.Errorf("failed to wrap DEK: %w", err)
	}

	// Create encryptor with current write version
	blockSize := 65536 // 64KB default block size for multipart
	iv := make([]byte, 16)
	if _, err := io.ReadFull(cryptoRand.Reader, iv); err != nil {
		return fmt.Errorf("failed to generate IV: %w", err)
	}

	encryptor, err := crypto.NewEncryptorWithVersion(dek, iv, blockSize, fm.currentWriteVersion)
	if err != nil {
		return fmt.Errorf("failed to create encryptor: %w", err)
	}

	// Create multipart upload
	uploadID, err := fm.backend.CreateMultipartUpload(ctx, fm.bucket, key, nil)
	if err != nil {
		return fmt.Errorf("failed to create multipart upload: %w", err)
	}

	// Split plaintext into parts (default 5MB parts)
	partSize := 5 * 1024 * 1024 // 5MB
	totalSize := len(plaintext)
	var parts []backend.CompletedPart

	for partNumber := 1; partNumber*partSize <= totalSize; partNumber++ {
		start := (partNumber - 1) * partSize
		end := partNumber * partSize
		if end > totalSize {
			end = totalSize
		}
		partPlaintext := plaintext[start:end]

		// Encrypt this part
		partCiphertext, _, err := encryptor.Encrypt(partPlaintext)
		if err != nil {
			return fmt.Errorf("failed to encrypt part %d: %w", partNumber, err)
		}

		// Upload the part
		etag, err := fm.backend.UploadPart(ctx, fm.bucket, key, uploadID, int32(partNumber), bytesReader(partCiphertext), int64(len(partCiphertext)))
		if err != nil {
			return fmt.Errorf("failed to upload part %d: %w", partNumber, err)
		}

		parts = append(parts, backend.CompletedPart{
			PartNumber: int32(partNumber),
			ETag:       etag,
		})
	}

	// Handle remaining data if any
	if totalSize%partSize != 0 {
		partNumber := totalSize/partSize + 1
		start := (totalSize / partSize) * partSize
		partPlaintext := plaintext[start:]

		// Encrypt this part
		partCiphertext, _, err := encryptor.Encrypt(partPlaintext)
		if err != nil {
			return fmt.Errorf("failed to encrypt final part: %w", err)
		}

		// Upload the part
		etag, err := fm.backend.UploadPart(ctx, fm.bucket, key, uploadID, int32(partNumber), bytesReader(partCiphertext), int64(len(partCiphertext)))
		if err != nil {
			return fmt.Errorf("failed to upload final part: %w", err)
		}

		parts = append(parts, backend.CompletedPart{
			PartNumber: int32(partNumber),
			ETag:       etag,
		})
	}

	// Build metadata for the completed multipart upload
	newMeta := fm.buildNewMetadata(oldMeta, iv, wrappedDEK, blockSize, totalSize, plaintextSHA, mekFingerprint)
	newMeta[armorMetaMultipart] = "true"
	newMeta[armorMetaPartSize] = fmt.Sprintf("%d", partSize)

	// Complete the multipart upload
	_, err = fm.backend.CompleteMultipartUpload(ctx, fm.bucket, key, uploadID, parts)
	if err != nil {
		return fmt.Errorf("failed to complete multipart upload: %w", err)
	}

	// Update the object metadata with a CopyObject call to set the metadata
	// (B2 CompleteMultipartUpload doesn't support custom metadata)
	if err := fm.backend.Copy(ctx, fm.bucket, key, fm.bucket, key, newMeta, true); err != nil {
		return fmt.Errorf("failed to update object metadata: %w", err)
	}

	return nil
}

// buildNewMetadata constructs new metadata for the migrated object.
func (fm *FormatMigrator) buildNewMetadata(oldMeta map[string]string, iv, wrappedDEK []byte, blockSize, plaintextSize int, plaintextSHA []byte, mekFingerprint string) map[string]string {
	newMeta := make(map[string]string)

	// Copy all non-ARMOR metadata
	for k, v := range oldMeta {
		if !strings.HasPrefix(k, "x-amz-meta-armor-") {
			newMeta[k] = v
		}
	}

	// Set ARMOR metadata with new version
	newMeta[armorMetaVersion] = fmt.Sprintf("%d", fm.currentWriteVersion)
	// Emit v2 format if MEK fingerprint is provided, otherwise legacy base64
	if mekFingerprint != "" {
		base64Wrapped := base64.StdEncoding.EncodeToString(wrappedDEK)
		newMeta[armorMetaWrappedDEK] = fmt.Sprintf("v2:%s:%s", mekFingerprint, base64Wrapped)
	} else {
		newMeta[armorMetaWrappedDEK] = base64.StdEncoding.EncodeToString(wrappedDEK)
	}
	newMeta[armorMetaIV] = base64.StdEncoding.EncodeToString(iv)
	newMeta[armorMetaBlockSize] = fmt.Sprintf("%d", blockSize)
	newMeta[armorMetaPlaintextSize] = fmt.Sprintf("%d", plaintextSize)
	newMeta[armorMetaPlaintextSHA] = hex.EncodeToString(plaintextSHA)

	// Copy other ARMOR metadata that should be preserved
	// Note: armorMetaMultipart is NOT copied - the migration output format
	// (single-PUT vs multipart) is determined by the encrypt path chosen
	// based on plaintext size, not the input object's layout.
	if v, ok := oldMeta[armorMetaPartSize]; ok {
		newMeta[armorMetaPartSize] = v
	}
	if v, ok := oldMeta[armorMetaContentType]; ok {
		newMeta[armorMetaContentType] = v
	}
	if v, ok := oldMeta[armorMetaETag]; ok {
		newMeta[armorMetaETag] = v
	}
	if v, ok := oldMeta[armorMetaKeyID]; ok {
		newMeta[armorMetaKeyID] = v
	}
	if v, ok := oldMeta[armorMetaCompressed]; ok {
		newMeta[armorMetaCompressed] = v
	}
	if v, ok := oldMeta[armorMetaCompression]; ok {
		newMeta[armorMetaCompression] = v
	}

	return newMeta
}

// objectMetadata returns the object's full raw metadata map.
func (fm *FormatMigrator) objectMetadata(ctx context.Context, obj backend.ObjectInfo) (map[string]string, error) {
	if obj.Metadata != nil && obj.Metadata[armorMetaVersion] != "" {
		return obj.Metadata, nil
	}
	info, err := fm.backend.Head(ctx, fm.bucket, obj.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to get object metadata: %w", err)
	}
	return info.Metadata, nil
}

// shouldMigrateVersion checks if the given version should be migrated.
func (fm *FormatMigrator) shouldMigrateVersion(version uint8) bool {
	versionStr := fmt.Sprintf("%d", version)
	for _, v := range fm.includeVersions {
		if v == versionStr {
			return true
		}
	}
	return false
}

// classifyObject records one walked object in the cumulative classification
// counters on the migration state. Like the failure counters, classification
// is applied to the state immediately, so periodic saves, GetState() and the
// progress endpoint reflect objects already walked; a resumed run accumulates
// on top of the counts loaded with the state.
func (fm *FormatMigrator) classifyObject(src sourceKind, sizeBytes int64, fingerprint string, outcome outcomeKind) {
	fm.stateMu.Lock()
	defer fm.stateMu.Unlock()
	fm.state.Classification.record(src, sizeBytes, fingerprint, outcome)
}

// classifyListedObject derives the source bucket, classification size and
// MEK fingerprint for one listed object from the raw and parsed metadata the
// walk already holds. listSize is the stored size from the listing, used when
// no plaintext size is recorded. The rules mirror Migrate's own parse order
// so the classification and the skip decisions can never disagree about what
// an object is.
func classifyListedObject(rawMeta map[string]string, armorMeta *backend.ARMORMetadata, ok bool, version int, listSize int64) (sourceKind, int64, string) {
	fingerprint := ""
	if dek := rawMeta[armorMetaWrappedDEK]; dek != "" {
		switch {
		case armorMeta != nil && armorMeta.MEKFingerprint != "":
			fingerprint = armorMeta.MEKFingerprint
		case len(dek) > 4 && dek[:3] == "v2:":
			if parts := strings.SplitN(dek, ":", 3); len(parts) == 3 && parts[0] == "v2" {
				fingerprint = parts[1]
			}
		default:
			fingerprint = legacyFingerprintLabel
		}
	}

	sizeBytes := listSize
	if armorMeta != nil && armorMeta.PlaintextSize > 0 {
		sizeBytes = armorMeta.PlaintextSize
	}

	if !ok {
		// ParseARMORMetadata only returns !ok alongside a nil metadata:
		// the object carries no wrapped DEK. Callers only reach here for
		// objects that also claim an ARMOR version (Migrate's non-ARMOR
		// branch handles the no-header case), which makes them
		// self-contradictory.
		return srcContradictory, sizeBytes, fingerprint
	}

	multipart := rawMeta[armorMetaMultipart] == "true"
	switch version {
	case 1:
		if multipart {
			return srcV1Multipart, sizeBytes, fingerprint
		}
		return srcV1SinglePut, sizeBytes, fingerprint
	case 2:
		if multipart {
			return srcV2Multipart, sizeBytes, fingerprint
		}
		return srcV2SinglePut, sizeBytes, fingerprint
	default:
		return srcV3Plus, sizeBytes, fingerprint
	}
}

// advanceCursor advances the migration cursor to the given key.
func (fm *FormatMigrator) advanceCursor(key string) {
	fm.stateMu.Lock()
	defer fm.stateMu.Unlock()
	fm.state.LastKey = key
	fm.state.LastUpdated = time.Now()
}

// recordFailure creates a failure record for a failed migration.
// The caller appends the returned record to result.Failures (current run)
// and, under stateMu together with the state.FailedObjects increment, to
// fm.state.Failures (cumulative). Recording at the failure site means the
// periodic saves and an interrupted final saveState persist the failure,
// which is why the completion merge must not add this run's failures again.
func (fm *FormatMigrator) recordFailure(key, reason string) MigrationFailure {
	return MigrationFailure{
		Key:    key,
		Reason: reason,
		Time:   time.Now(),
	}
}

// initOrLoadState initializes a new migration state or loads an existing one.
func (fm *FormatMigrator) initOrLoadState(ctx context.Context, dryRun bool, concurrency int) error {
	// Compute migration ID
	migrationID := fmt.Sprintf("format-migration-%d", time.Now().Unix())

	fm.state = &MigrationState{
		ID:                  migrationID,
		StartTime:           time.Now(),
		LastUpdated:         time.Now(),
		Status:              "initialized",
		IncludeVersions:     fm.includeVersions,
		CurrentWriteVersion: fm.currentWriteVersion,
		DryRun:              dryRun,
		Concurrency:         concurrency,
	}

	// Try to load existing state
	existingState, err := fm.loadState(ctx)
	if err == nil && existingState != nil {
		// Resume an in-progress migration only between two live runs. A dry
		// run never resumes, in either direction: resuming a dry run's state
		// would leak its counters into the live run and skip every object the
		// dry run's cursor advanced past (leaving them un-migrated), and a dry
		// run that resumed any state would report counts for objects it never
		// scanned. Dry runs always scan the bucket from the beginning.
		if !dryRun && existingState.Status == "in_progress" && !existingState.DryRun {
			fm.state = existingState
			log.Printf("Resuming migration from key: %s", existingState.LastKey)
		}
	}

	return nil
}

// stateLocations returns every B2 key the migration state may live at, in
// probe order: the composed location first, then the pre-2026-09-20 bucket
// root for state written before the composition. Without a prefix the two
// coincide and exactly one is returned.
func (fm *FormatMigrator) stateLocations() []string {
	if fm.keyPrefix == "" {
		return []string{fm.statePath}
	}
	return []string{fm.statePath, ".armor/migration-state.json"}
}

// loadState loads the migration state from storage, probing the composed
// location first and then the bucket root (see stateLocations). An unreadable
// location (absent, or denied to a namePrefix-scoped B2 key) falls through to
// the next; a location that reads but fails to parse is returned as an error,
// not skipped — resuming past corrupt state would silently re-migrate.
func (fm *FormatMigrator) loadState(ctx context.Context) (*MigrationState, error) {
	var lastErr error
	for _, statePath := range fm.stateLocations() {
		reader, _, err := fm.backend.GetDirect(ctx, fm.bucket, statePath)
		if err != nil {
			lastErr = err
			continue
		}

		data, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read state: %w", err)
		}

		var state MigrationState
		if err := json.Unmarshal(data, &state); err != nil {
			return nil, fmt.Errorf("failed to parse state: %w", err)
		}

		return &state, nil
	}
	return nil, lastErr
}

// saveState saves the migration state to storage.
func (fm *FormatMigrator) saveState(ctx context.Context) error {
	fm.stateMu.Lock()
	state := *fm.state
	fm.stateMu.Unlock()

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

	if err := fm.backend.Put(ctx, fm.bucket, fm.statePath, reader, int64(len(data)), meta); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	return nil
}

// countObjects counts the total number of objects to migrate.
// This counts objects that will be processed (i.e., objects with ARMOR metadata
// where the version is in the include list). Objects that will be skipped
// (non-ARMOR objects, or ARMOR objects with a version not in the include list)
// are not counted, to match the behavior of Migrate().
func (fm *FormatMigrator) countObjects(ctx context.Context) error {
	var count int
	var continuationToken string

	for {
		listResult, err := fm.backend.List(ctx, fm.bucket, "", "", continuationToken, 1000)
		if err != nil {
			return err
		}

		for _, obj := range listResult.Objects {
			// Note: .armor/ objects are already filtered by the backend's List
			// method (for MockBackend in tests, this is done in the List
			// implementation, root location only — hence the prefix-aware guard)

			// Skip internal ARMOR objects - these are not counted in TotalObjects
			// and are also not counted as SkippedObjects (they never enter the pipeline)
			if fm.isInternalKey(obj.Key) {
				continue
			}

			// Check if object has ARMOR metadata
			rawMeta, err := fm.objectMetadata(ctx, obj)
			if err != nil {
				// Object will be skipped as FailedObjects in Migrate() - don't count here
				continue
			}

			armorMeta, ok := backend.ParseARMORMetadata(rawMeta)
			if !ok {
				// ParseARMORMetadata failed - check if this is an ARMOR object at all
				armorVersion := rawMeta[armorMetaVersion]
				if armorVersion == "" {
					// Not an ARMOR-encrypted object - will be skipped in Migrate()
					continue
				}
				// Has ARMOR version but invalid metadata - parse version directly
				var version int
				if _, err := fmt.Sscanf(armorVersion, "%d", &version); err != nil {
					// Invalid version string - will be skipped in Migrate()
					continue
				}
				// Version parsed successfully - check if it's in the include list
				if fm.shouldMigrateVersion(uint8(version)) {
					count++
				}
				// If not in include list, it will be skipped in Migrate() - don't count
			} else {
				// Parsed successfully - check if version should be counted
				// Check if version is in include list (matching Migrate's order)
				if !fm.shouldMigrateVersion(uint8(armorMeta.Version)) {
					// Version not in include list - don't count
					continue
				}
				// Skip objects already at target version (only checked for versions in include list)
				if uint8(armorMeta.Version) == fm.currentWriteVersion {
					// Already at target version - don't count as migration candidate
					continue
				}
				// Version is in include list and not at target version - count it
				count++
			}
		}

		if !listResult.IsTruncated {
			break
		}
		continuationToken = listResult.NextToken
	}

	fm.stateMu.Lock()
	fm.state.TotalObjects = count
	fm.stateMu.Unlock()

	return nil
}

// GetState returns the current migration state.
func (fm *FormatMigrator) GetState() *MigrationState {
	fm.stateMu.Lock()
	defer fm.stateMu.Unlock()
	if fm.state == nil {
		return nil
	}
	state := *fm.state
	return &state
}

// multipartThreshold returns the threshold for multipart uploads.
func (fm *FormatMigrator) multipartThreshold() int {
	return 5 * 1024 * 1024 // 5 MB default threshold
}

// Helper function to create a reader from bytes
func bytesReader(b []byte) io.Reader {
	return bytes.NewReader(b)
}
