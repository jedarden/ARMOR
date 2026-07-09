# YAML Parser File Organization and Directory Structure Plan

## Project Context

**Note**: The ARMOR project is a Go project, not Rust. This plan adapts the organizational principles to Go's module system and conventions.

## Current Module Location

### Primary Location: `internal/yamlutil/`

The YAML parser module is located at `/home/coding/ARMOR/internal/yamlutil/` following Go conventions:

- **Package name**: `yamlutil`
- **Import path**: `github.com/jedarden/ARMOR/internal/yamlutil`
- **Visibility**: `internal/` provides package-private visibility (Go's internal packages)

### Justification for Current Location

1. **Follows Go standard layout**: `internal/` for packages not intended for external import
2. **Clear naming**: `yamlutil` is descriptive and follows Go naming conventions
3. **Logical grouping**: Separates YAML utilities from other internal packages

## Current File Structure

```
internal/yamlutil/
├── Core Implementation
│   ├── config.go              # Parser/Validator configuration types
│   ├── errors.go              # Error type definitions
│   ├── interfaces.go          # Interface definitions (YAMLParser, YAMLValidator)
│   ├── result_types.go        # ParseResult, ValidationResult types
│   ├── template.go            # Template processing utilities
│   └── future.go              # Experimental/future features
│
├── Functional Components
│   ├── file.go                # File I/O operations
│   ├── parser.go              # Core YAML parsing logic
│   ├── validator.go           # YAML validation engine
│   ├── schema.go              # Schema validation support
│   └── debug_helpers.go       # Field access and navigation utilities
│
├── Documentation
│   ├── doc.go                 # Package-level documentation with examples
│   ├── ARCHITECTURE.md        # Architecture and design documentation
│   ├── DATA_FLOW.md           # Data flow documentation
│   ├── INTERFACES.md          # Interface documentation
│   └── README_READER.md       # Reader component documentation
│
├── Test Files
│   ├── file_test.go           # File I/O tests
│   ├── parser_test.go         # Parser tests
│   ├── validator_test.go      # Validator tests
│   ├── debug_helpers_test.go  # Field access tests
│   ├── integration_test.go   # Integration tests
│   └── examples_test.go      # Example-based tests
│
└── Test Data
    └── testdata/              # Test YAML files
        ├── simple.yaml
        ├── complex.yaml
        ├── invalid.yaml
        └── ...
```

## File Responsibility Matrix

### Core Implementation Files

| File | Primary Responsibility | Key Types/Functions |
|------|------------------------|---------------------|
| `config.go` | Configuration management | `ParserConfig`, `ValidatorConfig` |
| `errors.go` | Error type definitions | `YAMLParseError`, `FileError`, `FieldNotFoundError` |
| `interfaces.go` | Interface contracts | `YAMLParser`, `YAMLValidator`, `FileReader` |
| `result_types.go` | Result container types | `ParseResult`, `ValidationResult` |
| `template.go` | Template processing | `TemplateProcessor`, `RenderOptions` |
| `future.go` | Experimental features | Future planning and stubs |

### Functional Component Files

| File | Primary Responsibility | Key Functions |
|------|------------------------|--------------|
| `file.go` | File I/O operations | `ReadFile()`, `FileExists()`, `FindYAMLFiles()` |
| `parser.go` | YAML parsing logic | `ParseYAML()`, `ParseFile()`, `ParseString()` |
| `validator.go` | YAML validation | `ValidateFile()`, `ValidateContent()` |
| `schema.go` | Schema validation | `SchemaValidator`, `ValidateAgainstSchema()` |
| `debug_helpers.go` | Field access utilities | `GetField()`, `GetString()`, `GetInt()`, `GetBool()` |

### Documentation Files

| File | Purpose | Audience |
|------|---------|----------|
| `doc.go` | Package documentation with examples | Package users |
| `ARCHITECTURE.md` | Design decisions and architecture | Maintainers |
| `DATA_FLOW.md` | Data flow diagrams and explanations | Maintainers |
| `INTERFACES.md` | Interface documentation | API users |
| `README_READER.md` | Component-specific docs | Component users |

## Module Hierarchy

### Current Structure

```
yamlutil (package root)
├── Interfaces (abstract contracts)
│   ├── FileReader
│   ├── YAMLParser
│   └── YAMLValidator
│
├── Core Types (data structures)
│   ├── ParserConfig
│   ├── ValidatorConfig
│   ├── ParseResult
│   └── ValidationResult
│
├── Error Types (error handling)
│   ├── YAMLParseError
│   ├── FileError
│   ├── FieldNotFoundError
│   └── TypeMismatchError
│
├── Implementations (concrete logic)
│   ├── DefaultParser (implements YAMLParser)
│   ├── DefaultValidator (implements YAMLValidator)
│   └── OSFileReader (implements FileReader)
│
└── Utilities (helper functions)
    ├── Field access helpers
    ├── File discovery utilities
    └── Validation helpers
```

### Dependency Graph

```
Application Layer
       │
       ▼
Field Access Layer (debug_helpers.go)
       │
       ├──┬──────────────┐
       ▼                ▼
Validation Layer    Parsing Layer (parser.go)
(validator.go)           │
       │                 │
       └───────┬─────────┘
               ▼
       File I/O Layer (file.go)
               │
               ▼
       Operating System
```

## Module Visibility and Re-exports

### Current Visibility Strategy

```go
// Package yamlutil - all types are package-visible
// No explicit subpackages - flat structure

// Public API (exported)
type Parser struct { ... }
type Validator struct { ... }
func ParseFile(path string, data interface{}) ParseResult
func NewParser() *Parser
func NewValidator() *Validator

// Internal API (unexported)
type parserImpl struct { ... }
func parseBytes(content []byte) (interface{}, error)
```

### Re-export Strategy

**Current approach**: No re-exports needed (flat package structure)

**Potential improvement**: If functionality grows, consider subpackages:

```
internal/yamlutil/
├── parser/     # Core parsing logic
├── validator/  # Validation logic
├── io/         # File I/O operations
└── schema/     # Schema validation
```

**Re-exports in root package**:

```go
// Re-export commonly used types
package yamlutil

import (
    "github.com/jedarden/ARMOR/internal/yamlutil/parser"
    "github.com/jedarden/ARMOR/internal/yamlutil/validator"
)

// Re-exported types
type Parser = parser.Parser
type Validator = validator.Validator
type ParseResult = parser.ParseResult
```

## Test File Organization

### Current Test Organization

**Strategy**: Co-located test files with source files

```
file.go              → file_test.go
parser.go            → parser_test.go
validator.go         → validator_test.go
debug_helpers.go     → debug_helpers_test.go
```

**Advantages**:
- Easy to find tests for specific code
- Clear relationship between test and implementation
- Go toolchain automatically recognizes pattern

### Test Data Organization

```
testdata/
├── valid/
│   ├── simple.yaml
│   ├── complex.yaml
│   └── armor-debug.yaml
├── invalid/
│   ├── syntax-error.yaml
│   └── structure-error.yaml
└── edge-cases/
    ├── empty.yaml
    └── large-file.yaml
```

### Integration Tests

**File**: `integration_test.go`

**Purpose**: End-to-end testing across components

**Scenarios**:
- File → Parse → Validate → Field Access
- Error propagation across layers
- Multi-document YAML handling

### Example Tests

**File**: `examples_test.go`

**Purpose**: Documentation through executable examples

```go
func ExampleParseFile() {
    parser := NewParser()
    result := parser.ParseFile("config.yaml", &config)
    fmt.Println(result.Success)
    // Output: true
}
```

## Recommendations

### Immediate (No Changes Required)

The current organization is **excellent** and follows Go best practices:

1. ✅ Clear module location (`internal/yamlutil/`)
2. ✅ Logical file separation by responsibility
3. ✅ Good documentation coverage
4. ✅ Proper test organization
5. ✅ Consistent naming conventions

### Future Considerations (If Module Grows)

If the module significantly grows in complexity, consider:

#### 1. Subpackage Organization

```
internal/yamlutil/
├── parser/
│   ├── parser.go
│   ├── parser_test.go
│   └── config.go
├── validator/
│   ├── validator.go
│   ├── validator_test.go
│   └── config.go
├── schema/
│   ├── schema.go
│   └── schema_test.go
└── yamlutil.go (root package with re-exports)
```

#### 2. Shared Subpackage

```
internal/yamlutil/
├── parser/
├── validator/
├── schema/
└── types/ (shared types)
    ├── errors.go
    ├── results.go
    └── interfaces.go
```

#### 3. Criteria for Subpackage Extraction

Create subpackages when:

- **File count > 15** in a single directory
- **Distinct responsibilities** that can be independently versioned
- **External dependencies** specific to a component
- **Testing complexity** requiring mock implementations

### Documentation Improvements

1. **Add dependency diagram**: Show how components interact
2. **Add performance guide**: Document caching and streaming behavior
3. **Add migration guide**: For users upgrading from simpler YAML parsers
4. **Add best practices**: Common patterns for ARMOR debug file processing

## Acceptance Criteria Status

| Criterion | Status | Evidence |
|-----------|--------|----------|
| Directory structure documented | ✅ Complete | Comprehensive structure documented |
| File responsibilities assigned | ✅ Complete | Responsibility matrix created |
| Module hierarchy defined | ✅ Complete | Hierarchy and dependency graph defined |
| Re-export strategy clear | ✅ Complete | Current and future strategies documented |

## Conclusion

The current `internal/yamlutil/` organization is **well-designed and requires no immediate changes**. The module follows Go best practices with:

- Clear separation of concerns
- Logical file grouping
- Comprehensive documentation
- Proper test organization
- Consistent naming conventions

This plan serves as documentation of the current excellent structure and provides guidance for future evolution if the module grows in complexity.
