package provenance

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
)

// mockBackend implements backend.Backend for testing provenance.
type mockBackend struct {
	objects  map[string][]byte
	metadata map[string]map[string]string
}

func newMockBackend() *mockBackend {
	return &mockBackend{
		objects:  make(map[string][]byte),
		metadata: make(map[string]map[string]string),
	}
}

func (m *mockBackend) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	m.objects[bucket+"/"+key] = data
	if meta != nil {
		m.metadata[bucket+"/"+key] = meta
	}
	return nil
}

func (m *mockBackend) Get(ctx context.Context, bucket, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	data, ok := m.objects[bucket+"/"+key]
	if !ok {
		return nil, nil, fmt.Errorf("not found")
	}
	meta := m.metadata[bucket+"/"+key]
	if meta == nil {
		meta = make(map[string]string)
	}
	_, isARMOR := meta["x-amz-meta-armor-version"]
	return io.NopCloser(bytes.NewReader(data)), &backend.ObjectInfo{
		Key:              key,
		Size:             int64(len(data)),
		Metadata:         meta,
		IsARMOREncrypted: isARMOR,
	}, nil
}

func (m *mockBackend) GetDirect(ctx context.Context, bucket, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	return m.Get(ctx, bucket, key)
}

func (m *mockBackend) GetRange(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, error) {
	body, _, err := m.GetRangeWithHeaders(ctx, bucket, key, offset, length)
	return body, err
}

func (m *mockBackend) GetRangeWithHeaders(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, map[string]string, error) {
	data, ok := m.objects[bucket+"/"+key]
	if !ok {
		return nil, nil, fmt.Errorf("not found")
	}
	end := offset + length
	if end > int64(len(data)) {
		end = int64(len(data))
	}
	// Mock doesn't simulate CF caching, so return empty headers
	return io.NopCloser(bytes.NewReader(data[offset:end])), make(map[string]string), nil
}

func (m *mockBackend) Head(ctx context.Context, bucket, key string) (*backend.ObjectInfo, error) {
	data, ok := m.objects[bucket+"/"+key]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	meta := m.metadata[bucket+"/"+key]
	if meta == nil {
		meta = make(map[string]string)
	}
	_, isARMOR := meta["x-amz-meta-armor-version"]
	return &backend.ObjectInfo{
		Key:              key,
		Size:             int64(len(data)),
		Metadata:         meta,
		IsARMOREncrypted: isARMOR,
	}, nil
}

func (m *mockBackend) Delete(ctx context.Context, bucket, key string) error {
	delete(m.objects, bucket+"/"+key)
	delete(m.metadata, bucket+"/"+key)
	return nil
}

func (m *mockBackend) DeleteObjects(ctx context.Context, bucket string, keys []string) error {
	for _, key := range keys {
		delete(m.objects, bucket+"/"+key)
		delete(m.metadata, bucket+"/"+key)
	}
	return nil
}

func (m *mockBackend) List(ctx context.Context, bucket, prefix, delimiter, continuationToken string, maxKeys int) (*backend.ListResult, error) {
	var objects []backend.ObjectInfo
	for k, v := range m.objects {
		// Filter to only objects in this bucket
		if !strings.HasPrefix(k, bucket+"/") {
			continue
		}
		key := strings.TrimPrefix(k, bucket+"/")
		if prefix != "" && !strings.HasPrefix(key, prefix) {
			continue
		}
		meta := m.metadata[k]
		if meta == nil {
			meta = make(map[string]string)
		}
		_, isARMOR := meta["x-amz-meta-armor-version"]
		objects = append(objects, backend.ObjectInfo{
			Key:              key,
			Size:             int64(len(v)),
			Metadata:         meta,
			IsARMOREncrypted: isARMOR,
		})
	}
	return &backend.ListResult{Objects: objects}, nil
}

func (m *mockBackend) ListRaw(ctx context.Context, bucket, prefix, delimiter, continuationToken string, maxKeys int) (*backend.ListResult, error) {
	return m.List(ctx, bucket, prefix, delimiter, continuationToken, maxKeys)
}

func (m *mockBackend) Copy(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string, meta map[string]string, replaceMetadata bool) error {
	src := srcBucket + "/" + srcKey
	dst := dstBucket + "/" + dstKey
	data, ok := m.objects[src]
	if !ok {
		return fmt.Errorf("not found")
	}
	m.objects[dst] = data
	if replaceMetadata && meta != nil {
		m.metadata[dst] = meta
	} else if !replaceMetadata {
		m.metadata[dst] = m.metadata[src]
	}
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

func (m *mockBackend) ListMultipartUploads(ctx context.Context, bucket, prefix string) (*backend.ListMultipartUploadsResult, error) {
	return nil, nil
}

// Lifecycle configuration methods (stub implementations for testing)
func (m *mockBackend) GetBucketLifecycleConfiguration(ctx context.Context, bucket string) ([]byte, error) {
	return nil, fmt.Errorf("lifecycle configuration not found")
}

func (m *mockBackend) PutBucketLifecycleConfiguration(ctx context.Context, bucket string, config []byte) error {
	return nil
}

func (m *mockBackend) DeleteBucketLifecycleConfiguration(ctx context.Context, bucket string) error {
	return nil
}

// Object Lock methods (stub implementations for testing)
func (m *mockBackend) GetObjectLockConfiguration(ctx context.Context, bucket string) ([]byte, error) {
	return nil, fmt.Errorf("object lock configuration not found")
}

func (m *mockBackend) PutObjectLockConfiguration(ctx context.Context, bucket string, config []byte) error {
	return nil
}

func (m *mockBackend) GetObjectRetention(ctx context.Context, bucket, key string) ([]byte, error) {
	return nil, fmt.Errorf("retention not found")
}

func (m *mockBackend) PutObjectRetention(ctx context.Context, bucket, key string, retention []byte) error {
	return nil
}

func (m *mockBackend) GetObjectLegalHold(ctx context.Context, bucket, key string) ([]byte, error) {
	return nil, fmt.Errorf("legal hold not found")
}

func (m *mockBackend) PutObjectLegalHold(ctx context.Context, bucket, key string, legalHold []byte) error {
	return nil
}

func (m *mockBackend) ListObjectVersions(ctx context.Context, bucket, prefix, delimiter, keyMarker, versionIDMarker string, maxKeys int) (*backend.ListObjectVersionsResult, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockBackend) HeadVersion(ctx context.Context, bucket, key, versionID string) (*backend.ObjectInfo, error) {
	return nil, fmt.Errorf("not implemented")
}

// productionLikeBackend reproduces the two relevant B2 listing contracts:
// public List hides .armor/ objects and does not perform per-object HEAD calls,
// while ListRaw exposes internal objects to trusted subsystems.
type productionLikeBackend struct {
	*mockBackend
}

func (m *productionLikeBackend) List(ctx context.Context, bucket, prefix, delimiter, continuationToken string, maxKeys int) (*backend.ListResult, error) {
	result, err := m.mockBackend.List(ctx, bucket, prefix, delimiter, continuationToken, maxKeys)
	if err != nil {
		return nil, err
	}
	filtered := result.Objects[:0]
	for _, object := range result.Objects {
		if strings.HasPrefix(object.Key, ".armor/") {
			continue
		}
		object.Metadata = nil
		object.IsARMOREncrypted = false
		filtered = append(filtered, object)
	}
	result.Objects = filtered
	return result, nil
}

func (m *productionLikeBackend) ListRaw(ctx context.Context, bucket, prefix, delimiter, continuationToken string, maxKeys int) (*backend.ListResult, error) {
	return m.mockBackend.List(ctx, bucket, prefix, delimiter, continuationToken, maxKeys)
}

type failingListBackend struct {
	*mockBackend
	failPrefix string
}

func (m *failingListBackend) List(ctx context.Context, bucket, prefix, delimiter, continuationToken string, maxKeys int) (*backend.ListResult, error) {
	if prefix == m.failPrefix {
		return nil, fmt.Errorf("injected listing failure for %q", prefix)
	}
	return m.mockBackend.List(ctx, bucket, prefix, delimiter, continuationToken, maxKeys)
}

type omittingEntryListBackend struct {
	*mockBackend
}

func (m *omittingEntryListBackend) List(ctx context.Context, bucket, prefix, delimiter, continuationToken string, maxKeys int) (*backend.ListResult, error) {
	result, err := m.mockBackend.List(ctx, bucket, prefix, delimiter, continuationToken, maxKeys)
	if err == nil && prefix == ChainPrefix {
		result.Objects = nil
	}
	return result, err
}

func TestShouldRecord(t *testing.T) {
	m := NewManager(nil, "test-bucket", "test-writer")

	tests := []struct {
		key      string
		expected bool
	}{
		{"data/file.txt", true},
		{".armor/chain/test/1.json", false},
		{".armor/canary/test", false},
		{"regular/object.parquet", true},
		{".armor/rotation-state.json", false},
	}

	for _, tt := range tests {
		result := m.ShouldRecord(tt.key)
		if result != tt.expected {
			t.Errorf("ShouldRecord(%q) = %v, want %v", tt.key, result, tt.expected)
		}
	}
}

func TestComputeChainHash(t *testing.T) {
	entry := &Entry{
		Sequence:        1,
		ObjectKey:       "test/object.txt",
		PlaintextSHA256: "abc123",
		Timestamp:       time.Date(2026, 3, 24, 12, 0, 0, 0, time.UTC),
		WriterID:        "test-writer",
	}

	hash := computeChainHash(entry, InitialChainHash)

	// Verify hash is 64 hex characters
	if len(hash) != 64 {
		t.Errorf("chain hash length = %d, want 64", len(hash))
	}

	// Verify deterministic - same inputs should produce same hash
	hash2 := computeChainHash(entry, InitialChainHash)
	if hash != hash2 {
		t.Error("chain hash should be deterministic")
	}

	// Different prev hash should produce different result
	hash3 := computeChainHash(entry, "different-prev-hash")
	if hash == hash3 {
		t.Error("chain hash should change with different prev hash")
	}
}

func TestRecordUpload(t *testing.T) {
	ctx := context.Background()
	mb := newMockBackend()
	m := NewManager(mb, "test-bucket", "test-writer")

	// Record first upload
	plaintextSHA := fmt.Sprintf("%x", sha256.Sum256([]byte("test content")))
	err := m.RecordUpload(ctx, "data/file1.txt", plaintextSHA, "put")
	if err != nil {
		t.Fatalf("RecordUpload failed: %v", err)
	}

	// Verify chain head was saved
	head, err := m.loadHead(ctx)
	if err != nil {
		t.Fatalf("loadHead failed: %v", err)
	}
	if head.Sequence != 1 {
		t.Errorf("head sequence = %d, want 1", head.Sequence)
	}
	if head.WriterID != "test-writer" {
		t.Errorf("head writer ID = %s, want test-writer", head.WriterID)
	}

	// Record second upload
	err = m.RecordUpload(ctx, "data/file2.txt", plaintextSHA, "put")
	if err != nil {
		t.Fatalf("RecordUpload failed: %v", err)
	}

	head, err = m.loadHead(ctx)
	if err != nil {
		t.Fatalf("loadHead failed: %v", err)
	}
	if head.Sequence != 2 {
		t.Errorf("head sequence = %d, want 2", head.Sequence)
	}

	// Verify chain entry was saved
	entryKey := fmt.Sprintf("%s%s/%d.json", ChainPrefix, "test-writer", 1)
	entryData, ok := mb.objects["test-bucket/"+entryKey]
	if !ok {
		t.Fatal("chain entry not saved")
	}

	var entry Entry
	if err := json.Unmarshal(entryData, &entry); err != nil {
		t.Fatalf("failed to unmarshal entry: %v", err)
	}
	if entry.Sequence != 1 {
		t.Errorf("entry sequence = %d, want 1", entry.Sequence)
	}
	if entry.ObjectKey != "data/file1.txt" {
		t.Errorf("entry object key = %s, want data/file1.txt", entry.ObjectKey)
	}
	if entry.PrevChainHash != InitialChainHash {
		t.Errorf("entry prev chain hash = %s, want %s", entry.PrevChainHash, InitialChainHash)
	}
}

func TestAudit(t *testing.T) {
	ctx := context.Background()
	mb := newMockBackend()
	m := NewManager(mb, "test-bucket", "test-writer")

	// Record some uploads
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("data/file%d.txt", i)
		plaintextSHA := fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("content %d", i))))
		err := m.RecordUpload(ctx, key, plaintextSHA, "put")
		if err != nil {
			t.Fatalf("RecordUpload failed: %v", err)
		}
		// Add ARMOR metadata to the tracked objects
		meta := map[string]string{
			"x-amz-meta-armor-version": "1",
		}
		mb.metadata["test-bucket/"+key] = meta
	}

	// Perform audit
	auditor := NewAuditor(mb, "test-bucket")
	result, err := auditor.Audit(ctx)
	if err != nil {
		t.Fatalf("Audit failed: %v", err)
	}

	if result.Status != "valid" {
		t.Errorf("audit status = %s, want valid", result.Status)
	}

	if len(result.Writers) != 1 {
		t.Errorf("expected 1 writer, got %d", len(result.Writers))
	}

	if result.Writers[0].WriterID != "test-writer" {
		t.Errorf("writer ID = %s, want test-writer", result.Writers[0].WriterID)
	}

	if result.Writers[0].EntriesVerified != 5 {
		t.Errorf("entries verified = %d, want 5", result.Writers[0].EntriesVerified)
	}

	if result.TotalEntries != 5 {
		t.Errorf("total entries = %d, want 5", result.TotalEntries)
	}
}

func TestAuditUntrackedObjects(t *testing.T) {
	ctx := context.Background()
	mb := newMockBackend()

	// Add an ARMOR-encrypted object without provenance
	mb.objects["test-bucket/data/untracked.txt"] = []byte("content")
	mb.metadata["test-bucket/data/untracked.txt"] = map[string]string{
		"x-amz-meta-armor-version": "1",
	}

	// Perform audit
	auditor := NewAuditor(mb, "test-bucket")
	result, err := auditor.Audit(ctx)
	if err != nil {
		t.Fatalf("Audit failed: %v", err)
	}

	if result.Status != "invalid" {
		t.Errorf("audit status = %s, want invalid", result.Status)
	}

	if len(result.UntrackedObjects) != 1 {
		t.Errorf("expected 1 untracked object, got %d", len(result.UntrackedObjects))
	}

	if result.UntrackedObjects[0] != "data/untracked.txt" {
		t.Errorf("untracked object = %s, want data/untracked.txt", result.UntrackedObjects[0])
	}
}

// TestAuditWithKeyEvent tests that the audit can handle chains containing
// both Entry and KeyEvent objects (e.g., key rotation events).
//
// Verifies that:
// 1. Key events are not counted as upload entries
// 2. Key events are counted separately in KeyEvents field
// 3. Chain integrity is maintained across mixed entry types
func TestAuditWithKeyEvent(t *testing.T) {
	ctx := context.Background()
	mb := newMockBackend()
	m := NewManager(mb, "test-bucket", "test-writer")

	// Record an upload
	plaintextSHA := fmt.Sprintf("%x", sha256.Sum256([]byte("test content")))
	err := m.RecordUpload(ctx, "data/file1.txt", plaintextSHA, "put")
	if err != nil {
		t.Fatalf("RecordUpload failed: %v", err)
	}

	// Record a key event (simulating a key rotation)
	err = m.RecordKeyEvent(ctx, "key-rotate-start", KeyEventOpts{
		OldMEKHash: "aaaa1111bbbb2222",
		NewMEKHash: "cccc3333dddd4444",
		RotationID: "test-rotation-1",
	})
	if err != nil {
		t.Fatalf("RecordKeyEvent failed: %v", err)
	}

	// Add ARMOR metadata to the tracked object
	meta := map[string]string{
		"x-amz-meta-armor-version": "1",
	}
	mb.metadata["test-bucket/data/file1.txt"] = meta

	// Perform audit
	auditor := NewAuditor(mb, "test-bucket")
	result, err := auditor.Audit(ctx)
	if err != nil {
		t.Fatalf("Audit failed: %v", err)
	}

	// Verify overall status
	if result.Status != "valid" {
		t.Errorf("audit status = %s, want valid", result.Status)
	}

	if len(result.Writers) != 1 {
		t.Fatalf("expected 1 writer, got %d", len(result.Writers))
	}

	// CRITICAL: Key events should NOT be counted as upload entries
	if result.Writers[0].EntriesVerified != 1 {
		t.Errorf("EntriesVerified = %d, want 1 (key events shouldn't count as uploads)",
			result.Writers[0].EntriesVerified)
	}

	// CRITICAL: Key events should be counted separately
	if result.Writers[0].KeyEvents != 1 {
		t.Errorf("KeyEvents = %d, want 1", result.Writers[0].KeyEvents)
	}

	// Total entries should only count uploads, not key events
	if result.TotalEntries != 1 {
		t.Errorf("TotalEntries = %d, want 1 (key events shouldn't count)",
			result.TotalEntries)
	}

	// Verify the chain is valid (hashes link correctly)
	if !result.Writers[0].Valid {
		t.Error("Writer chain should be valid even with key events")
	}

	// Verify head sequence is 2 (upload + key event)
	if result.Writers[0].HeadSequence != 2 {
		t.Errorf("HeadSequence = %d, want 2", result.Writers[0].HeadSequence)
	}
}

// TestAuditBrokenChainHash tests that audit detects broken chain hash links.
func TestAuditBrokenChainHash(t *testing.T) {
	ctx := context.Background()
	mb := newMockBackend()
	m := NewManager(mb, "test-bucket", "test-writer")

	// Record two uploads
	plaintextSHA := fmt.Sprintf("%x", sha256.Sum256([]byte("test content")))
	err := m.RecordUpload(ctx, "data/file1.txt", plaintextSHA, "put")
	if err != nil {
		t.Fatalf("RecordUpload failed: %v", err)
	}

	err = m.RecordUpload(ctx, "data/file2.txt", plaintextSHA, "put")
	if err != nil {
		t.Fatalf("RecordUpload failed: %v", err)
	}

	// Tamper with the chain hash of entry 1
	entryKey := fmt.Sprintf("%s%s/%d.json", ChainPrefix, "test-writer", 1)
	entryData := mb.objects["test-bucket/"+entryKey]
	var entry Entry
	if err := json.Unmarshal(entryData, &entry); err != nil {
		t.Fatalf("Failed to unmarshal entry: %v", err)
	}
	// Corrupt the chain hash
	entry.ChainHash = "corruptedhash1234567890123456789012345678901234567890123456789012345678"
	corruptedData, _ := json.Marshal(entry)
	mb.objects["test-bucket/"+entryKey] = corruptedData

	// Perform audit
	auditor := NewAuditor(mb, "test-bucket")
	result, err := auditor.Audit(ctx)
	if err != nil {
		t.Fatalf("Audit failed: %v", err)
	}

	if result.Status != "invalid" {
		t.Errorf("audit status = %s, want invalid", result.Status)
	}

	if len(result.Writers) != 1 {
		t.Fatalf("expected 1 writer, got %d", len(result.Writers))
	}

	if result.Writers[0].Valid {
		t.Error("expected writer audit to be invalid due to broken chain hash")
	}

	if result.Writers[0].Error == "" {
		t.Error("expected error message for broken chain hash")
	}
}

// TestAuditMissingEntry tests that audit detects missing entries in the chain.
func TestAuditMissingEntry(t *testing.T) {
	ctx := context.Background()
	mb := newMockBackend()
	m := NewManager(mb, "test-bucket", "test-writer")

	// Record two uploads
	plaintextSHA := fmt.Sprintf("%x", sha256.Sum256([]byte("test content")))
	err := m.RecordUpload(ctx, "data/file1.txt", plaintextSHA, "put")
	if err != nil {
		t.Fatalf("RecordUpload failed: %v", err)
	}

	err = m.RecordUpload(ctx, "data/file2.txt", plaintextSHA, "put")
	if err != nil {
		t.Fatalf("RecordUpload failed: %v", err)
	}

	// Delete entry 1 to create a gap
	entryKey := fmt.Sprintf("%s%s/%d.json", ChainPrefix, "test-writer", 1)
	delete(mb.objects, "test-bucket/"+entryKey)

	// Perform audit
	auditor := NewAuditor(mb, "test-bucket")
	result, err := auditor.Audit(ctx)
	if err != nil {
		t.Fatalf("Audit failed: %v", err)
	}

	if result.Status != "invalid" {
		t.Errorf("audit status = %s, want invalid", result.Status)
	}

	if len(result.Writers) != 1 {
		t.Fatalf("expected 1 writer, got %d", len(result.Writers))
	}

	if result.Writers[0].Valid {
		t.Error("expected writer audit to be invalid due to missing entry")
	}

	if result.Writers[0].Error == "" {
		t.Error("expected error message for missing entry")
	}

	if len(result.Gaps) != 1 || result.Gaps[0].WriterID != "test-writer" || result.Gaps[0].MissingSeq != 1 {
		t.Fatalf("expected sequence 1 gap, got %+v", result.Gaps)
	}
}

// TestAuditGenesisLink tests that audit verifies chain links back to genesis.
func TestAuditGenesisLink(t *testing.T) {
	ctx := context.Background()
	mb := newMockBackend()
	m := NewManager(mb, "test-bucket", "test-writer")

	// Record an upload
	plaintextSHA := fmt.Sprintf("%x", sha256.Sum256([]byte("test content")))
	err := m.RecordUpload(ctx, "data/file1.txt", plaintextSHA, "put")
	if err != nil {
		t.Fatalf("RecordUpload failed: %v", err)
	}

	// Tamper with the first entry's prev chain hash to break genesis link
	entryKey := fmt.Sprintf("%s%s/%d.json", ChainPrefix, "test-writer", 1)
	entryData := mb.objects["test-bucket/"+entryKey]
	var entry Entry
	if err := json.Unmarshal(entryData, &entry); err != nil {
		t.Fatalf("Failed to unmarshal entry: %v", err)
	}
	// Set to non-initial hash
	entry.PrevChainHash = "brokenlink123456789012345678901234567890123456789012345678901234"
	corruptedData, _ := json.Marshal(entry)
	mb.objects["test-bucket/"+entryKey] = corruptedData

	// Perform audit
	auditor := NewAuditor(mb, "test-bucket")
	result, err := auditor.Audit(ctx)
	if err != nil {
		t.Fatalf("Audit failed: %v", err)
	}

	if result.Status != "invalid" {
		t.Errorf("audit status = %s, want invalid", result.Status)
	}

	if len(result.Writers) != 1 {
		t.Fatalf("expected 1 writer, got %d", len(result.Writers))
	}

	if result.Writers[0].Valid {
		t.Error("expected writer audit to be invalid due to broken genesis link")
	}

	if result.Writers[0].Error == "" {
		t.Error("expected error message for broken genesis link")
	}
}

// TestAuditMultipleWriters tests audit with multiple independent writer chains.
func TestAuditMultipleWriters(t *testing.T) {
	ctx := context.Background()
	mb := newMockBackend()

	// Create two writers
	m1 := NewManager(mb, "test-bucket", "writer-1")
	m2 := NewManager(mb, "test-bucket", "writer-2")

	// Writer 1 records 3 uploads
	plaintextSHA := fmt.Sprintf("%x", sha256.Sum256([]byte("content 1")))
	for i := 0; i < 3; i++ {
		key := fmt.Sprintf("data/writer1/file%d.txt", i)
		err := m1.RecordUpload(ctx, key, plaintextSHA, "put")
		if err != nil {
			t.Fatalf("Writer-1 RecordUpload failed: %v", err)
		}
		// Add ARMOR metadata
		meta := map[string]string{"x-amz-meta-armor-version": "1"}
		mb.metadata["test-bucket/"+key] = meta
	}

	// Writer 2 records 2 uploads
	for i := 0; i < 2; i++ {
		key := fmt.Sprintf("data/writer2/file%d.txt", i)
		err := m2.RecordUpload(ctx, key, plaintextSHA, "put")
		if err != nil {
			t.Fatalf("Writer-2 RecordUpload failed: %v", err)
		}
		meta := map[string]string{"x-amz-meta-armor-version": "1"}
		mb.metadata["test-bucket/"+key] = meta
	}

	// Perform audit
	auditor := NewAuditor(mb, "test-bucket")
	result, err := auditor.Audit(ctx)
	if err != nil {
		t.Fatalf("Audit failed: %v", err)
	}

	if result.Status != "valid" {
		t.Errorf("audit status = %s, want valid", result.Status)
	}

	if len(result.Writers) != 2 {
		t.Errorf("expected 2 writers, got %d", len(result.Writers))
	}

	if result.TotalEntries != 5 {
		t.Errorf("total entries = %d, want 5", result.TotalEntries)
	}

	// Verify each writer's counts
	writerCounts := make(map[string]int)
	for _, w := range result.Writers {
		writerCounts[w.WriterID] = w.EntriesVerified
	}

	if writerCounts["writer-1"] != 3 {
		t.Errorf("writer-1 entries = %d, want 3", writerCounts["writer-1"])
	}

	if writerCounts["writer-2"] != 2 {
		t.Errorf("writer-2 entries = %d, want 2", writerCounts["writer-2"])
	}
}

func TestAuditUsesInternalListingAndAuthoritativeMetadata(t *testing.T) {
	ctx := context.Background()
	mb := &productionLikeBackend{mockBackend: newMockBackend()}
	m := NewManager(mb, "test-bucket", "test-writer")
	plaintextSHA := fmt.Sprintf("%x", sha256.Sum256([]byte("content")))

	if err := m.RecordUpload(ctx, "data/file.txt", plaintextSHA, "put"); err != nil {
		t.Fatalf("RecordUpload failed: %v", err)
	}
	mb.objects["test-bucket/data/file.txt"] = []byte("encrypted content")
	mb.metadata["test-bucket/data/file.txt"] = map[string]string{
		"x-amz-meta-armor-version": "1",
	}

	result, err := NewAuditor(mb, "test-bucket").Audit(ctx)
	if err != nil {
		t.Fatalf("Audit failed: %v", err)
	}
	if result.Status != "valid" {
		t.Fatalf("audit status = %s, want valid: %+v", result.Status, result)
	}
	if len(result.Writers) != 1 || result.Writers[0].EntriesVerified != 1 {
		t.Fatalf("audit did not discover and verify internal chain: %+v", result.Writers)
	}
	if result.TotalObjects != 1 {
		t.Fatalf("total objects = %d, want 1", result.TotalObjects)
	}

	// Public B2-style listings do not identify encryption metadata. Confirm the
	// auditor's authoritative HEAD detects an encrypted object with no entry.
	mb.objects["test-bucket/data/untracked.txt"] = []byte("encrypted content")
	mb.metadata["test-bucket/data/untracked.txt"] = map[string]string{
		"x-amz-meta-armor-version": "1",
	}
	result, err = NewAuditor(mb, "test-bucket").Audit(ctx)
	if err != nil {
		t.Fatalf("Audit with untracked object failed: %v", err)
	}
	if result.Status != "invalid" || len(result.UntrackedObjects) != 1 || result.UntrackedObjects[0] != "data/untracked.txt" {
		t.Fatalf("metadata-free listing hid untracked object: %+v", result)
	}
}

func TestAuditRecomputesHeadEntryHash(t *testing.T) {
	ctx := context.Background()
	mb := newMockBackend()
	m := NewManager(mb, "test-bucket", "test-writer")
	plaintextSHA := fmt.Sprintf("%x", sha256.Sum256([]byte("content")))

	if err := m.RecordUpload(ctx, "data/file.txt", plaintextSHA, "put"); err != nil {
		t.Fatalf("RecordUpload failed: %v", err)
	}
	entryKey := "test-bucket/" + ChainPrefix + "test-writer/1.json"
	var entry Entry
	if err := json.Unmarshal(mb.objects[entryKey], &entry); err != nil {
		t.Fatalf("unmarshal entry: %v", err)
	}
	entry.PlaintextSHA256 = strings.Repeat("a", sha256.Size*2)
	mb.objects[entryKey], _ = json.Marshal(entry)

	result, err := NewAuditor(mb, "test-bucket").Audit(ctx)
	if err != nil {
		t.Fatalf("Audit failed: %v", err)
	}
	if result.Status != "invalid" || len(result.Writers) != 1 || result.Writers[0].Valid {
		t.Fatalf("tampered head entry passed audit: %+v", result)
	}
	if !strings.Contains(result.Writers[0].Error, "entry hash mismatch") {
		t.Fatalf("unexpected audit error: %q", result.Writers[0].Error)
	}
}

func TestAuditVerifiesHeadHashAgainstTip(t *testing.T) {
	ctx := context.Background()
	mb := newMockBackend()
	m := NewManager(mb, "test-bucket", "test-writer")
	plaintextSHA := fmt.Sprintf("%x", sha256.Sum256([]byte("content")))

	if err := m.RecordUpload(ctx, "data/file.txt", plaintextSHA, "put"); err != nil {
		t.Fatalf("RecordUpload failed: %v", err)
	}
	headKey := "test-bucket/" + ChainHeadPrefix + "test-writer"
	var head ChainHead
	if err := json.Unmarshal(mb.objects[headKey], &head); err != nil {
		t.Fatalf("unmarshal head: %v", err)
	}
	head.ChainHash = strings.Repeat("b", sha256.Size*2)
	mb.objects[headKey], _ = json.Marshal(head)

	result, err := NewAuditor(mb, "test-bucket").Audit(ctx)
	if err != nil {
		t.Fatalf("Audit failed: %v", err)
	}
	if result.Status != "invalid" || result.Writers[0].Valid {
		t.Fatalf("head hash mismatch passed audit: %+v", result)
	}
	if !strings.Contains(result.Writers[0].Error, "chain link mismatch") {
		t.Fatalf("unexpected audit error: %q", result.Writers[0].Error)
	}
}

func TestAuditRecomputesKeyEventHash(t *testing.T) {
	ctx := context.Background()
	mb := newMockBackend()
	m := NewManager(mb, "test-bucket", "test-writer")

	if err := m.RecordKeyEvent(ctx, "key-rotate-start", KeyEventOpts{
		OldMEKHash: "aaaa1111bbbb2222",
		NewMEKHash: "cccc3333dddd4444",
		RotationID: "rotation-1",
	}); err != nil {
		t.Fatalf("RecordKeyEvent failed: %v", err)
	}
	eventKey := "test-bucket/" + ChainPrefix + "test-writer/1.json"
	var event KeyEvent
	if err := json.Unmarshal(mb.objects[eventKey], &event); err != nil {
		t.Fatalf("unmarshal key event: %v", err)
	}
	event.RotationID = "tampered-rotation"
	mb.objects[eventKey], _ = json.Marshal(event)

	result, err := NewAuditor(mb, "test-bucket").Audit(ctx)
	if err != nil {
		t.Fatalf("Audit failed: %v", err)
	}
	if result.Status != "invalid" || result.Writers[0].Valid {
		t.Fatalf("tampered key event passed audit: %+v", result)
	}
	if !strings.Contains(result.Writers[0].Error, "key event hash mismatch") {
		t.Fatalf("unexpected audit error: %q", result.Writers[0].Error)
	}
}

func TestAuditRejectsMalformedHeadInsteadOfSkippingIt(t *testing.T) {
	ctx := context.Background()
	mb := newMockBackend()
	mb.objects["test-bucket/"+ChainHeadPrefix+"test-writer"] = []byte("not-json")

	result, err := NewAuditor(mb, "test-bucket").Audit(ctx)
	if err != nil {
		t.Fatalf("Audit failed: %v", err)
	}
	if result.Status != "invalid" || len(result.Writers) != 1 || result.Writers[0].Valid {
		t.Fatalf("malformed head was silently skipped: %+v", result)
	}
}

func TestAuditDetectsEntriesWithoutHead(t *testing.T) {
	ctx := context.Background()
	mb := newMockBackend()
	m := NewManager(mb, "test-bucket", "test-writer")
	plaintextSHA := fmt.Sprintf("%x", sha256.Sum256([]byte("content")))

	if err := m.RecordUpload(ctx, "data/file.txt", plaintextSHA, "put"); err != nil {
		t.Fatalf("RecordUpload failed: %v", err)
	}
	delete(mb.objects, "test-bucket/"+ChainHeadPrefix+"test-writer")

	result, err := NewAuditor(mb, "test-bucket").Audit(ctx)
	if err != nil {
		t.Fatalf("Audit failed: %v", err)
	}
	if result.Status != "incomplete" || len(result.Writers) != 1 || result.Writers[0].Valid {
		t.Fatalf("orphan chain entries did not make the audit incomplete: %+v", result)
	}
	if !strings.Contains(result.Writers[0].Error, "without a chain head") {
		t.Fatalf("unexpected audit error: %q", result.Writers[0].Error)
	}
}

func TestAuditReportsIncompleteWhenListingsFail(t *testing.T) {
	for _, failPrefix := range []string{ChainHeadPrefix, ChainPrefix, ""} {
		t.Run(fmt.Sprintf("prefix_%q", failPrefix), func(t *testing.T) {
			mb := &failingListBackend{mockBackend: newMockBackend(), failPrefix: failPrefix}
			result, err := NewAuditor(mb, "test-bucket").Audit(context.Background())
			if err != nil {
				t.Fatalf("Audit failed: %v", err)
			}
			if result.Status != "incomplete" {
				t.Fatalf("audit status = %q, want incomplete: %+v", result.Status, result)
			}
			if len(result.Errors) == 0 {
				t.Fatal("incomplete audit did not report its error")
			}
		})
	}
}

func TestAuditFetchesEntriesAddedAfterListingSnapshot(t *testing.T) {
	ctx := context.Background()
	mb := &omittingEntryListBackend{mockBackend: newMockBackend()}
	m := NewManager(mb, "test-bucket", "test-writer")
	plaintextSHA := fmt.Sprintf("%x", sha256.Sum256([]byte("content")))
	if err := m.RecordUpload(ctx, "data/file.txt", plaintextSHA, "put"); err != nil {
		t.Fatalf("RecordUpload failed: %v", err)
	}

	result, err := NewAuditor(mb, "test-bucket").Audit(ctx)
	if err != nil {
		t.Fatalf("Audit failed: %v", err)
	}
	if result.Status != "valid" || len(result.Writers) != 1 || !result.Writers[0].Valid {
		t.Fatalf("entry omitted from listing but reachable from head did not verify: %+v", result)
	}
}

func TestAuditReportsEntryBeyondHeadAsIncomplete(t *testing.T) {
	ctx := context.Background()
	mb := newMockBackend()
	m := NewManager(mb, "test-bucket", "test-writer")
	plaintextSHA := fmt.Sprintf("%x", sha256.Sum256([]byte("content")))
	if err := m.RecordUpload(ctx, "data/file-1.txt", plaintextSHA, "put"); err != nil {
		t.Fatalf("first RecordUpload failed: %v", err)
	}
	firstHead := append([]byte(nil), mb.objects["test-bucket/"+ChainHeadPrefix+"test-writer"]...)
	if err := m.RecordUpload(ctx, "data/file-2.txt", plaintextSHA, "put"); err != nil {
		t.Fatalf("second RecordUpload failed: %v", err)
	}
	mb.objects["test-bucket/"+ChainHeadPrefix+"test-writer"] = firstHead

	result, err := NewAuditor(mb, "test-bucket").Audit(ctx)
	if err != nil {
		t.Fatalf("Audit failed: %v", err)
	}
	if result.Status != "incomplete" || len(result.Writers) != 1 || result.Writers[0].Valid {
		t.Fatalf("entry beyond head did not make audit incomplete: %+v", result)
	}
	if !strings.Contains(result.Writers[0].Error, "beyond head sequence") {
		t.Fatalf("unexpected audit error: %q", result.Writers[0].Error)
	}
}

func TestManagerSerializesConcurrentAppends(t *testing.T) {
	ctx := context.Background()
	mb := newMockBackend()
	m := NewManager(mb, "test-bucket", "test-writer")
	const uploads = 64

	var wg sync.WaitGroup
	errCh := make(chan error, uploads)
	for i := 0; i < uploads; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("data/file-%d.txt", i)
			plaintextSHA := fmt.Sprintf("%x", sha256.Sum256([]byte(key)))
			if err := m.RecordUpload(ctx, key, plaintextSHA, "put"); err != nil {
				errCh <- err
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("concurrent RecordUpload failed: %v", err)
	}

	result, err := NewAuditor(mb, "test-bucket").Audit(ctx)
	if err != nil {
		t.Fatalf("Audit failed: %v", err)
	}
	if result.Status != "valid" || len(result.Writers) != 1 {
		t.Fatalf("concurrent chain is invalid: %+v", result)
	}
	if result.Writers[0].HeadSequence != uploads || result.Writers[0].EntriesVerified != uploads {
		t.Fatalf("concurrent chain lost entries: %+v", result.Writers[0])
	}
}

// TestWalkChainSegments tests the walkChainSegments method.
func TestWalkChainSegments(t *testing.T) {
	ctx := context.Background()
	mb := newMockBackend()
	auditor := NewAuditor(mb, "test-bucket")

	// Create some chain segment files
	writerID := "test-writer"
	segment1 := &Entry{
		Sequence:        1,
		ObjectKey:       "file1.txt",
		PlaintextSHA256: fmt.Sprintf("%x", sha256.Sum256([]byte("data1"))),
		ChainHash:       strings.Repeat("a", sha256.Size*2),
		PrevChainHash:   InitialChainHash,
		Timestamp:       time.Now().UTC(),
		WriterID:        writerID,
		Operation:       "put",
	}
	segment2 := &Entry{
		Sequence:        2,
		ObjectKey:       "file2.txt",
		PlaintextSHA256: fmt.Sprintf("%x", sha256.Sum256([]byte("data2"))),
		ChainHash:       strings.Repeat("b", sha256.Size*2),
		PrevChainHash:   strings.Repeat("a", sha256.Size*2),
		Timestamp:       time.Now().UTC(),
		WriterID:        writerID,
		Operation:       "put",
	}

	// Create a JSONL segment file
	var jsonlLines []string
	for _, entry := range []*Entry{segment1, segment2} {
		line, err := json.Marshal(entry)
		if err != nil {
			t.Fatalf("marshal entry: %v", err)
		}
		jsonlLines = append(jsonlLines, string(line))
	}
	segmentJSONL := []byte(strings.Join(jsonlLines, "\n") + "\n")

	// Put the segment file
	segmentKey := fmt.Sprintf(".armor/chain-segments/%s/1-2.jsonl", writerID)
	if err := mb.Put(ctx, "test-bucket", segmentKey, bytes.NewReader(segmentJSONL), int64(len(segmentJSONL)), map[string]string{"Content-Type": "application/jsonl"}); err != nil {
		t.Fatalf("put segment file: %v", err)
	}

	// Walk the chain segments
	entries, trackedObjects, err := auditor.walkChainSegments(ctx, writerID, 1, 2)
	if err != nil {
		t.Fatalf("walkChainSegments failed: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[1] == nil || entries[2] == nil {
		t.Fatalf("expected entries for sequences 1 and 2, got %+v", entries)
	}
	if len(trackedObjects) != 2 {
		t.Fatalf("expected 2 tracked objects, got %d", len(trackedObjects))
	}
	if !trackedObjects["file1.txt"] || !trackedObjects["file2.txt"] {
		t.Fatalf("tracked objects mismatch: %+v", trackedObjects)
	}
}

// TestWalkDeltaEntries tests the walkDeltaEntries method.
func TestWalkDeltaEntries(t *testing.T) {
	ctx := context.Background()
	mb := newMockBackend()
	auditor := NewAuditor(mb, "test-bucket")

	writerID := "test-writer"

	// Create some delta operations with chain entries
	deltaOps := []struct {
		op      string
		key     string
		chain   *ChainEntryData
	}{
		{
			op:  "put",
			key: "bucket/file1.txt",
			chain: &ChainEntryData{
				Sequence:      1,
				ChainHash:     strings.Repeat("a", sha256.Size*2),
				PrevChainHash: InitialChainHash,
			},
		},
		{
			op:  "put",
			key: "bucket/file2.txt",
			chain: &ChainEntryData{
				Sequence:      2,
				ChainHash:     strings.Repeat("b", sha256.Size*2),
				PrevChainHash: strings.Repeat("a", sha256.Size*2),
			},
		},
		{
			op:  "del",
			key: "bucket/file3.txt",
		},
	}

	// Create a JSONL delta file
	var jsonlLines []string
	for _, deltaOp := range deltaOps {
		op := struct {
			Operation string          `json:"op"`
			Key       string          `json:"key"`
			Chain     *ChainEntryData `json:"chain,omitempty"`
		}{
			Operation: deltaOp.op,
			Key:       deltaOp.key,
			Chain:     deltaOp.chain,
		}
		line, err := json.Marshal(op)
		if err != nil {
			t.Fatalf("marshal delta op: %v", err)
		}
		jsonlLines = append(jsonlLines, string(line))
	}
	deltaJSONL := []byte(strings.Join(jsonlLines, "\n") + "\n")

	// Put the delta file
	deltaKey := fmt.Sprintf(".armor/manifest/%s/delta-0000000001.jsonl", writerID)
	if err := mb.Put(ctx, "test-bucket", deltaKey, bytes.NewReader(deltaJSONL), int64(len(deltaJSONL)), map[string]string{"Content-Type": "application/jsonl"}); err != nil {
		t.Fatalf("put delta file: %v", err)
	}

	// Walk the delta entries
	entries, trackedObjects, err := auditor.walkDeltaEntries(ctx, writerID, 1)
	if err != nil {
		t.Fatalf("walkDeltaEntries failed: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 chain entries, got %d", len(entries))
	}
	if entries[1] == nil || entries[2] == nil {
		t.Fatalf("expected entries for sequences 1 and 2, got %+v", entries)
	}
	if len(trackedObjects) != 2 {
		t.Fatalf("expected 2 tracked objects, got %d", len(trackedObjects))
	}
	if !trackedObjects["file1.txt"] || !trackedObjects["file2.txt"] {
		t.Fatalf("tracked objects mismatch: %+v", trackedObjects)
	}
}

// TestChainHeadFormatDetection tests that the audit walker correctly
// identifies legacy vs manifest format chain heads.
func TestChainHeadFormatDetection(t *testing.T) {
	// Legacy format chain head
	legacyHead := &ChainHead{
		WriterID:  "test-writer",
		Sequence:  10,
		ChainHash: strings.Repeat("a", sha256.Size*2),
		Updated:   time.Now().UTC(),
	}

	if legacyHead.WriterID == "" {
		t.Fatal("legacy head should have WriterID")
	}
	if legacyHead.DeltaFile != "" {
		t.Fatal("legacy head should not have DeltaFile")
	}

	// Manifest format chain head
	manifestHead := &ChainHead{
		DeltaFile: ".armor/manifest/test-writer/delta-0000000001.jsonl",
		Sequence:  10,
		ChainHash: strings.Repeat("b", sha256.Size*2),
	}

	if manifestHead.DeltaFile == "" {
		t.Fatal("manifest head should have DeltaFile")
	}
	if manifestHead.WriterID != "" {
		t.Fatal("manifest head should not have WriterID")
	}
}

// TestCompactLegacyChain tests compaction of legacy chain entries into segments.
// This test creates 1,000 entries, compacts them, and verifies the audit can read from the segment.
func TestCompactLegacyChain(t *testing.T) {
	ctx := context.Background()
	backend := newMockBackend()
	manager := NewManager(backend, "test-bucket", "test-writer")

	// Create 1,000 legacy chain entries
	const numEntries = 1000
	prevHash := InitialChainHash

	for i := int64(1); i <= numEntries; i++ {
		// Create and save a chain entry
		entry := &Entry{
			Sequence:        i,
			ObjectKey:       fmt.Sprintf("object-%04d.txt", i),
			PlaintextSHA256: fmt.Sprintf("%064x", i),
			PrevChainHash:   prevHash,
			Timestamp:       time.Now().UTC(),
			WriterID:        "test-writer",
			Operation:       "put",
		}

		// Compute chain hash
		entry.ChainHash = computeChainHash(entry, prevHash)
		prevHash = entry.ChainHash

		// Save as a legacy entry (individual JSON file)
		key := fmt.Sprintf("%s%s/%d.json", ChainPrefix, "test-writer", i)
		data, err := json.MarshalIndent(entry, "", "  ")
		if err != nil {
			t.Fatalf("failed to marshal entry %d: %v", i, err)
		}

		if err := backend.Put(ctx, "test-bucket", key, bytes.NewReader(data), int64(len(data)), map[string]string{
			"Content-Type": "application/json",
		}); err != nil {
			t.Fatalf("failed to save entry %d: %v", i, err)
		}
	}

	// Create chain head
	head := &ChainHead{
		WriterID:  "test-writer",
		Sequence:  numEntries,
		ChainHash: prevHash,
		Updated:   time.Now().UTC(),
	}
	headKey := fmt.Sprintf("%s%s", ChainHeadPrefix, "test-writer")
	headData, err := json.MarshalIndent(head, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal head: %v", err)
	}
	if err := backend.Put(ctx, "test-bucket", headKey, bytes.NewReader(headData), int64(len(headData)), map[string]string{
		"Content-Type": "application/json",
	}); err != nil {
		t.Fatalf("failed to save head: %v", err)
	}

	// Perform compaction
	result, err := manager.CompactLegacyChain(ctx, "test-writer")
	if err != nil {
		t.Fatalf("CompactLegacyChain failed: %v", err)
	}

	// Verify compaction result
	if result.EntryCount != numEntries {
		t.Errorf("expected %d entries, got %d", numEntries, result.EntryCount)
	}
	if result.FromSequence != 1 {
		t.Errorf("expected FromSequence 1, got %d", result.FromSequence)
	}
	if result.ToSequence != numEntries {
		t.Errorf("expected ToSequence %d, got %d", numEntries, result.ToSequence)
	}
	expectedPath := fmt.Sprintf("%s%s/%010d-%010d.jsonl", ChainSegmentPrefix, "test-writer", 1, numEntries)
	if result.SegmentPath != expectedPath {
		t.Errorf("expected segment path %s, got %s", expectedPath, result.SegmentPath)
	}

	// Verify the segment file exists
	segmentData, ok := backend.objects["test-bucket/"+result.SegmentPath]
	if !ok {
		t.Fatalf("segment file not found at %s", result.SegmentPath)
	}

	// Parse the segment and verify it's valid JSONL
	lines := strings.Split(string(segmentData), "\n")
	if len(lines) != numEntries+1 { // +1 because of trailing newline
		t.Errorf("expected %d lines in segment, got %d", numEntries+1, len(lines))
	}

	// Verify each line can be parsed as a valid Entry
	for i, line := range lines[:numEntries] {
		var entry Entry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Errorf("failed to parse line %d: %v", i, err)
		}
		if entry.Sequence != int64(i+1) {
			t.Errorf("expected sequence %d at line %d, got %d", i+1, i, entry.Sequence)
		}
	}

	// Run audit and verify it can read from the segment
	auditor := NewAuditor(backend, "test-bucket")
	auditResult, err := auditor.Audit(ctx)
	if err != nil {
		t.Fatalf("audit failed: %v", err)
	}

	if auditResult.Status != "valid" {
		t.Errorf("expected valid audit, got %s: %+v", auditResult.Status, auditResult.Errors)
	}

	if len(auditResult.Writers) != 1 {
		t.Fatalf("expected 1 writer, got %d", len(auditResult.Writers))
	}

	writerAudit := auditResult.Writers[0]
	if writerAudit.WriterID != "test-writer" {
		t.Errorf("expected writer ID test-writer, got %s", writerAudit.WriterID)
	}
	if writerAudit.EntriesVerified != numEntries {
		t.Errorf("expected %d entries verified, got %d", numEntries, writerAudit.EntriesVerified)
	}
	if !writerAudit.Valid {
		t.Errorf("expected valid audit for writer, got error: %s", writerAudit.Error)
	}

	// Verify the audit correctly tracked all objects
	if auditResult.TotalEntries != numEntries {
		t.Errorf("expected %d total entries, got %d", numEntries, auditResult.TotalEntries)
	}

	// Verify that the original chain entries still exist (compaction should NOT delete)
	for i := int64(1); i <= numEntries; i++ {
		key := fmt.Sprintf("%s%s/%d.json", ChainPrefix, "test-writer", i)
		if _, ok := backend.objects["test-bucket/"+key]; !ok {
			t.Errorf("original chain entry %s was deleted (should be preserved)", key)
		}
	}
}

