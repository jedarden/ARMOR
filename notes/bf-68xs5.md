# Bead bf-68xs5: Add Field Path and Constraint to ValidationError Messages

## Task Summary
Update ValidationError.Error() to include field path (e.g., "spec.replicas") and constraint details in error messages, with support for nested field paths using dot notation.

## Findings
**The requested feature is already fully implemented in the codebase.**

### Current Implementation (internal/yamlutil/errors.go)

The `ValidationError` struct already includes the required fields:
- `FieldPath string` - Dot-notation path to the invalid field (optional)
- `Constraint string` - Constraint that was violated (optional)

The `Error()` method (lines 438-465) already formats messages correctly:

```go
func (ve *ValidationError) Error() string {
    // ... build base error with location ...
    
    // Add field path if available
    if ve.FieldPath != "" {
        sb.WriteString(fmt.Sprintf(" at field %s", ve.FieldPath))
    }
    
    // Add message
    sb.WriteString(fmt.Sprintf(": %s", ve.Message))
    
    // Add constraint if available
    if ve.Constraint != "" {
        sb.WriteString(fmt.Sprintf(" (constraint: %s)", ve.Constraint))
    }
    
    return sb.String()
}
```

### Example Outputs

**Basic field path:**
```
validation error in config.yaml at field server.port: invalid port number (constraint: must be between 1-65535)
```

**Nested field path with array index:**
```
validation error in deployment.yaml at line 22, column 18 at field spec.template.spec.containers[0].image: invalid image tag (constraint: must match registry/*:tag pattern)
```

**spec.replicas field path:**
```
validation error in manifest.yaml at line 8 at field spec.replicas: replicas must be positive (constraint: must be >= 0)
```

### Acceptance Criteria - All Met ✓

- ✓ ValidationError messages include field path (e.g., "server.port", "spec.replicas")
- ✓ Constraint information is clearly shown (e.g., "(constraint: must be between 1-65535)")
- ✓ Nested field paths use dot notation (e.g., "spec.template.spec.containers[0].image")

### Test Coverage
Comprehensive tests exist in `internal/yamlutil/errors_test.go`:
- `TestNewValidationError` - 7 test cases covering various scenarios
- `TestValidationErrorString` - String() method output tests
- All tests pass successfully

### Work Completed
Added `validation_error_demo_test.go` to demonstrate the existing functionality and verify acceptance criteria.
