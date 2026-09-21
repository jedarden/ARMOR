//go:build !integration
// +build !integration

// Tests for 'armor migrate' command
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// TestMigrateInvalidFlags tests that the migrate command validates flags correctly
func TestMigrateInvalidFlags(t *testing.T) {
	tests := []struct {
		name     string
		flags    []string
		env      map[string]string
		wantExit int
		wantErr  string
	}{
		{
			name:     "missing admin-url",
			flags:    []string{},
			env:      map[string]string{"ARMOR_ADMIN_TOKEN": "test-token"},
			wantExit: 2,
			wantErr:  "-admin-url is required",
		},
		{
			name:     "missing admin token",
			flags:    []string{"-admin-url", "http://localhost:9001"},
			env:      map[string]string{},
			wantExit: 1,
			wantErr:  "ARMOR_ADMIN_TOKEN environment variable is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original state: environment, exit stand-in, and the flag
			// storage vars migrate validates.
			oldEnv := os.Environ()
			oldExit := exit
			oldAdminURL := adminURLFlag
			defer func() {
				// Restore environment
				os.Clearenv()
				for _, v := range oldEnv {
					kv := strings.SplitN(v, "=", 2)
					if len(kv) == 2 {
						os.Setenv(kv[0], kv[1])
					}
				}
				exit = oldExit
				adminURLFlag = oldAdminURL
			}()

			// Set up test environment
			os.Clearenv()
			for k, v := range tt.env {
				os.Setenv(k, v)
			}

			// Capture exit
			var exitCode int
			var exitMsg strings.Builder
			exit = func(code int) {
				exitCode = code
				fmt.Fprintf(&exitMsg, "exited with code %d", code)
				panic("exit")
			}

			// Capture stderr too: migrate writes its validation errors
			// there, and the exit stand-in only records the code.
			oldStderr := os.Stderr
			stderrR, stderrW, _ := os.Pipe()
			os.Stderr = stderrW
			defer func() { os.Stderr = oldStderr }()

			// The missing-admin-url case must see an empty flag, not whatever
			// an earlier test parsed into the shared storage var.
			adminURLFlag = ""

			// Dispatch the way main does: parse this case's flags into
			// migrate's own FlagSet, then run the command.
			cmd := commands["migrate"]
			_ = cmd.Flags.Parse(tt.flags)

			// This will panic if exit is called, which we expect
			defer func() {
				if r := recover(); r != nil {
					stderrW.Close()
					var stderrBuf bytes.Buffer
					stderrBuf.ReadFrom(stderrR)
					if exitCode != tt.wantExit {
						t.Errorf("unexpected exit code: got %d, want %d", exitCode, tt.wantExit)
					}
					msg := stderrBuf.String() + exitMsg.String()
					if !strings.Contains(msg, tt.wantErr) {
						t.Errorf("error message does not contain expected text: got %q, want to contain %q", msg, tt.wantErr)
					}
				}
			}()

			cmd.Func(cmd.Flags)
		})
	}
}

// TestMigrateServerInteraction tests interaction with a mock admin server
func TestMigrateServerInteraction(t *testing.T) {
	// Set admin token in environment
	os.Setenv("ARMOR_ADMIN_TOKEN", "test-admin-token")

	tests := []struct {
		name           string
		flags          []string
		handler        func(http.ResponseWriter, *http.Request)
		wantStatusCode int
		wantExit       int
		verifyOutput   func(t *testing.T, output string)
	}{
		{
			name:  "successful migration start",
			flags: []string{"-admin-url", ""},
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("expected POST request, got %s", r.Method)
				}
				if r.URL.Path != "/admin/format/migrate" {
					t.Errorf("expected path /admin/format/migrate, got %s", r.URL.Path)
				}

				// Check auth header
				auth := r.Header.Get("Authorization")
				if !strings.HasPrefix(auth, "Bearer ") {
					t.Errorf("expected Bearer token, got %s", auth)
				}
				if !strings.HasSuffix(auth, "test-admin-token") {
					t.Errorf("expected token test-admin-token, got %s", auth)
				}

				// Return success response
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"status":            "in_progress",
					"total_objects":     1000,
					"processed_objects": 0,
					"failed_objects":    0,
					"dry_run":           false,
				})
			},
			wantStatusCode: http.StatusOK,
			wantExit:       0, // Should exit 0 in non-watch mode
			verifyOutput: func(t *testing.T, output string) {
				if !strings.Contains(output, "Migration started successfully") {
					t.Errorf("output does not contain success message: %s", output)
				}
			},
		},
		{
			name:  "dry-run migration",
			flags: []string{"-admin-url", "", "-dry-run"},
			handler: func(w http.ResponseWriter, r *http.Request) {
				// Check dry_run parameter
				if r.URL.Query().Get("dry_run") != "true" {
					t.Errorf("expected dry_run=true, got %s", r.URL.Query().Get("dry_run"))
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"status":  "in_progress",
					"dry_run": true,
				})
			},
			wantStatusCode: http.StatusOK,
			wantExit:       0,
		},
		{
			name:  "migration with include versions",
			flags: []string{"-admin-url", "", "-include", "v1,v2"},
			handler: func(w http.ResponseWriter, r *http.Request) {
				include := r.URL.Query().Get("include")
				if include != "v1,v2" {
					t.Errorf("expected include=v1,v2, got %s", include)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"status": "in_progress",
				})
			},
			wantStatusCode: http.StatusOK,
			wantExit:       0,
		},
		{
			name:  "migration with explicit target",
			flags: []string{"-admin-url", "", "-target", "v3", "-include", "v1,v2"},
			handler: func(w http.ResponseWriter, r *http.Request) {
				target := r.URL.Query().Get("target")
				if target != "v3" {
					t.Errorf("expected target=v3, got %s", target)
				}
				include := r.URL.Query().Get("include")
				if include != "v1,v2" {
					t.Errorf("expected include=v1,v2, got %s", include)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"status": "in_progress",
				})
			},
			wantStatusCode: http.StatusOK,
			wantExit:       0,
		},
		{
			name:  "migration with concurrency",
			flags: []string{"-admin-url", "", "-concurrency", "8"},
			handler: func(w http.ResponseWriter, r *http.Request) {
				concurrency := r.URL.Query().Get("concurrency")
				if concurrency != "8" {
					t.Errorf("expected concurrency=8, got %s", concurrency)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"status": "in_progress",
				})
			},
			wantStatusCode: http.StatusOK,
			wantExit:       0,
		},
		{
			name:  "server error response",
			flags: []string{"-admin-url", ""},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("internal server error"))
			},
			wantStatusCode: http.StatusInternalServerError,
			wantExit:       1,
		},
		{
			name:  "unauthorized response",
			flags: []string{"-admin-url", ""},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			wantStatusCode: http.StatusUnauthorized,
			wantExit:       1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(tt.handler))
			defer server.Close()

			// Substitute the -admin-url placeholder value with the test
			// server's URL. (The earlier rewrite loop appended the URL twice
			// for the ["-admin-url", ""] pattern, leaving a stray positional
			// argument that migrate rejects with "unexpected arguments"
			// before ever reaching the server.)
			flags := make([]string, 0, len(tt.flags))
			for i := 0; i < len(tt.flags); i++ {
				if tt.flags[i] == "-admin-url" && i+1 < len(tt.flags) && tt.flags[i+1] == "" {
					flags = append(flags, "-admin-url", server.URL)
					i++ // skip the placeholder value
				} else {
					flags = append(flags, tt.flags[i])
				}
			}

			// Set up for output capture
			oldStderr := os.Stderr
			oldStdout := os.Stdout
			rerr, werr, _ := os.Pipe()
			rout, wout, _ := os.Pipe()
			os.Stderr = werr
			os.Stdout = wout

			// Capture exit
			var exitCode int
			oldExit := exit
			exit = func(code int) {
				exitCode = code
				panic("exit")
			}

			// Defer cleanup
			defer func() {
				exit = oldExit
				os.Stderr = oldStderr
				os.Stdout = oldStdout
				werr.Close()
				wout.Close()
			}()

			// Dispatch the way main does: parse this case's flags into
			// migrate's own FlagSet, then run the command. (An earlier
			// revision of this test swapped in a fresh empty flag.CommandLine
			// here, which silently dropped every flag registration.)
			cmd := commands["migrate"]
			_ = cmd.Flags.Parse(flags)
			func() {
				defer func() {
					recover() // Expected exit via panic
				}()
				cmd.Func(cmd.Flags)
			}()

			// Restore output and read captured output
			werr.Close()
			wout.Close()
			os.Stderr = oldStderr
			os.Stdout = oldStdout

			var stderrBuf, stdoutBuf strings.Builder
			io.Copy(&stderrBuf, rerr)
			io.Copy(&stdoutBuf, rout)

			stderr := stderrBuf.String()
			stdout := stdoutBuf.String()

			// Verify exit code
			if exitCode != tt.wantExit {
				t.Errorf("unexpected exit code: got %d, want %d\nstderr: %s\nstdout: %s", exitCode, tt.wantExit, stderr, stdout)
			}

			// Verify output
			if tt.verifyOutput != nil {
				tt.verifyOutput(t, stderr+stdout)
			}
		})
	}
}

// TestMigrateWatchMode tests the --watch flag behavior
func TestMigrateWatchMode(t *testing.T) {
	os.Setenv("ARMOR_ADMIN_TOKEN", "test-admin-token")

	// Create a channel to signal the test server to advance states
	stateChan := make(chan string, 10)

	// Track the number of GET requests
	getCount := 0

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			// Initial migration start
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":            "in_progress",
				"total_objects":     100,
				"processed_objects": 0,
				"failed_objects":    0,
				"dry_run":           false,
			})
			stateChan <- "started"
			return
		}

		if r.Method == http.MethodGet {
			getCount++

			// Serve state based on channel
			select {
			case state := <-stateChan:
				w.Header().Set("Content-Type", "application/json")

				switch state {
				case "started":
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"status":            "in_progress",
						"total_objects":     100,
						"processed_objects": 10,
						"skipped_objects":   2,
						"failed_objects":    0,
						"dry_run":           false,
					})
					stateChan <- "progress"
				case "progress":
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"status":            "in_progress",
						"total_objects":     100,
						"processed_objects": 50,
						"skipped_objects":   5,
						"failed_objects":    1,
						"dry_run":           false,
					})
					stateChan <- "nearly_done"
				case "nearly_done":
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"status":            "in_progress",
						"total_objects":     100,
						"processed_objects": 95,
						"skipped_objects":   5,
						"failed_objects":    1,
						"dry_run":           false,
					})
					stateChan <- "completed"
				case "completed":
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"status":            "completed",
						"total_objects":     100,
						"processed_objects": 94,
						"skipped_objects":   5,
						"failed_objects":    1,
						"dry_run":           false,
					})
					// Don't send another state - migration is done
				default:
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"status":  "no_migration",
						"message": "No migration in progress",
					})
				}
			case <-time.After(5 * time.Second):
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"status":  "no_migration",
					"message": "No migration in progress",
				})
			}
			return
		}

		w.WriteHeader(http.StatusMethodNotAllowed)
	}

	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	// Set up for output capture
	oldStderr := os.Stderr
	oldStdout := os.Stdout
	rout, wout, _ := os.Pipe()
	os.Stderr = wout
	os.Stdout = wout

	// Capture exit
	var exitCode int
	oldExit := exit
	exit = func(code int) {
		exitCode = code
		panic("exit")
	}

	// Defer cleanup
	defer func() {
		exit = oldExit
		os.Stderr = oldStderr
		os.Stdout = oldStdout
		wout.Close()
	}()

	// Dispatch the way main does: parse --watch mode's flags into migrate's
	// own FlagSet, then run the command.
	cmd := commands["migrate"]
	_ = cmd.Flags.Parse([]string{"-admin-url", server.URL, "-watch"})

	// Run migrate in goroutine with timeout
	done := make(chan struct{})
	go func() {
		defer func() {
			recover() // Expected exit via panic
			close(done)
		}()
		cmd.Func(cmd.Flags)
	}()

	// Wait for migration to complete or timeout
	select {
	case <-done:
		// Migration completed
	case <-time.After(10 * time.Second):
		t.Fatal("migration did not complete within timeout")
	}

	// Restore output
	wout.Close()
	os.Stderr = oldStderr
	os.Stdout = oldStdout

	var outputBuf strings.Builder
	io.Copy(&outputBuf, rout)
	output := outputBuf.String()

	// Verify that we got multiple GET requests (at least 3: progress, nearly_done, completed)
	if getCount < 3 {
		t.Errorf("expected at least 3 GET requests, got %d", getCount)
	}

	// Verify output contains progress information
	if !strings.Contains(output, "Progress:") {
		t.Errorf("output does not contain progress information: %s", output)
	}

	// Verify that we detected the failure
	if !strings.Contains(output, "failed") {
		t.Errorf("output does not mention failures: %s", output)
	}

	// Verify exit code is non-zero due to failures
	if exitCode != 1 {
		t.Errorf("expected exit code 1 due to failures, got %d", exitCode)
	}
}

// TestMigrateWatchModeNoFailures tests watch mode with successful migration
func TestMigrateWatchModeNoFailures(t *testing.T) {
	os.Setenv("ARMOR_ADMIN_TOKEN", "test-admin-token")

	stateChan := make(chan string, 10)

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":            "in_progress",
				"total_objects":     10,
				"processed_objects": 0,
				"failed_objects":    0,
			})
			stateChan <- "started"
			return
		}

		if r.Method == http.MethodGet {
			select {
			case state := <-stateChan:
				w.Header().Set("Content-Type", "application/json")

				switch state {
				case "started":
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"status":            "completed",
						"total_objects":     10,
						"processed_objects": 10,
						"skipped_objects":   0,
						"failed_objects":    0,
						"classification": map[string]interface{}{
							"v1_single_put":            4,
							"v1_multipart":             0,
							"v2_single_put":            0,
							"v2_multipart":             0,
							"v3":                       6,
							"non_armor":                0,
							"malformed":                0,
							"contradictory":            0,
							"size_lt_1mb":              10,
							"size_1mb_to_10mb":         0,
							"size_10mb_to_100mb":       0,
							"size_100mb_to_1gb":        0,
							"size_1gb_to_10gb":         0,
							"size_gt_10gb":             0,
							"by_key_fingerprint":       map[string]interface{}{"0123456789abcdef": 10},
							"outcome_processed":        10,
							"outcome_skipped":          0,
							"outcome_failed":           0,
							"outcome_integrity_failed": 0,
						},
					})
				default:
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"status":  "no_migration",
						"message": "No migration in progress",
					})
				}
			case <-time.After(5 * time.Second):
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"status":  "no_migration",
					"message": "No migration in progress",
				})
			}
			return
		}

		w.WriteHeader(http.StatusMethodNotAllowed)
	}

	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	// Set up for output capture
	oldStderr := os.Stderr
	oldStdout := os.Stdout
	rout, wout, _ := os.Pipe()
	os.Stderr = wout
	os.Stdout = wout

	var exitCode int
	oldExit := exit
	exit = func(code int) {
		exitCode = code
		panic("exit")
	}

	defer func() {
		exit = oldExit
		os.Stderr = oldStderr
		os.Stdout = oldStdout
		wout.Close()
	}()

	// Dispatch the way main does: parse --watch mode's flags into migrate's
	// own FlagSet, then run the command.
	cmd := commands["migrate"]
	_ = cmd.Flags.Parse([]string{"-admin-url", server.URL, "-watch"})

	done := make(chan struct{})
	go func() {
		defer func() {
			recover()
			close(done)
		}()
		cmd.Func(cmd.Flags)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("migration did not complete within timeout")
	}

	wout.Close()
	os.Stderr = oldStderr
	os.Stdout = oldStdout

	var outputBuf strings.Builder
	io.Copy(&outputBuf, rout)
	output := outputBuf.String()

	if !strings.Contains(output, "completed successfully") {
		t.Errorf("output does not contain success message: %s", output)
	}

	// The completed state's classification must render human-readable, one
	// line per dimension.
	for _, want := range []string{
		"source:", "v1-single=4", "v3=6",
		"size:", "<1MB=10",
		"keys:", "0123456789abcdef=10",
		"outcome:", "processed=10",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("output does not contain classification report line %q:\n%s", want, output)
		}
	}

	// Verify exit code is 0 (success)
	if exitCode != 0 {
		t.Errorf("expected exit code 0 for successful migration, got %d", exitCode)
	}
}

// TestParseMigrationStateClassification checks that the per-dimension count
// report decodes off the polled state, and that a state without one (older
// server) leaves Classification nil rather than erroring.
func TestParseMigrationStateClassification(t *testing.T) {
	m := map[string]interface{}{
		"status":            "completed",
		"total_objects":     3,
		"processed_objects": 1,
		"skipped_objects":   1,
		"failed_objects":    1,
		"classification": map[string]interface{}{
			"v1_single_put":      1,
			"v1_multipart":       1,
			"v3":                 1,
			"size_lt_1mb":        2,
			"size_1gb_to_10gb":   1,
			"by_key_fingerprint": map[string]interface{}{"legacy": 2},
			"outcome_processed":  1,
			"outcome_skipped":    1,
			"outcome_failed":     1,
		},
	}
	state, err := parseMigrationState(m)
	if err != nil {
		t.Fatalf("parseMigrationState failed: %v", err)
	}
	if state.Classification == nil {
		t.Fatal("Classification not decoded from state map")
	}
	c := state.Classification
	if c.V1SinglePut != 1 || c.V1Multipart != 1 || c.V3 != 1 {
		t.Errorf("source buckets wrong: v1s=%d v1m=%d v3=%d", c.V1SinglePut, c.V1Multipart, c.V3)
	}
	if c.SizeLessThan1MB != 2 || c.Size1GBTo10GB != 1 {
		t.Errorf("size buckets wrong: <1MB=%d 1GB-10GB=%d", c.SizeLessThan1MB, c.Size1GBTo10GB)
	}
	if got := c.ByKeyFingerprint["legacy"]; got != 2 {
		t.Errorf("fingerprint bucket wrong: legacy=%d", got)
	}
	if c.OutcomeProcessed != 1 || c.OutcomeSkipped != 1 || c.OutcomeFailed != 1 {
		t.Errorf("outcome buckets wrong: processed=%d skipped=%d failed=%d",
			c.OutcomeProcessed, c.OutcomeSkipped, c.OutcomeFailed)
	}

	// A malformed classification member is an error, not silent data loss.
	bad := map[string]interface{}{
		"status":         "completed",
		"classification": map[string]interface{}{"v1_single_put": "not-a-number"},
	}
	if _, err := parseMigrationState(bad); err == nil {
		t.Error("parseMigrationState accepted a malformed classification member")
	}

	// Absent classification (older server) stays nil.
	plain, err := parseMigrationState(map[string]interface{}{"status": "in_progress"})
	if err != nil {
		t.Fatalf("parseMigrationState failed without classification: %v", err)
	}
	if plain.Classification != nil {
		t.Errorf("Classification = %+v without a classification member, want nil", plain.Classification)
	}
}

// mixedClassificationCompletedState is the canned completion document both
// JSON-report tests serve: a dry run whose classification covers all eight
// source categories, the same shape the server's mixed dry-run inventory
// produces (one object per rare category, three already-at-target v3s).
func mixedClassificationCompletedState() map[string]interface{} {
	return map[string]interface{}{
		"status":            "completed",
		"total_objects":     10,
		"processed_objects": 5,
		"skipped_objects":   5,
		"failed_objects":    4,
		"dry_run":           true,
		"classification": map[string]interface{}{
			"v1_single_put":            1,
			"v1_multipart":             1,
			"v2_single_put":            1,
			"v2_multipart":             1,
			"v3":                       3,
			"non_armor":                1,
			"malformed":                1,
			"contradictory":            1,
			"size_lt_1mb":              6,
			"size_1mb_to_10mb":         1,
			"size_100mb_to_1gb":        1,
			"size_1gb_to_10gb":         1,
			"size_gt_10gb":             1,
			"by_key_fingerprint":       map[string]interface{}{"1111111111111111": 3, "legacy": 2},
			"outcome_processed":        1,
			"outcome_skipped":          5,
			"outcome_failed":           3,
			"outcome_integrity_failed": 1,
		},
	}
}

// assertJSONReportAllCategories checks that a -json stdout is one JSON
// document whose classification carries every source category.
func assertJSONReportAllCategories(t *testing.T, stdout string) {
	t.Helper()
	var report map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("-json stdout does not parse as JSON: %v\n%s", err, stdout)
	}
	if got, ok := report["status"].(string); !ok || got != "completed" {
		t.Errorf("JSON report status = %v, want completed", report["status"])
	}
	cls, ok := report["classification"].(map[string]interface{})
	if !ok {
		t.Fatalf("JSON report carries no classification object: %s", stdout)
	}
	for key, want := range map[string]float64{
		"v1_single_put": 1, "v1_multipart": 1, "v2_single_put": 1, "v2_multipart": 1,
		"v3": 3, "non_armor": 1, "malformed": 1, "contradictory": 1,
	} {
		got, ok := cls[key]
		if !ok {
			t.Errorf("JSON report classification missing category %q:\n%s", key, stdout)
			continue
		}
		if got != want {
			t.Errorf("JSON report classification %s = %v, want %v", key, got, want)
		}
	}
}

// assertHumanSummaryAllCategories checks that the stderr human-readable
// summary names every source category.
func assertHumanSummaryAllCategories(t *testing.T, stderr string) {
	t.Helper()
	for _, want := range []string{
		"source:",
		"v1-single=1", "v1-multipart=1", "v2-single=1", "v2-multipart=1",
		"v3=3", "non-armor=1", "malformed=1", "contradictory=1", "(total 10)",
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr summary missing %q:\n%s", want, stderr)
		}
	}
}

// TestMigrateJSONReportAllCategories runs the migrate CLI against a server
// whose synchronous completion report carries a mixed eight-category
// classification (the same shape the server's dry-run inventory produces)
// and checks both report surfaces: -json prints the machine-readable report
// with every source category on a pure-JSON stdout, and the human-readable
// summary still renders on stderr.
func TestMigrateJSONReportAllCategories(t *testing.T) {
	os.Setenv("ARMOR_ADMIN_TOKEN", "test-admin-token")

	completed := mixedClassificationCompletedState()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST request, got %s", r.Method)
		}
		if r.URL.Query().Get("dry_run") != "true" {
			t.Errorf("expected dry_run=true, got %s", r.URL.Query().Get("dry_run"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(completed)
	}))
	defer server.Close()

	// Capture stdout and stderr separately: -json must keep stdout pure so
	// the report parses as one JSON document.
	oldStderr := os.Stderr
	oldStdout := os.Stdout
	rout, wout, _ := os.Pipe()
	rerr, werr, _ := os.Pipe()
	os.Stdout = wout
	os.Stderr = werr

	cmd := commands["migrate"]
	// -watch=false explicitly: the flag variables are package globals and an
	// earlier watch-mode test leaves watchFlag set.
	_ = cmd.Flags.Parse([]string{"-admin-url", server.URL, "-dry-run", "-watch=false", "-json"})

	done := make(chan struct{})
	go func() {
		defer close(done)
		cmd.Func(cmd.Flags)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("migration did not complete within timeout")
	}

	wout.Close()
	werr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var outBuf, errBuf strings.Builder
	io.Copy(&outBuf, rout)
	io.Copy(&errBuf, rerr)
	stdout := outBuf.String()
	stderr := errBuf.String()

	assertJSONReportAllCategories(t, stdout)
	assertHumanSummaryAllCategories(t, stderr)
}

// TestMigrateWatchModeJSONReport is the watch-mode face of the JSON
// completion report: the POST starts the run asynchronously, the first poll
// returns the completed state, and -json must still put one pure-JSON
// document on stdout — progress lines and the human summary move to stderr.
func TestMigrateWatchModeJSONReport(t *testing.T) {
	os.Setenv("ARMOR_ADMIN_TOKEN", "test-admin-token")

	completed := mixedClassificationCompletedState()
	getCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		switch r.Method {
		case http.MethodPost:
			// Async start: the completion document arrives via polling.
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":            "in_progress",
				"total_objects":     10,
				"processed_objects": 0,
				"failed_objects":    0,
				"dry_run":           true,
			})
		case http.MethodGet:
			getCount++
			json.NewEncoder(w).Encode(completed)
		default:
			t.Errorf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()

	oldStderr := os.Stderr
	oldStdout := os.Stdout
	rout, wout, _ := os.Pipe()
	rerr, werr, _ := os.Pipe()
	os.Stdout = wout
	os.Stderr = werr

	// The completed document reports failures, and watch mode exits
	// non-zero when the run had any — swap exit so it panics instead of
	// killing the test binary, as the other watch tests do.
	var exitCode int
	oldExit := exit
	exit = func(code int) {
		exitCode = code
		panic("exit")
	}

	cmd := commands["migrate"]
	_ = cmd.Flags.Parse([]string{"-admin-url", server.URL, "-dry-run", "-watch", "-json"})

	done := make(chan struct{})
	go func() {
		defer func() {
			recover() // expected exit via panic
			close(done)
		}()
		cmd.Func(cmd.Flags)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("migration did not complete within timeout")
	}

	wout.Close()
	werr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	exit = oldExit

	var outBuf, errBuf strings.Builder
	io.Copy(&outBuf, rout)
	io.Copy(&errBuf, rerr)
	stdout := outBuf.String()
	stderr := errBuf.String()

	if getCount < 1 {
		t.Errorf("expected at least one progress poll, got %d", getCount)
	}

	assertJSONReportAllCategories(t, stdout)

	// The progress lines and the human-readable summary share stderr;
	// neither may leak onto the JSON-only stdout.
	assertHumanSummaryAllCategories(t, stderr)
	if !strings.Contains(stderr, "Progress:") {
		t.Errorf("stderr does not contain the progress line:\n%s", stderr)
	}
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for a run with failures, got %d", exitCode)
	}
}
