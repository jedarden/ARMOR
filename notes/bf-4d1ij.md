# Int32 Test Pattern Structure Documentation

## Overview

This document analyzes the structure and patterns used in `int32_to_uint32_negative_conversion_test.go` to provide a clear template for int64 and similar integer conversion tests.

## Test Case Structure

### Core Fields

All test cases use this standard structure:

```go
tests := []struct {
    name          string        // Test case name for t.Run()
    yamlContent   string        // YAML input to parse
    target        interface{}   // Target struct for unmarshaling
    shouldError   bool          // Expected error state
    description   string        // Human-readable test description
    expectedInMsg []string      // Optional: strings that should appear in error message
}
```

### Field Descriptions

| Field | Type | Purpose | Example |
|-------|------|---------|---------|
| `name` | string | Test case identifier | `"int32 -1 to uint32 should error"` |
| `yamlContent` | string | YAML input to parse | `\nvalue: -1\n` |
| `target` | interface{} | Destination struct for unmarshal | `&struct{ Value uint32 }{}` |
| `shouldError` | bool | Whether error is expected | `true` for negative values |
| `description` | string | What the test validates | `"Negative value -1 cannot convert to uint32"` |
| `expectedInMsg` | []string | Error message substrings to verify | `[]string{"cannot unmarshal", "-1"}` |

## Test Function Types

### 1. Main Conversion Test

**Function**: `TestInt32ToUint32NegativeConversion`

Tests basic negative int32 to uint32 conversion scenarios.

**Key test categories**:
- Edge cases: -1, minimum value (-2147483648)
- Common negative values: -100, -256, -1000, etc.
- Values beyond int32 range (parser behavior tests)
- Positive control values (should succeed)

**Example structure**:
```go
{
    name: "int32 -1 to uint32 should error",
    yamlContent: `
value: -1
`,
    target:        &struct{ Value uint32 }{},
    shouldError:   true,
    description:   "Negative value -1 cannot convert to uint32",
    expectedInMsg: []string{"cannot unmarshal", "-1"},
},
```

### 2. Nested Structures Test

**Function**: `TestInt32ToUint32NegativeInNestedStructs`

Tests negative values in complex data structures.

**Structure** (note: no `expectedInMsg` field):
```go
{
    name:        string,
    yamlContent: string,
    target:      interface{},
    shouldError: bool,
    description: string,
}
```

**Test scenarios**:
- Nested struct with negative value
- Arrays containing negative values
- Maps with negative values
- Slices of structs with negative values

### 3. Different Formats Test

**Function**: `TestInt32ToUint32NegativeWithDifferentFormats`

Tests various YAML format representations.

**Same structure as Nested Structures** (no `expectedInMsg`).

**Test scenarios**:
- Negative decimal format: `-100.0`
- Zero-padded: `-00050`
- String-quoted: `"-256"`
- Octal: `"-0400"`
- Hexadecimal: `"-0x100"`

### 4. Boundary Values Test

**Function**: `TestInt32ToUint32BoundaryValues`

Tests numeric boundaries and edge cases.

**Structure** (includes `expectedInMsg`):
```go
{
    name:          string,
    yamlContent:   string,
    target:        interface{},
    shouldError:   bool,
    description:   string,
    expectedInMsg: []string,  // Used for error quality verification
}
```

**Negative boundaries**:
- Minimum int32: `-2147483648`
- One above minimum: `-2147483647`
- Powers of 2: `-65536`, `-32768`, `-256`, `-128`

**Positive boundaries**:
- Zero: `0`
- Type maximums: `255` (uint8), `65535` (uint16), `4294967295` (uint32)
- Overflow cases: `4294967296` (should error)

### 5. Error Message Quality Test

**Function**: `TestInt32ToUint32ErrorMessageQuality`

Verifies that error messages contain useful information.

**Structure** (uses `errorPatterns` instead of `expectedInMsg`):
```go
{
    name:          string,
    yamlContent:   string,
    target:        interface{},
    errorPatterns: []string,  // Note: different field name
    description:   string,
}
```

**Purpose**: Each test case verifies that error messages contain specific patterns for debugging.

## Helper Function: containsAny

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

**Usage**: Checks if error message contains any of several valid indicators.

**Example**:
```go
if containsAny(lowerErrMsg, []string{"negative", "cannot unmarshal", "out of range", "invalid"}) {
    t.Logf("✓ Error message indicates invalid conversion")
}
```

## Test Execution Pattern

### Standard Test Runner Loop

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
                // Optional: Verify error message quality
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

### Error Message Verification Pattern

When `expectedInMsg` is provided:

```go
errMsg := err.Error()
lowerErrMsg := strings.ToLower(errMsg)

for _, expected := range tt.expectedInMsg {
    if !strings.Contains(errMsg, expected) && !strings.Contains(lowerErrMsg, strings.ToLower(expected)) {
        t.Logf("Note: Error message doesn't contain expected pattern %q: %s", expected, errMsg)
    }
}
```

## Key Differences: Int32 vs Int64

### Value Ranges

| Type | Minimum | Maximum |
|------|---------|---------|
| int32 | -2147483648 | 2147483647 |
| int64 | -9223372036854775808 | 9223372036854775807 |
| uint32 | 0 | 4294967295 |
| uint64 | 0 | 18446744073709551615 |

### Target Types

- **Int32 tests**: Use `uint32` target
- **Int64 tests**: Use `uint64` target

### Behavioral Differences

1. **Overflow handling**: 
   - int32: `4294967296` produces error
   - int64: `18446744073709551616` wraps silently (YAML parser difference)

2. **Decimal format**:
   - int32: `-100.0` produces error
   - int64: `-100.0` converts to 100 (no error)

## Template for New Tests

### Basic Test Case Template

```go
{
    name: "<type> <value> to <target-type> should error",
    yamlContent: `
value: <test-value>
`,
    target:        &struct{ Value <target-type> }{},
    shouldError:   true,
    description:   "<human-readable description>",
    expectedInMsg: []string{"cannot unmarshal", "<value>"},
},
```

### Boundary Test Template

```go
{
    name: "<type> <boundary-name> <value> to <target-type>",
    yamlContent: `
value: <value>
`,
    target:        &struct{ Value <target-type> }{},
    shouldError:   <true-for-negative-false-for-positive>,
    description:   "<boundary-description>",
    expectedInMsg: []string{"cannot unmarshal"},
},
```

### Error Quality Test Template

```go
{
    name: "<type> <value> error message quality",
    yamlContent: `
value: <value>
`,
    target:        &struct{ Value <target-type> }{},
    errorPatterns: []string{"cannot unmarshal", "<value>"},
    description:   "Error for <value> should mention the value",
},
```

## Naming Conventions

### Test Function Names
- Format: `TestInt<Source>To<Target><Scenario>`
- Examples:
  - `TestInt32ToUint32NegativeConversion`
  - `TestInt64ToUint64BoundaryValues`
  - `TestInt32ToUint32ErrorMessageQuality`

### Test Case Names
- Format: `"<source-type> <value> to <target-type> <expectation>"`
- Examples:
  - `"int32 -1 to uint32 should error"`
  - `"int32 2147483647 to uint32 boundary"`
  - `"int32 minimum -2147483648 to uint32"`

## Summary

The int32 test pattern provides a comprehensive, structured approach to testing integer type conversions with:

1. **Consistent structure** across all test functions
2. **Clear separation of concerns** (basic conversion, nested structures, formats, boundaries, error quality)
3. **Verifiable error messages** through pattern matching
4. **Scalable template** that can be adapted for int64 and other integer types

The pattern emphasizes both correctness (does it error when it should?) and quality (are the error messages useful?).
