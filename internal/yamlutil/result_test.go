// Package yamlutil tests for Result[T, E] type
package yamlutil

import (
	"errors"
	"fmt"
	"testing"
)

// TestResult_Ok creates a successful Result and verifies its state
func TestResult_Ok(t *testing.T) {
	result := Ok[int, *ParseError](42)

	if !result.IsOk() {
		t.Error("Ok result should have IsOk() == true")
	}
	if result.IsErr() {
		t.Error("Ok result should have IsErr() == false")
	}
	if result.Unwrap() != 42 {
		t.Errorf("Unwrap() = %d, want 42", result.Unwrap())
	}
}

// TestResult_Err creates an error Result and verifies its state
func TestResult_Err(t *testing.T) {
	err := &ParseError{Message: "test error"}
	result := Err[int, *ParseError](err)

	if result.IsOk() {
		t.Error("Err result should have IsOk() == false")
	}
	if !result.IsErr() {
		t.Error("Err result should have IsErr() == true")
	}
	if result.UnwrapErr().Message != "test error" {
		t.Errorf("UnwrapErr().Message = %q, want %q", result.UnwrapErr().Message, "test error")
	}
}

// TestResult_Unwrap_panics_on_Err
func TestResult_Unwrap_panics_on_Err(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when calling Unwrap() on Err result")
		}
	}()

	result := Err[int, *ParseError](&ParseError{Message: "error"})
	result.Unwrap()
}

// TestResult_UnwrapErr_panics_on_Ok
func TestResult_UnwrapErr_panics_on_Ok(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when calling UnwrapErr() on Ok result")
		}
	}()

	result := Ok[int, *ParseError](42)
	result.UnwrapErr()
}

// TestResult_UnwrapOrDefault returns zero value for Err
func TestResult_UnwrapOrDefault(t *testing.T) {
	// Ok case
	okResult := Ok[int, *ParseError](42)
	if got := okResult.UnwrapOrDefault(); got != 42 {
		t.Errorf("UnwrapOrDefault() on Ok = %d, want 42", got)
	}

	// Err case
	errResult := Err[int, *ParseError](&ParseError{Message: "error"})
	if got := errResult.UnwrapOrDefault(); got != 0 {
		t.Errorf("UnwrapOrDefault() on Err = %d, want 0", got)
	}
}

// TestResult_UnwrapOr returns default for Err
func TestResult_UnwrapOr(t *testing.T) {
	// Ok case
	okResult := Ok[int, *ParseError](42)
	if got := okResult.UnwrapOr(99); got != 42 {
		t.Errorf("UnwrapOr() on Ok = %d, want 42", got)
	}

	// Err case
	errResult := Err[int, *ParseError](&ParseError{Message: "error"})
	if got := errResult.UnwrapOr(99); got != 99 {
		t.Errorf("UnwrapOr() on Err = %d, want 99", got)
	}
}

// TestResult_UnwrapOrElse computes default for Err
func TestResult_UnwrapOrElse(t *testing.T) {
	// Ok case
	okResult := Ok[int, *ParseError](42)
	if got := okResult.UnwrapOrElse(func() int { return 99 }); got != 42 {
		t.Errorf("UnwrapOrElse() on Ok = %d, want 42", got)
	}

	// Err case
	called := false
	errResult := Err[int, *ParseError](&ParseError{Message: "error"})
	if got := errResult.UnwrapOrElse(func() int { called = true; return 99 }); got != 99 {
		t.Errorf("UnwrapOrElse() on Err = %d, want 99", got)
	}
	if !called {
		t.Error("UnwrapOrElse() function was not called for Err result")
	}
}

// TestResult_Map transforms Ok values
func TestResult_Map(t *testing.T) {
	// Map on Ok
	result := Ok[int, *ParseError](21)
	mapped := result.Map(func(n int) int { return n * 2 })
	if !mapped.IsOk() {
		t.Error("Map() on Ok should return Ok")
	}
	if mapped.Unwrap() != 42 {
		t.Errorf("Map() result = %d, want 42", mapped.Unwrap())
	}

	// Map on Err
	errResult := Err[int, *ParseError](&ParseError{Message: "error"})
	mappedErr := errResult.Map(func(n int) int { return n * 2 })
	if mappedErr.IsOk() {
		t.Error("Map() on Err should return Err")
	}
}

// TestResult_MapErr transforms errors
func TestResult_MapErr(t *testing.T) {
	// MapErr on Ok
	result := Ok[int, *ParseError](42)
	mapped := result.MapErr(func(e *ParseError) *ParseError {
		e.Message = "modified: " + e.Message
		return e
	})
	if !mapped.IsOk() {
		t.Error("MapErr() on Ok should return Ok")
	}
	if mapped.Unwrap() != 42 {
		t.Errorf("MapErr() on Ok should preserve value, got %d", mapped.Unwrap())
	}

	// MapErr on Err
	errResult := Err[int, *ParseError](&ParseError{Message: "error"})
	mappedErr := errResult.MapErr(func(e *ParseError) *ParseError {
		e.Message = "modified: " + e.Message
		return e
	})
	if mappedErr.IsOk() {
		t.Error("MapErr() on Err should return Err")
	}
	if got := mappedErr.UnwrapErr().Message; got != "modified: error" {
		t.Errorf("MapErr() error message = %q, want %q", got, "modified: error")
	}
}

// TestResult_AndThen chains operations
func TestResult_AndThen(t *testing.T) {
	// AndThen on Ok
	result := Ok[int, *ParseError](21)
	chained := result.AndThen(func(n int) Result[int, *ParseError] {
		if n > 0 {
			return Ok[int, *ParseError](n * 2)
		}
		return Err[int, *ParseError](&ParseError{Message: "non-positive"})
	})
	if !chained.IsOk() {
		t.Error("AndThen() should return Ok")
	}
	if chained.Unwrap() != 42 {
		t.Errorf("AndThen() result = %d, want 42", chained.Unwrap())
	}

	// AndThen on Err
	errResult := Err[int, *ParseError](&ParseError{Message: "initial error"})
	chainedErr := errResult.AndThen(func(n int) Result[int, *ParseError] {
		return Ok[int, *ParseError](999)
	})
	if chainedErr.IsOk() {
		t.Error("AndThen() on Err should return Err")
	}
	if got := chainedErr.UnwrapErr().Message; got != "initial error" {
		t.Errorf("AndThen() on Err should preserve error, got %q", got)
	}
}

// TestResult_OrElse provides fallback
func TestResult_OrElse(t *testing.T) {
	// OrElse on Ok
	result := Ok[int, *ParseError](42)
	fallback := result.OrElse(func(e *ParseError) Result[int, *ParseError] {
		return Ok[int, *ParseError](99)
	})
	if !fallback.IsOk() {
		t.Error("OrElse() on Ok should return Ok")
	}
	if fallback.Unwrap() != 42 {
		t.Errorf("OrElse() on Ok should preserve value, got %d", fallback.Unwrap())
	}

	// OrElse on Err
	errResult := Err[int, *ParseError](&ParseError{Message: "error"})
	fallbackResult := errResult.OrElse(func(e *ParseError) Result[int, *ParseError] {
		return Ok[int, *ParseError](99)
	})
	if !fallbackResult.IsOk() {
		t.Error("OrElse() on Err should return Ok from fallback")
	}
	if fallbackResult.Unwrap() != 99 {
		t.Errorf("OrElse() fallback = %d, want 99", fallbackResult.Unwrap())
	}
}

// TestResult_Match calls appropriate function
func TestResult_Match(t *testing.T) {
	// Match on Ok
	okCalled := false
	errCalled := false
	result := Ok[int, *ParseError](42)
	result.Match(
		func(v int) { okCalled = true },
		func(e *ParseError) { errCalled = true },
	)
	if !okCalled {
		t.Error("Match() on Ok should call onOk")
	}
	if errCalled {
		t.Error("Match() on Ok should not call onErr")
	}

	// Match on Err
	okCalled = false
	errCalled = false
	errResult := Err[int, *ParseError](&ParseError{Message: "error"})
	errResult.Match(
		func(v int) { okCalled = true },
		func(e *ParseError) { errCalled = true },
	)
	if okCalled {
		t.Error("Match() on Err should not call onOk")
	}
	if !errCalled {
		t.Error("Match() on Err should call onErr")
	}
}

// TestResult_String provides readable representation
func TestResult_String(t *testing.T) {
	okResult := Ok[int, *ParseError](42)
	if got := okResult.String(); got == "" {
		t.Error("String() on Ok returned empty string")
	}

	errResult := Err[int, *ParseError](&ParseError{Message: "error"})
	if got := errResult.String(); got == "" {
		t.Error("String() on Err returned empty string")
	}
}

// TestCollectResults collects successful results
func TestCollectResults(t *testing.T) {
	// All Ok
	results := []Result[int, *ParseError]{
		Ok[int, *ParseError](1),
		Ok[int, *ParseError](2),
		Ok[int, *ParseError](3),
	}
	collected := CollectResults(results)
	if !collected.IsOk() {
		t.Error("CollectResults() should return Ok when all are Ok")
	}
	values := collected.Unwrap()
	if len(values) != 3 {
		t.Errorf("CollectResults() returned %d values, want 3", len(values))
	}

	// With Err
	resultsWithErr := []Result[int, *ParseError]{
		Ok[int, *ParseError](1),
		Err[int, *ParseError](&ParseError{Message: "error"}),
		Ok[int, *ParseError](3),
	}
	collectedErr := CollectResults(resultsWithErr)
	if collectedErr.IsOk() {
		t.Error("CollectResults() should return Err when any is Err")
	}
}

// TestPartitionResults separates ok and err results
func TestPartitionResults(t *testing.T) {
	results := []Result[int, *ParseError]{
		Ok[int, *ParseError](1),
		Err[int, *ParseError](&ParseError{Message: "error1"}),
		Ok[int, *ParseError](2),
		Err[int, *ParseError](&ParseError{Message: "error2"}),
	}

	oks, errs := PartitionResults(results)

	if len(oks) != 2 {
		t.Errorf("PartitionResults() returned %d Ok values, want 2", len(oks))
	}
	if len(errs) != 2 {
		t.Errorf("PartitionResults() returned %d errors, want 2", len(errs))
	}
}

// TestOption_Some creates an Option with value
func TestOption_Some(t *testing.T) {
	opt := Some(42)

	if !opt.IsSome() {
		t.Error("Some should have IsSome() == true")
	}
	if opt.IsNone() {
		t.Error("Some should have IsNone() == false")
	}
	if opt.Unwrap() != 42 {
		t.Errorf("Unwrap() = %d, want 42", opt.Unwrap())
	}
}

// TestOption_None creates an empty Option
func TestOption_None(t *testing.T) {
	opt := None[int]()

	if opt.IsSome() {
		t.Error("None should have IsSome() == false")
	}
	if !opt.IsNone() {
		t.Error("None should have IsNone() == true")
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when calling Unwrap() on None")
		}
	}()
	opt.Unwrap()
}

// TestOption_UnwrapOr returns default for None
func TestOption_UnwrapOr(t *testing.T) {
	// Some case
	someOpt := Some(42)
	if got := someOpt.UnwrapOr(99); got != 42 {
		t.Errorf("UnwrapOr() on Some = %d, want 42", got)
	}

	// None case
	noneOpt := None[int]()
	if got := noneOpt.UnwrapOr(99); got != 99 {
		t.Errorf("UnwrapOr() on None = %d, want 99", got)
	}
}

// TestResult_ToOption converts Result to Option
func TestResult_ToOption(t *testing.T) {
	// Ok to Some
	result := Ok[int, *ParseError](42)
	opt := result.ToOption()
	if !opt.IsSome() {
		t.Error("ToOption() on Ok should return Some")
	}
	if opt.Unwrap() != 42 {
		t.Errorf("ToOption() value = %d, want 42", opt.Unwrap())
	}

	// Err to None
	errResult := Err[int, *ParseError](&ParseError{Message: "error"})
	noneOpt := errResult.ToOption()
	if !noneOpt.IsNone() {
		t.Error("ToOption() on Err should return None")
	}
}

// TestAsParseError converts errors to ParseError
func TestAsParseError(t *testing.T) {
	// Already ParseError
	pe := &ParseError{Message: "test"}
	if got := AsParseError(pe); got != pe {
		t.Error("AsParseError() should return same ParseError")
	}

	// SyntaxError
	se := &SyntaxError{
		FilePath: "test.yaml",
		Line:     5,
		Column:   10,
		Message:  "syntax error",
	}
	converted := AsParseError(se)
	if converted == nil {
		t.Fatal("AsParseError() returned nil for SyntaxError")
	}
	if converted.Line != 5 {
		t.Errorf("AsParseError() Line = %d, want 5", converted.Line)
	}
	if converted.Column != 10 {
		t.Errorf("AsParseError() Column = %d, want 10", converted.Column)
	}

	// Generic error
	genericErr := errors.New("generic error")
	convertedGeneric := AsParseError(genericErr)
	if convertedGeneric == nil {
		t.Fatal("AsParseError() returned nil for generic error")
	}
	if convertedGeneric.Message != "generic error" {
		t.Errorf("AsParseError() Message = %q, want %q", convertedGeneric.Message, "generic error")
	}

	// Nil error
	if got := AsParseError(nil); got != nil {
		t.Error("AsParseError() should return nil for nil error")
	}
}

// TestFromError converts standard error to Result
func TestFromError(t *testing.T) {
	// Nil error
	result := FromError(42, nil)
	if !result.IsOk() {
		t.Error("FromError() with nil error should return Ok")
	}
	if result.Unwrap() != 42 {
		t.Errorf("FromError() with nil error value = %d, want 42", result.Unwrap())
	}

	// Non-nil error
	stdErr := errors.New("standard error")
	errResult := FromError(0, stdErr)
	if errResult.IsOk() {
		t.Error("FromError() with error should return Err")
	}
	convertedErr := errResult.UnwrapErr()
	if convertedErr.Message != "standard error" {
		t.Errorf("FromError() error message = %q, want %q", convertedErr.Message, "standard error")
	}
}

// TestWithLineNumber adds line number to ParseError
func TestWithLineNumber(t *testing.T) {
	// Ok result
	okResult := Ok[int, *ParseError](42)
	withLine := WithLineNumber(okResult, 10)
	if !withLine.IsOk() {
		t.Error("WithLineNumber() on Ok should return Ok")
	}

	// Err result
	errResult := Err[int, *ParseError](&ParseError{Message: "error"})
	withLineErr := WithLineNumber(errResult, 10)
	if withLineErr.IsOk() {
		t.Error("WithLineNumber() on Err should return Err")
	}
	if got := withLineErr.UnwrapErr().Line; got != 10 {
		t.Errorf("WithLineNumber() Line = %d, want 10", got)
	}
}

// TestWithContext adds context to ParseError
func TestWithContext(t *testing.T) {
	// Ok result
	okResult := Ok[int, *ParseError](42)
	withCtx := WithContext(okResult, "context")
	if !withCtx.IsOk() {
		t.Error("WithContext() on Ok should return Ok")
	}

	// Err result without existing context
	errResult := Err[int, *ParseError](&ParseError{Message: "error"})
	withCtxErr := WithContext(errResult, "while parsing")
	if withCtxErr.IsOk() {
		t.Error("WithContext() on Err should return Err")
	}
	if got := withCtxErr.UnwrapErr().ContextStr; got != "while parsing" {
		t.Errorf("WithContext() ContextStr = %q, want %q", got, "while parsing")
	}

	// Err result with existing context
	errWithContext := Err[int, *ParseError](&ParseError{Message: "error", ContextStr: "initial"})
	withMoreCtx := WithContext(errWithContext, "while parsing")
	if got := withMoreCtx.UnwrapErr().ContextStr; got != "while parsing: initial" {
		t.Errorf("WithContext() with existing context = %q, want %q", got, "while parsing: initial")
	}
}

// TestResult_Error returns error for Err, nil for Ok
func TestResult_Error(t *testing.T) {
	// Ok result
	okResult := Ok[int, *ParseError](42)
	if got := okResult.Error(); got != nil {
		t.Error("Error() on Ok should return nil")
	}

	// Err result with ParseError
	errResult := Err[int, *ParseError](&ParseError{Message: "error"})
	if got := errResult.Error(); got == nil {
		t.Error("Error() on Err should return error")
	}
}

// Example: Using Result for parsing operations
func ExampleResult() {
	// Simulate parsing that can fail
	parseNumber := func(s string) Result[int, *ParseError] {
		if s == "" {
			return Err[int, *ParseError](&ParseError{Message: "empty string"})
		}
		var n int
		_, err := fmt.Sscanf(s, "%d", &n)
		if err != nil {
			return Err[int, *ParseError](&ParseError{
				Message: fmt.Sprintf("invalid number: %s", s),
				Err:     err,
			})
		}
		return Ok[int, *ParseError](n)
	}

	// Using the result
	result := parseNumber("42")
	result.Match(
		func(n int) { fmt.Println("Parsed:", n) },
		func(err *ParseError) { fmt.Println("Error:", err.Message) },
	)
	// Output: Parsed: 42
}

// Example: Chaining operations with AndThen
func ExampleResult_AndThen() {
	// Chain multiple operations
	divide := func(a, b int) Result[int, *ParseError] {
		if b == 0 {
			return Err[int, *ParseError](&ParseError{Message: "division by zero"})
		}
		return Ok[int, *ParseError](a / b)
	}

	result := Ok[int, *ParseError](100).AndThen(func(n int) Result[int, *ParseError] {
		return divide(n, 2)
	}).AndThen(func(n int) Result[int, *ParseError] {
		return Ok[int, *ParseError](n + 10)
	})

	fmt.Println(result.Unwrap())
	// Output: 60
}

// Example: Using OrElse for fallback values
func ExampleResult_OrElse() {
	getValue := func(key string) Result[int, *ParseError] {
		if key == "valid" {
			return Ok[int, *ParseError](42)
		}
		return Err[int, *ParseError](&ParseError{Message: "key not found"})
	}

	result := getValue("invalid").OrElse(func(err *ParseError) Result[int, *ParseError] {
		return Ok[int, *ParseError](0) // Default value
	})

	fmt.Println(result.Unwrap())
	// Output: 0
}
