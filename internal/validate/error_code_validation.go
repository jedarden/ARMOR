package validate

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

/*
Error Code Validation

This module provides validation functions for checking specific error codes
in HTTP response bodies. Many APIs return structured error responses with
error code fields that need to be validated.

Common error code field names:
- "code" - Generic error code (string or int)
- "error_code" - Explicit error code field
- "errorCode" - CamelCase variant
- "error" - Error object that may contain code
- "status" - Status code in response body
*/

// =============================================================================
// ERROR CODE VALIDATION
// =============================================================================

// ValidateErrorCode validates that a response body contains the expected error code.
// This function searches for error codes in common fields and validates the value.
//
// Parameters:
//   - responseBody: The response body as bytes (typically JSON)
//   - expectedCode: The expected error code (string or int)
//
// Returns:
//   - nil if the error code matches or no error code is found (for success cases)
//   - error if the error code doesn't match with detailed mismatch information
//
// Example:
//   // JSON: {"error": {"code": "AUTH_FAILED"}, "message": "Invalid credentials"}
//   err := ValidateErrorCode(responseBody, "AUTH_FAILED")
//   // Returns nil for match, error for mismatch
//
// Example (numeric code):
//   // JSON: {"code": 401, "message": "Unauthorized"}
//   err := ValidateErrorCode(responseBody, 401)
//   // Returns nil for match, error for mismatch
func ValidateErrorCode(responseBody []byte, expectedCode interface{}) error {
	// Validate inputs
	if len(responseBody) == 0 {
		return NewValidationFormatter("error_code").
			WithExpected("non-empty response").
			WithActual("").
			WithContext("error code validation").
			WithValidationDetails(
				"Response body is empty",
				"Cannot validate error code against empty response",
			).
			WithSuggestions(
				"Response body is empty - cannot validate error code",
				"Ensure response contains valid JSON with error code field",
			).
			Format()
	}

	// Parse JSON body
	var body map[string]interface{}
	if err := json.Unmarshal(responseBody, &body); err != nil {
		return NewValidationFormatter("error_code").
			WithExpected("valid JSON").
			WithActual("").
			WithContext("error code validation").
			WithValidationDetails(fmt.Sprintf("Parse error: %v", err)).
			WithSuggestions(
				"Failed to parse response body",
				fmt.Sprintf("Parse error: %v", err),
				"Ensure response contains valid JSON",
			).
			Format()
	}

	// Search for error codes in common fields
	codeFields := []string{"code", "error_code", "errorCode", "status"}
	var foundCodes []foundCode

	for _, field := range codeFields {
		if value, exists := body[field]; exists {
			// Handle both string and numeric codes
			codeStr, codeInt := extractCodeValue(value)
			if codeStr != "" || codeInt != nil {
				foundCodes = append(foundCodes, foundCode{
					value:     value,
					fieldName: field,
					strValue:  codeStr,
					intValue:  codeInt,
				})
			}
		}
	}

	// Check nested error objects
	if nestedObj, ok := body["error"]; ok {
		if errorMap, ok := nestedObj.(map[string]interface{}); ok {
			if codeValue, exists := errorMap["code"]; exists {
				codeStr, codeInt := extractCodeValue(codeValue)
				foundCodes = append(foundCodes, foundCode{
					value:     codeValue,
					fieldName: "error.code",
					strValue:  codeStr,
					intValue:  codeInt,
				})
			}
		}
	}

	// If no error codes found, return success (might be a success response with no error code)
	if len(foundCodes) == 0 {
		return nil
	}

	// Check each found code against expected
	for _, foundCode := range foundCodes {
		if matchesExpectedCode(foundCode, expectedCode) {
			return nil // Found a match
		}
	}

	// No match found - build detailed error
	firstCode := foundCodes[0]
	expectedStr := formatExpectedCode(expectedCode)
	actualStr := formatActualCode(firstCode)

	snippet := extractResponseSnippet(responseBody)

	return NewValidationFormatter("error_code").
		WithExpected(expectedStr).
		WithActual(actualStr).
		WithFieldName(firstCode.fieldName).
		WithResponseSnippet(snippet).
		WithContext("error code validation").
		WithValidationDetails(
			fmt.Sprintf("Checked fields: %s", strings.Join(codeFields, ", ")),
			fmt.Sprintf("Found %d error code field(s) in response", len(foundCodes)),
		).
		WithSuggestions(generateErrorCodeMismatchSuggestions(expectedStr, foundCodes)...).
		Format()
}

// foundCode represents an error code found in the response.
type foundCode struct {
	value     interface{} // Original value
	fieldName string      // Field name where code was found
	strValue  string      // String representation if applicable
	intValue  *int        // Integer value if applicable
}

// extractCodeValue extracts string and integer representations from a code value.
func extractCodeValue(value interface{}) (string, *int) {
	switch v := value.(type) {
	case string:
		return v, nil
	case float64:
		// JSON numbers are parsed as float64
		intVal := int(v)
		return "", &intVal
	case int:
		return "", &v
	case int32:
		intVal := int(v)
		return "", &intVal
	case int64:
		intVal := int(v)
		return "", &intVal
	default:
		// Try to convert to string as fallback
		return fmt.Sprintf("%v", v), nil
	}
}

// matchesExpectedCode checks if a found code matches the expected code.
func matchesExpectedCode(found foundCode, expected interface{}) bool {
	switch exp := expected.(type) {
	case string:
		// Compare as strings
		if found.strValue != "" && found.strValue == exp {
			return true
		}
		// If expected is string, also try to compare with integer as string
		if found.intValue != nil && strconv.Itoa(*found.intValue) == exp {
			return true
		}
	case int:
		// Compare as integers
		if found.intValue != nil && *found.intValue == exp {
			return true
		}
		// If expected is int, also try to compare with string as int
		if found.strValue != "" {
			if intVal, err := strconv.Atoi(found.strValue); err == nil && intVal == exp {
				return true
			}
		}
	case int32:
		if found.intValue != nil && *found.intValue == int(exp) {
			return true
		}
	case int64:
		if found.intValue != nil && *found.intValue == int(exp) {
			return true
		}
	}
	return false
}

// formatExpectedCode formats the expected code for display.
func formatExpectedCode(code interface{}) string {
	switch c := code.(type) {
	case string:
		return fmt.Sprintf("'%s'", c)
	case int:
		return strconv.Itoa(c)
	case int32:
		return strconv.Itoa(int(c))
	case int64:
		return strconv.Itoa(int(c))
	default:
		return fmt.Sprintf("%v", c)
	}
}

// formatActualCode formats the found code for display.
func formatActualCode(found foundCode) string {
	if found.strValue != "" {
		return fmt.Sprintf("'%s'", found.strValue)
	}
	if found.intValue != nil {
		return strconv.Itoa(*found.intValue)
	}
	return fmt.Sprintf("%v", found.value)
}

// generateErrorCodeMismatchSuggestions provides relevant suggestions based on expected vs found codes.
func generateErrorCodeMismatchSuggestions(expected string, foundCodes []foundCode) []string {
	var suggestions []string

	// Add general suggestions
	suggestions = append(suggestions,
		"Verify the expected error code matches the API documentation",
		"Check if error codes vary by error condition (authentication, validation, etc.)",
	)

	// Add specific suggestions based on found codes
	for _, found := range foundCodes {
		if found.strValue != "" {
			// String code suggestions
			if strings.Contains(strings.ToLower(found.strValue), "auth") && !strings.Contains(strings.ToLower(expected), "auth") {
				suggestions = append(suggestions,
					fmt.Sprintf("Found '%s' but expected '%s' - this may be an authentication error", found.strValue, expected),
					"Consider checking credentials, tokens, and permissions",
				)
			}
			if strings.Contains(strings.ToLower(found.strValue), "valid") && !strings.Contains(strings.ToLower(expected), "valid") {
				suggestions = append(suggestions,
					fmt.Sprintf("Found '%s' but expected '%s' - this may be a validation error", found.strValue, expected),
					"Check request body format and required fields",
				)
			}
		}
	}

	// Add suggestion for numeric vs string mismatch
	if len(foundCodes) > 0 {
		first := foundCodes[0]
		if first.intValue != nil && strings.HasPrefix(expected, "'") {
			suggestions = append(suggestions,
				"Expected string code but found numeric code - check API documentation for code format",
			)
		}
		if first.strValue != "" && !strings.HasPrefix(expected, "'") {
			// Check if expected is numeric
			if _, err := strconv.Atoi(expected); err == nil {
				suggestions = append(suggestions,
					"Expected numeric code but found string code - check API documentation for code format",
				)
			}
		}
	}

	return suggestions
}

// =============================================================================
// ERROR CODE PATTERN VALIDATION
// =============================================================================

// ValidateErrorCodePattern validates that an error code matches a pattern.
// This is useful for validating error code formats or categories.
//
// Parameters:
//   - responseBody: The response body as bytes
//   - pattern: The pattern to match (supports regex-like patterns with wildcards)
//
// Returns:
//   - nil if any error code matches the pattern
//   - error if no error codes match or pattern is invalid
//
// Example:
//   // Match codes starting with "AUTH_"
//   err := ValidateErrorCodePattern(responseBody, "AUTH_*")
//
//   // Match numeric codes in 400-499 range
//   err := ValidateErrorCodePattern(responseBody, "4xx")
func ValidateErrorCodePattern(responseBody []byte, pattern string) error {
	if len(responseBody) == 0 {
		return NewValidationFormatter("error_code").
			WithExpected("non-empty response").
			WithActual("").
			WithContext("error code pattern validation").
			WithValidationDetails("Response body is empty").
			WithSuggestions("Response body is empty - cannot validate error code pattern").
			Format()
	}

	if pattern == "" {
		return NewValidationFormatter("error_code").
			WithExpected("non-empty pattern").
			WithActual("").
			WithContext("error code pattern validation").
			WithValidationDetails("Pattern cannot be empty").
			WithSuggestions("Provide a pattern to match error codes against").
			Format()
	}

	// Parse JSON body
	var body map[string]interface{}
	if err := json.Unmarshal(responseBody, &body); err != nil {
		return NewValidationFormatter("error_code").
			WithExpected("valid JSON").
			WithActual("").
			WithContext("error code pattern validation").
			WithValidationDetails(fmt.Sprintf("Parse error: %v", err)).
			WithSuggestions("Failed to parse response body", "Ensure response contains valid JSON").
			Format()
	}

	// Search for error codes
	codeFields := []string{"code", "error_code", "errorCode", "status"}
	var foundCodes []foundCode

	for _, field := range codeFields {
		if value, exists := body[field]; exists {
			codeStr, codeInt := extractCodeValue(value)
			if codeStr != "" || codeInt != nil {
				foundCodes = append(foundCodes, foundCode{
					value:     value,
					fieldName: field,
					strValue:  codeStr,
					intValue:  codeInt,
				})
			}
		}
	}

	// Check nested error objects
	if nestedObj, ok := body["error"]; ok {
		if errorMap, ok := nestedObj.(map[string]interface{}); ok {
			if codeValue, exists := errorMap["code"]; exists {
				codeStr, codeInt := extractCodeValue(codeValue)
				foundCodes = append(foundCodes, foundCode{
					value:     codeValue,
					fieldName: "error.code",
					strValue:  codeStr,
					intValue:  codeInt,
				})
			}
		}
	}

	// If no error codes found, return error
	if len(foundCodes) == 0 {
		snippet := extractResponseSnippet(responseBody)
		return NewValidationFormatter("error_code").
			WithExpected(pattern).
			WithActual("no error code found").
			WithResponseSnippet(snippet).
			WithContext("error code pattern validation").
			WithValidationDetails(
				fmt.Sprintf("Checked fields: %s", strings.Join(codeFields, ", ")),
				"Response does not contain any error code fields",
			).
			WithSuggestions(
				"Response does not contain an error code field",
				fmt.Sprintf("Checked fields: %s", strings.Join(codeFields, ", ")),
				"Verify the API returns error codes in the response",
			).
			Format()
	}

	// Check if any code matches the pattern
	for _, foundCode := range foundCodes {
		if codeMatchesPattern(foundCode, pattern) {
			return nil // Found a match
		}
	}

	// No match - build error
	firstCode := foundCodes[0]
	actualCodes := formatAllFoundCodes(foundCodes)

	snippet := extractResponseSnippet(responseBody)

	return NewValidationFormatter("error_code").
		WithExpected(pattern).
		WithActual(actualCodes).
		WithFieldName(firstCode.fieldName).
		WithResponseSnippet(snippet).
		WithContext("error code pattern validation").
		WithValidationDetails(
			fmt.Sprintf("Pattern '%s' did not match any error codes", pattern),
			fmt.Sprintf("Found %d error code field(s) in response", len(foundCodes)),
		).
		WithSuggestions(
			fmt.Sprintf("Error code(s) found: %s", actualCodes),
			"Review the expected pattern and actual error codes",
			"Check API documentation for valid error code formats",
		).
		Format()
}

// codeMatchesPattern checks if a code value matches a pattern.
func codeMatchesPattern(found foundCode, pattern string) bool {
	// Handle 4xx, 5xx patterns
	if strings.HasSuffix(pattern, "xx") && len(pattern) == 3 {
		century := int(pattern[0] - '0')
		if found.intValue != nil {
			code := *found.intValue
			if code >= century*100 && code <= century*100+99 {
				return true
			}
		}
	}

	// Handle wildcard patterns (AUTH_*, ERR_*, etc.)
	if strings.Contains(pattern, "*") {
		if found.strValue != "" {
			prefix := strings.TrimSuffix(pattern, "*")
			return strings.HasPrefix(found.strValue, prefix)
		}
	}

	// Handle direct string match
	if found.strValue != "" && found.strValue == pattern {
		return true
	}

	// Handle numeric match
	if found.intValue != nil {
		if num, err := strconv.Atoi(pattern); err == nil && *found.intValue == num {
			return true
		}
	}

	return false
}

// formatAllFoundCodes formats all found codes for display.
func formatAllFoundCodes(foundCodes []foundCode) string {
	var parts []string
	for _, fc := range foundCodes {
		parts = append(parts, formatActualCode(fc))
	}
	return strings.Join(parts, ", ")
}

// =============================================================================
// MULTIPLE ERROR CODE VALIDATION
// =============================================================================

// ValidateErrorCodeAny validates that the response contains one of several allowed error codes.
// This is useful when multiple error codes are acceptable for a scenario.
//
// Parameters:
//   - responseBody: The response body as bytes
//   - allowedCodes: Slice of acceptable error codes (can be mixed string/int)
//
// Returns:
//   - nil if any allowed code is found
//   - error if no allowed codes match
//
// Example:
//   // Accept any authentication-related error
//   err := ValidateErrorCodeAny(responseBody, []interface{}{"AUTH_FAILED", "TOKEN_EXPIRED", "INVALID_CREDENTIALS"})
func ValidateErrorCodeAny(responseBody []byte, allowedCodes []interface{}) error {
	if len(allowedCodes) == 0 {
		return NewValidationFormatter("error_code").
			WithExpected("at least one allowed code").
			WithActual("").
			WithContext("error code validation").
			WithValidationDetails("allowedCodes cannot be empty").
			WithSuggestions("Provide at least one allowed error code").
			Format()
	}

	if len(responseBody) == 0 {
		return NewValidationFormatter("error_code").
			WithExpected("non-empty response").
			WithActual("").
			WithContext("error code validation").
			WithValidationDetails("Response body is empty").
			WithSuggestions("Response body is empty - cannot validate error code").
			Format()
	}

	// Parse JSON body
	var body map[string]interface{}
	if err := json.Unmarshal(responseBody, &body); err != nil {
		return NewValidationFormatter("error_code").
			WithExpected("valid JSON").
			WithActual("").
			WithContext("error code validation").
			WithValidationDetails(fmt.Sprintf("Parse error: %v", err)).
			WithSuggestions("Failed to parse response body", "Ensure response contains valid JSON").
			Format()
	}

	// Search for error codes
	codeFields := []string{"code", "error_code", "errorCode", "status"}
	var foundCodes []foundCode

	for _, field := range codeFields {
		if value, exists := body[field]; exists {
			codeStr, codeInt := extractCodeValue(value)
			if codeStr != "" || codeInt != nil {
				foundCodes = append(foundCodes, foundCode{
					value:     value,
					fieldName: field,
					strValue:  codeStr,
					intValue:  codeInt,
				})
			}
		}
	}

	// Check nested error objects
	if nestedObj, ok := body["error"]; ok {
		if errorMap, ok := nestedObj.(map[string]interface{}); ok {
			if codeValue, exists := errorMap["code"]; exists {
				codeStr, codeInt := extractCodeValue(codeValue)
				foundCodes = append(foundCodes, foundCode{
					value:     codeValue,
					fieldName: "error.code",
					strValue:  codeStr,
					intValue:  codeInt,
				})
			}
		}
	}

	// Check if any found code matches any allowed code
	for _, foundCode := range foundCodes {
		for _, allowedCode := range allowedCodes {
			if matchesExpectedCode(foundCode, allowedCode) {
				return nil // Found a match
			}
		}
	}

	// No match - build error
	allowedStr := formatAllowedCodes(allowedCodes)
	var actualStr string
	var fieldName string
	if len(foundCodes) > 0 {
		actualStr = formatAllFoundCodes(foundCodes)
		fieldName = foundCodes[0].fieldName
	} else {
		actualStr = "no error code found"
	}

	snippet := extractResponseSnippet(responseBody)

	return NewValidationFormatter("error_code").
		WithExpected(allowedStr).
		WithActual(actualStr).
		WithFieldName(fieldName).
		WithResponseSnippet(snippet).
		WithContext("error code validation").
		WithValidationDetails(
			fmt.Sprintf("Allowed codes: %s", allowedStr),
			fmt.Sprintf("Checked fields: %s", strings.Join(codeFields, ", ")),
		).
		WithSuggestions(
			fmt.Sprintf("Expected one of: %s", allowedStr),
			"Actual error code does not match any allowed codes",
			"Check API documentation for valid error codes",
		).
		Format()
}

// formatAllowedCodes formats allowed codes for display.
func formatAllowedCodes(codes []interface{}) string {
	var parts []string
	for _, code := range codes {
		parts = append(parts, formatExpectedCode(code))
	}
	return strings.Join(parts, ", ")
}
