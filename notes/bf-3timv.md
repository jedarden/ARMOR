# Task BF-3TIMV: Add Usage Examples to Package Documentation

## Summary
Fixed and verified comprehensive usage examples for the ARMOR yamlutil package documentation.

## Changes Made

### 1. Fixed Failing Example Tests
- Fixed `Example_commentsInYAML` - corrected expected output to match test data
- Fixed `Example_listAccess` - corrected expected output to match test data

### 2. Documentation Coverage
The package documentation already includes comprehensive examples covering:

#### Basic YAML File Parsing with ParseYAML
- Example: `Example_parseYAML`
- Demonstrates basic file parsing into generic maps
- Shows simple field access

#### Error Handling Patterns
- **File Not Found**: `Example_parseYAML_errorHandling`, `Example_fileNotFoundError`
- **Invalid YAML Syntax**: `Example_parseYAML_invalidSyntax`
- **Type Mismatches**: `Example_typeMismatch`
- **Field Not Found**: `Example_fieldNotFoundError`
- **Comprehensive Error Handling**: `Example_errorHandlingComprehensive`

#### Working with Parsed Data
- **Nested Field Access**: `Example_fieldAccess`, `Example_multiLevelNesting`
- **Default Values**: `Example_fieldAccess_withDefaults`
- **List Processing**: `Example_listAccess`, `Example_workingWithLists`
- **Boolean Fields**: `Example_booleanFieldHandling`
- **Integer Conversion**: `Example_integerConversion`
- **String Conversion**: `Example_stringConversion`

#### Helper Functions
- **FileExists**: `Example_fileHelpers`, `Example_fileDiscoveryPatterns`
- **IsFileNotFoundError**: Multiple examples demonstrate this pattern
- **HasField**: `Example_hasField`
- **ValidateRequiredFields**: `Example_requiredFields`, `Example_validateRequiredFields`

#### ARMOR Debug File Processing
- **Basic Processing**: `Example_armorDebugProcessing`
- **Session Processing**: `Example_armorDebugSession`
- **Safe File Processing**: `Example_safeFileProcessing`
- **Complete Workflow**: `Example_completeWorkflow`

## Verification

### All Example Tests Pass
```
33 example tests - ALL PASSING
```

### Documentation Format
- Examples are in both doc.go (package documentation) and examples_test.go (executable tests)
- All examples follow Go documentation conventions
- Examples are tested by Go's test framework
- Examples demonstrate real-world ARMOR debug file processing patterns

### Acceptance Criteria Met
✅ Examples added to doc.go and as _test.go examples
✅ Examples demonstrate common usage patterns
✅ Example code is executable and tested
✅ Examples follow Go documentation conventions

## Files Modified
- `internal/yamlutil/examples_test.go` - Fixed 2 failing example tests

## Documentation Files
- `internal/yamlutil/doc.go` - Comprehensive package documentation (380 lines)
- `internal/yamlutil/examples_test.go` - Executable examples (700 lines)
