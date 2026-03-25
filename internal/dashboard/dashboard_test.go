package dashboard

import (
	"bytes"
	"context"
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
	objects map[string]*backend.ObjectInfo
	listErr error
	headErr error
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
		return nil, nil
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

	return &backend.ListResult{
		Objects: objects,
	}, nil
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
	return nil, nil, nil
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

func (m *mockBackend) ListMultipartUploads(ctx context.Context, bucket string) (*backend.ListMultipartUploadsResult, error) {
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

func TestDashboardHandler(t *testing.T) {
	mb := newMockBackend()
	mb.objects["test/file1.txt"] = &backend.ObjectInfo{
		Key:            "test/file1.txt",
		Size:           100,
		ContentType:    "text/plain",
		ETag:           "abc123",
		LastModified:   time.Now(),
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
			"x-amz-meta-armor-version":        "1",
			"x-amz-meta-armor-block-size":     "65536",
			"x-amz-meta-armor-plaintext-size": "1000",
			"x-amz-meta-armor-key-id":         "default",
			"x-amz-meta-armor-iv":             "dGVzdGl2MTIzNDU2Nzg5MA==",
			"x-amz-meta-armor-wrapped-dek":    "d3JhcHBlZGRlaw==",
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
