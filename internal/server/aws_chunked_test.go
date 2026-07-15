package server

// Unit tests for the aws-chunked streaming body decoder (matrix row U3,
// docs/upload-retrieval-test-matrix.md). This is the wire format AWS SDKs
// and litestream use for streamed PUTs (X-Amz-Content-Sha256:
// STREAMING-AWS4-HMAC-SHA256-PAYLOAD).
//
// NOTE: as of 2026-07-15 this package does not compile on main (bf-15sdaf,
// redeclared helpers in test_request_validation_helpers.go). These tests
// become runnable the moment that bead is fixed — run:
//   go test ./internal/server/ -run TestAWSChunked

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
)

// encodeAWSChunked produces the chunked wire format:
//
//	<hex-size>;chunk-signature=<64 hex>\r\n<data>\r\n ... 0;chunk-signature=<64 hex>\r\n\r\n
func encodeAWSChunked(payload []byte, chunkSize int) []byte {
	var buf bytes.Buffer
	sig := strings.Repeat("ab", 32) // 64 hex chars; ARMOR does not re-verify per-chunk sigs
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
			r := newAWSChunkedReader(io.NopCloser(bytes.NewReader(encoded)))

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
	r := newAWSChunkedReader(io.NopCloser(bytes.NewReader(encoded)))
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if len(data) != 0 {
		t.Fatalf("expected empty payload, got %d bytes", len(data))
	}
}
