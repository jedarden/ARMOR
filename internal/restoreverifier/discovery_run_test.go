package restoreverifier

// Tests for the armor-851dca86 discovery and run-loop rework: discovery is ONE
// shared walk per run (latest object + reservoir sample together), a DR drill
// reuses the most recent dual enumeration instead of re-walking, every run is
// bounded by a per-run deadline, and a queued ticker tick no longer chains
// runs back-to-back.

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
)

// TestEnumerateCandidatesSingleWalkLatestAndSample proves the shared walk
// computes both discovery outputs in ONE pagination of the bucket: the latest
// object is picked by LastModified (not by page position), the reservoir comes
// back at the requested size, and the List call count equals a single walk.
// The two-walk shape this replaced issued one 100-key-page walk for latest plus
// one 1000-key-page walk for the sample — on very large buckets that doubling
// was the whole run (armor-851dca86).
func TestEnumerateCandidatesSingleWalkLatestAndSample(t *testing.T) {
	const (
		n          = 2500
		pageSize   = 100
		sampleSize = 10
	)
	base := time.Now().Add(-24 * time.Hour)
	objects := make([]backend.ObjectInfo, n)
	for i := range objects {
		objects[i] = backend.ObjectInfo{
			Key:          fmt.Sprintf("backup-%06d.db", i),
			LastModified: base.Add(time.Duration(i) * time.Minute),
		}
	}
	// The newest object sits mid-bucket: the latest pick must be by
	// LastModified, not the last key the walk saw.
	objects[700].LastModified = base.Add(72 * time.Hour)

	mb := &paginatingBackend{objects: objects, pageSize: pageSize}
	v := New(mb, bytes.Repeat([]byte{0xA5}, 32), nil, 4096, nil, Config{})

	latest, ok, sample, err := v.enumerateCandidates(context.Background(), "bucket", sampleSize)
	if err != nil {
		t.Fatalf("enumerateCandidates: %v", err)
	}
	if !ok {
		t.Fatal("enumerateCandidates found no candidates")
	}
	if want := "backup-000700.db"; latest.Key != want {
		t.Fatalf("latest = %q, want %q (newest by LastModified, not walk end)", latest.Key, want)
	}
	if len(sample) != sampleSize {
		t.Fatalf("sample size = %d, want %d", len(sample), sampleSize)
	}
	if wantPages := (n + pageSize - 1) / pageSize; mb.listCalls != wantPages {
		t.Fatalf("shared walk issued %d List calls, want %d (one full walk at the fake's page size)",
			mb.listCalls, wantPages)
	}
}

// TestSampleForRunDrillReusesDualEnumeration proves a drill consumes the cached
// dual enumeration (zero additional List calls) while a drill with no cached
// walk enumerates for itself.
func TestSampleForRunDrillReusesDualEnumeration(t *testing.T) {
	const (
		n        = 300
		pageSize = 50
	)
	base := time.Now().Add(-24 * time.Hour)
	objects := make([]backend.ObjectInfo, n)
	for i := range objects {
		objects[i] = backend.ObjectInfo{
			Key:          fmt.Sprintf("backup-%06d.db", i),
			LastModified: base.Add(time.Duration(i) * time.Minute),
		}
	}
	mb := &paginatingBackend{objects: objects, pageSize: pageSize}
	v := New(mb, bytes.Repeat([]byte{0xA5}, 32), nil, 4096, nil, Config{})
	state := &BucketState{Bucket: "bucket", HistoricalSampleSize: 5}

	dualLatest, dualSample, err := v.sampleForRun(context.Background(), "bucket", state, false)
	if err != nil {
		t.Fatalf("dual sampleForRun: %v", err)
	}
	wantCalls := (n + pageSize - 1) / pageSize
	if mb.listCalls != wantCalls {
		t.Fatalf("dual run enumerated with %d List calls, want %d", mb.listCalls, wantCalls)
	}

	drillLatest, drillSample, err := v.sampleForRun(context.Background(), "bucket", state, true)
	if err != nil {
		t.Fatalf("drill sampleForRun: %v", err)
	}
	if mb.listCalls != wantCalls {
		t.Fatalf("drill re-enumerated: %d List calls after drill (want unchanged %d) — the drill must reuse the dual walk",
			mb.listCalls, wantCalls)
	}
	if drillLatest.Key != dualLatest.Key {
		t.Fatalf("drill latest = %q, want the cached dual latest %q", drillLatest.Key, dualLatest.Key)
	}
	if got, want := sampleKeySet(drillSample), sampleKeySet(dualSample); !reflect.DeepEqual(got, want) {
		t.Fatalf("drill sample keys = %v, want the cached dual sample keys %v", got, want)
	}

	// A drill with no completed prior walk (cold cache) must still enumerate.
	coldState := &BucketState{Bucket: "bucket", HistoricalSampleSize: 5}
	if _, _, err := v.sampleForRun(context.Background(), "bucket", coldState, true); err != nil {
		t.Fatalf("cold-cache drill sampleForRun: %v", err)
	}
	if mb.listCalls == wantCalls {
		t.Fatal("cold-cache drill issued no List calls; it must fall back to enumerating")
	}
}

// wedgedBackend models the failure mode behind armor-851dca86: a backend List
// that never returns (a slow/hung backend region with no deadline of its own).
type wedgedBackend struct {
	backend.Backend
}

func (w *wedgedBackend) List(ctx context.Context, _, _, _, _ string, _ int) (*backend.ListResult, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

// TestRunVerificationDeadlineBoundsWedgedRun proves the per-run deadline turns
// a wedged discovery walk into a promptly-failing run: runVerification returns
// within the deadline (not never), and the bucket records a failed
// enumeration so the restore-age gauge and failure alert still advance.
func TestRunVerificationDeadlineBoundsWedgedRun(t *testing.T) {
	v := New(&wedgedBackend{}, bytes.Repeat([]byte{0xA5}, 32), nil, 4096, nil, Config{
		Buckets:    []BucketConfig{{Bucket: "wedged", Enabled: true, HistoricalSampleSize: 5}},
		RunTimeout: 25 * time.Millisecond,
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		v.runVerification(context.Background())
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("runVerification did not return within 5s; the per-run deadline did not bound the wedged walk")
	}

	status := v.GetStatus()["wedged"]
	if status == nil {
		t.Fatal("no state recorded for bucket wedged")
	}
	if status.FailedObjects != 1 {
		t.Fatalf("FailedObjects = %d, want 1 (a deadline-killed run must record a failed enumeration)",
			status.FailedObjects)
	}
	if status.LastVerification.IsZero() {
		t.Fatal("LastVerification not set; a deadline-killed run must still advance the restore-age gauge")
	}
}

// TestNewRunTimeoutNormalization proves an unset (or negative) RunTimeout
// selects DefaultRunTimeout rather than an unbounded run.
func TestNewRunTimeoutNormalization(t *testing.T) {
	mek := bytes.Repeat([]byte{0xA5}, 32)
	if v := New(&wedgedBackend{}, mek, nil, 4096, nil, Config{}); v.runTimeout != DefaultRunTimeout {
		t.Fatalf("unset RunTimeout = %v, want DefaultRunTimeout (%v)", v.runTimeout, DefaultRunTimeout)
	}
	if v := New(&wedgedBackend{}, mek, nil, 4096, nil, Config{RunTimeout: -time.Second}); v.runTimeout != DefaultRunTimeout {
		t.Fatalf("negative RunTimeout = %v, want DefaultRunTimeout (%v)", v.runTimeout, DefaultRunTimeout)
	}
	if v := New(&wedgedBackend{}, mek, nil, 4096, nil, Config{RunTimeout: time.Hour}); v.runTimeout != time.Hour {
		t.Fatalf("explicit RunTimeout = %v, want 1h", v.runTimeout)
	}
}

// TestDrainTickerDropsQueuedTick proves drainTicker empties the one buffered
// tick a run longer than the interval leaves behind, so the loop cannot chain
// into the next run the instant the previous one ends.
func TestDrainTickerDropsQueuedTick(t *testing.T) {
	tk := time.NewTicker(50 * time.Millisecond)
	defer tk.Stop()

	// Let one tick queue, as happens while a run is executing.
	time.Sleep(80 * time.Millisecond)
	drainTicker(tk)

	select {
	case <-tk.C:
		t.Fatal("drainTicker left a queued tick; the loop would chain into the next run immediately")
	default:
	}
}
