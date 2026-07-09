"""
YAML Parser Utility Module.

Provides a simple, safe YAML parser with proper error handling
and structured result objects.
"""

from .yaml_parser import YAMLParser
from .result import ParseResult, ParseStatus

__all__ = ['YAMLParser', 'ParseResult', 'ParseStatus']
