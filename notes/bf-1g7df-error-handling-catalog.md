# YAMLUtil Error Handling Call Sites Catalog

**Task ID:** bf-1g7df  
**Package:** `internal/yamlutil`  
**Date:** 2026-07-12

## Overview

This document catalogs all error handling call sites in the yamlutil package, identifying where error type checking is implemented and where it may be missing or incomplete.

## Error Type Hierarchy

The yamlutil package defines a comprehensive error hierarchy:

```
YAMLError (base interface)
├── FileError (file I/O errors)
├── ParseError (YAML parsing errors)
│   ├── SyntaxError (YAML syntax errors)
│   ├── StructureError (YAML structure errors)
│   └── TypeMismatchError (type conversion errors)
├── ValidationError (validation errors)
│   ├── FieldNotFoundError (missing required fields)
│   ├── ConstraintError (constraint violations)
│   └── DuplicateKeyError (duplicate key errors)
└── SchemaError (schema-related errors)
    ├── SchemaLoadError (schema loading errors)
    └── SchemaValidationError (schema validation errors)
```

## Call Site Catalog

### 1. file.go - File I/O Error Handling

**File:** `internal/yamlutil/file.go`

#### Site 1.1: ReadFile() - Path Resolution Error (Line 20-26)
```go
absPath, err := filepath.Abs(filePath)
if err != nil {
    return nil, &FileError{
        Operation: "resolve",
        Path:      filePath,
        Err:       fmt.Errorf("failed to resolve absolute path: %w", err),
    }
}
```
**Status:** ✅ **COMPLETE** - Properly wraps error in FileError type
**Error Types:** FileError
**Pattern:** Direct FileError construction with wrapped error

#### Site 1.2: ReadFile() - File Read Error (Line 30-36)
```go
content, err := os.ReadFile(absPath)
if err != nil {
    return nil, &FileError{
        Operation: "read",
        Path:      absPath,
        Err:       wrapFileError(err),
    }
}
```
**Status:** ✅ **COMPLETE** - Uses wrapFileError() helper for type checking
**Error Types:** FileError (via wrapFileError)

#### Site 1.3: wrapFileError() - EOF Check (Line 61-64)
```go
if errors.Is(err, io.EOF) {
    return err
}
```
**Status:** ✅ **COMPLETE** - Proper io.EOF type check
**Error Types:** io.EOF
**Pattern:** `errors.Is()` for sentinel error

#### Site 1.4: wrapFileError() - PathError Type Assertion (Line 67-70)
```go
var pathErr *os.PathError
if errors.As(err, &pathErr) {
    return fmt.Errorf("path error during %s: %w", pathErr.Op, pathErr)
}
```
**Status:** ✅ **COMPLETE** - Proper os.PathError type assertion
**Error Types:** os.PathError
**Pattern:** `errors.As()` with type assertion

#### Site 1.5: wrapFileError() - File Not Found (Line 72-74)
```go
if os.IsNotExist(err) {
    return fmt.Errorf("file not found: %w", err)
}
```
**Status:** ✅ **COMPLETE** - Proper os.IsNotExist check
**Error Types:** os.ErrNotExist
**Pattern:** `os.IsNotExist()` sentinel check

#### Site 1.6: wrapFileError() - Permission Denied (Line 75-77)
```go
if os.IsPermission(err) {
    return fmt.Errorf("permission denied: %w", err)
}
```
**Status:** ✅ **COMPLETE** - Proper os.IsPermission check
**Error Types:** os.ErrPermission
**Pattern:** `os.IsPermission()` sentinel check

---

### 2. validator.go - Validation Error Handling

**File:** `internal/yamlutil/validator.go`

#### Site 2.1: ValidateFile() - EOF Check (Line 166-176)
```go
if err == io.EOF {
    ve := LocalValidationError{
        FilePath: filePath,
        Message:  "Unexpected end of file - file may be incomplete or truncated",
        Type:     ErrorTypeIO,
        Context:  "The file ended before all expected content was read",
    }
    result.Errors = append(result.Errors, ve.ToValidationError())
    return result
}
```
**Status:** ✅ **COMPLETE** - Proper io.EOF type check
**Error Types:** io.EOF → LocalValidationError (ErrorTypeIO)
**Pattern:** Direct equality check for sentinel error

#### Site 2.2: ValidateFile() - Generic File Read Error (Line 177-182)
```go
ve := LocalValidationError{
    FilePath: filePath,
    Message:  fmt.Sprintf("Failed to read file: %v", err),
    Type:     ErrorTypeIO,
}
```
**Status:** ⚠️ **INCOMPLETE** - Missing type assertion for FileError
**Error Types Should Check:**
- `*FileError` - Should extract ErrorCode and Context
- `os.ErrNotExist` - Should use ErrorTypeFile
- `os.ErrPermission` - Should use ErrorTypeFile with specific code
**Recommendation:** Add type assertion for FileError before generic fallback

#### Site 2.3: parseYAMLError() - TypeError Assertion (Line 199-207)
```go
if typeErr, ok := err.(*yaml.TypeError); ok {
    ve.Type = ErrorTypeStructure
    ve.Message = fmt.Sprintf("YAML type mismatch errors: %v", typeErr.Errors)
    if len(typeErr.Errors) > 0 {
        ve.Context = fmt.Sprintf("Type errors: %s", strings.Join(typeErr.Errors, "; "))
    }
    return ve
}
```
**Status:** ✅ **COMPLETE** - Proper yaml.TypeError type assertion
**Error Types:** `*yaml.TypeError`
**Pattern:** Type assertion with comma-ok pattern

#### Site 2.4: parseYAMLError() - Generic Error (Line 191-252)
**Status:** ⚠️ **INCOMPLETE** - Missing checks for:
- `*SyntaxError` - Should extract line/column info
- `*StructureError` - Should extract duplicate key info
- `*YAMLError` - Should extract ErrorCode and type
**Recommendation:** Add type assertions for YAMLError hierarchy before generic fallback

---

### 3. parser.go - Parsing Error Handling

**File:** `internal/yamlutil/parser.go`

#### Site 3.1: ParseFile() - EOF Check (Line 53-61)
```go
if errors.Is(err, io.EOF) {
    result.Error = &YAMLParseError{
        FilePath: filePath,
        Message:  "Unexpected end of file - file may be incomplete or truncated",
        RawError: err,
    }
    return result
}
```
**Status:** ✅ **COMPLETE** - Proper io.EOF check using errors.Is
**Error Types:** io.EOF → YAMLParseError
**Pattern:** `errors.Is()` sentinel check

#### Site 3.2: ParseFile() - YAML TypeError (Line 70-77)
```go
if typeErr, ok := err.(*yaml.TypeError); ok {
    result.Error = &YAMLParseError{
        FilePath: filePath,
        Message:  fmt.Sprintf("YAML type error: %v", typeErr.Errors),
        RawError: err,
    }
}
```
**Status:** ✅ **COMPLETE** - Proper yaml.TypeError type assertion
**Error Types:** `*yaml.TypeError`

#### Site 3.3: ParseFile() - Generic Error (Line 78-79)
```go
result.Error = fmt.Errorf("YAML parse error: %w", err)
```
**Status:** ⚠️ **INCOMPLETE** - Should check for FileError, SyntaxError, StructureError
**Recommendation:** Add YAMLError type assertions before generic fallback

#### Site 3.4: ParseFileToMap() - Same patterns as ParseFile() (Line 101-128)
**Status:** Same as Sites 3.1-3.3

#### Site 3.5: ParseYAML() - EOF Check (Line 302-309)
```go
if errors.Is(err, io.EOF) {
    return nil, &YAMLParseError{
        FilePath: filePath,
        Message:  "Unexpected end of YAML content - file may be incomplete",
        RawError: err,
    }
}
```
**Status:** ✅ **COMPLETE** - Proper io.EOF check

#### Site 3.6: ParseYAML() - YAML TypeError (Line 310-318)
```go
if typeErr, ok := err.(*yaml.TypeError); ok {
    return nil, &YAMLParseError{
        FilePath: filePath,
        Message:  fmt.Sprintf("YAML type error: %v", typeErr.Errors),
        RawError: err,
        Line:     extractErrorLine(err),
    }
}
```
**Status:** ✅ **COMPLETE** - Proper yaml.TypeError type assertion

#### Site 3.7: ParseYAML() - Generic Error (Line 319-325)
```go
return nil, &YAMLParseError{
    FilePath: filePath,
    Message:  err.Error(),
    RawError: err,
    Line:     extractErrorLine(err),
}
```
**Status:** ⚠️ **INCOMPLETE** - Should check for YAMLError types
**Recommendation:** Add YAMLError type assertions

---

### 4. schema.go - Schema Validation Error Handling

**File:** `internal/yamlutil/schema.go`

#### Site 4.1: SchemaValidator.Validate() - Schema Compile YAMLError (Line 191-201)
```go
if yamlErr, ok := err.(YAMLError); ok {
    result.Errors = append(result.Errors, SchemaValidationError{
        Message:   fmt.Sprintf("Schema compilation failed: %s", yamlErr.Error()),
        ErrorCode: yamlErr.Code(),
    })
} else {
    result.Errors = append(result.Errors, SchemaValidationError{
        Message: fmt.Sprintf("Schema compilation failed: %v", err),
    })
}
```
**Status:** ✅ **COMPLETE** - Proper YAMLError interface assertion
**Error Types:** YAMLError (any type)
**Pattern:** Interface type assertion with comma-ok

#### Site 4.2: SchemaValidator.Validate() - Data Validation YAMLError (Line 212-222)
```go
if yamlErr, ok := err.(YAMLError); ok {
    result.Errors = append(result.Errors, SchemaValidationError{
        Message:   fmt.Sprintf("Data validation failed: %v", yamlErr),
        ErrorCode: yamlErr.Code(),
    })
} else {
    result.Errors = append(result.Errors, SchemaValidationError{
        Message: fmt.Sprintf("Data validation failed: %v", err),
    })
}
```
**Status:** ✅ **COMPLETE** - Proper YAMLError interface assertion

#### Site 4.3: compileSchema() - YAMLError Check (Line 289-293)
```go
if yamlErr, ok := err.(YAMLError); ok {
    return fmt.Errorf("schema compilation failed: %w", yamlErr)
}
return fmt.Errorf("schema compilation failed: %w", err)
```
**Status:** ✅ **COMPLETE** - Proper YAMLError interface assertion

#### Site 4.4: ValidateFile() - File Read Error (Line 262-267)
```go
if err != nil {
    result.Valid = false
    result.Errors = append(result.Errors, SchemaValidationError{
        Message: fmt.Sprintf("Failed to read file: %v", err),
    })
    return result
}
```
**Status:** ⚠️ **INCOMPLETE** - Should check for FileError type
**Recommendation:** Add FileError type assertion to extract ErrorCode

#### Site 4.5: ValidateFile() - YAML Parse Error (Line 272-277)
```go
if err := yaml.Unmarshal(content, &data); err != nil {
    result.Valid = false
    result.Errors = append(result.Errors, SchemaValidationError{
        Message: fmt.Sprintf("Failed to parse YAML: %v", err),
    })
    return result
}
```
**Status:** ⚠️ **INCOMPLETE** - Should check for ParseError, SyntaxError, TypeError
**Recommendation:** Add YAMLError type assertions

---

### 5. result.go - Result Type Error Conversion

**File:** `internal/yamlutil/result.go`

#### Site 5.1: AsParseError() - ParseError Check (Line 382-384)
```go
if pe, ok := err.(*ParseError); ok {
    return pe
}
```
**Status:** ✅ **COMPLETE** - Proper *ParseError type assertion
**Pattern:** Type assertion with comma-ok

#### Site 5.2: AsParseError() - YAMLError Interface Check (Line 387-422)
```go
if ye, ok := err.(YAMLError); ok {
    // Extract line/column info from specific error types
    switch e := err.(type) {
    case *SyntaxError:
        line, column = e.Line, e.Column
        filePath = e.FilePath
        message = e.Message
    case *StructureError:
        line = e.Line
        filePath = e.FilePath
        message = e.Message
    case *TypeMismatchError:
        line = e.Line
        filePath = e.FilePath
        message = fmt.Sprintf("type mismatch at %s: expected %s, got %s",
            e.FieldPath, e.ExpectedType, e.ActualType)
    case *ValidationError:
        line, column = e.Line, e.Column
        filePath = e.FilePath
        message = e.Message
    }
    return &ParseError{...}
}
```
**Status:** ✅ **COMPLETE** - Comprehensive type switch for all YAMLError types
**Pattern:** YAMLError interface assertion followed by type switch

#### Site 5.3: AsParseError() - Generic Error Fallback (Line 425-429)
```go
return &ParseError{
    Message: err.Error(),
    Err:     err,
}
```
**Status:** ✅ **COMPLETE** - Proper generic fallback after type checks

---

### 6. future.go - Legacy Error Handling

**File:** `internal/yamlutil/future.go`

#### Site 6.1: EOF Check (Line 52-56)
```go
if errors.Is(err, io.EOF) {
    return fmt.Errorf("empty YAML stream: %w", err)
}
return fmt.Errorf("stream read error: %w", err)
```
**Status:** ⚠️ **INCOMPLETE** - Missing FileError type check
**Recommendation:** Should check for FileError before io.EOF

#### Site 6.2: YAML TypeError (Line 83-85)
```go
if typeErr, ok := err.(*yaml.TypeError); ok {
    return nil, fmt.Errorf("YAML type error: %v", typeErr.Errors)
}
```
**Status:** ✅ **COMPLETE** - Proper yaml.TypeError assertion

#### Site 6.3: Generic Parse Error (Line 85)
```go
return nil, fmt.Errorf("YAML parse error: %w", err)
```
**Status:** ⚠️ **INCOMPLETE** - Should check for YAMLError types

---

### 7. debug_helpers.go - Helper Function Error Handling

**File:** `internal/yamlutil/debug_helpers.go`

#### Site 7.1: GetRequiredString() - Generic Error Return (Line 177)
```go
return "", err
```
**Status:** ⚠️ **INCOMPLETE** - No type checking
**Recommendation:** Should check for FieldNotFoundError, ValidationError

#### Site 7.2: GetRequiredInt() - Generic Error Return (Line 196)
```go
return 0, err
```
**Status:** ⚠️ **INCOMPLETE** - No type checking
**Recommendation:** Should check for FieldNotFoundError, TypeMismatchError, ValidationError

#### Site 7.3: GetRequiredBool() - Generic Error Return (Line 263)
```go
return false, err
```
**Status:** ⚠️ **INCOMPLETE** - No type checking
**Recommendation:** Should check for FieldNotFoundError, TypeMismatchError, ValidationError

---

## Summary of Findings

### Complete Type Assertions ✅

1. **file.go** - All error sites properly check error types
2. **schema.go** - YAMLError assertions properly implemented
3. **result.go/AsParseError()** - Comprehensive YAMLError type switch

### Incomplete Type Assertions ⚠️

1. **validator.go** - Missing FileError, YAMLError checks in ValidateFile()
2. **parser.go** - Missing YAMLError checks in generic error handlers
3. **schema.go** - Missing FileError, YAMLError checks in ValidateFile()
4. **future.go** - Missing FileError, YAMLError checks
5. **debug_helpers.go** - No type checking in GetRequired* functions

### Missing Type Assertions ❌

**Sites that return errors without any type checking:**

1. **validator.go:177-182** - ValidateFile() generic read error
2. **parser.go:78-79** - ParseFile() generic parse error
3. **parser.go:319-325** - ParseYAML() generic error
4. **schema.go:262-267** - ValidateFile() file read error
5. **schema.go:272-277** - ValidateFile() YAML parse error
6. **future.go:56** - Stream read error
7. **future.go:85** - YAML parse error
8. **debug_helpers.go:177,196,263** - GetRequired* functions

## Recommended Implementation Pattern

### Standard Pattern for Error Type Checking

```go
if err != nil {
    // 1. Check for sentinel errors first
    if errors.Is(err, io.EOF) {
        return &YAMLError{...}
    }
    
    // 2. Check for YAMLError interface
    if yamlErr, ok := err.(YAMLError); ok {
        return &LocalError{
            Message:   yamlErr.Error(),
            ErrorCode: yamlErr.Code(),
            Type:      yamlErr.YAMLErrorType(),
        }
    }
    
    // 3. Check for specific error types
    var fileErr *FileError
    if errors.As(err, &fileErr) {
        return &LocalError{...}
    }
    
    // 4. Generic fallback
    return fmt.Errorf("operation failed: %w", err)
}
```

### Pattern for Type Switch on YAMLError

```go
if ye, ok := err.(YAMLError); ok {
    switch e := err.(type) {
    case *SyntaxError:
        // Handle SyntaxError specifics
    case *StructureError:
        // Handle StructureError specifics
    case *ValidationError:
        // Handle ValidationError specifics
    case *FileError:
        // Handle FileError specifics
    default:
        // Generic YAMLError handling
    }
}
```

## Priority Implementation Order

### High Priority (affects error messages and user experience)
1. **validator.go:177-182** - Add FileError type assertion
2. **parser.go:78-79** - Add YAMLError type assertions
3. **schema.go:262-267,272-277** - Add FileError and YAMLError assertions

### Medium Priority (improves error context)
4. **parser.go:319-325** - Add YAMLError type assertions
5. **future.go:56,85** - Add FileError and YAMLError assertions
6. **validator.go:191-252** - Enhance parseYAMLError() with more type checks

### Low Priority (helper functions)
7. **debug_helpers.go** - Add type checks to GetRequired* functions

## Conclusion

The yamlutil package has a well-defined error type hierarchy but inconsistent type assertion patterns across call sites. Key areas for improvement:

1. **validator.go** and **parser.go** need YAMLError type assertions in generic error handlers
2. **schema.go** ValidateFile() needs FileError and ParseError checks
3. **debug_helpers.go** functions should check for specific error types
4. All generic error returns should follow the standard pattern: sentinel → YAMLError interface → specific types → generic fallback

Implementing these type assertions will provide:
- Better error messages with structured error codes
- Consistent error handling across the package
- Easier debugging with proper error type context
- Improved programmatic error handling for consumers of the package
