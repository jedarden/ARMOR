# yaml.TypeError Message Format Documentation

## Overview

`yaml.TypeError` from `gopkg.in/yaml.v3` produces error messages in various formats depending on the context and nature of the type mismatch. This document catalogs all identified message formats, their patterns, and provides examples.

## Error Message Structure

### Basic Format Components

yaml.TypeError error messages typically contain the following components:

1. **Line/Column Location** - Where the error occurred in the YAML file
2. **Error Type** - The kind of error (usually "cannot unmarshal")
3. **Actual Type** - What was found in the YAML (with YAML type tags)
4. **Expected Type** - What Go type was expected
5. **Field Path** (optional) - Dot-notation path to the field
6. **Value** (optional) - The actual value that caused the error

## Identified Message Format Patterns

### Pattern 1: Basic Type Tag Format

**Format:**
```
line <N>: cannot unmarshal !!<tag> into <type>
```

**Examples:**
```
line 10: cannot unmarshal !!str into int
line 15: cannot unmarshal !!seq into []string
line 20: cannot unmarshal !!map into struct
```

**Components:**
- `line <N>`: Line number (1-indexed)
- `cannot unmarshal`: Error type indicator
- `!!<tag>`: YAML type tag (!!str, !!int, !!seq, !!map, !!bool, !!null, !!timestamp)
- `into <type>`: Expected Go type

**YAML Type Tags:**
- `!!str` - String type
- `!!int` - Integer type
- `!!float` - Floating-point number
- `!!bool` - Boolean type
- `!!seq` - Sequence/array type
- `!!map` - Mapping/object type
- `!!null` - Null/nil value
- `!!timestamp` - Timestamp value

### Pattern 2: YAML Prefix Format

**Format:**
```
yaml: line <N>: cannot unmarshal !!<tag> into <type>
```

**Examples:**
```
yaml: line 10: cannot unmarshal !!str into int
yaml: line 15: cannot unmarshal !!seq into []string
yaml: line 20: cannot unmarshal !!map into map[string]int
```

**Components:**
- `yaml:`: YAML parser prefix
- `line <N>`: Line number
- `cannot unmarshal !!<tag> into <type>`: Error description

### Pattern 3: Column Information Format

**Format:**
```
line <N>, column <C>: cannot unmarshal !!<tag> into <type>
```

**Examples:**
```
line 10, column 5: cannot unmarshal !!str into int
line 15, column 8: cannot unmarshal !!seq into []string
```

**Components:**
- `line <N>, column <C>`: Precise location (1-indexed)
- `cannot unmarshal !!<tag> into <type>`: Error description

### Pattern 4: Field Path Format

**Format:**
```
field <path> type mismatch: expected <type>, got <type>
```

**Examples:**
```
field server.port type mismatch: expected int, got string
field items[0].name type mismatch: expected string, got int
field metadata.annotations.version type mismatch: expected string, got int
```

**Components:**
- `field <path>`: Dot-notation field path (supports array indexing with `[0]`)
- `type mismatch`: Error type indicator
- `expected <type>`: Expected Go type
- `got <type>`: Actual type found

### Pattern 5: Field Path with Unmarshal Format

**Format:**
```
field <path> cannot unmarshal !!<tag> into <type>
```

**Examples:**
```
field items[0].name cannot unmarshal !!str into int
field config.enabled cannot unmarshal !!bool into string
```

**Components:**
- `field <path>`: Field path
- `cannot unmarshal !!<tag> into <type>`: Error description

### Pattern 6: Expected/Got Format

**Format:**
```
expected <type>, got <type>
```

**Examples:**
```
expected int, got string
expected bool, got string
expected []string, got int
```

**Components:**
- `expected <type>`: Expected type
- `got <type>`: Actual type

### Pattern 7: Cannot Convert Format

**Format:**
```
<type> cannot be converted to <type>
```

**Examples:**
```
string cannot be converted to int
int cannot be converted to bool
```

**Components:**
- `<type>`: Source type
- `cannot be converted to`: Conversion error indicator
- `<type>`: Target type

### Pattern 8: Value Format

**Format:**
```
cannot unmarshal !!<tag> `<value>` into <type>
```

**Examples:**
```
cannot unmarshal !!str `hello` into int
cannot unmarshal !!str `123abc` into float64
```

**Components:**
- `!!<tag>`: YAML type tag
- `` `<value>` ``: Actual value (backtick-quoted)
- `into <type>`: Expected type

### Pattern 9: Complex Error Messages

**Format:**
```
error converting YAML to JSON: yaml: line <N>: cannot unmarshal !!<tag> into <type>
```

**Examples:**
```
error converting YAML to JSON: yaml: line 10: cannot unmarshal !!str into int
error converting YAML to JSON: yaml: line 15: cannot unmarshal !!seq into []string
```

**Components:**
- `error converting YAML to JSON`: Context prefix
- `yaml: line <N>`: YAML parser location
- `cannot unmarshal !!<tag> into <type>`: Error description

### Pattern 10: Multiple Errors Format

**Format:**
```
yaml: unmarshal errors:
  line <N>: cannot unmarshal !!<tag> into <type>
  line <N>: cannot unmarshal !!<tag> into <type>
```

**Examples:**
```yaml
yaml: unmarshal errors:
  line 5: cannot unmarshal !!seq into []string
  line 10: cannot unmarshal !!str into int
```

**Components:**
- `yaml: unmarshal errors:`: Multiple error indicator
- Indented lines: Individual errors with line numbers

## Go Type Representations

### Basic Types
- `int`, `int8`, `int16`, `int32`, `int64` - Integer types
- `uint`, `uint8`, `uint16`, `uint32`, `uint64` - Unsigned integer types
- `float32`, `float64` - Floating-point types
- `string` - String type
- `bool` - Boolean type

### Complex Types
- `[]<type>` - Array/slice types (e.g., `[]string`, `[]int`)
- `[<N>]<type>` - Fixed-size arrays (e.g., `[10]string`)
- `map[<key>]<value>` - Map types (e.g., `map[string]int`)
- `*<type>` - Pointer types (e.g., `*string`, `*int`)
- `chan <type>` - Channel types (e.g., `chan int`)
- `<-chan <type>` - Receive-only channel (e.g., `<-chan string`)
- `chan<- <type>` - Send-only channel (e.g., `chan<- int`)
- `interface{}` - Interface type
- `struct` - Struct type

### Package-Qualified Types
- `<package>.<type>` - Types from other packages (e.g., `time.Time`, `http.Response`)

## Line Number Formats

### Identified Line Number Patterns:
1. `line <N>:` - Standard format
2. `line <N>, column <C>:` - With column
3. `yaml: line <N>:` - With YAML prefix
4. `at line <N>` - Alternative format
5. `error at line <N>:` - Error context format
6. `<N>:` - Number only at start

## Type Normalization

### YAML Tag to Type Mapping:
- `!!str` → "string"
- `!!int` → "integer"
- `!!float` → "float"
- `!!bool` → "boolean"
- `!!seq` → "array"
- `!!map` → "object"
- `!!null` → "null"
- `!!timestamp` → "timestamp"

### Go Type Normalization:
- Integer types (`int`, `int8`, etc.) → "integer"
- Unsigned types (`uint`, `uint8`, etc.) → "integer" (or "unsigned integer")
- Float types (`float32`, `float64`) → "float"
- Boolean types (`bool`) → "boolean"
- Array types (`[]string`, `[10]int`) → "array of <element type>"
- Pointer types (`*string`) → "pointer to <type>"
- Map types (`map[string]int`) → "map[key_type]value_type"
- Channel types (`chan int`) → "channel of <type>"
- Interface types (`interface{}`) → "interface"
- Package-qualified types (`time.Time`) → "<Type>" (package stripped)

## Field Path Patterns

### Valid Field Path Formats:
1. **Simple fields**: `server.port`, `config.name`, `metadata.version`
2. **Nested fields**: `metadata.annotations.version`, `config.server.port`
3. **Array indexing**: `items[0].name`, `values[5].type`, `data[0][1].key`
4. **Mixed**: `servers[0].ports[0].number`, `metadata.items[0].key`

### Field Path Extraction Patterns:
- `field <path> type mismatch:` - Type mismatch format
- `field <path> cannot unmarshal` - Unmarshal format
- `at field <path>` - Alternative format
- `path: <path>` - Path prefix format
- `in <path> field` - Context format

## Edge Cases Identified

### 1. Empty Error Strings
Empty error messages may occur in some edge cases.

### 2. No Line Number
Some errors don't include line number information:
```
cannot unmarshal !!map into struct
```

### 3. Quoted Types
Types may appear with quotes:
```
cannot unmarshal "string" into "int"
```

### 4. Complex Go Types
Errors involving complex Go type signatures:
```
cannot unmarshal !!map into map[string]interface{}
cannot unmarshal !!seq into []CustomType
cannot unmarshal !!str into chan int
```

### 5. Multiple Type Tags
Some errors may contain multiple YAML type tags.

### 6. Package-Qualified Types
Types from external packages:
```
cannot unmarshal !!str into time.Time
cannot unmarshal !!seq into []http.Response
```

### 7. Array Size Specifications
Fixed-size array types:
```
cannot unmarshal !!seq into [10]string
cannot unmarshal !!seq into [5]int
```

### 8. Pointer Dereferencing
Multiple pointer levels:
```
cannot unmarshal !!str into **string
cannot unmarshal !!str into *CustomType
```

### 9. Channel Directionality
Different channel types:
```
cannot unmarshal !!str into chan int
cannot unmarshal !!str into <-chan string
cannot unmarshal !!str into chan<- bool
```

### 10. Interface Type Errors
Special handling for interface{}:
```
cannot unmarshal !!str into interface{}
cannot unmarshal !!map into interface{}
```

## Test Coverage Categories

### 1. Basic Type Errors
- String to integer conversion
- Integer to string conversion
- Boolean type mismatches
- Float type mismatches

### 2. Array/Sequence Errors
- Scalar to array conversion
- Array to scalar conversion
- Array element type mismatches
- Nested array errors

### 3. Map/Object Errors
- Scalar to map conversion
- Map to scalar conversion
- Map key/value type mismatches

### 4. Complex Type Errors
- Pointer type errors
- Channel type errors
- Interface type errors
- Package-qualified type errors

### 5. Location Information
- Line number extraction
- Column number extraction
- Combined line and column
- Missing location information

### 6. Field Path Errors
- Simple field paths
- Nested field paths
- Array-indexed field paths
- Mixed field paths

## Usage in ARMOR Codebase

### Files Using yaml.TypeError:
1. `internal/yamlutil/errors.go` - Error formatting and parsing
2. `internal/yamlutil/typeerror_helpers.go` - Type error helper functions
3. `internal/yamlutil/schema.go` - Schema validation with type assertions
4. `internal/yamlutil/validator.go` - Validation with type assertions
5. `internal/yamlutil/parser.go` - YAML parsing with type assertions
6. `internal/yamlutil/syntax_validator.go` - Syntax validation with type assertions
7. `internal/yamlutil/future.go` - Future features with type assertions

### Common Pattern:
```go
if typeErr, ok := err.(*yaml.TypeError); ok {
    // Handle type error with detailed information
    for _, errMsg := range typeErr.Errors {
        // Parse individual error message
        detail := ParseTypeErrorString(errMsg)
        // Use extracted information
    }
}
```

## Related Functions

### Parsing Functions:
- `ParseTypeErrorString()` - Parse individual error strings
- `ParseTypeError()` - Parse full yaml.TypeError
- `ExtractFieldPathFromTypeError()` - Extract field paths
- `GetLineNumberFromTypeError()` - Extract line numbers

### Formatting Functions:
- `FormatTypeErrorDetail()` - Format individual error details
- `FormatTypeErrorSummary()` - Format error summaries
- `FormatExpectedVsActual()` - Format type comparisons
- `FormatYAMLErrorMessage()` - Format comprehensive error messages

### Helper Functions:
- `normalizeYAMLType()` - Normalize type names
- `extractTypeMismatchInfo()` - Extract type mismatch information
- `extractLineFromTypeError()` - Extract line numbers
- `extractColumnFromTypeError()` - Extract column numbers

## Recommendations for Handling

### 1. Always Check Type Assertions
```go
if typeErr, ok := err.(*yaml.TypeError); ok {
    // Handle TypeError
}
```

### 2. Process All Errors in TypeError.Errors
yaml.TypeError can contain multiple errors - always iterate through them.

### 3. Use Parsing Functions
Use the provided parsing functions instead of manual string parsing:
- `ParseTypeErrorString()` for individual errors
- `ParseTypeError()` for complete TypeError objects

### 4. Handle Edge Cases
- Empty error strings
- Missing line numbers
- Complex Go types
- Package-qualified types

### 5. Provide Context
Always include file path and field context when reporting errors.

## See Also

- `internal/yamlutil/errors.go` - Error type definitions
- `internal/yamlutil/typeerror_helpers.go` - Helper functions
- `internal/yamlutil/typeerror_helpers_test.go` - Comprehensive tests
- `test_data/typeerror_fixtures.yaml` - Test fixtures
