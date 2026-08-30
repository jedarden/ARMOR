//go:build !integration
// +build !integration

// Tests for 'armor migrate' command
package main

import (
	"encoding/json"
	"flag"
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
			// Save original environment
			oldEnv := os.Environ()
			defer func() {
				// Restore environment
				os.Clearenv()
				for _, v := range oldEnv {
					kv := strings.SplitN(v, "=", 2)
					if len(kv) == 2 {
						os.Setenv(kv[0], kv[1])
					}
				}
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

			// Run migrate with flags
			os.Args = append([]string{"armor", "migrate"}, tt.flags...)

			// This will panic if exit is called, which we expect
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

			migrate()
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

			// Update admin-url flag with server URL
			flags := make([]string, 0, len(tt.flags))
			for i, f := range tt.flags {
				if f == "-admin-url" && i+1 < len(tt.flags) && tt.flags[i+1] == "" {
					flags = append(flags, "-admin-url", server.URL)
				} else if i > 0 && tt.flags[i-1] == "-admin-url" && tt.flags[i-1] != "" {
					flags = append(flags, server.URL)
				} else {
					flags = append(flags, f)
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
			exit = func(code int) {
				exitCode = code
				panic("exit")
			}

			// Defer cleanup
			defer func() {
				os.Stderr = oldStderr
				os.Stdout = oldStdout
				werr.Close()
				wout.Close()
			}()

			// Reset flag parsing for this test
			flag.CommandLine = flag.NewFlagSet("migrate", flag.ContinueOnError)

			// Run migrate
			os.Args = append([]string{"armor", "migrate"}, flags...)
			func() {
				defer func() {
					recover() // Expected exit via panic
				}()
				migrate()
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
	exit = func(code int) {
		exitCode = code
		panic("exit")
	}

	// Defer cleanup
	defer func() {
		os.Stderr = oldStderr
		os.Stdout = oldStdout
		wout.Close()
	}()

	// Run migrate with --watch
	os.Args = []string{"armor", "migrate", "-admin-url", server.URL, "-watch"}

	// Reset flag parsing
	flag.CommandLine = flag.NewFlagSet("migrate", flag.ContinueOnError)

	// Run migrate in goroutine with timeout
	done := make(chan struct{})
	go func() {
		defer func() {
			recover() // Expected exit via panic
			close(done)
		}()
		migrate()
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
	exit = func(code int) {
		exitCode = code
		panic("exit")
	}

	defer func() {
		os.Stderr = oldStderr
		os.Stdout = oldStdout
		wout.Close()
	}()

	os.Args = []string{"armor", "migrate", "-admin-url", server.URL, "-watch"}
	flag.CommandLine = flag.NewFlagSet("migrate", flag.ContinueOnError)

	done := make(chan struct{})
	go func() {
		defer func() {
			recover()
			close(done)
		}()
		migrate()
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

	// Verify exit code is 0 (success)
	if exitCode != 0 {
		t.Errorf("expected exit code 0 for successful migration, got %d", exitCode)
	}
}
