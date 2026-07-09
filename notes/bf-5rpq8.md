# YAML Parser Error Handling - Verification (bf-5rpq8)

## Summary
Verified that comprehensive YAML parser error handling is implemented in `internal/yamlutil/`.

## Implementation Location
- `internal/yamlutil/error_types.py` - Custom exception classes
- `internal/yamlutil/reader.py` - File reading with error handling
- `internal/yamlutil/validator.py` - YAML validation with error detection

## Acceptance Criteria Verification

### 1. FileNotFoundError handled with clear message ✓
```python
from internal.yamlutil import read_yaml_file

result = read_yaml_file('/nonexistent/file.yaml')
if not result.success:
    exc = result.to_exception()
    # exc is YAMLFileNotFoundError with:
    # - filepath: '/nonexistent/file.yaml'
    # - message: "File not found: /nonexistent/file.yaml"
    # - context and suggestion for fixing
```

### 2. YAML syntax errors caught and reported ✓
```python
import tempfile
with tempfile.NamedTemporaryFile(mode='w', suffix='.yaml') as f:
    f.write('key:\n  value\n    bad_indent: true\n')
    temp_path = f.name

result = read_yaml_file(temp_path)
if not result.success:
    exc = result.to_exception()
    # exc is YAMLSyntaxError with:
    # - filepath: temp_path
    # - line: 3 (exact line of error)
    # - column: column number
    # - context: the lines around the error
    # - suggestion: how to fix
```

### 3. Custom exception classes defined ✓
The following exception classes are implemented:
- `YAMLParserError` - Base class for all YAML errors
- `YAMLFileNotFoundError` - File not found or not readable
- `YAMLSyntaxError` - YAML syntax errors
- `YAMLStructureError` - YAML structure errors
- `YAMLValidationError` - Schema validation errors
- `YAMLEmptyFileError` - Empty YAML file

All inherit from `YAMLParserError` and include:
- `message` - Human-readable error description
- `filepath` - Path to the file that caused the error
- `line` - Line number where the error occurred
- `column` - Column number where the error occurred

### 4. Error messages include file path and line number when available ✓
```python
try:
    result = read_yaml_file('config.yaml')
    result.raise_if_error()
except YAMLSyntaxError as e:
    print(str(e))
    # Output: "File: config.yaml | Line 5, Column 3: Indentation error"
    #         "Context: ..."
    #         "Suggestion: ..."
```

## Error Categories
The implementation categorizes errors into:
- SYNTAX - General syntax errors
- INDENTATION - Indentation issues
- STRUCTURE - Structural problems
- SCALAR - Scalar value errors
- FLOW - Flow collection errors ({})
- TAG - Tag handle errors
- ANCHOR - Anchor definition errors
- ALIAS - Alias reference errors
- DOCUMENT - Document-level errors

## Usage Examples

### Pattern 1: Result-based error handling
```python
result = read_yaml_file('config.yaml')
if result.success:
    data = result.data
else:
    for error in result.errors:
        print(f"Error: {error}")
```

### Pattern 2: Exception-based error handling
```python
result = read_yaml_file('config.yaml')
if not result.success:
    raise result.to_exception()
```

### Pattern 3: Direct exception raising
```python
result = read_yaml_file('config.yaml')
result.raise_if_error()  # Raises appropriate exception if failed
data = result.data  # Safe to use
```

## Tests
Comprehensive tests exist in:
- `tests/yamlutil/test_exceptions.py` - Exception class tests
- `tests/yamlutil/test_reader.py` - Reader functionality tests
- `tests/yamlutil/test_validator.py` - Validator functionality tests

## Verification Date
2026-07-09

## Status
✅ All acceptance criteria met and verified.
