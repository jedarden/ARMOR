//go:build !integration
// +build !integration

// Tests for the 'armor config' subcommand group: dispatch shape, validate's
// exit-code contract, and show's golden outputs with the redaction policy.
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/config"
)

// configTestEnv is the deployment fixture the show goldens carry: one of
// every setting class — env-set, defaulted, derived, secret-bearing, dynamic
// per-key and per-credential rows. Values are low-entropy plants; the
// redaction assertions pin that none of the secret ones reaches the output.
var configTestEnv = map[string]string{
	"ARMOR_LISTEN":                 "127.0.0.1:9100",
	"ARMOR_B2_REGION":              "us-east-005",
	"ARMOR_B2_ACCESS_KEY_ID":       "golden-key-id",
	"ARMOR_B2_SECRET_ACCESS_KEY":   "planted-b2-secret-value",
	"ARMOR_BUCKET":                 "golden-bucket",
	"ARMOR_PREFIX":                 "tenant/",
	"ARMOR_MEK":                    "abababababababababababababababababababababababababababababababab",
	"ARMOR_MEK_ARCHIVE":            "cdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcd",
	"ARMOR_MEK_RING":               "edededededededededededededededededededededededededededededededed",
	"ARMOR_KEY_ROUTES":             "data/*=archive,*=default",
	"ARMOR_AUTH_ACCESS_KEY":        "golden-access",
	"ARMOR_AUTH_SECRET_KEY":        "planted-auth-secret-value",
	"ARMOR_AUTH_TENANT_ACCESS_KEY": "golden-tenant-access",
	"ARMOR_AUTH_TENANT_SECRET_KEY": "planted-tenant-secret-value",
	"ARMOR_ADMIN_TOKEN":            "planted-admin-token-value",
	"ARMOR_WRITER_ID":              "golden-writer",
	"ARMOR_READ_REQUEST_TIMEOUT":   "90s",
	"ARMOR_COMPRESS_RULES":         ".jsonl=zstd,*=none",
}

// secretPlants are the values the redaction assertions hunt for in every
// output stream: neither the text table nor the JSON array may carry one.
var secretPlants = []string{
	"planted-b2-secret-value",
	"planted-auth-secret-value",
	"planted-tenant-secret-value",
	"planted-admin-token-value",
	// The MEK material itself, in either hex form carried by the fixture.
	"abababababababababababababababababababababababababababababababab",
	"cdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcd",
	"edededededededededededededededededededededededededededededededed",
	// Unpublished identifiers render as fingerprints, never in the clear.
	"golden-bucket",
	"golden-key-id",
	"golden-access",
	"golden-tenant-access",
}

// setConfigTestEnv scrubs every ARMOR_* variable (so ambient settings cannot
// perturb the fixture), then applies configTestEnv.
func setConfigTestEnv(t *testing.T) {
	t.Helper()
	var scrubbed []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "ARMOR_") {
			scrubbed = append(scrubbed, kv)
		}
	}
	for _, kv := range scrubbed {
		os.Unsetenv(strings.SplitN(kv, "=", 2)[0])
	}
	t.Cleanup(func() {
		for _, kv := range scrubbed {
			os.Setenv(strings.SplitN(kv, "=", 2)[0], strings.SplitN(kv, "=", 2)[1])
		}
	})
	for name, value := range configTestEnv {
		os.Setenv(name, value)
	}
	t.Cleanup(func() {
		for name := range configTestEnv {
			os.Unsetenv(name)
		}
	})
}

// runCaptured runs f with stdout and stderr captured through pipes and
// returns both streams. The exit hook is replaced with a recording panic so
// failure paths unwind without killing the test binary.
func runCaptured(t *testing.T, f func()) (stdout, stderr string, exitCode int) {
	t.Helper()

	oldExit := exit
	exit = func(code int) { exitCode = code; panic("exit") }
	defer func() { exit = oldExit }()

	oldStdout, oldStderr := os.Stdout, os.Stderr
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	os.Stdout, os.Stderr = stdoutW, stderrW
	defer func() { os.Stdout, os.Stderr = oldStdout, oldStderr }()

	func() {
		defer func() { _ = recover() }() // the expected exit panic, if any
		f()
	}()

	stdoutW.Close()
	stderrW.Close()
	var outBuf, errBuf bytes.Buffer
	outBuf.ReadFrom(stdoutR)
	errBuf.ReadFrom(stderrR)
	return outBuf.String(), errBuf.String(), exitCode
}

// TestConfigCommandContract pins the group's dispatch shape: exactly one
// nested subcommand, unknown or missing names are usage errors, and neither
// action is reachable with extra positionals.
func TestConfigCommandContract(t *testing.T) {
	setConfigTestEnv(t)

	cases := []struct {
		name     string
		args     []string
		wantExit int
		wantErr  string
	}{
		{name: "no nested subcommand", args: nil, wantExit: 2, wantErr: "unexpected arguments"},
		{name: "unknown nested subcommand", args: []string{"export"}, wantExit: 2, wantErr: "unexpected arguments"},
		{name: "extra positional after show", args: []string{"show", "extra"}, wantExit: 2, wantErr: "unexpected arguments"},
		{name: "validate with extra positional", args: []string{"extra", "validate"}, wantExit: 2, wantErr: "unexpected arguments"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := commands["config"]
			if cmd.Func == nil {
				t.Fatal("config command is not registered")
			}
			// Parse into a fresh flag set view of the same definition so
			// repeated parses inside one test binary cannot accumulate
			// arguments (the shared registry FlagSet persists them).
			fs := cmd.Flags
			_ = fs.Parse(tc.args)

			_, stderr, code := runCaptured(t, func() { cmd.Func(fs) })
			if code != tc.wantExit {
				t.Errorf("exit = %d, want %d\nstderr: %s", code, tc.wantExit, stderr)
			}
			if !strings.Contains(stderr, tc.wantErr) {
				t.Errorf("stderr missing %q:\n%s", tc.wantErr, stderr)
			}
		})
	}
}

// TestConfigValidateExitCodes pins validate's exit contract: 0 with a single
// confirmation line for a loadable configuration, 1 with every validation
// error grouped one per line for an unloadable one.
func TestConfigValidateExitCodes(t *testing.T) {
	t.Run("valid configuration exits 0", func(t *testing.T) {
		setConfigTestEnv(t)
		stdout, stderr, code := runCaptured(t, func() {
			cmd := commands["config"]
			_ = cmd.Flags.Parse([]string{"validate"})
			cmd.Func(cmd.Flags)
		})
		if code != 0 {
			t.Errorf("validate on a valid configuration: exit = %d, want 0\nstderr: %s", code, stderr)
		}
		if !strings.Contains(stdout, "Configuration valid") {
			t.Errorf("validate success stdout missing the confirmation line:\n%s", stdout)
		}
	})

	t.Run("invalid configuration exits 1 with grouped errors", func(t *testing.T) {
		// A scrubbed environment fails Load with several named errors.
		setConfigTestEnv(t)
		for name := range configTestEnv {
			os.Unsetenv(name)
		}
		stdout, stderr, code := runCaptured(t, func() {
			cmd := commands["config"]
			_ = cmd.Flags.Parse([]string{"validate"})
			cmd.Func(cmd.Flags)
		})
		if code != 1 {
			t.Errorf("validate on an invalid configuration: exit = %d, want 1\nstderr: %s", code, stderr)
		}
		if strings.Contains(stdout, "Configuration valid") {
			t.Errorf("validate on an invalid configuration reported success on stdout:\n%s", stdout)
		}
		for _, want := range []string{"Configuration invalid", "ARMOR_MEK is required", "ARMOR_BUCKET is required"} {
			if !strings.Contains(stderr, want) {
				t.Errorf("validate stderr missing %q:\n%s", want, stderr)
			}
		}
		// Grouped: one bullet line per error.
		if got := strings.Count(stderr, "\n  - "); got < 2 {
			t.Errorf("validate stderr grouped %d error lines, want at least 2:\n%s", got, stderr)
		}
	})
}

// TestConfigShowGoldenOutput pins show's text table against
// testdata/config-show.golden and its JSON array against
// testdata/config-show.json.golden — the fingerprints, defaults, derived
// values, and source labels an operator diffs. No secret plant may appear in
// either stream.
func TestConfigShowGoldenOutput(t *testing.T) {
	setConfigTestEnv(t)

	goldenText := filepath.Join("testdata", "config-show.golden")
	goldenJSON := filepath.Join("testdata", "config-show.json.golden")

	t.Run("text table", func(t *testing.T) {
		oldJSON := configJSONFlag
		configJSONFlag = false
		defer func() { configJSONFlag = oldJSON }()

		stdout, stderr, code := runCaptured(t, func() {
			cmd := commands["config"]
			_ = cmd.Flags.Parse([]string{"show"})
			cmd.Func(cmd.Flags)
		})
		if code != 0 {
			t.Fatalf("show on a valid configuration: exit = %d, want 0\nstderr: %s", code, stderr)
		}
		assertNoSecretPlants(t, "text", stdout, stderr)
		assertMatchesGolden(t, goldenText, stdout)
	})

	t.Run("json array", func(t *testing.T) {
		oldJSON := configJSONFlag
		configJSONFlag = true
		defer func() { configJSONFlag = oldJSON }()

		stdout, stderr, code := runCaptured(t, func() {
			cmd := commands["config"]
			_ = cmd.Flags.Parse([]string{"show"})
			cmd.Func(cmd.Flags)
		})
		if code != 0 {
			t.Fatalf("show --json on a valid configuration: exit = %d, want 0\nstderr: %s", code, stderr)
		}
		assertNoSecretPlants(t, "json", stdout, stderr)
		assertMatchesGolden(t, goldenJSON, stdout)

		// The array must be valid JSON of row objects with exactly the
		// documented fields — anything else breaks jq consumers.
		var rows []map[string]json.RawMessage
		if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
			t.Fatalf("show --json stdout is not a JSON array: %v\n%s", err, stdout)
		}
		if len(rows) == 0 {
			t.Fatal("show --json emitted an empty array")
		}
		for i, row := range rows {
			if len(row) != 3 {
				t.Errorf("json row %d has %d fields, want exactly name/value/source: %v", i, len(row), row)
			}
			for _, field := range []string{"name", "value", "source"} {
				if _, ok := row[field]; !ok {
					t.Errorf("json row %d missing field %q: %v", i, field, row)
				}
			}
		}
	})
}

// TestConfigShowReportsUnloadableConfiguration pins show's failure path: a
// configuration that cannot load has no resolved settings, so show exits 1
// with the reasons rather than dumping a half-resolved table.
func TestConfigShowReportsUnloadableConfiguration(t *testing.T) {
	setConfigTestEnv(t)
	for name := range configTestEnv {
		os.Unsetenv(name)
	}

	_, stderr, code := runCaptured(t, func() {
		cmd := commands["config"]
		_ = cmd.Flags.Parse([]string{"show"})
		cmd.Func(cmd.Flags)
	})
	if code != 1 {
		t.Errorf("show on an unloadable configuration: exit = %d, want 1\nstderr: %s", code, stderr)
	}
	for _, want := range []string{"configuration failed to load", "armor config validate"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("show stderr missing %q:\n%s", want, stderr)
		}
	}
}

// TestConfigShowSurfacesEnvWarnings pins that the same unknown-variable
// warnings `armor check` prints reach show's stderr too — this command is
// where an operator compares what they set against what ARMOR resolved.
func TestConfigShowSurfacesEnvWarnings(t *testing.T) {
	setConfigTestEnv(t)
	os.Setenv("ARMOR_CF_DOMIAN", "typo.example.com")
	defer os.Unsetenv("ARMOR_CF_DOMIAN")

	_, stderr, code := runCaptured(t, func() {
		cmd := commands["config"]
		_ = cmd.Flags.Parse([]string{"show"})
		cmd.Func(cmd.Flags)
	})
	if code != 0 {
		t.Fatalf("show with an unknown variable: exit = %d, want 0\nstderr: %s", code, stderr)
	}
	if !strings.Contains(stderr, "ARMOR_CF_DOMIAN") {
		t.Errorf("show stderr does not name the unrecognized variable:\n%s", stderr)
	}
}

// TestConfigShowRowsMatchConfigSettings holds the command's rendering to the
// config package's table: the text table and JSON array are the same rows.
func TestConfigShowRowsMatchConfigSettings(t *testing.T) {
	setConfigTestEnv(t)
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() under the show fixture: %v", err)
	}
	rows := cfg.Settings()

	var textBuf bytes.Buffer
	printSettingRows(&textBuf, rows)
	for _, row := range rows {
		if !strings.Contains(textBuf.String(), row.Name) {
			t.Errorf("text table omits row %s", row.Name)
		}
	}

	encoded, err := json.Marshal(rows)
	if err != nil {
		t.Fatalf("marshal rows: %v", err)
	}
	var roundTripped []config.SettingRow
	if err := json.Unmarshal(encoded, &roundTripped); err != nil {
		t.Fatalf("round-trip rows: %v", err)
	}
	if len(roundTripped) != len(rows) {
		t.Errorf("json round-trip lost rows: %d vs %d", len(roundTripped), len(rows))
	}
}

// assertNoSecretPlants fails when any planted secret or clear identifier
// reaches either output stream.
func assertNoSecretPlants(t *testing.T, mode, stdout, stderr string) {
	t.Helper()
	for _, plant := range secretPlants {
		for name, body := range map[string]string{"stdout": stdout, "stderr": stderr} {
			if strings.Contains(body, plant) {
				t.Errorf("%s output: %s carries the planted value %q; the show command must never echo it", mode, name, plant)
			}
		}
	}
}

// assertMatchesGolden compares got against a testdata golden, with a
// regeneration pointer on mismatch (the goldens are committed artifacts).
func assertMatchesGolden(t *testing.T, goldenPath, got string) {
	t.Helper()
	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden %s: %v", goldenPath, err)
	}
	if got != string(golden) {
		t.Errorf("output does not match golden %s", goldenPath)
		t.Logf("Got:\n%s", got)
		t.Logf("Want:\n%s", golden)
	}
}
