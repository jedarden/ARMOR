package provenance

// ADR-001 "Internal Namespaces" addendum, chain side: the provenance chain is
// the one internal namespace that does NOT compose onto ARMOR_PREFIX. Chain
// records stay at the bucket-level .armor/ tree on a prefixed deployment —
// NewManager takes no prefix and NewAuditorWithPrefix's doc calls the
// exception out — while the same auditor's manifest delta walk follows the
// tenant prefix (manifest_delta_walk_prefix_test.go). These tests pin the
// exception from both sides: a prefixed auditor must find root chain records
// (or every prefixed deployment's audit would come up empty), and it must
// NOT discover chain-shaped objects planted under the composed location (or
// one tenant's chain would bleed into another's audit).

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

// TestChainRecordsStayAtBucketRootUnderPrefix records a chain through the
// manager the server builds (NewManager has no prefix parameter), then runs
// the audit the server's /admin/provenance endpoint runs
// (NewAuditorWithPrefix over the armed prefix) and requires the root chain to
// be found intact — with nothing discovered from the composed location.
func TestChainRecordsStayAtBucketRootUnderPrefix(t *testing.T) {
	const (
		bucket   = "test-bucket"
		prefix   = "p/"
		writerID = "writer-p"
	)

	mb := newMockBackend()
	ctx := context.Background()

	// The manager server.New builds: bucket-level, no prefix. Two recorded
	// uploads give the writer a two-entry chain with a head.
	m := NewManager(mb, bucket, writerID)
	for i := 1; i <= 2; i++ {
		key := fmt.Sprintf("ledger/row-%d", i)
		sha := fmt.Sprintf("%x", sha256.Sum256([]byte(key)))
		if err := m.RecordUpload(ctx, key, sha, "put"); err != nil {
			t.Fatalf("RecordUpload(%s): %v", key, err)
		}
	}

	// The records physically sit at the BUCKET ROOT: head and both entries
	// under .armor/, never under p/.armor/ — the documented exception.
	headKey := ChainHeadPrefix + writerID
	if _, ok := mb.objects[bucket+"/"+headKey]; !ok {
		t.Fatalf("chain head missing at the bucket-root name %s", headKey)
	}
	for seq := 1; seq <= 2; seq++ {
		entryKey := fmt.Sprintf("%s%s/%d.json", ChainPrefix, writerID, seq)
		if _, ok := mb.objects[bucket+"/"+entryKey]; !ok {
			t.Errorf("chain entry missing at the bucket-root name %s", entryKey)
		}
	}
	for k := range mb.objects {
		if strings.HasPrefix(k, bucket+"/"+prefix+".armor/chain") {
			t.Errorf("chain record written under the composed location %s — the chain must stay at the bucket root", k)
		}
	}

	// A chain-shaped object planted at the COMPOSED location, headless. If
	// chain discovery ever started composing the tenant prefix, this would
	// surface as an orphan writer ("chain entries exist without a chain
	// head") and flip the audit status below.
	ghostKey := prefix + strings.TrimPrefix(fmt.Sprintf("%s%s/%d.json", ChainPrefix, "writer-ghost", 9), "/")
	if err := mb.Put(ctx, bucket, ghostKey, strings.NewReader("{}"), 2, nil); err != nil {
		t.Fatalf("seed composed-location ghost entry: %v", err)
	}

	// The auditor the server builds for a prefixed deployment finds the root
	// chain and verifies it; the composed-location ghost stays invisible.
	auditor := NewAuditorWithPrefix(mb, bucket, prefix)
	result, err := auditor.Audit(ctx)
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}

	if result.Status != "valid" {
		t.Fatalf("audit status = %s, want valid; errors: %v", result.Status, result.Errors)
	}
	if len(result.Writers) != 1 {
		t.Fatalf("audit discovered %d writers, want 1 (the root chain's): %+v", len(result.Writers), result.Writers)
	}
	w := result.Writers[0]
	if w.WriterID != writerID {
		t.Errorf("writer ID = %q, want %q", w.WriterID, writerID)
	}
	if !w.Valid {
		t.Errorf("writer audit not valid: %s", w.Error)
	}
	if w.EntriesVerified != 2 {
		t.Errorf("entries verified = %d, want 2", w.EntriesVerified)
	}
	if result.TotalEntries != 2 {
		t.Errorf("total entries = %d, want 2", result.TotalEntries)
	}
	for _, writer := range result.Writers {
		if writer.WriterID == "writer-ghost" {
			t.Errorf("composed-location chain object discovered as writer %q — chain discovery must not compose the tenant prefix", writer.WriterID)
		}
	}
}
