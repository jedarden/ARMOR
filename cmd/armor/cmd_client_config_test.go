//go:build !integration
// +build !integration

// Tests for 'armor client-config' command
package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
)

// TestClientConfigInvalidFlags tests that the client-config command validates flags correctly
func TestClientConfigInvalidFlags(t *testing.T) {
	tests := []struct {
		name     string
		flags    []string
		wantExit int
		wantErr  string
	}{
		{
			name:     "missing -for flag",
			flags:    []string{"-endpoint", "http://localhost:9000"},
			wantExit: 2,
			wantErr:  "-for is required",
		},
		{
			name:     "missing -endpoint flag",
			flags:    []string{"-for", "aws-cli"},
			wantExit: 2,
			wantErr:  "-endpoint is required",
		},
		{
			name:     "unknown tool",
			flags:    []string{"-for", "unknown-tool", "-endpoint", "http://localhost:9000"},
			wantExit: 2,
			wantErr:  "unknown tool",
		},
		{
			name:     "unexpected arguments",
			flags:    []string{"-for", "aws-cli", "-endpoint", "http://localhost:9000", "extra-arg"},
			wantExit: 2,
			wantErr:  "unexpected arguments",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original state — the flag storage vars, the exit stand-in
			// and stderr, all of which clientConfig() consumes directly.
			oldForFlag := forFlag
			oldEndpointFlag := endpointFlag
			oldBucketFlag := bucketFlag
			oldCredentialFlag := credentialFlag
			oldExit := exit
			oldStderr := os.Stderr
			defer func() {
				forFlag = oldForFlag
				endpointFlag = oldEndpointFlag
				bucketFlag = oldBucketFlag
				credentialFlag = oldCredentialFlag
				exit = oldExit
				os.Stderr = oldStderr
			}()

			// Reset flags to defaults
			forFlag = ""
			endpointFlag = ""
			bucketFlag = ""
			credentialFlag = ""

			// Capture exit. clientConfig doesn't return after a bad-flags
			// exit(2) the way it would after a real os.Exit, so this panics
			// (matching the recover() below) to unwind immediately -- os.Exit
			// itself can't be called here, it would kill the whole test binary.
			var exitCode int
			var exitMsg strings.Builder
			exit = func(code int) {
				exitCode = code
				fmt.Fprintf(&exitMsg, "exited with code %d", code)
				panic("exit")
			}

			// Capture stderr too: the wantErr assertions match the error text
			// clientConfig writes there, which the exit stand-in never sees.
			stderrR, stderrW, _ := os.Pipe()
			os.Stderr = stderrW

			// Dispatch the way main does: parse this case's flags into
			// client-config's own FlagSet, then run the command.
			cmd := commands["client-config"]
			_ = cmd.Flags.Parse(tt.flags)
			func() {
				defer func() { _ = recover() }() // the expected exit panic
				cmd.Func(cmd.Flags)
			}()

			stderrW.Close()
			os.Stderr = oldStderr
			var stderrBuf bytes.Buffer
			stderrBuf.ReadFrom(stderrR)

			if exitCode != tt.wantExit {
				t.Errorf("unexpected exit code: got %d, want %d", exitCode, tt.wantExit)
			}
			msg := stderrBuf.String() + exitMsg.String()
			if !strings.Contains(msg, tt.wantErr) {
				t.Errorf("error message does not contain expected text: got %q, want to contain %q", msg, tt.wantErr)
			}
		})
	}
}

// TestClientConfigGoldenFiles tests that the generated configs match the golden files
func TestClientConfigGoldenFiles(t *testing.T) {
	// Set up minimal environment for config loading
	os.Setenv("ARMOR_BUCKET", "test-bucket")
	os.Setenv("ARMOR_MEK", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	os.Setenv("ARMOR_B2_REGION", "us-east-005")
	os.Setenv("ARMOR_B2_ACCESS_KEY_ID", "test-key-id")
	os.Setenv("ARMOR_B2_SECRET_ACCESS_KEY", "test-secret-key")
	os.Setenv("ARMOR_AUTH_ACCESS_KEY", "test-auth-key")
	os.Setenv("ARMOR_AUTH_SECRET_KEY", "test-auth-secret")
	defer func() {
		os.Unsetenv("ARMOR_BUCKET")
		os.Unsetenv("ARMOR_MEK")
		os.Unsetenv("ARMOR_B2_REGION")
		os.Unsetenv("ARMOR_B2_ACCESS_KEY_ID")
		os.Unsetenv("ARMOR_B2_SECRET_ACCESS_KEY")
		os.Unsetenv("ARMOR_AUTH_ACCESS_KEY")
		os.Unsetenv("ARMOR_AUTH_SECRET_KEY")
	}()

	tests := []struct {
		name          string
		tool          string
		endpoint      string
		bucket        string
		credential    string
		formatVersion int
		goldenFile    string
	}{
		{
			name:          "aws-cli",
			tool:          "aws-cli",
			endpoint:      "http://localhost:9000",
			bucket:        "my-bucket",
			formatVersion: 2,
			goldenFile:    "testdata/client-config-aws-cli-v2.golden",
		},
		{
			name:          "aws-cli v3",
			tool:          "aws-cli",
			endpoint:      "http://localhost:9000",
			bucket:        "my-bucket",
			formatVersion: 3,
			goldenFile:    "testdata/client-config-aws-cli-v3.golden",
		},
		{
			name:          "rclone",
			tool:          "rclone",
			endpoint:      "http://localhost:9000",
			bucket:        "my-bucket",
			formatVersion: 2,
			goldenFile:    "testdata/client-config-rclone-v2.golden",
		},
		{
			name:          "rclone v3",
			tool:          "rclone",
			endpoint:      "http://localhost:9000",
			bucket:        "my-bucket",
			formatVersion: 3,
			goldenFile:    "testdata/client-config-rclone-v3.golden",
		},
		{
			name:          "boto3",
			tool:          "boto3",
			endpoint:      "http://localhost:9000",
			bucket:        "my-bucket",
			credential:    "backup-writer",
			formatVersion: 2,
			goldenFile:    "testdata/client-config-boto3-v2.golden",
		},
		{
			name:          "duckdb",
			tool:          "duckdb",
			endpoint:      "http://localhost:9000",
			bucket:        "parquet-bucket",
			formatVersion: 2,
			goldenFile:    "testdata/client-config-duckdb-v2.golden",
		},
		{
			name:          "litestream",
			tool:          "litestream",
			endpoint:      "http://armor.example.com:9000",
			bucket:        "sqlite-backups",
			formatVersion: 2,
			goldenFile:    "testdata/client-config-litestream-v2.golden",
		},
		{
			name:          "barman",
			tool:          "barman",
			endpoint:      "http://localhost:9000",
			bucket:        "postgres-backups",
			formatVersion: 2,
			goldenFile:    "testdata/client-config-barman-v2.golden",
		},
		{
			name:          "boto3 v3",
			tool:          "boto3",
			endpoint:      "http://localhost:9000",
			bucket:        "my-bucket",
			credential:    "backup-writer",
			formatVersion: 3,
			goldenFile:    "testdata/client-config-boto3-v3.golden",
		},
		{
			name:          "duckdb v3",
			tool:          "duckdb",
			endpoint:      "http://localhost:9000",
			bucket:        "parquet-bucket",
			formatVersion: 3,
			goldenFile:    "testdata/client-config-duckdb-v3.golden",
		},
		{
			name:          "litestream v3",
			tool:          "litestream",
			endpoint:      "http://armor.example.com:9000",
			bucket:        "sqlite-backups",
			formatVersion: 3,
			goldenFile:    "testdata/client-config-litestream-v3.golden",
		},
		{
			name:          "barman v3",
			tool:          "barman",
			endpoint:      "http://localhost:9000",
			bucket:        "postgres-backups",
			formatVersion: 3,
			goldenFile:    "testdata/client-config-barman-v3.golden",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set format version explicitly. The unset default is v3
			// (config.Load), so the legacy-contract cases must pin 2 —
			// unsetting would silently produce v3 output.
			if tt.formatVersion == 3 {
				os.Setenv("ARMOR_FORMAT_VERSION", "3")
			} else {
				os.Setenv("ARMOR_FORMAT_VERSION", "2")
			}
			defer os.Unsetenv("ARMOR_FORMAT_VERSION")

			// Set flags
			forFlag = tt.tool
			endpointFlag = tt.endpoint
			bucketFlag = tt.bucket
			credentialFlag = tt.credential

			// Capture output
			var output bytes.Buffer
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Dispatch with no command-line flags: the storage vars were set
			// directly above, and runCommand's Parse(nil) clears any args a
			// previous test left in the shared FlagSet.
			runCommand(t, "client-config")

			w.Close()
			os.Stdout = oldStdout

			// Read output
			output.ReadFrom(r)

			// Read golden file
			golden, err := os.ReadFile(tt.goldenFile)
			if err != nil {
				t.Fatalf("failed to read golden file %s: %v", tt.goldenFile, err)
			}

			// Compare
			if output.String() != string(golden) {
				t.Errorf("output does not match golden file %s", tt.goldenFile)
				t.Logf("Got:\n%s", output.String())
				t.Logf("Want:\n%s", golden)
			}
		})
	}
}

// Sentinel credential values pinned into the environment while the property
// tests below capture configs: cli-reference.md documents that client-config
// "never reads a credential and only ever prints variable names or
// YOUR_ACCESS_KEY_ID-style placeholders", so these values must never appear
// in any emitted config.
const (
	clientConfigSentinelAccess = "sentinel-armor-auth-access-4a220505"
	clientConfigSentinelSecret = "sentinel-armor-auth-secret-4a220505"
)

// pinClientConfigTestEnv pins the ARMOR_* environment clientConfig's
// config.Load() call needs, with sentinel auth values: every config captured
// through it doubles as a target for the value-never-emitted assertions.
func pinClientConfigTestEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ARMOR_BUCKET", "test-bucket")
	t.Setenv("ARMOR_MEK", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	t.Setenv("ARMOR_B2_REGION", "us-east-005")
	t.Setenv("ARMOR_B2_ACCESS_KEY_ID", "test-key-id")
	t.Setenv("ARMOR_B2_SECRET_ACCESS_KEY", "test-secret-key")
	t.Setenv("ARMOR_AUTH_ACCESS_KEY", clientConfigSentinelAccess)
	t.Setenv("ARMOR_AUTH_SECRET_KEY", clientConfigSentinelSecret)
}

// captureClientConfig sets the client-config flags, runs the command the way
// main dispatches it, and returns its stdout with the ARMOR_* environment
// pinned so config.Load() succeeds and the requested write format is honored.
func captureClientConfig(t *testing.T, tool, endpoint, bucket, credential, formatVersion string) string {
	t.Helper()
	pinClientConfigTestEnv(t)
	t.Setenv("ARMOR_FORMAT_VERSION", formatVersion)

	oldFor, oldEndpoint, oldBucket, oldCredential := forFlag, endpointFlag, bucketFlag, credentialFlag
	t.Cleanup(func() {
		forFlag, endpointFlag, bucketFlag, credentialFlag = oldFor, oldEndpoint, oldBucket, oldCredential
	})
	forFlag = tool
	endpointFlag = endpoint
	bucketFlag = bucket
	credentialFlag = credential

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	runCommand(t, "client-config")
	w.Close()
	os.Stdout = oldStdout

	var output bytes.Buffer
	output.ReadFrom(r)
	return output.String()
}

// TestClientConfigSigningSettings validates, per tool, the properties
// README.md's "Client configuration" section promises: the endpoint URL is
// wired into the tool's own config key, path-style addressing is set where
// the tool exposes a knob, the region placeholder is present, and
// credentials appear as variable names or placeholders only — never as the
// environment's values.
func TestClientConfigSigningSettings(t *testing.T) {
	tests := []struct {
		name            string
		tool            string
		wantEndpointOn  string   // the exact line wiring the endpoint into the tool's config
		wantAddressing  string   // the path-style addressing setting; empty = tool has no knob
		noAddressingWhy string   // required when wantAddressing is empty: why the knob is absent
		wantCredNames   []string // credential variable names/placeholders that must appear
	}{
		{
			name:           "aws-cli",
			tool:           "aws-cli",
			wantEndpointOn: "endpoint_url = https://armor.example.com:9000",
			wantAddressing: "addressing_style = path",
			wantCredNames:  []string{"ARMOR_AUTH_ACCESS_KEY_ID", "ARMOR_AUTH_SECRET_KEY"},
		},
		{
			name:           "rclone",
			tool:           "rclone",
			wantEndpointOn: "endpoint = https://armor.example.com:9000",
			wantAddressing: "s3_force_path_style = true",
			wantCredNames:  []string{"access_key_id = YOUR_ACCESS_KEY_ID", "secret_access_key = YOUR_SECRET_ACCESS_KEY"},
		},
		{
			name:           "boto3",
			tool:           "boto3",
			wantEndpointOn: `endpoint_url = "https://armor.example.com:9000"`,
			wantAddressing: "'addressing_style': 'path'",
			wantCredNames:  []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY"},
		},
		{
			name:           "duckdb",
			tool:           "duckdb",
			wantEndpointOn: "SET s3_endpoint = 'https://armor.example.com:9000';",
			wantAddressing: "s3_url_style = 'path'",
			wantCredNames:  []string{"s3_access_key_id", "s3_secret_access_key"},
		},
		{
			name:            "litestream",
			tool:            "litestream",
			wantEndpointOn:  "LITESTREAM_ENDPOINT=https://armor.example.com:9000",
			wantAddressing:  "",
			noAddressingWhy: "litestream's S3 replica is always path-style; it exposes no addressing knob",
			wantCredNames:   []string{"LITESTREAM_ACCESS_KEY_ID", "LITESTREAM_SECRET_ACCESS_KEY"},
		},
		{
			name:            "barman",
			tool:            "barman",
			wantEndpointOn:  "export AWS_ENDPOINT_URL=https://armor.example.com:9000",
			wantAddressing:  "",
			noAddressingWhy: "botocore defaults to path-style for a custom endpoint URL",
			wantCredNames:   []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := captureClientConfig(t, tt.tool, "https://armor.example.com:9000", "signing-bucket", "", "3")

			if !strings.Contains(out, tt.wantEndpointOn) {
				t.Errorf("endpoint not wired into %s config: got output without %q:\n%s", tt.tool, tt.wantEndpointOn, out)
			}
			if tt.wantAddressing != "" {
				if !strings.Contains(out, tt.wantAddressing) {
					t.Errorf("path-style addressing not set for %s: got output without %q:\n%s", tt.tool, tt.wantAddressing, out)
				}
			} else if tt.noAddressingWhy == "" {
				t.Errorf("tool %q asserts no addressing setting but documents no reason", tt.tool)
			}
			if !strings.Contains(out, "us-east-1") {
				t.Errorf("region placeholder missing from %s config:\n%s", tt.tool, out)
			}
			for _, name := range tt.wantCredNames {
				if !strings.Contains(out, name) {
					t.Errorf("credential variable name %q missing from %s config:\n%s", name, tt.tool, out)
				}
			}
			for _, sentinel := range []string{clientConfigSentinelAccess, clientConfigSentinelSecret} {
				if strings.Contains(out, sentinel) {
					t.Errorf("credential VALUE %q leaked into %s config:\n%s", sentinel, tt.tool, out)
				}
			}
		})
	}
}

// TestClientConfigNamedCredentialReference validates the -credential path for
// every tool: the named credential is referenced by name (retrieval stays by
// reference) and no credential value is ever emitted alongside it.
func TestClientConfigNamedCredentialReference(t *testing.T) {
	for _, tool := range []string{"aws-cli", "rclone", "boto3", "duckdb", "litestream", "barman"} {
		t.Run(tool, func(t *testing.T) {
			out := captureClientConfig(t, tool, "https://armor.example.com:9000", "cred-bucket", "backup-writer", "3")

			if !strings.Contains(out, "backup-writer") {
				t.Errorf("named credential not referenced in %s config:\n%s", tool, out)
			}
			for _, sentinel := range []string{clientConfigSentinelAccess, clientConfigSentinelSecret} {
				if strings.Contains(out, sentinel) {
					t.Errorf("credential VALUE %q leaked into %s config alongside named credential:\n%s", sentinel, tool, out)
				}
			}
		})
	}
}

// docFencedLines extracts the non-blank lines of the first fenced code block
// after heading (a line-prefix match) in the compatibility matrix.
func docFencedLines(t *testing.T, doc, heading string) []string {
	t.Helper()
	var (
		inSection bool
		inFence   bool
		collected []string
	)
	for _, line := range strings.Split(doc, "\n") {
		if !inSection {
			if strings.HasPrefix(line, heading) {
				inSection = true
			}
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			if inFence {
				return collected
			}
			inFence = true
			continue
		}
		if inFence && strings.TrimSpace(line) != "" {
			collected = append(collected, line)
		}
	}
	t.Fatalf("no fenced code block found under heading %q in compatibility matrix", heading)
	return nil
}

// assertDocumentedLinesEmitted checks every documented line appears in the
// emitted config, in the documented order.
func assertDocumentedLinesEmitted(t *testing.T, want []string, got string) {
	t.Helper()
	gotLines := strings.Split(got, "\n")
	cursor := 0
	for _, wantLine := range want {
		found := false
		for ; cursor < len(gotLines); cursor++ {
			if gotLines[cursor] == wantLine {
				found = true
				cursor++
				break
			}
		}
		if !found {
			t.Errorf("documented line not emitted (or emitted out of order): %q", wantLine)
		}
	}
}

// TestClientConfigDocumentedExamplesMatchOutput ties the compatibility
// matrix's "Tested configuration examples" section to the command: the
// section presents each block as what `armor client-config --for <tool>`
// emits, so every documented line must still be emitted, in the documented
// order — drift on either side fails here.
func TestClientConfigDocumentedExamplesMatchOutput(t *testing.T) {
	raw, err := os.ReadFile("../../docs/multipart-client-compatibility.md")
	if err != nil {
		t.Fatalf("read compatibility matrix: %v", err)
	}

	tests := []struct {
		heading string
		tool    string
	}{
		{heading: "### AWS CLI", tool: "aws-cli"},
		{heading: "### litestream", tool: "litestream"},
		{heading: "### barman (barman-cloud-backup)", tool: "barman"},
	}
	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			want := docFencedLines(t, string(raw), tt.heading)
			if len(want) == 0 {
				t.Fatalf("no documented lines extracted under %q", tt.heading)
			}
			// The documented examples use this endpoint and no bucket or
			// named credential; generate with the same inputs so the
			// comparison is line-for-line against what the docs show.
			out := captureClientConfig(t, tt.tool, "https://armor.example.com:9000", "", "", "3")
			assertDocumentedLinesEmitted(t, want, out)
		})
	}
}

// TestClientConfigMultipartCompatibilityBlock validates the emitted
// multipart contract block for every tool on both write formats: the
// format's contract statement, the per-tool compatibility note (which must
// be registered for every supported tool), and the pointer to the tested
// compatibility matrix.
func TestClientConfigMultipartCompatibilityBlock(t *testing.T) {
	const matrixPointer = "Tested compatibility matrix: docs/multipart-client-compatibility.md"

	for _, tool := range []string{"aws-cli", "rclone", "boto3", "duckdb", "litestream", "barman"} {
		for _, format := range []string{"2", "3"} {
			t.Run(fmt.Sprintf("%s/v%s", tool, format), func(t *testing.T) {
				out := captureClientConfig(t, tool, "http://localhost:9000", "compat-bucket", "", format)

				wantVersion := "write format v" + format
				otherVersion := "write format v3"
				if format == "3" {
					otherVersion = "write format v2"
				}
				if !strings.Contains(out, wantVersion) {
					t.Errorf("multipart block missing %q:\n%s", wantVersion, out)
				}
				if strings.Contains(out, otherVersion) {
					t.Errorf("multipart block for v%s names the other format %q:\n%s", format, otherVersion, out)
				}
				if format == "2" && !strings.Contains(out, "503 SlowDown") {
					t.Errorf("v2 multipart block missing the SlowDown deferral contract:\n%s", out)
				}
				if format == "3" && !strings.Contains(out, "no part-order or part-size") {
					t.Errorf("v3 multipart block missing the no-contract statement:\n%s", out)
				}

				note := multipartClientNotes[tool]
				if note == "" {
					t.Errorf("no compatibility note registered for supported tool %q", tool)
				} else if !strings.Contains(out, note) {
					t.Errorf("per-tool compatibility note missing from %s config:\nwant: %q\ngot:\n%s", tool, note, out)
				}
				if !strings.Contains(out, matrixPointer) {
					t.Errorf("compatibility matrix pointer missing from %s config:\n%s", tool, out)
				}
			})
		}
	}
}

// TestClientConfigToolAliases tests that tool name aliases work
func TestClientConfigToolAliases(t *testing.T) {
	os.Setenv("ARMOR_BUCKET", "test-bucket")
	os.Setenv("ARMOR_MEK", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	os.Setenv("ARMOR_B2_REGION", "us-east-005")
	os.Setenv("ARMOR_B2_ACCESS_KEY_ID", "test-key-id")
	os.Setenv("ARMOR_B2_SECRET_ACCESS_KEY", "test-secret-key")
	os.Setenv("ARMOR_AUTH_ACCESS_KEY", "test-auth-key")
	os.Setenv("ARMOR_AUTH_SECRET_KEY", "test-auth-secret")
	defer func() {
		os.Unsetenv("ARMOR_BUCKET")
		os.Unsetenv("ARMOR_MEK")
		os.Unsetenv("ARMOR_B2_REGION")
		os.Unsetenv("ARMOR_B2_ACCESS_KEY_ID")
		os.Unsetenv("ARMOR_B2_SECRET_ACCESS_KEY")
		os.Unsetenv("ARMOR_AUTH_ACCESS_KEY")
		os.Unsetenv("ARMOR_AUTH_SECRET_KEY")
	}()

	tests := []struct {
		name     string
		tool     string
		expected string
	}{
		{
			name:     "aws-cli alias",
			tool:     "aws-cli",
			expected: "AWS CLI configuration",
		},
		{
			name:     "awscli alias",
			tool:     "awscli",
			expected: "AWS CLI configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			forFlag = tt.tool
			endpointFlag = "http://localhost:9000"

			var output bytes.Buffer
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			runCommand(t, "client-config")

			w.Close()
			os.Stdout = oldStdout

			output.ReadFrom(r)

			if !strings.Contains(output.String(), tt.expected) {
				t.Errorf("output does not contain expected text %q", tt.expected)
				t.Logf("Got: %s", output.String())
			}
		})
	}
}
