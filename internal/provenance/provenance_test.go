package provenance

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"strings"
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
	return io.NopCloser(bytes.NewReader(data)), &backend.ObjectInfo{
		Key:      key,
		Size:     int64(len(data)),
		Metadata: meta,
	}, nil
}

func (m *mockBackend) GetDirect(ctx context.Context, bucket, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	return m.Get(ctx, bucket, key)
}

func (m *mockBackend) GetRange(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, error) {
	data, ok := m.objects[bucket+"/"+key]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	end := offset + length
	if end > int64(len(data)) {
		end = int64(len(data))
	}
	return io.NopCloser(bytes.NewReader(data[offset:end])), nil
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
	return &backend.ObjectInfo{
		Key:      key,
		Size:     int64(len(data)),
		Metadata: meta,
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

func (m *mockBackend) ListMultipartUploads(ctx context.Context, bucket string) (*backend.ListMultipartUploadsResult, error) {
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
