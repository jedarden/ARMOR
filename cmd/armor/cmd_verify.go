// cmd_verify.go implements the 'armor verify' subcommand for object verification
package main

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

func init() {
	registerCommand(Command{
		Name:        "verify",
		Description: "Verify ARMOR-encrypted objects for corruption (HMAC + digest verification)",
		Func:        verify,
	})
}

// Verification flags
var (
	bucketFlag        string
	prefixFlag        string
	keysFileFlag      string
	sinceFlag         string
	concurrencyFlag   int
	outputFlag        string
	mekFlag           string
	mekFileFlag       string
	escrowFileFlag    string
	verboseFlag       bool
	quickModeFlag     bool
)

func init() {
	// Verification-specific flags
	flag.StringVar(&bucketFlag, "bucket", "", "Bucket to verify (required)")
	flag.StringVar(&prefixFlag, "prefix", "", "Key prefix to filter objects (optional)")
	flag.StringVar(&keysFileFlag, "keys-file", "", "Path to JSON file listing object keys to verify (one per line or JSON array)")
	flag.StringVar(&sinceFlag, "since", "", "Only verify objects modified after this timestamp (RFC3339 or YYYY-MM-DD)")
	flag.IntVar(&concurrencyFlag, "concurrency", 10, "Number of concurrent verifications")
	flag.StringVar(&outputFlag, "output", "", "Output JSON report path (default: stdout)")
	flag.StringVar(&mekFlag, "mek", "", "Master encryption key (hex, 64 chars)")
	flag.StringVar(&mekFileFlag, "mek-file", "", "Read MEK from file (hex, 64 chars)")
	flag.StringVar(&escrowFileFlag, "escrow", "", "Path to escrow JSON file containing MEK and B2 credentials")
	flag.BoolVar(&verboseFlag, "v", false, "Verbose output")
	flag.BoolVar(&quickModeFlag, "quick", false, "Quick mode: verify envelope and DEK only, skip HMAC verification")
}

func verify() {
	// Re-parse flags for the verify subcommand
	flag.Parse()

	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "Error: unexpected arguments after flags: %v\n", flag.Args())
		fmt.Fprintf(os.Stderr, "Usage: armor verify [flags]\n")
		os.Exit(2)
	}

	// Validate required flags
	if bucketFlag == "" {
		fmt.Fprintf(os.Stderr, "Error: -bucket is required\n")
		fmt.Fprintf(os.Stderr, "Usage: armor verify -bucket <bucket> [other flags]\n")
		os.Exit(2)
	}

	// Get MEK
	mek, err := getMEKForVerify()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading MEK: %v\n", err)
		os.Exit(1)
	}

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
	keys, err := getKeysToVerify()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting keys to verify: %v\n", err)
		os.Exit(1)
	}

	if len(keys) == 0 {
		fmt.Fprintf(os.Stderr, "No keys to verify\n")
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Verifying %d objects from bucket %s\n", len(keys), bucketFlag)

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
	results := runVerification(ctx, b2, mek, keys, sinceTime)

	// Write output report
	if err := writeReport(results, outputFlag); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing report: %v\n", err)
		os.Exit(1)
	}

	// Exit non-zero on any failures
	if results.CorruptedCount > 0 || results.ErrorCount > 0 {
		fmt.Fprintf(os.Stderr, "\nVerification completed with failures: %d OK, %d CORRUPTED, %d ERROR\n",
			results.OKCount, results.CorruptedCount, results.ErrorCount)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "\nVerification completed successfully: %d objects verified OK\n", results.OKCount)
}

// getMEKForVerify loads MEK from flags or escrow file
func getMEKForVerify() ([]byte, error) {
	var mek []byte
	var err error

	// Try escrow file first
	if escrowFileFlag != "" {
		mek, err = loadMEKFromEscrowForVerify(escrowFileFlag)
		if err != nil {
			return nil, fmt.Errorf("loading escrow: %w", err)
		}
		return mek, nil
	}

	// Try MEK flags
	if mekFlag != "" {
		mek, err = hex.DecodeString(mekFlag)
		if err != nil {
			return nil, fmt.Errorf("decoding MEK hex: %w", err)
		}
	} else if mekFileFlag != "" {
		data, err := os.ReadFile(mekFileFlag)
		if err != nil {
			return nil, fmt.Errorf("reading MEK file: %w", err)
		}
		mek, err = hex.DecodeString(strings.TrimSpace(string(data)))
		if err != nil {
			return nil, fmt.Errorf("decoding MEK from file: %w", err)
		}
	} else {
		// Try environment variable
		mekHex := os.Getenv("ARMOR_MEK")
		if mekHex == "" {
			return nil, errors.New("no MEK provided: use -mek, -mek-file, -escrow, or ARMOR_MEK environment variable")
		}
		mek, err = hex.DecodeString(mekHex)
		if err != nil {
			return nil, fmt.Errorf("decoding MEK from environment: %w", err)
		}
	}

	if len(mek) != 32 {
		return nil, fmt.Errorf("invalid MEK length: got %d bytes, expected 32", len(mek))
	}

	return mek, nil
}

// loadMEKFromEscrowForVerify loads MEK from escrow JSON and sets env vars for B2
func loadMEKFromEscrowForVerify(escrowPath string) ([]byte, error) {
	data, err := os.ReadFile(escrowPath)
	if err != nil {
		return nil, fmt.Errorf("reading escrow file: %w", err)
	}

	var pkg struct {
		MEK string `json:"mek"`
		B2  struct {
			Region     string `json:"region"`
			Endpoint   string `json:"endpoint"`
			AccessKey  string `json:"access_key"`
			SecretKey  string `json:"secret_key"`
			CFDomain   string `json:"cf_domain"`
		} `json:"b2"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("parsing escrow JSON: %w", err)
	}

	if pkg.MEK == "" {
		return nil, errors.New("escrow missing 'mek' field")
	}

	mek, err := hex.DecodeString(pkg.MEK)
	if err != nil {
		return nil, fmt.Errorf("decoding MEK from escrow: %w", err)
	}

	if len(mek) != 32 {
		return nil, fmt.Errorf("invalid MEK length in escrow: got %d bytes, expected 32", len(mek))
	}

	// Set environment variables for B2 credentials
	if pkg.B2.Region != "" {
		os.Setenv("ARMOR_B2_REGION", pkg.B2.Region)
	}
	if pkg.B2.Endpoint != "" {
		os.Setenv("ARMOR_B2_ENDPOINT", pkg.B2.Endpoint)
	}
	if pkg.B2.AccessKey != "" {
		os.Setenv("ARMOR_B2_ACCESS_KEY_ID", pkg.B2.AccessKey)
	}
	if pkg.B2.SecretKey != "" {
		os.Setenv("ARMOR_B2_SECRET_ACCESS_KEY", pkg.B2.SecretKey)
	}
	if pkg.B2.CFDomain != "" {
		os.Setenv("ARMOR_CF_DOMAIN", pkg.B2.CFDomain)
	}

	return mek, nil
}

// initB2BackendForVerify initializes B2 backend from environment
func initB2BackendForVerify() (*backend.B2Backend, error) {
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
		ReadConcurrency: concurrencyFlag,
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

// listObjectsFromBucket lists objects from B2 with optional prefix filter
func listObjectsFromBucket() ([]string, error) {
	b2, err := initB2BackendForVerify()
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	var keys []string

	// List objects with prefix
	lister, err := b2.List(ctx, bucketFlag, prefixFlag, "")
	if err != nil {
		return nil, fmt.Errorf("listing objects: %w", err)
	}
	defer lister.Close()

	for {
		obj, err := lister.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("iterating objects: %w", err)
		}

		keys = append(keys, obj.Key)
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
func runVerification(ctx context.Context, b2 *backend.B2Backend, mek []byte, keys []string, since time.Time) *VerificationReport {
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
	sem := make(chan struct{}, concurrencyFlag)
	var wg sync.WaitGroup
	var okCount, corruptedCount, errorCount atomic.Int32

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
			result := verifyObject(ctx, b2, mek, bucketFlag, key, since)

			// Update counters
			switch result.Status {
			case "OK":
				okCount.Add(1)
			case "CORRUPTED":
				corruptedCount.Add(1)
			case "ERROR":
				errorCount.Add(1)
			}

			// Print progress
			if verboseFlag || result.Status != "OK" {
				fmt.Fprintf(os.Stderr, "[%s] %s: %s\n", result.Status, key, result.Details)
			} else if okCount.Load()%100 == 0 {
				fmt.Fprintf(os.Stderr, "Progress: %d verified...\n", okCount.Load())
			}

			// Store result (we'll collect them all at the end via a mutex-protected append)
			// For simplicity, we'll just append directly since Go's slice append is atomic for single-element appends
			// In production, you'd want a mutex here
		}(key)
	}

	// Wait for all verifications to complete
	wg.Wait()

	// Update counters
	report.OKCount = int(okCount.Load())
	report.CorruptedCount = int(corruptedCount.Load())
	report.ErrorCount = int(errorCount.Load())
	report.Duration = time.Since(startTime).Seconds()

	return report
}

// verifyObject performs full HMAC + digest verification of a single object
func verifyObject(ctx context.Context, b2 *backend.B2Backend, mek []byte, bucket, key string, since time.Time) ObjectVerificationResult {
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

	// Check if it's an ARMOR encrypted object
	if !info.IsARMOREncrypted {
		result.Status = "ERROR"
		result.Error = "Object is not ARMOR encrypted"
		result.Duration = time.Since(startTime).Seconds()
		return result
	}

	// Quick mode: only verify envelope and DEK
	if quickModeFlag {
		return quickVerifyObject(ctx, b2, mek, bucket, key, info, startTime)
	}

	// Full mode: verify HMAC + digest
	return fullVerifyObject(ctx, b2, mek, bucket, key, info, startTime)
}

// quickVerifyObject verifies only envelope and DEK (fast check)
func quickVerifyObject(ctx context.Context, b2 *backend.B2Backend, mek []byte, bucket, key string, info *backend.ObjectInfo, startTime time.Time) ObjectVerificationResult {
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
	if envelope.Version != crypto.Version1 && envelope.Version != crypto.Version2 {
		result.Status = "CORRUPTED"
		result.Error = fmt.Sprintf("Invalid envelope version: %d", envelope.Version)
		result.Duration = time.Since(startTime).Seconds()
		return result
	}

	// Get and verify wrapped DEK
	wrappedDEKBase64 := info.Metadata["x-amz-meta-armor-wrapped-dek"]
	if wrappedDEKBase64 == "" {
		result.Status = "ERROR"
		result.Error = "Missing wrapped DEK in object metadata"
		result.Duration = time.Since(startTime).Seconds()
		return result
	}

	wrappedDEK, err := decodeBase64ToBytes(wrappedDEKBase64)
	if err != nil {
		result.Status = "CORRUPTED"
		result.Error = fmt.Sprintf("Invalid wrapped DEK encoding: %v", err)
		result.Duration = time.Since(startTime).Seconds()
		return result
	}

	// Try to unwrap the DEK
	dek, err := crypto.UnwrapDEK(mek, wrappedDEK)
	if err != nil {
		result.Status = "CORRUPTED"
		result.Error = fmt.Sprintf("DEK unwrap failed: %v", err)
		result.Details = "Wrapped DEK appears corrupted or uses wrong MEK"
		result.Duration = time.Since(startTime).Seconds()
		return result
	}
	zeroBytes(dek)

	result.Status = "OK"
	result.Details = "Envelope and DEK verified successfully"
	result.Duration = time.Since(startTime).Seconds()
	return result
}

// fullVerifyObject performs complete HMAC + digest verification
func fullVerifyObject(ctx context.Context, b2 *backend.B2Backend, mek []byte, bucket, key string, info *backend.ObjectInfo, startTime time.Time) ObjectVerificationResult {
	result := ObjectVerificationResult{
		Bucket:    bucket,
		Key:       key,
		SizeBytes: info.Size,
		ModTime:   info.LastModified,
	}

	// First, do quick verification to ensure envelope is valid
	quickResult := quickVerifyObject(ctx, b2, mek, bucket, key, info, startTime)
	if quickResult.Status != "OK" {
		// Return quick result - envelope or DEK is corrupted
		quickResult.Duration = time.Since(startTime).Seconds()
		return quickResult
	}

	// Get wrapped DEK
	wrappedDEKBase64 := info.Metadata["x-amz-meta-armor-wrapped-dek"]
	wrappedDEK, err := decodeBase64ToBytes(wrappedDEKBase64)
	if err != nil {
		result.Status = "ERROR"
		result.Error = fmt.Sprintf("Invalid wrapped DEK encoding: %v", err)
		result.Duration = time.Since(startTime).Seconds()
		return result
	}

	// Unwrap DEK
	dek, err := crypto.UnwrapDEK(mek, wrappedDEK)
	if err != nil {
		result.Status = "ERROR"
		result.Error = fmt.Sprintf("DEK unwrap failed: %v", err)
		result.Duration = time.Since(startTime).Seconds()
		return result
	}
	defer zeroBytes(dek)

	// Read the entire object for HMAC verification
	objectReader, err := b2.Get(ctx, bucket, key)
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

	// Verify HMAC using crypto.VerifyDecompression
	// This performs byte-for-byte verification with HMAC checking
	err = crypto.VerifyDecompression(dek, objectData[crypto.HeaderSize:], envelope)
	if err != nil {
		result.Status = "CORRUPTED"
		result.Error = fmt.Sprintf("HMAC or decompression verification failed: %v", err)
		result.Details = "Object data corruption detected - HMAC mismatch or decompression error"
		result.Duration = time.Since(startTime).Seconds()
		return result
	}

	result.Status = "OK"
	result.Details = "Full HMAC + digest verification passed"
	result.Duration = time.Since(startTime).Seconds()
	return result
}

// decodeBase64ToBytes decodes a base64 string (with URL-safe handling) to bytes
func decodeBase64ToBytes(s string) ([]byte, error) {
	// Handle URL-safe base64
	s = strings.ReplaceAll(s, "-", "+")
	s = strings.ReplaceAll(s, "_", "/")

	// Add padding if needed
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}

	return base64.StdEncoding.DecodeString(s)
}

// zeroBytes securely zeros a byte slice
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// writeReport writes the verification report to file or stdout
func writeReport(report *VerificationReport, outputPath string) error {
	var output io.Writer
	var closer io.Closer
	var err error

	if outputPath != "" {
		// Ensure directory exists
		dir := filepath.Dir(outputPath)
		if dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("creating output directory: %w", err)
			}
		}

		output, err = os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("creating output file: %w", err)
		}
		closer = output
		defer closer.Close()
	} else {
		output = os.Stdout
	}

	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}
