# int32 Negative Conversion Test Pattern Analysis

## Overview
This document captures the test structure pattern from `int32_to_uint32_negative_conversion_test.go` for reference when implementing the int64 version.

## Test Case Structure

### Core Test Fields
```go
tests := []struct {
    name          string     // Test case name (used in t.Run)
    yamlContent   string     // YAML snippet to parse
    target        interface{} // Target struct for unmarshaling
    shouldError   bool       // Expected error outcome
    description   string     // Human-readable description
    expectedInMsg []string   // Optional: substrings expected in error message
}
```

### Field Usage Details

1. **name** - Test identifier in format: `"int32 <value> to uint32 should <outcome>"`
   - Examples: `"int32 -1 to uint32 should error"`, `"int32 100 to uint32 should succeed"`

2. **yamlContent** - Raw YAML with proper indentation (usually 1 space for first-level content)
   - Minimal structure: `\nvalue: <number>\n`

3. **target** - Anonymous struct with specific typed field
   - Pattern: `&struct{ Value <targetType> }{}`
   - For uint32: `&struct{ Value uint32 }{}`

4. **shouldError** - Boolean flag
   - `true` for negative values and overflow cases
   - `false` for valid positive values within range

5. **description** - Explains what the test validates
   - Format: `"<Value> <context>" or "<What> <outcome>"`

6. **expectedInMsg** - Optional array of error message patterns
   - Used for error quality verification
   - Common patterns: `"cannot unmarshal"`, `"<actual_value>"`

## Test Structure Pattern

### Standard Test Execution Loop
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

                // Error message quality checks (optional)
                errMsg := err.Error()
                lowerErrMsg := strings.ToLower(errMsg)

                for _, expected := range tt.expectedInMsg {
                    if !strings.Contains(errMsg, expected) && 
                       !strings.Contains(lowerErrMsg, strings.ToLower(expected)) {
                        t.Logf("Note: Error message doesn't contain expected pattern %q: %s", expected, errMsg)
                    }
                }

                // Verify negative value indication
                if containsAny(lowerErrMsg, []string{"negative", "cannot unmarshal", "out of range", "invalid"}) {
                    t.Logf("✓ Error message indicates invalid conversion for negative value")
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

## Test Categories (from int32 version)

1. **Basic Negative Conversions** - Core negative value tests
   - Edge cases: -1, minimum int32 (-2147483648)
   - Various negative magnitudes

2. **Nested Structures** - Negative values in complex YAML structures
   - Nested structs, arrays, maps, slices of structs

3. **Different Formats** - Various YAML representations
   - Decimal format, zero-padded, quoted strings, octal, hex

4. **Boundary Values** - Range edge cases
   - Minimum/maximum values for both int32 and uint32
   - Overflow cases (values exceeding uint32 maximum)

5. **Error Message Quality** - Verify error message content
   - Focus on expectedInMsg patterns and error clarity

## Helper Functions

### containsAny (local helper)
```go
func containsAny(s string, patterns []string) bool {
    for _, pattern := range patterns {
        if strings.Contains(s, pattern) {
            return true
        }
    }
    return false
}
```
- Used for checking if error message contains any of several expected patterns
- Applied to lowercase version of error message for case-insensitive matching

## Key Structural Elements to Replicate for int64

1. **Test file naming**: `int64_to_uint64_negative_conversion_test.go`
2. **Package declaration**: `package yamlutil`
3. **Imports**: `strings`, `testing`
4. **Main test function**: `TestInt64ToUint64NegativeConversion`
5. **Value range adjustments**:
   - Minimum int64: -9223372036854775808
   - Maximum int64: 9223372036854775807
   - Maximum uint64: 18446744073709551615
6. **Test categories** with int64-appropriate values
7. **Optional**: Add similar test functions for nested structures, formats, boundaries, error quality

## Naming Convention Pattern

- Test function: `TestInt64ToUint64NegativeConversion`
- Test names: `"int64 <value> to uint64 should <outcome>"`
- Description: `<Value> <context> <outcome>`

## Example: Simple Negative Test Case (int64)

```go
{
    name: "int64 -1 to uint64 should error",
    yamlContent: `
value: -1
`,
    target:        &struct{ Value uint64 }{},
    shouldError:   true,
    description:   "Negative value -1 cannot convert to uint64",
    expectedInMsg: []string{"cannot unmarshal", "-1"},
},
```
