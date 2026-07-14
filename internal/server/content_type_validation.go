package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// =============================================================================
// CONTENT-TYPE HEADER VALIDATION HELPERS
// =============================================================================
// This module provides comprehensive Content-Type header validation utilities
// for error response testing. It supports pattern matching, multiple allowed
// content-types, and flexible validation patterns.
//
// Functions:
// - ValidateContentType: Validate single expected content-type with pattern matching
// - ValidateContentTypeAny: Validate against multiple allowed content-types
// - CheckContentType: Non-asserting version that returns boolean
// - ValidateContentTypePrefix: Validate content-type starts with prefix
// - ValidateContentTypeJSON: Validates application/json content-type
// - ValidateContentTypeXML: Validates application/xml content-type
// - ValidateContentTypeText: Validates text/* content-type
// =============================================================================

// ValidateContentType validates that the HTTP response has the expected content-type.
//
// This helper supports pattern matching where the expected content-type can match
// even if the response includes additional parameters like charset. For example:
// - Expected "application/json" matches "application/json; charset=utf-8"
// - Expected "application/xml" matches "application/xml; charset=iso-8859-1"
//
// Parameters:
//   - t: The testing instance for assertions
//   - response: The HTTP response to validate (can be *httptest.ResponseRecorder or *http.Response)
//   - expectedContentType: The expected content-type (e.g., "application/json", "application/xml")
//
// Example:
//   ValidateContentType(t, w, "application/json")
func ValidateContentType(t *testing.T, response interface{}, expectedContentType string) {
	t.Helper()

	actualContentType := getContentType(response)

	if !contentTypeMatches(actualContentType, expectedContentType) {
		t.Errorf("Expected Content-Type '%s', got '%s'", expectedContentType, actualContentType)
	}
}

// ValidateContentTypeAny validates that the HTTP response has one of the allowed content-types.
//
// This helper is useful when multiple content-types are acceptable for a given scenario.
// For example, when testing JSON endpoints that might return JSON with or without charset.
//
// Parameters:
//   - t: The testing instance for assertions
//   - response: The HTTP response to validate
//   - allowedContentTypes: Slice of acceptable content-types
//
// Example:
//   ValidateContentTypeAny(t, w, []string{"application/json", "text/plain"})
func ValidateContentTypeAny(t *testing.T, response interface{}, allowedContentTypes []string) {
	t.Helper()

	if len(allowedContentTypes) == 0 {
		t.Error("ValidateContentTypeAny: allowedContentTypes cannot be empty")
		return
	}

	actualContentType := getContentType(response)

	// Check if actual content-type matches any in the allowed list
	for _, allowedType := range allowedContentTypes {
		if contentTypeMatches(actualContentType, allowedType) {
			return // Success - content-type is allowed
		}
	}

	// Build helpful error message showing what was allowed vs. what was received
	allowedList := strings.Join(allowedContentTypes, ", ")
	t.Errorf("Expected Content-Type to be one of [%s], got '%s'", allowedList, actualContentType)
}

// ValidateContentTypePrefix validates that the response content-type starts with the given prefix.
//
// This helper is useful for broad category validation, such as checking for any JSON-like
// content-type (application/json, application/problem+json, etc.).
//
// Parameters:
//   - t: The testing instance for assertions
//   - response: The HTTP response to validate
//   - prefix: The content-type prefix to match (e.g., "application/json", "text/")
//
// Example:
//   // Validate any JSON content-type
//   ValidateContentTypePrefix(t, w, "application/json")
//
//   // Validate any text content-type
//   ValidateContentTypePrefix(t, w, "text/")
func ValidateContentTypePrefix(t *testing.T, response interface{}, prefix string) {
	t.Helper()

	actualContentType := getContentType(response)

	if !strings.HasPrefix(actualContentType, prefix) {
		t.Errorf("Expected Content-Type to start with '%s', got '%s'", prefix, actualContentType)
	}
}

// CheckContentType checks if the HTTP response has the expected content-type without asserting.
//
// This non-asserting version returns a boolean, allowing for conditional logic
// in tests. Supports pattern matching like ValidateContentType.
//
// Parameters:
//   - response: The HTTP response to validate
//   - expectedContentType: The expected content-type
//
// Returns:
//   - true if the response has the expected content-type (with pattern matching), false otherwise
//
// Example:
//   if CheckContentType(w, "application/json") {
//       // Handle JSON case
//   } else {
//       // Handle other cases
//   }
func CheckContentType(response interface{}, expectedContentType string) bool {
	actualContentType := getContentType(response)
	return contentTypeMatches(actualContentType, expectedContentType)
}

// CheckContentTypeAny checks if the HTTP response has one of the allowed content-types.
//
// This non-asserting version returns a boolean for flexible content-type checking.
// Supports pattern matching for each allowed type.
//
// Parameters:
//   - response: The HTTP response to validate
//   - allowedContentTypes: Slice of acceptable content-types
//
// Returns:
//   - true if the response content-type is in the allowed list (with pattern matching), false otherwise
//
// Example:
//   allowedTypes := []string{"application/json", "text/plain"}
//   if CheckContentTypeAny(w, allowedTypes) {
//       // Handle allowed case
//   }
func CheckContentTypeAny(response interface{}, allowedContentTypes []string) bool {
	actualContentType := getContentType(response)

	for _, allowedType := range allowedContentTypes {
		if contentTypeMatches(actualContentType, allowedType) {
			return true
		}
	}
	return false
}

// CheckContentTypePrefix checks if the response content-type starts with the given prefix.
//
// This non-asserting version returns a boolean for prefix-based content-type validation.
//
// Parameters:
//   - response: The HTTP response to validate
//   - prefix: The content-type prefix to check
//
// Returns:
//   - true if the content-type starts with the prefix, false otherwise
//
// Example:
//   if CheckContentTypePrefix(w, "application/json") {
//       // Handle JSON case
//   }
func CheckContentTypePrefix(response interface{}, prefix string) bool {
	actualContentType := getContentType(response)
	return strings.HasPrefix(actualContentType, prefix)
}

// =============================================================================
// CONVENIENCE FUNCTIONS FOR COMMON CONTENT-TYPE CATEGORIES
// =============================================================================

// ValidateContentTypeJSON validates that the response has an application/json content-type.
//
// This convenience function validates any JSON content-type, including:
// - application/json
// - application/json; charset=utf-8
// - application/problem+json
// - application/ld+json
// - application/any+json
//
// Example:
//   ValidateContentTypeJSON(t, w)
func ValidateContentTypeJSON(t *testing.T, response interface{}) {
	t.Helper()
	actualContentType := getContentType(response)

	// Check for exact match or parameter-based match
	if contentTypeMatches(actualContentType, "application/json") {
		return
	}

	// Check for JSON variant (+json suffix)
	if IsContentTypeJSON(actualContentType) {
		return
	}

	t.Errorf("Expected JSON Content-Type (e.g., application/json, application/problem+json), got '%s'", actualContentType)
}

// ValidateContentTypeXML validates that the response has an application/xml content-type.
//
// This convenience function validates any XML content-type, including:
// - application/xml
// - application/xml; charset=utf-8
// - text/xml
// - text/xml; charset=iso-8859-1
//
// Example:
//   ValidateContentTypeXML(t, w)
func ValidateContentTypeXML(t *testing.T, response interface{}) {
	t.Helper()
	ValidateContentTypeAny(t, response, []string{"application/xml", "text/xml"})
}

// ValidateContentTypeText validates that the response has a text/* content-type.
//
// This convenience function validates any text content-type, including:
// - text/plain
// - text/html
// - text/csv
// - text/xml
//
// Example:
//   ValidateContentTypeText(t, w)
func ValidateContentTypeText(t *testing.T, response interface{}) {
	t.Helper()
	ValidateContentTypePrefix(t, response, "text/")
}

// ValidateContentTypeBinary validates that the response has a binary content-type.
//
// This convenience function validates any binary content-type, including:
// - application/octet-stream
// - application/pdf
// - image/*
// - video/*
//
// Example:
//   ValidateContentTypeBinary(t, w)
func ValidateContentTypeBinary(t *testing.T, response interface{}) {
	t.Helper()

	actualContentType := getContentType(response)

	// Check for common binary content-types
	binaryTypes := []string{
		"application/octet-stream",
		"application/pdf",
		"image/",
		"video/",
		"audio/",
	}

	isBinary := false
	for _, binaryType := range binaryTypes {
		if strings.HasPrefix(actualContentType, binaryType) {
			isBinary = true
			break
		}
	}

	if !isBinary {
		t.Errorf("Expected binary Content-Type (e.g., application/octet-stream, image/*), got '%s'", actualContentType)
	}
}

// ValidateContentTypeHTML validates that the response has an HTML content-type.
//
// This convenience function validates any HTML content-type, including:
// - text/html
// - application/xhtml+xml
//
// Example:
//   ValidateContentTypeHTML(t, w)
func ValidateContentTypeHTML(t *testing.T, response interface{}) {
	t.Helper()
	ValidateContentTypeAny(t, response, []string{"text/html", "application/xhtml+xml"})
}

// ValidateContentTypeForm validates that the response has a form-encoded content-type.
//
// This convenience function validates any form content-type, including:
// - application/x-www-form-urlencoded
// - multipart/form-data
//
// Example:
//   ValidateContentTypeForm(t, w)
func ValidateContentTypeForm(t *testing.T, response interface{}) {
	t.Helper()
	ValidateContentTypeAny(t, response, []string{"application/x-www-form-urlencoded", "multipart/form-data"})
}

// =============================================================================
// CONTENT-TYPE PATTERN MATCHING
// =============================================================================

// parseMediaType extracts the base media type from a content-type string.
//
// This function parses a content-type header and returns only the media type,
// stripping away any parameters like charset, boundary, etc. It handles
// whitespace and malformed content-type strings gracefully.
//
// Parameters:
//   - contentType: The content-type header value (may be empty or malformed)
//
// Returns:
//   - The base media type (e.g., "application/json") or empty string if parsing fails
//
// Examples:
//   - parseMediaType("application/json") returns "application/json"
//   - parseMediaType("application/json; charset=utf-8") returns "application/json"
//   - parseMediaType("text/xml; charset=iso-8859-1; version=1") returns "text/xml"
//   - parseMediaType("  application/json  ;  charset=utf-8  ") returns "application/json"
//   - parseMediaType("") returns ""
//   - parseMediaType("malformed without semicolon") returns "malformed without semicolon"
func parseMediaType(contentType string) string {
	if contentType == "" {
		return ""
	}

	// Split by semicolon to separate media type from parameters
	// The first part before any semicolon is the base media type
	idx := strings.Index(contentType, ";")
	if idx == -1 {
		// No parameters found, return the whole string trimmed
		return strings.TrimSpace(contentType)
	}

	// Extract and trim the media type portion
	return strings.TrimSpace(contentType[:idx])
}

// contentTypeMatches checks if the actual content-type matches the expected content-type.
//
// This function implements robust pattern matching where content-types match if their
// base media types are equal, regardless of parameters like charset.
//
// The function properly parses both content-type strings to extract their base media
// types before comparison, handling:
// - Exact matches: "application/json" == "application/json"
// - Parameters in actual: "application/json" matches "application/json; charset=utf-8"
// - Parameters in expected: "application/json; charset=utf-8" matches "application/json"
// - Parameters in both: "application/json; charset=utf-8" matches "application/json; version=1"
// - Whitespace variations: "application/json; charset=utf-8" matches "  application/json  "
// - Empty strings: "" does not match "application/json"
// - Malformed content-types: handled gracefully by treating them as literal strings
//
// Parameters:
//   - actual: The actual content-type header value from the response
//   - expected: The expected content-type to match against
//
// Returns:
//   - true if the base media types match, false otherwise
//
// Examples of matches:
//   - contentTypeMatches("application/json", "application/json") returns true
//   - contentTypeMatches("application/json; charset=utf-8", "application/json") returns true
//   - contentTypeMatches("application/json", "application/json; charset=iso-8859-1") returns true
//   - contentTypeMatches("application/json; charset=utf-8", "application/json; version=1") returns true
//   - contentTypeMatches("  application/json  ", "application/json") returns true
//
// Examples of non-matches:
//   - contentTypeMatches("application/json", "application/xml") returns false
//   - contentTypeMatches("text/plain", "application/json") returns false
//   - contentTypeMatches("", "application/json") returns false
//   - contentTypeMatches("application/json", "") returns false
func contentTypeMatches(actual, expected string) bool {
	// Parse both content-type strings to extract base media types
	actualMediaType := parseMediaType(actual)
	expectedMediaType := parseMediaType(expected)

	// Compare the parsed media types
	return actualMediaType == expectedMediaType
}

// getContentType extracts the Content-Type header from various response types.
//
// This helper supports both *httptest.ResponseRecorder and *http.Response,
// making the validation functions work with different response types.
// Returns empty string if header is not present.
func getContentType(response interface{}) string {
	switch r := response.(type) {
	case *httptest.ResponseRecorder:
		return r.Header().Get("Content-Type")
	case *http.Response:
		return r.Header.Get("Content-Type")
	default:
		panic(fmt.Sprintf("Unsupported response type: %T. Expected *httptest.ResponseRecorder or *http.Response", response))
	}
}

// =============================================================================
// CONTENT-TYPE ANALYSIS HELPERS
// =============================================================================

// GetContentTypeCharset extracts the charset parameter from a content-type header.
//
// This helper parses the content-type header and returns the charset value if present.
// Returns empty string if no charset is specified.
//
// Parameters:
//   - contentType: The content-type header value
//
// Returns:
//   - The charset value (e.g., "utf-8", "iso-8859-1") or empty string if not present
//
// Example:
//   charset := GetContentTypeCharset("application/json; charset=utf-8")
//   // charset = "utf-8"
func GetContentTypeCharset(contentType string) string {
	// Split by semicolon to get parameters
	parts := strings.Split(contentType, ";")
	for i := 1; i < len(parts); i++ {
		param := strings.TrimSpace(parts[i])
		if strings.HasPrefix(strings.ToLower(param), "charset=") {
			return strings.TrimPrefix(param, "charset=")
		}
	}
	return ""
}

// GetContentTypeWithoutParams returns the content-type without parameters.
//
// This helper strips parameters like charset from the content-type header,
// returning just the base MIME type.
//
// Parameters:
//   - contentType: The content-type header value
//
// Returns:
//   - The base content-type without parameters
//
// Example:
//   baseType := GetContentTypeWithoutParams("application/json; charset=utf-8")
//   // baseType = "application/json"
func GetContentTypeWithoutParams(contentType string) string {
	// Split by semicolon and return first part (base type)
	parts := strings.Split(contentType, ";")
	return strings.TrimSpace(parts[0])
}

// IsContentTypeJSON checks if a content-type string represents JSON.
//
// This helper checks if the content-type is any variant of JSON, including
// application/json, application/problem+json, etc.
//
// Parameters:
//   - contentType: The content-type header value
//
// Returns:
//   - true if the content-type represents JSON, false otherwise
func IsContentTypeJSON(contentType string) bool {
	baseType := GetContentTypeWithoutParams(contentType)
	return baseType == "application/json" || strings.HasSuffix(baseType, "+json")
}

// IsContentTypeXML checks if a content-type string represents XML.
//
// This helper checks if the content-type is any variant of XML, including
// application/xml, text/xml, application/xhtml+xml, etc.
//
// Parameters:
//   - contentType: The content-type header value
//
// Returns:
//   - true if the content-type represents XML, false otherwise
func IsContentTypeXML(contentType string) bool {
	baseType := GetContentTypeWithoutParams(contentType)
	return baseType == "application/xml" || baseType == "text/xml" || strings.HasSuffix(baseType, "+xml")
}
