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
		return fmt.Errorf("response body is empty")
	}

	if expectedPattern == "" {
		return fmt.Errorf("expected pattern cannot be empty")
	}

	// Parse JSON body
	var body map[string]interface{}
	if err := json.Unmarshal(response, &body); err != nil {
		return fmt.Errorf("failed to parse response body: %w", err)
	}

	// Search for error messages in common fields
	defaultFields := []string{"error", "message", "detail", "description", "error_description"}
	var foundMessages []string

	for _, field := range defaultFields {
		if value, exists := body[field]; exists {
			if strValue, ok := value.(string); ok && strValue != "" {
				foundMessages = append(foundMessages, strValue)
			}
			// Check nested error objects
			if nestedObj, ok := value.(map[string]interface{}); ok {
				if nestedValue, exists := nestedObj["message"]; exists {
					if strValue, ok := nestedValue.(string); ok && strValue != "" {
						foundMessages = append(foundMessages, strValue)
					}
				}
			}
		}
	}

	// If no error messages found, return error with response snippet
	if len(foundMessages) == 0 {
		snippet := extractResponseSnippet(response)
		return fmt.Errorf("no error message found in response body. Response snippet: %s", snippet)
	}

	// Detect if pattern is regex or substring
	isRegex := containsRegexMetacharacters(expectedPattern)

	// Check each found message for pattern match
	for _, message := range foundMessages {
		var matched bool

		if isRegex {
			// Try regex matching
			re, err := regexp.Compile(expectedPattern)
			if err != nil {
				return fmt.Errorf("invalid regex pattern '%s': %w", expectedPattern, err)
			}
			matched = re.MatchString(message)
		} else {
			// Simple substring matching (case-sensitive)
			matched = strings.Contains(message, expectedPattern)
		}

		if matched {
			// Pattern found - return nil (success)
			return nil
		}
	}

	// Pattern not found in any messages - return descriptive error
	snippet := extractResponseSnippet(response)
	firstMessage := foundMessages[0]
	if len(foundMessages) > 1 {
		return fmt.Errorf("pattern '%s' not found in error messages. First message: \"%s\". Response snippet: %s",
			expectedPattern, firstMessage, snippet)
	}
	return fmt.Errorf("pattern '%s' not found in error message: \"%s\". Response snippet: %s",
		expectedPattern, firstMessage, snippet)
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
//
// Returns a StatusCodeValidationResult with detailed validation information.
//
// Example usage:
//
//	result := ValidateStatusCodeWithDetails(response, []int{200, 201, 204})
//	if !result.Valid {
//	    log.Printf("Status validation failed: %s", result.MismatchDetails)
//	    log.Printf("Got: %d, expected one of: %v", result.ActualCode, result.ExpectedCodes)
//	}
//
//	// Single code validation
//	result := ValidateStatusCodeWithDetails(response, 200)
//	if result.Valid {
//	    log.Printf("Status code is valid: %d", result.ActualCode)
//	}
func ValidateStatusCodeWithDetails(resp *http.Response, expected interface{}) StatusCodeValidationResult {
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

	// Build detailed mismatch information
	if len(expectedCodes) == 1 {
		result.MismatchDetails = fmt.Sprintf("expected status code %d but got %d", expectedCodes[0], resp.StatusCode)
	} else {
		result.MismatchDetails = fmt.Sprintf("expected one of status codes %v but got %d", expectedCodes, resp.StatusCode)
	}

	return result
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
// Like ValidateStatusCodeRange but with comprehensive result information.
//
// Parameters:
//   - resp: The HTTP response to validate
//   - range: The status code range to validate against
//
// Returns a StatusCodeValidationResult with detailed information about the range validation.
//
// Example usage:
//
//	result := ValidateStatusCodeRangeWithDetails(response, Range4xx)
//	if !result.Valid {
//	    log.Printf("Status code %d is not in range %s (%d-%d)",
//	        result.ActualCode, result.Category, range.Min, range.Max)
//	}
func ValidateStatusCodeRangeWithDetails(resp *http.Response, statusRange StatusCodeRange) StatusCodeValidationResult {
	result := StatusCodeValidationResult{
		Valid:           false,
		ActualCode:      0,
		ExpectedCodes:   []int{},
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
		result.MismatchDetails = fmt.Sprintf("status code %d is not in range %s (%d-%d)",
			resp.StatusCode, statusRange.Description, statusRange.Min, statusRange.Max)
	}

	return result
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
		return FormatValidationError(
			"status_code_range",
			fmt.Sprintf("pattern in format 'Nxx' (3 chars)"),
			fmt.Sprintf("'%s' (%d chars)", pattern, len(pattern)),
			fmt.Sprintf("invalid pattern format: %s", pattern),
			"",
		)
	}

	// Extract the century digit (first character)
	centuryChar := pattern[0]
	if centuryChar < '1' || centuryChar > '5' {
		return FormatValidationError(
			"status_code_range",
			"century digit 1-5",
			fmt.Sprintf("'%c' (ASCII %d)", centuryChar, centuryChar),
			fmt.Sprintf("invalid pattern century in '%s'", pattern),
			"",
		)
	}

	// Validate 'xx' suffix
	if pattern[1] != 'x' || pattern[2] != 'x' {
		return FormatValidationError(
			"status_code_range",
			"pattern ending with 'xx'",
			fmt.Sprintf("'%s'", pattern[1:]),
			fmt.Sprintf("invalid pattern suffix in '%s'", pattern),
			"",
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

		return FormatValidationError(
			"status_code_range",
			expectedRange,
			actual,
			fmt.Sprintf("status code validation failed for pattern '%s'", pattern),
			"",
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
		'1': "Informational (1xx)",
		'2': "Success (2xx)",
		'3': "Redirection (3xx)",
		'4': "Client Error (4xx)",
		'5': "Server Error (5xx)",
	}

	desc, ok := descriptions[centuryChar]
	if !ok {
		return "", fmt.Errorf("unknown century: %c", centuryChar)
	}

	return desc, nil
}

// =============================================================================
// ENHANCED VALIDATION ERROR FORMATTING
// =============================================================================

// ValidationError represents a structured validation error with detailed context.
type ValidationError struct {
	// ValidationType is the category of validation (e.g., "status_code", "error_message", "content_type")
	ValidationType string
	// Expected is what was expected
	Expected interface{}
	// Actual is what was actually received
	Actual interface{}
	// Context provides additional context about the validation
	Context string
	// Suggestions provides actionable suggestions for fixing the issue
	Suggestions []string
	// ResponseSnippet is a truncated excerpt from the response for debugging
	ResponseSnippet string
}

// Error formats the ValidationError as a detailed, multi-line error message.
// The output includes:
// - The validation type
// - Expected vs actual values
// - Context (if provided)
// - Response snippet (if provided)
// - Suggestions for fixing the issue
func (ve ValidationError) Error() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("%s validation failed\n", ve.ValidationType))

	// Expected vs Actual
	switch exp := ve.Expected.(type) {
	case int:
		b.WriteString(fmt.Sprintf("  Expected: %d (%s)\n", exp, getStatusCodeDescription(exp)))
	case []int:
		b.WriteString("  Expected: one of [")
		for i, code := range exp {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(fmt.Sprintf("%d (%s)", code, getStatusCodeDescription(code)))
		}
		b.WriteString("]\n")
	case string:
		b.WriteString(fmt.Sprintf("  Expected: %s\n", exp))
	default:
		b.WriteString(fmt.Sprintf("  Expected: %v\n", exp))
	}

	// Actual value
	switch act := ve.Actual.(type) {
	case int:
		b.WriteString(fmt.Sprintf("  Actual:   %d (%s)\n", act, getStatusCodeDescription(act)))
	case string:
		if len(act) > 100 {
			act = act[:100] + "..."
		}
		b.WriteString(fmt.Sprintf("  Actual:   %s\n", act))
	default:
		b.WriteString(fmt.Sprintf("  Actual:   %v\n", act))
	}

	// Context
	if ve.Context != "" {
		b.WriteString(fmt.Sprintf("  Context:  %s\n", ve.Context))
	}

	// Response snippet
	if ve.ResponseSnippet != "" {
		b.WriteString(fmt.Sprintf("  Response: %s\n", ve.ResponseSnippet))
	}

	// Suggestions
	if len(ve.Suggestions) > 0 {
		b.WriteString("  Suggestions:\n")
		for _, suggestion := range ve.Suggestions {
			b.WriteString(fmt.Sprintf("    - %s\n", suggestion))
		}
	}

	return b.String()
}

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
//
// Returns a ValidationError with appropriate suggestions populated.
//
// Example usage:
//
//	err := FormatValidationError("status_code", 200, 404, "GET /api/users", "")
//	// Output includes suggestions like:
//	// - Verify the endpoint URL is correct
//	// - Check if the resource exists
//	// - Review authentication credentials
func FormatValidationError(validationType string, expected, actual interface{}, context, responseSnippet string) ValidationError {
	ve := ValidationError{
		ValidationType:  validationType,
		Expected:        expected,
		Actual:          actual,
		Context:         context,
		ResponseSnippet: responseSnippet,
		Suggestions:     generateSuggestions(validationType, expected, actual),
	}

	return ve
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
	var suggestions []string

	actualCode, ok := actual.(int)
	if !ok {
		return []string{"Verify the response format is correct"}
	}

	switch {
	case actualCode >= 400 && actualCode < 500:
		// Client errors
		switch actualCode {
		case 400:
			suggestions = append(suggestions,
				"Check request body syntax and formatting",
				"Verify all required fields are present",
				"Ensure content-type header matches request body format")
		case 401:
			suggestions = append(suggestions,
				"Verify authentication credentials are correct",
				"Check if API token or session has expired",
				"Ensure Authorization header is properly formatted")
		case 403:
			suggestions = append(suggestions,
				"Verify your account has permission to access this resource",
				"Check if additional scopes or roles are required",
				"Review API documentation for required permissions")
		case 404:
			suggestions = append(suggestions,
				"Verify the endpoint URL is correct",
				"Check if the resource ID or identifier exists",
				"Ensure the resource hasn't been deleted or moved")
		case 409:
			suggestions = append(suggestions,
				"Check if a resource with this identifier already exists",
				"Verify unique constraints on the resource",
				"Consider using PUT instead of POST for updates")
		case 429:
			suggestions = append(suggestions,
				"Implement rate limiting and exponential backoff",
				"Check API quota limits",
				"Consider caching responses to reduce request frequency")
		default:
			suggestions = append(suggestions,
				"Review request parameters and body",
				"Check API documentation for requirements",
				"Verify all headers are correctly set")
		}
	case actualCode >= 500 && actualCode < 600:
		// Server errors
		suggestions = append(suggestions,
			"Implement retry logic with exponential backoff",
			"Check service status page for ongoing issues",
			"Contact support if the issue persists")
	}

	if len(suggestions) == 0 {
		suggestions = append(suggestions, "Review the request and response for details")
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
	case strings.Contains(expectedLower, "token") && strings.Contains(actualLower, "expired"):
		suggestions = append(suggestions,
			"Refresh the authentication token",
			"Check token expiration time",
			"Implement automatic token refresh")
	case strings.Contains(expectedLower, "token") && strings.Contains(actualLower, "invalid"):
		suggestions = append(suggestions,
			"Verify token is correctly formatted",
			"Check token is for the correct resource",
			"Ensure token hasn't been revoked")
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

// getStatusCodeDescription returns a human-readable description for a status code.
// This is a lightweight version for error formatting.
func getStatusCodeDescription(code int) string {
	descriptions := map[int]string{
		100: "Continue", 101: "Switching Protocols", 102: "Processing",
		200: "OK", 201: "Created", 202: "Accepted", 203: "Non-Authoritative Information",
		204: "No Content", 205: "Reset Content", 206: "Partial Content",
		300: "Multiple Choices", 301: "Moved Permanently", 302: "Found",
		303: "See Other", 304: "Not Modified", 307: "Temporary Redirect",
		308: "Permanent Redirect",
		400: "Bad Request", 401: "Unauthorized", 402: "Payment Required",
		403: "Forbidden", 404: "Not Found", 405: "Method Not Allowed",
		406: "Not Acceptable", 407: "Proxy Authentication Required",
		408: "Request Timeout", 409: "Conflict", 410: "Gone",
		411: "Length Required", 412: "Precondition Failed",
		413: "Payload Too Large", 414: "URI Too Long",
		415: "Unsupported Media Type", 416: "Range Not Satisfiable",
		417: "Expectation Failed", 418: "I'm a teapot",
		422: "Unprocessable Entity", 423: "Locked", 424: "Failed Dependency",
		426: "Upgrade Required", 428: "Precondition Required",
		429: "Too Many Requests", 431: "Request Header Fields Too Large",
		451: "Unavailable For Legal Reasons",
		500: "Internal Server Error", 501: "Not Implemented",
		502: "Bad Gateway", 503: "Service Unavailable",
		504: "Gateway Timeout", 505: "HTTP Version Not Supported",
		506: "Variant Also Negotiates", 507: "Insufficient Storage",
		508: "Loop Detected", 510: "Not Extended",
		511: "Network Authentication Required",
	}

	if desc, ok := descriptions[code]; ok {
		return desc
	}
	return "Unknown"
}
