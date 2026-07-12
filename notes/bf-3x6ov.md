# Task bf-3x6ov: Verify Test Assertions for Plain Scalar Parser Behavior

## Task
Review and fix test assertions to match the actual parser behavior for plain multi-line scalars.

## Findings

### Test Status
✅ All 21 tests in `yaml_plain_multiline_scalar_comment_test.rs` are passing

### Parser Behavior Verified

The tests correctly verify the following YAML spec-compliant behavior for plain scalars:

1. **Lines starting with `#` ARE comments in plain scalar context**
   - Unlike literal/folded block scalars (where `#` is content)
   - Tests: `test_hash_in_plain_scalar_starts_comment`, `test_multiline_plain_scalar_with_comment_lines`

2. **Inline comment stripping works correctly**
   - `#` preceded by whitespace starts a comment (YAML spec rule)
   - `#` NOT preceded by space is preserved (e.g., URLs with anchors)
   - Tests: `test_hash_symbol_in_plain_scalar_value`, `test_strip_inline_comment_*`

3. **Hash preservation in content**
   - URLs: `http://example.com#anchor` preserves `#anchor`
   - Values: `value#hash` preserved when no space before `#`
   - Quoted strings: `#` inside quotes is always preserved
   - Tests: `test_multiple_hashes_in_plain_scalar`, `test_plain_scalar_with_mixed_content`

### Implementation Details
From `src/parsers/yaml/line_parser.rs`:

- `is_comment_line()` (lines 846-849): Returns `true` if trimmed line starts with `#`
- `strip_inline_comment()` (lines 1025-1080): 
  - Tracks quote state to preserve `#` inside strings
  - Checks previous character for whitespace to determine if `#` starts comment
  - Stops processing at first `#` preceded by whitespace

### Conclusion
✅ Test assertions are correct and match actual parser behavior
✅ All acceptance criteria met:
- All test assertions match actual parser behavior
- Tests correctly verify `#` starts comments in plain scalars (unlike block scalars)
- Inline comment stripping works correctly

## Recommendation
The bead can be closed as the work is complete - the tests are already correctly asserting the expected parser behavior.
