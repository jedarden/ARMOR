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
