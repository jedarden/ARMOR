package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
	"github.com/jedarden/armor/internal/manifest"
)

// mockRotationBackend is a mock backend for testing key rotation.
type mockRotationBackend struct {
	mu      sync.Mutex
	objects map[string]*mockObject
}

type mockObject struct {
	data     []byte
	metadata map[string]string
	// fakeSize, when > 0, overrides len(data) as the object's reported Size.
	// Used to simulate oversized objects for the B2 CopyObject ceiling test
	// without storing gigabytes of data.
	fakeSize int64
}

func newMockRotationBackend() *mockRotationBackend {
	return &mockRotationBackend{
		objects: make(map[string]*mockObject),
	}
}

// setFakeSize overrides the Size reported for an object, simulating an
// oversized object for the B2 CopyObject ceiling test.
func (m *mockRotationBackend) setFakeSize(bucket, key string, size int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if obj, ok := m.objects[bucket+"/"+key]; ok {
		obj.fakeSize = size
	}
}

// objectSize returns the reported size for a mock object, honoring fakeSize.
func (m *mockRotationBackend) objectSize(bucket, key string) int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	obj, ok := m.objects[bucket+"/"+key]
	if !ok {
		return 0
	}
	if obj.fakeSize > 0 {
		return obj.fakeSize
	}
	return int64(len(obj.data))
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
		Size:     reportedSize(obj),
		Metadata: obj.metadata,
	}, nil
}

// reportedSize returns the size a mock object should report (fakeSize if set).
func reportedSize(obj *mockObject) int64 {
	if obj.fakeSize > 0 {
		return obj.fakeSize
	}
	return int64(len(obj.data))
}

func (m *mockRotationBackend) GetRange(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, error) {
	body, _, err := m.GetRangeWithHeaders(ctx, bucket, key, offset, length)
	return body, err
}

func (m *mockRotationBackend) GetRangeWithHeaders(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, map[string]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	obj, ok := m.objects[bucket+"/"+key]
	if !ok {
		return nil, nil, fmt.Errorf("object not found")
	}

	end := offset + length
	if end > int64(len(obj.data)) {
		end = int64(len(obj.data))
	}

	// Mock doesn't simulate CF caching, so return empty headers
	return io.NopCloser(bytes.NewReader(obj.data[offset:end])), make(map[string]string), nil
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
		Size:             reportedSize(obj),
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

		// reportedSize honors fakeSize so the B2 CopyObject ceiling test can
		// report an oversized object from List without storing gigabytes.
		objects = append(objects, backend.ObjectInfo{
			Key:              key,
			Size:             reportedSize(obj),
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

// Object Lock methods (stub implementations for testing)
func (m *mockRotationBackend) GetObjectLockConfiguration(ctx context.Context, bucket string) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockRotationBackend) PutObjectLockConfiguration(ctx context.Context, bucket string, config []byte) error {
	return fmt.Errorf("not implemented")
}

func (m *mockRotationBackend) GetObjectRetention(ctx context.Context, bucket, key string) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockRotationBackend) PutObjectRetention(ctx context.Context, bucket, key string, retention []byte) error {
	return fmt.Errorf("not implemented")
}

func (m *mockRotationBackend) GetObjectLegalHold(ctx context.Context, bucket, key string) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockRotationBackend) PutObjectLegalHold(ctx context.Context, bucket, key string, legalHold []byte) error {
	return fmt.Errorf("not implemented")
}

func (m *mockRotationBackend) ListObjectVersions(ctx context.Context, bucket, prefix, delimiter, keyMarker, versionIDMarker string, maxKeys int) (*backend.ListObjectVersionsResult, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockRotationBackend) HeadVersion(ctx context.Context, bucket, key, versionID string) (*backend.ObjectInfo, error) {
	return nil, fmt.Errorf("not implemented")
}

// headCountingRotationBackend wraps mockRotationBackend and counts Head() calls.
type headCountingRotationBackend struct {
	*mockRotationBackend
	headCalls atomic.Int64
}

func (h *headCountingRotationBackend) Head(ctx context.Context, bucket, key string) (*backend.ObjectInfo, error) {
	h.headCalls.Add(1)
	return h.mockRotationBackend.Head(ctx, bucket, key)
}

// TestKeyRotationWithManifestIndex verifies that key rotation uses the in-memory
// manifest to skip per-object HeadObject calls when entries are present.
func TestKeyRotationWithManifestIndex(t *testing.T) {
	oldMEK := make([]byte, 32)
	newMEK := make([]byte, 32)
	rand.Read(oldMEK)
	rand.Read(newMEK)

	base := newMockRotationBackend()
	mock := &headCountingRotationBackend{mockRotationBackend: base}
	bucket := "test-bucket"

	idx := manifest.New()

	// Create objects and populate the manifest index.
	for i := 0; i < 3; i++ {
		key := fmt.Sprintf("data/file%d.parquet", i)
		plaintext := []byte(fmt.Sprintf("test data %d", i))

		meta, err := createTestARMORObject(oldMEK, bucket, key, plaintext)
		if err != nil {
			t.Fatalf("failed to create test object: %v", err)
		}

		var buf bytes.Buffer
		buf.Write(plaintext)
		mock.Put(context.Background(), bucket, key, &buf, int64(len(plaintext)), meta)

		// Parse the metadata to extract IV and wrapped DEK for the manifest.
		am, ok := backend.ParseARMORMetadata(meta)
		if !ok {
			t.Fatal("failed to parse ARMOR metadata")
		}
		idx.Put(bucket, key, &manifest.Entry{
			PlaintextSize: am.PlaintextSize,
			ContentType:   am.ContentType,
			ETag:          am.ETag,
			IV:            am.IV,
			WrappedDEK:    am.WrappedDEK,
			BlockSize:     am.BlockSize,
			LastModified:  time.Now(),
		})
	}

	rotator := NewKeyRotator(mock, bucket, oldMEK, newMEK, idx)
	result, err := rotator.Rotate(context.Background())
	if err != nil {
		t.Fatalf("rotation failed: %v", err)
	}

	if result.Status != "completed" {
		t.Errorf("expected status 'completed', got '%s'", result.Status)
	}
	if result.ProcessedObjects != 3 {
		t.Errorf("expected 3 processed objects, got %d", result.ProcessedObjects)
	}

	// With a fully-populated manifest, no backend Head() calls should have been
	// made during rotation — only List and Copy.
	if n := mock.headCalls.Load(); n != 0 {
		t.Errorf("expected 0 backend Head() calls (manifest fast-path), got %d", n)
	}

	// Verify DEKs were re-wrapped with the new MEK.
	for i := 0; i < 3; i++ {
		key := fmt.Sprintf("data/file%d.parquet", i)
		info, err := mock.Head(context.Background(), bucket, key)
		if err != nil {
			t.Fatalf("failed to head %s: %v", key, err)
		}
		am, ok := backend.ParseARMORMetadata(info.Metadata)
		if !ok {
			t.Fatalf("object %s is not ARMOR-encrypted after rotation", key)
		}
		dek, err := crypto.UnwrapDEK(newMEK, am.WrappedDEK)
		if err != nil {
			t.Fatalf("failed to unwrap DEK with new MEK for %s: %v", key, err)
		}
		if len(dek) != 32 {
			t.Errorf("invalid DEK length for %s: %d", key, len(dek))
		}
	}
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
	rotator := NewKeyRotator(mock, bucket, oldMEK, newMEK, nil)

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
	rotator := NewKeyRotator(mock, bucket, oldMEK, newMEK, nil)

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
	rotator := NewKeyRotator(mock, bucket, oldMEK, newMEK, nil)
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
	rotator := NewKeyRotator(mock, bucket, oldMEK, newMEK, nil)
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
	rotator := NewKeyRotator(mock, bucket, oldMEK, newMEK, nil)
	result, err := rotator.Rotate(context.Background())
	if err != nil {
		t.Fatalf("rotation failed: %v", err)
	}

	// Verify only regular object was processed
	if result.ProcessedObjects != 1 {
		t.Errorf("expected 1 processed object, got %d", result.ProcessedObjects)
	}
}

// multipartSidecarKey mirrors backend.MultipartStateManager.SaveHMACTable: the
// HMAC table for a multipart object lives at .armor/hmac/<sha256(key)>.
func multipartSidecarKey(key string) string {
	return fmt.Sprintf(".armor/hmac/%x", sha256.Sum256([]byte(key)))
}

// createTestMultipartARMORObject writes a multipart-format ARMOR object: it
// carries the x-amz-meta-armor-multipart / -part-size markers and stores its
// block HMAC table in a sidecar at .armor/hmac/<sha256(key)>, exactly as the
// real CompleteMultipartUpload path (handlers.go) does. The object body is raw
// concatenated part ciphertext with no embedded envelope header. It returns the
// sidecar bytes written so a test can assert rotation never touched them.
func createTestMultipartARMORObject(t *testing.T, b *mockRotationBackend, mek []byte, bucket, key string, plaintext []byte, partSize int64) []byte {
	t.Helper()
	dek, err := crypto.GenerateDEK()
	if err != nil {
		t.Fatalf("generate DEK: %v", err)
	}
	iv, err := crypto.GenerateIV()
	if err != nil {
		t.Fatalf("generate IV: %v", err)
	}
	wrappedDEK, err := crypto.WrapDEK(mek, dek)
	if err != nil {
		t.Fatalf("wrap DEK: %v", err)
	}

	plaintextSHA := sha256.Sum256(plaintext)
	meta := map[string]string{
		"x-amz-meta-armor-version":          "1",
		"x-amz-meta-armor-block-size":       "65536",
		"x-amz-meta-armor-plaintext-size":   fmt.Sprintf("%d", len(plaintext)),
		"x-amz-meta-armor-content-type":     "application/octet-stream",
		"x-amz-meta-armor-iv":               base64.StdEncoding.EncodeToString(iv),
		"x-amz-meta-armor-wrapped-dek":      base64.StdEncoding.EncodeToString(wrappedDEK),
		"x-amz-meta-armor-plaintext-sha256": hex.EncodeToString(plaintextSHA[:]),
		"x-amz-meta-armor-etag":             "etag-" + key,
		// The two markers backend.ARMORMetadata.ToMetadata() does NOT emit, and
		// that rotation must therefore preserve from the raw metadata map. A
		// dropped armor-multipart marker is exactly the bf-24sxh7 regression
		// rotation could reintroduce.
		"x-amz-meta-armor-multipart": "true",
		"x-amz-meta-armor-part-size": fmt.Sprintf("%d", partSize),
		// Non-ARMOR user metadata that must also survive the REPLACE copy.
		"x-amz-meta-customer-owner": "finance",
	}

	var buf bytes.Buffer
	buf.Write(plaintext)
	if err := b.Put(context.Background(), bucket, key, &buf, int64(len(plaintext)), meta); err != nil {
		t.Fatalf("put multipart object %s: %v", key, err)
	}

	// HMAC sidecar mirroring backend.MultipartStateManager.SaveHMACTable.
	sidecar := backend.HMACTableSidecar{
		Key:        key,
		BlockSize:  65536,
		BlockHMACs: [][]byte{[]byte("block-hmac-0"), []byte("block-hmac-1")},
	}
	sidecarData, err := json.Marshal(sidecar)
	if err != nil {
		t.Fatalf("marshal sidecar: %v", err)
	}
	var sbuf bytes.Buffer
	sbuf.Write(sidecarData)
	if err := b.Put(context.Background(), bucket, multipartSidecarKey(key), &sbuf, int64(len(sidecarData)), nil); err != nil {
		t.Fatalf("put sidecar for %s: %v", key, err)
	}
	return sidecarData
}

// dekUnwrapsWith reports whether the object's current wrapped DEK unwraps under
// the given MEK using the real AES-KWP (RFC 5649) unwrap.
func dekUnwrapsWith(t *testing.T, b *mockRotationBackend, bucket, key string, mek []byte) bool {
	t.Helper()
	info, err := b.Head(context.Background(), bucket, key)
	if err != nil {
		t.Fatalf("head %s: %v", key, err)
	}
	am, ok := backend.ParseARMORMetadata(info.Metadata)
	if !ok {
		t.Fatalf("%s is not ARMOR-encrypted", key)
	}
	_, err = crypto.UnwrapDEK(mek, am.WrappedDEK)
	return err == nil
}

// objectBody reads an object's full body from the mock backend.
func objectBody(t *testing.T, b *mockRotationBackend, bucket, key string) []byte {
	t.Helper()
	body, _, err := b.Get(context.Background(), bucket, key)
	if err != nil {
		t.Fatalf("get %s: %v", key, err)
	}
	defer body.Close()
	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read %s: %v", key, err)
	}
	return data
}

// TestKeyRotationMixedPrefixPreservesMultipart rotates a prefix containing both
// single-PUT and multipart objects and proves: every object decrypts with the
// NEW MEK and fails with the OLD MEK; plaintext bodies and ETags are unchanged;
// and — critically for multipart objects — the armor-multipart marker,
// armor-part-size, and HMAC sidecar all survive the CopyObject REPLACE, so the
// object round-trips (the read path keys off the multipart marker to find the
// sidecar; dropping it would reintroduce bf-24sxh7).
func TestKeyRotationMixedPrefixPreservesMultipart(t *testing.T) {
	oldMEK := make([]byte, 32)
	newMEK := make([]byte, 32)
	rand.Read(oldMEK)
	rand.Read(newMEK)

	mock := newMockRotationBackend()
	bucket := "test-bucket"

	// Two single-PUT objects.
	singleKeys := []string{"data/single-a.bin", "data/single-b.bin"}
	singlePlaintext := map[string][]byte{}
	for _, k := range singleKeys {
		pt := []byte("single-put payload " + k)
		singlePlaintext[k] = pt
		meta, err := createTestARMORObject(oldMEK, bucket, k, pt)
		if err != nil {
			t.Fatalf("create single object %s: %v", k, err)
		}
		var buf bytes.Buffer
		buf.Write(pt)
		if err := mock.Put(context.Background(), bucket, k, &buf, int64(len(pt)), meta); err != nil {
			t.Fatalf("put %s: %v", k, err)
		}
	}

	// Two multipart objects (payload spans more than one 64 KiB block).
	multipartKeys := []string{"data/multi-a.bin", "data/multi-b.bin"}
	multipartPlaintext := map[string][]byte{}
	sidecarBefore := map[string][]byte{}
	for _, k := range multipartKeys {
		pt := []byte(strings.Repeat("m", 130000))
		multipartPlaintext[k] = pt
		sidecarBefore[k] = createTestMultipartARMORObject(t, mock, oldMEK, bucket, k, pt, 65536)
	}

	allKeys := append(append([]string{}, singleKeys...), multipartKeys...)

	// Snapshot ETag, plaintext-sha, and body before rotation.
	type snapshot struct {
		etag, ptSHA string
		body        []byte
	}
	before := map[string]snapshot{}
	for _, k := range allKeys {
		info, err := mock.Head(context.Background(), bucket, k)
		if err != nil {
			t.Fatalf("head %s: %v", k, err)
		}
		before[k] = snapshot{
			etag:  info.Metadata["x-amz-meta-armor-etag"],
			ptSHA: info.Metadata["x-amz-meta-armor-plaintext-sha256"],
			body:  objectBody(t, mock, bucket, k),
		}
	}

	rotator := NewKeyRotator(mock, bucket, oldMEK, newMEK, nil)
	result, err := rotator.Rotate(context.Background())
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("status = %q, want completed", result.Status)
	}
	if result.ProcessedObjects != len(allKeys) {
		t.Errorf("processed = %d, want %d", result.ProcessedObjects, len(allKeys))
	}

	for _, k := range allKeys {
		// Decrypts with NEW MEK.
		if !dekUnwrapsWith(t, mock, bucket, k, newMEK) {
			t.Errorf("%s: DEK does not unwrap with NEW MEK after rotation", k)
		}
		// Fails with OLD MEK.
		if dekUnwrapsWith(t, mock, bucket, k, oldMEK) {
			t.Errorf("%s: DEK still unwraps with OLD MEK (not re-wrapped)", k)
		}
		// Plaintext body unchanged (CopyObject is server-side; body untouched).
		if got := objectBody(t, mock, bucket, k); !bytes.Equal(got, before[k].body) {
			t.Errorf("%s: plaintext body changed across rotation", k)
		}
		// ETag and plaintext-sha unchanged.
		info, err := mock.Head(context.Background(), bucket, k)
		if err != nil {
			t.Fatalf("head %s after rotation: %v", k, err)
		}
		if got := info.Metadata["x-amz-meta-armor-etag"]; got != before[k].etag {
			t.Errorf("%s: etag changed %q -> %q", k, before[k].etag, got)
		}
		if got := info.Metadata["x-amz-meta-armor-plaintext-sha256"]; got != before[k].ptSHA {
			t.Errorf("%s: plaintext-sha256 changed %q -> %q", k, before[k].ptSHA, got)
		}
	}

	// CRITICAL: rotation must preserve ALL x-amz-meta-armor-* metadata on
	// multipart objects and must not touch the HMAC sidecar. A dropped
	// armor-multipart marker silently makes every rotated multipart object
	// unreadable (the bf-24sxh7 failure mode reintroduced by rotation).
	for _, k := range multipartKeys {
		info, err := mock.Head(context.Background(), bucket, k)
		if err != nil {
			t.Fatalf("head multipart %s: %v", k, err)
		}
		if got := info.Metadata["x-amz-meta-armor-multipart"]; got != "true" {
			t.Errorf("%s: armor-multipart marker = %q after rotation, want \"true\" (bf-24sxh7 regression)", k, got)
		}
		if got := info.Metadata["x-amz-meta-armor-part-size"]; got != "65536" {
			t.Errorf("%s: armor-part-size = %q after rotation, want \"65536\"", k, got)
		}
		if got := info.Metadata["x-amz-meta-armor-content-type"]; got != "application/octet-stream" {
			t.Errorf("%s: content-type changed to %q", k, got)
		}
		// Non-ARMOR user metadata also survives the REPLACE copy.
		if got := info.Metadata["x-amz-meta-customer-owner"]; got != "finance" {
			t.Errorf("%s: non-ARMOR user metadata lost: customer-owner = %q", k, got)
		}

		// HMAC sidecar byte-identical and still parseable.
		sk := multipartSidecarKey(k)
		got := objectBody(t, mock, bucket, sk)
		if !bytes.Equal(got, sidecarBefore[k]) {
			t.Errorf("%s: HMAC sidecar was mutated by rotation (must be untouched)", k)
		}
		var sc backend.HMACTableSidecar
		if err := json.Unmarshal(got, &sc); err != nil {
			t.Errorf("%s: HMAC sidecar no longer parseable after rotation: %v", k, err)
		}
		if sc.Key != k || sc.BlockSize != 65536 {
			t.Errorf("%s: sidecar content drift: key=%q block-size=%d", k, sc.Key, sc.BlockSize)
		}

		// Round-trip: with the marker + wrapped-DEK + sidecar all intact, the
		// object is still readable exactly as the read path (handlers.go:714)
		// dispatches on the multipart marker to LoadHMACTable from this sidecar.
		if !dekUnwrapsWith(t, mock, bucket, k, newMEK) {
			t.Errorf("%s: multipart object not readable with new MEK after rotation", k)
		}
	}
}

// TestKeyRotationPassthroughUnchanged proves non-ARMOR (passthrough) objects are
// skipped entirely: body bytes and every metadata key are byte-identical, and
// no armor metadata is injected.
func TestKeyRotationPassthroughUnchanged(t *testing.T) {
	oldMEK := make([]byte, 32)
	newMEK := make([]byte, 32)
	rand.Read(oldMEK)
	rand.Read(newMEK)

	mock := newMockRotationBackend()
	bucket := "test-bucket"

	// One ARMOR object so rotation actually does work.
	armorKey := "data/encrypted.bin"
	pt := []byte("secret payload")
	meta, err := createTestARMORObject(oldMEK, bucket, armorKey, pt)
	if err != nil {
		t.Fatalf("create armor object: %v", err)
	}
	var ab bytes.Buffer
	ab.Write(pt)
	if err := mock.Put(context.Background(), bucket, armorKey, &ab, int64(len(pt)), meta); err != nil {
		t.Fatalf("put armor object: %v", err)
	}

	// Non-ARMOR passthrough object with arbitrary metadata.
	plainKey := "uploads/photo.jpg"
	plainMeta := map[string]string{
		"Content-Type":        "image/jpeg",
		"x-amz-meta-uploader": "alice",
	}
	plainBody := []byte("raw-jpeg-bytes-not-armor")
	var pb bytes.Buffer
	pb.Write(plainBody)
	if err := mock.Put(context.Background(), bucket, plainKey, &pb, int64(len(plainBody)), plainMeta); err != nil {
		t.Fatalf("put passthrough object: %v", err)
	}

	rotator := NewKeyRotator(mock, bucket, oldMEK, newMEK, nil)
	result, err := rotator.Rotate(context.Background())
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if result.ProcessedObjects != 1 {
		t.Errorf("processed = %d, want 1 (only the ARMOR object)", result.ProcessedObjects)
	}

	// Passthrough object untouched.
	info, err := mock.Head(context.Background(), bucket, plainKey)
	if err != nil {
		t.Fatalf("head passthrough: %v", err)
	}
	if got := info.Metadata["Content-Type"]; got != "image/jpeg" {
		t.Errorf("passthrough Content-Type = %q, want image/jpeg", got)
	}
	if got := info.Metadata["x-amz-meta-uploader"]; got != "alice" {
		t.Errorf("passthrough uploader metadata = %q, want alice", got)
	}
	if got := objectBody(t, mock, bucket, plainKey); !bytes.Equal(got, plainBody) {
		t.Errorf("passthrough body mutated by rotation")
	}
	if _, injected := info.Metadata["x-amz-meta-armor-version"]; injected {
		t.Error("rotation injected armor metadata into a passthrough object")
	}

	// ARMOR object was rotated.
	if !dekUnwrapsWith(t, mock, bucket, armorKey, newMEK) {
		t.Error("armor object not rotated to new MEK")
	}
}

// errSimulatedCrash is the sentinel panicked by crashBackend to model a pod
// SIGKILL mid-rotation. It is recovered by the test harness.
var errSimulatedCrash = errors.New("simulated crash mid-rotation")

// crashBackend wraps mockRotationBackend and panics exactly once, on the
// crashAfter-th Copy call, BEFORE delegating to the underlying Copy. This
// faithfully models a pod being killed mid-rotation: the rotator goroutine dies
// without running any graceful cancel/fail state-save, so the on-disk rotation
// state is whatever the periodic save (every 100 objects) last wrote — status
// "in_progress", the only status initOrLoadState will resume from.
type crashBackend struct {
	*mockRotationBackend
	copyCount  atomic.Int64
	crashAfter int64
	crashed    atomic.Bool
}

func (c *crashBackend) Copy(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string, meta map[string]string, replaceMetadata bool) error {
	n := c.copyCount.Add(1)
	if n == c.crashAfter && c.crashed.CompareAndSwap(false, true) {
		panic(errSimulatedCrash)
	}
	return c.mockRotationBackend.Copy(ctx, srcBucket, srcKey, dstBucket, dstKey, meta, replaceMetadata)
}

// TestKeyRotationInterruptedResume kills a rotation mid-flight (after the
// periodic save has written an in_progress state), then re-runs rotation and
// proves it resumes to idempotent completion with no object left old-wrapped.
func TestKeyRotationInterruptedResume(t *testing.T) {
	oldMEK := make([]byte, 32)
	rand.Read(oldMEK)
	newMEK := make([]byte, 32)
	rand.Read(newMEK)

	const total = 120
	mock := newMockRotationBackend()
	bucket := "test-bucket"
	keys := make([]string, total)
	for i := 0; i < total; i++ {
		keys[i] = fmt.Sprintf("data/file-%03d.bin", i)
		pt := []byte(fmt.Sprintf("payload-%03d", i))
		meta, err := createTestARMORObject(oldMEK, bucket, keys[i], pt)
		if err != nil {
			t.Fatalf("create %s: %v", keys[i], err)
		}
		var buf bytes.Buffer
		buf.Write(pt)
		if err := mock.Put(context.Background(), bucket, keys[i], &buf, int64(len(pt)), meta); err != nil {
			t.Fatalf("put %s: %v", keys[i], err)
		}
	}

	// Crash on the 101st Copy — immediately after the periodic save at the
	// 100th object wrote an in_progress state with LastKey = keys[99].
	cb := &crashBackend{mockRotationBackend: mock, crashAfter: 101}

	// First run: dies mid-rotation.
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected simulated crash on first rotation, but it completed")
			}
		}()
		rotator := NewKeyRotator(cb, bucket, oldMEK, newMEK, nil)
		_, _ = rotator.Rotate(context.Background())
	}()

	// Persisted state is the periodic save: in_progress, LastKey = keys[99].
	reader, _, err := mock.GetDirect(context.Background(), bucket, ".armor/rotation-state.json")
	if err != nil {
		t.Fatalf("no rotation state found after crash: %v", err)
	}
	sd, err := io.ReadAll(reader)
	reader.Close()
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	var state RotationState
	if err := json.Unmarshal(sd, &state); err != nil {
		t.Fatalf("parse state: %v", err)
	}
	if state.Status != "in_progress" {
		t.Fatalf("persisted status = %q, want \"in_progress\" (resume only resumes in_progress)", state.Status)
	}
	if state.LastKey != keys[99] {
		t.Errorf("persisted LastKey = %q, want %q", state.LastKey, keys[99])
	}

	// First 100 objects rotated (new MEK); the rest still old-wrapped.
	for i := 0; i < 100; i++ {
		if !dekUnwrapsWith(t, mock, bucket, keys[i], newMEK) {
			t.Errorf("pre-resume: %s not new-wrapped", keys[i])
		}
	}
	for i := 100; i < total; i++ {
		if dekUnwrapsWith(t, mock, bucket, keys[i], newMEK) {
			t.Errorf("pre-resume: %s unexpectedly new-wrapped (crash should leave it old)", keys[i])
		}
		if !dekUnwrapsWith(t, mock, bucket, keys[i], oldMEK) {
			t.Errorf("pre-resume: %s lost its old-wrapped DEK", keys[i])
		}
	}

	// Second run: resume and complete.
	rotator2 := NewKeyRotator(cb, bucket, oldMEK, newMEK, nil)
	result, err := rotator2.Rotate(context.Background())
	if err != nil {
		t.Fatalf("resume rotation: %v", err)
	}
	if result.Status != "completed" {
		t.Errorf("resume status = %q, want completed", result.Status)
	}

	// Idempotent completion: every object decrypts with the NEW MEK and NONE
	// with the OLD MEK — no object left old-wrapped.
	for i := 0; i < total; i++ {
		if !dekUnwrapsWith(t, mock, bucket, keys[i], newMEK) {
			t.Errorf("post-resume: %s not new-wrapped (left old-wrapped!)", keys[i])
		}
		if dekUnwrapsWith(t, mock, bucket, keys[i], oldMEK) {
			t.Errorf("post-resume: %s still unwraps with OLD MEK", keys[i])
		}
	}

	// Final state on disk is completed.
	finalState := rotator2.GetState()
	if finalState == nil || finalState.Status != "completed" {
		t.Errorf("final state = %+v, want status completed", finalState)
	}
}

// TestRotateObjectRejectsOversizedWithTypedError asserts rotateObject returns
// the typed ErrCopyObjectTooLarge (not an opaque CopyObject failure and not a
// silent skip) for objects above the B2 CopyObject size ceiling.
func TestRotateObjectRejectsOversizedWithTypedError(t *testing.T) {
	oldMEK := make([]byte, 32)
	rand.Read(oldMEK)
	newMEK := make([]byte, 32)
	rand.Read(newMEK)

	mock := newMockRotationBackend()
	bucket := "test-bucket"

	bigKey := "data/too-big.bin"
	pt := []byte("payload")
	meta, err := createTestARMORObject(oldMEK, bucket, bigKey, pt)
	if err != nil {
		t.Fatalf("create oversized object: %v", err)
	}
	var buf bytes.Buffer
	buf.Write(pt)
	if err := mock.Put(context.Background(), bucket, bigKey, &buf, int64(len(pt)), meta); err != nil {
		t.Fatalf("put oversized object: %v", err)
	}

	rotator := NewKeyRotator(mock, bucket, oldMEK, newMEK, nil)

	// As List would report it: above the 5 GiB ceiling.
	big := backend.ObjectInfo{
		Key:              bigKey,
		Size:             B2CopyObjectSizeCeiling + 1,
		Metadata:         meta,
		IsARMOREncrypted: true,
	}
	err = rotator.rotateObject(context.Background(), big)
	if !errors.Is(err, ErrCopyObjectTooLarge) {
		t.Errorf("rotateObject oversized: err = %v, want ErrCopyObjectTooLarge", err)
	}

	// Exactly at the ceiling is still copyable (boundary is exclusive >).
	atCeiling := backend.ObjectInfo{
		Key:              bigKey,
		Size:             B2CopyObjectSizeCeiling,
		Metadata:         meta,
		IsARMOREncrypted: true,
	}
	if err := rotator.rotateObject(context.Background(), atCeiling); err != nil {
		t.Errorf("rotateObject at ceiling: unexpected err = %v", err)
	}
}

// TestKeyRotationB2CopyObjectCeiling proves oversized ARMOR objects above the
// B2 CopyObject size ceiling are enumerated as EXCEPTIONS (with their keys in
// RotationResult.ExceptionKeys) and left old-wrapped, never silently skipped —
// while normal objects rotate normally.
func TestKeyRotationB2CopyObjectCeiling(t *testing.T) {
	oldMEK := make([]byte, 32)
	rand.Read(oldMEK)
	newMEK := make([]byte, 32)
	rand.Read(newMEK)

	mock := newMockRotationBackend()
	bucket := "test-bucket"

	// Normal ARMOR object — gets rotated.
	smallKey := "data/small.bin"
	pt := []byte("small payload")
	meta, err := createTestARMORObject(oldMEK, bucket, smallKey, pt)
	if err != nil {
		t.Fatalf("create small object: %v", err)
	}
	var sb bytes.Buffer
	sb.Write(pt)
	if err := mock.Put(context.Background(), bucket, smallKey, &sb, int64(len(pt)), meta); err != nil {
		t.Fatalf("put small object: %v", err)
	}

	// Oversized ARMOR object — above the 5 GiB CopyObject ceiling. fakeSize
	// lets List report it as oversized without storing 5 GiB of data.
	bigKey := "data/too-big.bin"
	bigPt := []byte("big payload")
	bigMeta, err := createTestARMORObject(oldMEK, bucket, bigKey, bigPt)
	if err != nil {
		t.Fatalf("create big object: %v", err)
	}
	var bb bytes.Buffer
	bb.Write(bigPt)
	if err := mock.Put(context.Background(), bucket, bigKey, &bb, int64(len(bigPt)), bigMeta); err != nil {
		t.Fatalf("put big object: %v", err)
	}
	mock.setFakeSize(bucket, bigKey, B2CopyObjectSizeCeiling+1)

	rotator := NewKeyRotator(mock, bucket, oldMEK, newMEK, nil)
	result, err := rotator.Rotate(context.Background())
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}

	// The normal object was rotated; the oversized one was enumerated as an
	// exception — NOT counted in ProcessedObjects and NOT silently skipped.
	if result.ProcessedObjects != 1 {
		t.Errorf("processed = %d, want 1 (only the small object)", result.ProcessedObjects)
	}
	if result.Exceptions != 1 {
		t.Errorf("exceptions = %d, want 1 (oversized object must be enumerated, not skipped)", result.Exceptions)
	}
	if len(result.ExceptionKeys) != 1 || result.ExceptionKeys[0] != bigKey {
		t.Errorf("exception keys = %v, want [%s]", result.ExceptionKeys, bigKey)
	}

	// The oversized object is left old-wrapped: rotation could not re-wrap it
	// via CopyObject, so an operator must re-wrap it with a multipart copy.
	if dekUnwrapsWith(t, mock, bucket, bigKey, newMEK) {
		t.Error("oversized object was re-wrapped with new MEK (should remain old-wrapped)")
	}
	if !dekUnwrapsWith(t, mock, bucket, bigKey, oldMEK) {
		t.Error("oversized object lost its old-wrapped DEK")
	}

	// The normal object was rotated.
	if !dekUnwrapsWith(t, mock, bucket, smallKey, newMEK) {
		t.Error("small object not rotated to new MEK")
	}
}
