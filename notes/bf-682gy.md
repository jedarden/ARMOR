# Bead bf-682gy: Update ParseError in parse_error test files

## Task
Update ParseError constructions in parse_error_design_test.go and parse_error_examples_test.go to use NewParseError().

## Investigation
Verified both files for direct ParseError/EnhancedParseError struct constructions.

### Results
- **parse_error_design_test.go**: All ParseError instances already use constructor functions
- **parse_error_examples_test.go**: All ParseError instances already use constructor functions

### Constructor Functions Used
- `NewSyntaxParseError()`
- `NewStructureParseError()`
- `NewTypeMismatchParseError()`
- `NewIOParseError()`
- `NewValidationParseError()`
- `NewSchemaParseError()`
- `NewEmptyParseError()`

### Pattern Matched
The files follow the same pattern as the recently updated `error_message_quality_test.go`, where direct struct constructions (like `&TypeMismatchError{...}`) are replaced with constructor function calls (like `NewTypeMismatchError(...)`).

## Conclusion
The task is already complete. Both test files correctly use constructor functions instead of direct struct construction. No changes needed.
