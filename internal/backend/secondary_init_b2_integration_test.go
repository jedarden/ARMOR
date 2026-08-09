// Package backend provides a pluggable storage backend interface for ARMOR.
package backend

import (
	"context"
	"os"
	"strings"
	"testing"
)

// TestInitB2Backend_BadCredentialsRejection is the live counterpart to
// TestInitB2Backend_UnreachableEndpoint. The offline test points at a closed
// localhost port so the HeadBucket dial fails (connection refused); this test
// points at a REAL, reachable B2 endpoint with a deliberately-wrong secret so
// the probe fails inside B2's auth layer (HTTP 403) rather than at the dial —
// i.e. it exercises actual credential rejection, which no offline test can.
//
// Gating. This test makes a real network call against a live B2 endpoint, so
// it must never run as part of the default `go test ./...` suite. It skips on
// BOTH of:
//   - testing.Short(): the same pattern multipart_integration_test.go uses for
//     its real-B2 integration tests (see TestMultipartUpload, lines 20-21).
//   - the ARMOR_B2_TEST_* env vars being unset: the operator must opt in by
//     providing a real, reachable B2 account. Any unset var -> skip, so the
//     package passes with no B2 credentials in the environment.
//
// Why env-var gating rather than always-on bad static credentials: the latter
// would hit the live B2 endpoint on every non-short run, a network dependency
// that fails or hangs offline and does not belong in the default suite. The
// env var keeps the default run clean; when set, the account config is real and
// reachable, so the failure the test asserts is a genuine credential rejection
// (a wrong secret), not a connection error.
//
// Why all five account fields are required yet the secret is then corrupted:
// providing a full real account (valid key id, reachable endpoint, real bucket)
// isolates the rejection to the one wrong value — the secret. A wrong secret
// against an otherwise-valid account yields a clean HTTP 403
// (SignatureDoesNotMatch), the canonical credential-rejection failure, instead
// of an InvalidAccessKeyId that could also come from a malformed key id. The
// real secret env var exists so the operator's account config is complete and
// known-valid; corrupting it here is what puts the rejection path under test.
func TestInitB2Backend_BadCredentialsRejection(t *testing.T) {
	// Skip in short mode — mirrors multipart_integration_test.go's gate.
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Skip unless the operator has opted in with a real, reachable B2 account.
	endpoint := os.Getenv("ARMOR_B2_TEST_ENDPOINT")
	region := os.Getenv("ARMOR_B2_TEST_REGION")
	keyID := os.Getenv("ARMOR_B2_TEST_KEY_ID")
	secret := os.Getenv("ARMOR_B2_TEST_SECRET")
	bucket := os.Getenv("ARMOR_B2_TEST_BUCKET")
	if endpoint == "" || region == "" || keyID == "" || secret == "" || bucket == "" {
		t.Skip("skipping live B2 integration test: set ARMOR_B2_TEST_{ENDPOINT,REGION,KEY_ID,SECRET,BUCKET} to run")
	}

	// Reuse the real account but corrupt the secret so B2 rejects the request.
	// NewB2Backend is lazy (no network call), so construction succeeds and the
	// bad credential reaches the HeadBucket probe — where rejection surfaces.
	cfg := BackendConfig{
		Type:        "b2",
		Bucket:      bucket,
		Region:      region,
		Endpoint:    endpoint,
		AccessKeyID: keyID,
		SecretKey:   secret + "ARMOR-REJECTION-SENTINEL",
	}

	b, err := InitB2Backend(context.Background(), cfg)

	// Bad credentials must surface as a non-nil error, never a backend.
	if err == nil {
		t.Fatalf("expected error for invalid credentials against %s, got backend %T", endpoint, b)
	}
	if b != nil {
		t.Errorf("expected nil backend on credential rejection, got %T", b)
	}

	// The error must carry the initialization-failure wrap context so it reads
	// as a backend-init failure, not a raw SDK error.
	const wrapPrefix = "b2 backend initialization failed"
	if !strings.Contains(err.Error(), wrapPrefix) {
		t.Errorf("error %q missing wrap prefix %q", err.Error(), wrapPrefix)
	}
}
