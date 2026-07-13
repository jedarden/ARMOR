# TestResult Test Scope and Cases Documentation

## Overview

This document catalogs all TestResult tests in the `internal/yamlutil` package, covering both the generic `Result[T, E]` type (from `result_test.go`) and the specialized `ParseResultWithError[T]` type (from `parse_result_test.go`).

---

## Tests in `result_test.go` (Result[T, E] Type)

### Basic Construction Tests

| Test Function | Purpose | Test Setup |
|---------------|---------|------------|
| **TestResult_Ok** | Verifies successful Result creation and state checks (IsOk, IsErr, Unwrap) | Creates `Ok[int, *ParseError](42)` |
| **TestResult_Err** | Verifies error Result creation and state checks (IsOk, IsErr, UnwrapErr) | Creates `Err[int, *ParseError]` with a ParseError |

### Unwrap Behavior Tests

| Test Function | Purpose | Test Setup |
|---------------|---------|------------|
| **TestResult_Unwrap_panics_on_Err** | Validates that calling Unwrap() on an Err result panics | Uses `defer/recover` to catch panic |
| **TestResult_UnwrapErr_panics_on_Ok** | Validates that calling UnwrapErr() on an Ok result panics | Uses `defer/recover` to catch panic |
| **TestResult_UnwrapOrDefault** | Tests UnwrapOrDefault returns zero value for Err, actual value for Ok | Tests both Ok and Err cases |
| **TestResult_UnwrapOr** | Tests UnwrapOr returns default value for Err, actual value for Ok | Tests both Ok and Err cases with custom default |
| **TestResult_UnwrapOrElse** | Tests UnwrapOrElse lazily computes default for Err, actual value for Ok | Verifies callback is only invoked for Err case |

### Transformation Tests

| Test Function | Purpose | Test Setup |
|---------------|---------|------------|
| **TestResult_Map** | Validates Map transforms Ok values, skips Err results | Tests mapping on Ok (21→42) and Err (returns Err) |
| **TestResult_MapErr** | Validates MapErr transforms error messages, preserves Ok values | Tests mapping errors on both Ok and Err results |

### Chaining Tests

| Test Function | Purpose | Test Setup |
|---------------|---------|------------|
| **TestResult_AndThen** | Chains operations that return Results; short-circuits on Err | Tests Ok→Ok chain and Err→Err (short-circuit) |
| **TestResult_OrElse** | Provides fallback Results for Err cases; preserves Ok | Tests Ok (preserved) and Err (fallback invoked) |

### Pattern Matching Tests

| Test Function | Purpose | Test Setup |
|---------------|---------|------------|
| **TestResult_Match** | Validates Match calls appropriate callback (onOk or onErr) | Uses boolean flags to verify correct callback invoked |

### String Representation Tests

| Test Function | Purpose | Test Setup |
|---------------|---------|------------|
| **TestResult_String** | Verifies String() returns non-empty representation for both Ok and Err | Tests both result types |

### Collection Operations Tests

| Test Function | Purpose | Test Setup |
|---------------|---------|------------|
| **TestCollectResults** | Collects successful Results; returns Err if any input is Err | Tests all Ok case, and case with one Err |
| **TestPartitionResults** | Separates Results into Ok values and errors | Tests mixed slice of Ok and Err results |

### Conversion Tests

| Test Function | Purpose | Test Setup |
|---------------|---------|------------|
| **TestResult_ToOption** | Converts Result to Option (Ok→Some, Err→None) | Tests both conversions |

### Helper Function Tests

| Test Function | Purpose | Test Setup |
|---------------|---------|------------|
| **TestAsParseError** | Converts various error types to ParseError | Tests ParseError, SyntaxError, generic error, nil |
| **TestFromError** | Converts (value, error) tuple to Result | Tests nil error (Ok) and non-nil error (Err) |
| **TestWithLineNumber** | Adds line number to ParseError in Err results | Tests Ok (no-op) and Err (adds Line) |
| **TestWithContext** | Adds context string to ParseError (chaining if existing) | Tests Ok (no-op), Err without context, Err with existing context |

### Error Interface Tests

| Test Function | Purpose | Test Setup |
|---------------|---------|------------|
| **TestResult_Error** | Validates Error() method returns nil for Ok, error for Err | Tests both result types |

---

## Tests in `parse_result_test.go` (ParseResultWithError[T] Type)

### Construction Tests

| Test Function | Purpose | Test Setup |
|---------------|---------|------------|
| **TestOkParseResult** | Verifies OkParse construction and all state methods (IsOk, IsError, Unwrap, Error, ErrorMsg, String) | Creates `OkParse(42)` |
| **TestErrResult** | Verifies ErrParse construction with EnhancedParseError | Creates `ErrParse[int]` with SyntaxError |

### Unwrap Tests

| Test Function | Purpose | Test Setup |
|---------------|---------|------------|
| **TestUnwrap** | Tests Unwrap() on Ok; validates panic on Err with recover | Table test with subtests for both cases |
| **TestUnwrapOr** | Tests UnwrapOr() with default values | Table test: Ok case, Err case, zero-value default |
| **TestUnwrapOrElse** | Tests UnwrapOrElse() with lazy callback (can access error) | Table test: Ok, Err with callback, Err accessing error details |

### Transformation Tests

| Test Function | Purpose | Test Setup |
|---------------|---------|------------|
| **TestMap** | Maps Ok values; passes through Err unchanged | Table test: map Ok, map Err (unchanged), type change |
| **TestMapErr** | Maps error on Err; passes through Ok unchanged | Table test: Ok (unchanged), Err (transformed) |

### Chaining Tests

| Test Function | Purpose | Test Setup |
|---------------|---------|------------|
| **TestAndThen** | Chains ParseResultWithError operations; short-circuits on Err | Table test: Ok→Ok, Err→Err (short-circuit, callback not called), Ok→Err in function, multi-step chain |
| **TestOrElse** | Returns alternative Result for Err; preserves Ok | Table test: Ok (preserved), Err→Ok alternative, Err→Err alternative |
| **TestOrElseTry** | Lazy alternative computation with callback | Table test: Ok (callback not called), Err→Ok from callback, lazy computation with counter |

### Error Kind Checking Tests

| Test Function | Purpose | Test Setup |
|---------------|---------|------------|
| **TestErrorKindCheckers** | Validates error type checker functions (IsParseSyntaxError, IsParseStructureError, etc.) | Table test with 7 different error kinds + Ok result |

### Error Accessor Tests

| Test Function | Purpose | Test Setup |
|---------------|---------|------------|
| **TestErrorAccessors** | Tests error metadata accessors (ErrorKind, ErrorFilePath, ErrorLine, ErrorColumn) | Creates Err result with SyntaxError, validates all accessors |
| **TestErrorAccessorsOnOkResult** | Validates error accessors return zero/empty values for Ok results | Ensures safe access on Ok results |

### Display and Formatting Tests

| Test Function | Purpose | Test Setup |
|---------------|---------|------------|
| **TestDetailedString** | Tests DetailedString() with file, snippet, and column indicator | Table test: Ok case, Err case with full location info |
| **TestResultErrorSummary** | Tests ErrorSummary() returns "No error" for Ok, single-line summary for Err | Table test: Ok ("No error"), Err (single-line with error type) |

### Batch Operations Tests

| Test Function | Purpose | Test Setup |
|---------------|---------|------------|
| **TestCollectParseResults** | Collects slice of ParseResultWithError into ParseResults with counts | Mixed slice of Ok and Err results, validates SuccessCount, ErrorCount |
| **TestFilterErrors** | Filters only error results from ParseResults | Uses CollectParseResults then filters |
| **TestFilterSuccesses** | Filters only successful results from ParseResults | Uses CollectParseResults then filters |
| **TestParseResultsErrorSummary** | Tests ErrorSummary() on ParseResults (aggregates multiple errors) | Creates multiple Err results, validates summary includes all errors |

### Type Alias Tests

| Test Function | Purpose | Test Setup |
|---------------|---------|------------|
| **TestMapResultTypeAlias** | Validates MapResult type alias works correctly | Creates MapResult with map[string]interface{} |
| **TestConfigResultTypeAlias** | Validates ConfigResult[T] type alias works correctly | Creates ConfigResult[Config] with struct |

---

## Test Setup/Teardown Requirements

### Common Setup Requirements

1. **ParseError Construction**: Most tests require creating ParseError or EnhancedParseError instances using:
   - `NewParseError()` for generic ParseError
   - `NewSyntaxParseError()`, `NewStructureParseError()`, etc. for EnhancedParseError

2. **Type Parameters**: Tests use generic type parameters like `Result[int, *ParseError]` or `ParseResultWithError[int]`

3. **Panic Recovery**: Unwrap panic tests require `defer/recover` pattern

### No Global Setup/Teardown

- Tests are self-contained with no shared setup
- No `t.Cleanup()` or global fixtures used
- Each test creates its own Result instances

---

## Test Categories Summary

| Category | Test Count | Files |
|----------|------------|-------|
| Construction | 2 | result_test.go |
| Unwrap Operations | 5 | result_test.go |
| Transformation | 2 | result_test.go |
| Chaining | 2 | result_test.go |
| Pattern Matching | 1 | result_test.go |
| String Display | 1 | result_test.go |
| Collections | 2 | result_test.go |
| Conversions | 1 | result_test.go |
| Helpers | 4 | result_test.go |
| Error Interface | 1 | result_test.go |
| **Total (result_test.go)** | **21** | |

| Category | Test Count | Files |
|----------|------------|-------|
| Construction | 2 | parse_result_test.go |
| Unwrap Operations | 3 | parse_result_test.go |
| Transformation | 2 | parse_result_test.go |
| Chaining | 3 | parse_result_test.go |
| Error Checking | 1 | parse_result_test.go |
| Error Accessors | 2 | parse_result_test.go |
| Display/Formatting | 2 | parse_result_test.go |
| Batch Operations | 4 | parse_result_test.go |
| Type Aliases | 2 | parse_result_test.go |
| **Total (parse_result_test.go)** | **21** | |

**Grand Total: 42 TestResult tests** across both files

---

## Key Test Patterns

1. **Table Tests**: Many tests in parse_result_test.go use `t.Run()` for subtest organization
2. **Callback Verification**: UnwrapOrElse, AndThen, OrElseTry verify callback invocation with flags/counters
3. **Both Branches Tested**: Nearly all tests verify both Ok and Err code paths
4. **No External Dependencies**: Tests are pure unit tests with no I/O or external systems

---

## Coverage Notes

- **Ok branch**: Covered by all tests
- **Err branch**: Covered by all tests
- **Type variations**: Tests use `int` as primary type parameter; type aliases validate other types
- **Error kinds**: All 7 EnhancedParseError kinds tested (Syntax, Structure, TypeMismatch, IO, Validation, Schema, Empty)
- **Edge cases**: Zero values, nil errors, panic recovery, empty collections all covered
