# Bead bf-3x6ov: Test Assertions Verified

## Task
Review and fix test assertions to match the actual parser behavior for plain multi-line scalars.

## Findings
All 21 tests in `yaml_plain_multiline_scalar_comment_test.rs` pass successfully.

### Verified Behavior

1. **Lines starting with # ARE comments in plain scalar context**
   - Unlike block scalars (literal `|` or folded `>`), plain scalars treat `#` as a comment start
   - Test: `test_hash_in_plain_scalar_starts_comment` ✓

2. **Inline comment stripping works correctly**
   - `#` preceded by whitespace triggers comment stripping
   - `#` NOT preceded by space is preserved (e.g., URLs with anchors)
   - Test: `test_hash_symbol_in_plain_scalar_value` ✓
   - Test: `test_multiple_hashes_in_plain_scalar` ✓

3. **Plain scalar classification is correct**
   - Mapping keys are properly classified
   - Continuation lines are not classified as comments
   - Tests: `test_plain_scalar_single_line_classification`, `test_plain_scalar_multiline_continuation` ✓

4. **Multi-line plain scalars work correctly**
   - Comment lines interspersed in plain scalars are detected
   - Hash symbols in content are handled correctly
   - Tests: `test_multiline_plain_scalar_with_comment_lines`, `test_multiline_plain_scalar_with_hash_in_content` ✓

5. **Plain vs block scalar differences are documented**
   - Tests document the key difference: plain scalars never treat `#` as content, while block scalars preserve `#` when in block context
   - Tests: `test_plain_vs_literal_block_classification`, `test_plain_vs_folded_block_classification` ✓

## Conclusion
No changes needed. The test assertions correctly verify the actual parser behavior according to YAML spec.

## Test Output
```
running 21 tests
test result: ok. 21 passed; 0 failed; 0 ignored; 0 measured
```
