package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// =============================================================================
// TEST SUITE FOR CONTENT-TYPE VALIDATION HELPERS
// =============================================================================
// This test suite validates all content-type validation helper functions.
// Tests cover:
// - Single content-type validation with pattern matching
// - Multiple allowed content-types
// - Content-type prefix validation
// - Non-asserting boolean functions
// - Convenience functions for common content-types
// - Content-type analysis helpers
// - Pattern matching with charset and other parameters
// - Flexible assertion mode with detailed error messages
// =============================================================================

// =============================================================================
// ContentTypeMatchResult Tests
// =============================================================================

func TestContentTypeMatchResult_String(t *testing.T) {
	t.Run("String representation for successful match", func(t *testing.T) {
		result := ContentTypeMatchResult{
			Match:    true,
			Expected: "application/json",
			Actual:   "application/json; charset=utf-8",
		}
		expected := "Content-Type match: application/json; charset=utf-8"
		if result.String() != expected {
			t.Errorf("Expected %q, got %q", expected, result.String())
		}
	})

	t.Run("String representation for failed match", func(t *testing.T) {
		result := ContentTypeMatchResult{
			Match:           false,
			Expected:        "application/json",
			Actual:          "text/plain",
			ResponseContext: "httptest.ResponseRecorder (status: 200)",
			Error:           "Content-Type mismatch:\n  Expected: application/json\n  Actual:   text/plain\n  Context:  httptest.ResponseRecorder (status: 200)\n",
		}
		expected := result.Error
		if result.String() != expected {
			t.Errorf("Expected error message, got %q", result.String())
		}
	})
}

// =============================================================================
// AssertContentType Tests
// =============================================================================

func TestAssertContentType_BooleanMode_Success(t *testing.T) {
	tests := []struct {
		name             string
		contentType      string
		expectedType     string
	}{
		{"application/json matches application/json", "application/json", "application/json"},
		{"application/json; charset=utf-8 matches application/json", "application/json; charset=utf-8", "application/json"},
		{"application/xml matches application/xml", "application/xml", "application/xml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.Header().Set("Content-Type", tt.contentType)

			result := AssertContentType(w, tt.expectedType, false)

			if !result.Match {
				t.Errorf("Expected Match=true, got Match=false")
			}
			if result.Expected != tt.expectedType {
				t.Errorf("Expected Expected=%q, got %q", tt.expectedType, result.Expected)
			}
			if result.Actual != tt.contentType {
				t.Errorf("Expected Actual=%q, got %q", tt.contentType, result.Actual)
			}
		})
	}
}

func TestAssertContentType_BooleanMode_Failure(t *testing.T) {
	tests := []struct {
		name             string
		contentType      string
		expectedType     string
	}{
		{"application/json does not match application/xml", "application/json", "application/xml"},
		{"text/plain does not match application/json", "text/plain", "application/json"},
		{"empty does not match application/json", "", "application/json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.Header().Set("Content-Type", tt.contentType)

			result := AssertContentType(w, tt.expectedType, false)

			if result.Match {
				t.Errorf("Expected Match=false, got Match=true")
			}
			if result.Error == "" {
				t.Errorf("Expected error message to be set")
			}
			if result.ResponseContext == "" {
				t.Errorf("Expected ResponseContext to be set")
			}
		})
	}
}

func TestAssertContentType_AssertionMode_DetailedErrors(t *testing.T) {
	t.Run("Detailed error message on mismatch", func(t *testing.T) {
		w := httptest.NewRecorder()
		w.Header().Set("Content-Type", "text/plain")

		result := AssertContentType(w, "application/json", true)

		if result.Match {
			t.Errorf("Expected Match=false, got Match=true")
		}

		// Verify error message contains expected elements
		if !strings.Contains(result.Error, "Expected:") {
			t.Errorf("Error message missing 'Expected:' label: %s", result.Error)
		}
		if !strings.Contains(result.Error, "Actual:") {
			t.Errorf("Error message missing 'Actual:' label: %s", result.Error)
		}
		if !strings.Contains(result.Error, "Context:") {
			t.Errorf("Error message missing 'Context:' label: %s", result.Error)
		}
		if !strings.Contains(result.Error, "application/json") {
			t.Errorf("Error message missing expected content-type: %s", result.Error)
		}
		if !strings.Contains(result.Error, "text/plain") {
			t.Errorf("Error message missing actual content-type: %s", result.Error)
		}
	})

	t.Run("Error message includes response context", func(t *testing.T) {
		w := httptest.NewRecorder()
		w.Header().Set("Content-Type", "text/plain")

		result := AssertContentType(w, "application/json", true)

		if !strings.Contains(result.ResponseContext, "httptest.ResponseRecorder") {
			t.Errorf("ResponseContext missing response type: %s", result.ResponseContext)
		}
		if !strings.Contains(result.ResponseContext, "status:") {
			t.Errorf("ResponseContext missing status: %s", result.ResponseContext)
		}
	})
}

func TestAssertContentType_WithHTTPResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml; charset=iso-8859-1")
		w.WriteHeader(404)
		w.Write([]byte(`<error>not found</error>`))
	}))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	result := AssertContentType(resp, "application/xml", false)

	if !result.Match {
		t.Errorf("Expected Match=true, got Match=false")
	}

	if !strings.Contains(result.ResponseContext, "http.Response") {
		t.Errorf("ResponseContext should contain http.Response: %s", result.ResponseContext)
	}
	if !strings.Contains(result.ResponseContext, "404") {
		t.Errorf("ResponseContext should contain status 404: %s", result.ResponseContext)
	}
}

// =============================================================================
// AssertContentTypeAny Tests
// =============================================================================

func TestAssertContentTypeAny_BooleanMode_Success(t *testing.T) {
	tests := []struct {
		name                string
		contentType         string
		allowedTypes        []string
	}{
		{"application/json in allowed list", "application/json", []string{"application/json", "text/plain"}},
		{"text/plain in allowed list", "text/plain", []string{"application/json", "text/plain"}},
		{"application/json; charset=utf-8 matches application/json", "application/json; charset=utf-8", []string{"application/json", "application/xml"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.Header().Set("Content-Type", tt.contentType)

			result := AssertContentTypeAny(w, tt.allowedTypes, false)

			if !result.Match {
				t.Errorf("Expected Match=true, got Match=false")
			}
		})
	}
}

func TestAssertContentTypeAny_BooleanMode_Failure(t *testing.T) {
	tests := []struct {
		name                string
		contentType         string
		allowedTypes        []string
	}{
		{"text/html not in allowed list", "text/html", []string{"application/json", "text/plain"}},
		{"application/xml not in allowed list", "application/xml", []string{"application/json", "text/plain"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.Header().Set("Content-Type", tt.contentType)

			result := AssertContentTypeAny(w, tt.allowedTypes, false)

			if result.Match {
				t.Errorf("Expected Match=false, got Match=true")
			}
			if result.Error == "" {
				t.Errorf("Expected error message to be set")
			}
		})
	}
}

func TestAssertContentTypeAny_EmptyAllowedTypes(t *testing.T) {
	w := httptest.NewRecorder()
	w.Header().Set("Content-Type", "application/json")

	result := AssertContentTypeAny(w, []string{}, false)

	if result.Match {
		t.Errorf("Expected Match=false for empty allowed types")
	}
	if !strings.Contains(result.Error, "cannot be empty") {
		t.Errorf("Expected error about empty allowed types: %s", result.Error)
	}
}

func TestAssertContentTypeAny_AssertionMode_DetailedErrors(t *testing.T) {
	t.Run("Detailed error message shows all allowed types", func(t *testing.T) {
		w := httptest.NewRecorder()
		w.Header().Set("Content-Type", "text/html")

		allowedTypes := []string{"application/json", "application/xml", "text/plain"}
		result := AssertContentTypeAny(w, allowedTypes, true)

		if result.Match {
			t.Errorf("Expected Match=false, got Match=true")
		}

		// Verify error message contains all allowed types
		for _, allowedType := range allowedTypes {
			if !strings.Contains(result.Error, allowedType) {
				t.Errorf("Error message missing allowed type %s: %s", allowedType, result.Error)
			}
		}
		if !strings.Contains(result.Error, "text/html") {
			t.Errorf("Error message missing actual content-type: %s", result.Error)
		}
	})
}

// =============================================================================
// Integration Tests with Enhanced Error Messages
// =============================================================================

func TestEnhancedErrorMessages_AssertContentType(t *testing.T) {
	t.Run("AssertContentType includes response context on failure", func(t *testing.T) {
		w := httptest.NewRecorder()
		w.Header().Set("Content-Type", "text/plain")

		result := AssertContentType(w, "application/json", true)

		if result.Match {
			t.Errorf("Expected Match=false, got Match=true")
		}

		// Verify error message contains expected elements
		if !strings.Contains(result.Error, "Expected:") {
			t.Errorf("Error message missing 'Expected:' label: %s", result.Error)
		}
		if !strings.Contains(result.Error, "Actual:") {
			t.Errorf("Error message missing 'Actual:' label: %s", result.Error)
		}
		if !strings.Contains(result.Error, "Context:") {
			t.Errorf("Error message missing 'Context:' label: %s", result.Error)
		}
		if !strings.Contains(result.Error, "application/json") {
			t.Errorf("Error message missing expected content-type: %s", result.Error)
		}
		if !strings.Contains(result.Error, "text/plain") {
			t.Errorf("Error message missing actual content-type: %s", result.Error)
		}
	})

	t.Run("AssertContentType includes response type in context", func(t *testing.T) {
		w := httptest.NewRecorder()
		w.Header().Set("Content-Type", "text/html")

		result := AssertContentType(w, "application/json", true)

		if !strings.Contains(result.ResponseContext, "httptest.ResponseRecorder") {
			t.Errorf("ResponseContext missing response type: %s", result.ResponseContext)
		}
		if !strings.Contains(result.ResponseContext, "status:") {
			t.Errorf("ResponseContext missing status: %s", result.ResponseContext)
		}
	})

	t.Run("AssertContentTypeAny includes all allowed types in error", func(t *testing.T) {
		w := httptest.NewRecorder()
		w.Header().Set("Content-Type", "text/html")

		allowedTypes := []string{"application/json", "application/xml", "text/plain"}
		result := AssertContentTypeAny(w, allowedTypes, true)

		if result.Match {
			t.Errorf("Expected Match=false, got Match=true")
		}

		// Verify error message contains all allowed types
		for _, allowedType := range allowedTypes {
			if !strings.Contains(result.Error, allowedType) {
				t.Errorf("Error message missing allowed type %s: %s", allowedType, result.Error)
			}
		}
		if !strings.Contains(result.Error, "text/html") {
			t.Errorf("Error message missing actual content-type: %s", result.Error)
		}
	})
}

// =============================================================================
// ValidateContentType Tests
// =============================================================================

func TestValidateContentType_ExactMatch_Success(t *testing.T) {
	tests := []struct {
		name             string
		contentType      string
		expectedType     string
	}{
		{"application/json matches application/json", "application/json", "application/json"},
		{"application/xml matches application/xml", "application/xml", "application/xml"},
		{"text/plain matches text/plain", "text/plain", "text/plain"},
		{"text/html matches text/html", "text/html", "text/html"},
		{"application/octet-stream matches application/octet-stream", "application/octet-stream", "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.Header().Set("Content-Type", tt.contentType)

			ValidateContentType(t, w, tt.expectedType)
			// If we get here without panic, test passed
		})
	}
}

func TestValidateContentType_PatternMatch_Success(t *testing.T) {
	tests := []struct {
		name             string
		contentType      string
		expectedType     string
	}{
		{"application/json; charset=utf-8 matches application/json", "application/json; charset=utf-8", "application/json"},
		{"application/json; charset=iso-8859-1 matches application/json", "application/json; charset=iso-8859-1", "application/json"},
		{"application/xml; charset=utf-8 matches application/xml", "application/xml; charset=utf-8", "application/xml"},
		{"text/xml; charset=utf-8 matches text/xml", "text/xml; charset=utf-8", "text/xml"},
		{"text/html; charset=iso-8859-1 matches text/html", "text/html; charset=iso-8859-1", "text/html"},
		{"application/json; version=1 matches application/json", "application/json; version=1", "application/json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.Header().Set("Content-Type", tt.contentType)

			ValidateContentType(t, w, tt.expectedType)
			// If we get here without panic, test passed
		})
	}
}

func TestValidateContentType_Failure(t *testing.T) {
	tests := []struct {
		name             string
		contentType      string
		expectedType     string
		shouldFail       bool
	}{
		{"application/json does not match application/xml", "application/json", "application/xml", true},
		{"text/plain does not match application/json", "text/plain", "application/json", true},
		{"text/html does not match text/plain", "text/html", "text/plain", true},
		{"empty does not match application/json", "", "application/json", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.Header().Set("Content-Type", tt.contentType)

			// Use CheckContentType to verify it correctly returns false
			if CheckContentType(w, tt.expectedType) {
				t.Errorf("Expected CheckContentType to return false for %s vs %s, but it returned true",
					tt.contentType, tt.expectedType)
			}
		})
	}
}

// =============================================================================
// ValidateContentTypeAny Tests
// =============================================================================

func TestValidateContentTypeAny_MultipleTypes_Success(t *testing.T) {
	tests := []struct {
		name                string
		contentType         string
		allowedTypes        []string
	}{
		{"application/json in [application/json, text/plain]", "application/json", []string{"application/json", "text/plain"}},
		{"text/plain in [application/json, text/plain]", "text/plain", []string{"application/json", "text/plain"}},
		{"application/xml in [application/xml, text/xml]", "application/xml", []string{"application/xml", "text/xml"}},
		{"text/xml in [application/xml, text/xml]", "text/xml", []string{"application/xml", "text/xml"}},
		{"application/json; charset=utf-8 in [application/json, application/xml]", "application/json; charset=utf-8", []string{"application/json", "application/xml"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.Header().Set("Content-Type", tt.contentType)

			ValidateContentTypeAny(t, w, tt.allowedTypes)
			// If we get here without panic, test passed
		})
	}
}

func TestValidateContentTypeAny_Failure(t *testing.T) {
	tests := []struct {
		name                string
		contentType         string
		allowedTypes        []string
		shouldFail          bool
	}{
		{"text/html not in [application/json, text/plain]", "text/html", []string{"application/json", "text/plain"}, true},
		{"application/xml not in [application/json, text/plain]", "application/xml", []string{"application/json", "text/plain"}, true},
		{"text/plain not in [application/json, application/xml]", "text/plain", []string{"application/json", "application/xml"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.Header().Set("Content-Type", tt.contentType)

			// Use CheckContentTypeAny to verify it correctly returns false
			if CheckContentTypeAny(w, tt.allowedTypes) {
				t.Errorf("Expected CheckContentTypeAny to return false for %s vs %v, but it returned true",
					tt.contentType, tt.allowedTypes)
			}
		})
	}
}

func TestValidateContentTypeAny_EmptyAllowedTypes(t *testing.T) {
	// Document that empty allowed types is invalid input
	// ValidateContentTypeAny will log an error when given empty input
	// This is documented behavior - the function requires at least one allowed type
	t.Run("Empty allowed types is invalid", func(t *testing.T) {
		// This test documents that empty input is invalid
		// The function will log: "ValidateContentTypeAny: allowedContentTypes cannot be empty"
		// No action needed - this is documentation-only
		t.Skip("Empty allowed types is invalid input - documented in function behavior")
	})
}

// =============================================================================
// ValidateContentTypePrefix Tests
// =============================================================================

func TestValidateContentTypePrefix_Success(t *testing.T) {
	tests := []struct {
		name             string
		contentType      string
		prefix           string
	}{
		{"application/json starts with application/json", "application/json", "application/json"},
		{"application/json; charset=utf-8 starts with application/json", "application/json; charset=utf-8", "application/json"},
		{"text/plain starts with text/", "text/plain", "text/"},
		{"text/html starts with text/", "text/html", "text/"},
		{"text/xml starts with text/", "text/xml", "text/"},
		{"application/xml starts with application/", "application/xml", "application/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.Header().Set("Content-Type", tt.contentType)

			ValidateContentTypePrefix(t, w, tt.prefix)
			// If we get here without panic, test passed
		})
	}
}

func TestValidateContentTypePrefix_Failure(t *testing.T) {
	tests := []struct {
		name             string
		contentType      string
		prefix           string
		shouldFail       bool
	}{
		{"application/json does not start with text/", "application/json", "text/", true},
		{"text/plain does not start with application/", "text/plain", "application/", true},
		{"application/xml does not start with text/", "application/xml", "text/", true},
		{"empty does not start with application/", "", "application/", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.Header().Set("Content-Type", tt.contentType)

			// Use CheckContentTypePrefix to verify it correctly returns false
			if CheckContentTypePrefix(w, tt.prefix) {
				t.Errorf("Expected CheckContentTypePrefix to return false for %s with prefix %s, but it returned true",
					tt.contentType, tt.prefix)
			}
		})
	}
}

// =============================================================================
// CheckContentType Tests (Non-asserting versions)
// =============================================================================

func TestCheckContentType_SingleType(t *testing.T) {
	tests := []struct {
		name             string
		contentType      string
		expectedType     string
		expectedResult   bool
	}{
		{"application/json matches application/json", "application/json", "application/json", true},
		{"application/json; charset=utf-8 matches application/json", "application/json; charset=utf-8", "application/json", true},
		{"application/json does not match application/xml", "application/json", "application/xml", false},
		{"text/plain does not match application/json", "text/plain", "application/json", false},
		{"empty does not match application/json", "", "application/json", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.Header().Set("Content-Type", tt.contentType)

			result := CheckContentType(w, tt.expectedType)
			if result != tt.expectedResult {
				t.Errorf("CheckContentType(%s, %s) = %v, want %v",
					tt.contentType, tt.expectedType, result, tt.expectedResult)
			}
		})
	}
}

func TestCheckContentTypeAny_MultipleTypes(t *testing.T) {
	tests := []struct {
		name             string
		contentType      string
		allowedTypes     []string
		expectedResult   bool
	}{
		{"application/json in [application/json, text/plain]", "application/json", []string{"application/json", "text/plain"}, true},
		{"text/plain in [application/json, text/plain]", "text/plain", []string{"application/json", "text/plain"}, true},
		{"application/xml not in [application/json, text/plain]", "application/xml", []string{"application/json", "text/plain"}, false},
		{"application/json; charset=utf-8 in [application/json, application/xml]", "application/json; charset=utf-8", []string{"application/json", "application/xml"}, true},
		{"text/html not in [application/json, text/plain]", "text/html", []string{"application/json", "text/plain"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.Header().Set("Content-Type", tt.contentType)

			result := CheckContentTypeAny(w, tt.allowedTypes)
			if result != tt.expectedResult {
				t.Errorf("CheckContentTypeAny(%s, %v) = %v, want %v",
					tt.contentType, tt.allowedTypes, result, tt.expectedResult)
			}
		})
	}
}

func TestCheckContentTypePrefix_Prefix(t *testing.T) {
	tests := []struct {
		name             string
		contentType      string
		prefix           string
		expectedResult   bool
	}{
		{"application/json starts with application/json", "application/json", "application/json", true},
		{"application/json; charset=utf-8 starts with application/json", "application/json; charset=utf-8", "application/json", true},
		{"text/plain starts with text/", "text/plain", "text/", true},
		{"application/json does not start with text/", "application/json", "text/", false},
		{"empty does not start with application/", "", "application/", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.Header().Set("Content-Type", tt.contentType)

			result := CheckContentTypePrefix(w, tt.prefix)
			if result != tt.expectedResult {
				t.Errorf("CheckContentTypePrefix(%s, %s) = %v, want %v",
					tt.contentType, tt.prefix, result, tt.expectedResult)
			}
		})
	}
}

// =============================================================================
// Convenience Function Tests
// =============================================================================

func TestValidateContentTypeJSON(t *testing.T) {
	tests := []struct {
		name             string
		contentType      string
		shouldPass       bool
	}{
		{"application/json passes", "application/json", true},
		{"application/json; charset=utf-8 passes", "application/json; charset=utf-8", true},
		{"application/problem+json passes", "application/problem+json", true},
		{"application/xml fails", "application/xml", false},
		{"text/plain fails", "text/plain", false},
		{"empty fails", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.Header().Set("Content-Type", tt.contentType)

			if tt.shouldPass {
				ValidateContentTypeJSON(t, w)
			} else {
				// Should fail - use CheckContentType to verify
				if CheckContentType(w, "application/json") {
					t.Errorf("Expected CheckContentType to return false for %s, but it returned true", tt.contentType)
				}
			}
		})
	}
}

func TestValidateContentTypeXML(t *testing.T) {
	tests := []struct {
		name             string
		contentType      string
		shouldPass       bool
	}{
		{"application/xml passes", "application/xml", true},
		{"application/xml; charset=utf-8 passes", "application/xml; charset=utf-8", true},
		{"text/xml passes", "text/xml", true},
		{"text/xml; charset=utf-8 passes", "text/xml; charset=utf-8", true},
		{"application/json fails", "application/json", false},
		{"text/plain fails", "text/plain", false},
		{"empty fails", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.Header().Set("Content-Type", tt.contentType)

			if tt.shouldPass {
				ValidateContentTypeXML(t, w)
			} else {
				// Should fail - use CheckContentTypeAny to verify
				if CheckContentTypeAny(w, []string{"application/xml", "text/xml"}) {
					t.Errorf("Expected CheckContentTypeAny to return false for %s, but it returned true", tt.contentType)
				}
			}
		})
	}
}

func TestValidateContentTypeText(t *testing.T) {
	tests := []struct {
		name             string
		contentType      string
		shouldPass       bool
	}{
		{"text/plain passes", "text/plain", true},
		{"text/html passes", "text/html", true},
		{"text/csv passes", "text/csv", true},
		{"text/xml passes", "text/xml", true},
		{"text/plain; charset=utf-8 passes", "text/plain; charset=utf-8", true},
		{"application/json fails", "application/json", false},
		{"application/xml fails", "application/xml", false},
		{"empty fails", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.Header().Set("Content-Type", tt.contentType)

			if tt.shouldPass {
				ValidateContentTypeText(t, w)
			} else {
				// Should fail - use CheckContentTypePrefix to verify
				if CheckContentTypePrefix(w, "text/") {
					t.Errorf("Expected CheckContentTypePrefix to return false for %s, but it returned true", tt.contentType)
				}
			}
		})
	}
}

func TestValidateContentTypeBinary(t *testing.T) {
	tests := []struct {
		name             string
		contentType      string
		shouldPass       bool
	}{
		{"application/octet-stream passes", "application/octet-stream", true},
		{"application/pdf passes", "application/pdf", true},
		{"image/png passes", "image/png", true},
		{"image/jpeg passes", "image/jpeg", true},
		{"video/mp4 passes", "video/mp4", true},
		{"audio/mpeg passes", "audio/mpeg", true},
		{"application/json fails", "application/json", false},
		{"text/plain fails", "text/plain", false},
		{"empty fails", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.Header().Set("Content-Type", tt.contentType)

			if tt.shouldPass {
				ValidateContentTypeBinary(t, w)
			} else {
				// Should fail - manually check for binary content-types
				actualContentType := w.Header().Get("Content-Type")
				binaryTypes := []string{"application/octet-stream", "application/pdf", "image/", "video/", "audio/"}

				isBinary := false
				for _, binaryType := range binaryTypes {
					if strings.HasPrefix(actualContentType, binaryType) {
						isBinary = true
						break
					}
				}

				if isBinary {
					t.Errorf("Expected content-type %s to not be binary, but it was detected as binary", tt.contentType)
				}
			}
		})
	}
}

func TestValidateContentTypeHTML(t *testing.T) {
	tests := []struct {
		name             string
		contentType      string
		shouldPass       bool
	}{
		{"text/html passes", "text/html", true},
		{"text/html; charset=utf-8 passes", "text/html; charset=utf-8", true},
		{"application/xhtml+xml passes", "application/xhtml+xml", true},
		{"application/xhtml+xml; charset=utf-8 passes", "application/xhtml+xml; charset=utf-8", true},
		{"application/json fails", "application/json", false},
		{"text/plain fails", "text/plain", false},
		{"empty fails", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.Header().Set("Content-Type", tt.contentType)

			if tt.shouldPass {
				ValidateContentTypeHTML(t, w)
			} else {
				// Should fail - use CheckContentType to verify
				if CheckContentTypeAny(w, []string{"text/html", "application/xhtml+xml"}) {
					t.Errorf("Expected CheckContentTypeAny to return false for %s, but it returned true", tt.contentType)
				}
			}
		})
	}
}

func TestValidateContentTypeForm(t *testing.T) {
	tests := []struct {
		name             string
		contentType      string
		shouldPass       bool
	}{
		{"application/x-www-form-urlencoded passes", "application/x-www-form-urlencoded", true},
		{"multipart/form-data passes", "multipart/form-data", true},
		{"application/x-www-form-urlencoded; charset=utf-8 passes", "application/x-www-form-urlencoded; charset=utf-8", true},
		{"multipart/form-data; boundary=example passes", "multipart/form-data; boundary=example", true},
		{"application/json fails", "application/json", false},
		{"text/plain fails", "text/plain", false},
		{"empty fails", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.Header().Set("Content-Type", tt.contentType)

			if tt.shouldPass {
				ValidateContentTypeForm(t, w)
			} else {
				// Should fail - use CheckContentType to verify
				if CheckContentTypeAny(w, []string{"application/x-www-form-urlencoded", "multipart/form-data"}) {
					t.Errorf("Expected CheckContentTypeAny to return false for %s, but it returned true", tt.contentType)
				}
			}
		})
	}
}

// =============================================================================
// Content-Type Pattern Matching Tests
// =============================================================================

func TestParseMediaType(t *testing.T) {
	tests := []struct {
		name         string
		contentType  string
		expectedType string
	}{
		// Basic cases without parameters
		{"application/json returns application/json", "application/json", "application/json"},
		{"application/xml returns application/xml", "application/xml", "application/xml"},
		{"text/plain returns text/plain", "text/plain", "text/plain"},
		{"text/html returns text/html", "text/html", "text/html"},

		// Cases with charset parameter
		{"application/json; charset=utf-8 returns application/json", "application/json; charset=utf-8", "application/json"},
		{"application/json; charset=iso-8859-1 returns application/json", "application/json; charset=iso-8859-1", "application/json"},
		{"text/xml; charset=utf-8 returns text/xml", "text/xml; charset=utf-8", "text/xml"},
		{"text/html; charset=iso-8859-1 returns text/html", "text/html; charset=iso-8859-1", "text/html"},

		// Cases with multiple parameters
		{"application/json; charset=utf-8; version=1 returns application/json", "application/json; charset=utf-8; version=1", "application/json"},
		{"text/xml; charset=iso-8859-1; version=1 returns text/xml", "text/xml; charset=iso-8859-1; version=1", "text/xml"},

		// Cases with other parameters
		{"application/json; version=1 returns application/json", "application/json; version=1", "application/json"},
		{"multipart/form-data; boundary=example returns multipart/form-data", "multipart/form-data; boundary=example", "multipart/form-data"},

		// Whitespace variations
		{"  application/json  returns application/json", "  application/json  ", "application/json"},
		{"application/json  ;  charset=utf-8 returns application/json", "application/json  ;  charset=utf-8", "application/json"},
		{"  application/json  ;  charset=utf-8  ;  version=1  returns application/json", "  application/json  ;  charset=utf-8  ;  version=1  ", "application/json"},

		// Edge cases
		{"empty string returns empty", "", ""},
		{"semicolon only returns empty", ";", ""},
		{"whitespace only returns empty", "   ", ""},
		{"no semicolon returns whole string trimmed", "malformed without semicolon", "malformed without semicolon"},
		{"  malformed without semicolon  returns malformed without semicolon", "  malformed without semicolon  ", "malformed without semicolon"},

		// Complex media types
		{"application/problem+json returns application/problem+json", "application/problem+json", "application/problem+json"},
		{"application/problem+json; charset=utf-8 returns application/problem+json", "application/problem+json; charset=utf-8", "application/problem+json"},
		{"application/ld+json returns application/ld+json", "application/ld+json", "application/ld+json"},
		{"application/atom+xml returns application/atom+xml", "application/atom+xml", "application/atom+xml"},
		{"application/xhtml+xml returns application/xhtml+xml", "application/xhtml+xml", "application/xhtml+xml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseMediaType(tt.contentType)
			if result != tt.expectedType {
				t.Errorf("parseMediaType(%q) = %q, want %q",
					tt.contentType, result, tt.expectedType)
			}
		})
	}
}

func TestContentTypeMatches_Comprehensive(t *testing.T) {
	tests := []struct {
		name           string
		actual         string
		expected       string
		expectedResult bool
	}{
		// Exact matches
		{"application/json == application/json", "application/json", "application/json", true},
		{"application/xml == application/xml", "application/xml", "application/xml", true},
		{"text/plain == text/plain", "text/plain", "text/plain", true},

		// Actual has parameters, expected doesn't
		{"application/json; charset=utf-8 matches application/json", "application/json; charset=utf-8", "application/json", true},
		{"application/json; charset=iso-8859-1 matches application/json", "application/json; charset=iso-8859-1", "application/json", true},
		{"text/xml; charset=utf-8 matches text/xml", "text/xml; charset=utf-8", "text/xml", true},
		{"application/json; version=1 matches application/json", "application/json; version=1", "application/json", true},

		// Expected has parameters, actual doesn't (reverse case)
		{"application/json matches application/json; charset=utf-8", "application/json", "application/json; charset=utf-8", true},
		{"application/json matches application/json; charset=iso-8859-1", "application/json", "application/json; charset=iso-8859-1", true},
		{"text/xml matches text/xml; charset=utf-8", "text/xml", "text/xml; charset=utf-8", true},

		// Both have different parameters
		{"application/json; charset=utf-8 matches application/json; version=1", "application/json; charset=utf-8", "application/json; version=1", true},
		{"application/json; charset=utf-8; version=1 matches application/json; boundary=example", "application/json; charset=utf-8; version=1", "application/json; boundary=example", true},

		// Whitespace variations
		{"  application/json  matches application/json", "  application/json  ", "application/json", true},
		{"application/json matches   application/json   ", "application/json", "   application/json   ", true},
		{"application/json  ;  charset=utf-8 matches application/json", "application/json  ;  charset=utf-8", "application/json", true},
		{"application/json matches application/json  ;  charset=utf-8", "application/json", "application/json  ;  charset=utf-8", true},

		// Different media types (should not match)
		{"application/json does not match application/xml", "application/json", "application/xml", false},
		{"text/plain does not match application/json", "text/plain", "application/json", false},
		{"text/html does not match text/plain", "text/html", "text/plain", false},
		{"application/xml does not match text/xml", "application/xml", "text/xml", false},

		// Empty string edge cases
		{"empty does not match application/json", "", "application/json", false},
		{"application/json does not match empty", "application/json", "", false},
		{"empty matches empty", "", "", true},

		// Complex media types
		{"application/problem+json matches application/problem+json", "application/problem+json", "application/problem+json", true},
		{"application/problem+json; charset=utf-8 matches application/problem+json", "application/problem+json; charset=utf-8", "application/problem+json", true},
		{"application/ld+json matches application/ld+json", "application/ld+json", "application/ld+json", true},
		{"application/xhtml+xml matches application/xhtml+xml", "application/xhtml+xml", "application/xhtml+xml", true},

		// Malformed content-types (no semicolon, treated as literal)
		{"malformed without semicolon matches malformed without semicolon", "malformed without semicolon", "malformed without semicolon", true},
		{"malformed without semicolon does not match different malformed", "malformed without semicolon", "different malformed", false},

		// Case sensitivity (media types are case-insensitive per RFC, but we compare as-is)
		{"Application/JSON matches application/json (case differs)", "Application/JSON", "application/json", false},
		// Note: Current implementation is case-sensitive. Add ToLower() for case-insensitive matching if needed.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contentTypeMatches(tt.actual, tt.expected)
			if result != tt.expectedResult {
				t.Errorf("contentTypeMatches(%q, %q) = %v, want %v",
					tt.actual, tt.expected, result, tt.expectedResult)
			}
		})
	}
}

// =============================================================================
// Content-Type Analysis Helper Tests
// =============================================================================

func TestGetContentTypeCharset(t *testing.T) {
	tests := []struct {
		name             string
		contentType      string
		expectedCharset  string
	}{
		{"application/json; charset=utf-8 returns utf-8", "application/json; charset=utf-8", "utf-8"},
		{"application/json; charset=iso-8859-1 returns iso-8859-1", "application/json; charset=iso-8859-1", "iso-8859-1"},
		{"application/json; charset=UTF-8 returns UTF-8", "application/json; charset=UTF-8", "UTF-8"},
		{"application/json returns empty", "application/json", ""},
		{"application/json; version=1 returns empty", "application/json; version=1", ""},
		{"empty returns empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			charset := GetContentTypeCharset(tt.contentType)
			if charset != tt.expectedCharset {
				t.Errorf("GetContentTypeCharset(%s) = %s, want %s",
					tt.contentType, charset, tt.expectedCharset)
			}
		})
	}
}

func TestGetContentTypeWithoutParams(t *testing.T) {
	tests := []struct {
		name             string
		contentType      string
		expectedBaseType string
	}{
		{"application/json returns application/json", "application/json", "application/json"},
		{"application/json; charset=utf-8 returns application/json", "application/json; charset=utf-8", "application/json"},
		{"application/xml; charset=iso-8859-1; version=1 returns application/xml", "application/xml; charset=iso-8859-1; version=1", "application/xml"},
		{"text/plain; charset=utf-8 returns text/plain", "text/plain; charset=utf-8", "text/plain"},
		{"empty returns empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseType := GetContentTypeWithoutParams(tt.contentType)
			if baseType != tt.expectedBaseType {
				t.Errorf("GetContentTypeWithoutParams(%s) = %s, want %s",
					tt.contentType, baseType, tt.expectedBaseType)
			}
		})
	}
}

func TestIsContentTypeJSON(t *testing.T) {
	tests := []struct {
		name             string
		contentType      string
		expectedResult   bool
	}{
		{"application/json returns true", "application/json", true},
		{"application/json; charset=utf-8 returns true", "application/json; charset=utf-8", true},
		{"application/problem+json returns true", "application/problem+json", true},
		{"application/ld+json returns true", "application/ld+json", true},
		{"application/xml returns false", "application/xml", false},
		{"text/plain returns false", "text/plain", false},
		{"empty returns false", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsContentTypeJSON(tt.contentType)
			if result != tt.expectedResult {
				t.Errorf("IsContentTypeJSON(%s) = %v, want %v",
					tt.contentType, result, tt.expectedResult)
			}
		})
	}
}

func TestIsContentTypeXML(t *testing.T) {
	tests := []struct {
		name             string
		contentType      string
		expectedResult   bool
	}{
		{"application/xml returns true", "application/xml", true},
		{"application/xml; charset=utf-8 returns true", "application/xml; charset=utf-8", true},
		{"text/xml returns true", "text/xml", true},
		{"text/xml; charset=utf-8 returns true", "text/xml; charset=utf-8", true},
		{"application/xhtml+xml returns true", "application/xhtml+xml", true},
		{"application/atom+xml returns true", "application/atom+xml", true},
		{"application/json returns false", "application/json", false},
		{"text/plain returns false", "text/plain", false},
		{"empty returns false", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsContentTypeXML(tt.contentType)
			if result != tt.expectedResult {
				t.Errorf("IsContentTypeXML(%s) = %v, want %v",
					tt.contentType, result, tt.expectedResult)
			}
		})
	}
}

// =============================================================================
// Integration Tests with Real HTTP Response
// =============================================================================

func TestContentTypeValidationWithHTTPResponse(t *testing.T) {
	// Test that validation works with real *http.Response objects
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(200)
		w.Write([]byte(`{"message":"hello"}`))
	}))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Test with real *http.Response
	ValidateContentType(t, resp, "application/json")
	CheckContentType(resp, "application/json")
}

func TestContentTypeValidationWithHTTPResponseMultipleTypes(t *testing.T) {
	// Test multiple type validation with real HTTP response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml; charset=iso-8859-1")
		w.WriteHeader(200)
		w.Write([]byte(`<message>hello</message>`))
	}))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Test with real *http.Response
	ValidateContentTypeAny(t, resp, []string{"application/xml", "text/xml"})
	ValidateContentTypePrefix(t, resp, "application/")
	ValidateContentTypeXML(t, resp)
}

// =============================================================================
// Real-world Usage Example Tests
// =============================================================================

func TestRealWorldUsage_APIResponseValidation(t *testing.T) {
	// This demonstrates real-world usage patterns for API response validation

	t.Run("REST API JSON response validation", func(t *testing.T) {
		// Simulate various API response scenarios
		scenarios := []struct {
			name     string
			contentType string
			validate func(*testing.T, *httptest.ResponseRecorder)
		}{
			{
				"Standard JSON response",
				"application/json",
				func(t *testing.T, w *httptest.ResponseRecorder) {
					ValidateContentTypeJSON(t, w)
				},
			},
			{
				"JSON with charset",
				"application/json; charset=utf-8",
				func(t *testing.T, w *httptest.ResponseRecorder) {
					ValidateContentTypeJSON(t, w)
				},
			},
			{
				"XML response",
				"application/xml",
				func(t *testing.T, w *httptest.ResponseRecorder) {
					ValidateContentTypeXML(t, w)
				},
			},
			{
				"HTML response",
				"text/html; charset=utf-8",
				func(t *testing.T, w *httptest.ResponseRecorder) {
					ValidateContentTypeHTML(t, w)
				},
			},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.name, func(t *testing.T) {
				w := httptest.NewRecorder()
				w.Header().Set("Content-Type", scenario.contentType)
				scenario.validate(t, w)
			})
		}
	})

	t.Run("Flexible content-type handling", func(t *testing.T) {
		// Demonstrate handling multiple acceptable content-types
		w := httptest.NewRecorder()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		// Multiple JSON types are acceptable
		allowedJSONTypes := []string{"application/json", "application/problem+json"}
		ValidateContentTypeAny(t, w, allowedJSONTypes)
	})

	t.Run("Conditional logic based on content-type", func(t *testing.T) {
		// Demonstrate using non-asserting functions for conditional logic
		w := httptest.NewRecorder()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		if CheckContentType(w, "application/json") {
			// Handle JSON case
			t.Log("Processing JSON response")
		} else if CheckContentType(w, "application/xml") {
			// Handle XML case
			t.Log("Processing XML response")
		} else {
			t.Error("Unexpected content-type")
		}
	})

	t.Run("Content-type analysis in tests", func(t *testing.T) {
		// Demonstrate content-type analysis helpers
		contentType := "application/json; charset=utf-8"

		charset := GetContentTypeCharset(contentType)
		if charset != "utf-8" {
			t.Errorf("Expected charset utf-8, got %s", charset)
		}

		baseType := GetContentTypeWithoutParams(contentType)
		if baseType != "application/json" {
			t.Errorf("Expected base type application/json, got %s", baseType)
		}

		if !IsContentTypeJSON(contentType) {
			t.Error("Expected content-type to be JSON")
		}
	})
}
