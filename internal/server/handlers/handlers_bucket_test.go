package handlers_test

import (
	"bytes"
	"crypto/rand"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/keymanager"
	"github.com/jedarden/armor/internal/server/handlers"
)

// TestHeadBucket tests S3 HeadBucket
func TestHeadBucket(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Create an object (which implicitly creates the bucket in our mock)
	content := []byte("test content")
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/head-bucket-test/file.txt", bytes.NewReader(content))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	// Head bucket should succeed
	req = httptest.NewRequest(http.MethodHead, "/test-bucket", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for existing bucket, got %d", w.Code)
	}

	// Head non-existent bucket should fail
	req = httptest.NewRequest(http.MethodHead, "/non-existent-bucket", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for non-existent bucket, got %d", w.Code)
	}
}

// TestListBuckets tests S3 ListBuckets

// TestListBuckets tests S3 ListBuckets
func TestListBuckets(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Create objects in multiple buckets
	for bucketNum := 0; bucketNum < 3; bucketNum++ {
		for fileNum := 0; fileNum < 2; fileNum++ {
			content := []byte(fmt.Sprintf("Content %d-%d", bucketNum, fileNum))
			req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/bucket-%d/file%d.txt", bucketNum, fileNum), bytes.NewReader(content))
			req.Header.Set("Content-Type", "text/plain")
			w := httptest.NewRecorder()
			h.HandleRoot(w, req)
		}
	}

	// List buckets
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Parse response
	var result struct {
		Buckets struct {
			Bucket []struct {
				Name string `xml:"Name"`
			} `xml:"Bucket"`
		} `xml:"Buckets"`
	}
	if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(result.Buckets.Bucket) != 3 {
		t.Errorf("expected 3 buckets, got %d", len(result.Buckets.Bucket))
	}

	// Verify bucket names
	bucketNames := make(map[string]bool)
	for _, b := range result.Buckets.Bucket {
		bucketNames[b.Name] = true
	}
	for i := 0; i < 3; i++ {
		expectedName := fmt.Sprintf("bucket-%d", i)
		if !bucketNames[expectedName] {
			t.Errorf("expected bucket %s in list", expectedName)
		}
	}
}

// TestCreateBucket tests S3 CreateBucket

// TestCreateBucket tests S3 CreateBucket
func TestCreateBucket(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Create bucket
	req := httptest.NewRequest(http.MethodPut, "/new-test-bucket", nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify Location header
	location := w.Header().Get("Location")
	if location != "/new-test-bucket" {
		t.Errorf("expected Location /new-test-bucket, got %s", location)
	}

	// Verify bucket exists via HEAD
	req = httptest.NewRequest(http.MethodHead, "/new-test-bucket", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for created bucket, got %d", w.Code)
	}
}

// TestDeleteBucket tests S3 DeleteBucket

// TestDeleteBucket tests S3 DeleteBucket
func TestDeleteBucket(t *testing.T) {
	cfg, mb, cache, footerCache, mek := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, mek, nil)

	// Create an empty bucket
	req := httptest.NewRequest(http.MethodPut, "/bucket-to-delete", nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	// Delete the bucket
	req = httptest.NewRequest(http.MethodDelete, "/bucket-to-delete", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}

	// Verify bucket no longer exists
	req = httptest.NewRequest(http.MethodHead, "/bucket-to-delete", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for deleted bucket, got %d", w.Code)
	}
}

// TestAbortMultipartUpload tests S3 AbortMultipartUpload

// mockBackendWithLifecycle is a mock backend that supports lifecycle configuration for testing
type mockBackendWithLifecycle struct {
	*mockBackend
	lifecycleConfig []byte
}

func newMockBackendWithLifecycle() *mockBackendWithLifecycle {
	return &mockBackendWithLifecycle{
		mockBackend: newMockBackend(),
	}
}

// TestGetBucketLifecycleConfiguration tests GET ?lifecycle on a bucket
func TestGetBucketLifecycleConfiguration(t *testing.T) {
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "REMOVED-NOT-A-SECRET-VALUE",
	}

	mb := newMockBackendWithLifecycle()
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// Test 1: Get lifecycle when not set - should return error
	req := httptest.NewRequest(http.MethodGet, "/test-bucket?lifecycle", nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	// Should return 500 because our mock returns an error when config doesn't exist
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500 for missing lifecycle config, got %d", w.Code)
	}

	// Test 2: Set and then get lifecycle configuration
	lifecycleXML := `<?xml version="1.0" encoding="UTF-8"?>
<LifecycleConfiguration>
  <Rule>
    <ID>test-rule</ID>
    <Status>Enabled</Status>
    <Filter>
      <Prefix>logs/</Prefix>
    </Filter>
    <Expiration>
      <Days>30</Days>
    </Expiration>
  </Rule>
</LifecycleConfiguration>`

	// PUT lifecycle configuration
	req = httptest.NewRequest(http.MethodPut, "/test-bucket?lifecycle", strings.NewReader(lifecycleXML))
	req.Header.Set("Content-Type", "application/xml")
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("PUT lifecycle: expected status 200, got %d", w.Code)
	}

	// GET lifecycle configuration
	req = httptest.NewRequest(http.MethodGet, "/test-bucket?lifecycle", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET lifecycle: expected status 200, got %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), "test-rule") {
		t.Error("GET lifecycle response should contain rule ID")
	}
}

// TestPutBucketLifecycleConfiguration tests PUT ?lifecycle on a bucket

// TestPutBucketLifecycleConfiguration tests PUT ?lifecycle on a bucket
func TestPutBucketLifecycleConfiguration(t *testing.T) {
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "REMOVED-NOT-A-SECRET-VALUE",
	}

	mb := newMockBackendWithLifecycle()
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// Test PUT with valid lifecycle configuration
	lifecycleXML := `<?xml version="1.0" encoding="UTF-8"?>
<LifecycleConfiguration>
  <Rule>
    <ID>expire-old-logs</ID>
    <Status>Enabled</Status>
    <Prefix>logs/</Prefix>
    <Expiration>
      <Days>7</Days>
    </Expiration>
  </Rule>
  <Rule>
    <ID>abort-incomplete-uploads</ID>
    <Status>Enabled</Status>
    <Filter>
      <Prefix>uploads/</Prefix>
    </Filter>
    <AbortIncompleteMultipartUpload>
      <DaysAfterInitiation>1</DaysAfterInitiation>
    </AbortIncompleteMultipartUpload>
  </Rule>
</LifecycleConfiguration>`

	req := httptest.NewRequest(http.MethodPut, "/test-bucket?lifecycle", strings.NewReader(lifecycleXML))
	req.Header.Set("Content-Type", "application/xml")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("PUT lifecycle: expected status 200, got %d", w.Code)
	}

	// Verify the configuration was stored
	if mb.lifecycleConfig == nil {
		t.Error("lifecycle configuration should have been stored")
	}
}

// TestDeleteBucketLifecycleConfiguration tests DELETE ?lifecycle on a bucket

// TestDeleteBucketLifecycleConfiguration tests DELETE ?lifecycle on a bucket
func TestDeleteBucketLifecycleConfiguration(t *testing.T) {
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "REMOVED-NOT-A-SECRET-VALUE",
	}

	mb := newMockBackendWithLifecycle()
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// First set a lifecycle configuration
	lifecycleXML := `<?xml version="1.0" encoding="UTF-8"?>
<LifecycleConfiguration>
  <Rule>
    <ID>test-rule</ID>
    <Status>Enabled</Status>
    <Prefix>test/</Prefix>
    <Expiration>
      <Days>30</Days>
    </Expiration>
  </Rule>
</LifecycleConfiguration>`

	req := httptest.NewRequest(http.MethodPut, "/test-bucket?lifecycle", strings.NewReader(lifecycleXML))
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT lifecycle: expected status 200, got %d", w.Code)
	}

	// Now delete it
	req = httptest.NewRequest(http.MethodDelete, "/test-bucket?lifecycle", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("DELETE lifecycle: expected status 204, got %d", w.Code)
	}

	// Verify it was deleted
	if mb.lifecycleConfig != nil {
		t.Error("lifecycle configuration should have been deleted")
	}
}

// mockBackendWithObjectLock is a mock backend that supports object lock operations for testing

// mockBackendWithObjectLock is a mock backend that supports object lock operations for testing
type mockBackendWithObjectLock struct {
	*mockBackend
	objectLockConfig map[string][]byte // bucket -> config
	retentionConfig  map[string][]byte // bucket/key -> config
	legalHoldConfig  map[string][]byte // bucket/key -> config
}

func newMockBackendWithObjectLock() *mockBackendWithObjectLock {
	return &mockBackendWithObjectLock{
		mockBackend:      newMockBackend(),
		objectLockConfig: make(map[string][]byte),
		retentionConfig:  make(map[string][]byte),
		legalHoldConfig:  make(map[string][]byte),
	}
}

// TestGetObjectLockConfiguration tests GET ?object-lock on a bucket
func TestGetObjectLockConfiguration(t *testing.T) {
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "REMOVED-NOT-A-SECRET-VALUE",
	}

	mb := newMockBackendWithObjectLock()
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// The public router rejects this subresource; exercise the lower-level
	// passthrough method directly so its backend adapter remains covered.
	req := httptest.NewRequest(http.MethodGet, "/test-bucket?object-lock", nil)
	w := httptest.NewRecorder()
	h.GetObjectLockConfiguration(w, req, "test-bucket")

	// Should return 500 because our mock returns an error when config doesn't exist
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500 for missing object lock config, got %d", w.Code)
	}

	// Test 2: Set and then get object lock configuration
	objectLockXML := `<?xml version="1.0" encoding="UTF-8"?>
<ObjectLockConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <ObjectLockEnabled>Enabled</ObjectLockEnabled>
  <Rule>
    <DefaultRetention>
      <Mode>GOVERNANCE</Mode>
      <Days>30</Days>
    </DefaultRetention>
  </Rule>
</ObjectLockConfiguration>`

	// PUT object lock configuration
	req = httptest.NewRequest(http.MethodPut, "/test-bucket?object-lock", strings.NewReader(objectLockXML))
	req.Header.Set("Content-Type", "application/xml")
	w = httptest.NewRecorder()
	h.PutObjectLockConfiguration(w, req, "test-bucket")

	if w.Code != http.StatusOK {
		t.Errorf("PUT object-lock: expected status 200, got %d", w.Code)
	}

	// GET object lock configuration
	req = httptest.NewRequest(http.MethodGet, "/test-bucket?object-lock", nil)
	w = httptest.NewRecorder()
	h.GetObjectLockConfiguration(w, req, "test-bucket")

	if w.Code != http.StatusOK {
		t.Errorf("GET object-lock: expected status 200, got %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), "Enabled") {
		t.Error("GET object-lock response should contain ObjectLockEnabled")
	}
}

// TestPutObjectLockConfiguration tests PUT ?object-lock on a bucket

// TestPutObjectLockConfiguration tests PUT ?object-lock on a bucket
func TestPutObjectLockConfiguration(t *testing.T) {
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "REMOVED-NOT-A-SECRET-VALUE",
	}

	mb := newMockBackendWithObjectLock()
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// Test PUT with valid object lock configuration
	objectLockXML := `<?xml version="1.0" encoding="UTF-8"?>
<ObjectLockConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <ObjectLockEnabled>Enabled</ObjectLockEnabled>
  <Rule>
    <DefaultRetention>
      <Mode>COMPLIANCE</Mode>
      <Years>1</Years>
    </DefaultRetention>
  </Rule>
</ObjectLockConfiguration>`

	req := httptest.NewRequest(http.MethodPut, "/test-bucket?object-lock", strings.NewReader(objectLockXML))
	req.Header.Set("Content-Type", "application/xml")
	w := httptest.NewRecorder()
	h.PutObjectLockConfiguration(w, req, "test-bucket")

	if w.Code != http.StatusOK {
		t.Errorf("PUT object-lock: expected status 200, got %d", w.Code)
	}

	// Verify the configuration was stored
	if mb.objectLockConfig["test-bucket"] == nil {
		t.Error("object lock configuration should have been stored")
	}
}

// TestGetObjectRetention tests GET ?retention on an object

// TestGetObjectRetention tests GET ?retention on an object
func TestGetObjectRetention(t *testing.T) {
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "REMOVED-NOT-A-SECRET-VALUE",
	}

	mb := newMockBackendWithObjectLock()
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// The public router rejects this subresource; exercise the lower-level
	// passthrough methods directly so their backend adapters remain covered.
	req := httptest.NewRequest(http.MethodGet, "/test-bucket/test-object.txt?retention", nil)
	w := httptest.NewRecorder()
	h.GetObjectRetention(w, req, "test-bucket", "test-object.txt")

	// Should return 500 because our mock returns an error when config doesn't exist
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500 for missing retention, got %d", w.Code)
	}

	// Test 2: Set and then get retention
	retentionXML := `<?xml version="1.0" encoding="UTF-8"?>
<Retention xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Mode>GOVERNANCE</Mode>
  <RetainUntilDate>2026-12-31T00:00:00Z</RetainUntilDate>
</Retention>`

	// PUT retention
	req = httptest.NewRequest(http.MethodPut, "/test-bucket/test-object.txt?retention", strings.NewReader(retentionXML))
	req.Header.Set("Content-Type", "application/xml")
	w = httptest.NewRecorder()
	h.PutObjectRetention(w, req, "test-bucket", "test-object.txt")

	if w.Code != http.StatusOK {
		t.Errorf("PUT retention: expected status 200, got %d", w.Code)
	}

	// GET retention
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/test-object.txt?retention", nil)
	w = httptest.NewRecorder()
	h.GetObjectRetention(w, req, "test-bucket", "test-object.txt")

	if w.Code != http.StatusOK {
		t.Errorf("GET retention: expected status 200, got %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), "GOVERNANCE") {
		t.Error("GET retention response should contain Mode")
	}
}

// TestPutObjectRetention tests PUT ?retention on an object

// TestPutObjectRetention tests PUT ?retention on an object
func TestPutObjectRetention(t *testing.T) {
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "REMOVED-NOT-A-SECRET-VALUE",
	}

	mb := newMockBackendWithObjectLock()
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// Test PUT with valid retention configuration
	retentionXML := `<?xml version="1.0" encoding="UTF-8"?>
<Retention xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Mode>COMPLIANCE</Mode>
  <RetainUntilDate>2027-01-01T00:00:00Z</RetainUntilDate>
</Retention>`

	req := httptest.NewRequest(http.MethodPut, "/test-bucket/retained-file.txt?retention", strings.NewReader(retentionXML))
	req.Header.Set("Content-Type", "application/xml")
	w := httptest.NewRecorder()
	h.PutObjectRetention(w, req, "test-bucket", "retained-file.txt")

	if w.Code != http.StatusOK {
		t.Errorf("PUT retention: expected status 200, got %d", w.Code)
	}

	// Verify the retention was stored
	key := "test-bucket/retained-file.txt"
	if mb.retentionConfig[key] == nil {
		t.Error("retention configuration should have been stored")
	}
}

// TestGetObjectLegalHold tests GET ?legal-hold on an object

// TestGetObjectLegalHold tests GET ?legal-hold on an object
func TestGetObjectLegalHold(t *testing.T) {
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "REMOVED-NOT-A-SECRET-VALUE",
	}

	mb := newMockBackendWithObjectLock()
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// The public router rejects this subresource; exercise the lower-level
	// passthrough methods directly so their backend adapters remain covered.
	req := httptest.NewRequest(http.MethodGet, "/test-bucket/test-object.txt?legal-hold", nil)
	w := httptest.NewRecorder()
	h.GetObjectLegalHold(w, req, "test-bucket", "test-object.txt")

	// Should return 500 because our mock returns an error when config doesn't exist
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500 for missing legal hold, got %d", w.Code)
	}

	// Test 2: Set and then get legal hold
	legalHoldXML := `<?xml version="1.0" encoding="UTF-8"?>
<LegalHold xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Status>ON</Status>
</LegalHold>`

	// PUT legal hold
	req = httptest.NewRequest(http.MethodPut, "/test-bucket/test-object.txt?legal-hold", strings.NewReader(legalHoldXML))
	req.Header.Set("Content-Type", "application/xml")
	w = httptest.NewRecorder()
	h.PutObjectLegalHold(w, req, "test-bucket", "test-object.txt")

	if w.Code != http.StatusOK {
		t.Errorf("PUT legal-hold: expected status 200, got %d", w.Code)
	}

	// GET legal hold
	req = httptest.NewRequest(http.MethodGet, "/test-bucket/test-object.txt?legal-hold", nil)
	w = httptest.NewRecorder()
	h.GetObjectLegalHold(w, req, "test-bucket", "test-object.txt")

	if w.Code != http.StatusOK {
		t.Errorf("GET legal-hold: expected status 200, got %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), "ON") {
		t.Error("GET legal-hold response should contain Status")
	}
}

// TestPutObjectLegalHold tests PUT ?legal-hold on an object

// TestPutObjectLegalHold tests PUT ?legal-hold on an object
func TestPutObjectLegalHold(t *testing.T) {
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}

	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "REMOVED-NOT-A-SECRET-VALUE",
	}

	mb := newMockBackendWithObjectLock()
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	// Test PUT with legal hold ON
	legalHoldOnXML := `<?xml version="1.0" encoding="UTF-8"?>
<LegalHold xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Status>ON</Status>
</LegalHold>`

	req := httptest.NewRequest(http.MethodPut, "/test-bucket/legal-hold-file.txt?legal-hold", strings.NewReader(legalHoldOnXML))
	req.Header.Set("Content-Type", "application/xml")
	w := httptest.NewRecorder()
	h.PutObjectLegalHold(w, req, "test-bucket", "legal-hold-file.txt")

	if w.Code != http.StatusOK {
		t.Errorf("PUT legal-hold ON: expected status 200, got %d", w.Code)
	}

	// Verify the legal hold was stored
	key := "test-bucket/legal-hold-file.txt"
	if mb.legalHoldConfig[key] == nil {
		t.Error("legal hold configuration should have been stored")
	}

	// Test PUT with legal hold OFF (removing the hold)
	legalHoldOffXML := `<?xml version="1.0" encoding="UTF-8"?>
<LegalHold xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Status>OFF</Status>
</LegalHold>`

	req = httptest.NewRequest(http.MethodPut, "/test-bucket/legal-hold-file.txt?legal-hold", strings.NewReader(legalHoldOffXML))
	req.Header.Set("Content-Type", "application/xml")
	w = httptest.NewRecorder()
	h.PutObjectLegalHold(w, req, "test-bucket", "legal-hold-file.txt")

	if w.Code != http.StatusOK {
		t.Errorf("PUT legal-hold OFF: expected status 200, got %d", w.Code)
	}

	// Verify the legal hold was updated
	if !bytes.Contains(mb.legalHoldConfig[key], []byte("OFF")) {
		t.Error("legal hold should have been updated to OFF")
	}
}

// TestListObjectVersions tests the ListObjectVersions operation
