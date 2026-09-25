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

// scheduledDrillFixture builds a verifier over one valid single-PUT SQLite
// backup, started with the given intervals. The returned armorGetSnaphot
// function reports the ARMOR read-path call count so a drill leg can prove it
// never touched that path.
func startScheduledDrillVerifier(t *testing.T, dual, drill time.Duration, m *metrics.Metrics) (*Verifier, *fakeBackend, string) {
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
	v, fb, bucket := startScheduledDrillVerifier(t, time.Hour, 500*time.Millisecond, m)

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
	v, _, bucket := startScheduledDrillVerifier(t, 30*time.Millisecond, 0, nil)

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
