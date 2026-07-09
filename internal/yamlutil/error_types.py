"""
YAML Error Type Definitions and Categorization
"""
from enum import Enum
from dataclasses import dataclass
from typing import Optional


# ============================================================================
# Custom Exception Classes for YAML Parsing
# ============================================================================

class YAMLParserError(Exception):
    """
    Base exception class for YAML parsing errors.

    All YAML-related exceptions inherit from this class, allowing
    users to catch all YAML errors with a single except clause.

    Attributes:
        message: Human-readable error description
        filepath: Path to the file that caused the error (if applicable)
        line: Line number where the error occurred (if available)
        column: Column number where the error occurred (if available)
    """

    def __init__(self, message: str, filepath: Optional[str] = None,
                 line: Optional[int] = None, column: Optional[int] = None):
        self.message = message
        self.filepath = filepath
        self.line = line
        self.column = column
        super().__init__(self._format_message())

    def _format_message(self) -> str:
        """Format the exception message with location information."""
        parts = []
        if self.filepath:
            parts.append(f"File: {self.filepath}")
        if self.line is not None:
            location = f"Line {self.line}"
            if self.column is not None:
                location += f", Column {self.column}"
            parts.append(location)
        if parts:
            return f"{' | '.join(parts)}: {self.message}"
        return self.message


class YAMLFileNotFoundError(YAMLParserError):
    """
    Exception raised when a YAML file cannot be found.

    This error occurs when:
    - The specified file path does not exist
    - The path points to a directory instead of a file
    - The file exists but is not readable due to permissions

    Example:
        try:
            reader = YAMLFileReader()
            result = reader.read_file('config.yaml')
            if not result.success:
                raise result.to_exception()
        except YAMLFileNotFoundError as e:
            print(f"Config file missing: {e.filepath}")
            print("Please create a config.yaml file")
    """

    def __init__(self, message: str, filepath: str):
        super().__init__(message, filepath=filepath)
        self.filepath = filepath


class YAMLSyntaxError(YAMLParserError):
    """
    Exception raised when YAML syntax is invalid.

    This error occurs when:
    - Indentation is incorrect
    - Key-value pairs have malformed syntax
    - Quoted strings are not properly closed
    - Flow-style collections (brackets/braces) are malformed

    Example:
        try:
            result = read_yaml_file('config.yaml')
            if not result.success:
                raise result.to_exception()
        except YAMLSyntaxError as e:
            print(f"Syntax error at line {e.line}: {e.message}")
    """

    def __init__(self, message: str, filepath: Optional[str] = None,
                 line: Optional[int] = None, column: Optional[int] = None,
                 context: Optional[str] = None, suggestion: Optional[str] = None):
        super().__init__(message, filepath, line, column)
        self.context = context
        self.suggestion = suggestion

    def __str__(self) -> str:
        """Format syntax error with context and suggestion."""
        base_msg = super().__str__()
        if self.context:
            base_msg += f"\n  Context:\n    {self.context.replace(chr(10), chr(10) + '    ')}"
        if self.suggestion:
            base_msg += f"\n  Suggestion: {self.suggestion}"
        return base_msg


class YAMLStructureError(YAMLParserError):
    """
    Exception raised when YAML structure is invalid.

    This error occurs when:
    - Duplicate keys are found in a mapping
    - Anchor/alias references are invalid
    - Tag handles are incorrect
    - Document structure is malformed

    Example:
        try:
            result = read_yaml_file('data.yaml')
            if not result.success:
                raise result.to_exception()
        except YAMLStructureError as e:
            print(f"Structure error: {e.message}")
    """

    def __init__(self, message: str, filepath: Optional[str] = None,
                 line: Optional[int] = None, column: Optional[int] = None):
        super().__init__(message, filepath, line, column)


class YAMLValidationError(YAMLParserError):
    """
    Exception raised when YAML validation fails.

    This error occurs when:
    - Schema validation fails
    - Required fields are missing
    - Data types are incorrect
    - Business rules are violated

    Example:
        try:
            result = validate_yaml_file('schema.yaml', schema=my_schema)
            if not result.is_valid:
                raise YAMLValidationError.from_validation_result(result)
        except YAMLValidationError as e:
            print(f"Validation failed: {e.message}")
    """

    def __init__(self, message: str, filepath: Optional[str] = None,
                 errors: Optional[list] = None):
        super().__init__(message, filepath=filepath)
        self.errors = errors or []

    @classmethod
    def from_validation_result(cls, validation_result: 'YAMLValidationResult',
                                filepath: Optional[str] = None) -> 'YAMLValidationError':
        """
        Create a validation exception from a YAMLValidationResult.

        Args:
            validation_result: The validation result that failed
            filepath: Optional file path for context

        Returns:
            YAMLValidationError with all validation errors
        """
        error_msgs = [str(e) for e in validation_result.errors]
        message = f"YAML validation failed with {len(error_msgs)} error(s)"
        return cls(message, filepath=filepath, errors=error_msgs)


class YAMLEmptyFileError(YAMLParserError):
    """
    Exception raised when a YAML file is empty.

    This error occurs when:
    - The file exists but contains no content
    - The file contains only whitespace

    Example:
        try:
            result = read_yaml_file('empty.yaml')
            if not result.success:
                raise result.to_exception()
        except YAMLEmptyFileError as e:
            print(f"Empty file: {e.filepath}")
    """

    def __init__(self, message: str, filepath: str):
        super().__init__(message, filepath=filepath)


# ============================================================================
# Error Type Definitions and Categorization
# ============================================================================


class YAMLErrorCategory(Enum):
    """Categories of YAML errors for better error handling and reporting."""
    SYNTAX = "syntax_error"
    INDENTATION = "indentation_error"
    STRUCTURE = "structure_error"
    SCALAR = "scalar_error"
    FLOW = "flow_error"
    TAG = "tag_error"
    ANCHOR = "anchor_error"
    ALIAS = "alias_error"
    DOCUMENT = "document_error"
    UNKNOWN = "unknown_error"


class YAMLErrorSeverity(Enum):
    """Severity levels for YAML errors."""
    CRITICAL = "critical"  # File cannot be parsed
    ERROR = "error"  # Major issue preventing correct parsing
    WARNING = "warning"  # Minor issue that doesn't prevent parsing
    INFO = "info"  # Informational message


@dataclass
class YAMLErrorDetail:
    """Detailed information about a YAML parsing error."""
    category: YAMLErrorCategory
    severity: YAMLErrorSeverity
    line: Optional[int] = None
    column: Optional[int] = None
    message: str = ""
    context: str = ""
    suggestion: str = ""

    def __str__(self) -> str:
        """Format error details for human-readable output."""
        parts = []

        # Location
        if self.line is not None:
            location = f"Line {self.line}"
            if self.column is not None:
                location += f", Column {self.column}"
            parts.append(location)

        # Category and severity
        parts.append(f"[{self.category.value.upper()}] ({self.severity.value.upper()})")

        # Message
        if self.message:
            parts.append(f": {self.message}")

        result = " ".join(parts)

        # Context and suggestion on separate lines
        if self.context:
            result += f"\n  Context: {self.context}"
        if self.suggestion:
            result += f"\n  Suggestion: {self.suggestion}"

        return result


@dataclass
class YAMLValidationResult:
    """Result of YAML validation."""
    is_valid: bool
    errors: list[YAMLErrorDetail]
    warnings: list[YAMLErrorDetail]

    def has_errors(self) -> bool:
        """Check if there are any errors (excluding warnings)."""
        return any(e.severity in [YAMLErrorSeverity.CRITICAL, YAMLErrorSeverity.ERROR]
                   for e in self.errors)

    def has_warnings(self) -> bool:
        """Check if there are any warnings."""
        return len(self.warnings) > 0

    def get_all_issues(self) -> list[YAMLErrorDetail]:
        """Get all issues (errors and warnings combined)."""
        return self.errors + self.warnings

    def __str__(self) -> str:
        """Format validation result for display."""
        if self.is_valid:
            if self.has_warnings():
                return f"✓ Valid with {len(self.warnings)} warning(s)"
            return "✓ Valid YAML"

        error_count = len([e for e in self.errors
                          if e.severity in [YAMLErrorSeverity.CRITICAL, YAMLErrorSeverity.ERROR]])
        warning_count = len(self.warnings)

        result = f"✗ Invalid YAML - {error_count} error(s)"
        if warning_count > 0:
            result += f", {warning_count} warning(s)"
        return result