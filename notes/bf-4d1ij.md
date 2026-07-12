# Int32 Test Pattern Structure Analysis

## Task: bf-4d1ij
Analyze int32 test pattern structure to understand proper structure for boundary values and error message quality tests for int64 implementation.

---

## Test Case Structure

### Standard Test Case Object

All test cases use the following struct structure:

```go
tests := []struct {
    name          string    // Test case name for t.Run()
    yamlContent   string    // YAML content to parse (triple-quoted template)
    target        interface{} // Target struct to unmarshal into
    shouldError   bool      // Whether parsing should produce an error
    description   string    // Human-readable description of what's being tested
    expectedInMsg []string  // OPTIONAL: Patterns that should appear in error message
}{
    // test cases...
}
```

### Field Descriptions

| Field | Type | Required | Purpose |
|-------|------|----------|---------|
| `name` | string | Yes | Test identifier used in `t.Run(name, ...)` |
| `yamlContent` | string | Yes | YAML input to parse, typically triple-quoted with template values |
| `target` | interface{} | Yes | Pointer to target struct (e.g., `&struct{ Value uint32 }{}`) |
| `shouldError` | bool | Yes | Whether error is expected (true) or success (false) |
| `description` | string | Yes | Human-readable explanation of test intent |
| `expectedInMsg` | []string | No | Error message patterns to verify (only for error tests) |

---

## Test Function Patterns

### 1. Basic Negative Conversion Test

**Function:** `TestInt32ToUint32NegativeConversion`

Tests the fundamental negative-to-unsigned conversion scenarios.

**Structure:**
- Edge case: `-1` (most common negative)
- Edge case: minimum value (`-2147483648`)
- Additional negative values across the range
- Extreme negative values beyond type range
- Positive control cases (should succeed)

**Example:**
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

### 2. Boundary Values Test

**Function:** `TestInt32ToUint32BoundaryValues`

Tests type boundaries systematically.

**Structure:**
- Negative boundary values (minimum, one above minimum, powers of 2)
- Positive boundary values (zero, uint8/uint16 max, type max)
- Overflow cases (values exceeding type maximum)

**Example:**
```go
{
    name: "int32 minimum -2147483648 to uint32",
    yamlContent: `
value: -2147483648
`,
    target:        &struct{ Value uint32 }{},
    shouldError:   true,
    description:   "Minimum int32 value cannot convert to uint32",
    expectedInMsg: []string{"cannot unmarshal"},
},
```

### 3. Error Message Quality Test

**Function:** `TestInt32ToUint32ErrorMessageQuality`

Specifically validates error message content and helpfulness.

**Structure:**
- Uses `errorPatterns` field (alias for `expectedInMsg`)
- Focuses on key values that should produce informative errors
- Validates pattern matching in error messages

**Example:**
```go
{
    name: "int32 -1 error message quality",
    yamlContent: `
value: -1
`,
    target:        &struct{ Value uint32 }{},
    errorPatterns: []string{"cannot unmarshal", "-1"},
    description:   "Error for -1 should mention the value",
},
```

### 4. Nested Structures Test

**Function:** `TestInt32ToUint32NegativeInNestedStructs`

Tests conversions in complex YAML structures.

**Structure:**
- Nested struct fields
- Arrays/slices
- Maps
- Slices of structs

**Note:** May have `shouldError: false` for some cases due to YAML parser behavior.

### 5. Format Variations Test

**Function:** `TestInt32ToUint32NegativeWithDifferentFormats`

Tests different YAML number representations.

**Structure:**
- Decimal format (`-100.0`)
- Zero-padded (`-00050`)
- Quoted strings (`"-256"`)
- Octal strings (`"-0400"`)
- Hex strings (`"-0x100"`)

---

## Test Execution Pattern

All test functions use the same execution loop:

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

                for _, expected := range tt.expectedInMsg {
                    if !strings.Contains(errMsg, expected) && 
                       !strings.Contains(lowerErrMsg, strings.ToLower(expected)) {
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

---

## Key Differences: Int32 vs Int64

### Value Ranges

| Type | Minimum | Maximum | Positive Limit |
|------|---------|---------|----------------|
| int32 | `-2147483648` | `2147483647` | uint32 max: `4294967295` |
| int64 | `-9223372036854775808` | `9223372036854775807` | uint64 max: `18446744073709551615` |

### Target Struct Types

- **Int32 tests:** `&struct{ Value uint32 }{}`
- **Int64 tests:** `&struct{ Value uint64 }{}`

### Behavior Differences

1. **Decimal format handling:**
   - Int32: `-100.0` → errors
   - Int64: `-100.0` → **succeeds** with value `100` (YAML parser converts)

2. **Overflow handling:**
   - Int32: `4294967296` → errors (exceeds uint32 max)
   - Int64: `18446744073709551616` → **succeeds** silently (YAML parser wraps)

3. **Beyond-minimum values:**
   - Both wrap silently by YAML parser
   - Example: `-9223372036854775809` wraps to `9223372036854775808`

---

## Helper Functions

### containsAny

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

**Purpose:** Check if a string contains any of the given patterns (case-sensitive).

**Usage:** Validates error messages contain expected keywords.

---

## Template for New Int64 Boundary Value Tests

```go
{
    name: "int64 [value] to uint64",
    yamlContent: `
value: [value]
`,
    target:        &struct{ Value uint64 }{},
    shouldError:   [true/false],
    description:   "[Description of what value represents and expected behavior]",
    expectedInMsg: []string{"cannot unmarshal"[, additional patterns]},
},
```

**Example for int64:**
```go
{
    name: "int64 minimum -9223372036854775808 to uint64",
    yamlContent: `
value: -9223372036854775808
`,
    target:        &struct{ Value uint64 }{},
    shouldError:   true,
    description:   "Minimum int64 value cannot convert to uint64",
    expectedInMsg: []string{"cannot unmarshal"},
},
```

---

## Template for New Int64 Error Message Quality Tests

```go
{
    name: "int64 [value] error message quality",
    yamlContent: `
value: [value]
`,
    target:        &struct{ Value uint64 }{},
    errorPatterns: []string{"cannot unmarshal"[, value string]},
    description:   "Error for [value] should [description of expected message content]",
},
```

**Example for int64:**
```go
{
    name: "int64 -9223372036854775808 error message quality",
    yamlContent: `
value: -9223372036854775808
`,
    target:        &struct{ Value uint64 }{},
    errorPatterns: []string{"cannot unmarshal", "-9223372036854775808"},
    description:   "Error for minimum int64 should mention the value",
},
```

---

## Summary

### Key Fields for Boundary Tests:
1. `name` - descriptive identifier
2. `yamlContent` - YAML with value to test
3. `target` - `&struct{ Value uint64 }{}` for int64 tests
4. `shouldError` - true for negatives, false for valid positives
5. `description` - explains what boundary is being tested
6. `expectedInMsg` - `[]string{"cannot unmarshal"}` minimum for errors

### Key Fields for Error Message Quality Tests:
1. `name` - "int64 [value] error message quality"
2. `yamlContent` - YAML with error-causing value
3. `target` - `&struct{ Value uint64 }{}`
4. `errorPatterns` - array of expected patterns (value + "cannot unmarshal")
5. `description` - explains what message should contain

### Pattern Differences Noted:
- Int64 has additional larger negative values to test
- Int64 decimal format succeeds (differs from int32)
- Int64 overflow wraps silently (differs from int32)
- Both use identical test structure and validation logic

---

**Generated for bead bf-4d1ij** - Task: Analyze int32 test pattern structure
