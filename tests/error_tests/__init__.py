#!/usr/bin/env python3
"""
Error Tests Package for ARMOR

This package provides the foundational test structure and table definitions
for error response testing. It includes extensible patterns for testing
specific error types and serves as the base for all error test modules.

Structure:
    base: Core test pattern and suite definitions
    auth_tests: Authentication and authorization error tests (extensible)
    validation_tests: Validation error tests (extensible)
    client_error_tests: Client error (4xx) tests (extensible)
    server_error_tests: Server error (5xx) tests (extensible)

Usage:
    from tests.error_tests import TestPattern, ErrorTestSuite, create_base_pattern

    # Create a pattern
    pattern = TestPattern(
        name="not_found",
        description="Test 404 Not Found errors",
        category="client_error",
        ...
    )

    # Create a suite
    suite = ErrorTestSuite(name="Error Tests")
    suite.add_pattern(pattern)

Extension:
    To add new error test modules:
    1. Create a new module in tests/error_tests/ (e.g., protocol_tests.py)
    2. Import TestPattern and ErrorTestSuite from .base
    3. Define patterns specific to your error type
    4. Export key functions/classes from this __init__.py

Bead: bf-2zqplr
Created: 2026-07-15
"""

from .base import (
    # Core classes
    TestPattern,
    PatternResult,
    ErrorTestSuite,
    SuiteResult,

    # Enums
    PatternStatus,
    ErrorCategory,

    # Helper functions
    create_base_pattern,
    create_error_suite,
)

__all__ = [
    # Core classes
    'TestPattern',
    'PatternResult',
    'ErrorTestSuite',
    'SuiteResult',

    # Enums
    'PatternStatus',
    'ErrorCategory',

    # Helper functions
    'create_base_pattern',
    'create_error_suite',
]

# Version info
__version__ = '1.0.0'
__author__ = 'ARMOR Test Team'
