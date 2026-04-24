// Package manifest provides an in-memory index for ARMOR object metadata
// with B2-backed persistence via snapshot files and append-only delta logs.
//
// The manifest is a performance optimization that eliminates HeadObject B2
// API calls by caching decryption metadata (IV, wrapped DEK, plaintext size)
// for all tracked objects in memory. It is not authoritative — B2 object
// headers remain the source of truth.
//
// B2 layout per writer:
//
//	{prefix}/{writer_id}/snapshot.json.gz        — full compacted index
//	{prefix}/{writer_id}/delta-{seq:010d}.jsonl  — incremental delta files
package manifest

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// DefaultPrefix is the B2 object-key prefix for all manifest blobs.
const DefaultPrefix = ".armor/manifest"

// Entry holds the decryption metadata for a single tracked object.
// All fields needed to decrypt the object without a B2 HeadObject call.
type Entry struct {
	PlaintextSize   int64     `json:"plaintext_size"`
	PlaintextSHA256 string    `json:"plaintext_sha256"`
	IV              []byte    `json:"iv"`
	WrappedDEK      []byte    `json:"wrapped_dek"`
	BlockSize       int       `json:"block_size"`
	ContentType     string    `json:"content_type"`
	ETag            string    `json:"etag"`
	LastModified    time.Time `json:"last_modified"`
}

// Op is a single delta log entry. The field name "op" matches the wire format
// documented in the plan.
type Op struct {
	Operation string    `json:"op"`              // "put" or "del"
	Key       string    `json:"key"`             // "bucket/object-key"
	Entry     *Entry    `json:"entry,omitempty"` // nil for "del"
	Ts        time.Time `json:"ts"`
}

// Index is a concurrent in-memory map from "bucket/object-key" to Entry.
type Index struct {
	mu      sync.RWMutex
	entries map[string]*Entry
	seq     uint64 // last written delta sequence number
}

// New returns a new empty Index.
func New() *Index {
	return &Index{entries: make(map[string]*Entry)}
}

// indexKey returns the canonical map key: "bucket/object-key".
func indexKey(bucket, objectKey string) string {
	return bucket + "/" + objectKey
}

// Put adds or replaces the entry for bucket + objectKey.
func (idx *Index) Put(bucket, objectKey string, entry *Entry) {
	k := indexKey(bucket, objectKey)
	idx.mu.Lock()
	idx.entries[k] = entry
	idx.mu.Unlock()
}

// Delete removes the entry for bucket + objectKey.
func (idx *Index) Delete(bucket, objectKey string) {
	k := indexKey(bucket, objectKey)
	idx.mu.Lock()
	delete(idx.entries, k)
	idx.mu.Unlock()
}

// Get returns the entry for bucket + objectKey. Returns (nil, false) if absent.
func (idx *Index) Get(bucket, objectKey string) (*Entry, bool) {
	k := indexKey(bucket, objectKey)
	idx.mu.RLock()
	e, ok := idx.entries[k]
	idx.mu.RUnlock()
	return e, ok
}

// Len returns the number of tracked entries.
func (idx *Index) Len() int {
	idx.mu.RLock()
	n := len(idx.entries)
	idx.mu.RUnlock()
	return n
}

// Seq returns the current delta sequence counter.
func (idx *Index) Seq() uint64 {
	idx.mu.RLock()
	s := idx.seq
	idx.mu.RUnlock()
	return s
}

// SetSeq sets the delta sequence counter. Used during startup to restore state
// after replaying delta files.
func (idx *Index) SetSeq(seq uint64) {
	idx.mu.Lock()
	idx.seq = seq
	idx.mu.Unlock()
}

// IncSeq atomically increments and returns the next delta sequence number.
func (idx *Index) IncSeq() uint64 {
	idx.mu.Lock()
	idx.seq++
	s := idx.seq
	idx.mu.Unlock()
	return s
}

// All returns a shallow copy of all entries, keyed by "bucket/object-key".
// The returned map is independent of the index and safe to read without locks.
func (idx *Index) All() map[string]*Entry {
	idx.mu.RLock()
	out := make(map[string]*Entry, len(idx.entries))
	for k, v := range idx.entries {
		out[k] = v
	}
	idx.mu.RUnlock()
	return out
}

// Merge incorporates all entries from src (keyed by "bucket/object-key") into
// idx. When the same key exists in both, last-write-wins by LastModified.
// Used during startup to merge manifests from multiple writer shards.
func (idx *Index) Merge(src map[string]*Entry) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	for k, incoming := range src {
		existing, ok := idx.entries[k]
		if !ok || incoming.LastModified.After(existing.LastModified) {
			idx.entries[k] = incoming
		}
	}
}

// MarshalSnapshot serializes the index to a gzip-compressed JSON blob suitable
// for storage as snapshot.json.gz in B2.
func (idx *Index) MarshalSnapshot() ([]byte, error) {
	entries := idx.All()
	raw, err := json.Marshal(entries)
	if err != nil {
		return nil, fmt.Errorf("marshal snapshot: %w", err)
	}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(raw); err != nil {
		return nil, fmt.Errorf("gzip write snapshot: %w", err)
	}
	if err := gz.Close(); err != nil {
		return nil, fmt.Errorf("gzip close snapshot: %w", err)
	}
	return buf.Bytes(), nil
}

// UnmarshalSnapshot replaces the index contents from a gzip-compressed JSON
// snapshot. The sequence counter is not stored in the snapshot — callers must
// set it separately via SetSeq after determining the latest delta sequence.
func (idx *Index) UnmarshalSnapshot(data []byte) error {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("gzip open snapshot: %w", err)
	}
	defer gz.Close()
	var entries map[string]*Entry
	if err := json.NewDecoder(gz).Decode(&entries); err != nil {
		return fmt.Errorf("json decode snapshot: %w", err)
	}
	if entries == nil {
		entries = make(map[string]*Entry)
	}
	idx.mu.Lock()
	idx.entries = entries
	idx.mu.Unlock()
	return nil
}

// MarshalDelta serializes a slice of Ops to JSONL (one JSON object per line).
func MarshalDelta(ops []Op) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for i := range ops {
		if err := enc.Encode(ops[i]); err != nil {
			return nil, fmt.Errorf("encode delta op %d: %w", i, err)
		}
	}
	return buf.Bytes(), nil
}

// UnmarshalDelta replays a JSONL delta log against the index. Entries are
// applied in file order; later entries override earlier ones for the same key.
func (idx *Index) UnmarshalDelta(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	idx.mu.Lock()
	defer idx.mu.Unlock()
	for dec.More() {
		var op Op
		if err := dec.Decode(&op); err != nil {
			return fmt.Errorf("decode delta op: %w", err)
		}
		switch op.Operation {
		case "put":
			if op.Entry == nil {
				return fmt.Errorf("put op missing entry for key %q", op.Key)
			}
			idx.entries[op.Key] = op.Entry
		case "del":
			delete(idx.entries, op.Key)
		default:
			return fmt.Errorf("unknown delta op %q", op.Operation)
		}
	}
	return nil
}

// SnapshotKey returns the B2 object key for the snapshot blob.
//
//	e.g. ".armor/manifest/writer-1/snapshot.json.gz"
func SnapshotKey(prefix, writerID string) string {
	return prefix + "/" + writerID + "/snapshot.json.gz"
}

// DeltaKey returns the B2 object key for a specific delta file.
// seq is zero-padded to 10 digits so lexicographic sort equals numeric sort.
//
//	e.g. ".armor/manifest/writer-1/delta-0000000001.jsonl"
func DeltaKey(prefix, writerID string, seq uint64) string {
	return fmt.Sprintf("%s/%s/delta-%010d.jsonl", prefix, writerID, seq)
}

// DeltaSeqFromKey extracts the sequence number from a delta object key.
// Returns (seq, true) on success; (0, false) if key does not match the pattern.
func DeltaSeqFromKey(prefix, writerID, key string) (uint64, bool) {
	expected := prefix + "/" + writerID + "/delta-"
	if len(key) <= len(expected) || key[:len(expected)] != expected {
		return 0, false
	}
	suffix := key[len(expected):]
	// suffix must be exactly "NNNNNNNNNN.jsonl" (10 digits + ".jsonl" = 16 chars)
	if len(suffix) != 16 || suffix[10:] != ".jsonl" {
		return 0, false
	}
	var seq uint64
	for _, c := range suffix[:10] {
		if c < '0' || c > '9' {
			return 0, false
		}
		seq = seq*10 + uint64(c-'0')
	}
	return seq, true
}

// WriterPrefix returns the B2 listing prefix for all objects of a given writer.
//
//	e.g. ".armor/manifest/writer-1/"
func WriterPrefix(prefix, writerID string) string {
	return prefix + "/" + writerID + "/"
}
