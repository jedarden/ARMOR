// cmd_verify.go implements the 'armor verify' subcommand for object verification
package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

func init() {
	// Verification flags, on verify's own flag set. The storage vars for
	// -bucket/-mek/-mek-file/-escrow/-output/-v/-concurrency are shared with
	// other subcommands (cmd_client_config.go / cmd_decrypt.go /
	// cmd_migrate.go): verify reuses the same package-level vars so the
	// shared helpers (getMEKForVerify, writeReport) keep working, but each
	// FlagSet registers its own flag definitions — with separate flag sets
	// the same flag name on two commands no longer panics ("flag
	// redefined"), it just binds the same storage from each set. Only one
	// command's set is ever parsed in a real process.
	//
	// -concurrency keeps migrate's default of 0, which isn't valid for
	// verify's own semaphore sizing — see verifyConcurrency() below.
	verifyFlags := flag.NewFlagSet("verify", flag.ExitOnError)
	verifyFlags.StringVar(&bucketFlag, "bucket", "", "Bucket name to verify (required)")
	verifyFlags.StringVar(&mekFlag, "mek", "", "Master encryption key (hex, 64 chars)")
	verifyFlags.StringVar(&mekFileFlag, "mek-file", "", "Read MEK from file (hex, 64 chars)")
	verifyFlags.StringVar(&escrowFile, "escrow", "", "Path to escrow JSON file containing MEK, MEK ring and B2 credentials (self-contained recovery; the only multi-key source)")
	verifyFlags.StringVar(&outputFlag, "output", "", "Output report file path (default: stdout)")
	verifyFlags.BoolVar(&decryptVerboseFlag, "v", false, "Verbose output")
	verifyFlags.StringVar(&prefixFlag, "prefix", "", "Key prefix to filter objects (optional)")
	verifyFlags.StringVar(&keysFileFlag, "keys-file", "", "Path to JSON file listing object keys to verify (one per line or JSON array)")
	verifyFlags.StringVar(&sinceFlag, "since", "", "Only verify objects modified after this timestamp (RFC3339 or YYYY-MM-DD)")
	verifyFlags.BoolVar(&quickModeFlag, "quick", false, "Quick mode: verify envelope and DEK only, skip HMAC verification")
	verifyFlags.IntVar(&concurrencyFlag, "concurrency", 0, "Number of concurrent workers (default: 10 for verify)")
	// Same storage var decrypt's -b2-prefix binds: the ADR-001 prefix decides
	// which .armor/ objects List hides, and the multipart HMAC sidecar is
	// NAMED by the UNprefixed client key while the ciphertext lives under the
	// prefixed key — so verify strips the prefix before resolving a sidecar
	// (the server's loadV3MultipartSidecar does the same).
	verifyFlags.StringVar(&b2PrefixFlag, "b2-prefix", normalizePrefix(os.Getenv("ARMOR_PREFIX")), "ADR-001 key prefix the bucket stores objects under (default: ARMOR_PREFIX)")

	registerCommand(Command{
		Name:        "verify",
		Description: "Verify ARMOR-encrypted objects for corruption (HMAC + digest verification)",
		Flags:       verifyFlags,
		Func:        verify,
	})
}

// Verification-specific flags
var (
	prefixFlag    string
	keysFileFlag  string
	sinceFlag     string
	quickModeFlag bool
)

// verifyConcurrency returns the shared -concurrency flag's value, defaulting
// to 10 for verify's own use when unset (its shared default of 0 comes from
// cmd_migrate.go, where 0 means "server-side default" -- here it would size
// a zero-buffered channel and deadlock every verification goroutine).
func verifyConcurrency() int {
	if concurrencyFlag <= 0 {
		return 10
	}
	return concurrencyFlag
}

func verify(fs *flag.FlagSet) {
	if fs.NArg() > 0 {
		// fs.Args(), not flag.Args(): main never parses the top-level
		// CommandLine, so the global call would always print an empty list
		// and hide the stray argument the operator needs to see.
		fmt.Fprintf(os.Stderr, "Error: unexpected arguments after flags: %v\n", fs.Args())
		fmt.Fprintf(os.Stderr, "Usage: armor verify [flags]\n")
		os.Exit(2)
	}

	// Validate required flags
	if bucketFlag == "" {
		fmt.Fprintf(os.Stderr, "Error: -bucket is required\n")
		fmt.Fprintf(os.Stderr, "Usage: armor verify -bucket <bucket> [other flags]\n")
		os.Exit(2)
	}

	// Get MEK (plus the MEK ring when the escrow file carries one)
	mek, ringKeys, err := getMEKForVerify()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading MEK: %v\n", err)
		os.Exit(1)
	}
	keySource := newVerifyKeySource(mek, ringKeys)

	// Parse since timestamp if provided
	var sinceTime time.Time
	if sinceFlag != "" {
		sinceTime, err = parseTimestamp(sinceFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing -since timestamp: %v\n", err)
			os.Exit(1)
		}
	}

	// Initialize B2 backend
	b2, err := initB2BackendForVerify()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing B2: %v\n", err)
		os.Exit(1)
	}

	// Get object keys to verify
	objKeys, err := getKeysToVerify()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting keys to verify: %v\n", err)
		os.Exit(1)
	}

	if len(objKeys) == 0 {
		fmt.Fprintf(os.Stderr, "No keys to verify\n")
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Verifying %d objects from bucket %s\n", len(objKeys), bucketFlag)

	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Fprintf(os.Stderr, "\nReceived interrupt signal, shutting down...\n")
		cancel()
	}()

	// Run verification
	results := runVerification(ctx, b2, keySource, objKeys, sinceTime)

	// Write output report
	if err := writeReport(results, outputFlag); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing report: %v\n", err)
		os.Exit(1)
	}

	// Exit non-zero on any failures
	if code := verificationExitCode(results); code != 0 {
		fmt.Fprintf(os.Stderr, "\nVerification completed with failures: %d OK, %d CORRUPTED, %d ERROR\n",
			results.OKCount, results.CorruptedCount, results.ErrorCount)
		os.Exit(code)
	}

	fmt.Fprintf(os.Stderr, "\nVerification completed successfully: %d objects verified OK\n", results.OKCount)
}

// verificationExitCode returns verify's process exit code for a completed
// report: 1 when any object verified CORRUPTED or ended in ERROR, 0 otherwise.
// Kept as its own function so the contract — a bucket with any failure never
// exits 0 — is unit-testable without exec-ing the binary.
func verificationExitCode(report *VerificationReport) int {
	if report.CorruptedCount > 0 || report.ErrorCount > 0 {
		return 1
	}
	return 0
}

// verifyKeySource unwraps object DEKs exactly the way the server read path
// does (internal/server/handlers: UnwrapDEKByFingerprint with keymanager
// lookup and legacy trial fallback): a v2:<fp16>:<base64> wrapped DEK selects
// its MEK by fingerprint (active key first, then the escrow ring); a legacy
// base64 value is trial-unwrapped with the active key and then each ring key.
// Before armor-28965aa0's fingerprint rollout was mirrored here, every object
// written with a fingerprinted wrapped DEK failed verification as CORRUPTED
// (armor-da67956d).
type verifyKeySource struct {
	mek      []byte
	ringKeys []crypto.RingKeyEntry
}

func newVerifyKeySource(mek []byte, ringKeys []crypto.RingKeyEntry) *verifyKeySource {
	return &verifyKeySource{mek: mek, ringKeys: ringKeys}
}

// lookupMEK resolves a 16-hex-char MEK fingerprint to its key material: the
// active key when it matches, then the ring keys the escrow file carried.
func (s *verifyKeySource) lookupMEK(keyID, fingerprint string) ([]byte, bool) {
	// The CLI holds one active key (no per-key-id keymanager), so a non-empty
	// keyID has no named pool to consult — only the ring can still match.
	if keyID == "" && crypto.MEKFingerprint(s.mek) == fingerprint {
		return s.mek, true
	}
	for _, rk := range s.ringKeys {
		if rk.Fingerprint == fingerprint && rk.MEK != nil {
			return rk.MEK, true
		}
	}
	return nil, false
}

// legacyFallback trial-unwraps a legacy (unfingerprinted) wrapped DEK: active
// MEK first, then ring keys in order — the server's fallback order.
func (s *verifyKeySource) legacyFallback(wrappedDEK []byte) ([]byte, error) {
	dek, err := crypto.UnwrapDEK(s.mek, wrappedDEK)
	if err == nil {
		return dek, nil
	}
	for _, rk := range s.ringKeys {
		if rk.MEK == nil {
			continue
		}
		dek, err := crypto.UnwrapDEK(rk.MEK, wrappedDEK)
		if err == nil {
			return dek, nil
		}
	}
	return nil, fmt.Errorf("no key in active or ring can unwrap DEK: %w", err)
}

// unwrapDEK resolves the raw x-amz-meta-armor-wrapped-dek metadata value —
// v2:<fp16>:<base64> or legacy base64 — to the unwrapped DEK. The second
// return of UnwrapDEKByFingerprint (the fingerprint used) is not needed here.
func (s *verifyKeySource) unwrapDEK(wrappedDEKStr string) ([]byte, error) {
	dek, _, err := crypto.UnwrapDEKByFingerprint(wrappedDEKStr, s.lookupMEK, s.legacyFallback)
	return dek, err
}

// unwrapObjectDEK unwraps the object's DEK and, on failure, records the
// classified result. A fingerprint this run brought no key for is an ERROR
// (keying gap — the object may be fine); any other unwrap failure is CORRUPTED
// (the wrapped DEK bytes themselves will not unwrap).
func unwrapObjectDEK(ks *verifyKeySource, meta map[string]string, result *ObjectVerificationResult, startTime time.Time) ([]byte, bool) {
	wrappedDEKStr := meta["x-amz-meta-armor-wrapped-dek"]
	if wrappedDEKStr == "" {
		result.Status = "ERROR"
		result.Error = "Missing wrapped DEK in object metadata"
		result.Duration = time.Since(startTime).Seconds()
		return nil, false
	}
	dek, err := ks.unwrapDEK(wrappedDEKStr)
	if err == nil {
		return dek, true
	}
	if errors.Is(err, crypto.ErrFingerprintNotFound) {
		result.Status = "ERROR"
		result.Error = fmt.Sprintf("DEK unwrap failed: %v", err)
		result.Details = "Object names a MEK fingerprint not present in the active key or ring; supply it via -escrow"
	} else {
		result.Status = "CORRUPTED"
		result.Error = fmt.Sprintf("DEK unwrap failed: %v", err)
		result.Details = "Wrapped DEK appears corrupted or uses wrong MEK"
	}
	result.Duration = time.Since(startTime).Seconds()
	return nil, false
}

// getMEKForVerify loads the MEK (and, from an escrow file, the MEK ring)
// that verification unwraps DEKs with.
func getMEKForVerify() ([]byte, []crypto.RingKeyEntry, error) {
	var mek []byte
	var err error

	// Try escrow file first. loadEscrow (cmd_decrypt.go, same package) is the
	// shared escrow reader: it validates the MEK, decodes the mek_ring array
	// into real key material — the only multi-key input verify has — and
	// exports the B2 credentials into the environment for the backend init
	// below.
	if escrowFile != "" {
		mek, ringKeys, err := loadEscrow()
		if err != nil {
			return nil, nil, fmt.Errorf("loading escrow: %w", err)
		}
		return mek, ringKeys, nil
	}

	// Try MEK flags
	if mekFlag != "" {
		mek, err = hex.DecodeString(mekFlag)
		if err != nil {
			return nil, nil, fmt.Errorf("decoding MEK hex: %w", err)
		}
	} else if mekFileFlag != "" {
		data, err := os.ReadFile(mekFileFlag)
		if err != nil {
			return nil, nil, fmt.Errorf("reading MEK file: %w", err)
		}
		mek, err = hex.DecodeString(strings.TrimSpace(string(data)))
		if err != nil {
			return nil, nil, fmt.Errorf("decoding MEK from file: %w", err)
		}
	} else {
		// Try environment variable
		mekHex := os.Getenv("ARMOR_MEK")
		if mekHex == "" {
			return nil, nil, errors.New("no MEK provided: use -mek, -mek-file, -escrow, or ARMOR_MEK environment variable")
		}
		mek, err = hex.DecodeString(mekHex)
		if err != nil {
			return nil, nil, fmt.Errorf("decoding MEK from environment: %w", err)
		}
	}

	if len(mek) != 32 {
		return nil, nil, fmt.Errorf("invalid MEK length: got %d bytes, expected 32", len(mek))
	}

	return mek, nil, nil
}

// initB2BackendForVerify initializes B2 backend from environment
func initB2BackendForVerify() (backend.Backend, error) {
	region := os.Getenv("ARMOR_B2_REGION")
	endpoint := os.Getenv("ARMOR_B2_ENDPOINT")
	accessKey := os.Getenv("ARMOR_B2_ACCESS_KEY_ID")
	secretKey := os.Getenv("ARMOR_B2_SECRET_ACCESS_KEY")
	cfDomain := os.Getenv("ARMOR_CF_DOMAIN")

	if region == "" || endpoint == "" || accessKey == "" || secretKey == "" {
		return nil, errors.New("B2 credentials not set: provide -escrow file or set ARMOR_B2_REGION, ARMOR_B2_ENDPOINT, ARMOR_B2_ACCESS_KEY_ID, ARMOR_B2_SECRET_ACCESS_KEY")
	}

	return backend.NewB2Backend(context.Background(), backend.B2Config{
		Region:          region,
		Endpoint:        endpoint,
		AccessKeyID:     accessKey,
		SecretKey:       secretKey,
		CFDomain:        cfDomain,
		ReadConcurrency: verifyConcurrency(),
		// Hand the ADR-001 prefix to the backend so List hides the prefixed
		// .armor/ internal namespace, exactly as the server's List does.
		KeyPrefix: b2PrefixFlag,
	})
}

// getKeysToVerify returns list of object keys to verify
func getKeysToVerify() ([]string, error) {
	// If keys file specified, read from it
	if keysFileFlag != "" {
		return readKeysFromFile(keysFileFlag)
	}

	// Otherwise, list objects from bucket with prefix
	return listObjectsFromBucket()
}

// readKeysFromFile reads object keys from a file
func readKeysFromFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading keys file: %w", err)
	}

	// Try parsing as JSON array first
	var keys []string
	if err := json.Unmarshal(data, &keys); err == nil {
		return keys, nil
	}

	// Otherwise, treat as newline-separated list
	lines := strings.Split(string(data), "\n")
	keys = make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			keys = append(keys, line)
		}
	}

	return keys, nil
}

// listObjectsFromBucket lists objects from B2 with optional prefix filter,
// skipping ARMOR internal bookkeeping the way the restore-verifier's sampling
// does (armor-86a90341). The backend's List already hides the .armor/
// namespace; the ADR-016 manifest sidecars live OUTSIDE it (suffix appended to
// the data key), so they are filtered here — a manifest is a metadata copy,
// not ciphertext, and a bucket-wide verify would otherwise exit non-zero on
// every one of them.
func listObjectsFromBucket() ([]string, error) {
	b2, err := initB2BackendForVerify()
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	var keys []string
	skippedInternal := 0

	// List objects with prefix
	result, err := b2.List(ctx, bucketFlag, prefixFlag, "", "", 1000)
	if err != nil {
		return nil, fmt.Errorf("listing objects: %w", err)
	}

	for _, obj := range result.Objects {
		if strings.HasSuffix(obj.Key, verifyManifestSuffix) {
			skippedInternal++
			continue
		}
		keys = append(keys, obj.Key)
	}
	if skippedInternal > 0 {
		fmt.Fprintf(os.Stderr, "Skipping %d ARMOR manifest sidecar objects\n", skippedInternal)
	}

	return keys, nil
}

// parseTimestamp parses a timestamp string in multiple formats
func parseTimestamp(s string) (time.Time, error) {
	// Try RFC3339 first
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}

	// Try YYYY-MM-DD
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("unable to parse timestamp: %s", s)
}

// ObjectVerificationResult represents the result of verifying one object
type ObjectVerificationResult struct {
	Bucket    string    `json:"bucket"`
	Key       string    `json:"key"`
	Status    string    `json:"status"` // OK, CORRUPTED, ERROR
	Error     string    `json:"error,omitempty"`
	Details   string    `json:"details,omitempty"`
	SizeBytes int64     `json:"size_bytes"`
	ModTime   time.Time `json:"modification_time"`
	Duration  float64   `json:"duration_seconds"`
}

// VerificationReport is the complete verification output
type VerificationReport struct {
	Bucket         string                    `json:"bucket"`
	Prefix         string                    `json:"prefix,omitempty"`
	TotalObjects   int                       `json:"total_objects"`
	OKCount        int                       `json:"ok_count"`
	CorruptedCount int                       `json:"corrupted_count"`
	ErrorCount     int                       `json:"error_count"`
	QuickMode      bool                      `json:"quick_mode"`
	VerifyDate     time.Time                 `json:"verification_date"`
	Duration       float64                   `json:"duration_seconds"`
	Results        []ObjectVerificationResult `json:"results"`
}

// runVerification performs concurrent verification of objects
func runVerification(ctx context.Context, b2 backend.Backend, ks *verifyKeySource, keys []string, since time.Time) *VerificationReport {
	startTime := time.Now()
	report := &VerificationReport{
		Bucket:     bucketFlag,
		Prefix:     prefixFlag,
		TotalObjects: len(keys),
		QuickMode:  quickModeFlag,
		VerifyDate: startTime,
		Results:    make([]ObjectVerificationResult, 0, len(keys)),
	}

	// Create semaphore for concurrency
	sem := make(chan struct{}, verifyConcurrency())
	var wg sync.WaitGroup
	var okCount, corruptedCount, errorCount atomic.Int32

	// Result rows are appended from every worker goroutine, so the slice is
	// mutex-protected — a plain append from concurrent goroutines is a data
	// race, which is why the report never carried rows before (armor-da67956d:
	// one row per key, non-empty Error on failures).
	var resultsMu sync.Mutex
	results := make([]ObjectVerificationResult, 0, len(keys))

	// Process keys concurrently
	for _, key := range keys {
		wg.Add(1)
		go func(key string) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			// Verify object
			result := verifyObject(ctx, b2, ks, bucketFlag, key, since)

			// Update counters
			switch result.Status {
			case "OK":
				okCount.Add(1)
			case "CORRUPTED":
				corruptedCount.Add(1)
			case "ERROR":
				errorCount.Add(1)
			}

			// Print progress. Failure lines carry result.Error — the
			// diagnosable reason — falling back to Details for verbose OK rows.
			if decryptVerboseFlag || result.Status != "OK" {
				msg := result.Error
				if msg == "" {
					msg = result.Details
				}
				fmt.Fprintf(os.Stderr, "[%s] %s: %s\n", result.Status, key, msg)
			} else if okCount.Load()%100 == 0 {
				fmt.Fprintf(os.Stderr, "Progress: %d verified...\n", okCount.Load())
			}

			resultsMu.Lock()
			results = append(results, result)
			resultsMu.Unlock()
		}(key)
	}

	// Wait for all verifications to complete
	wg.Wait()

	// Deterministic row order: workers append in completion order, so sort by
	// key to make two runs over the same bucket diff cleanly.
	sort.Slice(results, func(i, j int) bool { return results[i].Key < results[j].Key })

	// Update counters
	report.OKCount = int(okCount.Load())
	report.CorruptedCount = int(corruptedCount.Load())
	report.ErrorCount = int(errorCount.Load())
	report.Duration = time.Since(startTime).Seconds()
	report.Results = results

	return report
}

// verifyObject performs full HMAC + digest verification of a single object
func verifyObject(ctx context.Context, b2 backend.Backend, ks *verifyKeySource, bucket, key string, since time.Time) ObjectVerificationResult {
	startTime := time.Now()
	result := ObjectVerificationResult{
		Bucket: bucket,
		Key:    key,
	}

	// Get object metadata
	info, err := b2.Head(ctx, bucket, key)
	if err != nil {
		result.Status = "ERROR"
		result.Error = fmt.Sprintf("Failed to get object metadata: %v", err)
		result.Duration = time.Since(startTime).Seconds()
		return result
	}

	result.SizeBytes = info.Size
	result.ModTime = info.LastModified

	// Check since filter
	if !since.IsZero() && info.LastModified.Before(since) {
		result.Status = "OK"
		result.Details = "Skipped (before -since timestamp)"
		result.Duration = time.Since(startTime).Seconds()
		return result
	}

	// Resolve the ARMOR parameters. The object's own metadata is authoritative
	// for single-PUT objects, but B2 never persists CreateMultipartUpload
	// metadata onto the finished large file, so a multipart-completed object
	// heads with an empty map and the ADR-016 manifest beside it is the
	// surviving source of the IV, wrapped DEK and sizes (armor-86a90341).
	// resolveVerifyMetadata probes head metadata first, then the manifest —
	// the same order the restore-verifier resolves with.
	meta, ok := resolveVerifyMetadata(ctx, b2, bucket, key, info.Metadata)
	if !ok {
		// No parsable wrapped DEK anywhere. If the object carries any ARMOR
		// headers at all, its metadata is damaged (a corrupted wrapped DEK
		// fails base64 inside ParseARMORMetadata) — that is corruption, not
		// a plain object; only a header-less object is genuinely not ARMOR.
		if hasArmorMetadataHeader(info.Metadata) {
			result.Status = "CORRUPTED"
			result.Error = "ARMOR metadata present but the wrapped DEK is missing or unparseable"
			result.Details = "Object carries ARMOR headers; x-amz-meta-armor-wrapped-dek is absent or will not decode"
		} else {
			result.Status = "ERROR"
			result.Error = "Object is not ARMOR encrypted"
		}
		result.Duration = time.Since(startTime).Seconds()
		return result
	}

	// Multipart-completed objects store headerless ciphertext with the
	// per-block HMACs in a JSON sidecar (ADR-003): dispatch on the marker
	// before any envelope-header read.
	if meta["x-amz-meta-armor-multipart"] == "true" {
		if quickModeFlag {
			return quickVerifyMultipartObject(ctx, b2, ks, bucket, key, info, meta, startTime)
		}
		return fullVerifyMultipartObject(ctx, b2, ks, bucket, key, info, meta, startTime)
	}

	// Quick mode: only verify envelope and DEK
	if quickModeFlag {
		return quickVerifyObject(ctx, b2, ks, bucket, key, info, meta, startTime)
	}

	// Full mode: verify HMAC + digest
	return fullVerifyObject(ctx, b2, ks, bucket, key, info, meta, startTime)
}

// quickVerifyObject verifies only envelope and DEK (fast check) for a
// single-PUT object. meta is the resolved ARMOR metadata (object metadata, or
// the ADR-016 manifest's copy when the object's own head carried none).
func quickVerifyObject(ctx context.Context, b2 backend.Backend, ks *verifyKeySource, bucket, key string, info *backend.ObjectInfo, meta map[string]string, startTime time.Time) ObjectVerificationResult {
	result := ObjectVerificationResult{
		Bucket:    bucket,
		Key:       key,
		SizeBytes: info.Size,
		ModTime:   info.LastModified,
	}

	// Read and verify envelope header
	reader, err := b2.GetRange(ctx, bucket, key, 0, 1024)
	if err != nil {
		result.Status = "ERROR"
		result.Error = fmt.Sprintf("Failed to read object: %v", err)
		result.Duration = time.Since(startTime).Seconds()
		return result
	}
	defer reader.Close()

	header := make([]byte, 1024)
	n, err := io.ReadFull(reader, header)
	if err != nil && err != io.ErrUnexpectedEOF {
		result.Status = "ERROR"
		result.Error = fmt.Sprintf("Failed to read header: %v", err)
		result.Duration = time.Since(startTime).Seconds()
		return result
	}
	if n < crypto.HeaderSize {
		result.Status = "CORRUPTED"
		result.Error = fmt.Sprintf("Object too small: only %d bytes, cannot contain valid envelope (need %d bytes)", n, crypto.HeaderSize)
		result.Duration = time.Since(startTime).Seconds()
		return result
	}

	// Parse envelope header
	envelope, err := crypto.DecodeHeader(header)
	if err != nil {
		result.Status = "CORRUPTED"
		result.Error = fmt.Sprintf("Envelope header corruption: %v", err)
		result.Details = "Cannot read encrypted envelope - object is unrecoverable"
		result.Duration = time.Since(startTime).Seconds()
		return result
	}

	// Verify envelope magic
	if string(envelope.Magic[:]) != crypto.Magic {
		result.Status = "CORRUPTED"
		result.Error = "Invalid ARMOR magic bytes"
		result.Duration = time.Since(startTime).Seconds()
		return result
	}

	// Verify envelope version
	if envelope.Version != crypto.Version1 && envelope.Version != crypto.Version2 && envelope.Version != crypto.Version3 {
		result.Status = "CORRUPTED"
		result.Error = fmt.Sprintf("Invalid envelope version: %d", envelope.Version)
		result.Duration = time.Since(startTime).Seconds()
		return result
	}

	// Verify the wrapped DEK unwraps. The metadata value may be
	// v2:<fp16>:<base64> (fingerprinted, armor-28965aa0) or legacy base64;
	// unwrapObjectDEK handles both with ring fallback.
	dek, ok := unwrapObjectDEK(ks, meta, &result, startTime)
	if !ok {
		return result
	}
	zeroBytes(dek)

	result.Status = "OK"
	result.Details = "Envelope and DEK verified successfully"
	result.Duration = time.Since(startTime).Seconds()
	return result
}

// fullVerifyObject performs complete HMAC + digest verification
func fullVerifyObject(ctx context.Context, b2 backend.Backend, ks *verifyKeySource, bucket, key string, info *backend.ObjectInfo, meta map[string]string, startTime time.Time) ObjectVerificationResult {
	result := ObjectVerificationResult{
		Bucket:    bucket,
		Key:       key,
		SizeBytes: info.Size,
		ModTime:   info.LastModified,
	}

	// First, do quick verification to ensure envelope is valid
	quickResult := quickVerifyObject(ctx, b2, ks, bucket, key, info, meta, startTime)
	if quickResult.Status != "OK" {
		// Return quick result - envelope or DEK is corrupted
		quickResult.Duration = time.Since(startTime).Seconds()
		return quickResult
	}

	// Unwrap DEK (fingerprint-directed with ring fallback, as in quick mode)
	dek, ok := unwrapObjectDEK(ks, meta, &result, startTime)
	if !ok {
		return result
	}
	defer zeroBytes(dek)

	// Read the entire object for HMAC verification
	objectReader, _, err := b2.Get(ctx, bucket, key)
	if err != nil {
		result.Status = "ERROR"
		result.Error = fmt.Sprintf("Failed to read object: %v", err)
		result.Duration = time.Since(startTime).Seconds()
		return result
	}
	defer objectReader.Close()

	// Read entire object into memory (for verification)
	// In production, you'd want to stream this with a buffered reader
	objectData, err := io.ReadAll(objectReader)
	if err != nil {
		result.Status = "ERROR"
		result.Error = fmt.Sprintf("Failed to read object data: %v", err)
		result.Duration = time.Since(startTime).Seconds()
		return result
	}

	// Parse envelope again to get verification parameters
	envelope, err := crypto.DecodeHeader(objectData[:crypto.HeaderSize])
	if err != nil {
		result.Status = "ERROR"
		result.Error = fmt.Sprintf("Failed to parse envelope: %v", err)
		result.Duration = time.Since(startTime).Seconds()
		return result
	}

	// Verify HMAC using version-specific decryption
	var plaintext []byte

	if envelope.Version == crypto.Version3 {
		// v3 format: use trailer block table
		// Layout: [header][encrypted blocks with varying sizes][block table]
		blockCount := crypto.ComputeBlockCount(int64(envelope.PlaintextSize), envelope.BlockSize())
		blockTableSize := int64(blockCount) * crypto.BlockTableEntrySize

		if int64(len(objectData)) < crypto.HeaderSize+blockTableSize {
			result.Status = "CORRUPTED"
			result.Error = fmt.Sprintf("Object too small for v3 block table: got %d bytes, need at least %d",
				len(objectData), crypto.HeaderSize+blockTableSize)
			result.Duration = time.Since(startTime).Seconds()
			return result
		}

		// Extract encrypted data (everything after header except block table)
		encryptedData := objectData[crypto.HeaderSize : len(objectData)-int(blockTableSize)]

		// Extract block table from trailer
		blockTableData := objectData[len(objectData)-int(blockTableSize):]
		blockTable, err := crypto.DecodeBlockTable(blockTableData, envelope.BlockSize(), blockCount)
		if err != nil {
			result.Status = "CORRUPTED"
			result.Error = fmt.Sprintf("Failed to decode v3 block table: %v", err)
			result.Duration = time.Since(startTime).Seconds()
			return result
		}

		// Decrypt v3 data (part=0 for single-PUT)
		decryptor, err := crypto.NewDecryptorWithVersion(dek, envelope.IV[:], envelope.BlockSize(), crypto.Version3)
		if err != nil {
			result.Status = "ERROR"
			result.Error = fmt.Sprintf("Failed to create v3 decryptor: %v", err)
			result.Duration = time.Since(startTime).Seconds()
			return result
		}

		plaintext, err = decryptor.DecryptV3(encryptedData, 0, blockTable)
		if err != nil {
			result.Status = "CORRUPTED"
			result.Error = fmt.Sprintf("v3 decryption failed: %v", err)
			result.Details = "Object data corruption detected - HMAC mismatch or decompression error"
			result.Duration = time.Since(startTime).Seconds()
			return result
		}
	} else {
		// v1/v2 format: inline HMAC table. HMAC verification happens inside
		// decryptor.Decrypt (it returns an error on any block's HMAC
		// mismatch, same as v3's DecryptV3 above) -- crypto.VerifyDecompression
		// is a standalone decompressed-bytes comparator now, not a ciphertext
		// HMAC checker, so there's no separate pre-decrypt verification step.
		decryptor, err := crypto.NewDecryptorWithVersion(dek, envelope.IV[:], envelope.BlockSize(), envelope.Version)
		if err != nil {
			result.Status = "ERROR"
			result.Error = fmt.Sprintf("Failed to create decryptor: %v", err)
			result.Duration = time.Since(startTime).Seconds()
			return result
		}

		encryptedData := objectData[crypto.HeaderSize:]
		blockCount := crypto.ComputeBlockCount(int64(envelope.PlaintextSize), envelope.BlockSize())
		hmacTableSize := int64(blockCount) * crypto.HMACSize
		hmacTable := encryptedData[len(encryptedData)-int(hmacTableSize):]
		encryptedData = encryptedData[:len(encryptedData)-int(hmacTableSize)]

		plaintext, err = decryptor.Decrypt(encryptedData, hmacTable)
		if err != nil {
			result.Status = "CORRUPTED"
			result.Error = fmt.Sprintf("Decryption failed: %v", err)
			result.Details = "Object data corruption detected - HMAC mismatch"
			result.Duration = time.Since(startTime).Seconds()
			return result
		}
	}

	// Verify the declared plaintext digest. Single-PUT objects declare the
	// plain SHA-256 of the whole plaintext; a part-size metadata value flips
	// the comparison to the combined per-part digest (the multipart form —
	// unreachable here, but the helper keeps the two paths identical).
	expectedSHA := meta["x-amz-meta-armor-plaintext-sha256"]
	if digestDeclaredForVerify(expectedSHA) {
		plaintextSHAHex := plaintextDigestForVerify(plaintext, meta)
		if expectedSHA != plaintextSHAHex {
			result.Status = "CORRUPTED"
			result.Error = fmt.Sprintf("SHA-256 mismatch: expected=%s, got=%s", expectedSHA, plaintextSHAHex)
			result.Details = "Plaintext checksum verification failed"
			result.Duration = time.Since(startTime).Seconds()
			return result
		}
	}

	result.Status = "OK"
	result.Details = "Full HMAC + digest verification passed"
	result.Duration = time.Since(startTime).Seconds()
	return result
}

// decodeBase64ToBytes already exists in cmd_check.go (byte-for-byte
// identical); reused directly instead of a duplicate declaration.

// zeroBytes securely zeros a byte slice
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// writeReport writes the verification report to file or stdout
func writeReport(report *VerificationReport, outputPath string) error {
	var output io.Writer

	if outputPath != "" {
		// Ensure directory exists
		dir := filepath.Dir(outputPath)
		if dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("creating output directory: %w", err)
			}
		}

		// *os.File satisfies both io.Writer and io.Closer -- kept as its
		// concrete type here so both assignments below typecheck; assigning
		// through the io.Writer-typed `output` var to an io.Closer var
		// doesn't (io.Writer's method set doesn't include Close()).
		f, err := os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("creating output file: %w", err)
		}
		defer f.Close()
		output = f
	} else {
		output = os.Stdout
		// No need to close os.Stdout
	}

	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}
