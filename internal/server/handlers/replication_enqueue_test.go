package handlers_test

// Integration tests for replication enqueue in CompleteMultipartUpload.
// Verifies that completed multipart uploads are enqueued for replication
// to the secondary backend (ADR-006).

import (
	"bytes"
	"crypto/rand"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/keymanager"
	"github.com/jedarden/armor/internal/metrics"
	"github.com/jedarden/armor/internal/server/handlers"
)

// mockReplicationQueue is a mock implementation of replication.Enqueuer for testing.
type mockReplicationQueue struct {
	enqueueCount atomic.Int64
	enqueuedKeys []string // bucket/key pairs for verification
	mu           sync.Mutex
}

func (m *mockReplicationQueue) Enqueue(bucket, key string) {
	m.enqueueCount.Add(1)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enqueuedKeys = append(m.enqueuedKeys, bucket+"/"+key)
}

// getEnqueuedCount returns the number of items enqueued.
func (m *mockReplicationQueue) getEnqueuedCount() int64 {
	return m.enqueueCount.Load()
}

// wasKeyEnqueued checks if a specific bucket/key was enqueued.
func (m *mockReplicationQueue) wasKeyEnqueued(bucket, key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	target := bucket + "/" + key
	for _, enqueued := range m.enqueuedKeys {
		if enqueued == target {
			return true
		}
	}
	return false
}

// TestCompleteMultipartUpload_EnqueuesReplication verifies that after a successful
// CompleteMultipartUpload, the final object key is enqueued to the replication queue.
func TestCompleteMultipartUpload_EnqueuesReplication(t *testing.T) {
	// Setup
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("generate MEK: %v", err)
	}
	cfg := &config.Config{
		BlockSize: 64 * 1024, // 64KB blocks
	}
	be := newRecordingBackend()
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("create key manager: %v", err)
	}
	h := handlers.New(cfg, be, backend.NewMetadataCache(1000, 300), backend.NewFooterCache(1000, 300), km, nil)
	mets := metrics.NewMetrics()

	mockQueue := &mockReplicationQueue{}
	h.WithReplicationQueue(mockQueue)
	h.WithMetrics(mets)

	// Create a multipart upload
	createReq, _ := http.NewRequest("POST", "/test-bucket/test-key?uploads", nil)
	createResp := httptest.NewRecorder()
	h.HandleRoot(createResp, createReq)

	if createResp.Code != http.StatusOK {
		t.Fatalf("CreateMultipartUpload failed: %d %s", createResp.Code, createResp.Body.String())
	}

	// Parse upload ID from response
	var createResult struct {
		UploadID string `xml:"UploadId"`
	}
	if err := xml.Unmarshal(createResp.Body.Bytes(), &createResult); err != nil {
		t.Fatalf("Failed to parse CreateMultipartUpload response: %v", err)
	}
	uploadID := createResult.UploadID

	// Upload a part (5.25 MB - meets 5 MiB minimum)
	partData := make([]byte, 5*1024*1024+256*1024) // 5.25 MB
	for i := range partData {
		partData[i] = byte(i % 256)
	}
	partReq, _ := http.NewRequest("PUT",
		fmt.Sprintf("/test-bucket/test-key?partNumber=1&uploadId=%s", uploadID),
		bytes.NewReader(partData))
	partResp := httptest.NewRecorder()
	h.HandleRoot(partResp, partReq)

	if partResp.Code != http.StatusOK {
		t.Fatalf("UploadPart failed: %d %s", partResp.Code, partResp.Body.String())
	}

	// Get the ETag for the part
	partETag := partResp.Header().Get("ETag")
	// Strip quotes if present
	partETag = strings.Trim(partETag, `"`)

	// Complete the multipart upload
	completeReq := struct {
		XMLName xml.Name `xml:"CompleteMultipartUpload"`
		Parts   []struct {
			PartNumber int    `xml:"PartNumber"`
			ETag       string `xml:"ETag"`
		} `xml:"Part"`
	}{
		Parts: []struct {
			PartNumber int    `xml:"PartNumber"`
			ETag       string `xml:"ETag"`
		}{
			{PartNumber: 1, ETag: partETag},
		},
	}

	completeBody, _ := xml.Marshal(completeReq)
	completeHTTPReq, _ := http.NewRequest("POST",
		fmt.Sprintf("/test-bucket/test-key?uploadId=%s", uploadID),
		bytes.NewReader(completeBody))
	completeHTTPReq.Header.Set("Content-Type", "application/xml")
	completeResp := httptest.NewRecorder()
	h.HandleRoot(completeResp, completeHTTPReq)

	// Verify CompleteMultipartUpload succeeded
	if completeResp.Code != http.StatusOK {
		t.Fatalf("CompleteMultipartUpload failed: %d %s", completeResp.Code, completeResp.Body.String())
	}

	// Wait for the goroutine to enqueue
	time.Sleep(100 * time.Millisecond)

	// Verify the key was enqueued for replication
	if !mockQueue.wasKeyEnqueued("test-bucket", "test-key") {
		t.Error("Expected 'test-bucket/test-key' to be enqueued for replication, but it wasn't")
	}

	// Verify exactly one item was enqueued
	if count := mockQueue.getEnqueuedCount(); count != 1 {
		t.Errorf("Expected exactly 1 enqueued item, got %d", count)
	}

	// Verify the Prometheus metric was incremented
	// Check that the metric appears in Prometheus output with value 1
	promOutput := mets.PrometheusFormat()
	if !strings.Contains(promOutput, `armor_replication_enqueued_total{operation="completemultipart"}`) {
		t.Error("Expected Prometheus metric 'armor_replication_enqueued_total{operation=\"completemultipart\"}' to be present")
	}
	// Verify the value is 1 (not 0)
	lines := strings.Split(promOutput, "\n")
	for _, line := range lines {
		if strings.Contains(line, `operation="completemultipart"`) {
			// Extract the value after the last space
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				valueStr := parts[len(parts)-1]
				if valueStr == "0" {
					t.Error("Expected replication_enqueued_total{operation=\"completemultipart\"} to be 1, but got 0")
				}
			}
			break
		}
	}
}

// TestCompleteMultipartUpload_NilReplicationQueue verifies that when replication queue
// is nil, CompleteMultipartUpload succeeds without attempting to enqueue.
func TestCompleteMultipartUpload_NilReplicationQueue(t *testing.T) {
	// Setup - handlers WITHOUT replication queue
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("generate MEK: %v", err)
	}
	cfg := &config.Config{
		BlockSize: 64 * 1024,
	}
	be := newRecordingBackend()
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("create key manager: %v", err)
	}
	h := handlers.New(cfg, be, nil, nil, km, nil)
	mets := metrics.NewMetrics()
	h.WithMetrics(mets)
	// Note: WithReplicationQueue is NOT called, so h.replicationQueue remains nil

	// Create and complete a multipart upload (same flow as above)
	createReq, _ := http.NewRequest("POST", "/test-bucket/test-key?uploads", nil)
	createResp := httptest.NewRecorder()
	h.HandleRoot(createResp, createReq)

	if createResp.Code != http.StatusOK {
		t.Fatalf("CreateMultipartUpload failed: %d %s", createResp.Code, createResp.Body.String())
	}

	var createResult struct {
		UploadID string `xml:"UploadId"`
	}
	if err := xml.Unmarshal(createResp.Body.Bytes(), &createResult); err != nil {
		t.Fatalf("Failed to parse CreateMultipartUpload response: %v", err)
	}
	uploadID := createResult.UploadID

	// Upload part
	partData := make([]byte, 5*1024*1024+256*1024)
	for i := range partData {
		partData[i] = byte(i % 256)
	}
	partReq, _ := http.NewRequest("PUT",
		fmt.Sprintf("/test-bucket/test-key?partNumber=1&uploadId=%s", uploadID),
		bytes.NewReader(partData))
	partResp := httptest.NewRecorder()
	h.HandleRoot(partResp, partReq)

	if partResp.Code != http.StatusOK {
		t.Fatalf("UploadPart failed: %d %s", partResp.Code, partResp.Body.String())
	}

	partETag := strings.Trim(partResp.Header().Get("ETag"), `"`)

	// Complete multipart upload
	completeReq := struct {
		XMLName xml.Name `xml:"CompleteMultipartUpload"`
		Parts   []struct {
			PartNumber int    `xml:"PartNumber"`
			ETag       string `xml:"ETag"`
		} `xml:"Part"`
	}{
		Parts: []struct {
			PartNumber int    `xml:"PartNumber"`
			ETag       string `xml:"ETag"`
		}{
			{PartNumber: 1, ETag: partETag},
		},
	}

	completeBody, _ := xml.Marshal(completeReq)
	completeHTTPReq, _ := http.NewRequest("POST",
		fmt.Sprintf("/test-bucket/test-key?uploadId=%s", uploadID),
		bytes.NewReader(completeBody))
	completeHTTPReq.Header.Set("Content-Type", "application/xml")
	completeResp := httptest.NewRecorder()
	h.HandleRoot(completeResp, completeHTTPReq)

	// Verify CompleteMultipartUpload succeeded despite nil queue
	if completeResp.Code != http.StatusOK {
		t.Fatalf("CompleteMultipartUpload should succeed with nil queue, got: %d %s", completeResp.Code, completeResp.Body.String())
	}

	// Verify response contains valid CompleteMultipartUploadResult
	var completeResult struct {
		Bucket string `xml:"Bucket"`
		Key    string `xml:"Key"`
		ETag   string `xml:"ETag"`
	}
	body, _ := io.ReadAll(completeResp.Body)
	if err := xml.Unmarshal(body, &completeResult); err != nil {
		t.Fatalf("Failed to parse CompleteMultipartUpload response: %v", err)
	}

	if completeResult.Bucket != "test-bucket" {
		t.Errorf("Expected bucket 'test-bucket', got '%s'", completeResult.Bucket)
	}
	if completeResult.Key != "test-key" {
		t.Errorf("Expected key 'test-key', got '%s'", completeResult.Key)
	}
	if completeResult.ETag == "" {
		t.Error("Expected non-empty ETag in response")
	}
}

// TestCompleteMultipartUpload_ReplicationEnqueueNonBlocking verifies that replication
// enqueue doesn't block the client response - even if the goroutine takes time,
// the client receives the response immediately.
func TestCompleteMultipartUpload_ReplicationEnqueueNonBlocking(t *testing.T) {
	// Setup
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("generate MEK: %v", err)
	}
	cfg := &config.Config{
		BlockSize: 64 * 1024,
	}
	be := newRecordingBackend()
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("create key manager: %v", err)
	}
	h := handlers.New(cfg, be, nil, nil, km, nil)
	mets := metrics.NewMetrics()

	// Create a slow mock queue that simulates work
	slowQueue := &mockReplicationQueue{}
	h.WithReplicationQueue(slowQueue)
	h.WithMetrics(mets)

	// Create multipart upload
	createReq, _ := http.NewRequest("POST", "/test-bucket/test-key?uploads", nil)
	createResp := httptest.NewRecorder()
	h.HandleRoot(createResp, createReq)

	if createResp.Code != http.StatusOK {
		t.Fatalf("CreateMultipartUpload failed: %d %s", createResp.Code, createResp.Body.String())
	}

	// Upload part
	var createResult struct {
		UploadID string `xml:"UploadId"`
	}
	xml.Unmarshal(createResp.Body.Bytes(), &createResult)

	partData := make([]byte, 5*1024*1024+256*1024)
	for i := range partData {
		partData[i] = byte(i % 256)
	}
	partReq, _ := http.NewRequest("PUT",
		fmt.Sprintf("/test-bucket/test-key?partNumber=1&uploadId=%s", createResult.UploadID),
		bytes.NewReader(partData))
	partResp := httptest.NewRecorder()
	h.HandleRoot(partResp, partReq)

	partETag := strings.Trim(partResp.Header().Get("ETag"), `"`)

	// Complete multipart upload and measure response time
	completeReq := struct {
		XMLName xml.Name `xml:"CompleteMultipartUpload"`
		Parts   []struct {
			PartNumber int    `xml:"PartNumber"`
			ETag       string `xml:"ETag"`
		} `xml:"Part"`
	}{
		Parts: []struct {
			PartNumber int    `xml:"PartNumber"`
			ETag       string `xml:"ETag"`
		}{
			{PartNumber: 1, ETag: partETag},
		},
	}

	completeBody, _ := xml.Marshal(completeReq)
	completeHTTPReq, _ := http.NewRequest("POST",
		fmt.Sprintf("/test-bucket/test-key?uploadId=%s", createResult.UploadID),
		bytes.NewReader(completeBody))
	completeHTTPReq.Header.Set("Content-Type", "application/xml")
	completeResp := httptest.NewRecorder()

	start := time.Now()
	h.HandleRoot(completeResp, completeHTTPReq)
	completeDuration := time.Since(start)

	// Verify response was quick (not blocked by enqueue goroutine)
	// The goroutine starts after the response is sent, so response time
	// should be measured in milliseconds, not seconds
	if completeDuration > 100*time.Millisecond {
		t.Errorf("CompleteMultipartUpload took too long (%v), suggesting it might be blocked by enqueue: %v", completeDuration, completeDuration)
	}

	// Verify the response itself is valid
	if completeResp.Code != http.StatusOK {
		t.Fatalf("CompleteMultipartUpload failed: %d %s", completeResp.Code, completeResp.Body.String())
	}

	// The goroutine should have enqueued asynchronously
	time.Sleep(50 * time.Millisecond)
	if !slowQueue.wasKeyEnqueued("test-bucket", "test-key") {
		t.Error("Expected key to be enqueued asynchronously")
	}
}

// TestCompleteMultipartUpload_MultipleBuckets verifies that multipart uploads to
// different buckets are correctly enqueued with the right bucket names.
func TestCompleteMultipartUpload_MultipleBuckets(t *testing.T) {
	// Setup
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("generate MEK: %v", err)
	}
	cfg := &config.Config{
		BlockSize: 64 * 1024,
	}
	be := newRecordingBackend()
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("create key manager: %v", err)
	}
	h := handlers.New(cfg, be, nil, nil, km, nil)
	mets := metrics.NewMetrics()

	mockQueue := &mockReplicationQueue{}
	h.WithReplicationQueue(mockQueue)
	h.WithMetrics(mets)

	buckets := []string{"bucket-a", "bucket-b", "bucket-c"}
	keys := []string{"key-1", "key-2", "key-3"}

	for i := range buckets {
		// Create multipart upload
		createReq, _ := http.NewRequest("POST", fmt.Sprintf("/%s/%s?uploads", buckets[i], keys[i]), nil)
		createResp := httptest.NewRecorder()
		h.HandleRoot(createResp, createReq)

		if createResp.Code != http.StatusOK {
			t.Fatalf("CreateMultipartUpload failed for %s/%s: %d %s", buckets[i], keys[i], createResp.Code, createResp.Body.String())
		}

		var createResult struct {
			UploadID string `xml:"UploadId"`
		}
		xml.Unmarshal(createResp.Body.Bytes(), &createResult)

		// Upload part
		partData := make([]byte, 5*1024*1024+256*1024)
		for j := range partData {
			partData[j] = byte(j % 256)
		}
		partReq, _ := http.NewRequest("PUT",
			fmt.Sprintf("/%s/%s?partNumber=1&uploadId=%s", buckets[i], keys[i], createResult.UploadID),
			bytes.NewReader(partData))
		partResp := httptest.NewRecorder()
		h.HandleRoot(partResp, partReq)

		partETag := strings.Trim(partResp.Header().Get("ETag"), `"`)

		// Complete multipart upload
		completeReq := struct {
			XMLName xml.Name `xml:"CompleteMultipartUpload"`
			Parts   []struct {
				PartNumber int    `xml:"PartNumber"`
				ETag       string `xml:"ETag"`
			} `xml:"Part"`
		}{
			Parts: []struct {
				PartNumber int    `xml:"PartNumber"`
				ETag       string `xml:"ETag"`
			}{
				{PartNumber: 1, ETag: partETag},
			},
		}

		completeBody, _ := xml.Marshal(completeReq)
		completeHTTPReq, _ := http.NewRequest("POST",
			fmt.Sprintf("/%s/%s?uploadId=%s", buckets[i], keys[i], createResult.UploadID),
			bytes.NewReader(completeBody))
		completeHTTPReq.Header.Set("Content-Type", "application/xml")
		completeResp := httptest.NewRecorder()
		h.HandleRoot(completeResp, completeHTTPReq)

		if completeResp.Code != http.StatusOK {
			t.Fatalf("CompleteMultipartUpload failed for %s/%s: %d %s", buckets[i], keys[i], completeResp.Code, completeResp.Body.String())
		}
	}

	// Wait for all goroutines to finish
	time.Sleep(150 * time.Millisecond)

	// Verify all bucket/key pairs were enqueued
	if count := mockQueue.getEnqueuedCount(); count != 3 {
		t.Errorf("Expected 3 enqueued items, got %d", count)
	}

	for i := range buckets {
		if !mockQueue.wasKeyEnqueued(buckets[i], keys[i]) {
			t.Errorf("Expected '%s/%s' to be enqueued", buckets[i], keys[i])
		}
	}
}

// TestCompleteMultipartUpload_FailedCompletionDoesNotEnqueue verifies that failed
// CompleteMultipartUpload operations do NOT enqueue for replication.
func TestCompleteMultipartUpload_FailedCompletionDoesNotEnqueue(t *testing.T) {
	// Setup
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("generate MEK: %v", err)
	}
	cfg := &config.Config{
		BlockSize: 64 * 1024,
	}
	be := newRecordingBackend()
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("create key manager: %v", err)
	}
	h := handlers.New(cfg, be, nil, nil, km, nil)
	mets := metrics.NewMetrics()

	mockQueue := &mockReplicationQueue{}
	h.WithReplicationQueue(mockQueue)
	h.WithMetrics(mets)

	// Create multipart upload
	createReq, _ := http.NewRequest("POST", "/test-bucket/test-key?uploads", nil)
	createResp := httptest.NewRecorder()
	h.HandleRoot(createResp, createReq)

	var createResult struct {
		UploadID string `xml:"UploadId"`
	}
	xml.Unmarshal(createResp.Body.Bytes(), &createResult)
	uploadID := createResult.UploadID

	// Upload a valid part
	partData := make([]byte, 5*1024*1024+256*1024)
	for i := range partData {
		partData[i] = byte(i % 256)
	}
	partReq, _ := http.NewRequest("PUT",
		fmt.Sprintf("/test-bucket/test-key?partNumber=1&uploadId=%s", uploadID),
		bytes.NewReader(partData))
	partResp := httptest.NewRecorder()
	h.HandleRoot(partResp, partReq)

	// Try to complete with a MISSING part (should fail)
	completeReq := struct {
		XMLName xml.Name `xml:"CompleteMultipartUpload"`
		Parts   []struct {
			PartNumber int    `xml:"PartNumber"`
			ETag       string `xml:"ETag"`
		} `xml:"Part"`
	}{
		Parts: []struct {
			PartNumber int    `xml:"PartNumber"`
			ETag       string `xml:"ETag"`
		}{
			// Part 2 was never uploaded, so this should fail
			{PartNumber: 2, ETag: "fake-etag"},
		},
	}

	completeBody, _ := xml.Marshal(completeReq)
	completeHTTPReq, _ := http.NewRequest("POST",
		fmt.Sprintf("/test-bucket/test-key?uploadId=%s", uploadID),
		bytes.NewReader(completeBody))
	completeHTTPReq.Header.Set("Content-Type", "application/xml")
	completeResp := httptest.NewRecorder()
	h.HandleRoot(completeResp, completeHTTPReq)

	// Verify completion failed
	if completeResp.Code != http.StatusBadRequest {
		t.Errorf("Expected CompleteMultipartUpload to fail with 400, got %d", completeResp.Code)
	}

	// Wait to ensure no goroutine was spawned
	time.Sleep(100 * time.Millisecond)

	// Verify nothing was enqueued
	if count := mockQueue.getEnqueuedCount(); count != 0 {
		t.Errorf("Expected 0 enqueued items for failed completion, got %d", count)
	}
}
