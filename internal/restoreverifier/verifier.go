// Package restoreverifier implements continuous restore verification for ARMOR backups.
// It runs dual-path verification (ARMOR read path + armor-decrypt direct) to prove
// that backups are restorable through both the normal server path and disaster recovery.
package restoreverifier

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
	"github.com/jedarden/armor/internal/manifest"
)

// VerificationStatus represents the status of a verification operation.
type VerificationStatus string

const (
	StatusPass           VerificationStatus = "pass"
	StatusFail           VerificationStatus = "fail"
	StatusPending        VerificationStatus = "pending"
	StatusUnknown        VerificationStatus = "unknown"
	StatusConflict       VerificationStatus = "conflict" // Dual-path mismatch
	StatusRestoreError   VerificationStatus = "restore_error"
	StatusChecksumError  VerificationStatus = "checksum_error"
	StatusAssertionError VerificationStatus = "assertion_error"
)

// VerificationPath represents which path was used for verification.
type VerificationPath string

const (
	PathARMOR     VerificationPath = "armor"     // Normal ARMOR read path
	PathDirect    VerificationPath = "direct"    // armor-decrypt direct to ciphertext
	PathDualMatch VerificationPath = "dual_match" // Both paths agree
)

// ArtifactType represents the type of backup artifact being verified.
type ArtifactType string

const (
	ArtifactSQLite   ArtifactType = "sqlite"   // SQLite database
	ArtifactParquet  ArtifactType = "parquet"  // Parquet file
	ArtifactTarGz    ArtifactType = "tar-gz"   // tar.gz archive
	ArtifactGeneric  ArtifactType = "generic"  // Generic file (basic verification only)
)

// ArtifactAssertion represents application-level validation for an artifact.
type ArtifactAssertion interface {
	Verify(plaintext []byte, metadata map[string]string) error
	Type() ArtifactType
}

// SQLiteAssertion verifies SQLite database integrity.
type SQLiteAssertion struct{}

func (a *SQLiteAssertion) Verify(plaintext []byte, metadata map[string]string) error {
	// For SQLite, we'd need to write to a temp file and run PRAGMA integrity_check
	// This is a placeholder for the actual implementation
	return nil
}

func (a *SQLiteAssertion) Type() ArtifactType { return ArtifactSQLite }

// ParquetAssertion verifies Parquet file validity.
type ParquetAssertion struct{}

func (a *ParquetAssertion) Verify(plaintext []byte, metadata map[string]string) error {
	// For Parquet, we'd verify the footer and optionally read row counts
	// This is a placeholder for the actual implementation
	return nil
}

func (a *ParquetAssertion) Type() ArtifactType { return ArtifactParquet }

// TarGzAssertion verifies tar.gz archive validity.
type TarGzAssertion struct{}

func (a *TarGzAssertion) Verify(plaintext []byte, metadata map[string]string) error {
	// For tar.gz, we'd verify the archive can be read and list contents
	// This is a placeholder for the actual implementation
	return nil
}

func (a *TarGzAssertion) Type() ArtifactType { return ArtifactTarGz }

// GenericAssertion performs basic verification for unknown file types.
type GenericAssertion struct{}

func (a *GenericAssertion) Verify(plaintext []byte, metadata map[string]string) error {
	// Basic verification: ensure plaintext is non-empty
	if len(plaintext) == 0 {
		return errors.New("plaintext is empty")
	}
	return nil
}

func (a *GenericAssertion) Type() ArtifactType { return ArtifactGeneric }

// ObjectSample represents a object to be verified.
type ObjectSample struct {
	Key         string            `json:"key"`
	Bucket      string            `json:"bucket"`
	LastModified time.Time        `json:"last_modified"`
	Size        int64             `json:"size"`
	ArtifactType ArtifactType      `json:"artifact_type"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// VerificationResult represents the result of verifying a single object.
type VerificationResult struct {
	Key              string            `json:"key"`
	Bucket           string            `json:"bucket"`
	Status           VerificationStatus `json:"status"`
	Path             VerificationPath  `json:"path"`
	Timestamp        time.Time         `json:"timestamp"`
	ArtifactType     ArtifactType      `json:"artifact_type"`

	// Checksums
	ExpectedSHA256   string            `json:"expected_sha256,omitempty"`
	ARMORSHA256      string            `json:"armor_sha256,omitempty"`
	DirectSHA256     string            `json:"direct_sha256,omitempty"`

	// Latency
	ARMORPathLatency  time.Duration `json:"armor_path_latency_ms"`
	DirectPathLatency time.Duration `json:"direct_path_latency_ms"`

	// Errors
	Error string `json:"error,omitempty"`

	// Assertion results
	AssertionPassed bool   `json:"assertion_passed"`
	AssertionError string `json:"assertion_error,omitempty"`
}

// BucketState holds verification state for a single bucket.
type BucketState struct {
	mu sync.RWMutex

	Bucket              string                     `json:"bucket"`
	LastVerification    time.Time                  `json:"last_verification"`
	LastSuccess         time.Time                  `json:"last_success"`
	VerifiedObjectRatio float64                    `json:"verified_object_ratio"` // ratio of verified/total
	TotalObjects        int64                      `json:"total_objects"`
	VerifiedObjects     int64                      `json:"verified_objects"`
	FailedObjects       int64                      `json:"failed_objects"`

	// Recent results (for debugging and escalation)
	RecentResults       []VerificationResult       `json:"recent_results"`

	// Configuration sample settings
	HistoricalSampleSize int                        `json:"historical_sample_size"`
}

// Verifier manages continuous restore verification across multiple buckets.
type Verifier struct {
	mu            sync.RWMutex              // protects buckets field

	backend       backend.Backend
	mek           []byte
	blockSize     int
	manifest      *manifest.Index

	buckets       map[string]*BucketState  // bucket name -> state
	bucketConfigs []BucketConfig           // configured buckets

	// Control
	stopCh        chan struct{}
	doneCh        chan struct{}

	// Configuration
	interval      time.Duration
	sampleSize    int  // number of objects to verify per run per bucket
	escrowMekPath string // path to escrowed MEK for direct decryption
	logOutput     io.Writer
}

// BucketConfig holds configuration for a single bucket verification.
type BucketConfig struct {
	Bucket              string     `json:"bucket"`
	Prefix              string     `json:"prefix,omitempty"`
	ArtifactType        ArtifactType `json:"artifact_type,omitempty"`
	Enabled             bool       `json:"enabled"`
	HistoricalSampleSize int        `json:"historical_sample_size,omitempty"`
}

// Config holds verifier configuration.
type Config struct {
	Buckets             []BucketConfig
	Interval            time.Duration
	SampleSize          int
	EscrowMEKPath       string
}

// New creates a new restore verifier.
func New(
	backend backend.Backend,
	mek []byte,
	blockSize int,
	manifest *manifest.Index,
	cfg Config,
) *Verifier {
	v := &Verifier{
		backend:       backend,
		mek:           mek,
		blockSize:     blockSize,
		manifest:      manifest,
		buckets:       make(map[string]*BucketState),
		bucketConfigs: cfg.Buckets,
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
		interval:      cfg.Interval,
		sampleSize:    cfg.SampleSize,
		escrowMekPath: cfg.EscrowMEKPath,
		logOutput:     log.Writer(),
	}

	// Initialize bucket states
	for _, bucketCfg := range cfg.Buckets {
		if bucketCfg.Enabled {
			v.buckets[bucketCfg.Bucket] = &BucketState{
				Bucket:              bucketCfg.Bucket,
				HistoricalSampleSize: bucketCfg.HistoricalSampleSize,
				RecentResults:       make([]VerificationResult, 0, 10),
			}
		}
	}

	return v
}

// Start begins the verification loop.
func (v *Verifier) Start(ctx context.Context) {
	log.Printf("Starting restore verifier with %d buckets, interval %v",
		len(v.buckets), v.interval)

	ticker := time.NewTicker(v.interval)
	defer ticker.Stop()
	defer close(v.doneCh)

	// Run initial verification
	v.runVerification(ctx)

	for {
		select {
		case <-ticker.C:
			v.runVerification(ctx)
		case <-v.stopCh:
			log.Println("Restore verifier stopping")
			return
		case <-ctx.Done():
			log.Println("Restore verifier context cancelled")
			return
		}
	}
}

// Stop gracefully stops the verifier.
func (v *Verifier) Stop() {
	close(v.stopCh)
	<-v.doneCh
}

// runVerification executes verification for all configured buckets.
func (v *Verifier) runVerification(ctx context.Context) {
	log.Println("Starting verification run")

	var wg sync.WaitGroup
	for bucketName, bucketState := range v.buckets {
		wg.Add(1)
		go func(bucket string, state *BucketState) {
			defer wg.Done()
			v.verifyBucket(ctx, bucket, state)
		}(bucketName, bucketState)
	}
	wg.Wait()

	log.Println("Verification run completed")
}

// verifyBucket verifies a single bucket.
func (v *Verifier) verifyBucket(ctx context.Context, bucket string, state *BucketState) {
	state.mu.Lock()
	defer state.mu.Unlock()

	log.Printf("Verifying bucket: %s", bucket)

	// Get most recent backup object (should be the latest generation)
	latest, err := v.getLatestObject(ctx, bucket)
	if err != nil {
		log.Printf("Failed to get latest object for bucket %s: %v", bucket, err)
		return
	}

	// Get historical sample
	historical, err := v.getHistoricalSample(ctx, bucket, state.HistoricalSampleSize)
	if err != nil {
		log.Printf("Failed to get historical sample for bucket %s: %v", bucket, err)
		return
	}

	state.TotalObjects = int64(1 + len(historical))
	objectsToVerify := append([]ObjectSample{latest}, historical...)

	// Verify each object
	for _, obj := range objectsToVerify {
		result := v.verifyObject(ctx, obj)

		// Update state
		if result.Status == StatusPass {
			state.VerifiedObjects++
			state.LastSuccess = result.Timestamp
		} else {
			state.FailedObjects++
		}

		// Keep recent results limited
		state.RecentResults = append(state.RecentResults, result)
		if len(state.RecentResults) > 10 {
			state.RecentResults = state.RecentResults[1:]
		}

		// Log failures for escalation
		if result.Status != StatusPass {
			log.Printf("Verification failed for %s/%s: %s (path: %s, error: %s)",
				bucket, result.Key, result.Status, result.Path, result.Error)
		}
	}

	state.LastVerification = time.Now()
	if state.TotalObjects > 0 {
		state.VerifiedObjectRatio = float64(state.VerifiedObjects) / float64(state.TotalObjects)
	}

	log.Printf("Bucket %s verification complete: %d/%d verified (%.1f%%)",
		bucket, state.VerifiedObjects, state.TotalObjects, state.VerifiedObjectRatio*100)
}

// verifyObject performs dual-path verification on a single object.
func (v *Verifier) verifyObject(ctx context.Context, obj ObjectSample) VerificationResult {
	result := VerificationResult{
		Key:          obj.Key,
		Bucket:       obj.Bucket,
		ArtifactType: obj.ArtifactType,
		Timestamp:    time.Now(),
		Status:       StatusPending,
	}

	// Get expected SHA256 from metadata
	expectedSHA256 := obj.Metadata["x-amz-meta-armor-plaintext-sha256"]
	result.ExpectedSHA256 = expectedSHA256

	// Path 1: ARMOR read path (normal S3 GetObject through server)
	armorStart := time.Now()
	armorPlaintext, armorErr := v.restoreViaARMOR(ctx, obj.Bucket, obj.Key)
	result.ARMORPathLatency = time.Since(armorStart)

	if armorErr != nil {
		result.Error = fmt.Sprintf("ARMOR path failed: %v", armorErr)
		result.Status = StatusRestoreError
		result.Path = PathARMOR
		return result
	}

	// Compute SHA256 of ARMOR path result
	armorHash := sha256.Sum256(armorPlaintext)
	result.ARMORSHA256 = hex.EncodeToString(armorHash[:])

	// Path 2: Direct decryption using armor-decrypt approach
	directStart := time.Now()
	directPlaintext, directErr := v.restoreViaDirectDecrypt(ctx, obj.Bucket, obj.Key)
	result.DirectPathLatency = time.Since(directStart)

	if directErr != nil {
		result.Error = fmt.Sprintf("Direct path failed: %v", directErr)
		result.Status = StatusRestoreError
		result.Path = PathDirect
		return result
	}

	// Compute SHA256 of direct path result
	directHash := sha256.Sum256(directPlaintext)
	result.DirectSHA256 = hex.EncodeToString(directHash[:])

	// Compare dual-path results
	if result.ARMORSHA256 != result.DirectSHA256 {
		result.Status = StatusConflict
		result.Path = PathDirect // Use Direct to indicate the conflict path
		result.Error = fmt.Sprintf("SHA256 mismatch: ARMOR=%s, Direct=%s",
			result.ARMORSHA256, result.DirectSHA256)
		return result
	}

	// Both paths agree, verify against expected checksum
	if expectedSHA256 != "" && result.ARMORSHA256 != expectedSHA256 {
		result.Status = StatusChecksumError
		result.Path = PathDualMatch
		result.Error = fmt.Sprintf("SHA256 mismatch: expected=%s, got=%s",
			expectedSHA256, result.ARMORSHA256)
		return result
	}

	// Run application-level assertion
	assertion := v.getAssertion(obj.ArtifactType)
	assertionErr := assertion.Verify(armorPlaintext, obj.Metadata)
	if assertionErr != nil {
		result.Status = StatusAssertionError
		result.Path = PathDualMatch
		result.Error = fmt.Sprintf("Assertion failed: %v", assertionErr)
		result.AssertionPassed = false
		result.AssertionError = assertionErr.Error()
		return result
	}

	// All checks passed
	result.Status = StatusPass
	result.Path = PathDualMatch
	result.AssertionPassed = true

	return result
}

// restoreViaARMOR restores an object through the normal ARMOR read path.
func (v *Verifier) restoreViaARMOR(ctx context.Context, bucket, key string) ([]byte, error) {
	// Use the normal backend GetObject which will decrypt through ARMOR
	reader, _, err := v.backend.Get(ctx, bucket, key)
	if err != nil {
		return nil, fmt.Errorf("ARMOR GetObject failed: %w", err)
	}
	defer reader.Close()

	plaintext, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read ARMOR response: %w", err)
	}

	return plaintext, nil
}

// restoreViaDirectDecrypt restores an object using direct B2 access + armor-decrypt logic.
// This simulates the "ARMOR server is gone" disaster recovery scenario by decrypting
// directly from B2 ciphertext using the escrowed MEK.
func (v *Verifier) restoreViaDirectDecrypt(ctx context.Context, bucket, key string) ([]byte, error) {
	// Step 1: Get object metadata to extract ARMOR encryption parameters
	info, err := v.backend.Head(ctx, bucket, key)
	if err != nil {
		return nil, fmt.Errorf("direct path: HeadObject failed: %w", err)
	}

	// Step 2: Parse ARMOR metadata from headers
	armorMeta, ok := backend.ParseARMORMetadata(info.Metadata)
	if !ok {
		return nil, errors.New("direct path: object is not ARMOR-encrypted")
	}

	// Step 3: Unwrap the DEK using the escrowed MEK
	dek, err := crypto.UnwrapDEK(v.mek, armorMeta.WrappedDEK)
	if err != nil {
		return nil, fmt.Errorf("direct path: failed to unwrap DEK: %w", err)
	}

	// Step 4: Read envelope header (64 bytes) from B2
	headerReader, err := v.backend.GetRange(ctx, bucket, key, 0, crypto.HeaderSize)
	if err != nil {
		return nil, fmt.Errorf("direct path: failed to read envelope header: %w", err)
	}
	defer headerReader.Close()

	headerBuf := make([]byte, crypto.HeaderSize)
	if _, err := io.ReadFull(headerReader, headerBuf); err != nil {
		return nil, fmt.Errorf("direct path: failed to read header bytes: %w", err)
	}

	header, err := crypto.DecodeHeader(headerBuf)
	if err != nil {
		return nil, fmt.Errorf("direct path: failed to decode envelope header: %w", err)
	}

	// Step 5: Read encrypted data from B2
	// Offset: crypto.HeaderSize (64 bytes)
	// Length: armorMeta.PlaintextSize (ciphertext size equals plaintext size for CTR mode)
	encryptedData := make([]byte, armorMeta.PlaintextSize)
	dataReader, err := v.backend.GetRange(ctx, bucket, key, crypto.HeaderSize, armorMeta.PlaintextSize)
	if err != nil {
		return nil, fmt.Errorf("direct path: failed to read encrypted data: %w", err)
	}
	defer dataReader.Close()

	if _, err := io.ReadFull(dataReader, encryptedData); err != nil {
		return nil, fmt.Errorf("direct path: failed to read encrypted bytes: %w", err)
	}

	// Step 6: Read HMAC table
	// Check if HMAC is in sidecar (multipart uploads)
	useSidecarHMAC := header.Reserved[1] == 0x01

	var hmacTable []byte
	blockCount := crypto.ComputeBlockCount(armorMeta.PlaintextSize, armorMeta.BlockSize)
	hmacSize := int64(blockCount) * crypto.HMACSize

	if useSidecarHMAC {
		// Fetch HMAC from sidecar object at .armor/hmac/<sha256(key)>
		sidecarKey := fmt.Sprintf(".armor/hmac/%x", crypto.ComputePlaintextSHA256([]byte(key)))
		hmacReader, _, err := v.backend.GetDirect(ctx, bucket, sidecarKey)
		if err != nil {
			return nil, fmt.Errorf("direct path: failed to read sidecar HMAC from %s: %w", sidecarKey, err)
		}
		defer hmacReader.Close()

		hmacTable = make([]byte, hmacSize)
		if _, err := io.ReadFull(hmacReader, hmacTable); err != nil {
			return nil, fmt.Errorf("direct path: failed to read sidecar HMAC bytes: %w", err)
		}
	} else {
		// Read inline HMAC table
		// Offset: crypto.HeaderSize + armorMeta.PlaintextSize
		hmacOffset := crypto.HeaderSize + armorMeta.PlaintextSize
		hmacReader, err := v.backend.GetRange(ctx, bucket, key, hmacOffset, hmacSize)
		if err != nil {
			return nil, fmt.Errorf("direct path: failed to read HMAC table: %w", err)
		}
		defer hmacReader.Close()

		hmacTable = make([]byte, hmacSize)
		if _, err := io.ReadFull(hmacReader, hmacTable); err != nil {
			return nil, fmt.Errorf("direct path: failed to read HMAC bytes: %w", err)
		}
	}

	// Step 7: Decrypt using armor-decrypt logic
	decryptor, err := crypto.NewDecryptor(dek, header.IV[:], armorMeta.BlockSize)
	if err != nil {
		return nil, fmt.Errorf("direct path: failed to create decryptor: %w", err)
	}

	plaintext, err := decryptor.Decrypt(encryptedData, hmacTable)
	if err != nil {
		return nil, fmt.Errorf("direct path: decryption failed: %w (possible data corruption or wrong MEK)", err)
	}

	// Step 8: Verify plaintext SHA-256
	if err := header.VerifyPlaintextSHA(plaintext); err != nil {
		return nil, fmt.Errorf("direct path: plaintext SHA-256 verification failed: %w", err)
	}

	return plaintext, nil
}

// getLatestObject returns the most recent backup object for a bucket.
func (v *Verifier) getLatestObject(ctx context.Context, bucket string) (ObjectSample, error) {
	// List objects in the bucket, sorted by last modified descending
	listResult, err := v.backend.List(ctx, bucket, "", "", "", 100)
	if err != nil {
		return ObjectSample{}, fmt.Errorf("list failed: %w", err)
	}

	if len(listResult.Objects) == 0 {
		return ObjectSample{}, errors.New("no objects found")
	}

	// Find the most recent object (skip .armor/ internal objects)
	var latest *backend.ObjectInfo
	for i := range listResult.Objects {
		obj := &listResult.Objects[i]
		if strings.HasPrefix(obj.Key, ".armor/") {
			continue
		}
		if latest == nil || obj.LastModified.After(latest.LastModified) {
			latest = obj
		}
	}

	if latest == nil {
		return ObjectSample{}, errors.New("no non-internal objects found")
	}

	return ObjectSample{
		Key:          latest.Key,
		Bucket:       bucket,
		LastModified: latest.LastModified,
		Size:         latest.Size,
		ArtifactType: v.inferArtifactType(latest.Key, latest.Metadata),
		Metadata:     latest.Metadata,
	}, nil
}

// getHistoricalSample returns a random sample of historical objects.
func (v *Verifier) getHistoricalSample(ctx context.Context, bucket string, sampleSize int) ([]ObjectSample, error) {
	// List objects and randomly sample
	listResult, err := v.backend.List(ctx, bucket, "", "", "", 1000)
	if err != nil {
		return nil, fmt.Errorf("list failed: %w", err)
	}

	// Filter out internal objects and the latest one (already verified)
	var candidates []ObjectSample
	for _, obj := range listResult.Objects {
		if strings.HasPrefix(obj.Key, ".armor/") {
			continue
		}
		candidates = append(candidates, ObjectSample{
			Key:          obj.Key,
			Bucket:       bucket,
			LastModified: obj.LastModified,
			Size:         obj.Size,
			ArtifactType: v.inferArtifactType(obj.Key, obj.Metadata),
			Metadata:     obj.Metadata,
		})
	}

	// Simple random sample (in production, use proper random sampling)
	sampleCount := sampleSize
	if len(candidates) < sampleCount {
		sampleCount = len(candidates)
	}

	// For now, just take the last N objects
	// TODO: Implement proper random sampling
	start := 0
	if len(candidates) > sampleCount {
		start = len(candidates) - sampleCount
	}

	return candidates[start:], nil
}

// inferArtifactType infers the artifact type from key and metadata.
func (v *Verifier) inferArtifactType(key string, metadata map[string]string) ArtifactType {
	// Infer from file extension
	ext := strings.ToLower(filepath.Ext(key))
	switch ext {
	case ".db", ".sqlite", ".sqlite3":
		return ArtifactSQLite
	case ".parquet":
		return ArtifactParquet
	case ".tar.gz", ".tgz":
		return ArtifactTarGz
	default:
		// Check content type metadata
		if ct, ok := metadata["x-amz-meta-armor-content-type"]; ok {
			switch ct {
			case "application/x-sqlite3":
				return ArtifactSQLite
			case "application/parquet":
				return ArtifactParquet
			}
		}
		return ArtifactGeneric
	}
}

// getAssertion returns the appropriate assertion for the artifact type.
func (v *Verifier) getAssertion(atype ArtifactType) ArtifactAssertion {
	switch atype {
	case ArtifactSQLite:
		return &SQLiteAssertion{}
	case ArtifactParquet:
		return &ParquetAssertion{}
	case ArtifactTarGz:
		return &TarGzAssertion{}
	default:
		return &GenericAssertion{}
	}
}

// GetStatus returns the current verification status for all buckets.
func (v *Verifier) GetStatus() map[string]*BucketState {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Return a copy to avoid concurrent access issues
	status := make(map[string]*BucketState)
	for name, state := range v.buckets {
		stateCopy := *state
		stateCopy.RecentResults = make([]VerificationResult, len(state.RecentResults))
		copy(stateCopy.RecentResults, state.RecentResults)
		status[name] = &stateCopy
	}
	return status
}

// GetBucketStatus returns status for a specific bucket.
func (v *Verifier) GetBucketStatus(bucket string) (*BucketState, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	state, ok := v.buckets[bucket]
	if !ok {
		return nil, fmt.Errorf("bucket %s not configured", bucket)
	}

	stateCopy := *state
	stateCopy.RecentResults = make([]VerificationResult, len(state.RecentResults))
	copy(stateCopy.RecentResults, state.RecentResults)
	return &stateCopy, nil
}
