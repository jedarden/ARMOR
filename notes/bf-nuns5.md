# ParseError Verification - bf-nuns5

## Task
Update ParseError constructions in core error test files to use NewParseError().

## Files Checked
1. `internal/yamlutil/errors_test.go`
2. `internal/yamlutil/error_cases_test.go`

## Findings
Both files are **already compliant** with the requirement:

### errors_test.go
- Line 28: `NewParseError("test.yaml", "", 0, 0, "", "", "")`
- Line 76: `NewParseError("test.yaml", "", 0, 0, "", "", "")`
- Line 96: `fmt.Errorf("wrapped: %w", NewParseError(...))`
- Line 153: `NewParseError("test.yaml", "", 0, 0, "", "", "")`
- Line 158: `fmt.Errorf("wrapped: %w", NewParseError(...))`
- Line 287: `NewParseError(...)` in test body

### error_cases_test.go
- No direct ParseError struct constructions found
- Uses `NewYAMLParseError()` for YAML parsing errors (different type)

## Verification
```bash
# No direct struct constructions found
grep -n "ParseError{" internal/yamlutil/errors_test.go internal/yamlutil/error_cases_test.go
grep -n "&ParseError{" internal/yamlutil/errors_test.go internal/yamlutil/error_cases_test.go

# All tests pass
go test -v ./internal/yamlutil/ -run "Test.*Error"
```

## Result
✅ Task already complete - both files use `NewParseError()` exclusively
