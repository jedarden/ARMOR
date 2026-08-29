//go:build !integration
// +build !integration

// Tests for 'armor client-config' command
package main

import (
	"flag"
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
			// Save original flag state
			oldForFlag := forFlag
			oldEndpointFlag := endpointFlag
			oldBucketFlag := bucketFlag
			oldCredentialFlag := credentialFlag
			defer func() {
				forFlag = oldForFlag
				endpointFlag = oldEndpointFlag
				bucketFlag = oldBucketFlag
				credentialFlag = oldCredentialFlag
				flag.CommandLine = flag.NewFlagSet("test", flag.ContinueError)
			}()

			// Reset flags to defaults
			forFlag = ""
			endpointFlag = ""
			bucketFlag = ""
			credentialFlag = ""

			// Capture exit
			var exitCode int
			var exitMsg strings.Builder
			exit = func(code int) {
				exitCode = code
				fmt.Fprintf(&exitMsg, "exited with code %d", code)
			}

			// Run client-config with flags
			os.Args = append([]string{"armor", "client-config"}, tt.flags...)

			// This will panic if exit is called, which we expect
			func() {
				defer func() {
					if r := recover(); r != nil {
						if exitCode != tt.wantExit {
							t.Errorf("unexpected exit code: got %d, want %d", exitCode, tt.wantExit)
						}
						msg := exitMsg.String()
						if !strings.Contains(msg, tt.wantErr) {
							t.Errorf("error message does not contain expected text: got %q, want to contain %q", msg, tt.wantErr)
						}
					}
				}()
				clientConfig()
			}()
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
		name           string
		tool           string
		endpoint       string
		bucket         string
		credential     string
		formatVersion  int
		goldenFile     string
	}{
		{
			name:      "aws-cli",
			tool:      "aws-cli",
			endpoint:  "http://localhost:9000",
			bucket:    "my-bucket",
			formatVersion: 2,
			goldenFile: "testdata/client-config-aws-cli-v2.golden",
		},
		{
			name:      "aws-cli v3",
			tool:      "aws-cli",
			endpoint:  "http://localhost:9000",
			bucket:    "my-bucket",
			formatVersion: 3,
			goldenFile: "testdata/client-config-aws-cli-v3.golden",
		},
		{
			name:      "rclone",
			tool:      "rclone",
			endpoint:  "http://localhost:9000",
			bucket:    "my-bucket",
			formatVersion: 2,
			goldenFile: "testdata/client-config-rclone-v2.golden",
		},
		{
			name:      "rclone v3",
			tool:      "rclone",
			endpoint:  "http://localhost:9000",
			bucket:    "my-bucket",
			formatVersion: 3,
			goldenFile: "testdata/client-config-rclone-v3.golden",
		},
		{
			name:      "boto3",
			tool:      "boto3",
			endpoint:  "http://localhost:9000",
			bucket:    "my-bucket",
			credential: "backup-writer",
			formatVersion: 2,
			goldenFile: "testdata/client-config-boto3-v2.golden",
		},
		{
			name:      "duckdb",
			tool:      "duckdb",
			endpoint:  "http://localhost:9000",
			bucket:    "parquet-bucket",
			formatVersion: 2,
			goldenFile: "testdata/client-config-duckdb-v2.golden",
		},
		{
			name:      "litestream",
			tool:      "litestream",
			endpoint:  "http://armor.example.com:9000",
			bucket:    "sqlite-backups",
			formatVersion: 2,
			goldenFile: "testdata/client-config-litestream-v2.golden",
		},
		{
			name:      "barman",
			tool:      "barman",
			endpoint:  "http://localhost:9000",
			bucket:    "postgres-backups",
			formatVersion: 2,
			goldenFile: "testdata/client-config-barman-v2.golden",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set format version
			if tt.formatVersion == 3 {
				os.Setenv("ARMOR_FORMAT_VERSION", "3")
			} else {
				os.Unsetenv("ARMOR_FORMAT_VERSION")
			}
			defer os.Unsetenv("ARMOR_FORMAT_VERSION")

			// Set flags
			forFlag = tt.tool
			endpointFlag = tt.endpoint
			bucketFlag = tt.bucket
			credentialFlag = tt.credential

			// Capture output
			var output strings.Builder
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			clientConfig()

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

			var output strings.Builder
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			clientConfig()

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
