package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/canary"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/logging"
	"github.com/jedarden/armor/internal/metrics"
)

// countingBackend stores objects in memory (for canary round-trips) and
// counts HeadBucket invocations.
type countingBackend struct {
	headBucketCalls atomic.Int64
	mu              sync.Mutex
	failHeadBucket  bool
	objects         map[string][]byte
	meta            map[string]map[string]string
}

func newCountingBackend() *countingBackend {
	return &countingBackend{
		objects: make(map[string][]byte),
		meta:    make(map[string]map[string]string),
	}
}

func (b *countingBackend) HeadBucket(_ context.Context, _ string) error {
	b.headBucketCalls.Add(1)
	b.mu.Lock()
	fail := b.failHeadBucket
	b.mu.Unlock()
	if fail {
		return fmt.Errorf("simulated HeadBucket failure")
	}
	return nil
}

func (b *countingBackend) Put(_ context.Context, bucket, key string, body io.Reader, _ int64, meta map[string]string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	b.objects[bucket+"/"+key] = data
	m := make(map[string]string, len(meta))
	for k, v := range meta {
		m[k] = v
	}
	b.meta[bucket+"/"+key] = m
	return nil
}

func (b *countingBackend) Get(_ context.Context, bucket, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	k := bucket + "/" + key
	data, ok := b.objects[k]
	if !ok {
		return nil, nil, fmt.Errorf("object not found: %s", key)
	}
	return io.NopCloser(bytes.NewReader(data)), &backend.ObjectInfo{
		Key:      key,
		Size:     int64(len(data)),
		Metadata: b.meta[k],
	}, nil
}

func (b *countingBackend) GetRange(_ context.Context, bucket, key string, offset, length int64) (io.ReadCloser, error) {
	body, _, err := b.GetRangeWithHeaders(context.Background(), bucket, key, offset, length)
	return body, err
}

func (b *countingBackend) GetRangeWithHeaders(_ context.Context, bucket, key string, offset, length int64) (io.ReadCloser, map[string]string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	k := bucket + "/" + key
	data, ok := b.objects[k]
	if !ok {
		return nil, nil, fmt.Errorf("object not found: %s", key)
	}
	if offset >= int64(len(data)) {
		return nil, nil, fmt.Errorf("offset out of range")
	}
	end := offset + length
	if end > int64(len(data)) {
		end = int64(len(data))
	}
	return io.NopCloser(bytes.NewReader(data[offset:end])), make(map[string]string), nil
}

func (b *countingBackend) Head(_ context.Context, bucket, key string) (*backend.ObjectInfo, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	k := bucket + "/" + key
	data, ok := b.objects[k]
	if !ok {
		return nil, fmt.Errorf("object not found: %s", key)
	}
	return &backend.ObjectInfo{
		Key:      key,
		Size:     int64(len(data)),
		Metadata: b.meta[k],
	}, nil
}

func (b *countingBackend) Delete(_ context.Context, bucket, key string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	k := bucket + "/" + key
	delete(b.objects, k)
	delete(b.meta, k)
	return nil
}

func (b *countingBackend) DeleteObjects(_ context.Context, bucket string, keys []string) error {
	for _, key := range keys {
		b.Delete(context.Background(), bucket, key)
	}
	return nil
}

func (b *countingBackend) List(_ context.Context, bucket, prefix, _ string, _ string, _ int) (*backend.ListResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	p := bucket + "/" + prefix
	var objects []backend.ObjectInfo
	for k, data := range b.objects {
		if len(p) > 0 && (len(k) < len(p) || k[:len(p)] != p) {
			continue
		}
		objects = append(objects, backend.ObjectInfo{
			Key:      k[len(bucket)+1:],
			Size:     int64(len(data)),
			Metadata: b.meta[k],
		})
	}
	return &backend.ListResult{Objects: objects}, nil
}

func (b *countingBackend) Copy(_ context.Context, srcBucket, srcKey, dstBucket, dstKey string, meta map[string]string, _ bool) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	src := srcBucket + "/" + srcKey
	dst := dstBucket + "/" + dstKey
	data, ok := b.objects[src]
	if !ok {
		return fmt.Errorf("source not found: %s", srcKey)
	}
	b.objects[dst] = data
	m := make(map[string]string, len(meta))
	for k, v := range meta {
		m[k] = v
	}
	b.meta[dst] = m
	return nil
}

func (b *countingBackend) ListBuckets(_ context.Context) ([]backend.BucketInfo, error)        { return nil, nil }
func (b *countingBackend) CreateBucket(_ context.Context, _ string) error                      { return nil }
func (b *countingBackend) DeleteBucket(_ context.Context, _ string) error                      { return nil }
func (b *countingBackend) GetDirect(_ context.Context, _, _ string) (io.ReadCloser, *backend.ObjectInfo, error) {
	return nil, nil, fmt.Errorf("not implemented")
}
func (b *countingBackend) CreateMultipartUpload(_ context.Context, _, _ string, _ map[string]string) (string, error) {
	return "", nil
}
func (b *countingBackend) UploadPart(_ context.Context, _, _, _ string, _ int32, _ io.Reader, _ int64) (string, error) {
	return "", nil
}
func (b *countingBackend) CompleteMultipartUpload(_ context.Context, _, _, _ string, _ []backend.CompletedPart) (string, error) {
	return "", nil
}
func (b *countingBackend) AbortMultipartUpload(_ context.Context, _, _, _ string) error { return nil }
func (b *countingBackend) ListParts(_ context.Context, _, _, _ string) (*backend.ListPartsResult, error) {
	return &backend.ListPartsResult{}, nil
}
func (b *countingBackend) ListMultipartUploads(_ context.Context, _ string) (*backend.ListMultipartUploadsResult, error) {
	return &backend.ListMultipartUploadsResult{}, nil
}
func (b *countingBackend) GetBucketLifecycleConfiguration(_ context.Context, _ string) ([]byte, error) {
	return nil, fmt.Errorf("not found")
}
func (b *countingBackend) PutBucketLifecycleConfiguration(_ context.Context, _ string, _ []byte) error {
	return nil
}
func (b *countingBackend) DeleteBucketLifecycleConfiguration(_ context.Context, _ string) error {
	return nil
}
func (b *countingBackend) GetObjectLockConfiguration(_ context.Context, _ string) ([]byte, error) {
	return nil, fmt.Errorf("not found")
}
func (b *countingBackend) PutObjectLockConfiguration(_ context.Context, _ string, _ []byte) error {
	return nil
}
func (b *countingBackend) GetObjectRetention(_ context.Context, _, _ string) ([]byte, error) {
	return nil, fmt.Errorf("not found")
}
func (b *countingBackend) PutObjectRetention(_ context.Context, _, _ string, _ []byte) error {
	return nil
}
func (b *countingBackend) GetObjectLegalHold(_ context.Context, _, _ string) ([]byte, error) {
	return nil, fmt.Errorf("not found")
}
func (b *countingBackend) PutObjectLegalHold(_ context.Context, _, _ string, _ []byte) error {
	return nil
}
func (b *countingBackend) ListObjectVersions(_ context.Context, _, _, _, _, _ string, _ int) (*backend.ListObjectVersionsResult, error) {
	return nil, fmt.Errorf("not implemented")
}
func (b *countingBackend) HeadVersion(_ context.Context, _, _, _ string) (*backend.ObjectInfo, error) {
	return nil, fmt.Errorf("not implemented")
}

// TestReadyzConcurrentCachedHead verifies that 100 concurrent GETs to /readyz
// over a 5-second window issue at most ceil(5s/TTL) backend HeadBucket calls.
func TestReadyzConcurrentCachedHead(t *testing.T) {
	const cacheTTL = 1 // 1 second
	const numRequests = 100

	cb := newCountingBackend()
	s := &Server{
		config: &config.Config{
			Bucket:         "test-bucket",
			ReadyzCacheTTL: cacheTTL,
		},
		backend: cb,
		// No canary — forces the cached HeadBucket fallback path.
		logger:  logging.New("test"),
		metrics: metrics.DefaultMetrics,
	}

	var wg sync.WaitGroup
	wg.Add(numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
			rec := httptest.NewRecorder()
			s.readyz(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("expected 200, got %d", rec.Code)
			}
		}()
		// Spread requests over ~5 seconds.
		time.Sleep(50 * time.Millisecond)
	}
	wg.Wait()

	calls := cb.headBucketCalls.Load()
	// ceil(5s / 1s TTL) = 5. Allow generous margin for scheduling jitter.
	maxAllowed := int64(10)
	if calls > maxAllowed {
		t.Errorf("expected ≤ %d HeadBucket calls, got %d", maxAllowed, calls)
	}
	t.Logf("HeadBucket calls: %d (limit %d)", calls, maxAllowed)
}

// TestReadyzConcurrentCanaryMode verifies that 100 concurrent GETs to /readyz
// with a healthy canary make zero HeadBucket calls — the canary's in-memory
// state is the sole signal.
func TestReadyzConcurrentCanaryMode(t *testing.T) {
	const numRequests = 100

	cb := newCountingBackend()

	mek := make([]byte, 32)
	rand.Read(mek)
	m := canary.NewMonitor(canary.Config{
		Backend:    cb,
		Bucket:     "test-bucket",
		MEK:        mek,
		BlockSize:  65536,
		InstanceID: "test-instance",
		CanarySize: 100,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	m.Start(ctx)

	deadline := time.Now().Add(5 * time.Second)
	for !m.IsHealthy() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for canary to become healthy")
		}
		time.Sleep(50 * time.Millisecond)
	}

	s := &Server{
		config:        &config.Config{Bucket: "test-bucket"},
		backend:       cb,
		canary:        m,
		canaryStarted: true,
		logger:        logging.New("test"),
		metrics:       metrics.DefaultMetrics,
	}

	var wg sync.WaitGroup
	wg.Add(numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
			rec := httptest.NewRecorder()
			s.readyz(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("expected 200, got %d", rec.Code)
			}
		}()
	}
	wg.Wait()

	calls := cb.headBucketCalls.Load()
	if calls > 1 {
		t.Errorf("canary mode: expected ≤ 1 HeadBucket calls, got %d", calls)
	}
	t.Logf("Canary mode: HeadBucket calls: %d", calls)

	m.Stop()
}

// TestReadyzCanaryUnhealthy verifies that when the canary reports unhealthy,
// /readyz returns 503 without making any backend HeadBucket call.
func TestReadyzCanaryUnhealthy(t *testing.T) {
	cb := newCountingBackend()

	// NewMonitor defaults to StatusUnknown (not healthy).
	mek := make([]byte, 32)
	rand.Read(mek)
	m := canary.NewMonitor(canary.Config{
		Backend:    cb,
		Bucket:     "test-bucket",
		MEK:        mek,
		BlockSize:  65536,
		InstanceID: "test-instance",
	})

	s := &Server{
		config:        &config.Config{Bucket: "test-bucket"},
		backend:       cb,
		canary:        m,
		canaryStarted: true,
		logger:        logging.New("test"),
		metrics:       metrics.DefaultMetrics,
	}

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	s.readyz(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}
	if calls := cb.headBucketCalls.Load(); calls != 0 {
		t.Errorf("expected 0 HeadBucket calls with unhealthy canary, got %d", calls)
	}
}

// TestReadyzCanaryHealthy verifies that when the canary is healthy,
// /readyz returns 200 without any backend HeadBucket call.
func TestReadyzCanaryHealthy(t *testing.T) {
	cb := newCountingBackend()

	mek := make([]byte, 32)
	rand.Read(mek)
	m := canary.NewMonitor(canary.Config{
		Backend:    cb,
		Bucket:     "test-bucket",
		MEK:        mek,
		BlockSize:  65536,
		InstanceID: "test-instance",
		CanarySize: 100,
	})

	// Start the canary so its initial check runs (encrypt-then-decrypt round trip
	// against the in-memory backend), then wait for it to report healthy.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	m.Start(ctx)

	deadline := time.Now().Add(5 * time.Second)
	for !m.IsHealthy() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for canary to become healthy")
		}
		time.Sleep(50 * time.Millisecond)
	}
	m.Stop()

	s := &Server{
		config:        &config.Config{Bucket: "test-bucket"},
		backend:       cb,
		canary:        m,
		canaryStarted: true,
		logger:        logging.New("test"),
		metrics:       metrics.DefaultMetrics,
	}

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	start := time.Now()
	s.readyz(rec, req)
	elapsed := time.Since(start)

	// The canary path reads an in-memory boolean — should respond in well
	// under the handler's 5-second backend timeout.
	if elapsed > time.Second {
		t.Errorf("readyz took %v with healthy canary; expected < 1s", elapsed)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if body := rec.Body.String(); body != "Ready" {
		t.Errorf("expected body %q, got %q", "Ready", body)
	}
	if calls := cb.headBucketCalls.Load(); calls != 0 {
		t.Errorf("expected 0 HeadBucket calls with healthy canary, got %d", calls)
	}
}
