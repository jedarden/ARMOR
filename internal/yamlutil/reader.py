"""
YAML File Reader with Comprehensive Error Handling

Provides safe YAML file reading with file validation, path resolution,
and detailed error reporting for filesystem and parsing issues.
"""

import os
from pathlib import Path
from typing import Union, Dict, List, Any, Optional
from dataclasses import dataclass

from .error_types import (
    YAMLErrorCategory,
    YAMLErrorSeverity,
    YAMLErrorDetail,
    YAMLValidationResult
)


@dataclass
class YAMLReadResult:
    """
    Result of reading a YAML file.

    Attributes:
        success: Whether the file was read and parsed successfully
        data: The parsed YAML data (None if failed)
        errors: List of critical errors encountered
        warnings: List of warnings encountered
        filepath: The absolute path to the file that was read
    """
    success: bool
    data: Optional[Union[Dict[str, Any], List[Any]]]
    errors: List[YAMLErrorDetail]
    warnings: List[YAMLErrorDetail]
    filepath: str

    def has_errors(self) -> bool:
        """Check if any critical errors occurred."""
        return len(self.errors) > 0

    def has_warnings(self) -> bool:
        """Check if any warnings occurred."""
        return len(self.warnings) > 0

    def get_data(self) -> Union[Dict[str, Any], List[Any]]:
        """
        Get the parsed data, raising an error if read failed.

        Returns:
            The parsed YAML data

        Raises:
            RuntimeError: If the read operation failed
        """
        if not self.success:
            error_msg = "; ".join([e.message for e in self.errors])
            raise RuntimeError(f"Cannot get data from failed YAML read: {error_msg}")
        return self.data


class YAMLFileReader:
    """
    Comprehensive YAML file reader with validation and error handling.

    Provides safe file reading operations with:
    - File path validation and resolution
    - File existence and readability checks
    - YAML parsing with detailed error reporting
    - Support for both single-document and multi-document YAML files
    """

    def __init__(self, resolve_absolute: bool = True):
        """
        Initialize the YAML file reader.

        Args:
            resolve_absolute: Whether to resolve paths to absolute paths
        """
        self.resolve_absolute = resolve_absolute
        self.yaml = None
        self._import_yaml()

    def _import_yaml(self):
        """Import PyYAML with fallback handling."""
        try:
            import yaml
            self.yaml = yaml
        except ImportError:
            raise RuntimeError(
                "PyYAML is required but not available. "
                "Install it via: nix-shell -p python3.pkgs.pyyaml"
            )

    def _resolve_path(self, filepath: str) -> tuple[str, Optional[YAMLErrorDetail]]:
        """
        Resolve a file path to absolute path with validation.

        Args:
            filepath: Path to resolve

        Returns:
            Tuple of (resolved_path, error) - error is None if successful
        """
        if not filepath:
            return "", YAMLErrorDetail(
                category=YAMLErrorCategory.UNKNOWN,
                severity=YAMLErrorSeverity.CRITICAL,
                message="File path cannot be empty"
            )

        try:
            path = Path(filepath)

            if self.resolve_absolute:
                # Resolve to absolute path
                path = path.resolve()

            # Check if path exists
            if not path.exists():
                return str(path), YAMLErrorDetail(
                    category=YAMLErrorCategory.UNKNOWN,
                    severity=YAMLErrorSeverity.CRITICAL,
                    message=f"File not found: {filepath}",
                    context="The specified file does not exist on the filesystem",
                    suggestion="Check the file path and ensure the file exists"
                )

            # Check if it's a file (not a directory)
            if not path.is_file():
                return str(path), YAMLErrorDetail(
                    category=YAMLErrorCategory.UNKNOWN,
                    severity=YAMLErrorSeverity.CRITICAL,
                    message=f"Path is not a file: {filepath}",
                    context="The specified path exists but is not a regular file",
                    suggestion="Ensure the path points to a file, not a directory"
                )

            # Check if file is readable
            if not os.access(path, os.R_OK):
                return str(path), YAMLErrorDetail(
                    category=YAMLErrorCategory.UNKNOWN,
                    severity=YAMLErrorSeverity.CRITICAL,
                    message=f"File is not readable: {filepath}",
                    context="The file exists but cannot be read due to permissions",
                    suggestion="Check file permissions to ensure read access is allowed"
                )

            return str(path), None

        except Exception as e:
            return filepath, YAMLErrorDetail(
                category=YAMLErrorCategory.UNKNOWN,
                severity=YAMLErrorSeverity.CRITICAL,
                message=f"Error resolving file path: {str(e)}",
                context=f"Failed to resolve path: {filepath}"
            )

    def read_file(self, filepath: str, multi_document: bool = False) -> YAMLReadResult:
        """
        Read and parse a YAML file.

        Args:
            filepath: Path to the YAML file to read
            multi_document: If True, parse as multi-document YAML file

        Returns:
            YAMLReadResult with parsed data or error information
        """
        errors = []
        warnings = []

        # Resolve and validate file path
        resolved_path, path_error = self._resolve_path(filepath)
        if path_error:
            return YAMLReadResult(
                success=False,
                data=None,
                errors=[path_error],
                warnings=[],
                filepath=resolved_path
            )

        # Try to read the file
        try:
            with open(resolved_path, 'r') as f:
                content = f.read()

            # Check for empty file
            if not content.strip():
                errors.append(YAMLErrorDetail(
                    category=YAMLErrorCategory.DOCUMENT,
                    severity=YAMLErrorSeverity.ERROR,
                    message="Empty YAML file",
                    context=f"File contains no content: {resolved_path}",
                    suggestion="Add YAML content to the file"
                ))
                return YAMLReadResult(
                    success=False,
                    data=None,
                    errors=errors,
                    warnings=warnings,
                    filepath=resolved_path
                )

            # Parse the YAML content
            try:
                if multi_document:
                    # Parse as multi-document YAML
                    data = list(self.yaml.safe_load_all(content))
                    # If only one document, unwrap it for consistency
                    if len(data) == 1:
                        data = data[0]
                else:
                    # Parse as single-document YAML
                    data = self.yaml.safe_load(content)

                return YAMLReadResult(
                    success=True,
                    data=data,
                    errors=errors,
                    warnings=warnings,
                    filepath=resolved_path
                )

            except self.yaml.YAMLError as e:
                # Extract detailed error information
                error_detail = self._extract_yaml_error(e, content, resolved_path)
                errors.append(error_detail)

                return YAMLReadResult(
                    success=False,
                    data=None,
                    errors=errors,
                    warnings=warnings,
                    filepath=resolved_path
                )

        except IOError as e:
            errors.append(YAMLErrorDetail(
                category=YAMLErrorCategory.UNKNOWN,
                severity=YAMLErrorSeverity.CRITICAL,
                message=f"Failed to read file: {str(e)}",
                context=f"Error reading file: {resolved_path}"
            ))
            return YAMLReadResult(
                success=False,
                data=None,
                errors=errors,
                warnings=warnings,
                filepath=resolved_path
            )

    def _extract_yaml_error(self, error: Exception, content: str, filepath: str) -> YAMLErrorDetail:
        """
        Extract detailed error information from a YAML parsing error.

        Args:
            error: The YAML parsing error
            content: The YAML content being parsed
            filepath: The file being parsed

        Returns:
            YAMLErrorDetail with comprehensive error information
        """
        line = None
        column = None
        context_lines = []

        # Extract line/column from problem_mark
        if hasattr(error, 'problem_mark'):
            mark = error.problem_mark
            if mark:
                line = mark.line + 1  # Convert to 1-indexed
                column = mark.column + 1

                # Get context lines
                content_lines = content.split('\n')
                if 0 <= line - 1 < len(content_lines):
                    context_lines = content_lines[max(0, line-2):min(len(content_lines), line+1)]

        # Extract problem description
        problem = ""
        if hasattr(error, 'problem'):
            problem = error.problem

        # Build helpful message
        message = problem if problem else str(error)

        # Add context if available
        context = ""
        if context_lines:
            context = "\n  ".join(context_lines)

        # Generate suggestion
        suggestion = self._generate_error_suggestion(problem, str(error))

        return YAMLErrorDetail(
            category=YAMLErrorCategory.SYNTAX,
            severity=YAMLErrorSeverity.ERROR,
            line=line,
            column=column,
            message=message,
            context=context,
            suggestion=suggestion
        )

    def _generate_error_suggestion(self, problem: str, error_msg: str) -> str:
        """Generate helpful suggestions for common YAML errors."""
        error_lower = error_msg.lower()
        problem_lower = problem.lower() if problem else ""

        # Common YAML error patterns
        if 'indentation' in error_lower or 'indent' in error_lower:
            return "Check that indentation is consistent (use 2 or 4 spaces per level, never tabs)"

        if 'mapping' in error_lower and 'not allowed' in error_lower:
            return "Check that key-value pairs use 'key: value' syntax with proper indentation"

        if 'flow' in error_lower:
            return "Ensure all { } and [ ] brackets are properly closed and matched"

        if 'tab' in error_lower:
            return "Replace tabs with spaces - YAML requires spaces for indentation"

        if 'quoted string' in error_lower:
            return "Ensure quoted strings are properly terminated with matching quotes"

        if 'anchor' in error_lower or 'alias' in error_lower:
            return "Define anchors with &name and reference them with *name"

        if 'document' in error_lower:
            return "Use '---' to separate multiple documents in a file"

        return "Review the YAML syntax at the reported location"

    def read_multiple_files(self, filepaths: List[str], multi_document: bool = False) -> List[YAMLReadResult]:
        """
        Read multiple YAML files.

        Args:
            filepaths: List of paths to YAML files
            multi_document: If True, parse each as multi-document YAML

        Returns:
            List of YAMLReadResult objects
        """
        results = []
        for filepath in filepaths:
            results.append(self.read_file(filepath, multi_document))
        return results

    def validate_file(self, filepath: str) -> YAMLValidationResult:
        """
        Validate a YAML file without returning the parsed data.

        This is useful for pre-flight validation before actual reading.

        Args:
            filepath: Path to the YAML file to validate

        Returns:
            YAMLValidationResult with validation information
        """
        result = self.read_file(filepath)

        # Convert YAMLReadResult to YAMLValidationResult
        return YAMLValidationResult(
            is_valid=result.success,
            errors=result.errors,
            warnings=result.warnings
        )


# Convenience functions for quick file reading

def read_yaml_file(filepath: str, multi_document: bool = False) -> YAMLReadResult:
    """
    Quick read of a single YAML file.

    Args:
        filepath: Path to the YAML file
        multi_document: If True, parse as multi-document YAML

    Returns:
        YAMLReadResult with parsed data or error information

    Example:
        result = read_yaml_file('config.yaml')
        if result.success:
            data = result.data
            print(f"Loaded config with {len(data)} keys")
        else:
            for error in result.errors:
                print(f"Error: {error}")
    """
    reader = YAMLFileReader()
    return reader.read_file(filepath, multi_document)


def read_yaml_file_simple(filepath: str) -> Optional[Union[Dict[str, Any], List[Any]]]:
    """
    Simple YAML file reader that returns data directly or None on failure.

    This is the simplest interface for basic use cases where you want
    to handle errors yourself or don't need detailed error information.

    Args:
        filepath: Path to the YAML file

    Returns:
        Parsed YAML data as dict or list, or None if reading failed

    Example:
        data = read_yaml_file_simple('config.yaml')
        if data:
            print(f"Server: {data.get('server')}")
        else:
            print("Failed to read file")
    """
    result = read_yaml_file(filepath)
    return result.data if result.success else None


def validate_yaml_file(filepath: str) -> YAMLValidationResult:
    """
    Validate a YAML file for syntax and structural errors.

    Args:
        filepath: Path to the YAML file to validate

    Returns:
        YAMLValidationResult with validation information

    Example:
        result = validate_yaml_file('config.yaml')
        if result.is_valid:
            print("Valid YAML!")
        else:
            for error in result.errors:
                print(f"Error: {error}")
    """
    reader = YAMLFileReader()
    return reader.validate_file(filepath)