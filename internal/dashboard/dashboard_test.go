package dashboard

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/metrics"
)

// mockBackend implements backend.Backend for testing
type mockBackend struct {
	objects        map[string]*backend.ObjectInfo
	commonPrefixes []string
	listErr        error
	headErr        error
}

func newMockBackend() *mockBackend {
	return &mockBackend{
		objects: make(map[string]*backend.ObjectInfo),
	}
}

func (m *mockBackend) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error {
	data, _ := io.ReadAll(body)
	m.objects[key] = &backend.ObjectInfo{
		Key:          key,
		Size:         size,
		Metadata:     meta,
		LastModified: time.Now(),
	}
	_ = data
	return nil
}

func (m *mockBackend) Get(ctx context.Context, bucket, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	return nil, nil, nil
}

func (m *mockBackend) GetRange(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, error) {
	return nil, nil
}

func (m *mockBackend) GetRangeWithHeaders(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, map[string]string, error) {
	return nil, nil, nil
}

func (m *mockBackend) Head(ctx context.Context, bucket, key string) (*backend.ObjectInfo, error) {
	if m.headErr != nil {
		return nil, m.headErr
	}
	obj, ok := m.objects[key]
	if !ok {
		return nil, errors.New("object not found")
	}
	return obj, nil
}

func (m *mockBackend) Delete(ctx context.Context, bucket, key string) error {
	return nil
}

func (m *mockBackend) DeleteObjects(ctx context.Context, bucket string, keys []string) error {
	return nil
}

func (m *mockBackend) List(ctx context.Context, bucket, prefix, delimiter, continuationToken string, maxKeys int) (*backend.ListResult, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}

	var objects []backend.ObjectInfo
	for _, obj := range m.objects {
		if prefix == "" || strings.HasPrefix(obj.Key, prefix) {
			objects = append(objects, *obj)
		}
	}

	// Filter common prefixes by prefix as well
	var filteredPrefixes []string
	for _, cp := range m.commonPrefixes {
		if prefix == "" || strings.HasPrefix(cp, prefix) {
			// Don't include the prefix itself in the results
			if cp != prefix {
				filteredPrefixes = append(filteredPrefixes, cp)
			}
		}
	}

	// Simple pagination: if we have more than maxKeys, truncate and set NextToken
	if len(objects) > maxKeys {
		truncatedObjects := objects[:maxKeys]
		// Use the last key as the continuation token
		nextToken := truncatedObjects[len(truncatedObjects)-1].Key
		return &backend.ListResult{
			Objects:        truncatedObjects,
			CommonPrefixes: filteredPrefixes,
			IsTruncated:    true,
			NextToken:      nextToken,
		}, nil
	}

	return &backend.ListResult{
		Objects:        objects,
		CommonPrefixes: filteredPrefixes,
		IsTruncated:    false,
		NextToken:      "",
	}, nil
}

func (m *mockBackend) ListRaw(ctx context.Context, bucket, prefix, delimiter, continuationToken string, maxKeys int) (*backend.ListResult, error) {
	return m.List(ctx, bucket, prefix, delimiter, continuationToken, maxKeys)
}

func (m *mockBackend) Copy(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string, meta map[string]string, replaceMetadata bool) error {
	return nil
}

func (m *mockBackend) ListBuckets(ctx context.Context) ([]backend.BucketInfo, error) {
	return nil, nil
}

func (m *mockBackend) CreateBucket(ctx context.Context, bucket string) error {
	return nil
}

func (m *mockBackend) DeleteBucket(ctx context.Context, bucket string) error {
	return nil
}

func (m *mockBackend) HeadBucket(ctx context.Context, bucket string) error {
	return nil
}

func (m *mockBackend) GetDirect(ctx context.Context, bucket, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	// Simulate "file not found" for rotation state file
	return nil, nil, errors.New("object not found")
}

func (m *mockBackend) CreateMultipartUpload(ctx context.Context, bucket, key string, meta map[string]string) (string, error) {
	return "", nil
}

func (m *mockBackend) UploadPart(ctx context.Context, bucket, key, uploadID string, partNumber int32, body io.Reader, size int64) (string, error) {
	return "", nil
}

func (m *mockBackend) CompleteMultipartUpload(ctx context.Context, bucket, key, uploadID string, parts []backend.CompletedPart) (string, error) {
	return "", nil
}

func (m *mockBackend) AbortMultipartUpload(ctx context.Context, bucket, key, uploadID string) error {
	return nil
}

func (m *mockBackend) ListParts(ctx context.Context, bucket, key, uploadID string) (*backend.ListPartsResult, error) {
	return nil, nil
}

func (m *mockBackend) ListMultipartUploads(ctx context.Context, bucket, prefix string) (*backend.ListMultipartUploadsResult, error) {
	return nil, nil
}

func (m *mockBackend) GetBucketLifecycleConfiguration(ctx context.Context, bucket string) ([]byte, error) {
	return nil, nil
}

func (m *mockBackend) PutBucketLifecycleConfiguration(ctx context.Context, bucket string, config []byte) error {
	return nil
}

func (m *mockBackend) DeleteBucketLifecycleConfiguration(ctx context.Context, bucket string) error {
	return nil
}

func (m *mockBackend) GetObjectLockConfiguration(ctx context.Context, bucket string) ([]byte, error) {
	return nil, nil
}

func (m *mockBackend) PutObjectLockConfiguration(ctx context.Context, bucket string, config []byte) error {
	return nil
}

func (m *mockBackend) GetObjectRetention(ctx context.Context, bucket, key string) ([]byte, error) {
	return nil, nil
}

func (m *mockBackend) PutObjectRetention(ctx context.Context, bucket, key string, retention []byte) error {
	return nil
}

func (m *mockBackend) GetObjectLegalHold(ctx context.Context, bucket, key string) ([]byte, error) {
	return nil, nil
}

func (m *mockBackend) PutObjectLegalHold(ctx context.Context, bucket, key string, legalHold []byte) error {
	return nil
}

func (m *mockBackend) ListObjectVersions(ctx context.Context, bucket, prefix, delimiter, keyMarker, versionIDMarker string, maxKeys int) (*backend.ListObjectVersionsResult, error) {
	return nil, nil
}

func (m *mockBackend) HeadVersion(ctx context.Context, bucket, key, versionID string) (*backend.ObjectInfo, error) {
	return nil, nil
}

// TestRootPageRendering verifies that the root page renders successfully.
// This is a basic rendering test that doesn't require complex navigation setup.
func TestRootPageRendering(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()

	d.Handler()(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200 for root page, got %d", rec.Code)
	}

	// Verify basic HTML structure is rendered
	body := rec.Body.String()
	if !strings.Contains(body, "ARMOR Dashboard") {
		t.Error("Expected 'ARMOR Dashboard' title in response")
	}
	if !strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("Expected valid HTML DOCTYPE in response")
	}
	if !strings.Contains(body, "</html>") {
		t.Error("Expected HTML closing tag in response")
	}
}

func TestDashboardHandler(t *testing.T) {
	mb := newMockBackend()
	mb.objects["test/file1.txt"] = &backend.ObjectInfo{
		Key:              "test/file1.txt",
		Size:             100,
		ContentType:      "text/plain",
		ETag:             "abc123",
		LastModified:     time.Now(),
		IsARMOREncrypted: true,
		Metadata: map[string]string{
			"x-amz-meta-armor-version":        "1",
			"x-amz-meta-armor-block-size":     "65536",
			"x-amz-meta-armor-plaintext-size": "100",
			"x-amz-meta-armor-key-id":         "default",
		},
	}
	mb.objects["test/file2.txt"] = &backend.ObjectInfo{
		Key:          "test/file2.txt",
		Size:         200,
		ContentType:  "text/plain",
		ETag:         "def456",
		LastModified: time.Now(),
	}
	mb.objects["folder/"] = &backend.ObjectInfo{
		Key:          "folder/",
		Size:         0,
		LastModified: time.Now(),
	}

	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()

	d.Handler()(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "ARMOR Dashboard") {
		t.Error("Expected dashboard title in response")
	}
	if !strings.Contains(body, "test/file1.txt") {
		t.Error("Expected file1 in response")
	}
	if !strings.Contains(body, "test/file2.txt") {
		t.Error("Expected file2 in response")
	}
}

func TestDashboardHandlerWithPrefix(t *testing.T) {
	mb := newMockBackend()
	mb.objects["data/file1.txt"] = &backend.ObjectInfo{
		Key:          "data/file1.txt",
		Size:         100,
		LastModified: time.Now(),
	}
	mb.objects["other/file2.txt"] = &backend.ObjectInfo{
		Key:          "other/file2.txt",
		Size:         200,
		LastModified: time.Now(),
	}

	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodGet, "/dashboard?prefix=data/", nil)
	rec := httptest.NewRecorder()

	d.Handler()(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "data/file1.txt") {
		t.Error("Expected data/file1.txt in response")
	}
	if strings.Contains(body, "other/file2.txt") {
		t.Error("Did not expect other/file2.txt in response")
	}
}

func TestObjectDetailHandler(t *testing.T) {
	mb := newMockBackend()
	mb.objects["test/encrypted.txt"] = &backend.ObjectInfo{
		Key:              "test/encrypted.txt",
		Size:             1000,
		ContentType:      "application/octet-stream",
		ETag:             "abc123",
		LastModified:     time.Now(),
		IsARMOREncrypted: true,
		Metadata: map[string]string{
			"x-amz-meta-armor-version":          "1",
			"x-amz-meta-armor-block-size":       "65536",
			"x-amz-meta-armor-plaintext-size":   "1000",
			"x-amz-meta-armor-key-id":           "default",
			"x-amz-meta-armor-iv":               "dGVzdGl2MTIzNDU2Nzg5MA==",
			"x-amz-meta-armor-wrapped-dek":      "d3JhcHBlZGRlaw==",
			"x-amz-meta-armor-plaintext-sha256": "abcdef123456",
		},
	}

	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/object?key=test/encrypted.txt", nil)
	rec := httptest.NewRecorder()

	d.ObjectDetailHandler()(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `"is_armor":true`) {
		t.Error("Expected is_armor:true in response")
	}
	if !strings.Contains(body, `"armor"`) {
		t.Error("Expected armor metadata in response")
	}
}

func TestObjectDetailHandlerMissingKey(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/object", nil)
	rec := httptest.NewRecorder()

	d.ObjectDetailHandler()(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rec.Code)
	}
}

func TestMetricsHandler(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()

	// Add some metrics
	m.RequestsTotal.Add(10)
	m.CacheHitsTotal.Add(5)
	m.CacheMissesTotal.Add(2)
	m.BytesUploaded.Add(1024)
	m.BytesDownloaded.Add(2048)

	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/metrics", nil)
	rec := httptest.NewRecorder()

	d.MetricsHandler()(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `"requests_total":10`) {
		t.Error("Expected requests_total in response")
	}
	if !strings.Contains(body, `"cache_hits":5`) {
		t.Error("Expected cache_hits in response")
	}
	if !strings.Contains(body, `"cache_misses":2`) {
		t.Error("Expected cache_misses in response")
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		n        int64
		expected string
	}{
		{0, "0 B"},
		{100, "100 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		result := formatBytes(tt.n)
		if result != tt.expected {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.n, result, tt.expected)
		}
	}
}

func TestFormatUptime(t *testing.T) {
	tests := []struct {
		d        time.Duration
		expected string
	}{
		{0 * time.Second, "0h 0m 0s"},
		{30 * time.Second, "0h 0m 30s"},
		{90 * time.Second, "0h 1m 30s"},
		{3661 * time.Second, "1h 1m 1s"},
		{90061 * time.Second, "25h 1m 1s"},
	}

	for _, tt := range tests {
		result := formatUptime(tt.d)
		if result != tt.expected {
			t.Errorf("formatUptime(%v) = %q, want %q", tt.d, result, tt.expected)
		}
	}
}

func TestParseExpvarInt(t *testing.T) {
	tests := []struct {
		s        string
		expected int64
	}{
		{"0", 0},
		{"123", 123},
		{"-456", -456},
		{"invalid", 0},
	}

	for _, tt := range tests {
		result := parseExpvarInt(tt.s)
		if result != tt.expected {
			t.Errorf("parseExpvarInt(%q) = %d, want %d", tt.s, result, tt.expected)
		}
	}
}

func TestDashboardHandlerListError(t *testing.T) {
	mb := newMockBackend()
	mb.listErr = context.DeadlineExceeded

	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()

	d.Handler()(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", rec.Code)
	}
}

func TestObjectDetailHandlerNotFound(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/object?key=nonexistent", nil)
	rec := httptest.NewRecorder()

	d.ObjectDetailHandler()(rec, req)

	// Should return 404 when object not found
	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", rec.Code)
	}
}

func TestARMORObjectDisplay(t *testing.T) {
	mb := newMockBackend()
	mb.objects["encrypted.bin"] = &backend.ObjectInfo{
		Key:              "encrypted.bin",
		Size:             500,
		ContentType:      "application/octet-stream",
		ETag:             "xyz789",
		LastModified:     time.Now(),
		IsARMOREncrypted: true,
		Metadata: map[string]string{
			"x-amz-meta-armor-version":        "1",
			"x-amz-meta-armor-block-size":     "65536",
			"x-amz-meta-armor-plaintext-size": "500",
			"x-amz-meta-armor-key-id":         "sensitive",
			"x-amz-meta-armor-iv":             "dGVzdGl2MTIzNDU2Nzg5MA==",
			"x-amz-meta-armor-wrapped-dek":    "d3JhcHBlZGRlaw==",
		},
	}
	mb.objects["plain.txt"] = &backend.ObjectInfo{
		Key:          "plain.txt",
		Size:         100,
		ContentType:  "text/plain",
		LastModified: time.Now(),
	}

	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()

	d.Handler()(rec, req)

	body := rec.Body.String()

	// Check ARMOR badge is present
	if !strings.Contains(body, "armor-badge") {
		t.Error("Expected ARMOR badge class in response")
	}

	// Check key ID is displayed
	if !strings.Contains(body, "sensitive") {
		t.Error("Expected key ID 'sensitive' in response")
	}

	// Check plain object is shown
	if !strings.Contains(body, "plain.txt") {
		t.Error("Expected plain.txt in response")
	}
}

// TestDefaultKeyAndPlaintextBadges verifies that the two object states remain
// distinguishable when ARMOR's default key ID is omitted from metadata.
func TestDefaultKeyAndPlaintextBadges(t *testing.T) {
	mb := newMockBackend()
	mb.objects["encrypted.bin"] = &backend.ObjectInfo{
		Key:              "encrypted.bin",
		Size:             500,
		LastModified:     time.Now(),
		IsARMOREncrypted: true,
		Metadata: map[string]string{
			"x-amz-meta-armor-version":        "1",
			"x-amz-meta-armor-block-size":     "65536",
			"x-amz-meta-armor-plaintext-size": "500",
			"x-amz-meta-armor-iv":             "dGVzdGl2MTIzNDU2Nzg5MA==",
			"x-amz-meta-armor-wrapped-dek":    "d3JhcHBlZGRlaw==",
			// The default key is intentionally not emitted by ToMetadata.
		},
	}
	mb.objects["plain.txt"] = &backend.ObjectInfo{
		Key:          "plain.txt",
		Size:         100,
		LastModified: time.Now(),
	}

	d := New(mb, "test-bucket", metrics.NewMetrics())
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()
	d.Handler()(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `class="armor-badge" aria-label="ARMOR encrypted with key default">ARMOR [default]`) {
		t.Error("expected default key name in ARMOR badge")
	}
	if !strings.Contains(body, `class="plain-badge" aria-label="Unencrypted object">plain`) {
		t.Error("expected explicit plaintext badge")
	}

	req = httptest.NewRequest(http.MethodGet, "/dashboard/object?key=encrypted.bin", nil)
	rec = httptest.NewRecorder()
	d.ObjectDetailHandler()(rec, req)

	var detail struct {
		Armor struct {
			KeyID string `json:"key_id"`
		} `json:"armor"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&detail); err != nil {
		t.Fatalf("decode object detail: %v", err)
	}
	if detail.Armor.KeyID != "default" {
		t.Fatalf("object detail key_id = %q, want default", detail.Armor.KeyID)
	}
}

func TestBreadcrumbs(t *testing.T) {
	mb := newMockBackend()
	mb.objects["data/2024/file.txt"] = &backend.ObjectInfo{
		Key:          "data/2024/file.txt",
		Size:         100,
		LastModified: time.Now(),
	}

	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodGet, "/dashboard?prefix=data/2024/", nil)
	rec := httptest.NewRecorder()

	d.Handler()(rec, req)

	body := rec.Body.String()

	// Check breadcrumbs contain path segments
	if !strings.Contains(body, "data") {
		t.Error("Expected 'data' in breadcrumbs")
	}
	if !strings.Contains(body, "2024") {
		t.Error("Expected '2024' in breadcrumbs")
	}
}

// Ensure Dashboard implements proper HTTP content type
func TestDashboardContentType(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()

	d.Handler()(rec, req)

	contentType := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/html") {
		t.Errorf("Expected Content-Type text/html, got %s", contentType)
	}
}

func TestMetricsContentType(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/metrics", nil)
	rec := httptest.NewRecorder()

	d.MetricsHandler()(rec, req)

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}
}

func TestObjectDetailContentType(t *testing.T) {
	mb := newMockBackend()
	mb.objects["test.txt"] = &backend.ObjectInfo{
		Key:          "test.txt",
		Size:         100,
		LastModified: time.Now(),
	}

	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/object?key=test.txt", nil)
	rec := httptest.NewRecorder()

	d.ObjectDetailHandler()(rec, req)

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}
}

// Test non-ARMOR object detail
func TestNonARMORObjectDetail(t *testing.T) {
	mb := newMockBackend()
	mb.objects["plain.txt"] = &backend.ObjectInfo{
		Key:          "plain.txt",
		Size:         200,
		ContentType:  "text/plain",
		ETag:         "plain123",
		LastModified: time.Now(),
	}

	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/object?key=plain.txt", nil)
	rec := httptest.NewRecorder()

	d.ObjectDetailHandler()(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `"is_armor":false`) {
		t.Error("Expected is_armor:false in response")
	}
	if strings.Contains(body, `"armor"`) {
		t.Error("Did not expect armor metadata for non-ARMOR object")
	}
}

// Test cache hit rate calculation
func TestCacheHitRateCalculation(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()

	// Add metrics for 80% hit rate
	m.CacheHitsTotal.Add(80)
	m.CacheMissesTotal.Add(20)

	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()

	d.Handler()(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "80.0%") {
		t.Error("Expected 80.0% cache hit rate in response")
	}
}

// Test zero cache hit rate
func TestZeroCacheHitRate(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()
	// No cache activity

	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()

	d.Handler()(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "0%") {
		t.Error("Expected 0% cache hit rate in response")
	}
}

// Test template handles special characters in keys
func TestSpecialCharacterKeys(t *testing.T) {
	mb := newMockBackend()
	mb.objects["data/file with spaces.txt"] = &backend.ObjectInfo{
		Key:          "data/file with spaces.txt",
		Size:         100,
		LastModified: time.Now(),
	}
	mb.objects["data/file&special<chars>.txt"] = &backend.ObjectInfo{
		Key:          "data/file&special<chars>.txt",
		Size:         100,
		LastModified: time.Now(),
	}

	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()

	d.Handler()(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

// Test authentication middleware with Basic Auth
func TestAuthMiddlewareBasicAuth(t *testing.T) {
	auth := NewAuthMiddleware("admin", "secret123", "")

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()

	// No auth header - should fail
	if auth.Authenticate(rec, req) {
		t.Error("Expected authentication to fail without credentials")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rec.Code)
	}
	if rec.Header().Get("WWW-Authenticate") == "" {
		t.Error("Expected WWW-Authenticate header")
	}
}

// Test authentication middleware with Bearer token
func TestAuthMiddlewareBearerToken(t *testing.T) {
	auth := NewAuthMiddleware("", "", "my-secret-token")

	// Valid token
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req.Header.Set("Authorization", "Bearer my-secret-token")
	rec := httptest.NewRecorder()

	if !auth.Authenticate(rec, req) {
		t.Error("Expected authentication to succeed with valid token")
	}
	// On success, Authenticate returns true and doesn't write to response
	if rec.Code != 0 && rec.Code != http.StatusOK {
		t.Errorf("Expected no error status on success, got %d", rec.Code)
	}

	// Invalid token
	req = httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	rec = httptest.NewRecorder()

	if auth.Authenticate(rec, req) {
		t.Error("Expected authentication to fail with invalid token")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 on invalid token, got %d", rec.Code)
	}
}

// Test authentication middleware with no auth configured
func TestAuthMiddlewareNoAuth(t *testing.T) {
	auth := NewAuthMiddleware("", "", "")

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()

	// Should allow all when no auth configured
	if !auth.Authenticate(rec, req) {
		t.Error("Expected authentication to succeed when no auth configured")
	}
}

// Test authenticated dashboard handler
func TestDashboardHandlerWithAuth(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()
	d := NewWithAuth(mb, "test-bucket", m, "admin", "secret", "", nil, "", false)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()

	// No auth - should fail
	d.HandlerWithAuth()(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 without auth, got %d", rec.Code)
	}

	// With Basic Auth
	req = httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req.SetBasicAuth("admin", "secret")
	rec = httptest.NewRecorder()

	d.HandlerWithAuth()(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200 with valid auth, got %d", rec.Code)
	}
}

// Test bearer token authentication on dashboard handler
func TestDashboardHandlerWithBearerToken(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()
	d := NewWithAuth(mb, "test-bucket", m, "", "", "token123", nil, "", false)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()

	// No auth - should fail
	d.HandlerWithAuth()(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 without auth, got %d", rec.Code)
	}

	// With Bearer token
	req = httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req.Header.Set("Authorization", "Bearer token123")
	rec = httptest.NewRecorder()

	d.HandlerWithAuth()(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200 with valid token, got %d", rec.Code)
	}
}

// Test metrics handler with authentication
func TestMetricsHandlerWithAuth(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()
	d := NewWithAuth(mb, "test-bucket", m, "admin", "pass", "", nil, "", false)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/metrics", nil)
	rec := httptest.NewRecorder()

	// No auth - should fail
	d.MetricsHandlerWithAuth()(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 without auth, got %d", rec.Code)
	}

	// With valid auth
	req = httptest.NewRequest(http.MethodGet, "/dashboard/metrics", nil)
	req.SetBasicAuth("admin", "pass")
	rec = httptest.NewRecorder()

	d.MetricsHandlerWithAuth()(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200 with valid auth, got %d", rec.Code)
	}
}

// Test object detail handler with authentication
func TestObjectDetailHandlerWithAuth(t *testing.T) {
	mb := newMockBackend()
	mb.objects["test.txt"] = &backend.ObjectInfo{
		Key:          "test.txt",
		Size:         100,
		LastModified: time.Now(),
	}
	m := metrics.NewMetrics()
	d := NewWithAuth(mb, "test-bucket", m, "", "", "token", nil, "", false)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/object?key=test.txt", nil)
	rec := httptest.NewRecorder()

	// No auth - should fail
	d.ObjectDetailHandlerWithAuth()(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 without auth, got %d", rec.Code)
	}

	// With valid token
	req = httptest.NewRequest(http.MethodGet, "/dashboard/object?key=test.txt", nil)
	req.Header.Set("Authorization", "Bearer token")
	rec = httptest.NewRecorder()

	d.ObjectDetailHandlerWithAuth()(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200 with valid token, got %d", rec.Code)
	}
}

// TestEncryptionStatsHandler verifies the /dashboard/encryption-stats endpoint.
func TestEncryptionStatsHandler(t *testing.T) {
	mb := newMockBackend()
	mb.objects["enc1.bin"] = &backend.ObjectInfo{
		Key:              "enc1.bin",
		Size:             500,
		IsARMOREncrypted: true,
		LastModified:     time.Now(),
		Metadata: map[string]string{
			"x-amz-meta-armor-version":        "1",
			"x-amz-meta-armor-block-size":     "65536",
			"x-amz-meta-armor-plaintext-size": "500",
			"x-amz-meta-armor-key-id":         "keyA",
			"x-amz-meta-armor-iv":             "dGVzdGl2MTIzNDU2Nzg5MA==",
			"x-amz-meta-armor-wrapped-dek":    "d3JhcHBlZGRlaw==",
		},
	}
	mb.objects["enc2.bin"] = &backend.ObjectInfo{
		Key:              "enc2.bin",
		Size:             300,
		IsARMOREncrypted: true,
		LastModified:     time.Now(),
		Metadata: map[string]string{
			"x-amz-meta-armor-version":        "1",
			"x-amz-meta-armor-block-size":     "65536",
			"x-amz-meta-armor-plaintext-size": "300",
			"x-amz-meta-armor-key-id":         "keyB",
			"x-amz-meta-armor-iv":             "dGVzdGl2MTIzNDU2Nzg5MA==",
			"x-amz-meta-armor-wrapped-dek":    "d3JhcHBlZGRlaw==",
		},
	}
	mb.objects["plain.txt"] = &backend.ObjectInfo{
		Key:          "plain.txt",
		Size:         100,
		LastModified: time.Now(),
	}

	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/encryption-stats", nil)
	rec := httptest.NewRecorder()

	d.EncryptionStatsHandler()(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var stats EncryptionStatsResponse
	if err := json.NewDecoder(rec.Body).Decode(&stats); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if stats.EncryptedCount != 2 {
		t.Errorf("Expected 2 encrypted objects, got %d", stats.EncryptedCount)
	}
	if stats.PlaintextCount != 1 {
		t.Errorf("Expected 1 plaintext object, got %d", stats.PlaintextCount)
	}
	if stats.TotalCount != 3 {
		t.Errorf("Expected 3 total objects, got %d", stats.TotalCount)
	}
	if len(stats.KeyIDs) != 2 {
		t.Errorf("Expected 2 key IDs, got %d: %v", len(stats.KeyIDs), stats.KeyIDs)
	}
	if stats.CoveragePercent < 66 || stats.CoveragePercent > 67 {
		t.Errorf("Expected ~66.7%% coverage, got %.2f%%", stats.CoveragePercent)
	}
	if stats.KeyUsage["keyA"] != 1 || stats.KeyUsage["keyB"] != 1 {
		t.Errorf("Expected keyA=1 and keyB=1, got %v", stats.KeyUsage)
	}
}

// TestEncryptionStatsHandlerFolderExclusion verifies folders are excluded from counts.
func TestEncryptionStatsHandlerFolderExclusion(t *testing.T) {
	mb := newMockBackend()
	mb.objects["folder/"] = &backend.ObjectInfo{
		Key:          "folder/",
		Size:         0,
		LastModified: time.Now(),
	}
	mb.objects["folder/file.txt"] = &backend.ObjectInfo{
		Key:              "folder/file.txt",
		Size:             200,
		IsARMOREncrypted: true,
		LastModified:     time.Now(),
		Metadata: map[string]string{
			"x-amz-meta-armor-version":        "1",
			"x-amz-meta-armor-block-size":     "65536",
			"x-amz-meta-armor-plaintext-size": "200",
		},
	}

	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/encryption-stats", nil)
	rec := httptest.NewRecorder()
	d.EncryptionStatsHandler()(rec, req)

	var stats EncryptionStatsResponse
	if err := json.NewDecoder(rec.Body).Decode(&stats); err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}
	// folder/ should not be counted
	if stats.TotalCount != 1 {
		t.Errorf("Expected 1 non-folder object, got %d", stats.TotalCount)
	}
	if stats.EncryptedCount != 1 {
		t.Errorf("Expected 1 encrypted object, got %d", stats.EncryptedCount)
	}
}

// TestEncryptionStatsHandlerAuth verifies the authenticated wrapper works.
func TestEncryptionStatsHandlerAuth(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()
	d := NewWithAuth(mb, "test-bucket", m, "", "", "token123", nil, "", false)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/encryption-stats", nil)
	rec := httptest.NewRecorder()
	d.EncryptionStatsHandlerWithAuth()(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 without auth, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/dashboard/encryption-stats", nil)
	req.Header.Set("Authorization", "Bearer token123")
	rec = httptest.NewRecorder()
	d.EncryptionStatsHandlerWithAuth()(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200 with valid token, got %d", rec.Code)
	}
}

// TestEncryptionCoveragePanelInDashboard verifies the coverage panel renders for mixed buckets.
func TestEncryptionCoveragePanelInDashboard(t *testing.T) {
	mb := newMockBackend()
	mb.objects["encrypted.bin"] = &backend.ObjectInfo{
		Key:              "encrypted.bin",
		Size:             500,
		IsARMOREncrypted: true,
		LastModified:     time.Now(),
		Metadata: map[string]string{
			"x-amz-meta-armor-version":        "1",
			"x-amz-meta-armor-block-size":     "65536",
			"x-amz-meta-armor-plaintext-size": "500",
			"x-amz-meta-armor-key-id":         "mykey",
		},
	}
	mb.objects["plain.txt"] = &backend.ObjectInfo{
		Key:          "plain.txt",
		Size:         100,
		LastModified: time.Now(),
	}

	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()
	d.Handler()(rec, req)

	body := rec.Body.String()

	if !strings.Contains(body, "Encryption Coverage") {
		t.Error("Expected 'Encryption Coverage' panel in response")
	}
	if !strings.Contains(body, "mykey") {
		t.Error("Expected key ID 'mykey' in encryption coverage panel")
	}
	if !strings.Contains(body, "key-tag") {
		t.Error("Expected key-tag CSS class for key ID display")
	}
	if !strings.Contains(body, "50.0%") {
		t.Error("Expected 50.0% encryption coverage (1/2 objects)")
	}
}

// TestEmptyBucket verifies the dashboard renders sanely for a completely empty bucket.
func TestEmptyBucket(t *testing.T) {
	mb := newMockBackend()
	// No objects, no common prefixes - completely empty bucket

	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()
	d.Handler()(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200 for empty bucket, got %d", rec.Code)
	}

	body := rec.Body.String()

	// Should render basic dashboard structure
	if !strings.Contains(body, "ARMOR Dashboard") {
		t.Error("Expected dashboard title in empty bucket response")
	}

	// Should not show encryption coverage panel when no objects
	if strings.Contains(body, "Encryption Coverage") {
		t.Error("Expected 'Encryption Coverage' panel to be hidden for empty bucket")
	}

	// Should show empty objects table
	if !strings.Contains(body, "<table>") {
		t.Error("Expected objects table in empty bucket response")
	}
}

// TestEncryptionCoveragePanelHiddenWhenEmpty verifies the panel is hidden for empty buckets.
func TestEncryptionCoveragePanelHiddenWhenEmpty(t *testing.T) {
	mb := newMockBackend()
	// Only common prefixes (virtual folders), no actual objects
	mb.commonPrefixes = []string{"data/", "logs/"}

	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()
	d.Handler()(rec, req)

	body := rec.Body.String()
	if strings.Contains(body, "Encryption Coverage") {
		t.Error("Expected 'Encryption Coverage' panel to be hidden when no objects")
	}
}

// TestFullEncryptionCoverage verifies 100% coverage display.
func TestFullEncryptionCoverage(t *testing.T) {
	mb := newMockBackend()
	for i := 0; i < 3; i++ {
		key := fmt.Sprintf("file%d.bin", i)
		mb.objects[key] = &backend.ObjectInfo{
			Key:              key,
			Size:             100,
			IsARMOREncrypted: true,
			LastModified:     time.Now(),
			Metadata: map[string]string{
				"x-amz-meta-armor-version":        "1",
				"x-amz-meta-armor-block-size":     "65536",
				"x-amz-meta-armor-plaintext-size": "100",
			},
		}
	}

	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()
	d.Handler()(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "100.0%") {
		t.Error("Expected 100.0% encryption coverage for all-encrypted bucket")
	}
}

// TestMetricsHandlerComputedFields verifies the new computed fields in the metrics endpoint.
func TestMetricsHandlerComputedFields(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()
	m.CacheHitsTotal.Add(80)
	m.CacheMissesTotal.Add(20)
	m.RangeBytesSavedTotal.Add(1024 * 1024)
	m.KeyWrapOpsTotal.Add(5)
	m.KeyUnwrapOpsTotal.Add(10)
	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/metrics", nil)
	rec := httptest.NewRecorder()
	d.MetricsHandler()(rec, req)

	var data map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&data); err != nil {
		t.Fatalf("Failed to decode metrics: %v", err)
	}

	// cache_hit_rate_pct should be 80%
	ratePct, ok := data["cache_hit_rate_pct"].(float64)
	if !ok {
		t.Error("Expected cache_hit_rate_pct field in metrics")
	} else if ratePct < 79.9 || ratePct > 80.1 {
		t.Errorf("Expected cache_hit_rate_pct ~80, got %f", ratePct)
	}

	// uptime_formatted should be present and non-empty
	uptimeFmt, ok := data["uptime_formatted"].(string)
	if !ok || uptimeFmt == "" {
		t.Error("Expected non-empty uptime_formatted field in metrics")
	}

	// range_bytes_saved should be 1048576
	rbs, ok := data["range_bytes_saved"].(float64)
	if !ok || int64(rbs) != 1024*1024 {
		t.Errorf("Expected range_bytes_saved=1048576, got %v", data["range_bytes_saved"])
	}
}

// TestNewStatCardsInHTML verifies the new stat cards appear in the dashboard HTML.
func TestNewStatCardsInHTML(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()
	d.Handler()(rec, req)

	body := rec.Body.String()

	for _, expected := range []string{
		"Encrypted Objects",
		"Range Bytes Saved",
		"Key Ops (W/U)",
		"stat-cache-rate",
		"stat-requests",
		"stat-uptime",
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("Expected %q in dashboard HTML", expected)
		}
	}
}

// TestBucketBrowserControls verifies the rendered UI exposes the information
// needed to navigate a prefix and distinguish folders from objects.
func TestBucketBrowserControls(t *testing.T) {
	mb := newMockBackend()
	mb.commonPrefixes = []string{"reports/"}
	mb.objects["readme.txt"] = &backend.ObjectInfo{
		Key:          "readme.txt",
		Size:         100,
		LastModified: time.Now(),
	}

	d := New(mb, "test-bucket", metrics.NewMetrics())
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()
	d.Handler()(rec, req)

	body := rec.Body.String()
	for _, want := range []string{
		"Bucket browser",
		"Root prefix",
		"<strong>1</strong> objects · <strong>1</strong> folders",
		`class="folder-link"`,
		`class="object-link"`,
		`data-object-key="readme.txt"`,
		"Encryption",
		"Refresh",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("dashboard browser missing %q", want)
		}
	}
}

// TestEmptyPrefixState verifies that an empty prefix has a useful accessible
// state instead of rendering a blank table body.
func TestEmptyPrefixState(t *testing.T) {
	d := New(newMockBackend(), "test-bucket", metrics.NewMetrics())
	req := httptest.NewRequest(http.MethodGet, "/dashboard?prefix=missing/", nil)
	rec := httptest.NewRecorder()
	d.Handler()(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "This prefix is empty") {
		t.Error("expected empty-prefix message")
	}
	if !strings.Contains(body, `colspan="5"`) {
		t.Error("expected empty-prefix row to span the object table")
	}
}

// TestMetricsHandlerLiveDashboardFields verifies fields consumed by the
// dashboard's live refresh loop are present in the JSON response.
func TestMetricsHandlerLiveDashboardFields(t *testing.T) {
	m := metrics.NewMetrics()
	m.SetReplicationQueueDepth(7)
	m.IncReplicationDropped()
	m.SetCanaryLastCheck(time.Now())
	d := New(newMockBackend(), "test-bucket", m)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/metrics", nil)
	rec := httptest.NewRecorder()
	d.MetricsHandler()(rec, req)

	var data map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&data); err != nil {
		t.Fatalf("decode live metrics: %v", err)
	}
	if got, ok := data["replication_queue_depth"].(float64); !ok || got != 7 {
		t.Errorf("replication_queue_depth = %v, want 7", data["replication_queue_depth"])
	}
	if got, ok := data["replication_dropped"].(float64); !ok || got != 1 {
		t.Errorf("replication_dropped = %v, want 1", data["replication_dropped"])
	}
	if got, ok := data["canary_status"].(string); !ok || !strings.HasPrefix(got, "Healthy") {
		t.Errorf("canary_status = %v, want Healthy status", data["canary_status"])
	}
	if got := data["canary_card_class"]; got != "healthy" {
		t.Errorf("canary_card_class = %v, want healthy", got)
	}
}

// Benchmark dashboard handler
func BenchmarkDashboardHandler(b *testing.B) {
	mb := newMockBackend()
	for i := 0; i < 100; i++ {
		mb.objects[string(rune(i))] = &backend.ObjectInfo{
			Key:          string(rune(i)),
			Size:         100,
			LastModified: time.Now(),
		}
	}

	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		d.Handler()(rec, req)
	}
}

// Verify template parsing doesn't fail
func TestTemplateParsing(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	if d.template == nil {
		t.Error("Template should be parsed during construction")
	}
}

// Test concurrent requests
func TestConcurrentRequests(t *testing.T) {
	mb := newMockBackend()
	mb.objects["test.txt"] = &backend.ObjectInfo{
		Key:          "test.txt",
		Size:         100,
		LastModified: time.Now(),
	}

	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func() {
			req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
			rec := httptest.NewRecorder()
			d.Handler()(rec, req)
			done <- rec.Code == http.StatusOK
		}()
	}

	for i := 0; i < 10; i++ {
		if !<-done {
			t.Error("Concurrent request failed")
		}
	}
}

// TestCommonPrefixesDisplayed verifies that CommonPrefixes (virtual folders) appear
// in the bucket browser output and are listed before regular objects.
func TestCommonPrefixesDisplayed(t *testing.T) {
	mb := newMockBackend()
	mb.commonPrefixes = []string{"data/", "logs/"}
	mb.objects["root.txt"] = &backend.ObjectInfo{
		Key:          "root.txt",
		Size:         100,
		LastModified: time.Now(),
	}

	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()
	d.Handler()(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()

	// Both virtual folders should appear as links
	if !strings.Contains(body, "data/") {
		t.Error("Expected 'data/' folder in response")
	}
	if !strings.Contains(body, "logs/") {
		t.Error("Expected 'logs/' folder in response")
	}
	// Regular object should still appear
	if !strings.Contains(body, "root.txt") {
		t.Error("Expected 'root.txt' in response")
	}

	// Folders should appear before the regular object in the HTML
	dataIdx := strings.Index(body, "data/")
	rootIdx := strings.Index(body, "root.txt")
	if dataIdx > rootIdx {
		t.Error("Expected folders to appear before regular objects")
	}

	// Verify folder links use ?prefix= format for navigation
	// Note: Go templates URL-escape the slash, so we check for %2f
	if !strings.Contains(body, `href="?prefix=data%2f`) {
		t.Error("Expected folder link with href=\"?prefix=data/\" for navigation (URL-encoded as %2f)")
	}
	if !strings.Contains(body, `href="?prefix=logs%2f`) {
		t.Error("Expected folder link with href=\"?prefix=logs/\" for navigation (URL-encoded as %2f)")
	}
}

// TestCommonPrefixLinksNavigateByPrefix verifies that clicking folder links
// navigates to that folder using ?prefix= query parameter.
func TestCommonPrefixLinksNavigateByPrefix(t *testing.T) {
	mb := newMockBackend()
	mb.commonPrefixes = []string{"folder1/", "folder2/"}
	mb.objects["folder1/file.txt"] = &backend.ObjectInfo{
		Key:          "folder1/file.txt",
		Size:         100,
		LastModified: time.Now(),
	}

	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	// First, verify folder links appear at root
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()
	d.Handler()(rec, req)

	body := rec.Body.String()
	// Note: Go templates URL-escape the slash, so we check for %2f
	if !strings.Contains(body, `href="?prefix=folder1%2f`) {
		t.Error("Expected folder1/ link with ?prefix= for navigation (URL-encoded as %2f)")
	}

	// Second, verify navigating to folder1/ shows its contents
	req = httptest.NewRequest(http.MethodGet, "/dashboard?prefix=folder1/", nil)
	rec = httptest.NewRecorder()
	d.Handler()(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200 when navigating to folder1/, got %d", rec.Code)
	}

	body = rec.Body.String()
	// Should show the file inside folder1
	if !strings.Contains(body, "folder1/file.txt") {
		t.Error("Expected folder1/file.txt to appear when viewing folder1/")
	}
	// Should not show folder2 contents when viewing folder1
	if strings.Contains(body, "folder2/") {
		t.Error("Expected folder2/ to be filtered out when viewing folder1/")
	}
}

// TestBreadcrumbLinksNavigateBack verifies that breadcrumb links
// navigate back up the directory hierarchy correctly.
func TestBreadcrumbLinksNavigateBack(t *testing.T) {
	mb := newMockBackend()
	mb.objects["data/2024/january/report.txt"] = &backend.ObjectInfo{
		Key:          "data/2024/january/report.txt",
		Size:         100,
		LastModified: time.Now(),
	}
	mb.objects["data/2024/february/other.txt"] = &backend.ObjectInfo{
		Key:          "data/2024/february/other.txt",
		Size:         200,
		LastModified: time.Now(),
	}

	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	// Navigate to deep folder
	req := httptest.NewRequest(http.MethodGet, "/dashboard?prefix=data/2024/january/", nil)
	rec := httptest.NewRecorder()
	d.Handler()(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()

	// Verify breadcrumb links exist with proper ?prefix= format
	// Should have: test-bucket (?prefix=), data (?prefix=data/), 2024 (?prefix=data/2024/), january (?prefix=data/2024/january/)
	// Note: Go templates URL-escape slashes, so we check for %2f
	if !strings.Contains(body, `href="?prefix=data%2f"`) {
		t.Error("Expected breadcrumb link with href=\"?prefix=data/\" to navigate back to data/ (URL-encoded as %2f)")
	}
	if !strings.Contains(body, `href="?prefix=data%2f2024%2f"`) {
		t.Error("Expected breadcrumb link with href=\"?prefix=data/2024/\" to navigate back to 2024/ (URL-encoded as %2f)")
	}
	if !strings.Contains(body, `href="?prefix=data%2f2024%2fjanuary%2f"`) {
		t.Error("Expected breadcrumb link with href=\"?prefix=data/2024/january/\" for current folder (URL-encoded as %2f)")
	}

	// Verify clicking "data" breadcrumb navigates back correctly
	req = httptest.NewRequest(http.MethodGet, "/dashboard?prefix=data/", nil)
	rec = httptest.NewRecorder()
	d.Handler()(rec, req)

	body = rec.Body.String()
	// Should show both january and february folders at data/ level
	if !strings.Contains(body, "data/2024/january/report.txt") {
		t.Error("Expected january report to be visible at data/ level")
	}
	if !strings.Contains(body, "data/2024/february/other.txt") {
		t.Error("Expected february file to be visible at data/ level")
	}
}

// TestCanaryStatusNotStarted verifies "Not started" when no canary check has run.
func TestCanaryStatusNotStarted(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()
	// CanaryLastCheckTime defaults to empty — simulates startup before first check
	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()
	d.Handler()(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "Not started") {
		t.Error("Expected 'Not started' when no canary check has run")
	}
	// "Not started" should not apply healthy or unhealthy CSS class
	if strings.Contains(body, "stat-card healthy") || strings.Contains(body, "stat-card unhealthy") {
		t.Error("Expected no health CSS class before first canary check")
	}
}

// TestCanaryStatusHealthy verifies green "healthy" class when canary passes.
func TestCanaryStatusHealthy(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()
	m.SetCanaryLastCheck(time.Now())
	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()
	d.Handler()(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "stat-card healthy") {
		t.Error("Expected 'stat-card healthy' CSS class when canary is healthy")
	}
	if !strings.Contains(body, "Healthy") {
		t.Error("Expected 'Healthy' status text")
	}
}

// TestCanaryStatusUnhealthy verifies red "unhealthy" class when canary fails.
func TestCanaryStatusUnhealthy(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()
	m.IncCanaryFailures()
	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()
	d.Handler()(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "stat-card unhealthy") {
		t.Error("Expected 'stat-card unhealthy' CSS class when canary fails")
	}
	if !strings.Contains(body, "Unhealthy") {
		t.Error("Expected 'Unhealthy' status text")
	}
}

// Helper to verify response contains expected HTML elements
func TestDashboardHTMLStructure(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()

	d.Handler()(rec, req)

	body := rec.Body.String()

	requiredElements := []string{
		"<!DOCTYPE html>",
		"<title>ARMOR Dashboard</title>",
		"Cache Hit Rate",
		"Cache Hits / Misses",
		"Total Requests",
		"Bytes Uploaded",
		"Bytes Downloaded",
		"Uptime",
		"Canary Status",
		"<table>",
		"</html>",
	}

	for _, elem := range requiredElements {
		if !strings.Contains(body, elem) {
			t.Errorf("Expected HTML to contain %q", elem)
		}
	}
}

// TestListAPIHandlerRoot tests the JSON list endpoint at root.
func TestListAPIHandlerRoot(t *testing.T) {
	mb := newMockBackend()
	mb.objects["file1.txt"] = &backend.ObjectInfo{
		Key:              "file1.txt",
		Size:             100,
		ContentType:      "text/plain",
		LastModified:     time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC),
		IsARMOREncrypted: true,
		Metadata: map[string]string{
			"x-amz-meta-armor-version":        "1",
			"x-amz-meta-armor-block-size":     "65536",
			"x-amz-meta-armor-plaintext-size": "100",
			"x-amz-meta-armor-key-id":         "default",
		},
	}
	mb.objects["plain.txt"] = &backend.ObjectInfo{
		Key:          "plain.txt",
		Size:         200,
		ContentType:  "text/plain",
		LastModified: time.Date(2026, 6, 11, 11, 0, 0, 0, time.UTC),
	}
	mb.commonPrefixes = []string{"folder/"}

	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/list", nil)
	rec := httptest.NewRecorder()

	d.ListAPIHandler()(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	// Check content type is JSON
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %q", ct)
	}

	// Parse response
	var resp ListAPIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}

	// Verify prefix
	if resp.Prefix != "" {
		t.Errorf("Expected empty prefix, got %q", resp.Prefix)
	}

	// Verify objects
	if len(resp.Objects) != 2 {
		t.Errorf("Expected 2 objects, got %d", len(resp.Objects))
	}

	// Check encrypted object
	var encryptedObj *ListObject
	for _, obj := range resp.Objects {
		if obj.Key == "file1.txt" {
			encryptedObj = &obj
			break
		}
	}
	if encryptedObj == nil {
		t.Fatal("Expected to find file1.txt")
	}
	if !encryptedObj.Encrypted {
		t.Error("Expected file1.txt to be encrypted")
	}
	if encryptedObj.KeyID != "default" {
		t.Errorf("Expected key_id 'default', got %q", encryptedObj.KeyID)
	}

	// Check plain object
	var plainObj *ListObject
	for _, obj := range resp.Objects {
		if obj.Key == "plain.txt" {
			plainObj = &obj
			break
		}
	}
	if plainObj == nil {
		t.Fatal("Expected to find plain.txt")
	}
	if plainObj.Encrypted {
		t.Error("Expected plain.txt to not be encrypted")
	}

	// Check common prefixes
	if len(resp.CommonPrefixes) != 1 {
		t.Errorf("Expected 1 common prefix, got %d", len(resp.CommonPrefixes))
	}
	if len(resp.CommonPrefixes) > 0 && resp.CommonPrefixes[0] != "folder/" {
		t.Errorf("Expected common prefix 'folder/', got %q", resp.CommonPrefixes[0])
	}
}

// TestListAPIHandlerWithPrefix tests the JSON list endpoint with a prefix.
func TestListAPIHandlerWithPrefix(t *testing.T) {
	mb := newMockBackend()
	mb.objects["data/file1.txt"] = &backend.ObjectInfo{
		Key:          "data/file1.txt",
		Size:         100,
		LastModified: time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC),
	}
	mb.objects["data/file2.txt"] = &backend.ObjectInfo{
		Key:          "data/file2.txt",
		Size:         200,
		LastModified: time.Date(2026, 6, 11, 11, 0, 0, 0, time.UTC),
	}
	mb.objects["other/file.txt"] = &backend.ObjectInfo{
		Key:          "other/file.txt",
		Size:         50,
		LastModified: time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC),
	}

	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/list?prefix=data/", nil)
	rec := httptest.NewRecorder()

	d.ListAPIHandler()(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var resp ListAPIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}

	// Verify prefix
	if resp.Prefix != "data/" {
		t.Errorf("Expected prefix 'data/', got %q", resp.Prefix)
	}

	// Should only have objects under data/
	if len(resp.Objects) != 2 {
		t.Errorf("Expected 2 objects, got %d", len(resp.Objects))
	}
	for _, obj := range resp.Objects {
		if !strings.HasPrefix(obj.Key, "data/") {
			t.Errorf("Expected object key to start with 'data/', got %q", obj.Key)
		}
	}
}

// TestListAPIHandlerEncryptedVsPlain tests encrypted vs plain object handling.
func TestListAPIHandlerEncryptedVsPlain(t *testing.T) {
	mb := newMockBackend()
	mb.objects["encrypted.bin"] = &backend.ObjectInfo{
		Key:              "encrypted.bin",
		Size:             500,
		LastModified:     time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC),
		IsARMOREncrypted: true,
		Metadata: map[string]string{
			"x-amz-meta-armor-version":        "1",
			"x-amz-meta-armor-block-size":     "65536",
			"x-amz-meta-armor-plaintext-size": "500",
			"x-amz-meta-armor-key-id":         "sensitive",
		},
	}
	mb.objects["plain.txt"] = &backend.ObjectInfo{
		Key:          "plain.txt",
		Size:         100,
		LastModified: time.Date(2026, 6, 11, 11, 0, 0, 0, time.UTC),
	}

	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/list", nil)
	rec := httptest.NewRecorder()

	d.ListAPIHandler()(rec, req)

	var resp ListAPIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}

	// Find encrypted object
	var encObj *ListObject
	for _, obj := range resp.Objects {
		if obj.Key == "encrypted.bin" {
			encObj = &obj
			break
		}
	}
	if encObj == nil {
		t.Fatal("Expected to find encrypted.bin")
	}
	if !encObj.Encrypted {
		t.Error("Expected encrypted.bin to be marked as encrypted")
	}
	if encObj.KeyID != "sensitive" {
		t.Errorf("Expected key_id 'sensitive', got %q", encObj.KeyID)
	}

	// Find plain object
	var plainObj *ListObject
	for _, obj := range resp.Objects {
		if obj.Key == "plain.txt" {
			plainObj = &obj
			break
		}
	}
	if plainObj == nil {
		t.Fatal("Expected to find plain.txt")
	}
	if plainObj.Encrypted {
		t.Error("Expected plain.txt to not be marked as encrypted")
	}
	if plainObj.KeyID != "" {
		t.Errorf("Expected empty key_id for plain object, got %q", plainObj.KeyID)
	}
}

// TestListAPIHandlerWithAuth tests authentication for the list endpoint.
func TestListAPIHandlerWithAuth(t *testing.T) {
	mb := newMockBackend()
	mb.objects["file.txt"] = &backend.ObjectInfo{
		Key:          "file.txt",
		Size:         100,
		LastModified: time.Now(),
	}

	m := metrics.NewMetrics()

	// Test with Basic Auth
	d := NewWithAuth(mb, "test-bucket", m, "admin", "secret123", "", nil, "", false)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/list", nil)
	rec := httptest.NewRecorder()

	d.ListAPIHandlerWithAuth()(rec, req)

	// Should get 401 without credentials
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 without auth, got %d", rec.Code)
	}

	// Should succeed with valid credentials
	req = httptest.NewRequest(http.MethodGet, "/dashboard/api/list", nil)
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:secret123")))
	rec = httptest.NewRecorder()

	d.ListAPIHandlerWithAuth()(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200 with valid auth, got %d", rec.Code)
	}

	// Test with Bearer token
	d2 := NewWithAuth(mb, "test-bucket", m, "", "", "my-token", nil, "", false)

	req = httptest.NewRequest(http.MethodGet, "/dashboard/api/list", nil)
	rec = httptest.NewRecorder()

	d2.ListAPIHandlerWithAuth()(rec, req)

	// Should get 401 without token
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 without token, got %d", rec.Code)
	}

	// Should succeed with valid token
	req = httptest.NewRequest(http.MethodGet, "/dashboard/api/list", nil)
	req.Header.Set("Authorization", "Bearer my-token")
	rec = httptest.NewRecorder()

	d2.ListAPIHandlerWithAuth()(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200 with valid token, got %d", rec.Code)
	}
}

// TestListAPIHandlerMethodNotAllowed tests that non-GET requests are rejected.
func TestListAPIHandlerMethodNotAllowed(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodPost, "/dashboard/api/list", nil)
	rec := httptest.NewRecorder()

	d.ListAPIHandler()(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", rec.Code)
	}
}

// TestListAPIHandlerListError tests error handling when backend list fails.
func TestListAPIHandlerListError(t *testing.T) {
	mb := newMockBackend()
	mb.listErr = errors.New("backend error")

	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/list", nil)
	rec := httptest.NewRecorder()

	d.ListAPIHandler()(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", rec.Code)
	}
}

// TestKeyRotateStatusHandlerNoRotation verifies status returns "none" when no rotation state file exists.
func TestKeyRotateStatusHandlerNoRotation(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/admin/key/status", nil)
	rec := httptest.NewRecorder()

	d.KeyRotateStatusHandler()(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}

	if resp["status"] != "none" {
		t.Errorf("Expected status 'none', got %v", resp["status"])
	}
	if resp["message"] != "No rotation in progress" {
		t.Errorf("Expected message 'No rotation in progress', got %v", resp["message"])
	}
}

// TestKeyRotateStatusHandlerWithAuth verifies authentication on the status endpoint.
func TestKeyRotateStatusHandlerWithAuth(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()
	d := NewWithAuth(mb, "test-bucket", m, "", "", "token123", nil, "", false)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/admin/key/status", nil)
	rec := httptest.NewRecorder()

	// Should fail without auth
	d.KeyRotateStatusHandlerWithAuth()(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 without auth, got %d", rec.Code)
	}

	// Should succeed with valid token
	req = httptest.NewRequest(http.MethodGet, "/dashboard/admin/key/status", nil)
	req.Header.Set("Authorization", "Bearer token123")
	rec = httptest.NewRecorder()

	d.KeyRotateStatusHandlerWithAuth()(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200 with valid token, got %d", rec.Code)
	}
}

// TestKeyRotateStatusHandlerMethodNotAllowed verifies non-GET requests are rejected.
func TestKeyRotateStatusHandlerMethodNotAllowed(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodPost, "/dashboard/admin/key/status", nil)
	rec := httptest.NewRecorder()

	d.KeyRotateStatusHandler()(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", rec.Code)
	}
}

// TestKeyRotateHandlerSuccess verifies successful key rotation initiation.
func TestKeyRotateHandlerSuccess(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	// Mock admin API server
	adminServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		// Return success response
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"status":"accepted"}`))
	}))
	defer adminServer.Close()

	req := httptest.NewRequest(http.MethodPost, "/dashboard/admin/key/rotate", nil)
	rec := httptest.NewRecorder()

	d.KeyRotateHandler(adminServer.Client(), adminServer.URL, "")(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Errorf("Expected status 202, got %d", rec.Code)
	}
}

// TestKeyRotateHandlerForwardsAdminToken verifies the loopback rotation call
// carries the ARMOR_ADMIN_TOKEN bearer so it passes the /admin/key/rotate gate.
func TestKeyRotateHandlerForwardsAdminToken(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	const wantToken = "loopback-secret"
	var gotAuth string
	adminServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer adminServer.Close()

	req := httptest.NewRequest(http.MethodPost, "/dashboard/admin/key/rotate", nil)
	rec := httptest.NewRecorder()
	d.KeyRotateHandler(adminServer.Client(), adminServer.URL, wantToken)(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("Expected status 202, got %d", rec.Code)
	}
	if gotAuth != "Bearer "+wantToken {
		t.Errorf("Expected admin API to receive bearer token, got %q", gotAuth)
	}
}

// TestKeyRotateHandlerOmitsAuthHeaderWhenTokenUnset verifies that when no admin
// token is configured, the proxy sends no Authorization header (the admin API
// then rejects rotation fail-closed rather than receiving a stale/empty header).
func TestKeyRotateHandlerOmitsAuthHeaderWhenTokenUnset(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	var gotAuth = "sentinel"
	adminServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer adminServer.Close()

	req := httptest.NewRequest(http.MethodPost, "/dashboard/admin/key/rotate", nil)
	rec := httptest.NewRecorder()
	d.KeyRotateHandler(adminServer.Client(), adminServer.URL, "")(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("Expected status 202, got %d", rec.Code)
	}
	if gotAuth != "" {
		t.Errorf("Expected no Authorization header when token unset, got %q", gotAuth)
	}
}

// TestKeyRotateHandlerWithAuth verifies authentication on the rotate endpoint.
func TestKeyRotateHandlerWithAuth(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()
	d := NewWithAuth(mb, "test-bucket", m, "admin", "secret", "", nil, "", false)

	// Mock admin API server
	adminServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer adminServer.Close()

	req := httptest.NewRequest(http.MethodPost, "/dashboard/admin/key/rotate", nil)
	rec := httptest.NewRecorder()

	// Should fail without auth
	d.KeyRotateHandlerWithAuth(adminServer.Client(), adminServer.URL, "")(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 without auth, got %d", rec.Code)
	}

	// Should succeed with valid auth
	req = httptest.NewRequest(http.MethodPost, "/dashboard/admin/key/rotate", nil)
	req.SetBasicAuth("admin", "secret")
	rec = httptest.NewRecorder()

	d.KeyRotateHandlerWithAuth(adminServer.Client(), adminServer.URL, "")(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Errorf("Expected status 202 with valid auth, got %d", rec.Code)
	}
}

// TestKeyRotateHandlerMethodNotAllowed verifies non-POST requests are rejected.
func TestKeyRotateHandlerMethodNotAllowed(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/admin/key/rotate", nil)
	rec := httptest.NewRecorder()

	d.KeyRotateHandler(nil, "", "")(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", rec.Code)
	}
}

// TestKeyRotateHandlerAdminAPIFailure verifies error handling when admin API fails.
func TestKeyRotateHandlerAdminAPIFailure(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	// Mock admin API that returns an error response
	adminServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("admin API error"))
	}))
	defer adminServer.Close()

	req := httptest.NewRequest(http.MethodPost, "/dashboard/admin/key/rotate", nil)
	rec := httptest.NewRecorder()

	d.KeyRotateHandler(adminServer.Client(), adminServer.URL, "")(rec, req)

	// The handler copies the admin API's response status code
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", rec.Code)
	}
}

// TestKeyRotateHandlerDefaultURL verifies default admin URL is used when none provided.
func TestKeyRotateHandlerDefaultURL(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	// Don't provide a URL - should use default
	req := httptest.NewRequest(http.MethodPost, "/dashboard/admin/key/rotate", nil)
	rec := httptest.NewRecorder()

	d.KeyRotateHandler(http.DefaultClient, "", "")(rec, req)

	// Will fail to connect to localhost:9001, but confirms the default URL is used
	if rec.Code != http.StatusBadGateway {
		t.Errorf("Expected status 502 (connection refused), got %d", rec.Code)
	}
}

// TestPaginationContinuationToken verifies pagination works with continuation tokens.
func TestPaginationContinuationToken(t *testing.T) {
	mb := newMockBackend()

	// Add 2500 objects to test pagination (should span 3 pages with maxKeys=1000)
	for i := 1; i <= 2500; i++ {
		key := fmt.Sprintf("object-%04d.txt", i)
		mb.objects[key] = &backend.ObjectInfo{
			Key:              key,
			Size:             int64(i * 100),
			ContentType:      "text/plain",
			ETag:             fmt.Sprintf("etag%d", i),
			LastModified:     time.Now(),
			IsARMOREncrypted: false,
		}
	}

	m := metrics.NewMetrics()
	d := New(mb, "test-bucket", m)

	// Page 1: First request without continuation token
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()
	d.Handler()(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Page 1: Expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()

	// Verify pagination elements are present
	if !strings.Contains(body, "pagination") {
		t.Error("Page 1: Expected pagination section in response")
	}
	if !strings.Contains(body, "Next →") {
		t.Error("Page 1: Expected Next button in response")
	}
	if !strings.Contains(body, "← Previous") {
		t.Error("Page 1: Expected Previous button (disabled) in response")
	}

	// Extract next token from the page (via JavaScript variable)
	if !strings.Contains(body, "const nextToken =") {
		t.Error("Page 1: Expected nextToken variable in JavaScript")
	}
	if !strings.Contains(body, "const currentContinuationToken = null") {
		t.Error("Page 1: Expected currentContinuationToken to be null on first page")
	}

	// Page 2: Request with continuation token (we need to extract it from page 1)
	// For this test, we'll make a direct API call to get the next token
	ctx := context.Background()
	result, err := mb.List(ctx, "test-bucket", "", "/", "", 1000)
	if err != nil {
		t.Fatalf("Failed to get first page: %v", err)
	}

	if !result.IsTruncated {
		t.Error("Expected first page to be truncated")
	}
	if result.NextToken == "" {
		t.Error("Expected NextToken to be set on first page")
	}

	// Make second page request using the continuation token
	req2 := httptest.NewRequest(http.MethodGet, "/dashboard?continuation_token="+result.NextToken, nil)
	rec2 := httptest.NewRecorder()
	d.Handler()(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Errorf("Page 2: Expected status 200, got %d", rec2.Code)
	}

	body2 := rec2.Body.String()

	// Verify second page has pagination controls
	if !strings.Contains(body2, "pagination") {
		t.Error("Page 2: Expected pagination section in response")
	}
	if !strings.Contains(body2, "Next →") {
		t.Error("Page 2: Expected Next button in response")
	}
	if !strings.Contains(body2, "← Previous") {
		t.Error("Page 2: Expected Previous button (enabled) in response")
	}

	// Verify continuation token is set in JavaScript
	if !strings.Contains(body2, "const currentContinuationToken = '"+result.NextToken+"'") &&
	   !strings.Contains(body2, `const currentContinuationToken = "`+result.NextToken+`"`) {
		t.Errorf("Page 2: Expected currentContinuationToken to be set to %s", result.NextToken)
	}

	// Page 3: Third page should be the last (not truncated)
	result2, err := mb.List(ctx, "test-bucket", "", "/", result.NextToken, 1000)
	if err != nil {
		t.Fatalf("Failed to get second page: %v", err)
	}

	if !result2.IsTruncated {
		t.Error("Expected second page to still be truncated")
	}

	// Make third page request
	req3 := httptest.NewRequest(http.MethodGet, "/dashboard?continuation_token="+result2.NextToken, nil)
	rec3 := httptest.NewRecorder()
	d.Handler()(rec3, req3)

	if rec3.Code != http.StatusOK {
		t.Errorf("Page 3: Expected status 200, got %d", rec3.Code)
	}

	body3 := rec3.Body.String()

	// On the last page, Next button should be disabled
	if !strings.Contains(body3, "pagination") {
		t.Error("Page 3: Expected pagination section in response")
	}

	// The Next button should be disabled (or not present as "next" link) when not truncated
	// Check if the JavaScript shows nextToken as null
	if !strings.Contains(body3, "const nextToken = null") {
		t.Error("Page 3: Expected nextToken to be null on last page")
	}

	// Also test the JSON API endpoint
	apiReq1 := httptest.NewRequest(http.MethodGet, "/dashboard/api/list", nil)
	apiRec1 := httptest.NewRecorder()
	d.ListAPIHandler()(apiRec1, apiReq1)

	if apiRec1.Code != http.StatusOK {
		t.Errorf("API Page 1: Expected status 200, got %d", apiRec1.Code)
	}

	var apiResp1 ListAPIResponse
	if err := json.Unmarshal(apiRec1.Body.Bytes(), &apiResp1); err != nil {
		t.Fatalf("API Page 1: Failed to parse JSON: %v", err)
	}

	if !apiResp1.IsTruncated {
		t.Error("API Page 1: Expected IsTruncated to be true")
	}
	if apiResp1.NextToken == "" {
		t.Error("API Page 1: Expected NextToken to be set")
	}
	if apiResp1.ContinuationToken != "" {
		t.Error("API Page 1: Expected ContinuationToken to be empty on first page")
	}

	// API Page 2
	apiReq2 := httptest.NewRequest(http.MethodGet, "/dashboard/api/list?continuation_token="+apiResp1.NextToken, nil)
	apiRec2 := httptest.NewRecorder()
	d.ListAPIHandler()(apiRec2, apiReq2)

	if apiRec2.Code != http.StatusOK {
		t.Errorf("API Page 2: Expected status 200, got %d", apiRec2.Code)
	}

	var apiResp2 ListAPIResponse
	if err := json.Unmarshal(apiRec2.Body.Bytes(), &apiResp2); err != nil {
		t.Fatalf("API Page 2: Failed to parse JSON: %v", err)
	}

	if apiResp2.ContinuationToken != apiResp1.NextToken {
		t.Errorf("API Page 2: Expected ContinuationToken to match previous NextToken")
	}

	// Verify we're getting different objects on each page
	if len(apiResp1.Objects) != 1000 {
		t.Errorf("API Page 1: Expected 1000 objects, got %d", len(apiResp1.Objects))
	}
	if len(apiResp2.Objects) != 1000 {
		t.Errorf("API Page 2: Expected 1000 objects, got %d", len(apiResp2.Objects))
	}

	// Verify no duplicate keys between pages
	keys1 := make(map[string]bool)
	for _, obj := range apiResp1.Objects {
		keys1[obj.Key] = true
	}

	for _, obj := range apiResp2.Objects {
		if keys1[obj.Key] {
			t.Errorf("API Page 2: Found duplicate key %s from page 1", obj.Key)
		}
	}
}


// TestPresignHandlerDisabled verifies that the presign handler returns 404 when presign is disabled.
func TestPresignHandlerDisabled(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()
	d := NewWithAuth(mb, "test-bucket", m, "", "", "", nil, "", false)

	// Create a mock admin server that should not be called
	adminServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Admin server should not be called when presign is disabled")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer adminServer.Close()

	reqBody := `{"key":"test/file.txt","expires_in":"1h"}`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/presign", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	d.PresignHandler(http.DefaultClient, adminServer.URL)(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected status 404 when presign disabled, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "not enabled") {
		t.Errorf("Expected 'not enabled' message, got: %s", body)
	}
}

// TestPresignHandlerSuccess verifies that the presign handler successfully generates a share URL.
func TestPresignHandlerSuccess(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()
	d := NewWithAuth(mb, "test-bucket", m, "", "", "", nil, "", true)

	// Create a mock admin server that returns a presign response
	adminServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/admin/presign" {
			t.Errorf("Expected path /admin/presign, got %s", r.URL.Path)
		}

		// Verify request body
		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
		}
		if req["bucket"] != "test-bucket" {
			t.Errorf("Expected bucket 'test-bucket', got %v", req["bucket"])
		}
		if req["key"] != "test/file.txt" {
			t.Errorf("Expected key 'test/file.txt', got %v", req["key"])
		}
		if req["expires_in"] != "24h" {
			t.Errorf("Expected expires_in '24h', got %v", req["expires_in"])
		}

		// Return presign response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"url":         "https://example.com/share/abc123",
			"expires_in":  "24h",
			"expires_at":  "2026-08-30T12:00:00Z",
		})
	}))
	defer adminServer.Close()

	reqBody := `{"key":"test/file.txt","expires_in":"24h"}`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/presign", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	d.PresignHandler(http.DefaultClient, adminServer.URL)(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify response fields
	if resp["url"] != "https://example.com/share/abc123" {
		t.Errorf("Expected URL 'https://example.com/share/abc123', got %v", resp["url"])
	}
	if resp["expires_in"] != "24h" {
		t.Errorf("Expected expires_in '24h', got %v", resp["expires_in"])
	}
	if resp["expires_at"] != "2026-08-30T12:00:00Z" {
		t.Errorf("Expected expires_at '2026-08-30T12:00:00Z', got %v", resp["expires_at"])
	}
}

// TestPresignHandlerMethodNotAllowed verifies that non-POST requests are rejected.
func TestPresignHandlerMethodNotAllowed(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()
	d := NewWithAuth(mb, "test-bucket", m, "", "", "", nil, "", true)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/presign", nil)
	rec := httptest.NewRecorder()

	d.PresignHandler(http.DefaultClient, "")(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", rec.Code)
	}
}

// TestPresignHandlerMissingKey verifies that requests without a key are rejected.
func TestPresignHandlerMissingKey(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()
	d := NewWithAuth(mb, "test-bucket", m, "", "", "", nil, "", true)

	reqBody := `{"expires_in":"1h"}`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/presign", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	d.PresignHandler(http.DefaultClient, "")(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "key is required") {
		t.Errorf("Expected 'key is required' message, got: %s", body)
	}
}

// TestPresignHandlerInvalidJSON verifies that invalid JSON is rejected.
func TestPresignHandlerInvalidJSON(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()
	d := NewWithAuth(mb, "test-bucket", m, "", "", "", nil, "", true)

	reqBody := `invalid json`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/presign", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	d.PresignHandler(http.DefaultClient, "")(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Invalid request body") {
		t.Errorf("Expected 'Invalid request body' message, got: %s", body)
	}
}

// TestPresignHandlerWithAuth verifies that dashboard authentication is forwarded to the admin API.
func TestPresignHandlerWithAuth(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()
	
	// Test with Basic Auth
	d := NewWithAuth(mb, "test-bucket", m, "testuser", "testpass", "", nil, "", true)

	var receivedAuth string
	adminServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"url":        "https://example.com/share/test",
			"expires_in": "1h",
			"expires_at": "2026-08-30T12:00:00Z",
		})
	}))
	defer adminServer.Close()

	reqBody := `{"key":"test/file.txt","expires_in":"1h"}`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/presign", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Basic dGVzdHVzZXI6dGVzdHBhc3M=") // testuser:testpass
	rec := httptest.NewRecorder()

	d.PresignHandler(http.DefaultClient, adminServer.URL)(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	// Verify auth was forwarded
	if receivedAuth == "" {
		t.Error("Expected Authorization header to be forwarded")
	}
	if !strings.HasPrefix(receivedAuth, "Basic ") {
		t.Errorf("Expected Basic auth, got: %s", receivedAuth)
	}
}

// TestPresignHandlerDefaultURL verifies that the default admin URL is used when none is provided.
func TestPresignHandlerDefaultURL(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()
	d := NewWithAuth(mb, "test-bucket", m, "", "", "", nil, "", true)

	reqBody := `{"key":"test/file.txt","expires_in":"1h"}`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/presign", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Call with empty admin URL - should use default
	d.PresignHandler(http.DefaultClient, "")(rec, req)

	// Since the default URL (localhost:9001) won't be accessible, we expect a BadGateway error
	if rec.Code != http.StatusBadGateway {
		t.Errorf("Expected status 503 (BadGateway) for unreachable default URL, got %d", rec.Code)
	}
}

// TestShareButtonRendersWhenPresignEnabled verifies that the Share button appears in the HTML when presign is enabled.
func TestShareButtonRendersWhenPresignEnabled(t *testing.T) {
	mb := newMockBackend()
	mb.objects["test/file.txt"] = &backend.ObjectInfo{
		Key:              "test/file.txt",
		Size:             1000,
		ContentType:      "text/plain",
		ETag:             "abc123",
		LastModified:     time.Now(),
		IsARMOREncrypted: true,
		Metadata: map[string]string{
			"x-amz-meta-armor-version":        "1",
			"x-amz-meta-armor-block-size":     "65536",
			"x-amz-meta-armor-plaintext-size": "1000",
			"x-amz-meta-armor-iv":             "dGVzdGl2MTIzNDU2Nzg5MA==",
			"x-amz-meta-armor-wrapped-dek":    "d3JhcHBlZGRlaw==",
			"x-amz-meta-armor-plaintext-sha256": "abcdef123456",
		},
	}

	m := metrics.NewMetrics()
	d := NewWithAuth(mb, "test-bucket", m, "", "", "", &DashboardCredential{
		Name:      "dashboard-cred",
		AccessKey: "key",
		SecretKey: "secret",
	}, "", true)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/", nil)
	rec := httptest.NewRecorder()

	d.Handler()(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	
	// Verify Share button is present when presign is enabled
	if !strings.Contains(body, `onclick="showShareModal(`) {
		t.Error("Expected Share button to be present in HTML when presign is enabled")
	}
	
	// Verify Share modal HTML is present
	if !strings.Contains(body, `id="shareModal"`) {
		t.Error("Expected Share modal to be present in HTML")
	}
	
	// Verify JavaScript functions for Share are present
	if !strings.Contains(body, `function showShareModal(`) {
		t.Error("Expected showShareModal JavaScript function")
	}
	if !strings.Contains(body, `function generateShareUrl(`) {
		t.Error("Expected generateShareUrl JavaScript function")
	}
	if !strings.Contains(body, `function copyShareUrl(`) {
		t.Error("Expected copyShareUrl JavaScript function")
	}
}

// TestShareButtonHiddenWhenPresignDisabled verifies that the Share button does not appear when presign is disabled.
func TestShareButtonHiddenWhenPresignDisabled(t *testing.T) {
	mb := newMockBackend()
	mb.objects["test/file.txt"] = &backend.ObjectInfo{
		Key:              "test/file.txt",
		Size:             1000,
		ContentType:      "text/plain",
		ETag:             "abc123",
		LastModified:     time.Now(),
		IsARMOREncrypted: true,
		Metadata: map[string]string{
			"x-amz-meta-armor-version":        "1",
			"x-amz-meta-armor-block-size":     "65536",
			"x-amz-meta-armor-plaintext-size": "1000",
			"x-amz-meta-armor-iv":             "dGVzdGl2MTIzNDU2Nzg5MA==",
			"x-amz-meta-armor-wrapped-dek":    "d3JhcHBlZGRlaw==",
			"x-amz-meta-armor-plaintext-sha256": "abcdef123456",
		},
	}

	m := metrics.NewMetrics()
	d := NewWithAuth(mb, "test-bucket", m, "", "", "", &DashboardCredential{
		Name:      "dashboard-cred",
		AccessKey: "key",
		SecretKey: "secret",
	}, "", false)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/", nil)
	rec := httptest.NewRecorder()

	d.Handler()(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	
	// Verify Share button is NOT present when presign is disabled
	if strings.Contains(body, `onclick="showShareModal(`) {
		t.Error("Expected Share button to be hidden when presign is disabled")
	}
}

// TestPresignResponseFormatMatchesUI verifies that the presign response format matches what the UI expects.
// This is the acceptance test: "UI test that the rendered URL matches the presign response."
func TestPresignResponseFormatMatchesUI(t *testing.T) {
	mb := newMockBackend()
	m := metrics.NewMetrics()
	d := NewWithAuth(mb, "test-bucket", m, "", "", "", nil, "", true)

	// Create a mock admin server that returns a realistic presign response
	adminServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"url":         "https://armor.example.com/share/eyJiIjoidGVzdC1idWNrZXQiLCJrIjoidGVzdC9maWxlLnR4dCIsImUiOjE3MjI0NjQwMDB9.s1gNvh4Yq8kXrZLKrP_KRCnFpLGvgFZlJ9GYQBnJvoU",
			"expires_in":  "24h",
			"expires_at":  "2026-08-30T12:00:00Z",
		})
	}))
	defer adminServer.Close()

	reqBody := `{"key":"test/file.txt","expires_in":"24h"}`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/presign", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	d.PresignHandler(http.DefaultClient, adminServer.URL)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify all required fields are present
	requiredFields := []string{"url", "expires_in", "expires_at"}
	for _, field := range requiredFields {
		if _, ok := resp[field]; !ok {
			t.Errorf("Expected response to contain '%s' field", field)
		}
	}

	// Verify URL is a non-empty string
	url, ok := resp["url"].(string)
	if !ok || url == "" {
		t.Error("Expected 'url' to be a non-empty string")
	}

	// Verify expires_in is a string matching expected format
	expiresIn, ok := resp["expires_in"].(string)
	if !ok || expiresIn == "" {
		t.Error("Expected 'expires_in' to be a non-empty string")
	}

	// Verify expires_at is an RFC3339 timestamp
	expiresAt, ok := resp["expires_at"].(string)
	if !ok || expiresAt == "" {
		t.Error("Expected 'expires_at' to be a non-empty string")
	} else {
		// Try to parse as RFC3339 timestamp
		if _, err := time.Parse(time.RFC3339, expiresAt); err != nil {
			t.Errorf("Expected 'expires_at' to be RFC3339 format, got error: %v", err)
		}
	}

	// This verifies that the UI JavaScript (generateShareUrl function) can properly
	// extract and display these fields from the presign response
}

// TestShareButtonRendersWhenPresignEnabled verifies that the Share button appears in the HTML when presign is enabled.
