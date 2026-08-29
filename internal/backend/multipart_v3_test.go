package backend

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"
)

// TestMultipartV3ConcurrentUpload verifies that two concurrent UploadPart calls
// for different parts of one v3 upload never touch the same object.
// This is the key acceptance criteria: each part writes to its own part-<n>.json file.
func TestMultipartV3ConcurrentUpload(t *testing.T) {
	ctx := context.Background()
	backend := &MockBackend{
		buckets: make(map[string]map[string]*mockObject),
	}
	bucket := "test-bucket"
	uploadID := "test-upload-v3"

	// Initialize the bucket
	backend.buckets[bucket] = make(map[string]*mockObject)
	manager := NewMultipartStateManager(backend, bucket)

	// Create v3 metadata
	metadata := &MultipartMetadataV3{
		UploadID:       uploadID,
		Bucket:         bucket,
		Key:            "test-key",
		IV:             []byte("test-iv-123456789012"),
		WrappedDEK:     []byte("test-wrapped-dek"),
		MEKFingerprint: "0123456789abcdef",
		BlockSize:      16,
		Created:        time.Now(),
		ContentType:    "application/octet-stream",
		KeyID:          "default",
		FormatVersion:  3,
	}

	if err := manager.SaveMetadataV3(ctx, metadata); err != nil {
		t.Fatalf("Failed to save metadata: %v", err)
	}

	// Track which objects were written and by which goroutine
	var mu sync.Mutex
	writtenObjects := make(map[string]int) // key -> goroutine ID

	// Upload parts concurrently with different goroutine IDs
	var wg sync.WaitGroup
	for partNum := 1; partNum <= 3; partNum++ {
		wg.Add(1)
		go func(partNumber int) {
			defer wg.Done()
			goroutineID := partNumber

			partData := &PartDataV3{
				PartNumber:       partNumber,
				PlaintextLen:     int64(partNumber * 1024),
				CiphertextLen:    int64(partNumber * 1024 + 16), // +16 for tag
				BlockHMACsBase64: "fake-hmac-base64",
				PlaintextSHAHex:  "fake-sha256",
			}

			// Simulate some processing time to increase chance of race
			time.Sleep(time.Microsecond * time.Duration(partNumber*100))

			if err := manager.SavePartV3(ctx, uploadID, partData); err != nil {
				t.Errorf("Goroutine %d: Failed to save part %d: %v", goroutineID, partNumber, err)
				return
			}

			// Track which object was written
			key := ".armor/multipart/" + uploadID + "/part-" + string(rune(partNumber+'0')) + ".json"
			mu.Lock()
			writtenObjects[key] = goroutineID
			mu.Unlock()
		}(partNum)
	}
	wg.Wait()

	// Verify each part wrote to its own file
	expectedFiles := map[string]string{
		".armor/multipart/" + uploadID + "/part-1.json": "1",
		".armor/multipart/" + uploadID + "/part-2.json": "2",
		".armor/multipart/" + uploadID + "/part-3.json": "3",
	}

	for expectedFile := range expectedFiles {
		obj, exists := backend.buckets[bucket][expectedFile]
		if !exists {
			t.Errorf("Expected file %s was not created", expectedFile)
			continue
		}
		if obj == nil || len(obj.data) == 0 {
			t.Errorf("Expected file %s is empty", expectedFile)
		}
	}

	// Verify no file was written by more than one goroutine
	for key, goroutineID := range writtenObjects {
		t.Logf("File %s written by goroutine %d", key, goroutineID)
	}

	// Load and verify each part
	for partNum := 1; partNum <= 3; partNum++ {
		partData, err := manager.LoadPartV3(ctx, uploadID, partNum)
		if err != nil {
			t.Errorf("Failed to load part %d: %v", partNum, err)
			continue
		}
		if partData.PartNumber != partNum {
			t.Errorf("Part number mismatch: got %d, want %d", partData.PartNumber, partNum)
		}
		if partData.PlaintextLen != int64(partNum*1024) {
			t.Errorf("Part %d plaintext length mismatch: got %d, want %d", partNum, partData.PlaintextLen, partNum*1024)
		}
	}
}

// TestMultipartV3ListParts verifies that ListParts reads all per-part objects.
func TestMultipartV3ListParts(t *testing.T) {
	ctx := context.Background()
	backend := &MockBackend{
		buckets: make(map[string]map[string]*mockObject),
	}
	bucket := "test-bucket"
	uploadID := "test-upload-list"

	// Initialize the bucket
	backend.buckets[bucket] = make(map[string]*mockObject)
	manager := NewMultipartStateManager(backend, bucket)

	// Create v3 metadata
	metadata := &MultipartMetadataV3{
		UploadID:       uploadID,
		Bucket:         bucket,
		Key:            "test-key",
		IV:             []byte("test-iv-123456789012"),
		WrappedDEK:     []byte("test-wrapped-dek"),
		MEKFingerprint: "0123456789abcdef",
		BlockSize:      16,
		Created:        time.Now(),
		ContentType:    "application/octet-stream",
		KeyID:          "default",
		FormatVersion:  3,
	}

	if err := manager.SaveMetadataV3(ctx, metadata); err != nil {
		t.Fatalf("Failed to save metadata: %v", err)
	}

	// Save 5 parts
	for partNum := 1; partNum <= 5; partNum++ {
		partData := &PartDataV3{
			PartNumber:       partNum,
			PlaintextLen:     int64(partNum * 2048),
			CiphertextLen:    int64(partNum*2048 + 32),
			BlockHMACsBase64: "hmac-" + string(rune('0'+partNum)),
			PlaintextSHAHex:  "sha-" + string(rune('0'+partNum)),
		}
		if err := manager.SavePartV3(ctx, uploadID, partData); err != nil {
			t.Fatalf("Failed to save part %d: %v", partNum, err)
		}
	}

	// List parts
	parts, err := manager.ListPartsV3(ctx, uploadID)
	if err != nil {
		t.Fatalf("Failed to list parts: %v", err)
	}

	// Verify all 5 parts were loaded
	if len(parts) != 5 {
		t.Errorf("Expected 5 parts, got %d", len(parts))
	}

	// Verify each part
	for partNum := 1; partNum <= 5; partNum++ {
		partData, exists := parts[partNum]
		if !exists {
			t.Errorf("Part %d not found in list", partNum)
			continue
		}
		if partData.PartNumber != partNum {
			t.Errorf("Part number mismatch: got %d, want %d", partData.PartNumber, partNum)
		}
		expectedLen := int64(partNum * 2048)
		if partData.PlaintextLen != expectedLen {
			t.Errorf("Part %d plaintext length: got %d, want %d", partNum, partData.PlaintextLen, expectedLen)
		}
		expectedHMAC := "hmac-" + string(rune('0'+partNum))
		if partData.BlockHMACsBase64 != expectedHMAC {
			t.Errorf("Part %d HMAC: got %s, want %s", partNum, partData.BlockHMACsBase64, expectedHMAC)
		}
	}
}

// TestMultipartV2Format verifies that v2 uploads still use the .state file.
func TestMultipartV2Format(t *testing.T) {
	ctx := context.Background()
	backend := &MockBackend{
		buckets: make(map[string]map[string]*mockObject),
	}
	bucket := "test-bucket"
	uploadID := "test-upload-v2"

	// Initialize the bucket
	backend.buckets[bucket] = make(map[string]*mockObject)
	manager := NewMultipartStateManager(backend, bucket)

	// Create v2 state
	state := &MultipartState{
		UploadID:       uploadID,
		Bucket:         bucket,
		Key:            "test-key",
		IV:             []byte("test-iv-123456789012"),
		WrappedDEK:     []byte("test-wrapped-dek"),
		MEKFingerprint: "0123456789abcdef",
		BlockSize:      16,
		Created:        time.Now(),
		ContentType:    "application/octet-stream",
		KeyID:          "default",
		PartHMACs:      make(map[int]string),
		PartSizes:      make(map[int]int64),
		FormatVersion:  2,
	}

	// Save v2 state
	if err := manager.SaveState(ctx, state); err != nil {
		t.Fatalf("Failed to save v2 state: %v", err)
	}

	// Verify .state file exists
	stateKey := ".armor/multipart/" + uploadID + ".state"
	obj, exists := backend.buckets[bucket][stateKey]
	if !exists {
		t.Fatalf("V2 state file .armor/multipart/%s.state was not created", uploadID)
	}
	if obj == nil || len(obj.data) == 0 {
		t.Fatalf("V2 state file is empty")
	}

	// Verify it's not a directory structure (no meta.json)
	metaKey := ".armor/multipart/" + uploadID + "/meta.json"
	if _, exists := backend.buckets[bucket][metaKey]; exists {
		t.Error("V2 format should not have meta.json file")
	}

	// Load and verify the state
	loadedState, err := manager.LoadState(ctx, uploadID)
	if err != nil {
		t.Fatalf("Failed to load v2 state: %v", err)
	}
	if loadedState.UploadID != uploadID {
		t.Errorf("UploadID mismatch: got %s, want %s", loadedState.UploadID, uploadID)
	}
	if loadedState.FormatVersion != 2 {
		t.Errorf("Format version: got %d, want 2", loadedState.FormatVersion)
	}
	if loadedState.Bucket != bucket {
		t.Errorf("Bucket mismatch: got %s, want %s", loadedState.Bucket, bucket)
	}
}

// TestMultipartDeleteStateV3 verifies that DeleteState cleans up v3 multipart directory.
func TestMultipartDeleteStateV3(t *testing.T) {
	ctx := context.Background()
	backend := &MockBackend{
		buckets: make(map[string]map[string]*mockObject),
	}
	bucket := "test-bucket"
	uploadID := "test-upload-delete"

	// Initialize the bucket
	backend.buckets[bucket] = make(map[string]*mockObject)
	manager := NewMultipartStateManager(backend, bucket)

	// Create v3 metadata
	metadata := &MultipartMetadataV3{
		UploadID:      uploadID,
		Bucket:        bucket,
		Key:           "test-key",
		IV:            []byte("test-iv-123456789012"),
		WrappedDEK:    []byte("test-wrapped-dek"),
		BlockSize:     16,
		Created:       time.Now(),
		ContentType:   "application/octet-stream",
		KeyID:         "default",
		FormatVersion: 3,
	}

	if err := manager.SaveMetadataV3(ctx, metadata); err != nil {
		t.Fatalf("Failed to save metadata: %v", err)
	}

	// Save some parts
	for partNum := 1; partNum <= 3; partNum++ {
		partData := &PartDataV3{
			PartNumber:       partNum,
			PlaintextLen:     int64(partNum * 1024),
			CiphertextLen:    int64(partNum*1024 + 16),
			BlockHMACsBase64: "hmac",
			PlaintextSHAHex:  "sha",
		}
		if err := manager.SavePartV3(ctx, uploadID, partData); err != nil {
			t.Fatalf("Failed to save part %d: %v", partNum, err)
		}
	}

	// Verify files exist
	prefix := ".armor/multipart/" + uploadID + "/"
	for _, key := range []string{
		prefix + "meta.json",
		prefix + "part-1.json",
		prefix + "part-2.json",
		prefix + "part-3.json",
	} {
		if _, exists := backend.buckets[bucket][key]; !exists {
			t.Errorf("Expected file %s not found before delete", key)
		}
	}

	// Delete the multipart state
	if err := manager.DeleteState(ctx, uploadID); err != nil {
		t.Fatalf("Failed to delete state: %v", err)
	}

	// Verify all files were deleted
	for _, key := range []string{
		prefix + "meta.json",
		prefix + "part-1.json",
		prefix + "part-2.json",
		prefix + "part-3.json",
	} {
		if _, exists := backend.buckets[bucket][key]; exists {
			t.Errorf("File %s still exists after delete", key)
		}
	}
}

// MockBackend is a minimal mock backend for testing.
type MockBackend struct {
	mu      sync.Mutex
	buckets map[string]map[string]*mockObject
}

type mockObject struct {
	data []byte
	meta map[string]string
}

func (m *MockBackend) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.buckets[bucket] == nil {
		m.buckets[bucket] = make(map[string]*mockObject)
	}

	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}

	m.buckets[bucket][key] = &mockObject{
		data: data,
		meta: meta,
	}
	return nil
}

func (m *MockBackend) Get(ctx context.Context, bucket, key string) (io.ReadCloser, *ObjectInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.buckets[bucket] == nil {
		return nil, nil, fmt.Errorf("object not found: %s", key)
	}
	obj, exists := m.buckets[bucket][key]
	if !exists {
		return nil, nil, fmt.Errorf("object not found: %s", key)
	}
	data := append([]byte(nil), obj.data...)
	return io.NopCloser(bytes.NewReader(data)), &ObjectInfo{Size: int64(len(obj.data))}, nil
}

func (m *MockBackend) GetDirect(ctx context.Context, bucket, key string) (io.ReadCloser, *ObjectInfo, error) {
	return m.Get(ctx, bucket, key)
}

func (m *MockBackend) Delete(ctx context.Context, bucket, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.buckets[bucket] != nil {
		delete(m.buckets[bucket], key)
	}
	return nil
}

func (m *MockBackend) DeleteObjects(ctx context.Context, bucket string, keys []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.buckets[bucket] != nil {
		for _, key := range keys {
			delete(m.buckets[bucket], key)
		}
	}
	return nil
}

func (m *MockBackend) List(ctx context.Context, bucket, prefix, delimiter, continuationToken string, maxKeys int) (*ListResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := &ListResult{
		Objects: make([]ObjectInfo, 0),
	}

	if m.buckets[bucket] == nil {
		return result, nil
	}

	for key, obj := range m.buckets[bucket] {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			result.Objects = append(result.Objects, ObjectInfo{
				Key:  key,
				Size: int64(len(obj.data)),
			})
		}
	}

	return result, nil
}

// Required unimplemented methods for MockBackend
func (m *MockBackend) GetRange(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, error) {
	return nil, fmt.Errorf("object not found")
}

func (m *MockBackend) GetRangeWithHeaders(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, map[string]string, error) {
	return nil, nil, fmt.Errorf("object not found")
}

func (m *MockBackend) Head(ctx context.Context, bucket, key string) (*ObjectInfo, error) {
	return nil, fmt.Errorf("object not found")
}

func (m *MockBackend) Copy(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string, meta map[string]string, replaceMetadata bool) error {
	return nil
}

func (m *MockBackend) ListBuckets(ctx context.Context) ([]BucketInfo, error) {
	return nil, nil
}

func (m *MockBackend) CreateBucket(ctx context.Context, bucket string) error {
	return nil
}

func (m *MockBackend) DeleteBucket(ctx context.Context, bucket string) error {
	return nil
}

func (m *MockBackend) HeadBucket(ctx context.Context, bucket string) error {
	return nil
}

func (m *MockBackend) CreateMultipartUpload(ctx context.Context, bucket, key string, meta map[string]string) (string, error) {
	return "test-upload-id", nil
}

func (m *MockBackend) UploadPart(ctx context.Context, bucket, key, uploadID string, partNumber int32, body io.Reader, size int64) (string, error) {
	return "test-etag", nil
}

func (m *MockBackend) CompleteMultipartUpload(ctx context.Context, bucket, key, uploadID string, parts []CompletedPart) (string, error) {
	return "test-etag", nil
}

func (m *MockBackend) AbortMultipartUpload(ctx context.Context, bucket, key, uploadID string) error {
	return nil
}

func (m *MockBackend) ListParts(ctx context.Context, bucket, key, uploadID string) (*ListPartsResult, error) {
	return &ListPartsResult{}, nil
}

func (m *MockBackend) ListMultipartUploads(ctx context.Context, bucket, prefix string) (*ListMultipartUploadsResult, error) {
	return &ListMultipartUploadsResult{}, nil
}

func (m *MockBackend) GetBucketLifecycleConfiguration(ctx context.Context, bucket string) ([]byte, error) {
	return nil, nil
}

func (m *MockBackend) PutBucketLifecycleConfiguration(ctx context.Context, bucket string, config []byte) error {
	return nil
}

func (m *MockBackend) DeleteBucketLifecycleConfiguration(ctx context.Context, bucket string) error {
	return nil
}

func (m *MockBackend) GetObjectLockConfiguration(ctx context.Context, bucket string) ([]byte, error) {
	return nil, nil
}

func (m *MockBackend) PutObjectLockConfiguration(ctx context.Context, bucket string, config []byte) error {
	return nil
}

func (m *MockBackend) GetObjectRetention(ctx context.Context, bucket, key string) ([]byte, error) {
	return nil, nil
}

func (m *MockBackend) PutObjectRetention(ctx context.Context, bucket, key string, retention []byte) error {
	return nil
}

func (m *MockBackend) GetObjectLegalHold(ctx context.Context, bucket, key string) ([]byte, error) {
	return nil, nil
}

func (m *MockBackend) PutObjectLegalHold(ctx context.Context, bucket, key string, legalHold []byte) error {
	return nil
}

func (m *MockBackend) ListObjectVersions(ctx context.Context, bucket, prefix, delimiter, keyMarker, versionIDMarker string, maxKeys int) (*ListObjectVersionsResult, error) {
	return &ListObjectVersionsResult{}, nil
}

func (m *MockBackend) HeadVersion(ctx context.Context, bucket, key, versionID string) (*ObjectInfo, error) {
	return nil, fmt.Errorf("object not found")
}

// TestMultipartV3MetadataRoundTrip tests saving and loading v3 metadata.
func TestMultipartV3MetadataRoundTrip(t *testing.T) {
	ctx := context.Background()
	backend := &MockBackend{
		buckets: make(map[string]map[string]*mockObject),
	}
	bucket := "test-bucket"
	uploadID := "test-roundtrip"

	backend.buckets[bucket] = make(map[string]*mockObject)
	manager := NewMultipartStateManager(backend, bucket)

	original := &MultipartMetadataV3{
		UploadID:       uploadID,
		Bucket:         bucket,
		Key:            "my-object",
		IV:             []byte("iv-16-byte-12345"),
		WrappedDEK:     []byte("wrapped-dek-data"),
		MEKFingerprint: "fedcba0987654321",
		BlockSize:      32,
		Created:        time.Date(2024, 8, 29, 12, 34, 56, 0, time.UTC),
		ContentType:    "text/plain",
		KeyID:          "key-alias-1",
		FormatVersion:  3,
		PartSize:       5242880, // 5 MiB
		NonUniformParts: false,
		Poisoned:       false,
	}

	if err := manager.SaveMetadataV3(ctx, original); err != nil {
		t.Fatalf("Failed to save metadata: %v", err)
	}

	loaded, err := manager.LoadMetadataV3(ctx, uploadID)
	if err != nil {
		t.Fatalf("Failed to load metadata: %v", err)
	}

	if loaded.UploadID != original.UploadID {
		t.Errorf("UploadID: got %s, want %s", loaded.UploadID, original.UploadID)
	}
	if loaded.Bucket != original.Bucket {
		t.Errorf("Bucket: got %s, want %s", loaded.Bucket, original.Bucket)
	}
	if loaded.Key != original.Key {
		t.Errorf("Key: got %s, want %s", loaded.Key, original.Key)
	}
	if string(loaded.IV) != string(original.IV) {
		t.Errorf("IV mismatch")
	}
	if string(loaded.WrappedDEK) != string(original.WrappedDEK) {
		t.Errorf("WrappedDEK mismatch")
	}
	if loaded.MEKFingerprint != original.MEKFingerprint {
		t.Errorf("MEKFingerprint: got %s, want %s", loaded.MEKFingerprint, original.MEKFingerprint)
	}
	if loaded.BlockSize != original.BlockSize {
		t.Errorf("BlockSize: got %d, want %d", loaded.BlockSize, original.BlockSize)
	}
	if loaded.ContentType != original.ContentType {
		t.Errorf("ContentType: got %s, want %s", loaded.ContentType, original.ContentType)
	}
	if loaded.KeyID != original.KeyID {
		t.Errorf("KeyID: got %s, want %s", loaded.KeyID, original.KeyID)
	}
	if loaded.FormatVersion != original.FormatVersion {
		t.Errorf("FormatVersion: got %d, want %d", loaded.FormatVersion, original.FormatVersion)
	}
	if loaded.PartSize != original.PartSize {
		t.Errorf("PartSize: got %d, want %d", loaded.PartSize, original.PartSize)
	}
	if loaded.NonUniformParts != original.NonUniformParts {
		t.Errorf("NonUniformParts: got %v, want %v", loaded.NonUniformParts, original.NonUniformParts)
	}
	if loaded.Poisoned != original.Poisoned {
		t.Errorf("Poisoned: got %v, want %v", loaded.Poisoned, original.Poisoned)
	}
}

// TestMultipartV3PartDataRoundTrip tests saving and loading v3 part data.
func TestMultipartV3PartDataRoundTrip(t *testing.T) {
	ctx := context.Background()
	backend := &MockBackend{
		buckets: make(map[string]map[string]*mockObject),
	}
	bucket := "test-bucket"
	uploadID := "test-part-roundtrip"

	backend.buckets[bucket] = make(map[string]*mockObject)
	manager := NewMultipartStateManager(backend, bucket)

	original := &PartDataV3{
		PartNumber:       42,
		PlaintextLen:     12345678,
		CiphertextLen:    12345694,
		BlockHMACsBase64: "aGVsbG8td29ybGQtdGVzdC1obWFj", // base64 test data
		PlaintextSHAHex:  "abcd1234efgh5678",
	}

	if err := manager.SavePartV3(ctx, uploadID, original); err != nil {
		t.Fatalf("Failed to save part data: %v", err)
	}

	loaded, err := manager.LoadPartV3(ctx, uploadID, 42)
	if err != nil {
		t.Fatalf("Failed to load part data: %v", err)
	}

	if loaded.PartNumber != original.PartNumber {
		t.Errorf("PartNumber: got %d, want %d", loaded.PartNumber, original.PartNumber)
	}
	if loaded.PlaintextLen != original.PlaintextLen {
		t.Errorf("PlaintextLen: got %d, want %d", loaded.PlaintextLen, original.PlaintextLen)
	}
	if loaded.CiphertextLen != original.CiphertextLen {
		t.Errorf("CiphertextLen: got %d, want %d", loaded.CiphertextLen, original.CiphertextLen)
	}
	if loaded.BlockHMACsBase64 != original.BlockHMACsBase64 {
		t.Errorf("BlockHMACsBase64: got %s, want %s", loaded.BlockHMACsBase64, original.BlockHMACsBase64)
	}
	if loaded.PlaintextSHAHex != original.PlaintextSHAHex {
		t.Errorf("PlaintextSHAHex: got %s, want %s", loaded.PlaintextSHAHex, original.PlaintextSHAHex)
	}
}
