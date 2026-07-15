package validate

import (
	"fmt"
	"strings"
	"testing"
)

// TestValidateErrorMessage_ErrorContent_PatternType tests that error messages show pattern type (regex vs substring)
func TestValidateErrorMessage_ErrorContent_PatternType(t *testing.T) {
	tests := []struct {
		name            string
		response        string
		expectedPattern string
		wantPatternType string
	}{
		{
			name:            "regex pattern with wildcard",
			response:        `{"error": "access_denied"}`,
			expectedPattern: "invalid.*token",
			wantPatternType: "regex",
		},
		{
			name:            "regex pattern with character class",
			response:        `{"error": "test"}`,
			expectedPattern: "[0-9]+",
			wantPatternType: "regex",
		},
		{
			name:            "substring pattern plain text",
			response:        `{"error": "Access Denied"}`,
			expectedPattern: "not found",
			wantPatternType: "substring",
		},
		{
			name:            "substring pattern with spaces",
			response:        `{"message": "test"}`,
			expectedPattern: "access denied",
			wantPatternType: "substring",
		},
		{
			name:            "regex pattern with alternation",
			response:        `{"error": "other"}`,
			expectedPattern: "invalid|expired",
			wantPatternType: "regex",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateErrorMessage([]byte(tt.response), tt.expectedPattern)
			if err == nil {
				t.Fatalf("ValidateErrorMessage() expected error, got nil")
			}

			errMsg := err.Error()
			if !containsSubstring(errMsg, tt.wantPatternType) {
				t.Errorf("ValidateErrorMessage() error should contain pattern type '%s', got: %s", tt.wantPatternType, errMsg)
			}

			// Also check for the full pattern type label
			patternTypeLabel := "Pattern type: " + tt.wantPatternType
			if !containsSubstring(errMsg, patternTypeLabel) {
				t.Errorf("ValidateErrorMessage() error should contain pattern type label '%s', got: %s", patternTypeLabel, errMsg)
			}
		})
	}
}

// TestValidateErrorMessage_ErrorContent_ExpectedPattern tests that error messages display expected pattern clearly
func TestValidateErrorMessage_ErrorContent_ExpectedPattern(t *testing.T) {
	tests := []struct {
		name            string
		response        string
		expectedPattern string
	}{
		{
			name:            "simple substring pattern",
			response:        `{"error": "test"}`,
			expectedPattern: "not found",
		},
		{
			name:            "regex pattern with wildcards",
			response:        `{"error": "test"}`,
			expectedPattern: "invalid.*token",
		},
		{
			name:            "complex regex pattern",
			response:        `{"error": "test"}`,
			expectedPattern: "^Error [0-9]{3}:.*$",
		},
		{
			name:            "pattern with spaces",
			response:        `{"error": "test"}`,
			expectedPattern: "access denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateErrorMessage([]byte(tt.response), tt.expectedPattern)
			if err == nil {
				t.Fatalf("ValidateErrorMessage() expected error, got nil")
			}

			errMsg := err.Error()
			expectedLabel := "Expected pattern: " + tt.expectedPattern
			if !containsSubstring(errMsg, expectedLabel) {
				t.Errorf("ValidateErrorMessage() error should contain '%s', got: %s", expectedLabel, errMsg)
			}
		})
	}
}

// TestValidateErrorMessage_ErrorContent_ActualMessage tests that error messages show actual error message found
func TestValidateErrorMessage_ErrorContent_ActualMessage(t *testing.T) {
	tests := []struct {
		name              string
		response          string
		expectedPattern   string
		wantActualMessage string
	}{
		{
			name:              "actual message from error field",
			response:          `{"error": "Resource not found"}`,
			expectedPattern:   "invalid",
			wantActualMessage: "Resource not found",
		},
		{
			name:              "actual message from message field",
			response:          `{"message": "Access denied"}`,
			expectedPattern:   "not found",
			wantActualMessage: "Access denied",
		},
		{
			name:              "actual message from detail field",
			response:          `{"detail": "Validation failed"}`,
			expectedPattern:   "expired",
			wantActualMessage: "Validation failed",
		},
		{
			name:              "actual message from nested error object",
			response:          `{"error": {"message": "Token expired"}}`,
			expectedPattern:   "invalid",
			wantActualMessage: "Token expired",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateErrorMessage([]byte(tt.response), tt.expectedPattern)
			if err == nil {
				t.Fatalf("ValidateErrorMessage() expected error, got nil")
			}

			errMsg := err.Error()
			actualMessageLabel := "Actual error message: \"" + tt.wantActualMessage + "\""
			if !containsSubstring(errMsg, actualMessageLabel) {
				t.Errorf("ValidateErrorMessage() error should contain '%s', got: %s", actualMessageLabel, errMsg)
			}
		})
	}
}

// TestValidateErrorMessage_ErrorContent_ResponseExcerpt tests that error messages include response body excerpt
func TestValidateErrorMessage_ErrorContent_ResponseExcerpt(t *testing.T) {
	tests := []struct {
		name                 string
		response             string
		expectedPattern      string
		excerptShouldContain string
		excerptMaxLength     int
	}{
		{
			name:                 "short response included in excerpt",
			response:             `{"error": "Access denied", "code": 403}`,
			expectedPattern:      "not found",
			excerptShouldContain: `{"error": "Access denied"`,
			excerptMaxLength:     200,
		},
		{
			name:                 "long response is truncated",
			response:             `{"error": "` + strings.Repeat("a", 300) + `"}`,
			expectedPattern:      "not found",
			excerptShouldContain: "...",
			excerptMaxLength:     200,
		},
		{
			name:                 "response with newlines is sanitized",
			response:             "{\n  \"error\": \"test\"\n}\n",
			expectedPattern:      "not found",
			excerptShouldContain: "{",
			excerptMaxLength:     200,
		},
		{
			name:                 "response with tabs is sanitized",
			response:             "{\t\"error\": \"test\"\t}",
			expectedPattern:      "invalid",
			excerptShouldContain: "{",
			excerptMaxLength:     200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateErrorMessage([]byte(tt.response), tt.expectedPattern)
			if err == nil {
				t.Fatalf("ValidateErrorMessage() expected error, got nil")
			}

			errMsg := err.Error()
			if !containsSubstring(errMsg, tt.excerptShouldContain) {
				t.Errorf("ValidateErrorMessage() error should contain excerpt '%s', got: %s", tt.excerptShouldContain, errMsg)
			}

			// Check that excerpt is properly labeled
			if !containsSubstring(errMsg, "Response:") {
				t.Errorf("ValidateErrorMessage() error should contain 'Response:' label, got: %s", errMsg)
			}
		})
	}
}

// TestValidateErrorMessage_ErrorContent_FieldName tests that error messages include field name where error was found
func TestValidateErrorMessage_ErrorContent_FieldName(t *testing.T) {
	tests := []struct {
		name            string
		response        string
		expectedPattern string
		wantFieldName   string
	}{
		{
			name:            "error field",
			response:        `{"error": "Not found"}`,
			expectedPattern: "invalid",
			wantFieldName:   "error",
		},
		{
			name:            "message field",
			response:        `{"message": "Access denied"}`,
			expectedPattern: "not found",
			wantFieldName:   "message",
		},
		{
			name:            "detail field",
			response:        `{"detail": "Validation failed"}`,
			expectedPattern: "expired",
			wantFieldName:   "detail",
		},
		{
			name:            "description field",
			response:        `{"description": "Resource unavailable"}`,
			expectedPattern: "invalid",
			wantFieldName:   "description",
		},
		{
			name:            "error_description field",
			response:        `{"error_description": "Token expired"}`,
			expectedPattern: "invalid",
			wantFieldName:   "error_description",
		},
		{
			name:            "nested error.message field",
			response:        `{"error": {"message": "Invalid token"}}`,
			expectedPattern: "not found",
			wantFieldName:   "error.message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateErrorMessage([]byte(tt.response), tt.expectedPattern)
			if err == nil {
				t.Fatalf("ValidateErrorMessage() expected error, got nil")
			}

			errMsg := err.Error()
			fieldNameLabel := "Field name: " + tt.wantFieldName
			if !containsSubstring(errMsg, fieldNameLabel) {
				t.Errorf("ValidateErrorMessage() error should contain '%s', got: %s", fieldNameLabel, errMsg)
			}
		})
	}
}

// TestValidateErrorMessage_ErrorContent_CheckedFields tests that error messages include context about which fields were searched
func TestValidateErrorMessage_ErrorContent_CheckedFields(t *testing.T) {
	tests := []struct {
		name              string
		response          string
		expectedPattern   string
		wantCheckedFields string
	}{
		{
			name:              "no error field found",
			response:          `{"status": "ok"}`,
			expectedPattern:   "error",
			wantCheckedFields: "error, message, detail, description, error_description",
		},
		{
			name:              "pattern mismatch in found field",
			response:          `{"error": "Access denied"}`,
			expectedPattern:   "not found",
			wantCheckedFields: "error, message, detail, description, error_description",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateErrorMessage([]byte(tt.response), tt.expectedPattern)
			if err == nil {
				t.Fatalf("ValidateErrorMessage() expected error, got nil")
			}

			errMsg := err.Error()
			checkedFieldsLabel := "Checked fields: " + tt.wantCheckedFields
			if !containsSubstring(errMsg, checkedFieldsLabel) {
				t.Errorf("ValidateErrorMessage() error should contain '%s', got: %s", checkedFieldsLabel, errMsg)
			}
		})
	}
}

// TestValidateErrorMessage_ErrorContent_Suggestions tests that error messages include suggestions for pattern mismatches
func TestValidateErrorMessage_ErrorContent_Suggestions(t *testing.T) {
	tests := []struct {
		name                     string
		response                 string
		expectedPattern          string
		suggestionsShouldContain []string
	}{
		{
			name:            "token vs auth suggestions",
			response:        `{"error": "Access denied"}`,
			expectedPattern: "invalid.*token",
			suggestionsShouldContain: []string{
				"token",
				"authentication",
			},
		},
		{
			name:            "not found vs does not exist suggestions",
			response:        `{"error": "Resource does not exist"}`,
			expectedPattern: "not found",
			suggestionsShouldContain: []string{
				"not found|does not exist",
			},
		},
		{
			name:            "invalid vs expired suggestions",
			response:        `{"error": "Token has expired"}`,
			expectedPattern: "invalid.*token",
			suggestionsShouldContain: []string{
				"invalid",
				"expired",
			},
		},
		{
			name:            "regex dot metacharacter suggestions",
			response:        `{"error": "access_denied"}`,
			expectedPattern: "accessXdenied", // Pattern with X that won't match underscore
			suggestionsShouldContain: []string{
				"Review the expected pattern",
			},
		},
		{
			name:            "regex quantifier suggestions",
			response:        `{"error": "test"}`,
			expectedPattern: "xyz+", // Pattern that won't match test
			suggestionsShouldContain: []string{
				"Review the expected pattern",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateErrorMessage([]byte(tt.response), tt.expectedPattern)
			if err == nil {
				t.Fatalf("ValidateErrorMessage() expected error, got nil")
			}

			errMsg := err.Error()

			// Check for Suggestions label
			if !containsSubstring(errMsg, "Suggestions:") {
				t.Errorf("ValidateErrorMessage() error should contain 'Suggestions:' label, got: %s", errMsg)
			}

			// Check for specific suggestions
			for _, expectedSuggestion := range tt.suggestionsShouldContain {
				if !containsSubstring(errMsg, expectedSuggestion) {
					t.Errorf("ValidateErrorMessage() error should contain suggestion '%s', got: %s", expectedSuggestion, errMsg)
				}
			}
		})
	}
}

// TestValidateErrorMessage_ErrorContent_FormattingHelper tests that FormatValidationErrorWithDetails is used consistently
func TestValidateErrorMessage_ErrorContent_FormattingHelper(t *testing.T) {
	tests := []struct {
		name            string
		response        string
		expectedPattern string
	}{
		{
			name:            "regex pattern mismatch",
			response:        `{"error": "access_denied"}`,
			expectedPattern: "invalid.*token",
		},
		{
			name:            "substring pattern mismatch",
			response:        `{"error": "Access Denied"}`,
			expectedPattern: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateErrorMessage([]byte(tt.response), tt.expectedPattern)
			if err == nil {
				t.Fatalf("ValidateErrorMessage() expected error, got nil")
			}

			// Check that the error is a ValidationError type
			validationErr, ok := err.(ValidationError)
			if !ok {
				t.Errorf("ValidateErrorMessage() should return ValidationError type, got %T", err)
			}

			// Check that ValidationError has consistent formatting
			if validationErr.ErrorType == "" {
				t.Errorf("ValidationError should have ErrorType set")
			}
			if validationErr.Message == "" {
				t.Errorf("ValidationError should have Message set")
			}
			if len(validationErr.ValidationDetails) == 0 {
				t.Errorf("ValidationError should have ValidationDetails")
			}
			if len(validationErr.Suggestions) == 0 {
				t.Errorf("ValidationError should have Suggestions")
			}
		})
	}
}

// TestValidateErrorMessage_ErrorContent_ResponseExcerptLength tests that response excerpts are properly truncated to 200 chars
func TestValidateErrorMessage_ErrorContent_ResponseExcerptLength(t *testing.T) {
	tests := []struct {
		name             string
		response         string
		expectedPattern  string
		maxExcerptLength int
	}{
		{
			name:             "response at exactly 200 chars",
			response:         `{"error": "` + strings.Repeat("a", 195) + `"}`,
			expectedPattern:  "not found",
			maxExcerptLength: 200,
		},
		{
			name:             "response over 200 chars is truncated",
			response:         `{"error": "` + strings.Repeat("a", 300) + `"}`,
			expectedPattern:  "not found",
			maxExcerptLength: 200,
		},
		{
			name:             "response under 200 chars is not truncated",
			response:         `{"error": "test"}`,
			expectedPattern:  "not found",
			maxExcerptLength: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateErrorMessage([]byte(tt.response), tt.expectedPattern)
			if err == nil {
				t.Fatalf("ValidateErrorMessage() expected error, got nil")
			}

			errMsg := err.Error()

			// Find the "Response:" section and check its length
			responseIdx := strings.Index(errMsg, "Response:")
			if responseIdx == -1 {
				t.Fatalf("ValidateErrorMessage() error should contain 'Response:' section")
			}

			// Extract the response excerpt (from "Response:" to the end of line or next section)
			responseStart := responseIdx + len("Response:")
			responseEnd := strings.Index(errMsg[responseStart:], "\n")
			if responseEnd == -1 {
				responseEnd = len(errMsg) - responseStart
			}

			responseExcerpt := strings.TrimSpace(errMsg[responseStart : responseStart+responseEnd])

			// Check that excerpt doesn't exceed max length (plus "..." for truncation)
			if len(responseExcerpt) > tt.maxExcerptLength+3 { // +3 for "..."
				t.Errorf("Response excerpt length %d exceeds max %d", len(responseExcerpt), tt.maxExcerptLength)
			}

			// If original response was longer than max, check for truncation indicator
			if len(tt.response) > tt.maxExcerptLength {
				if !strings.HasSuffix(responseExcerpt, "...") {
					t.Errorf("Long response should be truncated with '...', got: %s", responseExcerpt)
				}
			}
		})
	}
}

// TestValidateErrorMessage_ErrorContent_ComprehensiveIntegration tests all acceptance criteria together
func TestValidateErrorMessage_ErrorContent_ComprehensiveIntegration(t *testing.T) {
	response := `{"error": "Token has expired", "code": 401, "timestamp": "2024-01-01"}`
	pattern := "invalid.*token"

	err := ValidateErrorMessage([]byte(response), pattern)
	if err == nil {
		t.Fatalf("ValidateErrorMessage() expected error, got nil")
	}

	errMsg := err.Error()

	// Check all acceptance criteria
	requiredElements := map[string]string{
		"Pattern type":         "Pattern type: regex",
		"Expected pattern":     "Expected pattern: " + pattern,
		"Actual error message": "Actual error message: \"Token has expired\"",
		"Field name":           "Field name: error",
		"Checked fields":       "Checked fields:",
		"Response excerpt":     "Response:",
		"Suggestions":          "Suggestions:",
	}

	for elementName, elementContent := range requiredElements {
		if !containsSubstring(errMsg, elementContent) {
			t.Errorf("ValidateErrorMessage() error should contain %s '%s', got: %s", elementName, elementContent, errMsg)
		}
	}

	// Verify ValidationError type
	validationErr, ok := err.(ValidationError)
	if !ok {
		t.Errorf("ValidateErrorMessage() should return ValidationError type, got %T", err)
	}

	// Check ValidationError fields
	if validationErr.ErrorType != "error_message" {
		t.Errorf("ValidationError.ErrorType should be 'error_message', got '%s'", validationErr.ErrorType)
	}
	if validationErr.Expected != pattern {
		t.Errorf("ValidationError.Expected should be '%s', got '%v'", pattern, validationErr.Expected)
	}
	if validationErr.Actual != "Token has expired" {
		t.Errorf("ValidationError.Actual should be 'Token has expired', got '%v'", validationErr.Actual)
	}
	if validationErr.FieldName != "error" {
		t.Errorf("ValidationError.FieldName should be 'error', got '%s'", validationErr.FieldName)
	}
	if len(validationErr.ValidationDetails) == 0 {
		t.Errorf("ValidationError should have ValidationDetails")
	}
	if len(validationErr.Suggestions) == 0 {
		t.Errorf("ValidationError should have Suggestions")
	}
}

// TestValidateErrorMessage_ErrorContent_ResponseSizeContext tests that error messages include response size context
func TestValidateErrorMessage_ErrorContent_ResponseSizeContext(t *testing.T) {
	tests := []struct {
		name            string
		response        string
		expectedPattern string
		wantSizeLabel   bool
	}{
		{
			name:            "small response shows size in bytes",
			response:        `{"error": "Not found"}`,
			expectedPattern: "invalid",
			wantSizeLabel:   true,
		},
		{
			name:            "medium response shows size in bytes",
			response:        `{"error": "Access denied", "code": 403, "message": "User does not have permission"}`,
			expectedPattern: "not found",
			wantSizeLabel:   true,
		},
		{
			name:            "large response shows size in bytes",
			response:        `{"error": "` + strings.Repeat("a", 1000) + `"}`,
			expectedPattern: "not found",
			wantSizeLabel:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateErrorMessage([]byte(tt.response), tt.expectedPattern)
			if err == nil {
				t.Fatalf("ValidateErrorMessage() expected error, got nil")
			}

			errMsg := err.Error()

			// Check for Response size: label
			if !containsSubstring(errMsg, "Response size:") {
				t.Errorf("ValidateErrorMessage() error should contain 'Response size:' label, got: %s", errMsg)
			}

			// Check for "bytes" unit
			if !containsSubstring(errMsg, "bytes") {
				t.Errorf("ValidateErrorMessage() error should contain 'bytes' unit, got: %s", errMsg)
			}

			// Verify the actual size is included by checking it matches the response length
			expectedSize := len(tt.response)
			sizeLabel := fmt.Sprintf("Response size: %d bytes", expectedSize)
			if !containsSubstring(errMsg, sizeLabel) {
				t.Errorf("ValidateErrorMessage() error should contain '%s', got: %s", sizeLabel, errMsg)
			}

			// Verify ValidationError contains size in ValidationDetails
			validationErr, ok := err.(ValidationError)
			if !ok {
				t.Fatalf("ValidateErrorMessage() should return ValidationError type, got %T", err)
			}

			// Check that ValidationDetails contains size information
			foundSizeInDetails := false
			for _, detail := range validationErr.ValidationDetails {
				if containsSubstring(detail, "Response size:") && containsSubstring(detail, "bytes") {
					foundSizeInDetails = true
					break
				}
			}
			if !foundSizeInDetails {
				t.Errorf("ValidationError.ValidationDetails should contain response size information")
			}
		})
	}
}
