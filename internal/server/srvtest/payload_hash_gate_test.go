package srvtest

// HTTP-level tests for the x-amz-content-sha256 payload-hash gate
// (armor-328896ea), driven through the real authenticated mux: a body that
// does not hash to its signed digest must fail the PUT with 400
// XAmzContentSHA256Mismatch and leave no object behind, an unsupported
// x-amz-content-sha256 mode must be rejected before the handler runs, and the
// two unsigned/streaming modes ARMOR does honour must keep working.

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// doPayloadHashRequest issues a request signed with an explicit
// x-amz-content-sha256 value against the harness mux.
func doPayloadHashRequest(h *Harness, method, target string, body []byte, payloadHash string) *httptest.ResponseRecorder {
	h.t.Helper()
	req := httptest.NewRequest(method, target, bytes.NewReader(body))
	SignS3RequestWithPayloadHash(req, payloadHash, TestAccessKey, TestSecretKey, TestRegion)
	rec := httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, req)
	return rec
}

type s3ErrorBody struct {
	Code    string `xml:"Code"`
	Message string `xml:"Message"`
}

func parseS3Error(h *Harness, rec *httptest.ResponseRecorder) s3ErrorBody {
	h.t.Helper()
	var parsed s3ErrorBody
	if err := xml.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		h.t.Fatalf("parse error response %q: %v", rec.Body.String(), err)
	}
	return parsed
}

// objectAbsent asserts nothing is stored under the client-visible key.
func objectAbsent(h *Harness, key string) {
	h.t.Helper()
	body, _, err := h.Primary.Get(context.Background(), h.Bucket, h.StoredKey(key))
	if err == nil {
		body.Close()
		h.t.Fatalf("object %q was stored despite the failed upload", key)
	}
}

// signedGET reads an object through the real mux and returns its plaintext.
func signedGET(h *Harness, key string) []byte {
	h.t.Helper()
	rec := h.Do(http.MethodGet, "/"+h.Bucket+"/"+key, nil)
	if rec.Code != 200 {
		h.t.Fatalf("GET %s: status = %d (body %s)", key, rec.Code, rec.Body.String())
	}
	return rec.Body.Bytes()
}

func TestPayloadHashGateMismatchFailsBeforeObjectStored(t *testing.T) {
	h := New(t)
	key := "gate/mismatched.bin"

	original := []byte("original bytes a client signed and sent")
	tampered := []byte("tampered bytes replayed inside the skew window")
	originalDigest := sha256Hex(original)

	rec := doPayloadHashRequest(h, "PUT", "/"+h.Bucket+"/"+key, tampered, originalDigest)
	if rec.Code != 400 {
		t.Fatalf("mismatched PUT: status = %d, want 400 (body %s)", rec.Code, rec.Body.String())
	}
	parsed := parseS3Error(h, rec)
	if parsed.Code != "XAmzContentSHA256Mismatch" {
		t.Fatalf("mismatched PUT: error code = %q, want XAmzContentSHA256Mismatch (body %s)", parsed.Code, rec.Body.String())
	}
	if !strings.Contains(parsed.Message, originalDigest) || !strings.Contains(parsed.Message, sha256Hex(tampered)) {
		t.Errorf("mismatch message should name both digests, got %q", parsed.Message)
	}
	objectAbsent(h, key)

	// The same signed request with its real body still succeeds, proving the
	// failure is the digest mismatch and not the request shape.
	rec = doPayloadHashRequest(h, "PUT", "/"+h.Bucket+"/"+key, original, originalDigest)
	if rec.Code != 200 {
		t.Fatalf("matching PUT: status = %d (body %s)", rec.Code, rec.Body.String())
	}
	got := signedGET(h, key)
	if string(got) != string(original) {
		t.Fatalf("read back %q, want %q", got, original)
	}
}

func TestPayloadHashGateUnsignedPayloadAccepted(t *testing.T) {
	h := New(t)
	key := "gate/unsigned.bin"
	body := []byte("uploaded under UNSIGNED-PAYLOAD")

	rec := doPayloadHashRequest(h, "PUT", "/"+h.Bucket+"/"+key, body, "UNSIGNED-PAYLOAD")
	if rec.Code != 200 {
		t.Fatalf("UNSIGNED-PAYLOAD PUT: status = %d (body %s)", rec.Code, rec.Body.String())
	}
	got := signedGET(h, key)
	if string(got) != string(body) {
		t.Fatalf("read back %q, want %q", got, body)
	}
}

func TestPayloadHashGateUnknownStreamingModeRejected(t *testing.T) {
	h := New(t)
	key := "gate/unknown-streaming.bin"

	rec := doPayloadHashRequest(h, "PUT", "/"+h.Bucket+"/"+key,
		[]byte("framing the client signed in a mode ARMOR cannot honour"),
		"STREAMING-UNSIGNED-PAYLOAD-TRAILER")
	if rec.Code != 400 {
		t.Fatalf("unknown streaming mode: status = %d, want 400 (body %s)", rec.Code, rec.Body.String())
	}
	parsed := parseS3Error(h, rec)
	if parsed.Code != "InvalidArgument" {
		t.Fatalf("unknown streaming mode: error code = %q, want InvalidArgument (body %s)", parsed.Code, rec.Body.String())
	}
	for _, mode := range []string{"STREAMING-AWS4-HMAC-SHA256-PAYLOAD", "UNSIGNED-PAYLOAD"} {
		if !strings.Contains(parsed.Message, mode) {
			t.Errorf("rejection message should name supported mode %s, got %q", mode, parsed.Message)
		}
	}
	objectAbsent(h, key)
}

func TestPayloadHashGateStreamingModeStillDecodes(t *testing.T) {
	h := New(t)
	key := "gate/streaming.bin"
	payload := []byte("payload carried in aws-chunked framing")

	rec := doPayloadHashRequest(h, "PUT", "/"+h.Bucket+"/"+key, encodeChunkedForGateTest(t, payload), "STREAMING-AWS4-HMAC-SHA256-PAYLOAD")
	if rec.Code != 200 {
		t.Fatalf("streaming PUT: status = %d (body %s)", rec.Code, rec.Body.String())
	}
	got := signedGET(h, key)
	if string(got) != string(payload) {
		t.Fatalf("decoded payload %q, want %q", got, payload)
	}
}

// TestPayloadHashGateMismatchStreamingPath covers the temp-file streaming PUT
// path (bodies over the 10MB buffering threshold): the mismatch has to fail
// the read before the streaming upload reaches the backend.
func TestPayloadHashGateMismatchStreamingPath(t *testing.T) {
	h := New(t)
	key := "gate/streaming-mismatch.bin"

	original := bytes.Repeat([]byte("A"), 11*1024*1024)
	tampered := append([]byte("B"), bytes.Repeat([]byte("A"), 11*1024*1024-1)...)

	rec := doPayloadHashRequest(h, "PUT", "/"+h.Bucket+"/"+key, tampered, sha256Hex(original))
	if rec.Code != 400 {
		t.Fatalf("streaming mismatched PUT: status = %d, want 400 (body %s)", rec.Code, rec.Body.String())
	}
	if code := parseS3Error(h, rec).Code; code != "XAmzContentSHA256Mismatch" {
		t.Fatalf("streaming mismatched PUT: error code = %q, want XAmzContentSHA256Mismatch", code)
	}
	objectAbsent(h, key)
}

// encodeChunkedForGateTest wraps payload in the aws-chunked wire format,
// including the X-Amz-Decoded-Content-Length header the server reads.
func encodeChunkedForGateTest(t *testing.T, payload []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	sig := strings.Repeat("ab", 32) // per-chunk signatures are not re-verified
	chunkSize := 8
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
