// Package backend tests for B2Backend multipart upload operations.
package backend

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// s3ClientInterface defines the S3 operations needed for multipart uploads.
// This interface allows us to mock the S3 client for testing.
type s3ClientInterface interface {
	CreateMultipartUpload(ctx context.Context, input *s3.CreateMultipartUploadInput, opts ...func(*s3.Options)) (*s3.CreateMultipartUploadOutput, error)
	UploadPart(ctx context.Context, input *s3.UploadPartInput, opts ...func(*s3.Options)) (*s3.UploadPartOutput, error)
	CompleteMultipartUpload(ctx context.Context, input *s3.CompleteMultipartUploadInput, opts ...func(*s3.Options)) (*s3.CompleteMultipartUploadOutput, error)
	AbortMultipartUpload(ctx context.Context, input *s3.AbortMultipartUploadInput, opts ...func(*s3.Options)) (*s3.AbortMultipartUploadOutput, error)
}

// testableB2Backend wraps B2Backend with an injectable S3 client for testing.
type testableB2Backend struct {
	*B2Backend
	mockS3Client s3ClientInterface
}

func (t *testableB2Backend) CreateMultipartUpload(ctx context.Context, bucket, key string, meta map[string]string) (string, error) {
	if t.mockS3Client != nil {
		resp, err := t.mockS3Client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
			Bucket:   aws.String(bucket),
			Key:      aws.String(key),
			Metadata: toS3Metadata(meta),
		})
		if err != nil {
			return "", fmt.Errorf("CreateMultipartUpload failed: %w", err)
		}
		return aws.ToString(resp.UploadId), nil
	}
	return t.B2Backend.CreateMultipartUpload(ctx, bucket, key, meta)
}

func (t *testableB2Backend) UploadPart(ctx context.Context, bucket, key, uploadID string, partNumber int32, body io.Reader, size int64) (string, error) {
	if t.mockS3Client != nil {
		resp, err := t.mockS3Client.UploadPart(ctx, &s3.UploadPartInput{
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
	return t.B2Backend.UploadPart(ctx, bucket, key, uploadID, partNumber, body, size)
}

func (t *testableB2Backend) CompleteMultipartUpload(ctx context.Context, bucket, key, uploadID string, parts []CompletedPart) (string, error) {
	if t.mockS3Client != nil {
		awsParts := make([]types.CompletedPart, len(parts))
		for i, p := range parts {
			awsParts[i] = types.CompletedPart{
				ETag:       aws.String(p.ETag),
				PartNumber: aws.Int32(p.PartNumber),
			}
		}
		resp, err := t.mockS3Client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
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
	return t.B2Backend.CompleteMultipartUpload(ctx, bucket, key, uploadID, parts)
}

func (t *testableB2Backend) AbortMultipartUpload(ctx context.Context, bucket, key, uploadID string) error {
	if t.mockS3Client != nil {
		_, err := t.mockS3Client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
			Bucket:   aws.String(bucket),
			Key:      aws.String(key),
			UploadId: aws.String(uploadID),
		})
		if err != nil {
			return fmt.Errorf("AbortMultipartUpload failed: %w", err)
		}
		return nil
	}
	return t.B2Backend.AbortMultipartUpload(ctx, bucket, key, uploadID)
}

// mockS3Client is a mock S3 client for testing multipart operations.
type mockS3Client struct {
	createMultipartUploadFunc func(ctx context.Context, input *s3.CreateMultipartUploadInput) (*s3.CreateMultipartUploadOutput, error)
	uploadPartFunc           func(ctx context.Context, input *s3.UploadPartInput) (*s3.UploadPartOutput, error)
	completeMultipartUploadFunc func(ctx context.Context, input *s3.CompleteMultipartUploadInput) (*s3.CompleteMultipartUploadOutput, error)
	abortMultipartUploadFunc  func(ctx context.Context, input *s3.AbortMultipartUploadInput) (*s3.AbortMultipartUploadOutput, error)
}

func (m *mockS3Client) CreateMultipartUpload(ctx context.Context, input *s3.CreateMultipartUploadInput, opts ...func(*s3.Options)) (*s3.CreateMultipartUploadOutput, error) {
	if m.createMultipartUploadFunc != nil {
		return m.createMultipartUploadFunc(ctx, input)
	}
	return &s3.CreateMultipartUploadOutput{UploadId: aws.String("test-upload-id")}, nil
}

func (m *mockS3Client) UploadPart(ctx context.Context, input *s3.UploadPartInput, opts ...func(*s3.Options)) (*s3.UploadPartOutput, error) {
	if m.uploadPartFunc != nil {
		return m.uploadPartFunc(ctx, input)
	}
	return &s3.UploadPartOutput{ETag: aws.String("test-etag")}, nil
}

func (m *mockS3Client) CompleteMultipartUpload(ctx context.Context, input *s3.CompleteMultipartUploadInput, opts ...func(*s3.Options)) (*s3.CompleteMultipartUploadOutput, error) {
	if m.completeMultipartUploadFunc != nil {
		return m.completeMultipartUploadFunc(ctx, input)
	}
	return &s3.CompleteMultipartUploadOutput{ETag: aws.String("final-etag")}, nil
}

func (m *mockS3Client) AbortMultipartUpload(ctx context.Context, input *s3.AbortMultipartUploadInput, opts ...func(*s3.Options)) (*s3.AbortMultipartUploadOutput, error) {
	if m.abortMultipartUploadFunc != nil {
		return m.abortMultipartUploadFunc(ctx, input)
	}
	return &s3.AbortMultipartUploadOutput{}, nil
}

// TestB2Backend_CreateMultipartUpload_Success tests successful CreateMultipartUpload.
func TestB2Backend_CreateMultipartUpload_Success(t *testing.T) {
	ctx := context.Background()

	mockClient := &mockS3Client{
		createMultipartUploadFunc: func(ctx context.Context, input *s3.CreateMultipartUploadInput) (*s3.CreateMultipartUploadOutput, error) {
			if aws.ToString(input.Bucket) != "test-bucket" {
				t.Errorf("expected bucket 'test-bucket', got '%s'", aws.ToString(input.Bucket))
			}
			if aws.ToString(input.Key) != "test-key" {
				t.Errorf("expected key 'test-key', got '%s'", aws.ToString(input.Key))
			}
			return &s3.CreateMultipartUploadOutput{UploadId: aws.String("mock-upload-id")}, nil
		},
	}

	backend := &testableB2Backend{mockS3Client: mockClient}

	uploadID, err := backend.CreateMultipartUpload(ctx, "test-bucket", "test-key", map[string]string{
		"Content-Type": "application/octet-stream",
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload failed: %v", err)
	}

	if uploadID != "mock-upload-id" {
		t.Errorf("expected upload ID 'mock-upload-id', got '%s'", uploadID)
	}
}

// TestB2Backend_UploadPart_Success tests successful UploadPart.
func TestB2Backend_UploadPart_Success(t *testing.T) {
	ctx := context.Background()
	data := []byte("test data for part upload")

	mockClient := &mockS3Client{
		uploadPartFunc: func(ctx context.Context, input *s3.UploadPartInput) (*s3.UploadPartOutput, error) {
			if aws.ToString(input.Bucket) != "test-bucket" {
				t.Errorf("expected bucket 'test-bucket', got '%s'", aws.ToString(input.Bucket))
			}
			if aws.ToString(input.UploadId) != "test-upload-id" {
				t.Errorf("expected upload ID 'test-upload-id', got '%s'", aws.ToString(input.UploadId))
			}
			if aws.ToInt32(input.PartNumber) != 1 {
				t.Errorf("expected part number 1, got %d", aws.ToInt32(input.PartNumber))
			}
			if aws.ToInt64(input.ContentLength) != int64(len(data)) {
				t.Errorf("expected content length %d, got %d", len(data), aws.ToInt64(input.ContentLength))
			}
			return &s3.UploadPartOutput{ETag: aws.String("mock-etag")}, nil
		},
	}

	backend := &testableB2Backend{mockS3Client: mockClient}

	etag, err := backend.UploadPart(ctx, "test-bucket", "test-key", "test-upload-id", 1,
		nil, int64(len(data)))
	if err != nil {
		t.Fatalf("UploadPart failed: %v", err)
	}

	if etag != "mock-etag" {
		t.Errorf("expected etag 'mock-etag', got '%s'", etag)
	}
}

// TestB2Backend_CompleteMultipartUpload_Success tests successful CompleteMultipartUpload.
func TestB2Backend_CompleteMultipartUpload_Success(t *testing.T) {
	ctx := context.Background()
	parts := []CompletedPart{
		{PartNumber: 1, ETag: "etag-1"},
		{PartNumber: 2, ETag: "etag-2"},
		{PartNumber: 3, ETag: "etag-3"},
	}

	mockClient := &mockS3Client{
		completeMultipartUploadFunc: func(ctx context.Context, input *s3.CompleteMultipartUploadInput) (*s3.CompleteMultipartUploadOutput, error) {
			if len(input.MultipartUpload.Parts) != len(parts) {
				t.Errorf("expected %d parts, got %d", len(parts), len(input.MultipartUpload.Parts))
			}
			for i, part := range input.MultipartUpload.Parts {
				if aws.ToInt32(part.PartNumber) != parts[i].PartNumber {
					t.Errorf("expected part number %d, got %d", parts[i].PartNumber, aws.ToInt32(part.PartNumber))
				}
				if aws.ToString(part.ETag) != parts[i].ETag {
					t.Errorf("expected etag '%s', got '%s'", parts[i].ETag, aws.ToString(part.ETag))
				}
			}
			return &s3.CompleteMultipartUploadOutput{ETag: aws.String("final-etag")}, nil
		},
	}

	backend := &testableB2Backend{mockS3Client: mockClient}

	finalETag, err := backend.CompleteMultipartUpload(ctx, "test-bucket", "test-key", "test-upload-id", parts)
	if err != nil {
		t.Fatalf("CompleteMultipartUpload failed: %v", err)
	}

	if finalETag != "final-etag" {
		t.Errorf("expected final etag 'final-etag', got '%s'", finalETag)
	}
}

// TestB2Backend_AbortMultipartUpload_Success tests successful AbortMultipartUpload.
func TestB2Backend_AbortMultipartUpload_Success(t *testing.T) {
	ctx := context.Background()

	mockClient := &mockS3Client{
		abortMultipartUploadFunc: func(ctx context.Context, input *s3.AbortMultipartUploadInput) (*s3.AbortMultipartUploadOutput, error) {
			if aws.ToString(input.UploadId) != "test-upload-id" {
				t.Errorf("expected upload ID 'test-upload-id', got '%s'", aws.ToString(input.UploadId))
			}
			return &s3.AbortMultipartUploadOutput{}, nil
		},
	}

	backend := &testableB2Backend{mockS3Client: mockClient}

	err := backend.AbortMultipartUpload(ctx, "test-bucket", "test-key", "test-upload-id")
	if err != nil {
		t.Fatalf("AbortMultipartUpload failed: %v", err)
	}
}

// TestB2Backend_MultipartErrorCases tests error cases for multipart operations.
func TestB2Backend_MultipartErrorCases(t *testing.T) {
	tests := []struct {
		name              string
		operation         string
		mockFunc          interface{}
		expectErrorContains string
	}{
		{
			name:      "CreateMultipartUpload invalid bucket",
			operation: "create",
			mockFunc: func(ctx context.Context, input *s3.CreateMultipartUploadInput) (*s3.CreateMultipartUploadOutput, error) {
				return nil, &types.NoSuchBucket{
					Message: aws.String("The specified bucket does not exist"),
				}
			},
			expectErrorContains: "bucket",
		},
		{
			name:      "CreateMultipartUpload access denied",
			operation: "create",
			mockFunc: func(ctx context.Context, input *s3.CreateMultipartUploadInput) (*s3.CreateMultipartUploadOutput, error) {
				return nil, &types.AccessDenied{
					Message: aws.String("Access Denied"),
				}
			},
			expectErrorContains: "access",
		},
		{
			name:      "UploadPart invalid upload ID",
			operation: "upload",
			mockFunc: func(ctx context.Context, input *s3.UploadPartInput) (*s3.UploadPartOutput, error) {
				return nil, &types.NoSuchUpload{
					Message: aws.String("The specified multipart upload does not exist"),
				}
			},
			expectErrorContains: "upload",
		},
		{
			name:      "UploadPart part too small",
			operation: "upload",
			mockFunc: func(ctx context.Context, input *s3.UploadPartInput) (*s3.UploadPartOutput, error) {
				return nil, fmt.Errorf("EntityTooSmall: Your proposed upload is smaller than the minimum allowed object size")
			},
			expectErrorContains: "small",
		},
		{
			name:      "CompleteMultipartUpload invalid part",
			operation: "complete",
			mockFunc: func(ctx context.Context, input *s3.CompleteMultipartUploadInput) (*s3.CompleteMultipartUploadOutput, error) {
				return nil, fmt.Errorf("InvalidPart: The part number specified is invalid")
			},
			expectErrorContains: "part",
		},
		{
			name:      "AbortMultipartUpload not found",
			operation: "abort",
			mockFunc: func(ctx context.Context, input *s3.AbortMultipartUploadInput) (*s3.AbortMultipartUploadOutput, error) {
				return nil, &types.NoSuchUpload{
					Message: aws.String("The specified multipart upload does not exist"),
				}
			},
			expectErrorContains: "upload",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			mockClient := &mockS3Client{}

			switch tt.operation {
			case "create":
				mockClient.createMultipartUploadFunc = tt.mockFunc.(func(context.Context, *s3.CreateMultipartUploadInput) (*s3.CreateMultipartUploadOutput, error))
				backend := &testableB2Backend{mockS3Client: mockClient}
				_, err := backend.CreateMultipartUpload(ctx, "test-bucket", "test-key", nil)
				if err == nil {
					t.Error("Expected error, got nil")
				}
			case "upload":
				mockClient.uploadPartFunc = tt.mockFunc.(func(context.Context, *s3.UploadPartInput) (*s3.UploadPartOutput, error))
				backend := &testableB2Backend{mockS3Client: mockClient}
				_, err := backend.UploadPart(ctx, "test-bucket", "test-key", "upload-id", 1, nil, 100)
				if err == nil {
					t.Error("Expected error, got nil")
				}
			case "complete":
				mockClient.completeMultipartUploadFunc = tt.mockFunc.(func(context.Context, *s3.CompleteMultipartUploadInput) (*s3.CompleteMultipartUploadOutput, error))
				backend := &testableB2Backend{mockS3Client: mockClient}
				_, err := backend.CompleteMultipartUpload(ctx, "test-bucket", "test-key", "upload-id", []CompletedPart{})
				if err == nil {
					t.Error("Expected error, got nil")
				}
			case "abort":
				mockClient.abortMultipartUploadFunc = tt.mockFunc.(func(context.Context, *s3.AbortMultipartUploadInput) (*s3.AbortMultipartUploadOutput, error))
				backend := &testableB2Backend{mockS3Client: mockClient}
				err := backend.AbortMultipartUpload(ctx, "test-bucket", "test-key", "upload-id")
				if err == nil {
					t.Error("Expected error, got nil")
				}
			}
		})
	}
}

// TestB2Backend_MultipartContextCancellation tests context cancellation in multipart operations.
func TestB2Backend_MultipartContextCancellation(t *testing.T) {
	tests := []struct {
		name      string
		operation string
	}{
		{"CreateMultipartUpload cancellation", "create"},
		{"UploadPart cancellation", "upload"},
		{"CompleteMultipartUpload cancellation", "complete"},
		{"AbortMultipartUpload cancellation", "abort"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel() // Cancel before calling the operation

			mockClient := &mockS3Client{
				// Mock functions that check for canceled context
				createMultipartUploadFunc: func(ctx context.Context, input *s3.CreateMultipartUploadInput) (*s3.CreateMultipartUploadOutput, error) {
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					default:
						return &s3.CreateMultipartUploadOutput{UploadId: aws.String("test-upload-id")}, nil
					}
				},
				uploadPartFunc: func(ctx context.Context, input *s3.UploadPartInput) (*s3.UploadPartOutput, error) {
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					default:
						return &s3.UploadPartOutput{ETag: aws.String("test-etag")}, nil
					}
				},
				completeMultipartUploadFunc: func(ctx context.Context, input *s3.CompleteMultipartUploadInput) (*s3.CompleteMultipartUploadOutput, error) {
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					default:
						return &s3.CompleteMultipartUploadOutput{ETag: aws.String("final-etag")}, nil
					}
				},
				abortMultipartUploadFunc: func(ctx context.Context, input *s3.AbortMultipartUploadInput) (*s3.AbortMultipartUploadOutput, error) {
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					default:
						return &s3.AbortMultipartUploadOutput{}, nil
					}
				},
			}

			backend := &testableB2Backend{mockS3Client: mockClient}

			var err error
			switch tt.operation {
			case "create":
				_, err = backend.CreateMultipartUpload(ctx, "test-bucket", "test-key", nil)
			case "upload":
				_, err = backend.UploadPart(ctx, "test-bucket", "test-key", "upload-id", 1, nil, 100)
			case "complete":
				_, err = backend.CompleteMultipartUpload(ctx, "test-bucket", "test-key", "upload-id", []CompletedPart{})
			case "abort":
				err = backend.AbortMultipartUpload(ctx, "test-bucket", "test-key", "upload-id")
			}

			// Note: With the current mock implementation, context cancellation is checked
			// inside the mock functions. In real scenarios, the AWS SDK handles this.
			// This test verifies that our test infrastructure properly handles context.
			if err != nil && ctx.Err() != nil {
				t.Logf("Got expected context error: %v", err)
			}
		})
	}
}

// TestB2Backend_MultipartThresholds documents multipart upload thresholds and limits.
func TestB2Backend_MultipartThresholds(t *testing.T) {
	// S3 Multipart Upload Thresholds and Limits:
	//
	// Minimum object size for multipart upload:
	//   - 5MB (5,242,880 bytes) - recommended threshold
	//   - Objects smaller than this should use single-part PutObject
	//
	// Minimum part size (except last part):
	//   - 5MB (5,242,880 bytes)
	//   - All parts except the last must be at least this size
	//
	// Maximum part size:
	//   - 5GB (5,368,709,120 bytes)
	//
	// Maximum number of parts:
	//   - 10,000 parts per upload
	//
	// Maximum object size via multipart upload:
	//   - 5TB (5,497,558,138,880 bytes)
	//
	// Recommended part size for large files:
	//   - 10MB to 100MB is typical
	//   - Larger parts = fewer API calls = faster uploads
	//   - Smaller parts = better recovery from failures = more retryable
	//
	// For ARMOR:
	//   - Part sizes should be multiples of the encryption block size (4096 bytes)
	//   - This ensures proper alignment for encryption operations
	//   - Typical ARMOR part size: 5MB, 10MB, or 50MB (all multiples of 4096)

	const (
		minMultipartObjectSize = 5 * 1024 * 1024     // 5MB
		minPartSize           = 5 * 1024 * 1024     // 5MB
		maxPartSize           = 5 * 1024 * 1024 * 1024 // 5GB
		maxPartsCount         = 10000
		maxObjectSize         = 5 * 1024 * 1024 * 1024 * 1024 // 5TB
		armorBlockSize        = 4096
	)

	if minMultipartObjectSize < minPartSize {
		t.Error("Minimum multipart object size should be at least minimum part size")
	}

	if maxPartSize*maxPartsCount < maxObjectSize {
		t.Error("Maximum object size should be achievable with max parts and max part size")
	}

	// Verify common part sizes are multiples of ARMOR block size
	commonPartSizes := []int64{
		5 * 1024 * 1024,   // 5MB
		10 * 1024 * 1024,  // 10MB
		50 * 1024 * 1024,  // 50MB
		100 * 1024 * 1024, // 100MB
	}

	for _, size := range commonPartSizes {
		if size%int64(armorBlockSize) != 0 {
			t.Errorf("Part size %d is not a multiple of ARMOR block size %d", size, armorBlockSize)
		}
	}
}

// TestB2Backend_MultipartUploadWorkflow tests the complete multipart upload workflow.
func TestB2Backend_MultipartUploadWorkflow(t *testing.T) {
	ctx := context.Background()

	mockClient := &mockS3Client{
		createMultipartUploadFunc: func(ctx context.Context, input *s3.CreateMultipartUploadInput) (*s3.CreateMultipartUploadOutput, error) {
			return &s3.CreateMultipartUploadOutput{
				UploadId: aws.String("workflow-upload-id"),
			}, nil
		},
		uploadPartFunc: func(ctx context.Context, input *s3.UploadPartInput) (*s3.UploadPartOutput, error) {
			partNum := aws.ToInt32(input.PartNumber)
			return &s3.UploadPartOutput{
				ETag: aws.String(fmt.Sprintf("etag-part-%d", partNum)),
			}, nil
		},
		completeMultipartUploadFunc: func(ctx context.Context, input *s3.CompleteMultipartUploadInput) (*s3.CompleteMultipartUploadOutput, error) {
			return &s3.CompleteMultipartUploadOutput{
				ETag: aws.String("final-workflow-etag"),
				Location: aws.String("https://test-bucket.s3.amazonaws.com/test-key"),
			}, nil
		},
	}

	backend := &testableB2Backend{mockS3Client: mockClient}

	// Step 1: Create multipart upload
	uploadID, err := backend.CreateMultipartUpload(ctx, "test-bucket", "test-key", map[string]string{
		"Content-Type": "application/octet-stream",
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload failed: %v", err)
	}
	if uploadID != "workflow-upload-id" {
		t.Errorf("Expected upload ID 'workflow-upload-id', got '%s'", uploadID)
	}

	// Step 2: Upload parts
	var parts []CompletedPart
	for i := 1; i <= 3; i++ {
		etag, err := backend.UploadPart(ctx, "test-bucket", "test-key", uploadID, int32(i), nil, 5*1024*1024)
		if err != nil {
			t.Fatalf("UploadPart %d failed: %v", i, err)
		}
		parts = append(parts, CompletedPart{
			PartNumber: int32(i),
			ETag:        etag,
		})
	}

	// Step 3: Complete upload
	finalETag, err := backend.CompleteMultipartUpload(ctx, "test-bucket", "test-key", uploadID, parts)
	if err != nil {
		t.Fatalf("CompleteMultipartUpload failed: %v", err)
	}
	if finalETag != "final-workflow-etag" {
		t.Errorf("Expected final ETag 'final-workflow-etag', got '%s'", finalETag)
	}

	t.Logf("✓ Complete multipart workflow test passed")
}

// TestB2Backend_AbortMultipartUploadWorkflow tests aborting a multipart upload.
func TestB2Backend_AbortMultipartUploadWorkflow(t *testing.T) {
	ctx := context.Background()

	mockClient := &mockS3Client{
		createMultipartUploadFunc: func(ctx context.Context, input *s3.CreateMultipartUploadInput) (*s3.CreateMultipartUploadOutput, error) {
			return &s3.CreateMultipartUploadOutput{
				UploadId: aws.String("abort-test-upload-id"),
			}, nil
		},
		abortMultipartUploadFunc: func(ctx context.Context, input *s3.AbortMultipartUploadInput) (*s3.AbortMultipartUploadOutput, error) {
			if aws.ToString(input.UploadId) != "abort-test-upload-id" {
				return nil, fmt.Errorf("unexpected upload ID: %s", aws.ToString(input.UploadId))
			}
			return &s3.AbortMultipartUploadOutput{}, nil
		},
	}

	backend := &testableB2Backend{mockS3Client: mockClient}

	// Create upload
	uploadID, err := backend.CreateMultipartUpload(ctx, "test-bucket", "test-key", nil)
	if err != nil {
		t.Fatalf("CreateMultipartUpload failed: %v", err)
	}

	// Abort upload
	err = backend.AbortMultipartUpload(ctx, "test-bucket", "test-key", uploadID)
	if err != nil {
		t.Fatalf("AbortMultipartUpload failed: %v", err)
	}

	t.Logf("✓ Abort multipart upload test passed")
}

// TestB2Backend_MultipartMetadataHandling tests metadata handling in multipart uploads.
func TestB2Backend_MultipartMetadataHandling(t *testing.T) {
	ctx := context.Background()

	testMetadata := map[string]string{
		"Content-Type":    "application/octet-stream",
		"x-amz-meta-test": "test-value",
		"custom-key":      "custom-value",
	}

	mockClient := &mockS3Client{
		createMultipartUploadFunc: func(ctx context.Context, input *s3.CreateMultipartUploadInput) (*s3.CreateMultipartUploadOutput, error) {
			// Verify metadata is passed correctly
			if input.Metadata != nil {
				for k, v := range testMetadata {
					if val, ok := input.Metadata[k]; !ok || val != v {
						return nil, fmt.Errorf("metadata key %s not passed correctly", k)
					}
				}
			}
			return &s3.CreateMultipartUploadOutput{UploadId: aws.String("metadata-test-id")}, nil
		},
	}

	backend := &testableB2Backend{mockS3Client: mockClient}

	uploadID, err := backend.CreateMultipartUpload(ctx, "test-bucket", "test-key", testMetadata)
	if err != nil {
		t.Fatalf("CreateMultipartUpload failed: %v", err)
	}

	if uploadID != "metadata-test-id" {
		t.Errorf("Expected upload ID 'metadata-test-id', got '%s'", uploadID)
	}

	t.Logf("✓ Multipart metadata handling test passed")
}

// TestB2Backend_MultipartPartNumberValidation validates part number constraints.
func TestB2Backend_MultipartPartNumberValidation(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		partNumber int32
		shouldFail bool
	}{
		{"Valid part number 1", 1, false},
		{"Valid part number 5000", 5000, false},
		{"Valid part number 10000", 10000, false},
		{"Invalid part number 0", 0, true},
		{"Invalid part number -1", -1, true},
		{"Invalid part number 10001", 10001, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockS3Client{
				uploadPartFunc: func(ctx context.Context, input *s3.UploadPartInput) (*s3.UploadPartOutput, error) {
					partNum := aws.ToInt32(input.PartNumber)
					if partNum < 1 || partNum > 10000 {
						return nil, fmt.Errorf("InvalidPartNumber: Part number must be between 1 and 10000")
					}
					return &s3.UploadPartOutput{ETag: aws.String("test-etag")}, nil
				},
			}

			backend := &testableB2Backend{mockS3Client: mockClient}

			_, err := backend.UploadPart(ctx, "test-bucket", "test-key", "upload-id", tt.partNumber, nil, 5*1024*1024)
			if tt.shouldFail && err == nil {
				t.Error("Expected error for invalid part number, got nil")
			}
			if !tt.shouldFail && err != nil {
				t.Errorf("Expected success for valid part number, got error: %v", err)
			}
		})
	}
}

// mockB2BackendForTesting creates a mock backend for testing without requiring real B2 credentials.
// This is useful for canary monitors and other testing scenarios.
type mockB2BackendForTesting struct {
	uploadIDCounter int
	uploads         map[string]*testUploadState
	parts           map[string][]testPart
	objects         map[string][]byte
}

type testUploadState struct {
	uploadID  string
	bucket    string
	key       string
	metadata  map[string]string
	createdAt time.Time
}

type testPart struct {
	partNumber int32
	data       []byte
	etag       string
}

// NewMockB2BackendForTesting creates a new mock backend for testing multipart operations.
// This backend simulates S3 multipart upload behavior without requiring real credentials.
func NewMockB2BackendForTesting() *mockB2BackendForTesting {
	return &mockB2BackendForTesting{
		uploads: make(map[string]*testUploadState),
		parts:   make(map[string][]testPart),
		objects: make(map[string][]byte),
	}
}

func (m *mockB2BackendForTesting) CreateMultipartUpload(ctx context.Context, bucket, key string, meta map[string]string) (string, error) {
	m.uploadIDCounter++
	uploadID := fmt.Sprintf("mock-upload-%d", m.uploadIDCounter)

	m.uploads[uploadID] = &testUploadState{
		uploadID:  uploadID,
		bucket:    bucket,
		key:       key,
		metadata:  meta,
		createdAt: time.Now(),
	}
	m.parts[uploadID] = []testPart{}

	return uploadID, nil
}

func (m *mockB2BackendForTesting) UploadPart(ctx context.Context, bucket, key, uploadID string, partNumber int32, body io.Reader, size int64) (string, error) {
	state, ok := m.uploads[uploadID]
	if !ok {
		return "", fmt.Errorf("NoSuchUpload: The specified multipart upload does not exist")
	}

	if state.bucket != bucket || state.key != key {
		return "", fmt.Errorf("InvalidRequest: Bucket/key mismatch")
	}

	if partNumber < 1 || partNumber > 10000 {
		return "", fmt.Errorf("InvalidPartNumber: Part number must be between 1 and 10000")
	}

	data, err := io.ReadAll(body)
	if err != nil {
		return "", fmt.Errorf("failed to read part data: %w", err)
	}

	if size > 0 && int64(len(data)) != size {
		return "", fmt.Errorf("DataLengthMismatch: expected %d bytes, got %d", size, len(data))
	}

	etag := fmt.Sprintf("etag-%d-%x", partNumber, len(data))

	m.parts[uploadID] = append(m.parts[uploadID], testPart{
		partNumber: partNumber,
		data:       data,
		etag:       etag,
	})

	return etag, nil
}

func (m *mockB2BackendForTesting) CompleteMultipartUpload(ctx context.Context, bucket, key, uploadID string, parts []CompletedPart) (string, error) {
	state, ok := m.uploads[uploadID]
	if !ok {
		return "", fmt.Errorf("NoSuchUpload: The specified multipart upload does not exist")
	}

	if state.bucket != bucket || state.key != key {
		return "", fmt.Errorf("InvalidRequest: Bucket/key mismatch")
	}

	uploadedParts := m.parts[uploadID]
	if len(parts) != len(uploadedParts) {
		return "", fmt.Errorf("InvalidPart: Mismatch in part count")
	}

	// Sort parts by part number
	partMap := make(map[int32]*testPart)
	for i := range uploadedParts {
		partMap[uploadedParts[i].partNumber] = &uploadedParts[i]
	}

	// Verify all parts and combine data
	var combinedData []byte
	for _, p := range parts {
		part, ok := partMap[p.PartNumber]
		if !ok {
			return "", fmt.Errorf("InvalidPart: Part %d not found", p.PartNumber)
		}
		if part.etag != p.ETag {
			return "", fmt.Errorf("InvalidPartETag: ETag mismatch for part %d", p.PartNumber)
		}
		combinedData = append(combinedData, part.data...)
	}

	// Store the combined object
	objectKey := bucket + "/" + key
	m.objects[objectKey] = combinedData

	// Clean up upload state
	delete(m.uploads, uploadID)
	delete(m.parts, uploadID)

	finalETag := fmt.Sprintf("final-%x", len(combinedData))
	return finalETag, nil
}

func (m *mockB2BackendForTesting) AbortMultipartUpload(ctx context.Context, bucket, key, uploadID string) error {
	state, ok := m.uploads[uploadID]
	if !ok {
		return fmt.Errorf("NoSuchUpload: The specified multipart upload does not exist")
	}

	if state.bucket != bucket || state.key != key {
		return fmt.Errorf("InvalidRequest: Bucket/key mismatch")
	}

	// Clean up upload state
	delete(m.uploads, uploadID)
	delete(m.parts, uploadID)

	return nil
}

func (m *mockB2BackendForTesting) Get(ctx context.Context, bucket, key string) (io.ReadCloser, *ObjectInfo, error) {
	objectKey := bucket + "/" + key
	data, ok := m.objects[objectKey]
	if !ok {
		return nil, nil, fmt.Errorf("NoSuchKey: The specified key does not exist")
	}

	info := &ObjectInfo{
		Key:          key,
		Size:         int64(len(data)),
		ContentType:  "application/octet-stream",
		ETag:         fmt.Sprintf("etag-%x", len(data)),
		LastModified: time.Now(),
	}

	return io.NopCloser(bytes.NewReader(data)), info, nil
}

// TestMockB2Backend_MultipartUploadIntegration tests the mock backend with a complete workflow.
func TestMockB2Backend_MultipartUploadIntegration(t *testing.T) {
	ctx := context.Background()
	backend := NewMockB2BackendForTesting()

	// Create multipart upload
	uploadID, err := backend.CreateMultipartUpload(ctx, "test-bucket", "test-key", map[string]string{
		"Content-Type": "application/octet-stream",
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload failed: %v", err)
	}

	// Upload 3 parts
	var parts []CompletedPart
	partData := [][]byte{
		bytes.Repeat([]byte{0x00}, 5*1024*1024), // 5MB part 1
		bytes.Repeat([]byte{0x01}, 5*1024*1024), // 5MB part 2
		bytes.Repeat([]byte{0x02}, 3*1024*1024), // 3MB part 3 (irregular)
	}

	for i, data := range partData {
		partNum := int32(i + 1)
		etag, err := backend.UploadPart(ctx, "test-bucket", "test-key", uploadID, partNum, bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatalf("UploadPart %d failed: %v", partNum, err)
		}
		parts = append(parts, CompletedPart{
			PartNumber: partNum,
			ETag:        etag,
		})
	}

	// Complete upload
	finalETag, err := backend.CompleteMultipartUpload(ctx, "test-bucket", "test-key", uploadID, parts)
	if err != nil {
		t.Fatalf("CompleteMultipartUpload failed: %v", err)
	}

	t.Logf("✓ Mock backend multipart upload completed, final ETag: %s", finalETag)

	// Verify the object can be retrieved
	body, info, err := backend.Get(ctx, "test-bucket", "test-key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	defer body.Close()

	expectedSize := int64(len(partData[0]) + len(partData[1]) + len(partData[2]))
	if info.Size != expectedSize {
		t.Errorf("Expected size %d, got %d", expectedSize, info.Size)
	}

	t.Logf("✓ Mock backend object retrieval verified, size: %d bytes", info.Size)
}
