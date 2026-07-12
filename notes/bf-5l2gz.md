# Task bf-5l2gz: Update ParseError in parse_error test files

## Task Summary
Update ParseError constructions to use NewParseError() in parse_error test files.

## Files Analyzed
- `internal/yamlutil/parse_error_design_test.go`
- `internal/yamlutil/parse_error_examples_test.go`

## Analysis Results

### parse_error_design_test.go
- **Type Used**: `EnhancedParseError` (not the older `ParseError` type)
- **Current State**: Already uses constructor functions
- **Constructor Functions Used**: 39 instances
  - `NewSyntaxParseError()` - for syntax errors
  - `NewStructureParseError()` - for structure errors
  - `NewTypeMismatchParseError()` - for type mismatch errors
  - `NewIOParseError()` - for I/O errors
  - `NewValidationParseError()` - for validation errors
  - `NewSchemaParseError()` - for schema errors
  - `NewEmptyParseError()` - for empty file errors
- **Direct Struct Constructions Found**: 0

### parse_error_examples_test.go
- **Type Used**: `EnhancedParseError` (not the older `ParseError` type)
- **Current State**: Already uses constructor functions
- **Constructor Functions Used**: 19 instances
  - Same set of constructors as above
- **Direct Struct Constructions Found**: 0

## Note on &yamlError{ Construction
The only direct struct construction found (`&yamlError{}` at line 283) is:
- A local mock type defined within Example 10 (`ExampleEnhancedParseError_transformFromYAML`)
- Used to simulate a yaml.v3 parser error for demonstration purposes
- Not related to the `ParseError` or `EnhancedParseError` types
- This is intentional example code showing how to transform external errors into `EnhancedParseError`

## Conclusion
Both files are **already in the correct state**. They use constructor functions instead of direct struct construction, which is exactly what the task requires. No changes were needed.

The task requirements have been met:
- ✅ All error constructions use appropriate `New*ParseError()` constructor functions
- ✅ No direct `EnhancedParseError{` or `ParseError{` struct constructions
- ✅ Test logic remains identical
- ✅ Tests remain readable

## Verification Date
2026-07-12
