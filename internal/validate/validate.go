// Package validate provides helper functions for validating HTTP responses and other data.
package validate

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

// HTTPStatusCodeIsValid checks if an HTTP response's status code matches one or more expected codes.
// It supports both a single status code and a slice of valid status codes, making it useful for
// validating API responses where multiple status codes might indicate success (e.g., 200 OK and 206 Partial Content).
//
// The expected parameter can be:
//   - A single int (e.g., 200)
//   - A slice of ints (e.g., []int{200, 201, 204})
//
// Returns true if the response status code matches any of the expected codes, false otherwise.
//
// Example usage:
//
//	singleCheck := HTTPStatusCodeIsValid(response, 200)
//	multipleCheck := HTTPStatusCodeIsValid(response, []int{200, 201, 204})
//	noContentCheck := HTTPStatusCodeIsValid(response, 204)
func HTTPStatusCodeIsValid(resp *http.Response, expected interface{}) bool {
	if resp == nil {
		return false
	}

	statusCode := resp.StatusCode

	switch v := expected.(type) {
	case int:
		return statusCode == v
	case []int:
		for _, code := range v {
			if statusCode == code {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// HTTPStatusCodeIsError checks if an HTTP response's status code indicates an error (4xx or 5xx).
//
// Returns true if the response status code is in the 400-599 range, false otherwise.
//
// Example usage:
//
//	if HTTPStatusCodeIsError(response) {
//	    // Handle error response
//	}
func HTTPStatusCodeIsError(resp *http.Response) bool {
	if resp == nil {
		return false
	}

	return resp.StatusCode >= 400 && resp.StatusCode < 600
}

// HTTPStatusCodeIsClientError checks if an HTTP response's status code indicates a client error (4xx).
//
// Returns true if the response status code is in the 400-499 range, false otherwise.
//
// Example usage:
//
//	if HTTPStatusCodeIsClientError(response) {
//	    // Handle 4xx client error (bad request, unauthorized, etc.)
//	}
func HTTPStatusCodeIsClientError(resp *http.Response) bool {
	if resp == nil {
		return false
	}

	return resp.StatusCode >= 400 && resp.StatusCode < 500
}

// HTTPStatusCodeIsServerError checks if an HTTP response's status code indicates a server error (5xx).
//
// Returns true if the response status code is in the 500-599 range, false otherwise.
//
// Example usage:
//
//	if HTTPStatusCodeIsServerError(response) {
//	    // Handle 5xx server error (internal server error, bad gateway, etc.)
//	}
func HTTPStatusCodeIsServerError(resp *http.Response) bool {
	if resp == nil {
		return false
	}

	return resp.StatusCode >= 500 && resp.StatusCode < 600
}

// ContentTypeIsValid checks if a response's Content-Type header matches the expected pattern.
// It supports pattern matching where the base content-type matches regardless of parameters.
// For example, "application/json" will match "application/json; charset=utf-8".
//
// Parameters:
//   - resp: The HTTP response to validate
//   - expected: The expected content-type pattern (e.g., "application/json")
//
// Returns true if the response Content-Type matches the expected pattern, false otherwise.
// Returns false if the response is nil or has no Content-Type header.
//
// Example usage:
//
//	if ContentTypeIsValid(response, "application/json") {
//	    // Handle JSON response
//	}
func ContentTypeIsValid(resp *http.Response, expected string) bool {
	if resp == nil {
		return false
	}

	actual := resp.Header.Get("Content-Type")
	if actual == "" {
		return false
	}

	// Parse both content-type strings to extract base media types
	actualMediaType := parseContentType(actual)
	expectedMediaType := parseContentType(expected)

	return actualMediaType == expectedMediaType
}

// parseContentType extracts the base media type from a content-type string.
// It strips parameters like charset, boundary, etc.
// For example, "application/json; charset=utf-8" becomes "application/json".
func parseContentType(contentType string) string {
	if contentType == "" {
		return ""
	}

	// Split by semicolon to separate media type from parameters
	// The first part before any semicolon is the base media type
	for i, c := range contentType {
		if c == ';' {
			return trimSpace(contentType[:i])
		}
	}

	// No parameters found, return the whole string trimmed
	return trimSpace(contentType)
}

// trimSpace removes leading and trailing whitespace from a string.
func trimSpace(s string) string {
	start := 0
	end := len(s)

	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}

	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}

	return s[start:end]
}

// ErrorResponseFieldNames specifies custom field names to check when validating error response structures.
// If nil, the default field names "error" and "message" are checked.
type ErrorResponseFieldNames struct {
	// PrimaryFieldName is the main error field to check (e.g., "error", "detail", "error_description")
	PrimaryFieldName string
	// SecondaryFieldName is an optional secondary field to check (e.g., "message", "description")
	SecondaryFieldName string
}

// DefaultErrorResponseFieldNames returns the default field names used for error response validation.
// This checks for both "error" and "message" fields.
func DefaultErrorResponseFieldNames() *ErrorResponseFieldNames {
	return &ErrorResponseFieldNames{
		PrimaryFieldName:   "error",
		SecondaryFieldName: "message",
	}
}

// ErrorResponseStructureIsValid validates whether a response object contains a valid error response structure.
// It checks for the presence of common error fields and returns whether the structure is valid.
//
// The response parameter can be:
//   - map[string]interface{}: A parsed JSON object representing the response body
//   - Any type that can be inspected for error fields using reflection
//
// The fieldNames parameter allows customization of which fields to check:
//   - If nil, checks for default fields "error" and "message"
//   - If provided, checks for the specified primary and secondary field names
//
// Returns true if the response contains at least one of the expected error fields with a non-empty value,
// false otherwise.
//
// Example usage:
//
//	// Using default field names (checks for "error" and "message")
//	body := map[string]interface{}{"error": "Invalid input"}
//	isValid := ErrorResponseStructureIsValid(body, nil)
//
//	// Using custom field names
//	customFields := &ErrorResponseFieldNames{PrimaryFieldName: "detail", SecondaryFieldName: "description"}
//	body := map[string]interface{}{"detail": "Resource not found"}
//	isValid := ErrorResponseStructureIsValid(body, customFields)
func ErrorResponseStructureIsValid(response interface{}, fieldNames *ErrorResponseFieldNames) bool {
	if response == nil {
		return false
	}

	// Use default field names if not provided
	if fieldNames == nil {
		fieldNames = DefaultErrorResponseFieldNames()
	}

	// Try to convert to map for field checking
	respMap, ok := response.(map[string]interface{})
	if !ok {
		return false
	}

	// Check primary field
	if fieldNames.PrimaryFieldName != "" {
		if value, exists := respMap[fieldNames.PrimaryFieldName]; exists {
			if isNonEmptyValue(value) {
				return true
			}
		}
	}

	// Check secondary field
	if fieldNames.SecondaryFieldName != "" {
		if value, exists := respMap[fieldNames.SecondaryFieldName]; exists {
			if isNonEmptyValue(value) {
				return true
			}
		}
	}

	return false
}

// isNonEmptyValue checks if a value is non-empty and meaningful.
// Returns true for strings with content, numbers (non-zero), and true booleans.
// Returns false for empty strings, zero values, false booleans, and nil.
func isNonEmptyValue(value interface{}) bool {
	if value == nil {
		return false
	}

	switch v := value.(type) {
	case string:
		return v != ""
	case int:
		return v != 0
	case int8:
		return v != 0
	case int16:
		return v != 0
	case int32:
		return v != 0
	case int64:
		return v != 0
	case uint:
		return v != 0
	case uint8:
		return v != 0
	case uint16:
		return v != 0
	case uint32:
		return v != 0
	case uint64:
		return v != 0
	case float32:
		return v != 0.0
	case float64:
		return v != 0.0
	case bool:
		return v
	case map[string]interface{}:
		return len(v) > 0
	case []interface{}:
		return len(v) > 0
	default:
		return true
	}
}

// CORSConfig specifies expected CORS header values for validation.
type CORSConfig struct {
	// AllowOrigin is the expected value for Access-Control-Allow-Origin.
	// Use "*" for wildcard CORS, or specify an exact origin like "https://example.com".
	// If empty, the origin header is not validated.
	AllowOrigin string

	// AllowMethods is the expected value for Access-Control-Allow-Methods.
	// If empty, the methods header is not validated.
	AllowMethods string

	// AllowHeaders is the expected value for Access-Control-Allow-Headers.
	// If empty, the headers header is not validated.
	AllowHeaders string

	// AllowCredentials is the expected value for Access-Control-Allow-Credentials.
	// If true, expects "true"; if false, expects "true" or header not present.
	// This is typically used with specific origins (not wildcard).
	AllowCredentials bool

	// ExposeHeaders is the expected value for Access-Control-Expose-Headers.
	// If empty, the expose headers header is not validated.
	ExposeHeaders string

	// MaxAge is the expected value for Access-Control-Max-Age.
	// If empty, the max-age header is not validated.
	MaxAge string
}

// CORSHeadersIsValid validates CORS headers on an HTTP response, particularly for error responses.
// It checks the presence and values of common CORS headers and returns whether they match the expected configuration.
//
// This function is particularly useful for validating that error responses (4xx/5xx) include proper CORS headers,
// which is required for browsers to receive and process error responses from cross-origin requests.
//
// Parameters:
//   - resp: The HTTP response to validate
//   - config: Expected CORS configuration (use nil for basic validation that only checks header presence)
//
// Returns true if all specified CORS headers are present and match expected values, false otherwise.
// Returns false if the response is nil.
//
// When config is nil, only validates that Access-Control-Allow-Origin header exists (non-empty).
// When config is provided, validates only the fields that are non-empty in the config.
//
// Example usage:
//
//	// Basic validation - check if CORS headers exist
//	if CORSHeadersIsValid(errorResponse, nil) {
//	    // CORS headers are present
//	}
//
//	// Validate specific origin
//	config := &CORSConfig{AllowOrigin: "https://example.com"}
//	if CORSHeadersIsValid(errorResponse, config) {
//	    // CORS headers match expected origin
//	}
//
//	// Validate wildcard CORS
//	config := &CORSConfig{AllowOrigin: "*", AllowCredentials: false}
//	if CORSHeadersIsValid(errorResponse, config) {
//	    // Wildcard CORS is properly configured
//	}
//
//	// Validate full CORS configuration
//	config := &CORSConfig{
//	    AllowOrigin: "https://example.com",
//	    AllowMethods: "GET, POST, OPTIONS",
//	    AllowHeaders: "Content-Type, Authorization",
//	    AllowCredentials: true,
//	}
//	if CORSHeadersIsValid(errorResponse, config) {
//	    // Full CORS configuration is valid
//	}
func CORSHeadersIsValid(resp *http.Response, config *CORSConfig) bool {
	if resp == nil {
		return false
	}

	// If no config provided, do basic validation: check if Access-Control-Allow-Origin exists
	if config == nil {
		origin := resp.Header.Get("Access-Control-Allow-Origin")
		return origin != ""
	}

	// Validate Access-Control-Allow-Origin if specified
	if config.AllowOrigin != "" {
		origin := resp.Header.Get("Access-Control-Allow-Origin")
		if origin == "" {
			return false
		}

		// Check for wildcard or exact match
		if config.AllowOrigin == "*" {
			// Wildcard is valid
			if origin != "*" {
				return false
			}
		} else {
			// Exact origin match (case-sensitive for origins)
			if origin != config.AllowOrigin {
				return false
			}
		}
	}

	// Validate Access-Control-Allow-Methods if specified
	if config.AllowMethods != "" {
		methods := resp.Header.Get("Access-Control-Allow-Methods")
		if methods != config.AllowMethods {
			return false
		}
	}

	// Validate Access-Control-Allow-Headers if specified
	if config.AllowHeaders != "" {
		headers := resp.Header.Get("Access-Control-Allow-Headers")
		if headers != config.AllowHeaders {
			return false
		}
	}

	// Validate Access-Control-Allow-Credentials if specified
	if config.AllowCredentials {
		credentials := resp.Header.Get("Access-Control-Allow-Credentials")
		if credentials != "true" {
			return false
		}
	}

	// Validate Access-Control-Expose-Headers if specified
	if config.ExposeHeaders != "" {
		exposeHeaders := resp.Header.Get("Access-Control-Expose-Headers")
		if exposeHeaders != config.ExposeHeaders {
			return false
		}
	}

	// Validate Access-Control-Max-Age if specified
	if config.MaxAge != "" {
		maxAge := resp.Header.Get("Access-Control-Max-Age")
		if maxAge != config.MaxAge {
			return false
		}
	}

	return true
}

// =============================================================================
// ERROR MESSAGE PATTERN VALIDATION
// =============================================================================

// ErrorMessagePattern specifies pattern matching configuration for error messages.
type ErrorMessagePattern struct {
	// Pattern is the regex pattern to match against error messages
	Pattern string

	// CaseInsensitive indicates whether matching should be case-insensitive
	CaseInsensitive bool

	// MatchAny indicates whether to match any field (true) or all fields (false)
	MatchAny bool

	// FieldNames specifies which fields to check (e.g., "error", "message", "detail")
	// If empty, checks default fields: "error", "message"
	FieldNames []string
}

// ValidateErrorMessagePattern validates error message content against regex patterns.
//
// This function extracts error messages from response bodies and validates them
// against a regex pattern. It supports both exact matching and case-insensitive
// pattern matching for flexible error validation.
//
// Parameters:
//   - bodyBytes: Raw response body bytes
//   - pattern: Regex pattern to match against error messages
//   - caseInsensitive: If true, performs case-insensitive matching
//
// Returns:
//   - true if the error message matches the pattern, false otherwise
//   - error if JSON parsing fails or response is invalid
//
// Example usage:
//
//	// Check for "not found" error (case-insensitive)
//	matches, err := ValidateErrorMessagePattern(body, `(?i)not found`, false)
//
//	// Check for "invalid.*token" pattern
//	matches, err := ValidateErrorMessagePattern(body, `invalid.*token`, true)
func ValidateErrorMessagePattern(bodyBytes []byte, pattern string, caseInsensitive bool) (bool, error) {
	if len(bodyBytes) == 0 {
		return false, fmt.Errorf("response body is empty")
	}

	if pattern == "" {
		return false, fmt.Errorf("pattern cannot be empty")
	}

	// Parse JSON body
	var body map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		return false, fmt.Errorf("failed to parse response body: %w", err)
	}

	// Compile regex pattern
	regexPattern := pattern
	if caseInsensitive {
		regexPattern = "(?i)" + pattern
	}

	re, err := regexp.Compile(regexPattern)
	if err != nil {
		return false, fmt.Errorf("invalid regex pattern: %w", err)
	}

	// Check default error fields
	defaultFields := []string{"error", "message", "detail", "description"}
	for _, field := range defaultFields {
		if value, exists := body[field]; exists {
			if strValue, ok := value.(string); ok {
				if re.MatchString(strValue) {
					return true, nil
				}
			}
			// Check nested error objects
			if nestedObj, ok := value.(map[string]interface{}); ok {
				if nestedValue, exists := nestedObj["message"]; exists {
					if strValue, ok := nestedValue.(string); ok {
						if re.MatchString(strValue) {
							return true, nil
						}
					}
				}
			}
		}
	}

	return false, nil
}

// ValidateErrorMessage validates error message content against a pattern.
//
// This function checks if a response body contains an error message that matches
// the expected pattern. It supports both regex patterns and simple substring matching.
// When the pattern is found, it returns nil. When the pattern is not found, it returns
// a descriptive error including a snippet of the actual response.
//
// The function automatically detects whether the pattern is a regex or substring:
// - If the pattern contains regex metacharacters (. * + ? ^ $ { } [ ] ( ) | \), it's treated as regex
// - Otherwise, it's treated as a simple substring search (case-sensitive)
//
// Parameters:
//   - response: Raw response body bytes
//   - expectedPattern: Pattern to search for (regex or substring)
//
// Returns:
//   - nil if the pattern is found in the response
//   - error if the pattern is not found or if response is invalid
//
// Example usage:
//
//	// Regex pattern matching
//	err := ValidateErrorMessage(body, "invalid.*token")
//	if err != nil {
//	    // Pattern not found
//	}
//
//	// Simple substring matching
//	err = ValidateErrorMessage(body, "not found")
//	if err != nil {
//	    // Substring not found
//	}
//
//	// Common error patterns
//	err = ValidateErrorMessage(body, "unauthorized")        // OAuth errors
//	err = ValidateErrorMessage(body, "invalid.*credentials") // Auth errors
//	err = ValidateErrorMessage(body, "rate.*limit")          // Rate limiting
//	err = ValidateErrorMessage(body, "timeout")               // Timeout errors
//
// # Common Error Pattern Examples
//
// The function auto-detects regex vs substring matching:
//
// # OAuth 2.0 Errors
//
//	response := []byte(`{"error": "invalid_token", "error_description": "The access token expired"}`)
//	err := ValidateErrorMessage(response, "invalid_token")  // substring match
//
//	response := []byte(`{"error": "access_denied"}`)
//	err = ValidateErrorMessage(response, "access_denied|invalid_grant")  // regex match
//
// # Authentication Errors
//
//	response := []byte(`{"message": "Authentication failed: invalid credentials"}`)
//	err := ValidateErrorMessage(response, "authentication.*failed")  // regex match
//
//	response := []byte(`{"error": "Invalid username or password"}`)
//	err = ValidateErrorMessage(response, "invalid.*credentials")  // regex match
//
// # Authorization Errors
//
//	response := []byte(`{"error": "Permission denied"}`)
//	err := ValidateErrorMessage(response, "permission.*denied")  // regex match
//
//	response := []byte(`{"detail": "User is not authorized to access this resource"}`)
//	err = ValidateErrorMessage(response, "not.*authorized")  // regex match
//
// # Validation Errors
//
//	response := []byte(`{"error": "Validation failed: required field missing"}`)
//	err = ValidateErrorMessage(response, "validation.*failed")  // regex match
//
//	response := []byte(`{"detail": "Invalid input: email format is incorrect"}`)
//	err = ValidateErrorMessage(response, "invalid.*input")  // regex match
//
// # Resource Errors
//
//	response := []byte(`{"error": "Resource not found"}`)
//	err = ValidateErrorMessage(response, "not found")  // substring match
//
//	response := []byte(`{"message": "User does not exist"}`)
//	err = ValidateErrorMessage(response, "does not exist")  // substring match
//
// # Rate Limiting Errors
//
//	response := []byte(`{"error": "Rate limit exceeded"}`)
//	err = ValidateErrorMessage(response, "rate.*limit")  // regex match
//
//	response := []byte(`{"detail": "Too many requests, please try again later"}`)
//	err = ValidateErrorMessage(response, "too many requests")  // substring match
//
// # Timeout Errors
//
//	response := []byte(`{"message": "Request timeout"}`)
//	err = ValidateErrorMessage(response, "timeout")  // substring match
//
//	response := []byte(`{"error": "Connection timeout after 30 seconds"}`)
//	err = ValidateErrorMessage(response, "timeout")  // substring match
//
// # Server Errors
//
//	response := []byte(`{"error": "Internal server error"}`)
//	err = ValidateErrorMessage(response, "internal.*error")  // regex match
//
//	response := []byte(`{"message": "Service temporarily unavailable"}`)
//	err = ValidateErrorMessage(response, "unavailable")  // substring match
func ValidateErrorMessage(response []byte, expectedPattern string) error {
	// Validate inputs
	if len(response) == 0 {
		return NewValidationFormatter("error_message").
			WithExpected("non-empty response").
			WithActual("").
			WithPatternDetails("cannot validate against empty response").
			WithContext("error message validation").
			WithSuggestions(
				"Response body is empty",
				"Cannot validate error message pattern against empty response",
			).
			Format()
	}

	if expectedPattern == "" {
		return NewValidationFormatter("error_message").
			WithExpected("non-empty pattern").
			WithActual("").
			WithPatternDetails("pattern cannot be empty").
			WithContext("error message validation").
			WithSuggestions(
				"Expected pattern cannot be empty",
				"Provide a pattern to match against error messages",
			).
			Format()
	}

	// Parse JSON body
	var body map[string]interface{}
	if err := json.Unmarshal(response, &body); err != nil {
		return NewValidationFormatter("error_message").
			WithExpected("valid JSON").
			WithActual("").
			WithPatternDetails("failed to parse response body").
			WithContext("error message validation").
			WithValidationDetails(fmt.Sprintf("Parse error: %v", err)).
			WithSuggestions(
				"Failed to parse response body",
				fmt.Sprintf("Parse error: %v", err),
				"Ensure response contains valid JSON",
			).
			Format()
	}

	// Search for error messages in common fields
	defaultFields := []string{"error", "message", "detail", "description", "error_description"}
	type foundMessage struct {
		message   string
		fieldName string
	}
	var foundMessages []foundMessage

	for _, field := range defaultFields {
		if value, exists := body[field]; exists {
			if strValue, ok := value.(string); ok && strValue != "" {
				foundMessages = append(foundMessages, foundMessage{
					message:   strValue,
					fieldName: field,
				})
			}
			// Check nested error objects
			if nestedObj, ok := value.(map[string]interface{}); ok {
				if nestedValue, exists := nestedObj["message"]; exists {
					if strValue, ok := nestedValue.(string); ok && strValue != "" {
						foundMessages = append(foundMessages, foundMessage{
							message:   strValue,
							fieldName: field + ".message",
						})
					}
				}
			}
		}
	}

	// If no error messages found, return detailed error with response snippet
	if len(foundMessages) == 0 {
		snippet := extractResponseSnippet(response)
		responseSize := len(response)
		return NewValidationFormatter("error_message").
			WithExpected("error message field").
			WithActual("").
			WithPatternDetails("no error message found in response").
			WithResponseSnippet(snippet).
			WithContext("error message validation").
			WithValidationDetails(
				fmt.Sprintf("Checked fields: %s", strings.Join(defaultFields, ", ")),
				fmt.Sprintf("Response size: %d bytes", responseSize),
			).
			WithSuggestions(
				"No error message found in response body",
				fmt.Sprintf("Checked fields: %s", strings.Join(defaultFields, ", ")),
				"Response may not contain standard error fields",
			).
			Format()
	}

	// Detect if pattern is regex or substring
	isRegex := containsRegexMetacharacters(expectedPattern)
	patternType := "substring"
	if isRegex {
		patternType = "regex"
	}

	// Check each found message for pattern match
	for _, foundMsg := range foundMessages {
		var matched bool

		if isRegex {
			// Try regex matching
			re, err := regexp.Compile(expectedPattern)
			if err != nil {
				return FormatValidationErrorWithDetails(
					"error_message",
					fmt.Sprintf("valid regex pattern '%s'", expectedPattern),
					fmt.Sprintf("invalid regex '%s'", expectedPattern),
					"",
					"",
					foundMsg.fieldName,
					"",
					nil,
					fmt.Sprintf("Pattern type: regex (contains metacharacters), Parse error: %v", err),
					"",
					[]string{
						fmt.Sprintf("Invalid regex pattern: %s", expectedPattern),
						fmt.Sprintf("Regex compilation error: %v", err),
						"Check regex syntax and escape special characters",
					},
				)
			}
			matched = re.MatchString(foundMsg.message)
		} else {
			// Simple substring matching (case-sensitive)
			matched = strings.Contains(foundMsg.message, expectedPattern)
		}

		if matched {
			// Pattern found - return nil (success)
			return nil
		}
	}

	// Pattern not found in any messages - return descriptive error with full details
	snippet := extractResponseSnippet(response)
	firstMessage := foundMessages[0]

	// Build validation details including response size context
	responseSize := len(response)
	var validationDetails []string
	validationDetails = append(validationDetails, fmt.Sprintf("Pattern type: %s", patternType))
	validationDetails = append(validationDetails, fmt.Sprintf("Expected pattern: %s", expectedPattern))
	validationDetails = append(validationDetails, fmt.Sprintf("Actual error message: \"%s\"", firstMessage.message))
	validationDetails = append(validationDetails, fmt.Sprintf("Field name: %s", firstMessage.fieldName))
	validationDetails = append(validationDetails, fmt.Sprintf("Checked fields: %s", strings.Join(defaultFields, ", ")))
	validationDetails = append(validationDetails, fmt.Sprintf("Response size: %d bytes", responseSize))

	if len(foundMessages) > 1 {
		validationDetails = append(validationDetails, fmt.Sprintf("Found %d error message fields in response", len(foundMessages)))
	}

	// Generate pattern-specific suggestions
	suggestions := generatePatternMismatchSuggestions(expectedPattern, firstMessage.message, isRegex)

	// Use ValidationFormatter builder pattern for enhanced error formatting
	return NewValidationFormatter("error_message").
		WithExpected(expectedPattern).
		WithActual(firstMessage.message).
		WithPatternDetails(fmt.Sprintf("%s pattern '%s' did not match", patternType, expectedPattern)).
		WithResponseSnippet(snippet).
		WithFieldName(firstMessage.fieldName).
		WithContext("error message validation").
		WithValidationDetails(validationDetails...).
		WithSuggestions(suggestions...).
		Format()
}

// containsRegexMetacharacters checks if a string contains regex metacharacters.
// This is used to auto-detect whether a pattern should be treated as regex or substring.
func containsRegexMetacharacters(pattern string) bool {
	regexChars := []string{".", "*", "+", "?", "^", "$", "{", "}", "[", "]", "(", ")", "|", "\\"}
	for _, char := range regexChars {
		if strings.Contains(pattern, char) {
			return true
		}
	}
	return false
}

// generatePatternMismatchSuggestions provides relevant suggestions based on pattern and actual message.
// This function analyzes the expected pattern and actual message to generate specific suggestions.
func generatePatternMismatchSuggestions(expectedPattern, actualMessage string, isRegex bool) []string {
	var suggestions []string

	expectedLower := strings.ToLower(expectedPattern)
	actualLower := strings.ToLower(actualMessage)

	// Common pattern mismatch checks
	if strings.Contains(expectedLower, "token") && !strings.Contains(actualLower, "token") {
		suggestions = append(suggestions,
			"Pattern looks for 'token' but actual message doesn't contain it",
			"Check if the error is about authentication/authorization rather than tokens",
			"Consider expanding pattern to include 'auth' or 'access' related terms",
		)
	}

	if strings.Contains(expectedLower, "invalid") && !strings.Contains(actualLower, "invalid") {
		if strings.Contains(actualLower, "expired") {
			suggestions = append(suggestions,
				"Pattern expects 'invalid' but message contains 'expired'",
				"Consider using pattern: 'invalid.*expired|expired.*invalid' to match both cases",
				"Or use separate patterns for different token states",
			)
		}
	}

	if strings.Contains(expectedLower, "not found") && strings.Contains(actualLower, "does not exist") {
		suggestions = append(suggestions,
			"Pattern uses 'not found' but message uses 'does not exist'",
			"Consider using pattern: 'not found|does not exist' to match both phrasings",
			"Or use simpler pattern: 'not.*found|does.*not.*exist'",
		)
	}

	if isRegex {
		// Regex-specific suggestions
		if strings.Contains(expectedPattern, ".") && !strings.Contains(expectedPattern, "\\") {
			suggestions = append(suggestions,
				"Regex contains '.' which matches any character (except newline)",
				"If you meant literal dot, escape it: '\\.'",
				"If substring search is intended, consider using plain text without metacharacters",
			)
		}

		if strings.Contains(expectedPattern, "*") && !strings.Contains(expectedPattern, ".*") {
			suggestions = append(suggestions,
				"Regex contains bare '*' which is a quantifier, not a wildcard",
				"For 'any characters' pattern, use: '.*' instead of '*'",
				"For 'zero or more' of previous character, ensure '*' is preceded by pattern to repeat",
			)
		}

		if strings.Contains(expectedPattern, "^") || strings.Contains(expectedPattern, "$") {
			suggestions = append(suggestions,
				"Pattern uses anchors (^ or $) which match start/end of string",
				"Ensure actual message doesn't have extra whitespace or newlines",
				"Consider removing anchors if message may contain additional text",
			)
		}

		// Case sensitivity check - detect if pattern and message have different casing
		expectedHasMixedCase := expectedPattern != expectedLower
		actualHasMixedCase := actualMessage != actualLower

		// Check if there's a case mismatch between pattern and message
		if expectedHasMixedCase && actualHasMixedCase && expectedLower != actualLower {
			suggestions = append(suggestions,
				"Pattern is case-sensitive but may need case-insensitive matching",
				"Consider using '(?i)' prefix for case-insensitive regex",
				"Or convert pattern to handle different casing patterns",
			)
		}
	} else {
		// Substring-specific suggestions
		if expectedLower != actualLower && strings.Contains(actualLower, expectedLower) {
			suggestions = append(suggestions,
				"Substring match is case-sensitive",
				"Pattern was found but with different casing",
				"Consider using regex with '(?i)' prefix for case-insensitive matching",
			)
		}

		// Check for word boundary issues
		if strings.Contains(expectedPattern, " ") && strings.Contains(actualMessage, strings.ReplaceAll(expectedPattern, " ", "")) {
			suggestions = append(suggestions,
				"Pattern contains spaces but actual message may not",
				"Consider using regex with '\\s*' to match optional whitespace",
				"Or remove spaces from pattern for flexible matching",
			)
		}
	}

	// Pattern complexity suggestions
	if len(expectedPattern) > 50 {
		suggestions = append(suggestions,
			"Pattern is quite long and complex",
			"Consider breaking it into simpler patterns",
			"Test pattern incrementally to identify which part fails",
		)
	}

	// If no specific suggestions, provide generic ones
	if len(suggestions) == 0 {
		suggestions = append(suggestions,
			"Review the expected pattern and actual message content",
			"Check if pattern should be regex or substring matching",
			"Verify the actual message format matches expectations",
			"Consider using a more flexible pattern if multiple message formats are possible",
		)
	}

	return suggestions
}

// extractResponseSnippet creates a truncated snippet of the response body for error messages.
// It limits the snippet to 200 characters to keep error messages readable.
func extractResponseSnippet(response []byte) string {
	// Convert to string and truncate
	responseStr := string(response)
	maxLen := 200
	if len(responseStr) > maxLen {
		responseStr = responseStr[:maxLen] + "..."
	}
	// Replace newlines with spaces for single-line error message
	responseStr = strings.ReplaceAll(responseStr, "\n", " ")
	responseStr = strings.ReplaceAll(responseStr, "\r", " ")
	return responseStr
}

// ValidateErrorMessagePatternWithConfig validates error messages using advanced pattern configuration.
//
// This function provides flexible error message validation with support for:
// - Custom field selection
// - Case-insensitive matching
// - Match-any vs match-all semantics
// - Multiple pattern checking
//
// Parameters:
//   - bodyBytes: Raw response body bytes
//   - config: Pattern matching configuration
//
// Returns:
//   - true if validation succeeds, false otherwise
//   - error if configuration is invalid or JSON parsing fails
//
// Example usage:
//
//	config := &ErrorMessagePattern{
//	    Pattern:         "unauthorized",
//	    CaseInsensitive: true,
//	    FieldNames:      []string{"error", "detail"},
//	}
//	matches, err := ValidateErrorMessagePatternWithConfig(body, config)
func ValidateErrorMessagePatternWithConfig(bodyBytes []byte, config *ErrorMessagePattern) (bool, error) {
	if len(bodyBytes) == 0 {
		return false, fmt.Errorf("response body is empty")
	}

	if config == nil {
		return false, fmt.Errorf("config cannot be nil")
	}

	if config.Pattern == "" {
		return false, fmt.Errorf("pattern cannot be empty")
	}

	// Parse JSON body
	var body map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		return false, fmt.Errorf("failed to parse response body: %w", err)
	}

	// Compile regex pattern
	regexPattern := config.Pattern
	if config.CaseInsensitive {
		regexPattern = "(?i)" + config.Pattern
	}

	re, err := regexp.Compile(regexPattern)
	if err != nil {
		return false, fmt.Errorf("invalid regex pattern: %w", err)
	}

	// Determine which fields to check
	fieldsToCheck := config.FieldNames
	if len(fieldsToCheck) == 0 {
		fieldsToCheck = []string{"error", "message", "detail", "description"}
	}

	// Check each specified field
	matchedFields := []string{}
	for _, field := range fieldsToCheck {
		if value, exists := body[field]; exists {
			if strValue, ok := value.(string); ok && strValue != "" {
				if re.MatchString(strValue) {
					matchedFields = append(matchedFields, field)
					if config.MatchAny {
						return true, nil
					}
				}
			}
			// Check nested error objects
			if nestedObj, ok := value.(map[string]interface{}); ok {
				if nestedValue, exists := nestedObj["message"]; exists {
					if strValue, ok := nestedValue.(string); ok && strValue != "" {
						if re.MatchString(strValue) {
							matchedFields = append(matchedFields, field)
							if config.MatchAny {
								return true, nil
							}
						}
					}
				}
			}
		}
	}

	// For match-all semantics, require all specified fields to match
	if !config.MatchAny {
		return len(matchedFields) == len(fieldsToCheck), nil
	}

	return false, nil
}

// =============================================================================
// SPECIFIC ERROR CODE VALIDATION
// =============================================================================

// ErrorCodeInResponse checks if a response body contains a specific error code.
//
// This function searches for error codes in common error response fields.
// It supports both simple string error codes and nested error objects.
//
// Parameters:
//   - bodyBytes: Raw response body bytes
//   - expectedCode: The error code to look for (e.g., "INVALID_TOKEN", "AUTH_FAILED")
//
// Returns:
//   - true if the error code is found in the response, false otherwise
//   - error if JSON parsing fails
//
// Example usage:
//
//	// Check for "AUTH_FAILED" error code
//	found, err := ErrorCodeInResponse(body, "AUTH_FAILED")
//
//	// Check for OAuth "invalid_token" error
//	found, err := ErrorCodeInResponse(body, "invalid_token")
func ErrorCodeInResponse(bodyBytes []byte, expectedCode string) (bool, error) {
	if len(bodyBytes) == 0 {
		return false, fmt.Errorf("response body is empty")
	}

	if expectedCode == "" {
		return false, fmt.Errorf("expected code cannot be empty")
	}

	// Parse JSON body
	var body map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		return false, fmt.Errorf("failed to parse response body: %w", err)
	}

	// Common error code field names
	errorCodeFields := []string{"error_code", "errorCode", "code", "error", "status"}

	// Check for error code in standard fields
	for _, field := range errorCodeFields {
		if value, exists := body[field]; exists {
			// Check for string match
			if strValue, ok := value.(string); ok {
				if strings.EqualFold(strValue, expectedCode) {
					return true, nil
				}
			}
			// Check for nested error code in error object
			if nestedObj, ok := value.(map[string]interface{}); ok {
				if codeValue, exists := nestedObj["code"]; exists {
					if strValue, ok := codeValue.(string); ok {
						if strings.EqualFold(strValue, expectedCode) {
							return true, nil
						}
					}
				}
			}
		}
	}

	// Check for OAuth-style error field (contains error code directly)
	if errorCode, exists := body["error"]; exists {
		if strValue, ok := errorCode.(string); ok {
			if strings.EqualFold(strValue, expectedCode) {
				return true, nil
			}
		}
	}

	return false, nil
}

// ValidateStatusCodeAndErrorCode validates both status code and error code in a single check.
//
// This is a convenience function that validates HTTP status code and error code
// together, ensuring that both the HTTP layer and application layer error
// information are consistent.
//
// Parameters:
//   - resp: The HTTP response to validate
//   - expectedStatus: Expected HTTP status code
//   - expectedCode: Expected error code in response body
//
// Returns:
//   - true if both status code and error code match, false otherwise
//   - error if body parsing fails
//
// Example usage:
//
//	// Validate 401 response with "UNAUTHORIZED" error code
//	valid, err := ValidateStatusCodeAndErrorCode(resp, 401, "UNAUTHORIZED")
//
//	// Validate 403 response with "FORBIDDEN" error code
//	valid, err := ValidateStatusCodeAndErrorCode(resp, 403, "FORBIDDEN")
func ValidateStatusCodeAndErrorCode(resp *http.Response, expectedStatus int, expectedCode string) (bool, error) {
	if resp == nil {
		return false, fmt.Errorf("response is nil")
	}

	// Check status code
	if resp.StatusCode != expectedStatus {
		return false, nil
	}

	// Read response body
	var bodyBytes []byte
	if resp.Body != nil {
		defer resp.Body.Close()
		var err error
		bodyBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			return false, fmt.Errorf("failed to read response body: %w", err)
		}
	}

	// Check error code
	found, err := ErrorCodeInResponse(bodyBytes, expectedCode)
	if err != nil {
		return false, err
	}

	return found, nil
}

// GetErrorMessage extracts the error message from a response body.
//
// This helper function searches for error messages in common fields and returns
// the first non-empty message found.
//
// Parameters:
//   - bodyBytes: Raw response body bytes
//
// Returns:
//   - The extracted error message, or empty string if not found
//   - error if JSON parsing fails
//
// Example usage:
//
//	message, err := GetErrorMessage(body)
//	if message != "" {
//	    fmt.Printf("Error: %s\n", message)
//	}
func GetErrorMessage(bodyBytes []byte) (string, error) {
	if len(bodyBytes) == 0 {
		return "", fmt.Errorf("response body is empty")
	}

	// Parse JSON body
	var body map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		return "", fmt.Errorf("failed to parse response body: %w", err)
	}

	// Check common message fields
	messageFields := []string{"error", "message", "detail", "description", "error_description"}
	for _, field := range messageFields {
		if value, exists := body[field]; exists {
			if strValue, ok := value.(string); ok && strValue != "" {
				return strValue, nil
			}
			// Check nested message in error object
			if nestedObj, ok := value.(map[string]interface{}); ok {
				if messageValue, exists := nestedObj["message"]; exists {
					if strValue, ok := messageValue.(string); ok && strValue != "" {
						return strValue, nil
					}
				}
			}
		}
	}

	return "", nil
}

// GetErrorCode extracts the error code from a response body.
//
// This helper function searches for error codes in common fields and returns
// the first non-empty code found.
//
// Parameters:
//   - bodyBytes: Raw response body bytes
//
// Returns:
//   - The extracted error code, or empty string if not found
//   - error if JSON parsing fails
//
// Example usage:
//
//	code, err := GetErrorCode(body)
//	if code != "" {
//	    fmt.Printf("Error code: %s\n", code)
//	}
func GetErrorCode(bodyBytes []byte) (string, error) {
	if len(bodyBytes) == 0 {
		return "", fmt.Errorf("response body is empty")
	}

	// Parse JSON body
	var body map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		return "", fmt.Errorf("failed to parse response body: %w", err)
	}

	// Check common code fields
	codeFields := []string{"error_code", "errorCode", "code", "error"}
	for _, field := range codeFields {
		if value, exists := body[field]; exists {
			// Direct string code
			if strValue, ok := value.(string); ok && strValue != "" {
				return strValue, nil
			}
			// Check nested code in error object
			if nestedObj, ok := value.(map[string]interface{}); ok {
				if codeValue, exists := nestedObj["code"]; exists {
					if strValue, ok := codeValue.(string); ok && strValue != "" {
						return strValue, nil
					}
				}
			}
		}
	}

	return "", nil
}

// =============================================================================
// DETAILED VALIDATION RESULTS
// =============================================================================

// StatusCodeValidationResult provides detailed information about status code validation.
// It includes not just whether validation passed, but also the expected vs actual values
// and detailed mismatch information to help diagnose validation failures.
type StatusCodeValidationResult struct {
	// Valid indicates whether the validation passed
	Valid bool
	// ActualCode is the HTTP status code from the response
	ActualCode int
	// ExpectedCodes contains the expected status code(s) that were validated against
	ExpectedCodes []int
	// MatchedCode is the specific expected code that matched (if any)
	MatchedCode *int
	// MismatchDetails contains human-readable information about why validation failed
	MismatchDetails string
	// IsClientError indicates whether the actual code is a client error (4xx)
	IsClientError bool
	// IsServerError indicates whether the actual code is a server error (5xx)
	IsServerError bool
	// Category describes the general category of the actual status code
	Category string
}

// ValidateStatusCodeWithDetails performs comprehensive status code validation with detailed results.
// Unlike HTTPStatusCodeIsValid which returns only a boolean, this function provides detailed information
// about the validation process, making it easier to diagnose why validation failed.
//
// Parameters:
//   - resp: The HTTP response to validate
//   - expected: Expected status code(s) - can be a single int or []int
//   - requestContext: Optional context about the request (e.g., "POST /api/users", "GET /api/resource/123")
//
// Returns a StatusCodeValidationResult with detailed validation information.
//
// Example usage:
//
//	result := ValidateStatusCodeWithDetails(response, []int{200, 201, 204}, "POST /api/users")
//	if !result.Valid {
//	    log.Printf("Status validation failed: %s", result.MismatchDetails)
//	    log.Printf("Got: %d, expected one of: %v", result.ActualCode, result.ExpectedCodes)
//	}
//
//	// Single code validation without context
//	result := ValidateStatusCodeWithDetails(response, 200, "")
//	if result.Valid {
//	    log.Printf("Status code is valid: %d", result.ActualCode)
//	}
func ValidateStatusCodeWithDetails(resp *http.Response, expected interface{}, requestContext ...string) StatusCodeValidationResult {
	result := StatusCodeValidationResult{
		Valid:             false,
		ActualCode:        0,
		ExpectedCodes:     []int{},
		MatchedCode:       nil,
		MismatchDetails:   "",
		IsClientError:     false,
		IsServerError:     false,
		Category:          "unknown",
	}

	if resp == nil {
		result.MismatchDetails = "response is nil"
		return result
	}

	result.ActualCode = resp.StatusCode

	// Set error flags and category
	switch {
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		result.IsClientError = true
		result.Category = "client_error"
	case resp.StatusCode >= 500 && resp.StatusCode < 600:
		result.IsServerError = true
		result.Category = "server_error"
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		result.Category = "success"
	case resp.StatusCode >= 300 && resp.StatusCode < 400:
		result.Category = "redirection"
	default:
		result.Category = "other"
	}

	// Parse expected codes
	var expectedCodes []int
	switch v := expected.(type) {
	case int:
		expectedCodes = []int{v}
	case []int:
		expectedCodes = v
	default:
		result.MismatchDetails = fmt.Sprintf("invalid expected type: %T", expected)
		return result
	}

	result.ExpectedCodes = expectedCodes

	// Check for match
	for _, code := range expectedCodes {
		if resp.StatusCode == code {
			result.Valid = true
			result.MatchedCode = &code
			result.MismatchDetails = ""
			return result
		}
	}

	// Extract optional context
	context := ""
	if len(requestContext) > 0 && requestContext[0] != "" {
		context = requestContext[0]
	}

	// Build comprehensive mismatch information with suggestions
	result.MismatchDetails = buildComprehensiveMismatchDetails(
		expectedCodes, resp.StatusCode, result.Category, result.IsClientError, result.IsServerError, context)

	return result
}

// buildComprehensiveMismatchDetails creates detailed mismatch information including
// status code category, context, and specific suggestions based on the actual code.
// It uses FormatValidationErrorWithDetails for consistent formatting across the codebase.
func buildComprehensiveMismatchDetails(expectedCodes []int, actualCode int, category string, isClientError, isServerError bool, context string) string {
	// Build validation details that include status code category information
	var validationDetails []string

	// Add status code category information to validation details
	categoryDesc := getCategoryDescription(category)
	if categoryDesc != "" {
		validationDetails = append(validationDetails, fmt.Sprintf("Status code category: %s", categoryDesc))
	}

	// Prepare range info with category details
	var rangeInfo string
	if isClientError {
		rangeInfo = "400-499 (Client Error)"
	} else if isServerError {
		rangeInfo = "500-599 (Server Error)"
	} else if category == "success" {
		rangeInfo = "200-299 (Success)"
	} else if category == "redirection" {
		rangeInfo = "300-399 (Redirection)"
	}

	// Determine expected value for FormatValidationErrorWithDetails
	var expected interface{}
	if len(expectedCodes) == 1 {
		expected = expectedCodes[0]
	} else {
		expected = expectedCodes
	}

	// Use FormatValidationErrorWithDetails for consistent error formatting
	validationErr := FormatValidationErrorWithDetails(
		"status_code",
		expected,
		actualCode,
		context,
		"", // responseSnippet - not needed for status code validation
		"", // fieldName - not applicable for status code validation
		"", // location - not applicable for status code validation
		nil, // relatedFields - not applicable for status code validation
		"", // patternDetails - not applicable for status code validation
		rangeInfo,
		validationDetails,
	)

	// Return the formatted error string
	return validationErr.Error()
}

// getCategoryDescription returns a human-readable description for a status code category.
func getCategoryDescription(category string) string {
	descriptions := map[string]string{
		"success":       "2xx - Successful responses",
		"redirection":   "3xx - Redirection messages",
		"client_error":  "4xx - Client error responses",
		"server_error":  "5xx - Server error responses",
		"other":         "Non-standard status codes",
	}
	return descriptions[category]
}

// getStatusCodeSpecificSuggestions returns relevant suggestions based on the specific status code received.
func getStatusCodeSpecificSuggestions(code int) []string {
	switch code {
	case 400:
		return []string{
			"Check request body syntax and formatting",
			"Verify all required fields are present",
			"Ensure Content-Type header matches request body format",
			"Validate JSON schema if applicable",
		}
	case 401:
		return []string{
			"Verify authentication credentials are correct",
			"Check if API token or session has expired",
			"Ensure Authorization header is properly formatted (e.g., 'Bearer <token>')",
			"Confirm authentication method is supported",
		}
	case 403:
		return []string{
			"Verify your account has permission to access this resource",
			"Check if additional scopes or roles are required",
			"Review API documentation for required permissions",
			"Ensure proper OAuth scopes are granted",
		}
	case 404:
		return []string{
			"Verify the endpoint URL is correct",
			"Check if the resource ID or identifier exists",
			"Ensure the resource hasn't been deleted or moved",
			"Review API versioning (URL path may have changed)",
		}
	case 409:
		return []string{
			"Check if a resource with this identifier already exists",
			"Verify unique constraints on the resource",
			"Consider using PUT instead of POST for updates",
			"Review state transition rules for the resource",
		}
	case 429:
		return []string{
			"Implement rate limiting and exponential backoff",
			"Check API quota limits and current usage",
			"Consider caching responses to reduce request frequency",
			"Review rate limit headers (Retry-After, X-RateLimit-Remaining)",
		}
	case 500:
		return []string{
			"Implement retry logic with exponential backoff",
			"Check service status page for ongoing issues",
			"Contact support if the issue persists",
			"Verify request doesn't trigger server-side bugs",
		}
	case 502:
		return []string{
			"Check if upstream services are operational",
			"Implement retry logic for transient failures",
			"Verify network connectivity to upstream services",
			"Review load balancer configuration",
		}
	case 503:
		return []string{
			"Implement retry with exponential backoff",
			"Check service maintenance schedules",
			"Verify service capacity and scaling",
			"Review circuit breaker patterns",
		}
	case 504:
		return []string{
			"Implement timeout and retry logic",
			"Check if upstream services are responding slowly",
			"Review request complexity and data size",
			"Verify network latency and connectivity",
		}
	default:
		// Generic suggestions for other codes
		if code >= 400 && code < 500 {
			return []string{
				"Review request parameters and body",
				"Check API documentation for requirements",
				"Verify all headers are correctly set",
				"Ensure request complies with API contracts",
			}
		}
		if code >= 500 && code < 600 {
			return []string{
				"Implement retry logic with exponential backoff",
				"Check service status page for ongoing issues",
				"Contact support if the issue persists",
				"Review server logs if accessible",
			}
		}
	}
	return []string{}
}

// StatusCodeRange defines a range of status codes for flexible validation.
// This is useful when you want to accept any status code within a certain range.
type StatusCodeRange struct {
	// Min is the minimum status code in the range (inclusive)
	Min int
	// Max is the maximum status code in the range (inclusive)
	Max int
	// Description is a human-readable description of what this range represents
	Description string
}

// Common status code ranges
var (
	// Range1xx covers informational responses (100-199)
	Range1xx = StatusCodeRange{Min: 100, Max: 199, Description: "Informational"}
	// Range2xx covers successful responses (200-299)
	Range2xx = StatusCodeRange{Min: 200, Max: 299, Description: "Success"}
	// Range3xx covers redirection responses (300-399)
	Range3xx = StatusCodeRange{Min: 300, Max: 399, Description: "Redirection"}
	// Range4xx covers client error responses (400-499)
	Range4xx = StatusCodeRange{Min: 400, Max: 499, Description: "Client Error"}
	// Range5xx covers server error responses (500-599)
	Range5xx = StatusCodeRange{Min: 500, Max: 599, Description: "Server Error"}
)

// ValidateStatusCodeRange checks if a response's status code falls within a specified range.
// This is useful for flexible validation where you want to accept any status code within a category.
//
// Parameters:
//   - resp: The HTTP response to validate
//   - range: The status code range to validate against
//
// Returns true if the response status code falls within the range, false otherwise.
// Returns false if the response is nil or the range is invalid (Min > Max).
//
// Example usage:
//
//	// Accept any success code (2xx)
//	if ValidateStatusCodeRange(response, Range2xx) {
//	    // Response is successful (200-299)
//	}
//
//	// Accept any client error (4xx)
//	if ValidateStatusCodeRange(response, Range4xx) {
//	    // Response is a client error (400-499)
//	}
//
//	// Custom range for specific validation
//	customRange := StatusCodeRange{Min: 200, Max: 204, Description: "Standard success"}
//	if ValidateStatusCodeRange(response, customRange) {
//	    // Response is in custom range
//	}
func ValidateStatusCodeRange(resp *http.Response, statusRange StatusCodeRange) bool {
	if resp == nil {
		return false
	}

	if statusRange.Min > statusRange.Max {
		return false
	}

	return resp.StatusCode >= statusRange.Min && resp.StatusCode <= statusRange.Max
}

// ValidateStatusCodeRangeWithDetails provides detailed status code range validation.
// Like ValidateStatusCodeRange but with comprehensive result information including
// range boundaries, distance from valid range, and contextual suggestions.
//
// Parameters:
//   - resp: The HTTP response to validate
//   - range: The status code range to validate against
//
// Returns a StatusCodeValidationResult with detailed information about the range validation,
// including expected range boundaries, actual status code, distance from valid range,
// range pattern context (e.g., '4xx', '5xx'), and suggestions for fixing the issue.
//
// Example usage:
//
//	result := ValidateStatusCodeRangeWithDetails(response, Range4xx)
//	if !result.Valid {
//	    log.Printf("Status code %d is not in range %s (%d-%d)",
//	        result.ActualCode, result.Category, range.Min, range.Max)
//	    // MismatchDetails now includes:
//	    // - Expected range with description (e.g., "400-499 Client Error")
//	    // - Actual received status code
//	    // - Distance from valid range (e.g., "99 codes below range")
//	    // - Range pattern context (e.g., "Validating 4xx pattern")
//	    // - Specific suggestions based on the actual code
//	}
func ValidateStatusCodeRangeWithDetails(resp *http.Response, statusRange StatusCodeRange) StatusCodeValidationResult {
	result := StatusCodeValidationResult{
		Valid:           false,
		ActualCode:      0,
		ExpectedCodes:   []int{statusRange.Min, statusRange.Max},
		MatchedCode:     nil,
		MismatchDetails: "",
		IsClientError:   false,
		IsServerError:   false,
		Category:        statusRange.Description,
	}

	if resp == nil {
		result.MismatchDetails = "response is nil"
		return result
	}

	if statusRange.Min > statusRange.Max {
		result.MismatchDetails = fmt.Sprintf("invalid range: Min (%d) > Max (%d)", statusRange.Min, statusRange.Max)
		return result
	}

	result.ActualCode = resp.StatusCode

	// Check range
	inRange := resp.StatusCode >= statusRange.Min && resp.StatusCode <= statusRange.Max
	result.Valid = inRange

	// Set error flags
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		result.IsClientError = true
	}
	if resp.StatusCode >= 500 && resp.StatusCode < 600 {
		result.IsServerError = true
	}

	if !inRange {
		// Build comprehensive mismatch details with range boundaries, distance, and suggestions
		result.MismatchDetails = buildRangeMismatchDetails(resp.StatusCode, statusRange)
	}

	return result
}

// buildRangeMismatchDetails creates comprehensive mismatch information for range validation failures.
// This function calculates the distance from the valid range, identifies the range pattern context,
// and generates detailed error messages with suggestions using FormatValidationErrorWithDetails.
func buildRangeMismatchDetails(actualCode int, statusRange StatusCodeRange) string {
	// Calculate distance from valid range
	var distanceInfo string
	var distanceDetails []string

	if actualCode < statusRange.Min {
		distance := statusRange.Min - actualCode
		if distance == 1 {
			distanceInfo = fmt.Sprintf("%d code below range", distance)
			distanceDetails = append(distanceDetails, fmt.Sprintf("Status code %d is %d code below the minimum %d", actualCode, distance, statusRange.Min))
		} else {
			distanceInfo = fmt.Sprintf("%d codes below range", distance)
			distanceDetails = append(distanceDetails, fmt.Sprintf("Status code %d is %d codes below the minimum %d", actualCode, distance, statusRange.Min))
		}
	} else {
		distance := actualCode - statusRange.Max
		if distance == 1 {
			distanceInfo = fmt.Sprintf("%d code above range", distance)
			distanceDetails = append(distanceDetails, fmt.Sprintf("Status code %d is %d code above the maximum %d", actualCode, distance, statusRange.Max))
		} else {
			distanceInfo = fmt.Sprintf("%d codes above range", distance)
			distanceDetails = append(distanceDetails, fmt.Sprintf("Status code %d is %d codes above the maximum %d", actualCode, distance, statusRange.Max))
		}
	}

	// Determine range pattern context (e.g., '4xx', '5xx')
	rangePattern := getRangePattern(statusRange)

	// Build expected range with description
	expectedRange := fmt.Sprintf("%d-%d", statusRange.Min, statusRange.Max)
	if statusRange.Description != "" {
		expectedRange = fmt.Sprintf("%s (%s)", expectedRange, statusRange.Description)
	}

	// Add pattern context to expected range
	if rangePattern != "" {
		expectedRange = fmt.Sprintf("%s - %s pattern", expectedRange, rangePattern)
	}

	// Build validation details
	validationDetails := []string{
		fmt.Sprintf("Expected range: %d-%d %s", statusRange.Min, statusRange.Max, statusRange.Description),
		fmt.Sprintf("Actual status code: %d", actualCode),
		fmt.Sprintf("Distance from valid range: %s", distanceInfo),
		fmt.Sprintf("Range pattern context: %s", rangePattern),
	}
	validationDetails = append(validationDetails, distanceDetails...)

	// Use FormatValidationErrorWithDetails for consistent formatting
	validationErr := FormatValidationErrorWithDetails(
		"status_code_range",
		expectedRange,
		actualCode,
		"", // context - not applicable for range validation
		"", // responseSnippet - not needed for status code validation
		"", // fieldName - not applicable for status code validation
		"", // location - not applicable for status code validation
		nil, // relatedFields - not applicable for status code validation
		fmt.Sprintf("Range pattern: %s, Distance: %s", rangePattern, distanceInfo),
		expectedRange,
		validationDetails,
	)

	return validationErr.Error()
}

// getRangePattern identifies the range pattern from a StatusCodeRange.
// Returns patterns like '4xx', '5xx', '2xx', etc. based on the range boundaries.
func getRangePattern(statusRange StatusCodeRange) string {
	min := statusRange.Min
	max := statusRange.Max

	// Check for standard century ranges
	if min/100 == max/100 && min%100 == 0 && max%100 == 99 {
		century := min / 100
		switch century {
		case 1:
			return "1xx (Informational)"
		case 2:
			return "2xx (Success)"
		case 3:
			return "3xx (Redirection)"
		case 4:
			return "4xx (Client Error)"
		case 5:
			return "5xx (Server Error)"
		}
	}

	// For custom ranges, provide the pattern context
	if min/100 == max/100 {
		century := min / 100
		return fmt.Sprintf("%dxx range (custom)", century)
	}

	return fmt.Sprintf("%d-%d range (custom)", min/100, max/100)
}

// =============================================================================
// ENHANCED ERROR MESSAGE VALIDATION
// =============================================================================

// EnhancedErrorMessagePattern defines a pattern for matching error messages in response bodies.
type EnhancedErrorMessagePattern struct {
	// FieldNames specifies which fields to check in the response body
	FieldNames *ErrorResponseFieldNames
	// Pattern is a regular expression pattern that the error message must match
	Pattern string
	// CaseInsensitive indicates whether pattern matching should be case-insensitive
	CaseInsensitive bool
	// MustContain specifies strings that must be present in the error message
	MustContain []string
	// MustNotContain specifies strings that must NOT be present in the error message
	MustNotContain []string
	// MinLength is the minimum length of the error message (0 for no minimum)
	MinLength int
	// MaxLength is the maximum length of the error message (0 for no maximum)
	MaxLength int
}

// ErrorMessageValidationResult provides detailed information about error message validation.
type ErrorMessageValidationResult struct {
	// Valid indicates whether the error message validation passed
	Valid bool
	// Found indicates whether an error message field was found in the response
	Found bool
	// Message is the actual error message content that was found
	Message string
	// FieldName is the name of the field where the error message was found
	FieldName string
	// PatternMatched indicates whether the regex pattern matched
	PatternMatched bool
	// MustContainResults shows which required strings were found
	MustContainResults map[string]bool
	// MustNotContainResults shows which forbidden strings were found (should all be false)
	MustNotContainResults map[string]bool
	// LengthValidation indicates whether the message length was valid
	LengthValidation bool
	// Issues contains a list of validation issues found
	Issues []string
}

// ValidateErrorMessageWithDetails performs comprehensive error message content validation.
// It checks for the presence of error messages and validates their content against various criteria.
//
// Parameters:
//   - responseBody: The parsed response body (should be a map[string]interface{})
//   - pattern: The validation pattern to apply
//
// Returns an ErrorMessageValidationResult with detailed validation information.
//
// Example usage:
//
//	body, _ := parseJSON(response.Body)
//	pattern := EnhancedErrorMessagePattern{
//	    FieldNames: DefaultErrorResponseFieldNames(),
//	    MustContain: []string{"invalid", "token"},
//	    MustNotContain: []string{"password", "secret"},
//	    MinLength: 10,
//	}
//	result := ValidateErrorMessageWithDetails(body, pattern)
//	if !result.Valid {
//	    log.Printf("Error message validation failed: %v", result.Issues)
//	}
func ValidateErrorMessageWithDetails(responseBody interface{}, pattern EnhancedErrorMessagePattern) ErrorMessageValidationResult {
	result := ErrorMessageValidationResult{
		Valid:                 false,
		Found:                 false,
		Message:               "",
		FieldName:             "",
		PatternMatched:        false,
		MustContainResults:    make(map[string]bool),
		MustNotContainResults: make(map[string]bool),
		LengthValidation:      true,
		Issues:                []string{},
	}

	if responseBody == nil {
		result.Issues = append(result.Issues, "response body is nil")
		return result
	}

	// Use default field names if not provided
	fieldNames := pattern.FieldNames
	if fieldNames == nil {
		fieldNames = DefaultErrorResponseFieldNames()
	}

	// Try to convert to map
	respMap, ok := responseBody.(map[string]interface{})
	if !ok {
		result.Issues = append(result.Issues, "response body is not a map")
		return result
	}

	// Search for error message in specified fields
	var foundMessage string
	var foundFieldName string

	// Check primary field
	if fieldNames.PrimaryFieldName != "" {
		if value, exists := respMap[fieldNames.PrimaryFieldName]; exists {
			if strValue, ok := value.(string); ok && strValue != "" {
				foundMessage = strValue
				foundFieldName = fieldNames.PrimaryFieldName
			}
		}
	}

	// Check secondary field if not found in primary
	if foundMessage == "" && fieldNames.SecondaryFieldName != "" {
		if value, exists := respMap[fieldNames.SecondaryFieldName]; exists {
			if strValue, ok := value.(string); ok && strValue != "" {
				foundMessage = strValue
				foundFieldName = fieldNames.SecondaryFieldName
			}
		}
	}

	result.Found = foundMessage != ""
	result.Message = foundMessage
	result.FieldName = foundFieldName

	if !result.Found {
		result.Issues = append(result.Issues, "no valid error message field found")
		return result
	}

	// Validate length
	if pattern.MinLength > 0 && len(foundMessage) < pattern.MinLength {
		result.LengthValidation = false
		result.Issues = append(result.Issues, fmt.Sprintf("message length %d is less than minimum %d", len(foundMessage), pattern.MinLength))
	}

	if pattern.MaxLength > 0 && len(foundMessage) > pattern.MaxLength {
		result.LengthValidation = false
		result.Issues = append(result.Issues, fmt.Sprintf("message length %d exceeds maximum %d", len(foundMessage), pattern.MaxLength))
	}

	// Validate regex pattern
	if pattern.Pattern != "" {
		regex := regexp.MustCompile(pattern.Pattern)
		if pattern.CaseInsensitive {
			regex = regexp.MustCompile("(?i)" + pattern.Pattern)
		}
		result.PatternMatched = regex.MatchString(foundMessage)
		if !result.PatternMatched {
			result.Issues = append(result.Issues, fmt.Sprintf("message does not match required pattern: %s", pattern.Pattern))
		}
	} else {
		result.PatternMatched = true // No pattern specified, consider it matched
	}

	// Validate MustContain strings
	allMustContainFound := true
	for _, mustContain := range pattern.MustContain {
		var found bool
		if pattern.CaseInsensitive {
			// Case-insensitive search
			found = strings.Contains(strings.ToLower(foundMessage), strings.ToLower(mustContain))
		} else {
			found = strings.Contains(foundMessage, mustContain)
		}
		result.MustContainResults[mustContain] = found
		if !found {
			allMustContainFound = false
			result.Issues = append(result.Issues, fmt.Sprintf("message does not contain required string: %s", mustContain))
		}
	}

	// Validate MustNotContain strings
	hasForbiddenStrings := false
	for _, mustNotContain := range pattern.MustNotContain {
		var found bool
		if pattern.CaseInsensitive {
			// Case-insensitive search
			found = strings.Contains(strings.ToLower(foundMessage), strings.ToLower(mustNotContain))
		} else {
			found = strings.Contains(foundMessage, mustNotContain)
		}
		result.MustNotContainResults[mustNotContain] = found
		if found {
			hasForbiddenStrings = true
			result.Issues = append(result.Issues, fmt.Sprintf("message contains forbidden string: %s", mustNotContain))
		}
	}

	// Overall validity
	result.Valid = result.Found && result.LengthValidation && result.PatternMatched &&
		allMustContainFound && !hasForbiddenStrings

	return result
}

// =============================================================================
// ERROR CODE DETECTION
// =============================================================================

// ErrorCodeMatch represents a found error code in a response.
type ErrorCodeMatch struct {
	// FieldName is the name of the field where the error code was found
	FieldName string
	// CodeValue is the actual error code value (as a string for flexibility)
	CodeValue string
	// NumericCode is the error code parsed as an integer (if applicable)
	NumericCode *int
	// MatchedPattern is the pattern that matched this code
	MatchedPattern string
	// Position describes where in the response the code was found
	Position string
}

// EnhancedErrorCodePattern defines a pattern for finding error codes in responses.
type EnhancedErrorCodePattern struct {
	// FieldNames specifies which fields to check for error codes
	FieldNames []string
	// NumericOnly indicates whether to only match numeric error codes
	NumericOnly bool
	// ValuePatterns contains regex patterns for valid error code values
	ValuePatterns []string
}

// FindErrorCodesInResponse searches for error codes in an HTTP response body.
// This is useful for validating that specific error codes are present in error responses.
//
// Parameters:
//   - responseBody: The parsed response body (should be a map[string]interface{})
//   - patterns: The patterns to use for finding error codes
//
// Returns a slice of ErrorCodeMatch representing all found error codes.
//
// Example usage:
//
//	body, _ := parseJSON(response.Body)
//	patterns := []EnhancedErrorCodePattern{
//	    {
//	        FieldNames: []string{"error_code", "code"},
//	        NumericOnly: true,
//	    },
//	    {
//	        FieldNames: []string{"error"},
//	        ValuePatterns: []string{"invalid_token", "access_denied"},
//	    },
//	}
//	matches := FindErrorCodesInResponse(body, patterns)
//	for _, match := range matches {
//	    log.Printf("Found error code: %s in field: %s", match.CodeValue, match.FieldName)
//	}
func FindErrorCodesInResponse(responseBody interface{}, patterns []EnhancedErrorCodePattern) []ErrorCodeMatch {
	var matches []ErrorCodeMatch

	if responseBody == nil {
		return matches
	}

	// Try to convert to map
	respMap, ok := responseBody.(map[string]interface{})
	if !ok {
		return matches
	}

	for _, pattern := range patterns {
		// Default to common error code fields if none specified
		fieldsToCheck := pattern.FieldNames
		if len(fieldsToCheck) == 0 {
			fieldsToCheck = []string{"error_code", "code", "error", "status"}
		}

		for _, fieldName := range fieldsToCheck {
			if value, exists := respMap[fieldName]; exists {
				var codeValue string
				var numericCode *int
				var matchedPattern string

				// Handle different value types
				switch v := value.(type) {
				case string:
					codeValue = v
					matchedPattern = "string"
					// Try to parse as number
					if num, err := parseIntFromString(v); err == nil && pattern.NumericOnly {
						numericCode = &num
					}
				case int:
					codeValue = fmt.Sprintf("%d", v)
					numericCode = &v
					matchedPattern = "numeric"
				case float64:
					// JSON numbers are often parsed as float64
					if v == float64(int(v)) {
						num := int(v)
						codeValue = fmt.Sprintf("%d", num)
						numericCode = &num
						matchedPattern = "numeric"
					}
				case bool:
					// Some APIs use true/false as error codes
					if v {
						codeValue = "true"
						matchedPattern = "boolean"
					} else {
						codeValue = "false"
						matchedPattern = "boolean"
					}
				default:
					continue // Skip unsupported types
				}

				// Validate against value patterns if specified
				patternsMatched := true
				if len(pattern.ValuePatterns) > 0 {
					patternsMatched = false
					for _, valuePattern := range pattern.ValuePatterns {
						matched, _ := regexp.MatchString(valuePattern, codeValue)
						if matched {
							patternsMatched = true
							matchedPattern = valuePattern
							break
						}
					}
				}

				// Check numeric-only constraint
				if pattern.NumericOnly && numericCode == nil {
					patternsMatched = false
				}

				if patternsMatched && codeValue != "" {
					matches = append(matches, ErrorCodeMatch{
						FieldName:      fieldName,
						CodeValue:      codeValue,
						NumericCode:    numericCode,
						MatchedPattern: matchedPattern,
						Position:       fmt.Sprintf("field '%s' in response body", fieldName),
					})
				}
			}
		}
	}

	return matches
}

// ValidateErrorCodeInResponse checks if a specific error code is present in a response.
// This is a convenience wrapper around FindErrorCodesInResponse for checking a single code.
//
// Parameters:
//   - responseBody: The parsed response body
//   - expectedCode: The error code value to look for (as string)
//   - fieldName: The field name to check (empty string to check all common fields)
//
// Returns true if the expected error code is found, false otherwise.
//
// Example usage:
//
//	body, _ := parseJSON(response.Body)
//	hasInvalidToken := ValidateErrorCodeInResponse(body, "invalid_token", "error")
//	if hasInvalidToken {
//	    log.Printf("Response contains invalid_token error")
//	}
func ValidateErrorCodeInResponse(responseBody interface{}, expectedCode string, fieldName string) bool {
	if responseBody == nil {
		return false
	}

	var pattern EnhancedErrorCodePattern
	if fieldName != "" {
		pattern = EnhancedErrorCodePattern{
			FieldNames: []string{fieldName},
		}
	} else {
		pattern = EnhancedErrorCodePattern{
			FieldNames: []string{"error_code", "code", "error", "status"},
		}
	}

	matches := FindErrorCodesInResponse(responseBody, []EnhancedErrorCodePattern{pattern})
	for _, match := range matches {
		if match.CodeValue == expectedCode {
			return true
		}
	}

	return false
}

// parseIntFromString attempts to parse a string as an integer.
func parseIntFromString(s string) (int, error) {
	var result int
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}

// =============================================================================
// STATUS CODE RANGE VALIDATION (PATTERN-BASED)
// =============================================================================

// ValidateStatusCodeRangeInt validates a status code against a pattern string.
// It supports patterns like '4xx', '5xx', '3xx', '2xx', '1xx' to check if a status code
// falls within a specified range.
//
// Parameters:
//   - pattern: The pattern string (e.g., '4xx', '5xx', '2xx')
//   - actual: The actual status code to validate
//
// Returns error if the pattern is invalid or if the actual code doesn't match the range.
// Returns nil if the actual code falls within the specified range.
//
// Example usage:
//
//	// Validate 404 is in 4xx range
//	err := ValidateStatusCodeRangeInt("4xx", 404)
//	if err != nil {
//	    log.Printf("Status validation failed: %v", err)
//	}
//
//	// Validate 500 is in 5xx range
//	err = ValidateStatusCodeRangeInt("5xx", 500)
//
//	// This returns an error because 200 is not in 4xx range
//	err = ValidateStatusCodeRangeInt("4xx", 200)
func ValidateStatusCodeRangeInt(pattern string, actual int) error {
	// Validate pattern format
	if len(pattern) != 3 {
		return FormatValidationErrorWithDetails(
			"status_code_range",
			"pattern in format 'Nxx' (3 chars)",
			fmt.Sprintf("'%s' (%d chars)", pattern, len(pattern)),
			fmt.Sprintf("invalid pattern format: %s", pattern),
			"", // responseSnippet
			"", // fieldName
			"", // location
			nil, // relatedFields
			"", // patternDetails
			"", // rangeInfo
			[]string{
				fmt.Sprintf("Pattern '%s' has invalid length", pattern),
				"Pattern must be exactly 3 characters (e.g., '4xx', '5xx')",
				fmt.Sprintf("Got %d characters, expected 3", len(pattern)),
			},
		)
	}

	// Extract the century digit (first character)
	centuryChar := pattern[0]
	if centuryChar < '1' || centuryChar > '5' {
		return FormatValidationErrorWithDetails(
			"status_code_range",
			"century digit 1-5",
			fmt.Sprintf("'%c' (ASCII %d)", centuryChar, centuryChar),
			fmt.Sprintf("invalid pattern century in '%s'", pattern),
			"", // responseSnippet
			"", // fieldName
			"", // location
			nil, // relatedFields
			"", // patternDetails
			"", // rangeInfo
			[]string{
				fmt.Sprintf("Pattern '%s' has invalid century digit", pattern),
				"Valid century digits: 1 (1xx), 2 (2xx), 3 (3xx), 4 (4xx), 5 (5xx)",
				fmt.Sprintf("Got '%c', expected digit between '1' and '5'", centuryChar),
			},
		)
	}

	// Validate 'xx' suffix
	if pattern[1] != 'x' || pattern[2] != 'x' {
		return FormatValidationErrorWithDetails(
			"status_code_range",
			"pattern ending with 'xx'",
			fmt.Sprintf("'%s'", pattern[1:]),
			fmt.Sprintf("invalid pattern suffix in '%s'", pattern),
			"", // responseSnippet
			"", // fieldName
			"", // location
			nil, // relatedFields
			"", // patternDetails
			"", // rangeInfo
			[]string{
				fmt.Sprintf("Pattern '%s' has invalid suffix", pattern),
				"Pattern must end with 'xx' (e.g., '4xx', '5xx')",
				fmt.Sprintf("Got '%s', expected 'xx'", pattern[1:]),
			},
		)
	}

	// Calculate the range from the pattern
	century := int(centuryChar - '0')
	minRange := century * 100
	maxRange := minRange + 99

	// Get description for the range
	desc, _ := GetStatusCodeRangeDescription(pattern)

	// Check if actual code is within the range
	if actual < minRange || actual > maxRange {
		expectedRange := fmt.Sprintf("%s (%d-%d)", pattern, minRange, maxRange)
		if desc != "" {
			expectedRange = fmt.Sprintf("%s %s (%d-%d)", pattern, desc, minRange, maxRange)
		}

		// Calculate distance from valid range
		var distanceInfo string
		var validationDetails []string

		if actual < minRange {
			distance := minRange - actual
			distanceInfo = fmt.Sprintf("%d codes below range", distance)
			validationDetails = append(validationDetails,
				fmt.Sprintf("Expected range: %d-%d (%s)", minRange, maxRange, desc),
				fmt.Sprintf("Actual status code: %d", actual),
				fmt.Sprintf("Distance from valid range: %s", distanceInfo),
				fmt.Sprintf("Range pattern context: %s", pattern),
				fmt.Sprintf("Status code %d is %d codes below the minimum %d", actual, distance, minRange))
		} else {
			distance := actual - maxRange
			distanceInfo = fmt.Sprintf("%d codes above range", distance)
			validationDetails = append(validationDetails,
				fmt.Sprintf("Expected range: %d-%d (%s)", minRange, maxRange, desc),
				fmt.Sprintf("Actual status code: %d", actual),
				fmt.Sprintf("Distance from valid range: %s", distanceInfo),
				fmt.Sprintf("Range pattern context: %s", pattern),
				fmt.Sprintf("Status code %d is %d codes above the maximum %d", actual, distance, maxRange))
		}

		return FormatValidationErrorWithDetails(
			"status_code_range",
			expectedRange,
			actual,
			fmt.Sprintf("status code validation failed for pattern '%s'", pattern),
			"", // responseSnippet - not needed for status code validation
			"", // fieldName - not applicable for status code validation
			"", // location - not applicable for status code validation
			nil, // relatedFields - not applicable for status code validation
			fmt.Sprintf("Range pattern: %s, Distance: %s", pattern, distanceInfo),
			expectedRange,
			validationDetails,
		)
	}

	return nil
}

// ParseStatusCodeRange extracts the century digit and range from a pattern string.
// This is a helper function for advanced use cases where you need the range bounds.
//
// Parameters:
//   - pattern: The pattern string (e.g., '4xx', '5xx', '2xx')
//
// Returns the min and max values of the range, or an error if the pattern is invalid.
//
// Example usage:
//
//	min, max, err := ParseStatusCodeRange("4xx")
//	if err != nil {
//	    log.Printf("Invalid pattern: %v", err)
//	} else {
//	    fmt.Printf("Range: %d-%d\n", min, max)
//	}
func ParseStatusCodeRange(pattern string) (min int, max int, err error) {
	// Validate pattern format
	if len(pattern) != 3 {
		return 0, 0, fmt.Errorf("invalid pattern format: %s (expected format: '4xx', '5xx', etc.)", pattern)
	}

	// Extract the century digit (first character)
	centuryChar := pattern[0]
	if centuryChar < '1' || centuryChar > '5' {
		return 0, 0, fmt.Errorf("invalid pattern century: %c (must be 1-5)", centuryChar)
	}

	// Validate 'xx' suffix
	if pattern[1] != 'x' || pattern[2] != 'x' {
		return 0, 0, fmt.Errorf("invalid pattern suffix: %s (expected 'xx')", pattern)
	}

	// Calculate the range from the pattern
	century := int(centuryChar - '0')
	minRange := century * 100
	maxRange := minRange + 99

	return minRange, maxRange, nil
}

// GetStatusCodeRangeDescription returns a human-readable description for a status code range pattern.
//
// Parameters:
//   - pattern: The pattern string (e.g., '4xx', '5xx', '2xx')
//
// Returns a description string or an error if the pattern is invalid.
//
// Example usage:
//
//	desc, err := GetStatusCodeRangeDescription("4xx")
//	if err != nil {
//	    log.Printf("Invalid pattern: %v", err)
//	} else {
//	    fmt.Printf("Description: %s\n", desc)
//	}
func GetStatusCodeRangeDescription(pattern string) (string, error) {
	if len(pattern) != 3 {
		return "", fmt.Errorf("invalid pattern format: %s", pattern)
	}

	centuryChar := pattern[0]
	if centuryChar < '1' || centuryChar > '5' {
		return "", fmt.Errorf("invalid pattern century: %c", centuryChar)
	}

	if pattern[1] != 'x' || pattern[2] != 'x' {
		return "", fmt.Errorf("invalid pattern suffix: %s", pattern)
	}

	descriptions := map[byte]string{
		'1': "Informational",
		'2': "Success",
		'3': "Redirection",
		'4': "Client Error",
		'5': "Server Error",
	}

	desc, ok := descriptions[centuryChar]
	if !ok {
		return "", fmt.Errorf("unknown century: %c", centuryChar)
	}

	return fmt.Sprintf("%s (%s)", desc, pattern), nil
}

// =============================================================================
// HELPER FUNCTIONS FOR CREATING VALIDATION ERRORS
// =============================================================================

// FormatValidationError creates a standardized ValidationError with appropriate suggestions.
//
// This helper function automatically generates relevant suggestions based on the
// validation type and the mismatch between expected and actual values.
//
// Parameters:
//   - validationType: The type of validation (e.g., "status_code", "error_message")
//   - expected: The expected value
//   - actual: The actual value received
//   - context: Additional context (optional)
//   - responseSnippet: Excerpt from response (optional)
//   - customSuggestions: Optional custom suggestions (if provided, these override auto-generated suggestions)
//
// Returns a ValidationError with appropriate suggestions.
//
// Example usage with auto-generated suggestions:
//
//	err := FormatValidationError("status_code", 200, 404, "GET /api/users", "")
//
// Example usage with custom suggestions:
//
//	err := FormatValidationError("status_code", 200, 404, "GET /api/users", "",
//	    "Check the endpoint documentation", "Verify the API version")
func FormatValidationError(validationType string, expected, actual interface{}, context, responseSnippet string, customSuggestions ...string) ValidationError {
	suggestions := customSuggestions
	if len(customSuggestions) == 0 {
		suggestions = generateSuggestions(validationType, expected, actual)
	}

	ve := ValidationError{
		ErrorType:        validationType,
		Message:          generateMessageFromParts(validationType, expected, actual),
		Expected:         expected,
		Actual:           actual,
		Context:          context,
		ResponseSnippet:  responseSnippet,
		Suggestions:      suggestions,
	}

	return ve
}

// FormatValidationErrorWithDetails creates a ValidationError with extended field support.
//
// This function provides full customization over all ValidationError fields, including
// optional fields like FieldName, Location, RelatedFields, PatternDetails, RangeInfo, ValidationDetails, and custom Suggestions.
// Use this when you need more detailed error information than FormatValidationError provides.
//
// Parameters:
//   - validationType: The type of validation (e.g., "status_code", "error_message", "content_type")
//   - expected: The expected value
//   - actual: The actual value received
//   - context: Additional context about the validation operation
//   - responseSnippet: Excerpt from the response for debugging
//   - fieldName: The specific field name where the error was found (e.g., "error", "message")
//   - location: Where in the input the error occurred (e.g., "line 5", "field 'user.email'", "position 123")
//   - relatedFields: List of fields related to this error (e.g., ["email", "email_confirmation"])
//   - patternDetails: Information about pattern matching failures
//   - rangeInfo: Range boundaries for range validation failures (e.g., "400-499 (Client Error)")
//   - validationDetails: Additional validation-specific details as a list of strings
//   - customSuggestions: Optional custom suggestions (if provided, these override auto-generated suggestions)
//
// Returns a ValidationError with all specified fields populated.
//
// Example usage with auto-generated suggestions:
//
//	ve := FormatValidationErrorWithDetails(
//	    "error_message",
//	    "invalid.*token",
//	    "access_denied",
//	    "OAuth validation",
//	    `{"error": "access_denied"}`,
//	    "error",
//	    "field 'user.access_token'",
//	    []string{"user.access_token", "user.refresh_token"},
//	    "regex pattern 'invalid.*token' did not match",
//	    "",
//	    []string{"No matching error field found", "Checked 3 error message fields"},
//	)
//
// Example usage with custom suggestions:
//
//	ve := FormatValidationErrorWithDetails(
//	    "error_message",
//	    "invalid.*token",
//	    "access_denied",
//	    "OAuth validation",
//	    `{"error": "access_denied"}`,
//	    "error",
//	    "line 42 in user configuration",
//	    []string{"token_type", "access_token"},
//	    "regex pattern 'invalid.*token' did not match",
//	    "",
//	    []string{"No matching error field found"},
//	    "Check token format", "Verify OAuth scopes",
//	)
func FormatValidationErrorWithDetails(
	validationType string,
	expected, actual interface{},
	context, responseSnippet string,
	fieldName, location string,
	relatedFields []string,
	patternDetails, rangeInfo string,
	validationDetails []string,
	customSuggestions ...string,
) ValidationError {
	suggestions := customSuggestions
	if len(customSuggestions) == 0 {
		suggestions = generateSuggestions(validationType, expected, actual)
	}

	ve := ValidationError{
		ErrorType:         validationType,
		Message:           generateMessageFromParts(validationType, expected, actual),
		Expected:          expected,
		Actual:            actual,
		Context:           context,
		ResponseSnippet:   responseSnippet,
		FieldName:         fieldName,
		Location:          location,
		RelatedFields:     relatedFields,
		PatternDetails:    patternDetails,
		RangeInfo:         rangeInfo,
		ValidationDetails: validationDetails,
		Suggestions:       suggestions,
	}

	return ve
}

// generateMessageFromParts creates a concise human-readable message from validation details.
func generateMessageFromParts(validationType string, expected, actual interface{}) string {
	// Start with the basic validation failure message
	msg := fmt.Sprintf("%s validation failed", validationType)

	// Add expected vs actual information if available
	if expected != nil || actual != nil {
		msg += ": expected "
		switch exp := expected.(type) {
		case int:
			msg += fmt.Sprintf("%d", exp)
		case string:
			msg += fmt.Sprintf("'%s'", exp)
		case []int:
			msg += "one of ["
			for i, code := range exp {
				if i > 0 {
					msg += ", "
				}
				msg += fmt.Sprintf("%d", code)
			}
			msg += "]"
		default:
			msg += fmt.Sprintf("%v", exp)
		}

		msg += ", got "
		switch act := actual.(type) {
		case int:
			msg += fmt.Sprintf("%d", act)
		case string:
			if len(act) > 50 {
				act = act[:50] + "..."
			}
			msg += fmt.Sprintf("'%s'", act)
		default:
			msg += fmt.Sprintf("%v", act)
		}
	}

	return msg
}

// generateSuggestions generates relevant suggestions based on validation type and values.
func generateSuggestions(validationType string, expected, actual interface{}) []string {
	var suggestions []string

	switch validationType {
	case "status_code":
		suggestions = generateStatusCodeSuggestions(expected, actual)
	case "error_message":
		suggestions = generateErrorMessageSuggestions(expected, actual)
	case "content_type":
		suggestions = generateContentTypeSuggestions(expected, actual)
	case "status_code_range":
		suggestions = generateStatusCodeRangeSuggestions(expected, actual)
	default:
		suggestions = []string{"Review the request parameters and try again"}
	}

	return suggestions
}

// generateStatusCodeSuggestions generates suggestions for status code mismatches.
func generateStatusCodeSuggestions(expected, actual interface{}) []string {
	actualCode, ok := actual.(int)
	if !ok {
		return []string{"Verify the response format is correct"}
	}

	// Use the specific suggestions function which provides detailed suggestions per code
	suggestions := getStatusCodeSpecificSuggestions(actualCode)

	if len(suggestions) == 0 {
		// Fallback to generic suggestions if no specific ones found
		if actualCode >= 400 && actualCode < 500 {
			return []string{
				"Review request parameters and body",
				"Check API documentation for requirements",
				"Verify all headers are correctly set",
				"Ensure request complies with API contracts",
			}
		}
		if actualCode >= 500 && actualCode < 600 {
			return []string{
				"Implement retry logic with exponential backoff",
				"Check service status page for ongoing issues",
				"Contact support if the issue persists",
				"Review server logs if accessible",
			}
		}
		return []string{"Review the request and response for details"}
	}

	return suggestions
}

// generateErrorMessageSuggestions generates suggestions for error message mismatches.
func generateErrorMessageSuggestions(expected, actual interface{}) []string {
	expectedStr, _ := expected.(string)
	actualStr, _ := actual.(string)

	expectedLower := strings.ToLower(expectedStr)
	actualLower := strings.ToLower(actualStr)

	var suggestions []string

	// Check for common error patterns
	switch {
	case strings.Contains(expectedLower, "token"):
		suggestions = append(suggestions,
			"Verify token is correctly formatted",
			"Check token is for the correct resource",
			"Ensure token hasn't been revoked",
			"Refresh the authentication token if expired",
			"Consider token refresh if the token is old",
			"Check token expiration time")
		if strings.Contains(actualLower, "expired") {
			suggestions = append(suggestions, "Implement automatic token refresh")
		}
	case strings.Contains(expectedLower, "auth") && strings.Contains(actualLower, "denied"):
		suggestions = append(suggestions,
			"Verify account permissions",
			"Check if additional authorization scopes are needed",
			"Review API key or token permissions")
	case strings.Contains(expectedLower, "not found") || strings.Contains(expectedLower, "does not exist"):
		suggestions = append(suggestions,
			"Verify the resource identifier is correct",
			"Check if the resource exists",
			"Ensure the resource hasn't been deleted")
	case strings.Contains(expectedLower, "validation") || strings.Contains(expectedLower, "invalid"):
		suggestions = append(suggestions,
			"Review request body for missing required fields",
			"Verify data types match expected format",
			"Check field constraints (length, format, values)")
	case strings.Contains(expectedLower, "rate") || strings.Contains(expectedLower, "quota"):
		suggestions = append(suggestions,
			"Implement rate limiting",
			"Check API quota limits",
			"Add exponential backoff for retries")
	default:
		suggestions = append(suggestions,
			"Review the error message for specific details",
			"Check API documentation for this error type",
			"Verify request parameters match requirements")
	}

	return suggestions
}

// generateContentTypeSuggestions generates suggestions for content-type mismatches.
func generateContentTypeSuggestions(expected, actual interface{}) []string {
	return []string{
		"Verify Content-Type header matches request body format",
		"Check if charset or boundary parameters are needed",
		"Ensure the body is properly formatted for the content type",
	}
}

// generateStatusCodeRangeSuggestions generates suggestions for status code range mismatches.
func generateStatusCodeRangeSuggestions(expected, actual interface{}) []string {
	actualCode, ok := actual.(int)
	if !ok {
		return []string{"Verify the response format is correct"}
	}

	var suggestions []string

	switch {
	case actualCode >= 200 && actualCode < 300:
		suggestions = append(suggestions,
			"The request succeeded - update test expectations if this is expected behavior",
			"Verify the test is checking for the correct response type")
	case actualCode >= 400 && actualCode < 500:
		suggestions = append(suggestions,
			"Review request parameters for errors",
			"Check authentication credentials",
			"Verify the resource exists and is accessible")
	case actualCode >= 500 && actualCode < 600:
		suggestions = append(suggestions,
			"Check if this is a temporary server issue",
			"Implement retry logic with backoff",
			"Verify service status and availability")
	default:
		suggestions = append(suggestions,
			"Review the unexpected status code",
			"Check API documentation for valid codes",
			"Verify the request is properly formatted")
	}

	return suggestions
}

// =============================================================================
// ENHANCED ERROR MESSAGE OUTPUT EXAMPLES
// =============================================================================

// The following examples demonstrate the enhanced error reporting output from
// ValidateErrorMessage, showing all the detailed information provided when
// pattern matching fails.

// Example enhanced error output for regex pattern mismatch:
//
//	response := []byte(`{"error": "access_denied", "error_description": "User lacks required scope"}`)
//	err := ValidateErrorMessage(response, "invalid.*token")
//	if err != nil {
//	    log.Println(err.Error())
//	}
//
// Output:
//
//	error_message validation failed: expected 'invalid.*token', got 'access_denied'
//	  Pattern type: regex
//	  Expected pattern: invalid.*token
//	  Actual error message: "access_denied"
//	  Field name: error
//	  Checked fields: error, message, detail, description, error_description
//	  Response: {"error": "access_denied", "error_description": "User lacks requ...
//	  Suggestions:
//	    - Pattern looks for 'token' but actual message doesn't contain it
//	    - Check if the error is about authentication/authorization rather than tokens
//	    - Consider expanding pattern to include 'auth' or 'access' related terms
//
// Example enhanced error output for substring pattern mismatch:
//
//	response := []byte(`{"message": "Resource Not Found"}`)
//	err := ValidateErrorMessage(response, "not found")
//	if err != nil {
//	    log.Println(err.Error())
//	}
//
// Output:
//
//	error_message validation failed: expected 'not found', got 'Resource Not Found'
//	  Pattern type: substring
//	  Expected pattern: not found
//	  Actual error message: "Resource Not Found"
//	  Field name: message
//	  Checked fields: error, message, detail, description, error_description
//	  Response: {"message": "Resource Not Found"}
//	  Suggestions:
//	    - Substring match is case-sensitive
//	    - Pattern was found but with different casing
//	    - Consider using regex with '(?i)' prefix for case-insensitive matching
//
// Example enhanced error output when no error message field is found:
//
//	response := []byte(`{"status": "ok", "data": {"id": 123}}`)
//	err := ValidateErrorMessage(response, "error")
//	if err != nil {
//	    log.Println(err.Error())
//	}
//
// Output:
//
//	error_message validation failed: expected 'error', got ''
//	  No error message found in response body
//	  Checked fields: error, message, detail, description, error_description
//	  Response: {"status": "ok", "data": {"id": 123}}
//	  Suggestions:
//	    - Response may not contain standard error fields
//	    - Check if the API uses different field names for error messages
//	    - Verify the response structure matches expected format
//
// Example enhanced error output for regex metacharacter issues:
//
//	response := []byte(`{"error": "access_denied"}`)
//	err := ValidateErrorMessage(response, "access.denied")
//	if err != nil {
//	    log.Println(err.Error())
//	}
//
// Output:
//
//	error_message validation failed: expected 'access.denied', got 'access_denied'
//	  Pattern type: regex
//	  Expected pattern: access.denied
//	  Actual error message: "access_denied"
//	  Field name: error
//	  Checked fields: error, message, detail, description, error_description
//	  Response: {"error": "access_denied"}
//	  Suggestions:
//	    - Regex contains '.' which matches any character (except newline)
//	    - If you meant literal dot, escape it: '\.'
//	    - If substring search is intended, consider using plain text without metacharacters
//
// Example enhanced error output for pattern mismatch with multiple error fields:
//
//	response := []byte(`{"error": "Validation failed", "message": "Invalid input format", "detail": "Field required"}`)
//	err := ValidateErrorMessage(response, "expired")
//	if err != nil {
//	    log.Println(err.Error())
//	}
//
// Output:
//
//	error_message validation failed: expected 'expired', got 'Validation failed'
//	  Pattern type: substring
//	  Expected pattern: expired
//	  Actual error message: "Validation failed"
//	  Field name: error
//	  Checked fields: error, message, detail, description, error_description
//	  Found 3 error message fields in response
//	  Response: {"error": "Validation failed", "message": "Invalid input format", "de...
//	  Suggestions:
//	    - Review the expected pattern and actual message content
//	    - Check if pattern should be regex or substring matching
//	    - Verify the actual message format matches expectations
//
// Example enhanced error output for long response truncation:
//
//	longError := strings.Repeat("error message ", 50) // Creates a ~600 char error message
//	response := []byte(`{"error": "` + longError + `"}`)
//	err := ValidateErrorMessage(response, "not found")
//	if err != nil {
//	    log.Println(err.Error())
//	}
//
// Output:
//
//	error_message validation failed: expected 'not found', got 'error message error message...'
//	  Pattern type: substring
//	  Expected pattern: not found
//	  Actual error message: "error message error message error message error message error..."
//	  Field name: error
//	  Checked fields: error, message, detail, description, error_description
//	  Response: {"error": "error message error message error message error message error message erro...
//	  Suggestions:
//	    - Review the expected pattern and actual message content
//	    - Check if pattern should be regex or substring matching
//	    - Verify the actual message format matches expectations
