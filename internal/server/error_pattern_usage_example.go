package server

// This file demonstrates usage of the predefined error patterns.
// It serves as documentation for how to use CommonErrorPatterns,
// AuthErrorPatterns, ClientErrorPatterns, and ServerErrorPatterns.

import (
	"fmt"
	"time"
)

// ExampleUsage demonstrates how to use the predefined error patterns.
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

// ExamplePatternForCode demonstrates using PatternForCode to get a pattern.
func ExamplePatternForCode() {
	// Get pattern by error code
	pattern := PatternForCode("NoSuchKey")
	fmt.Printf("Pattern for NoSuchKey: %s\n", pattern.Name)
	fmt.Printf("Expected status: %d\n", pattern.ExpectedStatus)

	// Get pattern for unknown code (returns default)
	unknownPattern := PatternForCode("UnknownCode")
	fmt.Printf("Unknown pattern: %s\n", unknownPattern.Name)
}

// ExamplePatternsForCategory demonstrates using PatternsForCategory.
func ExamplePatternsForCategory() {
	// Get all authentication patterns
	authPatterns := PatternsForCategory(CategoryAuth)
	fmt.Printf("Found %d auth patterns\n", len(authPatterns))

	for _, pattern := range authPatterns {
		fmt.Printf("- %s: %s\n", pattern.ExpectedCode, pattern.Name)
	}

	// Get all not found patterns
	notFoundPatterns := PatternsForCategory(CategoryNotFound)
	fmt.Printf("Found %d not found patterns\n", len(notFoundPatterns))
}

// ExampleAllCommonPatterns demonstrates using AllCommonPatterns.
func ExampleAllCommonPatterns() {
	// Get all common patterns at once
	allPatterns := AllCommonPatterns()
	fmt.Printf("Total common patterns: %d\n", len(allPatterns))

	for _, pattern := range allPatterns {
		fmt.Printf("- %s (%d): %s\n",
			pattern.ExpectedCode,
			pattern.ExpectedStatus,
			pattern.Name)
	}
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
