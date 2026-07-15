package validate

import (
	"strings"
	"testing"
)

// TestFormatValidationErrorHelper_BasicUsage tests the basic functionality
// of the FormatValidationErrorHelper with minimal parameters.
func TestFormatValidationErrorHelper_BasicUsage(t *testing.T) {
	tests := []struct {
		name      string
		errorType string
		expected  interface{}
		actual    interface{}
		wantType  string
	}{
		{
			name:      "status code validation",
			errorType: "status_code",
			expected:  200,
			actual:    404,
			wantType:  "status_code",
		},
		{
			name:      "error message validation",
			errorType: "error_message",
			expected:  "invalid.*token",
			actual:    "access_denied",
			wantType:  "error_message",
		},
		{
			name:      "content type validation",
			errorType: "content_type",
			expected:  "application/json",
			actual:    "text/html",
			wantType:  "content_type",
		},
		{
			name:      "custom validation type",
			errorType: "custom_validation",
			expected:  "required",
			actual:    "missing",
			wantType:  "custom_validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := FormatValidationErrorHelper(tt.errorType, tt.expected, tt.actual)

			// Verify error type is set correctly
			if err.ErrorType != tt.wantType {
				t.Errorf("ErrorType = %v, want %v", err.ErrorType, tt.wantType)
			}

			// Verify expected value is set correctly
			if err.Expected != tt.expected {
				t.Errorf("Expected = %v, want %v", err.Expected, tt.expected)
			}

			// Verify actual value is set correctly
			if err.Actual != tt.actual {
				t.Errorf("Actual = %v, want %v", err.Actual, tt.actual)
			}

			// Verify error interface is implemented
			errorMsg := err.Error()
			if errorMsg == "" {
				t.Error("Error() returned empty string")
			}

			// Verify error type appears in error message
			if !strings.Contains(errorMsg, tt.errorType) {
				t.Errorf("Error message does not contain error type %q: %s", tt.errorType, errorMsg)
			}
		})
	}
}

// TestFormatValidationErrorHelper_WithContext tests the helper with context option.
func TestFormatValidationErrorHelper_WithContext(t *testing.T) {
	tests := []struct {
		name      string
		errorType string
		expected  interface{}
		actual    interface{}
		context   string
		wantContext string
	}{
		{
			name:        "HTTP request context",
			errorType:   "status_code",
			expected:    200,
			actual:      404,
			context:     "GET /api/users/123",
			wantContext: "GET /api/users/123",
		},
		{
			name:        "OAuth validation context",
			errorType:   "error_message",
			expected:    "invalid.*token",
			actual:      "access_denied",
			context:     "OAuth token validation",
			wantContext: "OAuth token validation",
		},
		{
			name:        "Empty context",
			errorType:   "status_code",
			expected:    200,
			actual:      500,
			context:     "",
			wantContext: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := FormatValidationErrorHelper(tt.errorType, tt.expected, tt.actual,
				WithValidationContext(tt.context),
			)

			if err.Context != tt.wantContext {
				t.Errorf("Context = %v, want %v", err.Context, tt.wantContext)
			}

			// Verify context appears in error message if provided
			errorMsg := err.Error()
			if tt.context != "" && !strings.Contains(errorMsg, tt.context) {
				t.Errorf("Error message does not contain context %q: %s", tt.context, errorMsg)
			}
		})
	}
}

// TestFormatValidationErrorHelper_WithSuggestions tests the helper with custom suggestions.
func TestFormatValidationErrorHelper_WithSuggestions(t *testing.T) {
	tests := []struct {
		name         string
		errorType    string
		expected     interface{}
		actual       interface{}
		suggestions  []string
		wantSuggestionCount int
	}{
		{
			name:        "Custom suggestions for status code",
			errorType:   "status_code",
			expected:    200,
			actual:      404,
			suggestions: []string{"Check endpoint URL", "Verify resource exists"},
			wantSuggestionCount: 2,
		},
		{
			name:        "Single suggestion",
			errorType:   "error_message",
			expected:    "pattern",
			actual:      "nomatch",
			suggestions: []string{"Review pattern syntax"},
			wantSuggestionCount: 1,
		},
		{
			name:        "Multiple suggestions",
			errorType:   "custom_field",
			expected:    "required",
			actual:      "",
			suggestions: []string{"Field cannot be empty", "Check user input", "Verify form data"},
			wantSuggestionCount: 3,
		},
		{
			name:        "No custom suggestions (auto-generated)",
			errorType:   "status_code",
			expected:    200,
			actual:      404,
			suggestions: nil,
			wantSuggestionCount: 0, // Don't check count, just verify > 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err ValidationError
			if tt.suggestions != nil {
				err = FormatValidationErrorHelper(tt.errorType, tt.expected, tt.actual,
					WithSuggestions(tt.suggestions...),
				)
			} else {
				err = FormatValidationErrorHelper(tt.errorType, tt.expected, tt.actual)
			}

			if tt.wantSuggestionCount > 0 && len(err.Suggestions) != tt.wantSuggestionCount {
				t.Errorf("Suggestions count = %d, want %d", len(err.Suggestions), tt.wantSuggestionCount)
			}
			if tt.wantSuggestionCount == 0 && len(err.Suggestions) == 0 {
				t.Errorf("Expected auto-generated suggestions, got none")
			}

			// Verify suggestions appear in error message
			errorMsg := err.Error()
			for _, suggestion := range err.Suggestions {
				if !strings.Contains(errorMsg, suggestion) {
					t.Errorf("Error message does not contain suggestion %q: %s", suggestion, errorMsg)
				}
			}
		})
	}
}

// TestFormatValidationErrorHelper_WithFieldName tests the helper with field name option.
func TestFormatValidationErrorHelper_WithFieldName(t *testing.T) {
	tests := []struct {
		name      string
		errorType string
		expected  interface{}
		actual    interface{}
		fieldName string
		wantFieldName string
	}{
		{
			name:        "Error field",
			errorType:   "error_message",
			expected:    "pattern",
			actual:      "actual",
			fieldName:   "error",
			wantFieldName: "error",
		},
		{
			name:        "Message field",
			errorType:   "error_message",
			expected:    "expected",
			actual:      "actual",
			fieldName:   "message",
			wantFieldName: "message",
		},
		{
			name:        "Nested field",
			errorType:   "custom_validation",
			expected:    "value",
			actual:      "",
			fieldName:   "user.email",
			wantFieldName: "user.email",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := FormatValidationErrorHelper(tt.errorType, tt.expected, tt.actual,
				WithFieldName(tt.fieldName),
			)

			if err.FieldName != tt.wantFieldName {
				t.Errorf("FieldName = %v, want %v", err.FieldName, tt.wantFieldName)
			}

			// Verify field name appears in error message
			errorMsg := err.Error()
			if !strings.Contains(errorMsg, tt.fieldName) {
				t.Errorf("Error message does not contain field name %q: %s", tt.fieldName, errorMsg)
			}
		})
	}
}

// TestFormatValidationErrorHelper_Comprehensive tests the helper with multiple options.
func TestFormatValidationErrorHelper_Comprehensive(t *testing.T) {
	tests := []struct {
		name      string
		errorType string
		expected  interface{}
		actual    interface{}
		options   []FormatOption
		verifier  func(t *testing.T, err ValidationError)
	}{
		{
			name:      "Complete configuration",
			errorType: "error_message",
			expected:  "User .* not found",
			actual:    "Internal server error",
			options: []FormatOption{
				WithValidationContext("User profile lookup API"),
				WithFieldName("error"),
				WithResponseSnippet(`{"error": "Internal server error"}`),
				WithSuggestions("Check if user exists", "Verify user ID"),
			},
			verifier: func(t *testing.T, err ValidationError) {
				if err.Context != "User profile lookup API" {
					t.Errorf("Context not set correctly")
				}
				if err.FieldName != "error" {
					t.Errorf("FieldName not set correctly")
				}
				if err.ResponseSnippet != `{"error": "Internal server error"}` {
					t.Errorf("ResponseSnippet not set correctly")
				}
				if len(err.Suggestions) != 2 {
					t.Errorf("Expected 2 suggestions, got %d", len(err.Suggestions))
				}
			},
		},
		{
			name:      "Status code with range info",
			errorType: "status_code_range",
			expected:  "4xx",
			actual:    200,
			options: []FormatOption{
				WithRangeInfo("400-499 (Client Error)"),
				WithValidationContext("Error response check"),
			},
			verifier: func(t *testing.T, err ValidationError) {
				if err.RangeInfo != "400-499 (Client Error)" {
					t.Errorf("RangeInfo not set correctly")
				}
				if err.Context != "Error response check" {
					t.Errorf("Context not set correctly")
				}
			},
		},
		{
			name:      "Pattern matching with details",
			errorType: "error_message",
			expected:  "invalid.*token",
			actual:    "valid_token",
			options: []FormatOption{
				WithPatternDetails("Expected pattern 'invalid.*token' to match"),
				WithValidationDetails("Pattern validation failed", "No match found"),
			},
			verifier: func(t *testing.T, err ValidationError) {
				if err.PatternDetails != "Expected pattern 'invalid.*token' to match" {
					t.Errorf("PatternDetails not set correctly")
				}
				if len(err.ValidationDetails) != 2 {
					t.Errorf("Expected 2 validation details, got %d", len(err.ValidationDetails))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := FormatValidationErrorHelper(tt.errorType, tt.expected, tt.actual, tt.options...)

			// Verify basic structure
			if err.ErrorType != tt.errorType {
				t.Errorf("ErrorType = %v, want %v", err.ErrorType, tt.errorType)
			}
			if err.Expected != tt.expected {
				t.Errorf("Expected = %v, want %v", err.Expected, tt.expected)
			}
			if err.Actual != tt.actual {
				t.Errorf("Actual = %v, want %v", err.Actual, tt.actual)
			}

			// Run custom verifier
			if tt.verifier != nil {
				tt.verifier(t, err)
			}

			// Verify error message is generated
			errorMsg := err.Error()
			if errorMsg == "" {
				t.Error("Error() returned empty string")
			}
		})
	}
}

// TestFormatValidationErrorHelper_ErrorInterface tests that the returned error
// properly implements the error interface.
func TestFormatValidationErrorHelper_ErrorInterface(t *testing.T) {
	err := FormatValidationErrorHelper("status_code", 200, 404,
		WithValidationContext("GET /api/users"),
	)

	// Test that it implements error interface
	var _ error = err

	// Test Error() method
	errorMsg := err.Error()
	if errorMsg == "" {
		t.Error("Error() returned empty string")
	}

	// Test that error message contains expected components
	expectedComponents := []string{
		"status_code",
		"200",
		"404",
		"GET /api/users",
	}

	for _, component := range expectedComponents {
		if !strings.Contains(errorMsg, component) {
			t.Errorf("Error message does not contain expected component %q: %s", component, errorMsg)
		}
	}
}

// TestFormatValidationErrorHelper_DifferentValueTypes tests the helper with
// different value types for expected and actual parameters.
func TestFormatValidationErrorHelper_DifferentValueTypes(t *testing.T) {
	tests := []struct {
		name      string
		errorType string
		expected  interface{}
		actual    interface{}
	}{
		{
			name:      "Integer values",
			errorType: "status_code",
			expected:  200,
			actual:    404,
		},
		{
			name:      "String values",
			errorType: "error_message",
			expected:  "expected_pattern",
			actual:    "actual_message",
		},
		{
			name:      "Slice of integers",
			errorType: "status_code",
			expected:  []int{200, 201, 204},
			actual:    404,
		},
		{
			name:      "Nil expected value",
			errorType: "custom_validation",
			expected:  nil,
			actual:    "actual_value",
		},
		{
			name:      "Nil actual value",
			errorType: "custom_validation",
			expected:  "expected_value",
			actual:    nil,
		},
		{
			name:      "Both nil values",
			errorType: "custom_validation",
			expected:  nil,
			actual:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := FormatValidationErrorHelper(tt.errorType, tt.expected, tt.actual)

			// Verify values are set correctly (even if nil)
			// For slices, we need to check differently since Go doesn't allow direct comparison
			if expectedSlice, ok := tt.expected.([]int); ok {
				if actualSlice, ok := err.Expected.([]int); ok {
					if len(expectedSlice) != len(actualSlice) {
						t.Errorf("Expected slice length = %d, got %d", len(expectedSlice), len(actualSlice))
					} else {
						for i := range expectedSlice {
							if expectedSlice[i] != actualSlice[i] {
								t.Errorf("Expected[%d] = %v, got %v", i, expectedSlice[i], actualSlice[i])
							}
						}
					}
				} else {
					t.Errorf("Expected []int, got %T", err.Expected)
				}
			} else if err.Expected != tt.expected {
				t.Errorf("Expected = %v, want %v", err.Expected, tt.expected)
			}

			if actualSlice, ok := tt.actual.([]int); ok {
				if actualResultSlice, ok := err.Actual.([]int); ok {
					if len(actualSlice) != len(actualResultSlice) {
						t.Errorf("Actual slice length = %d, got %d", len(actualSlice), len(actualResultSlice))
					} else {
						for i := range actualSlice {
							if actualSlice[i] != actualResultSlice[i] {
								t.Errorf("Actual[%d] = %v, got %v", i, actualSlice[i], actualResultSlice[i])
							}
						}
					}
				} else {
					t.Errorf("Expected []int, got %T", err.Actual)
				}
			} else if err.Actual != tt.actual {
				t.Errorf("Actual = %v, want %v", err.Actual, tt.actual)
			}

			// Verify error message can be generated without panics
			errorMsg := err.Error()
			if errorMsg == "" {
				t.Error("Error() returned empty string")
			}
		})
	}
}

// TestFormatValidationErrorHelper_EdgeCases tests edge cases and boundary conditions.
func TestFormatValidationErrorHelper_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		errorType string
		expected  interface{}
		actual    interface{}
		options   []FormatOption
	}{
		{
			name:      "Empty error type",
			errorType: "",
			expected:  "value",
			actual:    "other",
		},
		{
			name:      "Very long context",
			errorType: "status_code",
			expected:  200,
			actual:    404,
			options: []FormatOption{
				WithValidationContext(strings.Repeat("very long context ", 100)),
			},
		},
		{
			name:      "Empty suggestions",
			errorType: "status_code",
			expected:  200,
			actual:    404,
			options: []FormatOption{
				WithSuggestions(),
			},
		},
		{
			name:      "Special characters in values",
			errorType: "error_message",
			expected:  "pattern with <special> & \"characters\"",
			actual:    "actual with 'quotes' and\nnewlines",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This should not panic
			err := FormatValidationErrorHelper(tt.errorType, tt.expected, tt.actual, tt.options...)

			// Verify basic structure
			if err.ErrorType != tt.errorType {
				t.Errorf("ErrorType = %v, want %v", err.ErrorType, tt.errorType)
			}

			// Verify error message can be generated
			errorMsg := err.Error()
			if errorMsg == "" {
				t.Error("Error() returned empty string")
			}
		})
	}
}

// TestFormatValidationErrorHelper_RealWorldScenarios tests real-world usage scenarios.
func TestFormatValidationErrorHelper_RealWorldScenarios(t *testing.T) {
	t.Run("API endpoint validation", func(t *testing.T) {
		err := FormatValidationErrorHelper("status_code", 200, 404,
			WithValidationContext("GET /api/users/123"),
			WithSuggestions("Verify endpoint URL", "Check if user exists"),
		)

		// Should contain all relevant information
		errorMsg := err.Error()
		requiredStrings := []string{
			"status_code",
			"200",
			"404",
			"GET /api/users/123",
			"Verify endpoint URL",
			"Check if user exists",
		}

		for _, s := range requiredStrings {
			if !strings.Contains(errorMsg, s) {
				t.Errorf("Error message missing required string %q: %s", s, errorMsg)
			}
		}
	})

	t.Run("Error message pattern validation", func(t *testing.T) {
		err := FormatValidationErrorHelper("error_message", "invalid.*token", "access_denied",
			WithFieldName("error"),
			WithValidationContext("OAuth token validation"),
		)

		errorMsg := err.Error()
		requiredStrings := []string{
			"error_message",
			"invalid.*token",
			"access_denied",
			"error",
			"OAuth token validation",
		}

		for _, s := range requiredStrings {
			if !strings.Contains(errorMsg, s) {
				t.Errorf("Error message missing required string %q: %s", s, errorMsg)
			}
		}
	})

	t.Run("Range validation with details", func(t *testing.T) {
		err := FormatValidationErrorHelper("status_code_range", "4xx", 200,
			WithRangeInfo("400-499 (Client Error)"),
			WithValidationContext("Error response validation"),
			WithValidationDetails("Expected error response", "Got success response"),
		)

		errorMsg := err.Error()
		if !strings.Contains(errorMsg, "4xx") {
			t.Errorf("Error message missing range pattern: %s", errorMsg)
		}
		if !strings.Contains(errorMsg, "200") {
			t.Errorf("Error message missing actual status: %s", errorMsg)
		}
	})
}

// TestFormatValidationErrorHelper_Consistency tests that the helper produces
// consistent results across multiple calls with the same parameters.
func TestFormatValidationErrorHelper_Consistency(t *testing.T) {
	params := struct {
		errorType string
		expected  interface{}
		actual    interface{}
		options   []FormatOption
	}{
		errorType: "status_code",
		expected:  200,
		actual:    404,
		options: []FormatOption{
			WithValidationContext("GET /api/users"),
			WithSuggestions("Check endpoint", "Verify resource"),
		},
	}

	// Call the helper multiple times with the same parameters
	var errors []ValidationError
	for i := 0; i < 5; i++ {
		err := FormatValidationErrorHelper(params.errorType, params.expected, params.actual, params.options...)
		errors = append(errors, err)
	}

	// Verify all errors are identical
	firstError := errors[0]
	for i, err := range errors {
		if err.ErrorType != firstError.ErrorType {
			t.Errorf("Error %d: ErrorType inconsistent", i)
		}
		if err.Expected != firstError.Expected {
			t.Errorf("Error %d: Expected inconsistent", i)
		}
		if err.Actual != firstError.Actual {
			t.Errorf("Error %d: Actual inconsistent", i)
		}
		if err.Context != firstError.Context {
			t.Errorf("Error %d: Context inconsistent", i)
		}
		if len(err.Suggestions) != len(firstError.Suggestions) {
			t.Errorf("Error %d: Suggestions count inconsistent", i)
		}
		if err.Error() != firstError.Error() {
			t.Errorf("Error %d: Error message inconsistent", i)
		}
	}
}

// TestFormatValidationErrorHelper_AutoGeneratedSuggestions tests that
// appropriate suggestions are auto-generated when custom ones aren't provided.
func TestFormatValidationErrorHelper_AutoGeneratedSuggestions(t *testing.T) {
	tests := []struct {
		name              string
		errorType         string
		expected          interface{}
		actual            interface{}
		wantSuggestionCount int
		suggestionKeywords []string // Keywords that should appear in suggestions
	}{
		{
			name:              "Status code 404",
			errorType:         "status_code",
			expected:          200,
			actual:            404,
			wantSuggestionCount: 0, // Don't enforce exact count
			suggestionKeywords: []string{"endpoint", "resource", "URL"},
		},
		{
			name:              "Status code 401",
			errorType:         "status_code",
			expected:          200,
			actual:            401,
			wantSuggestionCount: 0, // Don't enforce exact count
			suggestionKeywords: []string{"authentication", "credentials", "token"},
		},
		{
			name:              "Status code 500",
			errorType:         "status_code",
			expected:          200,
			actual:            500,
			wantSuggestionCount: 4,
			suggestionKeywords: []string{"retry", "server", "service", "backoff"},
		},
		{
			name:              "Error message mismatch",
			errorType:         "error_message",
			expected:          "pattern",
			actual:            "nomatch",
			wantSuggestionCount: 0, // Don't enforce exact count
			suggestionKeywords: []string{"message", "check", "review"},
		},
		{
			name:              "Content type mismatch",
			errorType:         "content_type",
			expected:          "application/json",
			actual:            "text/html",
			wantSuggestionCount: 3,
			suggestionKeywords: []string{"content-type", "header", "format"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := FormatValidationErrorHelper(tt.errorType, tt.expected, tt.actual)

			// Check suggestion count (only if specified)
			if tt.wantSuggestionCount > 0 && len(err.Suggestions) != tt.wantSuggestionCount {
				t.Logf("Got %d suggestions: %v", len(err.Suggestions), err.Suggestions)
				t.Errorf("Want %d suggestions, got %d", tt.wantSuggestionCount, len(err.Suggestions))
			}
			if tt.wantSuggestionCount == 0 && len(err.Suggestions) == 0 {
				t.Errorf("Expected auto-generated suggestions, got none")
			}

			// Check that suggestions contain expected keywords
			errorMsg := err.Error()
			for _, keyword := range tt.suggestionKeywords {
				found := false
				for _, suggestion := range err.Suggestions {
					if strings.Contains(strings.ToLower(suggestion), strings.ToLower(keyword)) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("No suggestion contains keyword %q. Suggestions: %v", keyword, err.Suggestions)
				}
			}

			// Verify suggestions appear in error message
			for _, suggestion := range err.Suggestions {
				if !strings.Contains(errorMsg, suggestion) {
					t.Errorf("Error message does not contain suggestion %q: %s", suggestion, errorMsg)
				}
			}
		})
	}
}

// TestFormatValidationErrorHelper_MultipleOptions tests that multiple options
// can be combined and work together correctly.
func TestFormatValidationErrorHelper_MultipleOptions(t *testing.T) {
	err := FormatValidationErrorHelper("error_message", "expected_pattern", "actual_value",
		WithValidationContext("Test validation"),
		WithFieldName("error_field"),
		WithResponseSnippet(`{"error": "actual_value"}`),
		WithPatternDetails("Pattern matching failed"),
		WithValidationDetails("Detail 1", "Detail 2"),
		WithSuggestions("Suggestion 1", "Suggestion 2"),
	)

	// Verify all options are applied
	if err.Context != "Test validation" {
		t.Errorf("Context not applied")
	}
	if err.FieldName != "error_field" {
		t.Errorf("FieldName not applied")
	}
	if err.ResponseSnippet != `{"error": "actual_value"}` {
		t.Errorf("ResponseSnippet not applied")
	}
	if err.PatternDetails != "Pattern matching failed" {
		t.Errorf("PatternDetails not applied")
	}
	if len(err.ValidationDetails) != 2 {
		t.Errorf("ValidationDetails not applied correctly, got %d", len(err.ValidationDetails))
	}
	if len(err.Suggestions) != 2 {
		t.Errorf("Suggestions not applied correctly, got %d", len(err.Suggestions))
	}

	// Verify error message contains all information
	errorMsg := err.Error()
	requiredComponents := []string{
		"Test validation",
		"error_field",
		"Pattern matching failed",
		"Detail 1",
		"Detail 2",
		"Suggestion 1",
		"Suggestion 2",
	}

	for _, component := range requiredComponents {
		if !strings.Contains(errorMsg, component) {
			t.Errorf("Error message missing component %q: %s", component, errorMsg)
		}
	}
}