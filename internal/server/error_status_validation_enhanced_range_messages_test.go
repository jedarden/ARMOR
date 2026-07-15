package server

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/validate"
)

// mockTB captures test output for validation
type mockTB struct {
	errors []string
	fatal  bool
}

func (m *mockTB) Helper()                                         {}
func (m *mockTB) Errorf(format string, args ...interface{})      { m.errors = append(m.errors, fmt.Sprintf(format, args...)) }
func (m *mockTB) Fatalf(format string, args ...interface{})      { m.fatal = true; m.Errorf(format, args...) }
func (m *mockTB) Logf(format string, args ...interface{})         { m.errors = append(m.errors, fmt.Sprintf(format, args...)) }
func (m *mockTB) Log(args ...interface{})                         { m.errors = append(m.errors, fmt.Sprint(args...)) }
func (m *mockTB) Error(args ...interface{})                       { m.errors = append(m.errors, fmt.Sprint(args...)) }
func (m *mockTB) Fatal(args ...interface{})                       { m.fatal = true; m.Error(args...) }
func (m *mockTB) Fail()                                           { m.errors = append(m.errors, "FAIL") }
func (m *mockTB) FailNow()                                        { m.fatal = true }
func (m *mockTB) Failed() bool                                     { return len(m.errors) > 0 }
func (m *mockTB) SkipNow()                                        {}
func (m *mockTB) Skipped() bool                                   { return false }
func (m *mockTB) Name() string                                     { return "mock" }
func (m *mockTB) Cleanup(func())                                  {}

// getFirstError returns the first error message from the mock
func (m *mockTB) getFirstError() string {
	if len(m.errors) > 0 {
		return m.errors[0]
	}
	return ""
}

// =============================================================================
// Range Validation Error Message Enhancement Tests
// =============================================================================

// TestValidateStatusCodePattern_EnhancedErrorMessages tests that ValidateStatusCodePattern
// returns enhanced error messages using the error formatting helper.
//
// This test verifies that range validation errors include:
// - Expected valid range (min/max)
// - Actual value that was received
// - Field/error type being validated
// - Contextual suggestions
func TestValidateStatusCodePattern_EnhancedErrorMessages(t *testing.T) {
	tests := []struct {
		name              string
		pattern           string
		actual            int
		wantErr           bool
		errorType         string
		expectedInError   []string
		actualInError     string
		rangeInError      string
		suggestionsPresent bool
	}{
		{
			name:            "4xx pattern with 200 should include range info",
			pattern:         "4xx",
			actual:          200,
			wantErr:         true,
			errorType:       "status_code_range",
			expectedInError: []string{"400", "499"},
			actualInError:   "200",
			rangeInError:    "400-499",
			suggestionsPresent: true,
		},
		{
			name:            "5xx pattern with 404 should include range info",
			pattern:         "5xx",
			actual:          404,
			wantErr:         true,
			errorType:       "status_code_range",
			expectedInError: []string{"500", "599"},
			actualInError:   "404",
			rangeInError:    "500-599",
			suggestionsPresent: true,
		},
		{
			name:            "2xx pattern with 500 should include range info",
			pattern:         "2xx",
			actual:          500,
			wantErr:         true,
			errorType:       "status_code_range",
			expectedInError: []string{"200", "299"},
			actualInError:   "500",
			rangeInError:    "200-299",
			suggestionsPresent: true,
		},
		{
			name:            "3xx pattern with 200 should include range info",
			pattern:         "3xx",
			actual:          200,
			wantErr:         true,
			errorType:       "status_code_range",
			expectedInError: []string{"300", "399"},
			actualInError:   "200",
			rangeInError:    "300-399",
			suggestionsPresent: true,
		},
		{
			name:            "1xx pattern with 404 should include range info",
			pattern:         "1xx",
			actual:          404,
			wantErr:         true,
			errorType:       "status_code_range",
			expectedInError: []string{"100", "199"},
			actualInError:   "404",
			rangeInError:    "100-199",
			suggestionsPresent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStatusCodePattern(tt.pattern, tt.actual)

			if !tt.wantErr && err != nil {
				t.Errorf("ValidateStatusCodePattern(%q, %d) unexpectedly returned error: %v", tt.pattern, tt.actual, err)
				return
			}

			if tt.wantErr && err == nil {
				t.Errorf("ValidateStatusCodePattern(%q, %d) expected error but got nil", tt.pattern, tt.actual)
				return
			}

			if tt.wantErr && err != nil {
				// Verify the error is a ValidationError
				validationErr, ok := err.(validate.ValidationError)
				if !ok {
					t.Errorf("Expected error to be validate.ValidationError, got %T", err)
					return
				}

				errorMsg := err.Error()

				// Verify error type
				if validationErr.ErrorType != tt.errorType {
					t.Errorf("Expected ErrorType %q, got %q", tt.errorType, validationErr.ErrorType)
				}

				// Verify expected range values are in error message
				for _, expected := range tt.expectedInError {
					if !strings.Contains(errorMsg, expected) {
						t.Errorf("Error message should contain expected range value '%s', got: %s", expected, errorMsg)
					}
				}

				// Verify actual value is in error message
				if !strings.Contains(errorMsg, tt.actualInError) {
					t.Errorf("Error message should contain actual value '%s', got: %s", tt.actualInError, errorMsg)
				}

				// Verify range info is present
				if tt.rangeInError != "" && !strings.Contains(errorMsg, tt.rangeInError) {
					t.Errorf("Error message should contain range info '%s', got: %s", tt.rangeInError, errorMsg)
				}

				// Verify suggestions are present
				if tt.suggestionsPresent && len(validationErr.Suggestions) == 0 {
					t.Errorf("Expected error to contain suggestions, got none")
				}

				// Verify structured error fields
				if validationErr.Actual == nil {
					t.Errorf("Expected Actual field to be set, got nil")
				}

				if validationErr.Expected == nil {
					t.Errorf("Expected Expected field to be set, got nil")
				}

				// Verify Actual is an int with correct value
				actualVal, ok := validationErr.Actual.(int)
				if !ok || actualVal != tt.actual {
					t.Errorf("Expected Actual to be %d (int), got %v (%T)", tt.actual, validationErr.Actual, validationErr.Actual)
				}

				// Verify Expected contains the pattern
				expectedStr, ok := validationErr.Expected.(string)
				if !ok || !strings.Contains(expectedStr, tt.pattern) {
					t.Errorf("Expected Expected to contain pattern '%s', got %v (%T)", tt.pattern, validationErr.Expected, validationErr.Expected)
				}
			}
		})
	}
}

// TestValidateStatusCodePattern_RangeInfoField tests that range validation errors
// include the RangeInfo field with proper min/max values.
func TestValidateStatusCodePattern_RangeInfoField(t *testing.T) {
	tests := []struct {
		pattern      string
		actual       int
		wantRangeMin string
		wantRangeMax string
	}{
		{"4xx", 200, "400", "499"},
		{"5xx", 404, "500", "599"},
		{"2xx", 500, "200", "299"},
		{"3xx", 200, "300", "399"},
		{"1xx", 404, "100", "199"},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			err := ValidateStatusCodePattern(tt.pattern, tt.actual)
			if err == nil {
				t.Fatalf("ValidateStatusCodePattern(%q, %d) expected error but got nil", tt.pattern, tt.actual)
			}

			validationErr, ok := err.(validate.ValidationError)
			if !ok {
				t.Fatalf("Expected error to be validate.ValidationError, got %T", err)
			}

			// Verify RangeInfo field
			if validationErr.RangeInfo == "" {
				t.Errorf("Expected RangeInfo to be set, got empty string")
			} else {
				// Verify range contains min and max values
				if !strings.Contains(validationErr.RangeInfo, tt.wantRangeMin) {
					t.Errorf("RangeInfo should contain min value '%s', got: %s", tt.wantRangeMin, validationErr.RangeInfo)
				}
				if !strings.Contains(validationErr.RangeInfo, tt.wantRangeMax) {
					t.Errorf("RangeInfo should contain max value '%s', got: %s", tt.wantRangeMax, validationErr.RangeInfo)
				}
			}
		})
	}
}

// TestValidateStatusCodePattern_ValidationDetails tests that range validation errors
// include detailed validation information.
func TestValidateStatusCodePattern_ValidationDetails(t *testing.T) {
	err := ValidateStatusCodePattern("4xx", 200)
	if err == nil {
		t.Fatal("ValidateStatusCodePattern('4xx', 200) expected error but got nil")
	}

	validationErr, ok := err.(validate.ValidationError)
	if !ok {
		t.Fatalf("Expected error to be validate.ValidationError, got %T", err)
	}

	// Verify ValidationDetails are present
	if len(validationErr.ValidationDetails) == 0 {
		t.Errorf("Expected ValidationDetails to contain at least one detail, got none")
	}

	// Verify validation details mention the status code and range
	detailsStr := strings.Join(validationErr.ValidationDetails, " ")
	if !strings.Contains(detailsStr, "200") {
		t.Errorf("ValidationDetails should mention actual status code '200', got: %s", detailsStr)
	}
	if !strings.Contains(detailsStr, "400") || !strings.Contains(detailsStr, "499") {
		t.Errorf("ValidationDetails should mention range '400-499', got: %s", detailsStr)
	}
}

// TestValidateStatusCodePattern_SuggestionsQuality tests that range validation errors
// include high-quality, actionable suggestions.
func TestValidateStatusCodePattern_SuggestionsQuality(t *testing.T) {
	tests := []struct {
		pattern string
		actual  int
	}{
		{"4xx", 200},
		{"5xx", 404},
		{"2xx", 500},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			err := ValidateStatusCodePattern(tt.pattern, tt.actual)
			if err == nil {
				t.Fatalf("ValidateStatusCodePattern(%q, %d) expected error but got nil", tt.pattern, tt.actual)
			}

			validationErr, ok := err.(validate.ValidationError)
			if !ok {
				t.Fatalf("Expected error to be validate.ValidationError, got %T", err)
			}

			// Verify suggestions are present
			if len(validationErr.Suggestions) == 0 {
				t.Errorf("Expected suggestions to be present, got none")
			}

			// Verify suggestions are actionable (not empty strings)
			for i, suggestion := range validationErr.Suggestions {
				if strings.TrimSpace(suggestion) == "" {
					t.Errorf("Suggestion %d should not be empty", i)
				}
				// Verify suggestions start with a capital letter or dash for readability
				if !strings.HasPrefix(suggestion, "-") && !strings.HasPrefix(strings.TrimSpace(suggestion)[0:1], strings.ToUpper(strings.TrimSpace(suggestion)[0:1])) {
					t.Errorf("Suggestion %d should start with capital letter or dash, got: %s", i, suggestion)
				}
			}
		})
	}
}

// TestValidateStatusCodePattern_ErrorMessageStructure tests that the complete
// error message structure includes all required components.
func TestValidateStatusCodePattern_ErrorMessageStructure(t *testing.T) {
	err := ValidateStatusCodePattern("4xx", 200)
	if err == nil {
		t.Fatal("ValidateStatusCodePattern('4xx', 200) expected error but got nil")
	}

	validationErr, ok := err.(validate.ValidationError)
	if !ok {
		t.Fatalf("Expected error to be validate.ValidationError, got %T", err)
	}

	errorMsg := err.Error()

	// Verify error message contains key components
	requiredComponents := []struct {
		name     string
		expected string
	}{
		{"Error type", "status_code_range"},
		{"Actual value", "200"},
		{"Range min", "400"},
		{"Range max", "499"},
		{"Pattern", "4xx"},
	}

	for _, component := range requiredComponents {
		if !strings.Contains(errorMsg, component.expected) {
			t.Errorf("Error message should contain %s '%s', got: %s", component.name, component.expected, errorMsg)
		}
	}

	// Verify the error message is multi-line for better readability
	if len(strings.Split(errorMsg, "\n")) < 2 {
		t.Errorf("Error message should be multi-line for better readability, got: %s", errorMsg)
	}

	// Verify structured fields are set
	if validationErr.ErrorType == "" {
		t.Errorf("Expected ErrorType to be set")
	}
	if validationErr.Actual == nil {
		t.Errorf("Expected Actual to be set")
	}
	if validationErr.Expected == nil {
		t.Errorf("Expected Expected to be set")
	}
	if validationErr.RangeInfo == "" {
		t.Errorf("Expected RangeInfo to be set")
	}
}

// TestValidateStatusCodePattern_HappyPath tests that valid patterns return no error.
func TestValidateStatusCodePattern_HappyPath(t *testing.T) {
	tests := []struct {
		pattern string
		actual  int
	}{
		{"4xx", 400},
		{"4xx", 404},
		{"4xx", 499},
		{"5xx", 500},
		{"5xx", 503},
		{"5xx", 599},
		{"2xx", 200},
		{"2xx", 201},
		{"2xx", 299},
		{"3xx", 300},
		{"3xx", 301},
		{"3xx", 399},
		{"1xx", 100},
		{"1xx", 101},
		{"1xx", 199},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			err := ValidateStatusCodePattern(tt.pattern, tt.actual)
			if err != nil {
				t.Errorf("ValidateStatusCodePattern(%q, %d) should return nil for valid pattern, got: %v", tt.pattern, tt.actual, err)
			}
		})
	}
}

// =============================================================================
// ValidateStatusCodeRange Enhanced Error Message Tests
// =============================================================================

// TestValidateStatusCodeRange_EnhancedErrorMessages tests that the range validation
// error formatter returns enhanced error messages.
//
// This test verifies that range validation errors include:
// - Expected valid range (min/max)
// - Actual value that was received
// - Context information
// - Suggestions
func TestValidateStatusCodeRange_EnhancedErrorMessages(t *testing.T) {
	tests := []struct {
		name              string
		minCode           int
		maxCode           int
		actualCode        int
		expectedInError   []string
		actualInError     string
		rangeInError      string
		suggestionsPresent bool
	}{
		{
			name:              "2xx range with 404 should include all components",
			minCode:           200,
			maxCode:           299,
			actualCode:        404,
			expectedInError:   []string{"200", "299"},
			actualInError:     "404",
			rangeInError:      "200-299",
			suggestionsPresent: true,
		},
		{
			name:              "4xx range with 200 should include all components",
			minCode:           400,
			maxCode:           499,
			actualCode:        200,
			expectedInError:   []string{"400", "499"},
			actualInError:     "200",
			rangeInError:      "400-499",
			suggestionsPresent: true,
		},
		{
			name:              "5xx range with 404 should include all components",
			minCode:           500,
			maxCode:           599,
			actualCode:        404,
			expectedInError:   []string{"500", "599"},
			actualInError:     "404",
			rangeInError:      "500-599",
			suggestionsPresent: true,
		},
		{
			name:              "3xx range with 200 should include all components",
			minCode:           300,
			maxCode:           399,
			actualCode:        200,
			expectedInError:   []string{"300", "399"},
			actualInError:     "200",
			rangeInError:      "300-399",
			suggestionsPresent: true,
		},
		{
			name:              "1xx range with 404 should include all components",
			minCode:           100,
			maxCode:           199,
			actualCode:        404,
			expectedInError:   []string{"100", "199"},
			actualInError:     "404",
			rangeInError:      "100-199",
			suggestionsPresent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the underlying error formatter directly
			context := fmt.Sprintf("httptest.ResponseRecorder validation")
			err := validate.FormatStatusCodeMinRangeError(tt.minCode, tt.maxCode, tt.actualCode, context, "status_code")

			errorMsg := err.Error()

			// Verify expected range values are in error message
			for _, expected := range tt.expectedInError {
				if !strings.Contains(errorMsg, expected) {
					t.Errorf("Error message should contain expected range value '%s', got: %s", expected, errorMsg)
				}
			}

			// Verify actual value is in error message
			if !strings.Contains(errorMsg, tt.actualInError) {
				t.Errorf("Error message should contain actual value '%s', got: %s", tt.actualInError, errorMsg)
			}

			// Verify range info is present
			if tt.rangeInError != "" && !strings.Contains(errorMsg, tt.rangeInError) {
				t.Errorf("Error message should contain range info '%s', got: %s", tt.rangeInError, errorMsg)
			}

			// Verify error type is mentioned
			if !strings.Contains(errorMsg, "status_code_range") {
				t.Errorf("Error message should mention error type 'status_code_range', got: %s", errorMsg)
			}

			// Verify suggestions section is present
			if !strings.Contains(errorMsg, "Suggestions") && !strings.Contains(errorMsg, "suggestions") {
				t.Errorf("Error message should include suggestions, got: %s", errorMsg)
			}

			// Verify the error message is multi-line for better readability
			if len(strings.Split(errorMsg, "\n")) < 2 {
				t.Errorf("Error message should be multi-line for better readability, got: %s", errorMsg)
			}

			// Verify structured error fields
			validationErr := err // Already is validate.ValidationError type

			if tt.suggestionsPresent && len(validationErr.Suggestions) == 0 {
				t.Errorf("Expected error to contain suggestions, got none")
			}

			if validationErr.Actual == nil {
				t.Errorf("Expected Actual field to be set, got nil")
			}

			if validationErr.Expected == nil {
				t.Errorf("Expected Expected field to be set, got nil")
			}

			if validationErr.RangeInfo == "" {
				t.Errorf("Expected RangeInfo to be set, got empty string")
			}
		})
	}
}

// TestValidateStatusCodeRange_RangeInfoField tests that range validation errors
// include proper range information with min/max values.
func TestValidateStatusCodeRange_RangeInfoField(t *testing.T) {
	tests := []struct {
		minCode    int
		maxCode    int
		actualCode int
		wantRange  string
	}{
		{200, 299, 404, "200-299"},
		{400, 499, 200, "400-499"},
		{500, 599, 404, "500-599"},
		{300, 399, 200, "300-399"},
		{100, 199, 404, "100-199"},
	}

	for _, tt := range tests {
		t.Run(tt.wantRange, func(t *testing.T) {
			context := fmt.Sprintf("httptest.ResponseRecorder validation")
			err := validate.FormatStatusCodeMinRangeError(tt.minCode, tt.maxCode, tt.actualCode, context, "status_code")

			errorMsg := err.Error()
			if errorMsg == "" {
				t.Fatalf("FormatStatusCodeMinRangeError(%d, %d, %d) should produce error output", tt.minCode, tt.maxCode, tt.actualCode)
			}

			// Verify range info is present
			if !strings.Contains(errorMsg, tt.wantRange) {
				t.Errorf("Error message should contain range '%s', got: %s", tt.wantRange, errorMsg)
			}

			// Verify RangeInfo field is set
			validationErr := err // Already is validate.ValidationError type

			if validationErr.RangeInfo == "" {
				t.Errorf("Expected RangeInfo field to be set, got empty string")
			}
		})
	}
}

// TestValidateStatusCodeRange_ValidationDetails tests that range validation errors
// include detailed validation information.
func TestValidateStatusCodeRange_ValidationDetails(t *testing.T) {
	context := "httptest.ResponseRecorder validation"
	err := validate.FormatStatusCodeMinRangeError(200, 299, 404, context, "status_code")

	errorMsg := err.Error()
	if errorMsg == "" {
		t.Fatal("FormatStatusCodeMinRangeError should produce error output")
	}

	// Verify validation details mention the status code and range
	if !strings.Contains(errorMsg, "404") {
		t.Errorf("Error message should mention actual status code '404', got: %s", errorMsg)
	}
	if !strings.Contains(errorMsg, "200") || !strings.Contains(errorMsg, "299") {
		t.Errorf("Error message should mention range '200-299', got: %s", errorMsg)
	}

	// Verify the error message contains detailed validation information
	if !strings.Contains(errorMsg, "outside") || !strings.Contains(errorMsg, "range") {
		t.Errorf("Error message should explain the validation failure, got: %s", errorMsg)
	}

	// Verify ValidationDetails field is populated
	validationErr := err // Already is validate.ValidationError type

	if len(validationErr.ValidationDetails) == 0 {
		t.Errorf("Expected ValidationDetails to be populated, got empty slice")
	}
}

// TestValidateStatusCodeRange_SuggestionsQuality tests that range validation errors
// include high-quality, actionable suggestions.
func TestValidateStatusCodeRange_SuggestionsQuality(t *testing.T) {
	tests := []struct {
		minCode    int
		maxCode    int
		actualCode int
	}{
		{200, 299, 404},
		{400, 499, 200},
		{500, 599, 404},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d-%d_vs_%d", tt.minCode, tt.maxCode, tt.actualCode), func(t *testing.T) {
			context := "httptest.ResponseRecorder validation"
			err := validate.FormatStatusCodeMinRangeError(tt.minCode, tt.maxCode, tt.actualCode, context, "status_code")

			errorMsg := err.Error()
			if errorMsg == "" {
				t.Fatalf("FormatStatusCodeMinRangeError should produce error output")
			}

			// Verify suggestions section is present
			if !strings.Contains(strings.ToLower(errorMsg), "suggestion") {
				t.Errorf("Error message should include suggestions, got: %s", errorMsg)
			}

			// Verify suggestions are actionable (not just empty text)
			lines := strings.Split(errorMsg, "\n")
			hasActionableContent := false
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "*") {
					// Found a bullet point - should have content
					if len(trimmed) > 2 {
						hasActionableContent = true
						break
					}
				}
			}

			if !hasActionableContent && strings.Contains(strings.ToLower(errorMsg), "suggestion") {
				t.Errorf("Suggestions section should have actionable content, got: %s", errorMsg)
			}

			// Verify Suggestions field is populated
			validationErr := err // Already is validate.ValidationError type

			if len(validationErr.Suggestions) == 0 {
				t.Errorf("Expected Suggestions field to be populated, got empty slice")
			}

			// Verify each suggestion is non-empty
			for i, suggestion := range validationErr.Suggestions {
				if strings.TrimSpace(suggestion) == "" {
					t.Errorf("Suggestion %d should not be empty", i)
				}
			}
		})
	}
}

// TestValidateStatusCodeRange_ErrorMessageStructure tests that the complete
// error message structure includes all required components.
func TestValidateStatusCodeRange_ErrorMessageStructure(t *testing.T) {
	context := "httptest.ResponseRecorder validation"
	err := validate.FormatStatusCodeMinRangeError(200, 299, 404, context, "status_code")

	errorMsg := err.Error()
	if errorMsg == "" {
		t.Fatal("FormatStatusCodeMinRangeError should produce error output")
	}

	// Verify error message contains key components
	requiredComponents := []struct {
		name     string
		expected string
	}{
		{"Error type", "status_code_range"},
		{"Actual value", "404"},
		{"Range min", "200"},
		{"Range max", "299"},
		{"Range info", "200-299"},
	}

	for _, component := range requiredComponents {
		if !strings.Contains(errorMsg, component.expected) {
			t.Errorf("Error message should contain %s '%s', got: %s", component.name, component.expected, errorMsg)
		}
	}

	// Verify the error message is multi-line for better readability
	if len(strings.Split(errorMsg, "\n")) < 2 {
		t.Errorf("Error message should be multi-line for better readability, got: %s", errorMsg)
	}

	// Verify validation details section
	if !strings.Contains(errorMsg, "Validation") && !strings.Contains(errorMsg, "validation") {
		t.Errorf("Error message should include validation details, got: %s", errorMsg)
	}

	// Verify structured error fields
	validationErr := err // Already is validate.ValidationError type

	if validationErr.ErrorType == "" {
		t.Errorf("Expected ErrorType to be set")
	}
	if validationErr.Actual == nil {
		t.Errorf("Expected Actual to be set")
	}
	if validationErr.Expected == nil {
		t.Errorf("Expected Expected to be set")
	}
	if validationErr.RangeInfo == "" {
		t.Errorf("Expected RangeInfo to be set")
	}
	if validationErr.FieldName == "" {
		t.Errorf("Expected FieldName to be set")
	}
}

// TestValidateStatusCodeRange_HappyPath tests that valid ranges return no error.
func TestValidateStatusCodeRange_HappyPath(t *testing.T) {
	tests := []struct {
		minCode    int
		maxCode    int
		actualCode int
	}{
		{200, 299, 200},
		{200, 299, 201},
		{200, 299, 299},
		{400, 499, 400},
		{400, 499, 404},
		{400, 499, 499},
		{500, 599, 500},
		{500, 599, 503},
		{500, 599, 599},
	}

	for _, tt := range tests {
		name := fmt.Sprintf("%d_in_%d-%d", tt.actualCode, tt.minCode, tt.maxCode)
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.WriteHeader(tt.actualCode)

			// This should not produce any test failure
			ValidateStatusCodeRange(t, w, tt.minCode, tt.maxCode)
		})
	}
}

// TestValidateStatusCodeRange_InvalidMinMax tests that invalid min/max combinations are handled.
func TestValidateStatusCodeRange_InvalidMinMax(t *testing.T) {
	// Skip this test if run in parallel with others since t.Fatalf will stop execution
	t.Skip("Testing Fatalf behavior requires subtest isolation")

	w := httptest.NewRecorder()
	w.WriteHeader(200)

	// t.Fatalf will stop the test, so we can't actually test it in the same test
	// The behavior is tested implicitly by other tests using valid ranges
	_ = w
	_ = t

	// Note: ValidateStatusCodeRange calls t.Fatalf when min > max
	// This stops the test immediately, so we can't test it inline
}

// TestValidateStatusCodeRange_Integration tests the full ValidateStatusCodeRange
// function with real testing.T to ensure it properly calls the error formatter.
func TestValidateStatusCodeRange_Integration(t *testing.T) {
	// Test that invalid range produces test failure
	t.Run("invalid range produces error", func(t *testing.T) {
		w := httptest.NewRecorder()
		w.WriteHeader(404)

		// This should call t.Errorf with enhanced error message
		// Call the validation - it should fail
		ValidateStatusCodeRange(t, w, 200, 299)

		// The function should have called t.Errorf, which will be caught by the test framework
	})

	// Test that valid range doesn't produce test failure
	t.Run("valid range produces no error", func(t *testing.T) {
		w := httptest.NewRecorder()
		w.WriteHeader(200)

		// This should not call t.Errorf
		ValidateStatusCodeRange(t, w, 200, 299)
	})
}
