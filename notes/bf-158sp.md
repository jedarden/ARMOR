# Test Location: type_like_string_false_positive

## File Location
`/home/coding/ARMOR/tests/type_like_string_false_positive_test.rs`

## Purpose
This comprehensive test suite validates that the YAML parser correctly handles **strings that look like types or tags but aren't actual YAML tags**. The file contains 9,240 lines with extensive test coverage.

## What It Validates

The test verifies that `classify_line_type()` correctly distinguishes between:

1. **Actual YAML tags** (should return `LineType::Tag`):
   - `!tag`, `!!str`, `!!map`, `!!seq`
   - `!custom_type`, `!ns:tag`
   - Indented tags: `  !tag`, `    !!str`

2. **False positives** (should NOT return `LineType::Tag`):
   - Comments with `!`: `# ! important note`, `# TODO: fix this!`
   - Quoted strings with `!`: `key: "value with ! inside"`
   - Values ending with `!`: `message: Hello World!`
   - URLs with `!`: `url: http://example.com/resource!`
   - `!` at end of quoted values: `key: "Check this out!"`

## Exclamation Mark Handling Behavior

The key exclamation mark classification logic being tested:

| Scenario | Example | Expected LineType | Reason |
|----------|---------|------------------|--------|
| Valid tag at line start | `!tag` | `Tag` | Standard YAML tag syntax |
| Double-bang prefix | `!!str` | `Tag` | YAML core type prefix |
| Tag with namespace | `!ns:tag` | `Tag` | Valid namespaced tag |
| `!` in comment | `# ! important` | `Comment` | Comments start with `#` |
| `!` in quoted value | `key: "value!"` | `MappingKey` | Inside quotes, not a tag |
| `!` at end of value | `note: Check!` | `MappingKey` | Part of string value |
| `!` with space after | `! tag` | `Tag` or `Unknown` | Ambiguous edge case |

## Test Structure

The file is organized into sections:
- **Section 1**: Exclamation marks in comments (not tags)
- **Section 2**: Exclamation marks in values (not tags)
- **Section 3**: Sequence and flow collection contexts
- **Section 4**: Valid vs invalid YAML tag patterns
- **Section 5**: False positive scenarios in various positions
- **Section 6**: Edge cases and ambiguous situations

## Related Bead
This test file is tied to bead `bf-rn9gx`.

## Key Functions Tested
- `classify_line_type()` - Main classification function
- `detect_mapping_key()` - Mapping key detection (imported but usage varies)

## Acceptance Criteria Status
- ✅ Found test file: `tests/type_like_string_false_positive_test.rs`
- ✅ Understood what validates: Distinguishes real YAML tags from false positives
- ✅ Identified exclamation mark handling: `!` at start of line = Tag, `!` in comments/values/quotes = Not Tag
- ✅ Documented location and purpose
