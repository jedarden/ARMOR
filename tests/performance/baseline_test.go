package performance

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

// TestRecordedBaseline runs the full local baseline matrix. It is deliberately
// inert in ordinary CI:
//
//	go test ./tests/performance -short          # skips (and CI runs -short)
//	go test ./tests/performance -run TestRecordedBaseline -v   # skips without ARMOR_PERF_RUN=1
//	ARMOR_PERF_RUN=1 go test ./tests/performance -run TestRecordedBaseline -v -timeout 60m
//
// Results land in $ARMOR_PERF_OUT (default <tmp>/armor-perf-results) as
// results.json and results.md. See docs/performance/README.md.
func TestRecordedBaseline(t *testing.T) {
	if testing.Short() {
		t.Skip("baseline is measurement work, not a -short test")
	}
	if os.Getenv("ARMOR_PERF_RUN") != "1" {
		t.Skip("set ARMOR_PERF_RUN=1 to run the measurement matrix")
	}

	cfg := DefaultMatrixConfig()
	if v := os.Getenv("ARMOR_PERF_SIZES"); v != "" {
		cfg.Sizes = strings.Split(v, ",")
	}
	if v := os.Getenv("ARMOR_PERF_READ_SAMPLES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.ReadSamples = n
		}
	}
	if v := os.Getenv("ARMOR_PERF_WRITE_SAMPLES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.WriteSamples = n
		}
	}
	if v := os.Getenv("ARMOR_PERF_PART"); v != "" {
		if n, err := ParseSize(v); err == nil {
			cfg.PartSize = n
		}
	}
	if v := os.Getenv("ARMOR_PERF_MULTIPART_CLIENTS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MultipartClients = n
		}
	}
	if v := os.Getenv("ARMOR_PERF_OUT"); v != "" {
		cfg.OutDir = v
	}
	if os.Getenv("ARMOR_PERF_NO_COMPRESS") == "1" {
		cfg.Compressed = false
	}

	report, err := RunBaseline(cfg)
	if err != nil {
		t.Fatalf("baseline: %v", err)
	}
	failed := 0
	for _, s := range report.Scenarios {
		if !s.Verified {
			failed++
			t.Errorf("scenario %s failed verification: %s", s.Name, s.VerifyNote)
		}
	}
	t.Logf("baseline complete: %d scenarios, %d verification failures\n%s",
		len(report.Scenarios), failed, mustReadFile(t, cfg.OutDir))
}

func mustReadFile(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(dir + "/results.md")
	if err != nil {
		return "(results.md unreadable: " + err.Error() + ")"
	}
	return string(b)
}
