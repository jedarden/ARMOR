# Task bf-271fe: Add Missing yaml.TypeError Type Assertions

## Summary
This task completed adding `*yaml.TypeError` type assertions to the two identified call sites in `internal/yamlutil/schema.go` that were missing them.

## Changes Made

### 1. SchemaValidator.ValidateFile() (Line 292)
Added `*yaml.TypeError` type assertion at the beginning of the error handling chain, immediately after `yaml.Unmarshal()`:

```go
if typeErr, ok := err.(*yaml.TypeError); ok {
    // Handle yaml.v3 TypeError - provide detailed type error information
    result.Errors = append(result.Errors, SchemaValidationError{
        Message:   fmt.Sprintf("YAML type error: %v", typeErr.Errors),
        ErrorCode: ErrCodeTypeMismatch,
    })
} else if parseErr, ok := err.(*ParseError); ok {
    // ... other error types
```

### 2. LoadSchema() (Line 715)
Added `*yaml.TypeError` type assertion in the YAML parsing case:

```go
if err := yaml.Unmarshal(content, &data); err != nil {
    // Type assertion: *yaml.TypeError captures type mismatch errors from yaml.v3
    // The Errors field contains a slice of error strings detailing each type mismatch
    // This preserves error information through the type assertion
    if typeErr, ok := err.(*yaml.TypeError); ok {
        // Provide detailed type error information
        return nil, &SchemaError{
            Message:  fmt.Sprintf("Failed to parse YAML schema: %v", typeErr.Errors),
            FilePath: schemaPath,
        }
    }
    // Generic error fallback
    return nil, &SchemaError{
        Message:  fmt.Sprintf("Failed to parse YAML schema: %v", err),
        FilePath: schemaPath,
    }
}
```

## Verification
- Code compiles without errors: ✅
- Type assertions follow the standard pattern: ✅
- Error information preserved through `typeErr.Errors`: ✅
- Both call sites from the audit now have type assertions: ✅

## References
- Audit document: `internal/yamlutil/yaml_typeerror_audit.md`
- Related beads: bf-4nqzv (audit phase)
