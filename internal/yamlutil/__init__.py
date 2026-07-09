"""
YAML Utility Module for ARMOR

Comprehensive YAML parsing, validation, and error reporting with:
- Detailed error categorization
- Line/column number reporting
- Human-readable error messages
- Pre-validation checks for common issues
- File reading with comprehensive error handling
- Support for multi-document YAML files

Usage:
    from internal.yamlutil import YAMLSyntaxValidator, validate_yaml_file, read_yaml_file

    # Read a YAML file
    result = read_yaml_file('config.yaml')
    if result.success:
        data = result.data
        print(f"Loaded {len(data)} keys")
    else:
        for error in result.errors:
            print(f"Error: {error}")

    # Validate a file
    result = validate_yaml_file('config.yaml')
    if result.is_valid:
        print("Valid YAML!")
    else:
        for error in result.errors:
            print(error)
"""

from .error_types import (
    YAMLErrorCategory,
    YAMLErrorSeverity,
    YAMLErrorDetail,
    YAMLValidationResult,
    YAMLParserError,
    YAMLFileNotFoundError,
    YAMLSyntaxError,
    YAMLStructureError,
    YAMLValidationError,
    YAMLEmptyFileError
)

from .validator import (
    YAMLSyntaxValidator,
    validate_yaml_file,
    validate_yaml_string
)

from .reader import (
    YAMLFileReader,
    YAMLReadResult,
    read_yaml_file,
    read_yaml_file_simple
)

__all__ = [
    # Error types
    'YAMLErrorCategory',
    'YAMLErrorSeverity',
    'YAMLErrorDetail',
    'YAMLValidationResult',

    # Custom exceptions
    'YAMLParserError',
    'YAMLFileNotFoundError',
    'YAMLSyntaxError',
    'YAMLStructureError',
    'YAMLValidationError',
    'YAMLEmptyFileError',

    # Validator
    'YAMLSyntaxValidator',

    # Reader
    'YAMLFileReader',
    'YAMLReadResult',

    # Convenience functions
    'validate_yaml_file',
    'validate_yaml_string',
    'read_yaml_file',
    'read_yaml_file_simple',
]

__version__ = '1.0.1'