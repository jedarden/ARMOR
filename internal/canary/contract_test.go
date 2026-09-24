package canary

import (
	"encoding/json"
	"testing"
	"time"
)

// contract_test.go pins the published observability contract for the canary
// status surface: the JSON field set of the shape served by GET /armor/canary
// (canary.Result), the richer diagnostic shape (Monitor.MarshalJSON /
// CanaryState), and the status transition rules. The human-readable contract
// is docs/observability-contract.md; a change here that breaks one of these
// assertions is a breaking change to that document and must land together.

// wantResultFields is the exact JSON key set of canary.Result — the shape the
// /armor/canary handler serves. The three *_last_error fields are omitempty
// and appear only once a check has failed.
var wantResultFields = map[string]bool{
	"status":                       true,
	"last_check":                   true,
	"last_error":                   true, // omitempty
	"upload_latency_ms":            true,
	"download_latency_ms":          true,
	"decrypt_verified":             true,
	"hmac_verified":                true,
	"cloudflare_cache_hit":         true,
	"multipart_healthy_status":     true,
	"multipart_healthy":            true,
	"multipart_last_check":         true,
	"multipart_consecutive_fails":  true,
	"multipart_last_error":         true, // omitempty
	"secondary_healthy_status":     true,
	"secondary_healthy":            true,
	"secondary_last_check":         true,
	"secondary_consecutive_fails":  true,
	"secondary_last_error":         true, // omitempty
	"secondary_replication_lag_ms": true,
	"secondary_queue_depth":        true,
}

// alwaysPresent is the omitempty-free subset: fields every response carries.
var alwaysPresent = []string{
	"status", "last_check",
	"upload_latency_ms", "download_latency_ms", "decrypt_verified", "hmac_verified", "cloudflare_cache_hit",
	"multipart_healthy_status", "multipart_healthy", "multipart_last_check", "multipart_consecutive_fails",
	"secondary_healthy_status", "secondary_healthy", "secondary_last_check", "secondary_consecutive_fails",
	"secondary_replication_lag_ms", "secondary_queue_depth",
}

// TestCanaryStatusResultJSONContract verifies that a fresh monitor's
// GetStatus serializes to exactly the documented /armor/canary field set,
// with the pre-first-check values the contract promises: all three statuses
// "unknown", both boolean projections false, and no last_error fields.
func TestCanaryStatusResultJSONContract(t *testing.T) {
	m := NewMonitor(Config{})

	raw, err := json.Marshal(m.GetStatus())
	if err != nil {
		t.Fatalf("marshal GetStatus: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, field := range alwaysPresent {
		if _, ok := got[field]; !ok {
			t.Errorf("contract field %q missing from /armor/canary response", field)
		}
	}
	for field := range got {
		if !wantResultFields[field] {
			t.Errorf("undocumented field %q in /armor/canary response; update docs/observability-contract.md", field)
		}
	}

	// Pre-first-check: every family is "unknown" and the boolean projections
	// are false — including multipart_healthy, whose gauge analogue also
	// reads 0 before the first completed check.
	for _, statusField := range []string{"status", "multipart_healthy_status", "secondary_healthy_status"} {
		if got[statusField] != string(StatusUnknown) {
			t.Errorf("fresh monitor %s = %v, want %q", statusField, got[statusField], StatusUnknown)
		}
	}
	for _, boolField := range []string{"multipart_healthy", "secondary_healthy"} {
		if got[boolField] != false {
			t.Errorf("fresh monitor %s = %v, want false", boolField, got[boolField])
		}
	}
	if _, ok := got["last_error"]; ok {
		t.Errorf("last_error must be omitted on a fresh monitor, got %v", got["last_error"])
	}
	if _, ok := got["instance_id"]; ok {
		t.Error("instance_id belongs to the CanaryState diagnostic shape, not the /armor/canary Result shape")
	}
}

// TestCanaryMarshalJSONDiagnosticShape pins the richer CanaryState shape
// served through Monitor.MarshalJSON: it adds instance_id, last_success, and
// consecutive_success on top of the Result fields.
func TestCanaryMarshalJSONDiagnosticShape(t *testing.T) {
	m := NewMonitor(Config{InstanceID: "contract-probe"})

	raw, err := m.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, field := range []string{"instance_id", "last_success", "consecutive_success", "status", "multipart_healthy", "secondary_healthy"} {
		if _, ok := got[field]; !ok {
			t.Errorf("diagnostic shape missing contract field %q", field)
		}
	}
	if got["instance_id"] != "contract-probe" {
		t.Errorf("instance_id = %v, want contract-probe", got["instance_id"])
	}
}

// TestCanaryStatusTransitions drives the state update paths directly and
// asserts the published transition rules: unknown -> healthy on success,
// healthy -> unhealthy only after the retry budget is exhausted, counter
// reset semantics, and the boolean projections following their status
// strings.
func TestCanaryStatusTransitions(t *testing.T) {
	m := NewMonitor(Config{})

	if got := m.GetStatus().Status; got != StatusUnknown {
		t.Fatalf("fresh monitor status = %q, want unknown", got)
	}

	now := time.Now()
	pass := &Result{Status: StatusHealthy, LastCheck: now, DecryptVerified: true, HMACVerified: true}

	// Success: healthy, counters move, error cleared, timestamps recorded.
	// The consecutive counters live on CanaryState (not the Result served by
	// /armor/canary), so they are read from the monitor's state directly.
	m.updateStateSuccess(pass)
	s := m.GetStatus()
	if s.Status != StatusHealthy {
		t.Errorf("after first success: status=%q", s.Status)
	}
	if m.state.ConsecutiveSuccess != 1 || m.state.ConsecutiveFailures != 0 {
		t.Errorf("after first success: succ=%d fail=%d", m.state.ConsecutiveSuccess, m.state.ConsecutiveFailures)
	}
	if m.state.LastSuccess.IsZero() {
		t.Error("successful check must stamp last_success")
	}
	if s.LastError != "" {
		t.Errorf("successful check must clear last_error, got %q", s.LastError)
	}
	if !s.DecryptVerified || !s.HMACVerified {
		t.Error("successful check must carry decrypt_verified/hmac_verified through to status")
	}

	// Failure: unhealthy, success counter resets, failure counter increments.
	m.updateStateFailure(errFake("boom"))
	s = m.GetStatus()
	if s.Status != StatusUnhealthy {
		t.Errorf("after failure: status=%q", s.Status)
	}
	if m.state.ConsecutiveSuccess != 0 || m.state.ConsecutiveFailures != 1 {
		t.Errorf("after failure: succ=%d fail=%d", m.state.ConsecutiveSuccess, m.state.ConsecutiveFailures)
	}
	if s.LastError != "boom" {
		t.Errorf("last_error = %q, want boom", s.LastError)
	}

	// Recovery: healthy again, failure counter resets, error cleared.
	m.updateStateSuccess(pass)
	s = m.GetStatus()
	if s.Status != StatusHealthy || s.LastError != "" {
		t.Errorf("after recovery: status=%q err=%q", s.Status, s.LastError)
	}
	if m.state.ConsecutiveFailures != 0 {
		t.Errorf("after recovery: consecutive_failures = %d, want 0", m.state.ConsecutiveFailures)
	}
}

// TestCanaryMultipartStatusTransitions pins the multipart family: its status
// lives independently of the small-object status, and the multipart_healthy
// boolean is a strict projection of multipart_healthy_status.
func TestCanaryMultipartStatusTransitions(t *testing.T) {
	m := NewMonitor(Config{})

	// A small-object success must not move the multipart family.
	m.updateStateSuccess(&Result{Status: StatusHealthy, LastCheck: time.Now()})
	if got := m.GetStatus().MultipartHealthy; got != StatusUnknown {
		t.Fatalf("multipart status moved without a multipart check: %q", got)
	}

	pass := &Result{Status: StatusHealthy, LastCheck: time.Now()}
	m.updateMultipartStateSuccess(pass)
	s := m.GetStatus()
	if s.MultipartHealthy != StatusHealthy || !s.MultipartHealthyBool {
		t.Errorf("after multipart success: status=%q bool=%v", s.MultipartHealthy, s.MultipartHealthyBool)
	}
	if m.state.MultipartLastSuccess.IsZero() {
		t.Error("multipart success must stamp multipart_last_success")
	}

	m.updateMultipartStateFailure(errFake("part 2 vanished"))
	s = m.GetStatus()
	if s.MultipartHealthy != StatusUnhealthy || s.MultipartHealthyBool {
		t.Errorf("after multipart failure: status=%q bool=%v", s.MultipartHealthy, s.MultipartHealthyBool)
	}
	if s.MultipartConsecutiveFails != 1 || s.MultipartLastError != "part 2 vanished" {
		t.Errorf("multipart fail=%d err=%q", s.MultipartConsecutiveFails, s.MultipartLastError)
	}

	m.updateMultipartStateSuccess(pass)
	if s = m.GetStatus(); s.MultipartConsecutiveFails != 0 || s.MultipartLastError != "" {
		t.Errorf("multipart recovery must reset counter and error, got fail=%d err=%q", s.MultipartConsecutiveFails, s.MultipartLastError)
	}
}

// TestCanarySecondaryStatusTransitions pins the secondary-backend family
// (ADR-006): status independent of the other families, and the lag/depth
// gauges carried through from the replication queue.
func TestCanarySecondaryStatusTransitions(t *testing.T) {
	m := NewMonitor(Config{})

	m.updateSecondaryStateSuccess(&Result{Status: StatusHealthy, LastCheck: time.Now()}, 7, 2) // depth 7, lag 2s
	s := m.GetStatus()
	if s.SecondaryHealthy != StatusHealthy || !s.SecondaryHealthyBool {
		t.Errorf("after secondary success: status=%q bool=%v", s.SecondaryHealthy, s.SecondaryHealthyBool)
	}
	if s.SecondaryQueueDepth != 7 || s.SecondaryReplicationLagMs != 2000 {
		t.Errorf("lag contract: depth=%d lag_ms=%d, want 7 / 2000", s.SecondaryQueueDepth, s.SecondaryReplicationLagMs)
	}

	m.updateSecondaryStateFailure(errFake("secondary unreachable"), 7, 2)
	s = m.GetStatus()
	if s.SecondaryHealthy != StatusUnhealthy || s.SecondaryConsecutiveFails != 1 || s.SecondaryLastError != "secondary unreachable" {
		t.Errorf("after secondary failure: status=%q fails=%d err=%q", s.SecondaryHealthy, s.SecondaryConsecutiveFails, s.SecondaryLastError)
	}
}

// errFake is a tiny error stand-in so the transition tests need no backend.
type errFake string

func (e errFake) Error() string { return string(e) }
