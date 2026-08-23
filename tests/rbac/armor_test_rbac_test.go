//go:build integration
// +build integration

// Package rbac tests RBAC verb coverage against B2 objects using armor-test credentials.
// This test suite verifies GET, PUT, and DELETE operations against the armor-test-jedarden
// bucket and documents allow/deny outcomes per ADR-012.
package rbac

import (
	"bytes"
	"context"
	"encoding/hex"
	"io"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jedarden/armor/internal/config"
)

const (
	// armor-test credentials from OpenBao
	armorTestAccessKey = "hd7jp9oeysgt2x3obewn7k8og1vtup337juf2qchc19eqrchqhgo4feeho9ip2ux"
	armorTestSecretKey = "b5fvnuj3d7f5xxxew9rxmb85pz3iro5zawb1gixupozd9g85ito1aqeji324ye2d"

	// armor-test bucket and region from ConfigMap
	armorTestBucket = "armor-test-jedarden"
	armorTestRegion = "us-west-002"

	// armor-test MEK from OpenBao
	armorTestMEK = "795613c7886f83c4a02b0056dabc76613a5ed998141d983f4e7bb3d69c85e3d6"

	// armor-test service endpoint (localhost via port-forward)
	armorTestEndpoint = "http://localhost:9000"
)

// TestRBAC_GET_Allowed tests that GET operations work with armor-test credentials.
// This verifies the default credential has full access (no ACL restrictions).
func TestRBAC_GET_Allowed(t *testing.T) {
	ctx := context.Background()
	client := newSDKClient(t, armorTestEndpoint)

	// Create a test object first
	testData := []byte("RBAC GET test - " + time.Now().Format(time.RFC3339))
	testKey := "rbac-test/get-allowed.txt"
	bucket := armorTestBucket

	// PUT the test object
	putIn := &s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &testKey,
		Body:   bytes.NewReader(testData),
	}

	_, err := client.PutObject(ctx, putIn)
	if err != nil {
		t.Fatalf("PUT failed (setup for GET test): %v", err)
	}
	t.Logf("PUT succeeded for %s", testKey)

	// GET the object
	getIn := &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &testKey,
	}

	getOut, err := client.GetObject(ctx, getIn)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer getOut.Body.Close()

	// Verify content
	retrievedData, err := io.ReadAll(getOut.Body)
	if err != nil {
		t.Fatalf("Failed to read GET response: %v", err)
	}

	if !bytes.Equal(retrievedData, testData) {
		t.Errorf("GET content mismatch: got %d bytes, want %d bytes", len(retrievedData), len(testData))
	}

	t.Logf("GET allowed: successfully retrieved %d bytes from %s", len(retrievedData), testKey)
}

// TestRBAC_PUT_Allowed tests that PUT operations work with armor-test credentials.
func TestRBAC_PUT_Allowed(t *testing.T) {
	ctx := context.Background()
	client := newSDKClient(t, armorTestEndpoint)

	testData := []byte("RBAC PUT test - " + time.Now().Format(time.RFC3339))
	testKey := "rbac-test/put-allowed.txt"
	bucket := armorTestBucket

	putIn := &s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &testKey,
		Body:   bytes.NewReader(testData),
	}

	result, err := client.PutObject(ctx, putIn)
	if err != nil {
		t.Fatalf("PUT failed: %v", err)
	}

	if result.ETag == nil {
		t.Error("PUT returned nil ETag")
	}

	t.Logf("PUT allowed: successfully uploaded %d bytes to %s (ETag: %s)", len(testData), testKey, *result.ETag)
}

// TestRBAC_DELETE_Allowed tests that DELETE operations work with armor-test credentials.
func TestRBAC_DELETE_Allowed(t *testing.T) {
	ctx := context.Background()
	client := newSDKClient(t, armorTestEndpoint)

	// Create a test object first
	testData := []byte("RBAC DELETE test object")
	testKey := "rbac-test/delete-allowed.txt"
	bucket := armorTestBucket

	putIn := &s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &testKey,
		Body:   bytes.NewReader(testData),
	}

	_, err := client.PutObject(ctx, putIn)
	if err != nil {
		t.Fatalf("PUT failed (setup for DELETE test): %v", err)
	}

	// DELETE the object
	deleteIn := &s3.DeleteObjectInput{
		Bucket: &bucket,
		Key:    &testKey,
	}

	_, err = client.DeleteObject(ctx, deleteIn)
	if err != nil {
		t.Fatalf("DELETE failed: %v", err)
	}

	t.Logf("DELETE allowed: successfully deleted %s", testKey)

	// Verify deletion - GET should return 404
	getIn := &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &testKey,
	}

	_, err = client.GetObject(ctx, getIn)
	if err == nil {
		t.Error("GET after DELETE should have failed, but succeeded")
	} else {
		t.Logf("Verified deletion: GET after DELETE correctly returned error: %v", err)
	}
}

// TestRBAC_CrossBucket_Denied tests that cross-bucket access is denied.
// This attempts to access a different bucket (not armor-test-jedarden).
func TestRBAC_CrossBucket_Denied(t *testing.T) {
	ctx := context.Background()
	client := newSDKClient(t, armorTestEndpoint)

	// Try to access a different bucket (production bucket name)
	differentBucket := "armor-test-other-bucket"
	testKey := "cross-bucket-test.txt"

	// Try GET from different bucket
	getIn := &s3.GetObjectInput{
		Bucket: &differentBucket,
		Key:    &testKey,
	}

	_, err := client.GetObject(ctx, getIn)
	if err == nil {
		t.Error("Cross-bucket GET should have been denied, but succeeded")
	} else {
		t.Logf("Cross-bucket GET correctly denied: %v", err)
	}

	// Try PUT to different bucket
	testData := []byte("Cross-bucket PUT test")
	putIn := &s3.PutObjectInput{
		Bucket: &differentBucket,
		Key:    &testKey,
		Body:   bytes.NewReader(testData),
	}

	_, err = client.PutObject(ctx, putIn)
	if err == nil {
		t.Error("Cross-bucket PUT should have been denied, but succeeded")
	} else {
		t.Logf("Cross-bucket PUT correctly denied: %v", err)
	}

	// Try DELETE in different bucket
	deleteIn := &s3.DeleteObjectInput{
		Bucket: &differentBucket,
		Key:    &testKey,
	}

	_, err = client.DeleteObject(ctx, deleteIn)
	if err == nil {
		t.Error("Cross-bucket DELETE should have been denied, but succeeded")
	} else {
		t.Logf("Cross-bucket DELETE correctly denied: %v", err)
	}

	t.Log("Cross-bucket access denial confirmed for all verbs")
}

// newSDKClient creates an S3 client configured for the armor-test endpoint.
func newSDKClient(t *testing.T, endpoint string) *s3.Client {
	t.Helper()

	// Create S3 client configured for the running armor-test service
	return s3.New(s3.Options{
		BaseEndpoint: &endpoint,
		Region:       armorTestRegion,
		Credentials:  &testCredentials{accessKey: armorTestAccessKey, secretKey: armorTestSecretKey},
		UsePathStyle: true, // ARMOR expects path-style URLs (http://host/bucket/key)
	})
}

// loadTestConfig loads ARMOR configuration for testing.
func loadTestConfig() *config.Config {
	mek, err := hex.DecodeString(armorTestMEK)
	if err != nil {
		return nil
	}

	return &config.Config{
		B2Region:          armorTestRegion,
		B2Endpoint:        "https://s3." + armorTestRegion + ".backblazeb2.com",
		B2AccessKeyID:     "00220ad670139170000000062", // From OpenBao
		B2SecretAccessKey: "K002UzuRrYPstDcEVLNIeJ24bcQ9R/k", // From OpenBao
		Bucket:            armorTestBucket,
		MEK:               mek,
		BlockSize:         65536,
		CacheMaxEntries:   1000,
		CacheTTL:          300,
		AuthAccessKey:     armorTestAccessKey,
		AuthSecretKey:     armorTestSecretKey,
		Credentials: map[string]*config.Credential{
			armorTestAccessKey: {
				AccessKey: armorTestAccessKey,
				SecretKey: armorTestSecretKey,
				ACLs:      nil, // No ACL restrictions = full access
			},
		},
	}
}

// testCredentials implements aws.CredentialsProvider for the S3 client.
type testCredentials struct {
	accessKey string
	secretKey string
}

func (c *testCredentials) Retrieve(_ context.Context) (aws.Credentials, error) {
	return aws.Credentials{
		AccessKeyID:     c.accessKey,
		SecretAccessKey: c.secretKey,
	}, nil
}