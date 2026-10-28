package handlers_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"syscall"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/keymanager"
	"github.com/jedarden/armor/internal/server/handlers"
)

// BenchmarkListObjectsV2Filesystem measures one 1000-key page from a bucket
// containing 10,000 v3 multipart objects beneath one prefix. Each ciphertext
// object has an ADR-016 .armor-manifest sibling, a filesystem .metadata file,
// and an internal .armor/hmac sidecar. The direct backend leg separates the
// filesystem walk from handler enrichment and XML serialization. Run with
// -benchtime=1x and one CPU to compare revisions against the same fixture.
func BenchmarkListObjectsV2Filesystem(b *testing.B) {
	const (
		bucket = "benchmark-bucket"
		prefix = "benchmark/"
		keys   = 10_000
	)
	ctx := context.Background()
	fs, err := backend.NewFSBackend(backend.FSConfig{BasePath: b.TempDir()})
	if err != nil {
		b.Fatal(err)
	}
	if err := fs.CreateBucket(ctx, bucket); err != nil {
		b.Fatal(err)
	}

	// The LIST path needs only the stored object's metadata. Keep fixture
	// construction outside both timers while preserving the multipart layout.
	armorMeta := (&backend.ARMORMetadata{
		Version:        3,
		BlockSize:      65536,
		PlaintextSize:  1,
		ContentType:    "application/octet-stream",
		IV:             make([]byte, 16),
		WrappedDEK:     make([]byte, 32),
		MEKFingerprint: "0123456789abcdef",
	}).ToMetadata()
	meta := armorMeta
	manifestMeta := make(map[string]string, len(armorMeta)+1)
	for key, value := range armorMeta {
		manifestMeta[key] = value
	}
	manifestMeta["Content-Type"] = "application/x-armor-manifest+json"
	manifestMeta["x-amz-meta-armor-completed-at"] = "2026-10-28T00:00:00Z"
	data := []byte("v3 ciphertext")
	manifest := []byte(`{"ciphertext_object":"benchmark/object"}`)
	sidecar := []byte("v3 sidecar")
	index := newMockManifestRecorder()
	for i := 0; i < keys; i++ {
		key := fmt.Sprintf("%sobject-%06d", prefix, i)
		if err := fs.Put(ctx, bucket, key, bytes.NewReader(data), int64(len(data)), meta); err != nil {
			b.Fatalf("seed ciphertext %s: %v", key, err)
		}
		manifestKey := key + ".armor-manifest"
		manifestMeta["x-amz-meta-armor-ciphertext-ref"] = key
		if err := fs.Put(ctx, bucket, manifestKey, bytes.NewReader(manifest), int64(len(manifest)), manifestMeta); err != nil {
			b.Fatalf("seed manifest %s: %v", manifestKey, err)
		}
		sidecarKey := backend.GetSidecarKey("", key)
		if err := fs.Put(ctx, bucket, sidecarKey, bytes.NewReader(sidecar), int64(len(sidecar)), nil); err != nil {
			b.Fatalf("seed sidecar %s: %v", sidecarKey, err)
		}
		index.seed(bucket, key, &handlers.ManifestEntry{PlaintextSize: 1})
	}

	b.Run("Backend", func(b *testing.B) {
		b.ReportAllocs()
		wall := make([]time.Duration, 0, b.N)
		b.ResetTimer()
		cpuStart := listBenchmarkCPUTime(b)
		var result *backend.ListResult
		for i := 0; i < b.N; i++ {
			start := time.Now()
			result, err = fs.List(ctx, bucket, prefix, "", "", 1000)
			if err != nil {
				b.Fatal(err)
			}
			wall = append(wall, time.Since(start))
		}
		cpuElapsed := listBenchmarkCPUTime(b) - cpuStart
		b.StopTimer()
		b.ReportMetric(float64(cpuElapsed.Nanoseconds())/float64(b.N), "cpu-ns/op")
		sort.Slice(wall, func(i, j int) bool { return wall[i] < wall[j] })
		b.ReportMetric(float64(wall[len(wall)/2].Microseconds())/1000, "p50-ms/op")
		if got := len(result.Objects); got != 1000 {
			b.Fatalf("backend returned %d objects, want one 1000-key page", got)
		}
	})

	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		b.Fatal(err)
	}
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		b.Fatal(err)
	}
	cfg := &config.Config{BlockSize: 65536, FormatWriteVersion: 3}
	h := handlers.New(cfg, fs, backend.NewMetadataCache(1000, 300), backend.NewFooterCache(1000, 300), km, nil)
	h.WithManifest(index)
	request := httptest.NewRequest(http.MethodGet, "/"+bucket+"?list-type=2&prefix="+prefix+"&max-keys=1000", nil)
	b.Run("Handler", func(b *testing.B) {
		b.ReportAllocs()
		wall := make([]time.Duration, 0, b.N)
		b.ResetTimer()
		cpuStart := listBenchmarkCPUTime(b)
		var response *httptest.ResponseRecorder
		for i := 0; i < b.N; i++ {
			start := time.Now()
			response = httptest.NewRecorder()
			h.ListObjectsV2(response, request, bucket)
			if response.Code != http.StatusOK {
				b.Fatalf("list: %d %s", response.Code, response.Body.String())
			}
			wall = append(wall, time.Since(start))
		}
		cpuElapsed := listBenchmarkCPUTime(b) - cpuStart
		b.StopTimer()
		b.ReportMetric(float64(cpuElapsed.Nanoseconds())/float64(b.N), "cpu-ns/op")
		sort.Slice(wall, func(i, j int) bool { return wall[i] < wall[j] })
		b.ReportMetric(float64(wall[len(wall)/2].Microseconds())/1000, "p50-ms/op")
		if got := bytes.Count(response.Body.Bytes(), []byte("<Contents>")); got != 1000 {
			b.Fatalf("handler returned %d XML Contents, want one 1000-key page", got)
		}
	})
}

func listBenchmarkCPUTime(b *testing.B) time.Duration {
	b.Helper()
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		b.Fatal(err)
	}
	return time.Duration(usage.Utime.Sec+usage.Stime.Sec)*time.Second +
		time.Duration(usage.Utime.Usec+usage.Stime.Usec)*time.Microsecond
}
