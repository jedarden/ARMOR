package awsclicompat

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	armorcrypto "github.com/jedarden/armor/internal/crypto"
	"github.com/jedarden/armor/internal/presign"
	"github.com/jedarden/armor/internal/server"
)

const compressionHarnessBucket = "compression-harness-bucket"

// compressionHarnessStorage adapts CompressionTestUtilities to the encrypted
// object format consumed by the real share handler. CompressionTestUtilities
// deliberately deals in payload bytes; this adapter adds the ARMOR envelope,
// encryption, HMAC table, and metadata needed to exercise the request path.
type compressionHarnessStorage struct {
	backend *mockBackend
	mek     []byte
}

func (s *compressionHarnessStorage) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("read compression test payload: %w", err)
	}
	if int64(len(data)) != size {
		return fmt.Errorf("compression test payload size = %d, want %d", len(data), size)
	}

	compressed := meta["test-compressed"] == "true"
	if compressed && !armorcrypto.IsCompressed(data) {
		return fmt.Errorf("compressed test payload is missing zstd framing")
	}

	const blockSize = 65536
	dek, err := armorcrypto.GenerateDEK()
	if err != nil {
		return fmt.Errorf("generate test DEK: %w", err)
	}
	iv, err := armorcrypto.GenerateIV()
	if err != nil {
		return fmt.Errorf("generate test IV: %w", err)
	}
	encryptor, err := armorcrypto.NewEncryptor(dek, iv, blockSize)
	if err != nil {
		return fmt.Errorf("create test encryptor: %w", err)
	}
	encrypted, hmacTable, err := encryptor.Encrypt(data)
	if err != nil {
		return fmt.Errorf("encrypt compression test payload: %w", err)
	}
	wrappedDEK, err := armorcrypto.WrapDEK(s.mek, dek)
	if err != nil {
		return fmt.Errorf("wrap test DEK: %w", err)
	}

	digest := sha256.Sum256(data)
	contentType := meta["content-type"]
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	armorMeta := &backend.ARMORMetadata{
		Version:       1,
		BlockSize:     blockSize,
		PlaintextSize: int64(len(data)),
		ContentType:   contentType,
		IV:            iv,
		WrappedDEK:    wrappedDEK,
		PlaintextSHA:  hex.EncodeToString(digest[:]),
		ETag:          fmt.Sprintf("\"%x\"", digest[:8]),
		Compressed:    compressed,
		CompressionType: func() backend.CompressionType {
			if compressed {
				return backend.CompressionZstd
			}
			return backend.CompressionNone
		}(),
	}

	header, err := armorcrypto.NewEnvelopeHeader(iv, int64(len(data)), blockSize, digest)
	if err != nil {
		return fmt.Errorf("create test envelope header: %w", err)
	}
	headerBytes, err := header.Encode()
	if err != nil {
		return fmt.Errorf("encode test envelope header: %w", err)
	}

	object := make([]byte, 0, len(headerBytes)+len(encrypted)+len(hmacTable))
	object = append(object, headerBytes...)
	object = append(object, encrypted...)
	object = append(object, hmacTable...)
	return s.backend.Put(ctx, bucket, key, bytes.NewReader(object), int64(len(object)), armorMeta.ToMetadata())
}

func (s *compressionHarnessStorage) Delete(ctx context.Context, bucket, key string) error {
	return s.backend.Delete(ctx, bucket, key)
}

// compressionRoundTripHarness owns an in-process ARMOR server and the object
// setup helpers used by compression/share/range tests. It intentionally keeps
// the share request separate from the SDK client so tests can exercise the
// public pre-signed URL operation directly.
type compressionRoundTripHarness struct {
	ctx     context.Context
	bucket  string
	server  *httptest.Server
	signer  *presign.Signer
	backend *mockBackend
	objects *armorcrypto.CompressionTestUtilities
}

func newCompressionRoundTripHarness(t *testing.T) *compressionRoundTripHarness {
	t.Helper()

	mek := testMEK()
	cfg := &config.Config{
		B2Region:        testRegion,
		MEK:             mek,
		BlockSize:       65536,
		CacheMaxEntries: 1000,
		CacheTTL:        300,
		AuthAccessKey:   testAccessKey,
		AuthSecretKey:   testSecretKey,
		Credentials: map[string]*config.Credential{
			testAccessKey: {
				AccessKey: testAccessKey,
				SecretKey: testSecretKey,
			},
		},
	}

	store := newMockBackend()
	armorServer, err := server.NewWithBackend(cfg, store)
	if err != nil {
		t.Fatalf("NewWithBackend: %v", err)
	}
	presignSecret, err := hex.DecodeString(testPresignSecret)
	if err != nil {
		t.Fatalf("decode test presign secret: %v", err)
	}
	signer := presign.NewSigner(presignSecret, "")
	armorServer.SetPresigner(signer)
	httpServer := httptest.NewServer(armorServer.Handler())

	harness := &compressionRoundTripHarness{
		ctx:     context.Background(),
		bucket:  compressionHarnessBucket,
		server:  httpServer,
		signer:  signer,
		backend: store,
	}
	harness.objects = armorcrypto.NewCompressionTestUtilities(&compressionHarnessStorage{
		backend: store,
		mek:     mek,
	})

	t.Cleanup(func() {
		if err := harness.objects.TeardownAllTestObjects(harness.ctx, harness.bucket); err != nil {
			t.Errorf("tear down compression test objects: %v", err)
		}
		httpServer.Close()
	})
	return harness
}

// shareGET generates a valid share token and performs the corresponding GET.
// A non-empty rangeHeader is sent as the request's Range header, allowing the
// same helper to cover full and partial share downloads.
func (h *compressionRoundTripHarness) shareGET(t *testing.T, key, rangeHeader string) *http.Response {
	t.Helper()

	token, err := h.signer.GenerateToken(h.bucket, key, time.Hour)
	if err != nil {
		t.Fatalf("generate share token: %v", err)
	}
	req, err := http.NewRequestWithContext(h.ctx, http.MethodGet, h.server.URL+"/share/"+token, nil)
	if err != nil {
		t.Fatalf("create share GET request: %v", err)
	}
	if rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("perform share GET: %v", err)
	}
	return response
}

// simulateRange parses and extracts a logical plaintext range. The compressed
// flag is retained in the simulator so callers exercise the same API for both
// object forms; the range is always resolved against the plaintext because a
// compressed object cannot safely seek by its decompressed offset.
func simulateRange(data []byte, compressed bool, rangeHeader string) (*armorcrypto.RangeResult, error) {
	return armorcrypto.NewRangeSimulator(data, compressed, 65536).SimulateRangeRequest(rangeHeader)
}

// verifyByteIdenticalDecompression is the harness-level assertion for share
// responses. It reports the first byte/length mismatch through the shared
// structured verifier rather than relying on string conversion or checksums.
func verifyByteIdenticalDecompression(t *testing.T, actual, expected []byte) {
	t.Helper()
	result := armorcrypto.VerifyDecompression(actual, expected)
	if !result.Pass {
		t.Fatalf("decompression verification failed: %s", result.Diagnostic)
	}
}

func verifyByteIdenticalRange(t *testing.T, original, actual []byte, offset, length int64) {
	t.Helper()
	result := armorcrypto.VerifyRangeDecompressionWithBounds(original, actual, offset, length)
	if !result.Pass {
		t.Fatalf("range decompression verification failed: %s", result.Diagnostic)
	}
}

// assertRangeRequestRejected verifies that a range request on a compressed object
// is rejected. Compressed objects reject all range requests with either:
// - 416 (RequestedRangeNotSatisfiable) for valid ranges that are rejected due to compression
// - 400 (BadRequest) for syntactically invalid ranges
func assertRangeRequestRejected(t *testing.T, resp *http.Response) {
	t.Helper()

	parsed, err := parseGETResponse(resp)
	if err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Share endpoint returns either 400 (invalid range syntax) or 416 (compressed object)
	// Both are acceptable fail-closed behaviors
	if parsed.StatusCode != http.StatusBadRequest && parsed.StatusCode != http.StatusRequestedRangeNotSatisfiable {
		t.Fatalf("expected status 400 or 416 for compressed object range request, got %d", parsed.StatusCode)
	}

	// For 416 responses, verify the error message is clear
	if parsed.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		errorBody := string(parsed.Body)
		// The share endpoint uses plain text error (http.Error), not XML
		if !bytes.Contains(parsed.Body, []byte("Range reads unsupported on compressed objects")) {
			t.Errorf("416 response should mention compressed objects, got: %s", errorBody)
		}
	}
}

func TestCompressionRoundTripHarness_ShareGET(t *testing.T) {
	harness := newCompressionRoundTripHarness(t)
	content := armorcrypto.GeneratePatternContent("share compression harness payload ", 1024)
	compressed, uncompressed, err := harness.objects.CreateTestObjectPair(
		harness.ctx,
		harness.bucket,
		"share/compressed-object",
		"share/uncompressed-object",
		content,
		"application/octet-stream",
	)
	if err != nil {
		t.Fatalf("create compression test object pair: %v", err)
	}
	if err := armorcrypto.VerifyCompressedData(compressed.StoredContent, content); err != nil {
		t.Fatalf("compressed setup verification: %v", err)
	}
	if !bytes.Equal(uncompressed.StoredContent, content) {
		t.Fatal("uncompressed setup object does not preserve its payload")
	}

	for _, object := range []*armorcrypto.TestObject{compressed, uncompressed} {
		t.Run(map[bool]string{true: "compressed", false: "uncompressed"}[object.Compressed], func(t *testing.T) {
			response := harness.shareGET(t, object.Key, "")
			parsed := assertGETSuccess(t, response)
			verifyByteIdenticalDecompression(t, parsed.Body, object.OriginalContent)
			if parsed.ContentLength != int64(len(object.OriginalContent)) {
				t.Fatalf("share Content-Length = %d, want %d", parsed.ContentLength, len(object.OriginalContent))
			}
		})
	}
}

func TestCompressionRoundTripHarness_RangeRequests(t *testing.T) {
	harness := newCompressionRoundTripHarness(t)
	content := armorcrypto.GeneratePatternContent("range compression harness payload ", 1024)
	compressed, uncompressed, err := harness.objects.CreateTestObjectPair(
		harness.ctx,
		harness.bucket,
		"range/compressed-object",
		"range/uncompressed-object",
		content,
		"application/octet-stream",
	)
	if err != nil {
		t.Fatalf("create range test object pair: %v", err)
	}

	const rangeHeader = "bytes=1024-4095"
	for _, object := range []*armorcrypto.TestObject{compressed, uncompressed} {
		t.Run(map[bool]string{true: "compressed", false: "uncompressed"}[object.Compressed], func(t *testing.T) {
			// The share endpoint validates a compressed request against the stored
			// (compressed) length before rejecting compressed seeking. Use a valid
			// one-byte request for that rejection path; the logical helper still
			// exercises the same plaintext range semantics for both object forms.
			requestRange := rangeHeader
			if object.Compressed {
				requestRange = "bytes=0-0"
			}
			simulated, err := simulateRange(object.OriginalContent, object.Compressed, requestRange)
			if err != nil {
				t.Fatalf("simulate range request: %v", err)
			}
			start, end := simulated.Spec.ResolveRange(int64(len(object.OriginalContent)))
			if err := simulated.Verify(object.OriginalContent[start : end+1]); err != nil {
				t.Fatalf("simulated range verification: %v", err)
			}

			response := harness.shareGET(t, object.Key, requestRange)
			parsed, err := parseGETResponse(response)
			if err != nil {
				t.Fatalf("parse range share response: %v", err)
			}
			if object.Compressed {
				if parsed.StatusCode != http.StatusRequestedRangeNotSatisfiable {
					t.Fatalf("compressed share range status = %d, want %d", parsed.StatusCode, http.StatusRequestedRangeNotSatisfiable)
				}
				return
			}

			if parsed.StatusCode != http.StatusPartialContent {
				t.Fatalf("uncompressed share range status = %d, want %d", parsed.StatusCode, http.StatusPartialContent)
			}
			verifyByteIdenticalRange(t, object.OriginalContent, parsed.Body, simulated.Spec.Start, simulated.ContentLength)
			if got := parsed.Headers["Content-Range"]; got != simulated.ContentRange {
				t.Fatalf("share Content-Range = %q, want %q", got, simulated.ContentRange)
			}
		})
	}
}

// TestCompressionRangeRequestFailClosed verifies that range requests against
// compressed objects are rejected with proper error messages, demonstrating
// fail-closed behavior
func TestCompressionRangeRequestFailClosed(t *testing.T) {
	harness := newCompressionRoundTripHarness(t)
	content := armorcrypto.GeneratePatternContent("fail-closed range test payload ", 2048)
	compressed, uncompressed, err := harness.objects.CreateTestObjectPair(
		harness.ctx,
		harness.bucket,
		"rangefail/compressed-object",
		"rangefail/uncompressed-object",
		content,
		"application/octet-stream",
	)
	if err != nil {
		t.Fatalf("create fail-closed test object pair: %v", err)
	}

	// Test various range request patterns against compressed objects
	rangePatterns := []string{
		"bytes=0-0",      // First byte
		"bytes=0-1023",   // First kilobyte
		"bytes=512-1535", // Middle range
		"bytes=-512",     // Last 512 bytes
		"bytes=1024-",    // From offset to end
	}

	for _, pattern := range rangePatterns {
		t.Run("compressed_rejects_"+pattern, func(t *testing.T) {
			response := harness.shareGET(t, compressed.Key, pattern)
			assertRangeRequestRejected(t, response)
		})

		t.Run("uncompressed_accepts_"+pattern, func(t *testing.T) {
			response := harness.shareGET(t, uncompressed.Key, pattern)
			parsed, err := parseGETResponse(response)
			if err != nil {
				t.Fatalf("parse uncompressed range response: %v", err)
			}

			// Uncompressed objects should accept range requests
			if parsed.StatusCode != http.StatusPartialContent && parsed.StatusCode != http.StatusOK {
				t.Fatalf("uncompressed object rejected range request: status %d, body %s", parsed.StatusCode, string(parsed.Body))
			}

			// Verify Content-Range header is present for partial content
			if parsed.StatusCode == http.StatusPartialContent {
				if _, ok := parsed.Headers["Content-Range"]; !ok {
					t.Errorf("uncompressed range request missing Content-Range header")
				}
			}
		})
	}
}
