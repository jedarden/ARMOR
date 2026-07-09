"""
Result Type Definitions for YAML Operations

This module defines the core Result dataclass and Status enum for representing
operation outcomes throughout the YAML parsing pipeline.

The Result<T> pattern provides:
- Explicit success/failure states via Status enum
- Type-safe data access with proper error handling
- Consistent error propagation across operations
- Generic type parameter for different data types

Example:
    from internal.yamlutil import Result, Status

    # Success case
    result = Result.success({"key": "value"})
    if result.is_success():
        data = result.data  # {"key": "value"}
        print(f"Loaded {len(data)} keys")

    # Error case
    result = Result.error("Invalid YAML syntax")
    if result.is_error():
        error = result.error  # "Invalid YAML syntax"
        print(f"Parse failed: {error}")

    # Generic with type annotation
    result: Result[dict[str, Any]] = Result.success({})
"""

from dataclasses import dataclass
from typing import TypeVar, Generic, Optional, Any
from enum import Enum


# ============================================================================
# Type Variable for Generic Result
# ============================================================================

T = TypeVar('T')


# ============================================================================
# Status Enumeration
# ============================================================================

class Status(Enum):
    """
    Status enumeration for Result types.

    Represents the outcome state of an operation, providing explicit
    success/failure semantics for error handling and control flow.

    Values:
        SUCCESS: Operation completed successfully
        ERROR: Operation failed with an error

    Example:
        status = Status.SUCCESS
        if status == Status.SUCCESS:
            print("Operation succeeded")
        elif status == Status.ERROR:
            print("Operation failed")
    """
    SUCCESS = "success"
    ERROR = "error"

    def is_success(self) -> bool:
        """Check if status is SUCCESS."""
        return self == Status.SUCCESS

    def is_error(self) -> bool:
        """Check if status is ERROR."""
        return self == Status.ERROR

    @classmethod
    def from_bool(cls, success: bool) -> 'Status':
        """
        Create Status from boolean result.

        Args:
            success: True for SUCCESS, False for ERROR

        Returns:
            Status.SUCCESS if success is True, else Status.ERROR

        Example:
            status = Status.from_bool(True)
            assert status == Status.SUCCESS
        """
        return cls.SUCCESS if success else cls.ERROR

    def as_bool(self) -> bool:
        """
        Convert Status to boolean.

        Returns:
            True if SUCCESS, False if ERROR

        Example:
            status = Status.SUCCESS
            assert status.as_bool() is True
        """
        return self.is_success()


# ============================================================================
# Result Dataclass
# ============================================================================

@dataclass
class Result(Generic[T]):
    """
    Result of an operation with explicit status, data, and error fields.

    This dataclass provides a type-safe way to represent operation outcomes,
    ensuring proper error handling and preventing access to data when
    operations fail.

    Type Parameters:
        T: The type of the data field on success

    Attributes:
        status: Status enum (SUCCESS or ERROR)
        data: Parsed content on success (None on error)
        error: Error message on error (None on success)

    Invariants:
        - When status is SUCCESS: data is not None, error is None
        - When status is ERROR: data is None, error is not None

    Example:
        # Success with data
        result = Result.success({"name": "test"})
        assert result.status == Status.SUCCESS
        assert result.data == {"name": "test"}
        assert result.error is None

        # Error with message
        result = Result.error("File not found")
        assert result.status == Status.ERROR
        assert result.data is None
        assert result.error == "File not found"

        # Using with generic types
        from typing import Dict, Any

        def parse_config(path: str) -> Result[Dict[str, Any]]:
            # ... parsing logic ...
            return Result.success({"key": "value"})

        result = parse_config("config.yaml")
        if result.is_success():
            config = result.get_data()  # type: Dict[str, Any]
    """

    status: Status
    data: Optional[T] = None
    error: Optional[str] = None

    def is_success(self) -> bool:
        """
        Check if the operation was successful.

        Returns:
            True if status is SUCCESS, False otherwise

        Example:
            result = Result.success("data")
            if result.is_success():
                print("Success!")
        """
        return self.status.is_success()

    def is_error(self) -> bool:
        """
        Check if the operation failed.

        Returns:
            True if status is ERROR, False otherwise

        Example:
            result = Result.error("Something went wrong")
            if result.is_error():
                print(f"Error: {result.error}")
        """
        return self.status.is_error()

    def get_data(self) -> T:
        """
        Get the data, raising an error if the operation failed.

        This method provides explicit error handling by raising an
        exception when attempting to access data from a failed result.

        Returns:
            The data value

        Raises:
            RuntimeError: If status is ERROR

        Example:
            result = Result.success({"key": "value"})
            try:
                data = result.get_data()
                print(f"Got data: {data}")
            except RuntimeError as e:
                print(f"Cannot get data: {e}")
        """
        if not self.is_success():
            raise RuntimeError(
                f"Cannot get data from failed result: {self.error}"
            )
        return self.data  # type: ignore

    def get_error(self) -> Optional[str]:
        """
        Get the error message if present.

        Returns:
            The error message string, or None if status is SUCCESS

        Example:
            result = Result.error("Parse error")
            error = result.get_error()
            if error:
                print(f"Error message: {error}")
        """
        return self.error

    def map(self, func) -> 'Result':
        """
        Apply a function to the data if successful.

        Args:
            func: Function to apply to data

        Returns:
            New Result with transformed data, or error result if failed

        Example:
            result = Result.success([1, 2, 3])
            mapped = result.map(lambda x: len(x))
            assert mapped.data == 3
        """
        if self.is_success():
            try:
                return Result.success(func(self.data))
            except Exception as e:
                return Result.error(f"Map function failed: {e}")
        return self

    def and_then(self, func) -> 'Result':
        """
        Chain operations that return Results.

        Args:
            func: Function that takes data and returns a Result

        Returns:
            Result from func, or error result if failed

        Example:
            def validate(data) -> Result:
                if "key" in data:
                    return Result.success(data)
                return Result.error("Missing 'key'")

            result = Result.success({"key": "value"})
            chained = result.and_then(validate)
        """
        if self.is_success():
            return func(self.data)
        return self

    @classmethod
    def success(cls, data: T) -> 'Result[T]':
        """
        Create a successful Result with data.

        Args:
            data: The parsed or computed data

        Returns:
            Result with status SUCCESS and the provided data

        Example:
            result = Result.success({"key": "value"})
            assert result.is_success()
            assert result.data == {"key": "value"}
        """
        return cls(status=Status.SUCCESS, data=data, error=None)

    @classmethod
    def error(cls, error_message: str) -> 'Result[T]':
        """
        Create a failed Result with an error message.

        Args:
            error_message: Description of the error

        Returns:
            Result with status ERROR and the provided error message

        Example:
            result = Result.error("File not found")
            assert result.is_error()
            assert result.error == "File not found"
        """
        return cls(status=Status.ERROR, data=None, error=error_message)

    def __str__(self) -> str:
        """
        Human-readable string representation.

        Returns:
            String showing status and either data type or error message

        Example:
            result = Result.success({"key": "value"})
            print(result)  # "Result(status=SUCCESS, data=dict)"
        """
        if self.is_success():
            data_type = type(self.data).__name__ if self.data is not None else "None"
            return f"Result(status={self.status.value}, data={data_type})"
        return f"Result(status={self.status.value}, error={self.error})"


# ============================================================================
# Convenience Type Aliases
# ============================================================================

# Common Result types for YAML operations
YAMLDataResult = Result[Any]
"""Result with arbitrary YAML data (dict, list, scalar, etc.)."""

DictResult = Result[dict]
"""Result with dictionary data."""

ListResult = Result[list]
"""Result with list data."""

StrResult = Result[str]
"""Result with string data."""

BoolResult = Result[bool]
"""Result with boolean data."""

IntResult = Result[int]
"""Result with integer data."""


# ============================================================================
# Exports
# ============================================================================

__all__ = [
    # Core types
    'Status',
    'Result',

    # Type aliases
    'YAMLDataResult',
    'DictResult',
    'ListResult',
    'StrResult',
    'BoolResult',
    'IntResult',
]
