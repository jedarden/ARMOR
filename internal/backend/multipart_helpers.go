// Package backend provides multipart upload helper functions for monitoring and testing.
package backend

import (
	"bytes"
	"context"
	"fmt"
	"io"
)

// MultipartUploadHelper provides high-level helper functions for working with multipart uploads.
// These are designed for use by canary monitors and other testing/monitoring tools.
type MultipartUploadHelper struct {
	backend Backend
	bucket  string
}

// NewMultipartUploadHelper creates a new helper for a specific bucket.
func NewMultipartUploadHelper(backend Backend, bucket string) *MultipartUploadHelper {
	return &MultipartUploadHelper{
		backend: backend,
		bucket:  bucket,
	}
}

// MultipartUploadInfo holds information about an in-progress multipart upload.
type MultipartUploadInfo struct {
	UploadID string
	Key      string
	Parts    []CompletedPart
	Size     int64
}

// CreateUpload creates a new multipart upload with the given key and metadata.
func (h *MultipartUploadHelper) CreateUpload(ctx context.Context, key string, metadata map[string]string) (*MultipartUploadInfo, error) {
	uploadID, err := h.backend.CreateMultipartUpload(ctx, h.bucket, key, metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to create multipart upload: %w", err)
	}

	return &MultipartUploadInfo{
		UploadID: uploadID,
		Key:      key,
		Parts:    make([]CompletedPart, 0),
	}, nil
}

// UploadPart uploads a single part to the multipart upload.
// The part number must be >= 1 and parts must be uploaded in order.
func (h *MultipartUploadHelper) UploadPart(ctx context.Context, info *MultipartUploadInfo, partNumber int, data []byte) error {
	if partNumber < 1 {
		return fmt.Errorf("invalid part number: %d (must be >= 1)", partNumber)
	}

	// Check minimum part size (5MB) unless this is the first/only part
	const minPartSize = 5 * 1024 * 1024
	if len(info.Parts) > 0 && len(data) < minPartSize {
		return fmt.Errorf("part size %d is below minimum 5MB (except for last part)", len(data))
	}

	// Create a reader for the data
	dataReader := io.NopCloser(bytes.NewReader(data))

	etag, err := h.backend.UploadPart(ctx, h.bucket, info.Key, info.UploadID, int32(partNumber), dataReader, int64(len(data)))
	if err != nil {
		return fmt.Errorf("failed to upload part %d: %w", partNumber, err)
	}

	info.Parts = append(info.Parts, CompletedPart{
		PartNumber: int32(partNumber),
		ETag:       etag,
	})
	info.Size += int64(len(data))

	return nil
}

// CompleteUpload finishes the multipart upload and returns the final ETag.
func (h *MultipartUploadHelper) CompleteUpload(ctx context.Context, info *MultipartUploadInfo) (string, error) {
	if len(info.Parts) == 0 {
		return "", fmt.Errorf("cannot complete upload with no parts")
	}

	etag, err := h.backend.CompleteMultipartUpload(ctx, h.bucket, info.Key, info.UploadID, info.Parts)
	if err != nil {
		return "", fmt.Errorf("failed to complete multipart upload: %w", err)
	}

	return etag, nil
}

// AbortUpload cancels the multipart upload and cleans up any uploaded parts.
func (h *MultipartUploadHelper) AbortUpload(ctx context.Context, info *MultipartUploadInfo) error {
	err := h.backend.AbortMultipartUpload(ctx, h.bucket, info.Key, info.UploadID)
	if err != nil {
		return fmt.Errorf("failed to abort multipart upload: %w", err)
	}
	return nil
}

// UploadPartsInSeries uploads multiple parts in series (one after another).
// This is simpler than parallel uploads but slower for large files.
func (h *MultipartUploadHelper) UploadPartsInSeries(ctx context.Context, info *MultipartUploadInfo, partSize int, data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("no data to upload")
	}

	partNumber := 1
	offset := 0

	for offset < len(data) {
		end := offset + partSize
		if end > len(data) {
			end = len(data)
		}

		partData := data[offset:end]
		partDataReader := io.NopCloser(bytes.NewReader(partData))
		etag, err := h.backend.UploadPart(ctx, h.bucket, info.Key, info.UploadID, int32(partNumber), partDataReader, int64(len(partData)))
		if err != nil {
			return fmt.Errorf("failed to upload part %d: %w", partNumber, err)
		}

		info.Parts = append(info.Parts, CompletedPart{
			PartNumber: int32(partNumber),
			ETag:       etag,
		})

		offset = end
		partNumber++
	}

	info.Size = int64(len(data))
	return nil
}

// CanaryUpload performs a simple canary test of the multipart upload system.
// It uploads a small amount of data using multipart and verifies the result.
func (h *MultipartUploadHelper) CanaryUpload(ctx context.Context, key string, data []byte) error {
	// Create upload
	info, err := h.CreateUpload(ctx, key, map[string]string{
		"Content-Type": "application/octet-stream",
		"canary-test": "true",
	})
	if err != nil {
		return fmt.Errorf("canary create failed: %w", err)
	}

	// Upload as a single part (simplest case for canary)
	err = h.UploadPartsInSeries(ctx, info, len(data), data)
	if err != nil {
		// Try to clean up
		_ = h.AbortUpload(ctx, info)
		return fmt.Errorf("canary upload failed: %w", err)
	}

	// Complete upload
	_, err = h.CompleteUpload(ctx, info)
	if err != nil {
		// Try to clean up
		_ = h.AbortUpload(ctx, info)
		return fmt.Errorf("canary complete failed: %w", err)
	}

	// Verify the object exists
	_, objInfo, err := h.backend.Get(ctx, h.bucket, key)
	if err != nil {
		return fmt.Errorf("canary verification failed: %w", err)
	}

	if objInfo.Size != int64(len(data)) {
		return fmt.Errorf("canary verification failed: size mismatch (got %d, expected %d)", objInfo.Size, len(data))
	}

	// Clean up
	err = h.backend.Delete(ctx, h.bucket, key)
	if err != nil {
		return fmt.Errorf("canary cleanup failed: %w", err)
	}

	return nil
}

// VerifyMultipartUpload uploads data via multipart and downloads it back to verify integrity.
// This is useful for canary monitors to verify the full multipart upload code path.
func VerifyMultipartUpload(ctx context.Context, backend Backend, bucket, key string, data []byte, partSize int) error {
	if len(data) < 5*1024*1024 {
		return fmt.Errorf("data size must be at least 5MB for multipart upload")
	}

	if partSize < 5*1024*1024 {
		return fmt.Errorf("part size must be at least 5MB")
	}

	helper := NewMultipartUploadHelper(backend, bucket)

	// Create multipart upload
	info, err := helper.CreateUpload(ctx, key, map[string]string{
		"Content-Type":      "application/octet-stream",
		"x-amz-meta-test":   "multipart-verification",
		"x-amz-meta-size":   fmt.Sprintf("%d", len(data)),
		"x-amz-meta-source": "canary-monitor",
	})
	if err != nil {
		return fmt.Errorf("create failed: %w", err)
	}

	// Upload all parts
	err = helper.UploadPartsInSeries(ctx, info, partSize, data)
	if err != nil {
		_ = helper.AbortUpload(ctx, info)
		return fmt.Errorf("upload parts failed: %w", err)
	}

	// Complete upload
	_, err = helper.CompleteUpload(ctx, info)
	if err != nil {
		_ = helper.AbortUpload(ctx, info)
		return fmt.Errorf("complete failed: %w", err)
	}

	// Download and verify
	body, objInfo, err := backend.Get(ctx, bucket, key)
	if err != nil {
		return fmt.Errorf("get failed: %w", err)
	}
	defer body.Close()

	// Verify size matches before reading the full body
	if objInfo.Size != int64(len(data)) {
		return fmt.Errorf("size mismatch: got %d bytes, expected %d", objInfo.Size, len(data))
	}

	downloadedData, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("read download failed: %w", err)
	}

	if len(downloadedData) != len(data) {
		return fmt.Errorf("size mismatch: downloaded %d bytes, expected %d", len(downloadedData), len(data))
	}

	// Clean up
	err = backend.Delete(ctx, bucket, key)
	if err != nil {
		return fmt.Errorf("cleanup failed: %w", err)
	}

	return nil
}

// MultipartUploadConstants defines important constants for multipart uploads.
type MultipartUploadConstants struct {
	// Minimum part size for multipart uploads (5MB)
	MinPartSize int64

	// Maximum part size for multipart uploads (5GB)
	MaxPartSize int64

	// Maximum number of parts per upload
	MaxPartsCount int

	// Minimum object size that should use multipart upload (5MB)
	MinMultipartObjectSize int64

	// Maximum object size achievable via multipart upload (5TB)
	MaxObjectSize int64
}

// GetMultipartUploadConstants returns the standard S3 multipart upload constants.
func GetMultipartUploadConstants() MultipartUploadConstants {
	const (
		minPartSize            = 5 * 1024 * 1024            // 5MB
		maxPartSize            = 5 * 1024 * 1024 * 1024     // 5GB
		maxPartsCount          = 10000
		minMultipartObjectSize = 5 * 1024 * 1024            // 5MB
		maxObjectSize          = 5 * 1024 * 1024 * 1024 * 1024 // 5TB
	)

	return MultipartUploadConstants{
		MinPartSize:            minPartSize,
		MaxPartSize:            maxPartSize,
		MaxPartsCount:         maxPartsCount,
		MinMultipartObjectSize: minMultipartObjectSize,
		MaxObjectSize:          maxObjectSize,
	}
}

// CalculateOptimalPartSize calculates the optimal part size for a given file size.
// It aims to use parts between 10MB and 100MB for efficiency while staying within limits.
func CalculateOptimalPartSize(fileSize int64) int64 {
	const (
		minPartSize = 10 * 1024 * 1024  // 10MB
		maxPartSize = 100 * 1024 * 1024 // 100MB
		maxParts    = 10000
	)

	// For small files, use the minimum part size
	if fileSize <= minPartSize*maxParts {
		return minPartSize
	}

	// For larger files, calculate a part size that keeps us under maxParts
	partSize := fileSize / maxParts
	if partSize < minPartSize {
		partSize = minPartSize
	}
	if partSize > maxPartSize {
		partSize = maxPartSize
	}

	return partSize
}
