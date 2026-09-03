// Package server provides end-to-end tests for the /admin/format/migrate
// endpoint's parameter validation, driven through the real admin mux and its
// bearer-token gate.
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
)

const (
	migrationTestBucket = "migrate-validation"
	migrationTestToken  = "migration-validation-admin-token"
)

// startMigrationServer runs the production admin mux against an empty
// filesystem backend configured with the given write version. The returned
// request helper performs an authenticated admin call and yields the response
// plus its body.
func startMigrationServer(t *testing.T, writeVersion int) func(method, query string) (*http.Response, string) {
	t.Helper()

	fsBackend, err := backend.NewFSBackend(backend.FSConfig{BasePath: t.TempDir()})
	if err != nil {
		t.Fatalf("create filesystem backend: %v", err)
	}
	if err := fsBackend.CreateBucket(context.Background(), migrationTestBucket); err != nil {
		t.Fatalf("create test bucket: %v", err)
	}

	cfg := &config.Config{
		Bucket:             migrationTestBucket,
		BlockSize:          65536,
		MEK:                bytes.Repeat([]byte{0x44}, 32),
		FormatWriteVersion: writeVersion,
		AdminToken:         migrationTestToken,
	}
	armorServer, err := NewWithBackend(cfg, fsBackend)
	if err != nil {
		t.Fatalf("create ARMOR server: %v", err)
	}

	adminServer := httptest.NewServer(armorServer.AdminHandler())
	t.Cleanup(adminServer.Close)

	return func(method, query string) (*http.Response, string) {
		req, err := http.NewRequest(method, adminServer.URL+"/admin/format/migrate", nil)
		if err != nil {
			t.Fatalf("build request: %v", err)
		}
		req.URL.RawQuery = query
		req.Header.Set("Authorization", "Bearer "+migrationTestToken)
		resp, err := adminServer.Client().Do(req)
		if err != nil {
			t.Fatalf("%s request failed: %v", method, err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("read response body: %v", err)
		}
		return resp, string(body)
	}
}

// migrationProgress is the subset of the persisted migration state the tests
// assert on.
type migrationProgress struct {
	Status              string   `json:"status"`
	IncludeVersions     []string `json:"include_versions"`
	CurrentWriteVersion uint8    `json:"current_write_version"`
}

// TestMigrateFormatTargetMismatchFailsClosed verifies that a target differing
// from Config.FormatWriteVersion is rejected with a clear error and that the
// rejected request leaves no migration state behind.
func TestMigrateFormatTargetMismatchFailsClosed(t *testing.T) {
	request := startMigrationServer(t, 3)

	resp, body := request(http.MethodPost, "target=2")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("target=2 with write version 3: status = %d, want 400 (body: %s)", resp.StatusCode, body)
	}
	if !bytes.Contains([]byte(body), []byte("does not match configured write version v3")) {
		t.Errorf("target=2 error body does not name the configured write version: %s", body)
	}

	// The failed request must not have started or staged a migration.
	resp, body = request(http.MethodGet, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("progress after rejected migration: status = %d (body: %s)", resp.StatusCode, body)
	}
	var progress migrationProgress
	if err := json.Unmarshal([]byte(body), &progress); err != nil {
		t.Fatalf("decode progress response: %v (body: %s)", err, body)
	}
	if progress.Status != "no_migration" {
		t.Errorf("progress status after rejected migration = %q, want no_migration", progress.Status)
	}
}

// TestMigrateFormatRejectsBadParameters covers the remaining fail-closed
// parameter rejections: an unparseable target and a source set that is not
// strictly older than the target.
func TestMigrateFormatRejectsBadParameters(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		wantInBody string
	}{
		{name: "non-numeric target", query: "target=invalid", wantInBody: "invalid target version"},
		{name: "V3 in source set", query: "include=v3", wantInBody: "not compatible with target version v3"},
		{name: "V3 in mixed source set", query: "include=v1,v3", wantInBody: "not compatible with target version v3"},
		{name: "non-numeric include entry", query: "include=v1,later", wantInBody: "invalid include version"},
		{name: "zero include entry", query: "include=v0", wantInBody: "invalid include version"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := startMigrationServer(t, 3)

			resp, body := request(http.MethodPost, tt.query)
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("%s: status = %d, want 400 (body: %s)", tt.query, resp.StatusCode, body)
			}
			if !bytes.Contains([]byte(body), []byte(tt.wantInBody)) {
				t.Errorf("%s: body %q does not contain %q", tt.query, body, tt.wantInBody)
			}
		})
	}
}

// TestMigrateFormatAcceptsExplicitTargetAndInclude drives the acceptance side
// of the contract: matching targets in both spellings, the v1,v2 source set,
// and the version-dependent default include.
func TestMigrateFormatAcceptsExplicitTargetAndInclude(t *testing.T) {
	tests := []struct {
		name            string
		writeVersion    int
		query           string
		expectedInclude []string
	}{
		{name: "no params uses configured target and default include", writeVersion: 3, query: "", expectedInclude: []string{"2"}},
		{name: "numeric target accepted", writeVersion: 3, query: "target=3", expectedInclude: []string{"2"}},
		{name: "v-prefixed target accepted", writeVersion: 3, query: "target=v3", expectedInclude: []string{"2"}},
		{name: "v1,v2 source set accepted", writeVersion: 3, query: "target=v3&include=v1,v2", expectedInclude: []string{"1", "2"}},
		{name: "single v1 source accepted", writeVersion: 3, query: "include=v1", expectedInclude: []string{"1"}},
		{name: "V2 server defaults to v1 source", writeVersion: 2, query: "", expectedInclude: []string{"1"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := startMigrationServer(t, tt.writeVersion)

			resp, body := request(http.MethodPost, tt.query)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("%q: status = %d, want 200 (body: %s)", tt.query, resp.StatusCode, body)
			}
			var result struct {
				Status string `json:"status"`
			}
			if err := json.Unmarshal([]byte(body), &result); err != nil {
				t.Fatalf("decode migration result: %v (body: %s)", err, body)
			}
			if result.Status != "completed" {
				t.Errorf("migration status = %q, want completed (body: %s)", result.Status, body)
			}

			// The persisted state must record the include set the request
			// resolved to, proving the default/normalization took effect.
			resp, body = request(http.MethodGet, "")
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("progress: status = %d (body: %s)", resp.StatusCode, body)
			}
			var progress migrationProgress
			if err := json.Unmarshal([]byte(body), &progress); err != nil {
				t.Fatalf("decode progress response: %v (body: %s)", err, body)
			}
			if progress.Status != "completed" {
				t.Errorf("progress status = %q, want completed", progress.Status)
			}
			if progress.CurrentWriteVersion != uint8(tt.writeVersion) {
				t.Errorf("state write version = %d, want %d", progress.CurrentWriteVersion, tt.writeVersion)
			}
			if len(progress.IncludeVersions) != len(tt.expectedInclude) {
				t.Fatalf("include versions = %v, want %v", progress.IncludeVersions, tt.expectedInclude)
			}
			for i, want := range tt.expectedInclude {
				if progress.IncludeVersions[i] != want {
					t.Errorf("include versions = %v, want %v", progress.IncludeVersions, tt.expectedInclude)
					break
				}
			}
		})
	}
}

// TestMigrateFormatUnsupportedWriteVersionFailsClosed verifies a server that
// cannot name an older source format refuses to migrate at all.
func TestMigrateFormatUnsupportedWriteVersionFailsClosed(t *testing.T) {
	request := startMigrationServer(t, 1)

	resp, body := request(http.MethodPost, "")
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("write version 1: status = %d, want 500 (body: %s)", resp.StatusCode, body)
	}
	if !bytes.Contains([]byte(body), []byte("Unsupported current write version: 1")) {
		t.Errorf("write version 1 body does not explain the refusal: %s", body)
	}
}
