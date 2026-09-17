package restoreverifier

import (
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/metrics"
)

func TestRecordBucketRunPublishesPerBucketLabels(t *testing.T) {
	m := metrics.NewMetrics()
	v := &Verifier{metrics: m}
	state := &BucketState{
		Bucket:       "armor-apexalgo",
		LastSuccess:  time.Unix(1_750_000_000, 0),
		TotalObjects: 4,
	}

	v.recordBucketRun(state.Bucket, state, 4)
	output := m.PrometheusFormat()
	for _, want := range []string{
		`armor_last_verified_restore_timestamp{bucket="armor-apexalgo"} 1750000000`,
		`armor_verified_object_ratio{bucket="armor-apexalgo"} 1`,
		`armor_restore_verification_failures_total{bucket="armor-apexalgo"} 0`,
	} {
		if !strings.Contains(output, want) {
			t.Errorf("metrics output missing %q:\n%s", want, output)
		}
	}
}

func TestRecordBucketRunMarksUnsuccessfulRestoreAsStale(t *testing.T) {
	m := metrics.NewMetrics()
	v := &Verifier{metrics: m}
	state := &BucketState{
		Bucket:           "iad-kalshi",
		LastVerification: time.Now(),
		TotalObjects:     2,
		FailedObjects:    2,
	}

	// The verifier attempted a run, but no object passed. The stale clock must
	// remain at zero rather than being refreshed by the failed attempt.
	v.recordBucketRun(state.Bucket, state, 0)
	output := m.PrometheusFormat()
	for _, want := range []string{
		`armor_last_verified_restore_timestamp{bucket="iad-kalshi"} 0`,
		`armor_verified_object_ratio{bucket="iad-kalshi"} 0`,
		`armor_restore_verification_failures_total{bucket="iad-kalshi"} 2`,
	} {
		if !strings.Contains(output, want) {
			t.Errorf("failed restore did not publish %q:\n%s", want, output)
		}
	}
}

func TestRecordBucketRunPublishesPartialVerificationFailure(t *testing.T) {
	m := metrics.NewMetrics()
	v := &Verifier{metrics: m}
	state := &BucketState{
		Bucket:        "rs-manager",
		LastSuccess:   time.Unix(1_750_001_000, 0),
		TotalObjects:  4,
		FailedObjects: 1,
	}

	v.recordBucketRun(state.Bucket, state, 3)
	output := m.PrometheusFormat()
	for _, want := range []string{
		`armor_last_verified_restore_timestamp{bucket="rs-manager"} 1750001000`,
		`armor_verified_object_ratio{bucket="rs-manager"} 0.75`,
		`armor_restore_verification_failures_total{bucket="rs-manager"} 1`,
	} {
		if !strings.Contains(output, want) {
			t.Errorf("partial verification did not publish %q:\n%s", want, output)
		}
	}
}
