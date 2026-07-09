"""
YAML Error Type Definitions and Categorization
"""
from enum import Enum
from dataclasses import dataclass
from typing import Optional


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