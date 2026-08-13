package crypto

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"testing"

	"github.com/klauspost/compress/zstd"
)

// mockTestStorage is a mock implementation of TestObjectStorage for testing
type mockTestStorage struct {
	mu      sync.RWMutex
	objects map[string]struct {
		data     []byte
		size     int64
		metadata map[string]string
	}
	putCalls    []putCall
	deleteCalls []deleteCall
	deleteError map[string]error
	putError    map[string]error
}

type putCall struct {
	bucket string
	key    string
	size   int64
	meta   map[string]string
}

type deleteCall struct {
	bucket string
	key    string
}

func newMockTestStorage() *mockTestStorage {
	return &mockTestStorage{
		objects:     make(map[string]struct{ data []byte; size int64; metadata map[string]string }),
		putCalls:    make([]putCall, 0),
		deleteCalls: make([]deleteCall, 0),
		deleteError: make(map[string]error),
		putError:    make(map[string]error),
	}
}

func (m *mockTestStorage) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err, exists := m.putError[key]; exists {
		return err
	}

	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}

	m.objects[key] = struct{ data []byte; size int64; metadata map[string]string }{
		data:     data,
		size:     size,
		metadata: meta,
	}

	m.putCalls = append(m.putCalls, putCall{
		bucket: bucket,
		key:    key,
		size:   size,
		meta:   meta,
	})

	return nil
}

func (m *mockTestStorage) Delete(ctx context.Context, bucket, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Always record the delete call, regardless of success/failure
	m.deleteCalls = append(m.deleteCalls, deleteCall{
		bucket: bucket,
		key:    key,
	})

	if err, exists := m.deleteError[key]; exists {
		return err
	}

	delete(m.objects, key)
	return nil
}

func (m *mockTestStorage) Get(ctx context.Context, bucket, key string) ([]byte, map[string]string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	obj, exists := m.objects[key]
	if !exists {
		return nil, nil, false
	}
	return obj.data, obj.metadata, true
}

func (m *mockTestStorage) Exists(key string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.objects[key]
	return exists
}

func (m *mockTestStorage) SetDeleteError(key string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleteError[key] = err
}

func (m *mockTestStorage) SetPutError(key string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.putError[key] = err
}

func (m *mockTestStorage) PutCallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.putCalls)
}

func (m *mockTestStorage) DeleteCallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.deleteCalls)
}

func TestNewCompressionTestUtilities(t *testing.T) {
	storage := newMockTestStorage()
	ctu := NewCompressionTestUtilities(storage)

	if ctu == nil {
		t.Fatal("NewCompressionTestUtilities returned nil")
	}

	if ctu.storage != storage {
		t.Error("storage not set correctly")
	}

	if ctu.objects == nil {
		t.Error("objects map not initialized")
	}

	if ctu.cleanupKey != "test-compression-obj-" {
		t.Errorf("cleanupKey not set correctly, got: %s", ctu.cleanupKey)
	}
}

func TestCreateCompressedTestObject(t *testing.T) {
	ctx := context.Background()
	storage := newMockTestStorage()
	ctu := NewCompressionTestUtilities(storage)

	content := []byte("Hello, ARMOR test content!")
	contentType := "application/octet-stream"

	testObj, err := ctu.CreateCompressedTestObject(ctx, "test-bucket", "test-key-1", content, contentType)
	if err != nil {
		t.Fatalf("CreateCompressedTestObject failed: %v", err)
	}

	// Verify test object properties
	if testObj.Key != "test-key-1" {
		t.Errorf("Key not set correctly, got: %s", testObj.Key)
	}
	if testObj.Bucket != "test-bucket" {
		t.Errorf("Bucket not set correctly, got: %s", testObj.Bucket)
	}
	if !testObj.Compressed {
		t.Error("Compressed flag not set correctly")
	}
	if testObj.ContentType != contentType {
		t.Errorf("ContentType not set correctly, got: %s", testObj.ContentType)
	}

	// Verify content sizes
	if testObj.PlaintextSize != int64(len(content)) {
		t.Errorf("PlaintextSize not set correctly, got: %d, want: %d", testObj.PlaintextSize, len(content))
	}
	if testObj.StoredSize == 0 || testObj.StoredSize >= testObj.PlaintextSize {
		t.Logf("Warning: StoredSize %d should be less than PlaintextSize %d for compressible data", testObj.StoredSize, testObj.PlaintextSize)
	}

	// Verify original content is preserved
	if !bytes.Equal(testObj.OriginalContent, content) {
		t.Error("OriginalContent not preserved correctly")
	}

	// Verify stored content is compressed
	if !IsCompressed(testObj.StoredContent) {
		t.Error("StoredContent should be compressed")
	}

	// Verify object was stored
	if !storage.Exists("test-key-1") {
		t.Error("Object was not stored in storage")
	}

	storedData, metadata, exists := storage.Get(ctx, "test-bucket", "test-key-1")
	if !exists {
		t.Fatal("Object not found in storage")
	}

	if !bytes.Equal(storedData, testObj.StoredContent) {
		t.Error("Stored data doesn't match test object's stored content")
	}

	if metadata["content-type"] != contentType {
		t.Errorf("Content-type metadata not set correctly, got: %s", metadata["content-type"])
	}

	if metadata["test-compressed"] != "true" {
		t.Error("test-compressed metadata not set to true")
	}

	// Verify object is tracked
	trackedObj, exists := ctu.GetTestObject("test-key-1")
	if !exists {
		t.Error("Object not tracked")
	}
	if trackedObj != testObj {
		t.Error("Tracked object is not the same instance")
	}
}

func TestCreateUncompressedTestObject(t *testing.T) {
	ctx := context.Background()
	storage := newMockTestStorage()
	ctu := NewCompressionTestUtilities(storage)

	content := []byte("Hello, ARMOR test content!")
	contentType := "text/plain"

	testObj, err := ctu.CreateUncompressedTestObject(ctx, "test-bucket", "test-key-2", content, contentType)
	if err != nil {
		t.Fatalf("CreateUncompressedTestObject failed: %v", err)
	}

	// Verify test object properties
	if testObj.Compressed {
		t.Error("Compressed flag should be false")
	}

	// Verify content sizes are equal (no compression)
	if testObj.StoredSize != testObj.PlaintextSize {
		t.Errorf("StoredSize %d should equal PlaintextSize %d for uncompressed data", testObj.StoredSize, testObj.PlaintextSize)
	}

	// Verify stored content matches original
	if !bytes.Equal(testObj.StoredContent, content) {
		t.Error("StoredContent should match original content for uncompressed objects")
	}

	// Verify metadata
	storedData, metadata, exists := storage.Get(ctx, "test-bucket", "test-key-2")
	if !exists {
		t.Fatal("Object not found in storage")
	}

	if !bytes.Equal(storedData, content) {
		t.Error("Stored data should match original content")
	}

	if metadata["test-compressed"] != "false" {
		t.Error("test-compressed metadata should be false")
	}
}

func TestCreateTestObjectPair(t *testing.T) {
	ctx := context.Background()
	storage := newMockTestStorage()
	ctu := NewCompressionTestUtilities(storage)

	content := []byte("Repeat pattern " + string(make([]byte, 1000))) // Make it compressible
	contentType := "application/json"

	compObj, uncompObj, err := ctu.CreateTestObjectPair(ctx, "test-bucket", "comp-key", "uncomp-key", content, contentType)
	if err != nil {
		t.Fatalf("CreateTestObjectPair failed: %v", err)
	}

	// Verify compressed object
	if compObj == nil {
		t.Fatal("Compressed object is nil")
	}
	if !compObj.Compressed {
		t.Error("Compressed object should have Compressed=true")
	}

	// Verify uncompressed object
	if uncompObj == nil {
		t.Fatal("Uncompressed object is nil")
	}
	if uncompObj.Compressed {
		t.Error("Uncompressed object should have Compressed=false")
	}

	// Verify both have same original content
	if !bytes.Equal(compObj.OriginalContent, uncompObj.OriginalContent) {
		t.Error("Compressed and uncompressed objects should have same original content")
	}

	// Verify both are stored
	if !storage.Exists("comp-key") || !storage.Exists("uncomp-key") {
		t.Error("Both objects should be stored")
	}

	// Verify compressed is actually compressed
	if !IsCompressed(compObj.StoredContent) {
		t.Error("Compressed object should have compressed stored content")
	}

	// Verify uncompressed is not compressed
	if IsCompressed(uncompObj.StoredContent) {
		t.Error("Uncompressed object should not have compressed stored content")
	}
}

func TestCreateTestObjectPair_CompressedFailure(t *testing.T) {
	ctx := context.Background()
	storage := newMockTestStorage()
	ctu := NewCompressionTestUtilities(storage)

	content := []byte("test content")
	contentType := "text/plain"

	// Set error on first Put to simulate compressed creation failure
	storage.SetPutError("comp-key", io.ErrClosedPipe)

	compObj, uncompObj, err := ctu.CreateTestObjectPair(ctx, "test-bucket", "comp-key", "uncomp-key", content, contentType)
	if err == nil {
		t.Error("Expected error when compressed creation fails")
	}
	if compObj != nil || uncompObj != nil {
		t.Error("Should not return objects when creation fails")
	}
}

func TestTeardownTestObject(t *testing.T) {
	ctx := context.Background()
	storage := newMockTestStorage()
	ctu := NewCompressionTestUtilities(storage)

	// Create a test object
	content := []byte("test content")
	_, err := ctu.CreateCompressedTestObject(ctx, "test-bucket", "delete-me", content, "text/plain")
	if err != nil {
		t.Fatalf("Failed to create test object: %v", err)
	}

	// Verify it exists
	if !storage.Exists("delete-me") {
		t.Fatal("Test object should exist before teardown")
	}

	_, tracked := ctu.GetTestObject("delete-me")
	if !tracked {
		t.Fatal("Test object should be tracked before teardown")
	}

	// Teardown
	err = ctu.TeardownTestObject(ctx, "test-bucket", "delete-me")
	if err != nil {
		t.Fatalf("TeardownTestObject failed: %v", err)
	}

	// Verify it's deleted from storage
	if storage.Exists("delete-me") {
		t.Error("Test object should be deleted from storage")
	}

	// Verify it's removed from tracking
	_, tracked = ctu.GetTestObject("delete-me")
	if tracked {
		t.Error("Test object should not be tracked after teardown")
	}
}

func TestTeardownAllTestObjects(t *testing.T) {
	ctx := context.Background()
	storage := newMockTestStorage()
	ctu := NewCompressionTestUtilities(storage)

	// Create multiple test objects
	objects := []string{"obj1", "obj2", "obj3"}
	for _, key := range objects {
		_, err := ctu.CreateCompressedTestObject(ctx, "test-bucket", key, []byte("content"), "text/plain")
		if err != nil {
			t.Fatalf("Failed to create test object %s: %v", key, err)
		}
	}

	// Verify all exist
	for _, key := range objects {
		if !storage.Exists(key) {
			t.Errorf("Object %s should exist before teardown", key)
		}
	}

	// Teardown all
	err := ctu.TeardownAllTestObjects(ctx, "test-bucket")
	if err != nil {
		t.Fatalf("TeardownAllTestObjects failed: %v", err)
	}

	// Verify all are deleted
	for _, key := range objects {
		if storage.Exists(key) {
			t.Errorf("Object %s should be deleted after teardown", key)
		}
	}

	// Verify tracking is cleared
	tracked := ctu.ListTrackedObjects()
	if len(tracked) != 0 {
		t.Errorf("Should have no tracked objects after teardown, got %d", len(tracked))
	}
}

func TestTeardownTestObjectsByKeyPrefix(t *testing.T) {
	ctx := context.Background()
	storage := newMockTestStorage()
	ctu := NewCompressionTestUtilities(storage)

	// Create objects with different prefixes
	objects := map[string]bool{
		"test-obj-1":  true,
		"test-obj-2":  true,
		"test-obj-3":  true,
		"other-obj-1": false,
		"keep-obj-1":  false,
	}

	for key := range objects {
		_, err := ctu.CreateCompressedTestObject(ctx, "test-bucket", key, []byte("content"), "text/plain")
		if err != nil {
			t.Fatalf("Failed to create test object %s: %v", key, err)
		}
	}

	// Teardown by prefix
	err := ctu.TeardownTestObjectsByKeyPrefix(ctx, "test-bucket", "test-obj-")
	if err != nil {
		t.Fatalf("TeardownTestObjectsByKeyPrefix failed: %v", err)
	}

	// Verify correct objects are deleted
	for key, shouldDelete := range objects {
		exists := storage.Exists(key)
		if shouldDelete && exists {
			t.Errorf("Object %s should be deleted", key)
		}
		if !shouldDelete && !exists {
			t.Errorf("Object %s should not be deleted", key)
		}
	}
}

func TestListTrackedObjects(t *testing.T) {
	ctx := context.Background()
	storage := newMockTestStorage()
	ctu := NewCompressionTestUtilities(storage)

	// Initially empty
	tracked := ctu.ListTrackedObjects()
	if len(tracked) != 0 {
		t.Errorf("Should have no tracked objects initially, got %d", len(tracked))
	}

	// Create some objects
	keys := []string{"key1", "key2", "key3"}
	for _, key := range keys {
		_, err := ctu.CreateCompressedTestObject(ctx, "test-bucket", key, []byte("content"), "text/plain")
		if err != nil {
			t.Fatalf("Failed to create test object: %v", err)
		}
	}

	// List tracked objects
	tracked = ctu.ListTrackedObjects()
	if len(tracked) != len(keys) {
		t.Errorf("Should have %d tracked objects, got %d", len(keys), len(tracked))
	}

	// Verify all keys are present
	trackedKeys := make(map[string]bool)
	for _, obj := range tracked {
		trackedKeys[obj.Key] = true
	}
	for _, key := range keys {
		if !trackedKeys[key] {
			t.Errorf("Key %s not found in tracked objects", key)
		}
	}
}

func TestGenerateTestContent(t *testing.T) {
	// Test different sizes
	sizes := []int{100, 1024, 10240}

	for _, size := range sizes {
		content, err := GenerateTestContent(size)
		if err != nil {
			t.Errorf("GenerateTestContent(%d) failed: %v", size, err)
			continue
		}

		if len(content) != size {
			t.Errorf("GenerateTestContent(%d) returned %d bytes, want %d", size, len(content), size)
		}

		// Verify content is not all zeros
		allZeros := true
		for _, b := range content {
			if b != 0 {
				allZeros = false
				break
			}
		}
		if allZeros {
			t.Errorf("GenerateTestContent(%d) returned all zeros", size)
		}
	}
}

func TestGeneratePatternContent(t *testing.T) {
	pattern := "ABC"
	repeatCount := 10

	content := GeneratePatternContent(pattern, repeatCount)
	expectedLen := len(pattern) * repeatCount

	if len(content) != expectedLen {
		t.Errorf("GeneratePatternContent returned %d bytes, want %d", len(content), expectedLen)
	}

	// Verify pattern repeats correctly
	expected := []byte{}
	for i := 0; i < repeatCount; i++ {
		expected = append(expected, []byte(pattern)...)
	}

	if !bytes.Equal(content, expected) {
		t.Error("GeneratePatternContent didn't generate expected pattern")
	}
}

func TestVerifyCompressedData(t *testing.T) {
	original := []byte("Hello, ARMOR test data!")

	// Create compressed data
	var buf bytes.Buffer
	encoder, _ := zstd.NewWriter(&buf)
	encoder.Write(original)
	encoder.Close()
	compressed := buf.Bytes()

	// Verify valid compressed data
	err := VerifyCompressedData(compressed, original)
	if err != nil {
		t.Errorf("VerifyCompressedData failed on valid data: %v", err)
	}

	// Test with uncompressed data (should fail)
	err = VerifyCompressedData(original, original)
	if err == nil {
		t.Error("VerifyCompressedData should fail on uncompressed data")
	}

	// Test with corrupted compressed data
	corrupted := make([]byte, len(compressed))
	copy(corrupted, compressed)
	corrupted[len(corrupted)-1] ^= 0xFF // Corrupt last byte

	err = VerifyCompressedData(corrupted, original)
	if err == nil {
		t.Error("VerifyCompressedData should fail on corrupted data")
	}

	// Test with wrong expected content
	wrongExpected := []byte("wrong content")
	err = VerifyCompressedData(compressed, wrongExpected)
	if err == nil {
		t.Error("VerifyCompressedData should fail when decompressed content doesn't match expected")
	}
}

func TestCalculateCompressionRatio(t *testing.T) {
	tests := []struct {
		name      string
		original  []byte
		compressed []byte
		expected  float64
	}{
		{
			name:      "50% compression",
			original:  make([]byte, 1000),
			compressed: make([]byte, 500),
			expected:  0.5,
		},
		{
			name:      "no compression",
			original:  make([]byte, 100),
			compressed: make([]byte, 100),
			expected:  1.0,
		},
		{
			name:      "expansion (worse than no compression)",
			original:  make([]byte, 100),
			compressed: make([]byte, 150),
			expected:  1.5,
		},
		{
			name:      "empty data",
			original:  []byte{},
			compressed: []byte{},
			expected:  0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ratio := CalculateCompressionRatio(tt.original, tt.compressed)
			if ratio != tt.expected {
				t.Errorf("CalculateCompressionRatio() = %f, want %f", ratio, tt.expected)
			}
		})
	}
}

func TestCreateCompressedTestObject_PutFailure(t *testing.T) {
	ctx := context.Background()
	storage := newMockTestStorage()
	ctu := NewCompressionTestUtilities(storage)

	content := []byte("test content")

	// Simulate storage failure
	storage.SetPutError("fail-key", io.ErrClosedPipe)

	_, err := ctu.CreateCompressedTestObject(ctx, "test-bucket", "fail-key", content, "text/plain")
	if err == nil {
		t.Error("Expected error when storage Put fails")
	}

	// Verify object is not tracked
	_, tracked := ctu.GetTestObject("fail-key")
	if tracked {
		t.Error("Failed object should not be tracked")
	}
}

func TestTeardownTestObject_DeleteFailure(t *testing.T) {
	ctx := context.Background()
	storage := newMockTestStorage()
	ctu := NewCompressionTestUtilities(storage)

	// Create a test object
	content := []byte("test content")
	_, err := ctu.CreateCompressedTestObject(ctx, "test-bucket", "delete-fail", content, "text/plain")
	if err != nil {
		t.Fatalf("Failed to create test object: %v", err)
	}

	// Simulate delete failure
	storage.SetDeleteError("delete-fail", io.ErrClosedPipe)

	// Teardown should fail
	err = ctu.TeardownTestObject(ctx, "test-bucket", "delete-fail")
	if err == nil {
		t.Error("Expected error when storage Delete fails")
	}

	// Object should still be tracked even after delete failure
	_, tracked := ctu.GetTestObject("delete-fail")
	if !tracked {
		t.Error("Object should remain tracked after failed deletion")
	}
}

func TestTeardownAllTestObjects_PartialFailure(t *testing.T) {
	ctx := context.Background()
	storage := newMockTestStorage()
	ctu := NewCompressionTestUtilities(storage)

	// Create multiple objects
	_, err := ctu.CreateCompressedTestObject(ctx, "test-bucket", "obj1", []byte("content1"), "text/plain")
	if err != nil {
		t.Fatalf("Failed to create obj1: %v", err)
	}
	_, err = ctu.CreateCompressedTestObject(ctx, "test-bucket", "obj2", []byte("content2"), "text/plain")
	if err != nil {
		t.Fatalf("Failed to create obj2: %v", err)
	}

	// Get initial call count (might have calls from setup)
	initialCalls := storage.DeleteCallCount()

	// Simulate delete failure for obj2
	storage.SetDeleteError("obj2", io.ErrClosedPipe)

	// Teardown all should return error but attempt both deletions
	err = ctu.TeardownAllTestObjects(ctx, "test-bucket")
	if err == nil {
		t.Error("Expected error when one deletion fails")
	}

	// Tracking should still be cleared
	tracked := ctu.ListTrackedObjects()
	if len(tracked) != 0 {
		t.Errorf("Tracking should be cleared even after partial failure, got %d objects", len(tracked))
	}

	// Verify both delete attempts were made
	finalCalls := storage.DeleteCallCount()
	attemptedDeletions := finalCalls - initialCalls
	if attemptedDeletions != 2 {
		t.Errorf("Should have attempted 2 deletions, got %d (initial: %d, final: %d)", attemptedDeletions, initialCalls, finalCalls)
	}

	// Verify that obj1 was actually deleted (no error set for it)
	if storage.Exists("obj1") {
		t.Error("obj1 should have been deleted successfully")
	}

	// Verify that obj2 failed to delete
	if !storage.Exists("obj2") {
		t.Error("obj2 should still exist (delete failed)")
	}
}

func TestCompressionTestUtilities_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	storage := newMockTestStorage()
	ctu := NewCompressionTestUtilities(storage)

	// Test concurrent object creation
	var wg sync.WaitGroup
	numGoroutines := 10
	objectsPerGoroutine := 5

	errors := make(chan error, numGoroutines*objectsPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < objectsPerGoroutine; j++ {
				key := fmt.Sprintf("goroutine-%d-obj-%d", goroutineID, j)
				_, err := ctu.CreateCompressedTestObject(ctx, "test-bucket", key, []byte("content"), "text/plain")
				if err != nil {
					errors <- err
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	errorCount := 0
	for err := range errors {
		errorCount++
		t.Errorf("Concurrent creation error: %v", err)
	}

	if errorCount > 0 {
		t.Fatalf("Had %d errors during concurrent access", errorCount)
	}

	// Verify all objects were created and tracked
	expectedCount := numGoroutines * objectsPerGoroutine
	tracked := ctu.ListTrackedObjects()
	if len(tracked) != expectedCount {
		t.Errorf("Expected %d tracked objects, got %d", expectedCount, len(tracked))
	}
}

func TestGetTestObject_NotFound(t *testing.T) {
	storage := newMockTestStorage()
	ctu := NewCompressionTestUtilities(storage)

	obj, exists := ctu.GetTestObject("nonexistent")
	if exists {
		t.Error("Should not find nonexistent object")
	}
	if obj != nil {
		t.Error("Should return nil object when not found")
	}
}

func TestCreateCompressedTestObject_DifferentContentTypes(t *testing.T) {
	ctx := context.Background()
	storage := newMockTestStorage()
	ctu := NewCompressionTestUtilities(storage)

	contentTypes := []string{
		"application/octet-stream",
		"text/plain",
		"application/json",
		"application/xml",
		"image/png",
	}

	for _, contentType := range contentTypes {
		_, err := ctu.CreateCompressedTestObject(ctx, "test-bucket", "key-"+contentType, []byte("content"), contentType)
		if err != nil {
			t.Errorf("Failed to create object with content-type %s: %v", contentType, err)
		}

		// Verify metadata
		_, metadata, exists := storage.Get(ctx, "test-bucket", "key-"+contentType)
		if !exists {
			t.Errorf("Object with content-type %s not found", contentType)
			continue
		}

		if metadata["content-type"] != contentType {
			t.Errorf("Content-type metadata mismatch for %s", contentType)
		}
	}
}
