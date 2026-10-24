package server

// Unit tests for the aws-chunked streaming body decoder and its chained
// per-chunk signature verification. This is the wire format AWS SDKs and
// litestream use for streamed PUTs (X-Amz-Content-Sha256:
// STREAMING-AWS4-HMAC-SHA256-PAYLOAD).

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/server/handlers"
)

// encodeAWSChunked produces the chunked wire format with placeholder
// signatures:
//
//	<hex-size>;chunk-signature=<64 hex>\r\n<data>\r\n ... 0;chunk-signature=<64 hex>\r\n\r\n
//
// Readers built with a nil verifier ignore the signature fields, so these
// bytes also pin the strip-framing-only behaviour presigned requests get.
func encodeAWSChunked(payload []byte, chunkSize int) []byte {
	var buf bytes.Buffer
	sig := strings.Repeat("ab", 32)
	for off := 0; off < len(payload); off += chunkSize {
		end := off + chunkSize
		if end > len(payload) {
			end = len(payload)
		}
		fmt.Fprintf(&buf, "%x;chunk-signature=%s\r\n", end-off, sig)
		buf.Write(payload[off:end])
		buf.WriteString("\r\n")
	}
	fmt.Fprintf(&buf, "0;chunk-signature=%s\r\n\r\n", sig)
	return buf.Bytes()
}

// encodeSignedAWSChunked signs payload with the chained chunk scheme using v
// as the signing state and returns the wire bytes. It shares no state with
// the verifier a reader uses to check the result.
func encodeSignedAWSChunked(t *testing.T, v *chunkSignatureVerifier, payload []byte, chunkSize int, trailers string) []byte {
	t.Helper()
	var buf bytes.Buffer
	for off := 0; off < len(payload); off += chunkSize {
		end := off + chunkSize
		if end > len(payload) {
			end = len(payload)
		}
		data := payload[off:end]
		v.start(strings.Repeat("0", 64))
		v.hash.Write(data)
		sig := v.computeAndAdvance()
		fmt.Fprintf(&buf, "%x;chunk-signature=%s\r\n", len(data), sig)
		buf.Write(data)
		buf.WriteString("\r\n")
	}
	v.start(strings.Repeat("0", 64))
	sig := v.computeAndAdvance() // zero chunk
	fmt.Fprintf(&buf, "0;chunk-signature=%s\r\n", sig)
	buf.WriteString(trailers)
	if !strings.HasSuffix(trailers, "\r\n") {
		buf.WriteString("\r\n")
	}
	return buf.Bytes()
}

// docExampleVectors are the signing parameters and expected chunk signatures
// of the worked example in the AWS SigV4 streaming documentation ("Signature
// Calculations: Transferring Payload in Multiple Chunks"): a 65536-byte chunk
// of 'a's, a 1024-byte chunk of 'a's, and the zero-length final chunk.
const (
	docExampleSecretKey = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY" // gitleaks:allow
	docExampleAmzDate   = "20130524T000000Z"
	docExampleScope     = "20130524/us-east-1/s3/aws4_request"
	docExampleSeed      = "4f232c4386841ef735655705268965c44a0e4690baa4adea153f7db9fa80a0a9"
)

func docExampleChunks() []struct {
	data []byte
	sig  string
} {
	return []struct {
		data []byte
		sig  string
	}{
		{bytes.Repeat([]byte("a"), 65536), "ad80c730a21e5b8d04586a2213dd63b9a0e99e0e2307b0ade35a65485a288648"},
		{bytes.Repeat([]byte("a"), 1024), "0055627c9e194cb4542bae2aa5492e3c1575bbb81b612b7d234b86a503ef5497"},
		{nil, "b6c6ea8a5354eaf15b3cb7646744f4275b71ea724fed81ceb9323e279d449df9"},
	}
}

// TestChunkSignatureChainAWSDocumentExample pins the chunk string-to-sign
// construction to the documented example vectors: seed signature plus the
// hash of each chunk's data, HMAC'd with the derived signing key.
func TestChunkSignatureChainAWSDocumentExample(t *testing.T) {
	key := sigV4SigningKey(docExampleSecretKey, "20130524", "us-east-1", "s3")
	v := newChunkSignatureVerifier(key, docExampleAmzDate, docExampleScope, docExampleSeed)
	for i, chunk := range docExampleChunks() {
		v.start(chunk.sig)
		v.hash.Write(chunk.data)
		if err := v.finish(); err != nil {
			t.Fatalf("chunk %d: signature mismatch: %v", i, err)
		}
	}
}

// TestAWSChunkedVerifyAWSDocumentExampleRunsThroughReader feeds the documented
// example stream through the wire-format reader and requires it to decode.
func TestAWSChunkedVerifyAWSDocumentExampleRunsThroughReader(t *testing.T) {
	key := sigV4SigningKey(docExampleSecretKey, "20130524", "us-east-1", "s3")

	// Rebuild the example's wire form from the documented signatures rather
	// than from our own chain, so the reader is pinned to the published
	// values end to end.
	var buf bytes.Buffer
	for _, chunk := range docExampleChunks() {
		fmt.Fprintf(&buf, "%x;chunk-signature=%s\r\n", len(chunk.data), chunk.sig)
		buf.Write(chunk.data)
		buf.WriteString("\r\n")
	}
	sig := docExampleChunks()[2].sig
	fmt.Fprintf(&buf, "0;chunk-signature=%s\r\n\r\n", sig)

	r := newAWSChunkedReader(io.NopCloser(bytes.NewReader(buf.Bytes())),
		newChunkSignatureVerifier(key, docExampleAmzDate, docExampleScope, docExampleSeed))
	decoded, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if want := 65536 + 1024; len(decoded) != want {
		t.Fatalf("decoded %d bytes, want %d", len(decoded), want)
	}
	if !bytes.Equal(decoded, append(bytes.Repeat([]byte("a"), 65536), bytes.Repeat([]byte("a"), 1024)...)) {
		t.Fatal("decoded content mismatch")
	}
}

// TestAWSChunkedVerifySignedStream decodes a properly signed stream (odd chunk
// sizes, partial reads, checksum trailers after the zero chunk).
func TestAWSChunkedVerifySignedStream(t *testing.T) {
	for _, tc := range []struct {
		name      string
		size      int
		chunkSize int
		trailers  string
	}{
		{"single-chunk", 1024, 4096, ""},
		{"multi-chunk-partial-reads", 64*1024 + 137, 8192, ""},
		{"tiny-chunks", 1000, 7, ""},
		{"checksum-trailers", 500, 200, "x-amz-checksum-crc32:AAAAAA==\r\n\r\n"},
		{"empty-payload", 0, 4096, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := make([]byte, tc.size)
			for i := range payload {
				payload[i] = byte(i % 253)
			}
			signingKey := sigV4SigningKey("test-secret-key-for-chunking", "20261024", "us-east-005", "s3")
			seed := hex.EncodeToString(hmacSHA256(signingKey, "seed"))
			encoded := encodeSignedAWSChunked(t,
				newChunkSignatureVerifier(signingKey, "20261024T000000Z", "20261024/us-east-005/s3/aws4_request", seed),
				payload, tc.chunkSize, tc.trailers)

			r := newAWSChunkedReader(io.NopCloser(bytes.NewReader(encoded)),
				newChunkSignatureVerifier(signingKey, "20261024T000000Z", "20261024/us-east-005/s3/aws4_request", seed))
			decoded, err := io.ReadAll(r)
			if err != nil {
				t.Fatalf("read error: %v", err)
			}
			if !bytes.Equal(decoded, payload) {
				t.Fatalf("decoded %d bytes, want %d; content mismatch", len(decoded), len(payload))
			}
		})
	}
}

// readAllChunks drains r, returning the sticky verification error if any.
func readAllChunks(t *testing.T, r *awsChunkedReader) ([]byte, error) {
	t.Helper()
	var decoded bytes.Buffer
	buf := make([]byte, 333)
	for {
		n, err := r.Read(buf)
		decoded.Write(buf[:n])
		if err != nil {
			return decoded.Bytes(), err
		}
	}
}

// TestAWSChunkedVerifyRejectsTamperedChunk flips one byte of the second
// chunk's data after signing: the read must fail with
// ChunkSignatureMismatchError at chunk 1, stay failed on the next read, and
// stop the stream before the final chunk.
func TestAWSChunkedVerifyRejectsTamperedChunk(t *testing.T) {
	payload := bytes.Repeat([]byte("B"), 512)
	signingKey := sigV4SigningKey("test-secret-key-for-chunking", "20261024", "us-east-005", "s3")
	seed := hex.EncodeToString(hmacSHA256(signingKey, "seed"))
	encoded := encodeSignedAWSChunked(t,
		newChunkSignatureVerifier(signingKey, "20261024T000000Z", "20261024/us-east-005/s3/aws4_request", seed),
		payload, 100, "")
	// Corrupt the first byte of the second chunk's data: the signature line
	// and chunk header stay intact, the content no longer matches. Chunk 0
	// spans its header line, 100 data bytes, and the trailing CRLF, so chunk
	// 1's data begins one header line after that.
	headerLen := len(fmt.Sprintf("64;chunk-signature=%s\r\n", strings.Repeat("0", 64)))
	corruptAt := headerLen + 100 + 2 + headerLen
	encoded[corruptAt] ^= 0xff

	r := newAWSChunkedReader(io.NopCloser(bytes.NewReader(encoded)),
		newChunkSignatureVerifier(signingKey, "20261024T000000Z", "20261024/us-east-005/s3/aws4_request", seed))
	_, err := readAllChunks(t, r)
	var mismatch *handlers.ChunkSignatureMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("read error = %v, want ChunkSignatureMismatchError", err)
	}
	if mismatch.Chunk != 1 {
		t.Fatalf("failing chunk = %d, want 1", mismatch.Chunk)
	}
	// Sticky: the verification failure repeats on every subsequent read.
	if _, err := r.Read(make([]byte, 16)); !errors.As(err, &mismatch) {
		t.Fatalf("read after failure = %v, want the same ChunkSignatureMismatchError", err)
	}
}

// TestAWSChunkedVerifyRejectsWrongSeed signs a stream with one seed and
// verifies with another: the very first chunk must fail.
func TestAWSChunkedVerifyRejectsWrongSeed(t *testing.T) {
	payload := []byte("signed under one seed, checked under another")
	signingKey := sigV4SigningKey("test-secret-key-for-chunking", "20261024", "us-east-005", "s3")
	encoded := encodeSignedAWSChunked(t,
		newChunkSignatureVerifier(signingKey, "20261024T000000Z", "20261024/us-east-005/s3/aws4_request", strings.Repeat("a", 64)),
		payload, 16, "")

	r := newAWSChunkedReader(io.NopCloser(bytes.NewReader(encoded)),
		newChunkSignatureVerifier(signingKey, "20261024T000000Z", "20261024/us-east-005/s3/aws4_request", strings.Repeat("b", 64)))
	_, err := readAllChunks(t, r)
	var mismatch *handlers.ChunkSignatureMismatchError
	if !errors.As(err, &mismatch) || mismatch.Chunk != 0 {
		t.Fatalf("read error = %v, want ChunkSignatureMismatchError{Chunk:0}", err)
	}
}

// TestAWSChunkedVerifyRejectsMissingSignature covers a stream whose chunk
// headers carry no chunk-signature parameter at all: with verification on,
// that is a failed chunk, not silently accepted framing.
func TestAWSChunkedVerifyRejectsMissingSignature(t *testing.T) {
	payload := []byte("framing with the signature parameter stripped")
	var buf bytes.Buffer
	for off := 0; off < len(payload); off += 16 {
		end := off + 16
		if end > len(payload) {
			end = len(payload)
		}
		fmt.Fprintf(&buf, "%x\r\n", end-off)
		buf.Write(payload[off:end])
		buf.WriteString("\r\n")
	}
	buf.WriteString("0\r\n\r\n")

	signingKey := sigV4SigningKey("test-secret-key-for-chunking", "20261024", "us-east-005", "s3")
	r := newAWSChunkedReader(io.NopCloser(bytes.NewReader(buf.Bytes())),
		newChunkSignatureVerifier(signingKey, "20261024T000000Z", "20261024/us-east-005/s3/aws4_request", strings.Repeat("c", 64)))
	_, err := readAllChunks(t, r)
	var mismatch *handlers.ChunkSignatureMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("read error = %v, want ChunkSignatureMismatchError", err)
	}
}

// TestAWSChunkedVerifyRejectsTamperedZeroChunk keeps every data chunk honest
// but corrupts only the zero chunk's signature: the chain must still fail,
// and after the whole payload has been read.
func TestAWSChunkedVerifyRejectsTamperedZeroChunk(t *testing.T) {
	payload := []byte("payload whose final chunk signature was swapped")
	signingKey := sigV4SigningKey("test-secret-key-for-chunking", "20261024", "us-east-005", "s3")
	seed := hex.EncodeToString(hmacSHA256(signingKey, "seed"))
	encoded := encodeSignedAWSChunked(t,
		newChunkSignatureVerifier(signingKey, "20261024T000000Z", "20261024/us-east-005/s3/aws4_request", seed),
		payload, 16, "")
	// Swap in a syntactically valid but wrong zero-chunk signature. Anchor on
	// the preceding CRLF so a data chunk header like "10;chunk-signature="
	// cannot match first.
	zeroAnchor := "\r\n0;chunk-signature="
	i := bytes.Index(encoded, []byte(zeroAnchor))
	if i < 0 {
		t.Fatal("could not locate zero chunk header in encoded stream")
	}
	start := i + len(zeroAnchor)
	copy(encoded[start:start+64], strings.Repeat("9", 64))

	r := newAWSChunkedReader(io.NopCloser(bytes.NewReader(encoded)),
		newChunkSignatureVerifier(signingKey, "20261024T000000Z", "20261024/us-east-005/s3/aws4_request", seed))
	decoded, err := readAllChunks(t, r)
	var mismatch *handlers.ChunkSignatureMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("read error = %v, want ChunkSignatureMismatchError", err)
	}
	if mismatch.Chunk != 3 { // 48 bytes / 16 per chunk = 3 data chunks
		t.Fatalf("failing chunk = %d, want 3 (the zero chunk)", mismatch.Chunk)
	}
	// The payload itself decoded; the failure is the terminal chunk's
	// signature, and nothing must be treated as complete.
	if !bytes.Equal(decoded, payload) {
		t.Fatalf("decoded %q, want %q", decoded, payload)
	}
	if _, err := r.Read(make([]byte, 8)); !errors.As(err, &mismatch) {
		t.Fatalf("read after failure = %v, want the same ChunkSignatureMismatchError", err)
	}
}

// TestAWSChunkedNilVerifierStripsFraming pins the fallback used for requests
// without an Authorization header (presigned URLs): placeholder signatures
// decode fine because no verification is requested.
func TestAWSChunkedNilVerifierStripsFraming(t *testing.T) {
	payload := []byte("framing stripped without verification")
	r := newAWSChunkedReader(io.NopCloser(bytes.NewReader(encodeAWSChunked(payload, 16))), nil)
	decoded, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if !bytes.Equal(decoded, payload) {
		t.Fatalf("decoded %q, want %q", decoded, payload)
	}
}

func TestAWSChunkedDecodeRoundTrip(t *testing.T) {
	cases := []struct {
		name      string
		size      int
		chunkSize int
	}{
		{"single-chunk", 1024, 4096},
		{"multi-chunk", 64*1024 + 137, 8192},
		{"chunk-boundary-exact", 16384, 4096},
		{"tiny-chunks", 1000, 7},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := make([]byte, tc.size)
			for i := range payload {
				payload[i] = byte(i % 253)
			}
			encoded := encodeAWSChunked(payload, tc.chunkSize)
			r := newAWSChunkedReader(io.NopCloser(bytes.NewReader(encoded)), nil)

			// Read through a deliberately small buffer to exercise partial reads.
			var decoded bytes.Buffer
			buf := make([]byte, 333)
			for {
				n, err := r.Read(buf)
				decoded.Write(buf[:n])
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatalf("read error: %v", err)
				}
			}
			if !bytes.Equal(decoded.Bytes(), payload) {
				t.Fatalf("decoded %d bytes, want %d; content mismatch", decoded.Len(), len(payload))
			}
		})
	}
}

func TestAWSChunkedDecodeEmptyPayload(t *testing.T) {
	encoded := encodeAWSChunked(nil, 4096)
	r := newAWSChunkedReader(io.NopCloser(bytes.NewReader(encoded)), nil)
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if len(data) != 0 {
		t.Fatalf("expected empty payload, got %d bytes", len(data))
	}
}
