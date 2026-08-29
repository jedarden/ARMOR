package manifest

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
)

// mockUploader tracks uploads and simulates B2 behavior
type mockUploader struct {
	mu       sync.Mutex
	uploads  map[string][]byte
	chainHeads map[string][]byte
}

func newMockUploader() *mockUploader {
	return &mockUploader{
		uploads:  make(map[string][]byte),
		chainHeads: make(map[string][]byte),
	}
}

func (m *mockUploader) Upload(ctx context.Context, key string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.uploads[key] = data
	return nil
}

func (m *mockUploader) UploadChainHead(ctx context.Context, key string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.chainHeads[key] = data
	return nil
}

func (m *mockUploader) GetUpload(key string) ([]byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.uploads[key]
	return data, ok
}

func (m *mockUploader) GetChainHead(key string) ([]byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.chainHeads[key]
	return data, ok
}

func (m *mockUploader) DeltaCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for key := range m.uploads {
		if strings.Contains(key, "delta-") {
			count++
		}
	}
	return count
}

func (m *mockUploader) ChainHeadCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.chainHeads)
}

// TestProvenanceEmbeddingInDeltas tests that chain entries are embedded in delta lines
func TestProvenanceEmbeddingInDeltas(t *testing.T) {
	idx := New()
	uploader := newMockUploader()

	writer := NewWriterWithChain(idx, ".armor/manifest", "test-writer",
		uploader.Upload, uploader.UploadChainHead, 10)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	writer.Start(ctx)
	defer writer.Stop()

	// Enqueue 3 puts with chain entries
	chainEntries := []*ChainEntry{
		{Sequence: 1, ChainHash: "hash1", PrevChainHash: "prev0"},
		{Sequence: 2, ChainHash: "hash2", PrevChainHash: "hash1"},
		{Sequence: 3, ChainHash: "hash3", PrevChainHash: "hash2"},
	}

	entry := &Entry{
		PlaintextSize:   100,
		PlaintextSHA256: "abcd1234",
		IV:              []byte{1, 2, 3, 4},
		WrappedDEK:      []byte{5, 6, 7, 8},
		BlockSize:       4096,
		ContentType:     "text/plain",
		ETag:            "etag-1",
		LastModified:    time.Now().UTC(),
	}

	for i, chain := range chainEntries {
		writer.EnqueuePut("test-bucket", "object-"+string(rune('1'+i)), entry, chain)
	}

	// Wait for batch to flush
	time.Sleep(500 * time.Millisecond)

	// Verify: exactly 1 delta file for 3 puts (batching)
	if uploader.DeltaCount() != 1 {
		t.Errorf("Expected 1 delta file, got %d", uploader.DeltaCount())
	}

	// Verify: exactly 1 chain-head file
	if uploader.ChainHeadCount() != 1 {
		t.Errorf("Expected 1 chain-head file, got %d", uploader.ChainHeadCount())
	}

	// Verify chain-head contains the latest sequence (3)
	chainHeadData, ok := uploader.GetChainHead(".armor/chain-head/test-writer")
	if !ok {
		t.Fatal("Chain-head file not found")
	}

	var chainHead struct {
		DeltaFile string `json:"delta_file"`
		Sequence  int64  `json:"sequence"`
		ChainHash string `json:"chain_hash"`
	}
	if err := json.Unmarshal(chainHeadData, &chainHead); err != nil {
		t.Fatalf("Failed to unmarshal chain-head: %v", err)
	}

	if chainHead.Sequence != 3 {
		t.Errorf("Expected chain-head sequence 3, got %d", chainHead.Sequence)
	}
	if chainHead.ChainHash != "hash3" {
		t.Errorf("Expected chain-head hash 'hash3', got '%s'", chainHead.ChainHash)
	}
	if !strings.Contains(chainHead.DeltaFile, "delta-") {
		t.Errorf("Expected delta file path in chain-head, got '%s'", chainHead.DeltaFile)
	}

	// Verify delta file contains chain entries
	var deltaData []byte
	for key, data := range uploader.uploads {
		if strings.Contains(key, "delta-") {
			deltaData = data
			break
		}
	}

	if deltaData == nil {
		t.Fatal("Delta file not found")
	}

	lines := strings.Split(string(deltaData), "\n")
	var opCount int
	var chainCount int
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var op Op
		if err := json.Unmarshal([]byte(line), &op); err != nil {
			t.Fatalf("Failed to unmarshal delta op: %v", err)
		}
		if op.Operation == "put" {
			opCount++
			if op.Chain != nil {
				chainCount++
			}
		}
	}

	if opCount != 3 {
		t.Errorf("Expected 3 put operations, got %d", opCount)
	}
	if chainCount != 3 {
		t.Errorf("Expected 3 chain entries, got %d", chainCount)
	}
}

// TestMultipleBatchesProvenance tests that multiple batches produce correct chain-head updates
func TestMultipleBatchesProvenance(t *testing.T) {
	idx := New()
	uploader := newMockUploader()

	writer := NewWriterWithChain(idx, ".armor/manifest", "test-writer",
		uploader.Upload, uploader.UploadChainHead, 2) // Small buffer to force multiple batches

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	writer.Start(ctx)
	defer writer.Stop()

	entry := &Entry{
		PlaintextSize:   100,
		PlaintextSHA256: "abcd1234",
		IV:              []byte{1, 2, 3, 4},
		WrappedDEK:      []byte{5, 6, 7, 8},
		BlockSize:       4096,
		ContentType:     "text/plain",
		ETag:            "etag-1",
		LastModified:    time.Now().UTC(),
	}

	// Enqueue 5 puts across multiple batches
	chainEntries := []*ChainEntry{
		{Sequence: 1, ChainHash: "hash1", PrevChainHash: "prev0"},
		{Sequence: 2, ChainHash: "hash2", PrevChainHash: "hash1"},
		{Sequence: 3, ChainHash: "hash3", PrevChainHash: "hash2"},
		{Sequence: 4, ChainHash: "hash4", PrevChainHash: "hash3"},
		{Sequence: 5, ChainHash: "hash5", PrevChainHash: "hash4"},
	}

	for i, chain := range chainEntries {
		writer.EnqueuePut("test-bucket", "object-"+string(rune('1'+i)), entry, chain)
		time.Sleep(100 * time.Millisecond) // Allow batch to flush
	}

	// Wait for final batch to flush
	time.Sleep(500 * time.Millisecond)

	// Verify chain-head contains the latest sequence (5)
	chainHeadData, ok := uploader.GetChainHead(".armor/chain-head/test-writer")
	if !ok {
		t.Fatal("Chain-head file not found")
	}

	var chainHead struct {
		DeltaFile string `json:"delta_file"`
		Sequence  int64  `json:"sequence"`
		ChainHash string `json:"chain_hash"`
	}
	if err := json.Unmarshal(chainHeadData, &chainHead); err != nil {
		t.Fatalf("Failed to unmarshal chain-head: %v", err)
	}

	if chainHead.Sequence != 5 {
		t.Errorf("Expected chain-head sequence 5, got %d", chainHead.Sequence)
	}
	if chainHead.ChainHash != "hash5" {
		t.Errorf("Expected chain-head hash 'hash5', got '%s'", chainHead.ChainHash)
	}
}

// TestNilChainEntry tests that nil chain entries are handled correctly
func TestNilChainEntry(t *testing.T) {
	idx := New()
	uploader := newMockUploader()

	writer := NewWriterWithChain(idx, ".armor/manifest", "test-writer",
		uploader.Upload, uploader.UploadChainHead, 10)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	writer.Start(ctx)
	defer writer.Stop()

	entry := &Entry{
		PlaintextSize:   100,
		PlaintextSHA256: "abcd1234",
		IV:              []byte{1, 2, 3, 4},
		WrappedDEK:      []byte{5, 6, 7, 8},
		BlockSize:       4096,
		ContentType:     "text/plain",
		ETag:            "etag-1",
		LastModified:    time.Now().UTC(),
	}

	// Enqueue puts with nil chain entries
	for i := 0; i < 3; i++ {
		writer.EnqueuePut("test-bucket", "object-"+string(rune('1'+i)), entry, nil)
	}

	// Wait for batch to flush
	time.Sleep(500 * time.Millisecond)

	// Verify: delta file exists but no chain-head (all nil chains)
	if uploader.DeltaCount() != 1 {
		t.Errorf("Expected 1 delta file, got %d", uploader.DeltaCount())
	}

	if uploader.ChainHeadCount() != 0 {
		t.Errorf("Expected 0 chain-head files (all nil chains), got %d", uploader.ChainHeadCount())
	}

	// Verify delta file contains put ops without chain entries
	var deltaData []byte
	for key, data := range uploader.uploads {
		if strings.Contains(key, "delta-") {
			deltaData = data
			break
		}
	}

	if deltaData == nil {
		t.Fatal("Delta file not found")
	}

	lines := strings.Split(string(deltaData), "\n")
	var opCount int
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var op Op
		if err := json.Unmarshal([]byte(line), &op); err != nil {
			t.Fatalf("Failed to unmarshal delta op: %v", err)
		}
		if op.Operation == "put" {
			opCount++
			if op.Chain != nil {
				t.Errorf("Expected nil chain entry, got sequence %d", op.Chain.Sequence)
			}
		}
	}

	if opCount != 3 {
		t.Errorf("Expected 3 put operations, got %d", opCount)
	}
}

// TestChainEntryMarshaling tests that ChainEntry marshals correctly to JSON
func TestChainEntryMarshaling(t *testing.T) {
	entry := &ChainEntry{
		Sequence:      12345,
		ChainHash:     "abcdef1234567890abcdef1234567890abcdef1234567890abcdef12",
		PrevChainHash: "0000000000000000000000000000000000000000000000000000000000000000",
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal ChainEntry: %v", err)
	}

	var unmarshaled ChainEntry
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal ChainEntry: %v", err)
	}

	if unmarshaled.Sequence != entry.Sequence {
		t.Errorf("Expected sequence %d, got %d", entry.Sequence, unmarshaled.Sequence)
	}
	if unmarshaled.ChainHash != entry.ChainHash {
		t.Errorf("Expected chain hash '%s', got '%s'", entry.ChainHash, unmarshaled.ChainHash)
	}
	if unmarshaled.PrevChainHash != entry.PrevChainHash {
		t.Errorf("Expected prev chain hash '%s', got '%s'", entry.PrevChainHash, unmarshaled.PrevChainHash)
	}
}
