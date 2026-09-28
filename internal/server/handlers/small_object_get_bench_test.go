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
	}

	t.Logf("matrix: %d cells (versions %v x sizes %v x concurrency %v), RTT %dms per backend call",
		len(cells), versions, sizes, concs, rttMs)

	for _, cell := range cells {
		cell := cell
		t.Run(fmt.Sprintf("v%d/%dKiB/c%d", cell.version, cell.sizeBytes/1024, cell.concurrency), func(t *testing.T) {
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
	key := fmt.Sprintf("bench-v%d-%d.bin", cell.version, cell.sizeBytes)
	plaintext := smallGETPlaintext(cell.sizeBytes)

	_, cb, h, _ := countingTestSetup(t, cell.version, rtt)
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
	const callsPerGET = 6
	totalCalls := len(cb.recordedCalls())
	if totalCalls != callsPerGET*target {
		t.Fatalf("benchmark made %d backend calls for %d GETs (%.2f per GET), expected exactly %d per GET — the read path changed; update the request-count pin and the baseline doc together",
			totalCalls, target, float64(totalCalls)/float64(target), callsPerGET)
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

	// Under a pure fixed-RTT model the expected per-object latency is
	// callsPerGET x RTT regardless of size or concurrency (the in-memory
	// backend has no bandwidth term); the size axis exists to demonstrate
	// exactly that and to mirror the production probe's matrix.
	expected := float64(6) * float64(rtt.Milliseconds())

	t.Logf("RESULT version=%d size=%dKiB concurrency=%d n=%d p50=%.1fms p95=%.1fms p99=%.1fms max=%.1fms mean=%.1fms objs/s=%.2f (expected ≈%.0fms = 6 x RTT)",
		cell.version, cell.sizeBytes/1024, cell.concurrency, len(sorted),
		pct(0.50), pct(0.95), pct(0.99), sorted[len(sorted)-1], totalMs/float64(len(sorted)), objectsPerSec, expected)
}
