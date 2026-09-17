package srvtest

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/replication"
	"github.com/jedarden/armor/internal/server"
)

const (
	// DefaultBucket is the single client-visible bucket the harness serves.
	DefaultBucket = "test-bucket"
	// DefaultPrefix is the ARMOR_PREFIX the harness configures. Replication
	// must land the STORED (prefixed) keys, so exercising a non-empty prefix
	// is the default, not the exception.
	DefaultPrefix = "tenant/"

	// WaitTimeout bounds enqueue/landing polling before a test fails.
	WaitTimeout = 10 * time.Second
	// PollInterval is the landing-poll cadence.
	PollInterval = 20 * time.Millisecond
)

// Harness is a fully wired dual-backend ARMOR deployment under test: the real
// internal/server Server (authenticated SigV4 mux, real S3 handlers), a real
// filesystem primary backend, a FaultBackend-wrapped filesystem secondary, and
// the real internal/replication queue — wired together exactly the way
// server.New wires them for a configured ADR-006 secondary.
type Harness struct {
	t *testing.T

	// Server is the real server; Handler() is the authenticated mux clients hit.
	Server *server.Server

	// Primary is the authoritative backend. Secondary is the replication
	// target wrapped with fault injection; use Secondary.Backend for the
	// unwrapped view.
	Primary   backend.Backend
	Secondary *FaultBackend

	// Queue is the real replication queue; QueueMetrics its counters.
	Queue        *replication.ReplicationQueue
	QueueMetrics *replication.Metrics

	Bucket string
	Prefix string

	cancel context.CancelFunc
}

// HarnessConfig overrides the harness defaults (all fields optional).
type HarnessConfig struct {
	Bucket string
	Prefix string
}

// New builds a Harness with defaults. Cleanup (queue stop) is registered on t.
func New(t *testing.T) *Harness {
	return NewWithConfig(t, HarnessConfig{})
}

// NewWithConfig builds a Harness with explicit bucket/prefix overrides.
func NewWithConfig(t *testing.T, hc HarnessConfig) *Harness {
	t.Helper()

	bucket := hc.Bucket
	if bucket == "" {
		bucket = DefaultBucket
	}
	prefix := hc.Prefix
	if prefix == "" {
		prefix = DefaultPrefix
	}

	primary, err := backend.NewFSBackend(backend.FSConfig{BasePath: t.TempDir(), KeyPrefix: prefix})
	if err != nil {
		t.Fatalf("srvtest: primary filesystem backend: %v", err)
	}
	realSecondary, err := backend.NewFSBackend(backend.FSConfig{BasePath: t.TempDir(), KeyPrefix: prefix})
	if err != nil {
		t.Fatalf("srvtest: secondary filesystem backend: %v", err)
	}
	secondary := NewFaultBackend(realSecondary)

	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("srvtest: generate MEK: %v", err)
	}
	cfg := &config.Config{
		BlockSize: 64 * 1024,
		MEK:       mek,
		B2Region:  TestRegion,
		Prefix:    prefix,
		Credentials: map[string]*config.Credential{
			TestAccessKey: {AccessKey: TestAccessKey, SecretKey: TestSecretKey},
		},
		CacheMaxEntries: 1000,
		CacheTTL:        300,
	}

	srv, err := server.NewWithBackend(cfg, primary)
	if err != nil {
		t.Fatalf("srvtest: server: %v", err)
	}

	// Mirror server.New's ADR-006 wiring: the secondary backend and the
	// replication queue (empty target bucket = reuse the source bucket, the
	// filesystem-backend behavior; key prefix matches the server prefix).
	srv.SetSecondaryBackend(secondary)
	queueMetrics := replication.NewMetrics()
	queue := replication.NewReplicationQueueWithTargetBucketAndPrefix(
		queueMetrics, primary, secondary, "", prefix, 256,
		log.New(io.Discard, "", 0))
	srv.SetReplicationQueue(queue)

	ctx, cancel := context.WithCancel(context.Background())
	queue.Start(ctx)
	t.Cleanup(func() {
		cancel()
		queue.Stop()
	})

	for _, be := range []backend.Backend{primary, realSecondary} {
		if err := be.CreateBucket(context.Background(), bucket); err != nil {
			t.Fatalf("srvtest: create bucket %q: %v", bucket, err)
		}
	}

	return &Harness{
		t:            t,
		Server:       srv,
		Primary:      primary,
		Secondary:    secondary,
		Queue:        queue,
		QueueMetrics: queueMetrics,
		Bucket:       bucket,
		Prefix:       prefix,
		cancel:       cancel,
	}
}

// Handler returns the server's authenticated S3 mux — the real client-facing
// request path (SigV4 verification, ACL check, aws-chunked decode, handlers).
func (h *Harness) Handler() http.Handler {
	return h.Server.Handler()
}

// StoredKey maps a client object key to the backend storage key the handlers
// write and the replication queue receives (ARMOR_PREFIX applied).
func (h *Harness) StoredKey(key string) string {
	return h.Prefix + key
}

// SignedRequest builds a SigV4-signed request against the harness server.
func (h *Harness) SignedRequest(method, target string, body []byte) *http.Request {
	h.t.Helper()
	req := httptest.NewRequest(method, target, bytes.NewReader(body))
	SignS3Request(req, body, TestAccessKey, TestSecretKey, TestRegion)
	return req
}

// Do performs a signed request through the real authenticated mux.
func (h *Harness) Do(method, target string, body []byte) *httptest.ResponseRecorder {
	h.t.Helper()
	req := h.SignedRequest(method, target, body)
	rec := httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, req)
	return rec
}

// PutObject writes an object through the real PUT path and returns its ETag.
func (h *Harness) PutObject(bucket, key string, body []byte) string {
	h.t.Helper()
	rec := h.Do(http.MethodPut, "/"+bucket+"/"+key, body)
	if rec.Code != http.StatusOK {
		h.t.Fatalf("PutObject %s/%s: status %d: %s", bucket, key, rec.Code, rec.Body.String())
	}
	return strings.Trim(rec.Header().Get("ETag"), `"`)
}

// MultipartUpload drives CreateMultipartUpload, one UploadPart per given part
// body, and CompleteMultipartUpload through the real paths, returning the
// final ETag.
func (h *Harness) MultipartUpload(bucket, key string, parts ...[]byte) string {
	h.t.Helper()

	rec := h.Do(http.MethodPost, "/"+bucket+"/"+key+"?uploads", nil)
	if rec.Code != http.StatusOK {
		h.t.Fatalf("CreateMultipartUpload %s/%s: status %d: %s", bucket, key, rec.Code, rec.Body.String())
	}
	var created struct {
		UploadID string `xml:"UploadId"`
	}
	if err := xml.Unmarshal(rec.Body.Bytes(), &created); err != nil || created.UploadID == "" {
		h.t.Fatalf("CreateMultipartUpload %s/%s: parse UploadId: %v (body %s)", bucket, key, err, rec.Body.String())
	}

	etags := make([]string, len(parts))
	for i, part := range parts {
		prec := h.Do(http.MethodPut,
			fmt.Sprintf("/%s/%s?partNumber=%d&uploadId=%s", bucket, key, i+1, created.UploadID), part)
		if prec.Code != http.StatusOK {
			h.t.Fatalf("UploadPart %d %s/%s: status %d: %s", i+1, bucket, key, prec.Code, prec.Body.String())
		}
		etags[i] = strings.Trim(prec.Header().Get("ETag"), `"`)
	}

	var completeBody strings.Builder
	completeBody.WriteString("<CompleteMultipartUpload>")
	for i := range parts {
		fmt.Fprintf(&completeBody, "<Part><PartNumber>%d</PartNumber><ETag>%s</ETag></Part>", i+1, etags[i])
	}
	completeBody.WriteString("</CompleteMultipartUpload>")

	crec := h.Do(http.MethodPost, "/"+bucket+"/"+key+"?uploadId="+created.UploadID, []byte(completeBody.String()))
	if crec.Code != http.StatusOK {
		h.t.Fatalf("CompleteMultipartUpload %s/%s: status %d: %s", bucket, key, crec.Code, crec.Body.String())
	}
	var done struct {
		ETag string `xml:"ETag"`
	}
	if err := xml.Unmarshal(crec.Body.Bytes(), &done); err != nil {
		h.t.Fatalf("CompleteMultipartUpload %s/%s: parse result: %v (body %s)", bucket, key, err, crec.Body.String())
	}
	return done.ETag
}

// WaitUntilEnqueued waits until the queue has accepted n replication tasks.
// The handlers enqueue in a goroutine after the client ack, so this closes the
// ack-to-enqueue race before landing is polled.
func (h *Harness) WaitUntilEnqueued(n int64, timeout time.Duration) {
	h.t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if h.QueueMetrics.EnqueuedTotal.Load() >= n {
			return
		}
		time.Sleep(PollInterval)
	}
	h.t.Fatalf("queue enqueued %d of %d expected tasks within %s",
		h.QueueMetrics.EnqueuedTotal.Load(), n, timeout)
}

// WaitUntilReplicated polls the secondary until every given stored key is
// present. Presence is the landing signal; byte comparison is done by
// AssertLandingSet.
func (h *Harness) WaitUntilReplicated(keys []string, timeout time.Duration) {
	h.t.Helper()
	deadline := time.Now().Add(timeout)
	var missing []string
	for {
		missing = missing[:0]
		for _, k := range keys {
			if _, err := h.Secondary.Head(context.Background(), h.Bucket, k); err != nil {
				missing = append(missing, k)
			}
		}
		if len(missing) == 0 {
			return
		}
		if !time.Now().Before(deadline) {
			break
		}
		time.Sleep(PollInterval)
	}
	h.t.Fatalf("secondary did not receive %v within %s (received %d objects)", missing, timeout, len(h.Snapshot(h.Secondary)))
}

// Snapshot enumerates every raw key in the bucket on the given backend and
// reads each object's bytes.
// snapshotAttempts bounds how many listings Snapshot will take when a listed
// key fails to Get. A listing swept while a backend put is mid-flight can
// observe its staging temp before the rename publishes it — the filesystem
// backend's .armor-put-*.tmp has no metadata sidecar yet, so Get on it fails
// and an unlucky convergence poll dies on an unrelated in-flight copy.
// Re-listing snapshots only stable state; a temp that genuinely leaks is the
// defect this must still catch, and it stays listed and fails every attempt.
const snapshotAttempts = 5

func (h *Harness) Snapshot(be backend.Backend) map[string][]byte {
	h.t.Helper()
	var (
		failedKey string
		failedErr error
	)
	for attempt := 1; ; attempt++ {
		out, key, err := h.listAndReadAll(be)
		if err == nil {
			return out
		}
		failedKey, failedErr = key, err
		if attempt == snapshotAttempts {
			h.t.Fatalf("srvtest: Get %q after listing still fails after %d listings (not a transient in-flight put temp): %v", failedKey, snapshotAttempts, failedErr)
		}
		time.Sleep(PollInterval)
	}
}

// listAndReadAll takes one listing of every key on be — including internal
// state, since callers decide what counts — and reads each object whole. On
// the first Get (or read) failure it returns the partial map, the failing
// key, and the error so Snapshot can decide whether a re-list heals it.
func (h *Harness) listAndReadAll(be backend.Backend) (map[string][]byte, string, error) {
	res, err := be.ListRaw(context.Background(), h.Bucket, "", "", "", 0)
	if err != nil {
		h.t.Fatalf("srvtest: ListRaw: %v", err)
	}
	out := make(map[string][]byte, len(res.Objects))
	for _, obj := range res.Objects {
		body, _, err := be.Get(context.Background(), h.Bucket, obj.Key)
		if err != nil {
			return out, obj.Key, err
		}
		data, err := io.ReadAll(body)
		body.Close()
		if err != nil {
			return out, obj.Key, err
		}
		out[obj.Key] = data
	}
	return out, "", nil
}

// IsInternalKey reports whether a backend key is reserved ARMOR state
// (multipart bookkeeping, HMAC sidecars, manifest deltas) that must never be
// replicated as a client object — the server-level counterpart of the
// queue's own isInternalKey filter.
func IsInternalKey(key string) bool {
	return strings.HasPrefix(key, ".armor/") || strings.Contains(key, "/.armor/")
}

// AssertLandingSet enumerates BOTH backends and asserts the dual-backend
// landing contract:
//
//   - every expected (replicated) key exists on the secondary, byte-identical
//     to the primary copy — this covers the embedded HMAC table of a v2/v3
//     envelope, which travels inside the replicated object bytes;
//   - the secondary holds EXACTLY the expected set — nothing missing, and no
//     extra keys. Internal `.armor/` state must not cross (consistent with
//     internal/replication's TestInternalStateIsNotReplicated), and neither
//     may any other primary-only key.
//
// Note on ADR-016 sidecars: the multipart manifest lives at
// `<stored-key>.armor-manifest` (not under `.armor/`), and handlers enqueue
// only the data key today — so the exact-set assertion pins it on the primary.
// If sidecar manifests are ever enqueued for replication, extend `expected`
// consciously rather than loosening this assertion.
func (h *Harness) AssertLandingSet(expected []string) {
	h.t.Helper()

	primary := h.Snapshot(h.Primary)
	secondary := h.Snapshot(h.Secondary)

	expectedSet := make(map[string]bool, len(expected))
	for _, k := range expected {
		expectedSet[k] = true
	}

	for _, k := range expected {
		pBytes, ok := primary[k]
		if !ok {
			h.t.Fatalf("primary is missing expected replicated key %q", k)
		}
		sBytes, ok := secondary[k]
		if !ok {
			h.t.Fatalf("secondary did not receive replicated key %q", k)
		}
		if !bytes.Equal(pBytes, sBytes) {
			h.t.Fatalf("secondary bytes differ from primary for %q (primary %d bytes, secondary %d bytes)", k, len(pBytes), len(sBytes))
		}
	}

	for k := range secondary {
		if !expectedSet[k] {
			h.t.Errorf("secondary holds unexpected key %q", k)
		}
		if IsInternalKey(k) {
			h.t.Errorf("internal ARMOR state %q was replicated to the secondary", k)
		}
	}

	for k := range primary {
		if expectedSet[k] || IsInternalKey(k) {
			continue
		}
		h.t.Logf("primary-only non-internal key %q was not replicated (handlers enqueue only the data key)", k)
	}
}

// AssertQueueDrained asserts the queue accepted exactly n tasks, dropped
// nothing, retried nothing, and holds no pending work.
func (h *Harness) AssertQueueDrained(n int64) {
	h.t.Helper()
	if got := h.QueueMetrics.EnqueuedTotal.Load(); got != n {
		h.t.Errorf("EnqueuedTotal = %d, want %d", got, n)
	}
	if got := h.QueueMetrics.DroppedTotal.Load(); got != 0 {
		h.t.Errorf("DroppedTotal = %d, want 0", got)
	}
	if got := h.QueueMetrics.ErrorsTotal.Load(); got != 0 {
		h.t.Errorf("ErrorsTotal = %d, want 0", got)
	}
	if got := h.QueueMetrics.RetriesTotal.Load(); got != 0 {
		h.t.Errorf("RetriesTotal = %d, want 0", got)
	}
	if got := h.QueueMetrics.QueueDepth.Load(); got != 0 {
		h.t.Errorf("QueueDepth = %d, want 0", got)
	}
}

// RandomBytes returns n cryptographically random bytes (incompressible,
// block-boundary-crossing payloads).
func RandomBytes(t *testing.T, n int) []byte {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("srvtest: random bytes: %v", err)
	}
	return b
}
