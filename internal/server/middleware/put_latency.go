package middleware

import "context"

// PutLatencyKey holds the context key for the PUT-latency split.
const PutLatencyKey contextKey = "putLatency"

// PutLatency carries the PUT-latency split for one completed request
// (armor-69dd394b): the upstream backend object PUT, the time the request
// spent waiting for the provenance append mutex, and the provenance chain
// work done while holding it. PUT handlers publish it into the request
// context after a successful upload so logCompletedRequest can add
// backend_put_ms, provenance_lock_wait_ms and provenance_write_ms to the
// request-completed log line. It is absent when the request did not take the
// provenance-recording path — provenance not wired, or ShouldRecord false —
// so the three fields are absent from the log line in exactly those cases.
type PutLatency struct {
	BackendPutMs         int64
	ProvenanceLockWaitMs int64
	ProvenanceWriteMs    int64
}

// WithPutLatency returns a context carrying pl.
func WithPutLatency(ctx context.Context, pl *PutLatency) context.Context {
	return context.WithValue(ctx, PutLatencyKey, pl)
}

// GetPutLatency extracts the PUT-latency split from the request context,
// or nil when the request did not take the provenance-recording path.
func GetPutLatency(ctx context.Context) *PutLatency {
	pl, _ := ctx.Value(PutLatencyKey).(*PutLatency)
	return pl
}
