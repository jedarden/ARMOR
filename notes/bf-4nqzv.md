# yaml.TypeError Type Assertions Audit

**Date:** 2026-07-12
**Bead:** bf-4nqzv
**Scope:** Complete audit of all error handling locations in yamlutil package to identify where *yaml.TypeError type assertions are needed.

## Executive Summary
This audit catalogs all error handling locations in the `internal/yamlutil` package where `yaml.v3` parser errors could return `*yaml.TypeError`. It identifies which sites have type assertions and which are missing them.

## Background
`yaml.v3` parser returns `*yaml.TypeError` when type mismatches occur during unmarshaling. Proper type assertions are needed to extract the detailed error information from the `Errors` field, which contains a slice of error strings.

## Complete Catalog of yaml.Unmarshal Call Sites

### Files Analyzed
- `/home/coding/ARMOR/internal/yamlutil/parser.go` - 4 call sites, 3 with type assertions ✓
- `/home/coding/ARMOR/internal/yamlutil/validator.go` - 1 call site, 1 with type assertion ✓ (via parseYAMLError)
- `/home/coding/ARMOR/internal/yamlutil/syntax_validator.go` - 2 call sites, 1 with type assertion ✓
- `/home/coding/ARMOR/internal/yamlutil/future.go` - 2 call sites, 1 with type assertion ✓
- `/home/coding/ARMOR/internal/yamlutil/schema.go` - 2 call sites, 1 with type assertion ✓, **1 missing** ❌

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

#### 1. `schema.go` - Line 288 (Validate method in ValidateFile)
- **Location**: `schema.go:288` - `yaml.Unmarshal(content, &data)`
- **Current handling**: Has type assertions for `*ParseError`, `*SyntaxError`, `*TypeMismatchError`, `*StructureError`, and `YAMLError` interface
- **Missing**: No `*yaml.TypeError` type assertion
- **Impact**: When yaml.v3 returns a TypeError, it falls through to the generic YAMLError interface check, losing the specific `typeErr.Errors[]` information
- **Recommendation**: Add `*yaml.TypeError` type assertion before the `YAMLError` interface check (around line 316)
- **Status**: ❌ NEEDS TYPE ASSERTION

#### 2. `future.go` - Line 73 (ParseStream method)
- **Location**: `future.go:73` - `yaml.Unmarshal(content, data)` (return statement)
- **Current handling**: Raw error pass-through with no error handling
- **Missing**: No error handling at all
- **Impact**: Raw yaml.v3 errors passed to caller without any transformation or type detail extraction
- **Recommendation**: Add error handling with `*yaml.TypeError` assertion, or document that this is intentional stub behavior (file marked `//go:build ignore`)
- **Status**: ❌ NEEDS TYPE ASSERTION OR DOCUMENTATION

### Sites NOT REQUIRING Type Assertions ⚠️

#### 1. `parser.go` - Line 205 (ParseString method)
- **Location**: `parser.go:205` - `yaml.Unmarshal([]byte(yamlContent), data)`
- **Reason**: Returns error directly to caller without wrapping (by design)
- **Status**: ⚠️ NOT APPLICABLE (caller responsibility)

#### 2. `syntax_validator.go` - Line 784 (DetectStructureErrors)
- **Location**: `syntax_validator.go:784` - `yaml.Unmarshal([]byte(yamlContent), &node)`
- **Reason**: Early return on error with comment "Parse errors are already captured in SyntaxErrors"
- **Status**: ⚠️ NOT APPLICABLE (handled by calling context - ValidateSyntax)

#### 3. `schema.go` - Line 703 (LoadSchema)
- **Note**: This site DOES have `*yaml.TypeError` type assertion (lines 704-710)
- **Status**: ✅ COMPLETE

#### 4. `validator.go` - Line 137 (ValidateStringWithPath)
- **Note**: Error is passed to `parseYAMLError` method which HAS the type assertion (line 269)
- **Status**: ✅ COMPLETE (handled by downstream method)

## Sites Requiring Action

### Priority 1: schema.go Line 288 (Validate method)
**Location**: `schema.go:288-328` in ValidateFile method
**Current code pattern**:
```go
if err := yaml.Unmarshal(content, &data); err != nil {
    result.Valid = false

    // Has ParseError, SyntaxError, TypeMismatchError, StructureError checks
    if parseErr, ok := err.(*ParseError); ok {
        // Handle ParseError
    } else if syntaxErr, ok := err.(*SyntaxError); ok {
        // Handle SyntaxError
    } else if typeErr, ok := err.(*TypeMismatchError); ok {
        // Handle TypeMismatchError
    } else if structErr, ok := err.(*StructureError); ok {
        // Handle StructureError
    } else if yamlErr, ok := err.(YAMLError); ok {
        // Handle YAMLError interface
    } else {
        // Generic fallback
    }
}
```

**Issue**: When yaml.v3 parser returns `*yaml.TypeError`, it falls through to the generic YAMLError interface handler, losing the specific `typeErr.Errors[]` information.

**Recommended fix** (insert after line 310, before YAMLError interface check at line 316):
```go
} else if typeErr, ok := err.(*yaml.TypeError); ok {
    // Handle yaml.TypeError from yaml.v3 parser
    result.Errors = append(result.Errors, SchemaValidationError{
        Message:   fmt.Sprintf("YAML type error: %v", typeErr.Errors),
        ErrorCode: ErrCodeTypeMismatch,
    })
    return result
} else if yamlErr, ok := err.(YAMLError); ok {
```

### Priority 2: future.go Line 73 (ParseStream method)
**Location**: `future.go:73` in StreamParser.ParseStream method
**Current code**:
```go
func (sp *StreamParser) ParseStream(reader io.Reader, data interface{}) error {
    content, err := io.ReadAll(reader)
    if err != nil {
        // ... error handling ...
    }

    return yaml.Unmarshal(content, data)  // Raw pass-through
}
```

**Issue**: Raw yaml.v3 errors passed to caller without any transformation or type detail extraction.

**Options**:
1. Add error handling similar to ParseStreamToMap (same file, line 93-110):
```go
if err := yaml.Unmarshal(content, data); err != nil {
    if yamlErr, ok := err.(YAMLError); ok {
        return fmt.Errorf("YAML parse error [%s: %s]: %w", yamlErr.Code(), yamlErr.YAMLErrorType(), err)
    }
    if typeErr, ok := err.(*yaml.TypeError); ok {
        return fmt.Errorf("YAML type error: %v", typeErr.Errors)
    }
    return fmt.Errorf("YAML parse error: %w", err)
}
```

2. Document that this is intentional stub behavior (file marked `//go:build ignore`)

**Recommendation**: Since this is in a `//go:build ignore` file marked as stub implementation, add a comment documenting the current behavior as intentional for the stub.

## Summary Statistics

- **Total yaml.Unmarshal call sites**: 12
- **With yaml.TypeError type assertions**: 9 ✅
- **Missing yaml.TypeError type assertions**: 2 ❌
- **Not requiring type assertions**: 3 ⚠️
- **Delegation to helpers with assertions**: 2 ✅
- **Completion percentage**: 82% (9/11 sites requiring action)

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

1. `/home/coding/ARMOR/internal/yamlutil/schema.go` (1 location - line 288)
   - High priority: user-facing Validate API should preserve all error details

2. `/home/coding/ARMOR/internal/yamlutil/future.go` (1 location - line 73)
   - Low priority: stub implementation in `//go:build ignore` file
   - Could add documentation instead of code change
