# Validate() Implementation and Error Flow Analysis

**Bead ID**: bf-2xbez
**Date**: 2026-07-12
**Scope**: ARMOR yamlutil package

## Executive Summary

The ARMOR codebase contains a comprehensive YAML validation system with a newly defined `ValidatedSchema` interface that is not yet fully integrated with existing schema implementations. The system has a well-structured error hierarchy but shows inconsistencies between interface definitions and concrete implementations.

## 1. ValidatedSchema Interface Definition

**Location**: `internal/yamlutil/schema_interfaces.go:31-44`

```go
type ValidatedSchema interface {
    Validate() YAMLError
    Name() string
    Description() string
    Version() string
}
```

**Key Characteristic**: This interface is designed to validate the **schema definition itself**, not data against the schema.

### Implementations Found

**Result**: **ZERO concrete implementations found**

No types in the codebase implement the `ValidatedSchema` interface. The interface appears to be newly defined as part of the bf-68hqo error hierarchy integration effort.

## 2. Existing Schema Type: SchemaDefinition

**Location**: `internal/yamlutil/schema.go:54-80`

`SchemaDefinition` is the primary schema implementation but implements a **different** interface:

```go
type Schema interface {
    Validate(value interface{}) error  // Validates DATA, not schema
}
```

### SchemaDefinition Methods

| Method | Signature | Purpose | Returns |
|--------|-----------|---------|---------|
| `Compile()` | `func (s *SchemaDefinition) Compile() error` | Validates schema definition | `error` (primitive Go type) |
| `Validate()` | `func (s *SchemaDefinition) Validate(value interface{}) error` | Validates data against schema | `error` (primitive Go type) |

**Key Finding**: `SchemaDefinition` does NOT implement `ValidatedSchema` because:
1. It has no `Validate() YAMLError` method
2. Its `Compile()` method returns `error`, not `YAMLError`
3. Its `Validate()` method takes a parameter (data to validate), unlike `ValidatedSchema.Validate()`

## 3. Validate() Method Implementations

### 3.1 SchemaDefinition.Compile()

**Location**: `internal/yamlutil/schema.go:732-748`

**Error Returns**:
- `NewSchemaLoadError()` - returns YAMLError
- `NewValidationError()` - returns YAMLError
- Method signature returns `error` (primitive type)

**Issue**: Returns YAMLError instances but typed as `error` interface.

### 3.2 SchemaDefinition.Validate()

**Location**: `internal/yamlutil/schema.go:750-785`

**Error Returns**:
- `NewValidationError()` - returns *ValidationError (YAMLError)
- `NewTypeMismatchError()` - returns *TypeMismatchError (YAMLError)
- `NewFieldNotFoundError()` - returns *FieldNotFoundError (YAMLError)
- Method signature returns `error` (primitive type)

## 4. Error Hierarchy

**Location**: `internal/yamlutil/errors.go`

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

## 5. Validate() Call Sites

### 5.1 SchemaValidator.Validate()
**Location**: `internal/yamlutil/schema.go:157-206`
**Pattern**: Wraps Schema.Validate() error in SchemaValidationResult struct.

### 5.2 SchemaValidator.ValidateFile()
**Location**: `internal/yamlutil/schema.go:212-244`
**Pattern**: File read → Parse → Validate.

### 5.3 SchemaValidator.compileSchema()
**Location**: `internal/yamlutil/schema.go:246-252`
**Pattern**: Calls Compile() during validator initialization.

## 6. Error Conversion Points

### Point 1: Compile() → error interface
**Issue**: Method signature returns primitive `error` but actual return values are YAMLError concrete types.

### Point 2: Validate() → error interface
**Issue**: Same as Compile() - YAMLError typed as `error`.

### Point 3: SchemaValidator wraps errors
**Issue**: YAMLError details lost when wrapped in SchemaValidationError struct. Only message preserved.

## 7. Current Error Return Patterns

### Pattern 1: Direct YAMLError return (method signature issue)
**Problem**: Caller must type-assert to access YAMLError methods.

### Pattern 2: Error wrapping in result structs
**Problem**: Loses error code, type, and context from original YAMLError.

### Pattern 3: Error propagation
**Status**: Preserves error type but signature mismatch remains.

## 8. Constraint Implementations

**Location**: `internal/yamlutil/schema_interfaces.go`

### Implementations

| Type | Method | Returns |
|------|--------|---------|
| `StringConstraintImpl` | `Validate(value interface{}) *ConstraintError` | Concrete error type |
| `NumberConstraintImpl` | `Validate(value interface{}) *ConstraintError` | Concrete error type |
| `ArrayConstraintImpl` | `Validate(value interface{}) *ConstraintError` | Concrete error type |
| `ObjectConstraintImpl` | `Validate(value interface{}) *ConstraintError` | Concrete error type |
| `BooleanConstraintImpl` | `Validate(value interface{}) *ConstraintError` | Concrete error type |
| `TypeConstraintImpl` | `Validate(value interface{}) *ConstraintError` | Concrete error type |

**Key Difference**: Constraints return `*ConstraintError` (concrete type) not wrapped in interface.

## 9. Identified Issues

### Issue 1: Interface-Implementation Mismatch
**Severity**: High

`ValidatedSchema` interface defines `Validate() YAMLError` but `SchemaDefinition` has:
- `Compile() error` - should this be `Validate() YAMLError`?
- Different method semantics (validates schema vs validates data)

### Issue 2: Error Type Loss
**Severity**: Medium

When SchemaValidator wraps errors in SchemaValidationResult, YAMLError details are lost.

### Issue 3: Inconsistent Return Types
**Severity**: Medium

`Compile()` and `Validate()` return YAMLError instances but typed as `error`:
- Prevents compile-time type checking
- Requires runtime type assertions
- Inconsistent with YAMLError-first design intent

### Issue 4: Unused Interface
**Severity**: Low

`ValidatedSchema` interface has zero implementations - may be incomplete design or future work.

## 10. Recommendations

### Recommendation 1: Align SchemaDefinition with ValidatedSchema
Either:
A. Add `Validate() YAMLError` to SchemaDefinition that validates the schema itself
B. Rename ValidatedSchema to clarify its purpose (e.g., `SelfValidatingSchema`)

### Recommendation 2: Fix Return Type Signatures
Change method signatures to return YAMLError directly:

```go
// Before
func (s *SchemaDefinition) Compile() error

// After
func (s *SchemaDefinition) Compile() YAMLError
func (s *SchemaDefinition) Validate() YAMLError
```

### Recommendation 3: Preserve YAMLError Details
When wrapping errors, preserve full YAMLError information.

### Recommendation 4: Type Assertion Helpers
Add helper functions for safe YAMLError handling.

## 11. Summary Statistics

| Category | Count |
|----------|-------|
| ValidatedSchema implementations | 0 |
| Validate() methods found | 20+ (different types) |
| YAMLError types defined | 11 |
| Error constructors | 8 |
| Direct YAMLError returns (constraints) | 6 |
| Wrapped error returns (schemas) | 2 |

## 12. Files Analyzed

| File | Lines | Purpose |
|------|-------|---------|
| `internal/yamlutil/schema_interfaces.go` | 964 | ValidatedSchema interface, constraints |
| `internal/yamlutil/errors.go` | 1280 | Error hierarchy definitions |
| `internal/yamlutil/schema.go` | 1122 | SchemaDefinition, SchemaValidator |

---

**Analysis Completed**: 2026-07-12
**Next Steps**: Implement ValidatedSchema interface or reconcile with existing SchemaDefinition
