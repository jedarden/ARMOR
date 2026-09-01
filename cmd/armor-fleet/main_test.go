// Package main provides unit tests for armor-fleet.
package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// TestLoadTargets tests target configuration loading.
func TestLoadTargets(t *testing.T) {
	// Create a temporary YAML file
	yamlContent := `
- name: test-armor-01
  cluster: test-cluster
  namespace: test-ns
  service: armor-test
  admin_port: 9001
`
	tmpfile, err := os.CreateTemp("", "targets-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(yamlContent)); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpfile.Close()

	// Load targets
	targets, err := LoadTargets(tmpfile.Name())
	if err != nil {
		t.Fatalf("LoadTargets failed: %v", err)
	}

	if len(targets) != 1 {
		t.Errorf("Expected 1 target, got %d", len(targets))
	}

	target := targets[0]
	if target.Name != "test-armor-01" {
		t.Errorf("Expected name 'test-armor-01', got '%s'", target.Name)
	}
	if target.Cluster != "test-cluster" {
		t.Errorf("Expected cluster 'test-cluster', got '%s'", target.Cluster)
	}
	if target.Namespace != "test-ns" {
		t.Errorf("Expected namespace 'test-ns', got '%s'", target.Namespace)
	}
	if target.Service != "armor-test" {
		t.Errorf("Expected service 'armor-test', got '%s'", target.Service)
	}
	if target.AdminPort != 9001 {
		t.Errorf("Expected admin_port 9001, got %d", target.AdminPort)
	}
}

// TestLoadTargetsValidation tests target validation.
func TestLoadTargetsValidation(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		wantErr     bool
		errContains string
	}{
		{
			name: "missing name",
			yamlContent: `
- cluster: test-cluster
  namespace: test-ns
  service: armor-test
  admin_port: 9001
`,
			wantErr:     true,
			errContains: "name is required",
		},
		{
			name: "missing cluster",
			yamlContent: `
- name: test-armor
  namespace: test-ns
  service: armor-test
  admin_port: 9001
`,
			wantErr:     true,
			errContains: "cluster is required",
		},
		{
			name: "missing namespace",
			yamlContent: `
- name: test-armor
  cluster: test-cluster
  service: armor-test
  admin_port: 9001
`,
			wantErr:     true,
			errContains: "namespace is required",
		},
		{
			name: "missing service",
			yamlContent: `
- name: test-armor
  cluster: test-cluster
  namespace: test-ns
  admin_port: 9001
`,
			wantErr:     true,
			errContains: "service is required",
		},
		{
			name: "missing admin_port",
			yamlContent: `
- name: test-armor
  cluster: test-cluster
  namespace: test-ns
  service: armor-test
`,
			wantErr:     true,
			errContains: "admin_port is required",
		},
		{
			name: "empty file",
			yamlContent: ``,
			wantErr:     true,
			errContains: "no targets found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpfile, err := os.CreateTemp("", "targets-*.yaml")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpfile.Name())

			if _, err := tmpfile.Write([]byte(tt.yamlContent)); err != nil {
				t.Fatalf("Failed to write temp file: %v", err)
			}
			tmpfile.Close()

			_, err = LoadTargets(tmpfile.Name())
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadTargets() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errContains != "" {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Error should contain %q, got %q", tt.errContains, err.Error())
				}
			}
		})
	}
}

// TestFleetMonitor tests the fleet monitor with a mock SEAM server.
func TestFleetMonitor(t *testing.T) {
	// Create a mock SEAM server
	seamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify authorization header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Route based on path
		path := r.URL.Path

		if strings.HasSuffix(path, "/version") {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"version": "0.1.1906",
			})
		} else if strings.HasSuffix(path, "/armor/canary") {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"healthy":         true,
				"message":         "All canaries OK",
				"multipart_canary": true,
			})
		} else if strings.HasSuffix(path, "/metrics") {
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte(`# HELP armor_errors_total Total number of ARMOR errors
# TYPE armor_errors_total counter
armor_errors_total 5.0

# HELP armor_restore_verifier_last_success_seconds Unix timestamp of last successful restore verification
# TYPE armor_restore_verifier_last_success_seconds gauge
armor_restore_verifier_last_success_seconds 1724927400.0

# HELP armor_restore_verifier_successes_total Total number of successful restore verifications
# TYPE armor_restore_verifier_successes_total counter
armor_restore_verifier_successes_total 42
`))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer seamServer.Close()

	// Create target pointing to mock server
	target := Target{
		Name:      "test-armor",
		Cluster:   "test-cluster",
		Namespace: "test-ns",
		Service:   "armor",
		AdminPort: 9001,
	}

	// Create monitor with short poll interval for testing
	monitor := NewFleetMonitor([]Target{target}, "test-token", 1)

	// Override the client and baseURL to use the mock server
	monitor.client = seamServer.Client()
	monitor.baseURL = seamServer.URL + "/k8s/%s/api/v1/namespaces/%s/services/%s:%d/proxy"

	// Manually trigger a poll to avoid waiting for the ticker
	ctx := context.Background()
	status := monitor.pollTarget(ctx, target)

	// Verify status
	if !status.Reachable {
		t.Errorf("Expected target to be reachable, got unreachable")
	}

	if status.Version != "0.1.1906" {
		t.Errorf("Expected version 0.1.1906, got %s", status.Version)
	}

	if !status.CanaryHealthy {
		t.Errorf("Expected canary to be healthy")
	}

	if !status.MultipartCanary {
		t.Errorf("Expected multipart canary to be enabled")
	}

	if status.ErrorRate != 5.0 {
		t.Errorf("Expected error rate 5.0, got %f", status.ErrorRate)
	}

	if len(status.RestoreVerifierGauges) == 0 {
		t.Errorf("Expected restore verifier gauges to be populated")
	}

	if status.Error != "" {
		t.Errorf("Expected no error message, got %s", status.Error)
	}
}

// TestFleetMonitorUnreachable tests the fleet monitor with an unreachable target.
func TestFleetMonitorUnreachable(t *testing.T) {
	// Create a mock SEAM server that returns errors
	seamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer seamServer.Close()

	target := Target{
		Name:      "unreachable-armor",
		Cluster:   "test-cluster",
		Namespace: "test-ns",
		Service:   "armor",
		AdminPort: 9001,
	}

	monitor := NewFleetMonitor([]Target{target}, "test-token", 1)
	monitor.client = seamServer.Client()

	ctx := context.Background()
	status := monitor.pollTarget(ctx, target)

	if status.Reachable {
		t.Errorf("Expected target to be unreachable")
	}

	if status.Error == "" {
		t.Errorf("Expected error message, got empty string")
	}
}

// TestServer tests the HTTP server endpoints.
func TestServer(t *testing.T) {
	// Create a monitor with mock data
	target := Target{
		Name:      "test-armor",
		Cluster:   "test-cluster",
		Namespace: "test-ns",
		Service:   "armor",
		AdminPort: 9001,
	}

	monitor := NewFleetMonitor([]Target{target}, "test-token", 1)
	monitor.mu.Lock()
	monitor.status[target.Name] = &TargetStatus{
		Name:             target.Name,
		Cluster:          target.Cluster,
		Namespace:        target.Namespace,
		Service:          target.Service,
		Version:          "0.1.1906",
		CanaryHealthy:    true,
		CanaryMessage:    "All OK",
		MultipartCanary:  true,
		ErrorRate:        0.0,
		LastSeen:         time.Now(),
		Reachable:        true,
		RestoreVerifierGauges: map[string]string{
			"last_success_seconds": "1724927400.0",
		},
	}
	monitor.mu.Unlock()

	// Create server
	server := NewServer(monitor, ":0")

	// Create test HTTP server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/fleet.json":
			server.handleFleetJSON(w, r)
		case "/":
			server.handleHTML(w, r)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer testServer.Close()

	// Test /fleet.json
	resp, err := http.Get(testServer.URL + "/fleet.json")
	if err != nil {
		t.Fatalf("Failed to GET /fleet.json: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	var status map[string]*TargetStatus
	if err := json.Unmarshal(body, &status); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if len(status) != 1 {
		t.Errorf("Expected 1 target status, got %d", len(status))
	}

	targetStatus, ok := status["test-armor"]
	if !ok {
		t.Errorf("Expected status for 'test-armor', not found")
	}

	if targetStatus.Version != "0.1.1906" {
		t.Errorf("Expected version 0.1.1906, got %s", targetStatus.Version)
	}

	// Test / (HTML page)
	resp, err = http.Get(testServer.URL + "/")
	if err != nil {
		t.Fatalf("Failed to GET /: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	contentType = resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("Expected Content-Type to contain text/html, got %s", contentType)
	}

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	htmlContent := string(body)
	if !strings.Contains(htmlContent, "ARMOR Fleet Dashboard") {
		t.Errorf("Expected HTML to contain 'ARMOR Fleet Dashboard'")
	}

	if !strings.Contains(htmlContent, "importmap") {
		t.Errorf("Expected HTML to contain importmap for Agentation")
	}
}

// TestMetricParsing tests the Prometheus metric parsing logic.
func TestMetricParsing(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantName string
		wantVal  string
		wantFloat float64
	}{
		{
			name:     "simple counter",
			line:     "armor_errors_total 42.0",
			wantName: "",
			wantVal:  "",
			wantFloat: 42.0,
		},
		{
			name:     "counter with labels",
			line:     "armor_errors_total{bucket=\"test\"} 5.0",
			wantName: "",
			wantVal:  "",
			wantFloat: 5.0,
		},
		{
			name:     "restore verifier gauge",
			line:     "armor_restore_verifier_last_success_seconds 1724927400.0",
			wantName: "last_success_seconds",
			wantVal:  "1724927400.0",
			wantFloat: 0,
		},
		{
			name:     "restore verifier gauge with labels",
			line:     "armor_restore_verifier_successes_total{bucket=\"test\"} 100",
			wantName: "successes_total",
			wantVal:  "100",
			wantFloat: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test parseMetricValue
			if tt.wantFloat > 0 {
				got := parseMetricValue(tt.line)
				if got != tt.wantFloat {
					t.Errorf("parseMetricValue() = %f, want %f", got, tt.wantFloat)
				}
			}

			// Test parseGaugeMetric
			if tt.wantName != "" {
				name, val := parseGaugeMetric(tt.line)
				if name != tt.wantName {
					t.Errorf("parseGaugeMetric() name = %s, want %s", name, tt.wantName)
				}
				if val != tt.wantVal {
					t.Errorf("parseGaugeMetric() value = %s, want %s", val, tt.wantVal)
				}
			}
		})
	}
}
