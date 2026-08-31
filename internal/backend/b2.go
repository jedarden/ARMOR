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
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
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
	readBlockSize   int64
	readConcurrency int
}

const (
	defaultReadBlockSize   = 64 * 1024
	defaultReadConcurrency = 16
)

// B2Config contains configuration for the B2 backend.
type B2Config struct {
	Region          string
	Endpoint        string
	AccessKeyID     string
	SecretKey       string
	CFDomain        string // Cloudflare domain for free egress downloads
	ReadConcurrency int    // Maximum concurrent ranged reads; zero uses the default
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
	})

	readConcurrency := cfg.ReadConcurrency
	if readConcurrency <= 0 {
		readConcurrency = defaultReadConcurrency
	}

	return &B2Backend{
		s3Client:        s3Client,
		region:          cfg.Region,
		endpoint:        cfg.Endpoint,
		cfDomain:        cfg.CFDomain,
		httpClient:      &http.Client{Timeout: 30 * time.Minute},
		readBlockSize:   defaultReadBlockSize,
		readConcurrency: readConcurrency,
	}, nil
}

// Put stores an object in B2.
func (b *B2Backend) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error {
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
	_, err := b.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentLength: aws.Int64(size),
		Metadata:      toS3Metadata(meta),
	})
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

	jobs := make(chan rangedBlockJob)
	results := make(chan rangedBlockResult, workerCount)
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
		pending:     make(map[int64][]byte, workerCount),
	}, headers, nil
}

// fetchRange performs one bounded ranged request and fully buffers its block.
// Buffering is intentional: it lets the caller consume blocks in order while
// later ranged requests complete out of order, without buffering the object.
func (b *B2Backend) fetchRange(ctx context.Context, bucket, key string, offset, length int64) ([]byte, map[string]string, error) {
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
		data, err := readRangeBody(resp.Body, length)
		if err != nil {
			return nil, nil, fmt.Errorf("direct S3 range response: %w", err)
		}
		return data, nil, nil
	}

	// Construct Cloudflare download URL.
	cfURL := fmt.Sprintf("https://%s/file/%s/%s", b.cfDomain, bucket, url.PathEscape(key))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfURL, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Range", rangeHeader)

	httpClient := b.httpClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("cloudflare request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return nil, nil, fmt.Errorf("cloudflare returned status %d", resp.StatusCode)
	}

	data, err := readRangeBody(resp.Body, length)
	if err != nil {
		return nil, nil, fmt.Errorf("cloudflare range response: %w", err)
	}

	return data, cloudflareHeaders(resp), nil
}

func readRangeBody(body io.Reader, expectedLength int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, expectedLength+1))
	if err != nil {
		return nil, fmt.Errorf("read %d bytes: %w", expectedLength, err)
	}
	if int64(len(data)) != expectedLength {
		return nil, fmt.Errorf("received %d bytes, want %d", len(data), expectedLength)
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
	resp, err := b.s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("HeadObject failed: %w", err)
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

// HeadVersion retrieves object metadata for a specific version.
// Returns ARMOR metadata (plaintext size) for the specific version.
func (b *B2Backend) HeadVersion(ctx context.Context, bucket, key, versionID string) (*ObjectInfo, error) {
	resp, err := b.s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket:    aws.String(bucket),
		Key:       aws.String(key),
		VersionId: aws.String(versionID),
	})
	if err != nil {
		return nil, fmt.Errorf("HeadObject failed: %w", err)
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
		// Filter out .armor/ internal objects
		if strings.HasPrefix(aws.ToString(obj.Key), ".armor/") {
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
		CopySource: aws.String(srcBucket + "/" + srcKey),
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

// IsRetryableError checks if an error is retryable.
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}
	// Check for common retryable conditions
	errStr := err.Error()
	return strings.Contains(errStr, "5xx") ||
		strings.Contains(errStr, "503") ||
		strings.Contains(errStr, "502") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "connection reset")
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
		// Filter out .armor/ internal objects
		if strings.HasPrefix(aws.ToString(version.Key), ".armor/") {
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
		// Filter out .armor/ internal objects
		if strings.HasPrefix(aws.ToString(marker.Key), ".armor/") {
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
