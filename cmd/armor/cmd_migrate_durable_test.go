package main

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestMigrationStartQueryDefaultsToV3Inventory(t *testing.T) {
	oldDryRun, oldInventory := dryRunFlag, inventoryOnlyFlag
	oldTarget, oldInclude, oldConcurrency := targetFlag, includeFlag, concurrencyFlag
	defer func() {
		dryRunFlag, inventoryOnlyFlag = oldDryRun, oldInventory
		targetFlag, includeFlag, concurrencyFlag = oldTarget, oldInclude, oldConcurrency
	}()

	dryRunFlag = false
	inventoryOnlyFlag = true
	targetFlag = "3"
	includeFlag = "1,2"
	concurrencyFlag = 7

	query := migrationStartQuery()
	if got := query.Get("target"); got != "3" {
		t.Errorf("target = %q, want 3", got)
	}
	if got := query.Get("include"); got != "1,2" {
		t.Errorf("include = %q, want 1,2", got)
	}
	if got := query.Get("dry_run"); got != "true" {
		t.Errorf("dry_run = %q, want true", got)
	}
	if got := query.Get("inventory_only"); got != "true" {
		t.Errorf("inventory_only = %q, want true", got)
	}
	if got := query.Get("concurrency"); got != "7" {
		t.Errorf("concurrency = %q, want 7", got)
	}
}

func TestMigrationStatusURLScopesOperation(t *testing.T) {
	got := migrationStatusURL("http://example.test/admin/format/migrate?foo=bar", "op/42")
	if !strings.Contains(got, "foo=bar") || !strings.Contains(got, "operation_id=op%2F42") {
		t.Fatalf("status URL = %q, want both original and operation query parameters", got)
	}
}

func TestPostMigrationAcceptsImmediateOperationID(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			return nil, errors.New("unexpected method")
		}
		return &http.Response{
			StatusCode: http.StatusAccepted,
			Body:       io.NopCloser(strings.NewReader(`{"operation_id":"op-42","status":"queued"}`)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}

	state, err := postMigration(client, "http://example.test/admin/format/migrate", nil, "token")
	if err != nil {
		t.Fatalf("postMigration: %v", err)
	}
	if got := migrationOperationID(state); got != "op-42" {
		t.Fatalf("operation ID = %q, want op-42", got)
	}
}

func TestParseMigrationStateCountsUnknownCollections(t *testing.T) {
	state, err := parseMigrationState(map[string]interface{}{
		"status":          "completed",
		"unknown_objects": []interface{}{"a", "b"},
	})
	if err != nil {
		t.Fatalf("parseMigrationState: %v", err)
	}
	if state.UnknownObjects != 2 {
		t.Fatalf("unknown objects = %d, want 2", state.UnknownObjects)
	}
}

func TestMigrationUnknownObjectsIncludesMalformedClassification(t *testing.T) {
	got := migrationUnknownObjects(map[string]interface{}{
		"classification": map[string]interface{}{
			"malformed":     float64(2),
			"contradictory": float64(1),
		},
	})
	if got != 3 {
		t.Fatalf("unknown objects = %d, want 3", got)
	}
}

func TestReportMigrationFailsForFailuresAndUnknownObjects(t *testing.T) {
	oldJSON := jsonOutputFlag
	jsonOutputFlag = false
	defer func() { jsonOutputFlag = oldJSON }()

	err := reportMigration(map[string]interface{}{
		"status":          "completed",
		"failed_objects":  1,
		"unknown_objects": []interface{}{"legacy/unknown"},
		"failures":        []interface{}{map[string]interface{}{"key": "broken"}},
	}, "test")
	if err == nil {
		t.Fatal("reportMigration returned nil for failed and unknown objects")
	}
}

func TestMigrationHTTPClientHasBoundedRequestLifetime(t *testing.T) {
	if got := migrationHTTPClient().Timeout; got != 30*time.Second {
		t.Fatalf("HTTP timeout = %s, want 30s", got)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
