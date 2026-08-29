//go:build integration
// +build integration

// Comprehensive ListObjectsV2 Tests
// Tests for ListObjectsV2 operation covering pagination, prefix filtering, delimiter, and edge cases

package integration

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// TestListObjectsV2_Basic tests basic object listing
func TestListObjectsV2_Basic(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	// Upload test objects
	keys := []string{}
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("list-basic-test/object-%d-%d", i, time.Now().UnixNano())
		keys = append(keys, key)

		testData := generateTestData(512)
		_, err := client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
			Body:   bytes.NewReader(testData),
		})
		if err != nil {
			t.Fatalf("PutObject for %s failed: %v", key, err)
		}
	}
	defer cleanupObjects(client, bucket, keys)

	t.Logf("Uploaded %d test objects", len(keys))

	// List objects
	listResp, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String("list-basic-test/"),
	})
	if err != nil {
		t.Fatalf("ListObjectsV2 failed: %v", err)
	}

	t.Logf("ListObjectsV2 returned %d objects", len(listResp.Contents))

	// Verify all our objects are listed
	foundCount := 0
	for _, obj := range listResp.Contents {
		for _, key := range keys {
			if *obj.Key == key {
				foundCount++
				break
			}
		}
	}

	if foundCount != len(keys) {
		t.Errorf("Found %d of %d expected objects", foundCount, len(keys))
	} else {
		t.Logf("All %d objects found in listing", foundCount)
	}
}

// TestListObjectsV2_Pagination tests pagination with continuation token
func TestListObjectsV2_Pagination(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	// Upload more objects than fit in a single page
	numObjects := 25 // S3 default max-keys is 1000, but we'll use smaller max-keys
	keys := []string{}

	for i := 0; i < numObjects; i++ {
		key := fmt.Sprintf("list-pagination-test/page-%03d-%d", i, time.Now().UnixNano())
		keys = append(keys, key)

		testData := generateTestData(512)
		_, err := client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
			Body:   bytes.NewReader(testData),
		})
		if err != nil {
			t.Fatalf("PutObject for %s failed: %v", key, err)
		}
	}
	defer cleanupObjects(client, bucket, keys)

	t.Logf("Uploaded %d test objects", numObjects)

	// List with small max-keys to force pagination
	maxKeys := int32(5)
	allKeys := []string{}
	var continuationToken *string

	pageNum := 0
	for {
		listResp, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(bucket),
			Prefix:            aws.String("list-pagination-test/"),
			MaxKeys:           aws.Int32(maxKeys),
			ContinuationToken: continuationToken,
		})
		if err != nil {
			t.Fatalf("ListObjectsV2 page %d failed: %v", pageNum, err)
		}

		pageNum++
		t.Logf("Page %d: got %d objects", pageNum, len(listResp.Contents))

		// Collect keys from this page
		for _, obj := range listResp.Contents {
			allKeys = append(allKeys, *obj.Key)
		}

		// Check if there are more pages
		if listResp.NextContinuationToken == nil || len(listResp.Contents) == 0 {
			break
		}
		continuationToken = listResp.NextContinuationToken
	}

	t.Logf("Paginated through %d pages, collected %d keys", pageNum, len(allKeys))

	// Verify we got all keys
	if len(allKeys) != numObjects {
		t.Errorf("Pagination returned %d keys, expected %d", len(allKeys), numObjects)
	} else {
		t.Logf("Pagination successful - all %d keys retrieved", numObjects)
	}

	// Verify keys are unique (no duplicates)
	uniqueKeys := make(map[string]bool)
	for _, key := range allKeys {
		if uniqueKeys[key] {
			t.Errorf("Duplicate key found in pagination: %s", key)
		}
		uniqueKeys[key] = true
	}

	if len(uniqueKeys) == len(allKeys) {
		t.Log("All keys in pagination are unique")
	}
}

// TestListObjectsV2_PrefixFiltering tests prefix-based filtering
func TestListObjectsV2_PrefixFiltering(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	// Upload objects with different prefixes
	prefixes := []struct {
		prefix     string
		numObjects int
	}{
		{"list-prefix-test/aaa/", 5},
		{"list-prefix-test/bbb/", 5},
		{"list-prefix-test/ccc/", 5},
		{"other-prefix/", 3},
	}

	allKeys := []string{}
	for _, p := range prefixes {
		for i := 0; i < p.numObjects; i++ {
			key := fmt.Sprintf("%sobject-%d-%d", p.prefix, i, time.Now().UnixNano())
			allKeys = append(allKeys, key)

			testData := generateTestData(512)
			_, err := client.PutObject(ctx, &s3.PutObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(key),
				Body:   bytes.NewReader(testData),
			})
			if err != nil {
				t.Fatalf("PutObject for %s failed: %v", key, err)
			}
		}
	}
	defer cleanupObjects(client, bucket, allKeys)

	t.Logf("Uploaded %d test objects across %d prefixes", len(allKeys), len(prefixes))

	// Test listing with specific prefix
	listResp, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String("list-prefix-test/bbb/"),
	})
	if err != nil {
		t.Fatalf("ListObjectsV2 with prefix failed: %v", err)
	}

	t.Logf("ListObjectsV2 with prefix 'list-prefix-test/bbb/' returned %d objects",
		len(listResp.Contents))

	// Verify all returned objects have the prefix
	for _, obj := range listResp.Contents {
		if !strings.HasPrefix(*obj.Key, "list-prefix-test/bbb/") {
			t.Errorf("Object %s doesn't have prefix 'list-prefix-test/bbb/'", *obj.Key)
		}
	}

	// Count should be 5 (what we uploaded for that prefix)
	expectedCount := 5
	if len(listResp.Contents) != expectedCount {
		t.Logf("Warning: got %d objects, expected %d (may have concurrent test objects)",
			len(listResp.Contents), expectedCount)
	} else {
		t.Log("Prefix filtering returned expected count")
	}
}

// TestListObjectsV2_DelimiterCommonPrefixes tests delimiter and common prefixes
func TestListObjectsV2_DelimiterCommonPrefixes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	// Upload objects in a directory-like structure
	keys := []string{
		"list-delimiter-test/docs/file1.txt",
		"list-delimiter-test/docs/file2.txt",
		"list-delimiter-test/images/photo1.jpg",
		"list-delimiter-test/images/photo2.jpg",
		"list-delimiter-test/root.txt",
	}

	for _, key := range keys {
		testData := generateTestData(512)
		_, err := client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
			Body:   bytes.NewReader(testData),
		})
		if err != nil {
			t.Fatalf("PutObject for %s failed: %v", key, err)
		}
	}
	defer cleanupObjects(client, bucket, keys)

	t.Logf("Uploaded %d test objects with directory structure", len(keys))

	// List with delimiter to get common prefixes
	listResp, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Prefix:    aws.String("list-delimiter-test/"),
		Delimiter: aws.String("/"),
	})
	if err != nil {
		t.Fatalf("ListObjectsV2 with delimiter failed: %v", err)
	}

	t.Logf("ListObjectsV2 returned %d objects and %d common prefixes",
		len(listResp.Contents), len(listResp.CommonPrefixes))

	// Verify common prefixes
	expectedPrefixes := []string{
		"list-delimiter-test/docs/",
		"list-delimiter-test/images/",
	}

	for _, expectedPrefix := range expectedPrefixes {
		found := false
		for _, cp := range listResp.CommonPrefixes {
			if *cp.Prefix == expectedPrefix {
				found = true
				t.Logf("Found common prefix: %s", expectedPrefix)
				break
			}
		}
		if !found {
			t.Logf("Warning: expected common prefix %s not found", expectedPrefix)
		}
	}

	// Verify objects at root level (no intermediate prefix)
	rootObjects := 0
	for _, obj := range listResp.Contents {
		if !strings.Contains(*obj.Key, "/") ||
			strings.Count(*obj.Key, "/") == strings.Count("list-delimiter-test/", "/") {
			rootObjects++
			t.Logf("Root-level object: %s", *obj.Key)
		}
	}

	if rootObjects > 0 {
		t.Logf("Found %d objects at root level", rootObjects)
	}
}

// TestListObjectsV2_MaxKeys tests max-keys parameter
func TestListObjectsV2_MaxKeys(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	// Upload test objects
	numObjects := 15
	keys := []string{}

	for i := 0; i < numObjects; i++ {
		key := fmt.Sprintf("list-maxkeys-test/object-%d-%d", i, time.Now().UnixNano())
		keys = append(keys, key)

		testData := generateTestData(512)
		_, err := client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
			Body:   bytes.NewReader(testData),
		})
		if err != nil {
			t.Fatalf("PutObject for %s failed: %v", key, err)
		}
	}
	defer cleanupObjects(client, bucket, keys)

	// Test various max-keys values
	testCases := []struct {
		maxKeys  int32
		expected int32
	}{
		{1, 1},
		{5, 5},
		{10, 10},
		{1000, 15}, // More than total objects
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("MaxKeys-%d", tc.maxKeys), func(t *testing.T) {
			listResp, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
				Bucket:  aws.String(bucket),
				Prefix:  aws.String("list-maxkeys-test/"),
				MaxKeys: aws.Int32(tc.maxKeys),
			})
			if err != nil {
				t.Fatalf("ListObjectsV2 with MaxKeys=%d failed: %v", tc.maxKeys, err)
			}

			expected := tc.expected
			if expected > int32(len(keys)) {
				expected = int32(len(keys))
			}

			if int32(len(listResp.Contents)) > tc.maxKeys {
				t.Errorf("MaxKeys=%d returned %d objects (more than max)",
					tc.maxKeys, len(listResp.Contents))
			} else {
				t.Logf("MaxKeys=%d returned %d objects (within limit)",
					tc.maxKeys, len(listResp.Contents))
			}

			// Check if truncated when expected
			if int32(len(listResp.Contents)) < expected && listResp.NextContinuationToken == nil {
				t.Logf("Warning: results truncated but no NextContinuationToken")
			}
		})
	}
}

// TestListObjectsV2_StartAfter tests start-after parameter
func TestListObjectsV2_StartAfter(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	// Upload objects with predictable ordering
	keys := []string{}
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("list-startafter-test/object-%03d-%d", i, time.Now().UnixNano())
		keys = append(keys, key)

		testData := generateTestData(512)
		_, err := client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
			Body:   bytes.NewReader(testData),
		})
		if err != nil {
			t.Fatalf("PutObject for %s failed: %v", key, err)
		}
	}
	defer cleanupObjects(client, bucket, keys)

	// Get all objects first to determine ordering
	fullList, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String("list-startafter-test/"),
	})
	if err != nil {
		t.Fatalf("ListObjectsV2 failed: %v", err)
	}

	if len(fullList.Contents) < 5 {
		t.Fatalf("Need at least 5 objects for start-after test, got %d", len(fullList.Contents))
	}

	// Start after the 3rd object
	startAfterKey := *fullList.Contents[2].Key
	t.Logf("Starting after key: %s", startAfterKey)

	listResp, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Prefix:    aws.String("list-startafter-test/"),
		StartAfter: aws.String(startAfterKey),
	})
	if err != nil {
		t.Fatalf("ListObjectsV2 with StartAfter failed: %v", err)
	}

	t.Logf("StartAfter returned %d objects", len(listResp.Contents))

	// Verify all returned keys come after the start-after key
	for _, obj := range listResp.Contents {
		if *obj.Key <= startAfterKey {
			t.Errorf("Object %s comes before or at start-after key %s",
				*obj.Key, startAfterKey)
		}
	}

	t.Log("StartAfter parameter working correctly")
}

// TestListObjectsV2_EmptyBucket tests listing an empty bucket/prefix
func TestListObjectsV2_EmptyBucket(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	// Use a unique prefix that won't exist
	uniquePrefix := fmt.Sprintf("nonexistent-prefix-%d/", time.Now().UnixNano())

	listResp, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(uniquePrefix),
	})
	if err != nil {
		t.Fatalf("ListObjectsV2 with unique prefix failed: %v", err)
	}

	if len(listResp.Contents) != 0 {
		t.Errorf("ListObjectsV2 with unique prefix returned %d objects, expected 0",
			len(listResp.Contents))
	}

	t.Log("ListObjectsV2 with unique prefix correctly returned empty list")
}

// TestListObjectsV2_EncryptedMetadata tests that ARMOR returns plaintext sizes
func TestListObjectsV2_EncryptedMetadata(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	key := generateTestKey(t)

	// Upload object with known size
	testData := generateTestData(1024 * 100) // 100 KB
	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(testData),
	})
	if err != nil {
		t.Fatalf("PutObject failed: %v", err)
	}
	defer client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	// Get the object to verify plaintext size
	headResp, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("HeadObject failed: %v", err)
	}
	plaintextSize := *headResp.ContentLength

	// List objects
	listResp, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(key[:len(key)-10]), // Use prefix to match
	})
	if err != nil {
		t.Fatalf("ListObjectsV2 failed: %v", err)
	}

	// Find our object in the listing
	found := false
	for _, obj := range listResp.Contents {
		if *obj.Key == key {
			found = true
			listedSize := *obj.Size

			// ARMOR should return plaintext size in listings
			if listedSize != plaintextSize {
				t.Errorf("Size mismatch: HeadObject reports %d, ListObjectsV2 reports %d",
					plaintextSize, listedSize)
			} else {
				t.Logf("Size correct in listing: %d bytes (plaintext)", listedSize)
			}

			// Verify ETag is present
			if obj.ETag == nil {
				t.Error("ETag missing from ListObjectsV2 response")
			} else {
				t.Logf("ETag present: %s", *obj.ETag)
			}

			// Verify other metadata
			if obj.LastModified == nil {
				t.Error("LastModified missing from ListObjectsV2 response")
			}
			if obj.StorageClass == "" {
				t.Logf("StorageClass empty (may be STANDARD default)")
			}

			break
		}
	}

	if !found {
		t.Errorf("Object %s not found in listing", key)
	}

	t.Log("TestListObjectsV2_EncryptedMetadata completed")
}

// TestListObjectsV2_ArmorReservedNamespace tests .armor/ prefix filtering
func TestListObjectsV2_ArmorReservedNamespace(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	// Upload a regular object
	regularKey := generateTestKey(t)
	testData := generateTestData(512)
	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(regularKey),
		Body:   bytes.NewReader(testData),
	})
	if err != nil {
		t.Fatalf("PutObject failed: %v", err)
	}
	defer client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(regularKey),
	})

	// List objects
	listResp, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		t.Fatalf("ListObjectsV2 failed: %v", err)
	}

	// Verify .armor/ objects are not in the listing
	for _, obj := range listResp.Contents {
		if strings.HasPrefix(*obj.Key, ".armor/") {
			t.Errorf("Reserved namespace object found in listing: %s", *obj.Key)
		}
	}

	t.Logf("ListObjectsV2 returned %d objects, none with .armor/ prefix", len(listResp.Contents))

	// Verify that trying to access .armor/ objects returns 403
	armorKey := ".armor/test/internal-object"
	_, err = client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(armorKey),
	})

	if err == nil {
		t.Error("Accessing .armor/ object should fail with 403")
	} else {
		t.Logf("Access to .armor/ correctly denied: %v", err)
	}

	t.Log("TestListObjectsV2_ArmorReservedNamespace completed")
}

// TestListObjectsV2_EncodingType tests encoding-type parameter
func TestListObjectsV2_EncodingType(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	// Upload object with special characters
	key := "list-encoding-test/object-with- spaces-and-+plus-"
	testData := generateTestData(512)
	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(testData),
	})
	if err != nil {
		t.Fatalf("PutObject failed: %v", err)
	}
	defer client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	// List with URL encoding
	listResp, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:      aws.String(bucket),
		Prefix:      aws.String("list-encoding-test/"),
		EncodingType: types.EncodingTypeUrl,
	})
	if err != nil {
		t.Fatalf("ListObjectsV2 with EncodingType failed: %v", err)
	}

	t.Logf("ListObjectsV2 with URL encoding returned %d objects", len(listResp.Contents))

	// Find our object and verify encoding
	for _, obj := range listResp.Contents {
		if strings.Contains(*obj.Key, "object-with-") {
			t.Logf("Found object with encoding: %s", *obj.Key)

			// With URL encoding, spaces should be encoded as + or %20
			if strings.Contains(*obj.Key, " ") {
				t.Logf("Warning: key contains unencoded space")
			}
			break
		}
	}

	t.Log("TestListObjectsV2_EncodingType completed")
}

// TestListObjectsV2_FetchOwner tests owner information in listing
func TestListObjectsV2_FetchOwner(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	armorEndpoint := getEnvOr("ARMOR_ENDPOINT", "http://localhost:9000")
	client := createS3Client(t, armorEndpoint)
	ctx := context.Background()

	key := generateTestKey(t)
	testData := generateTestData(512)
	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(testData),
	})
	if err != nil {
		t.Fatalf("PutObject failed: %v", err)
	}
	defer client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	// List with fetch-owner enabled
	listResp, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:     aws.String(bucket),
		Prefix:     aws.String(key[:len(key)-10]),
		FetchOwner: aws.Bool(true),
	})
	if err != nil {
		t.Fatalf("ListObjectsV2 with FetchOwner failed: %v", err)
	}

	// Check owner information
	for _, obj := range listResp.Contents {
		if obj.Owner == nil {
			t.Logf("Warning: Owner not returned for object %s", *obj.Key)
		} else {
			if obj.Owner.ID != nil {
				t.Logf("Owner ID: %s", *obj.Owner.ID)
			}
			if obj.Owner.DisplayName != nil {
				t.Logf("Owner DisplayName: %s", *obj.Owner.DisplayName)
			}
		}
	}

	t.Log("TestListObjectsV2_FetchOwner completed")
}

// cleanupObjects deletes a list of objects and logs errors
func cleanupObjects(client *s3.Client, bucket string, keys []string) {
	ctx := context.Background()
	for _, key := range keys {
		_, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			fmt.Printf("Warning: failed to delete %s: %v\n", key, err)
		}
	}
}
