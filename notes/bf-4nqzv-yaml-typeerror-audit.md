# yaml.TypeError Call Sites Audit Report

**Bead ID:** bf-4nqzv  
**Date:** 2026-07-12  
**Package:** internal/yamlutil  
**Objective:** Audit all error handling locations to identify where *yaml.TypeError type assertions are needed

## Executive Summary

The yamlutil package was audited for all error handling locations where `yaml.v3` parser returns errors that could be `*yaml.TypeError`. The audit found **11 call sites** with `yaml.Unmarshal` operations, of which:

- ✅ **6 sites** have proper *yaml.TypeError type assertions
- ⚠️ **3 sites** are missing *yaml.TypeError type assertions 
- ℹ️ **2 sites** delegate to methods that have type assertions

## Detailed Findings

### ✅ Sites WITH yaml.TypeError Type Assertions

| File | Line | Method/Function | Pattern |
|------|------|-----------------|---------|
| parser.go | 109 | ParseFile | `if typeErr, ok := err.(*yaml.TypeError); ok { ... }` |
| parser.go | 167 | ParseFileToMap | `if typeErr, ok := err.(*yaml.TypeError); ok { ... }` |
| parser.go | 397 | ParseYAML | `if typeErr, ok := err.(*yaml.TypeError); ok { ... }` |
| validator.go | 269 | parseYAMLError | `if typeErr, ok := err.(*yaml.TypeError); ok { ... }` |
| future.go | 103 | ParseStreamToMap | `if typeErr, ok := err.(*yaml.TypeError); ok { ... }` |
| syntax_validator.go | 1032 | convertParseError | `if typeErr, ok := err.(*yaml.TypeError); ok { ... }` |

**Pattern Used:**
```go
// Type assertion: *yaml.TypeError captures type mismatch errors from yaml.v3
// The Errors field contains a slice of error strings detailing each type mismatch
if typeErr, ok := err.(*yaml.TypeError); ok {
    // Provide detailed type error information
    return &YAMLParseError{
        FilePath: filePath,
        Message:  fmt.Sprintf("YAML type error: %v", typeErr.Errors),
        RawError: err,
    }
}
```

### ⚠️ Sites MISSING yaml.TypeError Type Assertions

| File | Line | Method/Function | Current Behavior | Impact |
|------|------|-----------------|------------------|---------|
| **schema.go** | 288 | ValidateFile | Checks *ParseError, *SyntaxError, *TypeMismatchError, *StructureError, YAMLError, generic fallback | **MEDIUM** - TypeErrors caught only by generic fallback |
| **future.go** | 73 | ParseStream | Returns raw error without type checking | **LOW** - Documented as stub implementation |
| **parser.go** | 205 | ParseString | Returns raw error without type checking | **LOW** - Simple passthrough method |

### 1. schema.go:288 - ValidateFile Method

**Current error handling chain:**
```go
if parseErr, ok := err.(*ParseError); ok {
    // Handle ParseError type
} else if syntaxErr, ok := err.(*SyntaxError); ok {
    // Handle SyntaxError type  
} else if typeErr, ok := err.(*TypeMismatchError); ok {
    // Handle TypeMismatchError type
} else if structErr, ok := err.(*StructureError); ok {
    // Handle StructureError type
} else if yamlErr, ok := err.(YAMLError); ok {
    // Handle generic YAMLError interface
} else {
    // Generic error fallback
}
```

**Gap:** Missing check for `*yaml.TypeError` from `yaml.v3` parser before the generic fallback.

**Recommendation:** Add `*yaml.TypeError` check after `*StructureError` and before the `YAMLError` interface check:

```go
} else if structErr, ok := err.(*StructureError); ok {
    // Handle StructureError type
    result.Errors = append(result.Errors, SchemaValidationError{
        Message:   fmt.Sprintf("Failed to parse YAML: %s", structErr.Error()),
        ErrorCode: structErr.Code(),
    })
} else if typeErr, ok := err.(*yaml.TypeError); ok {
    // Handle yaml.TypeError from yaml.v3 parser
    result.Errors = append(result.Errors, SchemaValidationError{
        Message:   fmt.Sprintf("YAML type error: %v", typeErr.Errors),
        ErrorCode: ErrCodeTypeMismatch,
    })
} else if yamlErr, ok := err.(YAMLError); ok {
```

### 2. future.go:73 - ParseStream Method

**Current behavior:**
```go
func (sp *StreamParser) ParseStream(reader io.Reader, data interface{}) error {
    content, err := io.ReadAll(reader)
    if err != nil {
        // error handling...
    }
    return yaml.Unmarshal(content, data)  // Just returns raw error
}
```

**Context:** This is a documented stub implementation (see file header: "go:build ignore"). The comment states: "Current implementation loads entire content into memory. Future versions will implement true streaming."

**Impact:** LOW - This is future/stub code not currently used in production paths.

### 3. parser.go:205 - ParseString Method  

**Current behavior:**
```go
func (p *Parser) ParseString(yamlContent string, data interface{}) error {
    return yaml.Unmarshal([]byte(yamlContent), data)  // Simple passthrough
}
```

**Context:** This is a simple passthrough method designed for convenience. The error handling is delegated to the caller.

**Impact:** LOW - Callers can handle the error themselves. However, for consistency with other parser methods, consider wrapping with type assertions.

### ℹ️ Sites DELEGATING to Type Assertion Methods

| File | Line | Method | Delegates To | Status |
|------|------|--------|--------------|--------|
| validator.go | 137 | ValidateStringWithPath | parseYAMLError (line 223) | ✅ Covered |
| syntax_validator.go | 386 | ValidateSyntax | convertParseError (line 1026) | ✅ Covered |
| syntax_validator.go | 784 | DetectStructureErrors | N/A (returns early) | ℹ️ Not needed |

**Note:** syntax_validator.go:784 returns early on parse errors because they're captured by the earlier call at line 386 which has proper type assertions.

## yaml.Unmarshal Call Site Inventory

All 11 `yaml.Unmarshal` call sites in the package:

| File | Line | Context | Type Assertion? |
|------|------|---------|------------------|
| parser.go | 67 | ParseFile | ✅ Yes (line 109) |
| parser.go | 162 | ParseFileToMap | ✅ Yes (line 167) |
| parser.go | 205 | ParseString | ❌ No (raw passthrough) |
| parser.go | 348 | ParseYAML | ✅ Yes (line 397) |
| schema.go | 288 | ValidateFile | ⚠️ No (has other types) |
| schema.go | 703 | ValidateFile | ⚠️ No (has other types) |
| validator.go | 137 | ValidateStringWithPath | ✅ Yes (via parseYAMLError) |
| future.go | 73 | ParseStream | ❌ No (stub code) |
| future.go | 93 | ParseStreamToMap | ✅ Yes (line 103) |
| syntax_validator.go | 386 | ValidateSyntax | ✅ Yes (via convertParseError) |
| syntax_validator.go | 784 | DetectStructureErrors | ℹ️ N/A (early return) |

## Current State of Error Handling Pattern

The yamlutil package uses a **standardized error handling pattern**:

```go
// Standard pattern: sentinel checks → YAMLError interface → specific types → generic fallback

// 1. Sentinel checks (io.EOF, os.IsNotExist, etc.)
if errors.Is(err, io.EOF) {
    // Handle EOF specifically
}

// 2. Custom error types (*SyntaxError, *StructureError, etc.)
if syntaxErr, ok := err.(*SyntaxError); ok {
    // Handle SyntaxError
}

// 3. YAMLError interface (base interface for all YAML errors)
if yamlErr, ok := err.(YAMLError); ok {
    // Handle YAMLError interface
}

// 4. yaml.v3 TypeError (*yaml.TypeError)
if typeErr, ok := err.(*yaml.TypeError); ok {
    // Handle yaml.TypeError from yaml.v3
}

// 5. Generic fallback
return fmt.Errorf("YAML parse error: %w", err)
```

**Current pattern ordering in parser.go and validator.go:**
1. Sentinel checks (io.EOF)
2. *SyntaxError
3. *StructureError  
4. YAMLError interface
5. ***yaml.TypeError** ← This is positioned AFTER YAMLError interface
6. Generic fallback

**Observation:** In parser.go, *yaml.TypeError is checked AFTER the YAMLError interface but before the generic fallback. This ordering makes sense because:
- YAMLError interface catches custom yamlutil error types
- *yaml.TypeError catches yaml.v3 library errors
- Generic fallback catches everything else

## Recommendations

### Priority 1: Fix schema.go Type Assertion Gap

**schema.go** has two `yaml.Unmarshal` call sites (lines 288 and 703) in the `ValidateFile` method that are missing *yaml.TypeError handling. Both sites use the same error handling pattern.

**Action:** Add *yaml.TypeError check to the error handling chain in `ValidateFile` method:

```go
} else if structErr, ok := err.(*StructureError); ok {
    result.Errors = append(result.Errors, SchemaValidationError{
        Message:   fmt.Sprintf("Failed to parse YAML: %s", structErr.Error()),
        ErrorCode: structErr.Code(),
    })
} else if typeErr, ok := err.(*yaml.TypeError); ok {
    // Handle yaml.TypeError from yaml.v3 parser  
    result.Errors = append(result.Errors, SchemaValidationError{
        Message:   fmt.Sprintf("YAML type error: %v", typeErr.Errors),
        ErrorCode: ErrCodeTypeMismatch,
    })
} else if yamlErr, ok := err.(YAMLError); ok {
```

**Files to modify:** `internal/yamlutil/schema.go` (lines ~315 and ~710)

### Priority 2: Consider Consistency for Passthrough Methods

**future.go** (line 73) and **parser.go** (line 205) have simple passthrough methods that return raw errors.

**Options:**
1. **Keep as-is** - These are convenience methods where error handling is delegated to callers
2. **Add type assertions** - For consistency with other package methods

**Recommendation:** Keep as-is for now, but document that these methods return raw yaml.v3 errors and callers should handle type assertions if needed.

### Priority 3: Document Error Handling Pattern

Consider adding a package-level comment documenting the standard error handling pattern:

```go
/*
Package yamlutil provides YAML parsing utilities with standardized error handling.

Error Handling Pattern:
  The package uses a consistent error handling pattern for yaml.Unmarshal operations:
  
  1. Sentinel checks (io.EOF, os.IsNotExist, etc.)
  2. Custom error types (*SyntaxError, *StructureError, etc.)  
  3. YAMLError interface (base interface for all YAML errors)
  4. yaml.v3 TypeError (*yaml.TypeError from gopkg.in/yaml.v3)
  5. Generic fallback
  
  When adding new yaml.Unmarshal call sites, follow this pattern to ensure
  comprehensive error type coverage.
*/
```

## Conclusion

The audit found that **6 out of 11** yaml.Unmarshal call sites have proper *yaml.TypeError type assertions. The main gap is in **schema.go**, which has two Unmarshal sites with comprehensive error handling for other types but is missing the *yaml.TypeError check. This should be added for consistency with the rest of the package.

The other gaps (future.go and parser.go passthrough methods) are lower priority as they are either stub code or simple convenience methods where error handling delegation is intentional.

**Status:** ✅ Audit complete - one fix recommended for schema.go

**Next Steps:**
1. Add *yaml.TypeError handling to schema.go ValidateFile method
2. Consider adding package-level documentation for error handling pattern
3. Monitor for any new yaml.Unmarshal call sites in future development
