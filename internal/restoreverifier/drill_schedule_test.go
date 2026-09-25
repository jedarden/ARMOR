package restoreverifier

// Scheduler-level tests for ADR-004's periodic DR-drill cadence
// (Config.DRDrillInterval / VERIFIER_DR_DRILL_INTERVAL, armor-445bcb28).
//
// The per-object drill behavior is pinned by TestVerifyObject_DRDrill_* and the
// on-demand trigger by contract_test.go. What nothing pinned before is the
// production shape the fleet's manifests actually run (all four
// restore-verifier Deployments set VERIFIER_DR_DRILL_INTERVAL=24h): a Verifier
// started with DRDrillInterval > 0 must, with no /trigger call,
//
//  1. execute the direct-only drill on its own schedule and prove recovery
//     with the ARMOR read path untouched (armorGet count unchanged),
//  2. report the results — the drill_* state fields advance AND the
//     armor_drill_* per-bucket gauges are published,
//  3. leave the dual-path restorability signal exactly as the startup dual run
//     left it (a scheduled drill never bumps the continuous-verification
//     ledger), and
//  4. stop cleanly via Stop.
//
// The mirror-image pause contract is pinned too: an unset interval must mean
// "paused" — the loop keeps ticking dual runs while no scheduled drill ever
// fires (drill runs remain available only via POST /trigger?mode=dr-drill).
// That is the safe-pause lever documented in
// docs/restore-verifier-deployment-guide.md.
//
// The failure mode is pinned at the same level (armor-a166a6bc): once a drill
// has passed, stored-ciphertext damage must make the following scheduled
// drills fail into exactly the documented health signals — drill_failed
// objects, a zero drill ratio, an incremented failure counter, and a
// drill_last_success that stays stale at the last proven recovery while
// drill_last_verification keeps advancing — with the ARMOR read path still
// untouched, the dual-path ledger untouched, and NO bead filed (escalation is
// dual-path-owned), and without a retry storm: the next attempt waits for the
// next tick (a full interval after the failed run completed, never sooner),
// and the same corruption re-found by a dual run files exactly one deduped
// bead.

import (
	"bytes"
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/metrics"
)

// startScheduledDrillVerifier builds a verifier over one valid single-PUT
// SQLite backup, started with the given intervals. esc (optional) is wired as
// the escalation hook so a test can observe filings; the returned armorGet
// field reports the ARMOR read-path call count so a drill leg can prove it
// never touched that path.
func startScheduledDrillVerifier(t *testing.T, dual, drill time.Duration, m *metrics.Metrics, esc *Escalator) (*Verifier, *fakeBackend, string) {
	t.Helper()

	const (
		bucket    = "sched"
		blockSize = 4096
		key       = "backups/valid.sqlite"
	)
	// A fixed 32-byte MEK keeps the test deterministic; WrapDEK requires 32 bytes.
	mek := bytes.Repeat([]byte{0xA5}, 32)
	plaintext := fixture(t, "valid.sqlite")
	ciphertext, meta := armorEncrypt(t, mek, blockSize, plaintext)

	info := &backend.ObjectInfo{
		Key:      key,
		Size:     int64(len(plaintext)),
		Metadata: meta,
	}
	fb := &fakeBackend{
		ciphertext:  ciphertext,
		plaintext:   plaintext,
		info:        info,
		listObjects: []backend.ObjectInfo{*info},
	}

	v := New(fb, mek, nil, blockSize, nil, Config{
		Buckets:         []BucketConfig{{Bucket: bucket, Enabled: true, HistoricalSampleSize: 3}},
		Interval:        dual,
		DRDrillInterval: drill,
		SampleSize:      3,
		Metrics:         m,
		Escalator:       esc,
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go v.Start(ctx)
	t.Cleanup(v.Stop)

	return v, fb, bucket
}

// waitFor polls cond until it holds or the deadline passes, naming the
// condition in the failure message.
func waitFor(t *testing.T, desc string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", desc)
}

// seriesValue returns the value of a bucket-labeled series from a
// PrometheusFormat dump, failing if the series is absent.
func seriesValue(t *testing.T, output, series, bucket string) string {
	t.Helper()
	prefix := series + "{"
	for _, line := range strings.Split(output, "\n") {
		if !strings.HasPrefix(line, prefix) || !strings.Contains(line, `bucket="`+bucket+`"`) {
			continue
		}
		_, value, ok := strings.Cut(line, "} ")
		if !ok {
			t.Fatalf("malformed series line %q", line)
		}
		return value
	}
	t.Fatalf("series %s{bucket=%q} absent from metrics output:\n%s", series, bucket, output)
	return ""
}

// TestStartScheduledDrillExecutesAndReports is the scheduler-level acceptance
// test for the production cadence: the ticker-driven drill must run, recover
// the backup direct-only, and report through both the /status state and the
// armor_drill_* gauges — without disturbing the dual-path signal.
func TestStartScheduledDrillExecutesAndReports(t *testing.T) {
	m := metrics.NewMetrics()
	// The 500ms drill cadence fires well inside waitFor's deadline while
	// spacing the NEXT tick far beyond the assertion window below, so the
	// assertions observe exactly one completed drill.
	v, fb, bucket := startScheduledDrillVerifier(t, time.Hour, 500*time.Millisecond, m, nil)

	// The startup dual run completes before the loop's first select, so a
	// non-zero LastVerification is a stable snapshot point: everything the
	// dual run did (including its ARMOR read-path calls) happened-before this
	// observation.
	waitFor(t, "the startup dual-path run to complete", func() bool {
		st, err := v.GetBucketStatus(bucket)
		return err == nil && st != nil && !st.LastVerification.IsZero()
	})
	before, err := v.GetBucketStatus(bucket)
	if err != nil {
		t.Fatalf("GetBucketStatus: %v", err)
	}
	armorGetBeforeDrill := fb.armorGet

	// The scheduled drill fires one full DRDrillInterval after start with no
	// /trigger call (armor-851dca86: deliberately NOT chained behind the
	// startup dual run).
	waitFor(t, "the scheduled DR drill to run", func() bool {
		st, err := v.GetBucketStatus(bucket)
		return err == nil && st != nil && !st.DrillLastVerification.IsZero()
	})
	after, err := v.GetBucketStatus(bucket)
	if err != nil {
		t.Fatalf("GetBucketStatus: %v", err)
	}

	// (1) The drill executed and recovered the listed backup direct-only: the
	// sample is latest + 1 historical slot (the same single object), so both
	// slots must have passed, and the ARMOR read path must have been called
	// zero additional times — the "ARMOR server is gone" property, now proven
	// for the scheduled form, not just the on-demand form.
	if after.DrillTotalObjects != 2 || after.DrillVerifiedObjects != 2 {
		t.Fatalf("drill reported %d/%d recovered, want 2/2 (latest + sampled slot)",
			after.DrillVerifiedObjects, after.DrillTotalObjects)
	}
	if after.DrillLastSuccess.IsZero() {
		t.Fatal("DrillLastSuccess is zero; a passing scheduled drill must record a direct-only recovery")
	}
	if fb.armorGet != armorGetBeforeDrill {
		t.Fatalf("scheduled drill invoked the ARMOR read path %d time(s); the direct-only drill must never call Get",
			fb.armorGet-armorGetBeforeDrill)
	}

	// (2) The results are reported on the armor_drill_* gauges.
	output := m.PrometheusFormat()
	for name, want := range map[string]string{
		"armor_drill_verified_object_ratio": "1",
		"armor_drill_failures_total":        "0",
	} {
		if got := seriesValue(t, output, name, bucket); got != want {
			t.Errorf("%s{bucket=%q} = %s, want %s", name, bucket, got, want)
		}
	}
	for _, name := range []string{"armor_drill_last_verified_timestamp", "armor_drill_last_success_timestamp"} {
		got, err := strconv.ParseInt(seriesValue(t, output, name, bucket), 10, 64)
		if err != nil {
			t.Fatalf("%s value %q is not an integer", name, seriesValue(t, output, name, bucket))
		}
		if got == 0 {
			t.Errorf("%s{bucket=%q} = 0; a completed scheduled drill must advance it", name, bucket)
		}
	}

	// (3) Drill isolation: the dual-path ledger is exactly what the startup
	// dual run left — the scheduled drill must not bump it (the documented
	// contract behind the separate drill_* fields).
	if !after.LastVerification.Equal(before.LastVerification) {
		t.Errorf("dual LastVerification advanced during the drill: %v -> %v",
			before.LastVerification, after.LastVerification)
	}
	if !after.LastSuccess.Equal(before.LastSuccess) {
		t.Errorf("dual LastSuccess advanced during the drill: %v -> %v",
			before.LastSuccess, after.LastSuccess)
	}
	if after.VerifiedObjectRatio != before.VerifiedObjectRatio {
		t.Errorf("dual VerifiedObjectRatio changed during the drill: %v -> %v",
			before.VerifiedObjectRatio, after.VerifiedObjectRatio)
	}
	if after.VerifiedObjects != before.VerifiedObjects || after.FailedObjects != before.FailedObjects {
		t.Errorf("dual cumulative counters changed during the drill: verified %d -> %d, failed %d -> %d",
			before.VerifiedObjects, after.VerifiedObjects, before.FailedObjects, after.FailedObjects)
	}
	if after.TotalObjects != before.TotalObjects {
		t.Errorf("dual TotalObjects changed during the drill: %d -> %d",
			before.TotalObjects, after.TotalObjects)
	}
}

// TestStartWithoutDrillIntervalLeavesDrillPaused pins the pause semantics the
// deployment guide documents: with DRDrillInterval unset (the fleet-wide pause
// lever — set VERIFIER_DR_DRILL_INTERVAL=0 in declarative-config), the loop
// keeps ticking dual-path runs while NO scheduled drill ever fires. The pause
// is proven against a demonstrably-live loop — three distinct dual runs — not
// against a wall-clock sleep; on-demand drills via POST /trigger?mode=dr-drill
// remain the only way a drill runs while paused (contract_test.go covers that
// path).
func TestStartWithoutDrillIntervalLeavesDrillPaused(t *testing.T) {
	v, _, bucket := startScheduledDrillVerifier(t, 30*time.Millisecond, 0, nil, nil)

	observed := make(map[int64]bool)
	waitFor(t, "three distinct dual-path runs (a live loop)", func() bool {
		st, err := v.GetBucketStatus(bucket)
		if err != nil || st == nil {
			return false
		}
		if !st.LastVerification.IsZero() {
			observed[st.LastVerification.UnixNano()] = true
		}
		return len(observed) >= 3
	})

	st, err := v.GetBucketStatus(bucket)
	if err != nil {
		t.Fatalf("GetBucketStatus: %v", err)
	}
	if !st.DrillLastVerification.IsZero() {
		t.Fatalf("a scheduled drill ran with DRDrillInterval unset (DrillLastVerification=%v); paused must mean no scheduled drill", st.DrillLastVerification)
	}
	if st.DrillTotalObjects != 0 || st.DrillVerifiedObjects != 0 || st.DrillFailedObjects != 0 || !st.DrillLastSuccess.IsZero() {
		t.Fatalf("drill state advanced while paused: total=%d verified=%d failed=%d success=%v",
			st.DrillTotalObjects, st.DrillVerifiedObjects, st.DrillFailedObjects, st.DrillLastSuccess)
	}
}

// TestStartScheduledDrillFailureSignalsWithoutRetryStorm is the scheduler-level
// failure-mode acceptance test (armor-a166a6bc). After a passing drill, stored
// ciphertext damage must make the following scheduled drills fail into exactly
// the documented health signals — drill_failed objects advance, the drill ratio
// gauge drops to 0, the failure counter advances once per run, drill_last_
// success stays stale at the last proven recovery while drill_last_verification
// keeps advancing, and the ARMOR read path and the dual-path ledger stay
// untouched — without a retry storm: the next attempt waits for the next tick
// (a full interval after the failed run completed, never sooner) and files no
// bead (escalation is dual-path-owned), while the same corruption re-found by a
// dual run files exactly one deduped bead, never a filing storm.
func TestStartScheduledDrillFailureSignalsWithoutRetryStorm(t *testing.T) {
	m := metrics.NewMetrics()
	rf := &recordingFiler{}
	// A one-hour freshness window keeps the (dual-path-owned) staleness filing
	// out of the picture: every filing observed below is a verification-failure
	// filing, attributable to a specific run.
	esc := NewEscalator(EscalatorConfig{Filer: rf, FreshnessWindow: time.Hour})
	const drillEvery = 500 * time.Millisecond
	v, fb, bucket := startScheduledDrillVerifier(t, time.Hour, drillEvery, m, esc)

	// Healthy baseline: the startup dual run passes both restore paths...
	waitFor(t, "the startup dual-path run to complete", func() bool {
		st, err := v.GetBucketStatus(bucket)
		return err == nil && st != nil && !st.LastVerification.IsZero()
	})
	if n := rf.count(); n != 0 {
		t.Fatalf("healthy startup recorded %d escalation filing(s), want 0", n)
	}

	// ...and the first scheduled drill recovers direct-only.
	waitFor(t, "the first scheduled DR drill to succeed", func() bool {
		st, err := v.GetBucketStatus(bucket)
		return err == nil && st != nil && !st.DrillLastSuccess.IsZero()
	})
	healthy, err := v.GetBucketStatus(bucket)
	if err != nil {
		t.Fatalf("GetBucketStatus: %v", err)
	}
	dualBefore := dualLedger(t, v, bucket)

	// Damage what the direct path reads. The flag is atomic, so storing it
	// from this goroutine while verifier runs read in GetRange is race-free;
	// the next drill tick is the first reader.
	fb.corrupt.Store(true)
	armorGetBeforeFailure := fb.armorGet

	// The next scheduled drill must fail into the documented health signals —
	// not silently pass, and not touch anything the drill does not own.
	waitFor(t, "the damaged ciphertext to fail a scheduled drill", func() bool {
		st, err := v.GetBucketStatus(bucket)
		return err == nil && st != nil && st.DrillFailedObjects > 0
	})
	failed1, err := v.GetBucketStatus(bucket)
	if err != nil {
		t.Fatalf("GetBucketStatus: %v", err)
	}

	// State: both sample slots (latest + the one historical slot — the same
	// object listed twice) failed, the cumulative verified count did not grow,
	// and the attempt advanced while the last PROVEN recovery stayed stale.
	if failed1.DrillTotalObjects != 2 || failed1.DrillFailedObjects != 2 {
		t.Errorf("failed drill reported total=%d failed=%d, want 2/2 slots failed",
			failed1.DrillTotalObjects, failed1.DrillFailedObjects)
	}
	if failed1.DrillVerifiedObjects != healthy.DrillVerifiedObjects {
		t.Errorf("drill verified count grew on a failed run: %d -> %d",
			healthy.DrillVerifiedObjects, failed1.DrillVerifiedObjects)
	}
	if !failed1.DrillLastSuccess.Equal(healthy.DrillLastSuccess) {
		t.Errorf("drill_last_success moved on a failed run: %v -> %v; it must stay stale at the last proven recovery",
			healthy.DrillLastSuccess, failed1.DrillLastSuccess)
	}
	if !failed1.DrillLastVerification.After(healthy.DrillLastVerification) {
		t.Errorf("failed drill did not advance drill_last_verification: %v", failed1.DrillLastVerification)
	}

	// Gauges: failure counter up once, ratio at zero, success timestamp stale,
	// attempt timestamp published.
	output := m.PrometheusFormat()
	if got, want := seriesValue(t, output, "armor_drill_failures_total", bucket), "2"; got != want {
		t.Errorf("armor_drill_failures_total = %s, want %s (one failed run, two slots)", got, want)
	}
	if got := seriesValue(t, output, "armor_drill_verified_object_ratio", bucket); got != "0" {
		t.Errorf("armor_drill_verified_object_ratio = %s after a failed drill, want 0", got)
	}
	lastSuccessTs, err := strconv.ParseInt(seriesValue(t, output, "armor_drill_last_success_timestamp", bucket), 10, 64)
	if err != nil {
		t.Fatalf("armor_drill_last_success_timestamp is not an integer: %q", seriesValue(t, output, "armor_drill_last_success_timestamp", bucket))
	}
	if lastSuccessTs != healthy.DrillLastSuccess.Unix() {
		t.Errorf("armor_drill_last_success_timestamp = %d, want %d (stale at the last proven recovery)",
			lastSuccessTs, healthy.DrillLastSuccess.Unix())
	}
	if lastVerifiedTs, err := strconv.ParseInt(seriesValue(t, output, "armor_drill_last_verified_timestamp", bucket), 10, 64); err != nil || lastVerifiedTs == 0 {
		t.Errorf("armor_drill_last_verified_timestamp = %d (err %v); a failed attempt must still publish it", lastVerifiedTs, err)
	}

	// The failed drill still never touched the ARMOR read path: damage on the
	// direct path must not fall back to it — that would defeat the drill.
	if fb.armorGet != armorGetBeforeFailure {
		t.Errorf("failed drill invoked the ARMOR read path %d time(s); a drill must never fall back to Get",
			fb.armorGet-armorGetBeforeFailure)
	}
	// ...and the dual-path ledger is exactly what the startup run left.
	assertDualUntouched(t, dualBefore, v, bucket)

	// Escalation stays dual-path-owned: the drill failure files no bead.
	if n := rf.count(); n != 0 {
		t.Fatalf("failed drill filed %d escalation bead(s); drill failures surface through gauges and logs only", n)
	}

	// No retry storm: over the measured window the drill may complete at most
	// one run per tick — a rate bound on runs per elapsed time, not a
	// wall-clock alignment assumption (a run ending just before a tick
	// boundary legitimately meets the next tick a moment later). Each failed
	// run adds exactly its two slot failures, so runs = failures/2.
	stormStart := time.Now()
	base := failed1.DrillFailedObjects
	var failed2 *BucketState
	deadline := stormStart.Add(4 * drillEvery)
	for {
		st, err := v.GetBucketStatus(bucket)
		if err != nil {
			t.Fatalf("GetBucketStatus: %v", err)
		}
		if st.DrillFailedObjects-base >= 4 || time.Now().After(deadline) {
			failed2 = st
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	elapsed := time.Since(stormStart)
	newRuns := (failed2.DrillFailedObjects - base) / 2
	if newRuns < 2 {
		t.Fatalf("drill cadence stopped after a failure: %d run(s) in the window, want >= 2 — a failed drill must keep its schedule", newRuns)
	}
	if maxRuns := int64(elapsed/drillEvery) + 1; newRuns > maxRuns {
		t.Errorf("%d drill runs completed in %v (bound %d at one run per %v tick): a failed drill retried — retry storm",
			newRuns, elapsed.Round(time.Millisecond), maxRuns, drillEvery)
	}
	if got := seriesValue(t, m.PrometheusFormat(), "armor_drill_failures_total", bucket); got != strconv.FormatInt(failed2.DrillFailedObjects, 10) {
		t.Errorf("armor_drill_failures_total = %s, want %d (the state counter and its gauge must agree)", got, failed2.DrillFailedObjects)
	}
	if n := rf.count(); n != 0 {
		t.Fatalf("repeated drill failures filed %d escalation bead(s), want 0", n)
	}

	// The mirror image: the SAME corruption re-found by a dual run IS filed —
	// one bead for the distinct active failure (both sample slots dedupe to a
	// single key) — proving the filer above was live, not inert. The dual run
	// exercises the ARMOR read path first, and its results name that path —
	// every drill result, healthy or failed, stays PathDirect. Damaged bytes
	// fail the dual run's ARMOR leg (the envelope is corrupt regardless of
	// which path fetches it), which is the failure class that files.
	armorPathResults := func() int {
		st, err := v.GetBucketStatus(bucket)
		if err != nil {
			t.Fatalf("GetBucketStatus: %v", err)
		}
		n := 0
		for _, r := range st.RecentResults {
			if r.Path == PathARMOR {
				n++
			}
		}
		return n
	}
	v.runVerification(context.Background())
	if n := rf.count(); n != 1 {
		t.Fatalf("dual run re-finding the same corruption filed %d bead(s), want exactly 1 (one per distinct failure)", n)
	}
	if got := armorPathResults(); got != 2 {
		t.Errorf("triggered dual run recorded %d armor-path results, want 2 (both sample slots exercised the ARMOR read path)", got)
	}
	// A second dual run on the same failure is deduped too — the filing rate
	// is bounded by distinct failures, never by the number of ticks.
	v.runVerification(context.Background())
	if n := rf.count(); n != 1 {
		t.Fatalf("second dual run filed again (%d beads total); dedupe must cap one distinct failure at one bead", n)
	}
	if got := armorPathResults(); got != 4 {
		t.Errorf("armor-path results = %d after the second dual run, want 4 (two slots per run)", got)
	}
	// The dual runs left the drill ledger's success evidence exactly as the
	// drills left it. (DrillFailedObjects and DrillLastVerification are
	// deliberately not compared: another scheduled drill tick may legitimately
	// have advanced them while these assertions ran — but nothing, dual runs
	// included, may advance the proven-recovery evidence.)
	failed3, err := v.GetBucketStatus(bucket)
	if err != nil {
		t.Fatalf("GetBucketStatus: %v", err)
	}
	if failed3.DrillVerifiedObjects != failed2.DrillVerifiedObjects || failed3.DrillTotalObjects != failed2.DrillTotalObjects ||
		!failed3.DrillLastSuccess.Equal(failed2.DrillLastSuccess) {
		t.Errorf("dual runs advanced drill success evidence: total %d -> %d, verified %d -> %d, success %v -> %v",
			failed2.DrillTotalObjects, failed3.DrillTotalObjects,
			failed2.DrillVerifiedObjects, failed3.DrillVerifiedObjects,
			failed2.DrillLastSuccess, failed3.DrillLastSuccess)
	}
}

// dualLedger snapshots the dual-path restorability fields the drill must never
// touch.
func dualLedger(t *testing.T, v *Verifier, bucket string) *BucketState {
	t.Helper()
	st, err := v.GetBucketStatus(bucket)
	if err != nil {
		t.Fatalf("GetBucketStatus: %v", err)
	}
	return st
}

// assertDualUntouched fails the test when any dual-path ledger field moved
// after the snapshot.
func assertDualUntouched(t *testing.T, before *BucketState, v *Verifier, bucket string) {
	t.Helper()
	after, err := v.GetBucketStatus(bucket)
	if err != nil {
		t.Fatalf("GetBucketStatus: %v", err)
	}
	if !after.LastVerification.Equal(before.LastVerification) {
		t.Errorf("dual LastVerification advanced: %v -> %v", before.LastVerification, after.LastVerification)
	}
	if !after.LastSuccess.Equal(before.LastSuccess) {
		t.Errorf("dual LastSuccess advanced: %v -> %v", before.LastSuccess, after.LastSuccess)
	}
	if after.VerifiedObjectRatio != before.VerifiedObjectRatio {
		t.Errorf("dual VerifiedObjectRatio changed: %v -> %v", before.VerifiedObjectRatio, after.VerifiedObjectRatio)
	}
	if after.VerifiedObjects != before.VerifiedObjects || after.FailedObjects != before.FailedObjects {
		t.Errorf("dual cumulative counters changed: verified %d -> %d, failed %d -> %d",
			before.VerifiedObjects, after.VerifiedObjects, before.FailedObjects, after.FailedObjects)
	}
	if after.TotalObjects != before.TotalObjects {
		t.Errorf("dual TotalObjects changed: %d -> %d", before.TotalObjects, after.TotalObjects)
	}
}
