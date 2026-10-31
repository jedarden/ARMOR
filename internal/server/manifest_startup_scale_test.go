package server

// Production-scale startup-load fixture (armor-29c23d26).
//
// Every iad-ci armor pod start on 0.1.2004 spent ~8 min in the manifest
// startup load and then logged "manifest startup load failed — continuing
// with an empty manifest index", and an empty index forces one real backend
// HEAD per listed key — ~1000 per ListObjectsV2 page, which is the CPU term
// the fleet split is chasing. The census taken against the iad-ci bucket
// (2026-10-31, recorded on armor-45abc172) pins the scale to reproduce:
//
//	8244 manifest objects across 71 writer shards, 205.0 MB total,
//	9 discovery list pages at 1000 keys/page,
//	top shards 1034 / 935 / 930 / 764 objects, live shard 21.
//
// 0.1.2004 fetched every snapshot and delta body sequentially, so the 480 s
// ARMOR_MANIFEST_LOAD_TIMEOUT expired mid-load. HEAD carries the parallel
// cold-load fetch (armor-f3c27b2b, released in 0.1.2006): discovery still
// lists page by page, but all shards load concurrently behind a shared bound
// of manifestLoadConcurrency in-flight fetches, with ordered replay per
// shard. Whether that actually fits the startup budget at the census scale
// had never been observed — production never ran the parallel loader — so
// this fixture is the non-production verification the roll was waiting on.
//
// It seeds the census shape into a filesystem store behind a backend that
// adds a configurable round-trip latency to every manifest-prefix fetch (the
// B2/Cloudflare term that made the sequential loader miss the budget), runs
// the production cold-load start path, and asserts both acceptance legs:
//
//  1. the load resolves "loaded" inside the 480 s budget, with wall time,
//     fetch count, and effective throughput logged; and
//  2. a 1000-key ListObjectsV2 page afterwards resolves every plaintext size
//     from the index — zero backend HEADs, where the empty-index fallback
//     costs one HEAD per key (TestListEnrichCapsConcurrentHeads pins that
//     counterfactual at small scale).
//
// Explicit opt-in behind ARMOR_MANIFEST_SCALE_TEST=1: materializing ~200 MB
// of manifest objects and spending real seconds under injected latency is a
// measurement run, not a unit test. -short skips it too. The per-fetch
// latency defaults to 150 ms and can be tuned with
// ARMOR_MANIFEST_SCALE_LATENCY_MS (e.g. 350 ms for a pessimistic B2 pass).

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/logging"
	"github.com/jedarden/armor/internal/manifest"
)

// Census-derived fixture shape (see the file comment for provenance).
const (
	scaleShards       = 71
	scaleTotalObjects = 8244 // includes one snapshot per shard
	// scaleTargetBytes is the ~205 MB census total the delta bodies sum to.
	scaleTargetBytes = 205 * 1024 * 1024
	// scaleListedKeys is the transcripts page the list leg resolves from
	// the index — one full S3 page, the exact shape iad-ci serves.
	scaleListedKeys = 1000
	// scaleListLatency is the default injected round trip per manifest
	// fetch, in milliseconds.
	scaleListLatencyMS = 150
	// scaleLoadBudget is the ARMOR_MANIFEST_LOAD_TIMEOUT the fixture runs
	// with — the production default and the budget the sequential loader
	// missed.
	scaleLoadBudget = 480 * time.Second
)

// scaleShardPlan returns the per-shard object count (snapshot included) for
// all 71 shards, top-heavy like the census: the four largest shards at
// 1034/935/930/764, a mid band, and a long tail, summing to
// scaleTotalObjects.
func scaleShardPlan() []int {
	plan := make([]int, scaleShards)
	plan[0], plan[1], plan[2], plan[3] = 1034, 935, 930, 764
	for i := 4; i <= 10; i++ {
		plan[i] = 320
	}
	remaining := scaleTotalObjects - 3663 - 7*320
	per := remaining / (scaleShards - 11)
	for i := 11; i < scaleShards; i++ {
		plan[i] = per
	}
	plan[11] += scaleTotalObjects - (3663 + 7*320 + per*(scaleShards-11))
	return plan
}

// scaleLatencyBackend wraps a backend and adds a fixed round-trip latency to
// every manifest-store fetch (Get/GetDirect of a manifest-prefix key, and
// ListRaw of the manifest prefix), emulating the B2/Cloudflare term the
// production load pays per object. Head is never delayed — the list leg
// asserts it is never called at all, and a latency there would only mask a
// regression — but every call is counted.
type scaleLatencyBackend struct {
	backend.Backend // nil: any method this fixture does not stub panics loudly

	latency time.Duration
	// manifestKeys is the set of object keys that belong to the manifest
	// store ("<bucket>/.armor/manifest/..."), priced at the injected
	// latency; everything else passes through untouched.
	manifestKeys map[string]bool

	fetches atomic.Int64
	heads   atomic.Int64
}

func (b *scaleLatencyBackend) delay(ctx context.Context) {
	select {
	case <-time.After(b.latency):
	case <-ctx.Done():
	}
}

func (b *scaleLatencyBackend) Get(ctx context.Context, bucket, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	if b.manifestKeys[bucket+"/"+key] {
		b.fetches.Add(1)
		b.delay(ctx)
	}
	return b.Backend.Get(ctx, bucket, key)
}

func (b *scaleLatencyBackend) GetDirect(ctx context.Context, bucket, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	if b.manifestKeys[bucket+"/"+key] {
		b.fetches.Add(1)
		b.delay(ctx)
	}
	return b.Backend.GetDirect(ctx, bucket, key)
}

func (b *scaleLatencyBackend) Head(ctx context.Context, bucket, key string) (*backend.ObjectInfo, error) {
	b.heads.Add(1)
	return b.Backend.Head(ctx, bucket, key)
}

// scaleSeedFixture materializes the census-shaped manifest store into be:
// per shard one snapshot plus its deltas, delta bodies sized so the store
// sums to ~scaleTargetBytes, plus scaleListedKeys transcripts objects whose
// index entries every shard churns. It returns the manifest key set for the
// latency backend and the plaintext size the transcripts entries carry.
func scaleSeedFixture(t *testing.T, be backend.Backend) (manifestKeys map[string]bool, transcriptsPlaintext int64) {
	t.Helper()
	seedStart := time.Now()
	manifestKeys = make(map[string]bool)
	transcriptsPlaintext = 4096

	// Size one representative op pair, then pack that many pairs per delta
	// so the store lands near the census total.
	base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	probe, err := manifest.MarshalDelta(scaleOpsForProbe(base))
	if err != nil {
		t.Fatalf("MarshalDelta probe: %v", err)
	}
	// Pack twice the per-object average into each delta: op bodies vary
	// with their entries, so the probe underestimates and the census total
	// is a target, not an exact bound.
	opsPerDelta := 2 * scaleTargetBytes / scaleTotalObjects / int64(len(probe))

	plan := scaleShardPlan()
	var objects, opCount int64
	var totalBytes int64
	for shard, shardObjects := range plan {
		writerID := fmt.Sprintf("armor-scale-%02d", shard)
		deltas := shardObjects - 1 // one slot is the snapshot

		// The snapshot carries the shard's earliest state so replay applies
		// it first and the deltas on top, exactly like a compacted shard.
		snapIdx := manifest.New()
		for i := 1; i <= int(opsPerDelta)*8 && deltas > 0; i++ {
			key := fmt.Sprintf("data/fil/%02d-snap-%06d.bin", shard, i)
			snapIdx.Put("b", key, &manifest.Entry{
				PlaintextSize: int64(1000 + i),
				BlockSize:     65536,
				ETag:          fmt.Sprintf("snap-etag-%02d-%06d", shard, i),
				LastModified:  base,
			})
		}
		snap, err := snapIdx.MarshalSnapshot()
		if err != nil {
			t.Fatalf("MarshalSnapshot(shard %d): %v", shard, err)
		}
		snapKey := manifest.SnapshotKey(".armor/manifest", writerID)
		if err := be.Put(context.Background(), "b", snapKey, bytes.NewReader(snap), int64(len(snap)), nil); err != nil {
			t.Fatalf("seed snapshot %s: %v", snapKey, err)
		}
		manifestKeys["b/"+snapKey] = true
		objects++
		totalBytes += int64(len(snap))

		for seq := 1; seq <= deltas; seq++ {
			ops := make([]manifest.Op, 0, opsPerDelta+1)
			for i := int64(0); i < opsPerDelta; i++ {
				n := opCount + i
				ops = append(ops, manifest.Op{
					Operation: "put",
					Key:       fmt.Sprintf("b/data/fil/%02d-%05d-%05d.bin", shard, seq, i),
					Entry: &manifest.Entry{
						PlaintextSize: 1024 + n%4096,
						BlockSize:     65536,
						ETag:          fmt.Sprintf("etag-%02d-%05d-%05d", shard, seq, i),
						LastModified:  base.Add(time.Duration(shard)*time.Hour + time.Duration(seq)*time.Minute),
					},
					Ts: base.Add(time.Duration(shard)*time.Hour + time.Duration(seq)*time.Minute),
				})
			}
			// Every shard churns the transcripts page: cross-writer
			// last-write-wins on the keys the list leg will resolve.
			// Op keys are "bucket/object-key" — the index map key the
			// handler's Lookup(bucket, clientKey) reads.
			ops = append(ops, manifest.Op{
				Operation: "put",
				Key:       fmt.Sprintf("b/transcripts/tick-%05d.json", seq%scaleListedKeys),
				Entry: &manifest.Entry{
					PlaintextSize: transcriptsPlaintext,
					BlockSize:     65536,
					ETag:          fmt.Sprintf("etag-transcript-%05d", seq),
					LastModified:  base.Add(time.Duration(shard)*time.Hour + time.Duration(seq)*time.Minute + time.Second),
				},
				Ts: base.Add(time.Duration(shard)*time.Hour + time.Duration(seq)*time.Minute + time.Second),
			})
			data, err := manifest.MarshalDelta(ops)
			if err != nil {
				t.Fatalf("MarshalDelta(shard %d seq %d): %v", shard, seq, err)
			}
			key := manifest.DeltaKey(".armor/manifest", writerID, uint64(seq))
			if err := be.Put(context.Background(), "b", key, bytes.NewReader(data), int64(len(data)), nil); err != nil {
				t.Fatalf("seed delta %s: %v", key, err)
			}
			manifestKeys["b/"+key] = true
			objects++
			opCount += int64(len(ops))
			totalBytes += int64(len(data))
		}
	}

	// The objects the transcripts page lists. Raw bytes are enough: the
	// index resolves their sizes and nothing ever HEADs or GETs them.
	for i := 0; i < scaleListedKeys; i++ {
		key := fmt.Sprintf("transcripts/tick-%05d.json", i)
		body := []byte("scale-fixture-transcript-body")
		if err := be.Put(context.Background(), "b", key, bytes.NewReader(body), int64(len(body)), nil); err != nil {
			t.Fatalf("seed transcript %s: %v", key, err)
		}
	}

	t.Logf("fixture seeded in %s: %d manifest objects across %d shards, %.1f MB, %d ops",
		time.Since(seedStart).Round(time.Millisecond), objects, len(plan), float64(totalBytes)/(1024*1024), opCount)
	if objects != scaleTotalObjects {
		t.Fatalf("fixture object count = %d, want %d (shard plan drifted)", objects, scaleTotalObjects)
	}
	return manifestKeys, transcriptsPlaintext
}

// scaleOpsForProbe builds the two-op pair whose marshaled size fixes the
// ops-per-delta packing: one filler op and one transcripts op.
func scaleOpsForProbe(base time.Time) []manifest.Op {
	return []manifest.Op{
		{
			Operation: "put",
			Key:       "b/data/fil/00-00000-00000.bin",
			Entry: &manifest.Entry{
				PlaintextSize: 1024,
				BlockSize:     65536,
				ETag:          "etag-00-00000-00000",
				LastModified:  base,
			},
			Ts: base,
		},
		{
			Operation: "put",
			Key:       "b/transcripts/tick-00000.json",
			Entry: &manifest.Entry{
				PlaintextSize: 4096,
				BlockSize:     65536,
				ETag:          "etag-transcript-00000",
				LastModified:  base,
			},
			Ts: base,
		},
	}
}

// listContentsV2 is the decoded Contents of a ListObjectsV2 response.
type listContentsV2 struct {
	XMLName xml.Name         `xml:"ListBucketResult"`
	Keys    []listEntryV2Key `xml:"Contents"`
}

type listEntryV2Key struct {
	Key  string `xml:"Key"`
	Size int64  `xml:"Size"`
}

// TestManifestStartupLoadIadCIScale is the measurement run described in the
// file comment: census-shaped store, injected per-fetch latency, production
// cold-load path, and the two acceptance assertions (budget + zero-HEAD
// list page).
func TestManifestStartupLoadIadCIScale(t *testing.T) {
	if os.Getenv("ARMOR_MANIFEST_SCALE_TEST") != "1" {
		t.Skip("ARMOR_MANIFEST_SCALE_TEST != 1: the iad-ci-scale startup-load fixture is an explicit opt-in measurement run; see this file's comment")
	}
	if testing.Short() {
		t.Skip("-short: iad-ci-scale startup-load fixture skipped")
	}

	latencyMS := scaleListLatencyMS
	if v := os.Getenv("ARMOR_MANIFEST_SCALE_LATENCY_MS"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil || parsed < 0 {
			t.Fatalf("ARMOR_MANIFEST_SCALE_LATENCY_MS: want a non-negative integer, got %q", v)
		}
		latencyMS = parsed
	}

	root := t.TempDir()
	fs, err := backend.NewFSBackend(backend.FSConfig{BasePath: root})
	if err != nil {
		t.Fatalf("NewFSBackend: %v", err)
	}
	manifestKeys, transcriptsPlaintext := scaleSeedFixture(t, fs)
	be := &scaleLatencyBackend{
		Backend:      fs,
		latency:      time.Duration(latencyMS) * time.Millisecond,
		manifestKeys: manifestKeys,
	}

	pairs := []string{
		"ARMOR_BACKEND", "filesystem",
		"ARMOR_FS_PATH", root,
		"ARMOR_BUCKET", "b",
		"ARMOR_MEK", asyncLoadTestMEK,
		"ARMOR_AUTH_ACCESS_KEY", asyncLoadTestAccessKey,
		"ARMOR_AUTH_SECRET_KEY", asyncLoadTestSecretKey,
		// The serving pod is a fresh writer, like every iad-ci restart:
		// its shard is none of the 71 seeded ones.
		"ARMOR_WRITER_ID", "armor-scale-live",
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
	srv, err := NewWithBackend(cfg, be)
	if err != nil {
		t.Fatalf("NewWithBackend(): %v", err)
	}
	// The production start path under measurement, installed exactly as
	// New does (mirrors newAsyncColdLoadServer).
	loadStart := time.Now()
	sys := newManifestSystem()
	sys.start(cfg, logging.New("test"), be)
	srv.manifestSys = sys
	t.Cleanup(srv.StopManifestWriter)

	// Leg 1: the load must resolve loaded inside the budget. The wait bound
	// is the budget plus margin so a degraded landing (timeout at 480 s)
	// reports as the failure it is rather than as a hung test.
	status := srv.WaitForManifestLoad(scaleLoadBudget + 60*time.Second)
	loadDuration := time.Since(loadStart)
	if status != manifestLoaded {
		t.Fatalf("manifest load settled as %q after %s, want %q — the census-shaped store did not load inside the %s budget", status, loadDuration.Round(time.Millisecond), manifestLoaded, scaleLoadBudget)
	}
	if loadDuration >= scaleLoadBudget {
		t.Fatalf("manifest load took %s, want < %s", loadDuration.Round(time.Millisecond), scaleLoadBudget)
	}
	fetches := be.fetches.Load()
	t.Logf("leg1: cold load of %d manifest objects (%d latency-priced fetches, %d ms injected latency each) resolved loaded in %s — %.1fx under the %s budget, %.0f fetches/s effective",
		len(manifestKeys), fetches, latencyMS, loadDuration.Round(time.Millisecond),
		float64(scaleLoadBudget)/float64(loadDuration), scaleLoadBudget,
		float64(fetches)/loadDuration.Seconds())
	if got := srv.manifestIndex().Len(); got < scaleListedKeys {
		t.Fatalf("loaded index holds %d entries, want at least the %d transcripts keys", got, scaleListedKeys)
	}

	// Leg 2: one full transcripts page must resolve entirely from the
	// index — zero backend HEADs. The empty-index fallback this replaces
	// costs one HEAD per key (1000 per page on iad-ci).
	s3Listener := httptest.NewServer(srv.Handler())
	t.Cleanup(s3Listener.Close)
	resp := signedS3Request(t, http.MethodGet, s3Listener.URL+"/b/?list-type=2&prefix=transcripts/&max-keys=1000", nil)
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		t.Fatalf("ListObjectsV2 status = %d: %s", resp.StatusCode, body)
	}
	var parsed listContentsV2
	if err := xml.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		t.Fatalf("decode ListObjectsV2 response: %v", err)
	}
	if len(parsed.Keys) != scaleListedKeys {
		t.Fatalf("ListObjectsV2 returned %d keys, want %d", len(parsed.Keys), scaleListedKeys)
	}
	for _, k := range parsed.Keys {
		if !strings.HasPrefix(k.Key, "transcripts/") {
			t.Fatalf("unexpected listed key %q", k.Key)
		}
		if k.Size != transcriptsPlaintext {
			t.Fatalf("key %s: size = %d, want the index-resolved plaintext %d", k.Key, k.Size, transcriptsPlaintext)
		}
	}
	heads := be.heads.Load()
	t.Logf("leg2: %d-key ListObjectsV2 page resolved %d/%d sizes from the manifest index with %d backend HEADs (empty-index fallback: ~%d HEADs/page)",
		len(parsed.Keys), len(parsed.Keys), scaleListedKeys, heads, scaleListedKeys)
	if heads != 0 {
		t.Fatalf("%d backend HEADs served a page the loaded index fully covers", heads)
	}
}
