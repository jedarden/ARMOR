package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewMetrics(t *testing.T) {
	m := NewMetrics()
	if m == nil {
		t.Fatal("NewMetrics returned nil")
	}

	// Verify all metric counters are initialized
	if m.RequestsTotal == nil {
		t.Error("RequestsTotal not initialized")
	}
	if m.BytesUploaded == nil {
		t.Error("BytesUploaded not initialized")
	}
	if m.CacheHitsTotal == nil {
		t.Error("CacheHitsTotal not initialized")
	}
}

func TestMetricsIncRequestsTotal(t *testing.T) {
	m := NewMetrics()

	// Initial value should be 0
	if m.RequestsTotal.String() != "0" {
		t.Errorf("expected initial value 0, got %s", m.RequestsTotal.String())
	}

	m.IncRequestsTotal("GET", 200)
	m.IncRequestsTotal("PUT", 201)
	m.IncRequestsTotal("GET", 200)

	if m.RequestsTotal.String() != "3" {
		t.Errorf("expected 3 requests, got %s", m.RequestsTotal.String())
	}
}

func TestMetricsInFlightRequests(t *testing.T) {
	m := NewMetrics()

	m.IncRequestsInFlight()
	if m.RequestsInFlight.String() != "1" {
		t.Errorf("expected 1 in-flight, got %s", m.RequestsInFlight.String())
	}

	m.IncRequestsInFlight()
	m.IncRequestsInFlight()
	if m.RequestsInFlight.String() != "3" {
		t.Errorf("expected 3 in-flight, got %s", m.RequestsInFlight.String())
	}

	m.DecRequestsInFlight()
	if m.RequestsInFlight.String() != "2" {
		t.Errorf("expected 2 in-flight, got %s", m.RequestsInFlight.String())
	}
}

func TestMetricsBytesTracking(t *testing.T) {
	m := NewMetrics()

	m.AddBytesUploaded(1024)
	m.AddBytesUploaded(2048)
	if m.BytesUploaded.String() != "3072" {
		t.Errorf("expected 3072 bytes uploaded, got %s", m.BytesUploaded.String())
	}

	m.AddBytesDownloaded(512)
	if m.BytesDownloaded.String() != "512" {
		t.Errorf("expected 512 bytes downloaded, got %s", m.BytesDownloaded.String())
	}

	m.AddBytesFetchedFromB2(4096)
	if m.BytesFetchedFromB2.String() != "4096" {
		t.Errorf("expected 4096 bytes fetched, got %s", m.BytesFetchedFromB2.String())
	}
}

func TestMetricsCacheTracking(t *testing.T) {
	m := NewMetrics()

	m.IncCacheHits()
	m.IncCacheHits()
	m.IncCacheMisses()

	if m.CacheHitsTotal.String() != "2" {
		t.Errorf("expected 2 cache hits, got %s", m.CacheHitsTotal.String())
	}
	if m.CacheMissesTotal.String() != "1" {
		t.Errorf("expected 1 cache miss, got %s", m.CacheMissesTotal.String())
	}
}

func TestMetricsRangeReadTracking(t *testing.T) {
	m := NewMetrics()

	m.IncRangeReads()
	m.AddRangeBytesSaved(10240)

	if m.RangeReadsTotal.String() != "1" {
		t.Errorf("expected 1 range read, got %s", m.RangeReadsTotal.String())
	}
	if m.RangeBytesSavedTotal.String() != "10240" {
		t.Errorf("expected 10240 bytes saved, got %s", m.RangeBytesSavedTotal.String())
	}
}

func TestMetricsCanaryTracking(t *testing.T) {
	m := NewMetrics()

	m.IncCanaryChecks()
	m.IncCanaryChecks()
	m.IncCanaryFailures()

	if m.CanaryChecksTotal.String() != "2" {
		t.Errorf("expected 2 canary checks, got %s", m.CanaryChecksTotal.String())
	}
	if m.CanaryCheckFailures.String() != "1" {
		t.Errorf("expected 1 canary failure, got %s", m.CanaryCheckFailures.String())
	}

	testTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	m.SetCanaryLastCheck(testTime)
	// expvar.String.String() returns JSON-encoded string with quotes
	expectedTime := `"2024-01-15T10:30:00Z"`
	if m.CanaryLastCheckTime.String() != expectedTime {
		t.Errorf("unexpected last check time: got %s, want %s", m.CanaryLastCheckTime.String(), expectedTime)
	}

	m.SetCanaryLastError("test error")
	// expvar.String returns JSON-encoded strings
	if m.CanaryLastCheckError.String() != `"test error"` {
		t.Errorf("expected error '\"test error\"', got %s", m.CanaryLastCheckError.String())
	}
}

func TestMetricsMultipartTracking(t *testing.T) {
	m := NewMetrics()

	m.IncActiveMultipartUploads()
	m.IncActiveMultipartUploads()
	m.IncMultipartPartsUploaded()
	m.DecActiveMultipartUploads()

	if m.ActiveMultipartUploads.String() != "1" {
		t.Errorf("expected 1 active multipart upload, got %s", m.ActiveMultipartUploads.String())
	}
	if m.MultipartPartsUploaded.String() != "1" {
		t.Errorf("expected 1 part uploaded, got %s", m.MultipartPartsUploaded.String())
	}
}

func TestMetricsKeyRotationTracking(t *testing.T) {
	m := NewMetrics()

	m.IncKeyRotations()
	m.AddKeyRotationObjects(100)
	m.IncKeyRotationErrors()
	m.SetKeyRotationStartTime(time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC))

	if m.KeyRotationsTotal.String() != "1" {
		t.Errorf("expected 1 key rotation, got %s", m.KeyRotationsTotal.String())
	}
	if m.KeyRotationObjects.String() != "100" {
		t.Errorf("expected 100 objects rotated, got %s", m.KeyRotationObjects.String())
	}
	if m.KeyRotationErrors.String() != "1" {
		t.Errorf("expected 1 key rotation error, got %s", m.KeyRotationErrors.String())
	}
}

func TestMetricsProvenanceTracking(t *testing.T) {
	m := NewMetrics()

	m.IncProvenanceEntries()
	m.IncProvenanceEntries()
	m.SetProvenanceChainLength(42)

	if m.ProvenanceEntriesTotal.String() != "2" {
		t.Errorf("expected 2 provenance entries, got %s", m.ProvenanceEntriesTotal.String())
	}
	if m.ProvenanceChainLength.String() != "42" {
		t.Errorf("expected chain length 42, got %s", m.ProvenanceChainLength.String())
	}
}

func TestMetricsPrometheusFormat(t *testing.T) {
	m := NewMetrics()

	// Add some data
	m.IncRequestsTotal("GET", 200)
	m.AddBytesUploaded(1024)
	m.IncCacheHits()

	output := m.PrometheusFormat()

	// Check for expected metric names
	expectedMetrics := []string{
		"armor_requests_total",
		"armor_bytes_uploaded_total",
		"armor_metadata_cache_hits_total",
		"armor_uptime_seconds",
	}

	for _, name := range expectedMetrics {
		if !strings.Contains(output, name) {
			t.Errorf("expected metric %q in Prometheus output", name)
		}
	}

	// Check for HELP and TYPE comments
	if !strings.Contains(output, "# HELP") {
		t.Error("expected HELP comments in Prometheus output")
	}
	if !strings.Contains(output, "# TYPE") {
		t.Error("expected TYPE comments in Prometheus output")
	}
}

func TestMetricsHandler(t *testing.T) {
	m := NewMetrics()
	m.IncRequestsTotal("GET", 200)

	handler := m.Handler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") {
		t.Errorf("expected text/plain content type, got %s", contentType)
	}

	if !strings.Contains(rec.Body.String(), "armor_requests_total") {
		t.Error("expected armor_requests_total in response body")
	}
}

func TestRequestTracker(t *testing.T) {
	rt := &RequestTracker{}

	if rt.Count() != 0 {
		t.Errorf("expected initial count 0, got %d", rt.Count())
	}

	rt.Start()
	rt.Start()
	if rt.Count() != 2 {
		t.Errorf("expected count 2, got %d", rt.Count())
	}

	rt.End()
	if rt.Count() != 1 {
		t.Errorf("expected count 1, got %d", rt.Count())
	}

	rt.End()
	if rt.Count() != 0 {
		t.Errorf("expected count 0, got %d", rt.Count())
	}
}

func TestRequestTrackerWait(t *testing.T) {
	rt := &RequestTracker{}

	done := make(chan bool)
	started := make(chan struct{})

	// Start must complete before Wait is called to avoid race with wg.Add
	go func() {
		rt.Start()
		close(started)
		time.Sleep(50 * time.Millisecond)
		rt.End()
	}()

	go func() {
		<-started // Wait for Start to complete before calling Wait
		rt.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(200 * time.Millisecond):
		t.Error("Wait did not complete in time")
	}
}

func TestDefaultMetrics(t *testing.T) {
	if DefaultMetrics == nil {
		t.Fatal("DefaultMetrics is nil")
	}

	// Test that DefaultMetrics can be used
	DefaultMetrics.IncCacheHits()
	if DefaultMetrics.CacheHitsTotal.String() != "1" {
		t.Errorf("expected 1 cache hit, got %s", DefaultMetrics.CacheHitsTotal.String())
	}
}

func TestDefaultRequestTracker(t *testing.T) {
	if DefaultRequestTracker == nil {
		t.Fatal("DefaultRequestTracker is nil")
	}

	// Reset count
	for DefaultRequestTracker.Count() > 0 {
		DefaultRequestTracker.End()
	}

	DefaultRequestTracker.Start()
	if DefaultRequestTracker.Count() != 1 {
		t.Errorf("expected count 1, got %d", DefaultRequestTracker.Count())
	}
	DefaultRequestTracker.End()
}
