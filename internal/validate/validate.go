// Package validate provides helper functions for validating HTTP responses and other data.
package validate

import "net/http"

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
