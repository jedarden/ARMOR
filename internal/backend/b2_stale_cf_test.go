package backend

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// These tests pin the stale-Cloudflare-copy defence. After an overwrite the
// edge can keep serving the old version's bytes while the request's HEAD (direct
// to B2) sees the new one; the read path must notice the mismatch through the
// pinned version and re-read straight from B2 instead of failing a block check.

const (
	staleVersion = "4_zold_version"
	freshVersion = "4_znew_version"
)

type staleFixture struct {
	cf         *httptest.Server
	s3srv      *httptest.Server
	mu         sync.Mutex
	cfPaths    []string
	s3Versions []string
	fresh      []byte
}

// newStaleFixture serves the stale object (all 'o') from the "edge" and the
// fresh object (all 'n') from the "origin" S3 API, selected by versionId.
func newStaleFixture(t *testing.T, edgeStatus int) *staleFixture {
	t.Helper()
	f := &staleFixture{fresh: []byte(strings.Repeat("n", 64))}
	f.cf = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.cfPaths = append(f.cfPaths, r.URL.RequestURI())
		f.mu.Unlock()
		w.Header().Set("X-Bz-File-Id", staleVersion)
		w.WriteHeader(edgeStatus)
		if edgeStatus == http.StatusPartialContent {
			_, _ = w.Write([]byte(strings.Repeat("o", 16)))
		}
	}))
	f.s3srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.s3Versions = append(f.s3Versions, r.URL.Query().Get("versionId"))
		f.mu.Unlock()
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(f.fresh[:16])
	}))
	t.Cleanup(f.cf.Close)
	t.Cleanup(f.s3srv.Close)
	return f
}

func (f *staleFixture) backend(fallback, pinnedURL bool) *B2Backend {
	return &B2Backend{
		s3Client: s3.New(s3.Options{
			BaseEndpoint: aws.String(f.s3srv.URL),
			Region:       "us-west-002",
			UsePathStyle: true,
			Credentials:  credentials.NewStaticCredentialsProvider("id", "secret", ""),
		}),
		cfDomain:        strings.TrimPrefix(f.cf.URL, "https://"),
		httpClient:      f.cf.Client(),
		readBlockSize:   64,
		readConcurrency: 2,
		cfStaleFallback: fallback,
		cfVersionPinned: pinnedURL,
	}
}

func readAll(t *testing.T, b *B2Backend, ctx context.Context) string {
	t.Helper()
	body, err := b.GetRange(ctx, "bucket", "key", 0, 16)
	if err != nil {
		t.Fatalf("GetRange: %v", err)
	}
	defer body.Close()
	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return string(data)
}

func pinnedCtx(version string) context.Context {
	ctx := WithVersionPin(context.Background())
	recordVersionPin(ctx, "bucket", "key", version)
	return ctx
}

func TestStaleCloudflareCopyIsRefetchedDirectAtPinnedVersion(t *testing.T) {
	f := newStaleFixture(t, http.StatusPartialContent)
	before := CFStaleFallbackCount()

	got := readAll(t, f.backend(true, false), pinnedCtx(freshVersion))

	if got != strings.Repeat("n", 16) {
		t.Fatalf("read %q, want the fresh version's bytes", got)
	}
	if len(f.s3Versions) != 1 || f.s3Versions[0] != freshVersion {
		t.Fatalf("direct reads = %v, want one at version %s", f.s3Versions, freshVersion)
	}
	if CFStaleFallbackCount() != before+1 {
		t.Fatalf("fallback counter did not advance")
	}
}

func TestStaleCloudflare416IsRefetchedDirect(t *testing.T) {
	f := newStaleFixture(t, http.StatusRequestedRangeNotSatisfiable)

	got := readAll(t, f.backend(true, false), pinnedCtx(freshVersion))

	if got != strings.Repeat("n", 16) {
		t.Fatalf("read %q, want the fresh version's bytes", got)
	}
}

func TestMatchingVersionIsServedFromCloudflare(t *testing.T) {
	f := newStaleFixture(t, http.StatusPartialContent)

	got := readAll(t, f.backend(true, false), pinnedCtx(staleVersion)) // edge copy IS current

	if got != strings.Repeat("o", 16) {
		t.Fatalf("read %q, want the edge bytes", got)
	}
	if len(f.s3Versions) != 0 {
		t.Fatalf("direct reads = %v, want none when the edge copy is current", f.s3Versions)
	}
}

func TestNoPinMeansNoFallback(t *testing.T) {
	f := newStaleFixture(t, http.StatusPartialContent)

	got := readAll(t, f.backend(true, false), context.Background())

	if got != strings.Repeat("o", 16) || len(f.s3Versions) != 0 {
		t.Fatalf("unpinned read changed behavior: %q, direct=%v", got, f.s3Versions)
	}
}

func TestFallbackDisabledKeepsOldBehavior(t *testing.T) {
	f := newStaleFixture(t, http.StatusPartialContent)

	got := readAll(t, f.backend(false, false), pinnedCtx(freshVersion))

	if got != strings.Repeat("o", 16) || len(f.s3Versions) != 0 {
		t.Fatalf("disabled fallback changed behavior: %q, direct=%v", got, f.s3Versions)
	}
}

func TestVersionPinnedURLNamesTheFileID(t *testing.T) {
	f := newStaleFixture(t, http.StatusPartialContent)

	_ = readAll(t, f.backend(false, true), pinnedCtx(freshVersion))

	if len(f.cfPaths) != 1 || f.cfPaths[0] != "/b2api/v1/b2_download_file_by_id?fileId="+freshVersion {
		t.Fatalf("cloudflare requests = %v, want the by-file-ID URL", f.cfPaths)
	}
}

func TestVersionPinnedURLNeedsAPin(t *testing.T) {
	f := newStaleFixture(t, http.StatusPartialContent)

	_ = readAll(t, f.backend(false, true), context.Background())

	if len(f.cfPaths) != 1 || f.cfPaths[0] != "/file/bucket/key" {
		t.Fatalf("cloudflare requests = %v, want the by-key URL when nothing is pinned", f.cfPaths)
	}
}

func TestPinKeepsTheFirstVersionPerKey(t *testing.T) {
	ctx := WithVersionPin(context.Background())
	recordVersionPin(ctx, "b", "k", "v1")
	recordVersionPin(ctx, "b", "k", "v2")
	recordVersionPin(ctx, "b", "other", "v9")
	if got := pinnedVersion(ctx, "b", "k"); got != "v1" {
		t.Fatalf("pinned = %q, want v1", got)
	}
	if got := pinnedVersion(ctx, "b", "other"); got != "v9" {
		t.Fatalf("pinned other = %q, want v9", got)
	}
	if got := pinnedVersion(context.Background(), "b", "k"); got != "" {
		t.Fatalf("pin leaked into an unpinned context: %q", got)
	}
}

func TestHeadPinsTheVersionB2Reports(t *testing.T) {
	f := newStaleFixture(t, http.StatusPartialContent)
	f.s3srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-amz-version-id", freshVersion)
		w.Header().Set("Content-Length", "64")
		w.WriteHeader(http.StatusOK)
	})
	b := f.backend(true, false)
	ctx := WithVersionPin(context.Background())

	info, err := b.Head(ctx, "bucket", "key")
	if err != nil {
		t.Fatalf("Head: %v", err)
	}
	if info.VersionID != freshVersion {
		t.Fatalf("ObjectInfo.VersionID = %q, want %q", info.VersionID, freshVersion)
	}
	if got := pinnedVersion(ctx, "bucket", "key"); got != freshVersion {
		t.Fatalf("pinned = %q, want %q", got, freshVersion)
	}
}
