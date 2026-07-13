# Bead bf-15n3r: generate_folded_scalar_tests_for_level Implementation

## Task
Add `generate_folded_scalar_tests_for_level` helper function

## Finding
The function is **already implemented** in `/home/coding/ARMOR/tests/type_like_string_false_positive_test.rs`.

## Verification Results

### Acceptance Criteria - ALL MET ✅

1. **Function exists**: ✅ Implemented at line ~870
   - Location: `tests/type_like_string_false_positive_test.rs`
   
2. **Level-specific generation**: ✅ Supports all required levels
   - `level0` → `""` (no indentation)
   - `level1` → `"  "` (2 spaces)
   - `level2` → `"    "` (4 spaces)
   - `level3` → `"      "` (6 spaces)
   - `level4` → `"        "` (8 spaces)
   - `tab` → `"\t"` (tab character)
   - Default: `"  "` (2-space fallback)

3. **Parameters**: ✅ Correct signature
   ```rust
   fn generate_folded_scalar_tests_for_level(
       level: &str,
       keys: &[&str],
       modifiers: &[&str],
       indent_nums: &[u32],
   ) -> Vec<(String, String, armor::parsers::yaml::LineType)>
   ```

4. **Return type**: ✅ Returns Vec of test case tuples for single level

5. **Compilation**: ✅ Verified with `cargo check --tests` - SUCCESS

## Implementation Details

The function:
1. Maps level string to actual indentation pattern
2. Iterates over keys × modifiers × indent_nums (Cartesian product)
3. Creates folded scalar test cases using `create_folded_scalar_test` helper
4. Returns collected Vec of (input, expected_value, expected_line_type) tuples

## Example Usage (from documentation)

```rust
// Level 0 only (no indentation)
let test_cases = generate_folded_scalar_tests_for_level(
    "level0",
    &["text", "data"],
    &[">", ">-", ">+"],
    &[1, 2, 3, 4, 5],
); // Generates: 2 keys × 3 modifiers × 5 indent_nums = 30 cases

// Tab indentation only
let tab_cases = generate_folded_scalar_tests_for_level(
    "tab",
    &["key1", "key2"],
    &[">-"],
    &[2, 3, 4],
);
```

## Conclusion
Task completed - function was already implemented and compiles successfully.
