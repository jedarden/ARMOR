"""
YAML Parser Module Interface Definitions

This module defines abstract interfaces and protocols for YAML parsing operations.
These interfaces provide type safety and enable extensibility for future implementations.
"""

from abc import ABC, abstractmethod
from typing import Union, Dict, List, Any, Optional, Protocol
from pathlib import Path
from dataclasses import dataclass

from .error_types import (
    YAMLErrorCategory,
    YAMLErrorSeverity,
    YAMLErrorDetail,
    YAMLValidationResult
)


# ============================================================================
# Type Definitions
# ============================================================================

YAMLData = Union[Dict[str, Any], List[Any], str, int, float, bool, None]
"""Valid YAML data types."""

FilePath = Union[str, Path]
"""Accepted file path types."""

YAMLContent = str
"""YAML content string type."""


# ============================================================================
# Core Interfaces
# ============================================================================

class IYAMLReader(ABC):
    """
    Interface for YAML file reading operations.

    Implementations must provide safe file reading with comprehensive
    error handling and detailed result reporting.
    """

    @abstractmethod
    def read_file(self, filepath: FilePath, multi_document: bool = False) -> 'YAMLReadResult':
        """
        Read and parse a YAML file.

        Args:
            filepath: Path to the YAML file
            multi_document: If True, parse as multi-document YAML

        Returns:
            YAMLReadResult with parsed data or error information
        """
        pass

    @abstractmethod
    def read_multiple_files(self, filepaths: List[FilePath], multi_document: bool = False) -> List['YAMLReadResult']:
        """
        Read multiple YAML files.

        Args:
            filepaths: List of paths to YAML files
            multi_document: If True, parse each as multi-document YAML

        Returns:
            List of YAMLReadResult objects
        """
        pass

    @abstractmethod
    def validate_file(self, filepath: FilePath) -> YAMLValidationResult:
        """
        Validate a YAML file without returning parsed data.

        Args:
            filepath: Path to the YAML file to validate

        Returns:
            YAMLValidationResult with validation information
        """
        pass


class IYAMLValidator(ABC):
    """
    Interface for YAML validation operations.

    Implementations must provide comprehensive syntax and structural
    validation with detailed error reporting and categorization.
    """

    @abstractmethod
    def validate_file(self, filepath: FilePath) -> YAMLValidationResult:
        """
        Validate a YAML file for syntax errors.

        Args:
            filepath: Path to the YAML file to validate

        Returns:
            YAMLValidationResult with detailed error information
        """
        pass

    @abstractmethod
    def validate_content(self, content: YAMLContent, source: str = "<string>") -> YAMLValidationResult:
        """
        Validate YAML content for syntax errors.

        Args:
            content: YAML content to validate
            source: Source identifier (filename or "<string>")

        Returns:
            YAMLValidationResult with detailed error information
        """
        pass

    @abstractmethod
    def validate_multiple_files(self, filepaths: List[FilePath]) -> List[YAMLValidationResult]:
        """
        Validate multiple YAML files.

        Args:
            filepaths: List of paths to YAML files

        Returns:
            List of YAMLValidationResult objects
        """
        pass


class IYAMLWriter(ABC):
    """
    Interface for YAML writing operations.

    Implementations must provide safe YAML serialization with
    formatting options and error handling.
    """

    @abstractmethod
    def write_file(self, data: YAMLData, filepath: FilePath, **options) -> bool:
        """
        Write data to a YAML file.

        Args:
            data: Data to serialize
            filepath: Target file path
            **options: Formatting options (indent, width, etc.)

        Returns:
            True if successful, False otherwise
        """
        pass

    @abstractmethod
    def to_string(self, data: YAMLData, **options) -> str:
        """
        Convert data to YAML string.

        Args:
            data: Data to serialize
            **options: Formatting options

        Returns:
            YAML string representation
        """
        pass


# ============================================================================
# Protocol Definitions (for Structural Subtyping)
# ============================================================================

class YAMLReadResultProtocol(Protocol):
    """
    Protocol for YAML read result objects.

    This protocol defines the expected interface for result objects
    returned by YAML reading operations, enabling structural subtyping.
    """

    success: bool
    data: Optional[YAMLData]
    errors: List[YAMLErrorDetail]
    warnings: List[YAMLErrorDetail]
    filepath: str

    def has_errors(self) -> bool:
        """Check if any critical errors occurred."""
        ...

    def has_warnings(self) -> bool:
        """Check if any warnings occurred."""
        ...

    def get_data(self) -> YAMLData:
        """Get the parsed data, raising an error if read failed."""
        ...

    def to_exception(self) -> Optional['YAMLParserError']:
        """Convert errors to appropriate exception."""
        ...

    def raise_if_error(self) -> None:
        """Raise an exception if the read failed."""
        ...


class YAMLErrorDetailProtocol(Protocol):
    """
    Protocol for YAML error detail objects.

    Defines the expected interface for error detail objects,
    enabling consistent error reporting across implementations.
    """

    category: YAMLErrorCategory
    severity: YAMLErrorSeverity
    line: Optional[int]
    column: Optional[int]
    message: str
    context: str
    suggestion: str


# ============================================================================
# Middleware Interfaces
# ============================================================================

class YAMLMiddleware(Protocol):
    """
    Protocol for YAML processing middleware.

    Middleware can transform YAML data during reading/writing operations,
    enabling features like environment variable expansion, includes,
    or custom processing.
    """

    def process(self, data: YAMLData, context: Dict[str, Any]) -> YAMLData:
        """
        Process YAML data through middleware.

        Args:
            data: YAML data to process
            context: Processing context and metadata

        Returns:
            Processed YAML data
        """
        ...


class SchemaValidatorProtocol(Protocol):
    """
    Protocol for schema validation implementations.

    Enables pluggable schema validation systems (JSON Schema,
    custom schemas, etc.) for YAML data validation.
    """

    def validate(self, data: YAMLData, schema: Any) -> 'SchemaValidationResult':
        """
        Validate data against schema.

        Args:
            data: YAML data to validate
            schema: Schema definition

        Returns:
            SchemaValidationResult with validation details
        """
        ...


@dataclass
class SchemaValidationResult:
    """
    Result of schema validation operations.

    Attributes:
        is_valid: Whether validation passed
        errors: Validation errors
        schema_path: Path to schema used for validation
        data_path: Path to data that was validated
    """

    is_valid: bool
    errors: List[str]
    schema_path: Optional[str] = None
    data_path: Optional[str] = None


# ============================================================================
# Result Data Structures
# ============================================================================

@dataclass
class YAMLWriteResult:
    """
    Result of writing YAML data with comprehensive state information.

    This dataclass encapsulates the outcome of YAML write operations,
    providing detailed success/failure information.

    Attributes:
        success: Whether the write operation completed successfully
        filepath: The absolute path to the file that was written
        bytes_written: Number of bytes written (if successful)
        errors: List of errors encountered during writing
        warnings: List of warnings during writing
    """

    success: bool
    filepath: str
    bytes_written: Optional[int]
    errors: List[str]
    warnings: List[str]

    def has_errors(self) -> bool:
        """Check if any errors occurred."""
        return len(self.errors) > 0

    def has_warnings(self) -> bool:
        """Check if any warnings occurred."""
        return len(self.warnings) > 0


# ============================================================================
# Exported Interfaces
# ============================================================================

__all__ = [
    # Type definitions
    'YAMLData',
    'FilePath',
    'YAMLContent',

    # Core interfaces
    'IYAMLReader',
    'IYAMLValidator',
    'IYAMLWriter',

    # Protocols
    'YAMLReadResultProtocol',
    'YAMLErrorDetailProtocol',
    'YAMLMiddleware',
    'SchemaValidatorProtocol',

    # Result data structures
    'YAMLWriteResult',
    'SchemaValidationResult',
]