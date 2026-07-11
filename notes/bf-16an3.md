# Bead bf-16an3: Define Error Code Constants

## Status: ✅ Complete

### Implementation Location
`/home/coding/ARMOR/internal/yamlutil/errors.go` (lines 158-189)

### Acceptance Criteria Verification

#### ✅ Error code constants defined
All required constants are defined:
- `ErrCodeInvalidSyntax`
- `ErrCodeTypeMismatch`
- `ErrCodeInvalidStructure`
- `ErrCodeDuplicateKey`
- `ErrCodeParseError`
- `ErrCodeValidationFailed`
- `ErrCodeRequiredField`
- `ErrCodeConstraintViolation`
- `ErrCodeInvalidValue`

#### ✅ Codes organized by category
Constants are organized into logical groups:

**File Error Codes:**
- `ErrCodeFileNotFound`
- `ErrCodeFileAccessDenied`
- `ErrCodeFileIOError`
- `ErrCodeFileEmpty`

**Parse Error Codes:**
- `ErrCodeInvalidSyntax`
- `ErrCodeTypeMismatch`
- `ErrCodeInvalidStructure`
- `ErrCodeDuplicateKey`
- `ErrCodeParseError`

**Validation Error Codes:**
- `ErrCodeValidationFailed`
- `ErrCodeRequiredField`
- `ErrCodeConstraintViolation`
- `ErrCodeInvalidValue`

**Schema Error Codes:**
- `ErrCodeSchemaLoadFailed`
- `ErrCodeSchemaValidation`
- `ErrCodeSchemaNotFound`
- `ErrCodeSchemaInvalid`

#### ✅ All codes have godoc documentation
Each constant includes inline comments documenting its purpose:
```go
ErrCodeInvalidSyntax ErrorCode = "INVALID_SYNTAX"    // YAML syntax error
ErrCodeTypeMismatch  ErrorCode = "TYPE_MISMATCH"     // Type conversion error
```

#### ✅ Constants are exported and usable by error types
- All constants use PascalCase (exported in Go)
- Type is `ErrorCode` (exported type)
- Used by error types via `Code()` methods:
  - `ParseError.Code()` → returns `ErrCodeParseError` by default
  - `ValidationError.Code()` → returns `ErrCodeValidationFailed` by default
  - `SyntaxError.Code()` → returns `ErrCodeInvalidSyntax` by default
  - All error types accept `ErrorCode` in their constructor functions

### Usage Example
```go
// Creating an error with specific error code
err := NewSyntaxError("config.yaml", "invalid indentation", 
    10, 5, "indentation", "space", ErrCodeInvalidSyntax)

// Error types return codes programmatically
switch err.Code() {
case ErrCodeInvalidSyntax:
    // Handle syntax errors
case ErrCodeTypeMismatch:
    // Handle type mismatch errors
}
```

### Conclusion
The error code constants were already implemented as part of the comprehensive error handling system in the yamlutil package. All acceptance criteria are met and the constants are actively used throughout the codebase for programmatic error handling.
