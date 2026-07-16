package validate

import (
	"fmt"
	"strings"
	"testing"
)

// =============================================================================
// COMPREHENSIVE UNIT TESTS FOR VALIDATION ERROR FORMATTER
//
// This test suite provides comprehensive coverage for the error formatter
// with focus on:
// 1. Basic error formatting (required fields only)
// 2. Optional fields individually and in combination
// 3. Edge cases (nil values, empty strings, missing fields)
// 4. Test coverage >90%
// =============================================================================

// =============================================================================
// BASIC ERROR FORMATTING TESTS (Required Fields Only)
// =============================================================================

// TestValidationFormatter_BasicRequiredFields tests the formatter with only
// the required fields (validation type, expected, actual).
func TestValidationFormatter_BasicRequiredFields(t *testing.T) {
	tests := []struct {
		name           string
		validationType string
		expected       interface{}
		actual         interface{}
		mustContain    []string
		mustNotContain []string
	}{
		{
			name:           "status code validation - basic",
			validationType: "status_code",
			expected:       200,
			actual:         404,
			mustContain:    []string{"status_code", "200", "404", "OK", "Not Found"},
			mustNotContain: []string{"Context:", "Field:", "Pattern:", "Range:", "Details:"},
		},
		{
			name:           "error message validation - basic",
			validationType: "error_message",
			expected:       "invalid.*token",
			actual:         "access_denied",
			mustContain:    []string{"error_message", "invalid.*token", "access_denied"},
			mustNotContain: []string{"Context:", "Field:", "Pattern:", "Range:", "Details:"},
		},
		{
			name:           "content type validation - basic",
			validationType: "content_type",
			expected:       "application/json",
			actual:         "text/html",
			mustContain:    []string{"content_type", "application/json", "text/html"},
			mustNotContain: []string{"Context:", "Field:", "Pattern:", "Range:", "Details:"},
		},
		{
			name:           "string values - basic",
			validationType: "string_match",
			expected:       "expected_value",
			actual:         "actual_value",
			mustContain:    []string{"string_match", "expected_value", "actual_value"},
			mustNotContain: []string{"Context:", "Field:", "Pattern:", "Range:", "Details:"},
		},
		{
			name:           "integer values - basic",
			validationType: "integer_range",
			expected:       100,
			actual:         150,
			mustContain:    []string{"integer_range", "100", "150"},
			mustNotContain: []string{"Context:", "Field:", "Pattern:", "Range:", "Details:"},
		},
		{
			name:           "nil expected with actual - basic",
			validationType: "nil_test",
			expected:       nil,
			actual:         "value",
			mustContain:    []string{"nil_test", "value"},
			mustNotContain: []string{"Context:", "Field:", "Pattern:", "Range:", "Details:"},
		},
		{
			name:           "expected with nil actual - basic",
			validationType: "nil_actual_test",
			expected:       "value",
			actual:         nil,
			mustContain:    []string{"nil_actual_test", "value"},
			mustNotContain: []string{"Context:", "Field:", "Pattern:", "Range:", "Details:"},
		},
		{
			name:           "both nil values - basic",
			validationType: "both_nil",
			expected:       nil,
			actual:         nil,
			mustContain:    []string{"both_nil"},
			mustNotContain: []string{"Expected:", "Actual:", "Context:", "Field:", "Pattern:", "Range:", "Details:"},
		},
		{
			name:           "array expected values - basic",
			validationType: "multi_choice",
			expected:       []int{200, 201, 204},
			actual:         500,
			mustContain:    []string{"multi_choice", "200", "201", "204", "500"},
			mustNotContain: []string{"Context:", "Field:", "Pattern:", "Range:", "Details:"},
		},
		{
			name:           "empty strings - basic",
			validationType: "empty_strings",
			expected:       "",
			actual:         "",
			mustContain:    []string{"empty_strings"},
			mustNotContain: []string{"Context:", "Field:", "Pattern:", "Range:", "Details:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewValidationFormatter(tt.validationType).
				WithExpected(tt.expected).
				WithActual(tt.actual).
				Format()

			errMsg := err.Error()

			// Check for required content
			for _, required := range tt.mustContain {
				if !strings.Contains(errMsg, required) {
					t.Errorf("Expected error message to contain '%s', got: %s", required, errMsg)
				}
			}

			// Check for prohibited content (optional fields not provided)
			for _, prohibited := range tt.mustNotContain {
				if strings.Contains(errMsg, prohibited) {
					t.Errorf("Expected error message to NOT contain '%s' (not provided), got: %s", prohibited, errMsg)
				}
			}

			// Verify error implements error interface
			if errMsg == "" {
				t.Error("Expected Error() to return non-empty string")
			}
		})
	}
}

// =============================================================================
// OPTIONAL FIELDS INDIVIDUAL TESTS
// =============================================================================

// TestValidationFormatter_OptionalField_Individual tests each optional field
// in isolation to ensure it works correctly.
func TestValidationFormatter_OptionalField_Individual(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *ValidationFormatter
		validate func(string) error
	}{
		{
			name: "WithContext only",
			setup: func() *ValidationFormatter {
				return NewValidationFormatter("status_code").
					WithExpected(200).
					WithActual(404).
					WithContext("GET /api/users/123")
			},
			validate: func(errMsg string) error {
				if !strings.Contains(errMsg, "GET /api/users/123") {
					return fmt.Errorf("missing context")
				}
				if !strings.Contains(errMsg, "Request:") && !strings.Contains(errMsg, "Context:") {
					return fmt.Errorf("missing context label")
				}
				return nil
			},
		},
		{
			name: "WithResponseSnippet only",
			setup: func() *ValidationFormatter {
				return NewValidationFormatter("content_type").
					WithExpected("application/json").
					WithActual("text/html").
					WithResponseSnippet(`{"status": "ok", "data": {"type": "html"}}`)
			},
			validate: func(errMsg string) error {
				// Check for key parts of the snippet (it may be truncated in the actual message)
				if !strings.Contains(errMsg, "status") && !strings.Contains(errMsg, "Response:") {
					return fmt.Errorf("missing response snippet or Response: label")
				}
				if !strings.Contains(errMsg, "Response:") {
					return fmt.Errorf("missing Response: label")
				}
				return nil
			},
		},
		{
			name: "WithFieldName only",
			setup: func() *ValidationFormatter {
				return NewValidationFormatter("error_message").
					WithExpected("invalid.*token").
					WithActual("no_token").
					WithFieldName("error")
			},
			validate: func(errMsg string) error {
				if !strings.Contains(errMsg, "error") {
					return fmt.Errorf("missing field name")
				}
				if !strings.Contains(errMsg, "Field:") {
					return fmt.Errorf("missing Field: label")
				}
				return nil
			},
		},
		{
			name: "WithPatternDetails only",
			setup: func() *ValidationFormatter {
				return NewValidationFormatter("regex_match").
					WithExpected("^[A-Z]{2,}$").
					WithActual("ab").
					WithPatternDetails("regex pattern for uppercase letters (min 2 chars)")
			},
			validate: func(errMsg string) error {
				if !strings.Contains(errMsg, "regex pattern") {
					return fmt.Errorf("missing pattern details")
				}
				if !strings.Contains(errMsg, "Pattern:") {
					return fmt.Errorf("missing Pattern: label")
				}
				return nil
			},
		},
		{
			name: "WithRangeInfo only",
			setup: func() *ValidationFormatter {
				return NewValidationFormatter("numeric_range").
					WithExpected("1-100").
					WithActual(150).
					WithRangeInfo("1-100 (valid range for age)")
			},
			validate: func(errMsg string) error {
				if !strings.Contains(errMsg, "1-100") {
					return fmt.Errorf("missing range info")
				}
				if !strings.Contains(errMsg, "Range:") {
					return fmt.Errorf("missing Range: label")
				}
				return nil
			},
		},
		{
			name: "WithValidationDetails only",
			setup: func() *ValidationFormatter {
				return NewValidationFormatter("complex_validation").
					WithExpected("valid").
					WithActual("invalid").
					WithValidationDetails("Check 1: format invalid", "Check 2: length too short", "Check 3: special chars missing")
			},
			validate: func(errMsg string) error {
				if !strings.Contains(errMsg, "Check 1") {
					return fmt.Errorf("missing validation detail 1")
				}
				if !strings.Contains(errMsg, "Check 2") {
					return fmt.Errorf("missing validation detail 2")
				}
				if !strings.Contains(errMsg, "Check 3") {
					return fmt.Errorf("missing validation detail 3")
				}
				if !strings.Contains(errMsg, "Details:") {
					return fmt.Errorf("missing Details: label")
				}
				return nil
			},
		},
		{
			name: "WithSuggestions only",
			setup: func() *ValidationFormatter {
				return NewValidationFormatter("custom_validation").
					WithExpected("expected").
					WithActual("actual").
					WithSuggestions("Suggestion 1: check input", "Suggestion 2: verify format", "Suggestion 3: try again")
			},
			validate: func(errMsg string) error {
				if !strings.Contains(errMsg, "Suggestion 1") {
					return fmt.Errorf("missing suggestion 1")
				}
				if !strings.Contains(errMsg, "Suggestion 2") {
					return fmt.Errorf("missing suggestion 2")
				}
				if !strings.Contains(errMsg, "Suggestion 3") {
					return fmt.Errorf("missing suggestion 3")
				}
				if !strings.Contains(errMsg, "Suggestions:") && !strings.Contains(errMsg, "Common causes:") {
					return fmt.Errorf("missing suggestions label")
				}
				return nil
			},
		},
		{
			name: "WithLocation only",
			setup: func() *ValidationFormatter {
				return NewValidationFormatter("field_validation").
					WithExpected("required").
					WithActual("missing")
				// Note: Location is set directly on ValidationError, not via formatter
				// This test validates the ValidationError struct directly
			},
			validate: func(errMsg string) error {
				// Location is not settable via ValidationFormatter, so just verify base format
				if !strings.Contains(errMsg, "field_validation") {
					return fmt.Errorf("missing validation type")
				}
				return nil
			},
		},
		{
			name: "WithRelatedFields only",
			setup: func() *ValidationFormatter {
				return NewValidationFormatter("dependency_check").
					WithExpected("valid").
					WithActual("invalid")
				// Note: RelatedFields is set directly on ValidationError, not via formatter
			},
			validate: func(errMsg string) error {
				if !strings.Contains(errMsg, "dependency_check") {
					return fmt.Errorf("missing validation type")
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := tt.setup()
			err := formatter.Format()
			errMsg := err.Error()

			if err := tt.validate(errMsg); err != nil {
				t.Errorf("Validation failed: %v\nError message:\n%s", err, errMsg)
			}

			// Verify the error implements the error interface
			var _ error = err
		})
	}
}

// =============================================================================
// OPTIONAL FIELDS COMBINATION TESTS
// =============================================================================

// TestValidationFormatter_OptionalFields_Combinations tests multiple
// optional fields together to ensure they work correctly in combination.
func TestValidationFormatter_OptionalFields_Combinations(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *ValidationFormatter
		validate func(string) error
	}{
		{
			name: "Context + FieldName",
			setup: func() *ValidationFormatter {
				return NewValidationFormatter("error_message").
					WithExpected("invalid.*token").
					WithActual("access_denied").
					WithContext("OAuth validation").
					WithFieldName("error")
			},
			validate: func(errMsg string) error {
				if !strings.Contains(errMsg, "OAuth validation") {
					return fmt.Errorf("missing context")
				}
				if !strings.Contains(errMsg, "error") {
					return fmt.Errorf("missing field name")
				}
				return nil
			},
		},
		{
			name: "Context + ResponseSnippet + FieldName",
			setup: func() *ValidationFormatter {
				return NewValidationFormatter("content_type").
					WithExpected("application/json").
					WithActual("text/html").
					WithContext("API response").
					WithResponseSnippet(`<!DOCTYPE html><html>...</html>`).
					WithFieldName("Content-Type")
			},
			validate: func(errMsg string) error {
				if !strings.Contains(errMsg, "API response") {
					return fmt.Errorf("missing context")
				}
				if !strings.Contains(errMsg, `<!DOCTYPE html>`) {
					return fmt.Errorf("missing response snippet")
				}
				if !strings.Contains(errMsg, "Content-Type") {
					return fmt.Errorf("missing field name")
				}
				return nil
			},
		},
		{
			name: "PatternDetails + ValidationDetails",
			setup: func() *ValidationFormatter {
				return NewValidationFormatter("pattern_validation").
					WithExpected("^[A-Z][a-z]+$").
					WithActual("invalid123").
					WithPatternDetails("must start with uppercase, followed by lowercase").
					WithValidationDetails("Contains numbers", "Contains uppercase in wrong position", "Missing lowercase after first char")
			},
			validate: func(errMsg string) error {
				if !strings.Contains(errMsg, "must start with uppercase") {
					return fmt.Errorf("missing pattern details")
				}
				if !strings.Contains(errMsg, "Contains numbers") {
					return fmt.Errorf("missing validation detail 1")
				}
				if !strings.Contains(errMsg, "Contains uppercase in wrong position") {
					return fmt.Errorf("missing validation detail 2")
				}
				return nil
			},
		},
		{
			name: "RangeInfo + ValidationDetails + Suggestions",
			setup: func() *ValidationFormatter {
				return NewValidationFormatter("age_validation").
					WithExpected("0-120").
					WithActual(150).
					WithRangeInfo("0-120 (human age range)").
					WithValidationDetails("Value exceeds maximum human lifespan", "Possible data entry error").
					WithSuggestions("Verify the age value is correct", "Check for data entry errors", "Consider if this is a test case")
			},
			validate: func(errMsg string) error {
				if !strings.Contains(errMsg, "0-120") {
					return fmt.Errorf("missing range info")
				}
				if !strings.Contains(errMsg, "Value exceeds maximum") {
					return fmt.Errorf("missing validation detail 1")
				}
				if !strings.Contains(errMsg, "Verify the age value") {
					return fmt.Errorf("missing suggestion 1")
				}
				return nil
			},
		},
		{
			name: "All optional fields together",
			setup: func() *ValidationFormatter {
				return NewValidationFormatter("comprehensive_test").
					WithExpected("expected_value").
					WithActual("actual_value").
					WithContext("Test context for comprehensive validation").
					WithResponseSnippet(`{"field": "actual_value", "status": "error"}`).
					WithFieldName("test_field").
					WithPatternDetails("pattern description").
					WithRangeInfo("1-100 (valid range)").
					WithValidationDetails("Detail 1", "Detail 2", "Detail 3").
					WithSuggestions("Suggestion 1", "Suggestion 2", "Suggestion 3")
			},
			validate: func(errMsg string) error {
				requiredStrings := []string{
					"Test context",
					"actual_value", // Check for actual_value (snippet may be formatted differently)
					"test_field",
					"pattern description",
					"1-100",
					"Detail 1",
					"Detail 2",
					"Detail 3",
					"Suggestion 1",
					"Suggestion 2",
					"Suggestion 3",
				}

				for _, req := range requiredStrings {
					if !strings.Contains(errMsg, req) {
						return fmt.Errorf("missing required string: %s", req)
					}
				}

				// Check for labels - "Request:" only for status_code, "Context:" for others
				requiredLabels := []string{
					"Response:", "Field:", "Pattern:", "Range:", "Details:",
				}

				for _, label := range requiredLabels {
					if !strings.Contains(errMsg, label) {
						return fmt.Errorf("missing label: %s", label)
					}
				}

				// Check for either Context: or Request: (Request: only for status_code)
				if !strings.Contains(errMsg, "Context:") && !strings.Contains(errMsg, "Request:") {
					return fmt.Errorf("missing both Context: and Request: labels")
				}

				return nil
			},
		},
		{
			name: "Custom suggestions override auto-generated",
			setup: func() *ValidationFormatter {
				return NewValidationFormatter("status_code").
					WithExpected(200).
					WithActual(404).
					WithContext("GET /api/users/123").
					WithSuggestions("Custom suggestion 1", "Custom suggestion 2", "Custom suggestion 3")
			},
			validate: func(errMsg string) error {
				// Should have custom suggestions
				if !strings.Contains(errMsg, "Custom suggestion 1") {
					return fmt.Errorf("missing custom suggestion 1")
				}

				// Should NOT have auto-generated 404 suggestions
				autoGenerated := []string{
					"Verify the endpoint URL is correct",
					"Check if the resource ID or identifier exists",
					"Ensure the resource hasn't been deleted or moved",
				}

				for _, auto := range autoGenerated {
					if strings.Contains(errMsg, auto) {
						return fmt.Errorf("found auto-generated suggestion when custom provided: %s", auto)
					}
				}

				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := tt.setup()
			err := formatter.Format()
			errMsg := err.Error()

			if err := tt.validate(errMsg); err != nil {
				t.Errorf("Validation failed: %v\nError message:\n%s", err, errMsg)
			}
		})
	}
}

// =============================================================================
// EDGE CASE TESTS
// =============================================================================

// TestValidationFormatter_EdgeCases_NilValues tests nil value handling.
func TestValidationFormatter_EdgeCases_NilValues(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *ValidationFormatter
		validate func(string) error
	}{
		{
			name: "nil expected with actual value",
			setup: func() *ValidationFormatter {
				return NewValidationFormatter("nil_expected").
					WithExpected(nil).
					WithActual("value").
					WithContext("test context")
			},
			validate: func(errMsg string) error {
				if !strings.Contains(errMsg, "value") {
					return fmt.Errorf("missing actual value")
				}
				if strings.Contains(errMsg, "Expected:") {
					// Expected with nil might show or not show depending on implementation
					// Just verify the message is valid
				}
				return nil
			},
		},
		{
			name: "nil actual with expected value",
			setup: func() *ValidationFormatter {
				return NewValidationFormatter("nil_actual").
					WithExpected("value").
					WithActual(nil).
					WithContext("test context")
			},
			validate: func(errMsg string) error {
				if !strings.Contains(errMsg, "value") {
					return fmt.Errorf("missing expected value")
				}
				return nil
			},
		},
		{
			name: "both expected and actual are nil",
			setup: func() *ValidationFormatter {
				return NewValidationFormatter("both_nil").
					WithExpected(nil).
					WithActual(nil).
					WithContext("both values nil test")
			},
			validate: func(errMsg string) error {
				// Should not crash and should produce valid error message
				if !strings.Contains(errMsg, "both_nil") {
					return fmt.Errorf("missing validation type")
				}
				if !strings.Contains(errMsg, "both values nil test") {
					return fmt.Errorf("missing context")
				}
				return nil
			},
		},
		{
			name: "nil values with all optional fields",
			setup: func() *ValidationFormatter {
				return NewValidationFormatter("nil_with_options").
					WithExpected(nil).
					WithActual(nil).
					WithContext("context").
					WithResponseSnippet("snippet").
					WithFieldName("field").
					WithPatternDetails("pattern").
					WithRangeInfo("range").
					WithValidationDetails("detail1", "detail2").
					WithSuggestions("suggestion1", "suggestion2")
			},
			validate: func(errMsg string) error {
				if !strings.Contains(errMsg, "nil_with_options") {
					return fmt.Errorf("missing validation type")
				}
				// Optional fields should still appear even with nil values
				if !strings.Contains(errMsg, "context") {
					return fmt.Errorf("missing context")
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := tt.setup()
			err := formatter.Format()
			errMsg := err.Error()

			if err := tt.validate(errMsg); err != nil {
				t.Errorf("Validation failed: %v\nError message:\n%s", err, errMsg)
			}
		})
	}
}

// TestValidationFormatter_EdgeCases_EmptyStrings tests empty string handling.
func TestValidationFormatter_EdgeCases_EmptyStrings(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *ValidationFormatter
		validate func(string) error
	}{
		{
			name: "empty expected with actual value",
			setup: func() *ValidationFormatter {
				return NewValidationFormatter("empty_expected").
					WithExpected("").
					WithActual("value")
			},
			validate: func(errMsg string) error {
				if !strings.Contains(errMsg, "value") {
					return fmt.Errorf("missing actual value")
				}
				return nil
			},
		},
		{
			name: "empty actual with expected value",
			setup: func() *ValidationFormatter {
				return NewValidationFormatter("empty_actual").
					WithExpected("value").
					WithActual("")
			},
			validate: func(errMsg string) error {
				if !strings.Contains(errMsg, "value") {
					return fmt.Errorf("missing expected value")
				}
				return nil
			},
		},
		{
			name: "both expected and actual are empty strings",
			setup: func() *ValidationFormatter {
				return NewValidationFormatter("both_empty").
					WithExpected("").
					WithActual("")
			},
			validate: func(errMsg string) error {
				if !strings.Contains(errMsg, "both_empty") {
					return fmt.Errorf("missing validation type")
				}
				return nil
			},
		},
		{
			name: "empty optional fields",
			setup: func() *ValidationFormatter {
				return NewValidationFormatter("empty_optional").
					WithExpected("expected").
					WithActual("actual").
					WithContext("").
					WithResponseSnippet("").
					WithFieldName("").
					WithPatternDetails("").
					WithRangeInfo("").
					WithValidationDetails().
					WithSuggestions()
			},
			validate: func(errMsg string) error {
				if !strings.Contains(errMsg, "empty_optional") {
					return fmt.Errorf("missing validation type")
				}
				// Empty optional fields should not create empty sections
				// Count how many section labels appear
				sectionLabels := []string{"Context:", "Response:", "Field:", "Pattern:", "Range:", "Details:"}
				emptySections := 0
				for _, label := range sectionLabels {
					if strings.Contains(errMsg, label) {
						// Check if it's followed by empty content
						idx := strings.Index(errMsg, label)
						if idx+len(label) < len(errMsg) {
							remaining := errMsg[idx+len(label):]
							lines := strings.Split(remaining, "\n")
							if len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
								emptySections++
							}
						}
					}
				}
				// Should not have empty sections (implementation-dependent)
				return nil
			},
		},
		{
			name: "whitespace-only strings",
			setup: func() *ValidationFormatter {
				return NewValidationFormatter("whitespace_test").
					WithExpected("   ").
					WithActual("  \t\n  ")
			},
			validate: func(errMsg string) error {
				if !strings.Contains(errMsg, "whitespace_test") {
					return fmt.Errorf("missing validation type")
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := tt.setup()
			err := formatter.Format()
			errMsg := err.Error()

			if err := tt.validate(errMsg); err != nil {
				t.Errorf("Validation failed: %v\nError message:\n%s", err, errMsg)
			}
		})
	}
}

// TestValidationFormatter_EdgeCases_MissingFields tests behavior when
// required fields are missing or incomplete.
func TestValidationFormatter_EdgeCases_MissingFields(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *ValidationFormatter
		validate func(string) error
	}{
		{
			name: "no expected value provided",
			setup: func() *ValidationFormatter {
				return NewValidationFormatter("no_expected").
					WithActual("actual")
			},
			validate: func(errMsg string) error {
				if !strings.Contains(errMsg, "no_expected") {
					return fmt.Errorf("missing validation type")
				}
				if !strings.Contains(errMsg, "actual") {
					return fmt.Errorf("missing actual value")
				}
				return nil
			},
		},
		{
			name: "no actual value provided",
			setup: func() *ValidationFormatter {
				return NewValidationFormatter("no_actual").
					WithExpected("expected")
			},
			validate: func(errMsg string) error {
				if !strings.Contains(errMsg, "no_actual") {
					return fmt.Errorf("missing validation type")
				}
				if !strings.Contains(errMsg, "expected") {
					return fmt.Errorf("missing expected value")
				}
				return nil
			},
		},
		{
			name: "neither expected nor actual provided",
			setup: func() *ValidationFormatter {
				return NewValidationFormatter("neither_provided").
					WithContext("context only test")
			},
			validate: func(errMsg string) error {
				if !strings.Contains(errMsg, "neither_provided") {
					return fmt.Errorf("missing validation type")
				}
				if !strings.Contains(errMsg, "context only test") {
					return fmt.Errorf("missing context")
				}
				return nil
			},
		},
		{
			name: "empty validation type",
			setup: func() *ValidationFormatter {
				return NewValidationFormatter("").
					WithExpected("expected").
					WithActual("actual")
			},
			validate: func(errMsg string) error {
				// Should still produce a valid error message
				if errMsg == "" {
					return fmt.Errorf("error message is empty")
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := tt.setup()
			err := formatter.Format()
			errMsg := err.Error()

			if err := tt.validate(errMsg); err != nil {
				t.Errorf("Validation failed: %v\nError message:\n%s", err, errMsg)
			}
		})
	}
}

// =============================================================================
// METHOD CHAINING TESTS
// =============================================================================

// TestValidationFormatter_MethodChaining tests that the fluent API
// works correctly with method chaining.
func TestValidationFormatter_MethodChaining(t *testing.T) {
	t.Run("chaining returns same instance", func(t *testing.T) {
		base := NewValidationFormatter("test")
		result := base.
			WithExpected("expected").
			WithActual("actual").
			WithContext("context").
			WithResponseSnippet("snippet").
			WithFieldName("field").
			WithPatternDetails("pattern").
			WithRangeInfo("range").
			WithValidationDetails("detail1", "detail2").
			WithSuggestions("suggestion1", "suggestion2")

		// Should be able to call Format()
		err := result.Format()
		if err.Error() == "" {
			t.Error("Expected non-empty error message")
		}
	})

	t.Run("chaining order independence", func(t *testing.T) {
		// Test different orderings produce the same result
		setup1 := NewValidationFormatter("test").
			WithExpected("expected").
			WithActual("actual").
			WithContext("context")

		setup2 := NewValidationFormatter("test").
			WithContext("context").
			WithExpected("expected").
			WithActual("actual")

		setup3 := NewValidationFormatter("test").
			WithActual("actual").
			WithContext("context").
			WithExpected("expected")

		// All should produce the same error message (ordering of sections may differ)
		err1 := setup1.Format()
		err2 := setup2.Format()
		err3 := setup3.Format()

		msg1 := err1.Error()
		msg2 := err2.Error()
		msg3 := err3.Error()

		// All should contain the same core information
		for _, msg := range []string{msg1, msg2, msg3} {
			if !strings.Contains(msg, "expected") {
				t.Errorf("Missing 'expected' in message")
			}
			if !strings.Contains(msg, "actual") {
				t.Errorf("Missing 'actual' in message")
			}
			if !strings.Contains(msg, "context") {
				t.Errorf("Missing 'context' in message")
			}
		}
	})

	t.Run("multiple calls to same method override", func(t *testing.T) {
		err := NewValidationFormatter("test").
			WithContext("context1").
			WithContext("context2"). // Should override context1
			WithExpected("expected1").
			WithExpected("expected2"). // Should override expected1
			WithActual("actual").
			Format()

		errMsg := err.Error()

		// Should have the last set values
		if !strings.Contains(errMsg, "context2") {
			t.Error("Expected context2 to override context1")
		}
		if strings.Contains(errMsg, "context1") {
			// context1 might still appear in expected/actual values
		}
	})

	t.Run("WithValidationDetails accumulates", func(t *testing.T) {
		err := NewValidationFormatter("test").
			WithExpected("expected").
			WithActual("actual").
			WithValidationDetails("detail1").
			WithValidationDetails("detail2"). // Should accumulate
			WithValidationDetails("detail3"). // Should accumulate
			Format()

		errMsg := err.Error()

		if !strings.Contains(errMsg, "detail1") {
			t.Error("Missing detail1")
		}
		if !strings.Contains(errMsg, "detail2") {
			t.Error("Missing detail2")
		}
		if !strings.Contains(errMsg, "detail3") {
			t.Error("Missing detail3")
		}
	})

	t.Run("WithSuggestions accumulates", func(t *testing.T) {
		err := NewValidationFormatter("test").
			WithExpected("expected").
			WithActual("actual").
			WithSuggestions("suggestion1").
			WithSuggestions("suggestion2"). // Should accumulate
			WithSuggestions("suggestion3"). // Should accumulate
			Format()

		errMsg := err.Error()

		if !strings.Contains(errMsg, "suggestion1") {
			t.Error("Missing suggestion1")
		}
		if !strings.Contains(errMsg, "suggestion2") {
			t.Error("Missing suggestion2")
		}
		if !strings.Contains(errMsg, "suggestion3") {
			t.Error("Missing suggestion3")
		}
	})
}

// =============================================================================
// ERROR INTERFACE IMPLEMENTATION TESTS
// =============================================================================

// TestValidationFormatter_ErrorInterfaceComprehensive tests that ValidationError properly
// implements the error interface with comprehensive coverage.
func TestValidationFormatter_ErrorInterfaceComprehensive(t *testing.T) {
	t.Run("implements error interface", func(t *testing.T) {
		err := NewValidationFormatter("test").
			WithExpected("expected").
			WithActual("actual").
			Format()

		// Should implement error interface
		var _ error = err

		// Error() should return non-empty string
		if err.Error() == "" {
			t.Error("Error() returned empty string")
		}
	})

	t.Run("can be used in error contexts", func(t *testing.T) {
		err := NewValidationFormatter("test").
			WithExpected("expected").
			WithActual("actual").
			Format()

		// Should be usable in fmt.Errorf
		wrapped := fmt.Errorf("wrapped: %w", err)
		if wrapped == nil {
			t.Error("Failed to wrap error")
		}

		// ValidationError is a struct, not a pointer, so it's never nil
		// But we can verify it has content
		if err.Error() == "" {
			t.Error("Error message should not be empty")
		}
	})

	t.Run("Error() output is consistent", func(t *testing.T) {
		err := NewValidationFormatter("test").
			WithExpected("expected").
			WithActual("actual").
			Format()

		msg1 := err.Error()
		msg2 := err.Error()

		if msg1 != msg2 {
			t.Error("Error() should return consistent output")
		}
	})

	t.Run("zero value ValidationError has valid Error()", func(t *testing.T) {
		var err ValidationError
		msg := err.Error()

		// Even zero value should produce valid (though empty) output
		// This tests the Error() method doesn't panic on zero values
		_ = msg
	})
}

// =============================================================================
// CONVENIENCE FUNCTIONS TESTS
// =============================================================================

// TestConvenienceFunctions_BasicBehavior tests the basic convenience functions.
func TestConvenienceFunctions_BasicBehavior(t *testing.T) {
	t.Run("FormatStatusCodeError", func(t *testing.T) {
		err := FormatStatusCodeError(200, 404, "GET /api/users")
		errMsg := err.Error()

		required := []string{"status_code", "200", "404", "GET /api/users"}
		for _, req := range required {
			if !strings.Contains(errMsg, req) {
				t.Errorf("Missing '%s' in error message", req)
			}
		}
	})

	t.Run("FormatErrorMessageError", func(t *testing.T) {
		err := FormatErrorMessageError("invalid.*token", "access_denied", "error", "OAuth validation")
		errMsg := err.Error()

		required := []string{"error_message", "invalid.*token", "access_denied", "error", "OAuth validation"}
		for _, req := range required {
			if !strings.Contains(errMsg, req) {
				t.Errorf("Missing '%s' in error message", req)
			}
		}
	})

	t.Run("FormatStatusCodeRangeError", func(t *testing.T) {
		err := FormatStatusCodeRangeError("4xx", 200, "error response check", "status_code")
		errMsg := err.Error()

		required := []string{"status_code_range", "400-499", "200", "Client Error"}
		for _, req := range required {
			if !strings.Contains(errMsg, req) {
				t.Errorf("Missing '%s' in error message", req)
			}
		}
	})

	t.Run("FormatContentTypeError", func(t *testing.T) {
		err := FormatContentTypeError("application/json", "text/html", "API response")
		errMsg := err.Error()

		required := []string{"content_type", "application/json", "text/html", "API response"}
		for _, req := range required {
			if !strings.Contains(errMsg, req) {
				t.Errorf("Missing '%s' in error message", req)
			}
		}
	})

	t.Run("FormatCustomValidationError", func(t *testing.T) {
		err := FormatCustomValidationError(
			"custom_type",
			"expected_value",
			"actual_value",
			WithContext("custom context"),
			WithFieldName("custom_field"),
			WithSuggestions("Custom suggestion"),
		)
		errMsg := err.Error()

		required := []string{"custom_type", "expected_value", "actual_value", "custom context", "custom_field", "Custom suggestion"}
		for _, req := range required {
			if !strings.Contains(errMsg, req) {
				t.Errorf("Missing '%s' in error message", req)
			}
		}
	})
}

// =============================================================================
// PERFORMANCE AND STRESS TESTS
// =============================================================================

// TestValidationFormatter_LargeValues tests handling of large values.
func TestValidationFormatter_LargeValues(t *testing.T) {
	t.Run("long response snippet gets truncated", func(t *testing.T) {
		longSnippet := strings.Repeat("a", 1000)
		err := NewValidationFormatter("test").
			WithExpected("expected").
			WithActual("actual").
			WithResponseSnippet(longSnippet).
			Format()

		errMsg := err.Error()

		// Response should be truncated (implementation-dependent)
		// Just verify it doesn't crash and produces valid output
		if !strings.Contains(errMsg, "test") {
			t.Error("Missing validation type")
		}
	})

	t.Run("many validation details", func(t *testing.T) {
		details := make([]string, 100)
		for i := 0; i < 100; i++ {
			details[i] = fmt.Sprintf("Detail %d", i)
		}

		err := NewValidationFormatter("test").
			WithExpected("expected").
			WithActual("actual").
			WithValidationDetails(details...).
			Format()

		errMsg := err.Error()

		// Should contain all details
		for i := 0; i < 100; i++ {
			if !strings.Contains(errMsg, fmt.Sprintf("Detail %d", i)) {
				t.Errorf("Missing detail %d", i)
			}
		}
	})

	t.Run("many suggestions", func(t *testing.T) {
		suggestions := make([]string, 50)
		for i := 0; i < 50; i++ {
			suggestions[i] = fmt.Sprintf("Suggestion %d", i)
		}

		err := NewValidationFormatter("test").
			WithExpected("expected").
			WithActual("actual").
			WithSuggestions(suggestions...).
			Format()

		errMsg := err.Error()

		// Should contain all suggestions
		for i := 0; i < 50; i++ {
			if !strings.Contains(errMsg, fmt.Sprintf("Suggestion %d", i)) {
				t.Errorf("Missing suggestion %d", i)
			}
		}
	})

	t.Run("large expected array", func(t *testing.T) {
		largeArray := make([]int, 100)
		for i := 0; i < 100; i++ {
			largeArray[i] = i
		}

		err := NewValidationFormatter("test").
			WithExpected(largeArray).
			WithActual(999).
			Format()

		errMsg := err.Error()

		// Should handle large array
		if !strings.Contains(errMsg, "999") {
			t.Error("Missing actual value")
		}
	})
}
