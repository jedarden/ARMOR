package handlers_test

// Latency-injected small-object full-GET benchmark (armor-50a36688).
//
// This is the controlled half of the small-object GET baseline: the backend is
// the in-memory callCountingBackend with a fixed artificial RTT injected
// before every call, so per-object latency is set purely by the number of
// sequential backend round trips the read path makes (pinned by the
// TestSmallObjectFullGETRequestCount* tests) — not by bandwidth, TLS, CPU or
// cache luck. The production half lives in the in-cluster probe documented
// alongside it in docs/performance/small-object-get-baseline.md.
//
// Run it explicitly (it skips in every normal gate):
//
//	ARMOR_SMALL_OBJECT_BENCH=1 go test ./internal/server/handlers/ \
//	  -run TestSmallObjectFullGETLatencyMatrix -v -timeout 45m
//
// ARMOR_SMALL_OBJECT_BENCH_RTT_MS overrides the per-call RTT (default 74, the
// measured cluster→us-west-002 round-trip time in docs/plan/plan.md).

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// benchCell is one matrix point: envelope version, object size, client
// concurrency.
type benchCell struct {
	version     int
	sizeBytes   int
	concurrency int
	// streaming disables the small-object single fetch (cfg.
	// SmallObjectFetchBytes = 0) so the A/B leg measures the pre-98864f12
	// per-region streaming shape in the same run at the same RTT.
	streaming bool
}

func TestSmallObjectFullGETLatencyMatrix(t *testing.T) {
	if os.Getenv("ARMOR_SMALL_OBJECT_BENCH") == "" {
		t.Skip("set ARMOR_SMALL_OBJECT_BENCH=1 to run the latency-injected small-object GET benchmark")
	}
	if testing.Short() {
		t.Skip("latency matrix is a long-running explicit benchmark")
	}

	rttMs := 74
	if v := os.Getenv("ARMOR_SMALL_OBJECT_BENCH_RTT_MS"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil || parsed <= 0 {
			t.Fatalf("ARMOR_SMALL_OBJECT_BENCH_RTT_MS must be a positive integer, got %q", v)
		}
		rttMs = parsed
	}
	rtt := time.Duration(rttMs) * time.Millisecond

	sizes := []int{4 * 1024, 32 * 1024, 256 * 1024, 1024 * 1024}
	concs := []int{1, 16, 64, 256}
	versions := []int{2, 3}

	var cells []benchCell
	for _, v := range versions {
		for _, sz := range sizes {
			for _, c := range concs {
				cells = append(cells, benchCell{version: v, sizeBytes: sz, concurrency: c})
			}
		}
		// The A/B leg: the same 32 KiB / c16 cell with the single fetch
		// disabled, so the improvement is measured against the streaming
		// shape in the same run instead of against a published number.
		cells = append(cells, benchCell{version: v, sizeBytes: 32 * 1024, concurrency: 16, streaming: true})
	}

	t.Logf("matrix: %d cells (versions %v x sizes %v x concurrency %v, plus %d streaming A/B cells), RTT %dms per backend call",
		len(cells), versions, sizes, concs, len(versions), rttMs)

	for _, cell := range cells {
		cell := cell
		name := fmt.Sprintf("v%d/%dKiB/c%d", cell.version, cell.sizeBytes/1024, cell.concurrency)
		if cell.streaming {
			name += "/streaming"
		}
		t.Run(name, func(t *testing.T) {
			latencies := runBenchCell(t, cell, rtt)
			reportBenchCell(t, cell, rtt, latencies)
		})
	}
}

// runBenchCell drives full-object GETs of one stored object from
// cell.concurrency client goroutines until exactly max(2*concurrency, 32)
// GETs completed, and returns the per-GET wall latencies in milliseconds.
func runBenchCell(t *testing.T, cell benchCell, rtt time.Duration) []float64 {
	t.Helper()
	const bucket = "bench-bucket"
	plaintext := smallGETPlaintext(cell.sizeBytes)

	cfg, cb, h, _ := countingTestSetup(t, cell.version, rtt)
	if cell.streaming {
		cfg.SmallObjectFetchBytes = 0
	} else {
		// The matrix measures the single-fetch path across its whole size
		// axis, so the threshold here clears the largest cell's STORED size:
		// a 1 MiB plaintext stores as plaintext + header + HMAC/block-table
		// expansion, which is over the 1 MiB production default. That
		// default still bounds the small-object population in production;
		// the at/over-threshold boundary is pinned by
		// TestSmallFetchBoundarySizes, not here. The buffer-slot cap that
		// must clear c256 is ReadBackendLimit, set inside countingTestSetup
		// before handlers.New sizes the channel.
		cfg.SmallObjectFetchBytes = 8 << 20
	}
	key := fmt.Sprintf("bench-v%d-%d.bin", cell.version, cell.sizeBytes)
	if cell.streaming {
		key += "-streaming"
	}
	putObjectThroughHandler(t, h, bucket, key, plaintext)
	cb.resetCalls()

	target := 2 * cell.concurrency
	if target < 32 {
		target = 32
	}

	var mu sync.Mutex
	latencies := make([]float64, 0, target)
	var issued atomic.Int64
	var failed atomic.Int64

	var wg sync.WaitGroup
	for w := 0; w < cell.concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				// Claim one of the target GET slots.
				if issued.Add(1) > int64(target) {
					return
				}

				req := httptest.NewRequest(http.MethodGet, "/"+bucket+"/"+key, nil)
				w := httptest.NewRecorder()
				start := time.Now()
				h.HandleRoot(w, req)
				elapsed := time.Since(start)

				if w.Code != http.StatusOK || w.Body.Len() != len(plaintext) {
					failed.Add(1)
					continue
				}

				mu.Lock()
				latencies = append(latencies, float64(elapsed.Microseconds())/1000.0)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if failed.Load() > 0 {
		t.Fatalf("%d benchmark GETs failed or returned wrong sizes", failed.Load())
	}
	if len(latencies) != target {
		t.Fatalf("collected %d latencies, expected %d", len(latencies), target)
	}

	// Every GET in this benchmark must make exactly the pinned number of
	// backend calls; a drift here belongs in the request-count tests first.
	// Single-fetch cells (armor-98864f12): 3 calls per GET — the overlapped
	// manifest probe ∥ plaintext-key HEAD pair (armor-420c15a5), then ONE
	// whole-envelope ranged request parsed locally. Nothing is cacheable and
	// nothing warms up: even v3's trailer table is sliced out of the fetched
	// buffer, so no generation-keyed table load happens on any GET.
	// Streaming A/B cells measure the pre-98864f12 shape: v2 makes 5 calls
	// per GET (probe ∥ HEAD, envelope header, inline HMAC prefetch, data);
	// v3 makes 4 (probe ∥ HEAD, header, data) plus exactly one generation-
	// keyed trailer-table fetch on the cell's first GET (armor-c3199f1c).
	callsPerGET, warmupCalls := 3, 0
	if cell.streaming {
		if cell.version == 3 {
			callsPerGET, warmupCalls = 4, 1
		} else {
			callsPerGET, warmupCalls = 5, 0
		}
	}
	totalCalls := len(cb.recordedCalls())
	if totalCalls != callsPerGET*target+warmupCalls {
		t.Fatalf("benchmark made %d backend calls for %d GETs (%.2f per GET), expected exactly %d per GET (+%d one-shot warm-up) — the read path changed; update the request-count pin and the baseline doc together",
			totalCalls, target, float64(totalCalls)/float64(target), callsPerGET, warmupCalls)
	}
	return latencies
}

// reportBenchCell prints one matrix row: per-object latency percentiles and
// achieved objects/s at this concurrency.
func reportBenchCell(t *testing.T, cell benchCell, rtt time.Duration, latencies []float64) {
	t.Helper()
	sorted := append([]float64(nil), latencies...)
	sort.Float64s(sorted)

	pct := func(p float64) float64 {
		idx := int(p * float64(len(sorted)-1))
		return sorted[idx]
	}
	totalMs := 0.0
	for _, l := range latencies {
		totalMs += l
	}
	objectsPerSec := float64(len(latencies)) / (totalMs / 1000.0)

	// Under a pure fixed-RTT model the expected per-object latency is the
	// critical-path round trip count x RTT regardless of size or concurrency
	// (the in-memory backend has no bandwidth term); the size axis exists to
	// demonstrate exactly that and to mirror the production probe's matrix.
	// Single-fetch cells (armor-98864f12): two round trips at every version —
	// the overlapped manifest probe ∥ plaintext-key HEAD (armor-420c15a5),
	// then the one whole-envelope request. Streaming A/B cells keep the
	// pre-98864f12 shape: four for v2 (probe ∥ HEAD, header, HMAC table,
	// data), three for v3 (the block table rides the generation-keyed cache,
	// armor-c3199f1c).
	criticalPath := 2
	if cell.streaming {
		criticalPath = 4
		if cell.version == 3 {
			criticalPath = 3
		}
	}
	expected := float64(criticalPath) * float64(rtt.Milliseconds())

	fetchMode := "single-fetch"
	if cell.streaming {
		fetchMode = "streaming"
	}
	t.Logf("RESULT version=%d size=%dKiB concurrency=%d n=%d p50=%.1fms p95=%.1fms p99=%.1fms max=%.1fms mean=%.1fms objs/s=%.2f mode=%s (expected ≈%.0fms = %d x RTT)",
		cell.version, cell.sizeBytes/1024, cell.concurrency, len(sorted),
		pct(0.50), pct(0.95), pct(0.99), sorted[len(sorted)-1], totalMs/float64(len(sorted)), objectsPerSec, fetchMode, expected, criticalPath)
}
