// Package restoreverifier implements continuous restore verification for ARMOR backups.
// It runs dual-path verification (ARMOR read path + armor-decrypt direct) to prove
// that backups are restorable through both the normal server path and disaster recovery.
package restoreverifier

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
	"github.com/jedarden/armor/internal/manifest"
	"github.com/jedarden/armor/internal/metrics"
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

// Mode selects which restore paths a verification run exercises.
type Mode string

const (
	// ModeDual runs both the ARMOR read path and the armor-decrypt direct path
	// and asserts they agree on every object. This is the default
	// continuous-verification mode (ADR-004).
	ModeDual Mode = "dual"

	// ModeDRDrill runs ONLY the direct-to-ciphertext path (MEK unwrap, raw B2
	// fetch, ADR-003-aware decrypt, checksum + artifact assertion) with the
	// ARMOR read path deliberately excluded. It automates the
	// "ARMOR-server-is-gone" restore drill from docs/disaster-recovery.md:
	// proving a fresh instance armed with only the escrowed MEK and B2
	// credentials can still recover ciphertext that no ARMOR server ever
	// touches during the run.
	ModeDRDrill Mode = "dr-drill"
)

// emptyStringSHA256Hex is the SHA-256 of the empty string. Before bf-1v2ehf,
// CompleteMultipartUpload wrote it as a placeholder plaintext digest for every
// multipart object (ADR-003 residual gap), so it could not be trusted as a real
// per-object checksum. New multipart uploads now store the real combined
// per-part digest (and x-amz-meta-armor-part-size), but objects written before
// the fix still carry this placeholder, so any checksum comparison must treat it
// (and an empty string) as "no digest declared" rather than a value to match.
const emptyStringSHA256Hex = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

// isPlaceholderPlaintextSHA reports whether a declared plaintext SHA-256 is
// absent or the legacy ADR-003 multipart placeholder, and therefore must not be
// enforced as a real checksum (legacy multipart objects written before bf-1v2ehf).
func isPlaceholderPlaintextSHA(s string) bool {
	return s == "" || s == emptyStringSHA256Hex
}

// plaintextDigestForMetadata returns the plaintext digest that should be
// compared against an object's declared x-amz-meta-armor-plaintext-sha256.
// Single-PUT objects declare the plain SHA-256 of the whole plaintext. Multipart
// objects (ADR-005) declare the combined per-part digest that
// CompleteMultipartUpload now stores, reproduced here by splitting the restored
// plaintext at the uniform part-size P boundaries (backend.ComputeMultipartDigest)
// — the order-sensitive combination that CombinePartPlaintextSHAs performs at
// Complete. P is read from x-amz-meta-armor-part-size; when absent or unparsable
// the plain whole-plaintext SHA-256 is used (single-PUT objects, and legacy
// multipart objects written before bf-1v2ehf that carry only the placeholder).
// This does not weaken the dual-path agreement check: both paths decrypt
// identical plaintext under identical metadata, so they always produce identical
// digests regardless of which form the object declares.
func plaintextDigestForMetadata(plaintext []byte, metadata map[string]string) string {
	if ps := metadata["x-amz-meta-armor-part-size"]; ps != "" {
		if partSize, err := strconv.ParseInt(ps, 10, 64); err == nil && partSize > 0 {
			return backend.ComputeMultipartDigest(plaintext, partSize)
		}
	}
	sum := sha256.Sum256(plaintext)
	return hex.EncodeToString(sum[:])
}

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

	// DR-drill state (ModeDRDrill). These are deliberately distinct from the
	// dual-path fields above: a direct-only drill run must never bump the
	// continuous-verification gauges, and the drill's own last-success
	// timestamp is queryable separately (drill_last_success). A drill that
	// succeeds while the ARMOR read path is down still records progress here
	// without claiming the dual path is healthy.
	DrillLastVerification time.Time `json:"drill_last_verification"`
	DrillLastSuccess      time.Time `json:"drill_last_success"`
	DrillTotalObjects     int64     `json:"drill_total_objects"`
	DrillVerifiedObjects  int64     `json:"drill_verified_objects"`
	DrillFailedObjects    int64     `json:"drill_failed_objects"`
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

		DrillLastVerification: s.DrillLastVerification,
		DrillLastSuccess:      s.DrillLastSuccess,
		DrillTotalObjects:     s.DrillTotalObjects,
		DrillVerifiedObjects:  s.DrillVerifiedObjects,
		DrillFailedObjects:    s.DrillFailedObjects,
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
	metrics   *metrics.Metrics // optional; receives per-bucket restorability gauges

	// escalator files one bead per distinct active failure and one staleness
	// bead per freshness window (ADR-004 §5). Nil = escalation disabled; every
	// call site is nil-guarded so a Config with no Escalator behaves exactly as
	// before. See escalation.go.
	escalator *Escalator

	buckets       map[string]*BucketState // bucket name -> state
	bucketConfigs []BucketConfig          // configured buckets

	// Control
	stopCh chan struct{}
	doneCh chan struct{}

	// Configuration
	interval      time.Duration
	drillInterval time.Duration // cadence of the periodic direct-only DR drill; 0 = disabled
	sampleSize    int           // number of objects to verify per run per bucket
	escrowMekPath string        // path to escrowed MEK for direct decryption
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
	Metrics       *metrics.Metrics // optional; when set, per-bucket gauges are published after each run

	// DRDrillInterval is the cadence of the periodic direct-only restore drill
	// (ModeDRDrill), independent of the dual-path Interval. Zero disables the
	// periodic drill; the drill can still be run on demand via the trigger
	// handler's ?mode=dr-drill query. Kept separate so a deployment can verify
	// both paths frequently yet exercise the ARMOR-server-is-gone recovery on
	// its own (typically longer) schedule.
	DRDrillInterval time.Duration

	// Escalator files verification-failure and staleness beads (ADR-004 §5).
	// Optional: nil disables escalation entirely, leaving verifier behavior
	// unchanged. Construct with NewEscalator (escalation.go).
	Escalator *Escalator
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
		metrics:       cfg.Metrics,
		escalator:     cfg.Escalator,
		buckets:       make(map[string]*BucketState),
		bucketConfigs: cfg.Buckets,
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
		interval:      cfg.Interval,
		drillInterval: cfg.DRDrillInterval,
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

	// The direct-only DR drill runs on its own schedule, independent of the
	// dual-path interval. A nil channel (drillInterval == 0) means the select
	// below never fires on it; the drill is still available on demand via the
	// trigger handler's ?mode=dr-drill query.
	var drillC <-chan time.Time
	if v.drillInterval > 0 {
		log.Printf("DR-drill (direct-only) enabled: interval %v", v.drillInterval)
		drillTicker := time.NewTicker(v.drillInterval)
		drillC = drillTicker.C
		defer drillTicker.Stop()
	}
	defer close(v.doneCh)

	// Run initial verification
	v.runVerification(ctx)
	if v.drillInterval > 0 {
		v.runDRDrill(ctx)
	}

	for {
		select {
		case <-ticker.C:
			v.runVerification(ctx)
		case <-drillC:
			v.runDRDrill(ctx)
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

// runVerification executes dual-path verification for all configured buckets.
func (v *Verifier) runVerification(ctx context.Context) {
	log.Println("Starting verification run")

	var wg sync.WaitGroup
	for bucketName, bucketState := range v.buckets {
		wg.Add(1)
		go func(bucket string, state *BucketState) {
			defer wg.Done()
			v.verifyBucket(ctx, bucket, state, ModeDual)
		}(bucketName, bucketState)
	}
	wg.Wait()

	log.Println("Verification run completed")
}

// runDRDrill executes a direct-only restore drill for all configured buckets.
// It reuses verifyBucket with ModeDRDrill so the sample selection and per-object
// loop are shared with the dual path; only the restore path exercised, the
// state fields written, and the metrics published differ.
func (v *Verifier) runDRDrill(ctx context.Context) {
	log.Println("Starting DR-drill (direct-only) verification run")

	var wg sync.WaitGroup
	for bucketName, bucketState := range v.buckets {
		wg.Add(1)
		go func(bucket string, state *BucketState) {
			defer wg.Done()
			v.verifyBucket(ctx, bucket, state, ModeDRDrill)
		}(bucketName, bucketState)
	}
	wg.Wait()

	log.Println("DR-drill verification run completed")
}

// verifyBucket verifies a single bucket. mode selects which restore path(s) the
// run exercises (ModeDual or ModeDRDrill); the per-object loop, sample
// selection, and recent-results bookkeeping are shared, while the state fields
// written and metrics published are mode-specific so a direct-only drill never
// bumps the dual-path restorability gauges (and vice versa).
func (v *Verifier) verifyBucket(ctx context.Context, bucket string, state *BucketState, mode Mode) {
	drill := mode == ModeDRDrill
	state.mu.Lock()
	// Single deferred tail: capture the run's final success timestamp under the
	// lock, release the lock, then run staleness escalation *outside* the
	// critical section so the bf shell-out (up to ExecTimeout) never blocks
	// concurrent /status readers of this bucket during an outage. Staleness is
	// one bead per freshness window (never per tick), deduped by the Escalator
	// itself, so calling it every tick cannot storm. One defer covers every exit
	// path, including the enumeration-failure early returns below. Escalation is
	// owned by the dual path: a drill failure will be re-found and filed by the
	// next dual run, so the drill never files beads (avoiding a second dedupe
	// key per object). Nil escalator = escalation disabled.
	defer func() {
		lastSuccess := state.LastSuccess // dual-path success; read while locked
		state.mu.Unlock()
		if v.escalator == nil || drill {
			return
		}
		if id, err := v.escalator.EscalateStaleness(ctx, bucket, lastSuccess); err != nil {
			log.Printf("Staleness escalation failed for bucket %s: %v", bucket, err)
		} else if id != "" {
			log.Printf("Escalated staleness for bucket %s to bead %s", bucket, id)
		}
	}()

	if drill {
		log.Printf("DR-drilling bucket (direct-only): %s", bucket)
	} else {
		log.Printf("Verifying bucket: %s", bucket)
	}

	// Get most recent backup object (should be the latest generation)
	latest, err := v.getLatestObject(ctx, bucket)
	if err != nil {
		log.Printf("Failed to get latest object for bucket %s: %v", bucket, err)
		// Record the attempt as a failure so a bucket that cannot be enumerated
		// still advances its restore-age gauge and trips the verification-failure
		// alert instead of silently emitting no series.
		v.recordFailedEnumeration(bucket, state, drill)
		return
	}

	// Get historical sample
	historical, err := v.getHistoricalSample(ctx, bucket, state.HistoricalSampleSize)
	if err != nil {
		log.Printf("Failed to get historical sample for bucket %s: %v", bucket, err)
		v.recordFailedEnumeration(bucket, state, drill)
		return
	}

	total := int64(1 + len(historical))
	objectsToVerify := append([]ObjectSample{latest}, historical...)

	// Per-run tallies. The restorability gauges must reflect this run's sample,
	// not the cumulative state counters (which grow without bound across runs
	// because objects are re-verified every cycle).
	var runVerified, runFailed int64

	// Verify each object
	for _, obj := range objectsToVerify {
		result := v.verifyObject(ctx, obj, mode)

		// Update state (mode-specific fields so the drill and dual paths keep
		// independent success/failure ledgers).
		if result.Status == StatusPass {
			runVerified++
			if drill {
				state.DrillVerifiedObjects++
				state.DrillLastSuccess = result.Timestamp
			} else {
				state.VerifiedObjects++
				state.LastSuccess = result.Timestamp
			}
		} else {
			runFailed++
			if drill {
				state.DrillFailedObjects++
			} else {
				state.FailedObjects++
			}
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

		// Escalation (ADR-004 §5): one bead per distinct active failure
		// (dedupe key = bucket + object key + path + failure class). A passing
		// object clears its keys so a genuine regression after recovery files a
		// fresh bead. The Escalator is storm-proof across ticks — it persists the
		// dedupe set, so a failing object never files more than one bead, and it
		// never retries (a failed filing leaves the key unrecorded; the next tick
		// may make one further bounded attempt, never a loop). Driven only by the
		// dual path (see the deferred staleness tail above); nil when disabled.
		if !drill {
			v.escalateResult(ctx, obj, result)
		}
	}

	now := time.Now()
	if drill {
		state.DrillLastVerification = now
		state.DrillTotalObjects = total
		v.recordDRDrillRun(bucket, state, runVerified)
		log.Printf("Bucket %s DR-drill complete: %d/%d recovered direct-only",
			bucket, runVerified, total)
		return
	}

	state.LastVerification = now
	state.TotalObjects = total

	// VerifiedObjectRatio reflects this run's sample so it stays in [0,1] and
	// the restorability PrometheusRule behaves correctly. (The state keeps the
	// cumulative Verified/Failed counters for the /status API and for the
	// monotonic failure counter below.)
	if state.TotalObjects > 0 {
		state.VerifiedObjectRatio = float64(runVerified) / float64(state.TotalObjects)
	}

	// Publish the per-bucket restorability gauges that back the restore-age and
	// verification-failure PrometheusRules. recordBucketRun computes the run
	// ratio from runVerified and forwards the monotonic state.FailedObjects, so
	// increase(armor_restore_verification_failures_total[window]) > 0 detects
	// new failures and resolves once they stop.
	v.recordBucketRun(bucket, state, runVerified)

	log.Printf("Bucket %s verification complete: %d/%d verified (%.1f%%)",
		bucket, runVerified, state.TotalObjects, state.VerifiedObjectRatio*100)
}

// recordFailedEnumeration advances a bucket's restore-age gauge after a run that
// could not even enumerate objects, so an unlistable bucket still trips its
// failure alert instead of silently emitting no series. drill selects whether
// the dual-path or DR-drill ledger/metrics are advanced.
func (v *Verifier) recordFailedEnumeration(bucket string, state *BucketState, drill bool) {
	now := time.Now()
	if drill {
		state.DrillLastVerification = now
		state.DrillTotalObjects = 0
		state.DrillFailedObjects++
		v.recordDRDrillRun(bucket, state, 0)
		return
	}
	state.LastVerification = now
	state.FailedObjects++
	v.recordBucketRun(bucket, state, 0)
}

// escalateResult files (or clears) escalation state for a single object's
// result. It is the sole caller of Escalator.EscalateFailure / ClearObject, so
// the dedupe invariant holds: each distinct active failure gets exactly one
// bead, and a recovered object is re-armed so a future regression files fresh.
// Provenance is the object's ARMOR envelope version from its metadata, captured
// "where available" (ADR-004 §5); WriterID stays empty until the provenance
// chain is wired into the verifier. No-op when escalation is disabled (nil).
//
// This never loops or retries: a failed filing returns an error and leaves the
// dedupe key unrecorded, so the next scheduler tick may make one further
// bounded attempt — bounded by the schedule cadence, never an unbounded retry.
func (v *Verifier) escalateResult(ctx context.Context, obj ObjectSample, result VerificationResult) {
	if v.escalator == nil {
		return
	}
	if result.Status == StatusPass {
		// Object recovered: drop its active-failure keys so a later regression
		// files a fresh bead instead of being deduped away.
		v.escalator.ClearObject(obj.Bucket, obj.Key)
		return
	}
	prov := Provenance{EnvelopeVersion: obj.Metadata["x-amz-meta-armor-version"]}
	id, err := v.escalator.EscalateFailure(ctx, result, prov)
	if err != nil {
		log.Printf("Escalation failed for %s/%s (%s): %v", obj.Bucket, obj.Key, result.Status, err)
		return
	}
	if id != "" {
		log.Printf("Escalated %s/%s (%s via %s path) to bead %s",
			obj.Bucket, obj.Key, result.Status, result.Path, id)
	}
}

// recordBucketRun publishes the per-bucket restorability gauges after a
// verification run — including runs that failed before any object could be
// verified (runVerified == 0), so an unenumerable bucket still advances its
// restore-age gauge and trips the verification-failure alert instead of
// silently emitting no series. state.LastVerification must already be set to
// this attempt's timestamp; state.FailedObjects is forwarded unchanged as the
// monotonic counter backing the failure alert.
func (v *Verifier) recordBucketRun(bucket string, state *BucketState, runVerified int64) {
	if v.metrics == nil {
		return
	}
	var runRatio float64
	if state.TotalObjects > 0 {
		runRatio = float64(runVerified) / float64(state.TotalObjects)
	}
	v.metrics.RecordRestoreBucketState(bucket, state.LastVerification, runRatio, state.FailedObjects)
}

// recordDRDrillRun publishes the per-bucket direct-only DR-drill gauges after a
// drill run — including runs that failed before any object could be recovered
// (runVerified == 0), so an unenumerable bucket still advances its
// drill-restore-age and trips the drill-failure signal. Mirrors recordBucketRun:
// the run ratio is computed from runVerified against this run's
// state.DrillTotalObjects, and state.DrillFailedObjects is forwarded as the
// monotonic counter so drill_failures_total only climbs on a new failure.
// state.DrillLastVerification and DrillLastSuccess must already reflect this
// attempt.
func (v *Verifier) recordDRDrillRun(bucket string, state *BucketState, runVerified int64) {
	if v.metrics == nil {
		return
	}
	var runRatio float64
	if state.DrillTotalObjects > 0 {
		runRatio = float64(runVerified) / float64(state.DrillTotalObjects)
	}
	v.metrics.RecordDRDrillRun(bucket, state.DrillLastVerification, state.DrillLastSuccess, runRatio, state.DrillFailedObjects)
}

// verifyObject verifies a single object. mode selects which restore path(s) the
// run exercises: ModeDual runs both the ARMOR read path and the direct decrypt
// and asserts they agree; ModeDRDrill runs ONLY the direct-to-ciphertext path,
// deliberately excluding the ARMOR read path so the run proves recovery works
// with the ARMOR server gone.
func (v *Verifier) verifyObject(ctx context.Context, obj ObjectSample, mode Mode) VerificationResult {
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

	if mode == ModeDRDrill {
		return v.verifyObjectDirectOnly(ctx, obj, result, expectedSHA256)
	}
	return v.verifyObjectDual(ctx, obj, result, expectedSHA256)
}

// verifyObjectDirectOnly is the DR-drill path (ModeDRDrill): it exercises ONLY
// the armor-decrypt direct route — MEK unwrap, raw B2 fetch, ADR-003-aware
// decrypt, checksum + artifact assertion — with the ARMOR read path
// (restoreViaARMOR) deliberately never called. This is the automated
// "ARMOR-server-is-gone" drill from docs/disaster-recovery.md: a fresh instance
// armed with only the escrowed MEK and B2 credentials recovers ciphertext that
// no ARMOR server touches during the run. result.Path stays PathDirect to make
// the direct-only nature visible in /status and escalation beads.
func (v *Verifier) verifyObjectDirectOnly(ctx context.Context, obj ObjectSample, result VerificationResult, expectedSHA256 string) VerificationResult {
	directStart := time.Now()
	plaintext, err := v.restoreViaDirectDecrypt(ctx, obj.Bucket, obj.Key)
	result.DirectPathLatency = time.Since(directStart)

	if err != nil {
		result.Error = fmt.Sprintf("Direct path failed: %v", err)
		result.Status = StatusRestoreError
		result.Path = PathDirect
		return result
	}

	result.DirectSHA256 = plaintextDigestForMetadata(plaintext, obj.Metadata)

	// Enforce the declared plaintext checksum only when the object actually
	// declared a real digest: legacy ADR-003 multipart uploads written before
	// bf-1v2ehf carry the SHA-256 of the empty string as a placeholder digest, so
	// treat that value (and an absent one) as "no digest declared" rather than
	// something to match. New multipart objects declare the combined per-part
	// digest, which plaintextDigestForMetadata reproduces.
	if !isPlaceholderPlaintextSHA(expectedSHA256) && result.DirectSHA256 != expectedSHA256 {
		result.Status = StatusChecksumError
		result.Path = PathDirect
		result.Error = fmt.Sprintf("SHA256 mismatch: expected=%s, got=%s",
			expectedSHA256, result.DirectSHA256)
		return result
	}

	// Application-level assertion: the only check beyond SHA-256, so a corrupt
	// artifact whose checksum happens to be internally consistent is still caught.
	assertion := v.getAssertion(obj.ArtifactType)
	if assertionErr := assertion.Verify(plaintext, obj.Metadata); assertionErr != nil {
		result.Status = StatusAssertionError
		result.Path = PathDirect
		result.Error = fmt.Sprintf("Assertion failed: %v", assertionErr)
		result.AssertionPassed = false
		result.AssertionError = assertionErr.Error()
		return result
	}

	result.Status = StatusPass
	result.Path = PathDirect
	result.AssertionPassed = true
	return result
}

// verifyObjectDual is the continuous-verification path (ModeDual, ADR-004): it
// runs both the ARMOR read path and the armor-decrypt direct route and asserts
// they agree on every object before honoring the checksum and artifact
// assertion. result.Path is PathDualMatch only when both paths agree and pass.
func (v *Verifier) verifyObjectDual(ctx context.Context, obj ObjectSample, result VerificationResult, expectedSHA256 string) VerificationResult {
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

	// Compute the metadata-aware digest of the ARMOR path result so it is
	// directly comparable to the declared digest (plain SHA-256 for single-PUT,
	// combined per-part digest for multipart).
	result.ARMORSHA256 = plaintextDigestForMetadata(armorPlaintext, obj.Metadata)

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

	// Compute the metadata-aware digest of the direct path result, matching the
	// form used for the ARMOR path so the dual-path agreement check below stays a
	// like-for-like comparison.
	result.DirectSHA256 = plaintextDigestForMetadata(directPlaintext, obj.Metadata)

	// Compare dual-path results
	if result.ARMORSHA256 != result.DirectSHA256 {
		result.Status = StatusConflict
		result.Path = PathDirect // Use Direct to indicate the conflict path
		result.Error = fmt.Sprintf("SHA256 mismatch: ARMOR=%s, Direct=%s",
			result.ARMORSHA256, result.DirectSHA256)
		return result
	}

	// Both paths agree; verify against the declared checksum. As on the drill
	// path, the legacy ADR-003 multipart placeholder (and an absent value) must
	// not be enforced as a real per-object digest. New multipart objects declare
	// the combined per-part digest, which plaintextDigestForMetadata reproduces.
	if !isPlaceholderPlaintextSHA(expectedSHA256) && result.ARMORSHA256 != expectedSHA256 {
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

// restoreViaDirectDecrypt restores an object using direct B2 access + armor-decrypt
// logic. This simulates the "ARMOR server is gone" disaster recovery scenario by
// decrypting directly from B2 ciphertext using the escrowed MEK, touching only
// backend primitives (Head/GetRange/GetDirect) that a fresh instance with B2
// credentials and the MEK would have — never the ARMOR read path.
//
// It honors both on-B2 layouts ARMOR writes (ADR-003):
//
//   - Single-PUT objects: [64-byte envelope header][encrypted blocks][inline
//     HMAC table]. IV and plaintext SHA come from the header.
//   - Multipart-completed objects: raw concatenated part ciphertext with NO
//     envelope header (plaintext offset N == ciphertext offset N) and the
//     per-block HMAC table in a JSON sidecar at .armor/hmac/<sha256(key)>. IV
//     comes from object metadata; the dispatch marker is
//     x-amz-meta-armor-multipart: true.
//
// A reader that assumes every object has the envelope layout fails on every
// multipart object (bf-24sxh7); the marker dispatch below is what ADR-003
// requires of every ARMOR reader, including this one.
func (v *Verifier) restoreViaDirectDecrypt(ctx context.Context, bucket, key string) ([]byte, error) {
	// Step 1: object metadata -> ARMOR encryption parameters.
	info, err := v.backend.Head(ctx, bucket, key)
	if err != nil {
		return nil, fmt.Errorf("direct path: HeadObject failed: %w", err)
	}
	armorMeta, ok := backend.ParseARMORMetadata(info.Metadata)
	if !ok {
		return nil, errors.New("direct path: object is not ARMOR-encrypted")
	}

	// Step 2: unwrap the DEK with the escrowed MEK.
	dek, err := crypto.UnwrapDEK(v.mek, armorMeta.WrappedDEK)
	if err != nil {
		return nil, fmt.Errorf("direct path: failed to unwrap DEK: %w", err)
	}

	// Step 3: gather ciphertext, HMAC table, and IV per the object's layout.
	isMultipart := info.Metadata["x-amz-meta-armor-multipart"] == "true"
	var (
		encryptedData []byte
		hmacTable     []byte
		iv            []byte
		header        *crypto.EnvelopeHeader // single-PUT only; nil for multipart
	)
	if isMultipart {
		encryptedData, hmacTable, iv, err = v.readMultipartCiphertext(ctx, bucket, key, armorMeta)
	} else {
		encryptedData, hmacTable, iv, header, err = v.readEnvelopeCiphertext(ctx, bucket, key, armorMeta)
	}
	if err != nil {
		return nil, err
	}

	// Step 4: decrypt with per-block HMAC verification (CTR mode: ciphertext
	// length equals plaintext length).
	decryptor, err := crypto.NewDecryptor(dek, iv, armorMeta.BlockSize)
	if err != nil {
		return nil, fmt.Errorf("direct path: failed to create decryptor: %w", err)
	}
	plaintext, err := decryptor.Decrypt(encryptedData, hmacTable)
	if err != nil {
		return nil, fmt.Errorf("direct path: decryption failed: %w (possible data corruption or wrong MEK)", err)
	}

	// Step 5: verify the plaintext digest. Single-PUT objects carry the true
	// whole-object SHA in the envelope header, so check it here. Multipart objects
	// have no envelope header — their real whole-object digest lives in metadata
	// (the combined per-part digest since bf-1v2ehf, recorded with
	// x-amz-meta-armor-part-size), which verifyObject enforces via
	// plaintextDigestForMetadata rather than this header check.
	if header != nil {
		if err := header.VerifyPlaintextSHA(plaintext); err != nil {
			return nil, fmt.Errorf("direct path: plaintext SHA-256 verification failed: %w", err)
		}
	}

	return plaintext, nil
}

// readEnvelopeCiphertext reads a single-PUT object: a 64-byte envelope header
// (decoded for the IV), the encrypted blocks immediately after it, and the
// inline HMAC table trailing the ciphertext. Returns the decoded header so the
// caller can run header.VerifyPlaintextSHA on the decrypted plaintext.
func (v *Verifier) readEnvelopeCiphertext(ctx context.Context, bucket, key string, armorMeta *backend.ARMORMetadata) (encryptedData, hmacTable, iv []byte, header *crypto.EnvelopeHeader, err error) {
	// Envelope header (64 bytes) at offset 0.
	headerReader, err := v.backend.GetRange(ctx, bucket, key, 0, crypto.HeaderSize)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("direct path: failed to read envelope header: %w", err)
	}
	defer headerReader.Close()
	headerBuf := make([]byte, crypto.HeaderSize)
	if _, err := io.ReadFull(headerReader, headerBuf); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("direct path: failed to read header bytes: %w", err)
	}
	header, err = crypto.DecodeHeader(headerBuf)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("direct path: failed to decode envelope header: %w", err)
	}

	// Encrypted data at offset HeaderSize; CTR mode keeps ciphertext == plaintext size.
	encryptedData = make([]byte, armorMeta.PlaintextSize)
	dataReader, err := v.backend.GetRange(ctx, bucket, key, crypto.HeaderSize, armorMeta.PlaintextSize)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("direct path: failed to read encrypted data: %w", err)
	}
	defer dataReader.Close()
	if _, err := io.ReadFull(dataReader, encryptedData); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("direct path: failed to read encrypted bytes: %w", err)
	}

	// Inline HMAC table trailing the ciphertext: one HMACSize entry per block.
	blockCount := crypto.ComputeBlockCount(armorMeta.PlaintextSize, armorMeta.BlockSize)
	hmacSize := int64(blockCount) * crypto.HMACSize
	hmacOffset := crypto.HeaderSize + armorMeta.PlaintextSize
	hmacReader, err := v.backend.GetRange(ctx, bucket, key, hmacOffset, hmacSize)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("direct path: failed to read HMAC table: %w", err)
	}
	defer hmacReader.Close()
	hmacTable = make([]byte, hmacSize)
	if _, err := io.ReadFull(hmacReader, hmacTable); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("direct path: failed to read HMAC bytes: %w", err)
	}

	return encryptedData, hmacTable, header.IV[:], header, nil
}

// readMultipartCiphertext reads an ADR-003 multipart-completed object: raw
// concatenated part ciphertext at offset 0 (no envelope header; plaintext
// offset N == ciphertext offset N) and the per-block HMAC table loaded from the
// JSON sidecar at .armor/hmac/<sha256(key)>. The IV is carried by object
// metadata (there is no header byte stream to read it from). The sidecar is
// loaded through the same MultipartStateManager the server uses, so the JSON
// wire format is shared exactly; its per-block HMACs are flattened into the
// contiguous raw table the Decryptor consumes.
func (v *Verifier) readMultipartCiphertext(ctx context.Context, bucket, key string, armorMeta *backend.ARMORMetadata) (encryptedData, hmacTable, iv []byte, err error) {
	if len(armorMeta.IV) == 0 {
		return nil, nil, nil, errors.New("direct path: multipart object missing IV metadata")
	}

	// Raw ciphertext at offset 0; CTR mode keeps ciphertext == plaintext size.
	encryptedData = make([]byte, armorMeta.PlaintextSize)
	dataReader, err := v.backend.GetRange(ctx, bucket, key, 0, armorMeta.PlaintextSize)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("direct path: failed to read multipart ciphertext: %w", err)
	}
	defer dataReader.Close()
	if _, err := io.ReadFull(dataReader, encryptedData); err != nil {
		return nil, nil, nil, fmt.Errorf("direct path: failed to read multipart ciphertext bytes: %w", err)
	}

	// HMAC table from the JSON sidecar, flattened to one HMACSize entry per block.
	sidecar, err := backend.NewMultipartStateManager(v.backend, bucket).LoadHMACTable(ctx, key)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("direct path: failed to load multipart HMAC sidecar: %w", err)
	}
	flat := make([]byte, 0, len(sidecar.BlockHMACs)*crypto.HMACSize)
	for _, h := range sidecar.BlockHMACs {
		flat = append(flat, h...)
	}

	return encryptedData, flat, armorMeta.IV, nil
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

// getHistoricalSample returns a cryptographically uniform random sample of
// historical backup objects drawn from the ENTIRE bucket, not just the first
// List page. It paginates through every object (honoring IsTruncated /
// NextToken) and feeds the stream into a reservoir sampler (Algorithm R), so
// memory is bounded by sampleSize regardless of how large the bucket grows and
// every object — old, new, or oddly-prefixed — has an equal sampleSize/N chance
// of being restore-verified each cycle. That uniformity is what makes the
// Phase 6 / ADR-004 restorability guarantee meaningful: no subset of objects
// can be permanently starved of verification the way a fixed tail slice would.
//
// Internal .armor/ bookkeeping objects are skipped (they are not user backups).
// The latest object is fetched separately by getLatestObject and verified
// unconditionally; it is not excluded here and may also appear in the sample.
func (v *Verifier) getHistoricalSample(ctx context.Context, bucket string, sampleSize int) ([]ObjectSample, error) {
	if sampleSize <= 0 {
		return nil, nil
	}

	// The reservoir holds at most sampleSize objects, so paginating a bucket
	// with millions of objects never grows memory beyond the sample size.
	reservoir := make([]ObjectSample, 0, sampleSize)
	var seen int // candidate (non-internal) objects fed to the sampler

	var continuationToken string
	for {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("historical sample cancelled: %w", err)
		}

		listResult, err := v.backend.List(ctx, bucket, "", "", continuationToken, 1000)
		if err != nil {
			return nil, fmt.Errorf("list failed: %w", err)
		}

		for _, obj := range listResult.Objects {
			if strings.HasPrefix(obj.Key, ".armor/") {
				continue
			}
			seen++
			sample := ObjectSample{
				Key:          obj.Key,
				Bucket:       bucket,
				LastModified: obj.LastModified,
				Size:         obj.Size,
				ArtifactType: v.inferArtifactType(obj.Key, obj.Metadata),
				Metadata:     obj.Metadata,
			}
			if seen <= sampleSize {
				// Fill phase: the first sampleSize candidates seed the reservoir.
				reservoir = append(reservoir, sample)
				continue
			}
			// Replacement phase (Algorithm R): for the seen-th candidate, draw a
			// uniform slot j in [0, seen) and replace reservoir[j] when j falls
			// inside the reservoir. Each candidate ends up retained with
			// probability sampleSize/N, giving a uniform sample of the full set.
			if j := cryptoRandInt(seen); j < sampleSize {
				reservoir[j] = sample
			}
		}

		if !listResult.IsTruncated {
			break
		}
		// Guard against a backend that reports truncated without advancing the
		// continuation token — stop rather than loop forever.
		if listResult.NextToken == continuationToken {
			break
		}
		continuationToken = listResult.NextToken
	}

	return reservoir, nil
}

// cryptoRandInt returns a uniform random int in the half-open interval [0, n)
// using crypto/rand, so the historical sample is unbiased and unpredictable —
// backing the ADR-004 guarantee that no objects are systematically starved of
// verification. On the essentially-impossible crypto/rand failure it returns 0
// rather than panicking; the reservoir sampler then replaces only slot 0 for
// that draw, a safe degradation that skews a single sample instead of crashing
// a verification run.
func cryptoRandInt(n int) int {
	if n <= 0 {
		return 0
	}
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0
	}
	return int(binary.LittleEndian.Uint64(b[:]) % uint64(n))
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
