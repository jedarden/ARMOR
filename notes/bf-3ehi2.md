# Bead bf-3ehi2: Indentation Level Calculation Function

## Task
Implement a function to calculate the indentation level of YAML lines.

## Status: Already Complete

The `calculateIndentation` function already exists in `internal/yamlutil/line_parser.go` (lines 151-164) and fully meets all acceptance criteria.

## Verification

### Acceptance Criteria Met
- ✅ Function to calculate indentation level (count leading spaces/tabs)
- ✅ Returns the number of leading whitespace characters
- ✅ Returns 0 for lines with no leading whitespace
- ✅ Documented: "Tabs are counted as single characters, not expanded to spaces"
- ✅ Function signature: `calculateIndentation(line string) int`

### Implementation
```go
// calculateIndentation calculates the indentation level of a YAML line.
//
// This function counts leading spaces and tabs to determine indentation depth.
// Tabs are counted as single characters, not expanded to spaces.
//
// Parameters:
//   - line: The line content to analyze
//
// Returns the number of leading whitespace characters (spaces + tabs).
// Returns 0 for lines with no leading whitespace.
func calculateIndentation(line string) int {
	trimmed := strings.TrimLeft(line, " \t")
	return len(line) - len(trimmed)
}
```

### Test Coverage
- `TestCalculateIndentationSimple` - Tests basic indentation counting
- `TestCalculateIndentationTabsAsSingleCharacter` - Verifies tabs are counted as single characters
- `TestCalculateIndentation` - Tests the comprehensive `CalculateIndentation` function
- All tests pass

## Notes
No code changes were needed. The function was already implemented in the codebase.
