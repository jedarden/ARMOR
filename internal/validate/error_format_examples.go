package validate

import (
	"fmt"
)

// This file contains examples demonstrating the validation error format data structure
// and how to use it consistently across different validation scenarios.

// Example 1: Basic status code validation error
func ExampleValidationError_statusCode() {
	// Using the builder pattern
	err := NewValidationFormatter("status_code").
		WithExpected(200).
		WithActual(404).
		WithContext("GET /api/users/123").
		Format()

	fmt.Println(err.Error())
	// Output:
	// status_code validation failed
	//   Expected: 200 (OK)
	//   Received: 404 (Not Found)
	//   Request:  GET /api/users/123
	//   Common causes:
	//     1. Verify the endpoint URL is correct
	//     2. Check if the resource ID or identifier exists
	//     3. Ensure the resource hasn't been deleted or moved
	//     4. Review API versioning (URL path may have changed)
	//     5. Check if the endpoint path in your context is correct
	//     6. Review API documentation for expected status codes
	//     7. Test with a tool like curl to verify the API behavior
}

// Example 2: Using convenience function for status code errors
func ExampleValidationError_statusCodeConvenience() {
	err := FormatStatusCodeError(200, 404, "GET /api/users/123")
	fmt.Println(err.Error())
	// Output:
	// status_code validation failed
	//   Expected: 200 (OK)
	//   Received: 404 (Not Found)
	//   Request:  GET /api/users/123
	//   Common causes:
	//     1. Verify the endpoint URL is correct
	//     2. Check if the resource ID or identifier exists
	//     3. Ensure the resource hasn't been deleted or moved
	//     4. Review API versioning (URL path may have changed)
	//     5. Check if the endpoint path in your context is correct
	//     6. Review API documentation for expected status codes
	//     7. Test with a tool like curl to verify the API behavior
}

// Example 3: Multiple expected status codes
func ExampleValidationError_multipleCodes() {
	err := NewValidationFormatter("status_code").
		WithExpected([]int{200, 201, 204}).
		WithActual(500).
		WithContext("POST /api/orders").
		Format()

	fmt.Println(err.Error())
	// Output:
	// status_code validation failed
	//   Expected: one of [200 (OK), 201 (Created), 204 (No Content)]
	//   Received: 500 (Internal Server Error)
	//   Request:  POST /api/orders
	//   Common causes:
	//     1. Implement retry logic with exponential backoff
	//     2. Check service status page for ongoing issues
	//     3. Contact support if the issue persists
	//     4. Verify request doesn't trigger server-side bugs
	//     5. Review server logs if accessible
}

// Example 4: Error message pattern validation
func ExampleValidationError_errorMessage() {
	err := FormatErrorMessageError(
		"invalid.*token",
		"access_denied",
		"error",
		"OAuth token validation",
	)

	fmt.Println(err.Error())
	// Output:
	// error_message validation failed
	//   Expected: invalid.*token
	//   Actual:   access_denied
	//   Context:  OAuth token validation
	//   Field:    error
	//   Common causes:
	//     1. Review the error message for specific details
	//     2. Check API documentation for this error type
	//     3. Verify request parameters match requirements
	//     4. Test with different parameter values
	//     5. OAuth-related validation - check token validity and scopes
}

// Example 5: Error message with response snippet
func ExampleValidationError_errorMessageWithSnippet() {
	err := NewValidationFormatter("error_message").
		WithExpected("not found").
		WithActual("internal server error").
		WithFieldName("message").
		WithContext("Resource lookup").
		WithResponseSnippet(`{"message": "internal server error", "code": "ERR_500"}`).
		Format()

	fmt.Println(err.Error())
	// Output:
	// error_message validation failed
	//   Expected: not found
	//   Actual:   internal server error
	//   Context:  Resource lookup
	//   Field:    message
	//   Response: {"message": "internal server error", "code": "ERR_500"}
	//   Common causes:
	//     1. Server error occurred
	//     2. Check service status page for ongoing issues
	//     3. Implement retry logic with exponential backoff
	//     4. Contact support if the issue persists
	//     5. Review server logs if accessible
}

// Example 6: Status code range validation
func ExampleValidationError_statusCodeRange() {
	err := FormatStatusCodeRangeError("4xx", 200, "error response check", "status_code")

	fmt.Println(err.Error())
	// Output:
	// status_code_range validation failed
	//   Expected: 4xx (Client Error)
	//   Actual:   200
	//   Context:  error response check
	//   Range:    400-499 (Client Error)
	//   Details:
	//     1. Status code 200 is outside range 400-499
	//   Common causes:
	//     1. Received success code (2xx) but expected error range
	//     2. The request succeeded - update test expectations if this is expected
	//     3. Verify the test is checking for the correct response type
	//     4. Check if the API behavior has changed
}

// Example 7: Content-Type validation
func ExampleValidationError_contentType() {
	err := FormatContentTypeError(
		"application/json",
		"text/html",
		"API response",
	)

	fmt.Println(err.Error())
	// Output:
	// content_type validation failed
	//   Expected: application/json
	//   Actual:   text/html
	//   Context:  API response
	//   Common causes:
	//     1. Expected JSON but received HTML - endpoint may be returning an error page
	//     2. Check if the API endpoint exists and is correct
	//     3. Verify the request method (GET vs POST) is correct
	//     4. Verify Accept header is set to 'application/json'
	//     5. Ensure request body is valid JSON if sending data
}

// Example 8: Custom validation with format options
func ExampleValidationError_customWithOptions() {
	err := FormatCustomValidationError(
		"header_validation",
		"Bearer Token",
		"",
		WithContext("Authorization header check"),
		WithResponseSnippet(`{"headers": {"Content-Type": "application/json"}}`),
		WithValidationDetails(
			"Authorization header is missing",
			"Expected 'Bearer' token format",
		),
		WithSuggestions(
			"Add Authorization header with Bearer token",
			"Verify token is properly formatted",
			"Check if authentication is required for this endpoint",
		),
	)

	fmt.Println(err.Error())
	// Output:
	// header_validation validation failed
	//   Expected: Bearer Token
	//   Actual:
	//   Context:  Authorization header check
	//   Response: {"headers": {"Content-Type": "application/json"}}
	//   Details:
	//     1. Authorization header is missing
	//     2. Expected 'Bearer' token format
	//   Common causes:
	//     1. Add Authorization header with Bearer token
	//     2. Verify token is properly formatted
	//     3. Check if authentication is required for this endpoint
}

// Example 9: Custom validation with pattern details
func ExampleValidationError_withPatternDetails() {
	err := NewValidationFormatter("error_message").
		WithExpected("authentication.*failed").
		WithActual("access denied").
		WithFieldName("error").
		WithContext("Authentication check").
		WithPatternDetails("regex pattern 'authentication.*failed' did not match").
		WithValidationDetails(
			"Searched for regex pattern 'authentication.*failed' in field 'error'",
			"Actual message: \"access denied\"",
		).
		Format()

	fmt.Println(err.Error())
	// Output:
	// error_message validation failed
	//   Expected: authentication.*failed
	//   Actual:   access denied
	//   Context:  Authentication check
	//   Field:    error
	//   Pattern:  regex pattern 'authentication.*failed' did not match
	//   Details:
	//     1. Searched for regex pattern 'authentication.*failed' in field 'error'
	//     2. Actual message: "access denied"
	//   Common causes:
	//     1. Authentication/authorization failed
	//     2. Verify your account has permission to access this resource
	//     3. Check if additional authorization scopes are required
	//     4. Review API key or token permissions
}

// Example 10: Complex validation with multiple details
func ExampleValidationError_complexValidation() {
	err := NewValidationFormatter("response_validation").
		WithExpected("valid JSON response with user data").
		WithActual("malformed JSON").
		WithContext("POST /api/users").
		WithResponseSnippet(`{"id": 123, "name": "John", "email": "john@example`).
		WithValidationDetails(
			"JSON parsing failed at position 56",
			"Unterminated string value",
			"Missing closing brace }",
		).
		WithSuggestions(
			"Verify JSON is properly formatted",
			"Check for special characters that need escaping",
			"Use JSON linter to validate format",
			"Review request body encoding",
		).
		Format()

	fmt.Println(err.Error())
	// Output:
	// response_validation validation failed
	//   Expected: valid JSON response with user data
	//   Actual:   malformed JSON
	//   Context:  POST /api/users
	//   Response: {"id": 123, "name": "John", "email": "john@example
	//   Details:
	//     1. JSON parsing failed at position 56
	//     1. Unterminated string value
	//     1. Missing closing brace }
	//   Common causes:
	//     Verify. JSON is properly formatted
	//     1. Check for special characters that need escaping
	//     Use. JSON linter to validate format
	//     1. Review request body encoding
}

// Example 11: Accessing ValidationError fields programmatically
func ExampleValidationError_programmaticAccess() {
	err := FormatStatusCodeError(200, 404, "GET /api/users")

	// Access fields for programmatic handling
	validationErr := err
	if validationErr.ErrorType == "status_code" {
		if actualCode, ok := validationErr.Actual.(int); ok {
			if actualCode == 404 {
				fmt.Println("Resource not found - check endpoint and ID")
			}
		}
	}

	fmt.Printf("Type: %s\n", validationErr.ErrorType)
	fmt.Printf("Expected: %v\n", validationErr.Expected)
	fmt.Printf("Actual: %v\n", validationErr.Actual)
	fmt.Printf("Has %d suggestions\n", len(validationErr.Suggestions))

	// Output:
	// Resource not found - check endpoint and ID
	// Type: status_code
	// Expected: 200
	// Actual: 404
	// Has 3 suggestions
}

// Example 12: ValidationError as error interface
func ExampleValidationError_asError() {
	var err error
	err = FormatStatusCodeError(200, 500, "GET /api/data")

	// ValidationError implements error interface
	if err != nil {
		fmt.Println("Error occurred:", err.Error())
	}

	// Type assertion to access ValidationError
	if validationErr, ok := err.(ValidationError); ok {
		fmt.Printf("Validation type: %s\n", validationErr.ErrorType)
		fmt.Printf("Context: %s\n", validationErr.Context)
	}

	// Output:
	// Error occurred: status_code validation failed
	//   Expected: 200 (OK)
	//   Actual:   500 (Internal Server Error)
	//   Context:  GET /api/data
	//   Common causes:
	//     1. Implement retry logic with exponential backoff
	//     1. Check service status page for ongoing issues
	//     1. Contact support if the issue persists
	// Validation type: status_code
	// Context: GET /api/data
}

// Example 13: Creating validation errors in tests
func ExampleValidationError_inTest() {
	// Simulate a test that expects status 200 but gets 401
	expectedStatus := 200
	actualStatus := 401
	context := "GET /api/protected"

	if expectedStatus != actualStatus {
		err := FormatStatusCodeError(expectedStatus, actualStatus, context)
		fmt.Printf("Test failed: %v\n", err.Error())
	}

	// Output:
	// Test failed: status_code validation failed
	//   Expected: 200 (OK)
	//   Actual:   401 (Unauthorized)
	//   Context:  GET /api/protected
	//   Common causes:
	//     1. Verify authentication credentials are correct
	//     1. Check if API token or session has expired
	//     Ensure. Authorization header is properly formatted
}

// Example 14: Conditional field population
func ExampleValidationError_conditionalFields() {
	baseError := NewValidationFormatter("status_code").
		WithExpected(200).
		WithActual(404)

	// Add context only if available
	context := "GET /api/users/123"
	if context != "" {
		baseError = baseError.WithContext(context)
	}

	// Add response snippet only if response body is available
	responseBody := `{"error": "User not found"}`
	if responseBody != "" {
		baseError = baseError.WithResponseSnippet(responseBody)
	}

	err := baseError.Format()
	fmt.Println(err.Error())

	// Output:
	// status_code validation failed
	//   Expected: 200 (OK)
	//   Actual:   404 (Not Found)
	//   Context:  GET /api/users/123
	//   Response: {"error": "User not found"}
	//   Common causes:
	//     1. Verify the endpoint URL is correct
	//     1. Check if the resource ID or identifier exists
	//     1. Ensure the resource hasn't been deleted or moved
}

// Example 15: Reusable validation helper
func ExampleValidationError_reusableHelper() {
	// Define a reusable helper function
	checkStatusCode := func(expected, actual int, operation string) error {
		if expected != actual {
			return FormatStatusCodeError(expected, actual, operation)
		}
		return nil
	}

	// Use the helper in various scenarios
	scenarios := []struct {
		name     string
		expected int
		actual   int
		context  string
	}{
		{"User lookup", 200, 200, "GET /api/users/123"},
		{"Create user", 201, 409, "POST /api/users"},
		{"Delete user", 204, 403, "DELETE /api/users/123"},
	}

	for _, scenario := range scenarios {
		if err := checkStatusCode(scenario.expected, scenario.actual, scenario.context); err != nil {
			fmt.Printf("%s failed: %v\n", scenario.name, err)
		} else {
			fmt.Printf("%s succeeded\n", scenario.name)
		}
	}

	// Output:
	// User lookup succeeded
	// Create user failed: status_code validation failed
	//   Expected: 201 (Created)
	//   Actual:   409 (Conflict)
	//   Context:  POST /api/users
	//   Common causes:
	//     1. Check if a resource with this identifier already exists
	//     1. Verify unique constraints on the resource
	//     1. Consider using PUT instead of POST for updates
	// Delete user failed: status_code validation failed
	//   Expected: 204 (No Content)
	//   Actual:   403 (Forbidden)
	//   Context:  DELETE /api/users/123
	//   Common causes:
	//     1. Verify your account has permission to access this resource
	//     1. Check if additional scopes or roles are required
	//     Review. API documentation for required permissions
}