# Debug Configuration File Parser Infrastructure

Comprehensive parsing infrastructure for debug configuration files in the ARMOR codebase. Supports YAML, JSON, and TOML formats with extensible architecture.

## Features

- **Multi-format Support**: YAML (multi-document), JSON, TOML
- **Syntax Validation**: Comprehensive error detection and reporting
- **Modular Design**: Separate parsers for each file type
- **Factory Pattern**: Automatic parser selection based on file extension
- **Error Handling**: Detailed error messages with line/column information
- **Batch Processing**: Efficient validation of multiple files

## Installation

The parser requires Python 3.7+ and PyYAML. Install dependencies:

```bash
# Via nix-shell (recommended)
nix-shell -p python3.pkgs.pyyaml

# Via pip (fallback)
pip install pyyaml
```

## Usage

### Command Line Validation

Validate all debug configuration files in the workspace:

```bash
./scripts/debug-config-parser/validate_debug_configs.py
```

Validate specific files:

```bash
./scripts/debug-config-parser/validate_debug_configs.py --files path1.yaml path2.json
```

Specify a different workspace:

```bash
./scripts/debug-config-parser/validate_debug_configs.py --workspace /path/to/armor
```

### Programmatic Usage

```python
from scripts.debug_config_parser.parsers import ParserFactory

# Create parser factory
factory = ParserFactory()

# Parse a single file
result = factory.parse_file('config.yaml')
print(f"Status: {result['status']}")
print(f"Data: {result.get('data')}")

# Validate multiple files
results = factory.batch_validate(['config1.yaml', 'config2.json'])
print(f"Total: {results['total_files']}, Errors: {results['errors']}")
```

## Architecture

### Parser Classes

- **`YAMLParser`**: Handles YAML configuration files
  - Multi-document YAML support
  - Custom format detection
  - Detailed syntax error reporting

- **`JSONParser`**: Handles JSON configuration files
  - Standard JSON syntax validation
  - Line/column error reporting

- **`TOMLParser`**: Handles TOML configuration files
  - TOML syntax validation
  - Graceful dependency handling

- **`ParserFactory`**: Parser management
  - Automatic file type detection
  - Parser selection and instantiation
  - Batch validation

### File Type Detection

The factory automatically detects file types based on extension:
- `.yaml`, `.yml` → YAML
- `.json` → JSON  
- `.toml` → TOML

## Error Reporting

All parsers provide detailed error information:

```python
result = factory.parse_file('config.yaml')
if result['status'] == 'error':
    print(f"Error: {result['error']}")
```

Error messages include:
- File type and location
- Specific syntax issues
- Line/column numbers (when available)
- Suggestions for fixing common issues

## Configuration Discovery

The validator automatically discovers configuration files by:
- Scanning workspace recursively
- Matching file patterns: `*.yaml`, `*.yml`, `*.json`, `*.toml`
- Excluding common directories: `.git`, `.beads`, `target`, `node_modules`, `logs`

## Validation Results

Validation produces a comprehensive report:

```
Debug Configuration File Validator
======================================================================
Workspace: /home/coding/ARMOR
Files discovered: 35
Pattern matches: *.yaml, *.yml, *.json, *.toml
Excluded directories: .git, .beads, target, node_modules, logs

✓ .needle.yaml (YAML) (1 document)
✓ pluck-config.yaml (YAML) (1 document)
✓ .env.pluck-debug (YAML)
  ⚠ Warning: File is empty

======================================================================
VALIDATION SUMMARY
======================================================================
Total files:    35
Successful:     33
Warnings:       2
Errors:         0

✓ All configuration files are valid!
```

## Extensibility

### Adding New Parsers

To add support for a new file format:

1. Create a new parser class in `parsers/new_format_parser.py`
2. Implement `parse_file()` and `validate_syntax()` methods
3. Add file extension mapping to `ParserFactory._extension_map`
4. Update the FileType enum

Example:

```python
class INIParser:
    def parse_file(self, filepath: str):
        # Implementation
        pass
```

## Testing

Test the parser infrastructure:

```bash
# Test with sample files
./scripts/debug-config-parser/validate_debug_configs.py --files test.yaml

# Full workspace validation
./scripts/debug-config-parser/validate_debug_configs.py
```

## Dependencies

- **Python**: 3.7+
- **PyYAML**: Required for YAML parsing
- **tomli**: Optional for TOML parsing (Python 3.11+ uses built-in `tomllib`)

## Troubleshooting

### PyYAML not available

If you get "PyYAML not available" error:

```bash
# Preferred: Use nix-shell
nix-shell -p python3.pkgs.pyyaml

# Fallback: Install via pip
pip install pyyaml
```

### File not recognized

Ensure your configuration files use standard extensions:
- YAML: `.yaml` or `.yml`
- JSON: `.json`
- TOML: `.toml`

## License

Part of the ARMOR project. See project license for details.
