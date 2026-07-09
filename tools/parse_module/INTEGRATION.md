# YAML Parser Module - Integration Guide

## Integration into Validation Pipeline

The YAML parser utility module (`tools/parse_module/`) provides a lightweight, safe YAML parsing layer that can be integrated into ARMOR's validation pipeline.

### Architecture

```
┌─────────────────────────────────────────────────────────┐
│                   Validation Pipeline                    │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  ┌──────────────────┐      ┌────────────────────────┐  │
│  │  YAML Input      │───▶  │  tools/parse_module/   │  │
│  │  (files/string)  │      │  - YAMLParser         │  │
│  └──────────────────┘      │  - ParseResult        │  │
│                             │  - ParseStatus        │  │
│                             └────────────────────────┘  │
│                                        │                 │
│                                        ▼                 │
│                             ┌────────────────────────┐  │
│                             │  Validation Logic      │  │
│                             │  (schema checks,       │  │
│                             │   content validation)   │  │
│                             └────────────────────────┘  │
│                                        │                 │
│                                        ▼                 │
│                             ┌────────────────────────┐  │
│                             │  Reporting/Actions     │  │
│                             │  - Error details       │  │
│                             │  - Success messages    │  │
│                             └────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
```

### Usage Patterns

#### 1. Direct Integration

```python
from tools.parse_module import YAMLParser

def validate_yaml_config(filepath: str) -> dict:
    """Validate a YAML configuration file."""
    parser = YAMLParser()
    result = parser.parse_file(filepath)
    
    if result.is_error():
        raise ValueError(f"YAML parsing failed: {result.error}")
    
    return result.data
```

#### 2. Batch Processing

```python
from tools.parse_module import YAMLParser

def validate_multiple_configs(filepaths: list[str]) -> dict:
    """Validate multiple YAML configuration files."""
    parser = YAMLParser()
    results = {}
    
    for filepath in filepaths:
        result = parser.parse_file(filepath)
        results[filepath] = result
        
        if result.is_error():
            print(f"❌ {filepath}: {result.error}")
        else:
            print(f"✓ {filepath}: Valid")
    
    return results
```

#### 3. String Content Validation

```python
from tools.parse_module import YAMLParser

def validate_yaml_string(content: str) -> bool:
    """Validate YAML string content."""
    parser = YAMLParser()
    result = parser.parse_string(content)
    return result.is_success()
```

### Error Handling Integration

The module provides structured error handling that integrates cleanly with ARMOR's validation framework:

```python
from tools.parse_module import YAMLParser, ParseStatus

class ValidationPipeline:
    def __init__(self):
        self.parser = YAMLParser()
        self.errors = []
        self.successes = []
    
    def process_file(self, filepath: str) -> bool:
        """Process a single YAML file."""
        result = self.parser.parse_file(filepath)
        
        if result.status == ParseStatus.ERROR:
            self.errors.append({
                'file': filepath,
                'error': result.error,
                'type': 'parse_error'
            })
            return False
        else:
            self.successes.append({
                'file': filepath,
                'data': result.data
            })
            return True
    
    def get_summary(self) -> dict:
        """Get validation summary."""
        return {
            'total': len(self.errors) + len(self.successes),
            'successful': len(self.successes),
            'failed': len(self.errors),
            'errors': self.errors
        }
```

### Integration Points

#### With ARMOR's Internal Modules

The `tools/parse_module/` serves as a lightweight parser complement to the more advanced `internal/yamlutil/` module:

- **tools/parse_module/**: Simple, safe parsing with basic error handling
- **internal/yamlutil/**: Advanced validation with detailed error categorization

Choose based on complexity needs:
- Use `tools/parse_module/` for basic parsing and simple validation
- Use `internal/yamlutil/` for complex schema validation and detailed error reporting

### Dependencies

```bash
# Install required dependencies
pip install -r tools/parse_module/requirements.txt
```

Required: `pyyaml>=6.0`

### Testing Integration

```python
# Run unit tests before integration
cd tools/parse_module
python -m pytest tests/ -v
```

### Migration Path

For existing code using basic YAML parsing, migrate to this module:

```python
# Before
import yaml
try:
    data = yaml.safe_load(content)
except yaml.YAMLError as e:
    print(f"Error: {e}")

# After
from tools.parse_module import YAMLParser
parser = YAMLParser()
result = parser.parse_string(content)
if result.is_error():
    print(f"Error: {result.error}")
else:
    data = result.data
```

### Notes

- Uses `yaml.safe_load()` for security - prevents execution of arbitrary Python objects
- Provides consistent `ParseResult` structure across all operations
- Thread-safe - can use multiple `YAMLParser` instances concurrently
- No external state - pure functional parsing
