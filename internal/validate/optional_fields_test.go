package validate

import (
	"strings"
	"testing"
)

// TestOptionalFields_ComprehensiveError tests that all optional fields work together
// and are truly optional (backward compatible)
func TestOptionalFields_ComprehensiveError(t *testing.T) {
	// Test with ALL optional fields populated
	fullError := FormatValidationErrorWithDetails(
		"error_message",
		"invalid.*token",
		"access_denied",
		"OAuth token validation",
		`{"error": "access_denied"}`,
		"error",
		"line 42 in user configuration", // location
		[]string{"token_type", "access_token"}, // relatedFields
		"regex pattern 'invalid.*token' did not match", // patternDetails
		"", // rangeInfo (empty - truly optional)
		[]string{"No matching error field found", "Checked 3 error message fields"}, // validationDetails
		"Check token format", "Verify OAuth scopes", // customSuggestions
	)

	fullMsg := fullError.Error()

	// Verify all populated fields appear in output
	requiredStrings := []string{
		"error_message validation failed",
		"invalid.*token",
		"access_denied",
		"OAuth token validation",
		"Field:",
		"error",
		"Location:",
		"line 42",
		"Related fields:",
		"token_type",
		"access_token",
		"Pattern:",
		"regex pattern",
		"Details:",
		"No matching error field found",
		"Checked 3 error message fields",
		"Check token format",
		"Verify OAuth scopes",
	}

	for _, required := range requiredStrings {
		if !strings.Contains(fullMsg, required) {
			t.Errorf("Expected full error to contain '%s', got:\n%s", required, fullMsg)
		}
	}

	// Verify rangeInfo does NOT appear since it was empty
	if strings.Contains(fullMsg, "Range:") {
		t.Errorf("Range field should not appear when empty, got:\n%s", fullMsg)
	}

	// Test with MINIMAL fields (backward compatibility)
	minimalError := FormatValidationError("custom_validation", "expected", "actual", "", "")
	minimalMsg := minimalError.Error()

	// Verify basic structure is present
	basicRequired := []string{
		"custom_validation validation failed",
		"Expected: expected",
		"Actual:   actual",
	}

	for _, required := range basicRequired {
		if !strings.Contains(minimalMsg, required) {
			t.Errorf("Expected minimal error to contain '%s', got:\n%s", required, minimalMsg)
		}
	}

	// Verify optional fields do NOT appear when not provided
	optionalFieldsThatShouldNotAppear := []string{
		"Context:",
		"Field:",
		"Location:",
		"Related fields:",
		"Pattern:",
		"Range:",
		"Details:",
		"Response:",
	}

	for _, shouldNotAppear := range optionalFieldsThatShouldNotAppear {
		if strings.Contains(minimalMsg, shouldNotAppear) {
			t.Errorf("Optional field '%s' should not appear in minimal error, got:\n%s", shouldNotAppear, minimalMsg)
		}
	}
}

// TestOptionalFields_ExpectedVsActual tests the expected vs actual value display
func TestOptionalFields_ExpectedVsActual(t *testing.T) {
	tests := []struct {
		name     string
		expected interface{}
		actual   interface{}
		validate func(t *testing.T, msg string)
	}{
		{
			name:     "integer status codes",
			expected: 200,
			actual:   404,
			validate: func(t *testing.T, msg string) {
				if !strings.Contains(msg, "Expected: 200") || !strings.Contains(msg, "Actual:   404") {
					t.Errorf("Integer format incorrect, got:\n%s", msg)
				}
			},
		},
		{
			name:     "multiple expected status codes",
			expected: []int{200, 201, 204},
			actual:   500,
			validate: func(t *testing.T, msg string) {
				if !strings.Contains(msg, "one of") || !strings.Contains(msg, "200") || !strings.Contains(msg, "201") {
					t.Errorf("Multiple codes format incorrect, got:\n%s", msg)
				}
			},
		},
		{
			name:     "regex pattern vs string",
			expected: "invalid.*token",
			actual:   "access_denied",
			validate: func(t *testing.T, msg string) {
				if !strings.Contains(msg, "Expected: invalid.*token") || !strings.Contains(msg, "Actual:   access_denied") {
					t.Errorf("String format incorrect, got:\n%s", msg)
				}
			},
		},
		{
			name:     "content type comparison",
			expected: "application/json",
			actual:   "text/html",
			validate: func(t *testing.T, msg string) {
				if !strings.Contains(msg, "application/json") || !strings.Contains(msg, "text/html") {
					t.Errorf("Content-Type format incorrect, got:\n%s", msg)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := FormatValidationError("test", tt.expected, tt.actual, "", "")
			tt.validate(t, err.Error())
		})
	}
}

// TestOptionalFields_ErrorContext tests error context including location and related fields
func TestOptionalFields_ErrorContext(t *testing.T) {
	tests := []struct {
		name          string
		location      string
		relatedFields []string
		validate      func(t *testing.T, msg string)
	}{
		{
			name:          "line number location",
			location:      "line 42",
			relatedFields: nil,
			validate: func(t *testing.T, msg string) {
				if !strings.Contains(msg, "Location:") || !strings.Contains(msg, "line 42") {
					t.Errorf("Location not properly formatted, got:\n%s", msg)
				}
			},
		},
		{
			name:          "field path location",
			location:      "field 'user.email'",
			relatedFields: nil,
			validate: func(t *testing.T, msg string) {
				if !strings.Contains(msg, "field 'user.email'") {
					t.Errorf("Field path location not shown, got:\n%s", msg)
				}
			},
		},
		{
			name:          "position location",
			location:      "position 123",
			relatedFields: nil,
			validate: func(t *testing.T, msg string) {
				if !strings.Contains(msg, "position 123") {
					t.Errorf("Position location not shown, got:\n%s", msg)
				}
			},
		},
		{
			name:          "related fields",
			location:      "",
			relatedFields: []string{"email", "email_confirmation"},
			validate: func(t *testing.T, msg string) {
				if !strings.Contains(msg, "Related fields:") {
					t.Errorf("Related fields section missing, got:\n%s", msg)
				}
				if !strings.Contains(msg, "email") || !strings.Contains(msg, "email_confirmation") {
					t.Errorf("Related fields not listed, got:\n%s", msg)
				}
			},
		},
		{
			name:          "location and related fields together",
			location:      "field 'user.password'",
			relatedFields: []string{"password", "password_confirmation"},
			validate: func(t *testing.T, msg string) {
				if !strings.Contains(msg, "Location: field 'user.password'") {
					t.Errorf("Location not shown with related fields, got:\n%s", msg)
				}
				if !strings.Contains(msg, "Related fields: [password, password_confirmation]") {
					t.Errorf("Related fields not properly formatted, got:\n%s", msg)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := FormatValidationErrorWithDetails(
				"test",
				"expected",
				"actual",
				"", // context
				"", // responseSnippet
				"", // fieldName
				tt.location,
				tt.relatedFields,
				"", // patternDetails
				"", // rangeInfo
				nil, // validationDetails
			)
			tt.validate(t, err.Error())
		})
	}
}

// TestOptionalFields_CustomSuggestions tests optional suggestion messages
func TestOptionalFields_CustomSuggestions(t *testing.T) {
	tests := []struct {
		name              string
		customSuggestions []string
		validate          func(t *testing.T, msg string, suggestions []string)
	}{
		{
			name:              "single custom suggestion",
			customSuggestions: []string{"Check the API documentation"},
			validate: func(t *testing.T, msg string, suggestions []string) {
				if !strings.Contains(msg, "Check the API documentation") {
					t.Errorf("Custom suggestion not shown, got:\n%s", msg)
				}
			},
		},
		{
			name:              "multiple custom suggestions",
			customSuggestions: []string{
				"Check the API documentation",
				"Verify the endpoint URL",
				"Review authentication credentials",
			},
			validate: func(t *testing.T, msg string, suggestions []string) {
				for _, suggestion := range suggestions {
					if !strings.Contains(msg, suggestion) {
						t.Errorf("Custom suggestion '%s' not shown, got:\n%s", suggestion, msg)
					}
				}
			},
		},
		{
			name:              "no custom suggestions (auto-generated)",
			customSuggestions: nil,
			validate: func(t *testing.T, msg string, suggestions []string) {
				// Should have auto-generated suggestions for status_code validation
				if !strings.Contains(msg, "Suggestions:") {
					t.Errorf("Suggestions section missing when auto-generating, got:\n%s", msg)
				}
				// Should not have custom suggestions
				if strings.Contains(msg, "Check the API documentation") {
					t.Errorf("Unexpected custom suggestion in auto-generated mode, got:\n%s", msg)
				}
			},
		},
		{
			name:              "empty custom suggestions (auto-generated)",
			customSuggestions: []string{},
			validate: func(t *testing.T, msg string, suggestions []string) {
				// Empty slice should trigger auto-generation
				if !strings.Contains(msg, "Suggestions:") {
					t.Errorf("Suggestions section missing with empty custom suggestions, got:\n%s", msg)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := FormatValidationError("status_code", 200, 404, "GET /api/users", "", tt.customSuggestions...)
			tt.validate(t, err.Error(), tt.customSuggestions)
		})
	}
}

// TestOptionalFields_BackwardCompatibility tests that the implementation is
// backward compatible with existing code
func TestOptionalFields_BackwardCompatibility(t *testing.T) {
	// Test that old code using the basic FormatValidationError still works
	err1 := FormatValidationError("status_code", 200, 404, "GET /api/users", "")
	if err1.Error() == "" {
		t.Error("Basic FormatValidationError should work")
	}

	// Test that ValidationFormatter works without setting optional fields
	err2 := NewValidationFormatter("test").
		WithExpected("expected").
		WithActual("actual").
		Format()

	if err2.Error() == "" {
		t.Error("ValidationFormatter without optional fields should work")
	}

	// Test that ValidationError struct can be created directly with minimal fields
	ve := ValidationError{
		ErrorType: "test",
		Expected:       "expected",
		Actual:         "actual",
	}

	if ve.Error() == "" {
		t.Error("Direct ValidationError construction should work")
	}

	// Test that convenience functions still work
	err3 := FormatStatusCodeError(200, 404, "GET /api/users")
	if err3.Error() == "" {
		t.Error("FormatStatusCodeError should still work")
	}

	err4 := FormatErrorMessageError("invalid.*token", "access_denied", "error", "OAuth validation")
	if err4.Error() == "" {
		t.Error("FormatErrorMessageError should still work")
	}

	err5 := FormatContentTypeError("application/json", "text/html", "API response")
	if err5.Error() == "" {
		t.Error("FormatContentTypeError should still work")
	}
}
