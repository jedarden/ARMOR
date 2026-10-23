package backend

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// newDirectS3TestBackend builds a backend with no CF domain, so ranged reads
// go through a real S3 client pointed at the test server (armor-73564f97).
func newDirectS3TestBackend(t *testing.T, endpoint string, blockSize int64, concurrency int) *B2Backend {
	t.Helper()
	awsCfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
	)
	if err != nil {
		t.Fatalf("aws config: %v", err)
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})
	return &B2Backend{
		s3Client:        client,
		region:          "us-east-1",
		readBlockSize:   blockSize,
		readConcurrency: concurrency,
		readTimeout:     5 * time.Second,
	}
}

// directS3Handler answers every ranged GET with 206 and a Content-Range whose
// start and end are shifted by shift bytes — 0 means a correct response.
func directS3Handler(t *testing.T, object []byte, shift int, requests *atomic.Int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if requests != nil {
			requests.Add(1)
		}
		start, end := testRange(t, r)
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start+shift, end+shift, len(object)))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(object[start : end+1])
	}
}

// The direct-S3 path (no CF domain) meets the same one-request-per-window
// acceptance as the CF path: a 1 MiB span at a 1 MiB fetch size is one data
// request, not 16 (armor-73564f97).
func TestB2BackendDirectS3GetRangeDefaultFetchSizeIsOneRequestPerMiB(t *testing.T) {
	object := bytes.Repeat([]byte("0123456789abcdef"), 1<<20/16)
	var requests atomic.Int64
	server := httptest.NewServer(directS3Handler(t, object, 0, &requests))
	defer server.Close()

	b := newDirectS3TestBackend(t, server.URL, defaultReadBlockSize, defaultReadConcurrency)
	body, _, err := b.GetRangeWithHeaders(context.Background(), "bucket", "key", 0, int64(len(object)))
	if err != nil {
		t.Fatalf("GetRangeWithHeaders: %v", err)
	}
	defer body.Close()
	got, err := io.ReadAll(body)
	if err != nil || !bytes.Equal(got, object) {
		t.Fatalf("read = %d bytes, err %v; want the object", len(got), err)
	}
	if n := requests.Load(); n != 1 {
		t.Fatalf("1 MiB took %d data requests, want 1", n)
	}
}

// The pipelined window works end to end over direct S3: a multi-block span
// arrives in order with exactly one request per block.
func TestB2BackendDirectS3GetRangePipelinesInOrder(t *testing.T) {
	const blockSize = 8
	object := bytes.Repeat([]byte("01234567"), 12) // 12 blocks
	var requests atomic.Int64
	server := httptest.NewServer(directS3Handler(t, object, 0, &requests))
	defer server.Close()

	b := newDirectS3TestBackend(t, server.URL, blockSize, 4)
	body, _, err := b.GetRangeWithHeaders(context.Background(), "bucket", "key", 0, int64(len(object)))
	if err != nil {
		t.Fatalf("GetRangeWithHeaders: %v", err)
	}
	defer body.Close()
	got, err := io.ReadAll(body)
	if err != nil || !bytes.Equal(got, object) {
		t.Fatalf("read = %d bytes, err %v; want the object", len(got), err)
	}
	if n := requests.Load(); n != int64(len(object)/blockSize) {
		t.Fatalf("%d blocks took %d requests", len(object)/blockSize, n)
	}
}

// A 206 whose Content-Range names a different span than the one requested is
// rejected on the direct-S3 path too (armor-73564f97).
func TestB2BackendDirectS3GetRangeRejectsWrongContentRange(t *testing.T) {
	object := bytes.Repeat([]byte("x"), 64)
	server := httptest.NewServer(directS3Handler(t, object, 1, nil))
	defer server.Close()

	b := newDirectS3TestBackend(t, server.URL, 8, 2)
	body, _, err := b.GetRangeWithHeaders(context.Background(), "bucket", "key", 0, 8)
	if err == nil {
		body.Close()
		t.Fatal("wrong Content-Range was accepted")
	}
	if !strings.Contains(err.Error(), "Content-Range") {
		t.Fatalf("error = %v, want a Content-Range mismatch", err)
	}
}

// A server that ignores Range and answers a mid-object span with 200 and the
// whole object cannot silently succeed: the body is strictly longer than the
// requested span (armor-73564f97).
func TestB2BackendDirectS3GetRangeRejectsRangeIgnoringServer(t *testing.T) {
	object := bytes.Repeat([]byte("x"), 100)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(object) // whole object for bytes=50-57
	}))
	defer server.Close()

	b := newDirectS3TestBackend(t, server.URL, 8, 2)
	body, _, err := b.GetRangeWithHeaders(context.Background(), "bucket", "key", 50, 8)
	if err == nil {
		body.Close()
		t.Fatal("range-ignoring 200 was accepted for a mid-object span")
	}
	if !strings.Contains(err.Error(), "want 8") {
		t.Fatalf("error = %v, want a length mismatch", err)
	}
}

// A truncated 206 — correct Content-Range, short body — fails the read
// instead of silently yielding a short block (armor-73564f97).
func TestB2BackendDirectS3GetRangeRejectsTruncatedBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start, end := testRange(t, r)
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, end+1))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(bytes.Repeat([]byte("x"), int(end-start))) // one byte short
	}))
	defer server.Close()

	b := newDirectS3TestBackend(t, server.URL, 8, 2)
	body, _, err := b.GetRangeWithHeaders(context.Background(), "bucket", "key", 0, 8)
	if err == nil {
		body.Close()
		t.Fatal("truncated 206 was accepted")
	}
	if !strings.Contains(err.Error(), "want 8") {
		t.Fatalf("error = %v, want a length mismatch", err)
	}
}
