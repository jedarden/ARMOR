package srvtest

// Server-level verification of the complete ADR-006 provider-outage route:
// promote the mirrored backend, serve reads and writes from it, attach a
// replacement backend as its secondary, backfill a pre-outage object by
// re-uploading it, and finally fail back to the replacement as the durable
// primary. The provider-outage restore tests cover the read-only promotion
// boundary; this drill covers the write and return-to-primary legs.

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/replication"
	"github.com/jedarden/armor/internal/server"
)

const (
	failbackAccessKey = "armor-failback-access-key"  // gitleaks:allow
	failbackSecretKey = "armor-failback-signing-key" // gitleaks:allow
)

// newFailoverServer constructs a fresh deployment against primary and, when
// supplied, wires a real replication queue to secondary. It intentionally
// uses new credentials: after provider loss, only the escrowed MEK crosses
// into the promoted deployment.
func newFailoverServer(t *testing.T, h *Harness, primary, secondary backend.Backend, accessKey, secretKey string) (*server.Server, *replication.ReplicationQueue, *replication.Metrics, context.CancelFunc) {
	t.Helper()
	cfg := &config.Config{
		BlockSize:          64 * 1024,
		MEK:                h.MEK,
		B2Region:           TestRegion,
		Prefix:             h.Prefix,
		FormatWriteVersion: 3,
		Credentials: map[string]*config.Credential{
			accessKey: {AccessKey: accessKey, SecretKey: secretKey},
		},
		CacheMaxEntries: 1000,
		CacheTTL:        300,
	}
	srv, err := server.NewWithBackend(cfg, primary)
	if err != nil {
		t.Fatalf("failover server: %v", err)
	}
	if secondary == nil {
		return srv, nil, nil, func() {}
	}

	srv.SetSecondaryBackend(secondary)
	metrics := replication.NewMetrics()
	queue := replication.NewReplicationQueueWithTargetBucketAndPrefix(
		metrics, primary, secondary, "", h.Prefix, 256,
		log.New(io.Discard, "", 0),
	)
	srv.SetReplicationQueue(queue)
	ctx, cancel := context.WithCancel(context.Background())
	queue.Start(ctx)
	t.Cleanup(func() {
		cancel()
		queue.Stop()
	})
	return srv, queue, metrics, cancel
}

func signFailbackRequest(t *testing.T, method, target string, body []byte, accessKey, secretKey string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, target, bytes.NewReader(body))
	SignS3Request(req, body, accessKey, secretKey, TestRegion)
	return req
}

func failbackDo(t *testing.T, handler http.Handler, method, target string, body []byte, accessKey, secretKey string) *httptest.ResponseRecorder {
	t.Helper()
	req := signFailbackRequest(t, method, target, body, accessKey, secretKey)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func readBackendObject(t *testing.T, be backend.Backend, bucket, key string) []byte {
	t.Helper()
	body, _, err := be.Get(context.Background(), bucket, key)
	if err != nil {
		t.Fatalf("backend Get %s/%s: %v", bucket, key, err)
	}
	data, err := io.ReadAll(body)
	body.Close()
	if err != nil {
		t.Fatalf("backend read %s/%s: %v", bucket, key, err)
	}
	return data
}

func waitForBackendObject(t *testing.T, be backend.Backend, bucket, key string, timeout time.Duration) []byte {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := be.Head(context.Background(), bucket, key); err == nil {
			return readBackendObject(t, be, bucket, key)
		}
		time.Sleep(PollInterval)
	}
	t.Fatalf("backend did not receive %s/%s within %s", bucket, key, timeout)
	return nil
}

func assertFailoverMetrics(t *testing.T, metrics *replication.Metrics, wantEnqueued int64) {
	t.Helper()
	if got := metrics.EnqueuedTotal.Load(); got != wantEnqueued {
		t.Errorf("promoted replication EnqueuedTotal = %d, want %d", got, wantEnqueued)
	}
	if got := metrics.QueueDepth.Load(); got != 0 {
		t.Errorf("promoted replication QueueDepth = %d after recovery, want 0", got)
	}
	if got := metrics.LagSeconds.Load(); got != 0 {
		t.Errorf("promoted replication LagSeconds = %d after recovery, want 0", got)
	}
	if got := metrics.DroppedTotal.Load(); got != 0 {
		t.Errorf("promoted replication DroppedTotal = %d, want 0", got)
	}
	if got := metrics.ErrorsTotal.Load(); got != 0 {
		t.Errorf("promoted replication ErrorsTotal = %d, want 0", got)
	}
	if got := metrics.RetriesTotal.Load(); got != 0 {
		t.Errorf("promoted replication RetriesTotal = %d, want 0", got)
	}
}

// TestProviderOutageFailoverWritesAndFailback exercises Route A end to end.
// The original primary is destroyed only after its pre-outage object has
// landed on the mirror. The promoted mirror then accepts a new write, sends
// it to a newly provisioned replacement, re-uploads the pre-outage object to
// backfill it, and is replaced as the durable primary once the queue drains.
func TestProviderOutageFailoverWritesAndFailback(t *testing.T) {
	h := NewWithConfig(t, HarnessConfig{FormatWriteVersion: 3})

	const (
		preOutageKey = "failover/pre-outage.txt"
		newKey       = "failover/written-on-promoted.bin"
		finalKey     = "failover/written-after-failback.txt"
	)
	preOutage := []byte("durable before the provider outage")
	newOnPromoted := RandomBytes(t, 192*1024+19)
	finalWrite := []byte("the replacement primary is serving writes")

	h.PutObject(h.Bucket, preOutageKey, preOutage)
	preStored := h.StoredKey(preOutageKey)
	h.WaitUntilEnqueued(1, WaitTimeout)
	h.WaitUntilReplicated([]string{preStored}, WaitTimeout)

	// The failed provider is gone. No request in the promoted deployment may
	// retain a route to h.Primary.
	h.cancel()
	h.Queue.Stop()
	destroyPrimary(t, h)

	replacement, err := backend.NewFSBackend(backend.FSConfig{
		BasePath:  t.TempDir(),
		KeyPrefix: h.Prefix,
	})
	if err != nil {
		t.Fatalf("replacement filesystem backend: %v", err)
	}
	if err := replacement.CreateBucket(context.Background(), h.Bucket); err != nil {
		t.Fatalf("replacement CreateBucket: %v", err)
	}

	// Promotion: the old secondary is now the primary, and its existing
	// object must remain readable with only the escrowed MEK.
	promoted, _, promotedMetrics, promotedCancel := newFailoverServer(t, h, h.Secondary.Backend, replacement, restoreAccessKey, restoreSecretKey)
	promotedHandler := promoted.Handler()
	rec := failbackDo(t, promotedHandler, http.MethodGet, "/"+h.Bucket+"/"+preOutageKey, nil, restoreAccessKey, restoreSecretKey)
	if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), preOutage) {
		t.Fatalf("promoted GET %s: status %d, body %d bytes; want the pre-outage plaintext", preOutageKey, rec.Code, rec.Body.Len())
	}

	// Writes continue while the replica is promoted. The client ack is from
	// the promoted backend; the new queue is the only path to replacement.
	rec = failbackDo(t, promotedHandler, http.MethodPut, "/"+h.Bucket+"/"+newKey, newOnPromoted, restoreAccessKey, restoreSecretKey)
	if rec.Code != http.StatusOK {
		t.Fatalf("promoted PUT %s: status %d: %s", newKey, rec.Code, rec.Body.String())
	}
	newStored := h.StoredKey(newKey)
	pollFor(t, WaitTimeout, "promoted write to enter replacement queue", func() bool {
		return promotedMetrics.EnqueuedTotal.Load() >= 1
	})
	newMirror := waitForBackendObject(t, replacement, h.Bucket, newStored, WaitTimeout)
	if promotedBytes := readBackendObject(t, h.Secondary.Backend, h.Bucket, newStored); !bytes.Equal(promotedBytes, newMirror) {
		t.Fatalf("new promoted write differs between promoted primary and replacement secondary")
	}

	// There is no bulk backfill. The pre-outage object reaches the replacement
	// only when it is read from the promoted deployment and re-uploaded, as
	// required by the runbook's failback procedure.
	if _, err := replacement.Head(context.Background(), h.Bucket, preStored); err == nil {
		t.Fatalf("replacement already contains pre-outage object before the documented re-upload backfill")
	}
	rec = failbackDo(t, promotedHandler, http.MethodPut, "/"+h.Bucket+"/"+preOutageKey, preOutage, restoreAccessKey, restoreSecretKey)
	if rec.Code != http.StatusOK {
		t.Fatalf("promoted re-upload %s: status %d: %s", preOutageKey, rec.Code, rec.Body.String())
	}
	pollFor(t, WaitTimeout, "pre-outage backfill to enter replacement queue", func() bool {
		return promotedMetrics.EnqueuedTotal.Load() >= 2
	})
	preMirror := waitForBackendObject(t, replacement, h.Bucket, preStored, WaitTimeout)
	if promotedBytes := readBackendObject(t, h.Secondary.Backend, h.Bucket, preStored); !bytes.Equal(promotedBytes, preMirror) {
		t.Fatalf("backfilled pre-outage object differs between promoted primary and replacement secondary")
	}
	assertFailoverMetrics(t, promotedMetrics, 2)

	// Fail back: stop the promoted deployment's replication worker only after
	// it is fully drained, then start a fresh server on the replacement. A
	// final write proves the replacement is now the active durable primary.
	promotedCancel()
	// The queue cleanup also calls Stop; this explicit call makes the handoff
	// boundary obvious and is idempotent by contract.
	promoted.StopReplicationQueue()

	final, _, _, _ := newFailoverServer(t, h, replacement, nil, failbackAccessKey, failbackSecretKey)
	finalCfgHandler := final.Handler()
	for key, want := range map[string][]byte{
		preOutageKey: preOutage,
		newKey:       newOnPromoted,
		finalKey:     finalWrite,
	} {
		if key == finalKey {
			rec = failbackDo(t, finalCfgHandler, http.MethodPut, "/"+h.Bucket+"/"+key, want, failbackAccessKey, failbackSecretKey)
			if rec.Code != http.StatusOK {
				t.Fatalf("failed-back PUT %s: status %d: %s", key, rec.Code, rec.Body.String())
			}
			rec = failbackDo(t, finalCfgHandler, http.MethodGet, "/"+h.Bucket+"/"+key, nil, failbackAccessKey, failbackSecretKey)
			if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), want) {
				t.Errorf("failed-back GET %s after final PUT: status %d, body %d bytes; want %d-byte plaintext", key, rec.Code, rec.Body.Len(), len(want))
			}
			continue
		}
		rec = failbackDo(t, finalCfgHandler, http.MethodGet, "/"+h.Bucket+"/"+key, nil, failbackAccessKey, failbackSecretKey)
		if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), want) {
			t.Errorf("failed-back GET %s: status %d, body %d bytes; want %d-byte plaintext", key, rec.Code, rec.Body.Len(), len(want))
		}
	}
	if _, err := replacement.Head(context.Background(), h.Bucket, h.StoredKey(finalKey)); err != nil {
		t.Fatalf("replacement primary is missing the final post-failback write: %v", err)
	}
}
