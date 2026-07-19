// Package canary implements a self-healing integrity monitor for ARMOR.
// It verifies the entire encryption/decryption pipeline by uploading a known-content
// canary file, downloading it through Cloudflare, and verifying the decryption.
package canary

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
	"github.com/jedarden/armor/internal/metrics"
	"golang.org/x/sync/errgroup"
)

// Status represents the health status of the canary.
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusUnhealthy Status = "unhealthy"
	StatusUnknown   Status = "unknown"
)

// CanaryState holds the current state of the canary monitor.
type CanaryState struct {
	mu sync.RWMutex

	Status              Status    `json:"status"`
	LastCheck           time.Time `json:"last_check"`
	LastSuccess         time.Time `json:"last_success"`
	ConsecutiveSuccess  int       `json:"consecutive_success"`
	ConsecutiveFailures int       `json:"consecutive_failures"`
	LastError           string    `json:"last_error,omitempty"`

	// Metrics from last check
	UploadLatencyMs   int64 `json:"upload_latency_ms"`
	DownloadLatencyMs int64 `json:"download_latency_ms"`
	DecryptVerified   bool  `json:"decrypt_verified"`
	HMACVerified      bool  `json:"hmac_verified"`
	CFCacheHit        bool  `json:"cloudflare_cache_hit"`

	// Multipart canary state
	MultipartHealthy          Status    `json:"multipart_healthy"`
	MultipartLastCheck        time.Time `json:"multipart_last_check"`
	MultipartLastSuccess      time.Time `json:"multipart_last_success"`
	MultipartConsecutiveFails int       `json:"multipart_consecutive_fails"`
	MultipartLastError        string    `json:"multipart_last_error,omitempty"`

	// Instance identification
	InstanceID string `json:"instance_id"`
}

// Result represents the result of a canary check.
type Result struct {
	Status            Status    `json:"status"`
	LastCheck         time.Time `json:"last_check"`
	UploadLatencyMs   int64     `json:"upload_latency_ms"`
	DownloadLatencyMs int64     `json:"download_latency_ms"`
	DecryptVerified   bool      `json:"decrypt_verified"`
	HMACVerified      bool      `json:"hmac_verified"`
	CFCacheHit        bool      `json:"cloudflare_cache_hit"`
	LastError         string    `json:"last_error,omitempty"`

	// Multipart canary result
	MultipartHealthy           Status    `json:"multipart_healthy_status"`
	MultipartHealthyBool       bool      `json:"multipart_healthy"`
	MultipartLastCheck         time.Time `json:"multipart_last_check"`
	MultipartConsecutiveFails  int       `json:"multipart_consecutive_fails"`
	MultipartLastError         string    `json:"multipart_last_error,omitempty"`
}

// Monitor manages the canary integrity checks.
type Monitor struct {
	backend    backend.Backend
	bucket     string
	mek        []byte
	blockSize  int
	instanceID string

	state CanaryState

	// Configuration
	interval          time.Duration
	canarySize        int
	maxRetries        int
	retryDelay        time.Duration
	multipartInterval time.Duration
	multipartSize     int

	// Control
	stopCh chan struct{}
	doneCh chan struct{}
}

// Config holds configuration for the canary monitor.
type Config struct {
	Backend          backend.Backend
	Bucket           string
	MEK              []byte
	BlockSize        int
	InstanceID       string
	Interval         time.Duration // Check interval (default 5 minutes)
	CanarySize       int           // Size of canary content (default 1024 bytes)
	MaxRetries       int           // Max retries on failure (default 3)
	RetryDelay       time.Duration // Delay between retries (default 10s)
	MultipartInterval time.Duration // Multipart check interval (default 1 hour)
	MultipartSize     int           // Size of multipart canary (default 6MB)
}

// NewMonitor creates a new canary monitor.
func NewMonitor(cfg Config) *Monitor {
	if cfg.Interval == 0 {
		cfg.Interval = 5 * time.Minute
	}
	if cfg.CanarySize == 0 {
		cfg.CanarySize = 1024
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}
	if cfg.RetryDelay == 0 {
		cfg.RetryDelay = 10 * time.Second
	}
	if cfg.MultipartInterval == 0 {
		cfg.MultipartInterval = 1 * time.Hour
	}
	if cfg.MultipartSize == 0 {
		// Two canary parts so the multipart path is exercised with at least one
		// non-final part subject to B2's 5 MiB minimum. For the default 64 KiB
		// block this is 2 * 5.25 MiB = 10.5 MiB.
		cfg.MultipartSize = 2 * canaryMultipartPartSize(crypto.DefaultBlockSize)
	}

	instanceID := cfg.InstanceID
	if instanceID == "" {
		instanceID, _ = os.Hostname()
		if instanceID == "" {
			// Generate random ID
			b := make([]byte, 8)
			rand.Read(b)
			instanceID = hex.EncodeToString(b)
		}
	}

	return &Monitor{
		backend:           cfg.Backend,
		bucket:            cfg.Bucket,
		mek:               cfg.MEK,
		blockSize:         cfg.BlockSize,
		instanceID:        instanceID,
		interval:          cfg.Interval,
		canarySize:        cfg.CanarySize,
		maxRetries:        cfg.MaxRetries,
		retryDelay:        cfg.RetryDelay,
		multipartInterval: cfg.MultipartInterval,
		multipartSize:     cfg.MultipartSize,
		stopCh:            make(chan struct{}),
		doneCh:            make(chan struct{}),
		state: CanaryState{
			Status:           StatusUnknown,
			MultipartHealthy: StatusUnknown,
			InstanceID:       instanceID,
		},
	}
}

// Start begins the periodic canary checks.
// It runs an initial check immediately, then periodically.
func (m *Monitor) Start(ctx context.Context) {
	go func() {
		defer close(m.doneCh)

		// Initial checks
		m.runCheck(ctx)
		m.runMultipartCheck(ctx)

		ticker := time.NewTicker(m.interval)
		multipartTicker := time.NewTicker(m.multipartInterval)
		defer ticker.Stop()
		defer multipartTicker.Stop()

		for {
			select {
			case <-m.stopCh:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.runCheck(ctx)
			case <-multipartTicker.C:
				m.runMultipartCheck(ctx)
			}
		}
	}()
}

// Stop stops the canary monitor.
func (m *Monitor) Stop() {
	close(m.stopCh)
	<-m.doneCh
}

// runCheck performs a single canary check with retries.
func (m *Monitor) runCheck(ctx context.Context) {
	var lastErr error

	metrics.DefaultMetrics.IncCanaryChecks()
	metrics.DefaultMetrics.SetCanaryLastCheck(time.Now())

	for attempt := 0; attempt < m.maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(m.retryDelay):
			case <-ctx.Done():
				return
			}
		}

		result, err := m.check(ctx)
		if err == nil {
			m.updateStateSuccess(result)
			metrics.DefaultMetrics.SetCanaryLastError("")
			return
		}
		lastErr = err
	}

	metrics.DefaultMetrics.IncCanaryFailures()
	metrics.DefaultMetrics.SetCanaryLastError(lastErr.Error())
	m.updateStateFailure(lastErr)
}

// runMultipartCheck performs a single multipart canary check with retries.
func (m *Monitor) runMultipartCheck(ctx context.Context) {
	var lastErr error
	var lastAttemptDuration time.Duration
	var lastAttemptOp string

	metrics.DefaultMetrics.IncMultipartCanaryChecks()
	metrics.DefaultMetrics.SetMultipartCanaryLastCheck(time.Now())

	for attempt := 0; attempt < m.maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(m.retryDelay):
			case <-ctx.Done():
				return
			}
		}

		checkStart := time.Now()
		result, err := m.checkMultipart(ctx)
		lastAttemptDuration = time.Since(checkStart)

		if err == nil {
			m.updateMultipartStateSuccess(result)
			metrics.DefaultMetrics.SetMultipartCanaryLastError("")
			metrics.DefaultMetrics.SetMultipartCanaryHealthy(true)
			return
		}
		lastErr = err

		// Determine which operation failed based on error message
		// This is best-effort since the error comes from deep in the call stack
		errStr := err.Error()
		if strings.Contains(errStr, "upload") || strings.Contains(errStr, "part") {
			lastAttemptOp = "upload"
		} else if strings.Contains(errStr, "download") || strings.Contains(errStr, "decrypt") || strings.Contains(errStr, "verify") {
			lastAttemptOp = "verify"
		} else {
			// Default to upload if we can't determine
			lastAttemptOp = "upload"
		}
	}

	// Record failure metric for the last attempt
	if lastAttemptOp != "" && lastAttemptDuration > 0 {
		metrics.DefaultMetrics.RecordMultipartUpload(lastAttemptOp, "failure", lastAttemptDuration)
	}

	metrics.DefaultMetrics.IncMultipartCanaryFailures()
	metrics.DefaultMetrics.SetMultipartCanaryLastError(lastErr.Error())
	metrics.DefaultMetrics.SetMultipartCanaryHealthy(false)
	m.updateMultipartStateFailure(lastErr)
}

// check performs a single canary integrity check.
func (m *Monitor) check(ctx context.Context) (*Result, error) {
	result := &Result{
		LastCheck: time.Now(),
	}

	// Generate unique canary content with timestamp
	timestamp := time.Now().UnixNano()
	canaryContent := make([]byte, m.canarySize)
	if _, err := rand.Read(canaryContent); err != nil {
		return nil, fmt.Errorf("failed to generate canary content: %w", err)
	}

	// Embed timestamp for verification
	binary.LittleEndian.PutUint64(canaryContent[:8], uint64(timestamp))

	// Generate unique key for this canary
	key := fmt.Sprintf(".armor/canary/%s/%d", m.instanceID, timestamp)

	// Generate DEK and IV
	dek, err := crypto.GenerateDEK()
	if err != nil {
		return nil, fmt.Errorf("failed to generate DEK: %w", err)
	}

	iv, err := crypto.GenerateIV()
	if err != nil {
		return nil, fmt.Errorf("failed to generate IV: %w", err)
	}

	// Wrap DEK with MEK
	wrappedDEK, err := crypto.WrapDEK(m.mek, dek)
	if err != nil {
		return nil, fmt.Errorf("failed to wrap DEK: %w", err)
	}

	// Compute plaintext SHA-256
	plaintextSHA := crypto.ComputePlaintextSHA256(canaryContent)

	// Create envelope header
	header, err := crypto.NewEnvelopeHeader(iv, int64(len(canaryContent)), m.blockSize, plaintextSHA)
	if err != nil {
		return nil, fmt.Errorf("failed to create header: %w", err)
	}

	headerBytes, err := header.Encode()
	if err != nil {
		return nil, fmt.Errorf("failed to encode header: %w", err)
	}

	// Encrypt
	encryptor, err := crypto.NewEncryptor(dek, iv, m.blockSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create encryptor: %w", err)
	}

	encrypted, hmacTable, err := encryptor.Encrypt(canaryContent)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt: %w", err)
	}

	// Build envelope
	envelope := make([]byte, 0, len(headerBytes)+len(encrypted)+len(hmacTable))
	envelope = append(envelope, headerBytes...)
	envelope = append(envelope, encrypted...)
	envelope = append(envelope, hmacTable...)

	// Compute ETag
	etag := backend.ComputeETag(canaryContent)

	// Build metadata
	meta := (&backend.ARMORMetadata{
		Version:       1,
		BlockSize:     m.blockSize,
		PlaintextSize: int64(len(canaryContent)),
		ContentType:   "application/octet-stream",
		IV:            iv,
		WrappedDEK:    wrappedDEK,
		PlaintextSHA:  hex.EncodeToString(plaintextSHA[:]),
		ETag:          etag,
	}).ToMetadata()

	// Upload to B2 (direct, not through Cloudflare)
	uploadStart := time.Now()
	if err := m.backend.Put(ctx, m.bucket, key, bytes.NewReader(envelope), int64(len(envelope)), meta); err != nil {
		return nil, fmt.Errorf("failed to upload canary: %w", err)
	}
	result.UploadLatencyMs = time.Since(uploadStart).Milliseconds()

	// Download through Cloudflare (via GetRangeWithHeaders to capture CF-Cache-Status)
	downloadStart := time.Now()
	body, headers, err := m.backend.GetRangeWithHeaders(ctx, m.bucket, key, 0, int64(len(envelope)))
	if err != nil {
		return nil, fmt.Errorf("failed to download canary: %w", err)
	}
	downloadedEnvelope, err := io.ReadAll(body)
	body.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to read downloaded canary: %w", err)
	}
	result.DownloadLatencyMs = time.Since(downloadStart).Milliseconds()

	// Check Cloudflare cache status
	// CF-Cache-Status values: HIT, MISS, EXPIRED, STALE, BYPASS, REVALIDATED, UPDATING, IGNORED
	if cfStatus, ok := headers["CF-Cache-Status"]; ok {
		result.CFCacheHit = (cfStatus == "HIT" || cfStatus == "STALE" || cfStatus == "REVALIDATED")
	}

	// Parse header
	downloadedHeader, err := crypto.DecodeHeader(downloadedEnvelope)
	if err != nil {
		return nil, fmt.Errorf("failed to decode header: %w", err)
	}

	// Extract encrypted blocks and HMAC table
	dataStart := int64(crypto.HeaderSize)
	dataEnd := dataStart + int64(len(canaryContent))
	hmacStart := downloadedHeader.HMACTableOffset()
	hmacEnd := hmacStart + int64(downloadedHeader.BlockCount())*crypto.HMACSize

	if hmacEnd > int64(len(downloadedEnvelope)) {
		return nil, fmt.Errorf("downloaded envelope too short for HMAC table")
	}

	downloadedEncrypted := downloadedEnvelope[dataStart:dataEnd]
	downloadedHMAC := downloadedEnvelope[hmacStart:hmacEnd]

	// Unwrap DEK
	unwrappedDEK, err := crypto.UnwrapDEK(m.mek, wrappedDEK)
	if err != nil {
		return nil, fmt.Errorf("failed to unwrap DEK: %w", err)
	}

	// Create decryptor
	decryptor, err := crypto.NewDecryptor(unwrappedDEK, iv, m.blockSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create decryptor: %w", err)
	}

	// Verify HMACs
	if err := decryptor.VerifyHMACs(downloadedEncrypted, downloadedHMAC); err != nil {
		result.HMACVerified = false
		return nil, fmt.Errorf("HMAC verification failed: %w", err)
	}
	result.HMACVerified = true

	// Decrypt
	decrypted, err := decryptor.Decrypt(downloadedEncrypted, downloadedHMAC)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	// Verify decrypted content matches original
	if !bytes.Equal(decrypted, canaryContent) {
		result.DecryptVerified = false
		return nil, fmt.Errorf("decrypted content does not match original")
	}
	result.DecryptVerified = true

	// Verify plaintext SHA
	if err := downloadedHeader.VerifyPlaintextSHA(decrypted); err != nil {
		return nil, fmt.Errorf("plaintext SHA verification failed: %w", err)
	}

	// Clean up canary (best effort)
	go func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		m.backend.Delete(cleanupCtx, m.bucket, key)
	}()

	result.Status = StatusHealthy

	return result, nil
}

// b2MinPartSize is the minimum size B2 (via its S3-compatible API) accepts for
// any non-final multipart part. Uploading a smaller non-final part makes
// CompleteMultipartUpload fail with HTTP 400 EntityTooSmall ("Your proposed
// upload is smaller than the minimum allowed size"). The final part has no
// minimum.
const b2MinPartSize = 5 * 1024 * 1024 // 5 MiB

// canaryMultipartPartSize returns the size to use for each non-final part of the
// multipart canary, chosen to satisfy two constraints:
//   - at least b2MinPartSize (5 MiB), so B2 does not reject the non-final parts,
//     and
//   - a multiple of the encryption block size, so a part boundary never splits
//     an encryption block (ARMOR's CTR counter derivation requires block-aligned
//     parts).
//
// We target 5.25 MiB and round up to a block multiple; for the default 64 KiB
// block this is exactly 84 * 64 KiB = 5.25 MiB, comfortably above the 5 MiB
// floor.
func canaryMultipartPartSize(blockSize int) int {
	if blockSize <= 0 {
		blockSize = crypto.DefaultBlockSize
	}
	const targetPartSize = 5505024 // 5.25 MiB
	partSize := ((targetPartSize + blockSize - 1) / blockSize) * blockSize
	if partSize < b2MinPartSize {
		// Defensive: never round below the B2 minimum.
		partSize = ((b2MinPartSize + blockSize - 1) / blockSize) * blockSize
	}
	return partSize
}

// checkMultipart performs a multipart canary check to exercise the multipart upload path.
func (m *Monitor) checkMultipart(ctx context.Context) (*Result, error) {
	result := &Result{
		LastCheck: time.Now(),
	}

	// Generate unique canary content with timestamp
	timestamp := time.Now().UnixNano()
	canaryContent := make([]byte, m.multipartSize)
	if _, err := rand.Read(canaryContent); err != nil {
		return nil, fmt.Errorf("failed to generate multipart canary content: %w", err)
	}

	// Embed timestamp for verification
	binary.LittleEndian.PutUint64(canaryContent[:8], uint64(timestamp))

	// Generate unique key for this canary
	key := fmt.Sprintf(".armor/canary-multipart/%s/%d", m.instanceID, timestamp)

	// Generate DEK and IV
	dek, err := crypto.GenerateDEK()
	if err != nil {
		return nil, fmt.Errorf("failed to generate DEK: %w", err)
	}

	iv, err := crypto.GenerateIV()
	if err != nil {
		return nil, fmt.Errorf("failed to generate IV: %w", err)
	}

	// Wrap DEK with MEK
	wrappedDEK, err := crypto.WrapDEK(m.mek, dek)
	if err != nil {
		return nil, fmt.Errorf("failed to wrap DEK: %w", err)
	}

	// Compute plaintext SHA-256
	plaintextSHA := crypto.ComputePlaintextSHA256(canaryContent)

	// Create envelope header
	header, err := crypto.NewEnvelopeHeader(iv, int64(len(canaryContent)), m.blockSize, plaintextSHA)
	if err != nil {
		return nil, fmt.Errorf("failed to create header: %w", err)
	}

	headerBytes, err := header.Encode()
	if err != nil {
		return nil, fmt.Errorf("failed to encode header: %w", err)
	}

	// Encrypt
	encryptor, err := crypto.NewEncryptor(dek, iv, m.blockSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create encryptor: %w", err)
	}

	encrypted, hmacTable, err := encryptor.Encrypt(canaryContent)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt: %w", err)
	}

	// Build envelope
	envelope := make([]byte, 0, len(headerBytes)+len(encrypted)+len(hmacTable))
	envelope = append(envelope, headerBytes...)
	envelope = append(envelope, encrypted...)
	envelope = append(envelope, hmacTable...)

	// Compute ETag
	etag := backend.ComputeETag(canaryContent)

	// Build metadata
	meta := (&backend.ARMORMetadata{
		Version:       1,
		BlockSize:     m.blockSize,
		PlaintextSize: int64(len(canaryContent)),
		ContentType:   "application/octet-stream",
		IV:            iv,
		WrappedDEK:    wrappedDEK,
		PlaintextSHA:  hex.EncodeToString(plaintextSHA[:]),
		ETag:          etag,
	}).ToMetadata()

	// Upload via multipart API (exercises create/upload/complete path)
	uploadStart := time.Now()

	// Step 1: Create multipart upload
	uploadID, err := m.backend.CreateMultipartUpload(ctx, m.bucket, key, meta)
	if err != nil {
		return nil, fmt.Errorf("failed to create multipart upload: %w", err)
	}

	// Step 2: Upload parts CONCURRENTLY (ADR-005 + its 2026-07-19 amendment make
	// out-of-order / concurrent part uploads a supported, first-class path). The
	// canary must exercise that path continuously — not only the sequential one —
	// so a regression that re-introduces ordering assumptions fails the canary
	// loudly. Each part is a stateless per-part S3 call addressed by part number,
	// so uploads are independent; goroutines complete in nondeterministic
	// ("shuffled") order and the ETags are reassembled in ascending part-number
	// order below, as CompleteMultipartUpload requires.
	//
	// B2 still rejects CompleteMultipartUpload with EntityTooSmall when any
	// non-final part is smaller than 5 MiB, so each part except the last is at
	// least b2MinPartSize and a multiple of the block size so a part boundary
	// never splits an encryption block. For the default 64 KiB block this is
	// 5.25 MiB (84 blocks).
	partSize := canaryMultipartPartSize(m.blockSize)

	// Pre-compute the disjoint part slices so each goroutine reads its own range
	// of the envelope with no shared cursor. (partData aliases envelope, which is
	// never mutated after this point, so concurrent reads are safe.)
	type canaryPart struct {
		num  int32
		data []byte
	}
	var canaryParts []canaryPart
	partNum := int32(1)
	for offset := 0; offset < len(envelope); offset += partSize {
		end := offset + partSize
		if end > len(envelope) {
			end = len(envelope)
		}
		canaryParts = append(canaryParts, canaryPart{
			num:  partNum,
			data: envelope[offset:end],
		})
		partNum++
	}

	// ETags indexed by part number (position num-1); each goroutine writes a
	// distinct slot, so no synchronization is needed on the slice itself.
	partETags := make([]string, len(canaryParts))
	g, gctx := errgroup.WithContext(ctx)
	for _, cp := range canaryParts {
		cp := cp // captured per-goroutine
		g.Go(func() error {
			etag, err := m.backend.UploadPart(gctx, m.bucket, key, uploadID, cp.num, bytes.NewReader(cp.data), int64(len(cp.data)))
			if err != nil {
				return fmt.Errorf("failed to upload part %d: %w", cp.num, err)
			}
			partETags[cp.num-1] = etag
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		// Abort on failure (first error wins; errgroup cancels the rest)
		m.backend.AbortMultipartUpload(ctx, m.bucket, key, uploadID)
		return nil, err
	}

	// Reassemble in ascending part-number order: the uploads above completed in
	// shuffled order, but S3/B2 require CompleteMultipartUpload parts sorted by
	// part number.
	parts := make([]backend.CompletedPart, len(canaryParts))
	for i := range canaryParts {
		parts[i] = backend.CompletedPart{
			PartNumber: int32(i + 1),
			ETag:       partETags[i],
		}
	}

	// Step 3: Complete multipart upload
	_, err = m.backend.CompleteMultipartUpload(ctx, m.bucket, key, uploadID, parts)
	if err != nil {
		// Abort on failure
		m.backend.AbortMultipartUpload(ctx, m.bucket, key, uploadID)
		return nil, fmt.Errorf("failed to complete multipart upload: %w", err)
	}

	result.UploadLatencyMs = time.Since(uploadStart).Milliseconds()

	// Record successful upload duration metric
	metrics.DefaultMetrics.RecordMultipartUpload("upload", "success", time.Since(uploadStart))

	// Download and verify (same verification as regular canary)
	downloadStart := time.Now()
	body, headers, err := m.backend.GetRangeWithHeaders(ctx, m.bucket, key, 0, int64(len(envelope)))
	if err != nil {
		return nil, fmt.Errorf("failed to download multipart canary: %w", err)
	}
	downloadedEnvelope, err := io.ReadAll(body)
	body.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to read downloaded multipart canary: %w", err)
	}
	result.DownloadLatencyMs = time.Since(downloadStart).Milliseconds()

	// Record successful verify duration metric
	metrics.DefaultMetrics.RecordMultipartUpload("verify", "success", time.Since(downloadStart))

	// Check Cloudflare cache status
	if cfStatus, ok := headers["CF-Cache-Status"]; ok {
		result.CFCacheHit = (cfStatus == "HIT" || cfStatus == "STALE" || cfStatus == "REVALIDATED")
	}

	// Verify size matches
	if len(downloadedEnvelope) != len(envelope) {
		return nil, fmt.Errorf("multipart download size mismatch: got %d, expected %d", len(downloadedEnvelope), len(envelope))
	}

	// Byte-for-byte verification (critical for multipart integrity)
	if !bytes.Equal(downloadedEnvelope, envelope) {
		return nil, fmt.Errorf("multipart content byte-for-byte verification failed")
	}

	// Parse header
	downloadedHeader, err := crypto.DecodeHeader(downloadedEnvelope)
	if err != nil {
		return nil, fmt.Errorf("failed to decode header: %w", err)
	}

	// Extract encrypted blocks and HMAC table
	dataStart := int64(crypto.HeaderSize)
	dataEnd := dataStart + int64(len(canaryContent))
	hmacStart := downloadedHeader.HMACTableOffset()
	hmacEnd := hmacStart + int64(downloadedHeader.BlockCount())*crypto.HMACSize

	if hmacEnd > int64(len(downloadedEnvelope)) {
		return nil, fmt.Errorf("downloaded envelope too short for HMAC table")
	}

	downloadedEncrypted := downloadedEnvelope[dataStart:dataEnd]
	downloadedHMAC := downloadedEnvelope[hmacStart:hmacEnd]

	// Unwrap DEK
	unwrappedDEK, err := crypto.UnwrapDEK(m.mek, wrappedDEK)
	if err != nil {
		return nil, fmt.Errorf("failed to unwrap DEK: %w", err)
	}

	// Create decryptor
	decryptor, err := crypto.NewDecryptor(unwrappedDEK, iv, m.blockSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create decryptor: %w", err)
	}

	// Verify HMACs
	if err := decryptor.VerifyHMACs(downloadedEncrypted, downloadedHMAC); err != nil {
		result.HMACVerified = false
		return nil, fmt.Errorf("HMAC verification failed: %w", err)
	}
	result.HMACVerified = true

	// Decrypt
	decrypted, err := decryptor.Decrypt(downloadedEncrypted, downloadedHMAC)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	// Verify decrypted content matches original
	if !bytes.Equal(decrypted, canaryContent) {
		result.DecryptVerified = false
		return nil, fmt.Errorf("decrypted content does not match original")
	}
	result.DecryptVerified = true

	// Verify plaintext SHA
	if err := downloadedHeader.VerifyPlaintextSHA(decrypted); err != nil {
		return nil, fmt.Errorf("plaintext SHA verification failed: %w", err)
	}

	// Clean up canary (best effort)
	go func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		m.backend.Delete(cleanupCtx, m.bucket, key)
	}()

	result.Status = StatusHealthy
	result.MultipartHealthy = StatusHealthy
	result.MultipartHealthyBool = true

	return result, nil
}

// updateStateSuccess updates state after a successful check.
func (m *Monitor) updateStateSuccess(result *Result) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()

	m.state.Status = StatusHealthy
	m.state.LastCheck = result.LastCheck
	m.state.LastSuccess = result.LastCheck
	m.state.ConsecutiveSuccess++
	m.state.ConsecutiveFailures = 0
	m.state.LastError = ""
	m.state.UploadLatencyMs = result.UploadLatencyMs
	m.state.DownloadLatencyMs = result.DownloadLatencyMs
	m.state.DecryptVerified = result.DecryptVerified
	m.state.HMACVerified = result.HMACVerified
	m.state.CFCacheHit = result.CFCacheHit
}

// updateStateFailure updates state after a failed check.
func (m *Monitor) updateStateFailure(err error) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()

	m.state.Status = StatusUnhealthy
	m.state.LastCheck = time.Now()
	m.state.ConsecutiveSuccess = 0
	m.state.ConsecutiveFailures++
	m.state.LastError = err.Error()
}

// updateMultipartStateSuccess updates multipart state after a successful check.
func (m *Monitor) updateMultipartStateSuccess(result *Result) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()

	m.state.MultipartHealthy = StatusHealthy
	m.state.MultipartLastCheck = result.LastCheck
	m.state.MultipartLastSuccess = result.LastCheck
	m.state.MultipartConsecutiveFails = 0
	m.state.MultipartLastError = ""
}

// updateMultipartStateFailure updates multipart state after a failed check.
func (m *Monitor) updateMultipartStateFailure(err error) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()

	m.state.MultipartHealthy = StatusUnhealthy
	m.state.MultipartLastCheck = time.Now()
	m.state.MultipartConsecutiveFails++
	m.state.MultipartLastError = err.Error()
}

// GetStatus returns the current canary status.
func (m *Monitor) GetStatus() Result {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()

	return Result{
		Status:                     m.state.Status,
		LastCheck:                  m.state.LastCheck,
		UploadLatencyMs:            m.state.UploadLatencyMs,
		DownloadLatencyMs:          m.state.DownloadLatencyMs,
		DecryptVerified:            m.state.DecryptVerified,
		HMACVerified:               m.state.HMACVerified,
		CFCacheHit:                 m.state.CFCacheHit,
		LastError:                  m.state.LastError,
		MultipartHealthy:           m.state.MultipartHealthy,
		MultipartHealthyBool:       m.state.MultipartHealthy == StatusHealthy,
		MultipartLastCheck:         m.state.MultipartLastCheck,
		MultipartConsecutiveFails:  m.state.MultipartConsecutiveFails,
		MultipartLastError:         m.state.MultipartLastError,
	}
}

// IsHealthy returns true if the canary is healthy.
func (m *Monitor) IsHealthy() bool {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()
	return m.state.Status == StatusHealthy
}

// MarshalJSON returns the state as JSON.
func (m *Monitor) MarshalJSON() ([]byte, error) {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()
	return json.Marshal(&m.state)
}
