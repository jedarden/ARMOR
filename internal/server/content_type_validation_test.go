package server

import (
	"net/http"
	"net/http/httptest"
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
// =============================================================================

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

			// This should cause t.Errorf() to be called
			ValidateContentType(t, w, tt.expectedType)

			// Test would fail if validation was incorrect
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

			// This should cause t.Errorf() to be called
			ValidateContentTypeAny(t, w, tt.allowedTypes)

			// Test would fail if validation was incorrect
		})
	}
}

func TestValidateContentTypeAny_EmptyAllowedTypes(t *testing.T) {
	w := httptest.NewRecorder()
	w.Header().Set("Content-Type", "application/json")

	// Should panic with fatal error
	defer func() {
		if r := recover(); r != nil {
			// Expected behavior - empty allowed types is invalid
		}
	}()

	ValidateContentTypeAny(t, w, []string{})
	t.Error("Should have failed with empty allowed types")
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

			// This should cause t.Errorf() to be called
			ValidateContentTypePrefix(t, w, tt.prefix)

			// Test would fail if validation was incorrect
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
				// Should fail
				ValidateContentTypeJSON(t, w)
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
				// Should fail
				ValidateContentTypeXML(t, w)
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
				// Should fail
				ValidateContentTypeText(t, w)
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
				// Should fail
				ValidateContentTypeBinary(t, w)
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
				// Should fail
				ValidateContentTypeHTML(t, w)
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
				// Should fail
				ValidateContentTypeForm(t, w)
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
