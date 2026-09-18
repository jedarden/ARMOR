package performance

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

// MatrixConfig bounds a benchmark run. Every field has a bounded default; the
// ARMOR_PERF_* environment variables override them (see docs/performance).
type MatrixConfig struct {
	// Sizes is the object-size list, e.g. ["64KiB","1MiB","64MiB"]. 1 GiB is
	// opt-in only.
	Sizes []string
	// ReadSamples is the warm-read sample count per read scenario.
	ReadSamples int
	// WriteSamples is the sample count per write scenario.
	WriteSamples int
	// ClientConcs is the client-concurrency list for the multi-client read
	// scenario (values > 1 run; 1 is always covered by the base scenarios).
	ClientConcs []int
	// MultipartClients is the concurrency of the concurrent-multipart write.
	MultipartClients int
	// PartSize is the uniform multipart part size (>= 5 MiB per ADR-015).
	PartSize int64
	// OutDir receives results.json and results.md.
	OutDir string
	// Compressed adds the ADR-007 single-PUT compression scenarios
	// (compressible payloads; ranges and multipart unsupported there).
	Compressed bool
}

// DefaultMatrixConfig returns the bounded default matrix.
func DefaultMatrixConfig() MatrixConfig {
	return MatrixConfig{
		Sizes:            []string{"64KiB", "1MiB", "64MiB"},
		ReadSamples:      7,
		WriteSamples:     3,
		ClientConcs:      []int{4},
		MultipartClients: 4,
		PartSize:         8 << 20,
		OutDir:           filepath.Join(os.TempDir(), "armor-perf-results"),
		Compressed:       true,
	}
}

// ScenarioResult is one measured scenario.
type ScenarioResult struct {
	Name               string          `json:"name"`
	ObjectBytes        int64           `json:"object_bytes"`
	Samples            int             `json:"samples"`
	ClientConcurrency  int             `json:"client_concurrency"`
	AggregateMBps      float64         `json:"aggregate_mbps"`
	MBpsP50            float64         `json:"mbps_p50"`
	MBpsP95            float64         `json:"mbps_p95"`
	TTFBMsP50          float64         `json:"ttfb_ms_p50"`
	TTFBMsP95          float64         `json:"ttfb_ms_p95"`
	CompletionMsP50    float64         `json:"completion_ms_p50"`
	CompletionMsP95    float64         `json:"completion_ms_p95"`
	BackendOpsDelta    map[string]int64 `json:"backend_ops_delta"`
	BackendBytesDelta  map[string]int64 `json:"backend_bytes_delta"`
	BackendMaxInflight int64           `json:"backend_max_inflight"`
	CPUSeconds         float64         `json:"cpu_seconds"`
	AllocBytes         uint64          `json:"alloc_bytes"`
	Verified           bool            `json:"verified"`
	VerifyNote         string          `json:"verify_note,omitempty"`
}

// RunReport is the full output of one matrix run.
type RunReport struct {
	GeneratedAt        string           `json:"generated_at"`
	ARMORCommit        string           `json:"armor_commit"`
	GoVersion          string           `json:"go_version"`
	Environment        string           `json:"environment"`
	ObjectFormat       string           `json:"object_format"`
	CFStatus           string           `json:"cf_cache_status"`
	TestLocation       string           `json:"test_location"`
	Scenarios          []ScenarioResult `json:"scenarios"`
	PeakRSSKiB         int64            `json:"peak_rss_kib"`
	TempDiskPeakBytes  int64            `json:"temp_disk_peak_bytes"`
	TempDiskDir        string           `json:"temp_disk_dir"`
	Notes              []string         `json:"notes"`
}

// RunBaseline executes the bounded default (or configured) matrix against
// local in-process ARMOR services and writes results.json / results.md into
// cfg.OutDir. Every read is hash-verified; every acknowledged write is
// re-read through the ordinary service GET path (full object) before it
// counts as verified.
func RunBaseline(cfg MatrixConfig) (*RunReport, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cfg.OutDir, 0o755); err != nil {
		return nil, err
	}

	v3, err := NewLocalEnv("local-v3", false)
	if err != nil {
		return nil, err
	}
	defer v3.Close()
	if err := runMatrixAgainst(v3, cfg, false); err != nil {
		return nil, err
	}

	var compressed *LocalEnv
	if cfg.Compressed {
		compressed, err = NewLocalEnv("local-v3-compressed", true)
		if err != nil {
			return nil, err
		}
		defer compressed.Close()
		if err := runCompressedScenarios(compressed, cfg); err != nil {
			return nil, err
		}
	}

	commit := vcsRevision()
	if c := os.Getenv("ARMOR_PERF_COMMIT"); c != "" {
		commit = c // an export has no VCS metadata; the runner passes the commit it exported
	}
	report := &RunReport{
		GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
		ARMORCommit:  commit,
		GoVersion:    runtime.Version(),
		Environment:  environment(),
		ObjectFormat: "v3 envelope (single-PUT and multipart), 64 KiB blocks; AES-GCM",
		CFStatus:     "n/a (local loopback; no Cloudflare in path)",
		TestLocation: "tests/performance (in-process service over filesystem backend)",
		Scenarios:    append(append([]ScenarioResult{}, v3.results...), compressedResults(compressed)...),
		TempDiskDir:  v3.fsDir,
	}
	report.PeakRSSKiB = peakRSSKiB()
	report.TempDiskPeakBytes = v3.peakTempDisk
	if compressed != nil && compressed.peakTempDisk > report.TempDiskPeakBytes {
		report.TempDiskPeakBytes = compressed.peakTempDisk
	}
	report.Notes = []string{
		"client and server share one process; CPU/RSS/alloc figures cover both",
		"payloads are synthetic; uncompressed scenarios use incompressible (crypto-random) data, compressed scenarios use a repeating pattern",
		"cold = first GET of the key after its write; warm = subsequent GETs of the same key",
		"writes are verified by re-reading the object through the ordinary service GET (full object) and comparing the payload SHA-256",
		"p50/p95 over the recorded per-sample values; sample counts are per scenario",
	}

	j, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(cfg.OutDir, "results.json"), j, 0o644); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(cfg.OutDir, "results.md"), []byte(renderMarkdown(report)), 0o644); err != nil {
		return nil, err
	}
	return report, nil
}

func compressedResults(e *LocalEnv) []ScenarioResult {
	if e == nil {
		return nil
	}
	return e.results
}

func (c MatrixConfig) validate() error {
	if len(c.Sizes) == 0 {
		return fmt.Errorf("perf: no sizes configured")
	}
	for _, s := range c.Sizes {
		if _, err := ParseSize(s); err != nil {
			return err
		}
	}
	if c.PartSize < 5<<20 {
		return fmt.Errorf("perf: part size %d < 5 MiB ADR-015 minimum", c.PartSize)
	}
	return nil
}

// runMatrixAgainst runs the uncompressed v3 scenario matrix on env.
func runMatrixAgainst(env *LocalEnv, cfg MatrixConfig, _ bool) error {
	ctx := context.Background()
	for _, sizeStr := range cfg.Sizes {
		size := MustParseSize(sizeStr)
		body := SyntheticBody(int(size))
		sha := hashHex(body)

		// --- reads ---
		readKey := BenchPrefix + fmt.Sprintf("read-%s", strings.ToLower(sizeStr))
		if _, err := env.PutObject(readKey, body); err != nil {
			return err
		}
		// cold: first GET of this key.
		if err := measureRead(env, cfg, fmt.Sprintf("read-full-cold/v3/%s", strings.ToLower(sizeStr)),
			readKey, "", sha, size, 1, 1); err != nil {
			return err
		}
		// warm: repeated GETs.
		if err := measureRead(env, cfg, fmt.Sprintf("read-full-warm/v3/%s", strings.ToLower(sizeStr)),
			readKey, "", sha, size, cfg.ReadSamples, 1); err != nil {
			return err
		}
		// ranges: aligned / unaligned / suffix.
		block := int64(64 << 10)
		alignedOff, alignedLen := int64(0), block/2
		if size > 2*block {
			alignedOff, alignedLen = 2*block, block
		}
		unalignedOff := alignedOff + 1234
		if unalignedOff+1024 > size {
			unalignedOff = 0
		}
		unalignedLen := int64(32 << 10)
		if unalignedOff+unalignedLen > size {
			unalignedLen = size - unalignedOff
		}
		suffixLen := block
		if suffixLen > size {
			suffixLen = size
		}
		rangeSamples := cfg.ReadSamples
		if size >= 64<<20 {
			rangeSamples = 3 // bounded: big-object range scenarios need fewer samples
		}
		for _, sc := range []struct {
			name  string
			rng   string
			bytes int64
		}{
			{"read-range-aligned/v3/" + strings.ToLower(sizeStr), fmt.Sprintf("bytes=%d-%d", alignedOff, alignedOff+alignedLen-1), alignedLen},
			{"read-range-unaligned/v3/" + strings.ToLower(sizeStr), fmt.Sprintf("bytes=%d-%d", unalignedOff, unalignedOff+unalignedLen-1), unalignedLen},
			{"read-range-suffix/v3/" + strings.ToLower(sizeStr), fmt.Sprintf("bytes=-%d", suffixLen), suffixLen},
		} {
			if err := measureRange(env, cfg, sc.name, readKey, sc.rng, sc.bytes, rangeSamples); err != nil {
				return err
			}
		}

		// --- write: single-PUT, verified by service read-back ---
		if err := measureWrite(env, cfg, fmt.Sprintf("write-single-put/v3/%s", strings.ToLower(sizeStr)),
			size, cfg.WriteSamples, nil); err != nil {
			return err
		}

		// --- multi-client reads (each client its own key) ---
		for _, conc := range cfg.ClientConcs {
			if conc <= 1 {
				continue
			}
			keys := make([]string, conc)
			shas := make([]string, conc)
			for i := range keys {
				keys[i] = BenchPrefix + fmt.Sprintf("read-%s-c%d", strings.ToLower(sizeStr), i)
				if _, err := env.PutObject(keys[i], body); err != nil {
					return err
				}
				shas[i] = sha
			}
			if err := measureConcurrentReads(env, cfg,
				fmt.Sprintf("read-full-warm/v3/%s/clients=%d", strings.ToLower(sizeStr), conc),
				keys, shas, size, cfg.ReadSamples); err != nil {
				return err
			}
		}
	}

	// --- multipart writes at the largest configured size (bounded: one size) ---
	bigStr := cfg.Sizes[len(cfg.Sizes)-1]
	big := MustParseSize(bigStr)
	if big >= 8*cfg.PartSize {
		nParts := int(big / cfg.PartSize)

		// single-client multipart
		if err := measureMultipart(env, cfg, fmt.Sprintf("write-multipart/v3/%s", strings.ToLower(bigStr)),
			1, nParts, cfg.PartSize, cfg.WriteSamples); err != nil {
			return err
		}
		// concurrent-client multipart
		if err := measureMultipart(env, cfg,
			fmt.Sprintf("write-multipart-concurrent/v3/%s/clients=%d", strings.ToLower(bigStr), cfg.MultipartClients),
			cfg.MultipartClients, nParts, cfg.PartSize, 2); err != nil {
			return err
		}
	}

	_ = env.deletePrefixErr(ctx) // scope cleanup to the benchmark prefix
	return nil
}

// runCompressedScenarios covers the ADR-007 single-PUT compression path:
// compressible payloads, single-PUT only (ranges and multipart are
// unsupported on this path by design).
func runCompressedScenarios(env *LocalEnv, cfg MatrixConfig) error {
	for _, sizeStr := range cfg.Sizes {
		size := MustParseSize(sizeStr)
		body := SyntheticCompressible(int(size))
		sha := hashHex(body)
		key := BenchPrefix + "compressed-" + strings.ToLower(sizeStr)
		if _, err := env.PutObject(key, body); err != nil {
			return err
		}
		if err := measureRead(env, cfg, fmt.Sprintf("read-full-warm/compressed/%s", strings.ToLower(sizeStr)),
			key, "", sha, size, cfg.ReadSamples, 1); err != nil {
			return err
		}
		writes := cfg.WriteSamples
		if size >= 64<<20 {
			writes = 2
		}
		if err := measureWriteOnKey(env, cfg, fmt.Sprintf("write-single-put/compressed/%s", strings.ToLower(sizeStr)),
			key, body, sha, writes); err != nil {
			return err
		}
	}
	_ = env.deletePrefixErr(context.Background())
	return nil
}

// scenario measurement helpers -----------------------------------------------

type sysSnapshot struct {
	cpuSeconds float64
	allocBytes uint64
}

func readSys() sysSnapshot {
	var ru syscall.Rusage
	_ = syscall.Getrusage(syscall.RUSAGE_SELF, &ru)
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return sysSnapshot{
		cpuSeconds: ruSec(ru.Utime) + ruSec(ru.Stime),
		allocBytes: ms.TotalAlloc,
	}
}

func (e *LocalEnv) beginScenario(name string, objBytes int64, samples, conc int) (counts, bytes map[string]int64, sys sysSnapshot) {
	e.scenarioName = name
	e.scenarioObjectBytes = objBytes
	e.scenarioSamples = samples
	e.scenarioConc = conc
	counts, bytes = e.Backend.Snapshot()
	return counts, bytes, readSys()
}

func (e *LocalEnv) endScenario(baseCounts, baseBytes map[string]int64, baseSys sysSnapshot, verified bool, note string) ScenarioResult {
	counts, bytes := e.Backend.Snapshot()
	sys := readSys()
	// track peak temporary-disk usage BEFORE the run-end cleanup can shrink it
	if ds := dirSize(e.fsDir); ds > e.peakTempDisk {
		e.peakTempDisk = ds
	}
	delta := func(base, now map[string]int64) map[string]int64 {
		d := map[string]int64{}
		for k, v := range now {
			d[k] = v - base[k]
		}
		return d
	}
	r := ScenarioResult{
		Name:               e.scenarioName,
		ObjectBytes:        e.scenarioObjectBytes,
		Samples:            e.scenarioSamples,
		ClientConcurrency:  e.scenarioConc,
		BackendOpsDelta:    delta(baseCounts, counts),
		BackendBytesDelta:  delta(baseBytes, bytes),
		BackendMaxInflight: e.Backend.MaxInflight(),
		CPUSeconds:         sys.cpuSeconds - baseSys.cpuSeconds,
		AllocBytes:         sys.allocBytes - baseSys.allocBytes,
		Verified:           verified,
		VerifyNote:         note,
	}
	if !verified {
		r.VerifyNote = note + " — VERIFICATION FAILED"
	}
	return r
}

// measureRead runs n sequential full GETs (or one ranged shape) and records
// percentiles. Every sample is hash-verified. A read error (e.g. the known
// compressed-path defect) is recorded in the result and ends that scenario —
// it does not abort the run.
func (e *LocalEnv) measureReadGeneric(name, key, rangeHeader, wantSHA string, objBytes int64, n int) error {
	counts, bytes, sys := e.beginScenario(name, objBytes, n, 1)
	var (
		ttfs, durs, mbps []float64
		verified         = true
		runErr           error
		received         int64
	)
	for i := 0; i < n; i++ {
		s, err := e.getWithRange(key, rangeHeader, false)
		if err != nil {
			verified = false
			runErr = err
			break
		}
		received += s.Bytes
		if s.SHA256 != wantSHA {
			verified = false
		}
		ttfs = append(ttfs, fms(s.TTFB))
		durs = append(durs, fms(s.Duration))
		mbps = append(mbps, mBps(s.Bytes, s.Duration))
	}
	note := "every sample SHA-256 verified against the written payload"
	if runErr != nil {
		note += "; read error: " + runErr.Error()
	}
	e.scenarioSamples = len(durs)
	r := e.endScenario(counts, bytes, sys, verified, note)
	r.TTFBMsP50, r.TTFBMsP95 = percentile(ttfs, 50), percentile(ttfs, 95)
	r.CompletionMsP50, r.CompletionMsP95 = percentile(durs, 50), percentile(durs, 95)
	r.MBpsP50, r.MBpsP95 = percentile(mbps, 50), percentile(mbps, 95)
	r.AggregateMBps = mBps(received, sumDur(durs))
	e.results = append(e.results, r)
	return nil
}

func measureRead(env *LocalEnv, cfg MatrixConfig, name, key, _ string, wantSHA string, objBytes int64, n, conc int) error {
	return env.measureReadGeneric(name, key, "", wantSHA, objBytes, n)
}

func measureRange(env *LocalEnv, cfg MatrixConfig, name, key, rng string, rangeBytes int64, n int) error {
	// For ranges the expected payload hash is the hash of the plaintext slice;
	// the service decrypts so the client receives PLAINTEXT bytes. Rather than
	// tracking which slice, verify by size and by stable cross-sample equality.
	first := ""
	verified := true
	var runErr error
	counts, bytes, sys := env.beginScenario(name, rangeBytes, n, 1)
	var ttfs, durs, mbps []float64
	for i := 0; i < n; i++ {
		s, err := env.getWithRange(key, rng, false)
		if err != nil {
			verified = false
			runErr = err
			break
		}
		if s.Bytes != rangeBytes {
			verified = false
		}
		if i == 0 {
			first = s.SHA256
		} else if s.SHA256 != first {
			verified = false
		}
		ttfs = append(ttfs, fms(s.TTFB))
		durs = append(durs, fms(s.Duration))
		mbps = append(mbps, mBps(s.Bytes, s.Duration))
	}
	note := "payload size == requested range; identical SHA-256 across samples"
	if runErr != nil {
		note += "; read error: " + runErr.Error()
	}
	env.scenarioSamples = len(durs)
	r := env.endScenario(counts, bytes, sys, verified, note)
	r.TTFBMsP50, r.TTFBMsP95 = percentile(ttfs, 50), percentile(ttfs, 95)
	r.CompletionMsP50, r.CompletionMsP95 = percentile(durs, 50), percentile(durs, 95)
	r.MBpsP50, r.MBpsP95 = percentile(mbps, 50), percentile(mbps, 95)
	r.AggregateMBps = mBps(rangeBytes*int64(n), sumDur(durs))
	env.results = append(env.results, r)
	return nil
}

// measureWrite runs n single-PUT writes (fresh key each), each verified by a
// full service GET read-back.
func measureWrite(env *LocalEnv, cfg MatrixConfig, name string, size int64, n int, _ []byte) error {
	body := SyntheticBody(int(size))
	return measureWriteOnKey(env, cfg, name, BenchPrefix+"write-"+fmt.Sprintf("%x", time.Now().UnixNano()), body, hashHex(body), n)
}

func measureWriteOnKey(env *LocalEnv, cfg MatrixConfig, name, key string, body []byte, sha string, n int) error {
	size := int64(len(body))
	counts, bytes, sys := env.beginScenario(name, size, n, 1)
	var durs, mbps []float64
	verified := true
	var verifyErr error
	written := 0
	for i := 0; i < n; i++ {
		ikey := fmt.Sprintf("%s-%d", key, i)
		pr, err := env.PutObject(ikey, body)
		if err != nil {
			verified = false
			verifyErr = err
			break
		}
		written++
		// Verify through the ordinary service GET path.
		s, err := env.GetObject(ikey, false)
		if err != nil {
			// the write was acknowledged but could not be read back — record
			// it and keep measuring (this is the compressed-path failure mode
			// on committed main)
			verified = false
			verifyErr = err
			continue
		}
		if s.SHA256 != sha || s.Bytes != size {
			verified = false
		}
		durs = append(durs, fms(pr.Duration))
		mbps = append(mbps, mBps(pr.Bytes, pr.Duration))
	}
	note := "acknowledged write re-read via full service GET; payload SHA-256 compared"
	if verifyErr != nil {
		note += "; verify error: " + verifyErr.Error()
	}
	env.scenarioSamples = written
	r := env.endScenario(counts, bytes, sys, verified, note)
	r.CompletionMsP50, r.CompletionMsP95 = percentile(durs, 50), percentile(durs, 95)
	r.MBpsP50, r.MBpsP95 = percentile(mbps, 50), percentile(mbps, 95)
	r.AggregateMBps = mBps(size*int64(n), sumDur(durs))
	env.results = append(env.results, r)
	return nil
}

// measureConcurrentReads runs len(keys) goroutines, each doing n warm full
// GETs of its own key; aggregate throughput is the headline number and
// per-sample percentiles cover every individual GET.
func measureConcurrentReads(env *LocalEnv, cfg MatrixConfig, name string, keys, shas []string, objBytes int64, n int) error {
	conc := len(keys)
	counts, bytes, sys := env.beginScenario(name, objBytes, n*conc, conc)
	var mu sync.Mutex
	var totalBytes int64
	var verified = true
	var firstErr error
	var ttfs, durs, mbps []float64
	var wg sync.WaitGroup
	start := time.Now()
	for gi, key := range keys {
		wg.Add(1)
		go func(key, sha string) {
			defer wg.Done()
			for i := 0; i < n; i++ {
				s, err := env.GetObject(key, false)
				mu.Lock()
				if err != nil {
					verified = false
					if firstErr == nil {
						firstErr = err
					}
					mu.Unlock()
					return
				}
				if s.SHA256 != sha {
					verified = false
				}
				totalBytes += s.Bytes
				ttfs = append(ttfs, fms(s.TTFB))
				durs = append(durs, fms(s.Duration))
				mbps = append(mbps, mBps(s.Bytes, s.Duration))
				mu.Unlock()
			}
		}(key, shas[gi])
	}
	wg.Wait()
	wall := time.Since(start)
	note := "per-client SHA-256 verified; aggregate over wall clock"
	if firstErr != nil {
		note += "; read error: " + firstErr.Error()
	}
	env.scenarioSamples = len(durs)
	r := env.endScenario(counts, bytes, sys, verified, note)
	r.AggregateMBps = mBps(totalBytes, wall)
	r.TTFBMsP50, r.TTFBMsP95 = percentile(ttfs, 50), percentile(ttfs, 95)
	r.CompletionMsP50, r.CompletionMsP95 = percentile(durs, 50), percentile(durs, 95)
	r.MBpsP50, r.MBpsP95 = percentile(mbps, 50), percentile(mbps, 95)
	env.results = append(env.results, r)
	return nil
}

// measureMultipart runs samples multipart uploads at the given client
// concurrency, each verified by full service GET read-backs of every client's
// object.
func measureMultipart(env *LocalEnv, cfg MatrixConfig, name string, clients, nParts int, partSize int64, samples int) error {
	size := partSize * int64(nParts)
	counts, bytes, sys := env.beginScenario(name, size, samples, clients)
	var durs, mbps []float64
	verified := true
	for s := 0; s < samples; s++ {
		var wg sync.WaitGroup
		var mu sync.Mutex
		start := time.Now()
		for c := 0; c < clients; c++ {
			wg.Add(1)
			go func(c, s int) {
				defer wg.Done()
				key := fmt.Sprintf("%smultipart-%d-c%d-s%d", BenchPrefix, s, c, time.Now().UnixNano()%100000)
				// identical part bodies across clients -> same expected whole hash
				parts := make([][]byte, nParts)
				parts[0] = SyntheticBody(int(partSize))
				for i := 1; i < nParts; i++ {
					parts[i] = parts[0]
				}
				pr, err := env.MultipartPut(key, parts)
				if err != nil {
					mu.Lock()
					verified = false
					mu.Unlock()
					return
				}
				got, err := env.GetObject(key, false)
				if err != nil || got.SHA256 != pr.PlaintextSHA || got.Bytes != size {
					mu.Lock()
					verified = false
					mu.Unlock()
					return
				}
				mu.Lock()
				durs = append(durs, fms(pr.Duration))
				mbps = append(mbps, mBps(pr.Bytes, pr.Duration))
				mu.Unlock()
			}(c, s)
		}
		wg.Wait()
		wall := time.Since(start)
		durs = append(durs, fms(wall))
		mbps = append(mbps, mBps(size*int64(clients), wall))
	}
	r := env.endScenario(counts, bytes, sys, verified, "every client's object re-read via full service GET; SHA-256 compared")
	r.CompletionMsP50, r.CompletionMsP95 = percentile(durs, 50), percentile(durs, 95)
	r.MBpsP50, r.MBpsP95 = percentile(mbps, 50), percentile(mbps, 95)
	r.AggregateMBps = mBps(size*int64(clients)*int64(samples), sumDur(durs))
	env.results = append(env.results, r)
	return nil
}

// small helpers --------------------------------------------------------------

func fms(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }
func mBps(bytes int64, d time.Duration) float64 {
	if d <= 0 {
		return 0
	}
	return float64(bytes) / (1024 * 1024) / d.Seconds()
}
func sumDur(dursMs []float64) time.Duration {
	var t float64
	for _, d := range dursMs {
		t += d
	}
	return time.Duration(t * float64(time.Millisecond))
}

// percentile returns the p-th percentile (no interpolation; rank = ceil(p*n)).
func percentile(vals []float64, p float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	s := append([]float64(nil), vals...)
	sort.Float64s(s)
	idx := int(math.Ceil(p/100*float64(len(s)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(s) {
		idx = len(s) - 1
	}
	return s[idx]
}

func ruSec(tv syscall.Timeval) float64 {
	return float64(tv.Sec) + float64(tv.Usec)/1e6
}

func peakRSSKiB() int64 {
	b, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "VmHWM:") {
			var kiB int64
			if _, err := fmt.Sscanf(line, "VmHWM: %d kB", &kiB); err == nil {
				return kiB
			}
		}
	}
	return 0
}

func dirSize(root string) int64 {
	var total int64
	_ = filepath.WalkDir(root, func(_ string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

func vcsRevision() string {
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			if s.Key == "vcs.revision" {
				return s.Value
			}
		}
	}
	return "unknown (build info without VCS settings)"
}

func environment() string {
	host, _ := os.Hostname()
	return fmt.Sprintf("%s %s/%s GOMAXPROCS=%d host=%s", runtime.GOOS, runtime.GOARCH, runtime.GOOS, runtime.GOMAXPROCS(0), host)
}

// renderMarkdown renders the report as a paste-able markdown table block.
func renderMarkdown(r *RunReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# ARMOR performance baseline — %s\n\n", r.GeneratedAt)
	fmt.Fprintf(&b, "- ARMOR commit: `%s`\n- Go: %s\n- Environment: %s\n- Object format: %s\n- CF-Cache-Status: %s\n- Test location: %s\n- Peak RSS: %d MiB\n- Temp-disk peak: %d MiB (%s)\n\n",
		r.ARMORCommit, r.GoVersion, r.Environment, r.ObjectFormat, r.CFStatus, r.TestLocation, r.PeakRSSKiB/1024, r.TempDiskPeakBytes/(1024*1024), r.TempDiskDir)
	b.WriteString("| scenario | object B | samples | clients | agg MB/s | MB/s p50 | MB/s p95 | TTFB p50 ms | completion p50 ms | completion p95 ms | backend GET/range | backend PUT/parts | max inflight | verified |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|---|---|---|---|---|\n")
	for _, s := range r.Scenarios {
		reads := s.BackendOpsDelta[opGet] + s.BackendOpsDelta[opGetRange]
		puts := s.BackendOpsDelta[opPut] + s.BackendOpsDelta[opUploadPart]
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %.1f | %.1f | %.1f | %.2f | %.2f | %.2f | %d | %d | %d | %v |\n",
			s.Name, s.ObjectBytes, s.Samples, s.ClientConcurrency, s.AggregateMBps, s.MBpsP50, s.MBpsP95,
			s.TTFBMsP50, s.CompletionMsP50, s.CompletionMsP95, reads, puts, s.BackendMaxInflight, s.Verified)
	}
	b.WriteString("\n")
	for _, n := range r.Notes {
		b.WriteString("- " + n + "\n")
	}
	return b.String()
}
