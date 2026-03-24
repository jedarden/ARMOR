package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"sync"
	"testing"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

// mockRotationBackend is a mock backend for testing key rotation.
type mockRotationBackend struct {
	mu      sync.Mutex
	objects map[string]*mockObject
}

type mockObject struct {
	data     []byte
	metadata map[string]string
}

func newMockRotationBackend() *mockRotationBackend {
	return &mockRotationBackend{
		objects: make(map[string]*mockObject),
	}
}

// sortObjectsByKey sorts backend.ObjectInfo slice by key (lexicographic order)
func sortObjectsByKey(objects []backend.ObjectInfo) {
	sort.Slice(objects, func(i, j int) bool {
		return objects[i].Key < objects[j].Key
	})
}

func (m *mockRotationBackend) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}

	m.objects[bucket+"/"+key] = &mockObject{
		data:     data,
		metadata: meta,
	}
	return nil
}

func (m *mockRotationBackend) Get(ctx context.Context, bucket, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	obj, ok := m.objects[bucket+"/"+key]
	if !ok {
		return nil, nil, fmt.Errorf("object not found")
	}

	return io.NopCloser(bytes.NewReader(obj.data)), &backend.ObjectInfo{
		Key:      key,
		Size:     int64(len(obj.data)),
		Metadata: obj.metadata,
	}, nil
}

func (m *mockRotationBackend) GetRange(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	obj, ok := m.objects[bucket+"/"+key]
	if !ok {
		return nil, fmt.Errorf("object not found")
	}

	end := offset + length
	if end > int64(len(obj.data)) {
		end = int64(len(obj.data))
	}

	return io.NopCloser(bytes.NewReader(obj.data[offset:end])), nil
}

func (m *mockRotationBackend) Head(ctx context.Context, bucket, key string) (*backend.ObjectInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	obj, ok := m.objects[bucket+"/"+key]
	if !ok {
		return nil, fmt.Errorf("object not found")
	}

	// Parse metadata to check if ARMOR encrypted
	armorMeta, isARMOR := backend.ParseARMORMetadata(obj.metadata)

	return &backend.ObjectInfo{
		Key:              key,
		Size:             int64(len(obj.data)),
		Metadata:         obj.metadata,
		IsARMOREncrypted: isARMOR && armorMeta != nil,
	}, nil
}

func (m *mockRotationBackend) Delete(ctx context.Context, bucket, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.objects, bucket+"/"+key)
	return nil
}

func (m *mockRotationBackend) DeleteObjects(ctx context.Context, bucket string, keys []string) error {
	for _, key := range keys {
		m.Delete(ctx, bucket, key)
	}
	return nil
}

func (m *mockRotationBackend) List(ctx context.Context, bucket, prefix, delimiter, continuationToken string, maxKeys int) (*backend.ListResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var objects []backend.ObjectInfo

	for keyPath, obj := range m.objects {
		// Extract key from bucket/key format
		prefix_ := bucket + "/"
		if len(keyPath) < len(prefix_) || keyPath[:len(prefix_)] != prefix_ {
			continue
		}
		key := keyPath[len(prefix_):]

		// Parse metadata to check if ARMOR encrypted
		armorMeta, isARMOR := backend.ParseARMORMetadata(obj.metadata)

		objects = append(objects, backend.ObjectInfo{
			Key:              key,
			Size:             int64(len(obj.data)),
			Metadata:         obj.metadata,
			IsARMOREncrypted: isARMOR && armorMeta != nil,
		})
	}

	// Sort objects by key for deterministic ordering (S3 returns objects in lexicographic order)
	sortObjectsByKey(objects)

	// Apply maxKeys limit
	if len(objects) > maxKeys {
		objects = objects[:maxKeys]
	}

	return &backend.ListResult{
		Objects:     objects,
		IsTruncated: false,
	}, nil
}

func (m *mockRotationBackend) Copy(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string, meta map[string]string, replaceMetadata bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	srcPath := srcBucket + "/" + srcKey
	obj, ok := m.objects[srcPath]
	if !ok {
		return fmt.Errorf("source object not found")
	}

	dstPath := dstBucket + "/" + dstKey

	// Copy data
	data := make([]byte, len(obj.data))
	copy(data, obj.data)

	// Handle metadata
	newMeta := make(map[string]string)
	if replaceMetadata && meta != nil {
		for k, v := range meta {
			newMeta[k] = v
		}
	} else {
		for k, v := range obj.metadata {
			newMeta[k] = v
		}
	}

	m.objects[dstPath] = &mockObject{
		data:     data,
		metadata: newMeta,
	}
	return nil
}

func (m *mockRotationBackend) ListBuckets(ctx context.Context) ([]backend.BucketInfo, error) {
	return []backend.BucketInfo{{Name: "test-bucket"}}, nil
}

func (m *mockRotationBackend) CreateBucket(ctx context.Context, bucket string) error {
	return nil
}

func (m *mockRotationBackend) DeleteBucket(ctx context.Context, bucket string) error {
	return nil
}

func (m *mockRotationBackend) HeadBucket(ctx context.Context, bucket string) error {
	return nil
}

func (m *mockRotationBackend) GetDirect(ctx context.Context, bucket, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	return m.Get(ctx, bucket, key)
}

func (m *mockRotationBackend) CreateMultipartUpload(ctx context.Context, bucket, key string, meta map[string]string) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (m *mockRotationBackend) UploadPart(ctx context.Context, bucket, key, uploadID string, partNumber int32, body io.Reader, size int64) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (m *mockRotationBackend) CompleteMultipartUpload(ctx context.Context, bucket, key, uploadID string, parts []backend.CompletedPart) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (m *mockRotationBackend) AbortMultipartUpload(ctx context.Context, bucket, key, uploadID string) error {
	return fmt.Errorf("not implemented")
}

func (m *mockRotationBackend) ListParts(ctx context.Context, bucket, key, uploadID string) (*backend.ListPartsResult, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockRotationBackend) ListMultipartUploads(ctx context.Context, bucket string) (*backend.ListMultipartUploadsResult, error) {
	return nil, fmt.Errorf("not implemented")
}

// Lifecycle configuration methods (stub implementations for testing)
func (m *mockRotationBackend) GetBucketLifecycleConfiguration(ctx context.Context, bucket string) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockRotationBackend) PutBucketLifecycleConfiguration(ctx context.Context, bucket string, config []byte) error {
	return fmt.Errorf("not implemented")
}

func (m *mockRotationBackend) DeleteBucketLifecycleConfiguration(ctx context.Context, bucket string) error {
	return fmt.Errorf("not implemented")
}

// createTestARMORObject creates a mock ARMOR-encrypted object for testing.
func createTestARMORObject(mek []byte, bucket, key string, plaintext []byte) (map[string]string, error) {
	// Generate DEK and IV
	dek, err := crypto.GenerateDEK()
	if err != nil {
		return nil, err
	}
	iv, err := crypto.GenerateIV()
	if err != nil {
		return nil, err
	}

	// Wrap DEK with MEK
	wrappedDEK, err := crypto.WrapDEK(mek, dek)
	if err != nil {
		return nil, err
	}

	// Create metadata
	meta := map[string]string{
		"x-amz-meta-armor-version":          "1",
		"x-amz-meta-armor-block-size":       "65536",
		"x-amz-meta-armor-plaintext-size":   fmt.Sprintf("%d", len(plaintext)),
		"x-amz-meta-armor-content-type":     "application/octet-stream",
		"x-amz-meta-armor-iv":               base64.StdEncoding.EncodeToString(iv),
		"x-amz-meta-armor-wrapped-dek":      base64.StdEncoding.EncodeToString(wrappedDEK),
		"x-amz-meta-armor-plaintext-sha256": "test-sha256",
		"x-amz-meta-armor-etag":             "test-etag",
	}

	return meta, nil
}

// TestKeyRotation tests the basic key rotation functionality.
func TestKeyRotation(t *testing.T) {
	// Generate old and new MEKs
	oldMEK := make([]byte, 32)
	newMEK := make([]byte, 32)
	rand.Read(oldMEK)
	rand.Read(newMEK)

	// Create mock backend
	mock := newMockRotationBackend()
	bucket := "test-bucket"

	// Create a few ARMOR-encrypted objects
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("data/file%d.txt", i)
		plaintext := []byte(fmt.Sprintf("This is test data %d", i))
		meta, err := createTestARMORObject(oldMEK, bucket, key, plaintext)
		if err != nil {
			t.Fatalf("failed to create test object: %v", err)
		}

		// Store object in mock backend
		var buf bytes.Buffer
		buf.Write(plaintext)
		mock.Put(context.Background(), bucket, key, &buf, int64(len(plaintext)), meta)
	}

	// Create key rotator
	rotator := NewKeyRotator(mock, bucket, oldMEK, newMEK)

	// Perform rotation
	result, err := rotator.Rotate(context.Background())
	if err != nil {
		t.Fatalf("rotation failed: %v", err)
	}

	// Verify results
	if result.Status != "completed" {
		t.Errorf("expected status 'completed', got '%s'", result.Status)
	}

	if result.ProcessedObjects != 5 {
		t.Errorf("expected 5 processed objects, got %d", result.ProcessedObjects)
	}

	// Verify that objects now have DEKs wrapped with new MEK
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("data/file%d.txt", i)
		info, err := mock.Head(context.Background(), bucket, key)
		if err != nil {
			t.Fatalf("failed to get object %s: %v", key, err)
		}

		armorMeta, ok := backend.ParseARMORMetadata(info.Metadata)
		if !ok {
			t.Fatalf("object %s is not ARMOR-encrypted", key)
		}

		// Verify DEK can be unwrapped with new MEK
		dek, err := crypto.UnwrapDEK(newMEK, armorMeta.WrappedDEK)
		if err != nil {
			t.Fatalf("failed to unwrap DEK with new MEK for %s: %v", key, err)
		}

		if len(dek) != 32 {
			t.Errorf("invalid DEK length for %s: %d", key, len(dek))
		}

		// Verify DEK cannot be unwrapped with old MEK
		_, err = crypto.UnwrapDEK(oldMEK, armorMeta.WrappedDEK)
		if err == nil {
			t.Errorf("DEK for %s should NOT be unwrappable with old MEK", key)
		}
	}
}

// TestKeyRotationResumption tests that rotation state is tracked.
func TestKeyRotationResumption(t *testing.T) {
	// Generate old and new MEKs
	oldMEK := make([]byte, 32)
	newMEK := make([]byte, 32)
	rand.Read(oldMEK)
	rand.Read(newMEK)

	// Create mock backend
	mock := newMockRotationBackend()
	bucket := "test-bucket"

	// Create ARMOR-encrypted objects
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("data/file%d.txt", i)
		plaintext := []byte(fmt.Sprintf("This is test data %d", i))
		meta, err := createTestARMORObject(oldMEK, bucket, key, plaintext)
		if err != nil {
			t.Fatalf("failed to create test object: %v", err)
		}

		var buf bytes.Buffer
		buf.Write(plaintext)
		mock.Put(context.Background(), bucket, key, &buf, int64(len(plaintext)), meta)
	}

	// Create key rotator
	rotator := NewKeyRotator(mock, bucket, oldMEK, newMEK)

	// Perform rotation
	result, err := rotator.Rotate(context.Background())
	if err != nil {
		t.Fatalf("rotation failed: %v", err)
	}

	// Verify completion
	if result.Status != "completed" {
		t.Errorf("expected status 'completed', got '%s'", result.Status)
	}

	// Verify rotation state was saved
	state := rotator.GetState()
	if state == nil {
		t.Fatal("rotation state is nil")
	}
	if state.Status != "completed" {
		t.Errorf("state status is not 'completed', got '%s'", state.Status)
	}
}

// TestKeyRotationStatePersistence tests that rotation state is correctly persisted.
func TestKeyRotationStatePersistence(t *testing.T) {
	oldMEK := make([]byte, 32)
	newMEK := make([]byte, 32)
	rand.Read(oldMEK)
	rand.Read(newMEK)

	mock := newMockRotationBackend()
	bucket := "test-bucket"

	// Create an object
	key := "data/test.txt"
	plaintext := []byte("test data")
	meta, err := createTestARMORObject(oldMEK, bucket, key, plaintext)
	if err != nil {
		t.Fatalf("failed to create test object: %v", err)
	}

	var buf bytes.Buffer
	buf.Write(plaintext)
	mock.Put(context.Background(), bucket, key, &buf, int64(len(plaintext)), meta)

	// Create rotator and rotate
	rotator := NewKeyRotator(mock, bucket, oldMEK, newMEK)
	result, err := rotator.Rotate(context.Background())
	if err != nil {
		t.Fatalf("rotation failed: %v", err)
	}

	if result.Status != "completed" {
		t.Errorf("expected status 'completed', got '%s'", result.Status)
	}

	// Read the state file from the mock backend
	reader, _, err := mock.GetDirect(context.Background(), bucket, ".armor/rotation-state.json")
	if err != nil {
		t.Fatalf("failed to get rotation state: %v", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to read rotation state: %v", err)
	}

	var state RotationState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("failed to parse rotation state: %v", err)
	}

	if state.Status != "completed" {
		t.Errorf("persisted state status is not 'completed', got '%s'", state.Status)
	}

	if state.ProcessedObjects != 1 {
		t.Errorf("expected 1 processed object in state, got %d", state.ProcessedObjects)
	}
}

// TestKeyRotationSkipsNonARMORObjects tests that rotation skips non-ARMOR objects.
func TestKeyRotationSkipsNonARMORObjects(t *testing.T) {
	oldMEK := make([]byte, 32)
	newMEK := make([]byte, 32)
	rand.Read(oldMEK)
	rand.Read(newMEK)

	mock := newMockRotationBackend()
	bucket := "test-bucket"

	// Create an ARMOR-encrypted object
	armorKey := "data/encrypted.txt"
	armorPlaintext := []byte("encrypted data")
	armorMeta, err := createTestARMORObject(oldMEK, bucket, armorKey, armorPlaintext)
	if err != nil {
		t.Fatalf("failed to create ARMOR object: %v", err)
	}
	var armorBuf bytes.Buffer
	armorBuf.Write(armorPlaintext)
	mock.Put(context.Background(), bucket, armorKey, &armorBuf, int64(len(armorPlaintext)), armorMeta)

	// Create a non-ARMOR object (no armor metadata)
	plainKey := "data/plain.txt"
	plainMeta := map[string]string{
		"Content-Type": "text/plain",
	}
	plainData := []byte("plain data")
	var plainBuf bytes.Buffer
	plainBuf.Write(plainData)
	mock.Put(context.Background(), bucket, plainKey, &plainBuf, int64(len(plainData)), plainMeta)

	// Rotate
	rotator := NewKeyRotator(mock, bucket, oldMEK, newMEK)
	result, err := rotator.Rotate(context.Background())
	if err != nil {
		t.Fatalf("rotation failed: %v", err)
	}

	// Verify only ARMOR object was processed
	if result.ProcessedObjects != 1 {
		t.Errorf("expected 1 processed object, got %d", result.ProcessedObjects)
	}

	// We skip 2 objects: the non-ARMOR object and the rotation state file (.armor/rotation-state.json)
	if result.SkippedObjects != 2 {
		t.Errorf("expected 2 skipped objects (non-ARMOR + rotation state), got %d", result.SkippedObjects)
	}

	// Verify ARMOR object was rotated
	info, err := mock.Head(context.Background(), bucket, armorKey)
	if err != nil {
		t.Fatalf("failed to get ARMOR object: %v", err)
	}
	armorMetaParsed, _ := backend.ParseARMORMetadata(info.Metadata)
	_, err = crypto.UnwrapDEK(newMEK, armorMetaParsed.WrappedDEK)
	if err != nil {
		t.Errorf("ARMOR object DEK not rotated: %v", err)
	}

	// Verify non-ARMOR object was not modified
	plainInfo, err := mock.Head(context.Background(), bucket, plainKey)
	if err != nil {
		t.Fatalf("failed to get plain object: %v", err)
	}
	if plainInfo.Metadata["Content-Type"] != "text/plain" {
		t.Error("plain object metadata was modified")
	}
}

// TestKeyRotationSkipsInternalObjects tests that rotation skips .armor/ prefixed objects.
func TestKeyRotationSkipsInternalObjects(t *testing.T) {
	oldMEK := make([]byte, 32)
	newMEK := make([]byte, 32)
	rand.Read(oldMEK)
	rand.Read(newMEK)

	mock := newMockRotationBackend()
	bucket := "test-bucket"

	// Create a regular ARMOR-encrypted object
	regularKey := "data/encrypted.txt"
	regularPlaintext := []byte("encrypted data")
	regularMeta, err := createTestARMORObject(oldMEK, bucket, regularKey, regularPlaintext)
	if err != nil {
		t.Fatalf("failed to create ARMOR object: %v", err)
	}
	var regularBuf bytes.Buffer
	regularBuf.Write(regularPlaintext)
	mock.Put(context.Background(), bucket, regularKey, &regularBuf, int64(len(regularPlaintext)), regularMeta)

	// Create an internal ARMOR object (under .armor/)
	internalKey := ".armor/canary/instance-1"
	internalMeta := map[string]string{
		"x-amz-meta-armor-version": "1",
	}
	var internalBuf bytes.Buffer
	internalBuf.Write([]byte("internal data"))
	mock.Put(context.Background(), bucket, internalKey, &internalBuf, 13, internalMeta)

	// Rotate
	rotator := NewKeyRotator(mock, bucket, oldMEK, newMEK)
	result, err := rotator.Rotate(context.Background())
	if err != nil {
		t.Fatalf("rotation failed: %v", err)
	}

	// Verify only regular object was processed
	if result.ProcessedObjects != 1 {
		t.Errorf("expected 1 processed object, got %d", result.ProcessedObjects)
	}
}
