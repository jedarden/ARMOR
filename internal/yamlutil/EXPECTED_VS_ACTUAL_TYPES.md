# yaml.TypeError Expected vs Actual Type Patterns

## Overview

This document catalogs common expected vs actual type patterns found in `yaml.TypeError` messages in the ARMOR codebase. These patterns represent the most frequent type mismatches encountered during YAML unmarshaling.

## Type Mismatch Categories

### 1. String to Primitive Type Mismatches

#### String to Integer
**Error**: `cannot unmarshal !!str into int`
- **Scenario**: YAML contains `"123"` but target expects `int`
- **Common in**: Configuration files with numeric values
- **Examples**:
  - `port: "8080"` → expects `int`
  - `timeout: "30"` → expects `int`
  - `replicas: "3"` → expects `int`

**Variants**:
- `cannot unmarshal !!str into int8`
- `cannot unmarshal !!str into int16`
- `cannot unmarshal !!str into int32`
- `cannot unmarshal !!str into int64`
- `cannot unmarshal !!str into uint`
- `cannot unmarshal !!str into uint8`
- `cannot unmarshal !!str into uint16`
- `cannot unmarshal !!str into uint32`
- `cannot unmarshal !!str into uint64`

#### String to Boolean
**Error**: `cannot unmarshal !!str into bool`
- **Scenario**: YAML contains `"true"` but target expects `bool`
- **Common in**: Feature flags and boolean configuration
- **Examples**:
  - `debug: "true"` → expects `bool`
  - `enabled: "false"` → expects `bool`
  - `verbose: "yes"` → expects `bool`

#### String to Float
**Error**: `cannot unmarshal !!str into float64`
- **Scenario**: YAML contains `"3.14"` but target expects `float64`
- **Common in**: Numeric configuration values
- **Examples**:
  - `rate: "0.5"` → expects `float64`
  - `ratio: "1.5"` → expects `float32`
  - `timeout: "1.5"` → expects `float64`

### 2. Sequence/Array Type Mismatches

#### String to Array
**Error**: `cannot unmarshal !!str into []string`
- **Scenario**: YAML contains single string but target expects array
- **Common in**: List configuration fields
- **Examples**:
  - `hosts: "localhost"` → expects `[]string`
  - `tags: "production"` → expects `[]string`
  - `allowed_ips: "192.168.1.1"` → expects `[]string`

#### Sequence to Wrong Array Type
**Error**: `cannot unmarshal !!seq into []int`
- **Scenario**: YAML contains array but wrong element type
- **Examples**:
  - `ports: ["8080", "8081"]` → expects `[]int`
  - `counts: ["1", "2", "3"]` → expects `[]int`

#### Map to Array
**Error**: `cannot unmarshal !!map into []string`
- **Scenario**: YAML contains object but target expects array
- **Examples**:
  - `servers: {host: localhost}` → expects `[]string`

#### Array to Array Mismatch
**Error**: `cannot unmarshal !!seq into [][]string`
- **Scenario**: Nested array type mismatch
- **Examples**:
  - `matrix: [1, 2, 3]` → expects `[][]string`

### 3. Map/Object Type Mismatches

#### String to Map
**Error**: `cannot unmarshal !!str into map[string]interface{}`
- **Scenario**: YAML contains string but target expects map
- **Examples**:
  - `metadata: "value"` → expects `map[string]interface{}`

#### Sequence to Map
**Error**: `cannot unmarshal !!seq into map[string]string`
- **Scenario**: YAML contains array but target expects map
- **Examples**:
  - `labels: [key1, key2]` → expects `map[string]string`

#### Integer to Map
**Error**: `cannot unmarshal !!int into map[string]int`
- **Scenario**: YAML contains number but target expects map
- **Examples**:
  - `counts: 5` → expects `map[string]int`

### 4. Boolean Type Mismatches

#### Boolean to String
**Error**: `cannot unmarshal !!bool into string`
- **Scenario**: YAML contains boolean but target expects string
- **Examples**:
  - `name: true` → expects `string`
  - `description: false` → expects `string`

#### Boolean to Integer
**Error**: `cannot unmarshal !!bool into int`
- **Scenario**: YAML contains boolean but target expects integer
- **Examples**:
  - `count: true` → expects `int`
  - `port: false` → expects `int`

#### String to Boolean (Reversed)
**Error**: `cannot unmarshal !!str into bool`
- **Scenario**: String representation of boolean
- **Examples**:
  - `enabled: "true"` → expects `bool`
  - `active: "yes"` → expects `bool`

### 5. Numeric Type Mismatches

#### Integer to Float
**Error**: `cannot unmarshal !!int into float64`
- **Scenario**: Integer provided but float expected
- **Examples**:
  - `rate: 5` → expects `float64`
  - `ratio: 3` → expects `float32`

#### Float to Integer
**Error**: `cannot unmarshal !!float into int`
- **Scenario**: Float provided but integer expected
- **Examples**:
  - `count: 3.5` → expects `int`
  - `port: 8080.0` → expects `int`

#### String to Float
**Error**: `cannot unmarshal !!str into float64`
- **Scenario**: String number but float expected
- **Examples**:
  - `rate: "0.5"` → expects `float64`
  - `price: "99.99"` → expects `float64`

### 6. Null Type Mismatches

#### Null to String
**Error**: `cannot unmarshal !!null into string`
- **Scenario**: Null value but string expected
- **Examples**:
  - `name: null` → expects `string`

#### Null to Primitive
**Error**: `cannot unmarshal !!null into int`
- **Scenario**: Null value but primitive expected
- **Examples**:
  - `count: null` → expects `int`
  - `enabled: null` → expects `bool`

### 7. Struct Type Mismatches

#### Map to Struct
**Error**: `cannot unmarshal !!map into struct`
- **Scenario**: YAML object incompatible with struct fields
- **Examples**:
  - YAML: `{name: "test", value: "data"}`
  - Struct: `{Name string; Count int}`

#### String to Struct
**Error**: `cannot unmarshal !!str into struct`
- **Scenario**: String value but struct expected
- **Examples**:
  - `config: "value"` → expects `struct`

### 8. Pointer Type Mismatches

#### String to Pointer
**Error**: `cannot unmarshal !!str into *string`
- **Scenario**: String value but pointer to string expected
- **Examples**:
  - `name: "test"` → expects `*string`

#### Value to Pointer
**Error**: `cannot unmarshal !!int into *int`
- **Scenario**: Non-pointer value but pointer expected
- **Examples**:
  - `count: 5` → expects `*int`

### 9. Interface Type Mismatches

#### Wrong Type to Interface
**Error**: `cannot unmarshal !!str into interface{}`
- **Scenario**: Type incompatible with interface constraint
- **Examples**:
  - Type constraint requires specific type but string provided

#### Map to Wrong Interface
**Error**: `cannot unmarshal !!map into io.Reader`
- **Scenario**: Map provided but specific interface expected
- **Examples**:
  - `reader: {key: value}` → expects `io.Reader`

### 10. Complex Type Mismatches

#### Nested Array Mismatches
**Error**: `cannot unmarshal !!seq into [][]string`
- **Scenario**: Flat array but nested array expected
- **Examples**:
  - `matrix: [1, 2, 3]` → expects `[][]string`

#### Map Value Type Mismatch
**Error**: `cannot unmarshal !!seq into map[string]int`
- **Scenario**: Array provided but map with specific value type expected
- **Examples**:
  - `counts: [1, 2, 3]` → expects `map[string]int`

#### Array Size Mismatch
**Error**: `cannot unmarshal !!seq into [5]int`
- **Scenario**: Variable-length array but fixed-size expected
- **Examples**:
  - `values: [1, 2]` → expects `[5]int`

## Common Scenarios

### Configuration File Errors

1. **Port Configuration**
   ```yaml
   # ERROR: String instead of integer
   server:
     port: "8080"  # should be: 8080
   ```
   **Error**: `cannot unmarshal !!str into int`

2. **Boolean Flags**
   ```yaml
   # ERROR: String instead of boolean
   features:
     enabled: "true"  # should be: true
   ```
   **Error**: `cannot unmarshal !!str into bool`

3. **Array Fields**
   ```yaml
   # ERROR: String instead of array
   hosts: "localhost"  # should be: ["localhost"]
   ```
   **Error**: `cannot unmarshal !!str into []string`

### Data Structure Errors

1. **Nested Configuration**
   ```yaml
   # ERROR: Type mismatch in nested field
   database:
     timeout: "30"  # should be: 30
   ```
   **Error**: `cannot unmarshal !!str into int`

2. **Map vs Array**
   ```yaml
   # ERROR: Array instead of map
   labels: [key1, key2]  # should be: {key1: value1, key2: value2}
   ```
   **Error**: `cannot unmarshal !!seq into map[string]string`

3. **Null Values**
   ```yaml
   # ERROR: Null instead of required value
   name: null  # should be: "value"
   ```
   **Error**: `cannot unmarshal !!null into string`

## Type Compatibility Matrix

| Actual \ Expected | string | int | bool | []string | map | struct |
|-------------------|--------|-----|------|----------|-----|---------|
| !!str | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ |
| !!int | ✗ | ✓ | ✗ | ✗ | ✗ | ✗ |
| !!float | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ |
| !!bool | ✗ | ✗ | ✓ | ✗ | ✗ | ✗ |
| !!seq | ✗ | ✗ | ✗ | ✓ | ✗ | ✗ |
| !!map | ✗ | ✗ | ✗ | ✗ | ✓ | ✓ |
| !!null | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ |

Legend:
- ✓ = Compatible (no error)
- ✗ = Incompatible (produces TypeError)

## Error Detection and Resolution

### Detection Patterns

1. **Line Number Extraction**
   - Pattern: `line (\d+)`
   - Used to locate the problematic line in YAML files

2. **Type Tag Identification**
   - Pattern: `!!(\w+)`
   - Identifies the actual YAML type encountered

3. **Expected Type Extraction**
   - Pattern: `into (\S+)`
   - Identifies the target Go type

4. **Value Extraction**
   - Pattern: `` `([^`]+)` ``
   - Extracts the actual value when provided

### Resolution Strategies

1. **For String to Primitive Errors**
   - Remove quotes from numeric values
   - Convert `true`/`false` to unquoted booleans
   - Ensure numeric values don't contain extra characters

2. **For Array/Sequence Errors**
   - Convert single values to arrays: `value` → `[value]`
   - Ensure array elements match expected types
   - Check for proper YAML array syntax

3. **For Map/Object Errors**
   - Verify map syntax with proper key-value pairs
   - Ensure keys are strings
   - Check value types match expected map value types

4. **For Null Value Errors**
   - Replace `null` with appropriate default values
   - Remove `null` for required fields
   - Use empty values (`""`, `[]`, `{}`) where appropriate

## ARMOR-Specific Patterns

### Type Normalization

ARMOR includes sophisticated type normalization:

```go
// YAML Type Tag Normalizations
"!!str"   → "string"
"!!int"   → "integer"
"!!float" → "number"
"!!bool"  → "boolean"
"!!seq"   → "array"
"!!map"   → "object"
"!!null"  → "null"
```

```go
// Go Type Normalizations
"int8", "int16", "int32", "int64" → "integer"
"uint8", "uint16", "uint32", "uint64" → "unsigned integer"
"float32", "float64" → "number"
"[]string" → "array of string"
"*string" → "pointer to string"
```

### Enhanced Error Messages

ARMOR enhances TypeError messages with:

1. **Field Path Information**
   - `server.port type mismatch`
   - `items[0].name cannot unmarshal`

2. **Line and Column Details**
   - `line 10, column 5: cannot unmarshal`
   - Precise location information

3. **Contextual Information**
   - `value: 'actual_value'`
   - `expected: int, got: string`

4. **Human-Readable Types**
   - `expected integer, got string`
   - `expected array of string, got string`

## Testing and Validation

### Test Coverage

The ARMOR codebase includes comprehensive test coverage:

1. **Format Pattern Tests** (`parse_type_error_test.go`)
   - Tests all 7+ error message formats
   - Validates line/column extraction
   - Confirms type tag parsing

2. **Helper Function Tests** (`typeerror_helpers_test.go`)
   - Tests type normalization
   - Validates field path extraction
   - Confirms error message formatting

3. **Integration Tests** (`yaml_typeerror_test.go`)
   - Tests type assertions across codebase
   - Validates error information preservation
   - Confirms integration with parsers/validators

### Fixture Usage

The test fixtures (`typeerror_test_fixtures.go`) provide:

1. **Format-Specific Fixtures**
   - Grouped by error message pattern
   - Cover all identified formats

2. **Type-Specific Fixtures**
   - Grouped by YAML type tag
   - Grouped by target Go type

3. **Real-World Scenarios**
   - Common configuration errors
   - Typical data structure issues

## Summary

The `yaml.TypeError` expected vs actual type patterns in ARMOR demonstrate:

1. **High Frequency Categories**
   - String to primitive types (most common)
   - Array/sequence mismatches (very common)
   - Boolean vs string confusion (common)

2. **Complex Scenarios**
   - Nested type mismatches
   - Pointer type handling
   - Interface type constraints

3. **Comprehensive Coverage**
   - All YAML type tags covered
   - All common Go types covered
   - Real-world scenarios represented

The ARMOR codebase includes robust parsing, normalization, and error handling for these patterns, making it easier to diagnose and resolve type mismatches in YAML configuration files.
