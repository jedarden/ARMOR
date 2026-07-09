"""
YAML Utility Module for ARMOR

Comprehensive YAML parsing, validation, and error reporting with:
- Detailed error categorization
- Line/column number reporting
- Human-readable error messages
- Pre-validation checks for common issues

Usage:
    from internal.yamlutil import YAMLSyntaxValidator, validate_yaml_file

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
    YAMLValidationResult
)

from .validator import (
    YAMLSyntaxValidator,
    validate_yaml_file,
    validate_yaml_string
)

__all__ = [
    # Error types
    'YAMLErrorCategory',
    'YAMLErrorSeverity',
    'YAMLErrorDetail',
    'YAMLValidationResult',

    # Validator
    'YAMLSyntaxValidator',

    # Convenience functions
    'validate_yaml_file',
    'validate_yaml_string',
]

__version__ = '1.0.0'