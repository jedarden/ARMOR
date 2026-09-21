//go:build integration
// +build integration

// Package rbac tests RBAC verb coverage against B2 objects using armor-test credentials.
// This test suite verifies GET, PUT, and DELETE operations against the armor-test-jedarden
// bucket and documents allow/deny outcomes per ADR-012.
package rbac

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Non-secret armor-test configuration. The endpoint is the armor-test Service
// on iad-ci reached via port-forward:
//
//	kubectl --kubeconfig ~/.kube/iad-ci.kubeconfig -n armor-test port-forward svc/armor-test 9000:9000
const (
	// armor-test bucket and region from ConfigMap
	armorTestBucket = "armor-test-jedarden"
	armorTestRegion = "us-west-002"

	// armor-test service endpoint (localhost via port-forward)
	armorTestEndpoint = "http://localhost:9000"
)

// armorTestAuth returns the armor-test credential pair from the environment.
// The values live at secret/rs-manager/iad-ci/armor-test (AUTH_ACCESS_KEY /
// AUTH_SECRET_KEY) and must never be committed to this repo (armor-ad708bfd:
// the literals compiled in from ad0d46b7, public 2026-08-19 to 2026-09-20, and
// every leaked value — this pair, the MEK, and the B2 backend key — has since
// been rotated; the MEK and B2 key are server-side concerns the S3 test client
// never needs). Populate the env from OpenBao and the suite skips itself when
// they are absent:
//
//	export ARMOR_TEST_ACCESS_KEY="$(bao-as rs-manager bao kv get -field=AUTH_ACCESS_KEY secret/rs-manager/iad-ci/armor-test)"
//	export ARMOR_TEST_SECRET_KEY="$(bao-as rs-manager bao kv get -field=AUTH_SECRET_KEY secret/rs-manager/iad-ci/armor-test)"
//	go test -tags integration ./tests/rbac/
func armorTestAuth(t *testing.T) (accessKey, secretKey string) {
	t.Helper()
	accessKey = os.Getenv("ARMOR_TEST_ACCESS_KEY")
	secretKey = os.Getenv("ARMOR_TEST_SECRET_KEY")
	if accessKey == "" || secretKey == "" {
		t.Skip("ARMOR_TEST_ACCESS_KEY/ARMOR_TEST_SECRET_KEY not set — populate from OpenBao (secret/rs-manager/iad-ci/armor-test), see armorTestAuth for the bao-as commands")
	}
	return accessKey, secretKey
}

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
	accessKey, secretKey := armorTestAuth(t)

	// Create S3 client configured for the running armor-test service
	return s3.New(s3.Options{
		BaseEndpoint: &endpoint,
		Region:       armorTestRegion,
		Credentials:  &testCredentials{accessKey: accessKey, secretKey: secretKey},
		UsePathStyle: true, // ARMOR expects path-style URLs (http://host/bucket/key)
	})
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
