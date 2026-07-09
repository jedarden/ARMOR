# YAML Syntax Validation

Comprehensive YAML syntax validation with detailed error detection, categorization, and reporting.

## Overview

The YAML validation module (`internal/yamlutil`) provides enterprise-grade YAML parsing and validation with:

- **Detailed error detection** - Line/column numbers for precise error location
- **Error categorization** - Errors grouped by type (indentation, syntax, structure, etc.)
- **Human-readable messages** - Clear explanations with helpful suggestions
- **Pre-validation checks** - Detection of common issues (tabs, trailing whitespace)
- **Severity levels** - Critical, Error, Warning, Info
- **Multi-document support** - Handles multi-document YAML files

## Installation

The module requires PyYAML. On NixOS systems:

```bash
nix-shell -p python3.pkgs.pyyaml
```

Or install via pip:

```bash
pip install pyyaml
```

## Quick Start

### Basic Validation

```python
from internal.yamlutil import validate_yaml_string, validate_yaml_file

# Validate YAML string
result = validate_yaml_string("key: value\nnested:\n  item: value")
if result.is_valid:
    print("Valid YAML!")
else:
    for error in result.errors:
        print(error)

# Validate YAML file
result = validate_yaml_file('config.yaml')
```

### Detailed Error Information

```python
from internal.yamlutil import YAMLSyntaxValidator

validator = YAMLSyntaxValidator()
result = validator.validate_file('config.yaml')

if not result.is_valid:
    for error in result.errors:
        print(f"Category: {error.category.value}")
        print(f"Severity: {error.severity.value}")
        print(f"Location: Line {error.line}, Column {error.column}")
        print(f"Message: {error.message}")
        print(f"Suggestion: {error.suggestion}")
```

## Error Categories

Errors are categorized into the following types:

| Category | Description | Example |
|----------|-------------|---------|
| `SYNTAX` | General YAML syntax errors | Unclosed quotes, invalid characters |
| `INDENTATION` | Indentation-related errors | Tabs instead of spaces, inconsistent indentation |
| `STRUCTURE` | YAML structure errors | Invalid block structures |
| `SCALAR` | Scalar value errors | String parsing issues |
| `FLOW` | Flow collection errors | Unclosed `{}`, `[]` brackets |
| `TAG` | Custom tag errors | Invalid tag syntax |
| `ANCHOR` | Anchor definition errors | Invalid `&anchor` syntax |
| `ALIAS` | Alias reference errors | Undefined `*alias` references |
| `DOCUMENT` | Document-level errors | Multi-document separation issues |
| `UNKNOWN` | Unclassified errors | Generic errors |

## Severity Levels

- **CRITICAL** - File cannot be parsed at all
- **ERROR** - Major issue preventing correct parsing
- **WARNING** - Minor issue that doesn't prevent parsing
- **INFO** - Informational message

## Examples

### Valid YAML

```yaml
config:
  name: my-app
  version: 1.0.0
  features:
    - authentication
    - logging
```

**Result:** `✓ Valid YAML`

### Tab Character Error

```yaml
key:\tvalue
nested:
\titem: value
```

**Result:**
```
✗ Invalid YAML - 3 error(s)

Line 1, Column 5 [INDENTATION_ERROR] (ERROR): Tab character found in YAML
  Context: key:	value
  Suggestion: Replace tabs with spaces (typically 2 spaces per indentation level)

Line 2, Column 1 [INDENTATION_ERROR] (ERROR): Tab character found in YAML
  Context: 	nested:
  Suggestion: Replace tabs with spaces (typically 2 spaces per indentation level)
```

### Indentation Error

```yaml
key: value
  bad_indent: true
```

**Result:**
```
✗ Invalid YAML - 1 error(s)

Line 2, Column 3 [INDENTATION_ERROR] (ERROR): mapping values are not allowed here
  Suggestion: Ensure all indentation uses spaces, not tabs, and is consistent
```

### Unclosed Flow Collection

```yaml
mapping: {key: value
array: [item1, item2
```

**Result:**
```
✗ Invalid YAML - 2 error(s)

Line 1, Column 20 [FLOW_ERROR] (ERROR): expected ',' or '}', but got '<stream end>'
  Suggestion: Ensure { } and [ ] brackets are properly matched and closed

Line 2, Column 20 [FLOW_ERROR] (ERROR): expected ',' or ']', but got '<stream end>'
  Suggestion: Ensure { } and [ ] brackets are properly matched and closed
```

## API Reference

### YAMLSyntaxValidator

Main validator class for comprehensive YAML validation.

#### Methods

- `validate_file(filepath: str) -> YAMLValidationResult` - Validate a YAML file
- `validate_content(content: str, source: str = "<string>") -> YAMLValidationResult` - Validate YAML content string
- `validate_multiple_files(filepaths: List[str]) -> List[YAMLValidationResult]` - Validate multiple files

### YAMLValidationResult

Result object containing validation outcome.

#### Attributes

- `is_valid: bool` - Whether YAML is valid
- `errors: List[YAMLErrorDetail]` - List of errors
- `warnings: List[YAMLErrorDetail]` - List of warnings

#### Methods

- `has_errors() -> bool` - Check if there are any errors
- `has_warnings() -> bool` - Check if there are any warnings
- `get_all_issues() -> List[YAMLErrorDetail]` - Get all issues (errors + warnings)

### YAMLErrorDetail

Detailed error information.

#### Attributes

- `category: YAMLErrorCategory` - Error category
- `severity: YAMLErrorSeverity` - Error severity
- `line: Optional[int]` - Line number (1-indexed)
- `column: Optional[int]` - Column number (1-indexed)
- `message: str` - Error message
- `context: str` - Context around the error
- `suggestion: str` - Helpful suggestion for fixing

## Testing

Run the functional tests:

```bash
nix-shell -p python3.pkgs.pyyaml --run "python3 tests/yamlutil/validate_yaml_functional.py"
```

Run the examples:

```bash
nix-shell -p python3.pkgs.pyyaml --run "python3 tests/yamlutil/examples.py"
```

## Integration with Existing Parsers

The new validation module can be used alongside existing YAML parsers in the codebase:

```python
from scripts.debug_config_parser.parsers import YAMLParser
from internal.yamlutil import YAMLSyntaxValidator

# Use existing parser for data extraction
parser = YAMLParser()
parse_result = parser.parse_file('config.yaml')

# Use new validator for detailed error checking
validator = YAMLSyntaxValidator()
validation_result = validator.validate_file('config.yaml')

if validation_result.is_valid:
    # Process the parsed data
    data = parse_result.data
else:
    # Display detailed error information
    for error in validation_result.errors:
        print(f"Line {error.line}: {error.message}")
        print(f"Suggestion: {error.suggestion}")
```

## Error Handling Patterns

### Common Error Types

1. **Tab Character Errors** - Replace tabs with spaces
2. **Indentation Errors** - Use consistent indentation (2 or 4 spaces)
3. **Flow Collection Errors** - Ensure brackets are properly matched
4. **Unclosed Quotes** - Terminate all string quotes
5. **Undefined Aliases** - Define anchors before referencing them

### Error Resolution

Each error includes:
- **Location** - Precise line and column
- **Category** - Type of error for filtering/grouping
- **Message** - Clear description of the problem
- **Suggestion** - Actionable advice for fixing

## Performance Considerations

- **Pre-validation checks** run before full parsing for quick feedback
- **Multi-document parsing** attempts `safe_load_all` first for efficiency
- **Context extraction** is limited to relevant lines around errors

## Future Enhancements

Potential improvements:
- Schema validation (YAML Schema, JSON Schema for YAML)
- Custom rule validation (e.g., naming conventions)
- Performance profiling for large files
- Integration with linting tools
- YAML 1.2 support (currently uses PyYAML default)