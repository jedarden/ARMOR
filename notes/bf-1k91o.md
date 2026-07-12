# Bead bf-1k91o: Update ParseError in remaining test files

## Task
Update ParseError constructions in errors_parsevariant_test.go and examples_test.go to use NewParseError().

## Verification Result

After thorough analysis, both files **do not contain any direct ParseError struct constructions**:

### errors_parsevariant_test.go
- Tests the `ParseErrorVariant` enum type
- Contains tests for String(), Description(), variant count, and distinctness
- No `ParseError{` or `&ParseError{` constructions found

### examples_test.go  
- Contains example functions for Go documentation
- Uses type assertions only: `if parseErr, ok := err.(*YAMLParseError); ok`
- No direct ParseError struct constructions found

## Conclusion
The task is complete as-is - there were no ParseError struct constructions to replace in these files. Both files were already using the correct patterns (enum testing and type assertions).
