//go:build !integration
// +build !integration

// Secret-safe output smoke for the documented CLI contract.
// docs/cli-reference.md's safe-use guidance ("Access and secret keys have no
// command-line flags on purpose", prefer -mek-file over -mek, keep escrow
// files mode 600) all presume the binary never echoes loaded credential
// material back through its own output streams. These tests run the
// documented credential paths with distinctive planted values and pin that
// presumption: every failure the binary reports on these paths names the
// problem, never the value. Without this, a refactor that adds the offending
// input to an error message would leak key material into the terminal and
// every captured session log.
package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// secretSafetyMEK is a well-formed 32-byte MEK in hex. The repeating
// pattern keeps it low-entropy (so credential scanners do not mistake the
// fixture for a real key) while staying distinctive enough that its
// appearance in any output stream is an unambiguous leak.
var secretSafetyMEK = strings.Repeat("9e", 32)

// assertSecretAbsent fails the test when the secret value appears in either
// output stream of the run.
func assertSecretAbsent(t *testing.T, secret, context string, res referenceRun) {
	t.Helper()
	for _, stream := range []struct{ name, body string }{{"stdout", res.stdout}, {"stderr", res.stderr}} {
		if strings.Contains(stream.body, secret) {
			t.Errorf("%s: %s carries the secret value; the CLI must never echo credential material:\n%s",
				context, stream.name, stream.body)
		}
	}
}

// TestCLIReferenceSecretsNeverReachOutput plants a distinctive MEK through
// every documented key source (-mek, -mek-file, ARMOR_MEK) and a
// distinctive token through ARMOR_ADMIN_TOKEN, drives each command into its
// documented failure path, and asserts the value never reaches stdout or
// stderr.
func TestCLIReferenceSecretsNeverReachOutput(t *testing.T) {
	env := armorReferenceEnv(t)

	t.Run("decrypt -mek with no input exits 1 without echoing the key", func(t *testing.T) {
		res := runReference(t, env, "decrypt", "-mek", secretSafetyMEK)
		if res.exitCode != 1 {
			t.Errorf("decrypt -mek with no input: exit = %d, want 1\nstderr: %s", res.exitCode, res.stderr)
		}
		assertSecretAbsent(t, secretSafetyMEK, "decrypt -mek", res)
	})

	t.Run("decrypt -mek-file with no input exits 1 without echoing the key", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "mek.hex")
		if err := os.WriteFile(path, []byte(secretSafetyMEK+"\n"), 0o600); err != nil {
			t.Fatalf("write MEK file: %v", err)
		}
		res := runReference(t, env, "decrypt", "-mek-file", path)
		if res.exitCode != 1 {
			t.Errorf("decrypt -mek-file with no input: exit = %d, want 1\nstderr: %s", res.exitCode, res.stderr)
		}
		assertSecretAbsent(t, secretSafetyMEK, "decrypt -mek-file", res)
	})

	t.Run("decrypt via ARMOR_MEK with no input exits 1 without echoing the key", func(t *testing.T) {
		res := runReference(t, armorReferenceEnv(t, "ARMOR_MEK="+secretSafetyMEK), "decrypt")
		if res.exitCode != 1 {
			t.Errorf("decrypt with ARMOR_MEK and no input: exit = %d, want 1\nstderr: %s", res.exitCode, res.stderr)
		}
		assertSecretAbsent(t, secretSafetyMEK, "decrypt ARMOR_MEK", res)
	})

	t.Run("decrypt with a malformed -mek exits 1 without echoing the value", func(t *testing.T) {
		// The malformed value is not itself a secret, but the error path
		// that reports it is the same one a malformed real key would hit;
		// echoing the offending input here would leak a truncated or
		// mistyped real key.
		const malformed = "zz-not-hex-4leakcheck"
		res := runReference(t, env, "decrypt", "-mek", malformed)
		if res.exitCode != 1 {
			t.Errorf("decrypt malformed -mek: exit = %d, want 1\nstderr: %s", res.exitCode, res.stderr)
		}
		assertSecretAbsent(t, malformed, "decrypt malformed -mek", res)
	})

	t.Run("verify with a loaded MEK and no backend exits 1 without echoing the key", func(t *testing.T) {
		// The MEK is fully loaded before the B2 backend initialization
		// fails, so this exercises the window where the key is in memory
		// while the command reports its error.
		res := runReference(t, env, "verify", "-bucket", "smoke-bucket", "-mek", secretSafetyMEK)
		if res.exitCode != 1 {
			t.Errorf("verify with MEK and no backend: exit = %d, want 1\nstderr: %s", res.exitCode, res.stderr)
		}
		assertSecretAbsent(t, secretSafetyMEK, "verify with MEK", res)
	})

	t.Run("migrate with a rejected token exits 1 without echoing the token", func(t *testing.T) {
		// The admin API rejects the credential with a 401 whose body the
		// binary echoes into its diagnostic — the token itself must not
		// ride along.
		const token = "cli-ref-secret-token-4leakcheck"
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		}))
		defer srv.Close()

		res := runReference(t, armorReferenceEnv(t, "ARMOR_ADMIN_TOKEN="+token), "migrate", "-admin-url", srv.URL)
		if res.exitCode != 1 {
			t.Errorf("migrate with rejected token: exit = %d, want 1\nstderr: %s", res.exitCode, res.stderr)
		}
		assertSecretAbsent(t, token, "migrate rejected token", res)
	})
}
