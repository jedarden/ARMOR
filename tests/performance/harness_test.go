package performance

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// The tests in this file are deterministic: they assert on backend request
// counts, request bytes, overlap (via injected per-call latency), and hash
// verification — never on wall-clock speed. They are safe for ordinary CI and
// `go test -short`.

func newTestEnv(t *testing.T, compress bool) *LocalEnv {
	t.Helper()
	env, err := NewLocalEnv("test", compress)
	if err != nil {
		t.Fatalf("boot local env: %v", err)
	}
	t.Cleanup(env.Close)
	return env
}

// TestFullGetRequestCountsBounded pins the service full-GET read shape: a
// 256 KiB object (4 × 64 KiB blocks) must be served with a small, bounded
// number of backend range fetches carrying roughly the stored bytes — not one
// fetch per block on the hot path, and not the whole bucket.
func TestFullGetRequestCountsBounded(t *testing.T) {
	env := newTestEnv(t, false)
	env.Backend.Delay = 5 * time.Millisecond

	body := SyntheticBody(256 << 10)
	key := BenchPrefix + "fullget"
	if _, err := env.PutObject(key, body); err != nil {
		t.Fatalf("put: %v", err)
	}
	baseCounts, baseBytes := env.Backend.Snapshot()

	sample, err := env.GetObject(key, false)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if sample.SHA256 != hashHex(body) {
		t.Fatalf("payload hash mismatch: got %s want %s", sample.SHA256, hashHex(body))
	}
	if sample.TTFB < env.Backend.Delay {
		t.Fatalf("TTFB %v below one injected backend call (%v) — backend latency not on the read critical path?", sample.TTFB, env.Backend.Delay)
	}

	counts, bytes := env.Backend.Snapshot()
	ranges := counts[opGetRange] - baseCounts[opGetRange]
	servedBytes := bytes[opGetRange] - baseBytes[opGetRange]
	if ranges < 1 || ranges > 5 {
		t.Fatalf("full GET of a 4-block object caused %d backend range fetches, want 1..5 (shape drift)", ranges)
	}
	if servedBytes > int64(len(body))+128<<10 {
		t.Fatalf("full GET fetched %d backend bytes for a %d-byte object — unbounded over-fetch", servedBytes, len(body))
	}
}

// TestSuffixRangeFetchesOnlyTail pins the v3 ranged-read shape: reading the
// last 64 KiB of an 8 MiB object must fetch only tail blocks from the backend,
// not stream the whole object.
func TestSuffixRangeFetchesOnlyTail(t *testing.T) {
	env := newTestEnv(t, false)

	body := SyntheticBody(8 << 20)
	key := BenchPrefix + "suffix"
	if _, err := env.PutObject(key, body); err != nil {
		t.Fatalf("put: %v", err)
	}
	baseCounts, baseBytes := env.Backend.Snapshot()

	sample, err := env.GetRange(key, "bytes=-65536", false)
	if err != nil {
		t.Fatalf("suffix range get: %v", err)
	}
	if sample.Bytes != 64<<10 {
		t.Fatalf("suffix range returned %d bytes, want %d", sample.Bytes, 64<<10)
	}

	_, bytes := env.Backend.Snapshot()
	servedBytes := bytes[opGetRange] - baseBytes[opGetRange]
	// 64 KiB payload + tail-block alignment + envelope header + HMAC table —
	// anything near the full 8 MiB means the range path degraded to full read.
	if servedBytes > 512<<10 {
		t.Fatalf("suffix range fetched %d backend bytes for a 64 KiB tail of an 8 MiB object — range path not selective", servedBytes)
	}
	counts, _ := env.Backend.Snapshot()
	if r := counts[opGetRange] - baseCounts[opGetRange]; r > 6 {
		t.Fatalf("suffix range caused %d backend fetches, want a bounded tail (< 6)", r)
	}
}

// TestMultipartPutOneUploadPerPart pins the multipart write shape: one
// CreateMultipartUpload, exactly one backend UploadPart per client part, one
// Complete; and the acknowledged object reads back identical through the
// ordinary service GET.
func TestMultipartPutOneUploadPerPart(t *testing.T) {
	env := newTestEnv(t, false)

	parts := [][]byte{SyntheticBody(5 << 20), SyntheticBody(5 << 20), SyntheticBody(5 << 20)}
	key := BenchPrefix + "multipart-shape"
	baseCounts, _ := env.Backend.Snapshot()

	pr, err := env.MultipartPut(key, parts)
	if err != nil {
		t.Fatalf("multipart put: %v", err)
	}
	counts, _ := env.Backend.Snapshot()
	if got := counts[opCreateMultipart] - baseCounts[opCreateMultipart]; got != 1 {
		t.Fatalf("CreateMultipartUpload count = %d, want 1", got)
	}
	if got := counts[opUploadPart] - baseCounts[opUploadPart]; got != int64(len(parts)) {
		t.Fatalf("UploadPart count = %d, want %d", got, len(parts))
	}
	if got := counts[opCompleteMultipart] - baseCounts[opCompleteMultipart]; got != 1 {
		t.Fatalf("CompleteMultipartUpload count = %d, want 1", got)
	}

	got, err := env.GetObject(key, false)
	if err != nil {
		t.Fatalf("verify read-back: %v", err)
	}
	if got.SHA256 != pr.PlaintextSHA || got.Bytes != pr.Bytes {
		t.Fatalf("read-back mismatch: sha %s/%s bytes %d/%d", got.SHA256, pr.PlaintextSHA, got.Bytes, pr.Bytes)
	}
}

// TestConcurrentUploadsOverlapAtBackend proves the overlap detector works in
// both directions: with a 100 ms injected backend latency, two clients
// uploading their own objects concurrently must overlap at the backend (max
// in-flight ≥ 2), while one client uploading parts sequentially must not.
func TestConcurrentUploadsOverlapAtBackend(t *testing.T) {
	parts := func() [][]byte {
		p := make([][]byte, 3)
		for i := range p {
			p[i] = SyntheticBody(5 << 20)
		}
		return p
	}()

	envSeq := newTestEnv(t, false)
	envSeq.Backend.Delay = 100 * time.Millisecond
	if _, err := envSeq.MultipartPut(BenchPrefix+"seq", parts); err != nil {
		t.Fatalf("sequential multipart: %v", err)
	}
	if got := envSeq.Backend.MaxInflight(); got > 2 {
		t.Fatalf("sequential single-client upload hit backend in-flight %d — overlapped unexpectedly", got)
	}

	envConc := newTestEnv(t, false)
	envConc.Backend.Delay = 100 * time.Millisecond
	var wg sync.WaitGroup
	for c := 0; c < 2; c++ {
		wg.Add(1)
		go func(c int) {
			defer wg.Done()
			if _, err := envConc.MultipartPut(fmt.Sprintf("%sconc-%d", BenchPrefix, c), parts); err != nil {
				t.Errorf("concurrent client %d: %v", c, err)
			}
		}(c)
	}
	wg.Wait()
	if got := envConc.Backend.MaxInflight(); got < 2 {
		t.Fatalf("two concurrent client uploads never overlapped at the backend (max in-flight %d)", got)
	}
}

// TestCompressedPathSeparation covers the ADR-007 single-PUT compression
// path's write-side and its boundary: compressible payloads must store
// smaller than plaintext, and ranged reads must be refused on this path.
//
// Full read round-trip is deliberately NOT asserted here: on committed main
// (verified 2026-09-18 against a clean HEAD export) the compressed read path
// fails with "HMAC table too short at block 0" and ARMOR's own
// share_decompress tests fail on the same export — a pre-existing defect,
// recorded on the benchmark bead for separate tracking. The opt-in baseline
// still measures this path and reports the hash mismatch honestly.
func TestCompressedPathSeparation(t *testing.T) {
	env := newTestEnv(t, true)

	body := SyntheticCompressible(1 << 20)
	key := BenchPrefix + "compressed"
	_, baseBytes := env.Backend.Snapshot()
	if _, err := env.PutObject(key, body); err != nil {
		t.Fatalf("compressed put: %v", err)
	}
	_, bytes := env.Backend.Snapshot()
	storedPut := bytes[opPut] - baseBytes[opPut]
	if storedPut >= int64(len(body)) {
		t.Fatalf("compressible payload stored at %d bytes (plaintext %d) — compression path not active", storedPut, len(body))
	}

	// Ranged reads are unsupported on the compressed path — expect an error
	// status, not silent full-object delivery.
	if _, err := env.GetRange(key, "bytes=0-1023", false); err == nil {
		t.Fatalf("range GET on compressed object unexpectedly succeeded (ADR-007 says ranges unsupported)")
	}
}

// TestBaselineTinyMatrix is an end-to-end smoke of the whole measurement
// pipeline at the smallest object size: every scenario must produce a result
// and every sample must verify. It is not a timing assertion — it only proves
// the harness plumbing (scenarios, verification, output files) works.
func TestBaselineTinyMatrix(t *testing.T) {
	outDir := t.TempDir()
	cfg := MatrixConfig{
		Sizes:            []string{"64KiB"},
		ReadSamples:      1,
		WriteSamples:     1,
		ClientConcs:      []int{2},
		MultipartClients: 2,
		PartSize:         8 << 20,
		OutDir:           outDir,
		// Compressed scenarios stay out of the CI smoke: the compressed read
		// path has a pre-existing round-trip defect on committed main (see
		// TestCompressedPathSeparation). The opt-in baseline measures it.
		Compressed: false,
	}
	report, err := RunBaseline(cfg)
	if err != nil {
		t.Fatalf("tiny baseline: %v", err)
	}
	if len(report.Scenarios) == 0 {
		t.Fatalf("tiny baseline produced no scenarios")
	}
	names := map[string]bool{}
	for _, s := range report.Scenarios {
		if s.Samples <= 0 {
			t.Errorf("scenario %s: samples = %d", s.Name, s.Samples)
		}
		if !s.Verified {
			t.Errorf("scenario %s failed verification: %s", s.Name, s.VerifyNote)
		}
		names[s.Name] = true
	}
	for _, want := range []string{"read-full-cold/v3/64kib", "read-full-warm/v3/64kib", "read-range-suffix/v3/64kib", "write-single-put/v3/64kib", "read-full-warm/v3/64kib/clients=2"} {
		if !names[want] {
			t.Errorf("missing scenario %q; got %v", want, keysOf(names))
		}
	}
	for _, f := range []string{"results.json", "results.md"} {
		if _, err := os.Stat(filepath.Join(outDir, f)); err != nil {
			t.Errorf("baseline output missing %s: %v", f, err)
		}
	}
}

// TestUncommittedPayloadRoundTrip guards the harness itself: capture-mode GET
// must return the exact bytes written.
func TestUncommittedPayloadRoundTrip(t *testing.T) {
	env := newTestEnv(t, false)
	body := SyntheticBody(3*64<<10 + 17)
	key := BenchPrefix + "capture"
	if _, err := env.PutObject(key, body); err != nil {
		t.Fatalf("put: %v", err)
	}
	sample, err := env.GetObject(key, true)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if string(sample.Received) != string(body) {
		t.Fatalf("captured payload differs: got %d bytes want %d", len(sample.Received), len(body))
	}
}

func keysOf(m map[string]bool) string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return strings.Join(out, ", ")
}
