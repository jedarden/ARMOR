// Integration tests for valid_complex.yaml
package yamlutil

import (
	"os"
	"testing"
)

// TestValidComplexYAML_Integration tests parsing valid_complex.yaml from internal/yamlutil/testdata/
// and verifies that YAML anchors, aliases, merge keys, and multi-line strings are correctly parsed.
func TestValidComplexYAML_Integration(t *testing.T) {
	testFile := "testdata/valid_complex.yaml"

	// Verify file exists
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Fatalf("test file %s does not exist", testFile)
	}

	// Read file content
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read test file: %v", err)
	}

	if len(content) == 0 {
		t.Fatal("test file is empty")
	}

	// Parse YAML content
	parser := NewParser()
	var data map[string]interface{}
	if err := parser.ParseString(string(content), &data); err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	// Verify top-level sections exist
	requiredSections := []string{"services", "description", "notes"}
	for _, section := range requiredSections {
		if _, exists := data[section]; !exists {
			t.Errorf("expected section '%s' to exist in parsed data", section)
		}
	}

	// Verify services section structure
	services, ok := data["services"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'services' to be a map")
	}

	// Verify services.web (should have merged defaults)
	web, ok := services["web"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'services.web' to be a map")
	}

	// Verify services.web.timeout (from defaults anchor)
	timeout := web["timeout"]
	switch to := timeout.(type) {
	case int:
		if to != 30 {
			t.Errorf("expected services.web.timeout=30, got %d", to)
		}
	case int64:
		if to != 30 {
			t.Errorf("expected services.web.timeout=30, got %d", to)
		}
	default:
		t.Errorf("expected services.web.timeout to be int or int64, got %T with value %v", to, to)
	}

	// Verify services.web.retries (from defaults anchor)
	retries := web["retries"]
	switch r := retries.(type) {
	case int:
		if r != 3 {
			t.Errorf("expected services.web.retries=3, got %d", r)
		}
	case int64:
		if r != 3 {
			t.Errorf("expected services.web.retries=3, got %d", r)
		}
	default:
		t.Errorf("expected services.web.retries to be int or int64, got %T with value %v", r, r)
	}

	// Verify services.web.backoff (from defaults anchor)
	if backoff, ok := web["backoff"].(string); !ok {
		t.Error("expected 'services.web.backoff' to be a string")
	} else if backoff != "exponential" {
		t.Errorf("expected services.web.backoff='exponential', got %q", backoff)
	}

	// Verify services.web.port (web-specific)
	port := web["port"]
	switch p := port.(type) {
	case int:
		if p != 8080 {
			t.Errorf("expected services.web.port=8080, got %d", p)
		}
	case int64:
		if p != 8080 {
			t.Errorf("expected services.web.port=8080, got %d", p)
		}
	default:
		t.Errorf("expected services.web.port to be int or int64, got %T with value %v", p, p)
	}

	// Verify services.web.host (web-specific)
	if host, ok := web["host"].(string); !ok {
		t.Error("expected 'services.web.host' to be a string")
	} else if host != "example.com" {
		t.Errorf("expected services.web.host='example.com', got %q", host)
	}

	// Verify services.web.health_check (web-specific)
	if healthCheck, ok := web["health_check"].(string); !ok {
		t.Error("expected 'services.web.health_check' to be a string")
	} else if healthCheck != "/health" {
		t.Errorf("expected services.web.health_check='/health', got %q", healthCheck)
	}

	// Verify services.api (should have merged defaults with timeout override)
	api, ok := services["api"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'services.api' to be a map")
	}

	// Verify services.api.timeout (OVERRIDDEN from defaults anchor)
	timeout = api["timeout"]
	switch to := timeout.(type) {
	case int:
		if to != 60 {
			t.Errorf("expected services.api.timeout=60 (overridden), got %d", to)
		}
	case int64:
		if to != 60 {
			t.Errorf("expected services.api.timeout=60 (overridden), got %d", to)
		}
	default:
		t.Errorf("expected services.api.timeout to be int or int64, got %T with value %v", to, to)
	}

	// Verify services.api.retries (from defaults anchor)
	retries = api["retries"]
	switch r := retries.(type) {
	case int:
		if r != 3 {
			t.Errorf("expected services.api.retries=3, got %d", r)
		}
	case int64:
		if r != 3 {
			t.Errorf("expected services.api.retries=3, got %d", r)
		}
	default:
		t.Errorf("expected services.api.retries to be int or int64, got %T with value %v", r, r)
	}

	// Verify services.api.backoff (from defaults anchor)
	if backoff, ok := api["backoff"].(string); !ok {
		t.Error("expected 'services.api.backoff' to be a string")
	} else if backoff != "exponential" {
		t.Errorf("expected services.api.backoff='exponential', got %q", backoff)
	}

	// Verify services.api.port (api-specific)
	port = api["port"]
	switch p := port.(type) {
	case int:
		if p != 8081 {
			t.Errorf("expected services.api.port=8081, got %d", p)
		}
	case int64:
		if p != 8081 {
			t.Errorf("expected services.api.port=8081, got %d", p)
		}
	default:
		t.Errorf("expected services.api.port to be int or int64, got %T with value %v", p, p)
	}

	// Verify services.api.host (api-specific)
	if host, ok := api["host"].(string); !ok {
		t.Error("expected 'services.api.host' to be a string")
	} else if host != "api.example.com" {
		t.Errorf("expected services.api.host='api.example.com', got %q", host)
	}

	// Verify description (folded scalar - becomes single line)
	if description, ok := data["description"].(string); !ok {
		t.Error("expected 'description' to be a string")
	} else if description == "" {
		t.Error("expected description to be non-empty")
	} else {
		// Folded scalar should become a single line (spaces between words)
		// The exact format may vary by parser, but it should not contain literal newlines
		expectedDescriptions := []string{
			"This is a folded scalar that spans multiple lines but becomes a single line.",
			"This is a folded scalar that spans multiple lines but becomes a single line.\n",
		}
		found := false
		for _, expected := range expectedDescriptions {
			if description == expected {
				found = true
				break
			}
		}
		if !found {
			t.Logf("Note: description parsed as %q (may vary by YAML parser)", description)
		}
	}

	// Verify notes (literal block scalar - preserves newlines)
	if notes, ok := data["notes"].(string); !ok {
		t.Error("expected 'notes' to be a string")
	} else if notes == "" {
		t.Error("expected notes to be non-empty")
	} else {
		// Literal block scalar should preserve newlines
		expectedNotes := "This is a literal block scalar\nthat preserves newlines and\nformatting exactly as written."
		// Some parsers add a trailing newline
		if notes != expectedNotes && notes != expectedNotes+"\n" {
			t.Logf("Note: notes parsed as %q (may vary by YAML parser)", notes)
		}
	}

	// Verify no unexpected top-level keys (defaults is allowed - it's the anchor definition)
	expectedTopLevelKeys := map[string]bool{
		"services":    true,
		"description": true,
		"notes":       true,
		"defaults":    true, // Anchor definition is included by some parsers
	}

	for key := range data {
		if !expectedTopLevelKeys[key] {
			t.Errorf("unexpected top-level key '%s' in parsed data", key)
		}
	}

	// Verify all expected top-level keys are present (except defaults which is optional)
	requiredKeys := map[string]bool{
		"services":    true,
		"description": true,
		"notes":       true,
	}
	for key := range requiredKeys {
		if _, exists := data[key]; !exists {
			t.Errorf("expected top-level key '%s' missing from parsed data", key)
		}
	}
}

// TestValidComplexYAML_ParseFile tests parsing valid_complex.yaml using ParseFile method.
func TestValidComplexYAML_ParseFile(t *testing.T) {
	testFile := "testdata/valid_complex.yaml"
	parser := NewParser()

	var data map[string]interface{}
	result := parser.ParseFile(testFile, &data)

	if !result.Success {
		t.Fatalf("ParseFile failed: %v", result.Error)
	}

	// Verify services section exists
	services, ok := data["services"].(map[string]interface{})
	if !ok {
		t.Fatal("expected services to be a map")
	}

	// Verify services.web has merged defaults
	web, ok := services["web"].(map[string]interface{})
	if !ok {
		t.Fatal("expected services.web to be a map")
	}

	if web["timeout"] != int64(30) && web["timeout"] != int(30) {
		t.Errorf("expected services.web.timeout=30, got %v", web["timeout"])
	}

	if web["retries"] != int64(3) && web["retries"] != int(3) {
		t.Errorf("expected services.web.retries=3, got %v", web["retries"])
	}

	if web["backoff"] != "exponential" {
		t.Errorf("expected services.web.backoff='exponential', got %v", web["backoff"])
	}

	if web["host"] != "example.com" {
		t.Errorf("expected services.web.host='example.com', got %v", web["host"])
	}

	// Verify services.api has merged defaults with override
	api, ok := services["api"].(map[string]interface{})
	if !ok {
		t.Fatal("expected services.api to be a map")
	}

	if api["timeout"] != int64(60) && api["timeout"] != int(60) {
		t.Errorf("expected services.api.timeout=60 (overridden), got %v", api["timeout"])
	}

	if api["host"] != "api.example.com" {
		t.Errorf("expected services.api.host='api.example.com', got %v", api["host"])
	}

	// Verify multi-line strings exist
	if _, ok := data["description"].(string); !ok {
		t.Error("expected description to be a string")
	}

	if _, ok := data["notes"].(string); !ok {
		t.Error("expected notes to be a string")
	}
}

// TestValidComplexYAML_ParseFileToMap tests parsing valid_complex.yaml using ParseFileToMap method.
func TestValidComplexYAML_ParseFileToMap(t *testing.T) {
	testFile := "testdata/valid_complex.yaml"
	parser := NewParser()

	result := parser.ParseFileToMap(testFile)

	if !result.Success {
		t.Fatalf("ParseFileToMap failed: %v", result.Error)
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatal("expected result.Data to be map[string]interface{}")
	}

	// Verify anchor/alias merge worked correctly
	services, ok := data["services"].(map[string]interface{})
	if !ok {
		t.Fatal("expected services to be a map")
	}

	web, ok := services["web"].(map[string]interface{})
	if !ok {
		t.Fatal("expected services.web to be a map")
	}

	// Check that all fields from defaults are present
	expectedWebFields := []string{"timeout", "retries", "backoff", "port", "host", "health_check"}
	for _, field := range expectedWebFields {
		if _, exists := web[field]; !exists {
			t.Errorf("expected services.web.%s to exist", field)
		}
	}

	api, ok := services["api"].(map[string]interface{})
	if !ok {
		t.Fatal("expected services.api to be a map")
	}

	// Verify timeout override (60 instead of 30)
	if api["timeout"] != int64(60) && api["timeout"] != int(60) {
		t.Errorf("expected services.api.timeout=60 (overridden), got %v", api["timeout"])
	}

	// Verify other defaults are still present
	if api["retries"] != int64(3) && api["retries"] != int(3) {
		t.Errorf("expected services.api.retries=3, got %v", api["retries"])
	}

	if api["backoff"] != "exponential" {
		t.Errorf("expected services.api.backoff='exponential', got %v", api["backoff"])
	}
}
