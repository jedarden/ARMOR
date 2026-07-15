package validate

import (
	"strings"
	"testing"
)

// TestValidateErrorMessage_ImplementationVerified tests that all acceptance criteria are met
func TestValidateErrorMessage_ImplementationVerified(t *testing.T) {
	response := []byte(`{"error": "Token has expired", "code": 401}`)
	pattern := "invalid.*token"

	err := ValidateErrorMessage(response, pattern)
	if err == nil {
		t.Fatalf("Expected error for pattern mismatch, got nil")
	}

	errMsg := err.Error()

	// Verify all acceptance criteria
	requiredElements := map[string]string{
		"Pattern type (regex vs substring)": "Pattern type: regex",
		"Expected pattern": "Expected pattern: " + pattern,
		"Actual error message": "Actual error message:",
		"Field name": "Field name: error",
		"Checked fields context": "Checked fields:",
		"Response excerpt": "Response:",
		"Suggestions": "Suggestions:",
	}

	for elementName, elementContent := range requiredElements {
		if !strings.Contains(errMsg, elementContent) {
			t.Errorf("Missing %s - error should contain '%s'", elementName, elementContent)
			t.Logf("Full error message:\n%s", errMsg)
		}
	}

	// Verify ValidationError type with all fields
	validationErr, ok := err.(ValidationError)
	if !ok {
		t.Fatalf("Error should be ValidationError type, got %T", err)
	}

	if validationErr.ErrorType != "error_message" {
		t.Errorf("Expected ErrorType 'error_message', got '%s'", validationErr.ErrorType)
	}

	if validationErr.FieldName != "error" {
		t.Errorf("Expected FieldName 'error', got '%s'", validationErr.FieldName)
	}

	if len(validationErr.ValidationDetails) == 0 {
		t.Error("Expected ValidationDetails to be populated")
	}

	if len(validationErr.Suggestions) == 0 {
		t.Error("Expected Suggestions to be populated")
	}
}

// TestValidateErrorMessage_ImplementationVerifiedSubstring tests substring pattern matching
func TestValidateErrorMessage_ImplementationVerifiedSubstring(t *testing.T) {
	response := []byte(`{"error": "Access Denied"}`)
	pattern := "not found"

	err := ValidateErrorMessage(response, pattern)
	if err == nil {
		t.Fatalf("Expected error for pattern mismatch, got nil")
	}

	errMsg := err.Error()

	// Verify pattern type is correctly identified as substring
	if !strings.Contains(errMsg, "Pattern type: substring") {
		t.Error("Pattern type should be 'substring'")
	}

	// Verify ValidationError type
	validationErr, ok := err.(ValidationError)
	if !ok {
		t.Fatalf("Error should be ValidationError type, got %T", err)
	}

	if validationErr.FieldName != "error" {
		t.Errorf("Expected FieldName 'error', got '%s'", validationErr.FieldName)
	}
}

// TestValidateErrorMessage_ResponseExcerptTruncation tests that long responses are properly truncated
func TestValidateErrorMessage_ResponseExcerptTruncation(t *testing.T) {
	// Create a response longer than 200 characters
	longResponse := `{"error": "` + strings.Repeat("a", 300) + `"}`
	
	err := ValidateErrorMessage([]byte(longResponse), "not found")
	if err == nil {
		t.Fatal("Expected error for pattern mismatch")
	}

	errMsg := err.Error()
	
	// Find the Response: section
	responseIdx := strings.Index(errMsg, "Response:")
	if responseIdx == -1 {
		t.Fatal("Error should contain 'Response:' section")
	}

	// Extract the response excerpt
	responseStart := responseIdx + len("Response:")
	responseEnd := strings.Index(errMsg[responseStart:], "\n")
	if responseEnd == -1 {
		responseEnd = len(errMsg) - responseStart
	}

	responseExcerpt := strings.TrimSpace(errMsg[responseStart : responseStart+responseEnd])

	// Check that excerpt doesn't exceed 200 characters + "..." for truncation
	if len(responseExcerpt) > 203 {
		t.Errorf("Response excerpt exceeds 203 characters (200 + ...), got %d", len(responseExcerpt))
	}

	// Verify truncation indicator is present
	if !strings.HasSuffix(responseExcerpt, "...") {
		t.Error("Long response should be truncated with '...' suffix")
	}
}

// TestValidateErrorMessage_RegexVsSubstringDetection tests automatic detection of regex vs substring patterns
func TestValidateErrorMessage_RegexVsSubstringDetection(t *testing.T) {
	tests := []struct {
		name            string
		pattern         string
		expectedPatternType string
	}{
		{
			name:            "regex with wildcard",
			pattern:         "invalid.*token",
			expectedPatternType: "regex",
		},
		{
			name:            "regex with alternation",
			pattern:         "invalid|expired",
			expectedPatternType: "regex",
		},
		{
			name:            "substring plain text",
			pattern:         "not found",
			expectedPatternType: "substring",
		},
		{
			name:            "substring with spaces",
			pattern:         "access denied",
			expectedPatternType: "substring",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := []byte(`{"error": "test"}`)
			err := ValidateErrorMessage(response, tt.pattern)
			if err == nil {
				t.Fatal("Expected error for pattern mismatch")
			}

			errMsg := err.Error()
			patternTypeLabel := "Pattern type: " + tt.expectedPatternType
			if !strings.Contains(errMsg, patternTypeLabel) {
				t.Errorf("Error should contain pattern type '%s'", patternTypeLabel)
				t.Logf("Full error message:\n%s", errMsg)
			}
		})
	}
}
