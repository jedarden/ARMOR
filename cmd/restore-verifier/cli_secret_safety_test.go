//go:build !integration
// +build !integration

// Secret-safe output smoke for the restore-verifier's documented contract.
// docs/companion-cli-reference.md documents the B2 secret key and the MEK
// ring as environment-only credentials whose diagnostics name what is
// missing or malformed. These tests plant distinctive credential values,
// drive the documented startup-validation failures, and pin that the values
// never reach stdout or stderr — a refactor that echoes the offending input
// into one of those diagnostics would leak loaded key material into the
// container log.
package main

import (
	"strings"
	"testing"
)

// assertVerifierSecretAbsent fails the test when the secret value appears in
// either output stream of the run.
func assertVerifierSecretAbsent(t *testing.T, secret, context string, res verifierRun) {
	t.Helper()
	for _, stream := range []struct{ name, body string }{{"stdout", res.stdout}, {"stderr", res.stderr}} {
		if strings.Contains(stream.body, secret) {
			t.Errorf("%s: %s carries the secret value; restore-verifier must never echo credential material:\n%s",
				context, stream.name, stream.body)
		}
	}
}

// TestVerifierCLIReferenceSecretsNeverReachOutput plants a distinctive B2
// secret, MEK, and MEK-ring entry, drives the documented startup-validation
// failures that run with those values loaded, and asserts the values never
// reach the output streams.
func TestVerifierCLIReferenceSecretsNeverReachOutput(t *testing.T) {
	const (
		secretKey   = "smoke-secret-4leakcheck"
		bucketFlag  = "-bucket=smoke-bucket"
		baseCredEnv = "ARMOR_B2_REGION=smoke-region,ARMOR_B2_ENDPOINT=http://127.0.0.1:9," +
			"ARMOR_B2_ACCESS_KEY_ID=smoke-key,ARMOR_B2_SECRET_ACCESS_KEY=" + secretKey
	)

	t.Run("loaded B2 credentials with no MEK exits 1 without echoing the secret", func(t *testing.T) {
		res := runVerifier(t, verifierReferenceEnv(t, strings.Split(baseCredEnv, ",")...))
		if res.exitCode != 1 {
			t.Errorf("credentials but no MEK: exit = %d, want 1\nstderr: %s", res.exitCode, res.stderr)
		}
		if !strings.Contains(res.stderr, "Missing MEK") {
			t.Errorf("credentials but no MEK: stderr missing the documented diagnostic:\n%s", res.stderr)
		}
		assertVerifierSecretAbsent(t, secretKey, "credentials but no MEK", res)
	})

	t.Run("malformed ARMOR_MEK exits 1 without echoing the value", func(t *testing.T) {
		const malformed = "zz-not-hex-mek-4leakcheck"
		env := append(strings.Split(baseCredEnv, ","), "ARMOR_MEK="+malformed)
		res := runVerifier(t, verifierReferenceEnv(t, env...), "-bucket=smoke-bucket")
		if res.exitCode != 1 {
			t.Errorf("malformed MEK: exit = %d, want 1\nstderr: %s", res.exitCode, res.stderr)
		}
		if !strings.Contains(res.stderr, "Invalid MEK hex") {
			t.Errorf("malformed MEK: stderr missing the documented diagnostic:\n%s", res.stderr)
		}
		assertVerifierSecretAbsent(t, malformed, "malformed MEK", res)
	})

	t.Run("malformed MEK ring entry exits 1 without echoing the entry", func(t *testing.T) {
		// A ring entry is wrapped key material; the ring-parse diagnostic
		// must name the problem, never the entry.
		const badEntry = "zz-not-hex-ring-4leakcheck"
		env := append(strings.Split(baseCredEnv, ","),
			"ARMOR_MEK=abababababababababababababababababababababababababababababababab",
			"VERIFIER_MEK_RING="+badEntry)
		res := runVerifier(t, verifierReferenceEnv(t, env...), "-bucket=smoke-bucket")
		if res.exitCode != 1 {
			t.Errorf("malformed MEK ring: exit = %d, want 1\nstderr: %s", res.exitCode, res.stderr)
		}
		if !strings.Contains(res.stderr, "Invalid MEK ring entry") {
			t.Errorf("malformed MEK ring: stderr missing the documented diagnostic:\n%s", res.stderr)
		}
		assertVerifierSecretAbsent(t, badEntry, "malformed MEK ring", res)
	})
}

// TestVerifierSecretValueFlagsAreRejected pins the secrets-by-reference
// rule for the verifier (armor-b1e3a56b): -mek and -b2-secret-key no longer
// accept credential values — argv lands in shell history, ps, and
// transcripts — so each is rejected with a diagnostic naming the
// environment variable that carries the value instead, without echoing the
// value it was given.
func TestVerifierSecretValueFlagsAreRejected(t *testing.T) {
	const (
		mekValue    = "abababababababababababababababababababababababababababababababab"
		secretValue = "rejected-secret-4safety"
	)

	t.Run("-mek is rejected naming ARMOR_MEK", func(t *testing.T) {
		res := runVerifier(t, verifierReferenceEnv(t), "-mek", mekValue)
		if res.exitCode != 1 {
			t.Errorf("-mek: exit = %d, want 1\nstderr: %s", res.exitCode, res.stderr)
		}
		for _, want := range []string{"-mek no longer accepts", "ARMOR_MEK environment variable"} {
			if !strings.Contains(res.stderr, want) {
				t.Errorf("-mek rejection missing %q:\n%s", want, res.stderr)
			}
		}
		assertVerifierSecretAbsent(t, mekValue, "-mek rejection", res)
	})

	t.Run("-b2-secret-key is rejected naming ARMOR_B2_SECRET_ACCESS_KEY", func(t *testing.T) {
		res := runVerifier(t, verifierReferenceEnv(t), "-b2-secret-key", secretValue)
		if res.exitCode != 1 {
			t.Errorf("-b2-secret-key: exit = %d, want 1\nstderr: %s", res.exitCode, res.stderr)
		}
		for _, want := range []string{"-b2-secret-key no longer accepts", "ARMOR_B2_SECRET_ACCESS_KEY environment variable"} {
			if !strings.Contains(res.stderr, want) {
				t.Errorf("-b2-secret-key rejection missing %q:\n%s", want, res.stderr)
			}
		}
		assertVerifierSecretAbsent(t, secretValue, "-b2-secret-key rejection", res)
	})

	t.Run("environment MEK still starts up past the rejection gate", func(t *testing.T) {
		// The env path must be untouched by the flag rejection: full B2
		// credentials and a MEK from the environment get past both gates
		// (the ring parse then rejects this deliberately-fingerprint-only
		// ring entry, which is the next validation after the MEK loads).
		const fpOnly = "0123456789abcdef"
		env := []string{
			"ARMOR_B2_REGION=smoke-region",
			"ARMOR_B2_ACCESS_KEY_ID=smoke-key",
			"ARMOR_B2_SECRET_ACCESS_KEY=smoke-secret",
			"ARMOR_MEK=" + mekValue,
			"VERIFIER_MEK_RING=" + fpOnly,
		}
		res := runVerifier(t, verifierReferenceEnv(t, env...), "-bucket=smoke-bucket")
		if res.exitCode != 1 {
			t.Errorf("env MEK with fingerprint-only ring: exit = %d, want 1\nstderr: %s", res.exitCode, res.stderr)
		}
		if !strings.Contains(res.stderr, "fingerprint-only entries carry no key material") {
			t.Errorf("fingerprint-only ring entry must be rejected with the grammar diagnostic:\n%s", res.stderr)
		}
	})
}
