// Package testutil provides a generic test table framework for organizing test cases.
//
// # Generic Test Table Framework
//
// This package provides a comprehensive, extensible test table framework for
// writing table-driven tests in Go. Unlike HTTP-specific test frameworks, this
// framework is designed to work with any type of test case.
//
// # Quick Start
//
// Define a test table:
//
//	table := []TableTestCase[MyInput, MyError]{
//	    {
//	        Name:        "valid input succeeds",
//	        Description: "Tests that valid input produces expected result",
//	        Input:       MyInput{Field: "value"},
//	        ExpectedError: nil,
//	    },
//	    {
//	        Name:        "invalid input returns error",
//	        Description: "Tests that invalid input returns expected error",
//	        Input:       MyInput{Field: ""},
//	        ExpectedError: ErrInvalidInput,
//	    },
//	}
//
// Run the test table:
//
//	for _, tc := range table {
//	    t.Run(tc.Name, func(t *testing.T) {
//	        result, err := Process(tc.Input)
//	        if tc.ExpectError {
//	            if err == nil {
//	                t.Errorf("expected error, got nil")
//	            }
//	        } else {
//	            if err != nil {
//	                t.Errorf("unexpected error: %v", err)
//	            }
//	        }
//	    })
//	}
package testutil

import "testing"

// =============================================================================
// BASE TEST TABLE STRUCTURES
// =============================================================================
// These core types form the foundation of the generic test table framework.
// They are designed to be type-safe, extensible, and work with any error type.
//
// Design Philosophy:
// - Generic: Works with any input and error types
// - Type-safe: Strong typing prevents errors
// - Reusable: Structures can be used across test files
// - Self-documenting: Clear field names and comprehensive documentation
// =============================================================================

// TableTestCase represents a single generic test case.
//
// This is the core building block for generic test tables. Each test case contains
// all the information needed to execute a single test.
//
// Type Parameters:
//   - I: The input type for the test
//   - E: The error type (typically error, but can be any custom error type)
//
// Use this type when:
//   - Building generic test tables
//   - Creating test cases for any function/method
//   - Writing table-driven tests
//
// Example (basic usage):
//
//	tc := TableTestCase[string, error]{
//	    Name:        "empty string is invalid",
//	    Description: "Tests that empty string returns error",
//	    Input:       "",
//	    ExpectedError: ErrEmptyString,
//	    ExpectError: true,
//	}
//
// Example (with custom error type):
//
//	tc := TableTestCase[MyInput, MyError]{
//	    Name:        "validation fails",
//	    Description: "Tests that invalid input returns validation error",
//	    Input:       MyInput{Count: -1},
//	    ExpectedError: MyError{Code: ErrValidation},
//	    ExpectError: true,
//	}
//
// Fields:
//   - Name: Test case name (used in t.Run)
//   - Description: Optional detailed description of what is being tested
//   - Input: The input to test
//   - ExpectedError: The expected error (nil if no error expected)
//   - ExpectError: Whether an error is expected (convenience field)
//   - ErrorContains: Optional substring that should be in the error message
//   - ErrorIs: Optional error that should match using errors.Is
//   - Skip: Whether to skip this test case
//   - Tags: Optional tags for test filtering
type TableTestCase[I any, E error] struct {
	// Name is the test case name for identification and test reporting
	Name string

	// Description is an optional detailed explanation of what the test validates
	Description string

	// Input is the input to test
	Input I

	// ExpectedError is the expected error (nil if no error expected)
	ExpectedError E

	// ExpectError indicates whether an error is expected
	// This is a convenience field - if true, the test expects an error
	// If false, the test expects no error (ExpectedError should be nil)
	ExpectError bool

	// ErrorContains is an optional substring that should be in the error message
	// If set, the error message must contain this substring
	ErrorContains string

	// ErrorIs is an optional error that should match using errors.Is
	// If set, errors.Is(actualError, this error) must return true
	ErrorIs error

	// Skip optionally marks this test case to be skipped
	Skip bool

	// Tags optionally provides tags for test filtering
	Tags []string
}

// TableTestTable represents a collection of related generic test cases.
//
// This type groups test cases together for organized testing. Tables can be
// predefined, custom, or extended from existing tables.
//
// Type Parameters:
//   - I: The input type for the test cases
//   - E: The error type for the test cases
//
// Use this type when:
//   - Creating reusable test tables
//   - Organizing tests by category
//   - Sharing test tables across multiple test files
//
// Example (creating a table):
//
//	table := TableTestTable[string, error]{
//	    Name:    "string validation tests",
//	    Description: "Test cases for string validation",
//	    TestCases: []TableTestCase[string, error]{
//	        {Name: "empty string", Input: "", ExpectedError: ErrEmpty, ExpectError: true},
//	        {Name: "valid string", Input: "test", ExpectedError: nil, ExpectError: false},
//	    },
//	}
//
// Fields:
//   - Name: The name of the test table
//   - Description: Optional description of the table's purpose
//   - TestCases: The collection of test cases
//   - Tags: Optional tags for the entire table
type TableTestTable[I any, E error] struct {
	// Name is the name of the test table
	Name string

	// Description optionally describes the purpose of this test table
	Description string

	// TestCases is the collection of test cases
	TestCases []TableTestCase[I, E]

	// Tags optionally provides tags for the entire table
	Tags []string
}

// =============================================================================
// TEST TABLE BUILDER
// =============================================================================
// These structures and functions provide a fluent interface for building test tables.
// =============================================================================

// TableBuilder provides a fluent interface for building test tables.
//
// Type Parameters:
//   - I: The input type for the test cases
//   - E: The error type for the test cases
//
// Use this builder when:
//   - Creating test tables programmatically
//   - Building complex test tables step by step
//   - Adding test cases conditionally
//
// Example:
//
//	table := NewTableBuilder[string, error]().
//	    WithName("string tests").
//	    WithDescription("String validation tests").
//	    WithTestCase(TableTestCase[string, error]{
//	        Name: "empty string",
//	        Input: "",
//	        ExpectedError: ErrEmpty,
//	        ExpectError: true,
//	    }).
//	    WithTestCase(TableTestCase[string, error]{
//	        Name: "valid string",
//	        Input: "test",
//	        ExpectedError: nil,
//	        ExpectError: false,
//	    }).
//	    Build()
type TableBuilder[I any, E error] struct {
	table TableTestTable[I, E]
}

// NewTableBuilder creates a new TableBuilder.
//
// Example:
//
//	builder := NewTableBuilder[string, error]()
func NewTableBuilder[I any, E error]() *TableBuilder[I, E] {
	return &TableBuilder[I, E]{
		table: TableTestTable[I, E]{
			TestCases: make([]TableTestCase[I, E], 0),
		},
	}
}

// WithName sets the name of the test table.
//
// Example:
//
//	builder := NewTableBuilder[string, error]().
//	    WithName("my test table")
func (tb *TableBuilder[I, E]) WithName(name string) *TableBuilder[I, E] {
	tb.table.Name = name
	return tb
}

// WithDescription sets the description of the test table.
//
// Example:
//
//	builder := NewTableBuilder[string, error]().
//	    WithDescription("Test cases for string validation")
func (tb *TableBuilder[I, E]) WithDescription(desc string) *TableBuilder[I, E] {
	tb.table.Description = desc
	return tb
}

// WithTags sets the tags for the test table.
//
// Example:
//
//	builder := NewTableBuilder[string, error]().
//	    WithTags([]string{"validation", "string"})
func (tb *TableBuilder[I, E]) WithTags(tags []string) *TableBuilder[I, E] {
	tb.table.Tags = tags
	return tb
}

// WithTestCase adds a test case to the table.
//
// Example:
//
//	builder := NewTableBuilder[string, error]().
//	    WithTestCase(TableTestCase[string, error]{
//	        Name: "empty string",
//	        Input: "",
//	        ExpectedError: ErrEmpty,
//	        ExpectError: true,
//	    })
func (tb *TableBuilder[I, E]) WithTestCase(tc TableTestCase[I, E]) *TableBuilder[I, E] {
	tb.table.TestCases = append(tb.table.TestCases, tc)
	return tb
}

// WithTestCases adds multiple test cases to the table.
//
// Example:
//
//	cases := []TableTestCase[string, error]{...}
//	builder := NewTableBuilder[string, error]().
//	    WithTestCases(cases)
func (tb *TableBuilder[I, E]) WithTestCases(cases []TableTestCase[I, E]) *TableBuilder[I, E] {
	tb.table.TestCases = append(tb.table.TestCases, cases...)
	return tb
}

// Build constructs and returns the test table.
//
// Example:
//
//	table := NewTableBuilder[string, error]().
//	    WithName("string tests").
//	    WithTestCase(tc1).
//	    WithTestCase(tc2).
//	    Build()
func (tb *TableBuilder[I, E]) Build() TableTestTable[I, E] {
	return tb.table
}

// =============================================================================
// RUNNER FUNCTIONS
// =============================================================================
// These functions provide convenient ways to run test tables.
// =============================================================================

// RunTableFunc is the function signature for running a single test case.
//
// Type Parameters:
//   - I: The input type for the test case
//
// Parameters:
//   - input: The input to test
//
// Returns:
//   - error: The error from the test (if any)
//
// Example implementation:
//
//	func myValidator(input string) error {
//	    if input == "" {
//	        return ErrEmpty
//	    }
//	    return nil
//	}
//
//	var runner RunTableFunc[string] = myValidator
type RunTableFunc[I any] func(input I) error

// RunTable executes all test cases in a table using the provided runner function.
//
// This function iterates through all test cases in the table and executes them
// using the provided runner function. It handles error checking, skipping,
// and test reporting.
//
// Type Parameters:
//   - I: The input type for the test cases
//
// Parameters:
//   - t: The testing.T instance for test reporting
//   - table: The test table to run
//   - runner: The function to execute for each test case
//
// Example:
//
//	func TestStringValidation(t *testing.T) {
//	    table := []TableTestCase[string, error]{
//	        {
//	            Name:        "empty string returns error",
//	            Input:       "",
//	            ExpectedError: ErrEmpty,
//	            ExpectError: true,
//	        },
//	        {
//	            Name:        "valid string succeeds",
//	            Input:       "test",
//	            ExpectedError: nil,
//	            ExpectError: false,
//	        },
//	    }
//
//	    RunTable(t, table, func(input string) error {
//	        return validateString(input)
//	    })
//	}
func RunTable[I any](t interface {
	Helper()
	Run(name string, f func(t *testing.T)) bool
	Skipf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}, table []TableTestCase[I, error], runner RunTableFunc[I]) {
	t.Helper()

	for _, tc := range table {
		t.Run(tc.Name, func(t *testing.T) {
			t.Helper()

			if tc.Skip {
				t.Skipf("test case skipped: %s", tc.Name)
				return
			}

			err := runner(tc.Input)

			// Check if error is expected
			if tc.ExpectError {
				if err == nil {
					t.Errorf("%s: expected error, got nil", tc.Name)
					return
				}

				// Check ErrorContains
				if tc.ErrorContains != "" {
					errMsg := err.Error()
					if !contains(errMsg, tc.ErrorContains) {
						t.Errorf("%s: error message does not contain expected substring\n"+
							"expected substring: %s\nactual error: %v", tc.Name, tc.ErrorContains, err)
						return
					}
				}

				// Check ErrorIs
				if tc.ErrorIs != nil {
					if !errorIs(err, tc.ErrorIs) {
						t.Errorf("%s: error does not match expected error\n"+
							"expected: %v\nactual: %v", tc.Name, tc.ErrorIs, err)
						return
					}
				}

				// Check ExpectedError
				if tc.ExpectedError != nil {
					if !errorIs(err, tc.ExpectedError) {
						t.Errorf("%s: error does not match expected error\n"+
							"expected: %v\nactual: %v", tc.Name, tc.ExpectedError, err)
					}
				}
			} else {
				if err != nil {
					t.Errorf("%s: unexpected error: %v", tc.Name, err)
					return
				}
			}
		})
	}
}

// contains is a helper to check if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(s) > len(substr) && findSubstring(s, substr))
}

// findSubstring is a simple substring search.
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// errorIs is a helper to check if an error matches another error.
func errorIs(err, target error) bool {
	if err == nil && target == nil {
		return true
	}
	if err == nil || target == nil {
		return false
	}
	return err.Error() == target.Error() || contains(err.Error(), target.Error())
}

// =============================================================================
// TABLE EXTENSION HELPERS
// =============================================================================
// These functions provide patterns for extending and manipulating test tables.
// =============================================================================

// ExtendTable adds custom test cases to an existing test table.
//
// This helper provides a clean pattern for extending predefined test tables
// with custom test cases without modifying the original table.
//
// Type Parameters:
//   - I: The input type for the test cases
//   - E: The error type for the test cases
//
// Parameters:
//   - base: The base test table to extend
//   - customCases: Custom test cases to add
//
// Returns a new test table containing both base and custom cases
//
// Example:
//
//	base := []TableTestCase[string, error]{...}
//	custom := []TableTestCase[string, error]{
//	    {Name: "custom test", Input: "custom", ExpectedError: nil, ExpectError: false},
//	}
//	extended := ExtendTable(base, custom)
func ExtendTable[I any, E error](base []TableTestCase[I, E], customCases []TableTestCase[I, E]) []TableTestCase[I, E] {
	extended := make([]TableTestCase[I, E], len(base)+len(customCases))
	copy(extended, base)
	copy(extended[len(base):], customCases)
	return extended
}

// FilterTable filters a test table by tag.
//
// This helper allows selective test execution based on tags.
// Useful for running specific subsets of tests.
//
// Type Parameters:
//   - I: The input type for the test cases
//   - E: The error type for the test cases
//
// Parameters:
//   - table: The test table to filter
//   - tag: The tag to filter by
//
// Returns a filtered test table
//
// Example:
//
//	allTests := []TableTestCase[string, error]{...}
//	skipTests := FilterTable(allTests, "skip")
func FilterTable[I any, E error](table []TableTestCase[I, E], tag string) []TableTestCase[I, E] {
	var filtered []TableTestCase[I, E]
	for _, tc := range table {
		for _, t := range tc.Tags {
			if t == tag {
				filtered = append(filtered, tc)
				break
			}
		}
	}
	return filtered
}

// FilterTableByTag filters a test table to exclude specific tags.
//
// This helper allows selective test execution by excluding tests with specific tags.
// Useful for excluding tests marked with specific tags (e.g., "skip", "slow").
//
// Type Parameters:
//   - I: The input type for the test cases
//   - E: The error type for the test cases
//
// Parameters:
//   - table: The test table to filter
//   - excludeTag: The tag to exclude
//
// Returns a filtered test table excluding tests with the specified tag
//
// Example:
//
//	allTests := []TableTestCase[string, error]{...}
//	withoutSkip := FilterTableByTag(allTests, "skip")
func FilterTableByTag[I any, E error](table []TableTestCase[I, E], excludeTag string) []TableTestCase[I, E] {
	var filtered []TableTestCase[I, E]
	for _, tc := range table {
		hasExcludeTag := false
		for _, t := range tc.Tags {
			if t == excludeTag {
				hasExcludeTag = true
				break
			}
		}
		if !hasExcludeTag {
			filtered = append(filtered, tc)
		}
	}
	return filtered
}

// =============================================================================
// COMMON TEST CASE PATTERNS
// =============================================================================
// These functions provide common test case patterns for quick test creation.
// =============================================================================

// SuccessCase creates a test case for successful execution.
//
// This helper creates a test case that expects no error.
//
// Type Parameters:
//   - I: The input type for the test case
//
// Parameters:
//   - name: The test case name
//   - input: The input to test
//
// Returns a test case configured for success
//
// Example:
//
//	case := SuccessCase("valid input", "test input")
func SuccessCase[I any](name string, input I) TableTestCase[I, error] {
	return TableTestCase[I, error]{
		Name:         name,
		Input:        input,
		ExpectedError: nil,
		ExpectError:  false,
	}
}

// ErrorCase creates a test case for error execution.
//
// This helper creates a test case that expects an error.
//
// Type Parameters:
//   - I: The input type for the test case
//
// Parameters:
//   - name: The test case name
//   - input: The input to test
//   - expectedErr: The expected error (optional, can be nil)
//
// Returns a test case configured for error
//
// Example:
//
//	case := ErrorCase("invalid input", "", ErrEmpty)
func ErrorCase[I any](name string, input I, expectedErr error) TableTestCase[I, error] {
	return TableTestCase[I, error]{
		Name:          name,
		Input:         input,
		ExpectedError: expectedErr,
		ExpectError:   true,
	}
}

// ErrorCaseWithMessage creates a test case for error execution with message validation.
//
// This helper creates a test case that expects an error with a specific message.
//
// Type Parameters:
//   - I: The input type for the test case
//
// Parameters:
//   - name: The test case name
//   - input: The input to test
//   - expectedErr: The expected error
//   - errorMsg: The substring that should be in the error message
//
// Returns a test case configured for error with message validation
//
// Example:
//
//	case := ErrorCaseWithMessage("invalid input", "", ErrEmpty, "input cannot be empty")
func ErrorCaseWithMessage[I any](name string, input I, expectedErr error, errorMsg string) TableTestCase[I, error] {
	return TableTestCase[I, error]{
		Name:          name,
		Input:         input,
		ExpectedError: expectedErr,
		ExpectError:   true,
		ErrorContains: errorMsg,
	}
}

// =============================================================================
// TEST EXECUTION HELPERS
// =============================================================================
// These functions provide additional helpers for test execution and validation.
// =============================================================================

// AssertNoError asserts that no error occurred.
//
// This helper fails the test if an error is present.
//
// Parameters:
//   - t: The testing.T instance
//   - err: The error to check
//   - msg: Optional message prefix
//
// Example:
//
//	AssertNoError(t, err, "validation failed")
func AssertNoError(t *testing.T, err error, msg ...string) {
	t.Helper()
	if err != nil {
		prefix := ""
		if len(msg) > 0 {
			prefix = msg[0] + ": "
		}
		t.Errorf("%sunexpected error: %v", prefix, err)
	}
}

// AssertError asserts that an error occurred.
//
// This helper fails the test if no error is present.
//
// Parameters:
//   - t: The testing.T instance
//   - err: The error to check
//   - msg: Optional message prefix
//
// Example:
//
//	AssertError(t, err, "expected validation to fail")
func AssertError(t *testing.T, err error, msg ...string) {
	t.Helper()
	if err == nil {
		prefix := ""
		if len(msg) > 0 {
			prefix = msg[0] + ": "
		}
		t.Errorf("%sexpected error, got nil", prefix)
	}
}

// AssertErrorIs asserts that the error matches the expected error.
//
// This helper fails the test if the error does not match the expected error.
//
// Parameters:
//   - t: The testing.T instance
//   - err: The error to check
//   - expected: The expected error
//   - msg: Optional message prefix
//
// Example:
//
//	AssertErrorIs(t, err, ErrInvalidInput, "error type mismatch")
func AssertErrorIs(t *testing.T, err error, expected error, msg ...string) {
	t.Helper()
	if err == nil {
		prefix := ""
		if len(msg) > 0 {
			prefix = msg[0] + ": "
		}
		t.Errorf("%sexpected error %v, got nil", prefix, expected)
		return
	}
	if !errorIs(err, expected) {
		prefix := ""
		if len(msg) > 0 {
			prefix = msg[0] + ": "
		}
		t.Errorf("%sexpected error %v, got %v", prefix, expected, err)
	}
}

// AssertErrorContains asserts that the error message contains a substring.
//
// This helper fails the test if the error message does not contain the substring.
//
// Parameters:
//   - t: The testing.T instance
//   - err: The error to check
//   - substring: The substring to look for
//   - msg: Optional message prefix
//
// Example:
//
//	AssertErrorContains(t, err, "invalid input", "error message mismatch")
func AssertErrorContains(t *testing.T, err error, substring string, msg ...string) {
	t.Helper()
	if err == nil {
		prefix := ""
		if len(msg) > 0 {
			prefix = msg[0] + ": "
		}
		t.Errorf("%serror message should contain %q, got nil", prefix, substring)
		return
	}
	if !contains(err.Error(), substring) {
		prefix := ""
		if len(msg) > 0 {
			prefix = msg[0] + ": "
		}
		t.Errorf("%serror message should contain %q, got %q", prefix, substring, err.Error())
	}
}
