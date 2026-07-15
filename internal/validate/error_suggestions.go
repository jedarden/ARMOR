package validate

import (
	"fmt"
	"strings"
)

/*
Enhanced Suggestion Generation for Validation Errors

This file provides comprehensive suggestion generation for all validation error types.
It extends the existing basic suggestion system with more detailed, actionable
recommendations for common failure scenarios.

Suggestion Strategy:
1. Error Type Detection: Identify the specific validation error type
2. Context Analysis: Analyze the expected vs actual values to understand the mismatch
3. Pattern Matching: Detect common failure patterns (e.g., expired tokens, missing fields)
4. Actionable Recommendations: Provide specific steps to resolve the issue
5. Progressive Detail: Start with most likely causes, then less common ones
*/

// =============================================================================
// COMPREHENSIVE SUGGESTION GENERATOR
// =============================================================================

// GenerateComprehensiveSuggestions creates detailed, actionable suggestions for any validation error.
// This is the main entry point for enhanced suggestion generation and provides comprehensive
// recommendations based on error type, expected/actual values, and common failure patterns.
//
// Parameters:
//   - errorType: The type of validation error (e.g., "status_code", "error_message", "content_type")
//   - expected: The expected value (can be any type)
//   - actual: The actual value received (can be any type)
//   - context: Optional additional context about where/when the validation occurred
//
// Returns a slice of suggestion strings, ordered from most to least likely solution.
//
// Example usage:
//
//	suggestions := GenerateComprehensiveSuggestions("status_code", 200, 404, "GET /api/users/123")
//	// Returns:
//	// ["Verify the endpoint URL is correct",
//	//  "Check if the resource ID or identifier exists",
//	//  "Ensure the resource hasn't been deleted or moved",
//	//  "Review API versioning (URL path may have changed)",
//	//  "Check authentication credentials"]
func GenerateComprehensiveSuggestions(errorType string, expected, actual interface{}, context string) []string {
	var suggestions []string

	// Normalize error type
	normalizedType := strings.ToLower(strings.TrimSpace(errorType))

	switch normalizedType {
	case "status_code":
		suggestions = generateEnhancedStatusCodeSuggestions(expected, actual, context)
	case "error_message", "error_message_pattern":
		suggestions = generateEnhancedErrorMessageSuggestions(expected, actual, context)
	case "content_type":
		suggestions = generateEnhancedContentTypeSuggestions(expected, actual, context)
	case "status_code_range":
		suggestions = generateEnhancedStatusCodeRangeSuggestions(expected, actual, context)
	case "cors_headers":
		suggestions = generateCORSSuggestions(expected, actual, context)
	case "response_structure":
		suggestions = generateResponseStructureSuggestions(expected, actual, context)
	case "response_body":
		suggestions = generateResponseBodySuggestions(expected, actual, context)
	case "response_encoding":
		suggestions = generateResponseEncodingSuggestions(expected, actual, context)
	case "auth_headers":
		suggestions = generateAuthHeaderSuggestions(expected, actual, context)
	case "custom_headers":
		suggestions = generateCustomHeaderSuggestions(expected, actual, context)
	case "json_schema":
		suggestions = generateJSONSchemaSuggestions(expected, actual, context)
	case "data_validation", "field_validation":
		suggestions = generateDataValidationSuggestions(expected, actual, context)
	case "type_validation":
		suggestions = generateTypeValidationSuggestions(expected, actual, context)
	case "timeout":
		suggestions = generateTimeoutSuggestions(expected, actual, context)
	case "rate_limit":
		suggestions = generateRateLimitSuggestions(expected, actual, context)
	case "retry_exceeded":
		suggestions = generateRetryExceededSuggestions(expected, actual, context)
	default:
		// For unknown or custom error types, provide generic suggestions
		suggestions = generateGenericValidationSuggestions(errorType, expected, actual, context)
	}

	// Ensure we always return at least one suggestion
	if len(suggestions) == 0 {
		suggestions = append(suggestions, "Review the validation error and check if the request/response meets expectations")
	}

	return suggestions
}

// =============================================================================
// ENHANCED STATUS CODE SUGGESTIONS
// =============================================================================

// generateEnhancedStatusCodeSuggestions provides detailed suggestions for HTTP status code mismatches.
// This extends the basic status code suggestions with context-aware recommendations.
func generateEnhancedStatusCodeSuggestions(expected, actual interface{}, context string) []string {
	actualCode, ok := actual.(int)
	if !ok {
		return []string{
			"Verify the response format is correct",
			"Ensure you're receiving an HTTP response with a status code",
			"Check if the response is being parsed correctly",
		}
	}

	var suggestions []string

	// Get specific suggestions based on the actual status code
	specificSuggestions := getStatusCodeSpecificSuggestions(actualCode)
	suggestions = append(suggestions, specificSuggestions...)

	// Add context-aware suggestions
	if context != "" {
		contextLower := strings.ToLower(context)

		// Detect common patterns in context
		if strings.Contains(contextLower, "get") && actualCode == 404 {
			suggestions = append(suggestions,
				"GET request returned 404 - verify the resource exists at the endpoint",
				"Check if the endpoint path in your context is correct",
			)
		}

		if strings.Contains(contextLower, "post") && actualCode == 400 {
			suggestions = append(suggestions,
				"POST request returned 400 - validate request body format",
				"Ensure Content-Type header matches request body format",
			)
		}

		if strings.Contains(contextLower, "put") || strings.Contains(contextLower, "patch") {
			if actualCode == 404 {
				suggestions = append(suggestions,
					"UPDATE request returned 404 - resource may not exist yet",
					"Consider using POST instead if creating a new resource",
				)
			}
		}

		if strings.Contains(contextLower, "delete") && actualCode == 404 {
			suggestions = append(suggestions,
				"DELETE request returned 404 - resource may already be deleted",
				"Check if the resource was already removed",
			)
		}

		if strings.Contains(contextLower, "oauth") || strings.Contains(contextLower, "auth") {
			if actualCode == 401 {
				suggestions = append(suggestions,
					"OAuth-related request failed authentication",
					"Verify access token is valid and not expired",
					"Check token has required scopes for this resource",
				)
			}
			if actualCode == 403 {
				suggestions = append(suggestions,
					"OAuth token lacks required permissions",
					"Verify token scopes include access to this resource",
				)
			}
		}
	}

	// Add general debugging suggestions
	suggestions = append(suggestions,
		"Review API documentation for expected status codes",
		"Test with a tool like curl to verify the API behavior",
	)

	return suggestions
}

// =============================================================================
// ENHANCED ERROR MESSAGE SUGGESTIONS
// =============================================================================

// generateEnhancedErrorMessageSuggestions provides detailed suggestions for error message pattern mismatches.
func generateEnhancedErrorMessageSuggestions(expected, actual interface{}, context string) []string {
	expectedStr, _ := expected.(string)
	actualStr, _ := actual.(string)

	expectedLower := strings.ToLower(expectedStr)
	actualLower := strings.ToLower(actualStr)

	var suggestions []string

	// Token-related errors (check both expected and actual)
	if strings.Contains(expectedLower, "token") || strings.Contains(actualLower, "token") {
		if strings.Contains(actualLower, "expired") {
			suggestions = append(suggestions,
				"Token has expired - implement token refresh logic",
				"Check token expiration time and refresh before expiry",
				"Use refresh token to obtain new access token",
				"Consider implementing automatic token renewal",
			)
		} else if strings.Contains(actualLower, "invalid") {
			suggestions = append(suggestions,
				"Token is invalid - verify token format and structure",
				"Check if token is properly encoded (JWT, etc.)",
				"Ensure token hasn't been corrupted in transmission",
				"Verify token signature if using JWT",
			)
		} else if strings.Contains(actualLower, "revoked") {
			suggestions = append(suggestions,
				"Token has been revoked - obtain a new token",
				"Check if user permissions have changed",
				"Verify token hasn't been invalidated by security events",
			)
		} else {
			suggestions = append(suggestions,
				"Token-related error detected",
				"Verify token is correctly formatted and valid",
				"Check token expiration and refresh if needed",
				"Ensure token has required scopes/permissions",
			)
		}
	}

	// Authentication/Authorization errors (check both expected and actual)
	if strings.Contains(expectedLower, "auth") || strings.Contains(actualLower, "auth") ||
	   strings.Contains(actualLower, "denied") || strings.Contains(actualLower, "forbidden") {
		if strings.Contains(actualLower, "denied") || strings.Contains(actualLower, "forbidden") {
			suggestions = append(suggestions,
				"Authentication/authorization failed",
				"Verify your account has permission to access this resource",
				"Check if additional authorization scopes are required",
				"Review API key or token permissions",
				"Ensure user account is in good standing",
			)
		} else if strings.Contains(actualLower, "invalid") || strings.Contains(actualLower, "failed") {
			suggestions = append(suggestions,
				"Authentication credentials are invalid",
				"Verify username/password or API key are correct",
				"Check if authentication method is supported",
				"Ensure credentials haven't been changed or reset",
			)
		}
	}

	// Resource not found errors (check both expected and actual)
	if strings.Contains(expectedLower, "not found") || strings.Contains(expectedLower, "does not exist") ||
	   strings.Contains(actualLower, "not found") || strings.Contains(actualLower, "does not exist") {
		if strings.Contains(actualLower, "not found") || strings.Contains(actualLower, "does not exist") {
			suggestions = append(suggestions,
				"Resource does not exist",
				"Verify the resource identifier is correct",
				"Check if the resource has been deleted or moved",
				"Ensure you have access to the resource",
				"Review API versioning (endpoint may have changed)",
			)
		} else if strings.Contains(actualLower, "unauthorized") || strings.Contains(actualLower, "forbidden") {
			suggestions = append(suggestions,
				"Resource exists but access is denied",
				"Check if you have permission to access this resource",
				"Verify authentication credentials are valid",
				"Review resource access control settings",
			)
		} else {
			suggestions = append(suggestions,
				"Resource-related error",
				"Verify the resource ID or identifier is correct",
				"Check if the resource exists and is accessible",
				"Ensure proper authentication/authorization",
			)
		}
	}

	// Validation errors (check both expected and actual)
	if strings.Contains(expectedLower, "validation") || strings.Contains(expectedLower, "invalid") ||
	   strings.Contains(actualLower, "validation") || strings.Contains(actualLower, "invalid") {
		suggestions = append(suggestions,
			"Input validation failed",
			"Review request body for missing required fields",
			"Verify data types match expected format",
			"Check field constraints (length, format, values)",
			"Ensure request body is valid JSON/XML",
		)

		if strings.Contains(actualLower, "required") {
			suggestions = append(suggestions,
				"Required field is missing",
				"Check API documentation for required fields",
				"Ensure all mandatory fields are included",
			)
		}
		if strings.Contains(actualLower, "format") {
			suggestions = append(suggestions,
				"Field format is invalid",
				"Verify field format matches requirements (email, date, etc.)",
				"Check if field value conforms to expected pattern",
			)
		}
	}

	// Rate limiting errors (check both expected and actual)
	if strings.Contains(expectedLower, "rate") || strings.Contains(expectedLower, "quota") || strings.Contains(expectedLower, "limit") ||
	   strings.Contains(actualLower, "rate") || strings.Contains(actualLower, "quota") || strings.Contains(actualLower, "limit") {
		suggestions = append(suggestions,
			"Rate limit or quota exceeded",
			"Implement rate limiting and exponential backoff",
			"Check API quota limits and current usage",
			"Add caching to reduce request frequency",
			"Review rate limit headers (Retry-After, X-RateLimit-Remaining)",
			"Consider upgrading your API plan if quota is too low",
		)

		if strings.Contains(actualLower, "too many") {
			suggestions = append(suggestions,
				"Too many requests - implement request throttling",
				"Add request queuing and throttling logic",
				"Consider batch requests to reduce frequency",
			)
		}
	}

	// Timeout errors (check both expected and actual)
	if strings.Contains(expectedLower, "timeout") || strings.Contains(actualLower, "timeout") {
		suggestions = append(suggestions,
			"Request timeout occurred",
			"Increase timeout duration if operation is slow",
			"Check if server is experiencing high load",
			"Implement retry logic with exponential backoff",
			"Verify network connectivity and latency",
		)
	}

	// Server errors (check both expected and actual)
	if strings.Contains(expectedLower, "server") || strings.Contains(expectedLower, "internal") ||
	   strings.Contains(actualLower, "server") || strings.Contains(actualLower, "internal") {
		suggestions = append(suggestions,
			"Server error occurred",
			"Check service status page for ongoing issues",
			"Implement retry logic with exponential backoff",
			"Contact support if the issue persists",
			"Review server logs if accessible",
		)

		if strings.Contains(actualLower, "500") || strings.Contains(actualLower, "internal server") {
			suggestions = append(suggestions,
				"Internal server error - this is a server-side issue",
				"Check if your request is triggering a server bug",
				"Verify request format matches API expectations",
			)
		}
	}

	// If no specific pattern matched, provide general suggestions
	if len(suggestions) == 0 {
		suggestions = append(suggestions,
			"Review the error message for specific details",
			"Check API documentation for this error type",
			"Verify request parameters match requirements",
			"Test with different parameter values",
		)
	}

	// Add context-specific suggestions if context is provided
	if context != "" {
		contextLower := strings.ToLower(context)
		if strings.Contains(contextLower, "oauth") || strings.Contains(contextLower, "token") {
			suggestions = append(suggestions,
				"OAuth-related validation - check token validity and scopes",
			)
		}
		if strings.Contains(contextLower, "user") || strings.Contains(contextLower, "account") {
			suggestions = append(suggestions,
				"User/account-related validation - verify user exists and has access",
			)
		}
	}

	return suggestions
}

// =============================================================================
// ENHANCED CONTENT TYPE SUGGESTIONS
// =============================================================================

// generateEnhancedContentTypeSuggestions provides detailed suggestions for Content-Type mismatches.
func generateEnhancedContentTypeSuggestions(expected, actual interface{}, context string) []string {
	expectedStr, _ := expected.(string)
	actualStr, _ := actual.(string)

	var suggestions []string

	expectedLower := strings.ToLower(expectedStr)
	actualLower := strings.ToLower(actualStr)

	// JSON-specific suggestions
	if strings.Contains(expectedLower, "json") {
		if strings.Contains(actualLower, "html") {
			suggestions = append(suggestions,
				"Expected JSON but received HTML - endpoint may be returning an error page",
				"Check if the API endpoint exists and is correct",
				"Verify the request method (GET vs POST) is correct",
			)
		} else if strings.Contains(actualLower, "xml") {
			suggestions = append(suggestions,
				"Expected JSON but received XML - check API documentation",
				"Verify Accept header includes 'application/json'",
				"Some APIs require format query parameter (e.g., ?format=json)",
			)
		} else if strings.Contains(actualLower, "text") {
			suggestions = append(suggestions,
				"Expected JSON but received plain text - endpoint may be misconfigured",
				"Check if the API requires authentication for JSON responses",
				"Verify the endpoint is not returning an error message",
			)
		} else {
			suggestions = append(suggestions,
				"Expected JSON but received different format",
				"Verify Accept header is set to 'application/json'",
				"Check API documentation for content negotiation",
				"Ensure request body is valid JSON if sending data",
			)
		}
	}

	// XML-specific suggestions
	if strings.Contains(expectedLower, "xml") {
		suggestions = append(suggestions,
			"Expected XML but received different format",
			"Verify Accept header is set to 'application/xml' or 'text/xml'",
			"Check API documentation for XML-specific requirements",
		)
	}

	// Form-specific suggestions
	if strings.Contains(expectedLower, "form") || strings.Contains(expectedLower, "urlencoded") {
		suggestions = append(suggestions,
			"Expected form data but received different format",
			"Set Content-Type to 'application/x-www-form-urlencoded'",
			"Ensure request body is properly formatted as key=value pairs",
			"URL-encode special characters in form data",
		)
	}

	// Multipart suggestions
	if strings.Contains(expectedLower, "multipart") {
		suggestions = append(suggestions,
			"Expected multipart data but received different format",
			"Set Content-Type to 'multipart/form-data' with boundary",
			"Ensure multipart body is correctly formatted",
			"Verify boundary parameter matches content",
		)
	}

	// General content type suggestions
	suggestions = append(suggestions,
		"Verify Content-Type header matches request body format",
		"Check if charset or boundary parameters are needed",
		"Ensure the body is properly formatted for the content type",
		"Review API documentation for required content type",
	)

	// Context-specific suggestions
	if context != "" {
		contextLower := strings.ToLower(context)
		if strings.Contains(contextLower, "post") || strings.Contains(contextLower, "put") {
			suggestions = append(suggestions,
				fmt.Sprintf("%s request - ensure Content-Type matches the data you're sending", strings.ToUpper(strings.Split(context, " ")[0])),
			)
		}
	}

	return suggestions
}

// =============================================================================
// ENHANCED STATUS CODE RANGE SUGGESTIONS
// =============================================================================

// generateEnhancedStatusCodeRangeSuggestions provides detailed suggestions for status code range mismatches.
func generateEnhancedStatusCodeRangeSuggestions(expected, actual interface{}, context string) []string {
	actualCode, ok := actual.(int)
	if !ok {
		return []string{
			"Verify the response format is correct",
			"Ensure you're checking a valid HTTP status code",
		}
	}

	var suggestions []string

	// Categorize the actual status code
	switch {
	case actualCode >= 200 && actualCode < 300:
		suggestions = append(suggestions,
			"Received success code (2xx) but expected error range",
			"The request succeeded - update test expectations if this is expected",
			"Verify the test is checking for the correct response type",
			"Check if the API behavior has changed",
		)

	case actualCode >= 300 && actualCode < 400:
		suggestions = append(suggestions,
			"Received redirect code (3xx) but expected different range",
			"Check if the API is redirecting to a different endpoint",
			"Verify redirect handling in your HTTP client",
			"Follow redirects if testing the final endpoint",
		)

	case actualCode >= 400 && actualCode < 500:
		suggestions = append(suggestions,
			"Received client error code (4xx) but expected different range",
			"Review request parameters for errors",
			"Check authentication credentials",
			"Verify the resource exists and is accessible",
			"Ensure request format matches API expectations",
		)

	case actualCode == 502:
		suggestions = append(suggestions,
			"Received Bad Gateway error - upstream server or gateway issue",
			"Check if gateway or upstream services are operational",
			"Verify network connectivity to gateway and upstream services",
			"Review load balancer and gateway configuration",
			"Implement retry logic for transient gateway failures",
		)
	case actualCode == 503:
		suggestions = append(suggestions,
			"Received Service Unavailable error - service is temporarily unavailable",
			"Check if service is undergoing maintenance or has scaling issues",
			"Verify service capacity and request scaling",
			"Implement retry with exponential backoff for temporary unavailability",
			"Review circuit breaker patterns and service health checks",
		)
	case actualCode == 504:
		suggestions = append(suggestions,
			"Received Gateway Timeout error - upstream server did not respond in time",
			"Check if gateway or upstream services are responding slowly",
			"Review request complexity and data size causing timeout",
			"Verify network latency and gateway timeout settings",
			"Implement timeout and retry logic for slow upstream services",
		)
	case actualCode >= 500 && actualCode < 600:
		suggestions = append(suggestions,
			"Received server error code (5xx) but expected different range",
			"Check if this is a temporary server issue",
			"Implement retry logic with exponential backoff",
			"Verify service status and availability",
			"Contact support if the issue persists",
		)

	default:
		suggestions = append(suggestions,
			"Received unexpected status code",
			"Check API documentation for valid status codes",
			"Verify the request is properly formatted",
		)
	}

	// Add specific code suggestions
	codeSuggestions := getStatusCodeSpecificSuggestions(actualCode)
	suggestions = append(suggestions, codeSuggestions...)

	return suggestions
}

// =============================================================================
// CORS HEADERS SUGGESTIONS
// =============================================================================

// generateCORSSuggestions provides detailed suggestions for CORS header validation failures.
func generateCORSSuggestions(expected, actual interface{}, context string) []string {
	return []string{
		"Verify CORS headers are properly configured on the server",
		"Check if Access-Control-Allow-Origin matches your request origin",
		"Ensure Access-Control-Allow-Methods includes your HTTP method",
		"Verify Access-Control-Allow-Headers includes your custom headers",
		"Check if credentials flag matches (Access-Control-Allow-Credentials)",
		"For preflight requests, ensure OPTIONS method is handled",
		"Verify CORS configuration allows your origin domain",
		"Check if wildcard origin (*) is used when credentials are required",
		"Review browser console for specific CORS errors",
	}
}

// =============================================================================
// RESPONSE STRUCTURE SUGGESTIONS
// =============================================================================

// generateResponseStructureSuggestions provides detailed suggestions for response structure validation failures.
func generateResponseStructureSuggestions(expected, actual interface{}, context string) []string {
	return []string{
		"Verify the response structure matches expected format",
		"Check if API response format has changed",
		"Review API documentation for expected response structure",
		"Ensure required fields are present in response",
		"Check if response is wrapped in a container object",
		"Verify field names match expected naming convention",
		"Check if array vs object type matches expectations",
		"Ensure response data types are correct (string, number, boolean)",
	}
}

// =============================================================================
// RESPONSE BODY SUGGESTIONS
// =============================================================================

// generateResponseBodySuggestions provides detailed suggestions for response body validation failures.
func generateResponseBodySuggestions(expected, actual interface{}, context string) []string {
	return []string{
		"Verify the response body is not empty",
		"Check if the endpoint returns data for this request",
		"Ensure response encoding is correct (UTF-8, etc.)",
		"Verify response body is valid JSON/XML",
		"Check if response body exceeds expected size limits",
		"Ensure response body matches expected content",
		"Review API documentation for expected response body",
	}
}

// =============================================================================
// RESPONSE ENCODING SUGGESTIONS
// =============================================================================

// generateResponseEncodingSuggestions provides detailed suggestions for response encoding validation failures.
func generateResponseEncodingSuggestions(expected, actual interface{}, context string) []string {
	return []string{
		"Verify response encoding is correct",
		"Check Content-Encoding header (gzip, deflate, etc.)",
		"Ensure response is properly decoded",
		"Verify character encoding (UTF-8, ASCII, etc.)",
		"Check for encoding-related characters in response",
		"Ensure HTTP client handles compression correctly",
	}
}

// =============================================================================
// AUTH HEADERS SUGGESTIONS
// =============================================================================

// generateAuthHeaderSuggestions provides detailed suggestions for authentication header validation failures.
func generateAuthHeaderSuggestions(expected, actual interface{}, context string) []string {
	return []string{
		"Verify authentication headers are correctly set",
		"Check if Authorization header format is correct (Bearer, Basic, etc.)",
		"Ensure API key or token is valid and not expired",
		"Verify credentials have required permissions",
		"Check if authentication method is supported by the API",
		"Ensure token is properly encoded (Base64 for Basic, JWT for Bearer)",
		"Review API documentation for authentication requirements",
	}
}

// =============================================================================
// CUSTOM HEADERS SUGGESTIONS
// =============================================================================

// generateCustomHeaderSuggestions provides detailed suggestions for custom header validation failures.
func generateCustomHeaderSuggestions(expected, actual interface{}, context string) []string {
	return []string{
		"Verify custom headers are correctly set",
		"Check header name format (case-insensitive, hyphens, underscores)",
		"Ensure header values are properly formatted",
		"Verify headers don't contain prohibited characters",
		"Check if custom headers are supported by the API",
		"Review API documentation for required headers",
	}
}

// =============================================================================
// JSON SCHEMA SUGGESTIONS
// =============================================================================

// generateJSONSchemaSuggestions provides detailed suggestions for JSON schema validation failures.
func generateJSONSchemaSuggestions(expected, actual interface{}, context string) []string {
	return []string{
		"Verify response matches JSON schema",
		"Validate response against JSON schema for proper validation",
		"Check if required fields are missing from response",
		"Ensure field data types match schema definition",
		"Verify field values conform to constraints (min, max, pattern)",
		"Check if array or object structure matches schema",
		"Review JSON schema definition for required format",
		"Ensure enum values match allowed values",
		"Verify format constraints (email, date-time, URI, etc.)",
	}
}

// =============================================================================
// DATA VALIDATION SUGGESTIONS
// =============================================================================

// generateDataValidationSuggestions provides detailed suggestions for data validation failures.
func generateDataValidationSuggestions(expected, actual interface{}, context string) []string {
	return []string{
		"Verify data validation rules are met",
		"Check if required fields are present",
		"Ensure field values meet validation constraints",
		"Verify data types are correct (string, number, boolean)",
		"Check if field values are within allowed ranges",
		"Ensure string values match required format or patterns",
		"Review validation rules and format requirements for this field",
		"Check if array or object structure is valid",
		"Validate field format matches expected pattern (email, date, etc.)",
	}
}

// =============================================================================
// TYPE VALIDATION SUGGESTIONS
// =============================================================================

// generateTypeValidationSuggestions provides detailed suggestions for type validation failures.
func generateTypeValidationSuggestions(expected, actual interface{}, context string) []string {
	return []string{
		"Verify field data type matches expected type",
		"Check if numeric field contains valid number",
		"Ensure boolean field contains true or false",
		"Verify string field contains text value",
		"Check if array field contains list of values",
		"Ensure object field contains structured data",
		"Review API documentation for expected field types",
		"Check for null values in non-nullable fields",
	}
}

// =============================================================================
// TIMEOUT SUGGESTIONS
// =============================================================================

// generateTimeoutSuggestions provides detailed suggestions for timeout validation failures.
func generateTimeoutSuggestions(expected, actual interface{}, context string) []string {
	return []string{
		"Request timeout occurred",
		"Increase timeout duration if operation is slow",
		"Check if server is experiencing high load",
		"Implement retry logic with exponential backoff",
		"Verify network connectivity and latency",
		"Check if the server is responding slowly",
		"Review request complexity - large requests may timeout",
		"Consider implementing request cancellation",
	}
}

// =============================================================================
// RATE LIMIT SUGGESTIONS
// =============================================================================

// generateRateLimitSuggestions provides detailed suggestions for rate limit validation failures.
func generateRateLimitSuggestions(expected, actual interface{}, context string) []string {
	return []string{
		"Rate limit exceeded",
		"Implement rate limiting and exponential backoff",
		"Check API quota limits and current usage",
		"Add caching to reduce request frequency",
		"Review rate limit headers (Retry-After, X-RateLimit-Remaining)",
		"Consider upgrading your API plan if quota is too low",
		"Implement request queuing and throttling",
		"Check if multiple clients are sharing the same quota",
	}
}

// =============================================================================
// RETRY EXCEEDED SUGGESTIONS
// =============================================================================

// generateRetryExceededSuggestions provides detailed suggestions for retry limit exceeded failures.
func generateRetryExceededSuggestions(expected, actual interface{}, context string) []string {
	return []string{
		"Retry limit exceeded",
		"Check if the operation is permanently failing",
		"Review error messages from failed attempts",
		"Implement exponential backoff for retries",
		"Consider increasing retry limit for transient failures",
		"Check if there's a configuration issue preventing success",
		"Verify the operation is valid and should succeed",
		"Contact support if issue persists after retries",
	}
}

// =============================================================================
// GENERIC VALIDATION SUGGESTIONS
// =============================================================================

// generateGenericValidationSuggestions provides suggestions for unknown or custom validation types.
func generateGenericValidationSuggestions(errorType string, expected, actual interface{}, context string) []string {
	suggestions := []string{
		fmt.Sprintf("Review the %s validation error", errorType),
	}

	if expected != nil || actual != nil {
		suggestions = append(suggestions,
			fmt.Sprintf("Expected: %v", expected),
			fmt.Sprintf("Actual: %v", actual),
		)
	}

	suggestions = append(suggestions,
		"Check API documentation for this validation type",
		"Verify request parameters match requirements",
		"Test with different values to identify the issue",
		"Review error logs for additional details",
	)

	if context != "" {
		suggestions = append(suggestions,
			fmt.Sprintf("Context: %s", context),
		)
	}

	return suggestions
}
