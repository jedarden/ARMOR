// Package provenance implements a cryptographic provenance chain for ARMOR.
// Each upload is linked to the previous one via a chain hash, creating a
// tamper-evident audit trail. Multiple ARMOR instances maintain independent
// per-writer chains that can be merged during audit.
package provenance

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/jedarden/armor/internal/backend"
)

const (
	// ChainPrefix is the prefix for chain entry objects in B2
	ChainPrefix = ".armor/chain/"

	// ChainHeadPrefix is the prefix for chain head objects in B2
	ChainHeadPrefix = ".armor/chain-head/"

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

// ChainHead represents the current head of a writer's chain.
type ChainHead struct {
	// WriterID identifies the ARMOR instance
	WriterID string `json:"writer_id"`

	// Sequence is the current sequence number
	Sequence int64 `json:"sequence"`

	// ChainHash is the chain hash of the most recent entry
	ChainHash string `json:"chain_hash"`

	// Updated is when the head was last updated
	Updated time.Time `json:"updated"`
}

// Manager handles provenance chain operations.
type Manager struct {
	backend backend.Backend
	bucket  string
	writerID string

	// In-memory cache of the current chain head
	mu       sync.RWMutex
	head     *ChainHead

	// Skip provenance for internal operations
	skipPrefixes []string
}

// NewManager creates a new provenance manager.
func NewManager(be backend.Backend, bucket, writerID string) *Manager {
	return &Manager{
		backend: be,
		bucket:  bucket,
		writerID: writerID,
		skipPrefixes: []string{
			".armor/",  // Internal ARMOR objects
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

// RecordUpload records an upload in the provenance chain.
// This should be called after a successful upload.
func (m *Manager) RecordUpload(ctx context.Context, objectKey, plaintextSHA256, operation string) error {
	// Skip internal objects
	if !m.ShouldRecord(objectKey) {
		return nil
	}

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

// AuditResult contains the result of a provenance chain audit.
type AuditResult struct {
	// Status is overall audit status: "valid", "invalid", "incomplete"
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
	WriterID      string `json:"writer_id"`
	HeadSequence  int64  `json:"head_sequence"`
	EntriesVerified int  `json:"entries_verified"`
	Valid         bool   `json:"valid"`
	Error         string `json:"error,omitempty"`
}

// GapInfo describes a gap in a chain.
type GapInfo struct {
	WriterID    string `json:"writer_id"`
	AfterSeq    int64  `json:"after_seq"`
	MissingSeq  int64  `json:"missing_seq"`
}

// Auditor performs provenance chain audits.
type Auditor struct {
	backend backend.Backend
	bucket  string
}

// NewAuditor creates a new provenance auditor.
func NewAuditor(be backend.Backend, bucket string) *Auditor {
	return &Auditor{
		backend: be,
		bucket:  bucket,
	}
}

// Audit performs a full provenance chain audit.
// It walks all writer chains, verifies integrity, and checks for untracked objects.
func (a *Auditor) Audit(ctx context.Context) (*AuditResult, error) {
	result := &AuditResult{
		Status: "valid",
	}

	// 1. Find all chain heads
	heads, err := a.listChainHeads(ctx)
	if err != nil {
		result.Status = "incomplete"
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to list chain heads: %v", err))
		return result, nil
	}

	// 2. Verify each writer's chain
	trackedObjects := make(map[string]bool)

	for _, head := range heads {
		writerAudit := a.auditWriterChain(ctx, head, trackedObjects)
		result.Writers = append(result.Writers, writerAudit)
		result.TotalEntries += int64(writerAudit.EntriesVerified)

		if !writerAudit.Valid {
			result.Status = "invalid"
		}
	}

	// 3. List all objects in bucket and find untracked ones
	if err := a.findUntrackedObjects(ctx, trackedObjects, result); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to find untracked objects: %v", err))
	}

	return result, nil
}

// listChainHeads lists all chain head objects in the bucket.
func (a *Auditor) listChainHeads(ctx context.Context) ([]*ChainHead, error) {
	var heads []*ChainHead

	continuationToken := ""
	for {
		result, err := a.backend.List(ctx, a.bucket, ChainHeadPrefix, "", continuationToken, 1000)
		if err != nil {
			return nil, err
		}

		for _, obj := range result.Objects {
			// Load the head
			body, _, err := a.backend.GetDirect(ctx, a.bucket, obj.Key)
			if err != nil {
				continue
			}

			data, err := io.ReadAll(body)
			body.Close()
			if err != nil {
				continue
			}

			var head ChainHead
			if err := json.Unmarshal(data, &head); err != nil {
				continue
			}

			heads = append(heads, &head)
		}

		if !result.IsTruncated {
			break
		}
		continuationToken = result.NextToken
	}

	return heads, nil
}

// auditWriterChain verifies a single writer's chain integrity.
func (a *Auditor) auditWriterChain(ctx context.Context, head *ChainHead, trackedObjects map[string]bool) WriterAudit {
	audit := WriterAudit{
		WriterID:     head.WriterID,
		HeadSequence: head.Sequence,
		Valid:        true,
	}

	// Walk the chain from head to genesis
	expectedSeq := head.Sequence
	expectedChainHash := head.ChainHash

	for seq := expectedSeq; seq > 0; seq-- {
		key := fmt.Sprintf("%s%s/%d.json", ChainPrefix, head.WriterID, seq)

		body, _, err := a.backend.GetDirect(ctx, a.bucket, key)
		if err != nil {
			audit.Valid = false
			audit.Error = fmt.Sprintf("Missing entry at sequence %d: %v", seq, err)
			return audit
		}

		data, err := io.ReadAll(body)
		body.Close()
		if err != nil {
			audit.Valid = false
			audit.Error = fmt.Sprintf("Failed to read entry at sequence %d: %v", seq, err)
			return audit
		}

		var entry Entry
		if err := json.Unmarshal(data, &entry); err != nil {
			audit.Valid = false
			audit.Error = fmt.Sprintf("Failed to parse entry at sequence %d: %v", seq, err)
			return audit
		}

		// Verify sequence
		if entry.Sequence != seq {
			audit.Valid = false
			audit.Error = fmt.Sprintf("Sequence mismatch at %d: got %d", seq, entry.Sequence)
			return audit
		}

		// Verify chain hash (for all but the first entry)
		if seq < expectedSeq && entry.ChainHash != expectedChainHash {
			audit.Valid = false
			audit.Error = fmt.Sprintf("Chain hash mismatch at sequence %d", seq)
			return audit
		}

		// Track the object
		trackedObjects[entry.ObjectKey] = true

		// Move to previous entry
		expectedChainHash = entry.PrevChainHash
		audit.EntriesVerified++
	}

	// Verify the chain links back to genesis
	if expectedChainHash != InitialChainHash {
		audit.Valid = false
		audit.Error = "Chain does not link back to genesis"
	}

	return audit
}

// findUntrackedObjects lists all objects and finds those not in any chain.
func (a *Auditor) findUntrackedObjects(ctx context.Context, tracked map[string]bool, result *AuditResult) error {
	continuationToken := ""

	for {
		listResult, err := a.backend.List(ctx, a.bucket, "", "", continuationToken, 1000)
		if err != nil {
			return err
		}

		for _, obj := range listResult.Objects {
			// Skip internal objects
			if len(obj.Key) >= 7 && obj.Key[:7] == ".armor/" {
				continue
			}

			result.TotalObjects++

			// Check if it's an ARMOR-encrypted object
			if !obj.IsARMOREncrypted {
				// Non-ARMOR objects are expected to be untracked
				continue
			}

			// Check if tracked
			if !tracked[obj.Key] {
				result.UntrackedObjects = append(result.UntrackedObjects, obj.Key)
			}
		}

		if !listResult.IsTruncated {
			break
		}
		continuationToken = listResult.NextToken
	}

	// If there are untracked objects, the audit is invalid
	if len(result.UntrackedObjects) > 0 {
		result.Status = "invalid"
	}

	return nil
}
