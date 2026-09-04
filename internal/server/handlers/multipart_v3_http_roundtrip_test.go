package handlers_test

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/jedarden/armor/internal/server/handlers"
)

// TestMultipartV3HTTPConcurrentShuffledUnalignedRoundTrip exercises the S3
// HTTP surface, rather than calling the individual multipart handlers. It
// covers both the barman-cloud part shape (chunk_size + n*512) and the
// aws-cli default transfer-manager shape (8 MiB parts with concurrency 10).
// Each upload deliberately has a one-byte final part: it is the smallest
// possible non-uniform final block and makes byte-boundary mistakes visible.
func TestMultipartV3HTTPConcurrentShuffledUnalignedRoundTrip(t *testing.T) {
	const mib = 1024 * 1024

	tests := []struct {
		name        string
		partSizes   []int
		concurrency int
	}{
		{
			name: "barman_chunk_plus_tar_blocks",
			partSizes: []int{
				5*mib + 512,
				5*mib + 3*512,
				5*mib + 11*512,
				1,
			},
			concurrency: 4,
		},
		{
			name:        "aws_cli_default_8_mib_concurrency_10",
			partSizes:   append(repeatPartSize(10, 8*mib), 1),
			concurrency: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, _, h := recordingTestSetup(t)
			cfg.FormatWriteVersion = 3

			bucket := "test-bucket"
			key := "v3-http-" + tt.name + ".tar"
			parts, plaintext := v3MultipartFixture(tt.partSizes)
			uploadID := initiateMultipart(t, h, bucket, key)

			etags := uploadV3PartsConcurrently(t, h, bucket, key, uploadID, parts, tt.concurrency)
			completeMultipart(t, h, bucket, key, uploadID, etags)

			fullGet := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/%s/%s", bucket, key), nil)
			fullResponse := httptest.NewRecorder()
			h.HandleRoot(fullResponse, fullGet)
			if fullResponse.Code != http.StatusOK {
				t.Fatalf("full GET failed: status %d: %s", fullResponse.Code, fullResponse.Body.String())
			}
			if !bytes.Equal(fullResponse.Body.Bytes(), plaintext) {
				t.Fatalf("full GET mismatch: got %d bytes, want %d; first divergence at %d",
					fullResponse.Body.Len(), len(plaintext), firstDivergence(fullResponse.Body.Bytes(), plaintext))
			}

			// Random bounded ranges cover partial blocks and part-local offsets.
			// The seed makes a failure reproducible while the ranges still span the
			// entire object rather than relying on hand-picked boundaries.
			rng := rand.New(rand.NewSource(20260829 + int64(len(plaintext))))
			for i := 0; i < 20; i++ {
				start := rng.Intn(len(plaintext))
				length := 1 + rng.Intn(4*65536)
				end := start + length - 1
				if end >= len(plaintext) {
					end = len(plaintext) - 1
				}

				rangeRequest := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/%s/%s", bucket, key), nil)
				rangeRequest.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
				rangeResponse := httptest.NewRecorder()
				h.HandleRoot(rangeResponse, rangeRequest)
				if rangeResponse.Code != http.StatusPartialContent {
					t.Fatalf("range %d (%d-%d) failed: status %d: %s", i, start, end, rangeResponse.Code, rangeResponse.Body.String())
				}
				if !bytes.Equal(rangeResponse.Body.Bytes(), plaintext[start:end+1]) {
					t.Fatalf("range %d (%d-%d) mismatch; first divergence at %d", i, start, end,
						firstDivergence(rangeResponse.Body.Bytes(), plaintext[start:end+1]))
				}
			}
		})
	}

	t.Run("abort_removes_v3_upload_state", func(t *testing.T) {
		cfg, rb, h := recordingTestSetup(t)
		cfg.FormatWriteVersion = 3
		bucket, key := "test-bucket", "aborted-v3-upload.tar"
		uploadID := initiateMultipart(t, h, bucket, key)

		// Persist both meta.json and a part file before aborting, so this proves
		// AbortMultipartUpload removes more than the backend upload handle.
		uploadPart(t, h, bucket, key, uploadID, 1, multipartPattern(0, 65536+17))

		abortRequest := httptest.NewRequest(http.MethodDelete,
			fmt.Sprintf("/%s/%s?uploadId=%s", bucket, key, uploadID), nil)
		abortResponse := httptest.NewRecorder()
		h.HandleRoot(abortResponse, abortRequest)
		if abortResponse.Code != http.StatusNoContent {
			t.Fatalf("AbortMultipartUpload failed: status %d: %s", abortResponse.Code, abortResponse.Body.String())
		}

		statePrefix := bucket + "/.armor/multipart/" + uploadID + "/"
		rb.mu.Lock()
		for objectKey := range rb.objects {
			if strings.HasPrefix(objectKey, statePrefix) {
				rb.mu.Unlock()
				t.Fatalf("abort left multipart state object %q", objectKey)
			}
		}
		rb.mu.Unlock()

		rb.rmu.Lock()
		_, uploadStillExists := rb.uploads[uploadID]
		rb.rmu.Unlock()
		if uploadStillExists {
			t.Fatal("abort left the backend multipart upload active")
		}

		// The removed state must make subsequent HTTP operations fail with the
		// standard S3 NoSuchUpload response rather than recreating state.
		code, _, body := uploadPartResponse(t, h, bucket, key, uploadID, 1, []byte("retry"))
		if code != http.StatusNotFound || !strings.Contains(body, "NoSuchUpload") {
			t.Fatalf("UploadPart after abort = %d %q, want 404 NoSuchUpload", code, body)
		}
	})
}

func repeatPartSize(count, size int) []int {
	parts := make([]int, count)
	for i := range parts {
		parts[i] = size
	}
	return parts
}

func v3MultipartFixture(partSizes []int) ([][]byte, []byte) {
	parts := make([][]byte, len(partSizes))
	var plaintext []byte
	base := 0
	for i, size := range partSizes {
		parts[i] = multipartPattern(base, size)
		plaintext = append(plaintext, parts[i]...)
		base += size
	}
	return parts, plaintext
}

// multipartPattern changes with its absolute byte offset so a wrong part
// ordering, offset, or range splice diverges immediately.
func multipartPattern(base, size int) []byte {
	part := make([]byte, size)
	for i := range part {
		offset := base + i
		part[i] = byte(offset*31) ^ byte(offset>>7) ^ byte(offset>>19)
	}
	return part
}

func uploadV3PartsConcurrently(t *testing.T, h *handlers.Handlers, bucket, key, uploadID string, parts [][]byte, concurrency int) []string {
	t.Helper()
	if concurrency < 1 {
		t.Fatal("multipart test requires positive concurrency")
	}

	order := make([]int, len(parts))
	for i := range order {
		order[i] = i
	}
	rand.New(rand.NewSource(20260829)).Shuffle(len(order), func(i, j int) {
		order[i], order[j] = order[j], order[i]
	})

	type result struct {
		part int
		etag string
		code int
		body string
	}
	results := make(chan result, len(parts))
	semaphore := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for _, partIndex := range order {
		partIndex := partIndex
		wg.Add(1)
		go func() {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			partNumber := partIndex + 1
			request := httptest.NewRequest(http.MethodPut,
				fmt.Sprintf("/%s/%s?partNumber=%d&uploadId=%s", bucket, key, partNumber, uploadID),
				bytes.NewReader(parts[partIndex]))
			response := httptest.NewRecorder()
			h.HandleRoot(response, request)
			results <- result{
				part: partNumber,
				etag: response.Header().Get("ETag"),
				code: response.Code,
				body: response.Body.String(),
			}
		}()
	}
	wg.Wait()
	close(results)

	etags := make([]string, len(parts))
	for result := range results {
		if result.code != http.StatusOK {
			t.Fatalf("UploadPart %d failed: status %d: %s", result.part, result.code, result.body)
		}
		if result.etag == "" {
			t.Fatalf("UploadPart %d returned no ETag", result.part)
		}
		etags[result.part-1] = result.etag
	}
	return etags
}

// v3SidecarKey reproduces the sidecar naming rule shared by the writer and both
// v3 read paths: .armor/hmac/<sha256(object key)>, relative to the bucket.
func v3SidecarKey(bucket, key string) string {
	sum := sha256.Sum256([]byte(key))
	return bucket + "/.armor/hmac/" + fmt.Sprintf("%x", sum)
}

// TestMultipartV3SidecarUsesClientKeyUnderPrefix is the v3 sibling of
// TestMultipartAppliesArmorPrefix. CompleteMultipartUpload saves the v3 HMAC
// sidecar under sha256 of the CLIENT key, while the assembled ciphertext is
// stored under the PREFIXED key. Both v3 read paths hashed the prefixed key
// instead, so on any deployment with ARMOR_PREFIX set every v3 multipart GET
// failed with "failed to load v3 HMAC table: 404 NoSuchKey" — the sidecar was
// in the bucket all along, under the name the writer chose. No data was lost.
//
// Invisible wherever ARMOR_PREFIX was empty, because the two keys coincide.
// Found live on ord-devimprint (ARMOR_PREFIX="commitgraph/"): 554 GetObject 500s
// across the 55 minutes after rollout, blocking litestream L2 compaction.
func TestMultipartV3SidecarUsesClientKeyUnderPrefix(t *testing.T) {
	const mib = 1024 * 1024
	const prefix = "tenant-a/"
	cfg, rb, h := recordingTestSetupWithPrefix(t, prefix)
	cfg.FormatWriteVersion = 3

	bucket, key := "test-bucket", "backups/base/data.tar"

	// 5 MiB+512 keeps the non-final part above multipartMinPartSize; the 1-byte
	// final part is the smallest non-uniform ending and pins the part splice.
	parts, plaintext := v3MultipartFixture([]int{5*mib + 512, 1})

	uploadID := initiateMultipart(t, h, bucket, key)
	etags := uploadV3PartsConcurrently(t, h, bucket, key, uploadID, parts, 4)
	completeMultipart(t, h, bucket, key, uploadID, etags)

	// The sidecar must exist under sha256 of the CLIENT key — the name the
	// writer chose — and must NOT exist under sha256 of the prefixed storage
	// key, which is what the broken readers asked for.
	rb.mu.Lock()
	_, sidecarAtClientKey := rb.objects[v3SidecarKey(bucket, key)]
	_, sidecarAtPrefixedKey := rb.objects[v3SidecarKey(bucket, prefix+key)]
	rb.mu.Unlock()
	if !sidecarAtClientKey {
		t.Fatalf("v3 sidecar missing at .armor/hmac/<sha256(%q)>", key)
	}
	if sidecarAtPrefixedKey {
		t.Errorf("v3 sidecar was written under the PREFIXED key %q; readers and writers disagree", prefix+key)
	}

	// Full GET: the object must read back byte-for-byte through the handler.
	fullGet := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/%s/%s", bucket, key), nil)
	fullResponse := httptest.NewRecorder()
	h.HandleRoot(fullResponse, fullGet)
	if fullResponse.Code != http.StatusOK {
		t.Fatalf("full GET failed: status %d: %s", fullResponse.Code, fullResponse.Body.String())
	}
	if !bytes.Equal(fullResponse.Body.Bytes(), plaintext) {
		t.Fatalf("full GET mismatch: got %d bytes, want %d; first divergence at %d",
			fullResponse.Body.Len(), len(plaintext), firstDivergence(fullResponse.Body.Bytes(), plaintext))
	}

	// Range GET covers the second v3 read path, which shares the sidecar loader.
	rangeRequest := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/%s/%s", bucket, key), nil)
	rangeRequest.Header.Set("Range", "bytes=0-65535")
	rangeResponse := httptest.NewRecorder()
	h.HandleRoot(rangeResponse, rangeRequest)
	if rangeResponse.Code != http.StatusPartialContent {
		t.Fatalf("range GET failed: status %d: %s", rangeResponse.Code, rangeResponse.Body.String())
	}
	if !bytes.Equal(rangeResponse.Body.Bytes(), plaintext[:65536]) {
		t.Fatalf("range GET mismatch; first divergence at %d",
			firstDivergence(rangeResponse.Body.Bytes(), plaintext[:65536]))
	}
}
