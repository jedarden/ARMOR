package validate

import (
	"fmt"
	"strings"
	"testing"
)

// TestErrorOutputExamples demonstrates the actual error output format for various error types.
// This test serves as both documentation and verification of error message clarity.
//
// The examples show:
// 1. Complete error output format with all fields
// 2. Auto-generated suggestions based on error type and values
// 3. Context-aware recommendations
// 4. Actionable and specific error messages
// 5. Proper formatting and structure
//
// Each example displays the actual error output and verifies its quality.
func TestErrorOutputExamples(t *testing.T) {
	errorCases := []struct {
		name        string
		err         ValidationError
		description string
		showExample bool
	}{
		{
			name: "404 Not Found with context",
			err: NewValidationFormatter("status_code").
				WithExpected(200).
				WithActual(404).
				WithContext("GET /api/users/123").
				Format(),
			description: "Shows clear resource not found error with actionable suggestions",
			showExample: true,
		},
		{
			name: "401 Unauthorized with OAuth context",
			err: NewValidationFormatter("status_code").
				WithExpected(200).
				WithActual(401).
				WithContext("OAuth token validation").
				Format(),
			description: "Shows authentication failure with token-related suggestions",
			showExample: true,
		},
		{
			name: "403 Forbidden with permission context",
			err: NewValidationFormatter("status_code").
				WithExpected(200).
				WithActual(403).
				WithContext("Resource access attempt").
				Format(),
			description: "Shows authorization failure with permission-related suggestions",
			showExample: true,
		},
		{
			name: "400 Bad Request with POST context",
			err: NewValidationFormatter("status_code").
				WithExpected(201).
				WithActual(400).
				WithContext("POST /api/users").
				Format(),
			description: "Shows request validation failure with body format suggestions",
			showExample: true,
		},
		{
			name: "Content Type Mismatch - JSON vs HTML",
			err: NewValidationFormatter("content_type").
				WithExpected("application/json").
				WithActual("text/html").
				WithContext("API response").
				Format(),
			description: "Shows content-type validation failure with debugging suggestions",
			showExample: true,
		},
		{
			name: "Content Type Mismatch - JSON vs XML",
			err: FormatContentTypeError("application/json", "application/xml", "API response"),
			description: "Shows format negotiation issue with Accept header suggestions",
			showExample: true,
		},
		{
			name: "Error Message Pattern Match - Token Expired",
			err: NewValidationFormatter("error_message").
				WithExpected("valid.*token").
				WithActual("token expired").
				WithFieldName("error").
				WithContext("Authentication check").
				WithPatternDetails("regex pattern 'valid.*token' did not match").
				Format(),
			description: "Shows error message validation with pattern-specific suggestions",
			showExample: true,
		},
		{
			name: "Error Message Pattern Match - Invalid Credentials",
			err: FormatErrorMessageError("success", "invalid credentials", "error", "User authentication"),
			description: "Shows authentication error with credential verification suggestions",
			showExample: true,
		},
		{
			name: "Rate Limit Error - 429",
			err: NewValidationFormatter("status_code").
				WithExpected(200).
				WithActual(429).
				WithContext("API rate limiting").
				Format(),
			description: "Shows rate limiting error with backoff and quota suggestions",
			showExample: true,
		},
		{
			name: "Timeout Error",
			err: NewValidationFormatter("timeout").
				WithExpected("< 5s").
				WithActual("30s timeout").
				WithContext("Database query").
				Format(),
			description: "Shows timeout validation with retry and backoff suggestions",
			showExample: true,
		},
		{
			name: "JSON Schema Validation",
			err: NewValidationFormatter("json_schema").
				WithExpected("valid user object").
				WithActual("missing required field 'email'").
				WithContext("User creation validation").
				Format(),
			description: "Shows schema validation with field-specific suggestions",
			showExample: true,
		},
		{
			name: "Status Code Range Error - Expected 4xx got 2xx",
			err: FormatStatusCodeRangeError("4xx", 200, "Error response check", "status_code"),
			description: "Shows range validation with category-specific suggestions",
			showExample: true,
		},
		{
			name: "Status Code Range Error - Expected 2xx got 5xx",
			err: FormatStatusCodeMinRangeError(200, 299, 500, "Success response check", "status_code"),
			description: "Shows range validation with server error suggestions",
			showExample: true,
		},
		{
			name: "CORS Headers Missing",
			err: NewValidationFormatter("cors_headers").
				WithExpected("CORS headers present").
				WithActual("CORS headers missing").
				WithContext("Cross-origin request").
				Format(),
			description: "Shows CORS validation with header configuration suggestions",
			showExample: true,
		},
		{
			name: "Custom Error with All Fields",
			err: NewValidationFormatter("custom_validation").
				WithExpected("expected_value").
				WithActual("actual_value").
				WithContext("Custom validation context").
				WithFieldName("custom_field").
				WithPatternDetails("Custom pattern mismatch").
				WithValidationDetails("Detail 1: First issue", "Detail 2: Second issue").
				WithResponseSnippet(`{"custom": "response snippet"}`).
				WithRangeInfo("1-100").
				Format(),
			description: "Shows comprehensive custom error with all fields populated",
			showExample: true,
		},
		{
			name: "500 Internal Server Error",
			err: FormatStatusCodeError(200, 500, "GET /api/data"),
			description: "Shows server error with retry and backoff suggestions",
			showExample: true,
		},
		{
			name: "502 Bad Gateway",
			err: NewValidationFormatter("status_code").
				WithExpected(200).
				WithActual(502).
				WithContext("API gateway request").
				Format(),
			description: "Shows gateway error with upstream service suggestions",
			showExample: true,
		},
		{
			name: "503 Service Unavailable",
			err: FormatStatusCodeError(200, 503, "API request"),
			description: "Shows service unavailability with maintenance and retry suggestions",
			showExample: true,
		},
	}

	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			// Display the error output for manual review and documentation
			if tc.showExample {
				fmt.Printf("\n╔═══════════════════════════════════════════════════════════════════════════╗\n")
				fmt.Printf("║ %s - %s\n", tc.name, tc.description)
				fmt.Printf("╚═══════════════════════════════════════════════════════════════════════════╝\n")
				fmt.Println(tc.err.Error())
				fmt.Println()

				// Show error structure details
				fmt.Printf("Error Type: %s\n", tc.err.ErrorType)
				fmt.Printf("Message: %s\n", tc.err.Message)
				if tc.err.Context != "" {
					fmt.Printf("Context: %s\n", tc.err.Context)
				}
				if tc.err.FieldName != "" {
					fmt.Printf("Field: %s\n", tc.err.FieldName)
				}
				if len(tc.err.ValidationDetails) > 0 {
					fmt.Printf("Validation Details: %v\n", tc.err.ValidationDetails)
				}
				if tc.err.PatternDetails != "" {
					fmt.Printf("Pattern Details: %s\n", tc.err.PatternDetails)
				}
				if tc.err.RangeInfo != "" {
					fmt.Printf("Range Info: %s\n", tc.err.RangeInfo)
				}
				fmt.Printf("Suggestions Count: %d\n", len(tc.err.Suggestions))
				fmt.Println("─────────────────────────────────────────────────────────────────────────────")
			}

			// Verify basic error properties
			if len(tc.err.Suggestions) == 0 {
				t.Errorf("Expected suggestions to be generated for %s", tc.name)
			}

			// Verify error message contains essential information
			errorOutput := tc.err.Error()
			requiredElements := []string{"validation failed", "Expected", "Common causes"}

			// Status code validation uses "Received:" instead of "Actual:"
			// and "Request:" instead of "Context:"
			if tc.err.ErrorType == "status_code" {
				requiredElements = append(requiredElements, "Received")
			} else {
				requiredElements = append(requiredElements, "Actual")
			}

			for _, element := range requiredElements {
				if !strings.Contains(errorOutput, element) {
					t.Errorf("Error output should contain '%s' for %s", element, tc.name)
				}
			}

			// Verify error type is in output
			if !contains(errorOutput, tc.err.ErrorType) {
				t.Errorf("Error output should contain error type '%s' for %s", tc.err.ErrorType, tc.name)
			}

			// Verify suggestions are actionable (contain action verbs)
			hasActionableSuggestion := false
			for _, suggestion := range tc.err.Suggestions {
				if containsActionWords(suggestion) {
					hasActionableSuggestion = true
					break
				}
			}
			if !hasActionableSuggestion {
				t.Errorf("Expected at least one actionable suggestion for %s", tc.name)
			}

			// Verify suggestions are properly formatted in error output
			if len(tc.err.Suggestions) > 0 {
				hasNumberedSuggestions := false
				for i, suggestion := range tc.err.Suggestions {
					// Check for numbered format like "1. suggestion"
					if contains(errorOutput, fmt.Sprintf("%d. %s", i+1, suggestion)) {
						hasNumberedSuggestions = true
						break
					}
				}
				if !hasNumberedSuggestions {
					t.Errorf("Expected suggestions to be numbered in error output for %s", tc.name)
				}
			}

			// Verify suggestions are not too long (max 150 chars each)
			for i, suggestion := range tc.err.Suggestions {
				if len(suggestion) > 150 {
					t.Errorf("Suggestion %d too long (%d chars) for %s: %s", i, len(suggestion), tc.name, suggestion)
				}
			}

			// Verify suggestions are not vague
			vaguePhrases := []string{"something", "thing", "somehow", "maybe"}
			for i, suggestion := range tc.err.Suggestions {
				for _, phrase := range vaguePhrases {
					if contains(toLower(suggestion), phrase) {
						t.Errorf("Suggestion %d contains vague phrase '%s' for %s: %s", i, phrase, tc.name, suggestion)
					}
				}
			}
		})
	}
}



// containsActionWords checks if a suggestion contains actionable words
func containsActionWords(suggestion string) bool {
	actionWords := []string{
		"verify", "check", "ensure", "review", "validate", "confirm",
		"implement", "add", "update", "use", "consider", "try",
		"contact", "increase", "reduce", "test", "verify",
	}

	lowerSuggestion := toLower(suggestion)
	for _, word := range actionWords {
		if strings.Contains(lowerSuggestion, toLower(word)) {
			return true
		}
	}
	return false
}

// toLower converts a string to lowercase
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c = c + 32
		}
		result[i] = c
	}
	return string(result)
}

/*
═══════════════════════════════════════════════════════════════════════════════════════════
ERROR MESSAGE FORMAT DOCUMENTATION
═══════════════════════════════════════════════════════════════════════════════════════════

The ARMOR validation system produces detailed, actionable error messages with the following format:

┌──────────────────────────────────────────────────────────────────────────────────────┐
│ ERROR TYPE MESSAGE                                                                 │
│   Expected: <expected_value>                                                       │
│   Actual:   <actual_value>                                                         │
│   Context:  <context_description>                                                  │
│   Field:    <field_name>                                                           │
│   Suggestions:                                                                     │
│     - <actionable_suggestion_1>                                                    │
│     - <actionable_suggestion_2>                                                    │
│     - <actionable_suggestion_3>                                                    │
└──────────────────────────────────────────────────────────────────────────────────────┘

KEY FEATURES:
═══════════════════════════════════════════════════════════════════════════════════════════

1. CLEAR ERROR TYPE IDENTIFICATION
   - Shows what type of validation failed (status_code, error_message, content_type, etc.)
   - Helps quickly identify the category of error

2. EXPECTED vs ACTUAL VALUES
   - Clearly shows what was expected vs what was actually received
   - Supports multiple value types (int, string, []int, etc.)
   - Displays in a readable, comparative format

3. CONTEXTUAL INFORMATION
   - Shows where/when the validation occurred (endpoint, operation, etc.)
   - Helps locate the source of the validation failure
   - Provides debugging context

4. FIELD-SPECIFIC DETAILS
   - Identifies the specific field that failed validation
   - Shows nested field paths (user.email, data.items[0].id)
   - Helps pinpoint exact validation location

5. ACTIONABLE SUGGESTIONS
   - Auto-generated based on error type and values
   - Context-aware recommendations
   - Specific, actionable steps to resolve the issue
   - Ordered from most to least likely solution

6. VALIDATION DETAILS (when applicable)
   - Additional debugging information
   - Pattern matching details
   - Range information
   - Response snippets for debugging

USAGE EXAMPLES:
═══════════════════════════════════════════════════════════════════════════════════════════

Example 1: Basic Status Code Validation
────────────────────────────────────────────────────────────────────────────────────────
	err := validate.FormatStatusCodeError(200, 404, "GET /api/users/123")
	log.Printf("Validation failed: %v", err)

Output:
	status_code validation failed
	  Expected: 200
	  Actual:   404
	  Context:  GET /api/users/123
	  Suggestions:
	    - Verify the endpoint URL is correct
	    - Check if the resource ID or identifier exists
	    - Ensure the resource hasn't been deleted or moved

Example 2: Error Message Pattern Validation
────────────────────────────────────────────────────────────────────────────────────────
	err := validate.FormatErrorMessageError(
		"valid.*token",
		"token expired",
		"error",
		"OAuth validation",
	)
	log.Printf("Error: %v", err)

Output:
	error_message validation failed
	  Expected: valid.*token
	  Actual:   token expired
	  Context:  OAuth validation
	  Field:    error
	  Suggestions:
	    - Token has expired - implement token refresh logic
	    - Check token expiration time and refresh before expiry
	    - Use refresh token to obtain new access token

Example 3: Content-Type Validation
────────────────────────────────────────────────────────────────────────────────────────
	err := validate.FormatContentTypeError("application/json", "text/html", "API response")
	log.Printf("Validation failed: %v", err)

Output:
	content_type validation failed
	  Expected: application/json
	  Actual:   text/html
	  Context:  API response
	  Suggestions:
	    - Expected JSON but received HTML - endpoint may be returning an error page
	    - Check if the API endpoint exists and is correct
	    - Verify the request method (GET vs POST) is correct

Example 4: Using the Builder Pattern
────────────────────────────────────────────────────────────────────────────────────────
	err := validate.NewValidationFormatter("status_code").
		WithExpected(200).
		WithActual(401).
		WithContext("OAuth token validation").
		WithFieldName("Authorization").
		WithResponseSnippet(`{"error": "invalid_token"}`).
		WithValidationDetails(
			"Token validation failed",
			"Token expired at 2024-01-15T10:30:00Z",
		).
		Format()

Output:
	status_code validation failed
	  Expected: 200
	  Actual:   401
	  Context:  OAuth token validation
	  Field:    Authorization
	  Response: {"error": "invalid_token"}
	  Validation Details:
	    - Token validation failed
	    - Token expired at 2024-01-15T10:30:00Z
	  Suggestions:
	    - Verify authentication credentials are correct
	    - Check if API token or session has expired
	    - Ensure Authorization header is properly formatted (e.g., 'Bearer <token>')

Example 5: Custom Suggestions
────────────────────────────────────────────────────────────────────────────────────────
	err := validate.NewValidationFormatter("custom_check").
		WithExpected("valid_state").
		WithActual("invalid_state").
		WithContext("Custom validation").
		WithSuggestions(
			"Check the custom validation logic",
			"Verify the state machine configuration",
			"Review the state transition rules",
		).
		Format()

Output:
	custom_check validation failed
	  Expected: valid_state
	  Actual:   invalid_state
	  Context:  Custom validation
	  Suggestions:
	    - Check the custom validation logic
	    - Verify the state machine configuration
	    - Review the state transition rules

BEST PRACTICES:
═══════════════════════════════════════════════════════════════════════════════════════════

1. ALWAYS PROVIDE CONTEXT
   - Include endpoint paths, operation types, or request context
   - Helps locate where the validation failed in the codebase

2. USE APPROPRIATE ERROR TYPES
   - Use "status_code" for HTTP status code validations
   - Use "error_message" for error message pattern matching
   - Use "content_type" for Content-Type header validation
   - Use custom types for domain-specific validations

3. LEVERAGE AUTO-GENERATED SUGGESTIONS
   - The system automatically generates relevant suggestions
   - Only use custom suggestions for domain-specific recommendations
   - Auto-generated suggestions are context-aware and comprehensive

4. INCLUDE RESPONSE SNIPPETS FOR DEBUGGING
   - Use WithResponseSnippet() to show actual response content
   - Keep snippets concise (100-200 characters recommended)
   - Helps identify unexpected response formats

5. USE FIELD NAMES FOR STRUCTURED DATA
   - Specify field names for nested validation errors
   - Use field path notation (user.email, data.items[0].id)
   - Helps pinpoint exact validation failures

6. PROVIDE VALIDATION DETAILS FOR COMPLEX CHECKS
   - Use WithValidationDetails() for multi-step validations
   - Include specific failure reasons and intermediate states
   - Helps understand complex validation logic

ERROR TYPE REFERENCE:
═══════════════════════════════════════════════════════════════════════════════════════════

Built-in Error Types with Auto-generated Suggestions:
─────────────────────────────────────────────────────
• status_code          - HTTP status code validation
• error_message        - Error message pattern matching
• content_type         - Content-Type header validation
• status_code_range    - Status code range validation (e.g., 4xx, 5xx)
• cors_headers         - CORS header validation
• response_structure   - Response body structure validation
• response_body        - Response body content validation
• response_encoding    - Response encoding validation
• auth_headers         - Authentication header validation
• custom_headers       - Custom header validation
• json_schema          - JSON schema validation
• data_validation      - Generic data validation
• field_validation     - Field-level validation
• type_validation      - Data type validation
• timeout              - Timeout validation
• rate_limit           - Rate limit validation
• retry_exceeded       - Retry limit exceeded

Each error type generates specific, context-aware suggestions based on the validation
scenario and expected/actual values.

For more information, see the format_helper.go documentation and error_suggestions.go.
═══════════════════════════════════════════════════════════════════════════════════════════
*/
