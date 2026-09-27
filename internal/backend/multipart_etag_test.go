package backend

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
)

// expectedCompositeETag computes the S3 composite from raw part payloads so
// the assertions below derive their oracle independently of
// ComputeCompositeETag.
func expectedCompositeETag(t *testing.T, parts ...[]byte) string {
	t.Helper()
	h := md5.New()
	for i, part := range parts {
		sum := md5.Sum(part)
		if len(part) == 0 {
			// md5.Sum(nil) already is the empty-input digest; spelled out so a
			// reader sees the zero-byte part is deliberately included.
			empty := md5.Sum([]byte{})
			if sum != empty {
				t.Fatalf("md5.Sum(nil) is not the empty-input digest for part %d", i+1)
			}
		}
		h.Write(sum[:])
	}
	return fmt.Sprintf("%x-%d", h.Sum(nil), len(parts))
}

func TestComputeCompositeETag(t *testing.T) {
	partA := []byte(strings.Repeat("a", 1024))
	partB := []byte(strings.Repeat("b", 512))
	partC := []byte(nil)

	etagA := hex.EncodeToString(mustMD5(t, partA))
	etagB := hex.EncodeToString(mustMD5(t, partB))
	etagC := hex.EncodeToString(mustMD5(t, partC))

	t.Run("matches the S3 md5-of-md5s form", func(t *testing.T) {
		got, err := ComputeCompositeETag([]string{etagA, etagB})
		if err != nil {
			t.Fatalf("ComputeCompositeETag: %v", err)
		}
		if want := expectedCompositeETag(t, partA, partB); got != want {
			t.Fatalf("composite = %q, want %q", got, want)
		}
	})

	t.Run("carries the part-count suffix rclone keys on", func(t *testing.T) {
		got, err := ComputeCompositeETag([]string{etagA, etagB, etagC})
		if err != nil {
			t.Fatalf("ComputeCompositeETag: %v", err)
		}
		if !strings.HasSuffix(got, "-3") {
			t.Fatalf("composite %q lacks the -3 part-count suffix", got)
		}
		body := strings.TrimSuffix(got, "-3")
		if len(body) != md5.Size*2 {
			t.Fatalf("composite %q body is not a 32-hex digest", got)
		}
		if len(got) == 32 {
			t.Fatalf("composite %q is a bare 32-hex digest — the exact shape rclone misreads as a content MD5", got)
		}
	})

	t.Run("zero-byte part contributes its digest and its count", func(t *testing.T) {
		got, err := ComputeCompositeETag([]string{etagA, etagC})
		if err != nil {
			t.Fatalf("ComputeCompositeETag: %v", err)
		}
		if want := expectedCompositeETag(t, partA, partC); got != want {
			t.Fatalf("composite with empty part = %q, want %q", got, want)
		}
	})

	t.Run("order matters", func(t *testing.T) {
		ab, err := ComputeCompositeETag([]string{etagA, etagB})
		if err != nil {
			t.Fatalf("ComputeCompositeETag: %v", err)
		}
		ba, err := ComputeCompositeETag([]string{etagB, etagA})
		if err != nil {
			t.Fatalf("ComputeCompositeETag: %v", err)
		}
		if ab == ba {
			t.Fatal("composite ignored part order")
		}
	})

	t.Run("rejects empty part list", func(t *testing.T) {
		if _, err := ComputeCompositeETag(nil); err == nil {
			t.Fatal("expected an error for an empty part list")
		}
	})

	t.Run("rejects malformed part digests", func(t *testing.T) {
		for name, bad := range map[string]string{
			"not hex":     "zzz",
			"short hex":   "abcd",
			"empty value": "",
		} {
			if _, err := ComputeCompositeETag([]string{etagA, bad}); err == nil {
				t.Fatalf("%s: expected an error for part digest %q", name, bad)
			}
		}
	})
}

func mustMD5(t *testing.T, b []byte) []byte {
	t.Helper()
	sum := md5.Sum(b)
	return sum[:]
}

// TestFSCompleteMultipartUpload_ETagIsS3Composite pins the completed-multipart
// ETag against the built-in storage to the S3 composite form: the previous
// bare md5-of-concatenated-ciphertext was read by rclone as a content MD5 and
// failed every download (armor-e8981148). It exercises the manifest-disabled
// read path too — Head serves the backend's own stored metadata — so the
// composite must survive there, not only in the completion response.
func TestFSCompleteMultipartUpload_ETagIsS3Composite(t *testing.T) {
	fs, err := NewFSBackend(FSConfig{BasePath: t.TempDir()})
	if err != nil {
		t.Fatalf("NewFSBackend: %v", err)
	}

	ctx := context.Background()
	bucket := "etag-composite"
	key := "dir/multipart.bin"
	if err := fs.CreateBucket(ctx, bucket); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	uploadID, err := fs.CreateMultipartUpload(ctx, bucket, key, map[string]string{"Content-Type": "application/octet-stream"})
	if err != nil {
		t.Fatalf("CreateMultipartUpload: %v", err)
	}

	// Three distinct parts plus a zero-byte final part, the shape aws-cli
	// emits; every digest must land in the composite.
	parts := [][]byte{
		[]byte(strings.Repeat("A", 512*1024)),
		[]byte(strings.Repeat("B", 512*1024)),
		[]byte(strings.Repeat("C", 512*1024)),
		{},
	}

	var completed []CompletedPart
	for i, data := range parts {
		partNumber := int32(i + 1)
		partETag, err := fs.UploadPart(ctx, bucket, key, uploadID, partNumber, strings.NewReader(string(data)), int64(len(data)))
		if err != nil {
			t.Fatalf("UploadPart %d: %v", partNumber, err)
		}
		completed = append(completed, CompletedPart{PartNumber: partNumber, ETag: partETag})
	}

	etag, err := fs.CompleteMultipartUpload(ctx, bucket, key, uploadID, completed)
	if err != nil {
		t.Fatalf("CompleteMultipartUpload: %v", err)
	}

	if want := expectedCompositeETag(t, parts...); etag != want {
		t.Fatalf("completed-multipart ETag = %q, want the S3 composite %q", etag, want)
	}
	if !strings.HasSuffix(etag, "-4") {
		t.Fatalf("ETag %q lacks the -4 part-count suffix", etag)
	}
	if len(etag) == 32 {
		t.Fatalf("ETag %q is bare 32-hex — rclone would read it as a content MD5", etag)
	}

	info, err := fs.Head(ctx, bucket, key)
	if err != nil {
		t.Fatalf("Head: %v", err)
	}
	if info.ETag != etag {
		t.Fatalf("Head ETag = %q, want the completion ETag %q", info.ETag, etag)
	}
}
