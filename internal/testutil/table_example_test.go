// Package testutil provides example tests for the generic test table framework.
package testutil

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// =============================================================================
// EXAMPLE ERROR TYPES
// =============================================================================
// These are example error types used in the test examples.
// =============================================================================

// Example custom error types for demonstration
var (
	ErrEmptyInput    = errors.New("input cannot be empty")
	ErrInvalidFormat = errors.New("invalid format")
	ErrOutOfRange    = errors.New("value out of range")
	ErrNotFound      = errors.New("not found")
)

// ValidationInput is an example input type for validation tests.
type ValidationInput struct {
	Value string
}

// ProcessInput is an example function that processes input and returns an error.
// This simulates a typical validation/function that might be tested.
func ProcessInput(input ValidationInput) error {
	if input.Value == "" {
		return ErrEmptyInput
	}
	if strings.Contains(input.Value, "invalid") {
		return ErrInvalidFormat
	}
	if len(input.Value) > 10 {
		return ErrOutOfRange
	}
	return nil
}

// ComplexInput is an example of a more complex input type.
type ComplexInput struct {
	Name   string
	Count  int
	Active bool
}

// ProcessComplexInput is an example function that processes complex input.
func ProcessComplexInput(input ComplexInput) error {
	if input.Name == "" {
		return ErrEmptyInput
	}
	if input.Count < 0 {
		return ErrOutOfRange
	}
	if !input.Active && input.Count > 5 {
		return ErrInvalidFormat
	}
	return nil
}

// =============================================================================
// EXAMPLE TEST TABLES
// =============================================================================
// These examples demonstrate how to create and use test tables.
// =============================================================================

// Example 1: Basic Test Table with Simple Validation
// This example shows the most basic usage of the test table framework.

// TestBasicValidation demonstrates basic test table usage.
func TestBasicValidation(t *testing.T) {
	// Define the test table
	table := []TableTestCase[ValidationInput, error]{
		{
			Name:        "empty input returns error",
			Description: "Tests that empty input returns ErrEmptyInput",
			Input:       ValidationInput{Value: ""},
			ExpectedError: ErrEmptyInput,
			ExpectError:  true,
		},
		{
			Name:        "valid input succeeds",
			Description: "Tests that valid input returns no error",
			Input:       ValidationInput{Value: "valid"},
			ExpectedError: nil,
			ExpectError:  false,
		},
		{
			Name:        "invalid format returns error",
			Description: "Tests that input with 'invalid' substring returns ErrInvalidFormat",
			Input:       ValidationInput{Value: "this is invalid"},
			ExpectedError: ErrInvalidFormat,
			ExpectError:  true,
		},
	}

	// Run the test table
	RunTable(t, table, func(input ValidationInput) error {
		return ProcessInput(input)
	})
}

// Example 2: Test Table with Helper Functions
// This example shows how to use the helper functions for common test patterns.

// TestValidationWithHelpers demonstrates using helper functions to create test cases.
func TestValidationWithHelpers(t *testing.T) {
	// Define test cases using helper functions
	table := []TableTestCase[ValidationInput, error]{
		SuccessCase("valid input", ValidationInput{Value: "test"}),
		SuccessCase("another valid input", ValidationInput{Value: "ok"}),
		ErrorCase("empty input", ValidationInput{Value: ""}, ErrEmptyInput),
		ErrorCase("invalid format", ValidationInput{Value: "invalid format"}, ErrInvalidFormat),
		ErrorCaseWithMessage("out of range", ValidationInput{Value: "this is too long"}, ErrOutOfRange, "out of range"),
	}

	RunTable(t, table, func(input ValidationInput) error {
		return ProcessInput(input)
	})
}

// Example 3: Test Table with Complex Input
// This example shows how to use test tables with more complex input types.

// TestComplexInputValidation demonstrates test tables with complex input types.
func TestComplexInputValidation(t *testing.T) {
	table := []TableTestCase[ComplexInput, error]{
		{
			Name:        "valid complex input",
			Description: "Tests that valid complex input succeeds",
			Input:       ComplexInput{Name: "test", Count: 5, Active: true},
			ExpectedError: nil,
			ExpectError:  false,
		},
		{
			Name:        "empty name returns error",
			Description: "Tests that empty name returns ErrEmptyInput",
			Input:       ComplexInput{Name: "", Count: 5, Active: true},
			ExpectedError: ErrEmptyInput,
			ExpectError:  true,
		},
		{
			Name:        "negative count returns error",
			Description: "Tests that negative count returns ErrOutOfRange",
			Input:       ComplexInput{Name: "test", Count: -1, Active: true},
			ExpectedError: ErrOutOfRange,
			ExpectError:  true,
		},
		{
			Name:        "inactive with high count returns error",
			Description: "Tests that inactive with count > 5 returns ErrInvalidFormat",
			Input:       ComplexInput{Name: "test", Count: 10, Active: false},
			ExpectedError: ErrInvalidFormat,
			ExpectError:  true,
		},
	}

	RunTable(t, table, func(input ComplexInput) error {
		return ProcessComplexInput(input)
	})
}

// Example 4: Using Table Builder
// This example shows how to use the fluent TableBuilder interface.

// TestWithTableBuilder demonstrates using the TableBuilder for programmatic table creation.
func TestWithTableBuilder(t *testing.T) {
	// Build the test table using the fluent builder
	table := NewTableBuilder[ValidationInput, error]().
		WithName("validation tests").
		WithDescription("Test cases for input validation").
		WithTags([]string{"validation", "input"}).
		WithTestCase(TableTestCase[ValidationInput, error]{
			Name:        "empty input",
			Input:       ValidationInput{Value: ""},
			ExpectedError: ErrEmptyInput,
			ExpectError:  true,
		}).
		WithTestCase(TableTestCase[ValidationInput, error]{
			Name:        "valid input",
			Input:       ValidationInput{Value: "valid"},
			ExpectedError: nil,
			ExpectError:  false,
		}).
		WithTestCases([]TableTestCase[ValidationInput, error]{
			{
				Name:        "invalid format",
				Input:       ValidationInput{Value: "invalid"},
				ExpectedError: ErrInvalidFormat,
				ExpectError:  true,
			},
		}).
		Build()

	RunTable(t, table.TestCases, func(input ValidationInput) error {
		return ProcessInput(input)
	})
}

// Example 5: Table Extension and Filtering
// This example shows how to extend and filter test tables.

// TestTableExtension demonstrates extending and filtering test tables.
func TestTableExtension(t *testing.T) {
	// Define base test cases
	baseCases := []TableTestCase[ValidationInput, error]{
		SuccessCase("valid input 1", ValidationInput{Value: "test1"}),
		SuccessCase("valid input 2", ValidationInput{Value: "test2"}),
		ErrorCase("empty input", ValidationInput{Value: ""}, ErrEmptyInput),
	}

	// Add custom cases
	customCases := []TableTestCase[ValidationInput, error]{
		{
			Name:        "custom invalid format",
			Input:       ValidationInput{Value: "custom invalid"},
			ExpectedError: ErrInvalidFormat,
			ExpectError:  true,
			Tags:        []string{"custom", "format"},
		},
		{
			Name:        "custom out of range",
			Input:       ValidationInput{Value: "this is way too long"},
			ExpectedError: ErrOutOfRange,
			ExpectError:  true,
			Tags:        []string{"custom", "range"},
		},
	}

	// Extend the table
	extendedTable := ExtendTable(baseCases, customCases)

	// Run extended table
	t.Run("extended table", func(t *testing.T) {
		RunTable(t, extendedTable, func(input ValidationInput) error {
			return ProcessInput(input)
		})
	})

	// Filter by tag
	customFormatTests := FilterTable(extendedTable, "format")
	t.Run("filtered table (format)", func(t *testing.T) {
		if len(customFormatTests) != 1 {
			t.Errorf("expected 1 format test, got %d", len(customFormatTests))
		}
	})

	// Filter out custom tests
	withoutCustom := FilterTableByTag(extendedTable, "custom")
	t.Run("filtered table (without custom)", func(t *testing.T) {
		if len(withoutCustom) != len(baseCases) {
			t.Errorf("expected %d tests without custom, got %d", len(baseCases), len(withoutCustom))
		}
	})
}

// Example 6: Error Message Validation
// This example shows how to validate error messages.

// TestErrorMessageValidation demonstrates error message validation.
func TestErrorMessageValidation(t *testing.T) {
	table := []TableTestCase[ValidationInput, error]{
		{
			Name:        "empty input validates error message",
			Input:       ValidationInput{Value: ""},
			ExpectedError: ErrEmptyInput,
			ExpectError:  true,
			ErrorContains: "empty",
		},
		{
			Name:        "invalid format validates error message",
			Input:       ValidationInput{Value: "invalid"},
			ExpectedError: ErrInvalidFormat,
			ExpectError:  true,
			ErrorContains: "invalid format",
		},
		{
			Name:        "out of range validates error message",
			Input:       ValidationInput{Value: "this is too long input"},
			ExpectedError: ErrOutOfRange,
			ExpectError:  true,
			ErrorContains: "out of range",
		},
	}

	RunTable(t, table, func(input ValidationInput) error {
		return ProcessInput(input)
	})
}

// Example 7: Test Case Skipping
// This example shows how to skip test cases.

// TestWithSkippedCases demonstrates skipping test cases.
func TestWithSkippedCases(t *testing.T) {
	table := []TableTestCase[ValidationInput, error]{
		SuccessCase("valid input", ValidationInput{Value: "test"}),
		{
			Name:        "skipped test",
			Description: "This test is skipped",
			Input:       ValidationInput{Value: "test"},
			ExpectedError: nil,
			ExpectError:  false,
			Skip:        true,
			Tags:        []string{"skip"},
		},
		ErrorCase("error case", ValidationInput{Value: ""}, ErrEmptyInput),
	}

	RunTable(t, table, func(input ValidationInput) error {
		return ProcessInput(input)
	})
}

// Example 8: Using Assertion Helpers
// This example shows how to use assertion helpers.

// TestWithAssertions demonstrates using assertion helpers.
func TestWithAssertions(t *testing.T) {
	tests := []struct {
		name  string
		input ValidationInput
	}{
		{"valid input", ValidationInput{Value: "valid"}},
		{"empty input", ValidationInput{Value: ""}},
		{"invalid format", ValidationInput{Value: "invalid"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ProcessInput(tt.input)

			// Use assertion helpers
			switch tt.name {
			case "valid input":
				AssertNoError(t, err, "valid input should not return error")
			case "empty input":
				AssertError(t, err, "empty input should return error")
				AssertErrorIs(t, err, ErrEmptyInput, "should return ErrEmptyInput")
				AssertErrorContains(t, err, "empty", "error message should contain 'empty'")
			case "invalid format":
				AssertError(t, err, "invalid format should return error")
				AssertErrorIs(t, err, ErrInvalidFormat, "should return ErrInvalidFormat")
			}
		})
	}
}

// Example 9: Real-World String Validation
// This example shows a real-world use case for string validation.

// ValidateString is a real-world example function that validates strings.
func ValidateString(s string) error {
	if s == "" {
		return fmt.Errorf("string cannot be empty")
	}
	if len(s) < 3 {
		return fmt.Errorf("string must be at least 3 characters")
	}
	if len(s) > 50 {
		return fmt.Errorf("string must not exceed 50 characters")
	}
	if !strings.Contains(s, "@") && !strings.Contains(s, ".") {
		return fmt.Errorf("string must contain @ or .")
	}
	return nil
}

// TestStringValidation demonstrates a real-world validation example.
func TestStringValidation(t *testing.T) {
	table := []TableTestCase[string, error]{
		{
			Name:        "empty string",
			Description: "Tests that empty string returns error",
			Input:       "",
			ExpectError: true,
			ErrorContains: "empty",
		},
		{
			Name:        "too short",
			Description: "Tests that string < 3 characters returns error",
			Input:       "ab",
			ExpectError: true,
			ErrorContains: "at least 3",
		},
		{
			Name:        "too long",
			Description: "Tests that string > 50 characters returns error",
			Input:       strings.Repeat("a", 51),
			ExpectError: true,
			ErrorContains: "exceed 50",
		},
		{
			Name:        "missing special characters",
			Description: "Tests that string without @ or . returns error",
			Input:       "justplaintext",
			ExpectError: true,
			ErrorContains: "@ or .",
		},
		{
			Name:        "valid email-like string",
			Description: "Tests that valid email-like string succeeds",
			Input:       "user@example.com",
			ExpectError: false,
		},
		{
			Name:        "valid domain-like string",
			Description: "Tests that valid domain-like string succeeds",
			Input:       "example.com",
			ExpectError: false,
		},
	}

	RunTable(t, table, func(input string) error {
		return ValidateString(input)
	})
}

// Example 10: Numeric Range Validation
// This example shows numeric validation with range checking.

// ValidateNumber is an example function that validates numbers.
func ValidateNumber(n int) error {
	if n < 0 {
		return fmt.Errorf("number must be non-negative")
	}
	if n > 100 {
		return fmt.Errorf("number must not exceed 100")
	}
	return nil
}

// TestNumericValidation demonstrates numeric range validation.
func TestNumericValidation(t *testing.T) {
	table := []TableTestCase[int, error]{
		{
			Name:        "negative number",
			Description: "Tests that negative numbers return error",
			Input:       -1,
			ExpectError: true,
			ErrorContains: "non-negative",
		},
		{
			Name:        "zero is valid",
			Description: "Tests that zero is accepted",
			Input:       0,
			ExpectError: false,
		},
		{
			Name:        "valid number in range",
			Description: "Tests that valid number succeeds",
			Input:       50,
			ExpectError: false,
		},
		{
			Name:        "boundary value 100",
			Description: "Tests that 100 is accepted",
			Input:       100,
			ExpectError: false,
		},
		{
			Name:        "number exceeds maximum",
			Description: "Tests that numbers > 100 return error",
			Input:       101,
			ExpectError: true,
			ErrorContains: "exceed 100",
		},
	}

	RunTable(t, table, func(input int) error {
		return ValidateNumber(input)
	})
}
