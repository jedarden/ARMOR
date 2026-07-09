"""
Result structure for parsing operations.

Provides a consistent result format with status, data, and error fields.
"""
from dataclasses import dataclass
from typing import Any, Optional
from enum import Enum


class ParseStatus(Enum):
    """Status of a parsing operation."""
    SUCCESS = "success"
    ERROR = "error"


@dataclass
class ParseResult:
    """
    Result of a parsing operation.

    Attributes:
        status: ParseStatus.SUCCESS or ParseStatus.ERROR
        data: Parsed content (present when status is SUCCESS)
        error: Error message (present when status is ERROR)

    Example:
        # Success case
        result = ParseResult(
            status=ParseStatus.SUCCESS,
            data={'key': 'value'},
            error=None
        )

        # Error case
        result = ParseResult(
            status=ParseStatus.ERROR,
            data=None,
            error="Invalid YAML syntax"
        )

        # Check result
        if result.status == ParseStatus.SUCCESS:
            print(f"Parsed data: {result.data}")
        else:
            print(f"Error: {result.error}")
    """
    status: ParseStatus
    data: Optional[Any] = None
    error: Optional[str] = None

    def is_success(self) -> bool:
        """Check if the parse operation was successful."""
        return self.status == ParseStatus.SUCCESS

    def is_error(self) -> bool:
        """Check if the parse operation failed."""
        return self.status == ParseStatus.ERROR

    def get_data(self) -> Any:
        """
        Get the parsed data.

        Returns:
            The parsed data if successful, raises RuntimeError otherwise.

        Raises:
            RuntimeError: If the parse operation was not successful.
        """
        if not self.is_success():
            raise RuntimeError(f"Cannot get data from failed result: {self.error}")
        return self.data

    def get_error(self) -> Optional[str]:
        """Get the error message if present."""
        return self.error

    def __str__(self) -> str:
        """Human-readable string representation."""
        if self.is_success():
            return f"ParseResult(status={self.status.value}, data={type(self.data).__name__})"
        return f"ParseResult(status={self.status.value}, error={self.error})"

    @classmethod
    def success(cls, data: Any) -> 'ParseResult':
        """
        Create a successful parse result.

        Args:
            data: The parsed data

        Returns:
            ParseResult with SUCCESS status
        """
        return cls(status=ParseStatus.SUCCESS, data=data, error=None)

    @classmethod
    def make_error(cls, error_message: str) -> 'ParseResult':
        """
        Create an error parse result.

        Args:
            error_message: Description of the error

        Returns:
            ParseResult with ERROR status
        """
        return cls(status=ParseStatus.ERROR, data=None, error=error_message)


__all__ = ['ParseStatus', 'ParseResult']
