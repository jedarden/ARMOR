package clicontract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSecretFlagsAreRejectedNamingEnvVars pins the secrets-by-reference rule
// for the companion CLIs (armor-b1e3a56b): a credential VALUE passed as a
// flag lands in argv, ps, and session transcripts, so the flags that
// historically accepted one are rejected with a diagnostic naming the
// environment variable that carries it instead. The rejection must not echo
// the offending value.
func TestSecretFlagsAreRejectedNamingEnvVars(t *testing.T) {
	bins := builtBinaries(t)

	t.Run("armor-fleet -seam-token is rejected naming SEAM_TOKEN", func(t *testing.T) {
		targets := filepath.Join(t.TempDir(), "targets.yaml")
		const contents = `- name: contract-target
  cluster: contract-cluster
  namespace: contract-ns
  service: armor
  admin_port: 9001
`
		if err := os.WriteFile(targets, []byte(contents), 0o644); err != nil {
			t.Fatalf("write targets file: %v", err)
		}
		const token = "seam-token-value-4rejection"
		res := runBinary(t, bins["armor-fleet"], cleanEnv(), "-targets", targets, "-seam-token", token)
		if res.exitCode == 0 {
			t.Errorf("armor-fleet accepted -seam-token: exit = 0, want a rejection\n%s", res.combined())
		}
		for _, want := range []string{"-seam-token", "SEAM_TOKEN"} {
			if !strings.Contains(res.stderr, want) {
				t.Errorf("armor-fleet -seam-token rejection missing %q:\n%s", want, res.stderr)
			}
		}
		if strings.Contains(res.combined(), token) {
			t.Errorf("armor-fleet -seam-token rejection echoed the token value:\n%s", res.combined())
		}
	})

	t.Run("restore-verifier -mek is rejected naming ARMOR_MEK", func(t *testing.T) {
		const mek = "abababababababababababababababababababababababababababababab"
		res := runBinary(t, bins["restore-verifier"], cleanEnv(), "-mek", mek)
		if res.exitCode == 0 {
			t.Errorf("restore-verifier accepted -mek: exit = 0, want a rejection\n%s", res.combined())
		}
		for _, want := range []string{"-mek", "ARMOR_MEK"} {
			if !strings.Contains(res.stderr, want) {
				t.Errorf("restore-verifier -mek rejection missing %q:\n%s", want, res.stderr)
			}
		}
		if strings.Contains(res.combined(), mek) {
			t.Errorf("restore-verifier -mek rejection echoed the MEK value:\n%s", res.combined())
		}
	})

	t.Run("restore-verifier -b2-secret-key is rejected naming ARMOR_B2_SECRET_ACCESS_KEY", func(t *testing.T) {
		const secret = "b2-secret-value-4rejection"
		res := runBinary(t, bins["restore-verifier"], cleanEnv(), "-b2-secret-key", secret)
		if res.exitCode == 0 {
			t.Errorf("restore-verifier accepted -b2-secret-key: exit = 0, want a rejection\n%s", res.combined())
		}
		for _, want := range []string{"-b2-secret-key", "ARMOR_B2_SECRET_ACCESS_KEY"} {
			if !strings.Contains(res.stderr, want) {
				t.Errorf("restore-verifier -b2-secret-key rejection missing %q:\n%s", want, res.stderr)
			}
		}
		if strings.Contains(res.combined(), secret) {
			t.Errorf("restore-verifier -b2-secret-key rejection echoed the secret value:\n%s", res.combined())
		}
	})
}

// TestFlagVocabularyIsUniform pins the one flag vocabulary across the armor
// CLI and the companions (armor-b1e3a56b): verbosity is -verbose everywhere
// (-v stays a deprecated alias that still works and prints a pointer), the
// decrypt envelope format flag is -envelope-version (-version stays a
// deprecated alias), and -mek-ring means the shared fingerprint:key grammar
// in every binary that has it.
func TestFlagVocabularyIsUniform(t *testing.T) {
	bins := builtBinaries(t)

	t.Run("armor decrypt help documents the vocabulary flags", func(t *testing.T) {
		res := runBinary(t, bins["armor"], cleanEnv(), "decrypt", "--help")
		if res.exitCode != 0 {
			t.Fatalf("armor decrypt --help: exit = %d\n%s", res.exitCode, res.combined())
		}
		for _, want := range []string{"-verbose", "-envelope-version", "-mek-ring"} {
			if !strings.Contains(res.combined(), want) {
				t.Errorf("armor decrypt --help missing %q:\n%s", want, res.combined())
			}
		}
	})

	t.Run("armor decrypt deprecated aliases print pointers and still work", func(t *testing.T) {
		res := runBinary(t, bins["armor"], cleanEnv(), "decrypt", "-v", "-version", "2")
		if res.exitCode != 1 {
			t.Fatalf("decrypt without a MEK: exit = %d, want 1\n%s", res.exitCode, res.combined())
		}
		for _, want := range []string{
			"-v is deprecated; use -verbose",
			"-version is deprecated; use -envelope-version",
		} {
			if !strings.Contains(res.stderr, want) {
				t.Errorf("decrypt deprecated alias missing pointer %q:\n%s", want, res.stderr)
			}
		}
	})

	t.Run("armor verify help offers -verbose", func(t *testing.T) {
		res := runBinary(t, bins["armor"], cleanEnv(), "verify", "--help")
		if res.exitCode != 0 {
			t.Fatalf("armor verify --help: exit = %d\n%s", res.exitCode, res.combined())
		}
		if !strings.Contains(res.combined(), "-verbose") {
			t.Errorf("armor verify --help missing -verbose:\n%s", res.combined())
		}
	})

	t.Run("restore-verifier help documents the shared -mek-ring grammar", func(t *testing.T) {
		res := runBinary(t, bins["restore-verifier"], cleanEnv(), "--help")
		if res.exitCode != 0 {
			t.Fatalf("restore-verifier --help: exit = %d\n%s", res.exitCode, res.combined())
		}
		for _, want := range []string{"fingerprint:hexkey", "@file"} {
			if !strings.Contains(res.combined(), want) {
				t.Errorf("restore-verifier --help -mek-ring missing %q:\n%s", want, res.combined())
			}
		}
	})
}
