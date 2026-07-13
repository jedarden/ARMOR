# yaml.TypeError Call Site Audit

## Executive Summary
This audit catalogs all error handling locations in the `internal/yamlutil` package where `yaml.v3` parser errors could return `*yaml.TypeError`. It identifies which sites have type assertions and which are missing them.

## Background
`yaml.v3` parser returns `*yaml.TypeError` when type mismatches occur during unmarshaling. Proper type assertions are needed to extract the detailed error information from the `Errors` field, which contains a slice of error strings.

## Complete Catalog of yaml.Unmarshal Call Sites

### Sites WITH yaml.TypeError Type Assertions ✅

#### 1. `parser.go` - Line 67 (ParseFile method)
- **Location**: `parser.go:67` - `yaml.Unmarshal(content, data)`
- **Type assertion**: Line 109
- **Code**: `if typeErr, ok := err.(*yaml.TypeError); ok {`
- **Status**: ✅ COMPLETE

#### 2. `parser.go` - Line 162 (ParseFileToMap method)
- **Location**: `parser.go:162` - `yaml.Unmarshal(content, &data)`
- **Type assertion**: Line 167
- **Code**: `if typeErr, ok := err.(*yaml.TypeError); ok {`
- **Status**: ✅ COMPLETE

#### 3. `parser.go` - Line 348 (ParseYAML function)
- **Location**: `parser.go:348` - `yaml.Unmarshal(content, &data)`
- **Type assertion**: Line 397
- **Code**: `if typeErr, ok := err.(*yaml.TypeError); ok {`
- **Status**: ✅ COMPLETE

#### 4. `validator.go` - Line 137 (ValidateStringWithPath)
- **Location**: `validator.go:137` - `yaml.Unmarshal([]byte(yamlContent), &node)`
- **Type assertion**: Line 269 in `parseYAMLError` method
- **Code**: `if typeErr, ok := err.(*yaml.TypeError); ok {`
- **Status**: ✅ COMPLETE

#### 5. `syntax_validator.go` - Line 386 (ValidateSyntax)
- **Location**: `syntax_validator.go:386` - `yaml.Unmarshal([]byte(yamlContent), &node)`
- **Type assertion**: Line 1032 in `convertParseError` method
- **Code**: `if typeErr, ok := err.(*yaml.TypeError); ok {`
- **Status**: ✅ COMPLETE

#### 6. `future.go` - Line 93 (ParseStreamToMap)
- **Location**: `future.go:93` - `yaml.Unmarshal(content, &data)`
- **Type assertion**: Line 103
- **Code**: `if typeErr, ok := err.(*yaml.TypeError); ok {`
- **Status**: ✅ COMPLETE (Note: future.go is marked `//go:build ignore`)

### Sites MISSING yaml.TypeError Type Assertions ❌

#### 1. `schema.go` - Line 288 (ValidateSchemaFile)
- **Location**: `schema.go:288` - `yaml.Unmarshal(content, &data)`
- **Current handling**: Has type assertions for `*ParseError`, `*SyntaxError`, `*TypeMismatchError`, `*StructureError`, and `YAMLError` interface
- **Missing**: No `*yaml.TypeError` type assertion
- **Impact**: When yaml.v3 returns a TypeError, it falls through to the generic YAMLError interface check or generic fallback
- **Recommendation**: Add `*yaml.TypeError` type assertion before the `YAMLError` interface check
- **Status**: ❌ NEEDS TYPE ASSERTION

#### 2. `schema.go` - Line 703 (LoadSchemaFromFile)
- **Location**: `schema.go:703` - `yaml.Unmarshal(content, &data)`
- **Current handling**: Generic error wrapping only
- **Missing**: No specific type assertions for any error type
- **Impact**: TypeError details are lost in generic error message
- **Recommendation**: Add full error handling pattern including `*yaml.TypeError` type assertion
- **Status**: ❌ NEEDS TYPE ASSERTION

### Sites NOT REQUIRING Type Assertions ⚠️

#### 1. `parser.go` - Line 205 (ParseString method)
- **Location**: `parser.go:205` - `yaml.Unmarshal([]byte(yamlContent), data)`
- **Reason**: Returns error directly to caller without wrapping
- **Status**: ⚠️ NOT APPLICABLE (caller responsibility)

#### 2. `future.go` - Line 73 (ParseStream)
- **Location**: `future.go:73` - `yaml.Unmarshal(content, data)`
- **Reason**: Returns error directly to caller without wrapping
- **Status**: ⚠️ NOT APPLICABLE (caller responsibility)
- **Note**: File is marked `//go:build ignore`

#### 3. `syntax_validator.go` - Line 784 (DetectStructureErrors)
- **Location**: `syntax_validator.go:784` - `yaml.Unmarshal([]byte(yamlContent), &node)`
- **Reason**: Early return on error - "Parse errors are already captured in SyntaxErrors"
- **Status**: ⚠️ NOT APPLICABLE (handled by calling context)

#### 4. `validator.go` - Line 137 (ValidateStringWithPath)
- **Note**: Error is passed to `parseYAMLError` method which HAS the type assertion
- **Status**: ✅ COMPLETE (handled by downstream method)

## Sites Requiring Action

### Priority 1: schema.go Line 288
**Current code**:
```go
if err := yaml.Unmarshal(content, &data); err != nil {
    result.Valid = false
    
    // Has ParseError, SyntaxError, TypeMismatchError, StructureError, YAMLError checks
    // Missing: *yaml.TypeError
    
    // Generic fallback
    result.Errors = append(result.Errors, SchemaValidationError{
        Message: fmt.Sprintf("Failed to parse YAML: %v", err),
    })
    return result
}
```

**Recommended fix**:
```go
if err := yaml.Unmarshal(content, &data); err != nil {
    result.Valid = false
    
    // Add *yaml.TypeError check here, before YAMLError interface check
    if typeErr, ok := err.(*yaml.TypeError); ok {
        result.Errors = append(result.Errors, SchemaValidationError{
            Message:   fmt.Sprintf("YAML type error: %v", typeErr.Errors),
            ErrorCode: ErrCodeTypeMismatch,
        })
        return result
    }
    
    // ... rest of existing checks ...
}
```

### Priority 2: schema.go Line 703
**Current code**:
```go
case ".yaml", ".yml":
    if err := yaml.Unmarshal(content, &data); err != nil {
        return nil, &SchemaError{
            Message:  fmt.Sprintf("Failed to parse YAML schema: %v", err),
            FilePath: schemaPath,
        }
    }
```

**Recommended fix**:
```go
case ".yaml", ".yml":
    if err := yaml.Unmarshal(content, &data); err != nil {
        // Add type assertion for *yaml.TypeError
        if typeErr, ok := err.(*yaml.TypeError); ok {
            return nil, &SchemaError{
                Message:  fmt.Sprintf("YAML type error in schema: %v", typeErr.Errors),
                FilePath: schemaPath,
            }
        }
        return nil, &SchemaError{
            Message:  fmt.Sprintf("Failed to parse YAML schema: %v", err),
            FilePath: schemaPath,
        }
    }
```

## Summary Statistics

- **Total yaml.Unmarshal call sites**: 11
- **With yaml.TypeError type assertions**: 6 ✅
- **Missing yaml.TypeError type assertions**: 2 ❌
- **Not requiring type assertions**: 3 ⚠️
- **Completion percentage**: 75% (6/8 actionable sites)

## Patterns Observed

### Standard Error Handling Pattern
The files that have proper type assertions follow this pattern:
1. Sentinel checks (`io.EOF`, etc.)
2. Custom error type checks (`*SyntaxError`, `*StructureError`, etc.)
3. **`*yaml.TypeError` type assertion** ← THIS STEP
4. `YAMLError` interface check
5. Generic fallback

### Missing Steps
The two sites in `schema.go` skip step 3, causing TypeError details to be lost.

## Verification Notes

All type assertions use the standard pattern:
```go
if typeErr, ok := err.(*yaml.TypeError); ok {
    // Access typeErr.Errors field for detailed error messages
}
```

The `typeErr.Errors` field contains `[]string` with individual type mismatch errors, which is the critical information being captured.

## Files Requiring Changes

1. `/home/coding/ARMOR/internal/yamlutil/schema.go` (2 locations)
