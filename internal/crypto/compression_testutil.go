package crypto

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"sync"

	"github.com/klauspost/compress/zstd"
)

// TestObject represents a test object with its metadata and content
type TestObject struct {
	Key              string
	Bucket           string
	Compressed       bool
	OriginalContent  []byte
	StoredContent    []byte
	PlaintextSize    int64
	StoredSize       int64
	ContentType      string
}

// TestObjectStorage defines the interface for storing test objects
type TestObjectStorage interface {
	Put(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error
	Delete(ctx context.Context, bucket, key string) error
}

// CompressionTestUtilities provides utilities for creating and managing compressed test objects
type CompressionTestUtilities struct {
	storage    TestObjectStorage
	objects    map[string]*TestObject
	mu         sync.RWMutex
	cleanupKey string
}

// NewCompressionTestUtilities creates a new compression test utilities instance
func NewCompressionTestUtilities(storage TestObjectStorage) *CompressionTestUtilities {
	return &CompressionTestUtilities{
		storage:    storage,
		objects:    make(map[string]*TestObject),
		cleanupKey: "test-compression-obj-",
	}
}

// CreateCompressedTestObject creates a compressed test object with known content
func (ctu *CompressionTestUtilities) CreateCompressedTestObject(ctx context.Context, bucket, key string, content []byte, contentType string) (*TestObject, error) {
	// Compress the content
	compressed, err := ctu.compressData(content)
	if err != nil {
		return nil, fmt.Errorf("failed to compress content: %w", err)
	}

	// Create the test object
	testObj := &TestObject{
		Key:             key,
		Bucket:          bucket,
		Compressed:      true,
		OriginalContent: content,
		StoredContent:   compressed,
		PlaintextSize:   int64(len(content)),
		StoredSize:      int64(len(compressed)),
		ContentType:     contentType,
	}

	// Store the object
	metadata := map[string]string{
		"content-type": contentType,
		"test-compressed": "true",
	}

	if err := ctu.storage.Put(ctx, bucket, key, bytes.NewReader(compressed), int64(len(compressed)), metadata); err != nil {
		return nil, fmt.Errorf("failed to store compressed object: %w", err)
	}

	// Track the object
	ctu.mu.Lock()
	ctu.objects[key] = testObj
	ctu.mu.Unlock()

	return testObj, nil
}

// CreateUncompressedTestObject creates an uncompressed test object with the same content
func (ctu *CompressionTestUtilities) CreateUncompressedTestObject(ctx context.Context, bucket, key string, content []byte, contentType string) (*TestObject, error) {
	// Create the test object (no compression)
	testObj := &TestObject{
		Key:             key,
		Bucket:          bucket,
		Compressed:      false,
		OriginalContent: content,
		StoredContent:   content,
		PlaintextSize:   int64(len(content)),
		StoredSize:      int64(len(content)),
		ContentType:     contentType,
	}

	// Store the object
	metadata := map[string]string{
		"content-type": contentType,
		"test-compressed": "false",
	}

	if err := ctu.storage.Put(ctx, bucket, key, bytes.NewReader(content), int64(len(content)), metadata); err != nil {
		return nil, fmt.Errorf("failed to store uncompressed object: %w", err)
	}

	// Track the object
	ctu.mu.Lock()
	ctu.objects[key] = testObj
	ctu.mu.Unlock()

	return testObj, nil
}

// CreateTestObjectPair creates both compressed and uncompressed versions of the same content
func (ctu *CompressionTestUtilities) CreateTestObjectPair(ctx context.Context, bucket, compressedKey, uncompressedKey string, content []byte, contentType string) (compressedObj, uncompressedObj *TestObject, err error) {
	// Create compressed version
	compObj, err := ctu.CreateCompressedTestObject(ctx, bucket, compressedKey, content, contentType)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create compressed object: %w", err)
	}

	// Create uncompressed version
	uncompObj, err := ctu.CreateUncompressedTestObject(ctx, bucket, uncompressedKey, content, contentType)
	if err != nil {
		// Clean up the compressed object if uncompressed creation fails
		_ = ctu.storage.Delete(ctx, bucket, compressedKey)
		return nil, nil, fmt.Errorf("failed to create uncompressed object: %w", err)
	}

	return compObj, uncompObj, nil
}

// GetTestObject retrieves a tracked test object by key
func (ctu *CompressionTestUtilities) GetTestObject(key string) (*TestObject, bool) {
	ctu.mu.RLock()
	defer ctu.mu.RUnlock()

	obj, exists := ctu.objects[key]
	return obj, exists
}

// TeardownTestObject removes a specific test object from storage and tracking
func (ctu *CompressionTestUtilities) TeardownTestObject(ctx context.Context, bucket, key string) error {
	// Delete from storage
	if err := ctu.storage.Delete(ctx, bucket, key); err != nil {
		return fmt.Errorf("failed to delete object from storage: %w", err)
	}

	// Remove from tracking
	ctu.mu.Lock()
	delete(ctu.objects, key)
	ctu.mu.Unlock()

	return nil
}

// TeardownAllTestObjects removes all tracked test objects
func (ctu *CompressionTestUtilities) TeardownAllTestObjects(ctx context.Context, bucket string) error {
	ctu.mu.Lock()
	keys := make([]string, 0, len(ctu.objects))
	for key := range ctu.objects {
		keys = append(keys, key)
	}
	ctu.mu.Unlock()

	var firstErr error
	for _, key := range keys {
		if err := ctu.storage.Delete(ctx, bucket, key); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("failed to delete object %s: %w", key, err)
		}
	}

	// Clear tracking
	ctu.mu.Lock()
	ctu.objects = make(map[string]*TestObject)
	ctu.mu.Unlock()

	return firstErr
}

// TeardownTestObjectsByKeyPrefix removes test objects matching a key prefix
func (ctu *CompressionTestUtilities) TeardownTestObjectsByKeyPrefix(ctx context.Context, bucket, prefix string) error {
	ctu.mu.Lock()
	var keysToDelete []string
	for key := range ctu.objects {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			keysToDelete = append(keysToDelete, key)
		}
	}
	ctu.mu.Unlock()

	var firstErr error
	for _, key := range keysToDelete {
		if err := ctu.storage.Delete(ctx, bucket, key); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("failed to delete object %s: %w", key, err)
		}
		// Remove from tracking
		ctu.mu.Lock()
		delete(ctu.objects, key)
		ctu.mu.Unlock()
	}

	return firstErr
}

// ListTrackedObjects returns a list of all tracked test objects
func (ctu *CompressionTestUtilities) ListTrackedObjects() []*TestObject {
	ctu.mu.RLock()
	defer ctu.mu.RUnlock()

	objects := make([]*TestObject, 0, len(ctu.objects))
	for _, obj := range ctu.objects {
		objects = append(objects, obj)
	}
	return objects
}

// compressData compresses data using zstd
func (ctu *CompressionTestUtilities) compressData(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	encoder, err := zstd.NewWriter(&buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create zstd encoder: %w", err)
	}

	if _, err := encoder.Write(data); err != nil {
		encoder.Close()
		return nil, fmt.Errorf("failed to write compressed data: %w", err)
	}

	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("failed to close encoder: %w", err)
	}

	return buf.Bytes(), nil
}

// GenerateTestContent generates random test content of the specified size
func GenerateTestContent(size int) ([]byte, error) {
	content := make([]byte, size)
	if _, err := rand.Read(content); err != nil {
		return nil, fmt.Errorf("failed to generate random content: %w", err)
	}
	return content, nil
}

// GeneratePatternContent generates content with a repeating pattern for testing
func GeneratePatternContent(pattern string, repeatCount int) []byte {
	var content []byte
	for i := 0; i < repeatCount; i++ {
		content = append(content, []byte(pattern)...)
	}
	return content
}

// VerifyCompressedData verifies that data decompresses correctly and matches expected content
func VerifyCompressedData(compressed, expected []byte) error {
	// Check if it's actually compressed
	if !IsCompressed(compressed) {
		return fmt.Errorf("data is not compressed (missing zstd magic bytes)")
	}

	// Decompress
	decompressed, err := Decompress(compressed)
	if err != nil {
		return fmt.Errorf("decompression failed: %w", err)
	}

	// Verify content matches
	if !bytes.Equal(decompressed, expected) {
		return fmt.Errorf("decompressed content doesn't match expected (got %d bytes, expected %d bytes)",
			len(decompressed), len(expected))
	}

	return nil
}

// CalculateCompressionRatio calculates the compression ratio
func CalculateCompressionRatio(original, compressed []byte) float64 {
	if len(original) == 0 {
		return 0.0
	}
	return float64(len(compressed)) / float64(len(original))
}
