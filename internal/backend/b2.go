package backend

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// B2Backend implements the Backend interface using B2's S3 API.
type B2Backend struct {
	s3Client    *s3.Client
	region      string
	endpoint    string
	cfDomain    string
	httpClient  *http.Client
}

// B2Config contains configuration for the B2 backend.
type B2Config struct {
	Region      string
	Endpoint    string
	AccessKeyID string
	SecretKey   string
	CFDomain    string // Cloudflare domain for free egress downloads
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
	})

	return &B2Backend{
		s3Client:   s3Client,
		region:     cfg.Region,
		endpoint:   cfg.Endpoint,
		cfDomain:   cfg.CFDomain,
		httpClient: &http.Client{Timeout: 30 * time.Minute},
	}, nil
}

// Put stores an object in B2.
func (b *B2Backend) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error {
	_, err := b.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentLength: aws.Int64(size),
		Metadata:    toS3Metadata(meta),
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
	body, err := b.GetRange(ctx, bucket, key, 0, info.Size)
	if err != nil {
		return nil, nil, err
	}

	return body, info, nil
}

// GetRange retrieves a byte range from an object via Cloudflare.
func (b *B2Backend) GetRange(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, error) {
	// Construct Cloudflare download URL
	cfURL := fmt.Sprintf("https://%s/file/%s/%s", b.cfDomain, bucket, url.PathEscape(key))

	req, err := http.NewRequestWithContext(ctx, "GET", cfURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set Range header
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", offset, offset+length-1))

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Cloudflare request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		resp.Body.Close()
		return nil, fmt.Errorf("Cloudflare returned status %d", resp.StatusCode)
	}

	return resp.Body, nil
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
		ContentType:  aws.ToString(resp.ContentType),
		ETag:         aws.ToString(resp.ETag),
		LastModified: aws.ToTime(resp.LastModified),
		Metadata:     fromS3Metadata(resp.Metadata),
	}

	// Check if this is an ARMOR-encrypted object
	if _, ok := ParseARMORMetadata(info.Metadata); ok {
		info.IsARMOREncrypted = true
		// Use plaintext size from metadata
		if am, _ := ParseARMORMetadata(info.Metadata); am != nil && am.PlaintextSize > 0 {
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

// Ensure bytes.Buffer is used when needed
var _ = bytes.NewReader(nil)
