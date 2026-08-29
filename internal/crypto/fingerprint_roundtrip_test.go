// Package crypto provides tests for MEK fingerprint-based DEK wrapping/unwrapping.
package crypto

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"testing"
)

// TestWrapUnwrapV2WithEmptyRing tests wrapping and unwrapping with v2 format
// when there is no ring configured (acceptance criterion: empty ring).
func TestWrapUnwrapV2WithEmptyRing(t *testing.T) {
	// Generate MEK and DEK
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("Failed to generate MEK: %v", err)
	}

	dek, err := GenerateDEK()
	if err != nil {
		t.Fatalf("Failed to generate DEK: %v", err)
	}

	// Wrap DEK with fingerprint
	wrappedDEKStr, err := WrapDEKWithFingerprint(mek, dek)
	if err != nil {
		t.Fatalf("Failed to wrap DEK with fingerprint: %v", err)
	}

	// Verify format: v2:<fp16>:<base64>
	if len(wrappedDEKStr) < 4 || wrappedDEKStr[:3] != "v2:" {
		t.Errorf("Wrapped DEK format incorrect, got: %s", wrappedDEKStr)
	}

	// Build lookup function that only knows the active key (no ring)
	activeFingerprint := MEKFingerprint(mek)
	lookupMEK := func(keyID, fingerprint string) ([]byte, bool) {
		if fingerprint == activeFingerprint {
			return mek, true
		}
		return nil, false
	}

	// Build legacy fallback (should not be called for v2 format)
	legacyFallback := func(wrappedDEK []byte) ([]byte, error) {
		return nil, fmt.Errorf("legacy fallback should not be called for v2 format")
	}

	// Unwrap DEK by fingerprint
	unwrappedDEK, usedFingerprint, err := UnwrapDEKByFingerprint(wrappedDEKStr, lookupMEK, legacyFallback)
	if err != nil {
		t.Fatalf("Failed to unwrap DEK by fingerprint: %v", err)
	}

	if usedFingerprint != activeFingerprint {
		t.Errorf("Used fingerprint %s, expected %s", usedFingerprint, activeFingerprint)
	}

	if !bytes.Equal(dek, unwrappedDEK) {
		t.Error("Unwrapped DEK doesn't match original")
	}
}

// TestWrapUnwrapV2WithRingKeyRotation tests object written under key A
// read by server whose active is B with A in ring (acceptance criterion:
// object written under key A read by server whose active is B with A in ring).
func TestWrapUnwrapV2WithRingKeyRotation(t *testing.T) {
	// Generate two different MEKs (simulating key rotation)
	oldMEK := make([]byte, 32)
	newMEK := make([]byte, 32)
	if _, err := rand.Read(oldMEK); err != nil {
		t.Fatalf("Failed to generate old MEK: %v", err)
	}
	if _, err := rand.Read(newMEK); err != nil {
		t.Fatalf("Failed to generate new MEK: %v", err)
	}

	// Ensure they're different
	if bytes.Equal(oldMEK, newMEK) {
		t.Fatal("Generated identical MEKs, retry")
	}

	dek, err := GenerateDEK()
	if err != nil {
		t.Fatalf("Failed to generate DEK: %v", err)
	}

	// Wrap DEK with OLD key (as if written before rotation)
	wrappedDEKStr, err := WrapDEKWithFingerprint(oldMEK, dek)
	if err != nil {
		t.Fatalf("Failed to wrap DEK with old MEK: %v", err)
	}

	oldFingerprint := MEKFingerprint(oldMEK)

	// Simulate server with NEW active key and OLD key in ring
	lookupMEK := func(keyID, fingerprint string) ([]byte, bool) {
		// Check if fingerprint matches the old key (now in ring)
		if fingerprint == oldFingerprint {
			return oldMEK, true
		}
		// Check if fingerprint matches the new active key
		if fingerprint == MEKFingerprint(newMEK) {
			return newMEK, true
		}
		return nil, false
	}

	// Build legacy fallback (should not be called for v2 format)
	legacyFallback := func(wrappedDEK []byte) ([]byte, error) {
		return nil, fmt.Errorf("legacy fallback should not be called for v2 format")
	}

	// Unwrap DEK - should find old key by fingerprint in ring
	unwrappedDEK, usedFingerprint, err := UnwrapDEKByFingerprint(wrappedDEKStr, lookupMEK, legacyFallback)
	if err != nil {
		t.Fatalf("Failed to unwrap DEK with ring key: %v", err)
	}

	if usedFingerprint != oldFingerprint {
		t.Errorf("Used fingerprint %s, expected old key fingerprint %s", usedFingerprint, oldFingerprint)
	}

	if !bytes.Equal(dek, unwrappedDEK) {
		t.Error("Unwrapped DEK doesn't match original")
	}
}

// TestWrapUnwrapLegacyFormatWithRing tests legacy unprefixed value under
// a ring key (acceptance criterion: legacy unprefixed value under a ring key).
func TestWrapUnwrapLegacyFormatWithRing(t *testing.T) {
	// Generate old MEK (in ring) and new active MEK
	oldMEK := make([]byte, 32)
	newMEK := make([]byte, 32)
	if _, err := rand.Read(oldMEK); err != nil {
		t.Fatalf("Failed to generate old MEK: %v", err)
	}
	if _, err := rand.Read(newMEK); err != nil {
		t.Fatalf("Failed to generate new MEK: %v", err)
	}

	dek, err := GenerateDEK()
	if err != nil {
		t.Fatalf("Failed to generate DEK: %v", err)
	}

	// Wrap DEK with OLD key using OLD format (no v2: prefix)
	wrappedDEK, err := WrapDEK(oldMEK, dek)
	if err != nil {
		t.Fatalf("Failed to wrap DEK: %v", err)
	}

	// Encode as legacy base64 (no v2: prefix)
	legacyWrappedDEKStr := base64.StdEncoding.EncodeToString(wrappedDEK)

	// Simulate server with NEW active key and OLD key in ring
	lookupMEK := func(keyID, fingerprint string) ([]byte, bool) {
		// Should not be called for legacy format
		return nil, false
	}

	// Build legacy fallback that tries active key then ring keys
	legacyFallback := func(wrappedDEKBytes []byte) ([]byte, error) {
		// Try new active key first (should fail)
		_, err := UnwrapDEK(newMEK, wrappedDEKBytes)
		if err == nil {
			return nil, fmt.Errorf("new active key should not unwrap old DEK")
		}

		// Try old key in ring (should succeed)
		unwrapped, err := UnwrapDEK(oldMEK, wrappedDEKBytes)
		if err != nil {
			return nil, fmt.Errorf("old key in ring failed to unwrap: %w", err)
		}
		return unwrapped, nil
	}

	// Unwrap DEK - should use legacy fallback and find old key in ring
	unwrappedDEK, usedFingerprint, err := UnwrapDEKByFingerprint(legacyWrappedDEKStr, lookupMEK, legacyFallback)
	if err != nil {
		t.Fatalf("Failed to unwrap legacy DEK with ring key: %v", err)
	}

	// Legacy format should return empty fingerprint
	if usedFingerprint != "" {
		t.Errorf("Legacy format should return empty fingerprint, got: %s", usedFingerprint)
	}

	if !bytes.Equal(dek, unwrappedDEK) {
		t.Error("Unwrapped DEK doesn't match original")
	}
}

// TestWrapUnwrapV2FingerprintNotInRing tests fingerprint not in ring
// → clean error (acceptance criterion: fingerprint in no ring → clean error).
func TestWrapUnwrapV2FingerprintNotInRing(t *testing.T) {
	// Generate MEK
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("Failed to generate MEK: %v", err)
	}

	dek, err := GenerateDEK()
	if err != nil {
		t.Fatalf("Failed to generate DEK: %v", err)
	}

	// Wrap DEK with fingerprint
	wrappedDEKStr, err := WrapDEKWithFingerprint(mek, dek)
	if err != nil {
		t.Fatalf("Failed to wrap DEK with fingerprint: %v", err)
	}

	// Build lookup function that doesn't know the fingerprint (simulating lost key)
	unknownFingerprint := MEKFingerprint(mek)
	lookupMEK := func(keyID, fingerprint string) ([]byte, bool) {
		// Return false for any fingerprint - key not found
		return nil, false
	}

	// Build legacy fallback (should not be called for v2 format)
	legacyFallback := func(wrappedDEK []byte) ([]byte, error) {
		return nil, fmt.Errorf("legacy fallback should not be called for v2 format")
	}

	// Unwrap DEK - should fail with clean error naming the fingerprint
	_, _, err = UnwrapDEKByFingerprint(wrappedDEKStr, lookupMEK, legacyFallback)
	if err == nil {
		t.Fatal("Expected error when fingerprint not in any ring")
	}

	if err != ErrFingerprintNotFound {
		t.Errorf("Expected ErrFingerprintNotFound, got: %v", err)
	}

	// Verify error message names the fingerprint
	errMsg := err.Error()
	if !contains(errMsg, unknownFingerprint) {
		t.Errorf("Error message should name the fingerprint %s, got: %v", unknownFingerprint, errMsg)
	}
}

// TestWrapUnwrapV2FormatComponents tests that v2 format components
// are correctly structured and can be parsed.
func TestWrapUnwrapV2FormatComponents(t *testing.T) {
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("Failed to generate MEK: %v", err)
	}

	dek, err := GenerateDEK()
	if err != nil {
		t.Fatalf("Failed to generate DEK: %v", err)
	}

	wrappedDEKStr, err := WrapDEKWithFingerprint(mek, dek)
	if err != nil {
		t.Fatalf("Failed to wrap DEK with fingerprint: %v", err)
	}

	// Parse format: v2:<fp16>:<base64>
	parts := bytes.SplitN([]byte(wrappedDEKStr), []byte(":"), 3)
	if len(parts) != 3 {
		t.Fatalf("Expected 3 parts, got %d", len(parts))
	}

	if string(parts[0]) != "v2" {
		t.Errorf("Expected prefix 'v2', got: %s", parts[0])
	}

	fingerprint := string(parts[1])
	if len(fingerprint) != 16 {
		t.Errorf("Expected 16-char fingerprint, got length %d: %s", len(fingerprint), fingerprint)
	}

	base64Wrapped := string(parts[2])

	// Verify base64 can be decoded
	wrappedBytes, err := base64.StdEncoding.DecodeString(base64Wrapped)
	if err != nil {
		t.Fatalf("Failed to decode base64: %v", err)
	}

	// Verify wrapped DEK can be unwrapped
	unwrapped, err := UnwrapDEK(mek, wrappedBytes)
	if err != nil {
		t.Fatalf("Failed to unwrap: %v", err)
	}

	if !bytes.Equal(dek, unwrapped) {
		t.Error("Unwrapped DEK doesn't match original")
	}
}

// TestTwoServerCrossRead tests two servers with different active keys
// but same ring can read each other's objects (acceptance criterion:
// two-server test with different active keys, same ring).
func TestTwoServerCrossRead(t *testing.T) {
	// Server 1: Active key A, Ring contains B
	server1Active := make([]byte, 32)
	server1RingKey := make([]byte, 32)
	if _, err := rand.Read(server1Active); err != nil {
		t.Fatalf("Failed to generate server1 active: %v", err)
	}
	if _, err := rand.Read(server1RingKey); err != nil {
		t.Fatalf("Failed to generate server1 ring: %v", err)
	}

	// Server 2: Active key B, Ring contains A
	server2Active := server1RingKey // Server 2's active is Server 1's ring
	server2RingKey := server1Active // Server 2's ring is Server 1's active

	dek, err := GenerateDEK()
	if err != nil {
		t.Fatalf("Failed to generate DEK: %v", err)
	}

	// Object written by Server 1 with its active key (key A)
	wrappedByServer1, err := WrapDEKWithFingerprint(server1Active, dek)
	if err != nil {
		t.Fatalf("Server 1 failed to wrap: %v", err)
	}

	// Server 2's lookup function (knows active B and ring A)
	server2Lookup := func(keyID, fingerprint string) ([]byte, bool) {
		fpServer2Active := MEKFingerprint(server2Active)
		fpServer2Ring := MEKFingerprint(server2RingKey)

		if fingerprint == fpServer2Active {
			return server2Active, true
		}
		if fingerprint == fpServer2Ring {
			return server2RingKey, true
		}
		return nil, false
	}

	server2LegacyFallback := func(wrappedDEK []byte) ([]byte, error) {
		return nil, fmt.Errorf("legacy fallback should not be called for v2 format")
	}

	// Server 2 reads object written by Server 1
	unwrappedByServer2, _, err := UnwrapDEKByFingerprint(wrappedByServer1, server2Lookup, server2LegacyFallback)
	if err != nil {
		t.Fatalf("Server 2 failed to unwrap Server 1's object: %v", err)
	}

	if !bytes.Equal(dek, unwrappedByServer2) {
		t.Error("Server 2's unwrapped DEK doesn't match original")
	}

	// Object written by Server 2 with its active key (key B)
	wrappedByServer2, err := WrapDEKWithFingerprint(server2Active, dek)
	if err != nil {
		t.Fatalf("Server 2 failed to wrap: %v", err)
	}

	// Server 1's lookup function (knows active A and ring B)
	server1Lookup := func(keyID, fingerprint string) ([]byte, bool) {
		fpServer1Active := MEKFingerprint(server1Active)
		fpServer1Ring := MEKFingerprint(server1RingKey)

		if fingerprint == fpServer1Active {
			return server1Active, true
		}
		if fingerprint == fpServer1Ring {
			return server1RingKey, true
		}
		return nil, false
	}

	server1LegacyFallback := func(wrappedDEK []byte) ([]byte, error) {
		return nil, fmt.Errorf("legacy fallback should not be called for v2 format")
	}

	// Server 1 reads object written by Server 2
	unwrappedByServer1, _, err := UnwrapDEKByFingerprint(wrappedByServer2, server1Lookup, server1LegacyFallback)
	if err != nil {
		t.Fatalf("Server 1 failed to unwrap Server 2's object: %v", err)
	}

	if !bytes.Equal(dek, unwrappedByServer1) {
		t.Error("Server 1's unwrapped DEK doesn't match original")
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}
