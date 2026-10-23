package performance

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestLargeObjectRangeReadBaseline is an opt-in, local benchmark for the
// range-read path on one large v3 single-PUT object. It is separate from the
// broader matrix so the report can be compared without write, full-read, or
// compression rows obscuring the range-specific backend cost.
//
//	ARMOR_PERF_RUN=1 go test ./tests/performance -run TestLargeObjectRangeReadBaseline -v
//
// The default object is 64 MiB. Set ARMOR_PERF_LARGE_OBJECT to opt into a
// larger object (for example, 1GiB), ARMOR_PERF_READ_SAMPLES to change the
// measured samples, and ARMOR_PERF_OUT to choose the report directory.
func TestLargeObjectRangeReadBaseline(t *testing.T) {
	if testing.Short() {
		t.Skip("large-object range baseline is measurement work, not a -short test")
	}
	if os.Getenv("ARMOR_PERF_RUN") != "1" {
		t.Skip("set ARMOR_PERF_RUN=1 to run the large-object range measurement")
	}

	objectSize := envSize(t, "ARMOR_PERF_LARGE_OBJECT", "64MiB")
	samples := envPositiveInt(t, "ARMOR_PERF_READ_SAMPLES", 5)
	backendRTTMs := 0
	if configured := os.Getenv("ARMOR_PERF_BACKEND_RTT_MS"); configured != "" {
		parsed, err := strconv.Atoi(configured)
		if err != nil || parsed < 0 {
			t.Fatalf("ARMOR_PERF_BACKEND_RTT_MS=%q must be a nonnegative integer", configured)
		}
		backendRTTMs = parsed
	}
	writeVersion := 3
	if selected := os.Getenv("ARMOR_PERF_WRITE_VERSION"); selected != "" {
		parsed, err := strconv.Atoi(selected)
		if err != nil || (parsed != 2 && parsed != 3) {
			t.Fatalf("ARMOR_PERF_WRITE_VERSION=%q must be 2 or 3", selected)
		}
		writeVersion = parsed
	}
	if objectSize < 40<<20 {
		t.Fatalf("large-object benchmark requires at least 40MiB, got %d bytes", objectSize)
	}

	env, err := NewLocalEnvWithVersion(fmt.Sprintf("large-object-range-v%d", writeVersion), false, writeVersion)
	if err != nil {
		t.Fatalf("boot local env: %v", err)
	}
	defer env.Close()
	env.Backend.Delay = time.Duration(backendRTTMs) * time.Millisecond

	body := SyntheticBody(int(objectSize))
	key := fmt.Sprintf("%slarge-range-%d", BenchPrefix, objectSize)
	if _, err := env.PutObject(key, body); err != nil {
		t.Fatalf("put %s: %v", key, err)
	}

	results := make([]largeRangeResult, 0, len(largeRangeSpecs(objectSize))+4)
	for _, spec := range largeRangeSpecs(objectSize) {
		result, err := measureLargeRange(env, key, body, spec, samples, "single-put")
		if err != nil {
			t.Fatalf("measure %s: %v", spec.Name, err)
		}
		results = append(results, result)
		if !result.Verified {
			t.Errorf("range %s failed verification: %s", spec.Name, result.VerifyNote)
		}
	}

	// Parquet-footer leg (armor-c3199f1c): the synthetic body's tail never
	// passes backend.IsParquetFile, so the rows above cannot engage the
	// decrypted-footer cache. This object carries a well-formed Parquet
	// tail — footerLen metadata bytes, the uint32 LE footer length, and the
	// PAR1 magic — so a read covering the whole footer stores it and every
	// measured sample must be served from it. A random tail is not enough:
	// its length field parses as an absurd footer length and
	// backend.GetParquetFooterRange rejects the object, and a read larger
	// than the cached footer falls back to block fetches by design.
	parquetBody := make([]byte, len(body))
	copy(parquetBody, body)
	const parquetFooterLen = 64_000
	binary.LittleEndian.PutUint32(parquetBody[len(parquetBody)-8:], parquetFooterLen)
	copy(parquetBody[len(parquetBody)-4:], "PAR1")
	parquetKey := fmt.Sprintf("%slarge-range-parquet-%d", BenchPrefix, objectSize)
	if _, err := env.PutObject(parquetKey, parquetBody); err != nil {
		t.Fatalf("put %s: %v", parquetKey, err)
	}
	footerSlice := int64(parquetFooterLen + 8)
	parquetFooterSpec := largeRangeSpec{
		Name:          "parquet-footer-fetch",
		Header:        fmt.Sprintf("bytes=-%d", footerSlice),
		Start:         objectSize - footerSlice,
		ResponseBytes: footerSlice,
		WantFooterHit: true,
	}
	// The 8-byte footer-length peek models the second half of a columnar
	// engine's footer probe. It must be measured AFTER the footer-fetch row:
	// an 8-byte cold read cannot store the footer (the handler needs the
	// full footer to cache it), while a warm peek lies entirely inside the
	// cached footer and must hit.
	parquetPeekSpec := largeRangeSpec{
		Name:          "parquet-footer-peek-8B",
		Header:        "bytes=-8",
		Start:         objectSize - 8,
		ResponseBytes: 8,
		WantFooterHit: true,
	}
	for _, spec := range []largeRangeSpec{parquetFooterSpec, parquetPeekSpec} {
		result, err := measureLargeRange(env, parquetKey, parquetBody, spec, samples, "single-put")
		if err != nil {
			t.Fatalf("measure %s: %v", spec.Name, err)
		}
		results = append(results, result)
		if !result.Verified {
			t.Errorf("range %s failed verification: %s", spec.Name, result.VerifyNote)
		}
	}
	if writeVersion == 3 {
		multipartKey := fmt.Sprintf("%slarge-range-multipart-%d", BenchPrefix, objectSize)
		const partSize = 8 << 20
		parts := make([][]byte, 0, (len(body)+partSize-1)/partSize)
		for offset := 0; offset < len(body); offset += partSize {
			end := offset + partSize
			if end > len(body) {
				end = len(body)
			}
			parts = append(parts, body[offset:end])
		}
		if _, err := env.MultipartPut(multipartKey, parts); err != nil {
			t.Fatalf("multipart put %s: %v", multipartKey, err)
		}
		boundaryStart := int64(partSize - 32<<10)
		boundarySpec := largeRangeSpec{
			Name:          "cross-part-boundary-64KiB",
			Header:        fmt.Sprintf("bytes=%d-%d", boundaryStart, boundaryStart+(64<<10)-1),
			Start:         boundaryStart,
			ResponseBytes: 64 << 10,
		}
		for _, spec := range []largeRangeSpec{largeRangeSpecs(objectSize)[2], boundarySpec, largeRangeSpecs(objectSize)[4]} {
			result, err := measureLargeRange(env, multipartKey, body, spec, samples, "multipart")
			if err != nil {
				t.Fatalf("measure multipart %s: %v", spec.Name, err)
			}
			results = append(results, result)
			if !result.Verified {
				t.Errorf("multipart range %s failed verification: %s", spec.Name, result.VerifyNote)
			}
		}

		// The same footer leg against the v3 multipart layout, whose range
		// dispatch integrates the same decrypted-footer fast path. The final
		// part ends in the well-formed Parquet tail, so the committed
		// multipart plaintext does too.
		multipartParquetKey := fmt.Sprintf("%slarge-range-multipart-parquet-%d", BenchPrefix, objectSize)
		parquetParts := make([][]byte, 0, (len(parquetBody)+partSize-1)/partSize)
		for offset := 0; offset < len(parquetBody); offset += partSize {
			end := offset + partSize
			if end > len(parquetBody) {
				end = len(parquetBody)
			}
			parquetParts = append(parquetParts, parquetBody[offset:end])
		}
		if _, err := env.MultipartPut(multipartParquetKey, parquetParts); err != nil {
			t.Fatalf("multipart put %s: %v", multipartParquetKey, err)
		}
		for _, spec := range []largeRangeSpec{parquetFooterSpec, parquetPeekSpec} {
			result, err := measureLargeRange(env, multipartParquetKey, parquetBody, spec, samples, "multipart")
			if err != nil {
				t.Fatalf("measure multipart %s: %v", spec.Name, err)
			}
			results = append(results, result)
			if !result.Verified {
				t.Errorf("multipart range %s failed verification: %s", spec.Name, result.VerifyNote)
			}
		}
	}

	report := largeRangeReport{
		GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
		ARMORCommit:  benchmarkCommit(),
		GoVersion:    runtime.Version(),
		Environment:  environment(),
		ObjectFormat: fmt.Sprintf("v%d single-PUT%s envelope, 64 KiB blocks; AES-256-CTR + HMAC-SHA256", writeVersion, map[bool]string{true: " and multipart", false: ""}[writeVersion == 3]),
		ObjectBytes:  objectSize,
		Samples:      samples,
		BackendRTTMs: backendRTTMs,
		TestLocation: "tests/performance (in-process service over filesystem backend)",
		Scenarios:    results,
		Notes: []string{
			"each object is synthetic incompressible data and is written once through the real authenticated S3 path",
			"each range is warmed once before measurement; latency and backend bytes are warm-metadata samples",
			"backend bytes are the bytes requested from the instrumented backend by Get/GetRange, including envelope metadata and ciphertext/HMAC ranges",
			"local loopback and filesystem results are a service-shape baseline, not B2, Cloudflare, or production throughput evidence",
			"parquet-footer rows are written with a well-formed Parquet tail (64,000-byte footer, uint32 LE length, PAR1 magic); their warm samples must come from the decrypted-footer cache — the row fails otherwise — and cold_backend_fetches/cold_backend_bytes in the JSON record the uncached first read every row's warm samples avoid",
		},
	}

	outDir := os.Getenv("ARMOR_PERF_OUT")
	if outDir == "" {
		outDir = filepath.Join(os.TempDir(), "armor-perf-results")
	}
	if err := writeLargeRangeReport(outDir, report); err != nil {
		t.Fatalf("write large-range report: %v", err)
	}
	t.Logf("large-object range baseline written to %s", filepath.Join(outDir, "large-range-results.md"))
}

type largeRangeSpec struct {
	Name          string
	Header        string
	Start         int64
	ResponseBytes int64
	// WantFooterHit marks scenarios whose measured samples must be served
	// from the decrypted Parquet-footer cache (X-Armor-Footer-Cache: HIT);
	// a sample that falls back to block fetches fails the row.
	WantFooterHit bool
}

// largeRangeSpecs models the common access shapes that matter for large
// columnar objects: one block, a short unaligned read, a multi-block read, and
// a footer-like suffix read.
func largeRangeSpecs(objectSize int64) []largeRangeSpec {
	block := int64(64 << 10)
	return []largeRangeSpec{
		{
			Name:          "aligned-64KiB",
			Header:        fmt.Sprintf("bytes=%d-%d", 32<<20, 32<<20+block-1),
			Start:         32 << 20,
			ResponseBytes: block,
		},
		{
			Name:          "unaligned-32KiB",
			Header:        fmt.Sprintf("bytes=%d-%d", 32<<20+1234, 32<<20+1234+(32<<10)-1),
			Start:         32<<20 + 1234,
			ResponseBytes: 32 << 10,
		},
		{
			Name:          "aligned-1MiB",
			Header:        fmt.Sprintf("bytes=%d-%d", 16<<20, 16<<20+(1<<20)-1),
			Start:         16 << 20,
			ResponseBytes: 1 << 20,
		},
		{
			Name:          "aligned-8MiB",
			Header:        fmt.Sprintf("bytes=%d-%d", 24<<20, 24<<20+(8<<20)-1),
			Start:         24 << 20,
			ResponseBytes: 8 << 20,
		},
		{
			Name:          "suffix-64KiB",
			Header:        "bytes=-65536",
			Start:         objectSize - block,
			ResponseBytes: block,
		},
	}
}

type largeRangeResult struct {
	Name                       string  `json:"name"`
	WriteVersion               int     `json:"write_version"`
	WritePath                  string  `json:"write_path"`
	Range                      string  `json:"range"`
	ObjectBytes                int64   `json:"object_bytes"`
	ResponseBytes              int64   `json:"response_bytes"`
	Samples                    int     `json:"samples"`
	LatencyMsP50               float64 `json:"latency_ms_p50"`
	LatencyMsP95               float64 `json:"latency_ms_p95"`
	BackendFetches             int64   `json:"backend_fetches"`
	BackendFetchesPerSample    float64 `json:"backend_fetches_per_sample"`
	BackendBytes               int64   `json:"backend_bytes"`
	BackendBytesPerSample      float64 `json:"backend_bytes_per_sample"`
	BackendToResponseByteRatio float64 `json:"backend_to_response_byte_ratio"`
	FooterCacheHits            int64   `json:"footer_cache_hits"`
	ColdBackendFetches         int64   `json:"cold_backend_fetches"`
	ColdBackendBytes           int64   `json:"cold_backend_bytes"`
	Verified                   bool    `json:"verified"`
	VerifyNote                 string  `json:"verify_note,omitempty"`
}

type largeRangeReport struct {
	GeneratedAt  string             `json:"generated_at"`
	ARMORCommit  string             `json:"armor_commit"`
	GoVersion    string             `json:"go_version"`
	Environment  string             `json:"environment"`
	ObjectFormat string             `json:"object_format"`
	ObjectBytes  int64              `json:"object_bytes"`
	Samples      int                `json:"samples"`
	BackendRTTMs int                `json:"backend_rtt_ms"`
	TestLocation string             `json:"test_location"`
	Scenarios    []largeRangeResult `json:"scenarios"`
	Notes        []string           `json:"notes"`
}

func measureLargeRange(env *LocalEnv, key string, body []byte, spec largeRangeSpec, samples int, writePath string) (largeRangeResult, error) {
	// Warm metadata before taking the counters so backend bytes measure the
	// range workload rather than the one-time object-header lookup. The
	// snapshot pair around the warm call also records the cold (uncached)
	// read cost that the measured warm samples avoid.
	coldCounts, coldBytes := env.Backend.Snapshot()
	warm, err := env.GetRange(key, spec.Header, true)
	if err != nil {
		return largeRangeResult{}, fmt.Errorf("warmup: %w", err)
	}
	expected := body[spec.Start : spec.Start+spec.ResponseBytes]
	if !bytes.Equal(warm.Received, expected) {
		return largeRangeResult{}, fmt.Errorf("warmup payload mismatch: got %d bytes, want %d", len(warm.Received), len(expected))
	}
	warmCounts, warmBytes := env.Backend.Snapshot()
	coldFetches := (warmCounts[opGet] - coldCounts[opGet]) + (warmCounts[opGetRange] - coldCounts[opGetRange])
	coldFetchedBytes := (warmBytes[opGet] - coldBytes[opGet]) + (warmBytes[opGetRange] - coldBytes[opGetRange])

	baseCounts, baseBytes := env.Backend.Snapshot()
	latencies := make([]float64, 0, samples)
	verified := true
	verifyNote := "every sample byte-compared with the exact plaintext slice"
	if spec.WantFooterHit {
		verifyNote = "every sample byte-compared with the exact plaintext slice and carried X-Armor-Footer-Cache: HIT"
	}
	var footerHits int64
	for i := 0; i < samples; i++ {
		sample, err := env.GetRange(key, spec.Header, true)
		if err != nil {
			return largeRangeResult{}, fmt.Errorf("sample %d: %w", i+1, err)
		}
		if !bytes.Equal(sample.Received, expected) {
			verified = false
			verifyNote = fmt.Sprintf("sample %d did not match the exact plaintext slice", i+1)
		}
		if sample.FooterCacheHit {
			footerHits++
			if !spec.WantFooterHit {
				verified = false
				verifyNote = fmt.Sprintf("sample %d was served from the footer cache although this scenario measures the block path", i+1)
			}
		} else if spec.WantFooterHit {
			verified = false
			verifyNote = fmt.Sprintf("sample %d missed the footer cache (no X-Armor-Footer-Cache: HIT header)", i+1)
		}
		latencies = append(latencies, fms(sample.Duration))
	}

	counts, fetchedBytes := env.Backend.Snapshot()
	backendFetches := (counts[opGet] - baseCounts[opGet]) + (counts[opGetRange] - baseCounts[opGetRange])
	backendBytes := (fetchedBytes[opGet] - baseBytes[opGet]) + (fetchedBytes[opGetRange] - baseBytes[opGetRange])
	return largeRangeResult{
		Name:                       spec.Name,
		WriteVersion:               env.WriteVersion,
		WritePath:                  writePath,
		Range:                      spec.Header,
		ObjectBytes:                int64(len(body)),
		ResponseBytes:              spec.ResponseBytes,
		Samples:                    samples,
		LatencyMsP50:               percentile(latencies, 50),
		LatencyMsP95:               percentile(latencies, 95),
		BackendFetches:             backendFetches,
		BackendFetchesPerSample:    float64(backendFetches) / float64(samples),
		BackendBytes:               backendBytes,
		BackendBytesPerSample:      float64(backendBytes) / float64(samples),
		BackendToResponseByteRatio: float64(backendBytes) / float64(spec.ResponseBytes*int64(samples)),
		FooterCacheHits:            footerHits,
		ColdBackendFetches:         coldFetches,
		ColdBackendBytes:           coldFetchedBytes,
		Verified:                   verified,
		VerifyNote:                 verifyNote,
	}, nil
}

func writeLargeRangeReport(outDir string, report largeRangeReport) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "large-range-results.json"), data, 0o644); err != nil {
		return err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# ARMOR large-object range baseline — %s\n\n", report.GeneratedAt)
	fmt.Fprintf(&b, "- ARMOR commit: `%s`\n- Go: %s\n- Environment: %s\n- Object: %d bytes (%s)\n- Format: %s\n- Samples per row: %d\n- Injected backend RTT: %d ms per call\n- Location: %s\n\n",
		report.ARMORCommit, report.GoVersion, report.Environment, report.ObjectBytes, formatBytes(report.ObjectBytes), report.ObjectFormat, report.Samples, report.BackendRTTMs, report.TestLocation)
	b.WriteString("All rows are warm-metadata samples. Backend bytes include every byte returned by the instrumented Get/GetRange calls; the ratio is backend bytes divided by response bytes across all samples.\n\n")
	b.WriteString("| write version | write path | range | response | latency p50 ms | latency p95 ms | backend fetches/sample | backend bytes/sample | backend/response | verified |\n")
	b.WriteString("|---|---|---|---:|---:|---:|---:|---:|---:|---|\n")
	for _, s := range report.Scenarios {
		fmt.Fprintf(&b, "| v%d | %s | %s (`%s`) | %s | %.2f | %.2f | %.1f | %s | %.2fx | %v |\n",
			s.WriteVersion, s.WritePath, s.Name, s.Range, formatBytes(s.ResponseBytes), s.LatencyMsP50, s.LatencyMsP95, s.BackendFetchesPerSample, formatBytes(int64(s.BackendBytesPerSample)), s.BackendToResponseByteRatio, s.Verified)
	}
	b.WriteString("\n")
	for _, note := range report.Notes {
		b.WriteString("- " + note + "\n")
	}
	return os.WriteFile(filepath.Join(outDir, "large-range-results.md"), []byte(b.String()), 0o644)
}

func envSize(t *testing.T, name, fallback string) int64 {
	t.Helper()
	v := fallback
	if configured := os.Getenv(name); configured != "" {
		v = configured
	}
	size, err := ParseSize(v)
	if err != nil || size <= 0 {
		t.Fatalf("%s=%q is not a positive size", name, v)
	}
	return size
}

func envPositiveInt(t *testing.T, name string, fallback int) int {
	t.Helper()
	v := fallback
	if configured := os.Getenv(name); configured != "" {
		var err error
		_, err = fmt.Sscanf(configured, "%d", &v)
		if err != nil || v <= 0 {
			t.Fatalf("%s=%q is not a positive integer", name, configured)
		}
	}
	return v
}

func benchmarkCommit() string {
	if commit := os.Getenv("ARMOR_PERF_COMMIT"); commit != "" {
		return commit
	}
	return vcsRevision()
}

func formatBytes(n int64) string {
	const (
		kiB = int64(1 << 10)
		miB = int64(1 << 20)
		giB = int64(1 << 30)
	)
	switch {
	case n >= giB && n%giB == 0:
		return fmt.Sprintf("%d GiB", n/giB)
	case n >= miB && n%miB == 0:
		return fmt.Sprintf("%d MiB", n/miB)
	case n >= kiB && n%kiB == 0:
		return fmt.Sprintf("%d KiB", n/kiB)
	default:
		return fmt.Sprintf("%d B", n)
	}
}
