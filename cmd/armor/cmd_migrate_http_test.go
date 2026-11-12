package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type migrationHTTPExit struct{ code int }

// runMigrateHTTP exercises the command without starting another process. The
// CLI's flags and exit hook are package globals, so these tests run serially.
func runMigrateHTTP(t *testing.T, adminURL string, watch bool, resume string) (int, string, string) {
	t.Helper()
	t.Setenv("ARMOR_ADMIN_TOKEN", "http-test-token")

	oldURL, oldDryRun, oldInventory := adminURLFlag, dryRunFlag, inventoryOnlyFlag
	oldTarget, oldInclude, oldConcurrency := targetFlag, includeFlag, concurrencyFlag
	oldWatch, oldResume, oldOperationID, oldJSON := watchFlag, resumeOperationIDFlag, operationIDFlag, jsonOutputFlag
	defer func() {
		adminURLFlag, dryRunFlag, inventoryOnlyFlag = oldURL, oldDryRun, oldInventory
		targetFlag, includeFlag, concurrencyFlag = oldTarget, oldInclude, oldConcurrency
		watchFlag, resumeOperationIDFlag, operationIDFlag, jsonOutputFlag = oldWatch, oldResume, oldOperationID, oldJSON
	}()
	adminURLFlag, dryRunFlag, inventoryOnlyFlag = adminURL, false, false
	targetFlag, includeFlag, concurrencyFlag = "3", "1,2", 0
	watchFlag, resumeOperationIDFlag, operationIDFlag, jsonOutputFlag = watch, resume, "", false

	oldExit, oldStdout, oldStderr := exit, os.Stdout, os.Stderr
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		stdoutR.Close()
		stdoutW.Close()
		t.Fatal(err)
	}
	os.Stdout, os.Stderr = stdoutW, stderrW
	defer func() {
		exit, os.Stdout, os.Stderr = oldExit, oldStdout, oldStderr
		stdoutR.Close()
		stdoutW.Close()
		stderrR.Close()
		stderrW.Close()
	}()

	exit = func(code int) { panic(migrationHTTPExit{code: code}) }
	exitCode := 0
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				if cliExit, ok := recovered.(migrationHTTPExit); ok {
					exitCode = cliExit.code
					return
				}
				panic(recovered)
			}
		}()
		migrate(flag.NewFlagSet("migrate", flag.ContinueOnError))
	}()
	stdoutW.Close()
	stderrW.Close()
	os.Stdout, os.Stderr = oldStdout, oldStderr
	stdout, err := io.ReadAll(stdoutR)
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := io.ReadAll(stderrR)
	if err != nil {
		t.Fatal(err)
	}
	return exitCode, string(stdout), string(stderr)
}

func TestMigrateHTTPAcceptedStartReturnsWithoutPolling(t *testing.T) {
	var posts, gets atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != migrationPath || r.Header.Get("Authorization") != "Bearer http-test-token" {
			t.Errorf("unexpected request path or authorization: %s", r.URL.Path)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		switch r.Method {
		case http.MethodPost:
			posts.Add(1)
			if r.URL.Query().Get("target") != "3" || r.URL.Query().Get("include") != "1,2" {
				t.Errorf("start query = %q", r.URL.RawQuery)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			fmt.Fprint(w, `{"status":"in_progress","operation_id":"op-start"}`)
		case http.MethodGet:
			gets.Add(1)
			fmt.Fprint(w, `{"status":"completed"}`)
		default:
			http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	exitCode, _, stderr := runMigrateHTTP(t, server.URL, false, "")
	if exitCode != 0 || posts.Load() != 1 || gets.Load() != 0 {
		t.Fatalf("exit=%d POST=%d GET=%d; stderr=%s", exitCode, posts.Load(), gets.Load(), stderr)
	}
	if !strings.Contains(stderr, "Migration operation started: op-start") {
		t.Fatalf("start output lacks operation ID: %s", stderr)
	}
}

func TestMigrateHTTPResumeWatchesRequestedOperation(t *testing.T) {
	var gets, posts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posts.Add(1)
			http.Error(w, "resume must not start", http.StatusBadRequest)
			return
		}
		gets.Add(1)
		if r.Method != http.MethodGet || r.URL.Query().Get("operation_id") != "op-resume" {
			t.Errorf("resume request = %s %s", r.Method, r.URL.String())
		}
		fmt.Fprint(w, `{"id":"op-resume","status":"completed","total_objects":1,"processed_objects":1}`)
	}))
	defer server.Close()

	exitCode, _, stderr := runMigrateHTTP(t, server.URL, true, "op-resume")
	if exitCode != 0 || posts.Load() != 0 || gets.Load() != 1 {
		t.Fatalf("exit=%d POST=%d GET=%d; stderr=%s", exitCode, posts.Load(), gets.Load(), stderr)
	}
}

func TestMigrateHTTPWatchReconnectsAfterDisconnect(t *testing.T) {
	var gets atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Query().Get("operation_id") != "op-reconnect" {
			t.Errorf("watch request = %s %s", r.Method, r.URL.String())
		}
		if gets.Add(1) == 1 {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Errorf("hijack connection: %v", err)
				return
			}
			conn.Close()
			return
		}
		fmt.Fprint(w, `{"id":"op-reconnect","status":"completed","total_objects":1,"processed_objects":1}`)
	}))
	defer server.Close()

	oldJSON := jsonOutputFlag
	jsonOutputFlag = false
	defer func() { jsonOutputFlag = oldJSON }()
	client := &http.Client{Timeout: 5 * time.Second}
	if err := watchMigration(client, server.URL+migrationPath, "http-test-token", "op-reconnect"); err != nil {
		t.Fatalf("watch after disconnect: %v", err)
	}
	if got := gets.Load(); got != 2 {
		t.Fatalf("GET count = %d, want 2", got)
	}
}

func TestMigrateHTTPWatchReportsFailedObjectsWithNonzeroExit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Query().Get("operation_id") != "op-failed" {
			t.Errorf("failure status request = %s %s", r.Method, r.URL.String())
		}
		fmt.Fprint(w, `{"id":"op-failed","status":"completed","total_objects":1,"failed_objects":1,"failures":[{"key":"broken-object","reason":"invalid ciphertext"}]}`)
	}))
	defer server.Close()

	exitCode, _, stderr := runMigrateHTTP(t, server.URL, true, "op-failed")
	if exitCode != 1 {
		t.Fatalf("exit=%d, want 1; stderr=%s", exitCode, stderr)
	}
	for _, want := range []string{"1 failures", "broken-object", "invalid ciphertext"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr missing %q: %s", want, stderr)
		}
	}
}

func TestMigrateHTTPWatchRejectsUnknownAndWrongOperation(t *testing.T) {
	tests := []struct {
		name     string
		response string
		want     string
	}{
		{"unknown object", `{"id":"op-check","status":"completed","unknown_objects":1}`, "1 unknown"},
		{"wrong operation", `{"id":"op-other","status":"completed"}`, "belongs to operation"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("operation_id") != "op-check" {
					t.Errorf("watch query = %q", r.URL.RawQuery)
				}
				fmt.Fprint(w, tt.response)
			}))
			defer server.Close()
			exitCode, _, stderr := runMigrateHTTP(t, server.URL, true, "op-check")
			if exitCode != 1 || !strings.Contains(stderr, tt.want) {
				t.Fatalf("exit=%d stderr=%s, want %q", exitCode, stderr, tt.want)
			}
		})
	}
}
