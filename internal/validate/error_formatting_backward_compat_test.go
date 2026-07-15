package validate

import (
	"strings"
	"testing"
)

// =============================================================================
// COMPREHENSIVE BACKWARD COMPATIBILITY TESTS
//
// This file contains comprehensive tests for backward compatibility of optional
// error fields in the error formatter. These tests ensure that:
// - All optional fields are truly optional
// - Existing code that calls the formatter still compiles without changes
// - Formatter performance hasn't degraded
// - All parameter combinations work correctly
//
// Acceptance Criteria:
// - Add comprehensive integration tests calling formatter with no optional parameters
// - Add tests with only one optional parameter at a time
// - Add tests with all optional parameters combined
// - Verify existing code that calls the formatter still compiles without changes
// - Add benchmark test to ensure formatter performance hasn't degraded
// - Update documentation showing optional parameters are not required
// =============================================================================

// TestFormatValidationErrorWithDetails_NoOptionalParameters tests that the formatter
// works correctly with no optional parameters (only required parameters).
func TestFormatValidationErrorWithDetails_NoOptionalParameters(t *testing.T) {
	tests := []struct {
		name          string
		validationType string
		expected      interface{}
		actual        interface{}
		context       string
		responseSnippet string
		checkOutput   func(t *testing.T, result ValidationError)
	}{
		{
			name:           "minimum required parameters only",
			validationType: "status_code",
			expected:       200,
			actual:         404,
			context:        "HTTP response validation",
			responseSnippet: `{"status": 404}`,
			checkOutput: func(t *testing.T, result ValidationError) {
				// Should create a valid ValidationError
				if result.ErrorType != "status_code" {
					t.Errorf("Expected ErrorType 'status_code', got %q", result.ErrorType)
				}
				if result.Expected != 200 {
					t.Errorf("Expected Expected 200, got %v", result.Expected)
				}
				if result.Actual != 404 {
					t.Errorf("Expected Actual 404, got %v", result.Actual)
				}
				// Optional fields should be empty/nil
				if result.FieldName != "" {
					t.Errorf("Expected empty FieldName, got %q", result.FieldName)
				}
				if result.Location != "" {
					t.Errorf("Expected empty Location, got %q", result.Location)
				}
				if result.RelatedFields != nil {
					t.Errorf("Expected nil RelatedFields, got %v", result.RelatedFields)
				}
			},
		},
		{
			name:           "string validation with minimum params",
			validationType: "error_message",
			expected:       "access_denied",
			actual:         "invalid_token",
			context:        "OAuth token validation",
			responseSnippet: `{"error": "invalid_token"}`,
			checkOutput: func(t *testing.T, result ValidationError) {
				if result.ErrorType != "error_message" {
					t.Errorf("Expected ErrorType 'error_message', got %q", result.ErrorType)
				}
				// Should have auto-generated suggestions
				if len(result.Suggestions) == 0 {
					t.Errorf("Expected auto-generated suggestions, got none")
				}
			},
		},
		{
			name:           "content type validation with minimum params",
			validationType: "content_type",
			expected:       "application/json",
			actual:         "text/html",
			context:        "Response content-type validation",
			responseSnippet: "<!DOCTYPE html>",
			checkOutput: func(t *testing.T, result ValidationError) {
				if result.ErrorType != "content_type" {
					t.Errorf("Expected ErrorType 'content_type', got %q", result.ErrorType)
				}
				// Message should be generated from parts
				if result.Message == "" {
					t.Errorf("Expected generated message, got empty string")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call with only required parameters - all optional params omitted
			result := FormatValidationErrorWithDetails(
				tt.validationType,
				tt.expected,
				tt.actual,
				tt.context,
				tt.responseSnippet,
				// All optional parameters omitted from here:
				"",           // fieldName
				"",           // location
				nil,          // relatedFields
				"",           // patternDetails
				"",           // rangeInfo
				nil,          // validationDetails
			)
			tt.checkOutput(t, result)
		})
	}
}

// TestFormatValidationErrorWithDetails_OneOptionalParameterAtATime tests that the formatter
// works correctly with only one optional parameter provided at a time.
func TestFormatValidationErrorWithDetails_OneOptionalParameterAtATime(t *testing.T) {
	baseParams := struct {
		validationType string
		expected       interface{}
		actual         interface{}
		context        string
		responseSnippet string
	}{
		validationType:  "status_code",
		expected:        200,
		actual:          404,
		context:         "HTTP validation",
		responseSnippet: `{"error": "not_found"}`,
	}

	tests := []struct {
		name           string
		fieldName      string
		location       string
		relatedFields  []string
		patternDetails string
		rangeInfo      string
		validationDetails []string
		customSuggestions []string
		checkOutput    func(t *testing.T, result ValidationError)
	}{
		{
			name:      "only fieldName optional parameter",
			fieldName: "response.status",
			checkOutput: func(t *testing.T, result ValidationError) {
				if result.FieldName != "response.status" {
					t.Errorf("Expected FieldName 'response.status', got %q", result.FieldName)
				}
				// All other optional fields should be empty/nil
				if result.Location != "" {
					t.Errorf("Expected empty Location, got %q", result.Location)
				}
			},
		},
		{
			name:    "only location optional parameter",
			location: "line 42 in config.yaml",
			checkOutput: func(t *testing.T, result ValidationError) {
				if result.Location != "line 42 in config.yaml" {
					t.Errorf("Expected Location 'line 42 in config.yaml', got %q", result.Location)
				}
				if result.FieldName != "" {
					t.Errorf("Expected empty FieldName, got %q", result.FieldName)
				}
			},
		},
		{
			name:          "only relatedFields optional parameter",
			relatedFields: []string{"status_code", "error_code"},
			checkOutput: func(t *testing.T, result ValidationError) {
				if len(result.RelatedFields) != 2 {
					t.Errorf("Expected 2 RelatedFields, got %d", len(result.RelatedFields))
				}
				if result.FieldName != "" {
					t.Errorf("Expected empty FieldName, got %q", result.FieldName)
				}
			},
		},
		{
			name:          "only patternDetails optional parameter",
			patternDetails: "regex pattern '2\\d{2}' did not match",
			checkOutput: func(t *testing.T, result ValidationError) {
				if result.PatternDetails != "regex pattern '2\\d{2}' did not match" {
					t.Errorf("Expected PatternDetails %q, got %q", "regex pattern '2\\d{2}' did not match", result.PatternDetails)
				}
				if result.FieldName != "" {
					t.Errorf("Expected empty FieldName, got %q", result.FieldName)
				}
			},
		},
		{
			name:     "only rangeInfo optional parameter",
			rangeInfo: "200-299 (Success)",
			checkOutput: func(t *testing.T, result ValidationError) {
				if result.RangeInfo != "200-299 (Success)" {
					t.Errorf("Expected RangeInfo '200-299 (Success)', got %q", result.RangeInfo)
				}
				if result.FieldName != "" {
					t.Errorf("Expected empty FieldName, got %q", result.FieldName)
				}
			},
		},
		{
			name:             "only validationDetails optional parameter",
			validationDetails: []string{"Checked status code", "Expected 2xx", "Got 404"},
			checkOutput: func(t *testing.T, result ValidationError) {
				if len(result.ValidationDetails) != 3 {
					t.Errorf("Expected 3 ValidationDetails, got %d", len(result.ValidationDetails))
				}
				if result.FieldName != "" {
					t.Errorf("Expected empty FieldName, got %q", result.FieldName)
				}
			},
		},
		{
			name:             "only customSuggestions optional parameter",
			customSuggestions: []string{"Check URL", "Verify authentication"},
			checkOutput: func(t *testing.T, result ValidationError) {
				if len(result.Suggestions) != 2 {
					t.Errorf("Expected 2 Suggestions, got %d", len(result.Suggestions))
				}
				// Should use custom suggestions, not auto-generated
				if result.Suggestions[0] != "Check URL" {
					t.Errorf("Expected custom suggestion 'Check URL', got %q", result.Suggestions[0])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call with base params plus only one optional parameter
			result := FormatValidationErrorWithDetails(
				baseParams.validationType,
				baseParams.expected,
				baseParams.actual,
				baseParams.context,
				baseParams.responseSnippet,
				tt.fieldName,
				tt.location,
				tt.relatedFields,
				tt.patternDetails,
				tt.rangeInfo,
				tt.validationDetails,
				tt.customSuggestions..., // Spread the slice
			)
			tt.checkOutput(t, result)
		})
	}
}

// TestFormatValidationErrorWithDetails_AllOptionalParameters tests that the formatter
// works correctly with all optional parameters combined.
func TestFormatValidationErrorWithDetails_AllOptionalParameters(t *testing.T) {
	result := FormatValidationErrorWithDetails(
		"status_code",
		200,
		404,
		"HTTP response validation",
		`{"error": "not_found", "path": "/api/users/123"}`,
		"response.status",
		"line 15 in api_client.go",
		[]string{"error_code", "response_body"},
		"expected status code 2xx, got 404",
		"200-299 (Success)",
		[]string{
			"Made GET request to /api/users/123",
			"Expected success status code",
			"Received 404 Not Found",
		},
		"Verify the resource exists", "Check authentication", "Review API endpoint",
	)

	// Verify all required fields
	if result.ErrorType != "status_code" {
		t.Errorf("Expected ErrorType 'status_code', got %q", result.ErrorType)
	}
	if result.Expected != 200 {
		t.Errorf("Expected Expected 200, got %v", result.Expected)
	}
	if result.Actual != 404 {
		t.Errorf("Expected Actual 404, got %v", result.Actual)
	}

	// Verify all optional fields are populated
	if result.FieldName != "response.status" {
		t.Errorf("Expected FieldName 'response.status', got %q", result.FieldName)
	}
	if result.Location != "line 15 in api_client.go" {
		t.Errorf("Expected Location 'line 15 in api_client.go', got %q", result.Location)
	}
	if len(result.RelatedFields) != 2 {
		t.Errorf("Expected 2 RelatedFields, got %d", len(result.RelatedFields))
	}
	if result.PatternDetails != "expected status code 2xx, got 404" {
		t.Errorf("Expected PatternDetails %q, got %q", "expected status code 2xx, got 404", result.PatternDetails)
	}
	if result.RangeInfo != "200-299 (Success)" {
		t.Errorf("Expected RangeInfo '200-299 (Success)', got %q", result.RangeInfo)
	}
	if len(result.ValidationDetails) != 3 {
		t.Errorf("Expected 3 ValidationDetails, got %d", len(result.ValidationDetails))
	}
	if len(result.Suggestions) != 3 {
		t.Errorf("Expected 3 Suggestions, got %d", len(result.Suggestions))
	}
	// Should use custom suggestions
	if result.Suggestions[0] != "Verify the resource exists" {
		t.Errorf("Expected custom suggestion 'Verify the resource exists', got %q", result.Suggestions[0])
	}
}

// TestFormatValidationErrorFull_NoOptionalContext tests FormatValidationErrorFull
// with no optional context parameter (context = nil).
func TestFormatValidationErrorFull_NoOptionalContext(t *testing.T) {
	err := ValidationError{
		ErrorType: string(ErrTypeRequired),
		Message:   "Field is required",
		FieldName: "email",
	}

	tests := []struct {
		name            string
		err             ValidationError
		includeSeverity bool
		context         *ValidationErrorContext
		expected        string
	}{
		{
			name:            "no context - with severity",
			err:             err,
			includeSeverity: true,
			context:         nil,
			expected:        "[⚠️] High [required] email: Field is required",
		},
		{
			name:            "no context - without severity",
			err:             err,
			includeSeverity: false,
			context:         nil,
			expected:        "[required] email: Field is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatValidationErrorFull(tt.err, tt.includeSeverity, tt.context)
			if result != tt.expected {
				t.Errorf("FormatValidationErrorFull() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestFormatValidationErrorFull_WithOptionalContext tests FormatValidationErrorFull
// with optional context parameter provided.
func TestFormatValidationErrorFull_WithOptionalContext(t *testing.T) {
	err := ValidationError{
		ErrorType: string(ErrTypeFormat),
		Message:   "Invalid format",
		FieldName: "email",
	}

	tests := []struct {
		name            string
		err             ValidationError
		includeSeverity bool
		context         *ValidationErrorContext
		expected        string
	}{
		{
			name:            "with location context",
			err:             err,
			includeSeverity: true,
			context:         &ValidationErrorContext{Location: "line 5"},
			expected:        "[⚡] Medium [format] email: Invalid format (location: line 5)",
		},
		{
			name:            "with related fields context",
			err:             err,
			includeSeverity: true,
			context:         &ValidationErrorContext{Location: "", RelatedFields: []string{"email_confirmation"}},
			expected:        "[⚡] Medium [format] email: Invalid format (related fields: email_confirmation)",
		},
		{
			name:            "with full context",
			err:             err,
			includeSeverity: true,
			context:         &ValidationErrorContext{Location: "line 10", RelatedFields: []string{"email_confirmation", "user.email"}},
			expected:        "[⚡] Medium [format] email: Invalid format (location: line 10, related fields: email_confirmation and user.email)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatValidationErrorFull(tt.err, tt.includeSeverity, tt.context)
			if result != tt.expected {
				t.Errorf("FormatValidationErrorFull() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestFormatValidationErrorWithExpectedActual_NoOptionalParameters tests
// FormatValidationErrorWithExpectedActual with no optional parameters.
func TestFormatValidationErrorWithExpectedActual_NoOptionalParameters(t *testing.T) {
	err := ValidationError{
		ErrorType: string(ErrTypeRequired),
		Message:   "Field is required",
		FieldName: "email",
	}

	tests := []struct {
		name            string
		err             ValidationError
		includeSeverity bool
		context         *ValidationErrorContext
		expectedActual  ExpectedActual
		expected        string
	}{
		{
			name:           "no optional parameters - with severity",
			err:            err,
			includeSeverity: true,
			context:        nil,
			expectedActual: ExpectedActual{},
			expected:       "[⚠️] High [required] email: Field is required",
		},
		{
			name:           "no optional parameters - without severity",
			err:            err,
			includeSeverity: false,
			context:        nil,
			expectedActual: ExpectedActual{},
			expected:       "[required] email: Field is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatValidationErrorWithExpectedActual(tt.err, tt.includeSeverity, tt.context, tt.expectedActual)
			if result != tt.expected {
				t.Errorf("FormatValidationErrorWithExpectedActual() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestFormatValidationErrorWithExpectedActual_WithOptionalParameters tests
// FormatValidationErrorWithExpectedActual with optional parameters.
func TestFormatValidationErrorWithExpectedActual_WithOptionalParameters(t *testing.T) {
	err := ValidationError{
		ErrorType: "status_code",
		Message:   "Wrong status code",
		FieldName: "response",
	}

	tests := []struct {
		name            string
		err             ValidationError
		includeSeverity bool
		context         *ValidationErrorContext
		expectedActual  ExpectedActual
		expected        string
	}{
		{
			name:            "with ExpectedActual only",
			err:             err,
			includeSeverity: true,
			context:         nil,
			expectedActual: ExpectedActual{Expected: 200, Actual: 404},
			expected:       "[⚠️] High [status_code] response: Wrong status code (expected: 200 (OK), actual: 404 (Not Found))",
		},
		{
			name:            "with context only",
			err:             err,
			includeSeverity: true,
			context:         &ValidationErrorContext{Location: "line 15"},
			expectedActual: ExpectedActual{},
			expected:       "[⚠️] High [status_code] response: Wrong status code (location: line 15)",
		},
		{
			name:            "with both ExpectedActual and context",
			err:             err,
			includeSeverity: true,
			context:         &ValidationErrorContext{Location: "line 20", RelatedFields: []string{"response_body"}},
			expectedActual: ExpectedActual{Expected: []int{200, 201, 204}, Actual: 404},
			expected:       "[⚠️] High [status_code] response: Wrong status code (expected: one of [200 (OK), 201 (Created), 204 (No Content)], actual: 404 (Not Found)) (location: line 20, related fields: response_body)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatValidationErrorWithExpectedActual(tt.err, tt.includeSeverity, tt.context, tt.expectedActual)
			if result != tt.expected {
				t.Errorf("FormatValidationErrorWithExpectedActual() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestFormatValidationError_VariadicSuggestions tests FormatValidationError
// with variadic customSuggestions parameter (can be 0, 1, or multiple suggestions).
func TestFormatValidationError_VariadicSuggestions(t *testing.T) {
	tests := []struct {
		name             string
		validationType   string
		expected         interface{}
		actual           interface{}
		context          string
		responseSnippet  string
		customSuggestions []string
		expectedSuggestionCount int
	}{
		{
			name:                     "no custom suggestions (auto-generated)",
			validationType:           "status_code",
			expected:                 200,
			actual:                   404,
			context:                  "HTTP validation",
			responseSnippet:          `{"error": "not_found"}`,
			customSuggestions:        []string{},
			expectedSuggestionCount:  0, // Will be auto-generated
		},
		{
			name:                     "one custom suggestion",
			validationType:           "status_code",
			expected:                 200,
			actual:                   404,
			context:                  "HTTP validation",
			responseSnippet:          `{"error": "not_found"}`,
			customSuggestions:        []string{"Check URL"},
			expectedSuggestionCount:  1,
		},
		{
			name:                     "multiple custom suggestions",
			validationType:           "status_code",
			expected:                 200,
			actual:                   404,
			context:                  "HTTP validation",
			responseSnippet:          `{"error": "not_found"}`,
			customSuggestions:        []string{"Check URL", "Verify auth", "Check permissions"},
			expectedSuggestionCount:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call FormatValidationError (simpler version) with variadic suggestions
			result := FormatValidationError(
				tt.validationType,
				tt.expected,
				tt.actual,
				tt.context,
				tt.responseSnippet,
				tt.customSuggestions...,
			)

			// Verify result was created successfully
			if result.ErrorType != tt.validationType {
				t.Errorf("Expected ErrorType %q, got %q", tt.validationType, result.ErrorType)
			}

			// If custom suggestions were provided, verify count
			if tt.customSuggestions != nil && len(tt.customSuggestions) > 0 {
				if len(result.Suggestions) != tt.expectedSuggestionCount {
					t.Errorf("Expected %d suggestions, got %d", tt.expectedSuggestionCount, len(result.Suggestions))
				}
			}
		})
	}
}

// TestBackwardCompatibility_ExistingCodeCompilation tests that existing code
// patterns still compile and work correctly. This simulates how existing code
// calls the formatter functions.
func TestBackwardCompatibility_ExistingCodeCompilation(t *testing.T) {
	// Test 1: Old-style call with positional parameters (should still compile)
	t.Run("old style positional parameters", func(t *testing.T) {
		err := FormatValidationErrorWithDetails(
			"status_code",        // validationType
			200,                  // expected
			404,                  // actual
			"HTTP validation",    // context
			`{"error": "not_found"}`, // responseSnippet
			"",                   // fieldName (optional)
			"",                   // location (optional)
			nil,                  // relatedFields (optional)
			"",                   // patternDetails (optional)
			"",                   // rangeInfo (optional)
			nil,                  // validationDetails (optional)
		)
		if err.ErrorType != "status_code" {
			t.Errorf("Basic backward compatibility failed")
		}
	})

	// Test 2: Call with some optional parameters (should still compile)
	t.Run("call with some optional params", func(t *testing.T) {
		err := FormatValidationErrorWithDetails(
			"error_message",
			"invalid_token",
			"access_denied",
			"OAuth validation",
			`{"error": "access_denied"}`,
			"error",                    // fieldName (optional)
			"line 42",                  // location (optional)
			nil,                        // relatedFields (optional)
			"",                         // patternDetails (optional)
			"",                         // rangeInfo (optional)
			nil,                        // validationDetails (optional)
		)
		if err.ErrorType != "error_message" {
			t.Errorf("Partial optional params compatibility failed")
		}
		if err.FieldName != "error" {
			t.Errorf("FieldName not set correctly")
		}
		if err.Location != "line 42" {
			t.Errorf("Location not set correctly")
		}
	})

	// Test 3: FormatValidationErrorFull with nil context (should still compile)
	t.Run("FormatValidationErrorFull with nil context", func(t *testing.T) {
		err := ValidationError{
			ErrorType: string(ErrTypeRequired),
			Message:   "Field is required",
			FieldName: "email",
		}
		result := FormatValidationErrorFull(err, true, nil)
		if !strings.Contains(result, "required") {
			t.Errorf("FormatValidationErrorFull with nil context failed")
		}
	})

	// Test 4: FormatValidationErrorWithExpectedActual with empty ExpectedActual (should still compile)
	t.Run("FormatValidationErrorWithExpectedActual with empty ExpectedActual", func(t *testing.T) {
		err := ValidationError{
			ErrorType: string(ErrTypeFormat),
			Message:   "Invalid format",
			FieldName: "email",
		}
		result := FormatValidationErrorWithExpectedActual(err, true, nil, ExpectedActual{})
		if !strings.Contains(result, "format") {
			t.Errorf("FormatValidationErrorWithExpectedActual with empty ExpectedActual failed")
		}
	})
}

// TestOptionalFields_TrulyOptional tests that optional fields can be omitted
// without affecting the core functionality.
func TestOptionalFields_TrulyOptional(t *testing.T) {
	// Create validation errors with different optional field combinations
	testCases := []struct {
		name         string
		createError  func() ValidationError
		shouldFormat bool
	}{
		{
			name: "no optional fields",
			createError: func() ValidationError {
				return ValidationError{
					ErrorType: "required",
					Message:   "Field is required",
				}
			},
			shouldFormat: true,
		},
		{
			name: "with FieldName only",
			createError: func() ValidationError {
				return ValidationError{
					ErrorType: "required",
					Message:   "Field is required",
					FieldName: "email",
				}
			},
			shouldFormat: true,
		},
		{
			name: "with Location only",
			createError: func() ValidationError {
				return ValidationError{
					ErrorType: "required",
					Message:   "Field is required",
					Location:  "line 5",
				}
			},
			shouldFormat: true,
		},
		{
			name: "with RelatedFields only",
			createError: func() ValidationError {
				return ValidationError{
					ErrorType: "required",
					Message:   "Field is required",
					RelatedFields: []string{"email", "email_confirmation"},
				}
			},
			shouldFormat: true,
		},
		{
			name: "with all optional fields",
			createError: func() ValidationError {
				return ValidationError{
					ErrorType: "required",
					Message:   "Field is required",
					FieldName: "email",
					Location:  "line 5",
					RelatedFields: []string{"email_confirmation"},
				}
			},
			shouldFormat: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.createError()
			result := FormatValidationErrorFull(err, false, nil)

			if tc.shouldFormat && result == "" {
				t.Errorf("Expected valid formatted output, got empty string")
			}

			// All should contain the error type and message
			if !strings.Contains(result, "required") {
				t.Errorf("Result should contain error type, got: %q", result)
			}
			if !strings.Contains(result, "Field is required") {
				t.Errorf("Result should contain message, got: %q", result)
			}
		})
	}
}
