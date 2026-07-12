# YAML Util Error Handling Call Sites Catalog

## Overview
This document catalogs all error handling call sites in the `internal/yamlutil/` package, identifies missing type assertions, and provides implementation guidance for proper YAMLError type checking.

## Error Type Hierarchy

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

---

## Call Sites by File

### 1. file.go

#### Line 20: `ReadFile()` - filepath.Abs() error
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
**Current Handling:** ✅ Creates FileError with proper wrapping
**Type Assertion Needed:** No - already creating proper error type
**Error Types:** `ErrorTypeFile`

#### Line 30: `ReadFile()` - os.ReadFile() error
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
**Current Handling:** ✅ Creates FileError with proper wrapping
**Type Assertion Needed:** No - already creating proper error type
**Error Types:** `ErrorTypeFile`, `ErrCodeFileNotFound`, `ErrCodeFileAccessDenied`

#### Line 61: `wrapFileError()` - io.EOF check
```go
if errors.Is(err, io.EOF) {
    return err
}
```
**Current Handling:** ✅ Proper io.EOF check
**Type Assertion Needed:** No

#### Line 68: `wrapFileError()` - os.PathError check
```go
var pathErr *os.PathError
if errors.As(err, &pathErr) {
    return fmt.Errorf("path error during %s: %w", pathErr.Op, pathErr)
}
```
**Current Handling:** ✅ Proper os.PathError type assertion
**Type Assertion Needed:** No

#### Lines 72-77: `wrapFileError()` - OS error checks
```go
if os.IsNotExist(err) {
    return fmt.Errorf("file not found: %w", err)
}
if os.IsPermission(err) {
    return fmt.Errorf("permission denied: %w", err)
}
```
**Current Handling:** ✅ Proper OS error checks
**Type Assertion Needed:** No
**Potential Enhancement:** Could return FileError with specific error codes instead of fmt.Errorf

---

### 2. validator.go

#### Line 138: `ValidateStringWithPath()` - yaml.Unmarshal() error
```go
err := yaml.Unmarshal([]byte(yamlContent), &node)
if err != nil {
    result.Valid = false
    ve := v.parseYAMLError(err, filePath, yamlContent)
    result.Errors = append(result.Errors, ve.ToValidationError())
    return result
}
```
**Current Handling:** ✅ Delegates to parseYAMLError
**Type Assertion Needed:** In parseYAMLError (line 191)

#### Line 166: `ValidateFile()` - io.EOF check
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
**Current Handling:** ⚠️ Direct equality check, should use `errors.Is`
**Type Assertion Needed:** No - checking for io.EOF is correct
**Enhancement Needed:** Use `errors.Is(err, io.EOF)` instead of `err == io.EOF`

#### Line 177: `ValidateFile()` - Generic os.ReadFile error
```go
ve := LocalValidationError{
    FilePath: filePath,
    Message:  fmt.Sprintf("Failed to read file: %v", err),
    Type:     ErrorTypeIO,
}
```
**Current Handling:** ❌ Missing type assertion
**Type Assertion Needed:** ✅ YES - Should check for specific error types
**Recommended Fix:**
```go
// Check for specific error types
var yamlErr YAMLError
if errors.As(err, &yamlErr) {
    ve := LocalValidationError{
        FilePath: filePath,
        Message:  yamlErr.Error(),
        Type:     yamlErr.YAMLErrorType(),
    }
    result.Errors = append(result.Errors, ve.ToValidationError())
    return result
}

// Check for OS-specific errors
if os.IsNotExist(err) {
    // Handle file not found
}
if os.IsPermission(err) {
    // Handle permission denied
}
```

#### Line 199: `parseYAMLError()` - yaml.TypeError check
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
**Current Handling:** ✅ Proper type assertion for yaml.TypeError
**Type Assertion Needed:** No - already correct

---

### 3. parser.go

#### Line 50: `ParseFile()` - os.ReadFile() error
```go
content, err := os.ReadFile(filePath)
if err != nil {
    result.Success = false
    if errors.Is(err, io.EOF) {
        result.Error = &YAMLParseError{
            FilePath: filePath,
            Message:  "Unexpected end of file - file may be incomplete or truncated",
            RawError: err,
        }
        return result
    }
    result.Error = fmt.Errorf("failed to read file: %w", err)
    return result
}
```
**Current Handling:** ⚠️ Checks for io.EOF but missing other error types
**Type Assertion Needed:** ✅ YES - Should check for FileError and other YAMLError types
**Recommended Fix:**
```go
if err != nil {
    result.Success = false
    
    // Check for io.EOF
    if errors.Is(err, io.EOF) {
        result.Error = &YAMLParseError{
            FilePath: filePath,
            Message:  "Unexpected end of file - file may be incomplete or truncated",
            RawError: err,
        }
        return result
    }
    
    // Check if it's already a FileError
    var fileErr *FileError
    if errors.As(err, &fileErr) {
        result.Error = fileErr
        return result
    }
    
    // Check for OS-specific errors and wrap in FileError
    if os.IsNotExist(err) {
        result.Error = NewFileError(filePath, "read", "file not found", err)
        return result
    }
    if os.IsPermission(err) {
        result.Error = NewFileError(filePath, "read", "permission denied", err)
        return result
    }
    
    result.Error = fmt.Errorf("failed to read file: %w", err)
    return result
}
```

#### Line 67: `ParseFile()` - yaml.Unmarshal() error
```go
if err := yaml.Unmarshal(content, data); err != nil {
    result.Success = false
    if typeErr, ok := err.(*yaml.TypeError); ok {
        result.Error = &YAMLParseError{
            FilePath: filePath,
            Message:  fmt.Sprintf("YAML type error: %v", typeErr.Errors),
            RawError: err,
        }
    } else {
        result.Error = fmt.Errorf("YAML parse error: %w", err)
    }
    return result
}
```
**Current Handling:** ⚠️ Checks for yaml.TypeError but missing other error types
**Type Assertion Needed:** ✅ YES - Should also check for other YAMLError types
**Recommended Fix:**
```go
if err := yaml.Unmarshal(content, data); err != nil {
    result.Success = false
    
    // Check for yaml.TypeError
    if typeErr, ok := err.(*yaml.TypeError); ok {
        result.Error = NewTypeMismatchError(filePath, "", "expected", "got", "", 0, ErrCodeTypeMismatch)
        return result
    }
    
    // Check if it's already a YAMLError
    var yamlErr YAMLError
    if errors.As(err, &yamlErr) {
        result.Error = err
        return result
    }
    
    // Wrap in generic ParseError
    result.Error = NewParseError(filePath, err.Error(), 0, 0, ErrCodeParseError, "", "")
    return result
}
```

#### Line 98: `ParseFileToMap()` - Same issues as ParseFile()
**Same issues and recommended fixes as above.**

#### Line 116: `ParseFileToMap()` - Same issues as ParseFile() yaml.Unmarshal
**Same issues and recommended fixes as above.**

#### Line 286: `ParseYAML()` - ReadFile() error
```go
content, err := ReadFile(filePath)
if err != nil {
    return nil, err
}
```
**Current Handling:** ✅ Passes through FileError from ReadFile
**Type Assertion Needed:** No - ReadFile already returns proper error types

#### Line 302: `ParseYAML()` - yaml.Unmarshal() io.EOF check
```go
if errors.Is(err, io.EOF) {
    return nil, &YAMLParseError{
        FilePath: filePath,
        Message:  "Unexpected end of YAML content - file may be incomplete",
        RawError: err,
    }
}
```
**Current Handling:** ✅ Proper io.EOF check
**Type Assertion Needed:** No

#### Line 310: `ParseYAML()` - yaml.TypeError check
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
**Current Handling:** ✅ Proper type assertion
**Type Assertion Needed:** No

---

### 4. future.go

#### Line 50: `ParseStream()` - io.ReadAll() error
```go
content, err := io.ReadAll(reader)
if err != nil {
    if errors.Is(err, io.EOF) {
        return fmt.Errorf("empty YAML stream: %w", err)
    }
    return fmt.Errorf("stream read error: %w", err)
}
```
**Current Handling:** ⚠️ Only checks for io.EOF
**Type Assertion Needed:** ✅ YES - Should check for other error types
**Recommended Fix:**
```go
if err != nil {
    if errors.Is(err, io.EOF) {
        return NewFileError("", "stream read", "empty YAML stream", err)
    }
    
    // Check if it's already a YAMLError
    var yamlErr YAMLError
    if errors.As(err, &yamlErr) {
        return err
    }
    
    return NewFileError("", "stream read", "stream read error", err)
}
```

#### Line 68: `ParseStreamToMap()` - Same issues as ParseStream()
**Same issues and recommended fixes as above.**

#### Line 81: `ParseStreamToMap()` - yaml.TypeError check
```go
if typeErr, ok := err.(*yaml.TypeError); ok {
    return nil, fmt.Errorf("YAML type error: %v", typeErr.Errors)
}
```
**Current Handling:** ⚠️ Uses fmt.Errorf instead of proper YAMLError type
**Type Assertion Needed:** ✅ YES - Should create TypeMismatchError
**Recommended Fix:**
```go
if typeErr, ok := err.(*yaml.TypeError); ok {
    return nil, NewTypeMismatchError("", "", "expected", "got", "", 0, ErrCodeTypeMismatch)
}
```

---

### 5. schema.go

#### Line 262: `SchemaValidator.Validate()` - SchemaDefinition.Validate() error
```go
err := sv.schema.Validate(data)
if err != nil || !matched {
    // Type assertion to YAMLError for structured error codes
    var yamlErr YAMLError
    if errors.As(err, &yamlErr) {
        code := yamlErr.Code()
        return SchemaValidationResult{
            Valid: false,
            Errors: []SchemaError{
                {
                    FilePath:  sv.filePath,
                    Message:   err.Error(),
                    ErrorCode: code,
                    Context:   yamlErr.Context(),
                },
            },
        }
    }
    return SchemaValidationResult{
        Valid: false,
        Errors: []SchemaError{
            {
                FilePath: sv.filePath,
                Message:  err.Error(),
            },
        },
    }
}
```
**Current Handling:** ✅ Proper YAMLError type assertion
**Type Assertion Needed:** No - already correct

---

### 6. debug_helpers.go

#### Line 176: `debugValidateYAML()` - Validator.ValidateFile() error
```go
result := validator.ValidateFile(filePath)
if err != nil {
    return "", err
}
```
**Current Handling:** ⚠️ Doesn't actually check result.Errors
**Type Assertion Needed:** ✅ YES - Should check for specific error types in result
**Recommended Fix:**
```go
result := validator.ValidateFile(filePath)
if result.HasErrors() {
    // Check specific error types in result.Errors
    for _, ve := range result.Errors {
        var yamlErr YAMLError
        if errors.As(ve, &yamlErr) {
            switch yamlErr.YAMLErrorType() {
            case ErrorTypeFile:
                // Handle file errors
            case ErrorTypeSyntax:
                // Handle syntax errors
            // ... other types
            }
        }
    }
    return "", result
}
```

#### Lines 195, 262: Similar issues in other debug helper functions
**Same pattern of passing through errors without type checking.**

---

## Summary of Missing Type Assertions

### Critical Missing Type Assertions

1. **validator.go:177** - `ValidateFile()` generic os.ReadFile error
   - Missing: FileError check, OS error type checks
   - Impact: Generic error message instead of structured FileError

2. **parser.go:50** - `ParseFile()` os.ReadFile error
   - Missing: FileError check, OS error type checks (except io.EOF)
   - Impact: Generic error message instead of structured FileError

3. **parser.go:67** - `ParseFile()` yaml.Unmarshal error
   - Missing: YAMLError check beyond yaml.TypeError
   - Impact: Potential double-wrapping or loss of error context

4. **future.go:50** - `ParseStream()` io.ReadAll error
   - Missing: YAMLError check, FileError for non-EOF errors
   - Impact: Generic fmt.Errorf instead of structured error

5. **future.go:81** - `ParseStreamToMap()` yaml.TypeError handling
   - Missing: Creates fmt.Errorf instead of TypeMismatchError
   - Impact: Loses type-specific error information

6. **debug_helpers.go:176** - `debugValidateYAML()`
   - Missing: Type checking on ValidationResult errors
   - Impact: Generic error handling

### Type Assertion Patterns

#### Pattern 1: Check for YAMLError interface
```go
var yamlErr YAMLError
if errors.As(err, &yamlErr) {
    // Access yamlErr.Code(), yamlErr.YAMLErrorType(), yamlErr.Context()
    switch yamlErr.YAMLErrorType() {
    case ErrorTypeFile:
        // Handle file errors
    case ErrorTypeParse:
        // Handle parse errors
    case ErrorTypeValidation:
        // Handle validation errors
    // ... other types
    }
}
```

#### Pattern 2: Check for specific YAMLError types
```go
var fileErr *FileError
if errors.As(err, &fileErr) {
    // Handle FileError specifically
}

var parseErr *ParseError
if errors.As(err, &parseErr) {
    // Handle ParseError specifically
}

var validErr *ValidationError
if errors.As(err, &validErr) {
    // Handle ValidationError specifically
}
```

#### Pattern 3: Check for OS-level errors before wrapping
```go
if os.IsNotExist(err) {
    return NewFileError(path, "read", "file not found", err)
}
if os.IsPermission(err) {
    return NewFileError(path, "read", "permission denied", err)
}
if errors.Is(err, io.EOF) {
    return NewFileError(path, "read", "unexpected end of file", err)
}
```

#### Pattern 4: Check for external library errors
```go
var yamlTypeErr *yaml.TypeError
if errors.As(err, &yamlTypeErr) {
    return NewTypeMismatchError(filePath, field, expected, actual, value, line, ErrCodeTypeMismatch)
}
```

---

## Implementation Priority

### High Priority (Critical Error Handling)
1. **parser.go** - ParseFile() and ParseFileToMap() error handling
2. **validator.go** - ValidateFile() error handling
3. **file.go** - Enhance wrapFileError() to return FileError consistently

### Medium Priority (Improved Error Context)
4. **future.go** - ParseStream() and ParseStreamToMap() error handling
5. **debug_helpers.go** - Add type checking to result processing

### Low Priority (Nice to Have)
6. **syntax_validator.go** - Review error handling in validation methods
7. **schema.go** - Ensure all Schema interface implementations return proper errors

---

## Testing Guidelines

### When Adding Type Assertions

1. **Test each error path separately**
   ```go
   // Test file not found
   _, err := ParseFile("nonexistent.yaml", &data)
   var fileErr *FileError
   assert.True(t, errors.As(err, &fileErr))
   assert.Equal(t, ErrCodeFileNotFound, fileErr.Code())
   ```

2. **Test error unwrapping**
   ```go
   // Test that wrapped errors can be unwrapped
   _, err := ParseFile("test.yaml", &data)
   assert.True(t, errors.Is(err, os.ErrNotExist))
   ```

3. **Test error type switching**
   ```go
   var yamlErr YAMLError
   if errors.As(err, &yamlErr) {
       switch yamlErr.YAMLErrorType() {
       case ErrorTypeFile:
           // Verify file error handling
       }
   }
   ```

### Existing Tests to Update

1. **file_test.go** - Add tests for specific error codes
2. **parser_test.go** - Add type assertion tests
3. **validator_test.go** - Add YAMLError type checking tests
4. **error_cases_test.go** - Ensure all error types are tested

---

## Reference Functions

### Helper Functions Available in errors.go

- `IsYAMLError(err error) bool` - Check if error is any YAMLError type
- `IsParseError(err error) bool` - Check if error is ParseError
- `IsValidationError(err error) bool` - Check if error is ValidationError
- `IsFileError(err error) bool` - Check if error is FileError
- `GetYAMLErrorType(err error) ErrorType` - Get error type without full assertion
- `IsFileNotFoundError(err error) bool` - Check for file not found
- `IsPermissionError(err error) bool` - Check for permission errors

### Constructor Functions

- `NewFileError()` - Create FileError with proper initialization
- `NewParseError()` - Create ParseError with proper initialization
- `NewValidationError()` - Create ValidationError with proper initialization
- `NewTypeMismatchError()` - Create TypeMismatchError
- `NewFieldNotFoundError()` - Create FieldNotFoundError
- `NewConstraintError()` - Create ConstraintError
- `NewSyntaxError()` - Create SyntaxError
- `NewStructureError()` - Create StructureError
- `NewDuplicateKeyError()` - Create DuplicateKeyError
- `NewSchemaLoadError()` - Create SchemaLoadError
- `NewSchemaValidationError()` - Create SchemaValidationError

---

## Conclusion

This catalog identifies **6 critical locations** where type assertions are missing or incomplete, primarily in:
- File I/O error handling (parser.go, validator.go)
- YAML parsing error handling (parser.go, future.go)
- Result processing (debug_helpers.go)

Implementing these type assertions will significantly improve error handling by:
1. Providing structured error information to callers
2. Enabling proper error type switching
3. Maintaining error context through wrapping chains
4. Supporting programmatic error handling via error codes

All recommended changes maintain backward compatibility while adding enhanced error context and type safety.
