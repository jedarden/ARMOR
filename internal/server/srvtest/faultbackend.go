// Package srvtest provides the server-level dual-backend (ADR-006) test
// harness. It constructs the real internal/server Server — the same
// authenticated S3 mux the live binary serves — wired to a real filesystem
// primary backend, a fault-injectable secondary backend wrapper, and the real
// internal/replication.ReplicationQueue, so tests can assert that objects the
// primary acknowledged actually LAND on the secondary after the queue drains.
//
// The harness lives in this committed shared package (not per-test) so later
// ADR-006 split children can import it instead of re-wiring servers or
// reimplementing backends.
package srvtest

import (
	"context"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jedarden/armor/internal/backend"
)

// FaultBackend wraps a real secondary backend and can inject faults into the
// replication write path: transient errors (the queue retries them with
// exponential backoff), slow writes (proves the primary ack is not blocked),
// or both at once. Every other operation is delegated to the wrapped backend
// unchanged via embedding, so the wrapper always mirrors real secondary
// behavior for reads, listings, and multipart operations.
type FaultBackend struct {
	backend.Backend

	mu       sync.Mutex
	putErr   error
	putDelay time.Duration

	putCalls atomic.Int64
}

// NewFaultBackend wraps a real backend with fault-injection controls.
func NewFaultBackend(wrapped backend.Backend) *FaultBackend {
	return &FaultBackend{Backend: wrapped}
}

// SetPutError makes every subsequent Put fail with err (nil clears the fault).
// The error should read as transient — the replication queue classifies
// unknown errors as retryable and drops only permanent ones.
func (f *FaultBackend) SetPutError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.putErr = err
}

// SetPutDelay makes every subsequent Put sleep for d before delegating.
func (f *FaultBackend) SetPutDelay(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.putDelay = d
}

// PutCalls reports how many Put calls reached the wrapper, including faulted
// ones — useful to prove the replication write path was actually exercised.
func (f *FaultBackend) PutCalls() int64 {
	return f.putCalls.Load()
}

// Put delegates to the wrapped backend, first applying any configured delay
// and failure.
func (f *FaultBackend) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error {
	f.mu.Lock()
	putErr := f.putErr
	delay := f.putDelay
	f.mu.Unlock()

	f.putCalls.Add(1)
	if delay > 0 {
		time.Sleep(delay)
	}
	if putErr != nil {
		return putErr
	}
	return f.Backend.Put(ctx, bucket, key, body, size, meta)
}
