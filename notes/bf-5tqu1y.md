# bf-5tqu1y: Track line type in parser state

## Summary

Feature to track line type classification in the parser's state tracking has been successfully implemented.

## Implementation Details

The implementation adds line type tracking to the YAML parser state with the following components:

### 1. Parser State Fields (yaml_parser.py:618-619)
```python
self.current_line_type: Optional[LineClassification] = None
self.current_line_number: int = 0
```

### 2. State Updates (yaml_parser.py:841-842)
```python
line_type = self.scope_stack._classify_line_type(raw_line)
self.current_line_type = line_type
```

### 3. Accessible to Downstream Logic (yaml_parser.py:866-888)
The line type is used in scope transition logic:
- Key-bearing lines trigger new scope creation on indent increase
- Indent-only lines do NOT create new scopes
- Empty lines don't trigger scope transitions

### 4. Public API Methods (yaml_parser.py:933-980)
- `get_current_line_type()` - Returns the current line classification
- `is_on_key_bearing_line()` - Check if on a key-bearing line
- `is_on_indent_only_line()` - Check if on an indent-only line
- `is_on_empty_line()` - Check if on an empty line
- `get_scope_summary()` - Returns scope summary including line type

### 5. Test Coverage
- `test_scope_type_transitions.py` - Comprehensive tests for line classification
- `test_yaml_parser.py` - Updated with type-specific scope transition tests

## Acceptance Criteria Met

✓ Line type is tracked in parser state
✓ State updates correctly on each line processed
✓ Line type information is available for downstream logic

## Related
- Commit: 1bd4760a feat(bf-5tqu1y): Track line type in parser state
- Bead: bf-2cbjqu (Type-specific scope transition handling)
