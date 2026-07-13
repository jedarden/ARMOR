# yaml.TypeError Call Sites Audit - yamlutil Package

**Bead ID:** bf-4nqzv  
**Date:** 2026-07-12  
**Scope:** Comprehensive audit of all error handling locations in the yamlutil package

## Executive Summary

The yamlutil package contains **11 yaml.Unmarshal call sites** across 5 files. Of these, **6 sites have proper *yaml.TypeError type assertions**, while **5 sites are missing** type assertions for yaml.v3's TypeError.

**Status:** 🟡 PARTIAL - 54.5% coverage (6/11 sites)

---

## Files WITH yaml.TypeError Type Assertions ✓

### 1. parser.go (3 sites)

**Line 109 - ParseFile() method:**
```go
if typeErr, ok := err.(*yaml.TypeError); ok {
    result.Error = &YAMLParseError{
        FilePath: filePath,
        Message:  fmt.Sprintf("YAML type error: %v", typeErr.Errors),
        RawError: err,
    }
    return result
}
```
- **Context:** Main file parsing with structured error handling
- **Pattern:** Full type assertion with detailed error information

**Line 167 - ParseFileToMap() method:**
```go
if typeErr, ok := err.(*yaml.TypeError); ok {
    result.Error = &YAMLParseError{
        FilePath: filePath,
        Message:  fmt.Sprintf("YAML type error: %v", typeErr.Errors),
        RawError: err,
    }
}
```
- **Context:** Generic map parsing
- **Pattern:** Full type assertion with detailed error information

**Line 397 - ParseYAML() function:**
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
- **Context:** Standalone YAML parsing function
- **Pattern:** Full type assertion with line number extraction

### 2. validator.go (1 site)

**Line 269 - parseYAMLError() method:**
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
- **Context:** Converts yaml.v3 errors to LocalValidationError
- **Pattern:** Full type assertion with error detail extraction
- **Called by:** ValidateFile() → yaml.Unmarshal (line 137)

### 3. syntax_validator.go (1 site)

**Line 1032 - convertParseError() method:**
```go
if typeErr, ok := err.(*yaml.TypeError); ok {
    se.Message = fmt.Sprintf("YAML type mismatch: %v", typeErr.Errors)
    se.ErrorCode = ErrCodeTypeMismatch
    return se
}
```
- **Context:** Converts yaml.v3 errors to SyntaxError
- **Pattern:** Full type assertion with error code assignment
- **Called by:** ValidateSyntax() → yaml.Unmarshal (line 386)

### 4. future.go (1 site)

**Line 103 - ParseStreamToMap() method:**
```go
if typeErr, ok := err.(*yaml.TypeError); ok {
    return nil, fmt.Errorf("YAML type error: %v", typeErr.Errors)
}
```
- **Context:** Streaming YAML parsing (stub implementation)
- **Pattern:** Full type assertion with error wrapping

---

## Files MISSING yaml.TypeError Type Assertions ✗

### 1. parser.go (1 site)

**Line 205 - ParseString() method:**
```go
func (p *Parser) ParseString(yamlContent string, data interface{}) error {
    return yaml.Unmarshal([]byte(yamlContent), data)
}
```
- **Issue:** Returns raw error without type checking
- **Impact:** Callers lose detailed type mismatch information
- **Recommendation:** Add error handling with *yaml.TypeError type assertion

### 2. schema.go (2 sites)

**Line 288 - ValidateFile() method:**
```go
if err := yaml.Unmarshal(content, &data); err != nil {
    // Checks: *ParseError, *SyntaxError, *TypeMismatchError (custom), *StructureError, YAMLError
    // Missing: *yaml.TypeError from yaml.v3
    ...
}
```
- **Issue:** Error chain includes custom TypeMismatchError but NOT yaml.v3's TypeError
- **Impact:** Type errors from yaml.v3 parser fall through to generic handler
- **Recommendation:** Add `} else if typeErr, ok := err.(*yaml.TypeError); ok {` before the generic fallback

**Line 703 - LoadSchema() function:**
```go
case ".yaml", ".yml":
    if err := yaml.Unmarshal(content, &data); err != nil {
        return nil, &SchemaError{
            Message:  fmt.Sprintf("Failed to parse YAML schema: %v", err),
            FilePath: schemaPath,
        }
    }
```
- **Issue:** Wraps all errors in SchemaError without type checking
- **Impact:** Type mismatch details are lost in generic error message
- **Recommendation:** Add *yaml.TypeError type assertion before wrapping

### 3. syntax_validator.go (1 site)

**Line 784 - DetectStructureErrors() method:**
```go
err := yaml.Unmarshal([]byte(yamlContent), &node)
if err != nil {
    // Parse errors are already captured in SyntaxErrors
    return errors
}
```
- **Issue:** Silently ignores parse errors, including type errors
- **Impact:** Type errors are completely dropped
- **Recommendation:** At minimum, log type errors; ideally, return them in the result

### 4. future.go (1 site)

**Line 73 - ParseBytes() function:**
```go
return yaml.Unmarshal(content, data)
```
- **Issue:** Returns raw error without type checking
- **Impact:** Callers lose detailed type mismatch information
- **Recommendation:** Add error handling with *yaml.TypeError type assertion

---

## Summary Statistics

| Metric | Count | Percentage |
|--------|-------|------------|
| **Total yaml.Unmarshal call sites** | 11 | 100% |
| **Sites with *yaml.TypeError assertions** | 6 | 54.5% |
| **Sites missing assertions** | 5 | 45.5% |

| File | Unmarshal Calls | With TypeError | Missing |
|------|----------------|---------------|----------|
| parser.go | 4 | 3 | 1 |
| validator.go | 1 | 1 | 0 |
| syntax_validator.go | 2 | 1 | 1 |
| future.go | 2 | 1 | 1 |
| schema.go | 2 | 0 | 2 |

---

## Recommendations

### Priority 1 - High Impact
1. **schema.go line 288**: Add *yaml.TypeError assertion to prevent type errors from falling through to generic handler
2. **schema.go line 703**: Add *yaml.TypeError assertion to preserve type error details in schema loading

### Priority 2 - Medium Impact  
3. **parser.go line 205**: Add error handling to ParseString() method for consistency with other Parser methods
4. **future.go line 73**: Add error handling to ParseBytes() function

### Priority 3 - Low Impact
5. **syntax_validator.go line 784**: Consider logging or returning type errors instead of silently dropping them

---

## Pattern Reference

The established pattern for *yaml.TypeError type assertions in this codebase:

```go
if typeErr, ok := err.(*yaml.TypeError); ok {
    // Type assertion: *yaml.TypeError captures type mismatch errors from yaml.v3
    // The Errors field contains a slice of error strings detailing each type mismatch
    result.Error = &YAMLParseError{
        FilePath: filePath,
        Message:  fmt.Sprintf("YAML type error: %v", typeErr.Errors),
        RawError: err,
        Line:     extractErrorLine(err),  // optional
    }
    return result
}
```

**Key elements:**
- Type assertion: `err.(*yaml.TypeError)`
- Access error slice: `typeErr.Errors`
- Provide detailed message: `"YAML type error: %v"`
- Preserve raw error in wrapper
- Optionally extract line number

---

## Related Context

- **yaml.v3 TypeError**: Returned by yaml.Unmarshal when YAML content cannot be unmarshaled into the target type due to type mismatches
- **TypeMismatchError**: Custom yamlutil type, NOT the same as yaml.TypeError from yaml.v3
- **YAMLError interface**: Base interface for all yamlutil custom error types
- **Error handling chain**: Standard pattern is: sentinel checks → YAMLError interface → specific types → generic fallback

---

## Verification Test Coverage

The file `yaml_typeerror_test.go` contains tests that verify the presence of *yaml.TypeError type assertions in:
- parser.go ✓
- validator.go ✓
- syntax_validator.go ✓
- future.go ✓

**Test gap:** schema.go is not verified by these tests despite having 2 yaml.Unmarshal call sites.
