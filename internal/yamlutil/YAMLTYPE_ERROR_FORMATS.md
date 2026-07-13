# yaml.TypeError Message Formats Documentation

## Overview

This document catalogs the various error message formats produced by `yaml.TypeError` in the ARMOR codebase, focusing on patterns found in the `gopkg.in/yaml.v3` library.

## yaml.TypeError Structure

```go
// From gopkg.in/yaml.v3
type TypeError struct {
    Errors []string  // Slice of error strings describing type mismatches
}
```

The `yaml.TypeError` struct contains a slice of error strings, where each string represents a distinct type conversion failure.

## Error Message Format Patterns

### Pattern 1: Basic Line-Based Error
**Format**: `line <number>: cannot unmarshal <actual> into <expected>`

**Examples**:
- `line 10: cannot unmarshal !!str into int`
- `line 15: cannot unmarshal !!seq into []string`
- `line 20: cannot unmarshal !!map into struct`

**Components**:
- `line <number>`: Line number where error occurred
- `cannot unmarshal`: Error indicator
- `<actual>`: Actual YAML type found (with YAML type tag like `!!str`)
- `into`: Separator
- `<expected>`: Expected Go type

### Pattern 2: YAML-Prefixed Line Error
**Format**: `yaml: line <number>: cannot unmarshal <actual> into <expected>`

**Examples**:
- `yaml: line 10: cannot unmarshal !!str into int`
- `yaml: line 15: cannot unmarshal !!seq into []string`

**Components**:
- `yaml:`: Prefix indicating YAML parsing context
- Rest same as Pattern 1

### Pattern 3: Line and Column Error
**Format**: `line <number>, column <number>: cannot unmarshal <actual> into <expected>`

**Examples**:
- `line 10, column 5: cannot unmarshal !!str into int`

**Components**:
- `line <number>`: Line number
- `column <number>`: Column number within the line
- Rest same as Pattern 1

### Pattern 4: Error Context Prefix
**Format**: `error at line <number>: cannot unmarshal <actual> into <expected>`

**Examples**:
- `error at line 25: cannot unmarshal !!str into int`

**Components**:
- `error at`: Context prefix
- `line <number>`: Line number
- Rest same as Pattern 1

### Pattern 5: Nested Context Error
**Format**: `error converting YAML to JSON: yaml: line <number>: cannot unmarshal <actual> into <expected>`

**Examples**:
- `error converting YAML to JSON: yaml: line 30: cannot unmarshal !!str into int`

**Components**:
- `error converting YAML to JSON`: Nested context
- `yaml:`: YAML parsing context
- Rest same as Pattern 2

### Pattern 6: Value-Specific Error
**Format**: `yaml: line <number>: cannot unmarshal !!str <value> into <expected>`

**Examples**:
- `yaml: line 10: cannot unmarshal !!str `hello` into int`

**Components**:
- Same as Pattern 2
- `<value>`: The actual value that caused the error (backtick-quoted)

### Pattern 7: Multi-Error Format
**Format**: `yaml: unmarshal errors:\n  <error1>\n  <error2>...`

**Examples**:
```yaml
yaml: unmarshal errors:
  line 5: cannot unmarshal !!seq into []string
  line 10: cannot unmarshal !!str into int
```

**Components**:
- `yaml: unmarshal errors:`: Header
- Indented individual errors (one per line)

## YAML Type Tags

The error messages use YAML type tags to indicate actual types:

| Tag | Type | Description |
|-----|------|-------------|
| `!!str` | string | String/scalar text |
| `!!int` | integer | Integer number |
| `!!float` | float | Floating-point number |
| `!!bool` | boolean | Boolean value |
| `!!seq` | sequence | Array/sequence |
| `!!map` | mapping | Object/dictionary |
| `!!null` | null | Null/nil value |
| `!!timestamp` | timestamp | Date/time value |

## Common Go Types in Errors

Expected types in errors are Go type representations:

### Basic Types
- `int`, `int8`, `int16`, `int32`, `int64`
- `uint`, `uint8`, `uint16`, `uint32`, `uint64`
- `float32`, `float64`
- `string`
- `bool`

### Composite Types
- `[]string` - Array/slice of strings
- `[]int` - Array/slice of integers
- `map[string]interface{}` - Map with string keys
- `map[string]int` - Map with string keys to int values
- `struct` - Struct type

### Pointer Types
- `*string` - Pointer to string
- `*int` - Pointer to integer
- `*CustomType` - Pointer to custom type

### Interface Types
- `interface{}` - Empty interface (any type)
- `io.Reader` - Interface with specific methods

## Error Message Analysis in ARMOR

### Helper Functions in typeerror_helpers.go

The ARMOR codebase includes comprehensive parsing functions:

1. **`ParseTypeErrorString`**: Main entry point for parsing error strings
2. **`extractLineFromTypeError`**: Extracts line numbers with multiple patterns
3. **`extractColumnFromTypeError`**: Extracts column information
4. **`extractTypeMismatchInfo`**: Parses field path and type information
5. **`normalizeYAMLType`**: Normalizes type tags to readable names

### Pattern Matching Regex Patterns

The code uses sophisticated regex patterns to extract information:

```go
// Line extraction patterns (in order of specificity)
`line\s+(\d+),\s*column`      // "line 10, column 5:"
`yaml:\s+line\s+(\d+):`       // "yaml: line 15:"
`line\s+(\d+):`                // "line 10:"
`at\s+line\s+(\d+)`            // "at line 25"
`error\s+at\s+line\s+(\d+)`    // "error at line 30"
`^(\d+):`                      // "10:" at start
```

## Type Normalization

ARMOR normalizes type descriptions for consistency:

### YAML Type Tag Conversions
- `!!str` → `string`
- `!!int` → `integer`
- `!!float` → `number`
- `!!bool` → `boolean`
- `!!seq` → `array`
- `!!map` → `object`
- `!!null` → `null`

### Go Type Normalizations
- Array types: `[]string` → `array of string`
- Pointer types: `*string` → `pointer to string`
- Map types: `map[string]int` → `map[string]integer`
- Integer variants: `int8`, `int16`, `int32`, `int64` → `integer`
- Unsigned variants: `uint8`, `uint16`, `uint32`, `uint64` → `unsigned integer`

## Field Path Patterns

Error messages may include field path information:

### Simple Field Paths
- `field server.port type mismatch` → Field: `server.port`
- `field items[0].name cannot unmarshal` → Field: `items[0].name`

### Array Indexing
- `items[0].name` - First item in array
- `servers[1].port` - Second item in array
- `data.values[2]` - Third item in nested array

### Nested Paths
- `server.port` - Simple nested field
- `database.connection.timeout` - Multi-level nesting
- `metadata.annotations["key"]` - Map-style access

## Edge Cases and Special Patterns

### 1. Quoted Values
When values are included in errors, they may be quoted:
- `cannot unmarshal !!str `hello` into int`
- `cannot unmarshal !!str "world" into bool`

### 2. Type Inference
When actual types cannot be determined from tags, the code infers from values:
- `"true"` → boolean
- `"123"` → number  
- `"text"` → string

### 3. Complex Go Types
Errors involving complex types:
- `cannot unmarshal !!seq into []map[string]interface{}`
- `cannot unmarshal !!map into struct { Name string; Age int }`

### 4. Interface Types
- `cannot unmarshal !!str into interface{}`
- `cannot unmarshal !!map into io.Reader`

## Related Files in ARMOR

- `internal/yamlutil/errors.go` - Error type definitions
- `internal/yamlutil/typeerror_helpers.go` - TypeError parsing utilities
- `internal/yamlutil/validator.go` - YAML validation with TypeError handling
- `internal/yamlutil/yaml_typeerror_test.go` - Type assertion tests
- `internal/yamlutil/parse_type_error_test.go` - Error parsing tests
- `internal/yamlutil/typeerror_helpers_test.go` - Helper function tests

## Summary

The `yaml.TypeError` in ARMOR produces structured error messages with these key characteristics:

1. **Multiple format patterns** - 7+ distinct error message formats
2. **Line/column information** - Precise location data when available
3. **YAML type tags** - Standard YAML type indicators for actual values
4. **Go type names** - Expected types represented as Go type names
5. **Field paths** - Optional field path information for nested structures
6. **Multi-error support** - Can represent multiple type errors in one TypeError

The ARMOR codebase includes comprehensive parsing and normalization of these error formats for enhanced debugging and error reporting.
