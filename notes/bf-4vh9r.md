# Bead bf-4vh9r: Verify ParseError Usage in Core Test Files

## Task
Update ParseError constructions to use NewParseError() in core test files.

## Files Verified
- `internal/yamlutil/parse_error_design_test.go`
- `internal/yamlutil/parse_error_examples_test.go`

## Findings

Both files test `EnhancedParseError` (not legacy `ParseError`). These files already use the proper `NewXxxParseError()` constructor functions:

### parse_error_design_test.go
- Uses `NewSyntaxParseError()` (8 occurrences)
- Uses `NewStructureParseError()` (9 occurrences)
- Uses `NewTypeMismatchParseError()` (5 occurrences)
- Uses `NewIOParseError()` (4 occurrences)
- Uses `NewValidationParseError()` (4 occurrences)
- Uses `NewSchemaParseError()` (4 occurrences)
- Uses `NewEmptyParseError()` (5 occurrences)

### parse_error_examples_test.go
- Uses `NewSyntaxParseError()` (8 occurrences)
- Uses `NewStructureParseError()` (2 occurrences)
- Uses `NewTypeMismatchParseError()` (3 occurrences)
- Uses `NewIOParseError()` (2 occurrences)
- Uses `NewValidationParseError()` (2 occurrences)
- Uses `NewSchemaParseError()` (1 occurrence)
- Uses `NewEmptyParseError()` (1 occurrence)

## Results
✓ All ParseError constructions already use constructor functions
✓ No direct struct literals found (0 matches)
✓ Test logic remains correct and readable

## Conclusion
No changes required - the task is already complete. These files were already using the proper EnhancedParseError constructor functions.
