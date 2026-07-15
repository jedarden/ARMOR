package validate

import (
	"strings"
	"testing"
)

// TestFormatValidationErrorWithDetails_BasicUsage tests basic functionality with auto-generated suggestions.
func TestFormatValidationErrorWithDetails_BasicUsage(t *testing.T) {
	ve := FormatValidationErrorWithDetails(
		"status_code",
		200,
		404,
		"GET /api/users/123",
		"",
		"",
		"",
		nil,
		"",
		"",
		nil,
	)

	// Verify core fields
	if ve.ErrorType != "status_code" {
		t.Errorf("Expected ErrorType 'status_code', got '%s'", ve.ErrorType)
	}
	if ve.Expected != 200 {
		t.Errorf("Expected Expected 200, got %v", ve.Expected)
	}
	if ve.Actual != 404 {
		t.Errorf("Expected Actual 404, got %v", ve.Actual)
	}
	if ve.Context != "GET /api/users/123" {
		t.Errorf("Expected Context 'GET /api/users/123', got '%s'", ve.Context)
	}

	// Verify suggestions were auto-generated
	if len(ve.Suggestions) == 0 {
		t.Error("Expected auto-generated suggestions, got none")
	}

	// Verify message was generated
	if ve.Message == "" {
		t.Error("Expected auto-generated message, got empty string")
	}
}

// TestFormatValidationErrorWithDetails_WithAllFields tests with all optional fields populated.
func TestFormatValidationErrorWithDetails_WithAllFields(t *testing.T) {
	ve := FormatValidationErrorWithDetails(
		"error_message",
		"invalid.*token",
		"access_denied",
		"OAuth validation",
		`{"error": "access_denied"}`,
		"error",
		"field 'user.access_token'",
		[]string{"user.access_token", "user.refresh_token"},
		"regex pattern 'invalid.*token' did not match",
		"",
		[]string{
			"No matching error field found",
			"Checked 3 error message fields",
		},
	)

	// Verify all fields are populated
	if ve.FieldName != "error" {
		t.Errorf("Expected FieldName 'error', got '%s'", ve.FieldName)
	}
	if ve.Location != "field 'user.access_token'" {
		t.Errorf("Expected Location 'field 'user.access_token'', got '%s'", ve.Location)
	}
	if len(ve.RelatedFields) != 2 {
		t.Errorf("Expected 2 RelatedFields, got %d", len(ve.RelatedFields))
	}
	if ve.PatternDetails != "regex pattern 'invalid.*token' did not match" {
		t.Errorf("Expected PatternDetails 'regex pattern 'invalid.*token' did not match', got '%s'", ve.PatternDetails)
	}
	if len(ve.ValidationDetails) != 2 {
		t.Errorf("Expected 2 ValidationDetails, got %d", len(ve.ValidationDetails))
	}
}

// TestFormatValidationErrorWithDetails_WithRangeInfo tests with range information.
func TestFormatValidationErrorWithDetails_WithRangeInfo(t *testing.T) {
	ve := FormatValidationErrorWithDetails(
		"status_code_range",
		"4xx",
		200,
		"Error response check",
		"",
		"",
		"",
		nil,
		"",
		"400-499 (Client Error)",
		[]string{"Status code 200 is outside expected range"},
	)

	if ve.RangeInfo != "400-499 (Client Error)" {
		t.Errorf("Expected RangeInfo '400-499 (Client Error)', got '%s'", ve.RangeInfo)
	}
	if len(ve.ValidationDetails) != 1 {
		t.Errorf("Expected 1 ValidationDetail, got %d", len(ve.ValidationDetails))
	}
}

// TestFormatValidationErrorWithDetails_CustomSuggestionsOverride tests that custom suggestions override auto-generated ones.
func TestFormatValidationErrorWithDetails_CustomSuggestionsOverride(t *testing.T) {
	customSuggestion1 := "Set Content-Type header to application/json"
	customSuggestion2 := "Ensure request body is valid JSON"

	ve := FormatValidationErrorWithDetails(
		"content_type",
		"application/json",
		"text/html",
		"POST /api/users",
		`{"status": "error"}`,
		"Content-Type",
		"",
		nil,
		"",
		"",
		nil,
		customSuggestion1,
		customSuggestion2,
	)

	// Verify custom suggestions were used
	if len(ve.Suggestions) != 2 {
		t.Errorf("Expected 2 custom suggestions, got %d", len(ve.Suggestions))
	}
	if ve.Suggestions[0] != customSuggestion1 {
		t.Errorf("Expected first suggestion '%s', got '%s'", customSuggestion1, ve.Suggestions[0])
	}
	if ve.Suggestions[1] != customSuggestion2 {
		t.Errorf("Expected second suggestion '%s', got '%s'", customSuggestion2, ve.Suggestions[1])
	}
}

// TestFormatValidationErrorWithDetails_PatternDetails tests pattern details field.
func TestFormatValidationErrorWithDetails_PatternDetails(t *testing.T) {
	patternDetails := "Expected pattern: ^User .* not found$, Actual: Invalid credentials"

	ve := FormatValidationErrorWithDetails(
		"error_message",
		"^User .* not found$",
		"Invalid credentials",
		"User lookup validation",
		`{"error": "Invalid credentials"}`,
		"message",
		"line 42",
		nil,
		patternDetails,
		"",
		nil,
	)

	if ve.PatternDetails != patternDetails {
		t.Errorf("Expected PatternDetails '%s', got '%s'", patternDetails, ve.PatternDetails)
	}
	if ve.Location != "line 42" {
		t.Errorf("Expected Location 'line 42', got '%s'", ve.Location)
	}
}

// TestFormatValidationErrorWithDetails_ResponseSnippet tests response snippet field.
func TestFormatValidationErrorWithDetails_ResponseSnippet(t *testing.T) {
	snippet := `{"error": "Invalid credentials", "code": "AUTH_FAILED"}`

	ve := FormatValidationErrorWithDetails(
		"error_message",
		"success",
		"error",
		"API response validation",
		snippet,
		"",
		"",
		nil,
		"",
		"",
		nil,
	)

	if ve.ResponseSnippet != snippet {
		t.Errorf("Expected ResponseSnippet '%s', got '%s'", snippet, ve.ResponseSnippet)
	}
}

// TestFormatValidationErrorWithDetails_RelatedFields tests related fields array.
func TestFormatValidationErrorWithDetails_RelatedFields(t *testing.T) {
	relatedFields := []string{"access_token", "refresh_token", "token_type"}

	ve := FormatValidationErrorWithDetails(
		"cors_headers",
		"Bearer token",
		"missing",
		"OAuth token validation",
		"",
		"Authorization",
		"",
		relatedFields,
		"",
		"",
		nil,
	)

	if len(ve.RelatedFields) != 3 {
		t.Errorf("Expected 3 RelatedFields, got %d", len(ve.RelatedFields))
	}
	for i, field := range relatedFields {
		if ve.RelatedFields[i] != field {
			t.Errorf("Expected RelatedFields[%d] '%s', got '%s'", i, field, ve.RelatedFields[i])
		}
	}
}

// TestFormatValidationErrorWithDetails_StatusCodeSuggestionsContext tests that status code suggestions are context-aware.
func TestFormatValidationErrorWithDetails_StatusCodeSuggestionsContext(t *testing.T) {
	testCases := []struct {
		name           string
		actual         int
		expectedInSug []string
	}{
		{
			name:   "404 suggestions",
			actual: 404,
			expectedInSug: []string{
				"Verify the endpoint URL is correct",
				"Check if the resource ID or identifier exists",
			},
		},
		{
			name:   "401 suggestions",
			actual: 401,
			expectedInSug: []string{
				"Verify authentication credentials are correct",
				"Check if API token or session has expired",
			},
		},
		{
			name:   "403 suggestions",
			actual: 403,
			expectedInSug: []string{
				"Verify your account has permission to access this resource",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ve := FormatValidationErrorWithDetails(
				"status_code",
				200,
				tc.actual,
				"API test",
				"",
				"",
				"",
				nil,
				"",
				"",
				nil,
			)

			// Check that expected suggestions are present
			for _, expectedSug := range tc.expectedInSug {
				found := false
				for _, sug := range ve.Suggestions {
					if strings.Contains(sug, expectedSug) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected suggestion containing '%s', got suggestions: %v", expectedSug, ve.Suggestions)
				}
			}
		})
	}
}

// TestFormatValidationErrorWithDetails_ErrorMessageSuggestions tests error message pattern-based suggestions.
func TestFormatValidationErrorWithDetails_ErrorMessageSuggestions(t *testing.T) {
	ve := FormatValidationErrorWithDetails(
		"error_message",
		"invalid.*token",
		"token expired",
		"OAuth validation",
		"",
		"",
		"",
		nil,
		"",
		"",
		nil,
	)

	// Verify suggestions were generated for error message validation
	if len(ve.Suggestions) == 0 {
		t.Error("Expected suggestions for error_message validation, got none")
	}

	// The suggestions should be relevant to token errors
	suggestionsText := strings.Join(ve.Suggestions, " ")
	hasRelevantSuggestion := strings.Contains(suggestionsText, "token") ||
		strings.Contains(suggestionsText, "Token")

	if !hasRelevantSuggestion {
		t.Errorf("Expected token-relevant suggestions, got: %v", ve.Suggestions)
	}
}

// TestFormatValidationErrorWithDetails_ValidationDetailsArray tests validation details array functionality.
func TestFormatValidationErrorWithDetails_ValidationDetailsArray(t *testing.T) {
	details := []string{
		"Step 1: Checked error field existence",
		"Step 2: Validated error message format",
		"Step 3: Verified error code presence",
		"Result: Pattern mismatch found",
	}

	ve := FormatValidationErrorWithDetails(
		"error_message",
		"expected.*pattern",
		"actual message",
		"Multi-step validation",
		"",
		"",
		"",
		nil,
		"",
		"",
		details,
	)

	if len(ve.ValidationDetails) != len(details) {
		t.Errorf("Expected %d ValidationDetails, got %d", len(details), len(ve.ValidationDetails))
	}

	for i, detail := range details {
		if ve.ValidationDetails[i] != detail {
			t.Errorf("Expected ValidationDetails[%d] '%s', got '%s'", i, detail, ve.ValidationDetails[i])
		}
	}
}

// TestFormatValidationErrorWithDetails_NilParameters tests with nil optional parameters.
func TestFormatValidationErrorWithDetails_NilParameters(t *testing.T) {
	ve := FormatValidationErrorWithDetails(
		"status_code",
		200,
		404,
		"",
		"",
		"",
		"",
		nil,
		"",
		"",
		nil,
	)

	// Verify nil fields remain empty
	if ve.Context != "" {
		t.Errorf("Expected empty Context, got '%s'", ve.Context)
	}
	if ve.ResponseSnippet != "" {
		t.Errorf("Expected empty ResponseSnippet, got '%s'", ve.ResponseSnippet)
	}
	if ve.FieldName != "" {
		t.Errorf("Expected empty FieldName, got '%s'", ve.FieldName)
	}
	if ve.Location != "" {
		t.Errorf("Expected empty Location, got '%s'", ve.Location)
	}
	if len(ve.RelatedFields) != 0 {
		t.Errorf("Expected empty RelatedFields, got %v", ve.RelatedFields)
	}
	if ve.PatternDetails != "" {
		t.Errorf("Expected empty PatternDetails, got '%s'", ve.PatternDetails)
	}
	if ve.RangeInfo != "" {
		t.Errorf("Expected empty RangeInfo, got '%s'", ve.RangeInfo)
	}
	if len(ve.ValidationDetails) != 0 {
		t.Errorf("Expected empty ValidationDetails, got %v", ve.ValidationDetails)
	}

	// Suggestions should still be auto-generated
	if len(ve.Suggestions) == 0 {
		t.Error("Expected auto-generated suggestions even with nil parameters")
	}
}

// TestFormatValidationErrorWithDetails_EmptyStringParameters tests with empty string optional parameters.
func TestFormatValidationErrorWithDetails_EmptyStringParameters(t *testing.T) {
	ve := FormatValidationErrorWithDetails(
		"status_code",
		200,
		404,
		"",
		"",
		"",
		"",
		nil,
		"",
		"",
		nil,
	)

	// Verify empty string fields
	if ve.Context != "" {
		t.Errorf("Expected empty Context, got '%s'", ve.Context)
	}
	if ve.PatternDetails != "" {
		t.Errorf("Expected empty PatternDetails, got '%s'", ve.PatternDetails)
	}
	if ve.RangeInfo != "" {
		t.Errorf("Expected empty RangeInfo, got '%s'", ve.RangeInfo)
	}
}

// TestFormatValidationErrorWithDetails_ComprehensiveExample tests a comprehensive real-world example.
func TestFormatValidationErrorWithDetails_ComprehensiveExample(t *testing.T) {
	// Simulate a complex OAuth token validation failure
	ve := FormatValidationErrorWithDetails(
		"error_message",
		"invalid.*token",
		"access_denied",
		"OAuth token validation during user authentication",
		`{"error": "access_denied", "error_description": "The access token is expired or invalid"}`,
		"error",
		"line 42 in authenticate_user function",
		[]string{"access_token", "refresh_token", "token_type", "expires_in"},
		"Expected pattern 'invalid.*token' or 'expired.*token', but got 'access_denied'",
		"",
		[]string{
			"Pattern mismatch indicates different error type than expected",
			"Expected token validation error, got authorization error instead",
			"This suggests the token format is correct but lacks permissions",
			"Check OAuth scopes and permissions for the access token",
		},
	)

	// Verify comprehensive field population
	if ve.ErrorType != "error_message" {
		t.Errorf("Expected ErrorType 'error_message', got '%s'", ve.ErrorType)
	}
	if ve.Context != "OAuth token validation during user authentication" {
		t.Errorf("Expected comprehensive context, got '%s'", ve.Context)
	}
	if !strings.Contains(ve.ResponseSnippet, "access_denied") {
		t.Errorf("Expected response snippet to contain 'access_denied', got '%s'", ve.ResponseSnippet)
	}
	if ve.FieldName != "error" {
		t.Errorf("Expected FieldName 'error', got '%s'", ve.FieldName)
	}
	if !strings.Contains(ve.Location, "line 42") {
		t.Errorf("Expected location to mention line 42, got '%s'", ve.Location)
	}
	if len(ve.RelatedFields) != 4 {
		t.Errorf("Expected 4 RelatedFields, got %d", len(ve.RelatedFields))
	}
	if !strings.Contains(ve.PatternDetails, "Expected pattern") && !strings.Contains(ve.PatternDetails, "invalid.*token") {
		t.Errorf("Expected pattern details to contain expected pattern info, got '%s'", ve.PatternDetails)
	}
	if len(ve.ValidationDetails) != 4 {
		t.Errorf("Expected 4 ValidationDetails, got %d", len(ve.ValidationDetails))
	}

	// Verify detailed validation messages
	expectedDetail := "Pattern mismatch indicates different error type than expected"
	found := false
	for _, detail := range ve.ValidationDetails {
		if detail == expectedDetail {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected validation detail '%s' not found in: %v", expectedDetail, ve.ValidationDetails)
	}
}
