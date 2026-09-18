// Package performance holds ARMOR's throughput benchmark harness: an
// in-process service environment (real S3 handlers over an instrumented
// filesystem backend), the bounded scenario matrix, deterministic
// request-count/overlap checks, and env-gated remote targets (real ARMOR
// service, Cloudflare, direct B2) documented in docs/performance/README.md.
//
// Nothing here runs heavy measurement in ordinary CI: the deterministic tests
// are -short-safe, and the measurement entry points are gated on
// ARMOR_PERF_RUN=1 (local matrix) or the ARMOR_PERF_* remote-target variables.
package performance

import (
	"context"
	"io"
	"sync/atomic"
	"time"

	"github.com/jedarden/armor/internal/backend"
)

// backendOp names are the counter keys reported in results.
const (
	opGet            = "get"
	opGetRange       = "get_range"
	opHead           = "head"
	opPut            = "put"
	opCreateMultipart = "create_multipart"
	opUploadPart     = "upload_part"
	opCompleteMultipart = "complete_multipart"
	opDelete         = "delete"
	opList           = "list"
)

// CountingBackend wraps a backend.Backend, counts requests per operation and
// bytes served, tracks the observed maximum in-flight request concurrency,
// and optionally injects a fixed per-call latency. The latency knob is what
// makes the request-count and overlap checks in harness_test.go
// deterministic: they assert on call counts and observed concurrency, never
// on wall-clock speed.
type CountingBackend struct {
	inner backend.Backend

	// Delay is injected before every backend call completes its count.
	Delay time.Duration

	counts map[string]*atomic.Int64
	bytes  map[string]*atomic.Int64

	inflight    atomic.Int64
	maxInflight atomic.Int64
}

// NewCountingBackend wraps inner with instrumentation.
func NewCountingBackend(inner backend.Backend) *CountingBackend {
	cb := &CountingBackend{inner: inner}
	cb.counts = map[string]*atomic.Int64{}
	cb.bytes = map[string]*atomic.Int64{}
	for _, op := range []string{opGet, opGetRange, opHead, opPut, opCreateMultipart, opUploadPart, opCompleteMultipart, opDelete, opList} {
		cb.counts[op] = &atomic.Int64{}
		cb.bytes[op] = &atomic.Int64{}
	}
	return cb
}

// Snapshot returns a copy of the per-op request counts and bytes served.
func (cb *CountingBackend) Snapshot() (counts, bytes map[string]int64) {
	counts = make(map[string]int64, len(cb.counts))
	bytes = make(map[string]int64, len(cb.counts))
	for op, c := range cb.counts {
		counts[op] = c.Load()
		bytes[op] = cb.bytes[op].Load()
	}
	return counts, bytes
}

// MaxInflight returns the highest number of backend calls observed
// concurrently since construction (backend-side concurrency).
func (cb *CountingBackend) MaxInflight() int64 { return cb.maxInflight.Load() }

// withInflight accounts one call of op carrying n bytes and keeps the
// in-flight window — and the injected latency — open for the duration of fn,
// so concurrent callers genuinely overlap inside the window.
func (cb *CountingBackend) withInflight(op string, n int64, fn func()) {
	cb.counts[op].Add(1)
	cb.bytes[op].Add(n)
	cur := cb.inflight.Add(1)
	for {
		max := cb.maxInflight.Load()
		if cur <= max || cb.maxInflight.CompareAndSwap(max, cur) {
			break
		}
	}
	defer cb.inflight.Add(-1)
	if cb.Delay > 0 {
		time.Sleep(cb.Delay)
	}
	if fn != nil {
		fn()
	}
}

func (cb *CountingBackend) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error {
	var err error
	cb.withInflight(opPut, size, func() {
		err = cb.inner.Put(ctx, bucket, key, body, size, meta)
	})
	return err
}

func (cb *CountingBackend) Get(ctx context.Context, bucket, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	var (
		rc   io.ReadCloser
		info *backend.ObjectInfo
		err  error
	)
	cb.withInflight(opGet, 0, func() {
		rc, info, err = cb.inner.Get(ctx, bucket, key)
		if err == nil && info != nil {
			cb.bytes[opGet].Add(info.StoredSize)
		}
	})
	return rc, info, err
}

func (cb *CountingBackend) GetRange(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, error) {
	var (
		rc  io.ReadCloser
		err error
	)
	cb.withInflight(opGetRange, length, func() {
		rc, err = cb.inner.GetRange(ctx, bucket, key, offset, length)
	})
	return rc, err
}

func (cb *CountingBackend) GetRangeWithHeaders(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, map[string]string, error) {
	var (
		rc      io.ReadCloser
		headers map[string]string
		err     error
	)
	cb.withInflight(opGetRange, length, func() {
		rc, headers, err = cb.inner.GetRangeWithHeaders(ctx, bucket, key, offset, length)
	})
	return rc, headers, err
}

func (cb *CountingBackend) Head(ctx context.Context, bucket, key string) (*backend.ObjectInfo, error) {
	var (
		info *backend.ObjectInfo
		err  error
	)
	cb.withInflight(opHead, 0, func() {
		info, err = cb.inner.Head(ctx, bucket, key)
	})
	return info, err
}

func (cb *CountingBackend) Delete(ctx context.Context, bucket, key string) error {
	var err error
	cb.withInflight(opDelete, 0, func() {
		err = cb.inner.Delete(ctx, bucket, key)
	})
	return err
}

func (cb *CountingBackend) DeleteObjects(ctx context.Context, bucket string, keys []string) error {
	var err error
	cb.withInflight(opDelete, int64(len(keys)), func() {
		err = cb.inner.DeleteObjects(ctx, bucket, keys)
	})
	return err
}

func (cb *CountingBackend) List(ctx context.Context, bucket, prefix, delimiter, token string, maxKeys int) (*backend.ListResult, error) {
	var (
		res *backend.ListResult
		err error
	)
	cb.withInflight(opList, 0, func() {
		res, err = cb.inner.List(ctx, bucket, prefix, delimiter, token, maxKeys)
	})
	return res, err
}

func (cb *CountingBackend) ListRaw(ctx context.Context, bucket, prefix, delimiter, token string, maxKeys int) (*backend.ListResult, error) {
	var (
		res *backend.ListResult
		err error
	)
	cb.withInflight(opList, 0, func() {
		res, err = cb.inner.ListRaw(ctx, bucket, prefix, delimiter, token, maxKeys)
	})
	return res, err
}

func (cb *CountingBackend) Copy(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string, meta map[string]string, replaceMetadata bool) error {
	return cb.inner.Copy(ctx, srcBucket, srcKey, dstBucket, dstKey, meta, replaceMetadata)
}

func (cb *CountingBackend) ListBuckets(ctx context.Context) ([]backend.BucketInfo, error) {
	return cb.inner.ListBuckets(ctx)
}

func (cb *CountingBackend) CreateBucket(ctx context.Context, bucket string) error {
	return cb.inner.CreateBucket(ctx, bucket)
}

func (cb *CountingBackend) DeleteBucket(ctx context.Context, bucket string) error {
	return cb.inner.DeleteBucket(ctx, bucket)
}

func (cb *CountingBackend) HeadBucket(ctx context.Context, bucket string) error {
	return cb.inner.HeadBucket(ctx, bucket)
}

func (cb *CountingBackend) GetDirect(ctx context.Context, bucket, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	return cb.inner.GetDirect(ctx, bucket, key)
}

func (cb *CountingBackend) CreateMultipartUpload(ctx context.Context, bucket, key string, meta map[string]string) (string, error) {
	var (
		id  string
		err error
	)
	cb.withInflight(opCreateMultipart, 0, func() {
		id, err = cb.inner.CreateMultipartUpload(ctx, bucket, key, meta)
	})
	return id, err
}

func (cb *CountingBackend) UploadPart(ctx context.Context, bucket, key, uploadID string, partNumber int32, body io.Reader, size int64) (string, error) {
	cb.withInflight(opUploadPart, size, nil)
	return cb.inner.UploadPart(ctx, bucket, key, uploadID, partNumber, body, size)
}

func (cb *CountingBackend) CompleteMultipartUpload(ctx context.Context, bucket, key, uploadID string, parts []backend.CompletedPart) (string, error) {
	var (
		etag string
		err  error
	)
	cb.withInflight(opCompleteMultipart, 0, func() {
		etag, err = cb.inner.CompleteMultipartUpload(ctx, bucket, key, uploadID, parts)
	})
	return etag, err
}

func (cb *CountingBackend) AbortMultipartUpload(ctx context.Context, bucket, key, uploadID string) error {
	return cb.inner.AbortMultipartUpload(ctx, bucket, key, uploadID)
}

func (cb *CountingBackend) ListParts(ctx context.Context, bucket, key, uploadID string) (*backend.ListPartsResult, error) {
	return cb.inner.ListParts(ctx, bucket, key, uploadID)
}

func (cb *CountingBackend) ListMultipartUploads(ctx context.Context, bucket, prefix string) (*backend.ListMultipartUploadsResult, error) {
	return cb.inner.ListMultipartUploads(ctx, bucket, prefix)
}

func (cb *CountingBackend) GetBucketLifecycleConfiguration(ctx context.Context, bucket string) ([]byte, error) {
	return cb.inner.GetBucketLifecycleConfiguration(ctx, bucket)
}

func (cb *CountingBackend) PutBucketLifecycleConfiguration(ctx context.Context, bucket string, config []byte) error {
	return cb.inner.PutBucketLifecycleConfiguration(ctx, bucket, config)
}

func (cb *CountingBackend) DeleteBucketLifecycleConfiguration(ctx context.Context, bucket string) error {
	return cb.inner.DeleteBucketLifecycleConfiguration(ctx, bucket)
}

func (cb *CountingBackend) GetObjectLockConfiguration(ctx context.Context, bucket string) ([]byte, error) {
	return cb.inner.GetObjectLockConfiguration(ctx, bucket)
}

func (cb *CountingBackend) PutObjectLockConfiguration(ctx context.Context, bucket string, config []byte) error {
	return cb.inner.PutObjectLockConfiguration(ctx, bucket, config)
}

func (cb *CountingBackend) GetObjectRetention(ctx context.Context, bucket, key string) ([]byte, error) {
	return cb.inner.GetObjectRetention(ctx, bucket, key)
}

func (cb *CountingBackend) PutObjectRetention(ctx context.Context, bucket, key string, retention []byte) error {
	return cb.inner.PutObjectRetention(ctx, bucket, key, retention)
}

func (cb *CountingBackend) GetObjectLegalHold(ctx context.Context, bucket, key string) ([]byte, error) {
	return cb.inner.GetObjectLegalHold(ctx, bucket, key)
}

func (cb *CountingBackend) PutObjectLegalHold(ctx context.Context, bucket, key string, legalHold []byte) error {
	return cb.inner.PutObjectLegalHold(ctx, bucket, key, legalHold)
}

func (cb *CountingBackend) ListObjectVersions(ctx context.Context, bucket, prefix, delimiter, keyMarker, versionIDMarker string, maxKeys int) (*backend.ListObjectVersionsResult, error) {
	return cb.inner.ListObjectVersions(ctx, bucket, prefix, delimiter, keyMarker, versionIDMarker, maxKeys)
}

func (cb *CountingBackend) HeadVersion(ctx context.Context, bucket, key, versionID string) (*backend.ObjectInfo, error) {
	return cb.inner.HeadVersion(ctx, bucket, key, versionID)
}
