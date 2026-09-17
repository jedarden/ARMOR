package provenance

// ADR-001 "Internal Namespaces" addendum: manifest deltas are tenant-scoped
// internal bookkeeping, so they sit at <ARMOR_PREFIX>.armor/manifest/ once a
// prefix is in force — and the auditor's delta walk follows the tenant prefix
// rather than the bucket-root location its own chain records use. These tests
// pin that composition: with a prefix the walk reads the prefixed tree and
// ignores a pre-prefix root delta sitting next to it.

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"strings"
	"testing"
)

// TestWalkDeltaEntriesFollowsTenantPrefix builds the auditor the server builds
// (NewAuditorWithPrefix over cfg.Prefix, which normalizePrefix keeps with its
// trailing slash) and verifies the walk finds deltas under
// p/.armor/manifest/<writer>/ while a same-writer delta left at the
// pre-prefix root location stays out of the result.
func TestWalkDeltaEntriesFollowsTenantPrefix(t *testing.T) {
	const (
		bucket   = "test-bucket"
		prefix   = "p/"
		writerID = "writer-p"
	)

	mb := newMockBackend()
	ctx := context.Background()

	// The delta the tenant's manifest writer produced, at the composed
	// location, carrying one chain entry for a put.
	entries := []struct {
		op    string
		key   string
		chain *ChainEntryData
	}{
		{
			op:  "put",
			key: bucket + "/ledger/row-1",
			chain: &ChainEntryData{
				Sequence:      1,
				ChainHash:     strings.Repeat("a", sha256.Size*2),
				PrevChainHash: InitialChainHash,
			},
		},
	}
	prefixedDelta, err := marshalProvenanceDelta(entries)
	if err != nil {
		t.Fatalf("marshal prefixed delta: %v", err)
	}
	prefixedKey := prefix + ".armor/manifest/" + writerID + "/delta-0000000001.jsonl"
	if err := mb.Put(ctx, bucket, prefixedKey, strings.NewReader(prefixedDelta), int64(len(prefixedDelta)), nil); err != nil {
		t.Fatalf("put prefixed delta: %v", err)
	}

	// A pre-prefix era delta for the same writer at the bucket root, carrying
	// a distinguishable sequence. If the walk dropped the prefix it would list
	// the root tree and this entry would surface.
	legacyDelta, err := marshalProvenanceDelta([]struct {
		op    string
		key   string
		chain *ChainEntryData
	}{
		{
			op:  "put",
			key: bucket + "/ledger/legacy-echo",
			chain: &ChainEntryData{
				Sequence:      7,
				ChainHash:     strings.Repeat("b", sha256.Size*2),
				PrevChainHash: InitialChainHash,
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal legacy delta: %v", err)
	}
	legacyKey := ".armor/manifest/" + writerID + "/delta-0000000007.jsonl"
	if err := mb.Put(ctx, bucket, legacyKey, strings.NewReader(legacyDelta), int64(len(legacyDelta)), nil); err != nil {
		t.Fatalf("put legacy root delta: %v", err)
	}

	auditor := NewAuditorWithPrefix(mb, bucket, prefix)
	walked, tracked, err := auditor.walkDeltaEntries(ctx, writerID, 1)
	if err != nil {
		t.Fatalf("walkDeltaEntries: %v", err)
	}

	if len(walked) != 1 {
		t.Fatalf("walk returned %d entries, want 1 (the prefixed delta only)", len(walked))
	}
	if walked[1] == nil || walked[1].ChainHash != strings.Repeat("a", sha256.Size*2) {
		t.Fatalf("walk did not return the prefixed delta's entry for sequence 1: %+v", walked[1])
	}
	if walked[7] != nil {
		t.Errorf("pre-prefix root delta leaked into the walk: sequence 7 = %+v", walked[7])
	}
	if !tracked["ledger/row-1"] {
		t.Errorf("prefixed delta's object not tracked, tracked = %+v", tracked)
	}
	if tracked["ledger/legacy-echo"] {
		t.Errorf("root delta's object tracked, tracked = %+v", tracked)
	}
}

// marshalProvenanceDelta renders delta operations as the JSONL body the
// manifest writer embeds chain entries in.
func marshalProvenanceDelta(ops []struct {
	op    string
	key   string
	chain *ChainEntryData
},
) (string, error) {
	var lines []string
	for _, deltaOp := range ops {
		line, err := json.Marshal(struct {
			Operation string          `json:"op"`
			Key       string          `json:"key"`
			Chain     *ChainEntryData `json:"chain,omitempty"`
		}{
			Operation: deltaOp.op,
			Key:       deltaOp.key,
			Chain:     deltaOp.chain,
		})
		if err != nil {
			return "", err
		}
		lines = append(lines, string(line))
	}
	return strings.Join(lines, "\n") + "\n", nil
}
