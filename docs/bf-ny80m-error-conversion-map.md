# Error Conversion Map in Validate() Flow

## Overview

This document maps all error type conversions that occur in the Validate() flow through the ARMOR codebase, from initial error generation through final consumption by callers.

## Error Type Hierarchy

```
error (Go built-in interface)
└── YAMLError (ARMOR base interface)
    ├── ValidationError
    │   ├── TypeMismatchError
    │   ├── FieldNotFoundError
    │   ├── ConstraintError
    │   └── DuplicateKeyError
    ├── LocalValidationError (validator-internal)
    ├── SchemaError
    │   ├── SchemaLoadError
    │   └── SchemaValidationError
    ├── FileError
    ├── ParseError
    │   ├── SyntaxError
    │   └── StructureError
    └── EnhancedParseError
```

## Error Conversion Points

### Conversion Point 1: yaml.v3 Native Error → LocalValidationError

**Location:** `internal/yamlutil/validator.go:178`

**Function:** `parseYAMLError(err error, filePath, content string) LocalValidationError`

**Source Type:** `error` (from `gopkg.in/yaml.v3`)

**Target Type:** `LocalValidationError`

**Conversion Logic:**
```go
func (v *Validator) parseYAMLError(err error, filePath, content string) LocalValidationError {
    ve := LocalValidationError{
        FilePath: filePath,
        Message:  err.Error(),           // Copy error message
        Type:     ErrorTypeSyntax,       // Classify as syntax error
    }
    
    // Parse line number from error message string
    errMsg := err.Error()
    if strings.Contains(errMsg, "line ") {
        parts := strings.Split(errMsg, "line ")
        if len(parts) > 1 {
            lineStr := strings.Fields(parts[1])[0]
            var line int
            if _, err := fmt.Sscanf(lineStr, "%d", &line); err == nil {
                ve.Line = line
            }
        }
    }
    
    return ve
}
```

**Rationale:** YAML parser errors contain location information embedded in string messages. This conversion extracts structured data (line numbers) from unstructured error strings.

**Used By:**
- `Validator.ValidateStringWithPath()` - line 139

---

### Conversion Point 2: os.File Read Error → LocalValidationError

**Location:** `internal/yamlutil/validator.go:164`

**Source Type:** `error` (from `os.ReadFile`)

**Target Type:** `LocalValidationError`

**Conversion Logic:**
```go
content, err := os.ReadFile(filePath)
if err != nil {
    result.Valid = false
    ve := LocalValidationError{
        FilePath: filePath,
        Message:  fmt.Sprintf("Failed to read file: %v", err),
        Type:     ErrorTypeIO,
    }
    result.Errors = append(result.Errors, ve.ToValidationError())
    return result
}
```

**Rationale:** File I/O errors need to be converted to validation errors for consistent error reporting.

**Used By:**
- `Validator.ValidateFile()` - line 164

---

### Conversion Point 3: Empty Content Check → LocalValidationError

**Location:** `internal/yamlutil/validator.go:125`

**Source Type:** (none - generated from condition)

**Target Type:** `LocalValidationError`

**Conversion Logic:**
```go
if strings.TrimSpace(yamlContent) == "" {
    result.Valid = false
    ve := LocalValidationError{
        FilePath: filePath,
        Message:  "YAML content is empty",
        Type:     ErrorTypeEmpty,
    }
    result.Errors = append(result.Errors, ve.ToValidationError())
    return result
}
```

**Rationale:** Empty content is not an error from external systems but represents validation failure.

**Used By:**
- `Validator.ValidateStringWithPath()` - line 125

---

### Conversion Point 4: LocalValidationError → ValidationError

**Location:** `internal/yamlutil/validator.go:49`

**Function:** `ToValidationError() ValidationError`

**Source Type:** `LocalValidationError`

**Target Type:** `ValidationError`

**Conversion Logic:**
```go
func (ve LocalValidationError) ToValidationError() ValidationError {
    return ValidationError{
        FilePath:   ve.FilePath,
        Message:    ve.Message,
        ContextStr: ve.Context,
        Line:       ve.Line,
        Column:     ve.Column,
        Type:       ve.Type,
        Path:       "", // Path is context-specific and populated by caller if needed
    }
}
```

**Rationale:** LocalValidationError is an internal type with richer context; ValidationError is the public API type. This conversion hides internal implementation details.

**Used By:**
- `Validator.ValidateStringWithPath()` - line 130, 140
- `Validator.ValidateFile()` - line 169
- `Validator.checkStructuralIssues()` - line 146

---

### Conversion Point 5: SchemaDefinition Errors → SchemaValidationResult

**Location:** `internal/yamlutil/schema.go:172`

**Function:** `SchemaValidator.Validate(data interface{}) SchemaValidationResult`

**Source Type:** `error` (from `compileSchema()` or `schema.Validate()`)

**Target Type:** `SchemaValidationError` (in `SchemaValidationResult.Errors`)

**Conversion Logic:**
```go
if err := sv.compileSchema(); err != nil {
    result.Valid = false
    result.Errors = append(result.Errors, SchemaValidationError{
        Message: fmt.Sprintf("Invalid schema: %v", err),
    })
    return result
}

if err := sv.schema.Validate(data); err != nil {
    result.Valid = false
    result.Errors = append(result.Errors, SchemaValidationError{
        Message: fmt.Sprintf("Validation failed: %v", err),
    })
    return result
}
```

**Rationale:** Converts YAMLError interface types into result struct error list. Errors are stringified, losing type information.

**Used By:**
- `SchemaValidator.Validate()` - lines 172, 183
- `SchemaValidator.ValidateFile()` - lines 227, 237

---

### Conversion Point 6: File Read Error → SchemaValidationResult

**Location:** `internal/yamlutil/schema.go:227`

**Function:** `SchemaValidator.ValidateFile(filePath string) SchemaValidationResult`

**Source Type:** `error` (from `os.ReadFile`)

**Target Type:** `SchemaValidationError`

**Conversion Logic:**
```go
content, err := os.ReadFile(filePath)
if err != nil {
    result.Valid = false
    result.Errors = append(result.Errors, SchemaValidationError{
        Message: fmt.Sprintf("Failed to read file: %v", err),
    })
    return result
}
```

**Rationale:** I/O errors wrapped in schema validation result for consistent API.

---

### Conversion Point 7: YAML Parse Error → SchemaValidationResult

**Location:** `internal/yamlutil/schema.go:237`

**Function:** `SchemaValidator.ValidateFile(filePath string) SchemaValidationResult`

**Source Type:** `error` (from `yaml.Unmarshal`)

**Target Type:** `SchemaValidationError`

**Conversion Logic:**
```go
var data map[string]interface{}
if err := yaml.Unmarshal(content, &data); err != nil {
    result.Valid = false
    result.Errors = append(result.Errors, SchemaValidationError{
        Message: fmt.Sprintf("Failed to parse YAML: %v", err),
    })
    return result
}
```

**Rationale:** YAML parser errors wrapped in schema validation result.

---

### Conversion Point 8: Schema Load Errors → SchemaLoadError

**Location:** `internal/yamlutil/schema.go:586-629`

**Function:** `SchemaDefinition.LoadFromFile(filePath string) error`

**Source Type:** Various (`os.ReadFile`, `json.Unmarshal`, `yaml.Unmarshal`)

**Target Type:** `*SchemaLoadError`

**Conversion Logic:**
```go
content, err := os.ReadFile(filePath)
if err != nil {
    return NewSchemaLoadError(filePath, "", fmt.Sprintf("Failed to read schema file: %v", err), ErrCodeFileRead)
}

if err := json.Unmarshal(content, &schemaDef); err != nil {
    return NewSchemaLoadError(filePath, "json", fmt.Sprintf("Failed to parse JSON schema: %v", err), ErrCodeParse)
}

if err := yaml.Unmarshal(content, &schemaDef); err != nil {
    return NewSchemaLoadError(filePath, "yaml", fmt.Sprintf("Failed to parse YAML schema: %v", err), ErrCodeParse)
}

if err := sd.Build(); err != nil {
    return NewSchemaLoadError(filePath, "", fmt.Sprintf("Failed to build schema: %v", err), ErrCodeSchemaBuild)
}

if err := sd.Compile(); err != nil {
    return NewSchemaLoadError(filePath, "", fmt.Sprintf("Failed to compile schema: %v", err), ErrCodeSchemaCompile)
}
```

**Rationale:** Converts various I/O and parse errors into structured SchemaLoadError with consistent error codes.

---

### Conversion Point 9: String Constraint Errors → *ConstraintError

**Location:** `internal/yamlutil/schema_interfaces.go:343-393`

**Function:** `StringConstraintImpl.Validate(value interface{}) *ConstraintError`

**Source Type:** (none - generated from validation logic)

**Target Type:** `*ConstraintError`

**Conversion Logic:**
```go
// Type check failure
str, ok := value.(string)
if !ok {
    return &ConstraintError{
        Constraint:     fmt.Sprintf("value is not a string: %T", value),
        ConstraintType: "string",
        Value:          fmt.Sprintf("%v", value),
    }
}

// Min length violation
if sc.minLength > 0 && len(str) < sc.minLength {
    return &ConstraintError{
        Constraint:     fmt.Sprintf("string length %d is less than minimum %d", len(str), sc.minLength),
        ConstraintType: "min_length",
        Value:          str,
    }
}
```

**Rationale:** Direct struct initialization for constraint errors. No constructor used. Returns nil on success.

**Used By:** Called by `SchemaDefinition.Validate()` during field validation.

---

### Conversion Point 10: Number Constraint Errors → *ConstraintError

**Location:** `internal/yamlutil/schema_interfaces.go:458-503`

**Function:** `NumberConstraintImpl.Validate(value interface{}) *ConstraintError`

**Source Type:** (none - generated from validation logic)

**Target Type:** `*ConstraintError`

**Conversion Logic:**
```go
// Non-numeric value
num, err := toFloat64(value)
if err != nil {
    return &ConstraintError{
        Constraint: fmt.Sprintf("value is not a number: %v", err),
    }
}

// Min violation
if nc.min != nil && num < *nc.min {
    return &ConstraintError{
        Constraint:     fmt.Sprintf("value %v is less than minimum %v", num, *nc.min),
        ConstraintType: "minimum",
        Value:          fmt.Sprintf("%v", num),
    }
}
```

**Rationale:** Direct struct initialization. Returns nil on success.

---

### Conversion Point 11: Array Constraint Errors → *ConstraintError

**Location:** `internal/yamlutil/schema_interfaces.go:560-595`

**Function:** `ArrayConstraintImpl.Validate(value interface{}) *ConstraintError`

**Source Type:** (none - generated from validation logic)

**Target Type:** `*ConstraintError`

**Conversion Logic:**
```go
// Non-array value
arr, ok := value.([]interface{})
if !ok {
    return &ConstraintError{
        Constraint:     fmt.Sprintf("value is not an array: %T", value),
        ConstraintType: "array",
        Value:          fmt.Sprintf("%v", value),
    }
}

// Min items violation
if ac.minItems != nil && len(arr) < *ac.minItems {
    return &ConstraintError{
        Constraint:     fmt.Sprintf("array length %d is less than minimum %d", len(arr), *ac.minItems),
        ConstraintType: "min_items",
        Value:          fmt.Sprintf("%d", len(arr)),
    }
}
```

**Rationale:** Direct struct initialization. Returns nil on success.

---

### Conversion Point 12: Object Constraint Errors → *ConstraintError

**Location:** `internal/yamlutil/schema_interfaces.go:647-685`

**Function:** `ObjectConstraintImpl.Validate(value interface{}) *ConstraintError`

**Source Type:** (none - generated from validation logic)

**Target Type:** `*ConstraintError`

**Conversion Logic:**
```go
// Non-object value
obj, ok := value.(map[string]interface{})
if !ok {
    return &ConstraintError{
        Constraint:     fmt.Sprintf("value is not an object: %T", value),
        ConstraintType: "object",
        Value:          fmt.Sprintf("%v", value),
    }
}

// Missing required property
for _, reqProp := range oc.required {
    if _, exists := obj[reqProp]; !exists {
        return &ConstraintError{
            Constraint:     fmt.Sprintf("missing required property: %s", reqProp),
            ConstraintType: "required",
            Value:          fmt.Sprintf("%v", obj),
        }
    }
}
```

**Rationale:** Direct struct initialization. Returns nil on success.

---

### Conversion Point 13: Boolean/Type Constraint Errors → *ConstraintError

**Location:** `internal/yamlutil/schema_interfaces.go:746-828`

**Functions:** 
- `BooleanConstraintImpl.Validate(value interface{}) *ConstraintError`
- `TypeConstraintImpl.Validate(value interface{}) *ConstraintError`

**Source Type:** (none - generated from validation logic)

**Target Type:** `*ConstraintError`

**Conversion Logic:**
```go
// Boolean constraint
if _, ok := value.(bool); !ok {
    return &ConstraintError{
        Constraint:     fmt.Sprintf("value is not a boolean: %T", value),
        ConstraintType: "boolean",
        Value:          fmt.Sprintf("%v", value),
    }
}

// Type constraint
if !tc.typeChecker(value) {
    return &ConstraintError{
        Constraint:     fmt.Sprintf("value does not match expected type"),
        ConstraintType: "type",
        Value:          fmt.Sprintf("%v", value),
    }
}
```

**Rationale:** Direct struct initialization. Returns nil on success.

---

## Conversion Chain Examples

### Example 1: Invalid YAML File through Validator

```
1. yaml.v3 parses file
   └─> Returns *yaml.TypeError (error interface)

2. Validator.ValidateFile() 
   └─> Calls Validator.ValidateStringWithPath()
       └─> Calls yaml.Unmarshal()
           └─> Returns error

3. Validator.ValidateStringWithPath()
   └─> Detects error from Unmarshal
       └─> Calls parseYAMLError(err)
           └─> Returns LocalValidationError

4. Validator.ValidateStringWithPath()
   └─> Calls LocalValidationError.ToValidationError()
       └─> Returns ValidationError

5. Validator.ValidateStringWithPath()
   └─> Appends to ValidationResult.Errors
       └─> Returns ValidationResult
```

### Example 2: Schema Validation Failure

```
1. SchemaValidator.ValidateFile()
   └─> Reads file with os.ReadFile()

2. SchemaValidator.ValidateFile()
   └─> Parses with yaml.Unmarshal()
       └─> Returns data map

3. SchemaValidator.Validate()
   └─> Calls SchemaDefinition.Validate(data)
       └─> Validates fields
           └─> Calls StringConstraintImpl.Validate()
               └─> Returns *ConstraintError

4. SchemaDefinition.Validate()
   └─> Returns *ConstraintError (as error interface)

5. SchemaValidator.Validate()
   └─> Detects error
       └─> Creates SchemaValidationError with fmt.Sprintf("%v", err)
           └─> Loses original error type information

6. SchemaValidator.Validate()
   └─> Appends to SchemaValidationResult.Errors
       └─> Returns SchemaValidationResult
```

## Custom Error Types

### 1. LocalValidationError

**Purpose:** Internal validator error type with richer context than public ValidationError

**Fields:**
- `FilePath string`
- `Line int`
- `Column int`
- `Message string`
- `Context string`
- `Type ErrorType`

**Conversion:** Converted to ValidationError via `ToValidationError()` method

**File:** `internal/yamlutil/validator.go:14`

### 2. SchemaValidationError

**Purpose:** Schema validation result error type (distinct from ValidationError)

**Fields:**
- `FilePath string`
- `SchemaPath string`
- `FieldPath string`
- `Message string`
- `Expected string`
- `Found string`
- `Line int`
- `ErrorCode ErrorCode`

**Conversion:** Created via direct struct initialization from stringified errors

**File:** `internal/yamlutil/errors.go:1149`

### 3. ConstraintError

**Purpose:** Constraint violation error with dual usage patterns

**Fields:**
- `Constraint string` - description of constraint
- `ConstraintType string` - category (min, max, pattern, etc.)
- `Value string` - actual value
- `FilePath string` - optional
- `FieldPath string` - optional
- `Line int` - optional
- `Code ErrorCode` - optional

**Two Usage Patterns:**
1. Via `NewConstraintError()` constructor in `SchemaDefinition.Validate()`
2. Via direct `&ConstraintError{}` initialization in constraint implementations

**File:** `internal/yamlutil/errors.go:1005`

## Key Observations

### 1. Type Information Loss

When errors are converted to result struct error lists (`SchemaValidationResult`, `ValidationResult`), they are stringified via `fmt.Sprintf("%v", err)`, losing the original error type. Callers receiving these results cannot type-assert to specific error types.

**Example:**
```go
result.Errors = append(result.Errors, SchemaValidationError{
    Message: fmt.Sprintf("Validation failed: %v", err),  // Type info lost
})
```

### 2. Dual Constructor Patterns

ConstraintError uses two different construction patterns:
- Constructor pattern (`NewConstraintError()`) in `SchemaDefinition`
- Direct struct initialization in constraint implementations (`StringConstraintImpl`, etc.)

This inconsistency exists because constraint implementations predate the constructor functions.

### 3. No Error Wrapping

Validate() methods do NOT use Go's error wrapping (`fmt.Errorf` with `%w` or `errors.Wrap()`). They return clean error types directly. This means `errors.Unwrap()` will not reveal additional context.

### 4. Interface-Based Type Checking

The `YAMLError` interface enables type-based error handling:
```go
if ye, ok := err.(YAMLError); ok {
    switch ye.YAMLErrorType() {
    case ErrorTypeValidation:
        // Handle validation errors
    case ErrorTypeSyntax:
        // Handle syntax errors
    }
}
```

### 5. Conversion Entry Points

Error conversions occur at these main entry points:
1. `Validator.ValidateFile()` - I/O and YAML errors → ValidationError
2. `Validator.ValidateStringWithPath()` - YAML errors → ValidationError  
3. `SchemaValidator.Validate()` - Schema errors → SchemaValidationError
4. `SchemaValidator.ValidateFile()` - I/O/YAML errors → SchemaValidationError
5. `SchemaDefinition.LoadFromFile()` - Various errors → SchemaLoadError

## Summary Table

| Conversion Point | Source Type | Target Type | Method | Location | Type Loss |
|-----------------|-------------|-------------|---------|----------|-----------|
| yaml.v3 error → LocalValidationError | `error` | `LocalValidationError` | `parseYAMLError()` | validator.go:178 | No |
| os error → LocalValidationError | `error` | `LocalValidationError` | Direct struct init | validator.go:164 | No |
| Empty check → LocalValidationError | (none) | `LocalValidationError` | Direct struct init | validator.go:125 | No |
| LocalValidationError → ValidationError | `LocalValidationError` | `ValidationError` | `ToValidationError()` | validator.go:49 | No |
| Schema error → SchemaValidationResult | `error` | `SchemaValidationError` | String format | schema.go:172, 183 | Yes |
| File I/O → SchemaValidationResult | `error` | `SchemaValidationError` | String format | schema.go:227 | Yes |
| YAML parse → SchemaValidationResult | `error` | `SchemaValidationError` | String format | schema.go:237 | Yes |
| Schema load errors → SchemaLoadError | Various | `*SchemaLoadError` | `NewSchemaLoadError()` | schema.go:586-629 | No |
| Constraint violations → *ConstraintError | (none) | `*ConstraintError` | Direct struct init | schema_interfaces.go | No |

## Generated: 2026-07-12
