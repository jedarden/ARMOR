//go:build integration
// +build integration

// Integration tests for Provenance Audit endpoint.
// Tests the /armor/audit endpoint against a running ARMOR server with real B2 backend.
//
// Environment variables required:
// ARMOR_INTEGRATION_TEST=1              - Must be set to run tests
// ARMOR_ADMIN_ENDPOINT                  - Admin API endpoint (default: http://localhost:9001)
// ARMOR_ADMIN_TOKEN                     - Admin token for authentication
// ARMOR_MEK                             - Current master encryption key (hex)
// ARMOR_BUCKET                          - B2 bucket name
// Other ARMOR_* env vars from integration_test.go

package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

// TestProvenanceAudit tests the /armor/audit endpoint.
// Verifies that:
// 1. The endpoint requires authentication
// 2. Returns valid audit results with correct structure
// 3. Reports chain integrity status
func TestProvenanceAudit(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	adminEndpoint := os.Getenv("ARMOR_ADMIN_ENDPOINT")
	if adminEndpoint == "" {
		adminEndpoint = "http://localhost:9001"
	}

	adminToken := os.Getenv("ARMOR_ADMIN_TOKEN")
	if adminToken == "" {
		t.Skip("Skipping: ARMOR_ADMIN_TOKEN not set")
	}

	client := &http.Client{Timeout: 30 * time.Second}

	// Test 1: Unauthenticated request should return 401
	t.Run("unauthenticated", func(t *testing.T) {
		req, err := http.NewRequest("GET", adminEndpoint+"/armor/audit", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", resp.StatusCode)
		}
	})

	// Test 2: Authenticated request should return audit results
	t.Run("authenticated", func(t *testing.T) {
		req, err := http.NewRequest("GET", adminEndpoint+"/armor/audit", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+adminToken)

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
		}

		// Parse response
		var result struct {
			Status           string `json:"status"`
			TotalEntries     int64  `json:"total_entries"`
			TotalObjects     int64  `json:"total_objects"`
			UntrackedObjects []string `json:"untracked_objects,omitempty"`
			Writers         []struct {
				WriterID        string `json:"writer_id"`
				HeadSequence    int64  `json:"head_sequence"`
				EntriesVerified int    `json:"entries_verified"`
				Valid           bool   `json:"valid"`
				Error           string `json:"error,omitempty"`
			} `json:"writers"`
			Errors []string `json:"errors,omitempty"`
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response: %v", err)
		}

		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		// Verify response structure
		if result.Status != "valid" && result.Status != "invalid" && result.Status != "incomplete" {
			t.Errorf("Unexpected status: %s", result.Status)
		}

		t.Logf("Audit status: %s", result.Status)
		t.Logf("Total entries: %d", result.TotalEntries)
		t.Logf("Total objects: %d", result.TotalObjects)
		t.Logf("Writers audited: %d", len(result.Writers))

		// Log writers if any
		for _, writer := range result.Writers {
			t.Logf("Writer %s: sequence=%d, verified=%d, valid=%v",
				writer.WriterID, writer.HeadSequence, writer.EntriesVerified, writer.Valid)
			if writer.Error != "" {
				t.Logf("  Error: %s", writer.Error)
			}
		}

		// Log untracked objects if any
		if len(result.UntrackedObjects) > 0 {
			t.Logf("Untracked objects: %d", len(result.UntrackedObjects))
			for i, obj := range result.UntrackedObjects {
				if i < 10 {
					t.Logf("  - %s", obj)
				}
			}
			if len(result.UntrackedObjects) > 10 {
				t.Logf("  ... and %d more", len(result.UntrackedObjects)-10)
			}
		}

		// Log errors if any
		if len(result.Errors) > 0 {
			t.Logf("Errors: %d", len(result.Errors))
			for _, err := range result.Errors {
				t.Logf("  - %s", err)
			}
		}
	})

	// Test 3: Only GET method is allowed
	t.Run("methodNotAllowed", func(t *testing.T) {
		req, err := http.NewRequest("POST", adminEndpoint+"/armor/audit", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+adminToken)

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("Expected status 405, got %d", resp.StatusCode)
		}
	})
}
