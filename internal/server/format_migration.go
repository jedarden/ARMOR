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
	"fmt"
	"io"
	"log"
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
	armorMetaPlaintextSize  = "x-amz-meta-armor-plaintext-size"
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
	Key     string `json:"key"`
	Reason  string `json:"reason"`
	Time    time.Time `json:"time"`
	Details string `json:"details,omitempty"`
}

// ObjectClassification provides detailed counts by source format, layout, and outcome.
type ObjectClassification struct {
	// By source version
	V1SinglePut   int `json:"v1_single_put"`
	V1Multipart   int `json:"v1_multipart"`
	V2SinglePut   int `json:"v2_single_put"`
	V2Multipart   int `json:"v2_multipart"`
	V3            int `json:"v3"` // Objects already at target version
	NonARMOR      int `json:"non_armor"`
	Malformed     int `json:"malformed"`
	Contradictory int `json:"contradictory"`

	// By size class (bytes)
	SizeLessThan1MB    int `json:"size_lt_1mb"`
	Size1MBTo10MB      int `json:"size_1mb_to_10mb"`
	Size10MBTo100MB    int `json:"size_10mb_to_100mb"`
	Size100MBTo1GB     int `json:"size_100mb_to_1gb"`
	Size1GBTo10GB      int `json:"size_1gb_to_10gb"`
	SizeGreater10GB    int `json:"size_gt_10gb"`

	// By MEK fingerprint (for rotation visibility)
	ByKeyFingerprint map[string]int `json:"by_key_fingerprint,omitempty"`

	// By outcome
	OutcomeProcessed  int `json:"outcome_processed"`
	OutcomeSkipped    int `json:"outcome_skipped"`
	OutcomeFailed     int `json:"outcome_failed"`
	OutcomeIntegrityFailed int `json:"outcome_integrity_failed"`
}

// MigrationResult contains the result of a format migration operation.
type MigrationResult struct {
	TotalObjects     int `json:"total_objects"`
	ProcessedObjects int `json:"processed_objects"`
	SkippedObjects   int `json:"skipped_objects"`
	FailedObjects    int `json:"failed_objects"`
	Failures         []MigrationFailure `json:"failures,omitempty"`
	Duration         time.Duration      `json:"duration"`
	Status           string             `json:"status"`
	ErrorMessage     string             `json:"error_message,omitempty"`
	DryRun           bool               `json:"dry_run"`
}

// FormatMigrator handles format migration operations.
type FormatMigrator struct {
	backend  backend.Backend
	bucket   string
	mek      []byte // Master encryption key
	keyID    string
	// currentWriteVersion is the target version for migration
	currentWriteVersion uint8
	// includeVersions are the source versions to migrate
	includeVersions []string
	// idx is the manifest index used to skip HeadObject calls
	idx *manifest.Index

	// state tracks migration progress
	state     *MigrationState
	stateMu   sync.Mutex
	statePath string // .armor/migration-state.json
}

// NewFormatMigrator creates a new format migrator.
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
			if len(obj.Key) >= 7 && obj.Key[:7] == ".armor/" {
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
				result.FailedObjects++
				result.Failures = append(result.Failures, fm.recordFailure(obj.Key, fmt.Sprintf("failed to get metadata: %v", err)))
				fm.stateMu.Lock()
				fm.state.FailedObjects++
				fm.state.Failures = append(fm.state.Failures, fm.recordFailure(obj.Key, fmt.Sprintf("failed to get metadata: %v", err)))
				fm.stateMu.Unlock()
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
					fm.advanceCursor(obj.Key)
					continue
				}
				if _, err := fmt.Sscanf(armorVersion, "%d", &version); err != nil {
					log.Printf("Warning: object %s has invalid version '%s', skipping", obj.Key, armorVersion)
					result.SkippedObjects++
					fm.advanceCursor(obj.Key)
					continue
				}
			}

			// Check if this object should be skipped:
			// First check if version is in the include list (not a source version we want to migrate from)
			// Then check if version is already at target version (for versions in the include list)
			// This order ensures each skipped object is counted exactly once with a clear reason
			if !fm.shouldMigrateVersion(uint8(version)) {
			// Version is not in the include list - skip it
			result.SkippedObjects++
			fm.advanceCursor(obj.Key)
			continue
		}
		// Version is in the include list - now check if already at target version
		if uint8(version) == fm.currentWriteVersion {
			// Object is already at target version - skip it
			result.SkippedObjects++
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
				result.FailedObjects++
				result.Failures = append(result.Failures, fm.recordFailure(obj.Key, fmt.Sprintf("migration failed: %v", err)))
				fm.stateMu.Lock()
				fm.state.FailedObjects++
				fm.state.Failures = append(fm.state.Failures, fm.recordFailure(obj.Key, fmt.Sprintf("migration failed: %v", err)))
				fm.stateMu.Unlock()
				// Continue with other objects - migration is best-effort
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
	// Preserve cumulative counts from previous runs
	// result.* contains only current run increments, state.* contains cumulative totals
	fm.state.ProcessedObjects += result.ProcessedObjects
	fm.state.SkippedObjects += result.SkippedObjects
	fm.state.FailedObjects += result.FailedObjects
	// Merge failures (append current run failures to existing list)
	fm.state.Failures = append(fm.state.Failures, result.Failures...)
	fm.stateMu.Unlock()

	if err := fm.saveState(ctx); err != nil {
		log.Printf("Warning: failed to save final migration state: %v", err)
	}

	result.TotalObjects = fm.state.TotalObjects
	result.ProcessedObjects = fm.state.ProcessedObjects
	result.SkippedObjects = fm.state.SkippedObjects
	result.FailedObjects = fm.state.FailedObjects
	result.Failures = fm.state.Failures
	result.Duration = time.Since(startTime)
	result.Status = "completed"


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
		return fmt.Errorf("SHA-256 mismatch after migration: expected %s, got %s",
			expectedSHA, verifyPlaintextSHA)
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
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

// decryptMultipartObject decrypts a multipart object.
// Multipart objects have no embedded envelope header; the HMAC table is stored
// in a sidecar at .armor/hmac/<sha256(key)>.
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
		return nil, fmt.Errorf("failed to decrypt with HMAC: %w", err)
	}

	return plaintext, nil
}

// loadHMCTableFromSidecar loads the HMAC table for a multipart object from its sidecar.
// The sidecar is stored at .armor/hmac/<sha256(key)>.
func (fm *FormatMigrator) loadHMCTableFromSidecar(key string) ([]byte, error) {
	// Compute SHA-256 of the key to get the sidecar path
	keySHA := sha256.Sum256([]byte(key))
	sidecarPath := fmt.Sprintf(".armor/hmac/%x", keySHA)

	// Read the sidecar
	reader, _, err := fm.backend.GetDirect(context.Background(), fm.bucket, sidecarPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read HMAC sidecar: %w", err)
	}
	defer reader.Close()

	// Read the entire HMAC table
	hmacTable, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read HMAC table: %w", err)
	}

	return hmacTable, nil
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
	if totalSize % partSize != 0 {
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

// advanceCursor advances the migration cursor to the given key.
func (fm *FormatMigrator) advanceCursor(key string) {
	fm.stateMu.Lock()
	defer fm.stateMu.Unlock()
	fm.state.LastKey = key
	fm.state.LastUpdated = time.Now()
}

// recordFailure creates a failure record for a failed migration.
// The returned record is appended to result.Failures by the caller,
// and also immediately appended to fm.state.Failures to ensure persistence
// even if the migration is interrupted before completion.
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
		ID:                 migrationID,
		StartTime:          time.Now(),
		LastUpdated:        time.Now(),
		Status:             "initialized",
		IncludeVersions:    fm.includeVersions,
		CurrentWriteVersion: fm.currentWriteVersion,
		DryRun:             dryRun,
		Concurrency:        concurrency,
	}

	// Try to load existing state
	existingState, err := fm.loadState(ctx)
	if err == nil && existingState != nil {
		// Check if this is a continuation of the same migration
		if existingState.Status == "in_progress" {
			fm.state = existingState
			log.Printf("Resuming migration from key: %s", existingState.LastKey)
		}
	}

	return nil
}

// loadState loads the migration state from storage.
func (fm *FormatMigrator) loadState(ctx context.Context) (*MigrationState, error) {
	reader, _, err := fm.backend.GetDirect(ctx, fm.bucket, fm.statePath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read state: %w", err)
	}

	var state MigrationState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state: %w", err)
	}

	return &state, nil
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
			// Note: .armor/ objects are already filtered by the backend's List method
			// (for MockBackend in tests, this is done in the List implementation)

			// Skip internal ARMOR objects - these are not counted in TotalObjects
			// and are also not counted as SkippedObjects (they never enter the pipeline)
			if len(obj.Key) >= 7 && obj.Key[:7] == ".armor/" {
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

// classifyObject determines the classification category for an object.
// Returns the category name and whether the object should be migrated.
func (fm *FormatMigrator) classifyObject(rawMeta map[string]string, objSize int64) (category string, shouldMigrate bool) {
	// Check if object has ARMOR metadata
	armorVersion := rawMeta[armorMetaVersion]
	if armorVersion == "" {
		return "non_armor", false
	}

	// Parse version
	var version int
	if _, err := fmt.Sscanf(armorVersion, "%d", &version); err != nil {
		// Has ARMOR metadata header but invalid version - malformed
		return "malformed", false
	}

	// Check if this is a V3 object (already at target)
	if version == 3 {
		return "v3", false
	}

	// Check if version is in the include list
	if !fm.shouldMigrateVersion(uint8(version)) {
		// Version not in include list - will be skipped
		return fmt.Sprintf("v%d", version), false
	}

	// Check for multipart flag
	isMultipart := rawMeta[armorMetaMultipart] == "true"

	// Classify by version and layout
	switch version {
	case 1:
		if isMultipart {
			return "v1_multipart", true
		}
		return "v1_single_put", true
	case 2:
		if isMultipart {
			return "v2_multipart", true
		}
		return "v2_single_put", true
	default:
		// Unknown version - malformed
		return "malformed", false
	}
}

// classifyBySize returns the size classification for an object.
func classifyBySize(size int64) string {
	switch {
	case size < 1*1024*1024:
		return "size_lt_1mb"
	case size < 10*1024*1024:
		return "size_1mb_to_10mb"
	case size < 100*1024*1024:
		return "size_10mb_to_100mb"
	case size < 1*1024*1024*1024:
		return "size_100mb_to_1gb"
	case size < 10*1024*1024*1024:
		return "size_1gb_to_10gb"
	default:
		return "size_gt_10gb"
	}
}

// getMEKFingerprint extracts the MEK fingerprint from wrapped DEK metadata.
// Returns the fingerprint or "unknown" if not found.
func getMEKFingerprint(rawMeta map[string]string) string {
	wrappedDEK := rawMeta[armorMetaWrappedDEK]
	if wrappedDEK == "" {
		return "unknown"
	}

	// Check for v2 format: "v2:<fingerprint>:<base64>"
	if strings.HasPrefix(wrappedDEK, "v2:") {
		parts := strings.SplitN(wrappedDEK, ":", 3)
		if len(parts) >= 2 {
			return parts[1] // Fingerprint is the second part
		}
	}

	// Legacy format - no fingerprint
	return "legacy"
}

// updateClassification updates the classification counts based on object metadata.
func (fm *FormatMigrator) updateClassification(rawMeta map[string]string, objSize int64, outcome string) {
	fm.stateMu.Lock()
	defer fm.stateMu.Unlock()

	if fm.state.Classification.ByKeyFingerprint == nil {
		fm.state.Classification.ByKeyFingerprint = make(map[string]int)
	}

	// Get category and determine if migration was attempted
	category, shouldMigrate := fm.classifyObject(rawMeta, objSize)

	// Update category counts
	switch category {
	case "v1_single_put":
		fm.state.Classification.V1SinglePut++
	case "v1_multipart":
		fm.state.Classification.V1Multipart++
	case "v2_single_put":
		fm.state.Classification.V2SinglePut++
	case "v2_multipart":
		fm.state.Classification.V2Multipart++
	case "v3":
		fm.state.Classification.V3++
	case "non_armor":
		fm.state.Classification.NonARMOR++
	case "malformed":
		fm.state.Classification.Malformed++
	case "contradictory":
		fm.state.Classification.Contradictory++
	}

	// Update size classification
	sizeClass := classifyBySize(objSize)
	switch sizeClass {
	case "size_lt_1mb":
		fm.state.Classification.SizeLessThan1MB++
	case "size_1mb_to_10mb":
		fm.state.Classification.Size1MBTo10MB++
	case "size_10mb_to_100mb":
		fm.state.Classification.Size10MBTo100MB++
	case "size_100mb_to_1gb":
		fm.state.Classification.Size100MBTo1GB++
	case "size_1gb_to_10gb":
		fm.state.Classification.Size1GBTo10GB++
	case "size_gt_10gb":
		fm.state.Classification.SizeGreater10GB++
	}

	// Update MEK fingerprint count
	fingerprint := getMEKFingerprint(rawMeta)
	if shouldMigrate {
		fm.state.Classification.ByKeyFingerprint[fingerprint]++
	}

	// Update outcome counts
	switch outcome {
	case "processed":
		fm.state.Classification.OutcomeProcessed++
	case "skipped":
		fm.state.Classification.OutcomeSkipped++
	case "failed":
		fm.state.Classification.OutcomeFailed++
	case "integrity_failed":
		fm.state.Classification.OutcomeIntegrityFailed++
	}
}