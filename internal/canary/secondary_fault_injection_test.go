package canary

// Fault-injection end-to-end coverage for the secondary-backend canary
// family (ADR-006). The transition tests in contract_test.go drive
// updateSecondaryStateSuccess/updateSecondaryStateFailure with synthetic
// results, so they pin the state machine but NOT the check itself. These
// tests drive the real checkSecondaryBackend/runSecondaryBackendCheck
// against a fault-injectable secondary, so a semantics change in what the
// check writes, reads back, or verifies cannot silently drain the alert the
// runbook watches (docs/disaster-recovery.md, "Step 1": secondary_healthy,
// secondary_consecutive_fails, secondary_last_error, and the
// armor_secondary_canary_* gauges).
//
// Two fault modes are drilled, matching the two failure shapes the runbook
// describes:
//
//   - a hard outage (every secondary write fails): the family must go
//     unhealthy, count consecutive failures, record the error, drop the
//     armor_secondary_canary_healthy gauge, and increment the failure
//     counter — while the primary family stays untouched;
//   - silent mirror corruption (writes accepted, bytes damaged on read
//     back): the check re-reads the canary envelope from the secondary and
//     must fail on it — this is exactly how the runbook distinguishes a
//     damaged secondary from a replication outage.

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/metrics"
)

// faultySecondary wraps the in-memory mock with injectable secondary faults:
// a hard Put failure and a Get corruption mode that flips one byte of the
// returned envelope (silent bit-rot on the standby). Everything else
// delegates to the mock unchanged.
type faultySecondary struct {
	*mockBackend

	mu         sync.Mutex
	putErr     error
	corruptGet bool
}

func (f *faultySecondary) setPutErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.putErr = err
}

func (f *faultySecondary) setCorruptGet(on bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.corruptGet = on
}

func (f *faultySecondary) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error {
	f.mu.Lock()
	putErr := f.putErr
	f.mu.Unlock()
	if putErr != nil {
		return putErr
	}
	return f.mockBackend.Put(ctx, bucket, key, body, size, meta)
}

func (f *faultySecondary) Get(ctx context.Context, bucket, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	rc, info, err := f.mockBackend.Get(ctx, bucket, key)
	if err != nil {
		return rc, info, err
	}
	f.mu.Lock()
	corrupt := f.corruptGet
	f.mu.Unlock()
	if !corrupt {
		return rc, info, err
	}
	data, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return nil, nil, err
	}
	data[len(data)/2] ^= 0xFF
	return io.NopCloser(bytes.NewReader(data)), info, nil
}

// newSecondaryTestMonitor builds a monitor against a healthy in-memory
// primary and the given secondary with a fast retry budget, so a failing
// check costs milliseconds instead of the 3×10s production default.
func newSecondaryTestMonitor(secondary backend.Backend) *Monitor {
	mek := make([]byte, 32)
	rand.Read(mek)
	return NewMonitor(Config{
		Backend:          newMockBackend(),
		SecondaryBackend: secondary,
		Bucket:           "test-bucket",
		MEK:              mek,
		BlockSize:        65536,
		InstanceID:       "fault-injection-test",
		CanarySize:       100,
		MaxRetries:       2,
		RetryDelay:       time.Millisecond,
	})
}

// TestMonitorSecondaryCheckFaultInjectionCycle drives the real secondary
// canary check through healthy → hard outage → recovery and pins the runbook
// fields and the armor_secondary_canary_* gauges at each stage.
func TestMonitorSecondaryCheckFaultInjectionCycle(t *testing.T) {
	secondary := &faultySecondary{mockBackend: newMockBackend()}
	m := newSecondaryTestMonitor(secondary)
	ctx := context.Background()

	failuresBefore := metrics.DefaultMetrics.SecondaryCanaryCheckFailures.Value()

	// Healthy: the check writes the canary to both backends, reads both
	// back, and reports the family healthy with a clean failure ledger.
	m.runSecondaryBackendCheck(ctx)
	s := m.GetStatus()
	if s.SecondaryHealthy != StatusHealthy || !s.SecondaryHealthyBool {
		t.Fatalf("healthy secondary: status %q bool %v", s.SecondaryHealthy, s.SecondaryHealthyBool)
	}
	if s.SecondaryConsecutiveFails != 0 || s.SecondaryLastError != "" {
		t.Errorf("healthy secondary: fails=%d err=%q, want 0 and empty", s.SecondaryConsecutiveFails, s.SecondaryLastError)
	}
	if got := metrics.DefaultMetrics.SecondaryCanaryHealthy.Value(); got != 1 {
		t.Errorf("armor_secondary_canary_healthy = %d after a healthy check, want 1", got)
	}
	if got := metrics.DefaultMetrics.SecondaryCanaryCheckFailures.Value(); got != failuresBefore {
		t.Errorf("failure counter moved by %d across a healthy check, want 0", got-failuresBefore)
	}

	// Hard outage: every secondary Put fails. The family must go unhealthy,
	// record the injected error verbatim, and flip the gauge — that flip is
	// what the runbook's alert watches.
	secondary.setPutErr(errors.New("injected secondary outage (canary fault-injection drill)"))
	m.runSecondaryBackendCheck(ctx)
	s = m.GetStatus()
	if s.SecondaryHealthy != StatusUnhealthy || s.SecondaryHealthyBool {
		t.Errorf("faulted secondary: status %q bool %v, want unhealthy", s.SecondaryHealthy, s.SecondaryHealthyBool)
	}
	if s.SecondaryConsecutiveFails != 1 {
		t.Errorf("faulted secondary: consecutive fails = %d, want 1", s.SecondaryConsecutiveFails)
	}
	if !strings.Contains(s.SecondaryLastError, "injected secondary outage") {
		t.Errorf("faulted secondary: last error %q does not carry the injected failure", s.SecondaryLastError)
	}
	if got := metrics.DefaultMetrics.SecondaryCanaryHealthy.Value(); got != 0 {
		t.Errorf("armor_secondary_canary_healthy = %d with the secondary faulted, want 0", got)
	}
	if got := metrics.DefaultMetrics.SecondaryCanaryCheckFailures.Value(); got != failuresBefore+1 {
		t.Errorf("failure counter moved by %d across the faulted check, want 1", got-failuresBefore)
	}

	// The primary family must not move: a secondary outage is not a primary
	// health signal (the families are independent — pinned synthetically in
	// contract_test.go, here with a real failing check).
	if s.Status != StatusUnknown {
		t.Errorf("primary family moved to %q during a secondary-only outage", s.Status)
	}

	// Recovery: the gauge returns to healthy and the ledger resets — the
	// transition the runbook requires before trusting the mirror again.
	secondary.setPutErr(nil)
	m.runSecondaryBackendCheck(ctx)
	s = m.GetStatus()
	if s.SecondaryHealthy != StatusHealthy || !s.SecondaryHealthyBool {
		t.Errorf("recovered secondary: status %q bool %v, want healthy", s.SecondaryHealthy, s.SecondaryHealthyBool)
	}
	if s.SecondaryConsecutiveFails != 0 || s.SecondaryLastError != "" {
		t.Errorf("recovered secondary: fails=%d err=%q, want both reset", s.SecondaryConsecutiveFails, s.SecondaryLastError)
	}
	if got := metrics.DefaultMetrics.SecondaryCanaryHealthy.Value(); got != 1 {
		t.Errorf("armor_secondary_canary_healthy = %d after recovery, want 1", got)
	}
	if got := metrics.DefaultMetrics.SecondaryCanaryCheckFailures.Value(); got != failuresBefore+1 {
		t.Errorf("failure counter moved by %d across the recovery check, want 0", got-failuresBefore-1)
	}
}

// TestMonitorSecondaryCheckDetectsMirrorCorruption pins the silent-damage
// mode: the secondary accepts every write but returns a corrupted envelope
// on read-back. The check re-reads the canary object from the secondary and
// must fail the family — bit-rot on the standby must trip secondary_healthy
// even though nothing errored on the write path.
func TestMonitorSecondaryCheckDetectsMirrorCorruption(t *testing.T) {
	secondary := &faultySecondary{mockBackend: newMockBackend()}
	m := newSecondaryTestMonitor(secondary)
	ctx := context.Background()

	m.runSecondaryBackendCheck(ctx)
	if s := m.GetStatus(); s.SecondaryHealthy != StatusHealthy {
		t.Fatalf("baseline check against a healthy secondary: status %q, want healthy", s.SecondaryHealthy)
	}

	secondary.setCorruptGet(true)
	m.runSecondaryBackendCheck(ctx)
	s := m.GetStatus()
	if s.SecondaryHealthy != StatusUnhealthy || s.SecondaryHealthyBool {
		t.Errorf("corrupted mirror: status %q bool %v, want unhealthy — the check did not catch read-back corruption", s.SecondaryHealthy, s.SecondaryHealthyBool)
	}
	if s.SecondaryConsecutiveFails != 1 {
		t.Errorf("corrupted mirror: consecutive fails = %d, want 1", s.SecondaryConsecutiveFails)
	}
	if s.SecondaryLastError == "" {
		t.Error("corrupted mirror: last error is empty; the corruption was not recorded")
	}
	if got := metrics.DefaultMetrics.SecondaryCanaryHealthy.Value(); got != 0 {
		t.Errorf("armor_secondary_canary_healthy = %d with the mirror corrupted, want 0", got)
	}
}
