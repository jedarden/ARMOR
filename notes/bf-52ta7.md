# Bead bf-52ta7: Update ParseError in remaining test files

## Task
Update ParseError constructions in remaining test files to use NewParseError().

## Files Verified
1. `internal/yamlutil/errors_parsevariant_test.go`
2. `internal/yamlutil/examples_test.go`

## Findings
**NO CHANGES NEEDED** - Both files are already correct:

### errors_parsevariant_test.go
- Tests `ParseErrorVariant` enum type (not `ParseError` struct)
- Contains no direct `ParseError{...}` struct constructions
- Only tests variant enum values and their String()/Description() methods

### examples_test.go
- Contains example functions demonstrating YAML parsing usage
- Uses type assertions `err.(*YAMLParseError)` to inspect errors
- Contains NO direct `ParseError{...}` struct constructions
- Error handling is correct - only inspects errors returned by parsing functions

## Verification Command
```bash
grep -n "ParseError{" internal/yamlutil/errors_parsevariant_test.go internal/yamlutil/examples_test.go
# Result: No ParseError{ found
```

## Conclusion
The files listed in the bead do not contain any direct ParseError struct constructions that need updating. The task is complete - there are no changes to make.
