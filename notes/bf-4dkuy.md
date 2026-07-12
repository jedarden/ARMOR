# Int32 Negative Conversion Test Pattern Reference

**Task:** bf-4dkuy
**Date:** 2026-07-12

## Overview

This document establishes the test case pattern used in `int32_to_uint32_negative_conversion_test.go` for reference when building similar tests for int64.

## Test File Structure

### Test Case Field Structure

```go
tests := []struct {
    name          string   // Test name: "<type> <value> to <target> should <result>"
    yamlContent   string   // YAML content to parse (multi-line string)
    target        interface{} // Target struct for unmarshaling
    shouldError   bool     // Whether parsing should produce error
    description   string   // Human-readable test description
    expectedInMsg []string // Optional: expected patterns in error message
}{
    // test cases...
}
```

### Field Naming Conventions

| Field | Convention | Example |
|-------|-----------|---------|
| `name` | `"int32 <value> to uint32 should <result>"` | `"int32 -1 to uint32 should error"` |
| `shouldError` | `true` for negative values, `false` for valid | `true` for -1 |
| `expectedInMsg` | Substrings expected in error | `[]string{"cannot unmarshal", "-1"}` |
| `description` | Human readable | `"Negative value -1 cannot convert to uint32"` |

## Test Categories

### 1. Basic Negative Conversion Tests

Tests negative int32 values converting to uint32:

- Edge cases: `-1`, `-2147483648` (min int32)
- Range of values: `-2`, `-10`, `-100`, `-128`, `-256`, `-1000`, `-32768`, `-65536`, `-1073741824`, `-2147483647`
- Extreme negatives beyond int32: `-2147483649`, `-4294967296`

### 2. Positive Control Tests

Tests that positive values still work:

- Zero: `0`
- Small positives: `100`, `255`, `65535`, `65536`
- Maximums: `2147483647` (max int32), `4294967295` (max uint32)

### 3. Nested Structure Tests

Tests negative values in complex structures:

- Nested structs: `config.port: -8080`
- Arrays: `[-8080, -443, 3000]`
- Maps: `services: {http: -443}`
- Slice of structs: `servers: [{port: -8443}]`

### 4. Different YAML Formats

Tests negative values in various YAML syntaxes:

- Decimal format: `-100.0`
- Zero-padded: `-00050`
- String quoted: `"-256"`
- Octal string: `"-0400"`
- Hex string: `"-0x100"`

### 5. Boundary Value Tests

Tests boundary conditions:

- Negative boundaries: min int32, common boundaries (-256, -32768, -65536)
- Positive boundaries: 0, 255, 65535, 65536, max int32, max uint32
- Overflow case: `4294967296` (exceeds uint32)

### 6. Error Message Quality Tests

Tests that error messages contain expected patterns:

- Value appears in error: `-1` appears in error message
- Keywords: "cannot unmarshal", "negative", "invalid", "out of range"

## Test Execution Pattern

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

                // Verify error message patterns (optional)
                errMsg := err.Error()
                lowerErrMsg := strings.ToLower(errMsg)
                for _, expected := range tt.expectedInMsg {
                    if !strings.Contains(errMsg, expected) && !strings.Contains(lowerErrMsg, strings.ToLower(expected)) {
                        t.Logf("Note: Error message doesn't contain expected pattern %q: %s", expected, errMsg)
                    }
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

## Helper Functions

### `containsAny` Helper

```go
// Helper function to check if a string contains any of the given patterns
func containsAny(s string, patterns []string) bool {
    for _, pattern := range patterns {
        if strings.Contains(s, pattern) {
            return true
        }
    }
    return false
}
```

**Usage:** Verify error messages indicate invalid conversion

```go
if containsAny(lowerErrMsg, []string{"negative", "cannot unmarshal", "out of range", "invalid"}) {
    t.Logf("✓ Error message indicates invalid conversion for negative value")
}
```

## Key Structural Elements

1. **Test Function Organization**: Separate test functions for each category (basic, nested, formats, boundaries, message quality)

2. **Table-Driven Tests**: All tests use the `[]struct{}` pattern for consistency

3. **Subtests via t.Run**: Each test case runs as a named subtest

4. **Error Message Verification**: Optional checking of error message content via `expectedInMsg` and `containsAny`

5. **Descriptive Logging**: Uses `t.Logf` with checkmarks (✓) for passing cases

## Pattern for int64 Adaptation

When replicating for int64 → uint64:

1. **Type substitutions**:
   - `int32` → `int64`
   - `uint32` → `uint64`
   - `-2147483648` → `-9223372036854775808` (min int64)
   - `2147483647` → `9223372036854775807` (max int64)
   - `4294967295` → `18446744073709551615` (max uint64)

2. **Boundary values to test**:
   - Min int64: `-9223372036854775808`
   - Near-min int64: `-9223372036854775807`
   - Powers of 2: `-2^63` through `-2^8`
   - Max int64: `9223372036854775807`
   - Max uint64: `18446744073709551615`
   - Overflow: `18446744073709551616`

3. **Maintain same structure**:
   - Same test categories (basic, nested, formats, boundaries, messages)
   - Same field names and conventions
   - Same execution pattern with t.Run
   - Same helper functions

## Example Test Case Int32 → Int64

### Int32 version:
```go
{
    name: "int32 -2147483648 to uint32 should error",
    yamlContent: `
value: -2147483648
`,
    target:        &struct{ Value uint32 }{},
    shouldError:   true,
    description:   "Minimum int32 value -2147483648 cannot convert to uint32",
    expectedInMsg: []string{"cannot unmarshal"},
}
```

### Int64 version:
```go
{
    name: "int64 -9223372036854775808 to uint64 should error",
    yamlContent: `
value: -9223372036854775808
`,
    target:        &struct{ Value uint64 }{},
    shouldError:   true,
    description:   "Minimum int64 value -9223372036854775808 cannot convert to uint64",
    expectedInMsg: []string{"cannot unmarshal"},
}
```

## Dependencies

- `testing` package
- `strings` package
- `NewParser()` from yamlutil package

## File Location

`/home/coding/ARMOR/internal/yamlutil/int32_to_uint32_negative_conversion_test.go`
