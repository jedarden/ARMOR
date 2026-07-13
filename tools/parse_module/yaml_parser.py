"""
YAML Parser Utility Module.

Provides a simple, safe YAML parser with proper error handling,
structured result objects, and scope depth tracking for hierarchical
key management.
"""

from pathlib import Path
from typing import Any, Optional, List, Dict, Set, Tuple
from dataclasses import dataclass, field
from enum import Enum
try:
    from .result import ParseResult, ParseStatus
except ImportError:
    from result import ParseResult, ParseStatus


class LineClassification(Enum):
    """Classification of YAML lines for parsing purposes."""
    KEY_BEARING = "key-bearing"
    INDENT_ONLY = "indent-only"
    EMPTY = "empty"


class IndentTransitionType(Enum):
    """Classification of indent transition types."""
    ENTER_SCOPE = "enter-scope"
    EXIT_SCOPE = "exit-scope"
    SAME_LEVEL = "same-level"


@dataclass
class IndentTransition:
    """Record of an indent transition during parsing."""
    line_number: int
    from_indent: int
    to_indent: int
    has_key: bool
    raw_line: str
    line_classification: LineClassification
    transition_type: IndentTransitionType

    def is_increase(self) -> bool:
        """Check if this is an indent increase."""
        return self.to_indent > self.from_indent

    def is_decrease(self) -> bool:
        """Check if this is an indent decrease."""
        return self.to_indent < self.from_indent

    def is_enter_scope(self) -> bool:
        """Check if this is an enter-scope transition."""
        return self.transition_type == IndentTransitionType.ENTER_SCOPE

    def is_exit_scope(self) -> bool:
        """Check if this is an exit-scope transition."""
        return self.transition_type == IndentTransitionType.EXIT_SCOPE

    def is_same_level(self) -> bool:
        """Check if this is a same-level transition."""
        return self.transition_type == IndentTransitionType.SAME_LEVEL

    def is_without_key(self) -> bool:
        """Check if this transition occurred without a key."""
        return not self.has_key


@dataclass
class Scope:
    """
    A scope representing a mapping context at a specific nesting level.

    Scopes are created when the parser encounters a parent mapping (a key whose value
    is itself a mapping). Each scope maintains its own set of keys, independent of
    keys in parent or sibling scopes.
    """
    indent_level: int
    start_line: int
    parent_key: Optional[str] = None
    is_flow_style: bool = False
    in_sequence_context: bool = False
    sequence_item_id: Optional[int] = None
    keys: Set[str] = field(default_factory=set)

    def add_key(self, key: str) -> bool:
        """
        Add a key to this scope, returning True if it's a duplicate.

        Args:
            key: The key to add

        Returns:
            True if the key already exists in this scope (duplicate), False otherwise
        """
        if key in self.keys:
            return True  # Already exists (duplicate)
        self.keys.add(key)
        return False  # Successfully added (not a duplicate)

    def contains_key(self, key: str) -> bool:
        """Check if this scope contains a key."""
        return key in self.keys

    def key_count(self) -> int:
        """Get the number of keys in this scope."""
        return len(self.keys)

    def clear_keys(self):
        """Clear all keys from this scope."""
        self.keys.clear()


class DuplicateKeyError(Exception):
    """Error raised when a duplicate key is detected in a scope."""

    def __init__(self, key: str, scope_path: str, first_line: int, duplicate_line: int):
        self.key = key
        self.scope_path = scope_path
        self.first_line = first_line
        self.duplicate_line = duplicate_line
        super().__init__(self.message())

    def message(self) -> str:
        """Get a formatted error message."""
        return (
            f"Line {self.duplicate_line}: duplicate key '{self.key}' in scope '{self.scope_path}'\n"
            f"  First defined at: Line {self.first_line}"
        )


class ScopeStack:
    """
    Hierarchical stack of active scopes during YAML parsing.

    The ScopeStack maintains the hierarchical nature of YAML scopes during parsing.
    As the parser encounters different indentation levels, it enters and exits scopes,
    ensuring that duplicate key detection only considers keys within the same scope.
    """

    def __init__(self, base_indent: int = 2):
        """
        Initialize a new scope stack.

        Args:
            base_indent: The base indentation size in spaces (usually 2 or 4)
        """
        self.scopes: List[Scope] = [Scope(indent_level=0, start_line=0, parent_key=None)]
        self.base_indent: int = base_indent
        self.sequence_item_counter: int = 0
        self.indent_transitions: List[IndentTransition] = []
        self.last_indent: int = 0

    def depth(self) -> int:
        """Get the number of active scopes in the stack."""
        return len(self.scopes)

    def current_indent(self) -> int:
        """Get the current indent level."""
        if not self.scopes:
            return 0
        return self.scopes[-1].indent_level

    def current_scope(self) -> Scope:
        """Get the current scope (top of stack)."""
        if not self.scopes:
            raise ValueError("Scope stack is empty")
        return self.scopes[-1]

    def get_scope_at_level(self, indent_level: int) -> Optional[Scope]:
        """Get scope for a specific indentation level."""
        for scope in self.scopes:
            if scope.indent_level == indent_level:
                return scope
        return None

    def get_scope_path(self) -> str:
        """Get human-readable path to current scope."""
        path_parts = []
        for scope in self.scopes:
            if scope.parent_key:
                path_parts.append(scope.parent_key)
        return ".".join(path_parts)

    def enter_scope(self, indent_level: int, line: int, parent_key: Optional[str] = None):
        """
        Enter a new scope (when indent increases).

        Args:
            indent_level: The new indentation level
            line: The line number where this scope starts (1-indexed)
            parent_key: Optional parent key that created this scope
        """
        # Check if we already have a scope at this level
        existing = self.get_scope_at_level(indent_level)

        if existing:
            # Re-entering a scope level - clear and reuse for sibling mappings
            fresh_scope = Scope(
                indent_level=indent_level,
                start_line=line,
                parent_key=parent_key
            )
            fresh_scope.is_flow_style = existing.is_flow_style

            # Remove all scopes deeper than this level
            self.scopes = [s for s in self.scopes if s.indent_level <= indent_level]
            self.scopes.append(fresh_scope)
        else:
            # Create new scope
            new_scope = Scope(
                indent_level=indent_level,
                start_line=line,
                parent_key=parent_key
            )
            self.scopes.append(new_scope)

    def enter_sequence_scope(self, indent_level: int, line: int):
        """
        Enter a sequence context (when we see a `-` item).

        Args:
            indent_level: The indentation level of the sequence item
            line: The line number where this sequence item starts (1-indexed)
        """
        # Remove all scopes at or deeper than this level
        self.scopes = [s for s in self.scopes if s.indent_level < indent_level]

        # Always create a new scope for each sequence item
        self.sequence_item_counter += 1
        new_scope = Scope(
            indent_level=indent_level,
            start_line=line,
            parent_key=None
        )
        new_scope.in_sequence_context = True
        new_scope.sequence_item_id = self.sequence_item_counter
        self.scopes.append(new_scope)

    def exit_to_scope(self, target_indent: int):
        """
        Exit to parent scope (when indent decreases).

        Args:
            target_indent: The indentation level to exit to
        """
        if target_indent > self.current_indent():
            # Can't exit to a deeper level
            return

        # Remove all scopes deeper than target
        self.scopes = [s for s in self.scopes if s.indent_level <= target_indent]

        # Ensure we have a scope at the target level
        if not self.scopes:
            # Create fallback scope if stack would be empty
            self.scopes.append(Scope(indent_level=0, start_line=0, parent_key=None))
        elif not any(s.indent_level == target_indent for s in self.scopes):
            # No exact match - create fallback
            fallback = Scope(indent_level=target_indent, start_line=0, parent_key=None)
            self.scopes.append(fallback)

    def exit_one_level(self) -> bool:
        """
        Exit one level to immediate parent scope.

        Returns:
            True if successfully exited to parent, False if already at root scope
        """
        if self.depth() <= 1:
            return False

        current_indent = self.current_indent()

        # Find the parent scope's indent level
        if self.depth() >= 2:
            parent_indent = self.scopes[-2].indent_level
        else:
            parent_indent = 0

        self.exit_to_scope(parent_indent)
        return True

    def contains_key(self, key: str) -> bool:
        """Check if current scope contains a key."""
        if not self.scopes:
            return False
        return self.scopes[-1].contains_key(key)

    def contains_key_in_any_scope(self, key: str) -> bool:
        """Check if any scope in the hierarchy contains a key."""
        return any(scope.contains_key(key) for scope in self.scopes)

    def add_key(self, key: str, line: int) -> None:
        """
        Add a key to current scope.

        Args:
            key: The key to add
            line: The line number where the key appears (1-indexed)

        Raises:
            DuplicateKeyError: If the key already exists in the current scope
        """
        if self.contains_key(key):
            scope = self.current_scope()
            raise DuplicateKeyError(
                key=key,
                scope_path=self.get_scope_path(),
                first_line=scope.start_line,
                duplicate_line=line
            )
        self.scopes[-1].add_key(key)

    def in_sequence_context(self) -> bool:
        """Check if we're in a sequence context."""
        if not self.scopes:
            return False
        return self.scopes[-1].in_sequence_context

    def record_indent_transition(self, line_number: int, new_indent: int,
                                 has_key: bool, raw_line: str):
        """
        Record an indent transition.

        Args:
            line_number: The line number where this transition occurred (1-indexed)
            new_indent: The new indentation level
            has_key: Whether this transition occurred on a line with a key token
            raw_line: The raw line content (for debugging)
        """
        if new_indent != self.last_indent:
            old_indent = self.last_indent
            line_classification = self._classify_line_type(raw_line)

            transition = IndentTransition(
                line_number=line_number,
                from_indent=old_indent,
                to_indent=new_indent,
                has_key=has_key,
                raw_line=raw_line,
                line_classification=line_classification,
                transition_type=self._classify_transition(old_indent, new_indent)
            )
            self.indent_transitions.append(transition)
            self.last_indent = new_indent

    def _classify_transition(self, from_indent: int, to_indent: int) -> IndentTransitionType:
        """Classify an indent transition."""
        if to_indent > from_indent:
            return IndentTransitionType.ENTER_SCOPE
        elif to_indent < from_indent:
            return IndentTransitionType.EXIT_SCOPE
        else:
            return IndentTransitionType.SAME_LEVEL

    def _classify_line_type(self, line: str) -> LineClassification:
        """Classify a YAML line as key-bearing or indent-only."""
        trimmed = line.strip()

        if not trimmed:
            return LineClassification.EMPTY

        # Check if line has a key token
        if self._extract_key_context(line):
            return LineClassification.KEY_BEARING
        else:
            return LineClassification.INDENT_ONLY

    def _extract_key_context(self, line: str) -> Optional[Dict[str, Any]]:
        """
        Extract key context from a line.

        Returns:
            Dict with key context info, or None if no key found
        """
        trimmed = line.strip()

        # Find colon position
        colon_pos = trimmed.find(':')
        if colon_pos == -1:
            return None

        key_part = trimmed[:colon_pos]
        after_colon = trimmed[colon_pos + 1:]

        # Skip if key is empty or contains invalid characters
        key = key_part.strip()
        if not key or any(c in key for c in ['{', '}', '[', ']']):
            return None

        # Strip sequence dash from key if present
        if key.startswith("- "):
            key = key[2:].strip()
        elif key.startswith('-') and len(key) > 1:
            key = key[1:].strip()

        if not key:
            return None

        # Classify based on what comes after the colon
        if after_colon.strip():
            return {
                'type': 'inline_scalar',
                'key': key,
                'value': after_colon.strip()
            }
        else:
            return {
                'type': 'parent_mapping',
                'key': key
            }

    def get_indent_transitions(self) -> List[IndentTransition]:
        """Get all recorded indent transitions."""
        return self.indent_transitions.copy()

    def clear_indent_transitions(self):
        """Clear all indent transitions."""
        self.indent_transitions.clear()
        self.last_indent = 0

    def reset(self):
        """Clear all scopes and reset to root."""
        self.scopes = [Scope(indent_level=0, start_line=0, parent_key=None)]
        self.clear_indent_transitions()


class YAMLParser:
    """
    YAML parser with safe_load wrapper, error handling, and scope depth tracking.

    Uses yaml.safe_load() to safely parse YAML files without
    executing arbitrary Python objects, and provides scope tracking
    for hierarchical key management.
    """

    def __init__(self, enable_scope_tracking: bool = False, base_indent: int = 2):
        """
        Initialize the YAML parser.

        Args:
            enable_scope_tracking: Whether to enable scope depth tracking during parsing
            base_indent: The base indentation size for scope tracking (usually 2 or 4)
        """
        self.yaml = None
        self._import_yaml()

        # Scope tracking state
        self.enable_scope_tracking = enable_scope_tracking
        self.scope_stack: Optional[ScopeStack] = None
        self.base_indent = base_indent
        self.scope_depth: int = 0

    def _import_yaml(self) -> None:
        """
        Import PyYAML module with fallback handling.

        Raises:
            RuntimeError: If PyYAML is not available
        """
        try:
            import yaml
            self.yaml = yaml
        except ImportError:
            raise RuntimeError(
                "PyYAML is required but not available. "
                "Install it via: pip install pyyaml"
            )

    def parse_string(self, yaml_content: str) -> ParseResult:
        """
        Parse YAML from a string.

        Args:
            yaml_content: String containing YAML content

        Returns:
            ParseResult with status, data, and error fields
        """
        if not self.yaml:
            return ParseResult.make_error('PyYAML not available')

        if not yaml_content or not yaml_content.strip():
            return ParseResult.make_error('Empty YAML content')

        try:
            data = self.yaml.safe_load(yaml_content)
            return ParseResult.success(data)
        except self.yaml.YAMLError as e:
            error_msg = self._format_yaml_error(str(e))
            return ParseResult.make_error(error_msg)
        except Exception as e:
            return ParseResult.make_error(f'Unexpected error: {str(e)}')

    def parse_file(self, filepath: str) -> ParseResult:
        """
        Parse YAML from a file.

        Args:
            filepath: Path to the YAML file

        Returns:
            ParseResult with status, data, and error fields
        """
        path = Path(filepath)

        # Check if file exists
        if not path.exists():
            return ParseResult.make_error(f'File not found: {filepath}')

        # Check if it's a file (not directory)
        if not path.is_file():
            return ParseResult.make_error(f'Path is not a file: {filepath}')

        try:
            with open(path, 'r', encoding='utf-8') as f:
                content = f.read()

            return self.parse_string(content)

        except FileNotFoundError:
            return ParseResult.make_error(f'File not found: {filepath}')
        except PermissionError:
            return ParseResult.make_error(f'Permission denied: {filepath}')
        except UnicodeDecodeError as e:
            return ParseResult.make_error(f'Encoding error reading file: {str(e)}')
        except Exception as e:
            return ParseResult.make_error(f'Error reading file: {str(e)}')

    def _format_yaml_error(self, error_message: str) -> str:
        """
        Format YAML error message for better readability.

        Args:
            error_message: Raw error message from PyYAML

        Returns:
            Formatted error message
        """
        # Clean up common PyYAML error patterns
        error_message = error_message.strip()

        # Add context for common errors
        if 'could not find expected' in error_message.lower():
            return f"YAML syntax error: {error_message}. Check indentation and structure."
        elif 'mapping values are not allowed here' in error_message.lower():
            return f"YAML structure error: {error_message}. Check colons and indentation."
        elif 'duplicate key' in error_message.lower():
            return f"YAML validation error: {error_message}"

        return f"YAML parsing error: {error_message}"

    # Scope tracking methods

    def _init_scope_tracking(self):
        """Initialize scope tracking for a new parse operation."""
        if self.enable_scope_tracking:
            self.scope_stack = ScopeStack(base_indent=self.base_indent)
            self.scope_depth = 0
        else:
            self.scope_stack = None
            self.scope_depth = 0

    def get_scope_depth(self) -> int:
        """
        Get the current scope depth during parsing.

        Returns:
            The current nesting depth (includes root scope as depth 1)
            Returns 0 if scope tracking is not enabled.
        """
        if self.scope_stack:
            return self.scope_stack.depth()  # Include all scopes including root
        return 0

    def get_scope_path(self) -> str:
        """
        Get the current scope path as a dot-separated string.

        Returns:
            A string like "services.web.database" representing the scope hierarchy.
            Returns empty string if scope tracking is not enabled.
        """
        if self.scope_stack:
            return self.scope_stack.get_scope_path()
        return ""

    def get_parent_scope(self) -> Optional[str]:
        """
        Get the parent scope reference.

        Returns:
            The parent scope key if available, None otherwise.
        """
        if self.scope_stack and self.scope_stack.depth() > 1:
            # Get the second-to-last scope's parent_key (current scope's parent)
            return self.scope_stack.scopes[-2].parent_key if len(self.scope_stack.scopes) > 1 else None
        return None

    def get_scope_stack(self) -> Optional[ScopeStack]:
        """
        Get the current scope stack.

        Returns:
            The ScopeStack instance if scope tracking is enabled, None otherwise.
        """
        return self.scope_stack

    def parse_with_scope_tracking(self, yaml_content: str) -> ParseResult:
        """
        Parse YAML content with scope depth tracking enabled.

        This method parses the YAML content line-by-line to track scope depth
        and hierarchy during parsing. It returns both the parsed data and
        scope tracking information.

        Args:
            yaml_content: String containing YAML content

        Returns:
            ParseResult with status, data, and error fields. If successful,
            the parser's scope tracking state will be populated.
        """
        if not self.yaml:
            return ParseResult.make_error('PyYAML not available')

        if not yaml_content or not yaml_content.strip():
            return ParseResult.make_error('Empty YAML content')

        # Initialize scope tracking
        original_setting = self.enable_scope_tracking
        self.enable_scope_tracking = True
        self._init_scope_tracking()

        try:
            # First, do the regular parse
            data = self.yaml.safe_load(yaml_content)

            # Then, track scopes by processing line by line
            self._track_scopes_from_yaml(yaml_content)

            return ParseResult.success(data)
        except self.yaml.YAMLError as e:
            error_msg = self._format_yaml_error(str(e))
            return ParseResult.make_error(error_msg)
        except Exception as e:
            error_msg = f'Unexpected error: {str(e)}'
            return ParseResult.make_error(error_msg)
        finally:
            # Restore original setting
            self.enable_scope_tracking = original_setting

    def _track_scopes_from_yaml(self, yaml_content: str):
        """
        Track scopes by processing YAML content line by line.

        Args:
            yaml_content: The YAML content to process
        """
        if not self.scope_stack:
            return

        lines = yaml_content.split('\n')

        for line_num, raw_line in enumerate(lines, start=1):
            trimmed = raw_line.strip()

            # Skip empty lines and comments for key detection
            is_empty = not trimmed or trimmed.startswith('#')

            # Get indent level
            indent = len(raw_line) - len(raw_line.lstrip())

            # Check if line has a key
            key_context = None
            if not is_empty:
                key_context = self.scope_stack._extract_key_context(raw_line)

            has_key = key_context is not None

            # Record indent transition
            self.scope_stack.record_indent_transition(line_num, indent, has_key, raw_line)

            # Handle scope transitions based on indent
            current_indent = self.scope_stack.current_indent()

            if indent > current_indent:
                # Indent increased - enter new scope
                parent_key = None
                if key_context and key_context.get('type') == 'parent_mapping':
                    parent_key = key_context['key']
                    # Add parent key to current scope first
                    try:
                        self.scope_stack.add_key(parent_key, line_num)
                    except DuplicateKeyError:
                        pass  # Ignore duplicate parent keys for now

                self.scope_stack.enter_scope(indent, line_num, parent_key)
                self.scope_depth = self.get_scope_depth()

            elif indent < current_indent:
                # Indent decreased - exit to parent scope
                self.scope_stack.exit_to_scope(indent)
                self.scope_depth = self.get_scope_depth()

            # Add key to current scope if present
            if key_context and has_key:
                key_type = key_context.get('type')
                key_name = key_context.get('key')

                if key_type == 'inline_scalar':
                    try:
                        self.scope_stack.add_key(key_name, line_num)
                    except DuplicateKeyError:
                        pass  # Ignore for now

    def get_scope_summary(self) -> Dict[str, Any]:
        """
        Get a summary of the current scope state.

        Returns:
            A dictionary containing scope tracking information:
            - depth: Current scope depth
            - path: Current scope path
            - stack_size: Number of scopes in the stack
            - in_sequence: Whether currently in a sequence context
            Returns empty dict if scope tracking is not enabled.
        """
        if not self.scope_stack:
            return {}

        return {
            'depth': self.get_scope_depth(),
            'path': self.get_scope_path(),
            'stack_size': self.scope_stack.depth(),
            'in_sequence': self.scope_stack.in_sequence_context(),
        }
