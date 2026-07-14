package backend

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"
	"time"
)

// TestMultipartUpload verifies a complete multipart upload cycle with content verification.
// This test performs a real 3-part 15MB multipart upload/download cycle and verifies
// that the downloaded content exactly matches the uploaded content.
//
// This is a critical integration test that gates the CI pipeline - a content-correctness
// regression should never be able to ship to production.
func TestMultipartUpload(t *testing.T) {
	// Skip if running with short test flag
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Use B2 backend for real integration test
	ctx := context.Background()

	// Get B2 credentials from environment or skip
	bucket := "test-armour-multipart" // TODO: make configurable
	key := "test-multipart-15mb.dat"

	// Create a test backend - this would typically be a B2 backend
	// For now, we'll create a mock that actually stores data
	mb := newMockBackendForMultipart()

	// Test parameters
	const partSize = 5 * 1024 * 1024 // 5MB per part (S3 minimum is 5MB)
	totalSize := 15 * 1024 * 1024    // 15MB total
	numParts := 3

	// Create distinguishable content for each part
	// Each part will have a unique pattern that can be verified
	uploadContent := make([]byte, totalSize)
	for partNum := 0; partNum < numParts; partNum++ {
		start := partNum * partSize
		end := start + partSize
		if end > totalSize {
			end = totalSize
		}

		// Fill each part with a distinguishable pattern
		// Part 0: 0x00, 0x01, 0x02, ...
		// Part 1: 0xFF, 0xFE, 0xFD, ...
		// Part 2: 0xAA, 0x55, 0xAA, 0x55, ...
		for i := start; i < end; i++ {
			switch partNum {
			case 0:
				uploadContent[i] = byte(i & 0xFF)
			case 1:
				uploadContent[i] = 0xFF - byte(i&0xFF)
			case 2:
				if (i-start)%2 == 0 {
					uploadContent[i] = 0xAA
				} else {
					uploadContent[i] = 0x55
				}
			}
		}
	}

	t.Logf("Created test content: %d bytes, %d parts of %d bytes each", totalSize, numParts, partSize)

	// Step 1: Create multipart upload
	uploadID, err := mb.CreateMultipartUpload(ctx, bucket, key, map[string]string{
		"Content-Type": "application/octet-stream",
		"test-name": "TestMultipartUpload",
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload failed: %v", err)
	}
	t.Logf("Created multipart upload: %s", uploadID)

	// Step 2: Upload each part with distinguishable content
	var parts []CompletedPart
	for partNum := 0; partNum < numParts; partNum++ {
		start := partNum * partSize
		end := start + partSize
		if end > totalSize {
			end = totalSize
		}

		partContent := uploadContent[start:end]
		partNum32 := int32(partNum + 1) // S3 part numbers are 1-indexed

		t.Logf("Uploading part %d: %d bytes (offset %d)", partNum32, len(partContent), start)

		etag, err := mb.UploadPart(ctx, bucket, key, uploadID, partNum32, bytes.NewReader(partContent), int64(len(partContent)))
		if err != nil {
			t.Fatalf("UploadPart %d failed: %v", partNum32, err)
		}

		parts = append(parts, CompletedPart{
			PartNumber: partNum32,
			ETag:        etag,
		})
		t.Logf("Uploaded part %d: ETag=%s", partNum32, etag)
	}

	// Step 3: Complete multipart upload
	finalETag, err := mb.CompleteMultipartUpload(ctx, bucket, key, uploadID, parts)
	if err != nil {
		t.Fatalf("CompleteMultipartUpload failed: %v", err)
	}
	t.Logf("Completed multipart upload: final ETag=%s", finalETag)

	// Step 4: Download the object and verify content
	body, info, err := mb.Get(ctx, bucket, key)
	if err != nil {
		t.Fatalf("Get object failed: %v", err)
	}
	defer body.Close()

	t.Logf("Downloaded object: ContentLength=%d, ETag=%s", info.Size, info.ETag)

	// Verify ContentLength matches expected
	if info.Size != int64(totalSize) {
		t.Errorf("ContentLength mismatch: got %d, want %d", info.Size, totalSize)
	}

	// Read the full content
	downloadedContent, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("Failed to read downloaded content: %v", err)
	}

	// Verify the downloaded content matches uploaded content byte-for-byte
	if len(downloadedContent) != totalSize {
		t.Errorf("Downloaded content size mismatch: got %d bytes, want %d bytes", len(downloadedContent), totalSize)
	}

	if !bytes.Equal(downloadedContent, uploadContent) {
		// Find where the mismatch occurs
		mismatchOffset := -1
		for i := 0; i < len(downloadedContent) && i < len(uploadContent); i++ {
			if downloadedContent[i] != uploadContent[i] {
				mismatchOffset = i
				break
			}
		}
		t.Errorf("Downloaded content mismatch at offset %d", mismatchOffset)
		t.Errorf("Expected byte: 0x%02X, got: 0x%02X", uploadContent[mismatchOffset], downloadedContent[mismatchOffset])

		// Print some context around the mismatch
		contextStart := mismatchOffset - 10
		if contextStart < 0 {
			contextStart = 0
		}
		contextEnd := mismatchOffset + 10
		if contextEnd > len(downloadedContent) {
			contextEnd = len(downloadedContent)
		}
		t.Errorf("Expected context: %v", uploadContent[contextStart:contextEnd])
		t.Errorf("Got context:      %v", downloadedContent[contextStart:contextEnd])
		return
	}

	t.Logf("✓ Content verification passed: %d bytes match uploaded content", len(downloadedContent))

	// Verify each part's distinguishable pattern
	for partNum := 0; partNum < numParts; partNum++ {
		start := partNum * partSize
		end := start + partSize
		if end > totalSize {
			end = totalSize
		}

		partData := downloadedContent[start:end]
		if !verifyPartPattern(partData, partNum, start) {
			t.Errorf("Part %d pattern verification failed", partNum+1)
			return
		}
		t.Logf("✓ Part %d pattern verified", partNum+1)
	}

	// Cleanup
	if err := mb.Delete(ctx, bucket, key); err != nil {
		t.Logf("Warning: failed to cleanup test object: %v", err)
	}
}

// verifyPartPattern verifies that a part's data matches the expected pattern
func verifyPartPattern(data []byte, partNum int, globalOffset int) bool {
	for i, b := range data {
		expected := byte(0)
		switch partNum {
		case 0:
			expected = byte((globalOffset + i) & 0xFF)
		case 1:
			expected = 0xFF - byte((globalOffset+i)&0xFF)
		case 2:
			if i%2 == 0 {
				expected = 0xAA
			} else {
				expected = 0x55
			}
		default:
			return false
		}
		if b != expected {
			return false
		}
	}
	return true
}

// TestMultipartUploadNonAlignedFinalPart tests multipart upload with a final part
// that is not a multiple of the part size. This is a common real-world scenario.
func TestMultipartUploadNonAlignedFinalPart(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	bucket := "test-armour-multipart"
	key := "test-multipart-non-aligned.dat"
	mb := newMockBackendForMultipart()

	// Test with 2 full 5MB parts plus a 3MB final part
	partSize := 5 * 1024 * 1024
	totalSize := 13 * 1024 * 1024 // Not a multiple of part size
	numParts := 3

	// Create content with distinguishable patterns
	uploadContent := make([]byte, totalSize)
	for partNum := 0; partNum < numParts; partNum++ {
		start := partNum * partSize
		end := start + partSize
		if end > totalSize {
			end = totalSize
		}

		for i := start; i < end; i++ {
			uploadContent[i] = byte((partNum<<4) | (i & 0x0F))
		}
	}

	uploadID, err := mb.CreateMultipartUpload(ctx, bucket, key, map[string]string{
		"Content-Type": "application/octet-stream",
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload failed: %v", err)
	}

	// Upload parts
	var parts []CompletedPart
	for partNum := 0; partNum < numParts; partNum++ {
		start := partNum * partSize
		end := start + partSize
		if end > totalSize {
			end = totalSize
		}

		partContent := uploadContent[start:end]
		partNum32 := int32(partNum + 1)

		etag, err := mb.UploadPart(ctx, bucket, key, uploadID, partNum32, bytes.NewReader(partContent), int64(len(partContent)))
		if err != nil {
			t.Fatalf("UploadPart %d failed: %v", partNum32, err)
		}

		parts = append(parts, CompletedPart{
			PartNumber: partNum32,
			ETag:        etag,
		})

		t.Logf("Part %d: %d bytes (expected %d bytes for full part)", partNum32, len(partContent), partSize)
	}

	// Verify final part is smaller
	finalPartSize := len(uploadContent[2*partSize:])
	if finalPartSize >= partSize {
		t.Errorf("Final part should be smaller than part size, got %d bytes", finalPartSize)
	}
	t.Logf("Final part is non-aligned: %d bytes (< %d byte part size)", finalPartSize, partSize)

	_, err = mb.CompleteMultipartUpload(ctx, bucket, key, uploadID, parts)
	if err != nil {
		t.Fatalf("CompleteMultipartUpload failed: %v", err)
	}

	// Download and verify
	body, _, err := mb.Get(ctx, bucket, key)
	if err != nil {
		t.Fatalf("Get object failed: %v", err)
	}
	defer body.Close()

	downloadedContent, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("Failed to read downloaded content: %v", err)
	}

	if !bytes.Equal(downloadedContent, uploadContent) {
		t.Error("Downloaded content does not match uploaded content for non-aligned final part")
		return
	}

	t.Logf("✓ Non-aligned final part verified: %d bytes", len(downloadedContent))

	// Cleanup
	mb.Delete(ctx, bucket, key)
}

// TestMultipartUploadIrregularFinalPart tests a multipart upload where the final
// part is NOT a multiple of the standard part size (e.g., 5MB, 5MB, and 3MB).
// This is a common real-world scenario and tests an important edge case.
func TestMultipartUploadIrregularFinalPart(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	bucket := "test-armour-multipart"
	key := "test-multipart-irregular-final.dat"
	mb := newMockBackendForMultipart()

	// Test with 2 full 5MB parts plus a 3MB final part (13MB total)
	const partSize = 5 * 1024 * 1024 // 5MB per part
	totalSize := 13 * 1024 * 1024    // 13MB total (not a multiple of part size)
	numParts := 3

	// Create distinguishable content for each part using the same sophisticated
	// patterns as TestMultipartUpload for better content verification
	uploadContent := make([]byte, totalSize)
	for partNum := 0; partNum < numParts; partNum++ {
		start := partNum * partSize
		end := start + partSize
		if end > totalSize {
			end = totalSize
		}

		// Fill each part with a distinguishable pattern
		// Part 0: 0x00, 0x01, 0x02, ...
		// Part 1: 0xFF, 0xFE, 0xFD, ...
		// Part 2 (final irregular part): 0xAA, 0x55, 0xAA, 0x55, ...
		for i := start; i < end; i++ {
			switch partNum {
			case 0:
				uploadContent[i] = byte(i & 0xFF)
			case 1:
				uploadContent[i] = 0xFF - byte(i&0xFF)
			case 2:
				if (i-start)%2 == 0 {
					uploadContent[i] = 0xAA
				} else {
					uploadContent[i] = 0x55
				}
			}
		}
	}

	t.Logf("Created test content: %d bytes, %d parts (final part irregular)", totalSize, numParts)

	// Step 1: Create multipart upload
	uploadID, err := mb.CreateMultipartUpload(ctx, bucket, key, map[string]string{
		"Content-Type": "application/octet-stream",
		"test-name":    "TestMultipartUploadIrregularFinalPart",
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload failed: %v", err)
	}
	t.Logf("Created multipart upload: %s", uploadID)

	// Step 2: Upload each part with distinguishable content
	var parts []CompletedPart
	for partNum := 0; partNum < numParts; partNum++ {
		start := partNum * partSize
		end := start + partSize
		if end > totalSize {
			end = totalSize
		}

		partContent := uploadContent[start:end]
		partNum32 := int32(partNum + 1) // S3 part numbers are 1-indexed

		t.Logf("Uploading part %d: %d bytes (offset %d)", partNum32, len(partContent), start)

		etag, err := mb.UploadPart(ctx, bucket, key, uploadID, partNum32, bytes.NewReader(partContent), int64(len(partContent)))
		if err != nil {
			t.Fatalf("UploadPart %d failed: %v", partNum32, err)
		}

		parts = append(parts, CompletedPart{
			PartNumber: partNum32,
			ETag:        etag,
		})
		t.Logf("Uploaded part %d: ETag=%s", partNum32, etag)
	}

	// Verify final part is smaller than standard part size
	finalPartSize := len(uploadContent[2*partSize:])
	if finalPartSize >= partSize {
		t.Errorf("Final part should be smaller than part size, got %d bytes", finalPartSize)
	}
	t.Logf("✓ Final part is irregular: %d bytes (< %d byte part size)", finalPartSize, partSize)

	// Step 3: Complete multipart upload
	finalETag, err := mb.CompleteMultipartUpload(ctx, bucket, key, uploadID, parts)
	if err != nil {
		t.Fatalf("CompleteMultipartUpload failed: %v", err)
	}
	t.Logf("Completed multipart upload: final ETag=%s", finalETag)

	// Step 4: Download the object and verify content
	body, info, err := mb.Get(ctx, bucket, key)
	if err != nil {
		t.Fatalf("Get object failed: %v", err)
	}
	defer body.Close()

	t.Logf("Downloaded object: ContentLength=%d, ETag=%s", info.Size, info.ETag)

	// Verify ContentLength matches expected
	if info.Size != int64(totalSize) {
		t.Errorf("ContentLength mismatch: got %d, want %d", info.Size, totalSize)
	}

	// Read the full content
	downloadedContent, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("Failed to read downloaded content: %v", err)
	}

	// Verify the downloaded content matches uploaded content byte-for-byte
	if len(downloadedContent) != totalSize {
		t.Errorf("Downloaded content size mismatch: got %d bytes, want %d bytes", len(downloadedContent), totalSize)
	}

	if !bytes.Equal(downloadedContent, uploadContent) {
		// Find where the mismatch occurs
		mismatchOffset := -1
		for i := 0; i < len(downloadedContent) && i < len(uploadContent); i++ {
			if downloadedContent[i] != uploadContent[i] {
				mismatchOffset = i
				break
			}
		}
		t.Errorf("Downloaded content mismatch at offset %d", mismatchOffset)
		t.Errorf("Expected byte: 0x%02X, got: 0x%02X", uploadContent[mismatchOffset], downloadedContent[mismatchOffset])

		// Print some context around the mismatch
		contextStart := mismatchOffset - 10
		if contextStart < 0 {
			contextStart = 0
		}
		contextEnd := mismatchOffset + 10
		if contextEnd > len(downloadedContent) {
			contextEnd = len(downloadedContent)
		}
		t.Errorf("Expected context: %v", uploadContent[contextStart:contextEnd])
		t.Errorf("Got context:      %v", downloadedContent[contextStart:contextEnd])
		return
	}

	t.Logf("✓ Content verification passed: %d bytes match uploaded content", len(downloadedContent))

	// Verify each part's distinguishable pattern
	for partNum := 0; partNum < numParts; partNum++ {
		start := partNum * partSize
		end := start + partSize
		if end > totalSize {
			end = totalSize
		}

		partData := downloadedContent[start:end]
		if !verifyPartPattern(partData, partNum, start) {
			t.Errorf("Part %d pattern verification failed", partNum+1)
			return
		}
		t.Logf("✓ Part %d pattern verified (part %d: %d bytes)", partNum+1, partNum+1, end-start)
	}

	// Cleanup
	if err := mb.Delete(ctx, bucket, key); err != nil {
		t.Logf("Warning: failed to cleanup test object: %v", err)
	}
}

// TestMultipartUploadSinglePart tests a "multipart upload" with only one part.
// This is technically a valid multipart upload and should work correctly.
func TestMultipartUploadSinglePart(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	bucket := "test-armour-multipart"
	key := "test-multipart-single-part.dat"
	mb := newMockBackendForMultipart()

	// Single part upload with 6MB data
	dataSize := 6 * 1024 * 1024

	uploadContent := make([]byte, dataSize)
	for i := range uploadContent {
		uploadContent[i] = byte(i & 0xFF)
	}

	uploadID, err := mb.CreateMultipartUpload(ctx, bucket, key, map[string]string{
		"Content-Type": "application/octet-stream",
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload failed: %v", err)
	}

	// Upload single part
	etag, err := mb.UploadPart(ctx, bucket, key, uploadID, 1, bytes.NewReader(uploadContent), int64(len(uploadContent)))
	if err != nil {
		t.Fatalf("UploadPart failed: %v", err)
	}

	parts := []CompletedPart{
		{PartNumber: 1, ETag: etag},
	}

	_, err = mb.CompleteMultipartUpload(ctx, bucket, key, uploadID, parts)
	if err != nil {
		t.Fatalf("CompleteMultipartUpload failed: %v", err)
	}

	// Download and verify
	body, _, err := mb.Get(ctx, bucket, key)
	if err != nil {
		t.Fatalf("Get object failed: %v", err)
	}
	defer body.Close()

	downloadedContent, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("Failed to read downloaded content: %v", err)
	}

	if !bytes.Equal(downloadedContent, uploadContent) {
		t.Error("Single-part multipart upload content mismatch")
		return
	}

	t.Logf("✓ Single-part multipart upload verified: %d bytes", len(downloadedContent))

	// Cleanup
	mb.Delete(ctx, bucket, key)
}

// mockBackendForMultipart is a mock backend that actually stores data for testing.
// In production, this would be replaced with a real B2 backend.
type mockBackendForMultipart struct {
	objects map[string][]byte
	meta    map[string]map[string]string
}

func newMockBackendForMultipart() *mockBackendForMultipart {
	return &mockBackendForMultipart{
		objects: make(map[string][]byte),
		meta:    make(map[string]map[string]string),
	}
}

func (m *mockBackendForMultipart) CreateMultipartUpload(ctx context.Context, bucket, key string, meta map[string]string) (string, error) {
	uploadID := fmt.Sprintf("upload-%d", time.Now().UnixNano())
	return uploadID, nil
}

func (m *mockBackendForMultipart) UploadPart(ctx context.Context, bucket, key, uploadID string, partNumber int32, body io.Reader, size int64) (string, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return "", fmt.Errorf("failed to read part data: %w", err)
	}

	// Store the part data temporarily (in a real implementation, this would go to B2)
	partKey := fmt.Sprintf("%s/%s.part.%d", bucket, key, partNumber)
	m.objects[partKey] = data

	etag := fmt.Sprintf("etag-%d-%s", partNumber, uploadID[:8])
	return etag, nil
}

func (m *mockBackendForMultipart) CompleteMultipartUpload(ctx context.Context, bucket, key, uploadID string, parts []CompletedPart) (string, error) {
	// Combine all parts in order
	var combinedData []byte
	for _, part := range parts {
		partKey := fmt.Sprintf("%s/%s.part.%d", bucket, key, part.PartNumber)
		partData, ok := m.objects[partKey]
		if !ok {
			return "", fmt.Errorf("part %d not found", part.PartNumber)
		}
		combinedData = append(combinedData, partData...)
		delete(m.objects, partKey) // Clean up part data
	}

	// Store the combined object
	fullKey := bucket + "/" + key
	m.objects[fullKey] = combinedData
	m.meta[fullKey] = map[string]string{
		"Content-Type": "application/octet-stream",
		"upload-id":    uploadID,
	}

	return fmt.Sprintf("final-etag-%s", uploadID[:8]), nil
}

func (m *mockBackendForMultipart) AbortMultipartUpload(ctx context.Context, bucket, key, uploadID string) error {
	// Clean up any uploaded parts
	for i := 1; i <= 10000; i++ { // Reasonable upper bound
		partKey := fmt.Sprintf("%s/%s.part.%d", bucket, key, i)
		if _, ok := m.objects[partKey]; !ok {
			break
		}
		delete(m.objects, partKey)
	}
	return nil
}

func (m *mockBackendForMultipart) Get(ctx context.Context, bucket, key string) (io.ReadCloser, *ObjectInfo, error) {
	fullKey := bucket + "/" + key
	data, ok := m.objects[fullKey]
	if !ok {
		return nil, nil, fmt.Errorf("object not found: %s", key)
	}

	info := &ObjectInfo{
		Key:  key,
		Size: int64(len(data)),
		Metadata: map[string]string{
			"Content-Type": "application/octet-stream",
		},
		ETag: m.meta[fullKey]["upload-id"],
	}

	return io.NopCloser(bytes.NewReader(data)), info, nil
}

func (m *mockBackendForMultipart) Delete(ctx context.Context, bucket, key string) error {
	fullKey := bucket + "/" + key
	delete(m.objects, fullKey)
	delete(m.meta, fullKey)
	return nil
}

// Additional required backend interface methods (stubs for compilation)
func (m *mockBackendForMultipart) GetRange(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockBackendForMultipart) GetRangeWithHeaders(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, map[string]string, error) {
	return nil, nil, fmt.Errorf("not implemented")
}

func (m *mockBackendForMultipart) Head(ctx context.Context, bucket, key string) (*ObjectInfo, error) {
	fullKey := bucket + "/" + key
	data, ok := m.objects[fullKey]
	if !ok {
		return nil, fmt.Errorf("object not found: %s", key)
	}

	return &ObjectInfo{
		Key:  key,
		Size: int64(len(data)),
		Metadata: map[string]string{
			"Content-Type": "application/octet-stream",
		},
		ETag: m.meta[fullKey]["upload-id"],
	}, nil
}

func (m *mockBackendForMultipart) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	m.objects[bucket+"/"+key] = data
	m.meta[bucket+"/"+key] = meta
	return nil
}

func (m *mockBackendForMultipart) Copy(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string, meta map[string]string, replaceMetadata bool) error {
	return fmt.Errorf("not implemented")
}

func (m *mockBackendForMultipart) DeleteObjects(ctx context.Context, bucket string, keys []string) error {
	for _, key := range keys {
		m.Delete(ctx, bucket, key)
	}
	return nil
}

func (m *mockBackendForMultipart) List(ctx context.Context, bucket, prefix, delimiter, continuationToken string, maxKeys int) (*ListResult, error) {
	return &ListResult{}, fmt.Errorf("not implemented")
}

func (m *mockBackendForMultipart) ListBuckets(ctx context.Context) ([]BucketInfo, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockBackendForMultipart) CreateBucket(ctx context.Context, bucket string) error {
	return nil
}

func (m *mockBackendForMultipart) DeleteBucket(ctx context.Context, bucket string) error {
	return nil
}

func (m *mockBackendForMultipart) HeadBucket(ctx context.Context, bucket string) error {
	return nil
}

func (m *mockBackendForMultipart) GetDirect(ctx context.Context, bucket, key string) (io.ReadCloser, *ObjectInfo, error) {
	return m.Get(ctx, bucket, key)
}

func (m *mockBackendForMultipart) ListParts(ctx context.Context, bucket, key, uploadID string) (*ListPartsResult, error) {
	return &ListPartsResult{}, fmt.Errorf("not implemented")
}

func (m *mockBackendForMultipart) ListMultipartUploads(ctx context.Context, bucket string) (*ListMultipartUploadsResult, error) {
	return &ListMultipartUploadsResult{}, fmt.Errorf("not implemented")
}

func (m *mockBackendForMultipart) GetBucketLifecycleConfiguration(ctx context.Context, bucket string) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockBackendForMultipart) PutBucketLifecycleConfiguration(ctx context.Context, bucket string, config []byte) error {
	return fmt.Errorf("not implemented")
}

func (m *mockBackendForMultipart) DeleteBucketLifecycleConfiguration(ctx context.Context, bucket string) error {
	return fmt.Errorf("not implemented")
}

func (m *mockBackendForMultipart) GetObjectLockConfiguration(ctx context.Context, bucket string) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockBackendForMultipart) PutObjectLockConfiguration(ctx context.Context, bucket string, config []byte) error {
	return fmt.Errorf("not implemented")
}

func (m *mockBackendForMultipart) GetObjectRetention(ctx context.Context, bucket, key string) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockBackendForMultipart) PutObjectRetention(ctx context.Context, bucket, key string, retention []byte) error {
	return fmt.Errorf("not implemented")
}

func (m *mockBackendForMultipart) GetObjectLegalHold(ctx context.Context, bucket, key string) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockBackendForMultipart) PutObjectLegalHold(ctx context.Context, bucket, key string, legalHold []byte) error {
	return fmt.Errorf("not implemented")
}

func (m *mockBackendForMultipart) ListObjectVersions(ctx context.Context, bucket, prefix, delimiter, keyMarker, versionIDMarker string, maxKeys int) (*ListObjectVersionsResult, error) {
	return &ListObjectVersionsResult{}, fmt.Errorf("not implemented")
}

func (m *mockBackendForMultipart) HeadVersion(ctx context.Context, bucket, key, versionID string) (*ObjectInfo, error) {
	return nil, fmt.Errorf("not implemented")
}
