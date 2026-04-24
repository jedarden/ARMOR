package manifest_test

import (
	"testing"
	"time"

	"github.com/jedarden/armor/internal/manifest"
)

func sampleEntry(size int64) *manifest.Entry {
	return &manifest.Entry{
		PlaintextSize:   size,
		PlaintextSHA256: "deadbeef",
		IV:              []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		WrappedDEK:      []byte{17, 18, 19, 20},
		BlockSize:       65536,
		ContentType:     "application/octet-stream",
		ETag:            "etag-abc",
		LastModified:    time.Now().UTC().Truncate(time.Millisecond),
	}
}

func TestPutGetDelete(t *testing.T) {
	idx := manifest.New()

	if _, ok := idx.Get("bucket", "key"); ok {
		t.Fatal("expected miss on empty index")
	}

	idx.Put("bucket", "key", sampleEntry(1024))
	if idx.Len() != 1 {
		t.Fatalf("len after put: got %d, want 1", idx.Len())
	}

	e, ok := idx.Get("bucket", "key")
	if !ok {
		t.Fatal("expected hit after put")
	}
	if e.PlaintextSize != 1024 {
		t.Fatalf("wrong size: got %d, want 1024", e.PlaintextSize)
	}

	// Overwrite
	idx.Put("bucket", "key", sampleEntry(2048))
	e2, _ := idx.Get("bucket", "key")
	if e2.PlaintextSize != 2048 {
		t.Fatalf("overwrite: want 2048, got %d", e2.PlaintextSize)
	}

	idx.Delete("bucket", "key")
	if _, ok := idx.Get("bucket", "key"); ok {
		t.Fatal("expected miss after delete")
	}
	if idx.Len() != 0 {
		t.Fatalf("len after delete: got %d, want 0", idx.Len())
	}
}

func TestDeleteNonExistent(t *testing.T) {
	idx := manifest.New()
	idx.Delete("bucket", "nonexistent") // must not panic
}

func TestAll(t *testing.T) {
	idx := manifest.New()
	idx.Put("b", "k1", sampleEntry(1))
	idx.Put("b", "k2", sampleEntry(2))

	all := idx.All()
	if len(all) != 2 {
		t.Fatalf("All(): want 2, got %d", len(all))
	}
	if all["b/k1"] == nil || all["b/k2"] == nil {
		t.Fatal("All(): missing expected keys")
	}
}

func TestSnapshotRoundtrip(t *testing.T) {
	idx := manifest.New()
	idx.Put("bucket", "a/b/c.parquet", sampleEntry(42000))
	idx.Put("bucket", "x/y.csv", sampleEntry(100))

	data, err := idx.MarshalSnapshot()
	if err != nil {
		t.Fatalf("MarshalSnapshot: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("MarshalSnapshot returned empty data")
	}

	idx2 := manifest.New()
	if err := idx2.UnmarshalSnapshot(data); err != nil {
		t.Fatalf("UnmarshalSnapshot: %v", err)
	}

	if idx2.Len() != 2 {
		t.Fatalf("after load: want 2 entries, got %d", idx2.Len())
	}
	e, ok := idx2.Get("bucket", "a/b/c.parquet")
	if !ok {
		t.Fatal("a/b/c.parquet missing after snapshot roundtrip")
	}
	if e.PlaintextSize != 42000 {
		t.Fatalf("wrong size: %d", e.PlaintextSize)
	}
	if e.PlaintextSHA256 != "deadbeef" {
		t.Fatalf("wrong sha256: %s", e.PlaintextSHA256)
	}
	if e.BlockSize != 65536 {
		t.Fatalf("wrong block size: %d", e.BlockSize)
	}
}

func TestSnapshotEmpty(t *testing.T) {
	idx := manifest.New()
	data, err := idx.MarshalSnapshot()
	if err != nil {
		t.Fatalf("MarshalSnapshot empty: %v", err)
	}
	idx2 := manifest.New()
	if err := idx2.UnmarshalSnapshot(data); err != nil {
		t.Fatalf("UnmarshalSnapshot empty: %v", err)
	}
	if idx2.Len() != 0 {
		t.Fatalf("expected empty index, got %d entries", idx2.Len())
	}
}

func TestDeltaRoundtrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	ops := []manifest.Op{
		{Operation: "put", Key: "bucket/file1.parquet", Entry: sampleEntry(500), Ts: now},
		{Operation: "put", Key: "bucket/file2.parquet", Entry: sampleEntry(600), Ts: now},
		{Operation: "del", Key: "bucket/file1.parquet", Ts: now},
	}

	data, err := manifest.MarshalDelta(ops)
	if err != nil {
		t.Fatalf("MarshalDelta: %v", err)
	}

	idx := manifest.New()
	if err := idx.UnmarshalDelta(data); err != nil {
		t.Fatalf("UnmarshalDelta: %v", err)
	}

	if idx.Len() != 1 {
		t.Fatalf("want 1 entry (file1 deleted), got %d", idx.Len())
	}
	if _, ok := idx.Get("bucket", "file1.parquet"); ok {
		t.Fatal("file1 should be deleted")
	}
	e, ok := idx.Get("bucket", "file2.parquet")
	if !ok {
		t.Fatal("file2 should exist")
	}
	if e.PlaintextSize != 600 {
		t.Fatalf("wrong size: %d", e.PlaintextSize)
	}
}

func TestDeltaAppliedOnTopOfSnapshot(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)

	idx := manifest.New()
	idx.Put("bucket", "old.parquet", sampleEntry(111))

	snap, _ := idx.MarshalSnapshot()

	ops := []manifest.Op{
		{Operation: "put", Key: "bucket/new.parquet", Entry: sampleEntry(222), Ts: now},
		{Operation: "del", Key: "bucket/old.parquet", Ts: now},
	}
	delta, _ := manifest.MarshalDelta(ops)

	idx2 := manifest.New()
	if err := idx2.UnmarshalSnapshot(snap); err != nil {
		t.Fatalf("UnmarshalSnapshot: %v", err)
	}
	if err := idx2.UnmarshalDelta(delta); err != nil {
		t.Fatalf("UnmarshalDelta: %v", err)
	}

	if idx2.Len() != 1 {
		t.Fatalf("want 1 entry, got %d", idx2.Len())
	}
	if _, ok := idx2.Get("bucket", "old.parquet"); ok {
		t.Fatal("old.parquet should be deleted")
	}
	if _, ok := idx2.Get("bucket", "new.parquet"); !ok {
		t.Fatal("new.parquet should exist")
	}
}

func TestDeltaUnknownOp(t *testing.T) {
	ops := []manifest.Op{{Operation: "invalid", Key: "bucket/key", Ts: time.Now()}}
	data, _ := manifest.MarshalDelta(ops)
	if err := manifest.New().UnmarshalDelta(data); err == nil {
		t.Fatal("expected error for unknown op")
	}
}

func TestDeltaPutMissingEntry(t *testing.T) {
	ops := []manifest.Op{{Operation: "put", Key: "bucket/key", Entry: nil, Ts: time.Now()}}
	data, _ := manifest.MarshalDelta(ops)
	if err := manifest.New().UnmarshalDelta(data); err == nil {
		t.Fatal("expected error for put with nil entry")
	}
}

func TestDeltaEmpty(t *testing.T) {
	data, err := manifest.MarshalDelta(nil)
	if err != nil {
		t.Fatalf("MarshalDelta nil: %v", err)
	}
	if err := manifest.New().UnmarshalDelta(data); err != nil {
		t.Fatalf("UnmarshalDelta empty: %v", err)
	}
}

func TestMergeLastWriteWins(t *testing.T) {
	now := time.Now().UTC()
	earlier := now.Add(-time.Minute)

	idx := manifest.New()
	idx.Put("bucket", "shared.parquet", &manifest.Entry{PlaintextSize: 100, LastModified: earlier})
	idx.Put("bucket", "local-only.parquet", &manifest.Entry{PlaintextSize: 200, LastModified: earlier})

	src := map[string]*manifest.Entry{
		"bucket/shared.parquet": {PlaintextSize: 999, LastModified: now},   // newer → wins
		"bucket/src-only.parquet": {PlaintextSize: 300, LastModified: now}, // new key
	}
	idx.Merge(src)

	// Newer src value wins for shared key
	e, ok := idx.Get("bucket", "shared.parquet")
	if !ok {
		t.Fatal("shared.parquet missing after merge")
	}
	if e.PlaintextSize != 999 {
		t.Fatalf("expected 999 (src wins), got %d", e.PlaintextSize)
	}

	// Local-only entry is preserved
	if _, ok := idx.Get("bucket", "local-only.parquet"); !ok {
		t.Fatal("local-only.parquet should remain")
	}

	// src-only entry is added
	if _, ok := idx.Get("bucket", "src-only.parquet"); !ok {
		t.Fatal("src-only.parquet should be merged in")
	}
}

func TestMergeOlderSrcDoesNotOverwrite(t *testing.T) {
	now := time.Now().UTC()
	earlier := now.Add(-time.Minute)

	idx := manifest.New()
	idx.Put("bucket", "key", &manifest.Entry{PlaintextSize: 100, LastModified: now})

	src := map[string]*manifest.Entry{
		"bucket/key": {PlaintextSize: 50, LastModified: earlier}, // older → loses
	}
	idx.Merge(src)

	e, _ := idx.Get("bucket", "key")
	if e.PlaintextSize != 100 {
		t.Fatalf("existing entry should win: got %d, want 100", e.PlaintextSize)
	}
}

func TestSeqCounter(t *testing.T) {
	idx := manifest.New()
	if idx.Seq() != 0 {
		t.Fatalf("initial seq should be 0, got %d", idx.Seq())
	}

	s1 := idx.IncSeq()
	if s1 != 1 {
		t.Fatalf("first IncSeq: want 1, got %d", s1)
	}
	s2 := idx.IncSeq()
	if s2 != 2 {
		t.Fatalf("second IncSeq: want 2, got %d", s2)
	}
	if idx.Seq() != 2 {
		t.Fatalf("Seq after two IncSeq: want 2, got %d", idx.Seq())
	}

	idx.SetSeq(100)
	if idx.Seq() != 100 {
		t.Fatalf("after SetSeq(100): want 100, got %d", idx.Seq())
	}
}

func TestDeltaKey(t *testing.T) {
	prefix := ".armor/manifest"
	writerID := "writer-1"

	key := manifest.DeltaKey(prefix, writerID, 1)
	want := ".armor/manifest/writer-1/delta-0000000001.jsonl"
	if key != want {
		t.Fatalf("DeltaKey: got %q, want %q", key, want)
	}

	seq, ok := manifest.DeltaSeqFromKey(prefix, writerID, key)
	if !ok {
		t.Fatal("DeltaSeqFromKey returned false")
	}
	if seq != 1 {
		t.Fatalf("DeltaSeqFromKey: got %d, want 1", seq)
	}
}

func TestDeltaKeyLargeSeq(t *testing.T) {
	prefix := ".armor/manifest"
	writerID := "writer-abc"
	const bigSeq = uint64(9999999999)

	key := manifest.DeltaKey(prefix, writerID, bigSeq)
	seq, ok := manifest.DeltaSeqFromKey(prefix, writerID, key)
	if !ok {
		t.Fatalf("DeltaSeqFromKey returned false for key %q", key)
	}
	if seq != bigSeq {
		t.Fatalf("DeltaSeqFromKey: got %d, want %d", seq, bigSeq)
	}
}

func TestDeltaSeqFromKeyInvalid(t *testing.T) {
	prefix := ".armor/manifest"
	writerID := "writer-1"

	cases := []string{
		"",
		".armor/manifest/writer-1/snapshot.json.gz",
		".armor/manifest/writer-1/delta-abc.jsonl",
		".armor/manifest/writer-1/delta-000000001.jsonl",  // 9 digits
		".armor/manifest/writer-1/delta-00000000001.jsonl", // 11 digits
	}
	for _, tc := range cases {
		_, ok := manifest.DeltaSeqFromKey(prefix, writerID, tc)
		if ok {
			t.Errorf("DeltaSeqFromKey(%q) should return false", tc)
		}
	}
}

func TestSnapshotKey(t *testing.T) {
	got := manifest.SnapshotKey(".armor/manifest", "writer-1")
	want := ".armor/manifest/writer-1/snapshot.json.gz"
	if got != want {
		t.Fatalf("SnapshotKey: got %q, want %q", got, want)
	}
}

func TestWriterPrefix(t *testing.T) {
	got := manifest.WriterPrefix(".armor/manifest", "writer-1")
	want := ".armor/manifest/writer-1/"
	if got != want {
		t.Fatalf("WriterPrefix: got %q, want %q", got, want)
	}
}
