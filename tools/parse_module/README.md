# YAML Parser Utility Module

A simple, safe YAML parser utility module with proper error handling and structured result objects.

## Features

- **Safe parsing**: Uses `yaml.safe_load()` to prevent execution of arbitrary Python objects
- **Error handling**: Comprehensive error handling with descriptive messages
- **Structured results**: Consistent result structure with `status`, `data`, and `error` fields
- **File and string parsing**: Support for parsing both YAML strings and files
- **Type safety**: Uses dataclasses for structured, type-safe results

## Installation

Requires PyYAML:

```bash
pip install pyyaml
```

## Usage

### Basic Usage

```python
from yaml_parser import YAMLParser, ParseResult

parser = YAMLParser()

# Parse a YAML file
result = parser.parse_file('config.yaml')

if result.is_success():
    config = result.data
    print(f"Config loaded: {config}")
else:
    print(f"Error: {result.error}")
```

### Parse String Content

```python
yaml_content = """
database:
  host: localhost
  port: 5432
  name: mydb
"""

result = parser.parse_string(yaml_content)

if result.is_success():
    db_config = result.data['database']
    print(f"Database: {db_config['name']}")
```

### Error Handling

```python
result = parser.parse_file('nonexistent.yaml')

if result.is_error():
    print(f"Failed to parse: {result.error}")
    # Error: File not found: nonexistent.yaml
```

### Result Structure

All parsing operations return a `ParseResult` object with the following fields:

- `status`: Either `'success'` or `'error'`
- `data`: Parsed YAML content (Python dict/list/etc.) - `None` if error occurred
- `error`: Error message string - `None` if successful

Helper methods:
- `is_success()`: Returns `True` if parsing was successful
- `is_error()`: Returns `True` if parsing resulted in an error

## API Reference

### `YAMLParser`

Main parser class for handling YAML content.

#### Methods

- `parse_string(yaml_content: str) -> ParseResult`: Parse YAML from a string
- `parse_file(filepath: str) -> ParseResult`: Parse YAML from a file path

### `ParseResult`

Dataclass containing the result of parsing operations.

#### Attributes

- `status: str`: Status of the parse operation ('success' or 'error')
- `data: Optional[Any]`: Parsed YAML content (None if error)
- `error: Optional[str]`: Error message (None if success)

#### Methods

- `is_success() -> bool`: Check if parsing was successful
- `is_error() -> bool`: Check if parsing resulted in an error

## Examples

### Configuration File Parsing

```python
from yaml_parser import YAMLParser

parser = YAMLParser()
result = parser.parse_file('app_config.yaml')

if result.is_success():
    config = result.data
    app_name = config.get('app_name')
    debug_mode = config.get('debug', False)
    
    print(f"App: {app_name}, Debug: {debug_mode}")
else:
    print(f"Configuration error: {result.error}")
```

### Multi-Document Handling

Note: This parser uses `safe_load()` which loads a single YAML document. For multi-document streams, consider using the more advanced parsers in `scripts/debug-config-parser/parsers/`.

### Error Recovery

```python
from yaml_parser import YAMLParser

parser = YAMLParser()

configs = ['config1.yaml', 'config2.yaml', 'config3.yaml']
results = []

for config_file in configs:
    result = parser.parse_file(config_file)
    results.append((config_file, result))

# Process successful configs
for config_file, result in results:
    if result.is_success():
        print(f"✓ {config_file}: Loaded successfully")
    else:
        print(f"✗ {config_file}: {result.error}")
```

## Testing

Run the unit tests:

```bash
cd tools/parse_module
python -m pytest tests/test_yaml_parser.py -v
```

Or run directly:

```bash
python tests/test_yaml_parser.py
```

## Error Messages

The parser provides descriptive error messages for common issues:

- **File not found**: "File not found: <path>"
- **Empty content**: "Empty YAML content"
- **Syntax errors**: "YAML syntax error: <details>"
- **Structure errors**: "YAML structure error: <details>"
- **Indentation errors**: "YAML syntax error: <details> - Check indentation and structure"

## Related Modules

For more advanced YAML parsing features, consider using the parsers in:
- `scripts/debug-config-parser/parsers/yaml_parser.py` - Advanced multi-document YAML parsing with detailed error categorization

## License

This module is part of the ARMOR project.
