package backend

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	awshttp "github.com/aws/smithy-go/transport/http"

	"github.com/jedarden/armor/internal/metrics"
)

// IsNoSuchUpload reports whether an error ultimately came from the S3
// NoSuchUpload response. CompleteMultipartUpload can have an ambiguous result:
// the object store may finish assembling the object after the proxy request
// times out, so a retry sees NoSuchUpload even though the object now exists.
func IsNoSuchUpload(err error) bool {
	var apiErr smithy.APIError
	return errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchUpload"
}

// B2Backend implements the Backend interface using B2's S3 API.
type B2Backend struct {
	s3Client        *s3.Client
	region          string
	endpoint        string
	cfDomain        string
	httpClient      *http.Client
	readBlockSize   int64 // network fetch size; independent of the stored ARMOR block size
	readConcurrency int
	readTimeout     time.Duration // per-request deadline; zero uses the default
	readBudget      chan struct{} // backend-wide cap on simultaneous ranged requests; nil = unlimited

	readRetryAttempts   int              // total tries per idempotent ranged GET/HEAD
	readRetryBackoff    time.Duration    // initial retry sleep; doubles with jitter
	readRetryMaxBackoff time.Duration    // cap on one retry sleep
	metrics             *metrics.Metrics // nil disables armor_backend_retries_total

	cfStaleFallback bool
	cfVersionPinned bool
	keyPrefix       string
	conditionalMu   sync.Mutex
	conditionalKeys map[string]*conditionalKeyLock
}

type conditionalKeyLock struct {
	mu   sync.Mutex
	refs int
}

const (
	// defaultReadBlockSize is the network fetch size, not the stored ARMOR
	// block size. Fetching 1 MiB per request amortises the ~75 ms cross-country
	// RTT over 16 crypto blocks (armor-73564f97).
	defaultReadBlockSize    = 1 << 20
	defaultReadConcurrency  = 16
	defaultReadTimeout      = 2 * time.Minute
	defaultReadBackendLimit = 64

	// Read-retry defaults (armor-f1791426): transient Cloudflare/B2 failures —
	// 5xx (CF 520-527 included), 429, connection resets, responses truncated
	// before the promised length — retry with bounded jittered backoff inside
	// the per-request deadline instead of failing a whole pipelined read over
	// one block.
	defaultReadRetryAttempts   = 3
	defaultReadRetryBackoff    = 100 * time.Millisecond
	defaultReadRetryMaxBackoff = 2 * time.Second
)

// B2Config contains configuration for the B2 backend.
type B2Config struct {
	Region          string
	Endpoint        string
	AccessKeyID     string
	SecretKey       string
	CFDomain        string // Cloudflare domain for free egress downloads
	ReadConcurrency int    // Maximum concurrent ranged reads; zero uses the default

	// ReadFetchSize is the size of each backend ranged request in bytes. It is
	// a network knob only and never changes the stored block size. Zero uses
	// the default.
	ReadFetchSize int64

	// ReadRequestTimeout bounds each backend ranged request so one wedged
	// request cannot hold an ordered read open forever. Zero uses the default.
	ReadRequestTimeout time.Duration

	// ReadBackendLimit caps simultaneous ranged requests across all readers of
	// this backend. Zero uses the default.
	ReadBackendLimit int

	// ReadRetryAttempts is the total number of tries for one idempotent ranged
	// GET or HEAD when the backend answers with a transient failure. Only
	// pre-delivery fetches retry: a block whose bytes have reached the client
	// is never re-fetched. Zero uses the default.
	ReadRetryAttempts int

	// ReadRetryBackoff is the initial sleep between read-retry attempts; it
	// doubles each attempt with full jitter up to ReadRetryMaxBackoff. The
	// whole sequence of a ranged GET stays inside ReadRequestTimeout. Zero
	// uses the default.
	ReadRetryBackoff time.Duration

	// ReadRetryMaxBackoff caps one read-retry backoff sleep. Zero uses the
	// default.
	ReadRetryMaxBackoff time.Duration

	// Metrics receives backend read-retry counters; nil disables recording.
	Metrics *metrics.Metrics

	// CFStaleFallback re-fetches a ranged read directly from B2, by version ID,
	// when Cloudflare answers with a version other than the one the request's
	// HEAD pinned (see VersionPin). It only changes reads that would otherwise
	// fail a block check.
	CFStaleFallback bool

	// CFVersionPinned reads through Cloudflare by B2 file ID instead of by key
	// whenever the request has a pinned version, so each version has its own
	// immutable URL and a stale edge copy cannot answer for a newer version.
	CFVersionPinned bool

	// KeyPrefix is the ADR-001 shared-bucket prefix (normalized with a trailing
	// slash) that the server prepends to every object key. The backend does not
	// apply it — callers pass already-prefixed keys — but needs it to locate the
	// reserved internal namespace, which sits at <KeyPrefix>.armor/ rather than
	// .armor/ when a prefix is in force. Empty means no prefix. Callers that
	// build a backend without access to the server config can leave it unset;
	// internal objects then only ever resolve at the bucket root.
	//
	// Unlike FSBackend there is no directory here to prune, so this field is the
	// only thing standing between a prefixed bucket's internal objects and a
	// client listing: leaving it unset while a prefix is in force leaks
	// <prefix>.armor/* into every List and ListObjectVersions.
	KeyPrefix string
}

type rangedBlockJob struct {
	index  int64
	offset int64
	length int64
}

type rangedBlockResult struct {
	index int64
	data  []byte
}

// NewB2Backend creates a new B2 backend.
func NewB2Backend(ctx context.Context, cfg B2Config) (*B2Backend, error) {
	readConcurrency := cfg.ReadConcurrency
	if readConcurrency <= 0 {
		readConcurrency = defaultReadConcurrency
	}

	fetchSize := cfg.ReadFetchSize
	if fetchSize <= 0 {
		fetchSize = defaultReadBlockSize
	}
	readTimeout := cfg.ReadRequestTimeout
	if readTimeout <= 0 {
		readTimeout = defaultReadTimeout
	}
	backendLimit := cfg.ReadBackendLimit
	if backendLimit <= 0 {
		backendLimit = defaultReadBackendLimit
	}
	retryAttempts := cfg.ReadRetryAttempts
	if retryAttempts <= 0 {
		retryAttempts = defaultReadRetryAttempts
	}
	retryBackoff := cfg.ReadRetryBackoff
	if retryBackoff < 0 {
		retryBackoff = defaultReadRetryBackoff
	}
	retryMaxBackoff := cfg.ReadRetryMaxBackoff
	if retryMaxBackoff < 0 {
		retryMaxBackoff = defaultReadRetryMaxBackoff
	}

	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretKey,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		o.UsePathStyle = true // B2 requires path-style URLs
		// SDK >= s3/v1.73 sends CRC32 request checksums by default; B2 then
		// records checksum-typed multipart parts and rejects a
		// CompleteMultipartUpload that supplies only ETags with InvalidPart.
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
		// Idle-pool sizing sized to this backend's own request budget; see
		// transport.go for the measurement rationale.
		o.HTTPClient = newS3HTTPClient(readConcurrency, backendLimit)
	})

	return &B2Backend{
		s3Client: s3Client,
		region:   cfg.Region,
		endpoint: cfg.Endpoint,
		cfDomain: cfg.CFDomain,
		httpClient: &http.Client{
			// Same pool sizing as the SDK client; the 30m ceiling is unchanged
			// and in practice shadowed by the per-request read deadline that
			// every caller of the CF client applies (fetchRange).
			Transport: newReadHTTPTransport(readConcurrency, backendLimit),
			Timeout:   30 * time.Minute,
		},
		readBlockSize:   fetchSize,
		readConcurrency: readConcurrency,
		readTimeout:     readTimeout,
		readBudget:      make(chan struct{}, backendLimit),

		readRetryAttempts:   retryAttempts,
		readRetryBackoff:    retryBackoff,
		readRetryMaxBackoff: retryMaxBackoff,
		metrics:             cfg.Metrics,
		cfStaleFallback:     cfg.CFStaleFallback,
		cfVersionPinned:     cfg.CFVersionPinned,
		keyPrefix:           cfg.KeyPrefix,
		conditionalKeys:     make(map[string]*conditionalKeyLock),
	}, nil
}

// Put stores an object in B2.
func (b *B2Backend) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error {
	return b.put(ctx, bucket, key, body, size, meta)
}

// PutIfAbsent stores an object only when its key does not already exist.
//
// B2's S3-compatible API closes PutObject requests carrying If-None-Match, so
// ARMOR serializes create-only writes for a key and checks existence first.
// This is atomic within one ARMOR process, which matches the supported
// single-replica B2 deployment topology. The lock remains held until PutObject
// completes, so an ambiguous client retry observes the completed object.
func (b *B2Backend) PutIfAbsent(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error {
	unlock := b.lockConditionalKey(bucket + "\x00" + key)
	defer unlock()

	_, err := b.s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err == nil {
		return ErrPreconditionFailed
	}
	if !isS3ObjectNotFound(err) {
		return fmt.Errorf("conditional HeadObject failed: %w", err)
	}

	return b.put(ctx, bucket, key, body, size, meta)
}

func (b *B2Backend) lockConditionalKey(key string) func() {
	b.conditionalMu.Lock()
	if b.conditionalKeys == nil {
		b.conditionalKeys = make(map[string]*conditionalKeyLock)
	}
	entry := b.conditionalKeys[key]
	if entry == nil {
		entry = &conditionalKeyLock{}
		b.conditionalKeys[key] = entry
	}
	entry.refs++
	b.conditionalMu.Unlock()

	entry.mu.Lock()
	return func() {
		entry.mu.Unlock()
		b.conditionalMu.Lock()
		entry.refs--
		if entry.refs == 0 {
			delete(b.conditionalKeys, key)
		}
		b.conditionalMu.Unlock()
	}
}

func isS3ObjectNotFound(err error) bool {
	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	code := apiErr.ErrorCode()
	return code == "NotFound" || code == "NoSuchKey"
}

func (b *B2Backend) put(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error {
	// B2 rejects requests without a Content-Length header. With an unseekable
	// body the Go transport falls back to Transfer-Encoding: chunked and drops
	// Content-Length (AWS accepts that, B2 does not), so spool to a temp file
	// to give the transport a seekable body with a known length.
	if _, ok := body.(io.ReadSeeker); !ok {
		tmp, err := os.CreateTemp("", "armor-put-*")
		if err != nil {
			return fmt.Errorf("PutObject spool failed: %w", err)
		}
		defer os.Remove(tmp.Name())
		defer tmp.Close()
		if _, err := io.Copy(tmp, body); err != nil {
			return fmt.Errorf("PutObject spool failed: %w", err)
		}
		if _, err := tmp.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("PutObject spool failed: %w", err)
		}
		body = tmp
	}
	input := &s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentLength: aws.Int64(size),
		Metadata:      toS3Metadata(meta),
	}
	_, err := b.s3Client.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("PutObject failed: %w", err)
	}
	return nil
}

// Get retrieves an object's full content and metadata.
// For ARMOR, this uses the Cloudflare domain for free egress.
func (b *B2Backend) Get(ctx context.Context, bucket, key string) (io.ReadCloser, *ObjectInfo, error) {
	// First get metadata via HeadObject (direct to B2, no egress cost)
	info, err := b.Head(ctx, bucket, key)
	if err != nil {
		return nil, nil, err
	}

	// Fetch body via Cloudflare for free egress
	readSize := info.StoredSize
	if readSize == 0 {
		readSize = info.Size
	}
	body, err := b.GetRange(ctx, bucket, key, 0, readSize)
	if err != nil {
		return nil, nil, err
	}

	return body, info, nil
}

// GetRange retrieves a byte range from an object using bounded pipelined
// ranged requests (via Cloudflare when configured).
func (b *B2Backend) GetRange(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, error) {
	body, _, err := b.GetRangeWithHeaders(ctx, bucket, key, offset, length)
	return body, err
}

// GetRangeWithHeaders retrieves a byte range using bounded pipelined ranged
// requests and returns the first response's relevant headers. It falls back to
// direct S3 API requests when cfDomain is empty (e.g. CF CDN unavailable).
func (b *B2Backend) GetRangeWithHeaders(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, map[string]string, error) {
	if offset < 0 || length < 0 {
		return nil, nil, fmt.Errorf("invalid range: offset=%d length=%d", offset, length)
	}
	if length == 0 {
		return io.NopCloser(bytes.NewReader(nil)), nil, nil
	}

	blockSize := b.readBlockSize
	if blockSize <= 0 {
		blockSize = defaultReadBlockSize
	}
	concurrency := b.readConcurrency
	if concurrency <= 0 {
		concurrency = defaultReadConcurrency
	}

	blockCount := 1 + (length-1)/blockSize
	firstLength := minInt64(blockSize, length)
	first, headers, err := b.fetchRange(ctx, bucket, key, offset, firstLength)
	if err != nil {
		return nil, nil, err
	}
	if blockCount == 1 {
		return io.NopCloser(bytes.NewReader(first)), headers, nil
	}

	// The first request is made synchronously so the existing header-reporting
	// contract remains intact. The rest of the range is fetched by a bounded
	// worker pool while the caller consumes the first block.
	pipelineCtx, cancel := context.WithCancelCause(ctx)
	workerCount := concurrency - 1
	if workerCount < 1 {
		workerCount = 1
	}
	if int64(workerCount) > blockCount-1 {
		workerCount = int(blockCount - 1)
	}

	// slots bounds the sliding window: a block may be scheduled only while a
	// slot is free, and the reader frees a slot when it consumes a block. In-
	// flight requests plus buffered results plus the reorder map therefore never
	// exceed window blocks, however long the object is or however long an early
	// block stalls.
	window := concurrency * 2
	slots := make(chan struct{}, window)
	jobs := make(chan rangedBlockJob)
	results := make(chan rangedBlockResult, window)
	var workers sync.WaitGroup
	workers.Add(workerCount)
	for i := 0; i < workerCount; i++ {
		go func() {
			defer workers.Done()
			for job := range jobs {
				data, _, fetchErr := b.fetchRange(pipelineCtx, bucket, key, job.offset, job.length)
				if fetchErr != nil {
					cancel(fmt.Errorf("fetch range block %d: %w", job.index, fetchErr))
					return
				}

				select {
				case results <- rangedBlockResult{index: job.index, data: data}:
				case <-pipelineCtx.Done():
					return
				}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for index := int64(1); index < blockCount; index++ {
			blockOffset := index * blockSize
			job := rangedBlockJob{
				index:  index,
				offset: offset + blockOffset,
				length: minInt64(blockSize, length-blockOffset),
			}
			select {
			case slots <- struct{}{}:
			case <-pipelineCtx.Done():
				return
			}
			select {
			case jobs <- job:
			case <-pipelineCtx.Done():
				return
			}
		}
	}()

	go func() {
		workers.Wait()
		close(results)
	}()

	return &pipelinedRangeReader{
		current:     first,
		nextBlock:   1,
		totalBlocks: blockCount,
		pipelineCtx: pipelineCtx,
		cancel:      cancel,
		workers:     &workers,
		results:     results,
		slots:       slots,
		pending:     make(map[int64][]byte, window),
	}, headers, nil
}

// fetchRange performs one bounded ranged request and fully buffers its block.
// Buffering is intentional: it lets the caller consume blocks in order while
// later ranged requests complete out of order, without buffering the object.
func (b *B2Backend) fetchRange(ctx context.Context, bucket, key string, offset, length int64) ([]byte, map[string]string, error) {
	if b.readBudget != nil {
		select {
		case b.readBudget <- struct{}{}:
			defer func() { <-b.readBudget }()
		case <-ctx.Done():
			return nil, nil, context.Cause(ctx)
		}
	}
	timeout := b.readTimeout
	if timeout <= 0 {
		timeout = defaultReadTimeout
	}
	// One deadline spans every retry attempt of this fetch (armor-f1791426):
	// the retry policy may spend the budget on more attempts but can never
	// exceed it. Retrying here is delivery-safe — the block is fully buffered
	// before any byte of it reaches the caller, and a failure discards the
	// partial buffer rather than delivering it.
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var data []byte
	var headers map[string]string
	err := b.doWithRetry(ctx, func() error {
		var fetchErr error
		data, headers, fetchErr = b.fetchRangeOnce(ctx, bucket, key, offset, length)
		return fetchErr
	})
	if err != nil {
		return nil, nil, err
	}
	return data, headers, nil
}

func (b *B2Backend) fetchRangeOnce(ctx context.Context, bucket, key string, offset, length int64) ([]byte, map[string]string, error) {
	rangeHeader := fmt.Sprintf("bytes=%d-%d", offset, offset+length-1)
	if b.cfDomain == "" {
		// No CF domain configured — fall back to direct S3 range request.
		resp, err := b.s3Client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
			Range:  aws.String(rangeHeader),
		})
		if err != nil {
			return nil, nil, fmt.Errorf("direct S3 range request failed: %w", err)
		}
		defer resp.Body.Close()
		// A 206 carries Content-Range naming the span actually served; a
		// present header must match the request exactly (armor-73564f97). A
		// range-ignoring 200 is caught by the length check below: with
		// offset > 0 the whole-object body is strictly longer than the span.
		if err := checkContentRange(aws.ToString(resp.ContentRange), offset, length); err != nil {
			return nil, nil, fmt.Errorf("direct S3 range response: %w", err)
		}
		data, err := readRangeBody(resp.Body, length)
		if err != nil {
			return nil, nil, fmt.Errorf("direct S3 range response: %w", err)
		}
		return data, nil, nil
	}

	// Construct Cloudflare download URL. With a pinned version and
	// CFVersionPinned the URL names the B2 file ID, which is immutable.
	pinned := pinnedVersion(ctx, bucket, key)
	cfURL := fmt.Sprintf("https://%s/file/%s/%s", b.cfDomain, bucket, url.PathEscape(key))
	if b.cfVersionPinned && pinned != "" {
		cfURL = fmt.Sprintf("https://%s/b2api/v1/b2_download_file_by_id?fileId=%s", b.cfDomain, url.QueryEscape(pinned))
	}

	httpClient := b.httpClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	// A ranged request with a nonzero offset must be answered with 206. B2
	// occasionally answers 200 with the whole object instead — observed live
	// on well-formed Range requests under concurrency (armor-817d9d92), and
	// reproducibly on any malformed Range. Reading that body surfaces as a
	// confusing length mismatch, so retry once and then fail with an explicit
	// message. For offset 0 a 200 is indistinguishable from a fulfilled range
	// and reads the correct bytes either way, so it is accepted.
	var resp *http.Response
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfURL, nil)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Range", rangeHeader)

		resp, err = httpClient.Do(req)
		if err != nil {
			return nil, nil, fmt.Errorf("cloudflare request failed: %w", err)
		}
		if resp.StatusCode != http.StatusOK || offset == 0 || attempt > 0 {
			break
		}
		// The origin ignored the Range header. Close without draining (the
		// body may be the entire object) and retry.
		resp.Body.Close()
	}
	defer resp.Body.Close()

	// A stale edge copy shows up as a different file ID than the HEAD saw, or
	// as a 416 for a range the new version has but the cached one is too short
	// to satisfy. Either way the bytes belong to another version: read this
	// range straight from B2 at the pinned version instead of failing the
	// block check downstream.
	if b.cfStaleFallback && pinned != "" {
		served := resp.Header.Get("X-Bz-File-Id")
		if (served != "" && served != pinned) || resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
			cfStaleFallbacks.Add(1)
			return b.fetchRangeVersion(ctx, bucket, key, pinned, rangeHeader, offset, length)
		}
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return nil, nil, &httpStatusError{status: resp.StatusCode}
	}
	if resp.StatusCode == http.StatusOK && offset > 0 {
		return nil, nil, fmt.Errorf("cloudflare ignored Range header (status 200 for bytes %d-%d)", offset, offset+length-1)
	}

	if resp.StatusCode == http.StatusPartialContent {
		if err := checkContentRange(resp.Header.Get("Content-Range"), offset, length); err != nil {
			return nil, nil, fmt.Errorf("cloudflare range response: %w", err)
		}
	}
	data, err := readRangeBody(resp.Body, length)
	if err != nil {
		return nil, nil, fmt.Errorf("cloudflare range response: %w", err)
	}

	return data, cloudflareHeaders(resp), nil
}

// fetchRangeVersion reads one range of a specific version directly from B2's S3
// API, bypassing Cloudflare.
func (b *B2Backend) fetchRangeVersion(ctx context.Context, bucket, key, versionID, rangeHeader string, offset, length int64) ([]byte, map[string]string, error) {
	resp, err := b.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket:    aws.String(bucket),
		Key:       aws.String(key),
		VersionId: aws.String(versionID),
		Range:     aws.String(rangeHeader),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("direct S3 range request for version %s after stale Cloudflare copy failed: %w", versionID, err)
	}
	defer resp.Body.Close()
	if err := checkContentRange(aws.ToString(resp.ContentRange), offset, length); err != nil {
		return nil, nil, fmt.Errorf("direct S3 range response for version %s: %w", versionID, err)
	}
	data, err := readRangeBody(resp.Body, length)
	if err != nil {
		return nil, nil, fmt.Errorf("direct S3 range response for version %s: %w", versionID, err)
	}
	return data, nil, nil
}

// checkContentRange rejects a 206 whose Content-Range names a different span
// than the one requested. A missing header is tolerated (some edges omit it);
// a present one must match exactly.
func checkContentRange(header string, offset, length int64) error {
	if header == "" {
		return nil
	}
	var start, end int64
	if _, err := fmt.Sscanf(header, "bytes %d-%d/", &start, &end); err != nil {
		return fmt.Errorf("unparseable Content-Range %q", header)
	}
	if start != offset || end != offset+length-1 {
		return fmt.Errorf("Content-Range %q does not match requested bytes %d-%d", header, offset, offset+length-1)
	}
	return nil
}

func readRangeBody(body io.Reader, expectedLength int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, expectedLength+1))
	if err != nil {
		return nil, fmt.Errorf("read %d bytes: %w", expectedLength, err)
	}
	if int64(len(data)) != expectedLength {
		return nil, &shortReadError{got: len(data), want: expectedLength}
	}
	return data, nil
}

func cloudflareHeaders(resp *http.Response) map[string]string {
	headers := make(map[string]string)
	if cfStatus := resp.Header.Get("CF-Cache-Status"); cfStatus != "" {
		headers["CF-Cache-Status"] = cfStatus
	}
	if cfRay := resp.Header.Get("CF-Ray"); cfRay != "" {
		headers["CF-Ray"] = cfRay
	}
	if age := resp.Header.Get("Age"); age != "" {
		headers["Age"] = age
	}
	return headers
}

type pipelinedRangeReader struct {
	current     []byte
	currentPos  int
	nextBlock   int64
	totalBlocks int64

	pipelineCtx context.Context
	cancel      context.CancelCauseFunc
	workers     *sync.WaitGroup
	results     <-chan rangedBlockResult
	slots       <-chan struct{}
	pending     map[int64][]byte
	closeOnce   sync.Once
}

func (r *pipelinedRangeReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	read := 0
	for read < len(p) {
		if err := r.pipelineError(); err != nil {
			return read, err
		}

		if r.currentPos < len(r.current) {
			n := copy(p[read:], r.current[r.currentPos:])
			r.currentPos += n
			read += n
			continue
		}

		if r.nextBlock >= r.totalBlocks {
			return read, io.EOF
		}

		data, err := r.nextBlockData()
		if err != nil {
			return read, err
		}
		r.current = data
		r.currentPos = 0
	}
	return read, nil
}

func (r *pipelinedRangeReader) nextBlockData() ([]byte, error) {
	for {
		if data, ok := r.pending[r.nextBlock]; ok {
			delete(r.pending, r.nextBlock)
			r.nextBlock++
			<-r.slots // block consumed: let the scheduler move the window forward
			return data, nil
		}
		if err := r.pipelineError(); err != nil {
			return nil, err
		}

		select {
		case result, ok := <-r.results:
			if !ok {
				return nil, fmt.Errorf("pipelined range read ended before block %d: %w", r.nextBlock, io.ErrUnexpectedEOF)
			}
			r.pending[result.index] = result.data
		case <-r.pipelineCtx.Done():
			return nil, r.pipelineError()
		}
	}
}

func (r *pipelinedRangeReader) pipelineError() error {
	select {
	case <-r.pipelineCtx.Done():
		cause := context.Cause(r.pipelineCtx)
		if cause == nil {
			cause = r.pipelineCtx.Err()
		}
		return fmt.Errorf("pipelined range read failed: %w", cause)
	default:
		return nil
	}
}

func (r *pipelinedRangeReader) Close() error {
	r.closeOnce.Do(func() {
		r.cancel(context.Canceled)
		r.workers.Wait()
	})
	return nil
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// Head retrieves object metadata without the body.
func (b *B2Backend) Head(ctx context.Context, bucket, key string) (*ObjectInfo, error) {
	var resp *s3.HeadObjectOutput
	err := b.doWithRetry(ctx, func() error {
		r, herr := b.s3Client.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		if herr != nil {
			return fmt.Errorf("HeadObject failed: %w", herr)
		}
		resp = r
		return nil
	})
	if err != nil {
		return nil, err
	}

	info := &ObjectInfo{
		Key:          key,
		Size:         aws.ToInt64(resp.ContentLength),
		StoredSize:   aws.ToInt64(resp.ContentLength),
		ContentType:  aws.ToString(resp.ContentType),
		ETag:         aws.ToString(resp.ETag),
		LastModified: aws.ToTime(resp.LastModified),
		Metadata:     fromS3Metadata(resp.Metadata),
		VersionID:    aws.ToString(resp.VersionId),
	}
	recordVersionPin(ctx, bucket, key, info.VersionID)

	// Check if this is an ARMOR-encrypted object
	if _, ok := ParseARMORMetadata(info.Metadata); ok {
		info.IsARMOREncrypted = true
		// Use plaintext size from metadata
		if am, _ := ParseARMORMetadata(info.Metadata); am != nil {
			info.Size = am.PlaintextSize
		}
	}

	return info, nil
}

// HeadVersion retrieves object metadata for a specific version.
// Returns ARMOR metadata (plaintext size) for the specific version.
func (b *B2Backend) HeadVersion(ctx context.Context, bucket, key, versionID string) (*ObjectInfo, error) {
	var resp *s3.HeadObjectOutput
	err := b.doWithRetry(ctx, func() error {
		r, herr := b.s3Client.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket:    aws.String(bucket),
			Key:       aws.String(key),
			VersionId: aws.String(versionID),
		})
		if herr != nil {
			return fmt.Errorf("HeadObject failed: %w", herr)
		}
		resp = r
		return nil
	})
	if err != nil {
		return nil, err
	}

	info := &ObjectInfo{
		Key:          key,
		Size:         aws.ToInt64(resp.ContentLength),
		StoredSize:   aws.ToInt64(resp.ContentLength),
		ContentType:  aws.ToString(resp.ContentType),
		ETag:         aws.ToString(resp.ETag),
		LastModified: aws.ToTime(resp.LastModified),
		Metadata:     fromS3Metadata(resp.Metadata),
	}

	// Check if this is an ARMOR-encrypted object
	if _, ok := ParseARMORMetadata(info.Metadata); ok {
		info.IsARMOREncrypted = true
		// Use plaintext size from metadata
		if am, _ := ParseARMORMetadata(info.Metadata); am != nil {
			info.Size = am.PlaintextSize
		}
	}

	return info, nil
}

// Delete removes an object from B2.
func (b *B2Backend) Delete(ctx context.Context, bucket, key string) error {
	_, err := b.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("DeleteObject failed: %w", err)
	}
	return nil
}

// DeleteObjects removes multiple objects from B2.
func (b *B2Backend) DeleteObjects(ctx context.Context, bucket string, keys []string) error {
	objects := make([]types.ObjectIdentifier, len(keys))
	for i, key := range keys {
		objects[i] = types.ObjectIdentifier{Key: aws.String(key)}
	}

	_, err := b.s3Client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(bucket),
		Delete: &types.Delete{
			Objects: objects,
		},
	})
	if err != nil {
		return fmt.Errorf("DeleteObjects failed: %w", err)
	}
	return nil
}

// List objects in a bucket with optional prefix.
func (b *B2Backend) List(ctx context.Context, bucket, prefix, delimiter, continuationToken string, maxKeys int) (*ListResult, error) {
	input := &s3.ListObjectsV2Input{
		Bucket:            aws.String(bucket),
		Prefix:            aws.String(prefix),
		ContinuationToken: aws.String(continuationToken),
	}
	if delimiter != "" {
		input.Delimiter = aws.String(delimiter)
	}
	if maxKeys > 0 {
		input.MaxKeys = aws.Int32(int32(maxKeys))
	}

	resp, err := b.s3Client.ListObjectsV2(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("ListObjectsV2 failed: %w", err)
	}

	result := &ListResult{
		IsTruncated: aws.ToBool(resp.IsTruncated),
		NextToken:   aws.ToString(resp.NextContinuationToken),
	}

	// Process objects
	for _, obj := range resp.Contents {
		// Filter out .armor/ internal objects (at the bucket root, or under the
		// ADR-001 prefix when one is configured)
		if isInternalKey(b.keyPrefix, aws.ToString(obj.Key)) {
			continue
		}

		info := &ObjectInfo{
			Key:          aws.ToString(obj.Key),
			Size:         aws.ToInt64(obj.Size),
			ETag:         aws.ToString(obj.ETag),
			LastModified: aws.ToTime(obj.LastModified),
		}

		// Try to get ARMOR metadata (requires additional HeadObject call)
		// For efficiency in listings, we rely on the plaintext size being
		// stored in the object's metadata which requires HeadObject per item.
		// For now, we return the raw size; callers needing plaintext size
		// should use HeadObject individually.

		result.Objects = append(result.Objects, *info)
	}

	// Process common prefixes
	for _, prefix := range resp.CommonPrefixes {
		result.CommonPrefixes = append(result.CommonPrefixes, aws.ToString(prefix.Prefix))
	}

	return result, nil
}

// Copy copies an object in B2, supporting cross-bucket copy.
func (b *B2Backend) Copy(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string, meta map[string]string, replaceMetadata bool) error {
	input := &s3.CopyObjectInput{
		Bucket:     aws.String(dstBucket),
		CopySource: aws.String(copySource(srcBucket, srcKey)),
		Key:        aws.String(dstKey),
	}

	if replaceMetadata {
		input.MetadataDirective = types.MetadataDirectiveReplace
		input.Metadata = toS3Metadata(meta)
	}

	_, err := b.s3Client.CopyObject(ctx, input)
	if err != nil {
		return fmt.Errorf("CopyObject failed: %w", err)
	}
	return nil
}

// copySource encodes each path segment for the x-amz-copy-source header while
// preserving key separators. PathEscape leaves '+' unchanged, but S3 expects
// it encoded so it cannot be interpreted as a space when the header is parsed.
func copySource(bucket, key string) string {
	segments := strings.Split(key, "/")
	for i, segment := range segments {
		segments[i] = strings.ReplaceAll(url.PathEscape(segment), "+", "%2B")
	}
	return url.PathEscape(bucket) + "/" + strings.Join(segments, "/")
}

// GetDirect retrieves an object directly from B2 (not via Cloudflare).
// Used for metadata operations where we don't need the body.
func (b *B2Backend) GetDirect(ctx context.Context, bucket, key string) (io.ReadCloser, *ObjectInfo, error) {
	resp, err := b.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("GetObject failed: %w", err)
	}

	info := &ObjectInfo{
		Key:          key,
		Size:         aws.ToInt64(resp.ContentLength),
		StoredSize:   aws.ToInt64(resp.ContentLength),
		ContentType:  aws.ToString(resp.ContentType),
		ETag:         aws.ToString(resp.ETag),
		LastModified: aws.ToTime(resp.LastModified),
		Metadata:     fromS3Metadata(resp.Metadata),
	}

	if _, ok := ParseARMORMetadata(info.Metadata); ok {
		info.IsARMOREncrypted = true
	}

	return resp.Body, info, nil
}

// ComputeETag computes an MD5 ETag for the given data.
func ComputeETag(data []byte) string {
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}

// ComputeETagReader computes an MD5 ETag from a reader.
func ComputeETagReader(r io.Reader) (string, error) {
	hash := md5.New()
	if _, err := io.Copy(hash, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// toS3Metadata converts a map to S3 metadata format.
func toS3Metadata(meta map[string]string) map[string]string {
	if meta == nil {
		return nil
	}
	result := make(map[string]string)
	for k, v := range meta {
		result[k] = v
	}
	return result
}

// fromS3Metadata converts S3 metadata to a regular map.
func fromS3Metadata(meta map[string]string) map[string]string {
	if meta == nil {
		return nil
	}
	result := make(map[string]string)
	for k, v := range meta {
		result[k] = v
	}
	return result
}

// ListRaw lists objects without filtering .armor/ internal objects.
// Used for internal operations such as manifest discovery where all keys
// under a prefix (including .armor/ prefixed keys) must be enumerated.
func (b *B2Backend) ListRaw(ctx context.Context, bucket, prefix, delimiter, continuationToken string, maxKeys int) (*ListResult, error) {
	input := &s3.ListObjectsV2Input{
		Bucket:            aws.String(bucket),
		Prefix:            aws.String(prefix),
		ContinuationToken: aws.String(continuationToken),
	}
	if delimiter != "" {
		input.Delimiter = aws.String(delimiter)
	}
	if maxKeys > 0 {
		input.MaxKeys = aws.Int32(int32(maxKeys))
	}

	resp, err := b.s3Client.ListObjectsV2(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("ListObjectsV2 failed: %w", err)
	}

	result := &ListResult{
		IsTruncated: aws.ToBool(resp.IsTruncated),
		NextToken:   aws.ToString(resp.NextContinuationToken),
	}

	for _, obj := range resp.Contents {
		info := &ObjectInfo{
			Key:          aws.ToString(obj.Key),
			Size:         aws.ToInt64(obj.Size),
			ETag:         aws.ToString(obj.ETag),
			LastModified: aws.ToTime(obj.LastModified),
		}
		result.Objects = append(result.Objects, *info)
	}

	for _, p := range resp.CommonPrefixes {
		result.CommonPrefixes = append(result.CommonPrefixes, aws.ToString(p.Prefix))
	}

	return result, nil
}

// S3Client returns the underlying S3 client for advanced operations.
func (b *B2Backend) S3Client() *s3.Client {
	return b.s3Client
}

// httpStatusError carries the HTTP status of a Cloudflare/B2 response that
// failed a read, so retry classification never has to string-match a status
// out of error text.
type httpStatusError struct {
	status int
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("cloudflare returned status %d", e.status)
}

// shortReadError reports a response body that ended before the length the
// response promised — a truncated edge or origin response.
type shortReadError struct {
	got  int
	want int64
}

func (e *shortReadError) Error() string {
	return fmt.Sprintf("received %d bytes, want %d", e.got, e.want)
}

// Transient-failure classes recorded on armor_backend_retries_total.
const (
	retryReasonHTTP5xx         = "http_5xx"
	retryReasonHTTP429         = "http_429"
	retryReasonConnectionReset = "connection_reset"
	retryReasonTruncatedBody   = "truncated_body"
	retryReasonNetwork         = "network"
)

// Outcomes recorded on armor_backend_retries_total.
const (
	retryOutcomeSuccess   = "success"
	retryOutcomeExhausted = "exhausted"
	retryOutcomeCancelled = "cancelled"
)

// retryClass reports the transient-failure class of a backend read error, or
// "" when the error is not worth retrying. Classification is typed — HTTP
// statuses arrive in httpStatusError values or SDK ResponseError wraps,
// truncations in shortReadError or as an unexpected EOF, resets and timeouts
// via errors.Is/errors.As — never by substring.
func retryClass(err error) string {
	if err == nil || errors.Is(err, context.Canceled) {
		return ""
	}
	statusClass := func(code int) string {
		switch {
		case code == http.StatusTooManyRequests:
			return retryReasonHTTP429
		case code >= 500:
			return retryReasonHTTP5xx
		}
		return ""
	}
	var statusErr *httpStatusError
	if errors.As(err, &statusErr) {
		return statusClass(statusErr.status)
	}
	var respErr *awshttp.ResponseError
	if errors.As(err, &respErr) {
		return statusClass(respErr.HTTPStatusCode())
	}
	var shortErr *shortReadError
	if errors.As(err, &shortErr) {
		return retryReasonTruncatedBody
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return retryReasonTruncatedBody
	}
	if errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNABORTED) {
		return retryReasonConnectionReset
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return retryReasonNetwork
	}
	return ""
}

// IsRetryableError reports whether a backend read error is transient and worth
// retrying. See retryClass for the accepted classes.
func IsRetryableError(err error) bool {
	return retryClass(err) != ""
}

// RetryPolicy bounds a retry sequence: total tries, the initial backoff sleep,
// and the cap on one sleep. It is shared by per-fetch backend retries and the
// manifest startup load (armor-f1791426).
type RetryPolicy struct {
	Attempts   int           // total tries including the first
	Backoff    time.Duration // initial sleep between attempts
	MaxBackoff time.Duration // upper bound on one sleep
}

// RetryOutcome describes how a DoWithRetry call ended when at least one retry
// was attempted; with no retry, Reason and Outcome stay "".
type RetryOutcome struct {
	Retries int    // retry decisions taken (attempts after the first)
	Reason  string // transient-failure class of the last retried failure
	Outcome string // success, exhausted, or cancelled
}

// DoWithRetry runs fn up to p.Attempts times, retrying errors accepted by
// IsRetryableError with jittered exponential backoff, and returns the last
// error. Sleeps watch ctx, so cancellation during backoff returns promptly
// with the failure that triggered the retry. Only idempotent work whose
// partial results are discarded on failure belongs here — a retried call must
// never be visible twice downstream.
func DoWithRetry(ctx context.Context, p RetryPolicy, fn func() error) (RetryOutcome, error) {
	attempts := p.Attempts
	if attempts < 1 {
		attempts = 1
	}
	var outcome RetryOutcome
	var err error
	for attempt := 1; ; attempt++ {
		err = fn()
		if err == nil {
			if outcome.Retries > 0 {
				outcome.Outcome = retryOutcomeSuccess
			}
			return outcome, nil
		}
		class := retryClass(err)
		// A dead context (deadline reached or caller gone) ended the request
		// that just failed; retrying inside it cannot succeed.
		if attempt >= attempts || class == "" || ctx.Err() != nil {
			if outcome.Retries > 0 {
				outcome.Outcome = retryOutcomeExhausted
			}
			return outcome, err
		}
		outcome.Retries++
		outcome.Reason = class
		if sleepErr := SleepBackoff(ctx, attempt, p.Backoff, p.MaxBackoff); sleepErr != nil {
			outcome.Outcome = retryOutcomeCancelled
			return outcome, err
		}
	}
}

// SleepBackoff sleeps the attempt-th backoff interval — base scaled by
// 2^(attempt-1), capped at maxBackoff, jittered uniformly over [0, interval) —
// and returns ctx.Err() if the context ends first. A non-positive base sleeps
// zero.
func SleepBackoff(ctx context.Context, attempt int, base, maxBackoff time.Duration) error {
	d := base
	for i := 1; i < attempt && (maxBackoff <= 0 || d < maxBackoff); i++ {
		d *= 2
	}
	if maxBackoff > 0 && d > maxBackoff {
		d = maxBackoff
	}
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(time.Duration(rand.Int64N(int64(d))))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// doWithRetry runs fn under the backend's read-retry configuration, recording
// one armor_backend_retries_total increment when any retry was attempted.
func (b *B2Backend) doWithRetry(ctx context.Context, fn func() error) error {
	outcome, err := DoWithRetry(ctx, RetryPolicy{
		Attempts:   b.readRetryAttempts,
		Backoff:    b.readRetryBackoff,
		MaxBackoff: b.readRetryMaxBackoff,
	}, fn)
	if outcome.Retries > 0 && b.metrics != nil {
		b.metrics.IncBackendRetry(outcome.Reason, outcome.Outcome)
	}
	return err
}

// ListBuckets lists all buckets.
func (b *B2Backend) ListBuckets(ctx context.Context) ([]BucketInfo, error) {
	resp, err := b.s3Client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, fmt.Errorf("ListBuckets failed: %w", err)
	}

	buckets := make([]BucketInfo, len(resp.Buckets))
	for i, bucket := range resp.Buckets {
		buckets[i] = BucketInfo{
			Name:         aws.ToString(bucket.Name),
			CreationDate: aws.ToTime(bucket.CreationDate),
		}
	}

	return buckets, nil
}

// CreateBucket creates a new bucket.
func (b *B2Backend) CreateBucket(ctx context.Context, bucket string) error {
	_, err := b.s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		return fmt.Errorf("CreateBucket failed: %w", err)
	}
	return nil
}

// DeleteBucket deletes an empty bucket.
func (b *B2Backend) DeleteBucket(ctx context.Context, bucket string) error {
	_, err := b.s3Client.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		return fmt.Errorf("DeleteBucket failed: %w", err)
	}
	return nil
}

// HeadBucket checks if a bucket exists.
func (b *B2Backend) HeadBucket(ctx context.Context, bucket string) error {
	_, err := b.s3Client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		return fmt.Errorf("HeadBucket failed: %w", err)
	}
	return nil
}

// ReaderAtCloser wraps a reader with Close and ReadAt methods.
type ReaderAtCloser struct {
	data []byte
}

// NewReaderAtCloser creates a ReaderAtCloser from a byte slice.
func NewReaderAtCloser(data []byte) *ReaderAtCloser {
	return &ReaderAtCloser{data: data}
}

func (r *ReaderAtCloser) Read(p []byte) (n int, err error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	n = copy(p, r.data)
	r.data = r.data[n:]
	return n, nil
}

func (r *ReaderAtCloser) ReadAt(p []byte, off int64) (n int, err error) {
	if off >= int64(len(r.data)) {
		return 0, io.EOF
	}
	n = copy(p, r.data[off:])
	if n < len(p) {
		err = io.EOF
	}
	return n, err
}

func (r *ReaderAtCloser) Close() error {
	return nil
}

func (r *ReaderAtCloser) Bytes() []byte {
	return r.data
}

// CreateMultipartUpload initiates a multipart upload.
func (b *B2Backend) CreateMultipartUpload(ctx context.Context, bucket, key string, meta map[string]string) (string, error) {
	resp, err := b.s3Client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		Metadata: toS3Metadata(meta),
	})
	if err != nil {
		return "", fmt.Errorf("CreateMultipartUpload failed: %w", err)
	}
	return aws.ToString(resp.UploadId), nil
}

// UploadPart uploads a part to a multipart upload.
func (b *B2Backend) UploadPart(ctx context.Context, bucket, key, uploadID string, partNumber int32, body io.Reader, size int64) (string, error) {
	resp, err := b.s3Client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(key),
		UploadId:      aws.String(uploadID),
		PartNumber:    aws.Int32(partNumber),
		Body:          body,
		ContentLength: aws.Int64(size),
	})
	if err != nil {
		return "", fmt.Errorf("UploadPart failed: %w", err)
	}
	return aws.ToString(resp.ETag), nil
}

// CompleteMultipartUpload completes a multipart upload.
func (b *B2Backend) CompleteMultipartUpload(ctx context.Context, bucket, key, uploadID string, parts []CompletedPart) (string, error) {
	// Convert to AWS SDK type
	awsParts := make([]types.CompletedPart, len(parts))
	for i, p := range parts {
		awsParts[i] = types.CompletedPart{
			ETag:       aws.String(p.ETag),
			PartNumber: aws.Int32(p.PartNumber),
		}
	}

	resp, err := b.s3Client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: awsParts,
		},
	})
	if err != nil {
		return "", fmt.Errorf("CompleteMultipartUpload failed: %w", err)
	}
	return aws.ToString(resp.ETag), nil
}

// AbortMultipartUpload aborts a multipart upload.
func (b *B2Backend) AbortMultipartUpload(ctx context.Context, bucket, key, uploadID string) error {
	_, err := b.s3Client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
	})
	if err != nil {
		return fmt.Errorf("AbortMultipartUpload failed: %w", err)
	}
	return nil
}

// ListParts lists the parts of a multipart upload.
func (b *B2Backend) ListParts(ctx context.Context, bucket, key, uploadID string) (*ListPartsResult, error) {
	resp, err := b.s3Client.ListParts(ctx, &s3.ListPartsInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
	})
	if err != nil {
		return nil, fmt.Errorf("ListParts failed: %w", err)
	}

	result := &ListPartsResult{
		Bucket:      aws.ToString(resp.Bucket),
		Key:         aws.ToString(resp.Key),
		UploadID:    aws.ToString(resp.UploadId),
		IsTruncated: aws.ToBool(resp.IsTruncated),
	}

	if resp.NextPartNumberMarker != nil {
		if val, err := strconv.Atoi(*resp.NextPartNumberMarker); err == nil {
			result.NextPartNumberMarker = val
		}
	}

	for _, part := range resp.Parts {
		result.Parts = append(result.Parts, PartInfo{
			PartNumber:   aws.ToInt32(part.PartNumber),
			ETag:         aws.ToString(part.ETag),
			Size:         aws.ToInt64(part.Size),
			LastModified: aws.ToTime(part.LastModified),
		})
	}

	return result, nil
}

// ListMultipartUploads lists active multipart uploads.
func (b *B2Backend) ListMultipartUploads(ctx context.Context, bucket, prefix string) (*ListMultipartUploadsResult, error) {
	input := &s3.ListMultipartUploadsInput{
		Bucket: aws.String(bucket),
	}
	// Apply the key prefix filter when one is provided so the backend only
	// returns uploads under the ARMOR namespace.
	if prefix != "" {
		input.Prefix = aws.String(prefix)
	}
	resp, err := b.s3Client.ListMultipartUploads(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("ListMultipartUploads failed: %w", err)
	}

	result := &ListMultipartUploadsResult{
		Bucket:             aws.ToString(resp.Bucket),
		IsTruncated:        aws.ToBool(resp.IsTruncated),
		NextKeyMarker:      aws.ToString(resp.NextKeyMarker),
		NextUploadIDMarker: aws.ToString(resp.NextUploadIdMarker),
	}

	for _, upload := range resp.Uploads {
		result.Uploads = append(result.Uploads, UploadInfo{
			UploadID:  aws.ToString(upload.UploadId),
			Key:       aws.ToString(upload.Key),
			Initiated: aws.ToTime(upload.Initiated),
		})
	}

	return result, nil
}

// GetBucketLifecycleConfiguration gets the lifecycle configuration for a bucket.
func (b *B2Backend) GetBucketLifecycleConfiguration(ctx context.Context, bucket string) ([]byte, error) {
	resp, err := b.s3Client.GetBucketLifecycleConfiguration(ctx, &s3.GetBucketLifecycleConfigurationInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		return nil, fmt.Errorf("GetBucketLifecycleConfiguration failed: %w", err)
	}

	// Build the XML response manually to ensure proper S3 format
	var rules []string
	for _, rule := range resp.Rules {
		ruleXML := b.buildLifecycleRuleXML(rule)
		rules = append(rules, ruleXML)
	}

	xmlBody := fmt.Sprintf(`<LifecycleConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/">%s</LifecycleConfiguration>`,
		strings.Join(rules, ""))
	return []byte(xmlBody), nil
}

// buildLifecycleRuleXML builds XML for a single lifecycle rule.
func (b *B2Backend) buildLifecycleRuleXML(rule types.LifecycleRule) string {
	var parts []string

	// ID
	if rule.ID != nil {
		parts = append(parts, fmt.Sprintf("<ID>%s</ID>", *rule.ID))
	}

	// Status
	if rule.Status != "" {
		parts = append(parts, fmt.Sprintf("<Status>%s</Status>", rule.Status))
	}

	// Filter
	if rule.Filter != nil {
		filterXML := b.buildLifecycleFilterXML(rule.Filter)
		parts = append(parts, fmt.Sprintf("<Filter>%s</Filter>", filterXML))
	}

	// Expiration
	if rule.Expiration != nil {
		var expParts []string
		if rule.Expiration.Days != nil {
			expParts = append(expParts, fmt.Sprintf("<Days>%d</Days>", *rule.Expiration.Days))
		}
		if rule.Expiration.Date != nil {
			expParts = append(expParts, fmt.Sprintf("<Date>%s</Date>", rule.Expiration.Date.Format(time.RFC3339)))
		}
		if len(expParts) > 0 {
			parts = append(parts, fmt.Sprintf("<Expiration>%s</Expiration>", strings.Join(expParts, "")))
		}
	}

	// NoncurrentVersionExpiration
	if rule.NoncurrentVersionExpiration != nil {
		var nveParts []string
		if rule.NoncurrentVersionExpiration.NoncurrentDays != nil {
			nveParts = append(nveParts, fmt.Sprintf("<NoncurrentDays>%d</NoncurrentDays>", *rule.NoncurrentVersionExpiration.NoncurrentDays))
		}
		if len(nveParts) > 0 {
			parts = append(parts, fmt.Sprintf("<NoncurrentVersionExpiration>%s</NoncurrentVersionExpiration>", strings.Join(nveParts, "")))
		}
	}

	// AbortIncompleteMultipartUpload
	if rule.AbortIncompleteMultipartUpload != nil {
		var aimuParts []string
		if rule.AbortIncompleteMultipartUpload.DaysAfterInitiation != nil {
			aimuParts = append(aimuParts, fmt.Sprintf("<DaysAfterInitiation>%d</DaysAfterInitiation>", *rule.AbortIncompleteMultipartUpload.DaysAfterInitiation))
		}
		if len(aimuParts) > 0 {
			parts = append(parts, fmt.Sprintf("<AbortIncompleteMultipartUpload>%s</AbortIncompleteMultipartUpload>", strings.Join(aimuParts, "")))
		}
	}

	return fmt.Sprintf("<Rule>%s</Rule>", strings.Join(parts, ""))
}

// buildLifecycleFilterXML builds XML for a lifecycle filter.
func (b *B2Backend) buildLifecycleFilterXML(filter *types.LifecycleRuleFilter) string {
	if filter == nil {
		return ""
	}

	// Check which field is set (only one can be set at a time)
	if filter.Prefix != nil {
		return fmt.Sprintf("<Prefix>%s</Prefix>", *filter.Prefix)
	}
	if filter.Tag != nil {
		return fmt.Sprintf("<Tag><Key>%s</Key><Value>%s</Value></Tag>", aws.ToString(filter.Tag.Key), aws.ToString(filter.Tag.Value))
	}
	if filter.And != nil {
		var andParts []string
		if filter.And.Prefix != nil {
			andParts = append(andParts, fmt.Sprintf("<Prefix>%s</Prefix>", *filter.And.Prefix))
		}
		for _, tag := range filter.And.Tags {
			andParts = append(andParts, fmt.Sprintf("<Tag><Key>%s</Key><Value>%s</Value></Tag>", aws.ToString(tag.Key), aws.ToString(tag.Value)))
		}
		return fmt.Sprintf("<And>%s</And>", strings.Join(andParts, ""))
	}
	return ""
}

// PutBucketLifecycleConfiguration sets the lifecycle configuration for a bucket.
func (b *B2Backend) PutBucketLifecycleConfiguration(ctx context.Context, bucket string, config []byte) error {
	// Parse the lifecycle configuration XML and convert to SDK types
	rules, err := b.parseLifecycleConfig(config)
	if err != nil {
		return fmt.Errorf("failed to parse lifecycle configuration: %w", err)
	}

	_, err = b.s3Client.PutBucketLifecycleConfiguration(ctx, &s3.PutBucketLifecycleConfigurationInput{
		Bucket:                 aws.String(bucket),
		LifecycleConfiguration: &types.BucketLifecycleConfiguration{Rules: rules},
	})
	if err != nil {
		return fmt.Errorf("PutBucketLifecycleConfiguration failed: %w", err)
	}
	return nil
}

// parseLifecycleConfig parses lifecycle configuration XML into SDK types.
func (b *B2Backend) parseLifecycleConfig(config []byte) ([]types.LifecycleRule, error) {
	// Simple XML parsing for common lifecycle rules
	// This handles the most common cases; full XML parsing would use encoding/xml
	var rules []types.LifecycleRule

	// Use a simple approach: parse with encoding/xml
	type LifecycleConfiguration struct {
		XMLName xml.Name `xml:"LifecycleConfiguration"`
		Rules   []struct {
			ID     string `xml:"ID"`
			Status string `xml:"Status"`
			Prefix string `xml:"Prefix"`
			Filter *struct {
				Prefix string `xml:"Prefix"`
			} `xml:"Filter"`
			Expiration *struct {
				Days int `xml:"Days"`
			} `xml:"Expiration"`
			AbortIncompleteMultipartUpload *struct {
				DaysAfterInitiation int `xml:"DaysAfterInitiation"`
			} `xml:"AbortIncompleteMultipartUpload"`
		} `xml:"Rule"`
	}

	var lc LifecycleConfiguration
	if err := xml.Unmarshal(config, &lc); err != nil {
		return nil, err
	}

	for _, r := range lc.Rules {
		rule := types.LifecycleRule{
			ID:     aws.String(r.ID),
			Status: types.ExpirationStatus(r.Status),
		}

		// Handle filter
		if r.Filter != nil && r.Filter.Prefix != "" {
			rule.Filter = &types.LifecycleRuleFilter{
				Prefix: aws.String(r.Filter.Prefix),
			}
		} else if r.Prefix != "" {
			rule.Filter = &types.LifecycleRuleFilter{
				Prefix: aws.String(r.Prefix),
			}
		}

		// Handle expiration
		if r.Expiration != nil {
			rule.Expiration = &types.LifecycleExpiration{
				Days: aws.Int32(int32(r.Expiration.Days)),
			}
		}

		// Handle abort incomplete multipart upload
		if r.AbortIncompleteMultipartUpload != nil {
			rule.AbortIncompleteMultipartUpload = &types.AbortIncompleteMultipartUpload{
				DaysAfterInitiation: aws.Int32(int32(r.AbortIncompleteMultipartUpload.DaysAfterInitiation)),
			}
		}

		rules = append(rules, rule)
	}

	return rules, nil
}

// DeleteBucketLifecycleConfiguration deletes the lifecycle configuration for a bucket.
func (b *B2Backend) DeleteBucketLifecycleConfiguration(ctx context.Context, bucket string) error {
	_, err := b.s3Client.DeleteBucketLifecycle(ctx, &s3.DeleteBucketLifecycleInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		return fmt.Errorf("DeleteBucketLifecycleConfiguration failed: %w", err)
	}
	return nil
}

// GetObjectLockConfiguration gets the object lock configuration for a bucket.
func (b *B2Backend) GetObjectLockConfiguration(ctx context.Context, bucket string) ([]byte, error) {
	output, err := b.s3Client.GetObjectLockConfiguration(ctx, &s3.GetObjectLockConfigurationInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		return nil, fmt.Errorf("GetObjectLockConfiguration failed: %w", err)
	}

	// Build XML response
	if output.ObjectLockConfiguration == nil {
		return []byte(`<?xml version="1.0" encoding="UTF-8"?><ObjectLockConfiguration/>`), nil
	}

	config := output.ObjectLockConfiguration
	var parts []string

	if config.ObjectLockEnabled == types.ObjectLockEnabledEnabled {
		parts = append(parts, "<ObjectLockEnabled>Enabled</ObjectLockEnabled>")
	}

	if config.Rule != nil && config.Rule.DefaultRetention != nil {
		var retentionParts []string
		retention := config.Rule.DefaultRetention

		switch retention.Mode {
		case types.ObjectLockRetentionModeGovernance:
			retentionParts = append(retentionParts, "<Mode>GOVERNANCE</Mode>")
		case types.ObjectLockRetentionModeCompliance:
			retentionParts = append(retentionParts, "<Mode>COMPLIANCE</Mode>")
		}

		if retention.Days != nil {
			retentionParts = append(retentionParts, fmt.Sprintf("<Days>%d</Days>", *retention.Days))
		}
		if retention.Years != nil {
			retentionParts = append(retentionParts, fmt.Sprintf("<Years>%d</Years>", *retention.Years))
		}

		if len(retentionParts) > 0 {
			parts = append(parts, fmt.Sprintf("<Rule><DefaultRetention>%s</DefaultRetention></Rule>", strings.Join(retentionParts, "")))
		}
	}

	xmlContent := strings.Join(parts, "")
	return []byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><ObjectLockConfiguration>%s</ObjectLockConfiguration>`, xmlContent)), nil
}

// PutObjectLockConfiguration sets the object lock configuration for a bucket.
func (b *B2Backend) PutObjectLockConfiguration(ctx context.Context, bucket string, config []byte) error {
	// Parse the object lock configuration XML
	type ObjectLockConfiguration struct {
		XMLName           xml.Name `xml:"ObjectLockConfiguration"`
		ObjectLockEnabled string   `xml:"ObjectLockEnabled"`
		Rule              *struct {
			DefaultRetention *struct {
				Mode  string `xml:"Mode"`
				Days  *int   `xml:"Days"`
				Years *int   `xml:"Years"`
			} `xml:"DefaultRetention"`
		} `xml:"Rule"`
	}

	var olc ObjectLockConfiguration
	if err := xml.Unmarshal(config, &olc); err != nil {
		return fmt.Errorf("failed to parse object lock configuration: %w", err)
	}

	input := &s3.PutObjectLockConfigurationInput{
		Bucket: aws.String(bucket),
	}

	if olc.ObjectLockEnabled == "Enabled" {
		input.ObjectLockConfiguration = &types.ObjectLockConfiguration{
			ObjectLockEnabled: types.ObjectLockEnabledEnabled,
		}

		if olc.Rule != nil && olc.Rule.DefaultRetention != nil {
			dr := &types.DefaultRetention{}
			switch olc.Rule.DefaultRetention.Mode {
			case "GOVERNANCE":
				dr.Mode = types.ObjectLockRetentionModeGovernance
			case "COMPLIANCE":
				dr.Mode = types.ObjectLockRetentionModeCompliance
			}
			if olc.Rule.DefaultRetention.Days != nil {
				dr.Days = aws.Int32(int32(*olc.Rule.DefaultRetention.Days))
			}
			if olc.Rule.DefaultRetention.Years != nil {
				dr.Years = aws.Int32(int32(*olc.Rule.DefaultRetention.Years))
			}
			input.ObjectLockConfiguration.Rule = &types.ObjectLockRule{
				DefaultRetention: dr,
			}
		}
	}

	_, err := b.s3Client.PutObjectLockConfiguration(ctx, input)
	if err != nil {
		return fmt.Errorf("PutObjectLockConfiguration failed: %w", err)
	}
	return nil
}

// GetObjectRetention gets the retention settings for an object.
func (b *B2Backend) GetObjectRetention(ctx context.Context, bucket, key string) ([]byte, error) {
	output, err := b.s3Client.GetObjectRetention(ctx, &s3.GetObjectRetentionInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("GetObjectRetention failed: %w", err)
	}

	// Build XML response
	if output.Retention == nil {
		return []byte(`<?xml version="1.0" encoding="UTF-8"?><Retention/>`), nil
	}

	var parts []string
	retention := output.Retention

	switch retention.Mode {
	case types.ObjectLockRetentionModeGovernance:
		parts = append(parts, "<Mode>GOVERNANCE</Mode>")
	case types.ObjectLockRetentionModeCompliance:
		parts = append(parts, "<Mode>COMPLIANCE</Mode>")
	}

	if retention.RetainUntilDate != nil {
		parts = append(parts, fmt.Sprintf("<RetainUntilDate>%s</RetainUntilDate>", retention.RetainUntilDate.Format(time.RFC3339)))
	}

	xmlContent := strings.Join(parts, "")
	return []byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><Retention>%s</Retention>`, xmlContent)), nil
}

// PutObjectRetention sets the retention settings for an object.
func (b *B2Backend) PutObjectRetention(ctx context.Context, bucket, key string, retention []byte) error {
	// Parse the retention XML
	type Retention struct {
		XMLName         xml.Name `xml:"Retention"`
		Mode            string   `xml:"Mode"`
		RetainUntilDate string   `xml:"RetainUntilDate"`
	}

	var r Retention
	if err := xml.Unmarshal(retention, &r); err != nil {
		return fmt.Errorf("failed to parse retention: %w", err)
	}

	input := &s3.PutObjectRetentionInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	switch r.Mode {
	case "GOVERNANCE":
		input.Retention = &types.ObjectLockRetention{
			Mode: types.ObjectLockRetentionModeGovernance,
		}
	case "COMPLIANCE":
		input.Retention = &types.ObjectLockRetention{
			Mode: types.ObjectLockRetentionModeCompliance,
		}
	}

	if r.RetainUntilDate != "" {
		t, err := time.Parse(time.RFC3339, r.RetainUntilDate)
		if err != nil {
			return fmt.Errorf("failed to parse RetainUntilDate: %w", err)
		}
		if input.Retention == nil {
			input.Retention = &types.ObjectLockRetention{}
		}
		input.Retention.RetainUntilDate = aws.Time(t)
	}

	_, err := b.s3Client.PutObjectRetention(ctx, input)
	if err != nil {
		return fmt.Errorf("PutObjectRetention failed: %w", err)
	}
	return nil
}

// GetObjectLegalHold gets the legal hold status for an object.
func (b *B2Backend) GetObjectLegalHold(ctx context.Context, bucket, key string) ([]byte, error) {
	output, err := b.s3Client.GetObjectLegalHold(ctx, &s3.GetObjectLegalHoldInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("GetObjectLegalHold failed: %w", err)
	}

	// Build XML response
	if output.LegalHold == nil {
		return []byte(`<?xml version="1.0" encoding="UTF-8"?><LegalHold/>`), nil
	}

	status := "OFF"
	if output.LegalHold.Status == types.ObjectLockLegalHoldStatusOn {
		status = "ON"
	}

	return []byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><LegalHold><Status>%s</Status></LegalHold>`, status)), nil
}

// PutObjectLegalHold sets the legal hold status for an object.
func (b *B2Backend) PutObjectLegalHold(ctx context.Context, bucket, key string, legalHold []byte) error {
	// Parse the legal hold XML
	type LegalHold struct {
		XMLName xml.Name `xml:"LegalHold"`
		Status  string   `xml:"Status"`
	}

	var lh LegalHold
	if err := xml.Unmarshal(legalHold, &lh); err != nil {
		return fmt.Errorf("failed to parse legal hold: %w", err)
	}

	input := &s3.PutObjectLegalHoldInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		LegalHold: &types.ObjectLockLegalHold{
			Status: types.ObjectLockLegalHoldStatusOff,
		},
	}

	if lh.Status == "ON" {
		input.LegalHold.Status = types.ObjectLockLegalHoldStatusOn
	}

	_, err := b.s3Client.PutObjectLegalHold(ctx, input)
	if err != nil {
		return fmt.Errorf("PutObjectLegalHold failed: %w", err)
	}
	return nil
}

// ListObjectVersions lists all versions of objects in a bucket.
// It corrects plaintext sizes for ARMOR-encrypted objects.
// ListObjectVersions lists all versions of objects in a bucket.
// When ARMOR_PREFIX is configured, the handler layer prepends the prefix to
// the prefix, keyMarker, and delimiter parameters before calling this method.
// This backend method receives and operates on prefixed keys, and returns
// results with prefixed keys. The handler layer strips the prefix from
// version keys and common prefixes before returning to the client.
// This matches the prefix handling pattern used by ListObjectsV2.
func (b *B2Backend) ListObjectVersions(ctx context.Context, bucket, prefix, delimiter, keyMarker, versionIDMarker string, maxKeys int) (*ListObjectVersionsResult, error) {
	input := &s3.ListObjectVersionsInput{
		Bucket: aws.String(bucket),
	}
	if prefix != "" {
		input.Prefix = aws.String(prefix)
	}
	if delimiter != "" {
		input.Delimiter = aws.String(delimiter)
	}
	if keyMarker != "" {
		input.KeyMarker = aws.String(keyMarker)
	}
	if versionIDMarker != "" {
		input.VersionIdMarker = aws.String(versionIDMarker)
	}
	if maxKeys > 0 {
		input.MaxKeys = aws.Int32(int32(maxKeys))
	}

	resp, err := b.s3Client.ListObjectVersions(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("ListObjectVersions failed: %w", err)
	}

	result := &ListObjectVersionsResult{
		IsTruncated:         aws.ToBool(resp.IsTruncated),
		NextKeyMarker:       aws.ToString(resp.NextKeyMarker),
		NextVersionIDMarker: aws.ToString(resp.NextVersionIdMarker),
	}

	// Process versions
	// Note: ListObjectVersions API does not return metadata or content-type.
	// For ARMOR metadata (plaintext size), caller needs to do HeadObject with VersionId.
	for _, version := range resp.Versions {
		// Filter out .armor/ internal objects (root or ADR-001-prefixed)
		if isInternalKey(b.keyPrefix, aws.ToString(version.Key)) {
			continue
		}

		info := ObjectVersionInfo{
			Key:          aws.ToString(version.Key),
			VersionID:    aws.ToString(version.VersionId),
			Size:         aws.ToInt64(version.Size),
			ETag:         aws.ToString(version.ETag),
			LastModified: aws.ToTime(version.LastModified),
			IsLatest:     aws.ToBool(version.IsLatest),
		}

		result.Versions = append(result.Versions, info)
	}

	// Process delete markers
	for _, marker := range resp.DeleteMarkers {
		// Filter out .armor/ internal objects (root or ADR-001-prefixed)
		if isInternalKey(b.keyPrefix, aws.ToString(marker.Key)) {
			continue
		}

		info := ObjectVersionInfo{
			Key:            aws.ToString(marker.Key),
			VersionID:      aws.ToString(marker.VersionId),
			LastModified:   aws.ToTime(marker.LastModified),
			IsLatest:       aws.ToBool(marker.IsLatest),
			IsDeleteMarker: true,
		}

		result.Versions = append(result.Versions, info)
	}

	// Process common prefixes
	for _, prefix := range resp.CommonPrefixes {
		result.CommonPrefixes = append(result.CommonPrefixes, aws.ToString(prefix.Prefix))
	}

	return result, nil
}

// Ensure bytes.Buffer is used when needed
var _ = bytes.NewReader(nil)
