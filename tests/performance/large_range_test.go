package performance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
	if objectSize < 40<<20 {
		t.Fatalf("large-object benchmark requires at least 40MiB, got %d bytes", objectSize)
	}

	env, err := NewLocalEnv("large-object-range-v3", false)
	if err != nil {
		t.Fatalf("boot local env: %v", err)
	}
	defer env.Close()

	body := SyntheticBody(int(objectSize))
	key := fmt.Sprintf("%slarge-range-%d", BenchPrefix, objectSize)
	if _, err := env.PutObject(key, body); err != nil {
		t.Fatalf("put %s: %v", key, err)
	}

	results := make([]largeRangeResult, 0, len(largeRangeSpecs(objectSize)))
	for _, spec := range largeRangeSpecs(objectSize) {
		result, err := measureLargeRange(env, key, body, spec, samples)
		if err != nil {
			t.Fatalf("measure %s: %v", spec.Name, err)
		}
		results = append(results, result)
		if !result.Verified {
			t.Errorf("range %s failed verification: %s", spec.Name, result.VerifyNote)
		}
	}

	report := largeRangeReport{
		GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
		ARMORCommit:  benchmarkCommit(),
		GoVersion:    runtime.Version(),
		Environment:  environment(),
		ObjectFormat: "v3 single-PUT envelope, 64 KiB blocks; AES-GCM",
		ObjectBytes:  objectSize,
		Samples:      samples,
		TestLocation: "tests/performance (in-process service over filesystem backend)",
		Scenarios:    results,
		Notes: []string{
			"the object is synthetic incompressible data and is written once through the real authenticated S3 path",
			"each range is warmed once before measurement; latency and backend bytes are warm-metadata samples",
			"backend bytes are the bytes requested from the instrumented backend by Get/GetRange, including envelope metadata and ciphertext/HMAC ranges",
			"local loopback and filesystem results are a service-shape baseline, not B2, Cloudflare, or production throughput evidence",
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
	TestLocation string             `json:"test_location"`
	Scenarios    []largeRangeResult `json:"scenarios"`
	Notes        []string           `json:"notes"`
}

func measureLargeRange(env *LocalEnv, key string, body []byte, spec largeRangeSpec, samples int) (largeRangeResult, error) {
	// Warm metadata before taking the counters so backend bytes measure the
	// range workload rather than the one-time object-header lookup.
	warm, err := env.GetRange(key, spec.Header, true)
	if err != nil {
		return largeRangeResult{}, fmt.Errorf("warmup: %w", err)
	}
	expected := body[spec.Start : spec.Start+spec.ResponseBytes]
	if !bytes.Equal(warm.Received, expected) {
		return largeRangeResult{}, fmt.Errorf("warmup payload mismatch: got %d bytes, want %d", len(warm.Received), len(expected))
	}

	baseCounts, baseBytes := env.Backend.Snapshot()
	latencies := make([]float64, 0, samples)
	verified := true
	verifyNote := "every sample byte-compared with the exact plaintext slice"
	for i := 0; i < samples; i++ {
		sample, err := env.GetRange(key, spec.Header, true)
		if err != nil {
			return largeRangeResult{}, fmt.Errorf("sample %d: %w", i+1, err)
		}
		if !bytes.Equal(sample.Received, expected) {
			verified = false
			verifyNote = fmt.Sprintf("sample %d did not match the exact plaintext slice", i+1)
		}
		latencies = append(latencies, fms(sample.Duration))
	}

	counts, fetchedBytes := env.Backend.Snapshot()
	backendFetches := (counts[opGet] - baseCounts[opGet]) + (counts[opGetRange] - baseCounts[opGetRange])
	backendBytes := (fetchedBytes[opGet] - baseBytes[opGet]) + (fetchedBytes[opGetRange] - baseBytes[opGetRange])
	return largeRangeResult{
		Name:                       spec.Name,
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
	fmt.Fprintf(&b, "- ARMOR commit: `%s`\n- Go: %s\n- Environment: %s\n- Object: %d bytes (%s)\n- Format: %s\n- Samples per row: %d\n- Location: %s\n\n",
		report.ARMORCommit, report.GoVersion, report.Environment, report.ObjectBytes, formatBytes(report.ObjectBytes), report.ObjectFormat, report.Samples, report.TestLocation)
	b.WriteString("All rows are warm-metadata samples. Backend bytes include every byte returned by the instrumented Get/GetRange calls; the ratio is backend bytes divided by response bytes across all samples.\n\n")
	b.WriteString("| range | response | latency p50 ms | latency p95 ms | backend fetches/sample | backend bytes/sample | backend/response | verified |\n")
	b.WriteString("|---|---:|---:|---:|---:|---:|---:|---|\n")
	for _, s := range report.Scenarios {
		fmt.Fprintf(&b, "| %s (`%s`) | %s | %.2f | %.2f | %.1f | %s | %.2fx | %v |\n",
			s.Name, s.Range, formatBytes(s.ResponseBytes), s.LatencyMsP50, s.LatencyMsP95, s.BackendFetchesPerSample, formatBytes(int64(s.BackendBytesPerSample)), s.BackendToResponseByteRatio, s.Verified)
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
