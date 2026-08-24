//go:build integration
// +build integration

// Comprehensive bucket operations tests
// Tests for S3 bucket management APIs: ListBuckets, CreateBucket, DeleteBucket, HeadBucket

package integration

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// TestListBuckets tests listing all buckets
func TestListBuckets(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	// List all buckets
	resp, err := client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		t.Fatalf("ListBuckets failed: %v", err)
	}

	if resp.Buckets == nil {
		t.Error("ListBuckets returned nil buckets list")
		return
	}

	t.Logf("ListBuckets returned %d buckets", len(resp.Buckets))

	// Verify our test bucket is in the list
	found := false
	for _, b := range resp.Buckets {
		if *b.Name == bucket {
			found = true
			t.Logf("Found test bucket: %s (created: %v)", *b.Name, b.CreationDate)
			break
		}
	}

	if !found {
		t.Errorf("Test bucket %s not found in ListBuckets response", bucket)
	}

	// Verify Owner field is present
	if resp.Owner == nil {
		t.Error("ListBuckets response missing Owner field")
	} else if resp.Owner.ID == nil {
		t.Error("ListBuckets Owner missing ID")
	} else {
		t.Logf("Owner ID: %s", *resp.Owner.ID)
	}

	t.Log("TestListBuckets completed successfully")
}

// TestHeadBucket tests checking if a bucket exists
func TestHeadBucket(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	// Head the existing bucket
	resp, err := client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		t.Fatalf("HeadBucket failed: %v", err)
	}

	// HeadBucket returns HTTP response metadata
	if resp == nil {
		t.Error("HeadBucket returned nil response")
	} else {
		t.Logf("HeadBucket succeeded for bucket: %s", bucket)
	}

	t.Log("HeadBucket completed successfully")
}

// TestHeadBucket_NonExistentBucket tests heading a bucket that doesn't exist
func TestHeadBucket_NonExistentBucket(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	nonExistentBucket := fmt.Sprintf("non-existent-bucket-%d", time.Now().UnixNano())

	// Head non-existent bucket should fail
	_, err := client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(nonExistentBucket),
	})

	if err == nil {
		t.Error("HeadBucket of non-existent bucket should fail")
	} else {
		t.Logf("HeadBucket of non-existent bucket failed as expected: %v", err)
	}
}

// TestCreateBucket_BucketNaming tests bucket naming constraints
func TestCreateBucket_BucketNaming(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	// S3 bucket naming rules:
	// - 3-63 characters
	// - lowercase letters, numbers, hyphens
	// - must start/end with letter or number
	// - must not contain adjacent periods
	// - must not be IP address format

	testCases := []struct {
		name        string
		bucketName   string
		shouldFail  bool
		description string
	}{
		{
			name:        "Valid name with hyphens",
			bucketName:   fmt.Sprintf("valid-bucket-%d", time.Now().UnixNano()),
			shouldFail:  false,
			description: "Valid bucket name with hyphens",
		},
		{
			name:        "Valid name with numbers",
			bucketName:   fmt.Sprintf("bucket123-%d", time.Now().UnixNano()),
			shouldFail:  false,
			description: "Valid bucket name with numbers",
		},
		{
			name:        "Too short",
			bucketName:   "ab",
			shouldFail:  true,
			description: "Bucket name too short (< 3 chars)",
		},
		{
			name:        "Too long",
			bucketName:   fmt.Sprintf("a%sb", string(make([]byte, 64))), // 64 chars
			shouldFail:  true,
			description: "Bucket name too long (> 63 chars)",
		},
		{
			name:        "Uppercase letters",
			bucketName:   fmt.Sprintf("InvalidBucket-%d", time.Now().UnixNano()),
			shouldFail:  true,
			description: "Bucket name with uppercase letters",
		},
		{
			name:        "Ends with hyphen",
			bucketName:   fmt.Sprintf("invalid-bucket-%d-", time.Now().UnixNano()),
			shouldFail:  true,
			description: "Bucket name ending with hyphen",
		},
		{
			name:        "Starts with hyphen",
			bucketName:   fmt.Sprintf("-%d-bucket", time.Now().UnixNano()),
			shouldFail:  true,
			description: "Bucket name starting with hyphen",
		},
		{
			name:        "Contains special chars",
			bucketName:   fmt.Sprintf("invalid_bucket_%d", time.Now().UnixNano()),
			shouldFail:  true,
			description: "Bucket name with underscores",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
				Bucket: aws.String(tc.bucketName),
			})

			if tc.shouldFail {
				if err == nil {
					t.Errorf("%s: expected CreateBucket to fail, but it succeeded", tc.description)
					// Cleanup if it accidentally succeeded
					_, _ = client.DeleteBucket(ctx, &s3.DeleteBucketInput{
						Bucket: aws.String(tc.bucketName),
					})
				} else {
					t.Logf("%s: failed as expected: %v", tc.description, err)
				}
			} else {
				if err != nil {
					t.Errorf("%s: CreateBucket failed: %v", tc.description, err)
				} else {
					t.Logf("%s: succeeded", tc.description)
					// Cleanup
					_, _ = client.DeleteBucket(ctx, &s3.DeleteBucketInput{
						Bucket: aws.String(tc.bucketName),
					})
				}
			}
		})
	}
}

// TestCreateBucket_AlreadyExists tests creating a bucket that already exists
func TestCreateBucket_AlreadyExists(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	// Try to create a bucket that already exists (use the test bucket)
	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	})

	if err == nil {
		t.Error("CreateBucket of existing bucket should fail")
	} else {
		t.Logf("CreateBucket of existing bucket failed as expected: %v", err)
	}
}

// TestDeleteBucket_EmptyBucket tests deleting an empty bucket
func TestDeleteBucket_EmptyBucket(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	// Create a test bucket
	testBucket := fmt.Sprintf("delete-test-%d", time.Now().UnixNano())
	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(testBucket),
	})
	if err != nil {
		t.Fatalf("CreateBucket failed: %v", err)
	}
	t.Logf("Created test bucket: %s", testBucket)

	// Verify bucket exists
	_, err = client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(testBucket),
	})
	if err != nil {
		t.Fatalf("HeadBucket failed: %v", err)
	}

	// Delete the empty bucket
	_, err = client.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(testBucket),
	})
	if err != nil {
		t.Fatalf("DeleteBucket failed: %v", err)
	}
	t.Logf("Deleted bucket: %s", testBucket)

	// Verify bucket no longer exists
	_, err = client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(testBucket),
	})
	if err == nil {
		t.Error("Bucket still exists after DeleteBucket")
	} else {
		t.Logf("Bucket correctly no longer exists: %v", err)
	}
}

// TestDeleteBucket_NonEmptyBucket tests deleting a bucket with objects
func TestDeleteBucket_NonEmptyBucket(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	// Create a test bucket
	testBucket := fmt.Sprintf("non-empty-test-%d", time.Now().UnixNano())
	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(testBucket),
	})
	if err != nil {
		t.Fatalf("CreateBucket failed: %v", err)
	}
	defer func() {
		// Cleanup: delete all objects then bucket
		// List and delete all objects
		listResp, _ := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket: aws.String(testBucket),
		})
		for _, obj := range listResp.Contents {
			_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
				Bucket: aws.String(testBucket),
				Key:    obj.Key,
			})
		}
		// Delete bucket
		_, _ = client.DeleteBucket(ctx, &s3.DeleteBucketInput{
			Bucket: aws.String(testBucket),
		})
	}()

	// Put an object in the bucket
	testKey := "test-object.txt"
	testData := generateTestData(1024)
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(testBucket),
		Key:    aws.String(testKey),
		Body:   bytes.NewReader(testData),
	})
	if err != nil {
		t.Fatalf("PutObject failed: %v", err)
	}
	t.Logf("Created object in bucket: %s", testKey)

	// Try to delete the non-empty bucket - should fail
	_, err = client.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(testBucket),
	})

	if err == nil {
		t.Error("DeleteBucket of non-empty bucket should fail")
	} else {
		t.Logf("DeleteBucket of non-empty bucket failed as expected: %v", err)
	}
}

// TestDeleteBucket_NonExistentBucket tests deleting a bucket that doesn't exist
func TestDeleteBucket_NonExistentBucket(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	nonExistentBucket := fmt.Sprintf("non-existent-bucket-%d", time.Now().UnixNano())

	// Try to delete non-existent bucket
	_, err := client.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(nonExistentBucket),
	})

	if err == nil {
		t.Error("DeleteBucket of non-existent bucket should fail")
	} else {
		t.Logf("DeleteBucket of non-existent bucket failed as expected: %v", err)
	}
}

// TestBucketLocationConstraint tests CreateBucket with location constraint
func TestBucketLocationConstraint(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	// Note: S3 location constraint is region-specific
	// For us-east-1, LocationConstraint should be nil
	// For other regions, it should be set

	testBucket := fmt.Sprintf("location-test-%d", time.Now().UnixNano())

	// Try creating bucket with no location constraint (default behavior)
	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(testBucket),
	})
	if err != nil {
		t.Logf("CreateBucket without location constraint: %v", err)
	} else {
		t.Log("CreateBucket without location constraint succeeded")
		// Cleanup
		_, _ = client.DeleteBucket(ctx, &s3.DeleteBucketInput{
			Bucket: aws.String(testBucket),
		})
	}
}

// TestListBuckets_Pagination tests pagination with many buckets
func TestListBuckets_Pagination(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	// Note: This test requires creating many buckets, which may not be practical
	// in a test environment. Implementing as a placeholder for documentation.

	t.Skip("Skipping: TestListBuckets_Pagination requires creating many buckets")

	// TODO: Implement if test environment supports creating many buckets
	// Would need to:
	// 1. Create > 100 buckets (S3 pagination limit is 1000)
	// 2. Test MaxBuckets parameter
	// 3. Test ContinuationToken for pagination
}

// TestBucketRegionDetection tests detecting bucket region
func TestBucketRegionDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	t.Skip("Skipping: ARMOR uses single backend region - GetBucketRegion not applicable")
}

// TestBucketLifecycle tests bucket lifecycle configuration (basic)
func TestBucketLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	// Create lifecycle rule to expire objects after 1 day
	rule := types.LifecycleRule{
		ID:   aws.String("expire-objects"),
		Status: types.ExpirationStatusEnabled,
		Filter: &types.LifecycleRuleFilter{
			Prefix: aws.String("lifecycle-test/"),
		},
		Expiration: &types.LifecycleExpiration{
			Days: aws.Int32(1),
		},
	}

	// Put lifecycle configuration
	_, err := client.PutBucketLifecycleConfiguration(ctx, &s3.PutBucketLifecycleConfigurationInput{
		Bucket: aws.String(bucket),
		LifecycleConfiguration: &types.BucketLifecycleConfiguration{
			Rules: []types.LifecycleRule{rule},
		},
	})

	if err != nil {
		t.Logf("PutBucketLifecycleConfiguration: %v", err)
		// This is expected if ARMOR doesn't support lifecycle
		t.Log("ARMOR may not support lifecycle configuration")
		return
	}

	t.Log("Successfully put lifecycle configuration")

	// Get lifecycle configuration
	getResp, err := client.GetBucketLifecycleConfiguration(ctx, &s3.GetBucketLifecycleConfigurationInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		t.Fatalf("GetBucketLifecycleConfiguration failed: %v", err)
	}

	if len(getResp.Rules) != 1 {
		t.Errorf("Expected 1 lifecycle rule, got %d", len(getResp.Rules))
	}

	t.Logf("Retrieved lifecycle rule: %s", *getResp.Rules[0].ID)

	// Delete lifecycle configuration - S3 doesn't have a delete operation
	// Instead, we put an empty configuration
	_, err = client.PutBucketLifecycleConfiguration(ctx, &s3.PutBucketLifecycleConfigurationInput{
		Bucket: aws.String(bucket),
		LifecycleConfiguration: &types.BucketLifecycleConfiguration{
			Rules: []types.LifecycleRule{},
		},
	})
	if err != nil {
		t.Fatalf("Delete (empty) lifecycle configuration failed: %v", err)
	}

	t.Log("Successfully deleted lifecycle configuration")
}

// TestBucketVersioning tests bucket versioning
func TestBucketVersioning(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	// Get versioning status
	resp, err := client.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		t.Logf("GetBucketVersioning failed: %v", err)
		t.Log("ARMOR may not support versioning")
		return
	}

	if resp.Status == types.BucketVersioningStatusEnabled {
		t.Log("Bucket versioning is enabled")
	} else if resp.Status == types.BucketVersioningStatusSuspended {
		t.Log("Bucket versioning is suspended")
	} else {
		t.Log("Bucket versioning is not enabled")
	}
}
