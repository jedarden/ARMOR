package validate

import (
	"fmt"
	"strings"
	"testing"
)

// =============================================================================
// COMPREHENSIVE TESTS FOR VALIDATION ERROR FORMATTING
// This file provides comprehensive coverage for error formatting functions
// to achieve >90% test coverage.
// =============================================================================

// =============================================================================
// TESTS FOR formatSeverityPrefix (LOW COVERAGE - 44.4%)
// =============================================================================

// TestFormatSeverityPrefix_AllCombinations tests all possible combinations
// of indicator and tag values to improve coverage of formatSeverityPrefix.
func TestFormatSeverityPrefix_AllCombinations(t *testing.T) {
	tests := []struct {
		name     string
		severity ErrorSeverity
		expected string
	}{
		{
			name:     "Critical severity - both indicator and tag",
			severity: SeverityCritical,
			expected: "[🚨 CRIT]",
		},
		{
			name:     "High severity - both indicator and tag",
			severity: SeverityHigh,
			expected: "[⚠️ HIGH]",
		},
		{
			name:     "Medium severity - both indicator and tag",
			severity: SeverityMedium,
			expected: "[⚡ MED]",
		},
		{
			name:     "Low severity - both indicator and tag",
			severity: SeverityLow,
			expected: "[ℹ️ LOW]",
		},
		{
			name:     "Info severity - both indicator and tag",
			severity: SeverityInfo,
			expected: "[💡 INFO]",
		},
		{
			name:     "Unknown severity - both indicator and tag",
			severity: ErrorSeverity("unknown"),
			expected: "[❓ UNK]",
		},
		{
			name:     "Empty severity - no indicator or tag",
			severity: ErrorSeverity(""),
			expected: "[❓ UNK]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatSeverityPrefix(tt.severity)
			if result != tt.expected {
				t.Errorf("formatSeverityPrefix(%v) = %q, want %q", tt.severity, result, tt.expected)
			}
		})
	}
}

// TestFormatSeverityPrefix_EdgeCases tests edge cases for severity formatting.
func TestFormatSeverityPrefix_EdgeCases(t *testing.T) {
	t.Run("whitespace severity", func(t *testing.T) {
		result := formatSeverityPrefix(ErrorSeverity("   "))
		// Should handle gracefully and return unknown indicator/tag
		if result != "[❓ UNK]" {
			t.Errorf("formatSeverityPrefix with whitespace = %q, want [❓ UNK]", result)
		}
	})

	t.Run("special characters in severity", func(t *testing.T) {
		result := formatSeverityPrefix(ErrorSeverity("🔥-critical"))
		if !strings.Contains(result, "[") || !strings.Contains(result, "]") {
			t.Error("formatSeverityPrefix should always return bracketed format")
		}
	})

	t.Run("very long severity string", func(t *testing.T) {
		longSeverity := ErrorSeverity(strings.Repeat("a", 1000))
		result := formatSeverityPrefix(longSeverity)
		// Should still work and return bracketed format
		if result == "" {
			t.Error("formatSeverityPrefix should handle long strings")
		}
	})
}

// =============================================================================
// TESTS FOR severityIndicator
// =============================================================================

// TestSeverityIndicator_AllSeverityLevels tests all defined severity levels.
func TestSeverityIndicator_AllSeverityLevels(t *testing.T) {
	tests := []struct {
		severity ErrorSeverity
		expected string
	}{
		{SeverityCritical, "🚨"},
		{SeverityHigh, "⚠️"},
		{SeverityMedium, "⚡"},
		{SeverityLow, "ℹ️"},
		{SeverityInfo, "💡"},
		{ErrorSeverity("unknown"), "❓"},
		{ErrorSeverity(""), "❓"},
	}

	for _, tt := range tests {
		t.Run(string(tt.severity), func(t *testing.T) {
			result := severityIndicator(tt.severity)
			if result != tt.expected {
				t.Errorf("severityIndicator(%v) = %q, want %q", tt.severity, result, tt.expected)
			}
		})
	}
}

// TestSeverityTag_AllSeverityLevels tests all severity tags.
func TestSeverityTag_AllSeverityLevels(t *testing.T) {
	tests := []struct {
		severity ErrorSeverity
		expected string
	}{
		{SeverityCritical, "CRIT"},
		{SeverityHigh, "HIGH"},
		{SeverityMedium, "MED"},
		{SeverityLow, "LOW"},
		{SeverityInfo, "INFO"},
		{ErrorSeverity("unknown"), "UNK"},
		{ErrorSeverity(""), "UNK"},
	}

	for _, tt := range tests {
		t.Run(string(tt.severity), func(t *testing.T) {
			result := severityTag(tt.severity)
			if result != tt.expected {
				t.Errorf("severityTag(%v) = %q, want %q", tt.severity, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// TESTS FOR generateValidationMessage (82.6% coverage - needs edge cases)
// =============================================================================

// TestGenerateValidationMessage_EdgeCases tests edge cases for message generation.
func TestGenerateValidationMessage_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		errorType string
		expected interface{}
		actual   interface{}
		wantContains []string
	}{
		{
			name:     "nil expected and nil actual",
			errorType: "validation",
			expected: nil,
			actual:   nil,
			wantContains: []string{"validation failed"},
		},
		{
			name:     "nil expected with actual",
			errorType: "status_code",
			expected: nil,
			actual:   404,
			wantContains: []string{"404", "Not Found"},
		},
		{
			name:     "expected with nil actual",
			errorType: "status_code",
			expected: 200,
			actual:   nil,
			wantContains: []string{"200", "OK"},
		},
		{
			name:     "slice of status codes as expected",
			errorType: "status_code",
			expected: []int{200, 201, 204},
			actual:   404,
			wantContains: []string{"one of", "200", "201", "204", "404"},
		},
		{
			name:     "empty slice as expected",
			errorType: "content_type",
			expected: []int{},
			actual:   "text/html",
			wantContains: []string{"validation failed"},
		},
		{
			name:     "string expected with string actual",
			errorType: "error_message",
			expected: "invalid.*token",
			actual:   "access_denied",
			wantContains: []string{"invalid.*token", "access_denied"},
		},
		{
			name:     "very long string actual (>50 chars)",
			errorType: "error_message",
			expected: "short",
			actual:   strings.Repeat("a", 100),
			wantContains: []string{"..."},
		},
		{
			name:     "boolean values",
			errorType: "validation",
			expected: true,
			actual:   false,
			wantContains: []string{"true", "false"},
		},
		{
			name:     "float values",
			errorType: "range",
			expected: 3.14,
			actual:   2.71,
			wantContains: []string{"3.14", "2.71"},
		},
		{
			name:     "map values",
			errorType: "structure",
			expected: map[string]interface{}{"key": "value"},
			actual:   map[string]interface{}{"key": "different"},
			wantContains: []string{"map"},
		},
		{
			name:     "slice of strings as expected",
			errorType: "enum",
			expected: []string{"a", "b", "c"},
			actual:   "d",
			wantContains: []string{"a", "b", "c"},
		},
		{
			name:     "integer expected with string actual",
			errorType: "type",
			expected: 100,
			actual:   "string",
			wantContains: []string{"100", "string"},
		},
		{
			name:     "string expected with integer actual",
			errorType: "type",
			expected: "number",
			actual:   12345,
			wantContains: []string{"number", "12345"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateValidationMessage(tt.errorType, tt.expected, tt.actual)

			// Check that all expected substrings are present
			for _, contain := range tt.wantContains {
				if !strings.Contains(result, contain) {
					t.Errorf("generateValidationMessage() result should contain %q, got: %s", contain, result)
				}
			}

			// Verify it contains the error type
			if !strings.Contains(result, tt.errorType) {
				t.Errorf("generateValidationMessage() should contain error type %q, got: %s", tt.errorType, result)
			}
		})
	}
}

// TestGenerateValidationMessage_StatusCodeDescriptions tests specific status codes.
func TestGenerateValidationMessage_StatusCodeDescriptions(t *testing.T) {
	statusCodes := []struct {
		code     int
		expected string
	}{
		{100, "Continue"},
		{200, "OK"},
		{201, "Created"},
		{204, "No Content"},
		{301, "Moved Permanently"},
		{302, "Found"},
		{304, "Not Modified"},
		{400, "Bad Request"},
		{401, "Unauthorized"},
		{403, "Forbidden"},
		{404, "Not Found"},
		{405, "Method Not Allowed"},
		{409, "Conflict"},
		{422, "Unprocessable Entity"},
		{429, "Too Many Requests"},
		{500, "Internal Server Error"},
		{502, "Bad Gateway"},
		{503, "Service Unavailable"},
		{504, "Gateway Timeout"},
	}

	for _, tt := range statusCodes {
		t.Run(fmt.Sprintf("status_%d", tt.code), func(t *testing.T) {
			result := generateValidationMessage("status_code", tt.code, nil)
			if !strings.Contains(result, tt.expected) {
				t.Errorf("Expected status code description %q for %d, got: %s", tt.expected, tt.code, result)
			}
		})
	}
}

// =============================================================================
// TESTS FOR getRangeInfo (85.7% coverage - needs error cases)
// =============================================================================

// TestGetRangeInfo_ValidPatterns tests valid range patterns.
func TestGetRangeInfo_ValidPatterns(t *testing.T) {
	tests := []struct {
		pattern      string
		expectedMin  int
		expectedMax  int
		expectedDesc string
	}{
		{"2xx", 200, 299, "Success (2xx)"},
		{"3xx", 300, 399, "Redirection (3xx)"},
		{"4xx", 400, 499, "Client Error (4xx)"},
		{"5xx", 500, 599, "Server Error (5xx)"},
		{"1xx", 100, 199, "Informational (1xx)"},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			min, max, desc, err := getRangeInfo(tt.pattern)

			if err != nil {
				t.Errorf("getRangeInfo(%q) returned unexpected error: %v", tt.pattern, err)
			}
			if min != tt.expectedMin {
				t.Errorf("getRangeInfo(%q) min = %d, want %d", tt.pattern, min, tt.expectedMin)
			}
			if max != tt.expectedMax {
				t.Errorf("getRangeInfo(%q) max = %d, want %d", tt.pattern, max, tt.expectedMax)
			}
			if desc != tt.expectedDesc {
				t.Errorf("getRangeInfo(%q) desc = %q, want %q", tt.pattern, desc, tt.expectedDesc)
			}
		})
	}
}

// TestGetRangeInfo_InvalidPatterns tests invalid range patterns.
func TestGetRangeInfo_InvalidPatterns(t *testing.T) {
	invalidPatterns := []string{
		"invalid",
		"6xx",
		"0xx",
		"7xx",
		"abc",
		"x2x",
		"2x",
		"xx",
		"2XXX",
		"2x ",
		" 2xx",
		"2 2",
		"-1xx",
		"999xx",
	}

	for _, pattern := range invalidPatterns {
		t.Run(pattern, func(t *testing.T) {
			_, _, _, err := getRangeInfo(pattern)
			if err == nil {
				t.Errorf("getRangeInfo(%q) should return error for invalid pattern", pattern)
			}
		})
	}
}

// TestGetRangeInfo_EdgeCases tests edge cases for range parsing.
func TestGetRangeInfo_EdgeCases(t *testing.T) {
	t.Run("pattern with mixed case", func(t *testing.T) {
		_, _, _, err := getRangeInfo("XxX")
		if err == nil {
			t.Error("getRangeInfo should return error for mixed case pattern")
		}
	})

	t.Run("pattern with special characters", func(t *testing.T) {
		_, _, _, err := getRangeInfo("2@x")
		if err == nil {
			t.Error("getRangeInfo should return error for pattern with special chars")
		}
	})

	t.Run("pattern with unicode", func(t *testing.T) {
		_, _, _, err := getRangeInfo("2🚨")
		if err == nil {
			t.Error("getRangeInfo should return error for pattern with unicode")
		}
	})

	t.Run("pattern with newline", func(t *testing.T) {
		_, _, _, err := getRangeInfo("2\nx")
		if err == nil {
			t.Error("getRangeInfo should return error for pattern with newline")
		}
	})
}

// =============================================================================
// TESTS FOR FormatFieldReference WITH QUOTE STYLES (92.6% coverage)
// =============================================================================

// TestFormatFieldReference_WithQuoteStyleTests tests all quote style combinations.
func TestFormatFieldReference_WithQuoteStyleTests(t *testing.T) {
	tests := []struct {
		name      string
		fieldPath string
		prefix    string
		style     QuoteStyle
		expected  string
	}{
		{
			name:      "NoQuote - simple field",
			fieldPath: "email",
			prefix:    "",
			style:     NoQuote,
			expected:  "email",
		},
		{
			name:      "SingleQuote - simple field",
			fieldPath: "email",
			prefix:    "",
			style:     SingleQuote,
			expected:  "'email'",
		},
		{
			name:      "DoubleQuote - simple field",
			fieldPath: "email",
			prefix:    "",
			style:     DoubleQuote,
			expected:  `"email"`,
		},
		{
			name:      "Backtick - simple field",
			fieldPath: "email",
			prefix:    "",
			style:     Backtick,
			expected:  "`email`",
		},
		{
			name:      "DoubleQuote - nested path",
			fieldPath: "user.email",
			prefix:    "",
			style:     DoubleQuote,
			expected:  `"user"."email"`,
		},
		{
			name:      "SingleQuote - with array index",
			fieldPath: "users.0.email",
			prefix:    "",
			style:     SingleQuote,
			expected:  "'users'[0].'email'",
		},
		{
			name:      "Backtick - with prefix",
			fieldPath: "data.items",
			prefix:    "response",
			style:     Backtick,
			expected:  "response.`data`.`items`",
		},
		{
			name:      "NoQuote - with array index in brackets",
			fieldPath: "users[0].email",
			prefix:    "",
			style:     NoQuote,
			expected:  "users[0].email",
		},
		{
			name:      "DoubleQuote - empty path",
			fieldPath: "",
			prefix:    "",
			style:     DoubleQuote,
			expected:  "(unknown field)",
		},
		{
			name:      "SingleQuote - empty path with prefix",
			fieldPath: "",
			prefix:    "request",
			style:     SingleQuote,
			expected:  "request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatFieldReference(tt.fieldPath, tt.prefix, WithQuoteStyle(tt.style))
			if result != tt.expected {
				t.Errorf("FormatFieldReference(%q, %q, WithQuoteStyle(%v)) = %q, want %q",
					tt.fieldPath, tt.prefix, tt.style, result, tt.expected)
			}
		})
	}
}

// TestFormatFieldReference_QuoteStyleEdgeCases tests edge cases for quote styles.
func TestFormatFieldReference_QuoteStyleEdgeCases(t *testing.T) {
	t.Run("quote style with special characters in field name", func(t *testing.T) {
		result := FormatFieldReference("user-email.address", "", WithQuoteStyle(DoubleQuote))
		expected := `"user-email"."address"`
		if result != expected {
			t.Errorf("With special chars = %q, want %q", result, expected)
		}
	})

	t.Run("quote style with numbers in field name", func(t *testing.T) {
		result := FormatFieldReference("field123.field456", "", WithQuoteStyle(SingleQuote))
		expected := `'field123'.'field456'`
		if result != expected {
			t.Errorf("With numbers = %q, want %q", result, expected)
		}
	})

	t.Run("quote style with unicode field name", func(t *testing.T) {
		result := FormatFieldReference("user.姓名.email", "", WithQuoteStyle(Backtick))
		expected := "`user`.`姓名`.`email`"
		if result != expected {
			t.Errorf("With unicode = %q, want %q", result, expected)
		}
	})

	t.Run("quote style with deeply nested path", func(t *testing.T) {
		result := FormatFieldReference("a.b.c.d.e.f.g", "", WithQuoteStyle(DoubleQuote))
		expected := `"a"."b"."c"."d"."e"."f"."g"`
		if result != expected {
			t.Errorf("Deeply nested = %q, want %q", result, expected)
		}
	})
}

// =============================================================================
// TESTS FOR BASIC ERROR FORMATTING WITH REQUIRED FIELDS ONLY
// =============================================================================

// TestBasicErrorFormatting_RequiredOnly tests formatting with only required fields.
func TestBasicErrorFormatting_RequiredOnly(t *testing.T) {
	tests := []struct {
		name      string
		errorType string
		message   string
		expected  string
	}{
		{
			name:      "minimal error - type and message only",
			errorType: "required",
			message:   "Field is required",
			expected:  "[required] Field is required",
		},
		{
			name:      "format error type",
			errorType: "format",
			message:   "Invalid email format",
			expected:  "[format] Invalid email format",
		},
		{
			name:      "range error type",
			errorType: "range",
			message:   "Value out of range",
			expected:  "[range] Value out of range",
		},
		{
			name:      "type error type",
			errorType: "type",
			message:   "Expected number",
			expected:  "[type] Expected number",
		},
		{
			name:      "validation error type",
			errorType: "validation",
			message:   "Validation failed",
			expected:  "[validation] Validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatErrorString(tt.errorType, tt.message)
			if result != tt.expected {
				t.Errorf("FormatErrorString(%q, %q) = %q, want %q",
					tt.errorType, tt.message, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// TESTS FOR OPTIONAL FIELDS INDIVIDUALLY
// =============================================================================

// TestOptionalFields_Individually tests each optional field in isolation.
func TestOptionalFields_Individually(t *testing.T) {
	t.Run("WithContext only", func(t *testing.T) {
		err := NewValidationFormatter("test_validation").
			WithExpected("expected").
			WithActual("actual").
			WithContext("API endpoint test").
			Format()

		errMsg := err.Error()
		if !strings.Contains(errMsg, "API endpoint test") {
			t.Error("Expected context in error message")
		}
		if !strings.Contains(errMsg, "Context:") {
			t.Error("Expected 'Context:' label")
		}
	})

	t.Run("WithResponseSnippet only", func(t *testing.T) {
		err := NewValidationFormatter("test_validation").
			WithExpected("expected").
			WithActual("actual").
			WithResponseSnippet(`{"result": "actual"}`).
			Format()

		errMsg := err.Error()
		if !strings.Contains(errMsg, `{"result": "actual"}`) {
			t.Error("Expected response snippet in error message")
		}
		if !strings.Contains(errMsg, "Response:") {
			t.Error("Expected 'Response:' label")
		}
	})

	t.Run("WithFieldName only", func(t *testing.T) {
		err := NewValidationFormatter("test_validation").
			WithExpected("expected").
			WithActual("actual").
			WithFieldName("test_field").
			Format()

		errMsg := err.Error()
		if !strings.Contains(errMsg, "test_field") {
			t.Error("Expected field name in error message")
		}
		if !strings.Contains(errMsg, "Field:") {
			t.Error("Expected 'Field:' label")
		}
	})

	t.Run("WithPatternDetails only", func(t *testing.T) {
		err := NewValidationFormatter("test_validation").
			WithExpected("expected").
			WithActual("actual").
			WithPatternDetails("Pattern 'test.*' did not match").
			Format()

		errMsg := err.Error()
		if !strings.Contains(errMsg, "Pattern 'test.*' did not match") {
			t.Error("Expected pattern details in error message")
		}
		if !strings.Contains(errMsg, "Pattern:") {
			t.Error("Expected 'Pattern:' label")
		}
	})

	t.Run("WithRangeInfo only", func(t *testing.T) {
		err := NewValidationFormatter("test_validation").
			WithExpected("expected").
			WithActual("actual").
			WithRangeInfo("200-299 (Success)").
			Format()

		errMsg := err.Error()
		if !strings.Contains(errMsg, "200-299 (Success)") {
			t.Error("Expected range info in error message")
		}
		if !strings.Contains(errMsg, "Range:") {
			t.Error("Expected 'Range:' label")
		}
	})

	t.Run("WithValidationDetails only", func(t *testing.T) {
		err := NewValidationFormatter("test_validation").
			WithExpected("expected").
			WithActual("actual").
			WithValidationDetails("Detail 1", "Detail 2", "Detail 3").
			Format()

		errMsg := err.Error()
		if !strings.Contains(errMsg, "Detail 1") || !strings.Contains(errMsg, "Detail 2") || !strings.Contains(errMsg, "Detail 3") {
			t.Error("Expected all validation details in error message")
		}
		if !strings.Contains(errMsg, "Details:") {
			t.Error("Expected 'Details:' label")
		}
	})

	t.Run("WithSuggestions only", func(t *testing.T) {
		customSuggestions := []string{"Suggestion 1", "Suggestion 2", "Suggestion 3"}
		err := NewValidationFormatter("test_validation").
			WithExpected("expected").
			WithActual("actual").
			WithSuggestions(customSuggestions...).
			Format()

		errMsg := err.Error()
		for _, suggestion := range customSuggestions {
			if !strings.Contains(errMsg, suggestion) {
				t.Errorf("Expected suggestion '%s' in error message", suggestion)
			}
		}
		if !strings.Contains(errMsg, "Suggestions:") {
			t.Error("Expected 'Suggestions:' label")
		}
	})
}

// =============================================================================
// TESTS FOR OPTIONAL FIELDS COMBINATIONS
// =============================================================================

// TestOptionalFields_Combinations tests various combinations of optional fields.
func TestOptionalFields_Combinations(t *testing.T) {
	t.Run("Context + FieldName", func(t *testing.T) {
		err := NewValidationFormatter("test").
			WithExpected("exp").
			WithActual("act").
			WithContext("test context").
			WithFieldName("test_field").
			Format()

		errMsg := err.Error()
		if !strings.Contains(errMsg, "test context") {
			t.Error("Expected context")
		}
		if !strings.Contains(errMsg, "test_field") {
			t.Error("Expected field name")
		}
	})

	t.Run("ResponseSnippet + PatternDetails", func(t *testing.T) {
		err := NewValidationFormatter("test").
			WithExpected("exp").
			WithActual("act").
			WithResponseSnippet(`{"key": "value"}`).
			WithPatternDetails("pattern mismatch").
			Format()

		errMsg := err.Error()
		if !strings.Contains(errMsg, `{"key": "value"}`) {
			t.Error("Expected response snippet")
		}
		if !strings.Contains(errMsg, "pattern mismatch") {
			t.Error("Expected pattern details")
		}
	})

	t.Run("RangeInfo + ValidationDetails", func(t *testing.T) {
		err := NewValidationFormatter("test").
			WithExpected("exp").
			WithActual("act").
			WithRangeInfo("1-100").
			WithValidationDetails("Detail 1", "Detail 2").
			Format()

		errMsg := err.Error()
		if !strings.Contains(errMsg, "1-100") {
			t.Error("Expected range info")
		}
		if !strings.Contains(errMsg, "Detail 1") || !strings.Contains(errMsg, "Detail 2") {
			t.Error("Expected validation details")
		}
	})

	t.Run("Context + ResponseSnippet + FieldName", func(t *testing.T) {
		err := NewValidationFormatter("test").
			WithExpected("exp").
			WithActual("act").
			WithContext("API call").
			WithResponseSnippet(`{"error": "failed"}`).
			WithFieldName("response").
			Format()

		errMsg := err.Error()
		if !strings.Contains(errMsg, "API call") {
			t.Error("Expected context")
		}
		if !strings.Contains(errMsg, `{"error": "failed"}`) {
			t.Error("Expected response snippet")
		}
		if !strings.Contains(errMsg, "response") {
			t.Error("Expected field name")
		}
	})

	t.Run("PatternDetails + RangeInfo + Suggestions", func(t *testing.T) {
		err := NewValidationFormatter("test").
			WithExpected("exp").
			WithActual("act").
			WithPatternDetails("regex error").
			WithRangeInfo("100-500").
			WithSuggestions("Fix pattern", "Check range").
			Format()

		errMsg := err.Error()
		if !strings.Contains(errMsg, "regex error") {
			t.Error("Expected pattern details")
		}
		if !strings.Contains(errMsg, "100-500") {
			t.Error("Expected range info")
		}
		if !strings.Contains(errMsg, "Fix pattern") || !strings.Contains(errMsg, "Check range") {
			t.Error("Expected suggestions")
		}
	})

	t.Run("All optional fields together", func(t *testing.T) {
		err := NewValidationFormatter("comprehensive_test").
			WithExpected("expected_value").
			WithActual("actual_value").
			WithContext("Full context test").
			WithResponseSnippet(`{"data": "test"}`).
			WithFieldName("test.field").
			WithPatternDetails("Pattern: test.*").
			WithRangeInfo("1-100").
			WithValidationDetails("Detail A", "Detail B", "Detail C").
			WithSuggestions("Suggestion 1", "Suggestion 2", "Suggestion 3").
			Format()

		errMsg := err.Error()

		requiredContent := []string{
			"comprehensive_test",
			"expected_value",
			"actual_value",
			"Full context test",
			`{"data": "test"}`,
			"test.field",
			"Pattern: test.*",
			"1-100",
			"Detail A",
			"Detail B",
			"Detail C",
			"Suggestion 1",
			"Suggestion 2",
			"Suggestion 3",
		}

		for _, content := range requiredContent {
			if !strings.Contains(errMsg, content) {
				t.Errorf("Expected error message to contain '%s'", content)
			}
		}

		requiredLabels := []string{
			"Context:",
			"Response:",
			"Field:",
			"Pattern:",
			"Range:",
			"Details:",
			"Suggestions:",
		}

		for _, label := range requiredLabels {
			if !strings.Contains(errMsg, label) {
				t.Errorf("Expected error message to contain label '%s'", label)
			}
		}
	})
}

// =============================================================================
// TESTS FOR EDGE CASES (nil values, empty strings, missing fields)
// =============================================================================

// TestEdgeCases_NilHandling tests handling of nil values.
func TestEdgeCases_NilHandling(t *testing.T) {
	t.Run("nil expected value", func(t *testing.T) {
		err := NewValidationFormatter("test").
			WithExpected(nil).
			WithActual("actual_value").
			Format()

		errMsg := err.Error()
		if !strings.Contains(errMsg, "actual_value") {
			t.Error("Expected actual value in error message even when expected is nil")
		}
	})

	t.Run("nil actual value", func(t *testing.T) {
		err := NewValidationFormatter("test").
			WithExpected("expected_value").
			WithActual(nil).
			Format()

		errMsg := err.Error()
		if !strings.Contains(errMsg, "expected_value") {
			t.Error("Expected expected value in error message even when actual is nil")
		}
	})

	t.Run("both expected and actual nil", func(t *testing.T) {
		err := NewValidationFormatter("test").
			WithExpected(nil).
			WithActual(nil).
			Format()

		errMsg := err.Error()
		if errMsg == "" {
			t.Error("Expected non-empty error message even when both values are nil")
		}
	})

	t.Run("nil in validation details", func(t *testing.T) {
		// This should not panic
		err := NewValidationFormatter("test").
			WithExpected("exp").
			WithActual("act").
			WithValidationDetails("Detail 1", "Detail 2").
			Format()

		if err.Error() == "" {
			t.Error("Expected non-empty error message")
		}
	})
}

// TestEdgeCases_EmptyStringHandling tests handling of empty strings.
func TestEdgeCases_EmptyStringHandling(t *testing.T) {
	t.Run("empty expected string", func(t *testing.T) {
		err := NewValidationFormatter("test").
			WithExpected("").
			WithActual("actual").
			Format()

		errMsg := err.Error()
		if !strings.Contains(errMsg, "actual") {
			t.Error("Expected actual value in error message")
		}
	})

	t.Run("empty actual string", func(t *testing.T) {
		err := NewValidationFormatter("test").
			WithExpected("expected").
			WithActual("").
			Format()

		errMsg := err.Error()
		if !strings.Contains(errMsg, "expected") {
			t.Error("Expected expected value in error message")
		}
	})

	t.Run("empty context string", func(t *testing.T) {
		err := NewValidationFormatter("test").
			WithExpected("exp").
			WithActual("act").
			WithContext("").
			Format()

		errMsg := err.Error()
		// Empty context should not appear in the error message
		if strings.Contains(errMsg, "Context:") {
			t.Error("Empty context should not produce Context: label")
		}
	})

	t.Run("empty response snippet", func(t *testing.T) {
		err := NewValidationFormatter("test").
			WithExpected("exp").
			WithActual("act").
			WithResponseSnippet("").
			Format()

		errMsg := err.Error()
		// Empty snippet should not appear in the error message
		if strings.Contains(errMsg, "Response:") {
			t.Error("Empty response snippet should not produce Response: label")
		}
	})

	t.Run("empty field name", func(t *testing.T) {
		err := NewValidationFormatter("test").
			WithExpected("exp").
			WithActual("act").
			WithFieldName("").
			Format()

		errMsg := err.Error()
		// Empty field name should not appear in the error message
		if strings.Contains(errMsg, "Field:") {
			t.Error("Empty field name should not produce Field: label")
		}
	})

	t.Run("empty pattern details", func(t *testing.T) {
		err := NewValidationFormatter("test").
			WithExpected("exp").
			WithActual("act").
			WithPatternDetails("").
			Format()

		errMsg := err.Error()
		// Empty pattern details should not appear in the error message
		if strings.Contains(errMsg, "Pattern:") {
			t.Error("Empty pattern details should not produce Pattern: label")
		}
	})

	t.Run("empty range info", func(t *testing.T) {
		err := NewValidationFormatter("test").
			WithExpected("exp").
			WithActual("act").
			WithRangeInfo("").
			Format()

		errMsg := err.Error()
		// Empty range info should not appear in the error message
		if strings.Contains(errMsg, "Range:") {
			t.Error("Empty range info should not produce Range: label")
		}
	})

	t.Run("empty suggestions slice", func(t *testing.T) {
		err := NewValidationFormatter("test").
			WithExpected("exp").
			WithActual("act").
			WithSuggestions().
			Format()

		errMsg := err.Error()
		// Should still have auto-generated suggestions
		if !strings.Contains(errMsg, "Suggestions:") {
			t.Error("Should have auto-generated suggestions when custom ones are empty")
		}
	})

	t.Run("empty validation details", func(t *testing.T) {
		err := NewValidationFormatter("test").
			WithExpected("exp").
			WithActual("act").
			WithValidationDetails().
			Format()

		errMsg := err.Error()
		// Empty details should not appear in the error message
		if strings.Contains(errMsg, "Details:") {
			t.Error("Empty validation details should not produce Details: label")
		}
	})
}

// TestEdgeCases_MissingFields tests with all optional fields omitted.
func TestEdgeCases_MissingFields(t *testing.T) {
	t.Run("only required fields (error type)", func(t *testing.T) {
		err := NewValidationFormatter("minimal_test").
			Format()

		errMsg := err.Error()
		if errMsg == "" {
			t.Error("Expected non-empty error message with only error type")
		}
		if !strings.Contains(errMsg, "minimal_test") {
			t.Error("Expected error type in message")
		}
	})

	t.Run("only error type and expected", func(t *testing.T) {
		err := NewValidationFormatter("test").
			WithExpected("expected_only").
			Format()

		errMsg := err.Error()
		if !strings.Contains(errMsg, "expected_only") {
			t.Error("Expected expected value in message")
		}
	})

	t.Run("only error type and actual", func(t *testing.T) {
		err := NewValidationFormatter("test").
			WithActual("actual_only").
			Format()

		errMsg := err.Error()
		if !strings.Contains(errMsg, "actual_only") {
			t.Error("Expected actual value in message")
		}
	})
}

// TestEdgeCases_WhitespaceStrings tests handling of whitespace-only strings.
func TestEdgeCases_WhitespaceStrings(t *testing.T) {
	t.Run("whitespace-only expected", func(t *testing.T) {
		err := NewValidationFormatter("test").
			WithExpected("   \t\n ").
			WithActual("actual").
			Format()

		// Should handle gracefully and produce valid error message
		if err.Error() == "" {
			t.Error("Expected non-empty error message with whitespace expected value")
		}
	})

	t.Run("whitespace-only actual", func(t *testing.T) {
		err := NewValidationFormatter("test").
			WithExpected("expected").
			WithActual("   \t\n ").
			Format()

		// Should handle gracefully and produce valid error message
		if err.Error() == "" {
			t.Error("Expected non-empty error message with whitespace actual value")
		}
	})

	t.Run("whitespace-only context", func(t *testing.T) {
		err := NewValidationFormatter("test").
			WithExpected("exp").
			WithActual("act").
			WithContext("   \t\n ").
			Format()

		errMsg := err.Error()
		// Whitespace-only context should not produce Context: label
		if strings.Contains(errMsg, "Context:") {
			t.Error("Whitespace-only context should not produce Context: label")
		}
	})
}

// =============================================================================
// TESTS FOR ValidationError STRUCT METHODS
// =============================================================================

// TestValidationError_ToMap tests the ToMap method with various field combinations.
func TestValidationError_ToMap(t *testing.T) {
	t.Run("full validation error to map", func(t *testing.T) {
		err := ValidationError{
			ErrorType:         "test_type",
			Message:          "Test message",
			Context:          "test context",
			Expected:         "expected",
			Actual:           "actual",
			FieldName:        "test_field",
			PatternDetails:   "pattern info",
			RangeInfo:        "1-100",
			ValidationDetails: []string{"detail1", "detail2"},
			ResponseSnippet:  `{"test": "data"}`,
			Suggestions:      []string{"suggestion1", "suggestion2"},
		}

		result := err.ToMap()

		// Check all fields are present
		if result["error_type"] != "test_type" {
			t.Error("Expected error_type in map")
		}
		if result["message"] != "Test message" {
			t.Error("Expected message in map")
		}
		if result["context"] != "test context" {
			t.Error("Expected context in map")
		}
		if result["expected"] != "expected" {
			t.Error("Expected expected value in map")
		}
		if result["actual"] != "actual" {
			t.Error("Expected actual value in map")
		}
		if result["field_name"] != "test_field" {
			t.Error("Expected field_name in map")
		}
		if result["pattern_details"] != "pattern info" {
			t.Error("Expected pattern_details in map")
		}
		if result["range_info"] != "1-100" {
			t.Error("Expected range_info in map")
		}
	})

	t.Run("minimal validation error to map", func(t *testing.T) {
		err := ValidationError{
			ErrorType: "test_type",
			Message:   "Test message",
		}

		result := err.ToMap()

		// Check only required fields are present
		if len(result) != 2 {
			t.Errorf("Expected 2 fields in map, got %d", len(result))
		}
		if result["error_type"] != "test_type" {
			t.Error("Expected error_type in map")
		}
		if result["message"] != "Test message" {
			t.Error("Expected message in map")
		}
	})

	t.Run("validation error with empty optional fields to map", func(t *testing.T) {
		err := ValidationError{
			ErrorType:         "test_type",
			Message:          "Test message",
			Context:          "",
			ResponseSnippet:  "",
			FieldName:        "",
			PatternDetails:   "",
			RangeInfo:        "",
		}

		result := err.ToMap()

		// Empty string fields should not be in map
		if len(result) != 2 {
			t.Errorf("Expected 2 fields in map (only required ones), got %d", len(result))
		}
	})
}

// TestValidationError_Validate tests the Validate method.
func TestValidationError_Validate(t *testing.T) {
	t.Run("valid validation error", func(t *testing.T) {
		err := ValidationError{
			ErrorType: "test_type",
			Message:   "Test message",
		}

		if validateErr := err.Validate(); validateErr != nil {
			t.Errorf("Expected valid error to pass validation, got: %v", validateErr)
		}
	})

	t.Run("missing error type", func(t *testing.T) {
		err := ValidationError{
			Message: "Test message",
		}

		if validateErr := err.Validate(); validateErr == nil {
			t.Error("Expected validation error for missing ErrorType")
		}
	})

	t.Run("missing message", func(t *testing.T) {
		err := ValidationError{
			ErrorType: "test_type",
		}

		if validateErr := err.Validate(); validateErr == nil {
			t.Error("Expected validation error for missing Message")
		}
	})

	t.Run("missing both required fields", func(t *testing.T) {
		err := ValidationError{}

		if validateErr := err.Validate(); validateErr == nil {
			t.Error("Expected validation error for missing both required fields")
		}
	})
}

// =============================================================================
// TESTS FOR FormatErrorWithSeverity
// =============================================================================

// TestFormatErrorWithSeverity_AllSeverities tests formatting with all severity levels.
func TestFormatErrorWithSeverity_AllSeverities(t *testing.T) {
	tests := []struct {
		name      string
		errorType ErrorType
		severity  ErrorSeverity
		message   string
		fieldName string
		expected  string
	}{
		{
			name:      "Critical severity",
			errorType: ErrTypeRequired,
			severity:  SeverityCritical,
			message:   "Field is required",
			fieldName: "email",
			expected:  "[🚨 CRIT] [required] email: Field is required",
		},
		{
			name:      "High severity",
			errorType: ErrTypeFormat,
			severity:  SeverityHigh,
			message:   "Invalid format",
			fieldName: "date",
			expected:  "[⚠️ HIGH] [format] date: Invalid format",
		},
		{
			name:      "Medium severity",
			errorType: ErrTypeRange,
			severity:  SeverityMedium,
			message:   "Out of range",
			fieldName: "age",
			expected:  "[⚡ MED] [range] age: Out of range",
		},
		{
			name:      "Low severity",
			errorType: ErrTypeLength,
			severity:  SeverityLow,
			message:   "Too short",
			fieldName: "password",
			expected:  "[ℹ️ LOW] [length] password: Too short",
		},
		{
			name:      "Info severity",
			errorType: ErrTypeValue,
			severity:  SeverityInfo,
			message:   "Value info",
			fieldName: "status",
			expected:  "[💡 INFO] [value] status: Value info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatErrorWithSeverity(tt.errorType, tt.severity, tt.message, tt.fieldName)
			if result != tt.expected {
				t.Errorf("FormatErrorWithSeverity() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestFormatErrorWithSeverity_ComprehensiveEdgeCases tests edge cases for severity formatting.
func TestFormatErrorWithSeverity_ComprehensiveEdgeCases(t *testing.T) {
	t.Run("empty severity defaults", func(t *testing.T) {
		result := FormatErrorWithSeverity(ErrTypeRequired, "", "Field required", "email")
		// Empty severity should still produce valid output
		if result == "" {
			t.Error("Expected non-empty result with empty severity")
		}
	})

	t.Run("empty message with field name", func(t *testing.T) {
		result := FormatErrorWithSeverity(ErrTypeRequired, SeverityHigh, "", "email")
		if !strings.Contains(result, "email validation failed") {
			t.Error("Expected fallback message with empty message and field name")
		}
	})

	t.Run("empty message without field name", func(t *testing.T) {
		result := FormatErrorWithSeverity(ErrTypeFormat, SeverityMedium, "", "")
		if !strings.Contains(result, "(no message provided)") {
			t.Error("Expected fallback message with empty message and no field")
		}
	})
}
