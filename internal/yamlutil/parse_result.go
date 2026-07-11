// Package yamlutil provides Result<T, ParseError> pattern for type-safe error handling.
//
// This file implements the Result type that integrates with EnhancedParseError
// for compile-time guaranteed error handling.
package yamlutil

import (
	"fmt"
	"strings"
)

// ParseResultWithError[T] represents a parsing operation result that can be either
// a success with data or a failure with EnhancedParseError.
//
// This type enforces error handling at compile time - callers must check the error
// before accessing data, eliminating nil pointer dereference bugs.
//
// Type parameter T is the type of successfully parsed data.
//
// Example usage:
//
//	result := ParseYAMLToFile[Config](filePath)
//	if result.IsError() {
//	    log.Printf("Parse failed: %v", result.Error)
//	    return
//	}
//	config := result.Unwrap()
type ParseResultWithError[T any] struct {
	// success indicates whether the operation succeeded
	success bool

	// data contains the successful result (only valid when success is true)
	data T

	// err contains the parse error (only valid when success is false)
	err *EnhancedParseError
}

// OkParse creates a successful result with data.
func OkParse[T any](data T) ParseResultWithError[T] {
	return ParseResultWithError[T]{
		success: true,
		data:    data,
	}
}

// ErrParse creates a failed result with an error.
func ErrParse[T any](err *EnhancedParseError) ParseResultWithError[T] {
	return ParseResultWithError[T]{
		success: false,
		err:     err,
	}
}

// IsError returns true if this result represents a failure.
func (r ParseResultWithError[T]) IsError() bool {
	return !r.success
}

// IsOk returns true if this result represents success.
func (r ParseResultWithError[T]) IsOk() bool {
	return r.success
}

// Unwrap returns the successful data.
// Panics if the result is an error - always check IsOk() first.
func (r ParseResultWithError[T]) Unwrap() T {
	if r.IsError() {
		panic(fmt.Sprintf("called Unwrap on error result: %v", r.err))
	}
	return r.data
}

// UnwrapOr returns the successful data, or the provided default value if error.
func (r ParseResultWithError[T]) UnwrapOr(defaultValue T) T {
	if r.IsError() {
		return defaultValue
	}
	return r.data
}

// UnwrapOrElse returns the successful data, or calls the provided function to
// compute a default value if error.
func (r ParseResultWithError[T]) UnwrapOrElse(fn func(*EnhancedParseError) T) T {
	if r.IsError() {
		return fn(r.err)
	}
	return r.data
}

// Error returns the parse error if this result is an error, nil otherwise.
func (r ParseResultWithError[T]) Error() *EnhancedParseError {
	if r.IsError() {
		return r.err
	}
	return nil
}

// ErrorMsg returns the error message if this result is an error, empty string otherwise.
func (r ParseResultWithError[T]) ErrorMsg() string {
	if r.IsError() {
		return r.err.Error()
	}
	return ""
}

// Map applies a function to the successful value, transforming the result type.
// Returns the error unchanged if this result is an error.
func (r ParseResultWithError[T]) Map(fn func(T) T) ParseResultWithError[T] {
	if r.IsError() {
		return r
	}
	return OkParse(fn(r.data))
}

// MapErr applies a function to the error, transforming it while preserving success.
// Returns the success unchanged if this result is successful.
func (r ParseResultWithError[T]) MapErr(fn func(*EnhancedParseError) *EnhancedParseError) ParseResultWithError[T] {
	if r.IsError() {
		return ErrParse[T](fn(r.err))
	}
	return r
}

// AndThen chains another operation that returns a Result.
// If this result is an error, returns the error without calling the function.
// If this result is successful, calls the function with the data and returns its result.
func (r ParseResultWithError[T]) AndThen(fn func(T) ParseResultWithError[T]) ParseResultWithError[T] {
	if r.IsError() {
		return r
	}
	return fn(r.data)
}

// OrElse provides an alternative result if this result is an error.
// If this result is successful, returns it unchanged.
func (r ParseResultWithError[T]) OrElse(alternative ParseResultWithError[T]) ParseResultWithError[T] {
	if r.IsError() {
		return alternative
	}
	return r
}

// OrElseTry provides an alternative computation if this result is an error.
// If this result is successful, returns it unchanged.
func (r ParseResultWithError[T]) OrElseTry(fn func() ParseResultWithError[T]) ParseResultWithError[T] {
	if r.IsError() {
		return fn()
	}
	return r
}

// String returns a string representation of the result.
func (r ParseResultWithError[T]) String() string {
	if r.IsError() {
		return fmt.Sprintf("Err(%v)", r.err)
	}
	return fmt.Sprintf("Ok(%v)", r.data)
}

// ============================================================================
// Result type aliases for common use cases
// ============================================================================

// MapResult is a Result for parsing into map[string]interface{}.
type MapResult = ParseResultWithError[map[string]interface{}]

// ConfigResult is a generic Result for parsing into struct types.
type ConfigResult[T any] = ParseResultWithError[T]

// ============================================================================
// Error Checking Helper Functions
// ============================================================================

// IsParseSyntaxError checks if a Result contains a syntax error.
func IsParseSyntaxError[T any](r ParseResultWithError[T]) bool {
	return r.IsError() && r.err.IsSyntaxError()
}

// IsParseStructureError checks if a Result contains a structure error.
func IsParseStructureError[T any](r ParseResultWithError[T]) bool {
	return r.IsError() && r.err.IsStructureError()
}

// IsParseTypeMismatchError checks if a Result contains a type mismatch error.
func IsParseTypeMismatchError[T any](r ParseResultWithError[T]) bool {
	return r.IsError() && r.err.IsTypeMismatchError()
}

// IsParseIOError checks if a Result contains an I/O error.
func IsParseIOError[T any](r ParseResultWithError[T]) bool {
	return r.IsError() && r.err.IsIOError()
}

// IsParseValidationError checks if a Result contains a validation error.
func IsParseValidationError[T any](r ParseResultWithError[T]) bool {
	return r.IsError() && r.err.IsValidationError()
}

// IsParseSchemaError checks if a Result contains a schema error.
func IsParseSchemaError[T any](r ParseResultWithError[T]) bool {
	return r.IsError() && r.err.IsSchemaError()
}

// IsParseEmptyError checks if a Result contains an empty file error.
func IsParseEmptyError[T any](r ParseResultWithError[T]) bool {
	return r.IsError() && r.err.IsEmpty()
}

// ============================================================================
// Display and Formatting
// ============================================================================

// DetailedString returns a detailed string representation including error details.
func (r ParseResultWithError[T]) DetailedString() string {
	if r.IsOk() {
		return fmt.Sprintf("Ok(%v)", r.data)
	}
	return r.err.String()
}

// ErrorSummary returns a one-line summary of the error.
func (r ParseResultWithError[T]) ErrorSummary() string {
	if r.IsOk() {
		return "No error"
	}
	return r.err.Error()
}

// ErrorKind returns the kind of error if this is an error result.
func (r ParseResultWithError[T]) ErrorKind() ParseErrorKind {
	if r.IsError() {
		return r.err.Kind
	}
	return ""
}

// ErrorFilePath returns the file path from the error if this is an error result.
func (r ParseResultWithError[T]) ErrorFilePath() string {
	if r.IsError() {
		return r.err.FilePath
	}
	return ""
}

// ErrorLine returns the line number from the error if this is an error result.
func (r ParseResultWithError[T]) ErrorLine() int {
	if r.IsError() {
		return r.err.LocationInfo.Line
	}
	return 0
}

// ErrorColumn returns the column number from the error if this is an error result.
func (r ParseResultWithError[T]) ErrorColumn() int {
	if r.IsError() {
		return r.err.LocationInfo.Column
	}
	return 0
}

// ============================================================================
// Batch Operations
// ============================================================================

// ParseResults[T] represents a collection of parse results.
type ParseResults[T any] struct {
	// Results contains individual results
	Results []ParseResultWithError[T]

	// SuccessCount is the number of successful results
	SuccessCount int

	// ErrorCount is the number of error results
	ErrorCount int
}

// CollectParseResults collects a slice of Results into ParseResults with statistics.
func CollectParseResults[T any](results []ParseResultWithError[T]) ParseResults[T] {
	successCount := 0
	errorCount := 0

	for _, r := range results {
		if r.IsOk() {
			successCount++
		} else {
			errorCount++
		}
	}

	return ParseResults[T]{
		Results:      results,
		SuccessCount: successCount,
		ErrorCount:   errorCount,
	}
}

// FilterErrors returns only the error results.
func (pr ParseResults[T]) FilterErrors() []ParseResultWithError[T] {
	var errors []ParseResultWithError[T]
	for _, r := range pr.Results {
		if r.IsError() {
			errors = append(errors, r)
		}
	}
	return errors
}

// FilterSuccesses returns only the successful results.
func (pr ParseResults[T]) FilterSuccesses() []ParseResultWithError[T] {
	var successes []ParseResultWithError[T]
	for _, r := range pr.Results {
		if r.IsOk() {
			successes = append(successes, r)
		}
	}
	return successes
}

// ErrorSummary returns a summary of all errors in the results.
func (pr ParseResults[T]) ErrorSummary() string {
	if pr.ErrorCount == 0 {
		return "No errors"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Parse errors (%d):\n", pr.ErrorCount))

	for i, r := range pr.Results {
		if r.IsError() {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, r.ErrorMsg()))
		}
	}

	return sb.String()
}

// String returns a summary of the parse results.
func (pr ParseResults[T]) String() string {
	return fmt.Sprintf("ParseResults{Total: %d, Success: %d, Errors: %d}",
		len(pr.Results), pr.SuccessCount, pr.ErrorCount)
}
