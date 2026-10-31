package server

// List-path CPU attribution fixture at iad-ci census scale (armor-4ff12061,
// chain 2 of the armor-45abc172 split).
//
// Production evidence (recorded on armor-45abc172, 2026-10-30): at
// ARMOR_LIST_HEAD_FANOUT=128 on 0.1.2004, list p50/p90 were 1704/2439 ms
// (target < 3000 ms: met) but container CPU averaged 0.7823 cores over the
// same 2 h windows (target < 0.3: failed), with backend HEAD volume up from
// 1.194M to 2.636M per 2 h. The CPU rise tracked the doubled HEAD volume,
// but no profile ever attributed where the cores actually went.
//
// This fixture measures both list-path states on the census-shaped store
// that manifest_startup_scale_test.go seeds, and produces the profile the
// attribution is read from:
//
//   - AFTER (index populated — what a successful startup load gives): full
//     ListObjectsV2 polls of the transcripts page through the real request
//     pipeline (SigV4 auth, ACL, metrics, access log), with the manifest
//     index resolving every size. Asserts zero backend HEADs, wall p50
//     within the 0.1.1993 baseline budget (~3 s), and a per-list CPU bound
//     far under the fanout-16 production level (0.72 core-s/list).
//
//   - BEFORE (empty index — the 0.1.2004 steady state after the startup
//     load timed out): a second server with no manifest subsystem, whose
//     pages resolve sizes through the bounded HEAD fan-out. The corpus is
//     PUT through the real S3 pipeline first, so every HEAD exercises the
//     production ARMOR-metadata parse branch (not the unencrypted
//     fallthrough), and pages walk fresh keys via continuation tokens so
//     the version-gated cache never covers the next page. Asserts exactly
//     one HEAD per key at fanout 128 and measures the marginal per-HEAD
//     CPU, which is what the 2.636M-HEAD volume was spending.
//
//   - The same walk on a third server at fanout 16, when
//     ARMOR_LIST_CPU_COMPARE_FANOUT=1: per-HEAD CPU at 16 vs 128 is the
//     in-harness half of the "does the default stay 128" decision — the
//     wall-time half is latency-bound and stays with the production
//     measurements on the parent bead.
//
// The transport term (TLS records, HTTP/2 framing, B2 signing over the
// wire) does not exist behind an in-process filesystem backend and cannot
// be measured here; it is attributed by difference against the production
// telemetry on the parent bead. Everything else — per-HEAD handler work,
// manifest/cache lookups, XML encode/decode, auth, logging, metrics, GC —
// runs in this process and lands in the profile.
//
// CPU profiles for both legs are written to ARMOR_LIST_CPU_PROFILE_DIR
// (default: a fresh os.MkdirTemp directory, logged) for offline
// `go tool pprof -top` attribution.
//
// Explicit opt-in behind ARMOR_LIST_CPU_SCALE_TEST=1: seeding ~200 MB of
// manifest objects plus tens of thousands of encrypted PUTs and then
// spending minutes of CPU is a measurement run, not a unit test. -short
// skips it too. Run with a generous -timeout (15m+).

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/logging"
)

const (
	// afterWallBudget is the 0.1.1993 list-p50 baseline (~3.0 s) the chain
	// must not regress past. In-harness wall excludes the backend transport
	// round trip entirely (filesystem backend), so this is a regression
	// guard on the code-side term, not a transport prediction.
	afterWallBudget = 3 * time.Second
	// afterHandlerCoreCPUBudgetMS bounds the CPU of one index-resolved
	// 1000-key list EXCLUDING the backend List call: 1000 index lookups,
	// XML encode, SigV4 auth, access log, metrics. The fanout-16 production
	// level is ~720 core-ms/list (0.199 cores, 1997 lists / 2 h) including
	// ~600 transport HEADs; this budget is an order of magnitude under it.
	afterHandlerCoreCPUBudgetMS = 50
	// afterFullPipelineCPUBudgetMS is a sanity guard on the whole request
	// including the filesystem backend's List — which loads a metadata JSON
	// per returned key, a term the B2 backend does not pay (its list
	// response carries metadata inline). Loose by design; the
	// production-relevant bound is the handler-core one above.
	afterFullPipelineCPUBudgetMS = 500
	// headworkPlaintextMS is the plaintext size of every BEFORE-leg corpus
	// object. Large enough that PUTs exercise the streaming path, small
	// enough that 20k of them seed quickly.
	headworkPlaintext = 4096
)

// procCPUMillis returns this process's cumulative CPU time (user+system)
// from /proc/self/stat, in milliseconds. Linux only — the fixture skips
// elsewhere. Clock ticks are USER_HZ=100 on every Linux this runs on.
func procCPUMillis(t *testing.T) float64 {
	t.Helper()
	raw, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		t.Fatalf("read /proc/self/stat: %v", err)
	}
	// The comm field is parenthesized and may contain spaces; everything
	// after the final ')' is plain fields. utime/stime are fields 14/15,
	// i.e. index 11/12 counting the state field as index 0.
	rest := raw[bytes.LastIndexByte(raw, ')')+2:]
	fields := strings.Fields(string(rest))
	if len(fields) < 13 {
		t.Fatalf("short /proc/self stat line: %q", raw)
	}
	utime, err1 := strconv.ParseFloat(fields[11], 64)
	stime, err2 := strconv.ParseFloat(fields[12], 64)
	if err1 != nil || err2 != nil {
		t.Fatalf("parse utime/stime from %q: %v / %v", raw, err1, err2)
	}
	return (utime + stime) * 1000 / 100
}

// cpuScaleEnvInt reads a positive integer knob with a default.
func cpuScaleEnvInt(t *testing.T, key string, def int) int {
	t.Helper()
	if v := os.Getenv(key); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil || parsed <= 0 {
			t.Fatalf("%s: want a positive integer, got %q", key, v)
		}
		return parsed
	}
	return def
}

// p50Duration returns the median of ds.
func p50Duration(ds []time.Duration) time.Duration {
	sorted := append([]time.Duration(nil), ds...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return sorted[len(sorted)/2]
}

// cpuProfileSession starts a CPU profile written to <dir>/<name>.pb.gz and
// returns the stop function, which finishes the profile and logs its path.
// Only one profile may run at a time; the legs run strictly in sequence.
func cpuProfileSession(t *testing.T, dir, name string) func() {
	t.Helper()
	path := filepath.Join(dir, name+".pb.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create profile %s: %v", path, err)
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		f.Close() //nolint:errcheck
		t.Fatalf("start CPU profile: %v", err)
	}
	return func() {
		pprof.StopCPUProfile()
		if err := f.Close(); err != nil {
			t.Fatalf("close profile %s: %v", path, err)
		}
		t.Logf("cpu profile: %s", path)
	}
}

// listPageV2 decodes the fields the fixture reads from a ListObjectsV2
// response: entries plus the truncation/token pair the BEFORE leg walks.
type listPageV2 struct {
	XMLName xml.Name `xml:"ListBucketResult"`
	Keys    []struct {
		Key  string `xml:"Key"`
		Size int64  `xml:"Size"`
	} `xml:"Contents"`
	IsTruncated           bool   `xml:"IsTruncated"`
	NextContinuationToken string `xml:"NextContinuationToken"`
}

// backendListCPUPerCallMS drives the raw backend List in a loop (same
// request shape the handler issues) and returns the CPU milliseconds per
// call. The filesystem backend's List walks the bucket directory and loads
// a metadata JSON per returned key — a backend-side term the full-pipeline
// measurement contains and the handler-core attribution must subtract,
// because the production B2 backend pays transport+XML for its list
// instead, a term no in-process backend can reproduce.
func backendListCPUPerCallMS(t *testing.T, be *scaleLatencyBackend, prefix, token string, calls int) float64 {
	t.Helper()
	for i := 0; i < 3; i++ { // warm the directory cache
		if _, err := be.List(context.Background(), "b", prefix, "", token, 1000); err != nil {
			t.Fatalf("warm backend List(%s): %v", prefix, err)
		}
	}
	cpu0 := procCPUMillis(t)
	for i := 0; i < calls; i++ {
		if _, err := be.List(context.Background(), "b", prefix, "", token, 1000); err != nil {
			t.Fatalf("backend List(%s): %v", prefix, err)
		}
	}
	return (procCPUMillis(t) - cpu0) / float64(calls)
}

// cpuScaleList issues one signed ListObjectsV2 request and returns the
// decoded page. Every response byte is drained so keep-alive reuse matches
// a real poller.
func cpuScaleList(t *testing.T, base, prefix, token string) listPageV2 {
	t.Helper()
	u := base + "/b/?list-type=2&prefix=" + url.QueryEscape(prefix) + "&max-keys=1000"
	if token != "" {
		u += "&continuation-token=" + url.QueryEscape(token)
	}
	resp := signedS3Request(t, http.MethodGet, u, nil)
	defer resp.Body.Close() //nolint:errcheck
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read list response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ListObjectsV2 %s = %d: %s", u, resp.StatusCode, body)
	}
	var page listPageV2
	if err := xml.Unmarshal(body, &page); err != nil {
		t.Fatalf("decode ListObjectsV2 response: %v", err)
	}
	return page
}

// seedHeadworkCorpus PUTs n ARMOR-encrypted objects under headwork/ through
// the real S3 pipeline of a loaded server, so the BEFORE leg's HEAD fan-out
// hits encrypted objects with parseable ARMOR metadata — the production
// shape — instead of the raw fixture bodies.
func seedHeadworkCorpus(t *testing.T, base string, n int) {
	t.Helper()
	body := bytes.Repeat([]byte("h"), headworkPlaintext)
	start := time.Now()
	for i := 0; i < n; i++ {
		key := fmt.Sprintf("headwork/obj-%06d.bin", i)
		resp := signedS3Request(t, http.MethodPut, base+"/b/"+key, body)
		io.Copy(io.Discard, resp.Body) //nolint:errcheck
		resp.Body.Close()              //nolint:errcheck
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("PUT %s = %d, want 200", key, resp.StatusCode)
		}
	}
	t.Logf("headwork corpus seeded: %d encrypted objects in %s", n, time.Since(start).Round(time.Millisecond))
}

// walkHeadworkPages drives the empty-index server page by page through the
// headwork corpus with continuation tokens, returning the pages walked, the
// total backend HEADs, per-page wall times, and the CPU spent. Every page
// must be full (1000 keys, sizes resolved from ARMOR metadata) and must pay
// exactly one HEAD per key — fresh keys per page keep the version-gated
// cache out of the way, which is the production steady state for a churning
// prefix (warp-ingest rewrites transcripts continuously).
func walkHeadworkPages(t *testing.T, srv *Server, be *scaleLatencyBackend, label string, maxPages int) (pages, heads int, walls []time.Duration, cpuMS float64) {
	t.Helper()
	listener := httptest.NewServer(srv.Handler())
	t.Cleanup(listener.Close)

	stop := cpuProfileSession(t, profileDir(), "list-cpu-"+label)
	cpu0 := procCPUMillis(t)
	token := ""
	for pages < maxPages {
		headsBefore := be.heads.Load()
		start := time.Now()
		page := cpuScaleList(t, listener.URL, "headwork/", token)
		walls = append(walls, time.Since(start))
		pageHeads := int(be.heads.Load() - headsBefore)

		if len(page.Keys) != scaleListedKeys {
			t.Fatalf("%s page %d: %d keys, want %d", label, pages, len(page.Keys), scaleListedKeys)
		}
		if pageHeads != scaleListedKeys {
			t.Fatalf("%s page %d: %d backend HEADs, want exactly %d (one per fresh key)", label, pages, pageHeads, scaleListedKeys)
		}
		for _, k := range page.Keys {
			if !strings.HasPrefix(k.Key, "headwork/") {
				t.Fatalf("%s page %d: unexpected key %q", label, pages, k.Key)
			}
			if k.Size != headworkPlaintext {
				t.Fatalf("%s page %d: key %s size = %d, want the ARMOR-metadata plaintext %d", label, pages, k.Key, k.Size, headworkPlaintext)
			}
		}
		heads += pageHeads
		pages++
		if !page.IsTruncated {
			break
		}
		token = page.NextContinuationToken
	}
	stop()
	cpuMS = procCPUMillis(t) - cpu0
	return pages, heads, walls, cpuMS
}

var cpuProfileDirOnce struct {
	dir string
	set bool
}

// profileDir resolves the shared profile output directory once per run:
// ARMOR_LIST_CPU_PROFILE_DIR, else a fresh temp directory that outlives the
// test (os.MkdirTemp, not t.TempDir) so pprof can be run on it afterwards.
func profileDir() string {
	if !cpuProfileDirOnce.set {
		dir := os.Getenv("ARMOR_LIST_CPU_PROFILE_DIR")
		if dir == "" {
			if made, err := os.MkdirTemp("", "armor-list-cpu-"); err == nil {
				dir = made
			}
		}
		cpuProfileDirOnce.dir, cpuProfileDirOnce.set = dir, true
	}
	return cpuProfileDirOnce.dir
}

// TestManifestListCPUAttributionIadCIScale is the measurement run described
// in the file comment: census-shaped store, index-populated poll loop,
// empty-index HEAD-fanout walk, optional fanout-16 comparison, CPU profiles
// for offline attribution.
func TestManifestListCPUAttributionIadCIScale(t *testing.T) {
	if os.Getenv("ARMOR_LIST_CPU_SCALE_TEST") != "1" {
		t.Skip("ARMOR_LIST_CPU_SCALE_TEST != 1: the list-CPU attribution fixture is an explicit opt-in measurement run; see this file's comment")
	}
	if testing.Short() {
		t.Skip("-short: list-CPU attribution fixture skipped")
	}
	if runtime.GOOS != "linux" {
		t.Skipf("/proc/self/stat CPU accounting is Linux-only, not %s", runtime.GOOS)
	}

	pagesWanted := cpuScaleEnvInt(t, "ARMOR_LIST_CPU_PAGES", 20)
	listsWanted := cpuScaleEnvInt(t, "ARMOR_LIST_CPU_LISTS", 200)
	compareFanout := os.Getenv("ARMOR_LIST_CPU_COMPARE_FANOUT") == "1"

	// 0 ms injected latency: this run prices CPU, not the transport round
	// trips the startup-load fixture measures.
	root := t.TempDir()
	fs, err := backend.NewFSBackend(backend.FSConfig{BasePath: root})
	if err != nil {
		t.Fatalf("NewFSBackend: %v", err)
	}
	manifestKeys, transcriptsPlaintext := scaleSeedFixture(t, fs)
	be := &scaleLatencyBackend{Backend: fs, manifestKeys: manifestKeys}

	pairs := []string{
		"ARMOR_BACKEND", "filesystem",
		"ARMOR_FS_PATH", root,
		"ARMOR_BUCKET", "b",
		"ARMOR_MEK", asyncLoadTestMEK,
		"ARMOR_AUTH_ACCESS_KEY", asyncLoadTestAccessKey,
		"ARMOR_AUTH_SECRET_KEY", asyncLoadTestSecretKey,
		"ARMOR_WRITER_ID", "armor-cpu-live",
		"ARMOR_CANARY_DISABLED", "true",
		"ARMOR_MANIFEST_LOAD_TIMEOUT", strconv.Itoa(int(scaleLoadBudget.Seconds())),
		"ARMOR_READ_RETRY_ATTEMPTS", "3",
		"ARMOR_READ_RETRY_BACKOFF", "1ms",
		"ARMOR_READ_RETRY_MAX_BACKOFF", "10ms",
	}
	originals := make(map[string]string, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		k, v := pairs[i], pairs[i+1]
		originals[k] = os.Getenv(k)
		if err := os.Setenv(k, v); err != nil {
			t.Fatalf("Setenv(%s): %v", k, err)
		}
	}
	t.Cleanup(func() {
		for k, v := range originals {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	})

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load(): %v", err)
	}

	// The AFTER server: production manifest start path, cold load from the
	// census-shaped store, index populated.
	srvIndex, err := NewWithBackend(cfg, be)
	if err != nil {
		t.Fatalf("NewWithBackend(): %v", err)
	}
	sys := newManifestSystem()
	sys.start(cfg, logging.New("test"), be)
	srvIndex.manifestSys = sys
	t.Cleanup(srvIndex.StopManifestWriter)

	loadStart := time.Now()
	if st := srvIndex.WaitForManifestLoad(scaleLoadBudget + 60*time.Second); st != manifestLoaded {
		t.Fatalf("manifest load settled as %q after %s, want %q", st, time.Since(loadStart).Round(time.Millisecond), manifestLoaded)
	}
	if got := srvIndex.manifestIndex().Len(); got < scaleListedKeys {
		t.Fatalf("loaded index holds %d entries, want at least the %d transcripts keys", got, scaleListedKeys)
	}
	t.Logf("setup: census cold load resolved loaded in %s at 0 ms injected latency", time.Since(loadStart).Round(time.Millisecond))

	indexListener := httptest.NewServer(srvIndex.Handler())
	t.Cleanup(indexListener.Close)

	// The BEFORE-leg corpus, PUT through the real pipeline while the index
	// is live (so its own entries exist too — the index server never lists
	// this prefix in its measured leg).
	seedHeadworkCorpus(t, indexListener.URL, pagesWanted*scaleListedKeys)

	// ---------------- AFTER leg: index populated ----------------
	// Warm caches and the FS directory tree before measuring.
	for i := 0; i < 10; i++ {
		page := cpuScaleList(t, indexListener.URL, "transcripts/", "")
		if len(page.Keys) != scaleListedKeys {
			t.Fatalf("warmup list returned %d keys, want %d", len(page.Keys), scaleListedKeys)
		}
	}
	runtime.GC()

	var walls []time.Duration
	cpu0 := procCPUMillis(t)
	heads0 := be.heads.Load()
	stop := cpuProfileSession(t, profileDir(), "list-cpu-after")
	for i := 0; i < listsWanted; i++ {
		start := time.Now()
		page := cpuScaleList(t, indexListener.URL, "transcripts/", "")
		walls = append(walls, time.Since(start))
		if len(page.Keys) != scaleListedKeys {
			t.Fatalf("list %d returned %d keys, want %d", i, len(page.Keys), scaleListedKeys)
		}
		for _, k := range page.Keys {
			if k.Size != transcriptsPlaintext {
				t.Fatalf("list %d: key %s size = %d, want the index-resolved plaintext %d", i, k.Key, k.Size, transcriptsPlaintext)
			}
		}
	}
	stop()
	afterCPU := procCPUMillis(t) - cpu0
	afterHeads := int(be.heads.Load() - heads0)

	if afterHeads != 0 {
		t.Fatalf("index-populated leg issued %d backend HEADs; the loaded index must cover every listed key", afterHeads)
	}
	afterP50 := p50Duration(walls)
	afterPerListMS := afterCPU / float64(listsWanted)

	// Split the measured per-list CPU: the backend List term (filesystem
	// walk + per-key metadata JSON) versus the handler core (auth, enrich
	// lookups, XML encode, logging, metrics) — the part whose production
	// counterpart is not backend-implementation-shaped.
	backendPerListMS := backendListCPUPerCallMS(t, be, "transcripts/", "", 30)
	handlerCorePerListMS := afterPerListMS - backendPerListMS
	t.Logf("AFTER (index populated): %d lists, %d HEADs total, wall p50 %s (budget %s), CPU %.1f ms/list full pipeline = %.1f backend List + %.1f handler core (budgets %d / %d core-ms)",
		listsWanted, afterHeads, afterP50.Round(time.Millisecond), afterWallBudget, afterPerListMS, backendPerListMS, handlerCorePerListMS, afterFullPipelineCPUBudgetMS, afterHandlerCoreCPUBudgetMS)
	if afterP50 > afterWallBudget {
		t.Fatalf("index-populated list wall p50 %s exceeds the %.0f s baseline budget", afterP50.Round(time.Millisecond), afterWallBudget.Seconds())
	}
	if afterPerListMS > afterFullPipelineCPUBudgetMS {
		t.Fatalf("index-populated list CPU %.1f ms/list exceeds the %d ms full-pipeline sanity budget", afterPerListMS, afterFullPipelineCPUBudgetMS)
	}
	if handlerCorePerListMS < 0 {
		t.Fatalf("handler-core attribution went negative (%.1f = %.1f - %.1f): the backend-only loop is not measuring the same List the handler issues", handlerCorePerListMS, afterPerListMS, backendPerListMS)
	}
	if handlerCorePerListMS > afterHandlerCoreCPUBudgetMS {
		t.Fatalf("index-populated handler-core list CPU %.1f ms/list exceeds the %d ms/list budget", handlerCorePerListMS, afterHandlerCoreCPUBudgetMS)
	}

	// ---------------- BEFORE leg: empty index, fanout 128 ----------------
	// A second server that never gets a manifest subsystem — the exact
	// steady state of a 0.1.2004 pod whose startup load timed out
	// ("continuing with empty manifest index"): every listed key misses
	// index and cache and pays a backend HEAD.
	srvEmpty128, err := NewWithBackend(cfg, be)
	if err != nil {
		t.Fatalf("NewWithBackend(empty-index): %v", err)
	}
	pages128, heads128, walls128, cpu128 := walkHeadworkPages(t, srvEmpty128, be, "before-fanout128", pagesWanted)
	beforeP50 := p50Duration(walls128)
	beforePerListMS := cpu128 / float64(pages128)
	// The per-HEAD marginal cost: page CPU minus the base the page would
	// have cost with the index populated — backend list of a headwork page
	// plus the handler core — divided by the 1000 HEADs each page paid.
	backendHeadworkPerPageMS := backendListCPUPerCallMS(t, be, "headwork/", "", 20)
	perHeadMS := (cpu128 - float64(pages128)*(backendHeadworkPerPageMS+handlerCorePerListMS)) / float64(heads128)
	t.Logf("BEFORE (empty index, fanout %d): %d pages, %d HEADs, wall p50 %s/page, CPU %.1f ms/page (base %.1f = backend %.1f + handler core %.1f); marginal per-HEAD CPU %.3f ms",
		cfg.ListHeadFanout, pages128, heads128, beforeP50.Round(time.Millisecond), beforePerListMS, backendHeadworkPerPageMS+handlerCorePerListMS, backendHeadworkPerPageMS, handlerCorePerListMS, perHeadMS)
	if pages128 != pagesWanted {
		t.Fatalf("walked %d pages, want %d (corpus truncated early?)", pages128, pagesWanted)
	}

	// ---------------- optional fanout-16 comparison ----------------
	if compareFanout {
		cfg16 := *cfg
		cfg16.ListHeadFanout = 16
		srvEmpty16, err := NewWithBackend(&cfg16, be)
		if err != nil {
			t.Fatalf("NewWithBackend(fanout-16): %v", err)
		}
		pages16, heads16, walls16, cpu16 := walkHeadworkPages(t, srvEmpty16, be, "before-fanout16", pagesWanted)
		p50Fanout16 := p50Duration(walls16)
		perHead16 := (cpu16 - float64(pages16)*(backendHeadworkPerPageMS+handlerCorePerListMS)) / float64(heads16)
		t.Logf("COMPARE (empty index, fanout 16): %d pages, %d HEADs, wall p50 %s/page, CPU %.1f ms/page, marginal per-HEAD CPU %.3f ms — per-HEAD CPU ratio 128/16 = %.2f",
			pages16, heads16, p50Fanout16.Round(time.Millisecond), cpu16/float64(pages16), perHead16, perHeadMS/perHead16)
		if pages16 != pagesWanted {
			t.Fatalf("fanout-16 walk covered %d pages, want %d", pages16, pagesWanted)
		}
	}

	// The transport term the harness cannot see, for the bead's
	// attribution arithmetic: production implied ~2.14 core-ms per HEAD
	// all-in (0.7823 cores over 2 h with 2.636M HEADs); the numbers above
	// are the in-process shares.
	t.Logf("attribution inputs: handler core %.1f core-ms/list, backend List %.1f core-ms/list (FS-shaped), per-HEAD handler %.3f core-ms, profiles in %s (run: go tool pprof -top <file>)",
		handlerCorePerListMS, backendPerListMS, perHeadMS, profileDir())
}
