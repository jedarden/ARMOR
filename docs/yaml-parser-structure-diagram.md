# YAML Parser Module - Quick Reference

## Directory Structure Visualization

```
internal/yamlutil/
│
├── 📄 Core Implementation
│   ├── parser.go          ⚙️  Main Parser struct
│   ├── config.go          🔧 Configuration
│   ├── interfaces.go      📋 Interface definitions
│   ├── result_types.go    📊 Result types
│   └── doc.go             📖 Package docs
│
├── 📁 File Operations
│   ├── file.go            📂 Read/Write/Find files
│   └── file_test.go       ✅ File tests
│
├── 🔍 Field Access
│   ├── access.go          🗝️  GetString/GetInt/GetBool
│   ├── convert.go         🔀 Type conversions
│   └── access_test.go     ✅ Access tests
│
├── ✔️  Validation
│   ├── validator.go       📐 Validator implementation
│   ├── schema.go          📏 Schema validation
│   ├── rules.go           📜 Validation rules
│   ├── validator_test.go  ✅ Validator tests
│   └── schema_test.go     ✅ Schema tests
│
├── ⚠️  Error Handling
│   ├── errors.go          💥 Error types
│   ├── context.go         📍 Error context
│   └── helpers.go         🛠️  Debug helpers
│
├── 🚀 Advanced Features
│   ├── template.go        📝 YAML templates
│   ├── merge.go           🔀 YAML merging
│   └── transform.go       🔄 YAML transformation
│
├── 🧪 Tests
│   ├── examples_test.go   📚 Usage examples
│   └── integration_test.go 🔗 Integration tests
│
├── 📊 Test Data
│   └── testdata/
│       ├── valid/         ✅ Valid YAML files
│       ├── invalid/       ❌ Invalid YAML files
│       └── edge_cases/    🎯 Edge case files
│
├── 📖 Documentation
│   ├── ARCHITECTURE.md    🏗️  Architecture overview
│   ├── DATA_FLOW.md       🌊 Data flow diagrams
│   ├── INTERFACES.md      📋 Interface docs
│   ├── API.md             📡 API reference
│   └── EXAMPLES.md        💡 Usage examples
│
└── 🐍 Python (parallel structure)
    ├── __init__.py
    ├── parser.py
    ├── validator.py
    └── tests/
```

## Key Public API Functions

### Parsing
```go
parser := NewParser()
result := parser.ParseFile("config.yaml", &config)
data, err := ParseYAML("config.yaml")
```

### Field Access
```go
GetString(data, "server.host", "localhost")
GetInt(data, "server.port", 8080)
GetBool(data, "debug.enabled", false)
HasField(data, "database.connection")
```

### Validation
```go
validator := NewValidator()
result := validator.ValidateFile("config.yaml")
```

### File Operations
```go
content, err := ReadFile("config.yaml")
exists := FileExists("config.yaml")
files, err := FindYAMLFiles("/etc/app")
```

## File Responsibility Matrix

| File | Primary Responsibility | Key Types/Functions |
|------|----------------------|---------------------|
| `parser.go` | Core parsing | `Parser`, `ParseFile`, `ParseString` |
| `config.go` | Configuration | `ParserConfig`, `DefaultParserConfig` |
| `interfaces.go` | Contracts | `ParserInterface`, `ValidatorInterface` |
| `result_types.go` | Results | `ParseResult`, `ValidationResult` |
| `errors.go` | Error types | `YAMLParseError`, `FileError` |
| `file.go` | File I/O | `ReadFile`, `FileExists`, `FindYAMLFiles` |
| `access.go` | Field access | `GetString`, `GetInt`, `GetBool` |
| `validator.go` | Validation | `Validator`, `ValidateFile` |
| `schema.go` | Schema validation | Schema parsing and validation |

## Module Dependencies

```
yamlutil (standalone module)
│
├── External Dependencies
│   └── gopkg.in/yaml.v3
│
├── Standard Library
│   ├── fmt
│   ├── os
│   ├── path/filepath
│   ├── io
│   └── strings
│
└── Internal Dependencies
    └── (none - standalone utility)
```

## Testing Strategy

```
Testing Pyramid
│
├── Unit Tests (*_test.go)
│   ├── parser_test.go
│   ├── validator_test.go
│   ├── file_test.go
│   └── access_test.go
│
├── Integration Tests
│   └── integration_test.go
│
└── Example Tests
    └── examples_test.go
```

## Quick Start Guide

### 1. Basic Parsing
```go
import "github.com/jedarden/armor/internal/yamlutil"

// Parse into a map
data, err := yamlutil.ParseYAML("config.yaml")

// Parse into a struct
parser := yamlutil.NewParser()
var config Config
result := parser.ParseFile("config.yaml", &config)
if !result.Success {
    log.Fatal(result.Error)
}
```

### 2. Field Access
```go
// Get with defaults
host := yamlutil.GetString(data, "server.host", "localhost")
port := yamlutil.GetInt(data, "server.port", 8080)

// Check existence
if yamlutil.HasField(data, "database.connection") {
    conn := yamlutil.GetString(data, "database.connection", "")
}
```

### 3. Validation
```go
validator := yamlutil.NewValidator()
result := validator.ValidateFile("config.yaml")

if result.HasErrors() {
    for _, err := range result.Errors {
        fmt.Printf("Line %d: %s\n", err.Line, err.Message)
    }
}
```

### 4. File Operations
```go
// Check existence
if yamlutil.FileExists("config.yaml") {
    content, err := yamlutil.ReadFile("config.yaml")
}

// Find YAML files
files, err := yamlutil.FindYAMLFilesRecursive("/etc/app")
```

## Module Characteristics

- **Standalone**: No dependencies on other ARMOR modules
- **Dual Implementation**: Go and Python versions
- **Well Tested**: Comprehensive unit and integration tests
- **Documented**: Extensive inline and external documentation
- **Production Ready**: Error handling and edge case coverage
- **Flexible**: Supports both typed and generic parsing
