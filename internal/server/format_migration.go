// Package server provides format migration functionality for ARMOR.
package server

import (
	"bytes"
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
	"github.com/jedarden/armor/internal/provenance"
)

const (
	// multipartMinPartSize is B2's minimum part size for multi-part objects (5 MiB)
	multipartMinPartSize = int64(5 * 1024 * 1024)
)

// MigrationState tracks the progress of a format migration operation.
type MigrationState struct {
	// ID is a unique identifier for this migration
	ID string `json:"id"`
	// TargetVersions is the comma-separated list of versions to migrate (e.g., "v1" or "v1,v2")
	TargetVersions string `json:"target_versions"`
	// WriteVersion is the format version to write (current write format, e.g., 2 for Version2)
	WriteVersion int `json:"write_version"`
	// StartTime is when the migration began
	StartTime time.Time `json:"start_time"`
	// LastUpdated is when the state was last updated
	LastUpdated time.Time `json:"last_updated"`
	// Status is the current status: "in_progress", "completed", "failed"
	Status string `json:"status"`
	// TotalObjects is the total number of objects to migrate
	TotalObjects int `json:"total_objects"`
	// ProcessedObjects is the number of objects processed so far
	ProcessedObjects int `json:"processed_objects"`
	// SkippedObjects is the number of objects skipped (not in target versions)
	SkippedObjects int `json:"skipped_objects"`
	// FailedObjects is the number of objects that failed migration
	FailedObjects int `json:"failed_objects"`
	// LastKey is the last object key processed (for resumption)
	LastKey string `json:"last_key"`
	// Failures records individual object failures
	Failures []MigrationFailure `json:"failures,omitempty"`
	// ErrorMessage contains any global error that occurred
	ErrorMessage string `json:"error_message,omitempty"`
	// DryRun is true if this was a dry-run migration
	DryRun bool `json:"dry_run"`
}

// MigrationFailure records a failed object migration.
type MigrationFailure struct {
	Key     string `json:"key"`
	Reason  string `json:"reason"`
	Version int    `json:"version"`
	Skipped bool   `json:"skipped"` // true if object was skipped (not retried)
}

// MigrationResult contains the result of a format migration operation.
type MigrationResult struct {
	TotalObjects     int               `json:"total_objects"`
	ProcessedObjects int               `json:"processed_objects"`
	SkippedObjects   int               `json:"skipped_objects"`
	FailedObjects    int               `json:"failed_objects"`
	Duration         time.Duration     `json:"duration"`
	Status           string            `json:"status"`
	ErrorMessage     string            `json:"error_message,omitempty"`
	Failures         []MigrationFailure `json:"failures,omitempty"`
}

// FormatMigrator handles format migration operations.
type FormatMigrator struct {
	backend     backend.Backend
	bucket      string
	keyManager  *keymanager.KeyManager
	manifest    *manifest.Index
	provenance  *provenance.Manager
	// targetVersions is the set of versions to migrate (e.g., map[int]bool{1: true})
	targetVersions map[int]bool
	// writeVersion is the format version to write (default: Version2)
	writeVersion int
	// concurrency is the number of parallel migration workers
	concurrency int
	// dryRun skips actual PUT operations
	dryRun bool

	// state tracks migration progress
	state     *MigrationState
	stateMu   sync.Mutex
	statePath string // .armor/migration-state.json
}

// NewFormatMigrator creates a new format migrator.
func NewFormatMigrator(
	b backend.Backend,
	bucket string,
	keyManager *keymanager.KeyManager,
	manifest *manifest.Index,
	provenance *provenance.Manager,
	targetVersions string,
	writeVersion int,
	concurrency int,
	dryRun bool,
) *FormatMigrator {
	// Parse target versions ("v1" or "v1,v2")
	tv := parseTargetVersions(targetVersions)

	// Default write version to Version2 if not specified
	if writeVersion == 0 {
		writeVersion = crypto.Version2
	}

	// Default concurrency to 4 if not specified
	if concurrency == 0 {
		concurrency = 4
	}

	return &FormatMigrator{
		backend:        b,
		bucket:         bucket,
		keyManager:     keyManager,
		manifest:       manifest,
		provenance:     provenance,
		targetVersions: tv,
		writeVersion:   writeVersion,
		concurrency:    concurrency,
		dryRun:         dryRun,
		statePath:      ".armor/migration-state.json",
	}
}

// parseTargetVersions parses a comma-separated version string into a set.
// Example: "v1" → {1: true}, "v1,v2" → {1: true, 2: true}
func parseTargetVersions(versions string) map[int]bool {
	result := make(map[int]bool)
	if versions == "" {
		// Default to migrating only v1
		result[1] = true
		return result
	}

	parts := strings.Split(versions, ",")
	for _, part := range parts {
		part = strings.TrimSpace(strings.ToLower(part))
		if strings.HasPrefix(part, "v") {
			part = strings.TrimPrefix(part, "v")
		}
		var v int
		if _, err := fmt.Sscanf(part, "%d", &v); err == nil {
			if v == 1 || v == 2 {
				result[v] = true
			}
		}
	}

	// Default to v1 if no valid versions parsed
	if len(result) == 0 {
		result[1] = true
	}

	return result
}

// shouldMigrateVersion returns true if the given version should be migrated.
func (fm *FormatMigrator) shouldMigrateVersion(version int) bool {
	return fm.targetVersions[version]
}

// Migrate performs the format migration, re-encrypting all objects in target versions
// with the current write format (Version2 by default).
func (fm *FormatMigrator) Migrate(ctx context.Context) (*MigrationResult, error) {
	startTime := time.Now()

	// Initialize or load state
	if err := fm.initOrLoadState(ctx); err != nil {
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
		Status:   "in_progress",
		Failures: []MigrationFailure{},
	}

	// Count total objects first
	if err := fm.countObjects(ctx); err != nil {
		return nil, fmt.Errorf("failed to count objects: %w", err)
	}

	// Create a worker pool for concurrent migration
	type migrateJob struct {
		obj backend.ObjectInfo
	}
	jobs := make(chan migrateJob, fm.concurrency*2)
	errors := make(chan error, 1)

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < fm.concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				if err := fm.migrateObject(ctx, job.obj); err != nil {
					// Record failure and continue
					log.Printf("Warning: failed to migrate object %s: %v", job.obj.Key, err)

					fm.stateMu.Lock()
					failure := MigrationFailure{
						Key:     job.obj.Key,
						Reason:  err.Error(),
						Skipped: true, // Never retry in a loop
					}
					fm.state.Failures = append(fm.state.Failures, failure)
					fm.state.FailedObjects++
					fm.state.ProcessedObjects++ // Count as processed (even if failed)
					fm.state.LastKey = job.obj.Key
					fm.state.LastUpdated = time.Now()
					fm.stateMu.Unlock()
				}
			}
		}()
	}

	// Feed objects to workers
	var continuationToken string
	for {
		select {
		case <-ctx.Done():
			// Context cancelled - close job channel and wait for workers
			close(jobs)
			wg.Wait()

			result.Status = "interrupted"
			result.ErrorMessage = ctx.Err().Error()
			result.Failures = fm.state.Failures

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
			close(jobs)
			wg.Wait()

			result.Status = "failed"
			result.ErrorMessage = err.Error()
			result.Failures = fm.state.Failures

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
				fm.stateMu.Lock()
				fm.state.SkippedObjects++
				fm.stateMu.Unlock()
				continue
			}

			// Get metadata to check version
			armorMeta, ok := backend.ParseARMORMetadata(obj.Metadata)
			if !ok {
				// Not an ARMOR-encrypted object
				fm.stateMu.Lock()
				fm.state.SkippedObjects++
				fm.stateMu.Unlock()
				continue
			}

			// Check if this object's version should be migrated
			if !fm.shouldMigrateVersion(armorMeta.Version) {
				fm.stateMu.Lock()
				fm.state.SkippedObjects++
				fm.stateMu.Unlock()
				continue
			}

			// Check if we should skip this object (already processed in a previous run)
			fm.stateMu.Lock()
			if fm.state.LastKey != "" && obj.Key <= fm.state.LastKey {
				fm.state.SkippedObjects++
				fm.stateMu.Unlock()
				continue
			}
			fm.stateMu.Unlock()

			// Send to workers
			jobs <- migrateJob{obj: obj}
		}

		if !listResult.IsTruncated {
			break
		}
		continuationToken = listResult.NextToken
	}

	// Close job channel and wait for workers
	close(jobs)
	wg.Wait()

	// Mark migration as complete
	fm.stateMu.Lock()
	fm.state.Status = "completed"
	fm.state.LastUpdated = time.Now()
	fm.state.ProcessedObjects = fm.state.TotalObjects - fm.state.SkippedObjects
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

// migrateObject migrates a single object: decrypt → re-encrypt → verify → PUT.
func (fm *FormatMigrator) migrateObject(ctx context.Context, obj backend.ObjectInfo) error {
	// Dry run: just count, don't migrate
	if fm.dryRun {
		return nil
	}

	// Get full object metadata (List doesn't return all metadata on B2)
	info, err := fm.backend.Head(ctx, fm.bucket, obj.Key)
	if err != nil {
		return fmt.Errorf("failed to get object metadata: %w", err)
	}

	armorMeta, ok := backend.ParseARMORMetadata(info.Metadata)
	if !ok {
		return fmt.Errorf("object %s is not ARMOR-encrypted", obj.Key)
	}

	// Decrypt the object through the normal read path
	plaintext, err := fm.decryptObject(ctx, obj.Key, info, armorMeta)
	if err != nil {
		return fmt.Errorf("failed to decrypt object: %w", err)
	}

	// Calculate plaintext SHA-256 for verification
	plaintextSHA := sha256.Sum256(plaintext)

	// Re-encrypt with current write format (Version2)
	// PUT to the same key (single PUT if ≤ multipart threshold, otherwise multipart)
	if err := fm.encryptAndPut(ctx, obj.Key, plaintext, plaintextSHA, armorMeta.ContentType); err != nil {
		return fmt.Errorf("failed to re-encrypt object: %w", err)
	}

	// Verify by reading back and comparing SHA-256
	verifyInfo, err := fm.backend.Head(ctx, fm.bucket, obj.Key)
	if err != nil {
		return fmt.Errorf("failed to verify object: %w", err)
	}

	verifyMeta, ok := backend.ParseARMORMetadata(verifyInfo.Metadata)
	if !ok {
		return fmt.Errorf("verified object is not ARMOR-encrypted")
	}

	if verifyMeta.PlaintextSHA != hex.EncodeToString(plaintextSHA[:]) {
		return fmt.Errorf("plaintext SHA-256 mismatch after migration")
	}

	// Update manifest (if enabled)
	if fm.manifest != nil {
		// Record the updated object in the manifest
		// This uses the same logic as a normal PUT
		fm.manifest.Put(fm.bucket, obj.Key, &manifest.Entry{
			WrappedDEK:     verifyMeta.WrappedDEK,
			Version:        verifyMeta.Version,
			BlockSize:      verifyMeta.BlockSize,
			PlaintextSize:  verifyMeta.PlaintextSize,
			PlaintextSHA:   verifyMeta.PlaintextSHA,
			KeyID:          verifyMeta.KeyID,
		})
	}

	// Update provenance (if enabled)
	if fm.provenance != nil {
		fm.provenance.RecordUpload(obj.Key, verifyMeta.PlaintextSize, verifyMeta.Version)
	}

	// Update state
	fm.stateMu.Lock()
	fm.state.ProcessedObjects++
	fm.state.LastKey = obj.Key
	fm.state.LastUpdated = time.Now()
	fm.stateMu.Unlock()

	// Save state periodically
	if fm.state.ProcessedObjects%100 == 0 {
		if err := fm.saveState(ctx); err != nil {
			log.Printf("Warning: failed to save migration state: %v", err)
		}
	}

	return nil
}

// decryptObject decrypts an object through the normal read path.
func (fm *FormatMigrator) decryptObject(
	ctx context.Context,
	key string,
	info *backend.ObjectInfo,
	armorMeta *backend.ARMORMetadata,
) ([]byte, error) {
	// Get MEK for this object's key ID
	keyID := armorMeta.KeyID
	if keyID == "" {
		keyID = "default"
	}
	mek, err := fm.keyManager.GetKey(keyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get MEK: %w", err)
	}

	// Unwrap DEK
	dek, err := crypto.UnwrapDEK(mek, armorMeta.WrappedDEK)
	if err != nil {
		return nil, fmt.Errorf("failed to unwrap DEK: %w", err)
	}

	// Download the encrypted object
	reader, _, err := fm.backend.GetDirect(ctx, fm.bucket, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %w", err)
	}
	defer reader.Close()

	// Read the envelope header
	header, err := crypto.ReadEnvelopeHeader(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read envelope header: %w", err)
	}

	// Create decryptor with the correct version
	decryptor, err := crypto.NewDecryptorWithVersion(
		dek,
		header.IV,
		header.BlockSize(),
		int64(header.PlaintextSize),
		uint8(armorMeta.Version),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create decryptor: %w", err)
	}

	// Read and decrypt the object
	plaintext := make([]byte, header.PlaintextSize)
	hmacTable := make([]byte, header.BlockCount()*crypto.HMACSize)

	// Read encrypted data and HMAC table
	encryptedData := make([]byte, int64(header.PlaintextSize))
	if _, err := io.ReadFull(reader, encryptedData); err != nil {
		return nil, fmt.Errorf("failed to read encrypted data: %w", err)
	}
	if _, err := io.ReadFull(reader, hmacTable); err != nil {
		return nil, fmt.Errorf("failed to read HMAC table: %w", err)
	}

	// Decrypt and verify
	if err := decryptor.DecryptAndVerify(plaintext, encryptedData, hmacTable); err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

// encryptAndPut re-encrypts plaintext with the current write format and PUTs it to the same key.
func (fm *FormatMigrator) encryptAndPut(
	ctx context.Context,
	key string,
	plaintext []byte,
	plaintextSHA [32]byte,
	contentType string,
) error {
	plaintextSize := int64(len(plaintext))

	// Determine if we should use single PUT or multipart
	useMultipart := plaintextSize > multipartMinPartSize

	if !useMultipart {
		// Single PUT (object ≤ 5 MiB)
		return fm.encryptAndPutSingle(ctx, key, plaintext, plaintextSHA, contentType)
	}

	// Multipart upload (object > 5 MiB)
	return fm.encryptAndPutMultipart(ctx, key, plaintext, plaintextSHA, contentType)
}

// encryptAndPutSingle performs a single PUT encryption and upload.
func (fm *FormatMigrator) encryptAndPutSingle(
	ctx context.Context,
	key string,
	plaintext []byte,
	plaintextSHA [32]byte,
	contentType string,
) error {
	// Get MEK for the key's routing (default key)
	mek, err := fm.keyManager.GetKey(key)
	if err != nil {
		return fmt.Errorf("failed to get MEK: %w", err)
	}

	// Generate DEK and IV
	dek, iv, err := crypto.GenerateDEK()
	if err != nil {
		return fmt.Errorf("failed to generate DEK: %w", err)
	}

	// Wrap DEK
	wrappedDEK, err := crypto.WrapDEK(mek, dek)
	if err != nil {
		return fmt.Errorf("failed to wrap DEK: %w", err)
	}

	// Create envelope header with Version2
	header, err := crypto.NewEnvelopeHeaderWithVersion(
		iv,
		int64(len(plaintext)),
		crypto.DefaultBlockSize,
		plaintextSHA,
		uint8(fm.writeVersion),
	)
	if err != nil {
		return fmt.Errorf("failed to create envelope header: %w", err)
	}

	// Encrypt the plaintext
	encryptor, err := crypto.NewEncryptorWithVersion(
		dek,
		iv,
		crypto.DefaultBlockSize,
		uint8(fm.writeVersion),
	)
	if err != nil {
		return fmt.Errorf("failed to create encryptor: %w", err)
	}

	encryptedData, hmacTable, err := encryptor.Encrypt(plaintext)
	if err != nil {
		return fmt.Errorf("failed to encrypt: %w", err)
	}

	// Build the full object body (header + encrypted data + HMAC table)
	headerBuf, err := header.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode header: %w", err)
	}

	fullObject := append(headerBuf, encryptedData...)
	fullObject = append(fullObject, hmacTable...)

	// Build ARMOR metadata
	armorMeta := &backend.ARMORMetadata{
		Version:       fm.writeVersion,
		BlockSize:     crypto.DefaultBlockSize,
		PlaintextSize: int64(len(plaintext)),
		ContentType:   contentType,
		IV:            iv,
		WrappedDEK:    wrappedDEK,
		PlaintextSHA:  hex.EncodeToString(plaintextSHA[:]),
	}
	metadata := armorMeta.ToMetadata()

	// PUT the object
	reader := io.Reader(&byteReader{data: fullObject})
	if err := fm.backend.Put(ctx, fm.bucket, key, reader, int64(len(fullObject)), metadata); err != nil {
		return fmt.Errorf("failed to put object: %w", err)
	}

	return nil
}

// encryptAndPutMultipart performs a multipart upload encryption and upload.
func (fm *FormatMigrator) encryptAndPutMultipart(
	ctx context.Context,
	key string,
	plaintext []byte,
	plaintextSHA [32]byte,
	contentType string,
) error {
	plaintextSize := int64(len(plaintext))
	blockSize := crypto.DefaultBlockSize

	// Generate DEK and IV for this object
	mek, err := fm.keyManager.GetKey(key)
	if err != nil {
		return fmt.Errorf("failed to get MEK: %w", err)
	}

	dek, iv, err := crypto.GenerateDEK()
	if err != nil {
		return fmt.Errorf("failed to generate DEK: %w", err)
	}

	wrappedDEK, err := crypto.WrapDEK(mek, dek)
	if err != nil {
		return fmt.Errorf("failed to wrap DEK: %w", err)
	}

	// Create multipart upload
	uploadID, err := fm.backend.CreateMultipartUpload(ctx, fm.bucket, key, nil)
	if err != nil {
		return fmt.Errorf("failed to create multipart upload: %w", err)
	}

	// Calculate part size: use B2's minimum (5 MiB) but align to block size
	// and ensure we don't create too many parts (max 10,000 parts)
	partSize := multipartMinPartSize
	remainder := plaintextSize % partSize
	numParts := plaintextSize / partSize
	if remainder > 0 {
		numParts++
	}

	// If we'd have too many parts, increase part size
	for numParts > 10000 {
		partSize *= 2
		numParts = plaintextSize / partSize
		if remainder := plaintextSize % partSize; remainder > 0 {
			numParts++
		}
	}

	// Align part size to block size (except possibly the last part)
	partSize = ((partSize + blockSize - 1) / blockSize) * blockSize

	// Upload each part
	var parts []backend.CompletedPart
	var allBlockHMACs [][]byte
	totalBlocks := uint32(0)

	partNumber := int32(1)
	offset := int64(0)

	for offset < plaintextSize {
		// Determine this part's size (last part may be smaller)
		thisPartSize := partSize
		if offset+thisPartSize > plaintextSize {
			thisPartSize = plaintextSize - offset
		}

		// Get plaintext for this part
		partPlaintext := plaintext[offset : offset+thisPartSize]

		// Calculate CTR offset for this part
		// Version2: counter = blockIndex * (blockSize / 16)
		startBlockIndex := uint32(offset / blockSize)

		// Encrypt this part at the correct offset
		offsetEncryptor, err := crypto.NewOffsetEncryptor(
			dek,
			iv,
			blockSize,
			fm.writeVersion,
		)
		if err != nil {
			return fmt.Errorf("failed to create offset encryptor: %w", err)
		}

		encryptedPart, hmacs, err := offsetEncryptor.EncryptAt(partPlaintext, startBlockIndex)
		if err != nil {
			return fmt.Errorf("failed to encrypt part %d: %w", partNumber, err)
		}

		// Upload the encrypted part
		etag, err := fm.backend.UploadPart(
			ctx,
			fm.bucket,
			key,
			uploadID,
			partNumber,
			bytes.NewReader(encryptedPart),
			int64(len(encryptedPart)),
		)
		if err != nil {
			return fmt.Errorf("failed to upload part %d: %w", partNumber, err)
		}

		// Track part for completion
		parts = append(parts, backend.CompletedPart{
			PartNumber: partNumber,
			ETag:       etag,
		})

		// Collect HMACs for the sidecar
		allBlockHMACs = append(allBlockHMACs, hmacs...)

		// Move to next part
		offset += thisPartSize
		partNumber++
		totalBlocks += uint32(thisPartSize + blockSize - 1) / uint32(blockSize)
	}

	// Complete the multipart upload
	completeETag, err := fm.backend.CompleteMultipartUpload(ctx, fm.bucket, key, uploadID, parts)
	if err != nil {
		return fmt.Errorf("failed to complete multipart upload: %w", err)
	}

	// Build and upload HMAC sidecar
	hmacSidecarKey := fmt.Sprintf(".armor/hmac/%x", sha256.Sum256([]byte(key)))
	hmacTableData := make([]byte, 0, len(allBlockHMACs)*crypto.HMACSize)
	for _, hmac := range allBlockHMACs {
		hmacTableData = append(hmacTableData, hmac...)
	}

	hmacMeta := map[string]string{
		"Content-Type": "application/octet-stream",
	}
	hmacReader := bytes.NewReader(hmacTableData)
	if err := fm.backend.Put(ctx, fm.bucket, hmacSidecarKey, hmacReader, int64(len(hmacTableData)), hmacMeta); err != nil {
		return fmt.Errorf("failed to upload HMAC sidecar: %w", err)
	}

	// Now update the object metadata with ARMOR metadata
	// We need to do a CopyObject with MetadataDirective=REPLACE to set the metadata
	armorMeta := &backend.ARMORMetadata{
		Version:       fm.writeVersion,
		BlockSize:     blockSize,
		PlaintextSize: plaintextSize,
		ContentType:   contentType,
		IV:            iv,
		WrappedDEK:    wrappedDEK,
		PlaintextSHA:  hex.EncodeToString(plaintextSHA[:]),
		ETag:          completeETag,
		// Multipart marker
		Multipart:   true,
		PartSize:    partSize,
	}
	metadata := armorMeta.ToMetadata()

	// Copy the object to update its metadata
	if err := fm.backend.Copy(ctx, fm.bucket, key, fm.bucket, key, metadata, true); err != nil {
		return fmt.Errorf("failed to update object metadata: %w", err)
	}

	return nil
}

// initOrLoadState initializes a new migration state or loads an existing one.
func (fm *FormatMigrator) initOrLoadState(ctx context.Context) error {
	// Compute migration ID
	versionList := make([]string, 0, len(fm.targetVersions))
	for v := range fm.targetVersions {
		versionList = append(versionList, fmt.Sprintf("v%d", v))
	}
	targetVersionsStr := strings.Join(versionList, ",")
	migrationID := fmt.Sprintf("%s-to-v%d-%d",
		targetVersionsStr,
		fm.writeVersion,
		time.Now().Unix())

	fm.state = &MigrationState{
		ID:             migrationID,
		TargetVersions: targetVersionsStr,
		WriteVersion:   fm.writeVersion,
		StartTime:      time.Now(),
		LastUpdated:    time.Now(),
		Status:         "initialized",
		DryRun:         fm.dryRun,
	}

	// Try to load existing state
	existingState, err := fm.loadState(ctx)
	if err == nil && existingState != nil {
		// Check if this is a continuation of the same migration
		if existingState.TargetVersions == fm.state.TargetVersions &&
			existingState.WriteVersion == fm.state.WriteVersion &&
			existingState.Status == "in_progress" &&
			existingState.DryRun == fm.dryRun {
			fm.state = existingState
			log.Printf("Resuming migration from key: %s", existingState.LastKey)
		}
	}

	return nil
}

// loadState loads the migration state from backend.
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

// saveState saves the migration state to backend.
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

			// Check if this is an ARMOR-encrypted object with a target version
			armorMeta, ok := backend.ParseARMORMetadata(obj.Metadata)
			if !ok {
				continue
			}

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

// byteReader wraps a byte slice to implement io.Reader.
type byteReader struct {
	data []byte
	pos  int
}

func (r *byteReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
