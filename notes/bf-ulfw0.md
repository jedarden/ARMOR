# Task: Update ParseError in error_formatting and verify test files (bf-ulfw0)

## Finding

The task requested updating ParseError constructions to use `NewParseError()` in three test files:

1. `error_message_format_examples_test.go`
2. `verify_error_formatting_test.go`
3. `verify_formatting_test.go`

## Status: Already Complete

All three test files **already use `NewParseError()` constructor calls**. No direct `ParseError{...}` struct literals exist in these files.

### Verification

```bash
# Count NewParseError usages in target files
grep -c "NewParseError" internal/yamlutil/error_message_format_examples_test.go  # 7 calls
grep -c "NewParseError" internal/yamlutil/verify_error_formatting_test.go         # 3 calls
grep -c "NewParseError" internal/yamlutil/verify_formatting_test.go               # 2 calls

# Search for any direct ParseError struct literals (found none)
grep -n "ParseError{" internal/yamlutil/error_message_format_examples_test.go
grep -n "ParseError{" internal/yamlutil/verify_error_formatting_test.go
grep -n "ParseError{" internal/yamlutil/verify_formatting_test.go
```

Result: **No direct `ParseError{` struct constructions found** in any of the three target test files.

### Examples from the files

All ParseError instances already use the constructor:

```go
// error_message_format_examples_test.go (line 38)
err := NewParseError("config.yaml", "missing colon", 10, 5, ErrCodeInvalidSyntax, "", "")

// verify_error_formatting_test.go (line 11)
pe := NewParseError("config.yaml", "invalid syntax", 10, 5, "", "identifier", "123")

// verify_formatting_test.go (line 13)
err := NewParseError("config.yaml", "invalid syntax", 10, 5, ErrCodeInvalidSyntax, "", "")
```

## Conclusion

No file changes required. The test files are already properly using `NewParseError()` constructors throughout, maintaining test logic and readability as required by the acceptance criteria.
