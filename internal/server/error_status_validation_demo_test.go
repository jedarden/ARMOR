package server

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/validate"
)

// TestDemo_EnhancedErrorMessages demonstrates the enhanced error messages for status code validation.
//
// This test shows how the new error formatting helper provides comprehensive,
// actionable error messages that include:
// - Error type categorization
// - Expected vs actual status codes
// - HTTP context (method and endpoint)
// - Auto-generated suggestions for fixing the issue
func TestDemo_EnhancedErrorMessages(t *testing.T) {
	t.Run("Demonstrate enhanced error messages", func(t *testing.T) {
		fmt.Println("\n=== Enhanced Status Code Error Messages ===")

		// Scenario 1: Basic status code mismatch
		fmt.Println("1. Basic status code mismatch (expected 200, got 404):")
		err := ValidateStatusCodeInt(200, 404)
		if err != nil {
			fmt.Printf("Error: %v\n\n", err.Error())
		}

		// Scenario 2: Status code validation with HTTP context
		fmt.Println("2. Status code mismatch with HTTP context (GET /api/users/123):")
		ctx := HTTPContext{Method: "GET", Endpoint: "/api/users/123"}
		err = ValidateStatusCodeIntWithContext(200, 404, ctx)
		if err != nil {
			fmt.Printf("Error: %v\n\n", err.Error())
		}

		// Scenario 3: Multiple allowed status codes
		fmt.Println("3. Status code mismatch with multiple allowed codes ([200, 201, 204] vs 404):")
		err = ValidateStatusCodeIntAny([]int{200, 201, 204}, 404)
		if err != nil {
			fmt.Printf("Error: %v\n\n", err.Error())
		}

		// Scenario 4: Multiple allowed codes with HTTP context
		fmt.Println("4. Multiple allowed codes with HTTP context (POST /api/orders):")
		ctx = HTTPContext{Method: "POST", Endpoint: "/api/orders"}
		err = ValidateStatusCodeIntAnyWithContext([]int{200, 201, 204}, 409, ctx)
		if err != nil {
			fmt.Printf("Error: %v\n\n", err.Error())
		}

		fmt.Println("=== All enhanced error messages include ===")
		fmt.Println("✓ Error type (status_code)")
		fmt.Println("✓ Expected status code(s)")
		fmt.Println("✓ Actual received status code")
		fmt.Println("✓ HTTP context (method and endpoint)")
		fmt.Println("✓ Auto-generated suggestions")
	})

	t.Run("Verify enhanced error components", func(t *testing.T) {
		// Test that enhanced errors have all required components
		ctx := HTTPContext{Method: "GET", Endpoint: "/api/users/123"}
		err := ValidateStatusCodeIntWithContext(200, 404, ctx)

		if err == nil {
			t.Fatal("Expected error for non-matching codes")
		}

		errorMsg := err.Error()
		validationErr := err.(validate.ValidationError)

		// Verify all components are present
		requiredComponents := map[string]string{
			"Error type":           "status_code",
			"Expected code":        "200",
			"Actual code":          "404",
			"HTTP method":          "GET",
			"Endpoint":             "/api/users/123",
			"Expected description": "OK",
			"Actual description":   "Not Found",
		}

		for componentName, componentValue := range requiredComponents {
			if !strings.Contains(errorMsg, componentValue) {
				t.Errorf("Error message should contain %s '%s', got: %s",
					componentName, componentValue, errorMsg)
			}
		}

		// Verify ValidationError fields are populated
		if validationErr.ErrorType == "" {
			t.Error("ValidationError should have ErrorType set")
		}
		if validationErr.Expected == nil {
			t.Error("ValidationError should have Expected set")
		}
		if validationErr.Actual == nil {
			t.Error("ValidationError should have Actual set")
		}
		if validationErr.Context == "" {
			t.Error("ValidationError should have Context set")
		}
		if len(validationErr.Suggestions) == 0 {
			t.Error("ValidationError should have Suggestions")
		}

		// Verify suggestions are relevant
		suggestionsStr := fmt.Sprintf("%v", validationErr.Suggestions)
		relevantSuggestions := []string{
			"endpoint",
			"resource",
			"URL",
		}
		hasRelevantSuggestion := false
		for _, suggestion := range relevantSuggestions {
			if strings.Contains(strings.ToLower(suggestionsStr), suggestion) {
				hasRelevantSuggestion = true
				break
			}
		}
		if !hasRelevantSuggestion {
			t.Errorf("Suggestions should be relevant to status code errors, got: %v", validationErr.Suggestions)
		}
	})

	t.Run("Compare basic vs enhanced error messages", func(t *testing.T) {
		// Show the difference between basic and enhanced error messages

		fmt.Println("\n=== Error Message Comparison ===")

		// Basic error (without context)
		fmt.Println("Basic error (without HTTP context):")
		err := ValidateStatusCodeInt(200, 404)
		if err != nil {
			fmt.Printf("Error: %v\n\n", err.Error())
		}

		// Enhanced error (with context)
		fmt.Println("Enhanced error (with HTTP context):")
		ctx := HTTPContext{Method: "GET", Endpoint: "/api/users/123"}
		err = ValidateStatusCodeIntWithContext(200, 404, ctx)
		if err != nil {
			fmt.Printf("Error: %v\n\n", err.Error())
		}

		fmt.Println("The enhanced version includes:")
		fmt.Println("✓ HTTP method (GET)")
		fmt.Println("✓ Endpoint path (/api/users/123)")
		fmt.Println("✓ Contextual suggestions")
	})
}

// TestDemo_UseCases demonstrates real-world use cases for enhanced status code errors.
func TestDemo_UseCases(t *testing.T) {
	t.Run("API endpoint validation", func(t *testing.T) {
		// Simulating API endpoint validation failures
		scenarios := []struct {
			name        string
			method      string
			endpoint    string
			expected    int
			actual      int
			description string
		}{
			{
				"User not found",
				"GET",
				"/api/users/123",
				200,
				404,
				"Expected successful response but got 404 Not Found",
			},
			{
				"Unauthorized access",
				"POST",
				"/api/orders",
				201,
				401,
				"Expected created response but got 401 Unauthorized",
			},
			{
				"Server error",
				"DELETE",
				"/api/users/123",
				204,
				500,
				"Expected no content response but got 500 Internal Server Error",
			},
		}

		fmt.Println("\n=== Real-World API Validation Scenarios ===")

		for _, scenario := range scenarios {
			fmt.Printf("Scenario: %s\n", scenario.name)
			fmt.Printf("Description: %s\n", scenario.description)
			fmt.Printf("Request: %s %s\n", scenario.method, scenario.endpoint)

			ctx := HTTPContext{Method: scenario.method, Endpoint: scenario.endpoint}
			err := ValidateStatusCodeIntWithContext(scenario.expected, scenario.actual, ctx)

			if err != nil {
				fmt.Printf("Error: %v\n", err.Error())
			}

			fmt.Println()
		}
	})
}
