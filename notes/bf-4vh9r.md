# Task bf-4vh9r: Update ParseError in core design and example test files

## Task Analysis

Target files:
- `internal/yamlutil/parse_error_design_test.go`
- `internal/yamlutil/parse_error_examples_test.go`

Goal: Replace direct ParseError struct constructions with NewParseError() calls.

## Current State

After analyzing both target files:

### parse_error_design_test.go
- Uses specialized EnhancedParseError constructors:
  - `NewSyntaxParseError()`
  - `NewStructureParseError()`
  - `NewTypeMismatchParseError()`
  - `NewIOParseError()`
  - `NewValidationParseError()`
  - `NewSchemaParseError()`
  - `NewEmptyParseError()`

- NO direct `ParseError{}` or `&ParseError{}` constructions found

### parse_error_examples_test.go
- Uses the same specialized EnhancedParseError constructors
- NO direct `ParseError{}` or `&ParseError{}` constructions found

## Conclusion

Both test files are already properly using constructor functions for the EnhancedParseError type. These files test the EnhancedParseError design, which is separate from the legacy ParseError type (which uses `NewParseError()`).

The task acceptance criteria are already met:
- ✓ All ParseError struct constructions use constructors
- ✓ Test logic remains identical  
- ✓ Tests remain readable

No changes were required - the files were already in compliance.
