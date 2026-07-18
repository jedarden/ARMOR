// Package restoreverifier implements continuous restore verification for ARMOR backups.
// It runs dual-path verification (ARMOR read path + armor-decrypt direct) to prove
// that backups are restorable through both the normal server path and disaster recovery.
package restoreverifier

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
	"github.com/jedarden/armor/internal/manifest"
	"github.com/parquet-go/parquet-go"

	// modernc.org/sqlite is a pure-Go SQLite driver used to run PRAGMA
	// integrity_check against restored database artifacts without cgo.
	_ "modernc.org/sqlite"
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
	PathARMOR     VerificationPath = "armor"      // Normal ARMOR read path
	PathDirect    VerificationPath = "direct"     // armor-decrypt direct to ciphertext
	PathDualMatch VerificationPath = "dual_match" // Both paths agree
)

// ArtifactType represents the type of backup artifact being verified.
type ArtifactType string

const (
	ArtifactSQLite  ArtifactType = "sqlite"  // SQLite database
	ArtifactParquet ArtifactType = "parquet" // Parquet file
	ArtifactTarGz   ArtifactType = "tar-gz"  // tar.gz archive
	ArtifactGeneric ArtifactType = "generic" // Generic file (basic verification only)
)

// ArtifactAssertion represents application-level validation for an artifact.
type ArtifactAssertion interface {
	Verify(plaintext []byte, metadata map[string]string) error
	Type() ArtifactType
}

// sqliteMagic is the 16-byte header that begins every well-formed SQLite
// database file (see https://www.sqlite.org/fileformat2.html section 1.3).
const sqliteMagic = "SQLite format 3\x00"

// SQLiteAssertion verifies SQLite database integrity.
type SQLiteAssertion struct{}

// Verify writes the restored plaintext to a temp file and runs
// PRAGMA integrity_check through a pure-Go SQLite driver. A healthy database
// reports "ok"; anything else (malformed page, bad b-tree, truncated file) is
// returned as a verification failure rather than swallowed. If metadata names a
// table via "x-amz-meta-armor-sqlite-table", an optional row-count probe asserts
// the table is present and non-empty.
func (a *SQLiteAssertion) Verify(plaintext []byte, metadata map[string]string) error {
	if len(plaintext) == 0 {
		return errors.New("sqlite assertion: plaintext is empty")
	}
	// Cheap structural pre-check before touching the SQLite engine.
	if len(plaintext) < len(sqliteMagic) || string(plaintext[:len(sqliteMagic)]) != sqliteMagic {
		return errors.New("sqlite assertion: missing \"SQLite format 3\" magic header")
	}

	tmpDir, err := os.MkdirTemp("", "armor-sqlite-verify-")
	if err != nil {
		return fmt.Errorf("sqlite assertion: create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "verify.db")
	if err := os.WriteFile(dbPath, plaintext, 0o600); err != nil {
		return fmt.Errorf("sqlite assertion: write temp db: %w", err)
	}

	// Open read-only so a verification never mutates the artifact under test.
	// immutable=1 tells SQLite the file will not change out from under it, so it
	// skips journal/recovery handling that would otherwise require write access.
	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro&immutable=1")
	if err != nil {
		return fmt.Errorf("sqlite assertion: open: %w", err)
	}
	defer db.Close()

	// PRAGMA integrity_check returns one row per problem; a clean DB returns the
	// single row "ok".
	rows, err := db.Query("PRAGMA integrity_check;")
	if err != nil {
		return fmt.Errorf("sqlite assertion: integrity_check failed: %w", err)
	}
	var problems []string
	for rows.Next() {
		var msg string
		if err := rows.Scan(&msg); err != nil {
			rows.Close()
			return fmt.Errorf("sqlite assertion: integrity_check scan: %w", err)
		}
		if msg != "ok" {
			problems = append(problems, msg)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("sqlite assertion: integrity_check rows: %w", err)
	}
	if len(problems) > 0 {
		// Cap the reported detail so a badly damaged DB does not flood the result.
		if len(problems) > 8 {
			problems = append(problems[:8], fmt.Sprintf("... and %d more", len(problems)-8))
		}
		return fmt.Errorf("sqlite assertion: integrity_check reported corruption: %s", strings.Join(problems, "; "))
	}

	// Optional row-count probe: assert a provider-declared table exists and is
	// non-empty. This is the ADR-004 "recency/row-count" probe.
	if table := metadata["x-amz-meta-armor-sqlite-table"]; table != "" {
		if err := sqliteRowCountProbe(db, table); err != nil {
			return err
		}
	}

	return nil
}

// sqliteRowCountProbe asserts that the named table exists and has at least one
// row. The table name comes from object metadata; we reject embedded double
// quotes to keep the interpolated SQL safe.
func sqliteRowCountProbe(db *sql.DB, table string) error {
	if strings.ContainsAny(table, "\"\x00") {
		return fmt.Errorf("sqlite assertion: refusing unsafe table name %q", table)
	}
	quoted := "\"" + strings.ReplaceAll(table, "\"", "\"\"") + "\""

	var name string
	if err := db.QueryRow(
		"SELECT name FROM sqlite_master WHERE type='table' AND name=" + quoted + " LIMIT 1",
	).Scan(&name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("sqlite assertion: declared table %q not present in database", table)
		}
		return fmt.Errorf("sqlite assertion: table lookup for %q failed: %w", table, err)
	}

	var n int64
	if err := db.QueryRow("SELECT count(*) FROM " + quoted).Scan(&n); err != nil {
		return fmt.Errorf("sqlite assertion: row count for %q failed: %w", table, err)
	}
	if n == 0 {
		return fmt.Errorf("sqlite assertion: table %q has 0 rows (expected non-empty backup)", table)
	}
	return nil
}

func (a *SQLiteAssertion) Type() ArtifactType { return ArtifactSQLite }

// parquetMagic is the 4-byte magic that begins and ends every Parquet file.
var parquetMagic = []byte("PAR1")

// ParquetAssertion verifies Parquet file validity.
type ParquetAssertion struct{}

// Verify validates the leading/trailing "PAR1" magic and parses the file footer
// (FileMetaData) to read the declared row count. A corrupt footer fails to parse
// and is reported as a verification failure. If metadata declares an expected row
// count via "x-amz-meta-armor-parquet-rows", the footer's count must match it.
func (a *ParquetAssertion) Verify(plaintext []byte, metadata map[string]string) error {
	if len(plaintext) < 12 { // 4 (head magic) + 4 (footer len) + 4 (tail magic) minimum
		return fmt.Errorf("parquet assertion: file too small (%d bytes)", len(plaintext))
	}
	if !bytes.Equal(plaintext[:4], parquetMagic) {
		return errors.New("parquet assertion: missing leading PAR1 magic")
	}
	if !bytes.Equal(plaintext[len(plaintext)-4:], parquetMagic) {
		return errors.New("parquet assertion: missing trailing PAR1 magic")
	}

	// OpenFile parses the footer metadata (row groups, schema, num_rows) without
	// decoding column data — exactly the "footer parse + row-count sanity" probe.
	file, err := parquet.OpenFile(bytes.NewReader(plaintext), int64(len(plaintext)))
	if err != nil {
		return fmt.Errorf("parquet assertion: footer parse failed: %w", err)
	}

	numRows := file.NumRows()
	rowGroups := len(file.RowGroups())

	// Row-count sanity: when the writer declared an expected count in metadata,
	// the restored footer must agree exactly.
	if wantStr := metadata["x-amz-meta-armor-parquet-rows"]; wantStr != "" {
		var want int64
		if _, perr := fmt.Sscanf(wantStr, "%d", &want); perr == nil {
			if numRows != want {
				return fmt.Errorf("parquet assertion: row count mismatch (metadata=%d, footer=%d)", want, numRows)
			}
		}
	}

	// A backup data artifact with zero row groups almost certainly indicates a
	// truncated or empty restore; flag it rather than passing silently.
	if rowGroups == 0 {
		return fmt.Errorf("parquet assertion: file declares 0 row groups (empty or truncated artifact)")
	}

	return nil
}

func (a *ParquetAssertion) Type() ArtifactType { return ArtifactParquet }

// tarGzSampleEvery is the sampling period for full-entry extraction during the
// tar.gz assertion: every Nth entry is fully decompressed and its byte count
// checked against the tar header's declared size.
const tarGzSampleEvery = 8

// tarGzMaxEntries bounds the number of entries processed; it prevents a
// maliciously large archive (or a decompression bomb) from running unbounded.
const tarGzMaxEntries = 100000

// TarGzAssertion verifies tar.gz archive validity.
type TarGzAssertion struct{}

// Verify walks the gzip member and lists every tar entry end-to-end (so a
// truncated or corrupt stream fails to parse). Every Nth entry is fully
// extracted to io.Discard and its byte count compared to the header-declared
// size, catching mid-archive corruption that a header-only listing would miss.
func (a *TarGzAssertion) Verify(plaintext []byte, metadata map[string]string) error {
	gz, err := gzip.NewReader(bytes.NewReader(plaintext))
	if err != nil {
		return fmt.Errorf("tar.gz assertion: invalid gzip header: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	var entries, sampled int
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar.gz assertion: failed reading entry %d: %w", entries, err)
		}
		entries++

		// Fully extract a sampled entry and verify its declared size. Non-sampled
		// entries are only listed (their content is skipped by the next Next()).
		if entries%tarGzSampleEvery == 1 {
			n, copyErr := io.Copy(io.Discard, tr)
			if copyErr != nil {
				return fmt.Errorf("tar.gz assertion: failed extracting entry %q: %w", hdr.Name, copyErr)
			}
			if n != hdr.Size {
				return fmt.Errorf("tar.gz assertion: size mismatch for %q (header=%d, actual=%d)", hdr.Name, hdr.Size, n)
			}
			sampled++
		}

		if entries > tarGzMaxEntries {
			return fmt.Errorf("tar.gz assertion: exceeded %d entry limit", tarGzMaxEntries)
		}
	}

	if entries == 0 {
		return errors.New("tar.gz assertion: archive contains no entries")
	}
	// metadata is reserved for future per-entry expectations (e.g. a declared file
	// count); currently unused but part of the ArtifactAssertion contract.
	_ = metadata
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
	Key          string            `json:"key"`
	Bucket       string            `json:"bucket"`
	LastModified time.Time         `json:"last_modified"`
	Size         int64             `json:"size"`
	ArtifactType ArtifactType      `json:"artifact_type"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// VerificationResult represents the result of verifying a single object.
type VerificationResult struct {
	Key          string             `json:"key"`
	Bucket       string             `json:"bucket"`
	Status       VerificationStatus `json:"status"`
	Path         VerificationPath   `json:"path"`
	Timestamp    time.Time          `json:"timestamp"`
	ArtifactType ArtifactType       `json:"artifact_type"`

	// Checksums
	ExpectedSHA256 string `json:"expected_sha256,omitempty"`
	ARMORSHA256    string `json:"armor_sha256,omitempty"`
	DirectSHA256   string `json:"direct_sha256,omitempty"`

	// Latency
	ARMORPathLatency  time.Duration `json:"armor_path_latency_ms"`
	DirectPathLatency time.Duration `json:"direct_path_latency_ms"`

	// Errors
	Error string `json:"error,omitempty"`

	// Assertion results
	AssertionPassed bool   `json:"assertion_passed"`
	AssertionError  string `json:"assertion_error,omitempty"`
}

// BucketState holds verification state for a single bucket.
type BucketState struct {
	mu sync.RWMutex

	Bucket              string    `json:"bucket"`
	LastVerification    time.Time `json:"last_verification"`
	LastSuccess         time.Time `json:"last_success"`
	VerifiedObjectRatio float64   `json:"verified_object_ratio"` // ratio of verified/total
	TotalObjects        int64     `json:"total_objects"`
	VerifiedObjects     int64     `json:"verified_objects"`
	FailedObjects       int64     `json:"failed_objects"`

	// Recent results (for debugging and escalation)
	RecentResults []VerificationResult `json:"recent_results"`

	// Configuration sample settings
	HistoricalSampleSize int `json:"historical_sample_size"`
}

// snapshot returns a copy of the state's data fields, excluding the mutex.
func (s *BucketState) snapshot() *BucketState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	c := &BucketState{
		Bucket:               s.Bucket,
		LastVerification:     s.LastVerification,
		LastSuccess:          s.LastSuccess,
		VerifiedObjectRatio:  s.VerifiedObjectRatio,
		TotalObjects:         s.TotalObjects,
		VerifiedObjects:      s.VerifiedObjects,
		FailedObjects:        s.FailedObjects,
		HistoricalSampleSize: s.HistoricalSampleSize,
		RecentResults:        make([]VerificationResult, len(s.RecentResults)),
	}
	copy(c.RecentResults, s.RecentResults)
	return c
}

// Verifier manages continuous restore verification across multiple buckets.
type Verifier struct {
	mu sync.RWMutex // protects buckets field

	backend   backend.Backend
	mek       []byte
	blockSize int
	manifest  *manifest.Index

	buckets       map[string]*BucketState // bucket name -> state
	bucketConfigs []BucketConfig          // configured buckets

	// Control
	stopCh chan struct{}
	doneCh chan struct{}

	// Configuration
	interval      time.Duration
	sampleSize    int    // number of objects to verify per run per bucket
	escrowMekPath string // path to escrowed MEK for direct decryption
	logOutput     io.Writer
}

// BucketConfig holds configuration for a single bucket verification.
type BucketConfig struct {
	Bucket               string       `json:"bucket"`
	Prefix               string       `json:"prefix,omitempty"`
	ArtifactType         ArtifactType `json:"artifact_type,omitempty"`
	Enabled              bool         `json:"enabled"`
	HistoricalSampleSize int          `json:"historical_sample_size,omitempty"`
}

// Config holds verifier configuration.
type Config struct {
	Buckets       []BucketConfig
	Interval      time.Duration
	SampleSize    int
	EscrowMEKPath string
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
				Bucket:               bucketCfg.Bucket,
				HistoricalSampleSize: bucketCfg.HistoricalSampleSize,
				RecentResults:        make([]VerificationResult, 0, 10),
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
		status[name] = state.snapshot()
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

	return state.snapshot(), nil
}
