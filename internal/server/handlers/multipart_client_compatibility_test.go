// The executable half of the multipart client-concurrency compatibility
// matrix (docs/multipart-client-compatibility.md).
//
// README promises "any S3-compatible client works without modification";
// ADR-003 §4's sequential-only enforcement once contradicted that promise,
// but it was superseded by ADR-015 (uniform-part-size contract, parts pinned
// from part 1, earlier arrivals deferred with retryable 503 SlowDown) and
// ADR-011 (non-uniform parts). The v3 write format — the ARMOR_FORMAT_VERSION
// default — imposes no part-order or part-size contract at all.
//
// These tests walk the three client shapes the matrix names against BOTH
// write formats and hold each to the property every client class depends on:
// complete the upload, then GET the object back byte-for-byte.
package handlers_test

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/keymanager"
	"github.com/jedarden/armor/internal/server/handlers"
)

// compatMatrixSetup is recordingTestSetup with an explicit write format:
// 3 is the ARMOR_FORMAT_VERSION default (no multipart contract), 2 is the
// legacy uniform-part-size contract (ADR-015 as amended).
func compatMatrixSetup(t *testing.T, formatVersion int) (*config.Config, *recordingBackend, *handlers.Handlers) {
	t.Helper()
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}
	cfg := &config.Config{
		BlockSize:          65536,
		AuthAccessKey:      "test-access-key",
		AuthSecretKey:      "REMOVED-NOT-A-SECRET-VALUE",
		FormatWriteVersion: formatVersion,
	}
	rb := newRecordingBackend()
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}
	h := handlers.New(cfg, rb, cache, footerCache, km, nil)
	return cfg, rb, h
}

// compatPart builds a part full of index-distinguishable bytes so a wrong
// part order or offset cannot alias another part's ciphertext.
func compatPart(t *testing.T, partNumber, size int) []byte {
	t.Helper()
	b := make([]byte, size)
	for i := range b {
		b[i] = byte(i%251) ^ byte((partNumber*7+i)>>9&0xFF)
	}
	return b
}

// compatVerifyRoundTrip GETs the completed object through the real handler
// path and requires it byte-for-byte equal to the plaintext the client sent.
func compatVerifyRoundTrip(t *testing.T, h *handlers.Handlers, bucket, key string, want []byte) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/"+bucket+"/"+key, nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET after complete failed: status %d: %s", w.Code, w.Body.String())
	}
	if !bytes.Equal(w.Body.Bytes(), want) {
		t.Fatalf("round-trip mismatch: got %d bytes, want %d; first divergence at %d",
			w.Body.Len(), len(want), firstDivergence(w.Body.Bytes(), want))
	}
}

// formatVersions walks {v3, v2}: the default format and the legacy contract.
var formatVersions = []int{3, 2}

// TestMultipartClientCompat_AWSCliDefaultConcurrent is the aws s3 cp
// default-concurrency shape (the ADR-015 amendment's acceptance case): all
// parts start at once and the short FINAL part — fewest bytes to transfer —
// completes first. On v3 that first arrival succeeds outright; on v2 it
// cannot have a CTR offset (part 1 has not pinned P) and must be deferred
// with retryable 503 SlowDown, then succeed transparently on the client's
// retry after part 1 lands. Either way the completed object must round-trip
// byte-for-byte.
func TestMultipartClientCompat_AWSCliDefaultConcurrent(t *testing.T) {
	for _, fv := range formatVersions {
		t.Run(fmt.Sprintf("format_v%d", fv), func(t *testing.T) {
			_, _, h := compatMatrixSetup(t, fv)
			bucket, key := "test-bucket", fmt.Sprintf("aws-cli-default-v%d.bin", fv)

			const fullParts = 6
			const fullSize = 5 * 1024 * 1024 // block-aligned, ≥ B2's 5 MiB minimum
			finalSize := 3*1024*1024 + 4567  // short final part (< P, unaligned)

			parts := make([][]byte, fullParts+1)
			var want []byte
			for p := 1; p <= fullParts+1; p++ {
				size := fullSize
				if p == fullParts+1 {
					size = finalSize
				}
				parts[p-1] = compatPart(t, p, size)
				want = append(want, parts[p-1]...)
			}

			uploadID := initiateMultipart(t, h, bucket, key)
			etags := make([]string, fullParts+1)

			// The short final part arrives and completes first.
			code, etag, body := uploadPartResponse(t, h, bucket, key, uploadID, fullParts+1, parts[fullParts])
			switch fv {
			case 3:
				if code != http.StatusOK {
					t.Fatalf("v3 imposes no part contract: short final part arriving first must be 200, got %d: %s", code, body)
				}
				etags[fullParts] = etag
			case 2:
				if code != http.StatusServiceUnavailable {
					t.Fatalf("v2 must defer a part>1 that arrives before part 1 with 503 SlowDown, got %d: %s", code, body)
				}
				if !bytes.Contains([]byte(body), []byte("SlowDown")) {
					t.Errorf("deferred part should return SlowDown, got: %s", body)
				}
			}

			// Part 1 lands (pinning P on v2); the client keeps uploading.
			etags[0] = uploadPart(t, h, bucket, key, uploadID, 1, parts[0])

			// The remaining full parts finish out of order, concurrently.
			var wg sync.WaitGroup
			errs := make(chan error, fullParts-1)
			for p := 2; p <= fullParts; p++ {
				wg.Add(1)
				go func(p int) {
					defer wg.Done()
					c, etag, body := uploadPartResponse(t, h, bucket, key, uploadID, p, parts[p-1])
					if c != http.StatusOK {
						errs <- fmt.Errorf("part %d: status %d: %s", p, c, body)
						return
					}
					etags[p-1] = etag
				}(p)
			}
			wg.Wait()
			close(errs)
			for err := range errs {
				t.Error(err)
			}

			// On v2 the deferred final part is retried transparently and now
			// succeeds at its correct offset. (On v3 it already succeeded.)
			if fv == 2 {
				c, etag, body := uploadPartResponse(t, h, bucket, key, uploadID, fullParts+1, parts[fullParts])
				if c != http.StatusOK {
					t.Fatalf("SlowDown-deferred final part must succeed on retry after part 1 pinned P, got %d: %s", c, body)
				}
				etags[fullParts] = etag
			}

			completeMultipart(t, h, bucket, key, uploadID, etags)
			compatVerifyRoundTrip(t, h, bucket, key, want)
		})
	}
}

// TestMultipartClientCompat_SDKTransferManager is the SDK transfer-manager
// shape (boto3's TransferConfig, the AWS SDK for Go's manager, s5cmd, ...):
// a bounded worker pool pulls part jobs off a queue, completions land out of
// order, a 503 SlowDown is retried inside the pool, and a part whose ETag was
// lost is re-uploaded idempotently (ADR-015 rule 5: same N → same offset →
// same ciphertext). The completed object must round-trip byte-for-byte.
func TestMultipartClientCompat_SDKTransferManager(t *testing.T) {
	for _, fv := range formatVersions {
		t.Run(fmt.Sprintf("format_v%d", fv), func(t *testing.T) {
			_, _, h := compatMatrixSetup(t, fv)
			bucket, key := "test-bucket", fmt.Sprintf("sdk-transfer-manager-v%d.bin", fv)

			const workers = 4
			const fullParts = 7
			const fullSize = 5 * 1024 * 1024
			// SDK retry policy for a 503-deferred part: real transfer
			// managers (boto3 TransferConfig, the Go SDK manager) retry
			// with backoff, not a tight loop — a deferred part must
			// outlast part 1's in-flight encryption rather than fail
			// the transfer after a few immediate attempts under CPU
			// contention.
			const deferralRetries = 12
			const deferralBackoff = 10 * time.Millisecond
			finalSize := 2*1024*1024 + 8191

			parts := make([][]byte, fullParts+1)
			var want []byte
			for p := 1; p <= fullParts+1; p++ {
				size := fullSize
				if p == fullParts+1 {
					size = finalSize
				}
				parts[p-1] = compatPart(t, p, size)
				want = append(want, parts[p-1]...)
			}

			uploadID := initiateMultipart(t, h, bucket, key)
			etags := make([]string, fullParts+1)

			jobs := make(chan int, fullParts+1)
			for p := 1; p <= fullParts+1; p++ {
				jobs <- p
			}
			close(jobs)

			var wg sync.WaitGroup
			errs := make(chan error, workers)
			for w := 0; w < workers; w++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for p := range jobs {
						// Standard SDK retry policy: 503 SlowDown is
						// retryable with backoff, anything else fails
						// the transfer.
						for attempt := 0; ; attempt++ {
							c, etag, body := uploadPartResponse(t, h, bucket, key, uploadID, p, parts[p-1])
							if c == http.StatusOK {
								etags[p-1] = etag
								break
							}
							if c == http.StatusServiceUnavailable && attempt < deferralRetries {
								time.Sleep(time.Duration(attempt+1) * deferralBackoff)
								continue
							}
							errs <- fmt.Errorf("part %d: status %d after retries: %s", p, c, body)
							break
						}
					}
				}()
			}
			wg.Wait()
			close(errs)
			for err := range errs {
				t.Error(err)
			}

			// The pool lost part 3's ETag and re-uploads it with the same
			// bytes. Same N → same offset → the retry must succeed (200), not
			// 400, on both formats.
			c, etag, body := uploadPartResponse(t, h, bucket, key, uploadID, 3, parts[2])
			if c != http.StatusOK {
				t.Fatalf("same-size part retry must be idempotent, got %d: %s", c, body)
			}
			etags[2] = etag

			completeMultipart(t, h, bucket, key, uploadID, etags)
			compatVerifyRoundTrip(t, h, bucket, key, want)
		})
	}
}

// TestMultipartClientCompat_Serial is the lowest-common-denominator client
// shape — one part at a time, strictly in order (aws cli with
// max_concurrent_requests=1, serial SDK transfer configs, barman). Accepted
// on both formats since ADR-003; the completed object must round-trip
// byte-for-byte.
func TestMultipartClientCompat_Serial(t *testing.T) {
	for _, fv := range formatVersions {
		t.Run(fmt.Sprintf("format_v%d", fv), func(t *testing.T) {
			_, _, h := compatMatrixSetup(t, fv)
			bucket, key := "test-bucket", fmt.Sprintf("serial-v%d.bin", fv)

			const fullParts = 4
			const fullSize = 5 * 1024 * 1024
			finalSize := 1024*1024 + 65535

			parts := make([][]byte, fullParts+1)
			var want []byte
			for p := 1; p <= fullParts+1; p++ {
				size := fullSize
				if p == fullParts+1 {
					size = finalSize
				}
				parts[p-1] = compatPart(t, p, size)
				want = append(want, parts[p-1]...)
			}

			uploadID := initiateMultipart(t, h, bucket, key)
			etags := make([]string, fullParts+1)
			for p := 1; p <= fullParts+1; p++ {
				etags[p-1] = uploadPart(t, h, bucket, key, uploadID, p, parts[p-1])
			}

			completeMultipart(t, h, bucket, key, uploadID, etags)
			compatVerifyRoundTrip(t, h, bucket, key, want)
		})
	}
}
