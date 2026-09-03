// Package server provides tests for format migration functionality.
package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"testing"
	"time"

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
		Key:      key,
		Size:     int64(len(obj.Data)),
		Metadata: obj.Metadata,
	}
	return io.NopCloser(strings.NewReader(string(obj.Data))), info, nil
}

func (m *MockBackend) GetDirect(ctx context.Context, bucket, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	obj, ok := m.objects[key]
	if !ok {
		return nil, nil, backend.ErrObjectNotFound
	}
	info := &backend.ObjectInfo{Key: key, Size: int64(len(obj.Data)), Metadata: obj.Metadata}
	return io.NopCloser(strings.NewReader(string(obj.Data))), info, nil
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
		// Skip internal ARMOR objects (.armor/ directory)
		if len(key) >= 7 && key[:7] == ".armor/" {
			continue
		}
		if prefix == "" || strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}
	// Real backends (S3, B2) list keys in lexicographic order, and the
	// migration cursor assumes it (`key <= LastKey` skip check). Return
	// sorted keys so resumption semantics match production.
	sort.Strings(keys)

	objects := make([]backend.ObjectInfo, 0, len(keys))
	for _, key := range keys {
		obj := m.objects[key]
		armorMeta, ok := backend.ParseARMORMetadata(obj.Metadata)
		objects = append(objects, backend.ObjectInfo{
			Key:              key,
			Size:             int64(len(obj.Data)),
			IsARMOREncrypted: ok,
			Metadata:         obj.Metadata,
		})
		_ = armorMeta // Use to avoid unused variable
	}

	return &backend.ListResult{
		Objects:     objects,
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

func (m *MockBackend) ListBuckets(ctx context.Context) ([]backend.BucketInfo, error) {
	return []backend.BucketInfo{{Name: "test-bucket"}}, nil
}

func (m *MockBackend) ListRaw(ctx context.Context, bucket, prefix, delimiter, continuationToken string, maxKeys int) (*backend.ListResult, error) {
	return m.List(ctx, bucket, prefix, delimiter, continuationToken, maxKeys)
}

func (m *MockBackend) CreateBucket(ctx context.Context, bucket string) error { return nil }
func (m *MockBackend) DeleteBucket(ctx context.Context, bucket string) error { return nil }
func (m *MockBackend) HeadBucket(ctx context.Context, bucket string) error   { return nil }

// The stubs below satisfy backend.Backend for interface conformance --
// format migration tests don't exercise multipart uploads, lifecycle,
// object lock, retention, legal hold, or versioning, so these return zero
// values rather than simulating real behavior.

func (m *MockBackend) CreateMultipartUpload(ctx context.Context, bucket, key string, meta map[string]string) (string, error) {
	return "", nil
}

func (m *MockBackend) UploadPart(ctx context.Context, bucket, key, uploadID string, partNumber int32, body io.Reader, size int64) (string, error) {
	return "", nil
}

func (m *MockBackend) CompleteMultipartUpload(ctx context.Context, bucket, key, uploadID string, parts []backend.CompletedPart) (string, error) {
	return "", nil
}

func (m *MockBackend) AbortMultipartUpload(ctx context.Context, bucket, key, uploadID string) error {
	return nil
}

func (m *MockBackend) ListParts(ctx context.Context, bucket, key, uploadID string) (*backend.ListPartsResult, error) {
	return &backend.ListPartsResult{}, nil
}

func (m *MockBackend) ListMultipartUploads(ctx context.Context, bucket, prefix string) (*backend.ListMultipartUploadsResult, error) {
	return &backend.ListMultipartUploadsResult{}, nil
}

func (m *MockBackend) GetBucketLifecycleConfiguration(ctx context.Context, bucket string) ([]byte, error) {
	return nil, backend.ErrObjectNotFound
}

func (m *MockBackend) PutBucketLifecycleConfiguration(ctx context.Context, bucket string, config []byte) error {
	return nil
}

func (m *MockBackend) DeleteBucketLifecycleConfiguration(ctx context.Context, bucket string) error {
	return nil
}

func (m *MockBackend) GetObjectLockConfiguration(ctx context.Context, bucket string) ([]byte, error) {
	return nil, backend.ErrObjectNotFound
}

func (m *MockBackend) PutObjectLockConfiguration(ctx context.Context, bucket string, config []byte) error {
	return nil
}

func (m *MockBackend) GetObjectRetention(ctx context.Context, bucket, key string) ([]byte, error) {
	return nil, backend.ErrObjectNotFound
}

func (m *MockBackend) PutObjectRetention(ctx context.Context, bucket, key string, retention []byte) error {
	return nil
}

func (m *MockBackend) GetObjectLegalHold(ctx context.Context, bucket, key string) ([]byte, error) {
	return nil, backend.ErrObjectNotFound
}

func (m *MockBackend) PutObjectLegalHold(ctx context.Context, bucket, key string, legalHold []byte) error {
	return nil
}

func (m *MockBackend) ListObjectVersions(ctx context.Context, bucket, prefix, delimiter, keyMarker, versionIDMarker string, maxKeys int) (*backend.ListObjectVersionsResult, error) {
	return &backend.ListObjectVersionsResult{}, nil
}

func (m *MockBackend) HeadVersion(ctx context.Context, bucket, key, versionID string) (*backend.ObjectInfo, error) {
	return nil, backend.ErrObjectNotFound
}

// TestFormatMigrationDryRun tests that dry run mode counts objects without migrating.
func TestFormatMigrationDryRun(t *testing.T) {
	ctx := context.Background()
	// Create fresh mock backend for this test to avoid state pollution
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
	ciphertext, hmacTable, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	// Build envelope header
	plaintextSHA := crypto.ComputePlaintextSHA256(plaintext)
	header, err := crypto.NewEnvelopeHeaderWithVersion(iv, int64(len(plaintext)), blockSize, plaintextSHA, crypto.Version1)
	if err != nil {
		t.Fatalf("Failed to build envelope header: %v", err)
	}
	headerBuf, err := header.Encode()
	if err != nil {
		t.Fatalf("Failed to encode envelope header: %v", err)
	}
	ciphertext = append(ciphertext, hmacTable...)

	// Store object with V1 metadata
	metadata := map[string]string{
		"x-amz-meta-armor-version":        "1",
		"x-amz-meta-armor-wrapped-dek":    base64.StdEncoding.EncodeToString(wrappedDEK),
		"x-amz-meta-armor-iv":             base64.StdEncoding.EncodeToString(iv),
		"x-amz-meta-armor-block-size":     "4096",
		"x-amz-meta-armor-plaintext-size": "25",
		"x-amz-meta-armor-sha256":         "test-sha256",
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

// TestFormatMigrationDryRunDoesNotPolluteLiveRun verifies that the state a
// dry run persists (counters and resume cursor) never leaks into a subsequent
// live run. A dry run that dies mid-run leaves its in-progress state behind;
// the next live run must start with clean counters and must still migrate
// objects the dry run only counted.
func TestFormatMigrationDryRunDoesNotPolluteLiveRun(t *testing.T) {
	ctx := context.Background()
	mockBackend := NewMockBackend()

	// Create a test V1 object (same construction as TestFormatMigrationDryRun)
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

	iv := make([]byte, 16)
	blockSize := 4096
	encryptor, err := crypto.NewEncryptorWithVersion(dek, iv, blockSize, crypto.Version1)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	plaintext := []byte("test data for migration")
	ciphertext, hmacTable, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	plaintextSHA := crypto.ComputePlaintextSHA256(plaintext)
	header, err := crypto.NewEnvelopeHeaderWithVersion(iv, int64(len(plaintext)), blockSize, plaintextSHA, crypto.Version1)
	if err != nil {
		t.Fatalf("Failed to build envelope header: %v", err)
	}
	headerBuf, err := header.Encode()
	if err != nil {
		t.Fatalf("Failed to encode envelope header: %v", err)
	}
	ciphertext = append(ciphertext, hmacTable...)

	metadata := map[string]string{
		"x-amz-meta-armor-version":        "1",
		"x-amz-meta-armor-wrapped-dek":    base64.StdEncoding.EncodeToString(wrappedDEK),
		"x-amz-meta-armor-iv":             base64.StdEncoding.EncodeToString(iv),
		"x-amz-meta-armor-block-size":     "4096",
		"x-amz-meta-armor-plaintext-size": "25",
		"x-amz-meta-armor-sha256":         "test-sha256",
	}

	fullData := append(headerBuf, ciphertext...)
	mockBackend.objects["test-object-v1.txt"] = &MockObject{
		Data:     fullData,
		Metadata: metadata,
	}

	// Seed the state a dry run leaves behind when it dies mid-run: in
	// progress, counters already merged from the dry run, cursor advanced
	// past the object.
	polluted := MigrationState{
		ID:                  "format-migration-dry-run-crashed",
		StartTime:           time.Now(),
		LastUpdated:         time.Now(),
		Status:              "in_progress",
		TotalObjects:        1,
		ProcessedObjects:    7,
		SkippedObjects:      5,
		FailedObjects:       3,
		LastKey:             "test-object-v1.txt",
		IncludeVersions:     []string{"1"},
		CurrentWriteVersion: crypto.Version2,
		DryRun:              true,
		Concurrency:         1,
	}
	stateData, err := json.MarshalIndent(polluted, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal polluted state: %v", err)
	}
	mockBackend.objects[".armor/migration-state.json"] = &MockObject{
		Data:     stateData,
		Metadata: map[string]string{"Content-Type": "application/json"},
	}

	// Run a live migration against the polluted state
	migrator := NewFormatMigrator(mockBackend, "test-bucket", mek, "default", crypto.Version2, []string{"1"}, nil)
	result, err := migrator.Migrate(ctx, false, 1)
	if err != nil {
		t.Fatalf("Live migration failed: %v", err)
	}

	// Live-run counters reflect only this run, not the dry run's
	if result.ProcessedObjects != 1 {
		t.Errorf("Expected 1 processed object, got %d (dry-run counters leaked into live run)", result.ProcessedObjects)
	}

	if result.SkippedObjects != 0 {
		t.Errorf("Expected 0 skipped objects, got %d (dry-run counters leaked into live run)", result.SkippedObjects)
	}

	if result.FailedObjects != 0 {
		t.Errorf("Expected 0 failed objects, got %d (dry-run counters leaked into live run)", result.FailedObjects)
	}

	// The dry run's resume cursor must not cause the live run to skip the
	// object - it has to actually be migrated
	obj, ok := mockBackend.objects["test-object-v1.txt"]
	if !ok {
		t.Fatal("Object was deleted during live run")
	}

	if obj.Metadata["x-amz-meta-armor-version"] != "2" {
		t.Errorf("Object version was not migrated to V2, got: %s (dry-run cursor leaked into live run)", obj.Metadata["x-amz-meta-armor-version"])
	}
}

// TestFormatMigrationDryRunDoesNotResumeLiveState verifies the reverse
// direction: a dry run never resumes persisted state, so its counters always
// describe a complete scan of the bucket rather than a previous live run's
// progress.
func TestFormatMigrationDryRunDoesNotResumeLiveState(t *testing.T) {
	ctx := context.Background()
	mockBackend := NewMockBackend()

	// Create a test V1 object (same construction as TestFormatMigrationDryRun)
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

	iv := make([]byte, 16)
	blockSize := 4096
	encryptor, err := crypto.NewEncryptorWithVersion(dek, iv, blockSize, crypto.Version1)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	plaintext := []byte("test data for migration")
	ciphertext, hmacTable, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	plaintextSHA := crypto.ComputePlaintextSHA256(plaintext)
	header, err := crypto.NewEnvelopeHeaderWithVersion(iv, int64(len(plaintext)), blockSize, plaintextSHA, crypto.Version1)
	if err != nil {
		t.Fatalf("Failed to build envelope header: %v", err)
	}
	headerBuf, err := header.Encode()
	if err != nil {
		t.Fatalf("Failed to encode envelope header: %v", err)
	}
	ciphertext = append(ciphertext, hmacTable...)

	metadata := map[string]string{
		"x-amz-meta-armor-version":        "1",
		"x-amz-meta-armor-wrapped-dek":    base64.StdEncoding.EncodeToString(wrappedDEK),
		"x-amz-meta-armor-iv":             base64.StdEncoding.EncodeToString(iv),
		"x-amz-meta-armor-block-size":     "4096",
		"x-amz-meta-armor-plaintext-size": "25",
		"x-amz-meta-armor-sha256":         "test-sha256",
	}

	fullData := append(headerBuf, ciphertext...)
	mockBackend.objects["test-object-v1.txt"] = &MockObject{
		Data:     fullData,
		Metadata: metadata,
	}

	// Seed the state a live run leaves behind when it dies mid-run
	polluted := MigrationState{
		ID:                  "format-migration-live-run-crashed",
		StartTime:           time.Now(),
		LastUpdated:         time.Now(),
		Status:              "in_progress",
		TotalObjects:        1,
		ProcessedObjects:    9,
		SkippedObjects:      4,
		FailedObjects:       2,
		LastKey:             "test-object-v1.txt",
		IncludeVersions:     []string{"1"},
		CurrentWriteVersion: crypto.Version2,
		DryRun:              false,
		Concurrency:         1,
	}
	stateData, err := json.MarshalIndent(polluted, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal polluted state: %v", err)
	}
	mockBackend.objects[".armor/migration-state.json"] = &MockObject{
		Data:     stateData,
		Metadata: map[string]string{"Content-Type": "application/json"},
	}

	// Run a dry-run migration against the polluted state
	migrator := NewFormatMigrator(mockBackend, "test-bucket", mek, "default", crypto.Version2, []string{"1"}, nil)
	result, err := migrator.Migrate(ctx, true, 1)
	if err != nil {
		t.Fatalf("Dry run migration failed: %v", err)
	}

	// Dry-run counters describe this scan only, not the interrupted live run
	if result.ProcessedObjects != 1 {
		t.Errorf("Expected 1 processed object, got %d (live-run counters leaked into dry run)", result.ProcessedObjects)
	}

	if result.SkippedObjects != 0 {
		t.Errorf("Expected 0 skipped objects, got %d (live-run counters leaked into dry run)", result.SkippedObjects)
	}

	if result.FailedObjects != 0 {
		t.Errorf("Expected 0 failed objects, got %d (live-run counters leaked into dry run)", result.FailedObjects)
	}

	// A dry run must not migrate the object
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
	ciphertext, hmacTable, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	// Build envelope header
	plaintextSHA := crypto.ComputePlaintextSHA256(plaintext)
	header, err := crypto.NewEnvelopeHeaderWithVersion(iv, int64(len(plaintext)), blockSize, plaintextSHA, crypto.Version1)
	if err != nil {
		t.Fatalf("Failed to build envelope header: %v", err)
	}
	headerBuf, err := header.Encode()
	if err != nil {
		t.Fatalf("Failed to encode envelope header: %v", err)
	}
	ciphertext = append(ciphertext, hmacTable...)

	// Store object with V1 metadata
	metadata := map[string]string{
		"x-amz-meta-armor-version":        "1",
		"x-amz-meta-armor-wrapped-dek":    base64.StdEncoding.EncodeToString(wrappedDEK),
		"x-amz-meta-armor-iv":             base64.StdEncoding.EncodeToString(iv),
		"x-amz-meta-armor-block-size":     "4096",
		"x-amz-meta-armor-plaintext-size": "30",
		"x-amz-meta-armor-sha256":         "test-sha256",
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
		"x-amz-meta-armor-version":     "2",
		"x-amz-meta-armor-wrapped-dek": "test-dek",
		"x-amz-meta-armor-iv":          "test-iv",
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

// TestFormatMigrationV2AlreadyAtTargetSkipped tests that a V2 object is
// skipped when the target write version is also V2 (include=["2"],
// currentWriteVersion=2). This exercises the already-at-target skip site in
// Migrate -- version in the include list but equal to the target -- distinct
// from TestFormatMigrationV2Skipped, which exercises the not-in-include-list
// skip site.
func TestFormatMigrationV2AlreadyAtTargetSkipped(t *testing.T) {
	ctx := context.Background()
	mockBackend := NewMockBackend()

	// Create a properly-encrypted V2 single-PUT object so that, if the skip
	// path were broken, the migration would actually succeed (making
	// ProcessedObjects != 0 an unambiguous signal) rather than failing on
	// fake metadata.
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

	// Create V2 encryptor
	iv := make([]byte, 16)
	blockSize := 4096
	encryptor, err := crypto.NewEncryptorWithVersion(dek, iv, blockSize, crypto.Version2)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	plaintext := []byte("test data already at v2")
	ciphertext, hmacTable, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	// Build envelope header
	plaintextSHA := crypto.ComputePlaintextSHA256(plaintext)
	header, err := crypto.NewEnvelopeHeaderWithVersion(iv, int64(len(plaintext)), blockSize, plaintextSHA, crypto.Version2)
	if err != nil {
		t.Fatalf("Failed to build envelope header: %v", err)
	}
	headerBuf, err := header.Encode()
	if err != nil {
		t.Fatalf("Failed to encode envelope header: %v", err)
	}
	ciphertext = append(ciphertext, hmacTable...)

	// Store object with V2 metadata
	metadata := map[string]string{
		"x-amz-meta-armor-version":        "2",
		"x-amz-meta-armor-wrapped-dek":    base64.StdEncoding.EncodeToString(wrappedDEK),
		"x-amz-meta-armor-iv":             base64.StdEncoding.EncodeToString(iv),
		"x-amz-meta-armor-block-size":     "4096",
		"x-amz-meta-armor-plaintext-size": fmt.Sprintf("%d", len(plaintext)),
		"x-amz-meta-armor-sha256":         hex.EncodeToString(plaintextSHA[:]),
	}

	// Combine header and ciphertext
	fullData := append(headerBuf, ciphertext...)
	originalData := append([]byte(nil), fullData...)
	mockBackend.objects["test-object-v2-at-target.txt"] = &MockObject{
		Data:     fullData,
		Metadata: metadata,
	}

	// Create migrator whose target version is V2 -- the object's version is
	// in the include list but already equals the current write version.
	migrator := NewFormatMigrator(mockBackend, "test-bucket", mek, "default", crypto.Version2, []string{"2"}, nil)

	// Run migration
	result, err := migrator.Migrate(ctx, false, 1)
	if err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	// Verify the object was skipped, not processed
	if result.ProcessedObjects != 0 {
		t.Errorf("Expected 0 processed objects, got %d", result.ProcessedObjects)
	}

	if result.SkippedObjects != 1 {
		t.Errorf("Expected 1 skipped object, got %d", result.SkippedObjects)
	}

	if result.FailedObjects != 0 {
		t.Errorf("Expected 0 failed objects, got %d", result.FailedObjects)
	}

	// Already-at-target objects are not counted as migration candidates
	// (see countObjects).
	if result.TotalObjects != 0 {
		t.Errorf("Expected 0 total objects, got %d", result.TotalObjects)
	}

	// Verify the object was left untouched
	obj, ok := mockBackend.objects["test-object-v2-at-target.txt"]
	if !ok {
		t.Fatal("Object was deleted during migration")
	}

	if !bytes.Equal(obj.Data, originalData) {
		t.Error("Object data was modified during migration")
	}

	if obj.Metadata["x-amz-meta-armor-version"] != "2" {
		t.Errorf("Object version was changed during migration, got: %s", obj.Metadata["x-amz-meta-armor-version"])
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
			"x-amz-meta-armor-version":     "1",
			"x-amz-meta-armor-wrapped-dek": "test-dek",
			"x-amz-meta-armor-iv":          "test-iv",
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
		ID:               "test-migration",
		Status:           "in_progress",
		LastKey:          "test-object-2.txt",
		TotalObjects:     3,
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
}

// TestFormatMigrationEndpoint tests the HTTP endpoint for format migration.
func TestFormatMigrationEndpoint(t *testing.T) {
	// This would require setting up a full server with admin auth
	// For now, we'll test the handler logic in isolation
	t.Skip("HTTP endpoint test requires full server setup")
}

// TestFormatMigrationMultipartToSingle tests migrating a V1 multipart object to V2 single-PUT.
func TestFormatMigrationMultipartToSingle(t *testing.T) {
	ctx := context.Background()
	mockBackend := NewMockBackend()

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

	// Small plaintext that will be migrated to single-PUT (< 5MB threshold)
	plaintext := []byte("multipart data for migration test")
	ciphertext, hmacTable, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	// For V1 multipart objects, HMAC table is in sidecar only, not appended to ciphertext
	// Store as multipart object (simulate assembled multipart)
	metadata := map[string]string{
		"x-amz-meta-armor-version":        "1",
		"x-amz-meta-armor-wrapped-dek":    base64.StdEncoding.EncodeToString(wrappedDEK),
		"x-amz-meta-armor-iv":             base64.StdEncoding.EncodeToString(iv),
		"x-amz-meta-armor-block-size":     "4096",
		"x-amz-meta-armor-plaintext-size": "29",
		"x-amz-meta-armor-sha256":         "test-sha256",
		"x-amz-meta-armor-multipart":      "true",
	}

	mockBackend.objects["multipart-test.dat"] = &MockObject{
		Data:     ciphertext,
		Metadata: metadata,
	}

	// Create HMAC sidecar with actual HMAC table from encryption
	keySHA := sha256.Sum256([]byte("multipart-test.dat"))
	sidecarPath := fmt.Sprintf(".armor/hmac/%x", keySHA)
	mockBackend.objects[sidecarPath] = &MockObject{
		Data: hmacTable,
	}

	// Create migrator
	migrator := NewFormatMigrator(mockBackend, "test-bucket", mek, "default", crypto.Version2, []string{"1"}, nil)

	// Run migration - should handle multipart properly
	result, err := migrator.Migrate(ctx, false, 1)
	if err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	// Verify migration completed
	if result.ProcessedObjects != 1 {
		t.Errorf("Expected 1 processed object, got %d", result.ProcessedObjects)
	}

	// Verify object was migrated
	obj, ok := mockBackend.objects["multipart-test.dat"]
	if !ok {
		t.Fatal("Object was deleted during migration")
	}

	if obj.Metadata["x-amz-meta-armor-version"] != "2" {
		t.Errorf("Object version was not migrated to V2, got: %s", obj.Metadata["x-amz-meta-armor-version"])
	}
}

// TestFormatMigrationFailureRecording tests that failed objects are recorded and not retried.
func TestFormatMigrationFailureRecording(t *testing.T) {
	ctx := context.Background()
	mockBackend := NewMockBackend()

	mek := make([]byte, 32)
	for i := range mek {
		mek[i] = byte(i)
	}

	// Create a corrupted object that will fail migration
	metadata := map[string]string{
		"x-amz-meta-armor-version":     "1",
		"x-amz-meta-armor-wrapped-dek": "invalid-dek",
		"x-amz-meta-armor-iv":          "invalid-iv",
	}

	mockBackend.objects["corrupted.dat"] = &MockObject{
		Data:     []byte("corrupted data"),
		Metadata: metadata,
	}

	// Create migrator
	migrator := NewFormatMigrator(mockBackend, "test-bucket", mek, "default", crypto.Version2, []string{"1"}, nil)

	// Run migration - should record failure and continue
	result, err := migrator.Migrate(ctx, false, 1)
	if err != nil {
		// Migration completes even with failures
		t.Logf("Migration completed with errors (expected): %v", err)
	}

	// Verify failure was recorded
	if result.FailedObjects != 1 {
		t.Errorf("Expected 1 failed object, got %d", result.FailedObjects)
	}

	if len(result.Failures) != 1 {
		t.Errorf("Expected 1 failure record, got %d", len(result.Failures))
	}

	if result.Failures[0].Key != "corrupted.dat" {
		t.Errorf("Expected failure for 'corrupted.dat', got: %s", result.Failures[0].Key)
	}

	if result.Failures[0].Reason == "" {
		t.Error("Expected failure reason to be recorded")
	}

	// The failure must also reach the persistent state atomically with the
	// counter increment, not be deferred to a completion merge. If the state
	// write is dropped from the failure path this reads 0; if the completion
	// merge re-adds this run's failures it reads 2 (each regression happened
	// historically - see docs/research/format-migration-failure-recording-flow.md).
	state := migrator.GetState()
	if state.FailedObjects != 1 {
		t.Errorf("Expected state.FailedObjects to be 1, got %d", state.FailedObjects)
	}

	if len(state.Failures) != 1 {
		t.Errorf("Expected 1 state failure record, got %d", len(state.Failures))
	}

	if len(state.Failures) == 1 && state.Failures[0].Key != "corrupted.dat" {
		t.Errorf("Expected state failure for 'corrupted.dat', got: %s", state.Failures[0].Key)
	}
}

// TestFormatMigrationFailurePersistenceRoundTrip verifies that a failure
// recorded during a run survives the saveState -> loadState JSON round trip
// against the backend. The in-memory assertions in
// TestFormatMigrationFailureRecording only cover the live state; this covers
// what a resumed run or operator actually reads back from
// .armor/migration-state.json after the process is gone.
func TestFormatMigrationFailurePersistenceRoundTrip(t *testing.T) {
	ctx := context.Background()
	mockBackend := NewMockBackend()

	mek := make([]byte, 32)
	for i := range mek {
		mek[i] = byte(i)
	}

	// A V1 object whose wrapped DEK cannot be unwrapped: migration fails and
	// a failure record is written at the failure site.
	metadata := map[string]string{
		"x-amz-meta-armor-version":     "1",
		"x-amz-meta-armor-wrapped-dek": "invalid-dek",
		"x-amz-meta-armor-iv":          "invalid-iv",
	}
	mockBackend.objects["corrupted-persist.dat"] = &MockObject{
		Data:     []byte("corrupted data"),
		Metadata: metadata,
	}

	migrator := NewFormatMigrator(mockBackend, "test-bucket", mek, "default", crypto.Version2, []string{"1"}, nil)

	startedAt := time.Now()
	result, err := migrator.Migrate(ctx, false, 1)
	if err != nil {
		// Migration completes even with failures
		t.Logf("Migration completed with errors (expected): %v", err)
	}
	if result.FailedObjects != 1 {
		t.Fatalf("Expected 1 failed object in run result, got %d", result.FailedObjects)
	}

	// Capture the live record so the reload can be checked for fidelity
	// (every field identical), not just presence.
	before := migrator.GetState()
	if len(before.Failures) != 1 {
		t.Fatalf("Expected 1 failure record in live state, got %d", len(before.Failures))
	}

	// The completion saveState must have left the state object on the backend.
	rawState, _, err := mockBackend.GetDirect(ctx, "test-bucket", ".armor/migration-state.json")
	if err != nil {
		t.Fatalf("Migration state was not persisted to the backend: %v", err)
	}
	rawData, err := io.ReadAll(rawState)
	rawState.Close()
	if err != nil {
		t.Fatalf("Failed to read persisted state: %v", err)
	}

	// Reload through the production loader (the path a resumed run takes).
	reloaded, err := migrator.loadState(ctx)
	if err != nil {
		t.Fatalf("Failed to load persisted migration state: %v", err)
	}

	if reloaded.FailedObjects != 1 {
		t.Errorf("Expected reloaded FailedObjects to be 1, got %d", reloaded.FailedObjects)
	}

	if len(reloaded.Failures) != 1 {
		t.Fatalf("Expected 1 failure record in reloaded state, got %d", len(reloaded.Failures))
	}

	f := reloaded.Failures[0]
	if f.Key != "corrupted-persist.dat" {
		t.Errorf("Expected persisted failure key 'corrupted-persist.dat', got: %s", f.Key)
	}
	if f.Reason == "" {
		t.Error("Expected persisted failure reason to be non-empty")
	}

	// Fidelity, not just presence: the reloaded record must equal the live
	// one field for field. time.Time needs Equal rather than == because the
	// monotonic clock reading is stripped by the JSON encoding (the wall
	// instant it represents is preserved exactly by RFC 3339 nano).
	live := before.Failures[0]
	if f.Key != live.Key || f.Reason != live.Reason || f.Details != live.Details || !f.Time.Equal(live.Time) {
		t.Errorf("Reloaded failure record differs from the live record:\n  live:     %+v\n  reloaded: %+v", live, f)
	}

	// A time.Time survives a JSON round trip either as a valid non-zero time
	// or not at all (unmarshal fails), so a zero value here means the field
	// was dropped or corrupted in persisting.
	if f.Time.IsZero() {
		t.Error("Expected persisted failure Time to be a non-zero timestamp")
	} else {
		if f.Time.Before(startedAt) {
			t.Errorf("Persisted failure Time %v predates migration start %v", f.Time, startedAt)
		}
		if f.Time.After(time.Now().Add(time.Minute)) {
			t.Errorf("Persisted failure Time %v is implausibly far in the future", f.Time)
		}
	}

	// Decode the raw backend object directly, so a loss here is attributed to
	// the persisted JSON rather than to the loader.
	var fromBackend MigrationState
	if err := json.Unmarshal(rawData, &fromBackend); err != nil {
		t.Fatalf("Persisted state JSON does not parse: %v", err)
	}
	if len(fromBackend.Failures) != 1 || fromBackend.Failures[0].Key != "corrupted-persist.dat" {
		t.Errorf("Raw persisted state lost the failure record: %s", string(rawData))
	}
}

// TestFormatMigrationV2SkippedByDefault tests that V2 objects are skipped when include=v1 (default).
func TestFormatMigrationV2SkippedByDefault(t *testing.T) {
	ctx := context.Background()
	mockBackend := NewMockBackend()

	mek := make([]byte, 32)
	for i := range mek {
		mek[i] = byte(i)
	}

	// Create V2 objects
	for i := 1; i <= 3; i++ {
		metadata := map[string]string{
			"x-amz-meta-armor-version":     "2",
			"x-amz-meta-armor-wrapped-dek": "test-dek",
			"x-amz-meta-armor-iv":          "test-iv",
		}
		mockBackend.objects[fmt.Sprintf("v2-object-%d.txt", i)] = &MockObject{
			Data:     []byte(fmt.Sprintf("v2 data %d", i)),
			Metadata: metadata,
		}
	}

	// Create migrator with default include (v1 only)
	migrator := NewFormatMigrator(mockBackend, "test-bucket", mek, "default", crypto.Version2, []string{"1"}, nil)

	// Run migration
	result, err := migrator.Migrate(ctx, false, 1)
	if err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	// Verify all V2 objects were skipped
	if result.ProcessedObjects != 0 {
		t.Errorf("Expected 0 processed objects (V2 should be skipped), got %d", result.ProcessedObjects)
	}

	if result.SkippedObjects != 3 {
		t.Errorf("Expected 3 skipped objects, got %d", result.SkippedObjects)
	}
}

// NOTE: a resume-capability test (verifying that re-running Migrate after a
// partial run only processes previously-unprocessed objects) was truncated
// here -- its setup/migrator-call was missing, leaving only trailing
// assertions with no enclosing function (a syntax error blocking every
// build). Removed rather than reconstructed: the intended setup isn't
// recoverable from what remained, and guessing at Migrate's resume
// semantics risks asserting something that was never actually true. Coverage
// for resume behavior is worth adding back deliberately, not guessed here.

// TestFormatMigrationFailure_InvalidBase64DEK tests that invalid base64 in wrapped-dek field is detected and recorded.
func TestFormatMigrationFailure_InvalidBase64DEK(t *testing.T) {
	ctx := context.Background()
	mockBackend := NewMockBackend()

	mek := make([]byte, 32)
	for i := range mek {
		mek[i] = byte(i)
	}

	// Create an object with invalid base64 in wrapped-dek field
	metadata := map[string]string{
		"x-amz-meta-armor-version":     "1",
		"x-amz-meta-armor-wrapped-dek": "invalid-base64!!!",
		"x-amz-meta-armor-iv":          base64.StdEncoding.EncodeToString(make([]byte, 16)),
		"x-amz-meta-armor-block-size":  "4096",
	}

	mockBackend.objects["invalid-dek.dat"] = &MockObject{
		Data:     []byte("test data"),
		Metadata: metadata,
	}

	// Create migrator
	migrator := NewFormatMigrator(mockBackend, "test-bucket", mek, "default", crypto.Version2, []string{"1"}, nil)

	// Run migration - should record failure and continue
	result, err := migrator.Migrate(ctx, false, 1)
	if err != nil {
		t.Logf("Migration completed with errors (expected): %v", err)
	}

	// Verify failure was recorded
	if result.FailedObjects != 1 {
		t.Errorf("Expected 1 failed object, got %d", result.FailedObjects)
	}

	if len(result.Failures) != 1 {
		t.Errorf("Expected 1 failure record, got %d", len(result.Failures))
	}

	if result.Failures[0].Key != "invalid-dek.dat" {
		t.Errorf("Expected failure for 'invalid-dek.dat', got: %s", result.Failures[0].Key)
	}

	// Verify the reason mentions invalid base64
	if result.Failures[0].Reason == "" {
		t.Error("Expected failure reason to be recorded")
	}

	if !strings.Contains(result.Failures[0].Reason, "invalid base64") &&
		!strings.Contains(result.Failures[0].Reason, "illegal base64") {
		t.Errorf("Expected reason to mention invalid base64, got: %s", result.Failures[0].Reason)
	}

	// Verify processed counter was incremented despite failure
	if result.ProcessedObjects != 1 {
		t.Errorf("Expected 1 processed object, got %d", result.ProcessedObjects)
	}
}

// TestFormatMigrationFailure_InvalidBase64IV tests that invalid base64 in iv field is detected and recorded.
func TestFormatMigrationFailure_InvalidBase64IV(t *testing.T) {
	ctx := context.Background()
	mockBackend := NewMockBackend()

	mek := make([]byte, 32)
	for i := range mek {
		mek[i] = byte(i)
	}

	// Create an object with invalid base64 in iv field
	metadata := map[string]string{
		"x-amz-meta-armor-version":     "1",
		"x-amz-meta-armor-wrapped-dek": base64.StdEncoding.EncodeToString(make([]byte, 64)),
		"x-amz-meta-armor-iv":          "invalid-base64@@@",
		"x-amz-meta-armor-block-size":  "4096",
	}

	mockBackend.objects["invalid-iv.dat"] = &MockObject{
		Data:     []byte("test data"),
		Metadata: metadata,
	}

	// Create migrator
	migrator := NewFormatMigrator(mockBackend, "test-bucket", mek, "default", crypto.Version2, []string{"1"}, nil)

	// Run migration - should record failure and continue
	result, err := migrator.Migrate(ctx, false, 1)
	if err != nil {
		t.Logf("Migration completed with errors (expected): %v", err)
	}

	// Verify failure was recorded
	if result.FailedObjects != 1 {
		t.Errorf("Expected 1 failed object, got %d", result.FailedObjects)
	}

	if len(result.Failures) != 1 {
		t.Errorf("Expected 1 failure record, got %d", len(result.Failures))
	}

	if result.Failures[0].Key != "invalid-iv.dat" {
		t.Errorf("Expected failure for 'invalid-iv.dat', got: %s", result.Failures[0].Key)
	}

	// Verify the reason mentions invalid base64 in IV
	if !strings.Contains(result.Failures[0].Reason, "invalid base64") &&
		!strings.Contains(result.Failures[0].Reason, "illegal base64") &&
		!strings.Contains(result.Failures[0].Reason, "IV") {
		t.Errorf("Expected reason to mention invalid base64 or IV, got: %s", result.Failures[0].Reason)
	}
}

// TestFormatMigrationFailure_CorruptedCiphertext tests that corrupted ciphertext fails during decrypt and is recorded.
func TestFormatMigrationFailure_CorruptedCiphertext(t *testing.T) {
	ctx := context.Background()
	mockBackend := NewMockBackend()

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

	iv := make([]byte, 16)
	blockSize := 4096

	// Create a valid V1 envelope header first
	plaintextSHA := sha256.Sum256([]byte("test data"))
	plaintextSHAArray := plaintextSHA
	header, err := crypto.NewEnvelopeHeaderWithVersion(iv, int64(len("test data")), blockSize, plaintextSHAArray, crypto.Version1)
	if err != nil {
		t.Fatalf("Failed to create header: %v", err)
	}
	headerBuf, err := header.Encode()
	if err != nil {
		t.Fatalf("Failed to encode header: %v", err)
	}

	// Create corrupted ciphertext - just random bytes that won't decrypt
	corruptedCiphertext := make([]byte, 100)
	for i := range corruptedCiphertext {
		corruptedCiphertext[i] = byte(i + 99)
	}

	metadata := map[string]string{
		"x-amz-meta-armor-version":        "1",
		"x-amz-meta-armor-wrapped-dek":    base64.StdEncoding.EncodeToString(wrappedDEK),
		"x-amz-meta-armor-iv":             base64.StdEncoding.EncodeToString(iv),
		"x-amz-meta-armor-block-size":     "4096",
		"x-amz-meta-armor-plaintext-size": "9",
		"x-amz-meta-armor-sha256":         hex.EncodeToString(plaintextSHA[:]),
	}

	// Combine header and corrupted ciphertext
	fullData := append(headerBuf, corruptedCiphertext...)
	mockBackend.objects["corrupted-cipher.dat"] = &MockObject{
		Data:     fullData,
		Metadata: metadata,
	}

	// Create migrator
	migrator := NewFormatMigrator(mockBackend, "test-bucket", mek, "default", crypto.Version2, []string{"1"}, nil)

	// Run migration - should record failure and continue
	result, err := migrator.Migrate(ctx, false, 1)
	if err != nil {
		t.Logf("Migration completed with errors (expected): %v", err)
	}

	// Verify failure was recorded
	if result.FailedObjects != 1 {
		t.Errorf("Expected 1 failed object, got %d", result.FailedObjects)
	}

	if len(result.Failures) != 1 {
		t.Errorf("Expected 1 failure record, got %d", len(result.Failures))
	}

	if result.Failures[0].Key != "corrupted-cipher.dat" {
		t.Errorf("Expected failure for 'corrupted-cipher.dat', got: %s", result.Failures[0].Key)
	}

	// Verify the reason mentions the decrypt failure
	if !strings.Contains(result.Failures[0].Reason, "decrypt") &&
		!strings.Contains(result.Failures[0].Reason, "HMAC") &&
		!strings.Contains(result.Failures[0].Reason, "ciphertext") {
		t.Errorf("Expected reason to mention decrypt/HMAC/ciphertext failure, got: %s", result.Failures[0].Reason)
	}
}

// TestFormatMigrationFailure_MissingMetadata tests that missing required metadata fields are detected and recorded.
func TestFormatMigrationFailure_MissingMetadata(t *testing.T) {
	ctx := context.Background()
	mockBackend := NewMockBackend()

	mek := make([]byte, 32)
	for i := range mek {
		mek[i] = byte(i)
	}

	// Create an object with missing wrapped-dek field (required metadata)
	metadata := map[string]string{
		"x-amz-meta-armor-version": "1",
		// Missing wrapped-dek
		"x-amz-meta-armor-iv": base64.StdEncoding.EncodeToString(make([]byte, 16)),
	}

	mockBackend.objects["missing-meta.dat"] = &MockObject{
		Data:     []byte("test data"),
		Metadata: metadata,
	}

	// Create migrator
	migrator := NewFormatMigrator(mockBackend, "test-bucket", mek, "default", crypto.Version2, []string{"1"}, nil)

	// Run migration - should record failure and continue
	result, err := migrator.Migrate(ctx, false, 1)
	if err != nil {
		t.Logf("Migration completed with errors (expected): %v", err)
	}

	// Verify failure was recorded
	if result.FailedObjects != 1 {
		t.Errorf("Expected 1 failed object, got %d", result.FailedObjects)
	}

	if len(result.Failures) != 1 {
		t.Errorf("Expected 1 failure record, got %d", len(result.Failures))
	}

	if result.Failures[0].Key != "missing-meta.dat" {
		t.Errorf("Expected failure for 'missing-meta.dat', got: %s", result.Failures[0].Key)
	}

	// Verify the reason mentions missing or invalid metadata
	if result.Failures[0].Reason == "" {
		t.Error("Expected failure reason to be recorded")
	}
}

// TestFormatMigrationFailedObjectsNotRetried tests that failed objects are skipped on subsequent migration runs.
func TestFormatMigrationFailedObjectsNotRetried(t *testing.T) {
	ctx := context.Background()
	mockBackend := NewMockBackend()

	mek := make([]byte, 32)
	for i := range mek {
		mek[i] = byte(i)
	}

	// buildValidV1Object stores a fully valid, migratable V1 single-PUT object.
	buildValidV1Object := func(key, plaintext string) {
		dek := make([]byte, 32)
		for i := range dek {
			dek[i] = byte(i + 1)
		}

		wrappedDEK, err := crypto.WrapDEK(mek, dek)
		if err != nil {
			t.Fatalf("Failed to wrap DEK: %v", err)
		}

		iv := make([]byte, 16)
		blockSize := 4096
		encryptor, err := crypto.NewEncryptorWithVersion(dek, iv, blockSize, crypto.Version1)
		if err != nil {
			t.Fatalf("Failed to create encryptor: %v", err)
		}

		pt := []byte(plaintext)
		ciphertext, hmacTable, err := encryptor.Encrypt(pt)
		if err != nil {
			t.Fatalf("Failed to encrypt: %v", err)
		}

		plaintextSHA := crypto.ComputePlaintextSHA256(pt)
		header, err := crypto.NewEnvelopeHeaderWithVersion(iv, int64(len(pt)), blockSize, plaintextSHA, crypto.Version1)
		if err != nil {
			t.Fatalf("Failed to build envelope header: %v", err)
		}
		headerBuf, err := header.Encode()
		if err != nil {
			t.Fatalf("Failed to encode envelope header: %v", err)
		}
		ciphertext = append(ciphertext, hmacTable...)

		metadata := map[string]string{
			"x-amz-meta-armor-version":        "1",
			"x-amz-meta-armor-wrapped-dek":    base64.StdEncoding.EncodeToString(wrappedDEK),
			"x-amz-meta-armor-iv":             base64.StdEncoding.EncodeToString(iv),
			"x-amz-meta-armor-block-size":     "4096",
			"x-amz-meta-armor-plaintext-size": fmt.Sprintf("%d", len(pt)),
			"x-amz-meta-armor-sha256":         hex.EncodeToString(plaintextSHA[:]),
		}

		fullData := append(headerBuf, ciphertext...)
		mockBackend.objects[key] = &MockObject{
			Data:     fullData,
			Metadata: metadata,
		}
	}

	// Object 1: Valid V1 object (migrates successfully)
	buildValidV1Object("a-valid-1.txt", "valid data 1")

	// Object 2: Invalid base64 in wrapped-dek (will fail)
	invalidMetadata := map[string]string{
		"x-amz-meta-armor-version":     "1",
		"x-amz-meta-armor-wrapped-dek": "invalid-base64!!!",
		"x-amz-meta-armor-iv":          base64.StdEncoding.EncodeToString(make([]byte, 16)),
		"x-amz-meta-armor-block-size":  "4096",
	}
	mockBackend.objects["b-invalid.txt"] = &MockObject{
		Data:     []byte("invalid data"),
		Metadata: invalidMetadata,
	}

	// Object 3: Valid V1 object (migrates successfully)
	buildValidV1Object("c-valid-2.txt", "valid data 2")

	// Create migrator for first run
	migrator1 := NewFormatMigrator(mockBackend, "test-bucket", mek, "default", crypto.Version2, []string{"1"}, nil)

	// First migration run - should process all 3, fail on 1
	result1, err := migrator1.Migrate(ctx, false, 1)
	if err != nil {
		t.Logf("First migration completed with errors (expected): %v", err)
	}

	// Verify first run results
	if result1.ProcessedObjects != 3 {
		t.Errorf("Expected 3 processed objects in first run, got %d", result1.ProcessedObjects)
	}

	if result1.FailedObjects != 1 {
		t.Errorf("Expected 1 failed object in first run, got %d", result1.FailedObjects)
	}

	if len(result1.Failures) != 1 {
		t.Fatalf("Expected 1 failure record in first run, got %d", len(result1.Failures))
	}

	if result1.Failures[0].Key != "b-invalid.txt" {
		t.Errorf("Expected failure for 'b-invalid.txt', got: %s", result1.Failures[0].Key)
	}

	// Verify the two valid objects were migrated to V2
	if v := mockBackend.objects["a-valid-1.txt"].Metadata["x-amz-meta-armor-version"]; v != "2" {
		t.Errorf("Expected a-valid-1.txt migrated to version 2, got %s", v)
	}
	if v := mockBackend.objects["c-valid-2.txt"].Metadata["x-amz-meta-armor-version"]; v != "2" {
		t.Errorf("Expected c-valid-2.txt migrated to version 2, got %s", v)
	}

	// Capture state from run 1. Simulate an interrupted migration by marking
	// the persisted state in_progress, exactly as a crashed run would leave
	// it, so run 2 exercises the real resume path in initOrLoadState.
	state := migrator1.GetState()
	if state == nil {
		t.Fatal("Expected migration state to exist")
	}

	// The failed object must sit at or before the cursor: it was visited, its
	// failure recorded, and the cursor advanced past it.
	if state.LastKey == "" {
		t.Fatal("Expected LastKey cursor to be set after run 1")
	}
	if state.LastKey < "b-invalid.txt" {
		t.Errorf("Expected cursor to have advanced past the failed object, got: %s", state.LastKey)
	}

	state.Status = "in_progress"
	stateData, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal state: %v", err)
	}
	mockBackend.objects[".armor/migration-state.json"] = &MockObject{Data: stateData}

	// Second migration run with a fresh migrator instance. It loads the
	// persisted state, resumes from LastKey, and must skip every already
	// visited key -- including the failed one.
	migrator2 := NewFormatMigrator(mockBackend, "test-bucket", mek, "default", crypto.Version2, []string{"1"}, nil)

	result2, err := migrator2.Migrate(ctx, false, 1)
	if err != nil {
		t.Fatalf("Second migration failed: %v", err)
	}

	// The result of run 2 reports cumulative state (run counters are merged
	// into state on completion), so compare against run 1's cumulative
	// values: nothing new processed, nothing new failed.
	if result2.ProcessedObjects != result1.ProcessedObjects {
		t.Errorf("Expected cumulative processed objects to remain %d (failed object must not be retried), got %d",
			result1.ProcessedObjects, result2.ProcessedObjects)
	}

	if result2.FailedObjects != 1 {
		t.Errorf("Expected cumulative failed objects to remain 1, got %d", result2.FailedObjects)
	}

	if len(result2.Failures) != 1 {
		t.Errorf("Expected cumulative failure records to remain 1, got %d", len(result2.Failures))
	}

	if len(result2.Failures) == 1 && result2.Failures[0].Key != "b-invalid.txt" {
		t.Errorf("Expected the only failure to still be 'b-invalid.txt', got: %s", result2.Failures[0].Key)
	}

	// The invalid object must remain untouched at V1 -- proving it was skipped
	// rather than retried or migrated.
	if v := mockBackend.objects["b-invalid.txt"].Metadata["x-amz-meta-armor-version"]; v != "1" {
		t.Errorf("Expected failed object to remain at version 1 (not retried), got %s", v)
	}

	// The failed object remains a migration candidate (still V1), but nothing
	// touched it: the cursor alone is why it was skipped.
	finalState := migrator2.GetState()
	if finalState == nil {
		t.Fatal("Expected migration state to exist after run 2")
	}

	if finalState.FailedObjects != 1 {
		t.Errorf("Expected final cumulative failed objects to remain 1, got %d", finalState.FailedObjects)
	}

	if len(finalState.Failures) != 1 {
		t.Errorf("Expected final cumulative failure records to remain 1, got %d", len(finalState.Failures))
	}
}
