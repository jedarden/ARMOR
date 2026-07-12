# int32 Test File Pattern Analysis (bf-3bn2e)

## Overview
Analyzed the int32 negative conversion test file (`int32_to_uint32_negative_conversion_test.go`) to establish the correct pattern for test structures.

## File Structure Pattern

### Package Declaration
```go
// Package yamlutil tests for int32 to uint32 negative conversion scenarios
package yamlutil
```

### Imports
```go
import (
    "strings"
    "testing"
)
```

## Core Test Structure

### Test Case Struct Pattern
```go
tests := []struct {
    name          string
    yamlContent   string
    target        interface{}
    shouldError   bool
    description   string
    expectedInMsg []string  // Optional: for error message validation
}{
    // Test cases here
}
```

### Test Execution Pattern
```go
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        parser := NewParser()
        
        err := parser.ParseString(tt.yamlContent, tt.target)
        
        if tt.shouldError {
            if err == nil {
                t.Errorf("Test '%s' should error but didn't: %s", tt.name, tt.description)
            } else {
                t.Logf("✓ Test '%s' correctly produced error: %v", tt.name, err)
                
                // Verify error message contains expected patterns
                errMsg := err.Error()
                lowerErrMsg := strings.ToLower(errMsg)
                
                // Check for expected patterns in error message
                for _, expected := range tt.expectedInMsg {
                    if !strings.Contains(errMsg, expected) && !strings.Contains(lowerErrMsg, strings.ToLower(expected)) {
                        t.Logf("Note: Error message doesn't contain expected pattern %q: %s", expected, errMsg)
                    }
                }
                
                // Verify negative value indication in error
                if containsAny(lowerErrMsg, []string{"negative", "cannot unmarshal", "out of range", "invalid"}) {
                    t.Logf("✓ Error message indicates invalid conversion for negative value")
                } else {
                    t.Logf("Note: Error message doesn't explicitly mention negative value: %s", errMsg)
                }
            }
        } else {
            if err != nil {
                t.Errorf("Test '%s' should succeed but errored: %v - %s", tt.name, err, tt.description)
            } else {
                t.Logf("✓ Test '%s' correctly succeeded: %s", tt.name, tt.description)
            }
        }
    })
}
```

## Test Function Categories

### 1. Basic Conversion Tests (`TestInt32ToUint32NegativeConversion`)
- Edge cases: -1, minimum values
- Range of negative values: -2, -10, -100, -128, -256, -32768, -65536, -1000000, etc.
- Extreme negative values beyond int32 range
- Positive value verification (0, 100, 65535, 2147483647, 4294967295)

### 2. Nested Structure Tests (`TestInt32ToUint32NegativeInNestedStructs`)
- Nested structs with negative uint32 fields
- Arrays with negative values
- Maps with negative values
- Slices of structs with negative values
- Large negative values in nested structs

### 3. Format Variation Tests (`TestInt32ToUint32NegativeWithDifferentFormats`)
- Negative decimal format
- Negative zero-padded values
- Negative string values
- Negative octal strings
- Negative hex strings

### 4. Boundary Value Tests (`TestInt32ToUint32BoundaryValues`)
- Minimum int32 value (-2147483648)
- Values above minimum
- Boundary positives (0, 255, 65535, 65536, 2147483647)
- Maximum uint32 value (4294967295)
- Overflow values (4294967296)

### 5. Error Message Quality Tests (`TestInt32ToUint32ErrorMessageQuality`)
- Validates that error messages contain expected patterns
- Checks for negative value indication in errors
- Ensures error messages are descriptive

## Key Structural Elements

### 1. Comments and Documentation
- Clear package-level comment explaining test purpose
- Test case comments categorizing test types (edge case, additional values, etc.)
- Inline comments explaining specific test scenarios

### 2. Test Naming Convention
- Pattern: `Test[SourceType]To[TargetType][TestCategory]`
- Examples: `TestInt32ToUint32NegativeConversion`, `TestInt32ToUint32BoundaryValues`
- Descriptive and consistent naming

### 3. YAML Content Format
```go
yamlContent: `
value: -1
`,
```

### 4. Target Structure Pattern
```go
target: &struct{ Value uint32 }{},
```

### 5. Error Handling Pattern
- `shouldError: true` for negative values
- `shouldError: false` for valid positive values
- Optional `expectedInMsg` field for error pattern validation

## Differences from int64 Version

### 1. Range Differences
- **int32**: Minimum value -2147483648, Maximum positive 4294967295
- **int64**: Minimum value -9223372036854775808, Maximum positive 18446744073709551615

### 2. Test Case Differences
- **int32** has extreme values beyond int32 range (lines 165-186):
  - `-2147483649` (below int32 minimum)
  - `-4294967296` (far below int32 minimum)
- **int64** has a note about YAML parser wrapping behavior (lines 187-190)
- **int64** has additional test cases in nested structs:
  - `int64 negative int32 minimum value` test (lines 384-398)
  - `int64 negative in nested map of structs` test (lines 400-418)

### 3. Format Differences
- **int64** includes scientific notation tests (lines 499-515):
  - `"-1.0e9"`
  - `"-9.223372036854775808e18"`

### 4. Boundary Differences
- **int32** overflow test expects error (line 600-608):
  - `4294967296` → `shouldError: true`
- **int64** overflow test wraps (lines 689-697):
  - `18446744073709551616` → `shouldError: false` with comment "YAML parser wraps overflow values"

## Helper Function
```go
func containsAny(s string, substrs []string) bool {
    for _, substr := range substrs {
        if strings.Contains(s, substr) {
            return true
        }
    }
    return false
}
```

## Best Practices Identified

1. **Comprehensive Coverage**: Test edge cases, boundaries, formats, and nested structures
2. **Clear Documentation**: Comments explain what each test validates
3. **Consistent Structure**: All tests follow the same pattern
4. **Error Validation**: Check for expected error patterns
5. **Positive Cases**: Include valid conversions to ensure they still work
6. **Logging**: Use t.Logf for success confirmation, not just errors
7. **Descriptive Names**: Test names clearly indicate what they test
8. **Category Organization**: Related tests grouped in separate functions

## Pattern for Replication

When creating similar test files for other integer types:

1. **Keep the same test case struct pattern**
2. **Include all 5 test function categories** (basic, nested, formats, boundaries, error quality)
3. **Adjust values for the specific integer range**
4. **Maintain consistent naming conventions**
5. **Include comments explaining test scenarios**
6. **Add specific edge cases for the type being tested**
7. **Test both error and success cases**
8. **Validate error message quality**

## Critical Notes

1. **The `containsAny` helper function** is used throughout the tests and must be available
2. **Error message validation** is case-insensitive for flexibility
3. **ExpectedInMsg is optional** - only include when specific error patterns need validation
4. **YAML parser behavior differences** exist between int32 and int64 for overflow values
5. **Nested struct behavior** may differ - some values wrap silently in nested contexts
