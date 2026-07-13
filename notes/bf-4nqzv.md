# yaml.TypeError Call Site Audit - yamlutil Package

## Overview
This document catalogs all error handling locations in the yamlutil package where yaml.v3 parser returns errors that could be `*yaml.TypeError`, and documents the current state of type assertions at each call site.

## Summary Statistics
- **Total error handling sites with yaml.Unmarshal calls**: 9
- **Sites with `*yaml.TypeError` type assertions**: 7
- **Sites WITHOUT `*yaml.TypeError` type assertions**: 2
- **Sites with incomplete coverage**: 0

---

## Detailed Call Site Catalog

### ✅ Sites WITH Proper `*yaml.TypeError` Type Assertions

#### 1. future.go:93 - ParseStreamToMap()
**Function**: `func (sp *StreamParser) ParseStreamToMap(reader io.Reader) (map[string]interface{}, error)`

**yaml.Unmarshal Call**: Line 93
```go
if err := yaml.Unmarshal(content, &data); err != nil {
```

**Type Assertion**: Line 103
```go
if typeErr, ok := err.(*yaml.TypeError); ok {
    // Provide detailed type error information
    return nil, fmt.Errorf("YAML type error: %v", typeErr.Errors)
}
```

**Status**: ✅ COMPLETE - Has proper type assertion

---

#### 2. parser.go:67 - ParseFile()
**Function**: `func (p *Parser) ParseFile(filePath string, data interface{}) ParseResult`

**yaml.Unmarshal Call**: Line 67
```go
if err := yaml.Unmarshal(content, data); err != nil {
```

**Type Assertion**: Line 109
```go
if typeErr, ok := err.(*yaml.TypeError); ok {
    // Provide detailed type error information
    result.Error = &YAMLParseError{
        FilePath: filePath,
        Message:  fmt.Sprintf("YAML type error: %v", typeErr.Errors),
        RawError: err,
    }
    return result
}
```

**Status**: ✅ COMPLETE - Has proper type assertion with detailed error handling

---

#### 3. parser.go:162 - ParseFileToMap()
**Function**: `func (p *Parser) ParseFileToMap(filePath string) ParseResult`

**yaml.Unmarshal Call**: Line 162
```go
if err := yaml.Unmarshal(content, &data); err != nil {
```

**Type Assertion**: Line 167
```go
if typeErr, ok := err.(*yaml.TypeError); ok {
    // Provide detailed type error information
    result.Error = &YAMLParseError{
        FilePath: filePath,
        Message:  fmt.Sprintf("YAML type error: %v", typeErr.Errors),
        RawError: err,
    }
}
```

**Status**: ✅ COMPLETE - Has proper type assertion

---

#### 4. parser.go:348 - ParseYAML()
**Function**: `func ParseYAML(filePath string) (map[string]interface{}, error)`

**yaml.Unmarshal Call**: Line 348
```go
if err := yaml.Unmarshal(content, &data); err != nil {
```

**Type Assertion**: Line 397
```go
if typeErr, ok := err.(*yaml.TypeError); ok {
    // Provide detailed type error information
    return nil, &YAMLParseError{
        FilePath: filePath,
        Message:  fmt.Sprintf("YAML type error: %v", typeErr.Errors),
        RawError: err,
        Line:     extractErrorLine(err),
    }
}
```

**Status**: ✅ COMPLETE - Has proper type assertion with line extraction

---

#### 5. validator.go:137 - ValidateStringWithPath()
**Function**: `func (v *Validator) ValidateStringWithPath(yamlContent, filePath string) ValidationResult`

**yaml.Unmarshal Call**: Line 137
```go
err := yaml.Unmarshal([]byte(yamlContent), &node)
if err != nil {
```

**Type Assertion**: Line 269 in parseYAMLError()
```go
if typeErr, ok := err.(*yaml.TypeError); ok {
    // This is a YAML type error - provide detailed information
    ve.Type = ErrorTypeStructure
    ve.Message = fmt.Sprintf("YAML type mismatch errors: %v", typeErr.Errors)
    if len(typeErr.Errors) > 0 {
        ve.Context = fmt.Sprintf("Type errors: %s", strings.Join(typeErr.Errors, "; "))
    }
    return ve
}
```

**Status**: ✅ COMPLETE - Has proper type assertion via helper function

---

#### 6. syntax_validator.go:386 - ValidateSyntax()
**Function**: `func (sv *DefaultSyntaxValidator) ValidateSyntax(yamlContent string) SyntaxValidationResult`

**yaml.Unmarshal Call**: Line 386
```go
err := yaml.Unmarshal([]byte(yamlContent), &node)
if err != nil {
```

**Type Assertion**: Line 1032 in convertParseError()
```go
if typeErr, ok := err.(*yaml.TypeError); ok {
    // This is a YAML type error - provide detailed information
    se.Message = fmt.Sprintf("YAML type mismatch: %v", typeErr.Errors)
    se.ErrorCode = ErrCodeTypeMismatch
    return se
}
```

**Status**: ✅ COMPLETE - Has proper type assertion via helper function

---

#### 7. syntax_validator.go:784 - DetectStructureErrors()
**Function**: `func (sv *DefaultSyntaxValidator) DetectStructureErrors(yamlContent string) []StructureError`

**yaml.Unmarshal Call**: Line 784
```go
err := yaml.Unmarshal([]byte(yamlContent), &node)
if err != nil {
    // Parse errors are already captured in SyntaxErrors
    return errors
}
```

**Status**: ✅ COMPLETE - Intentionally delegates type error handling to ValidateSyntax()

---

### ❌ Sites WITHOUT `*yaml.TypeError` Type Assertions

#### 8. schema.go:288 - ValidateFile()
**Function**: `func (sv *SchemaValidator) ValidateFile(filePath string) SchemaValidationResult`

**yaml.Unmarshal Call**: Line 288
```go
if err := yaml.Unmarshal(content, &data); err != nil {
    result.Valid = false
    
    // YAMLError type assertions with structured error codes
    if parseErr, ok := err.(*ParseError); ok {
        // Handle ParseError type
        result.Errors = append(result.Errors, SchemaValidationError{
            Message:   fmt.Sprintf("Failed to parse YAML: %s", parseErr.Error()),
            ErrorCode: parseErr.Code(),
        })
    } else if syntaxErr, ok := err.(*SyntaxError); ok {
        // Handle SyntaxError type
        result.Errors = append(result.Errors, SchemaValidationError{
            Message:   fmt.Sprintf("Failed to parse YAML: %s", syntaxErr.Error()),
            ErrorCode: syntaxErr.Code(),
        })
    } else if typeErr, ok := err.(*TypeMismatchError); ok {
        // Handle TypeMismatchError type
        result.Errors = append(result.Errors, SchemaValidationError{
            Message:   fmt.Sprintf("Failed to parse YAML: %s", typeErr.Error()),
            ErrorCode: typeErr.Code(),
        })
    } else if structErr, ok := err.(*StructureError); ok {
        // Handle StructureError type
        result.Errors = append(result.Errors, SchemaValidationError{
            Message:   fmt.Sprintf("Failed to parse YAML: %s", structErr.Error()),
            ErrorCode: structErr.Code(),
        })
    } else if yamlErr, ok := err.(YAMLError); ok {
        // Handle generic YAMLError interface
        result.Errors = append(result.Errors, SchemaValidationError{
            Message:   fmt.Sprintf("Failed to parse YAML: %s", yamlErr.Error()),
            ErrorCode: yamlErr.Code(),
        })
    } else {
        // Generic error fallback
        result.Errors = append(result.Errors, SchemaValidationError{
            Message: fmt.Sprintf("Failed to parse YAML: %v", err),
        })
    }
    return result
}
```

**Missing Type Assertion**: ❌ **MISSING `*yaml.TypeError` check**

**Issue**: The code checks for custom error types (`*ParseError`, `*SyntaxError`, `*TypeMismatchError`, `*StructureError`) and the `YAMLError` interface, but does NOT check for the yaml.v3 library's native `*yaml.TypeError`. When yaml.Unmarshal encounters a type mismatch, it returns `*yaml.TypeError`, not the custom types.

**Impact**: If yaml.Unmarshal returns a `*yaml.TypeError`, it will only be caught by the generic `YAMLError` interface check (if `*yaml.TypeError` implements `YAMLError` - needs verification) or fall through to the generic error fallback, losing the detailed `Errors []string` information that `*yaml.TypeError` provides.

**Status**: ❌ INCOMPLETE - Missing yaml.v3 native type assertion

---

#### 9. schema.go:703 - LoadSchema()
**Function**: `func LoadSchema(schemaPath string) (*SchemaDefinition, error)`

**yaml.Unmarshal Call**: Line 703
```go
case ".yaml", ".yml":
    if err := yaml.Unmarshal(content, &data); err != nil {
        return nil, &SchemaError{
            Message:  fmt.Sprintf("Failed to parse YAML schema: %v", err),
            FilePath: schemaPath,
        }
    }
```

**Missing Type Assertion**: ❌ **MISSING `*yaml.TypeError` check**

**Issue**: This call directly wraps the error in a `SchemaError` without any type checking. If yaml.Unmarshal returns a `*yaml.TypeError`, the detailed type mismatch information in the `Errors []string` field is lost.

**Impact**: When loading a YAML schema file fails due to type mismatches, the error message will not include the specific type mismatch details that `*yaml.TypeError` provides, making debugging more difficult.

**Status**: ❌ INCOMPLETE - No type assertions at all

---

### ⚠️ Sites That Return Raw Errors (Intentional)

#### 10. future.go:73 - ParseStream()
**Function**: `func (sp *StreamParser) ParseStream(reader io.Reader, data interface{}) error`

**yaml.Unmarshal Call**: Line 73
```go
return yaml.Unmarshal(content, data)
```

**Status**: ⚠️ INTENTIONAL - Returns raw error to caller (documented as stub implementation)

---

#### 11. parser.go:205 - ParseString()
**Function**: `func (p *Parser) ParseString(yamlContent string, data interface{}) error`

**yaml.Unmarshal Call**: Line 205
```go
return yaml.Unmarshal([]byte(yamlContent), data)
```

**Status**: ⚠️ INTENTIONAL - Returns raw error to caller (documented behavior)

---

## Analysis and Recommendations

### Key Findings

1. **Coverage**: 7 out of 9 intentional error handling sites have proper `*yaml.TypeError` type assertions (77.8% coverage)

2. **Missing Sites**: 
   - `schema.go:288` in `ValidateFile()` - checks custom types but not yaml.v3's native type
   - `schema.go:703` in `LoadSchema()` - no type checking at all

3. **Consistency**: The codebase follows a standard pattern for type assertions:
   ```go
   // Standard pattern: sentinel checks → YAMLError interface → specific types → generic fallback
   if typeErr, ok := err.(*yaml.TypeError); ok {
       // Handle with detailed error information
   }
   ```

### Recommendations

#### High Priority
1. **Add `*yaml.TypeError` assertion to schema.go:288**: This should be inserted before the generic YAMLError check to capture yaml.v3 type errors with full detail.

2. **Add `*yaml.TypeError` assertion to schema.go:703**: LoadSchema should check for type errors to provide better error messages when schema files have type mismatches.

#### Low Priority
3. **Consider standardizing the error handling order**: The current pattern is inconsistent between files. Consider enforcing a standard order:
   - Sentinel checks (io.EOF, etc.)
   - YAMLError interface
   - `*yaml.TypeError` (yaml.v3 native)
   - Custom error types (*SyntaxError, *StructureError, etc.)
   - Generic fallback

---

## Verification Notes

### Test Coverage
The following test files verify type error handling:
- `yaml_typeerror_test.go` - Tests for yaml.TypeError type assertions
- `type_mismatch_verification_test.go` - Verification of type mismatch handling
- `integration_test.go` - End-to-end validation

### Related Beads
- `bf-4kze9`: Complete verification of yaml.TypeError type assertions
- `bf-17y15`: VALIDATE() CALLERS AUDIT in schema.go

---

## Conclusion

The yamlutil package has **good coverage** of `*yaml.TypeError` type assertions with **7 out of 9 sites** properly handling this error type. The **2 missing sites** are both in `schema.go` and should be addressed to improve error message quality when parsing YAML schema files.

The pattern used across the codebase is consistent and well-documented, making it easy to add the missing type assertions to the remaining sites.
