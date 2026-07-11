# ParseError Design for Failure Cases

## Overview

This document describes the comprehensive ParseError type design and error handling strategy for YAML parsing failures in the ARMOR project. The design integrates with the Result<T, ParseError> pattern for type-safe error handling.

## Design Goals

1. **Clear Error Categorization**: Distinct error types for different failure modes
2. **Rich Context Information**: Line numbers, column positions, source snippets
3. **Type-Safe Error Handling**: Compile-time guarantees with Result<T, ParseError>
4. **Backward Compatibility**: Integration with existing error types
5. **Error Propagation Strategy**: Clear patterns for error transformation and chaining

## Error Type Hierarchy

```
EnhancedParseError (comprehensive error type)
├── Kind: ParseErrorKind (enum-style discriminator)
│   ├── Syntax (YAML syntax errors)
│   ├── Structure (structure errors like duplicate keys)
│   ├── TypeMismatch (type conversion errors)
│   ├── IO (file I/O errors)
│   ├── Validation (semantic validation errors)
│   ├── Schema (schema validation errors)
│   ├── Empty (empty file errors)
│   └── Unknown (unknown error types)
├── Context: ErrorContext (location and source info)
│   ├── Line (line number)
│   ├── Column (column number)
│   ├── Snippet (source line)
│   ├── SurroundingLines (context lines)
│   └── SnippetLineIndex (error line index)
├── Detail: ParseErrorDetail (error-specific details)
│   ├── Syntax fields (Expected, Found)
│   ├── Structure fields (DuplicateKey, Location)
│   ├── Type fields (FieldPath, ExpectedType, ActualType, Value)
│   ├── Validation fields (ConstraintType, Constraint)
│   └── Schema fields (SchemaPath, SchemaName)
└── Implements YAMLError interface (Code, YAMLErrorType, Context)
```

## Files Created

1. **parse_error_design.go** - EnhancedParseError implementation with:
   - ParseErrorKind enum for error categorization
   - ErrorContext for location and source snippets
   - ParseErrorDetail for error-specific information
   - Construction helpers for each error kind
   - Legacy conversion methods
   - Kind checker methods

2. **parse_result.go** - Result<T, ParseError> pattern implementation with:
   - ParseResultWithError[T] generic type
   - Ok/Err construction functions
   - Unwrap methods with safety guarantees
   - Map/AndThen/OrElse chaining operations
   - Batch operation support
   - Type aliases for common use cases

3. **parse_error_design_test.go** - Comprehensive error type tests
4. **parse_result_test.go** - Comprehensive Result pattern tests

## Summary

The ParseError design provides comprehensive error handling for YAML parsing with clear categorization, rich context, and type-safe error handling patterns.
