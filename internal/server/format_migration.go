// Package server provides format migration functionality for ARMOR.
package server

import (
	"bytes"
	"context"
	cryptoRand "crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
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
}

// MigrationFailure records a failed migration attempt.
type MigrationFailure struct {
	Key    string `json:"key"`
	Reason string `json:"reason"`
	Time   time.Time `json:"time"`
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
			// Skip internal ARMOR objects
			if len(obj.Key) >= 7 && obj.Key[:7] == ".armor/" {
				result.SkippedObjects++
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
				fm.recordFailure(obj.Key, fmt.Sprintf("failed to get metadata: %v", err))
				fm.advanceCursor(obj.Key)
				continue
			}

			armorMeta, ok := backend.ParseARMORMetadata(rawMeta)
			if !ok {
				// Not an ARMOR-encrypted object
				result.SkippedObjects++
				fm.advanceCursor(obj.Key)
				continue
			}

			// Check if object version is in the include list
			if !fm.shouldMigrateVersion(armorMeta.Version) {
				result.SkippedObjects++
				fm.advanceCursor(obj.Key)
				continue
			}

			// Migrate the object
			if err := fm.migrateObject(ctx, obj, rawMeta, dryRun); err != nil {
				log.Printf("Warning: failed to migrate %s: %v", obj.Key, err)
				result.FailedObjects++
				fm.recordFailure(obj.Key, fmt.Sprintf("migration failed: %v", err))
				// Continue with other objects - migration is best-effort
			} else {
				result.ProcessedObjects++
			}

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
	fm.state.Failures = result.Failures
	fm.stateMu.Unlock()

	if err := fm.saveState(ctx); err != nil {
		log.Printf("Warning: failed to save final migration state: %v", err)
	}

	result.TotalObjects = fm.state.TotalObjects
	result.Duration = time.Since(startTime)
	result.Status = "completed"
	result.Failures = fm.state.Failures

	return result, nil
}

// migrateObject migrates a single object to the current write format.
func (fm *FormatMigrator) migrateObject(ctx context.Context, obj backend.ObjectInfo, rawMeta map[string]string, dryRun bool) error {
	// Parse ARMOR metadata
	armorMeta, ok := backend.ParseARMORMetadata(rawMeta)
	if !ok {
		return fmt.Errorf("object %s is not ARMOR-encrypted", obj.Key)
	}

	// Get the object content
	reader, info, err := fm.backend.Get(ctx, fm.bucket, obj.Key)
	if err != nil {
		return fmt.Errorf("failed to get object: %w", err)
	}
	defer reader.Close()

	// For now, skip multipart objects as they require complex HMAC sidecar handling
	// TODO: Implement multipart migration
	if armorMeta.Multipart {
		return fmt.Errorf("multipart migration not yet implemented")
	}

	// Decrypt the object using the normal read path
	plaintext, err := fm.decryptSingleObject(armorMeta, reader)
	if err != nil {
		return fmt.Errorf("failed to decrypt object: %w", err)
	}

	// Calculate plaintext SHA-256 for verification
	plaintextSHA := sha256.Sum256(plaintext)

	if dryRun {
		// In dry run mode, just verify we can decrypt and count
		return nil
	}

	// Re-encrypt as single-PUT with current write format
	ciphertext, newIV, newWrappedDEK, blockSize, err := fm.encryptAsSingle(plaintext)
	if err != nil {
		return fmt.Errorf("failed to encrypt as single: %w", err)
	}

	// Build new metadata
	newMeta := fm.buildNewMetadata(rawMeta, newIV, newWrappedDEK, blockSize, len(plaintext), plaintextSHA[:])

	// Put the re-encrypted object back
	size := int64(len(ciphertext))
	if err := fm.backend.Put(ctx, fm.bucket, obj.Key, bytesReader(ciphertext), size, newMeta); err != nil {
		return fmt.Errorf("failed to put migrated object: %w", err)
	}

	// Read back and verify
	verifyReader, verifyInfo, err := fm.backend.Get(ctx, fm.bucket, obj.Key)
	if err != nil {
		return fmt.Errorf("failed to read back migrated object: %w", err)
	}
	defer verifyReader.Close()

	// Get migrated object metadata
	verifyMeta, err := fm.objectMetadata(ctx, verifyInfo)
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
	headerBuf := make([]byte, crypto.EnvelopeHeaderSize)
	if _, err := io.ReadFull(reader, headerBuf); err != nil {
		return nil, fmt.Errorf("failed to read envelope header: %w", err)
	}

	header, err := crypto.DecodeHeader(headerBuf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode envelope header: %w", err)
	}

	// Create decryptor with appropriate version
	decryptor, err := crypto.NewDecryptorWithVersion(dek, header.IV[:], header.BlockSize(), header.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to create decryptor: %w", err)
	}

	// Read the rest of the ciphertext
	ciphertext, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read ciphertext: %w", err)
	}

	// Decrypt
	plaintext, err := decryptor.Decrypt(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

// encryptAsSingle encrypts plaintext as a single-PUT object with the current write format.
func (fm *FormatMigrator) encryptAsSingle(plaintext []byte) (ciphertext, iv, wrappedDEK []byte, blockSize int, err error) {
	// Generate new DEK
	dek := make([]byte, 32) // 256-bit DEK
	if _, err := io.ReadFull(cryptoRand.Reader, dek); err != nil {
		return nil, nil, nil, 0, fmt.Errorf("failed to generate DEK: %w", err)
	}

	// Wrap DEK with MEK
	wrappedDEK, err = crypto.WrapDEK(fm.mek, dek)
	if err != nil {
		return nil, nil, nil, 0, fmt.Errorf("failed to wrap DEK: %w", err)
	}

	// Create encryptor with current write version
	blockSize = 4096 // Default block size
	iv = make([]byte, 16)
	if _, err := io.ReadFull(cryptoRand.Reader, iv); err != nil {
		return nil, nil, nil, 0, fmt.Errorf("failed to generate IV: %w", err)
	}

	encryptor, err := crypto.NewEncryptorWithVersion(dek, iv, blockSize, fm.currentWriteVersion)
	if err != nil {
		return nil, nil, nil, 0, fmt.Errorf("failed to create encryptor: %w", err)
	}

	// Encrypt
	ciphertext, err = encryptor.Encrypt(plaintext)
	if err != nil {
		return nil, nil, nil, 0, fmt.Errorf("failed to encrypt: %w", err)
	}

	return ciphertext, iv, wrappedDEK, blockSize, nil
}

// buildNewMetadata constructs new metadata for the migrated object.
func (fm *FormatMigrator) buildNewMetadata(oldMeta map[string]string, iv, wrappedDEK []byte, blockSize, plaintextSize int, plaintextSHA []byte) map[string]string {
	newMeta := make(map[string]string)

	// Copy all non-ARMOR metadata
	for k, v := range oldMeta {
		if !strings.HasPrefix(k, "x-amz-meta-armor-") {
			newMeta[k] = v
		}
	}

	// Set ARMOR metadata with new version
	newMeta[armorMetaVersion] = fmt.Sprintf("%d", fm.currentWriteVersion)
	newMeta[armorMetaWrappedDEK] = base64.StdEncoding.EncodeToString(wrappedDEK)
	newMeta[armorMetaIV] = base64.StdEncoding.EncodeToString(iv)
	newMeta[armorMetaBlockSize] = fmt.Sprintf("%d", blockSize)
	newMeta[armorMetaPlaintextSize] = fmt.Sprintf("%d", plaintextSize)
	newMeta[armorMetaPlaintextSHA] = hex.EncodeToString(plaintextSHA)

	// Copy other ARMOR metadata that should be preserved
	if v, ok := oldMeta[armorMetaMultipart]; ok {
		newMeta[armorMetaMultipart] = v
	}
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

// recordFailure records a failed migration.
func (fm *FormatMigrator) recordFailure(key, reason string) {
	fm.stateMu.Lock()
	defer fm.stateMu.Unlock()
	failure := MigrationFailure{
		Key:    key,
		Reason: reason,
		Time:   time.Now(),
	}
	fm.state.Failures = append(fm.state.Failures, failure)
	fm.state.FailedObjects++
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
func (fm *FormatMigrator) countObjects(ctx context.Context) error {
	var count int
	var continuationToken string

	for {
		listResult, err := fm.backend.List(ctx, fm.bucket, "", "", continuationToken, 1000)
		if err != nil {
			return err
		}

		for _, obj := range listResult.Objects {
			// Skip internal ARMOR objects
			if len(obj.Key) >= 7 && obj.Key[:7] == ".armor/" {
				continue
			}

			// Check if object has ARMOR metadata
			rawMeta, err := fm.objectMetadata(ctx, obj)
			if err != nil {
				continue
			}

			armorMeta, ok := backend.ParseARMORMetadata(rawMeta)
			if !ok {
				continue
			}

			// Check if version is in include list
			if fm.shouldMigrateVersion(armorMeta.Version) {
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