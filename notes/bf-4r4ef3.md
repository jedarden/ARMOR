# Bead bf-4r4ef3: Key-bearing vs indent-only line classification

## Summary

Implemented comprehensive line type classification for YAML parsing to distinguish between key-bearing lines and indent-only lines, enabling type-specific handling for scope transitions.

## Implementation

### 1. Core Classification (scope.rs)

**LineClassification enum** (lines 1440-1454):
- `KeyBearing`: Line contains a YAML key token (e.g., "key: value", "  nested:", "- item")
- `IndentOnly`: Line has no key token, only content/indentation
- `Empty`: Blank or whitespace-only line

**classify_line_type() function** (lines 1362-1412):
- Analyzes a line to determine its classification
- Uses existing `extract_key_context()` to detect key tokens
- Returns appropriate `LineClassification` variant

**has_key_token() helper** (lines 1414-1438):
- Convenience function that returns true if line is key-bearing
- Wrapper around `classify_line_type()`

### 2. Parser State Tracking (parser.rs)

**BasicParser fields added** (lines 73-76):
- `current_line_type: LineClassification`: Tracks current line type being processed
- `current_transition_state: IndentTransitionState`: Tracks indent transition state

**Helper methods added** (lines 114-131):
- `current_line_type()`: Returns current line classification
- `is_key_bearing_line()`: Checks if current line is key-bearing
- `is_indent_only_line()`: Checks if current line is indent-only
- `is_empty_line()`: Checks if current line is empty

### 3. Type-Specific Scope Handling (parser.rs)

**In detect_duplicate_keys_with_scope()**:
- Lines 146-151: Classify line type for each line
- Lines 186-189: Handle empty/indent-only lines with special logic
- Lines 194-200: Indent-only lines don't trigger scope entry but may trigger exit

**In parse_str()**:
- Lines 397-446: Similar type-specific handling for main parsing logic
- Proper handling of blank lines, comments, and indent-only content

### 4. Enhanced Transition Tracking (scope.rs)

**IndentTransition struct** (lines 1483-1603):
- Tracks line classification along with indent changes
- Includes `line_classification: LineClassification` field
- Provides methods to query transition type (enter/exit/same-level)

**IndentTransitionType enum** (lines 75-124):
- `EnterScope`: Indent increased
- `ExitScope`: Indent decreased
- `SameLevel`: No indent change

**IndentTransitionState** (lines 126-215):
- State machine for tracking indent transitions
- Tracks whether last transition had a key token
- Provides methods to query current state

## Acceptance Criteria Met

✅ Parser can identify key-bearing lines
✅ Parser can identify indent-only lines (no key token)
✅ Line type is tracked in parser state
✅ Classification works for complex YAML structures

## Tests

All existing tests pass, including:
- 250 YAML parser tests
- 13 scope-specific classification tests
- 32 parser integration tests covering complex scenarios

## Files Modified

- `src/parsers/yaml/parser.rs`: Added line classification integration and state tracking
- `src/parsers/yaml/scope.rs`: Added classification enums, functions, and transition tracking
