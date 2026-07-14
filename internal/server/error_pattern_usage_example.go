package server

// This file demonstrates comprehensive usage of the predefined error patterns.
// It serves as documentation for how to use CommonErrorPatterns,
// AuthErrorPatterns, ClientErrorPatterns, and ServerErrorPatterns.
//
// # Error Pattern Usage Guide
//
// This file contains usage examples for all error pattern collections and
// helper functions. Each example demonstrates a specific use case with
// detailed explanations.
//
// # Pattern Collections
//
//   - CommonErrorPatterns: General-purpose patterns for common S3 errors
//   - AuthErrorPatterns: Authentication and authorization failures
//   - ClientErrorPatterns: 4xx client-side errors
//   - ServerErrorPatterns: 5xx server-side errors
//
// # Helper Functions
//
//   - PatternForCode(): Get pattern by error code
//   - PatternsForCategory(): Get all patterns for a category
//   - AllCommonPatterns(): Get all common patterns
//
// # Quick Reference
//
// 1. Use a pattern directly: CommonErrorPatterns.ResourceNotFound
// 2. Get pattern by code: PatternForCode("NoSuchKey")
// 3. Get patterns by category: PatternsForCategory(CategoryAuth)
// 4. Get all common patterns: AllCommonPatterns()

import (
	"fmt"
	"time"
)

// =============================================================================
// SECTION 1: BASIC PATTERN ACCESS
// =============================================================================
// This section demonstrates how to access and use predefined error patterns.
// =============================================================================

// ExampleUsage demonstrates how to use the predefined error patterns.
//
// This example shows the basic pattern access patterns available in the
// framework. Use this as a quick reference for getting started.
func ExampleUsage() {
	// Access common error patterns directly
	notFoundPattern := CommonErrorPatterns.ResourceNotFound
	fmt.Printf("Pattern: %s\n", notFoundPattern.Name)
	fmt.Printf("Expected Code: %s\n", notFoundPattern.ExpectedCode)
	fmt.Printf("Expected Status: %d\n", notFoundPattern.ExpectedStatus)

	// Access authentication error patterns
	authPattern := AuthErrorPatterns.MissingAuthHeader
	fmt.Printf("Auth Pattern: %s\n", authPattern.Name)

	// Access client error patterns (4xx)
	clientPattern := ClientErrorPatterns.NotFound
	fmt.Printf("Client Pattern: %s\n", clientPattern.Name)

	// Access server error patterns (5xx)
	serverPattern := ServerErrorPatterns.InternalError
	fmt.Printf("Server Pattern: %s\n", serverPattern.Name)
}

// =============================================================================
// SECTION 2: COMMON ERROR PATTERNS
// =============================================================================
// This section demonstrates usage of CommonErrorPatterns.
//
// CommonErrorPatterns provide ready-to-use configurations for the most
// frequently encountered S3 error scenarios. Use these when testing
// standard error responses.
//
// Available patterns:
//   - ResourceNotFound: 404 errors for missing objects
//   - AccessDenied: 403 errors for authentication failures
//   - InvalidRequest: 400 errors for malformed requests
//   - UnsupportedMediaType: 415 errors for content-type validation
//   - MethodNotAllowed: 405 errors for HTTP method restrictions
//   - InternalServerError: 500 errors for server failures
//   - SignatureMismatch: 403 errors for AWS signature validation
//   - RequestExpired: 403 errors for expired timestamps
// =============================================================================

// ExampleCommonPatterns demonstrates usage of CommonErrorPatterns.
//
// Use this example when:
//   - Testing standard S3 error scenarios
//   - Validating error response structure
//   - Writing integration tests for error handling
func ExampleCommonPatterns() {
	// Example 1: Resource not found pattern
	notFound := CommonErrorPatterns.ResourceNotFound
	fmt.Printf("Name: %s\n", notFound.Name)
	fmt.Printf("Expected Status: %d\n", notFound.ExpectedStatus)
	fmt.Printf("Expected Code: %s\n", notFound.ExpectedCode)
	fmt.Printf("Expected Message: %s\n", notFound.ExpectedMessage)
	fmt.Printf("Max Response Time: %v\n", notFound.MaxResponseTime)

	// Example 2: Access denied pattern
	accessDenied := CommonErrorPatterns.AccessDenied
	fmt.Printf("Name: %s\n", accessDenied.Name)
	fmt.Printf("Category: %s\n", accessDenied.Category)

	// Example 3: Invalid request pattern
	invalidRequest := CommonErrorPatterns.InvalidRequest
	fmt.Printf("Description: %s\n", invalidRequest.Description)

	// Example 4: Iterating over all common patterns
	allPatterns := AllCommonPatterns()
	for i, pattern := range allPatterns {
		fmt.Printf("%d. %s: %s (%d)\n",
			i+1, pattern.Name, pattern.ExpectedCode, pattern.ExpectedStatus)
	}
}

// ExamplePatternInValidation demonstrates using patterns for validation.
//
// Use this example when:
//   - Validating actual error responses against patterns
//   - Writing test assertions for error responses
//   - Checking response times and message formats
func ExamplePatternInValidation() {
	// Simulated validation logic
	pattern := CommonErrorPatterns.ResourceNotFound

	// Check HTTP status
	actualStatus := 404
	if actualStatus != pattern.ExpectedStatus {
		fmt.Printf("Status mismatch: got %d, want %d\n",
			actualStatus, pattern.ExpectedStatus)
	}

	// Check error code
	actualCode := "NoSuchKey"
	if actualCode != pattern.ExpectedCode {
		fmt.Printf("Code mismatch: got %s, want %s\n",
			actualCode, pattern.ExpectedCode)
	}

	// Check message length
	message := "The specified key does not exist"
	if len(message) < pattern.MinMessageLength {
		fmt.Printf("Message too short: %d chars, want at least %d\n",
			len(message), pattern.MinMessageLength)
	}

	// Check response time
	responseTime := 150 * time.Millisecond
	if responseTime > pattern.MaxResponseTime {
		fmt.Printf("Response time too slow: %v, want < %v\n",
			responseTime, pattern.MaxResponseTime)
	}
}

// =============================================================================
// SECTION 3: AUTHENTICATION ERROR PATTERNS
// =============================================================================
// This section demonstrates usage of AuthErrorPatterns.
//
// AuthErrorPatterns provide configurations for all authentication failure
// scenarios. Use these when testing authentication, authorization, and
// AWS signature validation.
//
// Available patterns:
//   - MissingAuthHeader: No Authorization header present
//   - InvalidAccessKeyId: Invalid or non-existent access key
//   - SignatureDoesNotMatch: Incorrect signature calculation
//   - MissingDateHeader: Required date header is missing
//   - RequestExpired: Request timestamp is too old
//   - MalformedAuthHeader: Invalid Authorization header format
// =============================================================================

// ExampleAuthPatterns demonstrates usage of AuthErrorPatterns.
//
// Use this example when:
//   - Testing authentication failure scenarios
//   - Validating AWS signature behavior
//   - Writing security-focused tests
func ExampleAuthPatterns() {
	// Example 1: Missing authentication header
	missingAuth := AuthErrorPatterns.MissingAuthHeader
	fmt.Printf("Name: %s\n", missingAuth.Name)
	fmt.Printf("Expected Code: %s\n", missingAuth.ExpectedCode)
	fmt.Printf("Expected Status: %d\n", missingAuth.ExpectedStatus)
	fmt.Printf("Max Response Time: %v\n", missingAuth.MaxResponseTime)

	// Example 2: Invalid access key ID
	invalidKey := AuthErrorPatterns.InvalidAccessKeyId
	fmt.Printf("Description: %s\n", invalidKey.Description)

	// Example 3: Signature does not match
	sigMismatch := AuthErrorPatterns.SignatureDoesNotMatch
	fmt.Printf("Min Message Length: %d\n", sigMismatch.MinMessageLength)
	fmt.Printf("Expected Keywords: %v\n", sigMismatch.ExpectedKeywords)

	// Example 4: Request expired
	expired := AuthErrorPatterns.RequestExpired
	fmt.Printf("Category: %s\n", expired.Category)

	// Example 5: Get all auth patterns
	authPatterns := PatternsForCategory(CategoryAuth)
	fmt.Printf("Total auth patterns: %d\n", len(authPatterns))
}

// ExampleAuthInTesting demonstrates using auth patterns in test scenarios.
//
// Use this example when:
//   - Writing table-driven tests for authentication
//   - Creating comprehensive auth test suites
//   - Testing multiple auth failure scenarios
func ExampleAuthInTesting() {
	// Define auth test scenarios
	authScenarios := []struct {
		name    string
		pattern ErrorScenarioConfig
		testURL string
	}{
		{
			name:    "Missing Auth Header",
			pattern: AuthErrorPatterns.MissingAuthHeader,
			testURL: "/test-bucket/key",
		},
		{
			name:    "Invalid Access Key",
			pattern: AuthErrorPatterns.InvalidAccessKeyId,
			testURL: "/test-bucket/key",
		},
		{
			name:    "Signature Mismatch",
			pattern: AuthErrorPatterns.SignatureDoesNotMatch,
			testURL: "/test-bucket/key",
		},
		{
			name:    "Request Expired",
			pattern: AuthErrorPatterns.RequestExpired,
			testURL: "/test-bucket/key",
		},
	}

	// Simulate running tests
	for _, scenario := range authScenarios {
		fmt.Printf("Test: %s\n", scenario.name)
		fmt.Printf("  Pattern: %s\n", scenario.pattern.Name)
		fmt.Printf("  Expected Status: %d\n", scenario.pattern.ExpectedStatus)
		fmt.Printf("  Test URL: %s\n", scenario.testURL)
	}
}

// =============================================================================
// SECTION 4: CLIENT ERROR PATTERNS (4xx)
// =============================================================================
// This section demonstrates usage of ClientErrorPatterns.
//
// ClientErrorPatterns provide configurations for 4xx client-side errors.
// These are errors where the client made an invalid request and should
// correct their request before retrying.
//
// Available patterns:
//   - BadRequest: Generic 400 Bad Request errors
//   - NotFound: 404 Not Found errors
//   - MethodNotAllowed: 405 Method Not Allowed errors
//   - UnsupportedMediaType: 415 Unsupported Media Type errors
// =============================================================================

// ExampleClientPatterns demonstrates usage of ClientErrorPatterns.
//
// Use this example when:
//   - Testing 4xx client error responses
//   - Validating request validation logic
//   - Testing API contract compliance
func ExampleClientPatterns() {
	// Example 1: Bad request pattern
	badRequest := ClientErrorPatterns.BadRequest
	fmt.Printf("Name: %s\n", badRequest.Name)
	fmt.Printf("Expected Status: %d\n", badRequest.ExpectedStatus)
	fmt.Printf("Expected Code: %s\n", badRequest.ExpectedCode)

	// Example 2: Not found pattern (reuses CommonErrorPatterns)
	notFound := ClientErrorPatterns.NotFound
	fmt.Printf("Name: %s\n", notFound.Name)
	fmt.Printf("Description: %s\n", notFound.Description)

	// Example 3: Method not allowed pattern
	methodNotAllowed := ClientErrorPatterns.MethodNotAllowed
	fmt.Printf("Expected Message: %s\n", methodNotAllowed.ExpectedMessage)

	// Example 4: Unsupported media type pattern
	unsupportedMedia := ClientErrorPatterns.UnsupportedMediaType
	fmt.Printf("Max Response Time: %v\n", unsupportedMedia.MaxResponseTime)
}

// ExampleClientErrorsInAPI demonstrates using client patterns for API testing.
//
// Use this example when:
//   - Testing API endpoint error handling
//   - Validating client error responses
//   - Writing API integration tests
func ExampleClientErrorsInAPI() {
	// Define API error scenarios
	apiScenarios := []struct {
		method  string
		path    string
		pattern ErrorScenarioConfig
	}{
		{
			method:  "POST",
			path:    "/api/blobs/file.txt",
			pattern: ClientErrorPatterns.MethodNotAllowed,
		},
		{
			method:  "GET",
			path:    "/api/blobs/nonexistent.txt",
			pattern: ClientErrorPatterns.NotFound,
		},
		{
			method:  "PUT",
			path:    "/api/blobs/file.txt",
			pattern: ClientErrorPatterns.UnsupportedMediaType,
		},
	}

	// Simulate API testing
	for _, scenario := range apiScenarios {
		fmt.Printf("API Test: %s %s\n", scenario.method, scenario.path)
		fmt.Printf("  Expected Error: %s (%d)\n",
			scenario.pattern.ExpectedCode,
			scenario.pattern.ExpectedStatus)
	}
}

// =============================================================================
// SECTION 5: SERVER ERROR PATTERNS (5xx)
// =============================================================================
// This section demonstrates usage of ServerErrorPatterns.
//
// ServerErrorPatterns provide configurations for 5xx server-side errors.
// These are errors where the server failed to fulfill a valid request.
//
// Available patterns:
//   - InternalError: 500 Internal Server Error
//   - ServiceUnavailable: 503 Service Unavailable
// =============================================================================

// ExampleServerPatterns demonstrates usage of ServerErrorPatterns.
//
// Use this example when:
//   - Testing server error handling
//   - Verifying server failure responses
//   - Testing circuit breaker logic
func ExampleServerPatterns() {
	// Example 1: Internal error pattern
	internalError := ServerErrorPatterns.InternalError
	fmt.Printf("Name: %s\n", internalError.Name)
	fmt.Printf("Expected Status: %d\n", internalError.ExpectedStatus)
	fmt.Printf("Expected Code: %s\n", internalError.ExpectedCode)
	fmt.Printf("Max Response Time: %v\n", internalError.MaxResponseTime)

	// Example 2: Service unavailable pattern
	serviceUnavailable := ServerErrorPatterns.ServiceUnavailable
	fmt.Printf("Name: %s\n", serviceUnavailable.Name)
	fmt.Printf("Expected Status: %d\n", serviceUnavailable.ExpectedStatus)
	fmt.Printf("Description: %s\n", serviceUnavailable.Description)

	// Example 3: Comparing server error patterns
	fmt.Printf("Internal Error Time: %v\n", internalError.MaxResponseTime)
	fmt.Printf("Service Unavailable Time: %v\n", serviceUnavailable.MaxResponseTime)
}

// ExampleServerErrorsInResilience demonstrates using server patterns for resilience testing.
//
// Use this example when:
//   - Testing error recovery logic
//   - Verifying retry behavior
//   - Testing circuit breaker implementations
func ExampleServerErrorsInResilience() {
	// Define resilience test scenarios
	resilienceScenarios := []struct {
		scenario string
		pattern  ErrorScenarioConfig
		action   string
	}{
		{
			scenario: "Database Failure",
			pattern:  ServerErrorPatterns.InternalError,
			action:   "Retry with exponential backoff",
		},
		{
			scenario: "Maintenance Mode",
			pattern:  ServerErrorPatterns.ServiceUnavailable,
			action:   "Wait and retry",
		},
		{
			scenario: "Overload",
			pattern:  ServerErrorPatterns.ServiceUnavailable,
			action:   "Circuit breaker open",
		},
	}

	// Simulate resilience testing
	for _, scenario := range resilienceScenarios {
		fmt.Printf("Scenario: %s\n", scenario.scenario)
		fmt.Printf("  Pattern: %s (%d)\n",
			scenario.pattern.Name,
			scenario.pattern.ExpectedStatus)
		fmt.Printf("  Action: %s\n", scenario.action)
	}
}

// =============================================================================
// SECTION 6: HELPER FUNCTIONS
// =============================================================================
// This section demonstrates usage of helper functions for pattern access.
//
// Helper functions provide dynamic access to patterns based on error codes,
// categories, and other criteria.
//
// Available functions:
//   - PatternForCode(): Get pattern by error code
//   - PatternsForCategory(): Get all patterns for a category
//   - AllCommonPatterns(): Get all common patterns
// =============================================================================

// ExamplePatternForCode demonstrates using PatternForCode to get a pattern.
//
// Use this function when:
//   - Looking up patterns dynamically from error codes
//   - Handling errors from external sources
//   - Building flexible validation logic
//
// Example scenario:
//   - You receive an error response with code "NoSuchKey"
//   - You need to get the corresponding test pattern
//   - Use PatternForCode("NoSuchKey") to get the pattern
func ExamplePatternForCode() {
	// Example 1: Get pattern for known error code
	pattern := PatternForCode("NoSuchKey")
	fmt.Printf("Pattern for NoSuchKey: %s\n", pattern.Name)
	fmt.Printf("Expected status: %d\n", pattern.ExpectedStatus)
	fmt.Printf("Expected code: %s\n", pattern.ExpectedCode)

	// Example 2: Get pattern for authentication error
	authPattern := PatternForCode("AccessDenied")
	fmt.Printf("Auth pattern: %s\n", authPattern.Name)

	// Example 3: Get pattern for unknown code (returns default)
	unknownPattern := PatternForCode("UnknownCode")
	fmt.Printf("Unknown pattern: %s\n", unknownPattern.Name)
	fmt.Printf("Unknown pattern status: %d\n", unknownPattern.ExpectedStatus)

	// Example 4: Dynamic pattern lookup in tests
	errorCodes := []string{"NoSuchKey", "AccessDenied", "InvalidRequest"}
	for _, code := range errorCodes {
		pattern := PatternForCode(code)
		fmt.Printf("Code %s → Pattern %s\n", code, pattern.Name)
	}
}

// ExamplePatternsForCategory demonstrates using PatternsForCategory.
//
// Use this function when:
//   - Testing all errors of a specific type
//   - Building category-specific test suites
//   - Organizing tests by error category
func ExamplePatternsForCategory() {
	// Example 1: Get all authentication patterns
	authPatterns := PatternsForCategory(CategoryAuth)
	fmt.Printf("Found %d auth patterns\n", len(authPatterns))

	for _, pattern := range authPatterns {
		fmt.Printf("- %s: %s\n", pattern.ExpectedCode, pattern.Name)
	}

	// Example 2: Get all not found patterns
	notFoundPatterns := PatternsForCategory(CategoryNotFound)
	fmt.Printf("Found %d not found patterns\n", len(notFoundPatterns))

	// Example 3: Get all internal error patterns
	internalPatterns := PatternsForCategory(CategoryInternal)
	fmt.Printf("Found %d internal error patterns\n", len(internalPatterns))

	// Example 4: Handle unknown category
	unknownPatterns := PatternsForCategory("UnknownCategory")
	fmt.Printf("Unknown category patterns: %d\n", len(unknownPatterns))
}

// ExampleAllCommonPatterns demonstrates using AllCommonPatterns.
//
// Use this function when:
//   - Testing all common error scenarios
//   - Building comprehensive test suites
//   - Iterating over all common patterns
func ExampleAllCommonPatterns() {
	// Example 1: Get all common patterns
	allPatterns := AllCommonPatterns()
	fmt.Printf("Total common patterns: %d\n", len(allPatterns))

	for _, pattern := range allPatterns {
		fmt.Printf("- %s (%d): %s\n",
			pattern.ExpectedCode,
			pattern.ExpectedStatus,
			pattern.Name)
	}

	// Example 2: Count patterns by status code
	statusCounts := make(map[int]int)
	for _, pattern := range allPatterns {
		statusCounts[pattern.ExpectedStatus]++
	}
	fmt.Printf("Status code distribution: %v\n", statusCounts)

	// Example 3: Find slowest pattern
	slowestPattern := allPatterns[0]
	for _, pattern := range allPatterns {
		if pattern.MaxResponseTime > slowestPattern.MaxResponseTime {
			slowestPattern = pattern
		}
	}
	fmt.Printf("Slowest pattern: %s (%v)\n",
		slowestPattern.Name,
		slowestPattern.MaxResponseTime)
}

// ExampleCustomPattern demonstrates creating a custom pattern based on a predefined one.
func ExampleCustomPattern() {
	// Start with a predefined pattern
	basePattern := CommonErrorPatterns.ResourceNotFound

	// Create custom configuration
	customConfig := ErrorScenarioConfig{
		Name:              "Custom Not Found",
		ExpectedCode:      basePattern.ExpectedCode,
		ExpectedStatus:    basePattern.ExpectedStatus,
		ExpectedMessage:   "Custom not found message",
		ExpectedKeywords:  []string{"custom", "not", "found"},
		MinMessageLength:   basePattern.MinMessageLength,
		MaxResponseTime:   basePattern.MaxResponseTime,
		Description:       "Custom not found scenario",
		Category:          basePattern.Category,
	}

	fmt.Printf("Custom pattern: %s\n", customConfig.Name)
}

// ExamplePatternValidation demonstrates using patterns for validation.
func ExamplePatternValidation() {
	pattern := CommonErrorPatterns.ResourceNotFound

	// Use pattern values for validation
	expectedStatus := pattern.ExpectedStatus
	expectedCode := pattern.ExpectedCode
	minMessageLength := pattern.MinMessageLength
	maxResponseTime := pattern.MaxResponseTime

	// Simulated validation
	fmt.Printf("Validating against pattern: %s\n", pattern.Name)
	fmt.Printf("- Expected status: %d\n", expectedStatus)
	fmt.Printf("- Expected code: %s\n", expectedCode)
	fmt.Printf("- Min message length: %d\n", minMessageLength)
	fmt.Printf("- Max response time: %v\n", maxResponseTime)
}

// ExamplePatternMetadata demonstrates accessing pattern metadata.
func ExamplePatternMetadata() {
	pattern := AuthErrorPatterns.SignatureDoesNotMatch

	// Access metadata
	fmt.Printf("Pattern Metadata:\n")
	fmt.Printf("- Name: %s\n", pattern.Name)
	fmt.Printf("- Description: %s\n", pattern.Description)
	fmt.Printf("- Category: %s\n", pattern.Category)
	fmt.Printf("- Max Response Time: %v\n", pattern.MaxResponseTime)
	fmt.Printf("- Expected Keywords: %v\n", pattern.ExpectedKeywords)
}

// ExamplePatternInTesting demonstrates using patterns in test scenarios.
func ExamplePatternInTesting() {
	// Define test scenario using pattern
	scenarios := []struct {
		pattern ErrorScenarioConfig
		testURL string
	}{
		{
			pattern: CommonErrorPatterns.ResourceNotFound,
			testURL: "/test-bucket/nonexistent-key",
		},
		{
			pattern: AuthErrorPatterns.MissingAuthHeader,
			testURL: "/test-bucket/key",
		},
		{
			pattern: ClientErrorPatterns.MethodNotAllowed,
			testURL: "/test-bucket/key",
		},
	}

	for _, scenario := range scenarios {
		fmt.Printf("Test: %s -> %s\n",
			scenario.pattern.Name,
			scenario.testURL)
	}
}

// ExamplePatternComparison demonstrates comparing patterns.
func ExamplePatternComparison() {
	// Compare different patterns
	notFound := CommonErrorPatterns.ResourceNotFound
	accessDenied := CommonErrorPatterns.AccessDenied

	fmt.Printf("Pattern Comparison:\n")
	fmt.Printf("- NotFound status: %d, Denied status: %d\n",
		notFound.ExpectedStatus,
		accessDenied.ExpectedStatus)
	fmt.Printf("- NotFound code: %s, Denied code: %s\n",
		notFound.ExpectedCode,
		accessDenied.ExpectedCode)
	fmt.Printf("- NotFound category: %s, Denied category: %s\n",
		notFound.Category,
		accessDenied.Category)
}

// ExampleDefaultPatternUsage demonstrates using DefaultErrorScenarioConfig.
func ExampleDefaultPatternUsage() {
	// Get default configuration
	defaultConfig := DefaultErrorScenarioConfig()

	fmt.Printf("Default configuration:\n")
	fmt.Printf("- Min message length: %d\n", defaultConfig.MinMessageLength)
	fmt.Printf("- Category: %s\n", defaultConfig.Category)

	// Customize default configuration
	customConfig := defaultConfig
	customConfig.Name = "Custom Error"
	customConfig.ExpectedCode = "CustomCode"
	customConfig.ExpectedStatus = 400

	fmt.Printf("Customized config: %s\n", customConfig.Name)
}

// ExampleResponseTimeValidation demonstrates validating response times.
func ExampleResponseTimeValidation(actualDuration time.Duration) {
	pattern := CommonErrorPatterns.ResourceNotFound

	// Validate response time
	if actualDuration > pattern.MaxResponseTime {
		fmt.Printf("Response time %v exceeded maximum %v\n",
			actualDuration,
			pattern.MaxResponseTime)
	} else {
		fmt.Printf("Response time %v is within limit %v\n",
			actualDuration,
			pattern.MaxResponseTime)
	}
}
