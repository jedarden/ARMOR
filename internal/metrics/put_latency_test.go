package metrics

import (
	"strings"
	"testing"
	"time"
)

// TestRecordPutLatencyPrometheusFormat verifies the armor-69dd394b PUT
// latency split histograms: observations accumulate into sum/count/cumulative
// buckets and PrometheusFormat emits the three armor_put_*_ms series.
func TestRecordPutLatencyPrometheusFormat(t *testing.T) {
	m := NewMetrics()

	m.RecordPutLatency("put", 800*time.Millisecond, 12*time.Millisecond, 45*time.Millisecond)
	m.RecordPutLatency("put", 100*time.Millisecond, 1*time.Millisecond, 5*time.Millisecond)
	m.RecordPutLatency("put-streaming", 1500*time.Millisecond, 0, 0)

	out := m.PrometheusFormat()

	required := []string{
		"# TYPE armor_put_backend_ms histogram",
		"armor_put_backend_ms_sum{operation=\"put\"} 900",
		"armor_put_backend_ms_count{operation=\"put\"} 2",
		`armor_put_backend_ms_bucket{operation="put",le="1000"} 2`,
		`armor_put_backend_ms_bucket{operation="put",le="+Inf"} 2`,

		"# TYPE armor_put_provenance_lock_wait_ms histogram",
		"armor_put_provenance_lock_wait_ms_sum{operation=\"put\"} 13",
		"armor_put_provenance_lock_wait_ms_count{operation=\"put\"} 2",
		`armor_put_provenance_lock_wait_ms_bucket{operation="put",le="1"} 1`,
		`armor_put_provenance_lock_wait_ms_bucket{operation="put",le="25"} 2`,

		"# TYPE armor_put_provenance_write_ms histogram",
		"armor_put_provenance_write_ms_sum{operation=\"put\"} 50",
		"armor_put_provenance_write_ms_count{operation=\"put\"} 2",
		`armor_put_provenance_write_ms_bucket{operation="put",le="50"} 2`,

		// Streaming observations land under their own operation label
		"armor_put_backend_ms_sum{operation=\"put-streaming\"} 1500",
		"armor_put_backend_ms_count{operation=\"put-streaming\"} 1",
	}
	for _, want := range required {
		if !strings.Contains(out, want) {
			t.Errorf("PrometheusFormat missing %q\nGot:\n%s", want, out)
		}
	}
}
