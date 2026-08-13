package metrics

import (
	"strings"
	"testing"
)

// TestReplicationEnqueuedPrometheusExport verifies that IncReplicationEnqueued
// correctly exports metrics in Prometheus format with operation labels.
func TestReplicationEnqueuedPrometheusExport(t *testing.T) {
	m := NewMetrics()

	// Increment metrics for both operations
	m.IncReplicationEnqueued("put")
	m.IncReplicationEnqueued("put")
	m.IncReplicationEnqueued("put-streaming")

	// Get Prometheus format output
	output := m.PrometheusFormat()

	// Verify the output contains the expected sections
	requiredSections := []string{
		"# HELP armor_replication_enqueued_total Total number of items enqueued for replication by operation",
		"# TYPE armor_replication_enqueued_total counter",
		`armor_replication_enqueued_total{operation="put"} 2`,
		`armor_replication_enqueued_total{operation="put-streaming"} 1`,
	}

	for _, section := range requiredSections {
		if !strings.Contains(output, section) {
			t.Errorf("Prometheus export missing required section:\nwant: %s\ngot:\n%s", section, output)
		}
	}

	// Verify metric type is "counter"
	if !strings.Contains(output, "# TYPE armor_replication_enqueued_total counter") {
		t.Error("Metric type is not 'counter'")
	}

	// Verify operation labels are present
	if !strings.Contains(output, `operation="put"`) {
		t.Error("Missing 'put' operation label")
	}
	if !strings.Contains(output, `operation="put-streaming"`) {
		t.Error("Missing 'put-streaming' operation label")
	}
}
