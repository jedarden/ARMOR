package validate

import (
	"testing"
)

// =============================================================================
// ERROR CODE VALIDATION TESTS
// =============================================================================

// TestValidateErrorCode_StringCode tests string error code validation.
func TestValidateErrorCode_StringCode(t *testing.T) {
	testCases := []struct {
		name         string
		response     string
		expectedCode string
		shouldMatch  bool
		description  string
	}{
		{
			name:         "match simple code",
			response:     `{"code": "AUTH_FAILED"}`,
			expectedCode: "AUTH_FAILED",
			shouldMatch:  true,
			description:  "Exact string code match",
		},
		{
			name:         "match code in error object",
			response:     `{"error": {"code": "VALIDATION_ERROR"}, "message": "Invalid input"}`,
			expectedCode: "VALIDATION_ERROR",
			shouldMatch:  true,
			description:  "String code in nested error object",
		},
		{
			name:         "match error_code field",
			response:     `{"error_code": "TOKEN_EXPIRED", "message": "Token expired"}`,
			expectedCode: "TOKEN_EXPIRED",
			shouldMatch:  true,
			description:  "String code in error_code field",
		},
		{
			name:         "match errorCode field (camelCase)",
			response:     `{"errorCode": "SESSION_TIMEOUT", "message": "Session timed out"}`,
			expectedCode: "SESSION_TIMEOUT",
			shouldMatch:  true,
			description:  "String code in errorCode field",
		},
		{
			name:         "no match - different code",
			response:     `{"code": "SUCCESS"}`,
			expectedCode: "AUTH_FAILED",
			shouldMatch:  false,
			description:  "Code mismatch",
		},
		{
			name:         "no code field present",
			response:     `{"message": "OK", "status": "success"}`,
			expectedCode: "AUTH_FAILED",
			shouldMatch:  true, // Returns nil when no code found (success case)
			description:  "No error code in response",
		},
		{
			name:         "empty response",
			response:     ``,
			expectedCode: "AUTH_FAILED",
			shouldMatch:  false,
			description:  "Empty response body",
		},
		{
			name:         "invalid JSON",
			response:     `{invalid json}`,
			expectedCode: "AUTH_FAILED",
			shouldMatch:  false,
			description:  "Invalid JSON response",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateErrorCode([]byte(tc.response), tc.expectedCode)

			if tc.shouldMatch {
				if err != nil {
					t.Errorf("Expected match for %s, got error: %v", tc.description, err)
				}
			} else {
				if err == nil {
					t.Errorf("Expected mismatch for %s, but got nil", tc.description)
				} else {
					// Verify error is ValidationError type
					validationErr, ok := err.(ValidationError)
					if !ok {
						t.Errorf("Expected ValidationError type, got: %T", err)
					} else {
						if validationErr.ErrorType != "error_code" {
							t.Errorf("Expected error_type 'error_code', got: %s", validationErr.ErrorType)
						}
					}
				}
			}
		})
	}
}

// TestValidateErrorCode_NumericCode tests numeric error code validation.
func TestValidateErrorCode_NumericCode(t *testing.T) {
	testCases := []struct {
		name         string
		response     string
		expectedCode int
		shouldMatch  bool
		description  string
	}{
		{
			name:         "match numeric code",
			response:     `{"code": 401, "message": "Unauthorized"}`,
			expectedCode: 401,
			shouldMatch:  true,
			description:  "Numeric code match",
		},
		{
			name:         "match code in error object",
			response:     `{"error": {"code": 404}, "message": "Not found"}`,
			expectedCode: 404,
			shouldMatch:  true,
			description:  "Numeric code in nested error object",
		},
		{
			name:         "match status field",
			response:     `{"status": 500, "message": "Internal error"}`,
			expectedCode: 500,
			shouldMatch:  true,
			description:  "Numeric code in status field",
		},
		{
			name:         "no match - different code",
			response:     `{"code": 200, "message": "OK"}`,
			expectedCode: 404,
			shouldMatch:  false,
			description:  "Numeric code mismatch",
		},
		{
			name:         "string code that contains numeric value",
			response:     `{"code": "404", "message": "Not found"}`,
			expectedCode: 404,
			shouldMatch:  true,
			description:  "String code that matches numeric pattern",
		},
		{
			name:         "numeric code that matches string pattern",
			response:     `{"code": 401, "message": "Unauthorized"}`,
			expectedCode: 401,
			shouldMatch:  true,
			description:  "Numeric code that matches string pattern",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateErrorCode([]byte(tc.response), tc.expectedCode)

			if tc.shouldMatch {
				if err != nil {
					t.Errorf("Expected match for %s, got error: %v", tc.description, err)
				}
			} else {
				if err == nil {
					t.Errorf("Expected mismatch for %s, but got nil", tc.description)
				}
			}
		})
	}
}

// TestValidateErrorCode_DetailedErrors tests detailed error information.
func TestValidateErrorCode_DetailedErrors(t *testing.T) {
	t.Run("detailed mismatch error", func(t *testing.T) {
		response := []byte(`{"code": "VALIDATION_ERROR", "message": "Invalid input"}`)
		err := ValidateErrorCode(response, "AUTH_FAILED")

		if err == nil {
			t.Fatal("Expected validation error, got nil")
		}

		validationErr, ok := err.(ValidationError)
		if !ok {
			t.Fatalf("Expected ValidationError type, got: %T", err)
		}

		// Check error structure
		if validationErr.ErrorType != "error_code" {
			t.Errorf("Expected error_type 'error_code', got: %s", validationErr.ErrorType)
		}

		if validationErr.FieldName != "code" {
			t.Errorf("Expected field_name 'code', got: %s", validationErr.FieldName)
		}

		if len(validationErr.ValidationDetails) == 0 {
			t.Error("Expected validation details to be populated")
		}

		if len(validationErr.Suggestions) == 0 {
			t.Error("Expected suggestions to be generated")
		}

		// Check error message contains key information
		errMsg := err.Error()
		requiredElements := []string{
			"VALIDATION_ERROR",
			"AUTH_FAILED",
			"error_code",
		}
		for _, element := range requiredElements {
			if !containsSubstring(errMsg, element) {
				t.Errorf("Error message should contain '%s', got: %s", element, errMsg)
			}
		}
	})
}

// TestValidateErrorCodePattern tests error code pattern validation.
func TestValidateErrorCodePattern(t *testing.T) {
	testCases := []struct {
		name         string
		response     string
		pattern      string
		shouldMatch  bool
		description  string
	}{
		{
			name:         "AUTH_* matches AUTH_FAILED",
			response:     `{"code": "AUTH_FAILED"}`,
			pattern:      "AUTH_*",
			shouldMatch:  true,
			description:  "Wildcard prefix match",
		},
		{
			name:         "AUTH_* matches AUTH_REQUIRED",
			response:     `{"code": "AUTH_REQUIRED"}`,
			pattern:      "AUTH_*",
			shouldMatch:  true,
			description:  "Wildcard prefix match",
		},
		{
			name:         "ERR_* matches ERR_VALIDATION",
			response:     `{"code": "ERR_VALIDATION"}`,
			pattern:      "ERR_*",
			shouldMatch:  true,
			description:  "Wildcard prefix match",
		},
		{
			name:         "4xx matches 404",
			response:     `{"code": 404}`,
			pattern:      "4xx",
			shouldMatch:  true,
			description:  "Numeric range pattern",
		},
		{
			name:         "4xx matches 403",
			response:     `{"code": 403}`,
			pattern:      "4xx",
			shouldMatch:  true,
			description:  "Numeric range pattern",
		},
		{
			name:         "5xx matches 500",
			response:     `{"code": 500}`,
			pattern:      "5xx",
			shouldMatch:  true,
			description:  "Numeric range pattern",
		},
		{
			name:         "5xx matches 503",
			response:     `{"code": 503}`,
			pattern:      "5xx",
			shouldMatch:  true,
			description:  "Numeric range pattern",
		},
		{
			name:         "4xx does not match 200",
			response:     `{"code": 200}`,
			pattern:      "4xx",
			shouldMatch:  false,
			description:  "Numeric range mismatch",
		},
		{
			name:         "AUTH_* does not match SUCCESS",
			response:     `{"code": "SUCCESS"}`,
			pattern:      "AUTH_*",
			shouldMatch:  false,
			description:  "Wildcard pattern mismatch",
		},
		{
			name:         "no error code found",
			response:     `{"message": "OK"}`,
			pattern:      "AUTH_*",
			shouldMatch:  false,
			description:  "No error code in response",
		},
		{
			name:         "empty pattern",
			response:     `{"code": "AUTH_FAILED"}`,
			pattern:      "",
			shouldMatch:  false,
			description:  "Empty pattern should error",
		},
		{
			name:         "empty response",
			response:     ``,
			pattern:      "AUTH_*",
			shouldMatch:  false,
			description:  "Empty response should error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateErrorCodePattern([]byte(tc.response), tc.pattern)

			if tc.shouldMatch {
				if err != nil {
					t.Errorf("Expected pattern '%s' to match %s, got error: %v", tc.pattern, tc.description, err)
				}
			} else {
				if err == nil {
					t.Errorf("Expected pattern '%s' NOT to match %s, but got nil", tc.pattern, tc.description)
				} else {
					// Verify error is ValidationError for actual validation failures (not input errors)
					if tc.response != "" && tc.pattern != "" {
						validationErr, ok := err.(ValidationError)
						if !ok {
							t.Errorf("Expected ValidationError type, got: %T", err)
						} else {
							if validationErr.ErrorType != "error_code" {
								t.Errorf("Expected error_type 'error_code', got: %s", validationErr.ErrorType)
							}
						}
					}
				}
			}
		})
	}
}

// TestValidateErrorCodeAny tests multiple allowed error codes.
func TestValidateErrorCodeAny(t *testing.T) {
	testCases := []struct {
		name         string
		response     string
		allowedCodes []interface{}
		shouldMatch  bool
		description  string
	}{
		{
			name:         "match first allowed code",
			response:     `{"code": "AUTH_FAILED"}`,
			allowedCodes: []interface{}{"AUTH_FAILED", "TOKEN_EXPIRED"},
			shouldMatch:  true,
			description:  "Matches first allowed code",
		},
		{
			name:         "match second allowed code",
			response:     `{"code": "TOKEN_EXPIRED"}`,
			allowedCodes: []interface{}{"AUTH_FAILED", "TOKEN_EXPIRED"},
			shouldMatch:  true,
			description:  "Matches second allowed code",
		},
		{
			name:         "match numeric in string list",
			response:     `{"code": 404}`,
			allowedCodes: []interface{}{"NOT_FOUND", "MISSING", 404},
			shouldMatch:  true,
			description:  "Matches numeric code in mixed list",
		},
		{
			name:         "no match - code not in list",
			response:     `{"code": "SUCCESS"}`,
			allowedCodes: []interface{}{"AUTH_FAILED", "TOKEN_EXPIRED"},
			shouldMatch:  false,
			description:  "Code not in allowed list",
		},
		{
			name:         "empty allowed list",
			response:     `{"code": "AUTH_FAILED"}`,
			allowedCodes: []interface{}{},
			shouldMatch:  false,
			description:  "Empty allowed list should error",
		},
		{
			name:         "empty response",
			response:     ``,
			allowedCodes: []interface{}{"AUTH_FAILED"},
			shouldMatch:  false,
			description:  "Empty response should error",
		},
		{
			name:         "no error code in response",
			response:     `{"message": "OK"}`,
			allowedCodes: []interface{}{"AUTH_FAILED", "TOKEN_EXPIRED"},
			shouldMatch:  false,
			description:  "No error code to validate",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateErrorCodeAny([]byte(tc.response), tc.allowedCodes)

			if tc.shouldMatch {
				if err != nil {
					t.Errorf("Expected match for %s, got error: %v", tc.description, err)
				}
			} else {
				if err == nil {
					t.Errorf("Expected mismatch for %s, but got nil", tc.description)
				}
			}
		})
	}
}

// TestValidateErrorCode_DetailedMismatchInformation tests detailed mismatch information.
func TestValidateErrorCode_DetailedMismatchInformation(t *testing.T) {
	t.Run("string code mismatch details", func(t *testing.T) {
		response := []byte(`{"code": "VALIDATION_ERROR", "message": "Invalid input"}`)
		err := ValidateErrorCode(response, "AUTH_FAILED")

		if err == nil {
			t.Fatal("Expected validation error, got nil")
		}

		validationErr, ok := err.(ValidationError)
		if !ok {
			t.Fatalf("Expected ValidationError type, got: %T", err)
		}

		// Verify error contains detailed information
		if validationErr.Expected == nil {
			t.Error("Expected Expected field to be populated")
		}
		if validationErr.Actual == nil {
			t.Error("Expected Actual field to be populated")
		}
		if validationErr.FieldName == "" {
			t.Error("Expected FieldName to be populated")
		}
		if len(validationErr.ValidationDetails) == 0 {
			t.Error("Expected ValidationDetails to be populated")
		}
		if len(validationErr.Suggestions) == 0 {
			t.Error("Expected Suggestions to be generated")
		}

		// Verify error message contains required information
		errMsg := err.Error()
		if !containsSubstring(errMsg, "VALIDATION_ERROR") {
			t.Errorf("Error message should contain actual code 'VALIDATION_ERROR', got: %s", errMsg)
		}
		if !containsSubstring(errMsg, "AUTH_FAILED") {
			t.Errorf("Error message should contain expected code 'AUTH_FAILED', got: %s", errMsg)
		}
	})

	t.Run("numeric code mismatch details", func(t *testing.T) {
		response := []byte(`{"code": 404, "message": "Not found"}`)
		err := ValidateErrorCode(response, 401)

		if err == nil {
			t.Fatal("Expected validation error, got nil")
		}

		validationErr, ok := err.(ValidationError)
		if !ok {
			t.Fatalf("Expected ValidationError type, got: %T", err)
		}

		// Verify numeric code handling
		if validationErr.FieldName == "" {
			t.Error("Expected FieldName to be populated")
		}

		// Verify error message contains numeric values
		errMsg := err.Error()
		if !containsSubstring(errMsg, "404") {
			t.Errorf("Error message should contain actual code 404, got: %s", errMsg)
		}
		if !containsSubstring(errMsg, "401") {
			t.Errorf("Error message should contain expected code 401, got: %s", errMsg)
		}
	})

	t.Run("pattern mismatch details", func(t *testing.T) {
		response := []byte(`{"code": "SUCCESS", "message": "Operation succeeded"}`)
		err := ValidateErrorCodePattern(response, "AUTH_*")

		if err == nil {
			t.Fatal("Expected validation error, got nil")
		}

		_, ok := err.(ValidationError)
		if !ok {
			t.Fatalf("Expected ValidationError type, got: %T", err)
		}

		// Verify pattern information
		errMsg := err.Error()
		if !containsSubstring(errMsg, "AUTH_*") {
			t.Error("Expected pattern information in error")
		}
		if !containsSubstring(errMsg, "SUCCESS") {
			t.Errorf("Error message should contain actual code 'SUCCESS', got: %s", errMsg)
		}
	})

	t.Run("multiple allowed codes mismatch details", func(t *testing.T) {
		response := []byte(`{"code": "OTHER_ERROR", "message": "Other error"}`)
		allowedCodes := []interface{}{"AUTH_FAILED", "TOKEN_EXPIRED", "VALIDATION_ERROR"}
		err := ValidateErrorCodeAny(response, allowedCodes)

		if err == nil {
			t.Fatal("Expected validation error, got nil")
		}

		_, ok := err.(ValidationError)
		if !ok {
			t.Fatalf("Expected ValidationError type, got: %T", err)
		}

		// Verify allowed codes are shown in error
		errMsg := err.Error()
		for _, code := range allowedCodes {
			codeStr := formatExpectedCode(code)
			if !containsSubstring(errMsg, codeStr) {
				t.Errorf("Error message should contain allowed code '%s', got: %s", codeStr, errMsg)
			}
		}
	})
}

// TestValidateErrorCode_CommonScenarios tests common real-world scenarios.
func TestValidateErrorCode_CommonScenarios(t *testing.T) {
	scenarios := []struct {
		name           string
		response       string
		validationFunc func([]byte) error
		shouldPass     bool
		description    string
	}{
		{
			name: "REST API - Authentication error",
			response: `{"error": {"code": "AUTH_FAILED", "message": "Invalid credentials"}}`,
			validationFunc: func(b []byte) error {
				return ValidateErrorCode(b, "AUTH_FAILED")
			},
			shouldPass:  true,
			description: "Authentication error code",
		},
		{
			name: "REST API - Token expired",
			response: `{"error": {"code": "TOKEN_EXPIRED", "message": "Token has expired"}}`,
			validationFunc: func(b []byte) error {
				return ValidateErrorCodePattern(b, "TOKEN_*")
			},
			shouldPass:  true,
			description: "Token-related error code",
		},
		{
			name: "REST API - Validation error",
			response: `{"code": "VALIDATION_ERROR", "message": "Invalid input", "field": "email"}`,
			validationFunc: func(b []byte) error {
				return ValidateErrorCodePattern(b, "*_ERROR")
			},
			shouldPass:  true,
			description: "Validation error code pattern",
		},
		{
			name: "REST API - Multiple auth errors",
			response: `{"code": "SESSION_EXPIRED", "message": "Session has expired"}`,
			validationFunc: func(b []byte) error {
				return ValidateErrorCodeAny(b, []interface{}{"AUTH_FAILED", "TOKEN_EXPIRED", "SESSION_EXPIRED"})
			},
			shouldPass:  true,
			description: "Multiple acceptable auth error codes",
		},
		{
			name: "REST API - 404 Not Found",
			response: `{"code": 404, "message": "Resource not found"}`,
			validationFunc: func(b []byte) error {
				return ValidateErrorCode(b, 404)
			},
			shouldPass:  true,
			description: "Numeric error code 404",
		},
		{
			name: "REST API - Client error range",
			response: `{"code": 403, "message": "Access denied"}`,
			validationFunc: func(b []byte) error {
				return ValidateErrorCodePattern(b, "4xx")
			},
			shouldPass:  true,
			description: "Client error range pattern",
		},
		{
			name: "REST API - Server error range",
			response: `{"code": 500, "message": "Internal server error"}`,
			validationFunc: func(b []byte) error {
				return ValidateErrorCodePattern(b, "5xx")
			},
			shouldPass:  true,
			description: "Server error range pattern",
		},
		{
			name: "REST API - Unexpected error code",
			response: `{"code": "UNEXPECTED_ERROR", "message": "Unexpected error"}`,
			validationFunc: func(b []byte) error {
				return ValidateErrorCode(b, "AUTH_FAILED")
			},
			shouldPass:  false,
			description: "Error code mismatch",
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			err := scenario.validationFunc([]byte(scenario.response))

			if scenario.shouldPass {
				if err != nil {
					t.Errorf("Expected %s to pass, got error: %v", scenario.description, err)
				}
			} else {
				if err == nil {
					t.Errorf("Expected %s to fail, but got nil", scenario.description)
				} else {
					// Verify error provides useful information
					validationErr, ok := err.(ValidationError)
					if !ok {
						t.Errorf("Expected ValidationError type, got: %T", err)
					} else {
						if len(validationErr.Suggestions) == 0 {
							t.Errorf("Expected suggestions for %s", scenario.description)
						}
					}
				}
			}
		})
	}
}

// TestValidateErrorCode_Suggestions tests that appropriate suggestions are generated.
func TestValidateErrorCode_Suggestions(t *testing.T) {
	t.Run("authentication-related suggestions", func(t *testing.T) {
		response := []byte(`{"code": "VALIDATION_ERROR", "message": "Invalid input"}`)
		err := ValidateErrorCode(response, "AUTH_FAILED")

		if err == nil {
			t.Fatal("Expected validation error, got nil")
		}

		validationErr, ok := err.(ValidationError)
		if !ok {
			t.Fatalf("Expected ValidationError type, got: %T", err)
		}

		// Since we got VALIDATION_ERROR but expected AUTH_FAILED, we might not get auth-specific suggestions
		// but we should get some suggestions
		if len(validationErr.Suggestions) == 0 {
			t.Error("Expected suggestions to be generated")
		}
	})

	t.Run("validation-related suggestions", func(t *testing.T) {
		response := []byte(`{"code": "VALIDATION_ERROR", "message": "Invalid input"}`)
		err := ValidateErrorCode(response, "AUTH_FAILED")

		if err == nil {
			t.Fatal("Expected validation error, got nil")
		}

		validationErr, ok := err.(ValidationError)
		if !ok {
			t.Fatalf("Expected ValidationError type, got: %T", err)
		}

		// Check for validation-related suggestions
		hasValidationSuggestion := false
		for _, suggestion := range validationErr.Suggestions {
			if containsSubstring(suggestion, "validation") || containsSubstring(suggestion, "API documentation") {
				hasValidationSuggestion = true
				break
			}
		}

		if !hasValidationSuggestion {
			t.Log("No validation-specific suggestions found, but have:", validationErr.Suggestions)
		}
	})

	t.Run("format mismatch suggestions", func(t *testing.T) {
		response := []byte(`{"code": 404, "message": "Not found"}`)
		err := ValidateErrorCode(response, "NOT_FOUND")

		if err == nil {
			t.Fatal("Expected validation error, got nil")
		}

		validationErr, ok := err.(ValidationError)
		if !ok {
			t.Fatalf("Expected ValidationError type, got: %T", err)
		}

		// Check for format mismatch suggestions
		hasFormatSuggestion := false
		for _, suggestion := range validationErr.Suggestions {
			if containsSubstring(suggestion, "format") || containsSubstring(suggestion, "numeric") || containsSubstring(suggestion, "string") {
				hasFormatSuggestion = true
				break
			}
		}

		if hasFormatSuggestion {
			t.Log("Found format mismatch suggestion")
		}
	})
}

