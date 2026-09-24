package provenance

import (
	"context"
	"time"
)

// Timings carries the provenance-latency split for one chain operation back to
// the caller (armor-69dd394b). A caller attaches a zero Timings to the context
// passed to RecordUpload or CreateChainEntry via WithTimings; the manager
// fills in how long the call spent waiting for the append mutex and how long
// the work performed while holding it took. The split exists so the
// request-completed log and the armor_put_*_ms histograms can attribute PUT
// latency to provenance lock queueing versus the upstream object PUT.
type Timings struct {
	// LockWait is the time spent blocked acquiring appendMu.
	LockWait time.Duration

	// Write is the duration of the work done while holding appendMu: the
	// chain-entry and chain-head B2 writes on the RecordUpload path, and the
	// (cached in the steady state) head read plus in-memory head update on the
	// CreateChainEntry path.
	Write time.Duration
}

// timingsKey is the context key under which a *Timings travels.
type timingsKey struct{}

// WithTimings returns a context that carries t. The manager records into t
// only when one is present; without it the chain operations behave exactly as
// before and no timing work is done beyond two clock reads.
func WithTimings(ctx context.Context, t *Timings) context.Context {
	return context.WithValue(ctx, timingsKey{}, t)
}

// timingsFrom returns the Timings carried by ctx, or nil.
func timingsFrom(ctx context.Context) *Timings {
	t, _ := ctx.Value(timingsKey{}).(*Timings)
	return t
}
