# YAML File Reader Documentation

The YAML File Reader (`reader.py`) provides comprehensive YAML file reading functionality with robust error handling, file validation, and support for both single and multi-document YAML files.

## Features

- **File Path Validation**: Checks file existence, readability, and permissions
- **Comprehensive Error Handling**: Detailed error messages with line/column information
- **Multi-Document Support**: Read YAML files with multiple documents separated by `---`
- **Batch Processing**: Read multiple files efficiently
- **Type Safety**: Returns parsed data as Python dictionaries or lists
- **Absolute Path Resolution**: Optional path resolution to absolute paths
- **Detailed Result Objects**: Structured results with success/failure information

## Installation Requirements

The reader requires PyYAML. Install it via nix-shell:

```bash
nix-shell -p python3.pkgs.pyyaml
```

## Quick Start

### Basic Usage

```python
from internal.yamlutil import read_yaml_file

# Read a YAML file
result = read_yaml_file('config.yaml')
if result.success:
    data = result.data
    print(f"Loaded {len(data)} keys")
else:
    for error in result.errors:
        print(f"Error: {error}")
```

### Simple Interface

```python
from internal.yamlutil import read_yaml_file_simple

# Simple read - returns data directly or None on failure
data = read_yaml_file_simple('config.yaml')
if data:
    print(f"Server: {data.get('server')}")
```

### Multi-Document YAML

```python
from internal.yamlutil import read_yaml_file

# Read multi-document YAML file
result = read_yaml_file('multi_doc.yaml', multi_document=True)
if result.success:
    for doc in result.data:
        print(f"Document: {doc}")
```

## API Reference

### YAMLFileReader

Main reader class with comprehensive functionality.

```python
reader = YAMLFileReader(resolve_absolute=True)
```

**Parameters:**
- `resolve_absolute` (bool): Whether to resolve paths to absolute paths (default: True)

**Methods:**

#### read_file(filepath, multi_document=False)

Read and parse a YAML file.

**Parameters:**
- `filepath` (str): Path to the YAML file
- `multi_document` (bool): Parse as multi-document YAML (default: False)

**Returns:** `YAMLReadResult` object

**Example:**
```python
reader = YAMLFileReader()
result = reader.read_file('config.yaml')
if result.success:
    print(result.data)
```

#### read_multiple_files(filepaths, multi_document=False)

Read multiple YAML files at once.

**Parameters:**
- `filepaths` (List[str]): List of YAML file paths
- `multi_document` (bool): Parse each as multi-document YAML (default: False)

**Returns:** List of `YAMLReadResult` objects

**Example:**
```python
reader = YAMLFileReader()
results = reader.read_multiple_files(['config1.yaml', 'config2.yaml'])
for result in results:
    if result.success:
        print(f"Loaded: {result.filepath}")
```

### YAMLReadResult

Structured result object for YAML file reading operations.

**Attributes:**
- `success` (bool): Whether the read operation succeeded
- `data` (Optional[Dict|List]): Parsed YAML data (None if failed)
- `errors` (List[YAMLErrorDetail]): List of critical errors
- `warnings` (List[YAMLErrorDetail]): List of warnings
- `filepath` (str): Absolute path to the file

**Methods:**

#### has_errors() -> bool

Check if any critical errors occurred.

**Example:**
```python
if result.has_errors():
    print("Reading failed!")
```

#### has_warnings() -> bool

Check if any warnings occurred.

**Example:**
```python
if result.has_warnings():
    for warning in result.warnings:
        print(f"Warning: {warning}")
```

#### get_data() -> Dict|List

Get parsed data, raising an error if read failed.

**Example:**
```python
try:
    data = result.get_data()
    print(data['key'])
except RuntimeError as e:
    print(f"Cannot get data: {e}")
```

### Convenience Functions

#### read_yaml_file(filepath, multi_document=False)

Quick read of a single YAML file.

**Parameters:**
- `filepath` (str): Path to the YAML file
- `multi_document` (bool): Parse as multi-document YAML (default: False)

**Returns:** `YAMLReadResult` object

**Example:**
```python
from internal.yamlutil import read_yaml_file
result = read_yaml_file('config.yaml')
```

#### read_yaml_file_simple(filepath) -> Optional[Dict|List]

Simple read that returns data directly or None on failure.

**Parameters:**
- `filepath` (str): Path to the YAML file

**Returns:** Parsed YAML data or None

**Example:**
```python
from internal.yamlutil import read_yaml_file_simple
data = read_yaml_file_simple('config.yaml')
if data:
    print(data)
```

## Error Handling

### File Path Errors

The reader validates file paths and provides detailed error messages:

- **File Not Found**: When the specified file doesn't exist
- **Not a File**: When the path points to a directory
- **Not Readable**: When file permissions prevent reading
- **Empty Path**: When an empty path is provided

### YAML Parsing Errors

Parsing errors include line/column information and helpful suggestions:

- **Syntax Errors**: Invalid YAML syntax
- **Indentation Errors**: Incorrect indentation
- **Structure Errors**: Invalid YAML structure
- **Flow Style Errors**: Issues with { } or [ ] syntax

**Example Error:**
```python
result = read_yaml_file('invalid.yaml')
if not result.success:
    for error in result.errors:
        print(f"Line {error.line}: {error.message}")
        print(f"Suggestion: {error.suggestion}")
```

## Usage Patterns

### Pattern 1: Safe File Reading

```python
from internal.yamlutil import read_yaml_file

result = read_yaml_file('config.yaml')
if result.success:
    config = result.data
    # Process configuration
else:
    # Handle errors
    for error in result.errors:
        print(f"Error: {error}")
```

### Pattern 2: Batch Configuration Loading

```python
from internal.yamlutil import YAMLFileReader

reader = YAMLFileReader()
configs = ['app1.yaml', 'app2.yaml', 'app3.yaml']
results = reader.read_multiple_files(configs)

for result in results:
    if result.success:
        print(f"Loaded: {result.filepath}")
        # Use result.data
    else:
        print(f"Failed: {result.filepath}")
```

### Pattern 3: Multi-Document Processing

```python
from internal.yamlutil import read_yaml_file

result = read_yaml_file('logs.yaml', multi_document=True)
if result.success:
    for doc in result.data:
        print(f"Timestamp: {doc['timestamp']}")
        print(f"Level: {doc['level']}")
```

### Pattern 4: Error-Tolerant Processing

```python
from internal.yamlutil import YAMLFileReader

reader = YAMLFileReader()
results = reader.read_multiple_files(file_paths)

successful = [r for r in results if r.success]
failed = [r for r in results if not r.success]

print(f"Loaded {len(successful)} files, {len(failed)} failed")

for result in failed:
    print(f"Failed: {result.filepath}")
    for error in result.errors:
        print(f"  - {error.message}")
```

## YAML Format Support

### Supported YAML Features

- **Key-Value Pairs**: Simple mapping
- **Nested Structures**: Multi-level nesting
- **Lists/Arrays**: Sequence data
- **Flow Style**: { } and [ ] syntax
- **Multi-Doc**: Documents separated by `---`
- **Data Types**: Strings, integers, floats, booleans, null
- **Comments**: # prefixed comments

### Example YAML Structures

**Simple Configuration:**
```yaml
server:
  host: localhost
  port: 8080
  ssl: true

database:
  host: db.example.com
  port: 5432
```

**Complex Structure:**
```yaml
services:
  - name: web
    port: 8080
    endpoints:
      - /api
      - /health
  - name: db
    port: 5432

features:
  - authentication
  - caching
  - monitoring
```

**Multi-Document:**
```yaml
---
name: document1
version: 1.0
---
name: document2
version: 2.0
```

## Integration with ARMOR

The YAML reader integrates seamlessly with ARMOR's configuration management:

```python
from internal.yamlutil import read_yaml_file

# Read ARMOR debug configuration
result = read_yaml_file('/var/lib/armor/debug.yaml')
if result.success:
    debug_config = result.data
    # Access debug configuration
    log_level = debug_config.get('log_level', 'info')
    debug_mode = debug_config.get('debug_mode', False)
else:
    # Handle configuration errors
    print(f"Failed to load debug config: {result.errors}")
```

## Performance Considerations

- **File Reading**: Entire file is read into memory
- **Parsing**: PyYAML parses into native Python data structures
- **Memory Usage**: ~2x file size (raw content + parsed structure)
- **Recommendation**: For large files (>10MB), consider streaming parsers

## Security Considerations

- **Path Validation**: Paths are resolved to prevent directory traversal
- **Safe Loading**: Uses `yaml.safe_load()` to prevent code execution
- **Permission Checking**: Verifies file readability before parsing
- **Input Validation**: Empty files and invalid formats are detected

## Troubleshooting

### Common Issues

**Issue**: `ModuleNotFoundError: No module named 'yaml'`
- **Solution**: Install PyYAML via `nix-shell -p python3.pkgs.pyyaml`

**Issue**: File not found errors
- **Solution**: Check file path exists and is readable

**Issue**: Parsing errors with line numbers
- **Solution**: Use error suggestions to fix YAML syntax

### Debug Tips

1. **Use Detailed Errors**: Check error messages and line numbers
2. **Validate First**: Use `validate_yaml_file()` before processing
3. **Check Permissions**: Ensure files are readable
4. **Test Simple Cases**: Start with basic YAML structures

## Examples

See `examples/yaml_reader_usage.py` for comprehensive usage examples including:
- Basic file reading
- Simple interface usage
- Error handling
- Multi-document support
- Batch processing
- Result methods

## Testing

Run the test suite:

```bash
nix-shell -p python3.pkgs.pyyaml python3.pkgs.pytest --run \
    "python -m pytest tests/yamlutil/test_reader.py -v"
```

All 25 tests should pass, covering:
- File path validation
- YAML parsing
- Multi-document support
- Error handling
- Batch processing
- Result methods

## Version History

- **1.0.0** (2026-07-09): Initial implementation
  - Core file reading functionality
  - File path validation
  - Multi-document support
  - Comprehensive error handling
  - Test suite with 25 tests