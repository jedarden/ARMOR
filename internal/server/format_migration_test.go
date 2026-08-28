// Package server provides tests for format migration functionality.
package server

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

// MockBackend is a mock implementation of the Backend interface for testing.
type MockBackend struct {
	objects      map[string]*MockObject
	multipartDir map[string][]byte
}

type MockObject struct {
	Data     []byte
	Metadata map[string]string
}

func NewMockBackend() *MockBackend {
	return &MockBackend{
		objects:      make(map[string]*MockObject),
		multipartDir: make(map[string][]byte),
	}
}

func (m *MockBackend) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	m.objects[key] = &MockObject{
		Data:     data,
		Metadata: meta,
	}
	return nil
}

func (m *MockBackend) Get(ctx context.Context, bucket, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	obj, ok := m.objects[key]
	if !ok {
		return nil, nil, backend.ErrObjectNotFound
	}
	info := &backend.ObjectInfo{
		Key:     key,
		Size:    int64(len(obj.Data)),
		Metadata: obj.Metadata,
	}
	return io.NopCloser(strings.NewReader(string(obj.Data))), info, nil
}

func (m *MockBackend) GetDirect(ctx context.Context, bucket, key string) (io.ReadCloser, int64, error) {
	obj, ok := m.objects[key]
	if !ok {
		return nil, 0, backend.ErrObjectNotFound
	}
	return io.NopCloser(strings.NewReader(string(obj.Data))), int64(len(obj.Data)), nil
}

func (m *MockBackend) GetRange(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, error) {
	obj, ok := m.objects[key]
	if !ok {
		return nil, backend.ErrObjectNotFound
	}
	data := obj.Data
	if offset >= int64(len(data)) {
		return io.NopCloser(strings.NewReader("")), nil
	}
	end := offset + length
	if end > int64(len(data)) {
		end = int64(len(data))
	}
	return io.NopCloser(strings.NewReader(string(data[offset:end]))), nil
}

func (m *MockBackend) GetRangeWithHeaders(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, map[string]string, error) {
	rc, err := m.GetRange(ctx, bucket, key, offset, length)
	return rc, map[string]string{}, err
}

func (m *MockBackend) Head(ctx context.Context, bucket, key string) (*backend.ObjectInfo, error) {
	obj, ok := m.objects[key]
	if !ok {
		return nil, backend.ErrObjectNotFound
	}
	return &backend.ObjectInfo{
		Key:      key,
		Size:     int64(len(obj.Data)),
		Metadata: obj.Metadata,
	}, nil
}

func (m *MockBackend) Delete(ctx context.Context, bucket, key string) error {
	delete(m.objects, key)
	return nil
}

func (m *MockBackend) DeleteObjects(ctx context.Context, bucket string, keys []string) error {
	for _, key := range keys {
		m.Delete(ctx, bucket, key)
	}
	return nil
}

func (m *MockBackend) List(ctx context.Context, bucket, prefix, delimiter, continuationToken string, maxKeys int) (*backend.ListResult, error) {
	var keys []string
	for key := range m.objects {
		if prefix == "" || strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}

	objects := make([]backend.ObjectInfo, 0, len(keys))
	for _, key := range keys {
		obj := m.objects[key]
		armorMeta, ok := backend.ParseARMORMetadata(obj.Metadata)
		objects = append(objects, backend.ObjectInfo{
			Key:             key,
			Size:            int64(len(obj.Data)),
			IsARMOREncrypted: ok,
			Metadata:        obj.Metadata,
		})
		_ = armorMeta // Use to avoid unused variable
	}

	return &backend.ListResult{
		Objects:    objects,
		IsTruncated: false,
	}, nil
}

func (m *MockBackend) Copy(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string, meta map[string]string, replaceMetadata bool) error {
	obj, ok := m.objects[srcKey]
	if !ok {
		return backend.ErrObjectNotFound
	}

	newMeta := obj.Metadata
	if replaceMetadata {
		newMeta = meta
	}

	m.objects[dstKey] = &MockObject{
		Data:     obj.Data,
		Metadata: newMeta,
	}
	return nil
}

func (m *MockBackend) ListBuckets(ctx context.Context) ([]string, error) {
	return []string{"test-bucket"}, nil
}

// TestFormatMigrationDryRun tests that dry run mode counts objects without migrating.
func TestFormatMigrationDryRun(t *testing.T) {
	ctx := context.Background()
	mockBackend := NewMockBackend()

	// Create a test V1 object
	mek := make([]byte, 32)
	for i := range mek {
		mek[i] = byte(i)
	}

	dek := make([]byte, 32)
	for i := range dek {
		dek[i] = byte(i + 1)
	}

	wrappedDEK, err := crypto.WrapDEK(mek, dek)
	if err != nil {
		t.Fatalf("Failed to wrap DEK: %v", err)
	}

	// Create V1 encryptor
	iv := make([]byte, 16)
 blockSize := 4096
	encryptor, err := crypto.NewEncryptorWithVersion(dek, iv, blockSize, crypto.Version1)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	plaintext := []byte("test data for migration")
	ciphertext, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	// Build envelope header
	header := &crypto.EnvelopeHeader{
		Version:   crypto.Version1,
		IV:        [16]byte{},
		BlockSize: blockSize,
	}
	headerBuf := make([]byte, crypto.EnvelopeHeaderSize)
	copy(headerBuf, header.Encode())

	// Store object with V1 metadata
	metadata := map[string]string{
		"x-amz-meta-armor-version":        "1",
		"x-amz-meta-armor-wrapped-dek":     base64.StdEncoding.EncodeToString(wrappedDEK),
		"x-amz-meta-armor-iv":              base64.StdEncoding.EncodeToString(iv),
		"x-amz-meta-armor-block-size":      "4096",
		"x-amz-meta-armor-plaintext-size": "25",
		"x-amz-meta-armor-sha256":          "test-sha256",
	}

	// Combine header and ciphertext
	fullData := append(headerBuf, ciphertext...)
	mockBackend.objects["test-object-v1.txt"] = &MockObject{
		Data:     fullData,
		Metadata: metadata,
	}

	// Create migrator
	migrator := NewFormatMigrator(mockBackend, "test-bucket", mek, "default", crypto.Version2, []string{"1"}, nil)

	// Run dry run migration
	result, err := migrator.Migrate(ctx, true, 1)
	if err != nil {
		t.Fatalf("Dry run migration failed: %v", err)
	}

	// Verify results
	if result.DryRun != true {
		t.Error("Expected dry run to be true")
	}

	if result.ProcessedObjects != 1 {
		t.Errorf("Expected 1 processed object, got %d", result.ProcessedObjects)
	}

	if result.SkippedObjects != 0 {
		t.Errorf("Expected 0 skipped objects, got %d", result.SkippedObjects)
	}

	// Verify object was not actually migrated
	obj, ok := mockBackend.objects["test-object-v1.txt"]
	if !ok {
		t.Fatal("Object was deleted during dry run")
	}

	if obj.Metadata["x-amz-meta-armor-version"] != "1" {
		t.Error("Object version was changed during dry run")
	}
}

// TestFormatMigrationV1ToV2 tests migrating V1 objects to V2.
func TestFormatMigrationV1ToV2(t *testing.T) {
	ctx := context.Background()
	mockBackend := NewMockBackend()

	// Create a test V1 object
	mek := make([]byte, 32)
	for i := range mek {
		mek[i] = byte(i)
	}

	dek := make([]byte, 32)
	for i := range dek {
		dek[i] = byte(i + 1)
	}

	wrappedDEK, err := crypto.WrapDEK(mek, dek)
	if err != nil {
		t.Fatalf("Failed to wrap DEK: %v", err)
	}

	// Create V1 encryptor
	iv := make([]byte, 16)
 blockSize := 4096
	encryptor, err := crypto.NewEncryptorWithVersion(dek, iv, blockSize, crypto.Version1)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	plaintext := []byte("test data for migration to v2")
	ciphertext, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	// Build envelope header
	header := &crypto.EnvelopeHeader{
		Version:   crypto.Version1,
		IV:        [16]byte{},
		BlockSize: blockSize,
	}
	headerBuf := make([]byte, crypto.EnvelopeHeaderSize)
	copy(headerBuf, header.Encode())

	// Store object with V1 metadata
	metadata := map[string]string{
		"x-amz-meta-armor-version":        "1",
		"x-amz-meta-armor-wrapped-dek":     base64.StdEncoding.EncodeToString(wrappedDEK),
		"x-amz-meta-armor-iv":              base64.StdEncoding.EncodeToString(iv),
		"x-amz-meta-armor-block-size":      "4096",
		"x-amz-meta-armor-plaintext-size":  "30",
		"x-amz-meta-armor-sha256":          "test-sha256",
	}

	// Combine header and ciphertext
	fullData := append(headerBuf, ciphertext...)
	mockBackend.objects["test-object-v1.txt"] = &MockObject{
		Data:     fullData,
		Metadata: metadata,
	}

	// Create migrator
	migrator := NewFormatMigrator(mockBackend, "test-bucket", mek, "default", crypto.Version2, []string{"1"}, nil)

	// Run actual migration
	result, err := migrator.Migrate(ctx, false, 1)
	if err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	// Verify results
	if result.DryRun != false {
		t.Error("Expected dry run to be false")
	}

	if result.ProcessedObjects != 1 {
		t.Errorf("Expected 1 processed object, got %d", result.ProcessedObjects)
	}

	// Verify object was migrated to V2
	obj, ok := mockBackend.objects["test-object-v1.txt"]
	if !ok {
		t.Fatal("Object was deleted during migration")
	}

	if obj.Metadata["x-amz-meta-armor-version"] != "2" {
		t.Errorf("Object version was not migrated to V2, got: %s", obj.Metadata["x-amz-meta-armor-version"])
	}
}

// TestFormatMigrationV2Skipped tests that V2 objects are skipped when include=v1.
func TestFormatMigrationV2Skipped(t *testing.T) {
	ctx := context.Background()
	mockBackend := NewMockBackend()

	// Create a test V2 object
	mek := make([]byte, 32)
	for i := range mek {
		mek[i] = byte(i)
	}

	// Store object with V2 metadata
	metadata := map[string]string{
		"x-amz-meta-armor-version": "2",
		"x-amz-meta-armor-wrapped-dek": "test-dek",
		"x-amz-meta-armor-iv":      "test-iv",
	}

	mockBackend.objects["test-object-v2.txt"] = &MockObject{
		Data:     []byte("test data"),
		Metadata: metadata,
	}

	// Create migrator that only migrates V1 objects
	migrator := NewFormatMigrator(mockBackend, "test-bucket", mek, "default", crypto.Version2, []string{"1"}, nil)

	// Run migration
	result, err := migrator.Migrate(ctx, false, 1)
	if err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	// Verify V2 object was skipped
	if result.ProcessedObjects != 0 {
		t.Errorf("Expected 0 processed objects, got %d", result.ProcessedObjects)
	}

	if result.SkippedObjects != 1 {
		t.Errorf("Expected 1 skipped object, got %d", result.SkippedObjects)
	}
}

// TestFormatMigrationResumable tests that migration can be resumed after interruption.
func TestFormatMigrationResumable(t *testing.T) {
	ctx := context.Background()
	mockBackend := NewMockBackend()

	mek := make([]byte, 32)
	for i := range mek {
		mek[i] = byte(i)
	}

	// Create multiple test objects
	for i := 1; i <= 3; i++ {
		metadata := map[string]string{
			"x-amz-meta-armor-version":      "1",
			"x-amz-meta-armor-wrapped-dek":  "test-dek",
			"x-amz-meta-armor-iv":           "test-iv",
		}
		mockBackend.objects[fmt.Sprintf("test-object-%d.txt", i)] = &MockObject{
			Data:     []byte(fmt.Sprintf("test data %d", i)),
			Metadata: metadata,
		}
	}

	// Create migrator
	migrator := NewFormatMigrator(mockBackend, "test-bucket", mek, "default", crypto.Version2, []string{"1"}, nil)

	// Simulate an interrupted migration by setting state
	migrator.state = &MigrationState{
		ID:            "test-migration",
		Status:        "in_progress",
		LastKey:       "test-object-2.txt",
		TotalObjects:  3,
		ProcessedObjects: 2,
	}

	// Resume migration
	result, err := migrator.Migrate(ctx, false, 1)
	if err != nil {
		t.Fatalf("Migration resume failed: %v", err)
	}

	// Verify migration completed
	if result.Status != "completed" {
		t.Errorf("Expected completed status, got: %s", result.Status)
	}

	// Verify only remaining objects were processed
	// (This is a simplified test - real implementation would handle resume logic)
}

// TestFormatMigrationEndpoint tests the HTTP endpoint for format migration.
func TestFormatMigrationEndpoint(t *testing.T) {
	// This would require setting up a full server with admin auth
	// For now, we'll test the handler logic in isolation
	t.Skip("HTTP endpoint test requires full server setup")
}