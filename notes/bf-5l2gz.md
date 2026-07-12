# Bead bf-5l2gz: Verify ParseError uses constructors

## Task
Update ParseError constructions to use NewParseError() in parse_error test files.

## Files Analyzed
- `internal/yamlutil/parse_error_design_test.go`
- `internal/yamlutil/parse_error_examples_test.go`

## Findings

### File Types
Both files test `EnhancedParseError` (not the legacy `ParseError` type).

### Constructor Usage
All error constructions already use proper constructor functions:
- `NewSyntaxParseError()`
- `NewStructureParseError()`
- `NewTypeMismatchParseError()`
- `NewIOParseError()`
- `NewValidationParseError()`
- `NewSchemaParseError()`
- `NewEmptyParseError()`

### Verification
- No direct `&EnhancedParseError{...}` struct constructions found
- No direct `&ParseError{...}` struct constructions found
- Count of constructor calls:
  - `parse_error_examples_test.go`: 10 constructor calls
  - `parse_error_design_test.go`: 17 constructor calls

## Conclusion
The task is already complete. Both files use constructor functions exclusively for creating parse errors. No changes needed.

## Note
The task description mentioned `NewParseError()` which is a constructor for the legacy `ParseError` type. However, the specified files test `EnhancedParseError` which uses different, type-specific constructor functions.
