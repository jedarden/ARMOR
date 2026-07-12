# Bead bf-2p4o1: Update ParseError in parser and result test files

## Task
Update ParseError constructions in parser and result test files to use NewParseError().

## Files Verified
- `internal/yamlutil/parser_test.go`
- `internal/yamlutil/parse_result_test.go`
- `internal/yamlutil/result_types_test.go`
- `internal/yamlutil/result_test.go`

## Findings
All test files already use constructor functions exclusively:

### parser_test.go
- Uses `NewYAMLParseError()` for YAMLParseError construction
- No direct ParseError struct constructions found

### parse_result_test.go
- Uses `NewSyntaxParseError()` for syntax errors
- Uses `NewEmptyParseError()` for empty file errors
- Uses `NewStructureParseError()` for structure errors
- Uses `NewTypeMismatchParseError()` for type mismatch errors
- Uses `NewIOParseError()` for IO errors
- Uses `NewValidationParseError()` for validation errors
- Uses `NewSchemaParseError()` for schema errors
- Works with `EnhancedParseError` throughout
- No direct ParseError struct constructions found

### result_types_test.go
- Uses `NewSyntaxParseError()` for syntax errors
- Uses `NewTypeMismatchParseError()` for type mismatch errors
- Uses `NewValidationError()` for ValidationError construction
- No direct ParseError struct constructions found

### result_test.go
- Uses `NewParseError()` throughout for base ParseError construction
- No direct ParseError struct constructions found

## Acceptance Criteria
✅ All ParseError struct constructions already replaced with NewParseError() and related constructors
✅ Test logic remains identical (no changes needed)
✅ Tests remain readable (already using clear constructor functions)

## Conclusion
All four test files have already been refactored to use the appropriate constructor functions. No direct ParseError struct constructions exist in any of the target files. The refactoring was completed in a previous session.
