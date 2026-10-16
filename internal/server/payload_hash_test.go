package server

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/server/handlers"
)

func TestClassifyPayloadHashMode(t *testing.T) {
	digest := sha256Sum([]byte("anything"))

	t.Run("absent header behaves like UNSIGNED-PAYLOAD", func(t *testing.T) {
		mode, err := classifyPayloadHashMode("")
		if err != nil {
			t.Fatalf("absent header: unexpected error %v", err)
		}
		if mode != payloadUnsigned {
			t.Fatalf("absent header: mode = %v, want payloadUnsigned", mode)
		}
	})

	t.Run("UNSIGNED-PAYLOAD accepted", func(t *testing.T) {
		mode, err := classifyPayloadHashMode("UNSIGNED-PAYLOAD")
		if err != nil {
			t.Fatalf("UNSIGNED-PAYLOAD: unexpected error %v", err)
		}
		if mode != payloadUnsigned {
			t.Fatalf("UNSIGNED-PAYLOAD: mode = %v, want payloadUnsigned", mode)
		}
	})

	t.Run("streaming header accepted", func(t *testing.T) {
		mode, err := classifyPayloadHashMode("STREAMING-AWS4-HMAC-SHA256-PAYLOAD")
		if err != nil {
			t.Fatalf("streaming header: unexpected error %v", err)
		}
		if mode != payloadStreaming {
			t.Fatalf("streaming header: mode = %v, want payloadStreaming", mode)
		}
	})

	t.Run("hex digest accepted in either case", func(t *testing.T) {
		for _, value := range []string{digest, strings.ToUpper(digest)} {
			mode, err := classifyPayloadHashMode(value)
			if err != nil {
				t.Fatalf("hex digest %s: unexpected error %v", value, err)
			}
			if mode != payloadDigest {
				t.Fatalf("hex digest %s: mode = %v, want payloadDigest", value, mode)
			}
		}
	})

	t.Run("unsupported values rejected with the supported modes named", func(t *testing.T) {
		for _, value := range []string{
			"STREAMING-UNSIGNED-PAYLOAD-TRAILER",
			"UNSIGNED-PAYLOAD-TRAILER",
			"unsigned-payload",
			digest[:63],
			digest + "0",
			"zz",
		} {
			_, err := classifyPayloadHashMode(value)
			if err == nil {
				t.Fatalf("value %q: expected rejection, got nil error", value)
			}
			for _, wanted := range []string{value, unsignedPayloadHeader, streamingPayloadHeader} {
				if !strings.Contains(err.Error(), wanted) {
					t.Errorf("value %q: error %q does not name %q", value, err.Error(), wanted)
				}
			}
		}
	})
}

// sliceReader is a minimal io.ReadCloser over an in-memory buffer that can be
// told to coalesce its final bytes into a single read, exercising the
// (n > 0, io.EOF) return shape real request bodies produce.
type sliceReader struct {
	data        []byte
	off         int
	coalesceEOF bool
}

func (r *sliceReader) Read(p []byte) (int, error) {
	if r.off >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.off:])
	r.off += n
	if r.coalesceEOF && r.off >= len(r.data) && len(p) > n {
		return n, io.EOF
	}
	return n, nil
}

func (r *sliceReader) Close() error { return nil }

func TestPayloadHashVerifier(t *testing.T) {
	body := []byte("the quick brown fox jumps over the lazy dog")
	goodDigest := sha256.Sum256(body)
	goodHex := hex.EncodeToString(goodDigest[:])
	badHex := sha256Sum([]byte("something else"))

	t.Run("matching digest reads cleanly to EOF", func(t *testing.T) {
		v := newPayloadHashVerifier(io.NopCloser(strings.NewReader(string(body))), goodHex)
		got, err := io.ReadAll(v)
		if err != nil {
			t.Fatalf("ReadAll: unexpected error %v", err)
		}
		if string(got) != string(body) {
			t.Fatalf("body corrupted: got %q", got)
		}
	})

	t.Run("mismatched digest fails the read with the typed error", func(t *testing.T) {
		v := newPayloadHashVerifier(io.NopCloser(strings.NewReader(string(body))), badHex)
		data, err := io.ReadAll(v)
		if err == nil {
			t.Fatal("ReadAll: expected mismatch error, got nil")
		}
		var mismatch *handlers.ContentSHA256MismatchError
		if !errors.As(err, &mismatch) {
			t.Fatalf("ReadAll: error %v (%T) is not *handlers.ContentSHA256MismatchError", err, err)
		}
		if mismatch.Expected != badHex || mismatch.Got != goodHex {
			t.Fatalf("mismatch digests: expected=%s got=%s, want expected=%s got=%s", mismatch.Expected, mismatch.Got, badHex, goodHex)
		}
		// The mismatch is only knowable once the whole body has been read, so
		// every byte read before the failure must still have been delivered.
		if string(data) != string(body) {
			t.Fatalf("pre-error bytes lost: got %q", data)
		}
		// The error is sticky on subsequent reads.
		if _, err := v.Read(make([]byte, 1)); !errors.As(err, &mismatch) {
			t.Fatalf("second Read: error %v is not the mismatch", err)
		}
	})

	t.Run("body returned together with EOF still verifies", func(t *testing.T) {
		v := newPayloadHashVerifier(&sliceReader{data: body, coalesceEOF: true}, goodHex)
		if _, err := io.ReadAll(v); err != nil {
			t.Fatalf("ReadAll: unexpected error %v", err)
		}
		v2 := newPayloadHashVerifier(&sliceReader{data: body, coalesceEOF: true}, badHex)
		if _, err := io.ReadAll(v2); !errors.As(err, new(*handlers.ContentSHA256MismatchError)) {
			t.Fatalf("ReadAll: error %v is not the mismatch", err)
		}
	})

	t.Run("underlying errors other than EOF pass through unchanged", func(t *testing.T) {
		sentinel := errors.New("connection reset")
		v := newPayloadHashVerifier(io.NopCloser(errReader{sentinel}), goodHex)
		if _, err := io.ReadAll(v); !errors.Is(err, sentinel) {
			t.Fatalf("ReadAll: error %v does not wrap the underlying failure", err)
		}
	})
}

type errReader struct{ err error }

func (r errReader) Read([]byte) (int, error) { return 0, r.err }
func (r errReader) Close() error             { return nil }
