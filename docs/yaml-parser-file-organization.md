# YAML Parser Module - File Organization Plan

## Executive Summary

This document outlines the file organization and directory structure for the YAML parser module in ARMOR. The module is already located at `internal/yamlutil/` with substantial existing functionality. This plan builds upon the current architecture while proposing enhancements for better organization and maintainability.

## Current State Analysis

### Existing Structure
```
internal/yamlutil/
├── Go Files
│   ├── config.go           # Parser configuration
│   ├── debug_helpers.go   # Debug utilities
│   ├── doc.go              # Package documentation
│   ├── errors.go           # Error types
│   ├── file.go             # File I/O operations
│   ├── future.go           # Future/resolution types
│   ├── interfaces.go       # Interface definitions
│   ├── parser.go           # Core parser implementation
│   ├── result_types.go     # Result type definitions
│   ├── schema.go           # Schema validation
│   ├── template.go         # Template utilities
│   └── validator.go        # Validation logic
├── Python Files
│   ├── __init__.py
│   ├── error_types.py
│   ├── interfaces.py
│   ├── parser.py
│   ├── reader.py
│   └── validator.py
├── Test Files
│   ├── debug_helpers_test.go
│   ├── examples_test.go
│   ├── file_test.go
│   ├── integration_test.go
│   ├── parser_test.go
│   └── validator_test.go
├── Documentation
│   ├── ARCHITECTURE.md
│   ├── DATA_FLOW.md
│   ├── DATAFLOW.md
│   ├── INTERFACES.md
│   └── README_READER.md
├── Test Data
│   └── testdata/
│       ├── valid_*.yaml
│       └── invalid_*.yaml
├── Tests
│   └── tests/              # Python tests
└── __pycache__/
```

### Module Organization Patterns

1. **Single Responsibility**: Each file handles a specific concern
2. **Test Proximity**: Go tests are co-located with source files (`*_test.go`)
3. **Documentation**: Comprehensive markdown documentation alongside code
4. **Dual Implementation**: Both Go and Python implementations present
5. **Test Data Organization**: Separate `testdata/` directory for YAML fixtures

## Proposed File Organization

### Core Module Structure

```
internal/yamlutil/
├── Core Implementation
│   ├── parser.go           # Main Parser struct and basic operations
│   ├── config.go           # ParserConfig, DefaultParserConfig(), etc.
│   ├── interfaces.go       # Core interface definitions
│   ├── result_types.go     # ParseResult, ValidationResult, etc.
│   ├── errors.go           # All error types (YAMLParseError, FileError, etc.)
│   └── doc.go              # Package-level documentation and examples
│
├── File Operations
│   ├── file.go             # ReadFile, WriteFile, FileExists, FindYAMLFiles
│   └── file_test.go        # File operation tests
│
├── Field Access & Utilities
│   ├── access.go           # GetInt, GetBool, GetString, HasField, etc.
│   ├── convert.go          # Type conversion utilities
│   └── access_test.go      # Field access tests
│
├── Validation
│   ├── validator.go        # Validator struct and Validate methods
│   ├── schema.go           # Schema validation implementation
│   ├── rules.go            # Built-in validation rules
│   ├── validator_test.go   # Validation tests
│   └── schema_test.go      # Schema validation tests
│
├── Error Handling & Context
│   ├── errors.go           # Error type definitions
│   ├── context.go          # Error context and line information
│   └── helpers.go          # Debug helpers and error formatting
│
├── Advanced Features
│   ├── template.go         # YAML template processing
│   ├── merge.go            # YAML merge/overlay operations
│   ├── transform.go        # YAML transformation utilities
│   └── future.go           # Future/resolution types for async operations
│
├── Integration & Examples
│   ├── examples_test.go   # Usage examples as tests
│   └── integration_test.go # Full integration tests
│
├── Test Data
│   └── testdata/
│       ├── valid/
│       │   ├── simple.yaml
│       │   ├── nested.yaml
│       │   ├── list.yaml
│       │   ├── anchors.yaml
│       │   └── complex.yaml
│       ├── invalid/
│       │   ├── syntax_error.yaml
│       │   ├── missing_colon.yaml
│       │   ├── unmatched_bracket.yaml
│       │   └── indentation_error.yaml
│       └── edge_cases/
│           ├── empty.yaml
│           ├── whitespace_only.yaml
│           └── comments.yaml
│
├── Documentation
│   ├── ARCHITECTURE.md     # Architecture overview
│   ├── DATA_FLOW.md        # Data flow diagrams
│   ├── INTERFACES.md       # Interface documentation
│   ├── API.md              # API reference
│   └── EXAMPLES.md         # Usage examples
│
└── Python Implementation (Parallel Structure)
    ├── __init__.py
    ├── parser.py
    ├── validator.py
    ├── interfaces.py
    ├── error_types.py
    ├── reader.py
    └── tests/
        └── test_parser.py
```

## Module Visibility and Re-exports

### Public API (Exported)

```go
// Top-level constructors and utilities
func NewParser() *Parser
func NewStrictParser() *Parser
func ParseYAML(filePath string) (map[string]interface{}, error)
func ValidateYAML(content []byte) ValidationResult

// Field access helpers
func GetString(data map[string]interface{}, field string, defaultValue string) string
func GetInt(data map[string]interface{}, field string, defaultValue int) int
func GetBool(data map[string]interface{}, field string, defaultValue bool) bool
func HasField(data map[string]interface{}, field string) bool

// File operations
func ReadFile(filePath string) ([]byte, error)
func FileExists(filePath string) bool
func FindYAMLFiles(dirPath string) ([]string, error)
func FindYAMLFilesRecursive(dirPath string) ([]string, error)

// Error type checks
func IsFileNotFoundError(err error) bool
func IsPermissionError(err error) bool
func IsYAMLSyntaxError(err error) bool
```

### Internal Implementation (Unexported)

- Lower-level parsing routines
- Error line extraction utilities
- Internal type conversions
- Test helpers and fixtures

### Re-export Strategy

```go
// In doc.go or a dedicated exports.go file:

// Package yamlutil provides comprehensive YAML parsing, validation, and field access.
//
// Key types:
//   - Parser: Main parsing interface
//   - Validator: YAML validation interface
//   - ParseResult: Result of parsing operations
//   - ValidationResult: Result of validation operations
//   - YAMLParseError: Detailed YAML parsing errors
//   - FileError: File I/O errors with context
//
// Key functions:
//   - NewParser(): Create a new parser
//   - ParseYAML(): Quick parse a YAML file
//   - ReadFile(): Read a YAML file with error context
//   - GetString(), GetInt(), GetBool(): Field access helpers
```

## File Responsibilities

### Core Files

| File | Responsibility |
|------|---------------|
| `parser.go` | Parser struct, ParseFile, ParseString, ParseToMap, MustParse |
| `config.go` | ParserConfig struct, default configurations, strict mode setup |
| `interfaces.go` | ParserInterface, ValidatorInterface, and other contracts |
| `result_types.go` | ParseResult, ValidationResult, and other result types |
| `errors.go` | YAMLParseError, FileError, FieldNotFoundError, TypeMismatchError |

### File Operations

| File | Responsibility |
|------|---------------|
| `file.go` | ReadFile, WriteFile, FileExists, FindYAMLFiles, IsYAMLFile |
| `access.go` | GetString, GetInt, GetBool, HasField, ValidateRequiredFields |
| `convert.go` | Type conversions, safe casting, default value handling |

### Validation

| File | Responsibility |
|------|---------------|
| `validator.go` | Validator struct, ValidateFile, ValidateString, ValidateMultiple |
| `schema.go` | Schema parsing, validation against schemas, schema errors |
| `rules.go` | Built-in validation rules (required fields, types, ranges) |

### Testing Strategy

1. **Unit Tests**: `*_test.go` files alongside source files
2. **Integration Tests**: `integration_test.go` for end-to-end workflows
3. **Examples**: `examples_test.go` demonstrating usage patterns
4. **Test Data**: Organized in `testdata/` with valid/invalid/edge_cases subdirectories

## Module Hierarchy

```
yamlutil (package)
├── Public API
│   ├── Parser struct and methods
│   ├── Validator struct and methods
│   ├── Standalone functions (ParseYAML, ReadFile)
│   └── Field access helpers (GetString, GetInt, etc.)
├── Internal Implementation
│   ├── Error handling (errors.go, context.go)
│   ├── Type conversion (convert.go)
│   ├── Validation rules (rules.go)
│   └── Debug helpers (helpers.go)
└── Cross-cutting Concerns
    ├── Configuration (config.go)
    ├── Documentation (doc.go, *.md files)
    └── Testing (testdata/, *_test.go files)
```

## Testing Organization

### Go Tests

```
internal/yamlutil/
├── parser_test.go           # Core parser tests
├── validator_test.go        # Validator tests
├── file_test.go             # File operation tests
├── access_test.go          # Field access tests
├── integration_test.go     # End-to-end tests
└── examples_test.go        # Usage examples as tests
```

### Test Data Structure

```
testdata/
├── valid/                   # Valid YAML files
│   ├── simple.yaml
│   ├── nested.yaml
│   ├── list.yaml
│   ├── anchors.yaml
│   └── complex.yaml
├── invalid/                 # Invalid YAML files (should error)
│   ├── syntax_error.yaml
│   ├── missing_colon.yaml
│   ├── unmatched_bracket.yaml
│   └── indentation_error.yaml
└── edge_cases/              # Edge case testing
    ├── empty.yaml
    ├── whitespace_only.yaml
    └── comments.yaml
```

### Python Tests

```
internal/yamlutil/tests/
└── test_parser.py          # Python implementation tests
```

## Implementation Phases

### Phase 1: Foundation (Complete)
- [x] Core parser implementation
- [x] Basic file I/O
- [x] Error types
- [x] Field access helpers

### Phase 2: Validation (Complete)
- [x] Validator implementation
- [x] Schema validation
- [x] Validation rules

### Phase 3: Advanced Features (In Progress)
- [ ] Template processing
- [ ] YAML merge/overlay
- [ ] Transformation utilities

### Phase 4: Documentation & Examples (In Progress)
- [x] Package documentation (doc.go)
- [x] Architecture documentation
- [x] Interface documentation
- [ ] API reference
- [ ] Usage examples

### Phase 5: Testing (In Progress)
- [x] Unit tests for core functionality
- [x] Integration tests
- [x] Example tests
- [ ] Edge case coverage
- [ ] Performance benchmarks

## Migration Notes

The current `internal/yamlutil/` directory already contains substantial implementation. The proposed organization is a refactoring plan that:

1. **Respects Existing Code**: No breaking changes to the public API
2. **Improves Organization**: Better separation of concerns
3. **Enhances Testability**: Clearer test organization
4. **Maintains Compatibility**: Both Go and Python implementations remain

## Dependencies

### Go Dependencies
- `gopkg.in/yaml.v3` (already in go.mod)

### Internal Dependencies
- Standard library: `fmt`, `os`, `path/filepath`, `io`, `strings`
- No external ARMOR dependencies (standalone utility module)

## Conclusion

This file organization plan builds upon the existing `internal/yamlutil/` implementation, providing a clear structure for ongoing development while maintaining backward compatibility. The organization follows Go best practices and ARMOR's existing module patterns.

The dual Go/Python implementation is maintained for flexibility, and the testing strategy ensures comprehensive coverage of all functionality.
