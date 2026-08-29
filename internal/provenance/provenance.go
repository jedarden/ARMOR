// Package provenance implements a cryptographic provenance chain for ARMOR.
// Each upload is linked to the previous one via a chain hash, creating a
// tamper-evident audit trail. Multiple ARMOR instances maintain independent
// per-writer chains that can be merged during audit.
package provenance

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jedarden/armor/internal/backend"
)

const (
	// ChainPrefix is the prefix for chain entry objects in B2
	ChainPrefix = ".armor/chain/"

	// ChainHeadPrefix is the prefix for chain head objects in B2
	ChainHeadPrefix = ".armor/chain-head/"

	// ChainSegmentPrefix is the prefix for chain segment files
	ChainSegmentPrefix = ".armor/chain-segments/"

	// InitialChainHash is the zero value for the first chain entry
	InitialChainHash = "0000000000000000000000000000000000000000000000000000000000000000"
)

// Entry represents a single entry in the provenance chain.
type Entry struct {
	// Sequence is the monotonically increasing sequence number for this writer
	Sequence int64 `json:"sequence"`

	// ObjectKey is the S3 object key that was uploaded
	ObjectKey string `json:"object_key"`

	// PlaintextSHA256 is the SHA-256 hash of the plaintext content
	PlaintextSHA256 string `json:"plaintext_sha256"`

	// ChainHash is the hash linking this entry to the previous one
	ChainHash string `json:"chain_hash"`

	// PrevChainHash is the chain hash of the previous entry
	PrevChainHash string `json:"prev_chain_hash"`

	// Timestamp is when this entry was created
	Timestamp time.Time `json:"timestamp"`

	// WriterID identifies which ARMOR instance created this entry
	WriterID string `json:"writer_id"`

	// Operation is the type of operation (put, multipart, copy)
	Operation string `json:"operation"`
}

// KeyEvent represents a key management event in the provenance chain.
// These events track sensitive key operations that are part of the audit trail.
type KeyEvent struct {
	// Sequence is the monotonically increasing sequence number for this writer
	Sequence int64 `json:"sequence"`

	// EventType is the type of key event (key-rotate-start, key-rotate-complete, key-export)
	EventType string `json:"event_type"`

	// ChainHash is the hash linking this entry to the previous one
	ChainHash string `json:"chain_hash"`

	// PrevChainHash is the chain hash of the previous entry
	PrevChainHash string `json:"prev_chain_hash"`

	// Timestamp is when this event was created
	Timestamp time.Time `json:"timestamp"`

	// WriterID identifies which ARMOR instance created this entry
	WriterID string `json:"writer_id"`

	// OldMEKHash is the SHA-256 hash of the old master encryption key (first 16 hex chars)
	// For key-rotate-start and key-rotate-complete events
	OldMEKHash string `json:"old_mek_hash,omitempty"`

	// NewMEKHash is the SHA-256 hash of the new master encryption key (first 16 hex chars)
	// For key-rotate-start and key-rotate-complete events
	NewMEKHash string `json:"new_mek_hash,omitempty"`

	// RotationID uniquely identifies this rotation operation
	// For key-rotate-start and key-rotate-complete events
	RotationID string `json:"rotation_id,omitempty"`

	// RotationResult contains summary of rotation results
	// For key-rotate-complete events
	RotationResult *KeyRotationResult `json:"rotation_result,omitempty"`

	// ExportedMEKHash is the SHA-256 hash of the exported master encryption key (first 16 hex chars)
	// For key-export events
	ExportedMEKHash string `json:"exported_mek_hash,omitempty"`
}

// KeyRotationResult summarizes the outcome of a key rotation operation.
type KeyRotationResult struct {
	TotalObjects     int     `json:"total_objects"`
	ProcessedObjects int     `json:"processed_objects"`
	SkippedObjects   int     `json:"skipped_objects"`
	Exceptions       int     `json:"exceptions"`
	DurationSec      float64 `json:"duration_sec"`
	Status           string  `json:"status"`
}

// ChainHead represents the current head of a writer's chain.
// The format varies based on whether manifest is enabled:
// - Legacy mode (manifest disabled): {writer_id, sequence, chain_hash, updated}
// - Manifest mode (manifest enabled): {delta_file, sequence, chain_hash}
type ChainHead struct {
	// WriterID identifies the ARMOR instance (legacy format only)
	WriterID string `json:"writer_id,omitempty"`

	// DeltaFile is the manifest delta file containing the latest entry (manifest format only)
	DeltaFile string `json:"delta_file,omitempty"`

	// Sequence is the current sequence number
	Sequence int64 `json:"sequence"`

	// ChainHash is the chain hash of the most recent entry
	ChainHash string `json:"chain_hash"`

	// Updated is when the head was last updated (legacy format only)
	Updated time.Time `json:"updated,omitempty"`
}

// CompactResult represents the result of a chain compaction operation.
type CompactResult struct {
	// SegmentPath is the path to the created segment file
	SegmentPath string `json:"segment_path"`

	// FromSequence is the first sequence in the segment
	FromSequence int64 `json:"from_sequence"`

	// ToSequence is the last sequence in the segment
	ToSequence int64 `json:"to_sequence"`

	// EntryCount is the number of entries in the segment
	EntryCount int `json:"entry_count"`

	// KeyEventCount is the number of key events in the segment
	KeyEventCount int `json:"key_event_count"`
}

// Manager handles provenance chain operations.
type Manager struct {
	backend  backend.Backend
	bucket   string
	writerID string

	// appendMu serializes the read-head/write-entry/write-head transaction.
	// A writer serves concurrent HTTP requests, so protecting only the cached
	// head would allow two uploads to claim the same sequence number.
	appendMu sync.Mutex

	// In-memory cache of the current chain head
	mu   sync.RWMutex
	head *ChainHead

	// Skip provenance for internal operations
	skipPrefixes []string
}

// NewManager creates a new provenance manager.
func NewManager(be backend.Backend, bucket, writerID string) *Manager {
	return &Manager{
		backend:  be,
		bucket:   bucket,
		writerID: writerID,
		skipPrefixes: []string{
			".armor/", // Internal ARMOR objects
		},
	}
}

// ShouldRecord returns true if an object with this key should have
// its provenance recorded. Internal objects (starting with .armor/) are skipped.
func (m *Manager) ShouldRecord(key string) bool {
	for _, prefix := range m.skipPrefixes {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			return false
		}
	}
	return true
}

// ChainEntryData represents the minimal chain information for embedding
// in a manifest delta line.
type ChainEntryData struct {
	Sequence       int64  `json:"sequence"`
	ChainHash      string `json:"chain_hash"`
	PrevChainHash  string `json:"prev_chain_hash"`
}

// CreateChainEntry creates a chain entry for embedding in a manifest delta.
// It atomically increments the sequence number and computes the chain hash,
// but does not write the entry to B2. This is used when the manifest is enabled.
// The returned ChainEntryData contains {sequence, chain_hash, prev_chain_hash}.
func (m *Manager) CreateChainEntry(ctx context.Context, objectKey, plaintextSHA256, operation string) (*ChainEntryData, error) {
	// Skip internal objects
	if !m.ShouldRecord(objectKey) {
		return nil, nil
	}

	m.appendMu.Lock()
	defer m.appendMu.Unlock()

	// Get or load the current chain head
	head, err := m.getOrCreateHead(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain head: %w", err)
	}

	// Create the entry (same as RecordUpload but we don't write it)
	now := time.Now().UTC()
	entry := &Entry{
		Sequence:        head.Sequence + 1,
		ObjectKey:       objectKey,
		PlaintextSHA256: plaintextSHA256,
		PrevChainHash:   head.ChainHash,
		Timestamp:       now,
		WriterID:        m.writerID,
		Operation:       operation,
	}

	// Compute the chain hash
	entry.ChainHash = computeChainHash(entry, head.ChainHash)

	// Update in-memory cache immediately so next call sees new sequence
	newHead := &ChainHead{
		WriterID:  m.writerID,
		Sequence:  entry.Sequence,
		ChainHash: entry.ChainHash,
		Updated:   now,
	}
	m.mu.Lock()
	m.head = newHead
	m.mu.Unlock()

	// Return minimal data for embedding in delta line
	return &ChainEntryData{
		Sequence:      entry.Sequence,
		ChainHash:     entry.ChainHash,
		PrevChainHash: entry.PrevChainHash,
	}, nil
}

// RecordUpload records an upload in the provenance chain.
// This should be called after a successful upload.
// This method is used when the manifest is disabled; when manifest is enabled,
// use CreateChainEntry instead and embed the result in the delta line.
func (m *Manager) RecordUpload(ctx context.Context, objectKey, plaintextSHA256, operation string) error {
	// Skip internal objects
	if !m.ShouldRecord(objectKey) {
		return nil
	}

	m.appendMu.Lock()
	defer m.appendMu.Unlock()

	// Get or load the current chain head
	head, err := m.getOrCreateHead(ctx)
	if err != nil {
		return fmt.Errorf("failed to get chain head: %w", err)
	}

	// Create the new entry
	now := time.Now().UTC()
	entry := &Entry{
		Sequence:        head.Sequence + 1,
		ObjectKey:       objectKey,
		PlaintextSHA256: plaintextSHA256,
		PrevChainHash:   head.ChainHash,
		Timestamp:       now,
		WriterID:        m.writerID,
		Operation:       operation,
	}

	// Compute the chain hash
	entry.ChainHash = computeChainHash(entry, head.ChainHash)

	// Save the entry
	if err := m.saveEntry(ctx, entry); err != nil {
		return fmt.Errorf("failed to save chain entry: %w", err)
	}

	// Update and save the chain head
	newHead := &ChainHead{
		WriterID:  m.writerID,
		Sequence:  entry.Sequence,
		ChainHash: entry.ChainHash,
		Updated:   now,
	}

	if err := m.saveHead(ctx, newHead); err != nil {
		return fmt.Errorf("failed to save chain head: %w", err)
	}

	// Update in-memory cache
	m.mu.Lock()
	m.head = newHead
	m.mu.Unlock()

	return nil
}

// RecordKeyEvent records a key management event in the provenance chain.
// Supported event types: "key-rotate-start", "key-rotate-complete", "key-export".
// This should be called after a successful key operation.
func (m *Manager) RecordKeyEvent(ctx context.Context, eventType string, opts KeyEventOpts) error {
	// Validate event type
	switch eventType {
	case "key-rotate-start", "key-rotate-complete", "key-export":
		// Valid event types
	default:
		return fmt.Errorf("invalid event type: %s", eventType)
	}

	m.appendMu.Lock()
	defer m.appendMu.Unlock()

	// Get or load the current chain head
	head, err := m.getOrCreateHead(ctx)
	if err != nil {
		return fmt.Errorf("failed to get chain head: %w", err)
	}

	// Create the new key event
	now := time.Now().UTC()
	event := &KeyEvent{
		Sequence:      head.Sequence + 1,
		EventType:     eventType,
		PrevChainHash: head.ChainHash,
		Timestamp:     now,
		WriterID:      m.writerID,
	}

	// Set optional fields based on event type
	switch eventType {
	case "key-rotate-start", "key-rotate-complete":
		event.OldMEKHash = opts.OldMEKHash
		event.NewMEKHash = opts.NewMEKHash
		event.RotationID = opts.RotationID
		if eventType == "key-rotate-complete" && opts.RotationResult != nil {
			event.RotationResult = opts.RotationResult
		}
	case "key-export":
		event.ExportedMEKHash = opts.ExportedMEKHash
	}

	// Compute the chain hash
	event.ChainHash = computeKeyEventHash(event, head.ChainHash)

	// Save the event
	if err := m.saveKeyEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to save key event: %w", err)
	}

	// Update and save the chain head
	newHead := &ChainHead{
		WriterID:  m.writerID,
		Sequence:  event.Sequence,
		ChainHash: event.ChainHash,
		Updated:   now,
	}

	if err := m.saveHead(ctx, newHead); err != nil {
		return fmt.Errorf("failed to save chain head: %w", err)
	}

	// Update in-memory cache
	m.mu.Lock()
	m.head = newHead
	m.mu.Unlock()

	return nil
}

// KeyEventOpts holds optional parameters for key events.
type KeyEventOpts struct {
	// OldMEKHash is the first 16 hex characters of the old MEK's SHA-256
	OldMEKHash string
	// NewMEKHash is the first 16 hex characters of the new MEK's SHA-256
	NewMEKHash string
	// RotationID uniquely identifies this rotation operation
	RotationID string
	// RotationResult summarizes the rotation outcome
	RotationResult *KeyRotationResult
	// ExportedMEKHash is the first 16 hex characters of the exported MEK's SHA-256
	ExportedMEKHash string
}

// getOrCreateHead returns the current chain head, creating an initial one if needed.
func (m *Manager) getOrCreateHead(ctx context.Context) (*ChainHead, error) {
	// Check in-memory cache first
	m.mu.RLock()
	if m.head != nil {
		m.mu.RUnlock()
		return m.head, nil
	}
	m.mu.RUnlock()

	// Try to load from B2
	head, err := m.loadHead(ctx)
	if err == nil {
		m.mu.Lock()
		m.head = head
		m.mu.Unlock()
		return head, nil
	}

	// Create initial head
	initialHead := &ChainHead{
		WriterID:  m.writerID,
		Sequence:  0,
		ChainHash: InitialChainHash,
		Updated:   time.Now().UTC(),
	}

	m.mu.Lock()
	m.head = initialHead
	m.mu.Unlock()

	return initialHead, nil
}

// computeChainHash computes the chain hash for an entry.
// chain_hash = SHA-256(prev_chain_hash || object_key || plaintext_sha256 || timestamp || writer_id)
func computeChainHash(entry *Entry, prevChainHash string) string {
	h := sha256.New()

	// Write in deterministic order
	h.Write([]byte(prevChainHash))
	h.Write([]byte(entry.ObjectKey))
	h.Write([]byte(entry.PlaintextSHA256))
	h.Write([]byte(entry.Timestamp.Format(time.RFC3339Nano)))
	h.Write([]byte(entry.WriterID))

	return fmt.Sprintf("%064x", h.Sum(nil))
}

// computeKeyEventHash computes the chain hash for a key event.
// chain_hash = SHA-256(prev_chain_hash || event_type || timestamp || writer_id || mek_hash_or_rotation_id)
func computeKeyEventHash(event *KeyEvent, prevChainHash string) string {
	h := sha256.New()

	// Write in deterministic order
	h.Write([]byte(prevChainHash))
	h.Write([]byte(event.EventType))
	h.Write([]byte(event.Timestamp.Format(time.RFC3339Nano)))
	h.Write([]byte(event.WriterID))

	// Add event-specific fields for cryptographic binding.
	// NOTE: this is a chain-hash input. The case set and the order of the
	// h.Write calls must stay byte-for-byte identical to the previous
	// if/else-if form, or every existing provenance chain fails verification.
	// A tagged switch over the same values in the same order is exactly
	// equivalent; do not reorder or merge these writes.
	switch event.EventType {
	case "key-rotate-start", "key-rotate-complete":
		h.Write([]byte(event.OldMEKHash))
		h.Write([]byte(event.NewMEKHash))
		h.Write([]byte(event.RotationID))
	case "key-export":
		h.Write([]byte(event.ExportedMEKHash))
	}

	return fmt.Sprintf("%064x", h.Sum(nil))
}

// saveEntry saves a chain entry to B2.
func (m *Manager) saveEntry(ctx context.Context, entry *Entry) error {
	key := fmt.Sprintf("%s%s/%d.json", ChainPrefix, m.writerID, entry.Sequence)

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal entry: %w", err)
	}

	if err := m.backend.Put(ctx, m.bucket, key, bytes.NewReader(data), int64(len(data)), map[string]string{
		"Content-Type": "application/json",
	}); err != nil {
		return fmt.Errorf("failed to put entry: %w", err)
	}

	return nil
}

// saveKeyEvent saves a key event to B2.
// Key events are stored in the same chain namespace as upload events,
// ensuring they're part of the same tamper-evident audit trail.
func (m *Manager) saveKeyEvent(ctx context.Context, event *KeyEvent) error {
	key := fmt.Sprintf("%s%s/%d.json", ChainPrefix, m.writerID, event.Sequence)

	data, err := json.MarshalIndent(event, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal key event: %w", err)
	}

	if err := m.backend.Put(ctx, m.bucket, key, bytes.NewReader(data), int64(len(data)), map[string]string{
		"Content-Type": "application/json",
	}); err != nil {
		return fmt.Errorf("failed to put key event: %w", err)
	}

	return nil
}

// saveHead saves the chain head to B2.
func (m *Manager) saveHead(ctx context.Context, head *ChainHead) error {
	key := fmt.Sprintf("%s%s", ChainHeadPrefix, m.writerID)

	data, err := json.MarshalIndent(head, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal head: %w", err)
	}

	if err := m.backend.Put(ctx, m.bucket, key, bytes.NewReader(data), int64(len(data)), map[string]string{
		"Content-Type": "application/json",
	}); err != nil {
		return fmt.Errorf("failed to put head: %w", err)
	}

	return nil
}

// loadHead loads the chain head from B2.
func (m *Manager) loadHead(ctx context.Context) (*ChainHead, error) {
	key := fmt.Sprintf("%s%s", ChainHeadPrefix, m.writerID)

	body, _, err := m.backend.GetDirect(ctx, m.bucket, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get head: %w", err)
	}
	defer body.Close()

	data, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("failed to read head: %w", err)
	}

	var head ChainHead
	if err := json.Unmarshal(data, &head); err != nil {
		return nil, fmt.Errorf("failed to unmarshal head: %w", err)
	}

	return &head, nil
}

// CompactLegacyChain compacts legacy chain entries into a segment file.
// It reads all .armor/chain/<writer>/*.json entries in sequence order,
// writes them to .armor/chain-segments/<writer>/<from>-<to>.jsonl,
// verifies the segment, and returns the result. Original entries are NOT deleted.
func (m *Manager) CompactLegacyChain(ctx context.Context, writerID string) (*CompactResult, error) {
	// List all legacy chain entries for this writer
	prefix := ChainPrefix + writerID + "/"
	keys, err := m.listChainEntries(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list chain entries: %w", err)
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("no legacy chain entries found for writer %s", writerID)
	}

	// Parse sequence numbers and sort
	sequences := make([]int64, 0, len(keys))
	keyBySeq := make(map[int64]string)
	for _, key := range keys {
		_, seq, err := parseChainEntryKey(key)
		if err != nil {
			return nil, fmt.Errorf("failed to parse entry key %s: %w", key, err)
		}
		sequences = append(sequences, seq)
		keyBySeq[seq] = key
	}
	sort.Slice(sequences, func(i, j int) bool {
		return sequences[i] < sequences[j]
	})

	fromSeq := sequences[0]
	toSeq := sequences[len(sequences)-1]

	// Read all entries in sequence order
	entries := make([]interface{}, len(sequences))
	keyEventCount := 0
	for i, seq := range sequences {
		key := keyBySeq[seq]
		entry, isKeyEvent, err := m.readChainEntry(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("failed to read entry %s: %w", key, err)
		}
		entries[i] = entry
		if isKeyEvent {
			keyEventCount++
		}
	}

	// Write segment file
	segmentPath := fmt.Sprintf("%s%s/%010d-%010d.jsonl", ChainSegmentPrefix, writerID, fromSeq, toSeq)
	if err := m.writeSegmentFile(ctx, segmentPath, entries); err != nil {
		return nil, fmt.Errorf("failed to write segment file: %w", err)
	}

	// Verify the segment by re-reading it
	if err := m.verifySegmentFile(ctx, segmentPath, entries); err != nil {
		return nil, fmt.Errorf("segment verification failed: %w", err)
	}

	return &CompactResult{
		SegmentPath:    segmentPath,
		FromSequence:   fromSeq,
		ToSequence:     toSeq,
		EntryCount:     len(entries),
		KeyEventCount:  keyEventCount,
	}, nil
}

// listChainEntries lists all legacy chain entry keys for a writer.
func (m *Manager) listChainEntries(ctx context.Context, prefix string) ([]string, error) {
	var keys []string
	continuationToken := ""

	for {
		result, err := m.backend.List(ctx, m.bucket, prefix, "", continuationToken, 1000)
		if err != nil {
			return nil, err
		}
		if result == nil {
			return nil, fmt.Errorf("backend returned a nil listing for prefix %q", prefix)
		}

		for _, obj := range result.Objects {
			if strings.HasSuffix(obj.Key, ".json") {
				keys = append(keys, obj.Key)
			}
		}

		if !result.IsTruncated {
			break
		}
		if result.NextToken == "" || result.NextToken == continuationToken {
			return nil, fmt.Errorf("backend returned a truncated listing without a usable continuation token")
		}
		continuationToken = result.NextToken
	}

	sort.Strings(keys)
	return keys, nil
}

// readChainEntry reads a single chain entry and returns it as an interface{}.
// Returns (entry, isKeyEvent, error).
func (m *Manager) readChainEntry(ctx context.Context, key string) (interface{}, bool, error) {
	body, _, err := m.backend.GetDirect(ctx, m.bucket, key)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get entry: %w", err)
	}
	defer body.Close()

	data, err := io.ReadAll(body)
	if err != nil {
		return nil, false, fmt.Errorf("failed to read entry: %w", err)
	}

	// Try to determine the type by checking for specific fields
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return nil, false, fmt.Errorf("failed to parse JSON: %w", err)
	}

	_, hasObjectKey := rawMap["object_key"]
	_, hasEventType := rawMap["event_type"]

	if hasObjectKey && !hasEventType {
		// Regular Entry
		var entry Entry
		if err := json.Unmarshal(data, &entry); err != nil {
			return nil, false, fmt.Errorf("failed to parse entry: %w", err)
		}
		return &entry, false, nil
	} else if hasEventType && !hasObjectKey {
		// KeyEvent
		var event KeyEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return nil, false, fmt.Errorf("failed to parse key event: %w", err)
		}
		return &event, true, nil
	}

	return nil, false, fmt.Errorf("ambiguous or unknown entry type")
}

// writeSegmentFile writes entries to a segment file as JSONL.
func (m *Manager) writeSegmentFile(ctx context.Context, path string, entries []interface{}) error {
	var buf bytes.Buffer

	for _, entry := range entries {
		data, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("failed to marshal entry: %w", err)
		}
		buf.Write(data)
		buf.WriteByte('\n')
	}

	data := buf.Bytes()
	if err := m.backend.Put(ctx, m.bucket, path, bytes.NewReader(data), int64(len(data)), map[string]string{
		"Content-Type": "application/jsonl",
	}); err != nil {
		return fmt.Errorf("failed to put segment file: %w", err)
	}

	return nil
}

// verifySegmentFile re-reads a segment file and verifies all hash links.
func (m *Manager) verifySegmentFile(ctx context.Context, path string, expectedEntries []interface{}) error {
	body, _, err := m.backend.GetDirect(ctx, m.bucket, path)
	if err != nil {
		return fmt.Errorf("failed to get segment file: %w", err)
	}
	defer body.Close()

	// Read and parse all entries from the segment
	var entries []interface{}
	scanner := newJSONLScanner(body)
	for scanner.Scan() {
		var rawMap map[string]json.RawMessage
		if err := json.Unmarshal(scanner.Bytes(), &rawMap); err != nil {
			return fmt.Errorf("failed to parse JSON: %w", err)
		}

		_, hasObjectKey := rawMap["object_key"]
		_, hasEventType := rawMap["event_type"]

		if hasObjectKey && !hasEventType {
			var entry Entry
			if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
				return fmt.Errorf("failed to parse entry: %w", err)
			}
			entries = append(entries, &entry)
		} else if hasEventType && !hasObjectKey {
			var event KeyEvent
			if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
				return fmt.Errorf("failed to parse key event: %w", err)
			}
			entries = append(entries, &event)
		} else {
			return fmt.Errorf("ambiguous or unknown entry type")
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading segment: %w", err)
	}

	// Verify we have the same number of entries
	if len(entries) != len(expectedEntries) {
		return fmt.Errorf("entry count mismatch: got %d, expected %d", len(entries), len(expectedEntries))
	}

	// Verify each entry matches
	for i, entry := range entries {
		expectedEntry := expectedEntries[i]

		switch e := entry.(type) {
		case *Entry:
			expected, ok := expectedEntry.(*Entry)
			if !ok {
				return fmt.Errorf("type mismatch at entry %d", i)
			}
			if e.Sequence != expected.Sequence ||
				e.ObjectKey != expected.ObjectKey ||
				e.PlaintextSHA256 != expected.PlaintextSHA256 ||
				e.ChainHash != expected.ChainHash ||
				e.PrevChainHash != expected.PrevChainHash ||
				e.WriterID != expected.WriterID ||
				e.Operation != expected.Operation {
				return fmt.Errorf("entry mismatch at sequence %d", e.Sequence)
			}

		case *KeyEvent:
			expected, ok := expectedEntry.(*KeyEvent)
			if !ok {
				return fmt.Errorf("type mismatch at entry %d", i)
			}
			if e.Sequence != expected.Sequence ||
				e.EventType != expected.EventType ||
				e.ChainHash != expected.ChainHash ||
				e.PrevChainHash != expected.PrevChainHash ||
				e.WriterID != expected.WriterID {
				return fmt.Errorf("key event mismatch at sequence %d", e.Sequence)
			}
		}
	}

	return nil
}

// AuditResult contains the result of a provenance chain audit.
type AuditResult struct {
	// Status is overall audit status: "valid", "invalid", or "incomplete".
	// Incomplete takes precedence when a backend failure prevents a full audit.
	Status string `json:"status"`

	// Writers audited
	Writers []WriterAudit `json:"writers"`

	// Total entries verified
	TotalEntries int64 `json:"total_entries"`

	// Total objects in bucket
	TotalObjects int64 `json:"total_objects"`

	// Objects not in any chain (potential bypass)
	UntrackedObjects []string `json:"untracked_objects,omitempty"`

	// Chain gaps detected
	Gaps []GapInfo `json:"gaps,omitempty"`

	// Errors encountered
	Errors []string `json:"errors,omitempty"`
}

// WriterAudit contains audit results for a single writer's chain.
type WriterAudit struct {
	WriterID        string `json:"writer_id"`
	HeadSequence    int64  `json:"head_sequence"`
	EntriesVerified int    `json:"entries_verified"`
	KeyEvents       int    `json:"key_events"` // Number of key events in chain
	Valid           bool   `json:"valid"`
	Error           string `json:"error,omitempty"`
}

// GapInfo describes a gap in a chain.
type GapInfo struct {
	WriterID   string `json:"writer_id"`
	AfterSeq   int64  `json:"after_seq"`
	MissingSeq int64  `json:"missing_seq"`
}

// Auditor performs provenance chain audits.
type Auditor struct {
	backend backend.Backend
	bucket  string
	prefix  string
}

// NewAuditor creates a new provenance auditor.
func NewAuditor(be backend.Backend, bucket string) *Auditor {
	return NewAuditorWithPrefix(be, bucket, "")
}

// NewAuditorWithPrefix creates a provenance auditor for one logical ARMOR
// namespace. Provenance records live at the bucket-level .armor/ prefix, while
// data objects may live below ARMOR_PREFIX and are reported with that prefix
// removed, matching the keys stored in provenance entries.
func NewAuditorWithPrefix(be backend.Backend, bucket, prefix string) *Auditor {
	return &Auditor{
		backend: be,
		bucket:  bucket,
		prefix:  prefix,
	}
}

// Audit performs a full provenance chain audit.
// It walks all writer chains, verifies integrity, and checks for untracked objects.
func (a *Auditor) Audit(ctx context.Context) (*AuditResult, error) {
	result := &AuditResult{
		Status:  "valid",
		Writers: make([]WriterAudit, 0),
	}

	// Discover both sides of the chain namespace. Listing entries as well as
	// heads detects a deleted head, an entry beyond the advertised head, and
	// malformed objects in the reserved namespace.
	headKeys, err := a.listInternalKeys(ctx, ChainHeadPrefix)
	if err != nil {
		markIncomplete(result)
		result.Errors = append(result.Errors, fmt.Sprintf("failed to list chain heads: %v", err))
		return result, nil
	}
	entryKeys, err := a.listInternalKeys(ctx, ChainPrefix)
	if err != nil {
		markIncomplete(result)
		result.Errors = append(result.Errors, fmt.Sprintf("failed to list chain entries: %v", err))
		return result, nil
	}

	entriesByWriter := make(map[string]map[int64]string)
	for _, key := range entryKeys {
		writerID, sequence, parseErr := parseChainEntryKey(key)
		if parseErr != nil {
			markInvalid(result)
			result.Errors = append(result.Errors, parseErr.Error())
			continue
		}
		if entriesByWriter[writerID] == nil {
			entriesByWriter[writerID] = make(map[int64]string)
		}
		entriesByWriter[writerID][sequence] = key
	}

	trackedObjects := make(map[string]bool)
	headWriters := make(map[string]bool)

	for _, headKey := range headKeys {
		writerID, parseErr := parseChainHeadKey(headKey)
		if parseErr != nil {
			markInvalid(result)
			result.Errors = append(result.Errors, parseErr.Error())
			continue
		}
		headWriters[writerID] = true

		body, _, getErr := a.backend.GetDirect(ctx, a.bucket, headKey)
		if getErr != nil {
			errText := fmt.Sprintf("failed to load chain head for writer %q: %v", writerID, getErr)
			result.Writers = append(result.Writers, WriterAudit{WriterID: writerID, Valid: false, Error: errText})
			result.Errors = append(result.Errors, errText)
			markIncomplete(result)
			continue
		}
		if body == nil {
			errText := fmt.Sprintf("failed to load chain head for writer %q: backend returned a nil body", writerID)
			result.Writers = append(result.Writers, WriterAudit{WriterID: writerID, Valid: false, Error: errText})
			result.Errors = append(result.Errors, errText)
			markIncomplete(result)
			continue
		}

		data, readErr := io.ReadAll(body)
		closeErr := body.Close()
		if readErr != nil || closeErr != nil {
			if readErr == nil {
				readErr = closeErr
			}
			errText := fmt.Sprintf("failed to read chain head for writer %q: %v", writerID, readErr)
			result.Writers = append(result.Writers, WriterAudit{WriterID: writerID, Valid: false, Error: errText})
			result.Errors = append(result.Errors, errText)
			markIncomplete(result)
			continue
		}

		var head ChainHead
		if unmarshalErr := json.Unmarshal(data, &head); unmarshalErr != nil {
			errText := fmt.Sprintf("invalid chain head for writer %q: %v", writerID, unmarshalErr)
			result.Writers = append(result.Writers, WriterAudit{WriterID: writerID, Valid: false, Error: errText})
			result.Errors = append(result.Errors, errText)
			markInvalid(result)
			continue
		}

		writerTracked := make(map[string]bool)
		writerAudit, gap, incomplete := a.auditWriterChain(ctx, &head, writerID, entriesByWriter[writerID], writerTracked)
		result.Writers = append(result.Writers, writerAudit)
		result.TotalEntries += int64(writerAudit.EntriesVerified)
		if gap != nil {
			result.Gaps = append(result.Gaps, *gap)
		}

		if writerAudit.Valid {
			for objectKey := range writerTracked {
				trackedObjects[objectKey] = true
			}
		} else if incomplete {
			markIncomplete(result)
		} else {
			markInvalid(result)
		}
	}

	// An entry namespace with no corresponding head cannot be walked from a
	// trusted tip. This can be a deleted head or an append observed between its
	// entry and head writes, so report an incomplete audit rather than claiming
	// that the cryptographic contents themselves are invalid.
	orphanWriters := make([]string, 0)
	for writerID := range entriesByWriter {
		if !headWriters[writerID] {
			orphanWriters = append(orphanWriters, writerID)
		}
	}
	sort.Strings(orphanWriters)
	for _, writerID := range orphanWriters {
		errText := "chain entries exist without a chain head"
		result.Writers = append(result.Writers, WriterAudit{
			WriterID:        writerID,
			EntriesVerified: 0,
			Valid:           false,
			Error:           errText,
		})
		markIncomplete(result)
	}

	// Cross-reference the current object namespace. Backend listings do not
	// necessarily include metadata (B2's intentionally does not), so the helper
	// performs authoritative HEAD requests before deciding an object is ARMOR-
	// encrypted.
	if err := a.findUntrackedObjects(ctx, trackedObjects, result); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to find untracked objects: %v", err))
		markIncomplete(result)
	}
	if len(result.UntrackedObjects) > 0 {
		markInvalid(result)
	}

	sort.Slice(result.Writers, func(i, j int) bool {
		return result.Writers[i].WriterID < result.Writers[j].WriterID
	})
	sort.Strings(result.UntrackedObjects)

	return result, nil
}

// rawObjectLister is implemented by backends that intentionally hide .armor/
// objects from their public List method. Keeping it as a narrow optional
// interface avoids changing every Backend implementation and test double.
type rawObjectLister interface {
	ListRaw(ctx context.Context, bucket, prefix, delimiter, continuationToken string, maxKeys int) (*backend.ListResult, error)
}

// listInternalKeys lists a reserved .armor/ namespace without the public S3
// filtering applied by production backends.
func (a *Auditor) listInternalKeys(ctx context.Context, prefix string) ([]string, error) {
	keys := make([]string, 0)
	continuationToken := ""
	for {
		var (
			result *backend.ListResult
			err    error
		)
		if raw, ok := a.backend.(rawObjectLister); ok {
			result, err = raw.ListRaw(ctx, a.bucket, prefix, "", continuationToken, 1000)
		} else {
			result, err = a.backend.List(ctx, a.bucket, prefix, "", continuationToken, 1000)
		}
		if err != nil {
			return nil, err
		}
		if result == nil {
			return nil, fmt.Errorf("backend returned a nil listing for prefix %q", prefix)
		}
		for _, obj := range result.Objects {
			keys = append(keys, obj.Key)
		}

		if !result.IsTruncated {
			break
		}
		if result.NextToken == "" || result.NextToken == continuationToken {
			return nil, fmt.Errorf("backend returned a truncated listing without a usable continuation token")
		}
		continuationToken = result.NextToken
	}

	sort.Strings(keys)
	return keys, nil
}

// auditWriterChain verifies a single writer's chain integrity.
// It walks three sources in order:
// 1. Legacy chain objects (`.armor/chain/<writer>/*.json`)
// 2. Chain segments (`.armor/chain-segments/<writer>/<from>-<to>.jsonl`)
// 3. Delta-embedded entries (`.armor/manifest/<writer>/delta-*.jsonl`)
func (a *Auditor) auditWriterChain(
	ctx context.Context,
	head *ChainHead,
	expectedWriterID string,
	entryKeys map[int64]string,
	trackedObjects map[string]bool,
) (WriterAudit, *GapInfo, bool) {
	audit := WriterAudit{
		WriterID:     expectedWriterID,
		HeadSequence: head.Sequence,
		Valid:        true,
	}

	// Determine chain head format: legacy (has WriterID and Updated) vs manifest (has DeltaFile)
	isLegacyFormat := head.WriterID != "" && !head.Updated.IsZero()
	isManifestFormat := head.DeltaFile != ""

	if !isLegacyFormat && !isManifestFormat {
		audit.Valid = false
		audit.Error = "chain head has neither legacy nor manifest format fields"
		return audit, nil, false
	}

	if head.Sequence < 0 {
		audit.Valid = false
		audit.Error = fmt.Sprintf("invalid negative head sequence %d", head.Sequence)
		return audit, nil, false
	}
	if !isSHA256(head.ChainHash) {
		audit.Valid = false
		audit.Error = "chain head hash is not a canonical SHA-256 value"
		return audit, nil, false
	}

	// Walk the chain from head to genesis
	expectedSeq := head.Sequence
	expectedChainHash := head.ChainHash

	// For manifest format, we need to walk three sources:
	// 1. Legacy chain (up to compaction point)
	// 2. Chain segments (if any)
	// 3. Delta-embedded entries
	if isManifestFormat {
		// Extract the starting delta sequence from the delta file path
		// Format: .armor/manifest/<writer>/delta-0000000001.jsonl
		deltaSeq := uint64(0)
		if idx := strings.LastIndex(head.DeltaFile, "delta-"); idx >= 0 {
			seqStr := head.DeltaFile[idx+6:]
			if idx = strings.Index(seqStr, ".jsonl"); idx >= 0 {
				seqStr = seqStr[:idx]
				if parsed, err := strconv.ParseUint(seqStr, 10, 64); err == nil {
					deltaSeq = parsed
				}
			}
		}

		// Walk delta-embedded entries
		deltaEntries, deltaTracked, err := a.walkDeltaEntries(ctx, expectedWriterID, 1) // Start from delta 1
		if err != nil {
			audit.Valid = false
			audit.Error = fmt.Sprintf("failed to walk delta entries: %v", err)
			return audit, nil, true
		}

		// Verify delta entries
		for seq := expectedSeq; seq > 0; seq-- {
			chainData, ok := deltaEntries[seq]
			if !ok {
				// This entry might be in legacy chain or segments
				break
			}

			// Verify the chain link
			if chainData.ChainHash != expectedChainHash {
				audit.Valid = false
				audit.Error = fmt.Sprintf("delta chain link mismatch at sequence %d", seq)
				return audit, nil, false
			}

			expectedChainHash = chainData.PrevChainHash
			audit.EntriesVerified++
		}

		// Add delta-tracked objects to the tracked set
		for key := range deltaTracked {
			trackedObjects[key] = true
		}

		// If we verified all entries from deltas, we're done
		if audit.EntriesVerified == int(expectedSeq) {
			if expectedChainHash != InitialChainHash {
				audit.Valid = false
				audit.Error = "chain does not link back to genesis"
				return audit, nil, false
			}
			return audit, nil, false
		}

		// Fall through to legacy chain verification for remaining entries
		expectedSeq = head.Sequence - int64(audit.EntriesVerified)
	}

	// Walk legacy chain entries (for both legacy and manifest format)
	for seq := expectedSeq; seq > 0; seq-- {
		key, wasListed := entryKeys[seq]
		if !wasListed {
			// The head may have advanced after the entry listing completed. Fetch
			// the canonical key directly before deciding that a gap exists.
			key = fmt.Sprintf("%s%s/%d.json", ChainPrefix, expectedWriterID, seq)
		}

		body, _, err := a.backend.GetDirect(ctx, a.bucket, key)
		if err != nil {
			audit.Valid = false
			if wasListed {
				audit.Error = fmt.Sprintf("failed to load listed entry at sequence %d: %v", seq, err)
				return audit, nil, true
			}
			audit.Error = fmt.Sprintf("missing entry at sequence %d: %v", seq, err)
			return audit, &GapInfo{WriterID: expectedWriterID, AfterSeq: seq - 1, MissingSeq: seq}, false
		}
		if body == nil {
			audit.Valid = false
			audit.Error = fmt.Sprintf("failed to load entry at sequence %d: backend returned a nil body", seq)
			return audit, nil, true
		}

		data, err := io.ReadAll(body)
		closeErr := body.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			audit.Valid = false
			audit.Error = fmt.Sprintf("failed to read entry at sequence %d: %v", seq, err)
			return audit, nil, true
		}

		var rawMap map[string]json.RawMessage
		if err := json.Unmarshal(data, &rawMap); err != nil {
			audit.Valid = false
			audit.Error = fmt.Sprintf("failed to parse JSON at sequence %d: %v", seq, err)
			return audit, nil, false
		}

		_, hasObjectKey := rawMap["object_key"]
		_, hasEventType := rawMap["event_type"]
		if hasObjectKey == hasEventType {
			audit.Valid = false
			audit.Error = fmt.Sprintf("ambiguous or unknown entry type at sequence %d", seq)
			return audit, nil, false
		}

		if hasObjectKey {
			var entry Entry
			if err := json.Unmarshal(data, &entry); err != nil {
				audit.Valid = false
				audit.Error = fmt.Sprintf("failed to parse entry at sequence %d: %v", seq, err)
				return audit, nil, false
			}

			if entry.Sequence != seq {
				audit.Valid = false
				audit.Error = fmt.Sprintf("sequence mismatch at %d: got %d", seq, entry.Sequence)
				return audit, nil, false
			}
			if entry.WriterID != expectedWriterID {
				audit.Valid = false
				audit.Error = fmt.Sprintf("writer ID mismatch at sequence %d: got %q", seq, entry.WriterID)
				return audit, nil, false
			}
			if entry.ObjectKey == "" || !isSHA256(entry.PlaintextSHA256) || !isSHA256(entry.ChainHash) || !isSHA256(entry.PrevChainHash) {
				audit.Valid = false
				audit.Error = fmt.Sprintf("invalid cryptographic fields at sequence %d", seq)
				return audit, nil, false
			}
			if entry.ChainHash != expectedChainHash {
				audit.Valid = false
				audit.Error = fmt.Sprintf("chain link mismatch at sequence %d", seq)
				return audit, nil, false
			}
			computedHash := computeChainHash(&entry, entry.PrevChainHash)
			if entry.ChainHash != computedHash {
				audit.Valid = false
				audit.Error = fmt.Sprintf("entry hash mismatch at sequence %d", seq)
				return audit, nil, false
			}

			trackedObjects[entry.ObjectKey] = true
			expectedChainHash = entry.PrevChainHash
			audit.EntriesVerified++
		} else {
			var keyEvent KeyEvent
			if err := json.Unmarshal(data, &keyEvent); err != nil {
				audit.Valid = false
				audit.Error = fmt.Sprintf("failed to parse key event at sequence %d: %v", seq, err)
				return audit, nil, false
			}

			if keyEvent.Sequence != seq {
				audit.Valid = false
				audit.Error = fmt.Sprintf("sequence mismatch at %d: got %d", seq, keyEvent.Sequence)
				return audit, nil, false
			}
			if keyEvent.WriterID != expectedWriterID {
				audit.Valid = false
				audit.Error = fmt.Sprintf("writer ID mismatch at sequence %d: got %q", seq, keyEvent.WriterID)
				return audit, nil, false
			}
			if !validKeyEventType(keyEvent.EventType) {
				audit.Valid = false
				audit.Error = fmt.Sprintf("unknown key event type %q at sequence %d", keyEvent.EventType, seq)
				return audit, nil, false
			}
			if !isSHA256(keyEvent.ChainHash) || !isSHA256(keyEvent.PrevChainHash) {
				audit.Valid = false
				audit.Error = fmt.Sprintf("invalid cryptographic fields at sequence %d", seq)
				return audit, nil, false
			}
			if keyEvent.ChainHash != expectedChainHash {
				audit.Valid = false
				audit.Error = fmt.Sprintf("chain link mismatch at sequence %d", seq)
				return audit, nil, false
			}
			computedHash := computeKeyEventHash(&keyEvent, keyEvent.PrevChainHash)
			if keyEvent.ChainHash != computedHash {
				audit.Valid = false
				audit.Error = fmt.Sprintf("key event hash mismatch at sequence %d", seq)
				return audit, nil, false
			}

			expectedChainHash = keyEvent.PrevChainHash
			audit.KeyEvents++
		}
	}

	if expectedChainHash != InitialChainHash {
		audit.Valid = false
		audit.Error = "chain does not link back to genesis"
		return audit, nil, false
	}

	for sequence := range entryKeys {
		if sequence > head.Sequence {
			audit.Valid = false
			audit.Error = fmt.Sprintf("chain entry %d exists beyond head sequence %d", sequence, head.Sequence)
			return audit, nil, true
		}
	}

	return audit, nil, false
}
				audit.Error = fmt.Sprintf("failed to parse entry at sequence %d: %v", seq, err)
				return audit, nil, false
			}

			if entry.Sequence != seq {
				audit.Valid = false
				audit.Error = fmt.Sprintf("sequence mismatch at %d: got %d", seq, entry.Sequence)
				return audit, nil, false
			}
			if entry.WriterID != expectedWriterID {
				audit.Valid = false
				audit.Error = fmt.Sprintf("writer ID mismatch at sequence %d: got %q", seq, entry.WriterID)
				return audit, nil, false
			}
			if entry.ObjectKey == "" || !isSHA256(entry.PlaintextSHA256) || !isSHA256(entry.ChainHash) || !isSHA256(entry.PrevChainHash) {
				audit.Valid = false
				audit.Error = fmt.Sprintf("invalid cryptographic fields at sequence %d", seq)
				return audit, nil, false
			}
			if entry.ChainHash != expectedChainHash {
				audit.Valid = false
				audit.Error = fmt.Sprintf("chain link mismatch at sequence %d", seq)
				return audit, nil, false
			}
			computedHash := computeChainHash(&entry, entry.PrevChainHash)
			if entry.ChainHash != computedHash {
				audit.Valid = false
				audit.Error = fmt.Sprintf("entry hash mismatch at sequence %d", seq)
				return audit, nil, false
			}

			trackedObjects[entry.ObjectKey] = true
			expectedChainHash = entry.PrevChainHash
			audit.EntriesVerified++
		} else {
			var keyEvent KeyEvent
			if err := json.Unmarshal(data, &keyEvent); err != nil {
				audit.Valid = false
				audit.Error = fmt.Sprintf("failed to parse key event at sequence %d: %v", seq, err)
				return audit, nil, false
			}

			if keyEvent.Sequence != seq {
				audit.Valid = false
				audit.Error = fmt.Sprintf("sequence mismatch at %d: got %d", seq, keyEvent.Sequence)
				return audit, nil, false
			}
			if keyEvent.WriterID != expectedWriterID {
				audit.Valid = false
				audit.Error = fmt.Sprintf("writer ID mismatch at sequence %d: got %q", seq, keyEvent.WriterID)
				return audit, nil, false
			}
			if !validKeyEventType(keyEvent.EventType) {
				audit.Valid = false
				audit.Error = fmt.Sprintf("unknown key event type %q at sequence %d", keyEvent.EventType, seq)
				return audit, nil, false
			}
			if !isSHA256(keyEvent.ChainHash) || !isSHA256(keyEvent.PrevChainHash) {
				audit.Valid = false
				audit.Error = fmt.Sprintf("invalid cryptographic fields at sequence %d", seq)
				return audit, nil, false
			}
			if keyEvent.ChainHash != expectedChainHash {
				audit.Valid = false
				audit.Error = fmt.Sprintf("chain link mismatch at sequence %d", seq)
				return audit, nil, false
			}
			computedHash := computeKeyEventHash(&keyEvent, keyEvent.PrevChainHash)
			if keyEvent.ChainHash != computedHash {
				audit.Valid = false
				audit.Error = fmt.Sprintf("key event hash mismatch at sequence %d", seq)
				return audit, nil, false
			}

			expectedChainHash = keyEvent.PrevChainHash
			audit.KeyEvents++
		}
	}

	if expectedChainHash != InitialChainHash {
		audit.Valid = false
		audit.Error = "chain does not link back to genesis"
		return audit, nil, false
	}

	for sequence := range entryKeys {
		if sequence > head.Sequence {
			audit.Valid = false
			audit.Error = fmt.Sprintf("chain entry %d exists beyond head sequence %d", sequence, head.Sequence)
			return audit, nil, true
		}
	}

	return audit, nil, false
}

// findUntrackedObjects lists all objects and finds those not in any chain.
func (a *Auditor) findUntrackedObjects(ctx context.Context, tracked map[string]bool, result *AuditResult) error {
	continuationToken := ""

	for {
		listResult, err := a.backend.List(ctx, a.bucket, a.prefix, "", continuationToken, 1000)
		if err != nil {
			return err
		}
		if listResult == nil {
			return fmt.Errorf("backend returned a nil object listing")
		}

		for _, obj := range listResult.Objects {
			logicalKey := obj.Key
			if a.prefix != "" {
				if !strings.HasPrefix(logicalKey, a.prefix) {
					return fmt.Errorf("backend returned key %q outside configured prefix %q", logicalKey, a.prefix)
				}
				logicalKey = strings.TrimPrefix(logicalKey, a.prefix)
			}

			if strings.HasPrefix(logicalKey, ".armor/") {
				continue
			}

			result.TotalObjects++

			// B2's List deliberately avoids a HEAD per object, so its ObjectInfo
			// cannot identify ARMOR metadata. Resolve metadata authoritatively here.
			isARMOREncrypted := obj.IsARMOREncrypted
			if !isARMOREncrypted {
				info, headErr := a.backend.Head(ctx, a.bucket, obj.Key)
				if headErr != nil {
					return fmt.Errorf("head object %q: %w", logicalKey, headErr)
				}
				if info == nil {
					return fmt.Errorf("head object %q: backend returned nil metadata", logicalKey)
				}
				isARMOREncrypted = info.IsARMOREncrypted
			}
			if !isARMOREncrypted {
				continue
			}

			if !tracked[logicalKey] {
				result.UntrackedObjects = append(result.UntrackedObjects, logicalKey)
			}
		}

		if !listResult.IsTruncated {
			break
		}
		if listResult.NextToken == "" || listResult.NextToken == continuationToken {
			return fmt.Errorf("backend returned a truncated object listing without a usable continuation token")
		}
		continuationToken = listResult.NextToken
	}

	return nil
}

// walkChainSegments walks chain segment files for a writer.
// Chain segments are compacted legacy chain entries stored as JSONL files
// at .armor/chain-segments/<writer>/<from>-<to>.jsonl.
func (a *Auditor) walkChainSegments(ctx context.Context, writerID string, fromSeq, toSeq int64) (map[int64]*Entry, map[string]bool, error) {
	const segmentPrefix = ".armor/chain-segments/"
	segmentsByRange := make(map[string]int64) // range -> max sequence in that segment

	// List all segment files for this writer
	prefix := segmentPrefix + writerID + "/"
	keys, err := a.listInternalKeys(ctx, prefix)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list chain segments: %w", err)
	}

	// Parse segment file names: <from>-<to>.jsonl
	for _, key := range keys {
		if !strings.HasSuffix(key, ".jsonl") {
			continue
		}
		relative := strings.TrimPrefix(key, prefix)
		parts := strings.Split(strings.TrimSuffix(relative, ".jsonl"), "-")
		if len(parts) != 2 {
			continue
		}
		from, err1 := strconv.ParseInt(parts[0], 10, 64)
		to, err2 := strconv.ParseInt(parts[1], 10, 64)
		if err1 != nil || err2 != nil || from > to {
			continue
		}
		// Keep track of the highest sequence in each segment
		if existing, ok := segmentsByRange[relative]; !ok || to > existing {
			segmentsByRange[relative] = to
		}
	}

	entries := make(map[int64]*Entry)
	trackedObjects := make(map[string]bool)

	// Read segment files that intersect with [fromSeq, toSeq]
	for rangeKey, maxSeq := range segmentsByRange {
		parts := strings.Split(strings.TrimSuffix(rangeKey, ".jsonl"), "-")
		segFrom, _ := strconv.ParseInt(parts[0], 10, 64)
		segTo, _ := strconv.ParseInt(parts[1], 10, 64)

		// Skip segments that don't intersect with our target range
		if segTo < fromSeq || segFrom > toSeq {
			continue
		}

		segmentKey := prefix + rangeKey
		body, _, err := a.backend.GetDirect(ctx, a.bucket, segmentKey)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to load chain segment %s: %w", segmentKey, err)
		}
		if body == nil {
			return nil, nil, fmt.Errorf("failed to load chain segment %s: backend returned nil body", segmentKey)
		}

		// Parse JSONL entries
		scanner := newJSONLScanner(body)
		for scanner.Scan() {
			var entry Entry
			if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
				body.Close()
				return nil, nil, fmt.Errorf("failed to parse entry in segment %s: %w", segmentKey, err)
			}
			// Only include entries in our target range
			if entry.Sequence >= fromSeq && entry.Sequence <= toSeq {
				entries[entry.Sequence] = &entry
				trackedObjects[entry.ObjectKey] = true
			}
		}
		if err := scanner.Err(); err != nil {
			body.Close()
			return nil, nil, fmt.Errorf("error reading segment %s: %w", segmentKey, err)
		}
		body.Close()
	}

	return entries, trackedObjects, nil
}

// walkDeltaEntries walks manifest delta files for a writer and extracts
// chain entries embedded in "put" operations.
// Delta files are at .armor/manifest/<writer>/delta-{seq:010d}.jsonl.
func (a *Auditor) walkDeltaEntries(ctx context.Context, writerID string, fromDeltaSeq uint64) (map[int64]*ChainEntryData, map[string]bool, error) {
	const manifestPrefix = ".armor/manifest/"

	entries := make(map[int64]*ChainEntryData)
	trackedObjects := make(map[string]bool)

	// List all delta files for this writer
	prefix := manifestPrefix + writerID + "/"
	keys, err := a.listInternalKeys(ctx, prefix)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list manifest deltas: %w", err)
	}

	// Filter and sort delta files, starting from fromDeltaSeq
	var deltaFiles []string
	for _, key := range keys {
		if !strings.HasPrefix(key, prefix+"delta-") || !strings.HasSuffix(key, ".jsonl") {
			continue
		}
		// Extract sequence number from filename
		relative := strings.TrimPrefix(key, prefix)
		seqStr := strings.TrimPrefix(relative, "delta-")
		seqStr = strings.TrimSuffix(seqStr, ".jsonl")
		seq, err := strconv.ParseUint(seqStr, 10, 64)
		if err != nil {
			continue
		}
		if seq < fromDeltaSeq {
			continue
		}
		deltaFiles = append(deltaFiles, key)
	}
	sort.Strings(deltaFiles)

	// Read delta files and extract chain entries
	for _, deltaKey := range deltaFiles {
		body, _, err := a.backend.GetDirect(ctx, a.bucket, deltaKey)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to load delta file %s: %w", deltaKey, err)
		}
		if body == nil {
			return nil, nil, fmt.Errorf("failed to load delta file %s: backend returned nil body", deltaKey)
		}

		// Parse JSONL operations
		scanner := newJSONLScanner(body)
		for scanner.Scan() {
			var op struct {
				Operation string       `json:"op"`
				Key       string       `json:"key"`
				Chain     *ChainEntryData `json:"chain,omitempty"`
			}
			if err := json.Unmarshal(scanner.Bytes(), &op); err != nil {
				body.Close()
				return nil, nil, fmt.Errorf("failed to parse operation in delta %s: %w", deltaKey, err)
			}
			// Extract chain entries from put operations
			if op.Operation == "put" && op.Chain != nil {
				entries[op.Chain.Sequence] = op.Chain
				// Track the object key (remove bucket prefix if present)
				objectKey := op.Key
				if idx := strings.Index(objectKey, "/"); idx >= 0 {
					objectKey = objectKey[idx+1:]
				}
				trackedObjects[objectKey] = true
			}
		}
		if err := scanner.Err(); err != nil {
			body.Close()
			return nil, nil, fmt.Errorf("error reading delta %s: %w", deltaKey, err)
		}
		body.Close()
	}

	return entries, trackedObjects, nil
}

// jsonLScanner wraps a reader to scan JSONL (one JSON object per line).
type jsonLScanner struct {
	scanner *bufio.Scanner
}

func newJSONLScanner(r io.Reader) *jsonLScanner {
	return &jsonLScanner{
		scanner: bufio.NewScanner(r),
	}
}

func (s *jsonLScanner) Scan() bool {
	return s.scanner.Scan()
}

func (s *jsonLScanner) Bytes() []byte {
	return s.scanner.Bytes()
}

func (s *jsonLScanner) Err() error {
	return s.scanner.Err()
}

func parseChainHeadKey(key string) (string, error) {
	if !strings.HasPrefix(key, ChainHeadPrefix) {
		return "", fmt.Errorf("invalid chain head key %q", key)
	}
	writerID := strings.TrimPrefix(key, ChainHeadPrefix)
	if writerID == "" || strings.Contains(writerID, "/") {
		return "", fmt.Errorf("invalid chain head key %q", key)
	}
	return writerID, nil
}

func parseChainEntryKey(key string) (string, int64, error) {
	if !strings.HasPrefix(key, ChainPrefix) {
		return "", 0, fmt.Errorf("invalid chain entry key %q", key)
	}
	relative := strings.TrimPrefix(key, ChainPrefix)
	parts := strings.Split(relative, "/")
	if len(parts) != 2 || parts[0] == "" || !strings.HasSuffix(parts[1], ".json") {
		return "", 0, fmt.Errorf("invalid chain entry key %q", key)
	}
	sequence, err := strconv.ParseInt(strings.TrimSuffix(parts[1], ".json"), 10, 64)
	if err != nil || sequence <= 0 || parts[1] != strconv.FormatInt(sequence, 10)+".json" {
		return "", 0, fmt.Errorf("invalid chain entry key %q", key)
	}
	return parts[0], sequence, nil
}

func isSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validKeyEventType(eventType string) bool {
	switch eventType {
	case "key-rotate-start", "key-rotate-complete", "key-export":
		return true
	default:
		return false
	}
}

func markInvalid(result *AuditResult) {
	if result.Status == "valid" {
		result.Status = "invalid"
	}
}

func markIncomplete(result *AuditResult) {
	result.Status = "incomplete"
}
