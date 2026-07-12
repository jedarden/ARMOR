# Task: Update ParseError in parse_error test files

## Files Checked
- `internal/yamlutil/parse_error_design_test.go`
- `internal/yamlutil/parse_error_examples_test.go`

## Finding
Both files already use `NewParseError()` constructor functions exclusively. No direct `ParseError{` struct constructions were found.

### Current Usage in parse_error_design_test.go
- `NewSyntaxParseError()`
- `NewStructureParseError()`
- `NewTypeMismatchParseError()`
- `NewIOParseError()`
- `NewValidationParseError()`
- `NewSchemaParseError()`
- `NewEmptyParseError()`

### Current Usage in parse_error_examples_test.go
- `NewSyntaxParseError()`
- `NewStructureParseError()`
- `NewTypeMismatchParseError()`
- `NewIOParseError()`
- `NewValidationParseError()`
- `NewEmptyParseError()`
- `NewSchemaParseError()`

## Conclusion
Task acceptance criteria already met. All ParseError constructions use the appropriate constructor functions. Tests remain readable and logic is intact. No code changes required.
